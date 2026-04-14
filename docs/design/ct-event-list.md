# CloudTrail Events List View — Design

> Note: references to `styles.RowColorStyle` / `IsDimRowColor` in this file are historical. Current API: `styles.ColorStyle(resource.Color)` via `ResourceTypeDef.ResolveColor(r)`. See `docs/architecture.md` Row Coloring.

Status: design spec, not implemented. No code under `internal/`, `cmd/a9s/`,
`tests/`, or `.a9s/` changes as part of this document. Source of truth for
event shapes is `docs/design/ct-taxonomy.md`; the sibling detail-view spec
is `docs/design/ct-event-detail.md`. Every field path cited below points
back to a numbered section of the taxonomy.

This document covers **only the resource list view** (the table users see
when they open `:ct-events`). The detail view that opens on `enter` is
out of scope and is already specified in `ct-event-detail.md`.

---

## 1. Engine fit analysis

### 1.1 What the list engine actually does

`internal/tui/views/resourcelist.go` is generic. For each row it:

1. Reads columns from `.a9s/views/<short>.yaml` `list:` block as a
   `map[name]ListColumn{Field, Width}` (today the file is
   `.a9s/views/ct-events.yaml`, see header dump in §1.4).
2. Resolves each `ListColumn.Field` for a `Resource` via
   `internal/fieldpath`, which walks `Resource.RawStruct` by
   reflection. Special tokens `@id` / `@name` short-circuit to
   `Resource.ID` / `Resource.Name`.
3. Renders the value as a string in a fixed-width cell. Sort and
   filter (`/` text filter, `s` cycle sort field) operate on the
   already-rendered string per column.
4. Row coloring is a single-style call: `styles.RowColorStyle(r.Status)`
   in `resourcelist.go` looks up `Resource.Status` in a static cache
   keyed by lowercased status (`palette.go` defines the colors). There
   is **one row color per row, derived from one field**.

What this engine **cannot** express today:

- Per-cell color (only per-row). The W/R glyph and the eventName both
  want their own color independent of the row tint.
- Computed fields. Every column maps to a single reflected path.
- Conditional column visibility (e.g. `REGION` only when multi-region).
- Composite values like "actor string from 12 `userIdentity` variants".
- Anything inside `Event.CloudTrailEvent`. As established in
  `ct-event-detail.md` §1.1, that field is a single `*string` JSON
  blob; `fieldpath` cannot reach inside it. The current
  `ct_events.go` fetcher confirms this — it parses the JSON in Go
  (`extractRoleNameFromCTEventJSON`) and stores the result as a flat
  `Resource.Fields["role_name"]` string before the renderer ever
  sees it. That pattern is the entire trick.

### 1.2 Two real options

| Option | Mechanism | Pros | Cons |
|---|---|---|---|
| **(a) Flatten in fetcher.** Parse `CloudTrailEvent` JSON once inside `FetchCloudTrailEventsPage`. Write computed values into `Resource.Fields` under stable keys (`_ct.actor`, `_ct.verb`, `_ct.outcome`, `_ct.target`, `_ct.origin`, `_ct.error_code`, `_ct.account_id`, `_ct.recipient_account`, `_ct.is_root`, `_ct.cross_account`, `_ct.event_category`, `_ct.event_type`, `_ct.source_ip`, `_ct.region`, `_ct.session_age_secs`, `_ct.user_agent_kind`). YAML columns reference these by `key:` instead of `path:` (a key:-only `ListColumn` already works — see `ct-events.yaml` `Event ID` row). | Zero changes to renderer. Sort/filter/search work for free on the new fields because they live in `Resource.Fields`. Uses the existing flatten-on-fetch convention from §1.1 (`role_name` already does this). Downstream related-checkers and demo fixtures keep using the same `Fields` map. | Per-cell color and conditional columns still need a small renderer hook. Adds JSON parse cost on every page (~25 events). |
| **(b) List-view branch.** Parallel to the proposed `renderCTEvent` branch in `detail.go`, add a `renderCTEventRow` branch in `resourcelist.go`'s `renderDataRow`. | Per-cell color trivial; full freedom over layout. | Couples generic list code to one resource. Sort and filter no longer work without re-implementing them per branch. Drifts from every other list view. Big code surface to test. |

### 1.3 Decision — (a) flatten in fetcher, plus a tiny renderer hook

Pick **(a)**, with one targeted concession on the renderer side:

1. **Fetcher does the work.** Extend `FetchCloudTrailEventsPage` and
   `FetchCloudTrailEventsPageFiltered` (`internal/aws/ct_events.go`)
   to parse `CloudTrailEvent` once per event (the same JSON blob the
   detail view will parse — share the helper) and populate the
   `_ct.*` keys listed in §1.2 option (a) on `Resource.Fields`. The
   existing `event_name`, `time`, `user`, `source`, `resource_*`,
   `read_only`, `role_name` keys stay for backwards compatibility
   with related-checkers and tests. New keys are additive.

