---
name: a9s-add-resource
description: Blueprint for adding a new AWS resource type to a9s — 12-step checklist with templates (fetcher, types, config, tests, view-layer tests)
disable-model-invocation: true
---

# Adding a New AWS Resource Type

Follow this exact 12-step checklist for each new resource type. Use EC2 as the canonical example.

Steps 1-8: Implementation. Step 9: Fetcher tests. Steps 10-12: View-layer tests (detail, YAML, list).
**Skipping steps 10-12 is the #1 cause of test coverage gaps.**

## Prerequisites

You MUST have a resource spec from the architect with:
- ShortName, Aliases, Display Name
- AWS SDK import, SDK Type, API call
- Pattern: A, B, or C (see below)
- List columns (field keys, titles, widths)
- Detail paths

## Pattern Variants

Pick the right pattern based on the architect spec:

### Decision Tree

1. **Does this resource need a NEW AWS service client?**
   - YES → **Pattern A** (Simple). Example: Lambda, CloudWatch, IAM
   - NO (reuses existing client) → **Pattern B** (Client Reuse). Example: VPC, SG reuse EC2; Subnets reuse EC2
2. **Does fetching require multiple API calls?** (list parent → list children → describe each)
   - YES → **Pattern C** (Multi-Step Fetch). Example: Node Groups (ListClusters → ListNodegroups → DescribeNodegroup)

### Pattern A: Simple (EC2, RDS, S3)
- 1 API call, 1 new interface, new client field in `ServiceClients`
- Standard fetcher template (see Checklist step 1)
- Standard single-output mock

### Pattern B: Client Reuse (VPC, SG)
- 1 API call, 1 new interface, **NO new client field**
- Skip step 3 (client.go) entirely
- `init()` passes existing client: e.g., `c.EC2` for VPC/SG
- Architect spec includes `ExistingClient: EC2`

```go
// Pattern B init() — note c.EC2 not c.VPC
func init() {
    resource.Register("vpc", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
        c, ok := clients.(*ServiceClients)
        if !ok || c == nil {
            return nil, fmt.Errorf("AWS clients not initialized")
        }
        return FetchVPCs(ctx, c.EC2)  // reuses EC2 client
    })
}
```

### Pattern C: Multi-Step Fetch (Node Groups)
- Multiple API calls, multiple interfaces, fetcher takes multiple API params
- `init()` passes same client multiple times
- Nested loops: list parent → list children → describe each
- Architect spec includes `API Sequence:` with ordered steps

```go
// Pattern C init() — same client passed 3 times for 3 interfaces
func init() {
    resource.Register("ng", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
        c, ok := clients.(*ServiceClients)
        if !ok || c == nil {
            return nil, fmt.Errorf("AWS clients not initialized")
        }
        return FetchNodeGroups(ctx, c.EKS, c.EKS, c.EKS)
    })
}

// Pattern C fetcher signature — multiple API interfaces
func FetchNodeGroups(
    ctx context.Context,
    listClustersAPI EKSListClustersAPI,
    listNodegroupsAPI EKSListNodegroupsAPI,
    describeNodegroupAPI EKSDescribeNodegroupAPI,
) ([]resource.Resource, error) {
    // Step 1: List parents
    // Step 2: For each parent, list children
    // Step 3: For each child, describe
}
```

**Pattern C mocks** use map-based outputs keyed by parent resource name:

```go
type mockEKSListNodegroupsClient struct {
    outputs map[string]*eks.ListNodegroupsOutput // keyed by cluster name
    err     error
}

type mockEKSDescribeNodegroupClient struct {
    outputs map[string]*eks.DescribeNodegroupOutput // keyed by "cluster/nodegroup"
    err     error
}
```

## Common Sub-Patterns

### Name from Tags (VPC pattern)
Some resources have no `Name` field — extract from Tags:
```go
name := ""
for _, tag := range item.Tags {
    if tag.Key != nil && *tag.Key == "Name" {
        if tag.Value != nil {
            name = *tag.Value
        }
        break
    }
}
```

### No Status Field (SG pattern)
Some resources have no status concept. Set `Status: ""`:
```go
r := resource.Resource{
    ID:     groupID,
    Name:   groupName,
    Status: "",  // Security Groups have no status
    // ...
}
```

