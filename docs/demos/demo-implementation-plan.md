# Demo Mode Implementation Plan

`--demo` flag for a9s: run the full TUI with synthetic fixture data, no AWS credentials.

## Design Principles

1. **Minimal touch surface** -- demo mode is a data-injection concern, not a new UI mode. Views, layout, messages, and keys are unchanged.
2. **Intercept at the data layer** -- demo mode replaces the AWS connection and fetcher calls. Everything above `fetchResources` and `connectAWS` is unaware.
3. **Ship in release binary** -- fixtures live in production code (`internal/demo/`), not test files.
4. **No new message types** -- reuse `ClientsReadyMsg`, `ResourcesLoadedMsg`, `FlashMsg`. Demo mode produces the same messages real AWS does.

---

## Architecture Decision: Where to Intercept

The root model has two methods that talk to AWS:

- `connectAWS(profile, region)` -- returns `ClientsReadyMsg` with `*ServiceClients`
- `fetchResources(resourceType, s3Bucket, s3Prefix, r53ZoneId)` -- uses `m.clients` and the fetcher registry

**Decision:** In demo mode, both methods are replaced. `connectAWS` is a no-op that returns a sentinel `ClientsReadyMsg`. `fetchResources` returns fixture data from an in-memory map instead of calling AWS APIs.

This means:
- `m.clients` stays nil in demo mode (never constructed).
- The fetcher registry is never consulted.
- No AWS SDK code is invoked.

The alternative (mock `ServiceClients` with fake API implementations) was rejected because it would require implementing 40+ mock interfaces for 62 resource types, couples demo data to SDK types, and is far more complex for zero user-visible benefit.

---

## Design Decision: Scope of Demo Data

Per `demo-scenario.md`, only 4 resource types are shown in the VHS script:

| Resource | Views | Fixture Needs |
|----------|-------|---------------|
| EC2 | List + Detail + YAML | Fields + RawStruct (for reflection-based detail/YAML) |
| S3 Buckets | List | Fields only |
| S3 Objects | List (drill-down from bucket) | Fields only |
| Lambda | List | Fields only |
| RDS | List | Fields only |

**Decision:** Ship demo fixtures for these 5 (4 resource types + S3 objects). All other resource types return an empty list in demo mode (no error, just "0 resources"). This is honest -- the user sees the menu with all 62 types, navigates to any of them, and gets an empty table for non-demo types. The flash message can say "No resources (demo mode)" for empty results.

Rationale: Creating 62 fixture sets for the release binary is unnecessary bloat. The demo's purpose is a 30-second showcase, not a comprehensive simulation.

---

## Design Decision: Header Display

Current header format: `a9s v3.0.0  profile:region`

**Decision:** In demo mode, display `a9s v3.0.0  demo:us-east-1`.

- Profile string is `"demo"` (set in `main.go` when `--demo` is parsed).
- Region string is `"us-east-1"` (hardcoded default for demo).
- This matches the demo scenario spec exactly.

---

## Design Decision: Blocked Commands in Demo Mode

| Command | Behavior |
|---------|----------|
| `:ctx` / `:profile` | Flash error: `"Profile switching disabled in demo mode"` |
| `:region` | Flash error: `"Region switching disabled in demo mode"` |
| `:q` / `:quit` | Works normally |
| `:ec2`, `:s3`, etc. | Works normally (navigates, fetches demo data) |
| `Ctrl+R` (refresh) | Works normally (re-returns same fixtures) |
| `x` (reveal secret) | Flash error: `"Secret reveal disabled in demo mode"` |

**Decision:** The root model checks a `demoMode bool` field before dispatching `:ctx` and `:region` commands. Three lines of guard code in `executeCommand` and one in `handleReveal`.

---

## File Plan

### New Files

#### 1. `internal/demo/fixtures.go`

Package `demo` contains demo fixture data. No AWS SDK imports. Only depends on `internal/resource`.

```go
package demo

import "github.com/k2m30/a9s/internal/resource"

// DemoRegion is the synthetic region displayed in demo mode.
const DemoRegion = "us-east-1"

// DemoProfile is the synthetic profile displayed in demo mode.
const DemoProfile = "demo"

// GetResources returns fixture data for the given resource type.
// Returns nil, false for resource types without demo data.
func GetResources(resourceType string) ([]resource.Resource, bool) {
    fixtures, ok := demoData[resourceType]
    if !ok {
        return nil, false
    }
    return fixtures(), true
}

// GetS3Objects returns fixture data for S3 objects within a bucket.
// Returns nil, false if the bucket is not in demo data.
func GetS3Objects(bucket, prefix string) ([]resource.Resource, bool) {
    // Only one demo bucket has objects
    if bucket == "data-pipeline-logs" {
        return s3Objects(), true
    }
    return nil, false
}
```

