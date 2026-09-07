---
shortName: msk
name: MSK Clusters
awsApiRef: https://docs.aws.amazon.com/msk/1.0/apireference/v1-clusters.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# msk — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `msk`
- **Display name**: MSK Clusters
- **AWS API reference**: <https://docs.aws.amazon.com/msk/1.0/apireference/v1-clusters.html>
- **List API**: `ListClustersV2` — returns `Cluster[]`. The SDK confirms `ClusterArn`, `ClusterName`, `State`, `StateInfo`, `ClusterType`, `Provisioned`, `Serverless`, `CurrentVersion`, and `CreationTime` are all on the list shape, so the Wave 1 state signal is reachable with zero extra calls. The full broker configuration (`BrokerNodeGroupInfo.SecurityGroups`, `BrokerNodeGroupInfo.ClientSubnets`, `EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId`, `LoggingInfo.BrokerLogs`, `ClientAuthentication`) is nested under `Provisioned` (or `Serverless`) on the same response — related-panel discovery requires no extra API call per cluster.
- **Describe API (if any)**: not used for attention (Wave 2 is `None`). `DescribeClusterV2` returns the same `Cluster` shape and is not needed for signals. `ListScramSecrets` per cluster is required to resolve the `secrets` related target when SASL/SCRAM is enabled — see §2 `secrets` discovery.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `cfn`, `ct-events`, `kms`, `lambda`, `logs`, `s3`, `secrets`, `sg`, `subnet`, `vpc`.

### `alarm`

- **Why related**: CloudWatch alarms watching broker-level metrics (`ActiveControllerCount`, `OfflinePartitionsCount`, `UnderReplicatedPartitions`, `KafkaDataLogsDiskUsed`) for this cluster.
- **How discovered**: cross-reference the already-loaded `alarm` list — keep alarms whose `Namespace == "AWS/Kafka"` and whose `Dimensions` include `{Name: "Cluster Name", Value: <ClusterName>}` — a9s-devops: MSK emits metrics under the `AWS/Kafka` namespace dimensioned by cluster name; matching on loaded alarms avoids a per-cluster `DescribeAlarmsForMetric` call.
- **Count shown**: yes.

### `cfn`

- **Why related**: CloudFormation stack that created the cluster — operators pivot here to see the source-of-truth template and drift status.
- **How discovered**: read the `aws:cloudformation:stack-name` / `aws:cloudformation:stack-id` tag on `Cluster.Tags`; cross-reference the already-loaded `cfn` list by stack name — a9s-devops: CloudFormation propagates these tags to every managed resource; tag-based cross-reference is the standard pivot used across every CFN-managed type and needs no extra API call.
- **Count shown**: yes (typically 1 or 0).

### `kms`

- **Why related**: Customer-managed KMS key used to encrypt the broker data volumes at rest. If this key is disabled, in `PendingDeletion`, or rotated off, the cluster loses the ability to read/write storage.
- **How discovered**: read `Cluster.Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId` (ARN); cross-reference the already-loaded `kms` list by key ARN / key ID.
- **Count shown**: yes (typically 1).

### `lambda`

- **Why related**: Lambda functions consuming from MSK topics via event source mappings — the consumer side of the pipe. An operator diagnosing "messages piling up" pivots from the cluster to the consumer Lambdas.
- **How discovered**: reverse pivot — iterate the already-loaded `lambda` list and keep functions whose `EventSourceMappings[].EventSourceArn == Cluster.ClusterArn` (MSK ESM references the cluster ARN directly). If the lambda list has not loaded its event-source mappings yet, call `ListEventSourceMappings(EventSourceArn=<ClusterArn>)` once per cluster — a9s-devops: MSK does not publish its consumers in its own API surface; the consumer-to-source relationship lives on Lambda's side, so the reverse walk is the only correct pivot.
- **Count shown**: yes.

### `logs`

- **Why related**: CloudWatch Logs group receiving broker logs. When the cluster is emitting errors, the operator jumps straight to the log group.
- **How discovered**: read `Cluster.Provisioned.LoggingInfo.BrokerLogs.CloudWatchLogs.LogGroup`; cross-reference the loaded `logs` list by log-group name. Only populated when CloudWatch log delivery is enabled for the cluster.
- **Count shown**: yes (0 or 1).

