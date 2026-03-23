---
name: a9s-add-child-view
description: Blueprint for adding a new child view to a9s — step-by-step checklist with templates (child fetcher, child type registration, parent wiring, tests)
disable-model-invocation: true
---

# Adding a New Child View

Follow this checklist for each new child view. The generic child-view architecture handles all navigation, key dispatch, and message routing automatically.

Steps 1-8: Implementation. Steps 9-11: Tests.
**Skipping test steps is the #1 cause of regressions.**

## Prerequisites

You MUST have a resource spec from the architect with:
- Parent ShortName (e.g., `tg`, `ecs-svc`, `cfn`)
- Child ShortName (e.g., `tg_health`, `ecs_tasks`, `cfn_events`)
- Trigger key (`enter`, `e`, `L`, `r`, `s`)
- AWS API call(s)
- Context keys: what parameters the child fetcher needs from the parent resource
- List columns (field keys, titles, widths)
- Detail paths
- Pattern: A, B, C, or D (see below)

## Pattern Variants

### Decision Tree

1. **How many child views does the parent have?**
   - ONE child → **Pattern A** (Single Child). Trigger key is always `enter`.
   - MULTIPLE children → **Pattern B** (Multi-Child). One gets `enter`, others get `e`/`L`/`r`/`s`.
2. **Does the child view itself have children?**
   - YES → **Pattern C** (Level-2 Nested). The child's `RegisterChildType` includes its own `Children` slice.
3. **Does the child fetcher call a different AWS service than the parent?**
   - YES → **Pattern D** (Cross-Service). The fetcher resolves a log group/stream from parent metadata, then queries CloudWatch Logs.

Patterns combine: a cross-service level-2 nested view is Pattern C+D.

### Pattern A: Single Child (most common)
- Parent has 1 child. `Enter` on parent drills into child.
- Examples: Target Group Health, ASG Scaling Activities, Alarm History, ECR Images

### Pattern B: Multi-Child Parent
- Parent has 2+ children with different trigger keys.
- Examples:
  - ECS Services: `Enter`→Tasks, `e`→Events, `L`→Logs
  - CFN Stacks: `Enter`→Events, `r`→Resources
  - Lambda: `Enter`→Invocations, `s`→Code
- Each child gets its own `ChildViewDef` entry in the parent's `Children` slice.

### Pattern C: Level-2 Nested
- Child view itself has children (grandchild navigation).
- Examples:
  - Log Streams → Log Events (grandchild of Log Groups)
  - Lambda Invocations → Log Lines
  - ELB Listeners → Listener Rules
  - SFN Executions → Execution History
  - CodeBuild Builds → Build Logs
- The child's `RegisterChildType` includes `Children: []ChildViewDef{...}`
- Uses `@parent.` prefix in ContextKeys to carry context through nesting levels.

### Pattern D: Cross-Service
- Child fetcher calls a different AWS service than the parent.
- Examples:
  - Lambda → Invocations (uses CloudWatch Logs, not Lambda API)
  - ECS Services → Container Logs (uses CloudWatch Logs via task definition)
  - CodeBuild → Build Logs (uses CloudWatch Logs)
- May need multiple AWS API interfaces.
- Typically involves a resolution step (e.g., look up log group from parent metadata).

## Checklist

### 1. Child Fetcher: `internal/aws/{child_type}.go` (NEW FILE)

```go
package aws

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/aws/aws-sdk-go-v2/service/{service}"
    {service}types "github.com/aws/aws-sdk-go-v2/service/{service}/types"

    "github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
    resource.RegisterChildFetcher("{child_shortname}", func(ctx context.Context, clients interface{}, parentCtx resource.ParentContext) ([]resource.Resource, error) {
        c, ok := clients.(*ServiceClients)
        if !ok || c == nil {
            return nil, fmt.Errorf("AWS clients not initialized")
        }
        return Fetch{ChildTypeName}(ctx, c.{ServiceField}, parentCtx["{context_key}"])
    })

    resource.RegisterChildType(resource.ResourceTypeDef{
        Name:      "{Child Display Name}",
        ShortName: "{child_shortname}",
        Columns:   {ChildTypeName}Columns(),
        // For Pattern C (level-2 nested), add Children here:
        // Children: []resource.ChildViewDef{{...}},
    })

    resource.RegisterFieldKeys("{child_shortname}", []string{"{key1}", "{key2}", ...})
}

// {ChildTypeName}Columns returns the column definitions for the child list view.
func {ChildTypeName}Columns() []resource.Column {
    return []resource.Column{
        {Key: "{key1}", Title: "{Title1}", Width: 28, Sortable: true},
        {Key: "{key2}", Title: "{Title2}", Width: 12, Sortable: true},
        // ... from architect spec
    }
}

// Fetch{ChildTypeName} calls the AWS API to fetch child resources.
func Fetch{ChildTypeName}(ctx context.Context, api {InterfaceName}, {parentParam} string) ([]resource.Resource, error) {
    output, err := api.{APICall}(ctx, &{service}.{APICall}Input{
        {ParentField}: &{parentParam},
    })
    if err != nil {
        return nil, err
    }

    var resources []resource.Resource
    for _, item := range output.{ResultField} {
        id := /* ... */
        name := /* ... */
        status := /* ... */

        fields := map[string]string{
            "{key1}": /* ... */,
            "{key2}": /* ... */,
        }

        detail := map[string]string{ /* ... */ }

        rawJSON := ""
        if jsonBytes, err := json.MarshalIndent(item, "", "  "); err == nil {
            rawJSON = string(jsonBytes)
        }

        resources = append(resources, resource.Resource{
            ID:         id,
            Name:       name,
            Status:     status,
            Fields:     fields,
            DetailData: detail,
            RawJSON:    rawJSON,
            RawStruct:  item,
        })
    }
    return resources, nil
}
```

