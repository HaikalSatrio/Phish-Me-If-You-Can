# Validation

## New-name comparison and passage extraction, 24 October 2026

The default pipeline is now `relations-v4`. Three previously unsearched names were collected once and replayed through rules and local Qwen before and after the changes. Observed evidence hashes match across replays. See [COMPARISON-NEW-NAMES.md](COMPARISON-NEW-NAMES.md) for per-name results, errors, model configuration, artifacts, and limitations. Rules now fill six company fields and nine role fields across 15 candidates; the earlier pipeline filled one incorrect company and five roles. Model proposals passing validation increased from three to thirteen, including overlap with rules. These counts do not measure general accuracy or establish current employment truth.

The full Go test suite, vet, and Windows build passed. Added regression cases cover varied grammar, incorrect attribution to a longer personal name, topic/employer separation, source passage bounds, later subject mentions, source ID mapping, rejection of generated quotes, and preservation of source citations. The existing 13-case known regression corpus was also rerun with Qwen: all ten expected relationships were retained and the four negative cases produced no retained relationships.

## Earlier relationship summary and local-model validation, 24 October 2026

The earlier pipeline used `relations-v3`. Regression fixes covered undated corroboration erasing correct summaries, legal company suffixes being absorbed into roles, full English/Indonesian/ISO date expressions, historical claim visibility, and multiple role claims. Operator relationship review is independent of identity review and persists through replay. Tests also cover summary reference validation, preserved input snapshots, refusal to overwrite outputs, model rejection details, constrained source IDs, model metadata, and optional raw responses.

`TestRelationRegressionEvaluation` uses 13 labeled cases in `internal/extract/testdata/relation-regression.json`: nine positive cases containing ten relationships, plus four negative cases. This corpus was used during implementation, including to refine normalization after the first model run. It is a known regression set, not a held-out benchmark. Field metrics compare unique values per case; relationship metrics additionally require exact kind, subject, field boundaries, years, and temporal state.

| Extractor | Role precision / recall | Company precision / recall | Relationship precision / recall | Empty cases |
| --- | --- | --- | --- | --- |
| Rules | 1.000 / 1.000 | 1.000 / 1.000 | 1.000 / 1.000 | 4/13 |
| Local Qwen after grounding | 1.000 / 0.889 | 1.000 / 0.875 | 1.000 / 0.900 | 5/13 |

The local run used installed `qwen2.5:7b`, digest `845dbda0ea48ed749caafd9e6037047aa19acfcfd82e704d7ca97d631a0b697e`, temperature 0, context 8192, and maximum output 3072 tokens. Thirteen sequential local extractions took 22.585 seconds in total on this workstation. No retained proposal had a wrong subject or period in this small corpus. Qwen invented an end year in one otherwise useful proposal; validation rejected it, causing the remaining miss. Rules recovered that case. Empty-case counts include correct abstentions on the four negatives, and are not all extraction failures. Accepted model proposals still require operator review before summary projection.

Reproduce rules and optionally the real local model from the repository root:

```powershell
go test ./internal/extract -run '^TestRelationRegressionEvaluation$' -count=1 -v
$env:PMIYC_EVAL_MODEL = 'qwen2.5:7b'
$env:PMIYC_EVAL_OUT = Join-Path (Get-Location) 'data/my-regression-evaluation.json'
go test ./internal/extract -run '^TestRelationRegressionEvaluation$' -count=1 -v
Remove-Item Env:PMIYC_EVAL_MODEL, Env:PMIYC_EVAL_OUT
```

The optional report refuses to overwrite an existing file. It records each proposal and trace alongside the metrics. Local audit artifacts are under ignored `data/validation-relations-v3-20261024-105735/`. Snapshot replays restored the corroborated Engineer/Orbit Labs fields, separated CEO from Example Software Ltd, and retained the dated 2018-2022 employment as history with clean company text. Saved Yohan and Brian snapshots now show multiple role claims with individual references. The saved Joko Widodo snapshot retains the 2014-2024 public-office history in `role_claims`; it does not label that history as current employment or fill company from incidental organizations.

A separate real-snapshot replay used Qwen on the first three Yohan candidates. Two proposals failed grounding (`FIELD_NOT_QUOTED` and `QUOTE_NOT_FOUND`), and one returned an empty list; two further candidates were skipped by the request budget. No model proposal was retained in that run. Rules preserved the supported roles. This result demonstrates that success on short synthetic cases does not establish usefulness on multi-document web snapshots. Source-specific context selection and independently labeled real snapshots remain priorities; current input selection still starts near the first matching subject within each observation.

