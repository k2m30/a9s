---
shortName: asg
name: Auto Scaling Groups
awsApiRef: https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_AutoScalingGroup.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
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

Expected targets from `docs/related-resources.md` § Per-type contract: `alarm`, `ami`, `ec2`, `elb`, `ng`, `role`, `sg`, `sns`, `subnet`, `tg`, `vpc`, `ct-events`.

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
- Universal pivot — applies to every registered type; see docs/related-resources.md §Policy.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeAutoScalingGroups](https://docs.aws.amazon.com/autoscaling/ec2/APIReference/API_DescribeAutoScalingGroups.html)

Transcribed from `docs/attention-signals.md § Signals § COMPUTE` row `asg`.

### 3.1 Wave 1 — zero extra API calls

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

- **Signal**: `LaunchConfigurationName` set.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: fewer than two `AvailabilityZones`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: behind a load balancer with `HealthCheckType != ELB`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: Most recent scaling activity `StatusCode == Failed` (launch-failure loop).
  - **State bucket**: Broken.
  - **API call**: `DescribeScalingActivities(AutoScalingGroupName=<name>, MaxRecords=1)` — one call per ASG.
  - **Cost shape**: per-resource.

- **Signal**: launch configuration `MetadataOptions` absent or `HttpTokens != required`.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: launch configuration `AssociatePublicIpAddress == true`.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: credential in launch configuration `UserData`.
  - **State bucket**: Broken.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `GroupDesiredCapacity` vs `GroupInServiceInstances` delta sustained.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `asg`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `Status == "Delete in progress"` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `delete in progress` |
| Any `Instances[].HealthStatus == Unhealthy` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `<N unhealthy instance(s)>` |
| `InService < MinSize` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `<N> of <M> instances in service` |
| `SuspendedProcesses` contains `Launch`/`Terminate`/`HealthCheck` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `scaling suspended` |
| `LaunchConfigurationName` set | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `uses a launch configuration` |
| fewer than two `AvailabilityZones` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `single availability zone` |
| behind a load balancer with `HealthCheckType != ELB` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `no load balancer health check` |
| Latest `DescribeScalingActivities.StatusCode == Failed` | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `latest scaling activity failed` |
| launch configuration `MetadataOptions` absent or `HttpTokens != required` | 2 | Warning | `~` | S2, S3, S4, S5 | `launch configuration allows IMDSv1` |
| launch configuration `AssociatePublicIpAddress == true` | 2 | Warning | `~` | S2, S3, S4, S5 | `launch configuration assigns public IPs` |
| credential in launch configuration `UserData` | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `credential in launch configuration user data` |

Notes:

- The Wave 2 launch-failure signal typically co-occurs with Wave 1 `InService < MinSize` (Broken) — the row is already red. S4 should deduplicate with the existing `below min: …` text (prefer the more specific `launch failed: …` when both are present), and S5 carries the full `StatusMessage`. The `!` severity still bumps S1 because it is an important finding.
- The launch-failure signal can fire while `InService >= MinSize`, because old instances are still serving. It colours the row red on its own then, and the menu count takes it.

### 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every Warning/Broken row carries a specific cause in the Status column (`delete in progress`, `<N unhealthy instance(s)>`, `<N> of <M> instances in service`, `scaling suspended`, `latest scaling activity failed`), so triage decisions ("which ASG do I open first?") are possible directly from the list.

## 5. Out of Scope

- All §3.3 Wave 3 signals (CloudWatch `GroupDesiredCapacity` vs `GroupInServiceInstances` delta sustained).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` § "What is a9s?").

## 6. Citations

- a9s golden doc — `asg` contract row — `docs/related-resources.md` § `Per-type contract` line for `asg`.
- a9s golden doc — per-target reasoning (`alarm`, `ami`, `ct-events`, `ec2`, `elb`, `ng`, `role`, `sg`, `sns`, `subnet`, `tg`, `vpc`) — `docs/related-resources.md` § `asg`.
- a9s golden doc — `ct-events` universal-pivot policy — `docs/related-resources.md` § `Policy` item 4.
- a9s golden doc — Wave 1 and Wave 2 signals for `asg` — `docs/attention-signals.md § Signals § COMPUTE` row `asg`.
- a9s golden doc — Wave 3 OUT OF SCOPE signal — `docs/attention-signals.md § Not yet implemented`.
- a9s golden doc — read-only invariant — `docs/architecture.md` § `What is a9s?`.
- AWS Go SDK v2 — ASG list-response shape (`Status`, `Instances[]`, `SuspendedProcesses[]`, `MinSize`, `TargetGroupARNs`, `LoadBalancerNames`, `VPCZoneIdentifier`, `LaunchTemplate`, `LaunchConfigurationName`, `ServiceLinkedRoleARN`) — `AWS SDK Go v2 — autoscaling/types.AutoScalingGroup`.
- AWS Go SDK v2 — per-instance `HealthStatus` and `LifecycleState` — `AWS SDK Go v2 — autoscaling/types.Instance § HealthStatus`, `§ LifecycleState`.
- AWS Go SDK v2 — suspended-process shape — `AWS SDK Go v2 — autoscaling/types.SuspendedProcess § ProcessName`.
- AWS Go SDK v2 — Wave 2 response shape (`StatusCode`, `StatusMessage`) — `AWS SDK Go v2 — autoscaling/types.Activity § StatusCode`, `§ StatusMessage`; and `AWS SDK Go v2 — autoscaling/types.ScalingActivityStatusCode` (enum includes `Failed`).
- a9s-devops consultation — `alarm` discovery mechanism — `a9s-devops (2026-04-20): possible=yes, worth=yes. CloudWatch MetricAlarm.AlarmActions carries the scaling-policy ARN; the ASG name is embedded in that ARN so cross-ref from the already-loaded alarm list is sufficient. Answers the daily "what made this group scale?" question.`.
- a9s-devops consultation — `ng` reverse discovery — `a9s-devops (2026-04-20): possible=yes, worth=yes. Nodegroup.Resources.AutoScalingGroups.Name is the sole forward link. Reverse cross-ref from the already-loaded ng list identifies the owning node group so operators don't accidentally mutate k8s-managed ASGs.`.
- a9s-devops consultation — `sns` discovery (two extra ASG-scoped calls) — `a9s-devops (2026-04-20): possible=yes, worth=yes. DescribeNotificationConfigurations + DescribeLifecycleHooks are the only APIs that expose scale-event paging topology; two calls per ASG is acceptable because this info is invisible anywhere else in the detail view.`.
- a9s-devops consultation — `role` discovery (service-linked + instance-profile) — `a9s-devops (2026-04-20): possible=yes, worth=yes. ServiceLinkedRoleARN is on the list response; the instance-profile role requires one GetInstanceProfile round-trip but is central to permission troubleshooting when health checks or scaling fail.`.
- a9s-devops consultation — severity call for Wave 2 launch-failure on Healthy row — `a9s-devops (2026-04-20): possible=yes, worth=yes. Failed latest activity on an otherwise-green ASG is operator-actionable (new instances won't come up) — severity !, not ~.`.

<!-- BEGIN GENERATED: header -->
asg — COMPUTE. Status key: `status` — the key the status cell reads, and the column naming it is the status column.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| asg.state.deleting | delete in progress | warn | wave1 | The group is being removed and is terminating its instances, so the capacity it provided is going away. If anything still depends on it, stop the deletion now — once it completes, the group and its scaling history are gone. |
| asg.instances.underprovisioned | <N> of <M> instances in service | broken | wave1 | The group is running fewer instances in service than it is meant to, so the workload behind it carries its traffic short-handed. Read the group's scaling activities for launch failures: insufficient capacity in the Availability Zone, a bad launch template, or a failing health check are the usual causes. |
| asg.instances.unhealthy | <N unhealthy instance(s)> | warn | wave1 | Some instances in this group are failing their health checks and will be terminated and replaced, briefly reducing capacity. Look at those instances' system logs before they go, so the replacements do not simply repeat the failure. |
| asg.scaling.suspended | scaling suspended | warn | wave1 | One or more scaling processes are suspended, so the group will not add capacity under load or replace instances that fail, however its policies are written. Resume the suspended processes unless a deployment tool is holding them deliberately. |
| asg.scaling-activity-failed | latest scaling activity failed | broken | wave2 | The group's most recent attempt to launch or terminate an instance failed, so it is not at the size its policies asked for. The activity's status message names the cause — capacity, a launch template error, or an IAM permission — fix that and the group retries. |
| asg.launch-config.legacy | uses a launch configuration | warn | wave1 | The group launches from a launch configuration, an immutable legacy resource AWS no longer develops — it cannot carry IMDSv2 defaults, newer instance types, or versioned edits. Copy it to a launch template and point the group at that. |
| asg.single-az | single availability zone | warn | wave1 | Every instance in this group sits in one availability zone, so a single zone failure takes the whole group down. Add subnets from at least one more zone to the group. |
| asg.no-elb-health-check | no load balancer health check | warn | wave1 | The group is behind a load balancer but only watches EC2 status checks, so an instance whose application has stopped answering stays in service. Set the group's health check type to the load balancer's. |
| asg.launch-config.imdsv1 | launch configuration allows IMDSv1 | warn | wave2 | Instances this group launches answer metadata requests without a session token, so an SSRF bug on any of them leaks the attached role's credentials. Launch configurations cannot be edited — copy this one to a launch template that requires session tokens and repoint the group. |
| asg.launch-config.public-ip | launch configuration assigns public IPs | warn | wave2 | Every instance this group launches gets a routable public address, so each new instance is reachable from the internet on whatever its security groups leave open. Copy the launch configuration to a launch template with public address assignment off. |
| asg.launch-config.secret | credential in launch configuration user data | broken | wave2 | A credential is pasted into the launch configuration's user data, so it is readable by anyone who can call autoscaling:DescribeLaunchConfigurations and lands on every instance the group starts. Move the value to Secrets Manager or Systems Manager Parameter Store and rotate it. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| ec2 | EC2 Instances | no |
| tg | Target Groups | yes |
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
