---
shortName: lambda
name: Lambda Functions
awsApiRef: https://docs.aws.amazon.com/lambda/latest/api/API_FunctionConfiguration.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# lambda — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `lambda`
- **Display name**: Lambda Functions
- **AWS API reference**: <https://docs.aws.amazon.com/lambda/latest/api/API_FunctionConfiguration.html>
- **List API**: `ListFunctions` (returns `FunctionConfiguration` entries — same shape as `GetFunctionConfiguration`; includes `State`, `LastUpdateStatus`, `Runtime`, `DeadLetterConfig`, `VpcConfig`, `Role`, `KMSKeyArn`, `FileSystemConfigs`).
- **Describe API (if any)**: not used. All Wave 1 fields are already on the `ListFunctions` response. `attention-signals.md` lists Wave 2 as `None`.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `apigw`, `cf`, `cfn`, `ct-events`, `ddb`, `eb-rule`, `ecr`, `efs`, `eni`, `kinesis`, `kms`, `logs`, `msk`, `role`, `s3`, `secrets`, `sg`, `sns`, `sns-sub`, `sqs`, `ssm`, `subnet`, `tg`, `vpc`.

### `alarm`

- **Why related**: CloudWatch alarms watching Lambda `Errors`, `Throttles`, `Duration`, `ConcurrentExecutions` for this function.
- **How discovered**: cross-reference the already-loaded `alarm` list — match `Dimensions[]` where `Name=="FunctionName"` and `Value==FunctionName`. Namespace `AWS/Lambda`. — a9s-devops: standard CloudWatch dimension schema for Lambda metrics.
- **Count shown**: yes.

### `apigw`

- **Why related**: API Gateway integrations that invoke this function.
- **How discovered**: cross-reference `apigw` stage/integration metadata — match API Gateway integration `Uri` containing `/functions/<FunctionArn>/invocations` against the function's ARN. — a9s-devops: this is an apigw-side reference (the integration lives on the route), not a Lambda-side field.
- **Count shown**: yes.

### `cf`

- **Why related**: Lambda@Edge associations (CloudFront viewer/origin request/response triggers).
- **How discovered**: cross-reference `cf` distribution config — match `DefaultCacheBehavior.LambdaFunctionAssociations[].LambdaFunctionARN` and each `CacheBehaviors[].LambdaFunctionAssociations[]` against the function's versioned ARN. Lambda@Edge requires `us-east-1`. — a9s-devops: confirmed direction; `FunctionConfiguration` carries no CloudFront back-reference.
- **Count shown**: yes.

### `cfn`

- **Why related**: CloudFormation stack that created and manages the function.
- **How discovered**: read tag `aws:cloudformation:stack-name` from `ListTags` on the function ARN (or from the `Tags` field when loaded alongside), then cross-reference the `cfn` list by `StackName`. — a9s-devops: the `aws:cloudformation:*` tag set is how every CFN-managed resource exposes its origin stack.
- **Count shown**: yes.

### `ct-events`

- **Why related**: universal pivot — applies to every registered type; see related-resources.md §Policy. Audit trail for `CreateFunction`, `UpdateFunctionCode`, `UpdateFunctionConfiguration`, `DeleteFunction`, `Invoke`.
- **How discovered**: `LookupEvents` with `ResourceName=<FunctionName>` / `ResourceType=AWS::Lambda::Function`.
- **Count shown**: yes.

### `ddb`

- **Why related**: DynamoDB Streams that invoke this function as an event source.
- **How discovered**: call `ListEventSourceMappings(FunctionName=<name>)` — entries with `EventSourceArn` starting `arn:aws:dynamodb:…/stream/…` identify the source DDB table (table name is the segment before `/stream/`). Cross-reference against the loaded `ddb` list. — a9s-devops: event-source mappings are the canonical Lambda↔DDB Streams wiring.
- **Count shown**: yes.

### `eb-rule`

- **Why related**: EventBridge rules with this function as a target.
- **How discovered**: cross-reference `eb-rule` list — for each rule call `ListTargetsByRule`, match any `Targets[].Arn==FunctionArn`. — a9s-devops: EventBridge targets live rule-side; the function has no back-reference.
- **Count shown**: yes.

