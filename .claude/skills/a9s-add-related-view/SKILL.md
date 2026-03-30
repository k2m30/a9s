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

- `internal/resource/related.go` -- types, registries (`RelatedViewDef`, `NavigableFieldDef`), helper constructors
- `RelatedTypeCheckedMsg` and `NavigateToRelatedMsg` in `internal/tui/messages/messages.go`
- `ToggleRelated` and `StackResources` bindings in `internal/tui/keys/keys.go`
- Two-column detail view in `internal/tui/views/detail.go` (field-list model with embedded rightColumnModel)
- Handler code in `internal/tui/app_handlers.go` for `RelatedTypeCheckedMsg` and `NavigateToRelatedMsg`

If these don't exist, STOP. The infrastructure must land first.
See `docs/design/related-views-architecture.md` Phases 1-8.

## Architect Must Provide

You MUST have a scoped task from the architect with:

- Source type ShortName (e.g., "ec2")
- **Left column (navigable fields):**
  - For each navigable field: FieldPath, TargetType, IDExtract pattern
- **Right column (related types):**
  - For each related type:
    - Target ShortName (e.g., "sg", "vpc")
    - DisplayName override (if any)
    - Category: Forward / Reverse / Algorithmic
    - Priority: P0 / P1 / P2
    - For Forward: Fields key(s) to parse, helper to use
    - For Reverse: API call, interface name, filter params
    - For Algorithmic: algorithm description, API calls needed
- Exact files to create/modify with append points

**If you don't have this, STOP.** Reply with REJECTED and ask for architect scope.

## Agent Ownership

| Steps | Owner | Writes to |
|-------|-------|-----------|
| 1-7 (implementation) | **a9s-coder** | `internal/`, `cmd/` |
| 8-12 (tests) | **a9s-qa** | `tests/unit/` |

**Coder MUST NOT write test files. QA MUST NOT write production code.**

## Relationship Patterns

### Pattern F: Forward (cheap, count shown)

IDs are already in the source resource's Fields. No API call for checking.
Fetcher calls the target type's regular API with ID filters.

```go
// Checker: uses helper -- zero API calls
resource.ForwardSingleField("vpc_id")    // single ID in one field
resource.ForwardCommaSeparated("sg_ids") // comma-separated IDs

// Fetcher: makes targeted API call with known IDs
func(ctx context.Context, clients interface{}, source resource.Resource) ([]resource.Resource, error) {
    c := clients.(*aws.ServiceClients)
    id := source.Fields["vpc_id"]
    return FetchVPCsByIDs(ctx, c.EC2, []string{id})
}
```

### Pattern R: Reverse (expensive, no count shown)

Must query another AWS service to find resources that reference the source.

**MANDATORY**: All reverse checkers MUST wrap API calls in `RetryOnThrottle`
and use `MaxResults` (or equivalent) to minimize API data transfer. Checkers
only need existence + rough count, not full resource data.

```go
// Checker: makes API call with throttle protection
func(ctx context.Context, clients interface{}, source resource.Resource) (resource.RelatedCheckResult, error) {
    c := clients.(*aws.ServiceClients)
    output, err := aws.RetryOnThrottle(ctx, aws.DefaultRetryConfig(), func() (*ec2.DescribeInstancesOutput, error) {
        return c.EC2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
            Filters: []ec2types.Filter{
                {Name: aws.String("instance.group-id"), Values: []string{source.ID}},
            },
            MaxResults: aws.Int32(5), // existence check only -- not full fetch
        })
    })
    if err != nil {
        return resource.RelatedCheckResult{}, err
    }
    count := 0
    for _, r := range output.Reservations {
        count += len(r.Instances)
    }
    return resource.RelatedCheckResult{Available: count > 0, Count: count}, nil
}

// Fetcher: same API call but full results (also throttle-protected)
func(ctx context.Context, clients interface{}, source resource.Resource) ([]resource.Resource, error) {
    c := clients.(*aws.ServiceClients)
    return FetchInstancesBySG(ctx, c.EC2, source.ID) // internally uses RetryOnThrottle
}
```

