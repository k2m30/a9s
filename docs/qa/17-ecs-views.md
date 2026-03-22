# QA User Stories: ECS Child Views

Covers the three child views accessible from the ECS Services list:
- **A. Service Events** (`e` key) -- event timeline for an ECS service
- **B. Tasks** (Enter key) -- running and recently stopped tasks
- **C. Container Logs** (`L` key) -- CloudWatch log lines from the service's containers

All stories are written from a black-box perspective against the design spec and
`views.yaml` configuration files.

AWS CLI equivalents are cited so testers can verify data parity.

---

## A. ECS Service Events View

### A.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I select a service named "payment-api" in the ECS Services list and press `e`. | The view transitions to the service events view. A spinner appears centered in the frame with text like "Fetching ECS service events..." while the API call is in flight. The ECS Services list is pushed onto the view stack. |
| A.1.2 | The API responds successfully with event data. | The spinner disappears. Events are rendered as table rows with columns: Timestamp, Message. The frame title reads `ecs-events(87) -- payment-api` where 87 is the event count and "payment-api" is the parent service name. |
| A.1.3 | The API responds with an error (e.g., AccessDenied, cluster not found). | The spinner disappears. A red error flash message appears in the header right side (e.g., "Error: AccessDeniedException"). |
| A.1.4 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. The spinner continues uninterrupted. |

**AWS comparison:**
```
aws ecs describe-services --cluster CLUSTER --services SERVICE --query 'services[0].events'
```
Expected fields visible: Timestamp (from CreatedAt), Message

### A.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | A newly created service has zero events. I press `e` on it. | The frame title reads `ecs-events(0) -- my-new-service`. The content area shows a centered message (e.g., "No events found") with a hint to refresh or wait. No column headers are shown (or headers with no data rows). |
| A.2.2 | I press ctrl+r on the empty events view. | The loading spinner reappears while the refresh request is in flight. |

### A.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | Events load and the table renders. | Exactly two columns are displayed: "Timestamp" (width 22) and "Message" (width 0, meaning it fills the remaining terminal width). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| A.3.2 | I verify column data against `aws ecs describe-services --cluster CLUSTER --services SERVICE --query 'services[0].events'`. | The "Timestamp" column maps to `.Events[].CreatedAt`. The "Message" column maps to `.Events[].Message`. Every event returned by the CLI appears as a row. Events are ordered newest first (as returned by AWS). |
| A.3.3 | A message is longer than the remaining terminal width. | The message is truncated to fit the available width. No row wrapping occurs in the list view. |
| A.3.4 | The terminal is very wide (160+ columns). | The Message column expands to fill all available space since its width is 0 (fill remaining). More of each event message is visible without truncation. |

### A.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| A.4.1 | 87 events are loaded for service "payment-api". | The frame top border shows the title centered: `ecs-events(87) -- payment-api` with equal-length dashes on both sides. |
| A.4.2 | A filter is active and matches 5 of 87 events. | The frame title reads `ecs-events(5/87) -- payment-api`. |
| A.4.3 | A filter is active and matches 0 events. | The frame title reads `ecs-events(0/87) -- payment-api`. The content area is empty (no rows). |

### A.5 Row Coloring (Content-Based)

