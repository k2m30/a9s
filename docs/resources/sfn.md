---
shortName: sfn
name: Step Functions
awsApiRef: https://docs.aws.amazon.com/step-functions/latest/apireference/API_StateMachineListItem.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# sfn — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `sfn`
- **Display name**: Step Functions
- **AWS API reference**: <https://docs.aws.amazon.com/step-functions/latest/apireference/API_StateMachineListItem.html>
- **List API**: `ListStateMachines` — returns `StateMachineListItem[]` with `Name`, `StateMachineArn`, `Type` (STANDARD | EXPRESS), `CreationDate`. Per `attention-signals.md`, this list is config-only: the summary carries no status field, so **no Wave 1 health signal is reachable from the list alone**.
- **Describe API (if any)**: Two calls, both per-state-machine, used in Wave 2 / related discovery:
  - `ListExecutions(statusFilter=FAILED, maxResults=1)` — the only Wave 2 attention call; returns the most recent failed execution so a9s can flag recent-failure or consecutive-failure state.
  - `DescribeStateMachine` — used **only** to discover related targets (`role`, `kms`, `logs`, `lambda`); never used to produce attention signals. Returns `RoleArn`, `EncryptionConfiguration.KmsKeyId`, `LoggingConfiguration.Destinations[].CloudWatchLogsLogGroup.LogGroupArn`, and the ASL `Definition` string.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `eb-rule`, `kms`, `lambda`, `logs`, `role`, `ct-events`.

### `alarm`

- **Why related**: Execution-failure alarms — `docs/related-resources.md` § Per-type contract `sfn` → "Execution-failure alarms."
- **How discovered**: cross-reference the already-loaded `alarm` list; keep alarms whose `Dimensions[].Name == "StateMachineArn"` AND `Dimensions[].Value == <this state machine's ARN>`. No extra API call — a9s-devops: this is the canonical CloudWatch dimension for Step Functions metrics (`ExecutionsFailed`, `ExecutionsTimedOut`, `ExecutionThrottled` all live in the `AWS/States` namespace and are dimensioned by `StateMachineArn`). Operators wire these alarms during onboarding so an SFN failure pages the on-call; showing them on the detail view answers "is anything watching this state machine?".
- **Count shown**: yes.

### `eb-rule`

- **Why related**: EventBridge rules with this state machine as target — `docs/related-resources.md` § Per-type contract `sfn` → "EventBridge rules with this state machine as target."
- **How discovered**: reverse-scan the already-loaded `eb-rule` list (targets are resolved Wave 2 for `eb-rule`); keep rules whose `Targets[].Arn == <this state machine's ARN>` — a9s-devops: EventBridge → SFN wiring lives on the rule's target list; the state-machine side has no back-reference. This is the same reverse-scan pattern `pipeline` uses for `eb-rule`. Operator workflow: "which scheduled or event-driven trigger kicks off this state machine?".
- **Count shown**: yes.

### `kms`

- **Why related**: Execution-data encryption — `docs/related-resources.md` § Per-type contract `sfn` → "Execution-data encryption."
- **How discovered**: call `DescribeStateMachine`, read `EncryptionConfiguration.KmsKeyId` (`AWS SDK Go v2 — service/sfn.DescribeStateMachineOutput § EncryptionConfiguration`, `service/sfn/types.EncryptionConfiguration § KmsKeyId`); when set, cross-reference the loaded `kms` list by key ID / alias / ARN — a9s-devops: CMK-encrypted state machines are the common setup in regulated accounts; when `EncryptionConfiguration` is nil or `Type == AWS_OWNED_KEY`, the target legitimately shows 0 (AWS-owned keys do not appear in `kms`). Operator needs the pivot during key-rotation or pending-deletion response.
- **Count shown**: yes.

### `lambda`

- **Why related**: Lambda integrations invoked by the state machine — `docs/related-resources.md` § Per-type contract `sfn` → "Lambda integrations invoked by the state machine."
- **How discovered**: call `DescribeStateMachine`, parse `Definition` (ASL JSON); walk every `States.*.Type == "Task"` and collect `Resource` values matching `arn:aws:lambda:*:*:function:<name>` (direct invoke) or `arn:aws:states:::lambda:invoke` + `Parameters.FunctionName` (optimized integration); cross-reference the loaded `lambda` list by function name — a9s-devops: these two patterns cover >99% of real SFN→Lambda wiring. The optimized-integration form became the default in the console several years ago; the direct-ARN form still appears in older workflows. Parsing the ASL is the only way — there is no structured "integrations" field on any SFN API. Operator workflow during an incident: "which lambdas does this state machine actually call?".
- **Count shown**: yes.

