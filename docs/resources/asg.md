---
shortName: asg
name: Auto Scaling Groups
awsApiRef: https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_AutoScalingGroup.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# asg — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `asg`
- **Display name**: Auto Scaling Groups
- **AWS API reference**: <https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_AutoScalingGroup.html>
- **List API**: `DescribeAutoScalingGroups`
- **Describe API (if any)**: `DescribeScalingActivities` (Wave 2, one call per ASG with `MaxRecords=1`)

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `ami`, `ec2`, `elb`, `ng`, `role`, `sg`, `sns`, `subnet`, `tg`, `vpc`, `ct-events`.

### `alarm`

- **Why related**: Alarms that trigger the ASG's scaling policies — the "why is this group scaling?" pivot during an auto-scaling incident.
- **How discovered**: Cross-reference the already-loaded `alarm` list by `AlarmActions[]` — scaling-policy action ARNs embed the ASG name (`arn:aws:autoscaling:<region>:<acct>:scalingPolicy/<id>:autoScalingGroupName/<ASG>`); match the ASG name. — a9s-devops: possible=yes (`CloudWatch MetricAlarm.AlarmActions` carries the scaling-policy ARN; the ASG name is embedded in that ARN), worth=yes (daily-driver workflow — "what's making this group scale?").
- **Count shown**: yes.

### `ami`

- **Why related**: AMI the group's instances boot from — rollback target and vulnerability-scan pivot.
- **How discovered**: Read `LaunchConfiguration.ImageId` (legacy) or resolve `LaunchTemplate.LaunchTemplateData.ImageId` for the version the ASG references, then cross-reference the already-loaded `ami` list by ImageId. — a9s-devops: possible=yes (`AutoScalingGroup.LaunchConfigurationName` / `AutoScalingGroup.LaunchTemplate`), worth=yes (AMI drift and deprecation are common ASG failure causes).
- **Count shown**: yes.

### `ec2`

- **Why related**: Instances the ASG currently manages — the operator's primary drill-down when instance count is wrong or a subset is unhealthy.
- **How discovered**: Read `AutoScalingGroup.Instances[].InstanceId` and cross-reference the already-loaded `ec2` list by InstanceId.
- **Count shown**: yes.

### `elb`

- **Why related**: Load balancers the ASG registers instances with — the "is traffic reaching new instances?" pivot.
- **How discovered**: Read `AutoScalingGroup.LoadBalancerNames[]` (classic ELBv1 names) and `AutoScalingGroup.TargetGroupARNs[]`; for ALB/NLB, cross-reference the already-loaded `tg` list (targets → `LoadBalancerArns`) then the `elb` list by ARN. — a9s-devops: possible=yes (`AutoScalingGroup.LoadBalancerNames` + `AutoScalingGroup.TargetGroupARNs`; TG→ELB via `TargetGroup.LoadBalancerArns`), worth=yes (joint ASG+TG health is the canonical scale-event investigation).
- **Count shown**: yes.

### `ng`

- **Why related**: EKS node group that owns this ASG (if any) — shows Kubernetes ownership when the ASG is k8s-managed rather than operator-managed.
- **How discovered**: Reverse cross-reference — scan the already-loaded `ng` list for an entry whose `Nodegroup.Resources.AutoScalingGroups[].Name` matches this ASG's name. — a9s-devops: possible=yes (`Nodegroup.Resources.AutoScalingGroups` on the EKS describe response), worth=yes (EKS operators need to know which ASGs are k8s-controlled so they don't modify the wrong one).
- **Count shown**: yes (expected 0 or 1 — EKS creates one ASG per node group).

### `role`

- **Why related**: Service-linked role used by the ASG for EC2 calls, plus the instance profile role the launched instances assume.
- **How discovered**: Read `AutoScalingGroup.ServiceLinkedRoleARN` directly, and resolve `LaunchConfiguration.IamInstanceProfile` / `LaunchTemplate.LaunchTemplateData.IamInstanceProfile` → `GetInstanceProfile` → role name; cross-reference the already-loaded `role` list. — a9s-devops: possible=yes (`ServiceLinkedRoleARN` is on the list response; instance-profile role requires one extra IAM call), worth=yes (permission troubleshooting when scaling or health checks fail).
- **Count shown**: yes.

### `sg`

- **Why related**: Security groups attached to the instances the ASG launches — the "why can't new instances reach the DB?" pivot.
- **How discovered**: Read `LaunchConfiguration.SecurityGroups[]` or `LaunchTemplate.LaunchTemplateData.SecurityGroupIds[]` / `NetworkInterfaces[].Groups[]`, then cross-reference the already-loaded `sg` list by GroupId. — a9s-devops: possible=yes (via LaunchConfig/LaunchTemplate), worth=yes (SGs are the single most common cause of "ASG scaled but app is offline").
- **Count shown**: yes.

### `sns`

