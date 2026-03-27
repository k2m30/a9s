# QA User Stories: CloudTrail Event History Resource Type (Issue #59)

Covers the CloudTrail Event History as a new top-level resource type -- the audit log
of API calls across the entire AWS account. This is distinct from the existing
CloudTrail Trails resource (`trail`), which shows trail configuration. This resource
shows the actual events (who did what, when).

All stories are written from a black-box perspective against the design spec and
`views.yaml` / `views_reference.yaml` configuration files.

AWS CLI equivalents are cited so testers can verify data parity.

---

## A. Main Menu Integration

### A.1 Resource Type Appears in Main Menu

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I launch a9s and the main menu is displayed. | CloudTrail Event History appears as a row in the resource type list, within the MONITORING category alongside CloudWatch Alarms, CloudWatch Log Groups, and CloudTrail Trails. The row shows a display name (e.g., "CloudTrail Events") and a dimmed shortname alias (e.g., `:ct-events`). |
| A.1.2 | I select the CloudTrail Event History entry and press Enter. | The view transitions to the CloudTrail Event History list. A loading spinner appears while the API call is in flight. |

### A.2 Command Mode Navigation

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | I am in any view and press `:`. I type the shortname for CloudTrail events and press Enter. | The view navigates to the CloudTrail Event History list. |
| A.2.2 | I type a partial match and press Tab. | The shortname autocompletes. |

---

## B. CloudTrail Event History -- List View

### B.1 Loading State

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | I select CloudTrail Event History from the main menu and the API has not yet responded. | A spinner (animated dot) is displayed centered inside the frame. The text reads "Fetching CloudTrail events..." (or similar). The frame title shows the resource shortname with no count. The header shows "? for help" on the right. |
| B.1.2 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. The spinner continues uninterrupted. |
| B.1.3 | The API responds successfully with event data. | The spinner disappears. The table renders with column headers and rows. The frame title updates to include the event count (e.g., "ct-events(250)"). |
| B.1.4 | The API responds with an error (e.g., no credentials, AccessDenied, network timeout). | The spinner disappears. A red error flash message appears in the header right side (e.g., "Error: AccessDeniedException"). The frame content area shows an appropriate empty or error state. |

**AWS comparison:**
```
aws cloudtrail lookup-events --max-results 50
```
Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

### B.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| B.2.1 | The API returns zero events (e.g., brand-new account with no activity, or very narrow time filter). | The frame title shows count 0 (e.g., "ct-events(0)"). The content area shows a centered message (e.g., "No CloudTrail events found") with a hint to try a different time range or region. No column headers are shown (or headers with no data rows). |
| B.2.2 | I press ctrl+r on the empty state. | The loading spinner appears again while a fresh API call is made. |

### B.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| B.3.1 | Events load and the table renders. | Seven columns are displayed: "Time" (width ~22), "Event Name" (width ~26), "User" (width ~24), "Source" (width ~24), "Resource Type" (width ~20), "Resource Name" (width ~24), "Read Only" (width ~10). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| B.3.2 | I verify column data against `aws cloudtrail lookup-events`. | "Time" maps to `.Events[].EventTime`. "Event Name" maps to `.Events[].EventName`. "User" maps to `.Events[].Username`. "Source" maps to `.Events[].EventSource`. "Resource Type" maps to `.Events[].Resources[0].ResourceType`. "Resource Name" maps to `.Events[].Resources[0].ResourceName`. "Read Only" maps to `.Events[].ReadOnly`. Every event returned by the CLI appears as a row in the table. |
| B.3.3 | An event has no Resources (the Resources array is empty or null). | The "Resource Type" and "Resource Name" columns show a dash "-" or are empty for that row. No crash occurs. |
| B.3.4 | A Username value is very long (e.g., an assumed role ARN). | The username is truncated to fit the column width. No row wrapping occurs. |
| B.3.5 | The terminal is narrower than the combined column widths. | The rightmost column(s) are hidden (not truncated mid-value). Horizontal scroll with h/l is available to reveal hidden columns. |

