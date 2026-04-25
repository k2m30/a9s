# Lazy-Add Behavior — Stress Test User Stories

User-observable QA stories for the "lazy-add" contract: when the related panel
shows a target-type pivot with `Count >= 1`, drilling into that pivot MUST land
on a non-empty list containing those identities — even when the identities
would be excluded by the target type's top-level scope filter
(KMS customer-managed, AMI Owners=self, EBS-snapshot OwnerIds=self,
IAM-policy Scope=Local).

All stories are written at the operator-observable level. No source code or
internal structure is referenced.

Scope-filter baseline (the contract these stories stress):

| Target type  | Top-level list scope filter                              |
|--------------|----------------------------------------------------------|
| `kms`        | `KeyManager == CUSTOMER` (customer-managed only)         |
| `ami`        | `Owners=self` (account-owned images only)                |
| `ebs-snap`   | `OwnerIds=self` (account-owned snapshots only)           |
| `policy`     | `Scope=Local` (customer-managed policies only)           |

Cross-scope, resolved-filter drill-through is the happy path; the top-level
list must remain scope-filtered whenever it is opened directly from the main
menu.

---

## Table of Contents

- **Section A — Happy path per target type** (LA-001 .. LA-004)
- **Section B — Cross-scope mix and scope boundaries** (LA-010 .. LA-017)
- **Section C — Resolution failures and permission errors** (LA-020 .. LA-025)
- **Section D — Session lifecycle (profile / region / refresh)** (LA-030 .. LA-034)
- **Section E — Roundtrip, repeat, and idempotence** (LA-040 .. LA-044)
- **Section F — Race, timing, and concurrency** (LA-050 .. LA-054)
- **Section G — Count / footer / identity correctness** (LA-060 .. LA-064)
- **Section H — Size extremes and malformed inputs** (LA-070 .. LA-072)
- **Section I — Demo mode and baselines** (LA-080 .. LA-081)
- **Open contract questions**

---

## Section A — Happy path per target type

### STORY-LA-001: KMS drill-through shows AWS-managed key used by RDS

**Given** I am viewing an RDS DB instance's detail view
  **and** the instance's `KmsKeyId` resolves to `arn:aws:kms:us-east-1:123456789012:key/aws-managed-rds-uuid`
  **and** that key's `KeyManager` is `AWS` (alias `aws/rds`)
  **and** the top-level KMS list is scoped to `KeyManager==CUSTOMER`

**When** I press Enter on the `kms: 1` pivot in the related panel

**Then** a KMS list view opens
  **and** it contains exactly one row, the AWS-managed `aws/rds` key
  **and** the row's Alias column reads `aws/rds`
  **and** the footer does NOT show `m: load more`

**Notes** Without lazy-add, the default KMS list would hide this key because
it is AWS-managed. The drill-through contract is what guarantees the operator
sees it here.

### STORY-LA-002: AMI drill-through shows public marketplace AMI used by EC2

**Given** I am viewing an EC2 instance's detail view
  **and** the instance's `ImageId` is `ami-0abc1234` (an Amazon Linux public AMI,
  owner `amazon`)
  **and** the top-level AMI list is scoped to `Owners=self`

**When** I press Enter on the `ami: 1` pivot

**Then** an AMI list view opens
  **and** it contains exactly one row with ImageId `ami-0abc1234`
  **and** the row's Owner column shows `amazon`
  **and** the row's Name column is populated (e.g. `amzn2-ami-hvm-...`)
  **and** the footer does NOT show `m: load more`

**Notes** Compare to `aws ec2 describe-images --image-ids ami-0abc1234`. The
top-level `aws ec2 describe-images --owners self` would not return it.

### STORY-LA-003: EBS-snapshot drill-through shows shared snapshot used by EBS volume

**Given** I am viewing an EBS volume's detail view
  **and** the volume was created from `snap-0def5678`
  **and** that snapshot's `OwnerId` is a different AWS account (shared-in)
  **and** the top-level EBS-snapshot list is scoped to `OwnerIds=self`

**When** I press Enter on the `ebs-snap: 1` pivot

**Then** an EBS-snapshot list view opens
  **and** it contains exactly one row with SnapshotId `snap-0def5678`
  **and** the row's OwnerId column shows the foreign account id
  **and** the footer does NOT show `m: load more`

