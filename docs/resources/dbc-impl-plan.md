---
shortName: dbc
generatedFrom: docs/resources/dbc.md
status: drafted
---

# dbc — Implementation Plan

Contract: `docs/resources/dbc.md` (immutable for this run). No TBDs outstanding after phase 2.

## 0. Coverage Matrix (universal rules)

| ID | Invariant | Fixture | Test |
|----|-----------|---------|------|
| U1 | Healthy blank S4 | `dbc-prod-healthy` | `TestDBC_Fetch_Healthy_StatusBlank` |
| U2 | Warning/Broken §4 phrase | `warn-dbc-no-bkp`, `warn-dbc-unenc`, `warn-dbc-no-prot`, `warn-dbc-modifying`, `broken-dbc-failed`, `broken-dbc-enc-unreachable`, `broken-dbc-incompat-params`, `broken-dbc-no-writer` | `TestDBC_Fetch_PerSignal_S4Phrase` table |
| U3 | `!` glyph on Healthy+! finding | `healthy-dbc-maint-overdue` | scenario `ExpectRowNamePrefix("! ")` |
| U4 | `!` glyph present on healthy-with-finding | same as U3 | (dbc Wave 2 severity is `!` — covered by U3) |
| U5 | No glyph on non-green rows | any `warn-*`/`broken-*` | scenario `ExpectRowNoGlyphPrefix` |
| U6 | S1 badge counts `!`-severity instances | `healthy-dbc-maint-overdue` + `warn-dbc-no-bkp-plus-maint` | scenario `ExpectMenuIssueCount("dbc", 2)` |
| U7a | Multi-W1 `<top> (+N-1)` suffix | `warn-dbc-multi` (unenc + no-bkp + no-prot) | `TestDBC_Fetch_MultiW1_Suffix` |
| U7b | W1+W2 suffix bump | `warn-dbc-no-bkp-plus-maint` | `TestDBC_Enrich_MaintenanceOverdue_StacksWithW1` |
| U7c | S5 lists every Wave-2 finding | `warn-dbc-no-bkp-plus-maint` | scenario `ExpectViewContains(<w2 Action>)` |
| U7d | `!` beats `~` — N/A for dbc (no `~` Wave 2) | — | skipped |
| U7e | S5 every Wave-1 phrase | `warn-dbc-multi` | scenario `ExpectViewContains(<capitalised phrase>)` per Issues entry |
| U7f | `Resource.Issues` populated in §4 order | every `warn-*`/`broken-*` fixture | `TestDBC_Fetch_IssuesOrdered` |
| U8 | Broken > Warning | `broken-dbc-no-writer` (also unenc etc stacked) | `TestDBC_Fetch_BrokenBeatsWarning` |
| U9 | Related pivot counts (§2 `count shown: yes`) | `dbc-prod-healthy` | scenario `ExpectRelatedRowCountAtLeast` per pivot |
| U10 | No jargon columns (`CIS`, `Writer`, `Flags`) | all fixtures | scenario `ExpectViewNotContains("CIS", "Flags", "NOBKP", "UNENC", "NOPROT")` |
| U11 | Summary ≠ Rows content | `healthy-dbc-maint-overdue` | `TestDBC_Enrich_Summary_ExcludesRowValues` |

## 1. Pseudocode-Test Spec

### 1.1 Fetcher tests (`tests/unit/aws_dbc_test.go`)

