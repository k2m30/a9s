# CloudTrail Events List View — Design v2

Status: **proposed, awaiting approval**. Supersedes the coloring, sorting,
target extraction, and filter sections of `ct-event-list.md` once approved.
Tear-down of the existing per-cell ANSI composition is part of this spec.

Author: a9s-architect, 2026-04-08
Driver: visible bugs in v1 (per-cell rainbow, broken cursor highlight,
double sort glyph, "(none)" targets, BatchGetImage classified as write).

---

## 1. Coloring model — back to ec2 simplicity

**Rule: one row = one color.** No per-cell classifiers, no per-cell ANSI
composition, no `Color` field on `ListColumn`. The entire `internal/tui/views/`
per-cell pipeline (`cellOverrideFor`, `verbOverride`, `actorOverride`,
`outcomeOverride`, `originOverride`, `cellStyleFor`, `verbStyle`, `actorStyle`,
`outcomeStyle`, `originStyle`, `applyVerbColor`, `applyActorColor`,
`applyOutcomeColor`, `applyOriginColor`, `ApplyCellColor`) is **deleted**.

`renderDataRow` reverts to ec2 form: every cell renders with `base` only.
`base` is `RowSelected` on the cursor row, otherwise `RowColorStyle(r.Status)`.

### 1.1 Severity ladder

Three semantic statuses on `Resource.Status`. Names are literal strings
consumed by `RowColorStyle`:

| Status        | Color (palette)         | Meaning                                             |
|---------------|-------------------------|-----------------------------------------------------|
| `ct-info`     | `ColTerminated` (dim)   | Routine reads, normal-volume noise. Worth ignoring. |
| `ct-attention`| `ColPending` (yellow)   | Worth a glance. Writes, ROOT, sensitive reads, cross-account. |
| `ct-danger`   | `ColStopped` (red)      | Worth investigating now. Destructive ops, failures. |

Cursor row uses `RowSelected` (blue bg + readable fg+bold), suppressing the
status color — same as ec2 list. No exceptions.

### 1.2 Severity precedence

Computed once in the fetcher per event. **Highest match wins.** Top to bottom:

1. **`ct-danger`** if ANY of:
   - `errorCode != ""` (failure — overrides everything else)
   - Verb is `D` (destructive — see §2)
2. **`ct-attention`** if ANY of:
   - Verb is `W` (write)
   - User identity is Root (`userIdentity.type == "Root"`)
   - Cross-account (`accountId != recipientAccountId`)
   - Event is in the **sensitive-reads allowlist** (§1.3)
3. **`ct-info`** otherwise.

The severity is stored on `Resource.Status` and that is the only thing the
view layer uses for coloring. The verb glyph in the V column is plain text;
the V column has no `color` classifier.

### 1.3 Sensitive-reads allowlist

Reads of secret material — escalate to `ct-attention` even though they're
verb=R. Hard-coded list (no service heuristic; small enough to maintain):

```
secretsmanager:GetSecretValue
secretsmanager:BatchGetSecretValue
ssm:GetParameter
ssm:GetParameters
ssm:GetParametersByPath
sts:GetSessionToken
sts:GetFederationToken
sts:AssumeRole
sts:AssumeRoleWithSAML
sts:AssumeRoleWithWebIdentity
iam:GetAccessKeyLastUsed
iam:ListAccessKeys
iam:GetCredentialReport
iam:GenerateCredentialReport
iam:GetLoginProfile
acm:ExportCertificate
```

KMS is **excluded** per user direction. Match key is exact `<service>:<EventName>`
where service is derived from `eventSource` (e.g. `s3.amazonaws.com` → `s3`).

### 1.4 Cross-account visibility

When `accountId != recipientAccountId`:
- `Resource.Status` escalates to `ct-attention` (yellow) per §1.2.
- The ACTOR cell text is prefixed with the **counterparty account ID**
  using slash separator: `<accountID>/<actor>`.
  Example: `999988887777/alice`. Width consumed by the prefix is 13 chars
  (12-digit account + "/").

