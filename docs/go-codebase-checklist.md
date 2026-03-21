# a9s Codebase Checklist

Tailored for a Go TUI application built with Bubble Tea v2, Lipgloss v2, and AWS SDK Go v2.

---

## Design Principles

### KISS (Keep It Simple, Stupid)
- [ ] No abstraction without a concrete second use case — three similar lines > a premature helper
- [ ] No configuration for things that have one correct value
- [ ] No generics unless the type constraint eliminates real duplication across 3+ call sites
- [ ] Control flow is linear and obvious — no clever tricks, no callback chains
- [ ] A new contributor can understand any file in under 5 minutes

### DRY (Don't Repeat Yourself)
- [ ] Shared logic extracted only when identical code appears in 3+ places (not 2)
- [ ] Extracted helpers live as close to their callers as possible (same package > utility package)
- [ ] Test helpers live in `tests/unit/helpers_*.go` — not duplicated across test files
- [ ] AWS fetchers follow a consistent pattern but are NOT generated or abstracted into a framework — each is a straightforward function
- [ ] Style constants defined once in `styles/palette.go` — never inline hex strings

### YAGNI (You Aren't Gonna Need It)
- [ ] No feature flags, plugin systems, or extension points that aren't actively used
- [ ] No interfaces defined before they have 2+ implementations or a mock requirement
- [ ] No backward-compatibility shims for removed features — delete cleanly
- [ ] No configuration options for behavior nobody has asked to customize
- [ ] `views.yaml` overrides exist because users need them — not because they might someday

### TDA (Tell, Don't Ask)
- [ ] Views receive messages and act — they don't query the root model for state
- [ ] `Update()` returns commands — callers don't inspect model fields to decide what to do next
- [ ] AWS fetchers receive clients and return resources — they don't reach into global state
- [ ] `NavigateMsg` tells the root where to go — views don't ask "what view am I?"
- [ ] Flash messages tell the root what to display — views don't read flash state directly

### SOLID

**Single Responsibility:**
- [ ] Each view handles one screen (list, detail, YAML, help, profile, region, reveal)
- [ ] Each fetcher handles one AWS resource type
- [ ] `app.go` is the only file that knows about the view stack and routing
- [ ] `messages/` is data definitions only — no behavior

**Open/Closed:**
- [ ] New resource types added by creating a fetcher file + registering — no existing code modified
- [ ] New views added by implementing the `View` interface + adding a message case in `app.go`
- [ ] Styles extensible via `views.yaml` without touching Go code

**Liskov Substitution:**
- [ ] All views satisfy the `View` interface — root model treats them uniformly via the view stack
- [ ] All AWS fetchers satisfy the `FetchFunc` signature — registry dispatches without type knowledge

**Interface Segregation:**
- [ ] AWS interfaces are single-method — mocks implement exactly what they test
- [ ] `View` interface is minimal (5 methods) — no view is forced to implement unused capabilities
- [ ] `Filterable` is opt-in — only views that support filtering implement it

**Dependency Inversion:**
- [ ] Views depend on `messages` and `keys` (abstractions) — not on `app` (concrete root)
- [ ] Fetchers depend on SDK interfaces — not on concrete SDK clients
- [ ] Root model depends on `View` interface — not on concrete view types (except in the type switch, which is an accepted Bubble Tea pattern)

---

## Package Design
- [ ] Package name describes what it provides, not what it does (`resource` not `resourceutils`)
- [ ] No circular imports between packages
- [ ] `internal/` used to enforce API boundaries — all domain packages live under `internal/`
- [ ] Package paths are flat over deep (`internal/tui/views` not `internal/tui/components/views/models`)
- [ ] Dependency direction is strictly enforced:
  - `views` -> `keys`, `messages`, `styles`, `fieldpath`, `config`, `resource`
  - `messages` -> `resource` only
  - `layout` -> `styles` only
  - `text` -> stdlib + lipgloss + `charmbracelet/x/ansi` only
  - `styles` -> stdlib + lipgloss only
- [ ] `views` never imports `layout` (root composes frame around view content)
- [ ] `views` never imports `app` (communicate via messages only)
- [ ] `messages` never imports anything in `tui/`

---

## Interfaces
- [ ] AWS service interfaces are single-method, defined in `internal/aws/interfaces.go`
- [ ] Each interface wraps exactly one SDK operation (e.g., `EC2DescribeInstancesAPI`)
- [ ] Functions return concrete types, accept interfaces
- [ ] The `View` interface in `views/view.go` is the only view-level interface
- [ ] `Filterable` is an opt-in interface for views that support `/` filtering
- [ ] No preemptive interfaces — created only when mocking is needed or 2+ implementations exist

---

## Error Handling
- [ ] Errors wrapped with context: `fmt.Errorf("doing X: %w", err)`
- [ ] AWS errors classified via `ClassifyAWSError()` before display
- [ ] API errors surfaced to the user via `messages.APIErrorMsg` -> flash message
- [ ] No ignored errors — every `_ =` has a justifying comment
- [ ] Config loading returns `(nil, nil)` for "not found" vs `(nil, error)` for parse failures

