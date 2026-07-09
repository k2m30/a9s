---
shortName: backup
generatedFrom: docs/resources/backup.md
date: 2026-04-23
---

# backup — Implementation Plan

Derived from `docs/resources/backup.md` per `a9s-implement-resource` phases 3–5.
All pseudocode in §1 and fixture prose in §2 are implementation-blind — the QA
agent translates them to Go tests, the coder agent writes production code.

## 0. Coverage matrix (mandatory per skill §phase 3)

| ID | Invariant | Fixture | Test |
|----|-----------|---------|------|
| U1 | Healthy blank S4 | `plan-healthy-daily` | `ExpectRowStatusBlank` |
| U2 | Warning/Broken §4 phrase | `plan-broken-2failed`, `plan-warning-partial` | `ExpectRowStatusEquals` per plan |
| U3 | `~` glyph on Healthy+~ finding | `plan-warning-partial` | `ExpectRowNamePrefix("~ ")` |
| U4 | `!` glyph on Healthy+! finding | `plan-broken-2failed` | `ExpectRowNamePrefix("! ")` |
| U5 | No glyph on non-green rows | N/A for backup — every Wave 2 finding lands on a Healthy row, so no non-green rows exist. Healthy rows are tested in U3/U4 and a negative `ExpectRowNoGlyphPrefix` for `plan-healthy-daily`. | `ExpectRowNoGlyphPrefix(plan-healthy-daily)` |
| U6 | S1 menu `issues:N` counts `!` instances | all fixtures | `ExpectMenuIssueCount("backup", 1)` — only `plan-broken-2failed` is `!` |
| U7a | multi Wave-1 `(+N-1)` suffix | N/A — §3.1 empty | N/A |
| U7b | W1+W2 suffix bump | N/A — §3.1 empty | N/A |
| U7c | S5 carries every Row | `plan-broken-2failed` (2 failed jobs → 2 Rows) | `ExpectViewContains` each row's Value |
| U7d | `!` beats `~` | `plan-broken-mixed` (1 failed + 1 partial in window) | `ExpectRowNamePrefix("! ")`, `ExpectRowStatusEquals(id, "1 job failed in last 24h")` |
| U7e | detail enumerates every Wave-1 phrase | N/A — §3.1 empty | N/A |
| U7f | fetcher populates `Resource.Issues` | N/A — §3.1 empty; `Issues` stays empty for every backup fixture | N/A |
| U8 | Broken > Warning | covered by U7d | covered |
| U9 | Related pivots non-zero | `plan-graph-root` (= `plan-broken-2failed`) resolves ≥1 for kms, role, sns; ct-events = unknown → exempt | `ExpectRelatedRowCountAtLeast("IAM Roles", 1)` etc. |
| U10 | No jargon columns | all | `ExpectViewNotContains("CIS", "Flags", "Policy", "Issues", "NOBKP", "Last Status")` |
| U11 | Summary ≠ Rows content | `plan-broken-2failed`, `plan-warning-partial` | unit: `Summary == <short>` AND `!strings.Contains(Summary, row.Value)` |

Note that U7a/U7b/U7e/U7f are genuinely N/A because spec §3.1 declares "No Wave 1
signals." Rule-7 `(+N)` suffix arithmetic is driven by Wave-1 phrase stacking;
with zero Wave-1 signals, the suffix machinery never activates for backup. The
skip is cited, not silent.

## 1. Pseudocode test spec

Every `WHEN` case drives the real `FetchBackupPlansPage` + enricher chain. Mocks
implement the narrow interfaces in `internal/aws/backup_interfaces.go`.

```text
TEST: healthy_plan_no_jobs_is_silent
GIVEN: a backup plan "plan-healthy-daily" AND zero BackupJobs for it in the 24h window
WHEN:  list is fetched and enricher runs
THEN:
  - Resource.Status == "" (fetcher silence)
  - after enrichment, FieldUpdates for this plan do NOT set "status"
  - row color = green
  - no `!` / `~` glyph before the name
  - S1 issues count does NOT bump
  - Resource.Issues is empty (Wave-1 silent by spec §3.1)
```

