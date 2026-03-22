# Child View: Glue Jobs --> Job Runs

**Status:** Planned
**Tier:** SHOULD-HAVE

---

## Navigation

- **Entry:** Press Enter on a Glue job in the Glue Jobs list
- **Frame title:** `glue-runs(30) — nightly-etl-transform`
- **View stack:** Glue Jobs --> Job Runs --> (detail/YAML via d/y)
- **Esc** returns to Glue Jobs list
- **No new key bindings** beyond the standard set

## views.yaml

```yaml
glue_runs:
  list:
    Run ID:
      key: run_id_short
      width: 12
    State:
      path: JobRunState
      width: 12
    Started:
      path: StartedOn
      width: 22
    Execution Time:
      key: execution_time_human
      width: 14
    Error Message:
      path: ErrorMessage
      width: 44
    DPU Hours:
      key: dpu_hours
      width: 10
  detail:
    - Id
    - JobRunState
    - StartedOn
    - CompletedOn
    - ExecutionTime
    - ErrorMessage
    - Attempt
    - PreviousRunId
    - TriggerName
    - JobName
    - AllocatedCapacity
    - MaxCapacity
    - WorkerType
    - NumberOfWorkers
    - Timeout
    - GlueVersion
    - DPUSeconds
    - ExecutionClass
    - LogGroupName
```

Note on computed fields:
- `run_id_short`: first 8 characters of the `Id` field (a UUID, e.g., `jr_a1b2c3d4`)
- `execution_time_human`: `ExecutionTime` (in seconds) formatted as human-friendly (e.g., "47m 23s", "2h 15m")
- `dpu_hours`: computed from `DPUSeconds / 3600` formatted to 1 decimal (e.g., "12.5")

Source struct: `gluetypes.JobRun`

## AWS API

- `glue:GetJobRuns` with `JobName`
- Paginated via `NextToken` — returns runs newest first
- Returns up to 200 per page
- **Latency:** Fast (<1 second)

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────────── glue-runs(30) — nightly-etl-transform ─────────────────────────────┐
│ RUN ID       STATE        STARTED                EXEC TIME      ERROR MESSAG…    │
│ jr_a1b2c3    SUCCEEDED    2026-03-22 03:00       47m 23s        —                │
│ jr_e5f6a7    SUCCEEDED    2026-03-21 03:00       45m 12s        —                │
│ jr_c9d0e1    FAILED       2026-03-20 03:00       12m 05s        java.lang.Out…   │
│ jr_34a5b6    SUCCEEDED    2026-03-19 03:00       44m 58s        —                │
│ jr_d8e9f0    SUCCEEDED    2026-03-18 03:00       46m 30s        —                │
│ jr_2b3c4d    TIMEOUT      2026-03-17 03:00       120m 00s       Job reached t…   │
│ jr_f1e2d3    STOPPED      2026-03-16 03:00       5m 44s         —                │
│   · · · (23 more)                                                                │
└──────────────────────────────────────────────────────────────────────────────────┘
```

Scrolled right to show DPU Hours column:
```
│ ERROR MESSAGE                                      DPU HOURS  │
│ —                                                  12.5       │
│ —                                                  11.9       │
│ java.lang.OutOfMemoryError: GC overhead limit e…   3.2       │
│ —                                                  11.8       │
│ —                                                  12.3       │
│ Job reached the timeout limit of 120 minutes.      40.0       │
│ —                                                  1.5        │
```

Row coloring by job run state (entire row):
- `SUCCEEDED`: GREEN `#9ece6a`
- `FAILED`: RED `#f7768e`
- `ERROR`: RED `#f7768e`
- `TIMEOUT`: RED `#f7768e`
- `RUNNING` / `STARTING` / `WAITING`: YELLOW `#e0af68`
- `STOPPED`: DIM `#565f89`

Selected row: full-width blue background overrides state coloring.

## Copy Behavior

`c` copies the ErrorMessage if the run failed/timed out, otherwise the Run ID. The error message is what you paste into Jira or Slack when investigating ETL failures.

## Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ GLUE JOB RUNS         GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy Error/ID                      <G>       Bottom                     │
│                                            <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```
