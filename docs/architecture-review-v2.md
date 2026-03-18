# Architecture Review v2: a9s TUI

**Date:** 2026-03-18
**Reviewer:** Claude Opus 4.6 (architect agent)
**Codebase version:** 1.0.12
**Scope:** Full architecture, code quality, extensibility, related resources design

---

## Architecture Score: 6.5 / 10

**Justification:**

The v1.0 rewrite from the god-object to a view-stack architecture is a genuine improvement. The separation of concerns (messages, keys, styles, layout, views) is correct in principle. However, the architecture has several structural problems that will compound as new resource types are added:

1. The viewEntry union-struct pattern forces every new view type to modify 5 switch statements in app.go (view(), frameTitle(), propagateSize(), updateActiveView(), applyFilterToActiveView()).
2. The fetchResources switch is a bottleneck -- adding VPC requires editing this function and adding a new client to ServiceClients.
3. There are 3,593 lines of dead code in 5 old packages that still exist.
4. APIErrorMsg is produced but never consumed -- API failures silently disappear.
5. Five out of seven fetchers lack pagination, meaning users with >100 resources see truncated lists.
6. `handleRefresh` is broken for S3 objects inside a bucket.
7. Three message types are defined but never sent or received.

The foundation is workable, but these issues must be resolved before scaling to 10+ resource types.

---

## 1. Architecture Assessment

### 1.1 Package Structure

The `internal/tui/` hierarchy is sound:

```
tui/
  app.go          -- root model (872 lines)
  keys/keys.go    -- key bindings
  messages/       -- message types
  styles/         -- palette + composed styles
  layout/         -- frame, header rendering
  views/          -- 8 view models + util.go
```

This is viable for 10+ resource types IF the view dispatch is refactored (see 1.2).

### 1.2 The viewEntry Union-Struct Problem

`viewEntry` at `app.go:46-55` is a struct with one pointer per view type:

```go
type viewEntry struct {
    mainMenu     *views.MainMenuModel
    resourceList *views.ResourceListModel
    detail       *views.DetailModel
    yaml         *views.YAMLModel
    reveal       *views.RevealModel
    profile      *views.ProfileModel
    region       *views.RegionModel
    help         *views.HelpModel
}
```

Every operation on viewEntry requires a type-switch across all fields. There are **5 such switches** in app.go:
- `frameTitle()` (line 58-77)
- `view()` (line 81-101)
- `propagateSize()` (line 426-449)
- `updateActiveView()` (line 524-561)
- `applyFilterToActiveView()` (line 587-599)

Adding a new view type (e.g., RelatedResourcesModel) requires modifying all 5 locations. This violates the Open/Closed Principle.

**Recommendation:** Define a `View` interface:

```go
type View interface {
    Update(tea.Msg) (View, tea.Cmd)
    View() string
    SetSize(w, h int)
    FrameTitle() string
}
```

The view stack becomes `[]View`. All 5 switches collapse to single method calls. Filter-capable views implement an optional `Filterable` interface.

### 1.3 Message Contract

**Good:**
- Clear separation between navigation messages (`NavigateMsg`, `PopViewMsg`) and data messages (`ResourcesLoadedMsg`, `ClientsReadyMsg`)
- Flash system with generation counter to prevent stale clears is well designed

**Problems:**

1. **APIErrorMsg is never handled.** It is produced at `app.go:671,701,708` but there is no `case messages.APIErrorMsg:` in `Update()`. When an AWS API call fails, the error message is sent as a `tea.Msg`, but nothing receives it. The resource list stays in "Loading..." forever. This is a **production bug**.

2. **Three dead message types** defined but never used:
   - `CopiedMsg` (messages.go:73) -- clipboard copy uses FlashMsg instead
   - `RefreshMsg` (messages.go:103) -- refresh uses handleRefresh directly
   - `RevealSecretMsg` (messages.go:98) -- reveal uses fetchSecretValue directly

3. **Missing messages for related resources:**
   - `RelatedResourcesRequestMsg` -- user presses `r` on a resource
   - `RelatedResourcesLoadedMsg` -- related resources fetched
   - `TargetRelatedList` view target

