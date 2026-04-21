---
shortName: ebs
name: EBS Volumes
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Volume.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
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

Transcribed from `docs/attention-signals.md` §Compute row `ebs`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `State == "in-use"` → Healthy.
  - **State bucket**: Healthy.
  - **How obtained**: `Volume.State` on the `DescribeVolumes` list response (`AWS SDK Go v2 — ec2/types.Volume § State`).

- **Signal**: `State == "creating"` or `State == "deleting"` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `Volume.State` on the `DescribeVolumes` list response.

- **Signal**: `State == "error"` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `Volume.State` on the `DescribeVolumes` list response.

- **Signal**: `State == "available"` AND `CreateTime` older than 7d → Warning (orphan — unattached volume billing with no workload).
  - **State bucket**: Warning.
  - **How obtained**: `Volume.State` + `Volume.CreateTime` on the list response, compared against current time client-side.

- **Signal**: `Encrypted == false` → Warning (CIS EC2.7 — EBS encryption-at-rest best practice).
  - **State bucket**: Warning.
  - **How obtained**: `Volume.Encrypted` on the list response (`AWS SDK Go v2 — ec2/types.Volume § Encrypted`).

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: `VolumeStatus.Status == "impaired"` → Broken (AWS-observed volume I/O failure).
  - **State bucket**: Broken.
  - **API call**: `DescribeVolumeStatus` — one paginated account/region-wide call covering all volumes.
  - **Cost shape**: account-wide.

- **Signal**: `VolumeStatus.Status == "warning"` → Warning (degraded I/O).
  - **State bucket**: Warning.
  - **API call**: `DescribeVolumeStatus` — same call.
  - **Cost shape**: account-wide.

- **Signal**: `Events[]` non-empty → Warning (scheduled action such as I/O-enabling event, volume-stuck event, or AWS-notification event on this volume).
  - **State bucket**: Warning.
  - **API call**: `DescribeVolumeStatus` — same call; `Events[]` arrives on the same `VolumeStatusItem` (`AWS SDK Go v2 — ec2/types.VolumeStatusItem § Events`).
  - **Cost shape**: account-wide.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `VolumeQueueLength` (per-volume metric query).
- OUT OF SCOPE: `BurstBalance` on `gp2` volumes (per-volume CloudWatch metric).

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing" — e.g. orphan, unencrypted. `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `orphan: unattached 42d`, `impaired: I/O failing`). **Healthy rows render blank** — no `in-use`, no `OK`. Empty means "nothing to see." |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank. Silence is the UX.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4 (short cause), S5 (full sentence).
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph on green row. S3, S4 (short cause), S5 (full sentence). No S1.
- **Wave 2 finding on an already yellow/red/dim row** → redundant with color; S3 suppressed, S4 deduplicates with existing cause, S5 still carries the full sentence, S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `State == creating` | 1 | Warning | n/a | S2, S4 | `creating` | `Volume creation in progress.` |
| `State == deleting` | 1 | Warning | n/a | S2, S4 | `deleting` | `Volume is being deleted.` |
| `State == error` | 1 | Broken | n/a | S2, S4 | `error: volume unusable` | `Volume entered error state — AWS marked it unusable; recreate from snapshot.` |
| `State == available` & age > 7d | 1 | Warning | n/a | S2, S4 | `orphan: unattached <N>d` | `Unattached since creation <N> days ago — billed hourly for no workload.` |
| `Encrypted == false` (row in-use) | 2 | Healthy | `!` | S1, S3, S4, S5 | `unencrypted (CIS EC2.7)` | `Volume is not encrypted at rest — violates CIS EC2.7; re-create from encrypted snapshot.` |
| `VolumeStatus.Status == impaired` | 2 | Broken | n/a | S2, S4, S5 | `impaired: I/O failing` | `AWS reports impaired volume status — I/O is failing; detach and restore from snapshot.` |
| `VolumeStatus.Status == warning` | 2 | Warning | n/a | S2, S4, S5 | `degraded: I/O warning` | `AWS reports degraded performance — investigate recent workload and snapshot before action.` |
| `Events[] non-empty` (row in-use) | 2 | Warning | `~` | S3, S4, S5 | `event: <EventType>` | `<Event.Description> — window <NotBefore> to <NotAfter>.` |

Notes on rows omitted:

