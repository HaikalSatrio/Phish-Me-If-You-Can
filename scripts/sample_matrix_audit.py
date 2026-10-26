"""Select deterministic heldout claim samples for a separate source-level audit."""

import argparse
import hashlib
from pathlib import Path

from benchmark_matrix import read, save, digest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--run', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    if args.out.exists():
        parser.error('audit output must be new')
    claims = {}
    for path in sorted(args.run.glob('heldout/*/*-ollama.json')):
        snapshot = read(path)
        evidence = {e['id']: e for e in snapshot.get('evidence', [])}
        for r in snapshot.get('relations', []):
            if not r.get('role') and r['kind'] != 'employment':
                continue
            group = 'model' if r['method'].startswith('ollama/') else 'deterministic'
            key = '|'.join([path.parent.name, group, r['kind'], r.get('role', ''), r.get('organization', ''), r['quote']])
            identifier = hashlib.sha256(('matrix-audit-v1|' + key).encode()).hexdigest()
            e = evidence[r['evidence_id']]
            claims.setdefault(identifier, {
                'audit_id': identifier, 'case_id': path.parent.name, 'name': snapshot['search']['name'],
                'group': group, 'kind': r['kind'], 'subject': r['subject'], 'role': r.get('role', ''),
                'organization': r.get('organization', ''), 'temporal': r['temporal'],
                'quote': r['quote'], 'subject_quote': r.get('subject_quote', ''),
                'sources': [s['url'] for s in e.get('sources', [])],
                'snapshot': str(path), 'snapshot_sha256': digest(path), 'relation_id': r['id'],
                'assessment': 'NOT_REVIEWED', 'reason': '',
            })
    selected = []
    for group in ('deterministic', 'model'):
        for employment in (True, False):
            seen_people = set()
            limit = 8 if employment else 12
            for key, claim in sorted(claims.items()):
                if claim['group'] != group or (claim['kind'] == 'employment') != employment or claim['case_id'] in seen_people:
                    continue
                selected.append(claim)
                seen_people.add(claim['case_id'])
                if len(seen_people) >= limit:
                    break
    save(args.out, {'version': 'matrix-audit-v1', 'method': 'Hash-ordered heldout sample, at most one claim per person in each method/kind stratum; up to eight employment and twelve other role claims per method. Labels assess source entailment, not real-world truth or identity. This is a spot audit, not full-corpus precision.', 'claims': selected})
    print('Selected', len(selected), 'claims for source-level review.')


if __name__ == '__main__':
    main()
