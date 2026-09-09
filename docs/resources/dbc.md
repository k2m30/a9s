---
shortName: dbc
name: DB Clusters
awsApiRef: https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DBCluster.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# dbc — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `dbc`
- **Display name**: DB Clusters
- **AWS API reference**: <https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DBCluster.html>
- **List API**: BOTH `c.DocDB.DescribeDBClusters` AND `c.RDS.DescribeDBClusters`, results merged via the `docdb:` / `rds:` continuation-token prefix scheme.
- **Describe API (if any)**: `DescribePendingMaintenanceActions` (one account-wide call, shared with `dbi`)
- **Coverage**: this resource type covers BOTH DocumentDB clusters AND Aurora + Multi-AZ DB clusters.
  Both SDKs must be called to get complete coverage; the a9s fetcher calls both and merges
  results using the `docdb:` / `rds:` continuation-token prefix scheme.
  **The DocDB and RDS SDKs are NOT interchangeable, and they overlap.** The docdb-side SDK
  (docdb@v1.48.12/api_op_DescribeDBClusters.go:14-19) instructs callers to use
  `filterName=engine,Values=docdb` for DocDB-only results; unfiltered behavior is
  documented as ambiguous, not engine-agnostic. The rds-side SDK
  (rds@v1.116.3/api_op_DescribeDBClusters.go:19-28) returns Aurora + Multi-AZ clusters
  and may also return Neptune / DocumentDB rows per the official RDS docstring.
  **Empirically (AS-145, verified live on dev-readonly account `000000000000` eu-west-2),
  both endpoints return rows for both engine families** — e.g. an `aurora-postgresql`
  cluster surfaces from the DocDB endpoint as well as the RDS endpoint. Naïve concat would
  therefore double-count every cluster that both endpoints return.
  **Dedup contract**: results are concatenated DocDB-side first, then deduped by
  `Resource.ID` with first-occurrence-wins. The DocDB-side row is therefore preserved on
  collisions, which is the engine-correct one for detail enrichment and the
  `dbc → dbc-snap` related-panel pivot (those branches type-assert on
  `RawStruct` being a `docdbtypes.DBCluster`). See `core/aws/dbc.go` (concat region)
  and `dedupResourcesByID` for the implementation; this dedup behavior is part of the
  fetcher contract — do not remove it.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `alarm`, `dbi`, `dbc-snap`, `kms`, `logs`, `secrets`, `sg`, `subnet`, `vpc`, `ct-events`.

### `alarm`

- **Why related**: Cluster CW alarms — CPU, storage, connections — that watch this cluster.
- **How discovered**: call `DescribeAlarms` and filter client-side for `Dimensions` containing `Name=DBClusterIdentifier, Value=<cluster-id>` (AWS/DocDB namespace).
- **Count shown**: yes.

### `dbi`

- **Why related**: Cluster member instances — each writer/replica the cluster contains.
- **How discovered**: read `DBClusterMembers[].DBInstanceIdentifier` on the cluster; cross-reference the already-loaded `dbi` list by instance identifier.
- **Count shown**: yes.

### `dbc-snap`

- **Why related**: Cluster snapshots — point-in-time backups the operator may need to restore or audit.
- **How discovered**: call `DescribeDBClusterSnapshots(DBClusterIdentifier=<cluster-id>)`, or cross-reference the already-loaded `dbc-snap` list by `DBClusterIdentifier`.
- **Count shown**: yes.

### `kms`

- **Why related**: Cluster encryption key — customer-managed CMK that wraps cluster storage.
- **How discovered**: read `KmsKeyId` on the cluster; cross-reference the already-loaded `kms` list by key ARN.
- **Count shown**: yes.

### `logs`

- **Why related**: Cluster log exports — audit, profiler, query-log streams the cluster pushes to CloudWatch.
- **How discovered**: read `EnabledCloudwatchLogsExports[]` on the cluster; the matching log groups follow the convention `/aws/docdb/<cluster-id>/<log-type>`. Cross-reference the already-loaded `logs` list by that name prefix.
- **Count shown**: yes.

### `secrets`

