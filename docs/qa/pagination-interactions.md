# QA User Stories: Pagination Interactions

Issue: #110
Design Spec: `docs/design/pagination-interactions.md`
Target: a9s v3.24+

---

## 1. Frame Title Format

### Story: Frame title shows count with plus suffix for truncated lists

**Given:** The user opens a resource type that returns more results than one API page (e.g., CloudTrail Events with 200 results on the first page and more available on the server)
**When:** The initial page loads and the resource list is displayed
**Then:** The frame title reads `ct-events(200+)` where `+` indicates more data exists on the server

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200
```

Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: Frame title shows exact count when all results fit in one page

**Given:** The user opens a resource type where all results are returned in a single API page (e.g., EC2 Instances with 42 instances)
**When:** The resource list finishes loading
**Then:** The frame title reads `ec2(42)` with no `+` suffix, and no load-more hint appears at the bottom

**AWS comparison:**

```
aws ec2 describe-instances
```

Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

---

### Story: Frame title shows loading indicator during load-more fetch

**Given:** The user is viewing a truncated resource list showing `ct-events(200+)`
**When:** The user presses `M` to load the next page
**Then:** The frame title changes to `ct-events(200+ loading...)` while the fetch is in progress, and reverts to `ct-events(400+)` or `ct-events(400)` when loading completes (depending on whether more pages remain)

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200 --next-token <token>
```

Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: Frame title shows filtered count out of loaded count for truncated list

**Given:** The user is viewing a truncated CloudTrail Events list showing `ct-events(200+)`
**When:** The user presses `/` and types `bucket`
**Then:** The frame title changes to `ct-events(3/200+)` showing 3 matching items out of 200 loaded, with `+` indicating more exist on the server

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200 --lookup-attributes AttributeKey=EventName,AttributeValue=*bucket*
```

(Note: AWS CLI does not support client-side substring filtering; a9s performs this locally)
Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: Frame title shows filtered count out of total when all pages loaded

**Given:** The user has loaded all pages of CloudTrail Events (pressed `M` repeatedly until all data is loaded) showing `ct-events(1847)`
**When:** The user presses `/` and types `bucket`
**Then:** The frame title reads `ct-events(12/1847)` with no `+` suffix, and no load-more hint appears

**AWS comparison:**

```
aws cloudtrail lookup-events | grep -i bucket
```

Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

## 2. Load-More Hint

### Story: Standard load-more hint on truncated list without filter

**Given:** The user is viewing a truncated resource list (e.g., EBS Volumes with 200+ results)
**When:** The list is idle (not currently loading more data) and no filter is active
**Then:** A dim hint line appears at the bottom of the list reading `-- M: load more --`

**AWS comparison:**

```
aws ec2 describe-volumes --max-results 200
```

Expected fields visible: Name, Volume ID, State, Size (GiB), Type, IOPS, Encrypted, Attached To, AZ, Created

---

### Story: Load-more hint shows loading state

**Given:** The user is viewing a truncated resource list and presses `M`
**When:** The next page is being fetched from AWS
**Then:** The hint line changes to `-- loading... --` and the `M` key is unresponsive until loading completes

**AWS comparison:**

```
aws ec2 describe-volumes --max-results 200 --next-token <token>
```

Expected fields visible: Name, Volume ID, State, Size (GiB), Type, IOPS, Encrypted, Attached To, AZ, Created

---

### Story: Load-more hint includes filter warning when filter is active

**Given:** The user is viewing a truncated resource list and has typed a filter (e.g., `/prod`)
**When:** The filtered results are displayed and the list is idle
**Then:** The hint line reads `-- M: load more (filter applies to loaded data only) --` to remind the user that unloaded pages may contain additional matches

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200 | grep prod
```

(a9s filters client-side; AWS has no server-side text search across arbitrary fields)
Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: Load-more hint shows loading state even with filter active

