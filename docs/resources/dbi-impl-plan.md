---
shortName: dbi
kind: impl-plan
pairedSpec: docs/resources/dbi.md
generatedAt: 2026-04-21
---

# dbi — Implementation Plan

Companion to `docs/resources/dbi.md`. The spec is the contract; this doc translates it into pseudocode test cases, a fixture list, and a contract-surface gap analysis so the QA and coder agents can be dispatched with zero ambiguity.

Terminology:

- **S1** = menu `issues:N` aggregate `!` count. `~` does not bump.
- **S2** = row color (Healthy=green, Warning=yellow, Broken=red, Dim=gray).
- **S3** = `!` / `~` glyph on Healthy row only. Forbidden on non-green.
- **S4** = Status column short text. Blank on Healthy.
- **S5** = Detail-view enrichment sentence.

"Row color = green with no glyph" = silence. Healthy rows render S4 blank and do not bump S1.

## 1. Pseudocode test spec

One case per signal in `docs/resources/dbi.md` §3 and §4, plus a silence case and Wave 3 anti-tests.

### 1.1 Wave 1 — status bucket

```text
TEST: dbi-healthy-available
GIVEN: A DBInstance with DBInstanceStatus = "available" and every config field set safely
       (BackupRetentionPeriod = 7, PubliclyAccessible = false, StorageEncrypted = true,
        DeletionProtection = true).
WHEN:  the list is fetched and rendered.
THEN:
  - S2 row color = green
  - S3 glyph = none
  - S4 text = "" (blank — not "available", not "OK")
  - S5 detail line = none
  - S1 issues count does NOT bump

TEST: dbi-transitional-bare
GIVEN: A DBInstance with DBInstanceStatus = "rebooting" and PendingModifiedValues empty.
WHEN:  the list is fetched and rendered.
THEN:
  - S2 row color = yellow
  - S3 glyph = none  (forbidden on non-green)
  - S4 text = "rebooting"
  - S5 sentence contains "Instance is rebooting — pending changes in progress."
  - S1 issues count does NOT bump

TEST: dbi-transitional-with-pending
GIVEN: A DBInstance with DBInstanceStatus = "modifying" and
       PendingModifiedValues.DBInstanceClass = "db.m6i.xlarge" (first non-empty field).
WHEN:  the list is fetched and rendered.
THEN:
  - S2 row color = yellow
  - S3 glyph = none
  - S4 text = "modifying: DBInstanceClass"
  - S5 sentence contains "Instance is modifying — pending changes in progress."
  - S1 issues count does NOT bump

TEST: dbi-failed
GIVEN: A DBInstance with DBInstanceStatus = "failed".
WHEN:  the list is fetched and rendered.
THEN:
  - S2 row color = red
  - S3 glyph = none
  - S4 text = "failed"
  - S5 sentence contains "Instance is in a failed state — contact AWS support or restore from snapshot."
  - S1 issues count does NOT bump

TEST: dbi-storage-full
GIVEN: A DBInstance with DBInstanceStatus = "storage-full".
WHEN:  the list is fetched and rendered.
THEN:
  - S2 row color = red
  - S4 text = "storage-full"
  - S5 sentence contains "Instance storage full — scale up or free space to recover."

TEST: dbi-incompatible-network
GIVEN: A DBInstance with DBInstanceStatus = "incompatible-network".
WHEN:  the list is fetched and rendered.
THEN:
  - S2 row color = red
  - S4 text = "incompatible-network"
  - S5 sentence contains "Network config incompatible — check DB subnet group AZ coverage."

TEST: dbi-incompatible-option-group
GIVEN: A DBInstance with DBInstanceStatus = "incompatible-option-group".
WHEN:  the list is fetched and rendered.
THEN:
  - S2 row color = red
  - S4 text = "incompatible-option-group"
  - S5 sentence contains "Option group incompatible — remove or update options."

TEST: dbi-incompatible-parameters
GIVEN: A DBInstance with DBInstanceStatus = "incompatible-parameters".
WHEN:  the list is fetched and rendered.
THEN:
  - S2 row color = red
  - S4 text = "incompatible-parameters"
  - S5 sentence contains "Parameter group incompatible — review custom parameters."

TEST: dbi-incompatible-restore
GIVEN: A DBInstance with DBInstanceStatus = "incompatible-restore".
WHEN:  the list is fetched and rendered.
THEN:
  - S2 row color = red
  - S4 text = "incompatible-restore"
  - S5 sentence contains "Restore failed — check snapshot compatibility and engine version."

TEST: dbi-restore-error
GIVEN: A DBInstance with DBInstanceStatus = "restore-error".
WHEN:  the list is fetched and rendered.
THEN:
  - S2 row color = red
  - S4 text = "restore-error"
  - S5 sentence contains "Restore error — review source snapshot and target engine version."

TEST: dbi-inaccessible-encryption-credentials
GIVEN: A DBInstance with DBInstanceStatus = "inaccessible-encryption-credentials".
WHEN:  the list is fetched and rendered.
THEN:
  - S2 row color = red
  - S4 text = "encryption key unavailable"
  - S5 sentence contains "KMS key for storage is unavailable — check key state and grants."
```

