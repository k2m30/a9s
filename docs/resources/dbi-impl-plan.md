# dbi — Implementation Plan

Pseudocode test spec + fixture list + contract surface gap analysis for the `dbi` resource.
Source of truth: `docs/resources/dbi.md`.

## 0. TBD Resolution Log

The spec has no open `TBD` markers — §6 citations already carry `a9s-devops (2026-04-20)` decisions for every discovery mechanism and for count-shown policy. No user-facing questions remain.

Architect-autoresolved (2026-04-21) decisions (recorded here; also summarized in spec §6):

- **Maintenance-finding row-state interaction**: When a DB row is already Warning/Broken from a Wave 1 signal, Wave 2 pending-maintenance still attaches as a §4 finding but the `~` glyph (S3) is suppressed per §4 wave-to-surface mapping row 5 ("S3 suppressed on non-green rows"). S5 sentence still renders on the detail view. Rationale: consistent with attention-signals.md — glyphs never appear on non-green rows. Citation added to §6.
- **S4 text on the list row for Wave 2 maintenance on Healthy**: `maintenance scheduled` verbatim (per §4 table). Populated via the Wave 2 enricher's Finding.Summary projected onto the row's Status column at render time. The fetcher itself does NOT populate Status with `maintenance scheduled` (it has no access to pending-maintenance data). Rationale: enricher is the only layer that knows about maintenance; view-layer/Status projection is out of scope for the fetcher tests.
- **Bare `modifying` UX follow-up from §4.1**: deferred. Surfacing `PendingModifiedValues` first-key is recommended in §4.1 but NOT in the §4 signal table. Treated as a nice-to-have, not a contract requirement. Rationale: §4 is the authoritative contract; §4.1 is commentary. If not in §4, it is not tested.

## 1. Behavioral Test Spec (Pseudocode)

One case per signal row in §3 and §4 of the spec, plus silence and anti-tests.

### Wave 1 — signals from `DescribeDBInstances` alone

```text
TEST: dbi_healthy_available
GIVEN: a DBInstance with DBInstanceStatus="available", BackupRetentionPeriod=7,
       PubliclyAccessible=false, StorageEncrypted=true, DeletionProtection=true
WHEN:  the list is fetched
THEN:
  - Resource.Status is blank ("" — silence is the UX; no "available", no "OK")
  - no EnrichmentFinding attached
  - no `!` / `~` glyph implied (row is green)
  - S1 issues count does NOT bump
```

```text
TEST: dbi_transitional_modifying
GIVEN: a DBInstance with DBInstanceStatus="modifying", all other fields healthy
WHEN:  the list is fetched
THEN:
  - Resource.Status == "modifying"
  - state bucket = Warning (row color yellow; asserted via Status non-blank + keyword in transitional set)
  - no Wave 2 finding, no glyph
```

```text
TEST: dbi_transitional_rebooting
GIVEN: a DBInstance with DBInstanceStatus="rebooting"
WHEN:  the list is fetched
THEN:
  - Resource.Status == "rebooting"
```

(And analogous mini-cases for each transitional keyword: `creating`, `backing-up`, `renaming`, `resetting-master-credentials`, `starting`, `stopping`, `upgrading`, `maintenance`, `configuring-enhanced-monitoring`, `configuring-iam-database-auth`, `configuring-log-exports`, `converting-to-vpc`, `moving-to-vpc`, `storage-optimization`. QA may parameterize these into a single table-driven test.)

```text
TEST: dbi_broken_storage_full
GIVEN: a DBInstance with DBInstanceStatus="storage-full"
WHEN:  the list is fetched
THEN:
  - Resource.Status == "storage-full"
  - state bucket = Broken (keyword in broken set)
```

```text
TEST: dbi_broken_failed
GIVEN: DBInstanceStatus="failed"
WHEN:  fetched
THEN:
  - Resource.Status == "failed"
```

```text
TEST: dbi_broken_incompatible_network
GIVEN: DBInstanceStatus="incompatible-network"
THEN: Resource.Status == "incompatible-network"
```

