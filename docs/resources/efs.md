---
shortName: efs
name: EFS File Systems
awsApiRef: https://docs.aws.amazon.com/efs/latest/ug/API_FileSystemDescription.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# efs — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `efs`
- **Display name**: EFS File Systems
- **AWS API reference**: <https://docs.aws.amazon.com/efs/latest/ug/API_FileSystemDescription.html>
- **List API**: `DescribeFileSystems` — returns `FileSystemDescription[]`. The SDK confirms `LifeCycleState`, `NumberOfMountTargets`, `FileSystemId`, `FileSystemArn`, `Name`, `KmsKeyId`, `Encrypted`, `SizeInBytes`, `PerformanceMode`, `ThroughputMode` are all on the description shape, so both Wave 1 signals are reachable with zero extra calls.
- **Describe API (if any)**: `DescribeMountTargets` per file system — used in Wave 2 to read each mount target's `LifeCycleState`, `SubnetId`, `VpcId`, `NetworkInterfaceId`. These fields do not exist on the file-system summary.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `backup`, `cfn`, `ct-events`, `ec2`, `ecs-task`, `eni`, `kms`, `lambda`, `sg`, `subnet`, `vpc`.

### `alarm`

- **Why related**: CloudWatch alarms watching FS metrics — typically `BurstCreditBalance`, `PercentIOLimit`, `PermittedThroughput` — are the first place an operator looks when an EFS-backed workload slows down.
- **How discovered**: cross-reference the already-loaded `alarm` list for alarms with `Namespace=AWS/EFS` and `Dimensions[].Name=FileSystemId, Value=<fs-id>` — a9s-devops: `DescribeAlarms` returns dimensions on each alarm; no extra call needed when the sibling list is already in the sweep.
- **Count shown**: yes.

### `backup`

- **Why related**: AWS Backup recovery points for this file system — the only place operators recover from accidental deletion or corruption, since EFS itself has no snapshot surface.
- **How discovered**: call `backup:ListRecoveryPointsByResource(ResourceArn=FileSystemArn)` — a9s-devops: the FS ARN (`FileSystemDescription.FileSystemArn`) is the exact key Backup indexes recovery points by; one bounded call per FS detail view.
- **Count shown**: yes.

### `cfn`

- **Why related**: CloudFormation stack that created the FS — provenance and blast-radius when planning changes.
- **How discovered**: read the `aws:cloudformation:stack-name` tag on `FileSystemDescription.Tags[]`; cross-reference the loaded `cfn` list by stack name — a9s-devops: EFS tags are returned on the description directly (no extra `ListTagsForResource`); CloudFormation-managed resources always carry this tag. Fall back to "none" if the tag is absent.
- **Count shown**: yes (typically 1).

### `ec2`

- **Why related**: EC2 instances that mount this file system via NFS — the obvious set of consumers.
- **How discovered**: `TBD — a9s-devops (2026-04-20): possible=weak, worth=no. There is no direct EFS→EC2 field; the only inference is "instances whose subnet matches a mount-target subnet", which is noisy (every instance in those subnets appears, not just mounters). Daily operators reach EC2 via the`subnet` or `eni`pivots instead, so a9s should not fabricate a consumer list here.` Render the pivot as an empty panel unless a field-backed mechanism is added upstream.
- **Count shown**: unknown.

### `ecs-task`

- **Why related**: ECS tasks mounting this file system via EFS volume configuration.
- **How discovered**: cross-reference the already-loaded `ecs-task` list for tasks whose task definition has `volumes[].efsVolumeConfiguration.FileSystemId == <fs-id>`. The ecs-task fetcher joins task-definitions upstream — once per unique `TaskDefinitionArn` in the sweep via `DescribeTaskDefinition` — and attaches the resolved `Volumes` onto the task Resource so the EFS checker reads `FileSystemId` values without any per-detail-view call. See §6 citation `user decision (2026-04-24)`.
- **Count shown**: yes.

