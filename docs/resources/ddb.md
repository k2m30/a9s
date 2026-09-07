---
shortName: ddb
name: DynamoDB Tables
awsApiRef: https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_TableDescription.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# ddb — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `ddb`
- **Display name**: DynamoDB Tables
- **AWS API reference**: <https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_TableDescription.html>
- **List API**: `ListTables` — returns table names only (no status, no size, no config).
- **Describe API (if any)**: `DescribeTable` per table (N+1) for state and config; `DescribeContinuousBackups` per table for PITR posture.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `alarm`, `backup`, `kinesis`, `kms`, `lambda`, `logs`, `vpce`, `ct-events`.

### `alarm`

- **Why related**: Throttle/error/ReadCapacity alarms — an on-call engineer triaging a throttled table immediately wants the alarms watching it.
- **How discovered**: Reverse-scan the already-loaded `alarm` list — match each alarm's `Dimensions[]` entry where `Name=="TableName"` and `Value==<table name>`. CloudWatch `AWS/DynamoDB` metrics are dimensioned by `TableName` (and optionally `GlobalSecondaryIndexName` / `Operation`). — a9s-devops: `TableName` is the canonical DDB dimension; reverse-scan is zero extra API calls and catches both table-level and per-operation alarms.
- **Count shown**: yes.

### `backup`

- **Why related**: AWS Backup recovery points — the restore surface when PITR isn't enough or the table has been deleted.
- **How discovered**: Reverse-scan the already-loaded `backup` list. For each plan, check whether the table's ARN is covered by `BackupSelection.Resources` (with AWS wildcard semantics — e.g. `arn:aws:dynamodb:*:*:table/*`) and not excluded by `BackupSelection.NotResources`. A plan covers this table iff any Resources entry matches AND no NotResources entry matches. — a9s-devops: Backup coverage lives on the plan's selection, not on the table; reverse-scan against the already-loaded `backup` list is the cheapest approach. AWS Backup's `ListRecoveryPointsByResource(ResourceArn=<table ARN>)` is a per-table Wave 2 call and is out of scope for the panel.
- **Count shown**: yes.

### `kinesis`

- **Why related**: Kinesis Data Streams destination for DDB change data — common pipeline pattern (DDB → KDS → Firehose/analytics) that operators want to follow from the table.
- **How discovered**: Call `DescribeKinesisStreamingDestination(TableName=<name>)` and read `KinesisDataStreamDestinations[].StreamArn`; look each ARN up in the already-loaded `kinesis` list. — a9s-devops: `DescribeKinesisStreamingDestination` is the canonical field; `TableDescription` itself does not expose KDS destinations. This is the only AWS surface that links a table to its KDS destinations.
- **Count shown**: yes.

### `kms`

- **Why related**: Customer-managed encryption key — required pivot when the table is using CMK SSE and the operator is checking rotation / access.
- **How discovered**: Read `TableDescription.SSEDescription.KMSMasterKeyArn` from the `DescribeTable` response and look the ARN up in the already-loaded `kms` list. When `SSEDescription` is nil or `KMSMasterKeyArn` is absent, the table uses the AWS-owned key — no `kms` entry to surface. — AWS SDK Go v2 — `dynamodb/types.SSEDescription § KMSMasterKeyArn`.
- **Count shown**: yes.

### `lambda`

- **Why related**: Lambdas consuming DDB Streams from this table — the write-side app tier an operator jumps to when tracing downstream effects of a table change.
- **How discovered**: Read `TableDescription.LatestStreamArn` and call `lambda:ListEventSourceMappings(EventSourceArn=<stream ARN>)`; resolve each mapping's `FunctionArn` against the already-loaded `lambda` list. — a9s-devops: DDB Streams is the canonical wiring; reverse-scanning every Lambda's event-source-mappings would work but is more expensive than one ListEventSourceMappings call scoped to this table's stream ARN. Tables without streams contribute zero Lambda pivots.
- **Count shown**: yes.

### `logs`

