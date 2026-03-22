# QA User Stories: CloudWatch Log Streams and Log Events

Covers the Log Streams child view (entered from Log Groups) and the Log Events
child view (entered from Log Streams). Together with the parent Log Groups list,
these form a 3-level drill-down chain for reading CloudWatch logs.

All stories are written from a black-box perspective against the design spec and
`views.yaml` / `views_reference.yaml` configuration files.

AWS CLI equivalents are cited so testers can verify data parity.

---

## A. Log Streams Child View (Level 1: Log Groups --> Log Streams)

### A.1 Entry and Loading State

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I select a log group `/aws/lambda/payment-processor` in the Log Groups list and press Enter. | The view transitions to the Log Streams child view. A spinner (animated dot) appears centered in the frame with text like "Fetching log streams...". The frame title shows `log-streams` with no count while loading. |
| A.1.2 | I press keys (j, k, /, N, Enter) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. The spinner continues uninterrupted. |
| A.1.3 | The API responds successfully with stream data. | The spinner disappears. The table renders with column headers and rows. The frame title updates to `log-streams(N) --- /aws/lambda/payment-processor` where N is the total stream count. |
| A.1.4 | The API responds with an error (e.g., ResourceNotFoundException, AccessDeniedException). | The spinner disappears. A red error flash message appears in the header right side (e.g., "Error: AccessDenied"). The frame content area shows an appropriate empty or error state. |
| A.1.5 | The log group has thousands of streams and the API paginates multiple times. | The spinner remains visible until all pages are fetched. The frame title shows the running or final total once all pages complete. The user sees all streams, not just the first page. |

**AWS comparison:**
```
aws logs describe-log-streams --log-group-name /aws/lambda/payment-processor --order-by LastEventTime --descending
```
Expected fields visible: Stream Name, Last Event, First Event, Size

### A.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | The log group exists but contains zero log streams. | The frame title reads `log-streams(0) --- /aws/lambda/my-function`. The content area shows a centered message (e.g., "No log streams found") with a hint to refresh. No column headers are shown (or headers are shown with no data rows). |
| A.2.2 | I press ctrl+r on the empty state. | The loading spinner appears again while the refresh request is in flight. |
| A.2.3 | A newly created log group has never received any log data. | Same as A.2.1 -- the empty state is shown with count 0. |

### A.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | Log streams load and the table renders. | Exactly four columns are displayed: "Stream Name" (width 48), "Last Event" (width 22), "First Event" (width 22), "Size" (width 12). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| A.3.2 | I verify column data against `aws logs describe-log-streams`. | "Stream Name" maps to `.logStreams[].logStreamName`. "Last Event" maps to `.logStreams[].lastEventTimestamp`. "First Event" maps to `.logStreams[].firstEventTimestamp`. "Size" maps to `.logStreams[].storedBytes`. Every stream returned by the CLI appears as a row in the table. |
| A.3.3 | A stream name like `2026/03/22/[$LATEST]8a4b2c1d3e5f6a7b8c9d0e1f2a3b4c5d` is longer than 48 characters. | The name is truncated to fit the 48-character column width. No row wrapping occurs. |
| A.3.4 | The terminal is narrower than the combined column widths (48+22+22+12 = 104 plus borders/padding). | The rightmost column(s) are hidden (not truncated mid-value). Horizontal scroll with h/l is available to reveal hidden columns. |
| A.3.5 | The terminal is 80 columns wide. | At minimum, the Stream Name column is visible. Other columns may be hidden and accessible via horizontal scroll. |
| A.3.6 | I verify timestamps are displayed in a human-readable format. | The Last Event and First Event columns show formatted datetime values (e.g., "2026-03-22 02:47"), not raw epoch milliseconds. |
| A.3.7 | I verify the Size column shows human-readable sizes. | Stored bytes are displayed in a readable format (e.g., "14 KB", "2.3 MB") rather than raw byte counts. |

**AWS comparison:**
```
aws logs describe-log-streams --log-group-name /aws/lambda/my-function --order-by LastEventTime --descending --query 'logStreams[].{name:logStreamName,last:lastEventTimestamp,first:firstEventTimestamp,size:storedBytes}'
```
Expected fields visible: Stream Name (LogStreamName, width 48), Last Event (LastEventTimestamp, width 22), First Event (FirstEventTimestamp, width 22), Size (StoredBytes, width 12)

### A.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| A.4.1 | 347 streams are loaded for log group `/aws/lambda/payment-processor`. | The frame top border shows the title centered: `log-streams(347) --- /aws/lambda/payment-processor` with equal-length dashes on both sides. |
| A.4.2 | A filter is active and matches 5 of 347 streams. | The frame title reads `log-streams(5/347) --- /aws/lambda/payment-processor`. |
| A.4.3 | A filter is active and matches 0 streams. | The frame title reads `log-streams(0/347) --- /aws/lambda/payment-processor`. The content area is empty (no rows). |
| A.4.4 | The log group name is very long (e.g., `/aws/lambda/my-very-long-service-name-that-exceeds-frame-width`). | The title is rendered correctly. If the combined title exceeds the frame width, the log group name is truncated with enough space remaining for the count and dashes. |

