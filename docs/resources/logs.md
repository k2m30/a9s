---
shortName: logs
name: CloudWatch Log Groups
awsApiRef: https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_LogGroup.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# logs — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `logs`
- **Display name**: CloudWatch Log Groups
- **AWS API reference**: <https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_LogGroup.html>
- **List API**: `DescribeLogGroups` (per group returns `logGroupName`, `creationTime`, `retentionInDays`, `metricFilterCount`, `arn`, `storedBytes`, `kmsKeyId`, `dataProtectionStatus`, `logGroupClass`; no event-level data).
- **Describe API (if any)**: `DescribeLogStreams` (per log group, ordered by `LastEventTime` descending, `limit=1`) — used in Wave 2 to read `lastEventTimestamp` for silent-service detection.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `alarm`, `apigw`, `ecs-task`, `kinesis`, `kms`, `lambda`, `s3`, `ct-events`.

### `alarm`

- **Why related**: Metric-filter-driven alarms fire off patterns in this log group — when the operator is diagnosing a noisy log group, the very next question is which alarms are watching it. Cited in `docs/related-resources.md` §`logs` → "Metric-filter-driven alarms."
- **How discovered**: Call `DescribeMetricFilters(logGroupName=…)` to list the filters attached to this log group, read each filter's `metricTransformations[].metricName` + `metricNamespace`, then reverse-scan the already-loaded `alarm` list matching on `MetricName` + `Namespace`. — a9s-devops: the log group ↔ alarm linkage is indirect (log group → metric filter → CloudWatch metric → alarm); no direct field bridges them. possible=yes (DescribeMetricFilters is read-only, per-group), worth=yes (this is the canonical "what alerts if this log group goes bad?" workflow).
- **Count shown**: yes.

### `apigw`

- **Why related**: API Gateway writes execution and access logs to CloudWatch Logs — the operator reading a request failure wants to hop from the log group to the API definition. Cited in `docs/related-resources.md` §`logs` → "APIGW access logs."
- **How discovered**: Match the log group `logGroupName` against the API Gateway naming conventions — `API-Gateway-Execution-Logs_<apiId>/<stage>` (REST v1), `/aws/apigateway/welcome`, `/aws/http-api/<apiId>` (HTTP v2), or a user-chosen access-log destination — and cross-reference the already-loaded `apigw` list by `apiId`. — a9s-devops: naming convention is the only stable link for execution logs; user-chosen access-log groups need tag/stage-config walk that is Wave 2+ and out of scope here. possible=yes (naming convention + reverse-scan), worth=yes (apigw troubleshooting starts in logs).
- **Count shown**: yes.

### `ecs-task`

- **Why related**: ECS tasks using the `awslogs` driver write stdout/stderr into this log group — the operator reading an error line wants the task that produced it. Cited in `docs/related-resources.md` §`logs` → "awslogs driver log groups."
- **How discovered**: Reverse-scan the already-loaded `ecs-task` list — each task's `containers[]` carries a reference to its task definition's `containerDefinitions[].logConfiguration.logDriver=awslogs` and `options.awslogs-group` value; match that value against this log group's `logGroupName`. Requires the task-definition body to be available in the loaded task record; if not, pivot is best-effort. — a9s-devops: `awslogs-group` on the task-definition container is the authoritative field, but it's not on the `ListTasks` shape; enrichment may be needed. possible=yes (via loaded task-def), worth=yes (log → owning task is the core incident pivot).
- **Count shown**: yes.

### `kinesis`

- **Why related**: Subscription filters fan log events out to Kinesis Data Streams or Firehose — understanding where a log group's data is being consumed downstream matters for pipeline debugging. Cited in `docs/related-resources.md` §`logs` → "Subscription filter → Kinesis/Firehose."
- **How discovered**: Call `DescribeSubscriptionFilters(logGroupName=…)` per log group, read each filter's `destinationArn`; when the ARN is `arn:aws:kinesis:…:stream/<name>`, match against the already-loaded `kinesis` list by stream name. Firehose destinations are a different service and surface in a dedicated pivot if registered. — a9s-devops: subscription filters are the canonical fan-out mechanism and the only read path from log group to stream. possible=yes, worth=yes (required for "where does this log data end up?").
- **Count shown**: yes.

### `kms`

- **Why related**: When the log group is encrypted with a customer-managed KMS key, any KMS key disable / pending-deletion will silently block log ingestion — the operator reading "no new events" needs the key one key press away. Cited in `docs/related-resources.md` §`logs` → "LogGroup.KmsKeyId."
- **How discovered**: Read `LogGroup.kmsKeyId` directly from the list response (`AWS SDK Go v2 — cloudwatchlogs/types.LogGroup § KmsKeyId`); if non-empty, cross-reference the already-loaded `kms` list by key ARN.
- **Count shown**: yes.