The legacy `[cross]` literal prefix is removed.

---

## 2. Verb classification

Verb classification stays heuristic-on-name, but the name → verb table is
rearranged. Verb is computed from `eventName` and `eventCategory`/`eventType`.

### 2.1 Verb table

Order matters; first match wins. All matches are case-sensitive against the
exact `eventName`.

| Verb | Match rule                                                                   | Examples                                              |
|------|------------------------------------------------------------------------------|-------------------------------------------------------|
| `D`  | Prefix `Delete`, `Terminate`, `Destroy`, `Remove`, `Revoke`, `Disable`, `Stop`, `Detach`, `Cancel`, `Reject`, `Abort`, `Purge`, `Deregister`, `Disassociate` | `DeleteBucket`, `TerminateInstances`, `RevokeSecurityGroupIngress` |
| `R`  | Prefix `Get`, `Describe`, `List`, `Lookup`, `Search`, `Query`, `Scan`, `Head`, `Test`, `Check`, `Validate`, `Verify`, `Decrypt`, `BatchGet` | `GetObject`, `BatchGetImage`, `Decrypt`, `DescribeInstances` |
| `R`  | (additional) `Encrypt`, `Sign`, `Verify`, `ReEncrypt`, `GenerateDataKey`, `GenerateDataKeyWithoutPlaintext` — KMS use-key ops, no resource mutation | `Encrypt`, `Sign`, `GenerateDataKey` |
| `W`  | Prefix `Create`, `Put`, `Update`, `Modify`, `Set`, `Add`, `Attach`, `Associate`, `Register`, `Enable`, `Start`, `Run`, `Restore`, `Restart`, `Reboot`, `Tag`, `Untag`, `Activate`, `Reset`, `Replace`, `Apply`, `Import`, `Export`, `Copy`, `Move`, `Upload`, `Submit`, `Send`, `Publish`, `Invoke`, `Execute`, `Transition`, `Issue`, `Renew`, `Rotate` | `CreateBucket`, `PutObject`, `UpdateFunctionCode` |
| `S`  | `eventType == "AwsServiceEvent"`                                             | `InvokeExecution` from `states.amazonaws.com`         |
| `I`  | `eventCategory == "Insight"`                                                 | `ApiCallRateInsight`                                  |
| `N`  | `eventCategory == "NetworkActivity"`                                         | `VpcEndpointAccess`                                   |
| `?`  | none of the above                                                            | unknown future API names                              |

**Bug fixes baked in:**
- `BatchGetImage`, `BatchGetSecretValue`, `BatchGetItem` → `R` (was `W`).
- Other `Batch*` (e.g. `BatchWriteItem`, `BatchDeleteAttributes`) → fall to
  `W`/`D` via the normal prefix tables.
- `Decrypt` → `R` (use-key, no mutation).
- `Encrypt`, `Sign`, `ReEncrypt`, `GenerateDataKey*`, `Verify` → `R`
  (key use, no resource mutation; user direction).

### 2.2 Verb glyph color

The V column has **no per-cell color classifier**. The glyph itself is just
the letter `R`/`W`/`D`/`S`/`I`/`N`/`?` rendered in the row's `Status` color
(`ct-info` dim / `ct-attention` yellow / `ct-danger` red). Same as every
other cell.

---

## 3. Time format

`event_time` field stores the raw RFC3339 timestamp (unchanged, used by sort
and detail view). The list view renders it via a new `formatCTTimestamp`
helper:

```
in:  2026-04-07T17:00:59Z
out: Apr 07 17:00:59
```

Format: `<MonthAbbr> <DD> <HH:MM:SS>`, fixed 15 characters, zero-padded day.
Locale: English month abbreviations (`Jan` … `Dec`). The helper lives in
`internal/aws/ct_events.go` and runs at fetch time so the value stored in
`Resource.Fields["time"]` is already pre-formatted; sort uses
`Resource.Fields["event_time_raw"]` (new key) or the existing RFC3339 in
`event_time`.