### 1.4 View Stack

The stack pattern (`push`/`pop`) is correct for the current navigation model. It will work for related resources: pressing `r` pushes a related-resources list view, which itself can push detail views. The depth is bounded by user navigation, not recursion.

One gap: there is no way to **replace** the top of the stack. This would be useful for "refresh in place" or "switch to related resource of same type." Currently, refresh creates a new fetch but delivers results to the existing view via `ResourcesLoadedMsg`, which works.

### 1.5 Resource Type Registration

Currently hardcoded in three places that must stay in sync:
1. `resource/types.go` -- ResourceTypeDef definitions (7 types)
2. `config/defaults.go` -- defaultViews ViewDef definitions (8 entries, including s3_objects)
3. `app.go:fetchResources` -- switch statement dispatching to AWS fetchers

This is a manual coordination problem. Adding VPC requires editing all three files.

**Recommendation:** A registry pattern where each resource type registers itself:

```go
type ResourceTypeRegistry struct {
    types   []ResourceTypeDef
    fetchers map[string]FetchFunc
    views    map[string]ViewDef
}
```

Each AWS service file registers its type, fetcher, and default view. The switch disappears.

---

## 2. Code Quality Issues

### 2.1 Bugs Found

**BUG-1: APIErrorMsg silently dropped (CRITICAL)**
- File: `/Users/k2m30/projects/a9s/internal/tui/app.go`
- `fetchResources()` returns `APIErrorMsg` on lines 671, 701, 708
- `Update()` has no `case messages.APIErrorMsg:` handler
- Impact: When AWS API calls fail (expired credentials, access denied, throttling), the resource list shows "Loading..." forever with no error indication
- The `ClassifyAWSError` function in `errors.go` exists but is never called anywhere

**BUG-2: S3 object refresh is broken (HIGH)**
- File: `/Users/k2m30/projects/a9s/internal/tui/app.go:809-817`
- `handleRefresh()` calls `m.fetchResources(rt, "", "")` with empty bucket/prefix
- When viewing S3 objects inside a bucket, Ctrl+R fetches the bucket list instead of re-fetching objects
- The `ResourceListModel` has `s3Bucket` but no public accessor for it

**BUG-3: No pagination for 5 out of 7 fetchers (HIGH)**
- Files: `rds.go`, `redis.go`, `docdb.go`, `eks.go`, `secrets.go`
- Only S3 (`s3.go`) implements pagination with continuation tokens
- EC2 DescribeInstances also lacks pagination
- AWS APIs return at most 100 items per page by default
- Users with >100 RDS instances, Redis clusters, EKS clusters, or secrets will see truncated lists

### 2.2 Dead Code

**3,593 lines of dead code across 5 old packages:**

| Package | Files | Lines | Status |
|---------|-------|-------|--------|
| `internal/app/` | 4 files | 2,095 | Old god-object, fully replaced by `internal/tui/` |
| `internal/views/` | 8 files | 865 | Old view models, replaced by `internal/tui/views/` |
| `internal/ui/` | 5 files | 467 | Old UI components, replaced by `internal/tui/layout/` |
| `internal/styles/` | 1 file | 84 | Old styles, replaced by `internal/tui/styles/` |
| `internal/navigation/` | 1 file | 82 | Old nav history, replaced by view stack |

These packages are not imported by any current code but add confusion and bloat.

**Dead functions in live code:**
- `ClassifyAWSError` (`errors.go`) -- defined, tested, never called
- `parseCredentialsProfiles` (`profile.go:98`) -- defined, never called from `ListProfiles`
- `S3GetBucketLocationAPI` interface (`interfaces.go:33`) -- defined, never used
- `CopiedMsg`, `RefreshMsg`, `RevealSecretMsg` message types -- defined, never sent/received

### 2.3 Duplicate Logic

**Filter implementation repeated 3 times:**
- `MainMenuModel.applyFilter()` (mainmenu.go:151-165)
- `ProfileModel.applyFilter()` (profile.go:117-130)
- `RegionModel.applyFilter()` (region.go:117-130)

All three are case-insensitive substring matches on a string slice. Should be a single generic function.

