---
shortName: alarm
name: CloudWatch Alarms
awsApiRef: https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricAlarm.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# alarm — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `alarm`
- **Display name**: CloudWatch Alarms
- **AWS API reference**: <https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricAlarm.html>
- **List API**: `DescribeAlarms` — returns `MetricAlarm[]` (and `CompositeAlarm[]`; this spec is scoped to metric alarms, consistent with `related-resources.md`). The SDK confirms `StateValue`, `StateUpdatedTimestamp`, `ActionsEnabled`, `AlarmActions`, `OKActions`, `InsufficientDataActions`, `Dimensions`, `Namespace`, `MetricName`, `Period`, `StateReason` are all on the list shape, so every Wave 1 signal and every related-panel discovery path is reachable with zero extra calls.
- **Describe API (if any)**: not used — `DescribeAlarms` already returns the full `MetricAlarm` shape.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `apigw`, `asg`, `cb`, `dbi`, `ec2`, `ecs`, `eks`, `kms`, `lambda`, `logs`, `s3`, `sfn`, `sns`, `waf`, `ct-events`.

All non-action pivots (everything except `sns`, `asg`, and `ct-events`) are discovered the same way: read `MetricAlarm.Namespace` and `MetricAlarm.Dimensions[].Name`/`Value` on the already-loaded alarm, map the AWS namespace + dimension key to the target type, then cross-reference the loaded sibling-type list by the resource ID carried in `Dimensions[].Value`. CloudWatch alarms have no direct ARN field pointing at what they monitor — the namespace/dimensions pair is the canonical pivot path and is how the Wave 1 "zombie alarm" cross-reference already works.

### `apigw`

- **Why related**: Stage latency/error alarms watching an API Gateway API — operators open the API detail to see what the alarm is reacting to.
- **How discovered**: `Namespace == "AWS/ApiGateway"`; read `Dimensions[]` for `ApiName` (REST v1) or `ApiId` (HTTP/WebSocket v2); cross-reference the loaded `apigw` list by that identifier — a9s-devops (2026-04-20): possible=yes, worth=yes. The golden-doc row cites apigw only as a generic audit pivot; the real pivot is the dimension value. Namespace + dimension key is the CloudWatch-wide convention.
- **Count shown**: yes.

### `asg`

- **Why related**: The alarm drives an Auto Scaling Group scaling policy (`MetricAlarm.AlarmActions` pointing at ASG scaling policies, per the golden doc).
- **How discovered**: scan `AlarmActions[]`, `OKActions[]`, `InsufficientDataActions[]` for entries whose ARN is of the form `arn:aws:autoscaling:<region>:<acct>:scalingPolicy:<uuid>:autoScalingGroupName/<asg-name>`; the trailing `autoScalingGroupName/<asg-name>` segment is the ASG identifier. Cross-reference the loaded `asg` list by name. Also covered: `Namespace == "AWS/AutoScaling"` with `Dimensions[].Name == "AutoScalingGroupName"` — a9s-devops (2026-04-20): possible=yes, worth=yes. Scaling-policy ARNs encode the ASG name directly; this is the pivot operators want when they see an ASG-scaling alarm go red.
- **Count shown**: yes.

### `cb`

- **Why related**: CodeBuild project build-failure alarms watching `FailedBuilds` / `SucceededBuilds` metrics.
- **How discovered**: `Namespace == "AWS/CodeBuild"`; read `Dimensions[]` for `ProjectName`; cross-reference the loaded `cb` list by project name — a9s-devops (2026-04-20): possible=yes, worth=yes. Golden-doc row cites cb as a generic audit pivot; the real pivot is namespace + `ProjectName` dimension.
- **Count shown**: yes.

### `dbi`

- **Why related**: Common alarm dimension — RDS instance metrics (CPU, Connections, FreeStorageSpace, ReplicaLag). The golden doc cites this as a primary alarm pivot.
- **How discovered**: `Namespace == "AWS/RDS"`; read `Dimensions[]` for `DBInstanceIdentifier`; cross-reference the loaded `dbi` list by identifier.
- **Count shown**: yes.

