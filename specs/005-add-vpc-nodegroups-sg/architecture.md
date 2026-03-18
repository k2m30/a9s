# Architecture: 005-add-vpc-nodegroups-sg

**Author**: a9s-architect
**Date**: 2026-03-18
**Status**: Approved
**Spec**: `specs/005-add-vpc-nodegroups-sg/spec.md`

---

## 1. Frozen Package Boundary

Per architectural rules, `internal/aws/`, `internal/resource/`, `internal/config/`, and `internal/fieldpath/` are frozen. However, this feature **requires** modifications to:

- `internal/resource/types.go` — adding 3 new `ResourceTypeDef` entries to the `resourceTypes` slice
- `internal/aws/client.go` — no new client fields needed (VPC and SG use the existing `EC2` client; Node Groups use the existing `EKS` client)
- `internal/aws/interfaces.go` — adding 3 new narrow interfaces

**Ruling**: These are additive-only changes (appending to existing slices, adding new interfaces). No existing signatures or behavior change. This is the established pattern used by all 7 existing resource types. The frozen-package rule exists to prevent refactoring-induced breakage during the TUI rewrite; additive resource registration is the intended extension point for these packages. **Approved.**

New files (not frozen, purely additive):
- `internal/aws/vpc.go`
- `internal/aws/sg.go`
- `internal/aws/nodegroups.go`

---

## 2. Changes to `internal/resource/types.go`

Append these 3 entries to the `resourceTypes` slice, after the existing Secrets Manager entry (line 115):

### 2.1 VPC

```go
{
    Name:      "VPCs",
    ShortName: "vpc",
    Aliases:   []string{"vpc", "vpcs"},
    Columns: []Column{
        {Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
        {Key: "name", Title: "Name", Width: 28, Sortable: true},
        {Key: "cidr_block", Title: "CIDR Block", Width: 18, Sortable: true},
        {Key: "state", Title: "State", Width: 12, Sortable: true},
        {Key: "is_default", Title: "Default", Width: 9, Sortable: true},
    },
},
```

**Column width rationale**:
- `vpc_id` (24): VPC IDs are `vpc-` + 17 hex chars = 21 chars; 24 gives breathing room.
- `name` (28): Matches EC2 name column; typical VPC names are project-based, 10-25 chars.
- `cidr_block` (18): Longest common CIDR is `xxx.xxx.xxx.xxx/xx` = 18 chars.
- `state` (12): Values are `available` (9) or `pending` (7); 12 matches EC2 state column.
- `is_default` (9): Displays `true`/`false`; header "Default" is 7 chars; 9 is sufficient.

### 2.2 Security Groups

```go
{
    Name:      "Security Groups",
    ShortName: "sg",
    Aliases:   []string{"sg", "securitygroups", "security-groups"},
    Columns: []Column{
        {Key: "group_id", Title: "Group ID", Width: 24, Sortable: true},
        {Key: "group_name", Title: "Group Name", Width: 28, Sortable: true},
        {Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
        {Key: "description", Title: "Description", Width: 36, Sortable: false},
    },
},
```

**Column width rationale**:
- `group_id` (24): SG IDs are `sg-` + 17 hex chars = 20 chars; 24 gives breathing room.
- `group_name` (28): Typical SG names are descriptive, 10-30 chars; matches EC2 name column.
- `vpc_id` (24): Same as VPC ID column above.
- `description` (36): Descriptions are free-text, often 20-60 chars; 36 balances info vs. terminal width. Not sortable (free-text).

### 2.3 EKS Node Groups

```go
{
    Name:      "EKS Node Groups",
    ShortName: "nodegroups",
    Aliases:   []string{"nodegroups", "ng", "node-groups"},
    Columns: []Column{
        {Key: "nodegroup_name", Title: "Node Group", Width: 28, Sortable: true},
        {Key: "cluster_name", Title: "Cluster", Width: 24, Sortable: true},
        {Key: "status", Title: "Status", Width: 14, Sortable: true},
        {Key: "instance_types", Title: "Instance Types", Width: 20, Sortable: false},
        {Key: "desired_size", Title: "Desired", Width: 9, Sortable: true},
    },
},
```

