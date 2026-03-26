# QA User Stories: Unified Pagination, Lazy-Load, and Retry

Covers three GitHub issues as a combined QA document:
- **Issue #83:** Pagination and lazy-load with `M` (load more) key
- **Issue #80:** Rate limiting / retry with exponential backoff
- **Issue #19:** Configurable time range for log event fetching

All stories are written from a black-box perspective against the design spec
(`docs/design/design.md`, `docs/design/child-views/`), `views.yaml`, and
`views_reference.yaml`. No source code is referenced.

AWS CLI equivalents are cited so testers can verify data parity.

---

## A. Frame Title -- Count and Truncation Indicator

The frame title (centered in the top border of the frame box) shows the resource
type name and the item count. When only a partial page has been fetched and more
data is available on the server, the count includes a `+` suffix to signal
truncation. When all data has been loaded (no remaining pages), the count is
exact.

### A.1 Exact Count (All Data Loaded)

### Story: Frame title shows exact count when all data fits in one fetch
**Given:** My AWS account has 42 EC2 instances
**When:** I open the EC2 Instances resource list
**Then:** The frame title reads `ec2-instances(42)` with no `+` suffix. All 42 instances appear in the list.

**AWS comparison:**
aws ec2 describe-instances --query 'Reservations[].Instances[] | length(@)'
Expected fields visible: Name, Instance ID, State, Type, Private IP, Public IP, Launch Time (per views.yaml ec2 list)

### Story: Single item shows exact count
**Given:** My AWS account has exactly 1 Lambda function
**When:** I open the Lambda Functions list
**Then:** The frame title reads `lambda(1)`. The single function row is displayed.

**AWS comparison:**
aws lambda list-functions --query 'Functions | length(@)'
Expected fields visible: Function Name, Runtime, Memory, Timeout, Code Size, Last Modified (per views.yaml lambda list)

### Story: Empty result set shows zero count
**Given:** My AWS account has 0 RDS instances in the current region
**When:** I open the RDS Instances list
**Then:** The frame title reads `dbi(0)`. The content area shows an empty-state message (e.g., "No resources found"). No pagination indicator, no `M` key hint.

**AWS comparison:**
aws rds describe-db-instances --query 'DBInstances | length(@)'
Expected fields visible: DB Identifier, Status, Engine, Class, Storage, Multi-AZ, Endpoint (per views.yaml dbi list)

### Story: Exactly page-size items show exact count without truncation
**Given:** My AWS account has exactly 200 Step Function executions for a state machine
**When:** I drill into that state machine's executions
**Then:** The frame title reads `sfn-executions(200) -- my-workflow`. There is no `+` suffix because there are no further pages to fetch.

**AWS comparison:**
aws stepfunctions list-executions --state-machine-arn arn:aws:states:us-east-1:123456789012:stateMachine:my-workflow --query 'executions | length(@)'
Expected fields visible: Name, Status, Start Date, Stop Date, Duration (per views.yaml sfn_executions list)

---

### A.2 Truncated Count (More Data Available)

### Story: Frame title shows truncation indicator when first page is returned
**Given:** A CloudWatch log group has 2,000 log streams
**When:** I drill into that log group
**Then:** The loading spinner appears, then the first page of results loads. The frame title reads `log-streams(200+) -- /aws/lambda/my-function`, indicating that more pages exist on the server.

**AWS comparison:**
aws logs describe-log-streams --log-group-name /aws/lambda/my-function --order-by LastEventTime --descending --query 'logStreams | length(@)'
Expected fields visible: Stream Name, Last Event, First Event, Size (per views.yaml log_streams list)

### Story: 201 items trigger truncation indicator
**Given:** An ECR repository has 201 images
**When:** I drill into the ECR Images child view for that repository
**Then:** The frame title reads `ecr-images(200+) -- payment-api`. The first 200 images are displayed. The `+` indicates at least one more page is available.

**AWS comparison:**
aws ecr describe-images --repository-name payment-api --query 'imageDetails | length(@)'
Expected fields visible: Tag(s), Digest, Pushed At, Size, Scan Status, Findings (per views.yaml ecr_images list)

### Story: Frame title with truncation for CloudFormation events
**Given:** A long-lived CloudFormation stack has 3,000 events
**When:** I press Enter on that stack to view its events
**Then:** The frame title reads `cfn-events(200+) -- payment-service-prod`. The most recent 200 events are shown (reverse chronological order). The `+` signals more historical events are available.

**AWS comparison:**
aws cloudformation describe-stack-events --stack-name payment-service-prod --query 'StackEvents | length(@)'
Expected fields visible: Timestamp, Logical ID, Type, Status, Reason (per views.yaml cfn_events list)

---

### A.3 Filter Count with Truncation

### Story: Filtered view shows matched/total with truncation indicator
**Given:** I am viewing SFN executions with frame title `sfn-executions(200+) -- order-workflow`
**When:** I press `/` and type `FAILED`
**Then:** The frame title changes to `sfn-executions(15/200+) -- order-workflow`. Only the 15 executions whose visible fields match "FAILED" are displayed. The `200+` indicates the total loaded set is truncated.

**AWS comparison:**
aws stepfunctions list-executions --state-machine-arn arn:aws:states:us-east-1:123456789012:stateMachine:order-workflow --status-filter FAILED --query 'executions | length(@)'
Expected fields visible: Name, Status, Start Date, Stop Date, Duration (per views.yaml sfn_executions list)

