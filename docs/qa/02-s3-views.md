# QA User Stories: S3 Views

Covers the S3 Bucket List, S3 Object List, and S3 Detail views.
All stories are written from a black-box perspective against the design spec and
`views.yaml` / `views_reference.yaml` configuration files.

AWS CLI equivalents are cited so testers can verify data parity.

---

## A. S3 Bucket List View

### A.1 Loading State

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I select S3 Buckets from the main menu and the API has not yet responded. | A spinner (animated dot) is displayed centered inside the frame. The text reads "Fetching S3 buckets..." (or similar). The frame title reads "s3" with no count. The header shows "? for help" on the right. |
| A.1.2 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. The spinner continues uninterrupted. |
| A.1.3 | The API responds successfully with bucket data. | The spinner disappears. The table renders with column headers and rows. The frame title updates to "s3(N)" where N is the total bucket count. |
| A.1.4 | The API responds with an error (e.g., no credentials, network timeout). | The spinner disappears. A red error flash message appears in the header right side (e.g., "Error: no credentials"). The frame content area shows an appropriate empty or error state. |

### A.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | The API returns an empty bucket list (zero buckets). | The frame title reads "s3(0)". The content area shows a centered message (e.g., "No S3 buckets found") with a hint to refresh or change region. No column headers are shown (or headers are shown with no data rows). |
| A.2.2 | I press ctrl+r on the empty state. | The loading spinner appears again while the refresh request is in flight. |

### A.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | Buckets load and the table renders. | Exactly two columns are displayed: "Bucket Name" (width 20) and "Creation Date" (width 22). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| A.3.2 | I verify column data against `aws s3api list-buckets`. | The "Bucket Name" column maps to `.Buckets[].Name`. The "Creation Date" column maps to `.Buckets[].CreationDate`. Every bucket returned by the CLI appears as a row in the table. |
| A.3.3 | A bucket name is longer than 20 characters. | The name is truncated to fit the 20-character column width. No row wrapping occurs. |
| A.3.4 | The terminal is narrower than the combined column widths. | The rightmost column(s) are hidden (not truncated mid-value). Horizontal scroll with h/l is available to reveal hidden columns. |

### A.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| A.4.1 | 5 buckets are loaded. | The frame top border shows the title centered: "s3(5)" with equal-length dashes on both sides. |
| A.4.2 | A filter is active and matches 2 of 5 buckets. | The frame title reads "s3(2/5)". |
| A.4.3 | A filter is active and matches 0 buckets. | The frame title reads "s3(0/5)". The content area is empty (no rows). |

### A.5 Navigation