The `demoData` map is a `map[string]func() []resource.Resource` keyed by short name. Each call returns a fresh slice (no shared mutation). Functions:

- `ec2Instances()` -- 6-8 instances, mix of running/stopped/pending. **Must include RawStruct** (populated with an `ec2types.Instance` struct) for the detail+YAML views.
- `s3Buckets()` -- 5-6 buckets with realistic names.
- `s3Objects()` -- 4-5 objects (mix of folders and files) for one specific bucket.
- `lambdaFunctions()` -- 5-6 functions with various runtimes.
- `rdsInstances()` -- 4-5 instances, mix of available/creating/stopped.

EC2 is the only type needing RawStruct because it is the only type shown in detail+YAML views in the demo script. This means `internal/demo/fixtures.go` will import the EC2 SDK types package (`github.com/aws/aws-sdk-go-v2/service/ec2/types`) -- a single SDK import, not the entire SDK.

**Fixture data source:** Adapt from the existing test fixtures in `tests/unit/fixtures_test.go` but:
- Make names more demo-friendly (e.g., `web-prod-01`, `api-staging-02` instead of `VPN`, `kafka`).
- Add a `stopped` and a `pending` EC2 instance for status color variety.
- Add RawStruct to EC2 instances with realistic fields.

#### 2. `internal/demo/fixtures_ec2.go`

Separated because EC2 fixtures are the most complex (RawStruct with `ec2types.Instance`). Contains:
- `ec2Instances() []resource.Resource` with full `RawStruct` populated.
- Helper to construct `ec2types.Instance` structs with realistic data.

This keeps `fixtures.go` clean (only Fields-based resources) and isolates the single SDK dependency.

#### 3. `tests/unit/demo_test.go`

Tests for the demo package:
- `TestGetResources_EC2` -- verifies EC2 fixtures have RawStruct, Fields, correct field keys.
- `TestGetResources_S3` -- verifies S3 bucket fixtures.
- `TestGetResources_Lambda` -- verifies Lambda fixtures.
- `TestGetResources_RDS` -- verifies RDS fixtures.
- `TestGetResources_Unknown` -- verifies unknown type returns nil, false.
- `TestGetS3Objects` -- verifies drill-down returns objects for demo bucket.
- `TestEC2RawStructYAML` -- verifies EC2 RawStruct marshals to valid YAML (catches nil pointer panics).
- `TestAllDemoResourcesHaveFieldKeys` -- for each demo resource, verify every Fields key matches the registered field keys in `resource.GetFieldKeys()`.

### Modified Files

#### 4. `cmd/a9s/main.go`

Add `--demo` / `-d` flag.

```go
var demoMode bool
flag.BoolVar(&demoMode, "demo", false, "Run with synthetic demo data (no AWS credentials needed)")
flag.BoolVar(&demoMode, "d", false, "Run with synthetic demo data (shorthand)")
```

Update `flag.Usage` to include the new flag.

Pass demo mode to `tui.New`:

```go
model := tui.New(profile, region, tui.WithDemo(demoMode))
```

When `demoMode` is true and profile/region are not explicitly set:
```go
if demoMode {
    if profile == "" {
        profile = demo.DemoProfile
    }
    if region == "" {
        region = demo.DemoRegion
    }
}
```

The `tui.WithDemo` pattern uses a functional option to avoid changing the `tui.New` signature in a breaking way. See below.

#### 5. `internal/tui/app.go`

**5a. Add `demoMode` field to `Model`:**

```go
type Model struct {
    // ... existing fields ...
    demoMode bool
}
```

**5b. Add functional option for New:**

```go
// Option configures the root Model.
type Option func(*Model)

// WithDemo enables demo mode with synthetic fixture data.
func WithDemo(enabled bool) Option {
    return func(m *Model) {
        m.demoMode = enabled
    }
}

func New(profile, region string, opts ...Option) Model {
    // ... existing construction ...
    for _, opt := range opts {
        opt(&m)
    }
    return m
}
```

**5c. Modify `Init()` -- skip AWS connection in demo mode:**

