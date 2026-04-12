# a9s Architecture Guide

This document is the first thing you should read when joining the project. It covers every concept, pattern, and design decision you need to understand before touching code.

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
| `PopViewMsg` | Pop the top view (Esc/q) |
| `ResourcesLoadedMsg` | Deliver fetched resources to a list view |
| `EnterChildViewMsg` | Open a child resource list (e.g., S3 objects) |
| `RelatedCheckStartedMsg` | Start async related-resource checks |
| `RelatedCheckResultMsg` | Deliver one related-check result to detail view |
| `EnrichDetailMsg` | Start async detail enrichment (e.g., policy doc fetch) |
| `EnrichDetailResultMsg` | Deliver enriched resource back to detail view |
| `FlashMsg` | Show a temporary status/error message |
| `ClientsReadyMsg` | AWS clients connected and ready |

### View Stack

The app maintains a stack of views (`stack []views.View`):

```
[MainMenu] → [ResourceList] → [DetailModel] → [YAMLModel]
   ↑ bottom                              top ↑ (activeView)
```

- `pushView(v)` — append to stack
- `popView()` — remove top (Esc/q)
- `activeView()` — `stack[len(stack)-1]`, receives all messages

Views are created in `handleNavigate()` and pushed immediately. Async data arrives later via messages.

---

## Project Structure

```
cmd/
  a9s/           # main binary — CLI flags, tea.NewProgram
  readmegen/     # generates README.md from docs/README.tmpl.md
  viewsgen/      # generates ~/.a9s/views/*.yaml from built-in defaults
  refgen/        # generates views_reference.yaml from AWS SDK struct reflection
  preview/       # renders static TUI design mockups (no AWS)

internal/
  aws/           # AWS service clients, resource fetchers, related checkers
  buildinfo/     # version resolution (ldflags at build time)
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
    views/       #   all view models (see View Types below)

tests/
  unit/          # ~416 files — all unit tests
  integration/   # 14 files — gated by //go:build integration
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

`Update` is deliberately excluded from the interface because each view returns its own concrete type. The root model's `updateActiveView()` type-switches on each concrete type to dispatch.

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

When a detail view opens for a resource type with a registered enricher, the app async-fetches additional data and merges it into the resource.

**Flow:**
```
Detail view opens
  → resource.HasEnricher(resType)?
    → emit EnrichDetailMsg
      → handleEnrichDetail runs enricher in goroutine (10s timeout)
        → EnrichDetailResultMsg arrives
          → app.go: generation guard + error flash
            → detail.go: ResourceType + ResourceID guard
              → m.res = enriched, rebuild field list
