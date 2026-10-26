# Web and social collection

## Modes

| Source | Input | Collection mode | Requirement |
| --- | --- | --- | --- |
| auto | Known username | GitHub, DEV Community, and Bluesky public profile APIs | No required key; default for new username searches |
| direct | Known username | Site-specific public HTML profile rules | No API key or browser; GitHub and DEV Community only |
| web | Name, username, account clues, or public URL | Supplied page/account hints, SearXNG, DuckDuckGo HTML, Bing RSS, and English Wikipedia title fallback; then destination pages | No API key or browser |
| duckduckgo / bing | Name or username | One public search index, without destination fetches | No API key or browser |
| searxng | Name or username | Configured SearXNG JSON search, without destination fetches | Operator-run service; see local setup below |
| google | Name or username | Local browser renders the public search page | Installed Chrome, Chromium, or Edge; no API key |
| url | Explicit public URL | HTML metadata, page text, and links; bounded public redirects | Public HTTP(S) page without authentication |
| github | Name or username | Public GitHub profile API | Optional GITHUB_TOKEN |
| devto | Known username | DEV public profile API, including public location and website fields | No key |
| bluesky | Full handle | Public Bluesky profile API | No key |
| brave | Name or username | Search API index results | BRAVE_API_KEY |
| google-api | Name or username | Legacy Custom Search JSON API | Existing eligible access, GOOGLE_API_KEY and GOOGLE_CSE_ID |

New searches with `-account` default to web. With only `-username`, the default is auto. Name-only searches default to web. Explicit URLs without account flags default to url. Saved sessions retain their provider; legacy GitHub sessions continue to use GitHub. Set `-source` explicitly for predictable scripts. See [direct collection](DIRECT_COLLECTION.md) for collection receipts and offline snapshot comparisons.

Auto mode tries each supported API independently within the shared request and candidate budgets. A failed source does not prevent other platforms from being attempted. A short Bluesky username is tried only as `USERNAME.bsky.social`; custom domains require the full handle. Syntax-incompatible platforms are marked NOT_CHECKED. No usernames are merged into a shared identity.

## No-key web workflow

```powershell
.\bin\pmiyc.exe search -source web -name "Haikal Satrio"
.\bin\pmiyc.exe search -source web -name "Haikal Satrio" -platform instagram
.\bin\pmiyc.exe search -source web -name "Haikal" -username "@haikal_satrio" -platform instagram
.\bin\pmiyc.exe search -source web -name "Haikal" -account "instagram:@haikal_satrio" -account "github:HaikalSatrio"
.\bin\pmiyc.exe search -source web -name "Full Name" -url "https://example.org/profile"
```

Supplied URLs and scoped account hints are fetched before search indexes. Repeat `-account platform:username` for up to eight distinct clues. These are saved as `search.accounts`; refinement appends and deduplicates new clues. Do not mix `-account` with global `-username` or `-platform`. Handle routes are supported for github, devto, instagram, linkedin, facebook, x, tiktok, youtube, threads, reddit, pinterest, and bluesky. Bluesky requires a full handle. Use an explicit URL for other formats or federated instances. Route support does not guarantee that a site exposes usable public metadata; only Instagram and GitHub were checked live for this change.

A generated account URL is retained only when the destination stays on the requested platform and account path, and public title/description mentions the complete handle. Generic, mismatched, or unavailable-account titles do not establish a candidate. Explicit URLs are collected as supplied public-page clues. Private destinations, login pages, and challenges remain blocked. Duplicate page/index results share a candidate while keeping page and index provenance separate. Attempted account pages are not fetched twice in a run. Request and candidate budgets apply to all phases.

The index phase tries SearXNG at `http://127.0.0.1:8888`, or the configured `searxng_url`, then DuckDuckGo HTML and Bing RSS. Each account clue produces its own platform-scoped query. When supplied, username is the primary constraint; an optional leading `@` is removed. Without a username, results must match all normalized name words. SearXNG adds name, company, location, or combined hints, capped at four queries per account. Configure `search_pages` (1-3), `search_language`, `searxng_engines`, `search_time_range`, and `searxng_safesearch` in the collector config. Exact account paths rank ahead of incidental mentions. Company and location rank results rather than excluding incomplete snippets. Destination pages are fetched with remaining budget. A failed page does not discard indexed evidence. Neither a supplied handle nor a metadata match verifies identity or shared account ownership.

Search output includes targeted follow-up questions. `pmiyc next -input data/result.json` recomputes them without network access. Optional `-out data/questions.json` writes structured questions, candidate cards, and affected IDs without modifying the snapshot. `challenge -input data/result.json -out data/reviewed.json` prompts for identity decisions and a reason. Add `-interactive` to a new search to run this step after collection. Employment proposals use `review-relation`; missing public sources use another search. No identity or relationship is automatically accepted. See [identity review](IDENTITY-REVIEW.md).

