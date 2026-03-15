# Data Model: a9s — Terminal UI AWS Resource Manager

**Branch**: `001-aws-tui-manager` | **Date**: 2026-03-15

## Core Entities

### AppState

The root application state containing all runtime data.

| Field          | Type            | Description                              |
|----------------|-----------------|------------------------------------------|
| CurrentView    | ViewType (enum) | Active view (MainMenu, ResourceList, Detail, ProfileSelect, RegionSelect) |
| ActiveProfile  | string          | Current AWS profile name                 |
| ActiveRegion   | string          | Current AWS region code                  |
| Breadcrumbs    | []string        | Navigation path segments                 |
| History        | NavigationStack | Back/forward navigation history          |
| StatusMessage  | string          | Current status bar message               |
| Loading        | bool            | Whether an async operation is in progress |
| Error          | string          | Current error message (empty if none)    |
| Filter         | string          | Active filter text (empty if none)       |

### AWSProfile

Represents a named AWS configuration profile.

| Field     | Type   | Description                                |
|-----------|--------|--------------------------------------------|
| Name      | string | Profile name as it appears in config       |
| Region    | string | Default region for this profile (if set)   |
| IsSSO     | bool   | Whether this profile uses SSO              |
| AccountID | string | AWS account ID (if available from config)  |
| RoleARN   | string | Assumed role ARN (if applicable)           |
| Source    | string | "config" or "credentials" (file of origin) |

**Identity**: Name (unique across config + credentials files)

### AWSRegion

Represents an AWS geographic region.

| Field       | Type   | Description                     |
|-------------|--------|---------------------------------|
| Code        | string | Region code (e.g., us-east-1)   |
| DisplayName | string | Human name (e.g., US East (N. Virginia)) |

**Identity**: Code (unique)
**Source**: Hardcoded list, updatable in future releases.

### ResourceType

Defines a category of AWS resources the app can browse.

| Field          | Type       | Description                            |
|----------------|------------|----------------------------------------|
| Name           | string     | Display name (e.g., "EC2 Instances")   |
| ShortName      | string     | Colon command alias (e.g., "ec2")      |
| Aliases        | []string   | Alternative command names              |
| Columns        | []Column   | Table columns for list view            |
| DetailFields   | []string   | Fields shown in describe view          |
| FetchFunc      | function   | Function to fetch resources from AWS   |
| DescribeFunc   | function   | Function to fetch single resource detail |

**Identity**: ShortName (unique)

### Column

Defines a column in a resource table view.

| Field     | Type   | Description                              |
|-----------|--------|------------------------------------------|
| Key       | string | Field key used to extract value          |
| Title     | string | Column header display text               |
| Width     | int    | Fixed width (0 = flexible)               |
| Sortable  | bool   | Whether this column supports sorting     |
| Hidden    | bool   | Whether hidden by default (wide mode)    |

### Resource

A generic AWS resource instance displayed in a table row.

| Field      | Type              | Description                        |
|------------|-------------------|------------------------------------|
| ID         | string            | Primary identifier (instance ID, ARN, name) |
| Name       | string            | Display name (from Name tag or identifier) |
| Status     | string            | Current state/status               |
| Fields     | map[string]string | All visible column values by key   |
| RawJSON    | string            | Raw JSON for `y` (JSON view)       |
| DetailData | map[string]string | All attributes for describe view   |

**Identity**: ID (unique within a resource type + region)

### NavigationStack

Manages back/forward navigation history.

| Field   | Type         | Description                       |
|---------|--------------|-----------------------------------|
| Back    | []ViewState  | Previous views (push on navigate) |
| Forward | []ViewState  | Views after going back            |

### ViewState

A snapshot of a view for navigation history.

| Field        | Type     | Description                       |
|--------------|----------|-----------------------------------|
| ViewType     | enum     | Which view was active             |
| ResourceType | string   | Which resource type (if applicable) |
| CursorPos    | int      | Cursor position in the list       |
| Filter       | string   | Active filter text                |
| S3Prefix     | string   | Current S3 prefix (if S3 browsing) |

## Resource Type Definitions

### S3 Bucket

| Column        | Source Field   | Notes                     |
|---------------|----------------|---------------------------|
| Bucket Name   | Name           | Primary identifier        |
| Region        | BucketRegion   | Via GetBucketLocation     |
| Creation Date | CreationDate   | ISO format                |