- **Why related**: SNS topics the ASG notifies on scaling events and lifecycle-hook transitions — the "who got paged?" pivot.
- **How discovered**: Call `DescribeNotificationConfigurations(AutoScalingGroupNames=[name])` → `TopicARN` and `DescribeLifecycleHooks(AutoScalingGroupName=name)` → `NotificationTargetARN` (SNS-only); cross-reference the already-loaded `sns` list by ARN. Cost: two extra per-ASG calls. — a9s-devops: possible=yes (both APIs are ASG-scoped reads), worth=yes (scale-event paging topology is visible nowhere else in the ASG detail view).
- **Count shown**: yes.

### `subnet`

- **Why related**: Subnets the ASG launches new instances into — AZ coverage and capacity check.
- **How discovered**: Parse `AutoScalingGroup.VPCZoneIdentifier` (comma-separated subnet IDs) and cross-reference the already-loaded `subnet` list by SubnetId.
- **Count shown**: yes.

### `tg`

- **Why related**: Target groups the ASG registers instances with — per-group target health.
- **How discovered**: Read `AutoScalingGroup.TargetGroupARNs[]` and cross-reference the already-loaded `tg` list by ARN.
- **Count shown**: yes.

### `vpc`

- **Why related**: VPC(s) the ASG operates in — network-context pivot.
- **How discovered**: Parse `AutoScalingGroup.VPCZoneIdentifier` → subnet IDs, resolve each subnet's `VpcId` from the already-loaded `subnet` list, then cross-reference the already-loaded `vpc` list. Typically a single VPC; multiple indicates a misconfiguration.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Universal pivot — CloudTrail audit trail for scaling events and configuration changes to this ASG (who scaled, who changed `MinSize`, who suspended processes).
- **How discovered**: Call `LookupEvents(LookupAttributes=ResourceName=<ASG name>)`.
- **Count shown**: yes.
- Universal pivot — applies to every registered type; see related-resources.md §Policy.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `Status == ""` (no delete in progress).
  - **State bucket**: Healthy.
  - **How obtained**: `AutoScalingGroup.Status` field on the list response.

- **Signal**: `Status == "Delete in progress"`.
  - **State bucket**: Warning.
  - **How obtained**: `AutoScalingGroup.Status` field on the list response.

- **Signal**: Any `Instances[].HealthStatus == "Unhealthy"`.
  - **State bucket**: Warning.
  - **How obtained**: `AutoScalingGroup.Instances[].HealthStatus` field on the list response.

- **Signal**: `InService` count (count of `Instances[]` with `LifecycleState == "InService"`) `< MinSize`.
  - **State bucket**: Broken.
  - **How obtained**: Computed over `AutoScalingGroup.Instances[].LifecycleState` and `AutoScalingGroup.MinSize` on the list response.

- **Signal**: `SuspendedProcesses[].ProcessName` contains any of `Launch`, `Terminate`, `HealthCheck`.
  - **State bucket**: Warning.
  - **How obtained**: `AutoScalingGroup.SuspendedProcesses[].ProcessName` field on the list response.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: Most recent scaling activity `StatusCode == Failed` (launch-failure loop).
  - **State bucket**: Broken.
  - **API call**: `DescribeScalingActivities(AutoScalingGroupName=<name>, MaxRecords=1)` — one call per ASG.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `GroupDesiredCapacity` vs `GroupInServiceInstances` delta sustained.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause. **Healthy rows render blank.** |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping applied below. Wave 1 Healthy (`Status == ""`) is omitted — silence is the UX.

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `Status == "Delete in progress"` | 1 | Warning | n/a | S2, S4 | `deleting` | `Group deletion in progress — capacity will drop to zero.` |
| Any `Instances[].HealthStatus == Unhealthy` | 1 | Warning | n/a | S2, S4 | `unhealthy: N of M instances` | `N of M instances reporting Unhealthy — ASG will replace them.` |
| `InService < MinSize` | 1 | Broken | n/a | S2, S4 | `below min: K of MinSize` | `Only K instances InService, below MinSize — capacity breached.` |
| `SuspendedProcesses` contains `Launch`/`Terminate`/`HealthCheck` | 1 | Warning | n/a | S2, S4 | `suspended: <process list>` | `Scaling paused — Launch/Terminate/HealthCheck processes suspended by operator.` |
| Latest `DescribeScalingActivities.StatusCode == Failed` | 2 | Broken | n/a (row is already red) | S1, S4 (dedup), S5 | `launch failed: <StatusMessage>` | `Most recent scaling activity failed: <StatusMessage> — new instances are not coming up.` |

Notes:

- The Wave 2 launch-failure signal typically co-occurs with Wave 1 `InService < MinSize` (Broken) — the row is already red. Per mapping rules, S3 is suppressed (no glyph on red rows), S4 should deduplicate with the existing `below min: …` text (prefer the more specific `launch failed: …` when both are present), and S5 carries the full `StatusMessage`. The `!` severity still bumps S1 because it is an important finding.
- If the launch-failure signal appears on a Healthy row (edge case: activity failed but `InService >= MinSize` because old instances are still serving), treat as `!` on green → S1, S3 (`!`), S4, S5.

