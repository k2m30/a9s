# QA User Stories: Lambda Child Views

Covers the Lambda Invocations list (Level 1), Lambda Invocation Log Lines (Level 2),
and Lambda Function Code view (Level 1-alt).
All stories are written from a black-box perspective against the design spec and
`views.yaml` / `views_reference.yaml` configuration files.

AWS CLI equivalents are cited so testers can verify data parity.

---

## A. Lambda Invocations List View (Level 1)

### A.1 Loading State

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I select a Lambda function in the Lambda list and press Enter. | The view transitions to the invocations list. A spinner (animated dot) is displayed centered inside the frame. The text reads "Fetching invocations..." (or similar). The frame title reads "lambda-invocations" with no count and no function name suffix yet. The header shows "? for help" on the right. |
| A.1.2 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. The spinner continues uninterrupted. |
| A.1.3 | The CloudWatch Logs API responds with invocation data. | The spinner disappears. The table renders with column headers and rows. The frame title updates to "lambda-invocations(25) -- my-function" where 25 is the invocation count. |
| A.1.4 | The CloudWatch Logs API responds with an error (e.g., AccessDeniedException, log group not found). | The spinner disappears. A red error flash message appears in the header right side (e.g., "Error: AccessDeniedException"). The frame content shows an appropriate empty or error state. |
| A.1.5 | The Lambda function has a custom log group configured via LoggingConfig.LogGroup (not the default `/aws/lambda/{name}`). | The loading spinner appears and the app queries the custom log group. Invocations are fetched correctly from the custom log group. |
| A.1.6 | The Lambda function has no log group at all (newly created, never invoked). | The API returns an error (ResourceNotFoundException for the log group). A user-friendly message appears indicating no log group exists, rather than a raw API error. |

**AWS comparison:**
```
aws logs filter-log-events \
  --log-group-name /aws/lambda/my-function \
  --filter-pattern "REPORT RequestId"
```
Expected fields visible: Timestamp, Request ID, Status, Duration, Memory, Cold Start

### A.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | The log group exists but contains no REPORT lines (Lambda never invoked, or only test events with no output). | The frame title reads "lambda-invocations(0) -- my-function". The content area shows a centered message (e.g., "No invocations found") with a hint to refresh or check the function. |
| A.2.2 | I press ctrl+r on the empty state. | The loading spinner appears again while the refresh request is in flight. |

### A.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | Invocations load and the table renders. | Six columns are displayed: "Timestamp" (width 22), "Request ID" (width 12), "Status" (width 8), "Duration" (width 10), "Memory" (width 14), "Cold Start" (width 10). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| A.3.2 | I verify the Timestamp column data. | Timestamps are in a human-readable format (e.g., "2026-03-22 02:47") parsed from the REPORT log event's timestamp. The value corresponds to the CloudWatch Logs event timestamp for each REPORT line. |
| A.3.3 | I verify the Request ID column data. | The Request ID column shows a truncated form of the UUID (e.g., "a1b2c3d4") since the column width is 12 characters. The full UUID is available via copy (c key). |
| A.3.4 | I verify the Status column. | Status shows "OK" for successful invocations, "ERROR" for invocations with errors, or "TIMEOUT" for invocations that timed out. Status is derived by cross-referencing REPORT lines with ERROR/Task-timed-out log lines for the same RequestId. |
| A.3.5 | I verify the Duration column. | Duration shows the execution time in milliseconds (e.g., "2103 ms"), parsed from "Duration: 2103.45 ms" in the REPORT line. |
| A.3.6 | I verify the Memory column. | Memory shows used/configured format (e.g., "128/256 MB"), parsed from "Memory Size: 256 MB" and "Max Memory Used: 128 MB" in the REPORT line. |
| A.3.7 | I verify the Cold Start column. | Cold Start shows "yes" or "no". "yes" is shown when the REPORT line contains "Init Duration:" (which only appears for cold starts). |
| A.3.8 | A Request ID is longer than the 12-character column width. | The Request ID is truncated to fit. No row wrapping occurs. |
| A.3.9 | The terminal is narrower than the combined column widths (22+12+8+10+14+10=76 plus borders/padding). | The rightmost column(s) are hidden (not truncated mid-value). Horizontal scroll with h/l is available to reveal hidden columns. |

**AWS comparison:**
```
aws logs filter-log-events \
  --log-group-name /aws/lambda/my-function \
  --filter-pattern "REPORT RequestId" \
  --limit 25
```
Parse each REPORT line: `REPORT RequestId: abc123  Duration: 2103.45 ms  Billed Duration: 2200 ms  Memory Size: 256 MB  Max Memory Used: 128 MB  Init Duration: 312.52 ms`

### A.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| A.4.1 | 25 invocations are loaded for function "payment-processor". | The frame top border shows the title centered: "lambda-invocations(25) -- payment-processor" with equal-length dashes on both sides. |
| A.4.2 | A filter is active and matches 3 of 25 invocations. | The frame title reads "lambda-invocations(3/25) -- payment-processor". |
| A.4.3 | A filter is active and matches 0 invocations. | The frame title reads "lambda-invocations(0/25) -- payment-processor". The content area is empty (no rows). |

### A.5 Navigation

