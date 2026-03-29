# Plan: Remove demo pagination system, unify fetch paths (#103 bug 3)

## Context

Issue #103 bug 3: back-navigation loses loaded pages. Root cause: demo mode has a completely separate pagination system (`demo/pagination.go`) with global mutable overflow maps that don't exercise the real app code paths. The `DemoPageSize=5` artificial truncation and stateful overflow consumption create bugs that don't exist in real AWS mode.

Demo mode MUST showcase pagination — it's a key feature. The fix: demo registers **paginated fetchers** (both top-level and child) in the standard `resource` registry, with enough fixture data to naturally span multiple pages.

## Goal

1. Demo mode uses the exact same fetch code paths as real AWS
2. Demo mode demonstrates pagination for both top-level and child resources
3. The demo-specific pagination system (`pagination.go`) is deleted entirely

## Key Design Decision

**Demo registers `resource.PaginatedFetcher` for top-level types and `resource.PaginatedChildFetcher` for child types** in the standard registry, overwriting real AWS fetchers for the lifetime of the demo-mode process.

A `fetchClients interface{}` field on Model holds either `*awsclient.ServiceClients` (real) or `*demo.Clients` (demo sentinel). All fetcher paths pass this to registry fetchers. Demo fetchers ignore the clients parameter.

**Page size = 20.** This is realistic (AWS APIs typically return 20-100). Types with >20 fixtures naturally paginate; types with <=20 load in one shot.

## Fixture Expansion Plan

Types already >= 20 (naturally paginate): EC2(25), Lambda(25), S3(22), Alarms(22), Logs(22)

Types to expand for category coverage:

| Type | Category | Current | Target | How |
|------|----------|---------|--------|-----|
| sg | Networking | 10 | ~25 | Generate 15 more SGs with realistic names/rules |
| role | Security | 4 | ~25 | Generate 21 more IAM roles with realistic names |
| dbi | Databases | 5 | ~25 | Generate 20 more RDS instances |
| role_policies (child) | Security | 7 | ~25 | Generate 18 more policy entries |
| s3_objects (child) | Storage | ~5 | ~25 | Expand one bucket's objects |

After expansion, pagination is demonstrated across compute, storage, monitoring, networking, security, and databases — plus at least 2 child view types.

## Steps

### 1. Create demo sentinel and registration (`internal/demo/`)

**Create `internal/demo/clients.go`:**
- `type Clients struct{}` — sentinel passed as `clients` to demo fetchers

