# Demo Mode Implementation Tasks

Ordered task breakdown derived from `demo-implementation-plan.md`.
Each task has a single owner agent, explicit file targets, dependencies, and acceptance criteria.

---

## Task 1: Demo fixture tests (TDD -- tests first)

- **Agent:** a9s-qa
- **Files to create:**
  - `tests/unit/demo_test.go`
- **Dependencies:** None (tests written against the `internal/demo` API contract before implementation exists)
- **Description:**
  Write all tests for the `internal/demo` package. Tests import `internal/demo` and `internal/resource`. They will not compile until Task 2 and Task 3 create the package. The test file must be in package `unit` (matching the existing test convention in `tests/unit/`).

  Tests to write:
  1. `TestGetResources_EC2` -- returns non-empty slice, every resource has non-nil RawStruct, Fields contains keys matching `resource.GetFieldKeys("ec2")`, and at least one resource has Status "running" and one has Status "stopped".
  2. `TestGetResources_S3` -- returns non-empty slice, every resource has Fields with keys "name" and "creation_date", RawStruct is nil (Fields-only).
  3. `TestGetResources_Lambda` -- returns non-empty slice, every resource has Fields with keys "function_name", "runtime", "memory", "timeout", "handler", "last_modified", "code_size".
  4. `TestGetResources_RDS` -- returns non-empty slice, every resource has Fields with keys "db_identifier", "engine", "engine_version", "status", "class", "endpoint", "multi_az". At least one has Status "available" and one has Status different from "available" (e.g., "creating" or "stopped").
  5. `TestGetResources_Unknown` -- `demo.GetResources("nonexistent")` returns `nil, false`.
  6. `TestGetS3Objects` -- `demo.GetS3Objects("data-pipeline-logs", "")` returns non-empty slice; `demo.GetS3Objects("no-such-bucket", "")` returns `nil, false`.
  7. `TestEC2RawStructYAML` -- for each EC2 demo resource, `yaml.Marshal(r.RawStruct)` succeeds without error and produces non-empty output.
  8. `TestAllDemoResourcesHaveFieldKeys` -- for each resource type with registered field keys ("ec2", "s3", "lambda", "dbi"), call `demo.GetResources(shortName)` and verify every Fields key in every resource is present in `resource.GetFieldKeys(shortName)`. Note: RDS uses shortName "dbi" in the registry, so demo must be keyed accordingly. S3 objects are a special case (no registered field keys) -- validate their keys against `resource.S3ObjectColumns()` instead.
  9. `TestDemoConstants` -- `demo.DemoProfile == "demo"` and `demo.DemoRegion == "us-east-1"`.
  10. `TestEC2NameVariety` -- at least 6 EC2 instances returned, and at least 2 different name prefixes exist (e.g., "web-" and "api-" are both present). This validates the demo-scenario requirement for realistic naming.

- **Acceptance criteria:**
  - File compiles when `internal/demo/` package exists (after Tasks 2+3).
  - All 10 test functions fail with meaningful errors before Task 2+3 implementation.
  - All 10 test functions pass after Task 2+3 implementation.

---

## Task 2: Demo fixtures -- S3, Lambda, RDS (Fields-only)

- **Agent:** a9s-coder
- **Files to create:**
  - `internal/demo/fixtures.go`