### `ecr`

- **Why related**: container-image Lambda — the function runs the image from this ECR repository.
- **How discovered**: `PackageType==Image`; the image URI is returned by `GetFunction` under `Code.ImageUri` (not on `ListFunctions`/`FunctionConfiguration`). Parse the repository name from the URI (`<acct>.dkr.ecr.<region>.amazonaws.com/<repo>:<tag>`) and cross-reference the `ecr` list. — a9s-devops: `Code.ImageUri` is only on the `GetFunction` output; surfacing requires a one-call-per-image-function fan-out.
- **Count shown**: yes.

### `efs`

- **Why related**: EFS file systems mounted into the function's `/mnt` at cold start.
- **How discovered**: read `FunctionConfiguration.FileSystemConfigs[].Arn` (the EFS access-point ARN); resolve back to the file system via the access point, then cross-reference the `efs` list by `FileSystemId`.
- **Count shown**: yes.

### `eni`

- **Why related**: Lambda-in-VPC creates requester-managed ENIs (Hyperplane ENIs) for outbound network access.
- **How discovered**: cross-reference the `eni` list — match `RequesterId=="AWS Lambda VPC ENI"` / `Description` starting with `AWS Lambda VPC ENI-<FunctionName>-…`. — a9s-devops: confirmed ENI description pattern; `FunctionConfiguration` has no ENI-ID list.
- **Count shown**: yes.

### `kinesis`

- **Why related**: Kinesis stream consumed as an event source.
- **How discovered**: `ListEventSourceMappings(FunctionName=<name>)` — entries with `EventSourceArn` starting `arn:aws:kinesis:…/stream/…` identify the stream. Cross-reference the `kinesis` list by stream name.
- **Count shown**: yes.

### `kms`

- **Why related**: customer-managed KMS key used to encrypt environment variables, SnapStart snapshots, and cached container images.
- **How discovered**: read `FunctionConfiguration.KMSKeyArn`; cross-reference the `kms` list by key ARN/ID.
- **Count shown**: yes.

### `logs`

- **Why related**: CloudWatch Logs log group `/aws/lambda/<FunctionName>` where function logs land (or the custom group from `LoggingConfig.LogGroup`).
- **How discovered**: read `FunctionConfiguration.LoggingConfig.LogGroup` when set; otherwise default `/aws/lambda/<FunctionName>`. Cross-reference the `logs` list. — a9s-devops: `LoggingConfig` added to `FunctionConfiguration` in the 2023 advanced logging release.
- **Count shown**: yes.

### `msk`

- **Why related**: Amazon MSK cluster consumed as an event source.
- **How discovered**: `ListEventSourceMappings(FunctionName=<name>)` — entries with `EventSourceArn` starting `arn:aws:kafka:…/cluster/…` identify the MSK cluster. Cross-reference the `msk` list by cluster ARN.
- **Count shown**: yes.

### `role`

- **Why related**: execution role — the IAM identity Lambda assumes to invoke the function (logs, network, downstream AWS calls).
- **How discovered**: read `FunctionConfiguration.Role` (role ARN); cross-reference the `role` list by role name.
- **Count shown**: yes.

### `s3`

- **Why related**: S3 bucket whose object events (`s3:ObjectCreated:*`, etc.) invoke this function.
- **How discovered**: S3 → Lambda wiring lives on the bucket side, not the function side — cross-reference the `s3` list by `GetBucketNotificationConfiguration().LambdaFunctionConfigurations[].LambdaFunctionArn==FunctionArn`. — a9s-devops: the bucket holds the notification config; the function exposes no back-reference.
- **Count shown**: yes.

### `secrets`

