---
shortName: cfn
name: CloudFormation Stacks
awsApiRef: https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_Stack.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# cfn — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `cfn`
- **Display name**: CloudFormation Stacks
- **AWS API reference**: <https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_Stack.html>
- **List API**: `DescribeStacks`
- **Describe API (if any)**: `DescribeStackEvents` (Wave 2 — one per stack, client-side scan of the first page of recent events)

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `cfn`, `ct-events`, `eb-rule`, `role`, `s3`, `sns`.

### `cfn`

- **Why related**: Nested stacks — a stack can itself be a parent of child stacks or be nested under a parent. Operators debug nested-stack failures constantly because a parent `UPDATE_ROLLBACK_COMPLETE` often originates from a single child `*_FAILED`.
- **How discovered**: Read `ParentId` on the current stack to pivot to the parent; reverse-scan the already-loaded `cfn` list by `ParentId == <this stack's StackId>` to enumerate nested children. `RootId` gives the top-of-tree stack for multi-level nesting.
- **Count shown**: yes (parent is 0 or 1; nested-children count from the loaded stack list).

### `eb-rule`

- **Why related**: Stack-event publishing via EventBridge — when CloudFormation→EventBridge integration is on, stack lifecycle events fan out to EventBridge rules that ops teams route to Slack/PagerDuty.
- **How discovered**: Reverse-scan the already-loaded `eb-rule` list for rules whose event pattern references `source: aws.cloudformation` (and optionally filters on the stack ARN) — a9s-devops: no direct field on `Stack`, only reverse scan from the rule side.
- **Count shown**: yes, but only when the `eb-rule` list is already loaded for this region; otherwise `unknown` — a9s-devops: acceptable degradation, operator sees "—" and knows to list eb-rule first.

### `role`

- **Why related**: `Stack.RoleARN` is the IAM service role CloudFormation assumes to make changes; on a permission-related `*_FAILED` status this is the first resource an operator pivots to.
- **How discovered**: Read `RoleARN` on the stack; cross-reference the already-loaded `role` list by ARN.
- **Count shown**: yes (0 or 1).

### `s3`

- **Why related**: The stack template was uploaded from S3 (`TemplateURL` parameter on create/update); on drift or template-mystery debugging, ops want to find the source bucket. `NotificationARNs` can also point at SNS topics whose subscribers include S3 event destinations.
- **How discovered**: `TemplateURL` is **not** on the `Stack` response from `DescribeStacks` — it is on the `CreateStack`/`UpdateStack` input only. To resolve it post-hoc requires `GetTemplateSummary` (extra API call, not in Wave 2 budget). For this spec the S3 pivot is surfaced as "open the S3 list and search by stack tags" — a9s-devops: possible=partial, worth=yes for workflow, but not directly discoverable from the list row. Treat as a manual pivot, not a counted panel entry.
- **Count shown**: unknown — a9s-devops: no discoverable count without an extra Describe call.

### `sns`

- **Why related**: `Stack.NotificationARNs` are the SNS topics CloudFormation publishes stack events to; operators diagnosing "why didn't we get notified" or "which pager got this" pivot here.
- **How discovered**: Read `NotificationARNs` on the stack (list of ARNs); cross-reference the already-loaded `sns` list by ARN.
- **Count shown**: yes (length of `NotificationARNs`).

### `ct-events`

- **Why related**: Universal pivot — applies to every registered type; see `related-resources.md` §Policy 4. CloudTrail events filtered by `resources[].ARN == <StackId>` give the audit trail of who created/updated/rolled back the stack.
- **How discovered**: Query `LookupEvents` filtered on the stack ARN.
- **Count shown**: yes (event count).

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `StackStatus` ∈ {`CREATE_COMPLETE`, `UPDATE_COMPLETE`, `IMPORT_COMPLETE`, `UPDATE_COMPLETE_CLEANUP_IN_PROGRESS`} → Healthy.
  - **State bucket**: Healthy.
  - **How obtained**: `StackStatus` field on each element of the `DescribeStacks` response.
- **Signal**: `StackStatus` matches `*_IN_PROGRESS` or `REVIEW_IN_PROGRESS` → Warning (operation in flight).
  - **State bucket**: Warning.
  - **How obtained**: `StackStatus` field.