- **Dependencies:** None (can be built in parallel with Task 1, but tests verify it)
- **Description:**
  Create the `internal/demo` package with:
  - Constants: `DemoRegion = "us-east-1"`, `DemoProfile = "demo"`.
  - `GetResources(resourceType string) ([]resource.Resource, bool)` -- looks up `demoData[resourceType]` and returns the result. Returns `nil, false` for unknown types.
  - `GetS3Objects(bucket, prefix string) ([]resource.Resource, bool)` -- returns fixture S3 objects for `"data-pipeline-logs"` bucket only; `nil, false` for others.
  - `demoData` map of type `map[string]func() []resource.Resource` keyed by short name.
  - Fixture functions for Fields-only resource types:
    - `s3Buckets()` -- 5-6 buckets with realistic names (e.g., `data-pipeline-logs`, `webapp-assets-prod`, `ml-training-data`).
    - `s3Objects()` -- 4-5 objects in `data-pipeline-logs` (mix of folders and files, with keys like `logs/2026/03/`, `config.json`).
    - `lambdaFunctions()` -- 5-6 functions with varied runtimes (nodejs20.x, python3.12, go1.x, java21, etc.).
    - `rdsInstances()` -- 4-5 instances with mix of statuses (available, creating, stopped) and engines (aurora-postgresql, mysql, postgres).

  Field keys in each fixture MUST match the registered field keys exactly:
  - S3: `name`, `creation_date` (plus `bucket_name`)
  - Lambda: `function_name`, `runtime`, `memory`, `timeout`, `handler`, `last_modified`, `code_size`
  - RDS (shortName "dbi"): `db_identifier`, `engine`, `engine_version`, `status`, `class`, `endpoint`, `multi_az`
  - S3 objects: `key`, `size`, `last_modified`, `storage_class`

  The `demoData` map will initially have entries for "s3", "lambda", and "dbi". The "ec2" entry will be added in Task 3.

  **Important:** RDS is registered as shortName `"dbi"` in the resource registry (with `"rds"` as an alias). The demo data map must be keyed by `"dbi"` to match. The `GetResources` function should also accept `"rds"` as an alias and map it to `"dbi"` internally, OR the `fetchDemoResources` function in app.go (Task 5) will resolve the alias before calling `GetResources`. Prefer the simpler approach: key the map as `"dbi"` and let the caller resolve aliases via `resource.FindResourceType(cmd).ShortName`.

- **Acceptance criteria:**
  - `go build ./internal/demo/` compiles.
  - No AWS SDK imports in this file.
  - Only imports `internal/resource`.
  - Each fixture function returns a fresh slice (no shared global state).
  - All Fields keys match the registered column definitions in `resource/types.go`.

---

## Task 3: Demo fixtures -- EC2 with RawStruct

- **Agent:** a9s-coder
- **Files to create:**
  - `internal/demo/fixtures_ec2.go`
- **Files to modify:**
  - `internal/demo/fixtures.go` (add `"ec2"` entry to the `demoData` map)
- **Dependencies:** Task 2 (fixtures.go must exist for the map modification)
- **Description:**
  Create EC2 demo fixtures with populated `RawStruct` for detail and YAML views.

  - `ec2Instances() []resource.Resource` -- 6-8 instances with:
    - Mix of states: at least 2 "running", 1 "stopped", 1 "pending" (for status color variety in the demo).
    - Realistic demo names: `web-prod-01`, `web-prod-02`, `api-staging-01`, `worker-batch-03`, `bastion-prod`, `db-proxy-01`, etc. At least 2 names must start with `"web"` so the `/web` filter in the demo scenario shows results.
    - Fields matching EC2 column keys: `instance_id`, `name`, `state`, `type`, `private_ip`, `public_ip`, `launch_time`.
    - `RawStruct` populated with `ec2types.Instance` structs containing realistic data (InstanceId, InstanceType, State, PrivateIpAddress, PublicIpAddress, Tags, VpcId, SubnetId, LaunchTime, etc.).

  This file will import `github.com/aws/aws-sdk-go-v2/service/ec2/types` -- this is the single SDK dependency allowed in the demo package.

  After creating the file, add `"ec2": ec2Instances` to the `demoData` map in `fixtures.go`.

