# Phase 05 — Platform-agnostic app core; TUI boundary; gen unification; message taxonomy

**3 PRs, all mandatory. Depends on Phase 04 (catalog).**

The `Color` field was **retained** on the catalog struct (`ResourceTypeDef.Color func(domain.Resource) domain.Color`, REQUIRED for all registered types) — it classifies a row's health and returns a `domain.Color` (a renderer-free domain type, not a lipgloss style), so it does not breach the boundary. The originally-planned "Color → severity collapse" was not pursued; the per-type `Color` callback remains the source of row health and the TUI adapter maps `domain.Color` to a concrete `styles` style at render time. No standalone 5a-color PR was needed.

## Goal

Close the runtime/UI boundary and make the shared app core renderer-agnostic. Right now `internal/tui/` owns three things at once: the view stack and key handling (UI shell), the message handlers and orchestration (runtime), and the embedded session state (data). This phase splits them.

After this phase:

- **`internal/runtime/`** owns `Session`, app-core dispatch, fetcher invocation, selectors, queries, tasks, and generation stamping. It is platform-agnostic. It does not import Bubble Tea or any future desktop renderer toolkit.
- **`internal/tui/`** owns the Bubble Tea adapter: view stack, key resolution, lipgloss rendering, and translation between Bubble Tea messages/commands and the shared app-core events/tasks.
- **Future desktop delivery** plugs in as a second adapter. Candidate shells include Wails, Tauri with a Go sidecar, or an Electron-like main/renderer split. This phase does not pick one; it preserves the ability to add one later without re-architecting the core.
- **`internal/domain/Gen`** is the single generation-counter type used everywhere. The `int` / `uint64` salad of `availabilityGen` / `enrichmentGen` / `relatedGen` / `enrichGen` / `enrichmentTypeGen` collapses to one type.
- Messages are typed as either commands (UI adapter → app core: "do this") or events (app core → adapter: "this happened"), with gen-stamping enforced at the type system level. **This is mandatory, not polish.** Type-level gen-stamping prevents an entire bug class (handlers forgetting to gen-check) and "optional structural correctness" is itself the kind of compromise this refactor exists to remove.
- **Screen registration is declarative.** New deep workflows register screen IDs/descriptors with the app core; adapters decide how a given screen is rendered.
- **Slow async work uses one task contract.** Enrichment, related resolution, discovery flows, and future query scans share one runtime-owned background-task model for progress, cancellation, cache policy, and partial-completion state.