```text
TEST: dbi_broken_inaccessible_encryption_credentials
GIVEN: DBInstanceStatus="inaccessible-encryption-credentials"
WHEN:  fetched
THEN:
  - Resource.Status == "encryption key unavailable"
    (per §4 table — this is the one Wave 1 status the spec rewrites into prose;
     all other statuses pass through as keywords)
```

(Or alternatively, emit the raw keyword in Status and have the view layer rewrite. **Architect decision**: rewrite in the fetcher. Rationale: (a) §4 says "List text (S4)" column = `encryption key unavailable`; (b) keeping rewrites in one place (fetcher) is cheaper than spreading them across view surfaces; (c) the enrichment flow already normalizes strings elsewhere in the codebase. QA assertion: exact string match on `Resource.Status`.)

```text
TEST: dbi_broken_restore_error
GIVEN: DBInstanceStatus="restore-error"
THEN: Resource.Status == "restore-error"
```

```text
TEST: dbi_broken_incompatible_option_group
GIVEN: DBInstanceStatus="incompatible-option-group"
THEN: Resource.Status == "incompatible-option-group"
```

```text
TEST: dbi_broken_incompatible_parameters
GIVEN: DBInstanceStatus="incompatible-parameters"
THEN: Resource.Status == "incompatible-parameters"
```

```text
TEST: dbi_broken_incompatible_restore
GIVEN: DBInstanceStatus="incompatible-restore"
THEN: Resource.Status == "incompatible-restore"
```

```text
TEST: dbi_warning_no_backups
GIVEN: DBInstanceStatus="available", BackupRetentionPeriod=0
WHEN:  fetched
THEN:
  - Resource.Fields["backup_retention_period"] == "0"
  - Resource.Fields["cis_flags"] contains "NOBKP"
  - Status rendered as "no automated backups" when the row's primary signal
    is retention=0 (i.e. no other Wave 1 cause takes precedence)
```

Architect note: multiple Wave 1 signals can co-fire on one instance. Spec does not
explicitly rank them. **Architect-autoresolved (2026-04-21)**: precedence order for
Status column when multiple Wave 1 signals fire, most-severe first:

1. Broken keywords (exact status match) — `encryption key unavailable` for
   `inaccessible-encryption-credentials`, otherwise the raw keyword.
2. Transitional keywords — the raw keyword.
3. `no automated backups` (BackupRetentionPeriod==0).
4. `publicly accessible` (PubliclyAccessible==true).
5. `unencrypted storage` (StorageEncrypted==false).
6. `deletion protection off` (DeletionProtection==false).

Rationale: Broken > Transitional > configuration warnings (CIS policy misses).
Within warnings, the fixed order above is arbitrary but stable for tests.
All triggered warnings still get recorded in `cis_flags` for the detail view.

```text
TEST: dbi_warning_publicly_accessible
GIVEN: DBInstanceStatus="available", PubliclyAccessible=true, everything else healthy
THEN:
  - Resource.Status == "publicly accessible"
  - Resource.Fields["cis_flags"] contains "PUB"
```

```text
TEST: dbi_warning_unencrypted_storage
GIVEN: DBInstanceStatus="available", StorageEncrypted=false, everything else healthy
THEN:
  - Resource.Status == "unencrypted storage"
  - Resource.Fields["cis_flags"] contains "UNENC"
```

```text
TEST: dbi_warning_deletion_protection_off
GIVEN: DBInstanceStatus="available", DeletionProtection=false, everything else healthy
THEN:
  - Resource.Status == "deletion protection off"
  - Resource.Fields["cis_flags"] contains "NOPROT"
```

```text
TEST: dbi_multiple_warnings_precedence
GIVEN: DBInstanceStatus="available", BackupRetentionPeriod=0,
       PubliclyAccessible=true, StorageEncrypted=false, DeletionProtection=false
THEN:
  - Resource.Status == "no automated backups" (highest-precedence warning)
  - Resource.Fields["cis_flags"] contains all of "PUB", "UNENC", "NOBKP", "NOPROT"
```