- **Acceptance criteria:**
  - `go build ./internal/demo/` compiles.
  - EC2 fixtures have non-nil `RawStruct` of type `ec2types.Instance`.
  - `yaml.Marshal(rawStruct)` produces valid YAML without panics for every EC2 resource.
  - At least 6 instances with at least 3 distinct states.
  - At least 2 instance names matching `"web*"` for filter demo.
  - All Tasks 1 tests pass: `go test ./tests/unit/ -run TestGetResources_EC2 -count=1`
  - All Tasks 1 tests pass: `go test ./tests/unit/ -run TestEC2 -count=1`

---

## Task 4: Wire --demo flag in main.go

- **Agent:** a9s-coder
- **Files to modify:**
  - `cmd/a9s/main.go`
- **Dependencies:** Task 2 (needs `demo.DemoProfile` and `demo.DemoRegion` constants)
- **Description:**
  Add the `--demo` / `-d` flag to the CLI:
  1. Add `flag.BoolVar(&demoMode, "demo", false, "Run with synthetic demo data (no AWS credentials needed)")` and the `-d` shorthand.
  2. Update `flag.Usage` to include `-d, --demo` line.
  3. When `demoMode` is true, default profile to `demo.DemoProfile` and region to `demo.DemoRegion` if not explicitly provided.
  4. Pass `tui.WithDemo(demoMode)` to `tui.New(...)`.
  5. Add import for `"github.com/k2m30/a9s/internal/demo"`.

  The `tui.WithDemo` function does not exist yet (created in Task 5). The code will not compile until Task 5 is done. Order these tasks so they compile together.

  **Practical note:** Since Task 4 and Task 5 must both be done before either compiles, they should be implemented by the same coder in sequence, or Task 5 first (adding the WithDemo stub), then Task 4.

- **Acceptance criteria:**
  - `a9s --help` shows the `-d, --demo` flag.
  - `a9s --demo` launches without AWS credentials and shows the main menu with header `a9s vX.Y.Z  demo:us-east-1`.
  - `a9s -d` also works.
  - If `--demo` and `--profile myprofile` are both set, profile is "myprofile" (not overridden).

---

## Task 5: Wire demo mode in app.go (root model)

- **Agent:** a9s-coder
- **Files to modify:**
  - `internal/tui/app.go`
- **Dependencies:** Task 2 (needs `internal/demo` package to exist)
- **Description:**
  Integrate demo mode into the root model. All changes are in `app.go`:

  **5a.** Add `demoMode bool` field to `Model` struct.

  **5b.** Add functional option pattern:
  ```go
  type Option func(*Model)
  func WithDemo(enabled bool) Option { ... }
  ```
  Change `New(profile, region string)` to `New(profile, region string, opts ...Option)`.

  **5c.** Modify `Init()`: when `m.demoMode` is true, return a `tea.Cmd` that sends `messages.ClientsReadyMsg{}` (nil Clients, nil Err) instead of `messages.InitConnectMsg`.

  **5d.** Modify `fetchResources`: when `m.demoMode` is true, call a new `fetchDemoResources` method instead of the AWS fetcher path. The method:
  - For S3 drill-down (`s3Bucket != ""`): calls `demo.GetS3Objects(s3Bucket, s3Prefix)`.
  - For standard types: uses `resource.FindResourceType(resourceType)` to resolve the canonical short name, then calls `demo.GetResources(canonicalShortName)`.
  - Always returns `messages.ResourcesLoadedMsg` (never `APIErrorMsg`).
  - Wraps the call in a `tea.Cmd` to maintain the async pattern.

  **5e.** Modify `executeCommand`: add demo-mode guards before the `"ctx", "profile"` and `"region"` cases. Return `FlashMsg{Text: "Profile switching disabled in demo mode", IsError: true}` and `FlashMsg{Text: "Region switching disabled in demo mode", IsError: true}` respectively.

  **5f.** Modify `handleReveal`: add demo-mode guard at the top. Return `FlashMsg{Text: "Secret reveal disabled in demo mode", IsError: true}`.

  **5g.** Modify `handleClientsReady`: when `m.demoMode` is true, skip the region resolution block (the `if m.region == ""` block that calls `awsclient.GetDefaultRegion`).

  **5h.** Add import for `"github.com/k2m30/a9s/internal/demo"`.