---

## Dependency Injection
- [ ] Constructor injection via `New*()` functions — keys, config, and clients passed in
- [ ] Composition root is `cmd/a9s/main.go` -> `tui.New()` -> view constructors
- [ ] AWS clients created once in `connectAWS()` and stored on root model
- [ ] No service locators or runtime dependency lookup

---

## Bubble Tea v2 Architecture
- [ ] `Init() tea.Cmd` (not `(tea.Model, tea.Cmd)` — that was BT v1)
- [ ] Root `View() tea.View` via `tea.NewView(string)` — child views return `string`
- [ ] Root `Update() (tea.Model, tea.Cmd)` — child views `Update() (ConcreteType, tea.Cmd)`
- [ ] ALL I/O in `tea.Cmd` — never block `Update()`
- [ ] `View()` is pure — no side effects, no I/O, no sorting, no data mutation
- [ ] `tea.Tick()` used for timed operations (flash clear, spinner)
- [ ] `tea.Batch()` used to combine multiple commands
- [ ] Alt screen set on all `tea.View` returns: `v.AltScreen = true`
- [ ] Minimum terminal size guarded (60 cols, 7 rows) before rendering

---

## View Stack
- [ ] Views stored as `[]views.View` with push/pop, not a flat enum or router
- [ ] Only the root model satisfies `tea.Model`; child views are plain structs
- [ ] `SetSize(w, h)` called on every view in the stack when window resizes
- [ ] `propagateSize()` iterates the full stack, not just the active view
- [ ] `popView()` returns `false` when only one entry remains (prevents empty stack)
- [ ] `FrameTitle()` on every view provides the frame header text
- [ ] `CopyContent()` on every view provides clipboard content and label

---

## Message Contracts
- [ ] Messages are data-only structs — no methods, no behavior
- [ ] Messages defined in `internal/tui/messages/` with zero upward imports
- [ ] `NavigateMsg` carries a `ViewTarget` enum + optional resource/type data
- [ ] `PopViewMsg` is an empty struct — the root model handles stack manipulation
- [ ] Flash messages use a generation counter to prevent stale clears
- [ ] `ClientsReadyMsg.Clients` is `interface{}` to avoid importing `aws/` from messages

---

## Key Bindings
- [ ] All bindings defined in `internal/tui/keys/keys.go` — no inline `key.NewBinding`
- [ ] `key.NewBinding(key.WithKeys(...), key.WithHelp(...))` pattern used consistently
- [ ] `key.Matches(msg, binding)` used for dispatch — never raw string comparison
- [ ] Single `keys.Map` struct passed to all views via constructor

---

## AWS Resource Types
- [ ] Each resource type has a `ResourceTypeDef` in `internal/resource/types.go`
- [ ] Each fetcher registered via `resource.Register()` in an `init()` function
- [ ] Fetcher signature is `func(ctx context.Context, clients interface{}) ([]Resource, error)`
- [ ] Fetcher type-asserts `clients` to `*awsclient.ServiceClients` internally
- [ ] Column keys in `ResourceTypeDef` match the field keys populated by the fetcher
- [ ] Default view definitions in `internal/config/defaults.go` exist for every resource type
- [ ] `views.yaml` entries are optional overrides — defaults always work standalone
- [ ] `refgen` tool can regenerate `views_reference.yaml` from SDK struct reflection

---

## Resource Model
- [ ] `resource.Resource` has `ID`, `Name`, `Status`, `RawStruct interface{}`, `Fields map[string]string`
- [ ] `Fields` is the primary data source for list columns — populated by fetchers
- [ ] `RawStruct` holds the original AWS SDK struct for detail/YAML views
- [ ] `fieldpath` extracts nested fields from `RawStruct` via reflection paths
- [ ] `FindResourceType()` matches by `ShortName` or any alias (case-insensitive)

---

## Reflection (fieldpath)
- [ ] `fieldpath` package is FROZEN — never modify
- [ ] Handles nil pointers, slices, maps, and nested structs gracefully
- [ ] Returns empty string for missing/nil fields — never panics
- [ ] All reflection paths validated against actual SDK structs (via `refgen`)

---

## Config / YAML
- [ ] `views.yaml` uses ordered YAML maps for column definitions (parsed via `yaml.Node`)
- [ ] Config lookup chain: `./views.yaml` -> `$A9S_CONFIG_FOLDER/views.yaml` -> `~/.a9s/views.yaml`
- [ ] `GetViewDef()` merges user config with defaults — partial overrides supported
- [ ] Missing config file is not an error — defaults are always sufficient

---