```text
TEST: plan_with_zero_jobs_ever_is_still_healthy
GIVEN: a backup plan "plan-never-ran" AND NO BackupJobs at all (neither in window nor out)
WHEN:  list is fetched and enricher runs
THEN:
  - row color = green; S4 blank; no glyph
  - This is the explicit spec §4 rule: "A plan that has *never* run is also Healthy"
```

```text
TEST: plan_with_one_failed_job_shows_broken_phrase
GIVEN: a backup plan "plan-broken-1failed" with EXACTLY ONE BackupJob in 24h window, State=FAILED
WHEN:  list is fetched and enricher runs
THEN:
  - EnrichmentFinding emitted with:
      Severity = "!"
      Summary  = "1 job failed in last 24h"
      Rows[0]  = {Label: "State", Value: "FAILED"}
      Rows[1]  = {Label: "Most recent", Value: "2026-04-22 07:12 UTC"} (or whichever timestamp the fixture pins)
  - FieldUpdates[id]["status"] == "1 job failed in last 24h" (spec §4 "List text")
  - S1 issues count bumps by 1 (this instance has a `!` finding)
  - row renders:  `! plan-broken-1failed   1 job failed in last 24h`  (green row + glyph + phrase)
  - detail view Attention section contains the Summary, the State row, the timestamp row
  - Summary does NOT contain "FAILED" nor the timestamp (U11 — those belong in Rows)
```

```text
TEST: plan_with_two_failed_jobs_counts_correctly
GIVEN: a backup plan "plan-broken-2failed" with TWO BackupJobs in 24h window:
         job-a State=FAILED at 2026-04-22 07:12 UTC
         job-b State=EXPIRED at 2026-04-22 12:30 UTC
WHEN:  list is fetched and enricher runs
THEN:
  - EnrichmentFinding emitted with:
      Severity = "!"
      Summary  = "2 jobs failed in last 24h"
      Rows     contains both State values and the most-recent timestamp
  - FieldUpdates[id]["status"] == "2 jobs failed in last 24h"
  - Summary exactly matches spec §4 S4 text
  - row color green, glyph `!`, S1 counts +1
```

```text
TEST: plan_with_one_aborted_job_is_also_broken
GIVEN: a backup plan "plan-broken-aborted" with one BackupJob State=ABORTED in window
WHEN:  list is fetched and enricher runs
THEN:
  - Severity = "!"
  - Summary  = "1 job failed in last 24h"  (ABORTED maps to the "failed" bucket per §3.2)
  - spec §4 uses the single canonical phrase for FAILED+EXPIRED+ABORTED — they all count as "failed"
```

```text
TEST: plan_with_partial_job_is_warning_with_tilde_glyph
GIVEN: a backup plan "plan-warning-partial" with THREE BackupJobs in window:
         job-a State=COMPLETED (succeeded)
         job-b State=COMPLETED (succeeded)
         job-c State=PARTIAL (partial)
         and no FAILED/EXPIRED/ABORTED jobs
WHEN:  list is fetched and enricher runs
THEN:
  - EnrichmentFinding emitted with:
      Severity = "~"
      Summary  = "partial: 1 of 3 resources skipped"
      Rows[0]  = {Label: "Partial jobs", Value: "1"}
      Rows[1]  = {Label: "Total jobs", Value: "3"}
  - FieldUpdates[id]["status"] == "partial: 1 of 3 resources skipped"
  - row color = green, glyph `~`, S1 does NOT bump (spec: `~` never bumps)
  - Summary does NOT contain "PARTIAL" or the integer counts as a substring duplicate of rows (U11)
```

