---
shortName: redis
name: ElastiCache Redis
awsApiRef: https://docs.aws.amazon.com/AmazonElastiCache/latest/APIReference/API_ReplicationGroup.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# redis — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `redis`
- **Display name**: ElastiCache Redis
- **AWS API reference**: https://docs.aws.amazon.com/AmazonElastiCache/latest/APIReference/API_ReplicationGroup.html
- **List API**: `DescribeReplicationGroups`
- **Describe API (if any)**: not used (Wave 2 is `None` for `redis`)

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `cfn`, `ct-events`, `kms`, `logs`, `secrets`, `sg`, `sns`, `subnet`, `vpc`.

### `alarm`

- **Why related**: CloudWatch alarms fire on a failing Redis replication group during incidents — operator wants to see which thresholds tripped (cache hit rate, eviction, replication lag) before deciding whether this is a capacity issue or a data-plane issue.
- **How discovered**: cross-reference the already-loaded `alarm` list by `Dimensions.CacheClusterId` matching any member in `ReplicationGroup.MemberClusters[]`, or by a `ReplicationGroupId` dimension where the alarm is scoped to the group directly — a9s-devops: attached alarms are not a field on the replication group; the reverse lookup on the loaded alarm list is how operators find them in the Console.
- **Count shown**: yes.

### `cfn`

- **Why related**: when Redis is provisioned by IaC, the operator's first question after a degraded row is "whose stack owns this?" so the ownership pivot lands them on the change-control surface.
- **How discovered**: look up the `aws:cloudformation:stack-name` tag on the replication group via `ListTagsForResource` — a9s-devops: `ReplicationGroup` has no stack field; the AWS-managed tag is the only reliable IaC-ownership pivot, and `ListTagsForResource` is a single bounded extra call per resource of interest (or skipped if tags are already in the loaded sweep).
- **Count shown**: yes.

### `ct-events`

- **Why related**: universal pivot — applies to every registered type; see related-resources.md §Policy. Audit trail for group changes (MODIFY, FAILOVER, DELETE).
- **How discovered**: `LookupEvents` filtered by `ResourceName=ReplicationGroupId` or by `EventSource=elasticache.amazonaws.com`.
- **Count shown**: yes.

### `kms`

- **Why related**: at-rest encryption key — if KMS key is pending deletion or disabled, the replication group can't read its data; this pivot lets the operator confirm key state on a Broken row.
- **How discovered**: read field `ReplicationGroup.KmsKeyId` (AWS SDK Go v2 — `elasticache/types.ReplicationGroup` § `KmsKeyId`), then cross-reference the already-loaded `kms` list by `KeyId`/`KeyArn`.
- **Count shown**: yes.

### `logs`

- **Why related**: ElastiCache can ship slow-log and engine-log streams to CloudWatch Logs; when latency misbehaves, the operator wants direct link to the log group.
- **How discovered**: read field `ReplicationGroup.LogDeliveryConfigurations[].DestinationDetails.CloudWatchLogsDetails.LogGroup` (AWS SDK Go v2 — `elasticache/types.LogDeliveryConfiguration` § `DestinationDetails`), then cross-reference the loaded `logs` list.
- **Count shown**: yes.

### `secrets`

- **Why related**: when AUTH is enabled, the operator's AUTH token is stored in Secrets Manager (rotation pivot). Finding the backing secret lets them copy/rotate without leaving a9s.
- **How discovered**: no AWS field on `ReplicationGroup` links a secret directly; `ReplicationGroup.AuthTokenEnabled` is a boolean. Discovery relies on tag or naming convention — e.g. a secret tagged `elasticache:replication-group-id=<id>` or named `<id>/auth-token`. Operators who follow a rotation convention find the secret via Secrets Manager tag search — a9s-devops: possible=yes via tag-based cross-reference of the loaded `secrets` list, worth=yes because AUTH rotation is a real workflow; the count is best-effort and may be zero when no convention is followed.
- **Count shown**: yes (best-effort; zero when no tag/naming match).