2. **YAML columns address the new keys.** `.a9s/views/ct-events.yaml`
   `list:` block is rewritten to reference `key: _ct.actor` etc.
   instead of `path: Username` etc. This keeps the
   YAML-as-source-of-truth invariant for column definitions. The
   `internal/config/defaults.go` built-in defaults need the same
   rewrite so users without a custom YAML get the new layout.

3. **Per-cell color via a small render hook, not a branch.** Add a
   single optional `Color` field to `ListColumn` that names a
   classifier — one of:
   - `verb` — color by `_ct.verb` (`read|write|destructive|service|insight|ambiguous`).
   - `outcome` — green if blank/`OK`, red if non-empty error code.
   - `actor` — bright red when `_ct.is_root == "true"`, accent when
     `_ct.cross_account == "true"`, dim when service-event,
     default otherwise.
   - `none` (default) — current behavior.
   The renderer adds a `switch col.Color` in the cell formatter. No
   per-row branch on resource type, no JSON parsing in the view
   layer, no new generic column DSL. This is the smallest knob that
   buys us the W/R/D coloring users actually need.

4. **Row tint reuses existing EC2-style `Resource.Status` keys.** The
   fetcher writes a coarse status string into `Resource.Status`
   and the existing `styles.RowColorStyle` cache
   (`internal/tui/styles/styles.go:109`) colors the whole row. To
   stay consistent with every other list view in a9s (EC2, RDS,
   EKS, …), we **reuse the existing cache keys** wherever an
   equivalent semantic already exists. Mapping:

   | CT condition | `Resource.Status` value | Cache key provenance |
   |---|---|---|
   | Verb is `W` or `D` (write / destructive) | `"ct-write"` | **NEW** — foreground-only red |
   | Verb is `R`, `S`, `I`, or `N` (read / observation) | `"ct-read"` | **NEW** — foreground-only yellow |

   Two new keys are added to the `rowColorCache` init:
   - `"ct-write"` → `lipgloss.NewStyle().Foreground(ColStopped)` (red fg)
   - `"ct-read"`  → `lipgloss.NewStyle().Foreground(ColPending)` (yellow fg)

   **No background colors anywhere.** Backgrounds are unreadable
   on root accounts and small personal terminals — every cue is
   foreground-only. Errors, root identity, cross-account, and
   service events are signaled through **cell-level** color
   classifiers (ACTOR / OUTCOME), never through the row tint.

   Precedence (resolved at fetch time, highest first):
   1. Write (`ct-write`) — verb in {W, D}
   2. Read  (`ct-read`)  — verb in {R, S, I, N}

5. **No new key bindings.** Sort, filter, horizontal scroll, `tab`
   cycle stay as today. The filter is the generic substring matcher
   over `Resource.Fields`; no shortcuts or sigils.

This is the same hybrid-flatten pattern already in use for
`role_name` (`ct_events.go:117, 215`). It does not invent any new
plumbing the codebase doesn't already have.

### 1.4 Current YAML for reference

`.a9s/views/ct-events.yaml` today:

```yaml
list:
  Event ID:    { key: "@id",     width: 22 }
  Time:        { path: EventTime,   width: 22 }
  Event Name:  { path: EventName,   width: 28 }
  User:        { path: Username,    width: 24 }
  Source:      { path: EventSource, width: 28 }
  Read Only:   { path: ReadOnly,    width: 10 }
```

Total width 134 — already wider than an 80-col terminal. None of
the columns convey error, destructiveness, target, or origin. This
is the engine being underused.

---

## 2. Information model

The list answers, for each row at a glance: **WHO did WHAT to WHOM,
WHEN, with what OUTCOME and from WHERE**. The mapping is a strict
projection of the detail-view model from `ct-event-detail.md` §2.

