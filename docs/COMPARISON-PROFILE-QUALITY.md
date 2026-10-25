# Profile extraction validation, 24 October 2026

This check compares observed failures against `relations-v5` and `public-page-v4`. It is a small regression check, not a held-out accuracy estimate or a claim of conference readiness. Snapshots remain local under `data/quality-20261024/`.

## Same observations, different extraction

A new web search for Daphne Koller produced five candidates. The same `daphne-web.json` observations were replayed through the saved pre-change executable, the updated rules, and updated rules plus local `qwen2.5:7b`.

| Mode | Candidates | Company summaries | Role summaries |
| --- | ---: | ---: | ---: |
| Previous rules | 5 | 0 | 4 |
| Updated rules | 5 | 2 | 4 |
| Updated rules plus Qwen | 5 | 2 | 4 |

The two recovered companies are the source claim `insitro`. The Google Scholar snippet states `CEO and Founder, insitro`; the World Summit AI snippet states `CEO and Founder of insitro` followed by a descriptive clause. Previously, the lowercase employer and comma handling prevented employment extraction. Updated rules retain `CEO and Founder` as the role and `insitro` as the employer. These remain unverified source claims.

Qwen produced eight proposals that passed validation and two rejections. Passing validation does not mean operator acceptance or eight new facts; some proposals duplicate rules. Two additional candidates have pending model employment proposals. Their company summaries remain empty until relationship review. This run does not show a summary advantage from enabling Ollama.

Artifacts: `daphne-before.json`, `daphne-after.json`, and `daphne-hybrid.json`.

## Collection loss before extraction

The public Yohan Muliono profile was fetched before and after the change. Both responses have SHA-256 `d88a238bb2461abfbf040dd04452ee39ea1166b879e62c3ae3f5c9957e5bfc2b`, so the comparison uses identical HTML bytes.

Previously, removing every HTML header reduced the page text to 212 characters and left the direct-URL role empty. The updated collector retains the profile header, yielding 485 characters and the headline role `Head of Cyber Security Program` without Ollama. Company remains empty: a site title alone does not establish employment. Qwen's organization proposal was rejected because the selected passage did not contain that organization.

Artifacts: `yohan-before.json`, `yohan-after.json`, and `yohan-hybrid.json`. Existing snapshots with discarded header text require recollection to recover it.

## Regression checks

`go test ./...` and `go vet ./...` pass, and the CLI builds. The known synthetic relation corpus contains 19 cases with 13 expected relationships. Rules retain all 13, with zero extra relationships, wrong-subject assignments, or period errors on this corpus. Seven cases correctly abstain. These are development regressions, not general-web accuracy figures.

Additional tests cover profile headers, hidden content, adjacent profile fields, lowercase employers, descriptive commas, company placeholders, unrelated people, sponsor labels, and page-local JSON-LD references. Unresolved or ambiguous JSON-LD IDs do not supply an employer; cycles and traversal are bounded. No remote schema references are fetched.

Remaining limitations include unlabelled layouts, dynamic page sections, organization context split across observations, index availability, and model proposals requiring review. Cross-page company attribution and automatic account ownership resolution are not implemented by this change.
