# P1 findings — triage against main at 4fe2fc06

Every P1 from `review-claude.md` (30) and `review-codex.md` (10), re-verified against the tree.
Each is live, fixed, or disproved with evidence; nothing is dropped on confidence. The
per-finding evidence (`file:line` quotes, SDK doc comments, siblings) is in `p1-evidence-a.md`
(F01–F20) and `p1-evidence-b.md` (F21–F40); this file is the disposition and the grouping.

## Dispositions

| # | Type | Finding | Disposition |
|---|---|---|---|
| F01, F31 | acm | Default `ListCertificates` filter | Key-type half fixed in 3.58.0; the ACME key-pair origin and the snapshot collector's key types are live → task 4 |
| F02 | apigw | REST resource policy never parsed | Fixed — `core/iampolicy/policy.go:59-75`, `apigw_issue_enrichment.go:225-236` |
| F03 | asg | asg → elb pivot emits unmatched IDs | Fixed — `asg_related.go:154-159` |
| F04 | athena | Logs pivot reads the metrics flag, synthesises a group | Live → task 6 |
| F05, F06 | backup | Tag-condition and `NotResources` selection semantics | Fixed — `backup_match.go:31-70` |
| F07 | dbc | Terminal cluster states read as in progress | Live → task 5 |
| F08 | dbi | Terminal instance states get no lifecycle finding | Live → task 5 |
| F09 | eb-rule | Custom event buses never listed | Fixed — `eb_rule.go:109-180` |
| F10 | eb-rule | Failed `ListTargetsByRule` read as "no targets" | Fixed — `eb_rule_issue_enrichment.go:89, 113-129` |
| F11, F37 | ecr | Scan results read from fields Basic Scanning no longer fills | Live → task 2 |
| F12, F13 | ecs-svc | `ListServices` unpaginated; row ID not unique across clusters | Fixed — `ecs_svc.go:37-46, 99, 119, 126-128` |
| F14 | ecs-svc | Service log view shows the oldest events | Live → task 6 |
| F15, F16 | ecs-task | Task list and task pivots | Fixed — `ecs_task.go:42-50, 157`, `ecs_task_related_extra.go:25-58` |
| F17 | ecs | `DescribeClusters` include set | Fixed — `ecs.go:43-46` |
| F18 | lambda | Status read from fields `ListFunctions` does not return | Live → task 2 |
| F19 | lambda | End-of-life runtime list is stale | Live → task 2 |
| F20 | opensearch | `DescribeDomains` sent more names than one call accepts | Live → task 4 |
| F21 | policy | Policy pivot | Fixed — `iam_policies_related.go:44-61` |
| F22 | redis | RBAC read as no auth | Fixed — `redis.go:232` |
| F23 | s3 | Cross-Region bucket reads | Fixed — `s3_cross_region.go:41-119` |
| F24 | cf | Missing-origin-bucket check never runs on a live client | Live → task 3 |
| F25, F26 | ssm | Parameter reads and secret scan | Fixed — `ssm.go:62-137` |
| F27 | trail | Log-bucket exposure judged by a second classifier | Live → task 3 |
| F28 | vpce | State compared in a spelling AWS does not send | Live → task 2 |
| F29 | waf | Association check left on the API's default resource type | Live → task 4 (also `backlog.md` row 2) |
| F30 | waf | Web ACL listing | Fixed — `waf.go:22-60` |
| F32 | cf | Only the default cache behaviour judged for TLS | Live → task 3 |
| F33 | dbc | Cluster list left on the shared RDS endpoint's default engine set | Live → task 5 |
| F34 | dbc | Pending-maintenance call scoped to DocumentDB | Disproved — the docdb client signs as `rds` against the RDS endpoint (`docdb@v1.56.0/endpoints.go:121`), so the call is not engine-scoped |
| F35 | ec2 | IPv4 and IPv6 exposure collapsed into one verdict | Live → task 3 |
| F36 | ec2 | A partial security-group list read as complete | Live → task 3 |
| F38 | eip | Classic address with no allocation ID | Disproved — EC2-Classic is retired; `Address.Domain` documents only `vpc` (`ec2@v1.335.0/types/types.go:399`) |
| F39 | eks | Self-managed and Auto Mode nodes read as a proven zero | Live → task 6 |
| F40 | glue | Snapshot collector writes job argument values unredacted | Live → task 1 |

40 entries: 17 fixed, 2 disproved, 21 live. The live ones are 19 distinct defects — F01/F31 and
F11/F37 are the same defect found by both reviewers.

## Tasks, in order

1. (#560) **The snapshot collector persists secret values.** Glue `DefaultArguments` and CodeBuild
   plaintext environment variables are written to `snapshot.json` verbatim.
2. (#561) **A value read in a shape AWS does not send.** VPC endpoint state spelling, Lambda status
   fields absent from `ListFunctions`, ECR scan results under Basic Scanning, the stale Lambda
   end-of-life runtime list.
3. (#562) **An exposure verdict judged on part of the evidence.** EC2 IPv6 exposure, a partial
   security-group list, CloudFront's non-default cache behaviours, the missing-origin-bucket
   check lost behind the S3 client wrapper, and the trail log bucket judged by a second
   classifier.
4. (#563) **A request that asks AWS for less than the screen shows.** ACM's ACME origin and the
   collector's key types, the WAF association check per resource type, OpenSearch's
   five-domain describe limit.
5. (#564) **RDS lifecycle and engine scope.** Terminal cluster and instance states, and the
   cluster list's engine set.
6. (#565) **A related pivot or child view that answers without reading.** EKS nodes outside managed
   node groups, the Athena log-group pivot, and the oldest-first log windows of the ECS service
   log view and the Lambda invocation list (`backlog.md` row 12).
