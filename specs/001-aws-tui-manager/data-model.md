# Data Model: a9s — Terminal UI AWS Resource Manager

**Branch**: `001-aws-tui-manager` | **Date**: 2026-03-15

## Core Entities

### AppState

The root application state containing all runtime data.

| Field               | Type            | Description                              |
|---------------------|-----------------|------------------------------------------|
| CurrentView         | ViewType (enum) | Active view (MainMenu, ResourceList, Detail, JSONView, RevealView, ProfileSelect, RegionSelect) |
| ActiveProfile       | string          | Current AWS profile name                 |
| ActiveRegion        | string          | Current AWS region code                  |
| Breadcrumbs         | []string        | Navigation path segments                 |
| History             | NavigationStack | Back/forward navigation history          |
| StatusMessage       | string          | Current status bar message               |
| StatusIsError       | bool            | Whether StatusMessage is an error        |
| Loading             | bool            | Whether an async operation is in progress |
| Filter              | string          | Active filter text (empty if none)       |
| FilterMode          | bool            | Whether filter input mode is active      |
| CommandMode         | bool            | Whether command input mode is active     |
| CommandText         | string          | Current command input text               |
| CurrentResourceType | string          | ShortName of the active resource type    |
| SelectedIndex       | int             | Cursor position in the current list      |
| S3Bucket            | string          | Current S3 bucket for object browsing    |
| S3Prefix            | string          | Current S3 prefix for folder browsing    |
| HScrollOffset       | int             | Horizontal scroll offset for wide tables |
| ShowHelp            | bool            | Whether the help overlay is visible      |

### AWSProfile

In the current implementation, profiles are represented as plain
strings (profile names). The `ListProfiles()` function returns
`[]string` — a deduplicated, sorted list of profile names merged
from `~/.aws/config` and `~/.aws/credentials`. There is no
structured `AWSProfile` type with the fields below.

The region for a selected profile is resolved on demand via
`GetDefaultRegion(configPath, profileName)`.

**Conceptual fields** (not implemented as a struct):

| Field     | Type   | Description                                |
|-----------|--------|--------------------------------------------|
| Name      | string | Profile name as it appears in config       |
| Region    | string | Default region for this profile (resolved on demand) |

**Identity**: Name (unique across config + credentials files)

### AWSRegion

Represents an AWS geographic region.

| Field       | Type   | Description                     |
|-------------|--------|---------------------------------|
| Code        | string | Region code (e.g., us-east-1)   |
| DisplayName | string | Human name (e.g., US East (N. Virginia)) |

**Identity**: Code (unique)
**Source**: Hardcoded list, updatable in future releases.

### ResourceType (ResourceTypeDef)

Defines a category of AWS resources the app can browse.

| Field          | Type       | Description                            |
|----------------|------------|----------------------------------------|
| Name           | string     | Display name (e.g., "EC2 Instances")   |
| ShortName      | string     | Colon command alias (e.g., "ec2")      |
| Aliases        | []string   | Alternative command names              |
| Columns        | []Column   | Table columns for list view            |

**Identity**: ShortName (unique)

Fetch and describe functions are not stored in the type definition.
Instead, `app.go` dispatches to the correct `internal/aws/*.go`
fetch function based on `CurrentResourceType` (a switch statement
in `fetchResources()`). Detail data is stored in
`Resource.DetailData` at fetch time.

### Column

Defines a column in a resource table view.

| Field     | Type   | Description                              |
|-----------|--------|------------------------------------------|
| Key       | string | Field key used to extract value          |
| Title     | string | Column header display text               |
| Width     | int    | Fixed width (0 = flexible)               |
| Sortable  | bool   | Whether this column supports sorting     |

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
| S3Bucket     | string   | Current S3 bucket (if S3 browsing) |
| S3Prefix     | string   | Current S3 prefix (if S3 browsing) |

## Resource Type Definitions

### S3 Bucket

| Column        | Source Field   | Notes                     |
|---------------|----------------|---------------------------|
| Bucket Name   | Name           | Primary identifier        |
| Creation Date | CreationDate   | ISO format                |

Region is **not** included in the bucket list view because the S3
ListBuckets API does not return per-bucket region information.
Region is only available in the single-bucket detail view
(via GetBucketLocation).

**Drill-down**: Enter → S3 Object list (prefix browsing).
When inside a bucket, the table switches to S3 Object columns
(Key, Size, Last Modified, Storage Class) instead of the bucket
list columns above.

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
ResourceList + y → JSONView
ResourceList + Enter (S3 bucket) → S3ObjectList (inside bucket)
S3ObjectList + Enter (prefix) → S3ObjectList (deeper prefix)
S3ObjectList + Enter (file) → DetailView
ResourceList + Enter (non-S3) → DetailView
ResourceList + x (secrets) → RevealView
Any + :ctx → ProfileSelect
Any + :region → RegionSelect
Any + :main → MainMenu
Any + Escape → Previous view (via history pop)
Any + q → Quit (main menu) / Previous view (other views)
Any + [ → History back
Any + ] → History forward
ProfileSelect + Enter → MainMenu (new profile, clients recreated)
RegionSelect + Enter → MainMenu (new region, clients recreated)
```

### Loading States

```
Idle → [trigger fetch] → Loading → [response] → Idle
Idle → [trigger fetch] → Loading → [error] → Error → [5s auto-clear] → Idle
```

### Implementation Details

**Stale response guard**: When a `ResourcesLoadedMsg` arrives, the
handler checks that `msg.ResourceType` matches `m.CurrentResourceType`.
If the user navigated away before the response arrived, the stale
response is silently discarded. This prevents data from a previous
resource type overwriting the current view.

**Error auto-clear**: API errors are displayed in the status bar and
automatically cleared after 5 seconds via `tea.Tick` + `ClearErrorMsg`.
If the error has already been replaced by a non-error status message,
the clear is a no-op.