## Rendering
- [ ] `lipgloss.Width(s)` for ANSI-aware string width — NEVER `len(s)` or `utf8.RuneCountInString(s)`
- [ ] Frame constructed manually (not via `lipgloss.Border()`) per design spec
- [ ] Header and frame composed by root `View()` — views only return inner content
- [ ] Status values rendered with color via `styles.StatusStyle()` mapping
- [ ] Truncation uses ANSI-aware functions, never naive string slicing

---

## Concurrency
- [ ] All AWS API calls run in `tea.Cmd` closures (goroutines managed by Bubble Tea)
- [ ] No manual goroutine management — Bubble Tea's runtime handles scheduling
- [ ] `context.Background()` used in `tea.Cmd` closures (not stored on structs)
- [ ] No shared mutable state between `tea.Cmd` closures and the model
- [ ] Spinner runs via `tea.Tick` — not a background goroutine

---

## Project Structure
- [ ] `/cmd` contains only binary entrypoints with thin `main()`
- [ ] `/cmd/refgen` is a dev-time code generation tool (no AWS credentials needed)
- [ ] `/internal` holds all domain packages
- [ ] `/tests/unit/` contains all unit tests (external test package)
- [ ] `/tests/integration/` is behind build tags
- [ ] `/docs/design/` holds the visual design spec (architectural truth)

---

## init() Functions
- [ ] `init()` in `internal/aws/*.go` is acceptable — registers fetchers in the resource registry
- [ ] `init()` in `internal/tui/styles/` is acceptable — initializes computed style values
- [ ] No other `init()` functions exist outside these two locations
- [ ] Each `init()` is a single `resource.Register()` call — no complex logic

---

## Code Style
- [ ] `context.Context` is always a function parameter — never stored in structs
- [ ] Tests use table-driven `[]struct{ name, input, expected }` pattern
- [ ] No naked returns
- [ ] Every `//nolint` has an explanatory comment

---

## Module / Dependency Management
- [ ] One `go.mod` at the project root
- [ ] stdlib preferred — every external dependency is justified
- [ ] AWS SDK service imports are per-service (not the monolithic SDK)
- [ ] Bubble Tea v2 / Lipgloss v2 / Bubbles v2 are the `charm.land/` imports (not `github.com/charmbracelet/`)

---

## Testing
- [ ] TDD: failing tests written BEFORE implementation code
- [ ] All resource types tested — never just one representative type
- [ ] Subtests use `t.Run(tc.name, ...)` for parallel-safe isolation
- [ ] AWS mocks implement single-method interfaces from `interfaces.go`
- [ ] Mocks return canned data — no real AWS calls in unit tests
- [ ] `resource.Register()` / `resource.Unregister()` used for test isolation
- [ ] View tests construct models directly and call `Update()` / `View()` — no `tea.Program`
- [ ] Integration tests behind `//go:build integration` tag
- [ ] `go test -race ./...` should pass (no data races in unit tests)
- [ ] Version bumped in `cmd/a9s/main.go` after every code change

---

## Size Constraints

### File
- [ ] No file exceeds 500 lines (views and test files may push this — flag for review)
- [ ] Each file has one primary type; file is named after it
- [ ] Test files may be larger than source (table-driven tests with fixtures are verbose)

### Function / Method
- [ ] No function exceeds 50 lines (Update/View handlers may need more — flag for review)
- [ ] No more than 3 levels of nesting — early returns used over else chains
- [ ] No more than 5 parameters — options struct introduced beyond that
- [ ] Cyclomatic complexity kept low — type switches in `updateActiveView` are an accepted exception

### Struct
- [ ] No god structs — `Model` (root) is the known exception; its field count is monitored
- [ ] View models hold only their own state — no references to other views
- [ ] `ServiceClients` grows with resource types but each field is a typed SDK client

### Package
- [ ] No package exports more than ~15 symbols (except `resource` and `messages`)
- [ ] No package contains more than 10 source files (except `aws/` fetchers)

### Interface
- [ ] No interface has more than 5 methods
- [ ] AWS interfaces are strictly single-method
- [ ] `View` interface has exactly 5 methods: `View`, `SetSize`, `FrameTitle`, `CopyContent`, `GetHelpContext`

### Test
- [ ] No table test has more than 20 cases — split into logical groups otherwise
- [ ] No more than 3 levels of `t.Run` nesting

---

## Linting
- [ ] `.golangci.yml` configured at project root
- [ ] `govet` enabled (with `fieldalignment` disabled)
- [ ] `errcheck` enabled
- [ ] `staticcheck` enabled
- [ ] `gosimple` enabled (implicit via `staticcheck` in golangci-lint v2)
- [ ] `unused` enabled
- [ ] CI runs `go test` and `go build` on push/PR

---

## Meta
- [ ] No pattern requires a comment to explain why it exists — abstraction is self-evident
- [ ] Size limit violations are treated as domain model problems, not limit problems
- [ ] Design spec (`docs/design/design.md`) is the visual truth — code conforms to it, not the other way around
- [ ] `views.yaml` and `defaults.go` are kept in sync — every resource type has both
