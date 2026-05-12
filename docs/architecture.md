# a9s Architecture Guide

> **CURRENT-STATE ARCHITECTURE.** This document describes how `main` is
> built today. It is normative for code that still lives in the current
> architecture, but it is not the target-state design document for the
> refactor program under `docs/refactor/`.

This document is the first thing you should read when joining the project. It explains the runtime architecture that exists on `main` today, the constraints that current code must still honor, and the major implementation seams that the refactor plan is intentionally replacing.

For the target "no legacy / no lazy compromise" architecture and the migration plan, read:

- [`docs/refactor/00-overview.md`](refactor/00-overview.md) — program-level goals and invariants
- [`docs/refactor/01-projection-hook.md`](refactor/01-projection-hook.md) through [`docs/refactor/05-boundary.md`](refactor/05-boundary.md) — phase-by-phase target architecture

Latest target-architecture additions in the refactor docs:

- cross-cutting capabilities (logs, investigation, cost, future actions) stay separate from the resource catalog
- shared query-contract types live in `internal/domain`
- shared selector/matcher logic lives in `internal/semantics/selector`
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
- On `main` today, behavior is wired through registries and `runtime.Core` (Phase-05 refactor, AS-237). Session state is exclusively owned by `core.Session()` — `tui.Model` never accesses session fields directly.
- In the target refactor architecture, those patterns are replaced by an explicit catalog, shared selectors and query contracts, capability modules, and runtime-owned screen/task contracts.

## What is a9s?

