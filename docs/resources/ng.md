---
shortName: ng
name: EKS Node Groups
awsApiRef: https://docs.aws.amazon.com/eks/latest/APIReference/API_Nodegroup.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
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
- **How discovered**: node group → backing ASG (`Resources.AutoScalingGroups[0].Name`) → `autoscaling:DescribeAutoScalingGroups.Instances[].InstanceId` → `ec2:DescribeInstances.BlockDeviceMappings[].Ebs.VolumeId` — a9s-devops: chained pivot, cheap when ASG and EC2 lists are already loaded, otherwise fan-out.
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

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — the list API does not return fields usable for attention. `ListNodegroups` returns node-group name strings only; every attention signal requires `DescribeNodegroup`.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal. Each signal is derived from the `DescribeNodegroup` response.

- **Signal**: `status==ACTIVE`.
  - **State bucket**: Healthy.
  - **API call**: `DescribeNodegroup` — one call per node group.
  - **Cost shape**: per-resource.

- **Signal**: `status` in `CREATING` / `UPDATING` / `DELETING`.
  - **State bucket**: Warning.
  - **API call**: `DescribeNodegroup` — one call per node group.
  - **Cost shape**: per-resource.

- **Signal**: `status` in `CREATE_FAILED` / `DELETE_FAILED` / `DEGRADED`.
  - **State bucket**: Broken.
  - **API call**: `DescribeNodegroup` — one call per node group.
  - **Cost shape**: per-resource.

- **Signal**: `health.issues[]` contains a code in the broken set — `InsufficientFreeAddresses`, `Ec2LaunchTemplateVersionMismatch`, `AutoScalingGroupInvalidConfiguration`, `AccessDenied`, `Ec2SecurityGroupDeletionFailure`, `Ec2SecurityGroupNotFound`, `IamInstanceProfileNotFound`, `IamNodeRoleNotFound`, `InstanceLimitExceeded`, `NodeCreationFailure`, `ClusterUnreachable`, `Ec2LaunchTemplateNotFound`, `AsgInstanceLaunchFailures`, `AutoScalingGroupNotFound`, `Ec2SubnetInvalidConfiguration`, `Ec2InstanceTypeDoesNotExist`, `InternalFailure`.
  - **State bucket**: Broken.
  - **API call**: `DescribeNodegroup` — same call as above; no additional request.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

Copied verbatim from the `ng` row's Wave 3 cell. Documented so the reader knows what is intentionally excluded from a9s; these are not to be implemented.

- OUT OF SCOPE: AMI release drift.
- OUT OF SCOPE: `ListUpdates` per node group.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing." **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause. **Healthy rows render blank** — no `OK` / `ACTIVE`. Empty means "nothing to see." |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

