---
shortName: redshift
name: Redshift Clusters
awsApiRef: https://docs.aws.amazon.com/redshift/latest/APIReference/API_Cluster.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# redshift — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `redshift`
- **Display name**: Redshift Clusters
- **AWS API reference**: https://docs.aws.amazon.com/redshift/latest/APIReference/API_Cluster.html
- **List API**: `DescribeClusters` (returns fully-populated `Cluster` shapes in one paged call; no per-resource Describe needed for Wave 1 signals).
- **Describe API (if any)**: not used — Wave 2 is `None` per `attention-signals.md`. `DescribeLoggingStatus` is only invoked to resolve the `logs` / `s3` related-panel pivots, not for attention signals.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `cfn`, `kms`, `logs`, `role`, `s3`, `secrets`, `sg`, `subnet`, `vpc`, `ct-events`.

### `alarm`

- **Why related**: Cluster-scoped CloudWatch alarms (e.g. `CPUUtilization`, `PercentageDiskSpaceUsed`, `HealthStatus`) are the first page an operator opens when a cluster is in trouble.
- **How discovered**: cross-reference the already-loaded `alarm` list by `MetricAlarm.Dimensions[].Name == "ClusterIdentifier"` and `Value == Cluster.ClusterIdentifier` — a9s-devops: standard CloudWatch pattern, AWS/Redshift namespace uses `ClusterIdentifier` as the dimension key; no extra API call needed when the alarm list is already loaded in-session.
- **Count shown**: yes.

### `cfn`

- **Why related**: If the cluster was provisioned by CloudFormation, the operator wants to jump to the owning stack to read its events and related resources without re-navigating.
- **How discovered**: read `Cluster.Tags[]` and match the AWS-reserved tag key `aws:cloudformation:stack-name` (value = stack name); cross-reference the already-loaded `cfn` list on `StackName` — a9s-devops: `aws:cloudformation:stack-name` is the canonical AWS tag stamped on every CFN-managed resource; no extra API call needed.
- **Count shown**: yes (0 or 1).

### `kms`

- **Why related**: The CMK that encrypts storage for this cluster. Key state (disabled / pending deletion) directly affects cluster availability.
- **How discovered**: read `Cluster.KmsKeyId` on the cluster; cross-reference the already-loaded `kms` list by key ID / ARN.
- **Count shown**: yes (0 or 1).

### `logs`

- **Why related**: Redshift audit / connection / user-activity logs go either to CloudWatch Logs or S3. The operator wants one keypress to reach the log group that carries query and connection events for this cluster.
- **How discovered**: call `DescribeLoggingStatus(ClusterIdentifier)`. When `LoggingEnabled==true` AND `LogDestinationType==cloudwatch`, the log groups follow the well-known pattern `/aws/redshift/cluster/<ClusterIdentifier>/connectionlog` | `/useractivitylog` | `/userlog` (filter by the `LogExports[]` the cluster actually has enabled); cross-reference the already-loaded `logs` list by log-group-name prefix — a9s-devops: this is the AWS-documented naming pattern for Redshift audit logs; no per-cluster API beyond `DescribeLoggingStatus`.
- **Count shown**: yes.

### `role`

- **Why related**: IAM roles attached to the cluster (used by `COPY`, `UNLOAD`, federated-query). When a `COPY` fails with `AccessDenied`, this is the first pivot.
- **How discovered**: read `Cluster.IamRoles[].IamRoleArn`; cross-reference the already-loaded `role` list by role ARN / name.
- **Count shown**: yes.

### `s3`

- **Why related**: The S3 bucket that receives Redshift audit logs when logging is configured to S3 (and, by operator convention, the buckets used by `COPY`/`UNLOAD` staging — though only the audit bucket is statically discoverable).
- **How discovered**: call `DescribeLoggingStatus(ClusterIdentifier)`. When `LoggingEnabled==true` AND `LogDestinationType==s3`, read `BucketName` and cross-reference the already-loaded `s3` list by bucket name — a9s-devops: COPY/UNLOAD target buckets are not carried on any read-only Redshift API (they live inside SQL statement history), so the panel limits itself to the audit-log bucket.
- **Count shown**: yes (0 or 1 — the audit-log bucket when S3 logging is enabled).