- **Signal**: `StackStatus == ROLLBACK_COMPLETE` → Warning (failed-create tombstone — stack operationally dead, delete-and-recreate required, but not actively failing).
  - **State bucket**: Warning.
  - **How obtained**: `StackStatus` field.
- **Signal**: `StackStatus` ∈ {`UPDATE_ROLLBACK_COMPLETE`, `IMPORT_ROLLBACK_COMPLETE`} → Warning (update failed, stack reverted to prior state).
  - **State bucket**: Warning.
  - **How obtained**: `StackStatus` field.
- **Signal**: `StackStatus` matches `*_FAILED` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `StackStatus` field + `StackStatusReason` carries the human cause.
- **Signal**: `StackStatus` matches `*_IN_PROGRESS` with stack in that state >1h → Broken (stuck).
  - **State bucket**: Broken.
  - **How obtained**: `StackStatus` field + `LastUpdatedTime` (or `CreationTime` when never updated) on the list response.
- **Signal**: `DriftInformation.StackDriftStatus == DRIFTED` → Warning (signal is low-coverage until `DetectStackDrift` has been run).
  - **State bucket**: Warning.
  - **How obtained**: `DriftInformation.StackDriftStatus` field on the list response.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: Recent stack event with `ResourceStatus == *_FAILED` → Broken.
  - **State bucket**: Broken.
  - **API call**: `DescribeStackEvents` — one per stack, take the first response page and scan client-side for the most recent `*_FAILED` event.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `DetectStackDrift` + `DescribeStackDriftDetectionStatus` (async polling) for fresh drift detection.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". Never appears on yellow/red/dim rows. |
