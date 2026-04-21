---
shortName: sns
name: SNS Topics
awsApiRef: https://docs.aws.amazon.com/sns/latest/api/API_Topic.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# sns — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `sns`
- **Display name**: SNS Topics
- **AWS API reference**: <https://docs.aws.amazon.com/sns/latest/api/API_Topic.html>
- **List API**: `ListTopics` (returns `Topic.TopicArn` only — no attributes, no state field)
- **Describe API (if any)**: `GetTopicAttributes` (per topic, Wave 2)

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `ct-events`, `kms`, `role`, `sns-sub`.

### `alarm`

- **Why related**: CloudWatch alarms that notify this topic — `MetricAlarm.AlarmActions` / `OKActions` / `InsufficientDataActions` contain SNS topic ARNs. Primary incident pivot: "which alarms route to this channel?" (related-resources.md §`sns`, line 919; §`alarm` row, line 144).
- **How discovered**: cross-reference the already-loaded `alarm` list by matching the topic's `TopicArn` against any entry in each alarm's `AlarmActions` / `OKActions` / `InsufficientDataActions` — a9s-devops: standard list-scan, no extra API call needed since alarms are loaded in the same sweep.
- **Count shown**: yes — a9s-devops: number of alarms fanning into the topic is operationally meaningful (noisy channel detection).

### `ct-events`

- **Why related**: audit trail for topic changes (CreateTopic, SetTopicAttributes, DeleteTopic). Universal pivot — applies to every registered type; see related-resources.md §Policy §4.
- **How discovered**: universal — framework-level pivot, no per-type discovery logic.
- **Count shown**: unknown — not specified in related-resources.md.

### `kms`

- **Why related**: SSE-KMS encryption key — `GetTopicAttributes` returns `KmsMasterKeyId` when server-side encryption is enabled (related-resources.md §`sns` line 921; SDK `sns.GetTopicAttributesOutput` § `Attributes["KmsMasterKeyId"]`).
- **How discovered**: read `Attributes["KmsMasterKeyId"]` on the `GetTopicAttributes` response already fetched in Wave 2; match the returned key ID/ARN against the loaded `kms` list.
- **Count shown**: unknown — a topic references at most one KMS key, so the count is degenerate (0 or 1).

### `role`

- **Why related**: IAM principals granted publish/subscribe/manage permissions by the topic's resource policy — `GetTopicAttributes` returns the access-control document in `Attributes["Policy"]` as JSON, whose `Statement[].Principal` commonly lists role ARNs (related-resources.md §`sns` line 922; SDK `sns.GetTopicAttributesOutput` § `Attributes["Policy"]`) — a9s-devops: used during IAM audits to answer "who can publish to this topic?"; the 1/6-audit rationale in the golden doc is weak but the workflow is real.
- **How discovered**: parse `Attributes["Policy"]` JSON from `GetTopicAttributes` (Wave 2), extract each `Statement[].Principal.AWS` ARN whose ARN type is `role`, and cross-reference against the loaded `role` list — a9s-devops: JSON parse is cheap; skip silently if policy is absent (no statement → no principals → empty list).
- **Count shown**: unknown — not specified in related-resources.md.

### `sns-sub`

- **Why related**: subscriptions delivering messages off this topic — the core consumer-side pivot. "What's listening on this topic?" is the first question when publish latency or failed-delivery counts spike (related-resources.md §`sns` line 923; §`sns-sub` line 931).
- **How discovered**: call `ListSubscriptionsByTopic(TopicArn)` — a9s-devops: this is the dedicated SNS API for the relationship; no cheaper list-scan path exists because `ListSubscriptions` is account-wide and paginated.
- **Count shown**: yes — a9s-devops: fanout width (number of confirmed subscriptions) is decision-useful at a glance.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — the list API does not return fields usable for attention. `ListTopics` returns `Topic.TopicArn` only (SDK `sns/types.Topic`), no state field, no attribute map.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: `SubscriptionsConfirmed==0 AND SubscriptionsPending==0` → Warning (orphan topic).
  - **State bucket**: Warning (informational — the topic is Healthy in the AWS-state sense, but operationally orphaned).
  - **API call**: `GetTopicAttributes` — one call per topic.
  - **Cost shape**: per-resource.
- **Signal**: `KmsMasterKeyId` absent on sensitive topic → Warning.
  - **State bucket**: Warning.
  - **API call**: `GetTopicAttributes` — one call per topic (same call as above; no added cost).
  - **Cost shape**: per-resource.
  - **Note**: `attention-signals.md` does not define what makes a topic "sensitive" — the detection heuristic (tag, name pattern, policy content) is unspecified. See §5 Out of Scope. — a9s-devops: there is no reliable AWS-surface field identifying sensitivity from SNS alone; viable heuristics are tag-based (e.g. `sensitive=true`) or name-regex, both of which are per-deployment conventions. Worth=no as a universal default rule.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `NumberOfNotificationsFailed` — per-topic metric, rate-limited, outside Wave 2 budget.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". Never appears on yellow/red/dim rows. |
| S4 | Status / description column text | Short human-readable cause. Healthy rows render blank. |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping applied to `sns`:

- No Wave 1 signals → every topic row starts Healthy (green) with S4 blank until Wave 2 returns.
- Wave 2 orphan-topic finding is informational on a Healthy resource → `~` glyph on the green row (S3), short cause in S4, full sentence in S5. No S1 bump.
- Wave 2 missing-KMS finding is likewise informational `~` — but the "sensitive topic" trigger is not specified; this row exists to document the gap, not to drive implementation.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `SubscriptionsConfirmed==0 && SubscriptionsPending==0` | 2 | Warning | `~` | S3, S4, S5 | `no subscribers` | `Topic has no confirmed or pending subscriptions — published messages are discarded.` |
| `KmsMasterKeyId absent on sensitive topic` | 2 | Warning | `~` | S3, S4, S5 (pending trigger definition) | `not encrypted` | `Topic has no KMS master key configured — server-side encryption is not in effect.` |