```text
TEST: healthy_blank_status
GIVEN: a DBCluster with Status="available", DBClusterMembers has exactly one IsClusterWriter=true,
       DeletionProtection=true, StorageEncrypted=true, BackupRetentionPeriod=7
WHEN:  FetchDocDBClustersPage runs
THEN:  got.Status == ""
       got.Issues is nil/empty
       got.Fields["status"] == ""

TEST: transitional_modifying_renders_in_progress
GIVEN: Status="modifying" (all other fields healthy — single writer, encrypted, retention>0, protected)
THEN:  got.Status == "modifying: in progress"
       got.Issues == ["modifying: in progress"]

TEST: broken_status_failed
GIVEN: Status="failed"
THEN:  got.Status == "failed: cluster operation"
       got.Issues == ["failed: cluster operation"]

TEST: broken_status_inaccessible_encryption
GIVEN: Status="inaccessible-encryption-credentials"
THEN:  got.Status == "encryption key unreachable"
       got.Issues == ["encryption key unreachable"]

TEST: broken_status_incompatible_parameters
GIVEN: Status="incompatible-parameters"
THEN:  got.Status == "parameter group incompatible"
       got.Issues == ["parameter group incompatible"]

TEST: broken_no_writer
GIVEN: Status="available", DBClusterMembers has 2 entries both IsClusterWriter=false
THEN:  got.Status == "no writer: reads only"
       got.Issues == ["no writer: reads only"]

TEST: warning_deletion_protection_off
GIVEN: available, one writer, encrypted, retention>0, DeletionProtection=false
THEN:  got.Status == "delete-protection off"
       got.Issues == ["delete-protection off"]

TEST: warning_storage_not_encrypted
GIVEN: available, one writer, retention>0, protected, StorageEncrypted=false
THEN:  got.Status == "not encrypted at rest"
       got.Issues == ["not encrypted at rest"]

TEST: warning_no_automated_backups
GIVEN: available, one writer, encrypted, protected, BackupRetentionPeriod=0
THEN:  got.Status == "no automated backups"
       got.Issues == ["no automated backups"]

TEST: multi_w1_no_bkp_plus_unenc_plus_no_prot (U7a)
GIVEN: available, one writer, BackupRetentionPeriod=0, StorageEncrypted=false, DeletionProtection=false
WHEN:  FetchDocDBClustersPage runs
THEN:  got.Status == "no automated backups (+2)"  // "no automated backups" is §4-row-first
       got.Issues == ["no automated backups", "not encrypted at rest", "delete-protection off"]
       // Precedence order in §4 table (top to bottom of Wave-1 Warning rows):
       // delete-protection off | not encrypted at rest | no automated backups
       // The spec §4 table order is the tie-breaker. We pin it alphabetically-by-§4-table-position:
       //   "no automated backups" (row 8) wins the top slot ONLY IF precedence says so.
       //   Actually per skill rule: "within same severity, the spec §4 table pins a tie-breaker".
       //   For dbc, the §4 row order is: delete-protection, not-encrypted, no-automated-backups.
       //   The FIRST row in table order is "delete-protection off" — that is the top phrase.
       // Correct expectation:
       got.Status == "delete-protection off (+2)"
       got.Issues == ["delete-protection off", "not encrypted at rest", "no automated backups"]

TEST: broken_beats_warning
GIVEN: Status="available", 0 writer members, BackupRetentionPeriod=0, DeletionProtection=false
THEN:  got.Status == "no writer: reads only"   // Broken wins severity; no (+N) — only one Broken
       got.Issues == ["no writer: reads only"]
       // Warnings are NOT enumerated when a Broken is present — Broken terminates.

TEST: unknown_status_passthrough
GIVEN: Status="some-new-state-aws-invented"
THEN:  got.Status == "some-new-state-aws-invented"   // bare keyword acceptable on unknown statuses (future-proofing)
       got.Issues == ["some-new-state-aws-invented"]

TEST: raw_struct_preserved
GIVEN: any cluster
THEN:  got.RawStruct is the docdb/types.DBCluster for row reflection lookup

TEST: fields_no_cis_flags
GIVEN: any unencrypted + no-backup + no-protection cluster
THEN:  "cis_flags" is NOT a key in got.Fields   // U10 — column is deleted; field must go too
       // has_writer, backup_retention_period, etc. remain (color function still uses them)
```

### 1.2 Wave 2 enricher tests (`tests/unit/aws_dbc_issue_enrichment_test.go`)

