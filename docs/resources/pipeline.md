---
shortName: pipeline
name: CodePipelines
awsApiRef: https://docs.aws.amazon.com/codepipeline/latest/APIReference/API_PipelineDeclaration.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# pipeline — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `pipeline`
- **Display name**: CodePipelines
- **AWS API reference**: <https://docs.aws.amazon.com/codepipeline/latest/APIReference/API_PipelineDeclaration.html>
- **List API**: `ListPipelines` — returns `PipelineSummary[]`. Per `attention-signals.md`, `ListPipelines` is config-only: the summary carries `Name`, `Version`, `Created`, `Updated`, `ExecutionMode`, `PipelineType` and nothing about execution health — so no Wave 1 health signal is reachable from the list alone.
- **Describe API (if any)**: Two calls, both per-pipeline, used in Wave 2:
  - `GetPipelineState` — returns `stageStates[].latestExecution.status` used by every Wave 2 health signal.
  - `GetPipeline` — returns `PipelineDeclaration` with full `Stages[].Actions[]`, `ArtifactStore(s)`, `RoleArn`. Used exclusively to discover related targets; never used for attention signals.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `cb`, `cfn`, `codeartifact`, `eb-rule`, `ecr`, `ecs-svc`, `kms`, `lambda`, `role`, `s3`, `sns`, `ct-events`.

### `cb`

- **Why related**: CodeBuild projects used as pipeline actions — `docs/related-resources.md` § Per-type contract `pipeline`.
- **How discovered**: call `GetPipeline`, walk `PipelineDeclaration.Stages[].Actions[]` and keep actions where `ActionTypeId.Category == Build` AND `ActionTypeId.Provider == CodeBuild`; read `Configuration["ProjectName"]` for each match; cross-reference the already-loaded `cb` list by name — a9s-devops: this is the canonical CodeBuild-in-pipeline pattern; the project name is always carried in the action `Configuration` map, never as a structured field, because `Configuration` is how CodePipeline passes provider-specific params (`AWS SDK Go v2 — service/codepipeline/types.ActionDeclaration § Configuration`).
- **Count shown**: yes.

### `cfn`

- **Why related**: CloudFormation stacks this pipeline deploys — `docs/related-resources.md` § Per-type contract `pipeline`.
- **How discovered**: call `GetPipeline`, walk `PipelineDeclaration.Stages[].Actions[]` and keep actions where `ActionTypeId.Category == Deploy` AND `ActionTypeId.Provider == CloudFormation`; read `Configuration["StackName"]` for each match; cross-reference the already-loaded `cfn` list by stack name — a9s-devops: CloudFormation deploy actions always carry `StackName` in the action `Configuration`; this is the only field that identifies the target stack.
- **Count shown**: yes.

### `codeartifact`

- **Why related**: CodeArtifact repository used as a pipeline source — `docs/related-resources.md` § Per-type contract `pipeline`.
- **How discovered**: call `GetPipeline`, walk `PipelineDeclaration.Stages[].Actions[]` and keep actions where `ActionTypeId.Category == Source` AND `ActionTypeId.Provider == CodeCommit` is excluded — target is `Provider == CodeStarSourceConnection` with a CodeArtifact ARN in `Configuration`, or a direct CodeArtifact-backed source — a9s-devops: CodeArtifact as a first-class pipeline source is rare in practice; most teams consume CodeArtifact via a CodeBuild action's install step, which is invisible to `GetPipeline`. When a direct source action is present, `Configuration["RepositoryName"]` and `Configuration["DomainName"]` identify the CodeArtifact repository and can be cross-referenced against the loaded `codeartifact` list by `name`. When no direct source action exists this target legitimately shows 0.
- **Count shown**: yes.

### `eb-rule`

- **Why related**: EventBridge rule that triggers this pipeline on source events (CodeCommit push, ECR push, schedule) — `docs/related-resources.md` § Per-type contract `pipeline`.
- **How discovered**: reverse-scan the already-loaded `eb-rule` list; keep rules whose `Targets[].Arn` matches `arn:aws:codepipeline:<region>:<account>:<pipelineName>` — a9s-devops: EventBridge → CodePipeline trigger wiring lives on the rule target list, not on the pipeline. The pipeline declaration's own `Triggers[]` field describes Git-tag/branch triggers (V2 only) and does not reference EventBridge rules. Reverse scan is the only way.
- **Count shown**: yes.

