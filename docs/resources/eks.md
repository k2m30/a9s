---
shortName: eks
name: EKS Clusters
awsApiRef: https://docs.aws.amazon.com/eks/latest/APIReference/API_Cluster.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# eks — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `eks`
- **Display name**: EKS Clusters
- **AWS API reference**: <https://docs.aws.amazon.com/eks/latest/APIReference/API_Cluster.html>
- **List API**: `ListClusters` — returns cluster name strings only. The SDK confirms the list response carries nothing useful for attention: no status, no health, no VPC config. Every operator-visible signal requires the Describe.
- **Describe API (if any)**: `DescribeCluster` per cluster (N+1 fan-out) — returns `types.Cluster` carrying `Status`, `Health.Issues[]`, `Version`, `PlatformVersion`, `RoleArn`, `ResourcesVpcConfig`, `EncryptionConfig[]`, `Tags`. All Wave 2 signals and most related-panel pivots read from this shape.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `ami`, `asg`, `cfn`, `ec2`, `kms`, `logs`, `ng`, `role`, `sg`, `subnet`, `vpc`, `ct-events`.

### `alarm`

- **Why related**: CloudWatch alarms watching cluster or control-plane metrics — first signal of impact on the EKS control plane.
- **How discovered**: cross-reference the already-loaded `alarm` list; keep alarms whose `Dimensions[]` has `Name==ClusterName` AND `Value==Cluster.Name` — a9s-devops: `ClusterName` is the standard CloudWatch dimension for the `AWS/EKS` namespace; no extra API call needed once the `alarm` list is loaded in the same sweep.
- **Count shown**: yes.

### `ami`

