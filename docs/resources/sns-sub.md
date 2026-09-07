---
shortName: sns-sub
name: SNS Subscriptions
awsApiRef: https://docs.aws.amazon.com/sns/latest/api/API_Subscription.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# sns-sub — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `sns-sub`
- **Display name**: SNS Subscriptions
- **AWS API reference**: <https://docs.aws.amazon.com/sns/latest/api/API_Subscription.html>
- **List API**: `ListSubscriptions`
- **Describe API (if any)**: not used (Wave 2 is `None`; `GetSubscriptionAttributes` is documented as Wave 3 and stays out of scope)

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `sns`, `lambda`, `sqs`, `ct-events`.

### `sns`

- **Why related**: parent topic — every subscription belongs to exactly one topic; the operator's first question on any sub is "which topic does this deliver from?". (`docs/related-resources.md` § `sns-sub` — "Parent topic.")
- **How discovered**: read field `TopicArn` on the Subscription and cross-reference the already-loaded `sns` list by ARN (`AWS SDK Go v2 — sns/types.Subscription § TopicArn`).
- **Count shown**: no — cardinality is always exactly 1, surfacing "1" would be noise (a9s-devops 2026-04-20).

### `lambda`

- **Why related**: Lambda endpoint subscriber — when this subscription targets a Lambda function, the pivot takes the operator straight to the function that actually runs on each published message. (`docs/related-resources.md` § `sns-sub` — "Lambda endpoint subscriber.")
- **How discovered**: read field `Endpoint` on the Subscription when `Protocol=="lambda"` — Endpoint carries the Lambda function ARN — and cross-reference the already-loaded `lambda` list by ARN (`AWS SDK Go v2 — sns/types.Subscription § Endpoint`, `§ Protocol`). For subscriptions whose `Protocol` is not `lambda`, this pivot does not apply.
- **Count shown**: no — exactly 1 endpoint per subscription when applicable (a9s-devops 2026-04-20).

### `sqs`

- **Why related**: SQS endpoint subscriber — when this subscription fans out to a queue, the pivot lands on the queue that is actually accumulating messages from the topic. (`docs/related-resources.md` § `sns-sub` — "SQS endpoint subscriber.")
- **How discovered**: read field `Endpoint` on the Subscription when `Protocol=="sqs"` — Endpoint carries the queue ARN — and cross-reference the already-loaded `sqs` list by ARN (`AWS SDK Go v2 — sns/types.Subscription § Endpoint`, `§ Protocol`). For subscriptions whose `Protocol` is not `sqs` (e.g. `http`, `https`, `email`, `email-json`, `sms`, `firehose`, `application`, `lambda`), this pivot does not apply.
- **Count shown**: no — exactly 1 endpoint per subscription when applicable (a9s-devops 2026-04-20).

### `ct-events`

- **Why related**: audit trail for subscription changes — who created/confirmed/deleted this sub, and recent errors. (`docs/related-resources.md` § `sns-sub` — "Audit trail for subscription changes.")
- **How discovered**: universal pivot — applies to every registered type; see related-resources.md §Policy. Resolved by `LookupEvents` filtered on `SubscriptionArn`.
- **Count shown**: yes — reflects recent matching events, so volume is informative at a glance (a9s-devops 2026-04-20).

## 3. Attention / Issues Algorithm

