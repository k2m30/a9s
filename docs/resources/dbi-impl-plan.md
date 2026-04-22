---
shortName: dbi
generatedFrom: docs/resources/dbi.md
purpose: Implementation plan derived from the dbi spec. Pseudocode tests + fixture list + contract-surface gap analysis. Not re-entrant — regenerate from spec on any amendment.
---

# dbi — Implementation Plan

## 0. Preconditions

- Spec: `docs/resources/dbi.md` (user-approved 2026-04-21; no TBDs remain).
- Fetcher lives at `internal/aws/rds.go` (shared RDS service file, NOT `internal/aws/dbi.go`).
- Wave 2: one signal (`DescribePendingMaintenanceActions`). Severity `~` on Healthy rows.

## 1. Behavioral test spec (pseudocode)

One case per signal row in spec §4, plus silence + anti-tests. The QA agent turns each into a Go test in `tests/unit/aws_dbi*_test.go`.

### 1.1 Wave 1 — DBInstanceStatus status bucket

#### TEST: available_healthy_silence

GIVEN: a DBInstance with `DBInstanceStatus = "available"`, `BackupRetentionPeriod = 7`, `PubliclyAccessible = false`, `StorageEncrypted = true`, `DeletionProtection = true`.
WHEN: the list is fetched and rendered.
THEN:
- `Resource.Status == ""`  (blank — banned strings: `OK`, `ACTIVE`, `available`, `running`, `healthy`, `-`).
- No Wave 2 finding.
- S1 issues count does NOT bump.
- No `!` / `~` glyph.

#### TEST: transitional_modifying_with_pending_class_change