### 1.2 Wave 1 — configuration warnings (on "available")

Applied only when `DBInstanceStatus == "available"`. Precedence order:
`no automated backups` > `publicly accessible` > `unencrypted storage` > `deletion protection off`.

```text
TEST: dbi-no-backups
GIVEN: A DBInstance with DBInstanceStatus = "available" and BackupRetentionPeriod = 0.
       Other config fields safe.
WHEN:  the list is fetched and rendered.
THEN:
  - S2 row color = yellow
  - S4 text = "no automated backups"
  - S5 sentence contains "Automated backups disabled (BackupRetentionPeriod=0)."

TEST: dbi-publicly-accessible
GIVEN: available DBInstance with PubliclyAccessible = true, BackupRetentionPeriod > 0,
       StorageEncrypted = true, DeletionProtection = true.
WHEN:  the list is fetched and rendered.
THEN:
  - S2 row color = yellow
  - S4 text = "publicly accessible"
  - S5 sentence contains "Instance is reachable from the public internet (CIS RDS.2)."

TEST: dbi-unencrypted-storage
GIVEN: available DBInstance with StorageEncrypted = false, BackupRetentionPeriod > 0,
       PubliclyAccessible = false, DeletionProtection = true.
WHEN:  the list is fetched and rendered.
THEN:
  - S2 row color = yellow
  - S4 text = "unencrypted storage"
  - S5 sentence contains "Storage encryption at rest is disabled (CIS RDS.3)."

TEST: dbi-deletion-protection-off
GIVEN: available DBInstance with DeletionProtection = false, BackupRetentionPeriod > 0,
       PubliclyAccessible = false, StorageEncrypted = true.
WHEN:  the list is fetched and rendered.
THEN:
  - S2 row color = yellow
  - S4 text = "deletion protection off"
  - S5 sentence contains "Deletion protection is disabled — instance can be deleted in one API call."

TEST: dbi-warning-precedence
GIVEN: available DBInstance with BackupRetentionPeriod = 0 AND PubliclyAccessible = true
       AND StorageEncrypted = false AND DeletionProtection = false.
WHEN:  the list is fetched and rendered.
THEN:
  - S2 row color = yellow
  - S4 text = "no automated backups"  (highest-precedence warning wins)
  - S5 sentence contains "Automated backups disabled (BackupRetentionPeriod=0)."

TEST: dbi-broken-dominates-config-warnings
GIVEN: DBInstanceStatus = "storage-full" AND BackupRetentionPeriod = 0
       AND PubliclyAccessible = true.
WHEN:  the list is fetched and rendered.
THEN:
  - S2 row color = red (broken dominates)
  - S4 text = "storage-full" (not "no automated backups")
  - S5 sentence contains "Instance storage full — scale up or free space to recover."
```

### 1.3 Wave 2 — pending maintenance