- **Why related**: Master credentials stored in Secrets Manager — the secret AWS manages when the cluster uses `ManageMasterUserPassword`.
- **How discovered**: read `MasterUserSecret.SecretArn` on the cluster; cross-reference the already-loaded `secrets` list by ARN.
- **Count shown**: yes.

### `sg`

- **Why related**: VpcSecurityGroups — network ACL in front of the cluster's instances.
- **How discovered**: read `VpcSecurityGroups[].VpcSecurityGroupId` on the cluster; cross-reference the already-loaded `sg` list by ID.
- **Count shown**: yes.

### `subnet`

- **Why related**: DBSubnetGroup subnets — the AZs where cluster instances can be placed.
- **How discovered**: read `DBSubnetGroup` (subnet-group name) on the cluster, then call `DescribeDBSubnetGroups(DBSubnetGroupName=<name>)` once per unique name and extract `Subnets[].SubnetIdentifier`; cross-reference the already-loaded `subnet` list by ID.
- **Count shown**: yes.

### `vpc`

- **Why related**: DBSubnetGroup VPC — the network the cluster sits inside.
- **How discovered**: same as `subnet` — `DescribeDBSubnetGroups` returns `VpcId`; cross-reference the already-loaded `vpc` list by ID.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Audit trail for cluster changes (CreateDBCluster, ModifyDBCluster, DeleteDBCluster, FailoverDBCluster).
- **How discovered**: universal pivot — applies to every registered type; see docs/related-resources.md §Policy. Filter CloudTrail `LookupEvents` by `ResourceName=<cluster-id>` or `ResourceType=AWS::RDS::DBCluster`.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeDBClusters](https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DescribeDBClusters.html)