GIVEN: `DBInstanceStatus = "modifying"`, `PendingModifiedValues.DBInstanceClass = "db.r6g.xlarge"`.
WHEN: fetched.
THEN:
- `Resource.Status == "modifying: DBInstanceClass"` (exact string — `<status>: <first non-empty PendingModifiedValues key>`).
- Wave 1 only — no finding added by Wave 2.
- Row color expectation: Warning bucket (renderer's job to map "modifying" → yellow).

#### TEST: transitional_rebooting_no_pending

GIVEN: `DBInstanceStatus = "rebooting"`, `PendingModifiedValues` all-empty.
THEN:
- `Resource.Status == "rebooting"` (bare state keyword — no suffix when no pending value is set).

#### TEST: transitional_all_other_keywords

Parameterize over: `creating`, `backing-up`, `renaming`, `resetting-master-credentials`, `starting`, `stopping`, `upgrading`, `maintenance`, `configuring-enhanced-monitoring`, `configuring-iam-database-auth`, `configuring-log-exports`, `converting-to-vpc`, `moving-to-vpc`, `storage-optimization`.
Each with empty `PendingModifiedValues`.
THEN: `Resource.Status == "<status>"` (bare).

#### TEST: broken_failed

GIVEN: `DBInstanceStatus = "failed"`.
THEN: `Resource.Status == "failed"`.

#### TEST: broken_storage_full

GIVEN: `DBInstanceStatus = "storage-full"`.
THEN: `Resource.Status == "storage-full"`.

#### TEST: broken_incompatible_network

GIVEN: `DBInstanceStatus = "incompatible-network"`.
THEN: `Resource.Status == "incompatible-network"`.

#### TEST: broken_incompatible_option_group

GIVEN: `DBInstanceStatus = "incompatible-option-group"`.
THEN: `Resource.Status == "incompatible-option-group"`.

#### TEST: broken_incompatible_parameters

GIVEN: `DBInstanceStatus = "incompatible-parameters"`.
THEN: `Resource.Status == "incompatible-parameters"`.

#### TEST: broken_incompatible_restore

GIVEN: `DBInstanceStatus = "incompatible-restore"`.
THEN: `Resource.Status == "incompatible-restore"`.

#### TEST: broken_restore_error

GIVEN: `DBInstanceStatus = "restore-error"`.
THEN: `Resource.Status == "restore-error"`.

#### TEST: broken_inaccessible_encryption_credentials

GIVEN: `DBInstanceStatus = "inaccessible-encryption-credentials"`.
THEN: `Resource.Status == "encryption key unavailable"` (exact §4 "List text" remap).

#### TEST: broken_precedence_over_config_warnings

GIVEN: `DBInstanceStatus = "storage-full"`, `BackupRetentionPeriod = 0`, `PubliclyAccessible = true`, `StorageEncrypted = false`, `DeletionProtection = false`.
THEN: `Resource.Status == "storage-full"` (broken beats config warnings; no stacking).

### 1.2 Wave 1 — config warnings on Available row

#### TEST: available_no_automated_backups

GIVEN: `DBInstanceStatus = "available"`, `BackupRetentionPeriod = 0`, all other config OK.
THEN: `Resource.Status == "no automated backups"`.

#### TEST: available_publicly_accessible

GIVEN: `DBInstanceStatus = "available"`, `PubliclyAccessible = true`, backups OK, encrypted, protected.
THEN: `Resource.Status == "publicly accessible"`.

#### TEST: available_unencrypted_storage

GIVEN: `DBInstanceStatus = "available"`, `StorageEncrypted = false`, backups OK, not-public, protected.
THEN: `Resource.Status == "unencrypted storage"`.

#### TEST: available_deletion_protection_off

GIVEN: `DBInstanceStatus = "available"`, `DeletionProtection = false`, backups OK, not-public, encrypted.
THEN: `Resource.Status == "deletion protection off"`.

#### TEST: warning_precedence

GIVEN: `DBInstanceStatus = "available"`, `BackupRetentionPeriod = 0`, `PubliclyAccessible = true`, `StorageEncrypted = false`, `DeletionProtection = false` (all four warnings set).
THEN: `Resource.Status == "no automated backups"` (first in spec §4 precedence: backups > public > unencrypted > no-protection).

### 1.3 Wave 2 — DescribePendingMaintenanceActions

#### TEST: maintenance_pending_on_healthy_row

GIVEN: Healthy DBInstance `prod-dbi-1` AND `DescribePendingMaintenanceActions` returns one `ResourcePendingMaintenanceActions{ResourceIdentifier=<prod-dbi-1 ARN>, PendingMaintenanceActionDetails=[{Action="system-update", Description="New minor engine patch 16.2.3", AutoAppliedAfterDate=<past>}]}`.
WHEN: enricher runs.
THEN:
- `result.Findings["prod-dbi-1"].Severity == "~"`.
- `result.Findings["prod-dbi-1"].Summary` matches `^Pending maintenance action overdue: system-update \(New minor engine patch 16\.2\.3\)\.$`.
- `result.FieldUpdates["prod-dbi-1"]["status"] == "maintenance scheduled"`.
- `result.IssueCount == 0` (Wave 2 ~ does NOT bump S1 badge).

#### TEST: maintenance_pending_no_description

GIVEN: maintenance action with `Action="os-upgrade"` but `Description` nil.
THEN: `Summary == "Pending maintenance action overdue: os-upgrade."` (empty parens collapsed).

#### TEST: maintenance_does_not_overwrite_nonhealthy_status

GIVEN: DBInstance in Warning bucket (`Resource.Status = "publicly accessible"` already set by fetcher) AND pending-maintenance action.
THEN:
- `result.Findings[...]` contains the finding (for S5 visibility).
- `result.FieldUpdates[...]` does NOT set `status` (preserves Wave-1 phrase).

#### TEST: maintenance_no_match_no_finding

GIVEN: `DescribePendingMaintenanceActions` returns ARNs for instances NOT in the resources slice.
THEN: `result.Findings` is empty, `result.Truncated == false`.

#### TEST: maintenance_rds_clients_nil

GIVEN: `clients.RDS == nil`.
THEN: returns empty result, no error.

### 1.4 Related-panel discovery (one per §2 target)

For each fixture `prod-dbi-1` wired to the graph:

- `checkDbiSG(res)` returns the SG IDs from `VpcSecurityGroups[]`.
- `checkDbiKMS(res)` returns `[kmsKeyUUID]` when `StorageEncrypted && KmsKeyId set`; Count 0 otherwise.
- `checkDbiSubnets(res)` returns all `DBSubnetGroup.Subnets[].SubnetIdentifier`.
- `checkDbiVPC(res)` returns `[DBSubnetGroup.VpcId]`.
- `checkDbiAlarm(res, cache)` returns alarm IDs whose `Dimensions[].Name=="DBInstanceIdentifier"` matches `res.ID`.
- `checkDbiRDSSnap(res, cache)` returns snapshot IDs whose `DBInstanceIdentifier == res.ID`.
- `checkDBILogs(res, cache)` returns log-group IDs that start with `/aws/rds/instance/<res.ID>/`.
- `checkDbiSecrets(res, cache)` returns the secret ID when `MasterUserSecret.SecretArn` matches a cached secret's ARN.
- `checkDbiDBC(res, cache)` returns `[DBClusterIdentifier]` when the cluster is cached; 0 for non-Aurora.
- `checkDbiRole(res)` returns role names (last ARN segment) from `AssociatedRoles[].RoleArn` + `MonitoringRoleArn`.
- `checkDbiENI(ctx, clients, res)` calls `DescribeNetworkInterfaces(Filters=[description=RDSNetworkInterface, group-id=<sg-ids>])` and returns the returned ENI IDs.
- `checkDbiCTEvents(res, cache)` returns CloudTrail event IDs for events whose `resource_name == res.ID`; `Count = -1` when cache is truncated (count unknown, pivot still navigable); `FetchFilter["ResourceName"] = res.ID`.

### 1.5 Anti-tests (Wave 3 OUT OF SCOPE per spec §3.3)

- **TEST: no_cloudwatch_probe_for_free_storage** — no mock call to `cloudwatch.GetMetricStatistics` on `FreeStorageSpace`.
- **TEST: no_cloudwatch_probe_for_cpu** — no mock call to `cloudwatch.GetMetricStatistics` on `CPUUtilization`.
- **TEST: no_cloudwatch_probe_for_replica_lag** — no mock call on `ReplicaLag`.
- **TEST: no_cloudwatch_probe_for_connections** — no mock call on `DatabaseConnections`.

Assertion mechanism: tests use a mock CloudWatch API that records all calls and asserts zero during dbi fetch + enrich.

### 1.6 Status column universal rules (asserted in phase 8 visual test)

- No `CIS` / `Flags` / `Policy` / `Issues` column anywhere in the rendered frame.
- Healthy rows render blank S4.
- Warning/Broken rows render the exact spec §4 phrase.
- `~` glyph only on Healthy-with-finding rows.
- S1 menu badge counts zero (all Wave 2 findings are `~`, not `!`).

## 2. Fixture list (plain language)

All fixtures live in `internal/demo/fixtures/dbi.go` as a single `DBIFixtures` struct with a `NewDBIFixtures() *DBIFixtures` constructor. Tests and `./a9s --demo` import the same constants. Refactor existing `internal/demo/fixtures/rds.go` (currently `RDSFixtures` with DBInstances + DBSnapshots + Events):

- Move `DBInstances` + related constants/helpers into new `dbi.go` under `DBIFixtures{Instances []rdstypes.DBInstance}`.
- `DBSnapshots` stays in `rds.go` (or moves to `rds-snap.go`; owner type is `rds-snap`, out of this run's scope).
- `Events` stays where it is.
- Keep the old `NewRDSFixtures()` as a thin aggregator for backwards compatibility, OR update the two callers (`internal/demo/fakes/rds.go`, `internal/demo/fixtures/counts.go`). Coder chooses the least-invasive option.

### 2.1 Fixtures (non-adversarial)

Each fixture uses the account `123456789012`, region `us-east-1`, subnet group `acme-rds-subnet-group`, VPC `vpc-0abc123def456789a` (align with existing sibling fixtures).

#### FIXTURE: prod-dbi-1 (baseline Healthy, graph-connected)

- `DBInstanceIdentifier = "prod-dbi-1"`, ARN = `arn:aws:rds:us-east-1:123456789012:db:prod-dbi-1`.
- `Engine = "postgres"`, `EngineVersion = "16.2"`, `DBInstanceClass = "db.r6g.large"`, `Endpoint.Address = "prod-dbi-1.xxxxxxx.us-east-1.rds.amazonaws.com"`, `Endpoint.Port = 5432`.
- `DBInstanceStatus = "available"`, `MultiAZ = true`.
- `BackupRetentionPeriod = 7`, `PubliclyAccessible = false`, `StorageEncrypted = true`, `DeletionProtection = true`.
- `KmsKeyId = "arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"` (matches existing kms fixture).
- `VpcSecurityGroups = [{VpcSecurityGroupId: "sg-0ccc333333333333c"}]` (matches existing sg fixture).
- `DBSubnetGroup.VpcId = "vpc-0abc123def456789a"`; `DBSubnetGroup.Subnets = [{SubnetIdentifier: "subnet-0111111111111111a"}, {SubnetIdentifier: "subnet-0222222222222222b"}]` (matches subnet fixtures).
- `MasterUserSecret.SecretArn = "arn:aws:secretsmanager:us-east-1:123456789012:secret:rds!db-prod-dbi-1-ABCDEF"` (matches secrets fixture — add sibling if absent).
- `AssociatedRoles = [{RoleArn: "arn:aws:iam::123456789012:role/rds-monitoring-role"}]`, `MonitoringRoleArn = "arn:aws:iam::123456789012:role/rds-enhanced-monitoring"` (match role fixtures).
- `EnabledCloudwatchLogsExports = ["postgresql", "upgrade"]` (log groups `/aws/rds/instance/prod-dbi-1/postgresql`, `.../upgrade`).
- `DBClusterIdentifier = nil` (not Aurora — makes this RDS, not a cluster member).

Sibling graph must contain:
- `alarm` with `MetricAlarm.Dimensions=[{Name:"DBInstanceIdentifier", Value:"prod-dbi-1"}]`.
- `rds-snap` with `DBInstanceIdentifier = "prod-dbi-1"`.
- `logs` groups `/aws/rds/instance/prod-dbi-1/postgresql` and `/aws/rds/instance/prod-dbi-1/upgrade`.
- `kms`, `sg`, `subnet`, `vpc`, `secrets`, `role` targets already referenced above (add if missing).
- `ct-events` entries with `resource_name = "prod-dbi-1"`.

#### FIXTURE: prod-dbi-aurora-member (Aurora cluster member, Healthy)

- Same as prod-dbi-1 but:
  - `DBInstanceIdentifier = "prod-dbi-aurora-1"`.
  - `Engine = "aurora-postgresql"`, `EngineVersion = "16.4"`.
  - `DBClusterIdentifier = "prod-aurora-cluster"` (matches existing `dbc` fixture).
  - No `MasterUserSecret` (Aurora typically uses cluster-level secrets).

#### FIXTURE: staging-dbi-modifying (Warning — transitional with pending class change)

- `DBInstanceIdentifier = "staging-dbi-modifying"`.
- `DBInstanceStatus = "modifying"`.
- `PendingModifiedValues.DBInstanceClass = "db.r6g.xlarge"` (aws.String).
- Otherwise same defaults as prod-dbi-1 (Healthy config), so Wave-1 config warnings don't fire.
- Expected: `Status == "modifying: DBInstanceClass"`.

#### FIXTURE: staging-dbi-rebooting (Warning — transitional, no pending values)

- `DBInstanceIdentifier = "staging-dbi-rebooting"`.
- `DBInstanceStatus = "rebooting"`.
- `PendingModifiedValues = nil` or all-zero.
- Expected: `Status == "rebooting"` (bare).

#### FIXTURE: broken-dbi-storage-full (Broken)

- `DBInstanceIdentifier = "broken-dbi-storage-full"`.
- `DBInstanceStatus = "storage-full"`.
- Otherwise Healthy config.
- Expected: `Status == "storage-full"`.

#### FIXTURE: broken-dbi-encryption-locked (Broken — inaccessible-encryption-credentials)

- `DBInstanceIdentifier = "broken-dbi-encryption-locked"`.
- `DBInstanceStatus = "inaccessible-encryption-credentials"`.
- `StorageEncrypted = true`, `KmsKeyId = "arn:aws:kms:us-east-1:123456789012:key/deadbeef-0000-0000-0000-000000000000"` (a deleted/disabled key in the kms fixture).
- Expected: `Status == "encryption key unavailable"` (remap per §4).

#### FIXTURE: warn-dbi-no-backups (Warning — BackupRetentionPeriod=0)

- `DBInstanceIdentifier = "warn-dbi-no-backups"`.
- `DBInstanceStatus = "available"`.
- `BackupRetentionPeriod = 0`.
- Expected: `Status == "no automated backups"`.

#### FIXTURE: warn-dbi-publicly-accessible (Warning — CIS RDS.2)

- `DBInstanceIdentifier = "warn-dbi-public"`.
- `DBInstanceStatus = "available"`.
- `PubliclyAccessible = true`. Everything else Healthy.
- Expected: `Status == "publicly accessible"`.

#### FIXTURE: warn-dbi-unencrypted (Warning — CIS RDS.3)

- `DBInstanceIdentifier = "warn-dbi-unencrypted"`.
- `DBInstanceStatus = "available"`.
- `StorageEncrypted = false`, `KmsKeyId = nil`. Everything else Healthy.
- Expected: `Status == "unencrypted storage"`.

#### FIXTURE: warn-dbi-unprotected (Warning — DeletionProtection=false)

- `DBInstanceIdentifier = "warn-dbi-unprotected"`.
- `DBInstanceStatus = "available"`.
- `DeletionProtection = false`. Everything else Healthy.
- Expected: `Status == "deletion protection off"`.

#### FIXTURE: maint-dbi-scheduled (Healthy + pending maintenance)

- `DBInstanceIdentifier = "maint-dbi-scheduled"`.
- `DBInstanceStatus = "available"`. Config all Healthy.
- Also in `PendingMaintenanceActions` fixture list: one `ResourcePendingMaintenanceActions` with `ResourceIdentifier = ARN` and `PendingMaintenanceActionDetails = [{Action: "system-update", Description: "New minor engine patch 16.2.3", AutoAppliedAfterDate: 2026-04-01}]`.
- Expected Wave 2: `Findings["maint-dbi-scheduled"].Severity = "~"`, `Summary = "Pending maintenance action overdue: system-update (New minor engine patch 16.2.3)."`, `FieldUpdates[id]["status"] = "maintenance scheduled"`.
- Rendered row: green with `~ maint-dbi-scheduled` prefix, S4 shows `maintenance scheduled`.

### 2.2 Sibling-graph touchups

The coder's fixture work ensures every §2 pivot renders a non-zero count for the graph-root fixtures (prod-dbi-1 at minimum). Targets that need alignment:

- `alarm` fixture — add a `DBInstanceIdentifier` dimension entry for `prod-dbi-1`.
- `kms` fixture — already contains `a1b2c3d4-...` key; add disabled `deadbeef-...` for the broken-encryption fixture.
- `rds-snap` fixture — add at least two snapshots with `DBInstanceIdentifier = "prod-dbi-1"`.
- `logs` fixture — add log groups `/aws/rds/instance/prod-dbi-1/postgresql` and `.../upgrade`.
- `secrets` fixture — add the RDS-managed secret for `prod-dbi-1`.
- `role` fixture — ensure `rds-monitoring-role` and `rds-enhanced-monitoring` exist.
- `sg`, `subnet`, `vpc` — existing IDs already referenced; reuse.
- `dbc` fixture — ensure `prod-aurora-cluster` exists for the Aurora-member fixture.
- `ct-events` fixture — add at least two events whose `ResourceName` (via `Resources[].ResourceName`) is `prod-dbi-1`.

Adversarial cases (nil pointers, error paths, API-throttle) stay inline in the test file — they corrupt the showroom.

## 3. Contract-surface gap analysis

### 3.1 Column config (JARGON — must be removed)

**`internal/config/defaults_databases.go` dbi block currently declares:**

```text
{Title: "CIS", Path: "DBInstanceStatus", Key: "cis_flags", Width: 24},
```

This is the CIS jargon column (`PUB|UNENC|NOBKP|NOPROT`). The spec's universal UI rules forbid jargon columns. REMOVE.

**Status column binding:** currently `{Title: "Status", Path: "DBInstanceStatus", Width: 14}`. Rebind to the fetcher's computed Status:
```text
{Title: "Status", Key: "status", Path: "DBInstanceStatus", Width: 22},
```
(Width bump from 14 → 22 to fit longest phrase `encryption key unavailable` = 26 chars. Round up.)
Actually 26 > 22. Use `Width: 28`.

### 3.2 Fetcher (`internal/aws/rds.go` FetchRDSInstancesPage)

Current behavior:
- Writes raw `DBInstanceStatus` to `Resource.Status`.
- Computes `cis_flags` from four booleans and concatenates with `|`.
- Exposes `publicly_accessible`, `storage_encrypted`, `deletion_protection`, `backup_retention_period`, `cis_flags` in `Fields`.

Required behavior per spec:
- `Resource.Status` = derived per §4 precedence (broken > transitional > available-with-config-warning > blank).
- Delete the `cis_flags` computation and the `cis_flags` Fields entry.
- Keep `publicly_accessible`, `storage_encrypted`, `deletion_protection`, `backup_retention_period` in Fields for detail/reference (but NOT as list columns).
- `Fields["status"]` = same as `Resource.Status` (for column key binding).
- Update `resource.RegisterFieldKeys("dbi", ...)` to drop `cis_flags`.

Algorithm (coder pseudocode):

```text
phrase = ""
switch DBInstanceStatus:
  case "available":
    if BackupRetentionPeriod == 0: phrase = "no automated backups"
    elif PubliclyAccessible: phrase = "publicly accessible"
    elif !StorageEncrypted: phrase = "unencrypted storage"
    elif !DeletionProtection: phrase = "deletion protection off"
    else: phrase = ""  // Healthy silence
  case transitional set (14 keywords):
    key = firstNonEmptyPendingModifiedValueKey(PendingModifiedValues)  // "" when none
    phrase = status  if key == ""  else  status + ": " + key
  case "failed" | "storage-full" | "incompatible-network" | "incompatible-option-group" | "incompatible-parameters" | "incompatible-restore" | "restore-error":
    phrase = status
  case "inaccessible-encryption-credentials":
    phrase = "encryption key unavailable"
  default:
    phrase = status  // unknown status — pass through
```

`firstNonEmptyPendingModifiedValueKey` uses explicit priority order (NOT reflection) over the fields actually present on `rdstypes.PendingModifiedValues`:

```text
DBInstanceClass, AllocatedStorage, MasterUserPassword, Port, BackupRetentionPeriod,
MultiAZ, EngineVersion, LicenseModel, Iops, DBInstanceIdentifier, StorageType,
CACertificateIdentifier, DBSubnetGroupName, PendingCloudwatchLogsExports,
ProcessorFeatures, IAMDatabaseAuthenticationEnabled, AutomationMode,
ResumeFullAutomationModeTime, StorageThroughput, Engine, DedicatedLogVolume,
MultiTenant, RotateMasterUserPassword
```

Return the first field whose value is non-nil / non-empty. Deterministic and config-free.

### 3.3 Related panel (`internal/aws/dbi_related.go`)

All 11 non-universal targets already registered. Gap: `ct-events` NOT registered. Add:

```text
{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkDbiCTEvents, NeedsTargetCache: true},
```

Implement `checkDbiCTEvents` mirroring `checkEC2CloudTrailEvents` in `internal/aws/ec2_related.go`:
- Scan cached `ct-events` for entries where `fields["resource_name"] == res.ID` (or scan raw `cloudtrailtypes.Event` via `cloudTrailEventMentionsInstance`).
- `FetchFilter["ResourceName"] = res.ID` always set.
- `Count = -1` when cache is truncated (spec §2 ct-events: "count shown: unknown").

### 3.4 Wave 2 enricher (`internal/aws/dbi_issue_enrichment.go`)

Currently registers the shared `EnrichRDSDocDBMaintenance`. Per spec, dbi needs a dbi-specific enricher because:
1. Spec §3.2 S5 shape: `Pending maintenance action overdue: <ActionType> (<Description>).` — different from shared's `pending maintenance: <actions>`.
2. Spec requires `FieldUpdates[id]["status"] = "maintenance scheduled"` only on Healthy rows.
3. Scope: dbi-only. Shared enricher is still consumed by `rds` and `dbc` which have their own specs.

Action: create `EnrichDBIMaintenance` in `internal/aws/dbi_issue_enrichment.go` (replaces the shared dispatch), operating on DB-instance ARNs only.

### 3.5 Detail-view enricher

Spec §2 does NOT require a dedicated `dbi_detail_enrichment.go`. Skip.

### 3.6 Interfaces file

`internal/aws/rds_interfaces.go` already provides all needed interfaces (`RDSDescribeDBInstancesAPI`, `RDSDescribePendingMaintenanceAPI`, aggregate `RDSAPI`). No `dbi_interfaces.go` needed — the aggregate lives at the service level.

### 3.7 Detail config

Keep existing `Detail` field list in defaults_databases.go unchanged. Spec §2 does not mandate changes.

## 4. Approved scope for phase 7.5 diff gate

Phase 6a (fixtures) allowed paths:
- `internal/demo/fixtures/dbi.go` (NEW)
- `internal/demo/fixtures/rds.go` (refactor — extract DBInstance bits)
- `internal/demo/fakes/rds.go` (update NewRDSFake wiring if fixtures split)
- `internal/demo/fixtures/counts.go` (update dbi count wiring)
- sibling fixture edits in `internal/demo/fixtures/{alarm,kms,subnet,vpc,sg,secrets,role,rds-snap,logs,cloudtrail,dbc}.go` (any of these that must grow for graph completeness)

Phase 6b (QA) allowed paths:
- `tests/unit/aws_dbi_test.go` (fetcher + universal invariants)
- `tests/unit/aws_dbi_related_test.go` (per-target checker tests)
- `tests/unit/aws_dbi_issue_enrichment_test.go` (Wave 2 enricher)
- Delete stale: any `tests/unit/aws_dbi_*.go` from prior runs — the runner `rm`s these before dispatch.

Phase 7 (coder implementation) allowed paths:
- `internal/aws/rds.go` (fetcher body — Status derivation)
- `internal/aws/dbi_related.go` (ct-events addition)
- `internal/aws/dbi_issue_enrichment.go` (replace shared dispatch)
- `internal/config/defaults_databases.go` (column config)
- `.a9s/views/dbi.yaml` (regenerated)
- `internal/aws/dbi_interfaces.go` (ONLY if coder decides a narrow interface is needed — currently not required; if created, must be in scope).
- `internal/resource/types_databases.go` (dbi block only — the `Color` callback MUST map the new Status phrases to color buckets; the old implementation read raw DBInstanceStatus keywords which no longer exist on `Resource.Status`. Approved expansion 2026-04-21.)

Phase 8 (runner):
- `tests/integration/scenario_dbi_visual_test.go` (NEW — render-gate test).

Anything else touched = SCOPE VIOLATION.

## 5. Out-of-scope reminders for the coder

- No new list column beyond Status/name/engine/version/class/endpoint/multi-az.
- No `cis_flags` / `CIS` / `PUB` / `UNENC` / `NOBKP` / `NOPROT` anywhere in production code.
- No CloudWatch calls in the dbi fetcher or dbi enricher (Wave 3 out-of-scope).
- No per-instance `DescribeDBInstances` (list API returns full shape).
- No write operations.
- No changes to rds / dbc / rds-snap specs or behaviors.