### Story: Filtered view shows matched/total with exact count after all pages loaded
**Given:** I have loaded all 523 SFN executions (frame title: `sfn-executions(523) -- order-workflow`)
**When:** I press `/` and type `FAILED`
**Then:** The frame title changes to `sfn-executions(15/523) -- order-workflow`. The filter applies to the full loaded set. The denominator is the exact total with no `+`.

**AWS comparison:**
aws stepfunctions list-executions --state-machine-arn arn:aws:states:us-east-1:123456789012:stateMachine:order-workflow --query 'executions | length(@)'

---

## B. Load More (`M` Key)

The `M` key fetches the next page of results from the server and appends them to the current list. It is only active when the list is truncated (the `+` indicator is present in the frame title).

### B.1 Basic Load More

### Story: Pressing M loads the next page and appends items
**Given:** I am viewing `log-streams(200+) -- /aws/lambda/payment-processor` with the first 200 log streams displayed
**When:** I press `M`
**Then:** A "loading more..." indicator appears briefly. The next page of log streams is fetched and appended to the list. The frame title updates: if more pages remain, it shows `log-streams(400+) -- /aws/lambda/payment-processor`; if this was the last page, it shows the final exact count, e.g., `log-streams(347) -- /aws/lambda/payment-processor`.

**AWS comparison:**
aws logs describe-log-streams --log-group-name /aws/lambda/payment-processor --order-by LastEventTime --descending --max-items 200
aws logs describe-log-streams --log-group-name /aws/lambda/payment-processor --order-by LastEventTime --descending --starting-token <NextToken>
Expected fields visible: Stream Name, Last Event, First Event, Size (per views.yaml log_streams list)

### Story: Loading the last page removes the truncation indicator
**Given:** I am viewing `sfn-executions(400+) -- order-workflow` (two pages loaded so far)
**When:** I press `M` and the server returns 123 more executions with no further pagination token
**Then:** The 123 new executions are appended. The frame title changes to `sfn-executions(523) -- order-workflow`. The `+` is gone, indicating all data is loaded. Pressing `M` again has no effect.

**AWS comparison:**
aws stepfunctions list-executions --state-machine-arn arn:aws:states:us-east-1:123456789012:stateMachine:order-workflow --query 'executions | length(@)'
Expected fields visible: Name, Status, Start Date, Stop Date, Duration (per views.yaml sfn_executions list)

### Story: M key is a no-op when list is not truncated
**Given:** I am viewing `ec2-instances(42)` with all 42 instances loaded (no `+` indicator)
**When:** I press `M`
**Then:** Nothing happens. No spinner, no API call, no flash message. The list remains unchanged.

**AWS comparison:**
aws ec2 describe-instances --query 'Reservations[].Instances[] | length(@)'
Expected fields visible: Name, Instance ID, State, Type, Private IP, Public IP, Launch Time (per views.yaml ec2 list)

### Story: M key is a no-op while already loading more
**Given:** I am viewing a truncated list and have already pressed `M` (the "loading more..." indicator is visible)
**When:** I press `M` again before the first load completes
**Then:** The second press is ignored. Only one fetch request is in flight. No duplicate API calls are made.

**AWS comparison:**
N/A -- this is a UI debounce behavior, not an API-level concern.

---

### B.2 Cursor and Sort Preservation

### Story: Cursor position is preserved after load more
**Given:** I am viewing `cfn-events(200+) -- my-stack` with the cursor on row 150 (the 150th event)
**When:** I press `M` and 200 more events are appended
**Then:** After the append, the cursor remains on row 150. The same event is still highlighted. The newly appended rows appear after the existing 200 rows. I can scroll down past row 200 to see the newly loaded events.

**AWS comparison:**
aws cloudformation describe-stack-events --stack-name my-stack --query 'StackEvents | length(@)'
Expected fields visible: Timestamp, Logical ID, Type, Status, Reason (per views.yaml cfn_events list)

### Story: Sort order is reapplied after load more
**Given:** I am viewing `alarm-history(200+) -- cpu-alarm` sorted by Timestamp descending (the `A` key was pressed)
**When:** I press `M` and the next page of alarm history events appends
**Then:** The newly appended items are merged into the sorted order. The full list of (now 400+) events remains sorted by Timestamp descending. The sort indicator arrow on the Timestamp column header is still visible.

**AWS comparison:**
aws cloudwatch describe-alarm-history --alarm-name cpu-alarm --query 'AlarmHistoryItems | length(@)'
Expected fields visible: Timestamp, Type, Summary (per views.yaml alarm_history list)

### Story: Filter is reapplied to appended items
**Given:** I am viewing `ecr-images(200+) -- payment-api` with filter `/` active and text `v2.3` entered, showing `ecr-images(8/200+) -- payment-api`
**When:** I press `M` and 100 more images are fetched (some matching `v2.3`, some not)
**Then:** The filter is applied to all 300 images (original 200 + appended 100). Only images whose visible fields match `v2.3` appear in the filtered view. The frame title updates to reflect the new match count, e.g., `ecr-images(12/300) -- payment-api` if this was the last page, or `ecr-images(12/300+)` if more pages remain.

**AWS comparison:**
aws ecr describe-images --repository-name payment-api --query 'imageDetails | length(@)'
Expected fields visible: Tag(s), Digest, Pushed At, Size, Scan Status, Findings (per views.yaml ecr_images list)

---

### B.3 Load More for Specific Child Views