**Drill-down**: Enter → S3 Object list (prefix browsing)

### S3 Object

| Column        | Source Field  | Notes                     |
|---------------|---------------|---------------------------|
| Key           | Key           | Relative to current prefix |
| Size          | Size          | Human-readable (KB/MB/GB) |
| Last Modified | LastModified  | Relative time             |
| Storage Class | StorageClass  | STANDARD, IA, GLACIER, etc. |

**Prefix entries**: CommonPrefixes shown as folder-type rows.

### EC2 Instance

| Column      | Source Field       | Notes                     |
|-------------|---------------------|---------------------------|
| Instance ID | InstanceId          | Primary identifier        |
| Name        | Tags[Name]          | Name tag value            |
| State       | State.Name          | Color-coded by status     |
| Type        | InstanceType        |                           |
| Private IP  | PrivateIpAddress    |                           |
| Public IP   | PublicIpAddress     | Empty if none             |
| Launch Time | LaunchTime          | Relative time             |

### RDS Instance

| Column       | Source Field          | Notes                  |
|--------------|-----------------------|------------------------|
| DB Identifier | DBInstanceIdentifier | Primary identifier     |
| Engine       | Engine                | mysql, postgres, etc.  |
| Version      | EngineVersion         |                        |
| Status       | DBInstanceStatus      | Color-coded            |
| Class        | DBInstanceClass       |                        |
| Endpoint     | Endpoint.Address      |                        |
| Multi-AZ     | MultiAZ               | Yes/No                 |

### ElastiCache Redis Cluster

| Column       | Source Field            | Notes                 |
|--------------|-------------------------|-----------------------|
| Cluster ID   | CacheClusterId          | Primary identifier    |
| Version      | EngineVersion           |                       |
| Node Type    | CacheNodeType           |                       |
| Status       | CacheClusterStatus      | Color-coded           |
| Nodes        | NumCacheNodes           | Count                 |
| Endpoint     | ConfigurationEndpoint   |                       |

**Filter**: Client-side filter `Engine == "redis"`.

### DocumentDB Cluster

| Column        | Source Field           | Notes                 |
|---------------|------------------------|-----------------------|
| Cluster ID    | DBClusterIdentifier    | Primary identifier    |
| Version       | EngineVersion          |                       |
| Status        | Status                 | Color-coded           |
| Instances     | DBClusterMembers count |                       |
| Endpoint      | Endpoint               |                       |

**Filter**: Server-side filter `engine=docdb`.

### EKS Cluster

| Column          | Source Field     | Notes                  |
|-----------------|------------------|------------------------|
| Cluster Name    | Name             | Primary identifier     |
| Version         | Version          | Kubernetes version     |
| Status          | Status           | Color-coded            |
| Endpoint        | Endpoint         | API server endpoint    |
| Platform Version | PlatformVersion |                        |

**Two-step fetch**: ListClusters → DescribeCluster per name.

### Secrets Manager Secret

| Column          | Source Field      | Notes                 |
|-----------------|-------------------|-----------------------|
| Secret Name     | Name              | Primary identifier    |
| Description     | Description       |                       |
| Last Accessed   | LastAccessedDate  | Relative time         |
| Last Changed    | LastChangedDate   | Relative time         |
| Rotation        | RotationEnabled   | Yes/No                |

**Reveal (`x`)**: Calls GetSecretValue, displays SecretString
as plain text.

## State Transitions

### View Navigation

```
Launch → MainMenu
MainMenu + Enter/Command → ResourceList
ResourceList + d → DetailView
ResourceList + Enter (S3) → S3ObjectList
S3ObjectList + Enter (prefix) → S3ObjectList (deeper)
ResourceList + x (secrets) → RevealView
Any + :ctx → ProfileSelect
Any + :region → RegionSelect
Any + :main → MainMenu
Any + Escape → Previous view
Any + [ → History back
Any + ] → History forward
ProfileSelect + Enter → MainMenu (new profile)
RegionSelect + Enter → Previous view (new region)
```

### Loading States

```
Idle → [trigger fetch] → Loading → [response] → Idle
Idle → [trigger fetch] → Loading → [error] → Error → Idle
```