- **Why related**: ContributorInsights / Streams logs — the diagnostic tail when investigating hot keys or throttled partitions.
- **How discovered**: Cross-reference the already-loaded `logs` list by name prefix. ContributorInsights rules emit to log groups named `/aws/dynamodb/tables/<table-name>/*` (e.g. `.../insights/...`); match this table's `TableName` against the prefix segment. — a9s-devops: the DDB ContributorInsights log-group naming convention is stable and documented in the DynamoDB Developer Guide; prefix match is zero extra API calls. Export-to-S3 and Streams-to-Firehose pipelines have their own log groups named by the consumer, not the table, and are out of scope.
- **Count shown**: yes.

### `vpce`

- **Why related**: Gateway endpoint for DynamoDB — without it, VPC-bound callers hit the public endpoint (data-transfer cost, wrong IAM principal path). Operator debugging connectivity needs to see whether this region's VPCs have the endpoint.
- **How discovered**: Cross-reference the already-loaded `vpce` list — filter by `ServiceName == "com.amazonaws.<region>.dynamodb"` and `VpcEndpointType == "Gateway"`. The match is region-wide, not per-table (DDB endpoints are service-scoped), so the panel shows every DDB gateway endpoint in the region. — a9s-devops: the `com.amazonaws.<region>.dynamodb` service name is the canonical DDB gateway-endpoint identifier; there is no per-table endpoint binding on the AWS surface.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Audit trail for table schema/capacity changes — who resized capacity, who changed TTL, who deleted the table.
- **How discovered**: Universal pivot — applies to every registered type; see `related-resources.md` §Policy. `LookupEvents` filtered by `ResourceName==<table name>` or `ResourceType==AWS::DynamoDB::Table` with ARN match.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeTable](https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_DescribeTable.html)

Transcribed from `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `ddb`.

### 3.1 Wave 1 — zero extra API calls

The list API returns table names only, so the fetcher reads each table with `DescribeTable` before it builds the row. Every signal below is computed from that response as the row is built, with no second pass. Capacity, encryption and point-in-time recovery posture need `DescribeContinuousBackups` as well.

- **Signal**: `TableStatus == INACCESSIBLE_ENCRYPTION_CREDENTIALS`.
  - **State bucket**: Broken.
  - **API call**: `DescribeTable` — one call per table. Cause text available on `ArchivalSummary.ArchivalReason` once the 7-day timer expires; before that, only the status enum carries the signal.
  - **Cost shape**: per-resource.

- **Signal**: `TableStatus == ARCHIVED`.
  - **State bucket**: Broken.
  - **API call**: `DescribeTable` — one call per table. Cause available on `ArchivalSummary.ArchivalReason` (currently always `INACCESSIBLE_ENCRYPTION_CREDENTIALS`) and `ArchivalSummary.ArchivalDateTime`.
  - **Cost shape**: per-resource.

- **Signal**: `DeletionProtectionEnabled` not true.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `TableStatus == CREATING`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `TableStatus == UPDATING`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `TableStatus == DELETING`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `TableStatus == ARCHIVING`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `DescribeTable` was denied for this table.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `DescribeTable` answered with nothing usable.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal. `DescribeTable` and `DescribeContinuousBackups` run once per table, and every finding they raise is a row on `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `ddb`.

- **Signal**: PITR disabled (`PointInTimeRecoveryDescription.PointInTimeRecoveryStatus == DISABLED` on `DescribeContinuousBackups`).
  - **State bucket**: Warning (background finding — informational).
  - **API call**: `DescribeContinuousBackups` — one call per table.
  - **Cost shape**: per-resource.

- **Signal**: Resource policy names a foreign account.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: Resource policy allows any principal.
  - **State bucket**: Broken.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: no backup plan selection matches this table.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `ReadThrottleEvents` + `WriteThrottleEvents` (throttle rate per table).
