"""Retry failed local-model cases once, preserving the primary matrix unchanged."""

import argparse
import json
from pathlib import Path
import subprocess
import urllib.request

from benchmark_matrix import digest, read, save


def failures(snapshot):
    return sum(x['status'] == 'FAILED' for x in snapshot.get('extraction', []) if x['mode'] == 'hybrid')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--run', type=Path, required=True)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    if not (args.run / 'summary-all.json').exists():
        parser.error('finish the primary matrix first')
    if args.out.exists():
        parser.error('retry output directory must be new')
    spec = read(args.run / 'manifest.json')['spec']
    if digest(args.binary) != spec['binary_sha256']:
        parser.error('binary differs from the primary matrix')
    with urllib.request.urlopen('http://127.0.0.1:11434/api/tags', timeout=5) as response:
        models = json.load(response)['models']
    if not any(m['name'] == spec['model'] and m['digest'] == spec['model_digest'] for m in models):
        parser.error('local model differs from the primary matrix')
    rows = []
    for previous in sorted(args.run.glob('*/*/*-ollama.json')):
        snapshot = read(previous)
        before = failures(snapshot)
        if not before:
            continue
        relative = previous.relative_to(args.run)
        target = args.out / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        source = previous.with_name(previous.name.replace('-ollama.json', '-rules.json'))
        command = [str(args.binary.resolve()), 'enrich', '-input', str(source), '-out', str(target),
                   '-model', spec['model'], '-timeout', str(spec['model_timeout']), '-max-requests', str(spec['model_requests'])]
        with target.with_suffix('.log').open('w', encoding='utf-8') as stream:
            try:
                code = subprocess.run(command, stdout=stream, stderr=subprocess.STDOUT, check=False,
                                      timeout=spec['model_timeout'] * spec['model_requests'] + 45).returncode
            except subprocess.TimeoutExpired:
                code = -1
        after = failures(read(target)) if code == 0 and target.exists() else None
        row = {'case': previous.parent.name, 'mode': previous.stem, 'failed_traces_before': before,
               'failed_traces_after': after, 'exit_code': code,
               'input_sha256': digest(source), 'primary_snapshot_sha256': digest(previous),
               'retry_snapshot_sha256': digest(target) if target.exists() else None}
        rows.append(row)
        print(json.dumps(row), flush=True)
    save(args.out / 'summary.json', {
        'method': 'One recovery attempt per failed model case with unchanged inputs, binary, model, timeout, and request budget. Primary results are not overwritten.',
        'spec': spec, 'cases_retried': len(rows),
        'fully_recovered_cases': sum(row['exit_code'] == 0 and row['failed_traces_after'] == 0 for row in rows),
        'cases': rows,
    })


if __name__ == '__main__':
    main()
