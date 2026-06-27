# God-Node Analysis

Findings from the graphify knowledge graph (`graphify-out/`, built 2026-06-27 over
the full corpus: 1,374 code files via AST + 570 docs / 2 images via semantic
extraction). A **god node** is a vertex with unusually high degree — the most
connected symbols in the codebase, i.e. the things almost everything else
touches. They are where coupling concentrates and where a single change ripples
furthest.

## Method & a caveat on the degree numbers

Degree comes from graphify's combined AST + semantic graph. Go call edges are
resolved by the AST extractor; when several functions share a common name (e.g.
`Default`, `relatedResult`), the extractor links call sites to a definition with
**INFERRED** confidence, which inflates raw degree. So the graph degree below is
the *attention signal*, not a literal call count. Each row is cross-checked
against a real file-level reuse count (`grep -rln '<fn>(' …`) to separate true
hubs from name-collision artifacts.

## Summary table

| # | Symbol | Location | Graph degree | Real reuse (files) | Kind | Verdict |
|---|--------|----------|-------------:|-------------------:|------|---------|
| 1 | `rootApplyMsg()` | `tests/unit/tui_root_test.go:25` | 778 | 97 | Test helper | Harness hub — consolidate |
| 2 | `rootViewContent()` | `tests/unit/tui_root_test.go:31` | 545 | 63 | Test helper | Harness hub — consolidate |
| 3 | `newRootSizedModel()` | `tests/unit/tui_root_test.go:18` | 542 | 57 | Test helper | Harness hub — consolidate |
| 4 | `Default()` | `internal/tui/keys/keys.go:74` | 531 | 114 | Production | Healthy — single source of truth |
| 5 | `relatedResult()` | `internal/aws/ec2_related.go:382` | 514 | 83 | Production | Healthy primitive — **misplaced/misnamed** |
| 6 | `ensureNoColor()` | `tests/unit/helpers_external_test.go:41` | 363 | 23 | Test helper | Harness hub — consolidate |
| 7 | `newDetailModel()` | `tests/unit/helpers_external_test.go:69` | 318 | 9 | Test helper | Harness hub — consolidate |
| 8 | `buildResource()` | `tests/unit/helpers_external_test.go:95` | 299 | 8 | Test helper | Harness hub — consolidate |
| 9 | `FindResourceType()` | `internal/resource/types.go:43` | 282 | 142 | Production | Healthy — registry facade |
| 10 | `ApproximateZero()` | `internal/resource/related.go:209` | 254 | 76 | Production | Healthy primitive |

**Headline:** 6 of the top 10 god nodes are **test helpers**, concentrated in just
**two** files (`tui_root_test.go`, `helpers_external_test.go`). The 4 production
god nodes are all legitimate single-source-of-truth registries or shared
primitives — high degree there is by design and matches the project's stated
architecture principles. **The improvement leverage is in the test suite, not the
app.**

## Per-node findings

### 1. `rootApplyMsg()` — the root-model test pump

`tests/unit/tui_root_test.go:25`. A 3-line helper that sends one message through
`Model.Update` and re-asserts the `tea.Model` back to `tui.Model`:

```go
func rootApplyMsg(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
    newM, cmd := m.Update(msg)
    return newM.(tui.Model), cmd
}
```

It is the single most-connected node in the whole codebase, yet it is a test
helper, not production code. Its 778 edges are almost entirely
`<-- *_test.go [calls]`. The companion `rootKeyPress`/`rootSpecialKey` helpers in
the same file build synthetic key events. The production dispatch it wraps
(`internal/tui/app.go:192`, `Model.Update`) is by contrast a clean, flat typed
`switch` that delegates per-message — no production god function exists. **This
node is a test-suite finding, and it is the clearest example of the harness
concentration described below.**

Suggestion: covered by **Recommendation A** (consolidate the harness). Also note
the exact duplicate `applyMsg` (a second "send one message through Update"
helper) living in another test file — collapse the two.

### 2. `rootViewContent()` — the root-model render helper

`tests/unit/tui_root_test.go:31`. One-liner: `return m.View().Content`. Used in
63 test files to grab rendered output for assertions. Same harness file as #1/#3.
Part of **Recommendation A**.

### 3. `newRootSizedModel()` — the root-model builder

