# Phase 05 — Runtime/UI boundary; gen unification; message taxonomy

**3 PRs, all mandatory. Depends on Phase 04 (catalog).**

## Goal

Close the runtime/UI boundary. Right now `internal/tui/` owns three things at once: the view stack and key handling (UI shell), the message handlers and orchestration (runtime), and the embedded session state (data). This phase splits them.

After this phase:

- **`internal/runtime/`** owns `Session`, message dispatch, fetcher invocation, generation stamping. It is the orchestrator.
- **`internal/tui/`** owns view stack, key resolution, lipgloss rendering. It is the UI shell. It does not embed `Session`. It does not dispatch fetchers. It receives view-ready state and emits messages back.
- **`internal/domain/Gen`** is the single generation-counter type used everywhere. The `int` / `uint64` salad of `availabilityGen` / `enrichmentGen` / `relatedGen` / `enrichGen` / `enrichmentTypeGen` collapses to one type.
- Messages are typed as either commands (UI → runtime: "do this") or events (runtime → UI: "this happened"), with gen-stamping enforced at the type system level. **This is mandatory, not polish.** Type-level gen-stamping prevents an entire bug class (handlers forgetting to gen-check) and "optional structural correctness" is itself the kind of compromise this refactor exists to remove.

Views remain passive. They render state and emit messages. They do not consume use-case interfaces. The boundary is between message handlers and the view layer, not between views and a service API. (See cross-phase invariant #7 in `00-overview.md`.)

## ViewIntent contract — how the runtime tells the UI to update views

Today's handlers walk the view stack and mutate views in place — `app_handlers_availability.go:387,421,433` are concrete examples: a single Wave 2 message updates `m.probeResources`, then iterates `m.stack` updating any matching `*ResourceListModel.SetEnrichmentState(...)` and any matching `*DetailModel.SetEnrichmentFinding(...)`. After 5a-extract the runtime no longer owns `m.stack`, so we need an explicit contract for "update every view of kind X for resource type Y."

The contract: **runtime emits `[]ViewIntent` from each handler; UI shell applies them across `m.stack`.**

```go
// internal/runtime/intent.go
package runtime

type ViewIntent interface { isIntent() }

// UpdateResourceList: every ResourceListModel for ResourceType receives Update.
type UpdateResourceList struct {
    ResourceType string
    Update       func(rl ResourceListView) // narrow interface, see below
}
func (UpdateResourceList) isIntent() {}

// UpdateDetail: every DetailModel for ResourceType (and ResourceID, if non-empty) receives Update.
type UpdateDetail struct {
    ResourceType string
    ResourceID   string  // empty = all detail views of that type
    Update       func(d DetailView)
}
func (UpdateDetail) isIntent() {}

// UpdateMenu: the singleton MainMenuModel receives Update.
type UpdateMenu struct {
    Update func(m MenuView)
}
func (UpdateMenu) isIntent() {}

// PushView, PopView, ReplaceView: structural changes to the stack.
type PushView   struct { View tea.Model }
type PopView    struct{}
type ReplaceView struct { View tea.Model }
```

The narrow interfaces (`ResourceListView`, `DetailView`, `MenuView`) are defined by the UI shell (`internal/tui/views/`) and exported for runtime use. They expose only the methods runtime needs to call (`SetEnrichmentState`, `SetEnrichmentFinding`, `ApplyFieldUpdates`, `SetIssues`, etc.) — not the full Bubble Tea `Update` / `View` surface. This keeps runtime decoupled from rendering details.

Runtime handler signature:

```go
func (r *Runtime) HandleMsg(msg tea.Msg) ([]ViewIntent, tea.Cmd) {
    // ... mutate Session state ...
    return []ViewIntent{
        UpdateMenu{Update: func(m MenuView) { m.SetIssues(rt, count, trunc) }},
        UpdateResourceList{ResourceType: rt, Update: func(rl ResourceListView) { rl.SetEnrichmentState(...) }},
        UpdateDetail{ResourceType: rt, Update: func(d DetailView) { d.SetEnrichmentFinding(&f) }},
    }, nil
}
```

UI shell apply loop:

```go
// internal/tui/app.go
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    intents, cmd := m.runtime.HandleMsg(msg)
    for _, intent := range intents {
        m.applyIntent(intent)  // walks m.stack, dispatches to matching views
    }
    return m, cmd
}
```

This preserves today's stack-walking semantics — every matching view in the stack receives the update — while making the contract between runtime and UI explicit and testable. The runtime can be unit-tested by asserting on the returned `[]ViewIntent` without standing up a UI; the UI can be unit-tested by feeding it synthetic intents without standing up a runtime.

## PR breakdown

3 PRs total, all mandatory.

### PR-05a-extract — Runtime extraction; un-embed Session

**Goal.** Move all message handlers, fetcher dispatch, and session ownership out of `internal/tui/` into a new `internal/runtime/` package. `tui.Model` stops embedding `sessionRuntime` (now `internal/session.Session` after Phase 02). The Bubble Tea `Update` loop in the UI shell becomes a thin pass-through that forwards messages to the runtime and applies returned UI updates.

**Files added**

- `internal/runtime/orchestrator.go` — owns `*session.Session`, message dispatch entry point, fetcher invocation. Replaces the bulk of `internal/tui/app.go`.
- `internal/runtime/handlers.go` — moved from `internal/tui/app_handlers.go`.
- `internal/runtime/handlers_availability.go` — moved from `internal/tui/app_handlers_availability.go`.
- `internal/runtime/handlers_navigate.go` — moved from `internal/tui/app_handlers_navigate.go`.
- `internal/runtime/handlers_related.go` — moved from `internal/tui/app_handlers_related_navigate.go`.
- `internal/runtime/probes.go`, `enrich.go`, `related.go`, `fetchers.go` — moved from corresponding `app_*.go` files.
- `internal/runtime/state.go` — `RuntimeState` struct: the view-ready state the UI shell renders. Snapshot of caches, current findings, queue progress, etc.

**Files modified**

- `internal/tui/app.go` (currently 28 KB) — shrinks to ~3–5 KB. Holds `tui.Model` with view stack, key resolver, current `RuntimeState`. `Update(msg)` calls `runtime.HandleMsg(msg, &state)` and updates the view stack; nothing else.
- `internal/tui/app_input.go` — stays in tui (input mode is UI concern).
- `internal/tui/app_handlers*.go` — content moved to `internal/runtime/`; files deleted from `internal/tui/`.
- `cmd/a9s/main.go` — wires `runtime.New(session.New(), catalog.ResourceTypes)` and passes the orchestrator to `tui.New()`.

**Files deleted**

- `internal/tui/session_runtime.go` (or its remaining stub from Phase 02). `Session` lives in `internal/session/`; the embed in `tui.Model` is gone.

**Exit criteria**

```bash
# tui no longer owns Session:
rg 'sessionRuntime|session\.Session' internal/tui/app.go
# expected: zero hits (Model holds no Session field; runtime owns it)

# tui does not import internal/session or internal/aws:
go list -f '{{.Imports}}' github.com/k2m30/a9s/v3/internal/tui | tr ' ' '\n' | grep -E 'internal/session|internal/aws'
# expected: zero hits

# Handlers moved:
ls internal/tui/app_handlers*.go 2>&1
# expected: "No such file or directory" (or only a stub for input handlers if separated)
ls internal/runtime/handlers*.go
# expected: present

# Runtime owns the orchestration:
rg 'func.*HandleMsg' internal/runtime/
# expected: present

# tui/app.go is small:
wc -l internal/tui/app.go
# expected: under 200 lines (was ~700)
```

Behavior verification:
- `make test` and `make test-race` pass.
- `./a9s --demo`: every flow exercised. List → detail → YAML → JSON. Profile-switch. Region-switch. Ctrl+R refresh. Wave 2 enrichment progress visible.
- Integration test against real AWS: `A9S_CT_PROFILE=<profile> go test -tags integration ./tests/integration/ -run TestFullRelatedViewValidation`.

---

### PR-05a-gens — Generation type unification; UI shell shrink

**Goal.** Replace `availabilityGen int`, `enrichmentGen int`, `relatedGen uint64`, `enrichGen uint64`, and per-type `enrichmentTypeGen map[string]int` with a single `domain.Gen` type (`uint64`). One generation-counter type across the whole program.

This PR also captures any final UI-shell shrinkage: dead helpers in `internal/tui/` that became unreferenced after PR-05a-extract.

**Files added**

- `internal/domain/gen.go` — `Gen uint64` type, methods (`Bump()`, `Stamp() Gen`, etc.), thread-safety contract.

**Files modified**

- `internal/session/session.go` — generation counter fields (`availabilityGen`, `enrichmentGen`, `relatedGen`, `enrichGen`) all become `domain.Gen`. `Session.Rotate()` calls `.Bump()` on each.
- Every message type that carries a `Gen` or `TypeGen` field — switch to `domain.Gen`.
- Every gen-guard in handlers — switch to `domain.Gen` comparisons.
- `internal/runtime/orchestrator.go` — gen-stamping uniform.
- `internal/tui/views/*.go` — any view that holds gen state (e.g. `ResourceListModel.gen`) switches to `domain.Gen`.

**Exit criteria**

```bash
# Only one gen type:
rg 'availabilityGen \w+|enrichmentGen \w+|relatedGen \w+|enrichGen \w+' internal/
# expected: zero hits — type names are removed; field names may remain but the type is domain.Gen

rg '\bGen\s+(int|uint64)\b' internal/
# expected: zero hits — only domain.Gen type assignments

# All gen comparisons use domain.Gen:
rg '\.Gen\s*[!=]=' internal/runtime/ internal/tui/
# expected: hits all involve domain.Gen typed fields
```

Behavior verification:
- `make test` and `make test-race` pass — race detector is the critical guard for gen counter changes.
- Profile-switch + concurrent in-flight enrichment: stale results are dropped, fresh results land. Manual smoke test in demo mode.

---

### PR-05b — Command vs event message taxonomy

**Mandatory.** Type-level gen-stamping prevents handlers from forgetting to gen-check; that prevention is structural, not stylistic. "Optional structural correctness" is the kind of compromise this refactor exists to remove.

**Goal.** Split the 28 `*Msg` types in `internal/tui/messages/messages.go` into two clearly named families:

```go
// internal/runtime/cmd
type Cmd interface { isCmd() }
type LoadResources struct { ... }
type Refresh        struct { ... }
type EnrichDetail   struct { ... }
type Navigate       struct { ... }

// internal/runtime/event
type Event interface { isEvent() }
type ResourcesLoaded   struct { Gen domain.Gen; ... }
type RelatedChecked    struct { Gen domain.Gen; ... }
type EnrichmentChecked struct { Gen domain.Gen; ... }
type Flash             struct { ... }
```

Generation stamping moves into an `Event` base (or a generated wrapper) so handlers can no longer forget to gen-check.

**Files added**

- `internal/runtime/cmd/types.go` — command types (UI → runtime).
- `internal/runtime/event/types.go` — event types (runtime → UI).

**Files modified**

- Every message-emitting site — emits `cmd.LoadResources` or `event.ResourcesLoaded` instead of bare `messages.LoadResourcesMsg`.
- Every handler — receives typed cmd or event.

**Files deleted**

- `internal/tui/messages/messages.go` — replaced by the cmd/event split.

**Exit criteria**

```bash
# Split is clean:
rg 'isCmd|isEvent' internal/runtime/
# expected: every message type in cmd/ implements isCmd; every message type in event/ implements isEvent

# Old messages package is gone:
ls internal/tui/messages/messages.go 2>&1
# expected: "No such file or directory"

# Every event carries domain.Gen:
rg 'type \w+ struct' internal/runtime/event/types.go
# expected: every type has a Gen domain.Gen field
```

## Out of scope

- Renaming `internal/aws/` → `internal/transport/`. Mechanical, post-refactor.
- Further view-layer reorganization (`views/shell/`, `views/components/`). Cosmetic; defer.
- Theme system overhaul. Out of refactor scope.

## Cross-references

- **Depends on Phase 02**: `Session` exists as a clean type — un-embedding from `tui.Model` is mechanical.
- **Depends on Phase 04**: catalog is the source of truth that the runtime reads; no registry-coupling concerns at this point.
- **Closes the program**: when 05a-extract and 05a-gens land, the mechanical-resource-implementation acceptance test from `00-overview.md` is the final exit criterion. 05b is optional polish.

## Risk register

| Risk | Mitigation |
|---|---|
| 05a-extract sprawls because handler files are large (~64 KB total handler code) | If a single PR feels too wide, split per-handler-file (one PR per `app_handlers*.go` migration). The ordering doesn't matter — each handler is independent — so multiple small PRs work. |
| Tests that constructed `tui.Model` with embedded session break | Most tests use `tui.New(...)` constructor with options. Audit `tests/unit/` for direct `tui.Model{...}` literal construction; convert to `tui.New()`. |
| Gen unification flushes out latent races caught by `-race` | Good — that's the point. Land 05a-gens with `make test-race` as a hard gate. Any race surfaced is a bug fixed by the unification, not introduced by it. |
| 05b done in pieces leaves messages package half-migrated | 05b is one PR. Don't half-migrate; the cmd/event split is atomic. |
| Narrow view interfaces (`ResourceListView`, `DetailView`, `MenuView`) drift from concrete view types | Each view type implements its narrow interface explicitly with a compile-time assertion (`var _ runtime.ResourceListView = (*views.ResourceListModel)(nil)`). Adding a method to the interface fails to compile in any view that doesn't implement it. |
