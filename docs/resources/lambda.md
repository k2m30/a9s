---
shortName: lambda
name: Lambda Functions
awsApiRef: https://docs.aws.amazon.com/lambda/latest/api/API_FunctionConfiguration.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# lambda — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `lambda`
- **Display name**: Lambda Functions
- **AWS API reference**: <https://docs.aws.amazon.com/lambda/latest/api/API_FunctionConfiguration.html>
- **List API**: `ListFunctions` (returns `FunctionConfiguration` entries — `Runtime`, `DeadLetterConfig`, `VpcConfig`, `Role`, `KMSKeyArn`, `FileSystemConfigs`; it returns none of `State`, `StateReasonCode` or `LastUpdateStatus`, which the SDK documents as `GetFunction`-only).
- **Describe API (if any)**: `GetFunction`, once per function in Wave 2, for `State` and `LastUpdateStatus` (§3.2).

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `alarm`, `apigw`, `cf`, `cfn`, `ct-events`, `ddb`, `eb-rule`, `ecr`, `efs`, `eni`, `kinesis`, `kms`, `logs`, `msk`, `role`, `s3`, `secrets`, `sg`, `sns`, `sns-sub`, `sqs`, `ssm`, `subnet`, `tg`, `vpc`.

### `alarm`

- **Why related**: CloudWatch alarms watching Lambda `Errors`, `Throttles`, `Duration`, `ConcurrentExecutions` for this function.
- **How discovered**: cross-reference the already-loaded `alarm` list — match `Dimensions[]` where `Name=="FunctionName"` and `Value==FunctionName`. Namespace `AWS/Lambda`. — a9s-devops: standard CloudWatch dimension schema for Lambda metrics.
- **Count shown**: yes.

### `apigw`

- **Why related**: API Gateway integrations that invoke this function.
- **How discovered**: the integration that names the function is the fact, and it lives on the API's routes, which `Api` does not embed and only `GetIntegrations` per API returns. Until that read exists, the row offers candidates — the loaded APIs that name this function in a tag key or in their own Name. — a9s-devops: this is an apigw-side reference (the integration lives on the route), not a Lambda-side field.
- **Count shown**: no — the candidates are not a count of the APIs that invoke this function.

### `cf`

- **Why related**: Lambda@Edge associations (CloudFront viewer/origin request/response triggers).
- **How discovered**: cross-reference `cf` distribution config — match `DefaultCacheBehavior.LambdaFunctionAssociations[].LambdaFunctionARN` and each `CacheBehaviors[].LambdaFunctionAssociations[]` against the function's versioned ARN. Lambda@Edge requires `us-east-1`. — a9s-devops: confirmed direction; `FunctionConfiguration` carries no CloudFront back-reference.
- **Count shown**: yes.

### `cfn`

- **Why related**: CloudFormation stack that created and manages the function.
- **How discovered**: read tag `aws:cloudformation:stack-name` from `ListTags` on the function ARN (or from the `Tags` field when loaded alongside), then cross-reference the `cfn` list by `StackName`. — a9s-devops: the `aws:cloudformation:*` tag set is how every CFN-managed resource exposes its origin stack.
- **Count shown**: yes.

### `ct-events`

- **Why related**: universal pivot — applies to every registered type; see docs/related-resources.md §Policy. Audit trail for `CreateFunction`, `UpdateFunctionCode`, `UpdateFunctionConfiguration`, `DeleteFunction`, `Invoke`.
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
- **How discovered**: `PackageType==Image`; the image URI is returned by `GetFunction` under `Code.ImageUri` (not on `ListFunctions`/`FunctionConfiguration`). Match the URI against the loaded `ecr` rows' own `RepositoryUri`: registry host and repository path both equal, with the `:tag` or `@digest` after them naming an image inside the repository. — a9s-devops: `Code.ImageUri` is only on the `GetFunction` output; surfacing requires a one-call-per-image-function fan-out.
- **Count shown**: yes.