These checks do not measure search coverage, independent real-profile precision, cross-source identity resolution, or current employment truth. Those require separately labeled snapshots and review, including common names, conflicting dates, sparse profiles, and blocked-source cases. This validation does not establish Black Hat Arsenal readiness.

## Earlier evidence-bound relationships, 24 October 2026

The earlier pipeline used `relations-v2`. Field-extractor results below describe an older pipeline; those functions and corpora remain baseline tests, not accuracy claims for the current pipeline.

- `go test ./...`, `go vet ./...`, and the Windows executable build passed using Go 1.27.2. Build emitted a nonfatal sandbox module-cache warning and returned success.
- `TestRelationshipChallenge`: 26 known regression cases, with all 15 positive cases matched and 11 abstention cases passing. On these deliberately challenging cases, the previous company extractor emitted 16 values inconsistent with the expected current-employer field; the relationship pipeline emitted none. Historical employers count as incorrect only when presented as current. These figures are not held-out precision/recall or evidence of general OSINT accuracy.
- Tests cover other-person employment, multi-person clauses, birth-hospital snippets, table neighbors, missing alias proof, negation, uncertainty, public office, year ranges, conflicting periods, conflicting employers, replay stability, input preservation, citation tampering, structured Person ownership, paragraph/table boundaries, and exclusion from PES after identity confirmation.
- Relation-derived summary role values use the existing canonical English mapping; relation role values and quotes remain source-language for audit. Company proper nouns are unchanged.
- Local model HTTP fixtures verify JSON Schema requests, missing/extra JSON rejection, invented organizations/dates, wrong subjects/evidence IDs, incomplete responses, response limits, forbidden remote origins, redirect rejection, and isolation of model proposals from summaries and identity decisions. Literal quote validation alone cannot detect every semantic misattribution; a multi-person fixture explicitly checks that such a proposal cannot alter the summary.
- Replayed the existing local Joko Widodo snapshot without new web requests. The Indonesian and English biography text produced historical public-office relationships for 2014-2024 with source citations. All five company summaries were empty; the hospital, cabinet-table organization and incidental ministry no longer populated company. Older inferred fields remain marked superseded for audit. The output is in ignored `data/jokowi-relations-v2-final.json`.

Ollama was not available during the initial relations-v2 implementation. A local Ollama installation with `qwen2.5:7b` was later exercised through the opt-in hybrid configuration; no real-model accuracy benchmark or independent unseen-person evaluation was performed. The supplied local-model configuration remains opt-in. See [EXTRACTION.md](EXTRACTION.md) for the remaining limitations.

## Profile headline and role-boundary semantics, 24 October 2026

Undated profile headers now produce `headline` relations instead of `unknown` periods. A headline is summary-eligible but remains unverified and adds an audit note; historical headline text remains historical and is withheld. `Head of Cyber Security Program` and `Head of Cybersecurity` are stored as full occupation roles rather than employers, while `CEO of Tesla` becomes employment with role `CEO` and organization `Tesla`. Membership phrases such as `Treasurer of HIMTI BINUS University` remain membership relations.

`TestProfileHeadlineRelations` covers six extraction cases, including explicit `at` employers and a historical headline; `TestProfileHeadlineRejectsSkillList` prevents a skills headline from becoming a role. `TestProfileHeadlineSummary` covers headline role, headline employment, and historical withholding. `go test ./...`, `go vet ./...`, and the Windows build passed. Replaying the saved Yohan snapshot now yields `Head of Cyber Security Program` and `Head of Cybersecurity` roles with empty company summaries; replaying the saved Brian snapshot keeps the `Cybersecurity & Web Development` skill list out of the role summary and leaves multi-role LinkedIn headlines as alternatives. These are regression and collection-behavior observations, not a live-profile accuracy measurement.

## Deterministic role inference, 24 October 2026

Role inference is implemented as a separate extractor with explicit labels, subject-anchored phrases, and title-segment rules. `TestRoleEvaluation` runs the synthetic corpus at `internal/extract/testdata/role_eval.json`: 49 cases (30 positive, 19 negative), with precision 1.000 and recall 1.000 against gates of 0.95/0.85. Coverage includes Yohan's head-of-program phrasing, English and Indonesian labels, "is"/"works as"/"serves as"/"adalah"/"bekerja sebagai"/"menjabat sebagai" phrases, role title segments, international role prose, and negative cases for credentials, education, membership, location, unrelated titles, and missing subjects. This is a regression fixture, not empirical calibration.