```text
TEST: healthy_cluster_overdue_maintenance_bumps_S1
GIVEN: resources=[healthy-dbc with Status==""], DocDB fake returns one
       ResourcePendingMaintenanceActions entry whose ResourceIdentifier is the
       CLUSTER ARN (arn:aws:rds:...:cluster:<id>) with AutoAppliedAfterDate in the past
WHEN:  EnrichDBCMaintenance runs
THEN:  result.IssueCount == 1   // severity "!" bumps S1
       result.Findings[id].Severity == "!"
       result.Findings[id].Summary == "maintenance overdue"
       result.Findings[id].Rows includes Action/Description/Earliest Target
       result.FieldUpdates[id]["status"] == "maintenance overdue"   // Healthy + W2 sole phrase

TEST: summary_excludes_rows_content (U11)
GIVEN: same maintenance fixture
THEN:  !strings.Contains(finding.Summary, row.Value) for every row.Value

TEST: w1_plus_w2_stacks_suffix (U7b)
GIVEN: resources=[dbc with Status=="no automated backups"], maintenance action for same id
THEN:  result.FieldUpdates[id]["status"] == "no automated backups (+1)"
       result.Findings[id].Summary == "maintenance overdue"
       result.IssueCount == 1

TEST: multi_w1_plus_w2_bumps_existing_suffix
GIVEN: resources=[dbc with Status=="delete-protection off (+2)"], maintenance for same id
THEN:  result.FieldUpdates[id]["status"] == "delete-protection off (+3)"

TEST: not_overdue_produces_no_finding
GIVEN: ResourcePendingMaintenanceActions entry with AutoAppliedAfterDate in the FUTURE
       and ForcedApplyDate in the FUTURE (or nil)
THEN:  result.Findings is empty
       result.IssueCount == 0
       result.FieldUpdates is empty

TEST: instance_arn_filtered_out
GIVEN: ResourcePendingMaintenanceActions entry with instance ARN (arn:...:db:<id>)
THEN:  dbc enricher ignores it (no finding emitted; dbi handles instance ARNs)

TEST: no_matching_cluster_id_ignored
GIVEN: cluster ARN in maintenance list that does not match any resources entry
THEN:  no finding emitted for missing id; no panic

TEST: nil_docdb_client_returns_empty
GIVEN: clients.DocDB == nil
THEN:  result.Findings empty, no error
```

### 1.3 Related tests (`tests/unit/aws_dbc_related_test.go`)

One discovery test per §2 target. Each asserts Count > 0 when the fixture graph contains a target, and `Count == 0` when it doesn't.

```text
TEST: checkDbcSG_reads_VpcSecurityGroups
GIVEN: DBCluster with two VpcSecurityGroups entries
THEN:  Count == 2, IDs match the entries

TEST: checkDbcAlarm_matches_DBClusterIdentifier_dimension
GIVEN: alarm cache containing one MetricAlarm with Dimensions=[{Name:"DBClusterIdentifier",Value:<id>}]
       and one unrelated alarm
THEN:  Count == 1

TEST: checkDbcLogs_matches_naming_prefix
GIVEN: logs cache containing /aws/docdb/<id>/audit and /aws/docdb/<id>/profiler
       plus one unrelated log group
THEN:  Count == 2

TEST: checkDbcKMS_extracts_key_id
GIVEN: cluster KmsKeyId = "arn:...:key/<uuid>"
THEN:  Count == 1, the returned id is <uuid> (last path segment)

TEST: checkDbcSecrets_matches_MasterUserSecret_SecretArn
GIVEN: cluster with MasterUserSecret.SecretArn="<arn>"; secrets cache contains that entry
THEN:  Count == 1

TEST: checkDbcDBI_reverse_lookup_by_DBClusterIdentifier
GIVEN: dbi cache with one DBInstance whose DBClusterIdentifier == cluster id
       and one unrelated instance
THEN:  Count == 1

TEST: checkDbcDocdbSnap_reverse_lookup_by_DBClusterIdentifier
GIVEN: docdb-snap cache with two snapshots whose DBClusterIdentifier == cluster id
THEN:  Count == 2

TEST: checkDbcSubnet_resolves_via_DescribeDBSubnetGroups
GIVEN: cluster.DBSubnetGroup = "grp-1", fake DocDB returns a DBSubnetGroup with 3 Subnets
THEN:  Count == 3

TEST: checkDbcVPC_resolves_via_DescribeDBSubnetGroups
GIVEN: same as above, subnet group has VpcId="vpc-1"
THEN:  Count == 1, id == "vpc-1"

TEST: no_master_user_secret_returns_zero
GIVEN: cluster.MasterUserSecret == nil
THEN:  Count == 0 (not -1)
```

