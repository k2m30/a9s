---
name: a9s-add-related-view
description: Blueprint for adding related-resource views to an a9s resource type -- split into CODER steps (1-7) and QA steps (8-12) with templates
disable-model-invocation: true
---

# Adding Related Views to a Resource Type

Two agents, two tracks. The architect scopes both tasks using the per-resource
research docs in `docs/design/related-resources/{shortname}.md`. Coder and QA
can run in parallel (pattern is rigid).

## Prerequisites

**Infrastructure must be in place first.** These must exist before any
per-resource related views can be added:

- `internal/resource/related.go` -- types, registries (`RelatedDef`, `NavigableField`), helper constructors
- `RelatedCheckResultMsg` and `RelatedNavigateMsg` in `internal/tui/messages/messages.go`
- `ToggleRelated` binding in `internal/tui/keys/keys.go`
- Two-column detail view in `internal/tui/views/detail.go` (field-list model with embedded rightColumnModel)
- Handler code in `internal/tui/app.go` (main Update switch) and `internal/tui/app_related.go` for `RelatedCheckResultMsg` and `RelatedNavigateMsg`

If these don't exist, STOP. The infrastructure must land first.
See `docs/design/related-views-architecture.md` Phases 1-8.

## Architect Must Provide

You MUST have a scoped task from the architect with:

- Source type ShortName (e.g., "ec2")
- **Left column (navigable fields):**
  - For each navigable field: FieldPath, TargetType
- **Right column (related types):**
  - For each related type:
    - Target ShortName (e.g., "sg", "vpc")
    - DisplayName override (if any)
    - For cache-based checks: which cache key to look up, how to match
    - For field-based checks: which Fields key to read
- Exact files to create/modify with append points

**If you don't have this, STOP.** Reply with REJECTED and ask for architect scope.

## Agent Ownership

| Steps | Owner | Writes to |
|-------|-------|-----------|
| 1-7 (implementation) | **a9s-coder** | `internal/`, `cmd/` |
| 8-12 (tests) | **a9s-qa** | `tests/unit/` |

**Coder MUST NOT write test files. QA MUST NOT write production code.**

## Relationship Patterns

### Pattern F: Forward / Field-Based (cheap, count shown)

IDs are already in the source resource's Fields or RawStruct. No external API
call needed. The checker reads from Fields or RawStruct directly.

```go
// Example: EC2 -> EBS volumes (read from RawStruct block device mappings)
func checkEC2EBS(_ context.Context, _ interface{}, res resource.Resource, _ resource.ResourceCache) resource.RelatedCheckResult {
    ids := ec2VolumeIDs(res) // reads from res.RawStruct
    if len(ids) == 0 {
        return resource.RelatedCheckResult{TargetType: "ebs", Count: 0}
    }
    ordered := make([]string, 0, len(ids))
    for id := range ids {
        ordered = append(ordered, id)
    }
    sort.Strings(ordered)
    return relatedResult("ebs", ordered)
}
```

### Pattern C: Cache-Based (reads from already-loaded resource lists)

Looks up related resources in the ResourceCache. Falls back to a live API
call only when the cache doesn't contain the target type. All cache-based
checkers follow this helper pattern:

```go
func check{Source}{Target}(ctx context.Context, clients interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
    // 1. Extract identity from res (ID, relevant fields, or RawStruct)
    sourceID, _ := extractSourceIdentity(res)
    if sourceID == "" {
        return resource.RelatedCheckResult{TargetType: "{target}", Count: 0}
    }

    // 2. Load target list from cache or live fetch
    targetList, truncated, err := {source}RelatedResources(ctx, clients, cache, "{target}")
    if err != nil {
        return resource.RelatedCheckResult{TargetType: "{target}", Count: -1, Err: err}
    }
    if targetList == nil {
        return resource.RelatedCheckResult{TargetType: "{target}", Count: -1}
    }

    // 3. Match against source
    var ids []string
    for _, targetRes := range targetList {
        // match by RawStruct fields first, fall back to resource.Fields
        if matchesSource(targetRes, sourceID) {
            ids = append(ids, targetRes.ID)
        }
    }
    // Truncation guard: partial page with 0 matches → unknown, not zero
    if len(ids) == 0 && truncated {
        return resource.RelatedCheckResult{TargetType: "{target}", Count: -1}
    }
    return relatedResult("{target}", ids)
}
```