### `efs`

- **Why related**: EFS file systems mounted into the function's `/mnt` at cold start.
- **How discovered**: read `FunctionConfiguration.FileSystemConfigs[].Arn` (the EFS access-point ARN); resolve back to the file system via the access point, then cross-reference the `efs` list by `FileSystemId`.
- **Count shown**: yes.

### `eni`

- **Why related**: Lambda-in-VPC creates requester-managed ENIs (Hyperplane ENIs) for outbound network access.
- **How discovered**: cross-reference the `eni` list — EC2 types a hyperplane ENI `InterfaceType == lambda`, and its `Description`, `AWS Lambda VPC ENI-<FunctionName>-<uuid>`, names the function it belongs to. — a9s-devops: `FunctionConfiguration` has no ENI-ID list, and `RequesterId` is an account or service alias that varies.
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
- **Invocation list**: the newest 50 invocation reports of that group in the 24 hours before the list opened, newest first — `REPORT` lines of the text log format and `platform.report` records of the JSON format, matched by one filter so a function that switched format lists both — read with the same windowed newest-first read as the ECS service log view, including its narrowing of a window that does not fit the call budget; Load More continues with the next older 50 in the same window. A report whose status is `timeout` reads `TIMEOUT` and one whose status is `error` or `failure` reads `ERROR` with its error type, each flagged broken; an init duration, or a SnapStart restore duration, marks a cold start. In a custom group, which other functions may share, only the function's own streams (`YYYY/MM/DD/<name>[<version>][<guid>]`) are read; finding a stream opened before the window relies on its `lastEventTimestamp`, which AWS "typically" updates within an hour, so in rare cases such a stream is missed; a stream listing cut at its page cap is reported as a partial failure when the read ends. An invocation's log is its own stream from its START to its REPORT, lines without the request ID included. See `docs/design/child-views/lambda-invocations.md` for the reads and the AWS pages they rest on.

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