**Pattern D (Cross-Service) variation** — fetcher resolves log group first:
```go
func Fetch{ChildTypeName}(ctx context.Context, parentAPI {ParentInterfaceName}, logsAPI {LogsInterfaceName}, parentCtx resource.ParentContext) ([]resource.Resource, error) {
    // Step 1: Resolve log group from parent metadata
    logGroup := resolveLogGroup(ctx, parentAPI, parentCtx)
    // Step 2: Fetch log events from CloudWatch Logs
    output, err := logsAPI.FilterLogEvents(ctx, &cloudwatchlogs.FilterLogEventsInput{
        LogGroupName: &logGroup,
        // ...
    })
    // ... process and return resources
}
```

### 2. Interface: `internal/aws/interfaces.go` (APPEND)

```go
// {ChildTypeName}{APICall}API defines the interface for the {Service} {APICall} operation.
type {ChildTypeName}{APICall}API interface {
    {APICall}(ctx context.Context, params *{service}.{APICall}Input, optFns ...func(*{service}.Options)) (*{service}.{APICall}Output, error)
}
```

### 3. Client field: `internal/aws/client.go` (IF NEW SERVICE)

**Only needed if the child fetcher uses a service not already in `ServiceClients`.**

Add field to `ServiceClients` struct:
```go
{ServiceField} *{service}.Client
```

Add constructor line in `CreateServiceClients`:
```go
{ServiceField}: {service}.NewFromConfig(cfg),
```

**Skip this step** if the child reuses an existing client (e.g., ECS events reuse `ECS` client).

### 4. Add `Children` to parent `ResourceTypeDef`: `internal/resource/types.go` (EDIT)

Find the parent type's entry in the `resourceTypes` slice and add or append to its `Children`:

**Pattern A (single child):**
```go
Children: []ChildViewDef{{
    ChildType:      "{child_shortname}",
    Key:            "enter",
    ContextKeys:    map[string]string{"{context_key}": "ID"},
    DisplayNameKey: "{context_key}",
}},
```

**Pattern B (multi-child) — append to existing Children:**
```go
Children: []ChildViewDef{
    // existing child entry...
    {
        ChildType:      "{child_shortname}",
        Key:            "e",  // or "L", "r", "s"
        ContextKeys:    map[string]string{"{context_key}": "ID", "{other_key}": "field_name"},
        DisplayNameKey: "{display_key}",
    },
},
```

**ContextKeys source values:**
- `"ID"` → `Resource.ID`
- `"Name"` → `Resource.Name`
- `"Status"` → `Resource.Status`
- `"@parent.x"` → inherit from parent view's context (for Pattern C nesting)
- `"field_key"` → `Resource.Fields["field_key"]`

### 5. Default view config: `internal/config/defaults.go` (ADD)

```go
"{child_shortname}": {
    List: []ListColumn{
        {Title: "{Title1}", Path: "{SDKFieldName}", Width: 28},
        {Title: "{Title2}", Path: "{SDKFieldName}", Width: 12},
        // ... matching step 1 columns
    },
    Detail: []string{
        "{SDKField1}", "{SDKField2}", // from architect spec
    },
},
```

### 6. User view config: `.a9s/views.yaml` (ADD section)

```yaml
  {child_shortname}:
    list:
      {Title1}:
        path: {SDKFieldName}
        width: 28
      {Title2}:
        path: {SDKFieldName}
        width: 12
    detail:
      - {SDKField1}
      - {SDKField2}
```

For computed fields (no direct SDK path), use `key:` instead of `path:`:
```yaml
      Duration:
        key: duration
        width: 12
```

