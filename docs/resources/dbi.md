---
shortName: dbi
name: DB Instances
awsApiRef: https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBInstance.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# dbi — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `dbi`
- **Display name**: DB Instances
- **AWS API reference**: https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBInstance.html
- **List API**: `DescribeDBInstances`
- **Describe API (if any)**: `DescribePendingMaintenanceActions` (one account-wide call) — used by Wave 2 to discover scheduled maintenance. No per-instance `Describe*` is required on top of the list API; `DescribeDBInstances` already returns the full `DBInstance` shape.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `dbc`, `eni`, `kms`, `logs`, `dbi-snap`, `role`, `secrets`, `sg`, `subnet`, `vpc`, `ct-events`.

### `alarm`

- **Why related**: CloudWatch alarms on CPU/Storage/Connections — the first place an on-call engineer looks when an RDS instance starts misbehaving.
- **How discovered**: cross-reference the already-loaded `alarm` list by `MetricAlarm.Dimensions[].Name == "DBInstanceIdentifier"` matching `DBInstanceIdentifier` on this instance — a9s-devops: the only scalable discovery path is the cached-sibling scan; calling `DescribeAlarms` per DB instance does not scale for accounts with hundreds of alarms.
- **Count shown**: yes — a9s-devops: the alarm list is already loaded in cache, so an exact count is cheap and directly tells the operator "how many alerts are watching this DB".

### `dbc`

- **Why related**: Aurora instance → cluster. The cluster owns endpoints, failover policy, and backup configuration the operator needs to reason about.
- **How discovered**: read `DBClusterIdentifier` on the `DBInstance`; look up the already-loaded `dbc` list by that ID (non-Aurora engines leave the field empty and the row is simply absent).
- **Count shown**: yes — 0 for RDS engines, 1 for Aurora members.

### `eni`

- **Why related**: DB instances back onto ENIs — reachability problems (SG rule changes, subnet route changes, AZ failures) manifest at the ENI level, and operators need to see which ENI is attached where.
- **How discovered**: `DescribeNetworkInterfaces(Filters=[{Name=requester-id,Values=amazon-rds}, {Name=vpc-id,Values=<DBSubnetGroup.VpcId>}])`, then filter client-side where `Description` begins with `RDSNetworkInterface` for this instance identifier — a9s-devops: RDS-owned ENIs are not referenced by field on `DBInstance`; the service-owned filter + description-prefix match is the documented workflow; possible=yes, worth=yes because ENI disappearance is a common root cause for "DB unreachable".
- **Count shown**: yes — typically 1 for single-AZ, N for multi-AZ where N = subnet count.

### `kms`

- **Why related**: `KmsKeyId` — storage encryption key. Key disabled, pending deletion, or rotation failure directly breaks the DB.
- **How discovered**: read `KmsKeyId` on the `DBInstance`; look up the already-loaded `kms` list by ARN/KeyId.
- **Count shown**: yes — always 0 when `StorageEncrypted==false`, 1 when encrypted.

### `logs`

- **Why related**: DB engine log exports (e.g. `/aws/rds/instance/<id>/error`) — operators tail these during incident triage.
- **How discovered**: derive log group names from `DBInstanceIdentifier` + `EnabledCloudwatchLogsExports[]` using the AWS-documented pattern `/aws/rds/instance/<DBInstanceIdentifier>/<export-type>`; look up the already-loaded `logs` list by exact log-group name — a9s-devops: the names are deterministic from two fields that come for free on `DescribeDBInstances`; possible=yes, worth=yes because engine logs are the primary non-metric observability surface for RDS.
- **Count shown**: yes — equals `len(EnabledCloudwatchLogsExports)`.

### `dbi-snap`

- **Why related**: Snapshots of this instance — operators pivot here to verify recent backup success or to plan a restore.
- **How discovered**: `DescribeDBSnapshots(DBInstanceIdentifier=<id>)` (server-side filter) — a9s-devops: this is the documented RDS lookup; possible=yes, worth=yes because snapshot health is part of every DB incident post-mortem.
- **Count shown**: yes.

