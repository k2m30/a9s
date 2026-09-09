---
shortName: ebs-snap
name: EBS Snapshots
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Snapshot.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
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

Expected targets from `docs/related-resources.md` § Per-type contract: `ami`, `backup`, `ct-events`, `ebs`, `ec2`, `kms`.

### `ami`

- **Why related**: AMIs derived from this snapshot. An AMI's `BlockDeviceMappings[].Ebs.SnapshotId` points at one or more `ebs-snap` rows; the operator wants to know which AMIs would break if this snapshot were deleted.
- **How discovered**: reverse cross-reference — scan the already-loaded `ami` list for any image whose `BlockDeviceMappings[].Ebs.SnapshotId` equals this `Snapshot.SnapshotId`. No extra API call.
- **Count shown**: unknown — `docs/related-resources.md` § `ebs-snap` does not specify.

### `backup`

- **Why related**: Snapshots covered by AWS Backup. The operator wants to see whether retention and lifecycle for this snapshot are governed by a Backup plan (so "delete this orphan snapshot to save cost" is not safe when Backup still owns it).
- **How discovered**: no direct field on `Snapshot` points at a Backup plan. AWS Backup-created snapshots typically carry a `Description` beginning `"Created by AWS Backup ..."` and an auto-tag `aws:backup:source-resource`; authoritative resolution is `backup:ListRecoveryPointsByResource(ResourceArn=<snapshot-arn>)`. Golden doc is silent on which route a9s uses — `a9s-devops: not specified in docs/related-resources.md § ebs-snap; tag-scan on the already-loaded snapshot is preferred (zero extra calls), fall back to the Backup API when tags are absent`.
- **Count shown**: unknown — `docs/related-resources.md` § `ebs-snap` does not specify.

### `ebs`

- **Why related**: source volume. Every snapshot is born from an EBS volume; the operator jumps here to see whether the source still exists (orphan-snapshot workflow) or inspect the live volume's current state.
- **How discovered**: forward reference on `Snapshot.VolumeId` (AWS SDK Go v2 — `ec2/types.Snapshot § VolumeId`). Cross-reference the already-loaded `ebs` list by `VolumeId`; no extra API call. If the list lacks it, the source volume is deleted — this is exactly the orphan signal in §3.1.
- **Count shown**: unknown — `docs/related-resources.md` § `ebs-snap` does not specify.

### `ec2`

- **Why related**: instances that could be restored from this snapshot. Rollback / forensic workflow — "which running instance did this snapshot belong to, and could I restore it?"
- **How discovered**: indirect; `Snapshot` has no direct EC2 field. Two reverse-lookup paths, both against already-loaded lists: (a) find the `ebs` volume where `Volume.SnapshotId == Snapshot.SnapshotId` and then that volume's `Attachments[].InstanceId`; (b) find AMIs derived from the snapshot (see `ami` above), then instances with those AMI IDs. Golden doc is silent on which a9s uses — `a9s-devops: not specified in docs/related-resources.md § ebs-snap; path (a) is cheaper and more accurate for the restore workflow`.
- **Count shown**: unknown — `docs/related-resources.md` § `ebs-snap` does not specify.

### `kms`

- **Why related**: snapshot encryption key. When the snapshot is encrypted, operator must confirm the KMS key is Enabled and not `PendingDeletion` before restore will succeed.
- **How discovered**: forward reference on `Snapshot.KmsKeyId` (AWS SDK Go v2 — `ec2/types.Snapshot § KmsKeyId`); cross-reference the already-loaded `kms` list. No extra API call.
- **Count shown**: unknown — `docs/related-resources.md` § `ebs-snap` does not specify.

### `ct-events`

- **Why related**: audit trail for snapshot events — who created it, who tried to delete it, what copy operations ran against it.
- **How discovered**: universal pivot — applies to every registered type; see `docs/related-resources.md` §Policy.
- **Count shown**: unknown — universal pivot.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeSnapshots](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSnapshots.html)

Transcribed from `docs/attention-signals.md § Signals § COMPUTE` row `ebs-snap`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `State == pending`.
  - **State bucket**: Warning.
  - **How obtained**: `Snapshot.State` on the `DescribeSnapshots` list response. `Progress` (`"0%"`..`"100%"`) is available on the same shape for detail.

- **Signal**: `State == error`.
  - **State bucket**: Broken.
  - **How obtained**: `Snapshot.State` on the `DescribeSnapshots` list response. `StateMessage` carries AWS's human-readable cause (e.g. KMS permission failure on an encrypted copy) and is used for S4/S5 text.

- **Signal**: snapshot age > 365d with automated description — cost concern.
  - **State bucket**: Warning.
  - **How obtained**: `now() - Snapshot.StartTime > 365d` AND `Snapshot.Description` begins with `"Created by ..."` (automated-snapshot tell). Pure computation over the list response.

- **Signal**: `Encrypted == false` (CIS EC2.1 — EBS snapshots should be encrypted at rest).
  - **State bucket**: Warning.
  - **How obtained**: `Snapshot.Encrypted` on the `DescribeSnapshots` list response.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal. Each runs on the type's bounded second pass, after the rows are on screen.