**Notes** Compare to `aws ec2 describe-snapshots --snapshot-ids snap-0def5678`
vs `aws ec2 describe-snapshots --owner-ids self`.

### STORY-LA-004: IAM-policy drill-through shows AWS-managed policy attached to role

**Given** I am viewing an IAM role's detail view
  **and** that role has `AdministratorAccess`
  (`arn:aws:iam::aws:policy/AdministratorAccess`) attached
  **and** the top-level IAM-policy list is scoped to `Scope=Local`

**When** I press Enter on the `policy: 1` pivot

**Then** an IAM-policy list view opens
  **and** it contains exactly one row, `AdministratorAccess`
  **and** the Scope column reads `AWS`
  **and** the footer does NOT show `m: load more`

**Notes** Compare to `aws iam get-policy --policy-arn arn:aws:iam::aws:policy/AdministratorAccess`.
`aws iam list-policies --scope Local` would not return it.

---

## Section B — Cross-scope mix and scope boundaries

### STORY-LA-010: Mixed in-scope and out-of-scope KMS keys resolve together

**Given** I am viewing a Lambda function's detail view
  **and** the function references two keys: `aws/lambda` (AWS-managed) and a
  customer-managed CMK `alias/my-app-kms`

**When** I press Enter on the `kms: 2` pivot

**Then** the KMS list shows exactly two rows
  **and** one row shows KeyManager `AWS` / alias `aws/lambda`
  **and** the other shows KeyManager `CUSTOMER` / alias `alias/my-app-kms`
  **and** the footer does NOT show `m: load more`

### STORY-LA-011: All out-of-scope targets still populate the drill

**Given** I am viewing an IAM role with exactly three attached managed policies,
  all AWS-managed (`AdministratorAccess`, `ReadOnlyAccess`, `IAMFullAccess`)

**When** I press Enter on the `policy: 3` pivot

**Then** the IAM-policy list shows exactly three rows, one per AWS-managed policy
  **and** each row's Scope column reads `AWS`
  **and** the footer does NOT show `m: load more`

### STORY-LA-012: All in-scope targets (no lazy-add needed)

**Given** I am viewing a role with exactly two attached policies, both
  customer-managed (`MyAppPolicy`, `BillingRead`)
  **and** the top-level IAM-policy list has already been loaded

**When** I press Enter on the `policy: 2` pivot

**Then** the IAM-policy list shows exactly those two rows
  **and** no extra AWS calls fire beyond what was needed for the top-level list
  **and** the footer does NOT show `m: load more`

### STORY-LA-013: Duplicate IDs emitted by checker — drill de-duplicates

**Given** I am viewing an RDS DB instance whose `KmsKeyId` is the same
  customer-managed CMK referenced again by a log-subscription filter on that
  instance's database logs
  **and** the related panel shows `kms: 1` (not 2)

**When** I press Enter on the `kms` pivot

**Then** the KMS list shows exactly one row for that CMK
  **and** the row is not duplicated despite two references on the source

**Notes** Observable contract: the pivot count and the drilled list must agree.

### STORY-LA-014: Empty result — no pivot action

**Given** I am viewing an EC2 instance with no attached customer AMI, no KMS
  key, and no IAM profile
  **and** the related panel shows `policy: 0`

**When** I attempt to drill the `policy` pivot

**Then** either the pivot row is inactive (unselectable / greyed) and Enter is
  a no-op,
  **or** pressing Enter lands on the main IAM-policy list view (scope-filtered
  to `Scope=Local`) — not on an empty drill-through

**Notes** Flagged in Open Contract Questions: the design does not specify
which of these two behaviors is canonical.

### STORY-LA-015: Scope boundary — ARN vs bare name

**Given** the related checker emits the full ARN
  `arn:aws:iam::aws:policy/AdministratorAccess`
  **and** the top-level IAM-policy list keys rows on `PolicyName`

**When** I drill the `policy: 1` pivot

**Then** the IAM-policy list shows a single row with PolicyName
  `AdministratorAccess`
  **and** the row renders correctly (not duplicated, not missing)

### STORY-LA-016: Scope boundary — KMS key UUID vs alias display

**Given** a related checker emits a KMS key UUID (no alias yet resolved)
  **and** the account has a `ListAliases` entry mapping that UUID to
  `alias/my-cmk`