### 7. Refgen entry (IF child uses an SDK struct): `cmd/refgen/main.go` (APPEND)

```go
{"{child_shortname}", "{service}types.{SDKType}", reflect.TypeOf({service}types.{SDKType}{})},
```

Add import if new service types package. Skip if the child uses computed fields only (e.g., Lambda invocations parsed from log lines).

### 8. Mock: `tests/unit/mocks_test.go` (APPEND)

```go
// mock{ChildTypeName}Client implements awsclient.{InterfaceName} for testing.
type mock{ChildTypeName}Client struct {
    output *{service}.{APICall}Output
    err    error
}

func (m *mock{ChildTypeName}Client) {APICall}(
    ctx context.Context,
    params *{service}.{APICall}Input,
    optFns ...func(*{service}.Options),
) (*{service}.{APICall}Output, error) {
    return m.output, m.err
}
```

### 9. Fetcher tests: `tests/unit/aws_{child_shortname}_test.go` (NEW FILE)

Write tests covering:
- Happy path: fetcher returns correct resources with expected fields
- Empty response: fetcher returns empty slice, no error
- API error: fetcher returns the error
- Field extraction: all column keys populated correctly from mock data
- Parent context: fetcher correctly uses context keys (e.g., bucket name, zone ID)
- RawJSON: valid JSON marshaled
- RawStruct: original SDK struct preserved

**Use exact mock value assertions:**
```go
// GOOD — exact value comparison:
if r.Fields["target_id"] != "i-abc123" {
    t.Errorf("Fields[target_id] = %q, want %q", r.Fields["target_id"], "i-abc123")
}
```

### 10. Detail view tests: `tests/unit/qa_detail_new_types_test.go` (APPEND)

Follow the pattern from `a9s-add-resource` step 10. Add 3 tests:
- `TestQA_Detail_{ChildTypeName}_ViewContainsExpectedFields`
- `TestQA_Detail_{ChildTypeName}_NilFields`
- `TestQA_Detail_{ChildTypeName}_FrameTitle`

### 11. YAML + List view tests

Follow the pattern from `a9s-add-resource` steps 11-12.

### 12. Demo fixtures (OPTIONAL): `internal/demo/fixtures.go`

```go
func init() {
    RegisterChildDemo("{child_shortname}", func(parentCtx map[string]string) []resource.Resource {
        return []resource.Resource{
            {
                ID: "example-id", Name: "example-name", Status: "active",
                Fields: map[string]string{"{key1}": "value1", "{key2}": "value2"},
            },
            // ... more fixture data
        }
    })
}
```

## What You Do NOT Need to Change

- `app.go` — the generic `handleEnterChildView` and `fetchChildResources` handle all child views
- `messages.go` — `EnterChildViewMsg` handles all child navigation
- `resourcelist.go` — `handleChildKey` and `buildChildContext` handle all child key dispatch
- `keys.go` — child-view trigger keys (`e`, `L`, `r`, `s`, `t`) are already defined

## Quick Reference: Files per Pattern

### Pattern A (Simple Single-Child)

| Action | File |
|--------|------|
| CREATE | `internal/aws/{child_type}.go` |
| CREATE | `tests/unit/aws_{child_type}_test.go` |
| APPEND | `internal/aws/interfaces.go` |
| EDIT   | `internal/resource/types.go` (add Children to parent) |
| APPEND | `internal/config/defaults.go` |
| APPEND | `.a9s/views.yaml` |
| APPEND | `cmd/refgen/main.go` (if SDK struct) |
| APPEND | `tests/unit/mocks_test.go` |
| APPEND | `tests/unit/qa_detail_new_types_test.go` |
| APPEND | `tests/unit/qa_yaml_new_types_test.go` |
| APPEND | `tests/unit/qa_list_rawstruct_test.go` |

Skip `internal/aws/client.go` if the parent's service client already exists.

### Pattern B (Multi-Child)
Same as Pattern A. The `Children` edit in `types.go` appends to an existing slice.

### Pattern C (Level-2 Nested)
Same as Pattern A, plus the `RegisterChildType` call includes its own `Children` slice for the grandchild.

### Pattern D (Cross-Service)
Same as Pattern A, plus: add interfaces for ALL services, may need client fields for both, two mocks (resolve + fetch).

## Post-Implementation Steps

1. Run refgen (if SDK struct): `go run ./cmd/refgen/ > .a9s/views_reference.yaml`
2. Run tests: `go test ./tests/unit/ -count=1 -timeout 120s`
3. Run linter: `golangci-lint run ./...`
4. Run vulncheck: `govulncheck ./...`
5. Bump version in `cmd/a9s/main.go`
6. Build: `go build -o a9s ./cmd/a9s/`