### Story: Load more for Step Function executions
**Given:** A state machine `order-workflow` has 800 executions
**When:** I drill in and see `sfn-executions(200+) -- order-workflow`, then press `M` three times (waiting for each to complete)
**Then:** After the first `M`: `sfn-executions(400+)`. After the second: `sfn-executions(600+)`. After the third: `sfn-executions(800)`. All 800 executions are now loaded. Row coloring applies: SUCCEEDED=green, FAILED=red, RUNNING=yellow, ABORTED=dim.

**AWS comparison:**
aws stepfunctions list-executions --state-machine-arn arn:aws:states:us-east-1:123456789012:stateMachine:order-workflow --query 'executions | length(@)'
Expected fields visible: Name, Status, Start Date, Stop Date, Duration (per views.yaml sfn_executions list)

### Story: Load more for CloudWatch log streams
**Given:** A log group `/aws/ecs/prod-cluster` has 1,500 log streams
**When:** I drill in and see `log-streams(200+)`, then press `M` repeatedly until all are loaded
**Then:** Each `M` press appends the next page. Eventually the frame title shows `log-streams(1500)` with no `+`. Streams are ordered by last event time, most recent first.

**AWS comparison:**
aws logs describe-log-streams --log-group-name /aws/ecs/prod-cluster --order-by LastEventTime --descending --query 'logStreams | length(@)'
Expected fields visible: Stream Name, Last Event, First Event, Size (per views.yaml log_streams list)

### Story: Load more for ASG scaling activities
**Given:** An Auto Scaling Group `web-asg` has 500 scaling activities
**When:** I drill into the ASG activities child view and see `asg-activities(200+) -- web-asg`, then press `M`
**Then:** The next page loads. Activities are ordered newest first. The frame title updates to reflect the new total loaded.

**AWS comparison:**
aws autoscaling describe-scaling-activities --auto-scaling-group-name web-asg --query 'Activities | length(@)'
Expected fields visible: Start Time, Status, Description, Cause (per views.yaml asg_activities list)

### Story: Load more for CodeBuild builds
**Given:** A CodeBuild project `payment-build` has 400 builds
**When:** I drill in and see `cb-builds(200+) -- payment-build`, then press `M`
**Then:** The next page of builds appends. The frame title updates.

**AWS comparison:**
aws codebuild list-builds-for-project --project-name payment-build --query 'ids | length(@)'
Expected fields visible: Build #, Status, Start Time, Duration, Initiator, Source Version (per views.yaml cb_builds list)

### Story: Load more for Glue job runs
**Given:** A Glue job `etl-daily` has 600 runs
**When:** I drill in and see `glue-runs(200+) -- etl-daily`, then press `M`
**Then:** The next page of runs appends. Runs are ordered newest first.

**AWS comparison:**
aws glue get-job-runs --job-name etl-daily --query 'JobRuns | length(@)'
Expected fields visible: Run ID, State, Started, Duration, Error (per views.yaml glue_runs list)

### Story: Load more for CloudWatch log events
**Given:** A log stream has 50,000 log events
**When:** I open the log events child view and see `log-events(200+) -- 2026/03/22/[$LATEST]8a4b2c1d`, then press `M`
**Then:** Older log events are appended. The view continues to show events ordered by timestamp. Row coloring applies: ERROR/FATAL lines in red, WARN lines in yellow, REPORT lines in green, START/END lines dim.

**AWS comparison:**
aws logs get-log-events --log-group-name /aws/lambda/my-func --log-stream-name "2026/03/22/[\$LATEST]8a4b2c1d" --limit 200
Expected fields visible: Timestamp, Message (per views.yaml log_events list)

---

## C. Help View -- `M` Key Visibility

### Story: Help screen shows M key when list is truncated
**Given:** I am viewing a truncated child view `sfn-executions(200+) -- order-workflow`
**When:** I press `?` to open the help screen
**Then:** The help screen displays in 4-column layout. The RESOURCE or equivalent column includes `<M>    Load More` as an available key binding, alongside `<esc>  Back`, `<enter> View History`, `<d>  Detail`, `<y>  YAML`, `<c>  Copy ARN`. Help key color is green, category headers are orange/yellow, descriptions are plain white.

**AWS comparison:**
N/A -- help screen is a UI-only view.

### Story: Help screen does not show M key when list is fully loaded
**Given:** I am viewing a fully loaded list `ec2-instances(42)` (no truncation)
**When:** I press `?` to open the help screen
**Then:** The help screen does NOT include `<M>  Load More` in the available key bindings. The `M` key is context-sensitive and only appears when relevant.

**AWS comparison:**
N/A -- help screen is a UI-only view.

### Story: Any key closes help
**Given:** The help screen is displayed
**When:** I press any key (including `M`, `Esc`, `?`, or any letter)
**Then:** The help screen closes and the previous view is restored.

**AWS comparison:**
N/A -- help screen is a UI-only view.

---

## D. Top-Level Pagination Correctness

Top-level resource list fetchers (EC2, Lambda, RDS, etc.) must internally paginate through ALL server-side pages so that the user sees the complete set of resources. This is different from child-view lazy-load (section B), where the user controls fetching with `M`. Top-level fetchers silently exhaust all pages before displaying results.

### Story: EC2 fetcher returns all instances across multiple API pages
**Given:** My AWS account has 1,500 EC2 instances (requiring at least 2 API pages at 1,000 per page)
**When:** I open the EC2 Instances list
**Then:** The loading spinner appears while all pages are fetched. When loading completes, the frame title reads `ec2-instances(1500)`. All 1,500 instances are available for navigation, sorting, and filtering.

