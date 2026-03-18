# QA User Stories: RDS Views

Covers **RDS List**, **RDS Detail**, and **RDS YAML** views.
Black-box perspective only. No implementation knowledge assumed.

---

## Prerequisites (all stories)

- Terminal emulator at least 120 columns wide (full-column layout).
- AWS credentials configured with read access to RDS.
- At least one RDS instance exists in the selected region.
- Application launched: `./a9s`.

---

## A. RDS List View

### A.1 Navigation to RDS List

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| A.1.1 | Open RDS from main menu | Launch a9s. Press `j`/`k` to highlight "RDS Instances". Press `Enter`. | Frame title changes to `rds-instances(N)` where N is the count of RDS instances. A loading spinner appears while data is fetched. After loading, rows populate inside the frame. |
| A.1.2 | Open RDS via command mode | From any view, press `:`. Type `rds`. Press `Enter`. | Navigates directly to the RDS list view. Frame title shows `rds-instances(N)`. |
| A.1.3 | Return to main menu | From the RDS list view, press `Esc`. | Returns to the main menu. The cursor remains on the "RDS Instances" entry. |

### A.2 Column Layout

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| A.2.1 | All seven columns visible at 120+ cols | Open the RDS list in a terminal >= 120 columns wide. | Seven column headers are displayed in this exact order, left to right: **DB Identifier** (width 28), **Engine** (width 12), **Version** (width 10), **Status** (width 14), **Class** (width 16), **Endpoint** (width 40), **Multi-AZ** (width 10). Headers are bold, blue (`#7aa2f7`), with no separator line below them. |
| A.2.2 | Column header text matches config | Observe the column header row. | Headers read exactly: `DB Identifier`, `Engine`, `Version`, `Status`, `Class`, `Endpoint`, `Multi-AZ`. No extra or missing columns. |
| A.2.3 | Columns are space-aligned, not pipe-separated | Observe the column header row and data rows. | Columns are separated by spaces only. No vertical pipe characters appear between columns. |
| A.2.4 | Column horizontal scroll at narrow width | Resize terminal to 80 columns. Open the RDS list. | Rightmost columns that do not fit are hidden (not truncated mid-value). Press `l` to scroll right and reveal hidden columns. Press `h` to scroll left. Column headers scroll in sync with data rows. |
| A.2.5 | Very narrow terminal (60-79 cols) | Resize terminal to 70 columns. Open the RDS list. | Only the first two columns (DB Identifier, Engine) are visible. Horizontal scroll with `h`/`l` reveals the rest. |
| A.2.6 | Terminal too narrow (<60 cols) | Resize terminal below 60 columns. | An error message is displayed: "Terminal too narrow. Please resize." No table is rendered. |

### A.3 Data Mapping

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| A.3.1 | DB Identifier column | Compare the first column value for each row against `aws rds describe-db-instances` output, field `.DBInstances[].DBInstanceIdentifier`. | Values match exactly. |
| A.3.2 | Engine column | Compare the Engine column against `.DBInstances[].Engine`. | Values match (e.g., `postgres`, `mysql`, `aurora-mysql`, `aurora-postgresql`, `mariadb`, `oracle-ee`, `sqlserver-se`). |
| A.3.3 | Version column | Compare Version column against `.DBInstances[].EngineVersion`. | Values match (e.g., `14.9`, `8.0.35`, `5.7.mysql_aurora.2.11.4`). |
| A.3.4 | Status column | Compare Status column against `.DBInstances[].DBInstanceStatus`. | Values match (e.g., `available`, `stopped`, `creating`, `deleting`, `modifying`, `backing-up`, `rebooting`, `starting`, `stopping`, `failed`). |
| A.3.5 | Class column | Compare Class column against `.DBInstances[].DBInstanceClass`. | Values match (e.g., `db.t3.medium`, `db.r5.large`, `db.m6g.xlarge`). |
| A.3.6 | Endpoint column | Compare Endpoint column against `.DBInstances[].Endpoint.Address`. | Values match. This is the nested Address field, not the full Endpoint object. Example: `my-db.cluster-abc123.us-east-1.rds.amazonaws.com`. |
| A.3.7 | Multi-AZ column | Compare Multi-AZ column against `.DBInstances[].MultiAZ`. | Displays `true` or `false`. |
| A.3.8 | Row count matches API | Count rows in the table. Compare to the count in the frame title `rds-instances(N)`. Also compare to the number of objects returned by `aws rds describe-db-instances`. | All three counts match. |

