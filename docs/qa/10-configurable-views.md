# QA User Stories: Configurable Views (views.yaml)

Scope: the views.yaml configuration file that lets users customize which columns appear in list views and which fields appear in detail views for each resource type. All stories treat a9s as a black box. Every "Given" block uses the EXACT YAML from the project's views.yaml unless the story explicitly tests a modification scenario.

---

## Section A: Exact Current Config Verification

All stories in this section use the real views.yaml shipped with the project.

---

### A1. S3 Buckets -- List View

**Given:** this views.yaml s3 list config:
```yaml
s3:
  list:
    Bucket Name:
      path: Name
      width: 20
    Creation Date:
      path: CreationDate
      width: 22
```
**When:** user navigates to the S3 resource list
**Then:**
- Column headers appear in order: "Bucket Name", "Creation Date"
- Bucket Name column is 20 chars wide
- Creation Date column is 22 chars wide
- Bucket Name values come from the AWS `Name` field (e.g., `my-app-assets`)
- Creation Date values come from the AWS `CreationDate` field and display as a human-readable timestamp (e.g., `2024-01-15 09:22:31`), not a Go struct dump

**AWS comparison:** `aws s3api list-buckets`
- Name -> `.Buckets[].Name`
- CreationDate -> `.Buckets[].CreationDate`

---

### A2. S3 Buckets -- Detail View

**Given:** this views.yaml s3 detail config:
```yaml
s3:
  detail:
    - BucketArn
    - BucketRegion
    - CreationDate
```
**When:** user selects an S3 bucket and presses `d` for the detail view
**Then:**
- Detail view shows exactly 3 fields in this order: BucketArn, BucketRegion, CreationDate
- BucketArn shows the full ARN string (e.g., `arn:aws:s3:::my-app-assets`)
- BucketRegion shows a region identifier (e.g., `us-east-1`)
- CreationDate shows a formatted timestamp
- No other fields are displayed

---

### A3. S3 Objects -- List View

**Given:** this views.yaml s3_objects list config:
```yaml
s3_objects:
  list:
    Key:
      path: Key
      width: 20
    Size:
      path: Size
      width: 12
    Last Modified:
      path: LastModified
      width: 22
```
**When:** user navigates into an S3 bucket to view its objects
**Then:**
- Column headers appear in order: "Key", "Size", "Last Modified"
- Key column is 20 chars wide
- Size column is 12 chars wide
- Last Modified column is 22 chars wide
- Key values show object paths like `images/photo.png`
- Size values show numeric byte counts
- Last Modified values show formatted timestamps

**AWS comparison:** `aws s3api list-objects-v2 --bucket <name>`
- Key -> `.Contents[].Key`
- Size -> `.Contents[].Size`
- LastModified -> `.Contents[].LastModified`

---

### A4. S3 Objects -- Detail View

**Given:** this views.yaml s3_objects detail config:
```yaml
s3_objects:
  detail:
    - Name
    - LastModified
    - Owner
```
**When:** user selects an S3 object and presses `d` for the detail view
**Then:**
- Detail view shows exactly 3 fields in this order: Name, LastModified, Owner
- Owner is a struct and should show expanded sub-fields (DisplayName, ID)
- No other fields are displayed

---

### A5. EC2 Instances -- List View

**Given:** this views.yaml ec2 list config:
```yaml
ec2:
  list:
    Instance ID:
      path: InstanceId
      width: 20
    State:
      path: State.Name
      width: 12
    Type:
      path: InstanceType
      width: 14
    Private IP:
      path: PrivateIpAddress
      width: 16
    Public IP:
      path: PublicIpAddress
      width: 16
    Launch Time:
      path: LaunchTime
      width: 22
```
**When:** user navigates to EC2 resource list
**Then:**
- Column headers appear in order: "Instance ID", "State", "Type", "Private IP", "Public IP", "Launch Time"
- Instance ID column is 20 chars wide
- State column is 12 chars wide
- Type column is 14 chars wide
- Private IP column is 16 chars wide
- Public IP column is 16 chars wide
- Launch Time column is 22 chars wide
- Values come from AWS via the dot-paths (State.Name extracts the nested state name like "running", not the State object)
- Instances without a public IP show a blank cell in the Public IP column

**AWS comparison:** `aws ec2 describe-instances`
- InstanceId -> `.Reservations[].Instances[].InstanceId`
- State.Name -> `.Reservations[].Instances[].State.Name`
- InstanceType -> `.Reservations[].Instances[].InstanceType`
- PrivateIpAddress -> `.Reservations[].Instances[].PrivateIpAddress`
- PublicIpAddress -> `.Reservations[].Instances[].PublicIpAddress`
- LaunchTime -> `.Reservations[].Instances[].LaunchTime`

---

### A6. EC2 Instances -- Detail View

**Given:** this views.yaml ec2 detail config:
```yaml
ec2:
  detail:
    - InstanceId
    - State
    - InstanceType
    - ImageId
    - VpcId
    - SubnetId
    - PrivateIpAddress
    - PublicIpAddress
    - SecurityGroups
    - LaunchTime
    - Architecture
    - Platform
    - Tags
```
**When:** user selects an EC2 instance and presses `d`
**Then:**
- Detail view shows exactly 13 fields in this order: InstanceId, State, InstanceType, ImageId, VpcId, SubnetId, PrivateIpAddress, PublicIpAddress, SecurityGroups, LaunchTime, Architecture, Platform, Tags
- InstanceId shows a string like `i-0abc123def456789a`
- State is a struct and shows expanded sub-fields (Name, Code)
- SecurityGroups is an array of structs and shows as an expandable section listing each group with GroupId and GroupName
- Tags is an array of key-value structs and shows as a section with each tag's Key and Value
- Architecture shows a string like `x86_64`
- Platform shows a string (or blank if nil)
- No other fields are displayed

---

### A7. RDS Instances -- List View

