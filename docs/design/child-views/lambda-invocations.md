# Child View: Lambda --> Recent Invocations (from CloudWatch Logs)

**Status:** Planned
**Tier:** MUST-HAVE

---

## Level 1: Lambda --> Invocations

### Navigation

- **Entry:** Press Enter on a Lambda function in the Lambda list
- **Frame title:** `lambda-invocations(25) — payment-processor`
- **View stack:** Lambda --> Invocations --> (Log Lines via Enter, detail/YAML via d/y)
- **Esc** returns to Lambda list
- **New key bindings on Lambda function list (parent):**
  - `s` — open Function Code view for the selected Lambda (see Level 1-alt below)
- **New key bindings on Invocations list:**
  - `s` — open Function Code view for the parent Lambda (see Level 1-alt below)

### views.yaml

```yaml
lambda_invocations:
  list:
    Timestamp:
      key: timestamp
      width: 22
    Request ID:
      key: request_id
      width: 12
    Status:
      key: status
      width: 8
    Duration:
      key: duration_ms
      width: 10
    Memory:
      key: memory_used
      width: 14
    Cold Start:
      key: cold_start
      width: 10
  detail:
    - request_id
    - timestamp
    - status
    - duration_ms
    - billed_duration_ms
    - memory_size_mb
    - memory_used_mb
    - init_duration_ms
    - xray_trace_id
```

Note: This view is NOT backed by a single AWS SDK struct. It is parsed from CloudWatch Logs REPORT lines. Fields use `key:` (computed) rather than `path:` (struct field). Each REPORT line from `/aws/lambda/{FunctionName}` is parsed into these fields:
```
REPORT RequestId: abc123  Duration: 2103.45 ms  Billed Duration: 2200 ms  Memory Size: 256 MB  Max Memory Used: 128 MB  Init Duration: 312.52 ms
```

### AWS API

