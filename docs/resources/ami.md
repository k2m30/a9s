---
shortName: ami
name: AMIs
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Image.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# ami — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `ami`
- **Display name**: AMIs
- **AWS API reference**: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Image.html
- **List API**: `DescribeImages`
- **Describe API (if any)**: not used — all attention signals are served from the `DescribeImages` response plus cross-reference against already-loaded sibling lists (Wave 1 only; Wave 2 is `None`).

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `asg`, `cfn`, `ct-events`, `ebs-snap`, `ec2`, `kms`, `ng`.

### `asg`

- **Why related**: when a custom AMI is being rolled forward or deprecated, the operator needs to know which Auto Scaling Groups still launch from it so new scale-out events don't keep picking up the old image.
- **How discovered**: iterate the already-loaded `asg` list and match on its `LaunchConfiguration.ImageId` / `LaunchTemplate.LaunchTemplateData.ImageId` (resolved via `DescribeLaunchTemplateVersions` when only an ID+version pair is on the ASG). — a9s-devops (2026-04-20): launch templates are the standard ASG launch path; resolving `ImageId` from the chosen template version is a normal per-ASG step.
- **Count shown**: yes.

### `cfn`

- **Why related**: AMIs are often parameterized or hardcoded inside CloudFormation templates; governance wants to know which stacks reference an AMI before deregistering it.
- **How discovered**: `TBD — a9s-devops (2026-04-20): possible=yes but expensive, worth=partial. No cheap index maps AMI→stack — it requires scanning every stack template for Parameters/Resources referencing the ImageId. Not viable within Wave 1's zero-extra-call budget.`
- **Count shown**: unknown.

### `ct-events`

- **Why related**: AMI lifecycle events (`RegisterImage`, `DeregisterImage`, `ModifyImageAttribute`, `EnableImageDeprecation`) matter during audit and change-review — "who deregistered this AMI and when?" is a common on-call question.
- **How discovered**: call `LookupEvents` filtered on the AMI's `ImageId` as a `ResourceName` attribute — `ct-events` is the universal pivot; see `docs/related-resources.md` §Policy.
- **Count shown**: yes.

### `ebs-snap`

- **Why related**: every EBS-backed AMI is a thin index over one or more EBS snapshots that hold the actual bytes. Operators chase from AMI → snapshot when verifying image provenance, estimating storage cost, or hunting a broken image.
- **How discovered**: read `BlockDeviceMappings[].Ebs.SnapshotId` on the AMI and cross-reference the already-loaded `ebs-snap` list.
- **Count shown**: yes.

### `ec2`

- **Why related**: the single most-asked AMI question: "is this image still in use?" Operators need the reverse-lookup — instances launched from this AMI — before they deprecate or deregister it.
- **How discovered**: iterate the already-loaded `ec2` list and match `Instance.ImageId == this.ImageId`. — a9s-devops (2026-04-20): zero-cost when EC2 list is resident; matches the "can I retire this AMI?" workflow that every platform team runs.
- **Count shown**: yes.

### `kms`

- **Why related**: encrypted AMIs inherit KMS CMKs from their backing snapshots. Operators need to see the CMK to verify access (the role launching instances must be able to `Decrypt` on the key) and to catch cross-account AMI-sharing breakage when the key is scoped too tightly.
- **How discovered**: read `BlockDeviceMappings[].Ebs.KmsKeyId` on the AMI and cross-reference the already-loaded `kms` list.
- **Count shown**: yes.

### `ng`

- **Why related**: EKS managed node groups using a custom launch template pull their worker AMI from `LaunchTemplateData.ImageId`. Custom-AMI shops need to audit node groups before retiring a base image; EKS-managed AMI types (AL2_x86_64 and friends) are not operator-relevant here.
- **How discovered**: iterate the already-loaded `ng` list; for NGs with a custom launch template, resolve `LaunchTemplateData.ImageId` via `ec2:DescribeLaunchTemplateVersions` and match. — a9s-devops (2026-04-20): same launch-template resolution path used for `asg`.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `State == available` → Healthy.
  - **State bucket**: Healthy.
  - **How obtained**: `Image.State` on the `DescribeImages` list response.
- **Signal**: `State == pending` or `State == transient` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `Image.State` on the `DescribeImages` list response.
- **Signal**: `State == failed` or `State == error` or `State == invalid` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `Image.State` on the `DescribeImages` list response; pair with `StateReason.Message` for the cause string.
- **Signal**: `State == deregistered` or `State == disabled` → Dim.
  - **State bucket**: Dim.
  - **How obtained**: `Image.State` on the `DescribeImages` list response.
