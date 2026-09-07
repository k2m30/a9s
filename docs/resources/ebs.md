---
shortName: ebs
name: EBS Volumes
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Volume.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# ebs — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `ebs`
- **Display name**: EBS Volumes
- **AWS API reference**: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Volume.html>
- **List API**: `DescribeVolumes` — returns `Volume` objects with `State`, `Attachments[]`, `Encrypted`, `CreateTime`, `KmsKeyId`, `Size`, `VolumeType`, `AvailabilityZone`, `Tags[]`.
- **Describe API (if any)**: `DescribeVolumeStatus` — returns `VolumeStatusItem` with `VolumeStatus.Status` (`ok` / `warning` / `impaired` / `insufficient-data`) and `Events[]` (each with `EventType`, `Description`, `NotBefore`, `NotAfter`).

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `backup`, `cfn`, `ebs-snap`, `ec2`, `kms`, `ct-events`.

### `alarm`

- **Why related**: CloudWatch alarms watching this volume — first signal of throughput/IOPS/queue-length impact. Cited in `docs/related-resources.md` §`ebs` as "Volume CW alarms (throughput/IOPS)".
- **How discovered**: cross-reference the already-loaded `alarm` list by `MetricAlarm.Dimensions[].Name == "VolumeId"` and `Dimensions[].Value == Volume.VolumeId`. No extra API call — `alarm` carries its dimensions on the list response. — a9s-devops (persona): CloudWatch alarms for EBS use the `AWS/EBS` namespace and always dimension on `VolumeId`; sibling-list cross-ref is the standard pattern (same approach as ec2↔alarm).
- **Count shown**: yes.

### `backup`

- **Why related**: Which AWS Backup plan(s) cover this volume — answers "is this volume protected before we touch it?". Cited in `docs/related-resources.md` §`ebs` as "Volumes covered by AWS Backup".
- **How discovered**: cross-reference the already-loaded `backup` list by resource-selection tag matching, or (more reliable) resolve the volume's ARN against `ListProtectedResources` output if cached. — a9s-devops (persona): AWS Backup selection is either tag-based (plan `ResourceSelection.Conditions`) or resource-type blanket; there is no per-volume `BackupPlanId` field on `Volume`, so the pivot requires either a pre-loaded backup-plan list (sibling cross-ref) or an extra API. Practical answer: sibling-list cross-ref when `backup` list is loaded, otherwise the panel renders an empty "backup" group.
- **Count shown**: yes.

### `cfn`

- **Why related**: Which CloudFormation stack owns this volume — answers "can I delete it, or is it IaC-managed?". Cited in `docs/related-resources.md` §`ebs` as "Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot."
- **How discovered**: read `Volume.Tags[]` for the standard CFN-propagated tag `aws:cloudformation:stack-name` (or `aws:cloudformation:stack-id`); no extra API call. — a9s-devops (persona): CFN propagates those two tags to every resource it creates unless the user explicitly disables tagging; this is the idiomatic ownership-pivot on EC2-family resources.
- **Count shown**: yes (0 or 1 — a volume belongs to at most one stack).

### `ebs-snap`

- **Why related**: Snapshots of this volume — the recovery pivot. Cited in `docs/related-resources.md` §`ebs` as "Snapshots of this volume."
- **How discovered**: cross-reference the already-loaded `ebs-snap` list by `Snapshot.VolumeId == Volume.VolumeId`. The snapshot list-response carries `VolumeId` directly (`AWS SDK Go v2 — ec2/types.Snapshot § VolumeId`), so no extra API call.
- **Count shown**: yes.

### `ec2`

- **Why related**: Which instance this volume is attached to — answers "whose workload does this carry?". Cited in `docs/related-resources.md` §`ebs` as "Volume.Attachments[].InstanceId."
- **How discovered**: read `Volume.Attachments[].InstanceId` directly on the list-response (`AWS SDK Go v2 — ec2/types.VolumeAttachment § InstanceId`). No extra API call. `available` volumes return an empty `Attachments[]`.
- **Count shown**: yes (0 for available/orphan volumes, 1+ when multi-attach is enabled).

### `kms`

- **Why related**: Which KMS key encrypts this volume — needed when the key is in `PendingDeletion` or shared across accounts. Cited in `docs/related-resources.md` §`ebs` as "Volume.KmsKeyId — at-rest encryption key."
- **How discovered**: read `Volume.KmsKeyId` directly on the list-response (`AWS SDK Go v2 — ec2/types.Volume § KmsKeyId`). No extra API call. Unencrypted volumes return a nil `KmsKeyId`.
- **Count shown**: yes (0 when unencrypted, 1 when encrypted).

### `ct-events`

- **Why related**: Audit trail for volume changes — who attached/detached/deleted, when. Cited in `docs/related-resources.md` §`ebs` as "Audit trail for volume changes." Universal pivot — applies to every registered type; see `docs/related-resources.md` §Policy.
- **How discovered**: `LookupEvents` filtered by `ResourceName = Volume.VolumeId`. The call is on-demand — only fired when the operator opens the `ct-events` pivot, not during list refresh.
- **Count shown**: yes (last-hour or user-chosen window; see `ct-events` spec).