**Given:** The user has a filter active on a truncated list and presses `M`
**When:** The next page is being fetched
**Then:** The hint line changes to `-- loading... --` (the filter context warning is temporarily replaced by the loading indicator)

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200 --next-token <token>
```

Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: No load-more hint when all data is loaded

**Given:** The user is viewing a resource list where all results were returned in a single page (e.g., EC2 with 42 instances)
**When:** The list is displayed
**Then:** No load-more hint appears at the bottom of the list

**AWS comparison:**

```
aws ec2 describe-instances
```

Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

---

### Story: Load-more hint disappears after final page is loaded

**Given:** The user is viewing a truncated list showing `ct-events(1600+)` with the `-- M: load more --` hint
**When:** The user presses `M` and the final page is returned with no more pages available
**Then:** The frame title changes to `ct-events(1847)` (no `+`) and the load-more hint disappears entirely

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200 --next-token <final-token>
```

Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

## 3. Filtering (`/`)

### Story: Filter operates on loaded data only

**Given:** The user is viewing a truncated list of AMIs showing `ami(200+)`
**When:** The user presses `/` and types `ubuntu`
**Then:** Only AMIs whose visible fields contain "ubuntu" (case-insensitive) are shown. The filter operates instantly on the 200 loaded items without making any API calls. The frame title shows `ami(15/200+)` (example count).

**AWS comparison:**

```
aws ec2 describe-images --owners self --filters "Name=name,Values=*ubuntu*"
```

(Note: AWS supports server-side name filters, but a9s filters all visible fields client-side)
Expected fields visible: Name, Image ID, State, Arch, Platform, Root Device, Created, Public

---

### Story: Filter input appears in header

**Given:** The user is viewing any resource list
**When:** The user presses `/`
**Then:** The header right side changes from `? for help` to `/` followed by a text cursor, displayed in amber/bold. Characters typed appear after the `/`.

**AWS comparison:**
N/A (UI behavior, no AWS equivalent)

---

### Story: Filter is case-insensitive

**Given:** The user is viewing a resource list with items containing "CreateBucket" and "createbucket"
**When:** The user presses `/` and types `createbucket`
**Then:** Both items match and are displayed

**AWS comparison:**

```
aws cloudtrail lookup-events | grep -i createbucket
```

Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: Pressing M with active filter loads raw page and re-applies filter

**Given:** The user is viewing `ct-events(3/200+)` with filter `/bucket` active
**When:** The user presses `M`
**Then:** The app fetches the next raw (unfiltered) page from AWS, appends all new items to the loaded dataset, re-applies the "bucket" filter, and updates both counts. The title might show `ct-events(5/400+)` if 2 new matches were found in the second page.

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200 --next-token <token>
```

(Then client-side: filter results for "bucket")
Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: Backspace removes filter characters

**Given:** The user has typed `/prod` in filter mode
**When:** The user presses Backspace
**Then:** The filter text becomes `/pro` and the list re-filters to show items matching "pro"

**AWS comparison:**
N/A (UI behavior)

---

## 4. Ctrl+R (Refresh)

### Story: Refresh reloads from page 1 only

**Given:** The user has loaded 5 pages of CloudTrail Events (1000 items) by pressing `M` multiple times, with the title showing `ct-events(1000+)`
**When:** The user presses `Ctrl+R`
**Then:** The view shows a loading spinner, fetches only page 1 from AWS, and displays the first 200 results. The title reverts to `ct-events(200+)`. Previously loaded pages 2-5 are discarded.

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200
```

(Fresh call with no continuation token)
Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: Refresh clears the active filter

**Given:** The user has filter `/bucket` active showing `ct-events(3/200+)`
**When:** The user presses `Ctrl+R`
**Then:** The filter text is cleared (header right returns to `? for help` after loading), and fresh page 1 data is displayed without any filter applied. The title shows something like `ct-events(200+)`.

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200
```

Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: Refresh resets cursor to position 0

**Given:** The user has scrolled down to row 450 in a multi-page CloudTrail Events list
**When:** The user presses `Ctrl+R`
**Then:** After the fresh page loads, the cursor is at the first row (position 0) at the top of the list

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200
```

Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: Refresh shows loading spinner