## 4.1 UX review

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for the orphan-topic row — `~ no subscribers` on a green row is self-explanatory. The missing-KMS row is only actionable if the "sensitive topic" trigger is first defined (tag? name regex?); until then the spec row is documentation, not an implementation target — implementers must resolve the trigger before surfacing this finding, otherwise every unencrypted topic in the account lights up with `~ not encrypted` and the signal degrades to noise.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- `KmsMasterKeyId absent on sensitive topic` — a9s-devops: not worth it as a universal default. SNS has no reliable AWS-surface field identifying "sensitive"; viable heuristics (tag `sensitive=true`, name regex matching `prod|pii|secret`, policy-content scan) are per-deployment conventions and produce noisy signal at the account level. Recommend deferring until an explicit trigger is specified in `attention-signals.md`.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?", line 13).

## 6. Citations

- Identity — List API `ListTopics` returns ARN only — `AWS SDK Go v2 — sns/types.Topic § TopicArn`.
- Identity — Describe API `GetTopicAttributes` returns attribute map incl. `SubscriptionsConfirmed`, `SubscriptionsPending`, `KmsMasterKeyId`, `Policy` — `AWS SDK Go v2 — sns.GetTopicAttributesOutput § Attributes`.
- §2 related targets — contract row — `docs/related-resources.md` § Per-type contract § `sns` (line 100) and § `sns` section (lines 915-923).
- §2 `alarm` why related — SNS topic ARNs live on `MetricAlarm.AlarmActions`/`OKActions` — `docs/related-resources.md` § `alarm` (line 144).
- §2 `alarm` how discovered — a9s-devops (2026-04-20): possible=yes, worth=yes. CloudWatch alarms already loaded in the sweep carry the SNS ARNs in their action-ARN arrays; a single list-scan cross-reference. Standard incident pivot.
- §2 `alarm` count shown — a9s-devops (2026-04-20): possible=yes, worth=yes. Fanout width is operationally meaningful — a topic with 40 alarms is a noisy channel.
- §2 `ct-events` universal pivot — `docs/related-resources.md` § Policy §4 (line 34).
- §2 `ct-events` count shown — golden docs silent, marked unknown.
- §2 `kms` why related — `docs/related-resources.md` § `sns` (line 921) and `AWS SDK Go v2 — sns.GetTopicAttributesOutput § Attributes["KmsMasterKeyId"]`.
- §2 `kms` how discovered — a9s-devops (2026-04-20): possible=yes, worth=yes. `GetTopicAttributes` already called in Wave 2 carries `KmsMasterKeyId`; cross-reference against the loaded `kms` list. No extra API call.
- §2 `role` why related — `docs/related-resources.md` § `sns` (line 922) and `AWS SDK Go v2 — sns.GetTopicAttributesOutput § Attributes["Policy"]`.
- §2 `role` how discovered — a9s-devops (2026-04-20): possible=yes, worth=yes-but-marginal. Parse topic `Policy` JSON (already on Wave 2 response), extract role ARNs from `Statement[].Principal.AWS`, cross-reference against loaded `role` list. Golden-doc rationale is weak ("1/6 audits") but the IAM-audit workflow exists.
- §2 `sns-sub` why related — `docs/related-resources.md` § `sns` (line 923) and § `sns-sub` (line 931).
- §2 `sns-sub` how discovered — a9s-devops (2026-04-20): possible=yes, worth=yes. `ListSubscriptionsByTopic(TopicArn)` is the dedicated SNS API; no cheaper path — `ListSubscriptions` is account-wide and paginated with no topic filter.
- §2 `sns-sub` count shown — a9s-devops (2026-04-20): possible=yes, worth=yes. Fanout-width is a primary decision signal for an SNS operator.
- §3.1 no Wave 1 signals — `docs/attention-signals.md` § Messaging § `sns` row (line 83, Wave 1 cell: "None — `ListTopics` returns ARN only").
- §3.2 orphan-topic signal — `docs/attention-signals.md` § Messaging § `sns` row (line 83, Wave 2 cell).
- §3.2 missing-KMS signal — `docs/attention-signals.md` § Messaging § `sns` row (line 83, Wave 2 cell). Trigger definition for "sensitive topic" not specified.
- §3.2 `SubscriptionsConfirmed`/`SubscriptionsPending`/`KmsMasterKeyId` field names — `AWS SDK Go v2 — sns.GetTopicAttributesOutput § Attributes` doc comment.
- §3.3 Wave 3 CloudWatch metric — `docs/attention-signals.md` § Messaging § `sns` row (line 83, Wave 3 cell).
- §4 orphan-topic S4/S5 wording — a9s-devops (2026-04-20): possible=yes, worth=yes. `no subscribers` on the row lets an operator triage without drilling in; S5 spells out the consequence (messages discarded) for the detail view.
- §5 missing-KMS deferral — a9s-devops (2026-04-20): possible=yes-via-tags-or-regex, worth=no as a universal default. Without an explicit "sensitive" trigger the rule fires on every unencrypted topic, producing noise. Defer until `attention-signals.md` defines the trigger.
- §5 read-only invariant — `docs/architecture.md` § "What is a9s?" (line 15).
