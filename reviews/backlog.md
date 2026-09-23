# Open findings — backlog

Findings that need a behaviour decision and a QA → dev → acceptance round, kept beside
`review-claude.md` and `review-codex.md` so the whole open set is in one place.

A row leaves this file when it is fixed or disproved with evidence. "Probably fine",
"pre-existing" and "minor" are not dispositions.

Small, obvious fixes do not live here — they are done directly rather than filed.

## Related-panel and AWS reads

1. **REST APIs error on three pivots.** `core/aws/apigw_related.go:136-152` calls the
   apigatewayv2 `GetIntegrations` for v1 (REST) APIs, which answers NotFound for a REST id, so
   the Lambda, KMS and Role pivots error for every REST API in the account. Needs the v1 API's
   own call.
2. **Backup pivots ignore resource tags.** A plan that selects by tag makes the count a lower
   bound on ddb, s3, dbi-snap and dbc-snap, because the pivot never reads the resource's tags.
   Needs a per-resource tag read on detail open.
3. **A one-to-one pivot renders `(1+)`.** `ng` → eks cluster and `asg` → ng show a lower bound
   when the target list holds a degraded row, though the relationship can hold at most one row.
   Needs a per-pivot "at most one" notion.
4. **Cross-Region related lists are fetched with a nil cache** (`core/aws/related_shared.go:33`),
   so every detail open re-issues the other Region's list call. Needs a per-Region cache that
   does not let foreign rows into the session's own list for that type.
5. **`ct-events` registers no `ecr` pivot.** Over the read-only accounts, 60 of 61
   `AWS::ECR::Repository` CloudTrail records carry the repository ARN and one carries the bare
   name; the type is keyed on the ARN, so that record is reachable from neither end. Needs a
   registration with its AWS-field citation in `docs/related-resources.md`, resolving through
   `ctIDAlternatives`, which already offers both shapes.

6. **A detail opened on a row read in another Region runs against the session Region.**
   `core/runtime/detail_op.go:73` hands the detail operation `c.session.Clients`, and the list's
   Region never reaches `BeginDetailOperation` (`core/app/navigate.go:502`, `:589`). A pivot that
   drills into another Region (cf → acm in us-east-1, r53/trail/waf → logs, the alarm pivot)
   reads its list correctly through `InRegion`, but the detail of a row in it runs its enricher,
   every related checker and the CloudTrail row against the session Region: an ACM certificate is
   asked for in the wrong Region and its load-balancer and API Gateway rows error, and the
   CloudTrail row reports a confident 0. The detail operation should run on
   `c.session.Clients.InRegion(<the row's list Region>)`.

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

11. **Every alarm detail renders "EKS Clusters (0+)"** although the demo EKS fake returns no
    NextToken. Both branches report `truncated=true` for different reasons: the cache branch from
    `anyDegraded(rows)` alone, the fetch branch from a composite `DescribeCluster failed for 2 of
    12` that `FetchRelatedTarget` swallows, so the caller sees `truncated=true, err=nil` and
    cannot tell a per-item detail failure from a lost page. Excluding `anyDegraded` for an
    ID/Name-only spec fixes the cache branch alone, and a half-fix still renders `0+` whenever the
    operator has not already opened the EKS list. A correct fix has `FetchIsPartial` separate
    "some rows' details are unread" from "the list is a subset", across 22 `FetchRelatedTarget`
    and 197 `relatedResourcesFor` call sites. A spec reads beyond ID and Name exactly when
    `Values != nil || QualifierValue != nil || MetricsRegion != nil` — 10 of 35 specs do, audited
    by hand; whoever takes this should assert that `ValuesFromRawStruct` implies that predicate so
    the two cannot drift.
12. **A `dbi-snap` parent row is navigable but Enter opens nothing.** `dbiSnapParentRow` resolves
    a snapshot's parent through `DbiResourceId`, while `dbiRefToID` (`core/aws/ref_ids.go:567`)
    matches only a name the loaded list holds, so a snapshot taken before the instance was renamed
    has a row that leads nowhere. `TestRefConformance_NavigableFieldsOpenTargetRows` reproduces it
    the moment a fixture carries a pre-rename `DBInstanceIdentifier`.
## Structure

13. **Three copies of the ECS client-assertion and retry plumbing** remain around the one
    `DescribeTaskDefinition` read; `core/aws/related_common.go:233` is where they would collapse.
    They map "no client" and "refused" to different results per row, so collapsing them needs a
    ruling on that mapping. No behavioural difference today.