### `ecr`

- **Why related**: ECR repositories this pipeline pushes to or pulls from — `docs/related-resources.md` § Per-type contract `pipeline`.
- **How discovered**: call `GetPipeline`, walk `PipelineDeclaration.Stages[].Actions[]`:
  - Source actions with `ActionTypeId.Provider == ECR` carry `Configuration["RepositoryName"]` — a direct pull on image push.
  - Build actions (CodeBuild) that push to ECR do so inside the CodeBuild `buildspec`, not visibly in the pipeline declaration — that linkage lives on the CodeBuild project and is reached via the `cb` pivot.
  - a9s-devops: `GetPipeline` only exposes ECR sources, not ECR push sinks. Cross-reference against the loaded `ecr` list by `repositoryName`.
- **Count shown**: yes.

### `ecs-svc`

- **Why related**: ECS services this pipeline deploys to — `docs/related-resources.md` § Per-type contract `pipeline`.
- **How discovered**: call `GetPipeline`, walk `PipelineDeclaration.Stages[].Actions[]` and keep actions where `ActionTypeId.Category == Deploy` AND `ActionTypeId.Provider in {ECS, ECSBlueGreen, CodeDeployToECS}`; read `Configuration["ClusterName"]` and `Configuration["ServiceName"]` — a9s-devops: these two provider names cover both the stock ECS deploy action and the CodeDeploy blue/green variant. Both carry cluster and service names in the action `Configuration`. Cross-reference the loaded `ecs-svc` list by (cluster, service) tuple.
- **Count shown**: yes.

### `kms`

- **Why related**: Customer-managed KMS key used to encrypt the pipeline's artifact store — `docs/related-resources.md` § Per-type contract `pipeline`.
- **How discovered**: call `GetPipeline`, read `PipelineDeclaration.ArtifactStore.EncryptionKey.Id` (or, for cross-region pipelines, iterate `PipelineDeclaration.ArtifactStores[].EncryptionKey.Id`); when `EncryptionKey.Type == KMS`, cross-reference the loaded `kms` list by key ID / alias / ARN — a9s-devops: `EncryptionKey` is optional; when nil the artifact store uses the S3 default key, which is AWS-managed and not in the `kms` list. In that case the target legitimately shows 0. The SDK defines `ArtifactStore.EncryptionKey *EncryptionKey` and `EncryptionKey.Id *string` exactly for this lookup (`AWS SDK Go v2 — service/codepipeline/types.ArtifactStore § EncryptionKey`, `types.EncryptionKey § Id`).
- **Count shown**: yes.

### `lambda`

- **Why related**: Lambda functions invoked as pipeline actions — `docs/related-resources.md` § Per-type contract `pipeline`.
- **How discovered**: call `GetPipeline`, walk `PipelineDeclaration.Stages[].Actions[]` and keep actions where `ActionTypeId.Category == Invoke` AND `ActionTypeId.Provider == Lambda`; read `Configuration["FunctionName"]`; cross-reference the loaded `lambda` list by function name — a9s-devops: Invoke/Lambda is the one well-defined contract for pipeline-driven Lambda calls. `FunctionName` is always in `Configuration`.
- **Count shown**: yes.

### `role`

- **Why related**: IAM role the pipeline assumes to execute actions — `docs/related-resources.md` § Per-type contract `pipeline`.
- **How discovered**: call `GetPipeline`, read `PipelineDeclaration.RoleArn`; cross-reference the loaded `role` list by ARN. Per-action override roles (`ActionDeclaration.RoleArn`) are additionally walked and de-duplicated — a9s-devops: the pipeline service role is always on the declaration; per-action roles are optional and typically used for cross-account deploy targets. Both are worth surfacing. `RoleArn` is a `This member is required` field on `PipelineDeclaration` per the SDK (`AWS SDK Go v2 — service/codepipeline/types.PipelineDeclaration § RoleArn`).
- **Count shown**: yes.