**Without RetryOnThrottle, the coder task will be REJECTED during review.**

### Pattern A: Algorithmic (naming convention / multi-hop)

Requires resource-specific logic to determine the target.
Same throttle protection rules as Pattern R -- all API calls must use
`RetryOnThrottle` and minimize data transfer.

```go
// Example: Lambda -> CloudWatch Log Group via naming convention
func(ctx context.Context, clients interface{}, source resource.Resource) (resource.RelatedCheckResult, error) {
    c := clients.(*aws.ServiceClients)
    logGroupName := "/aws/lambda/" + source.Name
    exists, err := LogGroupExists(ctx, c.CWLogs, logGroupName) // internally uses RetryOnThrottle
    if err != nil {
        return resource.RelatedCheckResult{}, err
    }
    return resource.RelatedCheckResult{Available: exists, Count: 1}, nil
}
```

### Common Reverse Sub-Patterns

**Filter by VPC ID** (used by VPC -> EC2, VPC -> SG, VPC -> Subnet, etc.):
```go
func FetchInstancesByVPC(ctx context.Context, api EC2DescribeInstancesAPI, vpcID string) ([]resource.Resource, error)
func FetchSecurityGroupsByVPC(ctx context.Context, api EC2DescribeSecurityGroupsAPI, vpcID string) ([]resource.Resource, error)
```

**Filter by SG ID** (used by SG -> EC2, SG -> ENI, etc.):
```go
func FetchInstancesBySG(ctx context.Context, api EC2DescribeInstancesAPI, sgID string) ([]resource.Resource, error)
```

**Filter by tags** (used for CFN stack, EKS cluster discovery):
```go
func CheckTagValue(source resource.Resource, tagKey string) (string, bool)
```

These helper functions are reusable across multiple source types. Place
them in `internal/aws/related_{service}.go` files.

---

# CODER STEPS (1-7) -- a9s-coder agent only

### 1. Registration file: `internal/aws/{source}_related.go` (NEW FILE)

**IMPORTANT:** Module path is `github.com/k2m30/a9s/v3/...` (the `/v3` suffix is required).

```go
package aws

import (
    "context"
    "fmt"

    "github.com/aws/aws-sdk-go-v2/service/{service}"

    "github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
    // --- Left column: navigable fields ---
    resource.RegisterNavigableFields("{source}", []resource.NavigableFieldDef{
        {FieldPath: "{FieldName}", TargetType: "{target}", IDExtract: "direct"},
        {FieldPath: "{Section.FieldName}", TargetType: "{target}", IDExtract: "arn-resource"},
        // ... one entry per navigable field in the detail view
    })

    // --- Right column: reverse/algorithmic relationships ---
    // Forward relationships are NOT registered here -- they appear
    // only as navigable fields in the left column.

    // Reverse relationships (expensive, no count)
    resource.RegisterRelated("{source}", resource.RelatedViewDef{
        TargetType: "{target}",
        Category:   resource.RelReverse,
        Priority:   resource.PriorityP1,
    }, check{Source}{Target}, fetch{Source}{Target})

    // Algorithmic relationships
    resource.RegisterRelated("{source}", resource.RelatedViewDef{
        TargetType:  "{target}",
        DisplayName: "{Optional Override}",
        Category:    resource.RelAlgorithmic,
        Priority:    resource.PriorityP0,
    }, check{Source}{Target}, fetch{Source}{Target})
}

// --- Reverse checkers ---

func check{Source}{Target}(ctx context.Context, clients interface{}, source resource.Resource) (resource.RelatedCheckResult, error) {
    c, ok := clients.(*ServiceClients)
    if !ok || c == nil {
        return resource.RelatedCheckResult{}, fmt.Errorf("AWS clients not initialized")
    }
    // Make filtered API call to check existence -- MUST use RetryOnThrottle
    count, err := Count{Target}For{Source}(ctx, c.{ServiceField}, source.ID)
    if err != nil {
        return resource.RelatedCheckResult{}, err
    }
    return resource.RelatedCheckResult{Available: count > 0, Count: count}, nil
}

// --- Reverse fetchers ---

func fetch{Source}{Target}(ctx context.Context, clients interface{}, source resource.Resource) ([]resource.Resource, error) {
    c, ok := clients.(*ServiceClients)
    if !ok || c == nil {
        return nil, fmt.Errorf("AWS clients not initialized")
    }
    return Fetch{Target}By{Source}(ctx, c.{ServiceField}, source.ID)
}
```

