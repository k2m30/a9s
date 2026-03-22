# Child View: ECS Services --> Tasks

**Status:** Planned
**Tier:** MUST-HAVE

---

## Navigation

- **Entry:** Press Enter on an ECS service in the ECS Services list
- **Frame title:** `ecs-tasks(6) — payment-api`
- **View stack:** ECS Services --> Tasks --> (detail/YAML via d/y)
- **Esc** returns to ECS Services list
- **New key bindings on ECS Services list:**
  - `Enter` opens Tasks (default child — "what's running?")
  - `e` opens Service Events ("what happened?" — see ecs-svc-events.md)

## views.yaml

```yaml
ecs_tasks:
  list:
    Task ID:
      key: task_id_short
      width: 14
    Status:
      path: LastStatus
      width: 10
    Health:
      path: HealthStatus
      width: 10
    Task Def:
      key: task_def_short
      width: 20
    Started:
      path: StartedAt
      width: 22
    Stopped Reason:
      path: StoppedReason
      width: 36
  detail:
    - TaskArn
    - LastStatus
    - DesiredStatus
    - HealthStatus
    - TaskDefinitionArn
    - StartedAt
    - StoppedAt
    - StoppedReason
    - StopCode
    - StartedBy
    - Group
    - LaunchType
    - PlatformVersion
    - Cpu
    - Memory
    - Connectivity
    - Containers
    - Attachments
    - AvailabilityZone
    - CreatedAt
    - Tags
```

Note on computed fields:
- `task_id_short`: last 8 chars of the task ARN (e.g., `abc12345` from `arn:aws:ecs:...:task/cluster/abc12345def67890`)
- `task_def_short`: family:revision extracted from TaskDefinitionArn (e.g., `payment-api:47`)

Source struct: `ecstypes.Task`

## AWS API

- **Step 1:** `ecs:ListTasks` with `cluster` and `serviceName` filter — returns task ARNs
- **Step 2:** `ecs:DescribeTasks` batch call with up to 100 task ARNs
- Both RUNNING and recently STOPPED tasks should be fetched. Use `desiredStatus=RUNNING` first, then optionally `desiredStatus=STOPPED` to show recently stopped tasks (for crash-loop debugging)
- **Pagination:** ListTasks supports `nextToken`. Batch DescribeTasks up to 100 at a time.
- **Latency:** ~1-2 seconds for the two-call chain. Acceptable.

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────────────────── ecs-tasks(6) — payment-api ────────────────────────────────┐
│ TASK ID        STATUS     HEALTH     TASK DEF             STARTED              … │
│ a1b2c3d4       RUNNING    HEALTHY    payment-api:47       2026-03-22 02:48      │
│ e5f6a7b8       RUNNING    HEALTHY    payment-api:47       2026-03-22 02:48      │
│ c9d0e1f2       RUNNING    HEALTHY    payment-api:47       2026-03-22 02:48      │
│ 34a5b6c7       STOPPED    —          payment-api:46       2026-03-22 02:47      │
│ d8e9f0a1       STOPPED    —          payment-api:46       2026-03-22 02:15      │
│ 2b3c4d5e       STOPPED    —          payment-api:46       2026-03-22 02:15      │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Scrolled right to reveal Stopped Reason column (via `l` key):
```
│ TASK DEF             STARTED                STOPPED REASON                       │
│ payment-api:47       2026-03-22 02:48       —                                    │
│ payment-api:47       2026-03-22 02:48       —                                    │
│ payment-api:47       2026-03-22 02:48       —                                    │
│ payment-api:46       2026-03-22 02:47       Essential container exited           │
│ payment-api:46       2026-03-22 02:15       Scaling activity initiated by d…     │
│ payment-api:46       2026-03-22 02:15       Scaling activity initiated by d…     │
```

Row coloring by task status (entire row):
- `RUNNING` + `HEALTHY`: GREEN `#9ece6a`
- `RUNNING` + `UNHEALTHY`: RED `#f7768e`
- `RUNNING` + no health status: GREEN `#9ece6a`
- `PROVISIONING` / `PENDING` / `ACTIVATING`: YELLOW `#e0af68`
- `DEACTIVATING` / `STOPPING`: YELLOW `#e0af68`
- `STOPPED` with `StoppedReason` containing "error", "failed", "OOM", "exited": RED `#f7768e`
- `STOPPED` with normal stop reason (scaling, deployment): DIM `#565f89`

Selected row: full-width blue background overrides status coloring.

## Copy Behavior

`c` copies the full task ARN.

## Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ ECS TASKS             GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy ARN                           <G>       Bottom                     │
│                                            <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```
