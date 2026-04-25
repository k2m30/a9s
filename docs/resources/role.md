---
shortName: role
name: IAM Roles
awsApiRef: https://docs.aws.amazon.com/IAM/latest/APIReference/API_Role.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# role — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `role`
- **Display name**: IAM Roles
- **AWS API reference**: <https://docs.aws.amazon.com/IAM/latest/APIReference/API_Role.html>
- **List API**: `ListRoles` (each list entry is a `Role` struct, including the URL-encoded `AssumeRolePolicyDocument`).
- **Describe API (if any)**: `GetRole` — used in Wave 2 only, to fetch `RoleLastUsed.LastUsedDate` (the list response's `Role` does not carry `RoleLastUsed` reliably; `GetRole` is the canonical source).

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `ct-events`, `ec2`, `eks`, `glue`, `iam-group`, `iam-user`, `lambda`, `ng`, `policy`.

### `ec2`

- **Why related**: EC2 instances assuming this role via instance profile — operator is asking "who runs as this role?".
- **How discovered**: cross-reference the already-loaded `ec2` list by `Instance.IamInstanceProfile.Arn` matched against the instance profiles this role is a member of (the role-name segment of the profile ARN equals the role's name in the common one-profile-per-role convention) — a9s-devops: the list-level `ec2` response carries `IamInstanceProfile`, so no extra API call is needed when the `ec2` list is already loaded; a strict resolution (`ListInstanceProfilesForRole`) is Wave-2-grade and can be added if the filename-match heuristic is rejected.
- **Count shown**: yes.

### `eks`

- **Why related**: EKS service role — the cluster assumes this role to manage the control plane.
- **How discovered**: cross-reference the already-loaded `eks` list by `Cluster.RoleArn == this role's ARN` — a9s-devops: `DescribeCluster` returns `RoleArn` as a first-class field, so matching is exact on the already-loaded cluster list.
- **Count shown**: yes.

### `glue`

- **Why related**: Glue jobs assuming this role — operator triaging job failures often suspects role/permissions.
- **How discovered**: cross-reference the already-loaded `glue` list by `Job.Role == this role's name or ARN` — a9s-devops: `GetJobs` returns `Role` on the list response, no extra call.
- **Count shown**: yes.

### `iam-group`

- **Why related**: Trust relationships may reference groups (the trust policy's `Principal` list can name groups that can assume this role).
- **How discovered**: parse the role's URL-decoded `AssumeRolePolicyDocument` (available on `ListRoles`) and match `Statement[].Principal.AWS` ARNs ending in `:group/<name>` against the already-loaded `iam-group` list — a9s-devops: standard trust-policy parse; no extra API call.
- **Count shown**: yes.

### `iam-user`

- **Why related**: Trust may include user principals — named humans who can assume this role.
- **How discovered**: same `AssumeRolePolicyDocument` parse as `iam-group`, matching `Principal.AWS` ARNs ending in `:user/<name>` against the already-loaded `iam-user` list — a9s-devops: same parse, different principal suffix.
- **Count shown**: yes.

### `lambda`

- **Why related**: Lambdas executing as this role — the role is the Lambda's execution identity.
- **How discovered**: cross-reference the already-loaded `lambda` list by `FunctionConfiguration.Role == this role's ARN` — a9s-devops: `ListFunctions` returns `Role` on every function, no extra call.
- **Count shown**: yes.

### `ng`

- **Why related**: EKS node groups assuming this role — the node IAM role nodes assume at launch.
- **How discovered**: cross-reference the already-loaded `ng` list by `Nodegroup.NodeRole == this role's ARN` — a9s-devops: `DescribeNodegroup` returns `NodeRole` as a first-class field.
- **Count shown**: yes.

### `policy`

- **Why related**: Attached managed policies — what this role is permitted to do.
- **How discovered**: call `ListAttachedRolePolicies` per role — a9s-devops: this is the only authoritative route; reverse-scanning the already-loaded `policy` list does not work because a policy's `AttachmentCount` does not break down by principal. This is a Wave-2-class call (one API per opened role detail), not background enrichment.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Audit trail for role AssumeRole / policy attach events — who's been using this role, who changed its permissions. Universal pivot — applies to every registered type; see related-resources.md §Policy.
- **How discovered**: `LookupEvents` filtered by `userIdentity.sessionContext.sessionIssuer.arn == this role's ARN` (for AssumeRole usage) and by `resources.ARN == this role's ARN` (for policy-attach / trust-policy edits) — a9s-devops: both filters are documented CloudTrail pivots.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `AssumeRolePolicyDocument` (URL-encoded JSON on `ListRoles`) contains `Principal:{"AWS":"*"}` without an external-id condition.
  - **State bucket**: Broken.
  - **How obtained**: URL-decode and JSON-parse `Role.AssumeRolePolicyDocument` from the `ListRoles` response; search for a `Statement` whose `Effect==Allow` and `Principal.AWS=="*"` with no matching `Condition.StringEquals["sts:ExternalId"]`.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: `RoleLastUsed.LastUsedDate` missing or >90d (dormant; field is region-scoped — may false-warn in multi-region accounts).
  - **State bucket**: Warning.
  - **API call**: `GetRole` per role (one per resource).
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `ListAttachedRolePolicies` per role (admin-access detection).
- OUT OF SCOPE: `GenerateServiceLastAccessedDetails` async permission-usage audit.

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
| trust policy allows `Principal:AWS=*` without external-id | 1 | Broken | n/a (color is the signal) | S2, S4 | `trust allows *, no external-id` | `Trust policy allows any AWS principal to assume this role with no external-id guard — anyone can AssumeRole.` |
| dormant — `RoleLastUsed.LastUsedDate` missing or >90d | 2 | Healthy (finding on green row) | `~` | S3, S4, S5 | `unused >90d` | `No AssumeRole activity in the last 90 days (region-scoped — may miss usage in other regions).` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column.
- List text ≤ 40 chars, Detail text ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — a red row reading `trust allows *, no external-id` is actionable on sight (revoke or add external-id), and a green row prefixed `~` with `unused >90d` tells the operator this role is a candidate for deletion without needing to open detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- Severity choice `~` for the `unused >90d` signal — a9s-devops: not worth it to bump to `!` because a dormant role is informational; the decision to delete requires human review of trust and attached-policy blast radius, so chasing it from the menu `issues:N` count would be noise. Informational glyph is correct.

## 6. Citations

- Display name, Source API, Wave 1/Wave 2/Wave 3 cells — `docs/attention-signals.md` § `role` row (Security & IAM table).
- AWS API URL and expected related targets — `docs/related-resources.md` § Per-type contract → `role` row.
- Per-target reasoning (ct-events / ec2 / eks / glue / iam-group / iam-user / lambda / ng / policy) — `docs/related-resources.md` § `role` section.
- `AssumeRolePolicyDocument` is URL-encoded JSON on the list response — `AWS SDK Go v2 — iam/types.Role § AssumeRolePolicyDocument` (string field on the `Role` struct returned by `ListRoles`).
- `RoleLastUsed.LastUsedDate` is the dormancy field — `AWS SDK Go v2 — iam/types.RoleLastUsed § LastUsedDate`. SDK doc note: "Activity is only reported for the trailing 400 days" and is region-scoped.
- `RoleLastUsed` exists on the `Role` type itself — `AWS SDK Go v2 — iam/types.Role § RoleLastUsed` (populated canonically by `GetRole` / `GetAccountAuthorizationDetails`).
- Discovery mechanism for `ec2` related target (instance-profile ARN match on already-loaded list) — a9s-devops (2026-04-20): possible=yes, worth=yes. `IamInstanceProfile.Arn` on `ec2` list response is canonical; filename-match heuristic avoids an extra `ListInstanceProfilesForRole` call per role.
- Discovery mechanism for `eks`, `glue`, `lambda`, `ng` (direct role-ARN field on sibling list) — a9s-devops (2026-04-20): possible=yes, worth=yes. Every one of these sibling list responses carries the role reference as a first-class field; no extra API call needed when the sibling list is already loaded.
- Discovery mechanism for `iam-group` / `iam-user` (trust-policy `Principal.AWS` parse) — a9s-devops (2026-04-20): possible=yes, worth=yes. `AssumeRolePolicyDocument` is already on the list response; parsing it in-process is free and is the only way to surface named-user / named-group trust.
- Discovery mechanism for `policy` (`ListAttachedRolePolicies`) — a9s-devops (2026-04-20): possible=yes, worth=yes. Policy `AttachmentCount` does not decompose by principal, so the only authoritative route is the per-role call; worth the cost because attached policies are the #1 reason an operator opens a role's detail.
- Discovery mechanism for `ct-events` (sessionIssuer.arn + resources.ARN filters) — a9s-devops (2026-04-20): possible=yes, worth=yes. CloudTrail's `sessionContext.sessionIssuer.arn` surfaces AssumeRole usage; `resources.ARN` surfaces policy-attach / trust edits. Both are documented CloudTrail pivots.
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
- S1–S5 surface definitions and Wave→surface mapping — `.claude/skills/a9s-resource-spec/SKILL.md` § "Allowed visualization surfaces (exactly five)".
- `~` severity choice for dormant-role finding — a9s-devops (2026-04-20): possible=yes, worth=no (for bumping S1). Dormancy is informational; deletion requires human review of trust and attached-policy blast radius, so chasing it via the menu issues count would be noise. `~` (no S1 bump) is the correct surface.