### 2. Shared fetcher helpers: `internal/aws/related_{service}.go` (CREATE or APPEND)

If the target service's filtered fetch functions don't exist yet, create them.
These are reused across multiple source types.

```go
package aws

import (
    "context"
    "fmt"

    "github.com/aws/aws-sdk-go-v2/service/ec2"
    ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

    "github.com/k2m30/a9s/v3/internal/resource"
)

// Fetch{Type}ByIDs fetches resources matching specific IDs.
func Fetch{Type}ByIDs(ctx context.Context, api {InterfaceName}, ids []string) ([]resource.Resource, error) {
    // ... same field mapping as the regular fetcher
}

// Fetch{Type}ByFilter fetches resources matching a filter.
func Fetch{Type}ByFilter(ctx context.Context, api {InterfaceName}, filterName, filterValue string) ([]resource.Resource, error) {
    // ...
}

// Count{Type}ByFilter returns a count without full resource conversion.
func Count{Type}ByFilter(ctx context.Context, api {InterfaceName}, filterName, filterValue string) (int, error) {
    // Same API call but only count results, skip resource conversion
    // MUST use RetryOnThrottle and MaxResults
}
```

### 3. Interfaces: `internal/aws/interfaces.go` (APPEND)

Only needed for reverse/algorithmic relationships that use API calls not
already covered by existing interfaces.

Many related fetchers reuse existing interfaces (e.g., EC2 DescribeInstances
is already defined). Check before adding.

```go
// Only add if not already present:
type {TypeName}{APICall}API interface {
    {APICall}(ctx context.Context, params *{service}.{APICall}Input, optFns ...func(*{service}.Options)) (*{service}.{APICall}Output, error)
}
```

### 4. Demo overrides: `internal/demo/related_{category}.go` (CREATE or APPEND)

Only needed for reverse/algorithmic relationships (forward relationships
work automatically because demo fixtures populate the same Fields maps).

```go
package demo

import "github.com/k2m30/a9s/v3/internal/resource"

func init() {
    resource.RegisterDemoRelatedChecker("{source}", "{target}", func(
        _ context.Context, _ interface{}, _ resource.Resource,
    ) (resource.RelatedCheckResult, error) {
        return resource.RelatedCheckResult{Available: true, Count: 3}, nil
    })
}
```

### 5. Verify parent resource Fields

**Critical for left-column navigable fields.** Verify that the source
resource's regular fetcher populates the Fields keys that the
`NavigableFieldDef` entries reference.

For example, if a `NavigableFieldDef` has `FieldPath: "VpcId"`, verify
that `internal/aws/ec2.go` populates a field with key "VpcId" or that
the field appears in the detail view from RawStruct reflection.

If a required field is missing from Fields, add it to the regular fetcher.

### 6. Verify navigable field paths match detail view output

The `FieldPath` in `NavigableFieldDef` must match the actual key labels
rendered in the detail view. Run the detail view (or read `renderFromConfig`)
to verify the paths are correct.

For nested fields like `SecurityGroups.GroupId`, the detail view renders
these as indented sub-fields. The FieldPath must match the leaf label
that appears in the rendered output.

### 7. Post-implementation verification

```bash
go test ./tests/unit/ -count=1 -timeout 120s
golangci-lint run ./...
go build -o a9s ./cmd/a9s/
```

---

# QA STEPS (8-12) -- a9s-qa agent only

### 8. Mocks: `tests/unit/mocks_test.go` (APPEND)

Only needed for reverse/algorithmic relationships that introduce NEW
interfaces. Forward relationships reuse existing mocks.

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

