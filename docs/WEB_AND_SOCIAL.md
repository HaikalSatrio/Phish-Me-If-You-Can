# Web and social collection

## Modes

| Source | Input | Collection mode | Requirement |
| --- | --- | --- | --- |
| direct | Known username | Site-specific public HTML profile rules | No API key or browser; GitHub and DEV Community only |
| web / bing | Name or username | Public Bing search RSS with local relevance and domain filtering | No API key or browser |
| google | Name or username | Local browser renders the public search page | Installed Chrome, Chromium, or Edge; no API key |
| url | Explicit public URL | HTML title and description, bounded public redirects | Public HTTP(S) page without authentication |
| github | Name or username | Public GitHub profile API | Optional GITHUB_TOKEN |
| bluesky | Full handle | Public Bluesky profile API | No key |
| brave | Name or username | Search API index results | BRAVE_API_KEY |
| google-api | Name or username | Legacy Custom Search JSON API | Existing eligible access, GOOGLE_API_KEY and GOOGLE_CSE_ID |

New searches with a username default to direct. Name-only searches default to web. Explicit URLs default to url. Saved sessions retain their provider; legacy GitHub sessions continue to use GitHub. Set `-source` explicitly for predictable scripts. See [direct collection](DIRECT_COLLECTION.md) for collection receipts and offline snapshot comparisons.

## No-key web workflow

```powershell
.\bin\pmiyc.exe search -source web -name "Haikal Satrio"
.\bin\pmiyc.exe search -source web -name "Haikal Satrio" -platform instagram
```

This mode uses Bing's public RSS endpoint, not a search API. Each returned item must contain all normalized query words across its title, description, and URL. Social results must also belong to the configured platform domain. This conservative relevance filter can discard valid results; a textual match is not identity verification. Results remain pending review. No destination pages are fetched automatically.

The endpoint can ignore search terms or site filters. HTTP 200 alone is not success. Unrelated results are rejected and a feed without usable matches is FAILED, not proof of no public exposure. Initial diagnostic requests returned relevant NASA results but unrelated results for the supplied personal name. This alternative does not establish reliable personal-name or social search in this environment. A successful feed remains PARTIAL because coverage is incomplete.

Without `-out`, each run creates a new snapshot. A failed or blocked collection with no candidates saves diagnostics and exits nonzero. The CLI prints successful events and observed values as well as errors. Google is selectable separately; providers are not silently substituted.

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

The registry includes Instagram, Facebook, LinkedIn, X/Twitter, TikTok, YouTube, Threads, Reddit, Pinterest, Bluesky, Mastodon, and GitHub. `all` means these twelve configured entries, not every social network. Mastodon index discovery currently targets mastodon.social; other instances can be supplied as public URLs. Each platform gets its own site-restricted query and a bounded number of candidates. A blocked provider or exhausted budget leaves remaining entries NOT_CHECKED.

Except for the explicit GitHub and Bluesky API modes, these are indexed discovery and optional page metadata capabilities. They are not native adapters for posts, friends, private accounts, relationship graphs, or account ownership verification. Results may be posts, organization pages, or other documents, not personal profiles. No username permutations or account claims are invented.

## Explicit URL import

```powershell
.\bin\pmiyc.exe search -source url -name "Anton" -url "https://example.org/about" -out data/anton-page.json
```

The URL importer extracts title and description only. Scripts are not executed and page links are not followed. It does not infer employment, interests, email ownership, or relationships from arbitrary text. HTTP redirects are followed within a maximum of five requests and the remaining request budget. Each destination is validated, redirect loops and HTTPS downgrades are rejected, and the final URL is preserved as provenance. PDF and non-HTML files are not supported.

Only HTTP(S), standard ports, and public destinations are allowed. Private, loopback, link-local, reserved destinations, and credential-bearing URLs are rejected. DNS addresses are checked and the connection is pinned to a checked address. HTTP API tokens are scoped to their configured fixed API provider and never attached to imported pages. The browser mode has its own isolated navigation model and does not use this HTTP transport.

## Evidence and scoring

Index results preserve the search page or API URL as provenance. They do not falsely claim that the destination page was fetched. URL imports preserve the actual page URL. Search titles/snippets, indexed URLs, page titles/descriptions, and profile descriptions are retained as observations but excluded from PES. Confirming a candidate does not convert these into verified personal facts.

Structured GitHub and Bluesky profile fields can enter scoring after review under the experimental policy. Every candidate remains separate. The current review model confirms one candidate per session and does not merge evidence across social platforms. Cross-source linkage requires a future explicit association workflow.

## Optional API providers

API keys are read only from environment variables. Never put them in project files or command arguments. Google Custom Search JSON API is closed to new customers and Google states that existing customers must migrate by January 1, 2027. The legacy adapter is optional, not a requirement for Google browser mode.

API adapters are tested with mocked HTTP responses. No live Brave or Google API search has been verified with customer credentials. No paid services are enabled or purchased by the application.

## References

- https://developer.chrome.com/docs/automation-and-testing/headless
- https://developers.google.com/custom-search/v1/overview
- https://api-dashboard.search.brave.com/app/documentation/web-search/get-started
- https://docs.bsky.app/
- https://pkg.go.dev/golang.org/x/net/html