**Given:** this views.yaml rds list config:
```yaml
rds:
  list:
    DB Identifier:
      path: DBInstanceIdentifier
      width: 28
    Engine:
      path: Engine
      width: 12
    Version:
      path: EngineVersion
      width: 10
    Status:
      path: DBInstanceStatus
      width: 14
    Class:
      path: DBInstanceClass
      width: 16
    Endpoint:
      path: Endpoint.Address
      width: 40
    Multi-AZ:
      path: MultiAZ
      width: 10
```
**When:** user navigates to RDS resource list
**Then:**
- Column headers appear in order: "DB Identifier", "Engine", "Version", "Status", "Class", "Endpoint", "Multi-AZ"
- DB Identifier column is 28 chars wide
- Engine column is 12 chars wide
- Version column is 10 chars wide
- Status column is 14 chars wide
- Class column is 16 chars wide
- Endpoint column is 40 chars wide
- Multi-AZ column is 10 chars wide
- DB Identifier shows values like `mydb-prod`
- Engine shows values like `mysql`, `postgres`, `aurora-mysql`
- Version shows values like `8.0.35`, `15.4`
- Status shows values like `available`, `creating`, `stopped`
- Class shows values like `db.r5.large`, `db.t3.medium`
- Endpoint resolves the nested `Endpoint.Address` to show just the hostname string (e.g., `mydb.abc123.us-east-1.rds.amazonaws.com`), not the full Endpoint struct
- Multi-AZ shows a boolean value (Yes/No)

**AWS comparison:** `aws rds describe-db-instances`
- DBInstanceIdentifier -> `.DBInstances[].DBInstanceIdentifier`
- Engine -> `.DBInstances[].Engine`
- EngineVersion -> `.DBInstances[].EngineVersion`
- DBInstanceStatus -> `.DBInstances[].DBInstanceStatus`
- DBInstanceClass -> `.DBInstances[].DBInstanceClass`
- Endpoint.Address -> `.DBInstances[].Endpoint.Address`
- MultiAZ -> `.DBInstances[].MultiAZ`

---

### A8. RDS Instances -- Detail View

**Given:** this views.yaml rds detail config:
```yaml
rds:
  detail:
    - DBInstanceIdentifier
    - Engine
    - EngineVersion
    - DBInstanceStatus
    - DBInstanceClass
    - Endpoint
    - MultiAZ
    - AllocatedStorage
    - StorageType
    - AvailabilityZone
```
**When:** user selects an RDS instance and presses `d`
**Then:**
- Detail view shows exactly 10 fields in this order: DBInstanceIdentifier, Engine, EngineVersion, DBInstanceStatus, DBInstanceClass, Endpoint, MultiAZ, AllocatedStorage, StorageType, AvailabilityZone
- Endpoint is a struct and shows expanded sub-fields (Address, Port, HostedZoneId)
- MultiAZ shows Yes or No
- AllocatedStorage shows a numeric value (e.g., `100`)
- StorageType shows a string like `gp3`, `io1`
- No other fields are displayed

---

### A9. Redis (ElastiCache) -- List View

**Given:** this views.yaml redis list config:
```yaml
redis:
  list:
    Cluster ID:
      path: CacheClusterId
      width: 28
    Version:
      path: EngineVersion
      width: 10
    Node Type:
      path: CacheNodeType
      width: 18
    Status:
      path: CacheClusterStatus
      width: 14
    Nodes:
      path: NumCacheNodes
      width: 8
    Endpoint:
      path: ConfigurationEndpoint.Address
      width: 40
```
**When:** user navigates to Redis resource list
**Then:**
- Column headers appear in order: "Cluster ID", "Version", "Node Type", "Status", "Nodes", "Endpoint"
- Cluster ID column is 28 chars wide
- Version column is 10 chars wide
- Node Type column is 18 chars wide
- Status column is 14 chars wide
- Nodes column is 8 chars wide
- Endpoint column is 40 chars wide
- Cluster ID shows identifiers like `my-redis-cluster`
- Version shows Redis engine versions like `7.0`
- Node Type shows values like `cache.r6g.large`
- Status shows values like `available`, `creating`
- Nodes shows a numeric node count
- Endpoint resolves the nested `ConfigurationEndpoint.Address` to show just the address string, not the full ConfigurationEndpoint struct

**AWS comparison:** `aws elasticache describe-cache-clusters`
- CacheClusterId -> `.CacheClusters[].CacheClusterId`
- EngineVersion -> `.CacheClusters[].EngineVersion`
- CacheNodeType -> `.CacheClusters[].CacheNodeType`
- CacheClusterStatus -> `.CacheClusters[].CacheClusterStatus`
- NumCacheNodes -> `.CacheClusters[].NumCacheNodes`
- ConfigurationEndpoint.Address -> `.CacheClusters[].ConfigurationEndpoint.Address`

---

### A10. Redis (ElastiCache) -- Detail View

**Given:** this views.yaml redis detail config:
```yaml
redis:
  detail:
    - CacheClusterId
    - Engine
    - EngineVersion
    - CacheClusterStatus
    - CacheNodeType
    - NumCacheNodes
    - ConfigurationEndpoint
    - PreferredAvailabilityZone
```
**When:** user selects a Redis cluster and presses `d`
**Then:**
- Detail view shows exactly 8 fields in this order: CacheClusterId, Engine, EngineVersion, CacheClusterStatus, CacheNodeType, NumCacheNodes, ConfigurationEndpoint, PreferredAvailabilityZone
- ConfigurationEndpoint is a struct and shows expanded sub-fields (Address, Port)
- NumCacheNodes shows a numeric value
- No other fields are displayed

---

### A11. DocumentDB -- List View

**Given:** this views.yaml docdb list config:
```yaml
docdb:
  list:
    Cluster ID:
      path: DBClusterIdentifier
      width: 28
    Version:
      path: EngineVersion
      width: 10
    Status:
      path: Status
      width: 14
    Instances:
      path: DBClusterMembers
      width: 10
    Endpoint:
      path: Endpoint
      width: 48
```
**When:** user navigates to DocumentDB resource list
**Then:**
- Column headers appear in order: "Cluster ID", "Version", "Status", "Instances", "Endpoint"
- Cluster ID column is 28 chars wide
- Version column is 10 chars wide
- Status column is 14 chars wide
- Instances column is 10 chars wide
- Endpoint column is 48 chars wide
- Cluster ID shows identifiers like `my-docdb-cluster`
- Version shows engine versions like `5.0.0`
- Status shows values like `available`, `creating`
- Instances shows a count of cluster members (e.g., `3`), not an array dump of the DBClusterMembers struct array
- Endpoint shows the cluster endpoint hostname string

