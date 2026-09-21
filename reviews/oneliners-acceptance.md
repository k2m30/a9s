# Tier 1 + Tier 2 — one sitting, after t548b lands

Agreed with the user 2026-09-21: the whole list below is done in one batch, not deferred row by row. Tier 3 (11 rows) stays in `backlog.md` for the loop.

Each row lands with a test where behaviour changes, a fixture where the demo changes, and a doc line where the contract changes. Gates once at the end, plus `make integration` (fixtures move) and `make mdlint` (docs move).

## Tier 1 — one-liners

1. `core/aws/kms.go:190,209` — the alias-map failure joins the per-ID list, so one key renders `kms FetchByIDs failed for 2 of 1 IDs`. Count it separately.
2. `ecs_svc_logs` default detail renders `IngestionTime` as raw epoch ms; read the formatted `ingestion_time` key as `cb_build_logs` and `log_events` do.
3. CloudTrail `ORIGIN` renders `?` for a record with no `userAgent`; render nothing.
4. `dbi-snap` default detail omits `SourceDBSnapshotIdentifier`, so a copy shows DB Instances (0) beside a parent identifier with nothing saying it is a copy. Add the field, regenerate views.
5. A log group named exactly `/aws/cloudtrail` is not treated as an audit group (`HasPrefix "/aws/cloudtrail/"`). Prefix-or-equal.
6. `tests/unit/ct_events_demo_cache_lookup_test.go` — one failure message expects a resolved Count=0 where the two tests below expect Unknown. Align to Unknown.
7. The forbidden-helper guard over `tests/` cannot see a nil client after `context.Background()` (`[^)]*` stops at the `)`). Anchor on argument position and fix the four calls it then reports.
8. KMS pivot label "IAM Roles (grants)" also lists key-policy roles (`docs/resources/kms.md` §2). Relabel with the contract line.
9. Backup pivot: `docs/related-resources.md:347,513` say "recovery points" for a row that lists plans, and the short label is "Backup (N)" on ebs/ebs-snap/s3 and "Backup Plans (N)" on ddb/efs/dbi-snap/dbc-snap. One label, one contract.
10. A CloudTrail event acting on several subjects joins and truncates them in the TARGET cell. Name one subject and say how many more.
11. `TestEveryTypeDeclaresOneStatusColumn` admits a future type titling a column "State" over a raw AWS enum beside a declared status column. Tighten the rule.
12. The demo pipeline declares a CodeArtifact source provider that does not exist in AWS.
13. The demo EC2 fake ignores `DescribeNetworkInterfaces` filters, so an ENI drill lists every ENI in the account — the same defect dev fixed for `DescribeTransitGatewayAttachments`.

## Tier 2 — small, fixture or decision first

14. Demo alarm naming an ASG only through a scaling-policy action (no `AutoScalingGroupName` dimension); alarm pin moves.
15. Demo fixtures for an `ebs-snap` CopySnapshot row, a `DbiResourceId`-carrying `dbi-snap`, and a CreateImage snapshot whose instance is gone.
16. Demo backup plan whose selections were only partly read, for the join half of #551's row 4.
17. `acme-docdb-prod-01` is dbi row 52, past `EnrichmentCap=50`; place it inside the cap without pushing another witness out.
18. Demo log groups for the web-frontend and order-worker task-definition families, so more than one ECS service shows a count.
19. Every alarm detail renders "EKS Clusters (0+)" although the demo EKS fake returns no NextToken. Root-cause why `relatedResourcesFor(..., "eks")` reports truncated, then fix.
20. 25 demo policy rows state an `attachment_count` no principal backs. Derive from the attachment maps; `orphanUnattachedPolicyFinding` will fire on the unattached ones, so the badge changes belong to this change.
