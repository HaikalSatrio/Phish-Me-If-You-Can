# Validation

Validated on Windows with Go 1.27.2 on 18 October 2026.

- `go test ./...`: passed for CLI, evidence, identity, scoring, and storage.
- `go build -o bin/pmiyc.exe ./cmd/pmiyc`: passed.
- Executable import of the Kevin fixture and report generation for Kevin and Laura: passed.
- Experimental fixture results under `experimental_v1`: Kevin 93.25, Laura 40.32, each with declared synthetic-source coverage 1.0.

The scores demonstrate distinct outputs under an arbitrary development rubric. They are not validation of predictive accuracy, completeness of live collection, or actual susceptibility.

Tests cover duplicate resistance, normalization idempotence, insufficient evidence, failed sources, low identity confidence, identity contradictions, missing provenance, invalid weights, reference cycles, cross-candidate support, alias collisions, preservation of existing files, and the import/review/analyze flow.

For the initial evidence milestone, no live OSINT, model inference, or human-response study was performed. Go was downloaded to temporary storage for validation; it was not added to the system PATH. The built Windows executable is available locally under `bin/` and is ignored by Git.

## GitHub collector milestone

Validated on Windows with Go 1.27.2 on 18 October 2026:

- Automated collector tests cover user search and profile retrieval, rate limits, missing profiles, malformed responses, mismatched profile identity, request budgets, cancellation, and rejected redirects.
- Search orchestration tests cover unreviewed-score abstention, review followed by scoring, refinements, contradictory clues, attempt limits, empty results, and incomplete coverage.
- CLI tests cover exclusive candidate confirmation and recording operator decisions.
- Live username lookup of GitHub's public `octocat` example account created a project using one API request. Review and analysis produced a scoped experimental report.
- Live name search for `The Octocat` collected five candidates with six requests, returned PARTIAL, and preserved unresolved identities. Results depended on GitHub's current search index.

Live test snapshots and reports remain in ignored local `data/`; they are not repository fixtures. Collector tests use local HTTP servers and synthetic profiles, so normal tests do not make internet requests. These checks establish functionality, not identity calibration, complete OSINT coverage, or validity of human susceptibility predictions.

## Web and social milestone

Validated on Windows with Go 1.27.2 on 18 October 2026:

- Unit tests cover Google result HTML parsing, duplicate links, challenge detection, unknown layouts, public URL restrictions, IP address boundaries, metadata extraction, API formats, missing credentials, rate-limit stop behavior, Bluesky profile parsing, search refinement clues, and restricted browser destinations.
- Search tests confirm metadata-only projects do not receive a PES after review, and BLOCKED remains distinct from PARTIAL.
- Live direct HTML Google lookup returned a JavaScript gate. Local Chrome rendering also encountered a challenge, correctly producing BLOCKED with no fabricated results. Successful live Google extraction is not established.
- Live URL import from https://example.org produced page metadata with provenance and a pending candidate.
- Live public Bluesky lookup of the official bsky.app handle returned the expected handle and display name using one collection attempt.
- The platform listing exposes twelve configured discovery targets. It does not assert native access or successful live searches on all twelve platforms.
- No live Brave or Google Custom Search API calls were verified with customer credentials.

The current source build requires Go 1.26 or later due to golang.org/x/net v0.61.0. The supplied local executable was rebuilt. Captured HTML and project snapshots stay in ignored data/.
