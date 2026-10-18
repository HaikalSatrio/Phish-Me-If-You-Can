# Direct collection and observation changes

This experimental collection path uses public HTML, without API credentials or a browser. It implements two adapters: GitHub and DEV Community. Their rules were designed from public HTML samples. Live end-to-end reliability and automated regression coverage for these new adapters have not yet been established.

## Usage

Run from the repository root. Use an account you own or are authorized to assess.

```powershell
.\bin\pmiyc.exe adapters
.\bin\pmiyc.exe search -source direct -username "HaikalSatrio" -platform github
.\bin\pmiyc.exe search -source direct -username "YOUR_USERNAME" -platform all
```

New searches supplying a username default to direct HTML collection. Existing saved sessions retain their original source. `-platform all` in direct mode means only the adapters listed by `adapters`. The larger `platforms` registry describes search-index discovery, not implemented direct adapters. Instagram, Facebook, and LinkedIn do not yet have direct adapters.

The `github` source remains the existing optional API collector. Use `-source direct -platform github` for HTML without an API. Name-only discovery still uses the separate web path and retains its known limitations.

## Detection rules

GitHub requires matching canonical and Open Graph profile URLs, profile metadata, and the visible exact username in the expected profile element. DEV requires matching canonical and Open Graph URLs, a profile heading, site metadata, and a profile title marker. HTTP 200 alone is insufficient.

Structured display names and usernames become observed fields. Biography and page title remain metadata excluded from PES. No employment, personality, relationships, or interests are inferred from biography text. Identity confirmation remains an explicit operator decision. Matching usernames on two sites do not automatically merge identities.

Each request respects the configured timeout and session budget. Redirects are not followed in direct mode because they may lead to another username or a login page. Explicit URL mode has its separate bounded redirect policy. There are no challenge-solving or authentication steps in the direct adapter.

## Collection receipts

Every selected adapter produces a receipt containing its requested URL, rule version, timestamp, status, and reason. HTTP 200 response bodies receive a SHA-256 digest and byte count, including bodies rejected by the parser. Non-200 bodies are not retained or hashed. Successful evidence references the matching response digest and rule.

Receipts distinguish SUCCESS, EMPTY (HTTP 404), BLOCKED, FAILED, and NOT_CHECKED. A 404 describes the requested URL only. A challenge, timeout, changed layout, or budget stop does not establish account absence.

Response hashes identify bytes, not authenticity. They are not digital signatures, ownership proofs, or independent verification of profile claims. Raw HTML is not archived by this command. Capture timestamps are request-start times; evidence timestamps are generated when the collection result is assembled.

## Compare observations

Capture two snapshots with the same username, platform selection, clues, and collector configuration. Choose filenames that do not already exist.

```powershell
.\bin\pmiyc.exe search -source direct -username "HaikalSatrio" -platform github -out data/before.json
# Later, collect another snapshot after reviewing or changing your public profile.
.\bin\pmiyc.exe search -source direct -username "HaikalSatrio" -platform github -out data/after.json
.\bin\pmiyc.exe compare -before data/before.json -after data/after.json -out data/changes.json
```

Comparison runs offline. It checks account URLs and extraction rule versions, reports evidence IDs and values, and includes receipts and hashes of the normalized JSON snapshots. The report is deterministic for the same inputs.

Both observations must be successful under the same rule before fields are compared. Fields can be UNCHANGED, CHANGED, NEWLY_OBSERVED, or NOT_OBSERVED. If either collection was blocked, missing, failed, unattempted, or used a different rule, the account is INCOMPARABLE. An older after-snapshot is rejected.

NOT_OBSERVED means the extractor did not observe that field in the newer successful page. It is not proof of deletion. The report does not claim remediation success or calculate a PES reduction. Different response hashes with unchanged extracted values may simply reflect dynamic page content.

## Research positioning

This stage provides evidence-backed collection decisions and comparisons that preserve uncertainty about access and extraction. These are implemented capabilities, not a claim of novelty over other OSINT tools or readiness for conference submission. Benchmarking, false-positive measurement, adapter regression tests, broader platform coverage, and a reproducible offline demonstration remain future validation work.
