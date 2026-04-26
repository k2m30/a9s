# QA-14: Log View Configuration, Multi-Resource Logs, and Log Source Caching

Covers three related features:
- **Issue #19:** Configurable time range and max events for log views
- **Issue #20:** Extend `l` (logs) key to Lambda, ECS, EKS, and RDS
- **Issue #21:** Cache discovered log sources per session

All stories are written from a black-box perspective against the design spec
(`docs/design/ec2-logs-view.md`, `docs/design/design.md`) and `views.yaml`.

AWS CLI equivalents are cited so testers can verify data parity.

---

## A. Configurable Time Range (Issue #19)

### A.1 Default Time Range (No Config)

| # | Story | Expected |
|---|-------|----------|
| A.1.1 | I delete or never create `~/.a9s/config.yaml`. I press `l` on an EC2 instance, open CloudTrail Events. | Events from the last 24 hours are displayed. The time of the oldest event is no older than 24 hours ago. AWS comparison: `aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceType,AttributeValue=AWS::EC2::Instance --start-time $(date -u -v-24H +%Y-%m-%dT%H:%M:%SZ)` |
| A.1.2 | Same as A.1.1 but for CloudWatch Log Viewer. | Log events span the last 24 hours. AWS comparison: `aws logs filter-log-events --log-group-name GROUP --start-time $(date -u -v-24H +%s)000` |
| A.1.3 | Same as A.1.1 but for VPC Flow Logs. | Flow log records span the last 24 hours. |
| A.1.4 | Same as A.1.1 but for SSM Command History. | SSM commands shown span the last 24 hours. AWS comparison: `aws ssm list-commands --filters Key=InvokedAfter,Value=$(date -u -v-24H +%Y-%m-%dT%H:%M:%SZ)` |
| A.1.5 | Console Output has no time range concept (AWS caps at ~64KB). | Console Output displays all available output regardless of any time_range config. The config has no effect on this source. |

### A.2 Custom Time Range Values

| # | Story | Expected |
|---|-------|----------|
| A.2.1 | I set `logs.time_range: 1h` in `~/.a9s/config.yaml`. I open CloudTrail Events for an EC2 instance. | Only events from the last 1 hour are shown. The frame title count reflects the reduced set. AWS comparison: `aws cloudtrail lookup-events --start-time $(date -u -v-1H +%Y-%m-%dT%H:%M:%SZ)` |
| A.2.2 | I set `logs.time_range: 6h`. I open CloudTrail Events. | Events span the last 6 hours. |
| A.2.3 | I set `logs.time_range: 12h`. I open CloudWatch Log Viewer. | Log events span the last 12 hours. |
| A.2.4 | I set `logs.time_range: 7d`. I open CloudTrail Events. | Events span the last 7 days. AWS comparison: `aws cloudtrail lookup-events --start-time $(date -u -v-7d +%Y-%m-%dT%H:%M:%SZ)` |
| A.2.5 | I set `logs.time_range: 7d` and many events exist. I open CloudTrail. | A large number of events are fetched (up to max_events). The oldest visible event may be up to 7 days old. |
| A.2.6 | I set `logs.time_range: 1h` but no events occurred in the last hour. | The sub-view shows the empty state message (e.g., "No CloudTrail events in the last 1 hour"). The frame title shows count 0. |

### A.3 Time Range Validation

| # | Story | Expected |
|---|-------|----------|
| A.3.1 | I set `logs.time_range: 30m` (not in the allowed set). | The application either rejects the config on startup with a clear error message, or falls back to the default 24h with a warning. The user is informed which values are valid (1h, 6h, 12h, 24h, 7d). |
| A.3.2 | I set `logs.time_range: 0h`. | The application rejects this value. Zero is not a sensible time range. |
| A.3.3 | I set `logs.time_range: -1h` (negative). | The application rejects this value. Negative time ranges are nonsensical. |
| A.3.4 | I set `logs.time_range: abc` (non-numeric). | The application rejects this value with a clear error message indicating the expected format. |
| A.3.5 | I set `logs.time_range: 30d` (exceeds CloudTrail 90-day limit but is otherwise a valid duration). | The application either accepts this (CloudTrail API will naturally return only what is available) or rejects it as exceeding a reasonable maximum. Either way, no crash occurs. |
| A.3.6 | I set `logs.time_range: ""` (empty string). | The application falls back to the default 24h time range. No crash. |
| A.3.7 | The `logs` key exists in config but `time_range` is absent. | The default 24h is used. Only `max_events` (if present) takes effect. |

### A.4 Time Range Applied Consistently Across Log Sub-Views

