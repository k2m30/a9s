---
shortName: iam-group
name: IAM Groups
awsApiRef: https://docs.aws.amazon.com/IAM/latest/APIReference/API_Group.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# iam-group — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `iam-group`
- **Display name**: IAM Groups
- **AWS API reference**: <https://docs.aws.amazon.com/IAM/latest/APIReference/API_Group.html>
- **List API**: `ListGroups` — returns `Group` entries with `GroupName`, `GroupId`, `Arn`, `Path`, `CreateDate`, and no attention-relevant runtime state — `AWS SDK Go v2 — iam/types.Group`.
- **Describe API (if any)**: `GetGroup` — used in Wave 2 to read the `Users[]` membership list.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `ct-events`, `iam-user`, `policy`.

### `iam-user`

- **Why related**: Members of this group — operator pivots here to see who inherits the group's permissions (blast-radius check before editing attached policies).
- **How discovered**: Call `GetGroup` on this group and read `Users[]` from the response. The same API used by Wave 2 — one call serves both the membership count and the related-panel pivot. (a9s-devops persona: possible=yes, worth=yes.)
- **Count shown**: yes (length of `Users[]`).

### `policy`

- **Why related**: Managed policies attached directly to this group — the permissions every member inherits. Editing or detaching here changes access for every user in the group at once.
- **How discovered**: Call `ListAttachedGroupPolicies` on this group; results are AWS-managed and customer-managed policy ARNs that match the `policy` resource type. Inline group policies exist on the API surface (`ListGroupPolicies`) but are deliberately out of scope for the related panel — the `policy` shortName covers managed policies only. (a9s-devops persona: possible=yes, worth=yes.)
- **Count shown**: yes (length of the attached-policies list).

### `ct-events`

- **Why related**: Universal pivot — audit trail for group membership changes (`AddUserToGroup`, `RemoveUserFromGroup`), policy attach/detach, and group-level config edits. Operator uses this to answer "who changed this group and when?".
- **How discovered**: Call `LookupEvents` with `LookupAttributes=[{AttributeKey=ResourceName, AttributeValue=<GroupName>}]`. Universal pivot — applies to every registered type; see docs/related-resources.md §Policy.
- **Count shown**: yes (count of matched events in the lookup window).

## 3. Attention / Issues Algorithm

**Source API**: [GetGroup](https://docs.aws.amazon.com/IAM/latest/APIReference/API_GetGroup.html)

Transcribed from `docs/attention-signals.md § Signals § SECURITY & IAM` row `iam-group`.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — the list API does not return fields usable for attention. `ListGroups` is config-only: it carries identity (`GroupName`, `GroupId`, `Arn`, `Path`) and `CreateDate`, none of which describe runtime state or a permission posture worth flagging without a Wave 2 call.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: `GetGroup` returns `Users==[]` AND group age (`now - CreateDate`) > 30d → Warning (orphan group — created, never populated, still carrying any attached policies).
  - **State bucket**: Warning.
  - **API call**: `GetGroup` — one call per group.
  - **Cost shape**: per-resource.

- **Signal**: `AdministratorAccess` attached. `PowerUserAccess` does not count: it withholds IAM, Organizations and Account, so its holder cannot grant itself permissions.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `ListAttachedGroupPolicies` per group (admin-access + blast radius) — used by the related panel for the `policy` pivot, not for attention scoring. Detecting `AdministratorAccess` attachment or wildcard-policy blast radius on groups is deferred.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `iam-group`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

Orphan groups are informational-cost signals, not operational breakage — `~` severity (informational background finding on a Healthy row). S1 is not bumped: the count takes a Wave 2 finding only at Broken. The row is yellow and the Attention section carries `~` and the cause, so the operator can triage later.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| no members, or members but no attached or inline policies | 2 | Warning | `~` | S2, S3, S4, S5 | `no members or no policies` |
| `AdministratorAccess` attached (`PowerUserAccess` excluded — it withholds IAM, Organizations and Account) | 2 | Warning | `~` | S2, S3, S4, S5 | `has an administrator policy` |

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — the row goes yellow and the Status column names the cause (`no members or no policies`), so the operator can decide to ignore it or drill into detail without a second keypress.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- Inline group policies (`ListGroupPolicies`). The `policy` shortName covers managed policies; inline policies are a separate surface that the related panel does not currently expose. (a9s-devops persona: not worth it for the related panel — inline policies are editable only in place, so a pivot row would point nowhere actionable inside a9s.)

## 6. Citations

- Related targets list — `docs/related-resources.md` § Per-type contract, row `iam-group` (`ct-events`, `iam-user`, `policy`).
- `iam-user` related rationale (members of group) — `docs/related-resources.md` § `iam-group`, bullet "**`iam-user`** — Members of this group."
- `policy` related rationale (attached managed policies) — `docs/related-resources.md` § `iam-group`, bullet "**`policy`** — Attached managed policies."
- `ct-events` universal pivot — `docs/related-resources.md` § Policy, item 4 "Universal pivots — `ct-events` (CloudTrail audit trail) is implicitly ...".
- List API = `ListGroups`, config-only — `core/aws/iam_groups.go`.
- Wave 2 signal (empty group >30d) — `docs/attention-signals.md § Signals § SECURITY & IAM` row `iam-group`.
- Wave 3 deferred — `docs/attention-signals.md § Not yet implemented`.
- `Group` fields available on the list response — `AWS SDK Go v2 — service/iam/types.Group § Arn, CreateDate, GroupId, GroupName, Path`.
- `GetGroup` returns `Users[]` — `AWS SDK Go v2 — service/iam.GetGroupOutput § Users, Group, IsTruncated`.
- `CreateDate` drives the ">30d" age calculation — `AWS SDK Go v2 — service/iam/types.Group § CreateDate`.
- `iam-user` discovery via `GetGroup.Users[]` — a9s-devops persona (2026-04-20): possible=yes, worth=yes. One call already made for Wave 2 — reuse its response for the related-panel count, no extra API cost.
- `policy` discovery via `ListAttachedGroupPolicies` — a9s-devops persona (2026-04-20): possible=yes, worth=yes. Managed policies attached to a group drive the blast-radius pivot operators care about when editing group membership.
- Inline group policies deferred — a9s-devops persona (2026-04-20): possible=yes, worth=no. The `policy` shortName is managed-policy-only, and inline policies are not independently navigable a9s resources.
- `~` severity for orphan-group finding — a9s-devops persona (2026-04-20): empty groups are a cost/hygiene signal, not operational breakage; `~` matches the "worth knowing, no action needed" slot. No S1 bump.
- Read-only invariant — `docs/architecture.md` § "What is a9s?" (read-only by design).

<!-- BEGIN GENERATED: header -->
iam-group — SECURITY & IAM. Lifecycle key: none (the list API returns no lifecycle field).
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| iam-group.orphan-or-noop | no members or no policies | warn | wave2 | This group grants nothing to nobody: it either has no members or carries no policies, so it only adds noise to access reviews. Delete it, or attach the policy and members it was created for. |
| iam-group.admin-attached | has an administrator policy | warn | wave2 | This principal is attached to an AWS-managed policy that grants administrator-equivalent access, so anything it can be used for it can be used for everything. Replace the managed policy with a scoped policy covering only the actions this principal needs. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| iam-user | IAM Users | no |
| policy | IAM Policies | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
