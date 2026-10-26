"""Publish post-audit replay results separately from the frozen live matrix."""

import argparse
from pathlib import Path

from benchmark_matrix import read, save, digest


def compact_rules(path):
    data = read(path)
    fields = ('role_candidates', 'company_candidates', 'education_candidates',
              'relations', 'grounding_reference_errors')
    for row in data['cases']:
        for phase in ('before', 'after'):
            row[phase] = {k: row[phase][k] for k in fields}
    data['artifact_sha256'] = digest(path)
    data['totals_note'] = 'Field totals count cases with at least one populated candidate; per-case rows count individual candidates. Neither measures correctness.'
    return data


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--rules', type=Path, required=True)
    parser.add_argument('--baseline', type=Path, required=True)
    parser.add_argument('--proposals', type=Path, required=True)
    parser.add_argument('--recovery', type=Path, required=True)
    parser.add_argument('--smoke', type=Path, required=True)
    parser.add_argument('--primary-report', type=Path, required=True)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    rules, baseline, proposals = compact_rules(args.rules), compact_rules(args.baseline), read(args.proposals)
    smoke = read(args.smoke)
    binary_hash = digest(args.binary)
    if any(x['binary_sha256'] != binary_hash for x in (rules, baseline, smoke)):
        parser.error('replay and smoke binaries differ')
    primary = read(args.primary_report)
    if rules['corpus_sha256'] != primary['manifest']['spec']['corpus_sha256'] or rules['outputs'] * 2 != primary['outputs']:
        parser.error('replay does not match the complete primary matrix')
    proposals['artifact_sha256'] = digest(args.proposals)
    for row in proposals['claims']:
        path = Path(row['snapshot']).resolve()
        row['snapshot'] = path.relative_to(Path.cwd()).as_posix()
        row['adjustments'] = [a for a in row['adjustments'] or [] if a['field'] != 'quote']
    save(args.out, {
        'version': 'matrix-post-audit-v1',
        'primary_report_sha256': digest(args.primary_report),
        'binary_sha256': binary_hash,
        'limitations': [
            'Fixes followed inspection of the heldout source audit; post-audit results are no longer an untouched heldout evaluation.',
            'Rules were replayed over all frozen observations; previously accepted model proposals were revalidated without fresh inference.',
            'Rejected model outputs were not retained, so proposal revalidation cannot measure recall or reproduce all original model decisions.',
            'The fresh local-model smoke test uses four already-inspected snapshots, not a second 800-output live matrix.',
            'Retries are separate operational recovery attempts and do not replace failed primary results.'],
        'rules_replay': rules, 'baseline_rules_replay': baseline,
        'model_proposal_revalidation': proposals,
        'primary_model_recovery': read(args.recovery), 'fresh_model_smoke': smoke,
    })
    print('Published post-audit results:', args.out)


if __name__ == '__main__':
    main()