| # | Story | Expected |
|---|-------|----------|
| A.4.1 | I set `logs.time_range: 6h`. I open CloudTrail, note the time range. I go back, open Flow Logs. | Both sub-views use the same 6h time range. The oldest event in each is no older than 6 hours. |
| A.4.2 | I set `logs.time_range: 1h`. I open CloudWatch Log Viewer for a group. | The `filter-log-events` call uses a start time of 1 hour ago. Events older than 1 hour are not shown. |
| A.4.3 | I set `logs.time_range: 7d`. I view SSM Command History. | Commands from the last 7 days are shown. AWS comparison: `aws ssm list-commands --filters Key=InvokedAfter,Value=$(date -u -v-7d +%Y-%m-%dT%H:%M:%SZ)` |

---

## B. Configurable Max Events (Issue #19)

### B.1 Default Max Events (No Config)

| # | Story | Expected |
|---|-------|----------|
| B.1.1 | No config file exists. I open CloudTrail for a busy instance with thousands of events. | At most 1,000 events are loaded. When scrolling to the bottom, the dim hint reads: "Showing 1000 most recent -- more available". |
| B.1.2 | No config file. I open CW Log Viewer for a high-volume log group. | At most 1,000 log events are loaded. The same "Showing 1000 most recent" hint appears at the bottom. |
| B.1.3 | No config file. Fewer than 1,000 events exist for a source. | All events are shown. No "more available" hint appears. |

### B.2 Custom Max Events Values

| # | Story | Expected |
|---|-------|----------|
| B.2.1 | I set `logs.max_events: 100` (minimum). I open CloudTrail. | At most 100 events are loaded. The "Showing 100 most recent -- more available" hint appears if more than 100 exist in the time range. |
| B.2.2 | I set `logs.max_events: 500`. I open Flow Logs. | At most 500 flow log records are loaded. |
| B.2.3 | I set `logs.max_events: 10000` (maximum). I open CloudTrail. | Up to 10,000 events are loaded. This takes longer than the default. The lazy pagination spinner appears as pages are fetched. |
| B.2.4 | I set `logs.max_events: 10000` but only 200 events exist. | All 200 events are shown. No "more available" hint. The frame title shows the actual count (200). |
| B.2.5 | I set `logs.max_events: 100`. I open SSM Command History. | At most 100 commands are shown. |

### B.3 Max Events Boundary Conditions

| # | Story | Expected |
|---|-------|----------|
| B.3.1 | I set `logs.max_events: 99` (below minimum). | The application rejects this value. The error message indicates the valid range is 100-10000. |
| B.3.2 | I set `logs.max_events: 10001` (above maximum). | The application rejects this value. The error message indicates the valid range is 100-10000. |
| B.3.3 | I set `logs.max_events: 0`. | The application rejects this value. Zero events makes no sense. |
| B.3.4 | I set `logs.max_events: -1`. | The application rejects this value. Negative numbers are invalid. |
| B.3.5 | I set `logs.max_events: abc` (non-numeric). | The application rejects this value with a clear error message. |
| B.3.6 | I set `logs.max_events: 1000.5` (float). | The application rejects this value. Max events must be a whole number. |
| B.3.7 | The `logs` key exists in config but `max_events` is absent. | The default 1000 is used. |

### B.4 Max Events Interaction with Pagination

| # | Story | Expected |
|---|-------|----------|
| B.4.1 | I set `logs.max_events: 200`. I open CloudTrail. I scroll to the last row. | The first page loads (e.g., 50 events). As I scroll to the bottom, additional pages load via lazy pagination until 200 total events are reached. The "Loading more..." spinner appears during each page fetch. |
| B.4.2 | Max events is 200. I am at the 200th event and scroll further down. | No additional loading occurs. The dim hint "Showing 200 most recent -- more available" is visible. The cursor stays on the last row. |
| B.4.3 | I press Ctrl+R after reaching the max limit. | The view refreshes. Pagination resets to page 1. The most recent events are fetched again up to the configured max. |

### B.5 Max Events and Console Output