### `ec2`

- **Why related**: Common alarm dimension — EC2 CPU / StatusCheckFailed / network metrics. The golden doc cites this as a primary alarm pivot.
- **How discovered**: `Namespace == "AWS/EC2"`; read `Dimensions[]` for `InstanceId`; cross-reference the loaded `ec2` list by instance ID.
- **Count shown**: yes.

### `ecs`

- **Why related**: ECS cluster-level alarms (CPU/MemoryReservation, service-level pendingTasks, etc.).
- **How discovered**: `Namespace == "AWS/ECS"`; read `Dimensions[]` for `ClusterName` (and optionally `ServiceName` — when present, the alarm is really about an `ecs-svc`, but the cluster pivot is still valid); cross-reference the loaded `ecs` list by cluster name — a9s-devops (2026-04-20): possible=yes, worth=yes. Golden-doc row cites ecs as a generic audit pivot; the real pivot is namespace + `ClusterName` dimension.
- **Count shown**: yes.

### `eks`

- **Why related**: EKS control-plane and container-insights alarms.
- **How discovered**: `Namespace` in `"AWS/EKS"` or `"ContainerInsights"`; read `Dimensions[]` for `ClusterName`; cross-reference the loaded `eks` list by cluster name — a9s-devops (2026-04-20): possible=yes, worth=yes. EKS alarms in practice live under `ContainerInsights` (CloudWatch Container Insights) as much as under `AWS/EKS`; both should be recognized by the same dimension key.
- **Count shown**: yes.

### `kms`

- **Why related**: Alarms on KMS key usage (request rate, throttling, invalid-ciphertext).
- **How discovered**: `Namespace == "AWS/KMS"`; read `Dimensions[]` for `KeyId`; cross-reference the loaded `kms` list by key ID — a9s-devops (2026-04-20): possible=yes, worth=yes. `AWS/KMS` metrics are dimensioned by `KeyId` (UUID form); matching against the loaded `kms` list is free.
- **Count shown**: yes.

### `lambda`

- **Why related**: Common alarm dimension — Lambda Errors / Throttles / Duration / ConcurrentExecutions. The golden doc cites this as a primary alarm pivot.
- **How discovered**: `Namespace == "AWS/Lambda"`; read `Dimensions[]` for `FunctionName`; cross-reference the loaded `lambda` list by function name.
- **Count shown**: yes.

### `logs`

- **Why related**: Metric-filter-driven alarms — a CloudWatch Logs metric filter emits a custom metric that the alarm watches. The golden doc states this directly.
- **How discovered**: metric filters emit metrics in an operator-chosen custom namespace (not `AWS/Logs`), so `Namespace` alone is not reliable; instead, scan `Dimensions[]` for a `LogGroupName` key (set by the metric-filter definition when the filter includes log-group as a dimension) and, when absent, fall back to `MetricAlarm.AlarmDescription` / `AlarmName` substring match against loaded log-group names — a9s-devops (2026-04-20): possible=partial, worth=yes. The golden doc surfaces this pivot but AWS does not guarantee a structured link from a metric-filter alarm back to its log group; the `LogGroupName` dimension is convention-driven. Implementations should prefer the dimension when present and accept a name-match fallback.
- **Count shown**: yes.

### `s3`

- **Why related**: S3 request-metrics alarms (4xxErrors, 5xxErrors, FirstByteLatency). Request metrics are opt-in per bucket.
- **How discovered**: `Namespace == "AWS/S3"`; read `Dimensions[]` for `BucketName`; cross-reference the loaded `s3` list by bucket name — a9s-devops (2026-04-20): possible=yes, worth=yes. Bucket-level storage metrics also dimension by `BucketName`, so the same pivot covers both storage and request-metrics alarms.
- **Count shown**: yes.

### `sfn`

- **Why related**: Step Functions execution-failure / throttled / timed-out alarms.
- **How discovered**: `Namespace == "AWS/States"`; read `Dimensions[]` for `StateMachineArn`; cross-reference the loaded `sfn` list by state-machine ARN — a9s-devops (2026-04-20): possible=yes, worth=yes. `AWS/States` is the Step Functions namespace; `StateMachineArn` is the canonical dimension.
- **Count shown**: yes.

