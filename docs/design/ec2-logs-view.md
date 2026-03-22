# Child View: EC2 Instances -> Logs

**Status:** Planned
**Tier:** SHOULD-HAVE
**Priority:** P1

## Summary

Press `l` on an EC2 instance to see a categorized menu of available log
sources, then drill into any source to view and filter logs. Answers the
incident-response question: "what happened on this instance?"

This is the deepest view stack in a9s (up to 4 levels for CloudWatch Logs).
The design intentionally uses the same list/viewport patterns as every other
view -- no new chrome or navigation concepts.

---

## Navigation

- **Entry:** Press `l` on an EC2 instance in the resource list
- **Frame title:** `log-sources(6) -- i-0abc123def456 (web-server-prod)`
- **View stack depths:**
  - EC2 List -> Log Sources (2 levels)
  - EC2 List -> Log Sources -> CloudTrail / Flow Logs / SSM Commands (3 levels -- table views)
  - EC2 List -> Log Sources -> Console Output / Instance Status (3 levels -- viewport views)
  - EC2 List -> Log Sources -> CW Log Groups -> Log Viewer (4 levels -- deepest path)
- **Esc** pops one level at each step, all the way back to EC2 List.

Note: `l` must be added to the Resource List key bindings in `design.md`
section 5, scoped to EC2 only.

---

## State Transitions

| Msg                       | From                   | To                      | Notes                                     |
|---------------------------|------------------------|-------------------------|-------------------------------------------|
| `KeyMsg(l)`               | EC2 ResourceListView   | LogSourceMenuView       | Push view stack; start availability checks |
| `LogSourcesCheckedMsg`    | LogSourceMenu:loading  | LogSourceMenu:ready     | Cheap checks (CloudTrail, Console, Status) |
| `LogSourceDiscoveredMsg`  | LogSourceMenu:ready    | LogSourceMenu:updated   | CW/FlowLog/SSM results arrive async       |
| `KeyMsg(enter)`           | LogSourceMenuView      | Sub-view (varies)       | Only if source is available, not disabled  |
| `KeyMsg(enter)`           | CWLogGroupsView        | CWLogViewerView         | Push; fetch log events                     |
| `LogEventsLoadedMsg`      | Sub-view:loading       | Sub-view:ready          | Populates table/viewport                   |
| `APIErrorMsg`             | Sub-view:loading       | Header flash error      | Per-source error, not global crash         |
| `KeyMsg(esc)`             | Any log sub-view       | Previous view           | Pop view stack                             |
| `MoreEventsLoadedMsg`     | Sub-view:ready         | Sub-view:appended       | Lazy pagination result appended            |

---

## 1. Log Source Menu

A simple list where each row is a log source with an availability status on
the right. The three "always available" sources (CloudTrail, Console Output,
Instance Status) resolve immediately. The three "conditional" sources
(CloudWatch, Flow Logs, SSM) show a spinner while their availability is
checked in the background, then update in place.

Unavailable sources are shown but dimmed and cannot be selected (cursor skips
them). This way the user always sees the full picture rather than a subset
that changes.

### Wireframe -- Normal (all checks complete)

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------------- log-sources(6) -- i-0abc123def456 (web-server-prod) ---------------+
| [SELECTED]  CloudTrail Events              Available                               [/]|
| [GREEN]  System Console Output          Available                                  [/]|
| [GREEN]  CloudWatch Logs                3 log groups                               [/]|
| [GREEN]  Instance Status Events         Available                                  [/]|
| [DIM]  VPC Flow Logs                  Not configured                               [/]|
| [DIM]  SSM Command History            No commands found                            [/]|
|                                                                                      |
|                                                                                      |
+--------------------------------------------------------------------------------------+
```

### Wireframe -- Loading (discovery in progress)

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------------- log-sources(6) -- i-0abc123def456 (web-server-prod) ---------------+
| [SELECTED]  CloudTrail Events              Available                               [/]|
| [GREEN]  System Console Output          Available                                  [/]|
| [SPINNER]  CloudWatch Logs                Checking...                              [/]|
| [GREEN]  Instance Status Events         Available                                  [/]|
| [SPINNER]  VPC Flow Logs                  Checking...                              [/]|
| [SPINNER]  SSM Command History            Checking...                              [/]|
|                                                                                      |
|                                                                                      |
+--------------------------------------------------------------------------------------+
```

