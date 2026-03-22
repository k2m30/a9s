# QA User Stories: CloudWatch Alarm History, SFN Executions, SFN Execution History

Covers three child views across two resource types:
- **Alarm History** (child of CloudWatch Alarms) -- Issue #38
- **SFN Executions** (child of Step Functions) -- Issue #39
- **SFN Execution History** (child of SFN Executions) -- Issue #40

All stories are written from a black-box perspective against the design spec and
`views.yaml` configuration. AWS CLI equivalents are cited so testers can verify data parity.

---

## A. Alarm History (Child of CloudWatch Alarms)

### A.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I select an alarm named "api-error-rate-high" in the CloudWatch Alarms list and press Enter. | The view transitions to the alarm history list. A spinner (animated dot) is displayed centered inside the frame with text like "Fetching alarm history...". The frame title shows "alarm-history" with no count while loading. |
| A.1.2 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. The spinner continues uninterrupted. |
| A.1.3 | The API responds successfully with history data. | The spinner disappears. The table renders with column headers and rows. The frame title updates to "alarm-history(N) --- api-error-rate-high" where N is the total history item count. |
| A.1.4 | The API responds with an error (e.g., alarm not found, no credentials). | The spinner disappears. A red error flash message appears in the header right side. |

**AWS comparison:**
```
aws cloudwatch describe-alarm-history --alarm-name api-error-rate-high
```
Expected fields visible: Timestamp, Type (HistoryItemType), Summary (HistorySummary)

### A.2 Empty State (Newly Created Alarm)

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | I press Enter on an alarm that was just created seconds ago and has no history entries. | The frame title reads "alarm-history(0) --- new-alarm-name". The content area shows a centered message such as "No alarm history found" with a hint to check back later. No column headers are shown (or headers are shown with no data rows). |
| A.2.2 | I press ctrl+r on the empty state. | The loading spinner appears again while the refresh request is in flight. |

**AWS comparison:**
```
aws cloudwatch describe-alarm-history --alarm-name new-alarm-name
# Returns empty AlarmHistoryItems array for a brand-new alarm
```

### A.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | Alarm history loads and the table renders. | Exactly three columns are displayed: "Timestamp" (width 22), "Type" (width 18), "Summary" (width 60). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| A.3.2 | I verify column data against `aws cloudwatch describe-alarm-history --alarm-name ALARM`. | The "Timestamp" column maps to `.AlarmHistoryItems[].Timestamp`. The "Type" column maps to `.AlarmHistoryItems[].HistoryItemType`. The "Summary" column maps to `.AlarmHistoryItems[].HistorySummary`. Every history item returned by the CLI appears as a row in the table. |
| A.3.3 | A HistorySummary text is longer than 60 characters. | The summary is truncated to fit the 60-character column width. No row wrapping occurs. |
| A.3.4 | The terminal is narrower than the combined column widths (22+18+60 = 100 plus borders/padding). | The rightmost column(s) are hidden (not truncated mid-value). Horizontal scroll with h/l is available to reveal hidden columns. |

**AWS comparison:**
```
aws cloudwatch describe-alarm-history --alarm-name api-error-rate-high \
  --query 'AlarmHistoryItems[*].[Timestamp,HistoryItemType,HistorySummary]' --output table
```

### A.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| A.4.1 | 34 history items are loaded for alarm "api-error-rate-high". | The frame top border shows the title centered: "alarm-history(34) --- api-error-rate-high" with equal-length dashes on both sides. |
| A.4.2 | A filter is active and matches 8 of 34 entries. | The frame title reads "alarm-history(8/34) --- api-error-rate-high". |
| A.4.3 | A filter is active and matches 0 entries. | The frame title reads "alarm-history(0/34) --- api-error-rate-high". The content area is empty (no rows). |

### A.5 Row Coloring (State Transitions and Actions)