### `secrets`

- **Why related**: Admin credentials stored in AWS Secrets Manager. Rotation state and last-accessed timestamps matter the moment an operator needs to reset or diagnose auth.
- **How discovered**: read `Cluster.MasterPasswordSecretArn`; cross-reference the already-loaded `secrets` list by ARN.
- **Count shown**: yes (0 or 1).

### `sg`

- **Why related**: VPC security groups attached to the cluster. Connection-refused problems from clients almost always trace back here.
- **How discovered**: read `Cluster.VpcSecurityGroups[].VpcSecurityGroupId`; cross-reference the already-loaded `sg` list by ID.
- **Count shown**: yes.

### `subnet`

- **Why related**: The subnets the cluster's leader/compute nodes run in. Subnet exhaustion or AZ-out-of-capacity shows up here and explains resize / provisioning failures.
- **How discovered**: read `Cluster.ClusterSubnetGroupName`; resolve subnet IDs via `DescribeClusterSubnetGroups(ClusterSubnetGroupName)` (one call per cluster subnet group, cached per session); cross-reference the already-loaded `subnet` list by subnet ID — a9s-devops: the subnet list is not embedded on the `Cluster` shape; this extra call is the AWS-documented resolution path.
- **Count shown**: yes.

### `vpc`

- **Why related**: The cluster's VPC — entry point to wider network context (flow logs, peerings, endpoints).
- **How discovered**: read `Cluster.VpcId`; cross-reference the already-loaded `vpc` list by ID.
- **Count shown**: yes (0 or 1).

### `ct-events`

- **Why related**: Universal audit pivot — `CreateCluster`, `ModifyCluster`, `DeleteCluster`, `RebootCluster`, IAM-role attach/detach, `ModifyClusterIamRoles`, parameter-group changes. The second place an operator looks after the alarm panel.
- **How discovered**: universal pivot — applies to every registered type; see `docs/related-resources.md` §Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `ClusterStatus==available` → Healthy.
  - **State bucket**: Healthy.
  - **How obtained**: `Cluster.ClusterStatus` on the `DescribeClusters` response.

- **Signal**: `ClusterStatus` in `creating` / `modifying` / `resizing` / `rebooting` / `renaming` / `deleting` → Warning (transitional).
  - **State bucket**: Warning.
  - **How obtained**: `Cluster.ClusterStatus` on the `DescribeClusters` response.

- **Signal**: `ClusterStatus` in `incompatible-hsm` / `incompatible-network` / `incompatible-parameters` / `incompatible-restore` / `hardware-failure` / `storage-full` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `Cluster.ClusterStatus` on the `DescribeClusters` response.

- **Signal**: `ClusterAvailabilityStatus` in `Unavailable` / `Failed` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `Cluster.ClusterAvailabilityStatus` on the `DescribeClusters` response.

- **Signal**: `ClusterAvailabilityStatus` in `Maintenance` / `Modifying` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `Cluster.ClusterAvailabilityStatus` on the `DescribeClusters` response.

- **Signal**: `PendingModifiedValues` non-empty → Warning.
  - **State bucket**: Warning.
  - **How obtained**: any non-nil sub-field of `Cluster.PendingModifiedValues` on the `DescribeClusters` response.

- **Signal**: `DeferredMaintenanceWindows[]` active (now ∈ [`DeferMaintenanceStartTime`, `DeferMaintenanceEndTime`]) → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `Cluster.DeferredMaintenanceWindows` on the `DescribeClusters` response.

- **Signal**: `PubliclyAccessible==true` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `Cluster.PubliclyAccessible` on the `DescribeClusters` response.

