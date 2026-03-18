# QA-09: YAML View Across All Resource Types

Covers the YAML view for every resource type supported by a9s. The YAML view
displays the full AWS API response for a single resource as syntax-colored YAML,
equivalent to what `aws <service> describe-<resource> --output yaml` returns.

---

## Entry Points

| From            | Key | Result                                 |
|-----------------|-----|----------------------------------------|
| Resource list   | `y` | Opens YAML view for the selected row   |
| Detail view     | `y` | Switches from detail to YAML view      |

Pressing `esc` in the YAML view returns to whichever view the user came from
(detail view or resource list).

---

## Frame Title Format

The frame title embeds the resource identifier followed by the literal word
"yaml", centered in the top border:

```
+---------------- <resource-id> yaml ----------------+
```

Examples per resource type:

| Resource       | Frame title example                                   |
|----------------|-------------------------------------------------------|
| S3             | `my-data-bucket yaml`                                 |
| EC2            | `i-0abc123def456789a yaml`                            |
| RDS            | `mydb-prod yaml`                                      |
| Redis          | `redis-cluster-01 yaml`                               |
| DocumentDB     | `docdb-prod-cluster yaml`                             |
| EKS            | `my-eks-cluster yaml`                                 |
| Secrets        | `prod/api/database-password yaml`                     |

---

## Syntax Coloring Rules

Every YAML token is colored according to its type. No background color; all
coloring is foreground-only on the default terminal background.

| Token type      | Color hex | Color name | Style | Examples                              |
|-----------------|-----------|------------|-------|---------------------------------------|
| Key             | `#7aa2f7` | Blue       | ---   | `InstanceId:`, `Engine:`, `Name:`     |
| String value    | `#9ece6a` | Green      | ---   | `t3.medium`, `us-east-1a`, `running`  |
| Numeric value   | `#ff9e64` | Orange     | ---   | `100`, `5432`, `3.14`, `0`            |
| Boolean value   | `#bb9af7` | Purple     | ---   | `true`, `false`                       |
| Null value      | `#565f89` | Dim gray   | Dim   | `null`, `~`                           |
| Indent line     | `#414868` | Dark gray  | Dim   | Vertical tree connector characters    |

### Coloring verification checklist

For each resource type below, verify the named fields render in the correct
color.

**Booleans (purple `#bb9af7`):**
- RDS: `MultiAZ: true`, `StorageEncrypted: true`
- Redis: (none standard; check if any boolean fields appear)
- DocumentDB: `StorageEncrypted: true`
- Secrets: `RotationEnabled: true` / `RotationEnabled: false`
- EC2: `EbsOptimized: true`, `SourceDestCheck: true`,
  `BlockDeviceMappings[].Ebs.DeleteOnTermination: true`

**Numbers (orange `#ff9e64`):**
- RDS: `AllocatedStorage: 100`, `Port: 5432`
- Redis: `NumCacheNodes: 3`
- DocumentDB: `Port: 27017`
- EC2: `AmiLaunchIndex: 0`
- S3: (none in bucket-level describe)

**Nulls (dim `#565f89`):**
- EC2: `PublicIpAddress: null` (when instance has no public IP)
- RDS: fields that are not set return `null`
- Any optional field not populated by the AWS API

**Strings (green `#9ece6a`):**
- All identifiers, ARNs, timestamps, IP addresses, status values
- Timestamps rendered as ISO 8601 strings (e.g., `2024-01-15T09:22:31Z`)

---

## YAML Structure Rules

### Indentation

All nesting uses exactly 2-space indentation, matching standard YAML convention
and the output of `aws ... --output yaml`.

```
TopLevelKey:
  NestedKey:
    DeeplyNestedKey: value
```

### Indent Connectors

Vertical tree connector characters (`|`) appear in dim gray (`#414868`) at the
left edge of nested blocks to visually connect parent/child relationships.

### Arrays