| Concept | Detail-view source | List flatten key | Width budget | Notes |
|---|---|---|---|---|
| Verb | `eventName` (§5.6) | `_ct.verb` | 1 ch | One of `R`,`W`,`D`,`S`,`I`,`N`. Glyph only. |
| Time (absolute) | `eventTime` (taxonomy §2) | `event_time` (existing key) | 19 ch | `2006-01-02 15:04:05` UTC. Canonical a9s timestamp format — see `internal/aws/ct_events.go:96,194`, `internal/aws/rds_events.go:88`, `internal/aws/asg_activities.go:96`, `internal/aws/sfn_execution_history.go:100`. Sort is lexicographic (works because the format sorts correctly). |
| Actor | computed §2.1 of detail spec | `_ct.actor` | 26 ch | Same algorithm as detail-view §2.1. Truncated middle-elide. |
| Origin | `userAgent` (§5.5) + `sessionCredentialFromConsole` | `_ct.origin` | 7 ch | One of `Console`,`CLI`,`SDK`,`Service`,`TF`,`Boto`,`?`. |
| Event | `eventName` | `_ct.event_name` (alias of `event_name`) | 22 ch | Colored by `_ct.verb`. |
| Target | `resources[]` first non-empty (§6) → fallback per-service requestParameters → `insightDetails` / vpce / service principal (never blank — see §2.1) | `_ct.target` | 28 ch | Strip ARN prefix. Truncate middle. Navigable to underlying resource where supported (see §7). |
| Outcome | `errorCode` (§5.1) | `_ct.outcome` | 14 ch | `OK` (dim green) or `<errorCode>` (red). |
| Region | `awsRegion` | `_ct.region` (alias) | 11 ch | Hidden by default; shown when more than one distinct region in current page or via `tab`. |
| Source IP | `sourceIPAddress` | `_ct.source_ip` | 15 ch | Off by default; reachable via `tab` (horizontal scroll). |

Total **default** width (verb..outcome, no region, no IP):
1+19+26+7+22+28+14 = **117 ch** plus 7 single-space gutters = **124 ch**.
At 132 cols this fits comfortably. Below 132 the engine's existing
horizontal-scroll behavior (`hScrollOffset` in
`resourcelist.go:225-245`) drops columns left-to-right, which is
wrong for this layout — the leftmost column (verb glyph) is the most
informative. **Column drop priority** is therefore handled by
specifying *which columns may collapse* in §4d, not by editing the
generic h-scroll engine.

### 2.1 TARGET fallbacks (never blank)

For the four event categories that have no `resources[]` entry, the
fetcher computes a meaningful `_ct.target` instead of leaving it
empty:

| Category | Source | Rendered as | Example |
|---|---|---|---|
| `Insight` (taxonomy §1.3) | `insightDetails.state` + `insightDetails.insightType` + `insightContext.statistics.baseline.average` vs `insight.average` | `<eventName> ×<ratio>` | `ApiCallRateInsight ×4.2` |
| `NetworkActivity` (VPCE) (taxonomy §1.4) | top-level `vpcEndpointId` + `eventSource` service prefix | `<vpce-id> → <svc>` | `vpce-0fab12 → s3` |
| `AwsServiceEvent` | `eventSource` | service principal | `kms.amazonaws.com` |
| `Management` without resources[] (e.g. `ConsoleLogin`, `GetCallerIdentity`, `ListBuckets`) | — | `(none)` | `(none)` |

Only the last case renders `(none)`. Every other row has a concrete
target string.

---

## 3. Default columns

Numbered left-to-right.

### 3.1 The set

| # | Name | Width | Source key | Color rule | Sortable | Default? |
|---|---|---|---|---|---|---|
| 1 | (verb) | 1 | `_ct.verb` | `verb` classifier | yes (groups verbs) | yes |
| 2 | TIME | 19 | `event_time` (`2006-01-02 15:04:05`) | none (dim) | yes, lexicographic | yes |
| 3 | ACTOR | 26 | `_ct.actor` | `actor` classifier | yes | yes |
| 4 | ORIGIN | 7 | `_ct.origin` | dim if `Service`, accent if `Console` | yes | yes |
| 5 | EVENT | 22 | `event_name` | `verb` classifier | yes | yes |
| 6 | TARGET | 28 | `_ct.target` | accent + underline if navigable | yes | yes |
| 7 | OUTCOME | 14 | `_ct.outcome` | `outcome` classifier | yes | yes |
| 8 | REGION | 11 | `_ct.region` | none | yes | only when multi-region |
| 9 | SRC IP | 15 | `_ct.source_ip` | dim if `AWS Internal`, accent if a service DNS | yes | no (`tab` to reveal) |

The **W/R verb glyph** is rendered as a single colored character,
no header text — header row shows a space in column 1 to signal
"this column is just a tag". Glyphs:

```
R   read              ColDim         (#565f89)
W   write/mutating    ColPending     (#e0af68)
D   destructive       ColStopped     (#f7768e)
S   service event     ColAccent      (#7aa2f7)
I   insight           ColYAMLBool    (#bb9af7)  -- purple
?   ambiguous         ColHeaderFg    (#c0caf5)
```

### 3.2 Column header row

```
  TIME                ACTOR                      ORIGIN  EVENT                  TARGET                       OUTCOME
```