One row per signal from §3 that reaches at least one surface. Healthy is omitted (silence is the UX).

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `status==CREATING` | 2 | Warning | n/a | S2, S4 | `creating` | `Node group is being created; nodes will register with the cluster once ready.` |
| `status==UPDATING` | 2 | Warning | n/a | S2, S4 | `updating` | `Node group is updating — version, scaling, or launch-template change in progress.` |
| `status==DELETING` | 2 | Warning | n/a | S2, S4 | `deleting` | `Node group is being deleted; worker nodes are draining and terminating.` |
| `status==CREATE_FAILED` | 2 | Broken | n/a | S2, S4, S5 | `create failed` | `Node group create failed — see health issues for root cause, then delete and retry.` |
| `status==DELETE_FAILED` | 2 | Broken | n/a | S2, S4, S5 | `delete failed` | `Node group delete failed — backing ASG or ENIs may be stuck; inspect ASG for details.` |
| `status==DEGRADED` | 2 | Broken | n/a | S2, S4, S5 | `degraded: <first issue code, human-readable>` | `Node group degraded: <first issue message from health.issues[0].Message>.` |
| `health.issues[] InsufficientFreeAddresses` | 2 | Broken | n/a | S2, S4, S5 | `no free IPs in subnets` | `Subnets assigned to this node group have no free IP addresses; new nodes cannot launch.` |
| `health.issues[] Ec2LaunchTemplateVersionMismatch` | 2 | Broken | n/a | S2, S4, S5 | `launch template version mismatch` | `EC2 launch template version differs from the one EKS expects; revert or update the node group.` |
| `health.issues[] AutoScalingGroupInvalidConfiguration` | 2 | Broken | n/a | S2, S4, S5 | `ASG misconfigured` | `Backing Auto Scaling group has an invalid configuration; inspect the ASG for the exact setting.` |
| `health.issues[] AccessDenied` | 2 | Broken | n/a | S2, S4, S5 | `access denied to cluster` | `Node group cannot authenticate with the Kubernetes API server — check node IAM role and aws-auth.` |
| `health.issues[] Ec2SecurityGroupDeletionFailure` | 2 | Broken | n/a | S2, S4, S5 | `remote-access SG delete failed` | `Remote-access security group could not be deleted; remove dependencies from the SG to unblock.` |
| `health.issues[] Ec2SecurityGroupNotFound` | 2 | Broken | n/a | S2, S4, S5 | `cluster SG missing` | `Cluster security group is gone; the cluster itself must be recreated.` |
| `health.issues[] IamInstanceProfileNotFound` | 2 | Broken | n/a | S2, S4, S5 | `instance profile missing` | `IAM instance profile for worker nodes is missing; recreate it with the same settings.` |
| `health.issues[] IamNodeRoleNotFound` | 2 | Broken | n/a | S2, S4, S5 | `node IAM role missing` | `Node IAM role is missing; recreate it and reattach worker-node policies.` |
| `health.issues[] InstanceLimitExceeded` | 2 | Broken | n/a | S2, S4, S5 | `EC2 instance limit reached` | `Account cannot launch more of this instance type; request a limit increase or change type.` |
| `health.issues[] NodeCreationFailure` | 2 | Broken | n/a | S2, S4, S5 | `nodes cannot register` | `Launched instances fail to join the cluster — check node IAM permissions and outbound internet.` |
| `health.issues[] ClusterUnreachable` | 2 | Broken | n/a | S2, S4, S5 | `cluster unreachable` | `Nodes cannot reach the Kubernetes API server; check VPC routing, endpoint access, and SGs.` |
| `health.issues[] Ec2LaunchTemplateNotFound` | 2 | Broken | n/a | S2, S4, S5 | `launch template missing` | `EC2 launch template for this node group is gone; recreate it with the original settings.` |
| `health.issues[] AsgInstanceLaunchFailures` | 2 | Broken | n/a | S2, S4, S5 | `ASG launch failures` | `Backing Auto Scaling group is failing to launch instances; inspect scaling activities.` |
| `health.issues[] AutoScalingGroupNotFound` | 2 | Broken | n/a | S2, S4, S5 | `backing ASG missing` | `Backing Auto Scaling group is gone; recreate an ASG with matching settings to recover.` |
| `health.issues[] Ec2SubnetInvalidConfiguration` | 2 | Broken | n/a | S2, S4, S5 | `subnet public-IP setting wrong` | `One or more subnets do not auto-assign public IPs as required; toggle the subnet setting.` |
| `health.issues[] Ec2InstanceTypeDoesNotExist` | 2 | Broken | n/a | S2, S4, S5 | `instance type unavailable` | `Requested instance type is not available in this region; pick a supported type.` |
| `health.issues[] InternalFailure` | 2 | Broken | n/a | S2, S4, S5 | `EKS internal failure` | `Amazon EKS reported a server-side failure; retry the operation or open an AWS support case.` |

All `health.issues[]` rows above are Wave 2 findings on an already-red row (because `DEGRADED` / `CREATE_FAILED` / `DELETE_FAILED` already bucketed Broken). Per §4 mapping rule — S3 glyph is suppressed on non-green rows; S4 surfaces the *cause*, not a redundant state keyword; S5 carries the full operator sentence. S1 still counts these as `!`-severity findings so the menu tally is accurate.

