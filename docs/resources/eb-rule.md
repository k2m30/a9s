---
shortName: eb-rule
name: EventBridge Rules
awsApiRef: https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_Rule.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# eb-rule — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `eb-rule`
- **Display name**: EventBridge Rules
- **AWS API reference**: <https://docs.aws.amazon.com/eventbridge/latest/APIReference/API_Rule.html>
- **List API**: `ListRules` — returns `[]Rule`. The SDK confirms `Name`, `State`, `EventBusName`, `EventPattern`, `ScheduleExpression`, `Description`, `RoleArn`, and `Arn` all ride on the `Rule` shape, so every Wave 1 signal is reachable with zero extra calls.
- **Describe API (if any)**: `ListTargetsByRule` per rule — used in Wave 2 to read `Targets[]`, `Targets[].Arn`, and `Targets[].DeadLetterConfig`, none of which are on `ListRules`.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `kinesis`, `lambda`, `logs`, `role`, `sfn`, `sns`, `sqs`, `ct-events`.

### `kinesis`

- **Why related**: Kinesis data stream that is a target of this rule — answers "what stream does this event route to?".
- **How discovered**: call `ListTargetsByRule`, read `Targets[].Arn`, keep entries whose ARN prefix is `arn:aws:kinesis:*:*:stream/`; cross-reference the already-loaded `kinesis` list by stream ARN — a9s-devops: Target ARN prefix is the only reliable per-target service discriminator because EventBridge has no typed `TargetType` field; the stream segment is the canonical split.
- **Count shown**: yes.

### `lambda`

- **Why related**: Lambda function invoked when this rule matches — the single most common EventBridge target, the usual answer to "what runs on this schedule?".
- **How discovered**: call `ListTargetsByRule`, read `Targets[].Arn`, keep entries whose ARN prefix is `arn:aws:lambda:*:*:function:`; cross-reference the already-loaded `lambda` list by function ARN — a9s-devops: same ARN-prefix split; function ARNs may include a version or alias suffix which must be trimmed before matching.
- **Count shown**: yes.

### `logs`

- **Why related**: CloudWatch Logs log group that is the target of this rule — used for event archival, audit fan-out, or "put-events → logs" debug paths.
- **How discovered**: call `ListTargetsByRule`, read `Targets[].Arn`, keep entries whose ARN prefix is `arn:aws:logs:*:*:log-group:`; cross-reference the already-loaded `logs` list by log-group ARN — a9s-devops: ARN-prefix split. Log-group ARNs end in `:*` on the target side, which should be stripped before matching loaded entries.
- **Count shown**: yes.

### `role`

- **Why related**: IAM role that EventBridge assumes to deliver events to the target — if the rule is misfiring because of permissions, the role is where the operator starts looking.
- **How discovered**: read `Rule.RoleArn` on the already-loaded `eb-rule` entry (zero-call, no `ListTargetsByRule` needed); cross-reference the already-loaded `role` list by role ARN. Per-target overrides on `Targets[].RoleArn` are additional pivots discovered during the Wave 2 targets fetch — a9s-devops: the rule-level `RoleArn` is the dominant case; per-target `RoleArn` is rare but real when one rule fans out to targets with different permission boundaries.
- **Count shown**: yes.

### `sfn`

- **Why related**: Step Functions state machine that this rule invokes — typical schedule-driven workflow starter.
- **How discovered**: call `ListTargetsByRule`, read `Targets[].Arn`, keep entries whose ARN prefix is `arn:aws:states:*:*:stateMachine:`; cross-reference the already-loaded `sfn` list by state-machine ARN — a9s-devops: ARN-prefix split.
- **Count shown**: yes.

### `sns`

- **Why related**: SNS topic that this rule publishes events to for fan-out.
- **How discovered**: call `ListTargetsByRule`, read `Targets[].Arn`, keep entries whose ARN prefix is `arn:aws:sns:*:*:`; cross-reference the already-loaded `sns` list by topic ARN — a9s-devops: ARN-prefix split; SNS topic ARNs have no sub-path segment after the account ID.
- **Count shown**: yes.

### `sqs`

