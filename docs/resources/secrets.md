---
shortName: secrets
name: Secrets Manager
awsApiRef: https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_SecretListEntry.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# secrets — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `secrets`
- **Display name**: Secrets Manager
- **AWS API reference**: https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_SecretListEntry.html
- **List API**: `ListSecrets`
- **Describe API (if any)**: `DescribeSecret` (Wave 2 only — per-secret, to inspect `VersionIdsToStages` for stuck `AWSPENDING` versions)

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `cb`, `cfn`, `codeartifact`, `dbi`, `eb`, `ecs-task`, `kms`, `lambda`, `logs`, `role`, `sns`, `ct-events`.

### `cb`

- **Why related**: CodeBuild projects reference secrets as build-time environment variables — operators chasing a leaked/rotated secret need to find every build that consumes it.
- **How discovered**: Reverse-scan the already-loaded CodeBuild project list; match `Project.Environment.EnvironmentVariables[]` where `Type==SECRETS_MANAGER` AND `Value` equals this secret's ARN or name prefix.
- **Count shown**: yes.

### `cfn`

- **Why related**: Secrets created by CloudFormation carry the stack-name tag — operators want to jump to the stack that owns the secret.
- **How discovered**: Read `SecretListEntry.Tags["aws:cloudformation:stack-name"]` on the secret and match against the CFN stack cache.
- **Count shown**: yes.

### `codeartifact`

- **Why related**: CodeArtifact authorization tokens are commonly stashed in Secrets Manager; operators diagnosing package-pull failures want the linked repository.
- **How discovered**: Heuristic — secret `Name` or `Tags` contain the string `codeartifact` (no direct AWS cross-reference exists).
- **Count shown**: yes.

### `dbi`

- **Why related**: An RDS instance's master credentials can be managed in Secrets Manager — when a DB login fails, operators jump from the secret to the instance to check rotation status.
- **How discovered**: Reverse-scan the already-loaded RDS instance list; match `DBInstance.MasterUserSecret.SecretArn == this.ARN`.
- **Count shown**: yes.

### `eb`

- **Why related**: Elastic Beanstalk environments inject secrets via configuration placeholders — operators want to find every environment whose runtime config references the secret.
- **How discovered**: Reverse-scan Beanstalk environments; call `elasticbeanstalk:DescribeConfigurationSettings` and match `OptionSettings[].Value` containing `{{resolve:secretsmanager:<ARN>`.
- **Count shown**: yes.

### `ecs-task`

- **Why related**: ECS task definitions mount secrets as container environment variables or registry credentials — operators chasing a rotation impact need every task definition that pulls from this secret.
- **How discovered**: Reverse-scan task definitions; match `TaskDefinition.ContainerDefinitions[].Secrets[].ValueFrom == this.ARN` or `RepositoryCredentials.CredentialsParameter == this.ARN`.
- **Count shown**: yes.

### `kms`

- **Why related**: A secret is encrypted with a customer-managed KMS key (or the AWS-managed `aws/secretsmanager` key when `KmsKeyId` is absent). Operators investigating a key disable/deletion need the blast radius.
- **How discovered**: Read `SecretListEntry.KmsKeyId` on the secret; match UUID suffix against the KMS key cache.
- **Count shown**: yes.

### `lambda`

- **Why related**: Automatic rotation is performed by a Lambda function the operator may need to inspect (logs, config, failures).
- **How discovered**: Read `SecretListEntry.RotationLambdaARN`; match the function-name suffix against the Lambda cache.
- **Count shown**: yes.

### `logs`

- **Why related**: When rotation is broken, the first thing an operator opens is the rotation Lambda's log group.
- **How discovered**: Resolve `RotationLambdaARN` → `lambda:GetFunction` → `FunctionConfiguration.LoggingConfig.LogGroup` (fall back to the default `/aws/lambda/<name>` when unset).
- **Count shown**: yes.

### `role`

- **Why related**: Two role linkages matter: the IAM principals that can read the secret (resource policy) and the execution role of the rotation Lambda. Both appear during access-audit and rotation-debug workflows.
- **How discovered**: Call `secretsmanager:GetResourcePolicy` → parse `Statement[].Principal.AWS` for role ARNs; additionally resolve `RotationLambdaARN` → `lambda:GetFunction` → `FunctionConfiguration.Role`.
- **Count shown**: yes.

### `sns`

