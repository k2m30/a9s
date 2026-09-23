# Related panel and navigation — triage and order

Every related-panel and navigation finding in `review-claude.md`, `review-codex.md`, the
one-liner "rejected as not small" lists and backlog rows 1, 2, 3, 5, 11 and 12, checked
against `main` at `ad18304c`. The per-type evidence is in `slice-A.md` to `slice-D.md`
(acm–eb-rule, eb–kinesis, kms–s3, secrets–waf): one row per finding with its verdict,
class and current `file:line`. 115 rows are live.

Live rows by class: MATCH 31, CONTRACT 21, CARD 19, ID 11, CALL 10, PAGE 5, NAV 5, other 13.

## Groups, in the order they are fixed

Each group fixes one shared cause, at the shared site, so the per-type fixes after it do not
rewrite the same lines. One task at a time.

1. **One ref resolver for every emitted id and navigable field** (ID, NAV) — #542. The remaining hand
   ARN splits (#542: `core/semantics/ctevent/target.go`, `sns.go`, `sns_sub.go`,
   `issue_enrichment.go`, `dbc_issue_enrichment.go`, `backup_match.go`, `eb_rule_targets.go`,
   `ecs_svc.go`), resolvers that accept refs the target list can never hold (raw launch-template
   `ImageId`, the apigw Lambda placeholder, a Classic ELB name, a bare Lambda name without a list
   check), KMS alias resolution split between the panel (`kmsRelated`) and navigation
   (`resolveNavIDs`), SSM ids built outside a resolver.
2. **One coverage rule for a pivot's count** (CARD) — #567. `anyDegraded` stops meaning "the list is a
   subset" (`core/aws/related_fetch.go:41,65`; backlog 3 and 11); checkers reading `cache[target]`
   directly go through `FetchRelatedTarget` so a fields-only cache is not a proven 0; a checker
   that read a narrower shape than the relation does not return a proven 0.
3. **One matcher per kind of join** (MATCH) — #568. One EventBridge pattern evaluator for the ecr, s3 and
   ecs-svc rule pivots; boundary-safe name and prefix matching; heuristic joins replaced by the
   field AWS states membership in (DescribeTargetHealth, IntegrationUri, CapacityProviders,
   awslogs-group, the ENI attachment).
4. **Region carried by the source** (CALL) — #569. Global-service pivots (cf → lambda@edge), a bare name
   with a side Region field (pipeline `ActionDeclaration.Region`, r53 `VPCRegion`), backlog 4
   (cross-Region lists with a nil cache) and backlog 6 (a detail opened on another Region's row).
5. **Checkers read every source the contract names** (CONTRACT, other) — #570. Per-type fixes on the
   layers above: fields a checker skips, lifecycle and enabled state not filtered (tgw, vpc, ses),
   ECS secret refs classified twice, launch source computed two ways, the two contract documents
   disagreeing (kinesis → ddb), backlog 1, 2 and 5.
6. **Per-parent reads paged** (PAGE) — #571. kinesis tags, IAM group sweep, ec2 by-id batching.