- OUT OF SCOPE: CloudWatch `SystemErrors` (5xx error rate per table).

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `ddb`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `TableStatus == INACCESSIBLE_ENCRYPTION_CREDENTIALS` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `kms key inaccessible` |
| `TableStatus == ARCHIVED` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `archived: kms key lost` |
| `DeletionProtectionEnabled` not true | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `deletion protection off` |
| `TableStatus == CREATING` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `creating` |
| `TableStatus == UPDATING` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `updating` |
| `TableStatus == DELETING` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `deleting` |
| `TableStatus == ARCHIVING` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `archiving` |
| `DescribeTable` was denied for this table | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `details denied` |
| `DescribeTable` answered with nothing usable | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `details unavailable` |
| PITR disabled | 2 | Warning | `~` | S2, S3, S4, S5 | `point-in-time recovery disabled` |
| Resource policy names a foreign account | 2 | Warning | `~` | S2, S3, S4, S5 | `resource policy grants another account` |
| Resource policy allows any principal | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `resource policy open to anyone` |
| no backup plan selection matches this table | 2 | Warning | `~` | S2, S3, S4, S5 | `not covered by a backup plan` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`ACTIVE`, `ARCHIVED`) in the List text column is not acceptable without a cause. `archiving` and `archived` are paired with their cause above.
- For signals that legitimately have no operator-actionable cause (pure Healthy), the row is omitted from this table; §3 still describes it.
- Keep the List text short enough to fit: ≤ 40 chars. The Detail cell quotes the finding's Detail constant verbatim, however long it is.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes: every non-healthy DynamoDB row pairs its state with a cause operators recognise (`kms key inaccessible`, `archived: kms key lost`), and a table with no point-in-time recovery goes yellow reading `point-in-time recovery disabled`, which is the whole of it.

## 5. Out of Scope

