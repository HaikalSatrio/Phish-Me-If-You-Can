"""Summarize the pilot without treating field presence as semantic accuracy.

Reference coverage counts candidate URLs, never arbitrary links in page content.
API reference-field checks measure source agreement, not current real-world truth.
"""
import argparse
import collections
import itertools
import json
from pathlib import Path
import statistics
import urllib.parse

from benchmark_consistency import read, save, sha


def urlkey(value):
    u = urllib.parse.urlsplit(value)
    host = (u.hostname or "").lower().removeprefix("www.")
    path = urllib.parse.unquote(u.path).rstrip("/")
    if host == "github.com":
        path = path.lower()
    return host + path


def norm(value):
    return " ".join(str(value).casefold().split()).strip().lstrip("@")


def jaccard(a, b):
    return len(a & b) / len(a | b) if a | b else None


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--run", type=Path, required=True)
    p.add_argument("--corpus", type=Path, default=Path("fixtures/consistency-pilot-v1.json"))
    p.add_argument("--out", type=Path, required=True)
    p.add_argument("--audit", type=Path)
    p.add_argument("--partial", action="store_true")
    args = p.parse_args()
    corpus, manifest = read(args.corpus), read(args.run / "manifest.json")
    if sha(args.corpus) != manifest["spec"]["corpus_sha256"]:
        raise ValueError("Corpus changed")
    cases = {c["id"]: c for c in corpus["cases"]}
    labels = read(args.audit)["labels"] if args.audit else {}
    rows, snapshots, audit = [], {}, {}
    events = collections.defaultdict(collections.Counter)
    details = collections.Counter()
    bytype = collections.defaultdict(collections.Counter)
    for checkpoint in sorted(args.run.glob("round-*/*.result.json")):
        row = read(checkpoint)
        path = checkpoint.with_name(checkpoint.name.replace(".result.json", ".json"))
        case, arm, number = cases[row["case"]], row["arm"], row["round"]
        if not path.exists():
            rows.append({**row, "missing_snapshot": True})
            continue
        if sha(path) != row["snapshot_sha256"]:
            raise ValueError("Changed snapshot " + str(path))
        data = read(path)
        candidates = data.get("candidates") or []
        evidence = data.get("evidence") or []
        urls = {urlkey(c["profile_url"]) for c in candidates}
        known = {urlkey(u) for u in case["reference_urls"]}
        account_url = urlkey(("https://github.com/" if case["platform"] == "github" else "https://instagram.com/") + case["username"])
        candidate_by_id = {c["id"]: c for c in candidates}
        observed = {(urlkey(candidate_by_id[e["candidate_id"]]["profile_url"]), e["type"], norm(e["value"]))
                    for e in evidence if e["status"] == "OBSERVED" and e["candidate_id"] in candidate_by_id}
        snapshots[(row["case"], arm, number)] = (urls, observed)
        row.update(known_urls_found=len(urls & known), known_urls_total=len(known),
                   unique_candidate_urls=len(urls), duplicate_candidate_urls=len(candidates)-len(urls),
                   nonseed_urls_found=len(urls & (known - {account_url})), nonseed_urls_total=len(known - {account_url}),
                   at_candidate_cap=len(candidates) >= manifest["spec"]["config"]["candidate_limit"],
                   budget_exhausted=row["requests"] >= manifest["spec"]["config"]["request_budget"],
                   observed_types=dict(collections.Counter(e["type"] for e in evidence if e["status"] == "OBSERVED")),
                   identity_decisions=dict(collections.Counter(c["decision"] for c in candidates)),
                   candidate_urls=sorted(urls), audit_statuses=dict(collections.Counter(labels.get(case["id"] + "|" + u, {}).get("label", "UNREVIEWED") for u in urls)))
        for event in data.get("search", {}).get("events", []):
            operation = event["operation"]
            if operation.startswith("web:page:"):
                operation = "web:page"
            elif operation.startswith("github:user:"):
                operation = "github:user"
            events[arm + "|" + operation][event["status"]] += 1
            if event["status"] in ("FAILED", "BLOCKED", "NOT_CHECKED") or "unavailable" in event["detail"]:
                details[(arm, operation, event["status"], event["detail"])] += 1
        bytype[arm].update(row["observed_types"])
        for candidate in candidates:
            key = case["id"] + "|" + urlkey(candidate["profile_url"])
            values = [e for e in evidence if e["candidate_id"] == candidate["id"]]
            entry = audit.setdefault(key, {"case": case["id"], "name": case["name"], "url": candidate["profile_url"],
                                          "occurrences": [], "observations": [], "label": "UNREVIEWED"})
            entry["occurrences"].append(f"{number}:{arm}")
            for e in values:
                if e["type"] in ("page_title", "page_description", "search_title", "search_snippet", "name"):
                    item = {"type": e["type"], "value": e["value"][:500]}
                    if item not in entry["observations"]:
                        entry["observations"].append(item)
        refpath = args.run / "references" / (case["id"] + ".json")
        row["reference_fields"] = []
        if refpath.exists() and read(refpath).get("status") == 200:
            fields = read(refpath)["fields"]
            # Evaluate the same GitHub source in each arm, excluding email and bio.
            target_ids = {c["id"] for c in candidates if urlkey(c["profile_url"]) == account_url}
            scoped = [e for e in evidence if e["candidate_id"] in target_ids and e["status"] == "OBSERVED"]
            fieldtypes = {"name": "name", "company": "workplace", "blog": "website", "location": "location"}
            for field, kind in fieldtypes.items():
                value = fields.get(field)
                if not value:
                    continue
                equivalent = lambda x: urlkey(x) == urlkey(value) if field == "blog" else norm(x) == norm(value)
                raw_match = any(norm(value) in norm(e["value"]) for e in scoped)
                structured_match = any(e["type"] == kind and equivalent(e["value"]) for e in scoped)
                row["reference_fields"].append({"field": field, "source_present": bool(target_ids),
                                               "literal_observed": raw_match, "typed_observed": structured_match})
        rows.append(row)
    expected = manifest["spec"]["rounds"] * sum(3 if c["platform"] == "github" else 2 for c in cases.values())
    expected_keys = {(c["id"], a, n) for c in cases.values()
                     for a in (("web", "account", "api") if c["platform"] == "github" else ("web", "account"))
                     for n in range(1, manifest["spec"]["rounds"] + 1)}
    actual_keys = {(r["case"], r["arm"], r["round"]) for r in rows}
    if len(actual_keys) != len(rows) or not actual_keys <= expected_keys:
        raise ValueError("Unexpected or duplicate case/arm/round")
    if not args.partial and (len(rows) != expected or actual_keys != expected_keys):
        raise ValueError(f"Incomplete matrix {len(rows)}/{expected}")
    stability = []
    for case in cases:
        for arm in ("web", "account", "api"):
            runs = sorted((n, v) for (c, a, n), v in snapshots.items() if c == case and a == arm)
            for (n1, (u1, e1)), (n2, (u2, e2)) in itertools.combinations(runs, 2):
                stability.append({"case": case, "arm": arm, "rounds": [n1, n2],
                                  "url_jaccard": jaccard(u1, u2), "observation_jaccard": jaccard(e1, e2)})
    summary = {}
    for arm in ("web", "account", "api"):
        group = [r for r in rows if r["arm"] == arm and not r.get("missing_snapshot")]
        if not group:
            continue
        fields = [f for r in group for f in r["reference_fields"]]
        summary[arm] = {
            "runs": len(group), "status": dict(collections.Counter(r["status"] for r in group)),
            "exit_codes": dict(collections.Counter(r["exit_code"] for r in group)),
            "candidates": sum(r["candidates"] for r in group), "requests": sum(r["requests"] for r in group),
            "unique_candidate_url_instances": sum(r["unique_candidate_urls"] for r in group),
            "duplicate_candidate_url_instances": sum(r["duplicate_candidate_urls"] for r in group),
            "at_candidate_cap": sum(r["at_candidate_cap"] for r in group), "budget_exhausted": sum(r["budget_exhausted"] for r in group),
            "median_seconds": statistics.median(r["duration_seconds"] for r in group),
            "max_seconds": max(r["duration_seconds"] for r in group),
            "known_urls_found": sum(r["known_urls_found"] for r in group), "known_urls_total": sum(r["known_urls_total"] for r in group),
            "nonseed_urls_found": sum(r["nonseed_urls_found"] for r in group), "nonseed_urls_total": sum(r["nonseed_urls_total"] for r in group),
            "reference_fields": {"total": len(fields), "source_present": sum(f["source_present"] for f in fields),
                                 "literal_observed": sum(f["literal_observed"] for f in fields), "typed_observed": sum(f["typed_observed"] for f in fields)},
            "audit_statuses": dict(sum((collections.Counter(r["audit_statuses"]) for r in group), collections.Counter())),
            "identity_decisions": dict(sum((collections.Counter(r["identity_decisions"]) for r in group), collections.Counter())),
        }
        for metric in ("url_jaccard", "observation_jaccard"):
            scores = [r[metric] for r in stability if r["arm"] == arm and r[metric] is not None]
            summary[arm][metric + "_mean"] = statistics.mean(scores) if scores else None
    result = {"manifest": manifest, "rows": rows, "summary": summary, "stability": stability,
              "event_status_counts": {k: dict(v) for k, v in events.items()}, "observed_type_counts": {k: dict(v) for k,v in bytype.items()},
              "diagnostics": [{"arm": a, "operation": o, "status": s, "detail": d, "count": n} for (a,o,s,d),n in details.most_common()],
              "reference_hashes": {p.name: sha(p) for p in sorted((args.run / "references").glob("*.json"))},
              "audit_sha256": sha(args.audit) if args.audit else None, "reporter_sha256": sha(__file__),
              "limitations": corpus["limitations"] + ["Event counts are diagnostic operations, not per-HTTP-request availability.",
                  "API controls are only six supplied GitHub handles; not a discovery baseline.", "Literal matches are source-field retention proxies, not semantic verification.",
                  "Repetitions are dependent; do not treat candidate occurrences as independent people."]}
    save(args.out, result)
    save(args.run / "audit-candidates.json", {"labels": audit})
    print(json.dumps({"summary": summary, "events": result["event_status_counts"], "audit_candidates": len(audit)}, indent=2))


if __name__ == "__main__":
    main()