### A.4 Status Coloring

Per the design spec, the **entire row** is colored based on the status value, not just the Status cell.

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| A.4.1 | Available instance (green) | Have at least one RDS instance with status `available`. | The entire row for that instance is rendered in green (`#9ece6a`). All seven column values in that row are green. |
| A.4.2 | Stopped instance (red) | Have at least one RDS instance with status `stopped`. | The entire row is rendered in red (`#f7768e`). |
| A.4.3 | Creating instance (yellow) | Have at least one RDS instance with status `creating`. | The entire row is rendered in yellow (`#e0af68`). |
| A.4.4 | Modifying instance (yellow) | Have at least one RDS instance with status `modifying`. | The entire row is rendered in yellow (`#e0af68`). |
| A.4.5 | Deleting instance (yellow or applicable color) | Have an RDS instance with status `deleting`. | The row is colored according to the status-color mapping in the design spec. If `deleting` is not explicitly listed, the row uses the default plain color (`#c0caf5`). |
| A.4.6 | Backing-up instance | Have an RDS instance with status `backing-up`. | Row uses the color assigned by the status mapping. If not explicitly listed as running/stopped/pending category, defaults to plain (`#c0caf5`). |
| A.4.7 | Failed instance (red) | Have an RDS instance with status `failed`. | The entire row is rendered in red (`#f7768e`). |
| A.4.8 | Selected row overrides status color | Navigate the cursor to any row. | The selected row displays with a full-width blue background (`#7aa2f7`), dark foreground (`#1a1b26`), and bold text, regardless of the underlying status color. |
| A.4.9 | Moving selection restores previous row color | Select a green (available) row, then press `j` to move down. | The previously selected row returns to green. The newly selected row turns blue. |

### A.5 Edge Cases

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| A.5.1 | Instance without endpoint (creating state) | Have an RDS instance that is still in `creating` status (Endpoint is null). | The Endpoint column for that row displays an empty value or a dash/placeholder. The row does not crash or display "null" / "<nil>". The entire row is yellow (creating). |
| A.5.2 | Instance without endpoint (recently deleted/failed) | Have an RDS instance in `failed` or very early `creating` state where Endpoint has not been assigned. | The Endpoint column is blank or shows a placeholder. No error, no crash. |
| A.5.3 | Multi-engine mix (postgres + mysql) | Have both PostgreSQL and MySQL instances in the same region. | Both appear in the list. The Engine column correctly shows `postgres` for one and `mysql` for the other. Rows are independently colored by their own status. |
| A.5.4 | Aurora instances | Have Aurora instances (aurora-mysql or aurora-postgresql). | Aurora instances appear in the list. Engine shows `aurora-mysql` or `aurora-postgresql`. The engine value may be wider than 12 characters and should be truncated or clipped at the column boundary rather than overflowing into the next column. |
| A.5.5 | Long DB identifier (>28 chars) | Have an RDS instance whose DBInstanceIdentifier exceeds 28 characters. | The identifier is truncated at or near 28 characters. It does not overflow into the Engine column. |
| A.5.6 | Long endpoint address (>40 chars) | Have an RDS instance whose Endpoint.Address exceeds 40 characters. | The address is truncated at or near 40 characters. It does not overflow into the Multi-AZ column. |
| A.5.7 | Boolean Multi-AZ display | Observe the Multi-AZ column. | Values show `true` or `false` (boolean rendering). Not `Yes`/`No`, not `1`/`0`, not blank. |
| A.5.8 | No RDS instances in region | Switch to a region with zero RDS instances. | The frame title shows `rds-instances(0)`. A centered message is displayed inside the frame (e.g., hint to refresh or change region). No crash. |
| A.5.9 | Large number of instances | Have 50+ RDS instances in a region. | All instances load and are scrollable. The frame title shows the correct count. Scrolling with `j`/`k` works through the entire list. `g` jumps to top, `G` jumps to bottom. |
| A.5.10 | Read replica instances | Have a read replica RDS instance. | It appears as its own row in the list with its own identifier, status, and endpoint. |