Transcribed from `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `dbc`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `Status` is transitional (e.g. `creating`, `modifying`, `backing-up`, `maintenance`, `upgrading`, `starting`, `stopping`, `resetting-master-credentials`, `renaming`) → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `Status` field on the `DescribeDBClusters` response.

- **Signal**: `Status == inaccessible-encryption-credentials` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `Status` field on the `DescribeDBClusters` response.

- **Signal**: `Status == inaccessible-encryption-credentials`.
  - **State bucket**: Broken.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `Status == incompatible-parameters`.
  - **State bucket**: Broken.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: No `DBClusterMembers[]` entry with `IsClusterWriter == true` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: scan `DBClusterMembers[]` on the list response; no entry flagged as writer means the cluster has no primary accepting writes.

- **Signal**: `DeletionProtection == false` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `DeletionProtection` boolean on the list response.

- **Signal**: `StorageEncrypted == false` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `StorageEncrypted` boolean on the list response.

- **Signal**: `BackupRetentionPeriod == 0` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `BackupRetentionPeriod` int on the list response.

- **Signal**: `MultiAZ == false`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `AutoMinorVersionUpgrade == false` (Aurora only).
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `IAMDatabaseAuthenticationEnabled == false` (Aurora only).
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `MasterUsername` is a vendor default.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: Cluster has a pending maintenance action with `ForcedApplyDate` or `AutoAppliedAfterDate` in the past → Broken.
  - **State bucket**: Broken.
  - **API call**: `DescribePendingMaintenanceActions` — one account-wide call (shared with `dbi`), bucket results by `ResourceIdentifier` (cluster ARN).
  - **Cost shape**: account-wide.

- **Signal**: no backup plan selection matches this cluster.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `DBInstanceReplicaLag`, `DatabaseConnections`.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `dbc`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `Status` transitional | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `<status>: in progress` |
| `Status == failed` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `failed: cluster operation` |
| `Status == inaccessible-encryption-credentials` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `encryption key unreachable` |
| `Status == incompatible-parameters` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `parameter group incompatible` |
| No writer in `DBClusterMembers[]` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `no writer: reads only` |
| `DeletionProtection == false` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `delete-protection off` |
| `StorageEncrypted == false` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `not encrypted at rest` |
| `BackupRetentionPeriod == 0` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `no automated backups` |
| `MultiAZ == false` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `single-AZ` |
| `AutoMinorVersionUpgrade == false` (Aurora only) | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `auto minor version upgrade off` |
| `IAMDatabaseAuthenticationEnabled == false` (Aurora only) | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `IAM database authentication off` |
| `MasterUsername` is a vendor default | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `default master username` |
| Pending maintenance action overdue | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `maintenance overdue` |
| no backup plan selection matches this cluster | 2 | Warning | `~` | S2, S3, S4, S5 | `not covered by a backup plan` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`DORMANT`, `stopped`, `available`, `failed`) in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column. Tests will assert the cause is present.
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), you may omit the row from this table entirely; §3 still describes it.
- List text ≤ 40 chars. The Detail sentence lives on the finding definition and is generated into the Findings table below; it is never written here.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for every §4 row — the Status column always carries the cause (`no writer: reads only`, `encryption key unreachable`, `delete-protection off`, `maintenance overdue`), never a bare state word. All problem rows are self-explanatory in the list — operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- a9s golden doc — per-type contract for `dbc` lists 10 related targets — `docs/related-resources.md` § Per-type contract, row `dbc`.
- a9s golden doc — `alarm` pivot is "Cluster CW alarms" — `docs/related-resources.md` § `dbc`.
- a9s golden doc — `dbi` pivot is "Cluster member instances" — `docs/related-resources.md` § `dbc`.
- a9s golden doc — `dbc-snap` pivot is "Cluster snapshots" — `docs/related-resources.md` § `dbc`.
- a9s golden doc — `kms` pivot is "Cluster encryption key" — `docs/related-resources.md` § `dbc`.
- a9s golden doc — `logs` pivot is "Cluster log exports" — `docs/related-resources.md` § `dbc`.
- a9s golden doc — `secrets` pivot is "Master credentials in Secrets Manager" — `docs/related-resources.md` § `dbc`.
- a9s golden doc — `sg` pivot is "VpcSecurityGroups — cluster SGs" — `docs/related-resources.md` § `dbc`.
- a9s golden doc — `subnet` pivot is "DBSubnetGroup subnets" — `docs/related-resources.md` § `dbc`.
- a9s golden doc — `vpc` pivot is "DBSubnetGroup VPC" — `docs/related-resources.md` § `dbc`.
- a9s golden doc — `ct-events` pivot is "Audit trail for cluster changes" — `docs/related-resources.md` § `dbc` and § Policy (universal pivot).
- a9s golden doc — Wave 1 signals (`Status`, `DBClusterMembers`, `DeletionProtection`, `StorageEncrypted`, `BackupRetentionPeriod`) — `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `dbc`.
- a9s golden doc — Wave 2 signal (`DescribePendingMaintenanceActions`, shared with `dbi`) — `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `dbc`.
- a9s golden doc — read-only invariant — `docs/architecture.md` § "What is a9s?".
- AWS Go SDK v2 — `DBCluster.Status` / `DBClusterMembers[].IsClusterWriter` / `DeletionProtection` / `StorageEncrypted` / `BackupRetentionPeriod` / `KmsKeyId` / `MasterUserSecret.SecretArn` / `EnabledCloudwatchLogsExports` / `VpcSecurityGroups[].VpcSecurityGroupId` / `DBSubnetGroup` all present on the list response shape — `AWS SDK Go v2 — service/docdb/types.DBCluster`.
- AWS Go SDK v2 — `DBClusterMember.IsClusterWriter *bool` — `AWS SDK Go v2 — service/docdb/types.DBClusterMember § IsClusterWriter`.
- AWS Go SDK v2 — DocumentDB `DescribeDBClusters` is the list operation (not RDS's) — `AWS SDK Go v2 — service/docdb § DescribeDBClusters`.
- AWS API Reference (fallback) — DocumentDB `DescribeDBClusters` — <https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DescribeDBClusters.html>.
- AWS API Reference (fallback) — DocumentDB `DescribeDBSubnetGroups` (used to resolve subnets + VPC behind `DBSubnetGroup`) — <https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DescribeDBSubnetGroups.html>.
- amendment — `dbc` was corrected from RDS to DocumentDB: the display name and API reference follow DocumentDB, and the deferred replica-lag metric is `DBInstanceReplicaLag`, not `AuroraReplicaLag` — `docs/attention-signals.md § Not yet implemented`. Rationale: `docs/related-resources.md` § Per-type contract anchors `dbc` at `documentdb/latest/developerguide/API_DBCluster.html` and the user specification is `dbc (DocumentDB Cluster)`. The field names (`Status`, `DBClusterMembers`, `IsClusterWriter`, `DeletionProtection`, `StorageEncrypted`, `BackupRetentionPeriod`) match `service/docdb/types.DBCluster` verbatim, so no field edits were needed.

<!-- BEGIN GENERATED: header -->
dbc — DATABASES & STORAGE. Status key: `status` — the key the status cell reads, and the column naming it is the status column.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| dbc.broken.failed | failed: cluster operation | broken | wave1 | The cluster's last operation failed and it is not serving, so both its writer and its readers are unavailable. Read the cluster events for the failing step, and plan a snapshot restore — a cluster in this state rarely returns on its own. |
| dbc.broken.encryption\_key\_unreachable | encryption key unreachable | broken | wave1 | The KMS key protecting this cluster's volume cannot be used, so no node can read the data and the cluster will not start. Check whether the key is disabled, pending deletion, or whether its policy still allows the database service to use it. |
| dbc.broken.incompatible\_parameters | parameter group incompatible | broken | wave1 | The cluster parameter group holds a setting the engine rejects, so the cluster will not come up with it applied. Correct the parameter and reboot the cluster; the events list names the setting. |
| dbc.broken.no\_writer | no writer: reads only | broken | wave1 | The cluster has no writer instance, so every write fails while reads may still succeed and hide the outage from a shallow health check. Check whether a failover is stuck or the writer was deleted, then promote a reader or add an instance. |
| dbc.warn.transitional | <status>: in progress | warn | wave1 | The cluster is mid-operation, so it may fail over or drop connections before it settles. Wait for it to return to available rather than starting another change on top of this one. |
| dbc.warn.deletion\_protection\_off | delete-protection off | warn | wave1 | One delete call removes this cluster and every instance in it. Turn deletion protection on so it has to be disabled deliberately first. |
| dbc.warn.not\_encrypted\_at\_rest | not encrypted at rest | warn | wave1 | The cluster's volume, its snapshots and its backups are stored unencrypted, and that cannot be changed in place. Snapshot it, copy the snapshot with a KMS key, and restore into a new encrypted cluster at the next opportunity for a cutover. |
| dbc.warn.no\_automated\_backups | no automated backups | warn | wave1 | Backup retention is zero, so there is no point-in-time recovery for this cluster and a bad migration can only be undone from a manual snapshot. Set a retention period that matches how much data loss you could actually accept. |
| dbc.maintenance-overdue | maintenance overdue | broken | wave2 | A pending maintenance action on this cluster is past the date AWS will apply it by, so AWS takes the outage at a time of its choosing rather than yours. Apply it in your own maintenance window now. |
| dbc.single-az | single-AZ | warn | wave1 | The cluster has no instance in a second Availability Zone, so an AZ failure takes it down until you restore it. Add a replica in another AZ. |
| dbc.minor-upgrade-off | auto minor version upgrade off | warn | wave1 | Minor engine patches — including security fixes — are never applied automatically. Enable auto minor version upgrade, or schedule the patching yourself. |
| dbc.iam-auth-off | IAM database authentication off | warn | wave1 | Connections authenticate with long-lived database passwords only. Enable IAM database authentication so credentials become short-lived tokens tied to IAM identities. |
| dbc.default-master-user | default master username | warn | wave1 | The administrative account uses the vendor default name, so an attacker only has to guess the password. Create a differently-named administrative user and retire this one. |
| dbc.not-in-backup-plan | not covered by a backup plan | warn | wave2 | No backup plan selects this cluster, so its retention is whatever the cluster's own automated backups happen to be. Add it to a plan by ARN, or give it a tag one of your plans already selects on. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| sg | Security Groups | no |
| alarm | CloudWatch Alarms | yes |
| logs | Log Groups | yes |
| kms | KMS Key | no |
| secrets | Secrets Manager | yes |
| dbi | RDS Instances | yes |
| dbc-snap | DB Cluster Snapshots | yes |
| subnet | Subnets | no |
| vpc | VPC | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