**AWS comparison:**
aws ec2 describe-instances --query 'Reservations[].Instances[] | length(@)'
Expected fields visible: Name, Instance ID, State, Type, Private IP, Public IP, Launch Time (per views.yaml ec2 list)

### Story: Lambda fetcher returns all functions across multiple API pages
**Given:** My account has 200 Lambda functions (requiring 4 API pages at 50 per page)
**When:** I open the Lambda Functions list
**Then:** The spinner appears. When loading completes, the frame title reads `lambda(200)`. All 200 functions are listed. None are silently dropped due to incomplete pagination.

**AWS comparison:**
aws lambda list-functions --query 'Functions | length(@)'
Expected fields visible: Function Name, Runtime, Memory, Timeout, Code Size, Last Modified (per views.yaml lambda list)

### Story: RDS fetcher returns all instances across multiple API pages
**Given:** My account has 250 RDS instances (requiring 3 API pages at 100 per page)
**When:** I open the RDS Instances list
**Then:** The frame title reads `dbi(250)`. All 250 are present.

**AWS comparison:**
aws rds describe-db-instances --query 'DBInstances | length(@)'
Expected fields visible: DB Identifier, Status, Engine, Class, Storage, Multi-AZ, Endpoint (per views.yaml dbi list)

### Story: IAM Roles fetcher handles thousands of roles
**Given:** My account has 3,000 IAM roles (requiring 30 API pages at 100 per page)
**When:** I open the IAM Roles list
**Then:** The spinner animates while all 30 pages are fetched. The frame title reads `role(3000)`. All 3,000 roles are available.

**AWS comparison:**
aws iam list-roles --query 'Roles | length(@)'
Expected fields visible: Role Name, Created, Last Used, Path, Description (per views.yaml role list)

### Story: CloudWatch Logs fetcher handles many log groups
**Given:** My account has 500 log groups (requiring 10 API pages at 50 per page)
**When:** I open the CloudWatch Logs list
**Then:** The frame title reads `logs(500)`. All 500 log groups are listed.

**AWS comparison:**
aws logs describe-log-groups --query 'logGroups | length(@)'
Expected fields visible: Log Group Name, Stored Bytes, Retention, Metric Filters, Created (per views.yaml logs list)

### Story: Security Groups fetcher returns all groups
**Given:** My account has 1,200 security groups
**When:** I open the Security Groups list
**Then:** The frame title reads `sg(1200)`. All 1,200 groups are present.

**AWS comparison:**
aws ec2 describe-security-groups --query 'SecurityGroups | length(@)'
Expected fields visible: Group Name, Group ID, VPC ID, Description, Inbound Rules, Outbound Rules (per views.yaml sg list)

### Story: S3 buckets load in a single call (no pagination needed)
**Given:** My account has 85 S3 buckets
**When:** I open the S3 Buckets list
**Then:** The frame title reads `s3(85)`. All 85 buckets are listed. The `ListBuckets` API returns all buckets in one response.

**AWS comparison:**
aws s3api list-buckets --query 'Buckets | length(@)'
Expected fields visible: Bucket Name, Region, Created (per views.yaml s3 list)

---

## E. Retry with Exponential Backoff

When an AWS API call returns a throttling error (e.g., `ThrottlingException`, `TooManyRequestsException`, `RequestLimitExceeded`), the application retries with exponential backoff rather than immediately failing.

### E.1 Retryable Errors

### Story: Throttling error triggers retry and eventually succeeds
**Given:** I open the EC2 Instances list, and the first API call returns a `ThrottlingException`
**When:** The application retries (up to 3 attempts with exponential backoff starting at 500ms)
**Then:** On the second attempt the API succeeds. The spinner continues animating during the backoff wait. The EC2 instances load normally. The user sees no error -- the retry is transparent.

**AWS comparison:**
aws ec2 describe-instances
(ThrottlingException is rate-limit dependent and not easily reproduced with CLI)
Expected fields visible: Name, Instance ID, State, Type, Private IP, Public IP, Launch Time (per views.yaml ec2 list)

### Story: All retries exhausted shows error
**Given:** I open the Lambda Functions list, and every API attempt (3 retries) returns `ThrottlingException`
**When:** All 3 retry attempts are exhausted
**Then:** The spinner stops. An error flash message appears in the header right side in red (e.g., `Error: rate limit exceeded`). The error flash auto-clears after approximately 2 seconds. The frame content shows the loading-failed state.

**AWS comparison:**
aws lambda list-functions
(When rate-limited, CLI shows: An error occurred (ThrottlingException))
Expected fields visible: Function Name, Runtime, Memory, Timeout, Code Size, Last Modified (per views.yaml lambda list)

### Story: Throttling during load-more retries transparently
**Given:** I am viewing `sfn-executions(200+) -- order-workflow` and press `M` to load more
**When:** The load-more API call returns a `ThrottlingException` on the first attempt
**Then:** The application retries with backoff. If a subsequent attempt succeeds, the new executions are appended normally. The cursor position is preserved. The user may notice a slightly longer load time but sees no error.

**AWS comparison:**
aws stepfunctions list-executions --state-machine-arn arn:aws:states:us-east-1:123456789012:stateMachine:order-workflow --starting-token <NextToken>

---

### E.2 Non-Retryable Errors

### Story: AccessDenied error is not retried
**Given:** I open the Secrets Manager list, but my IAM credentials lack `secretsmanager:ListSecrets` permission
**When:** The API returns `AccessDeniedException`
**Then:** The error is NOT retried (retrying would not help -- it is a permissions issue). An error flash appears immediately in the header: `Error: AccessDenied`. No backoff delay occurs.