- **Signal**: `Encrypted==false` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `Cluster.Encrypted` on the `DescribeClusters` response.

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `PercentageDiskSpaceUsed`.
- OUT OF SCOPE: CloudWatch `HealthStatus`.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause. **Healthy rows render blank.** |
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
| `ClusterStatus` creating / modifying / resizing / rebooting / renaming / deleting | 1 | Warning | n/a | S2, S4 | `modifying` / `resizing` / `rebooting` / `renaming` / `creating` / `deleting` | `Cluster is <status>; queries may be intermittently unavailable.` |
| `ClusterStatus` incompatible-hsm / incompatible-network / incompatible-parameters / incompatible-restore | 1 | Broken | n/a | S2, S4 | `broken: incompatible-<hsm\|network\|parameters\|restore>` | `Cluster cannot start: <ClusterStatus>. Inspect parameter group / HSM / VPC settings.` |
| `ClusterStatus==hardware-failure` | 1 | Broken | n/a | S2, S4 | `broken: hardware-failure` | `Underlying hardware failed; AWS is recovering the cluster.` |
| `ClusterStatus==storage-full` | 1 | Broken | n/a | S2, S4 | `broken: storage-full` | `Cluster storage is full; writes and queries failing. Resize or free space.` |
| `ClusterAvailabilityStatus==Unavailable` | 1 | Broken | n/a | S2, S4 | `unavailable` | `Cluster is not available for queries (ClusterAvailabilityStatus=Unavailable).` |
| `ClusterAvailabilityStatus==Failed` | 1 | Broken | n/a | S2, S4 | `failed` | `Cluster has failed (ClusterAvailabilityStatus=Failed).` |
| `ClusterAvailabilityStatus==Maintenance` | 1 | Warning | n/a | S2, S4 | `maintenance` | `Cluster intermittently unavailable due to maintenance.` |
| `ClusterAvailabilityStatus==Modifying` | 1 | Warning | n/a | S2, S4 | `modifying` | `Cluster intermittently unavailable while modifications apply.` |
| `PendingModifiedValues` non-empty | 1 | Warning | n/a | S2, S4 | `pending change queued` | `Config change queued; will apply at next maintenance window.` |
| `DeferredMaintenanceWindows[]` active | 1 | Warning | n/a | S2, S4 | `maintenance deferred` | `Maintenance deferred; window active until <DeferMaintenanceEndTime>.` |
| `PubliclyAccessible==true` | 1 | Warning | n/a | S2, S4 | `publicly accessible` | `Cluster endpoint reachable from public internet; review SG and PubliclyAccessible flag.` |
| `Encrypted==false` | 1 | Warning | n/a | S2, S4 | `unencrypted at rest` | `Storage encryption is off (Encrypted=false). Not CIS-compliant.` |

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every non-healthy row carries a short cause in the Status column (`storage-full`, `unavailable`, `publicly accessible`, `pending change queued`, `maintenance deferred`) rather than a bare state keyword; operator can triage without opening detail, and the detail line adds one sentence of context rather than repeating the column.

## 5. Out of Scope

- All §3.3 Wave 3 signals (CloudWatch `PercentageDiskSpaceUsed`, `HealthStatus`).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- COPY / UNLOAD target buckets as `s3` pivots — not available on any read-only Redshift API. a9s-devops: not worth it, would require SQL statement history parsing or query-log scraping; the audit-log bucket (via `DescribeLoggingStatus`) covers the common operator workflow.
- Per-node health (`Cluster.ClusterNodes[]` role/status beyond the cluster roll-up). a9s-devops: not worth it, node-level failures roll up into `ClusterAvailabilityStatus` and `ClusterStatus==hardware-failure`; separate per-node surfaces would clutter without actionable value for a read-only tool.
- Transient `ClusterStatus` values not covered by the signals doc (`paused`, `final-snapshot`, `rotating-keys`, `updating-hsm`, `cancelling-resize`, `available, prep-for-resize`, `available, resize-cleanup`). a9s-devops: these are all transitional or admin-quiescent states; treat `paused` as Dim and the rest as Warning if encountered — not listed in §3 until the golden doc is extended.

