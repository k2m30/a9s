---
shortName: cb
name: CodeBuild Projects
awsApiRef: https://docs.aws.amazon.com/codebuild/latest/APIReference/API_Project.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# cb — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `cb`
- **Display name**: CodeBuild Projects
- **AWS API reference**: <https://docs.aws.amazon.com/codebuild/latest/APIReference/API_Project.html>
- **List API**: `ListProjects` (returns project name strings only — a config-only list; nothing on it classifies, see `docs/attention-signals.md § Signals § CI/CD` row `cb`).
- **Describe API (if any)**: `BatchGetProjects` for project config; `ListBuildsForProject(maxResults=1)` + `BatchGetBuilds` for Wave 2 latest-build status.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `alarm`, `ecr`, `kms`, `logs`, `pipeline`, `role`, `s3`, `secrets`, `sg`, `ssm`, `subnet`, `vpc`, `ct-events`.

### `alarm`

- **Why related**: Build-failure alarms — operators wire CloudWatch alarms on CodeBuild `FailedBuilds` / `Duration` metrics to page on-call when a build goes red (`docs/related-resources.md § cb`: "Build-failure alarms").
- **How discovered**: cross-reference the already-loaded `alarm` list by `MetricAlarm.Namespace=="AWS/CodeBuild"` AND `Dimensions[]` containing `Name=="ProjectName"` with `Value==<this project name>` — a9s-devops: MetricAlarm carries resource identity in `Dimensions[]`, CodeBuild's metric dimension is `ProjectName`; no extra API call needed when the `alarm` list is cached.
- **Count shown**: yes.

### `ecr`

- **Why related**: ECR repos the project pushes to — the build container image (or the artifact the build emits) usually lives in a team-owned ECR repo; operators pivot here when "build is green but the image didn't update" (`docs/related-resources.md § ecr`: "CodeBuild projects that push images").
- **How discovered**: read `Project.Environment.Image` — if the URI matches `<acct>.dkr.ecr.<region>.amazonaws.com/<repo>[:tag]`, the `<repo>` segment resolves against the loaded `ecr` cache — a9s-devops: possible=yes, worth=yes. `Environment.Image` is the only deterministic ECR reference on `Project`; push-target repos live only inside buildspec.yml, which a9s does not fetch. Starting with the build image covers the most common "what container am I building in?" workflow.
- **Count shown**: yes.

### `kms`

- **Why related**: Customer-managed key used to encrypt build output artifacts — operators land here when an artifact upload fails with `KMS.AccessDenied` or when auditing which projects touch a sensitive key (`docs/related-resources.md § cb`: "EncryptionKey on artifacts").
- **How discovered**: read `Project.EncryptionKey` — AWS SDK Go v2 — `codebuild/types.Project § EncryptionKey` carries the KMS key ARN or `alias/` reference; resolve against the loaded `kms` cache.
- **Count shown**: yes.

### `logs`

- **Why related**: Build log group — first place an operator opens when a build fails, to read the compiler / shell error (`docs/related-resources.md § cb`: "Build log group").
- **How discovered**: read `Project.LogsConfig.CloudWatchLogs.GroupName` if set; otherwise the default is `/aws/codebuild/<projectName>` — a9s-devops: possible=yes, worth=yes. `LogsConfig.CloudWatchLogs` (AWS SDK Go v2 — `codebuild/types.LogsConfig § CloudWatchLogs`) holds the explicit override; when it is nil or `CloudWatchLogsConfig.Status!="ENABLED"`, CodeBuild writes to the conventional default group name.
- **Count shown**: yes.

### `pipeline`