- All §3.3 Wave 3 signals (CloudWatch throttling and error-rate metrics).
- Per-table CloudWatch metric aggregation (`ReadThrottleEvents`, `WriteThrottleEvents`, `SystemErrors`) — out-of-scope for Wave 2 budget.
- Global table replica health beyond the base table's `TableStatus` — `Replicas[].ReplicaStatus` on multi-region tables would be a useful future Wave 2 signal but is not in `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `ddb`. — a9s-devops: possible=yes (`TableDescription.Replicas[].ReplicaStatus`), worth=yes for multi-region operators but intentionally deferred; recorded here so it isn't lost.
- DynamoDB Streams consumer-lag metrics (CloudWatch only).
- TTL misconfiguration (`DescribeTimeToLive` is a separate per-table call; not currently in the attention contract).
- `backup` discovery via `ListRecoveryPointsByResource` (per-table call, exceeds Wave 2 budget for the panel).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- a9s golden doc — `ddb` related targets are `alarm, backup, ct-events, kinesis, kms, lambda, logs, vpce` — `docs/related-resources.md` § Per-type contract row `ddb` and § `ddb` subsection.
- a9s golden doc — `ct-events` is a universal pivot — `docs/related-resources.md` § Policy bullet 4.
- a9s golden doc — the `ddb` signals — `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `ddb`; the deferred CloudWatch metrics — `docs/attention-signals.md § Not yet implemented`.
- AWS Go SDK v2 — `TableStatus` enum values and field on `TableDescription` — `AWS SDK Go v2 — dynamodb/types.TableDescription § TableStatus` (`CREATING`/`UPDATING`/`DELETING`/`ACTIVE`/`INACCESSIBLE_ENCRYPTION_CREDENTIALS`/`ARCHIVING`/`ARCHIVED`).
- AWS Go SDK v2 — `SSEDescription.KMSMasterKeyArn` carries the CMK ARN for `kms` pivot — `AWS SDK Go v2 — dynamodb/types.SSEDescription § KMSMasterKeyArn`.
- AWS Go SDK v2 — `ArchivalSummary.ArchivalReason` + `ArchivalDateTime` provide S5 cause text for the archived state — `AWS SDK Go v2 — dynamodb/types.ArchivalSummary § ArchivalReason, ArchivalDateTime`.
- AWS Go SDK v2 — PITR signal field — `AWS SDK Go v2 — dynamodb/types.PointInTimeRecoveryDescription § PointInTimeRecoveryStatus` (`ENABLED`/`DISABLED`).
- AWS Go SDK v2 — `ContinuousBackupsDescription` is the `DescribeContinuousBackups` response — `AWS SDK Go v2 — dynamodb/types.ContinuousBackupsDescription § PointInTimeRecoveryDescription`.
- a9s-devops consultation — `alarm` discovery via CloudWatch `Dimensions[].Name=="TableName"` — a9s-devops persona (2026-04-20): possible=yes, worth=yes. `TableName` is the canonical DDB CloudWatch dimension; reverse-scan of the already-loaded alarm list is zero extra API calls.
- a9s-devops consultation — `backup` discovery via `GetBackupSelection.Resources[]` reverse-scan with wildcard + NotResources matching — a9s-devops persona (2026-04-20): possible=yes, worth=yes. Backup coverage lives on the plan's selection, not on the table; plans may use wildcard ARNs (e.g. `arn:aws:dynamodb:*:*:table/*`) and NotResources exclusions. The table itself has no field pointing at its backup plans.
- a9s-devops consultation — `kinesis` discovery via `DescribeKinesisStreamingDestination` — a9s-devops persona (2026-04-20): possible=yes, worth=yes. This is the only AWS API linking a table to its KDS destinations; `TableDescription` does not expose them.
- a9s-devops consultation — `lambda` discovery via `ListEventSourceMappings(EventSourceArn=<stream ARN>)` — a9s-devops persona (2026-04-20): possible=yes, worth=yes. DDB Streams → Lambda is the canonical wiring; scoping the ESM call by the table's `LatestStreamArn` is cheaper than reverse-scanning every Lambda.
- a9s-devops consultation — `logs` discovery via `/aws/dynamodb/tables/<name>/` name-prefix match — a9s-devops persona (2026-04-20): possible=yes, worth=yes. ContributorInsights log-group naming is a stable convention; prefix match is zero extra API calls.
- a9s-devops consultation — `vpce` discovery via `ServiceName == com.amazonaws.<region>.dynamodb` — a9s-devops persona (2026-04-20): possible=yes, worth=yes. DDB gateway-endpoint service name is region-scoped and canonical; per-table endpoint binding does not exist on the AWS surface.
- a9s-devops consultation — global-table `Replicas[].ReplicaStatus` deferred — a9s-devops persona (2026-04-20): possible=yes, worth=yes but not in today's attention contract; recorded in §5 Out of Scope so it isn't lost.
- a9s golden doc — a9s is read-only — `docs/architecture.md` § "a9s is a read-only terminal UI for AWS".

<!-- BEGIN GENERATED: header -->
ddb — DATABASES & STORAGE. Lifecycle key: `status`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| ddb.broken.kms\_key\_inaccessible | kms key inaccessible | broken | wave1 | — |
| ddb.broken.archived\_kms\_lost | archived: kms key lost | broken | wave1 | — |
| ddb.warn.creating | creating | warn | wave1 | — |
| ddb.warn.updating | updating | warn | wave1 | — |
| ddb.warn.deleting | deleting | warn | wave1 | — |
| ddb.warn.archiving | archiving | warn | wave1 | — |
| ddb.pitr-off | point-in-time recovery disabled | warn | wave2 | — |
| ddb.deletion-protection-off | deletion protection off | warn | wave1 | A single delete call (DeleteTable) destroys this table and its data. Turn on deletion protection so removing it takes a deliberate second step. |
| ddb.cross-account-policy | resource policy grants another account | warn | wave2 | The table's resource policy grants access to an AWS account outside this one. Confirm each account belongs to a partner you meant to share with, and remove the rest. |
| ddb.public-policy | resource policy open to anyone | broken | wave2 | The table's resource policy allows any AWS principal, so anyone with an AWS account can reach it. Replace the wildcard principal with the specific roles that need access. |
| ddb.not-in-backup-plan | not covered by a backup plan | warn | wave2 | No backup plan selects this table, so nothing is scheduled to copy it and point-in-time recovery alone will not survive the table being deleted. Add it to a plan by ARN, or give it a tag one of your plans already selects on. |
| ddb.warn.details\_denied | details denied | warn | wave1 | Access to resource details was denied; only the name is visible. |
| ddb.warn.details\_unavailable | details unavailable | warn | wave1 | Details could not be retrieved; only the name is visible. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| kms | KMS Key | no |
| alarm | CloudWatch Alarms | yes |
| lambda | Lambda Functions | no |
| kinesis | Kinesis Streams | no |
| backup | Backup Plans | yes |
| logs | Log Groups | yes |
| vpce | VPC Endpoints | yes |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