**AWS comparison:**
aws secretsmanager list-secrets
(Shows: An error occurred (AccessDeniedException) when calling the ListSecrets operation)
Expected fields visible: Secret Name, Description, Last Accessed, Last Changed, Rotation (per views.yaml secrets list)

### Story: ResourceNotFoundException is not retried
**Given:** I drill into a CloudFormation stack that was deleted between page loads
**When:** The API returns `StackNotFoundException`
**Then:** The error is shown immediately without retry. The error flash appears in the header.

**AWS comparison:**
aws cloudformation describe-stack-events --stack-name deleted-stack
(Shows: An error occurred (ValidationError) when calling the DescribeStackEvents operation: Stack [deleted-stack] does not exist)
Expected fields visible: Timestamp, Logical ID, Type, Status, Reason (per views.yaml cfn_events list)

---

### E.3 Context Cancellation

### Story: Navigating away during backoff cancels the retry
**Given:** An API call for EC2 instances has failed with ThrottlingException and the application is in a backoff wait before retry
**When:** I press `Esc` to go back to the main menu
**Then:** The pending retry is cancelled. The backoff timer stops. I return to the main menu immediately without waiting for the retry to fire. No orphaned API calls are made.

**AWS comparison:**
N/A -- this is a cancellation behavior, not an API-level concern.

### Story: Switching resource type during backoff cancels the retry
**Given:** The application is retrying a throttled Lambda list-functions call
**When:** I press `:` and type `ec2` + Enter to switch to EC2 Instances
**Then:** The Lambda retry is cancelled. The EC2 list begins loading. No error from the cancelled Lambda fetch appears.

**AWS comparison:**
N/A -- this is a cancellation behavior.

---

### E.4 Network Errors During Load More

### Story: Network error during load more preserves existing data
**Given:** I am viewing `log-streams(200+) -- /aws/lambda/my-func` with 200 streams loaded
**When:** I press `M` and the network connection drops, causing the API call to fail
**Then:** The existing 200 log streams remain visible and intact. An error flash appears in the header (e.g., `Error: request failed`). The frame title remains `log-streams(200+)` (the truncation indicator persists since the load more did not succeed). I can press `M` again to retry when connectivity is restored.

**AWS comparison:**
N/A -- network failure scenario.
Expected fields visible: Stream Name, Last Event, First Event, Size (per views.yaml log_streams list)

---

## F. Refresh Behavior

### Story: Ctrl+R resets pagination and re-fetches from the first page
**Given:** I am viewing `sfn-executions(523) -- order-workflow` (all pages loaded after multiple `M` presses)
**When:** I press `Ctrl+R`
**Then:** The spinner appears. A full re-fetch starts from the first page. The previous 523 items are replaced. When the first page loads, the frame title shows `sfn-executions(200+) -- order-workflow` (assuming more than 200 executions still exist). Pagination state is reset -- I would need to press `M` again to load beyond the first page.

**AWS comparison:**
aws stepfunctions list-executions --state-machine-arn arn:aws:states:us-east-1:123456789012:stateMachine:order-workflow --query 'executions | length(@)'
Expected fields visible: Name, Status, Start Date, Stop Date, Duration (per views.yaml sfn_executions list)

### Story: Ctrl+R on a top-level list re-fetches all pages
**Given:** I am viewing `ec2-instances(1500)` (all instances loaded via top-level pagination)
**When:** I press `Ctrl+R`
**Then:** The spinner appears. All API pages are re-fetched. When complete, the frame title shows the updated count. The list contents reflect any changes that occurred since the previous fetch (new instances, terminated instances, state changes).

**AWS comparison:**
aws ec2 describe-instances --query 'Reservations[].Instances[] | length(@)'
Expected fields visible: Name, Instance ID, State, Type, Private IP, Public IP, Launch Time (per views.yaml ec2 list)

### Story: Ctrl+R during an empty list re-fetches
**Given:** I am viewing `dbi(0)` (no RDS instances in this region)
**When:** I press `Ctrl+R`
**Then:** The spinner appears, a fresh API call is made. If an RDS instance was just created, it now appears. The frame title updates accordingly.

**AWS comparison:**
aws rds describe-db-instances --query 'DBInstances | length(@)'
Expected fields visible: DB Identifier, Status, Engine, Class, Storage, Multi-AZ, Endpoint (per views.yaml dbi list)

---

## G. Navigation Across Views with Pagination State

### Story: Detail view and back preserves loaded data
**Given:** I pressed `M` twice on `cfn-events(200+) -- my-stack` and now see `cfn-events(600+) -- my-stack` with 600 events loaded
**When:** I select an event, press `d` to open the detail view, then press `Esc` to go back
**Then:** I return to the CFN events list with all 600 events still present. The frame title still shows `cfn-events(600+)`. The cursor returns to the event I had selected. No data is lost or re-fetched.

**AWS comparison:**
aws cloudformation describe-stack-events --stack-name my-stack --query 'StackEvents | length(@)'
Expected fields visible: Timestamp, Logical ID, Type, Status, Reason (per views.yaml cfn_events list)

### Story: YAML view and back preserves loaded data
**Given:** I loaded 400 SFN executions (`sfn-executions(400+) -- order-workflow`)
**When:** I select an execution, press `y` to view its YAML, then press `Esc`
**Then:** I return to the executions list with all 400 still present. Pagination state is intact.