**Source API**: [GetFunctionConfiguration](https://docs.aws.amazon.com/lambda/latest/api/API_GetFunctionConfiguration.html)

Transcribed from `docs/attention-signals.md § Signals § COMPUTE` row `lambda`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

A function that is `Active` with nothing else wrong raises no signal and
renders green and blank.

- **Signal**: `Runtime` whose AWS deprecation date — from the ["Deprecated runtimes" table, or the date the "Supported runtimes" table schedules](https://docs.aws.amazon.com/lambda/latest/dg/lambda-runtimes.html) — is on or before today.
  - **State bucket**: Broken.
  - **How obtained**: `ListFunctions` response field `Runtime`, looked up in the table's identifiers and deprecation dates baked into the build, and the date compared with the current date.
- **Signal**: `DeadLetterConfig==nil`.
  - **State bucket**: Warning.
  - **How obtained**: `ListFunctions` response field `DeadLetterConfig` (nil means async-invocation failures are silently dropped after retries).
- **Signal**: a plaintext environment variable holds what looks like a credential.
  - **State bucket**: Broken.
  - **How obtained**: `ListFunctions` response field `Environment.Variables`, scanned for credential-shaped values.

### 3.2 Wave 2 — bounded extra API calls

`State` and `LastUpdateStatus` come from one `GetFunction` call per function,
capped at the enrichment cap. A function past the cap, or whose `GetFunction`
read failed, is not inspected: its Status cell stays empty, never `Active`.

- **Signal**: `State` in `Pending`.
  - **State bucket**: Warning.
  - **API call**: `GetFunction` response field `Configuration.State`.
  - **Cost shape**: per-resource.
- **Signal**: `State` in `Inactive`.
  - **State bucket**: Dim.
  - **API call**: `GetFunction` response field `Configuration.State`. (Inactive means the function has been idle and will re-initialize on next invoke.)
  - **Cost shape**: per-resource.
- **Signal**: `State` in `Failed`.
  - **State bucket**: Broken.
  - **API call**: `GetFunction` response field `Configuration.State`; reason carried on `StateReason` + `StateReasonCode`.
  - **Cost shape**: per-resource.
- **Signal**: `LastUpdateStatus==Failed`.
  - **State bucket**: Broken.
  - **API call**: `GetFunction` response fields `Configuration.LastUpdateStatus` + `LastUpdateStatusReason` + `LastUpdateStatusReasonCode`.
  - **Cost shape**: per-resource.
- **Signal**: the function's resource policy allows a wildcard principal, so any AWS caller can invoke it.
  - A policy that does not parse leaves the row not inspected, never flagged.
  - **State bucket**: Broken.
  - **API call**: `GetPolicy` — one call per function.
  - **Cost shape**: per-resource.
- **Signal**: the function has a URL that requires no authentication (`AuthType == NONE`).
  - **State bucket**: Broken.
  - **API call**: `ListFunctionUrlConfigs` — one call per function.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `Errors/Invocations` ratio.
- OUT OF SCOPE: CloudWatch `Throttles`.
- OUT OF SCOPE: CloudWatch `Duration` p99 vs `Timeout`.
- OUT OF SCOPE: `GetFunctionConcurrency` per function.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `lambda`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `State==Pending` (`GetFunction`) | 2 | Warning | `~` | S2, S3, S4, S5 | `pending` |
| `State==Inactive` (`GetFunction`) | 2 | Dim | n/a | S2, S4 | `inactive, evicted after extended idle time` |
| `State==Failed` (`GetFunction`) | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `failed` |
| `LastUpdateStatus==Failed` (`GetFunction`) | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `last update failed to apply` |
| `Runtime` deprecated | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `runtime is end-of-life` |
| `DeadLetterConfig==nil` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `no dead-letter queue configured` |
| credential in `Environment.Variables` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `credential in environment variables` |
| resource policy allows a wildcard principal (`GetPolicy`) | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `invokable by anyone` |
| function URL with `AuthType == NONE` (`ListFunctionUrlConfigs`) | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `function endpoint open without authentication` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`DORMANT`, `stopped`, `available`, `failed`) in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column. Tests will assert the cause is present.
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), you may omit the row from this table entirely; §3 still describes it.
- Keep the List text short enough to fit: ≤ 40 chars. The Detail cell quotes the finding's Detail constant verbatim, however long it is.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for most rows — `failed`, `last update failed to apply`, `runtime is end-of-life`, `no dead-letter queue configured`, and `pending` all carry an actionable cause in 40 characters. One soft gap: `inactive, evicted after extended idle time` is grey-row informational only — the row colour already signals "nothing to act on" and the function may re-activate on the next invoke; the text exists so the operator does not mistake the dim row for a bug.

## 4.2 On-Demand Detail Enrichment

Opening the detail, YAML, or JSON view triggers one extra read-only call whose result is attached to the resource's raw structure for the stacked views. List views are never affected.

