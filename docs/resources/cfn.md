---
shortName: cfn
name: CloudFormation Stacks
awsApiRef: https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_Stack.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
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

Expected targets from `docs/related-resources.md` § Per-type contract: `cfn`, `ct-events`, `eb-rule`, `role`, `s3`, `sns`.

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

- **Why related**: Universal pivot — applies to every registered type; see `docs/related-resources.md` §Policy 4. CloudTrail events filtered by `resources[].ARN == <StackId>` give the audit trail of who created/updated/rolled back the stack.
- **How discovered**: Query `LookupEvents` filtered on the stack ARN.
- **Count shown**: yes (event count).

## 3. Attention / Issues Algorithm

**Source API**: [DescribeStacks](https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_DescribeStacks.html)

Transcribed from `docs/attention-signals.md § Signals § CI/CD` row `cfn`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `StackStatus` matches `*_IN_PROGRESS` or `REVIEW_IN_PROGRESS` → Warning (operation in flight).
  - **State bucket**: Warning.
  - **How obtained**: `StackStatus` field.

- **Signal**: `StackStatus == ROLLBACK_COMPLETE` → Broken (failed-create tombstone — stack operationally dead, delete-and-recreate required, but not actively failing).
  - **State bucket**: Broken.
  - **How obtained**: `StackStatus` field.

- **Signal**: `StackStatus` ∈ {`UPDATE_ROLLBACK_COMPLETE`, `IMPORT_ROLLBACK_COMPLETE`} → Broken (update failed, stack reverted to prior state).
  - **State bucket**: Broken.
  - **How obtained**: `StackStatus` field.

- **Signal**: `StackStatus` matches `*_FAILED` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `StackStatus` field + `StackStatusReason` carries the human cause.

- **Signal**: `StackStatus == DELETE_COMPLETE`.
  - **State bucket**: Dim.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `EnableTerminationProtection == false` on a live, top-level stack.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: a stack output value scans as a credential.
  - **State bucket**: Broken.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: `DriftInformation.StackDriftStatus == DRIFTED` → Warning (signal is low-coverage until `DetectStackDrift` has been run).
  - **State bucket**: Warning.
  - **How obtained**: `DriftInformation.StackDriftStatus` field on the list response.

- **Signal**: Recent stack event with `ResourceStatus == *_FAILED` → Broken.
  - **State bucket**: Broken.
  - **API call**: `DescribeStackEvents` — one per stack, take the first response page and scan client-side for the most recent `*_FAILED` event.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `DetectStackDrift` + `DescribeStackDriftDetectionStatus` (async polling) for fresh drift detection.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `cfn`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `*_IN_PROGRESS` / `REVIEW_IN_PROGRESS` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `<in-progress status, in words>` |
| `ROLLBACK_COMPLETE` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `<rollback status, in words>` |
| `UPDATE_ROLLBACK_COMPLETE` / `IMPORT_ROLLBACK_COMPLETE` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `<rollback status, in words>` |
| `*_FAILED` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `<failure status, in words>` |
| `StackStatus == DELETE_COMPLETE` | 1 | Dim | n/a | S2, S4 | `delete complete` |
| `EnableTerminationProtection == false` on a live, top-level stack | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `termination protection off` |
| a stack output value scans as a credential | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `credential in stack outputs` |
| `DriftInformation.StackDriftStatus == DRIFTED` | 2 | Warning | `~` | S2, S3, S4, S5 | `stack drifted from template` |
| Recent stack event `ResourceStatus == *_FAILED` | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `recent resource failure` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`ROLLBACK_COMPLETE`, `UPDATE_FAILED`) in the List text column is not acceptable alone. Pair it with the cause from `StackStatusReason`.
- List text ≤ 40 chars. The Detail sentence lives on the finding definition and is generated into the Findings table below; it is never written here.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Mostly yes — the Status column spells the stack's own state as `<failure status, in words>`, so a stack that reports `UPDATE_FAILED` reads as words rather than as an API constant, and `recent resource failure`, `stack drifted from template` and `termination protection off` name a posture miss outright. The one gap: a stack stuck in `*_IN_PROGRESS > 1h` reads exactly like a normal in-flight deploy, because the elapsed age is not part of the phrase.

## 4.2 On-Demand Detail Enrichment

Opening the detail, YAML, or JSON view triggers one extra read-only call whose result is attached to the resource's raw structure for the stacked views. List views are never affected.