```text
TEST: dbi_broken_beats_warning
GIVEN: DBInstanceStatus="storage-full", BackupRetentionPeriod=0, PubliclyAccessible=true
THEN:
  - Resource.Status == "storage-full" (broken > any warning)
```

### Wave 2 — DescribePendingMaintenanceActions enricher

```text
TEST: dbi_enricher_pending_maintenance_on_healthy_row
GIVEN: a Resource{ID: "prod-db-1", Status: ""} (healthy)
   AND DescribePendingMaintenanceActions returns one action:
       ResourceIdentifier = "arn:aws:rds:us-east-1:123456789012:db:prod-db-1"
       PendingMaintenanceActionDetails = [
         { Action: "system-update",
           Description: "Minor engine version upgrade",
           AutoAppliedAfterDate: 2026-05-01T00:00:00Z }
       ]
WHEN:  EnrichRDSDocDBMaintenance runs
THEN:
  - findings["prod-db-1"].Severity == "~"
  - findings["prod-db-1"].Summary == "pending maintenance: system-update"
  - findings["prod-db-1"].Rows has row Label="Action",         Value="system-update", Tier="~"
  - findings["prod-db-1"].Rows has row Label="Earliest Target", Value=formatted date, Tier="~"
  - findings["prod-db-1"].Rows has row Label="Description",    Value="Minor engine version upgrade"
  - IssueEnricherResult.IssueCount == 0 (tilde findings do NOT bump S1)
  - Truncated == false, TruncatedIDs empty
```

```text
TEST: dbi_enricher_no_pending_maintenance
GIVEN: a Resource{ID: "prod-db-1"}
   AND DescribePendingMaintenanceActions returns empty
THEN:
  - IssueEnricherResult.Findings is empty
  - IssueCount == 0
```

```text
TEST: dbi_enricher_maintenance_for_unrelated_resource_is_ignored
GIVEN: a Resource{ID: "prod-db-1"}
   AND DescribePendingMaintenanceActions returns one action for
       ResourceIdentifier = "arn:aws:rds:us-east-1:123456789012:db:staging-db"
THEN:
  - IssueEnricherResult.Findings is empty (no match for prod-db-1)
```

```text
TEST: dbi_enricher_pagination_walks_marker
GIVEN: page 1 returns Marker="m1" and one action for prod-db-1
   AND page 2 returns no Marker and one action for prod-db-2
   AND both resources are in the input slice
THEN:
  - findings["prod-db-1"] exists with severity "~"
  - findings["prod-db-2"] exists with severity "~"
```

```text
TEST: dbi_enricher_longest_suffix_match_wins
GIVEN: input resources include both "foo-db" and "bar-foo-db"
   AND DescribePendingMaintenanceActions returns one action for
       ResourceIdentifier = "arn:...:db:bar-foo-db"
THEN:
  - findings["bar-foo-db"] exists (longer suffix wins — deterministic)
  - findings["foo-db"] does NOT exist
```

### Related-panel discovery

One test per §2 target. All use typed `RawStruct` on `resource.Resource` and
the RelatedCheck result struct.

```text
TEST: dbi_related_sg
GIVEN: DBInstance.VpcSecurityGroups = [{VpcSecurityGroupId: "sg-aaa"}, {VpcSecurityGroupId: "sg-bbb"}]
WHEN:  checkDbiSG
THEN: result.TargetType == "sg", result.Count == 2, result.IDs contains both
```

```text
TEST: dbi_related_sg_empty_is_zero_not_error
GIVEN: DBInstance.VpcSecurityGroups = []
THEN: Count == 0, no error
```

```text
TEST: dbi_related_kms_present
GIVEN: DBInstance.KmsKeyId = "arn:aws:kms:us-east-1:123456789012:key/abc-123"
THEN: Count == 1, IDs == ["abc-123"]
```

```text
TEST: dbi_related_kms_absent_when_not_encrypted
GIVEN: DBInstance.KmsKeyId = nil, StorageEncrypted=false
THEN: Count == 0
```

