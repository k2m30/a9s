# Web UI + Headless Controller Plan (test harness first)

## Context

The 020 refactor already split a UI-agnostic `runtime.Core`
(`HandleEvent(Event) → ([]UIIntent, []TaskRequest)`, zero Bubble Tea deps) from the
TUI. But the split is **mid-flight**: `runtime.Core` only handles a subset of events
today (availability/enrichment/identity), while `internal/tui/app.go`'s `Update()`
still routes navigation, resources, related-checks, child-views, and
profile/region/theme directly to TUI-side handlers. The **view stack**
(`Model.stack []views.View`) and all **per-view state** (filter, sort, search cursor,
scroll offset, selected row, related-panel focus, wrap toggle) live *inside* Bubble Tea
view models (`resourcelist.go` ~600 LOC, `detail.go`, `yaml.go`, …).

Two goals, in order:

1. **Now — Web UI as a deterministic test harness.** Today's integration tests already
   drive `Model.Update` directly and recursively drain returned `tea.Cmd`s — so the gap is
   *not* "no terminal". The gap is that they assert on brittle rendered strings and
   TUI-internal fields. A server-rendered web UI over a **headless controller** with a
   **synchronously-drainable task queue** lets integration tests script real app behavior
   (`Apply(action) → drain tasks → assert on a JSON state snapshot`); the win is **stable
   JSON state assertions instead of rendered-string / internal-model assertions**. Runs via
   `--web` flag or `A9S_MODE=web` (TUI remains default).
2. **Next (separate effort, out of scope here) — rewrite the issue-detection/cache
   layer.** It's genuinely tangled (findings on mutable cached rows; three in-memory
   cache maps — `ResourceCache`/`LazyResourceCache`/`ProbeResources` — with inconsistent
   clear paths; enricher snapshot aliases shared backing arrays under a runtime-only
   contract; findings silently dropped when a row is evicted before Wave-2 lands; two
   parallel `applyEnrichment` paths). The web harness from goal 1 is the enabler that
   makes this rewrite verifiable.

Decisions taken: **web UI now / cache rework next**, **server-rendered HTML**, and
**extract a shared headless controller** (TUI + web both thin renderers, so they can't
drift and web tests validate the *real* app logic).

## Current Reality (verified against code, 2026-06)

The split is **less complete than an earlier draft of this plan implied** — corrected here
after review:

- **`runtime.Core.HandleEvent` dispatches only six result events today** —
  `messages.{AvailabilityCacheLoaded, AvailabilityPrefetched, AvailabilityChecked,
  EnrichmentChecked, IdentityLoaded, IdentityError}` (`orchestrator.go:56`). Every other
  event type **falls through to `nil, nil`**. So it is not the entry point for user
  *commands*, and it is not even a complete *result* dispatcher.
- **Other result events bypass `HandleEvent`.** `messages.{ResourcesLoaded,
  RelatedCheckResult, EnrichDetailResult, ThemeFileRead, ClientsReady, Flash, ClearFlash,
  Copied, ValueRevealed, APIError}` all implement `messages.Event` but are routed through
  dedicated `Core.HandleX` methods called *directly from the TUI adapter*
  (`internal/tui/runtime_adapter_*.go`), not through `HandleEvent`.
- **Naming trap — two parallel `*Event` families.** The `messages.*` result events are
  distinct types from the `runtime.*Event` *command* structs the `HandleX` methods take as
  input: `messages.Flash` (result) vs `runtime.FlashEvent` (command); `messages.APIError`
  vs `runtime.APIErrorEvent`; and `runtime.NavigateEvent` / `runtime.ProfileSelectedEvent`
  / … are command structs that are **not** `messages.Event` at all. Feeding a
  `runtime.*Event` command struct into the `Handle(Event)` result lane is a compile error —
  `Handle` accepts only `messages.Event`.