### Nil-Guarded Nested Access (Node Groups pattern)
Guard for nil nested structs before accessing fields:
```go
desiredSize := ""
if ng.ScalingConfig != nil {
    if ng.ScalingConfig.DesiredSize != nil {
        desiredSize = fmt.Sprintf("%d", *ng.ScalingConfig.DesiredSize)
    }
}
```

### Count-Based Display (SG detail pattern)
Singular/plural for counts in DetailData:
```go
count := len(sg.IpPermissions)
if count == 1 {
    detail["Inbound Rules"] = "1 rule"
} else {
    detail["Inbound Rules"] = fmt.Sprintf("%d rules", count)
}
```

### Label/Tag Map Iteration (Node Groups pattern)
For map-type labels/tags:
```go
for k, v := range ng.Labels {
    detail["Label: "+k] = v
}
for k, v := range ng.Tags {
    detail["Tag: "+k] = v
}
```

## Checklist

### 1. Fetcher: `internal/aws/{type}.go` (NEW FILE)

```go
package aws

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/aws/aws-sdk-go-v2/service/{service}"
    {service}types "github.com/aws/aws-sdk-go-v2/service/{service}/types"

    "github.com/k2m30/a9s/internal/resource"
)

func init() {
    resource.Register("{shortname}", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
        c, ok := clients.(*ServiceClients)
        if !ok || c == nil {
            return nil, fmt.Errorf("AWS clients not initialized")
        }
        return Fetch{TypeName}(ctx, c.{ServiceField})
    })
}

func Fetch{TypeName}(ctx context.Context, api {InterfaceName}) ([]resource.Resource, error) {
    output, err := api.{APICall}(ctx, &{service}.{APICall}Input{})
    if err != nil {
        return nil, err
    }

    var resources []resource.Resource
    for _, item := range output.{ResultField} {
        // Extract ID, Name, Status from SDK struct
        id := /* ... */
        name := /* ... */
        status := /* ... */

        // Build Fields map matching column keys from types.go
        fields := map[string]string{
            "key1": /* ... */,
        }

        // Build DetailData map
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

### 2. Interface: `internal/aws/interfaces.go` (APPEND)

```go
// {TypeName}{APICall}API defines the interface for the {Service} {APICall} operation.
type {TypeName}{APICall}API interface {
    {APICall}(ctx context.Context, params *{service}.{APICall}Input, optFns ...func(*{service}.Options)) (*{service}.{APICall}Output, error)
}
```

### 3. Client field: `internal/aws/client.go` (ADD TO ServiceClients + CreateServiceClients)

**If the service is NEW** (not already in ServiceClients), add field and constructor:

Add field to `ServiceClients` struct:
```go
{ServiceField} *{service}.Client
```

Add constructor line in `CreateServiceClients`:
```go
{ServiceField}: {service}.NewFromConfig(cfg),
```

Add import if new service.

**If the service already exists** (e.g., VPC and SG reuse `EC2` client, Node Groups reuse `EKS` client), skip this step. The fetcher's `init()` references the existing client field (e.g., `c.EC2` for VPC/SG).

### 4. Resource type def: `internal/resource/types.go` (APPEND to resourceTypes slice)

```go
{
    Name:      "{Display Name}",
    ShortName: "{shortname}",
    Aliases:   []string{"{alias1}", "{alias2}"},
    Columns: []Column{
        {Key: "key1", Title: "Title1", Width: 28, Sortable: true},
        // ... from architect spec
    },
},
```

### 5. Default view config: `internal/config/defaults.go` (ADD to defaultViews.Views map)

```go
"{shortname}": {
    List: []ListColumn{
        {Title: "Title1", Path: "SDKFieldName", Width: 28},
        // ... matching types.go columns
    },
    Detail: []string{
        "SDKField1", "SDKField2", // from architect spec
    },
},
```

### 6. User view config: `views.yaml` (ADD section)

```yaml
  {shortname}:
    list:
      Title1:
        path: SDKFieldName
        width: 28
      # ... matching defaults.go
    detail:
      - SDKField1
      - SDKField2
