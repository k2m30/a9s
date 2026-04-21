---
shortName: cb
name: CodeBuild Projects
awsApiRef: https://docs.aws.amazon.com/codebuild/latest/APIReference/API_Project.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# cb — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `cb`
- **Display name**: CodeBuild Projects
- **AWS API reference**: https://docs.aws.amazon.com/codebuild/latest/APIReference/API_Project.html
- **List API**: `ListProjects` (returns project name strings only — config-only list; see attention-signals.md row `cb`).
- **Describe API (if any)**: `BatchGetProjects` for project config; `ListBuildsForProject(maxResults=1)` + `BatchGetBuilds` for Wave 2 latest-build status.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `ecr`, `kms`, `logs`, `pipeline`, `role`, `s3`, `secrets`, `sg`, `ssm`, `subnet`, `vpc`, `ct-events`.

### `alarm`

- **Why related**: Build-failure alarms — operators wire CloudWatch alarms on CodeBuild `FailedBuilds` / `Duration` metrics to page on-call when a build goes red (`related-resources.md § cb`: "Build-failure alarms").
- **How discovered**: cross-reference the already-loaded `alarm` list by `MetricAlarm.Namespace=="AWS/CodeBuild"` AND `Dimensions[]` containing `Name=="ProjectName"` with `Value==<this project name>` — a9s-devops: MetricAlarm carries resource identity in `Dimensions[]`, CodeBuild's metric dimension is `ProjectName`; no extra API call needed when the `alarm` list is cached.
- **Count shown**: yes.

### `ecr`

- **Why related**: ECR repos the project pushes to — the build container image (or the artifact the build emits) usually lives in a team-owned ECR repo; operators pivot here when "build is green but the image didn't update" (`related-resources.md § ecr`: "CodeBuild projects that push images").
- **How discovered**: read `Project.Environment.Image` — if the URI matches `<acct>.dkr.ecr.<region>.amazonaws.com/<repo>[:tag]`, the `<repo>` segment resolves against the loaded `ecr` cache — a9s-devops: possible=yes, worth=yes. `Environment.Image` is the only deterministic ECR reference on `Project`; push-target repos live only inside buildspec.yml, which a9s does not fetch. Starting with the build image covers the most common "what container am I building in?" workflow.
- **Count shown**: yes.

### `kms`

- **Why related**: Customer-managed key used to encrypt build output artifacts — operators land here when an artifact upload fails with `KMS.AccessDenied` or when auditing which projects touch a sensitive key (`related-resources.md § cb`: "EncryptionKey on artifacts").
- **How discovered**: read `Project.EncryptionKey` — AWS SDK Go v2 — `codebuild/types.Project § EncryptionKey` carries the KMS key ARN or `alias/` reference; resolve against the loaded `kms` cache.
- **Count shown**: yes.

### `logs`

- **Why related**: Build log group — first place an operator opens when a build fails, to read the compiler / shell error (`related-resources.md § cb`: "Build log group").
- **How discovered**: read `Project.LogsConfig.CloudWatchLogs.GroupName` if set; otherwise the default is `/aws/codebuild/<projectName>` — a9s-devops: possible=yes, worth=yes. `LogsConfig.CloudWatchLogs` (AWS SDK Go v2 — `codebuild/types.LogsConfig § CloudWatchLogs`) holds the explicit override; when it is nil or `CloudWatchLogsConfig.Status!="ENABLED"`, CodeBuild writes to the conventional default group name.
- **Count shown**: yes.

### `pipeline`

- **Why related**: Pipelines consuming this project — when a CodePipeline stage is stuck, knowing which CodeBuild project powers it is the first triage step (`related-resources.md § pipeline`: "CodeBuild projects used as pipeline actions").
- **How discovered**: reverse-scan the loaded `pipeline` list for any `stageStates[].actionStates[]` (or `PipelineDeclaration.stages[].actions[]`) with `ActionTypeId.Provider=="CodeBuild"` and `configuration.ProjectName==<this project name>` — a9s-devops: possible=yes, worth=yes. `Project` has no back-pointer to CodePipeline; the relationship is declared only on the pipeline side, so the pivot requires iterating cached pipelines.
- **Count shown**: yes.