```text
TEST: dbi-maintenance-overdue
GIVEN: An available DBInstance with no config warnings. The account-wide
       DescribePendingMaintenanceActions response includes a
       ResourcePendingMaintenanceActions entry whose ResourceIdentifier matches this
       instance's DBInstanceArn, with PendingMaintenanceActionDetails[] containing one
       action where ForcedApplyDate is in the past and Action = "system-update",
       Description = "New minor version available".
WHEN:  the list is enriched via Wave 2.
THEN:
  - S2 row color = green  (row was Healthy)
  - S3 glyph = "~"  (informational, does not bump S1)
  - S4 text = "maintenance scheduled"
  - S5 sentence contains "Pending maintenance action overdue: system-update (New minor version available)."
  - S1 issues count does NOT bump (~ is informational)

TEST: dbi-maintenance-future-not-overdue
GIVEN: An available DBInstance. DescribePendingMaintenanceActions returns an action for
       this instance but ForcedApplyDate and AutoAppliedAfterDate are both in the future.
WHEN:  the list is enriched via Wave 2.
THEN:
  - S2 row color = green
  - S3 glyph = none  (only overdue actions raise ~)
  - S4 text = ""
  - S5 sentence = none

TEST: dbi-maintenance-on-warning-row
GIVEN: A DBInstance with DBInstanceStatus = "available" but PubliclyAccessible = true
       AND a pending maintenance action overdue.
WHEN:  the list is enriched via Wave 2.
THEN:
  - S2 row color = yellow (warning dominates)
  - S3 glyph = none (glyphs forbidden on non-green rows)
  - S4 text = "publicly accessible" (warning retains S4)
  - S5 sentence still contains "Pending maintenance action overdue: ..." (Wave 2 finding is preserved on detail)
  - S1 issues count does NOT bump
```

### 1.4 Wave 3 anti-tests (out of scope)

Each fixture carries the condition; the list/enrichment must NOT surface anything and must NOT call CloudWatch.

```text
TEST: dbi-wave3-freestoragespace-not-surfaced
GIVEN: A fixture whose surrounding metadata indicates low FreeStorageSpace. The fake
       CloudWatch client records every call made against it.
WHEN:  the list is fetched and enriched through Wave 1 and Wave 2.
THEN:
  - No S2/S3/S4/S5 signal derives from FreeStorageSpace.
  - The fake CloudWatch client records zero GetMetricData / GetMetricStatistics calls.

TEST: dbi-wave3-cpu-not-surfaced
Same as above for CPUUtilization.

TEST: dbi-wave3-replicalag-not-surfaced
Same as above for ReplicaLag.

TEST: dbi-wave3-connections-not-surfaced
Same as above for DatabaseConnections.
```

### 1.5 Related-panel discovery (§2)

One case per target in `docs/resources/dbi.md` §2. For each, the fixture provides the already-cached sibling list (or a fake for server-side lookups) and asserts the discovered count.