```

### 7. Refgen entry: `cmd/refgen/main.go` (APPEND to resources slice)

```go
{"{shortname}", "{service}types.{SDKType}", reflect.TypeOf({service}types.{SDKType}{})},
```

Add import if new service types package.

### 8. Mock: `tests/unit/mocks_test.go` (APPEND)

```go
// mock{TypeName}Client implements awsclient.{InterfaceName} for testing.
type mock{TypeName}Client struct {
    output *{service}.{APICall}Output
    err    error
}

func (m *mock{TypeName}Client) {APICall}(
    ctx context.Context,
    params *{service}.{APICall}Input,
    optFns ...func(*{service}.Options),
) (*{service}.{APICall}Output, error) {
    return m.output, m.err
}
```

### 9. Fetcher tests: `tests/unit/aws_{shortname}_test.go` (NEW FILE)

Write tests covering:
- Happy path: fetcher returns correct resources with expected fields
- Empty response: fetcher returns empty slice, no error
- API error: fetcher returns the error
- Field extraction: all column keys populated correctly
- RawJSON: valid JSON marshaled
- RawStruct: original SDK struct preserved

**CRITICAL — use exact mock value assertions, NOT `== ""`:**

```go
// BAD — weak assertion, caught by coverage analysis as a gap:
if r.DetailData["VPC"] == "" {
    t.Error("DetailData[VPC] must not be empty")
}

// GOOD — exact mock value comparison:
if r.DetailData["VPC"] != "vpc-111" {
    t.Errorf("DetailData[VPC] = %q, want %q", r.DetailData["VPC"], "vpc-111")
}
```

Every mock sets specific values (e.g., `VpcId: aws.String("vpc-111")`). The test MUST assert the exact value extracted by the fetcher, not just that it's non-empty. This catches mapping bugs where the wrong field is read.

---

### 10. Detail view tests: `tests/unit/qa_detail_new_types_test.go` (APPEND — package `unit_test`)

**Pattern file:** `tests/unit/qa_detail_test.go`
**Helpers:** `tests/unit/helpers_external_test.go` (`buildResource`, `configForType`, `newDetailModel`, `ensureNoColor`)

Add 3 tests per resource type:

```go
// 1. realistic SDK struct builder
func realistic{TypeName}() {service}types.{SDKType} {
    return {service}types.{SDKType}{
        {Field1}: ptrString("value1"),
        {Field2}: ptrString("value2"),
        // populate all fields used by DefaultViewDef detail paths
    }
}

// 2. ViewContainsExpectedFields — verify config-driven detail rendering
func TestQA_Detail_{TypeName}_ViewContainsExpectedFields(t *testing.T) {
    ensureNoColor(t)
    raw := realistic{TypeName}()
    res := buildResource("test-id", "test-name", raw)
    cfg := configForType("{shortname}")
    m := newDetailModel(res, "{shortname}", cfg)
    view := m.View()
    if !strings.Contains(view, "value1") {
        t.Errorf("{TypeName} detail should contain {Field1}, got:\n%s", view)
    }
}

// 3. NilFields — zero-value SDK struct must not panic
func TestQA_Detail_{TypeName}_NilFields(t *testing.T) {
    ensureNoColor(t)
    raw := {service}types.{SDKType}{} // all nil/zero
    res := buildResource("empty", "empty", raw)
    cfg := configForType("{shortname}")
    m := newDetailModel(res, "{shortname}", cfg)
    view := m.View()
    if view == "" {
        t.Error("detail view should not be empty with nil {TypeName} fields")
    }
}