### `s3`

- **Why related**: S3 bucket used as the pipeline's artifact store (or, less commonly, as a source) — `docs/related-resources.md` § Per-type contract `pipeline`.
- **How discovered**: call `GetPipeline`; read `PipelineDeclaration.ArtifactStore.Location` and each entry's `Location` in `PipelineDeclaration.ArtifactStores[]`. Also walk `Stages[].Actions[]` and add `Configuration["S3Bucket"]` from Source actions where `Provider == S3` and from Deploy actions where `Provider == S3`. Cross-reference the loaded `s3` list by bucket name — a9s-devops: `ArtifactStore.Location` is a required field and is the bucket name directly per the SDK (`AWS SDK Go v2 — service/codepipeline/types.ArtifactStore § Location`). Source/deploy S3 actions are the secondary case.
- **Count shown**: yes.

### `sns`

- **Why related**: SNS topic notified by manual-approval actions — `docs/related-resources.md` § Per-type contract `pipeline`.
- **How discovered**: call `GetPipeline`, walk `PipelineDeclaration.Stages[].Actions[]` and keep actions where `ActionTypeId.Category == Approval` AND `ActionTypeId.Provider == Manual`; read `Configuration["NotificationArn"]`; cross-reference the loaded `sns` list by topic ARN — a9s-devops: the manual-approval action is the only action type that accepts an SNS topic directly (via `NotificationArn`). Pipeline-state change notifications via the Developer Tools notifications service use a separate API (`codestarnotifications:ListNotificationRules`) and are out of scope for this target per the related-resources contract, which only names "Approval SNS topic".
- **Count shown**: yes.

### `ct-events`

- **Why related**: Universal pivot — audit trail for pipeline state changes (start, stop, approval, role changes) — `docs/related-resources.md` § Per-type contract `pipeline`.
- **How discovered**: pre-built CloudTrail query scoped to the pipeline `Name` as the resource identifier.
- **Count shown**: unknown (CloudTrail queries are windowed; a reliable total isn't available without a separate count call).
- Universal pivot — applies to every registered type; see `related-resources.md` §Policy, rule 4.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — `ListPipelines` is config-only (returns `PipelineSummary` carrying only `Name`, `Version`, `Created`, `Updated`, `ExecutionMode`, `PipelineType`; no execution state). All pipelines render green in Wave 1 and the Status column is blank until Wave 2 arrives.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: any `stageStates[].latestExecution.status` in `Failed`.
  - **State bucket**: Broken.
  - **API call**: `GetPipelineState` per pipeline — one call per pipeline.
  - **Cost shape**: per-resource.

- **Signal**: any `stageStates[].latestExecution.status` in `Stopped`.
  - **State bucket**: Broken.
  - **API call**: `GetPipelineState` per pipeline — same call, no additional cost.
  - **Cost shape**: per-resource.

- **Signal**: any `stageStates[].latestExecution.status` in `Cancelled`.
  - **State bucket**: Broken.
  - **API call**: `GetPipelineState` per pipeline — same call, no additional cost.
  - **Cost shape**: per-resource.

- **Signal**: stage `latestExecution.status == InProgress` with the stage running >2h.
  - **State bucket**: Warning.
  - **API call**: `GetPipelineState` per pipeline — same call, no additional cost; elapsed time is derived from the stage's latest transition timestamp on the response.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `ListPipelineExecutions` trend (long-horizon success-rate / failure-rate trend across recent executions).
- OUT OF SCOPE: dormant-pipeline detection (pipelines that have not executed within a configurable window).

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause. **Healthy rows render blank.** |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit).
- **Wave 1 Warning / Broken / Dim** → S2 + S4.
- **Wave 2 finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4, S5.
- **Wave 2 finding on a Healthy row, informational** → `~` glyph on green row. S3, S4, S5. No S1.
- **Wave 2 finding on an already yellow/red/dim row** → S3 suppressed, S4 deduplicates with existing cause, S5 carries the full sentence, S1 still counts if `!`.