### 9. Related resolver tests: `tests/unit/aws_{source}_related_test.go` (NEW FILE)

Write tests covering each checker, fetcher, AND navigable field registration:

```go
package unit_test

import (
    "context"
    "testing"

    "github.com/aws/aws-sdk-go-v2/aws"
    // ... service imports

    awsclient "github.com/k2m30/a9s/v3/internal/aws"
    "github.com/k2m30/a9s/v3/internal/resource"
)

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

// --- Forward Checker Tests ---

func TestRelated_{Source}_{Target}_ForwardChecker(t *testing.T) {
    checker := resource.ForwardSingleField("{field_key}")

    t.Run("available", func(t *testing.T) {
        source := resource.Resource{
            ID: "test-id",
            Fields: map[string]string{
                "{field_key}": "{target-id}",
            },
        }
        result, err := checker(context.Background(), nil, source)
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if !result.Available {
            t.Error("expected Available=true")
        }
        if result.Count != 1 {
            t.Errorf("Count = %d, want 1", result.Count)
        }
    })

    t.Run("empty field", func(t *testing.T) {
        source := resource.Resource{
            ID:     "test-id",
            Fields: map[string]string{},
        }
        result, err := checker(context.Background(), nil, source)
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if result.Available {
            t.Error("expected Available=false for empty field")
        }
    })

    t.Run("nil fields", func(t *testing.T) {
        source := resource.Resource{ID: "test-id"}
        result, err := checker(context.Background(), nil, source)
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if result.Available {
            t.Error("expected Available=false for nil Fields")
        }
    })
}

// --- Reverse Checker Tests ---

func TestRelated_{Source}_{Target}_ReverseChecker(t *testing.T) {
    t.Run("found", func(t *testing.T) {
        mock := &mock{Target}FilterClient{
            output: &{service}.{APICall}Output{
                {ResultField}: []types.{SDKType}{{ /* data */ }},
            },
        }
        result, err := awsclient.Check{Target}For{Source}(context.Background(), mock, "source-id")
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if !result.Available {
            t.Error("expected Available=true")
        }
    })

    t.Run("not found", func(t *testing.T) {
        mock := &mock{Target}FilterClient{
            output: &{service}.{APICall}Output{},
        }
        result, err := awsclient.Check{Target}For{Source}(context.Background(), mock, "source-id")
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if result.Available {
            t.Error("expected Available=false")
        }
    })

    t.Run("API error", func(t *testing.T) {
        mock := &mock{Target}FilterClient{err: fmt.Errorf("access denied")}
        _, err := awsclient.Check{Target}For{Source}(context.Background(), mock, "source-id")
        if err == nil {
            t.Fatal("expected error")
        }
    })
}

// --- Fetcher Tests ---

func TestRelated_{Source}_{Target}_Fetcher(t *testing.T) {
    t.Run("happy path", func(t *testing.T) {
        mock := &mock{Target}Client{
            output: &{service}.{APICall}Output{
                {ResultField}: []types.{SDKType}{
                    { /* realistic data */ },
                },
            },
        }
        resources, err := awsclient.Fetch{Target}ByIDs(context.Background(), mock, []string{"{target-id}"})
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if len(resources) != 1 {
            t.Fatalf("got %d resources, want 1", len(resources))
        }
    })

    t.Run("API error", func(t *testing.T) {
        mock := &mock{Target}Client{err: fmt.Errorf("access denied")}
        _, err := awsclient.Fetch{Target}ByIDs(context.Background(), mock, []string{"id"})
        if err == nil {
            t.Fatal("expected error")
        }
    })

    t.Run("empty result", func(t *testing.T) {
        mock := &mock{Target}Client{
            output: &{service}.{APICall}Output{},
        }
        resources, err := awsclient.Fetch{Target}ByIDs(context.Background(), mock, []string{"id"})
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        if len(resources) != 0 {
            t.Errorf("got %d resources, want 0", len(resources))
        }
    })
}
```

### 10. Throttle resilience tests (MANDATORY for every reverse/algorithmic checker)