The `relatedResult` helper (defined in `ec2_related.go`, copy-paste for other
source types) deduplicates and sorts IDs:

```go
func relatedResult(target string, ids []string) resource.RelatedCheckResult {
    if len(ids) == 0 {
        return resource.RelatedCheckResult{TargetType: target, Count: 0}
    }
    // deduplicate and sort ids ...
    return resource.RelatedCheckResult{
        TargetType:  target,
        Count:       len(uniq),
        ResourceIDs: uniq,
    }
}
```

**Count semantics:**
- `Count: 0` -- confirmed none
- `Count: -1` -- unknown (cache miss with no clients, or error)
- `Count: N` (N > 0) -- confirmed N found, ResourceIDs populated

### Pattern N: Naming-Convention (reverse lookup by name pattern)

The target resource's ID or name follows a predictable naming convention
that embeds the source resource's name. Search the target cache for matches.

```go
// Example: SFN → logs (log group name is /aws/vendedlogs/states/{sfn-name})
func checkSFNLogs(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
    sfnName := res.ID
    if sfnName == "" {
        return resource.RelatedCheckResult{TargetType: "logs", Count: 0}
    }
    expectedPrefix := "/aws/vendedlogs/states/" + sfnName

    logsList, truncated, err := sfnRelatedResources(ctx, clients, cache, "logs")
    // ... standard cache lookup, match by strings.HasPrefix(logRes.ID, expectedPrefix)
}
```

Known naming conventions:
- Lambda → logs: `/aws/lambda/{function-name}`
- CodeBuild → logs: `/aws/codebuild/{project-name}`
- SFN → logs: `/aws/vendedlogs/states/{state-machine-name}`
- EKS → logs: `/aws/eks/{cluster-name}/...`

### Pattern D: Dimension-Based (reverse lookup by alarm dimensions)

Search the alarm cache for alarms whose `Dimensions[]` contain a matching
dimension name/value. The source resource's ARN or name is the dimension value.

```go
// Example: SFN → alarm (alarm dimension StateMachineArn matches SFN ARN)
func checkSFNAlarm(ctx context.Context, clients any, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
    sfnARN := res.Fields["arn"]
    if sfnARN == "" {
        return resource.RelatedCheckResult{TargetType: "alarm", Count: -1}
    }
    alarmList, truncated, err := sfnRelatedResources(ctx, clients, cache, "alarm")
    // ... iterate alarms, assert MetricAlarm, check Dimensions for StateMachineArn == sfnARN
}
```

### When `Checker: nil` Is Acceptable

A nil checker (unknown count, shows "?" in UI) is ONLY acceptable when ALL
of the following are true:

1. **No forward fields**: The source RawStruct has no fields referencing the target
2. **No reverse fields**: No cached resource type has fields referencing this source
3. **No naming convention**: The target doesn't follow a name pattern embedding the source name
4. **No dimension match**: No alarm dimensions reference this source's ARN or name
5. **The relationship requires a separate API call** not available from any cached data

**Before marking a checker as nil, the architect MUST verify all five conditions.**
Checking the research doc (`docs/design/related-resources/{shortname}.md`) is
mandatory — it lists viable lookup strategies for each relationship.

Common mistakes:
- Marking alarm as nil when alarms have `Dimensions[]` referencing the source ARN
- Marking logs as nil when a `/aws/{service}/{name}` log group convention exists
- Marking a relationship as nil when OTHER cached resources have fields pointing back
  (e.g., SNS→alarm: alarm cache has `AlarmActions[]` containing topic ARNs)

---

# CODER STEPS (1-7) -- a9s-coder agent only

