# QA User Stories: Miscellaneous Child Views

Covers five child view implementations:
- **A.** SNS Topics --> Subscriptions
- **B.** EventBridge Rules --> Targets
- **C.** Glue Jobs --> Job Runs
- **D.** IAM Groups --> Group Members
- **E.** ELB Listeners --> Listener Rules (3-level drill-down)

All stories are written from a black-box perspective against the design spec and
`views.yaml` configuration files.

AWS CLI equivalents are cited so testers can verify data parity.

---

## A. SNS Topics --> Subscriptions

### A.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| A.1.1 | I select an SNS topic "critical-alerts-prod" and press Enter. | The view transitions to the SNS Subscriptions list. A spinner appears centered in the frame with text like "Fetching SNS subscriptions..." while the API call is in flight. The SNS Topics list is pushed onto the view stack. |
| A.1.2 | The API responds successfully with 5 subscriptions. | The spinner disappears. The table renders with column headers and rows. The frame title updates to `sns-subs(5) -- critical-alerts-prod`. |
| A.1.3 | I press keys (j, k, /, N) while the spinner is visible. | No navigation or sort occurs. Keypresses are ignored or queued until data loads. The spinner continues uninterrupted. |
| A.1.4 | The API responds with an error (e.g., TopicNotFound, no permissions). | The spinner disappears. A red error flash message appears in the header right side (e.g., "Error: topic not found"). |

**AWS comparison:**
```
aws sns list-subscriptions-by-topic --topic-arn arn:aws:sns:us-east-1:123456789012:critical-alerts-prod
```

### A.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| A.2.1 | The SNS topic has zero subscriptions. | The frame title reads `sns-subs(0) -- topic-name`. The content area shows a centered message (e.g., "No subscriptions found") with a hint to refresh or check the topic. No column headers are shown (or headers with no data rows). |
| A.2.2 | I press ctrl+r on the empty state. | The loading spinner appears again while the refresh request is in flight. |

### A.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| A.3.1 | Subscriptions load and the table renders. | Four columns are displayed: "Protocol" (width 10), "Endpoint" (width 48), "Status" (width 18), "Owner" (width 14). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| A.3.2 | I verify column data against the AWS CLI. | "Protocol" maps to `.Subscriptions[].Protocol`. "Endpoint" maps to `.Subscriptions[].Endpoint`. "Status" is computed from `SubscriptionArn` (Confirmed if it is a real ARN, PendingConfirmation if the ARN value equals "PendingConfirmation"). "Owner" maps to `.Subscriptions[].Owner`. |
| A.3.3 | An endpoint value (e.g., a long Lambda ARN) exceeds its 48-character column width. | The endpoint is truncated to fit. No row wrapping occurs. |
| A.3.4 | The terminal is narrower than the combined column widths (10+48+18+14 = 90 plus borders). | The rightmost column(s) are hidden (not truncated mid-value). Horizontal scroll with h/l is available to reveal hidden columns. |

**AWS comparison:**
```
aws sns list-subscriptions-by-topic --topic-arn arn:aws:sns:us-east-1:123456789012:critical-alerts-prod --query 'Subscriptions[].{Protocol:Protocol,Endpoint:Endpoint,SubscriptionArn:SubscriptionArn,Owner:Owner}'
```
Expected fields visible: Protocol, Endpoint, Status (derived from SubscriptionArn), Owner

### A.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| A.4.1 | 5 subscriptions are loaded for topic "critical-alerts-prod". | The frame top border shows the title centered: `sns-subs(5) -- critical-alerts-prod` with equal-length dashes on both sides. |
| A.4.2 | A filter is active and matches 2 of 5 subscriptions. | The frame title reads `sns-subs(2/5) -- critical-alerts-prod`. |
| A.4.3 | A filter is active and matches 0 subscriptions. | The frame title reads `sns-subs(0/5) -- critical-alerts-prod`. The content area is empty (no rows). |

### A.5 Row Coloring by Confirmation Status

