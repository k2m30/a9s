---
shortName: ebs-snap
name: EBS Snapshots
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Snapshot.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# ebs-snap — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `ebs-snap`
- **Display name**: EBS Snapshots
- **AWS API reference**: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Snapshot.html>
- **List API**: `DescribeSnapshots`
- **Describe API (if any)**: not used (Wave 2 is `None` for this resource; `DescribeSnapshotAttribute` is Wave 3 / out of scope).

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `ami`, `backup`, `ct-events`, `ebs`, `ec2`, `kms`.

### `ami`

- **Why related**: AMIs derived from this snapshot. An AMI's `BlockDeviceMappings[].Ebs.SnapshotId` points at one or more `ebs-snap` rows; the operator wants to know which AMIs would break if this snapshot were deleted.
- **How discovered**: reverse cross-reference — scan the already-loaded `ami` list for any image whose `BlockDeviceMappings[].Ebs.SnapshotId` equals this `Snapshot.SnapshotId`. No extra API call.
- **Count shown**: unknown — `docs/related-resources.md` does not specify.

### `backup`

- **Why related**: Snapshots covered by AWS Backup. The operator wants to see whether retention and lifecycle for this snapshot are governed by a Backup plan (so "delete this orphan snapshot to save cost" is not safe when Backup still owns it).
- **How discovered**: no direct field on `Snapshot` points at a Backup plan. AWS Backup-created snapshots typically carry a `Description` beginning `"Created by AWS Backup ..."` and an auto-tag `aws:backup:source-resource`; authoritative resolution is `backup:ListRecoveryPointsByResource(ResourceArn=<snapshot-arn>)`. Golden doc is silent on which route a9s uses — `a9s-devops: not specified in related-resources.md; tag-scan on the already-loaded snapshot is preferred (zero extra calls), fall back to the Backup API when tags are absent`.
- **Count shown**: unknown — `docs/related-resources.md` does not specify.

### `ebs`

- **Why related**: source volume. Every snapshot is born from an EBS volume; the operator jumps here to see whether the source still exists (orphan-snapshot workflow) or inspect the live volume's current state.
- **How discovered**: forward reference on `Snapshot.VolumeId` (AWS SDK Go v2 — `ec2/types.Snapshot § VolumeId`). Cross-reference the already-loaded `ebs` list by `VolumeId`; no extra API call. If the list lacks it, the source volume is deleted — this is exactly the orphan signal in §3.1.
- **Count shown**: unknown — `docs/related-resources.md` does not specify.

### `ec2`

- **Why related**: instances that could be restored from this snapshot. Rollback / forensic workflow — "which running instance did this snapshot belong to, and could I restore it?"
- **How discovered**: indirect; `Snapshot` has no direct EC2 field. Two reverse-lookup paths, both against already-loaded lists: (a) find the `ebs` volume where `Volume.SnapshotId == Snapshot.SnapshotId` and then that volume's `Attachments[].InstanceId`; (b) find AMIs derived from the snapshot (see `ami` above), then instances with those AMI IDs. Golden doc is silent on which a9s uses — `a9s-devops: not specified in related-resources.md; path (a) is cheaper and more accurate for the restore workflow`.
- **Count shown**: unknown — `docs/related-resources.md` does not specify.

### `kms`

- **Why related**: snapshot encryption key. When the snapshot is encrypted, operator must confirm the KMS key is Enabled and not `PendingDeletion` before restore will succeed.
- **How discovered**: forward reference on `Snapshot.KmsKeyId` (AWS SDK Go v2 — `ec2/types.Snapshot § KmsKeyId`); cross-reference the already-loaded `kms` list. No extra API call.
- **Count shown**: unknown — `docs/related-resources.md` does not specify.

### `ct-events`

- **Why related**: audit trail for snapshot events — who created it, who tried to delete it, what copy operations ran against it.
- **How discovered**: universal pivot — applies to every registered type; see `docs/related-resources.md` §Policy.
- **Count shown**: unknown — universal pivot.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `State == completed`.
  - **State bucket**: Healthy.
  - **How obtained**: `Snapshot.State` on the `DescribeSnapshots` list response.

- **Signal**: `State == pending`.
  - **State bucket**: Warning.
  - **How obtained**: `Snapshot.State` on the `DescribeSnapshots` list response. `Progress` (`"0%"`..`"100%"`) is available on the same shape for detail.