(Lipgloss `TableHeader` style — `ColAccent` bold, no underline; same
as every other a9s list view.)

### 3.3 Column ordering rationale

Verb-first because the eye lands on color. Time-second because
"how recent" is the next question. Actor-third because users scan
"who" before "what". Event then Target answers "what to whom".
Outcome anchors the right edge so failures form a vertical red
column down the page — the same scanning pattern users already use
in CloudFormation status columns (`stack-status`).

### 3.4 What we deliberately drop from the current YAML

| Dropped column | Why |
|---|---|
| Event ID (`@id`) | Unique GUID nobody types or scans. Still on the detail view header. |
| Read Only ("true"/"false") | Subsumed by the verb glyph and event coloring. |

`User`, `Event Name`, `Source`, `Time` are not dropped — they are
all replaced by their richer computed equivalents (`ACTOR`, `EVENT`
(colored), `eventSource` is folded into `ACTOR` via service events
and into `ORIGIN`, `Time` becomes relative).

---

## 4. Wireframes

All values synthetic. Account IDs `111111111111`–`999999999999`.
Tokyo Night palette colors are noted inline as `[fg]` per cell so
the preview can be cross-checked. Rendered at 120 columns unless
labeled otherwise. The frame border, header style, status bar, and
filter prompt match `internal/tui/views/resourcelist.go` and
`internal/tui/styles/styles.go`.

### 4a. Busy mixed list (default columns, 132 cols)

Wireframe is schematic — the preview at `cmd/preview/ct_event/list/`
is the authoritative rendering. Rows are in strict **time-descending
order** (newest at top), matching the default fetch order (§6.1).

```
╭─ ct-events [114] ─────────────────────────────────────────────────────────────────────────────────────────────────────────────╮
│   TIME                ACTOR                      ORIGIN  EVENT                  TARGET                       OUTCOME         │
│ D 2026-04-07 14:31:12 sso:alice@corp (AdminAcc)  Console TerminateInstances     ec2/i-0f1e2d3c4b5a69788      OK              │
│ W 2026-04-07 14:30:37 bob                        CLI     PutObject              s3/prod-logs/2026/04/07/…    FAILED AccessD… │
│ D 2026-04-07 14:30:05 ROOT                       Console PutBucketPolicy        s3/billing-archive           OK              │
│ R 2026-04-07 14:29:41 KarpenterNodeRole/k-1759   SDK     DescribeInstances      (none)                       OK              │
│ S 2026-04-07 14:28:52 ec2.amazonaws.com          Service TerminateInstanceInASG ec2/i-0a1b2c3d4e5f60718      OK              │
│ W 2026-04-07 14:27:44 terraform/CI               TF      UpdateFunctionCode     lambda/api-prod              OK              │
│ R 2026-04-07 14:25:13 bob                        CLI     ListBuckets            (none)                       OK              │
│ D 2026-04-07 14:19:02 ops-deployer/build-9821    SDK     DeleteLogGroup         logs//aws/lambda/api-prod    OK              │
│ I 2026-04-07 14:17:30 —                          —       ApiCallRateInsight     ApiCallRateInsight ×4.2      START           │
│ W 2026-04-07 14:13:11 federated:saml/dave        Browser AssumeRoleWithSAML     iam/AdminAccess              OK              │
│ R 2026-04-07 14:09:47 ReadOnlyAuditor/sess-22a   SDK     DescribeSecurityGroups vpc/sg-08feab23              OK              │
│ N 2026-04-07 14:06:21 vpce-0fab12/account-444444 VPCE    PutObject              vpce-0fab12 → s3             FAILED VpceAcc… │
│ W 2026-04-07 14:00:58 karpenter-controller/k-99  SDK     CreateFleet            ec2/eks-prod-ng              FAILED Unautho… │
│ R 2026-04-07 13:50:33 bob                        CLI     GetCallerIdentity      (none)                       OK              │
│ R 2026-04-07 13:30:12 ReadOnlyAuditor/sess-22a   SDK     DescribeStackResources cfn/billing-pipeline         OK              │
│                                                                                                                               │
│  ── m: load more (showing 25/114)                                                                                             │
╰─ /filter  s sort  tab cols  enter detail  r related  esc back  ? help ────────────────────────────────────────────────────────╯
```

Color guide for the rows above (left-to-right reading order):

