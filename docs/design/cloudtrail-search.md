# CloudTrail Search/Debug View Design Spec

Issue: #112
Version: 1.0
Target: a9s v3.25+
Status: Design

---

## 1. Overview

A dedicated CloudTrail investigation view that goes beyond the existing `ct-events`
list. While `ct-events` is a simple paginated resource list (browse recent events),
the search view is purpose-built for the seven investigation archetypes: incident
response, security sweep, access troubleshooting, change tracking, compliance/audit,
cost attribution, and debugging.

### Entry Points

- `:ct-search` command from any view
- `s` key on the CloudTrail Events list (opens search pre-populated with current time window)
- Direct alias: `:search`, `:investigate`

### What Makes This Different from ct-events

| Aspect | ct-events (existing) | ct-search (new) |
|--------|---------------------|-----------------|
| Purpose | Browse recent events | Targeted investigation |
| Filters | Client-side `/` text only | API + client-side structured filters |
| Time window | Whatever API returns | User-selected presets + custom |
| Presets | None | 6 instant investigation patterns |
| Entry | From main menu or `:ct-events` | `:ct-search` or `s` from ct-events |
| Event detail | Standard detail/YAML views | Enhanced detail with cross-resource nav |

---

## 2. Architecture: Three-Panel Flow

The search view is a three-screen flow, all within a single view stack entry:

```
[1] SEARCH FORM         [2] RESULTS LIST        [3] EVENT DETAIL
 (filter inputs)   -->   (table of events)  -->  (full event data)
      Enter               Enter/d                    Esc
       Esc                  Esc                      Esc
  (back to caller)     (back to form)          (back to results)
```

The search form and results share the same frame. The form is shown when there
are no results yet, or when the user presses `f` (re-open filters) from results.

---

## 3. Color Palette (Tokyo Night Dark Extensions)

All colors use the existing palette. New semantic mappings only:

| Element                    | Foreground  | Background | Style  | Notes                       |
|----------------------------|-------------|------------|--------|-----------------------------|
| **Filter chip label**      | `#c0caf5`   | `#24283b`  | --     | Active filter name          |
| **Filter chip value**      | `#7aa2f7`   | `#24283b`  | Bold   | Active filter value         |
| **Filter chip inactive**   | `#565f89`   | --         | Dim    | Empty filter placeholder    |
| **Time window badge**      | `#1a1b26`   | `#7aa2f7`  | Bold   | Selected time range         |
| **Toggle ON**              | `#9ece6a`   | --         | Bold   | Write-only, Error-only      |
| **Toggle OFF**             | `#565f89`   | --         | Dim    | Inactive toggles            |
| **Preset name**            | `#e0af68`   | --         | Bold   | Preset pattern labels       |
| **Preset description**     | `#565f89`   | --         | --     | Preset description text     |
| **Error event row**        | `#f7768e`   | --         | --     | Rows with errorCode set     |
| **Write event row**        | `#c0caf5`   | --         | --     | Normal write events         |
| **Read event row**         | `#565f89`   | --         | Dim    | ReadOnly=true events        |
| **Root event row**         | `#f7768e`   | --         | Bold   | userIdentity.type=Root      |
| **Cross-ref link**         | `#7aa2f7`   | --         | Underline | Navigable resource ARN   |
| **Section divider**        | `#414868`   | --         | --     | Thin horizontal rules       |
| **Data event warning**     | `#e0af68`   | --         | Italic | "Data events require trail" |
| **Result count**           | `#9ece6a`   | --         | Bold   | "247 events" in results     |
| **API filter badge**       | `#bb9af7`   | --         | Bold   | "API" tag on server filter  |
| **Client filter badge**    | `#565f89`   | --         | --     | "local" tag on client filter|

---

## 4. Borders and Components

| Component           | Border                          | Bubbles Component      |
|---------------------|---------------------------------|------------------------|
| Outer frame         | Manual box chars `#414868`          | Custom (same as all views) |
| Filter input fields | None (inline in form)           | `bubbles/textinput`    |
| Results table       | None (inside frame)             | Custom renderer (same as resource list) |
| Event detail scroll | None (inside frame)             | `bubbles/viewport`     |
| Loading spinner     | None                            | `bubbles/spinner`      |
| Time range selector | None (inline chip row)          | Custom                 |
| Preset list         | None (inside frame)             | Custom                 |

---

## 5. Screen 1: Search Form

### 5.1 Layout

The search form occupies the full frame. It has three sections stacked vertically:

1. **Time range row** -- horizontal chip selector
2. **Filter fields** -- vertical list of labeled text inputs
3. **Toggles + presets** -- toggle switches and quick-access presets

### 5.2 Wireframe: Empty Search Form (Initial State)

```
 a9s v0.5.0  prod:us-east-1                                            ? for help
┌──────────────────────── ct-search ───────────────────────────────────────────────┐
│                                                                                  │
│  TIME RANGE                                                                      │
│  [15m] [ 1h ] [ 4h ] [24h ] [ 7d ] [30d ] [custom]                              │
│         ^^^^                                                                     │
│  FILTERS                                                       API  local        │
│  Event Name:    ___________________________________            API                │
│  Username:      ___________________________________            API                │
│  Event Source:  ___________________________________            API                │
│  Resource ARN:  ___________________________________            API                │
│  Access Key:    ___________________________________            API                │
│  Error Code:    ___________________________________                 local        │
│  Source IP:     ___________________________________                 local        │
│                                                                                  │
│  TOGGLES                                                                         │
│  [x] Write events only    [ ] Error events only                                  │
│                                                                                  │
│  PRESETS                                                                         │
│  [1] Recent Changes        write events, last 2h                                 │
│  [2] Error Investigation   error events, last 1h                                 │
│  [3] Console Logins        ConsoleLogin events, last 7d                          │
│  [4] IAM Changes           iam.amazonaws.com, write, last 24h                    │
│  [5] Root Activity         Root identity, last 30d                               │
│  [6] Dangerous Ops         write events + dangerous op filter, last 4h            │
│                                                                                  │
│  Data events (S3 GetObject, Lambda Invoke) require a configured trail            │
│                                                                                  │
│                        Enter: search  Esc: cancel                                │
└──────────────────────────────────────────────────────────────────────────────────┘
```