// 4. FrameTitle — verify it returns the resource name
func TestQA_Detail_{TypeName}_FrameTitle(t *testing.T) {
    raw := realistic{TypeName}()
    res := buildResource("test-id", "test-name", raw)
    cfg := configForType("{shortname}")
    m := newDetailModel(res, "{shortname}", cfg)
    if m.FrameTitle() != "test-name" {
        t.Errorf("FrameTitle = %q, want %q", m.FrameTitle(), "test-name")
    }
}
```

**For types without SDK structs** (e.g., SQS uses attribute maps, ECS uses ARN strings):
use `buildResourceWithFields` and pass `nil` for RawStruct. The detail view falls back to Fields rendering.

### 11. YAML view tests: `tests/unit/qa_yaml_new_types_test.go` (APPEND — package `unit`)

**Pattern file:** `tests/unit/qa_yaml_test.go`
**Helpers:** `yamlView()` and `yamlModel()` from `qa_yaml_test.go`

Add 3 tests per resource type + a fixture function:

```go
// Fixture returning []resource.Resource with Fields map
func fixture{TypeName}() []resource.Resource {
    return []resource.Resource{{
        ID: "test-id", Name: "test-name", Status: "active",
        Fields: map[string]string{
            "key1": "value1",
            "key2": "value2",
            // all column keys from types.go
        },
    }}
}

// 1. ViewContainsFields — all field keys and values rendered
func TestQA_YAML_{TypeName}_ViewContainsFields(t *testing.T) {
    items := fixture{TypeName}()
    for _, item := range items {
        out := yamlView(t, item, 120, 40)
        for k, v := range item.Fields {
            if !strings.Contains(out, k) {
                t.Errorf("{TypeName} YAML missing key %q", k)
            }
            if v != "" && !strings.Contains(out, v) {
                t.Errorf("{TypeName} YAML missing value %q", v)
            }
        }
    }
}

// 2. FrameTitle — "yaml" in title
func TestQA_YAML_{TypeName}_FrameTitle(t *testing.T) {
    items := fixture{TypeName}()
    m := yamlModel(items[0], 120, 40)
    title := m.FrameTitle()
    if !strings.Contains(title, "yaml") {
        t.Errorf("FrameTitle() = %q, want 'yaml' in title", title)
    }
}

// 3. RawContentUncolored — no ANSI escape codes in plain content
func TestQA_YAML_{TypeName}_RawContentUncolored(t *testing.T) {
    items := fixture{TypeName}()
    m := yamlModel(items[0], 120, 40)
    raw := m.RawContent()
    if strings.Contains(raw, "\x1b[") {
        t.Error("{TypeName} RawContent() contains ANSI codes")
    }
}
```

### 12. List RawStruct test: `tests/unit/qa_list_rawstruct_test.go` (APPEND — package `unit_test`)

**Pattern file:** `tests/unit/qa_list_rawstruct_test.go` (see `TestQA_ListRawStruct_EC2`)
**Helper:** `newListModel()` from the same file

Add 1 test per resource type (skip types without SDK structs):

```go
func TestQA_ListRawStruct_{TypeName}(t *testing.T) {
    ensureNoColor(t)
    cfg := configForType("{shortname}")

    raw := realistic{TypeName}() // from step 10's builder
    res := resource.Resource{
        ID: "test-id", Name: "test-name", Status: "active",
        Fields: map[string]string{
            "key1": "value1", // matching types.go column keys
        },
        RawStruct: raw,
    }

    view := newListModel(t, "{shortname}", cfg, []resource.Resource{res})

    // Verify SDK struct field values appear in list output
    if !strings.Contains(view, "value1") {
        t.Errorf("{TypeName} list should contain value1 from RawStruct, got:\n%s", view)
    }
}
```

**Note:** The `realistic{TypeName}()` function from step 10 is reused here. Both files are in package `unit_test`, so they share builders.

---

## Why Steps 10-12 Matter

Coverage analysis found these gaps when steps 10-12 were missing:
- **22+ types with zero view-layer tests** — detail/YAML rendering was never verified
- **NilFields panics go undetected** — zero-value SDK structs can crash `fieldpath.ExtractSubtree`
- **Config-driven paths not exercised** — list columns from `defaults.go` might reference non-existent struct fields
- **Cross-cutting tests only covered types with explicit test functions** — adding RawStruct tests ensures the new type is exercised end-to-end

## Post-Implementation Steps

1. Run refgen: `go run ./cmd/refgen/ > views_reference.yaml`
2. Run tests: `go test ./tests/unit/ -count=1 -timeout 120s`
3. Build: `go build -o a9s ./cmd/a9s/`
4. Bump version in `cmd/a9s/main.go`
5. Rebuild: `go build -o a9s ./cmd/a9s/`