**Given:** The user is viewing any resource list
**When:** The user presses `Ctrl+R`
**Then:** A loading spinner with descriptive text (e.g., "Fetching CloudTrail Events...") is displayed in the frame content area while the data is being fetched

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200
```

Expected fields visible: (spinner visible, no data rows until load completes)

---

### Story: User can press M after refresh to load more

**Given:** The user pressed `Ctrl+R` and the view now shows fresh page 1 data as `ct-events(200+)`
**When:** The user presses `M`
**Then:** The next page loads and appends to the data. The title updates to `ct-events(400+)` or `ct-events(400)` depending on whether more pages exist.

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200 --next-token <new-token-from-page-1>
```

Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

## 5. Navigation Caching

### Story: Back-navigation from detail preserves all loaded pages

**Given:** The user has loaded 3 pages of CloudTrail Events (600 items) and the title shows `ct-events(600+)`. The cursor is at row 450.
**When:** The user presses `Enter` to open the detail view for the selected item, then presses `Esc` to go back
**Then:** The resource list shows all 600 items with the cursor at the same row 450. No data is re-fetched.

**AWS comparison:**
N/A (navigation behavior, no AWS call on back-nav)
Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: Back-navigation from YAML view preserves loaded pages

**Given:** The user has loaded 2 pages of EBS Volumes (400 items) showing `ebs(400+)`. The cursor is on a specific volume.
**When:** The user presses `y` to open the YAML view, then presses `Esc` to go back
**Then:** The resource list shows all 400 volumes with the cursor at the same position

**AWS comparison:**
N/A (navigation behavior)
Expected fields visible: Name, Volume ID, State, Size (GiB), Type, IOPS, Encrypted, Attached To, AZ, Created

---

### Story: Esc to main menu destroys the view

**Given:** The user has loaded 5 pages of CloudTrail Events (1000 items)
**When:** The user presses `Esc` to return to the main menu
**Then:** The CloudTrail Events view and all its loaded data are destroyed. The main menu is displayed.

**AWS comparison:**
N/A (navigation behavior)

---

### Story: Re-entering from main menu starts fresh

**Given:** The user previously loaded 5 pages of CloudTrail Events, then pressed `Esc` to return to the main menu
**When:** The user selects CloudTrail Events from the main menu and presses `Enter`
**Then:** A fresh fetch begins from page 1. The view shows a loading spinner, then displays `ct-events(200+)` (first page only). None of the previously loaded data is reused.

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200
```

Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: Profile switch clears entire navigation stack

**Given:** The user has loaded multiple pages of data in a resource list view
**When:** The user switches to a different AWS profile (via `:ctx` command)
**Then:** The entire view stack is cleared. The user returns to the main menu. All previously loaded data is discarded. Any subsequent resource list navigation will fetch fresh data using the new profile's credentials.

**AWS comparison:**

```
export AWS_PROFILE=new-profile
aws cloudtrail lookup-events --max-results 200
```

(Equivalent: switching credentials and starting over)

---

### Story: Region switch clears entire navigation stack

**Given:** The user has loaded multiple pages of EBS Snapshots in us-east-1
**When:** The user switches to a different region (via `:region` command)
**Then:** The entire view stack is cleared. All previously loaded data is discarded. Any subsequent navigation will fetch data from the new region.

**AWS comparison:**

```
aws ec2 describe-snapshots --owner-id self --region us-west-2 --max-results 200
```

Expected fields visible: Name, Snapshot ID, State, Volume ID, Size (GiB), Encrypted, Description, Started, Progress

---

### Story: No TTL-based cache expiration

**Given:** The user loaded CloudTrail Events 30 minutes ago and has been viewing detail views, YAML views, and navigating back to the list
**When:** The user navigates back to the CloudTrail Events list after 30 minutes
**Then:** All previously loaded data is still present. No automatic re-fetch occurs. The user must press `Ctrl+R` to refresh if they want updated data.

**AWS comparison:**
N/A (caching behavior has no AWS equivalent; contrast with k9s which auto-refreshes)

---

## 6. Sorting

### Story: Sort operates on loaded data only

**Given:** The user is viewing a truncated list of CloudTrail Events showing `ct-events(200+)`
**When:** The user presses `A` to sort by age/time
**Then:** The 200 loaded items are sorted by time. A sort indicator arrow (`↑` or `↓`) appears next to the Time column header. The `+` in the frame title persists to indicate partial data. No additional API calls are made.

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200 | sort by EventTime
```