**AWS comparison:** `aws docdb describe-db-clusters`
- DBClusterIdentifier -> `.DBClusters[].DBClusterIdentifier`
- EngineVersion -> `.DBClusters[].EngineVersion`
- Status -> `.DBClusters[].Status`
- DBClusterMembers -> `.DBClusters[].DBClusterMembers` (count of array)
- Endpoint -> `.DBClusters[].Endpoint`

---

### A12. DocumentDB -- Detail View

**Given:** this views.yaml docdb detail config:
```yaml
docdb:
  detail:
    - DBClusterIdentifier
    - Engine
    - EngineVersion
    - Status
    - Endpoint
    - ReaderEndpoint
    - Port
    - StorageEncrypted
    - DBClusterMembers
```
**When:** user selects a DocumentDB cluster and presses `d`
**Then:**
- Detail view shows exactly 9 fields in this order: DBClusterIdentifier, Engine, EngineVersion, Status, Endpoint, ReaderEndpoint, Port, StorageEncrypted, DBClusterMembers
- Port shows a numeric value like `27017`
- StorageEncrypted shows Yes or No
- DBClusterMembers is an array of structs and shows as an expandable section listing each member with sub-fields (DBInstanceIdentifier, IsClusterWriter, PromotionTier, DBClusterParameterGroupStatus)
- No other fields are displayed

---

### A13. EKS Clusters -- List View

**Given:** this views.yaml eks list config:
```yaml
eks:
  list:
    Cluster Name:
      path: Name
      width: 28
    Version:
      path: Version
      width: 10
    Status:
      path: Status
      width: 14
    Endpoint:
      path: Endpoint
      width: 48
    Platform Version:
      path: PlatformVersion
      width: 18
```
**When:** user navigates to EKS resource list
**Then:**
- Column headers appear in order: "Cluster Name", "Version", "Status", "Endpoint", "Platform Version"
- Cluster Name column is 28 chars wide
- Version column is 10 chars wide
- Status column is 14 chars wide
- Endpoint column is 48 chars wide
- Platform Version column is 18 chars wide
- Cluster Name shows identifiers like `my-production-cluster`
- Version shows Kubernetes versions like `1.29`
- Status shows values like `ACTIVE`, `CREATING`
- Endpoint shows the cluster API endpoint URL (e.g., `https://ABC123.gr7.us-east-1.eks.amazonaws.com`)
- Platform Version shows values like `eks.12`

**AWS comparison:** `aws eks list-clusters` + `aws eks describe-cluster --name <name>`
- Name -> `.cluster.name`
- Version -> `.cluster.version`
- Status -> `.cluster.status`
- Endpoint -> `.cluster.endpoint`
- PlatformVersion -> `.cluster.platformVersion`

---

### A14. EKS Clusters -- Detail View

**Given:** this views.yaml eks detail config:
```yaml
eks:
  detail:
    - Name
    - Version
    - Status
    - Endpoint
    - PlatformVersion
    - Arn
    - RoleArn
    - KubernetesNetworkConfig
```
**When:** user selects an EKS cluster and presses `d`
**Then:**
- Detail view shows exactly 8 fields in this order: Name, Version, Status, Endpoint, PlatformVersion, Arn, RoleArn, KubernetesNetworkConfig
- Arn shows the full cluster ARN
- RoleArn shows the IAM role ARN
- KubernetesNetworkConfig is a struct and shows as a nested section with sub-fields (IpFamily, ServiceIpv4Cidr, ServiceIpv6Cidr, ElasticLoadBalancing)
- No other fields are displayed

---

### A15. Secrets Manager -- List View

**Given:** this views.yaml secrets list config:
```yaml
secrets:
  list:
    Secret Name:
      path: Name
      width: 36
    Description:
      path: Description
      width: 30
    Last Accessed:
      path: LastAccessedDate
      width: 18
    Last Changed:
      path: LastChangedDate
      width: 18
    Rotation:
      path: RotationEnabled
      width: 10
```
**When:** user navigates to Secrets Manager resource list
**Then:**
- Column headers appear in order: "Secret Name", "Description", "Last Accessed", "Last Changed", "Rotation"
- Secret Name column is 36 chars wide
- Description column is 30 chars wide
- Last Accessed column is 18 chars wide
- Last Changed column is 18 chars wide
- Rotation column is 10 chars wide
- Secret Name shows names like `prod/api/database-password`
- Description shows free-text descriptions
- Last Accessed and Last Changed show formatted dates
- Rotation shows a boolean value (Yes/No)

**AWS comparison:** `aws secretsmanager list-secrets`
- Name -> `.SecretList[].Name`
- Description -> `.SecretList[].Description`
- LastAccessedDate -> `.SecretList[].LastAccessedDate`
- LastChangedDate -> `.SecretList[].LastChangedDate`
- RotationEnabled -> `.SecretList[].RotationEnabled`

---

### A16. Secrets Manager -- Detail View

**Given:** this views.yaml secrets detail config:
```yaml
secrets:
  detail:
    - Name
    - Description
    - LastAccessedDate
    - LastChangedDate
    - RotationEnabled
    - ARN
    - KmsKeyId
    - Tags
```
**When:** user selects a secret and presses `d`
**Then:**
- Detail view shows exactly 8 fields in this order: Name, Description, LastAccessedDate, LastChangedDate, RotationEnabled, ARN, KmsKeyId, Tags
- ARN shows the full secret ARN
- KmsKeyId shows the KMS key identifier (or blank if default)
- Tags is an array of key-value structs and shows as a section listing each tag's Key and Value
- No other fields are displayed

---

## Section B: Detail Field Ordering Verification

These stories verify that detail fields appear in the EXACT order from views.yaml, not alphabetically or in any default order.

---

### B1. EC2 detail field order matches views.yaml exactly