| ID | Story | Expected |
|----|-------|----------|
| A.5.1 | I press j (or down-arrow) with the first invocation selected. | The selection cursor moves to the second invocation. The previously selected row loses the blue highlight. The new row gains the full-width blue background (#7aa2f7) with dark foreground (#1a1b26), bold. |
| A.5.2 | I press k (or up-arrow) with the second invocation selected. | The selection cursor moves back to the first invocation. |
| A.5.3 | I press g. | The selection jumps to the very first invocation in the list (most recent). |
| A.5.4 | I press G. | The selection jumps to the very last invocation in the list (oldest). |
| A.5.5 | I press PageDown (or ctrl+d). | The selection moves down by one page of visible rows. If fewer rows remain below than a page, the cursor lands on the last row. |
| A.5.6 | I press PageUp (or ctrl+u). | The selection moves up by one page of visible rows. If fewer rows remain above than a page, the cursor lands on the first row. |
| A.5.7 | I press h (or left-arrow). | Columns scroll left, revealing any previously hidden left columns. Column headers scroll in sync with data. |
| A.5.8 | I press l (or right-arrow). | Columns scroll right, revealing any previously hidden right columns. Column headers scroll in sync with data. |
| A.5.9 | There are more invocations than fit on screen. I scroll past the visible area. | The table scrolls to keep the selected row visible. The column headers remain in place. |

### A.6 Row Coloring

| ID | Story | Expected |
|----|-------|----------|
| A.6.1 | An invocation has status "OK" and cold_start "no". | The entire row is rendered in GREEN (#9ece6a). |
| A.6.2 | An invocation has status "ERROR". | The entire row is rendered in RED (#f7768e). |
| A.6.3 | An invocation has status "TIMEOUT". | The entire row is rendered in RED (#f7768e). |
| A.6.4 | An invocation has status "OK" and cold_start "yes". | The entire row is rendered in YELLOW (#e0af68), because cold starts are performance anomalies worth highlighting even when the invocation succeeded. |
| A.6.5 | I select a row that is colored RED (ERROR). | The selected row shows full-width blue background (#7aa2f7), dark foreground (#1a1b26), bold. The red coloring is overridden by the selection highlight. |
| A.6.6 | I move selection away from the RED row. | The row reverts to its RED coloring. |
| A.6.7 | Alternating rows have status "OK" (no cold start). | Alternating rows have a subtle background color difference (#1e2030) for readability, in addition to the green text color. The selected row always has blue background regardless. |

### A.7 Sorting

| ID | Story | Expected |
|----|-------|----------|
| A.7.1 | I press N on the invocations list. | Rows are sorted by Request ID in ascending order. The "Request ID" column header shows a sort indicator: an up-arrow appended directly (e.g., "Request ID^"). |
| A.7.2 | I press N again. | Sort order toggles to descending. The indicator changes to a down-arrow. |
| A.7.3 | I press S on the invocations list. | Rows are sorted by Status in ascending order. The "Status" column header shows the sort indicator. |
| A.7.4 | I press A on the invocations list. | Rows are sorted by timestamp (age) in ascending order (oldest first). The "Timestamp" column header shows the up-arrow indicator. |
| A.7.5 | I press A again. | Sort order toggles to descending (newest first). The indicator changes to a down-arrow. |
| A.7.6 | I sort by status, then apply a filter. | The filtered subset remains sorted by status. The sort indicator persists. |

### A.8 Filter

| ID | Story | Expected |
|----|-------|----------|
| A.8.1 | I press / in the invocations list. | The header right side changes from "? for help" to "/|" (amber/bold, with cursor). Filter mode is active. |
| A.8.2 | I type "ERROR" in filter mode. | The header right shows "/ERROR|". Only rows whose status contains "ERROR" (case-insensitive) are displayed. The frame title updates to "lambda-invocations(M/N) -- my-function". |
| A.8.3 | I type "yes" to filter for cold starts. | Only invocations with "yes" in the Cold Start column are shown. |
| A.8.4 | I press Escape in filter mode. | The filter is cleared. All rows reappear. The frame title reverts to showing the total count. The header right reverts to "? for help". |
| A.8.5 | I type a filter string that matches no invocations. | Zero rows are displayed. The frame title shows "lambda-invocations(0/N) -- my-function". |
| A.8.6 | I have a filter active and press Enter on a selected invocation. | I drill into the selected invocation's log lines (same as unfiltered behavior). |

### A.9 Enter Key (Drill Into Invocation Log Lines)

| ID | Story | Expected |
|----|-------|----------|
| A.9.1 | I select an invocation and press Enter. | The view transitions to the Log Lines view for that specific invocation. A loading spinner appears while the log lines are fetched. The invocations list view is pushed onto the view stack. |
| A.9.2 | I verify Enter navigates to log lines, not a detail view. | Pressing Enter on an invocation navigates into the invocation to show its log output. It does NOT open the invocation detail/describe view. |

### A.10 Detail Key (d)

| ID | Story | Expected |
|----|-------|----------|
| A.10.1 | I select an invocation and press d. | The detail view opens for the selected invocation. The frame title shows the truncated Request ID. |
| A.10.2 | I verify the detail fields match views.yaml lambda_invocations detail config. | The detail view shows key-value pairs for: request_id, timestamp, status, duration_ms, billed_duration_ms, memory_size_mb, memory_used_mb, init_duration_ms, xray_trace_id. |
| A.10.3 | The invocation was a warm start (no cold start). | The init_duration_ms field shows an empty value, null, or dash. All other fields are populated. |
| A.10.4 | The invocation has no X-Ray trace ID (tracing not enabled). | The xray_trace_id field shows an empty value, null, or dash. |
| A.10.5 | I press Escape on the detail view. | I return to the invocations list. The cursor position is preserved on the same invocation I had selected. |

**AWS comparison:**
```
aws logs filter-log-events \
  --log-group-name /aws/lambda/my-function \
  --filter-pattern "REPORT RequestId" \
  --limit 1
```
Expected detail fields: request_id, timestamp, status, duration_ms, billed_duration_ms, memory_size_mb, memory_used_mb, init_duration_ms, xray_trace_id

### A.11 YAML Key (y)

| ID | Story | Expected |
|----|-------|----------|
| A.11.1 | I select an invocation and press y. | The YAML view opens. The frame title includes the Request ID and "yaml". The full invocation data is rendered as syntax-highlighted YAML. |
| A.11.2 | YAML keys are colored blue (#7aa2f7), string values green (#9ece6a), numbers orange (#ff9e64), booleans purple (#bb9af7), null values dim (#565f89). | Visual inspection confirms the color coding matches the design spec. |
| A.11.3 | I press Escape on the YAML view. | I return to the invocations list. |

### A.12 Copy Key (c)

| ID | Story | Expected |
|----|-------|----------|
| A.12.1 | I select an invocation and press c. | The full Request ID (complete UUID, e.g., "a1b2c3d4-e5f6-7890-abcd-ef1234567890") is copied to the system clipboard, NOT the truncated version shown in the table. A green flash message "Copied!" appears in the header right side. |
| A.12.2 | After approximately 2 seconds. | The "Copied!" flash message auto-clears and the header right reverts to "? for help". |
| A.12.3 | I paste from clipboard into another application. | The pasted text matches the full Request ID UUID exactly, not the truncated display value. |

### A.13 Source Code Key (s)

| ID | Story | Expected |
|----|-------|----------|
| A.13.1 | I press s while viewing the invocations list. | The view transitions to the Function Code view for the parent Lambda function. A loading spinner appears while the code is being downloaded. |
| A.13.2 | I verify the view stack after pressing s from invocations. | The view stack is: Lambda list --> Invocations --> Function Code. Pressing Escape from Function Code returns to the Invocations list. |

### A.14 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| A.14.1 | I press ctrl+r on the invocations list. | The loading spinner appears. A fresh CloudWatch Logs API call is made. When it completes, the table repopulates with current data. |
| A.14.2 | A new invocation occurred since the last load. I press ctrl+r. | The new invocation appears in the refreshed list. The count in the frame title increments. |
| A.14.3 | I had a filter active and press ctrl+r. | The data refreshes. The filter remains applied to the new data. The frame title count updates accordingly. |

### A.15 Escape (Back to Lambda List)

| ID | Story | Expected |
|----|-------|----------|
| A.15.1 | I press Escape on the invocations list (not in filter mode). | I return to the Lambda function list. The cursor is on the same function I had entered. |
| A.15.2 | I press Escape while filter mode is active. | The filter is cleared. I remain on the invocations list. I do NOT navigate back. |

### A.16 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| A.16.1 | I press ? on the invocations list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: INVOCATIONS, GENERAL, NAVIGATION, HOTKEYS. |
| A.16.2 | The INVOCATIONS column shows: esc (Back), enter (View Logs), d (Detail), y (YAML), c (Copy Req ID), s (Source Code). | All invocation-specific key bindings are listed. |
| A.16.3 | The GENERAL column shows: ctrl-r (Refresh), / (Filter), : (Command), q (Quit). | Standard general key bindings are present. |
| A.16.4 | The NAVIGATION column shows: j (Down), k (Up), g (Top), G (Bottom), h/l (Cols), pgup/dn (Page). | Standard navigation key bindings are present. |
| A.16.5 | The HOTKEYS column shows: ? (Help), : (Command). | Standard hotkey bindings are present. |
| A.16.6 | I press any key on the help screen. | The help screen closes and the invocations list table reappears. |

### A.17 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| A.17.1 | I press : on the invocations list. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| A.17.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list. |
| A.17.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The invocations list remains. |

### A.18 REPORT Line Parsing Edge Cases

| ID | Story | Expected |
|----|-------|----------|
| A.18.1 | A REPORT line has no "Init Duration" field (warm start). | The Cold Start column shows "no". The init_duration_ms detail field is empty/null/dash. All other fields are populated normally. |
| A.18.2 | A REPORT line has an "Init Duration" field (cold start). | The Cold Start column shows "yes". The init_duration_ms detail field shows the value (e.g., "312.52 ms"). The row is colored YELLOW even if status is OK. |
| A.18.3 | An invocation timed out (Duration equals or approaches the function's Timeout setting, and a "Task timed out" log line exists). | Status shows "TIMEOUT". The row is colored RED (#f7768e). Duration shows the full timeout value. |
| A.18.4 | An invocation ran out of memory (Max Memory Used equals or exceeds Memory Size, and an error log exists). | Status shows "ERROR". The Memory column shows used/configured where used is at or near the configured limit (e.g., "256/256 MB"). The row is colored RED. |
| A.18.5 | A REPORT line contains an XRAY TraceId field. | The xray_trace_id detail field is populated with the trace ID. This can be used for correlation with X-Ray traces. |
| A.18.6 | A REPORT line does NOT contain an XRAY TraceId field. | The xray_trace_id detail field shows empty/null/dash. No error or crash occurs. |

**AWS comparison:**
```
# Get REPORT lines
aws logs filter-log-events \
  --log-group-name /aws/lambda/my-function \
  --filter-pattern "REPORT RequestId"

# Get ERROR lines to cross-reference
aws logs filter-log-events \
  --log-group-name /aws/lambda/my-function \
  --filter-pattern "ERROR"

# Get TIMEOUT lines to cross-reference
aws logs filter-log-events \
  --log-group-name /aws/lambda/my-function \
  --filter-pattern "Task timed out"
```

### A.19 Custom Log Group

| ID | Story | Expected |
|----|-------|----------|
| A.19.1 | A Lambda function has LoggingConfig.LogGroup set to "/custom/my-lambda-logs" instead of the default "/aws/lambda/{name}". | When I press Enter on this function, invocations are fetched from "/custom/my-lambda-logs" and displayed normally. The frame title still shows the function name. |
| A.19.2 | I verify the data against CLI using the custom log group. | The invocations match `aws logs filter-log-events --log-group-name /custom/my-lambda-logs --filter-pattern "REPORT RequestId"`. |

---

## B. Lambda Invocation Log Lines View (Level 2)

### B.1 Loading State

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | I press Enter on an invocation in the invocations list. | The log lines view opens. A spinner appears centered in the frame with text like "Fetching log lines..." while the FilterLogEvents API call is in flight. |
| B.1.2 | The API responds successfully. | The spinner disappears. Log lines are rendered as table rows with columns: Timestamp, Message. The frame title shows the Request ID (truncated) and line count. |
| B.1.3 | The API responds with an error (e.g., AccessDenied). | The spinner disappears. A red error flash appears in the header. |

**AWS comparison:**
```
aws logs filter-log-events \
  --log-group-name /aws/lambda/my-function \
  --filter-pattern '"RequestId: a1b2c3d4-e5f6-7890-abcd-ef1234567890"'
```
Expected fields visible: Timestamp, Message

### B.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| B.2.1 | The invocation has no log lines at all (extremely unlikely edge case, perhaps log retention expired). | The frame title shows "invocation-logs(0) -- a1b2c3d4". A centered message indicates no log lines found. |

### B.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| B.3.1 | Log lines are loaded. | Two columns are displayed: "Timestamp" (width 22) and "Message" (width 0, fills remaining terminal width). Column headers are bold blue (#7aa2f7) with no separator line below. |
| B.3.2 | I verify column data against `aws logs filter-log-events` output for the specific RequestId. | "Timestamp" maps to the event timestamp (`.events[].timestamp`). "Message" maps to the log event message (`.events[].message`). All log lines for the RequestId appear as rows. |
| B.3.3 | The Message column width is 0 (fill remaining). | The Message column expands to fill all available horizontal space after the 22-character Timestamp column and frame borders. |
| B.3.4 | A log message is longer than the available Message column width. | The message is truncated at the column boundary. The full message is viewable via the detail view (d key) or word wrap (w key). |

### B.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| B.4.1 | I am viewing 8 log lines for invocation "a1b2c3d4". | The frame title shows: "invocation-logs(8) -- a1b2c3d4", centered in the top border. |
| B.4.2 | A filter is active matching 3 of 8 log lines. | The frame title shows "invocation-logs(3/8) -- a1b2c3d4". |

### B.5 Row Coloring

| ID | Story | Expected |
|----|-------|----------|
| B.5.1 | A log line contains "ERROR" in the message. | The entire row is rendered in RED (#f7768e). |
| B.5.2 | A log line contains "FATAL" in the message. | The entire row is rendered in RED (#f7768e). |
| B.5.3 | A log line contains "Exception" in the message. | The entire row is rendered in RED (#f7768e). |
| B.5.4 | A log line contains "Traceback" in the message (Python stack trace). | The entire row is rendered in RED (#f7768e). |
| B.5.5 | A log line contains "WARN" in the message. | The entire row is rendered in YELLOW (#e0af68). |
| B.5.6 | A log line starts with "REPORT RequestId:". | The entire row is rendered in GREEN (#9ece6a). |
| B.5.7 | A log line starts with "START RequestId:". | The entire row is rendered in DIM (#565f89). |
| B.5.8 | A log line starts with "END RequestId:". | The entire row is rendered in DIM (#565f89). |
| B.5.9 | A log line contains regular application output (no error/warn keywords). | The row is rendered in PLAIN text color (#c0caf5). |
| B.5.10 | I select a row that is colored RED (error line). | The selected row shows full-width blue background (#7aa2f7), overriding the red. |

### B.6 Navigation

| ID | Story | Expected |
|----|-------|----------|
| B.6.1 | I press j/k/g/G/PageUp/PageDown in the log lines list. | Navigation behaves identically to other list views: j moves down, k moves up, g jumps to top, G jumps to bottom, PageUp/PageDown scroll by page. |
| B.6.2 | There are more log lines than fit on screen. I scroll past the visible area. | The table scrolls to keep the selected row visible. Column headers remain in place. |

### B.7 Word Wrap (w)

| ID | Story | Expected |
|----|-------|----------|
| B.7.1 | A log line has a very long message that is truncated. I press w. | Word wrap is toggled on. The long message now wraps to multiple visual lines within the row. The full content is visible without horizontal scrolling. |
| B.7.2 | I press w again. | Word wrap is toggled off. Long messages are truncated again at the column boundary. |
| B.7.3 | A Python stack trace spans many lines, each appearing as separate log entries. With word wrap on. | Each log line wraps independently. The stack trace is readable across multiple rows, with each row wrapping its own content. |

### B.8 Invocation With Minimal Output

| ID | Story | Expected |
|----|-------|----------|
| B.8.1 | An invocation has only START, END, and REPORT lines (no application output). | Three rows are displayed. START and END are DIM, REPORT is GREEN. The frame title shows "invocation-logs(3) -- {requestId}". |
| B.8.2 | I verify against CLI output. | The three lines match `aws logs filter-log-events` output with the RequestId filter. |

### B.9 Invocation With Long Stack Trace

| ID | Story | Expected |
|----|-------|----------|
| B.9.1 | An invocation has a long error stack trace (e.g., 30+ traceback lines). | All stack trace lines appear as separate rows. Each traceback line is colored RED. The list scrolls to accommodate all lines. |
| B.9.2 | I scroll through the stack trace using j/k. | Navigation works line by line through the stack trace. Each line is individually selectable. |
| B.9.3 | I select a stack trace line and press c. | The full message text of that specific traceback line is copied to the clipboard. The "Copied!" flash appears. |

### B.10 Detail Key (d)

| ID | Story | Expected |
|----|-------|----------|
| B.10.1 | I select a log line and press d. | The detail view opens for the selected log line. |
| B.10.2 | I verify the detail fields match views.yaml lambda_invocation_logs detail config. | The detail view shows key-value pairs for: Timestamp, IngestionTime, Message, EventId. |
| B.10.3 | The full message text is visible in the detail view without truncation. | The Message field in the detail view wraps to show the complete text, unlike the truncated list column. |
| B.10.4 | I press Escape on the detail view. | I return to the log lines list. The cursor position is preserved on the same log line. |

**AWS comparison:**
```
aws logs filter-log-events \
  --log-group-name /aws/lambda/my-function \
  --filter-pattern '"RequestId: a1b2c3d4-e5f6-7890-abcd-ef1234567890"'
```
Expected detail fields: Timestamp, IngestionTime, Message, EventId

### B.11 YAML Key (y)

| ID | Story | Expected |
|----|-------|----------|
| B.11.1 | I select a log line and press y. | The YAML view opens. The full log event is rendered as syntax-highlighted YAML. |
| B.11.2 | I press Escape on the YAML view. | I return to the log lines list. |

### B.12 Copy Key (c)

| ID | Story | Expected |
|----|-------|----------|
| B.12.1 | I select a log line and press c. | The full message text of the selected log line is copied to the clipboard (not the timestamp, just the message). A green flash message "Copied!" appears. |
| B.12.2 | I paste from clipboard. | The pasted text matches the full message content exactly, including any leading whitespace. |

### B.13 Filter

| ID | Story | Expected |
|----|-------|----------|
| B.13.1 | I press / and type "ERROR". | Only log lines containing "ERROR" (case-insensitive) in any visible column are shown. The frame title updates with matched/total count. |
| B.13.2 | I press / and type "stripe" to search for specific application output. | Only log lines whose message contains "stripe" are shown. |
| B.13.3 | I press Escape while filter is active. | The filter clears. All log lines reappear. |

### B.14 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| B.14.1 | I press ctrl+r on the log lines view. | The spinner appears. A fresh FilterLogEvents call is made for the same RequestId. The table updates with current data. |

### B.15 Escape (Back to Invocations)

| ID | Story | Expected |
|----|-------|----------|
| B.15.1 | I press Escape on the log lines list (not in filter mode). | I return to the Lambda invocations list. The cursor is on the same invocation I had entered. |

### B.16 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| B.16.1 | I press ? on the log lines list. | The help screen replaces the table content. It displays a four-column layout with categories: LOG LINES, GENERAL, NAVIGATION, HOTKEYS. |
| B.16.2 | The LOG LINES column shows: esc (Back), d (Detail), y (YAML), c (Copy Message), w (Word Wrap). | All log-line-specific key bindings are listed. |
| B.16.3 | I press any key on the help screen. | The help screen closes and the log lines table reappears. |

### B.17 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| B.17.1 | I press : on the log lines list and type "lambda" and press Enter. | The view navigates to the Lambda function list. |

---

## C. Lambda Function Code View (Level 1-alt)

### C.1 Loading State

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | I press s on a Lambda function in the Lambda list. | The Function Code view opens. A spinner appears centered in the frame with text like "Downloading function code..." while the code package is being fetched. The frame title shows "lambda-code -- my-function" (no handler filename yet, since it is not resolved until the zip is downloaded and extracted). |
| C.1.2 | The download completes successfully. | The spinner disappears. The source code is rendered in the viewport with line numbers. The frame title updates to include the handler filename, e.g., "lambda-code -- payment-processor/handler.py". |
| C.1.3 | The download fails (e.g., network error, presigned URL expired). | The spinner disappears. A red error flash appears in the header. An appropriate error message is shown in the content area. |

**AWS comparison:**
```
aws lambda get-function --function-name my-function
# Check Code.Location for presigned URL
# Check Configuration.Handler for handler identification
# Check Configuration.Runtime for filename resolution
# Check Configuration.PackageType for Zip vs Image
# Check Configuration.CodeSize for package size
```

### C.2 Entry Points

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | I press s from the Lambda function list (parent). | The Function Code view opens. The view stack is: Lambda list --> Function Code. |
| C.2.2 | I press s from the Lambda invocations list (Level 1). | The Function Code view opens. The view stack is: Lambda list --> Invocations --> Function Code. |
| C.2.3 | I press Escape from Function Code entered via the Lambda list. | I return to the Lambda function list. |
| C.2.4 | I press Escape from Function Code entered via the invocations list. | I return to the invocations list (not the Lambda function list). |

### C.3 Line Number Rendering

| ID | Story | Expected |
|----|-------|----------|
| C.3.1 | A Python handler file with 17 lines is displayed. | Each line is prefixed with a right-aligned line number (e.g., " 1", " 2", ... "17"), a pipe separator, then the code text. Line numbers are right-aligned to the width of the maximum line number (2 chars for files up to 99 lines). |
| C.3.2 | Line numbers are styled dim (#565f89) and the pipe separator is styled dim (#414868). | Visual inspection confirms the dim styling. The actual code text is in PLAIN color (#c0caf5). |
| C.3.3 | A file with 150 lines is displayed. | Line numbers are right-aligned to 3 characters wide (e.g., "  1", "  2", ... "150"). |
| C.3.4 | No syntax highlighting is applied to the code. | The code text is rendered as plain text. There is no language-specific coloring of keywords, strings, or comments. Line numbers are the primary value add. |

### C.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| C.4.1 | Viewing handler.py for function "payment-processor". | The frame title shows: "lambda-code -- payment-processor/handler.py". |
| C.4.2 | Viewing index.js for function "image-resizer". | The frame title shows: "lambda-code -- image-resizer/index.js". |
| C.4.3 | During loading (before handler filename is resolved). | The frame title shows: "lambda-code -- payment-processor" (no filename). |
| C.4.4 | Handler file not found, showing file listing. | The frame title shows: "lambda-code -- payment-processor (files)". |
| C.4.5 | Viewing a file selected from the fallback file listing. | The frame title shows: "lambda-code -- payment-processor/src/handler.py". |

### C.5 Handler File Resolution

| ID | Story | Expected |
|----|-------|----------|
| C.5.1 | A Python Lambda with Handler "handler.process" and Runtime "python3.12". | The resolved filename is "handler.py". The code view shows the contents of handler.py from the zip. |
| C.5.2 | A Python Lambda with Handler "src/app.lambda_fn" and Runtime "python3.12". | The resolved filename is "src/app.py". The code view shows the contents of src/app.py from the zip. |
| C.5.3 | A Node.js Lambda with Handler "index.handler" and Runtime "nodejs20.x". | The resolved filename is "index.js". The code view shows the contents of index.js from the zip. |
| C.5.4 | A Node.js Lambda with Handler "src/app.handler" and Runtime "nodejs20.x". | The resolved filename is "src/app.js". The code view shows the contents of src/app.js from the zip. |
| C.5.5 | A Ruby Lambda with Handler "handler.process" and Runtime "ruby3.2". | The resolved filename is "handler.rb". |
| C.5.6 | A Go Lambda with Handler "bootstrap" and Runtime "provided.al2023". | The resolved filename is "bootstrap". The file is likely a compiled binary; the code view shows its text content (which may be unreadable). |
| C.5.7 | A Java Lambda with Handler "com.example.Handler::handleRequest". | The resolved filename is "com/example/Handler.java". If this file is not found in the zip (JAR structure differs), the fallback file listing is shown. |

### C.6 Container Image Lambda

| ID | Story | Expected |
|----|-------|----------|
| C.6.1 | I press s on a Lambda function where PackageType is "Image". | The code view does NOT attempt to download anything. A centered message is displayed: "Container image Lambda -- source code not viewable". |
| C.6.2 | Below the main message, I see metadata: "Package type: Image" and "Image URI: 123456789012.dkr.ecr.us-east-1.amazonaws.com/payment:latest". | The package type and image URI are shown in dim labels with plain values. |
| C.6.3 | The main message is styled YELLOW (#e0af68), bold. | Visual inspection confirms the warning styling. |
| C.6.4 | Labels ("Package type:", "Image URI:") are DIM (#565f89). Values are PLAIN (#c0caf5). | Visual inspection confirms. |
| C.6.5 | I press c while viewing the container image message. | Nothing is copied (there is no line to copy), or the Image URI is copied. Either way, no crash occurs. |
| C.6.6 | I press Escape. | I return to the previous view (Lambda list or invocations). |

**AWS comparison:**
```
aws lambda get-function --function-name my-container-function
# Configuration.PackageType will be "Image"
# Code.ImageUri will show the ECR image URI
```

### C.7 Package Too Large (>5MB)

| ID | Story | Expected |
|----|-------|----------|
| C.7.1 | I press s on a Lambda function where CodeSize exceeds 5,242,880 bytes (5 MB). | The code view does NOT attempt to download the zip. A centered message is displayed: "Package too large for inline viewing (23.4 MB)". |
| C.7.2 | Below the main message, I see: "Handler: handler.process", "Runtime: python3.12", "Code size: 23.4 MB (limit: 5 MB)". | Handler, runtime, and code size details are shown. |
| C.7.3 | The main message is styled YELLOW (#e0af68), bold. | Visual inspection confirms the warning styling. |
| C.7.4 | The code size value exceeding the limit is styled RED (#f7768e). | The "23.4 MB" value is rendered in red to emphasize it exceeds the 5 MB limit. |
| C.7.5 | Labels are DIM (#565f89), non-size values are PLAIN (#c0caf5). | Visual inspection confirms. |
| C.7.6 | A Lambda with CodeSize exactly 5,242,880 bytes (5 MB). | The boundary case: the code is either shown (if the limit is exclusive) or the warning is shown (if the limit is inclusive). Behavior must be consistent and documented. |
| C.7.7 | I press Escape. | I return to the previous view. |

**AWS comparison:**
```
aws lambda get-function --function-name my-large-function
# Configuration.CodeSize shows bytes, compare to 5242880 (5 * 1024 * 1024)
```

### C.8 Handler File Not Found (Fallback File Listing)

| ID | Story | Expected |
|----|-------|----------|
| C.8.1 | I press s on a Lambda where the expected handler file (e.g., handler.py) does not exist at the expected path in the zip. | A message is displayed: "Handler file not found: handler.py" followed by "Handler config: handler.process (python3.12)" and a navigable file listing of all files in the deployment package. |
| C.8.2 | The "Handler file not found:" message is styled YELLOW (#e0af68), bold. | Visual inspection confirms. |
| C.8.3 | The file listing is sorted alphabetically. | Files and directories are listed in alphabetical order. |
| C.8.4 | Directory entries (trailing /) are styled DIM (#565f89). File entries are PLAIN (#c0caf5). | Visual inspection confirms the styling distinction. |
| C.8.5 | I navigate the file listing with j/k. | The selection cursor moves between file entries. Directory entries with trailing / are NOT selectable. |
| C.8.6 | I press Enter on a file in the listing (e.g., "src/handler.py"). | The code viewport opens showing the contents of that file. The frame title updates to "lambda-code -- payment-processor/src/handler.py". |
| C.8.7 | I press Escape from the code viewport opened via file listing. | I return to the file listing, NOT to the Lambda list or invocations. |
| C.8.8 | I press Escape from the file listing itself. | I return to the previous view (Lambda list or invocations, depending on entry point). |
| C.8.9 | The frame title while viewing the file listing. | The title shows: "lambda-code -- payment-processor (files)". |

### C.9 Navigation (Code Viewport)

| ID | Story | Expected |
|----|-------|----------|
| C.9.1 | I press j (or down-arrow) in the code viewport. | The viewport scrolls down one line. |
| C.9.2 | I press k (or up-arrow) in the code viewport. | The viewport scrolls up one line. |
| C.9.3 | I press g. | The viewport jumps to the first line of the file. |
| C.9.4 | I press G. | The viewport jumps to the last line of the file. |
| C.9.5 | I press PageDown (or ctrl+d). | The viewport scrolls down by one page. |
| C.9.6 | I press PageUp (or ctrl+u). | The viewport scrolls up by one page. |
| C.9.7 | The file content is shorter than the visible area. | No scrolling occurs. No scroll indicators are shown. |
| C.9.8 | The file content is longer than the visible area. | Scroll indicators appear in dim text (e.g., "X lines above" / "X lines below"). |

### C.10 Word Wrap (w)

| ID | Story | Expected |
|----|-------|----------|
| C.10.1 | A source file has a very long line (common in minified JavaScript or configuration files). I press w. | Word wrap is toggled on. The long line wraps to the next visual line. The line number is only shown on the first visual line of the wrap; continuation lines have blank line number area. |
| C.10.2 | I press w again. | Word wrap is toggled off. Long lines are truncated at the viewport boundary. |

### C.11 Copy Key (c)

| ID | Story | Expected |
|----|-------|----------|
| C.11.1 | I press c while viewing source code with the cursor on line 42. | The text of line 42 is copied to the clipboard WITHOUT the line number prefix or pipe separator. Only the code text (including leading whitespace) is copied. A green "Copied!" flash appears. |
| C.11.2 | I paste from clipboard. | The pasted text is `    raise ValueError(f"Invalid amount: {amount}")` (with leading whitespace preserved), not `42 | raise ValueError(...)`. |

### C.12 Caching and Re-download

| ID | Story | Expected |
|----|-------|----------|
| C.12.1 | I view source code for a function, press Escape, then press s again on the same function. | The code appears instantly without a loading spinner. The source was cached in memory from the first download. |
| C.12.2 | I press ctrl+r while viewing cached source code. | The loading spinner appears. A fresh download is forced (the presigned URL is re-obtained via GetFunction, and the zip is re-downloaded). The viewport updates with potentially new code. |
| C.12.3 | I view source code, navigate to a completely different resource type (e.g., EC2), then return to Lambda and press s on the same function. | The cached source is still available within the same session. No re-download occurs. |

### C.13 Disabled Keys in Code View

| ID | Story | Expected |
|----|-------|----------|
| C.13.1 | I press d while viewing source code. | Nothing happens. The detail view does NOT open. The d key is not active in the code view. |
| C.13.2 | I press y while viewing source code. | Nothing happens. The YAML view does NOT open. |
| C.13.3 | I press / while viewing source code. | Nothing happens. Filter mode does NOT activate. Filtering is not applicable to source code. |
| C.13.4 | I press Enter while viewing source code. | Nothing happens. There is no sub-view to drill into from source code. |
| C.13.5 | I press N, S, or A while viewing source code. | Nothing happens. Sorting is not applicable to source code. |

### C.14 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| C.14.1 | I press ? on the code view. | The help screen replaces the viewport content. It displays a four-column layout with categories: FUNCTION CODE, GENERAL, NAVIGATION, HOTKEYS. |
| C.14.2 | The FUNCTION CODE column shows: esc (Back), c (Copy Line), w (Word Wrap). | All code-view-specific key bindings are listed. Note: d, y, /, enter are absent. |
| C.14.3 | The GENERAL column shows: ctrl-r (Refresh). | Only applicable general keys are listed. Filter and command are not shown (or shown as inactive). |
| C.14.4 | I press any key on the help screen. | The help screen closes and the code viewport reappears. |

### C.15 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| C.15.1 | I press : on the code view. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| C.15.2 | I type "s3" and press Enter. | The view navigates to the S3 bucket list. |

### C.16 Escape (Back)

| ID | Story | Expected |
|----|-------|----------|
| C.16.1 | I press Escape in the code view (entered from Lambda list). | I return to the Lambda function list. The cursor is on the same function. |
| C.16.2 | I press Escape in the code view (entered from invocations list). | I return to the invocations list. The cursor is on the same invocation. |

---

## D. Three-Level Drill-Down: Lambda --> Invocations --> Log Lines

### D.1 Full Navigation Stack

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | Main Menu --> Lambda list --> Invocations (Enter) --> Log Lines (Enter); then Escape three times. | Each Escape pops one level: Log Lines --> Invocations --> Lambda list --> Main Menu. No state is lost at any intermediate level. Cursor positions are preserved at each level. |
| D.1.2 | Main Menu --> Lambda list --> Invocations (Enter) --> Detail (d) --> YAML (y); then Escape three times. | YAML --> Detail --> Invocations --> Lambda list. Each level restores correctly. |
| D.1.3 | Main Menu --> Lambda list --> Invocations (Enter) --> Function Code (s); then Escape twice. | Function Code --> Invocations --> Lambda list. The s key from invocations pushes Function Code onto the stack. |
| D.1.4 | Main Menu --> Lambda list --> Function Code (s); then Escape once. | Function Code --> Lambda list. The view stack is only two levels deep. |
| D.1.5 | Main Menu --> Lambda list --> Invocations (Enter) --> Log Lines (Enter) --> Detail (d); then Escape three times. | Detail --> Log Lines --> Invocations --> Lambda list. The full four-level drill is navigable. |

### D.2 Cross-Level State Preservation

| ID | Story | Expected |
|----|-------|----------|
| D.2.1 | I have a filter active on the invocations list ("ERROR"), drill into a specific invocation's log lines, then press Escape. | I return to the invocations list. The "ERROR" filter is still active. The filtered view is preserved. |
| D.2.2 | I sort the invocations list by status, drill into an invocation, then press Escape. | I return to the invocations list. The sort order (by status) is preserved with the correct indicator. |
| D.2.3 | I select the 15th invocation in a long list, drill into log lines, then press Escape. | I return to the invocations list with the cursor still on the 15th invocation. The scroll position is preserved. |

---

## E. Cross-Cutting Concerns

### E.1 Header Consistency

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | In every Lambda child view (invocations list, log lines, function code), the header displays: "a9s" (accent bold), version (dim), profile:region (bold). | Visual inspection confirms across all Lambda child views. |
| E.1.2 | The header right side shows "? for help" in normal mode across all Lambda child views. | Confirmed in invocations list, log lines, and function code views. |
| E.1.3 | Flash messages ("Copied!", errors) appear in the header right side and auto-clear after approximately 2 seconds in all Lambda child views. | Confirmed across all views. |

### E.2 Terminal Resize

| ID | Story | Expected |
|----|-------|----------|
| E.2.1 | I resize the terminal while viewing the invocations list. | The layout reflows. Column visibility adjusts to the new width. The frame border redraws correctly. |
| E.2.2 | I resize the terminal while viewing the function code viewport. | The viewport adjusts to the new dimensions. Line wrapping (if enabled) reflows to the new width. Line numbers remain aligned. |
| E.2.3 | I resize the terminal to below 60 columns while viewing any Lambda child view. | An error message appears: "Terminal too narrow. Please resize." |
| E.2.4 | I resize the terminal to below 7 lines while viewing any Lambda child view. | An error message appears: "Terminal too short. Please resize." |

### E.3 Alternating Row Colors

| ID | Story | Expected |
|----|-------|----------|
| E.3.1 | The invocations list has more than 2 rows with OK status. | Alternating rows have a subtle background color difference (#1e2030) for readability, in addition to the status-based text color. Selected row always has blue background regardless. |
| E.3.2 | The log lines list has more than 2 rows. | Same alternating row pattern applies, combined with the log-level coloring (RED for errors, DIM for START/END, etc.). |

### E.4 Error Handling

| ID | Story | Expected |
|----|-------|----------|
| E.4.1 | CloudWatch Logs API returns ThrottlingException during invocation fetch. | A red error flash "Error: ThrottlingException" appears in the header. The spinner disappears. The user can retry with ctrl+r. |
| E.4.2 | The Lambda function's log group has been deleted but the function still exists. | When entering invocations, a user-friendly error message appears (e.g., "Log group not found") rather than a raw ResourceNotFoundException. |
| E.4.3 | Network connectivity is lost while downloading function code. | The loading spinner is replaced by an error state. A red flash appears in the header. ctrl+r can retry the download. |
| E.4.4 | The presigned URL from GetFunction has expired (very unlikely in normal use, but possible if cached too long). | An appropriate error message appears. ctrl+r forces a fresh GetFunction call to obtain a new presigned URL. |

### E.5 Performance

| ID | Story | Expected |
|----|-------|----------|
| E.5.1 | The invocations list takes 1-3 seconds to load (CloudWatch Logs latency). | The spinner is visible during the entire wait. The UI remains responsive (Escape can cancel and return to the Lambda list). |
| E.5.2 | The log lines for a specific invocation load quickly (<1 second). | The spinner may appear very briefly or not at all. The transition to the log lines view is smooth. |
| E.5.3 | Downloading a 4.9 MB Lambda package (just under the limit). | The spinner is visible during the download. Once complete, the code renders normally with line numbers. |