- **Why related**: AMIs applied to this cluster's worker nodes — used when auditing AMI drift or chasing a bad image.
- **How discovered**: indirect — AMIs are not on `Cluster`; the pivot goes via `ng` (each `Nodegroup.AmiType`/`ReleaseVersion` resolves to an AMI). From the cluster view, surface this as a pass-through that lists AMIs referenced by child node groups — a9s-devops: direct cluster→ami discovery is not possible from `DescribeCluster`; daily-driver operators normally drill into `ng` first, so the eks→ami pivot is secondary but still expected when node groups are already loaded.
- **Count shown**: yes (aggregated across this cluster's node groups).

### `asg`

- **Why related**: Backing Auto Scaling Groups — where worker-node capacity actually lives; scale-in/scale-out events appear on the ASG.
- **How discovered**: indirect — `AutoScalingGroups[]` lives on `Nodegroup.Resources`, not on the cluster. From the cluster, aggregate the ASG names across the cluster's `ng` entries and cross-reference the loaded `asg` list — a9s-devops: EKS does not expose a direct cluster→ASG mapping; operators reach ASGs through node groups, so this pivot is always a two-hop aggregate.
- **Count shown**: yes.

### `cfn`

- **Why related**: CloudFormation stack that created the cluster — infra-as-code linkage for `eksctl` and many in-house IaC flows.
- **How discovered**: read `Cluster.Tags["aws:cloudformation:stack-name"]`; cross-reference the loaded `cfn` list by `StackName` — a9s-devops: `aws:cloudformation:stack-name` is the standard CFN-owner tag propagated to every stack-managed resource, including EKS clusters created via CloudFormation or `eksctl` (which wraps CFN).
- **Count shown**: yes (typically 0 or 1).

### `ec2`

- **Why related**: Worker-node EC2 instances running this cluster's pods — where SSH, console-output, and status-check troubleshooting lands.
- **How discovered**: cross-reference the loaded `ec2` list; keep instances whose tags contain `kubernetes.io/cluster/<Cluster.Name>==owned` OR `eks:cluster-name==<Cluster.Name>` — a9s-devops: EKS worker nodes carry both tags (managed node groups set `eks:cluster-name`; the `kubernetes.io/cluster/<name>=owned` tag is the cluster-autoscaler and legacy Kubernetes convention). Cross-ref is a pure tag filter against the already-loaded list, no extra API call.
- **Count shown**: yes.

### `kms`

- **Why related**: Customer-managed KMS key encrypting cluster secrets at rest (envelope encryption for Kubernetes secrets).
- **How discovered**: read `Cluster.EncryptionConfig[].Provider.KeyArn`; cross-reference the loaded `kms` list by KeyArn — a9s-devops: `EncryptionConfig` is only populated when envelope encryption is enabled; suppress the pivot when the slice is empty.
- **Count shown**: yes (0 or 1 per cluster in practice).

### `logs`

- **Why related**: Control-plane log groups — `api`, `audit`, `authenticator`, `controllerManager`, `scheduler` streams for the cluster.
- **How discovered**: deterministic name derivation — the log group for this cluster is `/aws/eks/<Cluster.Name>/cluster`; cross-reference the loaded `logs` list by exact name match — a9s-devops: EKS always uses this fixed name pattern for control-plane logs; the pivot is effectively a direct lookup, no extra API call.
- **Count shown**: yes (typically 1 when control-plane logging is enabled, 0 otherwise).

### `ng`

- **Why related**: Managed node groups attached to the cluster — the direct child view where worker-node capacity, AMI version, and scaling config live.
- **How discovered**: cross-reference the loaded `ng` list by `Nodegroup.ClusterName == Cluster.Name` — the `ng` list is already cluster-scoped by its `ListNodegroups(clusterName=…)` call pattern, so the filter is an exact-string match.
- **Count shown**: yes.

### `role`

- **Why related**: EKS service role — the IAM role the Kubernetes control plane assumes to call AWS APIs (create ENIs, load balancers, etc.).
- **How discovered**: read `Cluster.RoleArn`; cross-reference the loaded `role` list by ARN — a9s-devops: `RoleArn` is a required field on every EKS cluster; the pivot is always present.
- **Count shown**: yes (always 1).

### `sg`

- **Why related**: Security groups protecting control-plane-to-data-plane traffic and any extra SGs attached to the cross-account ENIs.
- **How discovered**: read `Cluster.ResourcesVpcConfig.ClusterSecurityGroupId` (EKS-managed SG) plus `Cluster.ResourcesVpcConfig.SecurityGroupIds[]` (extra SGs); cross-reference the loaded `sg` list by ID — a9s-devops: `ClusterSecurityGroupId` is auto-created by EKS and is the canonical control-plane-to-node SG; `SecurityGroupIds[]` holds any extras configured at cluster creation.
- **Count shown**: yes.

### `subnet`

- **Why related**: Subnets where the cross-account ENIs and worker nodes live — AZ coverage and public/private routing context.
- **How discovered**: read `Cluster.ResourcesVpcConfig.SubnetIds[]`; cross-reference the loaded `subnet` list by subnet ID — direct field on the cluster, always populated.
- **Count shown**: yes.

### `vpc`

- **Why related**: The cluster's VPC — network parent for all worker nodes and the control-plane ENIs.
- **How discovered**: read `Cluster.ResourcesVpcConfig.VpcId`; cross-reference the loaded `vpc` list by VPC ID — direct field on the cluster, always populated.
- **Count shown**: yes (always 1).

### `ct-events`

- **Why related**: Universal pivot — who created, updated, or deleted this cluster; who changed its config or endpoint access.
- **How discovered**: pre-built CloudTrail query scoped to `Cluster.Arn` as the resource identifier.
- **Count shown**: unknown (CloudTrail queries are windowed; a reliable total isn't available without a separate count call).
- Universal pivot — applies to every registered type; see `related-resources.md` §Policy.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — the list API does not return fields usable for attention. `ListClusters` returns cluster name strings only.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: `Status == ACTIVE`.
  - **State bucket**: Healthy.
  - **API call**: `DescribeCluster` per cluster — one call per cluster.
  - **Cost shape**: per-resource.

- **Signal**: `Status == CREATING`.
  - **State bucket**: Warning.
  - **API call**: `DescribeCluster` per cluster.
  - **Cost shape**: per-resource.

- **Signal**: `Status == UPDATING`.
  - **State bucket**: Warning.
  - **API call**: `DescribeCluster` per cluster.
  - **Cost shape**: per-resource.

- **Signal**: `Status == DELETING`.
  - **State bucket**: Warning.
  - **API call**: `DescribeCluster` per cluster.
  - **Cost shape**: per-resource.

- **Signal**: `Status == PENDING`.
  - **State bucket**: Warning.
  - **API call**: `DescribeCluster` per cluster.
  - **Cost shape**: per-resource.

- **Signal**: `Status == FAILED`.
  - **State bucket**: Broken.
  - **API call**: `DescribeCluster` per cluster.
  - **Cost shape**: per-resource.

- **Signal**: `Health.Issues[]` non-empty.
  - **State bucket**: Broken.
  - **API call**: `DescribeCluster` per cluster (same call — `Health` is on the Describe shape). Each `ClusterIssue` carries `Code` (enum, e.g. `AccessDenied`), `Message` (human sentence), and `ResourceIds[]`.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

Copied verbatim from `docs/attention-signals.md` row `eks`, Wave 3 cell:

- OUT OF SCOPE: EKS version EOL calendar vs cluster `version`.
- OUT OF SCOPE: addon health.

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

One row per signal from §3. All EKS signals are Wave 2 because `ListClusters` is opaque; the Describe pass sets the row color, so these behave like Wave 1 colors to the operator (yellow/red is the attention signal, S3 suppressed because the row is not green):

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `Status == CREATING` | 2 | Warning | n/a | S2, S4 | `creating` | `Cluster is being provisioned by EKS; control-plane not yet reachable.` |
| `Status == UPDATING` | 2 | Warning | n/a | S2, S4 | `updating` | `Cluster update in progress (version, endpoint access, or logging change).` |
| `Status == DELETING` | 2 | Warning | n/a | S2, S4 | `deleting` | `Cluster is being deleted; workloads are being torn down.` |
| `Status == PENDING` | 2 | Warning | n/a | S2, S4 | `pending` | `Cluster create or update is queued; EKS has not started the operation.` |
| `Status == FAILED` | 2 | Broken | n/a | S2, S4 | `failed: see Health.Issues` | `Cluster is in FAILED state; see Health.Issues for the AWS-reported cause.` |
| `Health.Issues[]` non-empty | 2 | Broken | n/a | S2, S4, S5 | `issue: <Issue.Code>` | `<Issue.Code>: <Issue.Message>` (first issue; detail lists all). |

Notes:

- No `!`-on-green case exists for EKS: every Wave 2 signal moves the row off green (Warning or Broken). S1 count is driven by Broken rows (`Status == FAILED` and `Health.Issues[]` non-empty) under the standard "red rows bump the menu count" rule.
- `Health.Issues[]` can appear on an `ACTIVE` cluster (health is tracked independently of lifecycle state). When it does, the row moves to Broken — the health issue is the cause, and the `!`-on-green rule does not apply because the color already changed.
- S4 wording for `Status == FAILED` deliberately points the operator at `Health.Issues` — a bare `failed` keyword is not enough, and `StatusReason`-style fields do not exist on `types.Cluster`.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — a transitional state renders as `creating` / `updating` / `deleting` / `pending` (color alone is not enough, so the word clarifies which transition), a failed cluster renders `failed: see Health.Issues`, and a health issue renders the issue code directly (`issue: InsufficientFreeAddresses`, `issue: AccessDenied`). The `Message` detail is one keypress away in S5 for the long-form explanation.

## 5. Out of Scope

- All §3.3 Wave 3 signals: EKS version EOL calendar, addon health.
- Any UI element not listed in §4 — no new columns, icons, views, or key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- eks related-panel targets `alarm`, `ami`, `asg`, `cfn`, `ct-events`, `ec2`, `kms`, `logs`, `ng`, `role`, `sg`, `subnet`, `vpc` — `docs/related-resources.md` § Per-type contract, row `eks`.
- eks Wave 1 is `None` (list API is name-only) — `docs/attention-signals.md` § Signals, row `eks` Wave 1 cell.
- eks Wave 2 status mapping (`ACTIVE`→Healthy; `CREATING`/`UPDATING`/`DELETING`/`PENDING`→Warning; `FAILED`→Broken) and `Health.Issues[]` non-empty → Broken — `docs/attention-signals.md` § Signals, row `eks` Wave 2 cell.
- `ListClusters` returns cluster name strings only — `docs/attention-signals.md` § Signals, row `eks` Wave 1 cell; corroborated by AWS API Reference: `ListClusters` response shape.
- `Cluster.Status` exists on the Describe response — `AWS SDK Go v2 — service/eks/types.Cluster § Status` (type `ClusterStatus`).
- `Cluster.Health.Issues[]` with `Code` (`ClusterIssueCode`), `Message`, `ResourceIds[]` — `AWS SDK Go v2 — service/eks/types.Cluster § Health`, `types.ClusterHealth § Issues`, `types.ClusterIssue § Code, Message, ResourceIds`.
- `Cluster.RoleArn`, `Cluster.ResourcesVpcConfig.VpcId`, `SubnetIds`, `ClusterSecurityGroupId`, `SecurityGroupIds` — `AWS SDK Go v2 — service/eks/types.Cluster § RoleArn, ResourcesVpcConfig`, `types.VpcConfigResponse § VpcId, SubnetIds, ClusterSecurityGroupId, SecurityGroupIds`.
- `Cluster.EncryptionConfig[].Provider.KeyArn` — `AWS SDK Go v2 — service/eks/types.Cluster § EncryptionConfig`, `types.EncryptionConfig § Provider`.
- `Cluster.Tags` carries CFN stack ownership via `aws:cloudformation:stack-name` — `AWS SDK Go v2 — service/eks/types.Cluster § Tags`.
- Discovery of `alarm` via loaded-list cross-ref on dimension `ClusterName` — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. ClusterName is the standard dimension in the AWS/EKS namespace; operators rely on named alarms for control-plane incidents.`
- Discovery of `ec2` via tag cross-ref `kubernetes.io/cluster/<name>=owned` or `eks:cluster-name=<name>` — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. Both tags are written by EKS/eksctl on worker nodes; the pivot is a pure filter against the already-loaded ec2 list.`
- Discovery of `cfn` via the `aws:cloudformation:stack-name` tag — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. CFN propagates this tag to every stack-managed resource, including clusters created via eksctl.`
- Discovery of `logs` via deterministic name `/aws/eks/<clusterName>/cluster` — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. EKS uses this fixed pattern for control-plane log groups; daily-drivers look here first when the API or authenticator misbehaves.`
- Discovery of `ami` and `asg` via indirect hop through `ng` (aggregated across node groups) — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. Neither AMI nor backing ASG is on the Cluster shape; the cluster→ami and cluster→asg pivots are two-hop aggregates, reachable only when node groups are also loaded.`
- Discovery of `ng` via cluster-scoped node-group list — `docs/related-resources.md` § Per-target reasoning, `eks` row: "`ng` — Node groups attached to the cluster."
- `ct-events` is the universal pivot applied to every registered type — `docs/related-resources.md` § Policy.
- a9s is read-only — `docs/architecture.md` § "What is a9s?".
- Superseded HOW ignored — row middle-dot `·` marker, `⚠ Background Check` detail header, and derived list-level banner in `docs/enrichment-visibility.md` are not cited or reproduced per the skill's S1–S5 rules.

<!-- BEGIN GENERATED: header -->
eks — CONTAINERS. Lifecycle key: `status`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Approximate? |
| --- | --- | --- |
| ng | Node Groups | yes |
| alarm | CloudWatch Alarms | yes |
| cfn | CloudFormation Stacks | yes |
| logs | Log Groups | yes |
| sg | Security Groups | no |
| vpc | VPC | no |
| role | IAM Role | no |
| kms | KMS Key | no |
| subnet | Subnets | no |
| ami | AMI | no |
| asg | Auto Scaling Groups | yes |
| ec2 | EC2 Instances | no |
| ct-events | CloudTrail Events | yes |
<!-- END GENERATED: related -->