### 5.3 Wireframe: Form with Active Filters

```
 a9s v0.5.0  prod:us-east-1                                            ? for help
┌──────────────────────── ct-search ───────────────────────────────────────────────┐
│                                                                                  │
│  TIME RANGE                                                                      │
│  [15m] [ 1h ] [ 4h ] [24h ] [ 7d ] [30d ] [custom]                              │
│                                       ^^^^                                       │
│  FILTERS                                                       API  local        │
│  Event Name:    ___________________________________            API                │
│  Username:      deploy-bot_____________________________        API                │
│  Event Source:  ___________________________________            API                │
│  Resource ARN:  ___________________________________            API                │
│  Access Key:    ___________________________________            API                │
│  Error Code:    ___________________________________                 local        │
│  Source IP:     ___________________________________                 local        │
│                                                                                  │
│  TOGGLES                                                                         │
│  [ ] Write events only    [ ] Error events only                                  │
│                                                                                  │
│  The most selective API filter will be sent to CloudTrail.                        │
│  All other filters are applied locally on fetched results.                        │
│                                                                                  │
│                        Enter: search  Esc: cancel                                │
└──────────────────────────────────────────────────────────────────────────────────┘
```

### 5.4 Time Range Selector

Horizontal row of chip buttons. One is always selected (highlighted with
`#7aa2f7` background). Arrow keys or `h`/`l` to cycle. Default: `1h`.

| Preset | Duration | API Parameters |
|--------|----------|----------------|
| 15m | 15 minutes | `StartTime = now - 15m` |
| 1h | 1 hour (default) | `StartTime = now - 1h` |
| 4h | 4 hours | `StartTime = now - 4h` |
| 24h | 24 hours | `StartTime = now - 24h` |
| 7d | 7 days | `StartTime = now - 7d` |
| 30d | 30 days | `StartTime = now - 30d` |
| custom | User input | Prompts for start/end time strings |

### 5.5 Filter Fields

Each filter field is a `bubbles/textinput` with a fixed-width label. The `Tab` key
cycles focus between fields. The right-side badge indicates whether the filter maps
to a `LookupAttribute` (API-side, purple badge) or is applied client-side (dim badge).

Since CloudTrail only supports ONE LookupAttribute per API call, the implementation
must pick the most selective API filter and apply the rest client-side. The form
shows `API` and `local` badges but does not burden the user with choosing -- the
engine auto-selects.

**API filter selection heuristic** (in priority order):
1. Access Key ID (most specific -- single credential)
2. Resource ARN (specific resource)
3. Event Name (specific API call)
4. Username (specific principal)
5. Event Source (specific service)

If no API-eligible filter has a value, the search uses time range only.

### 5.6 Toggles

Two checkboxes, toggled with `Space` when focused or `W` (write-only) / `E`
(error-only) as shortcuts when a non-text-input element is focused (toggle,
time range, or preset row). These shortcuts are uppercase to avoid conflicts
with the `e` (Events child-view) and `w` (ToggleWrap) global bindings.

| Toggle | Default | Effect |
|--------|---------|--------|
| Write events only | ON | Adds `ReadOnly=false` to API call |
| Error events only | OFF | Client-side: show only events with errorCode |

### 5.7 Presets

Six numbered presets that auto-fill the form. Pressing `1`-`6` populates filters,
time range, and toggles, then immediately executes the search.

| # | Name | Time | Filters | Toggles |
|---|------|------|---------|---------|
| 1 | Recent Changes | 2h | -- | Write only ON |
| 2 | Error Investigation | 1h | -- | Error only ON |
| 3 | Console Logins | 7d | Event Name = ConsoleLogin | Both OFF |
| 4 | IAM Changes | 24h | Event Source = iam.amazonaws.com | Write only ON |
| 5 | Root Activity | 30d | Client-side filter: identity type = Root | Both OFF |
| 6 | Dangerous Ops | 4h | Event Name = (empty -- see note below) | Write only ON |

All presets execute immediately (no user input needed).
Preset 5 executes immediately (Root is a client-side filter).

**Preset 6 implementation note**: CloudTrail's `LookupEvents` API only supports
exact `EventName` matching -- wildcards like `Delete*` are not supported. Preset 6
uses time range (4h) + write-only (`ReadOnly=false`) as the API query parameters,
with the Event Name field left empty. ALL event name prefix matching
(Delete\*/Terminate\*/Remove\*/Revoke\*/Deregister\*/Detach\*/Disable\*) is performed
client-side on the fetched results. The preset description in the UI should read
"write events + dangerous op filter, last 4h" to clarify this is a client-side filter.

---

## 6. Screen 2: Results List

### 6.1 Layout

Standard a9s resource list table inside the same frame. The frame title changes
to show the search context.

### 6.2 Wireframe: Results (Write Events, Last 1 Hour)