(a9s sorts the 200 loaded items; the global sort order across all events is unknown without loading everything)
Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: Sort indicator coexists with plus suffix

**Given:** The user is viewing `ct-events(200+)` and presses `N` to sort by name
**When:** The sort completes
**Then:** The column header shows `Event Name↑` (ascending) and the frame title still shows `ct-events(200+)`. Both indicators are visible simultaneously. No extra warning about partial sort is displayed.

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200 | sort by EventName
```

Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: Sort toggles between ascending and descending

**Given:** The user is viewing a resource list sorted by name in ascending order (arrow `↑`)
**When:** The user presses `N` again
**Then:** The sort order reverses to descending and the indicator changes to `↓`

**AWS comparison:**
N/A (UI sorting behavior)

---

### Story: Sort composes with filter

**Given:** The user is viewing `ct-events(3/200+)` with filter `/bucket` active
**When:** The user presses `A` to sort by age
**Then:** The 3 filtered items are displayed sorted by time. The frame title remains `ct-events(3/200+)`. The sort indicator appears on the Time column header.

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200 | grep bucket | sort by EventTime
```

Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: Loading more data re-sorts and re-filters combined dataset

**Given:** The user has sorted by name (ascending) and has filter `/bucket` active, showing `ct-events(3/200+)`
**When:** The user presses `M` to load the next page
**Then:** The new page's data is appended to the full dataset. The entire combined dataset is re-sorted by name and re-filtered for "bucket". Both the sort and filter counts update. For example, the title might change to `ct-events(5/400+)` with all 5 matching items sorted alphabetically.

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200 --next-token <token>
```

(Then client-side: sort + filter the combined 400 items)
Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: Sort by ID on EBS Volumes

**Given:** The user is viewing a truncated list of EBS Volumes showing `ebs(200+)`
**When:** The user presses `I` to sort by ID
**Then:** The 200 loaded volumes are sorted by Volume ID. The sort indicator appears on the Volume ID column header.

**AWS comparison:**

```
aws ec2 describe-volumes --max-results 200 | sort by VolumeId
```

Expected fields visible: Name, Volume ID, State, Size (GiB), Type, IOPS, Encrypted, Attached To, AZ, Created

---

## 7. Edge Cases

### Story: Empty filter results on truncated list shows zero count with load-more hint

**Given:** The user is viewing a truncated CloudTrail Events list showing `ct-events(200+)`
**When:** The user presses `/` and types `xyz123abc` (a string matching no loaded items)
**Then:** The frame title shows `ct-events(0/200+)`. The content area displays "No resources found". The load-more hint `-- M: load more (filter applies to loaded data only) --` is still visible, indicating the user can load more pages that might contain matches.

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200 | grep xyz123abc
```

(Zero results from loaded data, but more data may exist on the server)
Expected fields visible: (none -- "No resources found" message displayed)

---

### Story: Esc clears filter before popping the view (two-step)

**Given:** The user is viewing `ct-events(3/200+)` with filter `/bucket` active
**When:** The user presses `Esc` once
**Then:** The filter is cleared. All 200 loaded items are displayed again. The title reverts to `ct-events(200+)`. The user remains on the CloudTrail Events list (the view is NOT popped).

**AWS comparison:**
N/A (UI navigation behavior)
Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: Second Esc after clearing filter pops the view

**Given:** The user just cleared a filter by pressing `Esc` (previous story) and is now viewing `ct-events(200+)` with no filter active
**When:** The user presses `Esc` again
**Then:** The view is popped from the stack and the user returns to the previous view (main menu or parent view)

**AWS comparison:**
N/A (UI navigation behavior)

---

### Story: Rapid M presses are ignored while loading

**Given:** The user is viewing a truncated list and presses `M` to load more. The title shows `ct-events(200+ loading...)`
**When:** The user presses `M` again while loading is still in progress
**Then:** The second `M` press is a no-op. Only one API call is made. No duplicate data appears.

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200 --next-token <token>
```

(Only one call in flight at a time)

---

### Story: API error during load-more preserves existing data

**Given:** The user is viewing `ct-events(200+)` and presses `M` to load more
**When:** The API call fails (e.g., network timeout, throttling, expired credentials)
**Then:** An error flash message appears in the header (red text). The existing 200 items remain displayed and accessible. The title reverts to `ct-events(200+)` (loading indicator is removed). The `M` key becomes responsive again so the user can retry.

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200 --next-token <token>
# Returns: An error occurred (ThrottlingException)
```

Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only (existing data preserved)