### A.6 Frame Title

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| A.6.1 | Title shows resource count | Open the RDS list. | The frame top border shows the title centered: `rds-instances(N)` where N is the number of instances. Dashes pad equally on both sides of the title. |
| A.6.2 | Title updates after filter | Press `/`, type a filter term that matches some instances. | The title changes to `rds-instances(M/N)` where M is the matched count and N is the total. |
| A.6.3 | Title after clearing filter | Press `Esc` to clear the filter. | The title reverts to `rds-instances(N)`. |

### A.7 Sorting

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| A.7.1 | Sort by name ascending | Press `N` once. | Rows re-order by DB Identifier ascending (A-Z). The DB Identifier column header gains an `^` or up-arrow indicator. |
| A.7.2 | Sort by name descending | Press `N` again. | Rows re-order by DB Identifier descending (Z-A). The header shows a down-arrow indicator. |
| A.7.3 | Sort by status | Press `S`. | Rows re-order by Status value. Pressing `S` again toggles between ascending and descending. The Status column header shows the sort arrow. |
| A.7.4 | Sort by age | Press `A`. | Rows re-order by age (instance creation time). Pressing `A` again toggles direction. |
| A.7.5 | Sort indicator display | After pressing any sort key, observe the column headers. | Exactly one column header has a sort arrow appended (no space between the title and the arrow). All other headers have no arrow. |

### A.8 Filtering

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| A.8.1 | Filter by partial name | Press `/`. Type `prod`. | Only RDS instances whose row data contains "prod" (case-insensitive) are shown. Other rows are hidden. |
| A.8.2 | Filter by engine | Press `/`. Type `postgres`. | Only rows containing "postgres" anywhere in the row are displayed. |
| A.8.3 | Filter with no matches | Press `/`. Type `zzz_nonexistent_zzz`. | No rows are displayed. The frame title shows `rds-instances(0/N)`. |
| A.8.4 | Filter header display | While filter is active, observe the header bar. | The right side of the header shows `/search-text` followed by a cursor, in amber/yellow bold (`#e0af68`). The "? for help" text is replaced. |
| A.8.5 | Backspace in filter | While typing a filter, press `Backspace`. | The last character of the filter text is removed. The matched rows update immediately. |
| A.8.6 | Esc clears filter | Press `Esc` while filter mode is active. | Filter is cleared. All rows reappear. The header right side reverts to "? for help". The frame title reverts to `rds-instances(N)`. |
| A.8.7 | Case insensitivity | Press `/`. Type `PROD`. | Matches the same rows as typing `prod`. The match is case-insensitive. |
| A.8.8 | Filter across all columns | Press `/`. Type `db.t3`. | Rows whose Class column contains `db.t3` are matched, confirming the filter searches all visible column values. |