- **Why related**: Pipelines consuming this project — when a CodePipeline stage is stuck, knowing which CodeBuild project powers it is the first triage step (`docs/related-resources.md § pipeline`: "CodeBuild projects used as pipeline actions").
- **How discovered**: reverse-scan the loaded `pipeline` list for any `stageStates[].actionStates[]` (or `PipelineDeclaration.stages[].actions[]`) with `ActionTypeId.Provider=="CodeBuild"` and `configuration.ProjectName==<this project name>` — a9s-devops: possible=yes, worth=yes. `Project` has no back-pointer to CodePipeline; the relationship is declared only on the pipeline side, so the pivot requires iterating cached pipelines.
- **Count shown**: yes.

### `role`

- **Why related**: IAM service role the project assumes to read source, write artifacts, and talk to KMS / Secrets Manager / Parameter Store — every "access denied" during a build starts here (`docs/related-resources.md § cb`: "Project.ServiceRole").
- **How discovered**: read `Project.ServiceRole` (AWS SDK Go v2 — `codebuild/types.Project § ServiceRole`); resolve the role ARN against the loaded `role` cache.
- **Count shown**: yes.

### `s3`

- **Why related**: Source / artifact buckets — the build pulls source from S3 and/or publishes build artifacts to S3, so operators pivot here to check object versions, ACLs, or retention (`docs/related-resources.md § cb`: "Source/artifact buckets").
- **How discovered**: read `Project.Source.Location` when `Project.Source.Type=="S3"`, `Project.SecondarySources[].Location` for the same, `Project.Artifacts.Location` when `Project.Artifacts.Type=="S3"`, `Project.SecondaryArtifacts[].Location`, and `Project.LogsConfig.S3Logs.Location` — a9s-devops: possible=yes, worth=yes. `ProjectSource.Location` is documented in AWS SDK Go v2 — `codebuild/types.ProjectSource § Type` (S3 case); the Location string for S3 sources/artifacts is `bucket/key`, from which the bucket name is the pivot key.
- **Count shown**: yes.

### `secrets`

- **Why related**: Secrets Manager secrets injected as build env variables — operators open these to confirm rotation state, ARN, or value when a build fails on credential resolution (`docs/related-resources.md § secrets`: "Reverse-scan: CodeBuild Project.Environment.EnvironmentVariables where Type=SECRETS_MANAGER and Value==ARN or name prefix").
- **How discovered**: read `Project.Environment.EnvironmentVariables[]` — entries with `Type==SECRETS_MANAGER` carry the secret ARN or name in `Value`; resolve against the loaded `secrets` cache (AWS SDK Go v2 — `codebuild/types.EnvironmentVariable § Type, Value`).
- **Count shown**: yes.

### `sg`

- **Why related**: Security groups attached to the build's VPC ENI — when a build running in VPC mode cannot reach a private database or internal registry, the SG is the first thing to inspect (`docs/related-resources.md § cb`: "VpcConfig.SecurityGroupIds").
- **How discovered**: read `Project.VpcConfig.SecurityGroupIds` (AWS SDK Go v2 — `codebuild/types.VpcConfig § SecurityGroupIds`); each SG ID resolves against the loaded `sg` cache.
- **Count shown**: yes.

### `ssm`

- **Why related**: SSM Parameter Store values injected as build env variables — operators pivot here to see the current value and last-modified date when a build picks up stale config (`docs/related-resources.md § cb`: "SSM parameters as build env").
- **How discovered**: read `Project.Environment.EnvironmentVariables[]` — entries with `Type==PARAMETER_STORE` carry the parameter name in `Value`; resolve against the loaded `ssm` cache (AWS SDK Go v2 — `codebuild/types.EnvironmentVariable § Type, Value`).
- **Count shown**: yes.

### `subnet`

- **Why related**: Subnets the build ENI lands in — IP exhaustion in one of these subnets causes `UNABLE_TO_CREATE_NETWORK_INTERFACE` build failures (`docs/related-resources.md § cb`: "VpcConfig.Subnets").
- **How discovered**: read `Project.VpcConfig.Subnets` (AWS SDK Go v2 — `codebuild/types.VpcConfig § Subnets`); each subnet ID resolves against the loaded `subnet` cache.
- **Count shown**: yes.