Array items use the standard YAML `- ` prefix (dash followed by a space).
Subsequent keys on the same item are indented to align with the first key.

```
SecurityGroups:
  - GroupId: sg-0abc123
    GroupName: my-security-group
  - GroupId: sg-0def456
    GroupName: another-group
```

### Empty Arrays

Empty arrays render as inline `[]`:

```
Tags: []
```

### Null Fields

Null or unset fields render as the literal word `null` in dim color:

```
PublicIpAddress: null
```

---

## Keyboard Controls in YAML View

| Key            | Action                                          |
|----------------|-------------------------------------------------|
| `j` / Down     | Scroll down one line                            |
| `k` / Up       | Scroll up one line                              |
| `g`            | Jump to top of YAML                             |
| `G`            | Jump to bottom of YAML                          |
| `PageUp`       | Scroll up one page                              |
| `PageDown`     | Scroll down one page                            |
| `w`            | Toggle word wrap for long values                |
| `c`            | Copy entire YAML to clipboard (plain, uncolored)|
| `esc`          | Return to previous view                         |
| `?`            | Open help screen                                |

---

## Scroll Behavior

The YAML view uses the bubbles viewport component. When YAML content exceeds the
visible frame area:

- Scrolling with `j`/`k` moves one line at a time
- `g` jumps to line 1; `G` jumps to the last line
- `PageUp`/`PageDown` scroll by one screenful
- Scroll position is indicated by a dim scroll indicator (`#414868`) showing
  how many lines are above or below the visible area (e.g., "12 lines above")

---

## Wrap Toggle (`w`)

Pressing `w` toggles word wrap on and off.

| State    | Behavior                                                      |
|----------|---------------------------------------------------------------|
| Wrap off | Long values (ARNs, endpoints, base64 data) extend beyond the visible frame width; horizontal content is clipped |
| Wrap on  | Long values break at the frame boundary and continue on the next line, indented to align with the value column |

Values that typically need wrapping:
- ARNs (e.g., `arn:aws:rds:us-east-1:123456789012:db:mydb-prod`)
- Endpoint URLs (e.g., `mydb-prod.c9abcdef.us-east-1.rds.amazonaws.com`)
- EKS API server endpoints (e.g., `https://ABCDEF1234.gr7.us-east-1.eks.amazonaws.com`)
- KMS key IDs
- Base64-encoded certificate data (EKS `CertificateAuthority.Data`)

---

## Copy (`c`)

Pressing `c` copies the entire YAML document to the system clipboard.

| Aspect         | Behavior                                                   |
|----------------|------------------------------------------------------------|
| Content        | Full YAML text, all lines, regardless of scroll position   |
| Formatting     | Plain text only; all color/style codes are stripped         |
| Feedback       | Header right side shows "Copied!" in green (`#9ece6a`) bold for approximately 2 seconds, then reverts to "? for help" |
| Failure        | If clipboard is unavailable, header shows error flash in red (`#f7768e`) |

---

## Resource-Specific YAML Stories

### US-S3: S3 Bucket YAML

**AWS CLI equivalent:** `aws s3api head-bucket --bucket X` (limited) or S3 API describe

**Precondition:** User is on the S3 resource list with at least one bucket visible.

**Steps:**
1. Select a bucket in the list
2. Press `y`

**Expected YAML output structure:**
```
Name: my-data-bucket
CreationDate: 2023-06-15T10:30:00Z
BucketArn: arn:aws:s3:::my-data-bucket
BucketRegion: us-east-1
```

**Coloring verification:**
- `Name:` -- blue key, `my-data-bucket` -- green string
- `CreationDate:` -- blue key, `2023-06-15T10:30:00Z` -- green string (ISO timestamp is a string)
- `BucketArn:` -- blue key, long ARN string -- green string
- `BucketRegion:` -- blue key, `us-east-1` -- green string

**Frame title:** `my-data-bucket yaml`