### A.9 Keyboard Navigation

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| A.9.1 | Move down with j | Press `j`. | The selection cursor moves down one row. |
| A.9.2 | Move down with arrow key | Press `Down Arrow`. | Same as `j`. |
| A.9.3 | Move up with k | Press `k`. | The selection cursor moves up one row. |
| A.9.4 | Move up with arrow key | Press `Up Arrow`. | Same as `k`. |
| A.9.5 | Jump to top | Press `g`. | The selection jumps to the first row. |
| A.9.6 | Jump to bottom | Press `G`. | The selection jumps to the last row. |
| A.9.7 | Open detail with Enter | Select a row. Press `Enter`. | Navigates to the Detail view for the selected RDS instance. |
| A.9.8 | Open detail with d | Select a row. Press `d`. | Navigates to the Detail view for the selected RDS instance. |
| A.9.9 | Open YAML with y | Select a row. Press `y`. | Navigates to the YAML view for the selected RDS instance. |
| A.9.10 | Copy resource ID with c | Select a row. Press `c`. | The DBInstanceIdentifier (or ARN) is copied to the system clipboard. The header right side briefly shows "Copied!" in green (`#9ece6a`), which auto-clears after approximately 2 seconds. |

### A.10 Loading and Error States

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| A.10.1 | Loading spinner | Navigate to the RDS list view. Observe the frame during data fetch. | A spinner animation is displayed centered in the frame content area with text like "Fetching RDS instances..." The spinner uses dot style in blue (`#7aa2f7`). |
| A.10.2 | API error (no credentials) | Run a9s without valid AWS credentials. Navigate to the RDS list. | The header right side shows an error message in red (`#f7768e`) bold. The frame does not crash. |
| A.10.3 | API error (no permissions) | Run a9s with credentials that lack `rds:DescribeDBInstances` permission. Navigate to the RDS list. | An error message appears in the header. The frame remains stable. |
| A.10.4 | Refresh | From the RDS list, press `Ctrl+r`. | The data is re-fetched from AWS. A loading spinner appears during the fetch. Updated data replaces the previous rows. |

### A.11 Header Bar

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| A.11.1 | Normal header in RDS list | Open the RDS list. Observe the header. | Left side shows: `a9s` (accent bold blue), version (dim), profile:region (bold). Right side shows `? for help` (dim). |
| A.11.2 | Help toggle | Press `?`. | The frame content is replaced by the help screen showing key bindings in a four-column layout. Press any key to close and return to the RDS list. |

---

## B. RDS Detail View

### B.1 Navigation to Detail

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| B.1.1 | Open detail via Enter | From the RDS list, select a row, press `Enter`. | The frame content changes to the Detail view. The frame title shows the DBInstanceIdentifier of the selected instance, centered in the top border. |
| B.1.2 | Open detail via d | From the RDS list, select a row, press `d`. | Same behavior as B.1.1. |
| B.1.3 | Return to list | From the Detail view, press `Esc`. | Returns to the RDS list. The cursor remains on the same row that was previously selected. |

### B.2 Field Display

The detail view shows key-value pairs. Keys are blue (`#7aa2f7`), values are plain white (`#c0caf5`). Section headers (if any) are yellow/orange (`#e0af68`) bold.

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| B.2.1 | DBInstanceIdentifier field | Open detail for any RDS instance. | A key-value pair is displayed: key reads `DBInstanceIdentifier`, value matches the AWS API response field. |
| B.2.2 | Engine field | Inspect the detail view. | Key `Engine` is present. Value matches the API (e.g., `postgres`, `mysql`, `aurora-mysql`). |
| B.2.3 | EngineVersion field | Inspect the detail view. | Key `EngineVersion` is present. Value matches the API (e.g., `14.9`, `8.0.35`). |
| B.2.4 | DBInstanceStatus field | Inspect the detail view. | Key `DBInstanceStatus` is present. Value matches the API (e.g., `available`, `stopped`). The status value may be colored according to the status-color mapping (green for available, red for stopped, yellow for creating/modifying). |
| B.2.5 | DBInstanceClass field | Inspect the detail view. | Key `DBInstanceClass` is present. Value matches the API (e.g., `db.t3.medium`). |
| B.2.6 | Endpoint field (nested object) | Inspect the detail view. | Key `Endpoint` is present. Since Endpoint is a nested object containing Address and Port, the detail view should display both sub-fields. Expected rendering: `Endpoint` as a section or parent key, with indented sub-fields `Address` (e.g., `my-db.abc123.us-east-1.rds.amazonaws.com`) and `Port` (e.g., `5432`). |
| B.2.7 | MultiAZ field | Inspect the detail view. | Key `MultiAZ` is present. Value is `true` or `false`. |
| B.2.8 | AllocatedStorage field | Inspect the detail view. | Key `AllocatedStorage` is present. Value is a numeric value (e.g., `20`, `100`, `500`), representing storage in GiB. |
| B.2.9 | StorageType field | Inspect the detail view. | Key `StorageType` is present. Value matches the API (e.g., `gp2`, `gp3`, `io1`, `standard`). |
| B.2.10 | AvailabilityZone field | Inspect the detail view. | Key `AvailabilityZone` is present. Value matches the API (e.g., `us-east-1a`, `eu-west-1b`). |
| B.2.11 | All 10 configured fields present | Count the distinct key-value pairs in the detail view. | Exactly 10 fields are shown (per the detail config in views.yaml): DBInstanceIdentifier, Engine, EngineVersion, DBInstanceStatus, DBInstanceClass, Endpoint, MultiAZ, AllocatedStorage, StorageType, AvailabilityZone. No extra fields. No missing fields. |
| B.2.12 | Field order matches config | Observe the order of fields top to bottom. | Fields appear in the same order as listed in views.yaml: DBInstanceIdentifier first, AvailabilityZone last. |

