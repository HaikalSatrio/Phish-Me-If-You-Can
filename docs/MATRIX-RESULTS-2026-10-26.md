# Public matrix results, 26 October 2026

The international matrix completed 200 people in four modes, producing 800 saved outputs. The run found both retrieval gaps and incorrect relationship extraction. Repairs improved coverage, but employer and education extraction remain weak. These results do not establish production accuracy or readiness for an Arsenal demonstration without further evaluation.

## Scope and controls

The frozen corpus contains 200 distinct people across acting (41), music (41), science/technology (36), sport (41), and writing/public education (41). Account clues cover Instagram (62), Facebook (62), X (61), and GitHub (15). References contain 53 distinct country identifiers. Forty names were used for development and 160 were initially held out. See [methodology and commands](MATRIX-EVALUATION.md).

All people were selected through Wikipedia articles and public Wikidata account statements. This favors prominent people and the new Wikipedia fallback. It does not represent people with sparse public information, private accounts, or all supported account adapters. A supplied account is a clue, not verified ownership.

For each name, web and account-assisted collection ran separately. Ollama then processed exactly the same observations as its corresponding rules run. The report verifier checked all 800 snapshot hashes, account inputs, recomputed metrics, and equality of paired observations and source metadata.

The primary run used Qwen2.5:7b, one model worker, 30 seconds per model request, and at most five requests per snapshot. Collection used two workers, six seconds between starts, and the existing 20-request/90-second session limits. Of 400 collections, 116 were preserved from earlier iterations with explicit provenance and replayed through the frozen primary extractor; 284 were newly collected with that executable. Services stopped during the interrupted session and were restarted before completion. Collection timing and upstream availability are therefore not controlled between iterations.

## Frozen primary run

Counts below are people with at least one populated candidate or summary field, not correct identities or accurate facts.

| Mode, 200 cases each | Candidates | Role | Company | Education | Extra model proposals |
| --- | ---: | ---: | ---: | ---: | ---: |
| Web + rules | 196 | 120 | 5 | 0 | 0 |
| Account + rules | 197 | 120 | 4 | 1 | 0 |
| Web + Ollama | 196 | 120 | 5 | 0 | 58 across 44 cases |
| Account + Ollama | 197 | 120 | 4 | 1 | 77 across 53 cases |

Ollama proposals remain unverified and do not automatically enter the summary. Identical summary counts are therefore expected. The additional proposals need review for attribution, relationship type, and time before they can be treated as useful facts.

Every collection had `PARTIAL` status, including cases with no candidates. There were zero runner errors, structural quotation/reference errors, or automatic identity decisions. Those checks do not establish semantic correctness. The reference article appeared in 196/200 cases in each collection arm.

The model attempted 909 requests: 409 for web and 500 for account-assisted inputs. Seven traces failed across four snapshots: three timeouts and four service-unavailable errors. A separate retry with unchanged model, binary, budget, and inputs recovered the four unavailable traces in one snapshot. The three timeout cases still failed. The original failures remain in the primary report.

Initially heldout results were 156/160 cases with candidates for web and 157/160 with account clues, with role summaries in 93/160 for both. Company counts were 4 and 3 respectively; education was zero in both. These are coverage measurements only.

## Source audit and subsequent fixes

A hash-selected, stratified sample of 37 claims was reviewed against its quoted text: 17 deterministic claims and 20 model proposals. The source audit assessed 29 as supported, five as partially supported, and three as unsupported. Deterministic claims had 14 supported, two partial, and one unsupported; model proposals had 15 supported, three partial, and two unsupported. This is a single-reviewer spot audit, not independent adjudication, full-corpus precision, or real-world fact verification.

Concrete failures included a performance description being interpreted as an employer, a later former position incorrectly dating the main occupation, `plays as` being treated as a job title, and software co-creation being classified as education. A claim can quote literal text and still be wrong.

The subsequent `relations-v11` changes:

- Stop profession parsing before descriptions introduced by known/famous/renowned for, and exclude parenthetical location descriptions from organization names.
- Keep the first occupation's tense separate from a later former occupation.
- Reject playing/working verbs as job titles and require explicit study or qualification language for otherwise unproved education proposals.
- Scope unproved model periods to sentences containing the proposed fields. Allow repeated statements with compatible periods, sentence-ending punctuation, and Indonesian education phrasing.