**Edge cases:**
- S3 bucket YAML is typically short (under 10 lines); no scrolling needed
- Bucket ARN can be long but rarely exceeds frame width

---

### US-EC2: EC2 Instance YAML

**AWS CLI equivalent:** `aws ec2 describe-instances --instance-ids i-xxx --output yaml`

**Precondition:** User is on the EC2 resource list with at least one instance visible.

**Steps:**
1. Select an instance in the list
2. Press `y`

**Expected YAML output structure (representative fields):**
```
AmiLaunchIndex: 0
Architecture: x86_64
BlockDeviceMappings:
  - DeviceName: /dev/xvda
    Ebs:
      AttachTime: 2024-01-15T09:22:45Z
      DeleteOnTermination: true
      Status: attached
      VolumeId: vol-0abc123def456
EbsOptimized: false
ImageId: ami-0abcdef01234567
InstanceId: i-0abc123def456789a
InstanceType: t3.medium
LaunchTime: 2024-01-15T09:22:31Z
Placement:
  AvailabilityZone: us-east-1a
  Tenancy: default
PrivateIpAddress: 10.0.1.42
PublicIpAddress: 54.123.45.67
SecurityGroups:
  - GroupId: sg-0abc123
    GroupName: my-security-group
State:
  Code: 16
  Name: running
Tags:
  - Key: Name
    Value: api-prod-01
  - Key: Environment
    Value: production
VpcId: vpc-0123456789abcdef0
SubnetId: subnet-0123456789abcde
```

**Coloring verification:**
- `AmiLaunchIndex: 0` -- blue key, orange number
- `DeleteOnTermination: true` -- blue key, purple boolean
- `EbsOptimized: false` -- blue key, purple boolean
- `InstanceType: t3.medium` -- blue key, green string
- `Code: 16` -- blue key, orange number
- Instance with no public IP: `PublicIpAddress: null` -- blue key, dim null
- All timestamps (e.g., `LaunchTime`, `AttachTime`) -- green strings

**Frame title:** `i-0abc123def456789a yaml`

**Nesting verification:**
- `BlockDeviceMappings` is an array; items use `- ` prefix
- `Ebs` is a nested object under each block device; indented 4 spaces from array item
- `Placement` is a nested object; `AvailabilityZone` is indented 2 spaces under it
- `State` is a nested object; `Code` and `Name` are indented 2 spaces under it
- `SecurityGroups` is an array of objects with `GroupId` and `GroupName`
- `Tags` is an array of objects with `Key` and `Value`
- Vertical dim tree connectors appear along the left edge of nested blocks

**Edge cases:**
- EC2 YAML is typically 50-150+ lines; scroll must work
- Terminated instances have many null fields
- Instances with many tags or security groups produce longer YAML
- Tags array can be empty: `Tags: []`

**Scroll test:**
1. Open YAML for an instance with many tags/security groups (100+ lines)
2. Press `G` -- viewport jumps to bottom, last line visible
3. Press `g` -- viewport jumps back to top, first line visible
4. Press `PageDown` repeatedly -- scrolls one page at a time
5. Press `j` repeatedly -- scrolls one line at a time

---

### US-RDS: RDS Instance YAML

**AWS CLI equivalent:** `aws rds describe-db-instances --db-instance-identifier X --output yaml`

**Precondition:** User is on the RDS resource list with at least one instance visible.

**Steps:**
1. Select an RDS instance in the list
2. Press `y`