```
 a9s v0.5.0  prod:us-east-1                                            ? for help
┌────── ct-search(47) — write events, last 1h ────────────────────────────────────┐
│ TIME                  EVENT NAME                USER              SOURCE         │
│ 2026-03-29 14:58:31   DeleteBucket              admin             s3.amazonaws.… │
│ 2026-03-29 14:57:12   PutBucketPolicy           ci-deploy         s3.amazonaws.… │
│ 2026-03-29 14:55:03   AuthorizeSecurityGrou…    platform-bot      ec2.amazonaws… │
│ 2026-03-29 14:52:44   RunInstances              ci-deploy         ec2.amazonaws… │
│ 2026-03-29 14:50:19   UpdateFunctionConfigu…    ci-deploy [svc]   lambda.amazon… │
│ 2026-03-29 14:47:33   CreateAccessKey            admin             iam.amazonaws… │
│ 2026-03-29 14:45:01   AssumeRole                ci-deploy         sts.amazonaws… │
│ 2026-03-29 14:42:18   ConsoleLogin              admin             signin.amazon… │
│   · · · (39 more)                                                                │
│── M: load more ──                                                                │
└──────────────────────────────────────────────────────────────────────────────────┘
```

### 6.3 Wireframe: Results with Active Filter Chips

When the user has set multiple filters, active filters are shown as a chip bar
below the column headers.

```
 a9s v0.5.0  prod:us-east-1                                            ? for help
┌── ct-search(12) — deploy-bot, write events, last 7d ───────────────────────────┐
│ TIME                  EVENT NAME                ERROR       SOURCE               │
│ 2026-03-29 14:57:12   PutBucketPolicy                       s3.amazonaws.com    │
│ 2026-03-29 14:50:19   UpdateFunctionConfigu…                 lambda.amazonaws.…  │
│ 2026-03-29 14:45:01   AssumeRole                             sts.amazonaws.com   │
│ 2026-03-28 22:10:45   UpdateFunctionCode                     lambda.amazonaws.…  │
│ 2026-03-28 20:03:11   CreateDeployment                       codedeploy.amazon…  │
│ 2026-03-28 18:55:22   RunInstances                           ec2.amazonaws.com   │
│ 2026-03-28 15:41:03   TerminateInstances                     ec2.amazonaws.com   │
│ 2026-03-27 09:12:44   PutBucketPolicy                        s3.amazonaws.com    │
│ 2026-03-27 08:30:15   CreateFunction                         lambda.amazonaws.…  │
│   · · · (3 more)                                                                 │
│── M: load more ──                                                                │
└──────────────────────────────────────────────────────────────────────────────────┘
```

### 6.4 Wireframe: Results with Error Events Highlighted

```
 a9s v0.5.0  prod:us-east-1                                            ? for help
┌────── ct-search(23) — errors only, last 1h ─────────────────────────────────────┐
│ TIME                  EVENT NAME                ERROR             USER           │
│ 2026-03-29 14:58:31   PutBucketPolicy           AccessDenied      lambda-role   │
│ 2026-03-29 14:55:03   AssumeRole                AccessDenied      ci-deploy     │
│ 2026-03-29 14:52:44   GetSecretValue            AccessDenied      app-svc-role  │
│ 2026-03-29 14:50:19   DescribeInstances         ThrottlingExcep…  monitoring    │
│ 2026-03-29 14:47:33   ListBuckets               AccessDenied      intern-role   │
│   · · · (18 more)                                                                │
│── M: load more ──                                                                │
└──────────────────────────────────────────────────────────────────────────────────┘
```

Row coloring in error-only mode: ALL rows are red (`#f7768e`) since they all have errors.

### 6.4.1 Service-Triggered Action Annotation

When an event's `invokedBy` field is an AWS service (e.g., `elasticmapreduce.amazonaws.com`,
`cloudformation.amazonaws.com`), a dim `[svc]` annotation is shown next to the username
in the USER column. This is a visual-only indicator -- not a filter field.

Example in results: `ci-deploy [svc]` where `[svc]` is rendered in dim (`#565f89`).

### 6.5 Wireframe: Results with Root Activity

```
 a9s v0.5.0  prod:us-east-1                                            ? for help
┌── ct-search(3) — root activity, last 30d ───────────────────────────────────────┐
│ TIME                  EVENT NAME                SOURCE             SOURCE IP     │
│ 2026-03-25 03:14:22   ConsoleLogin              signin.amazonaws…  198.51.100.1  │
│ 2026-03-18 11:02:05   CreateAccessKey            iam.amazonaws.com  198.51.100.1  │
│ 2026-03-10 22:45:33   ConsoleLogin              signin.amazonaws…  203.0.113.50  │
│                                                                                  │
│                                                                                  │
│                                                                                  │
│                                                                                  │
└──────────────────────────────────────────────────────────────────────────────────┘
```

Root events are rendered in bold red (`#f7768e` bold) to draw immediate attention.

### 6.5.1 Wireframe: Results (Console Logins, Last 7 Days)

```
 a9s v0.5.0  prod:us-east-1                                            ? for help
┌── ct-search(12) — ConsoleLogin, last 7d ────────────────────────────────────────┐
│ TIME                  EVENT NAME          USER                  SOURCE IP    MFA │
│ 2026-03-29 09:15:42   ConsoleLogin        admin                 198.51.100.1 Yes │
│ 2026-03-28 22:03:17   ConsoleLogin        dev-user              10.0.1.42    Yes │
│ 2026-03-28 14:45:33   ConsoleLogin        ci-deploy             203.0.113.50 No  │
│ 2026-03-27 11:20:05   ConsoleLogin        intern-role           192.0.2.100  No  │
│ 2026-03-25 03:14:22   ConsoleLogin        Root                  198.51.100.1 Yes │
│ 2026-03-24 16:30:11   ConsoleLogin        dev-user              10.0.1.42    Yes │
│ 2026-03-23 08:55:44   ConsoleLogin        platform-bot          10.0.1.42    Yes │
│   · · · (5 more)                                                                 │
│── M: load more ──                                                                │
└──────────────────────────────────────────────────────────────────────────────────┘
```

