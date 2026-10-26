# International collection and extraction evaluation

See the [completed 26 October 2026 results](MATRIX-RESULTS-2026-10-26.md) for the 800-output primary run, source audit, subsequent fixes, and remaining limits.

The public matrix contains 200 distinct people and four modes per person: web collection with rules, collection with a supplied account and rules, and local Qwen replay of each collection. Model comparisons use identical observed evidence within each pair. They do not repeat a web search and attribute changing search results to the model.

The corpus contains five occupation groups and account clues from Instagram, X, Facebook, and GitHub. Names, article references, and account statement identifiers were frozen from public Wikipedia and Wikidata responses before collection. Forty people form the development set; 160 form the heldout set. The split uses a deterministic hash of the reference identifier.

This is an operational evaluation with a deliberately limited reference corpus. It is not an accuracy certification. All selected people have Wikipedia articles and usable public account statements, so the Wikipedia fallback has a substantial selection advantage. Results do not establish performance for people with sparse public information, private profiles, unlisted accounts, or every supported platform. Reference accounts may be outdated, and reference article matches do not prove account ownership.

## Run and resume

Install Go and Python, build the CLI, start the configured SearXNG service, and make the chosen model available in local Ollama. The runner does not download a model or use a paid API.

```powershell
go build -o bin/pmiyc.exe ./cmd/pmiyc
powershell -File scripts/start-local-search.ps1
python scripts/benchmark_matrix.py --binary bin/pmiyc.exe --out data/public-matrix-run --collection-gap 6
```

Run the same command to resume. Each completed output has a checkpoint and SHA256. The runner refuses changed snapshots, binaries, corpus, model digest, configuration, or runner code in the same run directory. Use a new directory after changing the implementation. Keep the executable used for an unfinished run.

Collection uses the limits in `configs/collector-local-search.json`: five candidates, 20 requests, a 15-second request timeout, and a 90-second session timeout. Two collection workers start at least six seconds apart. One model worker uses `qwen2.5:7b`, at most five local requests per snapshot, and a 30-second timeout per model request. Provider restrictions remain visible in diagnostics; the runner does not bypass challenges or disable TLS validation.

```powershell
python scripts/benchmark_matrix.py --out data/public-matrix-run --report-only --split development
python scripts/report_matrix.py --run data/public-matrix-run --out benchmarks/results/public-matrix.json
python -m unittest discover -s scripts -p test_benchmark_matrix.py
```

The final report command refuses an incomplete matrix or changed snapshots, recomputes checkpoint metrics, and verifies that rules and model pairs retain exactly the same observed evidence and source metadata. JSON and CSV outputs contain per-case metrics and snapshot hashes. Full snapshots and logs stay under ignored `data/` directories.

To construct a new public reference corpus, use a new output path:

```powershell
python scripts/build_public_matrix_corpus.py --out fixtures/public-matrix-next.json --count 200
```

The builder caches reference responses locally, excludes ambiguous/non-person records and unusable account values, and records exclusions. The selected Wikidata statements are clues, not independently verified identity or employment labels. Reference APIs: [MediaWiki Search](https://www.mediawiki.org/wiki/API:Search), [TextExtracts](https://www.mediawiki.org/wiki/Extension:TextExtracts), and [Wikidata data access](https://www.wikidata.org/wiki/Help:Data_access).

## What the metrics mean

- `with_candidates`: at least one candidate was collected. It says nothing about identity correctness.
- `reference_article_seen`: a candidate or evidence source matches the frozen article URL. It is a retrieval agreement metric.
- `with_role`, `with_company`, and `with_education`: at least one candidate has that summary field or claim. Missing employers can be correct abstentions. Filled fields can still be wrong.
- `model_proposals`: additional relationships that survive grounding checks. They remain unverified and require operator review before entering the summary.
- `model_requests_completed`: attempted local extractions with a terminal trace, including failures and rejected outputs. Skipped candidates are counted separately in the status histogram.
- `grounding_reference_errors`: structural errors in evidence references or literal quotes. Zero does not imply semantic correctness.
- `unexpected_identity_decisions`: candidates that were automatically moved outside the pending/review state. The expected count is zero.

Rules and Ollama summaries can therefore be identical while Ollama adds useful pending relationships. The useful comparison is the additional proposals and their source-level correctness, not a count of filled fields alone. The evaluation does not silently accept model output to improve coverage numbers.

## Repairs driven by development cases

Account searches previously admitted other people's posts merely mentioning the supplied handle. Platform-scoped handle queries now require the account's URL path; unscoped research still admits third-party references. Instagram account views are deduplicated without collapsing individual posts. Account queries leave room for a general name search, including mononyms, and early link following no longer consumes that space.

SearXNG responses with no results and unavailable engines now report failure and expose bounded upstream diagnostics instead of appearing to be healthy empty results. The collector stops re-querying that failed provider in the same discovery session. An exact-title English Wikipedia fallback uses the public Action API, keeps response provenance, and abstains on missing/disambiguation articles. It does not establish that an article is the intended person.

Extraction now handles bounded citation markers and nested introductory parentheses, separates occupations from subordinate narrative clauses, and preserves the main relationship's tense. Page labels such as `Player profile` are rejected as roles. Sports team fields exclude a following league description, and balanced organization parentheses are preserved. Former aliases no longer override a proved model relationship's tense.

An article heading can bind an expanded name in its opening birth introduction when the shorter multiword name is an ordered subset. This is restricted to the same English Wikipedia article, with separate title and body evidence references. It cannot merge profiles, infer arbitrary aliases, or establish a legal name. The bound relationship is revalidated on load.

## Remaining limits

Source text can still be outdated, wrong, ambiguous, truncated, or unavailable. Arbitrary aliases, narrative pronouns, multiple nationalities and occupations, complex education prose, and multilingual phrasing remain incomplete. The model's literal-citation checks do not fully establish semantic attribution. Employer completeness must not be inferred from a corpus dominated by performers, writers, and athletes.

The dated result report records the completed run, development comparisons, failures, and exact implementation/model hashes. Additional semantic claims need independently adjudicated labels and a separate corpus of less prominent public profiles before making broader performance claims.

## Replay after an audit fix

Keep the primary outputs unchanged. Re-run deterministic extraction into a new directory and compare identical observations:

```powershell
python scripts/replay_matrix_rules.py --run data/public-matrix-run --binary bin/pmiyc.exe --out data/public-matrix-rules-replay
$env:PMIYC_MATRIX_REPLAY_INPUT = Join-Path (Get-Location) 'data/public-matrix-run'
$env:PMIYC_MATRIX_REPLAY_OUTPUT = Join-Path (Get-Location) 'data/public-matrix-proposal-replay.json'
go test ./internal/extract -run '^TestMatrixProposalReplay$' -v
Remove-Item Env:PMIYC_MATRIX_REPLAY_INPUT, Env:PMIYC_MATRIX_REPLAY_OUTPUT
```

The optional proposal test revalidates previously accepted model claims using the original evidence and extraction time. It does not rerun Ollama or recover rejected raw model outputs, so it cannot measure recall. `scripts/report_matrix_replay.py` publishes these results separately from the primary run. Source-audit sampling and operational recovery are available through `scripts/sample_matrix_audit.py` and `scripts/retry_matrix_models.py`; each requires a new output location.
