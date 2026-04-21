---
shortName: logs
name: CloudWatch Log Groups
awsApiRef: https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_LogGroup.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# logs — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `logs`
- **Display name**: CloudWatch Log Groups
- **AWS API reference**: https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_LogGroup.html
- **List API**: `DescribeLogGroups` (per group returns `logGroupName`, `creationTime`, `retentionInDays`, `metricFilterCount`, `arn`, `storedBytes`, `kmsKeyId`, `dataProtectionStatus`, `logGroupClass`; no event-level data).
- **Describe API (if any)**: `DescribeLogStreams` (per log group, ordered by `LastEventTime` descending, `limit=1`) — used in Wave 2 to read `lastEventTimestamp` for silent-service detection.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `apigw`, `ecs-task`, `kinesis`, `kms`, `lambda`, `s3`, `ct-events`.

### `alarm`

- **Why related**: Metric-filter-driven alarms fire off patterns in this log group — when the operator is diagnosing a noisy log group, the very next question is which alarms are watching it. Cited in `related-resources.md` §`logs` → "Metric-filter-driven alarms."
- **How discovered**: Call `DescribeMetricFilters(logGroupName=…)` to list the filters attached to this log group, read each filter's `metricTransformations[].metricName` + `metricNamespace`, then reverse-scan the already-loaded `alarm` list matching on `MetricName` + `Namespace`. — a9s-devops: the log group ↔ alarm linkage is indirect (log group → metric filter → CloudWatch metric → alarm); no direct field bridges them. possible=yes (DescribeMetricFilters is read-only, per-group), worth=yes (this is the canonical "what alerts if this log group goes bad?" workflow).
- **Count shown**: yes.

### `apigw`

- **Why related**: API Gateway writes execution and access logs to CloudWatch Logs — the operator reading a request failure wants to hop from the log group to the API definition. Cited in `related-resources.md` §`logs` → "APIGW access logs."
- **How discovered**: Match the log group `logGroupName` against the API Gateway naming conventions — `API-Gateway-Execution-Logs_<apiId>/<stage>` (REST v1), `/aws/apigateway/welcome`, `/aws/http-api/<apiId>` (HTTP v2), or a user-chosen access-log destination — and cross-reference the already-loaded `apigw` list by `apiId`. — a9s-devops: naming convention is the only stable link for execution logs; user-chosen access-log groups need tag/stage-config walk that is Wave 2+ and out of scope here. possible=yes (naming convention + reverse-scan), worth=yes (apigw troubleshooting starts in logs).
- **Count shown**: yes.

### `ecs-task`

- **Why related**: ECS tasks using the `awslogs` driver write stdout/stderr into this log group — the operator reading an error line wants the task that produced it. Cited in `related-resources.md` §`logs` → "awslogs driver log groups."
- **How discovered**: Reverse-scan the already-loaded `ecs-task` list — each task's `containers[]` carries a reference to its task definition's `containerDefinitions[].logConfiguration.logDriver=awslogs` and `options.awslogs-group` value; match that value against this log group's `logGroupName`. Requires the task-definition body to be available in the loaded task record; if not, pivot is best-effort. — a9s-devops: `awslogs-group` on the task-definition container is the authoritative field, but it's not on the `ListTasks` shape; enrichment may be needed. possible=yes (via loaded task-def), worth=yes (log → owning task is the core incident pivot).
- **Count shown**: yes.

### `kinesis`

- **Why related**: Subscription filters fan log events out to Kinesis Data Streams or Firehose — understanding where a log group's data is being consumed downstream matters for pipeline debugging. Cited in `related-resources.md` §`logs` → "Subscription filter → Kinesis/Firehose."
- **How discovered**: Call `DescribeSubscriptionFilters(logGroupName=…)` per log group, read each filter's `destinationArn`; when the ARN is `arn:aws:kinesis:…:stream/<name>`, match against the already-loaded `kinesis` list by stream name. Firehose destinations are a different service and surface in a dedicated pivot if registered. — a9s-devops: subscription filters are the canonical fan-out mechanism and the only read path from log group to stream. possible=yes, worth=yes (required for "where does this log data end up?").
- **Count shown**: yes.

### `kms`