```text
TEST: plan_mixed_failed_and_partial_picks_broken   (covers U7d)
GIVEN: a backup plan "plan-broken-mixed" with:
         job-a State=FAILED   at 2026-04-22 06:00 UTC
         job-b State=PARTIAL  at 2026-04-22 09:00 UTC
         job-c State=COMPLETED at 2026-04-22 12:00 UTC
WHEN:  list is fetched and enricher runs
THEN:
  - Severity = "!"  (Broken > Warning)
  - Summary  = "1 job failed in last 24h"  (the `!` bucket wins, partial info still lives in Rows)
  - Rows include both the failed job's State and the partial count (no finding silently disappears)
  - FieldUpdates[id]["status"] == "1 job failed in last 24h"
  - glyph = `!`
  - S1 bumps +1
```

```text
TEST: plan_job_outside_window_is_ignored
GIVEN: a backup plan "plan-old-failure" with ONE BackupJob State=FAILED created 48h ago (outside the 24h window)
WHEN:  list is fetched and enricher runs
THEN:
  - no EnrichmentFinding emitted for this plan
  - Resource.Status remains blank; FieldUpdates["status"] not set
  - row color green; no glyph; no S1 bump
```

```text
TEST: job_without_backupplanid_is_bucketed_nowhere
GIVEN: a BackupJob with CreatedBy.BackupPlanId == nil (on-demand job, unassociated with a plan)
WHEN:  the enricher walks the ListBackupJobs output
THEN:
  - the job does not produce a finding against any plan
  - no spurious entry in findings map
```

```text
TEST: banned_words_never_appear_in_status_or_detail
GIVEN: any fixture that triggers a Wave 2 finding
WHEN:  list is rendered and detail is opened
THEN:
  - rendered Status cell does NOT contain any of: "Wave 1", "Wave 2", "Wave 3",
    "finding", "enrichment", "probe", "truncated", "lower bound", "bucket", "severity"
  - rendered detail does not contain those either
  - rendered Status cell does NOT carry a bare state keyword alone
    (never just "FAILED", "PARTIAL", "ABORTED", "EXPIRED")
```

```text
TEST: related_pivots_resolve_nonzero_on_graph_root   (covers U9)
GIVEN: the graph-root plan `plan-broken-2failed` with:
         Rules[].TargetBackupVaultName = "acme-default-vault"
         acme-default-vault's EncryptionKeyArn = <some KMS key ARN already in the kms fixture>
         selections attached with IamRoleArn = <iam role ARN already in the role fixture>
         SNS notification configured on acme-default-vault → topic ARN present in sns fixture
WHEN:  detail opens and the related panel checkers run
THEN:
  - kms pivot count >= 1  (matches the vault's EncryptionKeyArn)
  - role pivot count >= 1 (matches IamRoleArn)
  - sns pivot count >= 1 (matches vault notification topic)
  - ct-events pivot count is "unknown" (-1) — not asserted (spec §2 exempts)
```

```text
TEST: out_of_scope_cadence_comparison_is_silent   (anti-test for §3.3 Wave 3)
GIVEN: a backup plan whose rule cadence is daily AND whose most recent successful
       job ran 4 days ago (older than cadence × 2)
WHEN:  list is fetched and enricher runs
THEN:
  - no finding emitted
  - no column, glyph, or phrase mentions staleness
  - row renders green
  - This pins that Wave 3 stays deferred — the ONLY Wave 2 signals are the §3.2 job-state signals
```

```text
TEST: list_view_carries_exactly_one_status_column_backed_by_status_key   (covers U10)
GIVEN: defaults_backup.go default list columns
WHEN:  the list view is rendered
THEN:
  - exactly one column is keyed off `status` (the Status column)
  - no column is keyed off `last_status`
  - no column titled any of: "Last Status", "CIS", "Flags", "Policy", "Issues", "NOBKP"
  - identity columns (Plan Name, Plan ID, Created, Last Execution) are permitted
```

## 2. Fixture list

