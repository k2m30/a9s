# QA User Stories: ELB Listeners, ASG Scaling Activities, Target Group Health

Covers three child views for load balancing and auto scaling resources:
- **A.** Target Groups --> Target Health (Issue #35)
- **B.** Auto Scaling Groups --> Scaling Activities (Issue #36)
- **C.** Load Balancers --> Listeners (Issue #37)
- **D.** Listeners --> Listener Rules (nested child from Issue #37)
- **E.** Cross-Cutting Concerns

All stories are written from a black-box perspective against the design spec and
`views.yaml` configuration files. AWS CLI equivalents are cited so testers can verify data parity.

---

## A. Target Group Health View

### A.1 Loading State

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I select a target group in the Target Groups list and press Enter. | A spinner (animated dot) is displayed centered inside the frame. The text reads "Fetching target health..." (or similar). The frame title reads "tg-health" with no count. The header shows "? for help" on the right. |
| A.1.2 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. The spinner continues uninterrupted. |
| A.1.3 | The API responds successfully with target health data. | The spinner disappears. The table renders with column headers and rows. The frame title updates to "tg-health(N) -- target-group-name" where N is the number of registered targets and target-group-name is the name of the parent target group. |
| A.1.4 | The API responds with an error (e.g., target group deleted while viewing, expired credentials). | The spinner disappears. A red error flash message appears in the header right side. The frame content area shows an appropriate empty or error state. |

**AWS comparison:**
```
aws elbv2 describe-target-health --target-group-arn arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/api-prod-tg/abc123
```
Expected fields visible: Target ID, Port, AZ, Health, Reason, Description

### A.2 Empty State (Zero Registered Targets)

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | The target group has zero registered targets. | The frame title reads "tg-health(0) -- target-group-name". The content area shows a centered message (e.g., "No targets registered") with a hint to refresh or check the target group configuration. No column headers are shown (or headers are shown with no data rows). |
| A.2.2 | I press ctrl+r on the empty state. | The loading spinner appears again while the refresh request is in flight. |
| A.2.3 | The target group was just created and has not had any targets registered. | Same behavior as A.2.1 -- the empty state is gracefully handled with a meaningful message. |

**AWS comparison:**
```
aws elbv2 describe-target-health --target-group-arn arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/empty-tg/abc123
```
Returns `{ "TargetHealthDescriptions": [] }`

### A.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | Target health data loads and the table renders. | Six columns are displayed: "Target ID" (width 24), "Port" (width 8), "AZ" (width 14), "Health" (width 14), "Reason" (width 28), "Description" (width 36). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| A.3.2 | I verify column data against `aws elbv2 describe-target-health`. | "Target ID" maps to `.TargetHealthDescriptions[].Target.Id`. "Port" maps to `.TargetHealthDescriptions[].Target.Port`. "AZ" maps to `.TargetHealthDescriptions[].Target.AvailabilityZone`. "Health" maps to `.TargetHealthDescriptions[].TargetHealth.State`. "Reason" maps to `.TargetHealthDescriptions[].TargetHealth.Reason`. "Description" maps to `.TargetHealthDescriptions[].TargetHealth.Description`. |
| A.3.3 | A Target ID is longer than 24 characters (e.g., a long Lambda ARN). | The ID is truncated to fit the 24-character column width. No row wrapping occurs. |
| A.3.4 | The terminal is narrower than the combined column widths (24+8+14+14+28+36=124 plus borders). | The rightmost column(s) are hidden (not truncated mid-value). Horizontal scroll with h/l is available to reveal hidden columns. |
| A.3.5 | The Description column is visible (scrolled right with `l` key). | The Description column shows the full human-readable text from the API (e.g., "Health checks failed"). Long descriptions are truncated to the 36-character column width. |

**AWS comparison:**
```
aws elbv2 describe-target-health --target-group-arn ARN --query 'TargetHealthDescriptions[].{TargetId:Target.Id,Port:Target.Port,AZ:Target.AvailabilityZone,Health:TargetHealth.State,Reason:TargetHealth.Reason,Desc:TargetHealth.Description}'
```

### A.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| A.4.1 | 4 targets are registered in target group "api-prod-tg". | The frame top border shows the title centered: "tg-health(4) -- api-prod-tg" with equal-length dashes on both sides. |
| A.4.2 | A filter is active and matches 2 of 4 targets. | The frame title reads "tg-health(2/4) -- api-prod-tg". |
| A.4.3 | A filter is active and matches 0 targets. | The frame title reads "tg-health(0/4) -- api-prod-tg". The content area is empty (no rows). |

### A.5 Row Coloring by Health State

| ID | Story | Expected |
|----|-------|----------|
| A.5.1 | A target has Health state "healthy". | The entire row is rendered in GREEN (#9ece6a). |
| A.5.2 | A target has Health state "unhealthy". | The entire row is rendered in RED (#f7768e). |
| A.5.3 | A target has Health state "draining". | The entire row is rendered in YELLOW (#e0af68). |
| A.5.4 | A target has Health state "initial". | The entire row is rendered in YELLOW (#e0af68). |
| A.5.5 | A target has Health state "unavailable". | The entire row is rendered in RED (#f7768e). |
| A.5.6 | A target has Health state "unused". | The entire row is rendered DIM (#565f89). |
| A.5.7 | I select a row. | The selected row has full-width blue background (#7aa2f7), dark foreground (#1a1b26), bold text. The health-state coloring is overridden by the selection style. |
| A.5.8 | All 4 targets are healthy. | All rows display in GREEN. No red, yellow, or dim rows are visible. |
| A.5.9 | Targets have mixed health states: 2 healthy, 1 unhealthy, 1 draining. | Rows display in mixed colors: two GREEN, one RED, one YELLOW. The selected row always shows blue regardless of its health state. |

**AWS comparison:**
```
aws elbv2 describe-target-health --target-group-arn ARN --query 'TargetHealthDescriptions[].TargetHealth.State'
```

### A.6 Instance-Based vs IP-Based vs Lambda Target Groups

| ID | Story | Expected |
|----|-------|----------|
| A.6.1 | The target group is instance-type. I view target health. | The Target ID column shows EC2 instance IDs (e.g., "i-0a1b2c3d4e5f67890"). The Port column shows the registered port. The AZ column shows the instance's availability zone. |
| A.6.2 | The target group is IP-type. I view target health. | The Target ID column shows IP addresses (e.g., "10.0.1.47"). The Port column shows the registered port. The AZ column shows the target's availability zone (may be "all" for cross-zone). |
| A.6.3 | The target group is Lambda-type (no port, no AZ). | The Target ID column shows the Lambda function ARN (likely truncated due to column width). The Port column shows empty/dash. The AZ column shows empty/dash. |
| A.6.4 | I press `c` on an instance-type target. | The instance ID (e.g., "i-0a1b2c3d4e5f67890") is copied to the clipboard. |
| A.6.5 | I press `c` on an IP-type target. | The IP address (e.g., "10.0.1.47") is copied to the clipboard. |

**AWS comparison (instance-type):**
```
aws elbv2 describe-target-health --target-group-arn ARN
# Returns Target.Id like "i-0a1b2c3d4e5f67890"
```
**AWS comparison (IP-type):**
```
aws elbv2 describe-target-health --target-group-arn ARN
# Returns Target.Id like "10.0.1.47"
```

### A.7 Draining State During Deployment

| ID | Story | Expected |
|----|-------|----------|
| A.7.1 | A deployment is in progress. I deregistered a target; it is now "draining". | The target appears in the list with Health "draining", Reason "Target.DeregistrationInProgress", and a Description like "Target deregistration is in progress". The entire row is YELLOW (#e0af68). |
| A.7.2 | I press ctrl+r while the target is draining. | After refresh, the target may still show "draining" (if the deregistration drain period has not elapsed) or may have disappeared from the list (if fully deregistered). |
| A.7.3 | A blue-green or rolling deployment is in progress. Mixed targets are healthy and draining. | The list shows both healthy (GREEN) and draining (YELLOW) targets simultaneously. The count in the frame title reflects all targets including draining ones. |

### A.8 Target in "initial" State (Newly Registered)

| ID | Story | Expected |
|----|-------|----------|
| A.8.1 | A target was just registered and health checks have not yet completed. | The Health column shows "initial". The Reason column shows "Elb.RegistrationInProgress" (or similar). The entire row is YELLOW (#e0af68). |
| A.8.2 | I wait and press ctrl+r after the health check interval passes. | After refresh, the target's health state has changed from "initial" to "healthy" (GREEN) or "unhealthy" (RED) depending on the health check result. |

### A.9 Target in "unavailable" State

| ID | Story | Expected |
|----|-------|----------|
| A.9.1 | A target is in the "unavailable" state (e.g., target is in an AZ not enabled for the load balancer, or the target's security group blocks health check traffic). | The Health column shows "unavailable". The Reason column shows the specific reason code (e.g., "Target.InvalidState"). The Description provides a human-readable explanation. The entire row is RED (#f7768e). |

### A.10 Navigation

| ID | Story | Expected |
|----|-------|----------|
| A.10.1 | I press j (or down-arrow) with the first target selected. | The selection cursor moves to the second target. The previously selected row loses the blue highlight and reverts to its health-state color. |
| A.10.2 | I press k (or up-arrow) with the second target selected. | The selection cursor moves back to the first target. |
| A.10.3 | I press g. | The selection jumps to the first target in the list. |
| A.10.4 | I press G. | The selection jumps to the last target in the list. |
| A.10.5 | I press PageDown (or ctrl+d). | The selection moves down by one page of visible rows. |
| A.10.6 | I press PageUp (or ctrl+u). | The selection moves up by one page of visible rows. |
| A.10.7 | I press h (or left-arrow). | Columns scroll left, revealing previously hidden left columns. |
| A.10.8 | I press l (or right-arrow). | Columns scroll right, revealing the Reason and Description columns that may be hidden on standard 80-column terminals. |

### A.11 Sorting

| ID | Story | Expected |
|----|-------|----------|
| A.11.1 | I press N on the target health list. | Rows are sorted by Target ID in ascending order. The "Target ID" column header shows an up-arrow indicator. |
| A.11.2 | I press N again. | Sort order toggles to descending. The indicator changes to a down-arrow. |
| A.11.3 | I press S on the target health list. | Rows are sorted by Health state alphabetically. The "Health" column header shows the sort indicator. |
| A.11.4 | I press A on the target health list. | Rows are sorted by age (if applicable). The appropriate column header shows the sort indicator. |

### A.12 Filter

| ID | Story | Expected |
|----|-------|----------|
| A.12.1 | I press / and type "unhealthy". | Only targets with "unhealthy" in their row text are shown. The frame title updates to "tg-health(M/N) -- target-group-name". |
| A.12.2 | I press / and type an instance ID fragment (e.g., "0a1b"). | Only targets whose Target ID contains "0a1b" are shown. |
| A.12.3 | I press / and type "10.0" in an IP-type target group. | Only IP-based targets starting with "10.0" are shown. |
| A.12.4 | I press Escape in filter mode. | The filter is cleared. All targets reappear. |

### A.13 Detail View (d)

| ID | Story | Expected |
|----|-------|----------|
| A.13.1 | I select a target and press d. | The detail view opens for the selected target. The frame title shows the target ID. |
| A.13.2 | I verify the detail fields match views.yaml tg_health detail config. | The detail view shows key-value pairs for: Target.Id, Target.Port, Target.AvailabilityZone, TargetHealth.State, TargetHealth.Reason, TargetHealth.Description, HealthCheckPort, AnomalyDetection. |
| A.13.3 | I press Escape on the detail view. | I return to the target health list. The cursor position is preserved. |

**AWS comparison:**
```
aws elbv2 describe-target-health --target-group-arn ARN --targets Id=i-0a1b2c3d4e5f67890,Port=8080
```
Expected detail fields: Target.Id, Target.Port, Target.AvailabilityZone, TargetHealth.State, TargetHealth.Reason, TargetHealth.Description, HealthCheckPort, AnomalyDetection

### A.14 YAML View (y)

| ID | Story | Expected |
|----|-------|----------|
| A.14.1 | I select a target and press y. | The YAML view opens. The frame title includes the target ID and "yaml". The full target health description is rendered as syntax-highlighted YAML. |
| A.14.2 | YAML keys are colored blue (#7aa2f7), string values green (#9ece6a), numbers orange (#ff9e64), booleans purple (#bb9af7), null values dim (#565f89). | Visual inspection confirms the color coding matches the design spec. |
| A.14.3 | I press Escape on the YAML view. | I return to the target health list. |

### A.15 Copy (c)

| ID | Story | Expected |
|----|-------|----------|
| A.15.1 | I select a target and press c. | The Target ID is copied to the system clipboard. A green flash message "Copied!" appears in the header right side. |
| A.15.2 | After ~2 seconds. | The "Copied!" flash message auto-clears and the header right reverts to "? for help". |
| A.15.3 | I paste from clipboard into another application. | The pasted text matches the Target ID exactly (e.g., "i-0a1b2c3d4e5f67890" for instance-type or "10.0.1.47" for IP-type). |

### A.16 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| A.16.1 | I press ctrl+r on the target health list. | The loading spinner appears. A fresh `DescribeTargetHealth` API call is made. When it completes, the table repopulates with current data. |
| A.16.2 | A target was newly registered since last load. I press ctrl+r. | The new target appears in the refreshed list. The count in the frame title increments. |
| A.16.3 | A target transitioned from "initial" to "healthy" since last load. I press ctrl+r. | The target's row color changes from YELLOW to GREEN. The Health column now shows "healthy". |

### A.17 Escape (Back to Target Groups)

| ID | Story | Expected |
|----|-------|----------|
| A.17.1 | I press Escape on the target health list. | I return to the Target Groups list. The cursor is on the same target group I had selected. |

### A.18 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| A.18.1 | I press ? on the target health list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: TARGET HEALTH, GENERAL, NAVIGATION, HOTKEYS. |
| A.18.2 | The TARGET HEALTH column shows: `<esc>` Back, `<d>` Detail, `<y>` YAML, `<c>` Copy Target. | These entries match the help wireframe from the design spec. |
| A.18.3 | I press any key on the help screen. | The help screen closes and the target health table reappears. |

### A.19 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| A.19.1 | I press : on the target health list. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| A.19.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list. |
| A.19.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The target health list remains. |

---

## B. ASG Scaling Activities View

### B.1 Loading State

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | I select an Auto Scaling Group in the ASG list and press Enter. | A spinner (animated dot) is displayed centered inside the frame. The text reads "Fetching scaling activities..." (or similar). The frame title reads "asg-activities" with no count. |
| B.1.2 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. |
| B.1.3 | The API responds successfully with activity data (paginated, newest first). | The spinner disappears. The table renders with activities in reverse chronological order (newest first). The frame title updates to "asg-activities(N) -- asg-name" where N is the total activity count. |
| B.1.4 | The API responds with an error (e.g., ASG deleted, expired credentials). | The spinner disappears. A red error flash message appears in the header right side. |

**AWS comparison:**
```
aws autoscaling describe-scaling-activities --auto-scaling-group-name api-prod-asg
```
Expected fields visible: Start Time, Status, Description, Cause

### B.2 Empty State (No Scaling Activities)

| ID | Story | Expected |
|----|-------|----------|
| B.2.1 | The ASG has zero scaling activities (newly created, no scaling events in past 6 weeks). | The frame title reads "asg-activities(0) -- asg-name". The content area shows a centered message (e.g., "No scaling activities found"). |
| B.2.2 | I press ctrl+r on the empty state. | The loading spinner appears again while the refresh request is in flight. |

**AWS comparison:**
```
aws autoscaling describe-scaling-activities --auto-scaling-group-name new-asg
```
Returns `{ "Activities": [] }`

### B.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| B.3.1 | Scaling activities load and the table renders. | Four columns are displayed: "Start Time" (width 22), "Status" (width 14), "Description" (width 50), "Cause" (width 40). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| B.3.2 | I verify column data against `aws autoscaling describe-scaling-activities`. | "Start Time" maps to `.Activities[].StartTime`. "Status" maps to `.Activities[].StatusCode`. "Description" maps to `.Activities[].Description`. "Cause" maps to `.Activities[].Cause`. |
| B.3.3 | A Description is longer than 50 characters. | The description is truncated to fit the 50-character column width. No row wrapping occurs. |
| B.3.4 | The terminal is narrower than the combined column widths (22+14+50+40=126 plus borders). | The rightmost column(s) are hidden. Horizontal scroll with h/l is available to reveal the Cause column. |
| B.3.5 | I scroll right with `l` to reveal the Cause column. | The Cause column shows the reason for the scaling activity (e.g., "At 2026-03-22T03:15:00Z an alarm triggered..."). Column headers scroll in sync with data. |

**AWS comparison:**
```
aws autoscaling describe-scaling-activities --auto-scaling-group-name asg-name --query 'Activities[].{StartTime:StartTime,Status:StatusCode,Description:Description,Cause:Cause}'
```

### B.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| B.4.1 | 48 activities are loaded for ASG "api-prod-asg". | The frame top border shows the title centered: "asg-activities(48) -- api-prod-asg" with equal-length dashes on both sides. |
| B.4.2 | A filter is active and matches 5 of 48 activities. | The frame title reads "asg-activities(5/48) -- api-prod-asg". |
| B.4.3 | A filter is active and matches 0 activities. | The frame title reads "asg-activities(0/48) -- api-prod-asg". The content area is empty (no rows). |

### B.5 Row Coloring by Activity Status

| ID | Story | Expected |
|----|-------|----------|
| B.5.1 | An activity has StatusCode "Successful". | The entire row is rendered in GREEN (#9ece6a). |
| B.5.2 | An activity has StatusCode "Failed". | The entire row is rendered in RED (#f7768e). |
| B.5.3 | An activity has StatusCode "InProgress". | The entire row is rendered in YELLOW (#e0af68). |
| B.5.4 | An activity has StatusCode "PreInService". | The entire row is rendered in YELLOW (#e0af68). |
| B.5.5 | An activity has StatusCode "WaitingForSpotInstanceRequestId". | The entire row is rendered in YELLOW (#e0af68). |
| B.5.6 | An activity has StatusCode "WaitingForSpotInstanceId". | The entire row is rendered in YELLOW (#e0af68). |
| B.5.7 | An activity has StatusCode "WaitingForInstanceId". | The entire row is rendered in YELLOW (#e0af68). |
| B.5.8 | An activity has StatusCode "WaitingForConnectionDraining". | The entire row is rendered in YELLOW (#e0af68). |
| B.5.9 | An activity has StatusCode "MidLifecycleAction". | The entire row is rendered in YELLOW (#e0af68). |
| B.5.10 | An activity has StatusCode "WaitingForELBConnectionDraining". | The entire row is rendered in YELLOW (#e0af68). |
| B.5.11 | An activity has StatusCode "Cancelled". | The entire row is rendered DIM (#565f89). |
| B.5.12 | I select a row. | The selected row has full-width blue background (#7aa2f7), dark foreground (#1a1b26), bold text. Status coloring is overridden by the selection style. |

**AWS comparison:**
```
aws autoscaling describe-scaling-activities --auto-scaling-group-name asg-name --query 'Activities[].StatusCode'
```

### B.6 Chronological Ordering

| ID | Story | Expected |
|----|-------|----------|
| B.6.1 | Activities load from the API. | Activities are displayed newest first (the API returns them in this order). The first row has the most recent Start Time. |
| B.6.2 | I press A to sort by age (ascending). | Rows reorder so the oldest activity is at the top. The "Start Time" column header shows an up-arrow indicator. |
| B.6.3 | I press A again. | Sort toggles to descending (newest first, matching the natural API order). The indicator changes to a down-arrow. |

### B.7 Failed Scaling Activity (Spot Capacity Unavailable)

| ID | Story | Expected |
|----|-------|----------|
| B.7.1 | An ASG tried to launch a Spot instance but capacity was unavailable. The activity has StatusCode "Failed". | The row is RED. The Description column shows something like "Launching a new EC2 instance. Status Reason: There is no Spot capacity available that matches your request." The Cause column explains the trigger. |
| B.7.2 | I select the failed activity and press d. | The detail view opens. I can see the full StatusMessage with the complete error text (not truncated). The Description and Cause fields are shown in full. |
| B.7.3 | I verify the detail fields match views.yaml asg_activities detail config. | The detail view shows: ActivityId, StartTime, EndTime, StatusCode, StatusMessage, Description, Cause, Details, Progress, AutoScalingGroupName, AutoScalingGroupARN, AutoScalingGroupState. |

**AWS comparison:**
```
aws autoscaling describe-scaling-activities --auto-scaling-group-name asg-name --query 'Activities[?StatusCode==`Failed`]'
```
Expected detail fields: ActivityId, StartTime, EndTime, StatusCode, StatusMessage, Description, Cause, Details, Progress, AutoScalingGroupName, AutoScalingGroupARN, AutoScalingGroupState

### B.8 In-Progress Scaling Activity

| ID | Story | Expected |
|----|-------|----------|
| B.8.1 | An ASG is currently scaling out. An activity has StatusCode "InProgress". | The row is YELLOW. The Description shows "Launching a new EC2 instance: i-..." The Progress detail field (via d) shows a percentage (e.g., 50). |
| B.8.2 | I press ctrl+r while a scaling activity is in progress. | After refresh, the activity may have progressed (higher Progress percentage) or completed (StatusCode changed to "Successful" or "Failed"). |

### B.9 ASG Cooldown Preventing Scaling

| ID | Story | Expected |
|----|-------|----------|
| B.9.1 | An ASG has a recent successful scaling activity and is in cooldown. I view activities. | The most recent activity shows "Successful" (GREEN). No new activity appears for the duration of the cooldown period. |
| B.9.2 | I press ctrl+r repeatedly during cooldown. | The activity list does not change -- no new activities are added until the cooldown expires and another scaling event triggers. |

### B.10 Activity Retention (6-Week Window)

| ID | Story | Expected |
|----|-------|----------|
| B.10.1 | The ASG has activities spanning several weeks. | Only activities from the past 6 weeks (42 days) are visible. Activities older than that have been purged by AWS and do not appear. |
| B.10.2 | I press G to jump to the bottom (oldest visible activity). | The oldest activity has a Start Time no older than approximately 6 weeks from today. |

### B.11 Navigation

| ID | Story | Expected |
|----|-------|----------|
| B.11.1 | I press j (or down-arrow) with the first activity selected. | The selection cursor moves to the second activity (the next oldest). |
| B.11.2 | I press k (or up-arrow). | The selection cursor moves to the previous (more recent) activity. |
| B.11.3 | I press g. | The selection jumps to the first (newest) activity. |
| B.11.4 | I press G. | The selection jumps to the last (oldest) activity. |
| B.11.5 | I press PageDown (or ctrl+d). | The selection moves down by one page of visible rows. |
| B.11.6 | I press PageUp (or ctrl+u). | The selection moves up by one page of visible rows. |
| B.11.7 | I press h (or left-arrow). | Columns scroll left, revealing previously hidden left columns. |
| B.11.8 | I press l (or right-arrow). | Columns scroll right, revealing the Cause column. Column headers scroll in sync. |
| B.11.9 | There are more activities than fit on screen (e.g., 48 activities). I scroll past the visible area. | The table scrolls to keep the selected row visible. Column headers remain in place. |

### B.12 Sorting

| ID | Story | Expected |
|----|-------|----------|
| B.12.1 | I press N on the activities list. | Rows are sorted by Description in ascending alphabetical order. The "Description" column header shows an up-arrow indicator. |
| B.12.2 | I press N again. | Sort order toggles to descending. The indicator changes to a down-arrow. |
| B.12.3 | I press S on the activities list. | Rows are sorted by Status in ascending order. The "Status" column header shows the sort indicator. |
| B.12.4 | I sort by status, then apply a filter. | The filtered subset remains sorted by status. The sort indicator persists. |

### B.13 Filter

| ID | Story | Expected |
|----|-------|----------|
| B.13.1 | I press / and type "Failed". | Only activities with "Failed" in their row text are shown. The frame title updates to "asg-activities(M/N) -- asg-name". |
| B.13.2 | I press / and type "Launching". | Only activities whose Description contains "Launching" are shown. Activities about terminating are filtered out. |
| B.13.3 | I press / and type an instance ID fragment (e.g., "i-0abc"). | Only activities whose Description mentions that instance are shown. |
| B.13.4 | I press Escape while filter is active. | The filter clears. All activities reappear. |

### B.14 Copy (c)

| ID | Story | Expected |
|----|-------|----------|
| B.14.1 | I select an activity and press c. | The Description text is copied to the system clipboard. A green flash message "Copied!" appears in the header right side. |
| B.14.2 | After ~2 seconds. | The "Copied!" flash message auto-clears and the header right reverts to "? for help". |
| B.14.3 | I paste from clipboard. | The pasted text matches the full Description (e.g., "Launching a new EC2 instance: i-0abc123def456789a"). |

### B.15 Detail View (d)

| ID | Story | Expected |
|----|-------|----------|
| B.15.1 | I select an activity and press d. | The detail view opens for the selected activity. |
| B.15.2 | I verify all detail fields are present. | The detail view shows: ActivityId, StartTime, EndTime, StatusCode, StatusMessage, Description, Cause, Details, Progress, AutoScalingGroupName, AutoScalingGroupARN, AutoScalingGroupState. |
| B.15.3 | The EndTime field for a completed activity. | EndTime shows the timestamp when the activity finished. |
| B.15.4 | The EndTime field for an in-progress activity. | EndTime shows null/empty/dash since the activity has not yet completed. |
| B.15.5 | I press Escape on the detail view. | I return to the activities list. The cursor position is preserved. |

### B.16 YAML View (y)

| ID | Story | Expected |
|----|-------|----------|
| B.16.1 | I select an activity and press y. | The YAML view opens. The full activity is rendered as syntax-highlighted YAML. |
| B.16.2 | The Cause field in YAML is a long multi-line string. | The Cause text wraps correctly in the YAML rendering. Scroll is available if it exceeds the viewport. |
| B.16.3 | I press Escape on the YAML view. | I return to the activities list. |

### B.17 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| B.17.1 | I press ctrl+r on the activities list. | The loading spinner appears. A fresh `DescribeScalingActivities` call is made. The table repopulates with current data. |
| B.17.2 | A new scaling activity occurred since last load. I press ctrl+r. | The new activity appears at the top of the list (newest first). The count in the frame title increments. |
| B.17.3 | I had a filter active and press ctrl+r. | The data refreshes. The filter remains applied to the new data. |

### B.18 Escape (Back to ASG List)

| ID | Story | Expected |
|----|-------|----------|
| B.18.1 | I press Escape on the activities list. | I return to the Auto Scaling Groups list. The cursor is on the same ASG I had selected. |

### B.19 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| B.19.1 | I press ? on the activities list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: SCALING ACTIVITIES, GENERAL, NAVIGATION, HOTKEYS. |
| B.19.2 | The SCALING ACTIVITIES column shows: `<esc>` Back, `<d>` Detail, `<y>` YAML, `<c>` Copy Desc. | These entries match the help wireframe from the design spec. |
| B.19.3 | I press any key on the help screen. | The help screen closes and the activities table reappears. |

### B.20 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| B.20.1 | I press : on the activities list. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| B.20.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list. |
| B.20.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". |

---

## C. ELB Listeners View

### C.1 Loading State

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | I select a load balancer in the ELB list and press Enter. | A spinner (animated dot) is displayed centered inside the frame. The text reads "Fetching listeners..." (or similar). The frame title reads "elb-listeners" with no count. |
| C.1.2 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. |
| C.1.3 | The API responds successfully with listener data. | The spinner disappears. The table renders with listeners. The frame title updates to "elb-listeners(N) -- alb-name" where N is the listener count and alb-name is the load balancer name. |
| C.1.4 | The API responds with an error (e.g., load balancer deleted). | The spinner disappears. A red error flash message appears in the header right side. |

**AWS comparison:**
```
aws elbv2 describe-listeners --load-balancer-arn arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/api-prod-alb/abc123
```
Expected fields visible: Port, Protocol, Action, Target, SSL Policy, Certificate

### C.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | The load balancer has zero listeners (possible immediately after creation before listeners are configured). | The frame title reads "elb-listeners(0) -- alb-name". The content area shows a centered message. |

### C.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| C.3.1 | Listeners load and the table renders. | Six columns are displayed: "Port" (width 8), "Protocol" (width 10), "Action" (width 16), "Target" (width 32), "SSL Policy" (width 24), "Certificate" (width 32). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| C.3.2 | I verify the Port column data. | "Port" maps to the listener's port number (e.g., 80, 443, 8443). |
| C.3.3 | I verify the Protocol column data. | "Protocol" maps to the listener's protocol (e.g., HTTP, HTTPS, TCP, TLS, UDP, TCP_UDP). |
| C.3.4 | I verify the Action column data (computed field). | "Action" shows the default action type extracted from the first default action (e.g., "forward", "redirect", "fixed-response", "authenticate-cognito", "authenticate-oidc"). |
| C.3.5 | I verify the Target column data (computed field). | For "forward" actions, "Target" shows the target group name. For "redirect" actions, it shows the redirect URL pattern. For "fixed-response" actions, it shows the status code. |
| C.3.6 | I verify the SSL Policy column. | For HTTPS/TLS listeners, "SSL Policy" shows the policy name (e.g., "ELBSecurityPolicy-2016-08"). For HTTP/TCP listeners, it shows empty/dash. |
| C.3.7 | I verify the Certificate column (computed field). | For HTTPS/TLS listeners, "Certificate" shows the domain name extracted from the ACM certificate ARN (e.g., "*.example.com"). For HTTP/TCP listeners, it shows empty/dash. |
| C.3.8 | The terminal is narrower than the combined column widths (8+10+16+32+24+32=122 plus borders). | The rightmost column(s) are hidden. Horizontal scroll with h/l is available. |

**AWS comparison:**
```
aws elbv2 describe-listeners --load-balancer-arn ARN --query 'Listeners[].{Port:Port,Protocol:Protocol,DefaultActions:DefaultActions[0].Type,SslPolicy:SslPolicy}'
```

### C.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| C.4.1 | 3 listeners exist for load balancer "api-prod-alb". | The frame top border shows the title centered: "elb-listeners(3) -- api-prod-alb" with equal-length dashes on both sides. |
| C.4.2 | A filter is active and matches 1 of 3 listeners. | The frame title reads "elb-listeners(1/3) -- api-prod-alb". |
| C.4.3 | A filter is active and matches 0 listeners. | The frame title reads "elb-listeners(0/3) -- api-prod-alb". |

### C.5 Row Coloring (No Status Semantics)

| ID | Story | Expected |
|----|-------|----------|
| C.5.1 | Listeners are displayed. | All rows are rendered in PLAIN text color (#c0caf5). Listeners do not have health/status semantics, so there is no status-based row coloring. |
| C.5.2 | I select a row. | The selected row has full-width blue background (#7aa2f7), dark foreground (#1a1b26), bold text. |
| C.5.3 | I move selection away from a row. | The previously selected row reverts to plain coloring. |
| C.5.4 | The list has more than 2 rows. | Alternating rows have a subtle background color difference (#1e2030) for readability. |

### C.6 ALB vs NLB Listener Differences

| ID | Story | Expected |
|----|-------|----------|
| C.6.1 | I view listeners for an Application Load Balancer (ALB). | Listeners show protocols HTTP and HTTPS. Action types include "forward", "redirect", "fixed-response", "authenticate-cognito", "authenticate-oidc". |
| C.6.2 | I view listeners for a Network Load Balancer (NLB). | Listeners show protocols TCP, TLS, UDP, or TCP_UDP. Action types are typically "forward" only. SSL Policy is shown for TLS listeners, empty for TCP/UDP. |
| C.6.3 | An NLB listener uses TLS protocol. | The Protocol column shows "TLS". The SSL Policy column shows the TLS policy name. The Certificate column shows the ACM certificate domain. |
| C.6.4 | An NLB listener uses TCP protocol. | The Protocol column shows "TCP". The SSL Policy column shows empty/dash. The Certificate column shows empty/dash. |

**AWS comparison (ALB):**
```
aws elbv2 describe-listeners --load-balancer-arn arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/abc123
```
**AWS comparison (NLB):**
```
aws elbv2 describe-listeners --load-balancer-arn arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/net/my-nlb/abc123
```

### C.7 HTTP-to-HTTPS Redirect Listener

| ID | Story | Expected |
|----|-------|----------|
| C.7.1 | A listener on port 80 with protocol HTTP has a redirect action to HTTPS. | The Port column shows "80". The Protocol column shows "HTTP". The Action column shows "redirect". The Target column shows the redirect URL pattern (e.g., "https://#{host}:443/#{path}?#{query}"). The SSL Policy shows empty/dash. The Certificate shows empty/dash. |
| C.7.2 | I press d on the HTTP redirect listener. | The detail view opens. DefaultActions shows the redirect configuration including protocol, port, host, path, query, and status code (301 or 302). |

**AWS comparison:**
```
aws elbv2 describe-listeners --load-balancer-arn ARN --query 'Listeners[?Port==`80`].DefaultActions'
```

### C.8 Listener with No SSL Policy (HTTP Only)

| ID | Story | Expected |
|----|-------|----------|
| C.8.1 | A listener uses HTTP (not HTTPS). | The SSL Policy column displays empty/dash. The Certificate column displays empty/dash. This is expected behavior -- HTTP listeners do not use SSL. |
| C.8.2 | I verify via the detail view (d). | The SslPolicy field shows null/empty. The Certificates field shows null/empty. |

### C.9 Multiple Certificates on a Listener

| ID | Story | Expected |
|----|-------|----------|
| C.9.1 | An HTTPS listener has a default certificate and additional certificates (SNI). | The Certificate column in the list view shows the default (first) certificate's domain name. |
| C.9.2 | I press d on the listener with multiple certificates. | The detail view's Certificates field shows all certificates (default and additional), each with their ARN. |

**AWS comparison:**
```
aws elbv2 describe-listener-certificates --listener-arn ARN
```

### C.10 Listener with Fixed Response (No Target Group)

| ID | Story | Expected |
|----|-------|----------|
| C.10.1 | A listener has a default action of type "fixed-response" (e.g., returns 200 with a static body). | The Action column shows "fixed-response". The Target column shows the HTTP status code (e.g., "200 text/plain" or similar summary). No target group is referenced. |
| C.10.2 | I press d to view the detail. | The DefaultActions section shows the FixedResponseConfig with StatusCode, ContentType, and MessageBody. |

**AWS comparison:**
```
aws elbv2 describe-listeners --load-balancer-arn ARN --query 'Listeners[?DefaultActions[0].Type==`fixed-response`]'
```

### C.11 Enter Key (Drill Into Listener Rules)

| ID | Story | Expected |
|----|-------|----------|
| C.11.1 | I select a listener and press Enter. | The view transitions to the Listener Rules view for that listener. A loading spinner appears while rules are fetched. The listeners view is pushed onto the view stack. |
| C.11.2 | I verify Enter navigates to rules, NOT to the detail view. | Pressing Enter on a listener opens the Listener Rules list, NOT the listener detail view. The detail view is accessed via `d` instead. |

### C.12 Navigation

| ID | Story | Expected |
|----|-------|----------|
| C.12.1 | I press j (or down-arrow) with the first listener selected. | The selection cursor moves to the second listener. |
| C.12.2 | I press k (or up-arrow). | The selection cursor moves up. |
| C.12.3 | I press g. | The selection jumps to the first listener. |
| C.12.4 | I press G. | The selection jumps to the last listener. |
| C.12.5 | I press PageDown (or ctrl+d). | The selection moves down by one page of visible rows. |
| C.12.6 | I press PageUp (or ctrl+u). | The selection moves up by one page of visible rows. |
| C.12.7 | I press h (or left-arrow). | Columns scroll left. |
| C.12.8 | I press l (or right-arrow). | Columns scroll right, revealing the SSL Policy and Certificate columns. Column headers scroll in sync. |

### C.13 Sorting

| ID | Story | Expected |
|----|-------|----------|
| C.13.1 | I press N on the listeners list. | Rows are sorted by Port in ascending order. The "Port" column header shows an up-arrow indicator. |
| C.13.2 | I press N again. | Sort order toggles to descending. The indicator changes to a down-arrow. |
| C.13.3 | I press S on the listeners list. | Rows are sorted by Protocol (or the status-equivalent column) in ascending order. The appropriate column header shows the sort indicator. |

### C.14 Filter

| ID | Story | Expected |
|----|-------|----------|
| C.14.1 | I press / and type "HTTPS". | Only listeners with "HTTPS" in their row text are shown. The frame title updates. |
| C.14.2 | I press / and type "443". | Only listeners whose Port is 443 are shown. |
| C.14.3 | I press / and type "redirect". | Only listeners whose Action is "redirect" are shown. |
| C.14.4 | I press Escape while filter is active. | The filter clears. All listeners reappear. |

### C.15 Detail View (d)

| ID | Story | Expected |
|----|-------|----------|
| C.15.1 | I select a listener and press d. | The detail view opens for the selected listener. |
| C.15.2 | I verify the detail fields match views.yaml elb_listeners detail config. | The detail view shows: ListenerArn, Port, Protocol, DefaultActions, SslPolicy, Certificates, AlpnPolicy, MutualAuthentication. |
| C.15.3 | I view detail for an HTTPS listener with ALPN policy. | The AlpnPolicy field shows the negotiation preference (e.g., "H2Preferred"). |
| C.15.4 | I view detail for an HTTP listener. | The SslPolicy, Certificates, AlpnPolicy, and MutualAuthentication fields show null/empty/dash. |
| C.15.5 | I press Escape on the detail view. | I return to the listeners list. The cursor position is preserved. |

**AWS comparison:**
```
aws elbv2 describe-listeners --load-balancer-arn ARN --query 'Listeners[?Port==`443`]'
```
Expected detail fields: ListenerArn, Port, Protocol, DefaultActions, SslPolicy, Certificates, AlpnPolicy, MutualAuthentication

### C.16 YAML View (y)

| ID | Story | Expected |
|----|-------|----------|
| C.16.1 | I select a listener and press y. | The YAML view opens. The frame title includes the listener identifier and "yaml". The full listener resource is rendered as syntax-highlighted YAML. |
| C.16.2 | The DefaultActions array in YAML is properly formatted. | Each action object shows Type, TargetGroupArn (or RedirectConfig/FixedResponseConfig), Order, etc. as nested YAML. |
| C.16.3 | I press Escape on the YAML view. | I return to the listeners list. |

### C.17 Copy (c)

| ID | Story | Expected |
|----|-------|----------|
| C.17.1 | I select a listener and press c. | The Listener ARN is copied to the system clipboard. A green flash message "Copied!" appears in the header right side. |
| C.17.2 | After ~2 seconds. | The "Copied!" flash auto-clears and the header right reverts to "? for help". |
| C.17.3 | I paste from clipboard. | The pasted text matches the full Listener ARN (e.g., "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/my-alb/abc123/def456"). |

### C.18 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| C.18.1 | I press ctrl+r on the listeners list. | The loading spinner appears. A fresh `DescribeListeners` call is made. The table repopulates with current data. |
| C.18.2 | A new listener was added since last load. I press ctrl+r. | The new listener appears in the refreshed list. The count in the frame title increments. |

### C.19 Escape (Back to ELB List)

| ID | Story | Expected |
|----|-------|----------|
| C.19.1 | I press Escape on the listeners list. | I return to the Load Balancers list. The cursor is on the same load balancer I had selected. |

### C.20 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| C.20.1 | I press ? on the listeners list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: LISTENERS, GENERAL, NAVIGATION, HOTKEYS. |
| C.20.2 | The LISTENERS column shows: `<esc>` Back, `<enter>` View Rules, `<d>` Detail, `<y>` YAML, `<c>` Copy ARN. | These entries match the help wireframe from the design spec. Note: `<enter>` is listed (drills into Listener Rules) and `<q>` Quit appears in GENERAL. |
| C.20.3 | I press any key on the help screen. | The help screen closes and the listeners table reappears. |

### C.21 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| C.21.1 | I press : on the listeners list. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| C.21.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list. |
| C.21.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". |

---

## D. Listener Rules View (Nested Child)

### D.1 Loading State

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | I select a listener in the Listeners list and press Enter. | A spinner is displayed centered inside the frame. The text reads "Fetching listener rules..." (or similar). The frame title reads "listener-rules" with no count. |
| D.1.2 | The API responds successfully with rule data. | The spinner disappears. The table renders with rules. The frame title updates to "listener-rules(N) -- :443 HTTPS" where N is the rule count and the subtitle shows the listener's port and protocol. |
| D.1.3 | The API responds with an error. | The spinner disappears. A red error flash message appears in the header right side. |

**AWS comparison:**
```
aws elbv2 describe-rules --listener-arn arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/api-prod-alb/abc123/def456
```
Expected fields visible: Priority, Conditions, Action, Target

### D.2 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| D.2.1 | Listener rules load and the table renders. | Four columns are displayed: "Priority" (width 10), "Conditions" (width 36), "Action" (width 16), "Target" (width 32). Column headers are bold, colored blue (#7aa2f7). |
| D.2.2 | I verify the Priority column. | Shows the numeric priority (1, 2, 3, ...) for custom rules and "default" for the default rule. |
| D.2.3 | I verify the Conditions column (computed field). | Shows a human-readable summary (e.g., "path: /api/v1/*", "host: api.example.com", "path: /health AND header: X-Custom=true"). |
| D.2.4 | I verify the Action column (computed field). | Shows the action type (e.g., "forward", "redirect", "fixed-response"). |
| D.2.5 | I verify the Target column (computed field). | For forward actions, shows the target group name. For redirect actions, shows the redirect URL. For fixed-response, shows the status code and content type. |

**AWS comparison:**
```
aws elbv2 describe-rules --listener-arn ARN --query 'Rules[].{Priority:Priority,Conditions:Conditions,Actions:Actions[0].Type}'
```

### D.3 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| D.3.1 | 5 rules exist for the HTTPS:443 listener. | The frame top border shows the title centered: "listener-rules(5) -- :443 HTTPS". |
| D.3.2 | A filter is active and matches 2 of 5 rules. | The frame title reads "listener-rules(2/5) -- :443 HTTPS". |

### D.4 Row Coloring

| ID | Story | Expected |
|----|-------|----------|
| D.4.1 | Custom rules (priority 1, 2, 3, ...) are displayed. | All custom rule rows are rendered in PLAIN text color (#c0caf5). Rules do not have health/status semantics. |
| D.4.2 | The default rule is displayed. | The default rule row is rendered DIM (#565f89) to visually distinguish it from custom rules. |
| D.4.3 | I select a row. | The selected row has full-width blue background, overriding plain or DIM coloring. |

### D.5 Path-Based Routing Rules

| ID | Story | Expected |
|----|-------|----------|
| D.5.1 | A rule has a path-pattern condition "/api/v1/*". | The Conditions column shows "path: /api/v1/*". The Action column shows "forward". The Target column shows the target group name. |
| D.5.2 | A rule has multiple conditions (path AND host). | The Conditions column shows a combined summary (e.g., "host: api.example.com AND path: /v2/*"). |

### D.6 Fixed Response Rule

| ID | Story | Expected |
|----|-------|----------|
| D.6.1 | A rule returns a fixed response (e.g., health check endpoint returning 200). | The Priority column shows the numeric priority. The Action column shows "fixed-response". The Target column shows "200 text/plain" (or similar status code + content type summary). |

### D.7 Default Rule

| ID | Story | Expected |
|----|-------|----------|
| D.7.1 | Every listener has a default rule (lowest priority, always present). | The default rule appears at the bottom of the list with Priority "default" and Conditions showing "--" (no conditions). |
| D.7.2 | The default rule typically forwards to a target group. | Action shows "forward" and Target shows the default target group name. |

### D.8 Navigation and Key Bindings

| ID | Story | Expected |
|----|-------|----------|
| D.8.1 | I press j/k/g/G/PageUp/PageDown in the rules list. | Navigation behaves identically to other list views. |
| D.8.2 | I press h/l to scroll columns horizontally. | Columns scroll left/right. Column headers scroll in sync. |
| D.8.3 | I select a rule and press d. | The detail view opens showing: RuleArn, Priority, Conditions, Actions, IsDefault. |
| D.8.4 | I select a rule and press y. | The YAML view opens showing the full rule as syntax-highlighted YAML. |
| D.8.5 | I select a rule and press c. | The conditions summary is copied to the clipboard (e.g., "path: /api/v1/*"). A "Copied!" flash appears. |

**AWS comparison:**
```
aws elbv2 describe-rules --listener-arn ARN --query 'Rules[?Priority!=`default`]'
```
Expected detail fields: RuleArn, Priority, Conditions, Actions, IsDefault

### D.9 Filter

| ID | Story | Expected |
|----|-------|----------|
| D.9.1 | I press / and type "/api". | Only rules whose Conditions contain "/api" are shown. |
| D.9.2 | I press / and type "forward". | Only rules whose Action is "forward" are shown. |
| D.9.3 | I press Escape while filter is active. | The filter clears. All rules reappear. |

### D.10 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| D.10.1 | I press ctrl+r on the rules list. | A fresh `DescribeRules` call is made. The table repopulates with current data. |
| D.10.2 | A new rule was added since last load. I press ctrl+r. | The new rule appears in the refreshed list. |

### D.11 Escape (Back to Listeners)

| ID | Story | Expected |
|----|-------|----------|
| D.11.1 | I press Escape on the rules list. | I return to the Listeners list. The cursor is on the same listener I had selected. |

### D.12 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| D.12.1 | I press ? on the rules list. | The help screen replaces the table content. It displays categories: LISTENER RULES, GENERAL, NAVIGATION, HOTKEYS. |
| D.12.2 | The LISTENER RULES column shows: `<esc>` Back, `<d>` Detail, `<y>` YAML, `<c>` Copy Rule. | These entries match the help wireframe from the design spec. |
| D.12.3 | I press any key on the help screen. | The help screen closes and the rules table reappears. |

---

## E. Cross-Cutting Concerns

### E.1 View Stack: ELB --> Listeners --> Rules --> Detail/YAML

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | Main Menu --> ELB list --> select ALB --> Enter (listeners) --> select listener --> Enter (rules) --> select rule --> d (detail). Then Escape four times. | Each Escape pops one level: Detail --> Rules --> Listeners --> ELB list --> Main Menu. No state is lost at any intermediate level. Cursor positions are preserved. |
| E.1.2 | Main Menu --> ELB list --> select ALB --> Enter (listeners) --> select listener --> y (YAML). Then Escape twice. | YAML --> Listeners --> ELB list. The cursor in the Listeners view is on the same listener. The cursor in the ELB list is on the same ALB. |
| E.1.3 | Main Menu --> ELB list --> Enter --> Enter --> d --> y (chaining detail to YAML). Then Escape three times. | YAML --> Detail --> Rules --> Listeners. |

### E.2 View Stack: TG --> Target Health --> Detail/YAML

| ID | Story | Expected |
|----|-------|----------|
| E.2.1 | Main Menu --> Target Groups --> select TG --> Enter (target health) --> select target --> d (detail). Then Escape three times. | Detail --> Target Health --> Target Groups --> Main Menu. Cursor positions preserved at each level. |
| E.2.2 | Main Menu --> Target Groups --> Enter --> y (YAML on target health). Then Escape twice. | YAML --> Target Health --> Target Groups. |

### E.3 View Stack: ASG --> Activities --> Detail/YAML

| ID | Story | Expected |
|----|-------|----------|
| E.3.1 | Main Menu --> ASG --> select ASG --> Enter (activities) --> select activity --> d (detail). Then Escape three times. | Detail --> Activities --> ASG --> Main Menu. Cursor positions preserved at each level. |
| E.3.2 | Main Menu --> ASG --> Enter --> select activity --> y (YAML). Then Escape twice. | YAML --> Activities --> ASG. |

### E.4 Header Consistency

| ID | Story | Expected |
|----|-------|----------|
| E.4.1 | In every child view (target health, activities, listeners, rules), the header displays: "a9s" (accent bold), version (dim), profile:region (bold). | Visual inspection confirms across all child views. |
| E.4.2 | The header right side shows "? for help" in normal mode across all child views. | Confirmed in target health, activities, listeners, and rules views. |
| E.4.3 | I switch profile while in a child view. | The header updates to reflect the new profile. The child view refreshes or navigates back to reflect the new AWS context. |
| E.4.4 | I switch region while in a child view. | The header updates to reflect the new region. The child view refreshes or navigates back to reflect the new AWS context. |

### E.5 Terminal Resize

| ID | Story | Expected |
|----|-------|----------|
| E.5.1 | I resize the terminal while viewing target health. | The layout reflows. Column visibility adjusts to the new width. The frame border redraws correctly. |
| E.5.2 | I resize the terminal while viewing scaling activities. | Same reflow behavior. The Description and Cause columns adjust visibility based on new width. |
| E.5.3 | I resize the terminal while viewing listeners. | Same reflow behavior. |
| E.5.4 | I resize the terminal to below 60 columns. | An error message appears: "Terminal too narrow. Please resize." |
| E.5.5 | I resize the terminal to below 7 lines. | An error message appears: "Terminal too short. Please resize." |

### E.6 Alternating Row Colors

| ID | Story | Expected |
|----|-------|----------|
| E.6.1 | The target health list has more than 2 rows. | Alternating rows have a subtle background color difference (#1e2030) for readability. Selected row always has blue background regardless. Health-state coloring takes precedence over alternating row background for non-selected rows. |
| E.6.2 | The activities list has more than 2 rows. | Same alternating row pattern applies. Status coloring takes precedence. |
| E.6.3 | The listeners list has more than 2 rows. | Alternating rows have the subtle background (#1e2030) since listeners have no status-based coloring. |
| E.6.4 | The rules list has more than 2 rows. | Alternating rows apply for custom rules. The default rule is DIM regardless. |

### E.7 Cross-Referencing: Target Group Health and ELB Listeners

| ID | Story | Expected |
|----|-------|----------|
| E.7.1 | I view an ALB's listeners and see a listener forwarding to target group "api-prod-tg". Then I navigate back and view the Target Groups list, select "api-prod-tg", and press Enter. | The target health view shows the same targets that the ALB is routing to. The health states reflect what the ALB's health checks report. |
| E.7.2 | I view target health showing an unhealthy target. I then navigate to the ELB listener that forwards to this target group. | The listener list does not show per-target health (listeners have no health semantics), but the target group referenced in the Target column is the same one with the unhealthy target. |
| E.7.3 | I copy a Target ID from the target health view (c key), navigate to EC2 instances, and use the filter (/) to search for it. | The same instance ID appears in the EC2 instances list, allowing me to cross-reference the target's health with the instance's details. |
| E.7.4 | I copy a Listener ARN from the listeners view, navigate to the listener rules. | The rules shown are for the specific listener whose ARN I copied. The view stack correctly associates the rules with the listener. |

### E.8 Error Handling

| ID | Story | Expected |
|----|-------|----------|
| E.8.1 | AWS credentials expire while I am viewing target health. I press ctrl+r. | A red error flash appears: "Error: ExpiredToken" (or similar). The previous data remains visible until I fix credentials and refresh again. |
| E.8.2 | The target group ARN is invalid or the target group was deleted. I enter the target health view. | A red error flash appears. The frame shows an empty or error state. I can press Escape to return to the Target Groups list. |
| E.8.3 | Network timeout while loading scaling activities. | A red error flash appears after the timeout. The spinner stops. I can retry with ctrl+r. |
| E.8.4 | I lack `elasticloadbalancingv2:DescribeListeners` permission. I try to view listeners. | A red error flash appears with an access denied message. I can press Escape to return to the ELB list. |
| E.8.5 | I lack `autoscaling:DescribeScalingActivities` permission. I try to view activities. | A red error flash appears with an access denied message. I can press Escape to return to the ASG list. |
| E.8.6 | I lack `elasticloadbalancingv2:DescribeTargetHealth` permission. I try to view target health. | A red error flash appears with an access denied message. I can press Escape to return to the Target Groups list. |

### E.9 Pagination (ASG Activities Only)

| ID | Story | Expected |
|----|-------|----------|
| E.9.1 | An ASG has more than 100 scaling activities (API returns paginated). | All activities are loaded transparently. The frame title shows the full count (e.g., "asg-activities(237) -- asg-name"). The user does not see pagination mechanics. |
| E.9.2 | I scroll to the bottom of a long activities list. | All activities are available for scrolling. No "load more" prompt is needed -- pagination is handled automatically. |

### E.10 Copy Behavior Summary

| ID | View | Copy Key (c) Copies | Expected |
|----|------|---------------------|----------|
| E.10.1 | Target Health | Target ID | Instance ID (e.g., "i-0a1b2c...") or IP address (e.g., "10.0.1.47") |
| E.10.2 | Scaling Activities | Description text | Full description (e.g., "Launching a new EC2 instance: i-0abc123...") |
| E.10.3 | Listeners | Listener ARN | Full ARN (e.g., "arn:aws:elasticloadbalancing:...") |
| E.10.4 | Listener Rules | Conditions summary | Human-readable rule (e.g., "path: /api/v1/*") |