---

### Story: G jumps to last loaded item without auto-loading

**Given:** The user is viewing a truncated list `ct-events(200+)` with the cursor at the top
**When:** The user presses `G` (jump to bottom)
**Then:** The cursor moves to the last loaded item (row 200). The load-more hint `-- M: load more --` is visible below the last row. No additional pages are automatically fetched. The user must press `M` explicitly to load more.

**AWS comparison:**
N/A (UI navigation behavior -- contrast with infinite-scroll UIs that auto-load)
Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: No auto-loading when scrolling past bottom

**Given:** The user is viewing a truncated list and scrolls down using `j` or `↓` to reach the last loaded row
**When:** The user continues pressing `j` or `↓` at the bottom
**Then:** The cursor stays on the last row. No automatic page load is triggered. The load-more hint remains visible. The user must press `M` to fetch more data.

**AWS comparison:**
N/A (design decision: explicit pagination only, no infinite scroll)

---

### Story: Empty filter results on fully-loaded list shows zero count without hint

**Given:** The user has loaded all pages of CloudTrail Events showing `ct-events(1847)` (no `+` suffix)
**When:** The user presses `/` and types `xyz123abc`
**Then:** The frame title shows `ct-events(0/1847)`. The content area displays "No resources found". No load-more hint appears because all data is already loaded.

**AWS comparison:**

```
aws cloudtrail lookup-events | grep xyz123abc
```

(Zero results from the complete dataset)

---

## 8. Non-Paginated Resource Types (Baseline)

### Story: Non-paginated resource list has no pagination indicators

**Given:** The user opens EC2 Instances and all 42 instances are returned in a single API response
**When:** The resource list is displayed
**Then:** The frame title reads `ec2(42)` (no `+`). No load-more hint appears. The `M` key has no effect. Filter and sort operate on the complete dataset.

**AWS comparison:**

```
aws ec2 describe-instances
```

Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

---

### Story: Filter on non-paginated list shows filtered/total without plus

**Given:** The user is viewing EC2 Instances showing `ec2(42)`
**When:** The user presses `/` and types `prod`
**Then:** The frame title shows `ec2(3/42)` (no `+`). Only matching rows are displayed. No load-more hint appears.

**AWS comparison:**

```
aws ec2 describe-instances | grep -i prod
```

Expected fields visible: Name, State, Lifecycle, Type, Private IP, Public IP, Instance ID, Launch Time

---

## 9. Help Screen Context

### Story: Help screen shows M key hint for paginated views

**Given:** The user is viewing a truncated resource list (e.g., `ct-events(200+)`)
**When:** The user presses `?` to open the help screen
**Then:** The help screen is displayed in a 4-column layout. The `M` key binding for "load more" is visible in the RESOURCE or HOTKEYS column. Category headers are displayed in orange uppercase. Keys are displayed in green bold.

**AWS comparison:**
N/A (help screen is UI-only)

---

### Story: Help screen does not show M key for non-paginated views

**Given:** The user is viewing a non-paginated resource list (e.g., `ec2(42)`)
**When:** The user presses `?` to open the help screen
**Then:** The help screen does not include the `M` key binding for "load more" since there is no additional data to load

**AWS comparison:**
N/A (help screen is UI-only)

---

## 10. Multi-Page Workflow (End-to-End)

### Story: Complete pagination workflow with filter, sort, and load-more