- `State == in-use` is Healthy and produces no §4 row — S2 renders green, S4 renders blank.
- The `available` + age>7d case is rendered as a yellow row (Warning), not a green row with `!`, because the condition is the current state of the volume, not a background check over a Healthy row. `!` is reserved for background-check findings on Healthy (in-use) rows. `Encrypted == false` on an `in-use` row is the canonical `!` case — the volume is running fine, but there is a security concern worth flagging.
- The `Events[] non-empty` signal is rendered as `~` (informational) on an `in-use` row. When the same volume is already yellow/red for another reason (e.g. `VolumeStatus.Status == warning`), the `~` glyph is suppressed per the "finding on already-yellow/red row" rule; the event sentence still appears in S5.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every non-healthy row carries a cause in S4 (`orphan: unattached 42d`, `impaired: I/O failing`, `unencrypted (CIS EC2.7)`) and the color is already the attention signal; the operator can triage "delete me", "AWS broke it", or "security debt" at a glance without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (CloudWatch `VolumeQueueLength`, `BurstBalance` on gp2).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings, no middle-dot row marker, no derived list-level banner, no ceremonial "Background Check" header in the detail view. (Superseded HOW passages in `docs/enrichment-visibility.md` describe such mechanisms; they are ignored by this spec per skill rules.)
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
- a9s golden doc — Wave 1 signals (`State` buckets, `CreateTime`>7d on available, `Encrypted==false`) — `docs/attention-signals.md` §Compute, row `ebs`, Wave 1 column.
- a9s golden doc — Wave 2 signals (`VolumeStatus.Status` impaired/warning, `Events[]` non-empty) — `docs/attention-signals.md` §Compute, row `ebs`, Wave 2 column.
- a9s golden doc — Wave 3 out-of-scope (`VolumeQueueLength`, `BurstBalance`) — `docs/attention-signals.md` §Compute, row `ebs`, Wave 3 column.
- a9s golden doc — read-only invariant — `docs/architecture.md` §"What is a9s?" (line 15).
- AWS SDK Go v2 — `Volume.State`, `Volume.Encrypted`, `Volume.CreateTime`, `Volume.KmsKeyId`, `Volume.Attachments[]` exist on list response — `AWS SDK Go v2 — ec2/types.Volume § State, Encrypted, CreateTime, KmsKeyId, Attachments`.
- AWS SDK Go v2 — `VolumeAttachment.InstanceId` — `AWS SDK Go v2 — ec2/types.VolumeAttachment § InstanceId`.
- AWS SDK Go v2 — `DescribeVolumeStatus` response shape — `AWS SDK Go v2 — ec2/types.VolumeStatusItem § VolumeStatus, Events`; `AWS SDK Go v2 — ec2/types.VolumeStatusInfo § Status`; `AWS SDK Go v2 — ec2/types.VolumeStatusEvent § EventType, Description, NotBefore, NotAfter`.
- AWS SDK Go v2 — `Snapshot.VolumeId` used for `ebs-snap` cross-ref — `AWS SDK Go v2 — ec2/types.Snapshot § VolumeId`.
- a9s-devops consultation (persona fallback, 2026-04-20) — `alarm` discovery: sibling-list cross-ref via `Dimensions[].Name=="VolumeId"`, same pattern as ec2↔alarm. possible=yes, worth=yes. Rationale: standard CloudWatch-for-EBS namespace pattern.
- a9s-devops consultation (persona fallback, 2026-04-20) — `backup` discovery: AWS Backup uses tag-based / resource-type selection rather than a per-volume `BackupPlanId` field; practical mechanism is sibling-list cross-ref or cached `ListProtectedResources`. possible=yes (with caveat), worth=yes. Rationale: operators regularly ask "is this volume protected before I delete it?".
- a9s-devops consultation (persona fallback, 2026-04-20) — `cfn` discovery: `Volume.Tags[]` lookup for `aws:cloudformation:stack-name` / `aws:cloudformation:stack-id` — CFN propagates these automatically. possible=yes, worth=yes. Rationale: IaC-ownership pivot is a standard ops question and requires no extra API.
- UX decision — `Encrypted==false` on in-use volumes uses `!` severity — governed by `docs/attention-signals.md` Wave 1 entry (Warning), rendered as `!` on Healthy rows per this skill's §4 mapping rule "Wave 2 background finding on a Healthy row, important". Treated as important (`!`) because CIS EC2.7 is a hard security-audit finding, not an informational note.
- UX decision — `Events[] non-empty` uses `~` severity — informational scheduled/AWS-notification event; does not require immediate action, so does not bump S1 menu count.