### B.3 Endpoint Nested Object Handling

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| B.3.1 | Endpoint Address sub-field | Open detail for an `available` RDS instance that has an endpoint. | The Endpoint section includes an Address sub-field showing the DNS hostname. |
| B.3.2 | Endpoint Port sub-field | Same as B.3.1. | The Endpoint section includes a Port sub-field showing the port number (e.g., `5432` for PostgreSQL, `3306` for MySQL). |
| B.3.3 | Endpoint for creating instance | Open detail for an RDS instance in `creating` state (no endpoint assigned yet). | The Endpoint field is present but its value is empty, null, or displays a placeholder. No crash or rendering error. |
| B.3.4 | Sub-field indentation | Observe the Endpoint sub-fields. | Sub-fields (Address, Port) are indented 2 spaces relative to the parent Endpoint key, per the design spec. |

### B.4 Detail View Formatting

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| B.4.1 | Key alignment | Observe all keys in the detail view. | Keys are left-aligned with a consistent fixed width (approximately 22 characters per the design spec). Values start at the same horizontal position for all rows. |
| B.4.2 | Key color | Observe the key labels. | All keys are rendered in blue (`#7aa2f7`). |
| B.4.3 | Value color | Observe the values. | All values are rendered in plain white (`#c0caf5`), except status values which may use status coloring. |
| B.4.4 | Frame title | Observe the frame top border. | The title is the DBInstanceIdentifier of the instance, centered in the top border with dashes on both sides. |

### B.5 Detail View Scrolling

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| B.5.1 | Scroll down | If content exceeds visible area, press `j` or Down Arrow. | The view scrolls down one line. |
| B.5.2 | Scroll up | Press `k` or Up Arrow. | The view scrolls up one line. |
| B.5.3 | Jump to top | Press `g`. | The view scrolls to the very top. |
| B.5.4 | Jump to bottom | Press `G`. | The view scrolls to the very bottom. |
| B.5.5 | Content fits in frame | If all 10 fields (plus Endpoint sub-fields) fit within the frame height. | No scroll indicator is shown. All content is visible without scrolling. |
| B.5.6 | Scroll indicator | If content exceeds the frame, scroll down. | A scroll indicator appears (e.g., "N lines above" in dim text) per the design spec. |