### `role`

- **Why related**: `MonitoringRoleArn` / S3-integration role — enhanced monitoring and S3 import/export both break silently if the role is missing or its trust policy is wrong.
- **How discovered**: read `MonitoringRoleArn` plus `AssociatedRoles[].RoleArn` on the `DBInstance`; look up the already-loaded `role` list by role name (parse ARN).
- **Count shown**: yes — 0 to N.

### `secrets`

- **Why related**: Secrets Manager entries holding master credentials. When app connections fail with "access denied", the password in Secrets Manager is the first suspect.
- **How discovered**: read `MasterUserSecret.SecretArn` on the `DBInstance`; look up the already-loaded `secrets` list by ARN — a9s-devops: this field only exists when RDS-managed passwords are enabled; for classic password-auth instances the field is nil and the row is simply absent; possible=yes, worth=yes.
- **Count shown**: yes — 0 or 1.

### `sg`

- **Why related**: `VpcSecurityGroups` — SGs attached to the instance. SG rule changes are the most common cause of "suddenly can't connect" and the operator needs one keypress to see them.
- **How discovered**: read `VpcSecurityGroups[].VpcSecurityGroupId` on the `DBInstance`; look up the already-loaded `sg` list by group ID.
- **Count shown**: yes — typically 1–5.

### `subnet`

- **Why related**: `DBSubnetGroup.Subnets` — subnets the instance spans. An operator checking AZ/zonal failure patterns pivots here.
- **How discovered**: read `DBSubnetGroup.Subnets[].SubnetIdentifier` on the `DBInstance`; look up the already-loaded `subnet` list by subnet ID.
- **Count shown**: yes.

### `vpc`

- **Why related**: `DBSubnetGroup.VpcId` — the VPC that hosts the DB, needed to reason about routing, flow logs, peering.
- **How discovered**: read `DBSubnetGroup.VpcId` on the `DBInstance`; look up the already-loaded `vpc` list by VPC ID.
- **Count shown**: yes — always 1.

### `ct-events`

- **Why related**: Audit trail for DB config / `ModifyDBInstance`. "Who changed this and when" is the universal incident question.
- **How discovered**: `LookupEvents` with `LookupAttributes=[{AttributeKey=ResourceName,AttributeValue=<DBInstanceIdentifier>}]` — universal pivot, applies to every registered type; see `related-resources.md` §Policy.
- **Count shown**: unknown — a9s-devops: CloudTrail `LookupEvents` returns windowed results; the panel typically shows a page rather than a total count, so "N" is misleading.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `DBInstanceStatus == "available"` → Healthy.
  - **State bucket**: Healthy.
  - **How obtained**: `DBInstance.DBInstanceStatus` on the `DescribeDBInstances` response.

- **Signal**: `DBInstanceStatus` in transitional set (`creating`, `modifying`, `backing-up`, `rebooting`, `renaming`, `resetting-master-credentials`, `starting`, `stopping`, `upgrading`, `maintenance`, `configuring-enhanced-monitoring`, `configuring-iam-database-auth`, `configuring-log-exports`, `converting-to-vpc`, `moving-to-vpc`, `storage-optimization`) → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `DBInstance.DBInstanceStatus` on the `DescribeDBInstances` response.

- **Signal**: `DBInstanceStatus` in `failed`, `storage-full`, `incompatible-network`, `incompatible-option-group`, `incompatible-parameters`, `incompatible-restore`, `inaccessible-encryption-credentials`, `restore-error` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `DBInstance.DBInstanceStatus` on the `DescribeDBInstances` response.

- **Signal**: `BackupRetentionPeriod == 0` → Warning (no automated backups).
  - **State bucket**: Warning.
  - **How obtained**: `DBInstance.BackupRetentionPeriod` on the `DescribeDBInstances` response.