### 1. Registration: add to `internal/aws/{source}.go` (or `{source}_related.go`)

**IMPORTANT:** Module path is `github.com/k2m30/a9s/v3/...` (the `/v3` suffix is required).

Both `RegisterRelated` and `RegisterNavigableFields` calls belong in the
same `init()` as the resource type registration. For large resources like EC2
the related checker functions live in a separate `{source}_related.go` file
for readability, but the `RegisterRelated` calls stay in the main `init()`.

```go
func init() {
    // ... existing RegisterType / RegisterFetcher calls ...

    // --- Right column: related resource definitions ---
    resource.RegisterRelated("{source}", []resource.RelatedDef{
        // NeedsTargetCache: true for Pattern C (cache-based) checkers — triggers pre-fetch before checker runs.
        // Omit (false) for Pattern A/B checkers that call AWS directly.
        {TargetType: "{target1}", DisplayName: "{Display Name 1}", Checker: check{Source}{Target1}, NeedsTargetCache: true},
        {TargetType: "{target2}", DisplayName: "{Display Name 2}", Checker: check{Source}{Target2}, NeedsTargetCache: true},
        // Checker may be nil for stubs (shows as unknown count):
        {TargetType: "{target3}", DisplayName: "{Display Name 3}", Checker: nil},
    })

    // --- Left column: navigable fields ---
    resource.RegisterNavigableFields("{source}", []resource.NavigableField{
        {FieldPath: "{FieldName}", TargetType: "{target}"},
        {FieldPath: "{Section.FieldName}", TargetType: "{target}"},
        // ... one entry per navigable field in the detail view
    })
}
```

`RelatedDef` fields:
- `TargetType string` -- target resource short name (e.g., "tg", "alarm")
- `DisplayName string` -- right-column row label (e.g., "Target Groups")
- `Checker RelatedChecker` -- async checker function (nil for stubs)
- `NeedsTargetCache bool` -- set `true` for Pattern C (cache-based) checkers; triggers coordinated pre-fetch before the checker runs. Omitting this on a cache-based checker causes cold-cache misses to return `Count: -1` instead of the real count.

`NavigableField` fields:
- `FieldPath string` -- matches a label rendered in the detail view (e.g., "VpcId")
- `TargetType string` -- resource short name to navigate to

### 2. Checker functions: `internal/aws/{source}_related.go` (NEW FILE)

The `RelatedChecker` type signature is:

```go
type RelatedChecker func(ctx context.Context, clients interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult
```

Note: **no error return** -- errors are embedded in `RelatedCheckResult.Err`.

```go
package aws

import (
    "context"
    "sort"

    // ... SDK type imports as needed ...

    "github.com/k2m30/a9s/v3/internal/resource"
)

// check{Source}{Target1} checks the cache for {target1} resources related to this {source}.
func check{Source}{Target1}(ctx context.Context, clients interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
    sourceID := res.ID
    if sourceID == "" {
        return resource.RelatedCheckResult{TargetType: "{target1}", Count: 0}
    }

    targetList, truncated, err := {source}RelatedResources(ctx, clients, cache, "{target1}")
    if err != nil {
        return resource.RelatedCheckResult{TargetType: "{target1}", Count: -1, Err: err}
    }
    if targetList == nil {
        return resource.RelatedCheckResult{TargetType: "{target1}", Count: -1}
    }

    var ids []string
    for _, r := range targetList {
        // prefer RawStruct for accuracy, fall back to Fields
        if r.Fields["{source_id_field}"] == sourceID {
            ids = append(ids, r.ID)
        }
    }
    // Truncation guard: partial page with 0 matches → unknown, not zero
    if len(ids) == 0 && truncated {
        return resource.RelatedCheckResult{TargetType: "{target1}", Count: -1}
    }
    return relatedResult("{target1}", ids)
}

// check{Source}{Target2} checks the cache for {target2} resources related to this {source}.
func check{Source}{Target2}(ctx context.Context, clients interface{}, res resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
    // ... similar pattern ...
}

// {source}RelatedResources returns the resource list for target from cache or
// fetches the first page via the registered paginated fetcher.
// Returns (resources, isTruncated, error).
// isTruncated=true means the list is partial; callers MUST return Count=-1
// when 0 matches are found in a truncated list.
func {source}RelatedResources(ctx context.Context, clients interface{}, cache resource.ResourceCache, target string) ([]resource.Resource, bool, error) {
    resources, isTruncated, err := FetchRelatedTarget(ctx, clients, cache, target)
    // When AWS clients are not initialized (nil or wrong type), registered fetchers
    // return "AWS clients not initialized". Treat as graceful no-op (Count=-1, no error).
    if err != nil {
        if _, ok := clients.(*ServiceClients); !ok {
            return nil, false, nil
        }
    }
    return resources, isTruncated, err
}

// Do NOT define a {source}RelatedResult function. Call the shared package-level
// relatedResult(target, ids) from ec2_related.go — it lives in the same package
// and handles deduplication and sorting for all resource types.
```