| # | Story | Expected |
|---|-------|----------|
| B.5.1 | I set `logs.max_events: 100`. I open Console Output. | Console Output displays the full buffer (up to AWS's ~64KB limit). The max_events config does NOT apply to Console Output, which is raw text, not event-based. |

---

## C. Config File Handling (Issue #19)

### C.1 Config File Format

| # | Story | Expected |
|---|-------|----------|
| C.1.1 | I create `~/.a9s/config.yaml` with valid `logs:` section: `time_range: 6h` and `max_events: 500`. I launch the app. | The application starts normally. Log views use 6h time range and 500 max events. |
| C.1.2 | The config file `~/.a9s/config.yaml` does not exist. | The application starts with all defaults (24h, 1000). No error is shown. |
| C.1.3 | The config file exists but is empty (zero bytes). | The application starts with all defaults. No error. |
| C.1.4 | The config file contains only `logs:` with no sub-keys. | The application starts with defaults for both time_range and max_events. |
| C.1.5 | The config file has a `logs` section alongside other valid config (e.g., views overrides). | Both the log config and other config sections are respected. |

### C.2 Malformed Config File

| # | Story | Expected |
|---|-------|----------|
| C.2.1 | The config file contains invalid YAML (e.g., tabs instead of spaces, unclosed quotes). | The application either refuses to start with a clear parse error message, or starts with defaults and shows a warning. The app does not crash. |
| C.2.2 | The config file has `logs:` with an unexpected sub-key (e.g., `logs.foo: bar`). | Unknown keys are ignored. Valid keys (`time_range`, `max_events`) are applied. The application does not crash. |
| C.2.3 | The config file has `logs.time_range` set to an integer (e.g., `24`) instead of a string. | The application either interprets it sensibly (as hours), rejects it with a type mismatch error, or falls back to the default. No crash. |
| C.2.4 | The config file permissions are read-only (0400). | The application reads the config normally. Write permission is not needed. |
| C.2.5 | The config file permissions deny read (0000). | The application falls back to defaults with a warning, or shows an error about file permissions. No crash. |

### C.3 Config in views.yaml vs config.yaml

| # | Story | Expected |
|---|-------|----------|
| C.3.1 | The `logs` section is placed in `views.yaml` (as proposed in the issue). | The application reads the log config from wherever it is defined (`~/.a9s/config.yaml` or `~/.a9s/views.yaml`). The behavior is consistent. |
| C.3.2 | Both `~/.a9s/config.yaml` and `~/.a9s/views.yaml` define `logs` settings with conflicting values. | One file takes precedence (the documentation should state which). The user sees consistent behavior, not a mix of values. |

---

## D. Extend `l` Key to Lambda (Issue #20)

### D.1 Lambda Log Source Menu

| # | Story | Expected |
|---|-------|----------|
| D.1.1 | I select a Lambda function in the Lambda resource list and press `l`. | The log source menu opens. The frame title shows the function name (e.g., `log-sources(N) -- my-function`). AWS comparison: `aws lambda get-function --function-name my-function` to confirm the function exists. |
| D.1.2 | The log source menu shows CloudWatch Logs as the primary source. | The menu includes "CloudWatch Logs" with its availability status. Lambda functions have a predictable log group: `/aws/lambda/{function-name}`. |
| D.1.3 | The Lambda function has a standard log group `/aws/lambda/my-function`. | The CloudWatch Logs source shows "Available" or "1 log group" in green. AWS comparison: `aws logs describe-log-groups --log-group-name-prefix /aws/lambda/my-function` |
| D.1.4 | I press Enter on the CloudWatch Logs source. | The CW log group list (or log viewer directly if only one group) opens. I see the log events from `/aws/lambda/my-function`. AWS comparison: `aws logs filter-log-events --log-group-name /aws/lambda/my-function --start-time START` |
| D.1.5 | The Lambda function uses a custom log group name (via `LoggingConfig.LogGroup`). | The log source discovery finds the custom log group instead of the default `/aws/lambda/` prefix. The log group name shown matches the one configured on the function. AWS comparison: `aws lambda get-function --function-name FUNC --query Configuration.LoggingConfig` |

### D.2 Lambda Log Source Availability

| # | Story | Expected |
|---|-------|----------|
| D.2.1 | The Lambda function's log group does not exist (function was never invoked). | CloudWatch Logs shows "Not configured" or "No log group" in dim text. The source is not selectable. |
| D.2.2 | I lack `logs:DescribeLogGroups` permission. | CloudWatch Logs shows "Access denied" in red. The source is not selectable. |
| D.2.3 | The Lambda function has invocations but logs were deleted. | CloudWatch Logs shows "Not configured" (the log group no longer exists). |
| D.2.4 | The log source menu also shows CloudTrail Events for the Lambda function. | CloudTrail source shows "Available" since API calls (Invoke, UpdateFunctionCode, etc.) are always logged. AWS comparison: `aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceType,AttributeValue=AWS::Lambda::Function` |

### D.3 Lambda UX Consistency

| # | Story | Expected |
|---|-------|----------|
| D.3.1 | I navigate Lambda logs: function list -> log sources -> CW logs -> log viewer. I press Esc. | Each Esc pops one level. Log viewer -> CW log groups (if present) -> log sources -> Lambda list. Same stack behavior as EC2 logs. |
| D.3.2 | I press `?` on the Lambda log source menu. | Help screen appears with the same layout as the EC2 log source menu help (NAVIGATION, GENERAL, ACTIONS columns). |
| D.3.3 | I press `/` in the Lambda CW log viewer. | Filter mode activates. I can filter log lines by substring. Same behavior as EC2 CW log viewer. |
| D.3.4 | I press `c` on a log event in the Lambda log viewer. | The log line is copied to clipboard. "Copied!" flash appears in the header. |
| D.3.5 | I press Ctrl+R on the Lambda log source menu. | All availability checks re-run. Spinners appear for conditional sources. |

---

## E. Extend `l` Key to ECS (Issue #20)

### E.1 ECS Service Log Source Menu

| # | Story | Expected |
|---|-------|----------|
| E.1.1 | I view ECS Services (child of ECS Clusters). I select a service and press `l`. | The log source menu opens showing sources relevant to ECS services. The frame title includes the service name. |
| E.1.2 | The log source menu shows CloudWatch Logs (awslogs driver) as a source. | If the service's task definition uses the `awslogs` log driver, the source shows "Available" with the log group name. AWS comparison: `aws ecs describe-task-definition --task-definition TASK_DEF --query taskDefinition.containerDefinitions[].logConfiguration` |
| E.1.3 | The ECS service's task definition uses a non-awslogs driver (e.g., `splunk`, `fluentd`). | CloudWatch Logs source shows "Not configured" or "Non-awslogs driver" in dim text. The source is not selectable. This is graceful degradation -- the user sees why logs are unavailable. |
| E.1.4 | The ECS service has multiple containers with different log groups. | The CW log group list shows all log groups from all container definitions. Each group is listed separately. |
| E.1.5 | The log source menu also includes CloudTrail Events for the ECS service. | CloudTrail source shows "Available". AWS comparison: `aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceType,AttributeValue=AWS::ECS::Service` |

### E.2 ECS Log Discovery

| # | Story | Expected |
|---|-------|----------|
| E.2.1 | The ECS service's task definition specifies `logConfiguration` with `awslogs-group`. | The log group is discovered directly from the task definition -- no scanning required. The source resolves quickly (no "Checking..." spinner, or only briefly). |
| E.2.2 | I lack `ecs:DescribeTaskDefinition` permission. | The log source shows "Access denied" in red. |
| E.2.3 | The task definition references a log group that no longer exists. | The source shows "Log group not found" or similar in dim text. |
| E.2.4 | I press Enter on an available CW log source for an ECS service. | The CW log viewer opens, showing log events from the container's log group. Stream names typically contain the task ID. AWS comparison: `aws logs filter-log-events --log-group-name /ecs/my-service` |

---

## F. Extend `l` Key to EKS Node Groups (Issue #20)

### F.1 EKS Node Group Log Sources

| # | Story | Expected |
|---|-------|----------|
| F.1.1 | I view EKS Node Groups. I select a node group and press `l`. | The log source menu opens. Since EKS node group nodes are EC2 instances, the log sources are the same as (or a subset of) the EC2 log sources. |
| F.1.2 | The node group has 3 instances. The log source menu shows instance-level sources. | The user first sees a list of EC2 instances in the node group, or the log sources apply to the node group collectively. The UX makes clear which instance's logs are being viewed. |
| F.1.3 | I select a node group instance and open CloudTrail Events. | CloudTrail events for that specific EC2 instance are shown. This reuses the EC2 CloudTrail sub-view. AWS comparison: `aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceType,AttributeValue=AWS::EC2::Instance` with the node's instance ID. |
| F.1.4 | I select a node group instance and open Console Output. | The EC2 console output for that node instance is shown. AWS comparison: `aws ec2 get-console-output --instance-id INSTANCE_ID` |
| F.1.5 | The node group has zero instances (scaled to 0). | The log source menu shows an appropriate message (e.g., "No instances in node group") or the `l` key shows a flash message indicating no instances to view logs for. |

---

## G. Extend `l` Key to RDS (Issue #20)

### G.1 RDS Log Source Menu

| # | Story | Expected |
|---|-------|----------|
| G.1.1 | I select an RDS instance in the RDS list and press `l`. | The log source menu opens. The frame title includes the DB identifier (e.g., `log-sources(N) -- prod-database`). |
| G.1.2 | The log source menu shows "RDS Log Files" as a source. | RDS has its own log API (`DescribeDBLogFiles`). The source shows "Available" with the number of log files. AWS comparison: `aws rds describe-db-log-files --db-instance-identifier prod-database` |
| G.1.3 | The RDS instance has 5 log files (e.g., error/general.log, slowquery/...). | The log file list shows all 5 files with their names and sizes. |
| G.1.4 | The log source menu also shows CloudTrail Events for the RDS instance. | CloudTrail source shows "Available". AWS comparison: `aws cloudtrail lookup-events --lookup-attributes AttributeKey=ResourceType,AttributeValue=AWS::RDS::DBInstance` |
| G.1.5 | The RDS instance has CloudWatch Logs enabled (e.g., error log exported to CloudWatch). | A CloudWatch Logs source appears showing the relevant log groups (e.g., `/aws/rds/instance/DB_ID/error`). AWS comparison: `aws logs describe-log-groups --log-group-name-prefix /aws/rds/instance/prod-database` |

### G.2 RDS Log File Viewer

| # | Story | Expected |
|---|-------|----------|
| G.2.1 | I press Enter on an RDS log file (e.g., `error/postgresql.log`). | A viewport opens showing the log file content. The frame title includes the file name. AWS comparison: `aws rds download-db-log-file-portion --db-instance-identifier DB --log-file-name error/postgresql.log` |
| G.2.2 | The RDS log file is very large. | The viewer fetches the most recent portion (using the RDS log API's marker-based pagination). Lazy loading works as the user scrolls. The configured `max_events` limit applies to the number of download portions. |
| G.2.3 | I press `/` in the RDS log viewer. | Filter mode activates. I can filter log lines by substring, same as other viewport views. |
| G.2.4 | I press `c` in the RDS log viewer. | The visible content is copied to clipboard. "Copied!" flash appears. |
| G.2.5 | I press `w` in the RDS log viewer. | Word wrap toggles, same as Console Output and CW Log Viewer. |

### G.3 RDS Log API Errors

| # | Story | Expected |
|---|-------|----------|
| G.3.1 | The RDS instance has no log files (logging disabled). | The "RDS Log Files" source shows "No log files" in dim text. Not selectable. |
| G.3.2 | I lack `rds:DescribeDBLogFiles` permission. | The source shows "Access denied" in red. Not selectable. |
| G.3.3 | The RDS instance is in `creating` state and logs are not yet available. | The source shows "Unavailable" in dim text. Not selectable. |
| G.3.4 | I request a log file download and the API returns a throttling error. | A red error flash appears in the header. The view shows a centered error message. Ctrl+R retries. |
| G.3.5 | The RDS instance has been deleted but is still visible in the list (e.g., final snapshot state). | The log sources show "Unavailable" for RDS-specific sources. CloudTrail may still show recent API events. |

---

## H. `l` Key Scope and Graceful Degradation (Issue #20)

### H.1 Resource Types Without Log Support

| # | Story | Expected |
|---|-------|----------|
| H.1.1 | I select an S3 bucket and press `l`. | Nothing happens. The `l` key is not active for S3. No error, no flash message. |
| H.1.2 | I select a VPC and press `l`. | Nothing happens. The `l` key is not active for VPC. |
| H.1.3 | I select a Security Group and press `l`. | Nothing happens. |
| H.1.4 | I press `?` on the EC2 list. The help screen shows `l` under HOTKEYS or ACTIONS. | The `l` key binding is listed in the help screen for EC2, Lambda, ECS, EKS Node Groups, and RDS. |
| H.1.5 | I press `?` on the S3 bucket list. | The `l` key binding does NOT appear in the help screen, since S3 does not support log views. |

### H.2 Graceful Degradation Per Resource Type

| # | Story | Expected |
|---|-------|----------|
| H.2.1 | A Lambda function exists but I have no CloudWatch Logs permissions. | The log source menu opens. CloudWatch Logs shows "Access denied" in red. CloudTrail (if included) may still be accessible. |
| H.2.2 | An ECS service uses a Fargate launch type with no awslogs driver configured. | CloudWatch Logs source shows "Not configured". The user sees why (non-awslogs driver) rather than a cryptic error. |
| H.2.3 | An RDS instance has all logging disabled (no error log, no slow query log, no audit log). | The log source menu opens. "RDS Log Files" shows "No log files" in dim. CloudWatch Logs shows "Not configured" if CloudWatch export is off. CloudTrail remains available. |
| H.2.4 | An EKS Node Group's instances have been replaced during a rolling update. The old instance IDs no longer exist. | CloudTrail may show events for old IDs. Console Output is unavailable for terminated nodes. The log source menu reflects current node state. |

### H.3 UX Pattern Consistency Across Resource Types

| # | Story | Expected |
|---|-------|----------|
| H.3.1 | I open logs for EC2, then go back. I open logs for Lambda. | Both log source menus follow the same UX pattern: same layout, same navigation (j/k), same availability indicators, same color coding. |
| H.3.2 | I open logs for RDS and select CloudTrail Events. | The CloudTrail sub-view uses the same columns (Time, Event, User, Source IP), same sorting (N/S/A), same navigation, and same detail view as EC2's CloudTrail sub-view. |
| H.3.3 | I open CW Log Viewer from Lambda, and from ECS. | Both use the same viewport-style log viewer with the same keybindings: j/k scroll, / filter, c copy, w word wrap, Ctrl+R refresh, Esc back. |

---

## I. Log Source Cache -- First Press (Issue #21)

### I.1 Cache Miss (First `l` Press)

| # | Story | Expected |
|---|-------|----------|
| I.1.1 | I launch the app fresh. I select an EC2 instance and press `l` for the first time. | The log source menu opens. "Always available" sources (CloudTrail, Console Output, Instance Status) show "Available" immediately. "Conditional" sources (CloudWatch, Flow Logs, SSM) show "Checking..." with a spinner while discovery runs. |
| I.1.2 | CloudWatch discovery completes and finds 3 log groups. | The "Checking..." spinner for CloudWatch Logs is replaced by "3 log groups" in green. The row becomes selectable. |
| I.1.3 | VPC Flow Logs discovery completes: no flow logs configured. | The spinner is replaced by "Not configured" in dim. The row is not selectable (cursor skips it). |
| I.1.4 | SSM discovery completes: 2 commands found. | The spinner is replaced by "Available" in green. |
| I.1.5 | I observe the total time for first-press discovery. | Discovery for the three conditional sources runs in parallel (not sequentially). All three spinners are visible simultaneously. Total time is bounded by the slowest source, not the sum. |
| I.1.6 | Discovery takes more than a few seconds (slow API). | The spinners remain animated. The user can still navigate (j/k) among the already-resolved sources. The menu is usable before all checks complete. |

### I.2 Cache Miss for Lambda

| # | Story | Expected |
|---|-------|----------|
| I.2.1 | I press `l` on a Lambda function for the first time. | Discovery runs: checks for the `/aws/lambda/{function-name}` log group. Shows "Checking..." briefly, then resolves to "1 log group" or "Not configured". |
| I.2.2 | The Lambda function has a custom log group. | Discovery checks `LoggingConfig.LogGroup` first, then falls back to the default prefix. The correct custom group is found and shown. |

### I.3 Cache Miss for RDS

| # | Story | Expected |
|---|-------|----------|
| I.3.1 | I press `l` on an RDS instance for the first time. | Discovery runs: calls `DescribeDBLogFiles` to find available log files. Shows "Checking..." then resolves to "5 log files" or similar. |

---

## J. Log Source Cache -- Subsequent Presses (Issue #21)

### J.1 Cache Hit (Second and Later `l` Press)

| # | Story | Expected |
|---|-------|----------|
| J.1.1 | I pressed `l` on an EC2 instance earlier. I go back to the EC2 list and press `l` on the SAME instance again. | The log source menu opens INSTANTLY with cached results. No "Checking..." spinners appear. All sources show their previously discovered status. |
| J.1.2 | Cached results show the same availability status as the first press. | If CloudWatch showed "3 log groups" on first press, it still shows "3 log groups" on second press. |
| J.1.3 | I pressed `l` on instance A. I now press `l` on instance B (different instance). | Instance B is a cache MISS. Discovery runs with spinners for instance B. Instance A's cache is unaffected. |
| J.1.4 | I pressed `l` on a Lambda function earlier. I press `l` on the same function again. | Instant display with cached results. No re-discovery. |
| J.1.5 | I pressed `l` on an RDS instance earlier. I press `l` again. | Instant display. No `DescribeDBLogFiles` call is made. |

### J.2 Cache Granularity

| # | Story | Expected |
|---|-------|----------|
| J.2.1 | I check logs for EC2 instance `i-abc`. I then check logs for EC2 instance `i-def`. I return to `i-abc`. | Instance `i-abc` uses its cache (instant). Instance `i-def` was a miss when first visited. Returning to `i-abc` is a hit (instant). |
| J.2.2 | I check logs for Lambda function `my-func`. I check logs for RDS instance `my-db`. | Each resource has its own cache entry. They do not interfere with each other. |

---

## K. Log Source Cache -- Invalidation (Issue #21)

### K.1 Manual Cache Invalidation (Ctrl+R)

| # | Story | Expected |
|---|-------|----------|
| K.1.1 | I am on the log source menu (cached results displayed). I press Ctrl+R. | The cache for THIS resource is invalidated. "Checking..." spinners reappear for conditional sources. Discovery re-runs from scratch. |
| K.1.2 | After Ctrl+R, CloudWatch now finds 4 log groups (was 3 cached). | The count updates to "4 log groups". A new log group was created since the original discovery. |
| K.1.3 | After Ctrl+R, VPC Flow Logs now shows "Available" (was "Not configured" cached). | The user enabled flow logs since the first check. The cache refresh picks this up. |
| K.1.4 | I press Ctrl+R on the log source menu. I go back, then press `l` again. | The refreshed results are now cached. The second `l` press shows the updated results instantly. |
| K.1.5 | I press Ctrl+R while on a log SUB-view (e.g., CloudTrail event table). | The sub-view data refreshes (re-fetches events from the API). This does NOT invalidate the log source menu cache. |

### K.2 Profile Switch Cache Invalidation

| # | Story | Expected |
|---|-------|----------|
| K.2.1 | I have cached log sources for instance `i-abc` under profile `prod`. I switch to profile `staging` via `:ctx`. | The entire log source cache is cleared. All profiles' cached data is discarded. |
| K.2.2 | After switching to `staging`, I press `l` on any instance. | Full discovery runs. No cached data from the `prod` profile is reused. |
| K.2.3 | I switch back to `prod`. I press `l` on `i-abc`. | Full discovery runs again. The cache was cleared on profile switch and is not restored when switching back. |

### K.3 Region Switch Cache Invalidation

| # | Story | Expected |
|---|-------|----------|
| K.3.1 | I have cached log sources for instance `i-abc` in `us-east-1`. I switch to `eu-west-1` via `:region`. | The entire log source cache is cleared. |
| K.3.2 | After switching to `eu-west-1`, I press `l` on an instance. | Full discovery runs. No cached data from `us-east-1` is reused. |

### K.4 Session Scope

| # | Story | Expected |
|---|-------|----------|
| K.4.1 | I use the app, build up a log source cache, then quit (q or Ctrl+C). I relaunch the app. | The cache is empty. All `l` presses trigger fresh discovery. The cache does not persist to disk. |
| K.4.2 | I have been running the app for 2 hours with cached log sources. I press `l` on an old instance. | Cached results from 2 hours ago are returned instantly. The cache has no automatic time-based expiration within a session. Only manual Ctrl+R, profile switch, or region switch invalidate it. |

---

## L. Cache and Configuration Interaction

### L.1 Config Changes and Cache

| # | Story | Expected |
|---|-------|----------|
| L.1.1 | I have `logs.time_range: 24h` configured. I check logs, cache is populated. I quit, change config to `1h`, relaunch. | The cache is empty (session-scoped). New discovery and data fetches use the 1h time range. |
| L.1.2 | The cache stores log SOURCE availability, not log EVENT data. | Changing `max_events` from 1000 to 500 does not require cache invalidation. The cache only affects the source menu; actual log events are always fetched fresh when opening a sub-view. |

---

## M. Combined Scenarios

### M.1 End-to-End Lambda Workflow

| # | Story | Expected |
|---|-------|----------|
| M.1.1 | I set `logs.time_range: 1h`, `logs.max_events: 500`. I select a Lambda function, press `l`, select CloudWatch Logs, open the log viewer. | The log viewer shows events from the last 1 hour, up to 500 events. Filter (`/`), copy (`c`), word wrap (`w`), and refresh (Ctrl+R) all work. |
| M.1.2 | I go back (Esc) to the log source menu. I press `l` again on the same function. | The log source menu opens instantly (cached). I open CloudWatch Logs again. The log events are fetched fresh (not cached -- only source availability is cached). |

### M.2 End-to-End RDS Workflow

| # | Story | Expected |
|---|-------|----------|
| M.2.1 | I set `logs.time_range: 7d`, `logs.max_events: 2000`. I select an RDS instance, press `l`, select RDS Log Files. | The RDS log file list shows available files. I select one and see the log content. |
| M.2.2 | I go back and select CloudTrail Events. | CloudTrail events for the RDS instance over the last 7 days are shown, up to 2000 events. |

### M.3 Cross-Resource Cache Behavior

| # | Story | Expected |
|---|-------|----------|
| M.3.1 | I check logs for an EC2 instance (cached), then a Lambda function (cached), then an RDS instance (cached). I switch profiles. | All three cache entries are cleared. Next `l` press on any resource triggers fresh discovery. |
| M.3.2 | I check logs for EC2 instance A (cached). I press Ctrl+R on instance A's log source menu. I then check logs for Lambda function B. | Instance A's cache is refreshed (re-discovered). Lambda B's cache is untouched (if it existed) or triggers fresh discovery (if it didn't). Ctrl+R on A does not affect B. |

### M.4 Empty State with Custom Config

| # | Story | Expected |
|---|-------|----------|
| M.4.1 | I set `logs.time_range: 1h`. No CloudTrail events occurred in the last hour. | CloudTrail sub-view shows: "No CloudTrail events in the last 1 hour". The empty state message reflects the configured time range, not the default. |
| M.4.2 | I set `logs.time_range: 7d`. Many events exist but `max_events: 100` is hit. | The sub-view shows 100 events with the "Showing 100 most recent -- more available" hint. The time range is 7 days, but only 100 events are loaded. |

---

## N. Cross-Cutting Concerns

### N.1 Header Consistency in Log Views

| # | Story | Expected |
|---|-------|----------|
| N.1.1 | In every log-related view (source menu, CloudTrail table, CW log viewer, RDS log viewer, etc.), the header displays: "a9s" (accent bold), version (dim), profile:region (bold). | Visual inspection confirms across all log views for all supported resource types (EC2, Lambda, ECS, EKS Node Groups, RDS). |
| N.1.2 | The header right side shows "? for help" in normal mode across all log views. | Confirmed in all log source menus and sub-views. |
| N.1.3 | Flash messages (Copied!, errors) work in all log sub-views. | Pressing `c` in any log sub-view shows the green "Copied!" flash. API errors show the red error flash. |

### N.2 View Stack Depth

| # | Story | Expected |
|---|-------|----------|
| N.2.1 | Lambda list -> log sources -> CW log viewer. Esc three times. | Returns to Lambda list -> log sources -> Lambda list. |
| N.2.2 | EC2 list -> log sources -> CW log groups -> log viewer. Esc four times. | Log viewer -> CW log groups -> log sources -> EC2 list. Deepest path (4 levels). |
| N.2.3 | RDS list -> log sources -> RDS log file viewer. Esc twice. | RDS log viewer -> log sources -> RDS list. |
| N.2.4 | ECS services -> log sources -> CW log viewer. Esc to ECS services. Then navigate to EC2, press `l`. | The ECS view stack is properly unwound. The EC2 log source menu opens fresh. |

### N.3 Terminal Resize in Log Views

| # | Story | Expected |
|---|-------|----------|
| N.3.1 | I resize the terminal while viewing the log source menu. | The layout reflows. The frame border redraws. Source names and availability text adjust to the new width. |
| N.3.2 | I resize the terminal while viewing a CloudTrail table sub-view. | Columns adjust per the standard responsive behavior. At 60-79 cols, only Time and Event columns are visible. At 120+, all 4 columns are visible. |
| N.3.3 | I resize the terminal below 60 columns while in any log view. | The "Terminal too narrow. Please resize." message appears. |
| N.3.4 | I resize the terminal while viewing a CW Log Viewer (viewport). | The viewport adjusts to the new dimensions. Long lines are truncated (or wrapped if `w` was toggled). |

### N.4 Sorting in Log Table Sub-Views

| # | Story | Expected |
|---|-------|----------|
| N.4.1 | I press `N` in the CloudTrail table for a Lambda function. | Events are sorted by event name. Sort indicator appears on the "Event" column header. |
| N.4.2 | I press `A` in the SSM Commands table (from an EC2 log view). | Commands are sorted by time (age). Sort indicator on the "Time" column. |
| N.4.3 | I sort, then Ctrl+R. | After refresh, the sort order and direction are preserved. |

### N.5 Filter in Log Views

| # | Story | Expected |
|---|-------|----------|
| N.5.1 | I press `/` in the CloudTrail table and type "Stop". | Only CloudTrail events with "Stop" in their event name (or any visible field) are shown. The frame title updates to show matched/total (e.g., "cloudtrail(3/156)"). |
| N.5.2 | I press `/` in the CW Log Viewer and type "ERROR". | Only log lines containing "ERROR" (case-insensitive) are shown. Non-matching lines are hidden. |
| N.5.3 | I press Esc while filter is active. | The filter clears. All rows/lines reappear. The header right reverts to "? for help". |

### N.6 Copy Behavior in Extended Log Views

| # | Story | Expected |
|---|-------|----------|
| N.6.1 | I press `c` in the Lambda CW Log Viewer. | The current log line is copied. "Copied!" flash in header. |
| N.6.2 | I press `c` in the RDS Log File Viewer. | The visible content (or current line) is copied. "Copied!" flash. |
| N.6.3 | I press `c` in the CloudTrail table for an RDS instance. | The full CloudTrail event JSON for the selected row is copied. Same behavior as EC2 CloudTrail. |
