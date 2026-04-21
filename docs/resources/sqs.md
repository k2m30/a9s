---
shortName: sqs
name: SQS Queues
awsApiRef: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_GetQueueAttributes.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# sqs — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `sqs`
- **Display name**: SQS Queues
- **AWS API reference**: <https://docs.aws.amazon.com/AWSSimpleQueueService/latest/APIReference/API_GetQueueAttributes.html>
- **List API**: `ListQueues` (returns `QueueUrls []string` only — no attributes, no state field; SDK `sqs.ListQueuesOutput § QueueUrls`)
- **Describe API (if any)**: `GetQueueAttributes(AttributeNames=[All])` (per queue, Wave 2 — returns `Attributes map[string]string`)

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract (line 102): `alarm`, `ct-events`, `eb-rule`, `kms`, `lambda`, `sns`, `sns-sub`, `sqs`.

### `alarm`

- **Why related**: CloudWatch alarms watching queue-depth / consumer-lag — SQS alarms carry `Namespace="AWS/SQS"` and `Dimensions[{Name: "QueueName", Value: <queue-name>}]`. Primary incident pivot: "why is my queue alerting on backlog?" (related-resources.md §`sqs` line 938 — "ApproximateAgeOfOldestMessage / MessagesVisible alarms").
- **How discovered**: cross-reference the already-loaded `alarm` list by matching the queue's name (last path segment of the `QueueUrl`, or `Attributes["QueueArn"]`) against `Dimensions[].Value` where `Dimensions[].Name=="QueueName"` — a9s-devops (2026-04-21): standard list-scan, zero extra API calls because alarms are loaded in the same sweep.
- **Count shown**: yes — a9s-devops (2026-04-21): number of alarms on a queue is operationally meaningful (a noisy queue usually has multiple alarms on depth, age, DLQ receive count).

### `ct-events`

- **Why related**: audit trail for queue attribute changes (CreateQueue, SetQueueAttributes, TagQueue, DeleteQueue). Universal pivot — applies to every registered type; see related-resources.md §Policy §4 (line 34) and §`sqs` line 939.
- **How discovered**: universal — framework-level pivot, no per-type discovery logic (`ct-events` looks up events whose `Resources[].ResourceName` matches the queue ARN/name).
- **Count shown**: unknown — not specified in related-resources.md.

### `eb-rule`

- **Why related**: EventBridge rules whose targets deliver events into this queue — an EB-rule target's `Arn` is the queue's ARN (related-resources.md §`sqs` line 940; §`eb-rule` contract row line 62 lists `sqs` as an expected target).
- **How discovered**: a9s-devops (2026-04-21): possible=yes, worth=yes. The authoritative mapping lives on `ListTargetsByRule` (per-rule fan-out, Wave 2) — but the `eb-rule` resource already calls `ListTargetsByRule` as part of its own Wave 2 enrichment (per `attention-signals.md` line 85), so a9s can piggy-back: for each loaded `eb-rule`, scan its cached targets for `Arn == <queue-arn>` and collect matching rule IDs. No additional API calls. Operator workflow: "what's producing traffic into this queue?" is a standard messaging-triage question.
- **Count shown**: yes — a9s-devops (2026-04-21): number of rules feeding a queue is meaningful (e.g. fan-in from multiple scheduled rules vs a single event-driven rule).

### `kms`

- **Why related**: SSE-KMS customer-managed key — `GetQueueAttributes` returns `Attributes["KmsMasterKeyId"]` when SSE-KMS is enabled (related-resources.md §`sqs` line 941; SDK `sqs/types.QueueAttributeName § KmsMasterKeyId`).
- **How discovered**: read `Attributes["KmsMasterKeyId"]` on the `GetQueueAttributes` response already fetched in Wave 2; cross-reference the returned key ID/ARN against the loaded `kms` list.
- **Count shown**: unknown — a queue references at most one KMS key, so the count is degenerate (0 or 1).

### `lambda`