- AWS API: `GetFunction`
- Payload: the full function configuration — adds `State`, `StateReason`, `LastUpdateStatus*` (absent from `ListFunctions`, exactly the fields needed when a deploy is stuck) — plus code location and reserved concurrency. State, update status, and reserved concurrency render as detail-view fields
- Cache: none — a single cheap call, and the state fields change during deploys, which is precisely when the operator is looking
- Failure: a flash message; the view still renders the un-enriched resource

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
- `cf` reasoning (Lambda@Edge) — `docs/related-resources.md` § `cf` bullet `lambda`.
- `cf` discovery (`LambdaFunctionAssociations[].LambdaFunctionARN` in distribution config, us-east-1) — a9s-devops (2026-04-20): possible=yes, worth=yes.
- `cfn` reasoning (stack that created the function) — `docs/related-resources.md` § `lambda` bullet `cfn`.
- `cfn` discovery (`aws:cloudformation:stack-name` tag) — a9s-devops (2026-04-20): possible=yes, worth=yes. Standard CFN-managed resource tag.
- `ct-events` universality — `docs/related-resources.md` § Policy §4 ("Universal pivots").
- `ddb` reasoning (Streams triggers) — `docs/related-resources.md` § `lambda` bullet `ddb`.
- `ddb` discovery (`ListEventSourceMappings` with dynamodb `EventSourceArn`) — a9s-devops (2026-04-20): possible=yes, worth=yes. Canonical Lambda↔DDB Streams wiring.
- `eb-rule` reasoning (rules with this function as a target) — `docs/related-resources.md` § `lambda` bullet `eb-rule`.
- `eb-rule` discovery (`ListTargetsByRule` for each rule) — a9s-devops (2026-04-20): possible=yes, worth=yes. EventBridge targets are rule-side.
- `ecr` reasoning (container-image Lambda) — `docs/related-resources.md` § `lambda` bullet `ecr`.
- `ecr` discovery (`PackageType==Image` + `GetFunction.Code.ImageUri`) — AWS SDK Go v2 — lambda/types.FunctionConfiguration § `PackageType`; `GetFunction` output carries `Code.ImageUri` (not on `ListFunctions`).
- `efs` reasoning (FileSystemConfigs) — `docs/related-resources.md` § `lambda` bullet `efs`.
- `efs` discovery (`FileSystemConfigs[].Arn`) — AWS SDK Go v2 — lambda/types.FunctionConfiguration § `FileSystemConfigs`; `FileSystemConfig.Arn` is the EFS access-point ARN.
- `eni` reasoning (Lambda-in-VPC ENIs) — `docs/related-resources.md` § `lambda` bullet `eni`.
- `eni` discovery (requester-managed ENI with `AWS Lambda VPC ENI` description) — a9s-devops (2026-04-20): possible=yes, worth=yes. Documented ENI description pattern.
- `kinesis` reasoning (event-source mapping) — `docs/related-resources.md` § `lambda` bullet `kinesis`.
- `kinesis` discovery (`ListEventSourceMappings` with kinesis `EventSourceArn`) — a9s-devops (2026-04-20): possible=yes, worth=yes.
- `kms` reasoning (env-var encryption key) — `docs/related-resources.md` § `lambda` bullet `kms`.
- `kms` discovery (`KMSKeyArn` field) — AWS SDK Go v2 — lambda/types.FunctionConfiguration § `KMSKeyArn`.
- `logs` reasoning (`/aws/lambda/<name>`) — `docs/related-resources.md` § `lambda` bullet `logs`.
- `logs` discovery (`LoggingConfig.LogGroup` or default name) — AWS SDK Go v2 — lambda/types.FunctionConfiguration § `LoggingConfig`.
- `msk` reasoning (MSK event-source mapping) — `docs/related-resources.md` § `lambda` bullet `msk`.
- `msk` discovery (`ListEventSourceMappings` with kafka `EventSourceArn`) — a9s-devops (2026-04-20): possible=yes, worth=yes.
- `role` reasoning (execution permissions) — `docs/related-resources.md` § `lambda` bullet `role`.
- `role` discovery (`Role` field) — AWS SDK Go v2 — lambda/types.FunctionConfiguration § `Role`.
- `s3` reasoning (S3 event source) — `docs/related-resources.md` § `lambda` bullet `s3`.
- `s3` discovery (`GetBucketNotificationConfiguration.LambdaFunctionConfigurations[]`) — a9s-devops (2026-04-20): possible=yes, worth=yes. S3→Lambda wiring is bucket-side.
- `secrets` reasoning (secrets accessed at runtime) — `docs/related-resources.md` § `lambda` bullet `secrets`.
- `secrets` discovery (env-var scan + role policy scan) — a9s-devops (2026-04-20): possible=yes (weak heuristic), worth=yes (incident-triage pivot). Expect false negatives on runtime-composed ARNs.
- `sg` reasoning (function ENI SGs) — `docs/related-resources.md` § `lambda` bullet `sg`.
- `sg` discovery (`VpcConfig.SecurityGroupIds`) — AWS SDK Go v2 — lambda/types.VpcConfigResponse § `SecurityGroupIds`.
- `sns` reasoning (SNS event source mapping / DLQ) — `docs/related-resources.md` § `lambda` bullet `sns`.
- `sns` discovery (`DeadLetterConfig.TargetArn` SNS ARN + topic-side subscription listing) — AWS SDK Go v2 — lambda/types.DeadLetterConfig § `TargetArn`.
- `sns-sub` reasoning (SNS subscriptions delivering to the function) — `docs/related-resources.md` § `lambda` bullet `sns-sub`.
- `sns-sub` discovery (`Protocol==lambda` + `Endpoint==FunctionArn`) — a9s-devops (2026-04-20): possible=yes, worth=yes. Standard SNS subscription attributes.
- `sqs` reasoning (event source or DLQ) — `docs/related-resources.md` § `lambda` bullet `sqs`.
- `sqs` discovery (`DeadLetterConfig.TargetArn` SQS ARN + `ListEventSourceMappings` with sqs `EventSourceArn`) — AWS SDK Go v2 — lambda/types.DeadLetterConfig § `TargetArn`.
- `ssm` reasoning (parameters as config) — `docs/related-resources.md` § `lambda` bullet `ssm`.
- `ssm` discovery (env-var scan + role policy scan) — a9s-devops (2026-04-20): possible=yes (weak heuristic), worth=yes (config-pivot for incident triage).
- `subnet` reasoning (function ENI subnets) — `docs/related-resources.md` § `lambda` bullet `subnet`.
- `subnet` discovery (`VpcConfig.SubnetIds`) — AWS SDK Go v2 — lambda/types.VpcConfigResponse § `SubnetIds`.
- `tg` reasoning (TargetGroup registration) — `docs/related-resources.md` § `lambda` bullet `tg`.
- `tg` discovery (TG `TargetType==lambda` + `DescribeTargetHealth` match on `FunctionArn`) — a9s-devops (2026-04-20): possible=yes, worth=yes. ALB→Lambda wiring is TG-side.
- `vpc` reasoning (VPC the function runs in) — `docs/related-resources.md` § `lambda` bullet `vpc`.
- `vpc` discovery (`VpcConfig.VpcId`) — AWS SDK Go v2 — lambda/types.VpcConfigResponse § `VpcId`.
- Wave 1 signals list — `docs/attention-signals.md § Signals § COMPUTE` row `lambda`.
- Wave 2 `State` values `Active`/`Pending`/`Inactive`/`Failed` — AWS SDK Go v2 — lambda/types.State § constants; `ListFunctions` does not return `State` (lambda/types.FunctionConfiguration § `State`: "use GetFunction").
- Wave 2 `LastUpdateStatus==Failed` — AWS SDK Go v2 — lambda/types.FunctionConfiguration § `LastUpdateStatus` and lambda/types.LastUpdateStatus § constants.
- Wave 1 runtime deprecation dates — AWS docs `lambda/latest/dg/lambda-runtimes.html` § Deprecated runtimes and § Supported runtimes (fetched 2026-09-23); the finding it feeds is `docs/attention-signals.md § Signals § COMPUTE` row `lambda`.
- Wave 1 `DeadLetterConfig==nil` — AWS SDK Go v2 — lambda/types.FunctionConfiguration § `DeadLetterConfig` and lambda/types.DeadLetterConfig § `TargetArn`.
- The wave-2 resource-policy and function-URL findings — `docs/attention-signals.md § Signals § COMPUTE` row `lambda`.
- Wave 3 OUT OF SCOPE items — `docs/attention-signals.md § Not yet implemented`.
- Read-only invariant — `docs/architecture.md` § "What is a9s?" (a9s makes no AWS write calls).
- S4 list-text wording (`creating`, `idle: not invoked recently`, `failed: <StateReasonCode>`, `update failed: <LastUpdateStatusReasonCode>`, `runtime deprecated: <Runtime>`, `no DLQ — async failures dropped`) — a9s-devops (2026-04-20): possible=yes, worth=yes. Paired state+cause wording per the skill's §4 rules; `StateReasonCode` / `LastUpdateStatusReasonCode` are AWS-provided short codes suitable for a 40-char status cell (AWS SDK Go v2 — lambda/types.StateReasonCode, lambda/types.LastUpdateStatusReasonCode).