### 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every Warning/Broken row carries a specific cause in the Status column (`deleting`, `unhealthy: N of M instances`, `below min: K of MinSize`, `suspended: <process list>`, `launch failed: <StatusMessage>`), so triage decisions ("which ASG do I open first?") are possible directly from the list.

## 5. Out of Scope

- All §3.3 Wave 3 signals (CloudWatch `GroupDesiredCapacity` vs `GroupInServiceInstances` delta sustained).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` § "What is a9s?").

## 6. Citations

- a9s golden doc — `asg` contract row — `docs/related-resources.md` § `Per-type contract` line for `asg`.
- a9s golden doc — per-target reasoning (`alarm`, `ami`, `ct-events`, `ec2`, `elb`, `ng`, `role`, `sg`, `sns`, `subnet`, `tg`, `vpc`) — `docs/related-resources.md` § `asg`.
- a9s golden doc — `ct-events` universal-pivot policy — `docs/related-resources.md` § `Policy` item 4.
- a9s golden doc — Wave 1 and Wave 2 signals for `asg` — `docs/attention-signals.md` § `Compute` row `asg`.
- a9s golden doc — Wave 3 OUT OF SCOPE signal — `docs/attention-signals.md` § `Compute` row `asg` (Wave 3 cell).
- a9s golden doc — read-only invariant — `docs/architecture.md` § `What is a9s?`.
- AWS Go SDK v2 — ASG list-response shape (`Status`, `Instances[]`, `SuspendedProcesses[]`, `MinSize`, `TargetGroupARNs`, `LoadBalancerNames`, `VPCZoneIdentifier`, `LaunchTemplate`, `LaunchConfigurationName`, `ServiceLinkedRoleARN`) — `AWS SDK Go v2 — autoscaling/types.AutoScalingGroup`.
- AWS Go SDK v2 — per-instance `HealthStatus` and `LifecycleState` — `AWS SDK Go v2 — autoscaling/types.Instance § HealthStatus`, `§ LifecycleState`.
- AWS Go SDK v2 — suspended-process shape — `AWS SDK Go v2 — autoscaling/types.SuspendedProcess § ProcessName`.
- AWS Go SDK v2 — Wave 2 response shape (`StatusCode`, `StatusMessage`) — `AWS SDK Go v2 — autoscaling/types.Activity § StatusCode`, `§ StatusMessage`; and `AWS SDK Go v2 — autoscaling/types.ScalingActivityStatusCode` (enum includes `Failed`).
- a9s-devops consultation — `alarm` discovery mechanism — `a9s-devops (2026-04-20): possible=yes, worth=yes. CloudWatch MetricAlarm.AlarmActions carries the scaling-policy ARN; the ASG name is embedded in that ARN so cross-ref from the already-loaded alarm list is sufficient. Answers the daily "what made this group scale?" question.`.
- a9s-devops consultation — `ng` reverse discovery — `a9s-devops (2026-04-20): possible=yes, worth=yes. Nodegroup.Resources.AutoScalingGroups.Name is the sole forward link. Reverse cross-ref from the already-loaded ng list identifies the owning node group so operators don't accidentally mutate k8s-managed ASGs.`.
- a9s-devops consultation — `sns` discovery (two extra ASG-scoped calls) — `a9s-devops (2026-04-20): possible=yes, worth=yes. DescribeNotificationConfigurations + DescribeLifecycleHooks are the only APIs that expose scale-event paging topology; two calls per ASG is acceptable because this info is invisible anywhere else in the detail view.`.
- a9s-devops consultation — `role` discovery (service-linked + instance-profile) — `a9s-devops (2026-04-20): possible=yes, worth=yes. ServiceLinkedRoleARN is on the list response; the instance-profile role requires one GetInstanceProfile round-trip but is central to permission troubleshooting when health checks or scaling fail.`.
- a9s-devops consultation — severity call for Wave 2 launch-failure on Healthy row — `a9s-devops (2026-04-20): possible=yes, worth=yes. Failed latest activity on an otherwise-green ASG is operator-actionable (new instances won't come up) — severity !, not ~. When the same condition coincides with InService < MinSize the row is already red and S3 is suppressed per S1–S5 rules.`.

<!-- BEGIN GENERATED: header -->
asg — COMPUTE. Lifecycle key: `status`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Approximate? |
| --- | --- | --- |
| ec2 | EC2 Instances | no |
| tg | Target Groups | no |
| subnet | Subnets | no |
| alarm | CloudWatch Alarms | yes |
| ng | EKS Node Groups | yes |
| ami | AMI | no |
| elb | Load Balancers | no |
| role | IAM Roles | no |
| sg | Security Groups | no |
| sns | SNS Topics | no |
| vpc | VPCs | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