`RelatedCheckResult` fields:
- `TargetType string` -- echoed from `RelatedDef.TargetType`
- `Count int` -- -1 = unknown; 0 = confirmed none; N > 0 = confirmed N
- `ResourceIDs []string` -- IDs of found related resources (empty when Count <= 0)
- `Err error` -- non-nil on error

**There is no `Available bool` field.**

### 3. Interfaces: `internal/aws/interfaces_<service>.go` (APPEND if needed)

Only needed if the checker's live-fetch fallback calls an API not already
covered by existing interfaces. Cache-only checkers that never call live APIs
do not need new interfaces.

```go
// Only add if not already present:
type {TypeName}{APICall}API interface {
    {APICall}(ctx context.Context, params *{service}.{APICall}Input, optFns ...func(*{service}.Options)) (*{service}.{APICall}Output, error)
}
```

### 4. Demo overrides

Register a demo checker so the related panel shows realistic data in demo mode.

**Hybrid fixture pattern (014-demo-transport-mock).** Demo mode has two layers: the legacy HTTP transport (`internal/demo/transport.go` + `handlers.go`) is the base for all services, and per-service typed fakes (`internal/demo/fakes/<service>.go`) override individual services. Currently only EC2 uses a typed fake.

- **Preferred (migrated services):** add fixture data to `internal/demo/fixtures/<service>.go` and extend the matching fake in `internal/demo/fakes/<service>.go`.
- **Legacy (non-migrated services):** add fixture data to the matching `internal/demo/fixtures_*.go` category file and (if needed) extend handlers in `internal/demo/handlers.go`.

When adding a new resource type, match the service's current layer. Do not mix layers for the same service.

#### Demo fixture support for related views

No separate demo registry is needed. Related checkers run against the typed fakes automatically. Ensure the target resource type's fixtures contain IDs that match what the source resource's fields reference. For example, if EC2 instances reference `vpc-prod-main` in their VpcId field, the VPC fake's fixtures must include a VPC with that ID.

Add fixture data to `internal/demo/fixtures/<service>.go` for the target resource type. The related checker will find it via the standard prefetch + cache path.

**Never** amend tests if fixtures do not have related IDs/fields. Fix fixtures, not tests.

Use the resource short name in PascalCase as `{SourceCamel}` and `{TargetCamel}`:
`ec2` → `EC2`, `rds` → `RDS`, `s3` → `S3`, `tg` → `TG`, `ebs` → `EBS`, etc.
Add a numeric suffix only when a source has multiple related IDs of the same target type.

### 5. Verify parent resource Fields

**Critical for left-column navigable fields.** Verify that the source
resource's regular fetcher populates the Fields keys that the
`NavigableField` entries reference.

For example, if a `NavigableField` has `FieldPath: "VpcId"`, verify
that `internal/aws/{source}.go` populates a field with key "VpcId" or that
the field appears in the detail view from RawStruct reflection.

If a required field is missing from Fields, add it to the regular fetcher.

### 6. Verify navigable field paths match detail view output