- **Why related**: When the log group is encrypted with a customer-managed KMS key, any KMS key disable / pending-deletion will silently block log ingestion — the operator reading "no new events" needs the key one key press away. Cited in `related-resources.md` §`logs` → "LogGroup.KmsKeyId."
- **How discovered**: Read `LogGroup.kmsKeyId` directly from the list response (`AWS SDK Go v2 — cloudwatchlogs/types.LogGroup § KmsKeyId`); if non-empty, cross-reference the already-loaded `kms` list by key ARN.
- **Count shown**: yes.

### `lambda`

- **Why related**: Lambda functions write to log groups named `/aws/lambda/<function-name>` — the single most common operator pivot is "whose function logs am I looking at?" Subscription-filter consumers (Lambda target of a filter) are a second, rarer case. Cited in `related-resources.md` §`logs` → "Lambdas whose logs land here OR subscription-filter consumers."
- **How discovered**: (a) Match `logGroupName` against the `/aws/lambda/<name>` convention and cross-reference the already-loaded `lambda` list by function name; (b) call `DescribeSubscriptionFilters(logGroupName=…)` and cross-reference filters whose `destinationArn` is `arn:aws:lambda:…:function:<name>`. — a9s-devops: the naming convention is stable and unambiguous for function-owned log groups; the subscription-filter case is additive. possible=yes, worth=yes (this is the #1 Lambda debugging pivot).
- **Count shown**: yes.

### `s3`

- **Why related**: Export tasks archive a log group's events into an S3 bucket for long-term retention or downstream analytics — operator auditing archival posture or investigating export failures pivots here. Cited in `related-resources.md` §`logs` → "Export tasks to S3."
- **How discovered**: Call `DescribeExportTasks` and filter by `logGroupName`, then cross-reference each task's `destination` bucket name against the already-loaded `s3` list. — a9s-devops: DescribeExportTasks is the only read-only surface that links a log group to an S3 archive target; there is no reverse field on the bucket. possible=yes (per-group call, bounded), worth=yes (archive posture is a compliance/cost workflow).
- **Count shown**: yes.

### `ct-events`

- **Why related**: Universal pivot — applies to every registered type; see related-resources.md §Policy. Audit trail for log group configuration changes (create, delete, put-retention-policy, associate-kms-key, put-subscription-filter).
- **How discovered**: `LookupEvents` with `LookupAttributes=[{AttributeKey:ResourceName, AttributeValue:<logGroupName>}]` in the current region.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `retentionInDays` is nil → Warning (Never Expire = cost drift).
  - **State bucket**: Warning.
  - **How obtained**: `DescribeLogGroups` list response — field `retentionInDays` (`AWS SDK Go v2 — cloudwatchlogs/types.LogGroup § RetentionInDays`, `*int32`; nil means "retain forever").

- **Signal**: `storedBytes==0 && creationTime<now()-90d` → Warning (orphan).
  - **State bucket**: Warning.
  - **How obtained**: `DescribeLogGroups` list response — fields `storedBytes` and `creationTime` (`types.LogGroup § StoredBytes`, `§ CreationTime`).

- **Signal**: Cross-ref `kms` — referenced `kmsKeyId` is in `PendingDeletion` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: Read `kmsKeyId` from the list response and cross-reference the already-loaded `kms` sibling-list by key ARN; if that key's `KeyState==PendingDeletion`, log ingestion will fail.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: `lastEventTimestamp` stale beyond expected write cadence → Warning (silent service).
  - **State bucket**: Warning.
  - **API call**: `DescribeLogStreams(logGroupName=…, orderBy=LastEventTime, descending=true, limit=1)` — one per log group.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: Metric-filter-count check.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing." **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause. **Healthy rows render blank.** |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph → S1, S3, S4, S5.
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph → S3, S4, S5. No S1.
- **Wave 2 finding on an already yellow/red/dim row** → S3 suppressed; S4 deduplicates; S5 still carries the full sentence; S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `retentionInDays` nil | 1 | Warning | n/a | S2, S4 | `retention: never expire` | `No retention policy set — events kept forever, billed indefinitely.` |
| `storedBytes==0` + age >90d | 1 | Warning | n/a | S2, S4 | `orphan: 0 bytes, age 90d+` | `No data ever written; log group created 90+ days ago and is unused.` |
| KMS key `PendingDeletion` | 1 | Broken | n/a | S2, S4 | `kms key pending deletion` | `Encryption key scheduled for deletion — ingestion will fail when it goes.` |
| `lastEventTimestamp` stale | 2 | Warning | `~` | S3, S4, S5 | `last event 3d ago` | `No new events since <timestamp> — emitter may be stopped or misconfigured.` |

Rules for filling list and detail text:

- Banned words: `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword in the List text column is not acceptable — pair with cause.
- Keep both columns short: List ≤ 40 chars, Detail ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for all three Wave 1 signals (retention, orphan, KMS deletion) — the S4 text names both the condition and its consequence; for the Wave 2 `lastEventTimestamp` staleness, the `~` glyph + `last event 3d ago` in S4 answers the question inline, with S5 carrying the exact timestamp for follow-up. No §4 gap identified.

## 5. Out of Scope

- All §3.3 Wave 3 signals (metric-filter-count check — `DescribeMetricFilters` per log group, cost-bounded but not currently surfaced).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- List API and list-response fields — `docs/attention-signals.md` § "Monitoring" table row `logs`; also `AWS SDK Go v2 — cloudwatchlogs/types.LogGroup § LogGroupName, CreationTime, RetentionInDays, StoredBytes, KmsKeyId, MetricFilterCount`.
- Describe API for Wave 2 — `docs/attention-signals.md` § "Monitoring" table row `logs`; also `AWS SDK Go v2 — cloudwatchlogs/types.LogStream § LastEventTimestamp`.
- Related targets (full list) — `docs/related-resources.md` § Per-type contract, row `logs`; detailed reasoning in `docs/related-resources.md` § `logs`.
- `alarm` via metric-filter bridge — `docs/related-resources.md` § `logs` ("Metric-filter-driven alarms") — a9s-devops (2026-04-20): possible=yes, worth=yes. Log-group → metric-filter → CW metric → alarm is the canonical bridge; DescribeMetricFilters is the read step, then reverse-scan the loaded alarm list.
- `apigw` via naming convention — `docs/related-resources.md` § `logs` ("APIGW access logs") — a9s-devops (2026-04-20): possible=yes, worth=yes. `API-Gateway-Execution-Logs_<apiId>/<stage>` (REST) and `/aws/http-api/<apiId>` (HTTP v2) are stable naming patterns.
- `ecs-task` via task-def `awslogs-group` — `docs/related-resources.md` § `logs` ("awslogs driver log groups") — a9s-devops (2026-04-20): possible=yes (requires enriched task-def on loaded task), worth=yes (log-to-task is the core incident pivot).
- `kinesis` via subscription filters — `docs/related-resources.md` § `logs` ("Subscription filter → Kinesis/Firehose") — a9s-devops (2026-04-20): possible=yes (DescribeSubscriptionFilters), worth=yes.
- `kms` via `LogGroup.kmsKeyId` — `AWS SDK Go v2 — cloudwatchlogs/types.LogGroup § KmsKeyId`; `docs/related-resources.md` § `logs` ("LogGroup.KmsKeyId").
- `lambda` via naming + subscription filters — `docs/related-resources.md` § `logs` ("Lambdas whose logs land here OR subscription-filter consumers") — a9s-devops (2026-04-20): possible=yes, worth=yes. `/aws/lambda/<name>` convention is stable and unambiguous.
- `s3` via `DescribeExportTasks` — `docs/related-resources.md` § `logs` ("Export tasks to S3") — a9s-devops (2026-04-20): possible=yes (per-group call), worth=yes (archive/compliance workflow).
- `ct-events` universal pivot — `docs/related-resources.md` § Policy item 4 ("`ct-events` … is implicitly relevant for every registered type").
- Wave 1 `retentionInDays` nil → Warning — `docs/attention-signals.md` § Monitoring row `logs`.
- Wave 1 `storedBytes==0 && creationTime<now()-90d` → Warning — `docs/attention-signals.md` § Monitoring row `logs`.
- Wave 1 KMS-PendingDeletion cross-ref — `docs/attention-signals.md` § Monitoring row `logs`.
- Wave 2 `lastEventTimestamp` staleness — `docs/attention-signals.md` § Monitoring row `logs`; `AWS SDK Go v2 — cloudwatchlogs/types.LogStream § LastEventTimestamp` (`*int64`, ms since epoch).
- Wave 3 metric-filter-count check (OUT OF SCOPE) — `docs/attention-signals.md` § Monitoring row `logs`.
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
- `~` severity choice for `lastEventTimestamp` staleness — user decision deferred; defaulted to `~` (informational background check on a green row) because a stale log stream is a lagging signal, not an active break. a9s-devops (2026-04-20): stale-log-group does not itself cause user-facing impact — it flags a silent emitter; worth surfacing but not worth bumping the menu `issues:N` count.
