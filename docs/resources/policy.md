---
shortName: policy
name: IAM Policies
awsApiRef: https://docs.aws.amazon.com/IAM/latest/APIReference/API_Policy.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# policy — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `policy`
- **Display name**: IAM Policies
- **AWS API reference**: <https://docs.aws.amazon.com/IAM/latest/APIReference/API_Policy.html>
- **List API**: `ListPolicies` (returns `Policy[]` with `Arn`, `PolicyName`, `AttachmentCount`, `DefaultVersionId`, `IsAttachable`, `Path`, `PermissionsBoundaryUsageCount`, `PolicyId`, `CreateDate`, `UpdateDate` — note: `Document` is NOT on the list response; it requires `GetPolicyVersion`).
- **Describe API (if any)**: `GetPolicyVersion` (Wave 2; fetches the policy `Document` for the `DefaultVersionId` so the JSON can be scanned for wildcard-admin statements).

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `ct-events`, `iam-group`, `iam-user`, `role`.

### `iam-group`

- **Why related**: Groups with this policy attached. Daily blast-radius question — when a policy is suspicious or being retired, the operator needs the list of groups carrying it. (related-resources.md §`policy`: "Groups with this policy attached.")
- **How discovered**: Call `ListEntitiesForPolicy(PolicyArn, EntityFilter=Group)` — AWS returns `PolicyGroups[].GroupName/GroupId` directly. — a9s-devops: the `Policy` shape on the list response has no attached-entity list (only `AttachmentCount`, a count, not a list), so either we call `ListEntitiesForPolicy` per policy detail-open (one call, bounded, clearly worth it for the blast-radius pivot) or we fan out through every group's `AttachedPolicies` from the other direction (N calls across the group list). `ListEntitiesForPolicy` is the canonical answer.
- **Count shown**: yes.

### `iam-user`

- **Why related**: Users with this policy attached directly (not via group). Direct user-attachments are a common cleanup target during least-privilege reviews. (related-resources.md §`policy`: "Users with this policy attached.")
- **How discovered**: Call `ListEntitiesForPolicy(PolicyArn, EntityFilter=User)` — AWS returns `PolicyUsers[].UserName/UserId`. Shares a single `ListEntitiesForPolicy` call with `iam-group` and `role` when `EntityFilter` is omitted (all three entity types returned together). — a9s-devops: one API call answers all three pivots at once; call shape is account-wide per-policy, not N+1 across entities.
- **Count shown**: yes.

### `role`

- **Why related**: Roles with this policy attached. For an incident where a policy grants too much, the operator needs every role trusting it. (related-resources.md §`policy`: "Roles with this policy attached.")
- **How discovered**: Call `ListEntitiesForPolicy(PolicyArn, EntityFilter=Role)` — AWS returns `PolicyRoles[].RoleName/RoleId`. Shares the single `ListEntitiesForPolicy` call noted above.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Audit trail for policy version / attach events — `CreatePolicyVersion`, `SetDefaultPolicyVersion`, `AttachUserPolicy`, `AttachRolePolicy`, `AttachGroupPolicy`, `DeletePolicy`. Universal audit pivot, especially sharp on IAM where every change is a security event. (related-resources.md §`policy`: "Audit trail for policy version / attach events.")
- **How discovered**: CloudTrail `LookupEvents` filtered by `ResourceName == <policy ARN>` or `ResourceType == AWS::IAM::Policy`. Same pattern as every other type — universal pivot.
- **Count shown**: unknown — universal pivot; count semantics vary by time window, not surfaced here.

