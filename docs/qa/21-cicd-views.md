# QA User Stories: CI/CD Child Views

Covers CodeBuild Builds (child of CodeBuild Projects), Build Logs (child of Builds,
cross-service to CloudWatch), and Pipeline Stages (child of CodePipeline).
All stories are written from a black-box perspective against the design spec and
`views.yaml` configuration files.

AWS CLI equivalents are cited so testers can verify data parity.

---

## A. CodeBuild Builds List View (Level 1 Child)

### A.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I select a CodeBuild project "payment-api-build" from the CodeBuild list and press Enter. | The view transitions to the builds list. A spinner appears centered in the frame with text like "Fetching builds..." while the two-call API chain (ListBuildsForProject then BatchGetBuilds) is in flight. The frame title shows "cb-builds" with no count during loading. The header shows "? for help" on the right. |
| A.1.2 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. The spinner continues uninterrupted. |
| A.1.3 | The API responds successfully with build data. | The spinner disappears. The table renders with column headers and rows. The frame title updates to "cb-builds(25) -- payment-api-build" where 25 is the total build count. |
| A.1.4 | The API responds with an error (e.g., AccessDeniedException, project not found). | The spinner disappears. A red error flash message appears in the header right side. The frame content area shows an appropriate empty or error state. |

**AWS comparison:**
```
aws codebuild list-builds-for-project --project-name payment-api-build
aws codebuild batch-get-builds --ids <build-id-1> <build-id-2> ...
```
Expected fields visible: Build #, Status, Start Time, Duration, Source Version, Initiator

### A.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | I press Enter on a newly created CodeBuild project that has never been built. | The frame title reads "cb-builds(0) -- my-new-project". The content area shows a centered message (e.g., "No builds found") with a hint to refresh. No column headers are shown (or headers are shown with no data rows). |
| A.2.2 | I press ctrl+r on the empty state. | The loading spinner appears again while the refresh request is in flight. |

**AWS comparison:**
```
aws codebuild list-builds-for-project --project-name my-new-project
```
Returns empty `ids` list.

### A.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | Builds load and the table renders. | Six columns are displayed: "Build #" (width 10), "Status" (width 14), "Start Time" (width 22), "Duration" (width 12), "Source Version" (width 14), "Initiator" (width 24). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| A.3.2 | I verify column data against the AWS CLI output. | "Build #" maps to `BuildNumber`. "Status" maps to `BuildStatus`. "Start Time" maps to `StartTime`. "Duration" is computed from `EndTime - StartTime` and displayed as human-friendly (e.g., "3m 22s", "12m 45s"). "Source Version" shows the first 8 characters of the commit SHA from `SourceVersion` or `ResolvedSourceVersion` (e.g., "a1b2c3d4"). "Initiator" maps to `Initiator`. |
| A.3.3 | An initiator string is longer than 24 characters (e.g., "codepipeline/my-very-long-pipeline-name"). | The initiator is truncated to fit the 24-character column width. No row wrapping occurs. |
| A.3.4 | The terminal is narrower than the combined column widths (10+14+22+12+14+24 = 96 plus borders/padding). | The rightmost column(s) are hidden (not truncated mid-value). Horizontal scroll with h/l is available to reveal hidden columns. |

**AWS comparison:**
```
aws codebuild batch-get-builds --ids <ids> --query 'builds[].{BuildNumber:buildNumber,BuildStatus:buildStatus,StartTime:startTime,EndTime:endTime,SourceVersion:sourceVersion,Initiator:initiator}'
```
Expected fields visible: Build #, Status, Start Time, Duration, Source Version, Initiator

### A.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| A.4.1 | 25 builds are loaded for project "payment-api-build". | The frame top border shows the title centered: "cb-builds(25) -- payment-api-build" with equal-length dashes on both sides. |
| A.4.2 | A filter is active and matches 3 of 25 builds. | The frame title reads "cb-builds(3/25) -- payment-api-build". |
| A.4.3 | A filter is active and matches 0 builds. | The frame title reads "cb-builds(0/25) -- payment-api-build". The content area is empty (no rows). |

### A.5 Row Coloring by Build Status