TIME column width drops from 19 to 15.

---

## 4. Target extraction — fallback table

Target extraction order (first non-empty wins):

1. `Resources[0].ResourceName` from the SDK convenience slice
2. `requestParameters.<resourceField>` lookup via per-event-name fallback table
3. `responseElements.<resourceField>` lookup via the same table
4. Existing tag-based / ARN-based extraction in `ExtractCTTarget`
5. Literal `(none)`

The per-event fallback table is hard-coded in `internal/aws/ct_events.go`:

| Event name (any service)             | Lookup path in `requestParameters`            | Notes |
|---------------------------------------|-----------------------------------------------|-------|
| `DescribeInstances`                   | `instancesSet.items[*].instanceId` (joined `,`) | If empty list → `(all)` |
| `UpdateInstanceInformation`           | `instanceId`                                  | SSM agent ping |
| `GetParameter` / `GetParameters`      | `name` / `names[]`                            | |
| `GetParametersByPath`                 | `path`                                        | |
| `GetSecretValue`                      | `secretId`                                    | |
| `Decrypt`                             | `keyId` (or absent → `(by alias)`)            | |
| `AssumeRole*`                         | `roleArn`                                     | strip arn prefix per §5 |
| `ListBuckets`                         | `(none)` literal — there is no target         | |
| `Lookup*` / `Search*`                 | `lookupAttributes` summary if present         | best-effort |
| `BatchGetImage`                       | `repositoryName`                              | |
| `BatchGet*` (DynamoDB)                | `requestItems` keys joined                    | |
| `Get*` / `Describe*` (catch-all)      | scan for any key matching `*Id`/`*Name`/`*Arn` | last resort |

The catch-all "scan for any key matching `*Id`/`*Name`/`*Arn`" is the generic
fallback so we don't have to enumerate every single API.

---

## 5. Target column rendering — strip ARN noise

ARNs in the TARGET cell collapse via this transform, applied at render time
(not at fetch time, so the raw `_ct.target` field stays intact for filtering
and detail view):

```
arn:aws:<service>:<region>:<account>:<resource> → <resource>
```

Examples:
- `arn:aws:s3:::webapp-assets-prod` → `webapp-assets-prod`
- `arn:aws:iam::123456789012:user/alice` → `user/alice`
- `arn:aws:lambda:us-east-1:123456789012:function:my-fn` → `function:my-fn`
- `arn:aws:ec2:us-east-1:123456789012:instance/i-0abc` → `instance/i-0abc`

**Cross-account exception:** when the ARN's account ID differs from the local
recipient account, keep the account ID inline so the user can see the
counterparty:

```
arn:aws:s3:::shared-bucket  (same — strip)         → shared-bucket
arn:aws:iam::999988887777:role/Admin (cross-acct) → 999988887777:role/Admin
```

The transform lives in a new helper `formatCTTarget(rawARN string, localAccount string) string`
in `internal/aws/ct_events.go`. The fetcher passes it through before storing
in `Resource.Fields["_ct.target"]`. Detail view uses `Resource.Fields["_ct.target_raw"]`
for the unmodified ARN.

TARGET column width grows from 28 to 36.

---

## 6. Sort indicator binding (cosmetic only)

Bug is **cosmetic**: actual sort order is correct, but `colHeaderTitle`
matches `SortAge` against any column whose key/title contains
`time`/`event`/`date`/etc. EVENT column title contains "event" → both TIME
and EVENT get the ↓ glyph in the header. The data is sorted by time
correctly; only the header decoration is wrong.

Fix scope: header decoration only. Bind the indicator to **one explicit
column** via `sortColKey string` on `ResourceListModel`, set when sort
changes. `colHeaderTitle` does an exact-key check instead of substring
match. The underlying `isAgeKey`-style matching stays where it's used for
sort *field selection* (resolving which `Resource.Fields` key the
comparator reads).

For ct-events the default sort is by time → `sortColKey = "event_time"`,
which matches only the TIME column.