- **Why related**: Lambda functions that either (a) consume this queue via event-source mapping, or (b) route failures to this queue via DLQ. Both are core "who owns this queue?" pivots (related-resources.md §`sqs` line 942 — "Lambda event-source mappings consuming this queue").
- **How discovered**: a9s-devops (2026-04-21): possible=yes, worth=yes. Two discovery paths, both cross-referencing the already-loaded `lambda` list: (1) DLQ path — scan each function's `DeadLetterConfig.TargetArn` field from `FunctionConfiguration` (already on the Wave 1 response) for a match on the queue's ARN; (2) consumer path — Lambda event-source mappings are NOT on `FunctionConfiguration` and require `ListEventSourceMappings(EventSourceArn=<queue-arn>)` as a dedicated call. The consumer path is a Wave 2 fan-out; the DLQ path is zero extra cost. Operator workflow: during an incident on a Lambda, "is this the DLQ?" and "who's reading this queue?" are the two first questions.
- **Count shown**: yes for the combined set — a9s-devops (2026-04-21): the operator cares about the total number of Lambda associations (consumers + DLQ users), so a single count is decision-useful at a glance.

### `sns`

- **Why related**: SNS topics that fan out into this queue via an SNS→SQS subscription — the producer side of pub/sub (related-resources.md §`sqs` line 943 — "SQS subscribed to SNS topic"; mirror of `sns-sub` line 932 which lists `sqs` as the endpoint).
- **How discovered**: a9s-devops (2026-04-21): possible=yes, worth=yes. Cross-reference the already-loaded `sns-sub` list, filtering by `Protocol=="sqs" && Endpoint==<queue-arn>`, then group by `TopicArn` — the resulting set of topic ARNs is the list to cross-match against the loaded `sns` list. Alternative: parse `Attributes["Policy"]` JSON (SQS queue policy) for statements whose `Principal.Service=="sns.amazonaws.com"` and extract `Condition.ArnLike."aws:SourceArn"` topic ARNs; this is a fallback when the subscription list wasn't loaded in this sweep. Operator workflow: "who is publishing into this queue?" during fan-out debugging.
- **Count shown**: yes — a9s-devops (2026-04-21): number of SNS topics feeding a queue is a primary pub/sub topology signal.

### `sns-sub`

- **Why related**: the individual SNS subscription records that bind a topic to this queue — the granular per-subscription attributes (RawMessageDelivery, FilterPolicy, DeadLetterConfig) live on the subscription, not the topic (related-resources.md §`sqs` line 944 — "SNS subscriptions delivering to this queue"; §`sns-sub` line 932 — "SQS endpoint subscriber").
- **How discovered**: cross-reference the already-loaded `sns-sub` list filtered by `Protocol=="sqs" && Endpoint==<queue-arn>` — a9s-devops (2026-04-21): standard list-scan, same filter as §`sns` above but surfacing the subscription records directly rather than grouping.
- **Count shown**: yes — a9s-devops (2026-04-21): the subscription count tells the operator how many independent SNS→SQS bindings exist (same topic can have multiple subscriptions with different filter policies).

### `sqs`