<!-- BEGIN GENERATED: header -->
lambda — COMPUTE. Status key: `state` — the key the status cell reads, and the column naming it is the status column.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| lambda.last-update.failed | last update failed to apply | broken | wave2 | The last configuration or code update did not take, so the function still runs the previous version while the console shows what you asked for. Read the update status reason — a bad VPC configuration, an invalid role or a missing layer are typical — fix it, and apply the update again. |
| lambda.runtime.deprecated | runtime is end-of-life | broken | wave1 | This function runs on a runtime AWS no longer patches, so language and base-image security fixes will never reach it. Existing functions keep being invoked, but AWS first stops you creating new functions on it and then stops you updating this one, which turns an urgent fix into a migration under pressure. Move to a supported runtime version and redeploy while the update path is still open. |
| lambda.state.pending | pending | warn | wave2 | The function is still being created or attached to its VPC, and invocations during this window are throttled or rejected. Wait for it to become active before wiring an event source to it. |
| lambda.state.failed | failed | broken | wave2 | The function cannot be invoked at all: its creation or VPC setup failed and it has no working execution environment. Read its state reason, fix the role, subnets or security groups it names, then update the function to retry. |
| lambda.state.inactive | inactive, evicted after extended idle time | dim | wave2 | — |
| lambda.dlq.missing | no dead-letter queue configured | warn | wave1 | Asynchronous invocations that exhaust their retries are dropped silently, so a bad deployment or a downstream outage loses events with no record of what was lost. Set a dead-letter queue or an on-failure destination so failed events can be inspected and replayed. |
| lambda.env-secret | credential in environment variables | broken | wave1 | A credential is stored as a plaintext environment variable on this function, readable by anyone who can call lambda:GetFunctionConfiguration. Move the value to Secrets Manager or Systems Manager Parameter Store, read it at cold start, and rotate the exposed one. |
| lambda.public-policy | invokable by anyone | broken | wave2 | The function's resource policy allows a wildcard principal, so any AWS caller can invoke it and whatever it does downstream runs on your account's bill and permissions. Replace the `*` principal with the specific account, service, or ARN that should be allowed to call it. |
| lambda.function-url-public | function endpoint open without authentication | broken | wave2 | The function has a web endpoint that requires no authentication, so anyone on the internet who learns the address can invoke it without credentials. Set the endpoint to require signed requests, or put an authorizing layer in front of it. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| role | IAM Roles | no |
| alarm | CW Alarms | yes |
| logs | Log Groups | yes |
| sg | Security Groups | no |
| vpc | VPC | no |
| kms | KMS Key | no |
| sqs | SQS Queues | yes |
| cfn | CloudFormation | yes |
| eb-rule | EventBridge Rules | yes |
| subnet | Subnets | no |
| efs | EFS File Systems | no |
| apigw | API Gateways | yes |
| cf | CloudFront | yes |
| ddb | DynamoDB Tables | yes |
| kinesis | Kinesis Streams | yes |
| msk | MSK Clusters | yes |
| ct-events | CloudTrail Events | no |
| tg | Target Groups | yes |
| sns | SNS Topics | yes |
| sns-sub | SNS Subscriptions | yes |
| s3 | S3 Buckets | yes |
| ecr | ECR Repositories | yes |
| eni | Network Interfaces | yes |
| secrets | Secrets | yes |
| ssm | SSM Parameters | yes |
<!-- END GENERATED: related -->