Search endpoints can ignore terms or site filters and may be unavailable on a network. HTTP 200 alone is not success. Unrelated results are rejected and a response without usable matches is EMPTY or FAILED (depending on the provider), not proof of no public exposure. A successful search remains PARTIAL because index coverage is incomplete. See VALIDATION.md for observed successes and failures.

An empty SearXNG response with unavailable upstream engines is FAILED, with bounded diagnostics for rate limits or connection problems. The same failed provider is not retried by later research steps in that collection. The Wikipedia fallback performs one exact-title public API request, follows source title redirects, and excludes missing and disambiguation pages. It is English-only and does not cover people without articles. Platform-scoped handles require the matching account path; third-party mentions remain available through unscoped research. Account clues also retain a general name query for mononyms. See the [international matrix](MATRIX-EVALUATION.md) for coverage and limitations.

Without `-out`, each run creates a new snapshot. A failed or blocked collection with no candidates saves diagnostics and exits nonzero. The CLI prints provider events and observed values as well as errors. Long page text and links have a 400-character CLI preview; the saved JSON contains the full bounded observations. Google remains separately selectable.

## Optional local SearXNG

Use this option when public search endpoints are unavailable or insufficient. It has no search API subscription, but needs Docker Desktop with Linux containers and network access to upstream search engines. From the repository root:

```powershell
powershell -File scripts/start-local-search.ps1
.\bin\pmiyc.exe search -source web -name "Haikal Satrio Dewandaru"
.\bin\pmiyc.exe search -source web -platform devto -name "Ben"
```

The startup script checks Docker, starts Docker Desktop when needed, generates the service secret without printing it, starts the container, and waits for its health endpoint and engine configuration. It exits with a specific error if setup fails. Re-running it reuses a healthy local service. The search CLI itself does not start containers. A missing `dockerDesktopLinuxEngine` pipe means Docker's Linux engine is not running; repeating a search command cannot start it.

