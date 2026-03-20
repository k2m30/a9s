# a9s Codebase Checklist

Tailored for a Go TUI application built with Bubble Tea v2, Lipgloss v2, and AWS SDK Go v2.

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
- [ ] `resource.Resource` has `ID`, `Name`, `Status`, `RawObject interface{}`, `Fields map[string]string`
- [ ] `Fields` is the primary data source for list columns — populated by fetchers
- [ ] `RawObject` holds the original AWS SDK struct for detail/YAML views
- [ ] `fieldpath` extracts nested fields from `RawObject` via reflection paths
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
- [ ] `gosimple` enabled
- [ ] `unused` enabled
- [ ] CI runs `go test` and `go build` on push/PR

---

## Meta
- [ ] No pattern requires a comment to explain why it exists — abstraction is self-evident
- [ ] Size limit violations are treated as domain model problems, not limit problems
- [ ] Design spec (`docs/design/design.md`) is the visual truth — code conforms to it, not the other way around
- [ ] `views.yaml` and `defaults.go` are kept in sync — every resource type has both