Views remain passive. They render state and emit messages. They do not consume use-case interfaces. The boundary is between the shared app core and the renderer adapters, with Bubble Tea as one adapter implementation. (See cross-phase invariants #7 and #9 in `00-overview.md`.)

## Screen and task contracts

`UIIntent` solves "how does the app core tell an adapter to update UI state?". This phase also introduces two adjacent app-core contracts needed by the open-issue roadmap:

- **Screen descriptors** — the structural home for multi-screen workflows such as logs, CloudTrail investigation, and cost views. New screens register declaratively instead of requiring more central shell branching.
- **Background tasks** — the structural home for slow work such as enrichment, related checks, log-source discovery, and future query scans. A task carries status, progress, cancellation handle, freshness/cache policy, and any view-ready summary state the UI needs to render.

Phase 05 does **not** implement those product features. It creates the app-core/adapter contracts they need so they can land without punching holes back through the boundary. It also does **not** choose the future desktop shell; that decision stays explicitly deferred until product/UI direction is clearer.

Minimum contract sketches:

```go
// internal/runtime/screens.go
package runtime

type ScreenID string

type ScreenContext struct {
    ResourceType string
    ResourceID   string
    Capability   domain.CapabilityID
    Query        domain.QuerySpec // zero value when the screen is not query-driven
}

type ScreenDescriptor struct {
    ID    ScreenID
    Title string
}

type ScreenRegistry interface {
    Register(ScreenDescriptor)
    Get(ScreenID) (ScreenDescriptor, bool)
    ScreenForCapability(domain.CapabilityID) (ScreenID, bool)
}
```

```go
// internal/tui/screens.go
package tui

type ScreenBuilder func(runtime.ScreenContext) tea.Model

type ScreenBuilders interface {
    Build(runtime.ScreenID, runtime.ScreenContext) (tea.Model, bool)
}
```

The TUI adapter populates `ScreenBuilders` during `tui.New(...)` with a fixed `runtime.ScreenID -> ScreenBuilder` map. Adding a new screen in the current program means adding one builder registration there; future adapters own their own builder maps.

```go
// internal/runtime/tasks.go
package runtime

type TaskKind string

type TaskKey struct {
    Kind  TaskKind // "enrich" | "related" | "logsource-discover" | ...
    Scope string   // resource-type / resource-id / query-id; kind-specific
}

type TaskStatus int

const (
    TaskPending TaskStatus = iota
    TaskRunning
    TaskPartial
    TaskComplete
    TaskFailed
    TaskCancelled
)

type CachePolicy int

const (
    CacheNone CachePolicy = iota
    CacheSession
    CacheUntilRotate
)

type TaskState struct {
    Key       TaskKey
    Status    TaskStatus
    Progress  float64 // 0..1; -1 = indeterminate
    StartedAt time.Time
    Cache     CachePolicy
    Cancel    context.CancelFunc
    Err       error
}

type TaskRequest struct {
    Key   TaskKey
    Cache CachePolicy
}
```

Capability dispatch rule: `ResourceTypeDef.Capabilities` declares that a type supports a cross-cutting capability; app core checks that opt-in list, resolves `CapabilityID -> ScreenID` through `ScreenRegistry`, and emits a screen request. The adapter then uses its local builder registry (`ScreenID -> ScreenBuilder`) to construct the concrete UI object. That keeps capability handlers out of `ResourceTypeDef` while still avoiding central type switches and keeps Bubble Tea types out of the shared core.

## UIIntent contract — how the app core tells an adapter to update UI state

Today's handlers walk the view stack and mutate views in place — `app_handlers_availability.go:387,421,433` are concrete examples: a single Wave 2 message updates `m.probeResources`, then iterates `m.stack` updating any matching `*ResourceListModel.SetEnrichmentState(...)` and any matching `*DetailModel.SetEnrichmentFinding(...)`. After 5a-extract the shared core no longer owns `m.stack`, so we need an explicit contract for "tell the adapter what changed" without importing renderer types.

The contract: **app core emits `[]UIIntent` plus `[]TaskRequest`; the adapter applies intents to its local UI tree and turns task requests into platform-specific async work.**

```go
// internal/runtime/intent.go
package runtime

type UIIntent interface { isIntent() }

// PatchResourceList: every resource-list view for ResourceType applies the data patch.
type PatchResourceList struct {
    ResourceType string
    Issues       *IssueBadgePatch
    Enrichment   *ListEnrichmentPatch
}
func (PatchResourceList) isIntent() {}

// PatchDetail: every detail view for ResourceType/ResourceID applies the data patch.
type PatchDetail struct {
    ResourceType string
    ResourceID   string  // empty = all detail views of that type
    Findings     []domain.Finding
    Attention    map[domain.FindingCode]attention.AttentionDetail
    FieldUpdates map[string]string
}
func (PatchDetail) isIntent() {}

type PatchMenu struct {
    ResourceType string
    Issues       int
    Truncated    bool
}
func (PatchMenu) isIntent() {}

// PushScreen, PopScreen, ReplaceScreen: structural changes described without renderer types.
type PushScreen struct {
    ID      ScreenID
    Context ScreenContext
}
type PopScreen struct{}
type ReplaceScreen struct {
    ID      ScreenID
    Context ScreenContext
}
```

App-core handler signature:

```go
func (c *Core) HandleEvent(ev event.Event) ([]UIIntent, []TaskRequest) {
    // ... mutate Session state ...
    return []UIIntent{
        PatchMenu{ResourceType: rt, Issues: count, Truncated: trunc},
        PatchResourceList{ResourceType: rt, Enrichment: &patch},
        PatchDetail{ResourceType: rt, ResourceID: id, Findings: findings, Attention: attn},
    }, nil
}
```

Bubble Tea adapter loop:

```go
// internal/tui/app.go
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    ev := translateMsg(msg)
    intents, tasks := m.core.HandleEvent(ev)
    for _, intent := range intents {
        m.applyIntent(intent)  // walks m.stack, patches matching views
    }
    return m, toTeaCmd(tasks)
}
```

This preserves today's stack-walking semantics — every matching view in the stack receives the update — while making the contract between app core and adapters explicit and testable. The shared core can be unit-tested by asserting on returned `[]UIIntent` / `[]TaskRequest` without standing up Bubble Tea. A future desktop adapter can consume the same intents over IPC without importing TUI code.

## PR breakdown

3 PRs total, all mandatory.

### PR-05a-extract — Runtime extraction; un-embed Session

**Goal.** Move all message handlers, fetcher dispatch, and session ownership out of `internal/tui/` into a new platform-agnostic `internal/runtime/` core package. `tui.Model` stops embedding `sessionRuntime` (now `internal/session.Session` after Phase 02). The Bubble Tea `Update` loop in the UI shell becomes a thin adapter that translates `tea.Msg` values into app-core events, forwards them to the core, applies returned UI intents, and turns task requests into `tea.Cmd`s.

#### Files added

- `internal/runtime/orchestrator.go` — owns `*session.Session`, app-core dispatch entry point, fetcher invocation. Replaces the bulk of `internal/tui/app.go`. **No Bubble Tea imports.**
- `internal/runtime/screens.go` — screen-descriptor definitions used to register stackable workflows without growing central shell switches.
- `internal/runtime/tasks.go` — background-task types (`TaskKey`, `TaskStatus`, `TaskProgress`, cache/freshness policy) used by enrichment, related resolution, and future discovery/query flows.
- `internal/runtime/handlers.go` — moved from `internal/tui/app_handlers.go`.
- `internal/runtime/handlers_availability.go` — moved from `internal/tui/app_handlers_availability.go`.
- `internal/runtime/handlers_navigate.go` — moved from `internal/tui/app_handlers_navigate.go`.
- `internal/runtime/handlers_related.go` — moved from `internal/tui/app_handlers_related_navigate.go`.
- `internal/runtime/probes.go`, `enrich.go`, `related.go`, `fetchers.go` — moved from corresponding `app_*.go` files.
- `internal/runtime/state.go` — `RuntimeState` struct: the view-ready state the UI shell renders. Snapshot of caches, current findings, queue progress, task state, etc.
- `internal/tui/screens.go` — Bubble Tea screen builders (`runtime.ScreenID -> tea.Model`) used by the TUI adapter only. The concrete registry plus its first four ScreenIDs (profile selector, reveal, child list, theme apply) lands in PR-05a-h4-a; see [`landed/05-pr-05a-h4.md`](./landed/05-pr-05a-h4.md).

#### Files modified

- `internal/tui/app.go` (currently ~880 lines / ~28 KB) — shrinks substantially. Realistic target: **300–400 lines**. The shell still owns view stack management, key resolver wiring, Bubble Tea message translation, screen-builder lookup, the `RuntimeState` field, the intent-application code, and `View()` rendering. The earlier draft's "under 200 lines" target was unrealistic given those responsibilities; calibrate to 300–400 instead and verify post-PR.
- `internal/tui/app_input.go` — stays in tui (input mode is UI concern).
- `internal/tui/app_handlers*.go` — content moved to `internal/runtime/`; files deleted from `internal/tui/`.
- `cmd/a9s/main.go` — wires `runtime.New(session.New(), catalog.ResourceTypes)` and passes the core plus TUI adapter wiring to `tui.New()`.

#### Files deleted

- `internal/tui/session_runtime.go` (or its remaining stub from Phase 02). `Session` lives in `internal/session/`; the embed in `tui.Model` is gone.

#### Exit criteria

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

# Shared core owns the orchestration:
rg 'func.*HandleEvent' internal/runtime/
# expected: present

# Screen/task contracts exist concretely:
rg 'type ScreenDescriptor struct|type ScreenRegistry interface' internal/runtime/screens.go
# expected: present
rg 'type TaskKey struct|type TaskState struct|type TaskStatus' internal/runtime/tasks.go
# expected: present

# Shared core packages are Bubble Tea-free:
rg 'tea\\.|bubbletea' internal/runtime/ internal/domain/ internal/session/ internal/aws/ internal/catalog/ internal/semantics/
# expected: zero hits

# tui/app.go is small:
wc -l internal/tui/app.go
# expected: 300–400 lines (was ~880)
```

> **Status note (post AS-646 audit, 2026-05-20)**: the exit criteria above did not land with PR-05a-h2/h3 — `internal/tui` still imports `internal/session` and `internal/aws`, and `app.go` grew to ~986 LOC during the migration. The follow-on spec [`landed/05-pr-05a-h4.md`](./landed/05-pr-05a-h4.md) (AS-650) breaks the residual work into three child PRs (h4-a screen-builder + four handler ports; h4-b `Update()`-switch extraction; h4-c final import scrub) and is the first to pass the bash gates above when it ships.

Behavior verification:
- `make test` and `make test-race` pass.
- `./a9s --demo`: every flow exercised. List → detail → YAML → JSON. Profile-switch. Region-switch. Ctrl+R refresh. Wave 2 enrichment progress visible.
- Integration test against real AWS: `A9S_CT_PROFILE=<profile> go test -tags integration ./tests/integration/ -run TestFullRelatedViewValidation`.

---

### PR-05a-gens — Generation type unification; UI shell shrink

**Goal.** Replace `availabilityGen int`, `enrichmentGen int`, `relatedGen uint64`, `enrichGen uint64`, and per-type `enrichmentTypeGen map[string]int` with a single `domain.Gen` type (`uint64`). One generation-counter type across the whole program.

This PR also captures any final UI-shell shrinkage: dead helpers in `internal/tui/` that became unreferenced after PR-05a-extract.

#### Files added

- `internal/domain/gen.go` — `Gen uint64` type, methods (`Bump()`, `Stamp() Gen`, etc.), thread-safety contract.

#### Files modified

- `internal/session/session.go` — generation counter fields (`availabilityGen`, `enrichmentGen`, `relatedGen`, `enrichGen`) all become `domain.Gen`. `Session.Rotate()` calls `.Bump()` on each.
- Every message type that carries a `Gen` or `TypeGen` field — switch to `domain.Gen`.
- Every gen-guard in handlers — switch to `domain.Gen` comparisons.
- `internal/runtime/orchestrator.go` — gen-stamping uniform.
- `internal/tui/views/*.go` — any view that holds gen state (e.g. `ResourceListModel.gen`) switches to `domain.Gen`.

#### Exit criteria

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

**Goal.** Split the 28 `*Msg` types in `internal/tui/messages/messages.go` into two clearly named families of platform-agnostic app-core contracts:

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

Generation stamping moves into an `Event` base (or a generated wrapper) so handlers can no longer forget to gen-check. These types are plain app-core contracts; renderer adapters translate to and from Bubble Tea, Electron IPC, or any future transport at the boundary.

#### Files added

- `internal/runtime/cmd/types.go` — command types (UI → runtime).
- `internal/runtime/event/types.go` — event types (runtime → UI).

#### Files modified

- Every message-emitting site — emits `cmd.LoadResources` or `event.ResourcesLoaded` instead of bare `messages.LoadResourcesMsg`.
- Every handler — receives typed cmd or event.

#### Files deleted

- `internal/tui/messages/messages.go` — replaced by the cmd/event split.

#### Exit criteria

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

- Renaming `internal/aws/` → `internal/transport/`. **Decision: no rename, ever, in this program** (see `04-catalog.md` "Out of scope" for rationale). `internal/aws/` is the permanent home of transport functions.
- Implementing logs, CloudTrail search/debug, cost explorer, or mutating actions. Phase 05 creates the screen/task boundary those features need; it does not ship the features themselves.
- Further view-layer reorganization (`views/shell/`, `views/components/`). Cosmetic; defer.
- Theme system overhaul. Out of refactor scope.

## Cross-references

- **Depends on Phase 02**: `Session` exists as a clean type — un-embedding from `tui.Model` is mechanical.
- **Depends on Phase 04**: catalog is the source of truth that the runtime reads; no registry-coupling concerns at this point.
- **Closes the program**: when all three PRs land (5a-extract, 5a-gens, 5b), the mechanical-resource-implementation acceptance test from `00-overview.md` is the final exit criterion.

## Risk register

| Risk | Mitigation |
|---|---|
| 05a-extract sprawls because handler files are large (~64 KB total handler code) | If a single PR feels too wide, split per-handler-file (one PR per `app_handlers*.go` migration). The ordering doesn't matter — each handler is independent — so multiple small PRs work. |
| Tests that constructed `tui.Model` with embedded session break | Most tests use `tui.New(...)` constructor with options. Audit `tests/unit/` for direct `tui.Model{...}` literal construction; convert to `tui.New()`. |
| Gen unification flushes out latent races caught by `-race` | Good — that's the point. Land 05a-gens with `make test-race` as a hard gate. Any race surfaced is a bug fixed by the unification, not introduced by it. |
| 05b done in pieces leaves messages package half-migrated | 05b is one PR. The cmd/event split is atomic because the new `Cmd` and `Event` types are permanent end-state contracts — `00-overview.md` "Migration discipline" hard-safety #4 forbids introducing them in a transitional shape and renaming later. PR-05b lands the full split or doesn't land at all. |
| Patch-data structs (`IssueBadgePatch`, `ListEnrichmentPatch`, `PatchDetail`) drift from what views actually need to render | Test the adapter, not compile-time callback interfaces. Unit tests feed each `Patch*` variant into the TUI adapter and assert the resulting view-state diff; view tests assert the patch payloads carry enough data to render list badges, enrichment state, and detail Attention rows without reaching back into runtime. |
