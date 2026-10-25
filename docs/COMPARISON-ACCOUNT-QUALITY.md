# Account clues and extraction validation, 24 October 2026

This change adds scoped account inputs, direct public-page attempts before indexes, and targeted follow-up questions. Extraction uses `relations-v6`. Artifacts are local under `data/quality-next-20261024/`; snapshots are not committed.

## Account collection

The live command used `-name Haikal -account instagram:@haikal_satrio -account github:HaikalSatrio`. It collected two candidates and six observations within ten requests. Both specified pages supplied matching public metadata. The candidates stayed separate, PENDING, and at identity confidence zero. No employment claim or shared ownership was inferred from supplied handles. Index failures did not erase the collected pages.

The earlier Instagram index-only run collected no matching candidates. This is evidence that direct hints can recover a page missed by indexes, not a general recall benchmark. Only Instagram and GitHub were exercised live. Other supported route templates require their own live reliability checks and may encounter login requirements or unavailable metadata.

Tests cover platform-specific queries, duplicate hints, redirects to other accounts, generic and unavailable-account titles, private URLs, cancellation, shared budgets, provenance merging, account-only inputs, and session refinement without mutating the earlier snapshot.

## Three new names

Tracy Chou, Mitchell Hashimoto, and Kelsey Hightower were collected before making the version 6 extraction changes. Each search produced five candidates. The same saved observations were replayed after fixing failures discovered in them.

| Case | Before | After |
| --- | --- | --- |
| Tracy Chou | `Executive Woman Media` incorrectly appeared among roles | Publisher name removed; supported occupation and founder/CEO claims retained |
| Mitchell Hashimoto | No company or role summaries | Personal-page claim `co-founder / Superlogical` recovered on one candidate |
| Kelsey Hightower | Employer included `working on Google's Cloud Platform`, with quote variants treated as competing companies | Employer boundary reduced to `Google` on that candidate |

The Kelsey page uses present-tense employment wording that may be stale. The result is a source claim, not a finding about current employment. Version 6 explicitly explains this distinction. Competing people with the same name remain candidates for identity review.

These names exposed bugs and were then used to fix them, so the final comparison is a development regression, not a held-out accuracy estimate. Numerous candidate fields remain empty; cross-page attribution and unlabeled layouts are still incomplete.

## Ollama comparison

The Mitchell snapshot was also processed with local `qwen2.5:7b` using five model requests. The final run retained the rules-derived employer and role, but all seven model proposals failed validation. Errors included invented employment years, joined role titles, and treating a governing body as an individual's role. Two candidates returned no proposals. Prioritizing a shorter personal introduction did not improve acceptance in this run.

The validator was not loosened to accept these proposals. Ollama is optional and is not automatically more accurate than rules. Its proposals still require relationship review. The negative result is retained in `mitchell-hybrid-focused.json` alongside the earlier run.

## Follow-up questions

Search output suggests up to three follow-ups based on unresolved evidence. `pmiyc next -input <snapshot>` recomputes the questions without fetching data; `-out <new-json>` saves question codes and affected IDs. The questions distinguish missing public profile clues, identity review, model relationship review, missing employer evidence, and unresolved periods. Rejected candidates do not trigger relationship acceptance requests. No answers are guessed and no decisions are changed.

This is a guided CLI workflow using the existing search/review commands, not a conversational stdin interface. The current identity-review model still selects one primary candidate per search session; it does not construct a confirmed cross-platform person graph.

## Validation and remaining work

The known synthetic regression corpus now has 23 cases and 15 expected relationships. All 15 are retained with no extra relationships, wrong-subject assignments, or period errors in this corpus; nine cases abstain. Additional collector, session, parser, and follow-up tests pass. These results do not imply comparable accuracy on arbitrary websites.

Remaining work includes independent annotated evaluation, dynamic content extraction where publicly accessible, dated employment corroboration, broader language/layout support, and explicit evidence linking accounts across platforms. Public sources and search services can be incomplete, inaccessible, contradictory, or out of date. This implementation does not claim perfection or conference readiness.