**When** I drill the pivot

**Then** the KMS list row shows Alias `alias/my-cmk`
  **and** the KeyId column holds the UUID

### STORY-LA-017: Scope boundary — inline IAM policy on group surfaces via lazy-add

**Given** the source detail view is an IAM group
  **and** the group has one inline policy `MyInlineBilling`
  **and** the related panel shows `policy: 1`

**When** I drill the `policy` pivot

**Then** the IAM-policy list contains a single row for `MyInlineBilling`
  **and** the row is clearly marked as inline (not as a managed policy)
  **or** the row shows the parent group name so the operator can trace it

**Notes** Flagged in Open Contract Questions: spec is silent on whether inline
policies render as `policy` rows at all. Story pins whichever interpretation
the code emits.

---

## Section C — Resolution failures and permission errors

### STORY-LA-020: Partial resolution failure — some IDs resolve, some don't

**Given** the related checker emits 5 KMS key IDs
  **and** AWS returns 3 successful `DescribeKey` responses and 2 errors
  (1 `AccessDenied`, 1 `NotFound`)

**When** I drill the `kms: 5` pivot

**Then** the KMS list renders at least the 3 rows that resolved
  **and** the view does not crash, hang, or flash a modal error that blocks
  navigation
  **and** an indication (inline badge, toast, dim row, or footer message) makes
  clear that 2 items could not be resolved — OR they are silently skipped

**Notes** Flagged in Open Contract Questions: the design does not pin which of
"flag the missing IDs" vs "silently skip" is canonical.

### STORY-LA-021: Full resolution failure — every ID errors

**Given** the related checker emits 2 IAM-policy ARNs
  **and** both `GetPolicy` calls return `AccessDenied`

**When** I drill the `policy: 2` pivot

**Then** the view lands on an IAM-policy list
  **and** the list has zero rows OR shows two placeholder rows with an error
  marker
  **and** the view does not crash and Esc returns to the source detail

### STORY-LA-022: Permissions — `kms:ListAliases` denied but `kms:DescribeKey` allowed

**Given** my IAM identity can `kms:DescribeKey` but not `kms:ListAliases`

**When** I drill any `kms` pivot

**Then** each row renders with its KeyId in the KeyId column
  **and** the Alias column is blank or shows `—`
  **and** the drill still populates (no crash, no stall)

### STORY-LA-023: Permissions — `kms:DescribeKey` denied for one key in batch

**Given** the checker emits 4 KMS key IDs and my identity can `DescribeKey` for
  3 of them

**When** I drill the `kms: 4` pivot

**Then** the list shows the 3 resolvable keys (with full metadata)
  **and** the 1 forbidden key is silently skipped OR shown with a dim placeholder

**Notes** Same ambiguity as LA-020.

### STORY-LA-024: Permissions — `iam:GetPolicy` denied on AWS-managed policy

**Given** an unusual SCP blocks `iam:GetPolicy` on AWS-managed ARNs
  **and** the role I am inspecting attaches `AdministratorAccess`

**When** I drill the `policy: 1` pivot

**Then** the list shows a single row with ARN `arn:aws:iam::aws:policy/AdministratorAccess`
  **and** the PolicyName column still shows `AdministratorAccess` (parseable
  from the ARN)
  **and** the AttachmentCount / metadata columns may be blank

### STORY-LA-025: Throttling during drill — retry does not duplicate

**Given** AWS returns `Throttling` for the first `DescribeKey` call in a batch
  of 3
  **and** the app retries that call

**When** the retry succeeds and the drill completes

**Then** the list shows exactly 3 rows, not 4
  **and** no row is duplicated from the retry

---

## Section D — Session lifecycle (profile / region / refresh)

### STORY-LA-030: Profile switch clears lazy-added targets

**Given** I am in profile `A`
  **and** I have drilled the `policy: 1` pivot on a role, landing on
  `arn:aws:iam::111111111111:policy/A-Policy`

**When** I press Ctrl+P and switch to profile `B`
  **and** I navigate to a comparable role in profile `B`
  **and** I drill that role's `policy` pivot

**Then** the resulting list contains only policies belonging to account `B`
  **and** `A-Policy` is NOT visible
  **and** the Scope / ARN columns do not leak account `A` IDs

### STORY-LA-031: Region switch clears lazy-added targets