### `logs`

- **Why related**: Execution log groups — `docs/related-resources.md` § Per-type contract `sfn` → "Execution log groups."
- **How discovered**: call `DescribeStateMachine`, read `LoggingConfiguration.Destinations[].CloudWatchLogsLogGroup.LogGroupArn` (`AWS SDK Go v2 — service/sfn.DescribeStateMachineOutput § LoggingConfiguration`); strip the ARN to log-group name and cross-reference the loaded `logs` list — a9s-devops: SFN logging is opt-in per-state-machine and is the primary channel for debugging STANDARD-type workflows. EXPRESS workflows require logging for any post-hoc debugging at all (history is not retained). When `LoggingConfiguration == nil` or `Destinations` is empty, the target shows 0; that itself is a useful observation for STANDARD workflows where logging is usually expected.
- **Count shown**: yes.

### `role`

- **Why related**: `StateMachine.RoleArn` — execution role — `docs/related-resources.md` § Per-type contract `sfn` → "StateMachine.RoleArn — execution role."
- **How discovered**: call `DescribeStateMachine`, read `RoleArn` (`AWS SDK Go v2 — service/sfn.DescribeStateMachineOutput § RoleArn` — required field); extract role name from the ARN and cross-reference the loaded `role` list — a9s-devops: the execution role is the single most interrogated attribute during an SFN failure incident ("does the role have permission for the Lambda / DynamoDB / SNS action the workflow is trying to invoke?"). Always exactly one role per state machine; count is always 0 or 1.
- **Count shown**: yes.

### `ct-events`

- **Why related**: universal pivot — applies to every registered type; see related-resources.md §Policy. Lets the operator audit "who changed this state machine and when" during an incident (definition updates, role changes, encryption-config flips).
- **How discovered**: `cloudtrail:LookupEvents` filtered by the state machine's ARN / name.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — the list API does not return fields usable for attention.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: `ListExecutions(statusFilter=FAILED, maxResults=1)` returns any recent failure.
  - **State bucket**: Warning.
  - **API call**: `ListExecutions` — one per state machine (N+1).
  - **Cost shape**: per-resource.
- **Signal**: `ListExecutions(statusFilter=FAILED, maxResults=1)` shows consecutive failures (the most recent failed execution is newer than the most recent succeeded execution across multiple runs — interpreted as an ongoing failure loop).
  - **State bucket**: Broken.
  - **API call**: `ListExecutions` — one per state machine (N+1); may call twice with differing `statusFilter` values to confirm no intervening success.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `ExecutionsFailed` trend.
- OUT OF SCOPE: CloudWatch `ExecutionsTimedOut` trend.
- OUT OF SCOPE: CloudWatch `ExecutionThrottled` trend.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing." **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause. **Healthy rows render blank** — no `OK` / `ACTIVE` / `running`. |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4, S5.
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph on green row. S3, S4, S5. No S1.
- **Wave 2 finding on an already yellow/red/dim row** → S3 suppressed; S4 deduplicates with any existing cause; S5 still carries the full sentence; S1 still counts if `!`.

Because Wave 1 is absent for `sfn`, **every `sfn` row is Healthy by color** until Wave 2 finishes. The `!` / `~` annotation is the only attention signal this resource produces.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| Recent failed execution (single) | 2 | Warning | `~` | S3, S4, S5 | `last run failed` | `Last execution failed — check execution history for the ARN and error cause.` |
| Consecutive failed executions (failure loop) | 2 | Broken | `!` | S1, S3, S4, S5 | `failing: consecutive failures` | `Consecutive executions have failed with no successful run in between — IAM, definition, or downstream-dep regression likely.` |

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — the `!` glyph plus the `failing: consecutive failures` Status text names the condition without requiring detail-view navigation, and the `~` + `last run failed` pair is similarly self-explanatory for the single-failure case; the operator still needs the detail view to read the failing ExecutionArn and error cause, which is the expected next step.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- Parsing the ASL `Definition` to expose anything beyond the `lambda` pivot (e.g., DynamoDB tables, SNS topics, SQS queues, nested state machines referenced as Task resources) — a9s-devops: possible=yes, worth=no. The related-resources.md contract for `sfn` lists only `lambda` from the ASL; widening the parse would require cross-referencing sibling lists that aren't in the contract and risks a combinatorial blow-up of related-panel entries.
- Per-execution drill-down (individual execution ARN → input / output / history). a9s-devops: possible=yes, worth=no for v1 — this is a detail-view feature that belongs in a future "execution browser" child view, not the main list; CloudTrail and the Console cover the incident-response workflow today.

