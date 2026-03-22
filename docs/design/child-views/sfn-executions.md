# Child View: Step Functions --> Executions --> Execution History

**Status:** Planned
**Tier:** SHOULD-HAVE (Executions) + Structural (Execution History)

---

## Level 1: Step Functions --> Executions

### Navigation

- **Entry:** Press Enter on a state machine in the Step Functions list
- **Frame title:** `sfn-executions(50) — order-processing-workflow`
- **View stack:** SFN --> Executions --> (Execution History via Enter, detail/YAML via d/y)
- **Esc** returns to Step Functions list
- **No new key bindings** beyond the standard set
- **Note:** Not supported for EXPRESS state machines. If the parent state machine type is EXPRESS, show a message: "Execution history is not available for Express state machines."

### views.yaml

```yaml
sfn_executions:
  list:
    Name:
      path: Name
      width: 36
    Status:
      path: Status
      width: 12
    Start Date:
      path: StartDate
      width: 22
    Stop Date:
      path: StopDate
      width: 22
    Duration:
      key: duration
      width: 12
  detail:
    - ExecutionArn
    - Name
    - Status
    - StartDate
    - StopDate
    - StateMachineArn
    - StateMachineAliasArn
    - StateMachineVersionArn
    - MapRunArn
    - ItemCount
    - RedriveCount
    - RedriveDate
    - RedriveStatus
    - RedriveStatusReason
```

Note: `duration` is computed from `StopDate - StartDate` and displayed as human-friendly (e.g., "2m 47s", "1h 23m", "47s"). For RUNNING executions, show elapsed time since StartDate.

Source struct: `sfntypes.ExecutionListItem`

### AWS API

- `states:ListExecutions` with `stateMachineArn`
- Paginated via `nextToken` — returns executions newest first
- Returns up to 100 per page
- **Latency:** Fast (<1 second)
- **Optional:** Can filter by `statusFilter` (RUNNING, SUCCEEDED, FAILED, TIMED_OUT, ABORTED) but default shows all

### ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────── sfn-executions(50) — order-processing-workflow ────────────────────────┐
│ NAME                                 STATUS       START DATE             STOP …  │
│ exec-2026-0322-0315-a1b2c3d4         SUCCEEDED    2026-03-22 03:15       2026…  │
│ exec-2026-0322-0215-e5f6a7b8         SUCCEEDED    2026-03-22 02:15       2026…  │
│ exec-2026-0322-0115-c9d0e1f2         FAILED       2026-03-22 01:15       2026…  │
│ exec-2026-0321-2315-34a5b6c7         SUCCEEDED    2026-03-21 23:15       2026…  │
│ exec-2026-0321-2215-d8e9f0a1         RUNNING      2026-03-21 22:15       —      │
│ exec-2026-0321-2115-2b3c4d5e         TIMED_OUT    2026-03-21 21:15       2026…  │
│ exec-2026-0321-2015-f1e2d3c4         ABORTED      2026-03-21 20:15       2026…  │
│   · · · (43 more)                                                               │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Row coloring by execution status (entire row):
- `SUCCEEDED`: GREEN `#9ece6a`
- `FAILED`: RED `#f7768e`
- `TIMED_OUT`: RED `#f7768e`
- `ABORTED`: DIM `#565f89`
- `RUNNING`: YELLOW `#e0af68`
- `PENDING_REDRIVE`: YELLOW `#e0af68`

Selected row: full-width blue background overrides status coloring.

### Copy Behavior

`c` copies the execution ARN.

### Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ EXECUTIONS            GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <enter> View History  <q>      Quit        <k>       Up         <:>   Command   │
│ <d>     Detail        </>      Filter      <g>       Top                        │
│ <y>     YAML          <:>      Command     <G>       Bottom                     │
│ <c>     Copy ARN                           <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Level 2: Executions --> Execution History

### Navigation

- **Entry:** Press Enter on an execution in the Executions list
- **Frame title:** `sfn-history(24) — exec-2026-0322-0115-c9d0e1f2`
- **View stack:** SFN --> Executions --> Execution History --> (detail/YAML via d/y)
- **Esc** returns to Executions list
- **No new key bindings** beyond the standard set