**AWS comparison:**
aws stepfunctions list-executions --state-machine-arn arn:aws:states:us-east-1:123456789012:stateMachine:order-workflow --output yaml
Expected fields visible: YAML view shows full execution resource in syntax-colored YAML (keys blue, strings green, numbers orange, booleans purple, null dim)

### Story: Switching resource type resets pagination state
**Given:** I loaded 600 CFN events via `M` presses
**When:** I press `Esc` to go back to the CFN stacks list, then drill into a different stack
**Then:** The new stack's events view starts fresh with its own first page. The previous 600 events from the other stack are no longer held. The frame title shows the new stack's count (possibly with `+` if truncated).

**AWS comparison:**
aws cloudformation describe-stack-events --stack-name other-stack --query 'StackEvents | length(@)'
Expected fields visible: Timestamp, Logical ID, Type, Status, Reason (per views.yaml cfn_events list)

### Story: Navigating to parent and back resets child pagination
**Given:** I am viewing `log-streams(400+) -- /aws/lambda/my-func` after pressing `M` once
**When:** I press `Esc` to return to the Log Groups list, then drill back into the same log group
**Then:** The log streams view starts from the first page again. The frame title initially shows the truncated count from a fresh fetch, not the 400+ from the previous session.

**AWS comparison:**
aws logs describe-log-streams --log-group-name /aws/lambda/my-func --order-by LastEventTime --descending --query 'logStreams | length(@)'
Expected fields visible: Stream Name, Last Event, First Event, Size (per views.yaml log_streams list)

---

## H. Demo Mode

### Story: Demo mode loads all data at once with no pagination
**Given:** I launch a9s with `--demo` flag (synthetic demo data, no AWS credentials needed)
**When:** I navigate to any resource type and then any child view
**Then:** All demo data loads at once. Frame titles show exact counts with no `+` suffix. The `M` key has no effect (there is no server to fetch more from). All navigation, sorting, and filtering work normally on the complete demo data set.

**AWS comparison:**
N/A -- demo mode uses synthetic data, not real AWS APIs.

---

## I. Edge Cases

### Story: API returns zero items on load more (despite pagination token)
**Given:** I am viewing a truncated list and press `M`
**When:** The API returns a response with 0 new items but no further pagination token
**Then:** The truncation indicator is removed from the frame title (now shows exact count). The list is unchanged in content. No error is shown. The `M` key becomes a no-op.

**AWS comparison:**
N/A -- edge case in API behavior.

### Story: Rapid M key presses are debounced
**Given:** I am viewing a truncated list
**When:** I press `M` five times in rapid succession
**Then:** Only one load-more request is made at a time. Subsequent presses during an in-flight fetch are ignored. After the first load completes, I can press `M` again for the next page.

**AWS comparison:**
N/A -- UI debounce behavior.

### Story: Rapid Ctrl+R does not cause concurrent fetches
**Given:** I am viewing a resource list
**When:** I press `Ctrl+R` three times in rapid succession
**Then:** The application does not send three concurrent fetch requests. It either debounces (only the last refresh fires) or cancels the in-flight request before starting a new one.

**AWS comparison:**
N/A -- UI debounce behavior.

### Story: Load more after sort preserves sort order
**Given:** I am viewing `ecr-images(200+) -- payment-api` sorted by Pushed At descending
**When:** I press `M` to load more images
**Then:** The newly loaded images are inserted into the correct positions according to the current sort order. The sort indicator on the Pushed At column remains visible. The overall list stays sorted.

**AWS comparison:**
aws ecr describe-images --repository-name payment-api --query 'imageDetails | sort_by(@, &imagePushedAt) | reverse(@) | length(@)'
Expected fields visible: Tag(s), Digest, Pushed At, Size, Scan Status, Findings (per views.yaml ecr_images list)

### Story: Load more while scrolled to bottom
**Given:** I am at the last row (row 200) of a truncated list `asg-activities(200+) -- web-asg`
**When:** I press `M` and 100 more activities are appended
**Then:** My cursor remains on row 200. The new rows 201-300 are accessible by pressing `j` or `G`. The scroll position does not jump to the top.

**AWS comparison:**
aws autoscaling describe-scaling-activities --auto-scaling-group-name web-asg --query 'Activities | length(@)'
Expected fields visible: Start Time, Status, Description, Cause (per views.yaml asg_activities list)

---

## J. Cross-Cutting: Terminal Resize During Pagination

### Story: Terminal resize during load more does not interrupt the fetch
**Given:** I pressed `M` on a truncated list and the fetch is in progress
**When:** I resize the terminal window
**Then:** The layout reflows to the new dimensions. The in-progress fetch is NOT interrupted. When the fetch completes, the new items are appended and rendered in the new layout.

**AWS comparison:**
N/A -- terminal behavior.

### Story: Minimum terminal size error does not lose data
**Given:** I have 400 items loaded in a list (after pressing `M` once)
**When:** I resize the terminal below the minimum width (< 60 columns) or minimum height (< 7 lines)
**Then:** The application shows the "Terminal too narrow" or "Terminal too short" error message. When I resize back to a valid size, the 400 items are still present. No data is lost due to the resize.

**AWS comparison:**
N/A -- terminal behavior.

---

## K. Configurable Time Range for Log Events (Issue #19)

### Story: Log events fetch respects default time range
**Given:** I drill into a log stream that has events spanning the past 30 days
**When:** The log events child view loads
**Then:** Only recent log events are fetched (e.g., the most recent N events or events from the last configurable time window). The frame title shows the count of loaded events with a `+` if more exist. Older events are available via the `M` key.