### Wireframe -- Error (permission denied on one source)

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------------- log-sources(6) -- i-0abc123def456 (web-server-prod) ---------------+
| [SELECTED]  CloudTrail Events              Available                               [/]|
| [GREEN]  System Console Output          Available                                  [/]|
| [RED]  CloudWatch Logs                Access denied                                [/]|
| [GREEN]  Instance Status Events         Available                                  [/]|
| [DIM]  VPC Flow Logs                  Not configured                               [/]|
| [DIM]  SSM Command History            No commands found                            [/]|
|                                                                                      |
|                                                                                      |
+--------------------------------------------------------------------------------------+
```

### Wireframe -- Terminated Instance

For terminated instances, Console Output and Instance Status are unavailable.
CloudTrail still works (up to 90 days of history).

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------------- log-sources(6) -- i-0abc123def456 (legacy-app) --------------------+
| [SELECTED]  CloudTrail Events              Available                               [/]|
| [DIM]  System Console Output          Unavailable (terminated)                     [/]|
| [DIM]  CloudWatch Logs                Not configured                               [/]|
| [DIM]  Instance Status Events         Unavailable (terminated)                     [/]|
| [DIM]  VPC Flow Logs                  Not configured                               [/]|
| [DIM]  SSM Command History            No commands found                            [/]|
|                                                                                      |
|                                                                                      |
+--------------------------------------------------------------------------------------+
```

### Status Colors

The availability text on the right uses the standard status color mapping:

| Status text                           | Color          | Hex       |
|---------------------------------------|----------------|-----------|
| Available, N log groups               | GREEN          | `#9ece6a` |
| Checking...                           | YELLOW/spinner | `#e0af68` |
| Not configured, No commands found     | DIM            | `#565f89` |
| Unavailable (terminated)              | DIM            | `#565f89` |
| Access denied                         | RED            | `#f7768e` |

Source name on the left uses normal text `#c0caf5` when available, dim
`#565f89` when unavailable.

### Behavior

- Enter on an available source opens the corresponding sub-view (see below).
- Enter on an unavailable/dimmed source does nothing (no error, no flash).
- Cursor (j/k) skips unavailable sources.
- `/` filter is **not** available on this menu (too few items to warrant it).
- `?` opens help for this view.
- `Ctrl+R` re-runs all availability checks.

---

## 2. Log Sub-Views

Each log source opens a different sub-view. They fall into two categories:
**table views** (CloudTrail, Flow Logs, SSM Commands) and **viewport views**
(Console Output, Instance Status, CW Log Viewer). Both categories inherit the
standard a9s frame, key bindings, and status colors.

### Common Behavior Across All Sub-Views

- **Time range:** Last 24 hours by default for all time-based sources.
- **Max events:** 1,000 entries (most recent first). If the limit is hit,
  show a dim hint on the last visible line: `[DIM]Showing 1000 most recent -- more available[/]`
- **Lazy pagination:** Fetch the first page only. When the user scrolls to the
  last row, fetch the next page (up to the 1,000 max). Show a spinner on the
  last line while loading: `[SPINNER] Loading more...[/]`
- **`Ctrl+R`:** Re-fetches from the API, resets to most-recent-first.
- **Loading state:** Centered spinner, same as all other views (see design.md
  section 4.2 loading wireframe).
- **Empty state:** Centered dim text, source-specific (see each sub-view).
- **Error state:** Header flash error (red, auto-clears after 2s), view shows
  centered dim error message.

---

### 2.1 CloudTrail Events (table view)

API activity against this instance -- start, stop, terminate, SG changes, etc.

**API:** `cloudtrail:LookupEvents` with `LookupAttributes`:
`ResourceType=AWS::EC2::Instance, ResourceName=instance-id`

**Pagination:** `NextToken` from response. First page only; lazy-load more.

#### views.yaml

```yaml
ec2_cloudtrail:
  list:
    Time:
      path: EventTime
      width: 22
    Event:
      path: EventName
      width: 26
    User:
      path: Username
      width: 24
    Source IP:
      path: SourceIPAddress
      width: 16
  detail:
    - EventId
    - EventTime
    - EventName
    - EventSource
    - Username
    - SourceIPAddress
    - ReadOnly
    - AccessKeyId
    - CloudTrailEvent
```

