---
shortName: ecr
name: ECR Repositories
awsApiRef: https://docs.aws.amazon.com/AmazonECR/latest/APIReference/API_Repository.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# ecr — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `ecr`
- **Display name**: ECR Repositories
- **AWS API reference**: <https://docs.aws.amazon.com/AmazonECR/latest/APIReference/API_Repository.html>
- **List API**: `DescribeRepositories` (returns `Repository` objects with `RepositoryName`, `RepositoryArn`, `RepositoryUri`, `RegistryId`, `CreatedAt`, `ImageTagMutability`, `ImageScanningConfiguration`, `EncryptionConfiguration` — no runtime state field; a repo is always "available" once created).
- **Describe API (if any)**: `DescribeImages` (per repository, used in Wave 2 to fetch the latest image and read its `imageScanFindingsSummary`).

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `cb`, `cfn`, `eb-rule`, `ecs-task`, `kms`, `lambda`, `pipeline`, `role`, `ct-events`.

### `cb`

- **Why related**: CodeBuild projects are the primary mechanism that pushes images into this repo — when an image is stale or a scan fails, operator hops to the build that produced it.
- **How discovered**: Reverse-scan the already-loaded `cb` list — for each project, match `Environment.Image` against `<RegistryId>.dkr.ecr.<region>.amazonaws.com/<RepositoryName>` and any `Environment.EnvironmentVariables[].Value` that references the repo URI or name. — a9s-devops: project environment image and env vars are the cheapest deterministic link; `buildspec` scanning would require artifact reads and is out of scope.
- **Count shown**: yes.

### `cfn`

- **Why related**: The CloudFormation stack that provisioned the repo is the fastest route to the template, parameters, and stack events — critical for governance and change history.
- **How discovered**: Call `ecr:ListTagsForResource` for this repo and read the `aws:cloudformation:stack-name` tag (set automatically on CFN-managed resources); look the stack up in the already-loaded `cfn` list. — a9s-devops: the tag is the canonical CFN-managed marker; no tag → not CFN-managed (skip).
- **Count shown**: yes.

### `eb-rule`

- **Why related**: EventBridge rules fire on ECR image-scan and image-action events — operator chasing a scan-to-alert pipeline starts here.
- **How discovered**: Reverse-scan the already-loaded `eb-rule` list — rules with `EventPattern.source=["aws.ecr"]` AND `EventPattern.detail["repository-name"]` matching this repo's `RepositoryName`. — a9s-devops: `aws.ecr` events include `ECR Image Scan` and `ECR Image Action`; the event pattern's repository-name filter is the definitive per-rule link.
- **Count shown**: yes.

### `ecs-task`

- **Why related**: ECS task definitions pull container images from this repo — the central "who runs this image?" pivot during deploys and incidents.
- **How discovered**: Reverse-scan the already-loaded `ecs-task` list — for each task definition, inspect `ContainerDefinitions[].Image` and match the registry/repo portion against `<RegistryId>.dkr.ecr.<region>.amazonaws.com/<RepositoryName>[:tag|@digest]`. — a9s-devops: `ContainerDefinitions[].Image` is the canonical field; parsing the URI is trivial and covers both tag- and digest-pinned images.
- **Count shown**: yes.

### `kms`

- **Why related**: Repository may be encrypted at rest with a customer-managed KMS key — operator wants to confirm the key is enabled before declaring the repo usable.
- **How discovered**: Read `Repository.EncryptionConfiguration.KmsKey` (set when `EncryptionType==KMS` or `KMS_DSSE`) and look the key ARN up in the already-loaded `kms` list. AWS-managed `aws/ecr` default keys surface as a standard alias — show as "managed default". — AWS SDK Go v2 cite: `ecr/types.EncryptionConfiguration § KmsKey`.
- **Count shown**: yes.

### `lambda`

- **Why related**: Container-image Lambda functions pull from this repo — debugging a cold-start or deployment failure starts at the image source.
- **How discovered**: Reverse-scan the already-loaded `lambda` list — for each function, when `PackageType==Image`, match `Code.ImageUri` against `<RegistryId>.dkr.ecr.<region>.amazonaws.com/<RepositoryName>[:tag|@digest]`. — a9s-devops: `Code.ImageUri` is the canonical pointer for container-image Lambdas; skip functions where `PackageType==Zip`.
- **Count shown**: yes.

### `pipeline`

- **Why related**: CodePipelines push images to this repo (ECR as destination) or pull from it (ECR as source) — the deployment-path pivot.
- **How discovered**: Reverse-scan the already-loaded `pipeline` list — for each pipeline, walk `Stages[].Actions[]` and match any action where `ActionTypeId.Provider==ECR` and `Configuration.RepositoryName==<this RepositoryName>`. — a9s-devops: ECR source-provider action is the only first-class pipeline→ECR linkage; push-to-ECR via CodeBuild actions is already covered by the `cb` pivot.
- **Count shown**: yes.

