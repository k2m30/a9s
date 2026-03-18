# QA-08: Detail View -- All Resource Types

Black-box user stories for the detail view across every resource type supported
by a9s. Each section lists the fields that must appear, how they map to the
equivalent AWS CLI JSON output, how nested/array values render, and how common
interactions (scroll, wrap, YAML, copy, back) behave.

---

## Table of Contents

1. [Common Behaviors (all types)](#1-common-behaviors-all-types)
2. [S3 Bucket](#2-s3-bucket)
3. [S3 Object](#3-s3-object)
4. [EC2 Instance](#4-ec2-instance)
5. [RDS Instance](#5-rds-instance)
6. [ElastiCache Redis](#6-elasticache-redis)
7. [DocumentDB Cluster](#7-documentdb-cluster)
8. [EKS Cluster](#8-eks-cluster)
9. [Secrets Manager](#9-secrets-manager)
10. [Cross-cutting Concerns](#10-cross-cutting-concerns)

---

## 1. Common Behaviors (all types)

These interactions apply identically regardless of resource type.

### 1.1 Entering the detail view

| # | Story | Expected |
|---|-------|----------|
| C-01 | Select a resource in the list and press `d` | Detail view opens; frame title shows the resource identifier (e.g. instance ID, bucket name) |
| C-02 | Select a resource in the list and press `Enter` | For most resource types, opens detail view (same as `d`). Exception: S3 buckets drill into the bucket's object list, and S3 folders navigate into the prefix. `d` always opens detail regardless of resource type. |
| C-03 | Select a resource that has no data loaded | Header shows "No detail data available for this resource"; view does NOT change |

### 1.2 Scroll

| # | Story | Expected |
|---|-------|----------|
| C-10 | Press `j` or Down arrow in detail view | Content scrolls down by one line |
| C-11 | Press `k` or Up arrow in detail view | Content scrolls up by one line |
| C-12 | Press `g` in detail view | Jumps to the very first line (top) |
| C-13 | Press `G` in detail view | Jumps to the last line (bottom) |
| C-14 | Press `k` when already at the top | Nothing happens; no wrap-around |
| C-15 | Detail content fits entirely within the frame height | Scroll keys have no visible effect; all lines remain visible |
| C-16 | Press `h` or Left arrow in detail view | Content scrolls left (horizontal scroll decreases by 4 chars) |
| C-17 | Press `l` or Right arrow in detail view | Content scrolls right (horizontal scroll increases by 4 chars) |

### 1.3 Word wrap toggle

| # | Story | Expected |
|---|-------|----------|
| C-20 | Press `w` in detail view | Long values (ARNs, endpoints, URLs) wrap at the frame width instead of overflowing right; horizontal scroll resets to 0 |
| C-21 | Press `w` again | Wrap turns off; long lines extend beyond the visible frame and can be scrolled horizontally with `h`/`l` |
| C-22 | Wrap is ON and user presses `l` | Horizontal scroll does NOT advance (wrap mode disables horizontal scroll) |

### 1.4 Switch to YAML

| # | Story | Expected |
|---|-------|----------|
| C-30 | Press `y` in detail view | View switches to YAML; frame title appends "yaml"; full resource struct is rendered in syntax-colored YAML |

### 1.5 Copy

| # | Story | Expected |
|---|-------|----------|
| C-40 | Press `c` in detail view | Full resource YAML is copied to system clipboard (not the rendered key-value text); header flashes "Copied YAML to clipboard" in green for ~2 seconds |
| C-41 | Copy fails (e.g. no clipboard daemon on headless Linux) | Header flashes "Copy failed: ..." in red for ~2 seconds |

### 1.6 Back

| # | Story | Expected |
|---|-------|----------|
| C-50 | Press `Esc` in detail view | Returns to the resource list; cursor remains on the same resource that was described |

### 1.7 Layout and styling

| # | Story | Expected |
|---|-------|----------|
| C-60 | Open any detail view | Keys are left-aligned, rendered in blue (#7aa2f7); values rendered in white (#c0caf5); each line formatted as ` Key:` padded to fixed width then value, with 1-space indent |
| C-61 | Detail view contains a multi-line / nested field | The field name appears alone on its line as ` FieldName:` (1-space indent, section header style) and sub-lines appear indented by 5 spaces below it |
| C-62 | A field has an empty or null value (config-driven rendering) | The field shows as ` FieldName:` padded to fixed width then `-` (dash placeholder). The field is NOT omitted from the view. |
| C-63 | A pointer field is nil in the AWS SDK struct | The field shows as ` FieldName:` with `-` (dash); it is NOT omitted from the view |

---

## 2. S3 Bucket

**Entry path:** Main Menu > S3 Buckets > select bucket > Enter/d

**AWS CLI equivalent:** `aws s3api list-buckets` (the ListBuckets response provides the base fields)

### 2.1 Detail fields

| # | Field displayed | views.yaml path | AWS CLI JSON key | Type | Example value |
|---|----------------|-----------------|-----------------|------|---------------|
| S3-D01 | BucketArn | BucketArn | `BucketArn` (if present in extended listing) | string | `arn:aws:s3:::my-bucket` |
| S3-D02 | BucketRegion | BucketRegion | `BucketRegion` (from GetBucketLocation or extended listing) | string | `us-east-1` |
| S3-D03 | CreationDate | CreationDate | `CreationDate` | time | `2024-01-15 09:22:31` |

### 2.2 Rendering stories

| # | Story | Expected |
|---|-------|----------|
| S3-D10 | Open detail for a bucket | Three fields appear: BucketArn, BucketRegion, CreationDate, in that order |
| S3-D11 | BucketArn contains a long ARN | Full ARN shown on one line; `w` toggles wrap to keep it visible without horizontal scroll |
| S3-D12 | BucketRegion is empty (legacy bucket without region metadata) | Line shows `  BucketRegion: ` (empty) |
| S3-D13 | CreationDate is a timestamp | Rendered as `YYYY-MM-DD HH:MM:SS` (not ISO 8601 with T/Z) |
| S3-D14 | All three fields fit on screen | No scrolling needed; `j`/`k` have no visible effect |

---

## 3. S3 Object

**Entry path:** Main Menu > S3 Buckets > select bucket > Enter > select object > Enter/d

**AWS CLI equivalent:** `aws s3api list-objects-v2 --bucket <name>` (the `Contents[]` items)

### 3.1 Detail fields

| # | Field displayed | views.yaml path | AWS CLI JSON key | Type | Example value |
|---|----------------|-----------------|-----------------|------|---------------|
| OBJ-D01 | Name | Name | `Key` (SDK maps to Object.Key; views.yaml references "Name" which may come from a transformed field) | string | `logs/2024/report.csv` |
| OBJ-D02 | LastModified | LastModified | `LastModified` | time | `2024-03-15 14:30:22` |
| OBJ-D03 | Owner | Owner | `Owner` (struct with `DisplayName` and `ID`) | nested | see below |

### 3.2 Rendering stories

| # | Story | Expected |
|---|-------|----------|
| OBJ-D10 | Open detail for an S3 object | Three fields: Name, LastModified, Owner in that order |
| OBJ-D11 | Owner field is populated | Because Owner is a struct (with DisplayName and ID sub-fields), it renders as a multi-line block: `  Owner:` on its own line, then indented YAML-formatted sub-fields below (e.g. `    DisplayName: someone`, `    ID: abc123...`) |
| OBJ-D12 | Owner field is nil (common when bucket owner not requested) | Line shows `  Owner: ` (empty string) |
| OBJ-D13 | Object key contains slashes (deep prefix path) | Full key shown as-is; no truncation |
| OBJ-D14 | LastModified renders as formatted timestamp | `YYYY-MM-DD HH:MM:SS` format |

---

## 4. EC2 Instance

**Entry path:** Main Menu > EC2 Instances > select instance > Enter/d

**AWS CLI equivalent:** `aws ec2 describe-instances` (the `Reservations[].Instances[]` items)

### 4.1 Detail fields

| # | Field displayed | views.yaml path | AWS CLI JSON key | Type | Example value |
|---|----------------|-----------------|-----------------|------|---------------|
| EC2-D01 | InstanceId | InstanceId | `InstanceId` | string | `i-0abc123def456789a` |
| EC2-D02 | State | State | `State` (struct: Name + Code) | nested | see below |
| EC2-D03 | InstanceType | InstanceType | `InstanceType` | string (enum) | `t3.medium` |
| EC2-D04 | ImageId | ImageId | `ImageId` | string | `ami-0abcdef01234567` |
| EC2-D05 | VpcId | VpcId | `VpcId` | string | `vpc-0123456789abcdef0` |
| EC2-D06 | SubnetId | SubnetId | `SubnetId` | string | `subnet-0123456789abcde` |
| EC2-D07 | PrivateIpAddress | PrivateIpAddress | `PrivateIpAddress` | string | `10.0.1.42` |
| EC2-D08 | PublicIpAddress | PublicIpAddress | `PublicIpAddress` | string | `54.123.45.67` |
| EC2-D09 | SecurityGroups | SecurityGroups | `SecurityGroups` (array of {GroupId, GroupName}) | array | see below |
| EC2-D10 | LaunchTime | LaunchTime | `LaunchTime` | time | `2024-01-15 09:22:31` |
| EC2-D11 | Architecture | Architecture | `Architecture` | string (enum) | `x86_64` |
| EC2-D12 | Platform | Platform | `Platform` | string | `windows` or empty |
| EC2-D13 | Tags | Tags | `Tags` (array of {Key, Value}) | array | see below |

### 4.2 Nested field rendering

#### State (struct)

| # | Story | Expected |
|---|-------|----------|
| EC2-D20 | State field for a running instance | Renders as multi-line block: `  State:` then indented sub-fields `    Code: 16` and `    Name: running` (zero-valued fields omitted) |
| EC2-D21 | State field for a terminated instance | Sub-fields show `    Code: 48` and `    Name: terminated` |

#### SecurityGroups (array of structs)

| # | Story | Expected |
|---|-------|----------|
| EC2-D30 | Instance has 2 security groups | Renders as multi-line: `  SecurityGroups:` on its own line, followed by YAML-formatted array: `    - GroupId: sg-aaa` / `      GroupName: web-sg` / `    - GroupId: sg-bbb` / `      GroupName: db-sg` |
| EC2-D31 | Instance has 0 security groups | Line shows `  SecurityGroups: ` (empty; zero-length slices render as empty) |
| EC2-D32 | Instance has 5+ security groups | All groups rendered; vertical scroll (`j`/`k`) needed if list exceeds frame height |

#### Tags (array of Key/Value structs)

| # | Story | Expected |
|---|-------|----------|
| EC2-D40 | Instance has 3 tags | Renders as multi-line: `  Tags:` then indented YAML: `    - Key: Name` / `      Value: api-prod-01` / `    - Key: Environment` / `      Value: production` / etc. |
| EC2-D41 | Instance has 0 tags | Line shows `  Tags: ` (empty) |
| EC2-D42 | A tag value is very long (e.g. a description) | Full value shown on one line; `w` wraps it within frame width |

### 4.3 Edge cases

| # | Story | Expected |
|---|-------|----------|
| EC2-D50 | PublicIpAddress is nil (instance in private subnet) | Line shows `  PublicIpAddress: ` (empty string) |
| EC2-D51 | Platform is nil (Linux instances do not set this) | Line shows `  Platform: ` (empty string) |
| EC2-D52 | All 13 fields displayed at 80-col, 24-row terminal | Some fields extend below visible area; `j`/`G` scroll to see Tags at the bottom |

---

## 5. RDS Instance

**Entry path:** Main Menu > RDS Instances > select instance > Enter/d

**AWS CLI equivalent:** `aws rds describe-db-instances` (the `DBInstances[]` items)

### 5.1 Detail fields

| # | Field displayed | views.yaml path | AWS CLI JSON key | Type | Example value |
|---|----------------|-----------------|-----------------|------|---------------|
| RDS-D01 | DBInstanceIdentifier | DBInstanceIdentifier | `DBInstanceIdentifier` | string | `mydb-prod` |
| RDS-D02 | Engine | Engine | `Engine` | string | `mysql` |
| RDS-D03 | EngineVersion | EngineVersion | `EngineVersion` | string | `8.0.35` |
| RDS-D04 | DBInstanceStatus | DBInstanceStatus | `DBInstanceStatus` | string | `available` |
| RDS-D05 | DBInstanceClass | DBInstanceClass | `DBInstanceClass` | string | `db.r5.large` |
| RDS-D06 | Endpoint | Endpoint | `Endpoint` (struct: Address, Port, HostedZoneId) | nested | see below |
| RDS-D07 | MultiAZ | MultiAZ | `MultiAZ` | bool | `Yes` or `No` |
| RDS-D08 | AllocatedStorage | AllocatedStorage | `AllocatedStorage` | int32 | `100` |
| RDS-D09 | StorageType | StorageType | `StorageType` | string | `gp3` |
| RDS-D10 | AvailabilityZone | AvailabilityZone | `AvailabilityZone` | string | `us-east-1a` |

### 5.2 Nested field rendering

#### Endpoint (struct)

| # | Story | Expected |
|---|-------|----------|
| RDS-D20 | Endpoint is populated | Renders as multi-line: `  Endpoint:` then indented sub-fields: `    Address: mydb-prod.c9abcdef.us-east-1.rds.amazonaws.com` / `    HostedZoneId: Z2R2ITUGPM61AM` / `    Port: 3306` (zero-valued sub-fields omitted) |
| RDS-D21 | Endpoint is nil (instance still creating) | Line shows `  Endpoint: ` (empty) |
| RDS-D22 | Endpoint Address is a long FQDN | Full address shown; `w` wraps to keep it within frame width |

### 5.3 Scalar rendering

| # | Story | Expected |
|---|-------|----------|
| RDS-D30 | MultiAZ is true | Rendered as `  MultiAZ: Yes` (booleans render as Yes/No) |
| RDS-D31 | MultiAZ is false | Rendered as `  MultiAZ: No` |
| RDS-D32 | AllocatedStorage is 100 | Rendered as `  AllocatedStorage: 100` (integer, no units) |
| RDS-D33 | All 10 fields fit on one screen | All visible without scrolling at 24+ row terminal |

---

## 6. ElastiCache Redis

**Entry path:** Main Menu > ElastiCache Redis > select cluster > Enter/d

**AWS CLI equivalent:** `aws elasticache describe-cache-clusters` (the `CacheClusters[]` items)

### 6.1 Detail fields

| # | Field displayed | views.yaml path | AWS CLI JSON key | Type | Example value |
|---|----------------|-----------------|-----------------|------|---------------|
| RED-D01 | CacheClusterId | CacheClusterId | `CacheClusterId` | string | `prod-redis-001` |
| RED-D02 | Engine | Engine | `Engine` | string | `redis` |
| RED-D03 | EngineVersion | EngineVersion | `EngineVersion` | string | `7.0.12` |
| RED-D04 | CacheClusterStatus | CacheClusterStatus | `CacheClusterStatus` | string | `available` |
| RED-D05 | CacheNodeType | CacheNodeType | `CacheNodeType` | string | `cache.r6g.large` |
| RED-D06 | NumCacheNodes | NumCacheNodes | `NumCacheNodes` | int32 | `3` |
| RED-D07 | ConfigurationEndpoint | ConfigurationEndpoint | `ConfigurationEndpoint` (struct: Address, Port) | nested | see below |
| RED-D08 | PreferredAvailabilityZone | PreferredAvailabilityZone | `PreferredAvailabilityZone` | string | `us-east-1a` |

### 6.2 Nested field rendering

#### ConfigurationEndpoint (struct)

| # | Story | Expected |
|---|-------|----------|
| RED-D20 | ConfigurationEndpoint is populated (cluster mode enabled) | Renders as multi-line: `  ConfigurationEndpoint:` then `    Address: prod-redis-001.abcdef.clustercfg.use1.cache.amazonaws.com` / `    Port: 6379` |
| RED-D21 | ConfigurationEndpoint is nil (cluster mode disabled or non-clustered) | Line shows `  ConfigurationEndpoint: ` (empty) |
| RED-D22 | Address is a long cluster-mode FQDN | Full address shown; `w` wraps |

### 6.3 Scalar rendering

| # | Story | Expected |
|---|-------|----------|
| RED-D30 | NumCacheNodes is 3 | Rendered as `  NumCacheNodes: 3` |
| RED-D31 | All 8 fields fit on one screen | All visible without scrolling |

---

## 7. DocumentDB Cluster

**Entry path:** Main Menu > DocumentDB Clusters > select cluster > Enter/d

**AWS CLI equivalent:** `aws docdb describe-db-clusters` (the `DBClusters[]` items)

### 7.1 Detail fields

| # | Field displayed | views.yaml path | AWS CLI JSON key | Type | Example value |
|---|----------------|-----------------|-----------------|------|---------------|
| DOC-D01 | DBClusterIdentifier | DBClusterIdentifier | `DBClusterIdentifier` | string | `prod-docdb` |
| DOC-D02 | Engine | Engine | `Engine` | string | `docdb` |
| DOC-D03 | EngineVersion | EngineVersion | `EngineVersion` | string | `5.0.0` |
| DOC-D04 | Status | Status | `Status` | string | `available` |
| DOC-D05 | Endpoint | Endpoint | `Endpoint` | string | `prod-docdb.cluster-c9abcdef.us-east-1.docdb.amazonaws.com` |
| DOC-D06 | ReaderEndpoint | ReaderEndpoint | `ReaderEndpoint` | string | `prod-docdb.cluster-ro-c9abcdef.us-east-1.docdb.amazonaws.com` |
| DOC-D07 | Port | Port | `Port` | int32 | `27017` |
| DOC-D08 | StorageEncrypted | StorageEncrypted | `StorageEncrypted` | bool | `Yes` or `No` |
| DOC-D09 | DBClusterMembers | DBClusterMembers | `DBClusterMembers` (array of structs) | array | see below |

### 7.2 Nested field rendering

#### DBClusterMembers (array of structs)

| # | Story | Expected |
|---|-------|----------|
| DOC-D20 | Cluster has 3 members | Renders as multi-line: `  DBClusterMembers:` then YAML array: `    - DBClusterParameterGroupStatus: in-sync` / `      DBInstanceIdentifier: prod-docdb-1` / `      IsClusterWriter: Yes` / `      PromotionTier: 1` / `    - DBInstanceIdentifier: prod-docdb-2` / `      IsClusterWriter: No` / etc. |
| DOC-D21 | Cluster has 0 members (newly created) | Line shows `  DBClusterMembers: ` (empty; zero-length slices render as empty) |
| DOC-D22 | Cluster has 5 members and terminal is 24 rows | Members list extends below visible area; `j`/`G` needed to scroll to see all |

#### Endpoint and ReaderEndpoint (strings in DocumentDB, not structs)

| # | Story | Expected |
|---|-------|----------|
| DOC-D30 | Endpoint is a long FQDN | Rendered as a single scalar line: `  Endpoint: prod-docdb.cluster-c9abcdef.us-east-1.docdb.amazonaws.com`; `w` wraps |
| DOC-D31 | ReaderEndpoint is a long FQDN | Same scalar rendering with wrap support |

### 7.3 Scalar rendering

| # | Story | Expected |
|---|-------|----------|
| DOC-D40 | StorageEncrypted is true | `  StorageEncrypted: Yes` |
| DOC-D41 | StorageEncrypted is false | `  StorageEncrypted: No` |
| DOC-D42 | Port is 27017 | `  Port: 27017` |

---

## 8. EKS Cluster

**Entry path:** Main Menu > EKS Clusters > select cluster > Enter/d

**AWS CLI equivalent:** `aws eks describe-cluster --name <name>` (the `cluster` object)

### 8.1 Detail fields

| # | Field displayed | views.yaml path | AWS CLI JSON key | Type | Example value |
|---|----------------|-----------------|-----------------|------|---------------|
| EKS-D01 | Name | Name | `name` | string | `prod-cluster` |
| EKS-D02 | Version | Version | `version` | string | `1.29` |
| EKS-D03 | Status | Status | `status` | string (enum) | `ACTIVE` |
| EKS-D04 | Endpoint | Endpoint | `endpoint` | string | `https://ABCDEF.gr7.us-east-1.eks.amazonaws.com` |
| EKS-D05 | PlatformVersion | PlatformVersion | `platformVersion` | string | `eks.8` |
| EKS-D06 | Arn | Arn | `arn` | string | `arn:aws:eks:us-east-1:123456789012:cluster/prod-cluster` |
| EKS-D07 | RoleArn | RoleArn | `roleArn` | string | `arn:aws:iam::123456789012:role/eks-cluster-role` |
| EKS-D08 | KubernetesNetworkConfig | KubernetesNetworkConfig | `kubernetesNetworkConfig` (struct) | nested | see below |

### 8.2 Nested field rendering

#### KubernetesNetworkConfig (struct)

| # | Story | Expected |
|---|-------|----------|
| EKS-D20 | KubernetesNetworkConfig is populated | Renders as multi-line: `  KubernetesNetworkConfig:` then indented sub-fields: `    IpFamily: ipv4` / `    ServiceIpv4Cidr: 172.20.0.0/16` (zero-valued sub-fields like ServiceIpv6Cidr and nested ElasticLoadBalancing are omitted if nil/zero) |
| EKS-D21 | KubernetesNetworkConfig has both IPv4 and IPv6 CIDRs | Both appear: `    ServiceIpv4Cidr: 172.20.0.0/16` and `    ServiceIpv6Cidr: fd00::/108` |
| EKS-D22 | KubernetesNetworkConfig includes ElasticLoadBalancing | Nested sub-struct appears: `    ElasticLoadBalancing:` / `      Enabled: Yes` |
| EKS-D23 | KubernetesNetworkConfig is nil (should not happen for active clusters) | Line shows `  KubernetesNetworkConfig: ` (empty) |

#### Long scalar values

| # | Story | Expected |
|---|-------|----------|
| EKS-D30 | Endpoint is a long HTTPS URL | Full URL on one line; requires horizontal scroll or `w` wrap to see fully in narrow terminals |
| EKS-D31 | Arn is a long ARN string | Full ARN on one line; `w` wraps |
| EKS-D32 | RoleArn is a long IAM ARN | Full ARN on one line; `w` wraps |

### 8.3 Edge cases

| # | Story | Expected |
|---|-------|----------|
| EKS-D40 | All 8 fields at 80 columns with KubernetesNetworkConfig expanded | Multi-line sub-fields may push total line count above visible area; `j`/`G` to scroll |

---

## 9. Secrets Manager

**Entry path:** Main Menu > Secrets Manager > select secret > Enter/d

**AWS CLI equivalent:** `aws secretsmanager list-secrets` (the `SecretList[]` items)

### 9.1 Detail fields

| # | Field displayed | views.yaml path | AWS CLI JSON key | Type | Example value |
|---|----------------|-----------------|-----------------|------|---------------|
| SEC-D01 | Name | Name | `Name` | string | `prod/api/db-password` |
| SEC-D02 | Description | Description | `Description` | string | `Production database credentials` |
| SEC-D03 | LastAccessedDate | LastAccessedDate | `LastAccessedDate` | time | `2024-03-17 00:00:00` |
| SEC-D04 | LastChangedDate | LastChangedDate | `LastChangedDate` | time | `2024-02-10 14:23:00` |
| SEC-D05 | RotationEnabled | RotationEnabled | `RotationEnabled` | bool | `Yes` or `No` |
| SEC-D06 | ARN | ARN | `ARN` | string | `arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/api/db-password-AbCdEf` |
| SEC-D07 | KmsKeyId | KmsKeyId | `KmsKeyId` | string | `arn:aws:kms:us-east-1:123456789012:key/12345678-...` or `alias/aws/secretsmanager` |
| SEC-D08 | Tags | Tags | `Tags` (array of {Key, Value}) | array | see below |

### 9.2 Nested field rendering

#### Tags (array of Key/Value structs)

| # | Story | Expected |
|---|-------|----------|
| SEC-D20 | Secret has 2 tags | Renders as multi-line: `  Tags:` then YAML array: `    - Key: Environment` / `      Value: production` / `    - Key: Team` / `      Value: platform` |
| SEC-D21 | Secret has 0 tags | Line shows `  Tags: ` (empty) |
| SEC-D22 | A tag has a very long value | Full value on one line; `w` wraps |

### 9.3 Scalar rendering

| # | Story | Expected |
|---|-------|----------|
| SEC-D30 | RotationEnabled is true | `  RotationEnabled: Yes` |
| SEC-D31 | RotationEnabled is false | `  RotationEnabled: No` |
| SEC-D32 | Description is empty string | `  Description: ` (empty) |
| SEC-D33 | KmsKeyId is nil (using default AWS-managed key) | `  KmsKeyId: ` (empty) |
| SEC-D34 | ARN is a long string | Full ARN; `w` wraps; `h`/`l` scrolls horizontally when wrap is off |
| SEC-D35 | LastAccessedDate is a date with time truncated to midnight by AWS | Rendered as `YYYY-MM-DD 00:00:00` |

### 9.4 Note on reveal vs. detail

| # | Story | Expected |
|---|-------|----------|
| SEC-D40 | Press `Enter`/`d` on a secret | Opens the standard detail view showing metadata fields above -- does NOT reveal the secret value |
| SEC-D41 | Press `x` on a secret (from the list view) | Opens the reveal view (a separate view type); NOT the detail view tested here |

---

## 10. Cross-cutting Concerns

### 10.1 Boolean formatting across all types

| # | Story | Expected |
|---|-------|----------|
| X-01 | Any boolean field with value true | Displayed as `Yes` |
| X-02 | Any boolean field with value false | Displayed as `No` |
| X-03 | Any boolean pointer that is nil | Displayed as empty string (not "No") |

### 10.2 Timestamp formatting across all types

| # | Story | Expected |
|---|-------|----------|
| X-10 | Any time.Time field with a value | Formatted as `YYYY-MM-DD HH:MM:SS` (no T separator, no Z suffix, no timezone label) |
| X-11 | Any time.Time field that is zero | Displayed as empty string |

### 10.3 Integer and float formatting

| # | Story | Expected |
|---|-------|----------|
| X-20 | An integer field (e.g. AllocatedStorage=100, Port=27017) | Displayed as a plain number with no commas, units, or formatting |
| X-21 | An integer field with value 0 | Displayed as empty string (zero values are omitted by isZeroOrNil) |

### 10.4 Nested struct rendering (general pattern)

| # | Story | Expected |
|---|-------|----------|
| X-30 | Any struct field (State, Endpoint, KubernetesNetworkConfig, Owner) | Field name appears alone: `  FieldName:` then each non-zero exported sub-field is indented below in YAML format |
| X-31 | A struct where all sub-fields are zero/nil | Line shows `  FieldName: ` (empty; ToSafeValue returns nil for all-zero structs) |
| X-32 | Nested struct within a struct (e.g. ElasticLoadBalancing inside KubernetesNetworkConfig) | Further indented YAML nesting |

### 10.5 Array/slice rendering (general pattern)

| # | Story | Expected |
|---|-------|----------|
| X-40 | Any array field with N > 0 elements (SecurityGroups, Tags, DBClusterMembers) | Field name on its own line: `  FieldName:` then each element rendered as a YAML array item with `    - key: value` indentation |
| X-41 | Any array field with 0 elements | Line shows `  FieldName: ` (empty; zero-length slices render as empty) |
| X-42 | Array of structs where each struct has mixed populated/empty fields | Only non-zero sub-fields appear in each array item |

### 10.6 Field ordering

| # | Story | Expected |
|---|-------|----------|
| X-50 | Open detail for any resource type | Fields appear in the exact order listed in the `detail:` section of views.yaml, NOT alphabetically sorted |
| X-51 | Compare field order with views.yaml `detail` list | One-to-one match; first field in the YAML list appears first in the rendered view |

### 10.7 Frame title

| # | Story | Expected |
|---|-------|----------|
| X-60 | Open detail for EC2 instance i-0abc123 | Frame title is centered in the top border: something like `--- i-0abc123 ---` (the resource name/ID) |
| X-61 | Open detail for a resource with a very long name | Title is truncated or the dashes on either side shrink to fit within the frame width |

### 10.8 AWS CLI field mapping accuracy

For each resource type, the following AWS CLI commands return the JSON
structures that contain the fields displayed in the detail view. A tester
should run these commands and verify that the a9s detail view shows the same
values (modulo formatting: timestamps, booleans, nested rendering).

| Resource type | AWS CLI command | JSON path to resource |
|---------------|----------------|----------------------|
| S3 bucket | `aws s3api list-buckets` | `Buckets[]` |
| S3 object | `aws s3api list-objects-v2 --bucket <name>` | `Contents[]` |
| EC2 | `aws ec2 describe-instances` | `Reservations[].Instances[]` |
| RDS | `aws rds describe-db-instances` | `DBInstances[]` |
| Redis | `aws elasticache describe-cache-clusters` | `CacheClusters[]` |
| DocumentDB | `aws docdb describe-db-clusters` | `DBClusters[]` |
| EKS | `aws eks describe-cluster --name <name>` | `cluster` |
| Secrets | `aws secretsmanager list-secrets` | `SecretList[]` |

### 10.9 Formatting differences between a9s and AWS CLI JSON

| Data type | AWS CLI JSON | a9s detail view |
|-----------|-------------|-----------------|
| Timestamps | `"2024-01-15T09:22:31Z"` | `2024-01-15 09:22:31` |
| Booleans | `true` / `false` | `Yes` / `No` |
| Null / missing | `null` or key absent | empty string |
| Integers | `100` | `100` |
| Enums | `"running"` | `running` (no quotes) |
| Nested structs | `{ "Address": "...", "Port": 3306 }` | Multi-line indented YAML sub-fields |
| Arrays | `[ { "Key": "Name", "Value": "x" } ]` | Multi-line YAML array items |
| Zero integers | `0` | empty string (omitted as zero value) |
| Empty strings | `""` | empty string |

---

## Story Count Summary

| Section | Stories |
|---------|---------|
| Common behaviors | 18 |
| S3 bucket | 5 |
| S3 object | 5 |
| EC2 instance | 16 |
| RDS instance | 7 |
| Redis | 4 |
| DocumentDB | 7 |
| EKS cluster | 8 |
| Secrets Manager | 10 |
| Cross-cutting | 20 |
| **Total** | **100** |