#### views_reference.yaml

Source struct: `cloudtrailtypes.Event`

```
- AccessKeyId
- CloudTrailEvent        (raw JSON string)
- EventId
- EventName
- EventSource
- EventTime
- ReadOnly
- Resources[].ResourceName
- Resources[].ResourceType
- Username
```

#### Wireframe -- Normal

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------- cloudtrail(12) -- i-0abc123def456 (web-server-prod) ---------------------+
| TIME                   EVENT                      USER                  SOURCE IP   |
| [SELECTED] 2024-01-15 14:23:05  StopInstances            deploy-role           10.0.1.50  [/]|
| 2024-01-15 14:22:58  ModifyInstanceAttribute  admin@company.com     203.0.113.5  |
| 2024-01-15 09:15:00  StartInstances           autoscaling.aws.com   --           |
| 2024-01-15 09:14:55  RunInstances             deploy-role           10.0.1.50    |
| 2024-01-15 09:10:00  CreateTags               deploy-role           10.0.1.50    |
|   . . . (7 more)                                                                  |
+------------------------------------------------------------------------------------+
```

#### Wireframe -- Empty

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------- cloudtrail(0) -- i-0abc123def456 (web-server-prod) ----------------------+
|                                                                                     |
|              [DIM]No CloudTrail events in the last 24 hours[/]                      |
|                                                                                     |
+-------------------------------------------------------------------------------------+
```

#### Wireframe -- Data Limit Reached

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------- cloudtrail(1000) -- i-0abc123def456 (web-server-prod) -------------------+
| TIME                   EVENT                      USER                  SOURCE IP   |
| [SELECTED] 2024-01-15 14:23:05  StopInstances            deploy-role           10.0.1.50  [/]|
| 2024-01-15 14:22:58  ModifyInstanceAttribute  admin@company.com     203.0.113.5  |
| ...                                                                                |
| [DIM]Showing 1000 most recent -- more available[/]                                 |
+------------------------------------------------------------------------------------+
```

**`d` (detail):** Opens a detail view of the selected CloudTrail event,
showing the fields from the `detail` config above.

**`y` (YAML):** Opens the raw `CloudTrailEvent` JSON (which is stored as a
JSON string in the API response) in the YAML viewer.

**`c` copies:** Full `CloudTrailEvent` JSON for the selected row.

---

### 2.2 System Console Output (viewport view)

Serial console buffer -- kernel messages, boot logs, cloud-init output. Up
to 64KB, capped by AWS.

**API:** `ec2:GetConsoleOutput` (InstanceId, Latest=true)

The response contains base64-encoded console output. Decode and render as a
scrollable text viewport (same component as YAML view).

No `views.yaml` entry needed -- this is raw text, not a table.

#### Wireframe -- Normal

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------- console-output -- i-0abc123def456 (web-server-prod) ---------------------+
| [    0.000000] Linux version 5.15.0-1051-aws (buildd@lcy02-amd64-003)             |
| [    0.000000] Command line: BOOT_IMAGE=/boot/vmlinuz-5.15.0-1051-aws root=...    |
| [    0.000000] KERNEL supported cpus:                                              |
| [    0.000000]   Intel GenuineIntel                                                |
| [    0.000000]   AMD AuthenticAMD                                                  |
| ...                                                                                |
| [   12.345678] cloud-init[1234]: Cloud-init v. 23.4.1 finished at Mon, 15 Jan...  |
| [   12.345679] cloud-init[1234]: DataSourceEc2Local: Crawl of metadata service... |
+------------------------------------------------------------------------------------+
```

