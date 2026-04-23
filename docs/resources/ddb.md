---
shortName: ddb
name: DynamoDB Tables
awsApiRef: https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_TableDescription.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# ddb — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `ddb`
- **Display name**: DynamoDB Tables
- **AWS API reference**: https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_TableDescription.html
- **List API**: `ListTables` — returns table names only (no status, no size, no config).
- **Describe API (if any)**: `DescribeTable` per table (N+1) for state and config; `DescribeContinuousBackups` per table for PITR posture.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `backup`, `kinesis`, `kms`, `lambda`, `logs`, `vpce`, `ct-events`.

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

Transcribed from `docs/attention-signals.md` row `ddb | DynamoDB Tables`.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — the list API does not return fields usable for attention. `ListTables` returns table names only; state, capacity, encryption, and PITR posture all require `DescribeTable` / `DescribeContinuousBackups`.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal. Per `attention-signals.md`: "`DescribeTable` per table (N+1): `TableStatus`: `ACTIVE`→Healthy; `CREATING`/`UPDATING`/`DELETING`/`ARCHIVING`→Warning; `INACCESSIBLE_ENCRYPTION_CREDENTIALS`/`ARCHIVED`→Broken. Plus `DescribeContinuousBackups` per table: PITR disabled → Warning".

- **Signal**: `TableStatus == ACTIVE`.
  - **State bucket**: Healthy.
  - **API call**: `DescribeTable` — one call per table.
  - **Cost shape**: per-resource.

- **Signal**: `TableStatus` in `CREATING` / `UPDATING` / `DELETING` / `ARCHIVING`.
  - **State bucket**: Warning.
  - **API call**: `DescribeTable` — one call per table.
  - **Cost shape**: per-resource.

- **Signal**: `TableStatus == INACCESSIBLE_ENCRYPTION_CREDENTIALS`.
  - **State bucket**: Broken.
  - **API call**: `DescribeTable` — one call per table. Cause text available on `ArchivalSummary.ArchivalReason` once the 7-day timer expires; before that, only the status enum carries the signal.
  - **Cost shape**: per-resource.

- **Signal**: `TableStatus == ARCHIVED`.
  - **State bucket**: Broken.
  - **API call**: `DescribeTable` — one call per table. Cause available on `ArchivalSummary.ArchivalReason` (currently always `INACCESSIBLE_ENCRYPTION_CREDENTIALS`) and `ArchivalSummary.ArchivalDateTime`.
  - **Cost shape**: per-resource.

- **Signal**: PITR disabled (`PointInTimeRecoveryDescription.PointInTimeRecoveryStatus == DISABLED` on `DescribeContinuousBackups`).
  - **State bucket**: Warning (background finding — informational).
  - **API call**: `DescribeContinuousBackups` — one call per table.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `ReadThrottleEvents` + `WriteThrottleEvents` (throttle rate per table).
- OUT OF SCOPE: CloudWatch `SystemErrors` (5xx error rate per table).

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing" — e.g. maintenance scheduled, PITR off. `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `archived: kms key inaccessible 7d+`, `PITR off`). **Healthy rows render blank** — no `OK` / `ACTIVE`. Empty means "nothing to see." |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). Not applicable here — no Wave 1 signals.
- **Wave 1 Warning / Broken / Dim** → S2 + S4. Not applicable here.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4, S5.
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph on green row. S3, S4, S5. No S1.
- **Wave 2 finding on an already yellow/red/dim row** → redundant with color; S3 suppressed, S4 deduplicates with existing cause, S5 still carries the full sentence, S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `TableStatus == ACTIVE` (healthy) | 2 | Healthy | n/a | — (omitted) | *(blank)* | *(none)* |
| `TableStatus` transitional (`CREATING`/`UPDATING`/`DELETING`/`ARCHIVING`) | 2 | Warning | n/a | S2 + S4 | `creating` / `updating` / `deleting` / `archiving` | `Table is <status>; wait for ACTIVE before routing traffic.` |
| `TableStatus == INACCESSIBLE_ENCRYPTION_CREDENTIALS` | 2 | Broken | n/a | S2 + S4 + S5 | `kms key inaccessible` | `KMS key inaccessible — table archives in 7d if not restored.` |
| `TableStatus == ARCHIVED` | 2 | Broken | n/a | S2 + S4 + S5 | `archived: kms key lost` | `Archived due to inaccessible KMS key; on-demand backup kept at ArchivalBackupArn.` |
| PITR disabled | 2 | Healthy (with `~` background finding) | `~` | S3 + S4 + S5 | `PITR off` | `Point-in-time recovery is disabled — 35-day rollback window unavailable.` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`ACTIVE`, `ARCHIVED`) in the List text column is not acceptable without a cause. `archiving` and `archived` are paired with their cause above.
- For signals that legitimately have no operator-actionable cause (pure Healthy), the row is omitted from this table; §3 still describes it.
- Keep both columns short enough to fit: List text ≤ 40 chars, Detail text ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes: every non-healthy DynamoDB row pairs its state with a cause operators recognise (`kms key inaccessible`, `archived: kms key lost`, `PITR off`), and the `~` PITR annotation tells them at a glance "green but PITR is off" without needing detail view.

## 5. Out of Scope

- All §3.3 Wave 3 signals (CloudWatch throttling and error-rate metrics).
- Per-table CloudWatch metric aggregation (`ReadThrottleEvents`, `WriteThrottleEvents`, `SystemErrors`) — out-of-scope for Wave 2 budget.
- Global table replica health beyond the base table's `TableStatus` — `Replicas[].ReplicaStatus` on multi-region tables would be a useful future Wave 2 signal but is not in `attention-signals.md` today. — a9s-devops: possible=yes (`TableDescription.Replicas[].ReplicaStatus`), worth=yes for multi-region operators but intentionally deferred; recorded here so it isn't lost.
- DynamoDB Streams consumer-lag metrics (CloudWatch only).
- TTL misconfiguration (`DescribeTimeToLive` is a separate per-table call; not currently in the attention contract).
- `backup` discovery via `ListRecoveryPointsByResource` (per-table call, exceeds Wave 2 budget for the panel).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- a9s golden doc — `ddb` related targets are `alarm, backup, ct-events, kinesis, kms, lambda, logs, vpce` — `docs/related-resources.md` § Per-type contract row `ddb` and § `ddb` subsection.
- a9s golden doc — `ct-events` is a universal pivot — `docs/related-resources.md` § Policy bullet 4.
- a9s golden doc — Wave 1/Wave 2/Wave 3 signals for `ddb` — `docs/attention-signals.md` § Databases & Storage row `ddb`.
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