### `sg`

- **Why related**: "why can't my app reach Redis?" always ends at a security-group rule. This pivot is the first stop during a connectivity incident.
- **How discovered**: `SecurityGroups[]` is **not** a field on `ReplicationGroup`; it lives on each member `CacheCluster` (AWS SDK Go v2 — `elasticache/types.CacheCluster` § `SecurityGroups`). Resolve by calling `DescribeCacheClusters(CacheClusterId=<MemberClusters[0]>)` and reading `SecurityGroups[].SecurityGroupId`, then cross-reference the loaded `sg` list — a9s-devops: one Describe call on any one member is sufficient because all members of a replication group share the same SG set.
- **Count shown**: yes.

### `sns`

- **Why related**: ElastiCache publishes lifecycle events (failover, node-replacement) to an SNS topic; that topic is where the on-call pager listens.
- **How discovered**: `NotificationConfiguration.TopicArn` is **not** on `ReplicationGroup`; it is on `CacheCluster` (AWS SDK Go v2 — `elasticache/types.CacheCluster` § `NotificationConfiguration`). Resolve by the same `DescribeCacheClusters` call used for `sg`; cross-reference the loaded `sns` list by topic ARN — a9s-devops: possible=yes via the member-cluster Describe.
- **Count shown**: yes.

### `subnet`

- **Why related**: the subnet group controls which AZs Redis can live in; a disappearing subnet breaks replacement-node placement.
- **How discovered**: `CacheSubnetGroupName` is on `CacheCluster`, not `ReplicationGroup` — call `DescribeCacheClusters` on one member, then `DescribeCacheSubnetGroups(CacheSubnetGroupName=<name>)` to read `CacheSubnetGroup.Subnets[].SubnetIdentifier` (AWS SDK Go v2 — `elasticache/types.CacheSubnetGroup` § `Subnets`). Cross-reference the loaded `subnet` list — a9s-devops: two bounded extra calls (one `DescribeCacheClusters`, one `DescribeCacheSubnetGroups`) resolve the full subnet set for the group.
- **Count shown**: yes.

### `vpc`

- **Why related**: identifies the VPC the group lives in — the root of every connectivity diagnostic.
- **How discovered**: same call chain as `subnet`; read `CacheSubnetGroup.VpcId` (AWS SDK Go v2 — `elasticache/types.CacheSubnetGroup` § `VpcId`) and cross-reference the loaded `vpc` list.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `ReplicationGroup.Status == "available"`.
  - **State bucket**: Healthy.
  - **How obtained**: list-response field `Status` on `DescribeReplicationGroups`.
- **Signal**: `ReplicationGroup.Status in ("creating", "modifying", "deleting", "snapshotting")`.
  - **State bucket**: Warning.
  - **How obtained**: list-response field `Status` on `DescribeReplicationGroups`.
- **Signal**: `ReplicationGroup.Status == "create-failed"`.
  - **State bucket**: Broken.
  - **How obtained**: list-response field `Status` on `DescribeReplicationGroups`.
