"""Replay frozen matrix observations through current rules without network or model calls."""

import argparse
from pathlib import Path
import subprocess

from benchmark_matrix import digest, metrics, read, save


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--run', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--corpus', type=Path, default=Path('fixtures/public-matrix-v1.json'))
    parser.add_argument('--split', choices=('all', 'development', 'heldout'), default='all')
    args = parser.parse_args()
    if args.out.exists():
        parser.error('replay output must be a new directory')
    manifest = read(args.run / 'manifest.json')
    if digest(args.corpus) != manifest['spec']['corpus_sha256']:
        parser.error('corpus differs from original collection')
    rows = []
    for case in read(args.corpus)['cases']:
        if args.split != 'all' and case['split'] != args.split:
            continue
        for arm in ('web', 'account'):
            relative = Path(case['split']) / case['id'] / (arm + '-rules.json')
            source, target = args.run / relative, args.out / relative
            checkpoint = read(source.with_name(arm + '-rules.result.json'))
            if digest(source) != checkpoint['snapshot_sha256']:
                raise ValueError('changed input snapshot: ' + str(source))
            target.parent.mkdir(parents=True, exist_ok=True)
            with target.with_suffix('.log').open('w', encoding='utf-8') as stream:
                subprocess.run([str(args.binary.resolve()), 'enrich', '-input', str(source), '-out', str(target)],
                               stdout=stream, stderr=subprocess.STDOUT, check=True, timeout=45)
            before, after = read(source), read(target)
            observed = lambda data: [e for e in data.get('evidence', []) if e['status'] == 'OBSERVED']
            if observed(before) != observed(after):
                raise ValueError('observations changed during replay')
            rows.append({'id': case['id'], 'split': case['split'], 'arm': arm,
                         'input_sha256': digest(source), 'output_sha256': digest(target),
                         'before': metrics(before, case), 'after': metrics(after, case)})
    totals = {}
    for split in ('all', 'development', 'heldout'):
        totals[split] = {}
        for arm in ('web', 'account'):
            selected = [r for r in rows if r['arm'] == arm and (split == 'all' or r['split'] == split)]
            totals[split][arm] = {'cases': len(selected)}
            for phase in ('before', 'after'):
                totals[split][arm][phase] = {
                    **{key: sum(bool(r[phase][key]) for r in selected)
                       for key in ('role_candidates', 'company_candidates', 'education_candidates')},
                    'relations': sum(r[phase]['relations'] for r in selected),
                    'grounding_reference_errors': sum(r[phase]['grounding_reference_errors'] for r in selected),
                }
    result = {'method': 'Rules-only replay of unchanged observations; no web collection or local model inference.',
              'binary_sha256': digest(args.binary), 'corpus_sha256': digest(args.corpus),
              'source_manifest_sha256': digest(args.run / 'manifest.json'),
              'outputs': len(rows), 'totals': totals, 'cases': rows}
    save(args.out / 'summary.json', result)
    print(result['outputs'], 'snapshots replayed; observations unchanged.')


if __name__ == '__main__':
    main()