| ID | Story | Expected |
|----|-------|----------|
| A.5.1 | A build has status SUCCEEDED. | The entire row is rendered in green (#9ece6a). |
| A.5.2 | A build has status FAILED. | The entire row is rendered in red (#f7768e). |
| A.5.3 | A build has status FAULT. | The entire row is rendered in red (#f7768e), same as FAILED. |
| A.5.4 | A build has status TIMED_OUT. | The entire row is rendered in red (#f7768e), same as FAILED. |
| A.5.5 | A build has status IN_PROGRESS. | The entire row is rendered in yellow (#e0af68). |
| A.5.6 | A build has status STOPPED. | The entire row is rendered in dim (#565f89). |
| A.5.7 | I select any row regardless of its status color. | The selected row has full-width blue background (#7aa2f7) with dark foreground (#1a1b26), bold text. The status color is overridden. |
| A.5.8 | I move selection away from a colored row. | The previously selected row reverts to its status-based coloring. |

**AWS comparison:**
```
aws codebuild batch-get-builds --ids <ids> --query 'builds[].buildStatus'
```
Each returned status value determines the row color.

### A.6 IN_PROGRESS Build (Edge Case)

| ID | Story | Expected |
|----|-------|----------|
| A.6.1 | A build is IN_PROGRESS (no EndTime yet). | The Duration column shows elapsed time since StartTime (e.g., "2m 47s"), not blank or error. The row is colored yellow (#e0af68). |
| A.6.2 | I press ctrl+r while a build is IN_PROGRESS. | The data refreshes. The Duration column updates to reflect the new elapsed time. If the build has since completed, the status and duration update accordingly. |

**AWS comparison:**
```
aws codebuild batch-get-builds --ids <in-progress-build-id> --query 'builds[0].{Status:buildStatus,StartTime:startTime,EndTime:endTime}'
```
EndTime is null for IN_PROGRESS builds.

### A.7 STOPPED Build (Edge Case)

| ID | Story | Expected |
|----|-------|----------|
| A.7.1 | A build was manually stopped mid-way. | The Status column shows "STOPPED". The Duration column shows the elapsed time from StartTime to the point it was stopped (e.g., "0m 45s"). The entire row is rendered dim (#565f89). |

**AWS comparison:**
```
aws codebuild stop-build --id <build-id>
aws codebuild batch-get-builds --ids <build-id> --query 'builds[0].buildStatus'
```
Returns "STOPPED".

### A.8 Build with No Source Version (Edge Case)

| ID | Story | Expected |
|----|-------|----------|
| A.8.1 | A build was triggered manually without specifying a source version. | The "Source Version" column displays an empty value, a dash, or is blank -- it does not crash or show an error. All other columns render normally. |

**AWS comparison:**
```
aws codebuild start-build --project-name my-project
aws codebuild batch-get-builds --ids <build-id> --query 'builds[0].sourceVersion'
```
Returns null when no source version was specified.

### A.9 Build Initiator Variants (Edge Case)

| ID | Story | Expected |
|----|-------|----------|
| A.9.1 | A build was triggered by CodePipeline. | The Initiator column shows the pipeline reference (e.g., "codepipeline/my-pipeline"). |
| A.9.2 | A build was triggered manually by an IAM user. | The Initiator column shows the user identity (e.g., "user/john.doe"). |
| A.9.3 | A build was triggered by a webhook (e.g., GitHub push). | The Initiator column shows the webhook/trigger information. |

**AWS comparison:**
```
aws codebuild batch-get-builds --ids <build-id> --query 'builds[0].initiator'
```
Expected values: "codepipeline/<name>", "<user-identity>", or webhook identifier.

### A.10 Navigation

| ID | Story | Expected |
|----|-------|----------|
| A.10.1 | I press j (or down-arrow) with the first build selected. | The selection cursor moves to the second build. The previously selected row loses the blue highlight and reverts to its status color. The new row gains the full-width blue background. |
| A.10.2 | I press k (or up-arrow) with the second build selected. | The selection cursor moves back to the first build. |
| A.10.3 | I press g. | The selection jumps to the very first build in the list. |
| A.10.4 | I press G. | The selection jumps to the very last build in the list. |
| A.10.5 | I press PageDown. | The selection moves down by one page of visible rows. If fewer rows remain below than a page, the cursor lands on the last row. |
| A.10.6 | I press PageUp. | The selection moves up by one page of visible rows. If fewer rows remain above than a page, the cursor lands on the first row. |
| A.10.7 | There are more builds than fit on screen. I scroll past the visible area. | The table scrolls to keep the selected row visible. The column headers remain in place. |
| A.10.8 | I press h (or left-arrow). | Columns scroll left (visible column window shifts), revealing any previously hidden left columns. |
| A.10.9 | I press l (or right-arrow). | Columns scroll right (visible column window shifts), revealing any previously hidden right columns. |

### A.11 Sorting

| ID | Story | Expected |
|----|-------|----------|
| A.11.1 | I press N on the builds list. | Rows are sorted by build number in ascending order. The "Build #" column header shows a sort indicator: an up-arrow appended directly (e.g., "Build #^"). |
| A.11.2 | I press N again. | Sort order toggles to descending. The indicator changes to a down-arrow. |
| A.11.3 | I press S on the builds list. | Rows are sorted by build status alphabetically. The "Status" column header shows the sort indicator. Other column headers lose their indicator. |
| A.11.4 | I press A on the builds list. | Rows are sorted by start time (age). The "Start Time" column header shows the sort indicator. |
| A.11.5 | I sort by name, then apply a filter. | The filtered subset remains sorted. The sort indicator persists on the column header. |
| A.11.6 | I sort by name, then refresh with ctrl+r. | After data reloads, the sort order and direction are preserved. The indicator remains. |

### A.12 Filter

| ID | Story | Expected |
|----|-------|----------|
| A.12.1 | I press /. | The header right side changes from "? for help" to "/|" (amber/bold, with cursor). Filter mode is active. |
| A.12.2 | I type "FAILED" in filter mode. | The header right shows "/FAILED|". Only rows whose visible content contains "FAILED" (case-insensitive) are displayed. The frame title updates to "cb-builds(M/N) -- project-name". |
| A.12.3 | I press backspace in filter mode. | The last character of the filter text is removed. The filtered result updates immediately. |
| A.12.4 | I press Escape in filter mode. | The filter is cleared. All rows reappear. The frame title reverts to the unfiltered count. The header right reverts to "? for help". |
| A.12.5 | I type a filter string that matches no builds. | Zero rows are displayed. The frame title shows "cb-builds(0/N) -- project-name". |
| A.12.6 | I type "codepipeline" to filter by initiator. | Only builds initiated by CodePipeline are shown. Filtering matches across all visible columns, not just build number. |

### A.13 Enter Key (Drill Into Build Logs)

| ID | Story | Expected |
|----|-------|----------|
| A.13.1 | I select a build and press Enter. | The view transitions to the Build Logs view for that build. A loading spinner appears while CloudWatch log events are fetched. The builds list view is pushed onto the view stack. |
| A.13.2 | I verify Enter opens Build Logs, NOT the detail view. | Pressing Enter on a build navigates into its CloudWatch build logs. It does NOT open the build detail/describe view. The detail view is accessed via `d`. |

**AWS comparison:**
```
aws codebuild batch-get-builds --ids <build-id> --query 'builds[0].logs.{GroupName:groupName,StreamName:streamName}'
aws logs get-log-events --log-group-name <group> --log-stream-name <stream>
```

### A.14 Detail Key (d)

| ID | Story | Expected |
|----|-------|----------|
| A.14.1 | I select a build and press d. | The detail view opens for the selected build. The frame title shows the build identifier. |
| A.14.2 | I verify the detail fields match the design spec. | The detail view shows key-value pairs for: Id, Arn, BuildNumber, BuildStatus, StartTime, EndTime, CurrentPhase, SourceVersion, ResolvedSourceVersion, Initiator, Source, Environment, Phases, Logs, Cache, VpcConfig, ServiceRole, TimeoutInMinutes, QueuedTimeoutInMinutes, BuildBatchArn. |
| A.14.3 | I press Escape on the detail view. | I return to the builds list. The cursor position is preserved on the same build I had selected. |

**AWS comparison:**
```
aws codebuild batch-get-builds --ids <build-id>
```
Expected fields visible: Id, Arn, BuildNumber, BuildStatus, StartTime, EndTime, CurrentPhase, SourceVersion, ResolvedSourceVersion, Initiator, Source, Environment, Phases, Logs, Cache, VpcConfig, ServiceRole, TimeoutInMinutes, QueuedTimeoutInMinutes, BuildBatchArn

### A.15 YAML Key (y)

| ID | Story | Expected |
|----|-------|----------|
| A.15.1 | I select a build and press y. | The YAML view opens. The frame title includes the build identifier and "yaml". The full build resource is rendered as syntax-highlighted YAML. |
| A.15.2 | YAML keys are colored blue (#7aa2f7), string values green (#9ece6a), numbers orange (#ff9e64), booleans purple (#bb9af7), null values dim (#565f89). | Visual inspection confirms the color coding matches the design spec. |
| A.15.3 | The YAML content is longer than the visible area. | I can scroll with j/k/g/G. Scroll indicators appear when content extends beyond the visible area. |
| A.15.4 | I press Escape on the YAML view. | I return to the builds list. |

### A.16 Copy Key (c)

| ID | Story | Expected |
|----|-------|----------|
| A.16.1 | I select a build and press c. | The build ID is copied to the system clipboard (e.g., "payment-api-build:a1b2c3d4-e5f6-7890-abcd-ef1234567890"). A green flash message "Copied!" appears in the header right side. |
| A.16.2 | After approximately 2 seconds. | The "Copied!" flash message auto-clears and the header right reverts to "? for help". |
| A.16.3 | I paste from clipboard into another application. | The pasted text matches the full build ID exactly (project-name:uuid format). |

**AWS comparison:**
```
aws codebuild batch-get-builds --ids <build-id> --query 'builds[0].id'
```
Returns the full build ID in "project:uuid" format.

### A.17 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| A.17.1 | I press ctrl+r on the builds list. | The loading spinner appears. A fresh API call chain (ListBuildsForProject + BatchGetBuilds) is made. When it completes, the table repopulates with current data. |
| A.17.2 | A new build was triggered since the last load. I press ctrl+r. | The new build appears in the refreshed list (at the top, since builds are returned newest first). The count in the frame title increments. |
| A.17.3 | I had a filter active and press ctrl+r. | The data refreshes. The filter remains applied to the new data. The frame title count updates accordingly. |

### A.18 Escape (Back)

| ID | Story | Expected |
|----|-------|----------|
| A.18.1 | I press Escape on the builds list (not in filter/command mode). | I return to the CodeBuild project list. The cursor is on the same project I had entered. |

### A.19 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| A.19.1 | I press ? on the builds list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: BUILDS, GENERAL, NAVIGATION, HOTKEYS. |
| A.19.2 | The BUILDS column shows the correct entries. | The column contains: `<esc>` Back, `<enter>` View Logs, `<d>` Detail, `<y>` YAML, `<c>` Copy Build ID. |
| A.19.3 | The GENERAL column shows the correct entries. | The column contains: `<ctrl-r>` Refresh, `<q>` Quit, `</>` Filter, `<:>` Command. |
| A.19.4 | The NAVIGATION column shows the correct entries. | The column contains: `<j>` Down, `<k>` Up, `<g>` Top, `<G>` Bottom, `<h/l>` Cols, `<pgup/dn>` Page. |
| A.19.5 | The HOTKEYS column shows the correct entries. | The column contains: `<?>` Help, `<:>` Command. |
| A.19.6 | I press any key on the help screen. | The help screen closes and the builds list table reappears. |

### A.20 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| A.20.1 | I press : on the builds list. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| A.20.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list. The builds view is no longer on the view stack. |
| A.20.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The builds list remains. |

### A.21 Alternating Row Colors

| ID | Story | Expected |
|----|-------|----------|
| A.21.1 | The builds list has more than 2 rows. | Alternating rows have a subtle background color difference (#1e2030) for readability. Status-based row coloring still applies. Selected row always has blue background regardless. |

### A.22 Build Ordering

| ID | Story | Expected |
|----|-------|----------|
| A.22.1 | Builds load with default ordering. | Builds are displayed newest first (highest build number at top), matching the order returned by `ListBuildsForProject`. |

**AWS comparison:**
```
aws codebuild list-builds-for-project --project-name my-project --sort-order DESCENDING
```
Returns IDs in descending chronological order.

---

## B. Build Logs View (Level 2 Child -- Cross-Service to CloudWatch)

### B.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | I select build #139 from the builds list and press Enter. | The build logs view opens. A spinner appears centered in the frame with text like "Fetching build logs..." while CloudWatch log events are retrieved. The frame title shows "build-logs" with no count during loading. |
| B.1.2 | The API responds successfully with log events. | The spinner disappears. Log lines are rendered as table rows with columns: Timestamp and Message. The frame title shows "build-logs(240) -- #139" where 240 is the total log line count. |
| B.1.3 | The API responds with an error (e.g., ResourceNotFoundException, log group does not exist). | The spinner disappears. A red error flash appears in the header. |

**AWS comparison:**
```
aws codebuild batch-get-builds --ids <build-id> --query 'builds[0].logs'
aws logs get-log-events --log-group-name /aws/codebuild/my-project --log-stream-name <stream-name>
```
Expected fields visible: Timestamp, Message

### B.2 Build with No Log Group Configured (Edge Case)

| ID | Story | Expected |
|----|-------|----------|
| B.2.1 | I press Enter on a build whose `Logs.GroupName` is empty (custom log config or logs disabled). | The view shows a centered message: "Build logs not available in CloudWatch." or similar. No crash, no spinner stuck in perpetuity. |
| B.2.2 | I press Escape from the "logs not available" state. | I return to the builds list. |

**AWS comparison:**
```
aws codebuild batch-get-builds --ids <build-id> --query 'builds[0].logs.groupName'
```
Returns null when logging is not configured to CloudWatch.

### B.3 Empty Logs

| ID | Story | Expected |
|----|-------|----------|
| B.3.1 | A build was just started and no log events exist yet. | The frame title shows "build-logs(0) -- #142". A centered message indicates no log events found. |
| B.3.2 | I press ctrl+r after waiting for the build to produce output. | The data refreshes. New log lines appear as they become available. |

### B.4 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| B.4.1 | Log events are loaded. | Two columns are displayed: "Timestamp" (width 22) and "Message" (width 0, fills remaining terminal width). Column headers are bold blue (#7aa2f7) with no separator line below. |
| B.4.2 | I verify column data against the AWS CLI output. | "Timestamp" maps to the event timestamp. "Message" maps to the log event message text. All log events returned by the CLI appear as rows. |
| B.4.3 | A log message is longer than the remaining terminal width. | The message is truncated at the available width. The `w` key can be pressed to toggle word wrap. |

**AWS comparison:**
```
aws logs get-log-events --log-group-name <group> --log-stream-name <stream> --query 'events[].{Timestamp:timestamp,Message:message}'
```
Expected fields visible: Timestamp, Message

### B.5 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| B.5.1 | I am viewing logs for build #139 with 240 log lines. | The frame title shows "build-logs(240) -- #139" centered in the top border. |
| B.5.2 | A filter is active matching 15 of 240 log lines. | The frame title shows "build-logs(15/240) -- #139". |

### B.6 Row Coloring by Log Content

| ID | Story | Expected |
|----|-------|----------|
| B.6.1 | A log line contains "FAIL" or "ERROR" or "error" or "Error" or "did not exit successfully". | The entire row is rendered in red (#f7768e). |
| B.6.2 | A log line contains "Phase complete" or "SUCCEEDED". | The entire row is rendered in green (#9ece6a). |
| B.6.3 | A log line contains "Entering phase" or "Running command". | The entire row is rendered in yellow (#e0af68). |
| B.6.4 | A log line contains none of the above patterns. | The row is rendered in plain text color (#c0caf5). |
| B.6.5 | I select a colored log line. | The selected row has full-width blue background (#7aa2f7) with dark foreground (#1a1b26), bold text. The content-based color is overridden. |
| B.6.6 | I move selection away from a colored log line. | The previously selected row reverts to its content-based coloring. |

**AWS comparison:**
```
aws logs get-log-events --log-group-name /aws/codebuild/my-project --log-stream-name <stream>
```
The message content determines row coloring based on keyword matching.

### B.7 FAILED Build Logs (Edge Case)

| ID | Story | Expected |
|----|-------|----------|
| B.7.1 | I drill into logs for a FAILED build. The logs contain test failure output (e.g., "FAIL src/payment.test.ts", "Expected: 200", "Received: 500"). | The lines containing "FAIL" are rendered in red. Non-error lines (like "Expected:" and "Received:") are rendered in plain text unless they match other color patterns. The log output is readable and scrollable. |
| B.7.2 | The build failed and the last few lines contain "Command did not exit successfully". | Those lines are rendered in red (#f7768e). |

### B.8 Large Log Output (Edge Case)

| ID | Story | Expected |
|----|-------|----------|
| B.8.1 | A build has thousands of log lines (e.g., a verbose test run). | The initial fetch loads and displays the first batch. The spinner disappears when the initial data is ready. The log view is scrollable. |
| B.8.2 | I scroll to the bottom of the loaded logs with G. | The cursor jumps to the last loaded log line. |

### B.9 Word Wrap Toggle (w)

| ID | Story | Expected |
|----|-------|----------|
| B.9.1 | I press w in the build logs view. | Word wrap is toggled on. Long message lines that previously extended beyond the visible width now wrap to the next line. |
| B.9.2 | I press w again. | Word wrap is toggled off. Wrapped lines collapse back to single truncated lines. |

### B.10 Navigation

| ID | Story | Expected |
|----|-------|----------|
| B.10.1 | I press j/k/g/G/PageUp/PageDown in the build logs view. | Navigation behaves identically to other list views: j moves down, k moves up, g jumps to top, G jumps to bottom, PageUp/PageDown scroll by page. |

### B.11 Filter

| ID | Story | Expected |
|----|-------|----------|
| B.11.1 | I press / in the build logs view and type "ERROR". | Only log lines whose content contains "ERROR" (case-insensitive) are shown. The frame title updates to show matched/total count. |
| B.11.2 | I press / and type "phase" to find all build phase transitions. | Lines containing "Entering phase" and "Phase complete" are shown. I can quickly scan the build phase timeline. |
| B.11.3 | I press Escape while filter is active. | The filter clears. All log lines reappear. |

### B.12 Copy Key (c)

| ID | Story | Expected |
|----|-------|----------|
| B.12.1 | I select a log line and press c. | The full message text of the selected log line is copied to the system clipboard. A green flash "Copied!" appears in the header. |
| B.12.2 | I paste from clipboard into another application. | The pasted text matches the message content of the selected log line exactly. |

**AWS comparison:**
```
aws logs get-log-events --log-group-name <group> --log-stream-name <stream> --query 'events[N].message'
```
The copied text matches the message field of the Nth event.

### B.13 Detail Key (d)

| ID | Story | Expected |
|----|-------|----------|
| B.13.1 | I select a log line and press d. | The detail view opens for that log event. |
| B.13.2 | I verify the detail fields match the design spec. | The detail view shows key-value pairs for: Timestamp, IngestionTime, Message, EventId. |
| B.13.3 | I press Escape on the detail view. | I return to the build logs list. |

**AWS comparison:**
```
aws logs get-log-events --log-group-name <group> --log-stream-name <stream> --query 'events[N]'
```
Expected fields visible: Timestamp, IngestionTime, Message, EventId

### B.14 YAML Key (y)

| ID | Story | Expected |
|----|-------|----------|
| B.14.1 | I select a log line and press y. | The YAML view opens showing the full log event as syntax-highlighted YAML. |
| B.14.2 | I press Escape on the YAML view. | I return to the build logs list. |

### B.15 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| B.15.1 | I press ctrl+r in the build logs view. | The spinner appears. A fresh `GetLogEvents` call is made. The table updates with new results. |
| B.15.2 | A build is IN_PROGRESS and producing new log output. I press ctrl+r. | New log lines appear in the refreshed list. The count in the frame title increases. |

### B.16 Escape (Back to Builds List)

| ID | Story | Expected |
|----|-------|----------|
| B.16.1 | I press Escape on the build logs view (not in filter/command mode). | I return to the builds list. The cursor is on the same build I had entered. |

### B.17 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| B.17.1 | I press ? on the build logs view. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: BUILD LOGS, GENERAL, NAVIGATION, HOTKEYS. |
| B.17.2 | The BUILD LOGS column shows the correct entries. | The column contains: `<esc>` Back, `<d>` Detail, `<y>` YAML, `<c>` Copy Message, `<w>` Word Wrap. |
| B.17.3 | The GENERAL column shows the correct entries. | The column contains: `<ctrl-r>` Refresh, `</>` Filter, `<:>` Command. |
| B.17.4 | The NAVIGATION column shows the correct entries. | The column contains: `<j>` Down, `<k>` Up, `<g>` Top, `<G>` Bottom, `<pgup/dn>` Page. |
| B.17.5 | The HOTKEYS column shows the correct entries. | The column contains: `<?>` Help, `<:>` Command. |
| B.17.6 | I press any key on the help screen. | The help screen closes and the build logs table reappears. |

### B.18 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| B.18.1 | I press : on the build logs view. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| B.18.2 | I type "s3" and press Enter. | The view navigates to the S3 bucket list. |
| B.18.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The build logs list remains. |

---

## C. Pipeline Stages View (Level 1 Child)

### C.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | I select a pipeline "payment-service-deploy" from the CodePipeline list and press Enter. | The view transitions to the pipeline stages list. A spinner appears centered in the frame with text like "Fetching pipeline stages..." while `GetPipelineState` is in flight. |
| C.1.2 | The API responds successfully with stage data. | The spinner disappears. The table renders with stage-action rows. The frame title updates to "pipeline-stages(4) -- payment-service-deploy" where 4 is the row count (each stage-action pair is one row). |
| C.1.3 | The API responds with an error (e.g., PipelineNotFoundException). | The spinner disappears. A red error flash message appears in the header right side. |

**AWS comparison:**
```
aws codepipeline get-pipeline-state --name payment-service-deploy
```
Expected fields visible: Stage, Stage Status, Action, Action Status, Last Changed, External URL

### C.2 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | Pipeline stages load and the table renders. | Six columns are displayed: "Stage" (width 20), "Stage Status" (width 14), "Action" (width 24), "Action Status" (width 14), "Last Changed" (width 22), "External URL" (width 40). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| C.2.2 | I verify column data against the AWS CLI output. | "Stage" maps to `StageStates[].StageName`. "Stage Status" maps to `StageStates[].LatestExecution.Status`. "Action" maps to `StageStates[].ActionStates[].ActionName`. "Action Status" maps to `StageStates[].ActionStates[].LatestExecution.Status`. "Last Changed" maps to `StageStates[].ActionStates[].LatestExecution.LastStatusChange`. "External URL" maps to `StageStates[].ActionStates[].LatestExecution.ExternalExecutionUrl`. |
| C.2.3 | A stage name is longer than 20 characters. | The name is truncated to fit the 20-character column width. |
| C.2.4 | An external URL is longer than 40 characters. | The URL is truncated to fit the 40-character column width. |
| C.2.5 | The terminal is narrower than the combined column widths (20+14+24+14+22+40 = 134 plus borders/padding). | The rightmost column(s) are hidden (not truncated mid-value). Horizontal scroll with h/l is available to reveal hidden columns. |

**AWS comparison:**
```
aws codepipeline get-pipeline-state --name my-pipeline --query 'stageStates[].{Stage:stageName,Status:latestExecution.status,Actions:actionStates[].{Name:actionName,Status:latestExecution.status,LastChange:latestExecution.lastStatusChange,URL:latestExecution.externalExecutionUrl}}'
```
Expected fields visible: Stage, Stage Status, Action, Action Status, Last Changed, External URL

### C.3 Flattened Stage-Action Rows

| ID | Story | Expected |
|----|-------|----------|
| C.3.1 | A pipeline has stages: Source (1 action), Build (1 action), Staging (2 actions: DeployToStaging + IntegrationTests), Production (2 actions: ApprovalGate + DeployToProduction). | The table shows 6 rows total: one row per stage-action pair. |
| C.3.2 | A stage has multiple actions (e.g., Staging with DeployToStaging and IntegrationTests). | The Stage column value ("Staging") is shown only on the first row for that stage. The second action row for the same stage leaves the Stage column blank for visual grouping. The Stage Status column also appears only on the first row. |
| C.3.3 | I verify the visual grouping. | Rows for the same stage are visually grouped by the blank Stage column on continuation rows, making it clear which actions belong to which stage. |

**AWS comparison:**
```
aws codepipeline get-pipeline-state --name my-pipeline --query 'stageStates[].{StageName:stageName,ActionCount:length(actionStates)}'
```
Confirms which stages have multiple actions.

### C.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| C.4.1 | 4 stages with 6 total stage-action rows are loaded for pipeline "payment-service-deploy". | The frame top border shows the title centered: "pipeline-stages(6) -- payment-service-deploy" (or the count may reflect total rows, not distinct stages). |
| C.4.2 | A filter is active and matches 2 rows. | The frame title reads "pipeline-stages(2/6) -- payment-service-deploy". |

### C.5 Row Coloring by Action Status

| ID | Story | Expected |
|----|-------|----------|
| C.5.1 | An action has status Succeeded. | The entire row is rendered in green (#9ece6a). |
| C.5.2 | An action has status Failed. | The entire row is rendered in red (#f7768e). |
| C.5.3 | An action has status InProgress. | The entire row is rendered in yellow (#e0af68). |
| C.5.4 | An action has status Stopped or Abandoned. | The entire row is rendered dim (#565f89). |
| C.5.5 | An action has no status (stage not yet reached in the pipeline execution). | The entire row is rendered dim (#565f89). The Action Status column shows a dash or is blank. |
| C.5.6 | I select any row regardless of its status color. | The selected row has full-width blue background (#7aa2f7) with dark foreground (#1a1b26), bold text. The status color is overridden. |

**AWS comparison:**
```
aws codepipeline get-pipeline-state --name my-pipeline --query 'stageStates[].actionStates[].latestExecution.status'
```
Each returned status value determines the row color.

### C.6 All Stages Succeeded (Edge Case)

| ID | Story | Expected |
|----|-------|----------|
| C.6.1 | Every stage and action in the pipeline has status Succeeded. | All rows are rendered in green (#9ece6a). Every Stage Status column shows "Succeeded". Every Action Status column shows "Succeeded". Every action has a Last Changed timestamp. |

**AWS comparison:**
```
aws codepipeline get-pipeline-state --name my-pipeline
```
All `latestExecution.status` values are "Succeeded".

### C.7 Pipeline Stuck at Approval (Edge Case)

| ID | Story | Expected |
|----|-------|----------|
| C.7.1 | A pipeline has a ManualApproval action that is InProgress (awaiting approval). | The approval action row is colored yellow (#e0af68). The Action Status shows "InProgress". Stages after the approval (e.g., DeployProd) show no status (dim, with dashes). |
| C.7.2 | The row for the Approval stage shows stage status InProgress. | The Stage Status column shows "InProgress" for the Approval stage. |
| C.7.3 | The Last Changed column for the InProgress approval action. | Shows a dash or the time the approval was requested, not a future time. |
| C.7.4 | Stages after the pending approval. | The DeployProd stage shows no status. Both Stage Status and Action Status columns show dashes. The rows are dim (#565f89). |

**AWS comparison:**
```
aws codepipeline get-pipeline-state --name my-pipeline --query 'stageStates[?stageName==`Approval`].latestExecution'
```
Status is "InProgress" for the Approval stage.

### C.8 Pipeline with Failed Stage (Edge Case)

| ID | Story | Expected |
|----|-------|----------|
| C.8.1 | A pipeline failed at the Build stage. Source succeeded, Build failed, subsequent stages have not run. | The Source row is green. The Build row is red. Subsequent stage rows (DeployStaging, DeployProd) are dim with dash status values. |
| C.8.2 | The failed Build action row shows the Last Changed time of the failure. | The timestamp reflects when the build failed. |

**AWS comparison:**
```
aws codepipeline get-pipeline-state --name my-pipeline
```
Build stage `latestExecution.status` is "Failed". Subsequent stages have no `latestExecution`.

### C.9 Pipeline with InProgress Stage (Edge Case)

| ID | Story | Expected |
|----|-------|----------|
| C.9.1 | The Build stage is currently InProgress. Source has succeeded. | The Source row is green. The Build row is yellow. Subsequent stage rows are dim. |
| C.9.2 | I press ctrl+r while a stage is InProgress. | The data refreshes from `GetPipelineState`. If the stage has since completed, the status colors update accordingly. |

### C.10 External URL Present vs Absent (Edge Case)

| ID | Story | Expected |
|----|-------|----------|
| C.10.1 | An action has an external execution URL (e.g., a link to the CodeBuild build console page). | The External URL column shows the URL, truncated if necessary. |
| C.10.2 | An action has no external URL (e.g., a manual approval, or a stage not yet reached). | The External URL column shows a dash or is blank. |
| C.10.3 | I scroll horizontally with l to reveal the External URL column if it was hidden. | The column becomes visible, showing the URL for actions that have one. |

**AWS comparison:**
```
aws codepipeline get-pipeline-state --name my-pipeline --query 'stageStates[].actionStates[].latestExecution.externalExecutionUrl'
```
Some actions have URLs (CodeBuild, CodeDeploy), some do not (manual approvals, source actions).

### C.11 Navigation

| ID | Story | Expected |
|----|-------|----------|
| C.11.1 | I press j (or down-arrow) with the first row selected. | The selection cursor moves to the second row. |
| C.11.2 | I press k (or up-arrow) with the second row selected. | The selection cursor moves back to the first row. |
| C.11.3 | I press g. | The selection jumps to the very first row. |
| C.11.4 | I press G. | The selection jumps to the very last row. |
| C.11.5 | I press PageDown/PageUp. | Standard page navigation applies. |
| C.11.6 | I press h/l. | Columns scroll left/right. Column headers scroll in sync with data columns. |

### C.12 Sorting

| ID | Story | Expected |
|----|-------|----------|
| C.12.1 | I press N on the pipeline stages list. | Rows are sorted by stage name alphabetically. The "Stage" column header shows a sort indicator. |
| C.12.2 | I press S on the pipeline stages list. | Rows are sorted by status. The sort indicator appears on the relevant status column header. |
| C.12.3 | I press A on the pipeline stages list. | Rows are sorted by last changed time. The "Last Changed" column header shows the sort indicator. |

### C.13 Filter

| ID | Story | Expected |
|----|-------|----------|
| C.13.1 | I press / and type "Failed". | Only rows where any visible column contains "Failed" (case-insensitive) are shown. The frame title updates to show matched/total count. |
| C.13.2 | I press / and type "Deploy". | Only action rows whose action name contains "Deploy" are shown. |
| C.13.3 | I press Escape while filter is active. | The filter clears. All rows reappear. |

### C.14 Copy Key (c)

| ID | Story | Expected |
|----|-------|----------|
| C.14.1 | I select a row that has an external execution URL and press c. | The external execution URL is copied to the system clipboard. A green flash "Copied!" appears in the header. |
| C.14.2 | I select a row that has no external execution URL and press c. | The action name is copied to the clipboard instead. A green flash "Copied!" appears. |
| C.14.3 | I paste from clipboard into a browser. | If a URL was copied, it navigates to the CodeBuild/CodeDeploy console page for that action execution. |

**AWS comparison:**
```
aws codepipeline get-pipeline-state --name my-pipeline --query 'stageStates[].actionStates[].latestExecution.externalExecutionUrl'
```
The copied value matches the externalExecutionUrl for that action, or the actionName if no URL exists.

### C.15 Detail Key (d)

| ID | Story | Expected |
|----|-------|----------|
| C.15.1 | I select a stage-action row and press d. | The detail view opens for the selected stage-action pair. |
| C.15.2 | I verify the detail fields match the design spec. | The detail view shows key-value pairs for: stage_name, stage_status, action_name, action_status, last_change_time, external_url, action_token, action_error_details, revision_id, revision_summary. |
| C.15.3 | I press Escape on the detail view. | I return to the pipeline stages list. The cursor position is preserved. |

**AWS comparison:**
```
aws codepipeline get-pipeline-state --name my-pipeline
```
Expected fields visible: stage_name, stage_status, action_name, action_status, last_change_time, external_url, action_token, action_error_details, revision_id, revision_summary

### C.16 YAML Key (y)

| ID | Story | Expected |
|----|-------|----------|
| C.16.1 | I select a stage-action row and press y. | The YAML view opens showing the full stage-action data as syntax-highlighted YAML. |
| C.16.2 | I press Escape on the YAML view. | I return to the pipeline stages list. |

### C.17 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| C.17.1 | I press ctrl+r on the pipeline stages list. | The loading spinner appears. A fresh `GetPipelineState` call is made. When it completes, the table repopulates with current data. |
| C.17.2 | A pipeline stage was InProgress and has since succeeded. I press ctrl+r. | The stage status and action status update. The row coloring changes from yellow to green. |
| C.17.3 | Fast API response (GetPipelineState is a single non-paginated call). | The spinner appears briefly (typically less than 1 second). The refresh feels near-instant. |

### C.18 Escape (Back)

| ID | Story | Expected |
|----|-------|----------|
| C.18.1 | I press Escape on the pipeline stages list (not in filter/command mode). | I return to the CodePipeline list. The cursor is on the same pipeline I had entered. |

### C.19 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| C.19.1 | I press ? on the pipeline stages list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: PIPELINE STAGES, GENERAL, NAVIGATION, HOTKEYS. |
| C.19.2 | The PIPELINE STAGES column shows the correct entries. | The column contains: `<esc>` Back, `<d>` Detail, `<y>` YAML, `<c>` Copy URL. |
| C.19.3 | The GENERAL column shows the correct entries. | The column contains: `<ctrl-r>` Refresh, `</>` Filter, `<:>` Command. |
| C.19.4 | The NAVIGATION column shows the correct entries. | The column contains: `<j>` Down, `<k>` Up, `<g>` Top, `<G>` Bottom, `<h/l>` Cols, `<pgup/dn>` Page. |
| C.19.5 | The HOTKEYS column shows the correct entries. | The column contains: `<?>` Help, `<:>` Command. |
| C.19.6 | I press any key on the help screen. | The help screen closes and the pipeline stages table reappears. |

### C.20 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| C.20.1 | I press : on the pipeline stages list. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| C.20.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list. |
| C.20.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The pipeline stages list remains. |

### C.21 Alternating Row Colors

| ID | Story | Expected |
|----|-------|----------|
| C.21.1 | The pipeline stages list has more than 2 rows. | Alternating rows have a subtle background color difference (#1e2030) for readability. Status-based row coloring still applies. Selected row always has blue background regardless. |

---

## D. Cross-Cutting Concerns

### D.1 Header Consistency

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | In every CI/CD child view (builds list, build logs, pipeline stages, detail, YAML), the header displays: "a9s" (accent bold), version (dim), profile:region (bold). | Visual inspection confirms across all CI/CD views. |
| D.1.2 | The header right side shows "? for help" in normal mode across all CI/CD child views. | Confirmed in builds list, build logs, pipeline stages, detail, and YAML views. |

### D.2 View Stack -- CodeBuild 3-Level Drill-Down

| ID | Story | Expected |
|----|-------|----------|
| D.2.1 | CodeBuild Projects -> Builds -> Build Logs; then Escape three times. | Each Escape pops one level: Build Logs -> Builds -> CodeBuild Projects. No state is lost at any intermediate level. Cursor positions are preserved at each level. |
| D.2.2 | CodeBuild Projects -> Builds -> Build Detail (d) -> YAML (y); then Escape three times. | YAML -> Detail -> Builds -> CodeBuild Projects. The cursor is still on the same build and project respectively. |
| D.2.3 | CodeBuild Projects -> Builds -> Build Logs -> Log Detail (d); then Escape three times. | Log Detail -> Build Logs -> Builds -> CodeBuild Projects. Full 4-level stack navigation. |
| D.2.4 | CodeBuild Projects -> Builds -> Build Logs -> YAML (y); then Escape three times. | YAML -> Build Logs -> Builds -> CodeBuild Projects. |

### D.3 View Stack -- CodePipeline 2-Level Drill-Down

| ID | Story | Expected |
|----|-------|----------|
| D.3.1 | CodePipeline -> Pipeline Stages; then Escape once. | I return to the CodePipeline list. The cursor is on the same pipeline. |
| D.3.2 | CodePipeline -> Pipeline Stages -> Detail (d) -> YAML (y); then Escape three times. | YAML -> Detail -> Pipeline Stages -> CodePipeline. No state is lost at any intermediate level. |

### D.4 Terminal Resize

| ID | Story | Expected |
|----|-------|----------|
| D.4.1 | I resize the terminal while viewing the builds list. | The layout reflows. Column visibility adjusts to the new width. The frame border redraws correctly. |
| D.4.2 | I resize the terminal while viewing build logs. | The Message column (width 0, fills remaining space) adjusts to the new terminal width. If word wrap is on, wrapped lines reflow. |
| D.4.3 | I resize the terminal while viewing pipeline stages. | Column visibility adjusts. The External URL column (widest at 40) may appear or disappear depending on terminal width. |
| D.4.4 | I resize the terminal to below 60 columns. | An error message appears: "Terminal too narrow. Please resize." |
| D.4.5 | I resize the terminal to below 7 lines. | An error message appears: "Terminal too short. Please resize." |

### D.5 Profile and Region Switch

| ID | Story | Expected |
|----|-------|----------|
| D.5.1 | I am viewing builds for a CodeBuild project. I switch profiles via command mode. | After the profile switch, I am navigated to an appropriate state (main menu or resource list). The builds list is no longer visible since the data belongs to the previous profile. |
| D.5.2 | I am viewing pipeline stages. I switch regions via command mode. | After the region switch, the pipeline stages data is no longer valid for the new region. Navigation returns to an appropriate state. |

### D.6 Build Logs Cross-Service Nature

| ID | Story | Expected |
|----|-------|----------|
| D.6.1 | The build logs view fetches from CloudWatch Logs, not CodeBuild. | The data displayed in the build logs view comes from CloudWatch Logs (`GetLogEvents`), using the log group and stream names from the build's `Logs` field. This is transparent to the user -- the drill-down feels like staying within CodeBuild context. |
| D.6.2 | The CloudWatch Logs API requires separate permissions from CodeBuild. | If the user has `codebuild:*` but not `logs:GetLogEvents`, the build logs view shows an appropriate permission error, not a generic crash. |

### D.7 Computed Fields

| ID | Story | Expected |
|----|-------|----------|
| D.7.1 | I verify the Duration column on a completed build shows human-friendly format. | Duration is displayed as "Xm Ys" (e.g., "3m 22s", "12m 45s"), not raw seconds or millisecond timestamps. |
| D.7.2 | I verify the Duration column on a build that took less than 1 minute. | Duration shows "0m 45s" or similar sub-minute format. |
| D.7.3 | I verify the Source Version column shows a truncated commit SHA. | Source Version shows the first 8 characters (e.g., "a1b2c3d4"), not the full 40-character SHA. |
| D.7.4 | I verify pipeline stage computed fields. | Stage, Stage Status, Action, Action Status, Last Changed, and External URL are all properly extracted from the hierarchical GetPipelineState response. |

**AWS comparison (Duration):**
```
aws codebuild batch-get-builds --ids <build-id> --query 'builds[0].{Start:startTime,End:endTime}'
```
Duration = EndTime - StartTime, formatted as human-friendly string.

**AWS comparison (Source Version):**
```
aws codebuild batch-get-builds --ids <build-id> --query 'builds[0].{SV:sourceVersion,RSV:resolvedSourceVersion}'
```
Displayed value = first 8 characters of sourceVersion or resolvedSourceVersion.
