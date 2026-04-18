# Enrichment UX Verification Table

Source of truth for the signal itself: [docs/attention-signals.md](./attention-signals.md)

Source of truth for the current Wave 2 implementation shape:

- `internal/aws/enrichment.go`
- `internal/tui/views/resourcelist.go`
- `internal/tui/views/detail_fields.go`
- `internal/config/defaults_*.go`

## Goal

Verify, per resource type, whether opening the top-level list makes it clear
what is actually wrong or worth attention.

Current implementation usually gives one of these:

- row color only
- menu badge + list marker + banner
- detail-only `Background Check` section after pressing `d`

That is not enough when the list tells the user "this row needs attention"
but does not tell them why.

## Decision Rules

1. Prefer a YAML/default-view column when the signal already exists in the
   fetched `RawStruct` or `Resource.Fields`.
2. If the signal only exists as a Wave 2 `EnrichmentFinding`, a YAML-only fix
   is not possible. In that case the right fix is a synthetic list field/column
   such as `Check` or `Issue`, backed by `finding.Summary`.
3. The detail-view `Background Check` section should remain, but only as the
   secondary surface. It is not enough as the primary explanation path.
4. Row color alone never satisfies "what is wrong?"

## Recommended Shared Pattern

For real Wave 2 enrichers, the cheapest reusable fix is:

- add a synthetic per-row field such as `check` or `issue_summary`
- populate it from `EnrichmentFinding.Summary` when a finding exists for that row
- expose it through normal view config as a list column

That keeps the UI data-driven:

- YAML/default-view opt-in per resource type
- one generic implementation in list/update code
- detail view still shows the full `Background Check` section

Example desired column:

```yaml
list:
  Check:
    key: issue_summary
    width: 32
```

## A. Fail: Real Wave 2 Findings Exist, But The List Does Not Explain Them

These types use real Wave 2 enrichers. Today the user mostly gets color,
marker, badge, banner, and detail drill-in. The missing piece is a short,
human-readable reason in the list itself.