**AWS comparison:**
```
aws cloudtrail lookup-events --query 'Events[].[EventTime,EventName,Username,EventSource,Resources[0].ResourceType,Resources[0].ResourceName,ReadOnly]' --output table
```
Expected fields visible: Time, Event Name, User, Source, Resource Type, Resource Name, Read Only

### B.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| B.4.1 | 250 events are loaded. | The frame top border shows the title centered (e.g., "ct-events(250)") with equal-length dashes on both sides. |
| B.4.2 | A filter is active and matches 15 of 250 events. | The frame title reads "ct-events(15/250)". |
| B.4.3 | A filter is active and matches 0 events. | The frame title reads "ct-events(0/250)". The content area is empty (no rows). |

### B.5 Navigation

| ID | Story | Expected |
|----|-------|----------|
| B.5.1 | I press j (or down-arrow) with the first event selected. | The selection cursor moves to the second event. |
| B.5.2 | I press k (or up-arrow) with the second event selected. | The selection cursor moves back to the first event. |
| B.5.3 | I press g. | The selection jumps to the very first event in the list. |
| B.5.4 | I press G. | The selection jumps to the very last event in the list. |
| B.5.5 | I press PageDown (or ctrl+d). | The selection moves down by one page of visible rows. |
| B.5.6 | I press PageUp (or ctrl+u). | The selection moves up by one page of visible rows. |
| B.5.7 | There are more events than fit on screen. I scroll past the visible area. | The table scrolls to keep the selected row visible. Column headers remain fixed. |

### B.6 Sorting

| ID | Story | Expected |
|----|-------|----------|
| B.6.1 | I press N on the event list. | Rows are sorted by event name in ascending order. The "Event Name" column header shows a sort indicator "^" (up-arrow). |
| B.6.2 | I press N again. | Sort order toggles to descending. The indicator changes to a down-arrow. |
| B.6.3 | I press A on the event list. | Rows are sorted by time (age) in ascending order (oldest first). The "Time" column header shows the sort indicator. |
| B.6.4 | I press A again. | Sort order toggles to descending (newest first). |
| B.6.5 | I press S on the event list. | Rows are sorted by a status-like field (e.g., Read Only true/false or Event Source). The appropriate column header shows the sort indicator. |

### B.7 Filter

| ID | Story | Expected |
|----|-------|----------|
| B.7.1 | I press /. | The header right side changes from "? for help" to "/|" (amber/bold, with cursor). Filter mode is active. |
| B.7.2 | I type "RunInstances" in filter mode. | Only rows whose event name contains "RunInstances" (case-insensitive) are displayed. The frame title updates to show matched/total. |
| B.7.3 | I type "s3" in filter mode. | Rows matching "s3" in any visible column (e.g., EventSource "s3.amazonaws.com", ResourceType "AWS::S3::Bucket") are displayed. |
| B.7.4 | I press Escape in filter mode. | The filter is cleared. All rows reappear. |
| B.7.5 | I type a string that matches no events. | Zero rows displayed. Frame title shows "ct-events(0/N)". |

### B.8 Row Coloring

