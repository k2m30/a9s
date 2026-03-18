# QA User Stories: Redis (ElastiCache) & DocumentDB Views

Covers LIST, DETAIL, and YAML views for Redis and DocumentDB resources.

---

## REDIS (ElastiCache)

### REDIS-LIST-01: Navigate to Redis list from main menu

**Given** the main menu is displayed with resource types listed
**When** I select "ElastiCache Redis" and press Enter (or type `:redis` and press Enter)
**Then** a loading spinner appears with a message like "Fetching ElastiCache clusters..."
**And** the frame title shows `redis` while loading

---

### REDIS-LIST-02: Redis list displays correct columns

**Given** the Redis list view has loaded with at least one cluster
**Then** the table header shows these columns in order: Cluster ID, Version, Node Type, Status, Nodes, Endpoint
**And** column widths are approximately: 28, 10, 18, 14, 8, 40 characters
**And** column headers are bold blue (#7aa2f7) with no separator line beneath them

---

### REDIS-LIST-03: Redis list populates column data from correct API fields

**Given** the Redis list view has loaded
**Then** the Cluster ID column shows the value from `CacheClusterId`
**And** the Version column shows the value from `EngineVersion`
**And** the Node Type column shows the value from `CacheNodeType`
**And** the Status column shows the value from `CacheClusterStatus`
**And** the Nodes column shows the value from `NumCacheNodes`
**And** the Endpoint column shows the value from `ConfigurationEndpoint.Address`

---

### REDIS-LIST-04: Redis list row count appears in frame title

**Given** the Redis list view has loaded with N clusters
**Then** the frame title reads `redis(N)` centered in the top border
**And** the count matches the actual number of rows displayed

---

### REDIS-LIST-05: Redis list row coloring by status

**Given** the Redis list view has loaded with clusters in various states
**Then** rows where Status is "available" are colored green (#9ece6a)
**And** rows where Status is "creating" are colored yellow (#e0af68)
**And** rows where Status is "deleting" are colored red (#f7768e)
**And** the currently selected row overrides any status color with a blue background (#7aa2f7) and dark foreground (#1a1b26)

---

### REDIS-LIST-06: Redis list cursor navigation

**Given** the Redis list view is displayed with multiple clusters
**When** I press `j` or Down arrow
**Then** the selection cursor moves down one row
**When** I press `k` or Up arrow
**Then** the selection cursor moves up one row
**When** I press `g`
**Then** the selection jumps to the first row
**When** I press `G`
**Then** the selection jumps to the last row

---

### REDIS-LIST-07: Redis list horizontal scroll for wide content

**Given** the Redis list is displayed in a terminal narrower than the total column width (28+10+18+14+8+40 = 118 characters plus borders)
**When** I press `l` or Right arrow
**Then** the visible columns shift right to reveal columns that were off-screen
**When** I press `h` or Left arrow
**Then** the visible columns shift left
**And** the column header scrolls in sync with the data rows

---

### REDIS-LIST-08: Redis list filter

**Given** the Redis list view is displayed with multiple clusters
**When** I press `/` and type a search term (e.g., "prod")
**Then** the header right side shows `/prod` in amber bold (#e0af68)
**And** only rows whose visible text contains "prod" (case-insensitive) are shown
**And** the frame title updates to `redis(matched/total)` (e.g., `redis(2/7)`)
**When** I press Escape
**Then** the filter is cleared and all rows reappear

---

### REDIS-LIST-09: Redis list sorting

**Given** the Redis list view is displayed
**When** I press `N`
**Then** the rows sort by the name/ID column (Cluster ID) in ascending order and a `^` or up-arrow indicator appears on that column header
**When** I press `N` again
**Then** the sort order toggles to descending and the indicator changes accordingly
**When** I press `S`
**Then** the rows sort by the Status column

---

### REDIS-LIST-10: Redis list copy cluster ID

**Given** the Redis list view is displayed with a cluster selected
**When** I press `c`
**Then** the CacheClusterId of the selected cluster is copied to the system clipboard
**And** the header right side briefly shows "Copied!" in green (#9ece6a) for approximately 2 seconds

---

### REDIS-LIST-11: Redis list with no clusters

**Given** the AWS account has no ElastiCache Redis clusters in the current region
**When** the Redis list view finishes loading
**Then** a centered empty-state message is displayed (e.g., hint to refresh or change region)
**And** the frame title shows `redis(0)`

---

### REDIS-LIST-12: Redis list with null ConfigurationEndpoint

**Given** a Redis cluster is in cluster mode disabled (single-node) and has no ConfigurationEndpoint
**When** the Redis list view loads
**Then** the Endpoint column for that cluster displays an empty string or a dash, not "null" or an error

---

### REDIS-DETAIL-01: Open Redis detail view from list

**Given** the Redis list view is displayed with a cluster selected
**When** I press Enter or `d`
**Then** the view transitions to the detail view for that cluster
**And** the frame title shows the selected CacheClusterId centered in the top border

---

### REDIS-DETAIL-02: Redis detail displays correct fields

**Given** the Redis detail view is displayed for a cluster
**Then** the following fields are shown as key-value pairs:
  - CacheClusterId
  - Engine
  - EngineVersion
  - CacheClusterStatus
  - CacheNodeType
  - NumCacheNodes
  - ConfigurationEndpoint
  - PreferredAvailabilityZone
**And** keys are colored blue (#7aa2f7) and values are colored plain white (#c0caf5)

---

### REDIS-DETAIL-03: Redis detail ConfigurationEndpoint shows nested fields

**Given** the Redis detail view is displayed for a cluster that has a ConfigurationEndpoint
**Then** the ConfigurationEndpoint field displays its sub-fields (Address and Port)
**And** sub-fields are indented relative to the parent key

---

### REDIS-DETAIL-04: Redis detail status coloring

**Given** the Redis detail view is displayed
**Then** the CacheClusterStatus value "available" is rendered in green (#9ece6a)
**And** the CacheClusterStatus value "creating" is rendered in yellow (#e0af68)
**And** the CacheClusterStatus value "deleting" is rendered in red (#f7768e)

---

### REDIS-DETAIL-05: Redis detail scroll

**Given** the Redis detail view content is taller than the visible frame
**When** I press `j` or Down arrow
**Then** the detail content scrolls down one line
**When** I press `k` or Up arrow
**Then** the detail content scrolls up one line
**When** I press `g`
**Then** the view scrolls to the top
**When** I press `G`
**Then** the view scrolls to the bottom

---

### REDIS-DETAIL-06: Redis detail copy

**Given** the Redis detail view is displayed
**When** I press `c`
**Then** the full detail content is copied to the system clipboard
**And** the header shows "Copied!" in green for approximately 2 seconds

---

### REDIS-DETAIL-07: Redis detail back navigation

**Given** the Redis detail view is displayed
**When** I press Escape
**Then** the view returns to the Redis list view
**And** the previously selected row is still highlighted

---

### REDIS-DETAIL-08: Redis detail with null ConfigurationEndpoint

**Given** the Redis detail view is displayed for a cluster without a ConfigurationEndpoint (cluster mode disabled)
**Then** the ConfigurationEndpoint field shows a null/empty indicator, not an error or crash

---

### REDIS-YAML-01: Open Redis YAML view from list

**Given** the Redis list view is displayed with a cluster selected
**When** I press `y`
**Then** the view transitions to the YAML view for that cluster
**And** the frame title shows `<CacheClusterId> yaml` centered in the top border

---

### REDIS-YAML-02: Open Redis YAML view from detail

**Given** the Redis detail view is displayed
**When** I press `y`
**Then** the view transitions to the YAML view for that cluster

---

### REDIS-YAML-03: Redis YAML view displays full resource data

**Given** the Redis YAML view is displayed for a cluster
**Then** the full AWS API response for that cluster is rendered as syntax-highlighted YAML
**And** keys are colored blue (#7aa2f7)
**And** string values are colored green (#9ece6a)
**And** numeric values are colored orange (#ff9e64)
**And** boolean values are colored purple (#bb9af7)
**And** null values are colored dim (#565f89)
**And** indent/tree connector lines are colored dim (#414868)

---

### REDIS-YAML-04: Redis YAML view shows all available fields

**Given** the Redis YAML view is displayed
**Then** the YAML output includes all non-nil fields from the API response, including but not limited to:
  - ARN
  - CacheClusterId
  - CacheClusterStatus
  - CacheNodeType
  - CacheNodes (as a YAML array with nested objects)
  - Engine
  - EngineVersion
  - NumCacheNodes
  - ConfigurationEndpoint (with Address and Port sub-keys)
  - PreferredAvailabilityZone
  - SecurityGroups (as a YAML array)

---

### REDIS-YAML-05: Redis YAML view scroll

**Given** the Redis YAML view content extends beyond the visible frame
**When** I press `j` or Down arrow
**Then** the YAML content scrolls down
**When** I press `k` or Up arrow
**Then** the YAML content scrolls up
**When** I press `g`
**Then** the view scrolls to the top
**When** I press `G`
**Then** the view scrolls to the bottom

---

### REDIS-YAML-06: Redis YAML view copy

**Given** the Redis YAML view is displayed
**When** I press `c`
**Then** the full YAML text is copied to the system clipboard
**And** the header shows "Copied!" in green for approximately 2 seconds

---

### REDIS-YAML-07: Redis YAML view back navigation

**Given** the Redis YAML view was opened from the list view
**When** I press Escape
**Then** the view returns to the Redis list view

**Given** the Redis YAML view was opened from the detail view
**When** I press Escape
**Then** the view returns to the Redis detail view

---

### REDIS-YAML-08: Redis YAML nested arrays render correctly

**Given** the Redis YAML view is displayed for a cluster with CacheNodes
**Then** CacheNodes appears as a YAML array with `-` list markers
**And** each node's sub-fields (CacheNodeId, CacheNodeStatus, Endpoint, etc.) are indented properly beneath the list item

---

## DOCUMENTDB

### DOCDB-LIST-01: Navigate to DocumentDB list from main menu

**Given** the main menu is displayed with resource types listed
**When** I select "DocumentDB Clusters" and press Enter (or type `:docdb` and press Enter)
**Then** a loading spinner appears with a message like "Fetching DocumentDB clusters..."
**And** the frame title shows `docdb` while loading

---

### DOCDB-LIST-02: DocumentDB list displays correct columns

**Given** the DocumentDB list view has loaded with at least one cluster
**Then** the table header shows these columns in order: Cluster ID, Version, Status, Instances, Endpoint
**And** column widths are approximately: 28, 10, 14, 10, 48 characters
**And** column headers are bold blue (#7aa2f7) with no separator line beneath them

---

### DOCDB-LIST-03: DocumentDB list populates column data from correct API fields

**Given** the DocumentDB list view has loaded
**Then** the Cluster ID column shows the value from `DBClusterIdentifier`
**And** the Version column shows the value from `EngineVersion`
**And** the Status column shows the value from `Status`
**And** the Instances column shows the value derived from `DBClusterMembers` (see DOCDB-LIST-05)
**And** the Endpoint column shows the value from `Endpoint`

---

### DOCDB-LIST-04: DocumentDB list row count appears in frame title

**Given** the DocumentDB list view has loaded with N clusters
**Then** the frame title reads `docdb(N)` centered in the top border
**And** the count matches the actual number of rows displayed

---

### DOCDB-LIST-05: DocumentDB list shows member count for Instances column

**Given** the DocumentDB list view has loaded
**And** a cluster has DBClusterMembers containing 3 member objects
**Then** the Instances column for that cluster shows "3" (the count of members), not the raw array content

---

### DOCDB-LIST-06: DocumentDB list shows zero for cluster with no members

**Given** the DocumentDB list view has loaded
**And** a cluster has an empty DBClusterMembers array
**Then** the Instances column for that cluster shows "0"

---

### DOCDB-LIST-07: DocumentDB list row coloring by status

**Given** the DocumentDB list view has loaded with clusters in various states
**Then** rows where Status is "available" are colored green (#9ece6a)
**And** rows where Status is "creating" are colored yellow (#e0af68)
**And** rows where Status is "deleting" are colored red (#f7768e)
**And** the currently selected row overrides any status color with a blue background (#7aa2f7) and dark foreground (#1a1b26)

---

### DOCDB-LIST-08: DocumentDB list cursor navigation

**Given** the DocumentDB list view is displayed with multiple clusters
**When** I press `j` or Down arrow
**Then** the selection cursor moves down one row
**When** I press `k` or Up arrow
**Then** the selection cursor moves up one row
**When** I press `g`
**Then** the selection jumps to the first row
**When** I press `G`
**Then** the selection jumps to the last row

---

### DOCDB-LIST-09: DocumentDB list horizontal scroll

**Given** the DocumentDB list is displayed in a terminal narrower than the total column width (28+10+14+10+48 = 110 characters plus borders)
**When** I press `l` or Right arrow
**Then** the visible columns shift right to reveal columns that were off-screen
**When** I press `h` or Left arrow
**Then** the visible columns shift left
**And** the column header scrolls in sync with the data rows

---

### DOCDB-LIST-10: DocumentDB list filter

**Given** the DocumentDB list view is displayed with multiple clusters
**When** I press `/` and type a search term (e.g., "prod")
**Then** the header right side shows `/prod` in amber bold (#e0af68)
**And** only rows whose visible text contains "prod" (case-insensitive) are shown
**And** the frame title updates to `docdb(matched/total)` (e.g., `docdb(1/5)`)
**When** I press Escape
**Then** the filter is cleared and all rows reappear

---

### DOCDB-LIST-11: DocumentDB list sorting

**Given** the DocumentDB list view is displayed
**When** I press `N`
**Then** the rows sort by the name/ID column (Cluster ID) in ascending order and a sort indicator appears on that column header
**When** I press `N` again
**Then** the sort order toggles to descending
**When** I press `S`
**Then** the rows sort by the Status column

---

### DOCDB-LIST-12: DocumentDB list copy cluster ID

**Given** the DocumentDB list view is displayed with a cluster selected
**When** I press `c`
**Then** the DBClusterIdentifier of the selected cluster is copied to the system clipboard
**And** the header right side briefly shows "Copied!" in green (#9ece6a) for approximately 2 seconds

---

### DOCDB-LIST-13: DocumentDB list with no clusters

**Given** the AWS account has no DocumentDB clusters in the current region
**When** the DocumentDB list view finishes loading
**Then** a centered empty-state message is displayed
**And** the frame title shows `docdb(0)`

---

### DOCDB-DETAIL-01: Open DocumentDB detail view from list

**Given** the DocumentDB list view is displayed with a cluster selected
**When** I press Enter or `d`
**Then** the view transitions to the detail view for that cluster
**And** the frame title shows the selected DBClusterIdentifier centered in the top border

---

### DOCDB-DETAIL-02: DocumentDB detail displays correct fields

**Given** the DocumentDB detail view is displayed for a cluster
**Then** the following fields are shown as key-value pairs:
  - DBClusterIdentifier
  - Engine
  - EngineVersion
  - Status
  - Endpoint
  - ReaderEndpoint
  - Port
  - StorageEncrypted
  - DBClusterMembers
**And** keys are colored blue (#7aa2f7) and values are colored plain white (#c0caf5)

---

### DOCDB-DETAIL-03: DocumentDB detail shows full DBClusterMembers

**Given** the DocumentDB detail view is displayed for a cluster with 3 members
**Then** the DBClusterMembers field displays all 3 members (not just a count)
**And** each member shows its sub-fields: DBInstanceIdentifier, IsClusterWriter, PromotionTier
**And** sub-fields are indented beneath the parent key

---

### DOCDB-DETAIL-04: DocumentDB detail DBClusterMembers distinguishes writer from readers

**Given** the DocumentDB detail view is displayed for a cluster with members
**Then** each member's IsClusterWriter field is visible
**And** the writer member (IsClusterWriter: true) and reader members (IsClusterWriter: false) are clearly distinguishable

---

### DOCDB-DETAIL-05: DocumentDB detail status coloring

**Given** the DocumentDB detail view is displayed
**Then** the Status value "available" is rendered in green (#9ece6a)
**And** the Status value "creating" is rendered in yellow (#e0af68)
**And** the Status value "deleting" is rendered in red (#f7768e)

---

### DOCDB-DETAIL-06: DocumentDB detail shows both endpoints

**Given** the DocumentDB detail view is displayed for a cluster
**Then** the Endpoint field (writer endpoint) is shown with the full hostname
**And** the ReaderEndpoint field (reader endpoint) is shown with the full hostname
**And** these are distinct values

---

### DOCDB-DETAIL-07: DocumentDB detail scroll

**Given** the DocumentDB detail view content is taller than the visible frame
**When** I press `j` or Down arrow
**Then** the detail content scrolls down one line
**When** I press `k` or Up arrow
**Then** the detail content scrolls up one line
**When** I press `g`
**Then** the view scrolls to the top
**When** I press `G`
**Then** the view scrolls to the bottom

---

### DOCDB-DETAIL-08: DocumentDB detail copy

**Given** the DocumentDB detail view is displayed
**When** I press `c`
**Then** the full detail content is copied to the system clipboard
**And** the header shows "Copied!" in green for approximately 2 seconds

---

### DOCDB-DETAIL-09: DocumentDB detail back navigation

**Given** the DocumentDB detail view is displayed
**When** I press Escape
**Then** the view returns to the DocumentDB list view
**And** the previously selected row is still highlighted

---

### DOCDB-DETAIL-10: DocumentDB detail with empty DBClusterMembers

**Given** the DocumentDB detail view is displayed for a cluster with no members (e.g., newly created)
**Then** the DBClusterMembers section shows an empty list or a "none" indicator, not an error

---

### DOCDB-YAML-01: Open DocumentDB YAML view from list

**Given** the DocumentDB list view is displayed with a cluster selected
**When** I press `y`
**Then** the view transitions to the YAML view for that cluster
**And** the frame title shows `<DBClusterIdentifier> yaml` centered in the top border

---

### DOCDB-YAML-02: Open DocumentDB YAML view from detail

**Given** the DocumentDB detail view is displayed
**When** I press `y`
**Then** the view transitions to the YAML view for that cluster

---

### DOCDB-YAML-03: DocumentDB YAML view displays full resource data

**Given** the DocumentDB YAML view is displayed for a cluster
**Then** the full AWS API response for that cluster is rendered as syntax-highlighted YAML
**And** keys are colored blue (#7aa2f7)
**And** string values are colored green (#9ece6a)
**And** numeric values are colored orange (#ff9e64)
**And** boolean values are colored purple (#bb9af7)
**And** null values are colored dim (#565f89)
**And** indent/tree connector lines are colored dim (#414868)

---

### DOCDB-YAML-04: DocumentDB YAML view shows all available fields

**Given** the DocumentDB YAML view is displayed
**Then** the YAML output includes all non-nil fields from the API response, including but not limited to:
  - DBClusterArn
  - DBClusterIdentifier
  - Engine
  - EngineVersion
  - Status
  - Endpoint
  - ReaderEndpoint
  - Port
  - StorageEncrypted
  - DBClusterMembers (as a YAML array)
  - AvailabilityZones (as a YAML array)
  - VpcSecurityGroups (as a YAML array)

---

### DOCDB-YAML-05: DocumentDB YAML view renders DBClusterMembers as array

**Given** the DocumentDB YAML view is displayed for a cluster with 3 members
**Then** DBClusterMembers appears as a YAML array with `-` list markers
**And** each member shows its sub-fields (DBInstanceIdentifier, IsClusterWriter, PromotionTier, DBClusterParameterGroupStatus) indented properly
**And** the array contains exactly 3 items

---

### DOCDB-YAML-06: DocumentDB YAML view scroll

**Given** the DocumentDB YAML view content extends beyond the visible frame
**When** I press `j` or Down arrow
**Then** the YAML content scrolls down
**When** I press `k` or Up arrow
**Then** the YAML content scrolls up
**When** I press `g`
**Then** the view scrolls to the top
**When** I press `G`
**Then** the view scrolls to the bottom

---

### DOCDB-YAML-07: DocumentDB YAML view copy

**Given** the DocumentDB YAML view is displayed
**When** I press `c`
**Then** the full YAML text is copied to the system clipboard
**And** the header shows "Copied!" in green for approximately 2 seconds

---

### DOCDB-YAML-08: DocumentDB YAML view back navigation

**Given** the DocumentDB YAML view was opened from the list view
**When** I press Escape
**Then** the view returns to the DocumentDB list view

**Given** the DocumentDB YAML view was opened from the detail view
**When** I press Escape
**Then** the view returns to the DocumentDB detail view

---

## CROSS-CUTTING: HELP OVERLAY

### CROSS-HELP-01: Help overlay accessible from Redis and DocumentDB views

**Given** any Redis or DocumentDB view (list, detail, or YAML) is displayed
**When** I press `?`
**Then** the help overlay appears inside the frame, replacing the current content
**And** it shows the four-column layout: RESOURCE, GENERAL, NAVIGATION, HOTKEYS
**When** I press any key
**Then** the help overlay closes and the previous view content is restored

---

## CROSS-CUTTING: COMMAND MODE

### CROSS-CMD-01: Switch directly between Redis and DocumentDB via command

**Given** the Redis list view is displayed
**When** I press `:`, type `docdb`, and press Enter
**Then** the view switches to the DocumentDB list view with a loading spinner

**Given** the DocumentDB list view is displayed
**When** I press `:`, type `redis`, and press Enter
**Then** the view switches to the Redis list view with a loading spinner

---

## CROSS-CUTTING: ERROR HANDLING

### CROSS-ERR-01: Redis API error displays flash message

**Given** the Redis list view is loading
**When** the AWS API call to `describe-cache-clusters` fails (e.g., no credentials, access denied, network error)
**Then** the header right side shows an error message in red (#f7768e) bold (e.g., "Error: access denied")
**And** the frame content area does not show a spinner indefinitely

---

### CROSS-ERR-02: DocumentDB API error displays flash message

**Given** the DocumentDB list view is loading
**When** the AWS API call to `describe-db-clusters` fails
**Then** the header right side shows an error message in red (#f7768e) bold
**And** the frame content area does not show a spinner indefinitely

---

## CROSS-CUTTING: REFRESH

### CROSS-REFRESH-01: Refresh Redis list

**Given** the Redis list view is displayed
**When** I press Ctrl+R
**Then** the data is re-fetched from the AWS API
**And** a loading spinner appears during the fetch
**And** the list updates with fresh data when the fetch completes

---

### CROSS-REFRESH-02: Refresh DocumentDB list

**Given** the DocumentDB list view is displayed
**When** I press Ctrl+R
**Then** the data is re-fetched from the AWS API
**And** a loading spinner appears during the fetch
**And** the list updates with fresh data when the fetch completes