#### Wireframe -- Empty (no output available)

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------- console-output -- i-0abc123def456 (web-server-prod) ---------------------+
|                                                                                     |
|              [DIM]No console output available[/]                                    |
|              [DIM]Instance may not have produced console output yet[/]              |
|                                                                                     |
+-------------------------------------------------------------------------------------+
```

**`/` filter:** Filters visible lines by substring match (case-insensitive).
Matching lines are shown; non-matching lines are hidden. The frame title does
NOT change (no count -- this is a viewport, not a table). Header shows
`/search-text` in amber. Esc clears the filter and restores all lines.

**`d`/`y`:** Not applicable -- this is already raw text.

**`c` copies:** Entire console output text to clipboard.

**`w`:** Toggle word wrap (same as detail/YAML views).

---

### 2.3 CloudWatch Logs (table + viewport, 2 sub-levels)

Application and system logs shipped to CloudWatch. This is the only log source
with two sub-levels: a log group list, then a log event viewer.

#### Discovery

Finding which log groups contain streams for this instance requires scanning.
The strategy:

1. Try common-prefix heuristics first: `/ec2/`, `/aws/ec2/`, `/var/log/`,
   instance-id as prefix.
2. Fall back to iterating `DescribeLogGroups` and calling
   `DescribeLogStreams` with `logStreamNamePrefix=instance-id` on each group.
3. **Timeout:** Stop scanning after 30 seconds or 200 log groups, whichever
   comes first. If the limit is hit, show `[DIM]Scanned 200 of N groups -- some may be missing[/]`.
4. Results are cached for the session (until Ctrl+R forces re-discovery).

**API:**
- `logs:DescribeLogGroups` -- list groups (paginated)
- `logs:DescribeLogStreams` -- find streams matching instance ID
- `logs:FilterLogEvents` -- read log entries from a specific group+stream

#### views.yaml (log group list)

```yaml
ec2_cw_log_groups:
  list:
    Log Group:
      path: LogGroupName
      width: 60
    Streams:
      key: stream_count
      width: 10
    Last Event:
      key: last_event_time
      width: 22
```

No `detail` section -- Enter opens the log event viewer, not a detail view.

#### Wireframe -- CW Log Group List

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------- cw-log-groups(3) -- i-0abc123def456 (web-server-prod) -------------------+
| LOG GROUP                                                     STREAMS  LAST EVENT   |
| [SELECTED] /ec2/web-server/var/log/syslog                          2  2024-01-15 14:23  [/]|
| /ec2/web-server/var/log/auth.log                              1  2024-01-15 09:10  |
| /ec2/web-server/app/api-server                                1  2024-01-15 14:22  |
|                                                                                     |
+-------------------------------------------------------------------------------------+
```

#### Wireframe -- CW Log Group List (empty)

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------- cw-log-groups(0) -- i-0abc123def456 (web-server-prod) -------------------+
|                                                                                     |
|              [DIM]No CloudWatch log groups found for this instance[/]               |
|              [DIM]CloudWatch agent may not be installed[/]                          |
|                                                                                     |
+-------------------------------------------------------------------------------------+
```

#### Log Event Viewer (viewport view)

After selecting a log group, the user sees a scrollable stream of log events.

**API:** `logs:FilterLogEvents` with `logGroupName`, `logStreamNames`
(streams matching this instance), `startTime` (24h ago), `limit` (100 per
page, up to 1000 total). Paginated via `nextToken`.

No `views.yaml` entry -- this is raw text in a viewport, not a table. Each
line is rendered as: `[DIM]timestamp[/]  message-text`

#### Wireframe -- Log Viewer

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------- /ec2/web-server/var/log/syslog -- i-0abc123def456 -----------------------+
| [DIM]2024-01-15 14:23:05[/]  web-server kernel: [42.123] Out of memory: Kill...    |
| [DIM]2024-01-15 14:23:04[/]  web-server kernel: [42.122] oom-kill: constraint...   |
| [DIM]2024-01-15 14:22:58[/]  web-server systemd[1]: api-server.service: Main...    |
| [DIM]2024-01-15 14:22:57[/]  web-server api-server[1234]: FATAL: database co...    |
| [DIM]2024-01-15 14:22:50[/]  web-server api-server[1234]: ERROR: health check...   |
| [DIM]2024-01-15 14:22:45[/]  web-server api-server[1234]: WARN: connection po...   |
| ...                                                                                 |
+-------------------------------------------------------------------------------------+
```

#### Wireframe -- Log Viewer (loading more)

When the user scrolls to the bottom and more pages are available:

```
| [DIM]2024-01-15 14:20:01[/]  web-server api-server[1234]: INFO: request compl...   |
| [DIM]2024-01-15 14:20:00[/]  web-server api-server[1234]: INFO: request start...   |
| [SPINNER] Loading more...[/]                                                        |
+-------------------------------------------------------------------------------------+
```

#### Wireframe -- Log Viewer (limit reached)

```
| [DIM]2024-01-15 12:05:00[/]  web-server api-server[1234]: INFO: startup compl...   |
| [DIM]Showing 1000 most recent -- more available[/]                                  |
+-------------------------------------------------------------------------------------+
```

**`/` filter:** Filters log lines by substring match. Matching lines are
shown; non-matching are hidden. Header shows `/search-text` in amber.
Esc clears filter. The frame title does NOT change (viewport, not table).

**`c` copies:** The current line at the cursor position (the line at the top
of the viewport, or highlighted line if cursor tracking is implemented).

**`w`:** Toggle word wrap.

---

### 2.4 Instance Status Events (viewport view)

AWS-detected issues -- status check failures, scheduled maintenance, hardware
degradation.

**API:** `ec2:DescribeInstanceStatus` (InstanceId, IncludeAllInstances=true)

#### views_reference.yaml

Source struct: `ec2types.InstanceStatus`

```
- AvailabilityZone
- Events[].Code
- Events[].Description
- Events[].InstanceEventId
- Events[].NotAfter
- Events[].NotBefore
- Events[].NotBeforeDeadline
- InstanceId
- InstanceState.Code
- InstanceState.Name
- InstanceStatus.Details[].ImpairedSince
- InstanceStatus.Details[].Name
- InstanceStatus.Details[].Status
- InstanceStatus.Status
- SystemStatus.Details[].ImpairedSince
- SystemStatus.Details[].Name
- SystemStatus.Details[].Status
- SystemStatus.Status
```

#### Wireframe -- Normal (healthy)

Rendered as a detail-style view (key: value pairs), not a table.

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------- instance-status -- i-0abc123def456 (web-server-prod) --------------------+
| Instance State:       [GREEN]running[/]                                             |
|                                                                                     |
| System Status:                                                                      |
|     Status:           [GREEN]ok[/]                                                  |
|     Reachability:     [GREEN]passed[/]                                              |
|                                                                                     |
| Instance Status:                                                                    |
|     Status:           [GREEN]ok[/]                                                  |
|     Reachability:     [GREEN]passed[/]                                              |
|                                                                                     |
| Scheduled Events:     [DIM]None[/]                                                  |
|                                                                                     |
+-------------------------------------------------------------------------------------+
```

#### Wireframe -- With Scheduled Event

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------- instance-status -- i-0abc123def456 (web-server-prod) --------------------+
| Instance State:       [GREEN]running[/]                                             |
|                                                                                     |
| System Status:                                                                      |
|     Status:           [GREEN]ok[/]                                                  |
|     Reachability:     [GREEN]passed[/]                                              |
|                                                                                     |
| Instance Status:                                                                    |
|     Status:           [GREEN]ok[/]                                                  |
|     Reachability:     [GREEN]passed[/]                                              |
|                                                                                     |
| Scheduled Events:                                                                   |
|     [YELLOW]instance-reboot[/]                                                      |
|     Description:      The instance is scheduled for a reboot                        |
|     Not Before:       2024-01-20 00:00:00                                           |
|     Not After:        2024-01-22 00:00:00                                           |
|                                                                                     |
+-------------------------------------------------------------------------------------+
```