**Source API**: [ListSubscriptions](https://docs.aws.amazon.com/sns/latest/api/API_ListSubscriptions.html)

Transcribed from `docs/attention-signals.md § Signals § MESSAGING` row `sns-sub`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `SubscriptionArn == "PendingConfirmation"` → Warning (never confirmed).
  - **State bucket**: Warning.
  - **How obtained**: field `SubscriptionArn` on the `ListSubscriptions` response item. When the subscription has not yet been confirmed by the endpoint, AWS returns the literal sentinel string `"PendingConfirmation"` instead of a real ARN (`AWS SDK Go v2 — sns/types.Subscription § SubscriptionArn`).

All other Wave 1 `SubscriptionArn` values — i.e. a well-formed ARN like `arn:aws:sns:<region>:<account>:<topic>:<uuid>` — map to Healthy.

- **Signal**: `Protocol == "http"`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: the subscription's endpoint has been deleted.
  - **State bucket**: Dim.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `GetSubscriptionAttributes` per subscription (DLQ inspection — e.g. `RedrivePolicy` reachability, `PendingConfirmation` attribute cross-check, `FilterPolicy` anomalies).
- OUT OF SCOPE: CloudWatch `NumberOfNotificationsFailed` per endpoint.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `sns-sub`: Wave 1 issue-colored rows only — this type registers no Wave 2 enricher, so nothing else bumps the count.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `SubscriptionArn == "PendingConfirmation"` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `endpoint has not confirmed the subscription` |
| `Protocol == "http"` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `delivers over plain HTTP` |
| the subscription's endpoint has been deleted | 1 | Dim | n/a | S2, S4 | `endpoint deleted` |

### 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — the yellow row with `endpoint has not confirmed the subscription` in the Status column says plainly that the endpoint never acknowledged the subscribe request, so no messages are being delivered to it; the fix is out of a9s's read-only scope (re-send confirm or re-subscribe) but the diagnosis is complete on the list.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Subscription-level `kms` pivot — encryption is topic-level, not subscription-level (`docs/related-resources.md` § Explicitly excluded).
- Subscription-level `policy` pivot — subscription policies are attributes, not standalone policies (`docs/related-resources.md` § Explicitly excluded).
- `ecs` pivot — SNS subscriptions don't target ECS clusters/services directly (`docs/related-resources.md` § Explicitly excluded).
- Any write operation. a9s is read-only by design (`docs/architecture.md` § "What is a9s?").

## 6. Citations

- Display name and Wave 1 signal — `docs/attention-signals.md § Signals § MESSAGING` row `sns-sub`.
- List API `ListSubscriptions` — `core/aws/sns_sub.go`.
- Wave 3 `GetSubscriptionAttributes` and CloudWatch `NumberOfNotificationsFailed` — `docs/attention-signals.md § Not yet implemented`.
- Related contract targets `ct-events`, `lambda`, `sns`, `sqs` — `docs/related-resources.md` § Per-type contract — `sns-sub` row, and § `sns-sub`.
- AWS API reference URL — `docs/related-resources.md` § `sns-sub` (`https://docs.aws.amazon.com/sns/latest/api/API_Subscription.html`).
- Subscription shape fields (`SubscriptionArn`, `TopicArn`, `Protocol`, `Endpoint`, `Owner`) — `AWS SDK Go v2 — sns/types.Subscription`.
- `PendingConfirmation` sentinel string on `SubscriptionArn` — `AWS SDK Go v2 — sns/types.Subscription § SubscriptionArn` (documented wire behaviour of `ListSubscriptions` when the endpoint has not yet confirmed).
- Deliberately-not-registered pivots (`kms`, `policy`, `ecs`) — `docs/related-resources.md` § Explicitly excluded.
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
- `sns` discovery via `TopicArn` — `a9s-devops (2026-04-20): possible=yes (SDK field TopicArn on sns/types.Subscription), worth=yes. The very first question an operator asks on any subscription row is "which topic?" — this pivot must exist.`
- `lambda` discovery via `Endpoint` when `Protocol=="lambda"` — `a9s-devops (2026-04-20): possible=yes (SDK field Endpoint carries the Lambda ARN for Protocol==lambda), worth=yes. Tracing a subscription to the function that runs on each published message is the primary debugging pivot for Lambda-backed fanout.`
- `sqs` discovery via `Endpoint` when `Protocol=="sqs"` — `a9s-devops (2026-04-20): possible=yes (SDK field Endpoint carries the queue ARN for Protocol==sqs), worth=yes. When messages are piling up on a queue, starting from the subscription and jumping to the queue is faster than listing all queues and guessing.`
- `ct-events` discovery via `SubscriptionArn` lookup — `a9s-devops (2026-04-20): possible=yes (LookupEvents filtered on ARN), worth=yes. Audit trail answers "when was this sub created/confirmed/modified and by whom" — a real incident question.`
- Count hidden for `sns`, `lambda`, `sqs` pivots — `a9s-devops (2026-04-20): possible=yes (cardinality is statically known to be 1), worth=no. Singular pivots with a forced count of "1" add visual noise; operators read the pivot name and already know the cardinality. Hide the count.`
- Count shown for `ct-events` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Event volume is itself diagnostic (no recent events on a changed subscription is a clue), so the number matters here.`

<!-- BEGIN GENERATED: header -->
sns-sub — MESSAGING. Lifecycle key: none (the list API returns no lifecycle field).
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| sns-sub.state.pending-confirmation | endpoint has not confirmed the subscription | warn | wave1 | — |
| sns-sub.state.deleted | endpoint deleted | dim | wave1 | — |
| sns-sub.plain-http | delivers over plain HTTP | warn | wave1 | The subscription delivers over plain HTTP, so every message crosses the network in the clear and anyone on the path can read or alter it before the endpoint sees it. Point the subscription at an HTTPS endpoint. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| sns | SNS Topic | yes |
| lambda | Lambda Function | yes |
| sqs | SQS Queue | yes |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