**Given:** The user opens CloudTrail Events from the main menu
**When:** The user performs the following sequence:
1. Initial load shows `ct-events(200+)` with the `-- M: load more --` hint
2. User presses `N` to sort by Event Name -- items sort alphabetically, arrow `↑` appears on Event Name
3. User presses `/` and types `s3` -- title becomes `ct-events(45/200+)`, hint changes to include filter warning
4. User presses `M` -- hint changes to `-- loading... --`, title shows `ct-events(45/200+ loading...)`
5. New page arrives -- title updates to `ct-events(87/400+)` (42 new matches from page 2), all sorted by name
6. User presses `Esc` -- filter clears, title reverts to `ct-events(400+)`, all 400 items visible and sorted
7. User presses `Ctrl+R` -- spinner shows, fresh page 1 loads, title shows `ct-events(200+)`, sort cleared
**Then:** Each step produces the expected frame title, hint text, and data state as described above

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200
aws cloudtrail lookup-events --max-results 200 --next-token <page2-token>
aws cloudtrail lookup-events --max-results 200  # after refresh
```

Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

---

### Story: Drill into detail from filtered paginated list and return

**Given:** The user is viewing `ct-events(5/400+)` with filter `/bucket` and sort by time active
**When:** The user moves the cursor to "CreateBucket" and presses `Enter` to view the detail
**Then:** The detail view shows fields: EventId, EventName, EventTime, EventSource, Username, ReadOnly, AccessKeyId, Resources, CloudTrailEvent
**When:** The user presses `Esc` to return to the list
**Then:** The list view is restored with all 400 loaded items, filter `/bucket` still active showing 5 matches, sort by time still active, and cursor on the same "CreateBucket" row

**AWS comparison:**

```
aws cloudtrail lookup-events --max-results 200
# Detail equivalent:
aws cloudtrail lookup-events --lookup-attributes AttributeKey=EventId,AttributeValue=<event-id>
```

Expected detail fields: EventId, EventName, EventTime, EventSource, Username, ReadOnly, AccessKeyId, Resources, CloudTrailEvent

---

## 11. Pagination with Other Resource Types

### Story: EBS Snapshots pagination

**Given:** The user opens EBS Snapshots, which returns 200 results on the first page with more available
**When:** The resource list loads
**Then:** The frame title reads `ebs-snap(200+)` and the load-more hint `-- M: load more --` appears at the bottom

**AWS comparison:**

```
aws ec2 describe-snapshots --owner-id self --max-results 200
```

Expected fields visible: Name, Snapshot ID, State, Volume ID, Size (GiB), Encrypted, Description, Started, Progress

---

### Story: AMI pagination

**Given:** The user opens AMIs, which returns 200 results on the first page with more available
**When:** The resource list loads
**Then:** The frame title reads `ami(200+)` and the load-more hint appears

**AWS comparison:**

```
aws ec2 describe-images --owners self --max-results 200
```

Expected fields visible: Name, Image ID, State, Arch, Platform, Root Device, Created, Public

---

### Story: EBS Volumes pagination with load-more

**Given:** The user is viewing `ebs(200+)` showing 200 EBS Volumes
**When:** The user presses `M` to load the next page, which returns 150 more volumes with no further pages
**Then:** The frame title changes to `ebs(350)` (no `+`). The load-more hint disappears. All 350 volumes are displayed.

**AWS comparison:**

```
aws ec2 describe-volumes --max-results 200 --next-token <token>
```

(Second page returns 150 results with no NextToken)
Expected fields visible: Name, Volume ID, State, Size (GiB), Type, IOPS, Encrypted, Attached To, AZ, Created

---

## 12. Color and Styling

### Story: Load-more hint uses dim styling

**Given:** The user is viewing a truncated resource list
**When:** The load-more hint is displayed at the bottom
**Then:** The hint text is rendered in the dim color (`#565f89`) consistent with other non-interactive text in the Tokyo Night Dark palette

**AWS comparison:**
N/A (visual styling)

---

### Story: Loading indicator in frame title uses bold styling

**Given:** The user has pressed `M` to load more data
**When:** The frame title shows `ct-events(200+ loading...)`
**Then:** The `loading...` text and the `+` suffix are rendered in bold with the standard frame title color (`#c0caf5`)

**AWS comparison:**
N/A (visual styling)

---

### Story: Frame border uses standard border color

**Given:** The user is viewing any paginated resource list
**When:** The frame is rendered
**Then:** The frame border (top, sides, bottom) uses the standard border color (`#414868`). No new colors are introduced for pagination-specific elements.

**AWS comparison:**
N/A (visual styling)