**Viewport initialization pattern repeated 4 times:**
- `DetailModel.SetSize()` (detail.go:87-98)
- `YAMLModel.SetSize()` (yaml.go:76-87)
- `RevealModel.SetSize()` (reveal.go:55-66)
- All use the same `if !m.ready { ... } else { ... }` pattern

**YAML marshaling duplicated:**
- `YAMLModel.RawContent()` (yaml.go:99-114) and `YAMLModel.renderContent()` (yaml.go:122-138) both marshal the resource to YAML with identical logic

### 2.4 Functions That Do Too Much

**`app.go:Update()` (lines 159-341):** 183 lines handling 15 message types. The root Update is unavoidably large in Bubble Tea, but the S3-specific message handling (`S3EnterBucketMsg`, `S3NavigatePrefixMsg`, `LoadResourcesMsg`) could be extracted to helper methods.

**`fetchResources()` (lines 667-718):** This switch is the main extensibility bottleneck. Each new resource type adds a case here.

### 2.5 Inconsistencies

1. **`interface{}` vs `any`:** Go 1.25 supports `any` but the codebase uses `interface{}` throughout (resource.go:19, messages.go:86, fieldpath/extract.go). Not a bug, but modernization improves readability.

2. **Error handling inconsistency:** `fetchResources` returns `APIErrorMsg` (which is dropped), while `fetchSecretValue` and `fetchProfiles` return `FlashMsg` on error (which is handled). The latter pattern actually works; the former does not.

3. **DetailData field on Resource:** `resource.Resource.DetailData` (map[string]string) is populated by all fetchers but never read by the detail view. The detail view uses either `RawStruct` (config-driven path) or `Fields` (fallback path). `DetailData` is dead weight.

4. **RawJSON field on Resource:** `resource.Resource.RawJSON` is populated by all fetchers (via `json.MarshalIndent`) but never used. The YAML view uses `RawStruct` directly. This wastes memory and CPU on every fetch.

---

## 3. Extensibility Assessment: Adding VPC

### Files That Need Changing

| # | File | Change | Complexity |
|---|------|--------|------------|
| 1 | `internal/aws/vpc.go` (NEW) | FetchVPCs function + FetchSubnets, FetchSecurityGroups, etc. | M |
| 2 | `internal/aws/interfaces.go` | Add EC2DescribeVpcsAPI, EC2DescribeSubnetsAPI, etc. | S |
| 3 | `internal/aws/client.go` | No change needed -- EC2 client already exists | -- |
| 4 | `internal/resource/types.go` | Add VPC, Subnet, SecurityGroup ResourceTypeDef entries | S |
| 5 | `internal/config/defaults.go` | Add vpc, subnet, sg ViewDef defaults | S |
| 6 | `internal/tui/app.go` | Add cases to `fetchResources` switch for vpc, subnet, sg | S |
| 7 | `views.yaml` | Add vpc, subnet, sg sections | S |
| 8 | Tests (multiple files) | New fixtures, fetcher tests, QA scenario tests | L |

**Total: 6 existing files + 1 new file + tests = 7 files minimum.**

With a registry pattern, items 4-6 would collapse into the single new file (`vpc.go`).

### Scalability Concern

The main menu currently shows 7 items. Adding VPC, Subnet, Security Group, Route Table, NAT Gateway, Internet Gateway, ELB, Target Group, Lambda, CloudWatch would bring it to 17. The main menu will need categories or grouping (e.g., "Compute", "Network", "Storage", "Security").

---

## 4. Related Resources Design

### 4.1 User Flow

```
EC2 Instance List
  -> select i-abc123
  -> press 'r'
  -> Related Resources View appears:
      ┌── related: i-abc123 ──┐
      │ VPC          vpc-123  │
      │ Subnet       sub-456  │
      │ SG           sg-789   │
      │ SG           sg-012   │
      └───────────────────────┘
  -> select vpc-123
  -> press Enter
  -> VPC detail view
```

### 4.2 Message Types Needed