**Column width rationale**:
- `nodegroup_name` (28): Node group names are typically 10-30 chars; matches cluster name columns.
- `cluster_name` (24): EKS cluster names, typically 8-25 chars.
- `status` (14): Values like `ACTIVE`, `CREATING`, `DEGRADED`, `DELETING`; 14 matches existing status columns.
- `instance_types` (20): Comma-joined list, e.g., `t3.medium, t3.large`; 20 covers single type well, truncates gracefully for multiple.
- `desired_size` (9): Small integer; header "Desired" is 7 chars; 9 is sufficient.

---

## 3. Changes to `internal/aws/client.go`

**No changes needed.**

VPC and Security Groups use the EC2 API (`DescribeVpcs`, `DescribeSecurityGroups`), which is already served by `ServiceClients.EC2` (`*ec2.Client`). The `*ec2.Client` already satisfies both `EC2DescribeInstancesAPI` and the new VPC/SG interfaces because the real client has all methods.

Node Groups use the EKS API (`ListNodegroups`, `DescribeNodegroup`), which is already served by `ServiceClients.EKS` (`*eks.Client`).

No new SDK service imports. No new client fields. The existing `CreateServiceClients` function is unchanged.

---

## 4. New Interfaces in `internal/aws/interfaces.go`

Add these 3 interfaces after the existing `SecretsManagerGetSecretValueAPI` interface:

```go
// EC2DescribeVpcsAPI defines the interface for the EC2 DescribeVpcs operation.
type EC2DescribeVpcsAPI interface {
    DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
}

// EC2DescribeSecurityGroupsAPI defines the interface for the EC2 DescribeSecurityGroups operation.
type EC2DescribeSecurityGroupsAPI interface {
    DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
}

// EKSListNodegroupsAPI defines the interface for the EKS ListNodegroups operation.
type EKSListNodegroupsAPI interface {
    ListNodegroups(ctx context.Context, params *eks.ListNodegroupsInput, optFns ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error)
}

// EKSDescribeNodegroupAPI defines the interface for the EKS DescribeNodegroup operation.
type EKSDescribeNodegroupAPI interface {
    DescribeNodegroup(ctx context.Context, params *eks.DescribeNodegroupInput, optFns ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error)
}
```

**Note**: No new imports required in `interfaces.go` — `ec2` and `eks` packages are already imported.

**Note**: 4 interfaces for 3 resource types because Node Groups require two EKS operations (`ListNodegroups` + `DescribeNodegroup`), following the same pattern as EKS Clusters (`EKSListClustersAPI` + `EKSDescribeClusterAPI`).

---

## 5. New Fetcher Files

### 5.1 `internal/aws/vpc.go`

**Pattern**: Single-step fetch (like EC2, S3). Same as `ec2.go`.

```
Package: aws
Imports: context, encoding/json, fmt, strings, ec2, ec2types, resource

init():
    resource.Register("vpc", func(ctx, clients) {
        c := clients.(*ServiceClients)
        return FetchVPCs(ctx, c.EC2)
    })

FetchVPCs(ctx context.Context, api EC2DescribeVpcsAPI) ([]resource.Resource, error):
    1. Call api.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{})
    2. For each vpc in output.Vpcs:
        a. Extract VpcId (*string -> string)
        b. Extract Name from Tags (iterate Tags, find Key=="Name")
        c. Extract CidrBlock (*string -> string)
        d. Extract State (VpcState -> string)
        e. Extract IsDefault (*bool -> "true"/"false")
        f. Build DetailData map[string]string with:
            - "VPC ID", "Name", "CIDR Block", "State", "Is Default"
            - "DHCP Options ID" (from DhcpOptionsId)
            - "Instance Tenancy" (from InstanceTenancy)
            - "Owner ID" (from OwnerId)
            - All tags as "Tag: <Key>" entries
        g. Build RawJSON via json.MarshalIndent(vpc, "", "  ")
        h. Create resource.Resource{
            ID:     vpcID,
            Name:   name,
            Status: state,
            Fields: map[string]string{
                "vpc_id":     vpcID,
                "name":       name,
                "cidr_block": cidrBlock,
                "state":      state,
                "is_default": isDefault,
            },
            DetailData: detail,
            RawJSON:    rawJSON,
            RawStruct:  vpc,
        }
    3. Return resources, nil
```