| Row | Verb glyph | Event name color | Outcome color | Row tint (fg only) | Why |
|---|---|---|---|---|---|
| 1 | `D` red bold | red bold | green dim | red fg (`ct-write`) | destructive |
| 2 | `W` orange bold | orange | red bold | red fg (`ct-write`) | write — outcome cell shows the failure |
| 3 | `D` red bold | red bold | green dim | red fg (`ct-write`) | destructive — `ROOT` in ACTOR cell (bold red) is the root signal |
| 4 | `R` dim | dim | green dim | yellow fg (`ct-read`) | read |
| 5 | `S` accent bold | accent | green dim | yellow fg (`ct-read`) | service-emitted observation |
| 6 | `W` orange | orange | green dim | red fg (`ct-write`) | mutating write |
| 7 | `R` dim | dim | green dim | yellow fg (`ct-read`) | read |
| 8 | `D` red bold | red bold | green dim | red fg (`ct-write`) | destructive |
| 9 | `I` purple bold | purple | warning yellow | yellow fg (`ct-read`) | insight start |
| 10 | `W` orange | orange | green dim | red fg (`ct-write`) | federated SAML write |
| 11 | `R` dim | dim | green dim | yellow fg (`ct-read`) | read |
| 12 | `N` accent (network) | dim | red bold | yellow fg (`ct-read`) | NetworkActivity — outcome cell shows the deny |
| 13 | `W` orange | orange | red bold | red fg (`ct-write`) | write — outcome cell shows the failure |
| 14 | `R` dim | dim | green dim | yellow fg (`ct-read`) | read |
| 15 | `R` dim | dim | green dim | yellow fg (`ct-read`) | read |

Notes:

- **Row tint is binary: yellow for reads, red for writes.** No
  backgrounds. Errors, root identity, cross-account, and service
  events are conveyed by cell-level classifiers (ACTOR, OUTCOME,
  EVENT) layered on top of the binary row tint.
- The actor column applies its `actor` classifier independently of
  the row tint. Row 3's `ROOT` is rendered bright red bold inside
  an otherwise red-fg row — root actions remain unmissable through
  the bold ACTOR cell, with no background needed.
- The verb-glyph column header is intentionally blank.
- `N` is added as a sixth glyph for `NetworkActivity` events
  (taxonomy §1.4). It uses `ColAccent` to distinguish from `S`.
  Listed in §3.1 footnote.
- Truncation uses middle-elide with `…` for TARGET and end-elide
  with `…` for OUTCOME long error codes.

### 4b. Filtered to errors (`/FAILED`)

The substring `FAILED` matches `_ct.outcome` whenever it contains
an error code. The existing `FilterResources` substring matcher
already handles this — no special parser, no filter shortcuts.

```
╭─ ct-events [4 of 114, filter: FAILED] ────────────────────────────────────────────────────────────────────────────────────────╮
│   TIME                ACTOR                      ORIGIN  EVENT                  TARGET                       OUTCOME         │
│ W 2026-04-07 14:30:37 bob                        CLI     PutObject              s3/prod-logs/2026/04/07/…    FAILED AccessD… │
│ N 2026-04-07 14:06:21 vpce-0fab12/account-444444 VPCE    PutObject              vpce-0fab12 → s3             FAILED VpceAcc… │
│ W 2026-04-07 14:00:58 karpenter-controller/k-99  SDK     CreateFleet            ec2/eks-prod-ng              FAILED Unautho… │
│ W 2026-04-07 12:41:09 ops-deployer/build-9817    SDK     AttachRolePolicy       iam/role/ops-runner          FAILED AccessD… │
│                                                                                                                               │
╰─ filter: FAILED ── enter to clear ────────────────────────────────────────────────────────────────────────────────────────────╯
```

The status bar replaces the keybinding hints with the active
filter line, mirroring `resourcelist.go` filter rendering.

### 4c. Filtered to a single principal (`/bob`)

```
╭─ ct-events [3 of 114, filter: bob] ───────────────────────────────────────────────────────────────────────────────────────────╮
│   TIME                ACTOR                      ORIGIN  EVENT                  TARGET                       OUTCOME         │
│ W 2026-04-07 14:30:37 bob                        CLI     PutObject              s3/prod-logs/2026/04/07/…    FAILED AccessD… │
│ R 2026-04-07 14:25:13 bob                        CLI     ListBuckets            (none)                       OK              │
│ R 2026-04-07 13:50:33 bob                        CLI     GetCallerIdentity      (none)                       OK              │
│                                                                                                                               │
╰─ filter: bob ─────────────────────────────────────────────────────────────────────────────────────────────────────────────────╯
```

Because the substring matches `_ct.actor` it equally finds
`AssumedRole` rows whose session name contains `bob`. For
authoritative principal-pivot, prefer the **right column** in the
detail view (`r → enter`), which uses the server-side
`Username`/`AccessKeyId` lookup attribute via
`FetchCloudTrailEventsPageFiltered` — see §6.

### 4d. Narrow terminal (80 cols, drop priority)