All 400 primary collection snapshots were replayed through the updated rules with unchanged observed evidence. Both collection arms now have role summaries in 122/200 cases and company summaries in 5/200; education remains 0 for web and 1 for account-assisted collection. The initially heldout subsets have role summaries in 95/160. Because these fixes followed inspection of the audit, these replay results are no longer an untouched heldout evaluation.

The updated validator also rechecked all 135 previously accepted model proposals against their original evidence and timestamps. It retained 118 and rejected 17: eleven unsupported education proposals, four with unresolved sentence scope, and two verb-as-role proposals. These are validator decisions, not seventeen independently proven errors. Previously rejected raw proposals were unavailable, so this replay cannot measure model recall. Four fresh model runs on the already-inspected Viola Davis and Simon Willison inputs completed without failed traces; they are an integration smoke test, not a second full live matrix.

## What improved relative to the starting code

On the 40 development names, the starting live run found candidates in 21 web cases and 37 account-assisted cases; the repaired collection found candidates in all 40 for both arms. Role coverage rose from 8/40 and 6/40 to 27/40. Different collection times and provider failures prevent attributing all of that gain to code changes.

To isolate extraction changes, the final binary also replayed the original 80 development collection snapshots without additional network or model requests. Role coverage rose from 8 to 13 web cases and from 6 to 11 account-assisted cases. Each arm gained one company case. This is a controlled comparison of extraction over the same evidence, still measuring populated fields rather than accuracy.

Retrieval repairs reserve request budget for name research alongside supplied accounts, reject unrelated account-owner paths, deduplicate account views without discarding individual posts, report unavailable SearXNG engines accurately, and add bounded exact-title Wikipedia retrieval. No paid API, challenge bypass, or disabled TLS verification was introduced.

## Validation and remaining work

The final implementation passes `go test ./...`, `go vet ./...`, the three Python runner tests, and Python script compilation. The existing 24-case synthetic benchmark still has claim TP=14, FP=0, FN=2 and correct abstentions=11/11. Narrative pronouns and education in prose remain missing in that fixture; this is regression evidence, not independent accuracy.

Company fields in only five of 200 cases and near-empty education coverage remain insufficient for a general people-research tool. Some missing employers are legitimate, but this corpus has no exhaustive employer labels to distinguish absence from extraction failure. Pending model claims can still have wrong temporal interpretation, and sources themselves can be outdated or incorrect. Account ownership and legal identity remain unverified.

Next evaluation should use a separately labeled corpus that includes less prominent public profiles, multiple languages, alias ambiguity, current versus historical roles, and explicit unknown/absent labels. Evaluate claim precision and recall separately from retrieval coverage, and calibrate the operator's identity challenge with evidence that distinguishes candidates. The existing corpus has now informed fixes and must not be advertised as an untouched final acceptance set.

## Artifacts and implementation identifiers

- [Primary JSON](../benchmarks/results/2026-10-26-public-matrix.json) and [per-case CSV](../benchmarks/results/2026-10-26-public-matrix.csv).
- [Source audit](../benchmarks/results/2026-10-26-source-audit.json), with labels, reasons, source URLs, and quotation/snapshot hashes. Full quotations remain in local ignored snapshots.
- [Post-audit rules replay, model revalidation, recovery, and smoke results](../benchmarks/results/2026-10-26-matrix-post-audit.json).

| Component | SHA256 or model digest |
| --- | --- |
| Corpus | `26bca8b7691cec556e9f3a6dac246798f5b99dd288a6679378b51fc23f9db046` |
| Frozen primary executable | `3a883f1019e14a2770c5eb7ca3a06edf4a27cbabb2f8e9e8aa193324364ed2b6` |
| Post-audit executable evaluated in this report | `649c32c18f2eacc3af3a62d3a61d5f483d7ea863c7238ed7ff04d7ebb561be65` |
| Qwen2.5:7b | `845dbda0ea48ed749caafd9e6037047aa19acfcfd82e704d7ca97d631a0b697e` |

Full collection provenance, configuration and runner hashes are in the primary report. The post-audit artifact links to that report by hash and keeps the later evaluation separate.