### `sns`

- **Why related**: `MetricAlarm.AlarmActions` / `OKActions` notified via an SNS topic — the golden doc cites this directly. This is the primary route operators use to find the paging destination for an alarm.
- **How discovered**: scan `AlarmActions[]`, `OKActions[]`, `InsufficientDataActions[]` for entries whose ARN begins with `arn:aws:sns:`; cross-reference the loaded `sns` list by topic ARN.
- **Count shown**: yes.

### `waf`

- **Why related**: WAF blocked-request / allowed-request alarms.
- **How discovered**: `Namespace == "AWS/WAFV2"` (WAFv2) or `"AWS/WAF"` (legacy WAF Classic); read `Dimensions[]` for `WebACL` (WAFv2) or `WebACLName`/`WebACLId` (Classic); cross-reference the loaded `waf` list by ACL identifier — a9s-devops (2026-04-20): possible=yes, worth=yes. WAFv2 is the modern namespace and the dimension is `WebACL` + `Region` + `Rule`; the cross-reference uses the `WebACL` value.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Universal pivot — audit trail for alarm configuration changes (create, modify, enable/disable-actions, delete).
- **How discovered**: pre-built CloudTrail query scoped to `AlarmArn` as the resource identifier.
- **Count shown**: unknown (CloudTrail queries are windowed; a reliable total isn't available without a separate count call).
- Universal pivot — applies to every registered type; see `related-resources.md` §Policy.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `StateValue == OK`.
  - **State bucket**: Healthy.
  - **How obtained**: `MetricAlarm.StateValue` from `DescribeAlarms`.

- **Signal**: `StateValue == INSUFFICIENT_DATA`.
  - **State bucket**: Warning.
  - **How obtained**: `MetricAlarm.StateValue` from `DescribeAlarms`.

- **Signal**: `StateValue == ALARM`.
  - **State bucket**: Broken.
  - **How obtained**: `MetricAlarm.StateValue` from `DescribeAlarms`; operator-readable cause is carried in `MetricAlarm.StateReason`.

- **Signal**: `ActionsEnabled == false` (muted alarm).
  - **State bucket**: Warning.
  - **How obtained**: `MetricAlarm.ActionsEnabled` from `DescribeAlarms`.

- **Signal**: `AlarmActions == []` (alert-to-nowhere).
  - **State bucket**: Warning.
  - **How obtained**: `MetricAlarm.AlarmActions` from `DescribeAlarms` (an empty slice means no action is wired for the ALARM transition).

- **Signal**: `StateValue == INSUFFICIENT_DATA` AND `StateUpdatedTimestamp` older than `2 × Period` (dead metric pipeline).
  - **State bucket**: Broken.
  - **How obtained**: `MetricAlarm.StateValue`, `MetricAlarm.StateUpdatedTimestamp`, and `MetricAlarm.Period` from `DescribeAlarms`, compared to wall-clock time. Overrides the plain `INSUFFICIENT_DATA` Warning when both apply.

- **Signal**: `Dimensions[]` reference a resource ID that is absent from the already-loaded sibling-type list (zombie alarm). Rule is skipped when the relevant sibling list was not loaded in this sweep, to avoid false positives.
  - **State bucket**: Warning.
  - **How obtained**: `MetricAlarm.Namespace` + `MetricAlarm.Dimensions[]` from `DescribeAlarms`, cross-referenced against the loaded list of whichever sibling type the namespace maps to (e.g. `AWS/EC2` + `InstanceId` vs loaded `ec2` list; `AWS/RDS` + `DBInstanceIdentifier` vs loaded `dbi` list; same pivot table as §2).

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

The golden doc's Wave 3 cell is `None` for this resource. Nothing to copy.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause. **Healthy rows render blank.** |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit).
- **Wave 1 Warning / Broken / Dim** → S2 + S4.
- **Wave 2 finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4, S5.
- **Wave 2 finding on a Healthy row, informational** → `~` glyph on green row. S3, S4, S5. No S1.
- **Wave 2 finding on an already yellow/red/dim row** → S3 suppressed, S4 deduplicates with existing cause, S5 carries the full sentence, S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `StateValue == INSUFFICIENT_DATA` | 1 | Warning | n/a | S2, S4 | `no data` | `Alarm has no recent data points — metric may not be reporting.` |
| `StateValue == ALARM` | 1 | Broken | n/a | S2, S4 | `firing: <StateReason short>` | `Alarm firing — <MetricAlarm.StateReason text>.` |
| `ActionsEnabled == false` | 1 | Warning | n/a | S2, S4 | `actions muted` | `Alarm is configured but actions are disabled — no one will be paged if it fires.` |
| `AlarmActions == []` | 1 | Warning | n/a | S2, S4 | `no action wired` | `Alarm has no ALARM-state action — transitions go unobserved.` |
| `INSUFFICIENT_DATA older than 2×Period` | 1 | Broken | n/a | S2, S4 | `metric pipeline stale <Xm>` | `Alarm stuck in INSUFFICIENT_DATA for more than 2x the evaluation period — metric stopped reporting.` |
| zombie alarm (dimension points at missing resource) | 1 | Warning | n/a | S2, S4 | `zombie: <dim-name>=<dim-value>` | `Alarm watches a <sibling-type> that is not in the loaded list — likely points at a deleted resource.` |

Notes:

- `StateReason` is a short text blob provided by CloudWatch explaining the transition (e.g. `Threshold Crossed: 1 datapoint [85.3] was greater than or equal to the threshold (80.0).`). The S4 `firing: <StateReason short>` cell should show the first short phrase of that text (truncate at ~30 chars); the full sentence belongs in S5.
- The "alert-to-nowhere" case (`AlarmActions == []`) and the "actions muted" case (`ActionsEnabled == false`) can coexist with `StateValue == OK`. Per the §4 mapping, those two Warning rules make the alarm's row yellow even when `OK`. That is deliberate: a silent alarm is a configuration issue an operator should see.
- When an alarm is both `ALARM` (Broken/red) and `ActionsEnabled == false`, the row stays red and S4 prefers the `ALARM` cause; the S5 line mentions the muted-actions condition in parentheses.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every non-healthy alarm carries a specific cause in S4 (`firing: Threshold Crossed`, `no data`, `actions muted`, `no action wired`, `metric pipeline stale 45m`, `zombie: InstanceId=i-abc`), which is exactly the triage information an on-call engineer needs to decide whether to drill in or move on. The only compression concern is the `firing: <StateReason>` cell — `StateReason` can exceed 40 characters; the list renderer must truncate and the full text must remain available in S5.

## 5. Out of Scope

- No Wave 3 signals are defined for this resource.
- Composite alarms (`DescribeAlarms` `CompositeAlarm[]`) are not covered by this spec — the related-resources.md row scopes `alarm` to `MetricAlarm`.
- Any UI element not listed in §4 — no new columns, icons, views, or key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` § "What is a9s?").

## 6. Citations

- alarm related-panel targets `apigw`, `asg`, `cb`, `dbi`, `ec2`, `ecs`, `eks`, `kms`, `lambda`, `logs`, `s3`, `sfn`, `sns`, `waf`, `ct-events` — `docs/related-resources.md` § Per-type contract, row `alarm`.
- alarm Wave 1 signal set (`StateValue` enum mapping, `ActionsEnabled==false`, `AlarmActions==[]`, `INSUFFICIENT_DATA` age vs `2×Period`, `Dimensions` zombie cross-ref) — `docs/attention-signals.md` § Monitoring, row `alarm` Wave 1 cell.
- alarm has no Wave 2 signals — `docs/attention-signals.md` § Monitoring, row `alarm` Wave 2 cell (`None`).
- alarm has no Wave 3 signals — `docs/attention-signals.md` § Monitoring, row `alarm` Wave 3 cell (`None`).
- `StateValue`, `StateReason`, `StateUpdatedTimestamp`, `ActionsEnabled`, `AlarmActions`, `OKActions`, `InsufficientDataActions`, `Dimensions`, `Namespace`, `MetricName`, `Period`, `AlarmArn`, `AlarmName` all present on `DescribeAlarms` response — `AWS SDK Go v2 — service/cloudwatch/types.MetricAlarm § StateValue, StateReason, StateUpdatedTimestamp, ActionsEnabled, AlarmActions, OKActions, InsufficientDataActions, Dimensions, Namespace, MetricName, Period, AlarmArn, AlarmName`.
- `Dimension` shape is `{Name, Value}` — `AWS SDK Go v2 — service/cloudwatch/types.Dimension § Name, Value`.
- SNS topic discovery via `AlarmActions`/`OKActions`/`InsufficientDataActions` ARN prefix `arn:aws:sns:` — `docs/related-resources.md` § Per-target reasoning, row `alarm → sns` ("MetricAlarm.AlarmActions / OKActions — SNS topics notified").
- ASG discovery via `AlarmActions`/`OKActions`/`InsufficientDataActions` ARN prefix `arn:aws:autoscaling:...scalingPolicy:...:autoScalingGroupName/...` — `docs/related-resources.md` § Per-target reasoning, row `alarm → asg` ("MetricAlarm.AlarmActions pointing at ASG scaling policies").
- apigw discovered via `Namespace == "AWS/ApiGateway"` + `Dimensions[]` `ApiName`/`ApiId` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Golden-doc row cites apigw only as a generic audit pivot; namespace + dimension key is the CloudWatch-wide convention used to link alarms to monitored resources.`
- cb discovered via `Namespace == "AWS/CodeBuild"` + `Dimensions[]` `ProjectName` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Golden-doc row cites cb as a generic audit pivot; dimension-based pivot matches CodeBuild's published namespace.`
- ecs discovered via `Namespace == "AWS/ECS"` + `Dimensions[]` `ClusterName` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Golden-doc row cites ecs as a generic audit pivot; namespace + ClusterName dimension is the standard ECS metric shape.`
- eks discovered via `Namespace` in `"AWS/EKS"` or `"ContainerInsights"` + `Dimensions[]` `ClusterName` — `a9s-devops (2026-04-20): possible=yes, worth=yes. EKS-adjacent alarms in practice live as often under Container Insights as under AWS/EKS; both should be recognized.`
- kms discovered via `Namespace == "AWS/KMS"` + `Dimensions[]` `KeyId` — `a9s-devops (2026-04-20): possible=yes, worth=yes. AWS/KMS metrics are dimensioned by KeyId.`
- logs discovered via `Dimensions[]` `LogGroupName` with a name-match fallback on `AlarmName`/`AlarmDescription` — `a9s-devops (2026-04-20): possible=partial, worth=yes. Metric-filter alarms emit to an operator-chosen custom namespace, so AWS does not guarantee a structured link back to the log group; the LogGroupName dimension is convention-driven but widely used. Name-match fallback preserves the pivot when the convention is not followed.`
- s3 discovered via `Namespace == "AWS/S3"` + `Dimensions[]` `BucketName` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Request-metrics and storage metrics both dimension by BucketName.`
- sfn discovered via `Namespace == "AWS/States"` + `Dimensions[]` `StateMachineArn` — `a9s-devops (2026-04-20): possible=yes, worth=yes. AWS/States is the Step Functions namespace and StateMachineArn is its canonical dimension.`
- waf discovered via `Namespace == "AWS/WAFV2"` or `"AWS/WAF"` + `Dimensions[]` `WebACL`/`WebACLName`/`WebACLId` — `a9s-devops (2026-04-20): possible=yes, worth=yes. WAFv2 is the modern namespace; the dimension key is WebACL. Legacy WAF Classic uses different dimension keys but is still in production at some accounts.`
- `ct-events` is the universal pivot applied to every registered type — `docs/related-resources.md` § Policy, rule 4.
- a9s is read-only — `docs/architecture.md` § "What is a9s?".
- Superseded HOW ignored — row middle-dot `·` marker, `⚠ Background Check` detail header, and derived list-level banner in `docs/enrichment-visibility.md` are not cited or reproduced per the skill's S1–S5 rules.