`tests/unit/tui_root_test.go:18`. Builds a `tui.New(...)` model and pumps a
`WindowSizeMsg{80,40}` so `View()` renders. Used in 57 files. It is one of **at
least nine** competing model/builder helpers across the suite (see
`setupDemoApp`, `newDemoColdCacheApp`, `buildModelWithMockClients`,
`setupEC2ListWith{Cache,CompleteCache,TruncatedCache}`,
`setupEC2DetailWithResults[Narrow]`), plus 63 test files that call `tui.New(`
inline with no helper at all. Part of **Recommendation A**.

### 4. `Default()` — the key-binding registry (PRODUCTION, healthy)

`internal/tui/keys/keys.go:74`. Returns the full `keys.Map` of every key binding
in the app (navigation, command/filter modes, child-view triggers, sort columns).
Called by essentially every view constructor (114 files). High degree here is
**correct and desirable** — it is the literal embodiment of the "key bindings in
`keys/keys.go` are the single source of truth" principle. No change recommended.

Minor observation: because every view pulls the *entire* `Map`, adding a binding
touches one place (good) but every view sees every binding (no per-view scoping).
That is an intentional trade-off, not a defect. Leave as is.

### 5. `relatedResult()` — related-check result constructor (PRODUCTION, misplaced)

`internal/aws/ec2_related.go:382`. A generic helper that dedups + sorts resource
IDs and returns a `resource.RelatedCheckResult`:

```go
func relatedResult(target string, ids []string) resource.RelatedCheckResult { … }
```

It is called by the large family of `check*()` related-resource checker functions
(**551** `func check…` definitions across `internal/aws/*_related.go`), making it
one of the busiest production primitives (83 files). The logic is fine and the
fan-in is by design — the related-resources subsystem is intentionally a
hub-and-spoke around shared primitives.