### B.6 Detail View Actions

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| B.6.1 | Copy detail content | Press `c` in the detail view. | The full detail content (all key-value pairs as text) is copied to the system clipboard. The header right side briefly shows "Copied!" in green. |
| B.6.2 | Switch to YAML from detail | Press `y` in the detail view. | Navigates to the YAML view for the same RDS instance. |
| B.6.3 | Toggle word wrap | Press `w`. | Long values that extend beyond the frame width are wrapped to the next line. Pressing `w` again toggles wrap off and restores truncation/horizontal clipping. |
| B.6.4 | Help from detail | Press `?`. | The help screen replaces the detail content. Press any key to return to the detail view. |
| B.6.5 | Refresh from detail | Press `Ctrl+r`. | The detail data is re-fetched from AWS. A loading spinner may appear briefly. Updated data replaces the current content. |

### B.7 Detail Edge Cases

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| B.7.1 | Postgres instance detail | Open detail for a PostgreSQL instance. | Engine shows `postgres`. Endpoint Port shows `5432` (default). All fields render correctly. |
| B.7.2 | MySQL instance detail | Open detail for a MySQL instance. | Engine shows `mysql`. Endpoint Port shows `3306` (default). |
| B.7.3 | Aurora instance detail | Open detail for an Aurora instance. | Engine shows `aurora-mysql` or `aurora-postgresql`. Endpoint and all other fields render correctly. |
| B.7.4 | Multi-AZ true instance | Open detail for a Multi-AZ instance. | MultiAZ shows `true`. |
| B.7.5 | Multi-AZ false instance | Open detail for a single-AZ instance. | MultiAZ shows `false`. |
| B.7.6 | Large allocated storage | Open detail for an instance with AllocatedStorage > 1000. | The numeric value displays correctly (e.g., `2000`), not truncated or formatted incorrectly. |
| B.7.7 | Narrow terminal detail view | Resize terminal to 80 columns. Open the detail view. | Key-value pairs still render. Long values may be clipped at the frame boundary. No crash. No column overflow into the border. |

---

## C. RDS YAML View

### C.1 Navigation to YAML

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| C.1.1 | Open YAML from list via y | From the RDS list, select a row, press `y`. | The frame content changes to the YAML view. The frame title shows the DBInstanceIdentifier followed by `yaml`, centered in the top border (e.g., `my-db-instance yaml`). |
| C.1.2 | Open YAML from detail via y | From the Detail view, press `y`. | Navigates to the YAML view for the same instance. |
| C.1.3 | Return from YAML | From the YAML view, press `Esc`. | Returns to the previous view (Detail or List, depending on where YAML was entered from). |

### C.2 YAML Content Completeness

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| C.2.1 | Full dump comparison | Open the YAML view for an instance. Compare the content against the output of `aws rds describe-db-instances --db-instance-identifier <id> --output yaml`. | The YAML view contains a complete dump of the RDS instance object. All top-level fields present in the AWS CLI YAML output are present in the a9s YAML view. |
| C.2.2 | Top-level scalar fields | Verify these fields are present in the YAML: `DBInstanceIdentifier`, `DBInstanceClass`, `Engine`, `EngineVersion`, `DBInstanceStatus`, `MasterUsername`, `DBName`, `AllocatedStorage`, `AvailabilityZone`, `MultiAZ`, `StorageType`, `StorageEncrypted`, `PubliclyAccessible`, `AutoMinorVersionUpgrade`, `DeletionProtection`, `CopyTagsToSnapshot`. | Each field appears as a YAML key with its corresponding value. |
| C.2.3 | Endpoint nested object | Locate the `Endpoint` key in the YAML view. | `Endpoint` is rendered as a YAML mapping with sub-keys: `Address`, `Port`, and `HostedZoneId`. Values match the API response. |
| C.2.4 | DBSubnetGroup nested object | Locate `DBSubnetGroup` in the YAML. | It renders as a nested mapping containing keys such as `DBSubnetGroupName`, `DBSubnetGroupDescription`, `SubnetGroupStatus`, `VpcId`, and a `Subnets` list. |
| C.2.5 | VpcSecurityGroups array | Locate `VpcSecurityGroups` in the YAML. | It renders as a YAML list (`-` items), each containing `VpcSecurityGroupId` and `Status`. |
| C.2.6 | TagList array | Locate `TagList` in the YAML. | It renders as a YAML list, each item containing `Key` and `Value`. |
| C.2.7 | DBParameterGroups array | Locate `DBParameterGroups` in the YAML. | It renders as a YAML list, each containing `DBParameterGroupName` and `ParameterApplyStatus`. |
| C.2.8 | PendingModifiedValues object | Locate `PendingModifiedValues` in the YAML. | It renders as a nested mapping. If there are no pending modifications, the sub-fields are empty/null. |
| C.2.9 | OptionGroupMemberships | Locate `OptionGroupMemberships`. | Renders as a YAML list with `OptionGroupName` and `Status`. |
| C.2.10 | CertificateDetails object | Locate `CertificateDetails`. | Renders with sub-keys `CAIdentifier` and `ValidTill`. |
| C.2.11 | No fields omitted | Scroll through the entire YAML output and compare field-by-field with the AWS CLI output. | No fields from the AWS API response are missing. The YAML view is a complete representation of the `describe-db-instances` response for that single instance. |