**Given** I drilled a KMS pivot in `us-east-1` and resolved a CMK there

**When** I press Ctrl+R (or equivalent region-change keybinding) and switch to
  `eu-west-1`

**Then** subsequent drills from `eu-west-1` detail views do NOT surface the
  `us-east-1` CMK
  **and** the main-menu KMS list opened in `eu-west-1` shows only `eu-west-1`
  customer-managed keys

### STORY-LA-032: Refresh during drill — behavior of the drill-through list

**Given** I have drilled into a KMS list containing 3 lazily-added
  AWS-managed keys

**When** I press `r` (refresh) on the drill-through list

**Then** the refreshed list continues to show the same 3 keys (filter is the
  contract)
  **or** the refresh refetches the filter's exact ID set and shows the same
  3 keys

**Notes** Flagged in Open Contract Questions — refresh semantics on a
filtered/narrowed list are unspecified.

### STORY-LA-033: Refresh of the source detail re-runs the checker

**Given** I am on an RDS DB instance detail view with `kms: 1`

**When** I press `r` on the detail view

**Then** the related panel re-counts
  **and** if the `KmsKeyId` changed since last load, the new count / target
  is shown
  **and** a subsequent drill picks up the new IDs, not the stale ones

### STORY-LA-034: Main-menu roundtrip keeps the top-level list scope-filtered

**Given** I have just drilled an `ami: 1` pivot that lazily added a public
  AMI (`ami-0abc1234`, owner `amazon`) to the KMS session state

**When** I press the main-menu key (`:` or equivalent) and choose `amis`

**Then** the top-level AMI list is scope-filtered (`Owners=self`)
  **and** `ami-0abc1234` is NOT present in the top-level list
  **and** the row count matches `aws ec2 describe-images --owners self`

---

## Section E — Roundtrip, repeat, and idempotence

### STORY-LA-040: Repeat drill from the same source is idempotent

**Given** I have just drilled `policy: 2` and pressed Esc back to the role
  detail view

**When** I press Enter on the `policy` pivot again

**Then** the list shows the same 2 rows, not 4
  **and** no duplicate entries appear

### STORY-LA-041: Repeat drill from a different source referencing the same target

**Given** I drilled role `Alpha`'s `policy: 1` pivot and saw `AdministratorAccess`
  **and** I navigate to role `Beta`, which also attaches `AdministratorAccess`

**When** I drill role `Beta`'s `policy: 1` pivot

**Then** the list shows a single row `AdministratorAccess`
  **and** no duplicate exists

### STORY-LA-042: Esc and re-drill after unrelated navigation

**Given** I drill a KMS pivot, Esc back, open a different related pivot (e.g.
  `logs`), Esc back again, then drill the KMS pivot once more

**When** the KMS drill repeats

**Then** the KMS list is identical to the first drill
  **and** neither the unrelated pivot nor the Esc path corrupted the state

### STORY-LA-043: Source-detail re-entry via Esc — uses cached result

**Given** I have pre-computed related checks on an EC2 instance detail view
  **and** I Esc to the list and re-open the same instance within the session

**When** I drill the `ami` pivot

**Then** the drill completes without a visible loading spinner for the
  already-resolved ID
  **and** the same AMI metadata is shown

### STORY-LA-044: Source with no related pivots — lazy-add never fires

**Given** I am viewing a resource type whose related-panel shows all-zero
  counts (baseline sanity — e.g. a brand-new empty VPC)

**When** I browse and Esc out

**Then** no drill-through paths are available
  **and** no extra AWS calls fire beyond what the detail view already needed

---

## Section F — Race, timing, and concurrency

### STORY-LA-050: Drill during ongoing background enrichment

**Given** I open a role detail view and the related-panel Wave-2 enrichment
  (e.g. `GetPolicyVersion` scans) is still running

**When** I drill the `policy: 3` pivot before enrichment finishes

**Then** the drill completes without waiting for enrichment
  **and** the list populates with the IDs the checker already emitted
  **and** when enrichment lands, the drill-through list is not reshuffled or
  corrupted

### STORY-LA-051: Esc during in-flight resolution

**Given** I press Enter on a `kms: 5` pivot
  **and** FetchByIDs is still in flight

**When** I press Esc before the batch completes