```go
func (m Model) Init() tea.Cmd {
    if m.demoMode {
        // No AWS connection needed. Send ClientsReadyMsg with nil clients
        // so the app transitions to "ready" state without AWS credentials.
        return func() tea.Msg {
            return messages.ClientsReadyMsg{}
        }
    }
    // ... existing connectAWS logic ...
}
```

**5d. Modify `fetchResources` -- return demo data when in demo mode:**

```go
func (m *Model) fetchResources(resourceType, s3Bucket, s3Prefix, r53ZoneId string) tea.Cmd {
    if m.demoMode {
        return m.fetchDemoResources(resourceType, s3Bucket)
    }
    // ... existing AWS fetcher logic ...
}

func (m *Model) fetchDemoResources(resourceType, s3Bucket string) tea.Cmd {
    return func() tea.Msg {
        // S3 object drill-down
        if s3Bucket != "" {
            resources, ok := demo.GetS3Objects(s3Bucket, "")
            if !ok {
                resources = nil
            }
            return messages.ResourcesLoadedMsg{
                ResourceType: resourceType,
                Resources:    resources,
            }
        }
        // Standard resource fetch
        resources, _ := demo.GetResources(resourceType)
        return messages.ResourcesLoadedMsg{
            ResourceType: resourceType,
            Resources:    resources,
        }
    }
}
```

Note: `fetchDemoResources` still returns a `tea.Cmd` (not inline data) to maintain the async message pattern. The cmd is trivial (no I/O), but the contract is preserved.

**5e. Modify `executeCommand` -- block `:ctx` and `:region` in demo mode:**

In the `executeCommand` function, add guards before the existing `case "ctx", "profile":` and `case "region":` blocks:

```go
case "ctx", "profile":
    if m.demoMode {
        return m, func() tea.Msg {
            return messages.FlashMsg{Text: "Profile switching disabled in demo mode", IsError: true}
        }
    }
    // ... existing profile logic ...

case "region":
    if m.demoMode {
        return m, func() tea.Msg {
            return messages.FlashMsg{Text: "Region switching disabled in demo mode", IsError: true}
        }
    }
    // ... existing region logic ...
```

**5f. Modify `handleReveal` -- block in demo mode:**

```go
func (m Model) handleReveal() (tea.Model, tea.Cmd) {
    if m.demoMode {
        return m, func() tea.Msg {
            return messages.FlashMsg{Text: "Secret reveal disabled in demo mode", IsError: true}
        }
    }
    // ... existing reveal logic ...
}
```

**5g. Modify `handleRefresh` -- allow refresh in demo mode (re-returns same fixtures):**

No changes needed. `handleRefresh` calls `fetchResources` which is already intercepted in 5d.

**5h. Modify `handleClientsReady` -- handle nil clients in demo mode:**

The existing code already handles this case:
```go
if clients, ok := msg.Clients.(*awsclient.ServiceClients); ok {
    m.clients = clients
}
```
When `msg.Clients` is nil (demo mode), this type assertion fails and `m.clients` stays nil. The region resolution code needs a guard:

```go
if m.region == "" && !m.demoMode {
    configPath := awsclient.DefaultConfigPath()
    m.region = awsclient.GetDefaultRegion(configPath, m.profile)
}
```

#### 6. `docs/demos/demo.tape` (update)

Update the VHS script to use `--demo`:
```
Type "a9s --demo"
```

And update the script to follow the demo scenario from `demo-scenario.md`.

### Files NOT Modified

- `internal/tui/messages/messages.go` -- no new message types needed.
- `internal/tui/views/*` -- views are data-agnostic; they render whatever `ResourcesLoadedMsg` contains.
- `internal/tui/keys/keys.go` -- no new key bindings.
- `internal/tui/layout/frame.go` -- header already renders whatever profile/region strings it receives.
- `internal/tui/styles/*` -- no style changes.
- `internal/resource/*` -- no changes to types or registry.
- `internal/aws/*` -- no changes to fetchers or clients.
- `internal/config/*` -- no config changes.

---

## Data Flow: Demo Mode

