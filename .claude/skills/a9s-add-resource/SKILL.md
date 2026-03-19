---
name: a9s-add-resource
description: Blueprint for adding a new AWS resource type to a9s — 9-file checklist with templates
disable-model-invocation: true
---

# Adding a New AWS Resource Type

Follow this exact 9-file checklist for each new resource type. Use EC2 as the canonical example.

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

### 9. Tests: `tests/unit/aws_{shortname}_test.go` (NEW FILE)

Write tests covering:
- Happy path: fetcher returns correct resources with expected fields
- Empty response: fetcher returns empty slice, no error
- API error: fetcher returns the error
- Field extraction: all column keys populated correctly
- RawJSON: valid JSON marshaled
- RawStruct: original SDK struct preserved

## Post-Implementation Steps

1. Run refgen: `go run ./cmd/refgen/ > views_reference.yaml`
2. Run tests: `go test ./tests/unit/ -count=1 -timeout 120s`
3. Build: `go build -o a9s ./cmd/a9s/`
4. Bump version in `cmd/a9s/main.go`
5. Rebuild: `go build -o a9s ./cmd/a9s/`