- **Why related**: The rotation Lambda may post failures to an SNS topic via its Dead Letter Queue config — operators diagnosing "rotation silently failing" need that topic.
- **How discovered**: Resolve `RotationLambdaARN` → `lambda:GetFunction` → `FunctionConfiguration.DeadLetterConfig.TargetArn` when it is an SNS ARN.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Audit trail for secret rotation events, `GetSecretValue` access, policy changes. Universal pivot — applies to every registered type; see `related-resources.md` §Policy.
- **How discovered**: CloudTrail `LookupEvents` filtered on `resources[].ARN` matching the secret ARN.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md` § Secrets & Config.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. AWS field names from `SecretListEntry` are verbatim.

- **Signal**: `RotationEnabled==true && now > NextRotationDate` → rotation overdue.
  - **State bucket**: Warning.
  - **How obtained**: `SecretListEntry.RotationEnabled` and `SecretListEntry.NextRotationDate` on the `ListSecrets` response.

- **Signal**: `RotationEnabled==true && (now - LastRotatedDate) > RotationRules.AutomaticallyAfterDays × 2` → rotation failing.
  - **State bucket**: Broken.
  - **How obtained**: `SecretListEntry.RotationEnabled`, `SecretListEntry.LastRotatedDate`, and `SecretListEntry.RotationRules.AutomaticallyAfterDays` on the `ListSecrets` response. Note: rotations scheduled via `RotationRules.ScheduleExpression` (cron/rate) leave `AutomaticallyAfterDays` null and this rule cannot fire on them — a9s-devops: possible=yes but not covered by the golden doc; worth=yes for operators using cron-based schedules, so flagged as a UX gap in §4.1.

- **Signal**: `LastAccessedDate` older than 180 days → dormant.
  - **State bucket**: Warning.
  - **How obtained**: `SecretListEntry.LastAccessedDate` on the `ListSecrets` response. Caveat carried from the golden doc: the field is day-truncated and excludes access in the current call, so the "180d" threshold is approximate.

- **Signal**: `DeletedDate` set → scheduled for deletion.
  - **State bucket**: Warning.
  - **How obtained**: `SecretListEntry.DeletedDate` on the `ListSecrets` response; presence of a non-null value means the secret is inside its recovery window and will be permanently deleted at the end of it.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: `VersionIdsToStages` stuck on `AWSPENDING` → rotation started but never finished.
  - **State bucket**: Broken.
  - **API call**: `DescribeSecret` — one call per secret.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

The attention-signals.md Wave 3 cell for `secrets` is empty. There are no Wave 3 signals defined for this resource.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". Never appears on yellow/red/dim rows. |
| S4 | Status / description column text | Short human-readable cause. Healthy rows render blank — no `OK`, no `available`. |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping applied:

- Healthy secret (none of the §3.1 conditions, no §3.2 finding) — no §4 row. S2 renders green, S4 renders blank.
- Wave 1 Warning / Broken signals — S2 (color) + S4 (cause text). No S1, S3, S5.
- Wave 2 Broken-style finding (`AWSPENDING` stuck) on an otherwise Healthy row — `!` glyph → S1, S3, S4, S5. If the row is already red (e.g. rotation failing per §3.1) the finding deduplicates with the existing cause; S3 is suppressed on non-green rows, but S1 still counts and S5 still carries the full sentence.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `now > NextRotationDate` | 1 | Warning | n/a | S2, S4 | `rotation overdue: due Apr 10` | `Rotation overdue — next rotation was due 2026-04-10.` |
| `(now - LastRotatedDate) > AutomaticallyAfterDays × 2` | 1 | Broken | n/a | S2, S4 | `rotation failing: last ok 92d ago` | `Rotation schedule is 30d but last successful rotation was 92d ago.` |
| `LastAccessedDate > 180d` | 1 | Warning | n/a | S2, S4 | `dormant: not read in 210d` | `Secret has not been read in this region for 210 days — check if still in use.` |
| `DeletedDate set` | 1 | Warning | n/a | S2, S4 | `deletion in 6d` | `Scheduled for deletion on 2026-04-26 — restore with RestoreSecret before window ends.` |
| `AWSPENDING stuck` | 2 | Broken | `!` | S1, S3, S4, S5 | `rotation stuck: AWSPENDING` | `Rotation started but never completed — an AWSPENDING version has been lingering.` |

Formatting notes applied:

- No banned words appear in S4 / S5.
- No bare state keyword stands alone — every List text pairs a condition with a cause.
- Healthy rows are intentionally absent from the table.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Mostly yes — all five problem conditions name the specific fault and an actionable number (age, due date, countdown). Gap to flag for implementation: secrets whose rotation is scheduled via `RotationRules.ScheduleExpression` (cron/rate) never populate `AutomaticallyAfterDays`, so the "rotation failing" rule silently misses them — the fix is to fall back to `now > NextRotationDate` with a `LastRotatedDate` sanity check for that subset, and keep the S4 text identical.

## 5. Out of Scope

- All §3.3 Wave 3 signals (none defined for `secrets`).
- Any UI element not listed in §4 — no new columns, no banners, no row middle-dot markers, no "Background Check" header. The S1–S5 surfaces are exhaustive.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?"). In particular: no `RotateSecret`, `PutSecretValue`, `RestoreSecret`, `DeleteSecret`, `UpdateSecret`, `TagResource`, `UntagResource`.
- Reading the secret value itself. `GetSecretValue` returns plaintext credentials and is never called by a9s; only metadata (`ListSecrets`, `DescribeSecret`, `GetResourcePolicy`) is used.

## 6. Citations

- Display name and list API `ListSecrets` — `docs/attention-signals.md` § Secrets & Config table row `secrets`.
- AWS API reference URL — `docs/related-resources.md` § Per-type contract row `secrets` and § `secrets` subsection.
- Wave 1 signals (overdue, failing, dormant, deleted) — `docs/attention-signals.md` § Secrets & Config table, `Wave 1` cell for `secrets`.
- Wave 2 `AWSPENDING` stuck via `DescribeSecret` — `docs/attention-signals.md` § Secrets & Config table, `Wave 2` cell for `secrets`.
- Wave 3 empty — `docs/attention-signals.md` § Secrets & Config table, `Wave 3` cell for `secrets` is blank.
- Field names `RotationEnabled`, `NextRotationDate`, `LastRotatedDate`, `LastAccessedDate`, `DeletedDate`, `KmsKeyId`, `RotationLambdaARN`, `Tags`, `RotationRules.AutomaticallyAfterDays`, `RotationRules.ScheduleExpression` — `AWS SDK Go v2 — service/secretsmanager/types.SecretListEntry` and `AWS SDK Go v2 — service/secretsmanager/types.RotationRulesType`.
- Field name `VersionIdsToStages` on `DescribeSecret` (distinct from `SecretVersionsToStages` on `SecretListEntry`) — `AWS SDK Go v2 — service/secretsmanager.DescribeSecretOutput § VersionIdsToStages`.
- Related target `cb` discovery (reverse-scan of `Project.Environment.EnvironmentVariables` with `Type=SECRETS_MANAGER`) — `docs/related-resources.md` § `secrets`, `cb` bullet.
- Related target `cfn` discovery (`Tags["aws:cloudformation:stack-name"]`) — `docs/related-resources.md` § `secrets`, `cfn` bullet.
- Related target `codeartifact` heuristic (name/tag contains `codeartifact`) — `docs/related-resources.md` § `secrets`, `codeartifact` bullet.
- Related target `dbi` reverse-scan (`DBInstance.MasterUserSecret.SecretArn`) — `docs/related-resources.md` § `secrets`, `dbi` bullet.
- Related target `eb` reverse-scan (`{{resolve:secretsmanager:<ARN>` in OptionSettings) — `docs/related-resources.md` § `secrets`, `eb` bullet.
- Related target `ecs-task` reverse-scan (`Secrets[].ValueFrom`, `RepositoryCredentials.CredentialsParameter`) — `docs/related-resources.md` § `secrets`, `ecs-task` bullet.
- Related target `kms` discovery (`KmsKeyId`) — `docs/related-resources.md` § `secrets`, `kms` bullet.
- Related target `lambda` discovery (`RotationLambdaARN`) — `docs/related-resources.md` § `secrets`, `lambda` bullet.
- Related target `logs` discovery (rotation Lambda's `LoggingConfig.LogGroup`) — `docs/related-resources.md` § `secrets`, `logs` bullet.
- Related target `role` discovery (`GetResourcePolicy` principals + rotation Lambda execution role) — `docs/related-resources.md` § `secrets`, `role` bullet.
- Related target `sns` discovery (rotation Lambda DLQ) — `docs/related-resources.md` § `secrets`, `sns` bullet.
- `ct-events` as universal pivot — `docs/related-resources.md` §Policy bullet 4 ("Universal pivots").
- Read-only invariant (no write APIs) — `docs/architecture.md` §"What is a9s?".
- ScheduleExpression gap (Wave 1 "rotation failing" rule misses cron/rate schedules) — a9s-devops (2026-04-20): possible=yes, worth=yes. Rationale: `RotationRules.ScheduleExpression` is set instead of `AutomaticallyAfterDays` for cron/rate-based rotations (confirmed in `AWS SDK Go v2 — service/secretsmanager/types.RotationRulesType § ScheduleExpression`). Daily operators using cron schedules would see silent false-negatives; the fix is a `NextRotationDate`-based fallback, and it belongs as a §4.1 UX gap rather than a change to the golden doc.
- `DeletedDate set` classified as Warning rather than Dim — `docs/attention-signals.md` § Secrets & Config, `Wave 1` cell for `secrets`. The golden doc explicitly writes `Warning`; this spec honors the classification.