**Given:** the ec2 detail config lists fields in this order:
1. InstanceId
2. State
3. InstanceType
4. ImageId
5. VpcId
6. SubnetId
7. PrivateIpAddress
8. PublicIpAddress
9. SecurityGroups
10. LaunchTime
11. Architecture
12. Platform
13. Tags

**When:** user views EC2 instance detail
**Then:** fields appear on screen in exactly that order, top to bottom. InstanceId is the first field visible. Tags is the last field (may require scrolling down).

---

### B2. RDS detail field order matches views.yaml exactly

**Given:** the rds detail config lists fields in this order:
1. DBInstanceIdentifier
2. Engine
3. EngineVersion
4. DBInstanceStatus
5. DBInstanceClass
6. Endpoint
7. MultiAZ
8. AllocatedStorage
9. StorageType
10. AvailabilityZone

**When:** user views RDS instance detail
**Then:** fields appear on screen in exactly that order. DBInstanceIdentifier is first. AvailabilityZone is last.

---

### B3. Redis detail field order matches views.yaml exactly

**Given:** the redis detail config lists fields in this order:
1. CacheClusterId
2. Engine
3. EngineVersion
4. CacheClusterStatus
5. CacheNodeType
6. NumCacheNodes
7. ConfigurationEndpoint
8. PreferredAvailabilityZone

**When:** user views Redis cluster detail
**Then:** fields appear in exactly that order. CacheClusterId is first. PreferredAvailabilityZone is last.

---

### B4. DocumentDB detail field order matches views.yaml exactly

**Given:** the docdb detail config lists fields in this order:
1. DBClusterIdentifier
2. Engine
3. EngineVersion
4. Status
5. Endpoint
6. ReaderEndpoint
7. Port
8. StorageEncrypted
9. DBClusterMembers

**When:** user views DocumentDB cluster detail
**Then:** fields appear in exactly that order. DBClusterIdentifier is first. DBClusterMembers is last.

---

### B5. EKS detail field order matches views.yaml exactly

**Given:** the eks detail config lists fields in this order:
1. Name
2. Version
3. Status
4. Endpoint
5. PlatformVersion
6. Arn
7. RoleArn
8. KubernetesNetworkConfig

**When:** user views EKS cluster detail
**Then:** fields appear in exactly that order. Name is first. KubernetesNetworkConfig is last.

---

### B6. Secrets detail field order matches views.yaml exactly

**Given:** the secrets detail config lists fields in this order:
1. Name
2. Description
3. LastAccessedDate
4. LastChangedDate
5. RotationEnabled
6. ARN
7. KmsKeyId
8. Tags

**When:** user views secret detail
**Then:** fields appear in exactly that order. Name is first. Tags is last.

---

### B7. S3 detail field order matches views.yaml exactly

**Given:** the s3 detail config lists fields in this order:
1. BucketArn
2. BucketRegion
3. CreationDate

**When:** user views S3 bucket detail
**Then:** fields appear in exactly that order. BucketArn is first. CreationDate is last.

---

### B8. S3 Objects detail field order matches views.yaml exactly

**Given:** the s3_objects detail config lists fields in this order:
1. Name
2. LastModified
3. Owner

**When:** user views S3 object detail
**Then:** fields appear in exactly that order. Name is first. Owner is last.

---

## Section C: Nested Path Resolution with Real Config

These stories verify that dot-path notation in the actual views.yaml resolves correctly to leaf values.

---

### C1. State.Name (EC2) resolves to the leaf string

**Given:** the ec2 list config uses `path: State.Name` for the "State" column
**When:** the EC2 list displays an instance whose State is `{Name: "running", Code: 16}`
**Then:** the "State" column shows `running` -- just the string value of the Name sub-field, not `{Name: running, Code: 16}` or any struct representation

---

### C2. Endpoint.Address (RDS) resolves to just the hostname

**Given:** the rds list config uses `path: Endpoint.Address` for the "Endpoint" column
**When:** the RDS list displays an instance whose Endpoint is `{Address: "mydb.abc123.us-east-1.rds.amazonaws.com", Port: 3306, HostedZoneId: "Z2R2ITUGPM61AM"}`
**Then:** the "Endpoint" column shows `mydb.abc123.us-east-1.rds.amazonaws.com` -- just the address string, not the port or hosted zone ID

---

### C3. ConfigurationEndpoint.Address (Redis) resolves to just the address

**Given:** the redis list config uses `path: ConfigurationEndpoint.Address` for the "Endpoint" column
**When:** the Redis list displays a cluster whose ConfigurationEndpoint is `{Address: "my-redis.abc123.cfg.use1.cache.amazonaws.com", Port: 6379}`
**Then:** the "Endpoint" column shows `my-redis.abc123.cfg.use1.cache.amazonaws.com` -- just the address, not the port

---

### C4. DBClusterMembers (DocDB list) shows member count, not array dump

**Given:** the docdb list config uses `path: DBClusterMembers` with width 10 for the "Instances" column
**When:** a DocumentDB cluster has 3 members in its DBClusterMembers array
**Then:** the "Instances" column shows `3` (the count), not a JSON/YAML dump of the array contents. The width of 10 is sufficient for displaying a count.

---

### C5. SecurityGroups (EC2 detail) shows as expandable section

**Given:** the ec2 detail config includes `SecurityGroups` in its field list
**When:** user views detail of an EC2 instance that has 2 security groups: `[{GroupId: "sg-abc123", GroupName: "web-servers"}, {GroupId: "sg-def456", GroupName: "ssh-access"}]`
**Then:** the detail view shows a "SecurityGroups" section with each group listed, displaying GroupId and GroupName for each entry -- not a raw array dump

---

### C6. Tags (EC2 detail) shows as key-value pairs

**Given:** the ec2 detail config includes `Tags` in its field list
**When:** user views detail of an EC2 instance that has tags `[{Key: "Name", Value: "api-prod-01"}, {Key: "Environment", Value: "production"}]`
**Then:** the detail view shows a "Tags" section listing each tag with its Key and Value displayed as readable key-value pairs

---

### C7. Tags (Secrets detail) shows as key-value pairs

**Given:** the secrets detail config includes `Tags` in its field list
**When:** user views detail of a secret that has tags `[{Key: "team", Value: "platform"}, {Key: "env", Value: "prod"}]`
**Then:** the detail view shows a "Tags" section listing each tag with its Key and Value displayed as readable key-value pairs