**AWS comparison:**
aws logs get-log-events --log-group-name /aws/lambda/my-func --log-stream-name "2026/03/22/[\$LATEST]8a4b2c1d" --limit 200
Expected fields visible: Timestamp, Message (per views.yaml log_events list)

### Story: Load more on log events fetches older entries
**Given:** I am viewing `log-events(200+) -- 2026/03/22/[$LATEST]8a4b2c1d` showing the most recent 200 events
**When:** I press `M`
**Then:** Older log events (preceding the oldest currently displayed event) are fetched and appended to the bottom of the list. The frame title updates to reflect the new count. Events remain ordered by timestamp.

**AWS comparison:**
aws logs get-log-events --log-group-name /aws/lambda/my-func --log-stream-name "2026/03/22/[\$LATEST]8a4b2c1d" --next-token <token>
Expected fields visible: Timestamp, Message (per views.yaml log_events list)

---

## L. Error Display Consistency

### Story: Error flash from failed fetch follows design spec
**Given:** Any API call fails (throttling exhausted, permissions error, network error)
**When:** The error is shown in the header
**Then:** The error flash appears on the right side of the header in red (`#f7768e`) bold text. It follows the format `Error: <message>`. It auto-clears after approximately 2 seconds. The header left side (app name, version, profile:region) is unchanged. This matches the design spec section 3.1 "Flash error" variant.

**AWS comparison:**
N/A -- UI behavior.

### Story: Error flash during load more does not displace the header hint
**Given:** I am viewing a list and `? for help` is shown in the header right
**When:** A load-more fetch fails
**Then:** The `? for help` hint is temporarily replaced by the red error flash. After the flash auto-clears (~2 seconds), the `? for help` hint returns.

**AWS comparison:**
N/A -- UI behavior.

---

## M. Fetcher Pagination Audit -- Child Views

Each child view's fetcher must handle server-side pagination tokens correctly.
This section verifies that each child view type's fetcher follows pagination
tokens until the page boundary (initial load) or until all pages are exhausted
(after repeated `M` presses).

| ID | Child View | AWS API | Pagination Mechanism | Page Size | Story |
|----|-----------|---------|---------------------|-----------|-------|
| M.1 | S3 Objects (`s3_objects`) | `ListObjectsV2` | `ContinuationToken` | 1,000 | Bucket with 5,000 objects: first load shows 200+, pressing `M` fetches next page. |
| M.2 | Log Streams (`log_streams`) | `DescribeLogStreams` | `nextToken` | 50 | Log group with 2,000 streams: first load shows 200+, `M` fetches more. |
| M.3 | Log Events (`log_events`) | `GetLogEvents` | `nextForwardToken` | ~10,000 events or 1MB | Stream with 50,000 events: first load shows 200+, `M` fetches older events. |
| M.4 | SFN Executions (`sfn_executions`) | `ListExecutions` | `nextToken` | 100 | State machine with 800 executions: first load shows 200+, `M` fetches more. |
| M.5 | SFN Execution History (`sfn_execution_history`) | `GetExecutionHistory` | `nextToken` | varies | Execution with 500 history events: first load shows 200+, `M` fetches more. |
| M.6 | CFN Events (`cfn_events`) | `DescribeStackEvents` | `NextToken` | varies | Stack with 3,000 events: first load shows 200+, `M` fetches older events. |
| M.7 | CFN Resources (`cfn_resources`) | `ListStackResources` | `NextToken` | varies | Stack with 500 resources: first load shows 200+, `M` fetches more. |
| M.8 | ASG Activities (`asg_activities`) | `DescribeScalingActivities` | `NextToken` | 100 | ASG with 500 activities: first load shows 200+, `M` fetches more. |
| M.9 | Alarm History (`alarm_history`) | `DescribeAlarmHistory` | `NextToken` | varies | Alarm with 300 history items: first load shows 200+, `M` fetches more. |
| M.10 | ECR Images (`ecr_images`) | `DescribeImages` | `nextToken` | varies | Repository with 400 images: first load shows 200+, `M` fetches more. |
| M.11 | CodeBuild Builds (`cb_builds`) | `ListBuildsForProject` | `nextToken` | varies | Project with 400 builds: first load shows 200+, `M` fetches more. |
| M.12 | Glue Job Runs (`glue_runs`) | `GetJobRuns` | `NextToken` | 200 | Job with 600 runs: first load shows 200+, `M` fetches more. |
| M.13 | ECS Tasks (`ecs_tasks`) | `ListTasks` + `DescribeTasks` | `nextToken` | 100 | Service with 500 tasks: first load shows 200+, `M` fetches more. |
| M.14 | R53 Records (`r53_records`) | `ListResourceRecordSets` | `NextRecordName`/`NextRecordType` | 300 | Zone with 10,000 records: first load shows 200+, `M` fetches more. |
| M.15 | SNS Subscriptions (`sns_subscriptions`) | `ListSubscriptionsByTopic` | `NextToken` | 100 | Topic with 500 subscriptions: first load shows 200+, `M` fetches more. |
| M.16 | Role Policies (`role_policies`) | `ListAttachedRolePolicies` + `ListRolePolicies` | `Marker`/`IsTruncated` | 100 | Role with 300 policies: first load shows 200+, `M` fetches more. |
| M.17 | IAM Group Members (`iam_group_members`) | `GetGroup` | `Marker`/`IsTruncated` | varies | Group with 500 members: first load shows 200+, `M` fetches more. |
| M.18 | DBI Events (`dbi_events`) | `DescribeEvents` | `Marker` | varies | Instance with 400 events: first load shows 200+, `M` fetches more. |
| M.19 | ELB Listeners (`elb_listeners`) | `DescribeListeners` | `Marker` | varies | LB with minimal listeners: typically all load in one page (no `M` needed). |
| M.20 | Pipeline Stages (`pipeline_stages`) | `GetPipelineState` | Not paginated | N/A | All stages load in one call. No `M` key needed. Frame title shows exact count. |
| M.21 | TG Health (`tg_health`) | `DescribeTargetHealth` | Not paginated | N/A | All targets load in one call. No `M` key needed. |
| M.22 | ECS Service Events (`ecs_svc_events`) | Service events from DescribeServices | Not paginated (last 100) | N/A | Events are embedded in the DescribeServices response. No `M` key needed. |