Healthy (`status==ACTIVE` with no issues) is omitted from the table: S2 renders green, S4 renders blank, no finding. Silence is the UX.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every Broken row pairs the state with a specific cause keyword in S4 (`no free IPs in subnets`, `node IAM role missing`, `launch template missing`, …), so the operator can triage and decide the next pivot (→ subnet, → role, → asg) without navigating into detail first.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): AMI release drift, `ListUpdates` per node group.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- `ng → kms` — listed under Explicitly-excluded pairs in `docs/related-resources.md`: no direct KMS field on a node group.

## 6. Citations

- Display name `EKS Node Groups`, Wave 1/2/3 cells, and Source URL — `docs/attention-signals.md` § Containers → `ng` row.
- Per-type contract for `ng` (`ami`, `asg`, `ct-events`, `ebs`, `ec2`, `eks`, `role`, `sg`, `subnet`) — `docs/related-resources.md` § Per-type contract, `ng` row.
- Per-target reasoning for every §2 bullet — `docs/related-resources.md` § `ng` (subsection).
- `ng → kms` exclusion — `docs/related-resources.md` § Explicitly excluded → "Unanimous `sometimes`".
- Nodegroup shape and field names (`Status`, `Health.Issues[]`, `Resources.AutoScalingGroups[].Name`, `Resources.RemoteAccessSecurityGroup`, `RemoteAccess.SourceSecurityGroups`, `NodeRole`, `ClusterName`, `Subnets`, `ReleaseVersion`, `LaunchTemplate.{Id,Version}`) — `AWS SDK Go v2 — service/eks/types.Nodegroup`.
- `NodegroupStatus` enum values (`CREATING`, `UPDATING`, `DELETING`, `ACTIVE`, `CREATE_FAILED`, `DELETE_FAILED`, `DEGRADED`) — `AWS SDK Go v2 — service/eks/types.NodegroupStatus`.
- `health.issues[].Code` enum values and human descriptions used for S4/S5 rewrites — `AWS SDK Go v2 — service/eks/types.Issue § Code` (docstring enumerates all NodegroupIssueCode values with plain-English descriptions).
- Wave 2 cost shape (one `DescribeNodegroup` per node group) — `docs/attention-signals.md` § `ng` row, Wave 2 cell ("N+1").
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
- `ami` discovery split (`ReleaseVersion` for EKS-optimized, `LaunchTemplate` + `DescribeLaunchTemplateVersions` for custom) — `a9s-devops (2026-04-20): possible=yes, worth=yes. ReleaseVersion identifies the EKS-managed AMI alias; LaunchTemplate fields are populated only when a custom LT was supplied at create time, and resolving to an ImageId needs DescribeLaunchTemplateVersions. Showing the AMI is valuable for patch-level verification and post-drift diagnosis.`
- `ec2` / `ebs` discovery via ASG → Instances → BlockDeviceMappings — `a9s-devops (2026-04-20): possible=yes, worth=yes. EKS API exposes no direct instance list on a node group; the ASG pivot is the canonical path and is the same one the AWS Console uses. Cheap when ec2 and asg lists are already cached, otherwise fan-out per node group.`
- `sg` split between `Resources.RemoteAccessSecurityGroup` and `RemoteAccess.SourceSecurityGroups` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Operators confuse these two; the first is the SG attached to nodes' ENIs for remote access, the second is the list of client SGs allowed to SSH in. Surfacing both (deduplicated) in the related panel prevents "why can't I SSH?" misdiagnosis. Primary data-plane SG for pod traffic lives on the cluster, not the node group.`
- S4/S5 cause-text rewrites for each `health.issues[]` code — `a9s-devops (2026-04-20): possible=yes, worth=yes. AWS surfaces the issue Code and Message verbatim; the spec rewrites jargon-free short causes for S4 (<= 40 chars) and one-line operator sentences for S5 (<= 100 chars). Keeping Message as fallback for DEGRADED so runtime detail isn't lost.`
