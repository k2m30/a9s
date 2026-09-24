---
shortName: eks
name: EKS Clusters
awsApiRef: https://docs.aws.amazon.com/eks/latest/APIReference/API_Cluster.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
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

Expected targets from `docs/related-resources.md` § Per-type contract: `alarm`, `ami`, `asg`, `cfn`, `ec2`, `kms`, `logs`, `ng`, `role`, `sg`, `subnet`, `vpc`, `ct-events`.

### `alarm`

- **Why related**: CloudWatch alarms watching cluster or control-plane metrics — first signal of impact on the EKS control plane.
- **How discovered**: cross-reference the already-loaded `alarm` list; keep alarms whose `Dimensions[]` has `Name==ClusterName` AND `Value==Cluster.Name` — a9s-devops: `ClusterName` is the standard CloudWatch dimension for the `AWS/EKS` namespace; no extra API call needed once the `alarm` list is loaded in the same sweep.
- **Count shown**: yes.

### `ami`

- **Why related**: AMIs applied to this cluster's worker nodes — used when auditing AMI drift or chasing a bad image.
- **How discovered**: the `ImageId` of every tagged node, plus the launch-template `ImageId` of each managed node group that has one (a managed group without a launch template records only `AmiType`/`ReleaseVersion`; its AMI is read off its nodes). The cluster's nodes are found in two places, and the three node pivots (`ec2`, `ami`, `asg`) read both. Managed node groups: `ListNodegroups`/`DescribeNodegroup` → `Nodegroup.Resources.AutoScalingGroups[]` → `DescribeAutoScalingGroups` members, and each group's launch-template `ImageId`. Tagged nodes: `DescribeInstances` (with `IncludeManagedResources`, pending/running/stopping/stopped only) for each of `kubernetes.io/cluster/<Cluster.Name>=owned` (self-managed node template, Karpenter), `eks:eks-cluster-name=<Cluster.Name>` (Karpenter, EKS Auto Mode) and `eks:cluster-name=<Cluster.Name>` (managed node group instances) — self-managed, Karpenter and Auto Mode nodes exist in no node group, and these tags are the only link AWS keeps. A cluster with neither is a proven 0; a node source that could not be read, or a tag query that was refused while the others answered, makes the count a lower bound that keeps every node read, never a proven 0.
- **Count shown**: yes.

### `asg`

- **Why related**: Backing Auto Scaling Groups — where worker-node capacity actually lives; scale-in/scale-out events appear on the ASG.
- **How discovered**: the ASG names on the cluster's `ng` entries (`Nodegroup.Resources.AutoScalingGroups[]`), plus the `aws:autoscaling:groupName` tag of every tagged node (self-managed node groups are plain ASGs). See `ec2` for how tagged nodes are read.
- **Count shown**: yes.

### `cfn`

- **Why related**: CloudFormation stack that created the cluster — infra-as-code linkage for `eksctl` and many in-house IaC flows.
- **How discovered**: read `Cluster.Tags["aws:cloudformation:stack-name"]`; cross-reference the loaded `cfn` list by `StackName` — a9s-devops: `aws:cloudformation:stack-name` is the standard CFN-owner tag propagated to every stack-managed resource, including EKS clusters created via CloudFormation or `eksctl` (which wraps CFN).
- **Count shown**: yes (typically 0 or 1).

### `ec2`