**Then** the source detail view reappears
  **and** the async result does NOT retroactively push me into the drill
  **and** the cache (if any) contains only what fully resolved, or nothing

### STORY-LA-052: Rapid consecutive Enter presses on the same pivot

**Given** I am hovering the `policy` pivot on a role detail

**When** I press Enter five times quickly

**Then** exactly one drill-through view opens
  **and** its list contains each ID exactly once
  **and** no double-fetch visibly fires (no flicker, no duplicated rows)

### STORY-LA-053: Profile switch mid-resolution discards the pending drill

**Given** I drill a KMS pivot in profile `A`
  **and** the resolution batch is still in flight

**When** I press Ctrl+P and switch to profile `B` before the drill completes

**Then** I land on the new profile's main menu (or equivalent entry view)
  **and** the old drill's result does NOT suddenly push me into a KMS list
  **and** profile `B`'s session state contains no keys from profile `A`

### STORY-LA-054: Region switch mid-resolution discards the pending drill

**Given** same setup as LA-053 but with Ctrl+R region switch

**Then** the `us-east-1` resolution's output must NOT surface in `eu-west-1`

---

## Section G — Count / footer / identity correctness

### STORY-LA-060: Pivot count equals drilled row count

**Given** the related panel shows `kms: 7`

**When** I drill

**Then** the resulting list has exactly 7 rows
  **and** if the list is longer than the viewport, the scrollbar / row-count
  footer reflects 7 total

**Notes** If partial-resolution failures occur, see LA-020 — this story covers
the no-failure happy path only.

### STORY-LA-061: Footer `m: load more` suppressed when filter is fully resolved

**Given** I drill an `ami: 3` pivot and all 3 AMIs resolve

**When** the list renders

**Then** the footer does NOT show `m: load more`
  **and** pressing `m` (if pressed) has no effect

**Notes** Baseline contract: a fully-resolved filter is complete.

### STORY-LA-062: Footer suppression even when upstream top-level list was truncated

**Given** the account has > 1000 customer-managed KMS keys and the top-level
  prefetch was truncated
  **and** I drill a `kms: 2` pivot whose both IDs fully resolved

**When** the drill-through list renders

**Then** the footer does NOT show `m: load more`
  **and** the truncation on the top-level list does NOT leak into this
  narrowed, fully-resolved filter

### STORY-LA-063: Single-ID auto-open (or not — pin observed behavior)

**Given** the related panel shows `kms: 1`

**When** I press Enter on the pivot

**Then** either the KMS detail view for that single key opens directly,
  **or** a one-row KMS list view opens and Enter again opens the detail

**Notes** Flagged in Open Contract Questions — spec does not pin single-ID
auto-open. Pin whichever behavior a9s currently exhibits.

### STORY-LA-064: Count never shows a number the drill can't deliver

**Given** the checker emitted 5 IDs
  **and** the related panel shows `kms: 5`

**When** I drill, and 2 of the 5 fail to resolve