### `eni`

- **Why related**: Mount-target ENIs — the exact network-interface objects AWS provisions per mount target, one per AZ the FS is mounted in.
- **How discovered**: call `DescribeMountTargets(FileSystemId=<fs-id>)` and read `MountTargetDescription.NetworkInterfaceId` per MT; cross-reference the loaded `eni` list by those IDs.
- **Count shown**: yes (equals `NumberOfMountTargets` on a healthy FS).

### `kms`

- **Why related**: Customer-managed KMS key encrypting the FS at rest — when the key is scheduled for deletion or disabled, every read/write against the FS starts failing.
- **How discovered**: read `FileSystemDescription.KmsKeyId` directly; cross-reference the loaded `kms` list by key ARN/alias.
- **Count shown**: yes (0 or 1).

### `lambda`

- **Why related**: Lambda functions mounting this file system via `FileSystemConfigs` — directly affected when the FS becomes unreachable.
- **How discovered**: cross-reference the already-loaded `lambda` list for functions whose `FunctionConfiguration.FileSystemConfigs[].Arn` matches `FileSystemArn` — a9s-devops: `FileSystemConfigs[].Arn` is an access-point ARN, not the FS ARN directly; match by prefix (access-point ARN embeds the FS id) so both forms resolve.
- **Count shown**: yes.

### `sg`

- **Why related**: Security groups attached to each mount target ENI — the first thing to check when clients get `connection refused` on port 2049.
- **How discovered**: for each mount target from `DescribeMountTargets`, call `DescribeMountTargetSecurityGroups(MountTargetId=<mt-id>)`; union the returned SG IDs and cross-reference the loaded `sg` list. Alternative: read `Groups` from the corresponding `eni` cross-reference (no extra call) — a9s-devops: both paths give the same set; prefer the ENI-join path when the `eni` list is already loaded, fall back to `DescribeMountTargetSecurityGroups` when not.
- **Count shown**: yes.

### `subnet`

- **Why related**: Subnets hosting the mount-target ENIs — an AZ's worth of connectivity for this FS dies when its subnet loses routing.
- **How discovered**: call `DescribeMountTargets(FileSystemId=<fs-id>)` and read `MountTargetDescription.SubnetId` per MT; cross-reference the loaded `subnet` list.
- **Count shown**: yes (equals `NumberOfMountTargets`).

### `vpc`

- **Why related**: The VPC the file system is mounted into — EFS mount targets are VPC-scoped, and a FS in a VPC that's being retired is a FS being retired.
- **How discovered**: call `DescribeMountTargets(FileSystemId=<fs-id>)` and read `MountTargetDescription.VpcId` from any mount target (all MTs of a single FS share one VPC); cross-reference the loaded `vpc` list.
- **Count shown**: yes (typically 1).

### `ct-events`