### views.yaml

```yaml
sfn_execution_history:
  list:
    Timestamp:
      path: Timestamp
      width: 22
    Event Type:
      key: event_type_short
      width: 24
    State:
      key: state_name
      width: 24
    Detail:
      key: event_detail
      width: 40
  detail:
    - Timestamp
    - Type
    - Id
    - PreviousEventId
    - ActivityFailedEventDetails
    - ActivityScheduleFailedEventDetails
    - ActivityScheduledEventDetails
    - ActivityStartedEventDetails
    - ActivitySucceededEventDetails
    - ActivityTimedOutEventDetails
    - ExecutionAbortedEventDetails
    - ExecutionFailedEventDetails
    - ExecutionStartedEventDetails
    - ExecutionSucceededEventDetails
    - ExecutionTimedOutEventDetails
    - LambdaFunctionFailedEventDetails
    - LambdaFunctionScheduledEventDetails
    - LambdaFunctionStartFailedEventDetails
    - LambdaFunctionSucceededEventDetails
    - LambdaFunctionTimedOutEventDetails
    - TaskFailedEventDetails
    - TaskScheduledEventDetails
    - TaskStartedEventDetails
    - TaskStartFailedEventDetails
    - TaskSubmitFailedEventDetails
    - TaskSubmittedEventDetails
    - TaskSucceededEventDetails
    - TaskTimedOutEventDetails
    - MapRunFailedEventDetails
    - MapRunStartedEventDetails
    - StateEnteredEventDetails
    - StateExitedEventDetails
```

Note on computed fields:
- `event_type_short`: simplified event type (e.g., "Task Failed" from `TaskFailed`, "State Entered" from `TaskStateEntered`)
- `state_name`: the name of the step/state involved (extracted from `StateEnteredEventDetails.Name`, `StateExitedEventDetails.Name`, or inferred from context)
- `event_detail`: error message if failed (from `*FailedEventDetails.Error` + `*FailedEventDetails.Cause`), or input/output summary for other events

Source struct: `sfntypes.HistoryEvent`

### AWS API

- `states:GetExecutionHistory` with `executionArn`
- Paginated via `nextToken`
- Events returned in chronological order (oldest first)
- **Latency:** Fast for short executions. Long-running executions with thousands of steps may have multiple pages.
- **Not available for EXPRESS state machines** — implementation must check parent state machine type and skip this view

### ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──── sfn-history(24) — exec-2026-0322-0115-c9d0e1f2 ───────────────────────────┐
│ TIMESTAMP              EVENT TYPE               STATE                    DETA…  │
│ 2026-03-22 01:15:00    Execution Started        —                        {"or…  │
│ 2026-03-22 01:15:00    State Entered            ValidateOrder            {"or…  │
│ 2026-03-22 01:15:01    Task Scheduled           ValidateOrder            lambd…  │
│ 2026-03-22 01:15:02    Task Succeeded           ValidateOrder            {"val…  │
│ 2026-03-22 01:15:02    State Exited             ValidateOrder            —      │
│ 2026-03-22 01:15:02    State Entered            ProcessPayment           {"ord…  │
│ 2026-03-22 01:15:03    Task Scheduled           ProcessPayment           lambd…  │
│ 2026-03-22 01:15:05    Task Failed              ProcessPayment           PayPr…  │
│ 2026-03-22 01:15:05    Execution Failed         —                        Stat…  │
│   · · · (15 more)                                                               │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Row coloring by event type (entire row):
- `*Succeeded`, `StateExited` (normal flow): GREEN `#9ece6a`
- `*Failed`, `*TimedOut`, `ExecutionAborted`: RED `#f7768e`
- `*Scheduled`, `*Started`, `StateEntered`: YELLOW `#e0af68`
- `ExecutionStarted`: PLAIN `#c0caf5`

Selected row: full-width blue background overrides event-type coloring.

### Copy Behavior

`c` copies the event detail text (error/cause for failures, input/output for others) — the text you want to paste into an incident channel.

### Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ EXECUTION HISTORY     GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy Detail                        <G>       Bottom                     │
│                                            <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```