#### Wireframe -- Impaired

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------- instance-status -- i-0abc123def456 (web-server-prod) --------------------+
| Instance State:       [GREEN]running[/]                                             |
|                                                                                     |
| System Status:                                                                      |
|     Status:           [RED]impaired[/]                                              |
|     Reachability:     [RED]failed[/]                                                |
|     Impaired Since:   2024-01-15 14:00:00                                           |
|                                                                                     |
| Instance Status:                                                                    |
|     Status:           [GREEN]ok[/]                                                  |
|     Reachability:     [GREEN]passed[/]                                              |
|                                                                                     |
| Scheduled Events:     [DIM]None[/]                                                  |
|                                                                                     |
+-------------------------------------------------------------------------------------+
```

**`c` copies:** Full status summary as text.

**`d`/`y`:** Not applicable -- this is already a detail-style view.

---

### 2.5 VPC Flow Logs (table view)

Network flow records filtered to this instance's ENIs.

**API:**
- `ec2:DescribeNetworkInterfaces` -- get all ENI IDs for this instance
- `ec2:DescribeFlowLogs` -- check if flow logs exist for the VPC/subnet/ENI
- `logs:FilterLogEvents` or `s3:GetObject` -- read flow log records

For instances with multiple ENIs, all ENIs are queried and results are merged
into a single table. The ENI ID is shown as a column so the user can tell
which interface each flow belongs to.

**Time range:** Last 24 hours.

#### views.yaml

```yaml
ec2_flow_logs:
  list:
    Time:
      key: timestamp
      width: 22
    ENI:
      key: eni_id
      width: 14
    Src:
      key: src_addr
      width: 16
    Dst:
      key: dst_addr
      width: 16
    Port:
      key: dst_port
      width: 7
    Proto:
      key: protocol
      width: 6
    Action:
      key: action
      width: 8
```

#### Wireframe -- Normal

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------- flow-logs(156) -- i-0abc123def456 (web-server-prod) ---------------------+
| TIME                   ENI            SRC              DST              PORT  PROTO |
| [SELECTED] 2024-01-15 14:23:05  eni-0abc123    10.0.1.50        10.0.2.100       5432  TCP  [/]|
| [GREEN] 2024-01-15 14:23:04  eni-0abc123    10.0.1.50        10.0.3.200        443  TCP  [/]|
| [RED] 2024-01-15 14:23:04  eni-0abc123    203.0.113.99     10.0.1.50          22  TCP  [/]|
| [GREEN] 2024-01-15 14:22:58  eni-0def456    10.0.2.10        10.0.4.50        8080  TCP  [/]|
|   . . . (152 more)                                                                 |
+------------------------------------------------------------------------------------+
```

Note: ACCEPT rows are green, REJECT rows are red. This maps to the standard
status color system: ACCEPT=running/available=green, REJECT=stopped/failed=red.

The Action column (not shown in narrow terminals due to overflow) determines
row color, just like STATUS determines row color in other table views.

#### Wireframe -- Empty

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------- flow-logs(0) -- i-0abc123def456 (web-server-prod) -----------------------+
|                                                                                     |
|              [DIM]No flow log records in the last 24 hours[/]                       |
|                                                                                     |
+-------------------------------------------------------------------------------------+
```

**`c` copies:** Selected flow record as a tab-separated line.

**`d` (detail):** Opens a detail view of the selected flow record.

---

### 2.6 SSM Command History (table view)

Past Run Command executions on this instance.

**API:**
- `ssm:ListCommands` with `InstanceId` filter
- `ssm:GetCommandInvocation` -- stdout/stderr for selected command (on Enter)

**Pagination:** `NextToken`. Fetches up to 1,000 commands.

#### views.yaml

```yaml
ec2_ssm_commands:
  list:
    Time:
      path: RequestedDateTime
      width: 22
    Command:
      path: DocumentName
      width: 32
    Status:
      path: StatusDetails
      width: 12
    User:
      key: requested_by
      width: 20
  detail:
    - CommandId
    - DocumentName
    - RequestedDateTime
    - StatusDetails
    - Comment
    - OutputS3BucketName
    - OutputS3KeyPrefix
```

#### views_reference.yaml

Source struct: `ssmtypes.Command`

```
- CommandId
- Comment
- CompletedCount
- DeliveryTimedOutCount
- DocumentName
- DocumentVersion
- ErrorCount
- ExpiresAfter
- InstanceIds[]
- MaxConcurrency
- MaxErrors
- OutputS3BucketName
- OutputS3KeyPrefix
- OutputS3Region
- Parameters
- RequestedDateTime
- ServiceRole
- StatusDetails
- TargetCount
- Targets[].Key
- Targets[].Values[]
- TimeoutSeconds
```

#### Wireframe -- Normal

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------- ssm-commands(4) -- i-0abc123def456 (web-server-prod) --------------------+
| TIME                   COMMAND                          STATUS       USER           |
| [SELECTED] 2024-01-15 14:00:00  AWS-RunShellScript               Success      deploy-role  [/]|
| [GREEN] 2024-01-14 09:30:00  AWS-UpdateSSMAgent                Success      ssm-service  [/]|
| [RED] 2024-01-13 16:00:00  AWS-RunPatchBaseline              Failed       patch-role   [/]|
| [YELLOW] 2024-01-13 15:55:00  AWS-RunShellScript               InProgress   deploy-role  [/]|
|                                                                                     |
+-------------------------------------------------------------------------------------+
```