## 6. Citations

- `List API returns name/ARN/type/creationDate only, no status` — `AWS SDK Go v2 — service/sfn/types.StateMachineListItem § Name/StateMachineArn/Type/CreationDate`.
- `Describe returns RoleArn, EncryptionConfiguration, LoggingConfiguration, Definition` — `AWS SDK Go v2 — service/sfn.DescribeStateMachineOutput § RoleArn, EncryptionConfiguration, LoggingConfiguration, Definition`.
- `ExecutionStatus enum is RUNNING | SUCCEEDED | FAILED | TIMED_OUT | ABORTED | PENDING_REDRIVE` — `AWS SDK Go v2 — service/sfn/types.ExecutionStatus`.
- `StateMachineStatus enum is ACTIVE | DELETING` (not used as an attention source because the signals doc bypasses it) — `AWS SDK Go v2 — service/sfn/types.StateMachineStatus`.
- `Related targets: alarm, ct-events, eb-rule, kms, lambda, logs, role` — `docs/related-resources.md` § Per-type contract `sfn`.
- `alarm — Execution-failure alarms` — `docs/related-resources.md` § `sfn` list.
- `eb-rule — EventBridge rules with this state machine as target` — `docs/related-resources.md` § `sfn` list.
- `kms — Execution-data encryption` — `docs/related-resources.md` § `sfn` list.
- `lambda — Lambda integrations invoked by the state machine` — `docs/related-resources.md` § `sfn` list.
- `logs — Execution log groups` — `docs/related-resources.md` § `sfn` list.
- `role — StateMachine.RoleArn — execution role` — `docs/related-resources.md` § `sfn` list.
- `alarm discovery uses StateMachineArn dimension` — `a9s-devops (2026-04-20): possible=yes, worth=yes. AWS/States metrics are dimensioned by StateMachineArn; reverse-scan the loaded alarm list — no extra API call.`
- `eb-rule reverse-scan via Targets[].Arn` — `a9s-devops (2026-04-20): possible=yes, worth=yes. EventBridge → SFN wiring lives on the rule's target list; no back-reference on the state-machine side. Same pattern used for pipeline/eb-rule.`
- `kms discovered via DescribeStateMachine.EncryptionConfiguration.KmsKeyId` — `a9s-devops (2026-04-20): possible=yes, worth=yes. CMK-encrypted SFN is standard in regulated accounts; pivot matters during key-rotation and pending-deletion response. AWS-owned-key workflows show 0 legitimately.`
- `lambda discovered by parsing ASL Definition for Task resources` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Both the direct arn:aws:lambda:...:function:... form and the optimized arn:aws:states:::lambda:invoke + Parameters.FunctionName form are documented patterns covering >99% of workflows. No structured integrations field exists on any SFN API.`
- `logs discovered via DescribeStateMachine.LoggingConfiguration.Destinations[]` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Opt-in per-state-machine; primary debugging channel for STANDARD; required for EXPRESS debugging at all.`
- `role discovered via DescribeStateMachine.RoleArn` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Required field; count always 0 or 1; single most interrogated attribute during an SFN incident.`
- `Wave 2 signal: recent failure → Warning, consecutive failures → Broken` — `docs/attention-signals.md` § Messaging row `sfn`.
- `Wave 3 signals: CloudWatch ExecutionsFailed/ExecutionsTimedOut/ExecutionThrottled trends` — `docs/attention-signals.md` § Messaging row `sfn`.
- `ListExecutions is the supported API for failure enumeration` — `docs/attention-signals.md` § Messaging row `sfn` Source column: [ListExecutions](https://docs.aws.amazon.com/step-functions/latest/apireference/API_ListExecutions.html).
- `S4 wording "last run failed" and "failing: consecutive failures", S5 sentences` — `a9s-devops (2026-04-20): possible=yes, worth=yes. State keywords alone violate the skill's "state keywords are not explanations" rule; pairing state with an operator-readable cue ("last run failed", "consecutive failures") plus a detail sentence that hints at the likely class of root cause (IAM / definition / downstream) matches the §4 surface rules and the 3am test.`
- `Out-of-scope: ASL parse beyond lambda` — `a9s-devops (2026-04-20): possible=yes, worth=no. related-resources.md contract restricts ASL-derived targets to lambda; widening violates the contract and risks combinatorial panel growth.`
- `Out-of-scope: per-execution drill-down` — `a9s-devops (2026-04-20): possible=yes, worth=no for v1. Belongs in a future execution-browser child view; CloudTrail + Console cover incident workflow today.`
- `Read-only invariant` — `docs/architecture.md` § "What is a9s?".