**Key decisions**:
- `Resource.ID` = VPC ID (the primary AWS identifier).
- `Resource.Name` = Name tag value (may be empty if no Name tag).
- `Resource.Status` = VPC state string (for status-color rendering).
- `RawStruct` = the `ec2types.Vpc` struct (enables fieldpath reflection for detail/YAML views).

### 5.2 `internal/aws/sg.go`

**Pattern**: Single-step fetch (like EC2). Same as `ec2.go`.

```
Package: aws
Imports: context, encoding/json, fmt, ec2, ec2types, resource

init():
    resource.Register("sg", func(ctx, clients) {
        c := clients.(*ServiceClients)
        return FetchSecurityGroups(ctx, c.EC2)
    })

FetchSecurityGroups(ctx context.Context, api EC2DescribeSecurityGroupsAPI) ([]resource.Resource, error):
    1. Call api.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{})
    2. For each sg in output.SecurityGroups:
        a. Extract GroupId (*string -> string)
        b. Extract GroupName (*string -> string)
        c. Extract VpcId (*string -> string)
        d. Extract Description (*string -> string)
        e. Build DetailData map[string]string with:
            - "Group ID", "Group Name", "VPC ID", "Description"
            - "Owner ID" (from OwnerId)
            - "Security Group ARN" (from SecurityGroupArn)
            - "Inbound Rules" (count as string, e.g., "3 rules")
            - "Outbound Rules" (count as string, e.g., "2 rules")
            - All tags as "Tag: <Key>" entries
        f. Build RawJSON via json.MarshalIndent(sg, "", "  ")
        g. Create resource.Resource{
            ID:     groupID,
            Name:   groupName,
            Status: "",   // SGs have no status field
            Fields: map[string]string{
                "group_id":    groupID,
                "group_name":  groupName,
                "vpc_id":      vpcID,
                "description": description,
            },
            DetailData: detail,
            RawJSON:    rawJSON,
            RawStruct:  sg,
        }
    3. Return resources, nil
```

**Key decisions**:
- `Resource.ID` = Security Group ID (the primary AWS identifier).
- `Resource.Name` = Group Name (the user-assigned name).
- `Resource.Status` = empty string. Security Groups do not have a status field. This is fine; the rendering layer handles empty status gracefully.
- Detail view shows rule **counts** only. Full rule detail is in YAML view (via `RawStruct` / `RawJSON`). This avoids the complexity of formatting `IpPermission` structs into `DetailData` strings.

### 5.3 `internal/aws/nodegroups.go`

**Pattern**: Three-step fetch. This is a new pattern, more complex than EKS clusters (two-step):
1. `ListClusters` to get cluster names (reuses existing `EKSListClustersAPI`)
2. `ListNodegroups` per cluster to get node group names
3. `DescribeNodegroup` per node group to get full details