| ID | Story | Expected |
|----|-------|----------|
| A.5.1 | I press j (or down-arrow) with the first bucket selected. | The selection cursor moves to the second bucket. The previously selected row loses the blue highlight. The new row gains the full-width blue background (#7aa2f7) with dark foreground (#1a1b26), bold. |
| A.5.2 | I press k (or up-arrow) with the second bucket selected. | The selection cursor moves back to the first bucket. |
| A.5.3 | I press g. | The selection jumps to the very first bucket in the list. |
| A.5.4 | I press G. | The selection jumps to the very last bucket in the list. |
| A.5.5 | I press PageDown. | The selection moves down by one page of visible rows. If fewer rows remain below than a page, the cursor lands on the last row. |
| A.5.6 | I press PageUp. | The selection moves up by one page of visible rows. If fewer rows remain above than a page, the cursor lands on the first row. |
| A.5.7 | I press j on the last row. | The behavior depends on wrap configuration. If wrapping, cursor moves to the first row. If not, cursor stays on the last row. |
| A.5.8 | I press k on the first row. | The behavior depends on wrap configuration. If wrapping, cursor moves to the last row. If not, cursor stays on the first row. |
| A.5.9 | There are more buckets than fit on screen. I scroll past the visible area. | The table scrolls to keep the selected row visible. The column headers remain in place. |

### A.6 Sorting

| ID | Story | Expected |
|----|-------|----------|
| A.6.1 | I press N on the bucket list. | Rows are sorted by bucket name in ascending order. The "Bucket Name" column header shows a sort indicator: an up-arrow appended directly (e.g., "Bucket Name^"). |
| A.6.2 | I press N again. | Sort order toggles to descending. The indicator changes to a down-arrow. |
| A.6.3 | I press A on the bucket list. | Rows are sorted by creation date (age) in ascending order (oldest first). The "Creation Date" column header shows the up-arrow indicator. The "Bucket Name" header no longer shows any indicator. |
| A.6.4 | I press A again. | Sort order toggles to descending (newest first). The indicator changes to a down-arrow. |
| A.6.5 | I sort by name, then apply a filter. | The filtered subset remains sorted by name. The sort indicator persists on the column header. |
| A.6.6 | I sort by name, then refresh with ctrl+r. | After data reloads, the sort order and direction are preserved. The indicator remains. |

### A.7 Filter

| ID | Story | Expected |
|----|-------|----------|
| A.7.1 | I press /. | The header right side changes from "? for help" to "/|" (amber/bold, with cursor). Filter mode is active. |
| A.7.2 | I type "prod" in filter mode. | The header right shows "/prod|". Only rows whose bucket name contains "prod" (case-insensitive) are displayed. The frame title updates to "s3(M/N)" where M is the matched count. |
| A.7.3 | I press backspace in filter mode. | The last character of the filter text is removed. The filtered result updates immediately. |
| A.7.4 | I press Escape in filter mode. | The filter is cleared. All rows reappear. The frame title reverts to "s3(N)". The header right reverts to "? for help". |
| A.7.5 | I type a filter string that matches no buckets. | Zero rows are displayed. The frame title shows "s3(0/N)". |
| A.7.6 | I type "PROD" (uppercase) and buckets named "prod-data", "PROD-logs" exist. | Both buckets appear. Filtering is case-insensitive. |
| A.7.7 | I have a filter active and press j/k. | Navigation works within the filtered result set only. |
| A.7.8 | I have a filter active and press Enter on a selected bucket. | I drill into the selected bucket's object list (same as unfiltered behavior). |

### A.8 Enter Key (Drill Into Bucket)

| ID | Story | Expected |
|----|-------|----------|
| A.8.1 | I select a bucket and press Enter. | The view transitions to the S3 Object List for that bucket. A loading spinner appears while objects are fetched. The bucket list view is pushed onto the view stack. |
| A.8.2 | I verify Enter does NOT open a detail view. | Pressing Enter on a bucket navigates into the bucket to show its objects. It does NOT open the bucket detail/describe view. |

### A.9 Detail Key (d)

| ID | Story | Expected |
|----|-------|----------|
| A.9.1 | I select a bucket and press d. | The detail view opens for the selected bucket. The frame title shows the bucket name. The detail fields are rendered as key-value pairs. |
| A.9.2 | I verify the detail fields match views.yaml s3 detail config. | The detail view shows: BucketArn, BucketRegion, CreationDate. These are the three fields listed under `views.s3.detail`. |
| A.9.3 | I press Escape on the detail view. | I return to the S3 bucket list. The cursor position is preserved on the same bucket I had selected. |

### A.10 YAML Key (y)

| ID | Story | Expected |
|----|-------|----------|
| A.10.1 | I select a bucket and press y. | The YAML view opens. The frame title includes the bucket name and "yaml" (e.g., "my-bucket yaml"). The full resource is rendered as syntax-highlighted YAML. |
| A.10.2 | YAML keys are colored blue (#7aa2f7), string values green (#9ece6a), numbers orange (#ff9e64), booleans purple (#bb9af7), null values dim (#565f89). | Visual inspection confirms the color coding matches the design spec. |
| A.10.3 | The YAML content is longer than the visible area. | I can scroll with j/k/g/G. Scroll indicators appear when content extends beyond the visible area. |
| A.10.4 | I press Escape on the YAML view. | I return to the S3 bucket list. |

### A.11 Copy Key (c)

| ID | Story | Expected |
|----|-------|----------|
| A.11.1 | I select a bucket and press c. | The bucket name is copied to the system clipboard. A green flash message "Copied!" appears in the header right side. |
| A.11.2 | After ~2 seconds. | The "Copied!" flash message auto-clears and the header right reverts to "? for help". |
| A.11.3 | I paste from clipboard into another application. | The pasted text matches the bucket name exactly. |

### A.12 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| A.12.1 | I press ctrl+r on the bucket list. | The loading spinner appears. A fresh API call is made. When it completes, the table repopulates with current data. |
| A.12.2 | A new bucket was created since the last load. I press ctrl+r. | The new bucket appears in the refreshed list. The count in the frame title increments. |
| A.12.3 | I had a filter active and press ctrl+r. | The data refreshes. The filter remains applied to the new data. The frame title count updates accordingly. |

### A.13 Escape (Back)

| ID | Story | Expected |
|----|-------|----------|
| A.13.1 | I press Escape on the S3 bucket list. | I return to the main menu. The main menu shows the resource type list with S3 Buckets among the entries. |

### A.14 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| A.14.1 | I press ? on the bucket list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: RESOURCE, GENERAL, NAVIGATION, HOTKEYS. |
| A.14.2 | I press any key on the help screen. | The help screen closes and the bucket list table reappears. |

### A.15 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| A.15.1 | I press : on the bucket list. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| A.15.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list. |
| A.15.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The bucket list remains. |

### A.16 Row Coloring

| ID | Story | Expected |
|----|-------|----------|
| A.16.1 | S3 buckets are displayed (buckets have no status field). | Rows are rendered in plain text color (#c0caf5) since S3 buckets do not have a status value that maps to running/stopped/etc. |
| A.16.2 | I select a row. | The selected row has full-width blue background (#7aa2f7), dark foreground (#1a1b26), bold text. All other rows revert to their normal coloring. |
| A.16.3 | I move selection away from a row. | The previously selected row reverts to plain coloring. |

---

## B. S3 Object List View

### B.1 Loading State

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | I press Enter on a bucket in the bucket list. | The object list view opens. A spinner appears centered in the frame with text like "Fetching S3 objects..." while `list-objects-v2` is in flight. |
| B.1.2 | The API responds successfully. | The spinner disappears. Objects are rendered as table rows with columns: Key, Size, Last Modified. The frame title shows the bucket name and object count. |
| B.1.3 | The API responds with an error (e.g., AccessDenied on the bucket). | The spinner disappears. A red error flash appears in the header. |

### B.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| B.2.1 | The bucket contains zero objects and zero common prefixes. | The frame title shows the bucket name with count 0 (e.g., "my-bucket(0)"). A centered message indicates no objects found. |

### B.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| B.3.1 | Objects are loaded. | Three columns are displayed: "Key" (width 20), "Size" (width 12), "Last Modified" (width 22). Column headers are bold blue (#7aa2f7) with no separator line below. |
| B.3.2 | I verify column data against `aws s3api list-objects-v2 --bucket BUCKET`. | "Key" maps to `.Contents[].Key`. "Size" maps to `.Contents[].Size`. "Last Modified" maps to `.Contents[].LastModified`. All objects returned by the CLI appear as rows. |
| B.3.3 | An object key is longer than 20 characters. | The key is truncated to fit the 20-character column width. |
| B.3.4 | The total column width exceeds the terminal width. | Rightmost columns are hidden. I can scroll horizontally with h and l keys. Column headers scroll in sync with data columns. |

### B.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| B.4.1 | I am viewing objects inside bucket "my-data" with 150 objects. | The frame title shows the bucket name and the object count, e.g., "my-data(150)", centered in the top border. |
| B.4.2 | I navigate into a prefix "logs/" containing 30 objects. | The frame title updates to reflect the current context and count within that prefix. |
| B.4.3 | A filter is active matching 5 of 150 objects. | The frame title shows the match ratio, e.g., "my-data(5/150)". |

### B.5 Folder Navigation (Common Prefixes)

| ID | Story | Expected |
|----|-------|----------|
| B.5.1 | The bucket has common prefixes (virtual folders) like "logs/", "data/", "backups/". | These prefixes appear as navigable entries in the object list. They are visually distinguishable from regular objects (e.g., displayed as folder-like entries). |
| B.5.2 | I select a folder entry (e.g., "logs/") and press Enter. | The view navigates into that prefix. A new API call fetches objects under the "logs/" prefix. The spinner appears during loading. |
| B.5.3 | I am inside a prefix "logs/" and press Escape. | I navigate back up one level to the bucket root (or previous prefix level). |
| B.5.4 | I navigate into nested prefixes: "logs/" then "2024/" then "03/". | Each Enter pushes deeper. Each Escape pops back one prefix level. |
| B.5.5 | I am at the bucket root (no prefix) and press Escape. | I return to the S3 bucket list view. |

### B.6 Enter on Object

| ID | Story | Expected |
|----|-------|----------|
| B.6.1 | I select an object (not a folder) and press Enter. | The detail view opens for that object. |
| B.6.2 | The detail view shows fields from views.yaml `s3_objects.detail`. | The fields displayed are: Name, LastModified, Owner. |

### B.7 Navigation

| ID | Story | Expected |
|----|-------|----------|
| B.7.1 | I press j/k/g/G/PageUp/PageDown in the object list. | Navigation behaves identically to the bucket list: j moves down, k moves up, g jumps to top, G jumps to bottom, PageUp/PageDown scroll by page. |
| B.7.2 | I press h (or left-arrow). | Columns scroll left (visible column window shifts), revealing any previously hidden left columns. |
| B.7.3 | I press l (or right-arrow). | Columns scroll right (visible column window shifts), revealing any previously hidden right columns. |
| B.7.4 | Column headers scroll in sync with data when I press h/l. | The column header row shifts horizontally by the same offset as data rows. |

### B.8 Horizontal Scroll

| ID | Story | Expected |
|----|-------|----------|
| B.8.1 | Terminal width is 80 columns and all three columns (20+12+22=54 plus borders/padding) fit. | No horizontal scrolling is needed. h/l does nothing visible or is a no-op. |
| B.8.2 | Terminal width is very narrow (e.g., 60 columns) and not all columns fit. | Rightmost columns are hidden. Pressing l reveals them while hiding leftmost columns. Pressing h reverses. |

### B.9 Filter

| ID | Story | Expected |
|----|-------|----------|
| B.9.1 | I press / in the object list and type "config". | Only objects whose key contains "config" (case-insensitive) are shown. The frame title updates to show matched/total. |
| B.9.2 | I press Escape while filter is active. | The filter clears. All objects reappear. |

### B.10 Sort

| ID | Story | Expected |
|----|-------|----------|
| B.10.1 | I press N in the object list. | Objects are sorted by key name alphabetically. The sort indicator appears on the "Key" column header. |
| B.10.2 | I press A in the object list. | Objects are sorted by last modified date. The sort indicator appears on the "Last Modified" column header. |

### B.11 Copy (c)

| ID | Story | Expected |
|----|-------|----------|
| B.11.1 | I select an object and press c. | The object key is copied to the clipboard. A "Copied!" flash appears in the header. |

### B.12 Detail (d) and YAML (y)

| ID | Story | Expected |
|----|-------|----------|
| B.12.1 | I select an object and press d. | The detail view opens showing the s3_objects detail fields: Name, LastModified, Owner. |
| B.12.2 | I select an object and press y. | The YAML view opens showing the full object metadata as syntax-highlighted YAML. |

### B.13 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| B.13.1 | I press ctrl+r in the object list. | The spinner appears. A fresh `list-objects-v2` call is made for the current bucket and prefix. The table updates with new results. |
| B.13.2 | A new object was uploaded since last load. I press ctrl+r. | The new object appears in the refreshed list. |

### B.14 Escape (Back to Bucket List)

| ID | Story | Expected |
|----|-------|----------|
| B.14.1 | I am at the root level of the object list (no prefix) and press Escape. | I return to the S3 bucket list. The cursor is on the same bucket I had entered. |
| B.14.2 | I am inside a prefix (e.g., "logs/") and press Escape. | I navigate up one prefix level (back to root), NOT back to the bucket list. |

### B.15 Help and Command Mode

| ID | Story | Expected |
|----|-------|----------|
| B.15.1 | I press ? in the object list. | The help overlay appears with the same four-column layout as in other list views. |
| B.15.2 | I press : in the object list. | Command mode activates in the header. I can type a command to navigate to another resource type. |

---

## C. S3 Detail View

### C.1 Bucket Detail (via d from bucket list)

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | I press d on a bucket in the bucket list. | The detail view opens. The frame title shows the bucket name (e.g., "my-bucket"). |
| C.1.2 | I verify the displayed fields. | The detail view shows key-value pairs for: BucketArn, BucketRegion, CreationDate. These match the `views.s3.detail` configuration. |
| C.1.3 | I compare BucketArn with expected format. | The ARN follows the pattern `arn:aws:s3:::bucket-name`. |
| C.1.4 | I compare BucketRegion with the current region. | The region value is a valid AWS region string (e.g., "us-east-1"). |
| C.1.5 | I compare CreationDate with `aws s3api list-buckets` output. | The date matches the `.Buckets[].CreationDate` value from the CLI for the same bucket. |
| C.1.6 | Keys are rendered in blue (#7aa2f7), values in white (#c0caf5). | Visual inspection confirms the key-value color coding from the design spec. |
| C.1.7 | Section headers (if present) are rendered in yellow/orange (#e0af68), bold. | Visual inspection confirms. |

### C.2 Object Detail (via Enter or d from object list)

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | I select an object and press Enter (or d) in the object list. | The detail view opens for that object. The frame title shows the object key. |
| C.2.2 | I verify the displayed fields. | The detail view shows key-value pairs for: Name, LastModified, Owner. These match the `views.s3_objects.detail` configuration. |
| C.2.3 | I compare data against `aws s3api head-object --bucket BUCKET --key KEY`. | LastModified matches the header response. Owner information matches (if available from the list response or a separate describe call). |
| C.2.4 | The Owner field may be empty or null for certain bucket configurations. | The detail view handles missing Owner gracefully: it displays null/empty/dash rather than crashing. |

### C.3 Detail View Navigation

| ID | Story | Expected |
|----|-------|----------|
| C.3.1 | I press j/k in the detail view. | The viewport scrolls up/down by one line. |
| C.3.2 | I press g in the detail view. | The viewport jumps to the top of the detail content. |
| C.3.3 | I press G in the detail view. | The viewport jumps to the bottom of the detail content. |
| C.3.4 | The detail content is shorter than the visible area. | No scrolling occurs. No scroll indicators are shown. |
| C.3.5 | The detail content is longer than the visible area. | Scroll indicators appear (e.g., "X lines above" / "X lines below") in dim text. |

### C.4 Detail View Actions

| ID | Story | Expected |
|----|-------|----------|
| C.4.1 | I press y in the detail view. | The view switches from detail to YAML view for the same resource. |
| C.4.2 | I press c in the detail view. | The full detail content is copied to the clipboard. A "Copied!" flash appears. |
| C.4.3 | I press w in the detail view. | Word wrap is toggled. Long values that previously extended beyond the visible width now wrap to the next line (or vice versa). |
| C.4.4 | I press Escape in the detail view. | I return to the previous list view (bucket list or object list, depending on where I came from). |

### C.5 Detail View from views_reference.yaml

| ID | Story | Expected |
|----|-------|----------|
| C.5.1 | I check all available S3 bucket fields from views_reference.yaml. | The reference lists: BucketArn, BucketRegion, CreationDate, Name. Only BucketArn, BucketRegion, CreationDate are shown in detail (per views.yaml config). Name is used in the list view. |
| C.5.2 | I check all available S3 object fields from views_reference.yaml. | The reference lists: ChecksumAlgorithm[], ChecksumType, ETag, Key, LastModified, Owner.DisplayName, Owner.ID, RestoreStatus.IsRestoreInProgress, RestoreStatus.RestoreExpiryDate, Size, StorageClass. Only Name, LastModified, Owner are shown in detail (per views.yaml config). |
| C.5.3 | I edit views.yaml to add StorageClass to s3_objects detail, restart the app. | The detail view for objects now shows StorageClass alongside the other fields. This confirms the config-driven approach works for S3 objects. |

---

## D. Cross-Cutting Concerns

### D.1 Header Consistency

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | In every S3 view (bucket list, object list, detail, YAML), the header displays: "a9s" (accent bold), version (dim), profile:region (bold). | Visual inspection confirms across all S3 views. |
| D.1.2 | The header right side shows "? for help" in normal mode across all S3 views. | Confirmed in bucket list, object list, detail, and YAML views. |

### D.2 View Stack

| ID | Story | Expected |
|----|-------|----------|
| D.2.1 | Main Menu -> S3 Bucket List -> Object List -> Object Detail -> YAML; then Escape four times. | Each Escape pops one level: YAML -> Detail -> Object List -> Bucket List -> Main Menu. No state is lost at any intermediate level. |
| D.2.2 | Main Menu -> S3 Bucket List -> Bucket Detail (d) -> YAML (y); then Escape twice. | YAML -> Detail -> Bucket List. The cursor is still on the same bucket. |

### D.3 Terminal Resize

| ID | Story | Expected |
|----|-------|----------|
| D.3.1 | I resize the terminal while viewing the S3 bucket list. | The layout reflows. Column visibility adjusts to the new width. The frame border redraws correctly. |
| D.3.2 | I resize the terminal to below 60 columns. | An error message appears: "Terminal too narrow. Please resize." |
| D.3.3 | I resize the terminal to below 7 lines. | An error message appears: "Terminal too short. Please resize." |
| D.3.4 | I resize the terminal while in the S3 object detail or YAML view. | The viewport adjusts to the new dimensions. Content reflows appropriately. |

### D.4 Alternating Row Colors

| ID | Story | Expected |
|----|-------|----------|
| D.4.1 | The bucket list has more than 2 rows. | Alternating rows have a subtle background color difference (#1e2030) for readability. Selected row always has blue background regardless. |
| D.4.2 | The object list has more than 2 rows. | Same alternating row pattern applies. |