All fixtures live in `internal/demo/fixtures/backup.go` (single source for
demo + unit tests). Sibling edits go to `kms.go`, `iam.go`, `sns.go`, and
`cloudtrail.go`. Adversarial fixtures (nil pointers, malformed responses) stay
inline in the QA test files — not here.

The existing fixture file has three plans (`acme-daily-backup`,
`acme-weekly-full-backup`, `acme-compliance-30day`) and a recovery-points map.
The coder rewrites `backup.go` to expose the fixture set below and folds the
three legacy plans into the new naming scheme (`plan-healthy-daily` replaces
`acme-daily-backup` as the showroom healthy instance).

### Plans

```text
FIXTURE: plan-healthy-daily (graph-unrelated healthy baseline)
BackupPlanName: "acme-daily-backup"
BackupPlanId:   "11111111-1111-1111-1111-111111111111"
BackupPlanArn:  "arn:aws:backup:us-east-1:123456789012:backup-plan:11111111-1111-1111-1111-111111111111"
CreationDate:   2025-01-15T09:00:00Z
LastExecutionDate: 2026-04-22T02:00:00Z  (~ recent)
Selections: one selection with IamRoleArn = "arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"
             Resources: [HealthyBucketARN, acme-shared EFS ARN]
Rules (via GetBackupPlan):
  - RuleName: "Daily"
    TargetBackupVaultName: "acme-default-vault"
    ScheduleExpression: "cron(0 5 ? * * *)"
Expected behavior: no BackupJobs in last 24h → Healthy silence.
  Row: green, S4 blank, no glyph, Issues empty.
```

```text
FIXTURE: plan-never-ran
BackupPlanName: "acme-newly-created"
BackupPlanId:   "22222222-2222-2222-2222-222222222222"
BackupPlanArn:  "arn:aws:backup:us-east-1:123456789012:backup-plan:22222222-2222-2222-2222-222222222222"
CreationDate:   2026-04-22T18:00:00Z  (very recent)
LastExecutionDate: nil  (never ran)
Selections: none
Rules (via GetBackupPlan): one rule, TargetBackupVaultName: "acme-default-vault"
Jobs: none at all (not even out-of-window).
Expected: Healthy — §4 bullet "A plan that has *never* run is also Healthy".
```

```text
FIXTURE: plan-broken-1failed
BackupPlanName: "acme-prod-critical"
BackupPlanId:   "33333333-3333-3333-3333-333333333333"
BackupPlanArn:  "arn:aws:backup:us-east-1:123456789012:backup-plan:33333333-3333-3333-3333-333333333333"
CreationDate:   2025-06-01T10:00:00Z
LastExecutionDate: 2026-04-22T07:12:00Z
Selections: one selection with IamRoleArn = "arn:aws:iam::123456789012:role/AcmeBackupRoleProd"
Rules: one rule TargetBackupVaultName: "acme-prod-vault"
Jobs (inside ListBackupJobs account-wide output):
  - BackupJobId: "job-33-a"
    State: FAILED
    CreationDate: 2026-04-22T07:12:00Z
    CreatedBy.BackupPlanId: "33333333-3333-3333-3333-333333333333"
    StatusMessage: "Backup vault access denied — check KMS key policy"
    IamRoleArn: "arn:aws:iam::123456789012:role/AcmeBackupRoleProd"
Expected:
  Row: green, glyph `!`, Status = "1 job failed in last 24h", S1 bumps.
```

