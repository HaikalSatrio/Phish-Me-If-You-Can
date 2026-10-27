"""Count actual network attempts separately from cooldown skips and candidate retrieval."""
import argparse
import collections
from pathlib import Path
import statistics
import re
from benchmark_consistency import read, save, sha

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--run', required=True, type=Path)
p.add_argument('--out', required=True, type=Path)
args = p.parse_args()
manifest = read(args.run / 'manifest.json')
hosts = collections.defaultdict(collections.Counter)
operations = collections.defaultdict(collections.Counter)
rows = []
upstream_counts = collections.Counter()
for checkpoint in sorted(args.run.glob('round-*/*.result.json')):
    row = read(checkpoint)
    path = checkpoint.with_name(checkpoint.name.replace('.result.json', '.json'))
    if not path.exists():
        rows.append({'case': row['case'], 'round': row['round'], 'arm': row['arm'], 'missing_snapshot': True})
        continue
    if sha(path) != row['snapshot_sha256']:
        raise ValueError('Changed snapshot')
    snapshot = read(path)
    access = snapshot['search'].get('access', [])
    outcomes = collections.Counter()
    for event in access:
        if event['outcome'] == 'RESPONSE':
            key = 'http_' + str(event['http_status'])
        else:
            key = event['outcome']
        hosts[event['host']][key] += 1
        outcomes[key] += 1
    for event in snapshot['search']['events']:
        operation = event['operation']
        if operation.startswith('searxng:'):
            match = re.search(r'(\d+) upstream engines unavailable', event.get('detail', ''))
            if match:
                upstream_counts['queries_with_upstream_errors' if int(match[1]) else 'queries_without_upstream_errors'] += 1
            elif event.get('detail', '').startswith('Upstream engines unavailable and no results returned'):
                upstream_counts['queries_with_upstream_errors'] += 1
        if operation.startswith('web:page:'):
            operation = 'web:page'
        if operation.startswith('github:user:'):
            operation = 'github:user'
        operations[operation][event['status']] += 1
    actual = sum(v for k,v in outcomes.items() if k.startswith('http_') or k == 'NETWORK_ERROR')
    if access and actual != row['requests']:
        raise ValueError('Actual request accounting mismatch: ' + str(path))
    if row['requests'] > manifest['spec']['config']['request_budget']:
        raise ValueError('Request budget exceeded: ' + str(path))
    rows.append({k:row[k] for k in ('case','arm','round','status','requests','candidates','duration_seconds')} | {'access_counts':dict(outcomes)})
summary = {}
for arm in ('web','account','api'):
    group = [r for r in rows if r['arm']==arm and not r.get('missing_snapshot')]
    if group:
        summary[arm] = {'runs':len(group), 'with_candidates':sum(r['candidates']>0 for r in group),
                        'statuses':dict(collections.Counter(r['status'] for r in group)),
                        'median_seconds':statistics.median(r['duration_seconds'] for r in group),
                        'max_seconds':max(r['duration_seconds'] for r in group),
                        'requests':sum(r['requests'] for r in group)}
completed_path = args.run / 'completed.json'
completed = read(completed_path) if completed_path.exists() else None
complete = completed is not None and len(rows) == len(completed['rows']) and not any(r.get('missing_snapshot') for r in rows)
result = {'run':str(args.run), 'manifest':manifest, 'complete':complete,
          'stopped':read(args.run / 'stopped.json') if (args.run / 'stopped.json').exists() else None,
          'finished_at':completed['finished_at'] if completed else None,
          'reporter_sha256':sha(__file__), 'searxng_upstreams':dict(upstream_counts),
          'summary':summary,'hosts':{k:dict(v) for k,v in hosts.items()},
          'operations':{k:dict(v) for k,v in operations.items()},'rows':rows,
          'limitations':['DEFERRED is not successful access.', 'HTTP 200 alone does not establish valid content, relevance, or identity.',
                         'HTTP 404 is a completed lookup, not a transport error.', 'Comparison with older runs is affected by changing provider availability.',
                         'DNS lookups are infrastructure requests outside the content-request budget; no cached content is used.',
                         'Request counts measure content-client attempts. A network error does not prove that a request reached the provider.']}
save(args.out,result)
print(__import__('json').dumps({k:v for k,v in result.items() if k not in ('rows','limitations','manifest')},indent=2))
