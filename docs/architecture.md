# a9s Architecture Guide

> **CURRENT-STATE ARCHITECTURE.** This document describes how `main` is
> built today. It is normative for code that still lives in the current
> architecture, but it is not the target-state design document for the
> refactor program under `docs/historical/refactor/`.

This document is the first thing you should read when joining the project. It explains the runtime architecture that exists on `main` today, the constraints that current code must still honor, and the major implementation seams that the refactor plan is intentionally replacing.

For the target "no legacy / no lazy compromise" architecture and the migration plan, read:

- [`docs/historical/refactor/00-overview.md`](historical/refactor/00-overview.md) — program-level goals and invariants
- [`docs/historical/refactor/03-finding-model.md`](historical/refactor/03-finding-model.md), [`docs/historical/refactor/04-catalog.md`](historical/refactor/04-catalog.md), [`docs/historical/refactor/05-boundary.md`](historical/refactor/05-boundary.md) — active phase specs
- [`docs/historical/refactor/landed/`](historical/refactor/landed/) — archived per-PR specs for Phase 01 / 02 / 05a (preserved verbatim as landed)

Latest target-architecture additions in the refactor docs:

- cross-cutting capabilities (logs, investigation, cost, future actions) stay separate from the resource catalog
- shared query-contract types live in `core/domain`
- shared selector/matcher logic lives in `core/semantics/selector`
- runtime owns screen descriptors and background-task contracts
- new capabilities must be test-bounded, not validated by unbounded full-account crawls

## High School Student Version

If you want the simplest possible mental model, think about a9s like this:

- AWS has many "things" such as EC2 instances, S3 buckets, and RDS databases. In a9s, each one shown on screen is a `Resource`.
- A `ResourceTypeDef` is the recipe for one kind of thing: how to fetch it, which columns to show, how detail view works, and what other resources it can jump to.
- The app has one big state object, the Bubble Tea `Model`. It remembers the current screen, the current AWS profile/region, cached data, and what background work is running.
- A `View` is just a screen.
- A `Msg` is a note that says "something happened."
- A `Cmd` is background work. It talks to AWS and later sends a `Msg` back.
- A `Fetcher` loads the first version of the data.
- An `Enricher` does slower extra checks after the fast first load.
- A related checker answers "what else is connected to this thing?"

The app loop is:

1. User presses a key.
2. The active view emits a message.
3. The root model decides what to do.
4. If AWS work is needed, it runs in a background command.
5. The result comes back as another message.
6. The screen redraws.

Important interaction rules:

- Views do not call AWS directly.
- `Update()` must not block; AWS/network work goes in `tea.Cmd`.
- Old async results are dropped if the user refreshed or switched profile/region.
- Cache lifetime follows the session; switching account or region must rotate session state.
- On `main` today, behavior is driven by the declarative catalog (`core/catalog` + `core/aws/catalog_*.go`), not by `init()`/`Register*` wiring. `runtime.Core` owns the active session (`session.Session`) and the app-core dispatch; the Phase-05 extraction has **landed**. The renderer-agnostic boundary holds: all of `core/` compiles with zero Bubble Tea / Lipgloss / `internal/` dependencies, gated by `make verify-renderer-free` (a transitive `go list -deps ./core/...` check, part of `make ready-to-push`). Theme YAML is validated in the TUI adapter and handed to the runtime as a domain-safe `ParseErr`.
- The target patterns are in place: an explicit catalog, shared selectors (`core/semantics/selector`), the canonical `Finding` model, and runtime-owned screen/task contracts. Of the cross-cutting capability modules, **cost is implemented**: the Cost Explorer lives as a pure domain state machine (`core/costs` for records/store/grid/windows, `core/costs/screen` for fetch planning, drill transitions, and the computed view) consumed by a thin `core/app` adapter — typed outcomes, fetch results that carry their own authority, and one computed view shared by cursor movement, rendering, and Enter. Its invariants are specified in [`specs/021-cost-explorer/architecture.md`](../specs/021-cost-explorer/architecture.md) and enforced by a delivery-matrix table test, a TUI/web lane-parity harness, and two AST discipline gates. Logs and CloudTrail scan keep their declarative contracts (`domain.CapabilityID`, `QuerySpec`, `ScreenRegistry`) as a follow-on workstream.

## What is a9s?

