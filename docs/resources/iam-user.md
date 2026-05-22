---
shortName: iam-user
name: IAM Users
awsApiRef: https://docs.aws.amazon.com/IAM/latest/APIReference/API_User.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# iam-user — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `iam-user`
- **Display name**: IAM Users
- **AWS API reference**: <https://docs.aws.amazon.com/IAM/latest/APIReference/API_User.html>
- **List API**: `ListUsers` (global; IAM is region-less — one call covers the account).
- **Describe API (if any)**: `ListAccessKeys` + `GetAccessKeyLastUsed` (per key) + `ListMFADevices` + `GetLoginProfile`, all per user. Wave 3 alternative is the account-wide `GenerateCredentialReport` + `GetCredentialReport`, which is OUT OF SCOPE (§3.3).

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `iam-group`, `policy`, `ct-events`.

### `iam-group`

- **Why related**: "Groups the user belongs to." Permissions for a user are typically inherited from group memberships — operator triaging "what can this user do?" needs the group list first.
- **How discovered**: call `ListGroupsForUser(UserName)` — returns `Groups[]`, each with `GroupName` that resolves into the `iam-group` list. SDK field: `ListGroupsForUserInput.UserName` → `ListGroupsForUserOutput.Groups[].GroupName`.
- **Count shown**: yes.

### `policy`

- **Why related**: "Attached managed policies." Direct user-attached managed policies are a second source of permissions (and an anti-pattern worth surfacing — best practice is to attach via group).
- **How discovered**: call `ListAttachedUserPolicies(UserName)` — returns `AttachedPolicies[]` with `PolicyArn` / `PolicyName` that resolves into the `policy` list. SDK field: `ListAttachedUserPoliciesInput.UserName` → `ListAttachedUserPoliciesOutput.AttachedPolicies[]`.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Universal pivot — "Audit trail for user actions and credential changes." Answers "what has this user been doing?" — the single most common question when a user looks dormant, has an old key, or is suspected of credential compromise.
- **How discovered**: `LookupEvents` with attribute `Username == <user.UserName>` (the iam-user short-name is the bare user name, not the ARN — see `docs/testing-detail-view-coverage.md` §35 and `docs/design/resource-to-cloudtrail.md` §98).
- **Count shown**: yes.
- Universal pivot — applies to every registered type; see `related-resources.md` §Policy.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `PasswordLastUsed` absent AND `CreateDate` >90d → dormant console user.
  - **State bucket**: Warning.
  - **How obtained**: `ListUsers` response — `User.PasswordLastUsed` (nullable `*time.Time`) and `User.CreateDate` (`*time.Time`). Both fields are present on the `ListUsers` output shape; no extra call is required.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: Access key with `Status==Active` AND `AccessKeyLastUsed.LastUsedDate` >90d → stale active key.
  - **State bucket**: Warning.
  - **API call**: `ListAccessKeys(UserName)` — one per user; then `GetAccessKeyLastUsed(AccessKeyId)` — one per discovered key (AWS users are capped at 2 access keys, so this is ≤2 calls per user).
  - **Cost shape**: per-resource.

- **Signal**: Access key with `Status==Active` AND `AccessKeyLastUsed.LastUsedDate` is null (never used) AND `AccessKeyMetadata.CreateDate` >90d → never-used old key.
  - **State bucket**: Warning.
  - **API call**: same pair as above — `ListAccessKeys(UserName)` + `GetAccessKeyLastUsed(AccessKeyId)`.
  - **Cost shape**: per-resource.

- **Signal**: `GetLoginProfile(UserName)` returns a profile (console login enabled) AND `ListMFADevices(UserName)` returns `MFADevices==[]` → console login without MFA.
  - **State bucket**: Broken.
  - **API call**: `GetLoginProfile(UserName)` — one per user; `ListMFADevices(UserName)` — one per user. Two calls per user total (short-circuit: skip `ListMFADevices` if `GetLoginProfile` throws `NoSuchEntity`).
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: Credential report (`GenerateCredentialReport` + `GetCredentialReport`) — async/polling, provides all of the above in one account-wide pull when cached. Excluded because the async polling flow does not fit the synchronous per-resource enrichment model; the per-user Wave 2 calls above deliver equivalent signal within a9s's request model.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause. **Healthy rows render blank.** |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4, S5.
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph on green row. S3, S4, S5. No S1.
- **Wave 2 finding on an already yellow/red/dim row** → S3 suppressed (color is already the signal), S4 deduplicates with the existing cause, S5 still carries the full sentence, S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `PasswordLastUsed` null AND `CreateDate` >90d | 1 | Warning | n/a | S2, S4 | `dormant: no console login in 90d+` | `Console user created 2y ago has never signed in — candidate for removal.` |
| Active key unused >90d | 2 | Warning | `~` | S3, S4, S5 | `key unused 120d` | `Access key AKIA…4QJZ last used 120 days ago — consider rotating or deactivating.` |
| Active key never used, CreateDate >90d | 2 | Warning | `~` | S3, S4, S5 | `key never used, 180d old` | `Access key AKIA…4QJZ created 180 days ago and never used — candidate for deletion.` |
| Console login without MFA | 2 | Broken | `!` | S1, S3, S4, S5 | `console login, no MFA` | `User has console password but zero MFA devices — add MFA or remove password.` |