- **Why related**: Universal pivot — who created, modified, tagged, or deleted this file system; who changed lifecycle / throughput mode.
- **How discovered**: pre-built CloudTrail query scoped to `FileSystemId` as the resource identifier (EFS `elasticfilesystem.amazonaws.com` event source).
- **Count shown**: unknown (CloudTrail queries are windowed; a reliable total isn't available without a separate count call).
- Universal pivot — applies to every registered type; see `related-resources.md` §Policy.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `LifeCycleState == available`.
  - **State bucket**: Healthy.
  - **How obtained**: `FileSystemDescription.LifeCycleState` from `DescribeFileSystems`.

- **Signal**: `LifeCycleState == creating`.
  - **State bucket**: Warning.
  - **How obtained**: `FileSystemDescription.LifeCycleState` from `DescribeFileSystems`.

- **Signal**: `LifeCycleState == updating`.
  - **State bucket**: Warning.
  - **How obtained**: `FileSystemDescription.LifeCycleState` from `DescribeFileSystems`.

- **Signal**: `LifeCycleState == deleting`.
  - **State bucket**: Warning.
  - **How obtained**: `FileSystemDescription.LifeCycleState` from `DescribeFileSystems`.

- **Signal**: `LifeCycleState == error`.
  - **State bucket**: Broken.
  - **How obtained**: `FileSystemDescription.LifeCycleState` from `DescribeFileSystems`.

- **Signal**: `NumberOfMountTargets == 0`.
  - **State bucket**: Broken.
  - **How obtained**: `FileSystemDescription.NumberOfMountTargets` from `DescribeFileSystems`. A FS with no mount targets is unreachable from any client — no NFS endpoint exists in any subnet.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: any mount target `LifeCycleState != available`.
  - **State bucket**: Broken.
  - **API call**: `DescribeMountTargets(FileSystemId=<fs-id>)` — one call per FS.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `PercentIOLimit` — per-FS metric query, exceeds Wave 2 cost envelope.
- OUT OF SCOPE: CloudWatch `BurstCreditBalance` — per-FS metric query, exceeds Wave 2 cost envelope.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause. **Healthy rows render blank**. |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4 (short cause), S5 (full sentence).
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph on green row. S3, S4, S5. No S1.
- **Wave 2 finding on an already yellow/red/dim row** → color is the signal; S3 suppressed, S4 deduplicates with existing cause, S5 still carries the full sentence, S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `LifeCycleState == creating` | 1 | Warning | n/a | S2, S4 | `creating` | `File system is provisioning; mount targets not yet usable.` |
| `LifeCycleState == updating` | 1 | Warning | n/a | S2, S4 | `updating` | `File system configuration change in progress.` |
| `LifeCycleState == deleting` | 1 | Warning | n/a | S2, S4 | `deleting` | `File system is being deleted; clients will lose access.` |
| `LifeCycleState == error` | 1 | Broken | n/a | S2, S4 | `error` | `File system is in error state; AWS could not complete last operation.` |
| `NumberOfMountTargets == 0` | 1 | Broken | n/a | S2, S4 | `no mount targets` | `No mount targets — file system is unreachable from any subnet.` |
| any mount target `LifeCycleState != available` | 2 | Broken | n/a | S2, S4, S5 | `mount target down` | `N of M mount targets not available (creating/deleting/error); AZ-level access may be degraded.` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword in the List text column is unacceptable unless it is itself readable AWS status language (`creating`, `updating`, `deleting`, `error`) — in EFS those words are the cause the operator would read on the list.
- List text ≤ 40 chars; Detail text ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every Wave 1/2 Broken or Warning row shows either the plain AWS lifecycle word (`error`, `deleting`) or a short cause phrase (`no mount targets`, `mount target down`) in the Status column; the operator can triage without pressing detail, and the S5 sentence in detail adds "how many / which AZ" context when they do.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- `ec2` related-panel pivot — a9s-devops (2026-04-20) verdict: possible=weak, worth=no. No direct EFS→EC2 field exists; inferring mounters from "instances in mount-target subnets" is noisy, so a9s should not surface a consumer list. Operators reach EC2 via the `eni` or `subnet` pivots instead.

## 6. Citations

- a9s golden doc — EFS per-type related contract — `docs/related-resources.md` § Per-type contract row `efs`.
- a9s golden doc — EFS related-target discovery notes (KmsKeyId, mount-target ENIs/SGs/subnets, lambda mounts, ecs-task mounts, backup recovery points, cfn stack, alarm metrics) — `docs/related-resources.md` § `### efs`.
- a9s golden doc — Universal-pivot policy for `ct-events` — `docs/related-resources.md` § Policy.
- a9s golden doc — EFS Wave 1/2/3 signals and Source — `docs/attention-signals.md` § Databases & Storage row `efs`.
- a9s golden doc — Read-only invariant — `docs/architecture.md` § "What is a9s?".
- AWS Go SDK v2 — `FileSystemDescription` shape and field names (`LifeCycleState`, `NumberOfMountTargets`, `FileSystemId`, `FileSystemArn`, `KmsKeyId`, `Encrypted`, `Tags`) — `AWS SDK Go v2 — service/efs/types.FileSystemDescription`.
- AWS Go SDK v2 — `LifeCycleState` enum values (`creating`, `available`, `updating`, `deleting`, `deleted`, `error`) — `AWS SDK Go v2 — service/efs/types.LifeCycleState`.
- AWS Go SDK v2 — `MountTargetDescription` fields (`LifeCycleState`, `SubnetId`, `VpcId`, `NetworkInterfaceId`, `MountTargetId`) — `AWS SDK Go v2 — service/efs/types.MountTargetDescription`.
- AWS Go SDK v2 — `DescribeFileSystems` is the list operation — `AWS SDK Go v2 — service/efs.Client.DescribeFileSystems`.
- a9s-devops consultation — `alarm` discovery via `Namespace=AWS/EFS` dimension `FileSystemId` — `a9s-devops (2026-04-20): possible=yes, worth=yes. DescribeAlarms returns dimensions; no extra call needed when sibling list is loaded.`
- a9s-devops consultation — `backup` discovery via `backup:ListRecoveryPointsByResource(ResourceArn=FileSystemArn)` — `a9s-devops (2026-04-20): possible=yes, worth=yes. FS ARN is the resource key AWS Backup indexes by.`
- a9s-devops consultation — `cfn` discovery via `aws:cloudformation:stack-name` tag on FS — `a9s-devops (2026-04-20): possible=yes, worth=yes. EFS returns tags on the description; CFN-managed resources always carry this tag.`
- a9s-devops consultation — `ec2` discovery is weak and noisy; deferred — `a9s-devops (2026-04-20): possible=weak, worth=no. No direct EFS→EC2 field; subnet-based inference is noisy. Recorded in §5 Out of Scope.`
- a9s-devops consultation — `ecs-task` discovery via task-def `volumes[].efsVolumeConfiguration.FileSystemId` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Requires task-def join onto loaded ecs-task list; degrade to TBD if not joined.`
- user decision (2026-04-24) — Upgrade the ecs-task fetcher to join task-definitions (one `DescribeTaskDefinition` per unique `TaskDefinitionArn` per sweep, result cached on each task's Resource) so the EFS `ecs-task` pivot renders a non-zero count without per-detail-view probing. Ties this skill's scope to a second resource's fetcher; that change is approved in-scope for this PR.
- a9s-devops consultation — `lambda` discovery via `FunctionConfiguration.FileSystemConfigs[].Arn` — `a9s-devops (2026-04-20): possible=yes, worth=yes. FileSystemConfigs[].Arn is an access-point ARN that embeds the FS id; prefix-match so both forms resolve.`
- a9s-devops consultation — `sg` discovery via `DescribeMountTargetSecurityGroups` or ENI `Groups` join — `a9s-devops (2026-04-20): possible=yes, worth=yes. Both paths return the same SG set; prefer ENI join when eni list is already loaded.`
- a9s-devops consultation — `eni`/`subnet`/`vpc` discovery via `DescribeMountTargets` fields `NetworkInterfaceId`/`SubnetId`/`VpcId` — `a9s-devops (2026-04-20): possible=yes, worth=yes. All three are direct SDK fields on MountTargetDescription; one DescribeMountTargets call covers all three pivots.`

<!-- BEGIN GENERATED: header -->
efs — DATABASES & STORAGE. Lifecycle key: `status`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Approximate? |
| --- | --- | --- |
| kms | KMS Keys | no |
| cfn | CloudFormation Stacks | yes |
| sg | Security Groups | no |
| subnet | Subnets | no |
| lambda | Lambda Functions | no |
| alarm | CloudWatch Alarms | yes |
| backup | Backup Plans | yes |
| ecs-task | ECS Tasks | yes |
| eni | Network Interfaces | yes |
| vpc | VPC | yes |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