```text
TEST: dbi_related_subnet_multi
GIVEN: DBInstance.DBSubnetGroup.Subnets = [{SubnetIdentifier: "subnet-a"}, {SubnetIdentifier: "subnet-b"}]
THEN: Count == 2, IDs contains both
```

```text
TEST: dbi_related_vpc
GIVEN: DBInstance.DBSubnetGroup.VpcId = "vpc-xyz"
THEN: Count == 1, IDs == ["vpc-xyz"]
```

```text
TEST: dbi_related_vpc_empty_when_no_subnet_group
GIVEN: DBInstance.DBSubnetGroup = nil
THEN: Count == 0
```

```text
TEST: dbi_related_role_monitoring_and_associated
GIVEN: DBInstance.MonitoringRoleArn = "arn:aws:iam::123:role/rds-monitoring"
   AND DBInstance.AssociatedRoles = [{RoleArn: "arn:aws:iam::123:role/s3-import"}]
THEN: Count == 2, IDs contains "rds-monitoring" and "s3-import"
```

```text
TEST: dbi_related_dbc_aurora
GIVEN: DBInstance.DBClusterIdentifier = "my-cluster"
   AND dbc cache has a resource with ID "my-cluster"
THEN: Count == 1, IDs == ["my-cluster"]
```

```text
TEST: dbi_related_dbc_empty_for_standalone_rds
GIVEN: DBInstance.DBClusterIdentifier = nil
THEN: Count == 0
```

```text
TEST: dbi_related_alarm_by_dimension
GIVEN: alarm cache contains a MetricAlarm with Dimensions = [{Name: "DBInstanceIdentifier", Value: "prod-db-1"}]
   AND Resource.ID = "prod-db-1"
THEN: Count >= 1, IDs contains that alarm's ID
```

```text
TEST: dbi_related_rds_snap_by_owner
GIVEN: rds-snap cache contains a DBSnapshot with DBInstanceIdentifier = "prod-db-1"
   AND Resource.ID = "prod-db-1"
THEN: Count >= 1, IDs contains that snapshot's ID
```

```text
TEST: dbi_related_logs_by_naming_prefix
GIVEN: logs cache contains a log group with ID "/aws/rds/instance/prod-db-1/error"
   AND Resource.ID = "prod-db-1"
THEN: Count >= 1, IDs contains that log-group ID
```

```text
TEST: dbi_related_secrets_by_arn
GIVEN: DBInstance.MasterUserSecret.SecretArn = "arn:aws:secretsmanager:...:secret:rds!prod-db-1-xyz"
   AND secrets cache contains a resource whose Fields["arn"] matches
THEN: Count == 1
```

```text
TEST: dbi_related_secrets_absent_for_classic_auth
GIVEN: DBInstance.MasterUserSecret = nil
THEN: Count == 0
```

```text
TEST: dbi_related_eni_via_describe_network_interfaces
GIVEN: DBInstance.VpcSecurityGroups = [{VpcSecurityGroupId: "sg-aaa"}]
   AND ec2 DescribeNetworkInterfaces with filters
       {description=RDSNetworkInterface, group-id=sg-aaa}
       returns NetworkInterfaces = [{NetworkInterfaceId: "eni-111"}]
THEN: Count == 1, IDs == ["eni-111"]
```

```text
TEST: dbi_related_eni_empty_when_no_sg
GIVEN: DBInstance.VpcSecurityGroups = []
THEN: Count == 0 (no API call made)
```

### Anti-tests — OUT OF SCOPE (§3.3)

```text
ANTI-TEST: dbi_no_cloudwatch_cpu_signal
GIVEN: a DBInstance (healthy) in a world where CloudWatch CPUUtilization is 95%
WHEN:  the list is fetched and enrichers run
THEN:
  - no CloudWatch metrics API is called by any dbi fetcher or enricher
  - no finding references "CPU" / "cpu" / "CPUUtilization"
```

```text
ANTI-TEST: dbi_no_freestorage_signal
THEN: no fetcher / enricher calls GetMetricStatistics for FreeStorageSpace.
```