### 1.4 Silence / anti-tests

```text
TEST: healthy_generates_no_finding
THEN: no row in §4, no Resource.Issues entry, color=Healthy green.

TEST: Wave_3_replica_lag_not_surfaced
GIVEN: a cluster with a CloudWatch DBInstanceReplicaLag metric (synthetic) — the fetcher
WHEN:  the fetcher runs
THEN:  got.Issues has no lag-related entry, Fields has no lag keys
      (ensures Wave 3 OUT OF SCOPE stays out)
```

## 2. Fixture list (`internal/demo/fixtures/dbc.go`)

Single-source file, graph-connected. Replaces `docdb.go` (which currently holds both cluster and cluster-snapshot fixtures — snapshots are left where they are for now and continue to be served by `DocDBFake`; the renaming / splitting of `docdb-snap` into its own file is out of scope for this run).

**Required exports:**
- `type DBCFixtures struct { DBClusters []docdbtypes.DBCluster; DBClusterSnapshots []docdbtypes.DBClusterSnapshot; DBSubnetGroups []docdbtypes.DBSubnetGroup; PendingMaintenanceActions []docdbtypes.ResourcePendingMaintenanceActions }`
- `func NewDBCFixtures() *DBCFixtures`
- Legacy `DocDBFixtures` / `NewDocDBFixtures()` kept as type alias + wrapper calling `NewDBCFixtures` so existing `fakes/docdb.go` compiles without edits inside this skill run — OR rename the fake to use the new type (simpler — prefer this).
- ID/ARN consts exported for sibling cross-reference:
  - `ProdDbcID = "acme-docdb-prod"`; `ProdDbcARN = "arn:aws:rds:us-east-1:123456789012:cluster:acme-docdb-prod"`
  - `MaintDbcOverdueID` / `MaintDbcOverdueARN`, `WarnDbcNoBkpMaintID` / `WarnDbcNoBkpMaintARN` etc. for the maintenance-bearing clusters.

### Fixtures