---

### C8. KubernetesNetworkConfig (EKS detail) shows as nested section

**Given:** the eks detail config includes `KubernetesNetworkConfig` in its field list
**When:** user views detail of an EKS cluster whose KubernetesNetworkConfig is `{IpFamily: "ipv4", ServiceIpv4Cidr: "172.20.0.0/16", ElasticLoadBalancing: {Enabled: true}}`
**Then:** the detail view shows a "KubernetesNetworkConfig" section with nested sub-fields: IpFamily, ServiceIpv4Cidr, ServiceIpv6Cidr, and ElasticLoadBalancing (which itself has an Enabled sub-field)

---

### C9. Endpoint (RDS detail) shows full struct with sub-fields

**Given:** the rds detail config includes `Endpoint` (not `Endpoint.Address`) in its field list
**When:** user views detail of an RDS instance whose Endpoint is `{Address: "mydb.abc123.us-east-1.rds.amazonaws.com", Port: 3306, HostedZoneId: "Z2R2ITUGPM61AM"}`
**Then:** the detail view shows an "Endpoint" section expanded to display Address, Port, and HostedZoneId as sub-fields. This contrasts with the list view where `Endpoint.Address` extracts only the address.

---

### C10. ConfigurationEndpoint (Redis detail) shows full struct with sub-fields

**Given:** the redis detail config includes `ConfigurationEndpoint` (not `ConfigurationEndpoint.Address`) in its field list
**When:** user views detail of a Redis cluster
**Then:** the detail view shows a "ConfigurationEndpoint" section with both Address and Port sub-fields visible. This contrasts with the list view where only Address is extracted.

---

### C11. State (EC2 detail) vs State.Name (EC2 list) -- struct vs leaf

**Given:** the ec2 list config uses `path: State.Name` (leaf extraction), while the ec2 detail config includes `State` (full struct)
**When:** user views the EC2 list, then opens detail for the same instance
**Then:**
- In the list view, the "State" column shows just `running` (the leaf value)
- In the detail view, "State" shows expanded sub-fields: Name (`running`) and Code (`16`)
- The same underlying data is presented differently depending on whether the path is a leaf path or a struct path

---

## Section D: Column Width Truncation with Real Widths

These stories use the actual widths from views.yaml to verify truncation behavior.

---

### D1. S3 "Bucket Name" at width 20 -- long name truncated

**Given:** the s3 list config sets "Bucket Name" column to width 20
**When:** a bucket named `my-very-long-bucket-name-example` (33 chars) is displayed
**Then:** the name is truncated to fit within 20 characters, with a truncation indicator (e.g., `my-very-long-buck...`)

---

### D2. S3 "Bucket Name" at width 20 -- short name fits

**Given:** the s3 list config sets "Bucket Name" column to width 20
**When:** a bucket named `my-app-assets` (13 chars) is displayed
**Then:** the name displays fully within the 20-character column with trailing padding

---

### D3. RDS "DB Identifier" at width 28 -- long identifier truncated

**Given:** the rds list config sets "DB Identifier" column to width 28
**When:** an RDS instance named `my-production-database-primary-replica` (38 chars) is displayed
**Then:** the identifier is truncated to fit within 28 characters, with a truncation indicator

---

### D4. EKS "Endpoint" at width 48 -- long URL truncated

**Given:** the eks list config sets "Endpoint" column to width 48
**When:** an EKS cluster has endpoint `https://ABCDEF1234567890ABCDEF1234567890.gr7.us-east-1.eks.amazonaws.com` (72 chars) is displayed
**Then:** the endpoint URL is truncated to fit within 48 characters, with a truncation indicator

---

### D5. Secrets "Secret Name" at width 36 -- path-style name truncated

**Given:** the secrets list config sets "Secret Name" column to width 36
**When:** a secret named `production/services/api/database-credentials` (45 chars) is displayed
**Then:** the name is truncated to fit within 36 characters, with a truncation indicator

---

### D6. Redis "Nodes" at width 8 -- numeric value fits

**Given:** the redis list config sets "Nodes" column to width 8
**When:** a cluster has 3 nodes
**Then:** the value `3` displays fully within the 8-character column. Even small widths are sufficient for numeric counts.

---

### D7. Redis "Endpoint" at width 40 -- long endpoint truncated

**Given:** the redis list config sets "Endpoint" column to width 40
**When:** a cluster has ConfigurationEndpoint.Address `my-redis-cluster.abc123.cfg.use1.cache.amazonaws.com` (52 chars)
**Then:** the endpoint is truncated to fit within 40 characters

---

### D8. EC2 total column widths and terminal width interaction

**Given:** the ec2 list config has columns totaling 100 chars of width (20+12+14+16+16+22)
**When:** the terminal is 80 columns wide
**Then:** the rightmost columns that do not fit are hidden. The user can press `l`/`h` to scroll the column window horizontally. Column headers scroll in sync with data rows.

---

## Section E: Config Modification Scenarios

These stories test what happens when users modify the views.yaml from its current state.

---

### E1. User adds a new column to ec2 list

**Given:** user adds a "VPC" column to the ec2 list config:
```yaml
ec2:
  list:
    Instance ID:
      path: InstanceId
      width: 20
    State:
      path: State.Name
      width: 12
    Type:
      path: InstanceType
      width: 14
    VPC:
      path: VpcId
      width: 24
    Private IP:
      path: PrivateIpAddress
      width: 16
    Public IP:
      path: PublicIpAddress
      width: 16
    Launch Time:
      path: LaunchTime
      width: 22
```
**When:** user restarts a9s and navigates to EC2 list
**Then:** the list now shows 7 columns, with "VPC" appearing between "Type" and "Private IP", displaying VPC IDs like `vpc-0123456789abcdef0`

---

### E2. User removes "Public IP" column from ec2 list