**These tests are NOT optional.** Every reverse/algorithmic checker must have
throttle coverage. See `docs/design/related-views-architecture.md` section 3.8.

Add to the same `tests/unit/aws_{source}_related_test.go` file:

```go
func TestRelated_{Source}_{Target}_ThrottleThenSuccess(t *testing.T) {
    callCount := 0
    mock := &mock{Target}FilterCallFnClient{
        callFn: func(ctx context.Context, params *{service}.{APICall}Input,
            optFns ...func(*{service}.Options)) (*{service}.{APICall}Output, error) {
            callCount++
            if callCount == 1 {
                return nil, &smithy.GenericAPIError{Code: "Throttling", Message: "rate exceeded"}
            }
            return &{service}.{APICall}Output{
                {ResultField}: []types.{SDKType}{{ /* data */ }},
            }, nil
        },
    }
    result, err := awsclient.Check{Target}For{Source}(context.Background(), mock, "source-id")
    if err != nil {
        t.Fatalf("expected retry success, got: %v", err)
    }
    if !result.Available {
        t.Error("expected Available=true after throttle retry")
    }
    if callCount < 2 {
        t.Errorf("expected at least 2 calls (throttle + success), got %d", callCount)
    }
}

func TestRelated_{Source}_{Target}_ThrottleExhausted(t *testing.T) {
    mock := &mock{Target}FilterCallFnClient{
        callFn: func(ctx context.Context, params *{service}.{APICall}Input,
            optFns ...func(*{service}.Options)) (*{service}.{APICall}Output, error) {
            return nil, &smithy.GenericAPIError{Code: "Throttling", Message: "rate exceeded"}
        },
    }
    _, err := awsclient.Check{Target}For{Source}(context.Background(), mock, "source-id")
    if err == nil {
        t.Fatal("expected error after retries exhausted")
    }
    if !strings.Contains(err.Error(), "max retries") {
        t.Errorf("error should mention max retries, got: %v", err)
    }
}

func TestRelated_{Source}_{Target}_ContextTimeout(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
    defer cancel()
    time.Sleep(5 * time.Millisecond)

    mock := &mock{Target}FilterCallFnClient{
        callFn: func(ctx context.Context, params *{service}.{APICall}Input,
            optFns ...func(*{service}.Options)) (*{service}.{APICall}Output, error) {
            return nil, ctx.Err()
        },
    }
    _, err := awsclient.Check{Target}For{Source}(ctx, mock, "source-id")
    if err == nil {
        t.Fatal("expected context timeout error")
    }
}
```

**Mock with per-call function** -- add to `tests/unit/mocks_test.go` alongside
the standard mock:

```go
type mock{Target}FilterCallFnClient struct {
    callFn func(ctx context.Context, params *{service}.{APICall}Input,
        optFns ...func(*{service}.Options)) (*{service}.{APICall}Output, error)
}

func (m *mock{Target}FilterCallFnClient) {APICall}(
    ctx context.Context,
    params *{service}.{APICall}Input,
    optFns ...func(*{service}.Options),
) (*{service}.{APICall}Output, error) {
    return m.callFn(ctx, params, optFns...)
}
```

### 11. Registry tests: `tests/unit/related_registry_test.go` (APPEND)

Verify the registration was successful:

```go
func TestRelated_{Source}_Registered(t *testing.T) {
    defs := resource.GetRelatedDefs("{source}")
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

    // Verify checkers and fetchers exist
    for _, def := range defs {
        if resource.GetRelatedChecker("{source}", def.TargetType) == nil {
            t.Errorf("no checker registered for {source}->%s", def.TargetType)
        }
        if def.Category == resource.RelCloudTrail {
            continue
        }
        if resource.GetRelatedFetcher("{source}", def.TargetType) == nil {
            t.Errorf("no fetcher registered for {source}->%s", def.TargetType)
        }
    }
}

func TestNavigableFields_{Source}_Registered(t *testing.T) {
    fields := resource.GetNavigableFields("{source}")
    if len(fields) == 0 {
        t.Fatal("no navigable fields registered for {source}")
    }

    // Verify expected field paths and target types
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
```