- **`dbc-prod-healthy`** (`acme-docdb-prod`) — Healthy baseline. Status=`available`, one writer + two readers, encrypted, retention=7, deletion-protected. All graph pivots present: KMS key, SGs, subnet group, logs, alarms, ct-events, dbi member, secrets. **Required: every §2 `count shown: yes` pivot returns ≥1 row for this fixture.**
- **`warn-dbc-modifying`** (alias the existing `staging-docdb` but rebaseline) — Status=`modifying`. All other fields healthy; writer present. Expected Status="modifying: in progress".
- **`broken-dbc-failed`** (alias existing `dbc-failed`) — Status=`failed`. Expected Status="failed: cluster operation".
- **`broken-dbc-enc-unreachable`** — Status=`inaccessible-encryption-credentials`. Writer present, all else healthy. Expected="encryption key unreachable".
- **`broken-dbc-incompat-params`** — Status=`incompatible-parameters`. Expected="parameter group incompatible".
- **`broken-dbc-no-writer`** (alias existing `dbc-no-writer`) — Status=`available`, two readers zero writer. Expected="no writer: reads only". (Single Broken phrase — no Warning stacking because Broken terminates.)
- **`warn-dbc-no-prot`** — available + writer + encrypted + retention=7 + DeletionProtection=false. Expected="delete-protection off".
- **`warn-dbc-unenc`** — available + writer + retention=7 + DeletionProtection=true + StorageEncrypted=false. Expected="not encrypted at rest".
- **`warn-dbc-no-bkp`** — available + writer + encrypted + DeletionProtection=true + BackupRetentionPeriod=0. Expected="no automated backups".
- **`warn-dbc-multi`** (U7a) — available + writer + StorageEncrypted=false + DeletionProtection=false + BackupRetentionPeriod=0. Expected Status="delete-protection off (+2)". Expected Issues=["delete-protection off", "not encrypted at rest", "no automated backups"].
- **`healthy-dbc-maint-overdue`** (U3/U6) — available + writer + encrypted + retention=7 + protected. Paired with a `ResourcePendingMaintenanceActions` entry whose ResourceIdentifier is the cluster ARN, AutoAppliedAfterDate in the past. Expected: Healthy color, glyph=`!`, Status="maintenance overdue", S1 bump.
- **`warn-dbc-no-bkp-plus-maint`** (U7b/U7c) — Wave 1 "no automated backups" + Wave 2 maintenance overdue. Expected post-enrichment Status="no automated backups (+1)".

### Sibling fixture updates (graph connectedness for `dbc-prod-healthy`)

- `alarm` fixtures (`internal/demo/fixtures/alarm.go`) — add one alarm with Dimensions=[{Name:"DBClusterIdentifier", Value:"acme-docdb-prod"}, Namespace="AWS/DocDB"].
- `logs` fixtures (`internal/demo/fixtures/logs.go`) — add two log groups `/aws/docdb/acme-docdb-prod/audit` and `/aws/docdb/acme-docdb-prod/profiler`.
- `dbi` fixtures — one DBInstance with `DBClusterIdentifier="acme-docdb-prod"` already provisioned? (The existing `prod-dbi-aurora-1` references `prod-aurora-cluster`, not `acme-docdb-prod`. If needed, add a matching DocumentDB member DBInstance.) Skipped for this run — DocumentDB cluster members are not modelled as RDS DBInstances; the pivot returns 0 for DocumentDB clusters, which is correct.
- `docdb-snap` fixtures — the existing `rds:acme-docdb-prod-2026-03-20` already references `acme-docdb-prod`. Count ≥ 1. OK.
- `kms` fixtures (`internal/demo/fixtures/kms.go`) — verify key id `a1b2c3d4-5678-90ab-cdef-111111111111` exists. Add if missing.
- `secrets` fixtures — add one SecretListEntry with ARN referenced from `acme-docdb-prod.MasterUserSecret.SecretArn`. Update `acme-docdb-prod` fixture to set `MasterUserSecret.SecretArn` to that ARN.
- `sg` fixtures — verify `sg-0ccc333333333333c` exists. Add if missing.
- `subnet` fixtures — for `DescribeDBSubnetGroups("acme-docdb-subnet-group")` the DocDB fake must return a `DBSubnetGroup` with Subnets matching two `subnet-*` IDs that exist in the subnet cache, and `VpcId` matching an existing vpc id. Add subnet group entries to `DBCFixtures.DBSubnetGroups`. The DocDB fake's `DescribeDBSubnetGroups` switches from returning empty to returning the requested group by name.
- `ct-events` fixtures — universal pivot; no action needed beyond existing `ct-events.go` entries (if any).

### Adversarial (inline-in-tests — NOT in fixture file)

- Nil `DBClusterIdentifier` on one DBCluster.
- Empty `PendingMaintenanceActions` response.
- `DescribeDBClusters` returning error.
- Nil `clients.DocDB`.

These stay inline in `tests/unit/aws_dbc_*_test.go` per skill's anti-fixture-corruption rule.

## 3. Contract-surface gap analysis