The service binds only to `127.0.0.1:8888`. JSON output is enabled in `deploy/searxng/settings.yml`. The image uses the upstream `latest` tag; pin a tested digest when preparing a reproducible demonstration. See the [upstream container documentation](https://docs.searxng.org/admin/installation-docker.html) and [search API](https://docs.searxng.org/dev/search_api.html).

An explicit `searxng_url` setting is saved in the search configuration and must remain unchanged when resuming a session. Sessions saved before the default config included `searxng_url` must be restarted rather than resumed. Web mode uses the fixed local service above unless the collector config sets a different `searxng_url`. Only the trusted search service may use loopback addressing; destination pages still require public addresses. Service redirects are not followed. Non-loopback service URLs must use public HTTPS on standard ports. `-source searxng` uses `configs/collector-v1.json` or another explicit configuration. Its `-platform all` performs the same expanded general queries and retains links belonging to registry domains, without issuing one query per platform. Upstream blocks and rate limits still apply.

## Exact usernames versus names

Web discovery can follow explicit biography links and search observed names or account handles beyond their original platform. Derived name hypotheses remain distinct from observed names. See [iterative research](ITERATIVE-RESEARCH.md) for budgets, provenance, education summaries, and measured limitations.

`search -source devto -username ben` retrieves the public account `https://dev.to/ben`, whose display name was Ben Halpern in the live check. It does not enumerate display names containing Ben. Use `search -source web -platform devto -name "Ben"` for indexed name discovery; results can include profile pages and articles and are not an exhaustive people directory. Multiword values need quotes: `-name "Haikal Satrio Dewandaru"`. CLI errors now explain this distinction before attempting invalid exact-username requests.

## No-key Google workflow

```powershell
.\bin\pmiyc.exe search -source google -name "Anton" -out data/anton-web.json
.\bin\pmiyc.exe candidates -input data/anton-web.json
.\bin\pmiyc.exe search -input data/anton-web.json -company "Example Company" -out data/anton-web-refined.json
```

The browser runs headless with a new temporary profile. It does not reuse personal cookies, saved logins, or extensions. No visible browser window is opened. Navigation is restricted to a generated Google search URL. Browser rendering loads Google page resources, so the budget counts navigations, not every network subrequest. Closing or cancellation is bounded by the configured timeout. Windows cancellation terminates the spawned browser process tree; the implementation does not terminate existing personal browser sessions.

The parser recognizes result links associated with heading elements. Consent, CAPTCHA, JavaScript gates, and unknown layouts do not become an empty successful result. Collection stops after a challenge. There is no CAPTCHA solving, consent clicking, user-agent rotation, or login automation. A successfully parsed first page remains PARTIAL because index completeness is unknown.

At the last live check, Google returned a challenge even after browser rendering. The mode is implemented and tested against representative HTML, but successful live extraction has not been established in this environment. It can fail due to search layout, network, or access restrictions. A saved JSON file preserves the diagnostic and is not proof of successful collection.

## Social discovery

```powershell
.\bin\pmiyc.exe platforms
.\bin\pmiyc.exe search -source google -name "Anton" -platform linkedin
.\bin\pmiyc.exe search -source google -name "Anton" -platform all
```

The registry includes DEV Community, Instagram, Facebook, LinkedIn, X/Twitter, TikTok, YouTube, Threads, Reddit, Pinterest, Bluesky, Mastodon, and GitHub. `all` means these thirteen configured entries, not every social network. Mastodon index discovery currently targets mastodon.social; other instances can be supplied as public URLs. DuckDuckGo and Bing issue site-restricted queries within the global candidate limit. A blocked provider or exhausted budget leaves remaining entries NOT_CHECKED.

The API modes provide structured GitHub, DEV, and Bluesky profile fields. Other platform entries provide indexed discovery and page observations. They are not native adapters for posts, friends, private accounts, relationship graphs, or account ownership verification. Results may be posts, organization pages, or other documents, not personal profiles.

## Explicit URL import

```powershell
.\bin\pmiyc.exe search -source url -name "Anton" -url "https://example.org/about" -out data/anton-page.json
```

The URL importer extracts title, description, up to 8,000 characters of page text plus a truncation marker, and up to 50 distinct public links. It prefers main or article elements, falling back to the body. Scripts, styles, navigation, forms, templates, and explicitly hidden elements are excluded; CSS visibility is not evaluated. Scripts are not executed and extracted links are not followed. It does not infer employment, interests, email ownership, or relationships from arbitrary text. HTTP redirects are followed within a maximum of five requests and the remaining request budget. Each destination is validated, redirect loops and HTTPS downgrades are rejected, and the final URL and response hash are preserved as provenance. PDF and non-HTML files are not supported.

Only HTTP(S), standard ports, and public destinations are allowed. Private, loopback, link-local, reserved destinations, and credential-bearing URLs are rejected. DNS addresses are checked and the connection is pinned to a checked address. HTTP API tokens are scoped to their configured fixed API provider and never attached to imported pages. The browser mode has its own isolated navigation model and does not use this HTTP transport.

## Evidence and scoring

Index results preserve the search page or API URL as provenance. Extracted destination observations preserve the actual fetched URL and response hash. Search titles/snippets, indexed URLs, page titles/descriptions/text/links, and profile descriptions are retained as observations but excluded from PES. Confirming a candidate does not convert these into verified personal facts.

Search now uses subject-bound relationships, with public offices, employment, education, membership and birthplace kept separate. Historical positions do not become current employers. Conflicting claims retain their citations and periods; the inferred summary abstains. See [EXTRACTION.md](EXTRACTION.md) for the schema and offline replay command.

Optional local Ollama proposals are validated against literal source quotes and remain unverified. They do not automatically overwrite company/role or enter PES. The previous company/role rules and their synthetic corpora remain available as evaluation baselines; their older regression scores do not measure the new pipeline or real-world model accuracy.

Structured GitHub, DEV, and Bluesky profile fields can enter scoring after review under the experimental policy. Every candidate and its evidence remain separate. `challenge` can explicitly associate multiple profiles with the target; a complete decision record and reason are required. Conflicting clues prevent confirmation. The single-candidate `review` command retains its primary-candidate selection behavior. Identity confirmation does not verify employment claims or make extracted profile names and links eligible for scoring.

## Optional API providers

API keys are read only from environment variables. Never put them in project files or command arguments. Google Custom Search JSON API is closed to new customers and Google states that existing customers must migrate by January 1, 2027. The legacy adapter is optional, not a requirement for Google browser mode.

API adapters are tested with mocked HTTP responses. No live Brave or Google API search has been verified with customer credentials. No paid services are enabled or purchased by the application.

## References

- https://developer.chrome.com/docs/automation-and-testing/headless
- https://developers.google.com/custom-search/v1/overview
- https://api-dashboard.search.brave.com/app/documentation/web-search/get-started
- https://docs.bsky.app/
- https://pkg.go.dev/golang.org/x/net/html
## Diagnosing an empty web collection

When no candidate is collected and a source reports `FAILED` or `BLOCKED`, an empty fallback must not turn that attempt into successful empty coverage. Web discovery preserves the failure status; the CLI saves diagnostics and returns a nonzero exit code. A missing Wikipedia article only describes that exact-title lookup. Biography expansion does not repeat the same Wikipedia title request.

A running local SearXNG service does not establish that its upstream engines work. Inspect `unresponsive_engines` and container logs. A TLS hostname mismatch means the received certificate does not identify the requested host; investigate DNS and the network path before changing certificate trust. Do not disable TLS verification. Rate-limited engines require their cooldown; increasing the request budget does not repair them. With no collected text, local model extraction has no source evidence to process.