```text
ANTI-TEST: dbi_no_replica_lag_signal
THEN: nothing references "ReplicaLag" or StatusInfos[].
```

```text
ANTI-TEST: dbi_no_connection_signal
THEN: nothing references "DatabaseConnections".
```

(QA may collapse the four anti-tests into one grep-style package-level test that asserts no dbi source file imports `cloudwatch.GetMetricStatistics`.)

### Silence test

Already covered by `TEST: dbi_healthy_available`. Repeated here for emphasis:

```text
TEST: dbi_silence_happy_path
GIVEN: a healthy DB instance (available, encrypted, backed up, private, deletion-protected)
THEN:
  - Resource.Status == ""
  - no EnrichmentFinding
  - no !/~ glyph
  - S1 count does not bump for this resource
```

## 2. Fixture List

Typed-fake fixtures for `internal/demo/fixtures/`. One baseline plus targeted mutations.

```text
FIXTURE: dbi-healthy-baseline
A DBInstance named "prod-db-1" running MySQL 8.0.35 on db.t3.medium in us-east-1.
  DBInstanceIdentifier = "prod-db-1"
  DBInstanceStatus     = "available"
  Engine               = "mysql"
  EngineVersion        = "8.0.35"
  DBInstanceClass      = "db.t3.medium"
  Endpoint             = { Address: "prod-db-1.abc.us-east-1.rds.amazonaws.com", Port: 3306 }
  MultiAZ              = false
  BackupRetentionPeriod = 7
  PubliclyAccessible   = false
  StorageEncrypted     = true
  DeletionProtection   = true
  KmsKeyId             = "arn:aws:kms:us-east-1:123456789012:key/aaaa-1111"
  VpcSecurityGroups    = [{VpcSecurityGroupId: "sg-prod", Status: "active"}]
  DBSubnetGroup        = { VpcId: "vpc-prod", Subnets: [{SubnetIdentifier: "subnet-a"}, {SubnetIdentifier: "subnet-b"}] }
  MasterUserSecret     = { SecretArn: "arn:aws:secretsmanager:us-east-1:123456789012:secret:rds!prod-db-1-xyz" }
  AssociatedRoles      = []
  MonitoringRoleArn    = ""
  DBClusterIdentifier  = nil
  DBInstanceArn        = "arn:aws:rds:us-east-1:123456789012:db:prod-db-1"
```

```text
FIXTURE: dbi-transitional-modifying
dbi-healthy-baseline with DBInstanceStatus = "modifying".
```

```text
FIXTURE: dbi-broken-storage-full
dbi-healthy-baseline with DBInstanceStatus = "storage-full".
```

```text
FIXTURE: dbi-broken-failed
dbi-healthy-baseline with DBInstanceStatus = "failed".
```

```text
FIXTURE: dbi-broken-inaccessible-encryption-credentials
dbi-healthy-baseline with DBInstanceStatus = "inaccessible-encryption-credentials".
```

```text
FIXTURE: dbi-broken-incompatible-network
dbi-healthy-baseline with DBInstanceStatus = "incompatible-network".
```

```text
FIXTURE: dbi-broken-restore-error
dbi-healthy-baseline with DBInstanceStatus = "restore-error".
```

```text
FIXTURE: dbi-warning-no-backups
dbi-healthy-baseline with BackupRetentionPeriod = 0.
```

```text
FIXTURE: dbi-warning-public
dbi-healthy-baseline with PubliclyAccessible = true.
```

```text
FIXTURE: dbi-warning-unencrypted
dbi-healthy-baseline with StorageEncrypted = false and KmsKeyId = nil.
```

```text
FIXTURE: dbi-warning-no-deletion-protection
dbi-healthy-baseline with DeletionProtection = false.
```

```text
FIXTURE: dbi-multi-warning
dbi-healthy-baseline with BackupRetentionPeriod=0, PubliclyAccessible=true,
StorageEncrypted=false, DeletionProtection=false. Used for precedence test.
```

```text
FIXTURE: dbi-aurora-member
dbi-healthy-baseline with DBClusterIdentifier = "my-cluster" and Engine = "aurora-mysql".
```

