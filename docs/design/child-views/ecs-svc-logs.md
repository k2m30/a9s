# Child View: ECS Services --> Container Logs (Cross-Service to CloudWatch)

**Status:** Planned
**Tier:** MUST-HAVE

---

## Navigation

- **Entry:** Press `L` on an ECS service in the ECS Services list
- **Frame title:** `ecs-logs(100) — payment-api`
- **View stack:** ECS Services --> Container Logs --> (detail via d)
- **Esc** returns to ECS Services list
- **New key bindings on ECS Services list:**
  - `L` (uppercase) opens Container Logs — the "show me application output" shortcut
  - This is the `kubectl logs deployment/my-service` equivalent for ECS

## views.yaml

```yaml
ecs_svc_logs:
  list:
    Timestamp:
      path: Timestamp
      width: 22
    Stream:
      key: stream_short
      width: 20
    Message:
      path: Message
      width: 0
  detail:
    - Timestamp
    - IngestionTime
    - LogStreamName
    - Message
    - EventId
```

Note: `stream_short` is a computed field that extracts the container-name/task-id-short suffix from the full log stream name (e.g., `app/a1b2c3d4` from `ecs/payment-api/app/a1b2c3d4e5f67890`). `width: 0` on Message fills remaining width.

## AWS API

This is a **cross-service** view requiring multiple calls to resolve the log group:

1. **`ecs:DescribeTaskDefinition`** with the TaskDefinition ARN from the parent service
   - Extract `ContainerDefinitions[].LogConfiguration.Options["awslogs-group"]` and `Options["awslogs-stream-prefix"]`
   - This tells us the CloudWatch log group and stream prefix
2. **`logs:FilterLogEvents`** on the extracted log group, with `logStreamNamePrefix` set to the awslogs-stream-prefix
   - Returns recent log lines from ALL containers across ALL tasks in the service, interleaved by timestamp
   - Limit to most recent 100-200 events

- **Pagination:** FilterLogEvents supports `nextToken` for loading older events
- **Latency warning:** 2-4 seconds for the two-call chain. The DescribeTaskDefinition call is fast (<500ms), but FilterLogEvents on a busy service's log group can be slow. Show spinner during fetch.
- **Edge case:** If the service uses multiple containers with different log groups, fetch from the primary (first) container's log group. Show the container name in the Stream column.
- **Edge case:** If `logConfiguration` is not `awslogs` (e.g., `firelens`), show a message: "Log driver is not awslogs. Logs not available in CloudWatch."

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌────────────────── ecs-logs(100) — payment-api ─────────────────────────────────┐
│ TIMESTAMP              STREAM               MESSAGE                             │
│ 2026-03-22 02:51:33    app/a1b2c3d4         {"level":"info","msg":"Request co…  │
│ 2026-03-22 02:51:33    app/e5f6a7b8         {"level":"info","msg":"Health che…  │
│ 2026-03-22 02:51:32    app/c9d0e1f2         {"level":"info","msg":"Request co…  │
│ 2026-03-22 02:51:31    app/a1b2c3d4         {"level":"error","msg":"Database …  │
│ 2026-03-22 02:51:30    app/e5f6a7b8         {"level":"info","msg":"Request co…  │
│ 2026-03-22 02:51:29    app/a1b2c3d4         {"level":"warn","msg":"Retry atte…  │
│ 2026-03-22 02:51:28    app/c9d0e1f2         {"level":"info","msg":"Processing…  │
│   · · · (93 more)                                                               │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Row coloring by log content (entire row):
- Lines containing `"error"`, `"fatal"`, `ERROR`, `FATAL`: RED `#f7768e`
- Lines containing `"warn"`, `WARN`: YELLOW `#e0af68`
- All other lines: PLAIN `#c0caf5`

Selected row: full-width blue background overrides content-based coloring.

## Copy Behavior

`c` copies the full message text of the selected log line.

## Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ CONTAINER LOGS        GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy Message                       <G>       Bottom                     │
│ <w>     Word Wrap                          <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```