```text
FIXTURE: plan-broken-2failed (NOMINATED GRAPH-ROOT for U9)
BackupPlanName: "acme-prod-database"
BackupPlanId:   "44444444-4444-4444-4444-444444444444"
BackupPlanArn:  "arn:aws:backup:us-east-1:123456789012:backup-plan:44444444-4444-4444-4444-444444444444"
CreationDate:   2025-04-10T09:00:00Z
LastExecutionDate: 2026-04-22T12:30:00Z
Selections: ONE selection
  SelectionName: "acme-prod-db-selection"
  IamRoleArn:    "arn:aws:iam::123456789012:role/AcmeBackupRoleProd"
  Resources:     ["arn:aws:rds:us-east-1:123456789012:db:acme-prod-primary"]
Rules: one rule
  TargetBackupVaultName: "acme-prod-vault"
  (the vault's EncryptionKeyArn points at an EXISTING KMS key in fixtures/kms.go,
   e.g. the `acme-prod-master-key`. Matching key ID must be added to kms fixtures
   as a sibling edit.)
  (the vault has an SNS notification → SNSTopicArn pointing at an EXISTING
   SNS topic in fixtures/sns.go, e.g. `acme-backup-alerts`. Matching topic
   name must be added to sns fixtures as a sibling edit.)
Jobs (in account-wide ListBackupJobs):
  - BackupJobId: "job-44-a"
    State: FAILED
    CreationDate: 2026-04-22T07:12:00Z
    CreatedBy.BackupPlanId: "44444444-4444-4444-4444-444444444444"
    StatusMessage: "KMSKeyNotAccessibleException: ..."
  - BackupJobId: "job-44-b"
    State: EXPIRED
    CreationDate: 2026-04-22T12:30:00Z  (most recent)
    CreatedBy.BackupPlanId: "44444444-4444-4444-4444-444444444444"
    StatusMessage: "backup job expired past completion window"
Expected:
  Row: green, glyph `!`, Status = "2 jobs failed in last 24h", S1 bumps.
  Related panel on this plan: kms>=1, role>=1, sns>=1, ct-events unknown (exempt).
```

```text
FIXTURE: plan-broken-aborted
BackupPlanName: "acme-staging-hourly"
BackupPlanId:   "55555555-5555-5555-5555-555555555555"
BackupPlanArn:  "arn:aws:backup:us-east-1:123456789012:backup-plan:55555555-5555-5555-5555-555555555555"
LastExecutionDate: 2026-04-22T15:00:00Z
Selections: one, IamRoleArn = "arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"
Rules: one rule TargetBackupVaultName: "acme-default-vault"
Jobs:
  - BackupJobId: "job-55-a"
    State: ABORTED
    CreationDate: 2026-04-22T15:00:00Z
    CreatedBy.BackupPlanId: "55555555-5555-5555-5555-555555555555"
    StatusMessage: "Backup job aborted by user"
Expected:
  Status = "1 job failed in last 24h", glyph `!`, S1 bumps (ABORTED counts as "failed" per §3.2).
```

```text
FIXTURE: plan-warning-partial
BackupPlanName: "acme-app-data"
BackupPlanId:   "66666666-6666-6666-6666-666666666666"
BackupPlanArn:  "arn:aws:backup:us-east-1:123456789012:backup-plan:66666666-6666-6666-6666-666666666666"
LastExecutionDate: 2026-04-22T06:00:00Z
Selections: one, IamRoleArn = "arn:aws:iam::123456789012:role/service-role/AWSBackupDefaultServiceRole"
Rules: one rule TargetBackupVaultName: "acme-default-vault"
Jobs (three jobs for this plan in the 24h window):
  - BackupJobId: "job-66-a" State: COMPLETED CreationDate: 2026-04-22T06:00:00Z CreatedBy.BackupPlanId: "66666666..."
  - BackupJobId: "job-66-b" State: COMPLETED CreationDate: 2026-04-22T06:01:00Z CreatedBy.BackupPlanId: "66666666..."
  - BackupJobId: "job-66-c" State: PARTIAL   CreationDate: 2026-04-22T06:02:00Z CreatedBy.BackupPlanId: "66666666..."
Expected:
  Row: green, glyph `~`, Status = "partial: 1 of 3 resources skipped", S1 does NOT bump.
```