### `role`

- **Why related**: IAM service role the project assumes to read source, write artifacts, and talk to KMS / Secrets Manager / Parameter Store — every "access denied" during a build starts here (`related-resources.md § cb`: "Project.ServiceRole").
- **How discovered**: read `Project.ServiceRole` (AWS SDK Go v2 — `codebuild/types.Project § ServiceRole`); resolve the role ARN against the loaded `role` cache.
- **Count shown**: yes.

### `s3`

- **Why related**: Source / artifact buckets — the build pulls source from S3 and/or publishes build artifacts to S3, so operators pivot here to check object versions, ACLs, or retention (`related-resources.md § cb`: "Source/artifact buckets").
- **How discovered**: read `Project.Source.Location` when `Project.Source.Type=="S3"`, `Project.SecondarySources[].Location` for the same, `Project.Artifacts.Location` when `Project.Artifacts.Type=="S3"`, `Project.SecondaryArtifacts[].Location`, and `Project.LogsConfig.S3Logs.Location` — a9s-devops: possible=yes, worth=yes. `ProjectSource.Location` is documented in AWS SDK Go v2 — `codebuild/types.ProjectSource § Type` (S3 case); the Location string for S3 sources/artifacts is `bucket/key`, from which the bucket name is the pivot key.
- **Count shown**: yes.

### `secrets`

- **Why related**: Secrets Manager secrets injected as build env variables — operators open these to confirm rotation state, ARN, or value when a build fails on credential resolution (`related-resources.md § secrets`: "Reverse-scan: CodeBuild Project.Environment.EnvironmentVariables where Type=SECRETS_MANAGER and Value==ARN or name prefix").
- **How discovered**: read `Project.Environment.EnvironmentVariables[]` — entries with `Type==SECRETS_MANAGER` carry the secret ARN or name in `Value`; resolve against the loaded `secrets` cache (AWS SDK Go v2 — `codebuild/types.EnvironmentVariable § Type, Value`).
- **Count shown**: yes.

### `sg`

- **Why related**: Security groups attached to the build's VPC ENI — when a build running in VPC mode cannot reach a private database or internal registry, the SG is the first thing to inspect (`related-resources.md § cb`: "VpcConfig.SecurityGroupIds").
- **How discovered**: read `Project.VpcConfig.SecurityGroupIds` (AWS SDK Go v2 — `codebuild/types.VpcConfig § SecurityGroupIds`); each SG ID resolves against the loaded `sg` cache.
- **Count shown**: yes.

### `ssm`

- **Why related**: SSM Parameter Store values injected as build env variables — operators pivot here to see the current value and last-modified date when a build picks up stale config (`related-resources.md § cb`: "SSM parameters as build env").
- **How discovered**: read `Project.Environment.EnvironmentVariables[]` — entries with `Type==PARAMETER_STORE` carry the parameter name in `Value`; resolve against the loaded `ssm` cache (AWS SDK Go v2 — `codebuild/types.EnvironmentVariable § Type, Value`).
- **Count shown**: yes.

### `subnet`

- **Why related**: Subnets the build ENI lands in — IP exhaustion in one of these subnets causes `UNABLE_TO_CREATE_NETWORK_INTERFACE` build failures (`related-resources.md § cb`: "VpcConfig.Subnets").
- **How discovered**: read `Project.VpcConfig.Subnets` (AWS SDK Go v2 — `codebuild/types.VpcConfig § Subnets`); each subnet ID resolves against the loaded `subnet` cache.
- **Count shown**: yes.

### `vpc`