```go
// TargetRelated is a new ViewTarget
TargetRelated ViewTarget = 9  // after TargetHelp

// NavigateMsg already has ResourceType and Resource fields,
// which are sufficient for the Related view.

// RelatedResourcesLoadedMsg carries the discovered relationships.
type RelatedResourcesLoadedMsg struct {
    SourceID     string
    SourceType   string
    Related      []RelatedItem
}

type RelatedItem struct {
    Type         string            // "vpc", "subnet", "sg"
    ID           string            // "vpc-123"
    Name         string            // optional display name
    Relationship string            // "member-of", "attached-to"
}
```

### 4.3 Relationship Detection Strategy

**Option A: Explicit config (recommended)**

Define relationships in `views.yaml` or a separate `relations.yaml`:

```yaml
relations:
  ec2:
    - type: vpc
      field: VpcId
      relationship: member-of
    - type: subnet
      field: SubnetId
      relationship: member-of
    - type: sg
      field: SecurityGroups[].GroupId
      relationship: attached-to
  rds:
    - type: vpc
      field: DBSubnetGroup.VpcId
      relationship: member-of
    - type: sg
      field: VpcSecurityGroups[].VpcSecurityGroupId
      relationship: attached-to
```

Advantages: explicit, testable, no heuristics. The `fieldpath.ExtractValue` function already handles dot-notation paths, so extracting `VpcId` from a resource's `RawStruct` is straightforward.

**Option B: Scan for *Id patterns (rejected)**

Auto-detect relationships by scanning `RawStruct` for fields ending in `Id`, `Arn`, or `GroupId`. This is brittle -- `ImageId` is not a navigable resource, `RequesterId` is not useful, and the relationship direction is ambiguous.

### 4.4 View Design

**Reuse ResourceListModel with a filtered/static dataset.** The related resources view is essentially a resource list pre-populated with specific items. Create a `NewRelatedResourcesList` constructor that accepts a slice of `RelatedItem` and presents them as a table:

| Type | ID | Name | Relationship |
|------|------|------|------|
| VPC | vpc-123 | prod-vpc | member-of |
| Subnet | sub-456 | private-a | member-of |
| SG | sg-789 | web-sg | attached-to |

Enter on a related item navigates to that resource's detail view (if the resource type is supported) or shows the ID for copying.

### 4.5 Keys Integration

```go
// In keys.go:
Related key.Binding  // bound to "r"

// In keys/keys.go Default():
Related: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "related")),
```

The `r` key is handled in `app.go Update()` at the global level (like `x` for reveal), checking if the active view is a resourceList or detail view.

---

## 5. Test Quality Assessment

### 5.1 Tests That Give False Confidence

1. **All tests with `configForType()` helper** -- This function explicitly works around the ViewDef iteration bug (documented in architecture-audit.md Bug 3). Tests pass with a single-type config but fail with the production config. The bug was fixed in v1.0.x (resourceType field added to DetailModel), but `configForType` should still be removed from tests and replaced with the full config to prevent regression.

2. **QA fixture data without RawStruct** -- List-view QA tests (`qa_ec2_test.go`, `qa_s3_test.go`, etc.) use `resource.Resource{Fields: map[string]string{...}, RawStruct: nil}`. This means config-driven column extraction (which uses `fieldpath.ExtractScalar` on `RawStruct`) is never exercised in list view context.

3. **Sort tests that only check indicators** -- Tests verify sort arrows appear but do not verify actual data order. A sort implementation that shows the arrow but does not reorder data would pass all tests.

### 5.2 Highest-Value Test Improvement

**Add APIErrorMsg handling test.** Currently there are zero tests for what happens when an AWS API call fails after the resource list view is pushed. This is the most impactful gap because:
- It is a production-visible bug (Loading... forever)
- It affects all 7 resource types
- It is trivially testable (send APIErrorMsg to the model, verify flash error appears)

---

## Task List

### P0 -- Must do before adding new resources

- [ ] TASK-001: Handle APIErrorMsg in root Update
  - **Agent:** coder | **Size:** S | **Files:** `internal/tui/app.go` (add `case messages.APIErrorMsg:` that sets flash error and stops loading spinner on active resourceList)
  - Also integrate `ClassifyAWSError` to provide user-friendly error messages