Width below 132 cannot fit the default set. The drop order is
deterministic and **does not depend on `hScrollOffset`** — it is
applied at column-layout time before the generic engine touches
the row. Order of removal as the terminal narrows:

1. SRC IP — drops below 150 (already off by default).
2. REGION — drops below 140 (already conditional).
3. ORIGIN — drops below 115.
4. TARGET truncates to 16, then 10.
5. ACTOR truncates to 14, then 10.
6. EVENT never truncates below 14 — it is the last cell to give up
   space.
7. Below 70: TIME collapses from full timestamp to `HH:MM:SS` only.
8. Below 60: only `verb · TIME · EVENT · OUTCOME` survive.

At 80 columns (TIME truncated to `HH:MM:SS`):

```
╭─ ct-events [114] ────────────────────────────────────────────────────────────╮
│   TIME     ACTOR          EVENT            TARGET           OUTCOME          │
│ D 14:31:12 sso:alice@co…  TerminateInst…   ec2/i-0f1e…      OK               │
│ W 14:30:37 bob            PutObject        s3/prod-log…     FAILED Ac…       │
│ D 14:30:05 ROOT           PutBucketPolicy  s3/billing-a…    OK               │
│ R 14:29:41 KarpenterNod…  DescribeInsta…   (none)           OK               │
│ S 14:28:52 ec2.amazonaw…  TerminateInst…   ec2/i-0a1b…      OK               │
╰─ /filter  s sort  tab cols  enter detail  esc back  ? help ──────────────────╯
```

ORIGIN dropped, TIME/TARGET/ACTOR/EVENT truncated. Verb glyph and
OUTCOME are preserved at all widths above 60.

---

## 5. Row coloring rules

The row tint is derived once in the fetcher and stored in
`Resource.Status`. The list renderer's existing
`styles.RowColorStyle` lookup (`internal/tui/styles/styles.go:47`)
paints the row. The redesign uses a **binary, foreground-only**
row tint based on the verb classifier — yellow for reads, red for
writes — and adds two new ct-specific keys to the cache.

**No background colors anywhere in the row tint.** Backgrounds
made the list unmanageable on root accounts and small personal
terminals. Every other dimension (errors, root identity,
cross-account, service events) is conveyed by **cell-level**
classifiers (ACTOR / OUTCOME / EVENT), not by the row tint.

| Condition | `Resource.Status` | Cache key source | Color | Precedence |
|---|---|---|---|---|
| Verb in {W, D} | `ct-write` | **NEW** in `rowColorCache` init | `ColStopped` red fg | 1 (higher) |
| Verb in {R, S, I, N} | `ct-read` | **NEW** in `rowColorCache` init | `ColPending` yellow fg | 2 (lower) |

Precedence is trivial: the verb classifier runs once at fetch
time, and the result is mapped to either `ct-write` or `ct-read`.
Errors, root, cross-account, and service identity are layered on
top via the cell-level classifiers in §8a (ACTOR, OUTCOME,
EVENT), so no precedence stack is needed for the row tint itself.

The two new cache entries to add:

```go
"ct-write": lipgloss.NewStyle().Foreground(ColStopped),  // red fg, no bg
"ct-read":  lipgloss.NewStyle().Foreground(ColPending),  // yellow fg, no bg
```

Both are foreground-only. No background, no bold — bold is
reserved for cell-level classifiers (the ROOT actor, destructive
verb glyph, failed outcome) so they remain visible on top of the
binary row tint. No other new color tokens introduced.

---

## 6. Sort, filter, search

### 6.1 Sort fields (`s` cycles) and fetch order

**Fetch order is TIME desc (newest first).** This is the default
ordering of the CloudTrail `LookupEvents` API — the SDK returns
events in reverse chronological order by `EventTime`, and
`FetchCloudTrailEventsPage` / `FetchCloudTrailEventsPageFiltered`
in `internal/aws/ct_events.go` do not reverse the slice. The
fetcher SHALL preserve that order when constructing the
`[]resource.Resource` it returns.

**List default sort is also TIME desc**, overriding the generic
name-ASC default. This is a deliberate, documented inconsistency
with every other a9s list view (EC2, RDS, S3, …) which sort by
name ascending. The justification: CloudTrail events are
fundamentally a time series — "what happened most recently" is
always the first question users ask. Sorting by name would be
meaningless (event names repeat constantly and convey no ordering).

Because TIME is stored as the canonical `2006-01-02 15:04:05`
string in `Resource.Fields["event_time"]`, lexicographic string
sort produces the correct chronological order. No separate
`event_time_unix` sidecar field is needed.

Order of cycling on `s`:

1. TIME (default, desc)
2. EVENT (asc)
3. ACTOR (asc)
4. OUTCOME (asc — errors group together)
5. TARGET (asc)