## 3. Attention / Issues Algorithm

**Source API**: [DescribeVolumeStatus](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeVolumeStatus.html)

Transcribed from `docs/attention-signals.md § Signals § COMPUTE` row `ebs`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `State == "creating"` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `Volume.State` on the `DescribeVolumes` list response.

- **Signal**: `State == deleting`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `State == "error"` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `Volume.State` on the `DescribeVolumes` list response.

- **Signal**: `State == "available"` AND `CreateTime` older than 7d → Warning (orphan — unattached volume billing with no workload).
  - **State bucket**: Warning.
  - **How obtained**: `Volume.State` + `Volume.CreateTime` on the list response, compared against current time client-side.

- **Signal**: `Encrypted == false` → Warning (data at rest is unprotected; a leaked snapshot or detached volume exposes its contents).
  - **State bucket**: Warning.
  - **How obtained**: `Volume.Encrypted` on the list response (`AWS SDK Go v2 — ec2/types.Volume § Encrypted`).

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: `VolumeStatus.Status == "impaired"` → Broken (AWS-observed volume I/O failure).
  - **State bucket**: Broken.
  - **API call**: `DescribeVolumeStatus` — one paginated account/region-wide call covering all volumes.
  - **Cost shape**: account-wide.

- **Signal**: `VolumeStatus.Status == "warning"` → Broken (degraded I/O).
  - **State bucket**: Broken.
  - **API call**: `DescribeVolumeStatus` — same call.
  - **Cost shape**: account-wide.

- **Signal**: no backup plan selection matches this volume.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: an attached volume with no snapshot behind it.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `VolumeQueueLength` (per-volume metric query).
- OUT OF SCOPE: `BurstBalance` on `gp2` volumes (per-volume CloudWatch metric).

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `ebs`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `State == creating` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `creating` |
| `State == deleting` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `deleting` |
| `State == error` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `error` |
| `State == available` & age > 7d | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `orphan: unattached <N>d` |
| `Encrypted == false` (row in-use) | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `unencrypted` |
| `VolumeStatus.Status == impaired` | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `volume I/O degraded` |
| `VolumeStatus.Status == warning` | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `volume I/O degraded` |
| no backup plan selection matches this volume | 2 | Warning | `~` | S2, S3, S4, S5 | `not covered by a backup plan` |
| an attached volume with no snapshot behind it | 2 | Warning | `~` | S2, S3, S4, S5 | `no snapshot exists` |

Notes on rows omitted:

- `State == in-use` is Healthy and produces no §4 row — S2 renders green, S4 renders blank.
- The `available` + age>7d case is a yellow row: the condition is the current state of the volume. `Encrypted == false` is yellow too and carries `~` in the Attention section — the volume is running fine, but there is a security concern worth flagging.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every non-healthy row carries a cause in S4 (`orphan: unattached 42d`, `impaired: I/O failing`, `unencrypted`) and the color is already the attention signal; the operator can triage "delete me", "AWS broke it", or "security debt" at a glance without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (CloudWatch `VolumeQueueLength`, `BurstBalance` on gp2).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings, no middle-dot row marker, no derived list-level banner, no ceremonial "Background Check" header in the detail view. (Superseded HOW passages in `docs/historical/analysis/enrichment-visibility.md` describe such mechanisms; they are ignored by this spec per skill rules.)
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?": "a9s never makes write calls to AWS. Every AWS API call is a List, Describe, or Get operation.").
- Public-snapshot detection via `DescribeSnapshotAttribute` — covered by `ebs-snap`, not `ebs`.
- Per-instance attachment permission analysis — no EBS-specific AWS field surfaces that without extra-cost calls.

## 6. Citations

