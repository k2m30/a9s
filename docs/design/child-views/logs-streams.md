# Child View: CloudWatch Log Groups --> Log Streams --> Log Events

**Status:** Planned
**Tier:** MUST-HAVE (Foundation — other cross-service views depend on log viewer)

---

## Level 1: Log Groups --> Log Streams

### Navigation

- **Entry:** Press Enter on a log group in the CloudWatch Log Groups list
- **Frame title:** `log-streams(N) — /aws/lambda/my-function`
- **View stack:** Log Groups --> Log Streams --> (detail/YAML via d/y)
- **Esc** returns to Log Groups list
- **No new key bindings** beyond the standard set

### views.yaml

```yaml
log_streams:
  list:
    Stream Name:
      path: LogStreamName
      width: 48
    Last Event:
      path: LastEventTimestamp
      width: 22
    First Event:
      path: FirstEventTimestamp
      width: 22
    Size:
      path: StoredBytes
      width: 12
  detail:
    - LogStreamName
    - LastEventTimestamp
    - FirstEventTimestamp
    - StoredBytes
    - UploadSequenceToken
    - CreationTime
    - Arn
```

### AWS API

- `logs:DescribeLogStreams` with `logGroupName`, `orderBy=LastEventTime`, `descending=true`
- Paginated via `nextToken`
- **Latency warning:** Large log groups (thousands of streams) may take 2-3 seconds. The API returns up to 50 streams per page.
- Streams ordered by most-recent-event-first — the stream you want during an incident is always at the top

### ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────── log-streams(347) — /aws/lambda/payment-processor ──────────────────────┐
│ STREAM NAME                                      LAST EVENT              SIZE   │
│ 2026/03/22/[$LATEST]8a4b2c1d3e5f                 2026-03-22 02:47       14 KB  │
│ 2026/03/22/[$LATEST]7f6e5d4c3b2a                 2026-03-22 02:45       11 KB  │
│ 2026/03/22/[$LATEST]1a2b3c4d5e6f                 2026-03-22 02:31        8 KB  │
│ 2026/03/21/[$LATEST]9c8d7e6f5a4b                 2026-03-21 23:59       22 KB  │
│ 2026/03/21/[$LATEST]4b5c6d7e8f9a                 2026-03-21 23:44       19 KB  │
│ 2026/03/21/[$LATEST]2e3f4a5b6c7d                 2026-03-21 22:15        6 KB  │
│   · · · (341 more)                                                              │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Selected row state:
```
│ 2026/03/22/[$LATEST]8a4b2c1d3e5f                 2026-03-22 02:47       14 KB  │
```
Selected row gets full-width blue background (`#7aa2f7` bg, `#1a1b26` fg, bold).

No status-based row coloring — log streams have no health/status semantics.

### Copy Behavior

`c` copies the full log stream name (e.g., `2026/03/22/[$LATEST]8a4b2c1d3e5f`).

### Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ LOG STREAMS           GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <enter> View Events   <q>      Quit        <k>       Up         <:>   Command   │
│ <d>     Detail        </>      Filter      <g>       Top                        │
│ <y>     YAML          <:>      Command     <G>       Bottom                     │
│ <c>     Copy Name                          <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Level 2: Log Streams --> Log Events

### Navigation

- **Entry:** Press Enter on a log stream in the Log Streams list
- **Frame title:** `log-events(N) — 2026/03/22/[$LATEST]8a4b2c1d`
- **View stack:** Log Groups --> Log Streams --> Log Events --> (detail via d)
- **Esc** returns to Log Streams list
- **New key bindings:**
  - `t` — toggle timestamp display (show/hide timestamps to give more room for log message)
  - `w` — toggle word wrap (log lines can be very long)

### views.yaml

```yaml
log_events:
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

Note: `width: 0` on Message means "fill remaining width" — log messages should use all available horizontal space after the timestamp column.

### AWS API

- `logs:GetLogEvents` with `logGroupName`, `logStreamName`, `startFromHead=false` (most recent first)
- Paginated via `nextForwardToken` / `nextBackwardToken`
- **Latency warning:** Large streams (>10MB) can be slow. Initial fetch returns ~1MB or 10,000 events, whichever comes first.
- Alternative: `logs:FilterLogEvents` with `logStreamNames=[stream]` for single-stream filtered access

### ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────── log-events(156) — 2026/03/22/[$LATEST]8a4b2c1d ───────────────────────┐
│ TIMESTAMP              MESSAGE                                                  │
│ 2026-03-22 02:47:31    START RequestId: a1b2c3d4-e5f6-7890-abcd-ef1234567890   │
│ 2026-03-22 02:47:31    Processing payment for order ORD-2026-0322-1847          │
│ 2026-03-22 02:47:32    Calling Stripe API for charge $149.99                    │
│ 2026-03-22 02:47:33    [ERROR] StripeError: Card declined (insufficient_funds)  │
│ 2026-03-22 02:47:33    END RequestId: a1b2c3d4-e5f6-7890-abcd-ef1234567890     │
│ 2026-03-22 02:47:33    REPORT RequestId: a1b2c3d4  Duration: 2103.45 ms  Bil…  │
│ 2026-03-22 02:45:12    START RequestId: f7e8d9c0-b1a2-3456-cdef-789012345678   │
│ 2026-03-22 02:45:12    Processing payment for order ORD-2026-0322-1845          │
│   · · · (148 more)                                                              │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Row coloring for log events:
- Lines containing `ERROR`, `FATAL`, `Exception`, `Traceback`: RED `#f7768e`
- Lines containing `WARN`: YELLOW `#e0af68`
- `REPORT` lines: GREEN `#9ece6a` (these are invocation summaries)
- `START`/`END` lines: DIM `#565f89`
- All other lines: PLAIN `#c0caf5`

### Copy Behavior

`c` copies the full message text of the selected log event (without timestamp prefix).

### Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ LOG EVENTS            GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy Message                       <G>       Bottom                     │
│ <t>     Toggle Time                        <h/l>     Cols                       │
│ <w>     Word Wrap                          <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```