- **Signal**: `State` in `error` / `recoverable` / `recovering`.
  - **State bucket**: Broken.
  - **How obtained**: `Snapshot.State` on the `DescribeSnapshots` list response. `StateMessage` carries AWS's human-readable cause (e.g. KMS permission failure on an encrypted copy) and is used for S4/S5 text.

- **Signal**: snapshot age > 365d with automated description — cost concern.
  - **State bucket**: Warning.
  - **How obtained**: `now() - Snapshot.StartTime > 365d` AND `Snapshot.Description` begins with `"Created by ..."` (automated-snapshot tell). Pure computation over the list response.

- **Signal**: `Encrypted == false` (CIS EC2.1 — EBS snapshots should be encrypted at rest).
  - **State bucket**: Warning.
  - **How obtained**: `Snapshot.Encrypted` on the `DescribeSnapshots` list response.

- **Signal**: source volume deleted — orphan snapshot. Cross-reference `ebs`.
  - **State bucket**: Warning.
  - **How obtained**: `Snapshot.VolumeId` not present in the already-loaded `ebs` list (rule skipped when the `ebs` list was not loaded in this sweep).

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `DescribeSnapshotAttribute(createVolumePermission)` per snapshot (public-snapshot detection).

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing" — e.g. snapshot approaching cost-age threshold, unencrypted snapshot. `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `error: KMS key disabled`, `orphan: source volume deleted`). **Healthy rows render blank** — no `OK` / `completed`. Empty means "nothing to see." |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank. Silence is the UX.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4 (short cause), S5 (full sentence). `ebs-snap` has no Wave 2, so this case does not arise here.
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph on green row. Same caveat.
- **Wave 2 finding on an already yellow/red/dim row** → redundant with color; S3 suppressed, S4 deduplicates, S5 still carries the full sentence, S1 still counts if `!`. Not applicable here.

Note: the Wave 1 signals `age > 365d` and `Encrypted == false` are background-check-style concerns but are Wave 1 (zero extra calls). They apply to rows that would otherwise be Healthy (`State == completed`). Treating them strictly per the mapping rules, they turn a green row yellow (Warning) via S2, and S4 carries the cause. They do not get a `~` glyph because the row is no longer green.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `State == pending` | 1 | Warning | n/a | S2, S4 | `creating (Progress%)` | `Snapshot still being created; progress reported by AWS.` |
| `State == error` | 1 | Broken | n/a | S2, S4 | `error: <StateMessage>` | `Snapshot failed; AWS reason: <StateMessage>.` |
| `State == recoverable` | 1 | Broken | n/a | S2, S4 | `recoverable: AWS degraded` | `Snapshot in recoverable state; contact AWS to restore.` |
| `State == recovering` | 1 | Broken | n/a | S2, S4 | `recovering: being restored by AWS` | `AWS is recovering this snapshot after an earlier failure.` |
| age > 365d AND automated description | 1 | Warning | n/a | S2, S4 | `age 420d: automated, review cost` | `Automated snapshot older than 365d; consider lifecycle policy.` |
| `Encrypted == false` | 1 | Warning | n/a | S2, S4 | `unencrypted: CIS EC2.1` | `Snapshot is not encrypted at rest; CIS EC2.1 flags this.` |
| orphan: source volume deleted | 1 | Warning | n/a | S2, S4 | `orphan: source volume deleted` | `Source EBS volume no longer exists in this account/region.` |

(Summary-row figures like `420d` and `<StateMessage>` are placeholders the view fills from the SDK fields `StartTime` and `StateMessage` respectively; List text ≤ 40 chars, Detail text ≤ 100 chars.)

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every problem row carries a named cause in S4 (`error: KMS key disabled`, `orphan: source volume deleted`, `age 420d: automated, review cost`, `unencrypted: CIS EC2.1`), so a red or yellow row never needs a keypress to triage; only the full `StateMessage` text and exact `StartTime` / `VolumeId` require opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above):
  - `DescribeSnapshotAttribute(createVolumePermission)` per snapshot (public-snapshot detection).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings. No row-dot, no banner ornament, no `⚠ Background Check` header.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- a9s golden doc — `ebs-snap` appears in Per-type contract with related targets `ami, backup, ct-events, ebs, ec2, kms` — `docs/related-resources.md` § Per-type contract (row `ebs-snap`).