**Expected YAML output structure (representative fields):**
```
DBInstanceIdentifier: mydb-prod
DBInstanceClass: db.r5.large
Engine: postgres
EngineVersion: 15.4
DBInstanceStatus: available
Endpoint:
  Address: mydb-prod.c9abcdef.us-east-1.rds.amazonaws.com
  Port: 5432
  HostedZoneId: Z2R2ITUGPM61AM
AllocatedStorage: 100
StorageType: gp3
MultiAZ: true
StorageEncrypted: true
AvailabilityZone: us-east-1a
DBInstanceArn: arn:aws:rds:us-east-1:123456789012:db:mydb-prod
VpcSecurityGroups:
  - VpcSecurityGroupId: sg-0abc123
    Status: active
DBSubnetGroup:
  DBSubnetGroupName: my-subnet-group
  Subnets:
    - SubnetIdentifier: subnet-0abc
      SubnetAvailabilityZone:
        Name: us-east-1a
    - SubnetIdentifier: subnet-0def
      SubnetAvailabilityZone:
        Name: us-east-1b
```

**Coloring verification:**
- `AllocatedStorage: 100` -- blue key, orange number
- `Port: 5432` -- blue key, orange number
- `MultiAZ: true` -- blue key, purple boolean
- `StorageEncrypted: true` -- blue key, purple boolean
- `DBInstanceStatus: available` -- blue key, green string
- `DBInstanceArn:` -- blue key, long ARN green string

**Frame title:** `mydb-prod yaml`

**Nesting verification:**
- `Endpoint` is a nested object; `Address`, `Port`, `HostedZoneId` indented 2 spaces
- `VpcSecurityGroups` is an array of objects
- `DBSubnetGroup.Subnets` is a nested array inside a nested object (3 levels deep)

**Edge cases:**
- RDS YAML is typically 80-200+ lines; scroll required
- `Endpoint.Address` is a long hostname (may need wrap toggle)
- `DBInstanceArn` is a long string (may need wrap toggle)
- Single-AZ instances: `MultiAZ: false` (purple boolean)

**Wrap test:**
1. Open YAML for an RDS instance
2. Find the `Endpoint.Address` line -- long hostname may be clipped
3. Press `w` -- value wraps to next line
4. Press `w` again -- wrap off, value clipped again

---

### US-REDIS: ElastiCache Redis YAML

**AWS CLI equivalent:** `aws elasticache describe-cache-clusters --cache-cluster-id X --output yaml`

**Precondition:** User is on the Redis resource list with at least one cluster visible.

**Steps:**
1. Select a Redis cluster in the list
2. Press `y`

**Expected YAML output structure (representative fields):**
```
CacheClusterId: redis-cluster-01
Engine: redis
EngineVersion: 7.0.12
CacheClusterStatus: available
CacheNodeType: cache.r6g.large
NumCacheNodes: 3
ConfigurationEndpoint:
  Address: redis-cluster-01.abc123.clustercfg.use1.cache.amazonaws.com
  Port: 6379
PreferredAvailabilityZone: us-east-1a
CacheClusterCreateTime: 2024-02-10T14:00:00Z
CacheNodes:
  - CacheNodeId: "0001"
    CacheNodeStatus: available
    Endpoint:
      Address: redis-cluster-01-001.abc123.0001.use1.cache.amazonaws.com
      Port: 6379
  - CacheNodeId: "0002"
    CacheNodeStatus: available
    Endpoint:
      Address: redis-cluster-01-002.abc123.0002.use1.cache.amazonaws.com
      Port: 6379
CacheParameterGroup:
  CacheParameterGroupName: default.redis7
  ParameterApplyStatus: in-sync
SecurityGroups:
  - SecurityGroupId: sg-0abc123
    Status: active
ARN: arn:aws:elasticache:us-east-1:123456789012:cluster:redis-cluster-01
```

**Coloring verification:**
- `NumCacheNodes: 3` -- blue key, orange number
- `Port: 6379` -- blue key, orange number
- `CacheClusterStatus: available` -- blue key, green string
- `CacheNodeId: "0001"` -- blue key, green string (quoted numeric string)

**Frame title:** `redis-cluster-01 yaml`

**Nesting verification:**
- `ConfigurationEndpoint` is a nested object with `Address` and `Port`
- `CacheNodes` is an array of objects, each containing a nested `Endpoint` object
- `CacheParameterGroup` is a nested object
- `SecurityGroups` is an array of objects

