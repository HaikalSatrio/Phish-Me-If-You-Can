# Validation

## Company inference audit and ambiguity hardening, 23 October 2026

The extractor now records a winning rule, focused source quote, quote type, up to three ranked alternatives, and an ambiguity flag. Ambiguity is raised when a competing organization is within 0.08 confidence and is not a related name, or when the profile/email domain conflicts with text. Related variants and abbreviations such as `Binus`, `Binus University`, and `Bina Nusantara University` remain auditable alternatives without being marked ambiguous. Education and event contexts such as "studied at", "alumni of", "student at", "spoke at", and "guest lecture" are rejected for bare organization-name rules; explicit employment and role rules still work. Bare organization spans are also rejected when they come only from page titles, award/article prose, or prose containing connectors, repeated tokens, or internal context prepositions. Domain inference is limited to profile-like URL paths; reference, marketplace, and social hosts are excluded. Labels and employment/phrase rules cover English, Indonesian, Spanish, French, German, Portuguese, Italian, and Dutch, with international legal forms and role phrases.

`TestCompanyEvaluation` runs the synthetic corpus at `internal/extract/testdata/company_eval.json`: 56 cases (30 positive, 26 negative), with precision 1.000 and recall 1.000 against the current regression gates (minimum precision 0.95, minimum recall 0.85). This is a regression fixture, not empirical calibration. Coverage includes English and Indonesian labels, international legal forms and employment phrases, role phrases, education/event exclusions, free-email/social domains, and prose false-positive regressions. Additional tests cover focused quotes, conflicting domain/text clues, related names and abbreviations/initialisms, structured alternative metadata, and persisted ambiguity metadata in search results.

`go test ./...`, `go vet ./...`, and `go build -o bin/pmiyc.exe ./cmd/pmiyc` passed. Live `search -source web -name "Linus Torvalds"` returned five candidates and 35 observations using 6/20 requests, status PARTIAL. Both Wikipedia pages and the GitHub profile inferred `Linux Foundation` (`employer-label-v1` or `organization-suffix-v2` after trimming social counts); the Wikiquote and Goodreads pages produced no company. No site-brand or article-prose company survived this run. The Computer History Museum candidate inferred historical employer `Transmeta` and retained `Open Source Development Labs`, `Free Standards Group`, and `Linux Foundation` as audited alternatives, with an ambiguity flag rather than silent resolution. A final Yohan run returned five candidates and 28 observations using 6/20 requests; `Binus`, `Binus University`, and `Binus Center` were inferred, with no ambiguity. One LinkedIn fetch was rate-limited and one BINUS page failed, so these are collection-behavior observations, not completeness claims.

## Deterministic company inference, 23 October 2026

`go test ./...`, `go vet ./...`, and `go build -o bin/pmiyc.exe ./cmd/pmiyc` passed after adding company extraction. New unit tests cover explicit company labels, employment/role phrases, Indonesian phrases, organization names, sentence boundaries, email/profile domains, free-email and social-domain rejection, personal domains, name relevance, and domain agreement. A search integration test confirms the inferred workplace is INFERRED, has supporting evidence IDs, does not alter candidate confidence, is listed in `excluded_ids`, and never enters a PES dimension.

Live `search -source web -name "Yohan Muliono"` returned five candidates and 28 observations using 6/20 requests, status PARTIAL. The inferred companies were `Binus`, `Bina Nusantara University`, `Binus Center`, and `Binus University`; each candidate was marked `company_inferred: true` and had one INFERRED workplace observation with no fabricated source record. Several destination pages were blocked or rate-limited, so this check demonstrates extraction behavior, not complete or verified employment history.

## Local search startup follow-up, 23 October 2026

Docker Desktop's Linux engine was started and the SearXNG container was pulled and started successfully. The service health endpoint returned `OK` and its engine configuration was accessible. The new `scripts/start-local-search.ps1` script was executed against the healthy service and completed successfully without recreating it. Its cold-start actions were exercised separately through the Docker CLI; no existing unrelated containers were stopped.

The rebuilt executable now tries local SearXNG first in default web mode, without requiring `-config`:

- `search -source web -name "Haikal Satrio Dewandaru"`: two indexed candidate links, six search observations, status PARTIAL. The destination Facebook page required login or a challenge and the SINTA page returned HTTP 403. Neither destination was successfully extracted or verified as the subject's profile.
- `search -source web -platform devto -name "Ben"`: five indexed links, 35 observations, and successful extraction of all five pages. Results included `dev.to/ben`, `dev.to/bfdev`, and article/series pages; this was not a list of five different people.
- `search -source devto -username ben`: one exact account and five observations. CLI output explicitly identifies exact username lookup rather than display-name search.

Both SearXNG searches reported one unavailable upstream engine while still returning results. Successful index discovery does not establish complete platform coverage or account ownership.

`go test ./...`, `go vet ./...`, and the Windows build passed after the follow-up changes. Added tests cover fallback to another validated IP while the first connection stalls, cancellation of losing dial attempts, rejection of mixed public/private DNS answers, secret-free network diagnostics, default local service routing, and clear CLI guidance for unquoted names and exact-username sources. HTTP connections still use validated, pinned public IPs; no system DNS settings were changed.

## Free public collection update, 23 October 2026

Validated on Windows with the existing Go 1.27.2 toolchain:

- `go test ./...`: passed, including redirect scope and budget, conflicting profile URLs, page text and link extraction, relevance ranking, search fallback, source provenance, API response validation, multi-source failure isolation, CLI defaults and resume, and SearXNG configuration tests.
- `go vet ./...`: passed.
- `go build -o bin/pmiyc.exe ./cmd/pmiyc`: passed; the local executable was rebuilt.
- `docker compose -f deploy/searxng/compose.yaml config --quiet`: passed with a validation-only secret supplied in the command environment.

The toolchain emitted nonfatal sandbox warnings when writing its external telemetry and module metadata caches. Test, vet, and build commands returned success.

Live checks, without paid services or API keys:

- DEV public API, username `ben`: one candidate and five observations, with a response hash and receipt.
- Direct DEV HTML, username `ben`: one candidate and four observations using `devto-html-v2`.
- Bluesky public API, handle `bsky.app`: one candidate and three observations.
- Default username collection, `ben`: DEV and Bluesky succeeded while the GitHub request failed in this environment. The session preserved both candidates separately and reported PARTIAL, using three requests. The matching usernames do not establish shared ownership.
- Web search, `NASA`: DuckDuckGo was unreachable; Bing returned five relevant links and all five destination pages were fetched. The saved result contained 29 observations using seven requests, including page text and public links where present.
- Explicit URL collection, `https://example.org`: one page candidate and two nonempty observations.
- Web search, `Ben Halpern`: no candidates. DuckDuckGo remained unreachable outside the sandbox, and all ten Bing items failed primary-name relevance. Diagnostics were saved and the CLI returned nonzero. This test does not establish account absence or reliable name-search coverage.

At the initial validation stage, SearXNG tests used a local HTTP fixture to verify JSON query parameters, ranking, deduplication, public-result restrictions, non-followed service redirects, and error handling. Docker Desktop's Linux engine was not running at that stage. The subsequent live startup and search validation is recorded above. SearXNG does not guarantee access to blocked upstream engines.

All live search snapshots are stored under ignored `data/`. These small checks demonstrate collection behavior, not broad platform reliability, identity accuracy, or PES validity. Generic page text and links remain excluded from PES, including after candidate confirmation; inferred company facts are also excluded.

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
