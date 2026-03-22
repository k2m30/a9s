# QA-13: EC2 Instance Logs View

Exhaustive black-box user stories for the EC2 Logs feature: log source menu,
CloudTrail events, system console output, CloudWatch Logs (group list + log
viewer), instance status events, VPC flow logs, and SSM command history
(including command output).

This is the deepest view stack in a9s -- up to 4 levels for the CloudWatch
Logs path: EC2 List -> Log Sources -> CW Log Groups -> Log Viewer.

Design doc: `docs/design/ec2-logs-view.md`

AWS CLI equivalents are cited so testers can verify data parity.

---

## A. Entry Point (l key on EC2 List)

### A.1 Opening the Logs View

| # | Story | Expected |
|---|-------|----------|
| A.1.1 | I am on the EC2 instance list with an instance selected. I press `l`. | The view transitions to the log source menu. The frame title reads `log-sources(6) -- i-XXXX (instance-name)` where `i-XXXX` is the instance ID and `instance-name` is the Name tag of the selected instance. |
| A.1.2 | I press `l` while the EC2 list is still loading (spinner visible). | Nothing happens. The key is ignored until data loads. |
| A.1.3 | I press `l` on a terminated instance. | The log source menu opens. Sources that are unavailable for terminated instances show `Unavailable (terminated)` in dim text. CloudTrail is still available (up to 90 days of history). |
| A.1.4 | I press `l` on an instance and then press `Esc` on the log source menu. | I return to the EC2 instance list. The cursor is on the same instance I had selected before pressing `l`. |
| A.1.5 | The `l` key does not appear on resource list views for any resource type other than EC2. | Pressing `l` on S3, RDS, Redis, EKS, Lambda, or any other resource list has no effect (horizontal scroll uses `h`/`l` -- `l` scrolls columns right on non-EC2 views). |

**AWS comparison:**
No single CLI command -- this opens a menu of multiple log source APIs.

---

## B. Log Source Menu

### B.1 Loading State

| # | Story | Expected |
|---|-------|----------|
| B.1.1 | I press `l` on an EC2 instance and the availability checks have not completed. | Six log sources are listed. The three "always available" sources (CloudTrail Events, System Console Output, Instance Status Events) immediately show `Available` in green. The three conditional sources (CloudWatch Logs, VPC Flow Logs, SSM Command History) each show `Checking...` in yellow with a spinner. |
| B.1.2 | I press `j`/`k` while conditional sources are still checking. | Navigation works normally among available sources. The cursor skips over sources that are still in `Checking...` state. |
| B.1.3 | All checks complete and CloudWatch has 3 log groups. | The CloudWatch Logs row updates from `Checking...` to `3 log groups` in green. The spinner disappears. The row becomes selectable. |
| B.1.4 | All checks complete and VPC Flow Logs are not configured. | The VPC Flow Logs row updates from `Checking...` to `Not configured` in dim text. The row is dimmed and the cursor skips it. |
| B.1.5 | All checks complete and SSM Command History has no commands. | The SSM row updates from `Checking...` to `No commands found` in dim text. The row is dimmed and the cursor skips it. |

### B.2 Source Availability States