### `role`

- **Why related**: IAM roles with pull/push permissions on this repo — operator auditing access or triaging a "denied" error wants the principals trusted by this repo's policy.
- **How discovered**: Call `ecr:GetRepositoryPolicy` for this repo (a single per-repo call) and parse the policy document — extract `Statement[].Principal.AWS` ARNs and match them against the already-loaded `role` list. If the repo has no explicit policy (default behavior), IAM-level grants govern access and this pivot is empty. — a9s-devops: repository policy is the authoritative per-repo principal source; walking the full role cache with `GetRolePolicy` per role is Wave 3.
- **Count shown**: yes.

### `ct-events`

- **Why related**: CloudTrail audit trail for image push/pull, repository policy changes, and lifecycle-policy edits.
- **How discovered**: Universal pivot — applies to every registered type; see `related-resources.md` §Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `imageScanningConfiguration.scanOnPush==false` → no vulnerability scanning configured on new image pushes.
  - **State bucket**: Warning.
  - **How obtained**: `DescribeRepositories` response — each `Repository` carries its `ImageScanningConfiguration.ScanOnPush` (bool). No extra call.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: latest image by `imagePushedAt` has `imageScanFindingsSummary.findingSeverityCounts.CRITICAL>0` → CRITICAL vulnerabilities present in the most recently pushed image.
  - **State bucket**: Broken.
  - **API call**: `DescribeImages` — one call per repository (N+1). Client-side sort by `imagePushedAt` descending to pick the latest image; the API itself does not order by time.
  - **Cost shape**: per-resource.
- **Signal**: latest image `imageScanFindingsSummary.findingSeverityCounts.HIGH>0` (and `CRITICAL==0`) → HIGH-severity vulnerabilities present.
  - **State bucket**: Warning.
  - **API call**: same `DescribeImages` response as above — no extra call.
  - **Cost shape**: per-resource.
  - Note: `imageScanFindingsSummary` is present only when a scan has run; if absent, no finding is surfaced for this signal.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `DescribeImageScanFindings` per image (full finding detail beyond the summary counts).
- OUT OF SCOPE: `GetLifecyclePolicy` per repo (lifecycle policy inspection for cost/retention audit).

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". Never appears on yellow/red/dim rows. |
| S4 | Status / description column text | Short human-readable cause. Healthy rows render blank. |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. |

Note on baseline state: the ECR Repository object carries no runtime status field — a repo is always "available" once created. The list row starts green by default and is repainted by the signals below.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `scanOnPush==false` | 1 | Warning | n/a | S2, S4 | `scan-on-push off` | `Vulnerability scanning is disabled for this repo — new images will push without a CVE scan.` |
| latest image `CRITICAL>0` | 2 | Broken | `!` | S1, S2, S4, S5 | `CRITICAL CVEs in latest` | `Latest image (pushed <date>) has N CRITICAL vulnerabilities — block deploys until patched.` |
| latest image `HIGH>0` (no CRITICAL) | 2 | Warning | `~` | S2, S4, S5 | `HIGH CVEs in latest` | `Latest image (pushed <date>) has N HIGH vulnerabilities — review before next deploy.` |

Rules applied:

- `scanOnPush==false` is a Wave 1 Warning and paints the row yellow; S4 carries the cause. No glyph (S3 is forbidden on yellow rows). No S5 (Wave 1 does not produce a finding object).
- `CRITICAL>0` is a Wave 2 Broken finding; the row is repainted red and the full cause appears in S4/S5. S1 counts this `!` finding. S3 is suppressed because the row is no longer green.
- `HIGH>0` is a Wave 2 Warning finding; the row is repainted yellow (if not already) and the cause appears in S4/S5. S3 is suppressed (non-green row); S1 does not count `~` findings.
- When multiple signals fire on the same repo, the highest-severity bucket wins the row color (Broken > Warning) and S4 shows the Broken cause; secondary causes go to S5.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every §4 row pairs the state with a concrete cause in the Status column (`scan-on-push off`, `CRITICAL CVEs in latest`, `HIGH CVEs in latest`), so the operator can triage which repo to block without opening detail. The detail view adds the push date and finding count so the follow-up "how bad, how old?" question is one keypress away.

## 5. Out of Scope

- All §3.3 Wave 3 signals (full per-finding detail, lifecycle policies).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- `ecs` as a related target — `ecr` → `ecs` (cluster) has no first-class AWS API linkage; use `ecr` → `ecs-task` for the deterministic image-to-workload pivot. See `docs/related-resources.md` §Non-matches (`ecr → ecs`, `ecr → eks`).
- `eks` as a related target — image resolution lives in Kubernetes, not the EKS API; `ecr` → `ecs-task` covers ECS workloads, and EKS image usage is Wave 3.
- Any write operation. a9s is read-only by design (`docs/architecture.md` §"What is a9s?").