## 6. Citations

- `Display name`, `List API`, shape of `ClusterStatus` / `ClusterAvailabilityStatus` / `PendingModifiedValues` / `DeferredMaintenanceWindows` / `PubliclyAccessible` / `Encrypted` — `docs/attention-signals.md` § Databases & Storage / `redshift` row.
- Wave 2 = `None`, Wave 3 = CloudWatch metrics — `docs/attention-signals.md` § Databases & Storage / `redshift` row.
- Related-panel target list (`alarm, cfn, ct-events, kms, logs, role, s3, secrets, sg, subnet, vpc`) — `docs/related-resources.md` § Per-type contract / `redshift` row; detail notes in `docs/related-resources.md` § `redshift`.
- `Cluster.KmsKeyId` — `AWS SDK Go v2 — service/redshift/types.Cluster § KmsKeyId`.
- `Cluster.IamRoles[].IamRoleArn` — `AWS SDK Go v2 — service/redshift/types.Cluster § IamRoles` + `types.ClusterIamRole § IamRoleArn`.
- `Cluster.VpcSecurityGroups[].VpcSecurityGroupId` — `AWS SDK Go v2 — service/redshift/types.Cluster § VpcSecurityGroups` + `types.VpcSecurityGroupMembership § VpcSecurityGroupId`.
- `Cluster.ClusterSubnetGroupName`, `Cluster.VpcId` — `AWS SDK Go v2 — service/redshift/types.Cluster § ClusterSubnetGroupName, VpcId`.
- `Cluster.MasterPasswordSecretArn` — `AWS SDK Go v2 — service/redshift/types.Cluster § MasterPasswordSecretArn`.
- `Cluster.Tags[]` carries `aws:cloudformation:stack-name` for CFN-managed clusters — a9s-devops (2026-04-20): possible=yes, worth=yes. AWS stamps this reserved tag on every CFN-managed resource; it is the standard pattern operators expect.
- `alarm` discovery via CloudWatch `Dimensions[].Name == "ClusterIdentifier"` — a9s-devops (2026-04-20): possible=yes, worth=yes. AWS/Redshift namespace uses `ClusterIdentifier` as the dimension key; matches the way operators write alarms.
- `logs` / `s3` discovery via `DescribeLoggingStatus` + known audit log-group naming pattern — `AWS SDK Go v2 — service/redshift.DescribeLoggingStatusOutput § BucketName, LogDestinationType, LogExports`; a9s-devops (2026-04-20): possible=yes, worth=yes. Audit log groups under `/aws/redshift/cluster/<ClusterIdentifier>/<logExport>` are AWS-documented; COPY/UNLOAD buckets are not statically discoverable so limit the panel to the audit bucket.
- `ClusterStatus` enum values — `AWS SDK Go v2 — service/redshift/types.Cluster § ClusterStatus` (enum listed in field doc-comment).
- `ClusterAvailabilityStatus` enum values (`Available`, `Unavailable`, `Maintenance`, `Modifying`, `Failed`) — `AWS SDK Go v2 — service/redshift/types.Cluster § ClusterAvailabilityStatus` (enum listed in field doc-comment).
- `DeferMaintenanceStartTime` / `DeferMaintenanceEndTime` for "active" window check — `AWS SDK Go v2 — service/redshift/types.DeferredMaintenanceWindow § DeferMaintenanceStartTime, DeferMaintenanceEndTime`.
- Read-only invariant (Out of Scope bullet) — `docs/architecture.md` § "What is a9s?" (line 15: "Read-only by design — a9s never makes write calls to AWS").
- §4 banned-word avoidance (`Wave`, `finding`, `bucket`, `severity` absent from all list/detail text) — per skill rules.
- `ct-events` as universal pivot — `docs/related-resources.md` § Policy.
