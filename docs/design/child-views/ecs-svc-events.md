# Child View: ECS Services --> Service Events

**Status:** Planned
**Tier:** MUST-HAVE

---

## Navigation

- **Entry:** Press `e` on an ECS service in the ECS Services list (Enter is reserved for Tasks child view, which is the more common drill-down)
- **Frame title:** `ecs-events(87) — payment-api`
- **View stack:** ECS Services --> Service Events --> (detail via d)
- **Esc** returns to ECS Services list
- **New key bindings:**
  - `e` on the ECS Services list opens this events view (documented in ECS Services help screen)

## views.yaml

```yaml
ecs_svc_events:
  list:
    Timestamp:
      path: CreatedAt
      width: 22
    Message:
      path: Message
      width: 0
  detail:
    - CreatedAt
    - Message
    - Id
```

Note: `width: 0` on Message means fill remaining width. ECS service events are essentially a timeline of text messages with timestamps. The message IS the data.

Source struct: `ecstypes.ServiceEvent`

## AWS API

- Data source: `ecs:DescribeServices` response, `Services[0].Events[]` field
- The Events array is already returned by DescribeServices (up to 100 most recent events, newest first)
- This is a **re-fetch** of the same API that populates the parent, but with `include=["EVENTS"]` to ensure events are populated
- **No additional API call needed** if the parent service response cached Events
- **No pagination** — AWS returns at most 100 events per service
- **Latency:** Fast (<1 second), same call as parent but extracting a different field

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌────────────────────── ecs-events(87) — payment-api ────────────────────────────┐
│ TIMESTAMP              MESSAGE                                                  │
│ 2026-03-22 02:51       (service payment-api) has reached a steady state.        │
│ 2026-03-22 02:48       (service payment-api) registered 1 targets in (target-g… │
│ 2026-03-22 02:47       (service payment-api) has started 1 tasks: (task abc123… │
│ 2026-03-22 02:45       (service payment-api) is unable to consistently start t… │
│ 2026-03-22 02:44       (service payment-api) stopped 1 tasks: (task def456).    │
│ 2026-03-22 02:43       (service payment-api) was unable to place a task becaus… │
│ 2026-03-22 02:15       (service payment-api) has reached a steady state.        │
│   · · · (80 more)                                                               │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Row coloring by event content (entire row):
- Messages containing "steady state": GREEN `#9ece6a` — service is healthy
- Messages containing "unable to", "failed", "stopped", "unhealthy": RED `#f7768e` — something went wrong
- Messages containing "has started", "registered", "deregistered": YELLOW `#e0af68` — change in progress
- All other messages: PLAIN `#c0caf5`

Selected row: full-width blue background overrides content-based coloring.

## Copy Behavior

`c` copies the full event message text of the selected event.

## Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ SERVICE EVENTS        GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy Message                       <G>       Bottom                     │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```
