"""Verify a completed paired matrix and publish compact operational results."""

import argparse
import collections
import csv
import datetime as dt
import json
from pathlib import Path

from benchmark_matrix import digest, metrics, read, report, save


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--run', type=Path, required=True)
    parser.add_argument('--corpus', type=Path, default=Path('fixtures/public-matrix-v1.json'))
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--baseline', type=Path)
    args = parser.parse_args()
    manifest = read(args.run / 'manifest.json')
    if digest(args.corpus) != manifest['spec']['corpus_sha256']:
        parser.error('corpus differs from the frozen run')
    cases = read(args.corpus)['cases']
    rows = []
    for case in cases:
        for arm in ('web', 'account'):
            observations = None
            for mode in (arm + '-rules', arm + '-ollama'):
                snapshot_path = args.run / case['split'] / case['id'] / (mode + '.json')
                checkpoint_path = snapshot_path.with_name(mode + '.result.json')
                if not snapshot_path.exists() or not checkpoint_path.exists():
                    parser.error('incomplete matrix: ' + str(snapshot_path))
                checkpoint, snapshot = read(checkpoint_path), read(snapshot_path)
                if checkpoint.get('runner_error') or checkpoint.get('snapshot_sha256') != digest(snapshot_path):
                    parser.error('failed or changed snapshot: ' + str(snapshot_path))
                if (checkpoint['id'], checkpoint['mode'], checkpoint['split']) != (case['id'], mode, case['split']):
                    parser.error('checkpoint identity mismatch')
                if snapshot.get('search', {}).get('name') != case['name']:
                    parser.error('snapshot query differs from the corpus case')
                expected_accounts = [{k: a[k] for k in ('platform', 'username')} for a in case['accounts']] if arm == 'account' else []
                if (snapshot.get('search', {}).get('accounts') or []) != expected_accounts:
                    parser.error('snapshot account clues differ from the corpus arm')
                for trace in snapshot.get('extraction', []):
                    if trace['mode'] == 'hybrid' and (trace.get('model') != manifest['spec']['model'] or trace.get('model_digest') and trace['model_digest'] != manifest['spec']['model_digest']):
                        parser.error('snapshot model differs from the frozen run')
                observed = [(e['id'], e['candidate_id'], e['type'], e['value'], e.get('sources'))
                            for e in snapshot.get('evidence', []) if e['status'] == 'OBSERVED']
                if observations is None:
                    observations = observed
                elif observed != observations:
                    parser.error('rules/model observation mismatch: ' + str(snapshot_path))
                calculated = metrics(snapshot, case)
                if calculated != checkpoint['metrics']:
                    parser.error('checkpoint metrics mismatch: ' + str(snapshot_path))
                rows.append({'id': case['id'], 'name': case['name'], 'split': case['split'],
                             'category': case['category'], 'account_platform': case['accounts'][0]['platform'],
                             'mode': mode, 'snapshot_sha256': checkpoint['snapshot_sha256'], **calculated})
    summary = {
        'version': 'public-matrix-report-v1', 'created_at': dt.datetime.now(dt.timezone.utc).isoformat(),
        'manifest': manifest, 'people': len(cases), 'outputs': len(rows),
        'paired_observations_equal': True,
        'categories': dict(collections.Counter(c['category'] for c in cases)),
        'account_platforms': dict(collections.Counter(c['accounts'][0]['platform'] for c in cases)),
        'country_reference_ids': len({x for c in cases for x in c['country_ids']}),
        'all': report(args.run, 'all'), 'development': report(args.run, 'development'),
        'heldout': report(args.run, 'heldout'),
        'limitations': read(args.corpus)['limitations'] + [
            'The Wikipedia fallback favors this corpus, whose inclusion required a Wikipedia article.',
            'A reference article match is not verification of identity or account ownership.',
            'Nonempty fields, literal citations, and successful model calls are not semantic accuracy.',
            'No independent full-corpus semantic labels or low-footprint private-person evaluation are included.',
            'Collection times and upstream availability vary; before/after retrieval is not a controlled causal comparison.',
            'Only four supplied-account platforms are covered in this live matrix; other adapters rely on separate regression tests.'],
        'cases': rows,
    }
    if args.baseline:
        summary['development_before'] = report(args.baseline, 'development')
        for mode in summary['development_before']['modes'].values():
            mode.pop('grounding_reference_errors', None)
        summary['development_before']['note'] = 'The earlier runner did not collect the structural grounding counter; it is omitted rather than treated as zero.'
    save(args.out, summary)
    flat = []
    for row in rows:
        flat.append({k: json.dumps(v, sort_keys=True) if isinstance(v, dict) else v for k, v in row.items()})
    with args.out.with_suffix('.csv').open('w', newline='', encoding='utf-8') as stream:
        writer = csv.DictWriter(stream, fieldnames=list(flat[0]))
        writer.writeheader()
        writer.writerows(flat)
    print(json.dumps({k: v for k, v in summary.items() if k not in ('cases', 'manifest', 'limitations')}, indent=2))


if __name__ == '__main__':
    main()