---

## 7. Filter: `ctrl+z` — toggle "show only what matters"

**Global** key binding registered in `internal/tui/keys/keys.go`. Active on
every resource list view. Semantics: hide rows whose `RowColorStyle(Status)`
falls into the dim/neutral branch; show only colored rows.

### 7.1 What counts as "dimmed"

A row is considered dim and hidden when toggle is on, if its `Status`
resolves through `RowColorStyle` to either:
- `ColTerminated` (the explicit dim color), OR
- `ColHeaderFg` (the default fall-through neutral color)

Concretely, the canonical "dimmed" status values across the app:
- ct-events: `ct-info`
- ec2: `terminated`, `shutting-down`, `stopped` (debatable — see §7.3)
- iam, vpc, etc: anything that resolves to default `ColHeaderFg`
- generic: any `Resource.Status == ""` (empty → default neutral)

The implementation does NOT enumerate status strings. It calls
`styles.IsDimRowColor(r.Status)` which inspects the `RowColorStyle` output
for each row and decides. New helper:

```go
// IsDimRowColor reports whether RowColorStyle for the given status produces
// a dim or neutral foreground (i.e., the row has no severity signal worth
// the user's attention). Used by the global ctrl+z "show interesting only"
// filter.
func IsDimRowColor(status string) bool
```

### 7.2 Behavior

- **Off → On**: hide every row where `IsDimRowColor(r.Status) == true`. Cursor
  resets to top of remaining rows.
- **On → Off**: restore. Cursor stays at current selected resource if it's
  still present, else top.

### 7.3 Persistence and scope

- **Per-view, per-session.** Toggle state lives on
  `ResourceListModel.attentionOnly bool`. Each list view has its own.
- Reset to off when the view is popped from the view stack (same lifecycle
  as text filter).
- Status line indicator: append `[!]` next to the filter indicator when
  active. Example: `:ec2 [filter:web] [!]`.
- The toggle does NOT bleed across resource types. Switching from `:ec2` to
  `:ct-events` starts with attentionOnly=off in the new view.

### 7.4 ec2 case — what counts as "interesting"?

For ec2 specifically: `running` is green, `pending`/`stopping` yellow,
`stopped`/`terminated` red/dim. With ctrl+z on, what does the user see?

**Decision:** ctrl+z hides **dim and default-neutral only**. Green (running)
stays visible. This matches the "show me anything that isn't routine or
dead" intent. `terminated` and `shutting-down` are dim → hidden. `stopped`
is red → visible. `running` is green → visible.

If the user later wants a stricter mode ("only red+yellow, hide green too"),
that's a future ctrl+shift+z or a tri-state toggle. Not in this spec.

### 7.5 Implementation

Piggyback on the existing `applyFilter` pipeline in
`internal/tui/views/resourcelist.go`. After the text filter runs, apply a
second pass that drops dim rows if `m.attentionOnly`. Cache invalidation
identical to text filter.

Key registration:
- `internal/tui/keys/keys.go`: new `ToggleAttentionOnly` binding,
  `ctrl+z`.
- `internal/tui/views/resourcelist.go::Update`: handle the key, flip
  `attentionOnly`, call `applySortAndFilter()`, reset cursor.

Help text: `ctrl+z   show only attention-worthy rows`.

---

## 8. New `.a9s/views/ct-events.yaml`

```yaml
list:
  V:
    key: "_ct.verb"
    width: 1
  TIME:
    key: "time"           # pre-formatted "Apr 07 17:00:59"
    width: 15
  ACTOR:
    key: "_ct.actor"
    width: 26
  ORIGIN:
    key: "_ct.origin"
    width: 7
  EVENT:
    path: "EventName"
    width: 24
  TARGET:
    key: "_ct.target"
    width: 36
  OUTCOME:
    key: "_ct.outcome"
    width: 14

detail:
  - EventId
  - EventName
  - EventTime
  - EventSource
  - Username
  - ReadOnly
  - AccessKeyId
  - Resources
  - CloudTrailEvent
```