```text
TEST: dbi-related-alarm
GIVEN: A DBInstance with DBInstanceIdentifier = "prod-db". The cached alarm list
       contains 3 MetricAlarms where Dimensions[] has {Name="DBInstanceIdentifier",
       Value="prod-db"} and 2 unrelated MetricAlarms.
THEN:
  - related("alarm").count == 3
  - no AWS API call made (purely cache scan)

TEST: dbi-related-dbc-aurora
GIVEN: DBInstance with DBClusterIdentifier = "cluster-xyz"; cached dbc list contains
       "cluster-xyz".
THEN: related("dbc").count == 1, no AWS API call.

TEST: dbi-related-dbc-rds-engine
GIVEN: DBInstance with DBClusterIdentifier = "" (classic RDS engine).
THEN: related("dbc") row is absent, no AWS API call.

TEST: dbi-related-eni
GIVEN: DBInstance with DBSubnetGroup.VpcId = "vpc-abc", DBInstanceIdentifier = "prod-db".
       DescribeNetworkInterfaces with Filters=[requester-id=amazon-rds, vpc-id=vpc-abc]
       returns 2 ENIs whose Description starts with "RDSNetworkInterface" referencing
       "prod-db", plus 1 ENI referencing a different instance.
THEN: related("eni").count == 2; exactly one DescribeNetworkInterfaces call with the
      expected filters; client-side description-prefix filter applied.

TEST: dbi-related-kms-encrypted
GIVEN: DBInstance with StorageEncrypted=true, KmsKeyId="arn:...:key/uuid"; cached kms
       list contains a KeyMetadata with matching Arn.
THEN: related("kms").count == 1, no AWS API call.

TEST: dbi-related-kms-unencrypted
GIVEN: DBInstance with StorageEncrypted=false (KmsKeyId absent).
THEN: related("kms") row is absent OR count == 0, no AWS API call.

TEST: dbi-related-logs
GIVEN: DBInstance with DBInstanceIdentifier="prod-db" and
       EnabledCloudwatchLogsExports=["error","slowquery"]. Cached logs list contains
       log groups "/aws/rds/instance/prod-db/error" and
       "/aws/rds/instance/prod-db/slowquery" and one unrelated group.
THEN: related("logs").count == 2, no AWS API call.

TEST: dbi-related-rds-snap
GIVEN: DBInstance with DBInstanceIdentifier="prod-db".
       DescribeDBSnapshots(DBInstanceIdentifier="prod-db") returns 4 DBSnapshot records.
THEN: related("rds-snap").count == 4; exactly one DescribeDBSnapshots call with the
      DBInstanceIdentifier filter set.

TEST: dbi-related-role
GIVEN: DBInstance with MonitoringRoleArn="arn:...:role/rds-monitoring" and
       AssociatedRoles=[{RoleArn="arn:...:role/s3-import"}]. Cached role list
       contains both role names (and others).
THEN: related("role").count == 2, no AWS API call.

TEST: dbi-related-secrets-managed
GIVEN: DBInstance with MasterUserSecret.SecretArn="arn:...:secret/db-master";
       cached secrets list contains that ARN.
THEN: related("secrets").count == 1, no AWS API call.

TEST: dbi-related-secrets-classic-auth
GIVEN: DBInstance with MasterUserSecret = nil.
THEN: related("secrets") row is absent OR count == 0, no AWS API call.

TEST: dbi-related-sg
GIVEN: DBInstance with VpcSecurityGroups=[{VpcSecurityGroupId="sg-1"},
       {VpcSecurityGroupId="sg-2"}]; cached sg list contains both.
THEN: related("sg").count == 2, no AWS API call.

TEST: dbi-related-subnet
GIVEN: DBInstance with DBSubnetGroup.Subnets=[{SubnetIdentifier="sub-a"},
       {SubnetIdentifier="sub-b"},{SubnetIdentifier="sub-c"}]; cached subnet list
       contains all three.
THEN: related("subnet").count == 3, no AWS API call.

TEST: dbi-related-vpc
GIVEN: DBInstance with DBSubnetGroup.VpcId="vpc-abc"; cached vpc list contains it.
THEN: related("vpc").count == 1, no AWS API call.

TEST: dbi-related-ct-events
GIVEN: DBInstance with DBInstanceIdentifier="prod-db".
THEN: related("ct-events") row is present with no count displayed (spec: windowed,
      count is misleading); discovery delegates to the universal ct-events pivot.
```

## 2. Fixture list

Typed fakes live at `internal/demo/fixtures/dbi_fixtures.go`. All DBInstance fields not named below use sensible defaults: `Engine="postgres"`, `EngineVersion="15.5"`, `DBInstanceClass="db.t3.medium"`, `AvailabilityZone="us-east-1a"`, `DBInstanceArn="arn:aws:rds:us-east-1:123456789012:db:<id>"`.