Because pipeline has no Wave 1 health signals, every row starts green and any §4 row here describes a Wave 2 finding that flips the row directly. These Wave 2 findings are operational failures (a stage failed) not "background concerns", so they bump S2 to red/yellow (not a green-row glyph); S3 is suppressed because the row is no longer green. S1 still counts the `!` findings.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `latestExecution.status == Failed` | 2 | Broken | `!` | S1, S2, S4, S5 | `failed: <stage>` | `Stage <stage> failed on last execution — open CodePipeline console for action logs.` |
| `latestExecution.status == Stopped` | 2 | Broken | `!` | S1, S2, S4, S5 | `stopped: <stage>` | `Stage <stage> was stopped by a user before completing.` |
| `latestExecution.status == Cancelled` | 2 | Broken | `!` | S1, S2, S4, S5 | `cancelled: <stage>` | `Stage <stage> was cancelled — pipeline definition changed mid-run.` |
| `latestExecution.status == InProgress >2h` | 2 | Warning | `!` | S1, S2, S4, S5 | `stuck >2h: <stage>` | `Stage <stage> has been running over 2 hours — likely hung action or awaiting approval.` |

Notes:

- `<stage>` is the first stage whose `latestExecution.status` matches the signal. When multiple stages match, the earliest failing stage (by stage order in the declaration) wins — it is the root failure.
- For `Cancelled`, the SDK comment explicitly says "A status of cancelled means that the pipeline's definition was updated before the stage execution could be completed" (`AWS SDK Go v2 — service/codepipeline/types.StageExecution § Status`). The S5 wording reflects that.
- Manual-approval stages appear as `InProgress` while waiting; the `>2h` threshold is blunt for those. When `ActionState.LatestExecution.Token` is non-nil on an approval action, a9s may refine S5 to `waiting for manual approval: <action>` — noted as a refinement, not required for the first pass.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for failed/stopped/cancelled pipelines (`failed: <stage>` / `stopped: <stage>` / `cancelled: <stage>` in S4 names the exact stage); for the `stuck >2h` case, the stage name is in S4 but the underlying reason (hung integration? awaiting approval?) still requires detail — consider refining S4 to `awaiting approval: <action>` when an approval token is pending, which the `GetPipelineState` response already carries.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Execution-history trend analysis and dormant-pipeline detection — both would require additional `ListPipelineExecutions` calls and are explicitly Wave 3.
- Pipeline `Triggers[]` (Git-tag / branch triggers, V2 pipelines only) — not referenced by any §2 target or §3 signal.
- `codestarnotifications` rules — out of scope per the §2 `sns` target rationale.
- Any UI element not listed in §4 — no new columns, icons, views, or key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- pipeline related-panel targets `cb`, `cfn`, `codeartifact`, `eb-rule`, `ecr`, `ecs-svc`, `kms`, `lambda`, `role`, `s3`, `sns`, `ct-events` — `docs/related-resources.md` § Per-type contract, row `pipeline`.
- pipeline has no Wave 1 signals (`ListPipelines` is config-only) — `docs/attention-signals.md` § CI/CD, row `pipeline` Wave 1 cell.
- pipeline Wave 2 signals (`Failed` / `Stopped` / `Cancelled` and `InProgress >2h`) use `GetPipelineState` per pipeline — `docs/attention-signals.md` § CI/CD, row `pipeline` Wave 2 cell.
- pipeline Wave 3 items (`ListPipelineExecutions` trend, dormant-pipeline detection) are out of scope — `docs/attention-signals.md` § CI/CD, row `pipeline` Wave 3 cell.
- `PipelineSummary` fields returned by `ListPipelines` are `Name`, `Version`, `Created`, `Updated`, `ExecutionMode`, `PipelineType` (no health fields) — `AWS SDK Go v2 — service/codepipeline/types.PipelineSummary`.
- `PipelineDeclaration.RoleArn` is required; `PipelineDeclaration.ArtifactStore` / `ArtifactStores` carry encryption and location — `AWS SDK Go v2 — service/codepipeline/types.PipelineDeclaration § RoleArn, ArtifactStore, ArtifactStores`.
- `ArtifactStore.Location` (bucket name) and `ArtifactStore.EncryptionKey` (optional KMS) drive `s3` and `kms` discovery — `AWS SDK Go v2 — service/codepipeline/types.ArtifactStore § Location, EncryptionKey`; `types.EncryptionKey § Id, Type`.
- `ActionDeclaration.ActionTypeId` (Category/Owner/Provider) + `ActionDeclaration.Configuration` drive every action-based target (`cb`, `cfn`, `ecs-svc`, `ecr`, `lambda`, `sns`, `codeartifact`, `s3` secondary) — `AWS SDK Go v2 — service/codepipeline/types.ActionDeclaration § ActionTypeId, Configuration`; `types.ActionTypeId § Category, Owner, Provider`.
- `StageState.LatestExecution.Status` is the field driving all Wave 2 findings; `StageExecutionStatus` SDK note on `Cancelled` explains the "definition updated mid-run" semantics — `AWS SDK Go v2 — service/codepipeline/types.StageState § LatestExecution`; `types.StageExecution § Status`.
- `ActionState.LatestExecution.Token` present indicates a manual approval is pending — `AWS SDK Go v2 — service/codepipeline/types.ActionExecution § Token`.
- `ct-events` universal pivot applied to every registered type — `docs/related-resources.md` § Policy, rule 4.
- a9s is read-only — `docs/architecture.md` § "What is a9s?".
- `cb` discovery via `Provider==CodeBuild` action with `Configuration["ProjectName"]` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Configuration map is the sole source of provider-specific params for action targets; ProjectName is the only CodeBuild linkage.`
- `cfn` discovery via `Provider==CloudFormation` deploy action with `Configuration["StackName"]` — `a9s-devops (2026-04-20): possible=yes, worth=yes. StackName is the canonical link.`
- `codeartifact` discovery via direct source actions only; indirect CodeBuild usage invisible — `a9s-devops (2026-04-20): possible=yes (when direct), worth=yes. Rare in practice but idiomatic; 0-count when no direct action is not a gap.`
- `eb-rule` discovery via reverse-scan of loaded rule targets by pipeline ARN — `a9s-devops (2026-04-20): possible=yes, worth=yes. EventBridge target wiring lives on the rule, not on the pipeline. Pipeline V2 Triggers[] covers Git-tag/branch only.`
- `ecr` discovery via `Provider==ECR` source actions; push side lives on CodeBuild project — `a9s-devops (2026-04-20): possible=yes (sources only), worth=yes. GetPipeline doesn't see ECR push sinks; operator reaches them via the cb pivot.`
- `ecs-svc` discovery via `Provider in {ECS, ECSBlueGreen, CodeDeployToECS}` deploy actions with `Configuration["ClusterName"]` + `Configuration["ServiceName"]` — `a9s-devops (2026-04-20): possible=yes, worth=yes. These three providers cover both stock and blue/green ECS deploy.`
- `kms` discovery via `ArtifactStore.EncryptionKey.Id` when `Type==KMS`; default-key case legitimately shows 0 — `a9s-devops (2026-04-20): possible=yes (when customer-managed), worth=yes. AWS-managed default key has no kms list entry, so 0-count is correct.`
- `lambda` discovery via `Provider==Lambda` invoke actions with `Configuration["FunctionName"]` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Invoke/Lambda is the sole direct pipeline→Lambda contract.`
- `role` discovery via `PipelineDeclaration.RoleArn` plus per-action overrides — `a9s-devops (2026-04-20): possible=yes, worth=yes. Service role always present; per-action roles common for cross-account deploy.`
- `s3` discovery via `ArtifactStore.Location` plus `Provider==S3` source/deploy actions — `a9s-devops (2026-04-20): possible=yes, worth=yes. ArtifactStore.Location is the bucket name directly.`
- `sns` discovery limited to manual-approval `Configuration["NotificationArn"]`; developer-tools notifications are a separate API and out of scope for this target — `a9s-devops (2026-04-20): possible=yes (approval only), worth=yes. The related-resources contract says "Approval SNS topic", so scope is bounded.`
- Superseded HOW ignored — row middle-dot `·` marker, `⚠ Background Check` detail header, and derived list-level banner in `docs/enrichment-visibility.md` are not cited or reproduced per the skill's S1–S5 rules.