- [ ] TASK-002: Delete 5 old packages (3,593 lines of dead code)
  - **Agent:** coder | **Size:** S | **Files:** Delete `internal/app/`, `internal/views/`, `internal/ui/`, `internal/styles/`, `internal/navigation/`
  - Verify no imports reference them (already confirmed -- none do)

- [ ] TASK-003: Fix S3 object refresh bug
  - **Agent:** coder | **Size:** S | **Files:** `internal/tui/app.go` (handleRefresh), `internal/tui/views/resourcelist.go` (add S3Bucket() accessor)
  - When refreshing inside an S3 bucket, pass the bucket name and current prefix to fetchResources

- [ ] TASK-004: Add pagination to all fetchers
  - **Agent:** coder | **Size:** M | **Files:** `internal/aws/ec2.go`, `rds.go`, `redis.go`, `docdb.go`, `eks.go`, `secrets.go`
  - EC2: use NextToken on DescribeInstances
  - RDS: use Marker on DescribeDBInstances
  - Redis: use Marker on DescribeCacheClusters
  - DocDB: use Marker on DescribeDBClusters
  - EKS: use NextToken on ListClusters
  - Secrets: use NextToken on ListSecrets

- [ ] TASK-005: Extract View interface to eliminate viewEntry switches
  - **Agent:** architect/coder | **Size:** M | **Files:** `internal/tui/app.go`, all files in `internal/tui/views/`
  - Define `View` interface with `Update`, `View`, `SetSize`, `FrameTitle` methods
  - Replace `viewEntry` struct with `View` interface in the stack
  - Add optional `Filterable` interface for views that support SetFilter
  - Eliminates 5 type-switch blocks (approximately 80 lines of boilerplate)

- [ ] TASK-006: Implement resource type registry pattern
  - **Agent:** architect/coder | **Size:** M | **Files:** `internal/resource/types.go` (add registry), `internal/config/defaults.go` (register defaults), `internal/tui/app.go` (replace fetchResources switch with registry lookup), `internal/aws/*.go` (each file registers its fetcher)
  - Adding a new resource type should require only: 1 new file + 1 registration call

### P1 -- Should do for quality

- [ ] TASK-010: Remove dead code in live packages
  - **Agent:** coder | **Size:** S | **Files:** `internal/tui/messages/messages.go` (remove CopiedMsg, RefreshMsg, RevealSecretMsg), `internal/aws/errors.go` (integrate ClassifyAWSError or remove), `internal/aws/interfaces.go` (remove S3GetBucketLocationAPI), `internal/resource/resource.go` (remove DetailData and RawJSON fields)
  - Update all fetchers to stop populating DetailData and RawJSON

- [ ] TASK-011: Deduplicate filter logic across views
  - **Agent:** coder | **Size:** S | **Files:** `internal/tui/views/mainmenu.go`, `profile.go`, `region.go`
  - Extract a generic `filterStringSlice(items []string, query string) []string` function
  - MainMenu filter is slightly different (matches on Name+ShortName) but can share the core

- [ ] TASK-012: Fix test infrastructure
  - **Agent:** qa | **Size:** M | **Files:** `tests/unit/fixtures_test.go`, `tests/unit/qa_detail_test.go`, `tests/unit/helpers_test.go`
  - Remove `configForType` workaround -- all tests should use full production config
  - Add RawStruct to fixture data for list-view QA tests
  - Consolidate `stripANSI` / `stripAnsi` duplication
  - Move `qa_detail_test.go` from `package unit_test` to `package unit`

- [ ] TASK-013: Add APIErrorMsg handling test
  - **Agent:** qa | **Size:** S | **Files:** `tests/unit/tui_root_test.go`
  - Send APIErrorMsg to a model with active resourceList
  - Verify: flash error appears, loading state clears, resource list shows error

- [ ] TASK-014: Add pagination tests for all fetchers
  - **Agent:** qa | **Size:** M | **Files:** `tests/unit/aws_*_test.go`
  - Mock multi-page responses for RDS, Redis, DocDB, EKS, Secrets, EC2
  - Verify all pages are fetched and combined