- **Primary:** `logs:FilterLogEvents` on log group `/aws/lambda/{FunctionName}` (or the custom group from parent's `LoggingConfig.LogGroup`)
- **Filter pattern:** `"REPORT RequestId"` — extracts invocation summary lines only
- **Limit:** 25-50 most recent (configurable, no pagination needed for initial view)
- **Latency warning:** Can take 1-3 seconds depending on log group size. The filter pattern is server-side, so only matching events are returned.
- **Error detection:** A second parallel call with filter pattern `"ERROR"` or `"Task timed out"` cross-referenced by RequestId to determine status
- **Pagination:** `logs:FilterLogEvents` supports `nextToken` for loading older invocations

### ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────────── lambda-invocations(25) — payment-processor ────────────────────────┐
│ TIMESTAMP              REQ ID       STATUS   DURATION   MEMORY         COLD     │
│ 2026-03-22 02:47       a1b2c3d4     ERROR    2103 ms    128/256 MB     no       │
│ 2026-03-22 02:45       f7e8d9c0     OK       187 ms     89/256 MB      no       │
│ 2026-03-22 02:31       3c4d5e6f     OK       203 ms     91/256 MB      no       │
│ 2026-03-22 02:15       9a8b7c6d     OK       195 ms     88/256 MB      no       │
│ 2026-03-22 01:47       2d3e4f5a     TIMEOUT  30000 ms   256/256 MB     no       │
│ 2026-03-22 01:45       6b7c8d9e     OK       201 ms     90/256 MB      yes      │
│ 2026-03-22 01:31       1e2f3a4b     OK       189 ms     87/256 MB      no       │
│   · · · (18 more)                                                               │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Row coloring by invocation status (entire row):
- `OK`: GREEN `#9ece6a`
- `ERROR`: RED `#f7768e`
- `TIMEOUT`: RED `#f7768e`
- Cold start = `yes`: YELLOW `#e0af68` (even if OK — cold starts are performance anomalies worth highlighting)

Selected row: full-width blue background overrides status color.

### Copy Behavior

`c` copies the full Request ID (e.g., `a1b2c3d4-e5f6-7890-abcd-ef1234567890`).

### Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ INVOCATIONS           GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <enter> View Logs     <q>      Quit        <k>       Up         <:>   Command   │
│ <d>     Detail        </>      Filter      <g>       Top                        │
│ <y>     YAML          <:>      Command     <G>       Bottom                     │
│ <c>     Copy Req ID                        <h/l>     Cols                       │
│ <s>     Source Code                        <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Level 1-alt: Lambda --> Function Code

### Navigation

- **Entry:** Press `s` on a Lambda function in the Lambda list (parent), OR press `s` from the Invocations list (Level 1)
- **Frame title:** `lambda-code — payment-processor/handler.py`
  - Format: `lambda-code — {FunctionName}/{HandlerFile}`
  - The handler file is derived from `Configuration.Handler` (e.g., `index.handler` yields `index.js` or `index.py` depending on runtime)
- **View stack when entered from Lambda list:** Lambda --> Function Code
- **View stack when entered from Invocations:** Lambda --> Invocations --> Function Code
- **Esc** returns to the view that launched it (Lambda list or Invocations list)

### Handler File Resolution

The `Configuration.Handler` field uses runtime-specific conventions to identify the entry point. The handler file is derived as follows:

| Runtime prefix | Handler example      | Resolved file         |
|----------------|----------------------|-----------------------|
| `python`       | `handler.process`    | `handler.py`          |
| `python`       | `src/app.lambda_fn`  | `src/app.py`          |
| `nodejs`       | `index.handler`      | `index.js`            |
| `nodejs`       | `src/app.handler`    | `src/app.js`          |
| `ruby`         | `handler.process`    | `handler.rb`          |
| `java`         | `com.example.Handler::handleRequest` | `com/example/Handler.java` |
| `go`           | `bootstrap`          | `bootstrap`           |
| `dotnet`       | `Assembly::Ns.Class::Method` | (see note below) |

The module part (everything before the last `.` or `::`) maps to the file path. For runtimes where mapping is ambiguous (Java, .NET), the view attempts to find the file by name in the zip. If no match is found, the view falls back to showing a file listing of the zip contents (see "Fallback: File Listing" below).

### AWS API

- **Primary:** `lambda:GetFunction` (already called by parent for detail/YAML — the `Code` section of the response is used here)
- **Key fields from response:**
  - `Code.Location` — presigned S3 URL to download the deployment package (.zip)
  - `Configuration.Handler` — identifies the handler file within the package
  - `Configuration.Runtime` — needed for handler-to-filename resolution
  - `Configuration.PackageType` — `Zip` or `Image`
  - `Configuration.CodeSize` — deployment package size in bytes
- **Fetch sequence:**
  1. Check `PackageType`: if `Image`, show container image message (no download)
  2. Check `CodeSize`: if > 5,242,880 bytes (5 MB), show package-too-large message (no download)
  3. HTTP GET on `Code.Location` to download the .zip
  4. Extract the handler file from the .zip (identified via `Handler` + `Runtime`)
  5. Render the file contents in the viewport
- **Latency:** The presigned URL download is typically fast (<1-2 seconds) for small packages. Show spinner during fetch.
- **Permissions:** Requires `lambda:GetFunction` (already granted for the parent Lambda list). No additional IAM permissions needed beyond what is already used.
- **Caching:** The downloaded source should be cached in memory for the lifetime of the Lambda detail session. Pressing `s` again after Esc should not re-download. `ctrl+r` forces a fresh download.

### Component

Bubbles component: `bubbles/viewport` — same as YAML view and detail view. The code is rendered as plain text inside the viewport with line numbers.

### Line Number Rendering

Each line of source code is prefixed with a right-aligned line number in dim style, followed by a pipe separator:

```
  1 │ import json
  2 │ import boto3
  3 │ from datetime import datetime
```

- Line number: DIM `#565f89`, right-aligned to width of max line number (e.g., 3 chars for files up to 999 lines)
- Pipe separator: DIM `#414868`
- Code text: PLAIN `#c0caf5`

No syntax highlighting — the view shows plain source text. This keeps the implementation simple and avoids the need for language-specific parsers. The line numbers are the primary value: they let the engineer correlate stack trace line numbers (e.g., "line 42") with the actual code.

### ASCII Wireframe — Normal (Python handler)

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────────────── lambda-code — payment-processor/handler.py ──────────────────────┐
│  1 │ import json                                                                  │
│  2 │ import boto3                                                                 │
│  3 │ from datetime import datetime                                                │
│  4 │                                                                              │
│  5 │ s3 = boto3.client('s3')                                                      │
│  6 │ dynamodb = boto3.resource('dynamodb')                                        │
│  7 │                                                                              │
│  8 │ def process(event, context):                                                 │
│  9 │     """Process incoming payment events."""                                   │
│ 10 │     order_id = event['detail']['order_id']                                   │
│ 11 │     amount = event['detail']['amount']                                       │
│ 12 │     currency = event['detail']['currency']                                   │
│ 13 │                                                                              │
│ 14 │     # Validate payment details                                               │
│ 15 │     if amount <= 0:                                                          │
│ 16 │         raise ValueError(f"Invalid amount: {amount}")                        │
│ 17 │                                                                              │
│   · · · (scroll for more)                                                        │
└──────────────────────────────────────────────────────────────────────────────────┘
```

### ASCII Wireframe — Normal (Node.js handler)

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────────────── lambda-code — image-resizer/index.js ────────────────────────────┐
│  1 │ const AWS = require('aws-sdk');                                              │
│  2 │ const sharp = require('sharp');                                              │
│  3 │                                                                              │
│  4 │ const s3 = new AWS.S3();                                                     │
│  5 │                                                                              │
│  6 │ exports.handler = async (event) => {                                         │
│  7 │   const bucket = event.Records[0].s3.bucket.name;                            │
│  8 │   const key = event.Records[0].s3.object.key;                                │
│  9 │                                                                              │
│ 10 │   const original = await s3.getObject({ Bucket: bucket, Key: key }).promise… │
│ 11 │   const resized = await sharp(original.Body).resize(800, 600).toBuffer();    │
│ 12 │                                                                              │
│ 13 │   await s3.putObject({                                                       │
│ 14 │     Bucket: bucket,                                                          │
│ 15 │     Key: `thumbnails/${key}`,                                                │
│ 16 │     Body: resized,                                                           │
│ 17 │     ContentType: 'image/jpeg',                                               │
│ 18 │   }).promise();                                                              │
│ 19 │                                                                              │
│ 20 │   return { statusCode: 200, body: 'OK' };                                   │
│ 21 │ };                                                                           │
└──────────────────────────────────────────────────────────────────────────────────┘
```

### ASCII Wireframe — Loading

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────────────── lambda-code — payment-processor ─────────────────────────────────┐
│                                                                                  │
│                                                                                  │
│                                                                                  │
│        ... Downloading function code...                                          │
│                                                                                  │
│                                                                                  │
│                                                                                  │
└──────────────────────────────────────────────────────────────────────────────────┘
```

Note: The frame title omits the handler filename during loading because the filename is not yet resolved. Once the code is fetched, the title updates to include the filename.

### ASCII Wireframe — Container Image Lambda

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────────────── lambda-code — payment-processor ─────────────────────────────────┐
│                                                                                  │
│                                                                                  │
│                                                                                  │
│  Container image Lambda — source code not viewable                               │
│                                                                                  │
│  Package type:  Image                                                            │
│  Image URI:     123456789012.dkr.ecr.us-east-1.amazonaws.com/payment:latest      │
│                                                                                  │
│                                                                                  │
└──────────────────────────────────────────────────────────────────────────────────┘
```

Message styling:
- Main message: YELLOW `#e0af68`, bold
- Labels ("Package type:", "Image URI:"): DIM `#565f89`
- Values: PLAIN `#c0caf5`

### ASCII Wireframe — Package Too Large

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────────────── lambda-code — payment-processor ─────────────────────────────────┐
│                                                                                  │
│                                                                                  │
│                                                                                  │
│  Package too large for inline viewing (23.4 MB)                                  │
│                                                                                  │
│  Handler:   handler.process                                                      │
│  Runtime:   python3.12                                                           │
│  Code size: 23.4 MB (limit: 5 MB)                                               │
│                                                                                  │
│                                                                                  │
└──────────────────────────────────────────────────────────────────────────────────┘
```

Message styling:
- Main message: YELLOW `#e0af68`, bold
- Labels ("Handler:", "Runtime:", "Code size:"): DIM `#565f89`
- Values: PLAIN `#c0caf5`
- Size values exceeding limit: RED `#f7768e`

### ASCII Wireframe — Handler File Not Found in Package

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────────────── lambda-code — payment-processor ─────────────────────────────────┐
│                                                                                  │
│  Handler file not found: handler.py                                              │
│  Handler config: handler.process (python3.12)                                    │
│                                                                                  │
│  Files in deployment package:                                                    │
│   requirements.txt                                                               │
│   lib/                                                                           │
│   lib/payment_utils.py                                                           │
│   lib/stripe_client.py                                                           │
│   src/                                                                           │
│   src/handler.py                                                                 │
│   src/models.py                                                                  │
│                                                                                  │
└──────────────────────────────────────────────────────────────────────────────────┘
```

This fallback is shown when the handler file cannot be located at the expected path in the zip. The file listing helps the engineer identify the correct file. Pressing Enter on a file in this listing opens that file in the code viewport.

Message styling:
- "Handler file not found:" message: YELLOW `#e0af68`, bold
- "Handler config:" label: DIM `#565f89`
- "Files in deployment package:" label: DIM `#565f89`
- Directory entries (trailing `/`): DIM `#565f89`
- File entries: PLAIN `#c0caf5`
- Selected file entry: full-width blue background `#7aa2f7` bg, `#1a1b26` fg, bold (standard selection style)

### Fallback: File Listing

When the handler file cannot be resolved (ambiguous runtime, file not at expected path), the view shows a navigable file listing of the zip contents. This is a simple list view (not a tree) showing all files in the package, sorted alphabetically. Directories are shown with a trailing `/` and are not selectable.

- `j`/`k` to navigate the file list
- `Enter` on a file opens it in the code viewport
- `Esc` from the code viewport returns to the file listing
- `Esc` from the file listing returns to the previous view (Lambda list or Invocations)

Frame title for file listing: `lambda-code — payment-processor (files)`
Frame title after selecting a file: `lambda-code — payment-processor/src/handler.py`

### Key Bindings

| Key      | Action                  | Notes                                          |
|----------|-------------------------|------------------------------------------------|
| `j`/`dn` | Scroll down one line    | Viewport scroll                                |
| `k`/`up` | Scroll up one line      | Viewport scroll                                |
| `g`      | Jump to top of file     |                                                |
| `G`      | Jump to end of file     |                                                |
| `w`      | Toggle word wrap        | Long lines are common in minified JS, configs  |
| `c`      | Copy current line       | Copies the code text of the cursor line (without line number) |
| `esc`    | Go back                 | Returns to Lambda list or Invocations list     |
| `?`      | Toggle help screen      |                                                |
| `pgup`/`ctrl+u` | Page up          |                                                |
| `pgdn`/`ctrl+d` | Page down         |                                                |

Note: `d`, `y`, `/`, and `enter` are NOT active in the code view. The code view is a read-only viewport like the YAML view. There is no detail or YAML sub-view of source code. Filter is not applicable to source code viewing.

### Copy Behavior

`c` copies the source text of the line at the current viewport cursor position, excluding the line number prefix. For example, if the cursor is on line 42:

```
 42 │     raise ValueError(f"Invalid amount: {amount}")
```

Then `c` copies: `    raise ValueError(f"Invalid amount: {amount}")` (with leading whitespace preserved, without line number or pipe).

Header flashes "Copied!" in green on successful copy.

### State Transitions

| Msg                       | From State              | To State                     |
|---------------------------|-------------------------|------------------------------|
| `KeyMsg(s)`               | Lambda list             | Function Code (loading)      |
| `KeyMsg(s)`               | Invocations list        | Function Code (loading)      |
| `FunctionCodeLoadedMsg`   | Function Code (loading) | Function Code (viewport)     |
| `FunctionCodeErrorMsg`    | Function Code (loading) | Function Code (error)        |
| `KeyMsg(esc)`             | Function Code           | Previous view (pop stack)    |
| `KeyMsg(ctrl+r)`          | Function Code           | Function Code (loading)      |

New Msg types:
- `FunctionCodeLoadedMsg` — carries the source text, resolved filename, and total line count
- `FunctionCodeErrorMsg` — carries the error message (API failure, download failure, etc.)

### Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ FUNCTION CODE         GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <c>     Copy Line                          <k>       Up         <:>   Command   │
│ <w>     Word Wrap                          <g>       Top                        │
│                                            <G>       Bottom                     │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Level 2: Invocations --> Log Lines

### Navigation

- **Entry:** Press Enter on an invocation in the Lambda Invocations list
- **Frame title:** `invocation-logs(12) — a1b2c3d4`
- **View stack:** Lambda --> Invocations --> Log Lines --> (detail via d)
- **Esc** returns to Invocations list
- **New key bindings:**
  - `w` — toggle word wrap (stack traces and error messages can be very long)

### views.yaml

```yaml
lambda_invocation_logs:
  list:
    Timestamp:
      path: Timestamp
      width: 22
    Message:
      path: Message
      width: 0
  detail:
    - Timestamp
    - IngestionTime
    - Message
    - EventId
```

Note: `width: 0` means fill remaining width. Same structure as log_events but scoped to a single invocation's RequestId.

### AWS API

- `logs:FilterLogEvents` on the same log group, with filter pattern `"RequestId: {request-id}"` (where `{request-id}` is the full UUID from the selected invocation)
- This returns all log lines for that specific invocation: START, application output, ERROR/Exception if any, END, REPORT
- Typically 5-50 lines per invocation. No pagination needed for most cases.
- **Latency:** Fast (<1 second) since FilterLogEvents with a specific RequestId pattern is highly selective

### ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────────────── invocation-logs(8) — a1b2c3d4 ────────────────────────────────┐
│ TIMESTAMP              MESSAGE                                                  │
│ 2026-03-22 02:47:31    START RequestId: a1b2c3d4-e5f6-7890-abcd-ef1234567890   │
│ 2026-03-22 02:47:31    Processing payment for order ORD-2026-0322-1847          │
│ 2026-03-22 02:47:32    Calling Stripe API for charge $149.99                    │
│ 2026-03-22 02:47:33    [ERROR] StripeError: Card declined (insufficient_funds)  │
│ 2026-03-22 02:47:33    Traceback (most recent call last):                       │
│ 2026-03-22 02:47:33      File "/var/task/handler.py", line 42, in process       │
│ 2026-03-22 02:47:33    END RequestId: a1b2c3d4-e5f6-7890-abcd-ef1234567890     │
│ 2026-03-22 02:47:33    REPORT RequestId: a1b2c3d4  Duration: 2103.45 ms  Bil…  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Row coloring (same as log_events):
- Lines containing `ERROR`, `FATAL`, `Exception`, `Traceback`: RED `#f7768e`
- Lines containing `WARN`: YELLOW `#e0af68`
- `REPORT` lines: GREEN `#9ece6a`
- `START`/`END` lines: DIM `#565f89`
- All other lines: PLAIN `#c0caf5`

### Copy Behavior

`c` copies the full message text of the selected log line.

### Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ LOG LINES             GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy Message                       <G>       Bottom                     │
│ <w>     Word Wrap                          <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```