- **Signal**: source volume deleted — orphan snapshot. Cross-reference `ebs`.
  - **State bucket**: Warning.
  - **How obtained**: `Snapshot.VolumeId` not present in the already-loaded `ebs` list (rule skipped when the `ebs` list was not loaded in this sweep).

- **Signal**: restorable by every AWS account (`DescribeSnapshots(RestorableByUserIds=[all])`).
  - **State bucket**: Broken.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `DescribeSnapshotAttribute(createVolumePermission)` per snapshot (public-snapshot detection).

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `ebs-snap`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

Note: the Wave 1 signals `age > 365d` and `Encrypted == false` are background-check-style concerns but are Wave 1 (zero extra calls). They apply to rows that would otherwise be Healthy (`State == completed`). They turn a green row yellow (Warning) via S2, S4 carries the cause, and the Attention section carries `~` and the sentence.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `State == pending` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `pending` |
| `State == error` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `error` |
| age > 365d AND automated description | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `automated, <N>d old` |
| `Encrypted == false` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `unencrypted` |
| orphan: source volume deleted | 2 | Warning | `~` | S2, S3, S4, S5 | `orphan: source volume deleted` |
| restorable by every AWS account (`DescribeSnapshots(RestorableByUserIds=[all])`) | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `shared with all AWS accounts` |

(Summary-row figures like `420d` and `<StateMessage>` are placeholders the view fills from the SDK fields `StartTime` and `StateMessage` respectively; List text ≤ 40 chars. The Detail cell quotes the finding's Detail constant verbatim, however long it is.)

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every problem row carries a named cause in S4 (`error`, `orphan: source volume deleted`, `automated, <N>d old`, `unencrypted`, `shared with all AWS accounts`), so a red or yellow row never needs a keypress to triage; only the full `StateMessage` text and exact `StartTime` / `VolumeId` require opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above):
  - `DescribeSnapshotAttribute(createVolumePermission)` per snapshot (public-snapshot detection).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings. No row-dot, no banner ornament, no `⚠ Background Check` header.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- a9s golden doc — `ebs-snap` appears in Per-type contract with related targets `ami, backup, ct-events, ebs, ec2, kms` — `docs/related-resources.md` § Per-type contract (row `ebs-snap`).
- a9s golden doc — Per-target reasoning lines for `ami`, `backup`, `ct-events`, `ebs`, `ec2`, `kms` — `docs/related-resources.md` § `ebs-snap`.
- a9s golden doc — `ct-events` is the universal pivot — `docs/related-resources.md` § Policy item 4.
- a9s golden doc — Wave 1 signals: `State` bucketing, age >365d with automated description, `Encrypted==false` CIS EC2.1, orphan via `ebs` cross-ref — `docs/attention-signals.md § Signals § COMPUTE` row `ebs-snap`.
- a9s golden doc — the `ebs-snap` signals — `docs/attention-signals.md § Signals § COMPUTE` row `ebs-snap`; the deferred snapshot-lineage cost attribution — `docs/attention-signals.md § Not yet implemented`.
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
- a9s-devops consultation — count-shown policy — `a9s-devops (2026-04-20): possible=yes, worth=no as a per-resource override. docs/related-resources.md § ebs-snap does not specify per-target count semantics; leaving as "unknown" until the WHAT doc adds a count column or a9s sets a project-wide rule. No value in guessing per-resource.`
- UX rewrite — S4 `error: <StateMessage>` vs bare `error` — `user default (2026-04-20): pair the state keyword with StateMessage so a red row is triageable without opening detail; matches the skill's "state keywords are not explanations" rule.`

<!-- BEGIN GENERATED: header -->
ebs-snap — COMPUTE. Status key: `state` — the key the status cell reads, and the column naming it is the status column.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| ebs-snap.state.pending | pending | warn | wave1 | The snapshot is still being written and cannot be used to restore a volume or copied to another region yet. Wait for it to complete before relying on it as the recovery point for anything. |
| ebs-snap.state.error | error | broken | wave1 | This snapshot failed and holds no usable copy of the volume, so any recovery plan naming it has a hole in it. Take a fresh snapshot of the source volume and delete this one. |
| ebs-snap.encryption.disabled | unencrypted | warn | wave1 | The snapshot's contents are stored unencrypted, and any volume restored from it starts unencrypted too. Copy it with a KMS key, restore from the copy, then delete this one. |
| ebs-snap.aged-automated | automated, <N>d old | warn | wave1 | This automated snapshot is old and no retention policy prunes it, so it is billed indefinitely; the age is in the status. Add a lifecycle policy, or delete it. |
| ebs-snap.orphan | orphan: source volume deleted | warn | wave2 | The volume this snapshot came from no longer exists, so nothing is refreshing it and it will never get any newer. Keep it deliberately as an archive with an owner, or delete it — either way its stored data is billed every month. |
| ebs-snap.public | shared with all AWS accounts | broken | wave2 | This snapshot is shared with every AWS account, so anyone can restore a volume from it and read whatever the source disk held. Stop sharing the snapshot with the `all` group. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| ami | AMIs | yes |
| ebs | EBS Volume | no |
| ec2 | EC2 Instance | no |
| kms | KMS Key | no |
| backup | Backup | yes |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