- **Signal**: `AutomaticFailover != "enabled"` on a multi-AZ replication group.
  - **State bucket**: Warning.
  - **How obtained**: list-response fields `AutomaticFailover` and `MultiAZ` on `DescribeReplicationGroups` (multi-AZ detected via `MultiAZ == "enabled"` per `elasticache/types.MultiAZStatus`).

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `DatabaseMemoryUsagePercentage`.
- OUT OF SCOPE: CloudWatch `Evictions`.
- OUT OF SCOPE: CloudWatch `ReplicationLag`.
- OUT OF SCOPE: CloudWatch `EngineCPUUtilization`.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause. **Healthy rows render blank** — no `OK` / `available` / `ACTIVE` / `running`. |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping applied below. `Status == "available"` with `AutomaticFailover == "enabled"` (Healthy) does not produce a §4 row — the row is green and S4 blank, silence is the UX.

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `Status == creating` | 1 | Warning | n/a | S2, S4 | `creating — new group` | `Replication group is being created; nodes are not yet serving traffic.` |
| `Status == modifying` | 1 | Warning | n/a | S2, S4 | `modifying — config change` | `Replication group is applying a configuration change; failover or latency spikes possible.` |
| `Status == snapshotting` | 1 | Warning | n/a | S2, S4 | `snapshotting — backup running` | `Replication group is taking a backup; performance may dip until it completes.` |
| `Status == deleting` | 1 | Warning | n/a | S2, S4 | `deleting — teardown` | `Replication group is being deleted; endpoints will stop accepting connections.` |
| `Status == create-failed` | 1 | Broken | n/a | S2, S4 | `create failed — see events` | `Replication group create failed; AWS did not surface a cause field — check CloudTrail events for the failure reason.` |
| `AutomaticFailover != enabled` on multi-AZ | 1 | Warning | n/a | S2, S4 | `multi-AZ without auto-failover` | `Replication group is deployed multi-AZ but automatic failover is not enabled; a primary-node loss will require manual intervention.` |

Notes for fillers:

- No `bare state keyword` appears in S4 alone; every row pairs the state with a short reason the operator cares about.
- The `create-failed` S4 text explicitly points to events because `ReplicationGroup` carries no `FailureMessage` or `FailureCode` field — a9s-devops confirmed the cause text is only available via CloudTrail for Redis create failures.
- The `AutomaticFailover != enabled` row only applies when `MultiAZ == "enabled"` — single-AZ groups are not expected to have automatic failover and do not produce this finding.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? All Warning rows carry an explicit cause in the Status column and are self-explanatory; the only partial exception is `create failed — see events`, which directs the operator to CloudTrail because AWS does not expose a cause field on `ReplicationGroup`. The operator knows the row is broken and knows where to look next without opening detail — acceptable as designed.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- Per-node-group (shard) status. `NodeGroup.Status` exists (`elasticache/types.NodeGroup` § `Status`) but a9s lists replication groups, not individual shards — a9s-devops: shard-level detail belongs in the detail view's shard panel, not in list-level attention, because it would force shard-aware fan-out the Wave 1 budget does not allow.
- In-transit / at-rest encryption posture (`TransitEncryptionEnabled`, `AtRestEncryptionEnabled`) — a9s-devops: possible=yes, worth=no at list level; encryption posture is audit-scope rather than incident-scope, and surfacing it as a Warning would noisy-up every legacy group without driving action. Best left to a future audit view.
- AUTH / user-group posture (`AuthTokenEnabled`, `UserGroupIds`) — a9s-devops: possible=yes, worth=no at list level; same reasoning as encryption posture.

## 6. Citations

