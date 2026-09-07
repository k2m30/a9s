---
shortName: ng
name: EKS Node Groups
awsApiRef: https://docs.aws.amazon.com/eks/latest/APIReference/API_Nodegroup.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# ng — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `ng`
- **Display name**: EKS Node Groups
- **AWS API reference**: <https://docs.aws.amazon.com/eks/latest/APIReference/API_Nodegroup.html>
- **List API**: `ListNodegroups` (per cluster — requires pre-loaded EKS cluster list to enumerate)
- **Describe API (if any)**: `DescribeNodegroup` (Wave 2, one call per node group — `ListNodegroups` returns name strings only).

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `ami`, `asg`, `ct-events`, `ebs`, `ec2`, `eks`, `role`, `sg`, `subnet`.

### `ami`

- **Why related**: node groups run a specific EKS-optimized AMI; operator checks it to confirm patch level, compare against latest approved image, or diagnose boot-time failures after an AMI drift.
- **How discovered**: for node groups without a custom launch template, `Nodegroup.ReleaseVersion` identifies the AWS-managed AMI alias directly. For node groups with a custom launch template, `Nodegroup.LaunchTemplate.{Id,Version}` resolves to an `ImageId` via `ec2:DescribeLaunchTemplateVersions` — a9s-devops: the field is only populated when a launch template was supplied at create time.
- **Count shown**: yes (0 or 1 — a node group pins exactly one AMI).

### `asg`

- **Why related**: the backing Auto Scaling group is where actual capacity changes, scaling activities, and instance-launch failures surface. When a node group is Broken, the ASG panel is the next place an operator looks.
- **How discovered**: `Nodegroup.Resources.AutoScalingGroups[].Name` — direct field on the Describe response.
- **Count shown**: yes (typically 1).

### `ct-events`

- **Why related**: audit trail for node group lifecycle changes (create, update-config, update-version, delete) and for "who scaled it" during an incident.
- **How discovered**: `LookupEvents` filtered by `ResourceName==NodegroupName` and/or node group ARN, scoped to the EKS event source — a9s-devops: standard CloudTrail pivot for every registered resource type.
- **Count shown**: yes.
- **Note**: universal pivot — applies to every registered type; see related-resources.md §Policy.

### `ebs`

- **Why related**: root and data volumes attached to worker nodes — capacity, IOPS, encryption posture. Operators pivot here when disk-pressure evictions or storage-full issues appear on the cluster.
- **How discovered**: zero-call cache join — cached EC2 instances tagged `eks:nodegroup-name` (with the `eks:cluster-name` guard, same match as the `ec2` pivot) → `Instance.BlockDeviceMappings[].Ebs.VolumeId`, deduplicated. Cold or missing EC2 cache → `?`; truncated cache with no match → truncated zero.
- **Count shown**: yes (sum across all worker-node instances).

### `ec2`