```text
FIXTURE: dbi-with-monitoring-role
dbi-healthy-baseline with MonitoringRoleArn = "arn:aws:iam::123456789012:role/rds-monitoring".
```

```text
FIXTURE: dbi-with-associated-roles
dbi-healthy-baseline with AssociatedRoles =
  [{RoleArn: "arn:aws:iam::123456789012:role/s3-import"}].
```

```text
FIXTURE: dbi-classic-auth
dbi-healthy-baseline with MasterUserSecret = nil. Used for secrets-absent test.
```

```text
FIXTURE: dbi-with-log-exports
dbi-healthy-baseline with EnabledCloudwatchLogsExports = ["error", "slowquery"].
```

### Pending-maintenance-actions fixtures (enricher)

```text
FIXTURE: pma-empty
DescribePendingMaintenanceActions returns an empty PendingMaintenanceActions slice and no Marker.
```

```text
FIXTURE: pma-one-action-for-prod-db-1
ResourceIdentifier = "arn:aws:rds:us-east-1:123456789012:db:prod-db-1"
PendingMaintenanceActionDetails = [
  { Action: "system-update",
    Description: "Minor engine version upgrade",
    AutoAppliedAfterDate: 2026-05-01T00:00:00Z,
    OptInStatus: "immediate" }
]
```

```text
FIXTURE: pma-two-pages
page1: Marker="m1", actions=[prod-db-1]
page2: no Marker, actions=[prod-db-2]
```

### Alarm / snapshot / logs / secrets cache fixtures

```text
FIXTURE: cached-alarm-for-prod-db-1
An alarm Resource{ID: "alm-high-cpu"} with RawStruct.Dimensions =
  [{Name: "DBInstanceIdentifier", Value: "prod-db-1"}].
```

```text
FIXTURE: cached-rds-snap-for-prod-db-1
A DBSnapshot Resource{ID: "rds:prod-db-1-2026-04-20-06-00"} with
  RawStruct.DBInstanceIdentifier = "prod-db-1".
```

```text
FIXTURE: cached-log-group-for-prod-db-1
A logs Resource{ID: "/aws/rds/instance/prod-db-1/error"}.
```

```text
FIXTURE: cached-secret-for-prod-db-1
A secrets Resource{ID: "rds!prod-db-1-xyz",
  Fields: {"arn": "arn:aws:secretsmanager:us-east-1:123456789012:secret:rds!prod-db-1-xyz"}}.
```

```text
FIXTURE: ec2-describe-network-interfaces-for-sg-prod
ec2.DescribeNetworkInterfacesOutput with NetworkInterfaces =
  [{NetworkInterfaceId: "eni-111", Description: "RDSNetworkInterface"}]
  when filters include {description=RDSNetworkInterface, group-id=sg-prod}.
```

## 3. Contract Surface Gap Analysis

### `rds_interfaces.go`

Spec demands:
- `DescribeDBInstances` (identity list). **Present** as `RDSDescribeDBInstancesAPI`.
- `DescribePendingMaintenanceActions` (Wave 2). **Present** as `RDSDescribePendingMaintenanceAPI`.
- `DescribeDBSnapshots` (rds-snap related). **Present** as `RDSDescribeDBSnapshotsAPI`.
- `DescribeDBSubnetGroups` (potentially for eni/subnet resolution). **Present** as `RDSDescribeDBSubnetGroupsAPI` (unused by current dbi related code; harmless).
- Aggregate `RDSAPI` composes all. **Present**.

**Delta**: none — interface file is complete for dbi's needs.

### `dbi_related.go`

Spec §2 demands: `alarm`, `dbc`, `eni`, `kms`, `logs`, `rds-snap`, `role`, `secrets`, `sg`, `subnet`, `vpc`, `ct-events`.

Registered in `rds.go` init: `sg`, `kms`, `subnet`, `alarm`, `rds-snap`, `logs`, `vpc`, `secrets`, `dbc`, `role`, `eni`. Eleven of twelve.