`s` is the existing key (`keys.go` `Sort`). No new binding.

### 6.2 Filter (`/`)

The existing `FilterResources` substring matcher checks every
`Resource.Fields` value plus `Resource.Name`/`ID`. Because the
fetcher writes `_ct.verb`, `_ct.outcome`, `_ct.origin`,
`_ct.is_root`, `_ct.cross_account`, `_ct.event_category`, and
`_ct.actor` into `Fields`, standard substring filtering Just Works
over any of those tokens. Examples that succeed with no new
parser:

- `/FAILED` — matches outcome
- `/bob` — matches actor substring
- `/DeleteLogGroup` — matches event_name substring
- `/i-0f1e2d3c` — matches target ARN substring

**No filter shortcuts, sigils, or prefix parsers are introduced.**
The user types literal substrings; the generic matcher handles
them. Filter is **client-side over the loaded page**, which is the
same limitation every other paginated list view in a9s has —
`resourcelist.go:441` already shows the "filter applies to loaded
data only" hint when paginated.

For **server-side** filtering by principal, the filtered fetcher
already exists: `FetchCloudTrailEventsPageFiltered` accepts
`Username`, `AccessKeyId`, `EventName`, `EventSource`,
`ResourceName`, `ResourceType`, `ReadOnly` as `LookupAttribute`
keys. The right way to invoke it from the list view is via the
**related navigation path**: from a detail view, press `r` and
pick "Events by this user" — which is exactly the pattern already
implemented for IAM User → ct-events. No new key binding needed
for the list view itself.

### 6.3 Search

`/` is filter, not search-highlight. a9s does not currently have
a separate "search-highlight in place" mode for list views, and
introducing one for ct-events alone would be inconsistent. Out of
scope.

---

## 7. Right column behavior

The right column (RELATED) is currently a **detail-view-only**
construct in a9s. `internal/tui/views/rightcolumn.go` is consumed
by `DetailModel`, not by `ResourceListModel`. No list view in the
codebase shows the related panel for the highlighted row.

**Decision:** keep that convention. The list view does **not**
show a right column. Reasons:

1. Consistency. Doing it for ct-events alone breaks the mental
   model "list = scan, detail = inspect / pivot".
2. Cost. The related-checkers in
   `ct-event-detail.md` §7b.10 require parsing
   `userIdentity` and walking related caches; doing that on every
   cursor move in a 25-row list page would either be slow or
   require a per-row cache that doesn't exist today.
3. The pivot use case is already covered by the
   detail-view right column — `enter` to drill, `r` to expand,
   `enter` on a related row to navigate.

If a future story wants list-row pivots, the cleanest path is the
detail-view right column (already designed in
`ct-event-detail.md` §7b.10), not a new list-view panel.

---

## 8. Open questions — resolved

All six questions from the prior revision are closed:

1. **Relative time cadence.** RESOLVED — no relative time. TIME
   column shows the absolute `2006-01-02 15:04:05` string from the
   canonical a9s timestamp format. See §2 and §3.1.
2. **`N` glyph for NetworkActivity.** RESOLVED — kept on the list
   view. Documented in §3.1 and in the Help additions below.
3. **Row tint vs. cell color collision.** RESOLVED — binary
   foreground-only row tint: `ct-write` (red) for verbs W/D,
   `ct-read` (yellow) for verbs R/S/I/N. **No background colors
   anywhere** — backgrounds were unmanageable on root accounts and
   small terminals. Errors, root, cross-account, and service
   events are signaled exclusively through cell-level classifiers
   on ACTOR / OUTCOME / EVENT. See §5.
4. **Target for Insight / NetworkActivity.** RESOLVED — targets
   are never blank. Insight renders `<eventName> ×<ratio>`;
   NetworkActivity renders `<vpce-id> → <service>`;
   AwsServiceEvent renders the service principal. See §2.1.
5. **Default sort direction.** RESOLVED — TIME desc. The
   inconsistency with name-ASC lists is intentional and justified
   in §6.1.
6. **Filter shortcuts.** RESOLVED — dropped. Substring filter
   only. See §6.2.

---

## 8a. Help screen additions

The CT events list introduces glyphs and status colors that are
not documented anywhere else in the app. The help view
(`internal/tui/views/help.go`) currently renders key bindings only;
this design requires a new **"CloudTrail Events legend"** section,
scoped to `HelpFromResourceList` when the active resource type is
`ct-events` (and `HelpFromResourceListPaginated` equivalently).

The coder implementing help.go SHALL add the following legend,
grouped into two subsections. Colors are named using the same
`styles.Col*` tokens the help view already imports from
`internal/tui/styles/palette.go`.

### Verb glyphs (column 1 of the list)

