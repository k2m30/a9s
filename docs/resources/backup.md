---
shortName: backup
name: Backup Plans
awsApiRef: https://docs.aws.amazon.com/aws-backup/latest/devguide/API_BackupPlan.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# backup — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `backup`
- **Display name**: Backup Plans
- **AWS API reference**: <https://docs.aws.amazon.com/aws-backup/latest/devguide/API_BackupPlan.html>
- **List API**: `ListBackupPlans` (returns `BackupPlansListMember` — config-only; no runtime state).
- **Describe API (if any)**: `ListBackupJobs` — one account-wide call filtered by `ByCreatedAfter=now-24h`, results bucketed client-side by `BackupPlanId`. `GetBackupPlan`, `ListBackupSelections`, `GetBackupSelection`, `DescribeBackupVault`, `GetBackupVaultNotifications` are used on demand to populate the related panel (see §2), not the row state.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `ct-events`, `kms`, `role`, `sns`.

### `kms`

- **Why related**: Recovery-point encryption key. Every backup written to a vault is server-side encrypted with the vault's KMS key; when a job fails with `KMSKeyNotAccessible`, the operator needs to jump to the key directly.
- **How discovered**: Indirect — `GetBackupPlan(planId)` returns `BackupPlan.Rules[].TargetBackupVaultName`; each distinct vault name is resolved via `DescribeBackupVault(BackupVaultName)` whose output carries `EncryptionKeyArn`. The union of `EncryptionKeyArn` values across all rules is the KMS set for this plan. Plans with rules pointing to the AWS-managed default vault will resolve to the AWS-managed `aws/backup` key. — a9s-devops: plan carries no direct KMS field; traversal is plan→rules→vaults→key. (a9s-devops 2026-04-20: possible=yes via `BackupPlan.Rules[].TargetBackupVaultName` + `DescribeBackupVault.EncryptionKeyArn`; worth=yes — KMS-access failures are the #1 non-transient cause of backup job failure.)
- **Count shown**: yes (unique `EncryptionKeyArn` across all rules in the plan).

### `role`

- **Why related**: Backup service role used for backup and restore jobs. When a backup job fails with permissions errors, the operator opens the role to check its trust policy and attached policies.
- **How discovered**: Indirect — `ListBackupSelections(BackupPlanId)` returns `BackupSelectionsList[].IamRoleArn` directly in the list response (no N+1 `GetBackupSelection` call needed for the ARN itself; that call is only required if `Resources`/`Conditions` must also be shown). The union of `IamRoleArn` across all selections attached to this plan is the role set. — a9s-devops: the role is bound per-selection, not per-plan; a plan with multiple selections may use multiple roles. (a9s-devops 2026-04-20: possible=yes via `ListBackupSelections.BackupSelectionsList[].IamRoleArn`; worth=yes — permissions are the other top cause of backup failure.)
- **Count shown**: yes (unique `IamRoleArn` across all selections for the plan).

### `sns`

- **Why related**: Vault failure / job-state notifications. Operators wire SNS to their paging system so a failed backup surfaces outside a9s too; the detail panel lets the operator verify that wiring exists.
- **How discovered**: Indirect — for each unique `TargetBackupVaultName` in the plan's rules (same set resolved for `kms`), call `GetBackupVaultNotifications(BackupVaultName)`. When present, the response carries `SNSTopicArn` and `BackupVaultEvents[]`. Vaults without notifications return the error `ResourceNotFoundException` — treat as "no SNS topic wired." — a9s-devops: SNS is per-vault, not per-plan; absence is itself informative. (a9s-devops 2026-04-20: possible=yes via `GetBackupVaultNotifications.SNSTopicArn` per resolved vault; worth=yes — a plan with zero SNS subscriptions on its vault is silently at risk.)
- **Count shown**: yes (unique `SNSTopicArn` across all referenced vaults; zero is a legitimate value).

### `ct-events`

- **Why related**: Audit trail for plan, selection, and job lifecycle events — who created or modified the plan, when jobs were started or aborted, and by which principal. Universal pivot — applies to every registered type; see `docs/related-resources.md` §Policy.
- **How discovered**: `LookupEvents` scoped to the plan's `BackupPlanArn` / `BackupPlanId` (CloudTrail read API).
- **Count shown**: unknown (CloudTrail events are streamed on demand; a finite count is not computed up front).

## 3. Attention / Issues Algorithm

**Source API**: [ListBackupJobs](https://docs.aws.amazon.com/aws-backup/latest/devguide/API_ListBackupJobs.html)

Transcribed from `docs/attention-signals.md § Signals § BACKUP` row `backup`.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — `ListBackupPlans` is config-only (`BackupPlansListMember` carries `BackupPlanId`, `BackupPlanName`, `CreationDate`, `DeletionDate`, `LastExecutionDate`, `VersionId`, `CreatorRequestId`, `AdvancedBackupSettings` — no runtime state, no job state, no failure field). The list API does not return fields usable for attention.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: any recent `BackupJob.State` in `FAILED` / `EXPIRED` / `ABORTED` for this plan in the last 24h.
  - **State bucket**: Broken.
  - **API call**: `ListBackupJobs(ByCreatedAfter=now-24h)` — **one account-wide call**, results bucketed client-side by `BackupPlanId`.
  - **Cost shape**: account-wide.

- **Signal**: any recent `BackupJob.State == PARTIAL` for this plan in the last 24h (some selected resources backed up, others failed).
  - **State bucket**: Warning.
  - **API call**: same `ListBackupJobs(ByCreatedAfter=now-24h)` call; no additional call.
  - **Cost shape**: account-wide.

### 3.3 Wave 3 — OUT OF SCOPE

Copied verbatim from `docs/attention-signals.md § Not yet implemented`, prefixed `OUT OF SCOPE:`.

- OUT OF SCOPE: "Newest completed older than rule cadence × 2" (requires `GetBackupPlan` per plan for rule cadence).

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `backup`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

Because Wave 1 is silent, every signal below is a Wave 2 finding. Each colours the row it lands on, so the colour and the cause text are what the operator sees first.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| recent job `FAILED` / `EXPIRED` / `ABORTED` | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `<N job(s)> failed in last 24h` |
| recent job `PARTIAL` | 2 | Warning | `~` | S2, S3, S4, S5 | `partial: <N> of <M resource(s)> skipped` |

Rules for filling list and detail text:

- Banned words (never appear): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- S4 never carries a bare state keyword like `FAILED` or `PARTIAL` alone — it always includes the count and the time window so the operator knows the scope at a glance.
- A plan with zero jobs in the 24h window is Healthy: S2 green, S4 blank, no glyph. (A plan that has *never* run is also Healthy by this rule — the out-of-scope Wave 3 signal is what would catch a stale plan, not Wave 2.)
- List text ≤ 40 chars. The Detail sentence lives on the finding definition and is generated into the Findings table below; it is never written here.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, the operator sees a red plan row reading `<N job(s)> failed in last 24h` and knows immediately to open the detail view for the error message and to pivot into the `role` and `kms` related panels. All problem rows are self-explanatory in the list — operator can triage without opening detail; detail adds the exact timestamp of the most recent failure so the operator can correlate with CloudTrail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings. In particular, the derived list-level `⚠ N issues detected by background checks` banner, the row middle-dot `·` marker, and the `⚠ Background Check` detail header described in `docs/historical/analysis/enrichment-visibility.md` are superseded HOW that this spec does not reuse.
- Per-rule cadence comparison ("newest completed older than rule cadence × 2") — requires `GetBackupPlan` per plan and is Wave 3 by budget.
- Write operations. a9s is read-only by design (`docs/architecture.md` — What is a9s?).
- `backup` → `eb-rule` and `backup` → `logs` linkages — `docs/related-resources.md` § Explicitly excluded.

## 6. Citations

- Display name `Backup Plans` — `docs/attention-signals.md § Signals § BACKUP` row `backup`.
- AWS API reference URL — `docs/related-resources.md` § Per-type contract, row `backup`.
- List API is `ListBackupPlans` and is config-only — `core/aws/backup.go`.
- List-response fields on `BackupPlansListMember` — `AWS SDK Go v2 — service/backup/types.BackupPlansListMember § BackupPlanId, BackupPlanName, CreationDate, DeletionDate, LastExecutionDate, VersionId, CreatorRequestId, AdvancedBackupSettings`.
- Wave 2 API is `ListBackupJobs(ByCreatedAfter=now-24h)`, account-wide, bucketed by `BackupPlanId` — `core/aws/backup_issue_enrichment.go`.
- `BackupJob.State` enum values include `FAILED`, `EXPIRED`, `ABORTED`, `PARTIAL` — `AWS SDK Go v2 — service/backup/types.BackupJobState` (`BackupJobStateFailed`, `BackupJobStateExpired`, `BackupJobStateAborted`, `BackupJobStatePartial`).
- `BackupJob.State` and `BackupJob.StatusMessage` are on the `BackupJob` shape returned by `ListBackupJobs` — `AWS SDK Go v2 — service/backup/types.BackupJob § State, StatusMessage, BackupPlanId (nested in CreatedBy.BackupPlanId)`.
- `ct-events` is a universal pivot — `docs/related-resources.md` § Policy.
- Related target `kms` — `docs/related-resources.md` § `backup` — "Recovery-point encryption key."
- Related target `role` — `docs/related-resources.md` § `backup` — "Backup service role used for restore jobs."
- Related target `sns` — `docs/related-resources.md` § `backup` — "Vault notifications."
- Related target `ct-events` — `docs/related-resources.md` § `backup` — "Audit trail for plan/selection/job events."
- `BackupPlan.Rules[].TargetBackupVaultName` exists and identifies the vault — `AWS SDK Go v2 — service/backup/types.BackupRule § TargetBackupVaultName`.
- `BackupSelection.IamRoleArn` is a required field on each selection — `AWS SDK Go v2 — service/backup/types.BackupSelection § IamRoleArn`.
- `BackupJob.IamRoleArn` is set per job when the job runs — `AWS SDK Go v2 — service/backup/types.BackupJob § IamRoleArn`.
- `DescribeBackupVault` returns `EncryptionKeyArn` — `AWS API Reference: DescribeBackupVault § EncryptionKeyArn` (<https://docs.aws.amazon.com/aws-backup/latest/devguide/API_DescribeBackupVault.html>).
- `GetBackupVaultNotifications` returns `SNSTopicArn` and `BackupVaultEvents[]` — `AWS API Reference: GetBackupVaultNotifications § SNSTopicArn, BackupVaultEvents` (<https://docs.aws.amazon.com/aws-backup/latest/devguide/API_GetBackupVaultNotifications.html>).
- `backup`→`eb-rule` and `backup`→`logs` exclusions — `docs/related-resources.md` § Explicitly excluded.
- Wave 3 cadence-comparison deferment — `docs/attention-signals.md § Not yet implemented`.
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
- Discovery mechanism for `kms` from a BackupPlan — `a9s-devops (2026-04-20): possible=yes, worth=yes. Plan carries no KMS field; traversal is plan→Rules[].TargetBackupVaultName→DescribeBackupVault.EncryptionKeyArn. Justified because KMSKeyNotAccessible is the top non-transient cause of backup failure and the operator needs a one-keypress pivot.`
- Discovery mechanism for `role` from a BackupPlan — `a9s-devops (2026-04-20): possible=yes, worth=yes. ListBackupSelections(BackupPlanId).BackupSelectionsList[].IamRoleArn gives the ARN directly; a plan with multiple selections may have multiple roles. Justified because IAM permissions failures are the second top cause of backup failure.`
- Discovery mechanism for `sns` from a BackupPlan — `a9s-devops (2026-04-20): possible=yes, worth=yes. SNS is per-vault; resolve distinct vault names from Rules[].TargetBackupVaultName, then GetBackupVaultNotifications per vault. Justified because absence-of-SNS on a backup vault is itself a silent-risk signal worth surfacing.`
- Severity choice: `FAILED/EXPIRED/ABORTED` = `!` (Broken) and `PARTIAL` = `~` (Warning) — `docs/attention-signals.md § Signals § BACKUP` row `backup` ships the failed-job finding as Broken and the partial-job finding as Warning, which maps to the S1-bumping `!` glyph and the non-bumping `~` glyph per the skill's Wave-to-surface rules.

<!-- BEGIN GENERATED: header -->
backup — BACKUP. Status key: `status` — the key the status cell reads, and the column naming it is the status column.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| backup.job-failed | <N job(s)> failed in last 24h | broken | wave2 | A backup job for this plan did not complete in the last day, so the recovery points you expect for that window do not exist. Open the job for its status message: an IAM permission, a resource deleted mid-job, or a vault lock rule are the usual causes. A plan cannot be rerun, so once it is fixed either start an on-demand backup for each resource that was missed or wait for the next scheduled run. |
| backup.job-partial | partial: <N> of <M resource(s)> skipped | warn | wave2 | The plan ran but skipped some of the resources it selects, so those resources have no recovery point for this window even though the job reports progress. Check the job's resource list against the plan's selection, and the backup role's permissions on the resources that were missed. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| role | IAM Roles | no |
| kms | KMS Keys | no |
| sns | SNS Topics | yes |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
