---
shortName: redshift
name: Redshift Clusters
awsApiRef: https://docs.aws.amazon.com/redshift/latest/APIReference/API_Cluster.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# redshift — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `redshift`
- **Display name**: Redshift Clusters
- **AWS API reference**: <https://docs.aws.amazon.com/redshift/latest/APIReference/API_Cluster.html>
- **List API**: `DescribeClusters` (returns fully-populated `Cluster` shapes in one paged call; no per-resource Describe needed for Wave 1 signals).
- **Describe API (if any)**: not used — Wave 2 is `None` per `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `redshift`. `DescribeLoggingStatus` is only invoked to resolve the `logs` / `s3` related-panel pivots, not for attention signals.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `alarm`, `cfn`, `kms`, `logs`, `role`, `s3`, `secrets`, `sg`, `subnet`, `vpc`, `ct-events`.

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

**Source API**: [DescribeClusters](https://docs.aws.amazon.com/redshift/latest/APIReference/API_DescribeClusters.html)

Transcribed from `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `redshift`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `ClusterStatus == incompatible-parameters` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `Cluster.ClusterStatus` on the `DescribeClusters` response.

- **Signal**: `ClusterStatus==hardware-failure`.
  - **State bucket**: Broken.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `ClusterStatus==storage-full`.
  - **State bucket**: Broken.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `ClusterAvailabilityStatus` in `Unavailable` / `Failed` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `Cluster.ClusterAvailabilityStatus` on the `DescribeClusters` response.

- **Signal**: `ClusterAvailabilityStatus==Failed`.
  - **State bucket**: Broken.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `ClusterStatus == modifying` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `ClusterStatus` on the `DescribeClusters` list response.

- **Signal**: `ClusterAvailabilityStatus == Maintenance` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `Cluster.ClusterAvailabilityStatus` on the `DescribeClusters` response.

- **Signal**: `ClusterAvailabilityStatus==Modifying`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

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

- **Signal**: `ClusterStatus == incompatible-hsm`.
  - **State bucket**: Broken.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `ClusterStatus == incompatible-network`.
  - **State bucket**: Broken.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `ClusterStatus == incompatible-restore`.
  - **State bucket**: Broken.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `ClusterStatus == creating`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `ClusterStatus == resizing`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `ClusterStatus == rebooting`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `ClusterStatus == renaming`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `ClusterStatus == deleting`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal. Each runs on the type's bounded second pass, after the rows are on screen.

- **Signal**: `DescribeLoggingStatus.LoggingEnabled` not true.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: Parameter group `require_ssl` not `true`.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `PercentageDiskSpaceUsed`.
- OUT OF SCOPE: CloudWatch `HealthStatus`.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `redshift`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `ClusterStatus == incompatible-parameters` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `broken: incompatible-parameters` |
| `ClusterStatus==hardware-failure` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `broken: hardware-failure` |
| `ClusterStatus==storage-full` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `broken: storage-full` |
| `ClusterAvailabilityStatus==Unavailable` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `unavailable` |
| `ClusterAvailabilityStatus==Failed` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `failed` |
| `ClusterAvailabilityStatus==Maintenance` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `maintenance` |
| `ClusterStatus == modifying` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `modifying — cluster settings` |
| `ClusterAvailabilityStatus==Modifying` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `modifying — availability affected` |
| `PendingModifiedValues` non-empty | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `pending change queued` |
| `DeferredMaintenanceWindows[]` active | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `maintenance deferred` |
| `PubliclyAccessible==true` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `public endpoint` |
| `Encrypted==false` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `unencrypted at rest` |
| `ClusterStatus == incompatible-hsm` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `broken: incompatible-hsm` |
| `ClusterStatus == incompatible-network` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `broken: incompatible-network` |
| `ClusterStatus == incompatible-restore` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `broken: incompatible-restore` |
| `ClusterStatus == creating` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `creating` |
| `ClusterStatus == resizing` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `resizing` |
| `ClusterStatus == rebooting` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `rebooting` |
| `ClusterStatus == renaming` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `renaming` |
| `ClusterStatus == deleting` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `deleting` |
| `DescribeLoggingStatus.LoggingEnabled` not true | 2 | Warning | `~` | S2, S3, S4, S5 | `audit logging off` |
| Parameter group `require_ssl` not `true` | 2 | Warning | `~` | S2, S3, S4, S5 | `SSL not required` |

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every non-healthy row carries a short cause in the Status column (`broken: storage-full`, `unavailable`, `public endpoint`, `pending change queued`, `maintenance deferred`) rather than a bare state keyword; operator can triage without opening detail, and the detail line adds one sentence of context rather than repeating the column.

## 5. Out of Scope

- All §3.3 Wave 3 signals (CloudWatch `PercentageDiskSpaceUsed`, `HealthStatus`).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- COPY / UNLOAD target buckets as `s3` pivots — not available on any read-only Redshift API. a9s-devops: not worth it, would require SQL statement history parsing or query-log scraping; the audit-log bucket (via `DescribeLoggingStatus`) covers the common operator workflow.
- Per-node health (`Cluster.ClusterNodes[]` role/status beyond the cluster roll-up). a9s-devops: not worth it, node-level failures roll up into `ClusterAvailabilityStatus` and `ClusterStatus==hardware-failure`; separate per-node surfaces would clutter without actionable value for a read-only tool.
- Transient `ClusterStatus` values not covered by the signals doc (`paused`, `final-snapshot`, `rotating-keys`, `updating-hsm`, `cancelling-resize`, `available, prep-for-resize`, `available, resize-cleanup`). a9s-devops: these are all transitional or admin-quiescent states; treat `paused` as Dim and the rest as Warning if encountered — not listed in §3 until the golden doc is extended.

## 6. Citations