### `s3`

- **Why related**: S3 bucket receiving broker-log archive deliveries. Operator pivots here for long-retention log analysis.
- **How discovered**: read `Cluster.Provisioned.LoggingInfo.BrokerLogs.S3.Bucket`; cross-reference the loaded `s3` list by bucket name. Only populated when S3 log delivery is enabled.
- **Count shown**: yes (0 or 1).

### `secrets`

- **Why related**: Secrets Manager secrets holding SASL/SCRAM client credentials associated with the cluster. If a secret is deleted or its KMS key is broken, SCRAM clients can no longer authenticate.
- **How discovered**: the `Cluster.Provisioned.ClientAuthentication.Sasl.Scram.Enabled` flag only indicates SCRAM is turned on — it does not carry the secret ARNs. When `Enabled == true`, call `ListScramSecrets(ClusterArn=<arn>)` once per cluster; the response `SecretArnList []string` is the association. Cross-reference the loaded `secrets` list by secret ARN — a9s-devops: there is no way to read MSK-associated SCRAM secrets without `ListScramSecrets`; the call is one-per-cluster only when SCRAM is enabled, so cost is bounded and worth paying for the pivot.
- **Count shown**: yes.

### `sg`

- **Why related**: Security groups attached to the broker elastic network interfaces — the firewall in front of the Kafka ports. Misconfigured SGs are the single most common cause of "can't connect to the cluster".
- **How discovered**: read `Cluster.Provisioned.BrokerNodeGroupInfo.SecurityGroups []string`; cross-reference the loaded `sg` list by group ID.
- **Count shown**: yes.

### `subnet`

- **Why related**: Client subnets where the broker ENIs live — one per AZ. Subnet AZ distribution determines broker AZ placement.
- **How discovered**: read `Cluster.Provisioned.BrokerNodeGroupInfo.ClientSubnets []string`; cross-reference the loaded `subnet` list by subnet ID.
- **Count shown**: yes (typically 2 or 3).

### `vpc`

- **Why related**: The VPC hosting the cluster. Operators pivot here to see peerings, route tables, and the overall network context.
- **How discovered**: the MSK `Cluster` response does not carry a direct `VpcId` field — derive it by reading the first subnet ID from `BrokerNodeGroupInfo.ClientSubnets`, look it up in the already-loaded `subnet` list, and follow `Subnet.VpcId` — a9s-devops: MSK places all brokers in one VPC (subnets are required to share a VPC), so any client subnet's VPC is the cluster's VPC; cross-referencing via the loaded subnet list avoids a dedicated `DescribeSubnets` call.
- **Count shown**: yes (1).

### `ct-events`