a9s is a read-only terminal UI for AWS. Think k9s for Kubernetes, but for AWS services. It uses [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) (the Elm Architecture for Go) and renders with [Lipgloss v2](https://github.com/charmbracelet/lipgloss).

**Read-only by design** — a9s never makes write calls to AWS. Every AWS API call is a List, Describe, or Get operation.

---

## Architectural Direction

The codebase has a clean separation of concerns. The 020-architecture-refactor has landed: a declarative catalog, the canonical `Finding` model, a renderer-agnostic `runtime.Core`, and session-owned state. This section documents the boundaries that exist on `main` so contributors can reason about the implementation and avoid accidental cross-layer coupling.

### Current Layer Boundaries On `main`

- **`cmd/a9s`** — bootstrap only: parse flags, validate startup inputs, load config/theme, wire clients and options, start Bubble Tea.
- **`internal/tui`** — UI shell and adapter: view stack, global key handling, message routing, sizing, and transient UI state. Holds a `*runtime.Core` and reaches session-scoped state through typed `m.core.*` accessors. As the renderer adapter it legitimately imports `core/session` and `core/aws` (to supply clients and translate runtime `TaskRequest`s into `tea.Cmd`s); the shared core never imports back into `internal/tui`.
- **`core/runtime`** — platform-agnostic app core: `runtime.Core` owns the active `*session.Session` and the catalog snapshot, dispatches inbound `messages.Event`s to handlers, and returns `UIIntent` / `TaskRequest` lists for adapters to apply. It compiles with zero Bubble Tea / Lipgloss dependencies (gated by `make verify-renderer-free`, a transitive `go list -deps` check). Interactive fetch lanes are deadline-bounded: every Core fetch (`FetchResources`, load-more, filtered, child, by-IDs) runs under the 30s `fetchTimeout` in `core/runtime/fetchers.go`. The cmd/event message taxonomy and screen-builder registry have landed; `HandleEvent` takes the typed `messages.Event` interface.
- **`core/session`** — session-scoped state container (Phase 02 deliverable, **done**): `session.Session` owns per-profile/region orchestration state — the `RowStore` (the single session-scoped per-type row store; see Caching Layers), `RelatedCache`, the enrichment queues and per-type maps, every generation counter (all typed `domain.Gen` after Phase 05a-gens), and the capability stores (`PolicyStore`, `IdentityStore`, `RuleSetStore`) that replaced the deleted `core/aws/` package globals (`allPoliciesMu`, `identityCacheMu`, `sesRuleSetCacheMu`). `Session.Rotate()` is the single point that invalidates all of it on profile/region switch.
- **`core/resource`** — the stable façade API over `core/domain` + `core/catalog`, and the package most of the codebase imports (~1000 files, vs ~48 importing `core/catalog` directly). It carries type aliases (`resource.Resource = domain.Resource`, `resource.ResourceTypeDef = catalog.ResourceTypeDef`) plus substantial original logic of its own: navigation ID normalization (`NavIDFromValue`), related-panel entry and validation (`RelatedEnter`, `ValidateRelatedResult`, `FormatRelatedCount`), child-context resolution (`ResolveChildContext`), and the test-only related override registry (`relatedTestOverrides` — `SetRelatedForTest`/`AppendRelated` panic outside a test binary via a `testing.Testing()` guard; production `RelatedDef`s live on catalog literals). Canonical TYPES live in `core/domain`; the canonical declarative registry is `core/catalog`, populated by the per-category `core/aws/catalog_*.go` literals: resource types, child-view metadata, related defs, navigable fields, and fetcher/enricher declarations.
- **`core/aws`** — primarily the adapter layer: call AWS SDK APIs, transform responses into `resource.Resource`, and host a few non-UI helper subsystems that have not yet been split out. This layer should not know about Bubble Tea views.
- **`core/cache`** — persistence only: the on-disk availability/row cache — one directory per profile+region pair, one YAML file per resource type, no TTL by contract ([`design/cache-requirements.md`](design/cache-requirements.md) C1).
- **`core/demo`** — injected fake transport for development and tests, not a parallel feature architecture.

### Architectural Invariants

The invariants below are normative. Code that violates them is wrong even
if it "works" — the gen guards, registry completeness checks, and related
validators in `tests/unit/architecture_conformance_test.go` fail loudly when
any of these drift.

These are **current-state invariants**. The 020-architecture-refactor that produced them has landed: the embedded-`sessionRuntime` model is gone (session state lives in `core/session`), package-`init()` + `Register*` feature wiring is replaced by the declarative catalog (`core/catalog` + `core/aws/catalog_*.go`; zero feature-wiring `init()`, enforced by `make verify-zero-init`), the `Status`/`Issues` resource model is replaced by the canonical `Finding` model, markdown is generated (not input), shared selectors live in `core/semantics/selector/`, and screen/task boundaries are runtime-owned. Of the cross-cutting capability modules, cost is implemented (the Cost Explorer domain state machine in `core/costs` + `core/costs/screen`); logs and CloudTrail scan keep their declarative contracts (`domain.CapabilityID`, `QuerySpec`, `ScreenRegistry`) as a follow-on workstream.

1. **One root application model owns session state and orchestration.**
   `tui.Model` owns the UI shell; the session state container
   (`session.Session`) lives in `runtime.Core` and is reached through
   typed `m.core.*` accessors. The renderer-agnostic boundary holds: all of
   `core/` compiles with zero Bubble Tea / Lipgloss / `internal/`
   dependencies, gated by `make verify-renderer-free` (a transitive
   `go list -deps ./core/...` check enforced by `make ready-to-push`).
   As the renderer adapter, `internal/tui` legitimately
   imports `core/session` and `core/aws`; nothing imports back into
   `internal/tui` from the core.
2. **Views render state and emit typed messages.** Views never call AWS
   directly. `m.clients` is passed to tea.Cmds created by the root model,
   not consumed inside `View()`.
3. **The catalog is the declarative source of truth.** Supported resource
   types, related defs, navigable fields, fetchers, detail enrichers, and
   Wave 2 issue enrichers are declared as `catalog.ResourceTypeDef` struct
   literals in `core/aws/catalog_*.go`, installed once at startup via
   `aws.Install()` + `catalog.SetTypes(...)` — no package `init()` or
   `Register*` wiring (enforced by `make verify-zero-init`). There is no
   hand-maintained allowlist in dispatch code — background systems iterate
   catalog state and sort by declarative priority metadata.
4. **`core/aws` stays non-UI.** It primarily translates SDK types into
   `resource.Resource` and hosts a few helper subsystems, but it does not
   own navigation or Bubble Tea policy. It does not import `internal/tui`.
5. **Every async result carries enough identity to reject stale updates.**
   Every Msg with a `Gen` or `TypeGen` field must be stamped at dispatch
   time. Handlers MUST drop messages whose generation does not match the
   current session-wide / per-type counter.
6. **Cache invalidation is explicit.** Refresh, profile switch, and region
   switch paths all call `m.core.Session().Rotate()`: every gen counter is
   bumped and every map/queue is rebuilt in one place.
7. **Feature-specific caches do not hang off transport objects.**
   `*awsclient.ServiceClients` carries AWS clients only. Session-scoped
   caches live on `session.Session` (owned by `runtime.Core`) and reach
   detail enrichers via `*awsclient.DetailEnrichmentCtx`. Phase 02
   (`docs/historical/refactor/landed/02-session-owner.md`) deleted the legacy package-global
   caches in `core/aws/` — `allPoliciesMu` (IAM policies),
   `identityCacheMu` (caller identity), and `sesRuleSetCacheMu` (SES rule
   sets) — and replaced them with `PolicyStore` / `IdentityStore` /
   `RuleSetStore` capabilities owned by `session.Session`. `core/aws/`
   is now globals-free.
8. **Global keys are order-sensitive.** `Esc` is the back/dismiss key. `q`
   is the quit key in normal mode; it is not a navigation primitive.
   Input-mode and search-mode semantics take precedence over view-local
   bindings.

---

## Core Concepts

### The Elm Architecture (Bubble Tea)

Every interaction follows this loop:

```text
User Input → Update(msg) → (Model, Cmd) → View() → Terminal
                                ↑                      |
                                └──────────────────────┘
```

- **Model**: The entire app state is a single `tui.Model` struct (`internal/tui/app.go`)
- **Update**: Pure function — receives a message, returns new model + optional async command
- **View**: Pure function — renders model to string, no side effects
- **Cmd**: `func() tea.Msg` — runs async work (AWS calls, timers), returns a message

**Critical rule**: No blocking I/O in `Update()`. All AWS calls, timers, and network operations go in `tea.Cmd` closures.

### Messages

Views communicate exclusively via typed messages (`core/runtime/messages/`, with `cmd.go` for UI→core commands, `event.go` for core→UI events, and `messages.go` carrying the `Cmd` / `Event` / `GenStamped` marker interfaces). Views never import each other. The root `Model.Update()` routes messages to the appropriate handler.

Key messages (the canonical taxonomy lives in `core/runtime/messages/{cmd,event}.go` as suffixless `Cmd`/`Event` types — e.g. `AvailabilityChecked`, `EnrichmentChecked`; the `*Msg` names below are the TUI-adapter/legacy forms):

| Message | Purpose |
|---------|---------|
| **Navigation** | |
| `NavigateMsg` | Push (or replace) a view — detail, YAML, JSON, etc. `ReplaceCurrent` swaps instead of stacking |
| `PopViewMsg` | Request current view dismissal (emitted by HelpModel, IdentityModel on keypress) |
| `EnterChildViewMsg` | Open a child resource list (e.g., S3 objects) |
| `RelatedNavigateMsg` | Navigate to a related resource type (from navigable field or CloudTrail pivot) |
| **Resource loading** | |
| `ResourcesLoadedMsg` | Deliver fetched resources to a list view |
| `LoadResourcesMsg` | Trigger async resource fetch |
| `LoadMoreMsg` | Trigger next-page fetch (pagination) |
| `RefreshMsg` | Trigger re-fetch of current list (Ctrl+R) |
| **Session & identity** | |
| `InitConnectMsg` | Trigger AWS session setup |
| `ClientsReadyMsg` | AWS clients connected and ready |
| `IdentityLoadedMsg` | Caller identity result for status bar |
| `IdentityErrorMsg` | Caller identity fetch failure |
| `ProfileSelectedMsg` | User confirmed profile selection |
| `RegionSelectedMsg` | User confirmed region selection |
| `ThemeSelectedMsg` | User confirmed theme selection |
| **Enrichment & related** | |
| `EnrichDetailMsg` | Start async detail enrichment (e.g., policy doc fetch) |
| `EnrichDetailResultMsg` | Deliver enriched resource back to detail/YAML/JSON view |
| `RelatedCheckStartedMsg` | Start async related-resource checks |
| `RelatedCheckResultMsg` | Deliver one related-check result to detail view — carries `Result.Err` (checker failure), `LazyAddError` (FetchByIDs failure; a panicking checker is recovered by the dispatch `tea.Cmd` in `internal/tui/runtime_adapter_related.go` and surfaced here too, with an `UnknownRelated` result), `LazyAddedResources` (out-of-scope targets resolved via `FetchByIDs`), `CachedPages` (cold-miss prefetch); app handler routes errors to `FlashMsg{IsError:true}` so the `!` error log captures them |
| **Availability & Issue Counts** | |
| `AvailabilityCacheLoadedMsg` | Deliver disk-cached availability + issue count data (includes `IssueCounts`, `IssueKnown` maps) |
| `AvailabilityPrefetchedMsg` | No-cache-mode availability + issue counts + retained resources for Wave 2; `PrefetchErr` carries per-type fetch failures aggregated across all registered paginated fetchers, surfaced via FlashMsg |
| `AvailabilityCheckedMsg` | One resource type's background probe result (includes `Issues` count + retained `Resources`) |
| `EnrichmentCheckedMsg` | One resource type's Wave 2 enrichment result (truncated flag + per-resource `Findings` / `AttentionDetails` / `FieldUpdates` / `TruncatedIDs` maps; dual-generation guard via `Gen` + `TypeGen`). It carries no issue count — the menu badge is recomputed by `unifiedIssueCount` (`core/runtime/handlers_availability.go`) over the freshly folded rows |
| **UI feedback** | |
| `FlashMsg` | Show a temporary status/error message |
| `ClearFlashMsg` | Auto-clear flash after timer |
| `APIErrorMsg` | AWS API call failure |
| `CopiedMsg` | Clipboard copy success |
| `ValueRevealedMsg` | Deliver revealed secret/parameter value |

### View Stack

The app maintains a stack of views (`stack []views.View`):

```text
[MainMenu] → [ResourceList] → [DetailModel] → [YAMLModel]
   ↑ bottom                              top ↑ (activeView)
```

- `pushView(v)` — append to stack
- `popView()` — remove top (Esc pops; `q` quits the app)
- `activeView()` — `stack[len(stack)-1]`, receives all messages via `updateActiveView()`

**View replacement** (`ReplaceCurrent`): `NavigateMsg` has a `ReplaceCurrent bool` field. When true, `handleNavigate` calls `popView()` before `pushView()`, effectively swapping the current view. This is used for inter-view navigation between Detail, YAML, and JSON — switching from YAML to JSON replaces in-place (`list → detail → JSON`) rather than stacking (`list → detail → YAML → JSON`). Esc always returns to the view underneath (detail), not to the previous sibling view.

Views are created in `handleNavigate()` and pushed immediately. Async data arrives later via messages.

---

## Project Structure

```text
cmd/
  a9s/              # main binary — CLI flags, tea.NewProgram (--demo, --web)
  readmegen/        # generates README.md from docs/README.tmpl.md + docs/shared/
  viewsgen/         # generates .a9s/views/*.yaml from built-in defaults
  refgen/           # generates views_reference.yaml from AWS SDK struct reflection
  preview/          # renders static TUI design mockups (no AWS)
  catalogen/        # catalog codegen
  snapshot/         # web-e2e snapshot collector
  checklist/        # web-e2e checklist oracle

core/            # platform-agnostic core — renderer-free, gated by `make verify-renderer-free`
  app/           # headless controller (Controller) — shared list/detail/menu/cost state+render, consumed by both tui/ and web/
  aws/           # AWS service clients, resource fetchers, related checkers, enrichers, catalog_<category>.go type defs
  buildinfo/     # version resolution (ldflags at build time)
  cache/         # on-disk availability/row cache — one dir per profile+region pair, one YAML file per type, no TTL (see Caching Layers)
  catalog/       # canonical resource catalog: ResourceTypeDef + FindingDef, aggregated from `core/aws/catalog_*.go` literals, installed via `aws.Install()` + `catalog.SetTypes(...)`. The sole source of truth; the legacy `Register*` registry is gone.
  config/        # YAML config loading, built-in defaults per service category (defaults_<category>.go)
  consolelink/   # AWS console deep-link URL builders (partition-aware Regional/Global/GoView, https+console-domain Valid guard, Resolve with the generic /go/view ARN fallback) — backs the o/O keys
  costs/         # Cost Explorer domain state machine (records/store/grid/windows); costs/screen for fetch planning + drill
  demo/          # synthetic fixture data for --demo mode
    fixtures/    #   per-service Go structs (ec2.go, iam.go, etc.)
    fakes/       #   per-service fake API implementations
  domain/        # leaf type-declaration package: Resource, Type, Severity, FindingCode, Finding, AttentionDetail, Color, Gen, plus query-contract types. Introduced in Phase 01 (`docs/historical/refactor/landed/01-projection-hook.md`); `Gen` added in Phase 05a-gens.
  fieldpath/     # struct field extraction via reflection (frozen — don't modify)
  jsonyaml/      # renderer-free JSON→YAML helpers (used by projection without pulling in lipgloss)
  resource/      # stable façade over domain/ + catalog/ — type aliases plus original helpers (NavIDFromValue, RelatedEnter, ResolveChildContext) and the test-only related override registry
  runtime/       # platform-agnostic app core: Core (orchestrator.go), handlers, screens, tasks, state, intents; 30s fetchTimeout on interactive fetch lanes (fetchers.go)
    messages/    #   typed Cmd/Event message taxonomy (cmd.go, event.go, messages.go marker interfaces)
  semantics/     # shared semantic helpers: projection (DetailProjector), ctevent (CloudTrail event summarization), selector (shared ARN/tag matching)
  session/       # session.Session — all session-scoped mutable state + capability stores; Rotate() invalidates in-flight gens
  web/           # web mode: HTTP server rendering the same controller state (server.go, templates/, static/)

internal/
  tui/           # Bubble Tea adapter shell — GPL-3.0-or-later only; as the renderer adapter it imports core/session and core/aws to supply clients and translate runtime TaskRequests into tea.Cmds
    keys/        #   key bindings (single Map struct, one file)
    layout/      #   frame rendering (borders, title, status line)
    styles/      #   Tokyo Night Dark palette, theming system
    text/        #   text utilities (PadOrTrunc for column rendering)
    views/       #   all view models (see View Types below)

tests/
  unit/          # black-box behavior tests, package `unit` (run via `make test`; `make test-race` adds -race). White-box tests live in-package next to the code — see Test Architecture.
  integration/   # gated by //go:build integration
  testdata/      # hand-crafted JSON fixtures — AWS SDK response bodies for fetcher unit tests (no live AWS)
```

---

## Resource Model

This section describes the current resource model on `main`. It is intentionally conservative: it explains the struct the codebase uses today, not the canonical finding model planned in [`docs/historical/refactor/03-finding-model.md`](historical/refactor/03-finding-model.md).

```go
// core/domain/resource.go   (Phase 01 moved the struct out of core/resource;
//                                core/resource/resource.go is a thin alias file:
//                                `type Resource = domain.Resource` plus the DedupByID helper.)
type Resource struct {
    ID               string                          // primary identifier (instance ID, ARN, name)
    Name             string                          // display name (from Name tag or identifier)
    Type             string                          // resource short name ("ec2", "rds", ...); empty = unknown
    Fields           map[string]string               // pre-extracted column values, snake_case keys ("instance_id", "vpc_id")
    RawStruct        any                             // original AWS SDK typed struct (e.g., ec2types.Instance) for reflection-based detail rendering
    Findings         []Finding                       // canonical finding list; populated by fetchers/catalog finding tables (Wave 1) and by Wave 2 (`applyEnrichment`)
    AttentionDetails map[FindingCode]AttentionDetail // structured detail rows keyed by stable `FindingCode`; consumed only by detail view's Attention section
}
```

- **Type** — short-name field added in Phase 01 (`docs/historical/refactor/landed/01-projection-hook.md`) so that downstream packages (semantics, projection) can route by type without re-deriving it.
- **Findings** — the canonical resource-health surface (`domain.Finding{Code, Phrase, Detail, Severity, Source}`; Phrase = short S4 cause, Detail = optional richer S5 sentence). Drives row coloring, list-view status display, menu issue badges, and the ctrl+z attention filter. Wave 1 entries carry `Source = "wave1"`; Wave 2 entries carry `Source = "wave2:<short>"` and are written by `applyEnrichment` in `internal/tui/app_enrich_fold.go`. The legacy `Status string` / `Issues []string` fields and the per-enricher `Bump/StripFindingSuffix` algebra are gone; the `(+N)` multi-finding suffix is derived centrally from `Findings` by `listPhraseFromFindings` (`core/app/list_columns.go`).
- **AttentionDetails** — supporting facts (rows shown in the detail-view Attention section) keyed by stable `FindingCode`. `FindingCode` is never displayed.
- **Fields** — flat key-value pairs populated by each fetcher. Used for list table columns and simple detail rendering. Keys are snake_case (e.g., `"instance_id"`, `"vpc_id"`).
- **RawStruct** — the actual AWS SDK struct (e.g., `ec2types.Instance`, `s3types.Bucket`). Used by detail/YAML/JSON views via reflection for deep field path traversal (e.g., `"State.Name"`, `"Placement.AvailabilityZone"`).

### Resource Type Registration

Resource types are registered declaratively through the catalog: each type is one `catalog.ResourceTypeDef` struct literal in a per-category `core/aws/catalog_*.go` file, aggregated by `core/catalog` and installed once at startup via `aws.Install()` + `catalog.SetTypes(...)`. There is no `init()`/`Register*` feature wiring — fetchers, enrichers, related defs, navigable fields, field keys, and aliases are all direct fields on the struct literal (see the `ResourceTypeDef` shape below). Adding a resource type is the mechanical four-file change described in [`docs/historical/refactor/00-overview.md`](historical/refactor/00-overview.md) (catalog literal + transport + demo fixture + tests); `make verify-zero-init` enforces zero feature-wiring `init()`.

### Resource Type Definitions

```go
// core/catalog/types.go    (canonical home; `core/resource/types.go`
//                               re-exports as `type ResourceTypeDef = catalog.ResourceTypeDef`
//                               for zero-churn backward compat.)
type ResourceTypeDef struct {
    // Identity & display
    Name        string           // "EC2 Instances" — display name
    ShortName   string           // "ec2" — colon-command alias and registry key
    Aliases     []string         // alternative command names
    Category    string           // main-menu group (e.g., "COMPUTE")
    ListTitle   string           // overrides ShortName in list frame titles; empty = use ShortName
    TitleOmitsID bool            // detail title renders "detail -- <Name>" for opaque synthetic IDs
    CostExplorerServiceName string // Cost Explorer SERVICE dimension for the cost drill-down; empty = unsupported
    Columns     []domain.Column  // table columns for list view
    LifecycleKey string          // Fields key holding lifecycle state; defaults to "state"
    IdentityKey string           // column key for enrichment row-marker placement; empty = use the IdentityColumnIndex cascade
    CellDecorators map[string]func(domain.Resource, string) string // transforms cell values per column before render
    CopyField   string           // overrides which Fields key `c` copies; empty = copy ID
    ConsoleURL  func(domain.Resource, string, string) string // AWS console deep link for a row (region, accountID); "" = no page / missing input, Resolve falls back to Fields["arn"] via /go/view

    // Behavior — fetchers and enrichers are struct fields, not registrations
    Fetcher             domain.PaginatedFetcher  // Wave 1 paginated fetcher
    AvailabilityFetcher domain.PaginatedFetcher  // optional cheaper probe-only fetcher; nil = use Fetcher
    Wave2               any                      // aws.IssueEnricher{Fn, Priority}; nil = no Wave 2 signal
    Project             domain.DetailProjector   // custom projector; nil = projection.GenericWithConfig fallback
    Related             []domain.RelatedDef      // right-column related-panel defs (contract: docs/related-resources.md)
    Navigable           []domain.NavigableField  // detail-view field → target-type navigation
    Children            []domain.ChildViewDef    // child views triggerable from the list (key bindings)
    Reveal              domain.RevealFetcher     // secret reveal (`x` key); nil = no reveal
    DetailEnrich        domain.DetailEnricher    // on-demand detail enricher; nil = none
    FieldKeys           []string                 // valid Resource.Fields keys the Wave 1 fetcher produces
    FieldAliases        map[string]string        // source field key → alias key copied by ApplyFieldAliases
    FetchByIDs          domain.FetchByIDsFunc    // by-ID fetch, bypassing pagination (lazy related adds)
    FilteredFetcher     domain.FilteredPaginatedFetcher // server-side filtered page fetch
    IssueEnricherFieldKeys []string              // Fields keys the Wave 2 enricher writes via FieldUpdates
    ChildFetcher        domain.PaginatedChildFetcher    // set on child-type entries via catalog.SetChildTypes

    // Cross-cutting
    CloudTrailKey string         // "LookupAttr:ValueSource" for CloudTrail pivot; empty = no `t` key
    ExcludeFromIssueBadge bool   // rows still colored + ctrl+z visible, but excluded from menu badge (used by ct-events)
    StubCreator func(string) domain.Resource // builds a minimal stub for auto-navigate when cache is empty
    RelatedContextFromIDs func([]string) map[string]string // extracts parent context for related-panel navigation

    // Color, augmentation, findings
    Color    func(domain.Resource) domain.Color // REQUIRED: classifies row health; findings-first, raw fields only as fallback
    Augment  domain.Augmenter                   // optional post-projector section hook (e.g. EC2 status checks)
    Findings []FindingDef                       // declarative finding-code table: {Code, Phrase, Severity, Source}
}
```

`Columns []domain.Column` is the source for a list column's identity: which columns a type has, what each cell reads (`Key`), and the order they are built in. The built-in view defaults (`core/config/defaults_*.go`) declare the same columns a second time, because a view file on disk is what an operator edits: the defaults may add columns the catalog does not declare, set the widths an operator will adjust, and carry the rendering hints the catalog literal has no field for (`Path`, `SortKey`, `Humanize`). `resource.ResolveListColumnCascade` merges the pair through one function, `mergeListColumn`, so both of its arms read the two declarations the same way — the catalog's `Key` wins wherever it declares one, the defaults supply the hints and their own columns. `tests/unit/misc4_r8_columns_agree_test.go` holds the overlap honest: for every title both declare, the two arms must render the same cell over every demo row, and hold one order.

`Color func(domain.Resource) domain.Color` is part of the type definition and drives row classification. Classifiers resolve color findings-first via `colorFromAnyFinding` (worst finding severity wins, wave1 or wave2); raw-field branches survive only as fallback for rows that carry no findings. Color, the Status cell text and the detail Attention block therefore share one source — the row's findings — which is enforced by the color-vs-findings conformance gate (`tests/unit/qa_color_findings_conformance_test.go`, empty allowlist). The function returns the renderer-free `domain.Color` health enum; the TUI maps it to a concrete style via `styles.ColorStyle` at render time.

Resource types are installed once at startup via `aws.Install()` + `catalog.SetTypes(...)`, aggregating the per-category `core/aws/catalog_*.go` literals. Categories map to type definition files:

| File | Category |
|------|----------|
| `catalog_compute.go` | Compute (EC2, Lambda, EKS Clusters, ASG, Elastic Beanstalk, EBS) |
| `catalog_containers.go` | Containers (EKS Node Groups, ECS Clusters, ECS Services) |
| `catalog_networking.go` | Networking (VPC, Subnet, SG, ELB, TG, IGW, NAT, Route Tables, ACM, API Gateway, WAF) |
| `catalog_databases.go` | Databases & Storage (RDS, S3, Redis, OpenSearch, DynamoDB, Redshift, MSK, EFS, Kinesis) |
| `catalog_security.go` | Security & IAM (IAM Roles, Users, Policies, Groups, KMS) |
| `catalog_secrets.go` | Secrets & Config (Secrets Manager, SSM Parameter Store) |
| `catalog_monitoring.go` | Monitoring (CloudWatch Alarms, Log Groups, CloudTrail) |
| `catalog_messaging.go` | Messaging (SNS, SQS, EventBridge Rules, Step Functions) |
| `catalog_cicd.go` | CI/CD (CloudFormation, CodePipeline, CodeBuild, ECR) |
| `catalog_dns_cdn.go` | DNS & CDN (Route 53, CloudFront) |
| `catalog_data.go` | Data & Analytics (Athena, Glue) |
| `catalog_backup.go` | Backup (AWS Backup, SES) |

---

## Fetcher Patterns

All registered via `catalog.ResourceTypeDef` literals in `core/aws/catalog_*.go`, implemented in `core/aws/*.go`:

| Pattern | Signature | Use Case |
|---------|-----------|----------|
| **PaginatedFetcher** | `func(ctx, clients, token) (FetchResult, error)` | Top-level resource lists (EC2, S3, RDS, etc.) |
| **PaginatedChildFetcher** | `func(ctx, clients, parentCtx, token) (FetchResult, error)` | Child resource lists (S3 objects, role policies, ECS tasks) |
| **FilteredPaginatedFetcher** | `func(ctx, clients, filter, token) (FetchResult, error)` | Server-side filtered queries (CloudTrail events) |
| **RevealFetcher** | `func(ctx, clients, resourceID) (string, error)` | On-demand secret reveal (`x` key — Secrets Manager, SSM) |
| **DetailEnricher** | `func(ctx, clients, Resource) (Resource, error)` | On-demand detail enrichment (policy documents) |
| **IssueEnricherFunc** | `func(ctx, *ServiceClients, []Resource) (IssueEnricherResult, error)` | Wave 2 issue enrichment; `IssueEnricherResult` carries `Truncated`, `TruncatedIDs`, and per-resource `Findings` / `AttentionDetails` / `FieldUpdates` maps — no issue count field; the badge count is derived by `unifiedIssueCount` (`core/runtime/handlers_availability.go`) |

Each fetcher takes `clients any` and type-asserts to `*aws.ServiceClients` internally. This allows tests to inject mocks.

**Throttling protection** (`core/aws/retry.go`): `RetryOnThrottle[T any](ctx, cfg, fn)` wraps an AWS API call with exponential backoff for `ThrottlingException` / `Throttling` / `RequestLimitExceeded` errors. Fetchers and enrichers that iterate per-resource (e.g., `EnrichTargetGroupHealth` calling `DescribeTargetHealth` once per TG) wrap each call in `RetryOnThrottle` so a throttled slice still completes instead of returning a half-populated result. Non-throttling errors are returned immediately without retry.

### Wave 2 Issue Enrichment Pipeline

Some resource types hide problems behind extra API calls (e.g., EC2 with impaired status checks, RDS with pending maintenance). Wave 2 enrichment discovers these hidden issues after Wave 1 probes complete.

This section documents the current Wave 2 implementation on `main`. The refactor plan in [`docs/historical/refactor/03-finding-model.md`](historical/refactor/03-finding-model.md) and [`docs/historical/refactor/04-catalog.md`](historical/refactor/04-catalog.md) replaces this registry-and-markdown-driven model with canonical findings and catalog-owned metadata.

**Architecture:**
- `core/aws/issue_enrichment.go` — Wave 2 shared types and helpers: `NoOpIssueEnricher`, `IssueEnricher` struct, `IssueEnricherFunc` / `IssueEnricherResult` types, shared helpers, `EnrichmentCap` / `PerParentPageCap`. As of AS-795n, the package-init `IssueEnricherRegistry` map and `registerIssueEnricher` helper are gone — registrations now live on each `catalog.ResourceTypeDef` literal's `Wave2` field.
- `core/aws/wave2.go` — read API over the catalog: `Wave2EnricherFor(shortName) (IssueEnricher, bool)` and `AllWave2() []Wave2Entry`. Also exposes `SetWave2EnricherForTest` / `DeleteWave2EnricherForTest` for the test-override map used by `tests/unit/`.
- `core/aws/catalog_*.go` — per-category catalog literals. Each entry's `Wave2` field carries an `IssueEnricher{Fn:..., Priority:...}`, and `IssueEnricherFieldKeys` lists the `Fields` keys the enricher writes via `IssueEnricherResult.FieldUpdates`. Types with `NoOpIssueEnricher` are explicit placeholders for in-fetcher Wave 2 work.
- `core/runtime/probes.go` — `(*Core).BuildEnrichQueue()` (queue construction lives on `runtime.Core`).
- `internal/tui/probe_adapter.go` — `(*Model).probeEnrichment()` — the TUI-side `tea.Cmd` wrapper that dispatches enrichers and emits `EnrichmentChecked` messages.
- `core/runtime/handlers_availability.go` — `(*Core).startEnrichment()` (builds the queue, dispatches the initial window) and `(*Core).handleEnrichmentChecked()` (folds one Wave-2 result onto the type's cached rows, rebases the probe status on the Wave-1 availability baseline, recomputes the menu badge via `unifiedIssueCount`, and refills the next queued type).

**Flow:**

```text
Wave 1 probes complete
  → startEnrichment() builds queue from awsclient.AllWave2() ∩ observed RowStore types
  → dispatches the first enrichDispatchWindow (4) types; the rest stay queued in
    session.EnrichQueue and each EnrichmentChecked completion refills exactly one
    (the refill branch in handleEnrichmentChecked) — dispatch is windowed, never unbounded
  → EnrichmentCheckedMsg arrives
    → findings/FieldUpdates fold onto the type's cached rows (applyEnrichment + AmendRows).
      A result replaces only the rows it answered for: the ids it reports as
      uninspected (`TruncatedIDs`), and every id when the probe came back
      empty-handed with an error, keep the Wave-2 state they already have.
      That decision is made once, in the runtime, and rides out on the
      `ListEnrichmentPatch` — `runtime.FoldWave2Rows` is the single fold both
      the stored rows and the rows on screen (`Controller.applyRowFindings`)
      go through, so a row that keeps its glyph in the file keeps it on the
      list, and neither surface re-decides which rows a result speaks for.
      The same set governs the on-screen carry across a later fetch
      (`applyResourcesLoaded`): a row the latest result answered for is
      re-folded from that result, and a row it did not answer for carries its
      previous Wave-2 findings onto the incoming row, since nothing has
      re-checked it and the re-fold has nothing to give it back
    → menu badge recomputed via unifiedIssueCount over the folded rows (a healed issue clears)
    → progress indicator updated
  → all done: save cache with enriched rows/findings (when caching enabled; the types the sweep actually answered for are named in `SaveCachePayload.Wave2Answered` and supersede their carried Wave-2 data, a type whose probe failed keeps what the file already knows);
    the RowStore retains the enriched rows for the session
```

**Registry**: Wave 2 capability is declared on each `catalog.ResourceTypeDef` literal's `Wave2` field (`IssueEnricher{Fn, Priority}`); a type with no Wave 2 signal simply omits the field — `Wave2EnricherFor` returns `ok=false` for it. Some types without a `Wave2` enricher still perform in-fetcher Wave 2 work — their fetchers already make per-resource Describe calls and populate health fields at fetch time (e.g., EKS `health_issues_count`, CloudTrail `is_logging`, OpenSearch `cluster_health`; since v3.50.x this list-then-describe-each shape is the standard for new types whose pivots live on the describe response: mwaa `GetEnvironment` per environment, transfer `DescribeServer` per server, lt one `DescribeLaunchTemplateVersions("$Default")` per template).

**Honest degradation (details denied)**: when the list API names a resource but the per-item describe fails (IAM denial, nil response), the row is KEPT — never dropped — through one shared implementation (`core/aws/degraded_resource.go`: `DetailsDeniedCode`/`DetailsDeniedFindingDef`/`DegradedDetailsDenied`). The row carries a `details denied` Warning finding; rich variants (transfer, lt) keep every list-borne field, name-only variants (mwaa, eks, ng, ddb, opensearch) keep the identity. Per-item failures aggregate into the E3 composite error alongside the partial rows (E5). Every new type with a describe step must adopt this contract — see the extension guide.

**Cache-scan enrichers (zero-API Wave 2)**: a second enricher shape derives findings by scanning a SIBLING type's already-loaded cache instead of calling AWS — `snapshot_cross_ref.go` (dbi-snap/dbc-snap orphan and past-retention), `lt_issue_enrichment.go` (deprecated AMI via the ami cache), `vpcpeer_issue_enrichment.go` (missing/blackholed routes via the rtb cache). They read the cache through the shared `cachedTypedRows[T]` helper (core/aws/related_common.go) under the same tri-state contract as cache-scan related checkers, and emit findings through the normal `IssueEnricherResult` path. Guard rule: an absent or truncated sibling cache produces NO findings — never a guess.

**Priority order** (`(*Core).BuildEnrichQueue`): Batchable enrichers that make account-wide calls are dispatched first (e.g., RDS/DocDB maintenance, EC2 instance status). Per-resource enrichers (e.g., DynamoDB PITR, KMS rotation, S3 PAB) iterate over resource IDs/ARNs, capped at `EnrichmentCap` (50). The registry key for each enricher must match the `ShortName` Wave 1 uses when observing rows into the `RowStore` — a mismatch silently skips the enricher. Queue membership comes from the store: a type enriches if its entry was observed at all (`TypeRows.Gen != 0` — observed-empty types still enrich); `Partial`-only entries never enter the queue.

**Resource identity**: Enrichers receive the session `RowStore`'s retained rows for the type (seeded by Wave 1 probes with first-page resources — the role the deleted `session.ProbeResources` map used to play). Account-wide enrichers make a single API call covering all resources. Per-resource enrichers fan out to individual resources, capped at `EnrichmentCap` (50).

**Current contract on `main`**: [`docs/attention-signals.md`](attention-signals.md) is the hand-maintained human reference for Wave 1 (Color func) and Wave 2 (issue-enricher) assignments per resource type; it is no longer machine-parsed. The catalog is the source of truth: `tests/unit/architecture_conformance_test.go` (`TestConformance_EveryCatalogWave2ResolvesThroughAccessor`, `TestConformance_Wave2Registry_IsNonEmpty`) enforces Wave-1 `Color` / Wave-2 enricher completeness directly from catalog data.

**Skip condition**: Wave 2 runs only when `isDemo=false`. Demo mode has no real AWS to query. `--no-cache` on live AWS still runs Wave 2 (it only disables disk persistence, not capabilities).

**Lifecycle**: the `RowStore` retains its rows for the whole session — there is no post-Wave-2 memory free, which is why a warm list open after the sweep seeds instantly — and is cleared only on profile/region switch (`Rotate()`). On a top-level resource list with a registered enricher, Ctrl+R bumps `Session.EnrichmentTypeGen[rt]`, clears `Session.EnrichmentRan[rt]`, calls `SetEnrichmentState(0, false, nil, nil)` on the active list, and dispatches `refreshActiveListWithEnrichmentRerun` (`internal/tui/runtime_adapter_navigate.go`) to rerun Wave 2 for that type. The main-menu Ctrl+R path invalidates Wave 2 for all types: it bumps `Session.EnrichmentGen` (the session-wide generation counter), resets `EnrichmentTypeGen` to an empty map, clears `EnrichmentRan` — then reloads the cache from disk. Wave 2 re-runs when the user next navigates to a resource list.

### Issue-Enrichment Visibility Subsystem

After Wave 2 issue enrichment runs, findings are surfaced in list and detail views.

**Types:** the canonical carrier is `domain.Finding` (`core/domain/finding.go`) — `Code` (stable `FindingCode`, never displayed), `Phrase` (short S4 cause), `Detail` (optional S5 operator sentence), `Severity` (`domain.Severity`), `Source` (`"wave1"` | `"wave2:<short>"`) — with supporting rows in `domain.AttentionDetail`.

**List view integration:**
- `ResourceListModel.SetEnrichmentState(issueCount int, truncated bool, findings map[string][]domain.Finding, details map[string]map[domain.FindingCode]domain.AttentionDetail)` — stores Wave 2 results; called on arrival and with zeroed args on Ctrl+R rerun start (`internal/tui/runtime_adapter_navigate.go`).
- Findings reach the visible row through the S2/S4 surfaces: row **color** and the Status-cell **phrase** (with its `(+N)` multi-finding suffix) — both derived from the row's `Findings` slice by the shared controller (`listPhraseFromFindings` et al., `core/app/list_columns.go` / `list_body.go`) and consumed by both TUI and web. Row color is the worst severity across **all** findings (the color-findings-conformance rule, `tests/unit/qa_color_findings_conformance_test.go`), so any `!`/`~` finding colors the row off-green rather than annotating a green row; the `!`/`~` glyph is the per-entry marker inside the detail-view Attention section (S5), not a live list-row surface. There is no banner and no row-marker dot — those surfaces were removed with the S1–S5 visualization contract (`docs/attention-signals.md` §Visualization Surfaces).

**Detail view integration:**
- `DetailModel.SetEnrichmentFinding(f *domain.Finding, ad *domain.AttentionDetail)` (`internal/tui/views/detail_helpers.go`) — injects (or, with `nil`, clears) the finding in the unified detail-view Attention section (`injectAttentionSection`, `internal/tui/views/detail_fields.go`).

**Stacked-view live-update pattern:**
- The runtime's `handleEnrichmentChecked` emits `PatchResourceList` and `PatchDetail` intents scoped to the affected type (`PatchDetail` with an empty `ResourceID` means "all open detail views of this type"). The adapter applies them to every matching view in the stack, not just the active one, so a user can navigate away to a detail view while Wave 2 runs and both the list (behind) and the detail receive the findings without requiring a re-open.

**Current-state ownership (Phase-05, AS-237)**: Session-scoped state lives exclusively in `session.Session`, owned by `runtime.Core` and accessed from `tui.Model` via `m.core.Session()`. The `tui.Model` struct holds only pure UI-shell state (view stack, input mode, flash, tab completion). Profile/region switches call `m.core.Session().Rotate()` which bumps every generation counter and rebuilds the maps — in-flight async messages tagged with the pre-switch gens are then rejected by the handlers' gen guards.

Representative fields on `session.Session` (`core/session/session.go`):
- `EnrichmentRan map[string]bool` — banner visibility signal; `true` only after Wave 2 completed for that type.
- `EnrichmentTypeGen map[string]domain.Gen` — per-type Wave 2 generation counter; bumped on Ctrl+R rerun to invalidate stale in-flight results.
- `EnrichmentTruncatedIDs map[string]map[string]bool` — per-type set of resource IDs the enricher could not inspect: an API error on that row, a per-parent page cap, or the tail past `EnrichmentCap`. It is the ONLY store for that fact; `Controller.listUninspectedIDs` reads it for both rendered surfaces (the list Status cell reads `not inspected`, the detail Attention block gains a `Not inspected` entry), so the two can never disagree. `capAtEnrichmentCap` is the one place a work list is trimmed to the cap, precisely so the dropped rows land here instead of rendering as inspected-and-healthy.
- `ConnectGen`, `EnrichmentGen`, `AvailabilityGen`, `DetailOpGen` — per-purpose session-wide generation counters; guard stale in-flight async results. `DetailOpGen` doubles as the `DetailOperation` ID source: the detail view's entire open/refresh lifecycle (enrichment + related checks) runs under one operation identity rather than separate counters. **All four are seeded at 1, never 0** — see "Why generation counters?" for why the seed is load-bearing rather than cosmetic.
- `RowStore *session.RowStore` — the single session-scoped per-type row store (see Caching Layers); `RelatedCache` — the related-check LRU. Both cleared on `Rotate()`.
- Sparse related-panel lazy adds — a checker emits IDs outside the top-level fetcher's scope filter (e.g. AWS-managed KMS key, public AMI, IAM `AdministratorAccess`) — land as `Partial` `RowStore` entries via `ObservePartial`. `handleRelatedNavigate` reads one store entry per type (full-beats-partial is a flag on the entry, not a two-map merge), and partial-only entries are visible to related checkers — that is their purpose — but NEVER eligible for the main-menu top-level list, the enrich queue, navigation seeds, or disk saves.

**Wave 2 findings (where they live):** PR-03a-fold deleted the parallel `EnrichmentFindings map[string]map[string]resource.EnrichmentFinding` map from `session.Session`. Wave 2 findings are now written onto each cached `resource.Resource.Findings` slice (with `Source` prefix `wave2:`) and `r.AttentionDetails`, via `applyEnrichment` in `internal/tui/app_enrich_fold.go`, which folds them into the `RowStore` through `Core.AmendRows`' copy-on-write mutation — findings and field updates apply exactly once, at the store. Reads use `findingFromResource` / `findingsFromRows` against the cached rows. The runtime's view-ready snapshot surface (`runtime.RuntimeState.EnrichmentFindings`, `core/runtime/state.go`) and the `PatchDetail.EnrichmentFindings` intent payload (`core/runtime/intent.go`) carry per-resource findings out to adapters, but neither replaces the cached-row authority — they are derived from it.

---

## View Types

All in `internal/tui/views/`:

| View | File(s) | Purpose |
|------|---------|---------|
| **MainMenuModel** | `mainmenu.go` | Category-grouped resource list with availability badges, issue count badges (`issues:N`), ctrl+z quad-state filter, enrichment progress indicator |
| **ResourceListModel** | `resourcelist.go` | Paginated table with filter, sort, child drill-down; owns its ctrl+z attention-only toggle; tracks `issueCount` for the frame-title `!N` issue suffix (`s3(50+) !5`; `!N+` when truncated, omitted when zero or in ctrl+z attention-only mode) and menu sync-back |
| **DetailModel** | `detail.go`, `detail_fields.go`, `detail_helpers.go` | Two-column: field list (left) + related panel (right) |
| **YAMLModel** | `yaml.go` | YAML dump of RawStruct with syntax highlighting + search. Also doubles as a raw-text viewer via `NewTextViewer()` (used for the `!` error log) |
| **JSONModel** | `json.go` | JSON dump of RawStruct with syntax highlighting + search |
| **RevealModel** | `reveal.go` | Displays revealed secret/parameter value |
| **HelpModel** | `help.go` | Context-sensitive key binding reference |
| **SelectorModel** | `selector.go` | Generic list picker (profile, region, theme selection) |
| **IdentityModel** | `identity.go` | Shows `sts:GetCallerIdentity` result |

Views are concrete models with no shared interface: each returns its own concrete type from its update method, and the root model's dispatch (`updateActiveRS`) type-switches on the active renderer state to route and write back.

Cross-view behaviors that older revisions expressed as per-view capability interfaces now live in the renderer-agnostic snapshot, so the TUI and web renderers consume one truth:

| Behavior | Source of truth |
|----------|-----------------|
| Filter / search state | `ViewState` body fields (`ListBody.Filter`, `DetailBody.Search`, `TextBody.Search`, …) populated by the controller |
| Footer key hints | `core/app` footer builders (`buildListFooterHints`, `MenuFooterHintsFor`, `CostsFooterHintsFor`) via `ViewState.Footer` |
| Copy content (`c`) | `Controller.CopyContent()` (`core/app/copy.go`) — one resolution for list/detail/text/identity, exposed as `ViewState.CopyText`/`CopyLabel`; the TUI's `handleCopy` delegates to it, the web client reads the rendered `data-copy-*` attributes. Only the reveal screen's copy stays adapter-local (the controller has no reveal screen). |
| Console link (`o`/`O`) | `Controller.ConsoleTarget()` (`core/app/snapshot.go`) — one target resolution for list (incl. child lists), detail, and a focused single-target related row (full cached row via `Core.AnyLaneResourceByID`, then `StubCreator`, then bare ID); URL built by `consolelink.Resolve` + `Valid` guard, exposed as `ViewState.ConsoleURL`/`IsDemo`. The TUI's `handleOpenConsole` delegates to it and execs the opener ($BROWSER argv-split, then per-GOOS, never a shell); the web client reads `data-console-url`/`data-is-demo` and calls `window.open`/clipboard in its keydown handler — the server never execs. |

### Issue Counting & Attention Filter

The main menu shows `issues:N` badges per resource type, counting resources in warning/error states. The ctrl+z key filters the menu to show only types with issues.

**Row Coloring:**
- `resource.Color` enum: `ColorHealthy` (green), `ColorWarning` (yellow), `ColorBroken` (red), `ColorDim` (grey).
- `(Color).IsIssue() bool` — returns true for `ColorWarning` and `ColorBroken`. Used by both the attention filter and issue-count badges.
- `ResourceTypeDef.Color func(Resource) Color` — per-type classification function. Classifiers resolve findings-first via `colorFromAnyFinding` (worst finding severity wins, wave1 or wave2-merged); the raw-field branches that remain are fallbacks for rows without findings (e.g., ad-hoc test doubles, states whose finding is still being emitted upstream). The conformance gate (`qa_color_findings_conformance_test.go`) pins color == findings-derived across the demo bench with an empty allowlist. REQUIRED for all registered types.
- `ResourceTypeDef.ResolveColor(r domain.Resource) domain.Color` (`core/catalog/types.go`) — dispatcher: calls `d.Color(r)` when non-nil, falls back to `colorFallback(r.Fields["status"])` for ad-hoc test doubles that omit `Color`.
- `colorFallback(status string) domain.Color` (`core/catalog`) — status-string fallback covering common AWS vocabulary; used only when `Color` is nil (test doubles).
- `styles.ColorStyle(c domain.Color) lipgloss.Style` — maps `domain.Color` to a palette foreground style for row rendering.

**`TierColorStyle`** (`styles.TierColorStyle(tier string) lipgloss.Style`): Maps detail-view tier strings to palette foreground styles. Tiers: `"ok"`, `"!"` (broken), `"~"` (warning/scheduled), `"impaired"`, `"initializing"`, `"ct-danger"`, `"ct-attention"`, `"ct-info"`.

**`IdentityKey` and `IdentityColumnIndex`**: The enrichment row-marker dot is placed in the "identity column" — the column that most clearly names the resource. `ResourceTypeDef.IdentityKey` pins the column by key. When empty, `app.IdentityColumnIndex(cols, td)` (`core/app/list_columns.go`) applies a 4-step cascade:
1. `td.IdentityKey` matches a column's `Key`
2. column `Key == "name"`
3. column `Title` equals `"Name"` (case-insensitive) or equals `td.Name`
4. fall back to column index 0

A column's field path is never consulted: a path that merely contains `Name` names a field of something else, and only the identity column may fall back to the row's own `Name` when its cell resolves to nothing (`ExtractCellValue`).

**`CellDecorators` and `lookupDecorator`**: `ResourceTypeDef.CellDecorators` is a `map[string]func(Resource, string) string` that transforms a cell's display value before render. `lookupDecorator(decs, col)` resolves the right decorator via a fallback chain: column `Key` → column `Path` → `Path`'s final segment (lowercased) → column `Title` (lowercased). Only EC2 currently uses this (to prefix state with `"! "` for impaired or `"~ "` for degraded-but-running).

**Issue counting flow**:
1. Wave 1 probes (or `demoPrefetchCounts()`) count `td.ResolveColor(r).IsIssue()` rows from first page
2. Counts flow to `MainMenuModel` via `SetIssues()`, rendered as `issues:N` badges
3. Wave 2 enrichment folds findings onto the cached rows, then `unifiedIssueCount` (`core/runtime/handlers_availability.go`) recomputes the badge from the folded rows and emits `PatchMenu` — a recount, not an only-increase merge, so a healed issue clears
4. The old `popView()` sync-back is gone: the list-count → menu-badge sync runs at the controller level (`core/app/handle.go`, `handleResourcesLoadedEvent`/`syncExactTotalToMenu`) on every `ResourcesLoaded` for a top-level list, so both the TUI and web renderers get it as soon as a fetch or load-more result lands
5. The availability count itself has one rule and one writer function, `applyAvailabilityObservation` (`core/app/menu.go`), called by both the list-open lane and the probe lane (`PatchMenuAvailability`): an untruncated observation always wins and clears the lower-bound marker; a truncated one wins only when nothing is known, when the stored count is itself truncated, or when it reports at least as many. Two lanes with two rules is what let a canonical list of 5 rows sit under a badge of 200 and queue that 200 to the availability cache writer

**`ExcludeFromIssueBadge`**: When set on a `ResourceTypeDef`, rows are still colored and ctrl+z is honored, but the type is excluded from the main-menu badge count. Used by ct-events where severity is event-level, not resource-health.

**Tri-state visibility** under ctrl+z on the main menu. Per [`docs/attention-signals.md`](attention-signals.md), every registered resource type has at least a Wave 1 or Wave 2 signal, so there is no "always healthy" escape hatch — a zero issue count is only "CONFIRMED zero" when the probe was not truncated.

| State | Condition | Badge | Visible under ctrl+z? |
|-------|-----------|-------|----------------------|
| Unknown | Not yet probed | None | Yes (prevent cold-start empty menu) |
| Confirmed zero | `issues == 0` AND `!truncated` | None | No — probe completed and saw no issues |
| Truncated zero | `issues == 0` AND `truncated == true` | None | Yes — lower bound; later pages may hold issues |
| Nonzero | `issues > 0` | `issues:N` (or `issues:N+` when truncated) | Yes |

`ExcludeFromIssueBadge` types (e.g. ct-events) are unconditionally hidden under ctrl+z — severity is event-level, not resource-health.

### What the first screen says about itself

A menu row seeded from the disk cache looks exactly like a freshly probed one unless the menu says otherwise. Three signals do that, all derived from state the sweep already records:

- **Sweep progress in the frame title.** `menuProgressIndicator` (`core/app/menu.go`) is the single formatter for the title's progress slot: `[verifying N/M]` while the Wave-1 availability sweep runs, `[enriching N/M]` while Wave-2 enrichment runs, empty otherwise. `menuFrameTitle` appends it and `MenuBody.Progress` carries it, so the headless body and the TUI cannot describe the same sweep differently.
- **Per-row cause.** A probe that hard-fails emits `PatchMenuProbeCause` carrying the class `classifyProbeErr` already assigned (`access-denied`, `expired`, `throttled`, `timeout`, `tls`, `dns`, `transport`, `region-unavailable`, or a raw AWS code); the row keeps its cached count and shows that class's own word (`denied`, `expired`, `throttled`, `timeout`, `tls`, `dns`, `transport`, `no service`, or `error` for a code the table does not name) in the alias column. A partial result is not a refusal and clears the mark, as does any successful probe. The error text is classified once, in `classifyProbeErr`, and never parsed again.
- **Account-wide cause.** When every probed type failed the same way and none was verified this session, `menuSweepCause` puts one phrase in the title (`sweep: access denied`, `session expired`) and every row drops its mark — an expired session is one fact about the session, not one per resource type.

A profile or region rotation goes through one clear point, `MenuState.ClearAvailability` (`core/app/screenstate.go`), fired by `MenuClearAvailabilityIntent`. Every per-type map on `MenuState` — counts, truncation, issue badges, origin, probe cause — is a fact about the pair that was probed, so all of them go together; what the operator typed and where the cursor sits is not, and stays. A test reflects over the per-type fields, so a field added later cannot be missed.

Origin has one writer: the `PatchMenuAvailability` intent case. Both lanes that observe a type emit that intent — the sweep's `AvailabilityChecked`, and a live list fetch, which `syncExactTotalToMenu` (`core/app/handle.go`) turns into the same intent rather than writing the menu's maps itself, so a type the operator opened and fetched is not "not yet verified" and its previous probe's failure mark is retired by the same batch. A lane that wrote the map directly would bypass the observation rule below, which is the reason the map has a rule at all.

Both halves of a menu entry — the count and the issue badge beside it — follow one observation rule, and neither half may follow a different one: a seed never outranks a value verified this session (`menuObservationOutranksStored`, the gate both intent cases call), an observation that carries a real answer assigns (a live probe or enrichment result, and a disk seed, which carries the last session's answer — `applyMenuIssueObservation`), and a rows-derived observation, whose zero proves only that the fetched rows carry no findings, only raises (`syncMenuIssueCount` with `authoritative=false`). The persisted issue count is that same reconciled badge, read back by `menuIssueBadge` at the list-open save rather than derived a second time from the same rows: the file records what the screen shows, so a restart cannot change a badge that nothing in the account answered for.

An error becomes words in exactly one place, `core/aws/partial_errors.go`. `ErrClass` assigns the class (`classifyProbeErr` delegates to it) and one table gives each class everything a surface can say about it: the cause `CauseOf` renders, the word `RowWord` puts in the menu row's alias column, and the phrase `SweepTitleForWord` puts in the title when every type failed that way. A class the table does not name is a raw AWS error code, whose own message is its cause and whose row word is the generic `error`. `region-unavailable` is the one class with no title: an account cannot be missing from its own region, so "every type failed this way" is unreachable for it. Every branch that treats the region gap specially — the per-type availability handler and the prefetch sweep — asks `ErrClass` for the class and compares it to `ClassRegionUnavailable`; there is no second predicate to agree with the class by luck. A failure the network caused is three classes, not one, because the operator does three different things about them: `tls` (a certificate that does not verify, or a plaintext answer to a handshake — a proxy or a wrong clock), `dns` (a resolver failure that is not a missing host) and `transport` (a connection refused or reset — the network path). `CauseOf` reads the cause off the error's own fields: the class's phrase where the class says more than the error does, otherwise the API error's code and its message. Nothing is read from the SDK's formatted chain, so the request id, the host id and the `operation error <Service>: <Op>` preamble are never in a cause, and a message whose own words spell one of them keeps them. Within the message field itself — where AWS models the denied action, the encoded authorization blob and any line break — the cause takes the action, drops the blob and keeps the first line. `MessageOf` is the one extraction of that message field for a surface that wants the AWS text verbatim, such as the costs screen's refusal note. `CauseInRegion` names the region, and only for the class that is about the region. Every flash, title and error-history line that names an AWS failure phrases it through `CauseOf`, never through an error's `%v`. In `core/runtime`, `failureLine` builds that sentence — the lane word and the type, then the cause, on one line. The type is named once and by one owner: the event's registry key at the surface, never a string an enricher or a fetcher wrote into its aggregate's label, which names the call alone (`enrich ddb: DescribeContinuousBackups failed for 2 of 2 IDs: …`). Where a label still leads with its type, the formatter drops its own prefix rather than saying it twice, and a two-pass enricher's joined aggregates are separated the way causes are instead of by the line break `errors.Join` writes. It is subject, then the cause, and the cause alone when there is no subject. Every failed call routes through it: the API error handler, the failed connect, the prefetch sweep's hard and soft failures and its per-type line, the per-type probe's banner and its log entry, the list-enrichment and detail-enrichment failures, the list fetch, the related check and its lazy add, and a failed reveal.

**An error flash is the session's record of the failure.** The controller writes one error-log entry for every error `FlashIntent` it applies, and that is the only place an entry is made — so whether a failure can still be read after its banner has gone does not depend on which host is rendering. Runtime handlers emit the flash and stop there; they no longer carry a second, history-only intent, and the terminal adapter no longer decides. A failure whose rows are already on screen saying what they are — a partial batch, a region gap — sets `FlashIntent.LogOnly`: the entry without the banner. Both bulk-forward sites in the terminal host (`applyIntents` in `app_dispatch.go`, `dispatchDetailOpResultIntents` in `runtime_adapter_resources.go`) withhold banner-raising flashes from their controller forward and re-emit them through `handleFlash`, whose own application is the one that reaches the controller; a log-only flash is never re-emitted and is applied by the forward alone. Either way each flash is applied once, and the web error-log surface shows the same entries the `!` overlay does. `CauseOf` has no caller outside `core/aws` any more; a surface that wants words asks `failureLine`. The banner and the log entry for one probe are therefore the same sentence, differing only in where they land; a banner never carries the internal `ProbeOutcome` or class name in place of the cause. When the probe that completes a sweep is the one that failed, the completion's `ClearFlash` — which exists to retire the progress message — is withheld, so the last type to fail is not the one type whose failure never reaches the screen. `AggregateFailures` reads the same words: a failed per-item call is recorded, never rendered, so nothing hands it a string to reduce back to a cause. The two local cache-save flashes (`core/app/handle.go`, `core/runtime/executor.go`) format a filesystem write error directly; it carries no AWS noise to remove.

A failed per-item call is recorded once, as a class and a cause, never as a string. `FailedCall` (and `UnusableAnswer`, for an answer that carries no error — a nil struct, a page that says it is truncated and names no marker) is the one writer of a `Failure`: the item's id, the class `ErrClass` assigns, and the cause `CauseOf` reads off the error's own fields. `MarkSkipped` is the Wave 2 form of the same record, adding the row to `TruncatedIDs` so it renders `?`. No fetcher, checker or enricher formats a failure itself. The record also names the operation, read from the SDK operation error's own fields: one aggregate can cover several calls — a related walk resolves a load balancer's name and then reads its listeners — and the id alone does not say which of them refused. A denied call's cause already names the action, so it is not decorated twice. A row marked uninspected says why. An enricher that could not read a resource records the failed call through `MarkSkipped` — or `MarkUnusable`, when the service answered without the field the check needs and there is no error to read a cause from — and folds the records into its returned error. Writing `TruncatedIDs` directly is the silent-skip shape, where the row renders `?` and nobody can say what refused; what remains of it is a page cap or a documented race (the resource went away between the list call and the per-item one), each carrying its one-line reason on the line above the write. `TestConformance_UninspectedRowsRecordTheirReason` holds the per-file count and requires that reason. It is a ratchet in both directions — a new direct write fails, and a file that loses one fails until its number comes down. A per-item failure travelling out as the enricher's error is not an abort: the walk finishes, every other row keeps its findings, and `Truncated` is raised only where the pass can emit an issue, so an informational-only pass reports a coverage gap without lower-bounding the badge.

Partial-batch failures are phrased in exactly one place, `AggregateFailures` (`core/aws/partial_errors.go`): the records are grouped by their recorded cause, and each cause is stated once with how many resources it covered and one example id. Grouping is per cause rather than per class so two actions the same role lacks stay two lines — one denial never speaks for another. A role denied one action fails on every resource of the type at once, so the aggregate names the action the role lacks rather than repeating one AWS error per resource with its request id, host id and encoded authorization message. When every record shares one class the composite carries it, so a fetcher's denied batch still reads as `access-denied` on the surfaces downstream instead of reclassifying as `Unknown`; a mixed batch carries none, because there is no single answer to give. The class travels for the surfaces that branch on it only: a surface that reads the composite gets its whole sentence, count and example included, and never the class word in its place.

Reading an AWS error code is `ClassifyAWSError`'s job alone. A site that acts on a class asks a named predicate — `IsAccessDenied`, `IsNotFoundErr` — and a site that names a code AWS models as an error but an operator reads as an answer (`PolicyNotFound`, `InvalidLaunchTemplateId.NotFound`, `PermanentRedirect`) asks `ErrCodeIs`. No site unwraps the SDK's error type for itself: a modeled exception's error code is not its type's name (IAM's `NoSuchEntityException` answers `NoSuchEntity`), so a second reading is a second answer.

---

## Key Handling

### Bindings

All bindings are defined in one file: `internal/tui/keys/keys.go`. Single `Map` struct, single `Default()` constructor. Views receive `keys.Map` at construction and use `key.Matches(msg, m.keys.XYZ)`. No runtime rebinding.

Key highlights:
- `Enter` — drill into detail/child view
- `d` — detail view, `y` — YAML view, `J` (uppercase) — JSON view. These three are inter-navigable: pressing any of them from another replaces the current view in-place.
- `1`–`9`, `0` — sort by column position (1=first column, 0=tenth). Pressing the same key toggles sort direction.
- `t` — jump to CloudTrail Events for the selected resource (all resource types)
- `!` — error log (session errors with timestamps, rendered via `YAMLModel.NewTextViewer`)
- `Ctrl+Z` — toggle attention filter: on resource lists, hides rows where `m.typeDef.ResolveColor(r).IsIssue()` is false (dim/routine rows); on main menu, filters to types with issues using quad-state visibility (unknown→visible, confirmed-zero→hidden, truncated-zero→visible, nonzero→visible)
- `c` — copy resource ID to clipboard
- `o` — open the selected resource in the AWS console (list/detail only; demo mode flashes instead), `O` — copy the console URL
- `e`, `L`, `R`, `s` — child view triggers (Events, Logs, Resources, Source)
- `r` — toggle related panel
- `x` — reveal secret value
- `w` — toggle line wrap (in YAML, JSON, detail, and reveal views)
- `m` — load more (next page for paginated lists)
- `Ctrl+R` — refresh (re-fetch resources, re-run related checks + enrichment)
- `/` — context-dependent: starts filter on list views, starts search on detail/YAML/JSON, view-dependent elsewhere
- `n`/`N` — next/previous search match (in detail/YAML/JSON)
- `g`/`G` — go to top/bottom
- `Esc` — back / cancel / pop view
- `q` — quit the application (in normal mode; swallowed when filter, command, or search input is active)
- `?` — help
- `i` — identity (STS caller identity)

### Global vs View-Local Key Resolution

The root model's `handleKeyMsg` (`internal/tui/app_input.go`) resolves keys in a specific order. Input modes take priority over global keys, which means `q`, `?`, etc. are swallowed during typing:

1. **`Ctrl+C`** — force quit (always global, never delegated)
2. **Input modes** — if filter, command, or search input is active, **all keys route to that input handler** (including `q`, `/`, `?`). This is why `q` doesn't quit while typing a filter.
3. **`?`** — help (global, pushes HelpModel)
4. **`i`** — identity (global, pushes IdentityModel)
5. **`!`** — error log (global, pushes a `YAMLModel` in raw-text viewer mode via `NewTextViewer`)
6. **`q`** — quit (`tea.Quit`)
7. **`Esc`** — complex: delegates to view first (search cancel, right-column defocus), then pops
8. **`:`** — enter command mode
9. **`/`** — routed to `updateActiveView`, which handles it per-view (filter on lists, search on detail/YAML/JSON, ignored elsewhere)
10. **Everything else** — falls through to `updateActiveView` (view-local handling)

The critical insight: steps 2-3 mean `q` only quits in normal mode. During filter/command/search input, all keys are captured by the input handler. `/` is not globally handled — it's always delegated to the active view.

### Sorting

Resource lists support column-position sorting via keys `1`–`9` and `0` (tenth column). `SortByCol [10]key.Binding` in `keys.Map` maps the digit keys to column indices; pressing a key sorts ascending and pressing it again toggles to descending, with a ↑/↓ indicator in the column header. Column position is the only sort model: the `SortField` alias and the `SortName` / `SortID` / `SortAge` sentinels were removed in #283.

**One key names a column.** The controller holds the sort as a column key plus a direction, and that key comes from `app.ColumnDef.SortColKey` — the column's `Key`, or its `Title` when it has none — and from nowhere else. `app.SortColIndex` is the only way back from a key to a column. Both directions of the list-view cache round trip go through the pair: leaving a list stores the sort column's index on the cache entry, and re-entering turns the index back into a key. A second spelling on either side drops the operator's sort silently on re-entry, which is why the header-arrow lookup and the comparator's column lookup ask the same two functions. `Path` is deliberately not in the formula: two columns of one view may read the same RawStruct path, so a path names a column ambiguously.

**One measure reserves and paints.** A header column's width is grown to fit its position prefix, its title and the sort arrow, and the cell is then padded or truncated to that width. Both steps measure in terminal columns through `text.Width`, the same measure `text.PadOrTrunc` uses. Counting runes on the reservation side gave a title carrying a wide rune (CJK, an emoji) half the room it needs: a short one overflowed its column and slid every column right of it off its rows, a longer one was cut by an ellipsis that ate the sort arrow, so the header read as unsorted over sorted rows. The sort key, the direction and the horizontal scroll offset reach `renderHeaderRow` as parameters from the body being rendered, so no render-time state is parked on the list model first.

**A column sorts by a stored value.** When the displayed text does not sort the way the value does — a size rendered `900 B`, a status rendered as a finding phrase — the view declares `sort_key`, a `Fields` key the Wave 1 fetcher writes (conventionally `<key>_raw`). It is a stored key rather than a RawStruct path on purpose: a list opens on cached rows that have no RawStruct, so a path-based comparison orders the warm frame differently from the frame the fetch lands, and the rows move under the cursor. The `sort_path` companion was removed for that reason. The comparator reads one representation on every frame — the column's stored sort value when it declares one, otherwise the cell text — and never the SDK struct; a gate (`tests/unit/sort_one_representation_test.go`) fails the build if `core/app/list_filter.go` reads `RawStruct` again. A sweep over every registered column, sorting the demo fixtures as a live frame and as the frame the disk cache replays, found no column the two representations ordered differently, so no fetcher owes a new sort value today; the one thing the struct could still express and the cache cannot is a launch time to the second, which the cached frame rounds to the minute.

**Generated view files carry a stamp.** `config.GeneratedViewsVersion` is written as a `generated: <n>` header into every file `EnsureViewsDir` produces, and bumped in the same change that adds, removes or renames a built-in column, or that changes where an existing one reads its value from. On launch, a file already at the current stamp is left byte for byte alone; a file with an older stamp (or none) keeps every column it has and gains the built-in columns whose titles it does not carry, each inserted at its built-in position. Without the stamp, an operator who ran a9s once keeps that day's column set forever.

**A corrected column source travels with the stamp.** A column whose source changes keeps its title, so the missing-title rule never sees it and an operator who had already launched a9s would keep reading the field the column was corrected away from — a cell that looks like it works and is wrong. `viewColumnSourceChanges` (`core/config/ensure_views.go`) records, per stamp, which built-in column changed and the source the previous build generated for it. On upgrade, an on-disk column matching that previous source takes this build's source and width; any other source under that title is the operator's own and is left exactly as it is. The corrected source is not stored in the table — the defaults are the one place a built-in column is defined. Add a row there in the same change that alters a built-in column's `Path` or `Key`.

---

## Related Views (Right Column)

The detail view has a right-column panel showing related resources.

> ⚠️ **The expected related-panel contract per resource type lives in [`related-resources.md`](./related-resources.md) — the SINGLE SOURCE OF TRUTH.**
> That document is produced from AWS API references + DevOps workflows (six
> independent audits). DO NOT edit the `Related` fields on catalog literals without reconciling
> against the golden table. Drift has already happened once — do not repeat it.

```go
// core/resource/related.go
type RelatedDef struct {
    TargetType       string         // e.g., "vpc"
    DisplayName      string         // e.g., "VPCs"
    Checker          RelatedChecker // async function
    NeedsTargetCache bool           // true = reads from the session RowStore
}
```

**Two checker patterns:**
- **Live API** (`NeedsTargetCache: false`): Calls AWS directly (e.g., `DescribeTargetHealth`). Fast, specific.
- **Cache scan** (`NeedsTargetCache: true`): Reads a `RowStore.SnapshotAll(true)` snapshot — one entry per type, `Partial` (lazy-add) entries included, and observed-empty types stay present in the snapshot. The dispatcher pre-fetches the target type if absent.

`(*Core).HandleRelatedCheckStarted` (`core/runtime/related.go`; TUI adapter `handleRelatedCheckStarted` in `internal/tui/runtime_adapter_related.go`) fans out one goroutine per `RelatedDef`, capped by `MaxConcurrentProbes`. Results carry a generation to discard stale results after Ctrl+R or profile/region switch.

**A checker result cannot carry a count alongside a failure.** `domain.RelatedCheckResult`'s fields are unexported; it is constructible only through four smart constructors, re-exported from `core/resource`:

| Constructor | Meaning | Renders |
|---|---|---|
| `KnownRelated(target, ids, truncated)` | the call succeeded; count is `len(unique ids)` | `N`, or `N+` when truncated |
| `UnknownRelated(target)` | we could not determine the answer | `?` |
| `ErrorRelated(target, err)` | the call failed, with a reason to surface | error marker |
| `DeferredRelated(target)` | not resolved yet by design | deferred |

This exists because the old struct permitted `Count: 0` after a failed call, and ~136 hand-written checkers each re-derived their own error handling — so they disagreed. `dbc_related.go` and `redis_related.go` performed the identical two-hop subnet-group resolution and returned opposite answers on failure; both shipped. A denied `logs:DescribeSubscriptionFilters` rendered as a proven `(0)` on a log group that was actively streaming. **Writing `RelatedCheckResult{Count: 0}` is now a compile error outside `core/domain`**, so the lazy default is unavailable; what the type cannot prevent is a well-typed but wrong choice — calling `KnownRelated` where the code should have caught an error and called `UnknownRelated`.

A genuine zero stays a proven zero: a successful call returning an empty list is `KnownRelated(target, nil, false)`. Turning those into unknowns is the opposite defect and makes every panel useless.

**Truncated-cache contract (`Truncated=true`)**: cache-scan checkers that can't see the full universe — because the target cache's `IsTruncated=true` after its first page — must signal the undercount rather than silently rendering `0`. `KnownRelated(target, ids, true)` carries that signal, and the UI renders `(N+)` or `(0+)` so operators know the real count is at least N. The same shape covers a partial union: when several calls back one pivot and only some succeed, the confirmed IDs render as `N+` rather than a false exact total. Truncation means "the population is larger than what we enumerated" — a hard failure with nothing confirmed is `ErrorRelated`, not truncation.

### Navigable Fields

```go
type NavigableField struct {
    FieldPath  string // e.g., "VpcId"
    TargetType string // e.g., "vpc"
}
```

In the detail view, navigable fields are underlined. Pressing Enter on one emits `RelatedNavigateMsg`, which pushes a filtered list of the target resource type.

**ID-format normalization**: Some navigable fields carry ARNs (KMS `KeyArn`, IAM `RoleArn`, ECS `ClusterArn`, Lambda `FunctionArn`, CloudWatch `LogGroupArn`) while the target resource's `Resource.ID` is a bare name or alias. `resource.NavIDFromValue(targetType, value)` (in `core/resource/related.go`) is a central registry that normalizes these values into bare IDs at navigation time. Target types with registered extractors: `kms`, `role`, `ecs`, `logs`, `s3`, `iam-user`. Other target types pass through unchanged. `projection.buildItems` in `core/semantics/projection/generic.go` applies this transform to every scalar navigable item before rendering so the resolved bare ID matches `Resource.ID` on the target's list.

**List-typed scalar extraction**: `fieldpath.ExtractFirstListScalar(obj, dotPath)` (in `core/fieldpath/extract.go`) walks slice-valued dotted paths to pull a scalar from the first element, enabling navigable fields on fields like `Subnets.SubnetId` without hand-rolled traversal. Returns an empty string when the path is empty, the slice is empty, or any intermediate step is nil — unlike `ExtractScalar`, which only walks struct fields and pointers.

---

## Enrichment

a9s has two distinct enrichment pipelines with disjoint contracts:

1. **Detail enrichment** (on-demand) — `resource.DetailEnricher` in `core/resource/enricher.go`. Fetches additional data when a user opens a detail/YAML/JSON view (e.g., IAM policy documents). See below.
2. **Wave 2 issue enrichment** (background) — `awsclient.IssueEnricherFunc` declared on each `catalog.ResourceTypeDef.Wave2` field and accessed via `awsclient.Wave2EnricherFor(shortName)` / `awsclient.AllWave2()` (shared types in `core/aws/issue_enrichment.go`; read API in `core/aws/wave2.go`; per-resource enricher bodies live in `*_issue_enrichment.go` files per short name). Discovers hidden issues via additional API calls after Wave 1 probes complete. See "Wave 2 Issue Enrichment Pipeline" under Fetcher Patterns.

### On-Demand Detail Enhancement

When a detail, YAML, or JSON view opens (or is refreshed with Ctrl+R) for a resource type with a registered enricher or related defs, all resulting async work runs under one **detail operation**.

**The cursor keeps its row, not its index.** An enrichment result rebuilds the detail body under whatever the operator is reading: a finding that outranks the ones already there sorts above them, so every row below it moves down — sometimes at a block length the rebuild happens to leave unchanged — and a detail enricher can replace the resource itself, moving the content rows too. One build returns one layout record (`detailLayout`: the identity of the row under the cursor and the size of the Attention block it was indexed against), and `snapshot()` — the only build that reaches a screen — stores it on `DetailState`. No build writes to the state it is handed, so a body built for a copy or a navigation lookup cannot record a layout nobody saw. After the rebuild, `relocateDetailCursor` puts the cursor on the row carrying that identity, wherever it landed; Attention rows and content rows go through the one function, so neither is relocated by a rule the other does not use. Only when the row is gone does the cursor fall back to keeping its distance below the block. An Attention row's identity is its finding's code plus its position within that finding — never the prose painted on it, which two findings can share word for word — and the section header has an identity of its own, so a cursor parked on it stays there while the line that counts the findings changes underneath. A content row is identified by its path and its label. One resource can repeat either identity — two targets of one CloudTrail event, two identical lines of a raw document, two findings under one code — so a repeat is numbered in the order it was reported: content rows in projection order, Attention entries in the order the enricher listed them, before the block is sorted by severity. Numbering after the sort would make the number mean "where you are", and two entries of one code would exchange identities the moment one outranked the other. The rendered label is never disturbed to make a key unique. `core/app/detail_identity_gate_test.go` walks every resource of every demo fixture through the real projection and fails on a tie. The cursor never rests on a blank line: the Attention block ends in a spacer, which is the last row of a resource whose projection yields no content, so both the jump to the bottom and the body's clamp walk back to the last row an operator can read, through the one function (`selectableRowAtOrAbove`) — the row the move chooses and the row the screen shows cannot be different rows. Recording the identity at the build rather than reconstructing the old block later is what makes it hold on the sweep's path: the runtime writes the session truncated-ID set (which adds the `Not inspected` entry) before the controller applies the intent, so a block rebuilt at that point is one the operator never saw. The terminal host reads the same `FieldCursor` off the controller's detail body, so both surfaces behave identically.

**The identity rule** (`core/runtime/detail_op.go`): every identity question a detail view's async work can ask — which generation am I, which AWS clients do I use, which in-flight calls may I share, is my result still wanted — has exactly one answer: the `DetailOperation` created synchronously at the user action.

```go
type DetailOperation struct {
    ID           domain.Gen                 // session.DetailOpGen, bumped per open/refresh
    ResourceType string
    Resource     resource.Resource
    Clients      *awsclient.ServiceClients  // captured at creation, under the controller lock
    Refresh      bool                       // Ctrl+R (drives cache-read bypass)
}
```

**Flow:**

```text
View opens or Ctrl+R (TUI keypress or web action — both under the controller lock)
  → Controller.beginDetailWorkloadLocked(rt, res, refresh, forceRelated)
      // the one entry point every such action routes through
    → Core.BeginDetailOperation(rt, res, effectiveRefresh)
        // bumps DetailOpGen, becomes the active op; returns the op AND its
        // complete []TaskRequest workload in one call — no separate
        // half-workload step
        → KindEnrichDetail task   (iff a DetailEnricher is registered;
                                   DetailEnrichmentCtx from op.Clients + session caches,
                                   SkipCache = op.Refresh, OpID = op.ID)
        → KindRelatedCheck task   (iff related defs are registered)
    → the related task is omitted when a cache replay already populated the
      panel (D6), or dropped after first deleting the stale cache entry when
      the caller is an explicit recompute (forceRelated — resolve-in-place,
      Back-reveal recompute, refresh)
  → BOTH renderers execute the SAME TaskRequests
      TUI: tea.Cmds — per-def runtime.RunRelatedDef fan-out for progressive rendering
      web: background drain — executor fan-out, same RunRelatedDef
  → results carry OperationID (EnrichDetailResult, RelatedCheckResult / RelatedCheckBatch)
    → ONE acceptance rule, in the shared Controller fold:
      a result folds iff its OperationID matches the active detail operation
```

There are no intermediary dispatch messages: the operation's tasks are created at the action, so there is no window in which a refresh can re-label an older open's work (the older operation's results simply stop matching). `Rotate()` bumps `DetailOpGen`, so results from a previous profile/region can never fold into the next.

**Sticky refresh**: an explicit refresh (`refresh=true`, Ctrl+R) is a demand on the resource, not on the one operation that happened to carry it. `session.PendingDetailRefresh` records the demanding operation's ID per resource key; `effectiveRefresh` (`beginDetailWorkloadLocked`) ORs the requested flag with any still-pending demand for that key, so a non-refresh operation beginning before the refresh's own enrichment has folded (a panel toggle, a related-row retry) inherits `SkipCache` too instead of silently reading the stale cached document. The entry is cleared only when an `EnrichDetailResult` for the key folds successfully with an operation ID `>=` the recorded one (`foldEnrichDetailResultLocked`) — a failed refresh never clears it, so it never downgrades the next open back to cache.

**Call coalescing** (`core/aws/coalesce.go`): SFN/SNS/S3/Lambda transports are wrapped in singleflight decorators whose keys are namespaced by the operation ID (`WithDetailOp` on the context, applied once per lane — the generic engine for enrichers, `RunRelatedDef` for checkers). Within one operation, the enricher and every related checker share a single in-flight call per API key while it is still in flight (in-flight dedup, exactly as before); once that call completes, its result is retained in a small per-decorator, per-operation bounded LRU (`completedResultMemo`, 128 entries) so any LATER call for the same key under the SAME operation returns the memoized result instead of re-fetching — an SFN detail refresh performs one `DescribeStateMachine` in total even when the web drain runs the enricher and every related checker sequentially rather than concurrently. The memo is never shared across operations (keyed by operation ID; a refresh mints a new operation and therefore a new, empty namespace) and never applies to non-operation calls (`opID == 0`) — those always re-execute after the previous one finishes, exactly as before this memo existed.

**Execution engine**: enrichers are `detailEnrichSpec`-based instances of the generic `enrichDetail` engine (`core/aws/detail_enrich_engine.go`) — unwrap/id/cache/fetch/wrap per resource type, wrapper structs embedding the original raw struct so field extraction and related checkers see the enriched value transparently. Per-def related-check execution — target-cache prefetch, timeout, panic recovery, lazy-add — lives once in `runtime.RunRelatedDef`, shared by both renderers.

**Error handling**: enrichment and lazy-add errors surface as an error flash through the same fold; no state is updated on error.

**Caching policy**:
- **Default**: no cache. Re-fetch on each enrichable view open when the data is cheap enough or may change during a session (`sfn`, `lambda`, `ec2`, `sns`, `s3`).
- **If caching is justified**: use a session-scoped, feature-specific cache owned by `session.Session` and passed to detail enrichers via `*awsclient.DetailEnrichmentCtx`. `SkipCache` (set on refresh) bypasses the cache read so Ctrl+R always fetches fresh.
- **Never**: use package-global cache state for enrichers.

**Enricher inventory** (issue #261 expanded the set): cached via `PolicyDocumentCache` (`session.Session.PolicyDocCache`, keys `managed:<policyArn>` / `inline:<roleName>/<policyName>`) — `policy`, `role_policies`; cached via `DetailDocCache` (same ownership/rotation rules, key `cfn:<stackId>:<lastUpdatedUnixSeconds>`, version-keyed so an in-session stack update forces a natural miss) — `cfn` (template body via `GetTemplate`); uncached — `transfer_agreements`, `sfn` (definition/status/role via `DescribeStateMachine` — the ARN survives a redeploy, so caching would serve a stale definition exactly while an operator watches a deploy), `lambda` (`GetFunction`), `ec2` (`DescribeInstanceAttribute` user data, base64/gzip-decoded), `sns` (`GetTopicAttributes`), `s3` (`GetBucketPolicy` + `GetBucketCors` + `GetBucketLifecycleConfiguration`). Both caches are replaced with fresh instances by `session.Session.Rotate()` so entries from a previous account cannot leak into the next.

---

## Child Views

Child views are drill-down lists from a parent resource (e.g., IAM Role → Role Policies).

```go
// core/resource/types.go
type ChildViewDef struct {
    ChildType         string               // "role_policies"
    Key               string               // trigger key: "p"
    ContextKeys       map[string]string    // fetcher params from parent
    DisplayNameKey    string               // field to show in title
    DrillCondition    func(Resource) bool  // optional predicate; when non-nil, drill is allowed only when true. nil = always drill. Example: SFN Executions, where Express state machines have no execution history
    DrillBlockMessage string               // flash text shown when DrillCondition returns false
}
```

**ContextKeys** resolution:
- `"ID"` → parent `Resource.ID`
- `"Name"` → parent `Resource.Name`
- `"@parent.x"` → parent view's `ParentContext["x"]` (for nested chains)
- anything else → `Resource.Fields[key]`

When triggered, `EnterChildViewMsg` is emitted. `handleEnterChildView` constructs a `NewChildResourceList` and calls the registered `PaginatedChildFetcher` with the resolved parent context.

---

## Caching Layers

The app has four distinct caches plus one enrichment-visibility state store:

The row-store unification (task #17) landed: every in-memory per-type row copy — Wave-1 probe retention (`ProbeResources`/`ProbeTruncated`), the list cache (`ResourceCache`), lazy related adds (`LazyResourceCache`), and the controller-side row mirror — collapsed into the single `session.RowStore`. Those maps are gone; a type's rows live in exactly one entry regardless of which lane wrote them.

One session field is deliberately EXEMPT from `Rotate()`'s clean-slate rule: `Session.SweptPairs` (v3.51.0) memoizes, per `profile--region` pair, that the availability sweep ran to COMPLETION this process lifetime — switching back to an already-swept pair skips the full re-probe (the menu seeds from the disk cache that sweep wrote), an interrupted sweep is not memoized, and `Ctrl+R` on the main menu clears the current pair's memo to force a re-sweep. It is the only cache-adjacent state that must survive profile switches by design.

| Cache | Location | Scope | Invalidation |
|-------|----------|-------|-------------|
| **Disk availability cache** | `core/cache/` | Persisted at `~/.a9s/cache/<profile>--<region>/` — one directory per pair, one YAML file per resource type (`<shortName>.yaml`). Each pair element is percent-escaped by `cache.EncodePathElem`, injectively — no encoded element contains `--`, starts with `-` or ends with `-`, so the first `--` is always the separator and `team/a` and `team_a` can never share (or overwrite) one directory. Ordinary names pass through unchanged. There is no fallback to the older, non-injective sanitized layout: that name is shared by every pair it collapses, so a pair whose name needs escaping starts cold once rather than inheriting another pair's rows | No TTL by contract (C1: cached content renders stale-marked and is re-verified on sight); each type's file replaced atomically via temp+rename, and a commit older than the last one committed for that file is skipped rather than landing last |
| **Row store** | `session.Session.RowStore` (owned by `runtime.Core`) | In-memory `map[string]session.TypeRows` — one entry per canonical resource type | Cleared on profile/region switch via `session.Rotate()` |
| **Related cache** | `session.Session.RelatedCache` | In-memory LRU with fixed capacity | Cleared on `Rotate()`; entry deleted on Ctrl+R |
| **Detail-enricher caches** | Feature-specific caches on `session.Session`, delivered to enrichers via `*awsclient.DetailEnrichmentCtx` (`PolicyDocumentCache`, `DetailDocCache`) | In-memory, session-scoped | Rotated by `session.Rotate()` on profile/region switch; refresh bypasses the read via `SkipCache` |
| **Enrichment visibility state** | `EnrichmentRan`, `EnrichmentTypeGen`, `EnrichmentTruncatedIDs`, `EnrichmentGen` on `session.Session` (Wave 2 progress/control); per-resource findings are folded into `resource.Resource.Findings` on cached rows — see "Wave 2 findings (where they live)" above | In-memory, session-scoped | Cleared per-type on Ctrl+R rerun start; cleared entirely on `Rotate()` |

**Disk availability cache** (`core/cache/cache.go`): Tracks which resource types have resources, their counts, issue counts, and render-sufficient row snapshots. Loaded on startup to instantly grey-out empty types, show issue badges in the main menu, and seed list screens with real rows before any live fetch. Layout: one directory per profile+region pair (`<cache root>/<profile>--<region>/`) containing one self-contained YAML file per resource type (`<shortName>.yaml`) — C7: per-type files, no merge logic. A file named after a type's alias (`rds.yaml` for `dbi`) is canonicalized once, at load, counts and rows together, with the canonically-named file preferred when both exist; `TypeFile.Population()` is the single accessor for how many resources the file knows the type has (its `Count`, which may exceed the rows it stored). Each file is a `TypeFile{Version, HasResources, Count, Exact, Issues, IssuesKnown, IssuesTruncated, Rows, SavedAt}`; `Rows` carry `{ID, Name, Fields, Findings, FindingFirstSeen}`. The schema is v2 (`cache.SchemaVersion`): v2 added `Row.FindingFirstSeen` (#463), and `LoadDirIn` still accepts a v1 file, backfilling `FindingFirstSeen` from the file's own `SavedAt` so a pre-#463 cache never regresses to "no cache". The `IssuesKnown` bool distinguishes "probed and found zero issues" from "not yet probed" (both unmarshal as int 0 without this flag). There is deliberately NO TTL ([`design/cache-requirements.md`](design/cache-requirements.md) C1): arbitrarily old rows may render, stale-marked, while re-verification runs in the background. When caching is enabled (not `--no-cache`), per-type files are saved after Wave 1 probes complete and again after Wave 2 enrichment completes (atomic temp+rename per file), so enriched findings persist across restarts; `Core.SaveAvailabilityCache` (`core/runtime/probes.go`) skips a type's write entirely when the counts-only scalars are unchanged against the existing on-disk entry. When `--no-cache` is active, the save lanes are no-ops.

**Row store** (`core/session/rowstore.go`): The single source of truth for every cached resource-list row the session has observed. One `TypeRows` entry per canonical short name carries: `Rows` (immutable once stored — every change produces a new slice, so snapshots can never be invalidated by a later write), `Pagination` (nil is never exact — conservatively treated as truncated, C5), `TotalCount` (may exceed `len(Rows)`), `Origin` (`disk|probe|fetch` — which lane last accepted a rows-carrying write), `Partial` (sparse `FetchByIDs` lazy adds; a full observe clears it — full-beats-partial — and a sparse add never downgrades a full entry), `Gen` (increments on every accepted write; `Gen != 0` makes "observed empty" first-class, distinct from "never observed"), and `ViewState` (filter/sort/cursor/h-scroll, so a warm re-entry restores the exact view the user left). Writes go through `Observe` (full rows), `ObservePartial` (sparse adds), `ObserveCount` (counts-only — never touches rows), and `Amend` (copy-on-write content mutation — the enrichment fold and finding patches apply exactly once, here); reads are defensive-copy `Snapshot`/`SnapshotAll`. **The store never shares an allocation with a caller in either direction**: `Observe`/`ObservePartial` deep-copy on ingress and on every return path, and `Amend` hands `fn` a copy rather than making copy-on-write each caller's responsibility. This is a locking contract, not tidiness — the caller holds its rows under the controller mutex and the store holds the same `TypeRows.Rows` under `RowStore.mu`, so one shared backing array has two independent locks, and an `append` into spare capacity on either side mutates the other's content with no `Gen` bump. Cloning at this one chokepoint is also strictly cheaper than the status quo, since `Snapshot` already pays the same deep copy on every read. Reconciliation rules: appends dedup by ID; a disk seed never overwrites a live probe/fetch observation — keyed on `Gen`, not on row count, so a live observation of an empty population outranks the seed too. The store is pair-scoped — `Rotate()` clears it on profile/region switch (C9).

The legacy accessor names survive on `runtime.Core` as store-backed views: `Core.ResourceCache(rt)` reports a hit only for a FULL, `OriginFetch` entry — this gates navigation's cache-hit promotion (a probe- or disk-origin entry seeds the list but still verifies with a live fetch, per C1: cached content renders before any AWS activity, then is verified); `Core.LazyResourceCache(rt)` reads `Partial` entries; `Core.AnyOriginResourceCache(rt)` serves related-navigate's any-origin (full-lane) cache hits; `Core.AnyLaneResources(rt)` returns a type's rows from EITHER lane (full or `Partial`) and is the render-time row source for a related-**filtered** list — the `RelatedIDSet` scopes it, so surfacing the lazy/by-ID `Partial` lane is safe and a cache-hit filtered list renders without a fetch on both the TUI and the web (`Controller.seedRelatedExactRows`, the single seed both renderers share).

**Per-screen views**: the headless controller's `ListState.Rows` is a per-screen VIEW adopted from the store's accepted rows for the canonical top-level list — the store reconciles, the screen adopts, and `ListState.RowsGen` pins the store generation the rows were adopted at. Child, related, and filtered screens stay screen-local (the C6 scope boundary) — their rows never route through the store.

**Fetch provenance** (`messages.FetchProvenance`) is what enforces that boundary. Every fetch lane — canonical list, related-filtered drill, `FetchByIDs`, child list — emits the same `messages.ResourcesLoaded`, so the message alone cannot say whether its rows are a canonical full replace or a scoped subset; `observeResourcesLoadedRows` admits a write only when `msg.Provenance.CanonicalList()`. The zero value is `FetchProvenanceUnknown` and `CanonicalList()` reports **false** for it, so a lane added later that forgets to declare provenance under-populates the store rather than corrupting it — the failure a test catches, not the one that reaches disk. `ProvenanceForContinuation` classifies `KindFetchMore` continuations from the parent context for both the executor and the TUI adapter; the two lanes share it because independently-written classifiers are how they drift apart.

Provenance gates the screen lane too, in `handleResourcesLoadedEvent`: a screen match on `ResourceType` alone is not enough, because a canonical top-level list screen and a by-ID/filtered/child result can share a type. When a canonical screen matches a result whose provenance is not canonical, the screen is skipped and the top-down stack scan **continues** rather than returning. Continuing is load-bearing, not defensive: every by-ID/filtered/child navigation pushes its own screen on top of whatever it navigated from, so a non-canonical result's true target is always further down the stack whenever it is still open — returning early would swallow the message and strand that screen. Once the scan is exhausted the result is dropped, which is correct: a screen that has already popped has no rows left to apply it to. There is no controller-side row mirror; field updates and finding patches reach every screen through the store's `Amend`. List body rendering is memoized, not rebuilt per frame: `buildListBody` (`core/app/list_body.go`) keeps a per-`ListState` memo invalidated only when `rowsVersion` (bumped on every content-changing row mutation), filter, attention-only toggle, sort column/direction, or the enrichment generation changes.

**One save lane**: every per-type disk save goes through `Core.SaveTypeRows` (`core/runtime/probes.go`), which takes the `session.Pair` the save was prepared for — the chokepoint drops it when the operator has since switched pairs, so a queued save can never write one account's rows into another's directory (C9) — resolves save columns via a view-config-aware resolver injected with `SetSaveColumns`, and hands the rows to `reconcileTypeFile` — the on-disk reconciliation rules ([`design/cache-requirements.md`](design/cache-requirements.md) C5/C6a/C6b, including the exact-authoritative shrink) are reachable from exactly one chokepoint (the former sweep/list two-materializer split persisted different field sets for the same row under user-reordered columns).

**Two rules decide which of two writes to the same type file wins, and they answer different questions.** Within one session, ordering is decided by preparation sequence: `Store.PrepareSave` stamps each `WritePlan` with the next number for its target path, and `CommitSave` refuses a plan older than the last one already committed, so a plan suspended between preparation and commit can never land its staler bytes on top of a newer plan's. Across sessions there is no sequence to compare — the file on disk was written by a process that is gone — so `reconcileTypeFile` decides by content instead: a rows-carrying write that is itself truncated, shallower than what is stored, and whose IDs are a subset of the stored rows is a shallower page of the same list, and it keeps the stored rows (`rowIDsAreSubset`; `Count`/`Exact` still advance per C5). The two live in one place each and are not interchangeable: sequence cannot rank a stored file against this session's first write, and the subset test cannot tell two writes of this session apart, since both would look like refreshes of each other. An exact write is never subject to the subset rule — an exact row set is proof of the live population, including a deletion that makes it a strict subset.

The single availability-count producer is `Core.SaveAvailabilityFromRows`: it derives every type's count, truncation and issue badge from `RowStore` and hands them to `SaveAvailabilityCache`. Both persistence lanes — the sweep's completion save and the menu-badge writer behind the exact-total sync-back — go through it, so the file records one answer. The menu lane used to freeze a clone of the menu's own five maps and write from that, which made the rendered menu a second, older source of truth for the same file.

The numbered rules above (C1, C5, C6, C9) are the cache contract in [`design/cache-requirements.md`](design/cache-requirements.md).

**Related cache**: LRU mapping `"resourceType:resourceID"` → related check results. Avoids re-running related checks when re-entering a detail view for the same resource.

**Enricher caches**: Caching is optional, not automatic. The default is no cache. When an enricher does cache, it should use a session-scoped, feature-specific cache on `session.Session` and reach the enricher through `*awsclient.DetailEnrichmentCtx`, so cache lifetime matches session lifetime and is rotated by `session.Rotate()`. Examples: the policy document enricher (`PolicyDocumentCache`, keys `managed:<policyArn>` / `inline:<roleName>/<policyName>`) and the CloudFormation template enricher (`DetailDocCache`, version-keyed `cfn:<stackId>:<lastUpdatedUnixSeconds>`). An explicit refresh sets `SkipCache`, bypassing the cache read while still writing the fresh result back.

---

## Config System

### View Configuration

Each resource type has configurable list columns and detail field paths:

```yaml
# ~/.a9s/views/ec2.yaml
list:
  - title: Name
    path: Name
    width: 24
  - title: Instance ID
    path: InstanceId
    width: 20
detail:
  - InstanceId
  - State.Name
  - InstanceType
```

**Loading chain** (`config.Load()`):
1. `<configDir>/views/{shortname}.yaml` (user global config)
2. `.a9s/views/{shortname}.yaml` (per-project CWD overrides)

The base config directory defaults to `~/.a9s/` but can be overridden via the `A9S_CONFIG_FOLDER` environment variable (`core/config/config.go:ConfigDir()`).

Missing configs fall back to built-in defaults in `core/config/defaults_*.go` (one file per service category).

**Column Path vs Key:**
- `Path` — dot-notation SDK struct field path resolved by reflection (e.g., `"State.Name"`)
- `Key` — pre-extracted `Fields` map key populated by the fetcher (e.g., `"lifecycle"`)

### Theming

Styles live in `internal/tui/styles/`. The default theme is Tokyo Night Dark. Themes are YAML-configurable via `<configDir>/themes/*.yaml` (same `A9S_CONFIG_FOLDER` override as views). `ApplyTheme()` replaces the active theme and reinitializes all `lipgloss.Style` vars. `NO_COLOR` env var disables all colors.

---

## Demo Mode

`./a9s --demo` runs with synthetic fixture data — no AWS credentials needed.

**Architecture:**
- `core/demo/fixtures/` — per-service Go files returning hardcoded SDK response objects
- `core/demo/fakes/` — per-service fake API implementations backed by fixtures
- `core/demo/transport.go` — fake HTTP transport for STS (the only service without a typed fake interface)
- `demo.NewServiceClients()` wires fakes into a `*aws.ServiceClients` struct

The `isDemo` flag controls whether Wave 2 enrichment runs — demo mode skips it (no real AWS to query), while `--no-cache` on live AWS preserves full functionality.

Demo mode is the primary way to develop and test the TUI without AWS access.

---

## Web Mode (internal-only)

`./a9s --web` (or `A9S_MODE=web`) runs an HTTP server instead of the TUI — unpublished for now, internal-only.

**Architecture:**
- `core/web/` — `server.go`, `handlers.go`, `render.go`, `construct.go`, plus `templates/` and `static/`
- The server renders the same headless-controller state (`core/app.Controller`) the TUI consumes — list/detail/menu/cost bodies are produced once in `core/app` and adapter-rendered per surface
- `--web-addr` sets the listen address; `--web-allow-reveal` gates secret reveal over HTTP
- Integration coverage lives in `tests/integration/web/`; snapshot-driven e2e in `docs/testing/snapshot-web-e2e.md`

---

## App Lifecycle

```text
main.go → parseFlags → tui.New(profile, region, opts...)
```

**`New()` (synchronous constructor):**
- Builds the root `Model` with profile, region, key bindings
- Seeds the main menu as `stack[0]` (the menu is always present)
- Loads `ViewsConfig` from disk
- Creates `appCtx`/`appCancel` for graceful shutdown
- Bootstraps `runtime.Core` (`runtime.Bootstrap`) — its `session.Session` carries the `RowStore`, `RelatedCache`, and generation counters — and wraps it in the headless controller (`app.New(core)`)
- Applies all options (`WithClients`, `WithNoCache`, etc.)

**`Init()` (first Bubble Tea message):**
- If `preSuppliedClients != nil` (demo/test): emits synthetic `ClientsReadyMsg` immediately
- Otherwise: emits `InitConnectMsg{Profile, Region}` to start async AWS connection

**`ClientsReadyMsg` handler:**
- Stores `m.clients` on the model
- If `-c` flag was set (e.g., `--command ec2`), emits `NavigateMsg` to auto-open that resource type
- Normal (cached) path: fires `fetchIdentity()` for the status bar, then `loadAvailabilityCache()`
- noCache/demo path: skips identity fetch, runs `demoPrefetchCounts()` instead

**Options:**
- `WithClients(c)` — pre-supply AWS clients (used by demo mode and tests)
- `WithNoCache(true)` — disable disk cache persistence (availability probes still run via `demoPrefetchCounts()`)
- `WithIsDemo(true)` — mark session as demo mode (skips Wave 2 enrichment; set by `--demo` CLI bootstrap)
- `WithProfile(p)` — override the profile string (used in tests to set a specific profile without live AWS)
- `WithRegion(r)` — override the region string (used in tests to set a specific region without live AWS)
- `WithCommand("ec2")` — open directly to a resource type on startup
- `WithActiveTheme(name)` — set the initial active theme filename for the theme selector (used by `--theme` CLI flag)

---

## Extension Guide

Everything a resource type does is declared on ONE `catalog.ResourceTypeDef` struct literal. There is no `Register*` call and no feature-wiring `init()` anywhere — `make verify-zero-init` fails the build if one appears.

### Adding a New Resource Type

1. **Write the catalog literal** — one `catalog.ResourceTypeDef` entry in the matching `core/aws/catalog_<category>.go` file (see the category table above; e.g. the `"ec2"` literal in `catalog_compute.go` is the reference example). Everything is a struct field on the literal:
   - identity/display: `Name`, `ShortName`, `Aliases`, `Category`, `Columns`
   - `Color` — REQUIRED; findings-first classification (see Issue Counting & Attention Filter)
   - `Fetcher` — the Wave 1 paginated fetcher (wrapped via `fetcherWithClients`); add `FetchByIDs` when other types' related pivots target this type (lazy adds)
   - `Wave2` — `IssueEnricher{Fn: ..., Priority: ...}` when the type has hidden issues behind extra API calls; omit for none, `NoOpIssueEnricher` for explicit in-fetcher Wave 2 work; declare `IssueEnricherFieldKeys` for any `Fields` keys the enricher writes via `FieldUpdates`
   - `Related` — the right-column pivots. ⚠️ Governed by [`related-resources.md`](./related-resources.md); every entry needs an AWS API field citation or documented DevOps workflow reason
   - `Findings` — the declarative `FindingDef` table: `{Code, Phrase, Severity, Source}` for every Wave 1 and Wave 2 finding the type can emit
   - `Navigable`, `Children`, `Reveal`, `DetailEnrich`, `CloudTrailKey`, `Augment` — as needed
2. **Implement the fetcher** in `core/aws/<shortname>.go`, returning `resource.Resource` values with meaningful `ID`, `Name`, `Type`, `Fields`, `RawStruct`, and Wave-1 `Findings` (the legacy `Status`/`Issues` string fields no longer exist on the model — see §Resource Model). If the fetcher makes per-item describe calls, adopt the honest-degradation contract (`DetailsDeniedFindingDef` — a listed resource whose describe is denied stays as a `details denied` row; see §Wave 2 Issue Enrichment Pipeline).
3. **Add built-in view defaults** in `core/config/defaults_<category>.go`, then regenerate the on-disk views (`go run ./cmd/viewsgen/`).
4. **Add demo fixtures and fakes** — `core/demo/fixtures/<shortName>.go` (graph-connected, shared by demo mode and tests) plus the typed fake in `core/demo/fakes/`.
5. **Add tests** in `tests/unit/`: fetcher-level tests (narrow interface mocks) and TUI-level behavior tests (demo fakes). The catalog conformance gates (`tests/unit/architecture_conformance_test.go`, `qa_color_findings_conformance_test.go`) pick the new literal up automatically.

### Adding a Child View

1. Define a `ChildViewDef` in the parent literal's `Children` field, and the child-type entry (with its `ChildFetcher`) installed via `catalog.SetChildTypes`.
2. Implement the `PaginatedChildFetcher` in `core/aws/`.
3. Ensure the parent resource exposes the `ContextKeys` required by the child fetcher (`ResolveChildContext` resolves `"ID"`, `"Name"`, `"@parent.x"`, and `Fields` keys).
4. Add demo data and a navigation test that drives the real root model.

### Adding an Enricher

Both enricher kinds are catalog struct fields — `Wave2` for background issue enrichment, `DetailEnrich` for on-demand detail enrichment. Tests may override them per-test via `awsclient.SetWave2EnricherForTest` / `resource.SetDetailEnricherForTest` (with the matching cleanup); production code never registers anything.

1. Keep the base list/detail fetch path fast; defer expensive or optional data to the enricher.
2. Make the enricher idempotent and safe to re-run.
3. Ensure stale results are rejectable by generation plus resource context (Wave 2 results carry the `Gen` + `TypeGen` dual guard; detail enrichment carries `enrichGen`).
4. A Wave 2 enricher returns `IssueEnricherResult{Truncated, TruncatedIDs, Findings, AttentionDetails, FieldUpdates}` — findings only, no counts; the badge is recomputed centrally by `unifiedIssueCount`. Every emitted `Finding.Code` must have a matching `FindingDef` row in the literal's `Findings` table.
5. If the enricher caches, use a session-scoped cache on `session.Session` reached via `*awsclient.DetailEnrichmentCtx`, and document its scope and invalidation rules.
6. Add tests for success, error, stale-result rejection, and cache invalidation paths.

---

## Test Architecture

### Philosophy

Tests verify **behavior**, not implementation. A test should assert on what the user sees (rendered output, message flow) or what the function returns, never on internal state.

### Directory Layout

Unit tests live in two layers:

- `tests/unit/` — **black-box behavior tests** (package `unit`). The bulk of the suite: they exercise the app through its public surfaces — fetcher functions, the root `tui.Model`, the headless controller — and assert on rendered output or returned values. Run via `make test`; `make test-race` adds `-race`.
- **In-package white-box tests** — ~39 `_test.go` files colocated with the code they test, under `core/app`, `core/runtime`, `core/aws`, `core/session`, `internal/tui`, and `internal/tui/views` (e.g. `core/runtime/handlers_availability_test.go`, `internal/tui/views/coverage_topup_whitebox_test.go`). They cover unexported behavior that has no public seam — internal reconcilers, whitebox regression pins, coverage top-ups. `make test`/`make test-race` run both layers (`go test ./...`).
- `tests/integration/` — gated by `//go:build integration`. Run manually with specific flags.

**Where a new test belongs**: default to a black-box test in `tests/unit/` — if the behavior is observable through a public surface, test it there. Add an in-package white-box test only when the logic is unexported and cannot be exercised meaningfully through the public surface.

### Test Categories

**Fetcher tests** (`aws_*_test.go`):
Test the data transformation layer. Given a specific AWS API response, verify the fetcher returns the correct `[]resource.Resource` with expected fields.

```go
// Pattern: narrow interface mock → call real fetcher → assert on output
type mockEC2Client struct {
    output *ec2.DescribeInstancesOutput
    err    error
}
func (m *mockEC2Client) DescribeInstances(...) (*ec2.DescribeInstancesOutput, error) {
    return m.output, m.err
}

func TestFetchEC2_ParsesFields(t *testing.T) {
    mock := &mockEC2Client{output: ...}
    result, err := awsclient.FetchEC2Instances(ctx, mock, "")
    // Assert on result.Resources[0].Fields["instance_id"], .Status, etc.
}
```

**View behavior tests** (`app_enrich_test.go`, `child_view_*_test.go`):
Test the full message-driven flow. Construct a real `tui.Model`, drive it with messages, assert on rendered output.

```go
// Pattern: create app → send messages → assert on View() output
app := tui.New("demo", "us-east-1",
    tui.WithClients(demo.NewServiceClients()),
    tui.WithIsDemo(true),
    tui.WithNoCache(true),
    tui.WithProfile(demo.DemoProfile),
    tui.WithRegion(demo.DemoRegion))
m, _ := rootApplyMsg(app, tea.WindowSizeMsg{Width: 120, Height: 40})
m, _ = rootApplyMsg(m, messages.NavigateMsg{...})
content := stripANSI(rootViewContent(m))
if !strings.Contains(content, "expected text") {
    t.Error("...")
}
```

**QA story tests** (`qa_*_test.go`):
The most numerous category. Follow a helper-then-story pattern: `newXxxListModel(t)` navigates the demo app to the relevant view and loads fixture data, then individual test functions assert on specific rendered content (column headers, row data, ANSI color presence, key navigation effects).

**Design contract tests** (`*_design_contract_test.go`):
Build a demo app, inject a hand-crafted resource, assert rendered output matches known golden content.

**Config round-trip tests**:
Load `ViewsConfig`, render a detail model, assert field paths and column definitions survive the round-trip through rendering.

### Mock Patterns

Two mock layers serve different purposes:

| Layer | Location | When to Use |
|-------|----------|-------------|
| **Interface mocks** | `tests/unit/mocks_test.go` | Testing a single fetcher function in isolation |
| **Demo fakes** | `core/demo/fakes/*.go` | Testing TUI behavior — full app wired together |

**Interface mocks** are minimal: single-method structs with `output` + `err` fields. Each implements one narrow AWS API interface (e.g., `EC2DescribeInstancesAPI`). Use when testing data transformation in a fetcher.

**Demo fakes** are rich: they simulate pagination, relationships, multi-method API surfaces, and are backed by realistic fixture data. Use when you need the whole app wired together (e.g., testing view navigation, enrichment flow, related checks).

### Test Helpers

| Helper | File | Purpose |
|--------|------|---------|
| `stripANSI(s)` | `helpers_test.go` | Remove ANSI escape codes for plain-text assertions |
| `rootApplyMsg(m, msg)` | `tui_root_test.go` | Apply a message to the root model, return (model, cmd) |
| `rootViewContent(m)` | `tui_root_test.go` | Render the root model's view |
| `newDemoColdCacheApp(t)` | `testhelpers_demo_harness.go` | Create a demo app with cold cache |
| `buildResource(...)` | `helpers_external_test.go` | Construct a `resource.Resource` for tests |
| `configForType(name)` | `helpers_external_test.go` | Load ViewsConfig for a specific type |

### Writing New Tests

1. **Test behavior, not implementation** — assert on what the user sees or what the function returns
2. **Use demo fakes for TUI tests** — `tui.New("demo", "us-east-1", tui.WithClients(demo.NewServiceClients()), tui.WithIsDemo(true), tui.WithNoCache(true), tui.WithProfile(demo.DemoProfile), tui.WithRegion(demo.DemoRegion))`
3. **Use narrow interface mocks for fetcher tests** — one mock per AWS API method
4. **Always `stripANSI` before string assertions** — rendered output contains escape codes
5. **Clean up registries** — use `t.Cleanup(func() { resource.CleanupDetailEnricherForTest(...) })` after `resource.SetDetailEnricherForTest(...)`; detail enrichers are otherwise catalog-declared (`DetailEnrich` field), not registered
6. **If an enricher caches, test session scoping** — different `ServiceClients` instances must get independent caches automatically. If an enricher does not cache, no cache cleanup should be required.

### Integration Tests

Gated by `//go:build integration`. Two modes:
- **Demo integration** — uses `demo.NewServiceClients()`, no real AWS. Tests full boot sequence including `Init()` and message propagation.
- **Live AWS integration** — requires `A9S_CT_PROFILE=<profile>`. Tests real API calls against a live AWS account.

Run: `A9S_CT_PROFILE=<profile> go test -tags integration ./tests/integration/ -run TestName -count=1 -v -timeout 600s`

### Diagnosing Concurrency Defects: Call Ledger and Trace Stream

Two off-by-default, zero-cost-when-disabled facilities make detail-operation concurrency invariants (the same AWS operation fetched twice while one logical operation is in flight; a stale async result silently accepted) mechanically checkable instead of something a reviewer has to re-derive from reading code.

**Call ledger** (`core/aws/call_ledger.go`) — each of the four coalescing decorators (SFN, SNS, S3, Lambda; `core/aws/coalesce.go`) can record every call it makes as `CallExecuted` (the real AWS call fired) or `CallServed` (answered with no new request, either from the operation-scoped memo or by joining another goroutine's already-in-flight `singleflight.Group` call for the same key). Off by default: the production constructors (`NewCoalescingSFN`, etc.) construct with a nil `*CallLedger`, which every recording method tolerates as a no-op — no lock, no allocation on the undecorated path. A test opts in via the `...WithLedger` constructor variant (`NewCoalescingSFNWithLedger(api, ledger)` and its three siblings — SNS, S3, Lambda), drives calls through the decorator, then asserts on `ledger.Records()` (the full ordered call log) or `ledger.Duplicates()` (every `(operation, api, args)` key that executed more than once — the "at most one call per operation" invariant, made directly assertable; a correctly-deduped join or memo hit is `CallServed` and never counts as a duplicate).

**Trace stream** (`core/trace`) — a process-wide JSON-lines event stream covering four moments: a detail operation beginning (`core/runtime.Core.BeginDetailOperation`), an AWS call (executed vs served, the same call sites the ledger hooks), a cache read/write in the on-demand enrich engine (`core/aws/detail_enrich_engine.go`), and a result fold's accept/reject decision (`core/app/handle.go`). Off by default — `trace.Enabled()` is a single atomic bool read, checked before any event field is even built. Enabled by `cmd/a9s --trace <path>`, or directly in a test via `trace.Enable(io.Writer)` / `trace.EnableFile(path)`. Always writes to a file (or whatever `io.Writer` a test supplies) — never `os.Stdout` — so it can run alongside a live TUI session without corrupting the rendered frame.

### Deterministic Interleaving Harness

The call ledger and trace stream let a test observe one specific run. Neither helps write the test in the first place: the defect shape found repeatedly in the detail-operation orchestration layer is "action B lands while action A is still in flight" — a refresh completing before an earlier open's enrichment, a related-check batch resolving after a newer operation has already superseded it — and an example-based test ("given this state, this action produces that output") never enters that space, because it drives one action to completion before starting the next. `core/app/apptest` makes the interleaving itself the thing under test.

**The scheduler seam.** `Controller.ExecuteOne(ctx, req)` is the single per-task unit — resolve the task's dispatch snapshot, run it through `Core.ExecuteTaskAt`, fold any resulting event through `Handle` — that both production draining (`DrainSyncContextProgress`, which always steps `pending[0]`) and `apptest.Scheduler.Complete(i)` call. A test builds a `Scheduler`, `Submit`s the tasks a user action spawned, and calls `Complete` at whatever index it chooses — including completing a task an earlier action spawned strictly after a later action's own tasks have already completed, the exact ordering `DrainSync*` can never produce. Production gains no scheduler-awareness branch: the FIFO drain loop and the test-chosen order are two callers of the same extracted step, not two code paths.

**The explorer.** `apptest.Explorer` enumerates every legal interleaving of a fixed `[]Action` script against the completion order of the tasks those actions spawn, and runs each one to quiescence against its own fresh `Controller`:

```go
explorer := &apptest.Explorer{
    NewController: newDemoController,
    Actions: []apptest.Action{
        {Name: "open(ec2/i-1)", Run: openDetail},
        {Name: "refresh", Run: refresh},
    },
    Check: func(vs app.ViewState, pending []runtime.TaskRequest, events []trace.Event) error {
        return apptest.NoOrphanLoadingRelated(vs, pending)
    },
}
if f := explorer.Explore(ctx); f != nil {
    t.Fatalf("%s", f) // e.g. "open(ec2/i-1) → refresh → complete(enrich@op1) → complete(related@op2): ..."
}
```

`Explore` returns nil when every interleaving reaches quiescence clean, or a `*Failure` at the first violation whose `Replay` is a directly reproducible sequence ("open(ec2/i-1) → refresh → complete(enrich@op1) → complete(related@op2)") and whose `Err` is whichever invariant fired — the replay string is the point: a failing interleaving must be readable and re-runnable by a human, not just a stack trace from a flaky goroutine race.

**The five invariants**, exported so `Check` (or a hand-driven `Scheduler` test) can call any subset:

- `NoOrphanLoadingRelated` — a detail screen's related-panel row is never stuck `Loading` once no `related-check` task is left in flight to ever resolve it.
- `MonotonicDetailFold` — an accepted enrich/related fold's `OperationID` never regresses across a single Controller's fold history (the acceptance guard actually held, not just claimed to).
- `MonotonicCacheWrites` — a cache write for a given key never lands from an older operation than one that already wrote it (the freshness-guard contract, mechanically checked).
- `NoDuplicateAWSCalls` — wraps `aws.CallLedger.Duplicates()` as a descriptive failure; "at most one call per (operation, api, args)" reused, not reimplemented.
- `LatchCleared` — a `session.PendingDetailRefresh` entry armed by an explicit refresh is actually cleared by quiescence, not left stuck.

**Single-Controller scoping is a hard rule, not a suggestion.** `TraceRecorder`, `MonotonicDetailFold`, and `MonotonicCacheWrites` all key on `OperationID`, which is per-session (`session.DetailOpGen` restarts at 1 for every fresh `Controller`). `Explorer` constructs a fresh `Controller` per search-tree node, so a `TraceRecorder` wrapped around an entire `Explore()` call accumulates events from many independent op-ID namespaces and reports a violation that is really just two unrelated sessions' numbering colliding — not a regression. Set `Explorer.Trace = true` instead of managing a recorder by hand: the explorer then installs and resets a fresh `TraceRecorder` per replay and passes only that replay's own events to `Check`. Driving one `Scheduler` directly (one `Controller`, one `StartTraceRecorder`) is the other correctly-scoped shape. This was found the hard way — an early smoke-check of the harness itself reported a fold-order violation that turned out to be exactly this cross-session mixing, confirmed clean once the same interleaving was replayed against a single Controller.

**Why `core/app/apptest` is a separate package.** `core/app` already has small, stable non-`_test.go` seams that ship in the binary (`DrainSync`, `testing.go`'s `ApplyResourcesLoaded`) because a test-only file inside the package that other test packages import is an accepted, minimal exception. The interleaving harness is a larger amount of purpose-built machinery with no reason to compile into `cmd/a9s`, so it lives in its own package instead: nothing under `cmd/` or `internal/` imports `core/app/apptest`, which makes its absence from the production binary path a property of the import graph rather than a claim that has to be re-verified by inspection.

---

## Design Decisions

### Why no write operations?

a9s is designed for investigation and monitoring. Write operations are dangerous in a TUI where a single keypress could modify production infrastructure. The CLI and Console exist for mutations.

### Why not use generics for fetchers?

Fetchers return `any` for clients because each AWS service has a different client type. Type assertions happen inside each fetcher. This keeps the registry simple and avoids a complex generic type hierarchy.

### Why `RawStruct` and `Fields` both exist?

`Fields` is fast and sufficient for table columns. `RawStruct` enables deep field extraction via reflection for detail/YAML/JSON views without pre-extracting every possible field.

### Why view stack instead of a router?

The stack model maps naturally to drill-down navigation (list → detail → YAML). Each view preserves its state when covered. Esc always pops back to the previous state.

### Why generation counters?

Async operations (related checks, enrichment) can outlive the view that triggered them. Generation counters (`ConnectGen`, `AvailabilityGen`, `EnrichmentGen`, `DetailOpGen`, `EnrichmentTypeGen`) — all typed `domain.Gen` (`uint64`) after Phase 05a-gens — are incremented on context changes, causing stale in-flight results to be silently discarded.

**Every counter seeds at 1.** This is a correctness requirement, not a style choice. A counter starting at 0 makes the first session's stamps indistinguishable from an unstamped message, so a task dispatched before a profile switch still matches the counter after it — and its result folds into the new session. `ConnectGen` was the one counter left unseeded, which is how a previous account's caller identity, a decrypted secret, and cost data could all surface under the newly-selected profile. Two invariants keep it closed: every counter seeds at 1, and every production dispatch site stamps the live value. The second is enforced mechanically — `tests/unit/event_genstamp_guard_test.go` derives each event's generation field from its own `GenStamp()` method and fails if any production construction site ships unstamped, so a new dispatch site cannot silently reintroduce the gap. Detail-view work (enrichment + related checks) shares the single `DetailOpGen`-derived operation identity (see On-Demand Detail Enhancement) instead of per-concern counters — the earlier split (`RelatedGen`/`EnrichGen`) let the two halves of one user action disagree about freshness. `EnrichmentTypeGen` is a per-type counter for Wave 2: bumped on profile/region switch and on Ctrl+R when the active view is a top-level list with a registered enricher. The dual-generation guard (`Gen` on `EnrichmentCheckedMsg` for session-wide staleness, `TypeGen` for per-type rerun staleness) lets multiple types enrich concurrently while rerun invalidation only cancels the refreshed type.

**The list has its own ordering counter.** The session-wide counters answer "is this result from the account and region I am looking at now"; they say nothing about which of two fetches for the same list was requested last. `Session.ListFetchSeq` closes that: every canonical top-level list fetch takes the next value for its type at dispatch time (`Core.StampListFetchSeq`, called from the Controller's dispatch boundary and from the TUI's, exactly where the dispatch snapshot is stamped), carries it on the task and then on `messages.ResourcesLoaded.ListSeq`, and `Core.ListResultSuperseded` — read at each lane's own door, so a superseded result reaches neither a screen nor the row store — accepts only the newest value handed out. An on-entry verification overtaken by a Ctrl+R, a load-more or another refresh is discarded whole, whatever its rows look like. This replaced a pair of content heuristics, one on the screen and one in the row store, that guessed staleness from shape: they could recognise only a smaller, still-truncated subset of what was already there, so a superseded result of the same size or an exact one was applied, while a legitimate Ctrl+R reset to page 1 of an already-exact list was discarded. The enrichment-rerun reseed reads the same rule rather than ordering by its own counter: `EnrichmentTypeGen` says which rerun a result answers, never which of two results for the list is newer, and the two disagree whenever the later request is not itself a rerun (Ctrl+R, then `m` before the refresh lands). Only fetches whose result is a canonical top-level list are stamped: a child or filtered drill's load-more shares the type but not the screen, and stamping it would let it supersede the verification of the list beneath it. An unstamped result (a cache-seed replay, a by-ID or filtered fetch) carries no ordering claim and is applied as-is.

**A list result has one apply point.** The list view has no `messages.ResourcesLoaded` case: it renders and handles keys, and every result reaches a screen through the controller, where the sequence is checked. The view used to carry its own copy of the apply and of the auto-open-single-detail branches, unreachable in production because the root `Update` routes the message to its handler first — a second apply point past the check, and a second place for the auto-open rules to drift.

**A re-entered list is re-verified.** Opening a list the row store still holds in full is a cache hit for its rows, never for their freshness: `HandleNavigate` returns the `KindFetchResources` task for that branch itself, so the TUI and the headless controller both seed the retained rows, mark the surface refreshing, and fetch. The task is not synthesised by whichever adapter happens to notice — that asymmetry is what left the TUI treating a re-entry as permanently fresh while the headless lane re-verified.

### Why a separate enricher pattern?

Detail views render from pre-fetched `Fields`/`RawStruct`. Some data (like policy documents) requires additional API calls that are too expensive to make at list-fetch time. The enricher pattern fetches this data on demand when the user actually opens the detail, YAML, or JSON view.

### Why four separate caches?

Each cache serves a fundamentally different access pattern: disk cache survives restarts for instant startup; the session row store is the one in-memory copy of every type's rows (probe retention, disk seed, list cache, and lazy related adds are lanes into it, not separate copies) and enables instant back-navigation; related cache avoids redundant API fanouts; enricher caches prevent repeated expensive single-resource fetches. Collapsing them would conflate invalidation and eviction policies.