- **Acceptance criteria:**
  - `go build ./cmd/a9s/` compiles.
  - In demo mode: `Init()` does not attempt AWS connection.
  - In demo mode: navigating to EC2 returns demo fixtures (not an API error).
  - In demo mode: `:ctx` and `:region` produce flash errors.
  - In demo mode: `x` (reveal) on any resource produces a flash error.
  - In demo mode: `Ctrl+R` (refresh) re-returns the same demo data.
  - In non-demo mode: all existing behavior is unchanged (no regression).

---

## Task 6: Integration tests for app.go demo wiring

- **Agent:** a9s-qa
- **Files to create:**
  - `tests/unit/demo_app_test.go`
- **Dependencies:** Task 5 (needs the modified app.go)
- **Description:**
  Write tests that verify the root model's demo-mode behavior:

  1. `TestDemoMode_Init_NoAWSConnection` -- create `tui.New("demo", "us-east-1", tui.WithDemo(true))`, call `Init()`, verify the returned `tea.Cmd` produces a `messages.ClientsReadyMsg` with nil `Clients` and nil `Err`.
  2. `TestDemoMode_FetchResources_EC2` -- create a demo-mode model, send `ClientsReadyMsg{}`, then send `NavigateMsg{Target: TargetResourceList, ResourceType: "ec2"}`. Verify the returned cmd produces `ResourcesLoadedMsg` with non-empty Resources.
  3. `TestDemoMode_FetchResources_Unknown` -- navigate to a non-demo resource type (e.g., "redis"). Verify `ResourcesLoadedMsg` has empty Resources (not an error).
  4. `TestDemoMode_BlockedCommand_Ctx` -- in demo mode, execute `:ctx` command. Verify it returns a `FlashMsg` with "disabled in demo mode".
  5. `TestDemoMode_BlockedCommand_Region` -- in demo mode, execute `:region` command. Verify it returns a `FlashMsg` with "disabled in demo mode".
  6. `TestDemoMode_BlockedReveal` -- in demo mode with a secrets resource list active, press `x`. Verify it returns a `FlashMsg` with "disabled in demo mode".
  7. `TestDemoMode_RefreshReturnsSameData` -- navigate to EC2 in demo mode, receive resources, then send a refresh. Verify the same count of resources is returned.
  8. `TestNonDemoMode_Unchanged` -- create `tui.New("", "")` (no demo), verify `Init()` produces `InitConnectMsg` (not `ClientsReadyMsg`).

  **Note:** These tests may need to work with the `tea.Model` interface. Look at how existing app.go tests (if any) handle this, or write against the concrete `tui.Model` type if it's exported.

- **Acceptance criteria:**
  - All 8 tests pass.
  - No tests rely on AWS credentials or network access.
  - Tests verify message types, not string output.

---

## Task 7: Update demo.tape VHS script

- **Agent:** a9s-coder
- **Files to modify:**
  - `docs/demos/demo.tape`
- **Dependencies:** Tasks 4+5 (working `--demo` flag)
- **Description:**
  Rewrite the VHS tape script to match `demo-scenario.md`. Key changes:
  1. Launch with `a9s --demo` (not bare `a9s`).
  2. Use `Hide` during launch so recording starts after app is visible.
  3. Follow the exact timing script from `demo-scenario.md`:
     - Act 1: Main menu pause, arrow down x3.
     - Act 2: Enter EC2, browse list, filter `/web`, clear filter, sort by name (N), horizontal scroll (Right x2).
     - Act 3: Detail view (d), scroll, YAML view (y), back to menu (Esc x2).
     - Act 4: `:s3` command, browse buckets, Enter to drill into bucket, Esc x2 back.
     - Act 5: `:lambda`, pause, `:rds`, pause, Esc back to menu, final pause for loop.
  4. Output to `assets/demo.gif` and `assets/demo.mp4`.
  5. Set terminal size to 120x35 per scenario spec.