- **Why related**: Secrets Manager secrets the function reads at runtime (DB creds, API keys).
- **How discovered**: two weak heuristics — (a) `FunctionConfiguration.Environment.Variables` values containing a Secrets Manager ARN or `secretsmanager:` reference, and (b) the execution role's attached policies granting `secretsmanager:GetSecretValue` on specific ARNs. No strong direct API linkage. — a9s-devops: possible=yes (env-var grep is the pragmatic pivot), worth=yes (operators jump from Lambda to its secrets during incident triage); neither is authoritative, so expect false negatives on runtime-assembled ARNs.
- **Count shown**: yes.

### `sg`

- **Why related**: security groups attached to the function's VPC ENIs — govern outbound reachability.
- **How discovered**: read `FunctionConfiguration.VpcConfig.SecurityGroupIds`; cross-reference the `sg` list by group ID.
- **Count shown**: yes.

### `sns`

- **Why related**: SNS topic that publishes to this function (asynchronous event source) or is the function's async-invocation DLQ target.
- **How discovered**: two pivots — (a) `FunctionConfiguration.DeadLetterConfig.TargetArn` when it's an SNS ARN (`arn:aws:sns:…`), and (b) SNS topic subscriptions with `Protocol==lambda` and `Endpoint==FunctionArn` (the topic-side view, collapsed into `sns-sub` below). — a9s-devops: the DLQ-SNS direction is function-side; the subscription direction is topic-side.
- **Count shown**: yes.

### `sns-sub`

- **Why related**: the specific SNS subscription(s) that deliver notifications to this function.
- **How discovered**: cross-reference the `sns-sub` list — match `Protocol=="lambda"` and `Endpoint==FunctionArn`. — a9s-devops: `sns` gives the topic; `sns-sub` gives the actual subscription row so the operator can see confirmation state.
- **Count shown**: yes.

### `sqs`

- **Why related**: SQS queue either invoking the function (event source) or used as the function's async-invocation DLQ.
- **How discovered**: two pivots — (a) `FunctionConfiguration.DeadLetterConfig.TargetArn` when it's an SQS ARN (`arn:aws:sqs:…`); (b) `ListEventSourceMappings(FunctionName=<name>)` entries with `EventSourceArn` starting `arn:aws:sqs:…`. Cross-reference the `sqs` list by queue name.
- **Count shown**: yes.

### `ssm`

- **Why related**: SSM Parameter Store parameters the function reads at runtime for configuration.
- **How discovered**: weak heuristic — scan `FunctionConfiguration.Environment.Variables` values for SSM parameter paths / ARNs, and inspect the execution role's `ssm:GetParameter*` allow statements. No direct API linkage. — a9s-devops: possible=yes (env-var hints + policy scan), worth=yes (config-pivot speeds incident triage); expect false negatives on dynamically-composed paths.
- **Count shown**: yes.

### `subnet`

- **Why related**: subnets that host the function's VPC ENIs — determine AZ reach and IP availability at cold start.
- **How discovered**: read `FunctionConfiguration.VpcConfig.SubnetIds`; cross-reference the `subnet` list by subnet ID.
- **Count shown**: yes.

### `tg`

- **Why related**: Application Load Balancer target group with this function registered as a target.
- **How discovered**: cross-reference the `tg` list — for each TG with `TargetType==lambda`, call `DescribeTargetHealth` and match `Targets[].Id==FunctionArn`. — a9s-devops: ALB→Lambda wiring is TG-side (`TargetType=lambda`); the function has no back-reference.
- **Count shown**: yes.

### `vpc`

- **Why related**: VPC the function runs in when VPC-configured.
- **How discovered**: read `FunctionConfiguration.VpcConfig.VpcId`; cross-reference the `vpc` list by VPC ID.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `State` in `Active` → Healthy.
  - **State bucket**: Healthy.
  - **How obtained**: `ListFunctions` response field `State` on each `FunctionConfiguration` entry.
- **Signal**: `State` in `Pending` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `ListFunctions` response field `State`.
- **Signal**: `State` in `Inactive` → Dim.
  - **State bucket**: Dim.
  - **How obtained**: `ListFunctions` response field `State`. (Inactive means the function has been idle and will re-initialize on next invoke.)