```text
FIXTURE: dbi-baseline-healthy
A healthy postgres DBInstance.
DBInstanceIdentifier = "prod-db"
DBInstanceStatus = "available"
BackupRetentionPeriod = 7
PubliclyAccessible = false
StorageEncrypted = true
KmsKeyId = "arn:aws:kms:us-east-1:123456789012:key/abcd-1234"
DeletionProtection = true
DBClusterIdentifier = ""                (classic RDS)
MasterUserSecret = nil                  (classic password auth)
VpcSecurityGroups = [{VpcSecurityGroupId="sg-1"}]
DBSubnetGroup.VpcId = "vpc-abc"
DBSubnetGroup.Subnets = [{SubnetIdentifier="sub-a"},{SubnetIdentifier="sub-b"}]
MonitoringRoleArn = ""
AssociatedRoles = []
EnabledCloudwatchLogsExports = []

FIXTURE: dbi-transitional-rebooting
Baseline mutated: DBInstanceStatus = "rebooting". PendingModifiedValues = {} (empty).

FIXTURE: dbi-transitional-modifying-class
Baseline mutated: DBInstanceStatus = "modifying".
PendingModifiedValues.DBInstanceClass = "db.m6i.xlarge".

FIXTURE: dbi-failed
Baseline mutated: DBInstanceStatus = "failed".

FIXTURE: dbi-storage-full
Baseline mutated: DBInstanceStatus = "storage-full".

FIXTURE: dbi-incompatible-network
Baseline mutated: DBInstanceStatus = "incompatible-network".

FIXTURE: dbi-incompatible-option-group
Baseline mutated: DBInstanceStatus = "incompatible-option-group".

FIXTURE: dbi-incompatible-parameters
Baseline mutated: DBInstanceStatus = "incompatible-parameters".

FIXTURE: dbi-incompatible-restore
Baseline mutated: DBInstanceStatus = "incompatible-restore".

FIXTURE: dbi-restore-error
Baseline mutated: DBInstanceStatus = "restore-error".

FIXTURE: dbi-inaccessible-kms
Baseline mutated: DBInstanceStatus = "inaccessible-encryption-credentials".

FIXTURE: dbi-no-backups
Baseline mutated: BackupRetentionPeriod = 0.

FIXTURE: dbi-publicly-accessible
Baseline mutated: PubliclyAccessible = true.

FIXTURE: dbi-unencrypted
Baseline mutated: StorageEncrypted = false, KmsKeyId = "".

FIXTURE: dbi-no-deletion-protection
Baseline mutated: DeletionProtection = false.

FIXTURE: dbi-all-warnings
Baseline mutated: BackupRetentionPeriod = 0, PubliclyAccessible = true,
StorageEncrypted = false, DeletionProtection = false.

FIXTURE: dbi-broken-with-warnings
Baseline mutated: DBInstanceStatus = "storage-full", BackupRetentionPeriod = 0,
PubliclyAccessible = true.

FIXTURE: dbi-maintenance-overdue
Baseline (available, all config safe). Paired fake for
DescribePendingMaintenanceActions returns one ResourcePendingMaintenanceActions with
ResourceIdentifier = DBInstanceArn of this fixture, PendingMaintenanceActionDetails =
[{Action="system-update", Description="New minor version available",
  ForcedApplyDate = 2026-04-15T00:00:00Z (past), AutoAppliedAfterDate = nil}].

FIXTURE: dbi-maintenance-future
Baseline (available, all config safe). Paired fake returns a maintenance action with
ForcedApplyDate = 2026-06-01T00:00:00Z (future), AutoAppliedAfterDate = 2026-06-08 (future).

FIXTURE: dbi-maintenance-on-warning
Baseline mutated: PubliclyAccessible = true. Paired fake returns overdue maintenance
action as in dbi-maintenance-overdue.

FIXTURE: dbi-aurora-member
Baseline mutated: Engine="aurora-postgresql", DBClusterIdentifier = "cluster-xyz".
Paired cached dbc list contains one DBCluster with DBClusterIdentifier="cluster-xyz".

FIXTURE: dbi-rds-managed-secret
Baseline mutated: MasterUserSecret.SecretArn =
"arn:aws:secretsmanager:us-east-1:123456789012:secret/db-master-AbCdE".
Paired cached secrets list contains that ARN.

FIXTURE: dbi-with-eni-fleet
Baseline (VpcId="vpc-abc", DBInstanceIdentifier="prod-db"). Paired fake
DescribeNetworkInterfaces response: 2 ENIs with Description="RDSNetworkInterface for
prod-db replica" and Description="RDSNetworkInterface for prod-db primary", plus 1
ENI with Description="RDSNetworkInterface for other-db" (must be filtered out client-side).

FIXTURE: dbi-with-logs-exports
Baseline mutated: EnabledCloudwatchLogsExports = ["error","slowquery"]. Paired cached
logs list contains "/aws/rds/instance/prod-db/error" and
"/aws/rds/instance/prod-db/slowquery" and one unrelated log group.

FIXTURE: dbi-with-snapshots
Baseline (DBInstanceIdentifier="prod-db"). Paired fake
DescribeDBSnapshots(DBInstanceIdentifier="prod-db") returns 4 DBSnapshot records.

FIXTURE: dbi-with-roles
Baseline mutated: MonitoringRoleArn = "arn:aws:iam::123456789012:role/rds-monitoring";
AssociatedRoles = [{RoleArn="arn:aws:iam::123456789012:role/s3-import",
FeatureName="s3Import"}]. Paired cached role list contains both names.

FIXTURE: dbi-with-multiple-sgs
Baseline mutated: VpcSecurityGroups = [{VpcSecurityGroupId="sg-1"},
{VpcSecurityGroupId="sg-2"}].

FIXTURE: dbi-with-three-subnets
Baseline mutated: DBSubnetGroup.Subnets = [{SubnetIdentifier="sub-a"},
{SubnetIdentifier="sub-b"},{SubnetIdentifier="sub-c"}].
```