**Then** the related panel on the source detail still reads `5` (count is
  honest to the checker's output)
  **and** the drilled list behavior is governed by LA-020 / LA-021

**Notes** Counter-story: if design prefers "count = resolvable", LA-064 must be
updated accordingly. Flagged in Open Contract Questions.

---

## Section H — Size extremes and malformed inputs

### STORY-LA-070: Very large ID set — 100 IDs drill without timeout

**Given** the related checker emits 100 IAM-policy ARNs (an over-permissioned
  role with every AWS-managed policy attached)

**When** I press Enter on `policy: 100`

**Then** the drill completes within a reasonable time (operator does not see a
  hung UI)
  **and** all 100 rows render (or pagination / lazy-render kicks in without
  losing rows)
  **and** the `m: load more` footer is absent if all 100 are resolved

### STORY-LA-071: Malformed IDs are filtered out safely

**Given** the related checker accidentally emits `""`, `"arn:aws:"`, and one
  valid KMS key ARN

**When** I drill

**Then** the list shows 1 row (the valid key)
  **and** the malformed strings do not render as rows
  **and** no panic, crash, or modal error blocks navigation

### STORY-LA-072: ID set grows across re-drill due to source change

**Given** I drill a role's `policy: 2` pivot, see the list, Esc back
  **and** an external change (out-of-band IAM attach) means the role now
  attaches 3 policies
  **and** I press `r` on the source detail view to refresh

**When** I drill again

**Then** the count reads `3`
  **and** the drill-through list shows 3 rows, including the newly-attached
  policy
  **and** the old 2-row cache is not shown stale

---

## Section I — Demo mode and baselines

### STORY-LA-080: Demo mode baseline

**Given** I started a9s with `--demo`

**When** I navigate detail views and drill related pivots for types with
  fixture data covering out-of-scope targets (e.g. an RDS instance whose
  demo fixture references `aws/rds`)

**Then** the drill behavior matches LA-001 style (non-empty list, correct
  Alias / Scope / Owner column)

**Notes** If the demo fixture contains no out-of-scope IDs, this story
degrades to the in-scope drill in LA-012. The demo fixture surface area is
documented in `internal/demo/fixtures/` (file-location note only — do not
read the fixture files to author these stories).

### STORY-LA-081: Cold-cache drill triggers prefetch for the target type

**Given** I launch a9s, navigate straight to an RDS instance detail view, and
  have never opened the KMS list in this session

**When** I drill the instance's `kms: 1` pivot

**Then** the drill still populates the 1-row list with correct Alias
  **and** subsequently pressing the main-menu key and choosing `kms` shows
  the full scope-filtered customer-managed list (the prefetch completed or
  fires on menu entry)

### STORY-LA-082: Warm-cache drill re-uses the top-level fetch and adds only missing IDs

**Given** I opened the main-menu KMS list 2 minutes ago (it is warm)
  **and** I navigate to an RDS detail view whose `kms: 1` pivot points at an
  AWS-managed key not in the warm list

**When** I drill

**Then** the list shows exactly 1 row for the AWS-managed key
  **and** no full KMS list refetch happens on the drill

---

## Open contract questions

The stories above pin observable behavior where the contract is explicit.
The following items are ambiguous in the published design spec and have
multiple plausible interpretations. Each is flagged inline in the relevant
story; this list is a consolidated index for design follow-up.

1. **LA-014 — empty pivot (Count=0)**: should the pivot be visually inactive
   and Enter be a no-op, or should Enter open the scope-filtered top-level
   list for that target type? Either is defensible; a9s needs one answer.

2. **LA-017 — inline IAM policies as `policy` rows**: the `policy` spec §5
   explicitly excludes inline policies from the `policy` resource type, but
   the task brief mentions an inline-group path that can emit an inline
   entry during a drill. Whether drill-through surfaces inline policies as
   rows in the IAM-policy list view is unresolved.

3. **LA-020 / LA-023 — partial resolution failure UX**: spec is silent on
   whether unresolved IDs should be (a) silently skipped, (b) rendered as
   dim placeholder rows, or (c) flagged via a toast / footer message. Need
   a pinned answer.

4. **LA-032 — refresh of a filtered drill-through list**: does `r` on a
   narrowed list refetch each filtered ID, reload the parent top-level list
   and re-filter, or no-op? Spec does not say.

5. **LA-063 — single-ID auto-open**: pivots with `Count=1` could either
   auto-open the lone target's detail view or land on a one-row list. Spec
   does not pin this.

6. **LA-064 — count honesty under partial failure**: should the related
   panel count reflect "emitted by the checker" or "resolvable right now"?
   The two diverge during permission errors.

7. **Count unknown** (`?` / "approximate"): per `kms.md` §2, some targets
   (e.g. `role` on KMS, `ct-events` generally) are documented as unknown-count.
   The drill-through UX for an unknown-count pivot is not specified —
   does Enter fire, or is the pivot inactive, or does Enter drill with a
   "best-effort" list?

8. **Lazy-add persistence across `r` on the source detail**: once a drill has
   enriched session state with an out-of-scope key, does pressing `r` on the
   source detail view retain that enrichment, clear it, or re-resolve it?

9. **Source-side cache visibility after lazy-add**: if I drill from role `A`
   and lazy-add key `K`, then open role `B` whose detail also references `K`,
   is `B`'s drill served from cached `K` metadata or does it re-fetch?

10. **Cross-region ARN handling**: if a related checker emits an ARN from a
    different region than the current session region (e.g. an IAM global
    resource referenced from a regional context), does the drill follow the
    ARN or silently fail? IAM is mostly global, but EC2/KMS are regional.