**Create `internal/demo/register.go`:**
- `func RegisterFetchers()`:
  - Iterates `demoData`, calls `resource.RegisterPaginated(shortName, paginatedFetcher)` for each
  - Iterates `childDemoData`, calls `resource.RegisterPaginatedChild(childType, paginatedFetcher)` for each
  - Availability probes now use `GetPaginatedFetcher` (legacy `Register`/`GetFetcher` removed in #107/#109)
- Token format: `"demo:<offset>"` — stateless, deterministic, survives back-navigation
- Page size: `const PageSize = 20`
- Shared pagination helper used by both top-level and child fetchers:
```go
func paginate(all []resource.Resource, token string) resource.FetchResult {
    offset := parseDemoToken(token) // 0 if empty
    end := offset + PageSize
    if end > len(all) { end = len(all) }
    truncated := end < len(all)
    nextToken := ""
    if truncated { nextToken = fmt.Sprintf("demo:%d", end) }
    return resource.FetchResult{
        Resources: all[offset:end],
        Pagination: &resource.PaginationMeta{
            IsTruncated: truncated,
            NextToken:   nextToken,
            PageSize:    end - offset,
            TotalHint:   len(all),
        },
    }
}
```
- `func UnregisterFetchers()` — cleanup for tests

### 2. Modify `fetchResources` to try paginated fetcher first

**`internal/tui/app_fetchers.go` — `fetchResources`:**

Currently: `resource.GetFetcher(type)` returns all at once.

Change to: try `resource.GetPaginatedFetcher(type)` first. If found, use it (returns first page with pagination metadata). Fall back to `resource.GetFetcher(type)` for types without paginated fetcher.

This is the key change that makes top-level pagination work for BOTH demo and real AWS. When real AWS types eventually get paginated fetchers registered, they'll work through this same path.

```go
func (m *Model) fetchResources(resourceType string) tea.Cmd {
    fc := m.fetchClients
    return func() tea.Msg {
        if fc == nil {
            return messages.APIErrorMsg{...}
        }
        // Try paginated fetcher first (demo always has these; real AWS will gain them over time)
        if pf := resource.GetPaginatedFetcher(resourceType); pf != nil {
            result, err := pf(ctx, fc, "")  // empty token = first page
            if err != nil { return messages.APIErrorMsg{...} }
            return messages.ResourcesLoadedMsg{
                ResourceType: resourceType,
                Resources:    result.Resources,
                Pagination:   result.Pagination,
            }
        }
        // Fall back to non-paginated (loads all at once)
        fetcher := resource.GetFetcher(resourceType)
        resources, err := fetcher(ctx, fc)
        return messages.ResourcesLoadedMsg{ResourceType: resourceType, Resources: resources}
    }
}
```

### 3. Wire into app startup

**`cmd/a9s/main.go`:** Call `demo.RegisterFetchers()` when `demoMode` is true, before `tui.New()`

**`internal/tui/app.go`:**
- Add `fetchClients interface{}` field to `Model`
- In `Init()` demo branch: send `ClientsReadyMsg{Clients: &demo.Clients{}}`

**`internal/tui/app_handlers.go` (`handleClientsReady`):**
- Always set `m.fetchClients = msg.Clients`
- Keep existing `m.clients` type-assertion for non-fetcher operations (reveal, identity)

### 4. Remove demo branches from fetch paths (`internal/tui/app_fetchers.go`)

**Delete entirely:** `fetchDemoResources`, `fetchDemoChildResources`

**Modify** `fetchResources` (as above), `fetchChildResources`, `fetchMoreResources`, `probeResourceAvailability`:
- Remove `if m.demoMode` branches
- Use `fc := m.fetchClients` instead of `clients := m.clients`
- Pass `fc` to registry fetchers

`probeResourceAvailability` uses `GetFetcher` (non-paginated) — demo also registers these, returning all fixtures. Probe gets full count. Main menu shows e.g. "EC2 Instances (25)" — accurate.

**Keep** `fetchIdentity` and `saveAvailabilityCache` demo branches (not resource fetchers).

### 5. Delete `internal/demo/pagination.go`

Removes ~150 LOC: `DemoPageSize`, overflow maps, mutex, `GetResourcesPaginated`, `GetMoreResources`, `GetChildResourcesPaginated`, `GetMoreChildResources`, `childKey`.

### 6. Expand fixture data

**`internal/demo/fixtures_networking.go`** — `sgFixtures`: expand from 10 to ~25
**`internal/demo/fixtures_security.go`** — `roleFixtures`: expand from 4 to ~25; `rolePolicyFixtures`: expand from 7 to ~25
**`internal/demo/fixtures_databases.go`** — `rdsInstances`: expand from 5 to ~25; one S3 object set: expand to ~25

### 7. Update tests

**Delete:** `tests/unit/demo_pagination_test.go` (669 LOC testing deleted code)

**Modify:** `tests/unit/tui_wiring_test.go` — probes now return full fixture count via non-paginated fetcher
**Modify:** `tests/unit/qa_pagination_stories_test.go` — Section H: demo top-level resources now paginate through standard `PaginatedFetcher` path (page size 20, not 5). Update expected counts.

**Create:** `tests/unit/qa_backnav_pagination_test.go` — root-level test through `tui.Model`:
1. `demo.RegisterFetchers()` + create demo-mode model
2. Navigate to EC2 (25 fixtures, page size 20 -> first page: 20 items, truncated)
3. Verify frame title `"ec2(20+)"`
4. Press M -> execute LoadMoreMsg -> receive page 2 (5 items, not truncated)
5. Verify frame title `"ec2(25)"`
6. Push detail view -> press ESC -> pop back
7. Assert: still 25 items, frame title `"ec2(25)"`, cursor preserved

**Create:** `tests/unit/demo_registry_test.go` — verify `RegisterFetchers()` populates standard registry, paginated fetchers return correct pages with `&demo.Clients{}`

## Files Changed

| File | Action |
|------|--------|
| `internal/demo/clients.go` | CREATE |
| `internal/demo/register.go` | CREATE |
| `internal/demo/pagination.go` | DELETE |
| `internal/demo/fixtures_networking.go` | expand SG fixtures to ~25 |
| `internal/demo/fixtures_security.go` | expand role + role_policies fixtures to ~25 |
| `internal/demo/fixtures_databases.go` | expand RDS + S3 object fixtures to ~25 |
| `internal/tui/app.go` | add `fetchClients` field, update Init() |
| `internal/tui/app_fetchers.go` | remove 6 demo branches, delete 2 functions, try paginated first, use `fetchClients` |
| `internal/tui/app_handlers.go` | set `fetchClients` in handleClientsReady |
| `cmd/a9s/main.go` | call `demo.RegisterFetchers()` |
| `tests/unit/demo_pagination_test.go` | DELETE |
| `tests/unit/tui_wiring_test.go` | update probe tests |
| `tests/unit/qa_pagination_stories_test.go` | update Section H |
| `tests/unit/qa_backnav_pagination_test.go` | CREATE |
| `tests/unit/demo_registry_test.go` | CREATE |

## Verification

1. `go build ./...` compiles
2. `go test ./tests/unit/ -count=1 -timeout 120s` passes
3. `golangci-lint run ./...` — 0 issues
4. `govulncheck ./...` — no vulnerabilities
5. `./a9s --demo`:
   - EC2 shows 20 items initially with `ec2(20+)` in title
   - Press M -> loads remaining 5, title becomes `ec2(25)`
   - Load-more indicator visible at bottom when truncated
   - Help screen shows M key when truncated
   - Back-navigation from detail preserves all 25 items
   - Main menu shows full counts: "EC2 Instances (25)", "Security Groups (25)", etc.