## 6. Citations

One bullet per claim in §§2–4.1.

- shortName + display name — `docs/attention-signals.md` §CI/CD, `ecr` row.
- AWS API URL — `docs/related-resources.md` §Per-type contract, `ecr` row.
- List API `DescribeRepositories` and Repository shape — `AWS SDK Go v2 — ecr/types.Repository § RepositoryName, RepositoryArn, RepositoryUri, EncryptionConfiguration, ImageScanningConfiguration`.
- Wave 2 API `DescribeImages` — `docs/attention-signals.md` §CI/CD, `ecr` row Wave 2 cell.
- `cb` related target — `docs/related-resources.md` §`ecr` — "CodeBuild projects that push images."
- `cb` discovery via `Environment.Image` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Project environment image URI is the cheapest deterministic link;`buildspec`scanning needs artifact reads and is out of scope.`
- `cfn` related target — `docs/related-resources.md` §`ecr` — "CloudFormation stack that created the repo."
- `cfn` discovery via tag — `a9s-devops (2026-04-20): possible=yes, worth=yes. The`aws:cloudformation:stack-name` tag is the canonical CFN-managed marker; requires `ecr:ListTagsForResource`per repo.`
- `ct-events` universal pivot — `docs/related-resources.md` §Policy #4.
- `eb-rule` related target — `docs/related-resources.md` §`ecr` — "Image-scan EventBridge events."
- `eb-rule` discovery via `aws.ecr` event pattern — `a9s-devops (2026-04-20): possible=yes, worth=yes. Event source`aws.ecr`(Image Scan / Image Action) with repository-name filter is the definitive reverse-scan link.`
- `ecs-task` related target — `docs/related-resources.md` §`ecr` — "Task defs pull from repo."
- `ecs-task` discovery via `ContainerDefinitions[].Image` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Image URI on container definition is the central "who runs this image?" pivot.`
- `kms` related target and field — `docs/related-resources.md` §`ecr` — "EncryptionConfiguration.KmsKey."
- `kms` discovery — `AWS SDK Go v2 — ecr/types.EncryptionConfiguration § KmsKey, EncryptionType`.
- `lambda` related target — `docs/related-resources.md` §`ecr` — "Lambda functions using container image from this repo."
- `lambda` discovery via `Code.ImageUri` with `PackageType==Image` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Container-image Lambda pointer is on FunctionConfiguration; skip zip-packaged functions.`
- `pipeline` related target — `docs/related-resources.md` §`ecr` — "Pipelines pushing to this repo."
- `pipeline` discovery via `ActionTypeId.Provider==ECR` — `a9s-devops (2026-04-20): possible=yes, worth=yes. ECR source-provider action is the only first-class pipeline→ECR linkage; push-via-CodeBuild is covered by the`cb`pivot.`
- `role` related target — `docs/related-resources.md` §`ecr` — "Pull/push IAM roles."
- `role` discovery via `GetRepositoryPolicy` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Repository policy is the authoritative per-repo principal source; walking the role cache with per-role calls is Wave 3.`
- Wave 1 signal `scanOnPush==false` — `docs/attention-signals.md` §CI/CD, `ecr` row Wave 1 cell.
- `ScanOnPush` field location — `AWS SDK Go v2 — ecr/types.ImageScanningConfiguration § ScanOnPush`.
- Wave 2 signals `CRITICAL>0` / `HIGH>0` — `docs/attention-signals.md` §CI/CD, `ecr` row Wave 2 cell.
- `imageScanFindingsSummary.findingSeverityCounts` shape — `AWS SDK Go v2 — ecr/types.ImageScanFindingsSummary § FindingSeverityCounts`; `AWS SDK Go v2 — ecr/types.ImageDetail § ImageScanFindingsSummary, ImagePushedAt`.
- Severity mapping (`!` for Broken, `~` for Warning on Healthy-baseline row) — `.claude/skills/a9s-resource-spec/SKILL.md` §"Mapping rules for §4".
- Wave 3 exclusions (per-image findings, lifecycle policy) — `docs/attention-signals.md` §CI/CD, `ecr` row Wave 3 cell.
- Non-matches (`ecr → ecs`, `ecr → eks`) — `docs/related-resources.md` §Non-matches (lines 1092–1093).
- Read-only invariant — `docs/architecture.md` §"What is a9s?".
- Removed stale `ecs` bullet from detailed `ecr` section — `a9s-resource-spec amendment (2026-04-20): contradicted per-type contract and Non-matches section; reason in HTML comment inline.`