### `lambda`

- **Why related**: Lambda functions write to log groups named `/aws/lambda/<function-name>` — the single most common operator pivot is "whose function logs am I looking at?" Subscription-filter consumers (Lambda target of a filter) are a second, rarer case. Cited in `docs/related-resources.md` §`logs` → "Lambdas whose logs land here OR subscription-filter consumers."
- **How discovered**: (a) Match `logGroupName` against the `/aws/lambda/<name>` convention and cross-reference the already-loaded `lambda` list by function name; (b) call `DescribeSubscriptionFilters(logGroupName=…)` and cross-reference filters whose `destinationArn` is `arn:aws:lambda:…:function:<name>`. — a9s-devops: the naming convention is stable and unambiguous for function-owned log groups; the subscription-filter case is additive. possible=yes, worth=yes (this is the #1 Lambda debugging pivot).
- **Count shown**: yes.

### `s3`

- **Why related**: Export tasks archive a log group's events into an S3 bucket for long-term retention or downstream analytics — operator auditing archival posture or investigating export failures pivots here. Cited in `docs/related-resources.md` §`logs` → "Export tasks to S3."
- **How discovered**: Call `DescribeExportTasks` and filter by `logGroupName`, then cross-reference each task's `destination` bucket name against the already-loaded `s3` list. — a9s-devops: DescribeExportTasks is the only read-only surface that links a log group to an S3 archive target; there is no reverse field on the bucket. possible=yes (per-group call, bounded), worth=yes (archive posture is a compliance/cost workflow).
- **Count shown**: yes.

### `ct-events`

- **Why related**: Universal pivot — applies to every registered type; see docs/related-resources.md §Policy. Audit trail for log group configuration changes (create, delete, put-retention-policy, associate-kms-key, put-subscription-filter).
- **How discovered**: `LookupEvents` with `LookupAttributes=[{AttributeKey:ResourceName, AttributeValue:<logGroupName>}]` in the current region.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeLogStreams](https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_DescribeLogStreams.html)

Transcribed from `docs/attention-signals.md § Signals § MONITORING` row `logs`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `retentionInDays` is nil → Warning (Never Expire = cost drift).
  - **State bucket**: Warning.
  - **How obtained**: `DescribeLogGroups` list response — field `retentionInDays` (`AWS SDK Go v2 — cloudwatchlogs/types.LogGroup § RetentionInDays`, `*int32`; nil means "retain forever").

- **Signal**: `storedBytes==0 && creationTime<now()-90d` → Warning (orphan).
  - **State bucket**: Warning.
  - **How obtained**: `DescribeLogGroups` list response — fields `storedBytes` and `creationTime` (`types.LogGroup § StoredBytes`, `§ CreationTime`).

- **Signal**: `kmsKeyId` empty.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: an audit log group with no metric filter over it.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: Metric-filter-count check.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `logs`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `retentionInDays` nil | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `retention: never expire` |
| `storedBytes==0` + age >90d | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `empty, created over 90 days ago` |
| `kmsKeyId` empty | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `not encrypted with KMS` |
| an audit log group with no metric filter over it | 2 | Warning | `~` | S2, S3, S4, S5 | `audit log group missing metric filters` |

Rules for filling list and detail text:

- Banned words: `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword in the List text column is not acceptable — pair with cause.
- Keep the List text short: ≤ 40 chars. The Detail cell quotes the finding's Detail constant verbatim, however long it is.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for all three Wave 1 signals (retention, orphan, KMS deletion) — the S4 text names both the condition and its consequence; an audit group with no metric filter over it goes yellow reading `audit log group missing metric filters`, and the detail view names which filters are missing. No §4 gap identified.

## 5. Out of Scope

- All §3.3 Wave 3 signals (metric-filter-count check — `DescribeMetricFilters` per log group, cost-bounded but not currently surfaced).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- List API and list-response fields — `AWS SDK Go v2 — cloudwatchlogs/types.LogGroup § LogGroupName, CreationTime, RetentionInDays, StoredBytes, KmsKeyId, MetricFilterCount`.
- Describe API for Wave 2 — `AWS SDK Go v2 — cloudwatchlogs/types.LogStream § LastEventTimestamp`.
- Related targets (full list) — `docs/related-resources.md` § Per-type contract, row `logs`; detailed reasoning in `docs/related-resources.md` § `logs`.
- `alarm` via metric-filter bridge — `docs/related-resources.md` § `logs` ("Metric-filter-driven alarms") — a9s-devops (2026-04-20): possible=yes, worth=yes. Log-group → metric-filter → CW metric → alarm is the canonical bridge; DescribeMetricFilters is the read step, then reverse-scan the loaded alarm list.
- `apigw` via naming convention — `docs/related-resources.md` § `logs` ("APIGW access logs") — a9s-devops (2026-04-20): possible=yes, worth=yes. `API-Gateway-Execution-Logs_<apiId>/<stage>` (REST) and `/aws/http-api/<apiId>` (HTTP v2) are stable naming patterns.
- `ecs-task` via task-def `awslogs-group` — `docs/related-resources.md` § `logs` ("awslogs driver log groups") — a9s-devops (2026-04-20): possible=yes (requires enriched task-def on loaded task), worth=yes (log-to-task is the core incident pivot).
- `kinesis` via subscription filters — `docs/related-resources.md` § `logs` ("Subscription filter → Kinesis/Firehose") — a9s-devops (2026-04-20): possible=yes (DescribeSubscriptionFilters), worth=yes.
- `kms` via `LogGroup.kmsKeyId` — `AWS SDK Go v2 — cloudwatchlogs/types.LogGroup § KmsKeyId`; `docs/related-resources.md` § `logs` ("LogGroup.KmsKeyId").
- `lambda` via naming + subscription filters — `docs/related-resources.md` § `logs` ("Lambdas whose logs land here OR subscription-filter consumers") — a9s-devops (2026-04-20): possible=yes, worth=yes. `/aws/lambda/<name>` convention is stable and unambiguous.
- `s3` via `DescribeExportTasks` — `docs/related-resources.md` § `logs` ("Export tasks to S3") — a9s-devops (2026-04-20): possible=yes (per-group call), worth=yes (archive/compliance workflow).
- `ct-events` universal pivot — `docs/related-resources.md` § Policy item 4 ("`ct-events` … is implicitly relevant for every registered type").
- Wave 1 `retentionInDays` nil → Warning — `docs/attention-signals.md § Signals § MONITORING` row `logs`.
- Wave 1 `storedBytes==0 && creationTime<now()-90d` → Warning — `docs/attention-signals.md § Signals § MONITORING` row `logs`.
- Wave 1 KMS-PendingDeletion cross-ref — `docs/attention-signals.md § Signals § MONITORING` row `logs`.
- Wave 2 `lastEventTimestamp` staleness — `docs/attention-signals.md § Signals § MONITORING` row `logs`; `AWS SDK Go v2 — cloudwatchlogs/types.LogStream § LastEventTimestamp` (`*int64`, ms since epoch).
- Wave 3 metric-filter-count check (OUT OF SCOPE) — `docs/attention-signals.md § Not yet implemented`.
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
- `~` severity choice for `lastEventTimestamp` staleness — user decision deferred; defaulted to `~` (informational, not an active break) because a stale log stream is a lagging signal, not an active break. a9s-devops (2026-04-20): stale-log-group does not itself cause user-facing impact — it flags a silent emitter; worth surfacing but not worth bumping the menu `issues:N` count.

<!-- BEGIN GENERATED: header -->
logs — MONITORING. Status key: `state` — the column naming it is the status column, and no fetcher writes it, so the cell is the finding phrase.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| logs.retention-never-expire | retention: never expire | warn | wave1 | Events in this group are kept forever and billed forever, and an unbounded audit log is also a growing pile of whatever the application logged. Set a retention period that matches how far back anyone actually looks. |
| logs.stale-empty | empty, created over 90 days ago | warn | wave1 | The group has existed for months and holds no events, so whatever was meant to write here never did, and anything relying on those logs for debugging or audit has nothing. Find the producer that should be writing and fix its permissions or configuration, or delete the group. |
| logs.missing-metric-filters | audit log group missing metric filters | warn | wave2 | This group carries audit or security logs but has no metric filter over it, so the events it collects are only found when somebody goes to look. Add metric filters and alarms for the events worth waking up for. |
| logs.no-kms | not encrypted with KMS | warn | wave1 | Log events are encrypted with the CloudWatch Logs service key, so anyone with read access to the log group can read them and you cannot revoke that access with a key policy. Associate a KMS key with this log group. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| lambda | Lambda Functions | yes |
| alarm | CW Alarms | yes |
| kms | KMS Key | no |
| apigw | API Gateway | yes |
| ecs-task | ECS Tasks | yes |
| kinesis | Kinesis Streams | no |
| s3 | S3 (exports) | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
