# a9s Architecture Guide

This document is the first thing you should read when joining the project. It covers every concept, pattern, interconnection, and design decision you need to understand before touching code.

## What is a9s?

a9s is a read-only terminal UI for AWS. Think k9s for Kubernetes, but for AWS services. It uses [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) (the Elm Architecture for Go) and renders with [Lipgloss v2](https://github.com/charmbracelet/lipgloss).

**Read-only by design** — a9s never makes write calls to AWS. Every AWS API call is a List, Describe, or Get operation.

---

## Core Concepts

### The Elm Architecture (Bubble Tea)

Every interaction follows this loop:

```
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

Views communicate exclusively via typed messages (`internal/tui/messages/messages.go`). Views never import each other. The root `Model.Update()` routes messages to the appropriate handler.

Key messages:

| Message | Purpose |
|---------|---------|
| `NavigateMsg` | Push a new view (detail, YAML, JSON, etc.) |
| `PopViewMsg` | Request current view dismissal (emitted by HelpModel, IdentityModel on keypress) |
| `ResourcesLoadedMsg` | Deliver fetched resources to a list view |
| `EnterChildViewMsg` | Open a child resource list (e.g., S3 objects) |
| `RelatedCheckStartedMsg` | Start async related-resource checks |
| `RelatedCheckResultMsg` | Deliver one related-check result to detail view |
| `EnrichDetailMsg` | Start async detail enrichment (e.g., policy doc fetch) |
| `EnrichDetailResultMsg` | Deliver enriched resource back to detail/YAML/JSON view |
| `FlashMsg` | Show a temporary status/error message |
| `ClientsReadyMsg` | AWS clients connected and ready |

### View Stack

The app maintains a stack of views (`stack []views.View`):

```
[MainMenu] → [ResourceList] → [DetailModel] → [YAMLModel]
   ↑ bottom                              top ↑ (activeView)
```

- `pushView(v)` — append to stack
- `popView()` — remove top (Esc pops; `q` quits the app)
- `activeView()` — `stack[len(stack)-1]`, receives all messages via `updateActiveView()`

Views are created in `handleNavigate()` and pushed immediately. Async data arrives later via messages.

---

## Project Structure

```
cmd/
  a9s/              # main binary — CLI flags, tea.NewProgram
  readmegen/        # generates README.md from docs/README.tmpl.md
  viewsgen/         # generates ~/.a9s/views/*.yaml from built-in defaults
  refgen/           # generates views_reference.yaml from AWS SDK struct reflection
  preview/          # renders static TUI design mockups (no AWS)
  preview-pagination/  # pagination preview
  preview-policy-doc/  # policy document preview
  preview_detail/      # detail view preview

internal/
  aws/           # AWS service clients, resource fetchers, related checkers, enrichers
  buildinfo/     # version resolution (ldflags at build time)
  cache/         # on-disk availability cache with TTL (see Caching Layers)
  config/        # YAML config loading, built-in defaults per service
  demo/          # synthetic fixture data for --demo mode
    fixtures/    #   per-service Go structs (ec2.go, iam.go, etc.)
    fakes/       #   per-service fake API implementations
  fieldpath/     # struct field extraction via reflection (frozen — don't modify)
  resource/      # generic resource model, type registry, fetcher registry
  tui/           # root Bubble Tea app model
    keys/        #   key bindings (single Map struct, one file)
    layout/      #   frame rendering (borders, title, status line)
    messages/    #   inter-view message types
    styles/      #   Tokyo Night Dark palette, theming system
    text/        #   text utilities (PadOrTrunc for column rendering)
    views/       #   all view models (see View Types below)

tests/
  unit/          # all unit tests (run via `make test` with -race)
  integration/   # gated by //go:build integration
  testdata/      # hand-crafted JSON fixtures
```

---

## Resource Model

```go
// internal/resource/resource.go
type Resource struct {
    ID        string            // primary identifier
    Name      string            // display name
    Status    string            // lifecycle state (used for row coloring)
    Fields    map[string]string // pre-extracted string values for table columns
    RawStruct any               // original AWS SDK typed struct
}
```

- **Fields** — flat key-value pairs populated by each fetcher. Used for list table columns and simple detail rendering. Keys are snake_case (e.g., `"instance_id"`, `"vpc_id"`).
- **RawStruct** — the actual AWS SDK struct (e.g., `ec2types.Instance`, `s3types.Bucket`). Used by detail/YAML/JSON views via reflection for deep field path traversal (e.g., `"State.Name"`, `"Placement.AvailabilityZone"`).

### Resource Type Registration

Each AWS service registers itself in `init()` via `internal/resource/`:

```go
// internal/resource/registry.go
resource.RegisterPaginated("ec2", fetchEC2Instances)
resource.RegisterPaginatedChild("role_policies", fetchRolePolicies)
resource.RegisterRelated("ec2", []RelatedDef{...})
resource.RegisterNavigableFields("ec2", []NavigableField{...})
resource.RegisterEnricher("role_policies", enrichRolePolicy)
```

### Resource Type Definitions

```go
// internal/resource/types.go
type ResourceTypeDef struct {
    Name       string           // "EC2 Instances"
    ShortName  string           // "ec2"
    Category   string           // "Compute"
    Columns    []Column         // table columns
    Children   []ChildViewDef   // child view triggers (key bindings)
    // ...
}
```

Types are built once at package init by `buildResourceTypes()`. Categories map to `types_compute.go`, `types_networking.go`, etc.

---

## Fetcher Patterns

All registered in `internal/resource/registry.go`, implemented in `internal/aws/*.go`:

| Pattern | Signature | Use Case |
|---------|-----------|----------|
| **PaginatedFetcher** | `func(ctx, clients, token) (FetchResult, error)` | Top-level resource lists (EC2, S3, RDS, etc.) |
| **PaginatedChildFetcher** | `func(ctx, clients, parentCtx, token) (FetchResult, error)` | Child resource lists (S3 objects, role policies, ECS tasks) |
| **FilteredPaginatedFetcher** | `func(ctx, clients, filter, token) (FetchResult, error)` | Server-side filtered queries (CloudTrail events) |
| **RevealFetcher** | `func(ctx, clients, resourceID) (string, error)` | On-demand secret reveal (`x` key — Secrets Manager, SSM) |
| **Enricher** | `func(ctx, clients, Resource) (Resource, error)` | On-demand detail enrichment (policy documents) |

Each fetcher takes `clients any` and type-asserts to `*aws.ServiceClients` internally. This allows tests to inject mocks.

---

## View Types

All in `internal/tui/views/`:

| View | File(s) | Purpose |
|------|---------|---------|
| **MainMenuModel** | `mainmenu.go` | Category-grouped resource list with availability badges |
| **ResourceListModel** | `resourcelist.go` | Paginated table with filter, sort, child drill-down |
| **DetailModel** | `detail.go`, `detail_fields.go`, `detail_helpers.go` | Two-column: field list (left) + related panel (right) |
| **YAMLModel** | `yaml.go` | YAML dump of RawStruct with syntax highlighting + search |
| **JSONModel** | `json.go` | JSON dump of RawStruct with syntax highlighting + search |
| **RevealModel** | `reveal.go` | Displays revealed secret/parameter value |
| **HelpModel** | `help.go` | Context-sensitive key binding reference |
| **SelectorModel** | `selector.go` | Generic list picker (profile, region, theme selection) |
| **IdentityModel** | `identity.go` | Shows `sts:GetCallerIdentity` result |

Views implement the `View` interface (`views/view.go`):
```go
type View interface {
    View() string
    SetSize(w, h int)
    FrameTitle() string
    CopyContent() (string, string)
    GetHelpContext() HelpContext
}
```

`Update` is deliberately excluded from the interface because each view returns its own concrete type. The root model's `updateActiveView()` type-switches on each concrete type to dispatch and write back.

---

## Key Handling

### Bindings

All bindings are defined in one file: `internal/tui/keys/keys.go`. Single `Map` struct, single `Default()` constructor. Views receive `keys.Map` at construction and use `key.Matches(msg, m.keys.XYZ)`. No runtime rebinding.

Key highlights:
- `Enter` — drill into detail/child view
- `y` — YAML view, `J` (uppercase) — JSON view
- `p` — role policies (and other child views via `ChildViewDef.Key`)
- `r` — toggle related panel
- `x` — reveal secret value
- `Ctrl+R` — refresh (re-fetch resources, re-run related checks + enrichment)
- `/` — context-dependent: starts filter on list views, starts search on detail/YAML/JSON, view-dependent elsewhere
- `Esc` — back / cancel / pop view
- `q` — quit the application (in normal mode; swallowed when filter, command, or search input is active)
- `?` — help
- `i` — identity (STS caller identity)

### Global vs View-Local Key Resolution

The root model's `handleKeyMsg` (`app_handlers.go`) resolves keys in a specific order. Input modes take priority over global keys, which means `q`, `?`, etc. are swallowed during typing:

1. **`Ctrl+C`** — force quit (always global, never delegated)
2. **Input modes** — if filter, command, or search input is active, **all keys route to that input handler** (including `q`, `/`, `?`). This is why `q` doesn't quit while typing a filter.
3. **`?`** — help (global, pushes HelpModel)
4. **`i`** — identity (global, pushes IdentityModel)
5. **`q`** — quit (`tea.Quit`)
6. **`Esc`** — complex: delegates to view first (search cancel, right-column defocus), then pops
7. **`:`** — enter command mode
8. **`/`** — routed to `updateActiveView`, which handles it per-view (filter on lists, search on detail/YAML/JSON, ignored elsewhere)
9. **Everything else** — falls through to `updateActiveView` (view-local handling)

The critical insight: steps 2-3 mean `q` only quits in normal mode. During filter/command/search input, all keys are captured by the input handler. `/` is not globally handled — it's always delegated to the active view.

---

## Related Views (Right Column)

The detail view has a right-column panel showing related resources.

```go
// internal/resource/related.go
type RelatedDef struct {
    TargetType       string         // e.g., "vpc"
    DisplayName      string         // e.g., "VPCs"
    Checker          RelatedChecker // async function
    NeedsTargetCache bool           // true = reads from ResourceCache
}
```

**Two checker patterns:**
- **Live API** (`NeedsTargetCache: false`): Calls AWS directly (e.g., `DescribeTargetHealth`). Fast, specific.
- **Cache scan** (`NeedsTargetCache: true`): Reads from `ResourceCache` (snapshot of loaded lists). The dispatcher pre-fetches the target type if absent.

`handleRelatedCheckStarted` (`app_related.go`) fans out one goroutine per `RelatedDef`, capped by `maxConcurrentProbes=4`. Results include a `Generation uint64` to discard stale results after Ctrl+R or profile/region switch.

### Navigable Fields

```go
type NavigableField struct {
    FieldPath  string // e.g., "VpcId"
    TargetType string // e.g., "vpc"
}
```

In the detail view, navigable fields are underlined. Pressing Enter on one emits `RelatedNavigateMsg`, which pushes a filtered list of the target resource type.

---

## Enrichment (On-Demand Detail Enhancement)

When a detail, YAML, or JSON view opens for a resource type with a registered enricher, the app async-fetches additional data and merges it into the resource.

**Flow:**
```
View opens (detail, YAML, or JSON)
  → resource.HasEnricher(resType)?
    → increment enrichGen (invalidate prior in-flight results)
    → emit EnrichDetailMsg
      → handleEnrichDetail runs enricher in goroutine (10s timeout)
        → EnrichDetailResultMsg arrives
          → app.go: generation guard + error flash
            → view: ResourceType + ResourceID guard
              → m.res = enriched, re-render content
```

**Generation guard**: `enrichGen uint64` is incremented on every new enrichable view open, Ctrl+R, profile switch, and region switch. Stale results (wrong generation) are silently discarded. Generation=0 (test injection) is always accepted.

**Error handling**: Enrichment errors produce a `FlashMsg` with `IsError: true`. The view is not updated on error.

**Cache**: The enricher manages its own per-resource cache (e.g., `policyDocCache` keyed by ARN for managed policies, `roleName/policyName` for inline). Cache is session-scoped with explicit invalidation on profile switch (different AWS accounts may share the same role/policy names).

---

## Child Views

Child views are drill-down lists from a parent resource (e.g., IAM Role → Role Policies).

```go
// internal/resource/types.go
type ChildViewDef struct {
    ChildType      string            // "role_policies"
    Key            string            // trigger key: "p"
    ContextKeys    map[string]string // fetcher params from parent
    DisplayNameKey string            // field to show in title
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

The app has four distinct caches, each serving a different purpose:

| Cache | Location | Scope | Invalidation |
|-------|----------|-------|-------------|
| **Disk availability cache** | `internal/cache/` | Persisted at `~/.a9s/cache/<profile>--<region>.yaml` | TTL of 1 hour; file replaced atomically |
| **Resource cache** | `app.go` `resourceCache` | In-memory `map[string]*resourceCacheEntry` | Cleared on profile/region switch |
| **Related cache** | `app.go` `relatedCache` | In-memory LRU with fixed capacity | Cleared on profile/region switch; entry deleted on Ctrl+R |
| **Enricher caches** | Per-enricher (e.g., `policyDocCache` in `role_policies_enrich.go`) | In-memory, package-level | Cleared on profile switch; generation guards discard stale results |

**Disk availability cache** (`internal/cache/cache.go`): Tracks which resource types have resources and their counts. Loaded on startup to instantly grey-out empty types in the main menu. Structure: `File{Profile, Region, CheckedAt, Resources map[string]Entry}` where `Entry{HasResources, Count}`.

**Resource cache**: Stores the full view state of previously-viewed resource lists (resources, pagination, filter, sort, cursor position). Enables instant back-navigation without re-fetching.

**Related cache**: LRU mapping `"resourceType:resourceID"` → related check results. Avoids re-running related checks when re-entering a detail view for the same resource.

**Enricher caches**: Each enricher manages its own cache. The policy document enricher caches decoded documents by ARN (managed) or `roleName/policyName` (inline). Session-scoped but explicitly cleared on profile switch to prevent cross-account stale data.

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

The base config directory defaults to `~/.a9s/` but can be overridden via the `A9S_CONFIG_FOLDER` environment variable (`internal/config/config.go:ConfigDir()`).

Missing configs fall back to built-in defaults in `internal/config/defaults_*.go` (one file per service category).

**Column Path vs Key:**
- `Path` — dot-notation SDK struct field path resolved by reflection (e.g., `"State.Name"`)
- `Key` — pre-extracted `Fields` map key populated by the fetcher (e.g., `"lifecycle"`)

### Theming

Styles live in `internal/tui/styles/`. The default theme is Tokyo Night Dark. Themes are YAML-configurable via `<configDir>/themes/*.yaml` (same `A9S_CONFIG_FOLDER` override as views). `ApplyTheme()` replaces the active theme and reinitializes all `lipgloss.Style` vars. `NO_COLOR` env var disables all colors.

---

## Demo Mode

`./a9s --demo` runs with synthetic fixture data — no AWS credentials needed.

**Architecture:**
- `internal/demo/fixtures/` — per-service Go files returning hardcoded SDK response objects
- `internal/demo/fakes/` — per-service fake API implementations backed by fixtures
- `demo.NewServiceClients()` wires fakes into a `*aws.ServiceClients` struct

**API note**: `tui.WithDemo(true)` is a compatibility shim that calls `WithClients(demo.NewServiceClients())` internally. New code should use the explicit form: `tui.WithClients(demo.NewServiceClients())` + `tui.WithNoCache(true)`.

Demo mode is the primary way to develop and test the TUI without AWS access.

---

## App Lifecycle

```
main.go → parseFlags → tui.New(profile, region, opts...)
```

**`New()` (synchronous constructor):**
- Builds the root `Model` with profile, region, key bindings
- Seeds the main menu as `stack[0]` (the menu is always present)
- Loads `ViewsConfig` from disk
- Creates `appCtx`/`appCancel` for graceful shutdown
- Initialises `resourceCache`, `relatedCache`, generation counters
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
- `WithNoCache(true)` — disable background availability probes
- `WithCommand("ec2")` — open directly to a resource type on startup
- `WithDemo(true)` — compatibility shim (see Demo Mode above)

---

## Test Architecture

### Philosophy

Tests verify **behavior**, not implementation. A test should assert on what the user sees (rendered output, message flow) or what the function returns, never on internal state.

### Directory Layout

- `tests/unit/` — all unit tests. Run via `make test` (with `-race`).
- `tests/integration/` — gated by `//go:build integration`. Run manually with specific flags.

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
app := tui.New("demo", "us-east-1", tui.WithDemo(true))
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
| **Demo fakes** | `internal/demo/fakes/*.go` | Testing TUI behavior — full app wired together |

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
2. **Use demo fakes for TUI tests** — `tui.New("demo", "us-east-1", tui.WithDemo(true))`
3. **Use narrow interface mocks for fetcher tests** — one mock per AWS API method
4. **Always `stripANSI` before string assertions** — rendered output contains escape codes
5. **Clean up registries** — use `t.Cleanup(func() { resource.UnregisterEnricher(...) })` for temporary registrations
6. **Clean up caches** — call `awsclient.ClearPolicyDocumentCache()` in `t.Cleanup` if your test exercises the enricher

### Integration Tests

Gated by `//go:build integration`. Two modes:
- **Demo integration** — uses `demo.NewServiceClients()`, no real AWS. Tests full boot sequence including `Init()` and message propagation.
- **Live AWS integration** — requires `A9S_CT_PROFILE=<profile>`. Tests real API calls against a live AWS account.

Run: `A9S_CT_PROFILE=<profile> go test -tags integration ./tests/integration/ -run TestName -count=1 -v -timeout 600s`

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
Async operations (related checks, enrichment) can outlive the view that triggered them. Generation counters (`relatedGen`, `enrichGen`) are incremented on refresh/profile/region switch and on new view opens, causing stale in-flight results to be silently discarded.

### Why a separate enricher pattern?
Detail views render from pre-fetched `Fields`/`RawStruct`. Some data (like policy documents) requires additional API calls that are too expensive to make at list-fetch time. The enricher pattern fetches this data on demand when the user actually opens the detail, YAML, or JSON view.

### Why four separate caches?
Each cache serves a fundamentally different access pattern: disk cache survives restarts for instant startup; resource cache enables instant back-navigation; related cache avoids redundant API fanouts; enricher caches prevent repeated expensive single-resource fetches. Collapsing them would conflate TTL/invalidation/eviction policies.