> `ct-events` is the **universal pivot — applies to every registered type**; see related-resources.md §Policy.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md` §Security & IAM row `policy`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `AttachmentCount==0` AND not AWS-managed → Warning (orphan — customer-managed policy attached to nothing, dead weight during IAM cleanup).
  - **State bucket**: Warning.
  - **How obtained**: `AttachmentCount` field on the `ListPolicies` response. "Not AWS-managed" is derived from the `Arn` — AWS-managed policies live under `arn:aws:iam::aws:policy/...` (the literal account id `aws`); customer-managed policies live under `arn:aws:iam::<account-id>:policy/...`. Orphan detection runs only on the customer-managed subset.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: Policy document (default version) contains an `"Effect":"Allow","Action":"*","Resource":"*"` statement → Broken (wildcard admin — the policy is effectively administrator access, regardless of name).
  - **State bucket**: Broken.
  - **API call**: `GetPolicyVersion(PolicyArn, VersionId=<DefaultVersionId>)` — one call per policy. The `Document` field is URL-encoded JSON per SDK docs — decode, parse, then scan the `Statement[]` array for any entry with `Effect==Allow`, `Action` containing `*`, and `Resource` containing `*` (accounting for the field being either a string or a list).
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: IAM Access Advisor unused-permission analysis.

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
| `AttachmentCount==0`, customer-managed | 1 | Warning | n/a | S2, S4 | `orphan: 0 attachments` | `Customer-managed policy attached to no users, groups, or roles.` |
| Document has `Allow *:* on *` | 2 | Broken | `!` | S1, S4, S5 (S2 red; S3 suppressed on non-green) | `wildcard admin: Allow *:*` | `Default version grants Action=* on Resource=* — effective AdministratorAccess.` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column. Tests will assert the cause is present.
- For signals that legitimately have no operator-actionable cause, you may omit the row from this table entirely; §3 still describes it.
- Keep both columns short enough to fit: List text ≤ 40 chars, Detail text ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — an orphan policy reads `orphan: 0 attachments` (yellow row) and a wildcard-admin policy reads `wildcard admin: Allow *:*` (red row with `!` in S1 count); both problems are legible without opening detail, so triage happens on the list page. Operators drill into detail only to see the actual affected users/groups/roles in the related panel.

## 5. Out of Scope

- All §3.3 Wave 3 signals (IAM Access Advisor unused-permission analysis).
- Inline policies on users/groups/roles — this resource type is **managed policies only**. Inline policies are attributes of their parent entity, not standalone `policy` rows; they surface (if at all) on the parent entity's detail view. — a9s-devops: not worth it as a top-level row because inline policies have no standalone ARN, can't be listed account-wide in one call, and their blast radius is already constrained to the single parent entity.
- Public-accessibility / cross-account-usable-policy checks (e.g. policies granting access to `"Principal":"*"` in trust docs) — those live on `role` trust policies, not on managed-policy documents. — a9s-devops: managed policies do not have trust policies; only roles do.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- `policy` is a registered a9s resource type — `docs/related-resources.md` § Per-type contract row `policy`.
- Expected related targets `ct-events`, `iam-group`, `iam-user`, `role` — `docs/related-resources.md` § `policy` (line "`policy` | [API_Policy] | `ct-events`, `iam-group`, `iam-user`, `role`").
- Reasoning for each target pivot — `docs/related-resources.md` § `### policy` subsection (lines 746-753).
- Wave 1 signal `AttachmentCount==0 AND not AWS-managed → Warning (orphan)` — `docs/attention-signals.md` § Security & IAM row `policy`.
- Wave 2 signal `GetPolicyVersion` document contains `"Effect":"Allow","Action":"*","Resource":"*" → Broken (wildcard admin)` — `docs/attention-signals.md` § Security & IAM row `policy`.
- Wave 3 IAM Access Advisor unused-permission analysis — `docs/attention-signals.md` § Security & IAM row `policy`.
- Read-only invariant (§5) — `docs/architecture.md` § "What is a9s?".
- AWS SDK Go v2 — `Policy` shape fields `Arn`, `AttachmentCount`, `DefaultVersionId`, `IsAttachable`, `PolicyName`, `PolicyId`, `PermissionsBoundaryUsageCount`, `CreateDate`, `UpdateDate` — `AWS SDK Go v2 — service/iam/types.Policy`.
- AWS SDK Go v2 — `PolicyVersion.Document` is URL-encoded JSON (must be decoded before parsing) — `AWS SDK Go v2 — service/iam/types.PolicyVersion § Document`.
- AWS-managed ARN prefix `arn:aws:iam::aws:policy/` distinguishes AWS-managed from customer-managed — `AWS SDK Go v2 — service/iam/types.Policy § Arn` (value shape documented on the linked [ARNs] reference); also matches `ListPolicies` `Scope` parameter semantics (`AWS` vs `Local`).
- Discovery of attached entities via `ListEntitiesForPolicy` (single call returns `PolicyGroups`, `PolicyUsers`, `PolicyRoles`) — `a9s-devops (2026-04-20): possible=yes, worth=yes. The Policy list response only carries AttachmentCount (a count, not a list); blast-radius is the canonical IAM-review question when opening a policy, so one bounded call per detail-open is clearly worth it.`
- Counts shown for `iam-group`, `iam-user`, `role` pivots — `a9s-devops (2026-04-20): possible=yes, worth=yes. "How many users/groups/roles does this policy touch?" is the first question during a least-privilege review; the ListEntitiesForPolicy response already contains the lists so deriving counts is free.`
- Inline policies excluded from `policy` resource type (§5) — `a9s-devops (2026-04-20): possible=no as a standalone row (no standalone ARN, no account-wide list), worth=no at top level (blast radius already constrained to single parent entity). Surface on parent entity detail only.`
- Managed policies have no trust-policy wildcard check (§5) — `a9s-devops (2026-04-20): possible=no — managed policies do not have trust policies; AssumeRole-wildcard checks live on role resource type, not here.`