**Edge cases:**
- Redis endpoint addresses are very long; wrap toggle is useful
- Clusters with many nodes produce longer YAML
- `ConfigurationEndpoint` may be `null` for non-cluster-mode clusters

---

### US-DOCDB: DocumentDB Cluster YAML

**AWS CLI equivalent:** `aws docdb describe-db-clusters --db-cluster-identifier X --output yaml`

**Precondition:** User is on the DocumentDB resource list with at least one cluster visible.

**Steps:**
1. Select a DocumentDB cluster in the list
2. Press `y`

**Expected YAML output structure (representative fields):**
```
DBClusterIdentifier: docdb-prod-cluster
Engine: docdb
EngineVersion: 5.0.0
Status: available
Endpoint: docdb-prod-cluster.cluster-c9abcdef.us-east-1.docdb.amazonaws.com
ReaderEndpoint: docdb-prod-cluster.cluster-ro-c9abcdef.us-east-1.docdb.amazonaws.com
Port: 27017
StorageEncrypted: true
DBClusterMembers:
  - DBInstanceIdentifier: docdb-prod-instance-1
    IsClusterWriter: true
    DBClusterParameterGroupStatus: in-sync
  - DBInstanceIdentifier: docdb-prod-instance-2
    IsClusterWriter: false
    DBClusterParameterGroupStatus: in-sync
VpcSecurityGroups:
  - VpcSecurityGroupId: sg-0abc123
    Status: active
DBClusterArn: arn:aws:rds:us-east-1:123456789012:cluster:docdb-prod-cluster
AssociatedRoles: []
AvailabilityZones:
  - us-east-1a
  - us-east-1b
  - us-east-1c
```

**Coloring verification:**
- `Port: 27017` -- blue key, orange number
- `StorageEncrypted: true` -- blue key, purple boolean
- `IsClusterWriter: true` -- blue key, purple boolean
- `IsClusterWriter: false` -- blue key, purple boolean
- `Status: available` -- blue key, green string
- `AssociatedRoles: []` -- blue key, dim or plain empty array

**Frame title:** `docdb-prod-cluster yaml`

**Nesting verification:**
- `DBClusterMembers` is an array of objects with nested fields
- `VpcSecurityGroups` is an array of objects
- `AvailabilityZones` is a simple array of strings (each prefixed with `- `)
- `AssociatedRoles` is an empty array rendered as `[]`

**Edge cases:**
- `Endpoint` and `ReaderEndpoint` are very long hostnames; wrap toggle needed
- `DBClusterArn` is a long ARN string
- Empty `AssociatedRoles: []` must render as inline empty array, not omitted
- Clusters with many members produce longer `DBClusterMembers` arrays

---

### US-EKS: EKS Cluster YAML

**AWS CLI equivalent:** `aws eks describe-cluster --name X --output yaml`

**Precondition:** User is on the EKS resource list with at least one cluster visible.

**Steps:**
1. Select an EKS cluster in the list
2. Press `y`

**Expected YAML output structure (representative fields):**
```
Name: my-eks-cluster
Version: "1.28"
Status: ACTIVE
Endpoint: https://ABCDEF1234567890.gr7.us-east-1.eks.amazonaws.com
Arn: arn:aws:eks:us-east-1:123456789012:cluster/my-eks-cluster
RoleArn: arn:aws:iam::123456789012:role/eks-cluster-role
PlatformVersion: eks.8
CertificateAuthority:
  Data: LS0tLS1CRUdJTi... (very long base64 string)
KubernetesNetworkConfig:
  ServiceIpv4Cidr: 172.20.0.0/16
  IpFamily: ipv4
ResourcesVpcConfig:
  SubnetIds:
    - subnet-0abc123
    - subnet-0def456
  SecurityGroupIds:
    - sg-0abc123
  ClusterSecurityGroupId: sg-0xyz789
  VpcId: vpc-0123456789abcdef0
  EndpointPublicAccess: true
  EndpointPrivateAccess: true
  PublicAccessCidrs:
    - 0.0.0.0/0
Logging:
  ClusterLogging:
    - Types:
        - api
        - audit
      Enabled: true
Tags:
  Environment: production
  Team: platform
CreatedAt: 2024-01-10T08:00:00Z
```