The `FieldPath` in `NavigableField` must match the actual key labels
rendered in the detail view. A mismatch silently leaves the field non-highlighted
across the entire resource type.

**Automated check (required):** Run the test added in QA step 9
(`TestNavigableFields_{Source}_FieldPathsResolve`) which verifies each registered
`FieldPath` resolves to a non-empty value against the resource's demo fixture:

```bash
go test ./tests/unit/ -run "TestNavigableFields_{Source}_FieldPathsResolve" -v -count=1
```

**Manual fallback:** If no demo fixture exists yet, check `.a9s/views_reference.yaml`
for the source resource — all field paths that appear in the detail view are
listed there. Verify each `FieldPath` matches a key in that file (case-sensitive).

For nested fields like `SecurityGroups.GroupId`, the detail view renders
these as indented sub-fields. The FieldPath must match the leaf label
that appears in the rendered output.

### 7. Post-implementation verification

```bash
make test
make lint
make gofix
make build
```

---

# QA STEPS (8-12) -- a9s-qa agent only

### 8. Mocks: `tests/unit/mocks_test.go` (APPEND if needed)

Only needed if the checker's live-fetch fallback introduces NEW interfaces
not already present. Cache-only checkers that never call live APIs do not
need new mocks.

```go
// mock{TypeName}By{Filter}Client implements awsclient.{InterfaceName} for testing.
type mock{TypeName}By{Filter}Client struct {
    output *{service}.{APICall}Output
    err    error
}

func (m *mock{TypeName}By{Filter}Client) {APICall}(
    ctx context.Context,
    params *{service}.{APICall}Input,
    optFns ...func(*{service}.Options),
) (*{service}.{APICall}Output, error) {
    return m.output, m.err
}
```

### 9. Related checker tests: `tests/unit/aws_{source}_related_test.go` (NEW FILE)

Write tests covering each checker and navigable field registration.
Checkers receive a `resource.ResourceCache` -- populate it with test data
to simulate the cache-hit path. Test the cache-miss path by passing an
empty or nil cache.

