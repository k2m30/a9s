# a9s Architecture Guide

This document is the first thing you should read when joining the project. It covers the current runtime architecture, the intended boundaries between layers, and the design rules that new code should follow.

## What is a9s?

a9s is a read-only terminal UI for AWS. Think k9s for Kubernetes, but for AWS services. It uses [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) (the Elm Architecture for Go) and renders with [Lipgloss v2](https://github.com/charmbracelet/lipgloss).

**Read-only by design** — a9s never makes write calls to AWS. Every AWS API call is a List, Describe, or Get operation.

---

## Architectural Direction

The current codebase is already organized around a useful separation of concerns. When adding new code, prefer changes that reinforce these boundaries rather than blur them.

### Desired Layer Boundaries

- **`cmd/a9s`** — bootstrap only: parse flags, validate startup inputs, load config/theme, wire clients and options, start Bubble Tea.
- **`internal/tui`** — UI shell and orchestration: view stack, global key handling, message routing, sizing, transient UI state, and session-scoped cache ownership.
- **`internal/resource`** — declarative registry: resource types, child-view metadata, related defs, navigable fields, and fetcher/enricher registration.
- **`internal/aws`** — adapter layer: call AWS SDK APIs and transform responses into `resource.Resource`. This layer should not know about Bubble Tea views.
- **`internal/cache`** — persistence only: on-disk availability cache and TTL rules.
- **`internal/demo`** — injected fake transport for development and tests, not a parallel feature architecture.

### Architectural Invariants

- Views never call AWS directly.
- Views communicate with the rest of the app by emitting typed messages.
- The root `tui.Model` owns navigation, clients, session caches, and async result routing.
- Every async result must carry enough context to reject stale or wrong-target updates.
- Cache invalidation rules must be explicit for refresh, profile switch, region switch, and view-open flows.
- `Esc` is the back/dismiss key. `q` is the quit key in normal mode; it is not the navigation primitive.

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

Views communicate exclusively via typed messages (`internal/tui/messages/messages.go`). Views never import each other. The root `Model.Update()` routes messages to the appropriate handler.

Key messages:

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
| `RelatedCheckResultMsg` | Deliver one related-check result to detail view |
| **Availability & Issue Counts** | |
| `AvailabilityCacheLoadedMsg` | Deliver disk-cached availability + issue count data (includes `IssueCounts`, `IssueKnown` maps) |
| `AvailabilityPrefetchedMsg` | No-cache-mode availability + issue counts + retained resources for Wave 2 |
| `AvailabilityCheckedMsg` | One resource type's background probe result (includes `Issues` count + retained `Resources`) |
| `EnrichmentCheckedMsg` | One resource type's Wave 2 enrichment result (issue count + truncated flag) |
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

Types are built once at package init by `buildResourceTypes()`. Categories map to type definition files:

| File | Category |
|------|----------|
| `types_compute.go` | Compute (EC2, Lambda, EKS, ASG, Elastic Beanstalk, EBS) |
| `types_containers.go` | Containers (ECS Clusters, ECS Services) |
| `types_networking.go` | Networking (VPC, Subnet, SG, ELB, TG, IGW, NAT, Route Tables, Node Groups, ACM, API Gateway, WAF) |
| `types_databases.go` | Databases & Storage (RDS, S3, Redis, OpenSearch, DynamoDB, Redshift, MSK, EFS, Kinesis) |
| `types_security.go` | Security & IAM (IAM Roles, Users, Policies, Groups, KMS) |
| `types_secrets.go` | Secrets & Config (Secrets Manager, SSM Parameter Store) |
| `types_monitoring.go` | Monitoring (CloudWatch Alarms, Log Groups, CloudTrail) |
| `types_messaging.go` | Messaging (SNS, SQS, EventBridge Rules, Step Functions, SES) |
| `types_cicd.go` | CI/CD (CloudFormation, CodePipeline, CodeBuild, ECR) |
| `types_dns_cdn.go` | DNS & CDN (Route 53, CloudFront) |
| `types_data.go` | Data & Analytics (Athena, Glue) |
| `types_backup.go` | Backup (AWS Backup) |

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
| **EnricherFunc** | `func(ctx, *ServiceClients, []Resource) (int, bool, error)` | Wave 2 issue enrichment (returns issue count + truncated flag) |

Each fetcher takes `clients any` and type-asserts to `*aws.ServiceClients` internally. This allows tests to inject mocks.

### Wave 2 Issue Enrichment Pipeline

Some resource types hide problems behind extra API calls (e.g., EC2 with impaired status checks, RDS with pending maintenance). Wave 2 enrichment discovers these hidden issues after Wave 1 probes complete.

**Architecture:**
- `internal/aws/enrichment.go` — 9 enricher functions implementing `EnricherFunc`, registered in `EnricherRegistry`
- `internal/tui/app_fetchers.go` — `buildEnrichQueue()`, `probeEnrichment()`
- `internal/tui/app_handlers_navigate.go` — `startEnrichment()`, `handleEnrichmentChecked()` with only-increase guard

**Flow:**
```text
Wave 1 probes complete
  → startEnrichment() builds queue from EnricherRegistry ∩ probeResources
  → probeEnrichment() dispatches enrichers (4-at-a-time, same as Wave 1)
  → EnrichmentCheckedMsg arrives
    → only-increase guard: menu badge updated only if new count > current
    → progress indicator updated
  → all done: clear probeResources, save cache with enriched counts (when caching enabled)
```

**Priority order** (batchable first): RDS/DocDB maintenance → EC2 status checks → EBS volume status → CodeBuild → Target Groups → CodePipeline → DynamoDB → Step Functions → Glue.

**Resource identity**: Enrichers receive retained first-page resources from Wave 1 probes (`probeResources map[string][]resource.Resource`). Batchable enrichers (priorities 1-3) make account-wide calls. Per-resource enrichers (priorities 5-9) iterate over resource IDs/ARNs, capped at `EnrichmentCap` (50).

**Skip condition**: Wave 2 runs only when `isDemo=false`. Demo mode has no real AWS to query. `--no-cache` on live AWS still runs Wave 2 (it only disables disk persistence, not capabilities).

**Lifecycle**: `probeResources` is cleared after Wave 2 completes and on profile/region switch. Manual refresh (Ctrl+R) does not currently clear Wave 2 state — it only bumps `availabilityGen` and reloads the cache.

---

## View Types

All in `internal/tui/views/`:

| View | File(s) | Purpose |
|------|---------|---------|
| **MainMenuModel** | `mainmenu.go` | Category-grouped resource list with availability badges, issue count badges (`issues:N`), ctrl+z quad-state filter, enrichment progress indicator |
| **ResourceListModel** | `resourcelist.go` | Paginated table with filter, sort, child drill-down; embeds `AttentionFilter` for ctrl+z; tracks `issueCount` for title badge |
| **DetailModel** | `detail.go`, `detail_fields.go`, `detail_helpers.go` | Two-column: field list (left) + related panel (right) |
| **YAMLModel** | `yaml.go` | YAML dump of RawStruct with syntax highlighting + search. Also doubles as a raw-text viewer via `NewTextViewer()` (used for the `!` error log) |
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

### Issue Counting & Attention Filter

The main menu shows `issues:N` badges per resource type, counting resources in warning/error states. The ctrl+z key filters the menu to show only types with issues.

**Two predicates** serve different purposes:
- `IsDimRowColor(status)` — identifies dim/dead rows (terminated, deleted). Used by ctrl+z on resource lists to hide dead rows. Returns true for `ColTerminated` and `ColHeaderFg` colors.
- `IsIssueRowColor(status)` — identifies warning/error rows (stopped, failed, pending, alarm). Used for issue count badges. Color-independent: uses a pre-built `issueStatusSet` map, works under `NO_COLOR`.

**AttentionFilter** (`internal/tui/views/attention.go`): A shared toggle struct embedded by both `MainMenuModel` and `ResourceListModel`. Owns only enabled/disabled state — views do their own counting and rendering.

**Issue counting flow**:
1. Wave 1 probes (or `demoPrefetchCounts()`) count `IsIssueRowColor()` rows from first page
2. Counts flow to `MainMenuModel` via `SetIssues()`, rendered as `issues:N` badges
3. Wave 2 enrichment discovers hidden issues, updates badges (only-increase guard)
4. `popView()` sync-back: when user returns from a list, the list's Status-based `issueCount` syncs back to the menu — but only if higher than the current menu count (prevents overwriting enriched counts)

**Tri-state visibility** under ctrl+z on the main menu:

| State | Badge | Visible under ctrl+z? |
|-------|-------|----------------------|
| Unknown (not probed) | None | Yes (prevent cold-start empty menu) |
| Zero issues (any truncation) | None | No — config-only types (S3, ENI, IAM) never have issues regardless of page count |
| Nonzero issues | `issues:N` | Yes |

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
- `Ctrl+Z` — toggle attention filter: on resource lists, hides dim/routine rows (`!IsDimRowColor`); on main menu, filters to types with issues using quad-state visibility (unknown→visible, confirmed-zero→hidden, truncated-zero→visible, nonzero→visible)
- `c` — copy resource ID to clipboard
- `p` — role policies (and other child views via `ChildViewDef.Key`)
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

The root model's `handleKeyMsg` (`app_handlers.go`) resolves keys in a specific order. Input modes take priority over global keys, which means `q`, `?`, etc. are swallowed during typing:

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

Resource lists support column-position sorting via keys `1`–`9` and `0` (tenth column). Implementation lives in `internal/tui/views/sort.go`:

- `sortColIdx int` tracks the active sort column index
- `SortByCol [10]key.Binding` in `keys.Map` maps digit keys to column indices
- Pressing a sort key sorts ascending; pressing the same key again toggles to descending
- Sort indicator (▲/▼) appears in the column header

The old named sort sentinels (`SortName`, `SortID`, `SortAge`) are deprecated and map to column indices 0, 1, 2 respectively. New code should use column-position sorting exclusively.

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

## Enrichment

a9s has two distinct enrichment pipelines:

1. **Detail enrichment** (on-demand) — fetches additional data when a user opens a detail/YAML/JSON view (e.g., IAM policy documents). See below.
2. **Issue enrichment (Wave 2)** (background) — discovers hidden issues via additional API calls after Wave 1 probes complete. See "Wave 2 Issue Enrichment Pipeline" under Fetcher Patterns.

### On-Demand Detail Enhancement

When a detail, YAML, or JSON view opens for a resource type with a registered enricher, the app async-fetches additional data and merges it into the resource.

**Flow:**

```text
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

**Caching policy**:
- **Default**: no cache. Re-fetch on each enrichable view open when the data is cheap enough or may change during a session.
- **If caching is justified**: use a session-scoped, feature-specific cache owned by session context (`ServiceClients` today). This is appropriate when the enrichment is relatively expensive and the data is unlikely to change within a session.
- **Never**: use package-global cache state for enrichers.

**Current example**: IAM policy document enrichment uses the session-scoped `PolicyDocCache` on `ServiceClients`. Cache keys are explicitly namespaced: `managed:<policyArn>` for managed policies, `inline:<roleName>/<policyName>` for inline. When `ServiceClients` is replaced on profile/region switch, the old cache is garbage collected — no explicit invalidation hooks needed.

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
| **Enricher caches** | Feature-specific cache on `ServiceClients` (current example: `PolicyDocCache`) | In-memory, session-scoped | Automatically GC'd when ServiceClients is replaced on profile/region switch |

**Disk availability cache** (`internal/cache/cache.go`): Tracks which resource types have resources, their counts, and issue counts. Loaded on startup to instantly grey-out empty types and show issue badges in the main menu. Structure: `File{Profile, Region, CheckedAt, Resources map[string]Entry}` where `Entry{HasResources, Count, Truncated, Issues, IssuesTruncated, IssuesKnown}`. The `IssuesKnown` bool distinguishes "probed and found zero issues" from "not yet probed" (both unmarshal as int 0 without this flag). When caching is enabled (not `--no-cache`), the cache is saved after Wave 1 probes complete and again after Wave 2 enrichment completes, so enriched issue counts persist across restarts. When `--no-cache` is active, `saveAvailabilityCache()` is a no-op.

**Resource cache**: Stores the full view state of previously-viewed resource lists (resources, pagination, filter, sort, cursor position). Enables instant back-navigation without re-fetching.

**Related cache**: LRU mapping `"resourceType:resourceID"` → related check results. Avoids re-running related checks when re-entering a detail view for the same resource.

**Enricher caches**: Caching is optional, not automatic. The default is no cache. When an enricher does cache, it should use a session-scoped, feature-specific cache owned by `ServiceClients`, so cache lifetime matches session lifetime. The current example is the policy document enricher, which caches decoded documents by `managed:<policyArn>` or `inline:<roleName>/<policyName>`.

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
- `internal/demo/transport.go` — fake HTTP transport for STS (the only service without a typed fake interface)
- `demo.NewServiceClients()` wires fakes into a `*aws.ServiceClients` struct

**API note**: `tui.WithDemo(true)` is a compatibility shim equivalent to `WithClients(demo.NewServiceClients())` + `WithNoCache(true)` + `WithIsDemo(true)` (it directly sets the same fields rather than calling those options). New code should use the explicit form. The `isDemo` flag controls whether Wave 2 enrichment runs — demo mode skips it (no real AWS to query), while `--no-cache` on live AWS preserves full functionality.

Demo mode is the primary way to develop and test the TUI without AWS access.

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
- `WithNoCache(true)` — disable disk cache persistence (availability probes still run via `demoPrefetchCounts()`)
- `WithIsDemo(true)` — mark session as demo mode (skips Wave 2 enrichment; set by `--demo` CLI bootstrap)
- `WithCommand("ec2")` — open directly to a resource type on startup
- `WithDemo(true)` — compatibility shim: equivalent to `WithClients` + `WithNoCache` + `WithIsDemo` (see Demo Mode above)

---

## Extension Guide

### Adding a New Resource Type

1. Add or update the `ResourceTypeDef` and built-in default view config.
2. Implement the fetcher in `internal/aws/` so it returns stable `resource.Resource` values with meaningful `ID`, `Name`, `Status`, `Fields`, and `RawStruct`.
3. Register the resource behavior in `internal/resource/`:
   - paginated fetcher
   - child fetchers, if any
   - related defs, if any
   - navigable fields, if any
   - reveal fetcher or enricher, if needed
4. Add demo fixtures and fakes so the feature can be exercised without AWS access.
5. Add both fetcher-level tests and TUI-level behavior tests.

### Adding a Child View

1. Define a `ChildViewDef` on the parent resource type.
2. Implement and register the `PaginatedChildFetcher`.
3. Ensure the parent resource exposes the `ContextKeys` required by the child fetcher.
4. Add demo data and a navigation test that drives the real root model.

### Adding an Enricher

1. Keep the base list/detail fetch path fast; defer expensive or optional data to the enricher.
2. Make the enricher idempotent and safe to re-run.
3. Ensure stale results are rejectable by generation plus resource context.
4. If the enricher caches, document its scope and invalidation rules.
5. Add tests for success, error, stale-result rejection, and cache invalidation paths.

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
6. **If an enricher caches, test session scoping** — different `ServiceClients` instances must get independent caches automatically. If an enricher does not cache, no cache cleanup should be required.

### Integration Tests

Gated by `//go:build integration`. Two modes:
- **Demo integration** — uses `demo.NewServiceClients()`, no real AWS. Tests full boot sequence including `Init()` and message propagation.
- **Live AWS integration** — requires `A9S_CT_PROFILE=<profile>`. Tests real API calls against a live AWS account.

Run: `A9S_CT_PROFILE=<profile> go test -tags integration ./tests/integration/ -run TestName -count=1 -v -timeout 600s`

---

## Known Compromises

- `WithDemo(true)` still exists as a compatibility shim for older tests. New code should prefer explicit client injection plus `WithNoCache(true)` plus `WithIsDemo(true)`. Migration and removal of `WithDemo(true)` is tracked in #270.
- The root `tui.Model` intentionally does double duty as both UI shell and orchestration layer. That keeps Bubble Tea integration simple, but it also means some operational concerns still live in `internal/tui`.
- When enrichers cache today, they do so via feature-specific fields on `ServiceClients`. This is correct for session lifetime management, but it does mean feature-specific state lives on a general-purpose struct.
- Key handling is centralized and order-sensitive. This is pragmatic, but behavioral changes to global keys should always be reviewed against input-mode and search-mode semantics.

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

Async operations (related checks, enrichment) can outlive the view that triggered them. Generation counters (`relatedGen`, `enrichGen`, `availabilityGen`, `enrichmentGen`) are incremented on context changes, causing stale in-flight results to be silently discarded. `enrichmentGen` specifically guards Wave 2 enrichment — bumped on profile/region switch alongside `availabilityGen` to cancel in-flight Wave 2 probes. Note: manual refresh (Ctrl+R) currently only bumps `availabilityGen`, not `enrichmentGen`.

### Why a separate enricher pattern?

Detail views render from pre-fetched `Fields`/`RawStruct`. Some data (like policy documents) requires additional API calls that are too expensive to make at list-fetch time. The enricher pattern fetches this data on demand when the user actually opens the detail, YAML, or JSON view.

### Why four separate caches?

Each cache serves a fundamentally different access pattern: disk cache survives restarts for instant startup; resource cache enables instant back-navigation; related cache avoids redundant API fanouts; enricher caches prevent repeated expensive single-resource fetches. Collapsing them would conflate TTL/invalidation/eviction policies.