- **Why related**: SQS queue that this rule delivers events to — either the primary target, or (importantly) the dead-letter queue configured on a target via `DeadLetterConfig.Arn`. Both are worth a pivot because "events are piling up in an SQS queue" is the first symptom of a downstream-consumer failure.
- **How discovered**: call `ListTargetsByRule`, take both `Targets[].Arn` whose prefix is `arn:aws:sqs:*:*:` and `Targets[].DeadLetterConfig.Arn`; cross-reference the already-loaded `sqs` list by queue ARN, de-duplicated — a9s-devops: surface DLQ ARNs alongside primary targets so the operator can jump straight to the queue where undelivered events land.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Universal pivot — who created, enabled, disabled, or modified this rule; also captures `PutRule` / `PutTargets` audit history.
- **How discovered**: pre-built CloudTrail query scoped to the rule ARN as the resource identifier.
- **Count shown**: unknown (CloudTrail queries are windowed; a reliable total isn't available without a separate count call).
- Universal pivot — applies to every registered type; see `related-resources.md` §Policy.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `State == ENABLED`.
  - **State bucket**: Healthy.
  - **How obtained**: `Rule.State` from `ListRules`.

- **Signal**: `State == ENABLED_WITH_ALL_CLOUDTRAIL_MANAGEMENT_EVENTS`.
  - **State bucket**: Healthy.
  - **How obtained**: `Rule.State` from `ListRules`.

- **Signal**: `State == DISABLED`.
  - **State bucket**: Dim (admin-off).
  - **How obtained**: `Rule.State` from `ListRules`.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: rule `State == ENABLED` AND `len(Targets) == 0` — the rule matches but has nowhere to route.
  - **State bucket**: Broken.
  - **API call**: `ListTargetsByRule` — one per rule.
  - **Cost shape**: per-resource.

- **Signal**: rule `State == DISABLED` AND `len(Targets) > 0` — probable oversight, a rule was turned off but its wiring left in place.
  - **State bucket**: Warning.
  - **API call**: `ListTargetsByRule` — one per rule.
  - **Cost shape**: per-resource.

- **Signal**: any `Targets[].DeadLetterConfig` unset — failed deliveries will be silently dropped.
  - **State bucket**: Warning.
  - **API call**: `ListTargetsByRule` — one per rule (same call that serves the two signals above; cost is shared).
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `FailedInvocations` / `ThrottledRules` per rule.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing" — e.g. maintenance scheduled, certificate expiring soon. `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `stopping: Server.SpotInstanceShutdown`, `expires in 7d`). **Healthy rows render blank** — no `OK` / `available` / `ACTIVE` / `running`. Empty means "nothing to see." |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank. Silence is the UX.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4 (short cause), S5 (full sentence).
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph on green row. S3, S4 (short cause), S5 (full sentence). No S1.
- **Wave 2 finding on an already yellow/red/dim row** → redundant with color; S3 suppressed, S4 deduplicates with existing cause, S5 still carries the full sentence, S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `State == DISABLED` | 1 | Dim | n/a | S2, S4 | `disabled — admin-off` | `Rule disabled by an operator; matches no events until re-enabled.` |
| `ENABLED` rule with `len(Targets)==0` | 2 | Broken | `!` | S1, S2, S4, S5 | `no targets — events dropped` | `Rule is enabled but has no targets; every matched event is silently discarded.` |
| `DISABLED` rule with `len(Targets)>0` | 2 | Dim + finding | `!` | S1, S4 (dedup), S5 | `disabled but still wired to N targets` | `Rule is disabled yet still has targets attached — probable stale wiring from a half-done change.` |
| target without `DeadLetterConfig` | 2 | Warning | `~` | S3 (if rule row is green), S4, S5 | `N target(s) have no DLQ` | `One or more targets lack DeadLetterConfig; failed deliveries will be dropped without trace.` |

Notes on the table above:

- The `DISABLED` + targets signal lands on a row that is already Dim (gray). Per the Wave → surface mapping, S3 is suppressed on non-green rows; the `!` still bumps S1, and S4 deduplicates with the bare `disabled — admin-off` cause by appending `— still wired to N targets`. S5 carries the full sentence.
- The missing-DLQ signal is the only `~` on `eb-rule` — it's informational background hygiene, not a rule-is-broken event. It gets a glyph only when the row is green (ENABLED and has targets); on yellow/red/dim rows the signal still records in S5 but no glyph is painted.
- Severity choice (`!` for no-targets and disabled-with-targets, `~` for missing DLQ) — a9s-devops: no-targets on an enabled rule and stale disabled-but-wired rules are both "something is broken right now"; missing DLQ is a latent risk that only matters when another failure occurs, which is textbook `~`.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — a broken enabled rule reads `no targets — events dropped` in red, a stale disabled rule reads `disabled but still wired to N targets` in gray with `!`, and a missing-DLQ finding reads `N target(s) have no DLQ` next to a `~` on green; all three are triageable in the list without navigating to detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- Per-target typed discriminator (target kind without parsing ARN prefixes) — a9s-devops: not worth it, EventBridge does not expose a typed `TargetType` field; ARN-prefix parsing is the idiomatic and only reliable split, so introducing a custom classifier would add complexity without new information.
- Rule-per-custom-event-bus coverage map — a9s-devops: not worth it, the `EventBusName` field already surfaces on the list row and the related panel covers the per-target pivots; building a second view keyed by event bus would duplicate information already reachable by filtering.

## 6. Citations

- a9s golden doc — `eb-rule` is in the Per-type contract with targets `ct-events, kinesis, lambda, logs, role, sfn, sns, sqs` — `docs/related-resources.md` § Per-type contract, row `eb-rule`.
- a9s golden doc — per-target reasoning (`kinesis`, `lambda`, `logs`, `role`, `sfn`, `sns`, `sqs`, `ct-events`) — `docs/related-resources.md` § `eb-rule` (lines 358–369).
- a9s golden doc — Wave 1/2/3 signals for `eb-rule` — `docs/attention-signals.md` § Messaging & Events table, row `eb-rule`.
- a9s golden doc — read-only invariant — `docs/architecture.md` § "What is a9s?".
- AWS Go SDK v2 — `Rule.State`, `Rule.Name`, `Rule.EventPattern`, `Rule.ScheduleExpression`, `Rule.EventBusName`, `Rule.RoleArn`, `Rule.Arn`, `Rule.Description` all on the `Rule` shape returned by `ListRules` — `AWS SDK Go v2 — eventbridge/types.Rule`.
- AWS Go SDK v2 — `Target.Arn`, `Target.DeadLetterConfig`, `Target.RoleArn` are on the `Target` shape returned by `ListTargetsByRule` — `AWS SDK Go v2 — eventbridge/types.Target`.
- AWS Go SDK v2 — `DeadLetterConfig.Arn` points to the SQS queue used as DLQ for a target — `AWS SDK Go v2 — eventbridge/types.DeadLetterConfig § Arn`.
- AWS Go SDK v2 — `RuleState` enum: `ENABLED`, `DISABLED`, `ENABLED_WITH_ALL_CLOUDTRAIL_MANAGEMENT_EVENTS` — `AWS SDK Go v2 — eventbridge/types.RuleState`.
- a9s-devops consultation — ARN-prefix split is the only reliable per-target service discriminator (kinesis, lambda, logs, sfn, sns, sqs targets) — `a9s-devops (2026-04-20): possible=yes, worth=yes. EventBridge Target has no typed kind field; ARN prefix is canonical.`
- a9s-devops consultation — include `Targets[].DeadLetterConfig.Arn` in the `sqs` related pivot — `a9s-devops (2026-04-20): possible=yes, worth=yes. DLQs are the first place to look when events are failing delivery; surfacing them alongside primary SQS targets saves a hop.`
- a9s-devops consultation — `Rule.RoleArn` is the dominant IAM pivot; per-target `Targets[].RoleArn` is a rare override — `a9s-devops (2026-04-20): possible=yes, worth=yes. Both fields are standard; rule-level role is zero-call from ListRules, per-target role is read-along with other Wave 2 target fields.`
- a9s-devops consultation — severity mapping (`!` for no-targets and disabled-with-targets, `~` for missing DLQ) — `a9s-devops (2026-04-20): possible=yes, worth=yes. The first two are live broken states; DLQ-missing is latent risk that only manifests on another failure — textbook informational glyph.`
- a9s-devops consultation — per-target typed discriminator not worth introducing — `a9s-devops (2026-04-20): possible=no (no AWS field), worth=no. ARN-prefix parsing is idiomatic.`
- a9s-devops consultation — rule-per-bus coverage map not worth adding — `a9s-devops (2026-04-20): possible=yes, worth=no. EventBusName is already on the list row and per-target pivots already cover the drill-down workflow.`

<!-- BEGIN GENERATED: header -->
eb-rule — MESSAGING. Lifecycle key: `state`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Approximate? |
| --- | --- | --- |
| role | IAM Role | no |
| kinesis | Kinesis (targets) | no |
| lambda | Lambda (targets) | no |
| logs | Log Groups (targets) | no |
| sfn | Step Functions (targets) | no |
| sns | SNS (targets) | no |
| sqs | SQS (targets) | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