| ID | Story | Expected |
|----|-------|----------|
| B.8.1 | An event is a read-only event (ReadOnly = "true", e.g., DescribeInstances). | The entire row is rendered in DIM (#565f89), indicating passive/read operations. |
| B.8.2 | An event is a write event (ReadOnly = "false", e.g., RunInstances, TerminateInstances). | The entire row is rendered in PLAIN color (#c0caf5) or a highlighted color, making write operations stand out against read events. |
| B.8.3 | An event name contains "Delete" or "Terminate" (destructive action). | The entire row is rendered in RED (#f7768e), making destructive operations immediately visible. |
| B.8.4 | I select any row. | The selected row has full-width blue background (#7aa2f7), dark foreground (#1a1b26), bold text. Content-based coloring is overridden. |
| B.8.5 | I move selection away from a colored row. | The previously selected row reverts to its content-based coloring. |
| B.8.6 | Alternating rows have subtle background difference. | Alternating unselected rows show the subtle alternating background (#1e2030) for readability. |

### B.9 Horizontal Scroll

| ID | Story | Expected |
|----|-------|----------|
| B.9.1 | All seven columns do not fit in the terminal width. | Only the leftmost columns that fit are visible. |
| B.9.2 | I press l (or right-arrow). | The visible column window shifts right, revealing hidden right columns while hiding leftmost columns. Column headers scroll in sync with data. |
| B.9.3 | I press h (or left-arrow). | The visible column window shifts left. |

### B.10 Copy (c)

| ID | Story | Expected |
|----|-------|----------|
| B.10.1 | I select an event and press c. | The event ID (or event name) is copied to the system clipboard. A green flash message "Copied!" appears in the header right side. |
| B.10.2 | After ~2 seconds. | The "Copied!" flash message auto-clears and the header right reverts to "? for help". |

### B.11 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| B.11.1 | I press ctrl+r on the event list. | The loading spinner appears. A fresh `LookupEvents` API call is made. When it completes, the table repopulates with current data. |
| B.11.2 | New events occurred since the last load. I press ctrl+r. | The new events appear in the refreshed list. The count in the frame title updates. |
| B.11.3 | I had a filter active and press ctrl+r. | The data refreshes. The filter remains applied to the new data. |

### B.12 Escape (Back)

| ID | Story | Expected |
|----|-------|----------|
| B.12.1 | I press Escape on the CloudTrail Event History list. | I return to the main menu. The main menu shows the resource type list with CloudTrail Event History among the entries. |

### B.13 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| B.13.1 | I press ? on the event list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: RESOURCE, GENERAL, NAVIGATION, HOTKEYS. |
| B.13.2 | I press any key on the help screen. | The help screen closes and the event list table reappears. |

### B.14 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| B.14.1 | I press : on the event list. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| B.14.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list. |
| B.14.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The event list remains. |

---

## C. Default Time Window

### C.1 Default Behavior -- Last 1 Hour

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | I open CloudTrail Event History with no custom configuration. | Only events from approximately the last 1 hour are fetched and displayed. The oldest visible event is no older than ~1 hour. This keeps the initial load fast. |
| C.1.2 | No events occurred in the last 1 hour. | The empty state is displayed with a message like "No CloudTrail events in the last 1 hour". |
| C.1.3 | I verify the time window against AWS CLI. | `aws cloudtrail lookup-events --start-time $(date -u -v-1H +%Y-%m-%dT%H:%M:%SZ)` returns the same events as displayed in the list. |

**AWS comparison:**
```
aws cloudtrail lookup-events --start-time $(date -u -v-1H +%Y-%m-%dT%H:%M:%SZ) --max-results 50
```

### C.2 Configurable Time Window

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | I configure `logs.time_range: 6h` in `~/.a9s/config.yaml` and open CloudTrail Event History. | Events from the last 6 hours are displayed. The oldest event is no older than 6 hours. |
| C.2.2 | I configure `logs.time_range: 24h`. | Events from the last 24 hours are displayed. |
| C.2.3 | I configure `logs.time_range: 7d`. | Events from the last 7 days are displayed. This may produce a large number of events subject to max_events limits. |

**AWS comparison:**
```
aws cloudtrail lookup-events --start-time $(date -u -v-6H +%Y-%m-%dT%H:%M:%SZ)
```

---

## D. CloudTrail Event Detail View

### D.1 Detail View (d key)

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | I select an event and press d. | The detail view opens for the selected event. The frame title shows identifying information (e.g., the event name or event ID). |
| D.1.2 | The detail view shows key-value pairs. | The detail view displays fields such as: EventId, EventTime, EventName, EventSource, Username, SourceIPAddress, ReadOnly, Resources. Keys are colored blue (#7aa2f7), values in white (#c0caf5). |
| D.1.3 | The Resources field contains multiple resource entries. | The resources are displayed as a list or nested structure showing each resource's ResourceType and ResourceName. |
| D.1.4 | I press j/k to scroll in the detail view. | The viewport scrolls up and down by one line. |
| D.1.5 | I press g to jump to top, G to jump to bottom. | Navigation works as in other detail views. |
| D.1.6 | I press w to toggle word wrap. | Long values wrap to the next line (or unwrap if already wrapped). |
| D.1.7 | I press c in the detail view. | The full detail content is copied to the clipboard. A "Copied!" flash appears. |
| D.1.8 | I press Escape in the detail view. | I return to the CloudTrail Event History list. The cursor position is preserved. |

**AWS comparison:**
```
aws cloudtrail lookup-events --query 'Events[0]' --output json
```
Expected fields visible: EventId, EventTime, EventName, EventSource, Username, ReadOnly, Resources

### D.2 YAML View (y key) -- Full CloudTrail Event JSON

| ID | Story | Expected |
|----|-------|----------|
| D.2.1 | I select an event and press y. | The YAML view opens showing the full CloudTrail event data as syntax-highlighted YAML. The frame title includes the event identifier and "yaml". |
| D.2.2 | The CloudTrailEvent field (raw JSON of the full API call details) is rendered. | The raw CloudTrailEvent JSON is displayed as YAML. This includes requestParameters, responseElements, userIdentity, and other fields not visible in the list or detail views. |
| D.2.3 | YAML syntax coloring is applied. | Keys are blue (#7aa2f7), string values green (#9ece6a), numbers orange (#ff9e64), booleans purple (#bb9af7), null values dim (#565f89). |
| D.2.4 | I press j/k/g/G to scroll in the YAML view. | Scrolling works as in other YAML views. |
| D.2.5 | I press c in the YAML view. | The full YAML content is copied to the clipboard. |
| D.2.6 | I press Escape in the YAML view. | I return to the CloudTrail Event History list. |

**AWS comparison:**
```
aws cloudtrail lookup-events --query 'Events[0].CloudTrailEvent' --output text | python3 -m json.tool
```

---

## E. Pagination and Data Limits

### E.1 Lazy Pagination

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | The initial API call returns the first page of events (e.g., 50) with a NextToken. | The first page of events is displayed immediately. As I scroll toward the bottom, additional pages are fetched lazily. A "Loading more..." spinner appears during each page fetch. |
| E.1.2 | I scroll to the bottom and all available pages have been loaded. | No further API calls are made. The total count in the frame title reflects all loaded events. |
| E.1.3 | I have `logs.max_events: 500` configured. I am viewing a busy account with thousands of events. | At most 500 events are loaded. When scrolling to the bottom after reaching the limit, a dim hint reads: "Showing 500 most recent -- more available". |

**AWS comparison:**
```
aws cloudtrail lookup-events --max-results 50
# Note: max-results per page is 50. Multiple pages via NextToken.
```

### E.2 Data Volume

| ID | Story | Expected |
|----|-------|----------|
| E.2.1 | The account has very high API activity (thousands of events per hour). | The default 1-hour time window keeps the initial load manageable. Lazy pagination loads additional pages only as the user scrolls. The application remains responsive. |
| E.2.2 | The configured time range is 7 days in a busy account. | The application handles large result sets gracefully. Pagination limits prevent loading the entire 7-day history at once. The max_events configuration (default 1000) caps total loaded events. |

---

## F. Event Source Diversity

### F.1 Various AWS Service Sources

| ID | Story | Expected |
|----|-------|----------|
| F.1.1 | The event list contains events from EC2 (ec2.amazonaws.com). | The "Source" column shows "ec2.amazonaws.com". The "Event Name" column shows actions like "RunInstances", "TerminateInstances", "DescribeInstances". |
| F.1.2 | The event list contains events from S3 (s3.amazonaws.com). | The "Source" column shows "s3.amazonaws.com". Event names include "PutObject", "DeleteBucket", "CreateBucket". |
| F.1.3 | The event list contains events from IAM (iam.amazonaws.com). | The "Source" column shows "iam.amazonaws.com". Event names include "CreateRole", "AttachRolePolicy". |
| F.1.4 | The event list contains events from STS (sts.amazonaws.com). | The "Source" column shows "sts.amazonaws.com". Event names include "AssumeRole", "GetCallerIdentity". |
| F.1.5 | I filter by typing "ec2" to see only EC2-related events. | Only rows with "ec2" appearing in any visible column are displayed (matching EventSource "ec2.amazonaws.com" or ResourceType "AWS::EC2::Instance"). |

### F.2 Various User Types

| ID | Story | Expected |
|----|-------|----------|
| F.2.1 | An event was performed by a human user via console. | The "User" column shows the IAM username (e.g., "admin@company.com"). |
| F.2.2 | An event was performed by an assumed role. | The "User" column shows the role session name or a recognizable identifier. |
| F.2.3 | An event was performed by an AWS service. | The "User" column shows the service identifier (e.g., "autoscaling.amazonaws.com", "elasticloadbalancing.amazonaws.com"). |
| F.2.4 | An event has no Username (null). | The "User" column shows a dash "-" or is empty. No crash. |

---

## G. Error Handling

### G.1 Permission Errors

| ID | Story | Expected |
|----|-------|----------|
| G.1.1 | The IAM user/role does not have `cloudtrail:LookupEvents` permission. | A red error flash appears in the header: "Error: AccessDeniedException" (or similar). The event list is empty. The application remains navigable. |
| G.1.2 | The credentials are expired. | A red error flash appears. The application does not crash. The user can switch profiles or regions. |

### G.2 API Cost Warning

| ID | Story | Expected |
|----|-------|----------|
| G.2.1 | CloudTrail LookupEvents is free (no per-request charge for management events). | No cost warning is displayed. (Note: LookupEvents is free, unlike Cost Explorer.) |

---

## H. View Stack Integration

### H.1 Navigation Stack

| ID | Story | Expected |
|----|-------|----------|
| H.1.1 | Main Menu -> CloudTrail Events -> Event Detail -> YAML; then Escape three times. | Each Escape pops one level: YAML -> Detail -> Event List -> Main Menu. No state is lost at any intermediate level. |
| H.1.2 | Main Menu -> CloudTrail Events -> Event Detail (d) -> YAML (y); then Escape twice. | YAML -> Detail -> Event List. The cursor is still on the same event I had selected. |
| H.1.3 | I navigate to CloudTrail Events via command mode from another resource list. | The event list loads correctly. Pressing Escape returns to the main menu (not the previous resource list). |

---

## I. Demo Mode

### I.1 Synthetic Fixtures

| ID | Story | Expected |
|----|-------|----------|
| I.1.1 | I launch a9s in demo mode (e.g., `a9s --demo`). I navigate to CloudTrail Event History. | Synthetic CloudTrail events are displayed with realistic data: diverse event names, multiple users, various AWS service sources, and a mix of read and write events. No real AWS API calls are made. |
| I.1.2 | The demo data includes events with missing Resources. | Some events show empty Resource Type and Resource Name columns, demonstrating graceful handling of null fields. |

---

## J. Cross-Cutting Concerns

### J.1 Header Consistency

| ID | Story | Expected |
|----|-------|----------|
| J.1.1 | In every CloudTrail Event History view (list, detail, YAML), the header displays: "a9s" (accent bold), version (dim), profile:region (bold). | Visual inspection confirms across all views. |
| J.1.2 | The header right side shows "? for help" in normal mode across all views. | Confirmed in list, detail, and YAML views. |

### J.2 Terminal Resize

| ID | Story | Expected |
|----|-------|----------|
| J.2.1 | I resize the terminal while viewing the CloudTrail event list. | The layout reflows. Column visibility adjusts. The frame redraws correctly. |
| J.2.2 | I resize to below 60 columns. | An error message appears: "Terminal too narrow. Please resize." |
| J.2.3 | I resize to below 7 lines. | An error message appears: "Terminal too short. Please resize." |

### J.3 Alternating Row Colors

| ID | Story | Expected |
|----|-------|----------|
| J.3.1 | The event list has more than 2 rows. | Alternating rows have a subtle background color difference (#1e2030) for readability. Selected row always has blue background regardless. |