Row colors: Success=green, Failed=red, InProgress=yellow, TimedOut=red,
Cancelled=dim.

#### Wireframe -- Empty

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------- ssm-commands(0) -- i-0abc123def456 (web-server-prod) --------------------+
|                                                                                     |
|              [DIM]No SSM commands found for this instance[/]                        |
|                                                                                     |
+-------------------------------------------------------------------------------------+
```

#### SSM Command Output (viewport view, nested)

Enter on a command opens a scrollable viewport showing stdout and stderr from
`GetCommandInvocation`.

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------- ssm-output -- AWS-RunShellScript (2024-01-15 14:00) ---------------------+
| [YELLOW]STDOUT:[/]                                                                 |
| total 32                                                                            |
| drwxr-xr-x   2 root root  4096 Jan 15 14:00 .                                     |
| drwxr-xr-x  23 root root  4096 Jan 10 09:00 ..                                    |
| -rw-r--r--   1 root root  1234 Jan 15 13:55 config.yaml                           |
| -rwxr-xr-x   1 root root 16384 Jan 15 12:00 api-server                            |
|                                                                                     |
| [YELLOW]STDERR:[/]                                                                 |
| [DIM](empty)[/]                                                                    |
|                                                                                     |
+-------------------------------------------------------------------------------------+
```

**`c` copies:** Full command output (stdout + stderr).

**`w`:** Toggle word wrap.

---

## Key Bindings

All standard key bindings are inherited from design.md. The following are
specific to log views or behave differently in this context:

### New Key Bindings

| Key | Action | Context | Notes |
|-----|--------|---------|-------|
| `l` | Open logs view | EC2 resource list | Pushes LogSourceMenuView |

### Inherited Keys (behavior notes for log views)

| Key | Action | Context | Notes |
|-----|--------|---------|-------|
| `j`/`k` | Move cursor / scroll | All log sub-views | Cursor in tables, scroll in viewports |
| `g`/`G` | Jump to top/bottom | All log sub-views | |
| `Enter` | Open / drill down | Log source menu, CW groups, SSM commands | Context-dependent target |
| `d` | Detail view | CloudTrail, SSM Commands, Flow Logs | Only for table views |
| `y` | YAML view | CloudTrail, SSM Commands | Raw API response |
| `c` | Copy | All sub-views | Context-specific (see each source) |
| `/` | Filter | Table sub-views, viewport sub-views | Substring match |
| `w` | Toggle word wrap | Console Output, CW Log Viewer, SSM Output | Viewport views only |
| `Ctrl+R` | Refresh | All sub-views | Re-fetch from API |
| `Esc` | Back | All sub-views | Pop view stack |
| `?` | Help | All sub-views | Context-sensitive help screen |
| `N`/`S`/`A` | Sort | CloudTrail, Flow Logs, SSM Commands | Table views only |
| `h`/`l` | Scroll columns | CloudTrail, Flow Logs, SSM Commands | Table views only |

---

## Help Screens

Each log sub-view shows a context-sensitive help screen via `?`. The layout
follows the standard 4-column format from design.md section 3.6.

