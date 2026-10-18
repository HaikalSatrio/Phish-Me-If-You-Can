# Architecture

The core uses Go and the standard library. The CLI is an adapter; domain, evidence, identity, scoring, and storage packages can be called by a future local GUI service.

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

Identity matching is a policy over supplied confidence annotations, matches, contradictions, and operator decisions. Automated identifier comparison is a later collector/identity milestone. Names alone should not be treated as strong matching evidence.

Conflict detection is deliberately conservative and limited to divergent workplace and role values within a candidate. Concurrent roles can be valid but require a richer temporal schema in a later revision. Distinct accepted candidate profiles are not merged across platforms, so cross-profile duplicate control remains future work.

No source requests, message delivery, or model inference occur in the current engine.