Console Login results include an MFA column. Row coloring: Root login rows are
bold red (`#f7768e` bold), MFA=No rows are amber (`#e0af68` italic) to flag
logins without multi-factor authentication, all other rows use standard coloring.

### 6.6 Column Configuration

The results table adapts its columns based on which filters are active:

| Context | Columns shown |
|---------|---------------|
| Default (write events) | TIME, EVENT NAME, USER, SOURCE |
| Error investigation | TIME, EVENT NAME, ERROR, USER |
| Security sweep (by user) | TIME, EVENT NAME, SOURCE, SOURCE IP |
| Root activity | TIME, EVENT NAME, SOURCE, SOURCE IP |
| Console Logins | TIME, EVENT NAME, USER, SOURCE IP, MFA |
| Resource-specific | TIME, EVENT NAME, USER, ERROR |
| All filters off | TIME, EVENT NAME, USER, SOURCE, READ ONLY |

The TIME column is always first and always visible.

### 6.7 Row Coloring Rules

Row coloring in the results list follows these precedence rules:

| Condition | Color | Hex | Priority |
|-----------|-------|-----|----------|
| Selected row | Blue bg / dark fg | `#7aa2f7` bg, `#1a1b26` fg | 1 (highest) |
| Root identity event | Red bold | `#f7768e` bold | 2 |
| Error event (any errorCode) | Red | `#f7768e` | 3 |
| Delete/Terminate event | Red | `#f7768e` | 4 |
| MFA=No (ConsoleLogin only) | Amber italic | `#e0af68` italic | 5 |
| Write event (normal) | Plain white | `#c0caf5` | 6 |
| Read event | Dim | `#565f89` | 7 (lowest) |

### 6.8 Frame Title Format

The frame title encodes the search context concisely:

```
ct-search(count) -- summary
```

Summary is built from active filters, truncated to fit:
- `write events, last 1h` -- default search
- `deploy-bot, write events, last 7d` -- username filter
- `errors only, last 1h` -- error-only toggle
- `root activity, last 30d` -- root preset
- `iam.amazonaws.com, write, last 24h` -- service filter

When paginated: `ct-search(47+) -- write events, last 1h`

---

## 7. Screen 3: Event Detail

### 7.1 Layout

Replaces the results table in the frame. Scrollable viewport (`bubbles/viewport`).
Five sections with copy targets marked.

### 7.2 Wireframe: Event Detail (Write Event, No Error)

```
 a9s v0.5.0  prod:us-east-1                                            ? for help
┌──────── ct-search — RunInstances ───────────────────────────────────────────────┐
│                                                                                  │
│ Event:                                                                           │
│  EventName:           RunInstances                                               │
│  EventSource:         ec2.amazonaws.com                                          │
│  EventTime:           2026-03-29 14:52:44                                        │
│  AwsRegion:           us-east-1                                                  │
│  ReadOnly:            false                                                      │
│                                                                                  │
│ Identity:                                                                        │
│  Type:                AssumedRole                                                │
│  PrincipalId:         AROA3XFRBF23COEXAMPLE:ci-deploy                           │
│  Arn:                 arn:aws:sts::123456789012:assumed-role/ci-deploy-role/ci-…  │
│  AccessKeyId:         ASIA3XFRBF23EXAMPLE                                        │
│  SourceIpAddress:     10.0.1.42                                                  │
│  UserAgent:           aws-cli/2.15.0 Python/3.11.6                               │
│  SharedEventID:       a1b2c3d4-1234-5678-abcd-example12345                       │
│                                                                                  │
│ Resources:                                                                       │
│  [1] AWS::EC2::Instance   i-0abc123def456789a                                    │
│                                                                                  │
│ Request Parameters:                                                              │
│  instanceType:        m5.xlarge                                                  │
│  imageId:             ami-0abcdef01234567                                         │
│  minCount:            2                                                          │
│  maxCount:            2                                                          │
│  subnetId:            subnet-0123456789abcde                                     │
│                                                                                  │
│ Response:                                                                        │
│  instancesSet:        {items: [{instanceId: i-0abc123def456789a, ...}]}          │
│                                                                                  │
└──────────────────────────────────────────────────────────────────────────────────┘
```

### 7.3 Wireframe: Event Detail (AccessDenied Error)

```
 a9s v0.5.0  prod:us-east-1                                            ? for help
┌──────── ct-search — PutBucketPolicy ────────────────────────────────────────────┐
│                                                                                  │
│ Event:                                                                           │
│  EventName:           PutBucketPolicy                                            │
│  EventSource:         s3.amazonaws.com                                           │
│  EventTime:           2026-03-29 14:58:31                                        │
│  AwsRegion:           us-east-1                                                  │
│  ReadOnly:            false                                                      │
│                                                                                  │
│ Identity:                                                                        │
│  Type:                AssumedRole                                                │
│  Arn:                 arn:aws:sts::123456789012:assumed-role/lambda-role/funct…   │
│  SourceIpAddress:     10.0.2.55                                                  │
│  UserAgent:           aws-sdk-python/1.34.0                                      │
│                                                                                  │
│ Error:                                                                           │
│  ErrorCode:           AccessDenied                                               │
│  ErrorMessage:        User: arn:aws:sts::123456789012:assumed-role/lambda-rol…   │
│                                                                                  │
│ Resources:                                                                       │
│  [1] AWS::S3::Bucket     prod-data-bucket                                        │
│                                                                                  │
│ Request Parameters:                                                              │
│  bucketName:          prod-data-bucket                                           │
│  bucketPolicy:        {"Version":"2012-10-17","Statement":[...]}                 │
│                                                                                  │
└──────────────────────────────────────────────────────────────────────────────────┘
```

