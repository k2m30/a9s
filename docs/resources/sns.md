---
shortName: sns
name: SNS Topics
awsApiRef: https://docs.aws.amazon.com/sns/latest/api/API_Topic.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# sns — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `sns`
- **Display name**: SNS Topics
- **AWS API reference**: <https://docs.aws.amazon.com/sns/latest/api/API_Topic.html>
- **List API**: `ListTopics` (returns `Topic.TopicArn` only — no attributes, no state field)
- **Describe API (if any)**: `ListSubscriptionsByTopic` (per topic, paginated — drives the Wave 2 subscription signals); `GetTopicAttributes` (per topic — attribute-based pivots such as `kms`/`role`)

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `alarm`, `ct-events`, `kms`, `role`, `sns-sub`.

### `alarm`

- **Why related**: CloudWatch alarms that notify this topic — `MetricAlarm.AlarmActions` / `OKActions` / `InsufficientDataActions` contain SNS topic ARNs. Primary incident pivot: "which alarms route to this channel?" (docs/related-resources.md § `sns`; `docs/related-resources.md` § `alarm`).
- **How discovered**: cross-reference the already-loaded `alarm` list by matching the topic's `TopicArn` against any entry in each alarm's `AlarmActions` / `OKActions` / `InsufficientDataActions` — a9s-devops: standard list-scan, no extra API call needed since alarms are loaded in the same sweep.
- **Count shown**: yes — a9s-devops: number of alarms fanning into the topic is operationally meaningful (noisy channel detection).

### `ct-events`

- **Why related**: audit trail for topic changes (CreateTopic, SetTopicAttributes, DeleteTopic). Universal pivot — applies to every registered type; see docs/related-resources.md §Policy §4.
- **How discovered**: universal — framework-level pivot, no per-type discovery logic.
- **Count shown**: unknown — `docs/related-resources.md` § `sns` does not specify.

### `kms`

- **Why related**: SSE-KMS encryption key — `GetTopicAttributes` returns `KmsMasterKeyId` when server-side encryption is enabled (docs/related-resources.md §`sns`; SDK `sns.GetTopicAttributesOutput` § `Attributes["KmsMasterKeyId"]`).
- **How discovered**: read `Attributes["KmsMasterKeyId"]` on the `GetTopicAttributes` response already fetched in Wave 2; match the returned key ID/ARN against the loaded `kms` list.
- **Count shown**: unknown — a topic references at most one KMS key, so the count is degenerate (0 or 1).

### `role`

- **Why related**: IAM principals granted publish/subscribe/manage permissions by the topic's resource policy — `GetTopicAttributes` returns the access-control document in `Attributes["Policy"]` as JSON, whose `Statement[].Principal` commonly lists role ARNs (docs/related-resources.md §`sns`; SDK `sns.GetTopicAttributesOutput` § `Attributes["Policy"]`) — a9s-devops: used during IAM audits to answer "who can publish to this topic?"; the 1/6-audit rationale in the golden doc is weak but the workflow is real.
- **How discovered**: parse `Attributes["Policy"]` JSON from `GetTopicAttributes` (Wave 2), extract each `Statement[].Principal.AWS` ARN whose ARN type is `role`, and cross-reference against the loaded `role` list — a9s-devops: JSON parse is cheap; skip silently if policy is absent (no statement → no principals → empty list).
- **Count shown**: unknown — `docs/related-resources.md` § `sns` does not specify.

### `sns-sub`

- **Why related**: subscriptions delivering messages off this topic — the core consumer-side pivot. "What's listening on this topic?" is the first question when publish latency or failed-delivery counts spike (docs/related-resources.md §`sns`; §`sns-sub`).
- **How discovered**: call `ListSubscriptionsByTopic(TopicArn)` — a9s-devops: this is the dedicated SNS API for the relationship; no cheaper list-scan path exists because `ListSubscriptions` is account-wide and paginated.
- **Count shown**: yes — a9s-devops: fanout width (number of confirmed subscriptions) is decision-useful at a glance.

## 3. Attention / Issues Algorithm