- Contract URL and expected related targets — `docs/related-resources.md` § Per-type contract row `redis`.
- Per-target reasoning (alarm, cfn, ct-events, kms, logs, secrets, sg, sns, subnet, vpc) — `docs/related-resources.md` § `redis`.
- Wave 1 signals and state buckets — `docs/attention-signals.md` § Databases & Storage row `redis`.
- Wave 3 out-of-scope metrics — `docs/attention-signals.md` § Databases & Storage row `redis` column "Wave 3".
- List API `DescribeReplicationGroups` — `docs/attention-signals.md` § Databases & Storage row `redis` column "Source".
- `ReplicationGroup.Status` enum values (`available`, `creating`, `modifying`, `deleting`, `snapshotting`, `create-failed`) — AWS SDK Go v2 — `elasticache/types.ReplicationGroup` § `Status`.
- `ReplicationGroup.KmsKeyId` — AWS SDK Go v2 — `elasticache/types.ReplicationGroup` § `KmsKeyId`.
- `ReplicationGroup.LogDeliveryConfigurations[]` — AWS SDK Go v2 — `elasticache/types.ReplicationGroup` § `LogDeliveryConfigurations`; `LogDeliveryConfiguration.DestinationDetails` — AWS SDK Go v2 — `elasticache/types.LogDeliveryConfiguration` § `DestinationDetails`.
- `ReplicationGroup.MemberClusters` — AWS SDK Go v2 — `elasticache/types.ReplicationGroup` § `MemberClusters`.
- `ReplicationGroup.AutomaticFailover` enum (`enabled`, `enabling`, `disabled`, `disabling`) — AWS SDK Go v2 — `elasticache/types.AutomaticFailoverStatus`.
- `ReplicationGroup.MultiAZ` enum (`enabled`, `disabled`) — AWS SDK Go v2 — `elasticache/types.MultiAZStatus`.
- `CacheCluster.SecurityGroups[]` — AWS SDK Go v2 — `elasticache/types.CacheCluster` § `SecurityGroups`.
- `CacheCluster.NotificationConfiguration` — AWS SDK Go v2 — `elasticache/types.CacheCluster` § `NotificationConfiguration`.
- `CacheCluster.CacheSubnetGroupName` — AWS SDK Go v2 — `elasticache/types.CacheCluster` § `CacheSubnetGroupName`.
- `CacheSubnetGroup.Subnets[].SubnetIdentifier` — AWS SDK Go v2 — `elasticache/types.CacheSubnetGroup` § `Subnets`.
- `CacheSubnetGroup.VpcId` — AWS SDK Go v2 — `elasticache/types.CacheSubnetGroup` § `VpcId`.
- Read-only invariant — `docs/architecture.md` § "What is a9s?" ("a9s is a read-only terminal UI for AWS … Every AWS API call is a List, Describe, or Get operation").
- `alarm` discovery — a9s-devops (2026-04-20): possible=yes, worth=yes. No field on `ReplicationGroup` lists attached alarms; discovery is a reverse lookup on the loaded `alarm` list by `Dimensions.CacheClusterId` matching `ReplicationGroup.MemberClusters[]`, same pivot the Console uses on the replication-group detail page.
- `cfn` discovery — a9s-devops (2026-04-20): possible=yes, worth=yes. `ReplicationGroup` has no stack field; the `aws:cloudformation:stack-name` tag is the only reliable IaC-ownership pivot, resolved via `ListTagsForResource`.
- `secrets` discovery — a9s-devops (2026-04-20): possible=partial, worth=yes. No AWS field links a replication group to a Secrets Manager ARN; tag-based or naming-convention cross-reference on the loaded `secrets` list is the operator's real-world pivot during AUTH rotation; best-effort, may return zero.
- `sg` / `sns` discovery via member Describe — a9s-devops (2026-04-20): possible=yes, worth=yes. Both fields live on `CacheCluster`, not `ReplicationGroup`; one `DescribeCacheClusters` call on any member resolves both because all members share the SG set and notification topic.
- `subnet` / `vpc` discovery via subnet-group chain — a9s-devops (2026-04-20): possible=yes, worth=yes. Two bounded Describe calls (one `DescribeCacheClusters`, one `DescribeCacheSubnetGroups`) resolve the full subnet set and parent VPC.
- `create-failed` cause text unavailable on `ReplicationGroup` — a9s-devops (2026-04-20): possible=no, worth=yes. AWS does not expose a `FailureMessage`/`FailureCode` field on the replication-group response; the cause is only in CloudTrail events; S4 directs the operator there.
- Encryption / AUTH posture out of scope at list level — a9s-devops (2026-04-20): possible=yes, worth=no. Audit-scope not incident-scope; surfacing as Warning would noisy-up legacy groups without driving action.
- Shard-level status out of scope at list level — a9s-devops (2026-04-20): possible=yes, worth=no at list level. `NodeGroup.Status` exists but belongs in a per-replication-group shard panel, not on the list row.
