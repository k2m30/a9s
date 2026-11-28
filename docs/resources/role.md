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
- **Describe API (if any)**: `GetRole`. Wave 2 reads `RoleLastUsed.LastUsedDate` from it for every role, service-linked roles included, and writes the "Last Used" column (`last_used`); `ListRoles` does not return `PermissionsBoundary`, `RoleLastUsed` or `Tags` (`ListOmits`). The detail takes those three members from `GetRole`, read once per session.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `ct-events`, `ec2`, `eks`, `glue`, `iam-group`, `iam-user`, `lambda`, `ng`, `policy`.

### `ec2`

- **Why related**: EC2 instances assuming this role via instance profile — operator is asking "who runs as this role?".
- **How discovered**: `iam:ListInstanceProfilesForRole` names the instance profiles that hold this role; the `ec2` list is cross-referenced by `Instance.IamInstanceProfile.Arn` against those profiles' ARNs. A profile's name says nothing about the role it holds.
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
- **How discovered**: parse the role's URL-decoded `AssumeRolePolicyDocument` (available on `ListRoles`) and match `Statement[].Principal.AWS` ARNs ending in `:group/<name>` against the already-loaded `iam-group` list — a9s-devops: standard trust-policy parse; no extra API call. Principals are those the policy's Allow statements grant, less any an unconditional Deny takes away; another account's principal is never matched against a local row, and a policy that does not parse leaves the pivot unknown.
- **Count shown**: yes.

### `iam-user`

- **Why related**: Trust may include user principals — named humans who can assume this role.
- **How discovered**: same `AssumeRolePolicyDocument` parse as `iam-group`, matching `Principal.AWS` ARNs ending in `:user/<name>` against the already-loaded `iam-user` list — a9s-devops: same parse, different principal suffix. Principals are those the policy's Allow statements grant, less any an unconditional Deny takes away; another account's principal is never matched against a local row, and a policy that does not parse leaves the pivot unknown.
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

- **Why related**: Audit trail for the role's own configuration — who created it, who attached or detached its policies, who edited its trust policy. Universal pivot — applies to every registered type; see docs/related-resources.md §Policy.
- **How discovered**: `LookupEvents` filtered by `ResourceName == this role's ARN`, in us-east-1 where IAM records its events. `AssumeRole` calls naming this role are recorded in the Region the STS call was made in and under the caller's identity, so they belong to the caller's own history rather than to this row.
- **Count shown**: no — the row carries the row's own lookup, taken when it is opened.

## 3. Attention / Issues Algorithm

