"""One bounded query per already configured SearXNG engine; does not clear suspensions."""
import argparse
import json
from pathlib import Path
import time
import urllib.parse
import urllib.request
from benchmark_consistency import now, save

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--out', required=True, type=Path)
p.add_argument('--engines', nargs='+', default=['google', 'bing', 'brave'])
args = p.parse_args()
if args.out.exists():
    p.error('Use a new output')
rows = []
for index, engine in enumerate(args.engines):
    row = {'engine': engine, 'at': now()}
    try:
        params = urllib.parse.urlencode({'q': '"Simon Willison"', 'engines': engine, 'format': 'json'})
        with urllib.request.urlopen('http://127.0.0.1:8888/search?' + params, timeout=15) as response:
            data = json.load(response)
        row.update(results=len(data.get('results', [])), unavailable=data.get('unresponsive_engines'),
                   actual_engines=sorted({e for r in data.get('results', []) for e in r.get('engines', [])}))
    except Exception as exc:
        row['error'] = str(exc)
    rows.append(row)
    save(args.out, rows)
    print(json.dumps(row), flush=True)
    if index + 1 < len(args.engines):
        time.sleep(12)