### `vpc`

- **Why related**: VPC the build runs inside — contextual pivot for flow logs, DNS resolution, and endpoint reachability when the build can't reach AWS APIs (`docs/related-resources.md § cb`: "VpcConfig.VpcId").
- **How discovered**: read `Project.VpcConfig.VpcId` (AWS SDK Go v2 — `codebuild/types.VpcConfig § VpcId`); resolve against the loaded `vpc` cache.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Audit trail for build events — "who deleted this project?", "when was ServiceRole last changed?" — CloudTrail is the universal answer (`docs/related-resources.md § cb`: "Audit trail for build events").
- **How discovered**: universal pivot — applies to every registered type; see docs/related-resources.md §Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

**Source API**: [BatchGetBuilds](https://docs.aws.amazon.com/codebuild/latest/APIReference/API_BatchGetBuilds.html)

Transcribed from `docs/attention-signals.md § Signals § CI/CD` row `cb`.

### 3.1 Wave 1 — zero extra API calls

`ListProjects` returns project-name strings only, so the fetcher reads the
projects with `BatchGetProjects` before it builds the rows. The four signals
below come off that response as the row is built, and they are independent: a
project that is wrong four ways carries four of them.

- **Signal**: the project's build results are readable without an AWS account (`ProjectVisibility == PUBLIC_READ`).
  - **State bucket**: Broken.
  - **API call**: `BatchGetProjects` — batched across projects, no per-project call.
  - **Cost shape**: per-sweep.

- **Signal**: the build instructions come from a file in the source repository rather than the project definition.
  - **State bucket**: Warning.
  - **API call**: same `BatchGetProjects` — `Source.Buildspec`. The buildspec path becomes a row under the finding.
  - **Cost shape**: per-sweep.

- **Signal**: the source repository address embeds a credential.
  - **State bucket**: Broken.
  - **API call**: same `BatchGetProjects` — `Source.Location`. The redacted address becomes a row under the finding.
  - **Cost shape**: per-sweep.

- **Signal**: a plaintext environment variable on the project holds what looks like a credential.
  - **State bucket**: Broken.
  - **API call**: same `BatchGetProjects` — `Environment.EnvironmentVariables`.
  - **Cost shape**: per-sweep.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: latest build `buildStatus` in `FAILED` / `FAULT` / `TIMED_OUT` (excluding user-initiated `STOPPED`).
  - **State bucket**: Broken.
  - **API call**: `ListBuildsForProject(maxResults=1)` per project (one per resource) + one batched `BatchGetBuilds` across the collected build IDs.
  - **Cost shape**: hybrid (per-resource list call + one batched Describe across all projects).

### 3.3 Wave 3 — OUT OF SCOPE

Copied verbatim from `docs/attention-signals.md § Not yet implemented`.

- OUT OF SCOPE: Stale-project (>90d); cache-config + perf signals.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `cb`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| build results public | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `build results publicly visible` |
| buildspec from the source repository | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `buildspec taken from the source repository` |
| credential in the source address | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `credential in the source repository address` |
| credential in a plaintext environment variable | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `credential in environment variables` |
| latest build `FAILED` / `FAULT` / `TIMED_OUT` | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `latest build <status>` |

Cause-field sources for S4 / S5: `Build.BuildStatus` (enum) and `Build.EndTime` from the batched `BatchGetBuilds` response (AWS SDK Go v2 — `codebuild/types.Build § BuildStatus, EndTime`). `FAULT` reflects a platform problem, `FAILED` a user-code exit, `TIMED_OUT` the project's `TimeoutInMinutes`; the list line pairs the status with the date so the operator sees whether it is fresh without opening detail.

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`FAILED`, `FAULT`, `TIMED_OUT`) in the List text column is not acceptable. The line pairs it with the date of the build.
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), the row is omitted entirely from this table; §3 still describes it.
- List text ≤ 40 chars. The Detail sentence lives on the finding definition and is generated into the Findings table below; it is never written here.

