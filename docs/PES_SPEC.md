# Experimental PES v1

All five dimensions are in [0, 1]. The aggregate is displayed in [0, 100]. This policy is a development fixture, not a validated scientific scale. Equal weights and the configured thresholds are experimental defaults only.

## Eligibility

Only OBSERVED facts with positive confidence and ACCEPTED candidate linkage participate. Unknown, inferred, rejected, unresolved, and conflicting workplace/role facts are excluded and listed in the report. Confidence values are heuristic annotations, not probabilities.

## Rubrics

| Dimension | Formula |
| --- | --- |
| Discoverability | Mean of supplied discoverability multiplied by evidence confidence over eligible distinct facts |
| Linkability | Mean of candidate confidence multiplied by evidence confidence over eligible distinct facts |
| Contactability | 1 when an eligible configured contact type exists, otherwise 0 |
| Context richness | Number of represented configured context types divided by number of configured context types |
| Scenario support | Maximum satisfied-prerequisite fraction among configured scenarios |

Linkability is currently an identity association proxy. It does not measure independently calibrated cross-platform linkage. Contactability and prerequisite matching are categorical presence checks. Partial prerequisite completion does not mean a scenario is feasible. Each scenario lists its missing types and supporting evidence IDs.

The aggregation is the weighted mean of dimensions multiplied by 100 and rounded to two decimal places. The policy requires all five weight keys and a positive weight sum. Calculation is a pure function of the input case and policy.

## Missingness and coverage

Coverage is the proportion of declared sources with SUCCESS or EMPTY status. FAILED, BLOCKED, and NOT_CHECKED remain incomplete. No declared sources gives zero coverage. Coverage is a scope indicator, not an estimate of the fraction of all public information discovered.

If there are no eligible facts or coverage is below the policy minimum, PES is null and status is INSUFFICIENT_EVIDENCE. Dimension values still describe the observed subset; zero is not a claim that the person has no exposure. The accepted report status is EXPERIMENTAL.

Duplicate facts use the minimum confidence and maximum discoverability of their observations. Identical duplicate records do not increase scores. The rule preserves source distribution without treating it as independent corroboration.

## Policy changes

Change the version whenever thresholds, weights, category lists, or scenario requirements change. Keep the exact policy with experiment artifacts. Rubric changes in Go require a corresponding policy version update and regression tests. Never compare scores from different policies as measured privacy improvement.