| ID | Story | Expected |
|----|-------|----------|
| A.5.1 | An event message contains "has reached a steady state". | The entire row is rendered in GREEN (#9ece6a). |
| A.5.2 | An event message contains "unable to place a task". | The entire row is rendered in RED (#f7768e). |
| A.5.3 | An event message contains "unable to consistently start tasks". | The entire row is rendered in RED (#f7768e). |
| A.5.4 | An event message contains "was unable to". | The entire row is rendered in RED (#f7768e). |
| A.5.5 | An event message contains "failed". | The entire row is rendered in RED (#f7768e). |
| A.5.6 | An event message contains "stopped". | The entire row is rendered in RED (#f7768e). |
| A.5.7 | An event message contains "unhealthy". | The entire row is rendered in RED (#f7768e). |
| A.5.8 | An event message contains "has started 1 tasks". | The entire row is rendered in YELLOW (#e0af68). |
| A.5.9 | An event message contains "registered 1 targets". | The entire row is rendered in YELLOW (#e0af68). |
| A.5.10 | An event message contains "deregistered". | The entire row is rendered in YELLOW (#e0af68). |
| A.5.11 | An event message does not match any of the above patterns (e.g., a generic informational event). | The entire row is rendered in PLAIN text color (#c0caf5). |
| A.5.12 | I select a row that is colored GREEN (steady state). | The selected row gains full-width blue background (#7aa2f7) with dark foreground (#1a1b26), bold. The green coloring is overridden by the selection highlight. |
| A.5.13 | I move selection away from that row. | The row reverts to its GREEN content-based coloring. |
| A.5.14 | Events alternate between steady-state (green) and error (red) rows. | The alternating row background (#1e2030) is applied as a subtle difference. Status coloring takes precedence for foreground text. Selected row always overrides both. |

### A.6 Navigation

| ID | Story | Expected |
|----|-------|----------|
| A.6.1 | I press j (or down-arrow) with the first event selected. | The selection cursor moves to the second event. |
| A.6.2 | I press k (or up-arrow) with the second event selected. | The selection cursor moves back to the first event. |
| A.6.3 | I press g. | The selection jumps to the very first event (newest). |
| A.6.4 | I press G. | The selection jumps to the very last event (oldest). |
| A.6.5 | I press PageDown (or ctrl+d). | The selection moves down by one page of visible rows. |
| A.6.6 | I press PageUp (or ctrl+u). | The selection moves up by one page of visible rows. |
| A.6.7 | There are more events than fit on screen. I scroll past the visible area. | The table scrolls to keep the selected row visible. The column headers remain in place. |

### A.7 Sorting

| ID | Story | Expected |
|----|-------|----------|
| A.7.1 | I press N on the events list. | Rows are sorted by the Message column alphabetically ascending. The "Message" column header shows a sort indicator (up-arrow appended, e.g., "Message^"). |
| A.7.2 | I press N again. | Sort order toggles to descending. The indicator changes to a down-arrow. |
| A.7.3 | I press A on the events list. | Rows are sorted by Timestamp (age) ascending (oldest first). The "Timestamp" column header shows the up-arrow indicator. |
| A.7.4 | I press A again. | Sort order toggles to descending (newest first). The indicator changes to a down-arrow. |

### A.8 Filter

| ID | Story | Expected |
|----|-------|----------|
| A.8.1 | I press /. | The header right side changes from "? for help" to "/|" (amber/bold, with cursor). Filter mode is active. |
| A.8.2 | I type "steady" in filter mode. | The header right shows "/steady|". Only rows whose message contains "steady" (case-insensitive) are displayed. The frame title updates to `ecs-events(M/N) -- payment-api`. |
| A.8.3 | I type "unable" to find all error events. | Only events containing "unable" are shown. These should all be RED-colored rows. |
| A.8.4 | I press Escape in filter mode. | The filter is cleared. All rows reappear. The frame title reverts to `ecs-events(N) -- payment-api`. The header right reverts to "? for help". |
| A.8.5 | I type a filter string that matches no events. | Zero rows are displayed. The frame title shows `ecs-events(0/N) -- payment-api`. |

### A.9 Copy (c)

| ID | Story | Expected |
|----|-------|----------|
| A.9.1 | I select an event and press c. | The full event message text is copied to the system clipboard. A green flash message "Copied!" appears in the header right side. |
| A.9.2 | After approximately 2 seconds. | The "Copied!" flash message auto-clears and the header right reverts to "? for help". |
| A.9.3 | I paste from clipboard into another application. | The pasted text matches the full event message exactly, including the service name and task references (e.g., "(service payment-api) has reached a steady state."). |
| A.9.4 | An event message was truncated in the list display due to terminal width. | The copied text contains the FULL untruncated message, not the truncated display version. |

**AWS comparison:**
```
aws ecs describe-services --cluster CLUSTER --services SERVICE --query 'services[0].events[0].message' --output text
```

### A.10 Detail View (d)

| ID | Story | Expected |
|----|-------|----------|
| A.10.1 | I select an event and press d. | The detail view opens. The frame title shows the event timestamp or identifying info. |
| A.10.2 | I verify the displayed fields. | The detail view shows key-value pairs for: CreatedAt, Message, Id. These match the `ecs_svc_events.detail` configuration. |
| A.10.3 | I compare the Id field value. | The Id is a UUID-format string uniquely identifying this event. |
| A.10.4 | The Message field in detail contains a very long event message. | The full message is displayed. If it extends beyond the visible width, `w` toggles word wrap so the full text is readable. |
| A.10.5 | I press Escape in the detail view. | I return to the events list. The cursor position is preserved on the same event I had selected. |

**AWS comparison:**
```
aws ecs describe-services --cluster CLUSTER --services SERVICE --query 'services[0].events[0].{Id:id,CreatedAt:createdAt,Message:message}'
```
Expected fields visible: CreatedAt, Message, Id

### A.11 YAML View (y)

| ID | Story | Expected |
|----|-------|----------|
| A.11.1 | I select an event and press y. | The YAML view opens. The frame title includes "yaml". The full event resource is rendered as syntax-highlighted YAML. |
| A.11.2 | YAML keys are colored blue (#7aa2f7), string values green (#9ece6a). | Visual inspection confirms the color coding matches the design spec. |
| A.11.3 | I press Escape on the YAML view. | I return to the events list. |

### A.12 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| A.12.1 | I press ctrl+r on the events list. | The loading spinner appears. A fresh API call is made. When it completes, the table repopulates with current data. |
| A.12.2 | The service has generated new events since the last load (e.g., a deployment completed). I press ctrl+r. | The new events appear at the top of the refreshed list. The count in the frame title updates. |
| A.12.3 | I had a filter active and press ctrl+r. | The data refreshes. The filter remains applied to the new data. The frame title count updates accordingly. |

### A.13 Escape (Back)

| ID | Story | Expected |
|----|-------|----------|
| A.13.1 | I press Escape on the events list (not in filter or detail). | I return to the ECS Services list. The cursor position is preserved on the same service I had selected. |

### A.14 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| A.14.1 | I press ? on the events list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: SERVICE EVENTS, GENERAL, NAVIGATION, HOTKEYS. |
| A.14.2 | I verify the help screen content. | SERVICE EVENTS column shows: `<esc>` Back, `<d>` Detail, `<y>` YAML, `<c>` Copy Message. GENERAL column shows: `<ctrl-r>` Refresh, `</>` Filter, `<:>` Command. NAVIGATION column shows: `<j>` Down, `<k>` Up, `<g>` Top, `<G>` Bottom, `<pgup/dn>` Page. HOTKEYS column shows: `<?>` Help, `<:>` Command. |
| A.14.3 | I press any key on the help screen. | The help screen closes and the events list table reappears. |

### A.15 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| A.15.1 | I press : on the events list. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| A.15.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list. The events and ECS Services views are removed from the view stack. |
| A.15.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The events list remains. |

### A.16 Maximum Events

| ID | Story | Expected |
|----|-------|----------|
| A.16.1 | A busy service has reached the 100-event maximum returned by AWS. | The frame title reads `ecs-events(100) -- busy-service`. All 100 events are displayed. Events older than the 100th are not available (this is an AWS API limitation). |
| A.16.2 | I verify the event ordering. | Events are displayed newest first (matching the AWS API return order). The first row has the most recent timestamp. |

---

## B. ECS Tasks View

### B.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | I select a service named "payment-api" in the ECS Services list and press Enter. | The view transitions to the tasks view. A spinner appears centered in the frame with text like "Fetching ECS tasks..." while the API calls are in flight. The ECS Services list is pushed onto the view stack. |
| B.1.2 | The API responds successfully with task data. | The spinner disappears. Tasks are rendered as table rows. The frame title reads `ecs-tasks(6) -- payment-api` where 6 is the task count. |
| B.1.3 | The API responds with an error (e.g., ClusterNotFoundException). | The spinner disappears. A red error flash message appears in the header right side. |
| B.1.4 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. |

**AWS comparison:**
```
aws ecs list-tasks --cluster CLUSTER --service-name SERVICE
aws ecs describe-tasks --cluster CLUSTER --tasks TASK_ARN_1 TASK_ARN_2 ...
```
Expected fields visible: Task ID (task_id_short), Status (LastStatus), Health (HealthStatus), Task Def (task_def_short), Started (StartedAt), Stopped Reason (StoppedReason)

### B.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| B.2.1 | A service is scaled to 0 desired count and has no running or recently stopped tasks. I press Enter on it. | The frame title reads `ecs-tasks(0) -- scaled-down-service`. The content area shows a centered message (e.g., "No tasks found") with a hint to check desired count or refresh. |
| B.2.2 | I press ctrl+r on the empty tasks view. | The loading spinner reappears while the refresh request is in flight. |

### B.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| B.3.1 | Tasks load and the table renders. | Six columns are displayed: "Task ID" (width 14), "Status" (width 10), "Health" (width 10), "Task Def" (width 20), "Started" (width 22), "Stopped Reason" (width 36). Column headers are bold, colored blue (#7aa2f7), with no separator line below. |
| B.3.2 | I verify the Task ID column. | The Task ID is a short identifier: the last 8 characters extracted from the full task ARN (e.g., "a1b2c3d4" from a long ARN). This is a computed field (`task_id_short`). |
| B.3.3 | I verify the Task Def column. | The Task Def shows "family:revision" extracted from the full TaskDefinitionArn (e.g., "payment-api:47"). This is a computed field (`task_def_short`). |
| B.3.4 | I verify the Status column against `aws ecs describe-tasks`. | The Status maps to `.Tasks[].LastStatus`. Values include RUNNING, STOPPED, PENDING, PROVISIONING, DEACTIVATING, STOPPING. |
| B.3.5 | I verify the Health column. | The Health maps to `.Tasks[].HealthStatus`. Values include HEALTHY, UNHEALTHY, UNKNOWN, or empty/dash for tasks without health checks. |
| B.3.6 | I verify the Started column. | The Started column maps to `.Tasks[].StartedAt`. For PENDING tasks that have not yet started, this field is empty/dash. |
| B.3.7 | I verify the Stopped Reason column. | The Stopped Reason maps to `.Tasks[].StoppedReason`. For running tasks, this field is empty/dash. |
| B.3.8 | The total column width (14+10+10+20+22+36 = 112 plus borders/padding) exceeds the terminal width (80 columns). | Rightmost columns are hidden (Stopped Reason, then Started). I can scroll horizontally with h and l keys to reveal them. |
| B.3.9 | I press l (or right-arrow) to scroll right. | The visible column window shifts. The Stopped Reason column becomes visible. Column headers scroll in sync with data columns. |

**AWS comparison:**
```
aws ecs describe-tasks --cluster CLUSTER --tasks $(aws ecs list-tasks --cluster CLUSTER --service-name SERVICE --query 'taskArns' --output text) --query 'tasks[].{TaskArn:taskArn,LastStatus:lastStatus,HealthStatus:healthStatus,TaskDefinitionArn:taskDefinitionArn,StartedAt:startedAt,StoppedReason:stoppedReason}'
```

### B.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| B.4.1 | 6 tasks are loaded for service "payment-api". | The frame top border shows the title centered: `ecs-tasks(6) -- payment-api` with equal-length dashes on both sides. |
| B.4.2 | A filter is active and matches 3 of 6 tasks (e.g., only RUNNING). | The frame title reads `ecs-tasks(3/6) -- payment-api`. |
| B.4.3 | A filter is active and matches 0 tasks. | The frame title reads `ecs-tasks(0/6) -- payment-api`. The content area is empty. |

### B.5 Row Coloring (Status-Based)

| ID | Story | Expected |
|----|-------|----------|
| B.5.1 | A task has LastStatus=RUNNING and HealthStatus=HEALTHY. | The entire row is rendered in GREEN (#9ece6a). |
| B.5.2 | A task has LastStatus=RUNNING and HealthStatus=UNHEALTHY. | The entire row is rendered in RED (#f7768e). |
| B.5.3 | A task has LastStatus=RUNNING with no health status (empty or UNKNOWN). | The entire row is rendered in GREEN (#9ece6a). |
| B.5.4 | A task has LastStatus=PENDING. | The entire row is rendered in YELLOW (#e0af68). |
| B.5.5 | A task has LastStatus=PROVISIONING. | The entire row is rendered in YELLOW (#e0af68). |
| B.5.6 | A task has LastStatus=ACTIVATING. | The entire row is rendered in YELLOW (#e0af68). |
| B.5.7 | A task has LastStatus=DEACTIVATING. | The entire row is rendered in YELLOW (#e0af68). |
| B.5.8 | A task has LastStatus=STOPPING. | The entire row is rendered in YELLOW (#e0af68). |
| B.5.9 | A task has LastStatus=STOPPED with StoppedReason containing "OOM" (out of memory). | The entire row is rendered in RED (#f7768e). |
| B.5.10 | A task has LastStatus=STOPPED with StoppedReason containing "Essential container in task exited". | The entire row is rendered in RED (#f7768e) because the reason contains "exited". |
| B.5.11 | A task has LastStatus=STOPPED with StoppedReason containing "error". | The entire row is rendered in RED (#f7768e). |
| B.5.12 | A task has LastStatus=STOPPED with StoppedReason containing "failed". | The entire row is rendered in RED (#f7768e). |
| B.5.13 | A task has LastStatus=STOPPED with StoppedReason "Scaling activity initiated by deployment". | The entire row is rendered in DIM (#565f89) -- this is a normal stop due to scaling/deployment. |
| B.5.14 | A task has LastStatus=STOPPED with StoppedReason "Task stopped by user". | The entire row is rendered in DIM (#565f89) -- normal intentional stop. |
| B.5.15 | I select a RED-colored STOPPED task. | The selected row gains full-width blue background (#7aa2f7) with dark foreground (#1a1b26), bold. The red coloring is overridden by the selection highlight. |
| B.5.16 | I move selection away from that row. | The row reverts to its RED status-based coloring. |
| B.5.17 | A mix of RUNNING (green), PENDING (yellow), and STOPPED (red/dim) tasks are displayed. | Each row independently reflects its status color. The list visually communicates the health of the service at a glance. |

### B.6 Navigation

| ID | Story | Expected |
|----|-------|----------|
| B.6.1 | I press j (or down-arrow) with the first task selected. | The selection cursor moves to the second task. |
| B.6.2 | I press k (or up-arrow) with the second task selected. | The selection cursor moves back to the first task. |
| B.6.3 | I press g. | The selection jumps to the very first task. |
| B.6.4 | I press G. | The selection jumps to the very last task. |
| B.6.5 | I press PageDown (or ctrl+d). | The selection moves down by one page of visible rows. |
| B.6.6 | I press PageUp (or ctrl+u). | The selection moves up by one page of visible rows. |
| B.6.7 | I press h (or left-arrow). | Columns scroll left (visible column window shifts), revealing any previously hidden left columns. |
| B.6.8 | I press l (or right-arrow). | Columns scroll right, revealing any previously hidden right columns such as Stopped Reason. |
| B.6.9 | There are more tasks than fit on screen. I scroll past the visible area. | The table scrolls to keep the selected row visible. Column headers remain in place. |

### B.7 Horizontal Scroll

| ID | Story | Expected |
|----|-------|----------|
| B.7.1 | Terminal width is 120 columns and all six columns fit. | No horizontal scrolling needed. h/l does nothing visible. |
| B.7.2 | Terminal width is 80 columns. Columns "Task ID" through "Task Def" are visible (14+10+10+20 = 54 plus borders). "Started" and "Stopped Reason" are hidden. | Pressing l reveals "Started" while hiding "Task ID". Pressing l again reveals "Stopped Reason". Pressing h reverses the scroll. |
| B.7.3 | Column headers scroll in sync with data when I press h/l. | The column header row shifts horizontally by the same offset as data rows. |

### B.8 Sorting

| ID | Story | Expected |
|----|-------|----------|
| B.8.1 | I press N on the tasks list. | Rows are sorted by Task ID alphabetically ascending. The "Task ID" column header shows a sort indicator (up-arrow). |
| B.8.2 | I press N again. | Sort order toggles to descending. The indicator changes to a down-arrow. |
| B.8.3 | I press S on the tasks list. | Rows are sorted by Status. The "Status" column header shows the up-arrow indicator. |
| B.8.4 | I press A on the tasks list. | Rows are sorted by Started date (age) ascending (oldest first). The "Started" column header shows the up-arrow indicator. |

### B.9 Filter

| ID | Story | Expected |
|----|-------|----------|
| B.9.1 | I press / and type "RUNNING". | Only tasks with RUNNING status (or any field containing "RUNNING") are shown. The frame title updates to `ecs-tasks(M/N) -- payment-api`. |
| B.9.2 | I press / and type "STOPPED". | Only stopped tasks are shown. These should be RED or DIM colored rows. |
| B.9.3 | I press / and type the first few characters of a task ID. | Only the matching task is shown. |
| B.9.4 | I press Escape while filter is active. | The filter clears. All tasks reappear. |
| B.9.5 | I type "OOM" to find tasks that crashed with out-of-memory. | Only tasks whose Stopped Reason contains "OOM" are shown. |

### B.10 Copy (c)

| ID | Story | Expected |
|----|-------|----------|
| B.10.1 | I select a task and press c. | The full task ARN is copied to the system clipboard. A green flash message "Copied!" appears in the header right side. |
| B.10.2 | After approximately 2 seconds. | The "Copied!" flash message auto-clears and the header right reverts to "? for help". |
| B.10.3 | I paste from clipboard into another application. | The pasted text is the full ARN (e.g., "arn:aws:ecs:us-east-1:123456789012:task/my-cluster/a1b2c3d4e5f67890"), not the short task ID displayed in the list. |

**AWS comparison:**
```
aws ecs list-tasks --cluster CLUSTER --service-name SERVICE --query 'taskArns[0]' --output text
```

### B.11 Detail View (d)

| ID | Story | Expected |
|----|-------|----------|
| B.11.1 | I select a task and press d. | The detail view opens. The frame title shows the task identifier. |
| B.11.2 | I verify the displayed fields. | The detail view shows key-value pairs for all 21 fields: TaskArn, LastStatus, DesiredStatus, HealthStatus, TaskDefinitionArn, StartedAt, StoppedAt, StoppedReason, StopCode, StartedBy, Group, LaunchType, PlatformVersion, Cpu, Memory, Connectivity, Containers, Attachments, AvailabilityZone, CreatedAt, Tags. |
| B.11.3 | I compare TaskArn with expected format. | The ARN follows the pattern `arn:aws:ecs:REGION:ACCOUNT:task/CLUSTER/TASK_ID`. |
| B.11.4 | A RUNNING task has no StoppedAt or StoppedReason. | These fields display null/empty/dash rather than crashing. |
| B.11.5 | A STOPPED task has both StoppedAt and StoppedReason populated. | StoppedAt shows a timestamp. StoppedReason shows the reason text (e.g., "Essential container in task exited", "Scaling activity initiated by deployment"). |
| B.11.6 | StopCode is present for a stopped task. | StopCode shows values like "TaskFailedToStart", "EssentialContainerExited", or "UserInitiated". |
| B.11.7 | The Containers section in detail. | Containers is rendered as a section header with sub-fields for each container (name, image, status, exit code). |
| B.11.8 | A task was started by ECS deployment vs. manual run. | The StartedBy field shows the deployment ID or "ecs-svc/..." for deployment-started tasks. |
| B.11.9 | I press w in the detail view. | Word wrap toggles. Long values like TaskArn or StoppedReason wrap to the next line. |
| B.11.10 | I press Escape in the detail view. | I return to the tasks list. The cursor position is preserved. |

**AWS comparison:**
```
aws ecs describe-tasks --cluster CLUSTER --tasks TASK_ARN --query 'tasks[0]'
```
Expected fields visible: TaskArn, LastStatus, DesiredStatus, HealthStatus, TaskDefinitionArn, StartedAt, StoppedAt, StoppedReason, StopCode, StartedBy, Group, LaunchType, PlatformVersion, Cpu, Memory, Connectivity, Containers, Attachments, AvailabilityZone, CreatedAt, Tags

### B.12 YAML View (y)

| ID | Story | Expected |
|----|-------|----------|
| B.12.1 | I select a task and press y. | The YAML view opens. The frame title includes the task identifier and "yaml". The full task resource is rendered as syntax-highlighted YAML. |
| B.12.2 | The YAML content includes nested structures (Containers, Attachments). | Nested YAML keys are properly indented. Tree connectors (dim vertical lines) appear for nested structures. |
| B.12.3 | I press Escape on the YAML view. | I return to the tasks list. |

### B.13 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| B.13.1 | I press ctrl+r on the tasks list. | The loading spinner appears. Fresh ListTasks + DescribeTasks calls are made. The table repopulates with current data. |
| B.13.2 | A new deployment is in progress. Old tasks are stopping and new tasks are starting. I press ctrl+r. | The refreshed list shows the updated status of all tasks. New PENDING/RUNNING tasks appear. STOPPED tasks from the old deployment are visible. |
| B.13.3 | I had a filter active and press ctrl+r. | The data refreshes. The filter remains applied. The frame title count updates. |

### B.14 Escape (Back)

| ID | Story | Expected |
|----|-------|----------|
| B.14.1 | I press Escape on the tasks list (not in filter or detail). | I return to the ECS Services list. The cursor position is preserved on the same service. |

### B.15 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| B.15.1 | I press ? on the tasks list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: ECS TASKS, GENERAL, NAVIGATION, HOTKEYS. |
| B.15.2 | I verify the help screen content. | ECS TASKS column shows: `<esc>` Back, `<d>` Detail, `<y>` YAML, `<c>` Copy ARN. GENERAL column shows: `<ctrl-r>` Refresh, `</>` Filter, `<:>` Command. NAVIGATION column shows: `<j>` Down, `<k>` Up, `<g>` Top, `<G>` Bottom, `<h/l>` Cols, `<pgup/dn>` Page. HOTKEYS column shows: `<?>` Help, `<:>` Command. |
| B.15.3 | I press any key on the help screen. | The help screen closes and the tasks list reappears. |

### B.16 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| B.16.1 | I press : on the tasks list. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| B.16.2 | I type "s3" and press Enter. | The view navigates to the S3 Buckets list. |
| B.16.3 | I press Escape in command mode. | Command mode is cancelled. The tasks list remains. |

### B.17 Deployment Scenarios

| ID | Story | Expected |
|----|-------|----------|
| B.17.1 | A rolling deployment is in progress. 3 tasks run task-def revision 47 and 3 tasks run revision 46. | The Task Def column shows both "payment-api:47" and "payment-api:46". Tasks with the new revision may show PENDING/PROVISIONING (yellow). Tasks with the old revision may show STOPPING (yellow) or STOPPED (red/dim). |
| B.17.2 | I filter by "payment-api:46" to see only old-revision tasks. | Only tasks running the old task definition revision are displayed. |
| B.17.3 | I filter by "payment-api:47" to see only new-revision tasks. | Only tasks running the new task definition revision are displayed. |

### B.18 Task Health Scenarios

| ID | Story | Expected |
|----|-------|----------|
| B.18.1 | A task is RUNNING but its health check has failed (HealthStatus=UNHEALTHY). | The Health column shows "UNHEALTHY". The entire row is RED (#f7768e) -- this is a critical signal that the container is up but not healthy. |
| B.18.2 | A task is RUNNING with HealthStatus=UNKNOWN (health check not yet evaluated). | The Health column shows "UNKNOWN". The row is GREEN because LastStatus is RUNNING and health is not confirmed unhealthy. |
| B.18.3 | A task does not have a health check configured. | The Health column shows empty/dash. The row is GREEN because LastStatus is RUNNING. |

### B.19 OOM and Crash Scenarios

| ID | Story | Expected |
|----|-------|----------|
| B.19.1 | A task stopped with StoppedReason "OutOfMemoryError: Container killed due to memory usage". | The row is RED (#f7768e). The Stopped Reason column shows as much of the reason as fits in 36 characters, truncated with ellipsis. |
| B.19.2 | I press d on the OOM-stopped task. | The detail view shows the full StoppedReason without truncation. StopCode shows "EssentialContainerExited" or similar. |
| B.19.3 | A task is in a crash loop (repeatedly starting and stopping). | Multiple STOPPED tasks with recent timestamps appear alongside PENDING/RUNNING tasks. The visual mix of RED/DIM stopped rows and YELLOW/GREEN active rows signals the instability. |

---

## C. ECS Container Logs View

### C.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | I select a service named "payment-api" in the ECS Services list and press `L` (uppercase). | The view transitions to the container logs view. A spinner appears centered in the frame with text like "Fetching container logs..." while the cross-service API calls are in flight (DescribeTaskDefinition + FilterLogEvents). The ECS Services list is pushed onto the view stack. |
| C.1.2 | The API responds successfully with log data. | The spinner disappears. Log lines are rendered as table rows with columns: Timestamp, Stream, Message. The frame title reads `ecs-logs(100) -- payment-api`. |
| C.1.3 | The API responds with an error on DescribeTaskDefinition (e.g., task definition not found). | The spinner disappears. A red error flash message appears in the header right side. |
| C.1.4 | The API responds with an error on FilterLogEvents (e.g., log group does not exist). | The spinner disappears. A red error flash message appears (e.g., "Error: ResourceNotFoundException"). |
| C.1.5 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued. |

**AWS comparison:**
```
aws ecs describe-task-definition --task-definition TASK_DEF_ARN --query 'taskDefinition.containerDefinitions[0].logConfiguration'
aws logs filter-log-events --log-group-name LOG_GROUP --log-stream-name-prefix STREAM_PREFIX --limit 100
```
Expected fields visible: Timestamp, Stream (stream_short), Message

### C.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | A newly deployed service has not yet produced any log output. I press `L` on it. | The frame title reads `ecs-logs(0) -- new-service`. The content area shows a centered message (e.g., "No log events found") with a hint to wait for the service to produce output or refresh. |
| C.2.2 | I press ctrl+r on the empty logs view. | The loading spinner reappears. If the service has produced logs since the initial load, they appear after refresh. |

### C.3 Non-awslogs Driver

| ID | Story | Expected |
|----|-------|----------|
| C.3.1 | A service's task definition uses the `firelens` log driver instead of `awslogs`. I press `L` on it. | Instead of the normal log table, a message is displayed: "Log driver is not awslogs. Logs not available in CloudWatch." The frame title still shows the service name. |
| C.3.2 | A service's task definition uses the `fluentd` log driver. I press `L` on it. | Same behavior as C.3.1 -- the message indicates logs are not available in CloudWatch. |
| C.3.3 | A service's task definition has no logConfiguration at all. I press `L` on it. | A message indicates no log configuration was found. The view does not crash. |

### C.4 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| C.4.1 | Logs load and the table renders. | Three columns are displayed: "Timestamp" (width 22), "Stream" (width 20), "Message" (width 0, fills remaining terminal width). Column headers are bold, colored blue (#7aa2f7), with no separator line below. |
| C.4.2 | I verify the Timestamp column. | The Timestamp maps to the log event timestamp from FilterLogEvents. Format is a human-readable datetime. |
| C.4.3 | I verify the Stream column. | The Stream shows a short version of the log stream name (`stream_short` computed field), extracting the container-name/task-id-short suffix (e.g., "app/a1b2c3d4" from "ecs/payment-api/app/a1b2c3d4e5f67890"). |
| C.4.4 | I verify the Message column. | The Message maps to the log event message from FilterLogEvents. It fills all remaining width. Long messages are truncated in the list. |
| C.4.5 | The terminal is very wide (160+ columns). | The Message column expands to fill all available space. More of each log message is visible without truncation. |
| C.4.6 | The terminal is narrow (80 columns). | The Message column gets whatever space remains after Timestamp (22) and Stream (20) plus borders. If insufficient, the Message column may be very narrow or hidden, scrollable with h/l. |

**AWS comparison:**
```
aws logs filter-log-events --log-group-name /ecs/payment-api --log-stream-name-prefix ecs/payment-api --limit 100 --query 'events[].{timestamp:timestamp,logStreamName:logStreamName,message:message}'
```

### C.5 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| C.5.1 | 100 log events are loaded for service "payment-api". | The frame top border shows the title centered: `ecs-logs(100) -- payment-api` with equal-length dashes on both sides. |
| C.5.2 | A filter is active matching 12 of 100 log events. | The frame title reads `ecs-logs(12/100) -- payment-api`. |
| C.5.3 | A filter is active matching 0 events. | The frame title reads `ecs-logs(0/100) -- payment-api`. The content area is empty. |

### C.6 Row Coloring (Content-Based)

| ID | Story | Expected |
|----|-------|----------|
| C.6.1 | A log message contains `"level":"error"` (JSON structured log). | The entire row is rendered in RED (#f7768e). |
| C.6.2 | A log message contains `"level":"fatal"`. | The entire row is rendered in RED (#f7768e). |
| C.6.3 | A log message contains the word `ERROR` (plain text log). | The entire row is rendered in RED (#f7768e). |
| C.6.4 | A log message contains the word `FATAL`. | The entire row is rendered in RED (#f7768e). |
| C.6.5 | A log message contains `"level":"warn"` (JSON structured log). | The entire row is rendered in YELLOW (#e0af68). |
| C.6.6 | A log message contains the word `WARN`. | The entire row is rendered in YELLOW (#e0af68). |
| C.6.7 | A log message is a normal info-level log (e.g., `"level":"info"`). | The entire row is rendered in PLAIN text color (#c0caf5). |
| C.6.8 | A log message contains "error" inside a URL or field name (e.g., "error_count: 0" or "/api/errors"). | The entire row is rendered in RED (#f7768e) -- content matching is substring-based. |
| C.6.9 | I select a RED-colored error log line. | The selected row gains full-width blue background (#7aa2f7) with dark foreground (#1a1b26), bold. The red coloring is overridden. |
| C.6.10 | I move selection away from that row. | The row reverts to its RED content-based coloring. |
| C.6.11 | Alternating rows with different content-based colors (info, warn, error intermixed). | Each row independently reflects its content-based color. Alternating row background (#1e2030) provides subtle differentiation. |

### C.7 Navigation

| ID | Story | Expected |
|----|-------|----------|
| C.7.1 | I press j (or down-arrow) with the first log line selected. | The selection cursor moves to the second log line. |
| C.7.2 | I press k (or up-arrow) with the second log line selected. | The selection cursor moves back to the first log line. |
| C.7.3 | I press g. | The selection jumps to the first log line (most recent). |
| C.7.4 | I press G. | The selection jumps to the last log line (oldest in the batch). |
| C.7.5 | I press PageDown (or ctrl+d). | The selection moves down by one page. |
| C.7.6 | I press PageUp (or ctrl+u). | The selection moves up by one page. |
| C.7.7 | I press h (or left-arrow). | Columns scroll left. |
| C.7.8 | I press l (or right-arrow). | Columns scroll right, potentially revealing more of the Message column if it was truncated at the right edge. |
| C.7.9 | There are more log lines than fit on screen. I scroll past the visible area. | The table scrolls to keep the selected row visible. Column headers remain in place. |

### C.8 Sorting

| ID | Story | Expected |
|----|-------|----------|
| C.8.1 | I press N on the logs list. | Rows are sorted by the Stream column alphabetically ascending. The "Stream" column header shows a sort indicator. |
| C.8.2 | I press A on the logs list. | Rows are sorted by Timestamp ascending (oldest first). The "Timestamp" column header shows the up-arrow indicator. |
| C.8.3 | I press A again. | Sort order toggles to descending (newest first). |

### C.9 Filter

| ID | Story | Expected |
|----|-------|----------|
| C.9.1 | I press / and type "error". | Only log lines whose message, stream, or timestamp contains "error" (case-insensitive) are shown. The frame title updates to `ecs-logs(M/N) -- payment-api`. |
| C.9.2 | I press / and type a container name/task-id prefix (e.g., "a1b2c3d4"). | Only log lines from that specific task are shown. |
| C.9.3 | I press / and type "Database connection" to search for a specific error. | Only matching log lines are shown. |
| C.9.4 | I press Escape while filter is active. | The filter clears. All log lines reappear. |
| C.9.5 | I type "ERROR" (uppercase) and log lines contain both "ERROR" and "error". | Both forms match. Filtering is case-insensitive. |

### C.10 Copy (c)

| ID | Story | Expected |
|----|-------|----------|
| C.10.1 | I select a log line and press c. | The full message text of the selected log line is copied to the system clipboard. A green "Copied!" flash appears. |
| C.10.2 | After approximately 2 seconds. | The "Copied!" flash message auto-clears. |
| C.10.3 | The log message was a JSON-structured log like `{"level":"error","msg":"Database connection refused","ts":"2026-03-22T02:51:31Z"}`. I paste from clipboard. | The full JSON string is pasted, not the truncated version displayed in the list. |
| C.10.4 | A very long error message with a multi-line stack trace was stored as a single log event. | The full message including all stack trace lines is copied. |

**AWS comparison:**
```
aws logs filter-log-events --log-group-name LOG_GROUP --log-stream-name-prefix PREFIX --limit 1 --query 'events[0].message' --output text
```

### C.11 Word Wrap (w)

| ID | Story | Expected |
|----|-------|----------|
| C.11.1 | I press w on the logs list view. | Word wrap toggles for the Message column. Long messages that were previously truncated now wrap to the next line, making the full message readable in the list without opening detail view. |
| C.11.2 | I press w again. | Word wrap toggles off. Messages revert to single-line truncated display. |
| C.11.3 | A JSON log message is 200+ characters. Word wrap is enabled. | The message wraps across multiple visual lines. The row height increases to accommodate the wrapped text. |

### C.12 Detail View (d)

| ID | Story | Expected |
|----|-------|----------|
| C.12.1 | I select a log line and press d. | The detail view opens. The frame title shows identifying info for the log event. |
| C.12.2 | I verify the displayed fields. | The detail view shows key-value pairs for: Timestamp, IngestionTime, LogStreamName, Message, EventId. These match the `ecs_svc_logs.detail` configuration. |
| C.12.3 | The Timestamp field in detail. | Shows the full timestamp of when the log event was emitted. |
| C.12.4 | The IngestionTime field in detail. | Shows when CloudWatch Logs ingested the event (may differ from Timestamp). |
| C.12.5 | The LogStreamName field in detail. | Shows the full log stream name (e.g., "ecs/payment-api/app/a1b2c3d4e5f67890"), not the shortened `stream_short` version. |
| C.12.6 | The Message field in detail contains a very long JSON log line. | The full message is displayed. Pressing `w` toggles word wrap so the entire JSON is readable. |
| C.12.7 | The EventId field. | Shows a unique identifier for the log event assigned by CloudWatch Logs. |
| C.12.8 | I press Escape in the detail view. | I return to the logs list. The cursor position is preserved. |

**AWS comparison:**
```
aws logs filter-log-events --log-group-name LOG_GROUP --limit 1 --query 'events[0].{timestamp:timestamp,ingestionTime:ingestionTime,logStreamName:logStreamName,message:message,eventId:eventId}'
```
Expected fields visible: Timestamp, IngestionTime, LogStreamName, Message, EventId

### C.13 YAML View (y)

| ID | Story | Expected |
|----|-------|----------|
| C.13.1 | I select a log line and press y. | The YAML view opens. The frame title includes "yaml". The full log event is rendered as syntax-highlighted YAML. |
| C.13.2 | The Message field in YAML. | The message string value is rendered in green (#9ece6a). If it's a long string, it may use YAML multi-line formatting. |
| C.13.3 | I press Escape on the YAML view. | I return to the logs list. |

### C.14 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| C.14.1 | I press ctrl+r on the logs list. | The loading spinner appears. Fresh DescribeTaskDefinition + FilterLogEvents calls are made. The table repopulates with current log data. |
| C.14.2 | The service has produced new log output since the last load. I press ctrl+r. | The new log lines appear in the refreshed list. The count in the frame title updates. |
| C.14.3 | I had a filter active and press ctrl+r. | The data refreshes. The filter remains applied. |

### C.15 Escape (Back)

| ID | Story | Expected |
|----|-------|----------|
| C.15.1 | I press Escape on the logs list (not in filter or detail). | I return to the ECS Services list. The cursor position is preserved on the same service. |

### C.16 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| C.16.1 | I press ? on the logs list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: CONTAINER LOGS, GENERAL, NAVIGATION, HOTKEYS. |
| C.16.2 | I verify the help screen content. | CONTAINER LOGS column shows: `<esc>` Back, `<d>` Detail, `<y>` YAML, `<c>` Copy Message, `<w>` Word Wrap. GENERAL column shows: `<ctrl-r>` Refresh, `</>` Filter, `<:>` Command. NAVIGATION column shows: `<j>` Down, `<k>` Up, `<g>` Top, `<G>` Bottom, `<h/l>` Cols, `<pgup/dn>` Page. HOTKEYS column shows: `<?>` Help, `<:>` Command. |
| C.16.3 | I press any key on the help screen. | The help screen closes and the logs list reappears. |

### C.17 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| C.17.1 | I press : on the logs list. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| C.17.2 | I type "rds" and press Enter. | The view navigates to the RDS instances list. |
| C.17.3 | I press Escape in command mode. | Command mode is cancelled. The logs list remains. |

### C.18 Multi-Container Task

| ID | Story | Expected |
|----|-------|----------|
| C.18.1 | A service's task definition has two containers: "app" (primary) and "sidecar" (e.g., envoy proxy). I press `L`. | Logs are fetched from the primary (first) container's log group. The Stream column shows the container name prefix (e.g., "app/a1b2c3d4"). |
| C.18.2 | I want to see sidecar logs. | The current design fetches logs from the primary container's log configuration only. Sidecar logs require navigating to the CloudWatch log group directly (via the `:logs` command or other resource). |

---

## D. Cross-Cutting: ECS Services Parent Key Bindings

### D.1 Key Binding Differentiation

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | I am on the ECS Services list. I press Enter. | I navigate to the Tasks child view (the default drill-down, answering "what is running?"). |
| D.1.2 | I am on the ECS Services list. I press `e`. | I navigate to the Service Events child view (answering "what happened?"). |
| D.1.3 | I am on the ECS Services list. I press `L` (uppercase). | I navigate to the Container Logs child view (answering "what is the service outputting?"). |
| D.1.4 | I am on the ECS Services list. I press `d`. | I navigate to the standard ECS Service detail view (key-value fields for the service itself), NOT to any of the child views. |
| D.1.5 | I am on the ECS Services list. I press `y`. | I navigate to the YAML view for the selected ECS service, NOT to any child view. |
| D.1.6 | I am on the ECS Services list. I press `l` (lowercase). | Columns scroll right (horizontal scroll), NOT container logs. `L` (uppercase/shift) is distinct from `l` (lowercase). |

### D.2 Help Screen Updates on ECS Services

| ID | Story | Expected |
|----|-------|----------|
| D.2.1 | I press ? on the ECS Services list. | The help screen shows the three child view key bindings: `<Enter>` Tasks, `<e>` Events, `<L>` Logs, in addition to the standard keys (`<d>` Detail, `<y>` YAML, `<c>` Copy, etc.). |

---

## E. Cross-Cutting: View Stack and Navigation

### E.1 View Stack Depth

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | Main Menu -> ECS Services -> Tasks (Enter) -> Task Detail (d) -> YAML (y); then Escape four times. | Each Escape pops one level: YAML -> Task Detail -> Tasks -> ECS Services -> Main Menu. No state is lost at any intermediate level. Cursor positions are preserved. |
| E.1.2 | Main Menu -> ECS Services -> Events (e) -> Event Detail (d); then Escape twice. | Event Detail -> Events -> ECS Services. The cursor is on the same service in the ECS Services list. |
| E.1.3 | Main Menu -> ECS Services -> Logs (L) -> Log Detail (d) -> YAML (y); then Escape four times. | YAML -> Log Detail -> Logs -> ECS Services -> Main Menu. |

### E.2 Switching Between Child Views

| ID | Story | Expected |
|----|-------|----------|
| E.2.1 | I enter the Tasks view (Enter) for "payment-api". I press Escape to go back to ECS Services. I then press `e` on the same service. | I navigate to the Events view for "payment-api". The Tasks view is no longer on the stack (it was popped by Escape). |
| E.2.2 | I enter the Events view (`e`). I press Escape. I then press `L` on the same service. | I navigate to the Container Logs view. Each child view is independently accessed from the parent. |
| E.2.3 | I enter the Tasks view. Inside Tasks, I press `e`. | `e` is not a recognized key binding in the Tasks child view. Nothing happens. The `e` key for Events only works from the ECS Services parent list. |
| E.2.4 | I enter the Logs view. Inside Logs, I press Enter. | Enter either does nothing or opens the detail for the selected log line (depending on key binding), but does NOT navigate to the Tasks view. Child view entry keys only work from the ECS Services parent list. |

### E.3 Header Consistency

| ID | Story | Expected |
|----|-------|----------|
| E.3.1 | In the Events view, the header displays: "a9s" (accent bold), version (dim), profile:region (bold). | Same header format as all other views. |
| E.3.2 | In the Tasks view, the header right side shows "? for help" in normal mode. | Confirmed same behavior as all other list views. |
| E.3.3 | In the Logs view, when I press / to filter, the header right changes to filter mode. | Header right shows "/text|" (amber/bold) -- same behavior as all other views. |
| E.3.4 | In any child view, when I press c to copy, a green "Copied!" flash appears. | Same flash behavior across all three child views. Auto-clears after approximately 2 seconds. |

---

## F. Cross-Cutting: Terminal Resize

### F.1 Resize During Child Views

| ID | Story | Expected |
|----|-------|----------|
| F.1.1 | I resize the terminal while viewing ECS Tasks. | The layout reflows. Column visibility adjusts to the new width. The frame border redraws correctly. If columns no longer fit, they become scrollable with h/l. |
| F.1.2 | I resize the terminal to below 60 columns while in Events view. | An error message appears: "Terminal too narrow. Please resize." |
| F.1.3 | I resize the terminal to below 7 lines while in Logs view. | An error message appears: "Terminal too short. Please resize." |
| F.1.4 | I resize the terminal while in the Task Detail view. | The viewport adjusts to the new dimensions. Content reflows appropriately. |
| F.1.5 | I resize the terminal to make it wider while in Logs view. | The Message column (width 0) expands to fill the newly available space. More of each log message becomes visible. |

---

## G. Cross-Cutting: Alternating Row Colors

### G.1 Alternating Rows in Child Views

| ID | Story | Expected |
|----|-------|----------|
| G.1.1 | The Events list has more than 2 rows. | Alternating rows have a subtle background color difference (#1e2030) for readability. Content-based coloring (green/red/yellow) applies as foreground text on top of alternating backgrounds. Selected row always has blue background regardless. |
| G.1.2 | The Tasks list has more than 2 rows. | Same alternating row pattern applies. Status-based coloring (green/red/yellow/dim) is overlaid. |
| G.1.3 | The Logs list has more than 2 rows. | Same alternating row pattern applies. Content-based coloring (red for errors, yellow for warnings) is overlaid. |

---

## H. Edge Cases and Error Scenarios

### H.1 Service Context

| ID | Story | Expected |
|----|-------|----------|
| H.1.1 | I select a service in the ECS Services list, press Enter, view tasks, press Escape, then select a DIFFERENT service and press Enter. | The Tasks view loads tasks for the newly selected service, not the previous one. The frame title shows the new service name. |
| H.1.2 | I press `e` on a service, view events, press Escape, then press `L` on the same service. | The Logs view loads correctly for the same service. Each child view independently fetches its data. |

### H.2 Very Long Values

| ID | Story | Expected |
|----|-------|----------|
| H.2.1 | An event message is 500+ characters (common for "unable to place" errors with constraint details). | The message is truncated in the list view. Pressing `c` copies the full message. Pressing `d` shows the full message in detail view with word wrap available via `w`. |
| H.2.2 | A task's StoppedReason is 200+ characters (e.g., detailed ECS service crash reason). | The StoppedReason column (width 36) truncates the value. The full text is visible in the detail view and via clipboard copy. |
| H.2.3 | A log message is a multi-line JSON object serialized as a single line (500+ characters). | Truncated in the list. Full text available via detail view. Word wrap (`w`) in the list view may help, and word wrap in detail view definitely shows the full text. |
| H.2.4 | A log message contains a Java stack trace spanning many lines but stored as a single CloudWatch event. | The full stack trace is preserved in the message field. Visible in detail view with word wrap enabled. `c` copies the entire stack trace. |

### H.3 Timing and Latency

| ID | Story | Expected |
|----|-------|----------|
| H.3.1 | Events load quickly (<1 second) because DescribeServices returns events inline. | The spinner appears briefly and the events table renders almost instantly. |
| H.3.2 | Tasks load in approximately 1-2 seconds (two API calls: ListTasks + DescribeTasks). | The spinner is visible for 1-2 seconds. The user sees a smooth transition from spinner to table. |
| H.3.3 | Container Logs load in 2-4 seconds (DescribeTaskDefinition + FilterLogEvents on a busy service). | The spinner is visible for 2-4 seconds. This is the slowest of the three child views. |
| H.3.4 | I press ctrl+r rapidly multiple times. | Each refresh properly cancels or supersedes the previous one. The view does not display stale or mixed data. |