**Coloring verification:**
- `Version: "1.28"` -- blue key, green string (quoted)
- `Status: ACTIVE` -- blue key, green string
- `EndpointPublicAccess: true` -- blue key, purple boolean
- `EndpointPrivateAccess: true` -- blue key, purple boolean
- `Enabled: true` -- blue key, purple boolean

**Frame title:** `my-eks-cluster yaml`

**Nesting verification:**
- `CertificateAuthority.Data` is deeply nested
- `KubernetesNetworkConfig` is a nested object
- `ResourcesVpcConfig` contains nested arrays (`SubnetIds`, `SecurityGroupIds`, `PublicAccessCidrs`)
- `Logging.ClusterLogging` is an array containing objects with nested arrays (`Types`)
- `Tags` may render as a map (key-value pairs) rather than an array

**Edge cases:**
- `CertificateAuthority.Data` is an extremely long base64 string (hundreds of characters); wrap toggle is essential
- `Endpoint` is a long HTTPS URL; may need wrapping
- `Arn` and `RoleArn` are long ARN strings
- EKS YAML is typically 60-120+ lines; scroll required
- `Tags` may be a map structure rather than a list of Key/Value pairs depending on the SDK response format

**Wrap test (critical for EKS):**
1. Open YAML for an EKS cluster
2. Scroll to `CertificateAuthority.Data` -- base64 string extends far beyond frame
3. With wrap off, the string is clipped at the frame boundary
4. Press `w` -- base64 string wraps across multiple lines
5. Verify indentation of wrapped continuation lines aligns with the value column

---

### US-SECRETS: Secrets Manager YAML

**AWS CLI equivalent:** `aws secretsmanager describe-secret --secret-id X --output yaml`

**Precondition:** User is on the Secrets Manager resource list with at least one secret visible.

**Steps:**
1. Select a secret in the list
2. Press `y`

**Expected YAML output structure (representative fields):**
```
Name: prod/api/database-password
Description: Production database credentials
ARN: arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/api/database-password-AbCdEf
KmsKeyId: arn:aws:kms:us-east-1:123456789012:key/12345678-1234-1234-1234-123456789012
RotationEnabled: true
RotationRules:
  AutomaticallyAfterDays: 30
LastRotatedDate: 2024-01-10T14:23:00Z
LastChangedDate: 2024-01-10T14:23:00Z
LastAccessedDate: 2024-03-17T00:00:00Z
VersionIdsToStages:
  AWSCURRENT:
    - "v1"
  AWSPREVIOUS:
    - "v0"
Tags:
  - Key: Environment
    Value: production
  - Key: Team
    Value: platform
CreatedDate: 2023-06-01T12:00:00Z
```

**Coloring verification:**
- `RotationEnabled: true` -- blue key, purple boolean
- `RotationEnabled: false` (for secrets without rotation) -- blue key, purple boolean
- `AutomaticallyAfterDays: 30` -- blue key, orange number
- `Name: prod/api/database-password` -- blue key, green string (contains `/` special chars)
- `Description: null` (when no description set) -- blue key, dim null

**Frame title:** `prod/api/database-password yaml`

**Nesting verification:**
- `RotationRules` is a nested object
- `VersionIdsToStages` is a map with array values
- `Tags` is an array of Key/Value objects

**Edge cases:**
- `ARN` includes a random 6-character suffix; long string
- `KmsKeyId` is a long ARN string
- Secrets without rotation: `RotationEnabled: false`, `RotationRules: null`, `LastRotatedDate: null`
- Secrets with no description: `Description: null` (dim)
- Secrets with no tags: `Tags: []`
- Secret names with special characters (slashes, dots): rendered as-is in green