| S4 | Status / description column text | Short human-readable cause (e.g. `rollback_complete: Resource <X> failed`, `update_failed: permission denied on iam:PassRole`). **Healthy rows render blank.** |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4 (short cause), S5 (full sentence).
- **Wave 2 finding on an already yellow/red/dim row** → redundant with color; S3 suppressed, S4 deduplicates with existing cause, S5 still carries the full sentence, S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `*_IN_PROGRESS` / `REVIEW_IN_PROGRESS` | 1 | Warning | n/a | S2, S4 | `update in progress` (or verb matching status) | `Stack operation <StackStatus> in progress since <LastUpdatedTime>.` |
| `ROLLBACK_COMPLETE` | 1 | Warning | n/a | S2, S4 | `rollback: failed create, delete required` | `Stack creation failed and rolled back; delete the stack and recreate it.` |
| `UPDATE_ROLLBACK_COMPLETE` / `IMPORT_ROLLBACK_COMPLETE` | 1 | Warning | n/a | S2, S4 | `update rolled back: <reason>` | `Update failed and reverted: <StackStatusReason>.` |
| `*_FAILED` | 1 | Broken | n/a | S2, S4 | `failed: <StackStatusReason short>` | `Stack <StackStatus>: <StackStatusReason>.` |
| `*_IN_PROGRESS` > 1h | 1 | Broken | n/a | S2, S4 | `stuck: in progress 2h` (actual age) | `Stack has been <StackStatus> for <age> — likely stuck, check stack events.` |
| `DriftInformation.StackDriftStatus == DRIFTED` | 1 | Warning | n/a | S2, S4 | `drifted since <LastCheckTimestamp>` | `Stack configuration differs from template; last drift check <LastCheckTimestamp>.` |
| Recent stack event `ResourceStatus == *_FAILED` | 2 | Broken | n/a | S2 (row already red), S4 (deduped), S5 | `failed: <LogicalResourceId>` | `Recent event: <LogicalResourceId> <ResourceStatus> — <ResourceStatusReason>.` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`ROLLBACK_COMPLETE`, `UPDATE_FAILED`) in the List text column is not acceptable alone. Pair it with the cause from `StackStatusReason`.
- Keep both columns short: List text ≤ 40 chars, Detail text ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Mostly yes — `StackStatus` values like `UPDATE_FAILED` paired with the `StackStatusReason` excerpt in S4 are self-explanatory, and the `rollback: failed create, delete required` phrasing for `ROLLBACK_COMPLETE` removes the ambiguity of a bare status word. The one gap: the stuck `*_IN_PROGRESS > 1h` row must show the elapsed age (`stuck: in progress 2h`) in S4 — the status alone (`UPDATE_IN_PROGRESS`) is indistinguishable from a normal in-flight deploy, so implementations MUST compute and render the age on that row.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): `DetectStackDrift` + `DescribeStackDriftDetectionStatus` async polling.
- Fresh drift detection — `DriftInformation` on `DescribeStacks` only reflects the last manually-triggered drift run.
- The S3 `TemplateURL` pivot as a first-class counted panel entry — a9s-devops: not worth the per-stack `GetTemplateSummary` call in Wave 2 budget; surfaced only as "open S3 list" workflow.
- Any UI element not listed in §4 — no new columns, icons, views, or key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- `cfn` is in both golden docs — `docs/related-resources.md` § Per-type contract row `cfn` + `docs/attention-signals.md` § CI/CD row `cfn`.
- AWS API reference URL — `docs/related-resources.md` § `cfn` (`https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_Stack.html`).
- List API is `DescribeStacks` — `docs/attention-signals.md` § CI/CD row `cfn` Source column.
- Wave 2 uses `DescribeStackEvents` per stack, first page only, scanned client-side — `docs/attention-signals.md` § CI/CD row `cfn` Wave 2 cell.
- Related targets `cfn`, `ct-events`, `eb-rule`, `role`, `s3`, `sns` — `docs/related-resources.md` § `cfn`.
- `ct-events` is a universal pivot — `docs/related-resources.md` § Policy item 4.
- Nested-stack pivot via `ParentId`/`RootId` — `AWS SDK Go v2 — cloudformation/types.Stack § ParentId, RootId`.
- Service role pivot via `Stack.RoleARN` — `AWS SDK Go v2 — cloudformation/types.Stack § RoleARN`.
- SNS topics pivot via `Stack.NotificationARNs` — `AWS SDK Go v2 — cloudformation/types.Stack § NotificationARNs`.
- `TemplateURL` is not on `DescribeStacks` response — `AWS SDK Go v2 — cloudformation/types.Stack` (no `TemplateURL` field; template recovery requires `GetTemplate`/`GetTemplateSummary`).
- `eb-rule` discovered by reverse scan (no direct stack field) — a9s-devops (2026-04-20): possible=yes via reverse-scan of EventBridge rules for `source: aws.cloudformation`, worth=yes for "where do stack events fan out?" workflow; direct field on Stack would be ideal but does not exist.
- S3 pivot not directly discoverable from list row — a9s-devops (2026-04-20): possible=partial (requires `GetTemplateSummary` not in Wave 2 budget), worth=yes workflow but not worth the extra per-stack call; treat as "open S3 list" manual pivot, no count.
- `!` vs `~` severity — not applicable to `cfn`: Wave 2 failures land on rows that are already colored by Wave 1 `*_FAILED`/`ROLLBACK_*`, so S3 glyphs are suppressed by rule (glyphs only on green rows).
- Wave 1 `StackStatus` buckets — `docs/attention-signals.md` § CI/CD row `cfn` Wave 1 cell (Healthy / Warning / Broken mapping as transcribed in §3.1).
- `StackStatusReason` carries the cause surfaced in S4 — `AWS SDK Go v2 — cloudformation/types.Stack § StackStatusReason` ("Success/failure message associated with the stack status").
- `DriftInformation.StackDriftStatus` is the drift field — `AWS SDK Go v2 — cloudformation/types.Stack § DriftInformation` → `cloudformation/types.StackDriftInformation § StackDriftStatus` (values include `DRIFTED`, `IN_SYNC`, `NOT_CHECKED`, `UNKNOWN`).
- `LastCheckTimestamp` for drift — `AWS SDK Go v2 — cloudformation/types.StackDriftInformation § LastCheckTimestamp`.
- Stack event fields `ResourceStatus`, `ResourceStatusReason`, `LogicalResourceId` for S5 text — `AWS SDK Go v2 — cloudformation/types.StackEvent § ResourceStatus, ResourceStatusReason, LogicalResourceId`.
- Read-only invariant (no write ops) — `docs/architecture.md` § "What is a9s?" ("a9s never makes write calls to AWS").