- **Why related**: Universal pivot — who created, updated, deleted, or rebooted this cluster; who rotated SCRAM secrets; when the last `UpdateBrokerStorage` happened.
- **How discovered**: pre-built CloudTrail query scoped to `ClusterArn` as the resource identifier.
- **Count shown**: unknown (CloudTrail queries are windowed; a reliable total isn't available without a separate count call).
- Universal pivot — applies to every registered type; see `related-resources.md` §Policy.

## 3. Attention / Issues Algorithm

**Source API**: [ListClustersV2](https://docs.aws.amazon.com/msk/1.0/apireference/v2-clusters.html)

Transcribed from `docs/attention-signals.md § Signals § MESSAGING` row `msk`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `State == CREATING`.
  - **State bucket**: Warning.
  - **How obtained**: `Cluster.State` from `ListClustersV2`.

- **Signal**: `State == UPDATING`.
  - **State bucket**: Warning.
  - **How obtained**: `Cluster.State` from `ListClustersV2`.

- **Signal**: `State == MAINTENANCE`.
  - **State bucket**: Warning.
  - **How obtained**: `Cluster.State` from `ListClustersV2`.

- **Signal**: `State == REBOOTING_BROKER`.
  - **State bucket**: Warning.
  - **How obtained**: `Cluster.State` from `ListClustersV2`.

- **Signal**: `State == HEALING`.
  - **State bucket**: Warning.
  - **How obtained**: `Cluster.State` from `ListClustersV2`. `HEALING` buckets with the other transient-warning states because cluster capacity is degraded while AWS replaces the broker — `docs/attention-signals.md § Signals § MESSAGING` row `msk`.

- **Signal**: `State == DELETING`.
  - **State bucket**: Warning.
  - **How obtained**: `Cluster.State` from `ListClustersV2`.

- **Signal**: `State == FAILED`.
  - **State bucket**: Broken.
  - **How obtained**: `Cluster.State` from `ListClustersV2`; the cause text for S4/S5 is `Cluster.StateInfo.Code` and `Cluster.StateInfo.Message` (both optional strings on the same list response).

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal. Each runs on the type's bounded second pass, after the rows are on screen.

Per-broker runtime state is not on any read-only AWS action: `ListNodes` returns node metadata but no `RUNNING` enum. Deeper broker-level health belongs to CloudWatch and is deferred (see §3.3).

- **Signal**: `PublicAccess.Type` publishes the brokers.
  - **State bucket**: Broken.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: `ClientAuthentication.Unauthenticated.Enabled`.
  - **State bucket**: Broken.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: the cluster runs a broker version behind the newest AWS offers.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: client-broker encryption allows plaintext.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `ActiveControllerCount` (Kafka controller health).
- OUT OF SCOPE: CloudWatch `OfflinePartitionsCount` (partitions currently unavailable).
- OUT OF SCOPE: CloudWatch `UnderReplicatedPartitions` (replication lag / broker-replacement symptom).
- OUT OF SCOPE: CloudWatch `KafkaDataLogsDiskUsed` (broker disk-fill warning).

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count + list frame title `!N` suffix | Aggregated count of `!`-severity findings. `~` findings do not bump. The list frame title appends a space-separated `!N` after the count parentheses when the current list has N > 0 issues (`s3(50+) !5`, `ec2(17) !1`), or `!N+` when N is a truncated lower bound; N uses the same aggregation as the menu badge; the generated note under this table says which waves feed it for this type. No suffix when N = 0, and omitted in attention-only mode (`ctrl+z`) — the filtered count already is the issue count, so `name(5 of 50+) [!]` stays as-is. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause. **Healthy rows render blank.** |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. |

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `msk`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit).
- **Wave 1 Warning / Broken / Dim** → S2 + S4.
- **Wave 2 finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4, S5. (No Wave 2 for msk.)
- **Wave 2 finding on a Healthy row, informational** → `~` glyph on green row. S3, S4, S5. No S1. (No Wave 2 for msk.)
- **Wave 2 finding on an already yellow/red/dim row** → S3 suppressed, S4 deduplicates with existing cause, S5 carries the full sentence, S1 still counts if `!`. (No Wave 2 for msk.)

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `State == CREATING` | 1 | Warning | n/a | S2, S4 | `creating` |
| `State == UPDATING` | 1 | Warning | n/a | S2, S4 | `updating` |
| `State == MAINTENANCE` | 1 | Warning | n/a | S2, S4 | `maintenance` |
| `State == REBOOTING_BROKER` | 1 | Warning | n/a | S2, S4 | `rebooting broker` |
| `State == HEALING` | 1 | Warning | n/a | S2, S4 | `healing` |
| `State == DELETING` | 1 | Warning | n/a | S2, S4 | `deleting` |
| `State == FAILED` | 1 | Broken | n/a | S2, S4 | `failed` |
| `PublicAccess.Type` publishes the brokers | 2 | Broken | `!` | S1, S3, S4, S5 | `brokers reachable from the internet` |
| `ClientAuthentication.Unauthenticated.Enabled` | 2 | Broken | `!` | S1, S3, S4, S5 | `unauthenticated access allowed` |
| the cluster runs a broker version behind the newest AWS offers | 2 | Warning | `~` | S3, S4, S5 | `broker software outdated` |
| client-broker encryption allows plaintext | 2 | Warning | `~` | S3, S4, S5 | `encryption in transit not enforced` |

Notes:

- `ACTIVE` is Healthy and has no §4 row by design — S4 renders blank, no glyph.
- For `FAILED`, the cause text is always `StateInfo.Code` (compact) in S4 and `StateInfo.Message` (sentence) in S5. When `StateInfo` is nil (can happen on very old clusters that failed before the field existed), fall back to `failed` in S4 and a generic detail line; this should be rare.
- For `UPDATING`, `StateInfo.Code` often carries the update reason (e.g. `UPDATING_CONFIGURATION`, `UPDATING_BROKER_STORAGE`) — surface it when present; fall back to the generic wording above.
- There is no Wave 2, so no `!` / `~` glyphs appear on MSK rows under the current contract. If per-broker runtime state becomes available via a future AWS API, new §3.2 signals could introduce them.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for every state: the yellow/red/dim row carries a specific cause in S4 (`updating: UPDATING_BROKER_STORAGE`, `rebooting broker`, `healing broker`, `failed: <code>`), and the most dangerous state — `FAILED` — surfaces the `StateInfo.Code` inline so the operator can tell "bad AMI" from "subnet gone" without navigating. The only residual gap is that deeper correctness (partition offline, controller dead, disk 90%) lives in CloudWatch and is intentionally Wave 3 — operators wanting that signal use the `alarm` pivot from the related panel, which is the designed flow.

## 5. Out of Scope

- All §3.3 Wave 3 signals (CloudWatch broker-level metrics).
- Per-broker runtime state — no read-only AWS API returns it today.
- Any UI element not listed in §4 — no new columns, icons, views, or key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- msk related-panel targets `alarm`, `cfn`, `ct-events`, `kms`, `lambda`, `logs`, `s3`, `secrets`, `sg`, `subnet`, `vpc` — `docs/related-resources.md` § Per-type contract, row `msk`.
- Per-target field citations (`EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId`, `LoggingInfo.BrokerLogs.CloudWatchLogs`, `LoggingInfo.BrokerLogs.S3`, `ClientAuthentication.Sasl.Scram`, `BrokerNodeGroupInfo.SecurityGroups`, `BrokerNodeGroupInfo.ClientSubnets`, `BrokerNodeGroupInfo.ClientVpcIpAddresses → VPC`) — `docs/related-resources.md` § `msk`.
- msk Wave 1 signal set (`State` enum mapping, `FAILED`→Broken, `DELETING`→Dim) — `docs/attention-signals.md § Signals § MESSAGING` row `msk`.
- msk has no Wave 2 signals; per-broker runtime state not exposed by read-only APIs — `docs/attention-signals.md § Signals § MESSAGING` row `msk`.
- msk Wave 3 CloudWatch metrics (`ActiveControllerCount`, `OfflinePartitionsCount`, `UnderReplicatedPartitions`, `KafkaDataLogsDiskUsed`) — `docs/attention-signals.md § Not yet implemented`.
- `State`, `StateInfo`, `ClusterArn`, `ClusterName`, `ClusterType`, `Provisioned`, `Serverless`, `CreationTime`, `CurrentVersion`, `Tags` present on the list response — `AWS SDK Go v2 — service/kafka/types.Cluster § State, StateInfo, ClusterArn, ClusterName, ClusterType, Provisioned, Serverless, CreationTime, CurrentVersion, Tags`.
- `StateInfo.Code` / `StateInfo.Message` are the cause fields for S4 / S5 — `AWS SDK Go v2 — service/kafka/types.StateInfo § Code, Message`.
- `ClusterState` enum values include `ACTIVE, CREATING, DELETING, FAILED, HEALING, MAINTENANCE, REBOOTING_BROKER, UPDATING` — `AWS SDK Go v2 — service/kafka/types.ClusterState § const values`.
- `HEALING` is defined in the SDK and buckets Warning — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. HEALING is MSK's auto-broker-replacement; cluster is degraded-but-serving, which matches Warning semantics used for REBOOTING_BROKER and MAINTENANCE.`
- `BrokerNodeGroupInfo.SecurityGroups`, `BrokerNodeGroupInfo.ClientSubnets` are on `Provisioned.BrokerNodeGroupInfo` — `AWS SDK Go v2 — service/kafka/types.BrokerNodeGroupInfo § SecurityGroups, ClientSubnets`.
- `EncryptionAtRest.DataVolumeKMSKeyId` carries the KMS ARN — `AWS SDK Go v2 — service/kafka/types.EncryptionAtRest § DataVolumeKMSKeyId`.
- `LoggingInfo.BrokerLogs.CloudWatchLogs` / `LoggingInfo.BrokerLogs.S3` present — `AWS SDK Go v2 — service/kafka/types.BrokerLogs § CloudWatchLogs, S3`.
- `ClientAuthentication.Sasl.Scram.Enabled` is a `*bool`; SCRAM secret ARNs are NOT on the describe response — `AWS SDK Go v2 — service/kafka/types.Scram § Enabled`.
- `secrets` discovery uses `ListScramSecrets(ClusterArn)` once per cluster when SCRAM is enabled, not derivable from describe alone — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. The describe shape only advertises SCRAM is on; the secret association lives on ListScramSecrets. Call is bounded (one per SCRAM-enabled cluster) and the pivot is high-value for "why can't my client auth" triage.`
- `alarm` discovery via `AWS/Kafka` namespace cross-reference — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. MSK publishes metrics under AWS/Kafka dimensioned by cluster name; matching loaded alarms avoids a per-cluster API call.`
- `cfn` discovery via `aws:cloudformation:stack-name` tag — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. Stack tag propagation is universal across CFN-managed resources; standard pivot.`
- `lambda` reverse pivot via `EventSourceMappings[].EventSourceArn == ClusterArn` — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. MSK does not list its consumers; the consumer-to-source relationship exists only on the Lambda side. ListEventSourceMappings(EventSourceArn=ClusterArn) is the correct scoped call when lambda list has not pre-loaded ESMs.`
- `vpc` derivation via first subnet lookup — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. MSK clusters share one VPC across all broker subnets; any ClientSubnet's VpcId is the cluster's VPC. Avoids a dedicated DescribeSubnets call.`
- `ct-events` is the universal pivot applied to every registered type — `docs/related-resources.md` § Policy.
- a9s is read-only — `docs/architecture.md` § "What is a9s?".
- Superseded HOW ignored — row middle-dot `·` marker, `⚠ Background Check` detail header, and derived list-level banner in `docs/historical/analysis/enrichment-visibility.md` are not cited or reproduced per the skill's S1–S5 rules.

<!-- BEGIN GENERATED: header -->
msk — MESSAGING. Lifecycle key: `state`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| msk.warn.creating | creating | warn | wave1 | — |
| msk.warn.updating | updating | warn | wave1 | — |
| msk.warn.maintenance | maintenance | warn | wave1 | — |
| msk.warn.rebooting\_broker | rebooting broker | warn | wave1 | — |
| msk.warn.healing | healing | warn | wave1 | — |
| msk.warn.deleting | deleting | warn | wave1 | — |
| msk.broken.failed | failed | broken | wave1 | — |
| msk.broker-outdated | broker software outdated | warn | wave2 | — |
| msk.encryption-not-tls | encryption in transit not enforced | warn | wave2 | — |
| msk.public-access | brokers reachable from the internet | broken | wave2 | Kafka brokers are published to the internet with their own public addresses, so the cluster is reachable from anywhere its security groups allow rather than only from inside the VPC. Turn public access off and reach the brokers from within the VPC or over a peered network. |
| msk.unauthenticated | unauthenticated access allowed | broken | wave2 | The cluster accepts Kafka clients that present no credentials at all, so anyone who can reach a broker can read and write every topic. Turn unauthenticated access off and require one of the cluster's authentication methods. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| alarm | CW Alarms | yes |
| sg | Security Groups | no |
| kms | KMS Key | no |
| lambda | Lambda Functions | no |
| cfn | CloudFormation | yes |
| subnet | Subnets | no |
| vpc | VPC | no |
| logs | Log Groups | no |
| s3 | S3 (broker logs) | no |
| secrets | Secrets Manager | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