### 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — a red row for a failing project carries `latest build <status>` in S4, which names the outcome; the build's end date is one row down in the detail view, so the operator knows whether the failure is fresh or stale and can triage the team to investigate without first opening the detail view.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): stale-project (>90d); cache-config + perf signals.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?": "Read-only by design — a9s never makes write calls to AWS. Every AWS API call is a List, Describe, or Get operation.").
- Buildspec.yml content discovery (would require downloading the buildspec from S3/git). The `ecr` pivot therefore uses only `Environment.Image`, not push-target repos declared inside buildspec commands — a9s-devops: possible=no (no AWS API returns parsed buildspec references), captured here rather than as `TBD`.

## 6. Citations

- Display name and the project signals — `docs/attention-signals.md § Signals § CI/CD` row `cb`; list API — `core/aws/cb.go`.
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
- `ListProjects` is config-only, so no signal reads the list response alone — `core/aws/cb.go`.
- Wave 2 signal and API wiring — the latest build per project via `ListBuildsForProject(maxResults=1)` and batched `BatchGetBuilds`, with `FAILED`/`FAULT`/`TIMED_OUT` reading Broken and a user-initiated `STOPPED` reading nothing — `core/aws/cb_issue_enrichment.go`; the finding it raises is `docs/attention-signals.md § Signals § CI/CD` row `cb`.
- Wave 2 status field semantics — AWS SDK Go v2 — `codebuild/types.Build § BuildStatus, CurrentPhase, EndTime` and `codebuild/types.StatusType` (values `FAILED`, `FAULT`, `IN_PROGRESS`, `STOPPED`, `SUCCEEDED`, `TIMED_OUT`).
- Deferred signals (stale project >90d, cache configuration and performance) — `docs/attention-signals.md § Not yet implemented`.
- Buildspec-based ECR discovery exclusion — `a9s-devops (2026-04-20): possible=no, worth=n/a. AWS APIs do not return parsed buildspec content; the buildspec is either inline YAML or a file reference, and neither is exposed as structured references on Project.`

<!-- BEGIN GENERATED: header -->
cb — CI/CD. Lifecycle key: none (the list API returns no lifecycle field).
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| cb.latest-build-failed | latest build <status> | broken | wave2 | — |
| cb.public-builds | build results publicly visible | broken | wave1 | Build logs, environment variables and artifacts for this project are readable by anyone on the internet without an AWS account, so any credential or internal hostname a build prints is public. Set the project's visibility back to private and rotate anything the logs have already exposed. |
| cb.buildspec-from-source | buildspec taken from the source repository | warn | wave1 | The build instructions come from a file in the source repository, so anyone who can open a pull request can change what runs inside the build role. Move the buildspec inline into the project definition, or restrict who can trigger builds from unmerged branches. |
| cb.source-url-credential | credential in the source repository address | broken | wave1 | The source repository address embeds a username and password or token, which is stored in the project definition and printed in build logs in clear text. Move the credential into a CodeBuild source credential or Secrets Manager entry and rotate it, because it must be assumed leaked. |
| cb.env-secret | credential in environment variables | broken | wave1 | A plaintext environment variable on this project holds what looks like a credential; every build log and anyone who can read the project definition sees its value. Move it to Secrets Manager or Parameter Store, reference it by type, and rotate the exposed value. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| logs | Log Groups | yes |
| role | IAM Roles | no |
| pipeline | CodePipelines | yes |
| sg | Security Groups | no |
| subnet | Subnets | no |
| vpc | VPC | no |
| kms | KMS Key | no |
| alarm | CloudWatch Alarms | yes |
| ecr | ECR Repositories | yes |
| s3 | S3 Buckets | yes |
| secrets | Secrets Manager | no |
| ssm | SSM Parameters | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