No `color:` keys anywhere. The V column gets a real header label (`V`) so
its width=1 column lines up with the data correctly.

---

## 9. Tear-down checklist (for the implementation pass)

**Delete** (production code):
- `internal/tui/views/resourcelist.go`: `applyVerbColor`, `applyActorColor`,
  `applyOutcomeColor`, `applyOriginColor`, `ApplyCellColor`, `verbStyle`,
  `actorStyle`, `outcomeStyle`, `originStyle`, `cellStyleFor`, `cellOverrideFor`,
  `verbOverride`, `actorOverride`, `outcomeOverride`, `originOverride`.
- `internal/tui/views/table_render.go`: the per-cell override branch in
  `renderDataRow`. Replace with ec2-form: `b.WriteString(base.Render(padded))`.
- `internal/tui/styles/styles.go` `rowColorCache`: remove `ct-write` and
  `ct-read` entries. Add `ct-info` (ColTerminated), `ct-attention` (ColPending),
  `ct-danger` (ColStopped).
- `internal/config/types.go` (or wherever `ListColumn` lives): **delete the
  `Color` field entirely.** No deprecation. Verify with grep that no other
  resource YAML in `.a9s/views/` references it; if any do, strip them.

**Delete** (tests — they assert the old per-cell behavior):
- `tests/unit/views_table_render_cell_color_test.go`
- `tests/unit/views_resourcelist_outcome_failure_color_test.go`
- `tests/unit/views_resourcelist_row_tint_compose_test.go`
- `tests/unit/views_resourcelist_cursor_compose_test.go`
- `tests/unit/views_resourcelist_color_test.go` — verify whether any
  assertions there are still useful; likely fully obsolete.
- `tests/unit/aws_ct_events_status_test.go`: rewrite. The new contract is
  `ct-info`/`ct-attention`/`ct-danger`, not `ct-write`/`ct-read`.

**New tests:**
- `tests/unit/aws_ct_events_severity_test.go`: severity ladder per §1.2.
  One test per row of the precedence table (ROOT+read, ROOT+destroy,
  failed+read, sensitive-read, write, cross-account, plain read).
- `tests/unit/aws_ct_events_verb_classification_test.go`: every entry in
  the verb table from §2.1, plus the bug-fix cases (`BatchGetImage`,
  `Decrypt`, `Encrypt`, `GenerateDataKey`).
- `tests/unit/aws_ct_events_target_fallback_test.go`: one test per row of
  the fallback table in §4. (File already exists but is currently empty —
  expand it.)
- `tests/unit/aws_ct_events_format_test.go`: `formatCTTimestamp` and
  `formatCTTarget` (ARN strip + cross-account exception).
- `tests/unit/views_resourcelist_dim_filter_test.go`: ctrl+z toggle on,
  off, persistence, cursor reset.
- `tests/unit/views_resourcelist_sort_indicator_test.go`: assert exactly
  ONE column carries the ↓/↑ glyph for each sort mode, on ct-events,
  ec2, and at least one other resource.

**Update** golden files: `tests/testdata/golden/issue119/`,
`tests/testdata/golden/issue140/` will both shift because the per-cell
ANSI is gone. Regenerate with `UPDATE_GOLDEN=1`.

---

## 10. Decisions log (resolved 2026-04-08)

| # | Question | Decision |
|---|----------|----------|
| 1 | ctrl+z scope | **Global** — every list view (§7) |
| 2 | `ListColumn.Color` field | **Delete entirely**, no deprecation (§9) |
| 3 | Cross-account ARN format | `999988887777:role/Admin` (§5) |
| 4 | ACTOR cross-account format | `999988887777/alice` (§1.4) |
| 5 | `Encrypt`/`Sign`/`GenerateDataKey` verb | **R** (read), KMS use-key ops are not mutations (§2.1) |
| 6 | Sort glyph bug | **Cosmetic only**, header-decoration fix only, sort order unaffected (§6) |