- **Why related**: VPC the build runs inside — contextual pivot for flow logs, DNS resolution, and endpoint reachability when the build can't reach AWS APIs (`related-resources.md § cb`: "VpcConfig.VpcId").
- **How discovered**: read `Project.VpcConfig.VpcId` (AWS SDK Go v2 — `codebuild/types.VpcConfig § VpcId`); resolve against the loaded `vpc` cache.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Audit trail for build events — "who deleted this project?", "when was ServiceRole last changed?" — CloudTrail is the universal answer (`related-resources.md § cb`: "Audit trail for build events").
- **How discovered**: universal pivot — applies to every registered type; see related-resources.md §Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — the list API does not return fields usable for attention. `ListProjects` is config-only; it returns project-name strings (attention-signals.md row `cb` Wave 1 cell: "None — `ListProjects` is config-only").

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: latest build `buildStatus` in `FAILED` / `FAULT` / `TIMED_OUT` (excluding user-initiated `STOPPED`).
  - **State bucket**: Broken.
  - **API call**: `ListBuildsForProject(maxResults=1)` per project (one per resource) + one batched `BatchGetBuilds` across the collected build IDs.
  - **Cost shape**: hybrid (per-resource list call + one batched Describe across all projects).

### 3.3 Wave 3 — OUT OF SCOPE

Copied verbatim from attention-signals.md row `cb` Wave 3 cell.

- OUT OF SCOPE: Stale-project (>90d); cache-config + perf signals.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing" — e.g. maintenance scheduled, certificate expiring soon. `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `failed: exit 1 in build phase`). **Healthy rows render blank** — no `OK` / `available` / `ACTIVE` / `running`. Empty means "nothing to see." |
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
| latest build `FAILED` / `FAULT` / `TIMED_OUT` | 2 | Broken | `!` | S1, S3, S4, S5 (green row stays green; `!` glyph + cause) | `last build failed: <CurrentPhase>` | `Most recent build ended <buildStatus> in phase <CurrentPhase> on <EndTime>.` |

Cause-field sources for S4 / S5: `Build.BuildStatus` (enum) plus `Build.CurrentPhase` and `Build.EndTime` from the batched `BatchGetBuilds` response (AWS SDK Go v2 — `codebuild/types.Build § BuildStatus, CurrentPhase, EndTime`). When `BuildStatus==FAULT` the fault usually reflects a platform/infrastructure problem; `FAILED` reflects a user-code/script exit; `TIMED_OUT` reflects the project's `TimeoutInMinutes`. The S4 line uses the status keyword paired with the phase so the operator sees where it broke without opening detail.

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`FAILED`, `FAULT`, `TIMED_OUT`) in the List text column is not acceptable. The spec pairs it with the build phase.
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), the row is omitted entirely from this table; §3 still describes it.
- List text ≤ 40 chars, Detail text ≤ 100 chars. The sample rows above fit.

### 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — a red row for a failing project carries `last build failed: <CurrentPhase>` in S4, which names both the outcome and the build phase; the operator knows whether this is a runtime failure, a pre-build environment failure, or a post-build artifact-push failure, and can triage the team to investigate without first opening the detail view.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): stale-project (>90d); cache-config + perf signals.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?": "Read-only by design — a9s never makes write calls to AWS. Every AWS API call is a List, Describe, or Get operation.").
- Buildspec.yml content discovery (would require downloading the buildspec from S3/git). The `ecr` pivot therefore uses only `Environment.Image`, not push-target repos declared inside buildspec commands — a9s-devops: possible=no (no AWS API returns parsed buildspec references), captured here rather than as `TBD`.

## 6. Citations