| Type | Golden signal worth surfacing | How it is done now | Verdict | How it should be | Cheapest viable path |
|---|---|---|---|---|---|
| `ec2` | failed status checks, scheduled retirement/reboot | `State` column + marker/banner/detail | FAIL | short list reason like `status checks impaired` / `scheduled retirement` | synthetic `Check` column from finding summary |
| `dbi` / `rds` / `dbc` | pending maintenance | `Status` column stays healthy; only marker/detail explains it | FAIL | list should show maintenance summary | synthetic `Maintenance` or `Check` column |
| `ebs` | volume health impaired / warning / events | `State` column only; finding hidden behind marker/detail | FAIL | list reason like `volume impaired` | synthetic `Volume Health` / `Check` column |
| `tg` | unhealthy targets, all unhealthy, orphan | no top-level health reason; child `tg_health` view has detail | FAIL | top-level list should show `unhealthy targets` summary | synthetic `Target Health` / `Check` column |
| `pipeline` | failed / cancelled latest stage | no status/result column on top-level list | FAIL | top-level result summary | synthetic `Latest Result` column |
| `cb` | latest build failed / timed out | no build-result column on top-level list | FAIL | top-level `Latest Build` summary | synthetic `Latest Build` column |
| `sfn` | latest execution failed | no result column on top-level list | FAIL | top-level `Latest Execution` summary | synthetic `Latest Execution` column |
| `glue` | latest run failed / timed out | top-level list only shows job definition data | FAIL | top-level `Latest Run` summary | synthetic `Latest Run` column |
| `ddb` | PITR disabled | table looks `ACTIVE`; warning reason is hidden | FAIL | list should say `PITR disabled` | synthetic `Backup` / `Check` column |
| `s3` | public access block missing / disabled | bucket list has no security posture column | FAIL | list should say `public access block off` | synthetic `Public Access` / `Check` column |
| `vpc` | flow logs missing | VPC can look fully healthy from `State` alone | FAIL | list should say `flow logs missing` | synthetic `Flow Logs` / `Check` column |
| `asg` | latest scaling activity failed | desired/min/max counts do not explain failure loop | FAIL | list should show latest failure summary | synthetic `Latest Activity` / `Check` column |
| `backup` | recent backup job failed / partial | top-level plan list shows dates only | FAIL | list should show last job health | synthetic `Last Job` / `Check` column |
| `ses` | account probation / shutdown / quota pressure | identity rows show verification only | FAIL | list should show account health summary | synthetic `Account Health` / `Check` column |
| `kms` | rotation disabled | list has no rotation column | FAIL | list should say `rotation off` | synthetic `Rotation` / `Check` column |
| `efs` | zero mount targets | `Mounts` count exists, but zero is not framed as a problem | PARTIAL | either treat count as enough, or show explicit `no mount targets` | synthetic `Check` column if needed |
| `tgw` | failed / rejected attachments | TGW `State` alone hides attachment-level failures | FAIL | list should show attachment issue summary | synthetic `Attachments` / `Check` column |
| `eb` | health causes from `DescribeEnvironmentHealth` | `Health` and `HealthStatus` exist, but cause is hidden | PARTIAL | keep health columns, add short cause when degraded | synthetic `Health Cause` / `Check` column |
| `elb` | config issues from attributes | `State` is visible, config faults are not | FAIL | list should show the config problem, not just color | synthetic `Config` / `Check` column |
| `sqs` | missing DLQ / old messages / DLQ backlog | queue name alone is not explanatory | FAIL | list should show the specific queue concern | synthetic `Queue Health` / `Check` column |
| `sns` | zero / pending subscriptions | topic list does not explain attention | FAIL | list should say `no subscriptions` / `pending only` | synthetic `Subscriptions` / `Check` column |
| `msk` | software update due / cluster runtime concern | top-level list mostly shows inventory | FAIL | list should show actionable cluster health | synthetic `Cluster Health` / `Check` column |
| `acm` | renewal failed | `Status` and `Expires` help, but renewal failure is hidden | PARTIAL | keep `Expires`, add explicit renewal issue summary | synthetic `Renewal` / `Check` column |
| `cf` | logging disabled / no WAF | top-level list does not surface either | FAIL | list should show missing protection / logging | synthetic `Config` / `Check` column |
| `apigw` | no deployed stage | API inventory list looks healthy | FAIL | list should say `no stages deployed` | synthetic `Stage` / `Check` column |
| `cfn` | recent failed event, drift | `StackStatus` is useful but not enough for drift or latest failure reason | PARTIAL | keep `Status`; add drift / failure reason summary | YAML `StackStatusReason` plus synthetic `Check` for drift/events |
| `codeartifact` | repo policy risk / empty repo | inventory list hides the reason | FAIL | list should show `wide-open policy` / `empty repo` | synthetic `Policy` / `Check` column |
| `athena` | no bytes cutoff / no result encryption | workgroup list hides governance issue | FAIL | list should show governance summary | synthetic `Governance` / `Check` column |
| `r53` | no DNSSEC / likely unused zone | record count alone is weak | FAIL | list should show the actual concern | synthetic `DNSSEC` / `Check` column |
| `waf` | no logging / no attached resources | list is pure inventory | FAIL | list should show attachment / logging problem | synthetic `WAF Health` / `Check` column |
| `role` | dormant role | `Last Used` column helps, but reason still relies on color/threshold knowledge | PARTIAL | make dormancy explicit only when it is a problem | synthetic `Check` column |
| `policy` | wildcard admin | attachment count/path do not explain the risk | FAIL | list should say `admin policy` | synthetic `Risk` / `Check` column |
| `iam-user` | no MFA, stale active key | list has age fields but not the actual auth risk | FAIL | list should show `no MFA` / `stale key` | synthetic `Auth Risk` / `Check` column |
| `iam-group` | empty group / admin group | list is pure inventory | FAIL | list should show why the group is noteworthy | synthetic `Check` column |
| `logs` | missing metric filters | log group list does not explain the gap | FAIL | list should say `no metric filters` for audit groups | synthetic `Check` column |
| `ecs-svc` | rollout failed, placement failure, ELB health failure | counts help, but cause is buried in events/detail | PARTIAL | keep counts; add short cause when degraded | synthetic `Service Health` / `Check` column |
| `ecs` | sustained pending tasks / no running tasks with instances | counts help, but not enough to explain why attention exists | PARTIAL | keep counts; add short problem summary only when needed | synthetic `Cluster Health` / `Check` column |
| `eb-rule` | enabled rule with no targets, target without DLQ | top-level rule list hides why it is bad | FAIL | list should show `no targets` / `targets lack DLQ` | synthetic `Targets` / `Check` column |