- **Signal**: `DeprecationTime < now()` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `Image.DeprecationTime` (ISO-8601 string) on the `DescribeImages` list response, parsed and compared against the current time.
- **Signal**: Cross-ref `ebs-snap` (owner-scoped only — skip public/marketplace AMIs) — backing snapshot missing → Warning.
  - **State bucket**: Warning.
  - **How obtained**: read `Image.BlockDeviceMappings[].Ebs.SnapshotId` and look each ID up in the already-loaded `ebs-snap` list; a miss on an owner-scoped AMI is the signal. Skip the check when `ImageOwnerAlias` is `amazon`/`aws-marketplace` or the AMI is otherwise not in the caller's account.

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `DescribeImageAttribute(launchPermission)` per AMI.

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
| `State == pending` or `transient` | 1 | Warning | n/a | S2, S4 | `pending: registering image` | AMI registration in progress; not yet launchable. |
| `State == failed / error / invalid` | 1 | Broken | n/a | S2, S4 | `failed: <StateReason.Message>` | AMI registration failed: `<StateReason.Message>` — image is unusable. |
| `State == deregistered` | 1 | Dim | n/a | S2, S4 | `deregistered` | AMI has been deregistered; cannot launch new instances from it. |
| `State == disabled` | 1 | Dim | n/a | S2, S4 | `disabled` | AMI is disabled in this account; launches are blocked until re-enabled. |
| `DeprecationTime < now()` | 1 | Warning | n/a | S2, S4 | `deprecated <Nd> ago` | AWS marked this AMI deprecated on `<DeprecationTime>`; replace with current image. |
| Backing snapshot missing (owner-scoped) | 1 | Warning | n/a | S2, S4 | `backing snapshot missing` | One or more EBS snapshots in `BlockDeviceMappings` are absent — AMI cannot be used to launch. |

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? All problem rows carry a cause next to the AMI name — `failed: <reason>`, `deprecated <Nd> ago`, `backing snapshot missing`, or the explicit `deregistered` / `disabled` keyword — so the operator can triage (re-register, re-create, or rotate to a newer AMI) straight from the list without pressing detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- `cfn` as a related target for AMI has no cheap discovery path — a9s-devops (2026-04-20): possible=yes but worth=partial; requires scanning every stack template for `ImageId` references. The row remains in the related panel but shows no count until a cheaper mechanism exists.

## 6. Citations

- `ami` list API is `DescribeImages` — `docs/attention-signals.md` § Compute table row `ami`.
- `DescribeImages` returns `Image.State`, `Image.StateReason`, `Image.DeprecationTime`, `Image.BlockDeviceMappings` — `AWS SDK Go v2 — ec2/types.Image § State, StateReason, DeprecationTime, BlockDeviceMappings`.
- `State` enum values `available`, `pending`, `transient`, `failed`, `error`, `invalid`, `deregistered`, `disabled` — `AWS SDK Go v2 — ec2/types.ImageState`; buckets Healthy/Warning/Broken/Dim per `docs/attention-signals.md` § Compute table row `ami`.
- `StateReason.Message` carries the failure cause for Broken rows — `AWS SDK Go v2 — ec2/types.StateReason § Message`.
- `DeprecationTime < now()` → Warning — `docs/attention-signals.md` § Compute table row `ami`.
- Cross-ref `ebs-snap` via `BlockDeviceMappings[].Ebs.SnapshotId`, skip public/marketplace — `docs/attention-signals.md` § Compute table row `ami`; `AWS SDK Go v2 — ec2/types.EbsBlockDevice § SnapshotId`; `AWS SDK Go v2 — ec2/types.Image § ImageOwnerAlias` identifies owner scope.
- Related targets `asg`, `cfn`, `ct-events`, `ebs-snap`, `ec2`, `kms`, `ng` — `docs/related-resources.md` § Per-type contract row `ami`.
- `ebs-snap` pivot mechanism (read `BlockDeviceMappings[].Ebs.SnapshotId`) — `docs/related-resources.md` § `ami` subsection.
- `kms` pivot mechanism (read `BlockDeviceMappings[].Ebs.KmsKeyId`) — `docs/related-resources.md` § `ami` subsection.
- `ec2` pivot — reverse lookup on `Instance.ImageId` — `docs/related-resources.md` § `ami` subsection; `AWS SDK Go v2 — ec2/types.Instance § ImageId`.
- `asg` pivot — operator workflow rationale and discovery via `LaunchConfiguration.ImageId` / `LaunchTemplateData.ImageId` — a9s-devops (2026-04-20): possible=yes, worth=yes. Custom-AMI shops audit which ASGs still launch from a soon-to-be-deprecated image before deregistering.
- `ng` pivot — operator workflow rationale and discovery via `ec2:DescribeLaunchTemplateVersions` for custom-AMI node groups — a9s-devops (2026-04-20): possible=yes, worth=yes. Same audit workflow as `asg`, scoped to EKS NGs whose launch template ImageId is operator-controlled.
- `cfn` pivot — `docs/related-resources.md` § `ami` subsection says "often consumed by CloudFormation templates"; discovery deferred — a9s-devops (2026-04-20): possible=yes but worth=partial, no cheap index maps AMI→stack, would require scanning every template. Recorded in §5 Out of Scope.
- `ct-events` is the universal pivot — `docs/related-resources.md` §Policy.
- `DeprecationTime` S4 wording (`deprecated <Nd> ago`) and `StateReason.Message` S4 wording (`failed: <reason>`) — a9s-devops (2026-04-20): operator-readable phrasing; mirrors the ACM `expires in Nd` pattern already used elsewhere in a9s, pairs a state keyword with its cause so the S4 rule (no bare state keywords) is satisfied.
- Backing-snapshot-missing S4 wording (`backing snapshot missing`) — a9s-devops (2026-04-20): plain-language restatement of the cross-ref failure; tells the on-call engineer the AMI is unusable without naming the snapshot ID on the list row.
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