- Display name and the `ClusterStatus` / `ClusterAvailabilityStatus` / `PendingModifiedValues` / `DeferredMaintenanceWindows` / `PubliclyAccessible` / `Encrypted` signals — `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `redshift`; list API — `core/aws/redshift.go`.
- The deferred CloudWatch metrics — `docs/attention-signals.md § Not yet implemented`.
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

<!-- BEGIN GENERATED: header -->
redshift — DATABASES & STORAGE. Status key: `status` — the key the status cell reads, and the column naming it is the status column.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| redshift.broken.incompatible\_hsm | broken: incompatible-hsm | broken | wave1 | The cluster cannot reach the hardware security module holding its encryption key, so it will not come up. Check the client certificate for that module and the network path to it, then restore the connection. |
| redshift.broken.incompatible\_network | broken: incompatible-network | broken | wave1 | The cluster's subnet group no longer provides what it needs — free addresses, or the Availability Zone it was created in — so it cannot start. Fix the subnet group, then restore the cluster. |
| redshift.broken.incompatible\_parameters | broken: incompatible-parameters | broken | wave1 | A value in this cluster's parameter group is rejected, so the cluster will not come up with it applied. Correct the parameter group and reboot the cluster. |
| redshift.broken.incompatible\_restore | broken: incompatible-restore | broken | wave1 | The restore from snapshot failed, so this cluster holds no usable data. Check the snapshot's node type and encryption against the target, then restore again. |
| redshift.broken.hardware\_failure | broken: hardware-failure | broken | wave1 | A node's underlying hardware failed. Redshift replaces the node itself, but the cluster is degraded or unavailable until it does; watch the events and restore from the latest snapshot if it does not recover. |
| redshift.broken.storage\_full | broken: storage-full | broken | wave1 | The cluster has no disk left, so queries that need to spill fail and loads are rejected. Delete or unload cold tables, vacuum to reclaim space, then resize to more storage. |
| redshift.broken.unavailable | unavailable | broken | wave1 | The cluster is not answering queries, so every dashboard and job behind it is failing. Check the cluster events for the cause, and its most recent snapshot, before deciding between waiting and restoring. |
| redshift.broken.failed | failed | broken | wave1 | The cluster is in a failed state and will not serve queries again in place. Restore the most recent snapshot into a new cluster and repoint the applications at it. |
| redshift.warn.creating | creating | warn | wave1 | The cluster is still being provisioned and cannot take connections yet. Wait for it to become available before loading data or pointing tools at the endpoint. |
| redshift.warn.modifying | modifying — cluster settings | warn | wave1 | A configuration change is being applied to the cluster, and it may reboot or run with reduced capacity before it settles. Wait for it to finish before starting another change or judging query times. |
| redshift.warn.resizing | resizing | warn | wave1 | The cluster is changing node count or node type; depending on the resize type it is read-only or unavailable for part of it. Hold off on loads until it is done, and expect query plans to change afterwards. |
| redshift.warn.rebooting | rebooting | warn | wave1 | The cluster is restarting, so open connections are dropped and queries in flight are lost. Wait for it to come back; applications should reconnect on their own. |
| redshift.warn.renaming | renaming | warn | wave1 | The cluster identifier is changing, which changes its endpoint address, so anything holding the old name stops connecting. Update the connection strings and any DNS record pointing at the old endpoint. |
| redshift.warn.deleting | deleting | warn | wave1 | The cluster is being removed, and unless a final snapshot was requested its data goes with it. If this was not intended, check now whether a snapshot exists to restore from. |
| redshift.warn.maintenance | maintenance | warn | wave1 | The cluster is in its maintenance window and AWS is applying updates, so it can be briefly unavailable. Nothing to do beyond expecting the interruption; move the window if it clashes with your load schedule. |
| redshift.warn.availability\_modifying | modifying — availability affected | warn | wave1 | Redshift reports the cluster's availability as changing rather than steady, so queries may be refused or slow while the change lands. Wait for it to report available again before treating a query failure as an application fault. |
| redshift.warn.pending\_change | pending change queued | warn | wave1 | A modification is queued for the next maintenance window, so the cluster's live configuration is not the one shown as desired. Check what is pending and, if it needs an outage, apply it at a time you choose. |
| redshift.warn.maintenance\_deferred | maintenance deferred | warn | wave1 | Maintenance on this cluster has been postponed, so it runs without updates AWS has scheduled, and the deferral has an end date. Plan a window before that date, or AWS will pick one for you. |
| redshift.warn.publicly\_accessible | public endpoint | warn | wave1 | The cluster answers on a routable address outside the VPC, so only its security groups separate the warehouse from the internet. Turn public access off and reach it over a private link unless an outside system genuinely needs it. |
| redshift.warn.unencrypted\_at\_rest | unencrypted at rest | warn | wave1 | The cluster's blocks and its snapshots are stored unencrypted. Turning encryption on requires a cluster migration, so plan it into a maintenance window rather than leaving it indefinitely. |
| redshift.audit-logging-off | audit logging off | warn | wave2 | Nothing records connections and queries against this cluster, so an incident leaves no trail to follow. Enable audit logging to an S3 bucket or a CloudWatch log group. |
| redshift.require-ssl-off | SSL not required | warn | wave2 | The cluster accepts unencrypted client connections, so credentials and query results can be read off the wire. Set the parameter group's require-SSL parameter (require_ssl) to true and reboot. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| alarm | CW Alarms | yes |
| sg | Security Groups | no |
| vpc | VPC | no |
| role | IAM Role | no |
| kms | KMS Key | no |
| cfn | CloudFormation | yes |
| secrets | Secrets Manager | yes |
| logs | Log Groups | no |
| s3 | S3 Buckets | no |
| subnet | Subnets | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