## 3. Contract-surface gap analysis

Derived from phase 5 reading of `rds_interfaces.go`, `dbi_related.go`, `dbi_issue_enrichment.go`, and registration calls in `rds.go`. No `dbi_detail_enrichment.go` exists and none is required (§2 of the spec does not call for one).

### 3.1 Interfaces (`internal/aws/rds_interfaces.go`)

Spec needs: `DescribeDBInstances` (list), `DescribePendingMaintenanceActions` (Wave 2), `DescribeDBSnapshots` (rds-snap related), `DescribeNetworkInterfaces` (eni related — lives on EC2 client, not RDS).

Current file provides an aggregate `RDSAPI` that embeds `RDSDescribeDBInstancesAPI`, `RDSDescribeDBSnapshotsAPI`, `RDSDescribeEventsAPI`, `RDSDescribePendingMaintenanceAPI`, and `RDSDescribeDBSubnetGroupsAPI`. ENI lookup uses an EC2 client separately.

**Delta**: none. No new narrow interface is required for dbi.

### 3.2 Related-target registrations (`dbi_related.go` + `rds.go` init)

Spec §2 calls for 12 targets: `alarm`, `dbc`, `eni`, `kms`, `logs`, `rds-snap`, `role`, `secrets`, `sg`, `subnet`, `vpc`, `ct-events`.

Current `RegisterRelated("dbi", ...)` registers 11 checkers: `sg`, `kms`, `subnet`, `alarm`, `rds-snap`, `logs`, `vpc`, `secrets`, `dbc`, `role`, `eni`. The 12th (`ct-events`) is registered centrally in `ct_events.go` as the universal pivot and applies to every type.

**Delta A — ENI filter does not match spec**: spec §2 calls for `DescribeNetworkInterfaces(Filters=[{requester-id,amazon-rds},{vpc-id,<DBSubnetGroup.VpcId>}])` plus a client-side `Description` prefix match tied to `DBInstanceIdentifier`. Current `checkDbiENI` uses `Filters=[{description,RDSNetworkInterface},{group-id,<sgIDs>}]` without the vpc-id scope and without a per-instance description match. Effect: on accounts where multiple DB instances share a security group the current implementation counts ENIs from other DB instances as "related" to this one. Coder must switch to the spec-documented filter pair + client-side `DBInstanceIdentifier` check.

**Delta B — no other target-list gaps**: the other 10 checkers match the spec's discovery mechanism and count policy.

### 3.3 Wave 2 enricher (`dbi_issue_enrichment.go` + `EnrichRDSDocDBMaintenance` in `rds_issue_enrichment.go`)