- AWS API: `GetTemplate`
- Payload: the template body — rendered as structured YAML when the body is JSON, kept as raw text when the template is authored in YAML
- Cache: session-scoped, version-keyed by the stack's last-update time (falling back to creation time) — an in-session stack update changes the key, so the cache misses and re-fetches instead of serving a pre-update template; cleared on profile/region rotation
- Failure: a flash message; the view still renders the un-enriched resource

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): `DetectStackDrift` + `DescribeStackDriftDetectionStatus` async polling.
- Fresh drift detection — `DriftInformation` on `DescribeStacks` only reflects the last manually-triggered drift run.
- The S3 `TemplateURL` pivot as a first-class counted panel entry — a9s-devops: not worth the per-stack `GetTemplateSummary` call in Wave 2 budget; surfaced only as "open S3 list" workflow.
- Any UI element not listed in §4 — no new columns, icons, views, or key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- `cfn` is in both golden docs — `docs/related-resources.md` § Per-type contract row `cfn` + `docs/attention-signals.md § Signals § CI/CD` row `cfn`.
- AWS API reference URL — `docs/related-resources.md` § `cfn` (`https://docs.aws.amazon.com/AWSCloudFormation/latest/APIReference/API_Stack.html`).
- List API is `DescribeStacks` — `core/aws/cfn.go`.
- Wave 2 uses `DescribeStackEvents` per stack, first page only, scanned client-side — `docs/attention-signals.md § Signals § CI/CD` row `cfn`.
- Related targets `cfn`, `ct-events`, `eb-rule`, `role`, `s3`, `sns` — `docs/related-resources.md` § `cfn`.
- `ct-events` is a universal pivot — `docs/related-resources.md` § Policy item 4.
- Nested-stack pivot via `ParentId`/`RootId` — `AWS SDK Go v2 — cloudformation/types.Stack § ParentId, RootId`.
- Service role pivot via `Stack.RoleARN` — `AWS SDK Go v2 — cloudformation/types.Stack § RoleARN`.
- SNS topics pivot via `Stack.NotificationARNs` — `AWS SDK Go v2 — cloudformation/types.Stack § NotificationARNs`.
- `TemplateURL` is not on `DescribeStacks` response — `AWS SDK Go v2 — cloudformation/types.Stack` (no `TemplateURL` field; template recovery requires `GetTemplate`/`GetTemplateSummary`).
- `eb-rule` discovered by reverse scan (no direct stack field) — a9s-devops (2026-04-20): possible=yes via reverse-scan of EventBridge rules for `source: aws.cloudformation`, worth=yes for "where do stack events fan out?" workflow; direct field on Stack would be ideal but does not exist.
- S3 pivot not directly discoverable from list row — a9s-devops (2026-04-20): possible=partial (requires `GetTemplateSummary` not in Wave 2 budget), worth=yes workflow but not worth the extra per-stack call; treat as "open S3 list" manual pivot, no count.
- Wave 2 failures land on rows Wave 1 has already coloured `*_FAILED` / `ROLLBACK_*`, so the colour is not news; the Attention section still lists the Wave 2 finding under its own `!`.
- Wave 1 `StackStatus` buckets — `docs/attention-signals.md § Signals § CI/CD` row `cfn`.
- `StackStatusReason` carries the cause surfaced in S4 — `AWS SDK Go v2 — cloudformation/types.Stack § StackStatusReason` ("Success/failure message associated with the stack status").
- `DriftInformation.StackDriftStatus` is the drift field — `AWS SDK Go v2 — cloudformation/types.Stack § DriftInformation` → `cloudformation/types.StackDriftInformation § StackDriftStatus` (values include `DRIFTED`, `IN_SYNC`, `NOT_CHECKED`, `UNKNOWN`).
- `LastCheckTimestamp` for drift — `AWS SDK Go v2 — cloudformation/types.StackDriftInformation § LastCheckTimestamp`.
- Stack event fields `ResourceStatus`, `ResourceStatusReason`, `LogicalResourceId` for S5 text — `AWS SDK Go v2 — cloudformation/types.StackEvent § ResourceStatus, ResourceStatusReason, LogicalResourceId`.
- Read-only invariant (no write ops) — `docs/architecture.md` § "What is a9s?" ("a9s never makes write calls to AWS").

<!-- BEGIN GENERATED: header -->
cfn — CI/CD. Status key: `status` — the key the status cell reads, and the column naming it is the status column.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| cfn.stack.failed | <failure status, in words> | broken | wave1 | The stack's last operation failed and CloudFormation left it in a state that blocks further updates, so nothing in this stack can be changed until it is resolved. Open the stack events, find the first resource that failed, fix that cause, then continue the update or delete the stack if it never reached a usable state. |
| cfn.stack.rollback | <rollback status, in words> | broken | wave1 | CloudFormation undid the last change, so the stack is back on its previous template and whatever the update was meant to deliver is not deployed. Read the events for the resource that triggered the rollback and fix it before pushing the template again. |
| cfn.stack.in\_progress | <in-progress status, in words> | warn | wave1 | An operation is running against this stack right now, so its resources are being created, replaced or removed and no other change will be accepted until it ends. Watch the stack events until a terminal status appears. |
| cfn.stack.deleted | delete complete | dim | wave1 | — |
| cfn.recent-resource-failure | recent resource failure | broken | wave2 | At least one resource in this stack reported a failure during its most recent operation, so what is deployed is not what the template describes. Open the failing resource in the events list and fix the cause before the next deployment repeats it. |
| cfn.stack-drifted | stack drifted from template | warn | wave2 | The live resources no longer match the template CloudFormation last applied, so the next stack update may overwrite a change somebody made by hand, or fail outright. Run a drift detail report, then either fold the manual change into the template or revert it. |
| cfn.termination-protection-off | termination protection off | warn | wave1 | A single delete call removes this stack and every resource it owns, with no second step to stop an accidental or scripted deletion. Turn on termination protection so the stack must be unprotected deliberately before it can be deleted. |
| cfn.output-secret | credential in stack outputs | broken | wave1 | A stack output holds what looks like a credential, and outputs are readable by anyone who can describe the stack and importable by any other stack in the account. Move the value into Secrets Manager, export only its name, and rotate the exposed credential. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| role | IAM Roles | no |
| cfn | Related Stacks | yes |
| sns | SNS Topics | no |
| s3 | S3 (stack resources) | no |
| eb-rule | EventBridge Rules | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