| ID | Story | Expected |
|----|-------|----------|
| A.5.1 | A history entry has HistoryItemType = "StateUpdate" and HistorySummary contains "to ALARM". | The entire row is rendered in RED (#f7768e). |
| A.5.2 | A history entry has HistoryItemType = "StateUpdate" and HistorySummary contains "to OK". | The entire row is rendered in GREEN (#9ece6a). |
| A.5.3 | A history entry has HistoryItemType = "StateUpdate" and HistorySummary contains "to INSUFFICIENT_DATA". | The entire row is rendered in YELLOW (#e0af68). |
| A.5.4 | A history entry has HistoryItemType = "Action" and HistorySummary contains "Successfully". | The entire row is rendered in PLAIN color (#c0caf5). |
| A.5.5 | A history entry has HistoryItemType = "Action" and HistorySummary contains "Failed". | The entire row is rendered in RED (#f7768e). |
| A.5.6 | A history entry has HistoryItemType = "ConfigurationUpdate". | The entire row is rendered in DIM (#565f89). |
| A.5.7 | I select any row (regardless of its content-based coloring). | The selected row has full-width blue background (#7aa2f7), dark foreground (#1a1b26), bold text. The content-based color is overridden by the selection highlight. |
| A.5.8 | I move selection away from a colored row. | The previously selected row reverts to its content-based coloring (RED, GREEN, YELLOW, DIM, or PLAIN). |

**AWS comparison:**
```
aws cloudwatch describe-alarm-history --alarm-name api-error-rate-high \
  --history-item-type StateUpdate
# Check HistorySummary text for "to ALARM", "to OK", "to INSUFFICIENT_DATA"
```

### A.6 Flapping Alarm (Rapid OK/ALARM/OK Transitions)

| ID | Story | Expected |
|----|-------|----------|
| A.6.1 | An alarm has been flapping with rapid state changes: OK -> ALARM -> OK -> ALARM -> OK within minutes. I view its history. | Multiple entries appear in quick succession with timestamps seconds or minutes apart. StateUpdate rows alternate between RED (to ALARM) and GREEN (to OK). Interleaved Action rows appear (one for each state transition that triggered an SNS notification). |
| A.6.2 | The flapping alarm has 50+ history entries from rapid transitions. | All entries are listed (up to the 30-day retention). Navigation with j/k scrolls through all rows. The frame title shows the full count. |

**AWS comparison:**
```
aws cloudwatch describe-alarm-history --alarm-name flapping-alarm \
  --query 'AlarmHistoryItems | length(@)'
# Should return the total count matching what a9s shows
```

### A.7 Alarm with Action Entries

| ID | Story | Expected |
|----|-------|----------|
| A.7.1 | An alarm transitions to ALARM state and fires an SNS action. | Two consecutive entries appear: a StateUpdate row (RED, "to ALARM") followed by an Action row (PLAIN, "Successfully executed action arn:aws:sns:..."). |
| A.7.2 | An alarm action fails to deliver (e.g., SNS topic deleted). | The Action row appears in RED because the HistorySummary contains "Failed". The Summary text describes the failure reason. |
| A.7.3 | I filter by typing "/" then "action". | Only Action-type entries remain visible. StateUpdate and ConfigurationUpdate entries are hidden. The frame title shows the filtered count. |

**AWS comparison:**
```
aws cloudwatch describe-alarm-history --alarm-name my-alarm \
  --history-item-type Action
```

### A.8 Configuration Update Entries

| ID | Story | Expected |
|----|-------|----------|
| A.8.1 | An alarm had its threshold changed from 80 to 90. | A ConfigurationUpdate entry appears with DIM row coloring. The Summary describes the configuration change. |
| A.8.2 | I view history for an alarm that has only been modified (configured) but never triggered. | All entries are ConfigurationUpdate type, all rendered in DIM. No RED or GREEN rows appear. |

**AWS comparison:**
```
aws cloudwatch describe-alarm-history --alarm-name my-alarm \
  --history-item-type ConfigurationUpdate
```

### A.9 30-Day Retention Boundary

| ID | Story | Expected |
|----|-------|----------|
| A.9.1 | An alarm has been in ALARM state continuously for 45 days. I view its history. | Only history entries from the last 30 days are visible (AWS retention limit). The oldest visible entry may be a StateUpdate "to ALARM" from 30 days ago. Entries older than 30 days are not returned by the API and do not appear. |
| A.9.2 | An alarm was created 10 days ago. I view its history. | All history entries since creation are visible, including the initial configuration creation entry. |

**AWS comparison:**
```
aws cloudwatch describe-alarm-history --alarm-name long-alarm \
  --start-date $(date -v-30d -u +%Y-%m-%dT%H:%M:%SZ) \
  --end-date $(date -u +%Y-%m-%dT%H:%M:%SZ)
# AWS only retains 30 days; older entries are gone
```

### A.10 Navigation

| ID | Story | Expected |
|----|-------|----------|
| A.10.1 | I press j (or down-arrow) with the first history entry selected. | The selection cursor moves to the second entry. The previously selected row loses the blue highlight and reverts to its content-based color. The new row gains the full-width blue background. |
| A.10.2 | I press k (or up-arrow) with the second entry selected. | The selection cursor moves back to the first entry. |
| A.10.3 | I press g. | The selection jumps to the very first (most recent) entry. |
| A.10.4 | I press G. | The selection jumps to the very last (oldest) entry. |
| A.10.5 | I press PageDown. | The selection moves down by one page of visible rows. |
| A.10.6 | I press PageUp. | The selection moves up by one page of visible rows. |
| A.10.7 | There are more entries than fit on screen. I scroll past the visible area. | The table scrolls to keep the selected row visible. The column headers remain in place. |

### A.11 Sorting

| ID | Story | Expected |
|----|-------|----------|
| A.11.1 | I press N on the alarm history list. | Rows are sorted by the first sortable column (Timestamp or Type) in ascending order. The corresponding column header shows a sort indicator (up-arrow). |
| A.11.2 | I press N again. | Sort order toggles to descending. The indicator changes to a down-arrow. |
| A.11.3 | I press A on the alarm history list. | Rows are sorted by Timestamp (age). The "Timestamp" column header shows the sort indicator. |

### A.12 Filter

| ID | Story | Expected |
|----|-------|----------|
| A.12.1 | I press /. | The header right side changes from "? for help" to "/|" (amber/bold, with cursor). Filter mode is active. |
| A.12.2 | I type "StateUpdate" in filter mode. | Only rows whose visible text contains "StateUpdate" (case-insensitive) are displayed. The frame title updates to show matched/total. |
| A.12.3 | I type "ALARM" in filter mode. | Rows containing "ALARM" in any column (Type or Summary) appear. This includes StateUpdate rows transitioning to ALARM as well as any rows mentioning ALARM in the summary. |
| A.12.4 | I press Escape in filter mode. | The filter is cleared. All rows reappear. The frame title reverts to the full count. |

### A.13 Copy (c)

| ID | Story | Expected |
|----|-------|----------|
| A.13.1 | I select a history entry and press c. | The HistorySummary text of the selected entry is copied to the system clipboard. A green flash message "Copied!" appears in the header right side. |
| A.13.2 | After approximately 2 seconds. | The "Copied!" flash message auto-clears and the header right reverts to "? for help". |
| A.13.3 | I paste from clipboard into another application. | The pasted text matches the HistorySummary of the entry I had selected (e.g., "Alarm updated from OK to ALARM"). |

**AWS comparison:**
```
aws cloudwatch describe-alarm-history --alarm-name my-alarm \
  --query 'AlarmHistoryItems[0].HistorySummary' --output text
# The copied text should match this value
```

### A.14 Detail View (d)

| ID | Story | Expected |
|----|-------|----------|
| A.14.1 | I select a history entry and press d. | The detail view opens. The frame title reflects the selected entry context. |
| A.14.2 | I verify the displayed fields. | The detail view shows key-value pairs for: Timestamp, HistoryItemType, HistorySummary, HistoryData, AlarmName, AlarmType. These match the `alarm_history.detail` configuration from the design spec. |
| A.14.3 | I view HistoryData in the detail view. | The HistoryData field contains a JSON string describing the state transition details (old/new state, reason, timestamp). It is displayed as a raw string value. |
| A.14.4 | I press Escape on the detail view. | I return to the alarm history list. The cursor position is preserved on the same entry. |

**AWS comparison:**
```
aws cloudwatch describe-alarm-history --alarm-name my-alarm \
  --query 'AlarmHistoryItems[0].[Timestamp,HistoryItemType,HistorySummary,HistoryData,AlarmName,AlarmType]'
```

### A.15 YAML View (y)

| ID | Story | Expected |
|----|-------|----------|
| A.15.1 | I select a history entry and press y. | The YAML view opens. The full history entry is rendered as syntax-highlighted YAML. Keys are blue, string values green, timestamps green. |
| A.15.2 | I press Escape on the YAML view. | I return to the alarm history list. |

### A.16 Horizontal Scroll

| ID | Story | Expected |
|----|-------|----------|
| A.16.1 | Terminal width is 120+ columns and all three columns (22+18+60=100 plus borders/padding) fit. | All columns are visible. h/l does nothing or is a no-op. |
| A.16.2 | Terminal width is 80 columns and not all columns fit. | Rightmost columns are hidden. Pressing l reveals them while hiding leftmost columns. Pressing h reverses. Column headers scroll in sync with data. |

### A.17 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| A.17.1 | I press ctrl+r on the alarm history list. | The loading spinner appears. A fresh API call is made. When it completes, the table repopulates with current data. |
| A.17.2 | The alarm transitioned state since my last load. I press ctrl+r. | The new StateUpdate and Action entries appear in the refreshed list. The count in the frame title increments. |

### A.18 Escape (Back to Alarms List)

| ID | Story | Expected |
|----|-------|----------|
| A.18.1 | I press Escape on the alarm history list (not in filter or command mode). | I return to the CloudWatch Alarms list. The cursor is on the same alarm I had entered. |

### A.19 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| A.19.1 | I press ? on the alarm history list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: ALARM HISTORY, GENERAL, NAVIGATION, HOTKEYS. |
| A.19.2 | The ALARM HISTORY column contains: `<esc>` Back, `<d>` Detail, `<y>` YAML, `<c>` Copy Summary. | All four entries are present with correct key bindings and descriptions. |
| A.19.3 | The GENERAL column contains: `<ctrl-r>` Refresh, `</>` Filter, `<:>` Command. | All three entries are present. |
| A.19.4 | The NAVIGATION column contains: `<j>` Down, `<k>` Up, `<g>` Top, `<G>` Bottom, `<h/l>` Cols, `<pgup/dn>` Page. | All six entries are present. |
| A.19.5 | The HOTKEYS column contains: `<?>` Help, `<:>` Command. | Both entries are present. |
| A.19.6 | I press any key on the help screen. | The help screen closes and the alarm history table reappears. |

### A.20 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| A.20.1 | I press : on the alarm history list. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| A.20.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list. |
| A.20.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The alarm history list remains. |

---

## B. SFN Executions (Child of Step Functions)

### B.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | I select a STANDARD state machine named "order-processing-workflow" in the Step Functions list and press Enter. | The view transitions to the executions list. A spinner is displayed centered inside the frame with text like "Fetching executions...". The frame title shows "sfn-executions" with no count while loading. |
| B.1.2 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. |
| B.1.3 | The API responds successfully with execution data. | The spinner disappears. The table renders with column headers and rows. The frame title updates to "sfn-executions(N) --- order-processing-workflow" where N is the total execution count. |
| B.1.4 | The API responds with an error (e.g., state machine ARN invalid, no credentials). | The spinner disappears. A red error flash message appears in the header right side. |

**AWS comparison:**
```
aws stepfunctions list-executions \
  --state-machine-arn arn:aws:states:us-east-1:123456789012:stateMachine:order-processing-workflow
```
Expected fields visible: Name, Status, Start Date, Stop Date, Duration

### B.2 EXPRESS State Machine (Not Supported)

| ID | Story | Expected |
|----|-------|----------|
| B.2.1 | I select an EXPRESS state machine in the Step Functions list and press Enter. | An informational message is displayed: "Execution history is not available for Express state machines." No API call to ListExecutions is made. The frame content area shows this message instead of a table. |
| B.2.2 | I press Escape on the EXPRESS message. | I return to the Step Functions list. The cursor is on the same state machine I had selected. |
| B.2.3 | I press ctrl+r on the EXPRESS message. | The message remains unchanged. No API call is made since EXPRESS state machines do not support execution listing. |

**AWS comparison:**
```
aws stepfunctions list-executions \
  --state-machine-arn arn:aws:states:us-east-1:123456789012:stateMachine:express-workflow
# Returns error or empty for EXPRESS type workflows (execution history not retained)
```

### B.3 Empty State (No Executions)

| ID | Story | Expected |
|----|-------|----------|
| B.3.1 | I press Enter on a STANDARD state machine that has never been executed. | The frame title reads "sfn-executions(0) --- idle-workflow". The content area shows a centered message such as "No executions found" with a hint to refresh. |
| B.3.2 | I press ctrl+r on the empty state. | The loading spinner appears again while the refresh request is in flight. |

**AWS comparison:**
```
aws stepfunctions list-executions \
  --state-machine-arn arn:aws:states:us-east-1:123456789012:stateMachine:idle-workflow
# Returns empty Executions array
```

### B.4 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| B.4.1 | Executions load and the table renders. | Five columns are displayed: "Name" (width 36), "Status" (width 12), "Start Date" (width 22), "Stop Date" (width 22), "Duration" (width 12). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| B.4.2 | I verify column data against `aws stepfunctions list-executions`. | "Name" maps to `.Executions[].Name`. "Status" maps to `.Executions[].Status`. "Start Date" maps to `.Executions[].StartDate`. "Stop Date" maps to `.Executions[].StopDate`. "Duration" is computed from StopDate minus StartDate. Every execution returned by the CLI appears as a row in the table. |
| B.4.3 | An execution name is longer than 36 characters. | The name is truncated to fit the 36-character column width. |
| B.4.4 | The terminal is narrower than the combined column widths (36+12+22+22+12=104 plus borders/padding). | The rightmost column(s) are hidden. Horizontal scroll with h/l is available. |

**AWS comparison:**
```
aws stepfunctions list-executions \
  --state-machine-arn arn:aws:states:us-east-1:123456789012:stateMachine:my-workflow \
  --query 'Executions[*].[Name,Status,StartDate,StopDate]' --output table
```

### B.5 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| B.5.1 | 50 executions are loaded for state machine "order-processing-workflow". | The frame top border shows the title centered: "sfn-executions(50) --- order-processing-workflow" with equal-length dashes on both sides. |
| B.5.2 | A filter is active and matches 5 of 50 executions. | The frame title reads "sfn-executions(5/50) --- order-processing-workflow". |
| B.5.3 | A filter is active and matches 0 executions. | The frame title reads "sfn-executions(0/50) --- order-processing-workflow". The content area is empty. |

### B.6 Row Coloring (Execution Status)

| ID | Story | Expected |
|----|-------|----------|
| B.6.1 | An execution has Status = SUCCEEDED. | The entire row is rendered in GREEN (#9ece6a). |
| B.6.2 | An execution has Status = FAILED. | The entire row is rendered in RED (#f7768e). |
| B.6.3 | An execution has Status = TIMED_OUT. | The entire row is rendered in RED (#f7768e). |
| B.6.4 | An execution has Status = ABORTED. | The entire row is rendered in DIM (#565f89). |
| B.6.5 | An execution has Status = RUNNING. | The entire row is rendered in YELLOW (#e0af68). |
| B.6.6 | An execution has Status = PENDING_REDRIVE. | The entire row is rendered in YELLOW (#e0af68). |
| B.6.7 | I select any row (regardless of its status coloring). | The selected row has full-width blue background (#7aa2f7), dark foreground (#1a1b26), bold text. The status-based color is overridden. |
| B.6.8 | I move selection away from a colored row. | The previously selected row reverts to its status-based coloring. |

### B.7 Running Execution (No Stop Date)

| ID | Story | Expected |
|----|-------|----------|
| B.7.1 | An execution is currently RUNNING with no StopDate. | The "Stop Date" column shows a dash or empty value (e.g., "---"). The "Duration" column shows the elapsed time since StartDate as a live or snapshot value (e.g., "5m 23s"). |
| B.7.2 | The RUNNING execution has been active for 2 hours and 15 minutes. | The "Duration" column shows a human-friendly format such as "2h 15m". |
| B.7.3 | I press ctrl+r to refresh. | The Duration value updates to reflect the current elapsed time since the execution is still running. |

**AWS comparison:**
```
aws stepfunctions list-executions \
  --state-machine-arn arn:aws:states:us-east-1:123456789012:stateMachine:my-workflow \
  --status-filter RUNNING
# StopDate will be null for RUNNING executions
```

### B.8 TIMED_OUT Execution

| ID | Story | Expected |
|----|-------|----------|
| B.8.1 | An execution timed out after 1 hour (the state machine timeout was 3600 seconds). | The row is RED. Status column shows "TIMED_OUT". The Duration column shows "1h 0m" or similar. Both Start Date and Stop Date are populated. |

**AWS comparison:**
```
aws stepfunctions list-executions \
  --state-machine-arn arn:aws:states:us-east-1:123456789012:stateMachine:my-workflow \
  --status-filter TIMED_OUT
```

### B.9 ABORTED Execution

| ID | Story | Expected |
|----|-------|----------|
| B.9.1 | An execution was manually aborted by a user. | The row is DIM (#565f89). Status column shows "ABORTED". Both dates and duration are populated. |

**AWS comparison:**
```
aws stepfunctions list-executions \
  --state-machine-arn arn:aws:states:us-east-1:123456789012:stateMachine:my-workflow \
  --status-filter ABORTED
```

### B.10 Duration Display Formats

| ID | Story | Expected |
|----|-------|----------|
| B.10.1 | An execution completed in 47 seconds. | The Duration column shows "47s". |
| B.10.2 | An execution completed in 2 minutes and 47 seconds. | The Duration column shows "2m 47s". |
| B.10.3 | An execution completed in 1 hour, 23 minutes. | The Duration column shows "1h 23m". |
| B.10.4 | An execution completed in under 1 second. | The Duration column shows "0s" or "<1s" or similar human-friendly representation. |

### B.11 Large Number of Executions (Pagination)

| ID | Story | Expected |
|----|-------|----------|
| B.11.1 | A state machine has 500+ executions. I view the executions list. | All executions are loaded (via paginated API calls). The frame title shows the complete count (e.g., "sfn-executions(523) --- busy-workflow"). Scrolling with j/k/G/g navigates through all rows. |
| B.11.2 | I scroll down through hundreds of executions. | Performance remains smooth. The table does not noticeably lag. |

**AWS comparison:**
```
aws stepfunctions list-executions \
  --state-machine-arn arn:aws:states:us-east-1:123456789012:stateMachine:busy-workflow \
  --query 'Executions | length(@)'
# Count should match what a9s displays
```

### B.12 Navigation

| ID | Story | Expected |
|----|-------|----------|
| B.12.1 | I press j (or down-arrow) with the first execution selected. | The selection cursor moves to the second execution. |
| B.12.2 | I press k (or up-arrow) with the second execution selected. | The selection cursor moves back to the first execution. |
| B.12.3 | I press g. | The selection jumps to the first (most recent) execution. |
| B.12.4 | I press G. | The selection jumps to the last (oldest) execution. |
| B.12.5 | I press PageDown. | The selection moves down by one page of visible rows. |
| B.12.6 | I press PageUp. | The selection moves up by one page of visible rows. |

### B.13 Sorting

| ID | Story | Expected |
|----|-------|----------|
| B.13.1 | I press N on the executions list. | Rows are sorted by execution name in ascending order. The "Name" column header shows a sort indicator (up-arrow). |
| B.13.2 | I press N again. | Sort order toggles to descending. The indicator changes to a down-arrow. |
| B.13.3 | I press S on the executions list. | Rows are sorted by Status. The "Status" column header shows the sort indicator. |
| B.13.4 | I press A on the executions list. | Rows are sorted by Start Date (age). The "Start Date" column header shows the sort indicator. |

### B.14 Filter

| ID | Story | Expected |
|----|-------|----------|
| B.14.1 | I press / and type "FAILED". | Only executions containing "FAILED" in any visible column are displayed. The frame title updates to show matched/total. |
| B.14.2 | I press / and type a partial execution name (e.g., "0322"). | Only executions whose name contains "0322" appear. |
| B.14.3 | I press Escape in filter mode. | The filter is cleared. All executions reappear. |

### B.15 Enter Key (Drill Into Execution History)

| ID | Story | Expected |
|----|-------|----------|
| B.15.1 | I select a SUCCEEDED execution and press Enter. | The view transitions to the Execution History for that execution. A loading spinner appears while the history is fetched. The executions list is pushed onto the view stack. |
| B.15.2 | I select a FAILED execution and press Enter. | The Execution History view opens, showing all step-by-step events including the failure event. |
| B.15.3 | I select a RUNNING execution and press Enter. | The Execution History view opens, showing events up to the current point in execution. |

### B.16 Copy (c)

| ID | Story | Expected |
|----|-------|----------|
| B.16.1 | I select an execution and press c. | The execution ARN is copied to the system clipboard. A green flash message "Copied!" appears in the header right side. |
| B.16.2 | I paste from clipboard into another application. | The pasted text is a valid execution ARN (e.g., "arn:aws:states:us-east-1:123456789012:execution:order-processing-workflow:exec-2026-0322-0315-a1b2c3d4"). |

**AWS comparison:**
```
aws stepfunctions list-executions \
  --state-machine-arn arn:aws:states:us-east-1:123456789012:stateMachine:my-workflow \
  --query 'Executions[0].ExecutionArn' --output text
# The copied text should match this value
```

### B.17 Detail View (d)

| ID | Story | Expected |
|----|-------|----------|
| B.17.1 | I select an execution and press d. | The detail view opens. The frame title reflects the execution name. |
| B.17.2 | I verify the displayed fields. | The detail view shows key-value pairs for: ExecutionArn, Name, Status, StartDate, StopDate, StateMachineArn, StateMachineAliasArn, StateMachineVersionArn, MapRunArn, ItemCount, RedriveCount, RedriveDate, RedriveStatus, RedriveStatusReason. These match the `sfn_executions.detail` configuration from the design spec. |
| B.17.3 | The execution has no MapRunArn (not a Map state run). | The MapRunArn field shows null or empty. |
| B.17.4 | The execution has been redriven (RedriveCount > 0). | RedriveCount, RedriveDate, RedriveStatus, and RedriveStatusReason all show their respective values. |
| B.17.5 | I press Escape on the detail view. | I return to the executions list. The cursor position is preserved. |

**AWS comparison:**
```
aws stepfunctions describe-execution \
  --execution-arn arn:aws:states:us-east-1:123456789012:execution:my-workflow:my-execution
```

### B.18 YAML View (y)

| ID | Story | Expected |
|----|-------|----------|
| B.18.1 | I select an execution and press y. | The YAML view opens showing the full execution metadata as syntax-highlighted YAML. |
| B.18.2 | I press Escape on the YAML view. | I return to the executions list. |

### B.19 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| B.19.1 | I press ctrl+r on the executions list. | The loading spinner appears. A fresh ListExecutions call is made. The table repopulates. |
| B.19.2 | A new execution started since my last load. I press ctrl+r. | The new execution appears (likely as a RUNNING row at the top if sorted by most recent). The count increments. |

### B.20 Escape (Back to Step Functions List)

| ID | Story | Expected |
|----|-------|----------|
| B.20.1 | I press Escape on the executions list (not in filter or command mode). | I return to the Step Functions list. The cursor is on the same state machine I had entered. |

### B.21 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| B.21.1 | I press ? on the executions list. | The help screen displays a four-column layout with categories: EXECUTIONS, GENERAL, NAVIGATION, HOTKEYS. |
| B.21.2 | The EXECUTIONS column contains: `<esc>` Back, `<enter>` View History, `<d>` Detail, `<y>` YAML, `<c>` Copy ARN. | All five entries are present with correct key bindings and descriptions. |
| B.21.3 | The GENERAL column contains: `<ctrl-r>` Refresh, `<q>` Quit, `</>` Filter, `<:>` Command. | All four entries are present. |
| B.21.4 | The NAVIGATION column contains: `<j>` Down, `<k>` Up, `<g>` Top, `<G>` Bottom, `<h/l>` Cols, `<pgup/dn>` Page. | All six entries are present. |
| B.21.5 | The HOTKEYS column contains: `<?>` Help, `<:>` Command. | Both entries are present. |
| B.21.6 | I press any key on the help screen. | The help screen closes and the executions table reappears. |

### B.22 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| B.22.1 | I press : on the executions list. | The header right side changes to ":|" (amber/bold). |
| B.22.2 | I type "s3" and press Enter. | The view navigates to the S3 bucket list. |
| B.22.3 | I press Escape in command mode. | Command mode is cancelled. The executions list remains. |

### B.23 Horizontal Scroll

| ID | Story | Expected |
|----|-------|----------|
| B.23.1 | Terminal width is 120+ columns and all five columns fit. | All columns visible. h/l does nothing. |
| B.23.2 | Terminal width is 80 columns. | Rightmost columns (Stop Date, Duration) are hidden. Pressing l reveals them. Pressing h reverses. Headers scroll in sync. |

---

## C. SFN Execution History (Child of SFN Executions)

### C.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | I select an execution named "exec-2026-0322-0115-c9d0e1f2" in the executions list and press Enter. | The view transitions to the execution history list. A spinner is displayed with text like "Fetching execution history...". |
| C.1.2 | I press keys (j, k) while the spinner is visible. | No navigation occurs. Keypresses are ignored until data loads. |
| C.1.3 | The API responds successfully with history events. | The spinner disappears. The table renders with column headers and rows. The frame title updates to "sfn-history(N) --- exec-2026-0322-0115-c9d0e1f2" where N is the total event count. |
| C.1.4 | The API responds with an error (e.g., execution ARN not found). | The spinner disappears. A red error flash message appears in the header right side. |

**AWS comparison:**
```
aws stepfunctions get-execution-history \
  --execution-arn arn:aws:states:us-east-1:123456789012:execution:my-workflow:exec-2026-0322-0115-c9d0e1f2
```
Expected fields visible: Timestamp, Event Type, State, Detail

### C.2 EXPRESS Execution History (Not Supported)

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | I attempt to view execution history for an execution that belongs to an EXPRESS state machine. | An informational message is displayed indicating that execution history is not available for Express state machines. No API call to GetExecutionHistory is made. |

**AWS comparison:**
```
aws stepfunctions get-execution-history \
  --execution-arn arn:aws:states:us-east-1:123456789012:express:my-express-wf:exec-id:attempt-id
# Returns error: execution history not available for EXPRESS
```

### C.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| C.3.1 | Execution history loads and the table renders. | Four columns are displayed: "Timestamp" (width 22), "Event Type" (width 24), "State" (width 24), "Detail" (width 40). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| C.3.2 | I verify the "Event Type" column is a human-friendly transformation. | Event types are displayed in readable form (e.g., "Task Failed" instead of "TaskFailed", "State Entered" instead of "TaskStateEntered", "Execution Started" instead of "ExecutionStarted"). |
| C.3.3 | I verify the "State" column shows the step/state name. | For StateEntered and StateExited events, the state name is shown (e.g., "ValidateOrder", "ProcessPayment"). For execution-level events (ExecutionStarted, ExecutionFailed), the state column shows a dash "---". |
| C.3.4 | I verify the "Detail" column shows relevant context. | For failed events, the detail shows the error message and/or cause. For other events, it shows input/output summary text. Detail text is truncated to 40 characters if longer. |
| C.3.5 | The terminal is narrower than the combined column widths (22+24+24+40=110 plus borders/padding). | Rightmost columns are hidden. h/l horizontal scroll is available. |

**AWS comparison:**
```
aws stepfunctions get-execution-history \
  --execution-arn arn:aws:states:us-east-1:123456789012:execution:my-workflow:my-exec \
  --query 'events[*].[timestamp,type,stateEnteredEventDetails.name,taskFailedEventDetails.error]' \
  --output table
```

### C.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| C.4.1 | 24 history events are loaded for execution "exec-2026-0322-0115-c9d0e1f2". | The frame top border shows: "sfn-history(24) --- exec-2026-0322-0115-c9d0e1f2" centered with equal dashes. |
| C.4.2 | A filter is active and matches 4 of 24 events. | The frame title reads "sfn-history(4/24) --- exec-2026-0322-0115-c9d0e1f2". |
| C.4.3 | A filter is active and matches 0 events. | The frame title reads "sfn-history(0/24) --- exec-2026-0322-0115-c9d0e1f2". |

### C.5 Row Coloring (Event Types)

| ID | Story | Expected |
|----|-------|----------|
| C.5.1 | An event type ends with "Succeeded" or is "StateExited" (normal flow). | The entire row is rendered in GREEN (#9ece6a). Examples: TaskSucceeded, LambdaFunctionSucceeded, ActivitySucceeded, ExecutionSucceeded, StateExited. |
| C.5.2 | An event type ends with "Failed" or "TimedOut", or is "ExecutionAborted". | The entire row is rendered in RED (#f7768e). Examples: TaskFailed, LambdaFunctionFailed, ExecutionFailed, TaskTimedOut, ExecutionTimedOut, ExecutionAborted. |
| C.5.3 | An event type ends with "Scheduled" or "Started", or is "StateEntered". | The entire row is rendered in YELLOW (#e0af68). Examples: TaskScheduled, TaskStarted, LambdaFunctionScheduled, StateEntered, MapRunStarted. |
| C.5.4 | The event type is "ExecutionStarted". | The entire row is rendered in PLAIN color (#c0caf5), not YELLOW. |
| C.5.5 | I select any row. | The selected row has full-width blue background (#7aa2f7), dark foreground (#1a1b26), bold. The event-type coloring is overridden. |
| C.5.6 | I move selection away from a colored row. | The previously selected row reverts to its event-type coloring. |

### C.6 Failed Execution with Error and Cause

| ID | Story | Expected |
|----|-------|----------|
| C.6.1 | An execution failed at a Task step. I view the execution history. | The history shows events in chronological order. Near the end, a TaskFailed event (RED) shows the error in the "Detail" column (e.g., "PaymentProcessingError: Card declined"). |
| C.6.2 | The TaskFailed event has both Error and Cause fields. | The "Detail" column shows a combined summary of the error and cause, truncated to fit the column width. |
| C.6.3 | I press d on the TaskFailed event. | The detail view opens showing all the TaskFailedEventDetails fields including the full Error string and the full Cause string (which may be a long JSON payload). |
| C.6.4 | I press c on the TaskFailed event. | The error detail text (error/cause) is copied to the clipboard. |

**AWS comparison:**
```
aws stepfunctions get-execution-history \
  --execution-arn arn:aws:states:us-east-1:123456789012:execution:my-workflow:failed-exec \
  --query 'events[?type==`TaskFailed`].[taskFailedEventDetails.error,taskFailedEventDetails.cause]'
```

### C.7 Execution with Map State (Parallel Branches)

| ID | Story | Expected |
|----|-------|----------|
| C.7.1 | An execution included a Map state that spawned parallel branches. | The history includes MapRunStarted events (YELLOW) along with events from individual branches. The event count may be large (hundreds of events for complex maps). |
| C.7.2 | A Map run failed. | A MapRunFailed event (RED) appears in the history. The Detail column shows the failure reason. |

**AWS comparison:**
```
aws stepfunctions get-execution-history \
  --execution-arn arn:aws:states:us-east-1:123456789012:execution:my-workflow:map-exec \
  --query 'events[?contains(type, `MapRun`)]'
```

### C.8 Execution with Retry Attempts

| ID | Story | Expected |
|----|-------|----------|
| C.8.1 | A step was configured with a Retry policy and failed, then retried successfully. | The history shows: TaskScheduled (YELLOW) -> TaskStarted (YELLOW) -> TaskFailed (RED) -> TaskScheduled (YELLOW, retry) -> TaskStarted (YELLOW) -> TaskSucceeded (GREEN). The repeated sequence of scheduled/started/failed entries shows the retry pattern. |
| C.8.2 | A step exhausted all retries and permanently failed. | The history shows multiple TaskFailed events (all RED) followed by an ExecutionFailed event (RED). |

### C.9 Execution History for a Succeeded Execution

| ID | Story | Expected |
|----|-------|----------|
| C.9.1 | I view history for a simple, successful 3-step execution. | Events appear in chronological order (oldest first): ExecutionStarted (PLAIN), then for each step: StateEntered (YELLOW), TaskScheduled (YELLOW), TaskStarted (YELLOW), TaskSucceeded (GREEN), StateExited (GREEN). Finally ExecutionSucceeded (GREEN). |
| C.9.2 | The total event count reflects all events. | For a 3-step workflow, the count might be 16-20 events (execution start + per-step events + execution succeeded). |

### C.10 Navigation

| ID | Story | Expected |
|----|-------|----------|
| C.10.1 | I press j (or down-arrow) with the first event selected. | The selection moves to the second event. |
| C.10.2 | I press k (or up-arrow). | The selection moves up one event. |
| C.10.3 | I press g. | The selection jumps to the first event (ExecutionStarted). |
| C.10.4 | I press G. | The selection jumps to the last event (ExecutionSucceeded or ExecutionFailed). |
| C.10.5 | I press PageDown/PageUp. | The selection moves by one page of visible rows. |
| C.10.6 | There are 100+ events. I scroll through them. | The table scrolls smoothly. Column headers remain in place. |

### C.11 Filter

| ID | Story | Expected |
|----|-------|----------|
| C.11.1 | I press / and type "Failed". | Only rows containing "Failed" in any visible column are displayed (e.g., TaskFailed events, LambdaFunctionFailed events). |
| C.11.2 | I press / and type a state name (e.g., "ValidateOrder"). | Only events related to that specific state appear. |
| C.11.3 | I press Escape in filter mode. | The filter clears. All events reappear. |

### C.12 Copy (c)

| ID | Story | Expected |
|----|-------|----------|
| C.12.1 | I select a TaskFailed event and press c. | The event detail text (error/cause for failures) is copied to the system clipboard. A green flash "Copied!" appears. |
| C.12.2 | I select a StateEntered event and press c. | The event detail text (input/output summary for non-failure events) is copied to the clipboard. |
| C.12.3 | I paste from clipboard. | The pasted text matches the Detail column content (or the full detail text if the column was truncated). |

**AWS comparison:**
```
aws stepfunctions get-execution-history \
  --execution-arn arn:aws:states:us-east-1:123456789012:execution:my-workflow:my-exec \
  --query 'events[?type==`TaskFailed`].taskFailedEventDetails' --output json
# Error + Cause fields should match what was copied
```

### C.13 Detail View (d)

| ID | Story | Expected |
|----|-------|----------|
| C.13.1 | I select a history event and press d. | The detail view opens showing all fields for that event. |
| C.13.2 | I verify the detail fields for a TaskFailed event. | The detail view shows: Timestamp, Type, Id, PreviousEventId, and TaskFailedEventDetails (with nested Error and Cause fields). Other event detail fields (ActivityFailed, LambdaFunctionFailed, etc.) show null or are omitted. |
| C.13.3 | I verify the detail fields for an ExecutionStarted event. | The detail view shows: Timestamp, Type, Id, PreviousEventId, and ExecutionStartedEventDetails (with input and roleArn). |
| C.13.4 | The detail content is long (e.g., a large JSON Cause string). | I can scroll with j/k/g/G within the detail view. Scroll indicators appear when content extends beyond the visible area. |
| C.13.5 | I press w in the detail view. | Word wrap is toggled. Long values that extended beyond the visible width now wrap. |
| C.13.6 | I press Escape on the detail view. | I return to the execution history list. Cursor position is preserved. |

**AWS comparison:**
```
aws stepfunctions get-execution-history \
  --execution-arn arn:aws:states:us-east-1:123456789012:execution:my-workflow:my-exec \
  --query 'events[0]'
# All fields from the event should be visible in the detail view
```

### C.14 YAML View (y)

| ID | Story | Expected |
|----|-------|----------|
| C.14.1 | I select a history event and press y. | The YAML view opens showing the full event as syntax-highlighted YAML. |
| C.14.2 | The YAML for a TaskFailed event includes nested structures. | The nested TaskFailedEventDetails with Error and Cause are rendered with proper YAML indentation and color coding. |
| C.14.3 | I press Escape on the YAML view. | I return to the execution history list. |

### C.15 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| C.15.1 | I press ctrl+r on the execution history list. | The spinner appears. A fresh GetExecutionHistory call is made. The table repopulates. |
| C.15.2 | The execution is RUNNING and new events occurred. I press ctrl+r. | New events appear at the bottom of the list (chronological order). The count increments. |

### C.16 Escape (Back to Executions List)

| ID | Story | Expected |
|----|-------|----------|
| C.16.1 | I press Escape on the execution history list (not in filter or command mode). | I return to the SFN Executions list. The cursor is on the same execution I had entered. |

### C.17 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| C.17.1 | I press ? on the execution history list. | The help screen displays a four-column layout with categories: EXECUTION HISTORY, GENERAL, NAVIGATION, HOTKEYS. |
| C.17.2 | The EXECUTION HISTORY column contains: `<esc>` Back, `<d>` Detail, `<y>` YAML, `<c>` Copy Detail. | All four entries are present with correct key bindings and descriptions. |
| C.17.3 | The GENERAL column contains: `<ctrl-r>` Refresh, `</>` Filter, `<:>` Command. | All three entries are present. |
| C.17.4 | The NAVIGATION column contains: `<j>` Down, `<k>` Up, `<g>` Top, `<G>` Bottom, `<h/l>` Cols, `<pgup/dn>` Page. | All six entries are present. |
| C.17.5 | The HOTKEYS column contains: `<?>` Help, `<:>` Command. | Both entries are present. |
| C.17.6 | I press any key on the help screen. | The help screen closes and the execution history table reappears. |

### C.18 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| C.18.1 | I press : on the execution history list. | The header right side changes to ":|" (amber/bold). |
| C.18.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list. |
| C.18.3 | I press Escape in command mode. | Command mode cancelled. Execution history list remains. |

### C.19 Horizontal Scroll

| ID | Story | Expected |
|----|-------|----------|
| C.19.1 | Terminal width is 120+ columns and all four columns fit. | All columns visible. h/l does nothing. |
| C.19.2 | Terminal width is 80 columns. | Rightmost columns (Detail) are hidden. Pressing l reveals them. Pressing h reverses. Headers scroll in sync. |

---

## D. Three-Level Drill-Down: Step Functions -> Executions -> History

### D.1 Full Navigation Stack

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | Main Menu -> Step Functions -> state machine (Enter) -> Executions -> execution (Enter) -> Execution History; then Escape three times. | Each Escape pops one level: Execution History -> Executions -> Step Functions -> Main Menu. No state is lost at any intermediate level. |
| D.1.2 | I drill from Step Functions to Executions to Execution History. At Execution History, I press d to view event detail, then y to switch to YAML. I press Escape four times. | YAML -> Detail -> Execution History -> Executions -> Step Functions. Cursor position is preserved at each list level. |
| D.1.3 | I am in Execution History and press : then type "s3" and Enter. | I navigate directly to S3 buckets list, bypassing the Executions and Step Functions levels. The entire drill-down stack is replaced by the new navigation. |

### D.2 Context Preservation Across Levels

| ID | Story | Expected |
|----|-------|----------|
| D.2.1 | I select the 5th state machine, drill into its executions, select the 3rd execution, drill into its history. I press Escape twice. | I return to the Executions list with the 3rd execution still selected. I press Escape again and return to Step Functions with the 5th state machine still selected. |
| D.2.2 | I filter executions to show only "FAILED", select one, drill into its history. I press Escape. | I return to the Executions list. The "FAILED" filter is still active with the same filtered results visible. |

### D.3 Mixed Detail/YAML Across Levels

| ID | Story | Expected |
|----|-------|----------|
| D.3.1 | In Execution History, I press d on a TaskFailed event. I see the detail with full Error and Cause text. I press Escape, then press Escape again to go to Executions, press d on the FAILED execution. | The Execution detail shows: ExecutionArn, Name, Status=FAILED, StartDate, StopDate, and other execution-level fields. This is different from the event-level detail I saw in the history. |
| D.3.2 | In Execution History, I press y on an event. The YAML shows event-level fields (Timestamp, Type, event details). I press Escape to history, Escape to executions, press y on the execution. | The YAML shows execution-level fields (ExecutionArn, Name, Status, dates). This is the execution metadata YAML, not the event YAML. |

---

## E. Cross-Cutting Concerns

### E.1 Header Consistency

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | In every child view (alarm history, sfn executions, sfn execution history), the header displays: "a9s" (accent bold), version (dim), profile:region (bold). | Visual inspection confirms across all three child views. |
| E.1.2 | The header right side shows "? for help" in normal mode across all three child views. | Confirmed in alarm history, sfn executions, and sfn execution history. |

### E.2 Terminal Resize

| ID | Story | Expected |
|----|-------|----------|
| E.2.1 | I resize the terminal while viewing the alarm history list. | The layout reflows. Column visibility adjusts to the new width. The frame border redraws correctly. |
| E.2.2 | I resize the terminal while viewing the sfn executions list. | Same reflow behavior. Columns hidden/revealed based on new width. |
| E.2.3 | I resize the terminal while viewing the sfn execution history. | Same reflow behavior. |
| E.2.4 | I resize the terminal to below 60 columns while in any child view. | An error message appears: "Terminal too narrow. Please resize." |
| E.2.5 | I resize the terminal to below 7 lines while in any child view. | An error message appears: "Terminal too short. Please resize." |

### E.3 Alternating Row Colors

| ID | Story | Expected |
|----|-------|----------|
| E.3.1 | The alarm history list has more than 2 rows. | Alternating rows have a subtle background color difference (#1e2030) for readability. Selected row always has blue background regardless. Content-based row coloring (RED, GREEN, DIM, YELLOW, PLAIN) is applied on top of the alternating pattern. |
| E.3.2 | The sfn executions list has more than 2 rows. | Same alternating row pattern applies with status-based coloring. |
| E.3.3 | The sfn execution history has more than 2 rows. | Same alternating row pattern applies with event-type coloring. |

### E.4 Profile and Region Switch

| ID | Story | Expected |
|----|-------|----------|
| E.4.1 | I am viewing alarm history and switch profile via : -> ctx. | After profile switch, I return to the main menu (or the alarms list reloads). The alarm history data from the previous profile is no longer visible. |
| E.4.2 | I am viewing sfn executions and switch region via : -> region. | After region switch, the Step Functions list reloads for the new region. The previous execution data is no longer visible. |

### E.5 Error Handling

| ID | Story | Expected |
|----|-------|----------|
| E.5.1 | The DescribeAlarmHistory API call fails with AccessDenied. | A red error flash "Error: AccessDenied" appears in the header right side. The frame content shows an empty or error state. |
| E.5.2 | The ListExecutions API call fails with a network timeout. | A red error flash with the timeout message appears. The spinner stops. |
| E.5.3 | The GetExecutionHistory API call fails because the execution was deleted. | A red error flash appears. The user can press Escape to return to the executions list. |