```text
FIXTURE: plan-broken-mixed (U7d — `!` beats `~`)
BackupPlanName: "acme-compliance-mixed"
BackupPlanId:   "77777777-7777-7777-7777-777777777777"
BackupPlanArn:  "arn:aws:backup:us-east-1:123456789012:backup-plan:77777777-7777-7777-7777-777777777777"
LastExecutionDate: 2026-04-22T09:00:00Z
Selections: one, IamRoleArn = "arn:aws:iam::123456789012:role/AcmeBackupRoleProd"
Rules: one rule TargetBackupVaultName: "acme-prod-vault"
Jobs:
  - BackupJobId: "job-77-a" State: FAILED   CreationDate: 2026-04-22T06:00:00Z CreatedBy.BackupPlanId: "77777777..."
  - BackupJobId: "job-77-b" State: PARTIAL  CreationDate: 2026-04-22T09:00:00Z CreatedBy.BackupPlanId: "77777777..."
  - BackupJobId: "job-77-c" State: COMPLETED CreationDate: 2026-04-22T12:00:00Z CreatedBy.BackupPlanId: "77777777..."
Expected:
  Row: green, glyph `!` (Broken beats Warning), Status = "1 job failed in last 24h".
  Rows include BOTH the failed-job State and the partial-job count so nothing silently disappears.
```

```text
FIXTURE: plan-old-failure (window-exclusion test)
BackupPlanName: "acme-dev-sporadic"
BackupPlanId:   "88888888-8888-8888-8888-888888888888"
BackupPlanArn:  "arn:aws:backup:us-east-1:123456789012:backup-plan:88888888-8888-8888-8888-888888888888"
LastExecutionDate: 2026-04-20T15:00:00Z
Selections: one
Rules: one rule TargetBackupVaultName: "acme-default-vault"
Jobs:
  - BackupJobId: "job-88-a" State: FAILED CreationDate: 2026-04-20T15:00:00Z  (48h+ old)
    CreatedBy.BackupPlanId: "88888888-..."
Expected: Healthy silence — job outside window, ignored.
```

### Sibling edits (required for graph-root U9)

All edits are additive — existing fixtures keep their identities and IDs.

- **`internal/demo/fixtures/kms.go`** — ensure a KMS key entry has
  `ID = "acme-prod-master-key"` (or equivalent key ID — the key ID is the
  trailing segment after `key/` in the ARN that `DescribeBackupVault` returns).
  If the key already exists, reference its ID. If not, ADD it — do not rename
  existing keys.
- **`internal/demo/fixtures/iam.go`** — ensure an IAM role exists with ARN
  `arn:aws:iam::123456789012:role/AcmeBackupRoleProd` (role name
  `AcmeBackupRoleProd`). The default `AWSBackupDefaultServiceRole` already
  exists in iam fixtures (confirm by grep; add only if missing).
- **`internal/demo/fixtures/sns.go`** — ensure an SNS topic exists with name
  `acme-backup-alerts` so the sns pivot on `plan-broken-2failed` resolves.
- **`internal/demo/fixtures/cloudtrail.go`** — optional; ct-events pivot is
  `count shown: unknown` and exempt from U9. Add 1–2 events referencing
  `plan-broken-2failed`'s BackupPlanArn only if cheap; otherwise skip.

### Adversarial fixtures (stay inline in tests/unit/)

These do NOT land in `internal/demo/fixtures/backup.go`. Each lives inside the
QA test that needs it.

- `BackupJob` with `CreationDate == nil` → enricher must skip (not crash).
- `BackupJob` with `CreatedBy == nil` → enricher must skip.
- `BackupJob` with `CreatedBy.BackupPlanId == nil` → enricher must skip.
- `ListBackupJobs` returning an API error → enricher returns the error unwrapped;
  does not partially populate findings.
- Plan with `BackupPlanId == nil` → skipped by fetcher, no panic.

## 3. Contract-surface gap analysis