Error section: `ErrorCode` and `ErrorMessage` are rendered in RED (`#f7768e`).
The section header "Error:" is also red bold.

### 7.4 Wireframe: Event Detail (Root ConsoleLogin)

```
 a9s v0.5.0  prod:us-east-1                                            ? for help
┌──────── ct-search — ConsoleLogin ───────────────────────────────────────────────┐
│                                                                                  │
│ Event:                                                                           │
│  EventName:           ConsoleLogin                                               │
│  EventSource:         signin.amazonaws.com                                       │
│  EventTime:           2026-03-25 03:14:22                                        │
│  AwsRegion:           us-east-1                                                  │
│  ReadOnly:            false                                                      │
│                                                                                  │
│ Identity:                                                                        │
│  Type:                Root                                                       │
│  PrincipalId:         123456789012                                               │
│  Arn:                 arn:aws:iam::123456789012:root                              │
│  SourceIpAddress:     198.51.100.1                                               │
│  UserAgent:           Mozilla/5.0 (Macintosh; ...)                               │
│                                                                                  │
│ Additional:                                                                      │
│  MFAUsed:             Yes                                                        │
│  LoginTo:             https://console.aws.amazon.com/console/home                │
│                                                                                  │
│ No resources referenced                                                          │
│                                                                                  │
└──────────────────────────────────────────────────────────────────────────────────┘
```

Root identity: the "Type: Root" value is rendered in RED BOLD (`#f7768e` bold).

### 7.5 Section Rendering

| Section | Header Color | Key Color | Value Color | Notes |
|---------|-------------|-----------|-------------|-------|
| Event | `#e0af68` bold | `#7aa2f7` | `#c0caf5` | Standard detail style |
| Identity | `#e0af68` bold | `#7aa2f7` | `#c0caf5` | Type=Root is red bold; SourceAccount shown with "(cross-account)" annotation when ARN account differs from active profile |
| Error | `#f7768e` bold | `#7aa2f7` | `#f7768e` | Entire section is red-tinted |
| Resources | `#e0af68` bold | `#7aa2f7` | `#c0caf5` | Navigable entries are underlined blue |
| Request Parameters | `#e0af68` bold | `#7aa2f7` | `#c0caf5` | Rendered from parsed JSON |
| Response | `#e0af68` bold | `#7aa2f7` | `#c0caf5` | Rendered from parsed JSON |
| Additional | `#e0af68` bold | `#7aa2f7` | `#c0caf5` | Extra fields (MFA, login URL, SharedEventID) |

### 7.6 Cross-Resource Navigation

When an event references a resource that a9s knows about, the resource entry in
the Resources section becomes navigable. The resource ARN/ID is rendered with
underlined blue text (`#7aa2f7` underline).

Pressing `Enter` on a navigable resource opens that resource's detail view in a9s.
The view stack becomes:

```
ct-search form --> ct-search results --> event detail --> [resource detail]
```

Supported cross-resource types:

| CloudTrail ResourceType | a9s Resource | Navigation Target |
|------------------------|-------------|-------------------|
| `AWS::EC2::Instance` | ec2 | EC2 detail view |
| `AWS::S3::Bucket` | s3 | S3 detail view |
| `AWS::RDS::DBInstance` | dbi | RDS detail view |
| `AWS::Lambda::Function` | lambda | Lambda detail view |
| `AWS::EC2::SecurityGroup` | sg | SG detail view |
| `AWS::IAM::Role` | role | IAM Role detail view |
| `AWS::IAM::Policy` | policy | IAM Policy detail view |
| `AWS::ECS::Service` | ecs-svc | ECS Service detail view |
| `AWS::EKS::Cluster` | eks | EKS detail view |
| `AWS::DynamoDB::Table` | ddb | DynamoDB detail view |
| `AWS::ElasticLoadBalancingV2::LoadBalancer` | elb | ELB detail view |
| `AWS::ElasticLoadBalancingV2::TargetGroup` | tg | Target Group detail view |
| `AWS::SecretsManager::Secret` | secrets | Secrets Manager detail view |
| `AWS::SQS::Queue` | sqs | SQS detail view |
| `AWS::SNS::Topic` | sns | SNS detail view |
| `AWS::SSM::Parameter` | ssm | SSM Parameter detail view |
| `AWS::CloudFormation::Stack` | cfn | CloudFormation detail view |

Resources without a matching a9s type are shown as plain text (no underline,
not navigable).

---

## 8. Key Bindings

### 8.1 Search Form (Screen 1)

| Key | Action | Notes |
|-----|--------|-------|
| `Tab` / `Shift+Tab` | Cycle focus between fields | Wraps around |
| `h` / `l` | Cycle time range selection | When time range row is focused |
| `Space` | Toggle checkbox | When toggle is focused |
| `W` | Toggle write-only | Shortcut from non-text-input focus (toggle, time range, preset) |
| `E` | Toggle error-only | Shortcut from non-text-input focus (toggle, time range, preset) |
| `1`-`6` | Apply preset and search | Immediate execution |
| `Enter` | Execute search | Validates at least time range is set |
| `Esc` | Cancel / go back | Returns to caller view |
| `?` | Show help | Standard help overlay |

### 8.2 Results List (Screen 2)