- a9s golden doc — related targets list (`alarm`, `backup`, `cfn`, `ct-events`, `ebs-snap`, `ec2`, `kms`) — `docs/related-resources.md` § Per-type contract, row `ebs` (line 63); long-form §`ebs` (lines 371–381).
- a9s golden doc — `ec2` pivot via `Volume.Attachments[].InstanceId` — `docs/related-resources.md` §`ebs`, bullet `ec2`.
- a9s golden doc — `kms` pivot via `Volume.KmsKeyId` — `docs/related-resources.md` §`ebs`, bullet `kms`.
- a9s golden doc — `ebs-snap` pivot ("Snapshots of this volume") — `docs/related-resources.md` §`ebs`, bullet `ebs-snap`.
- a9s golden doc — `alarm` pivot ("Volume CW alarms (throughput/IOPS)") — `docs/related-resources.md` §`ebs`, bullet `alarm`.
- a9s golden doc — `backup` pivot ("Volumes covered by AWS Backup") — `docs/related-resources.md` §`ebs`, bullet `backup`.
- a9s golden doc — `cfn` pivot (DevOps-audit mention) — `docs/related-resources.md` §`ebs`, bullet `cfn`.
- a9s golden doc — `ct-events` is a universal pivot — `docs/related-resources.md` §Policy.
- a9s golden doc — Wave 1 signals (`State` buckets, `CreateTime`>7d on available, `Encrypted==false`) — `docs/attention-signals.md § Signals § COMPUTE` row `ebs`.
- a9s golden doc — Wave 2 signals (`VolumeStatus.Status` impaired/warning, `Events[]` non-empty) — `docs/attention-signals.md § Signals § COMPUTE` row `ebs`.
- a9s golden doc — Wave 3 out-of-scope (`VolumeQueueLength`, `BurstBalance`) — `docs/attention-signals.md § Not yet implemented`.
- a9s golden doc — read-only invariant — `docs/architecture.md` §"What is a9s?" (line 15).
- AWS SDK Go v2 — `Volume.State`, `Volume.Encrypted`, `Volume.CreateTime`, `Volume.KmsKeyId`, `Volume.Attachments[]` exist on list response — `AWS SDK Go v2 — ec2/types.Volume § State, Encrypted, CreateTime, KmsKeyId, Attachments`.
- AWS SDK Go v2 — `VolumeAttachment.InstanceId` — `AWS SDK Go v2 — ec2/types.VolumeAttachment § InstanceId`.
- AWS SDK Go v2 — `DescribeVolumeStatus` response shape — `AWS SDK Go v2 — ec2/types.VolumeStatusItem § VolumeStatus, Events`; `AWS SDK Go v2 — ec2/types.VolumeStatusInfo § Status`; `AWS SDK Go v2 — ec2/types.VolumeStatusEvent § EventType, Description, NotBefore, NotAfter`.
- AWS SDK Go v2 — `Snapshot.VolumeId` used for `ebs-snap` cross-ref — `AWS SDK Go v2 — ec2/types.Snapshot § VolumeId`.
- a9s-devops consultation (persona fallback, 2026-04-20) — `alarm` discovery: sibling-list cross-ref via `Dimensions[].Name=="VolumeId"`, same pattern as ec2↔alarm. possible=yes, worth=yes. Rationale: standard CloudWatch-for-EBS namespace pattern.
- a9s-devops consultation (persona fallback, 2026-04-20) — `backup` discovery: AWS Backup uses tag-based / resource-type selection rather than a per-volume `BackupPlanId` field; practical mechanism is sibling-list cross-ref or cached `ListProtectedResources`. possible=yes (with caveat), worth=yes. Rationale: operators regularly ask "is this volume protected before I delete it?".
- a9s-devops consultation (persona fallback, 2026-04-20) — `cfn` discovery: `Volume.Tags[]` lookup for `aws:cloudformation:stack-name` / `aws:cloudformation:stack-id` — CFN propagates these automatically. possible=yes, worth=yes. Rationale: IaC-ownership pivot is a standard ops question and requires no extra API.
- UX decision — `Encrypted==false` on in-use volumes uses `!` severity — governed by `docs/attention-signals.md § Signals § COMPUTE` row `ebs`. Treated as important (`!`) because unencrypted data at rest is a hard security-audit finding, not an informational note.
- UX decision — `Events[] non-empty` uses `~` severity — informational scheduled/AWS-notification event; does not require immediate action, so does not bump S1 menu count.

<!-- BEGIN GENERATED: header -->
ebs — COMPUTE. Lifecycle key: `state`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| ebs.state.creating | creating | warn | wave1 | — |
| ebs.state.deleting | deleting | warn | wave1 | The volume is being deleted; its data is going with it and nothing else about it is worth reporting until it is gone. |
| ebs.state.error | error | broken | wave1 | — |
| ebs.orphan-unattached | orphan: unattached <N>d | warn | wave1 | The volume has been unattached since it was created, so it is billed hourly for no workload; the age is in the status. Snapshot it if the data matters, then delete it. |
| ebs.encryption.disabled | unencrypted | warn | wave1 | Volume is not encrypted at rest — re-create from encrypted snapshot. |
| ebs.volume-io-degraded | volume I/O degraded | broken | wave2 | — |
| ebs.not-in-backup-plan | not covered by a backup plan | warn | wave2 | No backup plan selects this volume, so nothing is scheduled to copy it and a deletion is final. Add it to a plan by ARN, or give it a tag one of your plans already selects on. |
| ebs.no-snapshot | no snapshot exists | warn | wave2 | This volume is attached and in use, and no snapshot of it exists, so there is no point to restore from. Take one, or put the volume in a backup plan that will. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| ec2 | EC2 Instance | no |
| ebs-snap | EBS Snapshots | yes |
| kms | KMS Key | no |
| alarm | CW Alarms | yes |
| backup | Backup | yes |
| cfn | CloudFormation | yes |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