**Source API**: [GetRole](https://docs.aws.amazon.com/IAM/latest/APIReference/API_GetRole.html)

Transcribed from `docs/attention-signals.md § Signals § SECURITY & IAM` row `role`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `AssumeRolePolicyDocument` (URL-encoded JSON on `ListRoles`) has an Allow to `"*"` that no condition scopes to fixed values (see How obtained); an `sts:ExternalId` condition does not clear it.
  - **Explicit Deny**: a Deny statement that takes the grant from every caller on every granted resource clears the signal, and so do Deny statements that together fence both AWS service principals and every other caller to an account, organisation, VPC endpoint, address range or the principals a NotPrincipal block names. A conditional Deny that spares AWS service principals, or that names only part of the granted resources, does not clear it and is shown in the `Deny when` row. A condition that holds for a request without the key (a `ForAllValues:` operator), or that names the resource being called (`aws:ResourceAccount`, `aws:ResourceOrgID`, `aws:ResourceOrgPaths`, `s3:ResourceAccount`), scopes nobody.
  - **State bucket**: Broken.
  - **How obtained**: URL-decode and JSON-parse `Role.AssumeRolePolicyDocument` from the `ListRoles` response; evaluate it with `iampolicy.Evaluate`: an Allow to `Principal.AWS=="*"` (or `"*"`) is flagged unless a condition scopes the caller to fixed values (`docs/architecture.md` Policy Evaluation); `sts:ExternalId` scopes nobody, and the Attention block's Condition row shows the conditions the grant carries.

- **Signal**: an AWS service is trusted with no `aws:SourceAccount` / `aws:SourceArn` scoping — a condition scopes the trust only when a positive operator (`StringEquals`, `ArnLike` and their `ForAnyValue:` forms) compares `aws:SourceAccount`, `aws:SourceArn`, `aws:SourceOrgID` or `aws:SourceOrgPaths` against a value that names one source: an ARN with a fixed 12-digit account (its resource may be a wildcard), or any other value with no `*`, `?` or policy variable (an organization path may end in `*` below a fixed organization ID); `Null`, a negated operator, an `IfExists` variant and a `ForAllValues:` form (true for a request without the key) leave the trust unscoped. A service an unconditional Deny removes from the trust is not reported.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: the trust policy does not parse, so neither trust signal can be judged.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: `RoleLastUsed.LastUsedDate` missing or >90d (dormant; field is region-scoped — may false-warn in multi-region accounts).
  - **State bucket**: Warning.
  - **API call**: `GetRole` per role (one per resource).
  - **Cost shape**: per-resource.

- **Signal**: the role's policies, read together as IAM reads them, allow every action on every resource, and its permissions boundary, when it has one, does too. The statements of every attached (AWS-managed or customer-managed) and inline policy are joined into one document, so an unconditional Deny in any of them leaves less than every action. The document decides, not the policy's name: `AdministratorAccess` counts because its document is `Allow * on *`; `PowerUserAccess` does not, because its document withholds IAM, Organizations and Account.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type: `GetRole` (whose answer carries the permissions boundary), `ListAttachedRolePolicies` and `ListRolePolicies`, then each document and the boundary's through the session's policy document cache (`GetPolicy` + `GetPolicyVersion` once per managed policy, `GetRolePolicy` once per inline policy). A list, a document or a boundary that cannot be read marks the role not inspected with the refused call, and raises neither this signal nor the two below: the document nobody read may hold the Deny or the boundary that decides.
  - **Cost shape**: per-resource, with documents shared across roles.

- **Signal**: the joined policies grant a known privilege-escalation action combination that the attached policies alone grant, the boundary allows every action of it too, and the role is not an administrator. The rows name each policy that grants an action of the combination.
  - **Explicit Deny**: an action an unconditional Deny in any of the role's policies covers is not granted; a Deny with `NotAction` covers every action no listed entry overlaps. A conditioned Deny removes nothing.
  - **State bucket**: Broken.
  - **How obtained**: the same Wave 2 reads as the administrator signal.

- **Signal**: the joined policies grant a known privilege-escalation action combination, the boundary allows every action of it too, the role is not an administrator, and no combination is granted without an inline policy. A role shows this signal or the one above, never both.
  - **Explicit Deny**: as above.
  - **State bucket**: Broken.
  - **How obtained**: the same Wave 2 reads as the administrator signal.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `GenerateServiceLastAccessedDetails` async permission-usage audit.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `role`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| trust policy allows a wildcard principal with no restrictive condition | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `trust policy open to anyone` |
| the trust policy does not parse | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `trust policy not inspected` |
| an AWS service is trusted with no `aws:SourceAccount` / `aws:SourceArn` scoping compared against a concrete value | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `service can assume without source scoping` |
| dormant — `RoleLastUsed.LastUsedDate` missing or >90d | 2 | Warning | `~` | S2, S3, S4, S5 | `dormant role (><N>d)` |
| the role's policies together, within its boundary, allow every action on every resource (`PowerUserAccess` excluded — it withholds IAM, Organizations and Account) | 2 | Warning | `~` | S2, S3, S4, S5 | `has an administrator policy` |
| the role's attached policies together, within its boundary, grant a known privilege-escalation action combination | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `attached policy allows privilege escalation` |
| the role's policies together, within its boundary, grant a known privilege-escalation action combination only with an inline policy's grant | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `inline policy allows privilege escalation` |

A trust policy that does not parse raises neither trust signal: the row reads `trust policy not inspected` in yellow, and the Trust column reads `unknown` rather than claiming no wildcard.

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column.
- Keep the List text short enough to fit: ≤ 40 chars. The Detail cell quotes the finding's Detail constant verbatim, however long it is.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — a red row reading `trust policy open to anyone` is actionable on sight (name the accounts or the organization), and a yellow row reading `dormant role (><N>d)` tells the operator this role is a candidate for deletion without needing to open detail.

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
role — SECURITY & IAM. Status key: `state` — the column naming it is the status column, and no fetcher writes it, so the cell is the finding phrase.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Applies to | Detail |
| --- | --- | --- | --- | --- | --- |
| role.trust.confused-deputy | service can assume without source scoping | warn | wave1 | standard | An AWS service principal can assume this role on behalf of any caller, so another customer's resource can trick the service into using your role. Add an aws:SourceAccount or aws:SourceArn condition that requires the key to equal the account or ARN you expect; a condition that only says whether the key is set, or that lists what it must not be, scopes nothing. |
| role.inline-privilege-escalation | inline policy allows privilege escalation | broken | wave2 | standard | The policies of this role, read together within its permissions boundary, grant a combination of actions that lets its holder grant itself full administrator, and an inline policy supplies part of it. Remove the escalation actions from the inline policy, or scope them to resources that cannot be used to escalate. |
| iam-role.dormant | dormant role (><N>d) | warn | wave2 | standard | Nothing has assumed this role in over 90 days, so its trust policy and permissions are live but unexercised. Confirm the workload that used it is gone, then delete the role. |
| role.admin-attached | has an administrator policy | warn | wave2 | standard | A policy that applies to this principal allows every action on every resource, so anything it can be used for it can be used for everything. Replace that policy with a scoped policy covering only the actions this principal needs. |
| role.privilege-escalation | attached policy allows privilege escalation | broken | wave2 | standard | The managed policies attached to this role, read together within its permissions boundary, grant a combination of actions that lets its holder grant itself full administrator. Replace an AWS-managed policy among them with a narrower one, and remove the escalation actions from the policies the account owns, or scope them to resources that cannot be used to escalate. |
| role.trust.wildcard-principal | trust policy open to anyone | broken | wave1 | all | The trust policy allows any principal, so anyone can use this role, depending on the actions allowed. Replace the wildcard principal with the accounts or roles that need it, or add a condition that requires the caller's account or ARN to equal one you expect; a condition that only says whether a key is set scopes nothing. |
| role.trust.policy-not-inspected | trust policy not inspected | warn | wave1 | all | The trust policy document AWS returned does not parse, so a9s cannot say whether it opens this role to anyone. Read the document in the detail view and judge its grants by hand. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| lambda | Lambda Functions | yes |
| glue | Glue Jobs | yes |
| ng | Node Groups | yes |
| policy | IAM Policies | yes |
| ec2 | EC2 Instances | yes |
| eks | EKS Clusters | yes |
| iam-group | IAM Groups (trust) | no |
| iam-user | IAM Users (trust) | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
