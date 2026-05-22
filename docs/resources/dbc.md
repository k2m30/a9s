---
shortName: dbc
name: DB Clusters
awsApiRef: https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DBCluster.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
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
  **Empirically (AS-145, verified live on dev-readonly account `872515270585` eu-west-2),
  both endpoints return rows for both engine families** — e.g. an `aurora-postgresql`
  cluster surfaces from the DocDB endpoint as well as the RDS endpoint. Naïve concat would
  therefore double-count every cluster that both endpoints return.
  **Dedup contract**: results are concatenated DocDB-side first, then deduped by
  `Resource.ID` with first-occurrence-wins. The DocDB-side row is therefore preserved on
  collisions, which is the engine-correct one for detail enrichment and the
  `dbc → dbc-snap` related-panel pivot (those branches type-assert on
  `RawStruct` being a `docdbtypes.DBCluster`). See `internal/aws/dbc.go` (concat region)
  and `dedupResourcesByID` for the implementation; this dedup behavior is part of the
  fetcher contract — do not remove it.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `dbi`, `dbc-snap`, `kms`, `logs`, `secrets`, `sg`, `subnet`, `vpc`, `ct-events`.

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
- **How discovered**: universal pivot — applies to every registered type; see related-resources.md §Policy. Filter CloudTrail `LookupEvents` by `ResourceName=<cluster-id>` or `ResourceType=AWS::RDS::DBCluster`.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `Status == "available"` → Healthy.
  - **State bucket**: Healthy.
  - **How obtained**: `Status` field on the `DescribeDBClusters` response.

- **Signal**: `Status` is transitional (e.g. `creating`, `modifying`, `backing-up`, `maintenance`, `upgrading`, `starting`, `stopping`, `resetting-master-credentials`, `renaming`) → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `Status` field on the `DescribeDBClusters` response.

- **Signal**: `Status` in `failed` / `inaccessible-encryption-credentials` / `incompatible-parameters` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `Status` field on the `DescribeDBClusters` response.

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

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: Cluster has a pending maintenance action with `ForcedApplyDate` or `AutoAppliedAfterDate` in the past → Warning.
  - **State bucket**: Warning.
  - **API call**: `DescribePendingMaintenanceActions` — one account-wide call (shared with `dbi`), bucket results by `ResourceIdentifier` (cluster ARN).
  - **Cost shape**: account-wide.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `DBInstanceReplicaLag`, `DatabaseConnections`.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing" — e.g. maintenance scheduled, certificate expiring soon. `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `stopping: Server.SpotInstanceShutdown`, `expires in 7d`). **Healthy rows render blank** — no `OK` / `available` / `ACTIVE` / `running`. Empty means "nothing to see." |
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
| `Status` transitional | 1 | Warning | n/a | S2, S4 | `<status>: in progress` (e.g. `modifying: in progress`) | `Cluster is <status>; operations in flight — wait for it to settle.` |
| `Status == failed` | 1 | Broken | n/a | S2, S4 | `failed: cluster operation` | `Cluster reports Status=failed; inspect recent CloudTrail ModifyDBCluster events.` |
| `Status == inaccessible-encryption-credentials` | 1 | Broken | n/a | S2, S4 | `encryption key unreachable` | `KMS key for this cluster is disabled or inaccessible — restore the key to recover the cluster.` |
| `Status == incompatible-parameters` | 1 | Broken | n/a | S2, S4 | `parameter group incompatible` | `Cluster parameter group has values the engine rejects — revert the last parameter change.` |
| No writer in `DBClusterMembers[]` | 1 | Broken | n/a | S2, S4 | `no writer: reads only` | `No cluster member has IsClusterWriter=true — writes are refused until a primary is present.` |
| `DeletionProtection == false` | 1 | Warning | n/a | S2, S4 | `delete-protection off` | `DeletionProtection is disabled — an accidental DeleteDBCluster will destroy the cluster.` |
| `StorageEncrypted == false` | 1 | Warning | n/a | S2, S4 | `not encrypted at rest` | `StorageEncrypted is false — cluster storage is not protected by KMS.` |
| `BackupRetentionPeriod == 0` | 1 | Warning | n/a | S2, S4 | `no automated backups` | `BackupRetentionPeriod is 0 — automated snapshots are disabled; PITR will not work.` |
| Pending maintenance action overdue | 2 | Warning | `!` | S1, S3, S4, S5 | `maintenance overdue` | `AWS-mandated maintenance past its ForcedApplyDate — AWS will apply it in the next window.` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`DORMANT`, `stopped`, `available`, `failed`) in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column. Tests will assert the cause is present.
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), you may omit the row from this table entirely; §3 still describes it.
- Keep both columns short enough to fit: List text ≤ 40 chars, Detail text ≤ 100 chars.

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
- a9s golden doc — Wave 1 signals (`Status`, `DBClusterMembers`, `DeletionProtection`, `StorageEncrypted`, `BackupRetentionPeriod`) — `docs/attention-signals.md` § Databases & Storage, row `dbc`.
- a9s golden doc — Wave 2 signal (`DescribePendingMaintenanceActions`, shared with `dbi`) — `docs/attention-signals.md` § Databases & Storage, row `dbc`.
- a9s golden doc — read-only invariant — `docs/architecture.md` § "What is a9s?".
- AWS Go SDK v2 — `DBCluster.Status` / `DBClusterMembers[].IsClusterWriter` / `DeletionProtection` / `StorageEncrypted` / `BackupRetentionPeriod` / `KmsKeyId` / `MasterUserSecret.SecretArn` / `EnabledCloudwatchLogsExports` / `VpcSecurityGroups[].VpcSecurityGroupId` / `DBSubnetGroup` all present on the list response shape — `AWS SDK Go v2 — service/docdb/types.DBCluster`.
- AWS Go SDK v2 — `DBClusterMember.IsClusterWriter *bool` — `AWS SDK Go v2 — service/docdb/types.DBClusterMember § IsClusterWriter`.
- AWS Go SDK v2 — DocumentDB `DescribeDBClusters` is the list operation (not RDS's) — `AWS SDK Go v2 — service/docdb § DescribeDBClusters`.
- AWS API Reference (fallback) — DocumentDB `DescribeDBClusters` — <https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DescribeDBClusters.html>.
- AWS API Reference (fallback) — DocumentDB `DescribeDBSubnetGroups` (used to resolve subnets + VPC behind `DBSubnetGroup`) — <https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DescribeDBSubnetGroups.html>.
- amendment — the Source URL and display name for `dbc` in `docs/attention-signals.md` were corrected from RDS (`API_DescribeDBClusters` under `AmazonRDS`) to DocumentDB (`API_DescribeDBClusters` under `documentdb`), and the Wave 3 CloudWatch metric `AuroraReplicaLag` was replaced with `DBInstanceReplicaLag` to match the DocumentDB namespace. Rationale: `docs/related-resources.md` anchors `dbc` at `documentdb/latest/developerguide/API_DBCluster.html` and the user specification is `dbc (DocumentDB Cluster)`. Field names listed in the Wave 1 cell (`Status`, `DBClusterMembers`, `IsClusterWriter`, `DeletionProtection`, `StorageEncrypted`, `BackupRetentionPeriod`) match `service/docdb/types.DBCluster` verbatim, so no field edits were needed.

<!-- BEGIN GENERATED: header -->
dbc — DATABASES & STORAGE. Lifecycle key: `status`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Approximate? |
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