**Source API**: [GetTopicAttributes](https://docs.aws.amazon.com/sns/latest/api/API_GetTopicAttributes.html)

Transcribed from `docs/attention-signals.md § Signals § MESSAGING` row `sns`.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — the list API does not return fields usable for attention. `ListTopics` returns `Topic.TopicArn` only (SDK `sns/types.Topic`), no state field, no attribute map.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: `ListSubscriptionsByTopic` returns zero subscriptions → Warning (orphan topic — same semantics as the golden-doc `SubscriptionsConfirmed==0 AND SubscriptionsPending==0` condition).
  - **State bucket**: Warning (informational — the topic is Healthy in the AWS-state sense, but operationally orphaned).
  - **API call**: `ListSubscriptionsByTopic` — one paginated call chain per topic (follows `NextToken` to completion).
  - **Cost shape**: per-resource.

- **Signal**: every subscription returned by `ListSubscriptionsByTopic` is still unconfirmed (`SubscriptionArn == "PendingConfirmation"`) → Warning (deliveries go nowhere until an endpoint confirms).
  - **State bucket**: Warning (informational — companion finding from the same per-topic call; fires only when at least one subscription exists and none is confirmed).
  - **API call**: `ListSubscriptionsByTopic` — same call as above; no added cost.
  - **Cost shape**: per-resource.

- **Signal**: `KmsMasterKeyId` absent or empty.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: access `Policy` allows a wildcard principal.
  - **State bucket**: Broken.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: `KmsMasterKeyId` absent on sensitive topic → Warning. — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06)
  - **State bucket**: Warning.
  - **API call**: `GetTopicAttributes` — one call per topic (same call as above; no added cost).
  - **Cost shape**: per-resource.
  - **Note**: what makes a topic "sensitive" is undefined — the detection heuristic (tag, name pattern, policy content) is unspecified, and `docs/attention-signals.md § Signals § MESSAGING` row `sns` carries no finding for it. See §5 Out of Scope. — a9s-devops: there is no reliable AWS-surface field identifying sensitivity from SNS alone; viable heuristics are tag-based (e.g. `sensitive=true`) or name-regex, both of which are per-deployment conventions. Worth=no as a universal default rule.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `NumberOfNotificationsFailed` — per-topic metric, rate-limited, outside Wave 2 budget.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `sns`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| zero subscriptions on the topic | 2 | Warning | `~` | S2, S3, S4, S5 | `topic has no subscribers` |
| all subscriptions unconfirmed | 2 | Warning | `~` | S2, S3, S4, S5 | `all pending confirmation` |
| `KmsMasterKeyId` absent or empty | 2 | Warning | `~` | S2, S3, S4, S5 | `not encrypted with KMS` |
| access `Policy` allows a wildcard principal | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `topic policy open to anyone` |

## 4.1 UX review

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for the orphan-topic row — a yellow row reading `topic has no subscribers` is self-explanatory, and so is a yellow row reading `all pending confirmation`. A topic reachable by anyone shows a red row reading `topic policy open to anyone`. Every unencrypted topic shows a yellow row reading `not encrypted with KMS`, so operators who do not want that signal on low-value topics should read it as a posture note rather than an incident.

## 4.2 On-Demand Detail Enrichment

Opening the detail, YAML, or JSON view triggers one extra read-only call whose result is attached to the resource's raw structure for the stacked views. List views are never affected.

- AWS API: `GetTopicAttributes`
- Payload: the full topic attribute map — subscription counts (confirmed/pending), KMS master key, delivery policies; JSON-valued attributes (`Policy`, `DeliveryPolicy`, `EffectiveDeliveryPolicy`) render as structured YAML
- Cache: none — a single cheap call, and subscription counts drift during a session
- Failure: a flash message; the view still renders the un-enriched resource

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- `KmsMasterKeyId absent on sensitive topic` — a9s-devops: not worth it as a universal default. SNS has no reliable AWS-surface field identifying "sensitive"; viable heuristics (tag `sensitive=true`, name regex matching `prod|pii|secret`, policy-content scan) are per-deployment conventions and produce noisy signal at the account level. Recommend deferring until an explicit trigger is specified.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?", line 13).

## 6. Citations