---

## Cross-Resource Test Scenarios

### TC-YAML-01: Copy YAML to Clipboard

| Step | Action                                         | Expected                                        |
|------|-------------------------------------------------|-------------------------------------------------|
| 1    | Open YAML view for any resource                 | YAML content displayed with syntax coloring     |
| 2    | Press `c`                                       | Header right shows "Copied!" in green bold      |
| 3    | Paste into a text editor                        | Full YAML text appears, no color codes, no ANSI escape sequences |
| 4    | Wait approximately 2 seconds                    | "Copied!" reverts to "? for help"               |

### TC-YAML-02: Scroll Large YAML

| Step | Action                                         | Expected                                        |
|------|-------------------------------------------------|-------------------------------------------------|
| 1    | Open YAML view for EC2 instance (100+ lines)   | First lines visible, content extends below      |
| 2    | Press `j` five times                            | Scrolls down 5 lines                            |
| 3    | Press `k` two times                             | Scrolls up 2 lines (net 3 lines down from top)  |
| 4    | Press `G`                                       | Jumps to last line of YAML                      |
| 5    | Press `g`                                       | Jumps back to first line                        |
| 6    | Press `PageDown`                                | Scrolls down one full page                      |
| 7    | Press `PageUp`                                  | Scrolls back up one full page                   |

### TC-YAML-03: Wrap Toggle

| Step | Action                                         | Expected                                        |
|------|-------------------------------------------------|-------------------------------------------------|
| 1    | Open YAML for EKS cluster                       | YAML displayed, long values clipped at right    |
| 2    | Scroll to `CertificateAuthority.Data`           | Base64 string clipped at frame boundary         |
| 3    | Press `w`                                       | Long value wraps; continuation lines indented   |
| 4    | Press `w` again                                 | Wrap off; value clipped at frame boundary again |

### TC-YAML-04: Empty Arrays

| Step | Action                                         | Expected                                        |
|------|-------------------------------------------------|-------------------------------------------------|
| 1    | Open YAML for a resource with empty arrays      | e.g., DocumentDB with `AssociatedRoles: []`     |
| 2    | Verify empty array field                        | Renders as `[]` on the same line as the key     |

### TC-YAML-05: Null Fields

| Step | Action                                         | Expected                                        |
|------|-------------------------------------------------|-------------------------------------------------|
| 1    | Open YAML for EC2 instance without public IP    | `PublicIpAddress: null`                         |
| 2    | Verify null rendering                           | `null` in dim gray (`#565f89`)                  |

### TC-YAML-06: Navigate from Detail to YAML

| Step | Action                                         | Expected                                        |
|------|-------------------------------------------------|-------------------------------------------------|
| 1    | From resource list, press `d` or `enter`        | Detail view opens                               |
| 2    | Press `y`                                       | YAML view opens, showing full resource YAML     |
| 3    | Press `esc`                                     | Returns to detail view (not resource list)       |

### TC-YAML-07: Navigate from Resource List to YAML

| Step | Action                                         | Expected                                        |
|------|-------------------------------------------------|-------------------------------------------------|
| 1    | From resource list, press `y`                   | YAML view opens directly                        |
| 2    | Press `esc`                                     | Returns to resource list (not detail view)       |

### TC-YAML-08: Help from YAML View

| Step | Action                                         | Expected                                        |
|------|-------------------------------------------------|-------------------------------------------------|
| 1    | Open YAML view for any resource                 | YAML content displayed                          |
| 2    | Press `?`                                       | Help screen replaces content                    |
| 3    | Press any key                                   | Returns to YAML view, scroll position preserved |

### TC-YAML-09: Special Characters in Values