- [ ] TASK-015: Add sort order verification tests
  - **Agent:** qa | **Size:** S | **Files:** `tests/unit/qa_ec2_test.go` (and others)
  - After sorting, extract data rows and verify actual order, not just arrow indicator

- [ ] TASK-016: Use `any` instead of `interface{}`
  - **Agent:** coder | **Size:** S | **Files:** `internal/resource/resource.go`, `internal/tui/messages/messages.go`, `internal/fieldpath/extract.go`
  - Go 1.25 project should use modern syntax

### P2 -- Nice to have / Future prep

- [ ] TASK-020: Design and implement related resources view
  - **Agent:** architect | **Size:** L | **Files:** New `internal/tui/views/related.go`, `internal/tui/messages/messages.go`, `internal/tui/keys/keys.go`, `internal/tui/app.go`, new `relations.yaml` config
  - Add `r` key binding
  - Define RelatedItem model and RelatedResourcesLoadedMsg
  - Create RelatedResourcesList view (reuse ResourceListModel patterns)
  - Add relationship config for EC2 (VPC, Subnet, SG)

- [ ] TASK-021: Add VPC resource type
  - **Agent:** coder | **Size:** M | **Files:** New `internal/aws/vpc.go`, `internal/aws/interfaces.go`, `internal/resource/types.go`, `internal/config/defaults.go`, `internal/tui/app.go` (or registry), `views.yaml`
  - Fetch VPCs, display in list/detail/YAML views
  - Include CIDR, state, tags

- [ ] TASK-022: Add Subnet resource type
  - **Agent:** coder | **Size:** S | **Files:** Same pattern as TASK-021
  - Fetch Subnets, include VPC ID, AZ, CIDR, available IPs

- [ ] TASK-023: Add Security Group resource type
  - **Agent:** coder | **Size:** S | **Files:** Same pattern as TASK-021
  - Fetch Security Groups, include VPC ID, rule count, description

- [ ] TASK-024: Add main menu categories/grouping
  - **Agent:** coder | **Size:** M | **Files:** `internal/resource/types.go` (add Category field), `internal/tui/views/mainmenu.go` (render groups)
  - Group resource types: Compute (EC2, EKS), Storage (S3), Database (RDS, Redis, DocDB), Network (VPC, Subnet, SG, Route Table, NAT GW), Security (Secrets, IAM)

- [ ] TASK-025: Add Route Table, NAT Gateway, Internet Gateway resource types
  - **Agent:** coder | **Size:** M | **Files:** New `internal/aws/network.go` (or per-type files)
  - All use EC2 client, similar patterns

- [ ] TASK-026: Performance -- lazy client initialization
  - **Agent:** coder | **Size:** S | **Files:** `internal/aws/client.go`, `internal/tui/app.go`
  - Currently `CreateServiceClients` creates all 7 service clients upfront. With 15+ resource types, this wastes memory. Create clients on first use.

- [ ] TASK-027: Add golden file snapshot tests for View() output
  - **Agent:** qa | **Size:** M | **Files:** New test files
  - Capture rendered output for each view type with known data
  - Compare against stored snapshots to catch visual regressions (column alignment, border characters)

---

## Summary of Critical Findings

| # | Finding | Severity | Impact |
|---|---------|----------|--------|
| 1 | APIErrorMsg silently dropped | CRITICAL | AWS errors invisible to user, Loading... forever |
| 2 | No pagination in 6/7 fetchers | HIGH | Truncated resource lists for large accounts |
| 3 | S3 object refresh broken | HIGH | Ctrl+R in bucket shows bucket list instead |
| 4 | 3,593 lines dead code | MEDIUM | Confusion, slower navigation, false security |
| 5 | viewEntry switches scale O(n) with view types | MEDIUM | Every new view requires 5 edits to app.go |
| 6 | fetchResources switch is extensibility bottleneck | MEDIUM | Every new resource type requires app.go edit |
| 7 | DetailData + RawJSON fields unused | LOW | Wasted memory/CPU on every fetch |
| 8 | 3 dead message types | LOW | Misleading API surface |

The P0 tasks (TASK-001 through TASK-006) should be completed before adding any new resource types. They address production bugs and architectural bottlenecks that would compound with each new type added.