## B. Partial: Data Already Exists, So A YAML/View Change Can Close The Gap

These are the best cheap wins. The signal is already in the fetched resource or
in existing derived `Resource.Fields`, so the user does not need a new Wave 2
presentation mechanism. We just need to actually expose the field.

| Type | Existing field/key already present | How it is done now | Verdict | How it should be | Cheapest path |
|---|---|---|---|---|---|
| `lambda` | `LastUpdateStatus`, `LastUpdateStatusReason`, `State`, `StateReason` | list only shows `State` | PARTIAL | add a reason/result column | YAML/default-view change |
| `ecs-task` | `StopCode`, `StoppedReason`, `HealthStatus` | list only shows `LastStatus` | PARTIAL | add `StopCode` or `Stopped Reason` | YAML/default-view change |
| `nat` | `FailureCode`, `FailureMessage` | list only shows `State` | PARTIAL | add `Failure` column | YAML/default-view change |
| `trail` | `Fields["is_logging"]`, `Fields["latest_delivery_error"]` | list hides the runtime health source entirely | PARTIAL | add `Logging` and/or `Delivery Error` columns | default-view change using existing field keys |
| `eks` | `Fields["health_issues_count"]`, `Fields["health_issues"]` | list only shows `Status` | PARTIAL | add `Issues` or `Health` column | default-view change using existing field keys |
| `ng` | `Fields["health_issues_count"]`, `Fields["health_issues"]` | list only shows `Status` | PARTIAL | add `Issues` or `Health` column | default-view change using existing field keys |
| `sg` | `Fields["dangerous_open_count"]`, `Fields["wide_open"]` | list only shows name/id/description | PARTIAL | add `Risk` / `Open` column | default-view change using existing field keys |
| `cfn` | `StackStatusReason` already exists on the stack object | list only shows `StackStatus` | PARTIAL | add `Reason` column before any synthetic work | YAML/default-view change |
| `acm` | `NotAfter`, `InUse`, `Status` are already present | list already has some help | PARTIAL | keep `Expires`; consider narrower issue-focused ordering before synthetic work | default-view tuning |

## C. Verification Summary

The repo currently has two different UX situations:

- **Real Wave 2 enrichers**: the signal is usually not in the list row model, so
  the user gets marker/banner/detail but not a plain-English list reason.
- **In-fetcher / derived-field types**: the data already exists, but the default
  view often does not expose it.

That means the implementation strategy should split cleanly:

1. **Cheap wins first**: update default views for `lambda`, `ecs-task`, `trail`,
   `eks`, `ng`, `sg`, `nat`, and `cfn`.
2. **Structural fix second**: add one generic synthetic list field for real
   Wave 2 findings, then opt-in per type via view config.
3. **Keep detail enrichment**: retain `Background Check` as the full drill-down,
   but stop forcing users to press `d` just to learn the first-order reason.

## Proposed Order

1. Add a generic synthetic list field, e.g. `issue_summary`, from
   `EnrichmentFinding.Summary`.
2. Add a default `Check` column only for the types in section A where the list
   is currently opaque.
3. Update the default views in section B to expose already-fetched reason fields.
4. Leave color, marker, banner, and detail `Background Check` in place as
   secondary signals, not the primary explanation surface.