a9s is a read-only terminal UI for AWS. Think k9s for Kubernetes, but for AWS services. It uses [Bubble Tea v2](https://github.com/charmbracelet/bubbletea) (the Elm Architecture for Go) and renders with [Lipgloss v2](https://github.com/charmbracelet/lipgloss).

**Read-only by design** — a9s never makes write calls to AWS. Every AWS API call is a List, Describe, or Get operation.

---

## Architectural Direction

The current codebase has a real separation of concerns, but several of those boundaries are still legacy-shaped. This section documents the boundaries that exist on `main` so contributors can reason about the current implementation and avoid accidental cross-layer coupling while the refactor is in flight.

### Current Layer Boundaries On `main`

- **`cmd/a9s`** — bootstrap only: parse flags, validate startup inputs, load config/theme, wire clients and options, start Bubble Tea.
- **`internal/tui`** — UI shell and adapter: view stack, global key handling, message routing, sizing, and transient UI state. Holds a `*runtime.Core` (Phase-05, AS-237) and reaches session-scoped state via `m.core.Session()`; it does **not** own the session caches itself.
- **`internal/runtime`** — platform-agnostic app core: `runtime.Core` owns the active `*session.Session` and the catalog snapshot, dispatches inbound events to handlers, and returns `UIIntent` / `TaskRequest` lists for adapters to apply. The handler moves out of `internal/tui` are landing incrementally under PR-05a..h.
- **`internal/session`** — session-scoped state container: `session.Session` owns per-profile/region orchestration state — `ResourceCache`, `LazyResourceCache`, `RelatedCache`, the enrichment queues and per-type maps, and every generation counter. `Session.Rotate()` is the single point that invalidates all of it on profile/region switch.
- **`internal/resource`** — declarative registry: resource types, child-view metadata, related defs, navigable fields, and fetcher/enricher registration.
- **`internal/aws`** — primarily the adapter layer: call AWS SDK APIs, transform responses into `resource.Resource`, and host a few non-UI helper subsystems that have not yet been split out. This layer should not know about Bubble Tea views.
- **`internal/cache`** — persistence only: on-disk availability cache and TTL rules.
- **`internal/demo`** — injected fake transport for development and tests, not a parallel feature architecture.

### Architectural Invariants

The invariants below are normative. Code that violates them is wrong even
if it "works" — the gen guards, registry completeness checks, and related
validators in `tests/unit/architecture_conformance_test.go` fail loudly when
any of these drift.

These are **current-state invariants**, not promises about the final architecture. In particular, the refactor plan under `docs/refactor/` intentionally replaces the old embedded-`sessionRuntime` model (removed in Phase-05, AS-237), package-`init()` registry wiring, the `Status`-centered resource model, and markdown-as-input contracts, and adds explicit capability modules, selector/query contracts, and runtime-owned screen/task boundaries.

1. **One root application model owns session state and orchestration.**
   `tui.Model` owns the UI shell; all session state lives in `runtime.Core`
   (Phase-05, AS-237) and is accessed via `m.core.Session()`. The boundary is
   explicit: no orchestration or session state leaks into view structs.
2. **Views render state and emit typed messages.** Views never call AWS
   directly. `m.clients` is passed to tea.Cmds created by the root model,
   not consumed inside `View()`.
3. **Registries are the declarative source of truth.** Supported resource
   types, related defs, navigable fields, fetchers, detail enrichers, and
   Wave 2 issue enrichers are all registered at package init. There is no
   hand-maintained allowlist in dispatch code — background systems iterate
   registry state and sort by declarative priority metadata.
4. **`internal/aws` stays non-UI.** It primarily translates SDK types into
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
   detail enrichers via `*awsclient.DetailEnrichmentCtx`; a few legacy
   package-global caches still exist in `internal/aws` and are treated as
   debt, not the desired pattern.
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

Views communicate exclusively via typed messages (`internal/runtime/messages/`, split into `cmd.go` for UI→core commands and `event.go` for core→UI events). Views never import each other. The root `Model.Update()` routes messages to the appropriate handler.

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
| `RelatedCheckResultMsg` | Deliver one related-check result to detail view — carries `Result.Err` (checker failure), `LazyAddError` (FetchByIDs failure), `LazyAddedResources` (out-of-scope targets resolved via `FetchByIDs`), `CachedPages` (cold-miss prefetch); app handler routes errors to `FlashMsg{IsError:true}` so the `!` error log captures them |
| **Availability & Issue Counts** | |
| `AvailabilityCacheLoadedMsg` | Deliver disk-cached availability + issue count data (includes `IssueCounts`, `IssueKnown` maps) |
| `AvailabilityPrefetchedMsg` | No-cache-mode availability + issue counts + retained resources for Wave 2; `PrefetchErr` carries per-type fetch failures aggregated across all registered paginated fetchers, surfaced via FlashMsg |
| `AvailabilityCheckedMsg` | One resource type's background probe result (includes `Issues` count + retained `Resources`) |
| `EnrichmentCheckedMsg` | One resource type's Wave 2 enrichment result (issue count + truncated flag + per-resource `Findings` map; dual-generation guard via `Gen` + `TypeGen`) |
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
  catalog/       # resource type definitions and color helpers (Phase-05, AS-237)
  config/        # YAML config loading, built-in defaults per service
  demo/          # synthetic fixture data for --demo mode
    fixtures/    #   per-service Go structs (ec2.go, iam.go, etc.)
    fakes/       #   per-service fake API implementations
  domain/        # shared query-contract types: Resource, Color, Finding, Gen (Phase-05)
  fieldpath/     # struct field extraction via reflection (frozen — don't modify)
  resource/      # generic resource model, type registry, fetcher registry
  runtime/       # platform-agnostic app core: Core, orchestrator, handlers (Phase-05)
    messages/    #   typed Cmd/Event message taxonomy (cmd.go, event.go)
  session/       # session.Session — all session-scoped mutable state; Rotate() invalidates in-flight gens
  tui/           # root Bubble Tea app model (tui.Model wraps runtime.Core)
    keys/        #   key bindings (single Map struct, one file)
    layout/      #   frame rendering (borders, title, status line)
    styles/      #   Tokyo Night Dark palette, theming system
    text/        #   text utilities (PadOrTrunc for column rendering)
    views/       #   all view models (see View Types below)

tests/
  unit/          # all unit tests (run via `make test` with -race)
  integration/   # gated by //go:build integration
  testdata/      # hand-crafted JSON fixtures — AWS SDK response bodies for fetcher unit tests (no live AWS)
```

---

## Resource Model

This section describes the current resource model on `main`. It is intentionally conservative: it explains the struct the codebase uses today, not the canonical finding model planned in [`docs/refactor/03-finding-model.md`](refactor/03-finding-model.md).

```go
// internal/resource/resource.go
type Resource struct {
    ID        string            // primary identifier
    Name      string            // display name
    Status    string            // lifecycle state (used for row coloring)
    Issues    []string          // active Wave 1 issue phrases in precedence order
    Fields    map[string]string // pre-extracted string values for table columns
    RawStruct any               // original AWS SDK typed struct
}
```

- **Issues** — the full ordered set of active Wave 1 phrases for the row. Detail view uses this to render the leading Attention section instead of relying only on the top phrase in `Status`.
- **Fields** — flat key-value pairs populated by each fetcher. Used for list table columns and simple detail rendering. Keys are snake_case (e.g., `"instance_id"`, `"vpc_id"`).
- **RawStruct** — the actual AWS SDK struct (e.g., `ec2types.Instance`, `s3types.Bucket`). Used by detail/YAML/JSON views via reflection for deep field path traversal (e.g., `"State.Name"`, `"Placement.AvailabilityZone"`).
- **Current limitation** — `Status` and `Issues` split one concept across two fields on `main`: `Status` carries the top phrase for row display, while `Issues` carries the full ordered set. That coupling is documented here because it exists today, not because it is the intended end state. The refactor plan moves issue semantics into canonical findings and lifecycle display into typed field selection.

### Resource Type Registration

This is the current registration model on `main`. It is being replaced by the explicit catalog in [`docs/refactor/04-catalog.md`](refactor/04-catalog.md).

Each AWS service registers itself in `init()` via `internal/resource/`:

```go
// internal/resource/registry.go
resource.RegisterPaginated("ec2", fetchEC2Instances)
resource.RegisterPaginatedChild("role_policies", fetchRolePolicies)
resource.RegisterRelated("ec2", []RelatedDef{...})
resource.RegisterNavigableFields("ec2", []NavigableField{...})
resource.RegisterDetailEnricher("role_policies", enrichRolePolicy)
resource.RegisterFieldKeys("ec2", []string{"instance_id", "state", "type", ...})
resource.RegisterFieldAliases("ec2", map[string]string{"id": "instance_id", "az": "availability_zone"})
```

**Field registry** (`RegisterFieldKeys`, `RegisterFieldAliases`): Each fetcher declares the `Fields` map keys it emits and any user-facing aliases for them. `RegisterFieldKeys` lists the canonical keys (used by detail-view field ordering, YAML rendering, and column validation). `RegisterFieldAliases` maps alternative names to canonical keys (used by filter/search input and column config). Builtins are recorded on first registration; later calls (e.g., from tests) override without mutating the builtin snapshot, which keeps test registrations from leaking across suites.

Package-`init()` registration is a property of the current implementation, not a pattern new architecture work should preserve indefinitely. While `main` still uses it, current changes must keep registry state coherent. The target direction is explicit catalog wiring with no hidden registration side effects.

### Resource Type Definitions

```go
// internal/resource/types.go
type ResourceTypeDef struct {
    Name        string           // "EC2 Instances" — display name
    ShortName   string           // "ec2" — colon-command alias and registry key
    ListTitle   string           // overrides ShortName in list frame titles; empty = use ShortName
    Aliases     []string         // alternative command names
    Category    string           // main-menu group (e.g., "COMPUTE")
    Columns     []Column         // table columns for list view
    Children    []ChildViewDef   // child views triggerable from the list (key bindings)
    CopyField   string           // overrides which Fields key `c` copies; empty = copy ID
    StubCreator func(id string) Resource // builds a minimal stub for auto-navigate when cache is empty
    RelatedContextFromIDs func(relatedIDs []string) map[string]string // extracts parent context for related-panel navigation
    CloudTrailKey string         // "LookupAttr:ValueSource" for CloudTrail pivot; empty = no `t` key
    IdentityKey string           // column key for enrichment row-marker placement; empty = use 5-step cascade
    Color func(Resource) Color   // REQUIRED: classifies row health; reads structural fields directly
    ExcludeFromIssueBadge bool   // rows still colored + ctrl+z visible, but excluded from menu badge (used by ct-events)
    CellDecorators map[string]func(r Resource, value string) string // transforms cell values per column before render
}
```

On `main`, `Color func(Resource) Color` is part of the type definition and drives row classification directly. The refactor plan intentionally demotes color to presentation and moves severity into domain data; this section is documenting the current shape, not defending it as the final model.

Types are built once at package init by `buildResourceTypes()`. Categories map to type definition files:

| File | Category |
|------|----------|
| `types_compute.go` | Compute (EC2, Lambda, EKS Clusters, ASG, Elastic Beanstalk, EBS) |
| `types_containers.go` | Containers (EKS Node Groups, ECS Clusters, ECS Services) |
| `types_networking.go` | Networking (VPC, Subnet, SG, ELB, TG, IGW, NAT, Route Tables, ACM, API Gateway, WAF) |
| `types_databases.go` | Databases & Storage (RDS, S3, Redis, OpenSearch, DynamoDB, Redshift, MSK, EFS, Kinesis) |
| `types_security.go` | Security & IAM (IAM Roles, Users, Policies, Groups, KMS) |
| `types_secrets.go` | Secrets & Config (Secrets Manager, SSM Parameter Store) |
| `types_monitoring.go` | Monitoring (CloudWatch Alarms, Log Groups, CloudTrail) |
| `types_messaging.go` | Messaging (SNS, SQS, EventBridge Rules, Step Functions) |
| `types_cicd.go` | CI/CD (CloudFormation, CodePipeline, CodeBuild, ECR) |
| `types_dns_cdn.go` | DNS & CDN (Route 53, CloudFront) |
| `types_data.go` | Data & Analytics (Athena, Glue) |
| `types_backup.go` | Backup (AWS Backup, SES) |

---

## Fetcher Patterns

All registered in `internal/resource/registry.go`, implemented in `internal/aws/*.go`:

| Pattern | Signature | Use Case |
|---------|-----------|----------|
| **PaginatedFetcher** | `func(ctx, clients, token) (FetchResult, error)` | Top-level resource lists (EC2, S3, RDS, etc.) |
| **PaginatedChildFetcher** | `func(ctx, clients, parentCtx, token) (FetchResult, error)` | Child resource lists (S3 objects, role policies, ECS tasks) |
| **FilteredPaginatedFetcher** | `func(ctx, clients, filter, token) (FetchResult, error)` | Server-side filtered queries (CloudTrail events) |
| **RevealFetcher** | `func(ctx, clients, resourceID) (string, error)` | On-demand secret reveal (`x` key — Secrets Manager, SSM) |
| **DetailEnricher** | `func(ctx, clients, Resource) (Resource, error)` | On-demand detail enrichment (policy documents) |
| **IssueEnricherFunc** | `func(ctx, *ServiceClients, []Resource) (IssueEnricherResult, error)` | Wave 2 issue enrichment; `IssueEnricherResult` carries issue count, truncated flag, and per-resource `Findings` map |

Each fetcher takes `clients any` and type-asserts to `*aws.ServiceClients` internally. This allows tests to inject mocks.

**Throttling protection** (`internal/aws/retry.go`): `RetryOnThrottle[T any](ctx, cfg, fn)` wraps an AWS API call with exponential backoff for `ThrottlingException` / `Throttling` / `RequestLimitExceeded` errors. Fetchers and enrichers that iterate per-resource (e.g., `EnrichTargetGroupHealth` calling `DescribeTargetHealth` once per TG) wrap each call in `RetryOnThrottle` so a throttled slice still completes instead of returning a half-populated result. Non-throttling errors are returned immediately without retry.

### Wave 2 Issue Enrichment Pipeline

Some resource types hide problems behind extra API calls (e.g., EC2 with impaired status checks, RDS with pending maintenance). Wave 2 enrichment discovers these hidden issues after Wave 1 probes complete.

This section documents the current Wave 2 implementation on `main`. The refactor plan in [`docs/refactor/03-finding-model.md`](refactor/03-finding-model.md) and [`docs/refactor/04-catalog.md`](refactor/04-catalog.md) replaces this registry-and-markdown-driven model with canonical findings and catalog-owned metadata.

**Architecture:**
- `internal/aws/issue_enrichment.go` — Wave 2 infrastructure: `IssueEnricherRegistry` map, unexported `registerIssueEnricher` helper (panics on empty name, nil fn, duplicate short name), `NoOpIssueEnricher`, `IssueEnricher` struct, `IssueEnricherFunc` / `IssueEnricherResult` types, shared helpers, `EnrichmentCap` / `PerParentPageCap`.
- `internal/aws/*_issue_enrichment.go` — per-type enricher registrations, including real enrichers and `NoOpIssueEnricher` placeholders for types whose Wave 2 column is currently "None". Each file's `init()` calls `registerIssueEnricher(<shortname>, <fn>, <priority>)` and — if the enricher writes `FieldUpdates` — `resource.RegisterIssueEnricherFieldKeys(<shortname>, [...])`.
- `internal/tui/app_probes.go` — `buildEnrichQueue()`, `probeEnrichment()`
- `internal/tui/app_handlers_availability.go` — `startEnrichment()`, `handleEnrichmentChecked()` with only-increase guard

**Flow:**

```text
Wave 1 probes complete
  → startEnrichment() builds queue from IssueEnricherRegistry ∩ probeResources
  → probeEnrichment() dispatches issue enrichers (4-at-a-time, same as Wave 1)
  → EnrichmentCheckedMsg arrives
    → only-increase guard: menu badge updated only if new count > current
    → progress indicator updated
  → all done: clear probeResources, save cache with enriched counts (when caching enabled)
```

**Registry**: On `main`, Wave 2 capability is expressed through `IssueEnricherRegistry` entries, including `NoOpIssueEnricher` placeholders that make "no Wave 2 signal" explicit and testable. Some types with `NoOpIssueEnricher` still perform in-fetcher Wave 2 work — their fetchers already make per-resource Describe calls and populate health fields at fetch time (e.g., EKS `health_issues_count`, CloudTrail `is_logging`, OpenSearch `cluster_health`).

**Priority order** (`buildEnrichQueue`): Batchable enrichers that make account-wide calls are dispatched first (e.g., RDS/DocDB maintenance, EC2 instance status). Per-resource enrichers (e.g., DynamoDB PITR, KMS rotation, S3 PAB) iterate over resource IDs/ARNs, capped at `EnrichmentCap` (50). The registry key for each enricher must match the `ShortName` Wave 1 uses to store probe resources — a mismatch silently skips the enricher.

**Resource identity**: Enrichers receive retained first-page resources from Wave 1 probes (`probeResources map[string][]resource.Resource`). Account-wide enrichers make a single API call covering all resources. Per-resource enrichers fan out to individual resources, capped at `EnrichmentCap` (50).

**Current contract on `main`**: [`docs/attention-signals.md`](attention-signals.md) is the hand-maintained source of truth for Wave 1 (Color func) and Wave 2 (issue-enricher) assignments per resource type. `TestAttentionSignalsDoc` parses the markdown table and enforces: every type with Wave 1 != "None" has a non-nil `Color` func, and every type with Wave 2 != "None" has an `IssueEnricherRegistry` entry. The refactor plan replaces this with generated docs from catalog data.

**Skip condition**: Wave 2 runs only when `isDemo=false`. Demo mode has no real AWS to query. `--no-cache` on live AWS still runs Wave 2 (it only disables disk persistence, not capabilities).

**Lifecycle**: `probeResources` is cleared after Wave 2 completes and on profile/region switch. On a top-level resource list with a registered enricher, Ctrl+R bumps `enrichmentTypeGen[rt]`, clears `enrichmentFindings[rt]` and `enrichmentRan[rt]`, calls `SetEnrichmentState(0, false, false, nil)` on the active list, and dispatches `refreshResourceListWithEnrichmentRerun` to rerun Wave 2 for that type. The main-menu Ctrl+R path invalidates Wave 2 for all types: it bumps `enrichmentGen` (the session-wide generation counter), resets `enrichmentTypeGen` to an empty map, clears `enrichmentFindings` and `enrichmentRan` — then reloads the cache from disk. Wave 2 re-runs when the user next navigates to a resource list.

### Issue-Enrichment Visibility Subsystem

After Wave 2 issue enrichment runs, findings are surfaced in list and detail views.

**Types:**
- `resource.EnrichmentFinding` (`internal/resource/enrichment.go`) — `Severity string` (`"!"` broken/degraded, `"~"` scheduled/informational) + `Summary string` (human-readable description).

**List view integration:**
- `ResourceListModel.SetEnrichmentState(issueCount int, truncated, ran bool, findings map[string]resource.EnrichmentFinding)` — stores Wave 2 results; called by `handleEnrichmentChecked` on arrival and called with zeroed args on Ctrl+R rerun start.
- `renderEnrichmentBanner()` — emits a styled banner line above the table when `ran=true` and findings exist; invisible until Wave 2 completes.
- Row-marker dot: a severity-colored middle dot (`·`) is prepended to the identity column for each resource that has a finding. Column position is determined by `resolveIdentityColumn()`.

**Detail view integration:**
- `DetailModel.SetEnrichmentFinding(f *resource.EnrichmentFinding)` — injects (or clears) a "Background Check" section in the detail view showing severity + summary. `nil` clears the section.

**Stacked-view live-update pattern:**
- `handleEnrichmentChecked` iterates the full view stack, not just the active view. This allows enrichment messages to update non-active `ResourceListModel` and `DetailModel` instances for the affected type. A user can navigate away to a detail view while Wave 2 runs and both the list (behind) and the detail receive the findings without requiring a re-open.

**Current-state ownership (Phase-05, AS-237)**: Session-scoped state lives exclusively in `session.Session`, owned by `runtime.Core` and accessed from `tui.Model` via `m.core.Session()`. The `tui.Model` struct holds only pure UI-shell state (view stack, input mode, flash, tab completion). Profile/region switches call `m.core.Session().Rotate()` which bumps every generation counter and rebuilds the maps — in-flight async messages tagged with the pre-switch gens are then rejected by the handlers' gen guards.

Representative fields on `session.Session` (`internal/session/session.go`):
- `EnrichmentRan map[string]bool` — banner visibility signal; `true` only after Wave 2 completed for that type.
- `EnrichmentTypeGen map[string]domain.Gen` — per-type Wave 2 generation counter; bumped on Ctrl+R rerun to invalidate stale in-flight results.
- `EnrichmentTruncatedIDs map[string]map[string]bool` — per-type set of resource IDs the enricher had to skip due to API truncation.
- `EnrichmentGen`, `AvailabilityGen`, `RelatedGen`, `EnrichGen` — per-purpose session-wide generation counters; guard stale in-flight async results.
- `ResourceCache`, `RelatedCache`, `LazyResourceCache` — session-scoped resource and related-check caches.
- `LazyResourceCache map[string][]resource.Resource` — sparse per-type cache populated by the related-panel lazy-add path when a checker emits IDs outside the top-level fetcher's scope filter (e.g. AWS-managed KMS key, public AMI, IAM `AdministratorAccess`). Consulted by `handleRelatedNavigate` (union-read with `ResourceCache`, `ResourceCache` wins on ID collision) but NEVER by the main-menu top-level list. Cleared on `Rotate()`.

**Wave 2 findings (where they live):** PR-03a-fold deleted the parallel `EnrichmentFindings map[string]map[string]resource.EnrichmentFinding` map from `session.Session`. Wave 2 findings are now written directly onto each cached `resource.Resource.Findings` slice (with `Source` prefix `wave2:`) and `r.AttentionDetails`, via `applyEnrichment` in `internal/tui/app_enrich_fold.go`. Reads use `findingFromResource` / `findingsFromRows` against the cached rows. The runtime's view-ready snapshot surface (`runtime.RuntimeState.EnrichmentFindings`, `internal/runtime/state.go`) and the `PatchDetail.EnrichmentFindings` intent payload (`internal/runtime/intent.go`) carry per-resource findings out to adapters, but neither replaces the cached-row authority — they are derived from it.

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

**Optional capability interfaces** (`views/view.go`): views implement these only when the capability applies. The root model type-asserts against each interface and calls only the implementations present, so older views without the capability keep working.

| Interface | Methods | Implemented by |
|-----------|---------|----------------|
| `Filterable` | `SetFilter(text string)`, `GetFilter() string` | Navigable list views (main menu, resource list, selector). Static views (detail, YAML, JSON, help, reveal) do NOT implement it. |
| `Searchable` | `IsSearchActive() bool`, `IsSearchInputMode() bool`, `SearchInfo() string` | Text-content views (detail, YAML, JSON). Drives the search status line in the frame footer. |
| `Hintable` | `BottomHints() []layout.KeyHint` | Views that render context-specific key hints along the bottom border. Views that omit it get a plain bottom border — the absence is backward-compatible. |

### Issue Counting & Attention Filter

The main menu shows `issues:N` badges per resource type, counting resources in warning/error states. The ctrl+z key filters the menu to show only types with issues.

**Row Coloring:**
- `resource.Color` enum: `ColorHealthy` (green), `ColorWarning` (yellow), `ColorBroken` (red), `ColorDim` (grey).
- `(Color).IsIssue() bool` — returns true for `ColorWarning` and `ColorBroken`. Used by both the attention filter and issue-count badges.
- `ResourceTypeDef.Color func(Resource) Color` — per-type classification function. Two patterns: (1) status-driven types read `r.Fields["state"]` or `r.Fields["status"]` (e.g., EC2, ECS, VPC); (2) field-specific types check multi-field conditions (e.g., SG checks `dangerous_open_count > 0`, IAM Role checks `assume_role_policy_document` for wildcard principal). Types with Wave 1 = "None" in `docs/attention-signals.md` have a trivial `func(_ Resource) Color { return ColorHealthy }` — they rely on Wave 2 enrichers. REQUIRED for all registered types.
- `ResourceTypeDef.ResolveColor(r Resource) Color` — dispatcher: calls `d.Color(r)` when non-nil, falls back to `resource.fallbackColor(r.Status)` for ad-hoc test doubles that omit `Color`.
- `resource.fallbackColor(status string) Color` — status-string fallback covering common AWS vocabulary; used only when `Color` is nil (test doubles).
- `styles.ColorStyle(c resource.Color) lipgloss.Style` — maps `resource.Color` to a palette foreground style for row rendering.

**`TierColorStyle`** (`styles.TierColorStyle(tier string) lipgloss.Style`): Maps detail-view tier strings to palette foreground styles. Tiers: `"ok"`, `"!"` (broken), `"~"` (warning/scheduled), `"impaired"`, `"initializing"`, `"ct-danger"`, `"ct-attention"`, `"ct-info"`.

**`IdentityKey` and `resolveIdentityColumn`**: The enrichment row-marker dot is placed in the "identity column" — the column that most clearly names the resource. `ResourceTypeDef.IdentityKey` pins the column by key. When empty, `resolveIdentityColumn(cols, td)` applies a 5-step cascade:
1. `td.IdentityKey` matches a column's `Key`
2. column `Key == "name"`
3. column `Path` contains `"Name"` or `"Identifier"`
4. column `Title` equals `"Name"` (case-insensitive) or equals `td.Name`
5. fall back to column index 0

**`CellDecorators` and `lookupDecorator`**: `ResourceTypeDef.CellDecorators` is a `map[string]func(Resource, string) string` that transforms a cell's display value before render. `lookupDecorator(decs, col)` resolves the right decorator via a fallback chain: column `Key` → column `Path` → `Path`'s final segment (lowercased) → column `Title` (lowercased). Only EC2 currently uses this (to prefix state with `"! "` for impaired or `"~ "` for degraded-but-running).

**AttentionFilter** (`internal/tui/views/attention.go`): A shared toggle struct embedded by both `MainMenuModel` and `ResourceListModel`. Owns only enabled/disabled state — views do their own counting and rendering.

**Issue counting flow**:
1. Wave 1 probes (or `demoPrefetchCounts()`) count `td.ResolveColor(r).IsIssue()` rows from first page
2. Counts flow to `MainMenuModel` via `SetIssues()`, rendered as `issues:N` badges
3. Wave 2 enrichment discovers hidden issues, updates badges (only-increase guard)
4. `popView()` sync-back: when user returns from a list, the list's `issueCount` syncs back to the menu — but only if higher than the current menu count (prevents overwriting enriched counts)

**`ExcludeFromIssueBadge`**: When set on a `ResourceTypeDef`, rows are still colored and ctrl+z is honored, but the type is excluded from the main-menu badge count. Used by ct-events where severity is event-level, not resource-health.

**Tri-state visibility** under ctrl+z on the main menu. Per [`docs/attention-signals.md`](attention-signals.md), every registered resource type has at least a Wave 1 or Wave 2 signal, so there is no "always healthy" escape hatch — a zero issue count is only "CONFIRMED zero" when the probe was not truncated.

| State | Condition | Badge | Visible under ctrl+z? |
|-------|-----------|-------|----------------------|
| Unknown | Not yet probed | None | Yes (prevent cold-start empty menu) |
| Confirmed zero | `issues == 0` AND `!truncated` | None | No — probe completed and saw no issues |
| Truncated zero | `issues == 0` AND `truncated == true` | None | Yes — lower bound; later pages may hold issues |
| Nonzero | `issues > 0` | `issues:N` (or `issues:N+` when truncated) | Yes |

`ExcludeFromIssueBadge` types (e.g. ct-events) are unconditionally hidden under ctrl+z — severity is event-level, not resource-health.

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

Column-position sorting is the only sort model: the `SortField` alias and the `SortName` / `SortID` / `SortAge` sentinels were removed in #283. Sort state is a column index (`sortColIdx int`) plus a direction flag.

---

## Related Views (Right Column)

The detail view has a right-column panel showing related resources.

> ⚠️ **The expected related-panel contract per resource type lives in [`related-resources.md`](./related-resources.md) — the SINGLE SOURCE OF TRUTH.**
> That document is produced from AWS API references + DevOps workflows (six
> independent audits). DO NOT edit `RegisterRelated` calls without reconciling
> against the golden table. Drift has already happened once — do not repeat it.

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

**Truncated-cache contract (`Approximate=true` / `ApproximateZero`)**: cache-scan checkers that can't see the full universe — because the target cache's `IsTruncated=true` after its first page — must signal the undercount rather than silently rendering `0`. `resource.ApproximateZero(shortName)` returns a sentinel `RelatedCheckResult{Count:0, Approximate:true}` used when a truncated cache yielded no matches yet later pages may contain some. File-local `truncatedResult*` helpers (in `ddb_related.go`, `s3_related.go`, `ses_related.go`, `redis_related.go`) produce the same shape when matches were found but the cache was still truncated. The UI renders these as `(N+)` or `(0+)` so operators know the real count is at least N.

### Navigable Fields

```go
type NavigableField struct {
    FieldPath  string // e.g., "VpcId"
    TargetType string // e.g., "vpc"
}
```

In the detail view, navigable fields are underlined. Pressing Enter on one emits `RelatedNavigateMsg`, which pushes a filtered list of the target resource type.

**ID-format normalization**: Some navigable fields carry ARNs (KMS `KeyArn`, IAM `RoleArn`, ECS `ClusterArn`, Lambda `FunctionArn`, CloudWatch `LogGroupArn`) while the target resource's `Resource.ID` is a bare name or alias. `resource.NavIDFromValue(targetType, value)` (in `internal/resource/related.go`) is a central registry that normalizes these values into bare IDs at navigation time. Target types with registered extractors: `kms`, `role`, `ecs`, `logs`, `s3`, `iam-user`. Other target types pass through unchanged. `buildFieldList` in `internal/tui/views/detail_fields.go` applies this transform to every scalar navigable item before rendering so the resolved bare ID matches `Resource.ID` on the target's list.

**List-typed scalar extraction**: `fieldpath.ExtractFirstListScalar(obj, dotPath)` (in `internal/fieldpath/extract.go`) walks slice-valued dotted paths to pull a scalar from the first element, enabling navigable fields on fields like `Subnets.SubnetId` without hand-rolled traversal. Returns an empty string when the path is empty, the slice is empty, or any intermediate step is nil — unlike `ExtractScalar`, which only walks struct fields and pointers.

---

## Enrichment

a9s has two distinct enrichment pipelines with disjoint contracts:

1. **Detail enrichment** (on-demand) — `resource.DetailEnricher` in `internal/resource/enricher.go`. Fetches additional data when a user opens a detail/YAML/JSON view (e.g., IAM policy documents). See below.
2. **Wave 2 issue enrichment** (background) — `awsclient.IssueEnricherFunc` registered in `awsclient.IssueEnricherRegistry` (infrastructure in `internal/aws/issue_enrichment.go`; one `*_issue_enrichment.go` file per short name). Discovers hidden issues via additional API calls after Wave 1 probes complete. See "Wave 2 Issue Enrichment Pipeline" under Fetcher Patterns.

### On-Demand Detail Enhancement

When a detail, YAML, or JSON view opens for a resource type with a registered enricher, the app async-fetches additional data and merges it into the resource.

**Flow:**

```text
View opens (detail, YAML, or JSON)
  → resource.HasDetailEnricher(resType)?
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
- **If caching is justified**: use a session-scoped, feature-specific cache owned by `session.Session` and passed to detail enrichers via `*awsclient.DetailEnrichmentCtx`. This is appropriate when the enrichment is relatively expensive and the data is unlikely to change within a session.
- **Never**: use package-global cache state for enrichers.

**Current example**: IAM policy document enrichment uses a session-scoped `PolicyDocumentCache` owned by `session.Session.PolicyDocCache` and passed to enrichers via `*awsclient.DetailEnrichmentCtx`. Cache keys are explicitly namespaced: `managed:<policyArn>` for managed policies, `inline:<roleName>/<policyName>` for inline. `session.Session.Rotate()` replaces the cache with a fresh instance on profile/region rotation so entries from a previous account cannot leak into the next.

---

## Child Views

Child views are drill-down lists from a parent resource (e.g., IAM Role → Role Policies).

```go
// internal/resource/types.go
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

This table describes the caches that exist on `main` today. The refactor plan's target is stricter ownership through an explicit session/runtime boundary; until that lands, the cache behavior below is the current contract.

| Cache | Location | Scope | Invalidation |
|-------|----------|-------|-------------|
| **Disk availability cache** | `internal/cache/` | Persisted at `~/.a9s/cache/<profile>--<region>.yaml` | TTL of 1 hour; file replaced atomically |
| **Resource cache** | `session.Session.ResourceCache` (owned by `runtime.Core`) | In-memory `map[string]*session.ResourceCacheEntry` | Cleared on profile/region switch via `session.Rotate()` |
| **Related cache** | `session.Session.RelatedCache` | In-memory LRU with fixed capacity | Cleared on `Rotate()`; entry deleted on Ctrl+R |
| **Detail-enricher caches** | Feature-specific cache on `session.Session`, delivered to enrichers via `*awsclient.DetailEnrichmentCtx` (current example: `PolicyDocumentCache`) | In-memory, session-scoped | Rotated by `session.Rotate()` on profile/region switch |
| **Enrichment visibility state** | `EnrichmentRan`, `EnrichmentTypeGen`, `EnrichmentTruncatedIDs`, `EnrichmentGen` on `session.Session` (Wave 2 progress/control); per-resource findings are folded into `resource.Resource.Findings` on cached rows — see "Wave 2 findings (where they live)" above | In-memory, session-scoped | Cleared per-type on Ctrl+R rerun start; cleared entirely on `Rotate()` |

**Disk availability cache** (`internal/cache/cache.go`): Tracks which resource types have resources, their counts, and issue counts. Loaded on startup to instantly grey-out empty types and show issue badges in the main menu. Structure: `File{Profile, Region, CheckedAt, Resources map[string]Entry}` where `Entry{HasResources, Count, Truncated, Issues, IssuesTruncated, IssuesKnown}`. The `IssuesKnown` bool distinguishes "probed and found zero issues" from "not yet probed" (both unmarshal as int 0 without this flag). When caching is enabled (not `--no-cache`), the cache is saved after Wave 1 probes complete and again after Wave 2 enrichment completes, so enriched issue counts persist across restarts. When `--no-cache` is active, `saveAvailabilityCache()` is a no-op.

**Resource cache**: Stores the full view state of previously-viewed resource lists (resources, pagination, filter, sort, cursor position). Enables instant back-navigation without re-fetching.

**Related cache**: LRU mapping `"resourceType:resourceID"` → related check results. Avoids re-running related checks when re-entering a detail view for the same resource.

**Enricher caches**: Caching is optional, not automatic. The default is no cache. When an enricher does cache, it should use a session-scoped, feature-specific cache on `session.Session` and reach the enricher through `*awsclient.DetailEnrichmentCtx`, so cache lifetime matches session lifetime and is rotated by `session.Rotate()`. The current example is the policy document enricher, which caches decoded documents by `managed:<policyArn>` or `inline:<roleName>/<policyName>`.

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

The `isDemo` flag controls whether Wave 2 enrichment runs — demo mode skips it (no real AWS to query), while `--no-cache` on live AWS preserves full functionality.

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
- `WithProfile(p)` — override the profile string (used in tests to set a specific profile without live AWS)
- `WithRegion(r)` — override the region string (used in tests to set a specific region without live AWS)
- `WithCommand("ec2")` — open directly to a resource type on startup
- `WithActiveTheme(name)` — set the initial active theme filename for the theme selector (used by `--theme` CLI flag)

---

## Extension Guide

The steps below describe how to extend the current `main` architecture. They are intentionally not the target contributor workflow after the refactor lands. For the target shape, see [`docs/refactor/00-overview.md`](refactor/00-overview.md) and [`docs/refactor/04-catalog.md`](refactor/04-catalog.md).

### Adding a New Resource Type

1. Add or update the `ResourceTypeDef` and built-in default view config.
2. Implement the fetcher in `internal/aws/` so it returns stable `resource.Resource` values with meaningful `ID`, `Name`, `Status`, `Issues`, `Fields`, and `RawStruct` for the current resource model on `main`.
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
4. Treat this `Register*`/Wave-2 path as current-state implementation guidance, not the long-term target API. New refactor work should follow the phased design under `docs/refactor/`.
5. If the enricher caches, document its scope and invalidation rules.
6. Add tests for success, error, stale-result rejection, and cache invalidation paths.

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
2. **Use demo fakes for TUI tests** — `tui.New("demo", "us-east-1", tui.WithClients(demo.NewServiceClients()), tui.WithIsDemo(true), tui.WithNoCache(true), tui.WithProfile(demo.DemoProfile), tui.WithRegion(demo.DemoRegion))`
3. **Use narrow interface mocks for fetcher tests** — one mock per AWS API method
4. **Always `stripANSI` before string assertions** — rendered output contains escape codes
5. **Clean up registries** — use `t.Cleanup(func() { resource.UnregisterDetailEnricher(...) })` for temporary registrations
6. **If an enricher caches, test session scoping** — different `ServiceClients` instances must get independent caches automatically. If an enricher does not cache, no cache cleanup should be required.

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

Async operations (related checks, enrichment) can outlive the view that triggered them. Generation counters (`relatedGen`, `enrichGen`, `availabilityGen`, `enrichmentTypeGen`) are incremented on context changes, causing stale in-flight results to be silently discarded. `enrichmentTypeGen` is a per-type counter for Wave 2: bumped on profile/region switch and on Ctrl+R when the active view is a top-level list with a registered enricher. The dual-generation guard (`Gen` on `EnrichmentCheckedMsg` for session-wide staleness, `TypeGen` for per-type rerun staleness) lets multiple types enrich concurrently while rerun invalidation only cancels the refreshed type.

### Why a separate enricher pattern?

Detail views render from pre-fetched `Fields`/`RawStruct`. Some data (like policy documents) requires additional API calls that are too expensive to make at list-fetch time. The enricher pattern fetches this data on demand when the user actually opens the detail, YAML, or JSON view.

### Why four separate caches?

Each cache serves a fundamentally different access pattern: disk cache survives restarts for instant startup; resource cache enables instant back-navigation; related cache avoids redundant API fanouts; enricher caches prevent repeated expensive single-resource fetches. Collapsing them would conflate TTL/invalidation/eviction policies.
