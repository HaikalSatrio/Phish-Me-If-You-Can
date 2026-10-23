# GitHub search and collection

The first collector uses public GitHub REST endpoints. It searches names with `GET /search/users` and fetches bounded candidate profiles with `GET /users/{username}`. Supplying a known username skips name search. Search rank is not identity confidence.

References:

- https://docs.github.com/en/rest/search/search#search-users
- https://docs.github.com/en/rest/users/users#get-a-user

## Usage

Run from the repository root:

```powershell
.\bin\pmiyc.exe search -source github -name "Anton" -out data/anton-search.json
.\bin\pmiyc.exe candidates -input data/anton-search.json
```

The terminal displays candidate IDs, usernames, public profile links, company/location fields, matching clues, missing clues, and contradictions. A company derived from text rather than a structured profile field is marked `(inferred, unverified)`. If ambiguous, refine with a known username or compare additional clues:

```powershell
.\bin\pmiyc.exe search -input data/anton-search.json -username "KNOWN_USERNAME" -out data/anton-refined.json
```

Replace `KNOWN_USERNAME` with an actual username. Alternatively supply `-company "Company"` or `-location "City"`. Company/location clues compare the bounded returned profiles; they do not expand the search or guarantee finding a person outside that result window. Resume refreshes observations, resets candidate decisions, and preserves session events and cumulative budgets. Previously collected profiles remain in the earlier snapshot.

Confirm a candidate using the ID printed by the command:

```powershell
.\bin\pmiyc.exe review -input data/anton-refined.json -candidate github-12345 -decision CONFIRM -out data/anton-reviewed.json
.\bin\pmiyc.exe analyze -input data/anton-reviewed.json -out data/anton-report.json
```

Replace `github-12345` with the returned candidate ID. CONFIRM is an operator assertion, not independent verification. Conflicting clues block confirmation. Start a fresh session with corrected clues when a clue itself was wrong. REJECT and UNSURE are also supported. Confirming a different candidate resets any previous confirmation in that search snapshot, preventing evidence from two selected people being combined.

Without `-out`, search automatically creates `data/<session-id>-<attempt>.json`. Existing output files are never overwritten. No JSON editing is necessary. There is no interactive questionnaire yet: CLI follow-up flags implement the guided refinement steps.

## Scope and evidence

Public profile username, display name, company, location, email, and website fields are captured when present. Biography is retained as profile_description metadata and excluded from PES. Empty fields produce no observed fact. Company is stored as workplace context but is a self-description, not verified employment. If the structured company is empty, deterministic extraction from page/snippet/biography text, email domain, or profile domain can add a separate INFERRED workplace supported by those observations; it is marked unverified, excluded from PES, and does not alter identity matching. The evidence carries the inference rule, exact quote, quote type, and ranked alternatives; near-ties and domain/text conflicts are marked ambiguous. Text is not parsed into personality traits, relationships, hobbies, or routines. Website links are stored as text and never followed.

The CLI prints collected observations. An account with only a public username is still a successful retrieval; blank name, company, or location fields do not mean the request failed. Supply one username after `-username`; do not append a second example username such as `octocat`.

Every fact carries the API source URL, observation timestamp, collector version, candidate ID, and explicit experimental annotations. The collector configuration is recorded in the search session. Profiles are always pending review, even when the heuristic score exceeds the PES acceptance threshold.

Reports include the GitHub-only scope. Collection coverage counts completed requests within declared scope, not completeness of public exposure across the internet. Truncated search results or interrupted profile collection add NOT_CHECKED coverage. Missing profiles are distinguished from blocked requests and errors. Search failure still saves a session with diagnostics, so inspect the displayed status even when saving succeeds.

## Configuration and network behavior

`configs/collector-v1.json` defines candidate limits, cumulative request budget, attempt budget, request/session timeouts, and annotation values. Defaults allow five candidates, twenty cumulative requests, and three attempts. Collection is sequential and bounded. There are no automatic retries or pagination beyond the first bounded search page.

The fixed API host is `api.github.com`; redirects are rejected. Response bodies are limited to 2 MiB. HTTP 401, 403, and 429 are recorded as BLOCKED. Error response bodies and token values are not persisted. Optional `GITHUB_TOKEN` is read from the process environment; do not store tokens in project JSON or the repository. The collector operates without a token where the API permits it.

Name matching uses normalized exact equality. Known usernames establish a stronger clue, and exact company/location matches add a configurable boost. Scores are experimental annotations, not calibrated identity probabilities. Case-insensitive mismatch in supplied company/location is treated conservatively as a contradiction when it is compared with a structured public field; an inferred company never matches or contradicts the operator clue. Spelling variants can require corrected clues. No automatic identity acceptance occurs.

Live GitHub collection sends the name or username query to GitHub. This collector does not collect private profiles, repositories, commits, or email permutations. Other collection modes are documented in WEB_AND_SOCIAL.md.
