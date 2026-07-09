# Related Resources — Engine Architecture

> How the detail-view RELATED panel counts related resources and decides what
> each row **shows** and **does on Enter**. Companion to
> [`related-resources.md`](related-resources.md) (the *Golden Contract*): which
> pivots each type has and why. This describes the actual committed engine.

---

## 1. The row states

Every row is exactly one of these. `(?)` is never shown.

| Row shows | Meaning | Enter |
|-----------|---------|-------|
| `Name (N)` | Found N (complete scan). | opens the N targets (N=1 → straight to its detail) |
| `Name (N+)` / `Name (0+)` | Found N so far, the target list is **truncated** — more may exist. | opens the found targets / population; the list shows "m for more" |
| `Name (0)` dimmed | Complete scan, found nothing. | — (dead end, cursor skips) |
| `Name` (no count) | Couldn't count (Unknown) or navigates via a server-side filter (Deferred). | drills in |
| `Name —` dimmed | The check errored. | — (dead end); the error shows in the error window + `!` log, **Ctrl+R** retries |
| `Name` dimmed, no count | Still checking (transient). | — (resolves into one of the above) |

The **"+"** means only "the target list is truncated, so after Enter there is
*m for more*". **`0+` and `10+` are the same case** (N found so far, list
truncated), not two — the number is how many were found, the `+` is the
truncation marker. There is no separate "zero" result and no "approximate": a
truncated scan sets `Truncated` for any N via one helper (`relatedResultTrunc`).

---

## 2. What a checker returns

A checker returns one value:

```go
type RelatedCheckResult struct {
    TargetType  string
    State       RelatedRowState   // Resolved | Loading | Error | Unknown | Deferred
    Count       int               // authoritative when Resolved (== len(ResourceIDs))
    Truncated   bool              // the target scan hit a truncated page → "(N+)"
    ResourceIDs []string          // the found targets — drives the drill-in
    FetchFilter map[string]string // server-side filter for a Deferred drill-in
    Err         error             // non-nil → the error row (via EffectiveState)
}
```

Two pure functions in
[`internal/resource/related.go`](../internal/resource/related.go) are the **only**
deciders of how a row looks — called by the TUI right column, the
controller/web `RelatedBlock`, and the cursor-skip alike, so the renderers can
never drift:

- `FormatRelatedCount(state, count, truncated)` → the badge: `(N)`, `(N+)`, or
  blank. Only a `Resolved` result gets a number; `Truncated` adds the `+`.
- `IsRelatedActionable(state, count, truncated)` → clickable or dead end:
  `Loading` and `Error` are not clickable; a `Resolved` `(0)` (count 0, **not**
  truncated) is not clickable; everything else — including any `(N+)`/`(0+)` and
  blank Unknown/Deferred rows — is.

Reverse-scan checkers set the truncation flag through
`relatedResultTrunc(target, ids, truncated)`, which carries it uniformly for any
count (so a truncated scan that found 10 shows `(10+)`, and one that found 0
shows `(0+)` — same path).

---

## 3. Navigation — one rule

Enter on a clickable row:

1. the row has **ResourceIDs** → a list filtered to exactly those targets
   (`N == 1` → straight to the target's detail); else
2. the row has a **FetchFilter** → a server-side filtered fetch; else
3. **neither** (a scoreless row — a blank Unknown row, or a `(0+)` that found
   nothing) → **resolve in place**: it re-dispatches the source resource's
   related checks so the row firms up to its real count on the detail, WITHOUT
   opening the plain unfiltered target list ("goes to all"). Both the headless
   controller (`handleActionRelatedSelect`) and the live TUI Enter path
   (`app_stack.go`) do this.

`(0)`, error, and still-checking rows are not clickable — the cursor skips them.

---

## 4. The pipeline

On detail open, the panel behaves like Wave-2 issue enrichment:

```
open detail
  ├─ SEED     one dimmed, count-less row per registered def → panel is never empty
  ├─ CACHE?   RelatedCache hit (key "type:id") → replay instantly, done
  ├─ DISPATCH one check per def
  ├─ CHECK    each checker returns a RelatedCheckResult (Err → Error via EffectiveState)
  ├─ MERGE    update the matching row in place (by the def's display key)
  ├─ STORE    write results to the session cache for next time
  └─ RENDER   §2 — every seeded row now shows (N)/(N+)/(0)/blank/error
```

Two runners share the same checker contract, merge, and display — they differ
only in scheduling: **headless/web** runs checkers sequentially (deterministic);
the **TUI** runs them concurrently (capped at 4, 10s timeout + panic-recovery per
checker). The panel is never empty when a type has pivots, and every seeded row
always resolves.

---

## 5. Cache

Session-scoped LRU (`internal/session/related_cache.go`, cap 500), keyed
`type:id`, holding the per-row results and replayed on re-entry so reopening a
detail is instant. Dropped on profile/region switch; no disk persistence.

---

## 6. Navigation reads one row — the controller's

Enter/click navigation resolves the focused row from **`DetailState.RelatedRows`**
(via `SelectedRelatedRow` / `focusedRelatedRow`), and that row carries the
`ResourceIDs`/`FetchFilter` the drill needs. The live detail render is
controller-sourced too (`RenderDetail(body.Related)`), so what the badge shows
and what Enter navigates read the **same** row — they cannot disagree on whether
a row is navigable.

This was verified by instrumentation after a "renders `(N)` but Enter dead-ends"
report: the controller row already held the IDs. The dead-end was **not** a
store divergence — its two real causes were a checker emitting the wrong id
(an assumed-role *session* name instead of the role) and `NavigationKindDetail`
silently no-op'ing on a cache miss instead of falling back to a by-ID fetch.
Both are fixed (see §7); the ct-event role pivot drills into the role detail.

`RightColumnModel` remains as the related-panel **widget** (cursor movement,
filter typing); its row slice is interaction state, not the navigation source of
truth. Folding that residual cursor/filter state into the controller is a
code-structure cleanup with no known correctness impact — the navigation
contract above already reads a single owned row.

---

## 7. Notes

- **`(?)` and the `Count: -1` sentinel are retired.** Unknown/Deferred rows are
  blank (clickable), a proven zero is `(0)` (dimmed), an error is the dimmed `—`
  row. There is no `ApproximateZero` constructor.
- **IAM policy lazy-add** (drilling into an attached-policy pivot) resolves names
  via `ListPolicies(Scope=Local)` for customer-managed plus one `GetPolicy` per
  requested name for AWS-managed — never `ListPolicies(Scope=All)`, whose ~1000+
  catalog timed out even on empty accounts.
- **CloudTrail → IAM Role pivot** resolves the *target* role of an AssumeRole\*
  event from `requestParameters.roleArn`; `roleNameFromARN` extracts the role
  from an STS assumed-role ARN (`assumed-role/<role>/<session>` → `<role>`),
  never the trailing session name. `NavigationKindDetail` falls back to a by-ID
  fetch when the target isn't in the adapter cache (e.g. a lazily-added role),
  so the drill lands on the role detail instead of no-op'ing.
- **Golden Contract rule 7** points here for how a checker result renders.
