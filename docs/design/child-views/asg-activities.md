# Child View: Auto Scaling Groups --> Scaling Activities

**Status:** Planned
**Tier:** MUST-HAVE

---

## Navigation

- **Entry:** Press Enter on an ASG in the Auto Scaling Groups list
- **Frame title:** `asg-activities(48) — api-prod-asg`
- **View stack:** ASG --> Scaling Activities --> (detail/YAML via d/y)
- **Esc** returns to ASG list
- **No new key bindings** beyond the standard set

## views.yaml

```yaml
asg_activities:
  list:
    Start Time:
      path: StartTime
      width: 22
    Status:
      path: StatusCode
      width: 14
    Description:
      path: Description
      width: 50
    Cause:
      path: Cause
      width: 40
  detail:
    - ActivityId
    - StartTime
    - EndTime
    - StatusCode
    - StatusMessage
    - Description
    - Cause
    - Details
    - Progress
    - AutoScalingGroupName
    - AutoScalingGroupARN
    - AutoScalingGroupState
```

Source struct: `autoscalingtypes.Activity`

## AWS API

- `autoscaling:DescribeScalingActivities` with `AutoScalingGroupName`
- Paginated via `NextToken` — activities returned newest first
- **Latency:** Fast (<1 second). Returns up to 100 activities per page.
- **Note:** Activities are retained for 6 weeks (42 days). Older activities are automatically purged by AWS.

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────────── asg-activities(48) — api-prod-asg ─────────────────────────────────┐
│ START TIME             STATUS         DESCRIPTION                               … │
│ 2026-03-22 03:15       Successful     Launching a new EC2 instance: i-0abc123…   │
│ 2026-03-22 03:15       Successful     Launching a new EC2 instance: i-0def456…   │
│ 2026-03-22 03:15       Successful     Launching a new EC2 instance: i-0789abc…   │
│ 2026-03-22 02:00       Successful     Terminating EC2 instance: i-0111222333…    │
│ 2026-03-21 23:30       Failed         Launching a new EC2 instance.  Status M…   │
│ 2026-03-21 22:15       Successful     Launching a new EC2 instance: i-0444555…   │
│ 2026-03-21 14:00       Cancelled      Launching a new EC2 instance.              │
│   · · · (41 more)                                                                │
└──────────────────────────────────────────────────────────────────────────────────┘
```

Scrolled right to reveal Cause column (via `l` key):
```
│ DESCRIPTION                                      CAUSE                           │
│ Launching a new EC2 instance: i-0abc123def456…   At 2026-03-22T03:15:00Z an al…  │
│ Launching a new EC2 instance: i-0def456789abc…   At 2026-03-22T03:15:00Z an al…  │
│ Launching a new EC2 instance: i-0789abcdef012…   At 2026-03-22T03:15:00Z an al…  │
│ Terminating EC2 instance: i-0111222333444555…    At 2026-03-22T02:00:00Z an al…  │
│ Launching a new EC2 instance.  Status Message…   At 2026-03-21T23:30:00Z an al…  │
│ Launching a new EC2 instance: i-0444555666777…   At 2026-03-21T22:15:00Z a use…  │
│ Launching a new EC2 instance.                    At 2026-03-21T14:00:00Z a use…  │
```

Row coloring by activity status (entire row):
- `Successful`: GREEN `#9ece6a`
- `Failed`: RED `#f7768e`
- `InProgress` / `PreInService` / `WaitingForSpotInstanceRequestId` / `WaitingForSpotInstanceId` / `WaitingForInstanceId` / `WaitingForConnectionDraining` / `MidLifecycleAction` / `WaitingForELBConnectionDraining`: YELLOW `#e0af68`
- `Cancelled`: DIM `#565f89`

Selected row: full-width blue background overrides status coloring.

## Copy Behavior

`c` copies the Description text — which typically contains the instance ID that was launched/terminated.

## Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ SCALING ACTIVITIES    GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy Desc                          <G>       Bottom                     │
│                                            <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```