```
main.go: --demo flag parsed
    |
    v
tui.New("demo", "us-east-1", tui.WithDemo(true))
    |
    v
Model.Init() --> ClientsReadyMsg{Clients: nil, Err: nil}
    |
    v
handleClientsReady: m.clients stays nil, region stays "us-east-1"
    |
    v
User selects EC2 from menu --> NavigateMsg{TargetResourceList, "ec2"}
    |
    v
handleNavigate: pushes ResourceListModel, calls fetchResources("ec2", "", "", "")
    |
    v
fetchResources: demoMode=true, calls fetchDemoResources("ec2", "")
    |
    v
fetchDemoResources: returns tea.Cmd that calls demo.GetResources("ec2")
    |
    v
ResourcesLoadedMsg{Resources: [6 EC2 instances with RawStruct]}
    |
    v
ResourceListModel.Update: renders table with 6 rows, status colors work
    |
    v
User presses 'd' --> NavigateMsg{TargetDetail, Resource: &selectedEC2}
    |
    v
DetailModel: renders key-value pairs from RawStruct via fieldpath
    |
    v
User presses 'y' --> NavigateMsg{TargetYAML, Resource: &selectedEC2}
    |
    v
YAMLModel: marshals RawStruct to YAML, applies syntax coloring
```

S3 drill-down flow:
```
User types :s3 --> NavigateMsg{TargetResourceList, "s3"}
    |
    v
fetchDemoResources("s3", "") --> demo.GetResources("s3") --> 5 buckets
    |
    v
User presses Enter on "data-pipeline-logs" --> S3EnterBucketMsg
    |
    v
handleS3EnterBucket: pushes S3ObjectsList, calls fetchResources("s3", "data-pipeline-logs", "", "")
    |
    v
fetchDemoResources("s3", "data-pipeline-logs") --> demo.GetS3Objects("data-pipeline-logs", "")
    |
    v
ResourcesLoadedMsg with 4-5 objects
```

---

## Dependency Graph (New)

```
cmd/a9s/main.go --> internal/demo (for DemoProfile, DemoRegion constants)
cmd/a9s/main.go --> internal/tui  (existing)

internal/tui/app.go --> internal/demo (for GetResources, GetS3Objects)
internal/tui/app.go --> internal/tui/messages (existing)

internal/demo/fixtures.go --> internal/resource (Resource struct only)
internal/demo/fixtures_ec2.go --> internal/resource
internal/demo/fixtures_ec2.go --> github.com/aws/aws-sdk-go-v2/service/ec2/types (RawStruct)
```

This introduces exactly one new import edge: `tui/app.go --> demo`. The `demo` package only imports `resource` (and `ec2/types` for EC2 RawStruct), creating no cycles.

---

## Implementation Order

1. **Create `internal/demo/fixtures.go`** with the `GetResources` / `GetS3Objects` API and S3, Lambda, RDS fixtures (Fields-only).
2. **Create `internal/demo/fixtures_ec2.go`** with EC2 fixtures including `RawStruct`.
3. **Write `tests/unit/demo_test.go`** -- all fixture tests (TDD: write tests first, they fail, then fill in fixture data).
4. **Modify `cmd/a9s/main.go`** -- add `--demo` flag, pass to `tui.New`.
5. **Modify `internal/tui/app.go`** -- add `demoMode` field, `WithDemo` option, intercept `Init`, `fetchResources`, `executeCommand`, `handleReveal`, `handleClientsReady`.
6. **Update `docs/demos/demo.tape`** -- use `--demo` flag.
7. **Update `cmd/a9s/main.go` version** -- bump version string.
8. **Run full test suite** -- `go test ./tests/unit/ -count=1 -timeout 120s`.
9. **Run lint** -- `golangci-lint run ./...`.

---

## Risk Assessment

| Risk | Mitigation |
|------|-----------|
| EC2 RawStruct SDK types drift from real API | Test that RawStruct marshals to YAML without panic; fieldpath tests validate field extraction |
| Demo fixtures become stale as resource types evolve | Demo only covers 4 types; fixtures are static data, not coupled to API changes |
| Binary size increase from embedded fixtures | Negligible: ~5KB of Go literals, zero embedded files |
| User confusion (demo vs real mode) | Header clearly shows `demo:us-east-1`; blocked commands flash explicit errors |
| S3 drill-down only works for one bucket | Acceptable: demo is scripted, non-demo buckets return empty list |

---

## Size Estimate

| File | Lines (approx) |
|------|----------------|
| `internal/demo/fixtures.go` | ~120 |
| `internal/demo/fixtures_ec2.go` | ~180 |
| `tests/unit/demo_test.go` | ~150 |
| `cmd/a9s/main.go` changes | ~15 |
| `internal/tui/app.go` changes | ~50 |
| `docs/demos/demo.tape` changes | ~5 |
| **Total** | ~520 |