```go
package unit_test

import (
    "context"
    "testing"

    // ... SDK type imports as needed ...

    "github.com/k2m30/a9s/v3/internal/demo"
    "github.com/k2m30/a9s/v3/internal/fieldpath"
    "github.com/k2m30/a9s/v3/internal/resource"
)

func {source}CheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
    t.Helper()
    for _, def := range resource.GetRelated("{source}") {
        if def.TargetType == target {
            if def.Checker == nil {
                t.Fatalf("{source} related checker for %s is nil", target)
            }
            return def.Checker
        }
    }
    t.Fatalf("{source} related checker for %s not found", target)
    return nil
}

// --- Navigable Field Registration Tests ---

func TestNavigableFields_{Source}_Registered(t *testing.T) {
    fields := resource.GetNavigableFields("{source}")
    if len(fields) == 0 {
        t.Fatal("no navigable fields registered for {source}")
    }

    expected := map[string]string{
        "{FieldPath1}": "{target1}",
        "{FieldPath2}": "{target2}",
    }
    for path, targetType := range expected {
        nav := resource.IsFieldNavigable("{source}", path)
        if nav == nil {
            t.Errorf("expected navigable field %q not found", path)
            continue
        }
        if nav.TargetType != targetType {
            t.Errorf("field %q: TargetType = %q, want %q", path, nav.TargetType, targetType)
        }
    }
}

// TestNavigableFields_{Source}_FieldPathsResolve verifies that each registered
// NavigableField.FieldPath resolves to a non-empty value against the demo fixture.
// A mismatch here means the field will silently never be highlighted in the detail view.
func TestNavigableFields_{Source}_FieldPathsResolve(t *testing.T) {
    // Get demo resource for this source type (must be populated by the demo fixture)
    resources, ok := demo.GetResources("{source}")
    if !ok {
        t.Skip("no demo fixture registered for {source}")
    }
    if len(resources) == 0 {
        t.Skip("demo fixture returned no resources for {source}")
    }
    r := resources[0]

    fields := resource.GetNavigableFields("{source}")
    if len(fields) == 0 {
        t.Fatal("no navigable fields registered for {source}")
    }

    for _, nav := range fields {
        items := fieldpath.ExtractFieldList(r.RawStruct, []string{nav.FieldPath}, r.Fields, nil)
        found := false
        for _, item := range items {
            if item.Value != "" && item.Value != "-" {
                found = true
                break
            }
        }
        if !found {
            t.Errorf("NavigableField.FieldPath %q resolved to empty/missing value in demo fixture — check FieldPath spelling or add field to fetcher", nav.FieldPath)
        }
    }
}

// --- Checker Tests ---

func TestRelated_{Source}_{Target}_Found(t *testing.T) {
    // Build a fake target resource that should match the source
    fakeTarget := resource.Resource{
        ID: "target-id-1",
        Fields: map[string]string{
            "{source_id_field}": "source-id-1",
        },
    }
    cache := resource.ResourceCache{
        "{target}": resource.ResourceCacheEntry{Resources: []resource.Resource{fakeTarget}},
    }
    source := resource.Resource{ID: "source-id-1"}

    checker := {source}CheckerByTarget(t, "{target}")
    result := checker(context.Background(), nil, source, cache)

    if result.Count != 1 {
        t.Errorf("Count = %d, want 1", result.Count)
    }
    if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "target-id-1" {
        t.Errorf("ResourceIDs = %v, want [target-id-1]", result.ResourceIDs)
    }
    if result.Err != nil {
        t.Errorf("unexpected error: %v", result.Err)
    }
}

func TestRelated_{Source}_{Target}_NotFound(t *testing.T) {
    fakeTarget := resource.Resource{
        ID: "target-id-2",
        Fields: map[string]string{
            "{source_id_field}": "other-source-id",
        },
    }
    cache := resource.ResourceCache{
        "{target}": resource.ResourceCacheEntry{Resources: []resource.Resource{fakeTarget}},
    }
    source := resource.Resource{ID: "source-id-1"}

    checker := {source}CheckerByTarget(t, "{target}")
    result := checker(context.Background(), nil, source, cache)

    if result.Count != 0 {
        t.Errorf("Count = %d, want 0", result.Count)
    }
    if len(result.ResourceIDs) != 0 {
        t.Errorf("ResourceIDs = %v, want []", result.ResourceIDs)
    }
}

func TestRelated_{Source}_{Target}_CacheMissNoClients(t *testing.T) {
    // Empty cache + nil clients -> unknown (-1), no error
    source := resource.Resource{ID: "source-id-1"}
    checker := {source}CheckerByTarget(t, "{target}")
    result := checker(context.Background(), nil, source, resource.ResourceCache{})

    if result.Count != -1 {
        t.Errorf("Count = %d, want -1 (unknown)", result.Count)
    }
}

func TestRelated_{Source}_{Target}_EmptySourceID(t *testing.T) {
    source := resource.Resource{ID: ""}
    checker := {source}CheckerByTarget(t, "{target}")
    result := checker(context.Background(), nil, source, resource.ResourceCache{})

    if result.Count != 0 {
        t.Errorf("Count = %d, want 0 for empty source ID", result.Count)
    }
}
```

### 10. Cold-cache checker tests (MANDATORY for every registered source type)

Add to the same `tests/unit/aws_{source}_related_test.go` file. Tests drive the real checker through the typed fakes via the cold-cache harness — no demo registry needed:

```go
func TestRelated_{Source}_ColdCacheChecker(t *testing.T) {
    m := newDemoColdCacheApp(t)
    // Size, inject clients, navigate to source, fetch, open detail
    // Assert related check produces non-empty results via the real checker path
}
```

### 11. Registry tests: `tests/unit/related_registry_test.go` (APPEND)

Verify the registration was successful:

```go
func TestRelated_{Source}_Registered(t *testing.T) {
    defs := resource.GetRelated("{source}")
    if len(defs) == 0 {
        t.Fatal("no related defs registered for {source}")
    }

    // Verify expected target types are present
    expected := []string{"{target1}", "{target2}", "{target3}"}
    for _, exp := range expected {
        found := false
        for _, def := range defs {
            if def.TargetType == exp {
                found = true
                break
            }
        }
        if !found {
            t.Errorf("expected related def for target %q not found", exp)
        }
    }

    // Verify non-stub checkers exist
    for _, def := range defs {
        if def.Checker == nil {
            continue // stub entry, intentional
        }
        // Verify checker is callable (non-nil is sufficient for registration check)
    }
}
```

### 12. Post-test verification

```bash
go test ./tests/unit/ -count=1 -timeout 120s -run "Related_{Source}"
go test ./tests/unit/ -count=1 -timeout 120s -run "NavigableFields_{Source}"
make test
make lint
make gofix
```

---

## What You Do NOT Need to Change (per resource)

- `detail.go` -- the two-column detail view renders from registries generically
- `app.go` -- generic handlers dispatch to registry
- `messages.go` -- generic message types carry strings
- `keys.go` -- `r` toggle and `Tab` switching are generic
- `app_related.go` -- `handleRelatedCheckStarted()` and `handleRelatedNavigate()` are generic

## Research Reference

Each resource's related relationships are documented in:
`docs/design/related-resources/{shortname}.md`

These docs contain:
- Real-world use cases (why engineers need this relationship)
- Which other resource types reference or are referenced by this resource
- Which Fields/RawStruct paths to use for matching

Forward relationships and navigable field paths come from the resource's
own API response fields, documented in `.a9s/views_reference.yaml`.

## Architect Handoff Format

When the architect scopes related views for resource X, the handoff uses:

```
## RELATED VIEWS: {Source Display Name} ({source_shortname})

### Left Column -- Navigable Fields:
| Field Path | Target Type | Notes |
|------------|-------------|-------|
| {FieldPath} | {target} | {optional notes} |
| {Section.Field} | {target} | {optional notes} |

### Right Column -- Related Definitions:
| Target | DisplayName | Match Strategy | Cache Key | Notes |
|--------|------------|----------------|-----------|-------|
| {target} | {Display Name} | field: {field_key} == sourceID | {cache_key} | {notes} |
| {target} | {Display Name} | rawstruct: {field_path} | {cache_key} | {notes} |
| {target} | {Display Name} | nil (stub) | n/a | |

### CODER TASK:
Files to create:
  internal/aws/{source}_related.go -- checker functions + cache helper (reuse shared relatedResult and assertStruct from ec2_related.go — do NOT redefine)
Files to modify:
  internal/aws/{source}.go -- append RegisterRelated + RegisterNavigableFields in init()
  internal/demo/fixtures/<service>.go -- ensure target resource fixtures contain matching IDs
  internal/aws/interfaces_{service}.go -- append new interfaces (only if live-fetch fallback needs them)
    Append point: after last narrow interface, before the aggregate {Service}API
Context files (read-only):
  internal/aws/{source}.go -- verify Fields keys exist
  internal/aws/ec2_related.go -- canonical checker pattern
  internal/resource/related.go -- type definitions
  docs/design/related-resources/{source}.md -- relationship details
  .a9s/views_reference.yaml -- verify field paths

### QA TASK:
Test files to create:
  tests/unit/aws_{source}_related_test.go -- checker + navigable field + demo tests
Test files to modify:
  tests/unit/mocks_test.go -- append mocks (only if live-fetch fallback needs new interfaces)
    Append point: last mock in file
  tests/unit/related_registry_test.go -- append registration tests
    Append point: last TestRelated_ function
What to test:
  - Navigable field registration: all expected fields registered with correct target types
  - Checkers: found / not found / cache-miss-no-clients / empty-source-ID
  - Cold-cache checker: drives real checker through typed fakes via newDemoColdCacheApp
  - Registry: all expected defs registered
Context files (read-only):
  internal/aws/{source}_related.go -- function signatures
  internal/resource/related.go -- type definitions
```