- **User commands route through separate `Core.HandleX` methods with non-uniform return
  types**: `HandleNavigate(NavigateEvent) → (NavigateResult, []TaskRequest)`,
  `HandleRelatedNavigate(RelatedNavigateEvent) → (NavigationResult, []TaskRequest)`, and
  `HandleProfileSelected / HandleThemeSelected / HandleRegionSelected / HandleEnterChildView
  / HandleValueRevealed → ([]UIIntent, []TaskRequest)`. There is **no single
  `HandleCommand`** today.
- **View state still lives in Bubble Tea models** (`internal/tui/views/*`); the
  controller's `ScreenState` does not exist yet.
- **Task execution is scattered** across TUI adapter methods
  (`internal/tui/runtime_adapter_*.go`): fetches, probes, connect, cache save, theme file
  read/write, reveal, copy, related fetches — and some read TUI state (e.g. availability
  cache-save reads `m.stack[0]`). There is no renderer-neutral task runner.

Consequences baked into the phasing below: a **two-lane controller API**, a **task-executor
extraction PR (PR-B0)**, and a **full state-inventory PR (PR-B1)** — all *before* the
view-state lift in PR-C.

## Architecture

```text
            ┌───────────────────────────────────────────────┐
            │ internal/app  (NEW — headless controller)       │
            │  • Screen stack (data, not Bubble Tea models)   │
            │  • per-screen view state (filter/sort/search/…) │
            │  • Action set (semantic, not keystrokes)        │
            │  • ViewState snapshot (renderer-agnostic, JSON) │
            │  • drives runtime.Core; owns a Task queue       │
            └───────────────┬───────────────┬─────────────────┘
        Apply(Action)       │               │   HandleEvent(Event)
        → ViewState + Tasks │               │   ← Task results
                ┌───────────┴───┐       ┌───┴────────────┐
                │ internal/tui  │       │ internal/web    │  (NEW)
                │ key→Action;   │       │ HTTP; POST      │
                │ Lipgloss      │       │ /action; html/  │
                │ render of     │       │ template render │
                │ ViewState;    │       │ of ViewState;   │
                │ Task→tea.Cmd  │       │ GET /state JSON;│
                └───────────────┘       │ SSE for async   │
                                        └─────────────────┘
                          shared, unchanged ▼
        internal/runtime.Core ─ internal/session ─ internal/aws ─ internal/catalog
```

Seam types that already exist and are reused as-is: `runtime.UIIntent` variants
(`internal/runtime/intent.go`), `runtime.RuntimeState` (`state.go`),
`runtime.TaskRequest` (`tasks.go`), the `messages.Event` family
(`internal/runtime/messages/`), and the catalog (`resource.AllResourceTypes()`).

### The headless controller (`internal/app`)

- **Screen stack as data.**
  `type Screen struct { ID runtime.ScreenID; Ctx runtime.ScreenContext; State ScreenState }`.
  The stack is `[]Screen`. `runtime.Core` already emits
  `PushScreen`/`PopScreen`/`ReplaceScreen` intents keyed by `ScreenID`, so the
  controller applies them to its own stack — no renderer types involved.
- **Per-screen view state** moves here out of the Bubble Tea models: `ListState`
  (filter, sortCol, sortDir, selectedRow, scrollX/Y, attentionOnly, pagination cursor),
  `DetailState` (search query/cursor, wrap, relatedFocus, scrollY), `TextState`
  (yaml/json: search, wrap, scrollY).
- **Action set** — semantic verbs, the single input vocabulary both renderers translate
  into: `MoveUp/Down/Top/Bottom`, `Select`, `Back`,
  `OpenDetail/OpenYAML/OpenJSON/OpenHelp/OpenIdentity`, `Reveal`, `SetFilter(s)`,
  `Sort(col)`, `Search(s)/SearchNext/SearchPrev/SearchClear`, `Copy`, `ToggleRelated`,
  `ToggleWrap`, `ChildView(trigger)` (e/L/r/s/Enter/t), `LoadMore`, `Refresh`,
  `ToggleAttention`, `Command(s)`, `SelectProfile(s)/SelectRegion(s)`, `SelectTheme(s)`,
  `Quit`.