```
Package: aws
Imports: context, encoding/json, fmt, strings, aws, eks, ekstypes, resource

init():
    resource.Register("nodegroups", func(ctx, clients) {
        c := clients.(*ServiceClients)
        return FetchNodeGroups(ctx, c.EKS, c.EKS, c.EKS)
    })

FetchNodeGroups(
    ctx context.Context,
    listClustersAPI EKSListClustersAPI,
    listNodegroupsAPI EKSListNodegroupsAPI,
    describeNodegroupAPI EKSDescribeNodegroupAPI,
) ([]resource.Resource, error):
    1. Call listClustersAPI.ListClusters(ctx, &eks.ListClustersInput{})
       - On error, return nil, err
    2. For each clusterName in listOutput.Clusters:
        a. Call listNodegroupsAPI.ListNodegroups(ctx, &eks.ListNodegroupsInput{
               ClusterName: aws.String(clusterName),
           })
           - On error, return nil, err
        b. For each ngName in ngListOutput.Nodegroups:
            i. Call describeNodegroupAPI.DescribeNodegroup(ctx, &eks.DescribeNodegroupInput{
                   ClusterName:   aws.String(clusterName),
                   NodegroupName: aws.String(ngName),
               })
               - On error, return nil, err
            ii. ng := descOutput.Nodegroup
            iii. Extract fields:
                - nodegroupName: *ng.NodegroupName (string)
                - clusterName: *ng.ClusterName (string)
                - status: string(ng.Status) (NodegroupStatus -> string)
                - instanceTypes: strings.Join(ng.InstanceTypes, ", ")
                - desiredSize: fmt.Sprintf("%d", *ng.ScalingConfig.DesiredSize)
                  (guard for nil ScalingConfig)
            iv. Build DetailData map[string]string with:
                - "Node Group Name", "Cluster Name", "Status"
                - "Instance Types", "AMI Type" (string(ng.AmiType))
                - "Capacity Type" (string(ng.CapacityType))
                - "Disk Size" (fmt *ng.DiskSize)
                - "Desired Size", "Min Size", "Max Size" (from ScalingConfig)
                - "Node Role" (from *ng.NodeRole)
                - "Node Group ARN" (from *ng.NodegroupArn)
                - "Release Version" (from *ng.ReleaseVersion)
                - "Kubernetes Version" (from *ng.Version)
                - "Subnets" (strings.Join(ng.Subnets, ", "))
                - "Created At" (ng.CreatedAt.Format(...))
                - All tags from ng.Tags as "Tag: <Key>" entries
                - Labels as "Label: <Key>" entries
            v. Build RawJSON via json.MarshalIndent(ng, "", "  ")
            vi. Create resource.Resource{
                ID:     nodegroupName,
                Name:   nodegroupName,
                Status: status,
                Fields: map[string]string{
                    "nodegroup_name": nodegroupName,
                    "cluster_name":   clusterName,
                    "status":         status,
                    "instance_types": instanceTypes,
                    "desired_size":   desiredSize,
                },
                DetailData: detail,
                RawJSON:    rawJSON,
                RawStruct:  ng,
            }
    3. Return resources, nil
```