- **Why related**: individual worker-node instances — state, IP, SSM reachability. When a node group is `DEGRADED` the operator wants to see which specific nodes are bad.
- **How discovered**: node group → `Resources.AutoScalingGroups[0].Name` → `autoscaling:DescribeAutoScalingGroups.Instances[].InstanceId` (cross-reference the already-loaded `ec2` list when present) — a9s-devops: pivot via ASG is the only first-class path; EKS API does not list nodes directly.
- **Count shown**: yes (equals the ASG's in-service count).

### `eks`

- **Why related**: the parent cluster — node group health is meaningless without cluster context (version, endpoint reachability, control-plane state).
- **How discovered**: `Nodegroup.ClusterName` — direct field.
- **Count shown**: yes (always 1).

### `role`

- **Why related**: the IAM role worker nodes assume. Missing or misconfigured permissions here manifest as `NodeCreationFailure` or `AccessDenied` health issues; operators pivot here to inspect or attach policies.
- **How discovered**: `Nodegroup.NodeRole` (role ARN) — direct field.
- **Count shown**: yes (always 1).

### `sg`

- **Why related**: the security groups attached to worker-node ENIs — first stop when pods cannot reach the control plane or when SSH access is misconfigured.
- **How discovered**: two fields on the Describe response, showing two different SG sets — a9s-devops: worth surfacing together. `Resources.RemoteAccessSecurityGroup` is the EKS-managed SG attached to the nodes' ENIs for remote access. `RemoteAccess.SourceSecurityGroups[]` is the list of *client* SGs permitted to SSH into nodes (populated only when `RemoteAccess` was configured). The primary data-plane SG for pod traffic is inherited from the cluster's `resourcesVpcConfig` and is discoverable via the parent `eks` record, not the node group itself.
- **Count shown**: yes (union of the two fields, deduplicated).

### `subnet`

- **Why related**: the subnets the node group launches nodes into — used when diagnosing AZ placement, `InsufficientFreeAddresses`, or a misconfigured public subnet that should have been private.
- **How discovered**: `Nodegroup.Subnets[]` — direct field.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeNodegroup](https://docs.aws.amazon.com/eks/latest/APIReference/API_DescribeNodegroup.html)

Transcribed from `docs/attention-signals.md § Signals § CONTAINERS` row `ng`.

### 3.1 Wave 1 — zero extra API calls

`ListNodegroups` returns node-group name strings only, so the fetcher reads each node group with `DescribeNodegroup` before it builds the row. Every signal below is computed from that response as the row is built, with no second pass.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal. Each signal is derived from the `DescribeNodegroup` response.

An `ACTIVE` node group with nothing else wrong raises no signal and renders
green and blank.

- **Signal**: `status==CREATING`.
  - **State bucket**: Warning.
  - **API call**: `DescribeNodegroup` — one call per node group.
  - **Cost shape**: per-resource.

- **Signal**: `status==UPDATING`.
  - **State bucket**: Warning.
  - **API call**: `DescribeNodegroup` — one call per node group.
  - **Cost shape**: per-resource.

- **Signal**: `status==DELETING`.
  - **State bucket**: Warning.
  - **API call**: `DescribeNodegroup` — one call per node group.
  - **Cost shape**: per-resource.

- **Signal**: `status==CREATE_FAILED`.
  - **State bucket**: Broken.
  - **API call**: `DescribeNodegroup` — one call per node group.
  - **Cost shape**: per-resource.

- **Signal**: `status==DELETE_FAILED`.
  - **State bucket**: Broken.
  - **API call**: `DescribeNodegroup` — one call per node group.
  - **Cost shape**: per-resource.

- **Signal**: `status==DEGRADED`. Every code in `health.issues[]` becomes a row under the finding, so the detail view names what AWS reported.
  - **State bucket**: Broken.
  - **API call**: `DescribeNodegroup` — same call as above; no additional request.
  - **Cost shape**: per-resource.

- **Signal**: `health.issues[]` is non-empty on a node group whose `status` says nothing is wrong (`ACTIVE`). Health is tracked independently of the lifecycle state, the same way it is for the cluster, and every reported code becomes a row under the finding.
  - **State bucket**: Warning.
  - **API call**: `DescribeNodegroup` — same call as above; no additional request.
  - **Cost shape**: per-resource.

- **Signal**: `DescribeNodegroup` was denied for this node group.
  - **State bucket**: Warning.
  - **API call**: the same describe; only the node-group name is visible after it.
  - **Cost shape**: per-resource.

- **Signal**: `DescribeNodegroup` answered with nothing usable.
  - **State bucket**: Warning.
  - **API call**: the same describe; only the node-group name is visible after it.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

The reads a9s does not make for a node group, listed on `docs/attention-signals.md § Not yet implemented`. They are here so the reader knows what is intentionally excluded; they are not to be implemented.

- OUT OF SCOPE: AMI release drift.
- OUT OF SCOPE: `ListUpdates` per node group.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `ng`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3 that reaches at least one surface. Healthy is omitted (silence is the UX).

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `status==CREATING` | 1 | Warning | n/a | S1, S2, S4 | `creating` |
| `status==UPDATING` | 1 | Warning | n/a | S1, S2, S4 | `updating` |
| `status==DELETING` | 1 | Warning | n/a | S1, S2, S4 | `deleting` |
| `status==CREATE_FAILED` | 1 | Broken | n/a | S1, S2, S4, S5 | `create failed` |
| `status==DELETE_FAILED` | 1 | Broken | n/a | S1, S2, S4, S5 | `delete failed` |
| `status==DEGRADED` | 1 | Broken | n/a | S1, S2, S4, S5 | `degraded` |
| `health.issues[]` non-empty on an `ACTIVE` group | 1 | Warning | n/a | S1, S2, S4, S5 | `health issue` |
| describe denied | 1 | Warning | n/a | S1, S2, S4 | `details denied` |
| describe answered with nothing | 1 | Warning | n/a | S1, S2, S4 | `details unavailable` |

The two health rows carry what AWS reported: each code in `health.issues[]` is a
row under the finding in the detail view, so `insufficient free addresses` is
read there rather than squeezed into the Status column. S3 is suppressed on all
of these — the row is never green when one fires — and the Broken rows count
toward S1.

Healthy (`status==ACTIVE` with no issues) is omitted from the table: S2 renders green, S4 renders blank, no finding. Silence is the UX.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — a red row reads `create failed`, `delete failed` or `degraded` and a yellow one reads `health issue`, which is enough to pick the group to open; the codes AWS reported are rows under the finding in the detail view, and that is where the next pivot (→ subnet, → role, → asg) is chosen.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): AMI release drift, `ListUpdates` per node group.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- `ng → kms` — listed under Explicitly-excluded pairs in `docs/related-resources.md`: no direct KMS field on a node group.

## 6. Citations