- **Acceptance criteria:**
  - `vhs docs/demos/demo.tape` runs without error (requires VHS installed + the built binary).
  - Output GIF starts and ends on the main menu (loopable).
  - Total duration is approximately 30-35 seconds.

---

## Task 8: Version bump and final verification

- **Agent:** a9s-coder
- **Files to modify:**
  - `cmd/a9s/main.go` (version string)
- **Dependencies:** All previous tasks
- **Description:**
  1. Bump version string in `cmd/a9s/main.go`.
  2. Run full build: `go build -o a9s ./cmd/a9s/`
  3. Run full test suite: `go test ./tests/unit/ -count=1 -timeout 120s`
  4. Run linter: `golangci-lint run ./...`
  5. Verify: `./a9s --demo` launches and header shows updated version.

- **Acceptance criteria:**
  - `go build` succeeds.
  - All unit tests pass (existing + new demo tests).
  - `golangci-lint run ./...` reports zero issues.
  - `./a9s --demo` launches, shows `demo:us-east-1` in header.

---

## Dependency Graph

```
Task 1 (qa: tests) ──────────────────────────────────┐
                                                      │
Task 2 (coder: fixtures.go) ──┬── Task 3 (coder: EC2 fixtures) ──┐
                               │                                   │
                               ├── Task 4 (coder: main.go) ───────┤
                               │                                   │
                               └── Task 5 (coder: app.go) ────────┤
                                                                   │
                                   Task 6 (qa: app tests) ────────┤
                                                                   │
                                   Task 7 (coder: demo.tape) ─────┤
                                                                   │
                                   Task 8 (coder: verify) ────────┘
```

Parallelism opportunities:
- **Task 1** and **Task 2** can start simultaneously (tests written against API contract).
- **Task 3**, **Task 4**, and **Task 5** can begin once Task 2 is done. Task 4 and 5 must compile together.
- **Task 6** begins after Task 5.
- **Task 7** can run after Tasks 4+5 are wired.
- **Task 8** is the final gate.

## Implementation Notes for Agents

### For a9s-coder:
- The `tui.New` signature change (adding `...Option`) is backward-compatible -- existing callers pass zero options.
- The RDS resource type's canonical short name is `"dbi"`, not `"rds"`. The `:rds` command resolves to `"dbi"` via `resource.FindResourceType("rds").ShortName`. The `fetchDemoResources` method must resolve aliases the same way before looking up demo data.
- EC2 `RawStruct` requires importing `github.com/aws/aws-sdk-go-v2/service/ec2/types` only in `fixtures_ec2.go`. Use pointer fields where the SDK type expects them (e.g., `InstanceId *string`, `State *ec2types.InstanceState`).
- The `fetchDemoResources` method must still return a `tea.Cmd` (not inline data), even though there is no I/O. This preserves the async contract.
- Never chain bash commands. Always single, standalone commands.

### For a9s-qa:
- Tests go in package `unit` in `tests/unit/`.
- Import `"github.com/k2m30/a9s/internal/demo"` for fixture tests.
- For app.go integration tests, you may need to exercise the `Update` method by sending messages directly. Look at the existing `tui.Model` type -- `Update()` returns `(tea.Model, tea.Cmd)`, and you can type-assert cmd results by invoking the returned `tea.Cmd`.
- The EC2 YAML marshal test uses `gopkg.in/yaml.v3` (already a dependency).
- For `TestAllDemoResourcesHaveFieldKeys`, note that field key registration happens in `init()` functions in `internal/aws/*.go`. These init functions run when the package is imported. The test file must import `_ "github.com/k2m30/a9s/internal/aws"` (or a specific fetcher file) to trigger registration, OR use the known column keys directly from the type definitions.