- **Why related**: this queue's DLQ (outbound RedrivePolicy) and/or the queues for which this queue is the DLQ (inbound RedriveAllowPolicy / ListDeadLetterSourceQueues). Self-referential pivot for DLQ inspection — a messaging operator's first move on a failing queue (related-resources.md §`sqs` line 945 — "DLQ reference / RedriveTarget"; SDK `sqs/types.QueueAttributeName § RedrivePolicy § RedriveAllowPolicy`).
- **How discovered**: a9s-devops (2026-04-21): possible=yes, worth=yes. Two directions: (1) outbound — parse `Attributes["RedrivePolicy"]` JSON (already on Wave 2 response), extract `deadLetterTargetArn`, cross-reference against the loaded `sqs` list by ARN; (2) inbound — scan the loaded `sqs` list for any queue whose parsed `RedrivePolicy.deadLetterTargetArn` equals this queue's ARN. Both directions are zero extra API calls — the attribute map is already fetched. The dedicated `ListDeadLetterSourceQueues` API is an alternative for (2) but is redundant when the sibling list is in memory.
- **Count shown**: yes — a9s-devops (2026-04-21): the combined count (own DLQ + queues I am DLQ for) is decision-useful; a "DLQ for 12 queues" cell is an immediate signal about centralized failure routing.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md` § Messaging § `sqs` row (line 82).

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — the list API does not return fields usable for attention. `ListQueues` returns only `QueueUrls []string` (SDK `sqs.ListQueuesOutput § QueueUrls`); there is no state field and no attribute map on the list response.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: `ApproximateNumberOfMessages` > threshold → Warning (queue backlog).
  - **State bucket**: Warning.
  - **API call**: `GetQueueAttributes(AttributeNames=[All])` — one call per queue.
  - **Cost shape**: per-resource.
  - **Note**: the threshold is not specified in `attention-signals.md` — see §5 Out of Scope for deferral rationale.
- **Signal**: `ApproximateNumberOfMessages` rising unbounded → Broken (consumer stopped).
  - **State bucket**: Broken.
  - **API call**: `GetQueueAttributes` — same per-queue call (no added cost).
  - **Cost shape**: per-resource.
  - **Note**: "rising unbounded" requires two-sample trending across sweeps — the sampling cadence and delta-required are unspecified in `attention-signals.md`; see §5.
- **Signal**: `ApproximateAgeOfOldestMessage > VisibilityTimeout × 5` → Warning (consumer lag).
  - **State bucket**: Warning.
  - **API call**: `GetQueueAttributes` — same per-queue call.
  - **Cost shape**: per-resource.
  - **Rationale**: oldest-message age exceeding 5× the visibility timeout indicates consumers are failing to process messages within normal re-delivery windows.
- **Signal**: is-DLQ with messages → Warning.
  - **State bucket**: Warning.
  - **API call**: `GetQueueAttributes` — same per-queue call; DLQ-role detection uses either `Attributes["RedriveAllowPolicy"]` being present on this queue or cross-reference against sibling queues' `RedrivePolicy.deadLetterTargetArn`.
  - **Cost shape**: per-resource (plus sibling list-scan, zero extra calls).
- **Signal**: `RedrivePolicy` unset on main queue → Warning.
  - **State bucket**: Warning.
  - **API call**: `GetQueueAttributes` — same per-queue call.
  - **Cost shape**: per-resource.
  - **Note**: "main queue" (i.e. non-DLQ) is inferred as the complement of the is-DLQ detection above.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `NumberOfMessagesSent`/`NumberOfMessagesReceived` trend — per-queue metric time series, exceeds Wave 2 budget.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". Never appears on yellow/red/dim rows. |
| S4 | Status / description column text | Short human-readable cause. Healthy rows render blank. |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping applied to `sqs`:

- No Wave 1 signals → every queue row starts Healthy (green) with S4 blank until Wave 2 returns.
- Wave 2 **Warning-bucket** findings land on a yellow row (S2) with short cause in S4 and full sentence in S5. Because the row is yellow, S3 is suppressed (glyphs only appear on green rows). S1 is not bumped (`~`-style informational).
- Wave 2 **Broken-bucket** finding ("rising unbounded") lands on a red row (S2) with cause in S4 and full sentence in S5. S1 is bumped (`!`-severity).
- Multiple Wave 2 findings on the same queue (common — e.g. backlog + DLQ unset) deduplicate: S4 shows the most severe cause (Broken > Warning), S5 lists each sentence on its own line.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `ApproximateNumberOfMessages > threshold` | 2 | Warning | `~` | S2, S4, S5 | `backlog: <N> msgs` | `Queue depth (<N> messages) exceeds the configured backlog threshold.` |
| `ApproximateNumberOfMessages rising unbounded` | 2 | Broken | `!` | S1, S2, S4, S5 | `backlog growing: <N> msgs` | `Queue depth is increasing across samples — consumer has stopped or can't keep up.` |
| `ApproximateAgeOfOldestMessage > VisibilityTimeout × 5` | 2 | Warning | `~` | S2, S4, S5 | `oldest msg age: <D>` | `Oldest message age exceeds 5× visibility timeout — consumers are lagging.` |
| `is-DLQ with messages` | 2 | Warning | `~` | S2, S4, S5 | `DLQ has <N> msgs` | `Dead-letter queue holds <N> un-redriven messages — investigate failed consumers.` |
| `RedrivePolicy unset on main queue` | 2 | Warning | `~` | S2, S4, S5 | `no DLQ configured` | `Queue has no redrive policy — poison-pill messages will never be diverted.` |