- Display name `EKS Node Groups` and the node-group signals — `docs/attention-signals.md § Signals § CONTAINERS` row `ng`.
- Per-type contract for `ng` (`ami`, `asg`, `ct-events`, `ebs`, `ec2`, `eks`, `role`, `sg`, `subnet`) — `docs/related-resources.md` § Per-type contract, `ng` row.
- Per-target reasoning for every §2 bullet — `docs/related-resources.md` § `ng` (subsection).
- `ng → kms` exclusion — `docs/related-resources.md` § Explicitly excluded → "Unanimous `sometimes`".
- Nodegroup shape and field names (`Status`, `Health.Issues[]`, `Resources.AutoScalingGroups[].Name`, `Resources.RemoteAccessSecurityGroup`, `RemoteAccess.SourceSecurityGroups`, `NodeRole`, `ClusterName`, `Subnets`, `ReleaseVersion`, `LaunchTemplate.{Id,Version}`) — `AWS SDK Go v2 — service/eks/types.Nodegroup`.
- `NodegroupStatus` enum values (`CREATING`, `UPDATING`, `DELETING`, `ACTIVE`, `CREATE_FAILED`, `DELETE_FAILED`, `DEGRADED`) — `AWS SDK Go v2 — service/eks/types.NodegroupStatus`.
- `health.issues[].Code` enum values and human descriptions used for S4/S5 rewrites — `AWS SDK Go v2 — service/eks/types.Issue § Code` (docstring enumerates all NodegroupIssueCode values with plain-English descriptions).
- Wave 2 cost shape (one `DescribeNodegroup` per node group) — `core/aws/ng.go`.
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
- `ami` discovery split (`ReleaseVersion` for EKS-optimized, `LaunchTemplate` + `DescribeLaunchTemplateVersions` for custom) — `a9s-devops (2026-04-20): possible=yes, worth=yes. ReleaseVersion identifies the EKS-managed AMI alias; LaunchTemplate fields are populated only when a custom LT was supplied at create time, and resolving to an ImageId needs DescribeLaunchTemplateVersions. Showing the AMI is valuable for patch-level verification and post-drift diagnosis.`
- `ec2` / `ebs` discovery via ASG → Instances → BlockDeviceMappings — `a9s-devops (2026-04-20): possible=yes, worth=yes. EKS API exposes no direct instance list on a node group; the ASG pivot is the canonical path and is the same one the AWS Console uses. Cheap when ec2 and asg lists are already cached, otherwise fan-out per node group.`
  - Amended 2026-07-06: the "otherwise fan-out" branch violated Policy rule 7 (two AWS calls per checker). Both pivots now use the zero-call variant the citation itself calls cheap: the `eks:nodegroup-name` / `eks:cluster-name` tag join over the cached EC2 list (EKS managed node groups always tag their instances), with `BlockDeviceMappings` read from the cached Instance structs for `ebs`. Cold EC2 cache → `?` instead of a fan-out.

- `sg` split between `Resources.RemoteAccessSecurityGroup` and `RemoteAccess.SourceSecurityGroups` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Operators confuse these two; the first is the SG attached to nodes' ENIs for remote access, the second is the list of client SGs allowed to SSH in. Surfacing both (deduplicated) in the related panel prevents "why can't I SSH?" misdiagnosis. Primary data-plane SG for pod traffic lives on the cluster, not the node group.`
- Health-issue codes as rows under the finding — `a9s-devops (2026-04-20): possible=yes, worth=yes. AWS surfaces the issue Code and Message verbatim; the humanized code is what the operator reads.` The list line stays the finding's own phrase; each reported code is a row in the detail view, so a group with three issues has three to read.

<!-- BEGIN GENERATED: header -->
ng — CONTAINERS. Lifecycle key: `status`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| ng.state.creating | creating | warn | wave1 | — |
| ng.state.updating | updating | warn | wave1 | — |
| ng.state.deleting | deleting | warn | wave1 | — |
| ng.state.create-failed | create failed | broken | wave1 | — |
| ng.state.delete-failed | delete failed | broken | wave1 | — |
| ng.state.degraded | degraded | broken | wave1 | The node group is degraded, so some nodes are failing or not joining; every health issue AWS reports is a row under this finding. Fix the cause, usually IAM, subnet capacity or the launch template, and let the group reconcile. |
| ng.health-issue | health issue | warn | wave1 | The node group reports a health issue while its state says nothing is wrong; every reported code is a row under this finding. Nodes may be failing to join or to stay healthy until it clears. |
| ng.warn.details\_denied | details denied | warn | wave1 | Access to resource details was denied; only the name is visible. |
| ng.warn.details\_unavailable | details unavailable | warn | wave1 | Details could not be retrieved; only the name is visible. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| eks | EKS Clusters | yes |
| role | IAM Roles | no |
| asg | Auto Scaling Groups | yes |
| ec2 | EC2 Instances | yes |
| sg | Security Groups | no |
| ami | AMI | no |
| ebs | EBS Volumes | yes |
| subnet | Subnets | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