```

**Generation guard**: `enrichGen uint64` is incremented on Ctrl+R, profile switch, and region switch. Stale results (wrong generation) are silently discarded. Generation=0 (test injection) is always accepted.

**Error handling**: Enrichment errors produce a `FlashMsg` with `IsError: true`. The detail view is not updated on error.

**Cache**: The enricher manages its own per-resource cache (e.g., `policyDocCache` keyed by ARN). Cache is session-scoped — no invalidation, no TTL.

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
1. `~/.a9s/views/{shortname}.yaml` (user global config)
2. `.a9s/views/{shortname}.yaml` (per-project CWD overrides)

Missing configs fall back to built-in defaults in `internal/config/defaults_*.go` (one file per service category).

**Column Path vs Key:**
- `Path` — dot-notation SDK struct field path resolved by reflection (e.g., `"State.Name"`)
- `Key` — pre-extracted `Fields` map key populated by the fetcher (e.g., `"lifecycle"`)

### Theming

Styles live in `internal/tui/styles/`. The default theme is Tokyo Night Dark. Themes are YAML-configurable via `~/.a9s/themes/*.yaml`. `ApplyTheme()` replaces the active theme and reinitializes all `lipgloss.Style` vars. `NO_COLOR` env var disables all colors.

---

## Demo Mode

`./a9s --demo` runs with synthetic fixture data — no AWS credentials needed.

**Architecture:**
- `internal/demo/fixtures/` — per-service Go files returning hardcoded SDK response objects
- `internal/demo/fakes/` — per-service fake API implementations backed by fixtures
- `demo.NewServiceClients()` wires fakes into a `*aws.ServiceClients` struct
- `tui.WithDemo(true)` injects the fake clients and disables cache probes

Demo mode is the primary way to develop and test the TUI without AWS access.

---

## Key Bindings

All bindings in one file: `internal/tui/keys/keys.go`. Single `Map` struct, single `Default()` constructor. Views receive `keys.Map` at construction and use `key.Matches(msg, m.keys.XYZ)`. No runtime rebinding.

Key highlights:
- `Enter` — drill into detail/child view
- `y` — YAML view, `j` — JSON view
- `p` — role policies (and other child views via `ChildViewDef.Key`)
- `r` — toggle related panel
- `x` — reveal secret value
- `Ctrl+R` — refresh (re-fetch resources, re-run related checks + enrichment)
- `/` — filter (list) or search (detail/YAML/JSON)
- `Esc` — back / cancel
- `q` — quit (from menu) or back (from views)

---

## App Lifecycle

```
main.go → parseFlags → tui.New(profile, region, opts...)
  → Init():
      if preSuppliedClients (demo/test):
        emit ClientsReadyMsg immediately
      else:
        emit InitConnectMsg → async AWS connection
  → ClientsReadyMsg → store clients, push initial view
  → User interaction loop (Update/View cycle)
```

**Options:**
- `WithDemo(true)` — inject fake clients, disable cache
- `WithClients(c)` — pre-supply clients (used by tests)
- `WithCommand("ec2")` — open directly to a resource type
- `WithNoCache(true)` — disable background availability probes

---

## Test Architecture

### Directory Layout

- `tests/unit/` — ~416 files, all unit tests. Run via `make test` (with `-race`).
- `tests/integration/` — 14 files, gated by `//go:build integration`. Run manually with specific flags.

### Test Categories

**Fetcher tests** (`aws_*_test.go`):
Each test creates a lightweight interface mock implementing a single AWS API method, calls the real fetcher function directly, and asserts on returned `[]resource.Resource`.

```go
type mockEC2Client struct {
    output *ec2.DescribeInstancesOutput
    err    error
}
func (m *mockEC2Client) DescribeInstances(...) (*ec2.DescribeInstancesOutput, error) {
    return m.output, m.err
}
```

**View behavior tests** (`app_enrich_test.go`, `child_view_*_test.go`):
Construct a real `tui.Model` via `tui.New("demo", "us-east-1", tui.WithDemo(true))`, drive with `model.Update(tea.Msg)`, assert on `m.View()` output after `stripANSI()`.

**QA story tests** (`qa_*_test.go`):
The most numerous category. Follow a helper-then-story pattern: `newXxxListModel(t)` navigates to the relevant view, then individual tests assert on rendered content (columns, colors, navigation).

**Design contract tests** (`*_design_contract_test.go`):
Build a demo app, inject a hand-crafted resource, assert rendered output matches known golden content.

**Config round-trip tests**:
Load `ViewsConfig`, render a detail model, assert field paths and column definitions survive the round-trip.

### Mock Patterns

Two mock layers serve different purposes:

| Layer | Location | Used For |
|-------|----------|----------|
| **Interface mocks** | `tests/unit/mocks_test.go` | Fetcher-layer tests — one struct per AWS API method |
| **Demo fakes** | `internal/demo/fakes/*.go` | TUI-level tests — full fake implementations backed by fixtures |

**Interface mocks** are minimal: single-method structs with `output` + `err` fields. Used when testing a fetcher function in isolation.

**Demo fakes** are rich: they simulate pagination, relationships, and multi-method API surfaces. Used when you need the whole app wired together (e.g., testing view navigation, enrichment flow).

### Test Helpers

- `stripANSI(s)` — remove ANSI escape codes for plain-text assertions
- `rootApplyMsg(m, msg)` — apply a message to the root model, return updated model + cmd
- `rootViewContent(m)` — render the root model's view
- `newDemoColdCacheApp(t)` — create a demo app with cold cache for integration-style unit tests

### Integration Tests

Gated by `//go:build integration`. Two modes:
- **Demo integration** — uses `demo.NewServiceClients()`, no real AWS. Tests full boot sequence.
- **Live AWS integration** — requires `-args -a9s.profile <profile>`. Tests real API calls.

---

## Design Decisions

### Why no write operations?
a9s is designed for investigation and monitoring. Write operations are dangerous in a TUI where a single keypress could modify production infrastructure. The CLI and Console exist for mutations.

### Why not use generics for fetchers?
Fetchers return `any` for clients because each AWS service has a different client type. Type assertions happen inside each fetcher. This keeps the registry simple and avoids a complex generic type hierarchy.

### Why `RawStruct` and `Fields` both exist?
`Fields` is fast and sufficient for table columns. `RawStruct` enables deep field extraction via reflection for detail/YAML/JSON views without pre-extracting every possible field.

### Why view stack instead of a router?
The stack model maps naturally to drill-down navigation (list → detail → YAML). Each view preserves its state when covered. Esc/q always pops back to the previous state.

### Why generation counters?
Async operations (related checks, enrichment) can outlive the view that triggered them. Generation counters (`relatedGen`, `enrichGen`) are incremented on refresh/profile/region switch, causing stale in-flight results to be silently discarded.

### Why a separate enricher pattern?
Detail views render from pre-fetched `Fields`/`RawStruct`. Some data (like policy documents) requires additional API calls that are too expensive to make at list-fetch time. The enricher pattern fetches this data on demand when the user actually opens the detail view.