## 4.1 UX review

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for every §4 row — `backlog: 50k msgs`, `oldest msg age: 3h`, `DLQ has 12 msgs`, `no DLQ configured`, and `backlog growing: 120k msgs` are all self-explanatory at a glance and let the operator prioritize which queue to drill into first. The only latent gap is that the backlog and rising-unbounded thresholds are not defined in `attention-signals.md` — implementers must choose sensible defaults (see §5) or the signals cannot ship; without that choice, every queue either looks fine or every queue looks broken.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- **Backlog threshold definition** — `attention-signals.md` line 82 writes `ApproximateNumberOfMessages > threshold` without specifying the threshold. a9s-devops (2026-04-21): possible=yes-via-heuristic, worth=yes-but-decision-deferred. No universal absolute number is correct — a 50k message queue is normal for a batch pipeline, broken for an interactive service. Viable approaches: (a) fixed default (e.g. 1 000 or 10 000), (b) per-queue tag override (`a9s:backlog_threshold=N`), (c) relative-to-VisibilityTimeout heuristic. Recommend deferring until a decision is added to `attention-signals.md`; shipping without a default silently means the signal never fires.
- **Rising-unbounded trend detection** — a9s-devops (2026-04-21): possible=yes, worth=yes-but-needs-sampling-contract. Requires two samples across sweeps with a defined minimum delta and time window; cache format change may be needed to store the previous sample. Defer until `attention-signals.md` specifies sample cadence and delta threshold.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` § "What is a9s?").

## 6. Citations

- Identity — List API `ListQueues` returns URLs only — `AWS SDK Go v2 — sqs.ListQueuesOutput § QueueUrls`.
- Identity — Describe API `GetQueueAttributes` returns attribute map — `AWS SDK Go v2 — sqs.GetQueueAttributesOutput § Attributes`.
- Identity — attribute-name enum (Policy, VisibilityTimeout, ApproximateNumberOfMessages, ApproximateAgeOfOldestMessage, RedrivePolicy, RedriveAllowPolicy, KmsMasterKeyId, QueueArn, …) — `AWS SDK Go v2 — sqs/types.QueueAttributeName`.
- §2 related targets — contract row — `docs/related-resources.md` § Per-type contract § `sqs` (line 102) and § `sqs` section (lines 934-945).
- §2 `alarm` why related — `docs/related-resources.md` § `sqs` (line 938 — "ApproximateAgeOfOldestMessage / MessagesVisible alarms").
- §2 `alarm` how discovered — a9s-devops (2026-04-21): possible=yes, worth=yes. Alarms loaded in the same sweep carry `Dimensions[QueueName]`; list-scan by queue-name/ARN gives a zero-extra-call cross-reference.
- §2 `alarm` count shown — a9s-devops (2026-04-21): possible=yes, worth=yes. Alarm count per queue is operationally meaningful during backlog triage.
- §2 `ct-events` universal pivot — `docs/related-resources.md` § Policy §4 (line 34) and §`sqs` (line 939).
- §2 `ct-events` count shown — golden docs silent, marked unknown.
- §2 `eb-rule` why related — `docs/related-resources.md` § `sqs` (line 940) and § `eb-rule` contract row (line 62).
- §2 `eb-rule` how discovered — a9s-devops (2026-04-21): possible=yes, worth=yes. `eb-rule` Wave 2 already calls `ListTargetsByRule` per rule (see `attention-signals.md` line 85); piggy-back on its cached targets and filter by `Target.Arn == queue-arn`. No additional API calls.
- §2 `eb-rule` count shown — a9s-devops (2026-04-21): possible=yes, worth=yes. Number of EB rules feeding a queue is a fan-in topology signal.
- §2 `kms` why related — `docs/related-resources.md` § `sqs` (line 941) and `AWS SDK Go v2 — sqs/types.QueueAttributeName § KmsMasterKeyId`.
- §2 `kms` how discovered — a9s-devops (2026-04-21): possible=yes, worth=yes. `KmsMasterKeyId` is a standard key on the Wave 2 attribute map; cross-reference against loaded `kms` list. No extra API call.
- §2 `lambda` why related — `docs/related-resources.md` § `sqs` (line 942 — "Lambda event-source mappings consuming this queue").
- §2 `lambda` how discovered — a9s-devops (2026-04-21): possible=yes, worth=yes. DLQ path = scan `FunctionConfiguration.DeadLetterConfig.TargetArn` on loaded `lambda` list (zero cost). Consumer path = `ListEventSourceMappings(EventSourceArn=<queue-arn>)` (one per queue, Wave 2). Both are core "who owns this queue?" pivots.
- §2 `lambda` count shown — a9s-devops (2026-04-21): possible=yes, worth=yes. Combined count (consumers + DLQ users) gives the operator the breadth of Lambda coupling at a glance.
- §2 `sns` why related — `docs/related-resources.md` § `sqs` (line 943) and § `sns-sub` (line 932).
- §2 `sns` how discovered — a9s-devops (2026-04-21): possible=yes, worth=yes. Cross-reference `sns-sub` list by `Protocol=="sqs" && Endpoint==queue-arn`, group by `TopicArn`, then match against loaded `sns` list. Fallback: parse SQS `Attributes["Policy"]` for `Principal.Service=="sns.amazonaws.com"` statements when the subscription list is absent.
- §2 `sns` count shown — a9s-devops (2026-04-21): possible=yes, worth=yes. Count of producing topics is a primary fan-in topology signal.
- §2 `sns-sub` why related — `docs/related-resources.md` § `sqs` (line 944) and § `sns-sub` (line 932).
- §2 `sns-sub` how discovered — a9s-devops (2026-04-21): possible=yes, worth=yes. Same cross-reference as `sns` but without the grouping step — returns subscription records directly.
- §2 `sns-sub` count shown — a9s-devops (2026-04-21): possible=yes, worth=yes. Independent subscription count (multiple filter policies on one topic produce multiple subscriptions) is decision-useful.
- §2 `sqs` (self) why related — `docs/related-resources.md` § `sqs` (line 945 — "DLQ reference / RedriveTarget") and `AWS SDK Go v2 — sqs/types.QueueAttributeName § RedrivePolicy § RedriveAllowPolicy`.
- §2 `sqs` (self) how discovered — a9s-devops (2026-04-21): possible=yes, worth=yes. Outbound: parse `RedrivePolicy.deadLetterTargetArn` from Wave 2 attributes. Inbound: scan sibling queues' `RedrivePolicy` for this queue's ARN. Zero extra API calls; dedicated `ListDeadLetterSourceQueues` is redundant when sibling list is cached.
- §2 `sqs` (self) count shown — a9s-devops (2026-04-21): possible=yes, worth=yes. Combined DLQ count (own DLQ + queues I am DLQ for) is a primary messaging-topology signal.
- §3.1 no Wave 1 signals — `docs/attention-signals.md` § Messaging § `sqs` row (line 82, Wave 1 cell: "None — `ListQueues` returns URLs only") and `AWS SDK Go v2 — sqs.ListQueuesOutput § QueueUrls`.
- §3.2 Wave 2 signals (backlog threshold, rising unbounded, oldest-message age, is-DLQ with messages, RedrivePolicy unset) — `docs/attention-signals.md` § Messaging § `sqs` row (line 82, Wave 2 cell).
- §3.2 attribute field names (`ApproximateNumberOfMessages`, `ApproximateAgeOfOldestMessage`, `VisibilityTimeout`, `RedrivePolicy`, `RedriveAllowPolicy`) — `AWS SDK Go v2 — sqs/types.QueueAttributeName` enum constants.
- §3.3 Wave 3 CloudWatch trend — `docs/attention-signals.md` § Messaging § `sqs` row (line 82, Wave 3 cell).
- §4 S4/S5 wording — a9s-devops (2026-04-21): possible=yes, worth=yes. Each row gives the operator a concrete cause in the list (e.g. `backlog: 50k msgs`, `DLQ has 12 msgs`) that can be triaged without opening detail; S5 adds the consequence sentence for the detail view.
- §5 backlog-threshold deferral — a9s-devops (2026-04-21): possible=yes-via-heuristic, worth=yes-but-decision-deferred. No universal absolute number works across interactive vs batch workloads; recommend deferring to an explicit decision in `attention-signals.md` (fixed default, per-queue tag override, or relative heuristic).
- §5 rising-unbounded deferral — a9s-devops (2026-04-21): possible=yes, worth=yes-but-needs-sampling-contract. Needs sample-cadence + delta-threshold specification and possibly cache-format change; defer until `attention-signals.md` specifies.
- §5 read-only invariant — `docs/architecture.md` § "What is a9s?".