**Key decisions**:
- `Resource.ID` = Node Group Name. This is not globally unique (two clusters can have same NG name), but it follows the pattern of EKS clusters where `ID = Name`. The combination of cluster_name + nodegroup_name in `Fields` provides full identification in the list view.
- `Resource.Name` = Node Group Name (same as ID; there's no separate display name).
- `Resource.Status` = NodegroupStatus string (e.g., "ACTIVE", "CREATING", "DEGRADED").
- The function takes **3 interface parameters** (listClusters, listNodegroups, describeNodegroup), each independently mockable. In production, all three are satisfied by `*eks.Client`.
- **No pagination** on ListClusters or ListNodegroups for v1. The existing EKS fetcher also does not paginate. If we later need pagination, it's an additive change to the fetcher logic, not an interface change.

---

## 6. Changes to `views.yaml`

Add these 3 sections after the existing `secrets:` section.

### 6.1 VPC

```yaml
  vpc:
    list:
      VPC ID:
        path: VpcId
        width: 24
      CIDR Block:
        path: CidrBlock
        width: 18
      State:
        path: State
        width: 12
      Default:
        path: IsDefault
        width: 9
    detail:
      - VpcId
      - CidrBlock
      - State
      - IsDefault
      - InstanceTenancy
      - DhcpOptionsId
      - OwnerId
      - CidrBlockAssociationSet
      - Ipv6CidrBlockAssociationSet
      - Tags
```

**Note**: No `Name` column in `views.yaml` list section. The Name is extracted from Tags in the fetcher and placed in `Fields["name"]`. The `views.yaml` list paths reference `RawStruct` field paths; there is no `Name` field on `ec2types.Vpc`. The fetcher-populated `Fields["name"]` is used by the list view's column rendering via the `Column.Key` mechanism, not via the `views.yaml` path mechanism. This is consistent with how EC2 instances handle the Name tag.

### 6.2 Security Groups

```yaml
  sg:
    list:
      Group ID:
        path: GroupId
        width: 24
      Group Name:
        path: GroupName
        width: 28
      VPC ID:
        path: VpcId
        width: 24
      Description:
        path: Description
        width: 36
    detail:
      - GroupId
      - GroupName
      - VpcId
      - Description
      - OwnerId
      - SecurityGroupArn
      - IpPermissions
      - IpPermissionsEgress
      - Tags
```

### 6.3 EKS Node Groups

```yaml
  nodegroups:
    list:
      Node Group:
        path: NodegroupName
        width: 28
      Cluster:
        path: ClusterName
        width: 24
      Status:
        path: Status
        width: 14
      Instance Types:
        path: InstanceTypes
        width: 20
      Desired:
        path: ScalingConfig.DesiredSize
        width: 9
    detail:
      - NodegroupName
      - ClusterName
      - Status
      - InstanceTypes
      - AmiType
      - CapacityType
      - DiskSize
      - ScalingConfig
      - NodeRole
      - NodegroupArn
      - ReleaseVersion
      - Version
      - Subnets
      - LaunchTemplate
      - Labels
      - Taints
      - Tags
      - Health
      - CreatedAt
```

---

## 7. Changes to `cmd/refgen/main.go`

Add 3 new entries to the `resources` slice to generate reference paths for `views_reference.yaml`:

```go
{"vpc", "ec2types.Vpc", reflect.TypeOf(ec2types.Vpc{})},
{"sg", "ec2types.SecurityGroup", reflect.TypeOf(ec2types.SecurityGroup{})},
{"nodegroups", "ekstypes.Nodegroup", reflect.TypeOf(ekstypes.Nodegroup{})},
```

No new imports needed — `ec2types` and `ekstypes` are already imported.

After implementation, run `go run ./cmd/refgen/ > views_reference.yaml` to regenerate.

---

## 8. New Test Files

### 8.1 New Mocks (add to `tests/unit/mocks_test.go`)

```go
// mockEC2DescribeVpcsClient implements awsclient.EC2DescribeVpcsAPI
type mockEC2DescribeVpcsClient struct {
    output *ec2.DescribeVpcsOutput
    err    error
}
func (m *mockEC2DescribeVpcsClient) DescribeVpcs(...) (*ec2.DescribeVpcsOutput, error)

// mockEC2DescribeSecurityGroupsClient implements awsclient.EC2DescribeSecurityGroupsAPI
type mockEC2DescribeSecurityGroupsClient struct {
    output *ec2.DescribeSecurityGroupsOutput
    err    error
}
func (m *mockEC2DescribeSecurityGroupsClient) DescribeSecurityGroups(...) (*ec2.DescribeSecurityGroupsOutput, error)

// mockEKSListNodegroupsClient implements awsclient.EKSListNodegroupsAPI
type mockEKSListNodegroupsClient struct {
    outputs map[string]*eks.ListNodegroupsOutput  // keyed by cluster name
    err     error
}
func (m *mockEKSListNodegroupsClient) ListNodegroups(...) (*eks.ListNodegroupsOutput, error)

// mockEKSDescribeNodegroupClient implements awsclient.EKSDescribeNodegroupAPI
type mockEKSDescribeNodegroupClient struct {
    outputs map[string]*eks.DescribeNodegroupOutput  // keyed by "cluster/nodegroup"
    err     error
}
func (m *mockEKSDescribeNodegroupClient) DescribeNodegroup(...) (*eks.DescribeNodegroupOutput, error)
```

**Note**: `mockEKSListNodegroupsClient.outputs` is keyed by cluster name because `ListNodegroups` takes a `ClusterName` parameter. `mockEKSDescribeNodegroupClient.outputs` is keyed by `"clusterName/nodegroupName"` compound key.

### 8.2 Test Files

Create 3 new test files following the exact pattern of `aws_ec2_test.go` and `aws_eks_test.go`:

**`tests/unit/aws_vpc_test.go`** — test cases:
1. `TestFetchVPCs_ParsesMultipleVPCs` — 2+ VPCs with varying fields (default/non-default, with/without Name tag)
2. `TestFetchVPCs_DetailDataPopulated` — verify DetailData contains expected keys
3. `TestFetchVPCs_ErrorResponse` — API returns error
4. `TestFetchVPCs_EmptyResponse` — no VPCs in region
5. `TestFetchVPCs_RawStructPopulated` — verify RawStruct is set for fieldpath

**`tests/unit/aws_sg_test.go`** — test cases:
1. `TestFetchSecurityGroups_ParsesMultipleGroups` — 2+ SGs with varying fields
2. `TestFetchSecurityGroups_DetailDataPopulated` — verify DetailData contains expected keys
3. `TestFetchSecurityGroups_ErrorResponse` — API returns error
4. `TestFetchSecurityGroups_EmptyResponse` — no SGs (unlikely but must handle)
5. `TestFetchSecurityGroups_RawStructPopulated` — verify RawStruct is set

**`tests/unit/aws_nodegroups_test.go`** — test cases:
1. `TestFetchNodeGroups_ParsesMultipleClustersAndGroups` — 2 clusters, each with 1+ node groups
2. `TestFetchNodeGroups_DetailDataPopulated` — verify ScalingConfig, AMI type, etc.
3. `TestFetchNodeGroups_ListClustersError` — first API call fails
4. `TestFetchNodeGroups_ListNodegroupsError` — second API call fails
5. `TestFetchNodeGroups_DescribeNodegroupError` — third API call fails
6. `TestFetchNodeGroups_EmptyClusters` — no clusters exist
7. `TestFetchNodeGroups_ClustersButNoNodeGroups` — clusters exist but no managed NGs
8. `TestFetchNodeGroups_RawStructPopulated` — verify RawStruct is set

### 8.3 Existing Test Updates

**`tests/unit/qa_registry_test.go`** — must be updated to expect 10 registered resource types (was 7). Add `"vpc"`, `"sg"`, `"nodegroups"` to the expected list.

**`tests/unit/qa_mainmenu_test.go`** — if it validates main menu items, must be updated to include the 3 new entries.

**`tests/unit/column_key_mismatch_test.go`** — this test validates that every Column.Key in ResourceTypeDef has a corresponding Fields entry in fetcher output. The 3 new resource types are automatically covered if the test iterates `AllResourceTypes()`.

---

## 9. Implementation Order

Strictly TDD — tests first, then implementation.

1. **Phase 1: VPC** (P1, simplest — single-step EC2 fetch)
   - Add VPC `ResourceTypeDef` to `types.go`
   - Add `EC2DescribeVpcsAPI` interface to `interfaces.go`
   - Add mock to `mocks_test.go`
   - Write `aws_vpc_test.go` (tests fail)
   - Create `internal/aws/vpc.go` (tests pass)
   - Add VPC section to `views.yaml`
   - Update `cmd/refgen/main.go`, regenerate `views_reference.yaml`
   - Update registry/mainmenu tests

2. **Phase 2: Security Groups** (P1, single-step EC2 fetch)
   - Add SG `ResourceTypeDef` to `types.go`
   - Add `EC2DescribeSecurityGroupsAPI` interface to `interfaces.go`
   - Add mock to `mocks_test.go`
   - Write `aws_sg_test.go` (tests fail)
   - Create `internal/aws/sg.go` (tests pass)
   - Add SG section to `views.yaml`
   - Update `cmd/refgen/main.go`, regenerate `views_reference.yaml`
   - Update registry/mainmenu tests

3. **Phase 3: EKS Node Groups** (P2, three-step EKS fetch)
   - Add Node Groups `ResourceTypeDef` to `types.go`
   - Add `EKSListNodegroupsAPI` and `EKSDescribeNodegroupAPI` interfaces to `interfaces.go`
   - Add mocks to `mocks_test.go`
   - Write `aws_nodegroups_test.go` (tests fail)
   - Create `internal/aws/nodegroups.go` (tests pass)
   - Add Node Groups section to `views.yaml`
   - Update `cmd/refgen/main.go`, regenerate `views_reference.yaml`
   - Update registry/mainmenu tests

4. **Phase 4: Version bump and full test run**
   - Bump version in `cmd/a9s/main.go`
   - Run `go test ./tests/unit/ -count=1 -timeout 120s` — all 1,045+ tests pass plus new ones
   - Rebuild binary: `go build -o a9s ./cmd/a9s/`

---

## 10. AWS SDK Struct Field Reference

For the implementer's convenience, these are the exact AWS SDK Go v2 struct fields.

### `ec2types.Vpc` fields

| Field | Go Type | Notes |
|-------|---------|-------|
| `BlockPublicAccessStates` | `*BlockPublicAccessStates` | |
| `CidrBlock` | `*string` | Primary IPv4 CIDR |
| `CidrBlockAssociationSet` | `[]VpcCidrBlockAssociation` | Additional CIDRs |
| `DhcpOptionsId` | `*string` | |
| `EncryptionControl` | `*VpcEncryptionControl` | |
| `InstanceTenancy` | `Tenancy` (string enum) | `default`, `dedicated`, `host` |
| `Ipv6CidrBlockAssociationSet` | `[]VpcIpv6CidrBlockAssociation` | |
| `IsDefault` | `*bool` | |
| `OwnerId` | `*string` | |
| `State` | `VpcState` (string enum) | `available`, `pending` |
| `Tags` | `[]Tag` | Tag.Key, Tag.Value are `*string` |
| `VpcId` | `*string` | |

### `ec2types.SecurityGroup` fields

| Field | Go Type | Notes |
|-------|---------|-------|
| `Description` | `*string` | |
| `GroupId` | `*string` | |
| `GroupName` | `*string` | |
| `IpPermissions` | `[]IpPermission` | Inbound rules |
| `IpPermissionsEgress` | `[]IpPermission` | Outbound rules |
| `OwnerId` | `*string` | |
| `SecurityGroupArn` | `*string` | |
| `Tags` | `[]Tag` | |
| `VpcId` | `*string` | |

### `ekstypes.Nodegroup` fields

| Field | Go Type | Notes |
|-------|---------|-------|
| `AmiType` | `AMITypes` (string enum) | e.g., `AL2_x86_64` |
| `CapacityType` | `CapacityTypes` (string enum) | `ON_DEMAND`, `SPOT`, `CAPACITY_BLOCK` |
| `ClusterName` | `*string` | |
| `CreatedAt` | `*time.Time` | |
| `DiskSize` | `*int32` | In GiB; nil if launch template used |
| `Health` | `*NodegroupHealth` | Contains `Issues []Issue` |
| `InstanceTypes` | `[]string` | e.g., `["t3.medium"]` |
| `Labels` | `map[string]string` | Kubernetes labels |
| `LaunchTemplate` | `*LaunchTemplateSpecification` | Id, Name, Version fields |
| `ModifiedAt` | `*time.Time` | |
| `NodeRepairConfig` | `*NodeRepairConfig` | |
| `NodeRole` | `*string` | IAM role ARN |
| `NodegroupArn` | `*string` | |
| `NodegroupName` | `*string` | |
| `ReleaseVersion` | `*string` | AMI version |
| `RemoteAccess` | `*RemoteAccessConfig` | |
| `Resources` | `*NodegroupResources` | ASG and SG references |
| `ScalingConfig` | `*NodegroupScalingConfig` | DesiredSize, MinSize, MaxSize (`*int32`) |
| `Status` | `NodegroupStatus` (string enum) | `ACTIVE`, `CREATING`, `UPDATING`, `DELETING`, `DEGRADED`, etc. |
| `Subnets` | `[]string` | |
| `Tags` | `map[string]string` | |
| `Taints` | `[]Taint` | Effect, Key, Value |
| `UpdateConfig` | `*NodegroupUpdateConfig` | |
| `Version` | `*string` | Kubernetes version |

---

## 11. Dependency Graph Verification

All new code follows the existing dependency graph:

```
internal/aws/vpc.go        -> ec2, ec2types, resource     (same as ec2.go)
internal/aws/sg.go         -> ec2, ec2types, resource     (same as ec2.go)
internal/aws/nodegroups.go -> eks, ekstypes, resource      (same as eks.go, plus aws for pointer helpers)
tests/unit/*_test.go       -> ec2, ec2types, eks, ekstypes, awsclient  (same as existing tests)
```

No new dependencies introduced. No circular imports. No imports from `internal/tui/` or old `internal/app/`.