| Key | Action | Notes |
|-----|--------|-------|
| `j` / `k` / arrows | Navigate results | Standard list navigation |
| `g` / `G` | Jump to top/bottom | Standard |
| `Enter` / `d` | Open event detail | Drill into selected event |
| `y` | Open YAML view | Full CloudTrail event JSON as YAML |
| `c` | Copy event ID | Copies EventId to clipboard |
| `C` | Copy full JSON | Copies entire CloudTrailEvent JSON. **Note**: `CopyFull` binding must be added to `keys.Map` before implementation. |
| `Y` | Copy all results JSON | Copies all results as JSON array; flash "Copied 47 events as JSON". **Note**: `CopyAll` binding must be added to `keys.Map` before implementation. |
| `f` | Re-open search form | Edit filters and re-search |
| `/` | Filter results | Client-side text filter on loaded results |
| `M` | Load more | Paginate (same as standard resource list) |
| `N` | Sort by event name | Standard sort toggle |
| `A` | Sort by time | Standard sort toggle |
| `Esc` | Back to search form | Preserves form state |
| `?` | Show help | Standard help overlay |

### 8.3 Event Detail (Screen 3)

| Key | Action | Notes |
|-----|--------|-------|
| `j` / `k` / arrows | Scroll | Standard viewport navigation |
| `g` / `G` | Jump to top/bottom | Standard |
| `Enter` | Navigate to resource | Only on navigable resource entries |
| `c` | Copy principal ARN | Copies the identity ARN |
| `C` | Copy full JSON | Copies entire event JSON |
| `R` | Copy resource ARN | Copies first resource ARN; uppercase avoids `r` Resources child-view conflict |
| `E` | Copy error message | Copies errorCode + errorMessage; uppercase avoids `e` Events child-view conflict |
| `y` | Open YAML view | Full event data as YAML |
| `w` | Toggle word wrap | For long values |
| `Esc` | Back to results | Standard |
| `?` | Show help | Standard help overlay |

---

## 9. Help Screens

### 9.1 Search Form Help

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ SEARCH FORM           GENERAL              NAVIGATION           PRESETS         │
│                                                                                 │
│ <enter> Search        <ctrl-r> Refresh     <tab>     Next field  <1> Recent    │
│ <esc>   Cancel        <?>      Help        <s-tab>   Prev field  <2> Errors    │
│ <W>     Write toggle  <q>      Quit        <h/l>     Time range  <3> Logins    │
│ <E>     Error toggle                       <space>   Toggle      <4> IAM       │
│                                                                   <5> Root     │
│                                                                   <6> Danger   │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 9.2 Results List Help

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ RESULTS               GENERAL              NAVIGATION           COPY            │
│                                                                                 │
│ <enter> Detail        <ctrl-r> Re-search   <j>       Down       <c> Event ID   │
│ <esc>   Back to form  </>      Filter      <k>       Up         <C> Full JSON  │
│ <f>     Edit filters  <:>      Command     <g>       Top        <Y> All JSON   │
│ <y>     YAML          <?>      Help        <G>       Bottom                    │
│ <M>     Load more                          <N>       Sort Name                 │
│                                            <A>       Sort Time                 │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 9.3 Event Detail Help

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ EVENT DETAIL          GENERAL              NAVIGATION           COPY            │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <c> Principal  │
│ <enter> Navigate res  <?>      Help        <k>       Up         <C> Full JSON  │
│ <y>     YAML                               <g>       Top        <R> Resource   │
│ <w>     Word wrap                          <G>       Bottom     <E> Error msg  │
│                                            <pgup/dn> Page                      │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## 10. State Transitions

### 10.1 Msg Types

| Msg Type | Trigger | Effect |
|----------|---------|--------|
| `CTSearchFormSubmitMsg` | Enter on form | Start API call, switch to loading state |
| `CTSearchResultsMsg` | API response | Populate results, switch to results view |
| `CTSearchPartialResultsMsg` | Esc during loading | Stop pagination, show matches so far |
| `CTSearchErrorMsg` | API failure | Flash error, stay on form |
| `CTSearchLoadMoreMsg` | M key in results | Fetch next page with same filters |
| `CTSearchPresetMsg{N}` | Number key 1-6 | Auto-fill form and submit |
| `NavigateToResourceMsg` | Enter on navigable resource | Push resource detail onto view stack |
| `KeyMsg(f)` | f in results | Switch back to form (preserving filters) |
| `KeyMsg(esc)` | Esc in results | Switch back to form |
| `KeyMsg(esc)` | Esc in loading | Stop search, show partial results |
| `KeyMsg(esc)` | Esc in form | Pop ct-search from view stack |

### 10.2 State Machine

```
                    ┌─────────────────┐
                    │  SEARCH FORM    │ (initial)
                    │  filters empty  │
                    └────────┬────────┘
                             │ Enter / preset key
                             v
                    ┌─────────────────┐
                    │    LOADING      │ spinner + live counter
                    │ "Loaded N, M   │
                    │  match filters" │
                    └───┬─────────┬───┘
                        │         │
          Esc (partial) │         │ CTSearchResultsMsg (complete)
                        v         v
              ┌──────────────────────────┐
              │     RESULTS LIST         │
              │  ct-search(N) -- summary │
              └──┬────────────────┬──────┘
                 │                │
            f/Esc│           Enter/d
                 v                v
        ┌────────────┐   ┌────────────────┐
        │ SEARCH FORM│   │ EVENT DETAIL   │
        │ (preserved)│   │ scrollable     │
        └────────────┘   └───────┬────────┘
                                 │ Enter on
                                 │ navigable resource
                                 v
                         ┌────────────────┐
                         │ RESOURCE DETAIL│
                         │ (a9s standard) │
                         └────────────────┘
```