- **Signal**: `State` in `Failed` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `ListFunctions` response field `State`; reason carried on `StateReason` + `StateReasonCode`.
- **Signal**: `LastUpdateStatus==Failed` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `ListFunctions` response fields `LastUpdateStatus` + `LastUpdateStatusReason` + `LastUpdateStatusReasonCode`.
- **Signal**: `Runtime` in [deprecated-runtimes list](https://docs.aws.amazon.com/lambda/latest/dg/lambda-runtimes.html) → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `ListFunctions` response field `Runtime` compared against the AWS-published deprecated-runtimes list baked into the build.
- **Signal**: `DeadLetterConfig==nil` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `ListFunctions` response field `DeadLetterConfig` (nil means async-invocation failures are silently dropped after retries).

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `Errors/Invocations` ratio.
- OUT OF SCOPE: CloudWatch `Throttles`.
- OUT OF SCOPE: CloudWatch `Duration` p99 vs `Timeout`.
- OUT OF SCOPE: `GetFunctionConcurrency` per function.

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
| `State==Pending` | 1 | Warning | n/a | S2, S4 | `creating` | — |
| `State==Inactive` | 1 | Dim | n/a | S2, S4 | `idle: not invoked recently` | — |
| `State==Failed` | 1 | Broken | n/a | S2, S4 | `failed: <StateReasonCode>` | — |
| `LastUpdateStatus==Failed` | 1 | Broken | n/a | S2, S4 | `update failed: <LastUpdateStatusReasonCode>` | — |
| `Runtime` deprecated | 1 | Broken | n/a | S2, S4 | `runtime deprecated: <Runtime>` | — |
| `DeadLetterConfig==nil` | 1 | Warning | n/a | S2, S4 | `no DLQ — async failures dropped` | — |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`DORMANT`, `stopped`, `available`, `failed`) in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column. Tests will assert the cause is present.
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), you may omit the row from this table entirely; §3 still describes it.
- Keep both columns short enough to fit: List text ≤ 40 chars, Detail text ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for most rows — `failed: <reason-code>`, `update failed: <reason-code>`, `runtime deprecated: <name>`, `no DLQ — async failures dropped`, and `creating` all carry actionable cause in 40 chars. One soft gap: `idle: not invoked recently` for `State==Inactive` is grey-row informational only — the row color already signals "nothing to act on" and this resource may re-activate on next invoke; the text exists so the operator doesn't mistake the dim row for a bug.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): CloudWatch `Errors/Invocations`, `Throttles`, `Duration` p99 vs `Timeout`, and `GetFunctionConcurrency` per function.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

One bullet per claim in §§2–4.1. Citation sources, in order of authority:

- Per-type contract targets — `docs/related-resources.md` § Per-type contract row `lambda`.
- `alarm` reasoning ("Errors/Throttles/Duration alarms") — `docs/related-resources.md` § `lambda` bullet `alarm`.
- `alarm` discovery (FunctionName dimension on `AWS/Lambda` namespace) — a9s-devops (2026-04-20): possible=yes, worth=yes. Standard CloudWatch schema; no Lambda-side alarm back-reference.
- `apigw` reasoning (API Gateway integrations) — `docs/related-resources.md` § `lambda` bullet `apigw`.
- `apigw` discovery (integration Uri match) — a9s-devops (2026-04-20): possible=yes, worth=yes. Integration reference lives on the API Gateway route.
- `cf` reasoning (Lambda@Edge) — `docs/related-resources.md` § `cf` bullet `lambda` and `docs/related-resources.md` line 241.
- `cf` discovery (`LambdaFunctionAssociations[].LambdaFunctionARN` in distribution config, us-east-1) — a9s-devops (2026-04-20): possible=yes, worth=yes.
- `cfn` reasoning (stack that created the function) — `docs/related-resources.md` § `lambda` bullet `cfn`.
- `cfn` discovery (`aws:cloudformation:stack-name` tag) — a9s-devops (2026-04-20): possible=yes, worth=yes. Standard CFN-managed resource tag.
- `ct-events` universality — `docs/related-resources.md` § Policy §4 ("Universal pivots").
- `ddb` reasoning (Streams triggers) — `docs/related-resources.md` § `lambda` bullet `ddb`.
- `ddb` discovery (`ListEventSourceMappings` with dynamodb `EventSourceArn`) — a9s-devops (2026-04-20): possible=yes, worth=yes. Canonical Lambda↔DDB Streams wiring.
- `eb-rule` reasoning (rules with this function as a target) — `docs/related-resources.md` § `lambda` bullet `eb-rule` and line 364.
- `eb-rule` discovery (`ListTargetsByRule` for each rule) — a9s-devops (2026-04-20): possible=yes, worth=yes. EventBridge targets are rule-side.
- `ecr` reasoning (container-image Lambda) — `docs/related-resources.md` § `lambda` bullet `ecr` and line 429.
- `ecr` discovery (`PackageType==Image` + `GetFunction.Code.ImageUri`) — AWS SDK Go v2 — lambda/types.FunctionConfiguration § `PackageType`; `GetFunction` output carries `Code.ImageUri` (not on `ListFunctions`).
- `efs` reasoning (FileSystemConfigs) — `docs/related-resources.md` § `lambda` bullet `efs` and line 498.
- `efs` discovery (`FileSystemConfigs[].Arn`) — AWS SDK Go v2 — lambda/types.FunctionConfiguration § `FileSystemConfigs`; `FileSystemConfig.Arn` is the EFS access-point ARN.
- `eni` reasoning (Lambda-in-VPC ENIs) — `docs/related-resources.md` § `lambda` bullet `eni` and line 563.
- `eni` discovery (requester-managed ENI with `AWS Lambda VPC ENI` description) — a9s-devops (2026-04-20): possible=yes, worth=yes. Documented ENI description pattern.
- `kinesis` reasoning (event-source mapping) — `docs/related-resources.md` § `lambda` bullet `kinesis` and line 617.
- `kinesis` discovery (`ListEventSourceMappings` with kinesis `EventSourceArn`) — a9s-devops (2026-04-20): possible=yes, worth=yes.
- `kms` reasoning (env-var encryption key) — `docs/related-resources.md` § `lambda` bullet `kms`.
- `kms` discovery (`KMSKeyArn` field) — AWS SDK Go v2 — lambda/types.FunctionConfiguration § `KMSKeyArn`.
- `logs` reasoning (`/aws/lambda/<name>`) — `docs/related-resources.md` § `lambda` bullet `logs`.
- `logs` discovery (`LoggingConfig.LogGroup` or default name) — AWS SDK Go v2 — lambda/types.FunctionConfiguration § `LoggingConfig`.
- `msk` reasoning (MSK event-source mapping) — `docs/related-resources.md` § `lambda` bullet `msk` and line 681.
- `msk` discovery (`ListEventSourceMappings` with kafka `EventSourceArn`) — a9s-devops (2026-04-20): possible=yes, worth=yes.
- `role` reasoning (execution permissions) — `docs/related-resources.md` § `lambda` bullet `role` and line 819.
- `role` discovery (`Role` field) — AWS SDK Go v2 — lambda/types.FunctionConfiguration § `Role`.
- `s3` reasoning (S3 event source) — `docs/related-resources.md` § `lambda` bullet `s3` and line 850.
- `s3` discovery (`GetBucketNotificationConfiguration.LambdaFunctionConfigurations[]`) — a9s-devops (2026-04-20): possible=yes, worth=yes. S3→Lambda wiring is bucket-side.
- `secrets` reasoning (secrets accessed at runtime) — `docs/related-resources.md` § `lambda` bullet `secrets`.
- `secrets` discovery (env-var scan + role policy scan) — a9s-devops (2026-04-20): possible=yes (weak heuristic), worth=yes (incident-triage pivot). Expect false negatives on runtime-composed ARNs.
- `sg` reasoning (function ENI SGs) — `docs/related-resources.md` § `lambda` bullet `sg`.
- `sg` discovery (`VpcConfig.SecurityGroupIds`) — AWS SDK Go v2 — lambda/types.VpcConfigResponse § `SecurityGroupIds`.
- `sns` reasoning (SNS event source mapping / DLQ) — `docs/related-resources.md` § `lambda` bullet `sns`.
- `sns` discovery (`DeadLetterConfig.TargetArn` SNS ARN + topic-side subscription listing) — AWS SDK Go v2 — lambda/types.DeadLetterConfig § `TargetArn`.
- `sns-sub` reasoning (SNS subscriptions delivering to the function) — `docs/related-resources.md` § `lambda` bullet `sns-sub` and line 928.
- `sns-sub` discovery (`Protocol==lambda` + `Endpoint==FunctionArn`) — a9s-devops (2026-04-20): possible=yes, worth=yes. Standard SNS subscription attributes.
- `sqs` reasoning (event source or DLQ) — `docs/related-resources.md` § `lambda` bullet `sqs` and line 940.
- `sqs` discovery (`DeadLetterConfig.TargetArn` SQS ARN + `ListEventSourceMappings` with sqs `EventSourceArn`) — AWS SDK Go v2 — lambda/types.DeadLetterConfig § `TargetArn`.
- `ssm` reasoning (parameters as config) — `docs/related-resources.md` § `lambda` bullet `ssm`.
- `ssm` discovery (env-var scan + role policy scan) — a9s-devops (2026-04-20): possible=yes (weak heuristic), worth=yes (config-pivot for incident triage).
- `subnet` reasoning (function ENI subnets) — `docs/related-resources.md` § `lambda` bullet `subnet`.
- `subnet` discovery (`VpcConfig.SubnetIds`) — AWS SDK Go v2 — lambda/types.VpcConfigResponse § `SubnetIds`.
- `tg` reasoning (TargetGroup registration) — `docs/related-resources.md` § `lambda` bullet `tg` and line 983.
- `tg` discovery (TG `TargetType==lambda` + `DescribeTargetHealth` match on `FunctionArn`) — a9s-devops (2026-04-20): possible=yes, worth=yes. ALB→Lambda wiring is TG-side.
- `vpc` reasoning (VPC the function runs in) — `docs/related-resources.md` § `lambda` bullet `vpc`.
- `vpc` discovery (`VpcConfig.VpcId`) — AWS SDK Go v2 — lambda/types.VpcConfigResponse § `VpcId`.
- Wave 1 signals list — `docs/attention-signals.md` § Compute row `lambda` Wave 1 cell.
- Wave 1 `State` values `Active`/`Pending`/`Inactive`/`Failed` — AWS SDK Go v2 — lambda/types.State § constants.
- Wave 1 `LastUpdateStatus==Failed` — AWS SDK Go v2 — lambda/types.FunctionConfiguration § `LastUpdateStatus` and lambda/types.LastUpdateStatus § constants.
- Wave 1 deprecated-runtimes list URL — `docs/attention-signals.md` § Compute row `lambda` Wave 1 cell (links to AWS docs `lambda/latest/dg/lambda-runtimes.html`).
- Wave 1 `DeadLetterConfig==nil` — AWS SDK Go v2 — lambda/types.FunctionConfiguration § `DeadLetterConfig` and lambda/types.DeadLetterConfig § `TargetArn`.
- Wave 2 "None" — `docs/attention-signals.md` § Compute row `lambda` Wave 2 cell.
- Wave 3 OUT OF SCOPE items — `docs/attention-signals.md` § Compute row `lambda` Wave 3 cell.
- Read-only invariant — `docs/architecture.md` § "What is a9s?" (a9s makes no AWS write calls).
- S4 list-text wording (`creating`, `idle: not invoked recently`, `failed: <StateReasonCode>`, `update failed: <LastUpdateStatusReasonCode>`, `runtime deprecated: <Runtime>`, `no DLQ — async failures dropped`) — a9s-devops (2026-04-20): possible=yes, worth=yes. Paired state+cause wording per the skill's §4 rules; `StateReasonCode` / `LastUpdateStatusReasonCode` are AWS-provided short codes suitable for a 40-char status cell (AWS SDK Go v2 — lambda/types.StateReasonCode, lambda/types.LastUpdateStatusReasonCode).
