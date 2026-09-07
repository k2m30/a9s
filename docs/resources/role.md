---
shortName: role
name: IAM Roles
awsApiRef: https://docs.aws.amazon.com/IAM/latest/APIReference/API_Role.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
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

**Source API**: [GetRole](https://docs.aws.amazon.com/IAM/latest/APIReference/API_GetRole.html)

Transcribed from `docs/attention-signals.md § Signals § SECURITY & IAM` row `role`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `AssumeRolePolicyDocument` (URL-encoded JSON on `ListRoles`) contains `Principal:{"AWS":"*"}` without an external-id condition.
  - **State bucket**: Broken.
  - **How obtained**: URL-decode and JSON-parse `Role.AssumeRolePolicyDocument` from the `ListRoles` response; search for a `Statement` whose `Effect==Allow` and `Principal.AWS=="*"` with no matching `Condition.StringEquals["sts:ExternalId"]`.

- **Signal**: an AWS service is trusted with no `aws:SourceAccount` / `aws:SourceArn` scoping.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: an inline policy grants a known privilege-escalation action combination.
  - **State bucket**: Broken.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: `RoleLastUsed.LastUsedDate` missing or >90d (dormant; field is region-scoped — may false-warn in multi-region accounts).
  - **State bucket**: Warning.
  - **API call**: `GetRole` per role (one per resource).
  - **Cost shape**: per-resource.

- **Signal**: `AdministratorAccess` or `PowerUserAccess` attached.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `ListAttachedRolePolicies` per role (admin-access detection).
- OUT OF SCOPE: `GenerateServiceLastAccessedDetails` async permission-usage audit.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `role`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| trust policy allows a wildcard principal with no restrictive condition | 1 | Broken | n/a | S1, S2, S4, S5 | `anyone can assume this role` |
| an AWS service is trusted with no `aws:SourceAccount` / `aws:SourceArn` scoping | 1 | Warning | n/a | S1, S2, S4, S5 | `service can assume without source scoping` |
| an inline policy grants a known privilege-escalation action combination | 1 | Broken | n/a | S1, S2, S4, S5 | `inline policy allows privilege escalation` |
| dormant — `RoleLastUsed.LastUsedDate` missing or >90d | 2 | Warning | `~` | S3, S4, S5 | `dormant role (>90d)` |
| `AdministratorAccess` or `PowerUserAccess` attached | 2 | Warning | `~` | S3, S4, S5 | `has an administrator policy` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column.
- Keep the List text short enough to fit: ≤ 40 chars. The Detail cell quotes the finding's Detail constant verbatim, however long it is.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — a red row reading `anyone can assume this role` is actionable on sight (name the accounts, or add an external-id condition), and a green row prefixed `~` with `unused >90d` tells the operator this role is a candidate for deletion without needing to open detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- Severity choice `~` for the `unused >90d` signal — a9s-devops: not worth it to bump to `!` because a dormant role is informational; the decision to delete requires human review of trust and attached-policy blast radius, so chasing it from the menu `issues:N` count would be noise. Informational glyph is correct.

## 6. Citations

- Display name and the `role` signals — `docs/attention-signals.md § Signals § SECURITY & IAM` row `role`; the deferred permission-usage audit — `docs/attention-signals.md § Not yet implemented`.
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

<!-- BEGIN GENERATED: header -->
role — SECURITY & IAM. Lifecycle key: none (the list API returns no lifecycle field).
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| role.trust.wildcard-principal | anyone can assume this role | broken | wave1 | Any AWS account can call sts:AssumeRole on this role and obtain its permissions. Replace the "*" principal in the trust policy with the specific account or role ARNs, or add an sts:ExternalId condition. |
| role.trust.confused-deputy | service can assume without source scoping | warn | wave1 | An AWS service principal can assume this role on behalf of any caller, so another customer's resource can trick the service into using your role. Add an aws:SourceAccount or aws:SourceArn condition to the trust statement. |
| role.inline-privilege-escalation | inline policy allows privilege escalation | broken | wave1 | An inline policy on this role grants a combination of actions that lets its holder grant itself full administrator. Split or scope the inline policy so the escalation actions are not all available together. |
| iam-role.dormant | dormant role (>90d) | warn | wave2 | Nothing has assumed this role in over 90 days, so its trust policy and permissions are live but unexercised. Confirm the workload that used it is gone, then delete the role. |
| role.admin-attached | has an administrator policy | warn | wave2 | This principal is attached to an AWS-managed policy that grants administrator-equivalent access, so anything it can be used for it can be used for everything. Replace the managed policy with a scoped policy covering only the actions this principal needs. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| lambda | Lambda Functions | yes |
| glue | Glue Jobs | yes |
| ng | Node Groups | yes |
| policy | IAM Policies | no |
| ec2 | EC2 Instances | yes |
| eks | EKS Clusters | yes |
| iam-group | IAM Groups (trust) | no |
| iam-user | IAM Users (trust) | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