**Delta**: `ct-events` is not registered in `dbi_related.go` (expected — it's a universal pivot registered globally in `internal/aws/ct_events*.go`; not per-type). Spec §2 itself notes "Count shown: unknown" for ct-events and defers it. No action required.

All eleven per-type checkers exist and read the correct AWS fields per the spec. No code gap here.

### `dbi_issue_enrichment.go` + `rds_issue_enrichment.go`

Spec §3.2 demands: `DescribePendingMaintenanceActions` (account-wide, Wave 2, severity "~", IssueCount=0).

`dbi_issue_enrichment.go` registers `EnrichRDSDocDBMaintenance` at priority 10 (the shared enricher). `rds_issue_enrichment.go` implements it and matches the spec: severity "~", IssueCount=0, Marker pagination, suffix-match on ARN, longest-suffix-wins. **Present and correct.**

**Delta**: none on the contract. QA must write tests asserting §3.2 semantics; current test coverage is opaque to the architect by design.

### `dbi_detail_enrichment.go`

Spec has no detail-specific enricher requirements beyond what Wave 2 injects via `EnrichmentFinding.Rows`.

**Delta**: file absent. Do NOT create one — §2 and §4 of the spec do not demand additional detail fields beyond the list shape and the Wave 2 finding rows.

### `rds.go` fetcher gaps vs. §4

This is the principal delta:

| Signal (§4 row) | Current rds.go fetcher | Required by spec |
|---|---|---|
| Wave 1 Healthy (`available`) | `Status: "available"` (passes through) | `Status: ""` (silence) |
| Wave 1 Transitional (`modifying` etc.) | `Status: "modifying"` | `Status: "modifying"` — matches |
| Wave 1 Broken statuses | `Status: "<keyword>"` | same, EXCEPT `inaccessible-encryption-credentials` → `encryption key unavailable` |
| Wave 1 `BackupRetentionPeriod==0` | Fields only (`cis_flags` contains `NOBKP`) | **MUST also set Status = `no automated backups`** when no higher-precedence signal |
| Wave 1 `PubliclyAccessible==true` | Fields only | **MUST also set Status = `publicly accessible`** |
| Wave 1 `StorageEncrypted==false` | Fields only | **MUST also set Status = `unencrypted storage`** |
| Wave 1 `DeletionProtection==false` | Fields only | **MUST also set Status = `deletion protection off`** |

Coder must add a `deriveDbiStatus(db)` helper in `rds.go` that:

1. Returns `""` if `DBInstanceStatus == "available"` AND all four CIS-style policies pass (retention>0, not-public, encrypted, protected).
2. Returns `"encryption key unavailable"` if `DBInstanceStatus == "inaccessible-encryption-credentials"`.
3. Otherwise returns `DBInstanceStatus` if it's non-empty and not `"available"` (covers transitional + broken).
4. Otherwise (status is `available` but a warning policy fires) returns the highest-precedence warning text from the list above.

The field `Status` on `resource.Resource` is then set to that derived value. `cis_flags` behavior stays as-is.

## 4. Dispatch Notes

- QA task will create / overwrite:
  - `tests/unit/aws_dbi_test.go`
  - `tests/unit/aws_dbi_issue_enrichment_test.go`
  - `tests/unit/aws_dbi_related_test.go`
  - (no `aws_dbi_detail_enrichment_test.go` — no detail enricher)
  - `internal/demo/fixtures/dbi_fixtures.go` (extend existing or create)

- Coder task will:
  - Modify `internal/aws/rds.go` to add `deriveDbiStatus` and set `Resource.Status` accordingly.
  - Leave `internal/aws/rds_interfaces.go` untouched (nothing missing).
  - Leave `internal/aws/dbi_related.go` untouched (all eleven per-type checkers correct).
  - Leave `internal/aws/dbi_issue_enrichment.go` and `rds_issue_enrichment.go` untouched (logic matches spec).
  - No `dbi_detail_enrichment.go` created.

- No stubs expected to be removed in this pass — the existing code is under-populated, not over-mocked.