| Step | Action                                         | Expected                                        |
|------|-------------------------------------------------|-------------------------------------------------|
| 1    | Open YAML for a secret with `/` in its name     | e.g., `prod/api/database-password`              |
| 2    | Verify the value renders correctly              | `Name: prod/api/database-password` -- green     |
| 3    | Check ARN with colons and slashes               | Full ARN renders as single green string          |

### TC-YAML-10: Timestamps as Strings

| Step | Action                                         | Expected                                        |
|------|-------------------------------------------------|-------------------------------------------------|
| 1    | Open YAML for any resource with timestamp fields | e.g., EC2 `LaunchTime`, RDS `InstanceCreateTime`|
| 2    | Verify timestamp coloring                       | ISO 8601 string in green (`#9ece6a`), not orange |

### TC-YAML-11: Deeply Nested YAML

| Step | Action                                         | Expected                                        |
|------|-------------------------------------------------|-------------------------------------------------|
| 1    | Open YAML for RDS instance                      | `DBSubnetGroup.Subnets[].SubnetAvailabilityZone.Name` is 4 levels deep |
| 2    | Verify indentation                              | Each level adds exactly 2 spaces                |
| 3    | Verify tree connectors                          | Dim vertical lines at left edge of nested blocks|

### TC-YAML-12: Boolean Values Across Resources

| Step | Action                                         | Expected                                        |
|------|-------------------------------------------------|-------------------------------------------------|
| 1    | Open YAML for RDS instance                      | `MultiAZ: true` -- purple                       |
| 2    | Open YAML for single-AZ RDS instance            | `MultiAZ: false` -- purple                      |
| 3    | Open YAML for Secrets with rotation              | `RotationEnabled: true` -- purple               |
| 4    | Open YAML for Secrets without rotation           | `RotationEnabled: false` -- purple              |
| 5    | Open YAML for DocumentDB cluster                 | `StorageEncrypted: true` -- purple              |
| 6    | Open YAML for EC2 instance                       | `EbsOptimized: false` -- purple                 |

### TC-YAML-13: Numeric Values Across Resources

| Step | Action                                         | Expected                                        |
|------|-------------------------------------------------|-------------------------------------------------|
| 1    | Open YAML for RDS instance                      | `AllocatedStorage: 100` -- orange               |
| 2    | Verify port number                              | `Port: 5432` -- orange                          |
| 3    | Open YAML for Redis cluster                     | `NumCacheNodes: 3` -- orange                    |
| 4    | Open YAML for DocumentDB cluster                | `Port: 27017` -- orange                         |
| 5    | Open YAML for EC2 instance                      | `AmiLaunchIndex: 0` -- orange                   |

---

## AWS CLI Comparison Reference

For each resource type, the YAML view content should closely match the output of
the corresponding AWS CLI command with `--output yaml`. Differences may exist in
field ordering (a9s may alphabetize keys) and in how SDK struct fields are named
versus the CLI JSON-to-YAML conversion.

| Resource   | AWS CLI command                                                              |
|------------|------------------------------------------------------------------------------|
| S3         | `aws s3api head-bucket --bucket X` (limited) or S3 control/API describe      |
| EC2        | `aws ec2 describe-instances --instance-ids i-xxx --output yaml`              |
| RDS        | `aws rds describe-db-instances --db-instance-identifier X --output yaml`     |
| Redis      | `aws elasticache describe-cache-clusters --cache-cluster-id X --output yaml` |
| DocumentDB | `aws docdb describe-db-clusters --db-cluster-identifier X --output yaml`     |
| EKS        | `aws eks describe-cluster --name X --output yaml`                            |
| Secrets    | `aws secretsmanager describe-secret --secret-id X --output yaml`             |

**Key comparison points:**
- Field names should match the AWS SDK Go struct field names (PascalCase), which
  may differ slightly from the CLI output (also PascalCase but from JSON source)
- All fields from the API response should be present in the YAML view; no fields
  should be silently omitted
- Nested structure (objects, arrays) should match the API response shape exactly
- Null/unset fields should appear explicitly as `null`, not be omitted