**Given:** user removes the "Public IP" entry from the ec2 list config so it becomes:
```yaml
ec2:
  list:
    Instance ID:
      path: InstanceId
      width: 20
    State:
      path: State.Name
      width: 12
    Type:
      path: InstanceType
      width: 14
    Private IP:
      path: PrivateIpAddress
      width: 16
    Launch Time:
      path: LaunchTime
      width: 22
```
**When:** user restarts a9s and navigates to EC2 list
**Then:** the list now shows 5 columns. There is no "Public IP" column anywhere. Column order is: Instance ID, State, Type, Private IP, Launch Time.

---

### E3. User changes width of "Instance ID" from 20 to 30

**Given:** user changes the ec2 list "Instance ID" width from 20 to 30:
```yaml
Instance ID:
  path: InstanceId
  width: 30
```
**When:** user restarts a9s and navigates to EC2 list
**Then:** the "Instance ID" column is now 30 chars wide, giving more room for the full instance ID without truncation. All other column widths remain as configured.

---

### E4. User reorders columns in ec2 list

**Given:** user reorders the ec2 list config to put Launch Time first:
```yaml
ec2:
  list:
    Launch Time:
      path: LaunchTime
      width: 22
    Instance ID:
      path: InstanceId
      width: 20
    State:
      path: State.Name
      width: 12
    Type:
      path: InstanceType
      width: 14
    Private IP:
      path: PrivateIpAddress
      width: 16
    Public IP:
      path: PublicIpAddress
      width: 16
```
**When:** user restarts a9s and navigates to EC2 list
**Then:** column order is now: Launch Time, Instance ID, State, Type, Private IP, Public IP. The columns follow the YAML key order, not any default or alphabetical ordering.

---

### E5. User adds "PlatformDetails" to ec2 detail

**Given:** user appends `PlatformDetails` to the ec2 detail list:
```yaml
ec2:
  detail:
    - InstanceId
    - State
    - InstanceType
    - ImageId
    - VpcId
    - SubnetId
    - PrivateIpAddress
    - PublicIpAddress
    - SecurityGroups
    - LaunchTime
    - Architecture
    - Platform
    - Tags
    - PlatformDetails
```
**When:** user restarts a9s and views EC2 instance detail
**Then:** the detail view now shows 14 fields. "PlatformDetails" appears as the last field, after "Tags", showing a value like `Linux/UNIX` (which is a valid path per views_reference.yaml).

---

### E6. User removes "Platform" from ec2 detail

**Given:** user removes `Platform` from the ec2 detail list, leaving:
```yaml
ec2:
  detail:
    - InstanceId
    - State
    - InstanceType
    - ImageId
    - VpcId
    - SubnetId
    - PrivateIpAddress
    - PublicIpAddress
    - SecurityGroups
    - LaunchTime
    - Architecture
    - Tags
```
**When:** user restarts a9s and views EC2 instance detail
**Then:** the detail view now shows 12 fields. "Platform" does not appear anywhere. The field list jumps from "Architecture" directly to "Tags".

---

### E7. S3 Objects "Storage Class" is commented out -- column does not appear

**Given:** the actual views.yaml s3_objects list config has Storage Class commented out:
```yaml
s3_objects:
  list:
    Key:
      path: Key
      width: 20
    Size:
      path: Size
      width: 12
    Last Modified:
      path: LastModified
      width: 22
#      Storage Class:
#        path: StorageClass
#        width: 16
```
**When:** user navigates into an S3 bucket to view its objects
**Then:** the object list shows exactly 3 columns: "Key", "Size", "Last Modified". There is no "Storage Class" column. The commented-out YAML is treated as if it does not exist.

---

### E8. User uncomments "Storage Class" in s3_objects

**Given:** user removes the comment markers from the Storage Class config:
```yaml
s3_objects:
  list:
    Key:
      path: Key
      width: 20
    Size:
      path: Size
      width: 12
    Last Modified:
      path: LastModified
      width: 22
    Storage Class:
      path: StorageClass
      width: 16
```
**When:** user restarts a9s and navigates into an S3 bucket
**Then:** the object list now shows 4 columns: "Key", "Size", "Last Modified", "Storage Class". The Storage Class column shows values like `STANDARD`, `GLACIER`, `INTELLIGENT_TIERING`.

---

## Section F: Fallback Behavior with Real Config

These stories verify the interaction between views.yaml presence and built-in defaults.

---

### F1. Full views.yaml with all 8 types -- all use config-driven rendering

**Given:** the current views.yaml has complete list and detail sections for all 8 resource types: s3, s3_objects, ec2, rds, redis, docdb, eks, secrets
**When:** user navigates to each resource type's list and detail views
**Then:** every resource type uses its views.yaml configuration. No resource type falls back to built-in defaults. Column counts, headers, widths, field orders -- all match the config exactly.

---

### F2. Delete the ec2 section -- EC2 falls back, others remain

**Given:** user removes the entire `ec2:` block from views.yaml, leaving s3, s3_objects, rds, redis, docdb, eks, and secrets intact
**When:** user restarts a9s and navigates through all resource types
**Then:**
- EC2 list shows its built-in default columns (not the configured 6-column layout)
- EC2 detail shows its built-in default fields
- S3 list still shows 2 configured columns: "Bucket Name", "Creation Date"
- RDS list still shows 7 configured columns: "DB Identifier", "Engine", "Version", "Status", "Class", "Endpoint", "Multi-AZ"
- All other configured resource types continue to use their views.yaml configurations

---

### F3. Delete entire views.yaml -- all types use built-in defaults

**Given:** the views.yaml file is deleted or does not exist
**When:** user launches a9s and navigates through all resource types
**Then:**
- Every resource type uses its built-in default columns and detail fields
- No error is displayed about missing configuration
- The application works normally with default layouts for all 8 types

---

### F4. views.yaml with only s3 and ec2 -- those use config, rest use defaults