### 10.3 Loading State

```
 a9s v0.5.0  prod:us-east-1                                            ? for help
┌──────────────────── ct-search — Searching... ───────────────────────────────────┐
│                                                                                  │
│                                                                                  │
│            ⠿ Searching CloudTrail events...                                      │
│              Loaded 150 events, 3 match filters                                  │
│                                                                                  │
│              write events, last 1h                                               │
│              API filter: Username = deploy-bot                                   │
│              Client filters: Source IP                                            │
│                                                                                  │
│                             Esc: stop and show matches                           │
│                                                                                  │
└──────────────────────────────────────────────────────────────────────────────────┘
```

The loading screen shows:
- A live counter of total events loaded and how many match client-side filters
- Which filter was sent to the API and which filters will be applied client-side
- An `Esc` hint that lets the user stop pagination early and view partial results

---

## 11. Layout Composition

### 11.1 Overall Structure

Same as all a9s views -- `JoinVertical`:

```go
lipgloss.JoinVertical(lipgloss.Left,
    header,    // 1 line, standard a9s header
    frameBox,  // manual border with centered title
)
```

### 11.2 Search Form Interior

```go
lipgloss.JoinVertical(lipgloss.Left,
    timeRangeRow,     // horizontal chip selector
    filterSection,    // vertical list of text inputs
    toggleRow,        // two checkboxes
    presetSection,    // numbered preset list
    dataEventWarning, // dim italic note
    actionHints,      // "Enter: search  Esc: cancel"
)
```

### 11.3 Results Interior

Same as standard resource list: column headers + data rows + optional load-more hint.

### 11.4 Event Detail Interior

Same as standard detail view: section headers + key-value pairs in a viewport.

---

## 12. Responsive Behavior

### 12.1 Width Breakpoints

| Terminal Width | Form Behavior | Results Behavior |
|---------------|---------------|-----------------|
| < 60 cols | "Terminal too narrow" error | Same |
| 60-79 cols | Stacked layout: labels above inputs | TIME, EVENT NAME, USER only |
| 80-119 cols | Side-by-side labels + inputs | TIME, EVENT NAME, USER, SOURCE |
| 120+ cols | Full layout with badges | All columns visible |

### 12.2 Height Breakpoints

| Terminal Height | Behavior |
|-----------------|----------|
| < 15 lines | Form truncated: only time range + 2 filter fields visible |
| 15-24 lines | Form shows all filters but no presets (presets via number keys still work) |
| 25+ lines | Full form with all sections visible |

### 12.3 Search Form on Narrow Terminal (60-79)

```
┌──── ct-search ──────────────────────────────────┐
│                                                  │
│ TIME RANGE                                       │
│ [15m] [1h] [4h] [24h] [7d] [30d]                │
│                                                  │
│ Event Name:                                      │
│ ____________________________________________     │
│ Username:                                        │
│ ____________________________________________     │
│ Event Source:                                    │
│ ____________________________________________     │
│                                                  │
│ [x] Write only   [ ] Error only                  │
│                                                  │
│         Enter: search  Esc: cancel               │
└──────────────────────────────────────────────────┘
```

Labels move above inputs. API/local badges are hidden. Presets are hidden
(still accessible via `1`-`6` keys).

---

## 13. API Interaction Design

### 13.1 Filter-to-API Mapping

```
User fills form:
  Event Name = "RunInstances"
  Username = "ci-deploy"
  Time Range = 1h
  Write Only = ON

Engine selects API filter (priority: most specific wins):
  API: LookupAttribute = { EventName: "RunInstances" }
  Client-side: filter by Username == "ci-deploy"
  API: StartTime = now - 1h, ReadOnly = "false"

CloudTrail API call:
  LookupEvents(
    LookupAttributes: [{Key: EventName, Value: RunInstances}],
    StartTime: <1h ago>,
    EndTime: <now>,
    MaxResults: 50,
  )

Response: 50 events where EventName == "RunInstances"
Client filter: keep only where Username contains "ci-deploy"
Display: filtered results
```

### 13.2 Pagination

Same as standard paginated resource list:
- Initial search fetches page 1 (50 events)
- `M` key fetches next page with same filters + NextToken
- `+` suffix in title indicates more pages available
- Client-side filters re-applied on each page load
- `Ctrl+R` re-executes the search from scratch (page 1)

### 13.3 Rate Limiting

CloudTrail LookupEvents has a 2 TPS limit. The implementation must:
- Respect this limit when paginating (no rapid M-key presses should stack calls)
- Show loading state between pages
- The existing `loadingMore` guard handles this (M is a no-op while loading)

---

## 14. Demo Mode Fixtures

Demo mode must provide realistic fixture data covering all investigation archetypes:

| Fixture | Type | Fields |
|---------|------|--------|
| Successful write event | RunInstances | ci-deploy, ec2, instance resource |
| AccessDenied error | PutBucketPolicy | lambda-role, s3, error fields |
| Root console login | ConsoleLogin | Root identity, source IP, MFA info |
| AssumeRole chain | AssumeRole | Federated user, STS, role ARN |
| Delete operation | DeleteBucket | admin, s3, bucket resource |
| Multi-resource event | RunInstances | ci-deploy, ec2, 3 instance resources |
| Throttling error | DescribeInstances | monitoring role, ThrottlingException |
| IAM mutation | AttachRolePolicy | admin, iam, policy + role resources |
| Security concern | CreateAccessKey | unknown user, iam, suspicious source IP |
| Read event (for toggle) | DescribeInstances | monitoring, ec2, ReadOnly=true |

---

## 15. Implementation Notes

### 15.1 View Registration