Rules applied:

- All cause text avoids banned jargon (`Wave 1`, `Wave 2`, `finding`, `enrichment`, etc.).
- No bare state keyword stands alone — every S4 value pairs the state with an operator-readable cause and (where relevant) an age.
- Wave 1 Healthy users (active console user OR user created <90d ago) produce no §4 row — green, blank Status, no glyph. Silence is the UX.
- The "console login without MFA" finding is classed Broken (`!`) even though the user's row is green under Wave 1 — the security risk is critical enough to bump S1 and draw a `!` on the green row. Per skill rule "Wave 2 Broken-style background finding on a Healthy resource gets `!` → S1, S3, S4, S5".
- Stale/never-used key findings are `~` informational — they're hygiene prompts, not incidents, and shouldn't inflate the menu issues count.

## 4.1 UX review

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every row carries a cause in the Status column: dormant rows say `dormant: no console login in 90d+`, console-without-MFA rows say `console login, no MFA` with a `!` glyph, and stale-key rows say `key unused 120d` with a `~` glyph. The operator can triage the whole list (delete dormant users, fix MFA on the flagged users, rotate the aging keys) without opening a single detail view.

## 5. Out of Scope

- All §3.3 Wave 3 signals (credential report polling flow).
- `iam-user` → `kms` related panel — no direct key-user attribute on a user. Cited at `docs/related-resources.md` §"Known NOT-related pairs".
- `iam-user` → `role` related panel — indirect via trust policies across all roles; would require a reverse scan. Cited at `docs/related-resources.md` §"Known NOT-related pairs".
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- Related targets `iam-group`, `policy`, `ct-events` — `docs/related-resources.md` § "Per-type contract" table row for `iam-user`, and § "`iam-user`" detail block.
- `iam-group` discovery via `ListGroupsForUser(UserName)` — `AWS SDK Go v2 — iam.ListGroupsForUserInput § UserName` (required field); output `Groups[].GroupName` keys into `iam-group` list.
- `policy` discovery via `ListAttachedUserPolicies(UserName)` — `AWS SDK Go v2 — iam.ListAttachedUserPoliciesInput § UserName` (required field); output `AttachedPolicies[].PolicyArn`.
- `ct-events` discovery by `Username` attribute — `docs/related-resources.md` §`ct-events` ("`userIdentity.userName` (Type=IAMUser)"); `docs/testing-detail-view-coverage.md` §§35, 93 (short-name is the bare user name).
- Wave 1 `PasswordLastUsed` / `CreateDate` signal — `docs/attention-signals.md` § "Security & IAM" table, `iam-user` row; `AWS SDK Go v2 — iam/types.User § PasswordLastUsed` and `§ CreateDate` (both on `ListUsers` response).
- Wave 2 access-key age signals — `docs/attention-signals.md` § "Security & IAM" table, `iam-user` row; `AWS SDK Go v2 — iam/types.AccessKeyMetadata § Status, CreateDate` and `iam/types.AccessKeyLastUsed § LastUsedDate`.
- Wave 2 console-without-MFA signal — `docs/attention-signals.md` § "Security & IAM" table, `iam-user` row ("console login enabled AND no MFA device → Broken"); `AWS SDK Go v2 — iam/types.MFADevice § SerialNumber, EnableDate` (per `ListMFADevices`); `GetLoginProfile` presence indicates console access.
- Wave 3 credential report — `docs/attention-signals.md` § "Security & IAM" table, `iam-user` row Wave 3 cell; OUT OF SCOPE per template rule.
- Severity assignment `!` vs `~` for Wave 2 findings — persona decision: MFA-missing is a security incident (Broken + `!` drives S1 count); stale/never-used keys are hygiene (`~` annotation, no S1 bump). Grounded in skill surface rules (§S1, §S3) and CIS IAM benchmarks' treatment of MFA as mandatory.
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
- `iam-user` NOT-related pairs (`kms`, `role`) — `docs/related-resources.md` §"Known NOT-related pairs" lines 1101–1102.

<!-- BEGIN GENERATED: header -->
iam-user — SECURITY & IAM. Lifecycle key: `state`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Approximate? |
| --- | --- | --- |
| iam-group | IAM Groups | no |
| policy | IAM Policies | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