- **Why related**: Worker-node EC2 instances running this cluster's pods — where SSH, console-output, and status-check troubleshooting lands.
- **How discovered**: the members of each managed node group's ASGs, plus every tagged node. The cluster's nodes are found in two places, and the three node pivots (`ec2`, `ami`, `asg`) read both. Managed node groups: `ListNodegroups`/`DescribeNodegroup` → `Nodegroup.Resources.AutoScalingGroups[]` → `DescribeAutoScalingGroups` members, and each group's launch-template `ImageId`. Tagged nodes: `DescribeInstances` (with `IncludeManagedResources`, pending/running/stopping/stopped only) for each of `kubernetes.io/cluster/<Cluster.Name>=owned` (self-managed node template, Karpenter), `eks:eks-cluster-name=<Cluster.Name>` (Karpenter, EKS Auto Mode) and `eks:cluster-name=<Cluster.Name>` (managed node group instances) — self-managed, Karpenter and Auto Mode nodes exist in no node group, and these tags are the only link AWS keeps. A cluster with neither is a proven 0; a node source that could not be read, or a tag query that was refused while the others answered, makes the count a lower bound that keeps every node read, never a proven 0.
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
- **How discovered**: Cluster.RoleArn — EKS service role — and an Auto Mode cluster's `ComputeConfig.NodeRoleArn`, the role its nodes run as ([API_ComputeConfigResponse](https://docs.aws.amazon.com/eks/latest/APIReference/API_ComputeConfigResponse.html)).
- **Count shown**: yes (1, or 2 on an Auto Mode cluster).

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
- Universal pivot — applies to every registered type; see `docs/related-resources.md` §Policy.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeCluster](https://docs.aws.amazon.com/eks/latest/APIReference/API_DescribeCluster.html)

Transcribed from `docs/attention-signals.md § Signals § CONTAINERS` row `eks`.

### 3.1 Wave 1 — zero extra API calls

`ListClusters` returns cluster name strings only, so the fetcher reads each
cluster with `DescribeCluster` before it builds the row. Every signal below is
computed from that response as the row is built, with no second pass — a
cluster that is `ACTIVE` and reports nothing else raises no signal and renders
green and blank.

One bullet per distinct signal.

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
  - **State bucket**: Warning.
  - **API call**: `DescribeCluster` per cluster (same call — `Health` is on the Describe shape). Each `ClusterIssue` carries `Code` (enum, e.g. `AccessDenied`), `Message` (human sentence), and `ResourceIds[]`; every reported code becomes a row under the finding.
  - **Cost shape**: per-resource.

- **Signal**: the cluster's Kubernetes endpoint answers from the whole public internet (`PublicAccessCidrs` contains a `/0`).
  - **State bucket**: Broken.
  - **API call**: same `DescribeCluster` — `ResourcesVpcConfig.EndpointPublicAccess` with `PublicAccessCidrs`.
  - **Cost shape**: per-resource.

- **Signal**: the endpoint is public but scoped to the listed address ranges.
  - **State bucket**: Warning.
  - **API call**: same `DescribeCluster` — `ResourcesVpcConfig.EndpointPublicAccess` with `PublicAccessCidrs`.
  - **Cost shape**: per-resource.

- **Signal**: not all five control-plane log types are sent to CloudWatch.
  - **State bucket**: Warning.
  - **API call**: same `DescribeCluster` — `Logging.ClusterLogging`.
  - **Cost shape**: per-resource.

- **Signal**: no KMS key covers the cluster's Kubernetes secrets, on a cluster below Kubernetes 1.28 — from 1.28 AWS envelope-encrypts secrets with an AWS-owned key on every cluster, so an absent encryption configuration there means no customer-managed key rather than no encryption.
  - **State bucket**: Warning.
  - **API call**: same `DescribeCluster` — `EncryptionConfig[].Resources`.
  - **Cost shape**: per-resource.

- **Signal**: the cluster's Kubernetes minor is past standard support.
  - **State bucket**: Broken.
  - **API call**: same `DescribeCluster` for `Version`, compared against `DescribeClusterVersions` — one call per sweep, shared by every cluster.
  - **Cost shape**: per-sweep.

- **Signal**: `DescribeCluster` was denied for this cluster.
  - **State bucket**: Warning.
  - **API call**: the same describe; only the cluster name is visible after it.
  - **Cost shape**: per-resource.

- **Signal**: `DescribeCluster` answered with nothing usable.
  - **State bucket**: Warning.
  - **API call**: the same describe; only the cluster name is visible after it.
  - **Cost shape**: per-resource.

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals: no enricher runs a second pass over a cluster.

### 3.3 Wave 3 — OUT OF SCOPE

Copied verbatim from `docs/attention-signals.md § Not yet implemented`:

- OUT OF SCOPE: EKS version EOL calendar vs cluster `version`.
- OUT OF SCOPE: addon health.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `eks`: Wave 1 issue-colored rows only — this type registers no Wave 2 enricher, so nothing else bumps the count.
<!-- END GENERATED: badge -->

One row per signal from §3. The fetcher's own `DescribeCluster` sets the row color, so every signal is Wave 1:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `Status == CREATING` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `creating` |
| `Status == UPDATING` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `updating` |
| `Status == DELETING` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `deleting` |
| `Status == PENDING` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `pending` |
| `Status == FAILED` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `failed` |
| `Health.Issues[]` non-empty | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `health issue` |
| endpoint open to the internet | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `cluster endpoint reachable from the internet` |
| endpoint public but scoped to named ranges | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `cluster endpoint reachable from listed networks` |
| control-plane log types missing | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `control plane logging incomplete` |
| no KMS key over secrets, below Kubernetes 1.28 | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `secrets not encrypted with KMS` |
| Kubernetes minor past standard support | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `Kubernetes <version> is out of standard support` |
| describe denied | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `details denied` |
| describe answered with nothing | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `details unavailable` |

Notes:

- Every EKS signal moves the row off green (Warning or Broken). The S1 count is driven by the Broken rows, `Status == FAILED` among them, under the standard "red rows bump the menu count" rule.
- `Health.Issues[]` can appear on an `ACTIVE` cluster (health is tracked independently of lifecycle state). When it does, the row moves to Warning — the health issue is the cause, and each reported code is a row under the finding in the detail view.
- A failed cluster reads `failed` in the list; every health issue AWS reports for it is a supporting row in the detail view, which is where the codes are read. `types.Cluster` carries no `StatusReason`-style field to name a cause in the list.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — a transitional state renders as `creating` / `updating` / `deleting` / `pending` (colour alone is not enough, so the word clarifies which transition), a failed cluster renders `failed`, and a cluster AWS reports a problem on renders `health issue`. The issue's own `Message` is one keypress away in S5 for the long-form explanation.

## 5. Out of Scope

- All §3.3 Wave 3 signals: EKS version EOL calendar, addon health.
- Any UI element not listed in §4 — no new columns, icons, views, or key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- eks related-panel targets `alarm`, `ami`, `asg`, `cfn`, `ct-events`, `ec2`, `kms`, `logs`, `ng`, `role`, `sg`, `subnet`, `vpc` — `docs/related-resources.md` § Per-type contract, row `eks`.
- `ListClusters` is name-only, so every `eks` signal comes from the per-cluster describe — `docs/attention-signals.md § Signals § CONTAINERS` row `eks`.
- `eks` status mapping (`ACTIVE`→Healthy; `CREATING`/`UPDATING`/`DELETING`/`PENDING`→Warning; `FAILED`→Broken) and the health-issue finding — `docs/attention-signals.md § Signals § CONTAINERS` row `eks`.
- `ListClusters` returns cluster name strings only — `core/aws/eks.go`; corroborated by AWS API Reference: `ListClusters` response shape.
- `Cluster.Status` exists on the Describe response — `AWS SDK Go v2 — service/eks/types.Cluster § Status` (type `ClusterStatus`).
- `Cluster.Health.Issues[]` with `Code` (`ClusterIssueCode`), `Message`, `ResourceIds[]` — `AWS SDK Go v2 — service/eks/types.Cluster § Health`, `types.ClusterHealth § Issues`, `types.ClusterIssue § Code, Message, ResourceIds`.
- `Cluster.RoleArn`, `Cluster.ResourcesVpcConfig.VpcId`, `SubnetIds`, `ClusterSecurityGroupId`, `SecurityGroupIds` — `AWS SDK Go v2 — service/eks/types.Cluster § RoleArn, ResourcesVpcConfig`, `types.VpcConfigResponse § VpcId, SubnetIds, ClusterSecurityGroupId, SecurityGroupIds`.
- `Cluster.EncryptionConfig[].Provider.KeyArn` — `AWS SDK Go v2 — service/eks/types.Cluster § EncryptionConfig`, `types.EncryptionConfig § Provider`.
- `Cluster.Tags` carries CFN stack ownership via `aws:cloudformation:stack-name` — `AWS SDK Go v2 — service/eks/types.Cluster § Tags`.
- Discovery of `alarm` via loaded-list cross-ref on dimension `ClusterName` — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. ClusterName is the standard dimension in the AWS/EKS namespace; operators rely on named alarms for control-plane incidents.`
- Node tags — `kubernetes.io/cluster/<name>=owned`: the self-managed node template `amazon-eks-nodegroup.yaml` (`PropagateAtLaunch: true`), <https://docs.aws.amazon.com/eks/latest/userguide/launch-workers.html>, and Karpenter's default tags, <https://karpenter.sh/docs/concepts/nodeclasses/>. `eks:eks-cluster-name`: the same Karpenter page, and the Auto Mode cluster role's `ec2:RunInstances` condition `aws:RequestTag/eks:eks-cluster-name`, <https://docs.aws.amazon.com/eks/latest/userguide/auto-cluster-iam-role.html>. Auto Mode instances are hidden from `DescribeInstances` unless `IncludeManagedResources` is set, <https://docs.aws.amazon.com/eks/latest/userguide/automode-learn-instances.html>. `eks:cluster-name`: on managed node group instances next to `eks:nodegroup-name`, read from `DescribeInstances` on live accounts; the EKS user guide does not list it.
- Discovery of `cfn` via the `aws:cloudformation:stack-name` tag — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. CFN propagates this tag to every stack-managed resource, including clusters created via eksctl.`
- Discovery of `logs` via deterministic name `/aws/eks/<clusterName>/cluster` — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. EKS uses this fixed pattern for control-plane log groups; daily-drivers look here first when the API or authenticator misbehaves.`
- Discovery of `ami`, `asg` and `ec2` from the cluster's nodes (see §2) — neither AMI, ASG nor instance is on the `Cluster` shape. The managed node groups are read with `ListNodegroups`/`DescribeNodegroup` (`Nodegroup.Resources.AutoScalingGroups[]`, `Nodegroup.LaunchTemplate`), the same read for all three pivots; nodes outside managed node groups are found by their instance tags (the node-tag citation above), and `aws:autoscaling:groupName` names a tagged node's ASG — `AWS SDK Go v2 — service/eks/types.Nodegroup § Resources, LaunchTemplate`.
- Discovery of `ng` via cluster-scoped node-group list — `docs/related-resources.md` § Per-target reasoning, `eks` row: "`ng` — Node groups attached to the cluster."
- `ct-events` is the universal pivot applied to every registered type — `docs/related-resources.md` § Policy.
- a9s is read-only — `docs/architecture.md` § "What is a9s?".
- Superseded HOW ignored — row middle-dot `·` marker, `⚠ Background Check` detail header, and derived list-level banner in `docs/historical/analysis/enrichment-visibility.md` are not cited or reproduced per the skill's S1–S5 rules.

<!-- BEGIN GENERATED: header -->
eks — CONTAINERS. Status key: `status` — the key the status cell reads, and the column naming it is the status column.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| eks.state.creating | creating | warn | wave1 | The control plane is still being built, so kubectl and the node groups that will join it cannot connect yet. Wait for the cluster to become active before creating node groups or add-ons against it. |
| eks.state.updating | updating | warn | wave1 | A control-plane change is in flight — a version upgrade, a logging or endpoint change — and other cluster updates are rejected while it runs. Wait for it to finish, then confirm your nodes and add-ons are on a compatible version. |
| eks.state.deleting | deleting | warn | wave1 | The cluster is being torn down and its workloads are going with it. Move anything that still has to run to another cluster now, because the control plane stops answering before the deletion finishes. |
| eks.state.pending | pending | warn | wave1 | The control plane exists but is not serving, so nothing can be scheduled on the cluster yet. Wait for it to become active; a cluster that stays pending usually cannot reach the subnets it was given. |
| eks.state.failed | failed | broken | wave1 | The cluster is in a failed state and will not recover on its own; every health issue AWS reports is a row under this finding. Open a support case or recreate the cluster. |
| eks.health-issue | health issue | warn | wave1 | The control plane reports at least one health issue; every reported code is a row under this finding, and the EKS console carries the message. Add-ons and nodes may misbehave until it clears. |
| eks.public-endpoint | cluster endpoint reachable from the internet | broken | wave1 | The cluster's Kubernetes endpoint answers from the public internet, so its authentication is the only thing between the control plane and every scanner on the network. Turn off public endpoint access and reach the cluster over the VPC, or at minimum restrict public access to the office and build ranges. |
| eks.public-endpoint-restricted | cluster endpoint reachable from listed networks | warn | wave1 | The cluster's Kubernetes endpoint is public but only the listed address ranges may reach it, so the exposure is bounded by a list somebody has to keep correct. Check the ranges are still the ones you meant, and prefer reaching the cluster over the VPC. |
| eks.control-plane-logging-off | control plane logging incomplete | warn | wave1 | Some control-plane log types are not being sent to CloudWatch, so an authentication attempt or an admission decision made during an incident leaves no record to investigate. Enable all five control-plane log types on the cluster. |
| eks.secrets-not-kms | secrets not encrypted with KMS | warn | wave1 | This cluster's Kubernetes version does not envelope-encrypt secrets on its own, and no customer managed key is attached, so secrets sit in etcd with only the platform's own protection. Attach a KMS key to the cluster's secrets encryption configuration, or upgrade to a version that encrypts by default and attach a key if you need to control it yourself. |
| eks.version-unsupported | Kubernetes <version> is out of standard support | broken | wave1 | This Kubernetes minor is past standard support, so it no longer receives the full patch stream and AWS will upgrade it on its own schedule if you do not. Plan an upgrade to a version in standard support before the automatic one lands during business hours. |
| eks.warn.details\_denied | details denied | warn | wave1 | The per-item describe call for this row was denied, so a9s can show its name and nothing about its posture — the row is unjudged, not healthy. Grant the read-only describe permission for this type to the role you browse with, then refresh. |
| eks.warn.details\_unavailable | details unavailable | warn | wave1 | The per-item describe call for this row failed, so a9s can show its name and nothing about its posture — the row is unjudged, not healthy. Retry the refresh; if it persists, check the service's health and whether the call is being throttled. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
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
| ami | AMI | yes |
| asg | Auto Scaling Groups | yes |
| ec2 | EC2 Instances | yes |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