### Help -- Log Source Menu

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------------------------- Help ---------------------------------------------------+
| NAVIGATION            GENERAL              ACTIONS                                  |
|                                                                                     |
| <j>      Down         <ctrl-r> Refresh     <enter>  Open source                    |
| <k>      Up           <?>      Help        <esc>    Back to EC2 list               |
| <g>      Top          <:>      Command                                              |
| <G>      Bottom       <q>      Quit                                                 |
|                                                                                     |
|                    [DIM]Press any key to close[/]                                    |
+-------------------------------------------------------------------------------------+
```

### Help -- Table Sub-Views (CloudTrail, Flow Logs, SSM Commands)

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------------------------- Help ---------------------------------------------------+
| NAVIGATION            GENERAL              ACTIONS              SORT                |
|                                                                                     |
| <j>      Down         <ctrl-r> Refresh     <enter>  Detail      <N>   By name      |
| <k>      Up           <?>      Help        <d>      Detail      <S>   By status    |
| <g>      Top          <:>      Command     <y>      YAML        <A>   By age       |
| <G>      Bottom       </>      Filter      <c>      Copy                            |
| <h/l>    Cols         <q>      Quit        <esc>    Back                            |
| <pgup>   Page up                                                                    |
| <pgdn>   Page down                                                                  |
|                                                                                     |
|                    [DIM]Press any key to close[/]                                    |
+-------------------------------------------------------------------------------------+
```

### Help -- Viewport Sub-Views (Console Output, CW Log Viewer, SSM Output)

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
+---------------------------- Help ---------------------------------------------------+
| NAVIGATION            GENERAL              ACTIONS                                  |
|                                                                                     |
| <j>      Down         <ctrl-r> Refresh     <c>      Copy                           |
| <k>      Up           <?>      Help        <w>      Word wrap                      |
| <g>      Top          </>      Filter      <esc>    Back                            |
| <G>      Bottom       <:>      Command                                              |
| <pgup>   Page up      <q>      Quit                                                 |
| <pgdn>   Page down                                                                  |
|                                                                                     |
|                    [DIM]Press any key to close[/]                                    |
+-------------------------------------------------------------------------------------+
```

---

## Data Limits

To avoid downloading huge volumes of log data:

- **Time range:** Last 24 hours by default for all time-based sources.
- **Max events per source:** 1,000 entries (most recent first). If the limit
  is hit, show a dim footer hint on the last visible line.
- **Pagination:** Fetch the first page only. User scrolls to bottom to trigger
  next page load (lazy pagination), up to the max limit.
- **Console Output:** Already capped at ~64KB by AWS. No additional limit.
- **CloudWatch discovery:** Stop scanning after 30 seconds or 200 log groups.
  Show a hint if the scan was truncated.

These defaults keep initial fetch fast and memory usage bounded. A future
enhancement could make time range and max events configurable.

---

## Graceful Degradation

| Condition | Behavior |
|-----------|----------|
| Missing IAM permissions | Source shows `[RED]Access denied[/]` in the menu; dimmed, not selectable |
| Terminated instance | Console Output + Instance Status show `Unavailable (terminated)`; CloudTrail still works (up to 90 days) |
| No CloudWatch agent | "CloudWatch Logs: Not configured" (dimmed) |
| No flow logs enabled | "VPC Flow Logs: Not configured" (dimmed) |
| No SSM commands | "SSM Command History: No commands found" (dimmed) |
| API timeout | Header flash error; source shows `[RED]Timed out[/]` in menu |
| API throttling | Header flash error; retry on Ctrl+R |

---

## Responsive Behavior

The log source menu is a simple list with no columns -- it works at any width
above the 60-column minimum.

Table sub-views (CloudTrail, Flow Logs, SSM Commands) follow the standard
column overflow strategy from design.md section 8:

| Terminal width | CloudTrail columns shown | Flow Logs columns shown | SSM columns shown |
|----------------|--------------------------|-------------------------|-------------------|
| 60-79 cols     | Time, Event              | Time, Src, Dst          | Time, Command     |
| 80-119 cols    | Time, Event, User        | Time, Src, Dst, Port    | Time, Command, Status |
| 120+ cols      | All 4 columns            | All 7 columns           | All 4 columns     |

Viewport sub-views (Console Output, CW Log Viewer, SSM Output) use word wrap
at any width. Long lines are truncated by default; `w` toggles word wrap.

---

## Implementation Phases

1. **MVP:** Log Source Menu + Console Output + CloudTrail Events + Instance
   Status (zero agent dependency, 3 simple APIs, immediate availability)
2. **Phase 2:** CloudWatch Logs (discovery + group list + log viewer -- the
   most complex piece due to scan logic and the 4-level view stack)
3. **Phase 3:** VPC Flow Logs + SSM Command History (both require additional
   AWS services and have conditional availability)