- **Two-lane controller API** — the boundary the review corrected. User intent and task
  results are *different* lanes because the Core exposes them differently:
  - `Apply(Action) (ViewState, []runtime.TaskRequest)` — translates an Action into the
    matching `Core.HandleX` command call (`HandleNavigate`, `HandleProfileSelected`,
    `HandleThemeSelected`, `HandleRegionSelected`, `HandleEnterChildView`,
    `HandleRelatedNavigate`) and applies its result to the stack. `HandleValueRevealed` is
    **not** a command — it is a task-*result* handler (the value arrives after the reveal
    fetch) and belongs to the `Handle` lane.
  - `Handle(Event) (ViewState, []runtime.TaskRequest)` — feeds a task-*result* event
    through the existing `Core.HandleEvent` orchestrator.
  - `ApplyIntents([]runtime.UIIntent) ViewState` — shared stack mechanics
    (Push/Pop/ReplaceScreen) used by both lanes and by tests.
  Returning `[]runtime.TaskRequest` directly (no wrapper type) keeps the seam thin.
- **ViewState snapshot** — the contract both renderers consume and tests assert on.
  Fully serializable (JSON tags), no Lipgloss/Bubble Tea/AWS types:
  - `Header{ Version, Profile, Region, Mode, RightSide, Flash{Text,IsError}, ErrorHintVisible }`
  - `FrameTitle string`, `Footer []KeyHint`, `HelpContext`
  - `Body` is a tagged union by screen kind:
    - `List{ Columns, Rows[]{Cells, Decorator "!"|"~"|"", Severity}, Selected, ScrollX, Filter, Sort, AttentionOnly, Loading, Truncated, Pagination }`
    - `Detail{ Fields, Attention[]{Finding+rows}, Related[]{Name,Count,Items}, RelatedFocused, Search, Wrap }`
    - `Text{ Lines, SearchMatches, Wrap }` (yaml/json)
    - `Menu{ Entries[]{ShortName, Display, Alias, IssueBadge{Count,Truncated}, Availability}, Selected, Filter, AttentionOnly, Progress }`
    - `Selector{ Items, Selected }` (profile/region/theme)
    - `Help{ Context, Sections[]{Title, Hints[]KeyHint} }`
    - `Identity{ AccountID, AccountAlias, ARN, IsAssumedRole, RoleName, SessionName,
      UserName, Profile, Region, Loading, ErrorMsg }` (mirrors `views.IdentityData` +
      session context)