- **Signal**: `PubliclyAccessible == true` → Warning (CIS RDS.2).
  - **State bucket**: Warning.
  - **How obtained**: `DBInstance.PubliclyAccessible` on the `DescribeDBInstances` response.

- **Signal**: `StorageEncrypted == false` → Warning (CIS RDS.3).
  - **State bucket**: Warning.
  - **How obtained**: `DBInstance.StorageEncrypted` on the `DescribeDBInstances` response.

- **Signal**: `DeletionProtection == false` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `DBInstance.DeletionProtection` on the `DescribeDBInstances` response.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: A `PendingMaintenanceAction` for this instance has `ForcedApplyDate` or `AutoAppliedAfterDate` in the past, or is otherwise actionable — scheduled maintenance overdue.
  - **State bucket**: Warning.
  - **API call**: `DescribePendingMaintenanceActions` — one account-wide call.
  - **Cost shape**: account-wide.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `FreeStorageSpace`.
- OUT OF SCOPE: CloudWatch `CPUUtilization`.
- OUT OF SCOPE: CloudWatch `ReplicaLag`.
- OUT OF SCOPE: CloudWatch `DatabaseConnections`.

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
| transitional status (`modifying`/`rebooting`/etc.) | 1 | Warning | n/a | S2, S4 | `<status>: <PendingModifiedValues first non-empty key>` when available, else bare `<status>` (e.g. `modifying: DBInstanceClass`, `rebooting`) | `Instance is <status> — pending changes in progress.` |
| `failed` | 1 | Broken | n/a | S2, S4 | `failed` | `Instance is in a failed state — contact AWS support or restore from snapshot.` |
| `storage-full` | 1 | Broken | n/a | S2, S4 | `storage-full` | `Instance storage full — scale up or free space to recover.` |
| `incompatible-network` | 1 | Broken | n/a | S2, S4 | `incompatible-network` | `Network config incompatible — check DB subnet group AZ coverage.` |
| `incompatible-option-group` | 1 | Broken | n/a | S2, S4 | `incompatible-option-group` | `Option group incompatible — remove or update options.` |
| `incompatible-parameters` | 1 | Broken | n/a | S2, S4 | `incompatible-parameters` | `Parameter group incompatible — review custom parameters.` |
| `incompatible-restore` | 1 | Broken | n/a | S2, S4 | `incompatible-restore` | `Restore failed — check snapshot compatibility and engine version.` |
| `restore-error` | 1 | Broken | n/a | S2, S4 | `restore-error` | `Restore error — review source snapshot and target engine version.` |
| `inaccessible-encryption-credentials` | 1 | Broken | n/a | S2, S4 | `encryption key unavailable` | `KMS key for storage is unavailable — check key state and grants.` |
| `BackupRetentionPeriod == 0` | 1 | Warning | n/a | S2, S4 | `no automated backups` | `Automated backups disabled (BackupRetentionPeriod=0).` |
| `PubliclyAccessible == true` | 1 | Warning | n/a | S2, S4 | `publicly accessible` | `Instance is reachable from the public internet (CIS RDS.2).` |
| `StorageEncrypted == false` | 1 | Warning | n/a | S2, S4 | `unencrypted storage` | `Storage encryption at rest is disabled (CIS RDS.3).` |
| `DeletionProtection == false` | 1 | Warning | n/a | S2, S4 | `deletion protection off` | `Deletion protection is disabled — instance can be deleted in one API call.` |
| Pending maintenance overdue | 2 | Warning on Healthy row | `~` | S3, S4, S5 | `maintenance scheduled` | `Pending maintenance action overdue: <ActionType> (<Description>).` |

Notes on the table:

- **Transitional S4 suffix**: when `DBInstanceStatus` is in the transitional set and `PendingModifiedValues` has at least one non-empty field, S4 = `<status>: <first-non-empty key>`. Key iteration order is the field order on `types.PendingModifiedValues` (deterministic via reflection or explicit priority list). When no pending value is set, S4 = bare status keyword. Rationale: §4.1 + §6 a9s-devops (2026-04-20, possible=yes, worth=yes); user decision (2026-04-21).
- **Per-failure S5 sentences**: each failure status has its own remedy sentence. Rationale: §4 groups them for brevity but operators need the specific remedy at 3am; user decision (2026-04-21).
- **Broken precedence**: when `DBInstanceStatus` is itself broken (`failed`, `storage-full`, `incompatible-*`, `restore-error`, `inaccessible-encryption-credentials`), the status-based S4/S5 takes precedence over configuration warnings (no-backups, publicly-accessible, unencrypted, deletion-protection-off). The row renders red, not yellow, and does not stack a second S4 string.
- **Warning precedence when available**: when `DBInstanceStatus == "available"` but one or more configuration warnings apply, S4 = the first one in this order: `no automated backups` > `publicly accessible` > `unencrypted storage` > `deletion protection off`. Rationale: operators want the highest-severity policy miss surfaced; only one line fits S4.

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`DORMANT`, `stopped`, `available`, `failed`) in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column. Tests will assert the cause is present.
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), you may omit the row from this table entirely; §3 still describes it.
- Keep both columns short enough to fit: List text ≤ 40 chars, Detail text ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Mostly yes — the failure statuses (`storage-full`, `inaccessible-encryption-credentials`) are already self-describing in S4 and the configuration warnings (`no automated backups`, `publicly accessible`, `unencrypted storage`, `deletion protection off`) name the exact policy miss without jargon. The one gap is the transitional bucket: a bare `modifying` tells the operator the instance is busy but not what's being modified — a minor follow-up is to append the first non-empty key of `PendingModifiedValues` (e.g. `modifying: DBInstanceClass`) so the list row explains itself.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- Per-instance `DescribeDBInstances` as a Wave 2 call — the list API already returns the full shape, so no Wave 2 per-resource fan-out is needed on top of `DescribePendingMaintenanceActions`.
- Read-replica lag and read-replica-specific status surfaced via `StatusInfos[]` — a9s-devops: not worth it as a per-row S4 signal for dbi; replica lag is Wave 3 (CloudWatch `ReplicaLag`) and is consumed in the context of a specific replication topology rather than a list row.
- `ct-events` count badge — a9s-devops: not worth filling from AWS surface; `LookupEvents` paginates over a time window with no documented total, and a truncated number in the panel would mislead the operator.

## 6. Citations