| ID | Story | Expected |
|----|-------|----------|
| A.5.1 | A subscription has Status "Confirmed" (its SubscriptionArn is a full ARN). | The entire row is rendered in GREEN (#9ece6a). |
| A.5.2 | A subscription has Status "PendingConfirmation" (its SubscriptionArn equals "PendingConfirmation"). | The entire row is rendered in YELLOW (#e0af68). This makes unconfirmed subscriptions immediately visible. |
| A.5.3 | I select a PendingConfirmation row. | The selected row has full-width blue background (#7aa2f7), dark foreground (#1a1b26), bold. The yellow coloring is overridden by the selection highlight. |
| A.5.4 | I move selection away from the PendingConfirmation row. | The row reverts to yellow text (#e0af68). |

### A.6 Mixed Protocol Types

| ID | Story | Expected |
|----|-------|----------|
| A.6.1 | The topic has an email subscription. | The Protocol column shows "email" and the Endpoint column shows the email address (e.g., "oncall-team@company.com"). |
| A.6.2 | The topic has an HTTPS subscription. | The Protocol column shows "https" and the Endpoint column shows the full URL (e.g., "https://hooks.slack.com/services/T01/B02/xyz"). |
| A.6.3 | The topic has a Lambda subscription. | The Protocol column shows "lambda" and the Endpoint column shows the Lambda function ARN (e.g., "arn:aws:lambda:us-east-1:123456:function:alert-handler"). |
| A.6.4 | The topic has an SQS subscription. | The Protocol column shows "sqs" and the Endpoint column shows the SQS queue ARN. |
| A.6.5 | The topic has an SMS subscription. | The Protocol column shows "sms" and the Endpoint column shows the phone number. |
| A.6.6 | AWS returns a partially obscured email endpoint (e.g., "***@example.com"). | The Endpoint column displays the obscured value exactly as returned by AWS. This is an AWS-side security behavior, not something a9s controls. |

**AWS comparison:**
```
aws sns list-subscriptions-by-topic --topic-arn arn:aws:sns:us-east-1:123456789012:mixed-protocol-topic
```

### A.7 Navigation

| ID | Story | Expected |
|----|-------|----------|
| A.7.1 | I press j (or down-arrow) with the first subscription selected. | The selection cursor moves to the second subscription. The previously selected row loses the blue highlight and reverts to its confirmation-status color. |
| A.7.2 | I press k (or up-arrow) with the second subscription selected. | The selection cursor moves back to the first subscription. |
| A.7.3 | I press g. | The selection jumps to the very first subscription. |
| A.7.4 | I press G. | The selection jumps to the very last subscription. |
| A.7.5 | I press PageDown (or ctrl+d). | The selection moves down by one page of visible rows. |
| A.7.6 | I press PageUp (or ctrl+u). | The selection moves up by one page of visible rows. |
| A.7.7 | I press h (or left-arrow). | Columns scroll left, revealing any previously hidden left columns. |
| A.7.8 | I press l (or right-arrow). | Columns scroll right, revealing any previously hidden right columns. |

### A.8 Filter

| ID | Story | Expected |
|----|-------|----------|
| A.8.1 | I press / in the subscription list. | The header right side changes from "? for help" to "/|" (amber/bold, with cursor). Filter mode is active. |
| A.8.2 | I type "email" in filter mode. | Only subscriptions whose visible fields contain "email" (case-insensitive) are displayed. The frame title updates to `sns-subs(M/N) -- topic-name`. |
| A.8.3 | I type "Pending" in filter mode. | Only subscriptions with "PendingConfirmation" status are shown. |
| A.8.4 | I press Escape in filter mode. | The filter is cleared. All rows reappear. The header right reverts to "? for help". |

### A.9 Sort

| ID | Story | Expected |
|----|-------|----------|
| A.9.1 | I press N on the subscription list. | Rows are sorted by Protocol in ascending order. The "Protocol" column header shows a sort indicator (up-arrow). |
| A.9.2 | I press N again. | Sort order toggles to descending. The indicator changes to a down-arrow. |
| A.9.3 | I press S on the subscription list. | Rows are sorted by confirmation status. |

### A.10 Copy (c)

| ID | Story | Expected |
|----|-------|----------|
| A.10.1 | I select an email subscription and press c. | The Endpoint value (the email address) is copied to the system clipboard. A green "Copied!" flash appears in the header right side. |
| A.10.2 | I select a Lambda subscription and press c. | The Endpoint value (the Lambda ARN) is copied to the clipboard. |
| A.10.3 | I select an HTTPS subscription and press c. | The Endpoint value (the webhook URL) is copied to the clipboard. |
| A.10.4 | After ~2 seconds. | The "Copied!" flash auto-clears and the header right reverts to "? for help". |
| A.10.5 | I paste from clipboard into another application. | The pasted text matches the endpoint value exactly. |

### A.11 Detail and YAML

| ID | Story | Expected |
|----|-------|----------|
| A.11.1 | I select a subscription and press d. | The detail view opens. The frame title shows the subscription info. The detail view shows key-value pairs for: SubscriptionArn, Protocol, Endpoint, Owner, TopicArn. |
| A.11.2 | I select a subscription and press y. | The YAML view opens. The full subscription data is rendered as syntax-highlighted YAML. |
| A.11.3 | I press Escape on the detail view. | I return to the subscriptions list. The cursor position is preserved. |

### A.12 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| A.12.1 | I press ctrl+r on the subscription list. | The loading spinner appears. A fresh API call is made. When it completes, the table repopulates with current data. |
| A.12.2 | A new subscription was added to the topic since the last load. I press ctrl+r. | The new subscription appears in the refreshed list. The count in the frame title increments. |
| A.12.3 | A PendingConfirmation subscription was confirmed since the last load. I press ctrl+r. | After refresh, the subscription row changes from yellow to green as its status becomes "Confirmed". |

### A.13 Escape (Back to SNS Topics)

| ID | Story | Expected |
|----|-------|----------|
| A.13.1 | I press Escape on the subscription list. | I return to the SNS Topics list. The cursor is on the same topic I had entered. |

### A.14 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| A.14.1 | I press ? on the subscription list. | The help screen replaces the table content inside the frame. It displays a four-column layout with categories: SUBSCRIPTIONS, GENERAL, NAVIGATION, HOTKEYS. The SUBSCRIPTIONS column includes `<c> Copy Endpoint`. |
| A.14.2 | I press any key on the help screen. | The help screen closes and the subscription list reappears. |

### A.15 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| A.15.1 | I press : on the subscription list. | The header right side changes to ":|" (amber/bold). Command mode is active. |
| A.15.2 | I type "ec2" and press Enter. | The view navigates to the EC2 instances list. |
| A.15.3 | I press Escape in command mode. | Command mode is cancelled. The header reverts to "? for help". The subscription list remains. |

---

## B. EventBridge Rules --> Targets

### B.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| B.1.1 | I select an EventBridge rule "daily-etl-trigger" and press Enter. | The view transitions to the EventBridge Targets list. A spinner appears centered in the frame with text like "Fetching rule targets..." while the API call is in flight. The EventBridge Rules list is pushed onto the view stack. |
| B.1.2 | The API responds successfully with 2 targets. | The spinner disappears. The table renders with column headers and rows. The frame title updates to `eb-targets(2) -- daily-etl-trigger`. |
| B.1.3 | The API responds with an error (e.g., ResourceNotFoundException). | The spinner disappears. A red error flash message appears in the header right side. |

**AWS comparison:**
```
aws events list-targets-by-rule --rule daily-etl-trigger
```

### B.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| B.2.1 | The EventBridge rule has zero targets (a rule with no targets configured). | The frame title reads `eb-targets(0) -- rule-name`. The content area shows a centered message (e.g., "No targets found"). |
| B.2.2 | I press ctrl+r on the empty state. | The loading spinner appears again while the refresh request is in flight. |

### B.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| B.3.1 | Targets load and the table renders. | Four columns are displayed: "Target ID" (width 20), "Target ARN" (width 48), "Resource" (width 28), "Input" (width 36). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| B.3.2 | I verify column data against the AWS CLI. | "Target ID" maps to `.Targets[].Id`. "Target ARN" maps to `.Targets[].Arn`. "Resource" is computed from the ARN (e.g., "Lambda: data-pipeline-daily"). "Input" shows a summary of input configuration. |
| B.3.3 | A Target ARN exceeds its 48-character column width. | The ARN is truncated to fit. No row wrapping occurs. |
| B.3.4 | The terminal is narrower than the combined column widths (20+48+28+36 = 132 plus borders). | The rightmost column(s) are hidden. Horizontal scroll with h/l is available. |

**AWS comparison:**
```
aws events list-targets-by-rule --rule daily-etl-trigger --query 'Targets[].{Id:Id,Arn:Arn}'
```
Expected fields visible: Target ID, Target ARN, Resource (computed), Input (computed)

### B.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| B.4.1 | 2 targets are loaded for rule "daily-etl-trigger". | The frame top border shows the title centered: `eb-targets(2) -- daily-etl-trigger`. |
| B.4.2 | A filter is active and matches 1 of 2 targets. | The frame title reads `eb-targets(1/2) -- daily-etl-trigger`. |
| B.4.3 | A filter is active and matches 0 targets. | The frame title reads `eb-targets(0/2) -- daily-etl-trigger`. |

### B.5 Row Coloring

| ID | Story | Expected |
|----|-------|----------|
| B.5.1 | Targets are displayed. | All rows are rendered in plain text color (#c0caf5). Targets have no health/status, so no status-based coloring applies. |
| B.5.2 | I select a row. | The selected row has full-width blue background (#7aa2f7), dark foreground (#1a1b26), bold. |
| B.5.3 | I move selection away from a row. | The previously selected row reverts to plain coloring (#c0caf5). |

### B.6 Resource Type Extraction from ARN

| ID | Story | Expected |
|----|-------|----------|
| B.6.1 | A target has a Lambda function ARN (`arn:aws:lambda:us-east-1:123456:function:data-pipeline-daily`). | The "Resource" column shows "Lambda: data-pipeline-daily". |
| B.6.2 | A target has an SQS queue ARN (`arn:aws:sqs:us-east-1:123456:processing-queue`). | The "Resource" column shows "SQS: processing-queue". |
| B.6.3 | A target has a Step Functions ARN (`arn:aws:states:us-east-1:123456:stateMachine:order-workflow`). | The "Resource" column shows "SFN: order-workflow". |
| B.6.4 | A target has an ECS cluster ARN (`arn:aws:ecs:us-east-1:123456:cluster/prod`). | The "Resource" column shows "ECS: prod". |
| B.6.5 | A target has a Kinesis stream ARN. | The "Resource" column shows a human-readable name extracted from the ARN. |

### B.7 Input Summary

| ID | Story | Expected |
|----|-------|----------|
| B.7.1 | A target has a constant Input JSON (`{"mode": "full"}`). | The "Input" column shows the JSON string, truncated to fit the 36-character width. |
| B.7.2 | A target has an InputPath set (e.g., `$.detail`). | The "Input" column shows the JSONPath expression (e.g., `$.detail`). |
| B.7.3 | A target has an InputTransformer configured. | The "Input" column shows "InputTransformer". |
| B.7.4 | A target passes the event through unmodified (no Input, InputPath, or InputTransformer). | The "Input" column shows a dash or empty indicator (e.g., "--"). |

### B.8 Navigation

| ID | Story | Expected |
|----|-------|----------|
| B.8.1 | I press j/k/g/G/PageUp/PageDown in the targets list. | Navigation behaves identically to other list views. |
| B.8.2 | I press h/l to scroll columns horizontally. | Columns scroll to reveal the Input column (which is likely hidden at standard 80-col width). |

### B.9 Filter

| ID | Story | Expected |
|----|-------|----------|
| B.9.1 | I press / and type "lambda". | Only targets whose visible fields contain "lambda" (case-insensitive) are shown. This matches both the Target ARN (containing "lambda") and the Resource column (containing "Lambda:"). |
| B.9.2 | I type "sqs" in filter mode. | Only targets with SQS-related ARNs or resource names are shown. |
| B.9.3 | I press Escape while filter is active. | The filter clears. All targets reappear. |

### B.10 Copy (c)

| ID | Story | Expected |
|----|-------|----------|
| B.10.1 | I select a target and press c. | The Target ARN is copied to the system clipboard. A green "Copied!" flash appears in the header right side. |
| B.10.2 | After ~2 seconds. | The "Copied!" flash auto-clears and the header right reverts to "? for help". |
| B.10.3 | I paste from clipboard into another application. | The pasted text matches the Target ARN exactly (the full ARN, not the truncated display). |

### B.11 Detail and YAML

| ID | Story | Expected |
|----|-------|----------|
| B.11.1 | I select a target and press d. | The detail view opens. The detail view shows key-value pairs for: Id, Arn, RoleArn, Input, InputPath, InputTransformer, DeadLetterConfig, RetryPolicy, SqsParameters, EcsParameters, KinesisParameters, BatchParameters, HttpParameters, SageMakerPipelineParameters, RedshiftDataParameters, AppSyncParameters. |
| B.11.2 | A target has no DeadLetterConfig, RetryPolicy, or EcsParameters. | Those fields show null/empty/dash in the detail view rather than crashing. |
| B.11.3 | I select a target and press y. | The YAML view opens showing the full target metadata as syntax-highlighted YAML. |
| B.11.4 | I press Escape on the detail view. | I return to the targets list. The cursor position is preserved. |

### B.12 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| B.12.1 | I press ctrl+r on the targets list. | The loading spinner appears. A fresh API call is made. When it completes, the table repopulates. |
| B.12.2 | A new target was added to the rule since the last load. I press ctrl+r. | The new target appears. The count in the frame title increments. |

### B.13 Escape (Back to EventBridge Rules)

| ID | Story | Expected |
|----|-------|----------|
| B.13.1 | I press Escape on the targets list. | I return to the EventBridge Rules list. The cursor is on the same rule I had entered. |

### B.14 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| B.14.1 | I press ? on the targets list. | The help screen replaces the table content. It displays a four-column layout with categories: EB TARGETS, GENERAL, NAVIGATION, HOTKEYS. The EB TARGETS column includes `<c> Copy ARN`. |
| B.14.2 | I press any key on the help screen. | The help screen closes and the targets list reappears. |

### B.15 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| B.15.1 | I press : on the targets list. | Command mode activates in the header. |
| B.15.2 | I type "s3" and press Enter. | The view navigates to the S3 Buckets list. |
| B.15.3 | I press Escape in command mode. | Command mode is cancelled. The targets list remains. |

---

## C. Glue Jobs --> Job Runs

### C.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| C.1.1 | I select a Glue job "nightly-etl-transform" and press Enter. | The view transitions to the Glue Job Runs list. A spinner appears centered in the frame with text like "Fetching Glue job runs..." while the API call is in flight. The Glue Jobs list is pushed onto the view stack. |
| C.1.2 | The API responds successfully with 30 runs. | The spinner disappears. The table renders with column headers and rows. The frame title updates to `glue-runs(30) -- nightly-etl-transform`. Runs are ordered newest first. |
| C.1.3 | The API responds with an error (e.g., EntityNotFoundException, no permissions). | The spinner disappears. A red error flash message appears in the header right side. |

**AWS comparison:**
```
aws glue get-job-runs --job-name nightly-etl-transform
```

### C.2 Empty State (Job Never Run)

| ID | Story | Expected |
|----|-------|----------|
| C.2.1 | The Glue job has never been run (zero job runs). | The frame title reads `glue-runs(0) -- job-name`. The content area shows a centered message (e.g., "No job runs found"). |
| C.2.2 | I press ctrl+r on the empty state. | The loading spinner appears while the refresh request is in flight. |

### C.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| C.3.1 | Job runs load and the table renders. | Six columns are displayed: "Run ID" (width 12), "State" (width 12), "Started" (width 22), "Execution Time" (width 14), "Error Message" (width 44), "DPU Hours" (width 10). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| C.3.2 | I verify column data against the AWS CLI. | "Run ID" is the first 8 characters of `.JobRuns[].Id`. "State" maps to `.JobRuns[].JobRunState`. "Started" maps to `.JobRuns[].StartedOn`. "Execution Time" is `.JobRuns[].ExecutionTime` formatted as human-readable (e.g., "47m 23s"). "Error Message" maps to `.JobRuns[].ErrorMessage`. "DPU Hours" is `.JobRuns[].DPUSeconds / 3600` formatted to 1 decimal. |
| C.3.3 | The Run ID in the display shows a truncated form (e.g., "jr_a1b2c3d4"). | The Run ID column shows the first 8 characters of the run UUID for readability. |
| C.3.4 | The terminal is narrower than the combined column widths (12+12+22+14+44+10 = 114 plus borders). | The rightmost columns are hidden. Horizontal scroll with h/l reveals the Error Message and DPU Hours columns. |

**AWS comparison:**
```
aws glue get-job-runs --job-name nightly-etl-transform --query 'JobRuns[].{Id:Id,JobRunState:JobRunState,StartedOn:StartedOn,ExecutionTime:ExecutionTime,ErrorMessage:ErrorMessage,DPUSeconds:DPUSeconds}'
```
Expected fields visible: Run ID (truncated Id), State, Started, Execution Time (human), Error Message, DPU Hours (computed)

### C.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| C.4.1 | 30 runs are loaded for job "nightly-etl-transform". | The frame top border shows the title centered: `glue-runs(30) -- nightly-etl-transform`. |
| C.4.2 | A filter is active and matches 3 of 30 runs (e.g., filtering by "FAILED"). | The frame title reads `glue-runs(3/30) -- nightly-etl-transform`. |
| C.4.3 | A filter is active and matches 0 runs. | The frame title reads `glue-runs(0/30) -- nightly-etl-transform`. |

### C.5 Row Coloring by Job Run State

| ID | Story | Expected |
|----|-------|----------|
| C.5.1 | A job run has state "SUCCEEDED". | The entire row is rendered in GREEN (#9ece6a). |
| C.5.2 | A job run has state "FAILED". | The entire row is rendered in RED (#f7768e). |
| C.5.3 | A job run has state "ERROR". | The entire row is rendered in RED (#f7768e). |
| C.5.4 | A job run has state "TIMEOUT". | The entire row is rendered in RED (#f7768e). |
| C.5.5 | A job run has state "RUNNING". | The entire row is rendered in YELLOW (#e0af68). |
| C.5.6 | A job run has state "STARTING". | The entire row is rendered in YELLOW (#e0af68). |
| C.5.7 | A job run has state "WAITING". | The entire row is rendered in YELLOW (#e0af68). |
| C.5.8 | A job run has state "STOPPED". | The entire row is rendered DIM (#565f89). |
| C.5.9 | I select a FAILED (red) row. | The selected row has full-width blue background (#7aa2f7), dark foreground (#1a1b26), bold. The red coloring is overridden by the selection highlight. |
| C.5.10 | I move selection away from the FAILED row. | The row reverts to red text (#f7768e). |

### C.6 Currently Running Job

| ID | Story | Expected |
|----|-------|----------|
| C.6.1 | The most recent run has state "RUNNING" with StartedOn a few minutes ago. | The row is in YELLOW (#e0af68). The "Execution Time" column shows elapsed time since start (or a dash/in-progress indicator if execution time is not yet available). The "Error Message" column shows a dash or is empty. |
| C.6.2 | I press ctrl+r to refresh while a job is running. | The running job's execution time updates to reflect the latest value. The state remains RUNNING/YELLOW if still in progress. |
| C.6.3 | After refresh, the previously running job has completed with SUCCEEDED. | The row changes from YELLOW to GREEN. The "Execution Time" shows the final duration. |

### C.7 Failed Run with Error Message

| ID | Story | Expected |
|----|-------|----------|
| C.7.1 | A FAILED run has ErrorMessage "java.lang.OutOfMemoryError: GC overhead limit exceeded". | The "Error Message" column shows the error text, truncated at 44 characters. The full message is visible in the detail view. |
| C.7.2 | A TIMEOUT run has ErrorMessage "Job reached the timeout limit of 120 minutes." | The error message is displayed in the column, truncated if needed. |
| C.7.3 | A SUCCEEDED run has no ErrorMessage. | The "Error Message" column shows a dash ("--") or is empty. |
| C.7.4 | I scroll right with l to see the full Error Message column. | The Error Message column (width 44) becomes visible with more of the message text. |

### C.8 DPU Hours Calculation

| ID | Story | Expected |
|----|-------|----------|
| C.8.1 | A run has DPUSeconds = 45000. | The "DPU Hours" column shows "12.5" (45000 / 3600 = 12.5). |
| C.8.2 | A short run has DPUSeconds = 360. | The "DPU Hours" column shows "0.1". |
| C.8.3 | A run with null or zero DPUSeconds. | The column shows "0.0" or a dash. |

### C.9 Execution Time Human Format

| ID | Story | Expected |
|----|-------|----------|
| C.9.1 | A run has ExecutionTime = 2843 seconds. | The "Execution Time" column shows "47m 23s". |
| C.9.2 | A run has ExecutionTime = 8100 seconds. | The column shows "2h 15m". |
| C.9.3 | A run has ExecutionTime = 45 seconds. | The column shows "45s" or "0m 45s". |
| C.9.4 | A run has ExecutionTime = 7200 seconds (exactly 2 hours). | The column shows "2h 0m" or "120m 0s". |

### C.10 Navigation

| ID | Story | Expected |
|----|-------|----------|
| C.10.1 | I press j/k/g/G/PageUp/PageDown in the runs list. | Navigation behaves identically to other list views. |
| C.10.2 | I press h/l to scroll columns horizontally. | Columns scroll to reveal the DPU Hours column (likely hidden at standard 80-col width since total width = 114). |
| C.10.3 | Column headers scroll in sync with data when I press h/l. | The column header row shifts horizontally by the same offset as data rows. |

### C.11 Sort

| ID | Story | Expected |
|----|-------|----------|
| C.11.1 | I press N on the runs list. | Rows are sorted by Run ID. The sort indicator appears on the column header. |
| C.11.2 | I press S on the runs list. | Rows are sorted by State. Failed/Error runs group together. |
| C.11.3 | I press A on the runs list. | Rows are sorted by Started date. |
| C.11.4 | Default load order. | Runs are displayed newest first, matching the API default. |

### C.12 Filter

| ID | Story | Expected |
|----|-------|----------|
| C.12.1 | I press / and type "FAILED". | Only rows with state FAILED are shown (case-insensitive match). |
| C.12.2 | I type "OutOfMemory" in filter mode. | Only rows whose Error Message contains "OutOfMemory" are shown. This helps identify recurring error patterns. |
| C.12.3 | I press Escape while filter is active. | The filter clears. All runs reappear. |

### C.13 Copy (c)

| ID | Story | Expected |
|----|-------|----------|
| C.13.1 | I select a FAILED run and press c. | The Error Message is copied to the system clipboard (e.g., "java.lang.OutOfMemoryError: GC overhead limit exceeded"). A green "Copied!" flash appears. |
| C.13.2 | I select a TIMEOUT run and press c. | The Error Message is copied (e.g., "Job reached the timeout limit of 120 minutes."). |
| C.13.3 | I select a SUCCEEDED run and press c. | The Run ID is copied to the clipboard (since there is no error message). A green "Copied!" flash appears. |
| C.13.4 | I select a RUNNING run (no error message yet) and press c. | The Run ID is copied to the clipboard. |
| C.13.5 | After ~2 seconds. | The "Copied!" flash auto-clears and the header right reverts to "? for help". |
| C.13.6 | I paste the copied error message into a Jira ticket or Slack channel. | The pasted text matches the full error message exactly (not the truncated display value). |

### C.14 Detail and YAML

| ID | Story | Expected |
|----|-------|----------|
| C.14.1 | I select a run and press d. | The detail view opens. It shows key-value pairs for: Id, JobRunState, StartedOn, CompletedOn, ExecutionTime, ErrorMessage, Attempt, PreviousRunId, TriggerName, JobName, AllocatedCapacity, MaxCapacity, WorkerType, NumberOfWorkers, Timeout, GlueVersion, DPUSeconds, ExecutionClass, LogGroupName. |
| C.14.2 | A SUCCEEDED run has no ErrorMessage. | The ErrorMessage field in detail shows null/empty/dash. |
| C.14.3 | I select a run and press y. | The YAML view opens showing the full job run data as syntax-highlighted YAML. |
| C.14.4 | I press Escape on the detail view. | I return to the runs list. The cursor position is preserved. |

### C.15 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| C.15.1 | I press ctrl+r on the runs list. | The loading spinner appears. A fresh API call is made. When it completes, the table repopulates. |
| C.15.2 | A new run was triggered since the last load. I press ctrl+r. | The new run appears at the top of the list (newest first). The count increments. |

### C.16 Escape (Back to Glue Jobs)

| ID | Story | Expected |
|----|-------|----------|
| C.16.1 | I press Escape on the runs list. | I return to the Glue Jobs list. The cursor is on the same job I had entered. |

### C.17 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| C.17.1 | I press ? on the runs list. | The help screen replaces the table content. It displays a four-column layout with categories: GLUE JOB RUNS, GENERAL, NAVIGATION, HOTKEYS. The GLUE JOB RUNS column includes `<c> Copy Error/ID`. |
| C.17.2 | I press any key on the help screen. | The help screen closes and the runs list reappears. |

---

## D. IAM Groups --> Group Members

### D.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| D.1.1 | I select an IAM group "Admins" and press Enter. | The view transitions to the Group Members list. A spinner appears centered in the frame with text like "Fetching group members..." while the API call is in flight. The IAM Groups list is pushed onto the view stack. |
| D.1.2 | The API responds successfully with 4 members. | The spinner disappears. The table renders with column headers and rows. The frame title updates to `group-members(4) -- Admins`. |
| D.1.3 | The API responds with an error (e.g., NoSuchEntity, access denied). | The spinner disappears. A red error flash message appears in the header right side. |

**AWS comparison:**
```
aws iam get-group --group-name Admins --query 'Users[].{UserName:UserName,UserId:UserId,CreateDate:CreateDate,PasswordLastUsed:PasswordLastUsed}'
```

### D.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| D.2.1 | The IAM group has zero members. | The frame title reads `group-members(0) -- group-name`. The content area shows a centered message (e.g., "No group members found"). |
| D.2.2 | I press ctrl+r on the empty state. | The loading spinner appears while the refresh request is in flight. |

### D.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| D.3.1 | Members load and the table renders. | Four columns are displayed: "User Name" (width 28), "User ID" (width 24), "Created" (width 22), "Password Last Used" (width 22). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| D.3.2 | I verify column data against the AWS CLI. | "User Name" maps to `.Users[].UserName`. "User ID" maps to `.Users[].UserId`. "Created" maps to `.Users[].CreateDate`. "Password Last Used" maps to `.Users[].PasswordLastUsed`. |
| D.3.3 | A username exceeds its 28-character column width. | The name is truncated to fit. No row wrapping occurs. |
| D.3.4 | The terminal is narrower than the combined column widths (28+24+22+22 = 96 plus borders). | The rightmost column(s) are hidden. Horizontal scroll with h/l reveals the "Password Last Used" column. |

**AWS comparison:**
```
aws iam get-group --group-name Admins
```
Expected fields visible: User Name, User ID, Created, Password Last Used

### D.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| D.4.1 | 4 members are loaded for group "Admins". | The frame top border shows the title centered: `group-members(4) -- Admins`. |
| D.4.2 | A filter is active and matches 1 of 4 members. | The frame title reads `group-members(1/4) -- Admins`. |
| D.4.3 | A filter is active and matches 0 members. | The frame title reads `group-members(0/4) -- Admins`. |

### D.5 Row Coloring by Credential Staleness

| ID | Story | Expected |
|----|-------|----------|
| D.5.1 | A user's PasswordLastUsed is within the last 90 days (active credentials). | The entire row is rendered in PLAIN text color (#c0caf5). |
| D.5.2 | A user's PasswordLastUsed is more than 90 days ago (stale credentials). | The entire row is rendered in YELLOW (#e0af68). This immediately flags users with stale credentials during a security audit. |
| D.5.3 | A user's PasswordLastUsed is null (never logged in / service account). | The entire row is rendered DIM (#565f89). The "Password Last Used" column shows null, dash, or "Never". |
| D.5.4 | A user's PasswordLastUsed is more than 180 days ago (severely stale). | The entire row is highlighted in a warning color (YELLOW #e0af68 or potentially RED for severe staleness, per the acceptance criteria of "yellow/red" for >90 days). |
| D.5.5 | I select a stale-credential (yellow) row. | The selected row has full-width blue background (#7aa2f7), dark foreground (#1a1b26), bold. The yellow coloring is overridden. |
| D.5.6 | I move selection away from the stale row. | The row reverts to yellow (#e0af68). |

### D.6 Stale Credential Edge Cases

| ID | Story | Expected |
|----|-------|----------|
| D.6.1 | A user was created yesterday and has never logged in (PasswordLastUsed = null, CreateDate = yesterday). | The row is DIM (#565f89). This could be a new user who has not yet set up their credentials, or a service account. |
| D.6.2 | A user has PasswordLastUsed exactly 90 days ago (today minus 90). | The row color follows the boundary rule (at or beyond 90 days = stale). |
| D.6.3 | A user has PasswordLastUsed exactly 89 days ago. | The row is PLAIN (#c0caf5) -- just within the active window. |
| D.6.4 | A user named "departed-employee" has CreateDate from over a year ago and PasswordLastUsed from 6+ months ago. | The row is clearly highlighted as stale (YELLOW or RED). This is the canonical "who should we investigate?" scenario. |
| D.6.5 | All 4 members have active credentials (< 90 days). | All rows are PLAIN (#c0caf5). No color warnings appear. |

### D.7 Navigation

| ID | Story | Expected |
|----|-------|----------|
| D.7.1 | I press j/k/g/G/PageUp/PageDown in the members list. | Navigation behaves identically to other list views. |
| D.7.2 | I press h/l to scroll columns horizontally. | Columns scroll to reveal the "Password Last Used" column (which may be hidden at narrow terminal widths). |

### D.8 Sort

| ID | Story | Expected |
|----|-------|----------|
| D.8.1 | I press N on the members list. | Rows are sorted by User Name alphabetically. The sort indicator appears. |
| D.8.2 | I press A on the members list. | Rows are sorted by Created date. The sort indicator appears on the "Created" column header. |
| D.8.3 | I press N again after ascending sort. | Sort toggles to descending. |

### D.9 Filter

| ID | Story | Expected |
|----|-------|----------|
| D.9.1 | I press / and type "smith". | Only members whose User Name contains "smith" (case-insensitive) are shown. |
| D.9.2 | I type a User ID prefix in filter mode. | Only members whose User ID matches are shown. |
| D.9.3 | I press Escape while filter is active. | The filter clears. All members reappear. |

### D.10 Copy (c)

| ID | Story | Expected |
|----|-------|----------|
| D.10.1 | I select a member "jsmith" and press c. | The User Name "jsmith" is copied to the system clipboard. A green "Copied!" flash appears in the header right side. |
| D.10.2 | After ~2 seconds. | The "Copied!" flash auto-clears and the header right reverts to "? for help". |
| D.10.3 | I paste from clipboard into another application. | The pasted text matches the username exactly (e.g., "jsmith"). |

### D.11 Detail and YAML

| ID | Story | Expected |
|----|-------|----------|
| D.11.1 | I select a member and press d. | The detail view opens. The detail view shows key-value pairs for: UserName, UserId, Arn, Path, CreateDate, PasswordLastUsed, Tags. |
| D.11.2 | A user with PasswordLastUsed = null. | The detail view shows PasswordLastUsed as null/dash/empty, not as a crash or blank screen. |
| D.11.3 | I select a member and press y. | The YAML view opens showing the full user data as syntax-highlighted YAML. |
| D.11.4 | I press Escape on the detail view. | I return to the members list. The cursor position is preserved. |

### D.12 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| D.12.1 | I press ctrl+r on the members list. | The loading spinner appears. A fresh API call is made. When it completes, the table repopulates. |
| D.12.2 | A new user was added to the group since the last load. I press ctrl+r. | The new user appears. The count in the frame title increments. |
| D.12.3 | A user was removed from the group since the last load. I press ctrl+r. | The user disappears from the list. The count decrements. |

### D.13 Escape (Back to IAM Groups)

| ID | Story | Expected |
|----|-------|----------|
| D.13.1 | I press Escape on the members list. | I return to the IAM Groups list. The cursor is on the same group I had entered. |

### D.14 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| D.14.1 | I press ? on the members list. | The help screen replaces the table content. It displays a four-column layout with categories: GROUP MEMBERS, GENERAL, NAVIGATION, HOTKEYS. The GROUP MEMBERS column includes `<c> Copy User`. |
| D.14.2 | I press any key on the help screen. | The help screen closes and the members list reappears. |

### D.15 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| D.15.1 | I press : on the members list. | Command mode activates in the header. |
| D.15.2 | I type "secrets" and press Enter. | The view navigates to the Secrets Manager list. |
| D.15.3 | I press Escape in command mode. | Command mode is cancelled. The members list remains. |

---

## E. ELB Listeners --> Listener Rules (3-Level Drill-Down)

This section covers the Level 2 child view: pressing Enter on a Listener row to see its routing rules. The Level 1 view (ELB --> Listeners) is covered separately; here we focus on the Rules view and the 3-level navigation.

### E.1 Entry and Loading

| ID | Story | Expected |
|----|-------|----------|
| E.1.1 | I select a listener (port 443, HTTPS) in the ELB Listeners list and press Enter. | The view transitions to the Listener Rules list. A spinner appears centered in the frame with text like "Fetching listener rules..." while the API call is in flight. The Listeners list is pushed onto the view stack. |
| E.1.2 | The API responds successfully with 5 rules. | The spinner disappears. The table renders with column headers and rows. The frame title updates to `listener-rules(5) -- :443 HTTPS`. |
| E.1.3 | I select a listener on port 80 (HTTP) and press Enter. | The frame title updates to `listener-rules(N) -- :80 HTTP`. |
| E.1.4 | The API responds with an error (e.g., ListenerNotFound, access denied). | The spinner disappears. A red error flash message appears in the header right side. |

**AWS comparison:**
```
aws elbv2 describe-rules --listener-arn arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/api-prod-alb/abc123/def456
```

### E.2 Empty State

| ID | Story | Expected |
|----|-------|----------|
| E.2.1 | The listener has only the default rule (which always exists for an ALB listener). | The frame title reads `listener-rules(1) -- :443 HTTPS`. A single "default" rule row is shown. |
| E.2.2 | The listener is deleted or invalid. | A red error flash appears. |

### E.3 Column Layout

| ID | Story | Expected |
|----|-------|----------|
| E.3.1 | Rules load and the table renders. | Four columns are displayed: "Priority" (width 10), "Conditions" (width 36), "Action" (width 16), "Target" (width 32). Column headers are bold, colored blue (#7aa2f7), with no separator line below them. |
| E.3.2 | I verify column data against the AWS CLI. | "Priority" maps to `.Rules[].Priority`. "Conditions" is a human-readable summary of `.Rules[].Conditions[]`. "Action" is extracted from `.Rules[].Actions[0].Type`. "Target" is extracted from the action configuration (target group name for forward, redirect URL for redirect, status code for fixed-response). |
| E.3.3 | A conditions summary exceeds its 36-character column width. | The text is truncated to fit. No row wrapping occurs. |
| E.3.4 | The terminal is narrower than the combined column widths (10+36+16+32 = 94 plus borders). | The rightmost column(s) are hidden. Horizontal scroll with h/l is available. |

**AWS comparison:**
```
aws elbv2 describe-rules --listener-arn arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/api-prod-alb/abc123/def456 --query 'Rules[].{Priority:Priority,Conditions:Conditions,Actions:Actions,IsDefault:IsDefault}'
```
Expected fields visible: Priority, Conditions (computed summary), Action (computed type), Target (computed target)

### E.4 Frame Title

| ID | Story | Expected |
|----|-------|----------|
| E.4.1 | 5 rules are loaded for listener ":443 HTTPS". | The frame top border shows the title centered: `listener-rules(5) -- :443 HTTPS`. |
| E.4.2 | A filter is active and matches 2 of 5 rules. | The frame title reads `listener-rules(2/5) -- :443 HTTPS`. |
| E.4.3 | A filter is active and matches 0 rules. | The frame title reads `listener-rules(0/5) -- :443 HTTPS`. |

### E.5 Default Rule vs. Custom Rules

| ID | Story | Expected |
|----|-------|----------|
| E.5.1 | The rules list includes a default rule (Priority = "default"). | The default rule row is displayed with Priority column showing "default". The row is rendered DIM (#565f89) to visually distinguish it from custom rules. |
| E.5.2 | The default rule has a forward action to "api-prod-tg". | The "Conditions" column shows a dash ("--") since the default rule matches all traffic. The "Action" column shows "forward". The "Target" column shows "api-prod-tg". |
| E.5.3 | Custom rules (Priority = 1, 2, 3, etc.) have specific conditions. | Each custom rule row is displayed in PLAIN text color (#c0caf5). Priority values are numeric. |
| E.5.4 | I select the default (DIM) rule row. | The selected row has full-width blue background (#7aa2f7), overriding the dim styling. |

### E.6 Path-Based Conditions

| ID | Story | Expected |
|----|-------|----------|
| E.6.1 | A rule has a path-pattern condition matching "/api/v1/*". | The "Conditions" column shows `path: /api/v1/*`. |
| E.6.2 | A rule has a path-pattern condition matching "/api/v2/*". | The "Conditions" column shows `path: /api/v2/*`. |
| E.6.3 | A rule has a path-pattern condition matching "/health". | The "Conditions" column shows `path: /health`. |

### E.7 Host-Based Conditions

| ID | Story | Expected |
|----|-------|----------|
| E.7.1 | A rule has a host-header condition matching "admin.example.com". | The "Conditions" column shows `host: admin.example.com`. |
| E.7.2 | A rule has a host-header condition matching "*.example.com". | The "Conditions" column shows `host: *.example.com`. |

### E.8 Multiple Conditions

| ID | Story | Expected |
|----|-------|----------|
| E.8.1 | A rule has both a path-pattern AND a host-header condition. | The "Conditions" column shows a combined summary (e.g., `path: /api/* AND host: api.example.com`), truncated if it exceeds 36 characters. |
| E.8.2 | A rule has a path-pattern AND a custom HTTP header condition. | The "Conditions" column shows a combined summary (e.g., `path: /health AND header: X-Custom=true`). |

### E.9 Action Types

| ID | Story | Expected |
|----|-------|----------|
| E.9.1 | A rule has a "forward" action to a target group. | The "Action" column shows "forward". The "Target" column shows the target group name (e.g., "api-v1-tg"). |
| E.9.2 | A rule has a "redirect" action (HTTP to HTTPS). | The "Action" column shows "redirect". The "Target" column shows the redirect URL pattern (e.g., `https://#{host}:443/#{path}?#{query}`). |
| E.9.3 | A rule has a "fixed-response" action returning 200. | The "Action" column shows "fixed-response". The "Target" column shows the status code and content type (e.g., "200 text/plain"). |
| E.9.4 | A rule has an "authenticate-cognito" or "authenticate-oidc" action. | The "Action" column shows the authentication action type. |

### E.10 Row Coloring

| ID | Story | Expected |
|----|-------|----------|
| E.10.1 | Custom rules are displayed. | All custom rule rows are rendered in PLAIN text color (#c0caf5). Rules have no health/status semantics. |
| E.10.2 | The default rule row. | The default rule is rendered DIM (#565f89) to visually distinguish it from custom rules. |
| E.10.3 | I select a row. | The selected row has full-width blue background regardless of whether it is default or custom. |

### E.11 Navigation

| ID | Story | Expected |
|----|-------|----------|
| E.11.1 | I press j/k/g/G/PageUp/PageDown in the rules list. | Navigation behaves identically to other list views. |
| E.11.2 | I press h/l to scroll columns horizontally. | Columns scroll to reveal the Target column (may be hidden at narrow widths). |
| E.11.3 | Column headers scroll in sync with data when I press h/l. | The column header row shifts by the same offset as data rows. |

### E.12 Sort

| ID | Story | Expected |
|----|-------|----------|
| E.12.1 | I press N on the rules list. | Rows are sorted by Priority. Numeric priorities sort correctly (1, 2, 3..., not "1", "10", "2"). The "default" rule sorts last. |
| E.12.2 | I press N again after ascending sort. | Sort toggles to descending. The "default" rule sorts first (or last, depending on implementation). |

### E.13 Filter

| ID | Story | Expected |
|----|-------|----------|
| E.13.1 | I press / and type "api". | Only rules whose visible fields contain "api" (case-insensitive) are shown. This matches conditions like "path: /api/v1/*" and targets like "api-v1-tg". |
| E.13.2 | I type "forward" in filter mode. | Only rules with forward actions are shown. |
| E.13.3 | I type "default" in filter mode. | Only the default rule is shown. |
| E.13.4 | I press Escape while filter is active. | The filter clears. All rules reappear. |

### E.14 Copy (c)

| ID | Story | Expected |
|----|-------|----------|
| E.14.1 | I select a rule with condition "path: /api/v1/*" and press c. | The condition pattern "path: /api/v1/*" is copied to the system clipboard. A green "Copied!" flash appears. |
| E.14.2 | I select a rule with condition "host: admin.example.com" and press c. | The condition "host: admin.example.com" is copied. |
| E.14.3 | I select a rule with multiple conditions and press c. | The full conditions summary is copied (e.g., "path: /api/* AND host: api.example.com"). |
| E.14.4 | I select the default rule (conditions = "--") and press c. | The dash or empty value is copied, or the default rule's identity information. |
| E.14.5 | After ~2 seconds. | The "Copied!" flash auto-clears and the header right reverts to "? for help". |

### E.15 Detail and YAML

| ID | Story | Expected |
|----|-------|----------|
| E.15.1 | I select a rule and press d. | The detail view opens. The detail view shows key-value pairs for: RuleArn, Priority, Conditions, Actions, IsDefault. |
| E.15.2 | The Conditions field in detail shows the full structured conditions data. | Unlike the truncated summary in the list, the detail view shows the complete condition configuration. |
| E.15.3 | The Actions field in detail shows the full structured actions data. | Including target group ARN, redirect configuration, or fixed-response body. |
| E.15.4 | I select a rule and press y. | The YAML view opens showing the full rule data as syntax-highlighted YAML. |
| E.15.5 | I press Escape on the detail view. | I return to the rules list. The cursor position is preserved. |

### E.16 Refresh (ctrl+r)

| ID | Story | Expected |
|----|-------|----------|
| E.16.1 | I press ctrl+r on the rules list. | The loading spinner appears. A fresh API call is made. When it completes, the table repopulates. |
| E.16.2 | A new rule was added since the last load. I press ctrl+r. | The new rule appears. The count in the frame title increments. |

### E.17 Escape (Back to Listeners)

| ID | Story | Expected |
|----|-------|----------|
| E.17.1 | I press Escape on the rules list. | I return to the ELB Listeners list (NOT the ELB list). The cursor is on the same listener I had entered. |
| E.17.2 | I press Escape a second time (now on the Listeners list). | I return to the ELB list. The cursor is on the same load balancer I had entered. |
| E.17.3 | I press Escape a third time (now on the ELB list). | I return to the main menu. |

### E.18 3-Level Drill-Down Navigation

| ID | Story | Expected |
|----|-------|----------|
| E.18.1 | Main Menu --> ELB --> Listeners --> Rules: I navigate the full path. | I select an ELB, press Enter, see listeners. I select a listener, press Enter, see rules. Each level has its own frame title and data. |
| E.18.2 | Rules --> Listeners --> ELB --> Main Menu: I press Escape three times. | Each Escape pops one level. Rules -> Listeners -> ELB -> Main Menu. No state is lost at any intermediate level. Cursor position is preserved at each level. |
| E.18.3 | I open the help screen (?) at the Rules level. | The help screen shows categories: LISTENER RULES, GENERAL, NAVIGATION, HOTKEYS. It does NOT show the parent Listeners or ELB help. |
| E.18.4 | I open a detail view (d) at the Rules level, then press Escape. | I return to the Rules list, NOT to the Listeners list. |
| E.18.5 | I open a YAML view (y) at the Rules level, then press Escape. | I return to the Rules list. |
| E.18.6 | From the Rules level, I use command mode (: + "ec2" + Enter). | I navigate directly to the EC2 instances list, bypassing the entire ELB view stack. |

### E.19 Help (?)

| ID | Story | Expected |
|----|-------|----------|
| E.19.1 | I press ? on the rules list. | The help screen replaces the table content. It displays a four-column layout with categories: LISTENER RULES, GENERAL, NAVIGATION, HOTKEYS. The LISTENER RULES column includes `<c> Copy Rule`. |
| E.19.2 | I press any key on the help screen. | The help screen closes and the rules list reappears. |

### E.20 Command Mode (:)

| ID | Story | Expected |
|----|-------|----------|
| E.20.1 | I press : on the rules list. | Command mode activates in the header. |
| E.20.2 | I type "rds" and press Enter. | The view navigates to the RDS instances list. |
| E.20.3 | I press Escape in command mode. | Command mode is cancelled. The rules list remains. |

---

## F. Cross-Cutting Concerns for All Five Child Views

### F.1 Header Consistency

| ID | Story | Expected |
|----|-------|----------|
| F.1.1 | In every child view (SNS subscriptions, EB targets, Glue runs, IAM members, Listener rules), the header displays: "a9s" (accent bold), version (dim), profile:region (bold). | Visual inspection confirms the header format is identical across all five child views. |
| F.1.2 | The header right side shows "? for help" in normal mode across all five child views. | Confirmed in all child views. |

### F.2 View Stack Integrity

| ID | Story | Expected |
|----|-------|----------|
| F.2.1 | SNS Topics -> Subscriptions -> Detail -> YAML; then Escape three times. | YAML -> Detail -> Subscriptions -> SNS Topics. Cursor is preserved at each level. |
| F.2.2 | EventBridge Rules -> Targets -> Detail -> YAML; then Escape three times. | YAML -> Detail -> Targets -> EB Rules. Cursor is preserved at each level. |
| F.2.3 | Glue Jobs -> Runs -> Detail -> YAML; then Escape three times. | YAML -> Detail -> Runs -> Glue Jobs. Cursor is preserved at each level. |
| F.2.4 | IAM Groups -> Members -> Detail -> YAML; then Escape three times. | YAML -> Detail -> Members -> IAM Groups. Cursor is preserved at each level. |
| F.2.5 | ELB -> Listeners -> Rules -> Detail -> YAML; then Escape four times. | YAML -> Detail -> Rules -> Listeners -> ELB. Cursor is preserved at each level. |

### F.3 Terminal Resize

| ID | Story | Expected |
|----|-------|----------|
| F.3.1 | I resize the terminal while viewing any child list view. | The layout reflows. Column visibility adjusts to the new width. The frame border redraws correctly. |
| F.3.2 | I resize the terminal to below 60 columns while in a child view. | An error message appears: "Terminal too narrow. Please resize." |
| F.3.3 | I resize the terminal to below 7 lines while in a child view. | An error message appears: "Terminal too short. Please resize." |
| F.3.4 | I resize the terminal while in a detail or YAML view opened from a child view. | The viewport adjusts to the new dimensions. Content reflows appropriately. |

### F.4 Alternating Row Colors

| ID | Story | Expected |
|----|-------|----------|
| F.4.1 | Any child list view has more than 2 rows. | Alternating rows have a subtle background color difference (#1e2030) for readability. The selected row always has blue background regardless. |
| F.4.2 | Alternating row colors combine with status-based coloring (e.g., Glue FAILED rows, SNS PendingConfirmation). | The status-based foreground color (RED, YELLOW, GREEN) applies on top of the alternating background. The selected row always overrides both. |

### F.5 Error Handling

| ID | Story | Expected |
|----|-------|----------|
| F.5.1 | The AWS API returns a throttling error during any child view load. | A red error flash appears in the header right side (e.g., "Error: rate exceeded"). The user can press ctrl+r to retry. |
| F.5.2 | Network connectivity is lost while viewing a child view. | The existing data remains visible. Pressing ctrl+r shows the spinner, then a red error flash when the call fails. |
| F.5.3 | The parent resource is deleted while viewing its child view (e.g., SNS topic deleted while viewing subscriptions). | Pressing ctrl+r shows an error (e.g., "Error: topic not found"). The user can press Escape to return to the parent list. |

### F.6 Profile and Region Switch

| ID | Story | Expected |
|----|-------|----------|
| F.6.1 | I switch AWS profile while in a child view via command mode (:ctx). | The profile selector appears. After selecting a new profile, the application navigates back to the main menu (or refreshes context). Data specific to the new account is loaded. |
| F.6.2 | I switch AWS region while in a child view via command mode (:region). | The region selector appears. After selecting a new region, the context refreshes for the new region. |