### C.3 YAML Syntax Coloring

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| C.3.1 | Key coloring | Observe YAML keys (e.g., `DBInstanceIdentifier:`). | All keys are rendered in blue (`#7aa2f7`). |
| C.3.2 | String value coloring | Observe string values (e.g., `"postgres"`, `"us-east-1a"`). | String values are rendered in green (`#9ece6a`). |
| C.3.3 | Numeric value coloring | Observe numeric values (e.g., `20` for AllocatedStorage, `5432` for Port). | Numeric values are rendered in orange (`#ff9e64`). |
| C.3.4 | Boolean value coloring | Observe boolean values (e.g., `true` for MultiAZ, `false` for PubliclyAccessible). | Boolean values are rendered in purple (`#bb9af7`). |
| C.3.5 | Null value coloring | Observe null values (e.g., `~` or `null` for fields like `SecondaryAvailabilityZone` when not set). | Null values are rendered in dim (`#565f89`). |
| C.3.6 | Indent/tree connector coloring | Observe the vertical lines for nested structures. | Tree connectors (vertical bar characters) are rendered in dim (`#414868`). |
| C.3.7 | No color bleed | Observe transitions between key and value on the same line. | The key color ends at the colon. The value color begins after the space following the colon. No color bleeding across boundaries. |

### C.4 YAML Structure and Formatting

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| C.4.1 | Valid YAML syntax | Mentally parse (or copy and validate) the displayed content. | The content is valid YAML. Indentation is consistent (2-space indent for nested levels). Lists use `- ` prefix. |
| C.4.2 | Alphabetical or logical key ordering | Observe the order of top-level keys. | Keys appear in a consistent order (either alphabetical or matching the AWS SDK struct field order). |
| C.4.3 | Nested indentation | Observe nested objects like Endpoint, DBSubnetGroup. | Each nesting level is indented consistently (2 additional spaces per level). |
| C.4.4 | Array rendering | Observe array fields (VpcSecurityGroups, TagList, DBParameterGroups). | Arrays are rendered with YAML list syntax: each item prefixed with `- `, sub-fields indented under the list item. |
| C.4.5 | Empty array rendering | If a list field has no items (e.g., `ReadReplicaDBInstanceIdentifiers` for a non-source instance). | The field is present with an empty list representation (`[]` or no items listed). |

### C.5 YAML View Scrolling

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| C.5.1 | Scroll down | The YAML content for an RDS instance is typically very long. Press `j` or Down Arrow. | The view scrolls down one line. |
| C.5.2 | Scroll up | Press `k` or Up Arrow. | The view scrolls up one line. |
| C.5.3 | Jump to top | Press `g`. | The view scrolls to the first line of the YAML. |
| C.5.4 | Jump to bottom | Press `G`. | The view scrolls to the last line of the YAML. |
| C.5.5 | Scroll indicator present | Scroll down several lines. | A scroll indicator is shown (e.g., "N lines above" in dim text `#414868`). |
| C.5.6 | Smooth scrolling experience | Rapidly press `j` multiple times. | The view scrolls smoothly without lag or visual artifacts. |

