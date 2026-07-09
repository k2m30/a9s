# Related Resources — Engine Architecture

> How the detail-view RELATED panel counts related resources and decides what
> each row shows. Companion to [`related-resources.md`](related-resources.md)
> (the *Golden Contract*), which says **which** related types each resource has
> and **why**. This says **how** each row resolves and renders.
>
> Design rule for this engine: **keep it simple.** Every row is one of a small,
> fixed set of states; two tiny functions decide how a row looks; there is one
> place that owns the row data. No query languages, no typed IRs — see §7 for
> what we deliberately did not build and why.

---

## 1. The row states

Every row is exactly one of these. Nothing else. `(?)` is never shown.

| Row shows | Meaning | Clickable? |
|-----------|---------|-----------|
| `Name (N)` | Found N. | yes |
| `Name (N+)` / `Name (0+)` | Found N so far, the scan was truncated — more may exist. | yes |
| `Name (0)` dimmed | Complete scan, found nothing. | no — dead end |
| `Name` (no count) | We couldn't put a count on this row. | yes |
| `Name —` dimmed | The check errored. | no — dead end; the error shows in the error window + `!` log, **Ctrl+R** retries |
| `Name` dimmed, no count | Still checking (transient). | not yet — becomes one of the above when the check returns |

**What a click opens** — one rule, the same for every clickable row (this is the
whole of navigation; §7 does not restate it):

1. the row has found IDs → a list filtered to exactly those targets (a single
   target goes straight to its detail); else
2. the row has a server-side filter → a server-filtered list; else
3. the plain target list, to browse.

Case 3 is an honest "go look" — the only option when the row has nothing to
scope by (a truncated `(0+)` with no IDs, or a "couldn't count" row). It reaches
the full type list, not a related subset; we keep it because an operator who
can't get a count still wants to reach the list. `(0)`, error, and
still-checking rows are not clickable.

Two things to hold onto:

- **`(0)` is the only "nothing" row.** A *complete* scan found nothing. A partial
  scan that found nothing is `(0+)`, not `(0)`, and stays clickable — this is the
  rule that stops a truncated first page from lying "0".
- **Blank means "no count", not "dead end".** A blank row is still clickable
  (opens the list per the rule above) except the error row, which dims and
  dead-ends.

---

## 2. What a checker returns, and how the engine reads it

A checker reports only **raw facts** — what it found and whether it finished. It
does not decide how the row looks:

```go
type RelationFacts struct {
    FoundIDs []string          // canonical target Resource.IDs it found
    Scan     ScanState         // NotScanned | Partial | Complete
    Filter   map[string]string // optional server-side filter for the drill-in
    Err      error
}
```

**One place in the engine** turns those facts into the row — the single decider,
so two checkers can't answer the same shape two different ways:

| Facts | Row (from §1) |
|-------|---------------|
| no facts yet (seeded) | `Name` dimmed, no count — still checking |
| `Err != nil` | `Name —` dimmed — error, dead end (+ flash / `!` log) |
| `Scan = Complete`, k > 0 found | `(k)` — clickable |
| `Scan = Complete`, 0 found | `(0)` dimmed — dead end |
| `Scan = Partial`, k ≥ 0 found | `(k+)` / `(0+)` — clickable |
| `Scan = NotScanned` | blank — clickable (drill via `Filter` if set, else the plain list) |

The count is always `len(FoundIDs)` — there is no separate count field to drift
from the IDs. This derivation (today's `FormatRelatedCount` / `IsRelatedActionable`
in [`internal/resource/related.go`](../internal/resource/related.go), now fed the
raw facts instead of a checker-chosen state) is called by the TUI, the
web/controller, and the cursor alike — so a row can never look one way in the TUI
and another on the web.

---

## 3. The pipeline

On detail open, the panel behaves like Wave-2 issue enrichment — seed, dispatch,
resolve, cache:

```
open detail
  ├─ SEED     one row per registered related def, shown dimmed with no count → panel is never empty
  ├─ CACHE?   RelatedCache hit (key "type:id") → replay instantly, done
  ├─ DISPATCH one check per def
  ├─ CHECK    each checker returns RelationFacts; the engine derives the row (§2)
  ├─ MERGE    update the matching row in place (by the def's stable key, not its display label)
  ├─ STORE    write results to the session cache for next time
  └─ RENDER   §2 — every seeded row now shows (N) / (N+) / (0) / a count-less name / an error
```

Two runners share the same checker contract and the same merge and display —
they differ **only** in scheduling:

- **Headless / web** — runs checkers sequentially, returns one batch.
  Deterministic; used by the web renderer and tests.
- **TUI** — runs them concurrently (capped at 4), each row filling in as it
  finishes; 10s timeout and panic-recovery per checker.

Guarantees: the panel is never empty when a type has related defs (the seeded,
dimmed, count-less rows show right away), and every seeded row always resolves
(timeout/panic → a count-less "couldn't count" row, never stuck dim forever).

---

## 4. Writing a checker

A checker runs on every detail open, so it gets **at most one extra AWS call**
beyond the already-loaded sibling caches. It fills in `RelationFacts` — what it
found and whether it finished — and nothing else:

| Situation | Set |
|-----------|-----|
| Found the targets, looked everywhere | `FoundIDs: […], Scan: Complete` |
| Found nothing, looked everywhere | `Scan: Complete` (no IDs) |
| Found some, but the scan was truncated | `FoundIDs: […], Scan: Partial` |
| Couldn't scan here (a prerequisite lookup missed) | `Scan: NotScanned` |
| Drill-in is a server-side filter, not a local count | `Scan: NotScanned, Filter: {…}` |
| An AWS call failed | `Err: …` (aggregate per-item failures) |

The checker never picks `(N)` vs `(0)` vs `(0+)` vs blank — the engine derives
that from `Scan` + `FoundIDs` (§2). The one rule to get right: report
`Scan: Partial`, not `Complete`, whenever the scan could have missed matches —
that is what stops a truncated page from lying "0". Set `Err` on API failure so
the operator sees a flash instead of a silent wrong count. `(?)` cannot occur —
no field produces it.

Each `RelatedDef` carries a **stable key** (its own identity, not its display
label — one resource can register several defs of the same target type, e.g.
ct-events' four self-pivots). The merge and the cache key rows by that stable
key, so renaming a label or localizing it never merges the wrong row.

---

## 5. Cache

Session-scoped LRU (`internal/session/related_cache.go`, cap 500), keyed
`type:id`, holding the per-row results. Replayed on re-entry so reopening a
detail is instant. Dropped on profile/region switch; no disk persistence.

Staleness is bounded by two existing guards, not by a richer key: dispatch is
generation-stamped (a result from a superseded dispatch is discarded), and the
whole cache is dropped on profile/region switch. That covers the real cases. If a
concrete intra-session staleness bug ever appears (a sibling cache growing under
a cached row), the fix is to fold the dispatch generation into the key — one
field — not a versioned relation-signature key up front.

---

## 6. The one architectural fix: a single row store

This is the real problem worth fixing, and the fix is a **deletion, not an
abstraction**.

Today the same row data lives in **two** places, each with its own copy of the
merge, the loading-seed, and the self-pivot rule:

- controller: `DetailState.RelatedRows` + `mergeDetailRelatedRow`
- TUI: `RightColumnModel.rows` + an inline merge in `rightcolumn.go`

That is exactly the "fix it here, break it there" trap: change the merge or the
self-pivot rule in one and the other drifts.

The fix: **the controller owns the one row store; the TUI right column renders
from the controller snapshot** (the same way it already calls the two display
functions), instead of keeping its own `rows` slice and merge. Then there is one
merge, one seed, one self-pivot rule — and the two renderers can't diverge,
because there's nothing to keep in sync.

**Cursor invariant (part of the same fix).** One store is not enough on its own:
the related cursor must be **clamped to the rendered snapshot after every change**
— a row resolving, a filter narrowing the list, or a resize shrinking the
viewport. Whenever the visible row set changes, the cursor is re-clamped into
range and the viewport scrolls to keep it visible. This is what closes the
"cursor below the visible area" bug: selection is derived from the one snapshot,
never held independently of what is drawn.

That's the whole architectural change. Everything else in §1–§5 is the engine as
it already works.

---

## 7. What we deliberately did NOT build

To keep this implementable and predictable, these were considered and rejected —
they add complexity without changing what the operator sees:

- **A typed relation-query language / predicate AST** — the `map[string]string`
  filter covers every registered pivot today. If a future pivot genuinely needs
  OR / grouped predicates, add it then, for that pivot.
- **A `NavigationTarget` enum** — navigation is the single rule in §1 (found IDs
  → filter → plain list). Three cases in one place; no enum, no restating.
- **`Coverage` / `Degraded` metadata objects** — "complete vs partial" is the
  one `Truncated` bool; an errored check is the error row.
- **A structured identity schema, `RelationEdge` provenance, per-relation cache
  signatures** — the canonical `Resource.ID` and the `type:id` cache key are
  enough for the states in §1.

If a concrete bug or feature ever needs one of these, we add the smallest piece
that fixes it — not the whole framework up front.

**One tradeoff we take knowingly** (so it's a decision, not an oversight):

- **A "couldn't count" row opens the full type list** (§1, case 3), which is not
  a scoped relation view. We keep it because reaching the list is more useful
  than a dead end when no count was possible. If that ever reads as misleading,
  the fix is to make such a row non-clickable — a one-line change to the
  derivation in §2, not a navigation framework.

(The other candidate — "each checker invents its own state" — is *not* a tradeoff
we take: §2 moved that decision into the engine. The checker reports raw facts;
the engine derives the state, so checkers cannot drift.)

---

## 8. Migration note

`(?)` and the `Count: -1` sentinel are retired: unknown/deferred rows are blank
(clickable), a proven zero is `(0)` (dimmed), an error is the dimmed error row.
The per-checker `State` / `Count` / `Truncated` fields and the
`UnknownRelated` / `DeferredRelated` / `TruncatedResult` / `ErrorRelated`
constructors are replaced by `RelationFacts` (§2) plus the engine's derivation —
checkers stop choosing a state.

**Follow-up:** Golden Contract Policy rule 7 still describes the old `-1` / `(?)`
model and must be corrected to point here in the same change that finalizes this
doc.