A search integration test confirms the inferred role is INFERRED `role` evidence, retains supporting IDs and audit metadata, does not alter candidate confidence or contradictions, is listed in `excluded_ids`, and never enters a PES dimension. Live `search -source web -name "Yohan Muliono"` returned five candidates and 32 observations using 6/20 requests, status PARTIAL. Roles were `Head of Cyber Security Program` (two BINUS pages), `Lecturer` (LinkedIn), and `Head of Cybersecurity` (Instagram); the BINUS Center certification page produced no role. Every inferred role had an audit rule/quote/quote type and candidate confidence remained 0. LinkedIn was rate-limited and one page fetch failed, so these are collection-behavior observations, not completeness claims.

## Role/company boundary hardening, 24 October 2026

The company extractor now has `executive-role-organization-v1` for phrases such as `CEO of Tesla` and `Treasurer of HIMTI BINUS University`. Leadership prefixes are removed only when the remaining span looks organization-like; `President University` remains intact. Event and prose markers such as `training`, `competition`, `and`, `yang`, and `pada` reject book-title or article fragments, and Indonesian education cues such as `pendidikan`, `gelar`, `sarjana`, and `studi` reject education affiliations.

Role inference now parses pipe-delimited LinkedIn headline segments, including `General Treasurer of HIMTI BINUS University`, and strips leadership `Role of Organization` phrases so the organization is not part of the role. It also rejects organization page titles such as `Himpunan Mahasiswa Teknik Informatika` and book/article titles. New unit regressions cover both reported cases plus `President University`, event targets, education prose, and book titles. The existing role corpus remains 49 cases and the company corpus remains 56 cases; both evaluation gates still pass.

`go test ./...`, `go vet ./...`, and `go build -o bin/pmiyc.exe ./cmd/pmiyc` passed. Live `search -source web -name "Brian Filbert Chandra"` returned five candidates and 32 observations using 6/20 requests, status PARTIAL. The three LinkedIn directory candidates inferred company `HIMTI Binus University` and role `General Treasurer`, with `Cybersecurity Undergraduate` retained as an alternative. The HIMTI page produced no role, and the personal site produced `Computer Science Student` with `General Treasurer` as an alternative. The personal-site company remained ambiguous across HIMTI/BINUS event spans, so it was not silently treated as a verified employer.

Live `search -source web -name "Elon Musk"` returned five candidates and 27 observations using 6/20 requests, status PARTIAL. The Tesla page inferred company `Tesla` and role `Co-founder and CEO`; the English Wikipedia page produced neither, and the Indonesian Wikipedia page retained an ambiguous company; its audited infobox role was later canonicalized in the next section. A previous book-title false positive (`Spacex and Tesla Is Shaping Our Future`) was removed. These are collection-behavior observations, not completeness claims.

## English role canonicalization, 24 October 2026

Inferred role values from the field-extractor baseline are normalized to canonical English through `internal/extract/role_english.go`. Exact phrases, longest-first phrase replacements, and word-level fallbacks cover common Indonesian roles such as `Dosen` -> `Lecturer`, `Rekayasawan Perangkat Lunak` -> `Software Engineer`, `Direktur Teknologi` -> `Technology Director`, `Kepala Divisi Keamanan Informasi` -> `Head of Information Security Division`, and `Pelatih` -> `Trainer`. Leadership connectors and titles such as `CEO dan CTO` -> `CEO and CTO`, `Bendahara Umum` -> `General Treasurer`, and `Wakil Ketua` -> `Vice Chair` are also normalized. In that baseline, canonicalization runs only after the source value passes role-likeness and organization-title rejection, so detection behavior is unchanged. The evidence quote remains in the original source language, and company proper nouns are not translated.

`TestRoleEvaluation` still runs 49 cases (30 positive, 19 negative) with precision 1.000 and recall 1.000 against gates of 0.95/0.85; the six Indonesian corpus expectations were updated to their English values. New unit tests cover the mapping table, `CEO and Product Architect` for the Indonesian Wikipedia infobox, and preservation of the Indonesian quote while the stored value is English.

`go test ./...`, `go vet ./...`, and `go build -o bin/pmiyc.exe ./cmd/pmiyc` passed. Live `search -source web -name "Brian Filbert Chandra"` remained at five candidates and 32 observations using 6/20 requests, with the same English roles (`General Treasurer`, `Computer Science Student`). Live `search -source web -name "Elon Musk"` remained at five candidates and 27 observations using 6/20 requests; the Indonesian Wikipedia role is now `CEO and Product Architect` with `CEO and CTO` as an alternative, while the original Indonesian infobox quote is retained. The company on that page remains ambiguous and is not presented as verified. These are collection-behavior observations, not completeness claims.

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