The ct-search view is NOT a standard resource type. It does not appear in the main
menu resource list. It is a special view accessible only via command (`:ct-search`)
or keybinding (`s` from ct-events list).

This is similar to how the help view and profile selector are special views that
don't map to a ResourceTypeDef.

### 15.2 New Key on ct-events

Add `s` key to the CloudTrail Events resource list that opens ct-search. This
requires adding a child-view-like keybinding to the `ct-events` ResourceTypeDef,
but since ct-search is not a child view (it's a separate view type), it should be
handled as a special navigation message.

**Note on `s` key conflict**: The `s` key is normally reserved for the Source
child-view trigger (defined in `keys.Map`). For `ct-events`, `s` is explicitly
overridden to open ct-search instead. CloudTrail Events will never have a Source
child view -- the `s` override is intentional and must be documented in the
`ct-events` ResourceTypeDef registration.

### 15.3 app.go Routing

The ct-search view integrates into the existing view stack as follows:

1. **Command registration**: `:ct-search` (with aliases `:search`, `:investigate`)
   is registered in the command dispatcher alongside existing commands like `:profile`
   and `:region`. The dispatcher emits a `NavigateMsg` with a `ct-search` view type.

2. **View entry field**: The `viewEntry` struct gains a `ctSearch *CTSearchModel`
   field (nil when not active). This is analogous to how `helpModel` and
   `profileSelector` are stored as special view types outside the resource list flow.

3. **View stack management**: The three-screen flow (form -> results -> detail)
   is managed internally by the `CTSearchModel` itself, NOT as three separate stack
   entries. A single stack entry holds the `CTSearchModel`, which tracks its own
   `subView` state (form/loading/results/detail). Esc within the model navigates
   between sub-views; only Esc from the form screen pops the stack entry entirely.

### 15.4 Bubbles Components Used

| Component | Usage |
|-----------|-------|
| `bubbles/textinput` | 7 filter input fields in search form |
| `bubbles/spinner` | Loading state during search |
| `bubbles/viewport` | Event detail scrolling |
| `bubbles/help` | Help key style (not the full help component) |

### 15.4 New Message Types

```go
type CTSearchFilters struct {
    EventName   string
    Username    string
    EventSource string
    ResourceARN string
    AccessKey   string
    ErrorCode   string // client-side only
    SourceIP    string // client-side only
}

type CTSearchFormSubmitMsg struct {
    TimeRange   string
    Filters     CTSearchFilters
    WriteOnly   bool
    ErrorOnly   bool
}

type CTSearchResultsMsg struct {
    Events     []resource.Resource
    Pagination *resource.PaginationMeta
    Summary    string
}

type CTSearchPartialResultsMsg struct {
    Events     []resource.Resource
    Pagination *resource.PaginationMeta
    Summary    string
}

type CTSearchErrorMsg struct {
    Err error
}

type CTSearchPresetMsg struct {
    PresetNum int
}

type NavigateToResourceMsg struct {
    ResourceType string
    ResourceID   string
}
```

### 15.5 Three-Screen-in-One-Stack-Entry Guidance

Since the ct-search form, results, and event detail are three sub-views within
a single view stack entry, the following implementation details must be handled:

1. **WindowSizeMsg routing**: The top-level `CTSearchModel.Update()` receives
   `WindowSizeMsg` and forwards it to whichever sub-view is currently active.
   The form sub-view needs it for input field widths, the results sub-view for
   column layout, and the detail sub-view for viewport dimensions.

2. **Help context per sub-view**: The help overlay content differs per sub-view
   (see sections 9.1, 9.2, 9.3). The `CTSearchModel` must return different
   `help.KeyMap` bindings depending on the current `subView` state. The `?` key
   is handled at the `CTSearchModel` level and delegates to the active sub-view's
   key map.

3. **Esc dispatch per sub-state**:
   - `subView == detail`: Esc returns to results (internal state change)
   - `subView == results`: Esc returns to form with filters preserved (internal)
   - `subView == loading`: Esc stops the search, emits `CTSearchPartialResultsMsg`
   - `subView == form`: Esc pops the ct-search entry from the view stack entirely
     (returns a `messages.PopViewMsg` to the parent)

### 15.6 NavigateToResourceMsg Implementation

`NavigateToResourceMsg` should push a standard `NavigateMsg` to the view stack
(navigating to the resource type's list view) and then auto-select the matching
resource by ID. This reuses the existing navigation infrastructure rather than
creating a parallel navigation path. The sequence is:

1. Emit `NavigateMsg{ResourceType: "ec2"}` to push the EC2 list onto the stack
2. Emit a `SelectResourceByIDMsg{ID: "i-0abc123def456789a"}` to highlight and
   scroll to the matching row in the list

This ensures the user can then use standard list keybindings (d, y, Enter, etc.)
on the navigated resource.

---

## 16. Acceptance Criteria Mapping

| Criteria | Section |
|----------|---------|
| Search/filter interface accessible from CloudTrail event list | S2, S8.2 (s key), S15.2 |
| Time range selection with presets | S5.4 |
| Filter by: event name, username, event source, resource ARN, access key ID | S5.5 |
| Write-events-only toggle | S5.6 |
| Error-events-only toggle | S5.6 |
| Preset query patterns | S5.7 |
| Results sorted by time with event name, user, error visible | S6.2-6.6 |
| Event detail drill-down | S7 |
| Copy support | S8.3 |
| Cross-resource navigation | S7.6 |
| Pagination via NextToken | S13.2 |
| Visual distinction for error/write/root events | S6.7 |
| Demo mode fixtures | S14 |
| Data events note | S5.2 (bottom of form) |
