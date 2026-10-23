# Architecture

The core uses Go; HTML ingestion uses the version-pinned golang.org/x/net/html parser. The CLI is an adapter; domain, evidence, identity, scoring, and storage packages can be called by a future local GUI service.

## Pipeline

1. Decode a single JSON document, rejecting unknown fields.
2. Validate schema version, candidates, facts, provenance, and coverage.
3. Normalize whitespace and fact categories. Merge identical observed claims within a candidate, retaining source records and original IDs as aliases.
4. Resolve candidate states under the selected policy.
5. Exclude unaccepted identities, non-observed facts, zero-confidence facts, and conflicting workplace or role claims.
6. Compute five experimental dimensions and scenario prerequisites.
7. Produce a report, or abstain from aggregate scoring if coverage or evidence is insufficient.

Evidence status and identity state are independent. Confirming a candidate does not promote inferred facts into observed facts. Explicit identity contradictions remain in REVIEW even after CONFIRM. Resolve contradictory input evidence in a new project snapshot before accepting the linkage.

## Storage decision

Version 1 uses local JSON snapshots rather than SQLite. This keeps the first engine dependency-free and its input/output inspectable. Files carry schema versions. A future storage adapter can migrate snapshots into a database without changing scoring rules.

Writes use a temporary file, sync, and a hard link to publish without overwriting. Filesystems without hard-link support return an error. Temporary files are removed. The implementation does not promise power-loss durability of directory metadata or encryption at rest.

Review creates a new snapshot. There is no authenticated multi-user audit trail yet. Reports record engine/schema versions and SHA-256 hashes of canonical normalized input JSON and policy JSON. Keep source snapshots and policy files together with reports for reproducibility. Source documents themselves are not downloaded or archived.

## Current limitations

Manual datasets retain their supplied identity annotations. GitHub collection adds exact normalized name, username, company, and location comparisons under a configurable experimental policy. Collected candidates always require operator review. Names alone are weak matching evidence.

Search orchestration runs the evidence-bound relation pipeline described in [EXTRACTION.md](EXTRACTION.md). Structured Person metadata and subject-bound prose produce typed relationships with literal quotes, evidence IDs and periods. Only a single unambiguous deterministic present-tense relationship can populate an inferred summary. Historical positions, unknown periods and competing relationships remain explicit. The old field extractors remain as evaluation baselines.

Optional local Ollama extraction uses a bounded JSON Schema request. Literal grounding and candidate ownership are validated. Model proposals remain unverified and do not automatically populate summaries or scoring. Rules, structured observations and model results share the relationship schema, while extraction diagnostics remain separate from collection coverage. Relationship role values and quotes remain in the source language for audit; inferred summary role values use the canonical English mapping, and company proper nouns are not translated.

Divergent observed workplace and role values remain excluded from scoring. Competing inferred relationships are retained with source and year precision, without choosing one employer or combining unrelated roles. Distinct candidate profiles are not merged across platforms; cross-profile identity resolution and temporal reconciliation remain future work.

Collectors support public GitHub/Bluesky profiles, explicit page metadata, and search providers. Google browser mode uses an isolated local browser profile and counts navigations rather than all subrequests. Collection and search orchestration live in separate packages and can be called by a future GUI. Session snapshots preserve clues, budgets, events, and source scope. No message delivery occurs. Model inference is opt-in and local. See COLLECTORS.md and WEB_AND_SOCIAL.md for capabilities and limitations.