| # | Story | Expected |
|---|-------|----------|
| B.2.1 | A source shows `Available` (green). | The source name on the left is rendered in normal text color (#c0caf5). The status text on the right is green (#9ece6a). The row is selectable (cursor can land on it). |
| B.2.2 | A source shows `N log groups` (green). | Same as `Available` -- the source name is normal text, the count text is green, and the row is selectable. |
| B.2.3 | A source shows `Not configured` (dim). | The source name on the left is rendered in dim text (#565f89). The status text on the right is dim (#565f89). The cursor skips this row when navigating with `j`/`k`. |
| B.2.4 | A source shows `No commands found` (dim). | Same dimming and cursor-skip behavior as `Not configured`. |
| B.2.5 | A source shows `Unavailable (terminated)` (dim). | Same dimming and cursor-skip behavior. Applies to Console Output and Instance Status when the instance is terminated. |
| B.2.6 | A source shows `Access denied` (red). | The source name on the left is normal text (#c0caf5). The status text on the right is red (#f7768e). The row is dimmed and the cursor skips it. |
| B.2.7 | A source shows `Timed out` (red). | Same as `Access denied` -- red status text, cursor skips it. |
| B.2.8 | A source shows `Checking...` (yellow, with spinner). | The spinner animates in yellow (#e0af68). The source name is normal text. The cursor skips the row while it is checking. |

### B.3 All Sources Unavailable

| # | Story | Expected |
|---|-------|----------|
| B.3.1 | All six sources are dimmed (all unavailable or access denied). | All rows are visible but dimmed. The cursor has nowhere to land. Pressing `j`/`k` does nothing. Pressing `Enter` does nothing. Only `Esc`, `?`, `:`, `Ctrl+R`, and `q` are functional. |

### B.4 Frame Title

| # | Story | Expected |
|---|-------|----------|
| B.4.1 | The log source menu is open for instance `i-0abc123def456` named `web-server-prod`. | The frame title reads `log-sources(6) -- i-0abc123def456 (web-server-prod)` centered in the top border between dashes. |
| B.4.2 | The instance has no Name tag. | The frame title reads `log-sources(6) -- i-0abc123def456` with no parenthesized name. |
| B.4.3 | The count is always 6. | The count in `log-sources(6)` is always 6 regardless of how many sources are available. All six are shown; unavailable ones are dimmed. |

### B.5 Navigation

| # | Story | Expected |
|---|-------|----------|
| B.5.1 | I press `j` with the cursor on the first available source. | The cursor moves to the next available source, skipping any unavailable sources in between. |
| B.5.2 | I press `k` with the cursor on the second available source. | The cursor moves to the previous available source. |
| B.5.3 | I press `g`. | The cursor jumps to the first available source in the list. |
| B.5.4 | I press `G`. | The cursor jumps to the last available source in the list. |
| B.5.5 | I press `Enter` on an available source. | The corresponding sub-view opens (see sections C-H below). A spinner appears in the sub-view while data loads. |
| B.5.6 | I press `Enter` on an unavailable/dimmed source. | Nothing happens. No error, no flash message, no navigation. |
| B.5.7 | I press `Enter` on a source that is still in `Checking...` state. | Nothing happens. The source is not selectable until its availability is determined. |

### B.6 Filter

| # | Story | Expected |
|---|-------|----------|
| B.6.1 | I press `/` on the log source menu. | Nothing happens. Filter is not available on this menu (too few items to warrant it, per design spec). |

### B.7 Refresh

| # | Story | Expected |
|---|-------|----------|
| B.7.1 | I press `Ctrl+R` on the log source menu. | All availability checks are re-run. Conditional sources revert to `Checking...` with spinners while their checks execute again. |
| B.7.2 | A previously unavailable source (e.g., CloudWatch) has become available since the last check. I press `Ctrl+R`. | After checks complete, the source updates from dim/unavailable to green/available and becomes selectable. |

### B.8 Escape

| # | Story | Expected |
|---|-------|----------|
| B.8.1 | I press `Esc` on the log source menu. | I return to the EC2 instance list. The cursor is preserved on the same instance. |

### B.9 Help

| # | Story | Expected |
|---|-------|----------|
| B.9.1 | I press `?` on the log source menu. | The help screen replaces the frame content. It shows a 3-column layout with categories: NAVIGATION, GENERAL, ACTIONS. |
| B.9.2 | The NAVIGATION column lists: `<j>` Down, `<k>` Up, `<g>` Top, `<G>` Bottom. | All four navigation keys are listed. |
| B.9.3 | The GENERAL column lists: `<ctrl-r>` Refresh, `<?>` Help, `<:>` Command, `<q>` Quit. | All four general keys are listed. |
| B.9.4 | The ACTIONS column lists: `<enter>` Open source, `<esc>` Back to EC2 list. | Both action keys are listed. |
| B.9.5 | I press any key on the help screen. | The help screen closes and the log source menu reappears. |

### B.10 Command Mode

| # | Story | Expected |
|---|-------|----------|
| B.10.1 | I press `:` on the log source menu. | Command mode activates. The header right side shows `:|` in amber bold. |
| B.10.2 | I type `ec2` and press `Enter`. | The view navigates to the EC2 instance list (effectively going back). |
| B.10.3 | I press `Esc` in command mode. | Command mode is cancelled. The log source menu remains visible. |

### B.11 Terminated Instance

| # | Story | Expected |
|---|-------|----------|
| B.11.1 | I press `l` on a terminated EC2 instance. | The log source menu opens. CloudTrail Events shows `Available` (green). System Console Output shows `Unavailable (terminated)` (dim). CloudWatch Logs may show `Not configured` (dim). Instance Status Events shows `Unavailable (terminated)` (dim). |
| B.11.2 | I select CloudTrail on a terminated instance and press `Enter`. | CloudTrail events load normally. Terminated instances still have CloudTrail history for up to 90 days. |

**AWS comparison:**
No single CLI equivalent. The menu aggregates availability from:
`aws cloudtrail lookup-events`, `aws ec2 get-console-output`,
`aws logs describe-log-groups`, `aws ec2 describe-instance-status`,
`aws ec2 describe-flow-logs`, `aws ssm list-commands`

---

## C. CloudTrail Events (Table Sub-View)

### C.1 Loading State

| # | Story | Expected |
|---|-------|----------|
| C.1.1 | I select "CloudTrail Events" and press `Enter`. | A spinner appears centered in the frame with text like "Fetching CloudTrail events...". The frame title shows `cloudtrail(N) -- i-XXXX (instance-name)` where N appears after loading completes. |
| C.1.2 | The API responds with event data. | The spinner disappears. Events render as table rows. The frame title updates to include the event count, e.g., `cloudtrail(12) -- i-0abc123def456 (web-server-prod)`. |
| C.1.3 | The API responds with an error (e.g., access denied, timeout). | The spinner disappears. A red error flash appears in the header. The frame content shows a centered dim error message. |

### C.2 Empty State

| # | Story | Expected |
|---|-------|----------|
| C.2.1 | No CloudTrail events exist for this instance in the last 24 hours. | The frame title shows `cloudtrail(0) -- i-XXXX (instance-name)`. The content area shows centered dim text: "No CloudTrail events in the last 24 hours". |

### C.3 Column Layout

| # | Story | Expected |
|---|-------|----------|
| C.3.1 | Events load and the table renders. | Four columns appear in this order: "Time" (width 22), "Event" (width 26), "User" (width 24), "Source IP" (width 16). Column headers are bold blue (#7aa2f7) with no separator line below. |
| C.3.2 | I verify column data against `aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceType,AttributeValue=AWS::EC2::Instance`. | "Time" maps to `EventTime`. "Event" maps to `EventName`. "User" maps to `Username`. "Source IP" maps to `SourceIPAddress`. All events returned by the CLI for this instance appear as rows. |
| C.3.3 | An event name is longer than 26 characters. | The name is truncated to fit the 26-character column width. |
| C.3.4 | The terminal is narrower than the combined column widths. | Rightmost columns are hidden. Horizontal scroll with `h`/`l` is available. |

**AWS comparison:**
```
aws cloudtrail lookup-events \
  --lookup-attributes AttributeKey=ResourceType,AttributeValue=AWS::EC2::Instance \
  --lookup-attributes AttributeKey=ResourceName,AttributeValue=i-0abc123def456 \
  --start-time $(date -d '24 hours ago' --iso-8601=seconds) \
  --end-time $(date --iso-8601=seconds)
```
Expected fields visible: Time (EventTime), Event (EventName), User (Username), Source IP (SourceIPAddress)

### C.4 Navigation

| # | Story | Expected |
|---|-------|----------|
| C.4.1 | I press `j`/`k`/`g`/`G`/`PageUp`/`PageDown` in the CloudTrail table. | Standard table navigation: `j` moves cursor down, `k` up, `g` to first row, `G` to last row, `PageUp`/`PageDown` move by one page. |
| C.4.2 | I press `h`/`l` when columns overflow the terminal width. | Columns scroll left/right. Column headers scroll in sync with data columns. |

### C.5 Sorting

| # | Story | Expected |
|---|-------|----------|
| C.5.1 | I press `N` on the CloudTrail table. | Rows are sorted by event name ascending. The "Event" column header shows an up-arrow indicator (e.g., `Event^`). |
| C.5.2 | I press `N` again. | Sort toggles to descending. The indicator changes to a down-arrow. |
| C.5.3 | I press `S` on the CloudTrail table. | Rows are sorted by username (acting as the "status" sort key) ascending. The "User" column header shows the sort indicator. |
| C.5.4 | I press `A` on the CloudTrail table. | Rows are sorted by time (age) ascending (oldest first). The "Time" column header shows the sort indicator. |
| C.5.5 | I press `A` again. | Sort toggles to descending (newest first). |

### C.6 Filter

| # | Story | Expected |
|---|-------|----------|
| C.6.1 | I press `/` and type `Stop`. | The header right shows `/Stop|`. Only events whose name, user, or other visible columns contain "Stop" (case-insensitive) are shown. The frame title updates to `cloudtrail(M/N)`. |
| C.6.2 | I press `Esc` while filter is active. | The filter clears. All events reappear. The frame title reverts to `cloudtrail(N)`. |
| C.6.3 | I type a filter that matches no events. | Zero rows are shown. The frame title shows `cloudtrail(0/N)`. |

### C.7 Detail View (d)

| # | Story | Expected |
|---|-------|----------|
| C.7.1 | I select a CloudTrail event and press `d` (or `Enter`). | The detail view opens. The frame title shows the event name or event ID. |
| C.7.2 | I verify the displayed fields. | The detail view shows key-value pairs for: EventId, EventTime, EventName, EventSource, Username, SourceIPAddress, ReadOnly, AccessKeyId, CloudTrailEvent. These match the `ec2_cloudtrail.detail` configuration from the design doc. |
| C.7.3 | I press `Esc` on the detail view. | I return to the CloudTrail events table. The cursor is on the same event. |
| C.7.4 | I scroll the detail view with `j`/`k`/`g`/`G`. | The viewport scrolls through the detail content. CloudTrailEvent (raw JSON) may be long. |

**AWS comparison:**
```
aws cloudtrail lookup-events --lookup-attributes AttributeKey=EventId,AttributeValue=EVENT_ID
```
Expected detail fields: EventId, EventTime, EventName, EventSource, Username, SourceIPAddress, ReadOnly, AccessKeyId, CloudTrailEvent

### C.8 YAML View (y)

| # | Story | Expected |
|---|-------|----------|
| C.8.1 | I select a CloudTrail event and press `y`. | The YAML view opens showing the raw CloudTrailEvent JSON rendered in the syntax-highlighted YAML viewer. The frame title includes the event name and "yaml". |
| C.8.2 | The YAML content uses standard syntax coloring. | Keys blue (#7aa2f7), string values green (#9ece6a), numbers orange (#ff9e64), booleans purple (#bb9af7), null values dim (#565f89). |
| C.8.3 | I press `Esc` on the YAML view. | I return to the CloudTrail events table. |

### C.9 Copy (c)

| # | Story | Expected |
|---|-------|----------|
| C.9.1 | I select a CloudTrail event and press `c`. | The full `CloudTrailEvent` JSON for the selected row is copied to the system clipboard. A green "Copied!" flash appears in the header. |
| C.9.2 | After approximately 2 seconds. | The "Copied!" flash auto-clears and the header right reverts to `? for help`. |
| C.9.3 | I paste from clipboard. | The pasted text is the raw CloudTrailEvent JSON string. |

### C.10 Data Limits and Pagination

| # | Story | Expected |
|---|-------|----------|
| C.10.1 | The API returns fewer than 1,000 events. | All events are displayed. No footer hint about more data. |
| C.10.2 | The API returns the maximum 1,000 events. | The last visible line shows dim text: "Showing 1000 most recent -- more available". |
| C.10.3 | I scroll to the bottom of the table and more pages are available. | A spinner appears on the last line with "Loading more..." text while the next page is being fetched. |
| C.10.4 | The next page loads successfully. | New rows are appended to the table. The event count in the frame title increases. The spinner disappears. |
| C.10.5 | Lazy pagination continues until the 1,000 event limit is reached. | Once 1,000 events are loaded, no further pagination occurs. The dim footer hint appears. |
| C.10.6 | The default time range is 24 hours. | Only events from the last 24 hours are shown. Events older than 24 hours do not appear. |

### C.11 Refresh

| # | Story | Expected |
|---|-------|----------|
| C.11.1 | I press `Ctrl+R` on the CloudTrail table. | The spinner appears. A fresh API call is made. When it completes, the table repopulates with current data. Pagination state resets to the first page. |

### C.12 Escape

| # | Story | Expected |
|---|-------|----------|
| C.12.1 | I press `Esc` on the CloudTrail events table. | I return to the log source menu. The cursor is on "CloudTrail Events". |

### C.13 Help

| # | Story | Expected |
|---|-------|----------|
| C.13.1 | I press `?` on the CloudTrail table. | The help screen appears with a 4-column layout: NAVIGATION, GENERAL, ACTIONS, SORT. |
| C.13.2 | NAVIGATION lists: `<j>` Down, `<k>` Up, `<g>` Top, `<G>` Bottom, `<h/l>` Cols, `<pgup>` Page up, `<pgdn>` Page down. | All seven navigation keys are listed. |
| C.13.3 | GENERAL lists: `<ctrl-r>` Refresh, `<?>` Help, `<:>` Command, `</>` Filter, `<q>` Quit. | All five general keys are listed. |
| C.13.4 | ACTIONS lists: `<enter>` Detail, `<d>` Detail, `<y>` YAML, `<c>` Copy, `<esc>` Back. | All five action keys are listed. |
| C.13.5 | SORT lists: `<N>` By name, `<S>` By status, `<A>` By age. | All three sort keys are listed. |
| C.13.6 | I press any key on the help screen. | The help screen closes and the CloudTrail table reappears. |

### C.14 Row Coloring

| # | Story | Expected |
|---|-------|----------|
| C.14.1 | CloudTrail events are displayed. | Rows are rendered in plain text color (#c0caf5) since CloudTrail events do not have a status-based color mapping. |
| C.14.2 | The selected row. | Full-width blue background (#7aa2f7), dark foreground (#1a1b26), bold text. |
| C.14.3 | Alternating rows. | Alternating rows have a subtle background color difference (#1e2030) for readability. |

---

## D. System Console Output (Viewport Sub-View)

### D.1 Loading State

| # | Story | Expected |
|---|-------|----------|
| D.1.1 | I select "System Console Output" and press `Enter`. | A spinner appears centered in the frame with text like "Fetching console output...". The frame title shows `console-output -- i-XXXX (instance-name)`. |
| D.1.2 | The API responds with console output. | The spinner disappears. The decoded console text renders as a scrollable viewport. Lines show kernel messages, boot logs, and cloud-init output. |
| D.1.3 | The API responds with an error. | The spinner disappears. A red error flash appears in the header. The frame content shows a centered dim error message. |

**AWS comparison:**
```
aws ec2 get-console-output --instance-id i-0abc123def456 --latest
```
Expected: base64-decoded Output field rendered as scrollable text

### D.2 Empty State

| # | Story | Expected |
|---|-------|----------|
| D.2.1 | The API returns empty or null console output. | The frame content shows centered dim text: "No console output available" with a second line: "Instance may not have produced console output yet". |
| D.2.2 | The instance was just launched and has no output yet. | Same empty state as D.2.1. |

### D.3 Navigation

| # | Story | Expected |
|---|-------|----------|
| D.3.1 | I press `j`/`k` in the console output viewport. | The viewport scrolls down/up by one line. |
| D.3.2 | I press `g`. | The viewport jumps to the top of the console output. |
| D.3.3 | I press `G`. | The viewport jumps to the bottom of the console output. |
| D.3.4 | I press `PageUp`/`PageDown`. | The viewport scrolls by one page of visible lines. |
| D.3.5 | The console output is shorter than the visible area. | No scrolling occurs. All content is visible without scroll indicators. |
| D.3.6 | The console output is longer than the visible area. | Scroll indicators appear (e.g., "X lines above" / "X lines below") in dim text. |

### D.4 Filter

| # | Story | Expected |
|---|-------|----------|
| D.4.1 | I press `/` and type `error`. | The header right shows `/error|` in amber. Only lines containing "error" (case-insensitive) are visible. Non-matching lines are hidden. |
| D.4.2 | The frame title does NOT change when a filter is active. | Unlike table views, the viewport frame title shows no matched/total count. The title remains `console-output -- i-XXXX (instance-name)`. |
| D.4.3 | I press `Esc` while filter is active. | The filter clears. All lines reappear. |
| D.4.4 | I type a filter that matches no lines. | No lines are visible. The viewport is empty. |

### D.5 Word Wrap

| # | Story | Expected |
|---|-------|----------|
| D.5.1 | Long lines extend beyond the terminal width (word wrap off by default). | Lines are truncated at the terminal width. |
| D.5.2 | I press `w`. | Word wrap is toggled on. Long lines now wrap to the next line. The total number of visible lines may increase. |
| D.5.3 | I press `w` again. | Word wrap is toggled off. Lines revert to truncated display. |

### D.6 Copy (c)

| # | Story | Expected |
|---|-------|----------|
| D.6.1 | I press `c` in the console output viewport. | The entire console output text is copied to the system clipboard. A green "Copied!" flash appears in the header. |
| D.6.2 | I paste from clipboard. | The pasted text matches the full decoded console output. |

### D.7 Detail and YAML Not Applicable

| # | Story | Expected |
|---|-------|----------|
| D.7.1 | I press `d` in the console output viewport. | Nothing happens. Detail view is not applicable for raw text. |
| D.7.2 | I press `y` in the console output viewport. | Nothing happens. YAML view is not applicable for raw text. |

### D.8 Refresh

| # | Story | Expected |
|---|-------|----------|
| D.8.1 | I press `Ctrl+R` in the console output viewport. | The spinner appears. A fresh `GetConsoleOutput` call is made. The content updates with the latest output. |

### D.9 Escape

| # | Story | Expected |
|---|-------|----------|
| D.9.1 | I press `Esc` on the console output viewport. | I return to the log source menu. |

### D.10 Help

| # | Story | Expected |
|---|-------|----------|
| D.10.1 | I press `?` on the console output viewport. | The help screen appears with a 3-column layout: NAVIGATION, GENERAL, ACTIONS. |
| D.10.2 | NAVIGATION lists: `<j>` Down, `<k>` Up, `<g>` Top, `<G>` Bottom, `<pgup>` Page up, `<pgdn>` Page down. | All six navigation keys are listed. |
| D.10.3 | GENERAL lists: `<ctrl-r>` Refresh, `<?>` Help, `</>` Filter, `<:>` Command, `<q>` Quit. | All five general keys are listed. |
| D.10.4 | ACTIONS lists: `<c>` Copy, `<w>` Word wrap, `<esc>` Back. | All three action keys are listed. |
| D.10.5 | I press any key on the help screen. | The help screen closes and the console output reappears. |

---

## E. CloudWatch Logs (Two Sub-Levels)

### E.1 CW Log Group List -- Loading State

| # | Story | Expected |
|---|-------|----------|
| E.1.1 | I select "CloudWatch Logs" (showing "3 log groups") and press `Enter`. | The CW log group list view opens. The frame title reads `cw-log-groups(3) -- i-XXXX (instance-name)`. Three log groups are listed as table rows. |
| E.1.2 | The discovery process takes time (scanning log groups). | A spinner appears centered in the frame with text like "Discovering log groups...". |
| E.1.3 | Discovery completes. | The spinner disappears. Log groups matching this instance are listed. |
| E.1.4 | Discovery hits the 200 log group scan limit. | A dim hint appears: "Scanned 200 of N groups -- some may be missing" where N is the total number of log groups in the account. |
| E.1.5 | Discovery hits the 30-second timeout. | Same dim hint as E.1.4, indicating the scan was truncated. |

**AWS comparison:**
```
aws logs describe-log-groups
aws logs describe-log-streams --log-group-name /ec2/... --log-stream-name-prefix i-0abc123def456
```
Expected fields visible: Log Group (LogGroupName), Streams (stream_count), Last Event (last_event_time)

### E.2 CW Log Group List -- Empty State

| # | Story | Expected |
|---|-------|----------|
| E.2.1 | No log groups are found for this instance. | The frame title shows `cw-log-groups(0) -- i-XXXX (instance-name)`. The content area shows centered dim text: "No CloudWatch log groups found for this instance" with a second line: "CloudWatch agent may not be installed". |

### E.3 CW Log Group List -- Column Layout

| # | Story | Expected |
|---|-------|----------|
| E.3.1 | Log groups are loaded. | Three columns appear: "Log Group" (width 60), "Streams" (width 10), "Last Event" (width 22). Column headers are bold blue (#7aa2f7). |
| E.3.2 | I verify column data. | "Log Group" maps to `LogGroupName`. "Streams" maps to `stream_count` (number of streams matching this instance). "Last Event" maps to `last_event_time` (timestamp of the most recent event). |
| E.3.3 | A log group name is longer than 60 characters. | The name is truncated to fit the column width. |

### E.4 CW Log Group List -- Navigation

| # | Story | Expected |
|---|-------|----------|
| E.4.1 | I press `j`/`k`/`g`/`G`/`PageUp`/`PageDown`. | Standard table navigation within the log group list. |
| E.4.2 | I press `h`/`l` when columns overflow. | Horizontal column scrolling. |
| E.4.3 | I select a log group and press `Enter`. | The CW Log Viewer opens for that log group. A spinner appears while log events are fetched. This is the 4th level of the view stack (EC2 List -> Log Sources -> CW Log Groups -> Log Viewer). |

### E.5 CW Log Group List -- Filter

| # | Story | Expected |
|---|-------|----------|
| E.5.1 | I press `/` and type `syslog`. | Only log groups whose name contains "syslog" are shown. The frame title updates to `cw-log-groups(M/N)`. |
| E.5.2 | I press `Esc` while filter is active. | The filter clears. All log groups reappear. |

### E.6 CW Log Group List -- Copy

| # | Story | Expected |
|---|-------|----------|
| E.6.1 | I select a log group and press `c`. | The log group name is copied to the clipboard. A green "Copied!" flash appears in the header. |

### E.7 CW Log Group List -- Detail/YAML Not Applicable

| # | Story | Expected |
|---|-------|----------|
| E.7.1 | I press `d` on a selected log group. | Nothing happens. No detail view is defined for CW log groups (Enter opens the log viewer instead). |
| E.7.2 | I press `y` on a selected log group. | Nothing happens. No YAML view is defined for CW log groups. |

### E.8 CW Log Group List -- Refresh

| # | Story | Expected |
|---|-------|----------|
| E.8.1 | I press `Ctrl+R` on the CW log group list. | The spinner appears. Discovery re-runs (re-scans log groups for this instance). The session cache is cleared. |

### E.9 CW Log Group List -- Escape

| # | Story | Expected |
|---|-------|----------|
| E.9.1 | I press `Esc` on the CW log group list. | I return to the log source menu. The cursor is on "CloudWatch Logs". |

### E.10 CW Log Viewer (Viewport) -- Loading State

| # | Story | Expected |
|---|-------|----------|
| E.10.1 | I select a log group and press `Enter`. | The log viewer opens. A spinner appears while events are fetched. The frame title reads the log group name followed by the instance ID, e.g., `/ec2/web-server/var/log/syslog -- i-0abc123def456`. |
| E.10.2 | The API responds with log events. | The spinner disappears. Each log line renders as: timestamp (dim) followed by message text. |
| E.10.3 | The API responds with an error. | The spinner disappears. A red error flash appears in the header. |

**AWS comparison:**
```
aws logs filter-log-events \
  --log-group-name /ec2/web-server/var/log/syslog \
  --log-stream-names i-0abc123def456 \
  --start-time $(date -d '24 hours ago' +%s000) \
  --limit 100
```
Expected: Each event's timestamp (dim) + message text

### E.11 CW Log Viewer -- Empty State

| # | Story | Expected |
|---|-------|----------|
| E.11.1 | No log events exist in this group for the last 24 hours. | The content area shows centered dim text with a source-specific message (e.g., "No log events in the last 24 hours"). |

### E.12 CW Log Viewer -- Timestamp Display

| # | Story | Expected |
|---|-------|----------|
| E.12.1 | Log events are displayed. | Each line starts with a dim (#565f89) timestamp followed by the message text in normal color (#c0caf5). |
| E.12.2 | Timestamps are formatted consistently. | All timestamps use the same datetime format (e.g., `2024-01-15 14:23:05`). |

### E.13 CW Log Viewer -- Navigation

| # | Story | Expected |
|---|-------|----------|
| E.13.1 | I press `j`/`k` in the log viewer. | The viewport scrolls down/up by one line. |
| E.13.2 | I press `g`/`G`. | The viewport jumps to the top/bottom. |
| E.13.3 | I press `PageUp`/`PageDown`. | The viewport scrolls by one page. |

### E.14 CW Log Viewer -- Filter

| # | Story | Expected |
|---|-------|----------|
| E.14.1 | I press `/` and type `ERROR`. | The header right shows `/ERROR|` in amber. Only log lines containing "ERROR" (case-insensitive) are visible. |
| E.14.2 | The frame title does NOT change. | No matched/total count appears in the frame title (this is a viewport, not a table). |
| E.14.3 | I press `Esc` while filter is active. | The filter clears. All log lines reappear. |

### E.15 CW Log Viewer -- Word Wrap

| # | Story | Expected |
|---|-------|----------|
| E.15.1 | Long log lines extend beyond terminal width (default: no wrap). | Lines are truncated at the terminal width. |
| E.15.2 | I press `w`. | Word wrap toggles on. Long lines wrap to the next display line. |
| E.15.3 | I press `w` again. | Word wrap toggles off. Lines are truncated again. |

### E.16 CW Log Viewer -- Copy

| # | Story | Expected |
|---|-------|----------|
| E.16.1 | I press `c` in the log viewer. | The current line at the cursor position (the line at the top of the viewport, or a highlighted line) is copied to the clipboard. A green "Copied!" flash appears. |
| E.16.2 | I paste from clipboard. | The pasted text is the single log line (timestamp + message). |

### E.17 CW Log Viewer -- Data Limits and Pagination

| # | Story | Expected |
|---|-------|----------|
| E.17.1 | The first page of events (100) loads. | Only the first 100 events are shown initially. |
| E.17.2 | I scroll to the bottom of the viewport and more pages are available. | A spinner appears on the last line with "Loading more..." text while the next page is fetched. |
| E.17.3 | The next page loads. | New lines are appended to the viewport. The spinner disappears. |
| E.17.4 | The 1,000 event limit is reached. | The last line shows dim text: "Showing 1000 most recent -- more available". No further pagination occurs. |
| E.17.5 | The default time range is 24 hours. | Only events from the last 24 hours are fetched. |

### E.18 CW Log Viewer -- Refresh

| # | Story | Expected |
|---|-------|----------|
| E.18.1 | I press `Ctrl+R` in the log viewer. | The spinner appears. A fresh API call fetches events. Pagination state resets. |

### E.19 CW Log Viewer -- Escape

| # | Story | Expected |
|---|-------|----------|
| E.19.1 | I press `Esc` on the CW log viewer. | I return to the CW log group list. The cursor is on the log group I had selected. |

### E.20 CW Log Viewer -- Help

| # | Story | Expected |
|---|-------|----------|
| E.20.1 | I press `?` on the CW log viewer. | The help screen appears with a 3-column layout: NAVIGATION, GENERAL, ACTIONS. Same layout as the console output help (viewport sub-view help). |
| E.20.2 | ACTIONS lists: `<c>` Copy, `<w>` Word wrap, `<esc>` Back. | All three action keys are listed. |
| E.20.3 | I press any key on the help screen. | The help screen closes and the log viewer reappears. |

---

## F. Instance Status Events (Viewport Sub-View)

### F.1 Loading State

| # | Story | Expected |
|---|-------|----------|
| F.1.1 | I select "Instance Status Events" and press `Enter`. | A spinner appears while `DescribeInstanceStatus` is in flight. The frame title shows `instance-status -- i-XXXX (instance-name)`. |
| F.1.2 | The API responds with status data. | The spinner disappears. Status information renders as a detail-style view (key: value pairs). |
| F.1.3 | The API responds with an error. | The spinner disappears. A red error flash appears in the header. |

**AWS comparison:**
```
aws ec2 describe-instance-status --instance-ids i-0abc123def456 --include-all-instances
```
Expected fields: InstanceState.Name, SystemStatus.Status, SystemStatus.Details[].Name/Status,
InstanceStatus.Status, InstanceStatus.Details[].Name/Status, Events[].Code/Description/NotBefore/NotAfter

### F.2 Healthy Instance

| # | Story | Expected |
|---|-------|----------|
| F.2.1 | Instance is running with all checks passing. | The view shows: "Instance State: running" (green), "System Status: ok" (green) with "Reachability: passed" (green), "Instance Status: ok" (green) with "Reachability: passed" (green), "Scheduled Events: None" (dim). |
| F.2.2 | Status values `running`, `ok`, `passed` are colored green (#9ece6a). | Visual inspection confirms green status values. |
| F.2.3 | "None" for scheduled events is colored dim (#565f89). | Visual inspection confirms dim text. |

### F.3 Impaired Instance

| # | Story | Expected |
|---|-------|----------|
| F.3.1 | System status is `impaired` with reachability `failed`. | "System Status: impaired" is red (#f7768e). "Reachability: failed" is red (#f7768e). An "Impaired Since" timestamp is shown. |
| F.3.2 | Instance status is `ok` while system status is `impaired`. | Instance Status section shows green values, System Status section shows red values. Both are visible simultaneously. |

### F.4 Scheduled Events

| # | Story | Expected |
|---|-------|----------|
| F.4.1 | The instance has a scheduled reboot event. | The "Scheduled Events" section shows the event code (e.g., `instance-reboot`) in yellow (#e0af68), plus Description, Not Before, and Not After timestamps. |
| F.4.2 | The instance has multiple scheduled events. | Each event is listed with its own code, description, and time window. |

### F.5 Section Headers

| # | Story | Expected |
|---|-------|----------|
| F.5.1 | The view contains section headers: "System Status:", "Instance Status:", "Scheduled Events:". | Section headers are rendered in yellow/orange (#e0af68), bold, matching the detail view section header style from the design spec. |
| F.5.2 | Sub-fields (Status, Reachability, Impaired Since) are indented. | Sub-fields appear with 5-space indent under their parent section header. |

### F.6 Navigation

| # | Story | Expected |
|---|-------|----------|
| F.6.1 | I press `j`/`k` in the status view. | The viewport scrolls up/down by one line. |
| F.6.2 | I press `g`/`G`. | The viewport jumps to the top/bottom. |
| F.6.3 | The content is shorter than the visible area. | No scrolling occurs. All content is visible. |

### F.7 Copy (c)

| # | Story | Expected |
|---|-------|----------|
| F.7.1 | I press `c` in the instance status view. | The full status summary (all text) is copied to the clipboard. A green "Copied!" flash appears. |

### F.8 Detail and YAML Not Applicable

| # | Story | Expected |
|---|-------|----------|
| F.8.1 | I press `d` in the instance status view. | Nothing happens. This is already a detail-style view. |
| F.8.2 | I press `y` in the instance status view. | Nothing happens. This is already a detail-style view. |

### F.9 Refresh

| # | Story | Expected |
|---|-------|----------|
| F.9.1 | I press `Ctrl+R` in the instance status view. | The spinner appears. A fresh `DescribeInstanceStatus` call is made. The content updates with current status. |
| F.9.2 | A scheduled event was added since the last fetch. I press `Ctrl+R`. | The new event appears in the "Scheduled Events" section after refresh. |

### F.10 Escape

| # | Story | Expected |
|---|-------|----------|
| F.10.1 | I press `Esc` on the instance status view. | I return to the log source menu. The cursor is on "Instance Status Events". |

### F.11 Help

| # | Story | Expected |
|---|-------|----------|
| F.11.1 | I press `?` on the instance status view. | The help screen appears. Since this is a viewport-style view, the layout matches the viewport sub-view help: NAVIGATION, GENERAL, ACTIONS columns. |
| F.11.2 | ACTIONS lists: `<c>` Copy, `<esc>` Back. | The `<w>` word wrap key may or may not be listed (this is a detail-style viewport where wrap may not apply). |

---

## G. VPC Flow Logs (Table Sub-View)

### G.1 Loading State

| # | Story | Expected |
|---|-------|----------|
| G.1.1 | I select "VPC Flow Logs" and press `Enter`. | A spinner appears while flow log records are being fetched. The frame title shows `flow-logs(N) -- i-XXXX (instance-name)` where N appears after loading. |
| G.1.2 | The API responds with flow records. | The spinner disappears. Flow records render as table rows. The frame title updates with the record count. |
| G.1.3 | The API responds with an error. | The spinner disappears. A red error flash appears in the header. |

**AWS comparison:**
```
aws ec2 describe-network-interfaces --filters Name=attachment.instance-id,Values=i-0abc123def456
aws ec2 describe-flow-logs --filter Name=resource-id,Values=SUBNET_OR_VPC_ID
aws logs filter-log-events --log-group-name FLOW_LOG_GROUP --start-time ...
```
Expected fields visible: Time (timestamp), ENI (eni_id), Src (src_addr), Dst (dst_addr), Port (dst_port), Proto (protocol), Action (action)

### G.2 Empty State

| # | Story | Expected |
|---|-------|----------|
| G.2.1 | No flow log records exist in the last 24 hours. | The frame title shows `flow-logs(0) -- i-XXXX (instance-name)`. The content area shows centered dim text: "No flow log records in the last 24 hours". |

### G.3 Column Layout

| # | Story | Expected |
|---|-------|----------|
| G.3.1 | Flow log records are loaded. | Seven columns appear: "Time" (width 22), "ENI" (width 14), "Src" (width 16), "Dst" (width 16), "Port" (width 7), "Proto" (width 6), "Action" (width 8). Column headers are bold blue. |
| G.3.2 | I verify column data. | "Time" maps to `timestamp`. "ENI" maps to `eni_id`. "Src" maps to `src_addr`. "Dst" maps to `dst_addr`. "Port" maps to `dst_port`. "Proto" maps to `protocol`. "Action" maps to `action`. |
| G.3.3 | Multiple ENIs exist for the instance. | Records from all ENIs are merged into a single table. The ENI column shows which interface each record belongs to. |

### G.4 Row Coloring

| # | Story | Expected |
|---|-------|----------|
| G.4.1 | A flow record with Action `ACCEPT`. | The entire row is rendered in green (#9ece6a), following the standard status color mapping (ACCEPT = running/available = green). |
| G.4.2 | A flow record with Action `REJECT`. | The entire row is rendered in red (#f7768e), following the standard status color mapping (REJECT = stopped/failed = red). |
| G.4.3 | The selected row. | Full-width blue background (#7aa2f7), dark foreground (#1a1b26), bold text -- overrides ACCEPT/REJECT coloring. |
| G.4.4 | Alternating rows. | Alternating rows have a subtle background difference (#1e2030), combined with the ACCEPT/REJECT foreground colors. |

### G.5 Navigation

| # | Story | Expected |
|---|-------|----------|
| G.5.1 | I press `j`/`k`/`g`/`G`/`PageUp`/`PageDown`. | Standard table navigation. |
| G.5.2 | I press `h`/`l` when columns overflow. | Horizontal column scrolling. The Action column (rightmost) may be hidden on narrow terminals. |

### G.6 Sorting

| # | Story | Expected |
|---|-------|----------|
| G.6.1 | I press `N` on the flow logs table. | Rows are sorted by source address (or ENI -- the "name" column). Sort indicator appears. |
| G.6.2 | I press `S`. | Rows are sorted by action (ACCEPT/REJECT). |
| G.6.3 | I press `A`. | Rows are sorted by timestamp (age). |

### G.7 Filter

| # | Story | Expected |
|---|-------|----------|
| G.7.1 | I press `/` and type `REJECT`. | Only flow records with "REJECT" in any visible column are shown. The frame title updates to `flow-logs(M/N)`. |
| G.7.2 | I press `/` and type a specific IP address. | Only flow records involving that IP are shown. |
| G.7.3 | I press `Esc` while filter is active. | The filter clears. All records reappear. |

### G.8 Detail (d)

| # | Story | Expected |
|---|-------|----------|
| G.8.1 | I select a flow record and press `d` (or `Enter`). | The detail view opens for the selected flow record, showing all available fields for that record. |
| G.8.2 | I press `Esc` on the detail view. | I return to the flow logs table. |

### G.9 Copy (c)

| # | Story | Expected |
|---|-------|----------|
| G.9.1 | I select a flow record and press `c`. | The selected flow record is copied as a tab-separated line to the clipboard. A green "Copied!" flash appears. |
| G.9.2 | I paste from clipboard. | The pasted text is a tab-separated line with timestamp, ENI, source, destination, port, protocol, and action. |

### G.10 Data Limits and Pagination

| # | Story | Expected |
|---|-------|----------|
| G.10.1 | Fewer than 1,000 records are returned. | All records are displayed. No footer hint. |
| G.10.2 | Exactly 1,000 records are loaded. | The last line shows dim text: "Showing 1000 most recent -- more available". |
| G.10.3 | I scroll to the bottom with more pages available. | A spinner appears with "Loading more..." on the last line. |
| G.10.4 | The default time range is 24 hours. | Only flow records from the last 24 hours are shown. |

### G.11 Refresh

| # | Story | Expected |
|---|-------|----------|
| G.11.1 | I press `Ctrl+R` on the flow logs table. | The spinner appears. A fresh fetch is made. Pagination resets. |

### G.12 Escape

| # | Story | Expected |
|---|-------|----------|
| G.12.1 | I press `Esc` on the flow logs table. | I return to the log source menu. The cursor is on "VPC Flow Logs". |

### G.13 Help

| # | Story | Expected |
|---|-------|----------|
| G.13.1 | I press `?` on the flow logs table. | The help screen appears with a 4-column layout: NAVIGATION, GENERAL, ACTIONS, SORT. Same as CloudTrail table help. |
| G.13.2 | I press any key on the help screen. | The help screen closes and the flow logs table reappears. |

### G.14 Responsive Column Display

| # | Story | Expected |
|---|-------|----------|
| G.14.1 | Terminal width is 60-79 columns. | Only Time, Src, Dst columns are visible. |
| G.14.2 | Terminal width is 80-119 columns. | Time, Src, Dst, Port columns are visible. |
| G.14.3 | Terminal width is 120+ columns. | All 7 columns are visible. |

---

## H. SSM Command History (Table + Output Sub-Views)

### H.1 Loading State

| # | Story | Expected |
|---|-------|----------|
| H.1.1 | I select "SSM Command History" and press `Enter`. | A spinner appears while commands are being fetched. The frame title shows `ssm-commands(N) -- i-XXXX (instance-name)` where N appears after loading. |
| H.1.2 | The API responds with command data. | The spinner disappears. Commands render as table rows. |
| H.1.3 | The API responds with an error. | The spinner disappears. A red error flash appears in the header. |

**AWS comparison:**
```
aws ssm list-commands --instance-id i-0abc123def456
```
Expected fields visible: Time (RequestedDateTime), Command (DocumentName), Status (StatusDetails), User (requested_by)

### H.2 Empty State

| # | Story | Expected |
|---|-------|----------|
| H.2.1 | No SSM commands were executed on this instance. | The frame title shows `ssm-commands(0) -- i-XXXX (instance-name)`. The content area shows centered dim text: "No SSM commands found for this instance". |

### H.3 Column Layout

| # | Story | Expected |
|---|-------|----------|
| H.3.1 | Commands are loaded. | Four columns appear: "Time" (width 22), "Command" (width 32), "Status" (width 12), "User" (width 20). Column headers are bold blue. |
| H.3.2 | I verify column data. | "Time" maps to `RequestedDateTime`. "Command" maps to `DocumentName`. "Status" maps to `StatusDetails`. "User" maps to `requested_by`. |
| H.3.3 | A document name is longer than 32 characters. | The name is truncated to fit the column width. |

### H.4 Row Coloring

| # | Story | Expected |
|---|-------|----------|
| H.4.1 | A command with StatusDetails `Success`. | The entire row is green (#9ece6a). |
| H.4.2 | A command with StatusDetails `Failed`. | The entire row is red (#f7768e). |
| H.4.3 | A command with StatusDetails `InProgress`. | The entire row is yellow (#e0af68). |
| H.4.4 | A command with StatusDetails `TimedOut`. | The entire row is red (#f7768e). |
| H.4.5 | A command with StatusDetails `Cancelled`. | The entire row is dim (#565f89). |
| H.4.6 | The selected row. | Full-width blue background, dark foreground, bold -- overrides status coloring. |

### H.5 Navigation

| # | Story | Expected |
|---|-------|----------|
| H.5.1 | I press `j`/`k`/`g`/`G`/`PageUp`/`PageDown`. | Standard table navigation. |
| H.5.2 | I press `h`/`l` when columns overflow. | Horizontal column scrolling. |

### H.6 Sorting

| # | Story | Expected |
|---|-------|----------|
| H.6.1 | I press `N`. | Rows are sorted by command name (DocumentName) ascending. Sort indicator appears on "Command" header. |
| H.6.2 | I press `S`. | Rows are sorted by status ascending. Sort indicator appears on "Status" header. |
| H.6.3 | I press `A`. | Rows are sorted by time (age) ascending. Sort indicator appears on "Time" header. |

### H.7 Filter

| # | Story | Expected |
|---|-------|----------|
| H.7.1 | I press `/` and type `RunShell`. | Only commands whose document name contains "RunShell" (case-insensitive) are shown. Frame title updates to `ssm-commands(M/N)`. |
| H.7.2 | I press `/` and type `Failed`. | Only commands with "Failed" in any visible column are shown. |
| H.7.3 | I press `Esc` while filter is active. | The filter clears. All commands reappear. |

### H.8 Enter Key (Drill Into Command Output)

| # | Story | Expected |
|---|-------|----------|
| H.8.1 | I select a command and press `Enter`. | The SSM command output viewport opens. A spinner appears while `GetCommandInvocation` is called. The frame title shows `ssm-output -- DOCUMENT_NAME (TIMESTAMP)` with the document name and requested datetime. |
| H.8.2 | The API returns stdout and stderr. | The viewport shows "STDOUT:" (yellow, bold section header) followed by stdout content, then "STDERR:" (yellow, bold section header) followed by stderr content. |
| H.8.3 | Stderr is empty. | The "STDERR:" section shows "(empty)" in dim text. |
| H.8.4 | Both stdout and stderr are empty. | Both sections show "(empty)" in dim text. |

**AWS comparison:**
```
aws ssm get-command-invocation \
  --command-id COMMAND_ID \
  --instance-id i-0abc123def456
```
Expected: StandardOutputContent and StandardErrorContent fields

### H.9 Detail (d)

| # | Story | Expected |
|---|-------|----------|
| H.9.1 | I select a command and press `d`. | The detail view opens showing: CommandId, DocumentName, RequestedDateTime, StatusDetails, Comment, OutputS3BucketName, OutputS3KeyPrefix. These match the `ec2_ssm_commands.detail` configuration. |
| H.9.2 | I press `Esc` on the detail view. | I return to the SSM commands table. |

### H.10 YAML (y)

| # | Story | Expected |
|---|-------|----------|
| H.10.1 | I select a command and press `y`. | The YAML view opens showing the full command metadata as syntax-highlighted YAML. |
| H.10.2 | I press `Esc` on the YAML view. | I return to the SSM commands table. |

### H.11 Copy (c)

| # | Story | Expected |
|---|-------|----------|
| H.11.1 | I select a command in the SSM table and press `c`. | The command ID or relevant identifier is copied to the clipboard. A green "Copied!" flash appears. |

### H.12 Data Limits

| # | Story | Expected |
|---|-------|----------|
| H.12.1 | The API returns fewer than 1,000 commands. | All commands are displayed. No footer hint. |
| H.12.2 | The 1,000 command limit is reached. | The last line shows dim text: "Showing 1000 most recent -- more available". |
| H.12.3 | I scroll to the bottom with more pages available. | A spinner appears with "Loading more...". |

### H.13 SSM Command Output -- Navigation

| # | Story | Expected |
|---|-------|----------|
| H.13.1 | I press `j`/`k` in the command output viewport. | The viewport scrolls down/up by one line. |
| H.13.2 | I press `g`/`G`. | The viewport jumps to the top/bottom. |
| H.13.3 | I press `PageUp`/`PageDown`. | The viewport scrolls by one page. |

### H.14 SSM Command Output -- Word Wrap

| # | Story | Expected |
|---|-------|----------|
| H.14.1 | Long output lines extend beyond terminal width (default: no wrap). | Lines are truncated. |
| H.14.2 | I press `w`. | Word wrap toggles on. Long lines wrap. |

### H.15 SSM Command Output -- Copy

| # | Story | Expected |
|---|-------|----------|
| H.15.1 | I press `c` in the SSM command output viewport. | The full command output (stdout + stderr) is copied to the clipboard. A green "Copied!" flash appears. |

### H.16 SSM Command Output -- Filter

| # | Story | Expected |
|---|-------|----------|
| H.16.1 | I press `/` and type `config`. | Only lines containing "config" (case-insensitive) are visible. The frame title does NOT change (viewport, not table). |
| H.16.2 | I press `Esc` while filter is active. | The filter clears. All lines reappear. |

### H.17 SSM Command Output -- Escape

| # | Story | Expected |
|---|-------|----------|
| H.17.1 | I press `Esc` on the SSM command output viewport. | I return to the SSM commands table. The cursor is on the same command. |

### H.18 SSM Command Output -- Help

| # | Story | Expected |
|---|-------|----------|
| H.18.1 | I press `?` on the SSM command output viewport. | The help screen appears with viewport sub-view help layout: NAVIGATION, GENERAL, ACTIONS. |
| H.18.2 | ACTIONS lists: `<c>` Copy, `<w>` Word wrap, `<esc>` Back. | All three action keys are listed. |

### H.19 Refresh

| # | Story | Expected |
|---|-------|----------|
| H.19.1 | I press `Ctrl+R` on the SSM commands table. | The spinner appears. A fresh API call fetches commands. Pagination resets. |
| H.19.2 | I press `Ctrl+R` on the SSM command output viewport. | The spinner appears. A fresh `GetCommandInvocation` call is made. |

### H.20 Escape from SSM Commands Table

| # | Story | Expected |
|---|-------|----------|
| H.20.1 | I press `Esc` on the SSM commands table. | I return to the log source menu. The cursor is on "SSM Command History". |

### H.21 Help for SSM Commands Table

| # | Story | Expected |
|---|-------|----------|
| H.21.1 | I press `?` on the SSM commands table. | The help screen appears with a 4-column layout: NAVIGATION, GENERAL, ACTIONS, SORT. Same as other table sub-view help screens. |

### H.22 Responsive Column Display

| # | Story | Expected |
|---|-------|----------|
| H.22.1 | Terminal width is 60-79 columns. | Only Time, Command columns are visible. |
| H.22.2 | Terminal width is 80-119 columns. | Time, Command, Status columns are visible. |
| H.22.3 | Terminal width is 120+ columns. | All 4 columns are visible. |

---

## I. View Stack Depth

### I.1 Full Navigation Paths

| # | Story | Expected |
|---|-------|----------|
| I.1.1 | EC2 List -> Log Sources (2 levels). I press `Esc`. | I return to EC2 List. |
| I.1.2 | EC2 List -> Log Sources -> CloudTrail (3 levels). I press `Esc`. | I return to Log Sources. I press `Esc` again. I return to EC2 List. |
| I.1.3 | EC2 List -> Log Sources -> Console Output (3 levels). I press `Esc`. | I return to Log Sources. |
| I.1.4 | EC2 List -> Log Sources -> Instance Status (3 levels). I press `Esc`. | I return to Log Sources. |
| I.1.5 | EC2 List -> Log Sources -> CW Log Groups -> Log Viewer (4 levels). I press `Esc`. | I return to CW Log Groups. I press `Esc`. I return to Log Sources. I press `Esc`. I return to EC2 List. |
| I.1.6 | EC2 List -> Log Sources -> Flow Logs (3 levels). I press `Esc`. | I return to Log Sources. |
| I.1.7 | EC2 List -> Log Sources -> SSM Commands -> SSM Output (4 levels). I press `Esc`. | I return to SSM Commands. I press `Esc`. I return to Log Sources. I press `Esc`. I return to EC2 List. |
| I.1.8 | EC2 List -> Log Sources -> CloudTrail -> Detail (4 levels). I press `Esc`. | I return to CloudTrail events. I press `Esc`. I return to Log Sources. |
| I.1.9 | EC2 List -> Log Sources -> CloudTrail -> YAML (4 levels). I press `Esc`. | I return to CloudTrail events. |
| I.1.10 | EC2 List -> Log Sources -> SSM Commands -> Detail (4 levels). I press `Esc`. | I return to SSM Commands. |
| I.1.11 | EC2 List -> Log Sources -> Flow Logs -> Detail (4 levels). I press `Esc`. | I return to Flow Logs. |

### I.2 State Preservation Across Stack

| # | Story | Expected |
|---|-------|----------|
| I.2.1 | I navigate into CloudTrail, scroll to row 15, press `Esc`, then re-enter CloudTrail. | The cursor position may or may not be preserved on re-entry (depends on whether the view is re-fetched). The data is re-fetched from API. |
| I.2.2 | I apply a filter on the EC2 list, press `l` to open logs, then `Esc` back. | The EC2 list filter is still active after returning. |
| I.2.3 | I sort the EC2 list by name, press `l` to open logs, navigate into a sub-view, then `Esc` back to EC2 list. | The sort order on the EC2 list is preserved. |

---

## J. Cross-Cutting Concerns

### J.1 Header Consistency

| # | Story | Expected |
|---|-------|----------|
| J.1.1 | In every log view (source menu, CloudTrail, console output, CW groups, CW log viewer, instance status, flow logs, SSM commands, SSM output), the header displays: "a9s" (accent bold), version (dim), profile:region (bold). | Visual inspection confirms across all log views. |
| J.1.2 | The header right shows "? for help" in normal mode across all log views. | Confirmed in all log views. |
| J.1.3 | Flash messages ("Copied!", errors) appear in the header right and auto-clear after approximately 2 seconds across all log views. | Confirmed in all views that support `c` or can produce errors. |

### J.2 Terminal Resize

| # | Story | Expected |
|---|-------|----------|
| J.2.1 | I resize the terminal while viewing the log source menu. | The layout reflows. The frame border redraws correctly. |
| J.2.2 | I resize the terminal while viewing a table sub-view (CloudTrail, Flow Logs, SSM). | Column visibility adjusts to the new width. The frame redraws. |
| J.2.3 | I resize the terminal while viewing a viewport sub-view (Console Output, CW Log Viewer, SSM Output). | The viewport adjusts to new dimensions. Word-wrapped content reflows if wrap is enabled. |
| J.2.4 | I resize the terminal to below 60 columns while in any log view. | An error message appears: "Terminal too narrow. Please resize." |
| J.2.5 | I resize the terminal to below 7 lines while in any log view. | An error message appears: "Terminal too short. Please resize." |

### J.3 Alternating Row Colors

| # | Story | Expected |
|---|-------|----------|
| J.3.1 | Table sub-views (CloudTrail, Flow Logs, SSM Commands) with more than 2 rows. | Alternating rows have a subtle background color difference (#1e2030). Selected row always has blue background regardless. |

### J.4 Graceful Degradation

| # | Story | Expected |
|---|-------|----------|
| J.4.1 | The IAM role lacks `cloudtrail:LookupEvents` permission. | CloudTrail source shows "Access denied" (red) in the log source menu. The source is dimmed and cursor-skipped. |
| J.4.2 | The IAM role lacks `ec2:GetConsoleOutput` permission. | Console Output source shows "Access denied" (red). |
| J.4.3 | The IAM role lacks `logs:DescribeLogGroups` permission. | CloudWatch Logs source shows "Access denied" (red). |
| J.4.4 | The IAM role lacks `ec2:DescribeInstanceStatus` permission. | Instance Status source shows "Access denied" (red). |
| J.4.5 | The IAM role lacks `ec2:DescribeFlowLogs` permission. | VPC Flow Logs source shows "Access denied" (red). |
| J.4.6 | The IAM role lacks `ssm:ListCommands` permission. | SSM Command History source shows "Access denied" (red). |
| J.4.7 | A network timeout occurs during an availability check. | The affected source shows "Timed out" (red) in the menu. Other sources are unaffected. |
| J.4.8 | API throttling occurs during data fetch inside a sub-view. | A red error flash appears in the header. The user can press `Ctrl+R` to retry. |
| J.4.9 | The instance is terminated and I try to open Console Output. | The source is unavailable ("Unavailable (terminated)") and cannot be selected. No crash. |
| J.4.10 | The instance is terminated and I try to open Instance Status. | The source is unavailable and cannot be selected. No crash. |
| J.4.11 | No AWS credentials are configured at all. | The EC2 list itself would fail to load, so the `l` key scenario is not reached. The error is handled at the EC2 list level. |

### J.5 Read-Only Guarantee

| # | Story | Expected |
|---|-------|----------|
| J.5.1 | I navigate through all 6 log sources and their sub-views. | No write API calls are made at any point. All operations are read-only (LookupEvents, GetConsoleOutput, DescribeInstanceStatus, DescribeLogGroups, DescribeLogStreams, FilterLogEvents, DescribeFlowLogs, DescribeNetworkInterfaces, ListCommands, GetCommandInvocation). |
| J.5.2 | I press every available key in every log view. | No key triggers a write operation (no start, stop, terminate, modify, or delete API calls). |

### J.6 Responsive Log Source Menu

| # | Story | Expected |
|---|-------|----------|
| J.6.1 | Terminal width is exactly 60 columns. | The log source menu is fully usable. Source names and availability text fit within the frame. |
| J.6.2 | Terminal width is 120+ columns. | Extra space is padded. Layout remains centered and clean. |

### J.7 Command Mode from Any Log View

| # | Story | Expected |
|---|-------|----------|
| J.7.1 | I press `:` from any log sub-view and type `s3`. | The entire view stack is replaced and the S3 bucket list opens. |
| J.7.2 | I press `:` and type `ec2`. | The view navigates to the EC2 instance list (popping all log views off the stack). |
| J.7.3 | I press `Esc` in command mode from any log view. | Command mode is cancelled. The current log view remains. |

### J.8 Quit from Any Log View

| # | Story | Expected |
|---|-------|----------|
| J.8.1 | I press `Ctrl+C` from any log sub-view. | The application force-quits immediately. |
| J.8.2 | I press `q` from any log sub-view. | The application quits (or the key is handled per the global quit behavior). |
