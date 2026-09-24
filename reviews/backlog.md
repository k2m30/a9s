# Open findings — backlog

Findings that need a behaviour decision and a QA → dev → acceptance round, kept beside
`review-claude.md` and `review-codex.md` so the whole open set is in one place.

A row leaves this file when it is fixed or disproved with evidence. "Probably fine",
"pre-existing" and "minor" are not dispositions.

Small, obvious fixes do not live here — they are done directly rather than filed.

## Related-panel and AWS reads

1. Closed by #570.
2. Closed by #570.
3. Closed by #567 (`cad809f0`); the number is kept because issues cite rows 4–6 by number.
4. Closed by #569.
5. Closed by #570.

6. Closed by #569.

## Rendering and enrichment

7. **A metric-math or anomaly-detection alarm never shows what it watches.** The detail renders
   `MetricName: -`, `Namespace: -`, `Dimensions: -` and no `Metrics[]`, and the list's Metric and
   Namespace cells are empty. Needs the metric queries rendered, with the list columns falling
   back to them.
8. **The lazy-add drill hides an unreadable id.** A drill showing `policy(1)` under
   `IAM Policies (2)` gives no sign on the list that one id could not be read — it appears only
   in the `!` log.
9. **Wave-2 account walks re-run whole enrichers on a throttle.** ebs, ec2 and dbi should fold
   walk errors through `FailedOnPage` + `AggregateFailures` and wrap `next` in `RetryOnThrottle`
   as dbc does; today ec2 repeats up to 50 `DescribeInstanceAttribute` calls and the `!` log
   shows raw SDK text.
10. **An unparseable policy reads as safe.** Wave-1 OpenSearch access policies and IAM role trust
    policies that fail to parse render as not-public / not-wildcard with no mark
    (`core/aws/opensearch.go:86-88`, `core/aws/iam_roles.go:50-53`). Needs a ruling on what a
    Wave-1 row shows for a policy it could not read. AWS validates both on write, so there is no
    operator witness today.

11. Closed by #567.
12. **A `dbi-snap` parent row is navigable but Enter opens nothing.** `dbiSnapParentRow` resolves
    a snapshot's parent through `DbiResourceId`, while `dbiRefToID` (`core/aws/ref_ids.go:567`)
    matches only a name the loaded list holds, so a snapshot taken before the instance was renamed
    has a row that leads nowhere. `TestRefConformance_NavigableFieldsOpenTargetRows` reproduces it
    the moment a fixture carries a pre-rename `DBInstanceIdentifier`.
13. **A views file written by an older build never gains a new detail field.** `EnsureViewsDir`
    (`core/config/ensure_views.go:75-100`) merges new columns into an existing
    `~/.a9s/views/<type>.yaml` but not new `detail:` entries, so a field a later build adds to a
    detail view (for example `error_type` and `restore_duration_ms` on Lambda invocations) never
    reaches that operator's detail until the file is regenerated. Applies to every type.

## Structure

14. **Three copies of the ECS client-assertion and retry plumbing** remain around the one
    `DescribeTaskDefinition` read; `core/aws/related_common.go:233` is where they would collapse.
    They map "no client" and "refused" to different results per row, so collapsing them needs a
    ruling on that mapping. No behavioural difference today.