Spec §3.2 signal: "A `PendingMaintenanceAction` for this instance has `ForcedApplyDate` or `AutoAppliedAfterDate` in the past, or is otherwise actionable — scheduled maintenance overdue." Spec §4 row: Summary = `"Pending maintenance action overdue: <ActionType> (<Description>)."`, Severity = `~`.

Current enricher `EnrichRDSDocDBMaintenance` registered with priority 10 for dbi, shared with rds/docdb. It:
- paginates `DescribePendingMaintenanceActions` up to `EnrichmentCap` pages.
- emits a Finding for every pending action regardless of whether its `ForcedApplyDate` / `AutoAppliedAfterDate` is in the past.
- builds Summary as `"pending maintenance"` or `"pending maintenance: <action1>, <action2>, ..."`.
- sets Severity = `~` and IssueCount = 0. ✓ matches spec.
- does the probeID longest-suffix match on ARN. ✓ correct.

**Delta C — not filtering to overdue actions**: spec demands the finding fire only when the action is overdue (`ForcedApplyDate` or `AutoAppliedAfterDate` in the past, or otherwise actionable). Current enricher fires for every pending action. Coder must add the overdue predicate before recording the finding. Tests `dbi-maintenance-overdue` and `dbi-maintenance-future-not-overdue` pin this behavior.

**Delta D — Summary wording**: spec-mandated Summary is `"Pending maintenance action overdue: <ActionType> (<Description>)."`. Current Summary text differs. Because this enricher is shared with rds (priority 100) and docdb, the clean move is to take a shape-parameter, or to split a dbi-specific wrapper. Recommend: split into a thin dbi-specific wrapper function that calls a shared helper for the pagination and overdue check, then formats the Summary per dbi spec. Other resource types keep their current wording until their own specs demand otherwise.

### 3.4 Detail-view enricher (`dbi_detail_enrichment.go`)

Not present, not required. §2 of the spec defines related panel only; §4 sends the Wave 2 maintenance finding to S5 which already routes through the `EnrichmentFinding.Summary`. No separate detail enricher registration needed.

### 3.5 Fetcher / Status derivation (NOT read — by rule)

The fetcher body in `rds.go` and the status-derivation helper are not read by this skill. The coder will rewrite them against the spec. Expected shape, pinned by the spec and the test cases above:

- Fetcher maps every `rdstypes.DBInstance` returned by `DescribeDBInstances` to a `resource.Resource` with `RawStruct = db` (unchanged).
- `Status` field carries the S4 string:
  - Healthy (`DBInstanceStatus == "available"` with no config warning) → `""` (blank).
  - Transitional (status in the set of 16 transitional keywords) → `<status>` or `<status>: <first non-empty PendingModifiedValues key>` when present.
  - Broken (status in `failed`, `storage-full`, `incompatible-*`, `restore-error`) → the status keyword verbatim.
  - Broken (`inaccessible-encryption-credentials`) → `"encryption key unavailable"`.
  - Configuration warning (on `available`) → one of `"no automated backups"`, `"publicly accessible"`, `"unencrypted storage"`, `"deletion protection off"` per precedence order.
- Any status not in the above buckets with non-empty text → pass through defensively (no hidden blank).

## 4. Delta summary for the coder

1. **Delta A (ENI filter)** — rewrite `checkDbiENI` to use `requester-id=amazon-rds` + `vpc-id=<DBSubnetGroup.VpcId>` filters and a client-side description prefix check against `DBInstanceIdentifier`.
2. **Delta C (overdue predicate)** — only emit a maintenance finding when `ForcedApplyDate` or `AutoAppliedAfterDate` is in the past. Unit tests in §1.3 pin this.
3. **Delta D (Summary wording)** — dbi Summary = `"Pending maintenance action overdue: <ActionType> (<Description>)."`. Split from the shared enricher as needed so the rds and docdb shapes are unaffected.
4. **Delta E (Status derivation)** — rewrite `deriveDbiStatus` (or equivalent) to implement the exact S4 contract above, including the transitional `PendingModifiedValues` first-key suffix.

Everything else (Related target list, interface file, detail-enricher absence, pagination semantics, severity policy) is already correct and should not be touched.