### 12. Post-test verification

```bash
go test ./tests/unit/ -count=1 -timeout 120s -run "Related_{Source}"
go test ./tests/unit/ -count=1 -timeout 120s -run "NavigableFields_{Source}"
go test ./tests/unit/ -count=1 -timeout 120s  # full suite
golangci-lint run ./...
```

---

## What You Do NOT Need to Change (per resource)

- `detail.go` -- the two-column detail view renders from registries generically
- `app.go` -- generic handlers dispatch to registry
- `messages.go` -- generic message types carry strings
- `keys.go` -- `r` toggle and `Tab` switching are generic
- `app_handlers.go` -- `handleRelatedTypeChecked()` and `handleNavigateToRelated()` are generic

## Research Reference

Each resource's reverse and algorithmic relationships are documented in:
`docs/design/related-resources/{shortname}.md`

These docs contain:
- Real-world use cases (why engineers need this relationship)
- Reverse relationships: API call, filter method, priority
- Algorithmic relationships: algorithm, API calls, priority
- CloudTrail events (for reference only -- not registered per-resource)

Forward relationships and navigable field paths come from the resource's
own API response fields, documented in `.a9s/views_reference.yaml`.

## Architect Handoff Format

When the architect scopes related views for resource X, the handoff uses:

```
## RELATED VIEWS: {Source Display Name} ({source_shortname})

### Left Column -- Navigable Fields:
| Field Path | Target Type | ID Extract | Notes |
|------------|-------------|------------|-------|
| {FieldPath} | {target} | direct | {optional notes} |
| {Section.Field} | {target} | arn-resource | {optional notes} |

### Right Column -- Reverse Relationships (Pattern R):
| Target | API Call | Filter | Interface | MaxResults | Priority |
|--------|---------|--------|-----------|-----------|----------|
| {target} | {service}:{APICall} | {filter_name}={source_field} | {InterfaceName} | {5/10/N/A} | P1 |

### Right Column -- Algorithmic Relationships (Pattern A):
| Target | Algorithm | MaxResults | Priority |
|--------|-----------|-----------|----------|
| {target} | {description} | {5/10/N/A} | P0 |

### Rate Limit Evaluation:
- Total reverse/algorithmic checks per detail-view entry: {N}
- Same-service API collision groups: {e.g., "EC2: 3 calls, ELBv2: 1 call"}
- All reverse/algorithmic checkers use RetryOnThrottle: REQUIRED
- All checkers use MaxResults for existence checks: REQUIRED

### CODER TASK:
Files to create:
  internal/aws/{source}_related.go -- navigable fields + related defs + resolvers
  internal/aws/related_{service}.go -- shared fetch helpers (if new)
Files to modify:
  internal/aws/interfaces.go -- append new interfaces (if needed)
    Append point: last interface in file
  internal/demo/related_{category}.go -- demo overrides (if reverse)
    Append point: last RegisterDemoRelatedChecker in file
Context files (read-only):
  internal/aws/{source}.go -- verify Fields keys exist
  docs/design/related-resources/{source}.md -- relationship details
  .a9s/views_reference.yaml -- verify field paths

### QA TASK:
Test files to create:
  tests/unit/aws_{source}_related_test.go -- checker + fetcher + navigable field tests
Test files to modify:
  tests/unit/mocks_test.go -- append mocks (if new interfaces)
    Append point: last mock in file
  tests/unit/related_registry_test.go -- append registration test
    Append point: last TestRelated_ function
What to test:
  - Navigable field registration: all expected fields registered with correct target types
  - Forward checkers: available/unavailable/nil fields
  - Forward fetchers: happy path/empty/API error
  - Reverse checkers: found/not found/API error
  - Reverse fetchers: happy path/empty/API error
  - Throttle resilience (per reverse/algorithmic): throttle-then-success, throttle-exhausted, context-timeout
  - Registry: all expected defs registered
Context files (read-only):
  internal/aws/{source}_related.go -- function signatures
  internal/resource/related.go -- type definitions
```