- Identity — List API `ListTopics` returns ARN only — `AWS SDK Go v2 — sns/types.Topic § TopicArn`.
- Identity — Describe API `GetTopicAttributes` returns attribute map incl. `SubscriptionsConfirmed`, `SubscriptionsPending`, `KmsMasterKeyId`, `Policy` — `AWS SDK Go v2 — sns.GetTopicAttributesOutput § Attributes`.
- §2 related targets — contract row — `docs/related-resources.md` § Per-type contract, row `sns`, and `docs/related-resources.md` § `sns`.
- §2 `alarm` why related — SNS topic ARNs live on `MetricAlarm.AlarmActions`/`OKActions` — `docs/related-resources.md` § `alarm`.
- §2 `alarm` how discovered — a9s-devops (2026-04-20): possible=yes, worth=yes. CloudWatch alarms already loaded in the sweep carry the SNS ARNs in their action-ARN arrays; a single list-scan cross-reference. Standard incident pivot.
- §2 `alarm` count shown — a9s-devops (2026-04-20): possible=yes, worth=yes. Fanout width is operationally meaningful — a topic with 40 alarms is a noisy channel.
- §2 `ct-events` universal pivot — `docs/related-resources.md` § Policy §4.
- §2 `ct-events` count shown — golden docs silent, marked unknown.
- §2 `kms` why related — `docs/related-resources.md` § `sns` and `AWS SDK Go v2 — sns.GetTopicAttributesOutput § Attributes["KmsMasterKeyId"]`.
- §2 `kms` how discovered — a9s-devops (2026-04-20): possible=yes, worth=yes. `GetTopicAttributes` already called in Wave 2 carries `KmsMasterKeyId`; cross-reference against the loaded `kms` list. No extra API call.
- §2 `role` why related — `docs/related-resources.md` § `sns` and `AWS SDK Go v2 — sns.GetTopicAttributesOutput § Attributes["Policy"]`.
- §2 `role` how discovered — a9s-devops (2026-04-20): possible=yes, worth=yes-but-marginal. Parse topic `Policy` JSON (already on Wave 2 response), extract role ARNs from `Statement[].Principal.AWS`, cross-reference against loaded `role` list. Golden-doc rationale is weak ("1/6 audits") but the IAM-audit workflow exists.
- §2 `sns-sub` why related — `docs/related-resources.md` § `sns` and § `sns-sub`.
- §2 `sns-sub` how discovered — a9s-devops (2026-04-20): possible=yes, worth=yes. `ListSubscriptionsByTopic(TopicArn)` is the dedicated SNS API; no cheaper path — `ListSubscriptions` is account-wide and paginated with no topic filter.
- §2 `sns-sub` count shown — a9s-devops (2026-04-20): possible=yes, worth=yes. Fanout-width is a primary decision signal for an SNS operator.
- §3.1 no Wave 1 signals — `ListTopics` returns the topic ARN only, `AWS SDK Go v2 — sns.ListTopicsOutput § Topics`; every `sns` finding is wave 2, `docs/attention-signals.md § Signals § MESSAGING` row `sns`.
- §3.2 orphan-topic signal — `docs/attention-signals.md § Signals § MESSAGING` row `sns`. Mechanism amended to match the implementation: `ListSubscriptionsByTopic` with pagination and per-subscription `PendingConfirmation` detection (`core/aws/sns_issue_enrichment.go:22-99`), not `GetTopicAttributes` subscription counts; same signal semantics.
- §3.2 all-pending-confirmation signal — companion finding from the same `ListSubscriptionsByTopic` call (`core/aws/sns_issue_enrichment.go:22-99`): fires when every returned `SubscriptionArn == "PendingConfirmation"`.
- §3.2 missing-KMS signal — `docs/attention-signals.md § Signals § MESSAGING` row `sns`. Trigger definition for "sensitive topic" not specified.
- §3.2 `SubscriptionsConfirmed`/`SubscriptionsPending`/`KmsMasterKeyId` field names — `AWS SDK Go v2 — sns.GetTopicAttributesOutput § Attributes` doc comment.
- §3.3 deferred CloudWatch metric — `docs/attention-signals.md § Not yet implemented`.
- §4 orphan-topic S4/S5 wording — a9s-devops (2026-04-20): possible=yes, worth=yes. `no subscribers` on the row lets an operator triage without drilling in; S5 spells out the consequence (messages discarded) for the detail view.
- §5 missing-KMS deferral — a9s-devops (2026-04-20): possible=yes-via-tags-or-regex, worth=no as a universal default. Without an explicit "sensitive" trigger the rule fires on every unencrypted topic, producing noise. Defer until that trigger is defined.
- §5 read-only invariant — `docs/architecture.md` § "What is a9s?" (line 15).

<!-- BEGIN GENERATED: header -->
sns — MESSAGING. Lifecycle key: none (the list API returns no lifecycle field).
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| sns.no-subscribers | topic has no subscribers | warn | wave2 | Nothing is subscribed to this topic, so every message published to it is discarded on arrival. Either subscribe the endpoint that was meant to receive them, or delete the topic and whatever still publishes to it. |
| sns.all-pending-confirmation | all pending confirmation | warn | wave2 | Every subscription on this topic is still waiting for its endpoint to confirm, so no message is being delivered to anyone. Confirm the subscriptions from their endpoints, or remove the ones that were never wanted. |
| sns.public-policy | topic policy open to anyone | broken | wave2 | The topic's access policy grants publish or subscribe to every AWS principal, so anyone can read what this topic broadcasts or inject messages its subscribers will trust. Scope the policy's Principal to the accounts and roles that actually use the topic. |
| sns.no-kms | not encrypted with KMS | warn | wave2 | Messages sit unencrypted in the topic, so anyone who reaches the backing storage or a raw log of it reads their contents. Set a KMS key on the topic so AWS encrypts each message at rest. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| alarm | CloudWatch Alarms | yes |
| sns-sub | Subscriptions | yes |
| kms | KMS Key | no |
| role | IAM Role | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
