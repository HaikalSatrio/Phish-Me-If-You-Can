"""Read-only supplemental diagnostics; keeps TLS verification enabled."""
import argparse
import json
from pathlib import Path
import socket
import ssl
import subprocess
import time
import urllib.request

from benchmark_consistency import now, save


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True, type=Path)
    args = parser.parse_args()
    if args.out.exists():
        parser.error("Use a new output file")
    rows = []
    for host in ("html.duckduckgo.com", "www.bing.com", "api.github.com", "en.wikipedia.org"):
        row = {"host": host, "at": now()}
        start = time.monotonic()
        try:
            row["addresses"] = sorted({r[4][0] for r in socket.getaddrinfo(host, 443, type=socket.SOCK_STREAM)})
            with socket.create_connection((host, 443), timeout=10) as connection:
                with ssl.create_default_context().wrap_socket(connection, server_hostname=host) as secure:
                    cert = secure.getpeercert()
                    row.update(tls="verified", issuer=cert.get("issuer"), subject=cert.get("subject"))
        except Exception as exc:
            row.update(tls="failed", error=str(exc))
        row["seconds"] = round(time.monotonic() - start, 3)
        rows.append(row)
    search = {}
    try:
        url = "http://127.0.0.1:8888/search?q=%22Simon%20Willison%22&format=json"
        with urllib.request.urlopen(url, timeout=20) as response:
            data = json.load(response)
        search = {"url": url, "at": now(), "unresponsive_engines": data.get("unresponsive_engines"),
                  "result_count": len(data.get("results", [])),
                  "result_engines": sorted({e for r in data.get("results", []) for e in r.get("engines", [])})}
    except Exception as exc:
        search = {"error": str(exc)}
    try:
        container = subprocess.check_output(["docker", "inspect", "searxng-searxng-1", "--format", "{{.Config.Image}} {{.Image}}"], text=True).strip()
    except Exception as exc:
        container = str(exc)
    result = {"tls_probes": rows, "searxng": search, "container": container,
              "limitations": ["Supplemental probes are not the collector's own HTTP transport.",
                              "TLS failure cause needs investigation; verification was not disabled."]}
    save(args.out, result)
    print(json.dumps(result, indent=2))


if __name__ == "__main__":
    main()