### A.5 Default Sort Order

| ID | Story | Expected |
|----|-------|----------|
| A.5.1 | Log streams load for a log group with recent activity. | Streams are ordered by most recent event first (descending by LastEventTimestamp). The stream with the most recent activity appears at the top of the list. This matches the `orderBy=LastEventTime, descending=true` API parameter. |
| A.5.2 | I verify the default order against the AWS CLI. | Running `aws logs describe-log-streams --log-group-name GROUP --order-by LastEventTime --descending` produces the same ordering as displayed in the table. |

### A.6 Navigation

| ID | Story | Expected |
|----|-------|----------|
| A.6.1 | I press j (or down-arrow) with the first stream selected. | The selection cursor moves to the second stream. The previously selected row loses the blue highlight. The new row gains the full-width blue background (#7aa2f7) with dark foreground (#1a1b26), bold. |
| A.6.2 | I press k (or up-arrow) with the second stream selected. | The selection cursor moves back to the first stream. |
| A.6.3 | I press g. | The selection jumps to the very first stream in the list. |
| A.6.4 | I press G. | The selection jumps to the very last stream in the list. |
| A.6.5 | I press PageDown (or ctrl+d). | The selection moves down by one page of visible rows. If fewer rows remain below than a page, the cursor lands on the last row. |
| A.6.6 | I press PageUp (or ctrl+u). | The selection moves up by one page of visible rows. If fewer rows remain above than a page, the cursor lands on the first row. |
| A.6.7 | I press j on the last row. | The behavior depends on wrap configuration. If wrapping, cursor moves to the first row. If not, cursor stays on the last row. |
| A.6.8 | I press k on the first row. | The behavior depends on wrap configuration. If wrapping, cursor moves to the last row. If not, cursor stays on the first row. |
| A.6.9 | There are more streams than fit on screen. I scroll past the visible area. | The table scrolls to keep the selected row visible. The column headers remain in place. |
| A.6.10 | I press h (or left-arrow) when columns extend beyond the terminal width. | Columns scroll left (visible column window shifts), revealing any previously hidden left columns. |
| A.6.11 | I press l (or right-arrow) when columns extend beyond the terminal width. | Columns scroll right (visible column window shifts), revealing any previously hidden right columns. |
| A.6.12 | Column headers scroll in sync with data when I press h/l. | The column header row shifts horizontally by the same offset as data rows. |

### A.7 Sorting

| ID | Story | Expected |
|----|-------|----------|
| A.7.1 | I press N on the stream list. | Rows are sorted by stream name in ascending order. The "Stream Name" column header shows a sort indicator: an up-arrow appended directly (e.g., "Stream Name^"). |
| A.7.2 | I press N again. | Sort order toggles to descending. The indicator changes to a down-arrow. |
| A.7.3 | I press A on the stream list. | Rows are sorted by age (Last Event timestamp) in ascending order (oldest first). The "Last Event" column header shows the up-arrow indicator. The "Stream Name" header no longer shows any indicator. |
| A.7.4 | I press A again. | Sort order toggles to descending (newest first). The indicator changes to a down-arrow. |
| A.7.5 | I sort by name, then apply a filter. | The filtered subset remains sorted by name. The sort indicator persists on the column header. |
| A.7.6 | I sort by name, then refresh with ctrl+r. | After data reloads, the sort order and direction are preserved. The indicator remains. |

### A.8 Filter

| ID | Story | Expected |
|----|-------|----------|
| A.8.1 | I press /. | The header right side changes from "? for help" to "/\|" (amber/bold, with cursor). Filter mode is active. |
| A.8.2 | I type "LATEST" in filter mode. | The header right shows "/LATEST\|". Only rows whose stream name contains "LATEST" (case-insensitive) are displayed. The frame title updates to `log-streams(M/N) --- group-name` where M is the matched count. |
| A.8.3 | I press backspace in filter mode. | The last character of the filter text is removed. The filtered result updates immediately. |
| A.8.4 | I press Escape in filter mode. | The filter is cleared. All rows reappear. The frame title reverts to `log-streams(N) --- group-name`. The header right reverts to "? for help". |
| A.8.5 | I type a filter string that matches no streams. | Zero rows are displayed. The frame title shows `log-streams(0/N) --- group-name`. |
| A.8.6 | I type "latest" (lowercase) and stream names contain "[$LATEST]". | The matching streams appear. Filtering is case-insensitive. |
| A.8.7 | I have a filter active and press j/k. | Navigation works within the filtered result set only. |
| A.8.8 | I have a filter active and press Enter on a selected stream. | I drill into the selected stream's log events (same as unfiltered behavior). |
| A.8.9 | I filter by "2026/03/22" to find streams from a specific date. | Only streams whose names contain "2026/03/22" are shown. This is useful for Lambda-style stream names that embed dates. |

### A.9 Enter Key (Drill Into Stream)

| ID | Story | Expected |
|----|-------|----------|
| A.9.1 | I select a stream and press Enter. | The view transitions to the Log Events view for that stream. A loading spinner appears while events are fetched. The log streams view is pushed onto the view stack. |
| A.9.2 | I verify Enter navigates to log events, NOT to a detail view. | Pressing Enter on a stream navigates into the stream to show its log events. It does NOT open the stream detail/describe view. The detail view is accessed via d instead. |

### A.10 Detail Key (d)

| ID | Story | Expected |
|----|-------|----------|
| A.10.1 | I select a stream and press d. | The detail view opens for the selected stream. The frame title shows the stream name. The detail fields are rendered as key-value pairs. |
| A.10.2 | I verify the detail fields match views.yaml `log_streams.detail` config. | The detail view shows: LogStreamName, LastEventTimestamp, FirstEventTimestamp, StoredBytes, UploadSequenceToken, CreationTime, Arn. These are the seven fields listed under `views.log_streams.detail`. |
| A.10.3 | I compare detail fields against `aws logs describe-log-streams --log-group-name GROUP --log-stream-name-prefix STREAM`. | LogStreamName matches `.logStreams[0].logStreamName`. LastEventTimestamp matches `.logStreams[0].lastEventTimestamp`. StoredBytes matches `.logStreams[0].storedBytes`. Arn matches `.logStreams[0].arn`. |
| A.10.4 | I press Escape on the detail view. | I return to the Log Streams list. The cursor position is preserved on the same stream I had selected. |
| A.10.5 | Keys are rendered in blue (#7aa2f7), values in white (#c0caf5). | Visual inspection confirms the key-value color coding from the design spec. |

**AWS comparison:**
```
aws logs describe-log-streams --log-group-name /aws/lambda/my-function --log-stream-name-prefix "2026/03/22/[$LATEST]8a4b2c1d"
```
Expected detail fields: LogStreamName, LastEventTimestamp, FirstEventTimestamp, StoredBytes, UploadSequenceToken, CreationTime, Arn

### A.11 YAML Key (y)

| ID | Story | Expected |
|----|-------|----------|
| A.11.1 | I select a stream and press y. | The YAML view opens. The frame title includes the stream name and "yaml" (e.g., "2026/03/22/[$LATEST]8a4b yaml"). The full resource is rendered as syntax-highlighted YAML. |
| A.11.2 | YAML keys are colored blue (#7aa2f7), string values green (#9ece6a), numbers orange (#ff9e64), booleans purple (#bb9af7), null values dim (#565f89). | Visual inspection confirms the color coding matches the design spec. |
| A.11.3 | The YAML content is longer than the visible area. | I can scroll with j/k/g/G. Scroll indicators appear when content extends beyond the visible area. |
| A.11.4 | I press Escape on the YAML view. | I return to the Log Streams list. |

### A.12 Copy Key (c)

| ID | Story | Expected |
|----|-------|----------|
| A.12.1 | I select a stream and press c. | The full stream name is copied to the system clipboard (e.g., `2026/03/22/[$LATEST]8a4b2c1d3e5f`). A green flash message "Copied!" appears in the header right side. |
| A.12.2 | After ~2 seconds. | The "Copied!" flash message auto-clears and the header right reverts to "? for help". |
| A.12.3 | I paste from clipboard into another application. | The pasted text matches the full stream name exactly, including special characters like `[$LATEST]`. |
| A.12.4 | The stream name contains special characters (`[$LATEST]`, slashes, hashes). | All special characters are preserved in the copied text. No escaping or modification occurs. |

### A.13 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| A.13.1 | I press ctrl+r on the stream list. | The loading spinner appears. A fresh API call is made to `DescribeLogStreams`. When it completes, the table repopulates with current data. |
| A.13.2 | A new stream was created since the last load (e.g., a Lambda invocation created a new stream). I press ctrl+r. | The new stream appears in the refreshed list. The count in the frame title increments. |
| A.13.3 | I had a filter active and press ctrl+r. | The data refreshes. The filter remains applied to the new data. The frame title count updates accordingly. |
| A.13.4 | I had a sort override active (e.g., sorted by name) and press ctrl+r. | The data refreshes. The sort order and direction are preserved. |

### A.14 Escape (Back to Log Groups)

| ID | Story | Expected |
|----|-------|----------|
| A.14.1 | I press Escape on the Log Streams list (not in filter/command mode). | I return to the Log Groups list. The cursor is on the same log group I had entered. |

### A.15 Row Coloring

| ID | Story | Expected |
|----|-------|----------|
| A.15.1 | Log streams are displayed. | Rows are rendered in plain text color (#c0caf5). Log streams have no status field, so no status-based row coloring applies. |
| A.15.2 | I select a row. | The selected row has full-width blue background (#7aa2f7), dark foreground (#1a1b26), bold text. All other rows revert to their normal coloring. |
| A.15.3 | I move selection away from a row. | The previously selected row reverts to plain coloring. |
| A.15.4 | The stream list has more than 2 rows. | Alternating rows have a subtle background color difference (#1e2030) for readability. Selected row always has blue background regardless. |

### A.16 Stale Streams

| ID | Story | Expected |
|----|-------|----------|
| A.16.1 | A stream has a Last Event timestamp from weeks ago. | The timestamp is displayed normally. No special visual treatment is applied for stale timestamps (the timestamp simply shows the old date). |
| A.16.2 | A stream has no Last Event timestamp (null/empty -- never received events). | The Last Event column shows a dash, "null", or empty value rather than crashing. The stream still appears in the list. |
| A.16.3 | A stream has a First Event timestamp but no Last Event timestamp. | The First Event column displays the value; the Last Event column shows a null/empty indicator. Both columns are independently rendered. |

### A.17 Large Log Groups

| ID | Story | Expected |
|----|-------|----------|
| A.17.1 | A log group like `/aws/lambda/high-traffic-api` has 5,000+ streams. | All streams are fetched (the API paginates at 50 per page). The spinner remains until pagination completes. The frame title shows the full count (e.g., `log-streams(5247)`). |
| A.17.2 | I navigate with g/G on the 5,000-stream list. | g jumps to the first stream instantly. G jumps to the last stream instantly. Performance is acceptable (no visible lag). |
| A.17.3 | I filter the 5,000-stream list by typing a partial name. | Filtering responds within a reasonable time. Only matching streams are shown. |

### A.18 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| A.18.1 | I press ? on the stream list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: LOG STREAMS, GENERAL, NAVIGATION, HOTKEYS. |
| A.18.2 | The LOG STREAMS category lists the correct key bindings. | The category shows: `<esc>` Back, `<enter>` View Events, `<d>` Detail, `<y>` YAML, `<c>` Copy Name. |
| A.18.3 | I press any key on the help screen. | The help screen closes and the stream list table reappears. |

### A.19 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| A.19.1 | I press : on the stream list. | The header right side changes to ":\|" (amber/bold). Command mode is active. |
| A.19.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list, leaving the log chain entirely. |
| A.19.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The stream list remains. |

---

## B. Log Events Child View (Level 2: Log Streams --> Log Events)

### B.1 Entry and Loading State

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | I select a log stream in the Log Streams list and press Enter. | The view transitions to the Log Events child view. A spinner appears centered in the frame with text like "Fetching log events...". |
| B.1.2 | I press keys (j, k, /, t, w) while the spinner is visible. | No navigation or toggle occurs. Keypresses are ignored or queued until data loads. |
| B.1.3 | The API responds successfully with event data. | The spinner disappears. Log events are rendered as table rows with columns: Timestamp, Message. The frame title shows the stream name and event count. |
| B.1.4 | The API responds with an error (e.g., ResourceNotFoundException for a deleted stream). | The spinner disappears. A red error flash appears in the header (e.g., "Error: ResourceNotFoundException"). |
| B.1.5 | The stream contains a large volume of events (>10,000 events or >1MB). | The initial fetch returns the most recent events (up to API limits). The spinner is visible until the fetch completes. |

**AWS comparison:**
```
aws logs get-log-events --log-group-name /aws/lambda/payment-processor --log-stream-name "2026/03/22/[$LATEST]8a4b2c1d3e5f" --start-from-head false
```
Expected fields visible: Timestamp, Message

### B.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| B.2.1 | The log stream exists but contains zero log events. | The frame title reads `log-events(0) --- stream-name`. A centered message indicates no log events found. |
| B.2.2 | I press ctrl+r on the empty state. | The loading spinner appears again while the refresh request is in flight. |

### B.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| B.3.1 | Log events load and the table renders. | Two columns are displayed: "Timestamp" (width 22) and "Message" (width 0, meaning it fills all remaining horizontal space). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| B.3.2 | I verify column data against `aws logs get-log-events`. | "Timestamp" maps to `.events[].timestamp`. "Message" maps to `.events[].message`. Every event returned by the CLI appears as a row in the table. |
| B.3.3 | The Timestamp column displays a human-readable format. | Timestamps show formatted datetime values (e.g., "2026-03-22 02:47:31"), not raw epoch milliseconds. |
| B.3.4 | The Message column uses all remaining width after the Timestamp column. | On a 120-column terminal, the Message column is approximately 120 - 22 - borders/padding characters wide. |
| B.3.5 | A log message is longer than the available Message column width. | The message is truncated at the column boundary with an ellipsis or similar indicator. The full message is visible via the detail view (d) or by toggling word wrap (w). |

**AWS comparison:**
```
aws logs get-log-events --log-group-name GROUP --log-stream-name STREAM --start-from-head false --query 'events[].{timestamp:timestamp,message:message}'
```
Expected fields visible: Timestamp (Timestamp, width 22), Message (Message, width 0 = fill remaining)

### B.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| B.4.1 | 156 events are loaded for stream `2026/03/22/[$LATEST]8a4b2c1d`. | The frame title shows `log-events(156) --- 2026/03/22/[$LATEST]8a4b2c1d` centered in the top border. |
| B.4.2 | A filter is active and matches 12 of 156 events. | The frame title reads `log-events(12/156) --- 2026/03/22/[$LATEST]8a4b2c1d`. |
| B.4.3 | A filter is active and matches 0 events. | The frame title reads `log-events(0/156) --- stream-name`. The content area is empty. |
| B.4.4 | The stream name is very long. | The title is rendered correctly. If the combined title exceeds the frame width, the stream name is truncated. |

### B.5 Row Coloring (Log Level Detection)

| ID | Story | Expected |
|----|-------|----------|
| B.5.1 | A log event message contains `[ERROR]` (e.g., `[ERROR] StripeError: Card declined`). | The entire row is rendered in RED (#f7768e). |
| B.5.2 | A log event message contains `FATAL` (e.g., `FATAL: database connection refused`). | The entire row is rendered in RED (#f7768e). |
| B.5.3 | A log event message contains `Exception` (e.g., `java.lang.NullPointerException`). | The entire row is rendered in RED (#f7768e). |
| B.5.4 | A log event message contains `Traceback` (e.g., Python `Traceback (most recent call last):`). | The entire row is rendered in RED (#f7768e). |
| B.5.5 | A log event message contains `WARN` (e.g., `WARN: deprecated API version`). | The entire row is rendered in YELLOW (#e0af68). |
| B.5.6 | A log event message is a `REPORT` line (e.g., `REPORT RequestId: ... Duration: 2103.45 ms`). | The entire row is rendered in GREEN (#9ece6a). |
| B.5.7 | A log event message is a `START` line (e.g., `START RequestId: a1b2c3d4-...`). | The entire row is rendered in DIM (#565f89). |
| B.5.8 | A log event message is an `END` line (e.g., `END RequestId: a1b2c3d4-...`). | The entire row is rendered in DIM (#565f89). |
| B.5.9 | A log event message is a normal application log with no keywords. | The row is rendered in PLAIN text color (#c0caf5). |
| B.5.10 | I select an ERROR row. | The selected row has full-width blue background (#7aa2f7), dark foreground (#1a1b26), bold. The red coloring is overridden by the selection highlight. |
| B.5.11 | I move selection away from the ERROR row. | The row reverts to RED coloring. |
| B.5.12 | A message contains "error" in lowercase (e.g., `connection error occurred`). | Verify whether keyword detection is case-insensitive. If it is, the row is RED. If case-sensitive, the row is PLAIN. Document the actual behavior observed. |
| B.5.13 | A message contains "WARNING" (full word, not just "WARN"). | Verify whether the WARN detection matches partial substrings. If "WARNING" matches, the row is YELLOW. |
| B.5.14 | A REPORT line also contains "ERROR" in the message body. | The row coloring priority is determined: ERROR/FATAL/Exception/Traceback takes precedence over REPORT, or vice versa. Document the observed priority. |
| B.5.15 | Alternating rows apply alongside log-level coloring. | Alternating row background (#1e2030) is applied to even/odd rows. The foreground text color still reflects the log level (RED, YELLOW, GREEN, DIM, or PLAIN). |

### B.6 Navigation

| ID | Story | Expected |
|----|-------|----------|
| B.6.1 | I press j (or down-arrow) with the first event selected. | The selection cursor moves to the second event. The previously selected row loses the blue highlight. The new row gains the full-width blue background. |
| B.6.2 | I press k (or up-arrow) with the second event selected. | The selection cursor moves back to the first event. |
| B.6.3 | I press g. | The selection jumps to the very first event in the list (most recent, since events are loaded newest-first). |
| B.6.4 | I press G. | The selection jumps to the very last event in the list (oldest loaded event). |
| B.6.5 | I press PageDown (or ctrl+d). | The selection moves down by one page of visible rows. |
| B.6.6 | I press PageUp (or ctrl+u). | The selection moves up by one page of visible rows. |
| B.6.7 | There are more events than fit on screen. I scroll past the visible area. | The table scrolls to keep the selected row visible. The column headers remain in place. |

### B.7 Timestamp Toggle (t key)

| ID | Story | Expected |
|----|-------|----------|
| B.7.1 | I press t with the default view (timestamps visible). | The Timestamp column disappears. The Message column expands to fill the entire width of the frame. This gives maximum horizontal space for reading log messages. |
| B.7.2 | I press t again. | The Timestamp column reappears at its original width (22). The Message column shrinks accordingly. |
| B.7.3 | I press t to hide timestamps, then scroll through events. | Navigation works normally. Events are still ordered by time even though the timestamp column is hidden. |
| B.7.4 | I press t to hide timestamps, then press d on a selected event. | The detail view opens and shows the full Timestamp field among the detail fields. The timestamp is always visible in the detail view regardless of the toggle state. |
| B.7.5 | I press t to hide timestamps, then apply a filter. | Filtering still works. The filter searches message content. The frame title updates normally. |
| B.7.6 | I press t to hide timestamps, press Escape to return to Log Streams, then Enter on the same stream again. | Document whether the timestamp toggle state is preserved across re-entry or resets to default (timestamps visible). |
| B.7.7 | I press t to hide timestamps, then press ctrl+r to refresh. | The data refreshes. The timestamp toggle state is preserved (timestamps remain hidden). |

### B.8 Word Wrap Toggle (w key)

| ID | Story | Expected |
|----|-------|----------|
| B.8.1 | A long log message extends beyond the visible Message column width. With default settings (no wrap). | The message is truncated at the column boundary. |
| B.8.2 | I press w to enable word wrap. | Long messages wrap to subsequent lines within the table. The full message text is visible without horizontal scrolling. |
| B.8.3 | I press w again to disable word wrap. | Messages revert to single-line truncation. |
| B.8.4 | A JSON log message is multiple lines (e.g., a pretty-printed JSON payload). | With word wrap OFF, only the first line (or as much as fits) is shown truncated. With word wrap ON, the full JSON is visible across multiple visual lines. |
| B.8.5 | A stack trace log message spans many lines (e.g., Java or Python traceback). | With word wrap ON, the full stack trace is visible. With word wrap OFF, it is truncated to a single line. |
| B.8.6 | I enable word wrap and then navigate with j/k. | Each j/k moves to the next/previous log event, not the next visual line within a wrapped event. |
| B.8.7 | I enable word wrap and the table has fewer events but more visual lines. | Scrolling behavior adapts to the wrapped content height. The frame shows scroll indicators if content exceeds the visible area. |
| B.8.8 | I press w to toggle word wrap, then press ctrl+r. | The data refreshes. The word wrap state is preserved. |

### B.9 Horizontal Scroll (without word wrap)

| ID | Story | Expected |
|----|-------|----------|
| B.9.1 | Word wrap is OFF. A message extends beyond the visible width. I press l (or right-arrow). | The view scrolls horizontally to reveal more of the message text. |
| B.9.2 | I press h (or left-arrow) after scrolling right. | The view scrolls back to the left. |
| B.9.3 | Word wrap is ON. I press h/l. | Horizontal scroll is a no-op (or has no visible effect) since all content is already wrapped within the visible width. |
| B.9.4 | I have timestamps hidden (t toggle) and press l. | Horizontal scroll works on the message column, which now occupies the full width. |

### B.10 Filter on Log Events

| ID | Story | Expected |
|----|-------|----------|
| B.10.1 | I press / and type "ERROR". | Only events whose message contains "ERROR" (case-insensitive) are shown. The frame title updates to `log-events(M/N)`. All visible rows are RED-colored (since they all contain "ERROR"). |
| B.10.2 | I press / and type "payment". | Only events whose message contains "payment" are shown. This is useful for finding specific business logic in logs. |
| B.10.3 | I press / and type a UUID (e.g., "a1b2c3d4"). | Only events containing that request ID are shown. This simulates tracing a request through logs. |
| B.10.4 | I press Escape while filter is active. | The filter clears. All events reappear. |
| B.10.5 | I filter for "ERROR" and then press c on a selected event. | The message of the selected event is copied to the clipboard, not the filter text. |
| B.10.6 | I filter and the resulting set has zero matches. | Zero rows are displayed. The frame title shows `log-events(0/N)`. |

### B.11 Copy Behavior (c key)

| ID | Story | Expected |
|----|-------|----------|
| B.11.1 | I select a normal log event and press c. | The full message text of the selected event is copied to the clipboard (WITHOUT the timestamp prefix). A green flash "Copied!" appears in the header. |
| B.11.2 | After ~2 seconds. | The "Copied!" flash message auto-clears. |
| B.11.3 | The message contains special characters (brackets, quotes, newlines, JSON). | All special characters are preserved exactly in the copied text. |
| B.11.4 | The message contains a JSON payload: `{"orderId": "ORD-123", "amount": 149.99}`. | The copied text is the exact JSON string. |
| B.11.5 | The message contains a Python stack trace with newlines and indentation. | The copied text preserves the full multi-line stack trace. |
| B.11.6 | The message contains Lambda-specific tokens like `RequestId: a1b2c3d4-e5f6-...`. | The full RequestId is preserved in the copied text. |
| B.11.7 | I paste the copied message into a terminal to use with `aws logs filter-log-events`. | The pasted text is usable as a search pattern. |

### B.12 Detail Key (d)

| ID | Story | Expected |
|----|-------|----------|
| B.12.1 | I select a log event and press d. | The detail view opens for the selected event. The frame title reflects the event. |
| B.12.2 | I verify the detail fields match views.yaml `log_events.detail` config. | The detail view shows: Timestamp, IngestionTime, Message, EventId. These are the four fields listed under `views.log_events.detail`. |
| B.12.3 | The Message field in the detail view contains a very long string. | The full message is displayed in the detail view with scrolling support (j/k/g/G). Word wrap (w) is available in the detail view. |
| B.12.4 | I compare detail fields against `aws logs get-log-events`. | Timestamp matches `.events[].timestamp`. IngestionTime matches `.events[].ingestionTime`. Message matches `.events[].message`. |
| B.12.5 | I press Escape on the detail view. | I return to the Log Events list. The cursor position is preserved on the same event I had selected. |
| B.12.6 | The IngestionTime differs from the Timestamp. | Both values are displayed independently. IngestionTime is when CloudWatch received the event; Timestamp is when the application emitted it. |

**AWS comparison:**
```
aws logs get-log-events --log-group-name GROUP --log-stream-name STREAM --start-from-head false --query 'events[0]'
```
Expected detail fields: Timestamp, IngestionTime, Message, EventId

### B.13 YAML Key (y)

| ID | Story | Expected |
|----|-------|----------|
| B.13.1 | I select a log event and press y. | The YAML view opens. The frame title includes "yaml". The full event is rendered as syntax-highlighted YAML. |
| B.13.2 | The Message field in YAML view contains a long string or multi-line content. | The message value is rendered as a YAML string (possibly a block scalar). Syntax coloring applies: key in blue, string value in green. |
| B.13.3 | I press Escape on the YAML view. | I return to the Log Events list. |

### B.14 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| B.14.1 | I press ctrl+r on the log events list. | The spinner appears. A fresh `GetLogEvents` call is made. The table updates with new results. |
| B.14.2 | New events were written to the stream since last load. I press ctrl+r. | The new events appear in the refreshed list. The count in the frame title increases. The newest events appear at the top (since `startFromHead=false`). |
| B.14.3 | I had a filter active and press ctrl+r. | The data refreshes. The filter remains applied. |
| B.14.4 | I had timestamps hidden (t) and word wrap on (w), then press ctrl+r. | Both toggle states are preserved after refresh. |

### B.15 Escape (Back to Log Streams)

| ID | Story | Expected |
|----|-------|----------|
| B.15.1 | I press Escape on the Log Events list (not in filter/command mode). | I return to the Log Streams list. The cursor is on the same stream I had entered. |
| B.15.2 | I do NOT return to the Log Groups list. | Escape goes back exactly one level (Log Events --> Log Streams), not two levels. |

### B.16 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| B.16.1 | I press ? on the events list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: LOG EVENTS, GENERAL, NAVIGATION, HOTKEYS. |
| B.16.2 | The LOG EVENTS category lists the correct key bindings. | The category shows: `<esc>` Back, `<d>` Detail, `<y>` YAML, `<c>` Copy Message, `<t>` Toggle Time, `<w>` Word Wrap. |
| B.16.3 | The `<t>` and `<w>` keys appear in the help screen. | These are new key bindings specific to the Log Events view. They appear under the LOG EVENTS category. |
| B.16.4 | I press any key on the help screen. | The help screen closes and the events list table reappears. |

### B.17 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| B.17.1 | I press : on the events list. | The header right side changes to ":\|" (amber/bold). Command mode is active. |
| B.17.2 | I type "logs" and press Enter. | The view navigates to the CloudWatch Log Groups list, exiting the log chain entirely. |
| B.17.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The events list remains. |

### B.18 Multi-Line Log Messages

| ID | Story | Expected |
|----|-------|----------|
| B.18.1 | A log event contains a JSON object formatted across multiple lines. | In the list view with word wrap OFF, only the first line (or a concatenated single-line version) is shown, truncated. In the detail view, the full JSON is visible. |
| B.18.2 | A log event contains a Java stack trace (`at com.example.Class.method(File.java:42)`). | In the list view, the stack trace is compressed to one line (truncated). The detail view shows the full multi-line trace. The row is colored RED due to the Exception keyword. |
| B.18.3 | A log event contains a Python traceback with indented frames. | Similar to B.18.2 -- truncated in list, full in detail. Row is RED. |
| B.18.4 | I enable word wrap (w) on a list containing multi-line messages. | Wrapped messages display across multiple visual rows. Each log event is visually separated. Navigation (j/k) still moves between events, not between visual lines within one event. |
| B.18.5 | I copy (c) a multi-line log message. | The clipboard contains the full multi-line message text with preserved newlines and indentation. |

### B.19 Pagination

| ID | Story | Expected |
|----|-------|----------|
| B.19.1 | The log stream has more events than fit in a single API response. | The initial load fetches the most recent events. The frame title shows the count of fetched events. |
| B.19.2 | I scroll to the bottom of the loaded events (oldest loaded) and there are more events available. | Document whether automatic pagination loads older events, or whether the user must refresh/scroll further to trigger additional fetches. |
| B.19.3 | I press ctrl+r after viewing paginated results. | A fresh fetch occurs from the most recent events. Any pagination state resets. |

---

## C. Full 3-Level View Stack Navigation

### C.1 Forward Navigation (Drill Down)

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | Main Menu --> Log Groups --> press Enter on `/aws/lambda/my-function`. | The Log Streams view opens for that log group. Frame title: `log-streams(N) --- /aws/lambda/my-function`. |
| C.1.2 | Log Streams --> press Enter on `2026/03/22/[$LATEST]8a4b2c1d`. | The Log Events view opens for that stream. Frame title: `log-events(N) --- 2026/03/22/[$LATEST]8a4b2c1d`. |
| C.1.3 | Log Events --> press d on a selected event. | The Detail view opens for that event showing: Timestamp, IngestionTime, Message, EventId. |
| C.1.4 | Detail --> press y. | The YAML view opens for the same event. |

### C.2 Backward Navigation (Escape Chain)

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | YAML view of a log event --> press Escape. | Returns to the Detail view of the same event. |
| C.2.2 | Detail view of a log event --> press Escape. | Returns to the Log Events list. The cursor is on the same event. |
| C.2.3 | Log Events list --> press Escape. | Returns to the Log Streams list. The cursor is on the same stream. |
| C.2.4 | Log Streams list --> press Escape. | Returns to the Log Groups list. The cursor is on the same log group. |
| C.2.5 | Log Groups list --> press Escape. | Returns to the Main Menu. |

### C.3 Full Round Trip

| ID | Story | Expected |
|----|-------|----------|
| C.3.1 | Main Menu --> Log Groups --> Log Streams --> Log Events --> Detail --> YAML; then Escape five times. | Each Escape pops one level: YAML --> Detail --> Log Events --> Log Streams --> Log Groups --> Main Menu. No state is lost at any intermediate level. Cursor positions are preserved at each level. |
| C.3.2 | I drill into one log group, press Escape back to Log Groups, then drill into a different log group. | The second log group's streams are fetched. The previous log group's stream data is not shown. |
| C.3.3 | I drill into a stream, view events, press Escape to Log Streams, then Enter on a different stream. | The second stream's events are fetched. The previous stream's events are not shown. |

### C.4 Cross-Level State Preservation

| ID | Story | Expected |
|----|-------|----------|
| C.4.1 | I apply a filter on Log Streams, then Enter on a filtered stream to view events, then Escape back. | The Log Streams filter is preserved when I return. The filtered view is restored. |
| C.4.2 | I apply a sort on Log Streams, then Enter to view events, then Escape back. | The sort order is preserved when I return. |
| C.4.3 | I toggle timestamps off (t) in Log Events, press Escape to Log Streams, then Enter the same stream again. | Document whether the t-toggle state persists for the same stream or resets. |

---

## D. Cross-Cutting Concerns

### D.1 Header Consistency

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | In every view in the log chain (Log Groups, Log Streams, Log Events, Detail, YAML), the header displays: "a9s" (accent bold), version (dim), profile:region (bold). | Visual inspection confirms the header is identical across all views. |
| D.1.2 | The header right side shows "? for help" in normal mode across all log views. | Confirmed in Log Streams list, Log Events list, Detail, and YAML views. |
| D.1.3 | Flash messages (Copied!, errors) appear in the header right side across all log views. | A green "Copied!" flash after pressing c, and red error flashes on API failures, display correctly in every log view. |

### D.2 Terminal Resize

| ID | Story | Expected |
|----|-------|----------|
| D.2.1 | I resize the terminal while viewing the Log Streams list. | The layout reflows. Column visibility adjusts to the new width. The frame border redraws correctly. Stream Name column priority is maintained. |
| D.2.2 | I resize the terminal while viewing the Log Events list. | The layout reflows. The Message column (width 0 = fill remaining) adjusts to the new available width. |
| D.2.3 | I resize the terminal while viewing Log Events with word wrap ON. | Wrapped lines re-wrap to the new width. |
| D.2.4 | I resize the terminal to below 60 columns while in any log view. | An error message appears: "Terminal too narrow. Please resize." |
| D.2.5 | I resize the terminal to below 7 lines while in any log view. | An error message appears: "Terminal too short. Please resize." |

### D.3 Profile/Region Switch

| ID | Story | Expected |
|----|-------|----------|
| D.3.1 | I am viewing Log Streams and switch profiles via `:ctx`. | The view returns to the Main Menu (or the profile selector). After selecting a profile, the previous log chain context is not preserved (since the new profile may not have the same log groups). |
| D.3.2 | I am viewing Log Events and switch regions via `:region`. | The view returns to the region selector. After selecting a region, the previous log chain context is not preserved. |

### D.4 Read-Only Verification

| ID | Story | Expected |
|----|-------|----------|
| D.4.1 | I verify there are no write actions available in the Log Streams view. | No key binding creates, deletes, or modifies any log stream. All interactions are read-only. |
| D.4.2 | I verify there are no write actions available in the Log Events view. | No key binding creates, deletes, or modifies any log event. All interactions are read-only. |
| D.4.3 | I press x (reveal secret key) in the Log Streams or Log Events view. | Nothing happens. The x key is specific to Secrets Manager and has no effect in log views. |

### D.5 Very Long Stream Names

| ID | Story | Expected |
|----|-------|----------|
| D.5.1 | A Lambda stream name is `2026/03/22/[$LATEST]8a4b2c1d3e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b` (60+ chars). | In the list view, the name is truncated to 48 characters. The full name is visible in the detail view (d). The copy key (c) copies the full untruncated name. |
| D.5.2 | The stream name appears in the Log Events frame title. | If the full name exceeds available title space, it is truncated to fit within the frame border. |

### D.6 Timestamp Formatting Consistency

| ID | Story | Expected |
|----|-------|----------|
| D.6.1 | Timestamps in the Log Streams list (Last Event, First Event columns) use the same format as timestamps in the Log Events list (Timestamp column). | Both use the same human-readable datetime format for consistency. |
| D.6.2 | The detail view shows timestamps in the same format as the list view. | Timestamps in detail key-value pairs match the format used in list columns. |

### D.7 Quit from Deep in the Chain

| ID | Story | Expected |
|----|-------|----------|
| D.7.1 | I am viewing Log Events (3 levels deep) and press ctrl+c. | The application exits immediately from any depth in the view stack. |
| D.7.2 | I am viewing Log Events and press q. | Document whether q quits the app from within child views or only works on the main menu. |
