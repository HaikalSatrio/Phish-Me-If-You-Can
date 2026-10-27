"""Compare verified DNS and HTTP protocols; never disable TLS or replay credentials."""
import argparse
import json
import socket
import subprocess
import time
import urllib.parse
import urllib.request
from pathlib import Path

from benchmark_consistency import now, save


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--out', type=Path, required=True)
    args = p.parse_args()
    if args.out.exists():
        p.error('Use a new output file')
    result = {'started_at': now(), 'dns': [], 'http': []}
    addresses = {}
    for host in ('html.duckduckgo.com', 'www.bing.com'):
        row = {'host': host, 'system': sorted({x[4][0] for x in socket.getaddrinfo(host, 443, type=socket.SOCK_STREAM)})}
        try:
            url = 'https://cloudflare-dns.com/dns-query?' + urllib.parse.urlencode({'name': host, 'type': 'A'})
            req = urllib.request.Request(url, headers={'Accept': 'application/dns-json'})
            with urllib.request.urlopen(req, timeout=10) as response:
                data = json.load(response)
            row['https_dns'] = data
            addresses[host] = [x['data'] for x in data.get('Answer', []) if x['type'] == 1]
        except Exception as exc:
            row['error'] = str(exc)
        result['dns'].append(row)
    for host, path in [('html.duckduckgo.com', '/html/?q=%22Simon%20Willison%22'), ('www.bing.com', '/search?format=rss&q=%22Simon%20Willison%22')]:
        for route, ips in [('system', []), ('verified_https_dns', addresses.get(host, [])[:1])]:
            if route != 'system' and not ips:
                continue
            for protocol in ('--http1.1', '--http2'):
                bodypath = args.out.parent / (host + '-' + route + '-' + protocol[2:] + '.body')
                command = ['curl', '--silent', '--show-error', '--noproxy', '*', '--max-time', '12',
                           '--user-agent', 'PMIYC/0.3 public-exposure-research', protocol,
                           '--output', str(bodypath), '--write-out', '%{http_code} %{http_version} %{time_total}', 'https://' + host + path]
                if ips:
                    command += ['--resolve', host + ':443:' + ips[0]]
                run = subprocess.run(command, capture_output=True, text=True, timeout=16)
                row = {'host': host, 'route': route, 'protocol': protocol, 'exit_code': run.returncode,
                       'result': run.stdout, 'error': run.stderr.strip(), 'at': now()}
                if bodypath.exists():
                    body = bodypath.read_text(errors='replace')
                    row.update(bytes=bodypath.stat().st_size, rss='<rss' in body, challenge='anomaly-modal' in body or 'challenge-form' in body)
                result['http'].append(row)
                save(args.out, result)
                time.sleep(3)
    result['finished_at'] = now()
    save(args.out, result)
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()
