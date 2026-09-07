---
shortName: ecs
name: ECS Clusters
awsApiRef: https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Cluster.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# ecs — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `ecs`
- **Display name**: ECS Clusters
- **AWS API reference**: <https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Cluster.html>
- **List API**: `ListClusters` (returns cluster ARN strings) followed by `DescribeClusters` for the batch (standard ECS list-then-describe pattern — the per-cluster `status`, `pendingTasksCount`, `runningTasksCount`, `registeredContainerInstancesCount` used by Wave 1 are on the `Cluster` describe shape, not the bare list).
- **Describe API (if any)**: `DescribeClusters(include=STATISTICS)` — Wave 2 enrichment adds task/instance counts to already-loaded clusters.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `asg`, `cfn`, `ec2`, `ecs-svc`, `ecs-task`, `kms`, `logs`, `ct-events`.

### `alarm`

- **Why related**: Cluster-level alarms on resource utilization (CW `CPUReservation`, `MemoryReservation` on the `AWS/ECS` namespace are typically dimensioned by `ClusterName`).
- **How discovered**: cross-reference the already-loaded `alarm` list by any `Dimensions[].Name=="ClusterName"` whose `Value` matches this cluster's `clusterName`.
- **Count shown**: yes.

### `asg`

- **Why related**: Container-instance ASG for EC2 launch-type clusters — operator pivot for capacity issues.
- **How discovered**: read `Cluster.CapacityProviders[]`, then resolve each provider via `DescribeCapacityProviders` → `AutoScalingGroupProvider.AutoScalingGroupArn`; cross-reference the already-loaded `asg` list by ARN. No direct field on `Cluster` names the ASG.
- **Count shown**: yes.

### `cfn`

- **Why related**: CloudFormation stack that created the cluster — jump to the stack for change history and drift context.
- **How discovered**: read `Cluster.Tags[]` for the AWS-injected tag `aws:cloudformation:stack-name`; cross-reference the already-loaded `cfn` list by stack name.
- **Count shown**: yes.

### `ec2`

- **Why related**: Container instances (EC2 launch type) — operator pivot for host-level problems (agent disconnect, instance impaired).
- **How discovered**: call `ListContainerInstances(cluster=<arn>)` + `DescribeContainerInstances` → `Ec2InstanceId` per instance; cross-reference the already-loaded `ec2` list by instance ID. Fargate-only clusters return zero.
- **Count shown**: yes.

### `ecs-svc`

- **Why related**: Services running on this cluster — the natural child view for cluster operators.
- **How discovered**: call `ListServices(cluster=<arn>)` (AWS supports direct filter by cluster).
- **Count shown**: yes — also aligns with `Cluster.ActiveServicesCount` on the describe shape.

### `ecs-task`

- **Why related**: Tasks running in this cluster — the other natural child view, one step below services.
- **How discovered**: call `ListTasks(cluster=<arn>)` (AWS supports direct filter by cluster).
- **Count shown**: yes — aligns with `Cluster.RunningTasksCount` + `PendingTasksCount` on the describe shape.

### `kms`

- **Why related**: `ExecuteCommandConfiguration.KmsKeyId` — the key that encrypts the `ecs exec` session stream. A key in `PendingDeletion` breaks `ecs exec` for everyone on the cluster.
- **How discovered**: read `Cluster.Configuration.ExecuteCommandConfiguration.KmsKeyId`; cross-reference the already-loaded `kms` list by key ID/ARN.
- **Count shown**: yes (0 or 1).

### `logs`

- **Why related**: `ExecuteCommandConfiguration.LogConfiguration.CloudWatchLogGroupName` — the log group receiving `ecs exec` session transcripts.
- **How discovered**: read `Cluster.Configuration.ExecuteCommandConfiguration.LogConfiguration.CloudWatchLogGroupName`; cross-reference the already-loaded `logs` list by name.
- **Count shown**: yes (0 or 1).

### `ct-events`

- **Why related**: Audit trail for cluster config changes (`CreateCluster`, `UpdateCluster`, `DeleteCluster`, `PutClusterCapacityProviders`, execute-command session starts).
- **How discovered**: `LookupEvents(ResourceName=<cluster-arn>)` on demand — universal pivot — applies to every registered type; see `related-resources.md` §Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeClusters](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeClusters.html)

Transcribed from `docs/attention-signals.md § Signals § COMPUTE` row `ecs`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `status == PROVISIONING` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `Cluster.Status` field from `DescribeClusters`.

- **Signal**: `status == DEPROVISIONING` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `Cluster.Status` field from `DescribeClusters`.

- **Signal**: `status == FAILED` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `Cluster.Status` field from `DescribeClusters`.

- **Signal**: `status == INACTIVE` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `Cluster.Status` field from `DescribeClusters` (per SDK: `INACTIVE` = deleted but still visible for a grace period — surfaces as Broken so operators notice a stale reference).

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: `pendingTasksCount > 0` sustained → Warning.
  - **State bucket**: Warning.
  - **API call**: `DescribeClusters(include=STATISTICS)` — one call per batch of up to 100 clusters; the same response already consumed for Wave 1.
  - **Cost shape**: hybrid (piggy-backs on the existing describe, so effectively free per cluster beyond the first batch).

