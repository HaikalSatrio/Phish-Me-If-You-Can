"""Resumable paired live collection and local-model replay; no semantic accuracy claims."""

import argparse
import collections
import concurrent.futures
import datetime as dt
import hashlib
import json
from pathlib import Path
import subprocess
import threading
import time
import urllib.parse
import urllib.request


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def read(path):
    return json.loads(path.read_text(encoding='utf-8'))


def save(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    temp = path.with_suffix(path.suffix + '.tmp')
    temp.write_text(json.dumps(value, ensure_ascii=True, indent=2) + '\n', encoding='utf-8')
    temp.replace(path)


def url_key(value):
    parsed = urllib.parse.urlsplit(value)
    return (parsed.hostname or '').lower().removeprefix('www.') + urllib.parse.unquote(parsed.path).rstrip('/').casefold()


def metrics(snapshot, case):
    candidates = snapshot.get('candidates') or []
    evidence = snapshot.get('evidence') or []
    relations = snapshot.get('relations') or []
    runs = [x for x in snapshot.get('extraction', []) if x.get('mode') == 'hybrid']
    urls = {url_key(c.get('profile_url', '')) for c in candidates}
    urls.update(url_key(s.get('url', '')) for e in evidence for s in e.get('sources', []))
    known = {' '.join(n.casefold().split()) for n in case['reference_names']}
    names = snapshot.get('name_claims', [])
    by_id = {e['id']: e for e in evidence}
    grounding_errors = 0
    for claim in relations + names:
        source = by_id.get(claim.get('evidence_id'), {})
        if source.get('candidate_id') != claim.get('candidate_id') or not claim.get('quote') or claim.get('quote', '') not in source.get('value', ''):
            grounding_errors += 1
        if claim.get('subject_evidence_id'):
            title = by_id.get(claim['subject_evidence_id'], {})
            if title.get('candidate_id') != claim.get('candidate_id') or title.get('value') != claim.get('subject_quote'):
                grounding_errors += 1
    return {
        'collection_status': (snapshot.get('search') or {}).get('status', 'MISSING'),
        'requests': (snapshot.get('search') or {}).get('requests_used', 0),
        'candidates': len(candidates), 'observations': len(evidence),
        'reference_article_seen': url_key(case['reference_article']) in urls,
        'role_candidates': sum(bool(c.get('role')) for c in candidates),
        'company_candidates': sum(bool(c.get('company')) for c in candidates),
        'education_candidates': sum(bool(c.get('education_claims')) for c in candidates),
        'relations': len(relations),
        'model_proposals': sum(r.get('method', '').startswith('ollama') for r in relations),
        'name_claims': len(names),
        'name_reference_agreements': sum(' '.join(n['name'].casefold().split()) in known for n in names),
        'model_statuses': dict(collections.Counter(x['status'] for x in runs)),
        'model_errors': dict(collections.Counter(x.get('error_code', '') for x in runs if x['status'] == 'FAILED')),
        'model_rejected': sum(x.get('rejected', 0) for x in runs),
        'grounding_reference_errors': grounding_errors,
        'unexpected_identity_decisions': sum(c.get('decision') not in ('PENDING', 'REVIEW', '') for c in candidates),
    }


class Runner:
    def __init__(self, args, cases):
        self.args, self.cases = args, cases
        self.print_lock, self.start_lock = threading.Lock(), threading.Lock()
        self.last_start = 0

    def run_command(self, command, output, timeout):
        started = time.monotonic()
        with output.with_suffix('.log').open('w', encoding='utf-8') as stream:
            try:
                result = subprocess.run(command, stdout=stream, stderr=subprocess.STDOUT,
                                        timeout=timeout, check=False)
                code, error = result.returncode, ''
            except subprocess.TimeoutExpired:
                code, error = -1, 'process_timeout'
            except OSError as exc:
                code, error = -2, type(exc).__name__ + ': ' + str(exc)
        return {'exit_code': code, 'runner_error': error, 'duration_seconds': round(time.monotonic() - started, 3)}

    def task(self, case, arm, model=False):
        mode = arm + ('-ollama' if model else '-rules')
        folder = self.args.out / case['split'] / case['id']
        folder.mkdir(parents=True, exist_ok=True)
        output, checkpoint = folder / (mode + '.json'), folder / (mode + '.result.json')
        if checkpoint.exists():
            result = read(checkpoint)
            if result.get('snapshot_sha256') and (not output.exists() or digest(output) != result['snapshot_sha256']):
                raise ValueError('snapshot changed after checkpoint: ' + str(output))
            return result
        # A complete output written before interruption can be recovered without another request.
        result = {'exit_code': None, 'runner_error': '', 'duration_seconds': None, 'recovered': output.exists()}
        if not output.exists():
            if model:
                source = folder / (arm + '-rules.json')
                if not source.exists():
                    result['runner_error'] = 'missing_collection_snapshot'
                else:
                    command = [str(self.args.binary), 'enrich', '-input', str(source), '-out', str(output),
                               '-model', self.args.model, '-timeout', str(self.args.model_timeout),
                               '-max-requests', str(self.args.model_requests)]
                    result = self.run_command(command, output, self.args.model_timeout * self.args.model_requests + 45)
            else:
                command = [str(self.args.binary), 'search', '-source', 'web', '-name', case['name'],
                           '-config', str(self.args.config), '-out', str(output)]
                if arm == 'account':
                    for account in case['accounts']:
                        command += ['-account', account['platform'] + ':' + account['username']]
                with self.start_lock:
                    time.sleep(max(0, self.args.collection_gap - (time.monotonic() - self.last_start)))
                    self.last_start = time.monotonic()
                result = self.run_command(command, output, read(self.args.config)['session_timeout_seconds'] + 30)
        result.update({'id': case['id'], 'name': case['name'], 'split': case['split'], 'mode': mode})
        if output.exists():
            try:
                result['metrics'] = metrics(read(output), case)
                result['snapshot_sha256'] = digest(output)
            except (ValueError, KeyError) as exc:
                result['runner_error'] = 'invalid_snapshot: ' + str(exc)
        save(checkpoint, result)
        with self.print_lock:
            status = result.get('metrics', {}).get('collection_status', result['runner_error'])
            print(json.dumps({'id': case['id'], 'split': case['split'], 'mode': mode,
                              'status': status, 'seconds': result['duration_seconds']}, ensure_ascii=True), flush=True)
        return result

    def run(self):
        # One model worker prevents simultaneous inference from exhausting local memory.
        with concurrent.futures.ThreadPoolExecutor(max_workers=1) as model_pool:
            model_jobs = []
            with concurrent.futures.ThreadPoolExecutor(max_workers=2) as web_pool:
                jobs = {web_pool.submit(self.task, c, arm): (c, arm)
                        for c in self.cases for arm in ('web', 'account')}
                for job in concurrent.futures.as_completed(jobs):
                    job.result()
                    case, arm = jobs[job]
                    model_jobs.append(model_pool.submit(self.task, case, arm, True))
            for job in concurrent.futures.as_completed(model_jobs):
                job.result()


def report(root, split):
    groups = collections.defaultdict(list)
    for path in root.glob('*/*/*.result.json'):
        row = read(path)
        if split == 'all' or row['split'] == split:
            groups[row['mode']].append(row)
    out = {'split': split, 'modes': {}, 'limitations': [
        'Counts measure operational outcomes and reference agreement, not semantic precision.',
        'Missing roles or employers can be correct abstentions; nonempty fields can still be wrong.',
        'Model proposals remain unverified and do not automatically populate the public summary.',
        'Heldout outputs must remain uninspected during development.']}
    for mode, rows in sorted(groups.items()):
        items = [r['metrics'] for r in rows if 'metrics' in r]
        out['modes'][mode] = {
            'completed': len(rows), 'valid_snapshots': len(items),
            'runner_errors': dict(collections.Counter(r['runner_error'] for r in rows if r['runner_error'])),
            'collection_statuses': dict(collections.Counter(m['collection_status'] for m in items)),
            'with_candidates': sum(m['candidates'] > 0 for m in items),
            'with_role': sum(m['role_candidates'] > 0 for m in items),
            'with_company': sum(m['company_candidates'] > 0 for m in items),
            'with_education': sum(m['education_candidates'] > 0 for m in items),
            'with_model_proposals': sum(m['model_proposals'] > 0 for m in items),
            'model_proposals': sum(m['model_proposals'] for m in items),
            'model_requests_completed': sum(sum(n for status, n in m['model_statuses'].items() if status != 'SKIPPED') for m in items),
            'reference_article_seen': sum(m['reference_article_seen'] for m in items),
            'model_calls': dict(sum((collections.Counter(m['model_statuses']) for m in items), collections.Counter())),
            'model_errors': dict(sum((collections.Counter(m['model_errors']) for m in items), collections.Counter())),
            'unexpected_identity_decisions': sum(m['unexpected_identity_decisions'] for m in items),
            'grounding_reference_errors': sum(m.get('grounding_reference_errors', 0) for m in items),
        }
    return out


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--corpus', type=Path, default=Path('fixtures/public-matrix-v1.json'))
    parser.add_argument('--binary', type=Path, default=Path('bin/pmiyc.exe'))
    parser.add_argument('--config', type=Path, default=Path('configs/collector-local-search.json'))
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--model', default='qwen2.5:7b')
    parser.add_argument('--model-timeout', type=int, default=30)
    parser.add_argument('--model-requests', type=int, default=5)
    parser.add_argument('--collection-gap', type=float, default=6, help='minimum seconds between live collection starts')
    parser.add_argument('--split', choices=['all', 'development', 'heldout'], default='all')
    parser.add_argument('--report-only', action='store_true')
    args = parser.parse_args()
    if args.report_only:
        print(json.dumps(report(args.out, args.split), indent=2), flush=True)
        return
    if not 1 <= args.model_timeout <= 300 or not 1 <= args.model_requests <= 20:
        parser.error('invalid model limits')
    if not 2 <= args.collection_gap <= 120:
        parser.error('collection gap must be between 2 and 120 seconds')
    args.binary = args.binary.resolve()
    cases = read(args.corpus)['cases']
    if len({c['id'] for c in cases}) != len(cases):
        parser.error('duplicate reference IDs')
    if read(args.config).get('extraction', {}).get('mode', 'rules') != 'rules':
        parser.error('collection must use rules only to preserve paired inputs')
    with urllib.request.urlopen('http://127.0.0.1:11434/api/tags', timeout=5) as response:
        installed = json.load(response)['models']
    model = next((m for m in installed if m['name'] == args.model), None)
    if not model:
        parser.error('requested model is not installed locally')
    spec = {'binary_sha256': digest(args.binary), 'corpus_sha256': digest(args.corpus),
            'config_sha256': digest(args.config), 'model': args.model, 'model_digest': model['digest'],
            'model_timeout': args.model_timeout, 'model_requests': args.model_requests,
            'runner_sha256': digest(Path(__file__)), 'network_workers': 2, 'model_workers': 1,
            'collection_gap_seconds': args.collection_gap}
    provenance = args.out / 'collection-provenance.json'
    spec['collection_provenance_sha256'] = digest(provenance) if provenance.exists() else None
    metadata = args.out / 'manifest.json'
    if metadata.exists():
        if read(metadata)['spec'] != spec:
            parser.error('run configuration changed; use a new output directory')
    else:
        save(metadata, {'created_at': dt.datetime.now(dt.timezone.utc).isoformat(), 'spec': spec})
    selected = [c for c in cases if args.split == 'all' or c['split'] == args.split]
    Runner(args, selected).run()
    save(args.out / ('summary-' + args.split + '.json'), report(args.out, args.split))


if __name__ == '__main__':
    main()