| Surface | Spec demands | Current code | Delta |
|---|---|---|---|
| `backup_interfaces.go` | List/Describe/GetBackupPlan/Selections/Vault/Notifications/Jobs | All seven present | ✅ no change |
| `backup_related.go` | kms, role, sns, ct-events (count-shown: yes for first 3, unknown for ct-events) | role, kms, sns registered; ct-events auto-registered via `zzz_ct_events_all_related.go` | ✅ no change |
| `backup_issue_enrichment.go` Summary text | "2 jobs failed in last 24h", "partial: 1 of 3 resources skipped" (§4 S4 column) | "backup FAILED in last 24h", "backup PARTIAL in last 24h" — no count, bare state keyword embedded | 🔴 REWRITE Summary + phrase-building logic |
| `backup_issue_enrichment.go` FieldUpdates | Must write `status` (§4 maps Wave-2 text onto the Status column) | Writes `last_status` (raw state keyword) | 🔴 REPOINT to `status`, drop `last_status` writes |
| `backup_issue_enrichment.go` precedence | When both `!` and `~` job states exist on the same plan, `!` wins; Rows still include partial info | First iteration order wins — nondeterministic | 🔴 REWRITE precedence logic |
| `backup.go` fetcher | `Resource.Status = ""` always (no Wave 1 signals per §3.1); `Resource.Issues = nil` | Already sets Status ""; no Issues logic needed | ✅ no change to Wave-1 branch; drop `enumerateBackupPlanResources` CSV work if it is not referenced by any §2 pivot (the role pivot now reads `BackupSelection.IamRoleArn` directly) — KEEP because s3→backup and efs→backup use this field to cache-scan |
| `defaults_backup.go` list columns | One Status column keyed `status`; identity + metadata columns allowed | `Last Status` keyed `last_status` (banned bare state) | 🔴 REPLACE `Last Status/last_status` with `Status/status`; keep Plan Name, Plan ID, Created, Last Execution |
| `.a9s/views/backup.yaml` | Regenerated from defaults | Out-of-sync after defaults change | 🔴 regenerate via `go run ./cmd/viewsgen/` |
| `backup_detail_enrichment.go` | Spec §2 demands no per-field detail enrichment (related panel is separate) | Absent | ✅ do NOT create |

### Files that must change

- `internal/aws/backup_issue_enrichment.go` — rewrite.
- `internal/aws/backup.go` — minor: ensure `Issues: nil` explicitly, ensure
  Status always blank. Keep the resource CSV population (consumed by sibling
  pivots, not by backup's own signals).
- `internal/config/defaults_backup.go` — swap Last Status column for Status.
- `.a9s/views/backup.yaml` — regenerate.
- `internal/demo/fixtures/backup.go` — replace with the §2 set.
- `internal/demo/fakes/backup.go` — may need extension:
  - `GetBackupPlan` must return the plan's Rules (currently returns empty).
  - `DescribeBackupVault` must return EncryptionKeyArn (currently empty).
  - `GetBackupVaultNotifications` must return SNSTopicArn when the vault has one.
  - `ListBackupJobs` must return the fixture's Jobs slice (currently empty).
    The coder running the `a9s-create-demo-fixture` skill will add fields to the
    BackupFixtures struct and wire the fakes accordingly.

### Files that must NOT change

- `internal/aws/backup_interfaces.go` — already complete.
- `internal/aws/backup_related.go` — already correct (ct-events auto-registered
  elsewhere).
- `internal/aws/ct_events.go`, `zzz_ct_events_all_related.go` — universal pivot
  infrastructure, untouched.

### Resolved TBDs / deferred / out-of-scope

- All §2 pivot discovery TBDs were already resolved with `a9s-devops 2026-04-20`
  citations in spec §6. No phase-2 user questions needed.
- Wave 3 cadence-comparison stays deferred per §3.3.
- `backup`→`eb-rule` and `backup`→`logs` stay excluded per `docs/related-resources.md`.
- Write operations remain out of scope (read-only architecture invariant).