- **Async / Task model — the testing keystone.** `Apply` and `Handle` return
  `[]runtime.TaskRequest`. The controller never blocks. A renderer-neutral **TaskExecutor**
  (extracted in PR-B0) runs tasks; the *host* wires it:
  - TUI host: `TaskRequest → tea.Cmd`; result event → `controller.Handle` (in `Update`).
  - Web host: `TaskRequest → goroutine`; result `Event → controller.Handle`; push to
    clients via SSE.
  - **Tests:** a `DrainSync(controller, pending)` helper runs the returned tasks inline
    (tasks call the same fetchers; in demo/fake mode they're synchronous) and re-feeds
    result events through `Handle` until no tasks remain — deterministic, no terminal, no
    sleeps. **This is only real after PR-B0** (the TaskExecutor extraction); in PR-A
    `DrainSync` is a stub that drains the seeded slice without executing.
- **Concurrency contract.** A `Controller` is **not** safe for concurrent use: `Apply`,
  `Handle`, and `Snapshot` all mutate the stack. The host serializes access — the TUI is
  already single-goroutine (Bubble Tea's `Update`); the web host MUST funnel `/action`,
  SSE task-completion, and `/state` for a session through a single goroutine (actor loop)
  or a per-session mutex. One `Controller` per session (see PR-D security model).

## Phasing (sequenced PRs)

This is large; ship it as a sequence so the TUI keeps working at every step.

**PR-A — Contracts & controller skeleton.** Create `internal/app`: `Action`, `ViewState`
(+ JSON tags), `Controller` wrapping `runtime.Core` with the two-lane API
`Apply`/`Handle`/`ApplyIntents`/`Snapshot` (skeletons — stack mechanics live, verb routing
stubbed for PR-B). Lanes return `[]runtime.TaskRequest` **directly; no `Task` wrapper
type**. No behavior moved. Add the `DrainSync(c, pending)` test helper (a drain stub until
PR-B0 wires execution).

**PR-B — Command/event boundary.** Do **not** force all input through `HandleEvent`
(commands and results are distinct — see Current Reality). Instead make
`app.Controller.Apply(Action)` dispatch to the existing `Core.HandleNavigate /
HandleProfileSelected / HandleThemeSelected / HandleRegionSelected / HandleEnterChildView /
HandleRelatedNavigate`, and `Handle(Event)` to `Core.HandleEvent`. **Reveal** starts on the
command lane as `HandleNavigate(NavigateTargetReveal)` (emits the fetch task); its result
handler `HandleValueRevealed` runs on the `Handle` lane when the fetched value lands.
Build each `XEvent` from its Action; normalize the non-uniform return types
(`NavigateResult` / `NavigationResult` / `[]UIIntent`) into stack ops via `ApplyIntents`.
Thin out the now-duplicated `internal/tui/runtime_adapter_*.go` paths.
**Result-lane uniformity is DEFERRED past PR-C — verified blocked.** `HandleEvent` covers
only 6 events; the other result events (`ResourcesLoaded`, `RelatedCheckResult`,
`EnrichDetailResult`, `ThemeFileRead`, `ClientsReady`, `Flash`, `ClearFlash`, `ValueRevealed`,
`APIError`) are NOT cleanly addable to `HandleEvent` yet. Each reaches `Core.HandleX` today
through a **TUI shim that does renderer-coupled pre-processing first** — bumping `flash.gen`,
resolving `sourceID` from the active detail view, running `styles.ThemeFromYAML` to compute
`ParseErr`, passing `StackDepth`/`HasActiveRL`. A naive `messages.X → HandleX` case would
call Core with missing/wrong data, and (once the controller is live in PR-C) double-dispatch
against the surviving shim. Relocating that pre-processing into Core / a renderer-neutral step
depends on the PR-C state lift and the PR-B0 executor. **Until then `Controller.Handle`
stays at the original 6 events**, and `Handle(<an un-wired result event>)` is a documented
safe no-op pass-through. So PR-B's delivered scope is the **command lane** (6 stateless
actions) + `applyNavResult`, not result-lane uniformity.

**PR-B0 — Task executor extraction.** Move task-running logic out of the TUI adapter
methods into a renderer-neutral `TaskExecutor` with explicit inputs/outputs (no `m.stack`
reads). Three callers share it: TUI async (`→ tea.Cmd`), web async (`→ goroutine`), tests
(`sync`). Audit every current task — fetches, probes, connect, cache save, theme file
read/write, reveal, copy, related fetches — and give each a state-free signature.
`DrainSync` calls the sync executor. This is the prerequisite that makes `DrainSync` real
(today it only empties the queue).

*Status (landed):* **Pass A** added `Core.ExecuteTask(ctx, req) (messages.Event, error)`
(+ `isDemo` promoted to a Core field; `save-cache`/`related-check` de-coupled from
`m.stack`). **Pass B** rewired the TUI execution sites to delegate to `ExecuteTask` and
made `DrainSync` execute for real. Two honest residuals remain:

- **`DrainSync` executes but does not yet *apply* most results.** It runs tasks and feeds
  events through `Controller.Handle`, but the result lane is still deferred (only 6 events
  dispatched) and `ApplyIntents` no-ops menu/list/detail intents — so draining a fetch task
  does not yet populate list/detail/cache state through the controller. DrainSync is
  deterministic for *execution* + the 6 dispatched events; full state application lands with
  the result lane + `ApplyIntents` in PR-C.
- **Four task kinds were kept on the TUI's own path** (not yet unified through `ExecuteTask`),
  because the executor diverges from TUI semantics — this is residual drift to close, not a
  clean single path yet:
  1. `enrich-detail` — TUI wraps a 10s per-call timeout `ExecuteTask` lacks. *Fix:* add the
     timeout in `ExecuteTask`.
  2. `fetch-by-id-detail` (related) — `ExecuteTask` returns `ResourcesLoaded` but the TUI also
     emits a `Navigate{TargetDetail}` side-effect. *Fix:* model the navigation as a follow-up
     intent/result, not buried in the executor.
  3. `fetch-filtered` (related) — the related handler builds the task with no
     `fetchFilteredPayload` (filter lives in `result.FetchFilter`). *Fix:* normalize the
     related handler to populate the payload.
  4. `save-cache` — TUI reads live `MainMenuModel` counts; `ExecuteTask` derives from session
     `ResourceCache`. These converge once menu state is lifted in **PR-C**.

**PR-B1 — State inventory + snapshot contract.** Before lifting state, table *every*
mutable field in `MainMenuModel`, `ResourceListModel`, `DetailModel`, and the
YAML/JSON/selector/reveal models — including the ones the first draft missed: server-side
`fetchFilter`, `relatedIDSet`, `autoOpenSingleDetail`, parent context, title suffix,
reapply checker/source, enrichment/truncation maps, field cursor, right-column
filter/focus/selection, pending related dispatch, nav provider. For each field decide:
**controller state**, **derived ViewState**, **renderer-only cache**, or **delete**. The
output is the authoritative `ScreenState`/`ViewState` shape PR-C implements against — so
PR-C cannot silently drop behavior.

*Status (landed):* the full inventory is [`web-ui-state-inventory.md`](web-ui-state-inventory.md)
— per-model CONTROLLER/DERIVED/RENDERER/DELETE buckets, the consolidated `ScreenState` target,
task-coupling risks, and three open decisions (`fieldCursor`, `autoOpenSingleDetail` clear
semantics, `wrap` scope) to confirm during PR-C rather than guess.

**PR-C — Lift view-stack + view-state into the controller.** Move `Model.stack` and the
filter/sort/search/scroll/selection state out of `internal/tui/views/*` into
`app.Screen`/`ScreenState`. Refactor view rendering into pure functions
`Render(ViewState.Body) string` (TUI renderer keeps Lipgloss; reuse `table_render.go`,
`detail_render.go`, etc., but driven by ViewState, not internal model fields).
`internal/tui/app.go` becomes a thin shell: `tea.KeyMsg → Action` (the existing
`keys.Map` already maps keys; route through it), `controller.Apply`, render
`controller.Snapshot()`, `Task → tea.Cmd`. **Behavior must be unchanged** — gated by the
existing `tests/unit/tuitest` harness + the full unit suite.

**PR-D — Web renderer + server (`internal/web`).** `net/http` server (stdlib +
`html/template`, htmx inlined as a static asset — no JS build, no external deps):

- `GET /` → full page render of `controller.Snapshot()`.
- `POST /action` (htmx) → decode Action, `controller.Apply`, return the updated body
  fragment.
- `GET /state` → `controller.Snapshot()` as JSON (test assertion surface).
- `GET /events` → SSE stream; when a Task completes and mutates state, push a re-render
  trigger.
- Keyboard parity: a small inlined JS handler maps key presses to `POST /action` so the
  web UI is drivable the same way as the TUI (and matches the key map).
- Wire mode selection in `cmd/a9s/main.go`: add `--web` bool flag + read `A9S_MODE` env
  (`web` → server, anything else/unset → TUI). Branch before `tea.NewProgram`; reuse the
  same `--demo`/`--profile`/`--region` option plumbing. Both modes build the same
  `app.Controller`.
- **Security model (mandatory, not optional)** — this endpoint serves live AWS state:
  - Bind **`127.0.0.1` only** (configurable via `--web-addr`; never default to `0.0.0.0`).
  - **Random per-run token** required on every `/action` / `/state` / `/events` request;
    **no CORS**; `Cache-Control: no-store` on all responses.
  - **Per-session `app.Controller`** keyed by the token — never one shared controller
    across tabs/tests (it would race actions and SSE updates). Use a per-session store.
  - CSRF protection on `POST /action`; clean SSE lifecycle + graceful shutdown.
  - **No `Reveal` endpoint/action** exposed unless behind the token *and* an explicit
    opt-in flag — revealing secret values over HTTP is off by default.

**PR-E — Web integration test harness.** `tests/integration/web/`: `httptest.Server` +
the controller's `DrainSync`. Port the scenarios in
`tests/integration/SCENARIO_HARNESS.md` to drive `POST /action` and assert on
`GET /state` JSON. Cover all resource types and child-views (per the CLAUDE.md "test ALL
resource types" rule). This is the deliverable that makes the goal-2 cache rewrite
verifiable.

**Out of scope (explicit):** the issue-detection/cache subsystem rewrite. It becomes its
own plan once PR-E lands, using the web harness to lock current behavior before
refactoring.

## Critical files

- **Create:** `internal/app/{action.go,viewstate.go,controller.go,screenstate.go}` (no
  `task.go` — lanes return `[]runtime.TaskRequest` directly, no wrapper);
  `internal/app/drainsync.go` (test helper, non-`_test` so the integration pkg can import
  it, mirroring `tests/unit/tuitest/tuitest.go`);
  `internal/web/{server.go,handlers.go,render.go,templates/*.html,static/*}`;
  `tests/integration/web/*_test.go`.
- **Modify:** `cmd/a9s/main.go` (mode branch + `--web`/`A9S_MODE`); `internal/tui/app.go`
  and `internal/tui/app_input.go` (shell: key→Action, render ViewState);
  `internal/tui/views/*` (rendering → pure ViewState functions; state fields removed);
  `internal/runtime/handlers_*.go` (complete event migration); `docs/architecture.md`,
  `docs/shared/quickstart.md` (the `--web` flag), `docs/development-process.md` if gates
  change.
- **Reuse (do not duplicate):** `runtime.UIIntent`/`intent.go`,
  `runtime.RuntimeState`/`state.go`, `runtime.TaskRequest`/`tasks.go`, `messages.Event`
  family, `resource.AllResourceTypes()`, existing `keys.Map`, existing Lipgloss render
  helpers in `internal/tui/views`.

## Risks & mitigations

- **PR-C is the big one** (lifting state out of the view models — `resourcelist.go`,
  `detail.go`, `yaml.go`, … — a few thousand LOC combined). Mitigate by doing
  it per-screen (menu → list → detail → text → selector), keeping the `tuitest` suite
  green after each, and treating any rendered-output diff as a regression.
- **Behavior drift during migration.** The `tuitest` harness
  (`Step`/`Render`/`StripANSI`) is the golden oracle: TUI rendered output must be
  byte-identical across PR-B/PR-C.
- **Async semantics mismatch.** The Task-queue + `HandleEvent` loop must reproduce Bubble
  Tea's "every matching view in the stack gets the intent" stack-walking (noted in
  `intent.go`). Encode that in `Controller.applyIntent` and pin it with a test.
- **Web ≠ pixel-identical to TUI** — acceptable; parity is at the *ViewState* level (same
  data, same actions), not the glyph level. That's what makes web tests meaningful for the
  TUI.

## Verification

- **Per PR:** `make ready-to-push` (`test-race`, `lint`, `security`, `gofix`,
  `verify-readonly`, `verify-zero-init`, `check-readme`, `snapshot`, `mdlint`). Note
  `ready-to-push` does **not** invoke `make build` — run it separately when a binary is
  needed.
- **TUI unchanged (PR-B/PR-C):** full `tests/unit` suite + `tuitest` rendered-output
  assertions stay green; manual `./a9s --demo` smoke check.
- **Web UI (PR-D):** `A9S_MODE=web ./a9s --demo`, open the served page, exercise menu →
  list → detail → yaml/json → related panel → a child view → filter/sort/search/copy
  across several resource types (EC2, RDS, S3, ECS, Lambda, …).
- **Web harness (PR-E):** `go test ./tests/integration/web/...` runs deterministically (no
  sleeps), driving `POST /action` and asserting on `GET /state` JSON for all resource
  types and child-views. Confirm a deliberately-induced state bug is caught by an
  assertion (proves the harness has teeth).