**AWS comparison (general pattern):**
```
aws <service> <list-operation> --<parent-filter> <parent-id> --query '<items> | length(@)'
```

---

## N. Interaction Matrix

This section verifies that pagination behaviors compose correctly with other features.

### Story: Copy ID works on items loaded via M
**Given:** I loaded 400 SFN executions (original 200 + 200 from `M`) and navigate to execution #350
**When:** I press `c`
**Then:** The execution ARN of row 350 is copied to the clipboard. "Copied!" flash appears in the header in green.

**AWS comparison:**
aws stepfunctions list-executions --state-machine-arn <arn> --query 'executions[349].executionArn'

### Story: Detail view works on items loaded via M
**Given:** I loaded 600 CFN events and select event #500
**When:** I press `d`
**Then:** The detail view opens for event #500, showing all detail fields: Timestamp, LogicalResourceId, PhysicalResourceId, ResourceType, ResourceStatus, ResourceStatusReason, ClientRequestToken, EventId (per views.yaml cfn_events detail).

**AWS comparison:**
aws cloudformation describe-stack-events --stack-name my-stack --query 'StackEvents[499]'

### Story: YAML view works on items loaded via M
**Given:** I loaded 400 ECR images and select image #350
**When:** I press `y`
**Then:** The YAML view opens showing the full resource as syntax-colored YAML. Keys are blue, strings green, numbers orange, booleans purple, null values dim. Tree connectors are visible for nested structures.

**AWS comparison:**
aws ecr describe-images --repository-name payment-api --output yaml

### Story: Sort toggle works after load more
**Given:** I loaded 400 log streams via `M`, currently sorted by Last Event descending
**When:** I press `N` to sort by Stream Name, then `N` again to toggle to descending
**Then:** The sort indicator moves to the Stream Name column header. First press: `NAME` with up-arrow (ascending). Second press: `NAME` with down-arrow (descending). All 400 streams are re-sorted both times. The cursor remains on a valid row.

**AWS comparison:**
aws logs describe-log-streams --log-group-name /aws/lambda/my-func --query 'logStreams | sort_by(@, &logStreamName) | [*].logStreamName'
Expected fields visible: Stream Name, Last Event, First Event, Size (per views.yaml log_streams list)

### Story: Horizontal scroll works after load more
**Given:** I loaded 400 CFN events via `M`. The terminal is 80 columns wide, so only Timestamp, Logical ID, and part of Type columns are visible.
**When:** I press `l` to scroll right
**Then:** The Status and Reason columns come into view. Column headers scroll in sync with data rows. All 400 loaded events scroll horizontally in unison.

**AWS comparison:**
aws cloudformation describe-stack-events --stack-name my-stack --output table
Expected fields visible: Timestamp, Logical ID, Type, Status, Reason (per views.yaml cfn_events list)

### Story: Page up/down works across load-more boundary
**Given:** I loaded 400 log streams (200 original + 200 from `M`). My terminal shows 20 rows at a time.
**When:** I press `PgDn` (or `Ctrl+D`) repeatedly to page through the list
**Then:** Paging is seamless across the original 200 and the appended 200. There is no visual break, stutter, or special handling at row 200. Row 201 follows row 200 naturally.

**AWS comparison:**
N/A -- UI navigation behavior.

---

## O. Key Binding Summary for Pagination Feature

All key bindings introduced or affected by this feature, mapped to the views where they apply:

| Key | View(s) | Action | Condition |
|-----|---------|--------|-----------|
| `M` | Resource List (child views) | Load next page of results | Only active when frame title shows `+` (truncated) |
| `Ctrl+R` | All list views | Full re-fetch from page 1, resets pagination state | Always active |
| `j`/`Down` | All list views | Move cursor down through all loaded items (including load-more items) | Always active |
| `k`/`Up` | All list views | Move cursor up | Always active |
| `g` | All list views | Jump to first item | Always active |
| `G` | All list views | Jump to last loaded item | Always active |
| `/` | All list views | Filter across all loaded items (both initial and load-more) | Always active |
| `N`/`S`/`A` | All list views | Sort all loaded items | Always active |
| `c` | All list views | Copy resource ID of any loaded item | Always active |
| `d` | All list views | Detail view for any loaded item | Always active |
| `y` | All list views | YAML view for any loaded item | Always active |
| `h`/`l` | All list views | Horizontal scroll (all loaded items scroll in sync) | Always active |
| `PgUp`/`Ctrl+U` | All list views | Page up through all loaded items | Always active |
| `PgDn`/`Ctrl+D` | All list views | Page down through all loaded items | Always active |
| `?` | All views | Help screen (shows `M` key when applicable) | Always active |
| `Esc` | All views | Back (cancels in-flight retry/load-more) | Always active |