- a9s golden doc — dbi Per-type contract row and AWS API URL — `docs/related-resources.md` § `Per-type contract` (dbi row) and § `dbi`.
- a9s golden doc — dbi reasoning for each related target (`alarm`, `dbc`, `eni`, `kms`, `logs`, `dbi-snap`, `role`, `secrets`, `sg`, `subnet`, `vpc`, `ct-events`) — `docs/related-resources.md` § `dbi`.
- a9s golden doc — Wave 1 / Wave 2 / Wave 3 signal cells — `docs/attention-signals.md` § Databases & Storage → `dbi` row.
- AWS API Reference — `DBInstance.DBInstanceStatus`, `.BackupRetentionPeriod`, `.PubliclyAccessible`, `.StorageEncrypted`, `.DeletionProtection`, `.KmsKeyId`, `.VpcSecurityGroups[].VpcSecurityGroupId`, `.DBSubnetGroup.{VpcId,Subnets[].SubnetIdentifier}`, `.DBClusterIdentifier`, `.EnabledCloudwatchLogsExports[]`, `.MonitoringRoleArn`, `.AssociatedRoles[].RoleArn`, `.MasterUserSecret.SecretArn` — `AWS API Reference: API_DBInstance` (https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBInstance.html).
- AWS API Reference — `DescribePendingMaintenanceActions` (account-wide) — (https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribePendingMaintenanceActions.html).
- AWS API Reference — `DescribeDBSnapshots` with `DBInstanceIdentifier` filter — (https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBSnapshots.html).
- AWS API Reference — `LookupEvents` with `ResourceName` attribute for ct-events pivot — (https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_LookupEvents.html).
- Read-only invariant — `docs/architecture.md` § `What is a9s?`.
- S1–S5 surface rules — skill `a9s-resource-spec` § `Allowed visualization surfaces (exactly five)`.
- `alarm` discovery mechanism (sibling-list dimension scan) — a9s-devops (2026-04-20): possible=yes, worth=yes. `DescribeAlarms` is already cached from the alarm top-level list; filtering its `Dimensions[].Name=="DBInstanceIdentifier"` is cheaper and more accurate than calling alarm APIs per DB.
- `eni` discovery mechanism (service-owned requester-id filter) — a9s-devops (2026-04-20): possible=yes, worth=yes. `DescribeNetworkInterfaces` supports `Filters=[{Name=requester-id,Values=amazon-rds}]`; description-prefix matching narrows to the specific DB instance.
- `logs` discovery mechanism (derived log-group name) — a9s-devops (2026-04-20): possible=yes, worth=yes. RDS log-export group names follow the deterministic pattern `/aws/rds/instance/<DBInstanceIdentifier>/<export-type>` driven by `DBInstanceIdentifier` + `EnabledCloudwatchLogsExports[]`.
- `dbi-snap` discovery mechanism (server-side DBInstanceIdentifier filter) — a9s-devops (2026-04-20): possible=yes, worth=yes. `DescribeDBSnapshots` accepts `DBInstanceIdentifier` as a server-side filter.
- `secrets` discovery mechanism (`MasterUserSecret.SecretArn`) — a9s-devops (2026-04-20): possible=yes, worth=yes. Field is populated only when RDS-managed passwords are enabled; absence is meaningful and expected for classic password-auth instances.
- Count-shown values for the related panel — a9s-devops (2026-04-20): possible=yes (most), worth=yes. For cached-sibling lookups (`alarm`, `kms`, `role`, `sg`, `subnet`, `vpc`, `dbc`, `logs`, `secrets`) exact counts are free and valuable; for `dbi-snap` and `eni` counts come from live API responses and are still cheap enough; `ct-events` is windowed and a count would be misleading.
- Cause text for failure statuses — a9s-devops (2026-04-20): possible=partial, worth=yes. `DescribeDBInstances` does not expose a structured reason for primary-instance failures (`StatusInfos[]` is read-replica-scoped); the status keyword itself is operator-meaningful and the S5 sentence names the operator remedy rather than fabricating a reason string.
- Pending maintenance S4 text (`maintenance scheduled`) and S5 sentence shape — a9s-devops (2026-04-20): possible=yes, worth=yes. `PendingMaintenanceAction.Action` + `.Description` + `.ForcedApplyDate`/`.AutoAppliedAfterDate` from `DescribePendingMaintenanceActions` provide the fields needed for a short human-readable summary.
- Transitional-status UX gap (bare `modifying`) and recommended fix — a9s-devops (2026-04-20): possible=yes, worth=yes. `PendingModifiedValues` on `DBInstance` carries the actual pending change set (e.g. `DBInstanceClass`, `AllocatedStorage`, `EngineVersion`); surfacing the first non-empty key gives the operator a readable "what's being changed" without opening detail.
- Wave 3 exclusions — a9s-devops (2026-04-20): possible=yes, worth=no as list-row signals. CloudWatch per-resource metrics scale per-datapoint per-minute; they belong in a metrics view, not on the resource list.
- user decision (2026-04-21): transitional S4 shape is `<status>: <first non-empty PendingModifiedValues key>` when available, else bare status. Pins the §4.1 recommendation into §4 so the coder has no latitude.
- user decision (2026-04-21): each Broken failure status renders its own S5 remedy sentence (per the table in §4), rather than a single generic "in a failed state" sentence.