### C.6 YAML View Actions

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| C.6.1 | Copy YAML content | Press `c` in the YAML view. | The full YAML text is copied to the system clipboard. The header shows "Copied!" in green, auto-clearing after ~2 seconds. |
| C.6.2 | Help from YAML | Press `?`. | The help screen replaces the YAML content. Press any key to return to the YAML view. |
| C.6.3 | Refresh from YAML | Press `Ctrl+r`. | The YAML data is re-fetched from AWS. Updated YAML replaces the current content. |
| C.6.4 | Esc returns to previous view | Press `Esc`. | Returns to the view from which YAML was opened (Detail or List). |

### C.7 YAML Edge Cases

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| C.7.1 | Creating instance YAML | Open YAML for an instance in `creating` state. | The YAML renders all available fields. Fields not yet populated (like Endpoint) show null. No crash. |
| C.7.2 | Instance with many tags | Open YAML for an instance that has 20+ tags. | All tags are listed under `TagList`. The YAML is scrollable to see all of them. |
| C.7.3 | Instance with special characters in values | If a DB name or tag value contains special characters (colons, quotes, brackets). | The YAML properly escapes or quotes the value. The syntax remains valid. |
| C.7.4 | Aurora cluster member YAML | Open YAML for an Aurora instance that is a cluster member. | The `DBClusterIdentifier` field is populated. All Aurora-specific fields are present and correctly rendered. |
| C.7.5 | Instance with PendingModifiedValues | Open YAML for an instance that has pending modifications (e.g., instance class change scheduled). | The `PendingModifiedValues` section contains the pending fields with their new values, not empty/null. |
| C.7.6 | Read replica YAML | Open YAML for a read replica. | `ReadReplicaSourceDBInstanceIdentifier` is populated with the source instance identifier. |
| C.7.7 | Narrow terminal YAML | Resize to 80 columns. Open the YAML view. | YAML lines that exceed the frame width are clipped at the right border. No horizontal overflow. Content remains readable via vertical scrolling. Word wrap (`w`) may be available to wrap long lines. |
| C.7.8 | Frame title format | Open YAML for instance `my-production-db`. | The frame title reads `my-production-db yaml` centered in the top border, with equal dashes on both sides. |

---

## D. Cross-View Interactions

| # | Story | Steps | Expected |
|---|-------|-------|----------|
| D.1 | List to Detail to YAML and back | From RDS list, select a row, press `Enter` (Detail). Then press `y` (YAML). Then press `Esc` (back to Detail). Then press `Esc` (back to List). | Each transition works. The view stack is: List > Detail > YAML. Esc pops one level at a time. The list cursor position is preserved. |
| D.2 | List to YAML directly and back | From RDS list, select a row, press `y` (YAML). Then press `Esc`. | Returns directly to the RDS list (not to Detail). The list cursor position is preserved. |
| D.3 | Command mode from any RDS view | From any of the three RDS views, press `:`. Type `ec2`. Press `Enter`. | Navigates away from the RDS views to the EC2 list. |
| D.4 | Filter does not persist to detail/YAML | Apply a filter in the RDS list. Select a filtered row, press `Enter`. | The Detail view shows all fields for that instance, not filtered. Press `Esc` to return to list; the filter is still active. |
| D.5 | Profile/region change reloads RDS data | From the RDS list, switch profiles or regions (via `:ctx` or `:region`). Navigate back to the RDS list. | The list re-fetches instances for the new profile/region. Frame title count updates. |
| D.6 | Help accessible from all RDS views | Press `?` from the RDS list, Detail view, and YAML view. | In each case, the help screen appears with the correct key bindings for the current context. Press any key to dismiss. |
| D.7 | Ctrl+C force quit from any RDS view | Press `Ctrl+c` from the list, detail, or YAML view. | The application exits immediately from any view. |
