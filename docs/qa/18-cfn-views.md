# QA User Stories: CloudFormation Child Views

Covers the CFN Stack Events child view (Issue #33) and CFN Stack Resources
child view (Issue #34). Both are accessed from the parent CFN Stacks list.
All stories are written from a black-box perspective against the design spec and
`views.yaml` / child view design documents.

AWS CLI equivalents are cited so testers can verify data parity.

---

## A. CFN Stack Events View (Enter from CFN Stacks)

### A.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I select a stack "payment-service-prod" in the CFN Stacks list and press Enter. | The view transitions to the Stack Events view. A spinner appears centered in the frame with text like "Fetching stack events..." while the `DescribeStackEvents` call is in flight. The frame title shows "cfn-events" with no count. |
| A.1.2 | The API responds with 156 events. | The spinner disappears. Events are rendered as table rows. The frame title updates to "cfn-events(156) -- payment-service-prod". Events appear in reverse chronological order (most recent first). |
| A.1.3 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. The spinner continues uninterrupted. |
| A.1.4 | The API responds with an error (e.g., "Stack does not exist"). | The spinner disappears. A red error flash appears in the header right side (e.g., "Error: Stack does not exist"). |

**AWS comparison:**
```
aws cloudformation describe-stack-events --stack-name payment-service-prod
```
Expected fields visible in list: Timestamp, LogicalResourceId, ResourceType, ResourceStatus, ResourceStatusReason

### A.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | A newly created stack has zero events returned (unlikely but defensive). | The frame title reads "cfn-events(0) -- stack-name". The content area shows a centered message (e.g., "No events found") with a hint to refresh. |
| A.2.2 | I press ctrl+r on the empty state. | The loading spinner appears again while the refresh request is in flight. |

### A.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | Events load and the table renders. | Five columns are displayed: "Timestamp" (width 22), "Logical ID" (width 28), "Type" (width 28), "Status" (width 24), "Reason" (width 40). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| A.3.2 | I verify column data against the AWS CLI output. | "Timestamp" maps to `.StackEvents[].Timestamp`. "Logical ID" maps to `.StackEvents[].LogicalResourceId`. "Type" maps to `.StackEvents[].ResourceType`. "Status" maps to `.StackEvents[].ResourceStatus`. "Reason" maps to `.StackEvents[].ResourceStatusReason`. |
| A.3.3 | A LogicalResourceId is longer than 28 characters (e.g., "MyVeryLongCustomResourceName123"). | The ID is truncated to fit the 28-character column width. No row wrapping occurs. |
| A.3.4 | A ResourceStatusReason is longer than 40 characters (e.g., "The security group 'sg-0abc123' does not exist in VPC 'vpc-456def'"). | The reason is truncated to fit the 40-character column width in the list view. The full text is visible in the detail view. |
| A.3.5 | A ResourceType is a long AWS type like "AWS::CloudFormation::Stack". | The type is displayed within the 28-character column, truncated if it exceeds that width. |
| A.3.6 | The terminal is narrower than the combined column widths (22+28+28+24+40 = 142 plus borders/padding). | The rightmost columns (Status, Reason) are hidden. Horizontal scroll with h/l keys reveals them. Column headers scroll in sync with data. |

**AWS comparison:**
```
aws cloudformation describe-stack-events --stack-name payment-service-prod \
  --query 'StackEvents[].{Timestamp:Timestamp,LogicalId:LogicalResourceId,Type:ResourceType,Status:ResourceStatus,Reason:ResourceStatusReason}'
```

### A.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| A.4.1 | 156 events are loaded for stack "payment-service-prod". | The frame top border shows the title centered: "cfn-events(156) -- payment-service-prod" with equal-length dashes on both sides. |
| A.4.2 | A filter is active and matches 12 of 156 events. | The frame title reads "cfn-events(12/156) -- payment-service-prod". |
| A.4.3 | A filter is active and matches 0 events. | The frame title reads "cfn-events(0/156) -- payment-service-prod". The content area is empty (no rows). |

### A.5 Row Coloring by Status

| ID | Story | Expected |
|----|-------|----------|
| A.5.1 | An event has ResourceStatus "CREATE_COMPLETE". | The entire row is rendered in green (#9ece6a). |
| A.5.2 | An event has ResourceStatus "UPDATE_COMPLETE". | The entire row is rendered in green (#9ece6a). |
| A.5.3 | An event has ResourceStatus "CREATE_IN_PROGRESS". | The entire row is rendered in yellow (#e0af68). |
| A.5.4 | An event has ResourceStatus "UPDATE_IN_PROGRESS". | The entire row is rendered in yellow (#e0af68). |
| A.5.5 | An event has ResourceStatus "DELETE_IN_PROGRESS". | The entire row is rendered in yellow (#e0af68). |
| A.5.6 | An event has ResourceStatus "CREATE_FAILED". | The entire row is rendered in red (#f7768e). |
| A.5.7 | An event has ResourceStatus "UPDATE_FAILED". | The entire row is rendered in red (#f7768e). |
| A.5.8 | An event has ResourceStatus "DELETE_FAILED". | The entire row is rendered in red (#f7768e). |
| A.5.9 | An event has ResourceStatus "ROLLBACK_IN_PROGRESS". | The entire row is rendered in red (#f7768e). ROLLBACK statuses are always red regardless of the suffix. |
| A.5.10 | An event has ResourceStatus "ROLLBACK_COMPLETE". | The entire row is rendered in red (#f7768e). |
| A.5.11 | An event has ResourceStatus "UPDATE_ROLLBACK_IN_PROGRESS". | The entire row is rendered in red (#f7768e). Any status containing ROLLBACK is red. |
| A.5.12 | An event has ResourceStatus "UPDATE_ROLLBACK_COMPLETE". | The entire row is rendered in red (#f7768e). |
| A.5.13 | An event has ResourceStatus "DELETE_COMPLETE". | The entire row is rendered dim (#565f89). |
| A.5.14 | I select a row that is colored red (FAILED). | The selected row has full-width blue background (#7aa2f7) with dark foreground (#1a1b26), bold. The blue selection overrides the red status coloring. |
| A.5.15 | I move the selection away from the red row. | The row reverts to its red status coloring. |

**AWS comparison:**
```
aws cloudformation describe-stack-events --stack-name payment-service-prod \
  --query 'StackEvents[].ResourceStatus' --output text
```
Possible status values: CREATE_IN_PROGRESS, CREATE_COMPLETE, CREATE_FAILED, UPDATE_IN_PROGRESS, UPDATE_COMPLETE, UPDATE_FAILED, DELETE_IN_PROGRESS, DELETE_COMPLETE, DELETE_FAILED, ROLLBACK_IN_PROGRESS, ROLLBACK_COMPLETE, UPDATE_ROLLBACK_IN_PROGRESS, UPDATE_ROLLBACK_COMPLETE, UPDATE_ROLLBACK_COMPLETE_CLEANUP_IN_PROGRESS, IMPORT_IN_PROGRESS, IMPORT_COMPLETE, IMPORT_ROLLBACK_IN_PROGRESS, IMPORT_ROLLBACK_COMPLETE, IMPORT_ROLLBACK_FAILED

### A.6 Navigation

| ID | Story | Expected |
|----|-------|----------|
| A.6.1 | I press j (or down-arrow) with the first event selected. | The selection cursor moves to the second event. The previously selected row loses the blue highlight. The new row gains the full-width blue background with dark foreground, bold. |
| A.6.2 | I press k (or up-arrow) with the second event selected. | The selection cursor moves back to the first event. |
| A.6.3 | I press g. | The selection jumps to the very first event (most recent). |
| A.6.4 | I press G. | The selection jumps to the very last event (oldest loaded). |
| A.6.5 | I press PageDown (or ctrl+d). | The selection moves down by one page of visible rows. |
| A.6.6 | I press PageUp (or ctrl+u). | The selection moves up by one page of visible rows. |
| A.6.7 | There are more events than fit on screen. I scroll past the visible area. | The table scrolls to keep the selected row visible. The column headers remain in place at the top. |

### A.7 Horizontal Scroll

| ID | Story | Expected |
|----|-------|----------|
| A.7.1 | The terminal is 120 columns wide and not all 5 columns fit. | Rightmost columns are hidden (not truncated mid-value). |
| A.7.2 | I press l (or right-arrow). | The visible column window shifts right, revealing "Status" and "Reason" columns while hiding "Timestamp" or other left columns. |
| A.7.3 | I press h (or left-arrow) to scroll back. | The visible column window shifts left, restoring the original leftmost columns. |
| A.7.4 | I scroll right to reveal Status and Reason. | The wireframe shows: Type, Status, Reason columns become visible. Status values like "UPDATE_COMPLETE" and reason like "User Initiated" are readable. |

### A.8 Sorting

| ID | Story | Expected |
|----|-------|----------|
| A.8.1 | I press N on the events list. | Rows are sorted by Logical ID in ascending order. The "Logical ID" column header shows an up-arrow appended directly (e.g., "Logical ID^"). |
| A.8.2 | I press N again. | Sort order toggles to descending. The indicator changes to a down-arrow. |
| A.8.3 | I press A on the events list. | Rows are sorted by Timestamp (age). The "Timestamp" column header shows the sort indicator. |
| A.8.4 | I press S on the events list. | Rows are sorted by Status. The "Status" column header shows the sort indicator. |
| A.8.5 | I sort by name, then apply a filter. | The filtered subset remains sorted. The sort indicator persists on the column header. |
| A.8.6 | I sort by name, then refresh with ctrl+r. | After data reloads, the sort order and direction are preserved. |

### A.9 Filter

| ID | Story | Expected |
|----|-------|----------|
| A.9.1 | I press / on the events list. | The header right side changes from "? for help" to "/\|" (amber/bold, with cursor). Filter mode is active. |
| A.9.2 | I type "FAILED" in filter mode. | Only rows whose visible content contains "FAILED" (case-insensitive) are displayed. This includes events with ResourceStatus containing "FAILED" as well as events with "failed" in the reason. The frame title updates to show matched/total. |
| A.9.3 | I type "TaskDefinition" in filter mode. | Only events with LogicalResourceId containing "TaskDefinition" are displayed. |
| A.9.4 | I press Escape in filter mode. | The filter is cleared. All rows reappear. The frame title reverts to the full count. The header right reverts to "? for help". |
| A.9.5 | I type a filter string that matches no events. | Zero rows are displayed. The frame title shows "cfn-events(0/156) -- stack-name". |

### A.10 Copy Key (c) -- Copies Reason

| ID | Story | Expected |
|----|-------|----------|
| A.10.1 | I select an event that has a ResourceStatusReason "The security group 'sg-0abc123' does not exist in VPC 'vpc-456def'" and press c. | The ResourceStatusReason is copied to the system clipboard. A green flash message "Copied!" appears in the header right side. |
| A.10.2 | After ~2 seconds. | The "Copied!" flash message auto-clears and the header right reverts to "? for help". |
| A.10.3 | I paste from clipboard into Slack. | The pasted text is the full error reason: "The security group 'sg-0abc123' does not exist in VPC 'vpc-456def'". |
| A.10.4 | I select an event that has no ResourceStatusReason (empty or null, e.g., a successful CREATE_COMPLETE). | Pressing c copies the LogicalResourceId instead (the fallback). A "Copied!" flash appears. |
| A.10.5 | I paste from clipboard after copying an event with no reason. | The pasted text is the LogicalResourceId (e.g., "ApiTargetGroup"). |

**AWS comparison:**
```
aws cloudformation describe-stack-events --stack-name payment-service-prod \
  --query 'StackEvents[?ResourceStatus==`CREATE_FAILED`].ResourceStatusReason'
```
The copied text should match the reason string from this query.

### A.11 Detail Key (d)

| ID | Story | Expected |
|----|-------|----------|
| A.11.1 | I select an event and press d. | The detail view opens for the selected event. The frame title shows identifying information for the event. |
| A.11.2 | I verify the detail fields match the cfn_events detail config. | The detail view shows key-value pairs for: Timestamp, LogicalResourceId, PhysicalResourceId, ResourceType, ResourceStatus, ResourceStatusReason, ClientRequestToken, HookType, HookStatus, HookStatusReason, HookInvocationPoint, HookFailureMode, EventId. |
| A.11.3 | The event has a PhysicalResourceId (e.g., "arn:aws:ecs:us-east-1:123456789012:service/my-service"). | The PhysicalResourceId is displayed as a value in the detail view. This is the actual AWS resource ARN. |
| A.11.4 | The event has a ClientRequestToken. | The ClientRequestToken value is displayed. This helps correlate events to specific deployment triggers. |
| A.11.5 | The event has no HookType (null). | The HookType, HookStatus, HookStatusReason, HookInvocationPoint, and HookFailureMode fields show null/empty/dash. |
| A.11.6 | Keys are rendered in blue (#7aa2f7), values in white (#c0caf5). | Visual inspection confirms the key-value color coding from the design spec. |
| A.11.7 | I press Escape on the detail view. | I return to the Stack Events list. The cursor position is preserved on the same event I had selected. |

**AWS comparison:**
```
aws cloudformation describe-stack-events --stack-name payment-service-prod \
  --query 'StackEvents[0]'
```
Expected detail fields: Timestamp, LogicalResourceId, PhysicalResourceId, ResourceType, ResourceStatus, ResourceStatusReason, ClientRequestToken, HookType, HookStatus, HookStatusReason, HookInvocationPoint, HookFailureMode, EventId

### A.12 YAML Key (y)

| ID | Story | Expected |
|----|-------|----------|
| A.12.1 | I select an event and press y. | The YAML view opens. The frame title includes identifying info and "yaml". The full event is rendered as syntax-highlighted YAML. |
| A.12.2 | YAML keys are colored blue (#7aa2f7), string values green (#9ece6a), timestamps green, null values dim (#565f89). | Visual inspection confirms the color coding. |
| A.12.3 | I press Escape on the YAML view. | I return to the Stack Events list. |

### A.13 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| A.13.1 | I press ctrl+r on the events list. | The loading spinner appears. A fresh `DescribeStackEvents` call is made. When it completes, the table repopulates with current data. |
| A.13.2 | A deployment started since the last load, producing new events. I press ctrl+r. | New events appear at the top of the list (most recent first). The count in the frame title increments. |
| A.13.3 | I had a filter active and press ctrl+r. | The data refreshes. The filter remains applied to the new data. The frame title count updates accordingly. |

### A.14 Escape (Back to CFN Stacks)

| ID | Story | Expected |
|----|-------|----------|
| A.14.1 | I press Escape on the Stack Events list. | I return to the CFN Stacks parent list. The cursor is on the same stack I had entered. |

### A.15 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| A.15.1 | I press ? on the events list. | The help screen replaces the table content inside the frame. It shows four columns: STACK EVENTS, GENERAL, NAVIGATION, HOTKEYS. |
| A.15.2 | The STACK EVENTS column shows key bindings. | Listed bindings include: esc (Back), d (Detail), y (YAML), c (Copy Reason). |
| A.15.3 | The GENERAL column shows: ctrl-r (Refresh), / (Filter), : (Command). | These match the standard general bindings. |
| A.15.4 | The NAVIGATION column shows: j (Down), k (Up), g (Top), G (Bottom), h/l (Cols), pgup/dn (Page). | All navigation keys are documented. |
| A.15.5 | The HOTKEYS column shows: ? (Help), : (Command). | Standard hotkeys are listed. |
| A.15.6 | I press any key on the help screen. | The help screen closes and the events list reappears. |

### A.16 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| A.16.1 | I press : on the events list. | The header right side changes to ":\|" (amber/bold). Command mode is active. |
| A.16.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list. |
| A.16.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The events list remains. |

---

## B. CFN Stack Events -- Scenario-Based Stories

### B.1 Active Deployment (IN_PROGRESS)

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | A stack deployment is currently in progress. I press Enter on the stack. | Events load. The most recent event is the stack itself with status "UPDATE_IN_PROGRESS" and reason "User Initiated". Multiple resources show "_IN_PROGRESS" statuses. |
| B.1.2 | I observe the event list during an active deployment. | Yellow rows (IN_PROGRESS) appear at the top of the list (most recent). Green rows (COMPLETE) appear further down from earlier events. The visual pattern gives an at-a-glance progress indicator. |
| B.1.3 | I press ctrl+r during the active deployment. | New events appear at the top. Resources that were IN_PROGRESS may now show as COMPLETE (green). The count may increase as new events are generated. |

**AWS comparison:**
```
aws cloudformation describe-stack-events --stack-name payment-service-prod \
  --query 'StackEvents[?contains(ResourceStatus, `IN_PROGRESS`)]'
```

### B.2 Completed Deployment (All COMPLETE)

| ID | Story | Expected |
|----|-------|----------|
| B.2.1 | A stack deployment completed successfully. I press Enter on the stack. | Events load. The most recent event is the stack itself with "UPDATE_COMPLETE". All resource events below show COMPLETE statuses. |
| B.2.2 | I observe the color pattern. | All rows are green (#9ece6a) for COMPLETE events, with yellow for the initial "UPDATE_IN_PROGRESS" / "User Initiated" event at the bottom of the recent batch. |

### B.3 Failed Deployment with Rollback

| ID | Story | Expected |
|----|-------|----------|
| B.3.1 | A stack deployment failed and rolled back. I press Enter on the stack. | Events load. The chronological sequence shows: first the initial "CREATE_IN_PROGRESS" (yellow), then a "CREATE_FAILED" (red) with an error reason, then "ROLLBACK_IN_PROGRESS" (red), successful rollback cleanups, and finally "ROLLBACK_COMPLETE" (red). |
| B.3.2 | I look at a ROLLBACK_COMPLETE event at the top. | The entire row is red. The reason column shows a summary like "The following resource(s) failed to create: [DatabaseSubnetGroup]". |
| B.3.3 | I look at a CREATE_FAILED event. | The entire row is red. The reason column shows the specific error, e.g., "The security group 'sg-0abc123' does not exist in VPC 'vpc-456def'". |
| B.3.4 | I select the CREATE_FAILED event and press c. | The error reason text is copied to the clipboard. This is the key debugging information. |
| B.3.5 | I filter with "/" and type "FAILED". | Only the events with FAILED in their status are shown. This quickly isolates the root cause events from the rollback noise. |
| B.3.6 | I observe UPDATE_ROLLBACK_IN_PROGRESS events. | These appear red. They indicate the stack is rolling back a failed update. |
| B.3.7 | I observe UPDATE_ROLLBACK_COMPLETE events. | These appear red. The ROLLBACK portion of the status causes red coloring even though the suffix is COMPLETE. |
| B.3.8 | I observe UPDATE_ROLLBACK_COMPLETE_CLEANUP_IN_PROGRESS events. | This is a valid CFN status. The row is colored appropriately (red, due to ROLLBACK in the name). |

**AWS comparison:**
```
aws cloudformation describe-stack-events --stack-name failed-stack \
  --query 'StackEvents[?ResourceStatus==`CREATE_FAILED` || contains(ResourceStatus, `ROLLBACK`)]'
```

### B.4 Stack with Nested Stacks

| ID | Story | Expected |
|----|-------|----------|
| B.4.1 | A parent stack has events referencing nested stacks (ResourceType = "AWS::CloudFormation::Stack"). | Events for the nested stack appear as rows. The Logical ID shows the nested stack's logical name. The Type column shows "AWS::CloudFormation::Stack". |
| B.4.2 | A nested stack event has status "CREATE_FAILED" with a reason referencing the nested stack. | The error reason (e.g., "Embedded stack arn:aws:cloudformation:... was not successfully created") is visible in the Reason column (truncated) and fully visible in the detail view. |
| B.4.3 | I select a nested stack event and press d. | The detail view shows the full event details including the PhysicalResourceId, which is the nested stack's ARN. |
| B.4.4 | I select a nested stack event and press c. | The reason (if present) or LogicalResourceId is copied. The PhysicalResourceId (nested stack ARN) is NOT what gets copied by c in the events view. |

### B.5 Hook Events

| ID | Story | Expected |
|----|-------|----------|
| B.5.1 | A stack has events with HookType set (e.g., a CloudFormation Guard hook). | These events appear in the list like any other event. The list columns do not show hook fields directly (HookType is not a list column). |
| B.5.2 | I select a hook event and press d. | The detail view shows HookType, HookStatus, HookStatusReason, HookInvocationPoint, and HookFailureMode fields with their values. |
| B.5.3 | A hook event has HookStatus "HOOK_COMPLETE_SUCCEEDED". | The event appears in the list. The ResourceStatus column shows the relevant status. Hook-specific fields are visible in the detail view. |
| B.5.4 | A hook event has HookStatus "HOOK_COMPLETE_FAILED". | The event is visible in the list. The detail view shows the HookStatusReason explaining why the hook failed and the HookFailureMode indicating whether it was FAIL or WARN. |

**AWS comparison:**
```
aws cloudformation describe-stack-events --stack-name hooked-stack \
  --query 'StackEvents[?HookType!=null]'
```

### B.6 Large Event History

| ID | Story | Expected |
|----|-------|----------|
| B.6.1 | A long-lived stack has thousands of events. The initial fetch returns the first page (~200 events). | The frame title shows the loaded count (e.g., "cfn-events(200) -- stack-name"). The events are displayed in reverse chronological order. |
| B.6.2 | I scroll down through all 200 events and reach the bottom. | Navigation stops at the last loaded event. The interface handles pagination gracefully. |
| B.6.3 | I use ctrl+r to refresh. | The latest events are fetched again. New events from recent deployments appear at the top. |

---

## C. CFN Stack Resources View (r from CFN Stacks)

### C.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | I select a stack "payment-service-prod" in the CFN Stacks list and press r. | The view transitions to the Stack Resources view. A spinner appears centered in the frame with text like "Fetching stack resources..." while the `ListStackResources` call is in flight. The frame title shows "cfn-resources" with no count. |
| C.1.2 | The API responds with 23 resources. | The spinner disappears. Resources are rendered as table rows. The frame title updates to "cfn-resources(23) -- payment-service-prod". |
| C.1.3 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. The spinner continues uninterrupted. |
| C.1.4 | The API responds with an error (e.g., "Stack does not exist", or permissions error). | The spinner disappears. A red error flash appears in the header right side. |

**AWS comparison:**
```
aws cloudformation list-stack-resources --stack-name payment-service-prod
```
Expected fields visible in list: LogicalResourceId, PhysicalResourceId, ResourceType, ResourceStatus, DriftInformation.StackResourceDriftStatus, LastUpdatedTimestamp

### C.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | A newly created stack has zero resources (e.g., a stack in CREATE_IN_PROGRESS that hasn't provisioned anything yet). | The frame title reads "cfn-resources(0) -- stack-name". The content area shows a centered message (e.g., "No resources found") with a hint to refresh. |
| C.2.2 | I press ctrl+r on the empty state. | The loading spinner appears again while the refresh request is in flight. |

### C.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| C.3.1 | Resources load and the table renders. | Six columns are displayed: "Logical ID" (width 28), "Physical ID" (width 28), "Type" (width 28), "Status" (width 24), "Drift" (width 12), "Updated" (width 22). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| C.3.2 | I verify column data against the AWS CLI output. | "Logical ID" maps to `.StackResourceSummaries[].LogicalResourceId`. "Physical ID" maps to `.StackResourceSummaries[].PhysicalResourceId`. "Type" maps to `.StackResourceSummaries[].ResourceType`. "Status" maps to `.StackResourceSummaries[].ResourceStatus`. "Drift" maps to `.StackResourceSummaries[].DriftInformation.StackResourceDriftStatus`. "Updated" maps to `.StackResourceSummaries[].LastUpdatedTimestamp`. |
| C.3.3 | A PhysicalResourceId is an ARN longer than 28 characters (e.g., "arn:aws:elb:us-east-1:123456789012:targetgroup/..."). | The ID is truncated to fit the 28-character column width. No row wrapping occurs. |
| C.3.4 | The terminal is narrower than the combined column widths (28+28+28+24+12+22 = 142 plus borders/padding). | The rightmost columns (Drift, Updated) are hidden. Horizontal scroll with h/l keys reveals them. Column headers scroll in sync with data. |
| C.3.5 | The Drift column shows various values: "NOT_CHECKED", "IN_SYNC", "MODIFIED". | Each value fits within the 12-character width. Values are displayed as plain text within the column. |

**AWS comparison:**
```
aws cloudformation list-stack-resources --stack-name payment-service-prod \
  --query 'StackResourceSummaries[].{LogicalId:LogicalResourceId,PhysicalId:PhysicalResourceId,Type:ResourceType,Status:ResourceStatus,Drift:DriftInformation.StackResourceDriftStatus,Updated:LastUpdatedTimestamp}'
```

### C.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| C.4.1 | 23 resources are loaded for stack "payment-service-prod". | The frame top border shows the title centered: "cfn-resources(23) -- payment-service-prod" with equal-length dashes on both sides. |
| C.4.2 | A filter is active and matches 5 of 23 resources. | The frame title reads "cfn-resources(5/23) -- payment-service-prod". |
| C.4.3 | A filter is active and matches 0 resources. | The frame title reads "cfn-resources(0/23) -- payment-service-prod". The content area is empty (no rows). |

### C.5 Row Coloring by Status

| ID | Story | Expected |
|----|-------|----------|
| C.5.1 | A resource has ResourceStatus "CREATE_COMPLETE". | The entire row is rendered in green (#9ece6a). |
| C.5.2 | A resource has ResourceStatus "UPDATE_COMPLETE". | The entire row is rendered in green (#9ece6a). |
| C.5.3 | A resource has ResourceStatus "CREATE_IN_PROGRESS". | The entire row is rendered in yellow (#e0af68). |
| C.5.4 | A resource has ResourceStatus "UPDATE_IN_PROGRESS". | The entire row is rendered in yellow (#e0af68). |
| C.5.5 | A resource has ResourceStatus "DELETE_IN_PROGRESS". | The entire row is rendered in yellow (#e0af68). |
| C.5.6 | A resource has ResourceStatus "CREATE_FAILED". | The entire row is rendered in red (#f7768e). |
| C.5.7 | A resource has ResourceStatus "UPDATE_FAILED". | The entire row is rendered in red (#f7768e). |
| C.5.8 | A resource has ResourceStatus "DELETE_COMPLETE". | The entire row is rendered dim (#565f89). |
| C.5.9 | I select a row that is colored green (COMPLETE). | The selected row has full-width blue background (#7aa2f7) with dark foreground (#1a1b26), bold. The blue selection overrides the green status coloring. |
| C.5.10 | I move the selection away from the green row. | The row reverts to its green status coloring. |

### C.6 Row Coloring by Drift Status

| ID | Story | Expected |
|----|-------|----------|
| C.6.1 | A resource has status "CREATE_COMPLETE" and drift status "MODIFIED". | The entire row is rendered in yellow (#e0af68). The drift status overrides the green COMPLETE coloring because MODIFIED (drifted) takes priority. |
| C.6.2 | A resource has status "CREATE_COMPLETE" and drift status "IN_SYNC". | The entire row is rendered in green (#9ece6a). IN_SYNC does not override the normal status coloring. |
| C.6.3 | A resource has status "CREATE_COMPLETE" and drift status "NOT_CHECKED". | The entire row is rendered in green (#9ece6a). NOT_CHECKED does not override the normal status coloring. |
| C.6.4 | A resource has status "CREATE_FAILED" and drift status "MODIFIED". | The entire row is rendered in red (#f7768e). FAILED takes priority over drift coloring. |
| C.6.5 | I select a drifted (yellow) row. | The selected row shows blue background, overriding the yellow drift coloring. |

**AWS comparison:**
```
aws cloudformation detect-stack-drift --stack-name payment-service-prod
aws cloudformation describe-stack-resource-drifts --stack-name payment-service-prod \
  --query 'StackResourceDrifts[?StackResourceDriftStatus==`MODIFIED`]'
```

### C.7 Navigation

| ID | Story | Expected |
|----|-------|----------|
| C.7.1 | I press j (or down-arrow) with the first resource selected. | The selection cursor moves to the second resource. |
| C.7.2 | I press k (or up-arrow) with the second resource selected. | The selection cursor moves back to the first resource. |
| C.7.3 | I press g. | The selection jumps to the very first resource in the list. |
| C.7.4 | I press G. | The selection jumps to the very last resource in the list. |
| C.7.5 | I press PageDown (or ctrl+d). | The selection moves down by one page of visible rows. |
| C.7.6 | I press PageUp (or ctrl+u). | The selection moves up by one page of visible rows. |
| C.7.7 | There are more resources than fit on screen. I scroll past the visible area. | The table scrolls to keep the selected row visible. The column headers remain in place at the top. |

### C.8 Horizontal Scroll

| ID | Story | Expected |
|----|-------|----------|
| C.8.1 | The terminal is 120 columns wide and not all 6 columns fit. | Rightmost columns (Drift, Updated) are hidden (not truncated mid-value). |
| C.8.2 | I press l (or right-arrow). | The visible column window shifts right, revealing "Status", "Drift", and "Updated" columns while hiding leftmost columns. |
| C.8.3 | I press h (or left-arrow) to scroll back. | The visible column window shifts left, restoring the original leftmost columns. |
| C.8.4 | I scroll right to reveal Drift and Updated columns. | The wireframe shows: Type, Status, Drift, Updated columns become visible. Drift values like "NOT_CHECKED", "IN_SYNC", "MODIFIED" are readable. |

### C.9 Sorting

| ID | Story | Expected |
|----|-------|----------|
| C.9.1 | I press N on the resources list. | Rows are sorted by Logical ID in ascending order. The "Logical ID" column header shows an up-arrow appended directly. |
| C.9.2 | I press N again. | Sort order toggles to descending. The indicator changes to a down-arrow. |
| C.9.3 | I press A on the resources list. | Rows are sorted by Updated (LastUpdatedTimestamp). The "Updated" column header shows the sort indicator. |
| C.9.4 | I press S on the resources list. | Rows are sorted by Status. The "Status" column header shows the sort indicator. |
| C.9.5 | I sort by name, then apply a filter. | The filtered subset remains sorted. The sort indicator persists on the column header. |
| C.9.6 | I sort by name, then refresh with ctrl+r. | After data reloads, the sort order and direction are preserved. |

### C.10 Filter

| ID | Story | Expected |
|----|-------|----------|
| C.10.1 | I press / on the resources list. | The header right side changes from "? for help" to "/\|" (amber/bold, with cursor). Filter mode is active. |
| C.10.2 | I type "ECS" in filter mode. | Only rows whose visible content contains "ECS" (case-insensitive) are displayed. This matches resources with Type "AWS::ECS::Service", "AWS::ECS::Cluster", etc. The frame title updates to show matched/total. |
| C.10.3 | I type "MODIFIED" in filter mode. | Only rows whose Drift column contains "MODIFIED" are displayed. This quickly isolates drifted resources. |
| C.10.4 | I press Escape in filter mode. | The filter is cleared. All rows reappear. The frame title reverts to the full count. The header right reverts to "? for help". |
| C.10.5 | I type a filter string that matches no resources. | Zero rows are displayed. The frame title shows "cfn-resources(0/23) -- stack-name". |

### C.11 Copy Key (c) -- Copies Physical ID

| ID | Story | Expected |
|----|-------|----------|
| C.11.1 | I select a resource with PhysicalResourceId "arn:aws:ecs:us-east-1:123456789012:service/my-cluster/my-service" and press c. | The PhysicalResourceId is copied to the system clipboard. A green flash message "Copied!" appears in the header right side. |
| C.11.2 | After ~2 seconds. | The "Copied!" flash message auto-clears and the header right reverts to "? for help". |
| C.11.3 | I paste from clipboard into the AWS Console search bar. | The pasted text is the full Physical ID: "arn:aws:ecs:us-east-1:123456789012:service/my-cluster/my-service". This is directly usable to navigate to the resource in the Console. |
| C.11.4 | I select a resource whose PhysicalResourceId is a short ID (e.g., "sg-0abc123def456789a"). | Pressing c copies "sg-0abc123def456789a". |
| C.11.5 | I select a resource whose PhysicalResourceId is a log group name (e.g., "/ecs/payment-api"). | Pressing c copies "/ecs/payment-api". |

**AWS comparison:**
```
aws cloudformation list-stack-resources --stack-name payment-service-prod \
  --query 'StackResourceSummaries[].PhysicalResourceId'
```
The copied text should match one of these values exactly.

### C.12 Detail Key (d)

| ID | Story | Expected |
|----|-------|----------|
| C.12.1 | I select a resource and press d. | The detail view opens for the selected resource. The frame title shows identifying information for the resource. |
| C.12.2 | I verify the detail fields match the cfn_resources detail config. | The detail view shows key-value pairs for: LogicalResourceId, PhysicalResourceId, ResourceType, ResourceStatus, ResourceStatusReason, DriftInformation, LastUpdatedTimestamp, ModuleInfo, Description. |
| C.12.3 | The DriftInformation field is a nested object. | It is rendered as a sub-section with fields like StackResourceDriftStatus and LastCheckTimestamp (if available). Section headers appear in yellow/orange (#e0af68), bold. |
| C.12.4 | A resource has no ResourceStatusReason (it succeeded). | The ResourceStatusReason field shows null/empty/dash in the detail view. |
| C.12.5 | A resource has no ModuleInfo. | The ModuleInfo field shows null/empty/dash. |
| C.12.6 | A resource has a Description. | The Description field displays the value from the template metadata. |
| C.12.7 | I press Escape on the detail view. | I return to the Stack Resources list. The cursor position is preserved on the same resource I had selected. |

**AWS comparison:**
```
aws cloudformation describe-stack-resource --stack-name payment-service-prod \
  --logical-resource-id ApiLoadBalancer
```
Expected detail fields: LogicalResourceId, PhysicalResourceId, ResourceType, ResourceStatus, ResourceStatusReason, DriftInformation, LastUpdatedTimestamp, ModuleInfo, Description

### C.13 YAML Key (y)

| ID | Story | Expected |
|----|-------|----------|
| C.13.1 | I select a resource and press y. | The YAML view opens. The frame title includes identifying info and "yaml". The full resource is rendered as syntax-highlighted YAML. |
| C.13.2 | YAML keys are colored blue (#7aa2f7), string values green (#9ece6a), timestamps green, null values dim (#565f89). | Visual inspection confirms the color coding. |
| C.13.3 | I press Escape on the YAML view. | I return to the Stack Resources list. |

### C.14 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| C.14.1 | I press ctrl+r on the resources list. | The loading spinner appears. A fresh `ListStackResources` call is made. When it completes, the table repopulates with current data. |
| C.14.2 | A deployment added new resources since the last load. I press ctrl+r. | New resources appear in the refreshed list. The count in the frame title increments. |
| C.14.3 | I had a filter active and press ctrl+r. | The data refreshes. The filter remains applied to the new data. The frame title count updates accordingly. |

### C.15 Escape (Back to CFN Stacks)

| ID | Story | Expected |
|----|-------|----------|
| C.15.1 | I press Escape on the Stack Resources list. | I return to the CFN Stacks parent list. The cursor is on the same stack I had entered. |

### C.16 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| C.16.1 | I press ? on the resources list. | The help screen replaces the table content inside the frame. It shows four columns: STACK RESOURCES, GENERAL, NAVIGATION, HOTKEYS. |
| C.16.2 | The STACK RESOURCES column shows key bindings. | Listed bindings include: esc (Back), d (Detail), y (YAML), c (Copy Phys ID). |
| C.16.3 | The GENERAL column shows: ctrl-r (Refresh), / (Filter), : (Command). | These match the standard general bindings. |
| C.16.4 | The NAVIGATION column shows: j (Down), k (Up), g (Top), G (Bottom), h/l (Cols), pgup/dn (Page). | All navigation keys are documented. |
| C.16.5 | The HOTKEYS column shows: ? (Help), : (Command). | Standard hotkeys are listed. |
| C.16.6 | I press any key on the help screen. | The help screen closes and the resources list reappears. |

### C.17 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| C.17.1 | I press : on the resources list. | The header right side changes to ":\|" (amber/bold). Command mode is active. |
| C.17.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list. |
| C.17.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The resources list remains. |

---

## D. CFN Stack Resources -- Scenario-Based Stories

### D.1 Active Deployment with IN_PROGRESS Resources

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | A stack update is in progress. I press r on the stack. | Resources load. Some resources show "UPDATE_IN_PROGRESS" (yellow), others show "UPDATE_COMPLETE" (green), and the rest show "CREATE_COMPLETE" (green, unchanged). |
| D.1.2 | I observe the visual pattern. | Yellow rows for resources being updated are visually distinct from green rows for completed resources. This gives an at-a-glance view of deployment progress. |
| D.1.3 | I press ctrl+r during the active deployment. | Resources that were IN_PROGRESS may now show as COMPLETE. The LastUpdatedTimestamp columns update for changed resources. |

**AWS comparison:**
```
aws cloudformation list-stack-resources --stack-name payment-service-prod \
  --query 'StackResourceSummaries[?contains(ResourceStatus, `IN_PROGRESS`)]'
```

### D.2 Completed Deployment (All COMPLETE)

| ID | Story | Expected |
|----|-------|----------|
| D.2.1 | A stack deployment completed successfully. I press r on the stack. | Resources load. All resources show COMPLETE statuses (CREATE_COMPLETE or UPDATE_COMPLETE). All rows are green. |
| D.2.2 | I verify the Updated column shows reasonable timestamps. | Resources updated during the most recent deployment have recent timestamps. Older resources have earlier timestamps. |

### D.3 DRIFTED Resources

| ID | Story | Expected |
|----|-------|----------|
| D.3.1 | A stack has been drift-detected and some resources have drift status "MODIFIED". | These resources appear with yellow row coloring (MODIFIED overrides the green COMPLETE coloring). The Drift column shows "MODIFIED". |
| D.3.2 | I filter with "/" and type "MODIFIED". | Only drifted resources are shown. This is a quick way to find all resources that have been changed outside of CloudFormation. |
| D.3.3 | A resource has drift status "IN_SYNC". | The Drift column shows "IN_SYNC". The row remains green (normal COMPLETE coloring). |
| D.3.4 | A resource has drift status "NOT_CHECKED". | The Drift column shows "NOT_CHECKED". The row remains green (normal COMPLETE coloring). Drift detection has not been run for this resource. |
| D.3.5 | I select a drifted resource and press d. | The detail view shows the full DriftInformation section with StackResourceDriftStatus. |

**AWS comparison:**
```
aws cloudformation describe-stack-resource-drifts --stack-name payment-service-prod \
  --stack-resource-drift-status-filters MODIFIED
```

### D.4 Stack with 100+ Resources

| ID | Story | Expected |
|----|-------|----------|
| D.4.1 | A large stack has 120 resources. The API call may require pagination. | All 120 resources are loaded and displayed. The frame title shows "cfn-resources(120) -- stack-name". |
| D.4.2 | I scroll through all 120 resources using j/k/g/G/PageDown/PageUp. | Navigation works smoothly. The table scrolls to keep the selected row visible. |
| D.4.3 | I filter the 120 resources by type "ECS". | Only ECS-related resources are shown. The frame title shows "cfn-resources(M/120) -- stack-name" where M is the matched count. |

### D.5 Nested Stack as a Resource

| ID | Story | Expected |
|----|-------|----------|
| D.5.1 | A parent stack has a resource with ResourceType "AWS::CloudFormation::Stack" (a nested stack). | The nested stack appears as a row in the resources list. Logical ID shows its logical name, Physical ID shows the nested stack ARN, Type shows "AWS::CloudFormation::Stack". |
| D.5.2 | I select the nested stack resource and press c. | The PhysicalResourceId (the nested stack ARN) is copied to the clipboard. |
| D.5.3 | I select the nested stack resource and press d. | The detail view shows the full nested stack resource summary including its ARN, status, and drift information. |

---

## E. Cross-Cutting Concerns -- Both CFN Child Views

### E.1 Entry Key Bindings from CFN Stacks List

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | I am on the CFN Stacks list and press Enter on a stack. | The Stack Events view opens (the "what happened?" timeline). This is the primary child view. |
| E.1.2 | I am on the CFN Stacks list and press r on a stack. | The Stack Resources view opens (the "what exists?" inventory). |
| E.1.3 | I am on the CFN Stacks list and press d on a stack. | The detail view opens for the stack itself (not Events or Resources). This is the standard describe behavior. |
| E.1.4 | I am on the CFN Stacks list and press y on a stack. | The YAML view opens for the stack itself. |

### E.2 Navigating Between Events and Resources

| ID | Story | Expected |
|----|-------|----------|
| E.2.1 | I open Events (Enter on a stack), then press Escape to go back, then press r to open Resources. | Events -> CFN Stacks -> Resources. Each transition works correctly. The view stack is: MainMenu -> CFN Stacks -> CFN Resources. |
| E.2.2 | I open Resources (r on a stack), then press Escape to go back, then press Enter to open Events. | Resources -> CFN Stacks -> Events. Each transition works correctly. The view stack is: MainMenu -> CFN Stacks -> CFN Events. |
| E.2.3 | I am in the Events view and press Escape. | I return to the CFN Stacks list. The cursor is on the same stack. From there I can press r to see Resources for the same stack. |
| E.2.4 | I am in the Resources view and press Escape. | I return to the CFN Stacks list. The cursor is on the same stack. From there I can press Enter to see Events for the same stack. |

### E.3 View Stack Depth

| ID | Story | Expected |
|----|-------|----------|
| E.3.1 | Main Menu -> CFN Stacks -> Stack Events -> Event Detail -> Event YAML; then Escape four times. | Each Escape pops one level: YAML -> Detail -> Events -> CFN Stacks -> Main Menu. No state is lost at any intermediate level. |
| E.3.2 | Main Menu -> CFN Stacks -> Stack Resources -> Resource Detail -> Resource YAML; then Escape four times. | Each Escape pops one level: YAML -> Detail -> Resources -> CFN Stacks -> Main Menu. No state is lost at any intermediate level. |
| E.3.3 | I am in Event Detail and press y. | The view switches from detail to YAML view for the same event. |
| E.3.4 | I am in Resource Detail and press y. | The view switches from detail to YAML view for the same resource. |

### E.4 Copy Behavior Differences

| ID | Story | Expected |
|----|-------|----------|
| E.4.1 | I am in the Events view and press c on a failed event. | The ResourceStatusReason (error message) is copied. This is the most useful text to paste into Slack during incident debugging. |
| E.4.2 | I am in the Resources view and press c on the same logical resource. | The PhysicalResourceId (actual AWS resource ID) is copied. This is the most useful text to paste into the Console search bar. |
| E.4.3 | I confirm the distinction: Events copies reason, Resources copies Physical ID. | The two views have different copy targets optimized for their primary use case. Events is for debugging (paste the error), Resources is for navigation (paste the resource ID). |
| E.4.4 | I am in the Events view and press c on a successful event with no reason. | The LogicalResourceId is copied as a fallback. |
| E.4.5 | I am in the Resources view and press c on a resource whose Physical ID is null/empty (resource creation failed before an ID was assigned). | The copy either copies empty/null or falls back to the LogicalResourceId. A "Copied!" flash still appears. |

### E.5 Header Consistency

| ID | Story | Expected |
|----|-------|----------|
| E.5.1 | In every CFN child view (events list, resources list, event detail, resource detail, event YAML, resource YAML), the header displays: "a9s" (accent bold), version (dim), profile:region (bold). | Visual inspection confirms across all views. |
| E.5.2 | The header right side shows "? for help" in normal mode across all CFN child views. | Confirmed in events list, resources list, detail views, and YAML views. |

### E.6 Terminal Resize

| ID | Story | Expected |
|----|-------|----------|
| E.6.1 | I resize the terminal while viewing the Stack Events list. | The layout reflows. Column visibility adjusts to the new width. The frame border redraws correctly. The frame title remains centered. |
| E.6.2 | I resize the terminal while viewing the Stack Resources list. | Same reflow behavior as Events. Column visibility adjusts. |
| E.6.3 | I resize the terminal to below 60 columns while in either child view. | An error message appears: "Terminal too narrow. Please resize." |
| E.6.4 | I resize the terminal to below 7 lines while in either child view. | An error message appears: "Terminal too short. Please resize." |
| E.6.5 | I resize the terminal while in the Event or Resource detail view. | The viewport adjusts to the new dimensions. Content reflows appropriately. |

### E.7 Alternating Row Colors

| ID | Story | Expected |
|----|-------|----------|
| E.7.1 | The events list has more than 2 rows. | Alternating rows have a subtle background color difference (#1e2030) for readability. This is combined with status coloring (green/yellow/red/dim text on alternating backgrounds). Selected row always has blue background regardless. |
| E.7.2 | The resources list has more than 2 rows. | Same alternating row pattern applies. |

### E.8 Detail View Actions

| ID | Story | Expected |
|----|-------|----------|
| E.8.1 | I press c in the event detail view. | The full detail content is copied to the clipboard. A "Copied!" flash appears. |
| E.8.2 | I press c in the resource detail view. | The full detail content is copied to the clipboard. A "Copied!" flash appears. |
| E.8.3 | I press w in either detail view. | Word wrap is toggled. Long values (e.g., ARNs, error reasons) that extended beyond the visible width now wrap to the next line (or vice versa). |
| E.8.4 | I press j/k in either detail view. | The viewport scrolls up/down by one line. |
| E.8.5 | I press g in either detail view. | The viewport jumps to the top of the detail content. |
| E.8.6 | I press G in either detail view. | The viewport jumps to the bottom of the detail content. |
| E.8.7 | The detail content is shorter than the visible area. | No scrolling occurs. No scroll indicators are shown. |
| E.8.8 | The detail content is longer than the visible area. | Scroll indicators appear in dim text when content extends beyond the visible area. |

### E.9 Very Long ResourceStatusReason

| ID | Story | Expected |
|----|-------|----------|
| E.9.1 | An event has a ResourceStatusReason that is 200+ characters (e.g., a full IAM policy error or multiple resource references). | In the list view, the reason is truncated to fit the 40-character "Reason" column. In the detail view (d), the full reason is visible with scrolling. |
| E.9.2 | I press c on the event with the long reason. | The FULL reason text is copied to the clipboard, not the truncated version. |
| E.9.3 | I press w in the detail view showing the long reason. | The long reason text wraps within the viewport width. |