- Display name, Wave descriptors, List API — `docs/attention-signals.md § CI/CD` row `cb`.
- AWS API reference URL and per-type related targets — `docs/related-resources.md § Per-type contract` row `cb` and `docs/related-resources.md § cb`.
- Read-only invariant — `docs/architecture.md § What is a9s?` ("Read-only by design — a9s never makes write calls to AWS").
- `ct-events` universal-pivot rule — `docs/related-resources.md § Policy` item 4 ("`ct-events` (CloudTrail audit trail) is implicitly relevant for every registered type").
- `alarm` discovery field — `a9s-devops (2026-04-20): possible=yes, worth=yes. CloudWatch MetricAlarm.Dimensions[] carries Name=ProjectName for AWS/CodeBuild namespace; reverse-scan of cached alarm list is the only correct pivot because Project has no alarm field.`
- `ecr` discovery field — `a9s-devops (2026-04-20): possible=yes, worth=yes. Project.Environment.Image is the only deterministic ECR reference; buildspec push targets are undiscoverable without downloading the buildspec.` Plus AWS SDK Go v2 — `codebuild/types.ProjectEnvironment § Image`.
- `kms` discovery field — AWS SDK Go v2 — `codebuild/types.Project § EncryptionKey`.
- `logs` discovery field — `a9s-devops (2026-04-20): possible=yes, worth=yes. LogsConfig.CloudWatchLogs.GroupName when explicit; fallback to default /aws/codebuild/<projectName> when unset or ENABLED-status missing.` Plus AWS SDK Go v2 — `codebuild/types.LogsConfig § CloudWatchLogs`.
- `pipeline` discovery field — `a9s-devops (2026-04-20): possible=yes, worth=yes. Project has no back-pointer; reverse-scan cached pipeline list for ActionTypeId.Provider==CodeBuild with configuration.ProjectName match.`
- `role` discovery field — AWS SDK Go v2 — `codebuild/types.Project § ServiceRole`.
- `s3` discovery field — `a9s-devops (2026-04-20): possible=yes, worth=yes. Project.Source.Location (when Type==S3), SecondarySources[].Location, Artifacts.Location (when Type==S3), SecondaryArtifacts[].Location, and LogsConfig.S3Logs.Location — bucket name is the first segment of the Location string.` Plus AWS SDK Go v2 — `codebuild/types.ProjectSource § Type` and `codebuild/types.Project § Artifacts, SecondaryArtifacts`.
- `secrets` discovery field — `docs/related-resources.md § secrets` ("Reverse-scan: CodeBuild Project.Environment.EnvironmentVariables where Type=SECRETS_MANAGER and Value==ARN or name prefix"). Plus AWS SDK Go v2 — `codebuild/types.EnvironmentVariable § Type, Value`.
- `sg` / `subnet` / `vpc` discovery fields — AWS SDK Go v2 — `codebuild/types.VpcConfig § SecurityGroupIds, Subnets, VpcId`.
- `ssm` discovery field — AWS SDK Go v2 — `codebuild/types.EnvironmentVariable § Type, Value` (Type==PARAMETER_STORE case).
- Wave 1 "none" claim — `docs/attention-signals.md § CI/CD` row `cb` Wave 1 cell: "None — `ListProjects` is config-only".
- Wave 2 signal and API wiring — `docs/attention-signals.md § CI/CD` row `cb` Wave 2 cell: "Latest build status per project (via `ListBuildsForProject(maxResults=1)` + batched `BatchGetBuilds`): latest `buildStatus` in `FAILED`/`FAULT`/`TIMED_OUT` → Broken (excluding user-initiated `STOPPED`)".
- Wave 2 status field semantics — AWS SDK Go v2 — `codebuild/types.Build § BuildStatus, CurrentPhase, EndTime` and `codebuild/types.StatusType` (values `FAILED`, `FAULT`, `IN_PROGRESS`, `STOPPED`, `SUCCEEDED`, `TIMED_OUT`).
- Wave 3 out-of-scope signals — `docs/attention-signals.md § CI/CD` row `cb` Wave 3 cell: "Stale-project (>90d); cache-config + perf signals".
- Buildspec-based ECR discovery exclusion — `a9s-devops (2026-04-20): possible=no, worth=n/a. AWS APIs do not return parsed buildspec content; the buildspec is either inline YAML or a file reference, and neither is exposed as structured references on Project.`