- **Signal**: `runningTasksCount == 0 && registeredContainerInstancesCount > 0` → Warning.
  - **State bucket**: Warning.
  - **API call**: `DescribeClusters(include=STATISTICS)` — same call as above.
  - **Cost shape**: hybrid.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `CPUReservation`/`MemoryReservation` per cluster.
- OUT OF SCOPE: `DescribeContainerInstances` per cluster for agent-disconnect detection.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `ecs`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `status == PROVISIONING` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `provisioning` |
| `status == DEPROVISIONING` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `deprovisioning` |
| `status == FAILED` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `failed` |
| `status == INACTIVE` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `inactive` |
| `pendingTasksCount > 0 sustained` (on ACTIVE cluster) | 2 | Warning | `~` | S2, S3, S4, S5 | `tasks pending or not running` |
| `runningTasksCount == 0 && registeredContainerInstancesCount > 0` (on ACTIVE cluster) | 2 | Warning | `~` | S2, S3, S4, S5 | `tasks pending or not running` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`DORMANT`, `stopped`, `available`, `failed`) in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column. Tests will assert the cause is present.
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), you may omit the row from this table entirely; §3 still describes it.
- Keep the List text short enough to fit: ≤ 40 chars. The Detail cell quotes the finding's Detail constant verbatim, however long it is.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for Wave 1 states — `failed: cluster creation failed`, `inactive: deleted (stale ref)`, `provisioning/deprovisioning` — each pairs the keyword with a cause. A cluster whose tasks are not running goes yellow reading `tasks pending or not running`, which separates "known-bad shape" from "transient burst" once the operator knows the cluster; the deeper reason (agent disconnect vs ENI attach failure) needs detail, where the counts are. All problem rows are self-explanatory in the list — operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- Superseded HOW in `docs/historical/analysis/enrichment-visibility.md`: row middle-dot `·` marker, `⚠ Background Check` detail header, and the derived list-level banner `⚠ N issues detected by background checks`. These are earlier UX calls replaced by the S1–S5 rules above and this per-resource spec.

## 6. Citations

- a9s golden doc — the `ecs` signal definitions — `docs/attention-signals.md § Signals § COMPUTE` row `ecs`.
- a9s golden doc — expected related targets `alarm, asg, cfn, ct-events, ec2, ecs-svc, ecs-task, kms, logs` — `docs/related-resources.md` § "Per-type contract" row `ecs` and § `ecs`.
- a9s golden doc — `ct-events` is the universal pivot — `docs/related-resources.md` § Policy, item 4.
- a9s golden doc — read-only invariant — `docs/architecture.md` § "What is a9s?".
- AWS Go SDK v2 — `Cluster.Status` values `ACTIVE / PROVISIONING / DEPROVISIONING / FAILED / INACTIVE` with grace-period semantics for `INACTIVE` — `AWS SDK Go v2 — ecs/types.Cluster § Status`.
- AWS Go SDK v2 — `pendingTasksCount`, `runningTasksCount`, `registeredContainerInstancesCount`, `ActiveServicesCount` on the describe shape — `AWS SDK Go v2 — ecs/types.Cluster § PendingTasksCount, RunningTasksCount, RegisteredContainerInstancesCount, ActiveServicesCount`.
- AWS Go SDK v2 — execute-command config carries the KMS key and CloudWatch log group references — `AWS SDK Go v2 — ecs/types.ClusterConfiguration § ExecuteCommandConfiguration` and `ecs/types.ExecuteCommandConfiguration § KmsKeyId, LogConfiguration`.
- AWS Go SDK v2 — capacity-provider ASG link is indirect, through `CapacityProviders[]` → `DescribeCapacityProviders` → `AutoScalingGroupProvider.AutoScalingGroupArn` — `AWS SDK Go v2 — ecs/types.Cluster § CapacityProviders`.
- AWS API Reference (fallback) — `ListServices`/`ListTasks` support a `cluster` filter — `AWS API Reference: ListServices` and `ListTasks` (<https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ListServices.html>, <https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ListTasks.html>).
- AWS API Reference (fallback) — container-instance EC2 linkage via `DescribeContainerInstances.Ec2InstanceId` — `AWS API Reference: DescribeContainerInstances` (<https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_DescribeContainerInstances.html>).
- AWS API Reference (fallback) — CloudFormation stack tag `aws:cloudformation:stack-name` is injected on resources created by a stack — `AWS API Reference: AWS resource and property types reference` (<https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-resource-tags.html>).
- AWS API Reference (fallback) — CloudWatch Alarm `Dimensions` shape — `AWS API Reference: Dimension` (<https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_Dimension.html>).

<!-- BEGIN GENERATED: header -->
ecs — COMPUTE. Lifecycle key: `status`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| ecs.state.provisioning | provisioning | warn | wave1 | — |
| ecs.state.deprovisioning | deprovisioning | warn | wave1 | — |
| ecs.state.failed | failed | broken | wave1 | — |
| ecs.state.inactive | inactive | broken | wave1 | — |
| ecs.cluster-issue | tasks pending or not running | warn | wave2 | — |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| ecs-svc | ECS Services | yes |
| alarm | CloudWatch Alarms | yes |
| cfn | CloudFormation Stacks | yes |
| kms | KMS Key | no |
| asg | Auto Scaling Groups | yes |
| ec2 | EC2 Instances | yes |
| ct-events | CloudTrail Events | no |
| ecs-task | ECS Tasks | yes |
| logs | Log Groups | yes |
<!-- END GENERATED: related -->