**Finding (real smell):** this generic, package-wide helper lives in a file named
`ec2_related.go` and is named `relatedResult`, even though it serves *every*
resource type's checkers, not EC2's. Its natural sibling — `ApproximateZero` /
`UnknownRelated` (#10) — lives correctly in `internal/resource/related.go`.

Suggestion (**Recommendation B**): move `relatedResult` (and any other
ec2-named-but-generic related helpers) into a neutral file such as
`internal/aws/related_common.go`, or promote it next to `ApproximateZero` /
`UnknownRelated` if it can be exported without a circular import. This is a
locality/naming fix only — no behavior change — but it removes a real
"why is the universal helper in the EC2 file?" papercut for every checker author.

### 6. `ensureNoColor()` — NO_COLOR test fixture

`tests/unit/helpers_external_test.go:41`. Forces deterministic, color-free
rendering for golden/string assertions; used in 23 files. This file
(`helpers_external_test.go`) is the **second** harness hub alongside
`tui_root_test.go`. Part of **Recommendation A**.

### 7. `newDetailModel()` — detail-view test builder

`tests/unit/helpers_external_test.go:69`. Builds a `views.DetailModel` from a
resource + type + config; used in 9 files. The detail-view analogue of
`newRootSizedModel`. Part of **Recommendation A**.

### 8. `buildResource()` — resource fixture builder

`tests/unit/helpers_external_test.go:95`. Constructs a `resource.Resource` from
id/name/raw-struct for tests; used in 8 files, alongside the variant
`buildResourceWithFields`. Part of **Recommendation A**.

### 9. `FindResourceType()` — resource registry facade (PRODUCTION, healthy)

`internal/resource/types.go:43`. A one-line facade over the catalog:

```go
func FindResourceType(name string) *ResourceTypeDef { return catalog.Find(name) }
```

The most-reused production symbol by file count (142 files): `main()`,
`handleNavigate`, `handleRelatedNavigate`, list/sort model builders, and the
related-drill machinery all resolve types through it. This is exactly what a
registry facade should look like — a single, stable lookup point over the
declarative catalog. No change recommended; the thin indirection is the seam that
lets the catalog be generated/swapped without touching callers.

### 10. `ApproximateZero()` — related-check sentinel (PRODUCTION, healthy)

`internal/resource/related.go:209`. A trivial constructor returning a
`RelatedCheckResult{Count:0, Approximate:true}` ("scanned the cache, found 0, but
more may exist"). Called by the same `check*()` checker family as #5 (76 files).
It sits correctly in the shared `internal/resource/related.go` next to its
siblings `UnknownRelated` (renders "?") and `NoopChecker`, which are well
documented. This is the model for where #5 (`relatedResult`) should live. No
change recommended.

## Cross-cutting themes

1. **The test harness is the real coupling hot spot.** Six of ten god nodes are
   test helpers, split across two files with overlapping, ad-hoc primitives:
   - send-one-message: `rootApplyMsg` **and** `applyMsg` (duplicate)
   - build-a-model: `newRootSizedModel`, `setupDemoApp`, `newDemoColdCacheApp`,
     `buildModelWithMockClients`, `setupEC2ListWith*`, `setupEC2DetailWith*`
     (≥9 builders) — plus 63 files calling `tui.New(` inline
   - drain/extract cmds: `drainCmds`, `extractMsg`
   - render/strip: `rootViewContent`, `ensureNoColor`, `stripAnsi`/`stripANSI`

   Any change to `Model.Update`/`Model.View` signatures (note the repeated
   `newM.(tui.Model)` assertion) ripples across ~100 files with no canonical
   entry point.

2. **Production hubs are healthy.** The 4 production god nodes are all
   single-source-of-truth registries (`Default`, `FindResourceType`) or shared
   related-check primitives (`relatedResult`, `ApproximateZero`). High degree
   there is the *intended* architecture, not debt. The graph effectively
   validates the project's "single source of truth" principle.

3. **The related-resources subsystem is the production center of gravity.** Two of
   the four production god nodes (`relatedResult`, `ApproximateZero`) plus the
   biggest cross-community *bridge* in the wider graph (`GetRelated()`,
   `internal/resource/related.go`, betweenness ≈ 0.27) all belong to it, fed by
   551 `check*` functions. This is why the project already gates it behind
   `docs/related-resources.md` as a contract — the graph confirms that decision
   and says this is where the strongest invariant tests and the most careful
   review of fan-out growth belong.

## Recommendations (prioritized)

### A. Consolidate the test harness into one `tuitest` kit (highest leverage)

Introduce a single canonical test-support file/package exposing the primitives
exactly once, and migrate callers to it:

- `Build(opts…) Model` — replaces `newRootSizedModel`, `setupDemoApp`,
  `newDemoColdCacheApp`, `buildModelWithMockClients`, and the inline `tui.New(`
  calls.
- `Step(m, msg) (Model, Cmd)` — replaces `rootApplyMsg` **and** `applyMsg`
  (collapse the duplicate first; it is a 5-minute win).
- `Render(m) string` / `NoColor(t)` — replaces `rootViewContent`,
  `ensureNoColor`, `stripAnsi`/`stripANSI`.
- `Drain(...)` / `Extract(...)` — unify `drainCmds` and `extractMsg`.
- Detail-view equivalents: fold `newDetailModel`/`buildResource` into the same
  kit so the detail harness stops being a second hub.

Payoff: one place to update on `Update`/`View` signature changes; typed helpers
reduce the AST INFERRED-edge ambiguity that inflated these god-node degrees. Do
this under the project's TDD split (this is a test-only change) and run
`make ready-to-push` before any push.

**Correction (2026-06-27): `tests/unit` is two packages, not one.** A later
manifest pass found `tests/unit` is split across **`package unit`** (internal,
~511 files) and **`package unit_test`** (external, ~153 files). External test
files cannot see unexported helpers in `package unit`, which is *why* the harness
is duplicated across the boundary (`stripANSI`/`stripAnsi`, `rootApplyMsg`/
`applyMsg`, `ensureNoColor` per package). So "collapse to one import" is not free:
the only way to share logic across the boundary is a shared, exported helper
package both sides import. The remediation below does this with thin forwarders so
the ~150 existing call sites stay unchanged. A full rename/qualify migration of
every call site is explicitly **not** pursued — it is cosmetic churn that does not
even lower the god-node degree (the replacement primitive just becomes the new
hub).

### B. Relocate/rename the misplaced related primitive (small, real)

Move `relatedResult` out of `internal/aws/ec2_related.go` into a neutral
`internal/aws/related_common.go` (or promote it beside
`resource.ApproximateZero`/`UnknownRelated`). Pure locality/naming fix, no
behavior change. Removes a recurring "why is the universal helper in the EC2
file?" papercut for the 551 checker authors.

### C. Leave the production registries alone; invest review there instead

`Default`, `FindResourceType`, `relatedResult`, and `ApproximateZero` are healthy
hubs. Do **not** "refactor to reduce coupling" — their centralization is the
design. Instead, treat the related-resources subsystem (theme #3) as the place
that earns the deepest invariant tests and the closest review of new pivots, per
the existing `docs/related-resources.md` contract.

## Remediation Plan & Tasks (2026-06-27)

No deferring: every actionable item below is being executed in this pass.
Recommendation C is "leave alone," so it has no tasks.

| Task | Rec | Scope | Owner | Status |
|------|-----|-------|-------|--------|
| **B** — relocate generic related helpers | B | Moved `relatedResult` + `assertStruct` from `internal/aws/ec2_related.go` into new `internal/aws/related_common.go` (same `package aws`, zero call-site change). EC2-specific helpers and `tagValue` left in place. | `a9s-coder` | ✅ done |
| **A1** — shared `tuitest` kit | A | Created `tests/unit/tuitest/tuitest.go` exporting `Step`, `StepModel`, `Render`, `Sized`, `StripANSI`, `Width`, `NoColor` — the canonical logic, once. | `a9s-qa` | ✅ done |
| **A2** — forward the duplicates | A | `rootApplyMsg`/`applyMsg`, `rootViewContent`, `newRootSizedModel`, `stripANSI`/`stripAnsi`, `lipglossWidth`, `ensureNoColor`/`setupNoColor` are now thin forwarders over `tuitest`; the cross-package `ansiRegex` dup is gone. Names unchanged → ~150 call sites untouched. | `a9s-qa` | ✅ done |
| **C** — production registries | C | `Default`, `FindResourceType`, `relatedResult`, `ApproximateZero` are healthy hubs by design. No change. | — | n/a (no-op) |

Verified on the integrated change: `make build`, `make test` (all packages incl. `tests/integration`), and `make lint` (0 issues) all green.

Note (A2 nuance): `ansiRe` in `package unit` was **not** deleted — ~60 `qa_*search*` files reference it directly (not via `stripANSI`), so the "delete if unused" condition wasn't met. `ansiRegex` in `package unit_test` was removed (only `stripAnsi` used it).

### Outcome: the degree metric is intentionally unchanged

After `graphify update .`, the God Nodes ranking is essentially identical
(`rootApplyMsg` still #1 at ~779 edges; `relatedResult` still 514). **This is the
correct result, not a failed refactor.** A god node's degree is its fan-in; the
remediation deliberately preserved every public helper name (forwarders) so the
~150 call sites stay put, which means the call graph — and therefore the degree —
is unchanged. What changed is the *real* debt the graph was pointing at:

- harness **logic** is now single-sourced in `tests/unit/tuitest` (was duplicated
  across the `unit` / `unit_test` boundary);
- the cross-package `ansiRegex` duplicate is gone;
- the universal `relatedResult` / `assertStruct` no longer hide in the EC2 file.

The degree itself is **irreducible** without making the tests worse: any shared
test pump is high-fan-in by nature. Retiring the forwarders and qualifying all
calls as `tuitest.Step` would simply make `tuitest.Step` the new ~779-edge #1 — a
rename of the hub, not a removal of it. The only way to lower the number is to
inline `m.Update` at every call site, which deletes the harness the whole suite
depends on. We are not doing that. The metric is an *attention signal* that
correctly flagged the harness; the harness is now consolidated, and the signal has
served its purpose.

### Explicitly out of scope (with reason, not deferral)

- **Full call-site rename/qualify migration** (`rootApplyMsg` → `tuitest.Step` across
  ~150 files). This is cosmetic: it does not change behavior and does not lower any
  god-node degree — whatever primitive everything routes through *is* the hub by
  definition. The forwarder approach (A2) already gives single-source harness logic.
- **Folding the ≥34 specialized model builders** (`setupEC2ListWith*`,
  `setupDemoApp`, …) into one `Build(opts…)`. Most encode genuinely different test
  setups (cold cache, truncated cache, live mode, narrow width); collapsing them
  into one variadic builder trades clarity for a smaller symbol count. Left as-is by
  design, not omission.
- **The two-package split itself** (`unit` vs `unit_test`) is intentional Go
  structure (external test packages exercise only the public API); it is not a defect
  to "fix."

## How to reproduce

```bash
graphify explain "<symbol>"     # node location, community, degree, neighbors
graphify query "<question>"     # BFS over the graph for a flow
# GRAPH_REPORT.md → "God Nodes" section lists the ranked top 10
```

Re-run `graphify update .` after code changes to keep the graph current.