- a9s golden doc — Per-target reasoning lines for `ami`, `backup`, `ct-events`, `ebs`, `ec2`, `kms` — `docs/related-resources.md` § `ebs-snap`.
- a9s golden doc — `ct-events` is the universal pivot — `docs/related-resources.md` § Policy item 4.
- a9s golden doc — Wave 1 signals: `State` bucketing, age >365d with automated description, `Encrypted==false` CIS EC2.1, orphan via `ebs` cross-ref — `docs/attention-signals.md` § Compute row `ebs-snap`.
- a9s golden doc — Wave 2 = None, Wave 3 = `DescribeSnapshotAttribute(createVolumePermission)` per snapshot — `docs/attention-signals.md` § Compute row `ebs-snap`.
- a9s golden doc — read-only invariant — `docs/architecture.md` § "What is a9s?".
- AWS Go SDK v2 — `Snapshot.State` carries the `SnapshotState` enum with values `pending`, `completed`, `error`, `recoverable`, `recovering` — `AWS SDK Go v2 — ec2/types.Snapshot § State` and `ec2/types.SnapshotState`.
- AWS Go SDK v2 — `Snapshot.StateMessage` carries the AWS-generated error-diagnostic string for failed snapshot copies — `AWS SDK Go v2 — ec2/types.Snapshot § StateMessage`.
- AWS Go SDK v2 — `Snapshot.VolumeId` is the source-volume reference used for the `ebs` pivot and orphan check — `AWS SDK Go v2 — ec2/types.Snapshot § VolumeId`.
- AWS Go SDK v2 — `Snapshot.KmsKeyId` is the encryption-key reference used for the `kms` pivot — `AWS SDK Go v2 — ec2/types.Snapshot § KmsKeyId`.
- AWS Go SDK v2 — `Snapshot.StartTime` is the age anchor for the >365d cost signal — `AWS SDK Go v2 — ec2/types.Snapshot § StartTime`.
- AWS Go SDK v2 — `Snapshot.Encrypted` is a bool on the list response — `AWS SDK Go v2 — ec2/types.Snapshot § Encrypted`.
- AWS Go SDK v2 — `Snapshot.Description` carries the automated-creator tell (`"Created by ..."`) — `AWS SDK Go v2 — ec2/types.Snapshot § Description`.
- AWS Go SDK v2 — `Snapshot.Progress` is available on the list response for S4 detail during `pending` — `AWS SDK Go v2 — ec2/types.Snapshot § Progress`.
- a9s-devops consultation — discovery mechanism for `backup` target — `a9s-devops (2026-04-20): possible=yes, worth=yes. Golden doc is silent on which path a9s uses; AWS Backup-created snapshots carry a Description beginning "Created by AWS Backup ..." and an aws:backup:source-resource tag, and backup:ListRecoveryPointsByResource(ResourceArn=<snapshot-arn>) is the authoritative API. Tag-scan on already-loaded data is preferred (zero extra calls).`
- a9s-devops consultation — discovery mechanism for `ec2` target — `a9s-devops (2026-04-20): possible=yes, worth=yes. Golden doc is silent; Snapshot has no direct EC2 field. Reverse path via ebs (Volume.SnapshotId → Volume.Attachments[].InstanceId) is preferred over ami → ec2 for the restore workflow — it is cheaper and more accurate.`
- a9s-devops consultation — count-shown policy — `a9s-devops (2026-04-20): possible=yes, worth=no as a per-resource override. docs/related-resources.md does not specify per-target count semantics for ebs-snap; leaving as "unknown" until the WHAT doc adds a count column or a9s sets a project-wide rule. No value in guessing per-resource.`
- UX rewrite — S4 `error: <StateMessage>` vs bare `error` — `user default (2026-04-20): pair the state keyword with StateMessage so a red row is triageable without opening detail; matches the skill's "state keywords are not explanations" rule.`

<!-- BEGIN GENERATED: header -->
ebs-snap — COMPUTE. Lifecycle key: `state`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Approximate? |
| --- | --- | --- |
| ami | AMIs | yes |
| ebs | EBS Volume | no |
| ec2 | EC2 Instance | no |
| kms | KMS Key | no |
| backup | Backup | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
