"""Bounded WSL-compatible live collector diagnostic; no model calls or automatic identity acceptance.

Run from repository root. Outputs preserve all failed runs. Existing checkpoints are
reused only with matching binary, config, corpus, script, and snapshot hashes.
"""
import argparse
import concurrent.futures
import datetime as dt
import hashlib
import json
from pathlib import Path
import platform
import random
import subprocess
import threading
import time
import urllib.request


def read(path):
    return json.loads(Path(path).read_text(encoding="utf-8"))


def save(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    temp = path.with_suffix(path.suffix + ".tmp")
    temp.write_text(json.dumps(value, ensure_ascii=True, indent=2) + "\n", encoding="utf-8")
    temp.replace(path)


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def now():
    return dt.datetime.now(dt.timezone.utc).isoformat()


def references(cases, root):
    """Freeze public API responses before experimental collection, excluding email."""
    for case in cases:
        if case["platform"] != "github":
            continue
        path = root / "references" / (case["id"] + ".json")
        if path.exists():
            continue
        url = "https://api.github.com/users/" + case["username"]
        result = {"url": url, "observed_at": now()}
        try:
            request = urllib.request.Request(url, headers={"User-Agent": "PMIYC-consistency-diagnostic", "Accept": "application/vnd.github+json"})
            with urllib.request.urlopen(request, timeout=15) as response:
                body = response.read(2 << 20)
                profile = json.loads(body)
                result.update(status=response.status, response_sha256=hashlib.sha256(body).hexdigest(),
                              fields={key: profile.get(key) for key in ("login", "name", "company", "blog", "location", "bio", "html_url")})
        except Exception as exc:
            result["error"] = str(exc)
        save(path, result)
        time.sleep(1)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, default=Path("bin/pmiyc-consistency"))
    parser.add_argument("--corpus", type=Path, default=Path("fixtures/consistency-pilot-v1.json"))
    parser.add_argument("--config", type=Path, default=Path("configs/collector-local-search.json"))
    parser.add_argument("--out", required=True, type=Path)
    parser.add_argument("--rounds", type=int, default=3)
    parser.add_argument("--gap", type=float, default=6)
    args = parser.parse_args()
    if args.rounds < 1 or args.rounds > 5 or args.gap < 6:
        parser.error("Use 1-5 rounds and at least six seconds between starts")
    args.binary = args.binary.resolve()
    corpus = read(args.corpus)
    config = read(args.config)
    if config.get("extraction", {}).get("mode", "rules") != "rules":
        parser.error("This diagnostic requires rules-only collection")
    cases = corpus["cases"]
    spec = {"binary_sha256": sha(args.binary), "corpus_sha256": sha(args.corpus),
            "config_sha256": sha(args.config), "runner_sha256": sha(__file__),
            "rounds": args.rounds, "gap": args.gap, "workers": 2,
            "commit": subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip(),
            "platform": platform.platform(), "config": config}
    manifest = args.out / "manifest.json"
    if manifest.exists():
        if read(manifest)["spec"] != spec:
            parser.error("Run spec changed; use a new directory")
    else:
        save(manifest, {"started_at": now(), "spec": spec, "limitations": corpus["limitations"]})
    references(cases, args.out)
    lock = threading.Lock()
    last_start = [0.0]

    def task(case, arm, round_number):
        stem = args.out / ("round-" + str(round_number)) / (case["id"] + "-" + arm)
        snapshot = stem.with_suffix(".json")
        checkpoint = stem.with_suffix(".result.json")
        if checkpoint.exists():
            row = read(checkpoint)
            if row.get("snapshot_sha256") and (not snapshot.exists() or sha(snapshot) != row["snapshot_sha256"]):
                raise ValueError("Changed snapshot: " + str(snapshot))
            return row
        if snapshot.exists():
            raise ValueError("Uncheckpointed output exists; inspect before resuming: " + str(snapshot))
        command = [str(args.binary), "search", "-config", str(args.config), "-out", str(snapshot)]
        if arm == "api":
            command += ["-source", "github", "-username", case["username"]]
        else:
            command += ["-source", "web", "-name", case["name"]]
            if arm == "account":
                command += ["-account", case["platform"] + ":" + case["username"]]
        stem.parent.mkdir(parents=True, exist_ok=True)
        with lock:
            delay = args.gap - (time.monotonic() - last_start[0])
            if delay > 0:
                time.sleep(delay)
            last_start[0] = time.monotonic()
        started = time.monotonic()
        row = {"case": case["id"], "arm": arm, "round": round_number, "started_at": now(), "command": command}
        with stem.with_suffix(".log").open("w", encoding="utf-8") as log:
            try:
                result = subprocess.run(command, stdout=log, stderr=subprocess.STDOUT,
                                        timeout=config["session_timeout_seconds"] + 30, check=False)
                row["exit_code"] = result.returncode
            except subprocess.TimeoutExpired:
                row.update(exit_code=-1, runner_error="process_timeout")
        row.update(finished_at=now(), duration_seconds=round(time.monotonic() - started, 3))
        if snapshot.exists():
            data = read(snapshot)
            row.update(snapshot_sha256=sha(snapshot), status=data.get("search", {}).get("status"),
                       requests=data.get("search", {}).get("requests_used"),
                       candidates=len(data.get("candidates") or []), observations=len(data.get("evidence") or []))
        save(checkpoint, row)
        print(json.dumps({key: value for key, value in row.items() if key not in ("command", "snapshot_sha256")}), flush=True)
        return row

    rows = []
    for round_number in range(1, args.rounds + 1):
        jobs = [(case, arm) for case in cases for arm in ("web", "account")]
        random.Random(20261027 + round_number).shuffle(jobs)
        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
            futures = [pool.submit(task, case, arm, round_number) for case, arm in jobs]
            rows.extend(future.result() for future in concurrent.futures.as_completed(futures))
        # API controls are serial; they measure supplied-handle access, not name discovery.
        for case in cases:
            if case["platform"] == "github":
                rows.append(task(case, "api", round_number))
        save(args.out / "progress.json", {"finished_round": round_number, "rows": rows})
    save(args.out / "completed.json", {"finished_at": now(), "rows": rows})


if __name__ == "__main__":
    main()