**Given:** views.yaml contains only:
```yaml
views:
  s3:
    list:
      Bucket Name:
        path: Name
        width: 20
      Creation Date:
        path: CreationDate
        width: 22
    detail:
      - BucketArn
      - BucketRegion
      - CreationDate
  ec2:
    list:
      Instance ID:
        path: InstanceId
        width: 20
      State:
        path: State.Name
        width: 12
      Type:
        path: InstanceType
        width: 14
      Private IP:
        path: PrivateIpAddress
        width: 16
      Public IP:
        path: PublicIpAddress
        width: 16
      Launch Time:
        path: LaunchTime
        width: 22
    detail:
      - InstanceId
      - State
      - InstanceType
      - ImageId
      - VpcId
      - SubnetId
      - PrivateIpAddress
      - PublicIpAddress
      - SecurityGroups
      - LaunchTime
      - Architecture
      - Platform
      - Tags
```
**When:** user navigates through all resource types
**Then:**
- S3 list shows 2 configured columns; S3 detail shows 3 configured fields
- EC2 list shows 6 configured columns; EC2 detail shows 13 configured fields
- RDS list and detail use built-in defaults
- Redis list and detail use built-in defaults
- DocumentDB list and detail use built-in defaults
- EKS list and detail use built-in defaults
- Secrets list and detail use built-in defaults
- S3 Objects list and detail use built-in defaults

---

### F5. views.yaml with list but no detail for ec2 -- detail falls back

**Given:** views.yaml ec2 section has only list, no detail:
```yaml
views:
  ec2:
    list:
      Instance ID:
        path: InstanceId
        width: 20
      State:
        path: State.Name
        width: 12
      Type:
        path: InstanceType
        width: 14
      Private IP:
        path: PrivateIpAddress
        width: 16
      Public IP:
        path: PublicIpAddress
        width: 16
      Launch Time:
        path: LaunchTime
        width: 22
```
**When:** user opens EC2 list, selects an instance, and presses `d`
**Then:**
- EC2 list shows the 6 configured columns from views.yaml
- EC2 detail falls back to built-in default fields (since no detail section is configured)

---

### F6. views.yaml with detail but no list for rds -- list falls back

**Given:** views.yaml rds section has only detail, no list:
```yaml
views:
  rds:
    detail:
      - DBInstanceIdentifier
      - Engine
      - EngineVersion
      - DBInstanceStatus
      - DBInstanceClass
      - Endpoint
      - MultiAZ
      - AllocatedStorage
      - StorageType
      - AvailabilityZone
```
**When:** user opens RDS list, selects an instance, and presses `d`
**Then:**
- RDS list shows built-in default columns (since no list section is configured)
- RDS detail shows the 10 configured fields from views.yaml

---

## Section G: Column Header Display Names vs Paths

These stories verify that column headers use the YAML map key, not the path.

---

### G1. EC2 column headers use display names, not paths

**Given:** the ec2 list config has these key-to-path mappings:
- "Instance ID" -> `InstanceId`
- "State" -> `State.Name`
- "Type" -> `InstanceType`
- "Private IP" -> `PrivateIpAddress`
- "Public IP" -> `PublicIpAddress`
- "Launch Time" -> `LaunchTime`

**When:** user views the EC2 list
**Then:** column headers read "Instance ID", "State", "Type", "Private IP", "Public IP", "Launch Time" -- NOT "InstanceId", "State.Name", "InstanceType", "PrivateIpAddress", "PublicIpAddress", "LaunchTime"

---

### G2. RDS column headers use display names, not paths

**Given:** the rds list config has these key-to-path mappings:
- "DB Identifier" -> `DBInstanceIdentifier`
- "Engine" -> `Engine`
- "Version" -> `EngineVersion`
- "Status" -> `DBInstanceStatus`
- "Class" -> `DBInstanceClass`
- "Endpoint" -> `Endpoint.Address`
- "Multi-AZ" -> `MultiAZ`

**When:** user views the RDS list
**Then:** column headers read "DB Identifier", "Engine", "Version", "Status", "Class", "Endpoint", "Multi-AZ" -- NOT the raw AWS SDK path names

---

### G3. DocDB "Instances" header maps to DBClusterMembers path

**Given:** the docdb list config maps "Instances" -> `DBClusterMembers`
**When:** user views the DocumentDB list
**Then:** the column header reads "Instances", not "DBClusterMembers". The display name is a user-friendly alias for the underlying array field.

---

### G4. Redis "Nodes" header maps to NumCacheNodes path

**Given:** the redis list config maps "Nodes" -> `NumCacheNodes`
**When:** user views the Redis list
**Then:** the column header reads "Nodes", not "NumCacheNodes"

---

## Section H: YAML View is Not Affected by views.yaml

---

### H1. YAML view shows all fields regardless of views.yaml config

**Given:** the ec2 detail config only lists 13 specific fields
**When:** user selects an EC2 instance and presses `y` for YAML view
**Then:** the YAML view shows the complete AWS resource as YAML with ALL fields (AmiLaunchIndex, Architecture, BlockDeviceMappings, BootMode, CpuOptions, EbsOptimized, and so on), not just the 13 fields configured in the detail section. The views.yaml configuration only affects list and detail views.

---

### H2. YAML view for RDS shows all fields regardless of views.yaml config

**Given:** the rds detail config only lists 10 specific fields
**When:** user selects an RDS instance and presses `y` for YAML view
**Then:** the YAML view shows the complete RDS instance as YAML with ALL fields (including ActivityStreamMode, BackupRetentionPeriod, CACertificateIdentifier, DBSubnetGroup, etc.), not just the 10 configured detail fields.

---

## Section I: Row Coloring Interacts with Config Columns

---

### I1. EC2 rows colored by State.Name despite custom column set

**Given:** the ec2 list config includes `State` column with `path: State.Name`
**When:** the EC2 list displays instances with various states
**Then:**
- Rows with State.Name = "running" display in green
- Rows with State.Name = "stopped" display in red
- Rows with State.Name = "terminated" display dimmed
- Rows with State.Name = "pending" display in yellow
- Row coloring applies to the entire row, not just the State column
- The selected row overrides state coloring with blue background

---

### I2. RDS rows colored by DBInstanceStatus

**Given:** the rds list config includes `Status` column with `path: DBInstanceStatus`
**When:** the RDS list displays instances with various statuses
**Then:**
- Rows with DBInstanceStatus = "available" display in green
- Rows with DBInstanceStatus = "stopped" display in red
- Rows with DBInstanceStatus = "creating" display in yellow

---

## Section J: Edge Cases Specific to Real Config

---

### J1. EC2 instance with no public IP -- blank cell in configured column