| Glyph | Meaning | Style | Color token |
|---|---|---|---|
| `R` | Read — `Describe*`, `Get*`, `List*`, `Head*` | dim | `ColDim` (`#565f89`) |
| `W` | Write / mutating — `Create*`, `Put*`, `Update*`, `Attach*`, `Modify*` | bold | `ColYAMLNum` orange (`#ff9e64`) |
| `D` | Destructive — `Delete*`, `Terminate*`, `Revoke*`, `Detach*` | bold | `ColError` (`#f7768e`) |
| `S` | Service event — `eventType == AwsServiceEvent` | bold | `ColAccent` (`#7aa2f7`) |
| `I` | Insight event — `eventCategory == Insight` | bold | `ColYAMLBool` purple (`#bb9af7`) |
| `N` | NetworkActivity — `eventCategory == NetworkActivity` | bold | `ColAccent` (`#7aa2f7`) |
| `?` | Ambiguous (no classifier match) | plain | `ColHeaderFg` (`#c0caf5`) |

### Row status colors (via `Resource.Status` → `RowColorStyle`)

Binary, foreground-only. **No backgrounds.** Errors, root,
cross-account, and service events are signaled by the cell-level
classifiers below, not by the row tint.

| Row appearance | Meaning | `Resource.Status` value |
|---|---|---|
| Red fg | Verb is `W` (write) or `D` (destructive) | `ct-write` |
| Yellow fg | Verb is `R` (read), `S` (service), `I` (insight), or `N` (network) | `ct-read` |

### Actor and outcome cell colors

| Cell | Appearance | Meaning |
|---|---|---|
| ACTOR | bold red | literal `ROOT` (dominates any row tint) |
| ACTOR | yellow | cross-account principal |
| ACTOR | dim | AWS service principal |
| OUTCOME | green | `OK` |
| OUTCOME | bold red | `FAILED <errorCode>` |
| OUTCOME | yellow | `START` / `END` (Insight state transitions) |

Implementation note for the coder: this is pure `help.go` content.
No changes to `keys.go`, no new message types. The legend is a
block of `lipgloss`-styled rows concatenated into the existing
help viewport when `m.context == HelpFromResourceList` and the
calling resource short name is `ct-events`. See the "context" plumbing
already used for `HelpFromSecretsList` as the pattern to mirror.

---

## 9. Files this design implies (NOT part of this PR)

| File | Change |
|---|---|
| `internal/aws/ct_events.go` | Parse `CloudTrailEvent` JSON once per event; write `_ct.*` keys to `Resource.Fields`; set `Resource.Status` to one of `ct-write` (verb W/D) or `ct-read` (verb R/S/I/N) per §5. Preserve LookupEvents newest-first order. Compute TARGET fallbacks per §2.1. Share the parser with the detail-view branch from `ct-event-detail.md`. |
| `internal/config/types.go` | Add `Color string` field to `ListColumn`. Backwards-compatible (zero value = no classifier). |
| `internal/tui/views/resourcelist.go` | In the cell formatter, switch on `col.Color` and apply one of the four classifiers. ~30 lines, generic, not ct-events-specific. Default sort override: when resource short name is `ct-events`, initial sort is TIME desc. |
| `internal/tui/styles/styles.go` | Add **two** new foreground-only keys to `rowColorCache`: `ct-write` (`ColStopped` red fg) and `ct-read` (`ColPending` yellow fg). No backgrounds, no bold. |
| `internal/tui/views/help.go` | Add CloudTrail Events legend block per §8a when context is `HelpFromResourceList*` and the calling resource type is `ct-events`. |
| `.a9s/views/ct-events.yaml` | Rewrite `list:` block per §3. |
| `internal/config/defaults.go` | Mirror the YAML rewrite for users without a custom config. |
| `cmd/preview/ct_event/list/main.go` | Preview included in this PR (see below). |

No changes to `app.go`, `messages.go`, or any other view.

---

## 10. Preview

`cmd/preview/ct_event/list_main.go` renders the four wireframes
above using the same Tokyo Night palette and the same Lipgloss
primitives the real list view will use. Run with:

```
go run ./cmd/preview/ct_event/list_main.go
```

The preview is a sibling to `cmd/preview/ct_event/main.go` (the
detail-view preview); both compile in the same package. They are
separated into different `main` packages by directory because Go
allows only one `main` per package — see file layout in §11.

## 11. File layout for preview

```
cmd/preview/
  ct_event/
    main.go        # detail view preview (existing)
    list/
      main.go      # NEW — list view preview (this design)
```

Build verification:

```
go build ./cmd/preview/ct_event/
go build ./cmd/preview/ct_event/list/
```

Both must succeed.