### `dbc_interfaces.go`

- Current: `DocDBDescribeDBClustersAPI`, `DocDBDescribeDBClusterSnapshotsAPI`, `DocDBDescribeDBSubnetGroupsAPI`, `DocDBAPI`.
- Gap: missing `DocDBDescribePendingMaintenanceActionsAPI` — needed by new `EnrichDBCMaintenance`.
- Delta: add the narrow API and extend `DocDBAPI` to embed it.

### `dbc_related.go`

- Current: 9 related checkers (`sg, alarm, logs, kms, secrets, dbi, docdb-snap, subnet, vpc`).
- Spec §2 expects 10 targets including `ct-events`.
- Gap: no explicit `ct-events` checker. But `ct-events` is registered as a universal pivot — confirm in `relations_ct_events.go` or equivalent. If absent, no per-dbc action needed (skill: universal pivots don't need per-type checkers). **Verify during phase 7, no change expected.**
- Delta: none assuming universal ct-events registration exists.

### `dbc_issue_enrichment.go`

- Current: `registerIssueEnricher("dbc", EnrichRDSDocDBMaintenance, 100)`.
- Spec §3.2 expects Severity=`!`, Summary=`"maintenance overdue"`, overdue check (ForcedApplyDate OR AutoAppliedAfterDate in the past), filter to cluster ARNs.
- Gap: `EnrichRDSDocDBMaintenance` filters to `isInstanceARN` only (dbc clusters never match), uses severity `~`, and Summary concatenates Actions (anti-pattern — violates EnrichmentFinding contract).
- Delta: rewrite completely. Introduce new `EnrichDBCMaintenance` that calls `clients.DocDB.DescribePendingMaintenanceActions`, filters `isClusterARN`, checks overdue, emits `!` severity and `"maintenance overdue"` Summary, bumps S4 (+N) suffix via `resource.BumpFindingSuffix`. Register it here. `EnrichRDSDocDBMaintenance` in `rds_issue_enrichment.go` may remain (still registered for `rds`) but is out of this run's scope.

### `dbc_detail_enrichment.go`

- Current: file does not exist.
- Spec §2/§3: no Wave 2 detail-only signals needed.
- Delta: do NOT create this file.

### `internal/config/defaults_databases.go` → "dbc"

- Current columns: `Cluster ID | Version | Status | CIS | Instances | Writer | Endpoint`.
- Jargon columns: `CIS` (width 18), `Writer` (derived from writer_count, Path="EngineVersion" — visually misleading).
- Spec rule: exactly one Status column, no jargon. Identity columns allowed.
- Delta: delete `CIS` and `Writer` columns. Final columns: `Cluster ID | Version | Status | Instances | Endpoint`. Regenerate `.a9s/views/dbc.yaml` via `go run ./cmd/viewsgen/`.

### `internal/resource/types_databases.go` → dbc entry

- `Columns` slice mirrors the list view. Currently has `cluster_id | engine_version | status | instances | endpoint` — already matches. **No change needed.**
- `Color` func: reads `Fields["status"]` as raw AWS keyword. After fetcher rewrite, `Fields["status"]` holds the **§4 phrase** (e.g. `"no automated backups"`, `"no writer: reads only"`, `"maintenance overdue"`). Current Color func will misclassify.
- Delta: rewrite Color func to:
  1. Strip `(+N)` suffix via `resource.StripFindingSuffix`.
  2. Phrase-match Broken prefixes (`"failed: ..."`, `"encryption key unreachable"`, `"parameter group incompatible"`, `"no writer: reads only"`) → `ColorBroken`.
  3. Transitional (`strings.HasSuffix(p, ": in progress")`) → `ColorWarning`.
  4. Warning phrases (`"delete-protection off"`, `"not encrypted at rest"`, `"no automated backups"`) → `ColorWarning`.
  5. Explicitly SKIP `"maintenance overdue"` — that is a Wave 2 phrase on a Healthy row; color stays Green so the `!` glyph can render.
  6. Empty phrase → `ColorHealthy`.
  7. Unknown phrase → `ColorHealthy` (unknown-status passthrough does not override green).
  8. Remove lookups for `has_writer` / `deletion_protection` / `storage_encrypted` / `backup_retention_period` — they are no longer authoritative. The fetcher has already folded them into the phrase.

### Fetcher (`internal/aws/dbc.go`)

- Gap inventory:
  - `Status` field copies raw AWS keyword. Spec §4 demands the §4 phrase.
  - `Resource.Issues` is never populated. Rule-7 regression.
  - `cis_flags` is computed and stuffed into Fields — jargon, universal-rule violation.
  - `writer_count` / `has_writer` remain in Fields but no longer drive UI (Color func will stop reading them). Keep for RawStruct-adjacent queries, or delete; deletion is cleaner.
- Delta: rewrite `FetchDocDBClustersPage` to:
  1. `computeDBCStatusAndIssues(cluster) (string, []string)` — mirrors `computeDBIStatusAndIssues` pattern in rds.go.
  2. Broken precedence: `failed` → `"failed: cluster operation"`; `inaccessible-encryption-credentials` → `"encryption key unreachable"`; `incompatible-parameters` → `"parameter group incompatible"`; no writer on an otherwise-available cluster → `"no writer: reads only"`.
  3. Transitional: status ∈ {`creating, modifying, backing-up, maintenance, upgrading, starting, stopping, resetting-master-credentials, renaming`} → `"<status>: in progress"`.
  4. Healthy (`available` + writer present): collect Warnings in §4-table order — `delete-protection off` (row 1), `not encrypted at rest` (row 2), `no automated backups` (row 3). Top phrase is index 0. Suffix `(+N-1)` when N≥2.
  5. Unknown status → passthrough (single-entry Issues, bare keyword in Status).
  6. Remove `cis_flags` from `RegisterFieldKeys` and Fields map.
  7. Keep `has_writer`, `writer_count`, `deletion_protection`, `storage_encrypted`, `backup_retention_period` in Fields map — operators may sort/filter on them even though the UI Color no longer reads them. (Open question — simpler to remove; if tests expect absence, the coder deletes.) **Decision: remove `cis_flags`; keep the others; Color func stops using them but they remain as data.**

### `.a9s/views/dbc.yaml` — regenerated

Expected post-viewsgen output:

```yaml
list:
  Cluster ID: { path: DBClusterIdentifier, width: 28 }
  Version:    { path: EngineVersion, width: 10 }
  Status:     { path: Status, width: 32 }   # widened to fit "delete-protection off (+2)"
  Instances:  { path: DBClusterMembers, width: 10 }
  Endpoint:   { path: Endpoint, width: 48 }
detail: (unchanged)
```

## 4. Phase-7 rule-7 wiring (explicit)

| Rule | Implementation location |
|------|------------------------|
| Multi-W1 `(+N)` suffix | `computeDBCStatusAndIssues` in `internal/aws/dbc.go` |
| W1+W2 suffix bump via `resource.BumpFindingSuffix` | `EnrichDBCMaintenance` in `internal/aws/dbc_issue_enrichment.go` |
| Color strips `(+N)` via `resource.StripFindingSuffix` | `types_databases.go` dbc Color func |
| `Resource.Issues` population | `computeDBCStatusAndIssues` returns the ordered Issues slice |
| `"maintenance overdue"` NOT in Warning switch | Color func skips it — row stays Green |
| Detail Attention via `injectAttentionSection` | universal (no per-resource code) |

## 5. Out-of-scope reminders

- Wave 3 CloudWatch metrics (DBInstanceReplicaLag, DatabaseConnections).
- DocumentDB `docdb-snap` resource type (own spec, own impl run).
- CIS header row visualization (banned per universal rules).
- Writer-count column (redundant with `no writer: reads only` Status phrase).