**Given:** the ec2 list config includes "Public IP" with `path: PublicIpAddress` at width 16
**When:** an EC2 instance has no public IP assigned (PublicIpAddress is nil)
**Then:** the "Public IP" column shows a blank cell, not `<nil>`, `null`, or `none`

---

### J2. RDS instance with nil Endpoint -- Endpoint.Address shows blank

**Given:** the rds list config uses `path: Endpoint.Address` for the "Endpoint" column
**When:** an RDS instance is still being created and has no Endpoint yet (Endpoint is nil)
**Then:** the "Endpoint" column shows a blank cell. The nested path resolution handles nil intermediate objects gracefully.

---

### J3. Redis cluster with nil ConfigurationEndpoint -- shows blank

**Given:** the redis list config uses `path: ConfigurationEndpoint.Address` for the "Endpoint" column
**When:** a Redis cluster in Cluster Mode Disabled has no ConfigurationEndpoint (it is nil)
**Then:** the "Endpoint" column shows a blank cell, not an error or crash

---

### J4. Secrets with no Description -- blank cell

**Given:** the secrets list config includes "Description" with `path: Description` at width 30
**When:** a secret has no description set (Description is nil or empty string)
**Then:** the "Description" column shows a blank cell

---

### J5. EKS cluster with long Endpoint and width 48

**Given:** the eks list config sets "Endpoint" to width 48
**When:** an EKS endpoint URL is 72 characters long
**Then:** the URL is truncated at 48 characters with a truncation indicator. The truncation does not break mid-character or cause layout misalignment with other rows.

---

### J6. DocDB cluster with zero members -- Instances shows 0

**Given:** the docdb list config maps "Instances" to `DBClusterMembers` at width 10
**When:** a DocumentDB cluster has an empty DBClusterMembers array
**Then:** the "Instances" column shows `0`, not blank or an error

---

### J7. Case-insensitive path matching with real config paths

**Given:** the views.yaml uses PascalCase paths like `InstanceId`, `State.Name`, `DBInstanceIdentifier`
**When:** a user changes a path to lowercase like `instanceid` or `state.name`
**Then:** the path still resolves correctly. The header comment in views.yaml states "Paths use AWS SDK Go struct field names (case-insensitive matching)."

---

## Section K: Config Loading Precedence

---

### K1. views.yaml in current directory is loaded

**Given:** the project's views.yaml exists at `./views.yaml` (the current working directory)
**When:** user launches a9s from that directory
**Then:** the S3 list shows exactly 2 columns ("Bucket Name", "Creation Date"), the EC2 list shows exactly 6 columns, etc. -- all matching the project's views.yaml content

---

### K2. Syntax error in views.yaml -- fallback to defaults with error

**Given:** the views.yaml is corrupted with invalid YAML:
```
views:
  ec2:
    list:
      - this is wrong: [[[
```
**When:** user launches a9s
**Then:** the application falls back to built-in defaults for all resource types. An error message is shown indicating the configuration file could not be parsed.

---

### K3. Empty views.yaml -- defaults used, no error

**Given:** views.yaml exists but contains only a comment:
```yaml
# Empty configuration
```
**When:** user launches a9s and opens any resource list
**Then:** all resource types use built-in default columns and detail fields. No error is displayed.

---

### K4. views.yaml with unknown resource type -- ignored gracefully

**Given:** views.yaml contains a `lambda:` section alongside the real `ec2:` section:
```yaml
views:
  lambda:
    list:
      Name:
        path: FunctionName
        width: 30
  ec2:
    list:
      Instance ID:
        path: InstanceId
        width: 20
```
**When:** user launches a9s
**Then:** the `lambda` section is silently ignored. The `ec2` section is applied normally. No crash or error about the unknown type.

---

### K5. Configuration is loaded once at startup -- no hot reload

**Given:** user has a9s running with the current views.yaml loaded. While a9s is running, user edits views.yaml to add a new column to ec2.
**When:** user navigates away from EC2 and back
**Then:** the EC2 list still shows the original 6 columns. The change does not take effect until a9s is restarted.

---

## Section L: Reference File Completeness

---

### L1. Every path used in views.yaml exists in views_reference.yaml

**Given:** the views.yaml uses these paths across all resource types:
- s3: `Name`, `CreationDate`
- s3_objects: `Key`, `Size`, `LastModified`
- ec2: `InstanceId`, `State.Name`, `InstanceType`, `PrivateIpAddress`, `PublicIpAddress`, `LaunchTime`
- rds: `DBInstanceIdentifier`, `Engine`, `EngineVersion`, `DBInstanceStatus`, `DBInstanceClass`, `Endpoint.Address`, `MultiAZ`
- redis: `CacheClusterId`, `EngineVersion`, `CacheNodeType`, `CacheClusterStatus`, `NumCacheNodes`, `ConfigurationEndpoint.Address`
- docdb: `DBClusterIdentifier`, `EngineVersion`, `Status`, `DBClusterMembers`, `Endpoint`
- eks: `Name`, `Version`, `Status`, `Endpoint`, `PlatformVersion`
- secrets: `Name`, `Description`, `LastAccessedDate`, `LastChangedDate`, `RotationEnabled`

**When:** user cross-references these paths against views_reference.yaml
**Then:** every path used in views.yaml exists in views_reference.yaml (directly or as a parent of listed sub-paths). No path is fabricated or unavailable.

---

### L2. User can pick any path from views_reference.yaml and use it

**Given:** views_reference.yaml lists `CpuOptions.CoreCount` under ec2
**When:** user adds it to views.yaml:
```yaml
ec2:
  list:
    CPU Cores:
      path: CpuOptions.CoreCount
      width: 10
```
**Then:** the EC2 list shows a "CPU Cores" column that resolves the path to show the core count (or blank if nil on that instance)

---

### L3. Commented-out StorageClass in s3_objects is a valid reference path

**Given:** the s3_objects list has `StorageClass` commented out, and views_reference.yaml lists `StorageClass` as a valid path for s3_objects
**When:** user uncomments the Storage Class column
**Then:** it resolves correctly, showing values like `STANDARD`, `GLACIER`, `INTELLIGENT_TIERING`
