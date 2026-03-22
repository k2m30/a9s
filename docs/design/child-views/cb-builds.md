# Child View: CodeBuild Projects --> Builds --> Build Logs

**Status:** Planned
**Tier:** SHOULD-HAVE

---

## Level 1: CodeBuild Projects --> Builds

### Navigation

- **Entry:** Press Enter on a CodeBuild project in the CodeBuild list
- **Frame title:** `cb-builds(25) — payment-api-build`
- **View stack:** CodeBuild --> Builds --> (Build Logs via Enter, detail/YAML via d/y)
- **Esc** returns to CodeBuild list
- **No new key bindings** beyond the standard set

### views.yaml

```yaml
cb_builds:
  list:
    Build #:
      path: BuildNumber
      width: 10
    Status:
      path: BuildStatus
      width: 14
    Start Time:
      path: StartTime
      width: 22
    Duration:
      key: duration
      width: 12
    Source Version:
      key: source_version_short
      width: 14
    Initiator:
      path: Initiator
      width: 24
  detail:
    - Id
    - Arn
    - BuildNumber
    - BuildStatus
    - StartTime
    - EndTime
    - CurrentPhase
    - SourceVersion
    - ResolvedSourceVersion
    - Initiator
    - Source
    - Environment
    - Phases
    - Logs
    - Cache
    - VpcConfig
    - ServiceRole
    - TimeoutInMinutes
    - QueuedTimeoutInMinutes
    - BuildBatchArn
```

Note on computed fields:
- `duration`: computed from `EndTime - StartTime`, displayed as human-friendly (e.g., "3m 22s", "12m 45s"). For IN_PROGRESS builds, show elapsed time.
- `source_version_short`: first 8 characters of the commit SHA from `SourceVersion` or `ResolvedSourceVersion` (e.g., `a1b2c3d4`)

Source struct: `cbtypes.Build`

### AWS API

- **Step 1:** `codebuild:ListBuildsForProject` with `projectName` — returns build IDs (newest first)
- **Step 2:** `codebuild:BatchGetBuilds` with up to 100 build IDs — returns full build details
- **Pagination:** ListBuildsForProject supports `nextToken`
- **Latency:** 1-2 seconds for the two-call chain

### ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌─────────────── cb-builds(25) — payment-api-build ──────────────────────────────┐
│ BUILD #    STATUS         START TIME             DURATION     SOURCE      INIT…  │
│ 142        IN_PROGRESS    2026-03-22 03:15       2m 47s       a1b2c3d4   CodeP…  │
│ 141        SUCCEEDED      2026-03-22 02:00       4m 12s       e5f6a7b8   CodeP…  │
│ 140        SUCCEEDED      2026-03-21 18:30       3m 58s       c9d0e1f2   CodeP…  │
│ 139        FAILED         2026-03-21 15:45       1m 03s       34a5b6c7   user/…  │
│ 138        SUCCEEDED      2026-03-21 14:00       4m 22s       d8e9f0a1   CodeP…  │
│ 137        STOPPED        2026-03-21 12:00       0m 45s       2b3c4d5e   user/…  │
│ 136        SUCCEEDED      2026-03-21 10:30       4m 08s       f1e2d3c4   CodeP…  │
│   · · · (18 more)                                                                │
└──────────────────────────────────────────────────────────────────────────────────┘
```

Row coloring by build status (entire row):
- `SUCCEEDED`: GREEN `#9ece6a`
- `FAILED`: RED `#f7768e`
- `FAULT`: RED `#f7768e`
- `TIMED_OUT`: RED `#f7768e`
- `IN_PROGRESS`: YELLOW `#e0af68`
- `STOPPED`: DIM `#565f89`

Selected row: full-width blue background overrides status coloring.

### Copy Behavior

`c` copies the build ID (e.g., `payment-api-build:a1b2c3d4-e5f6-7890-abcd-ef1234567890`).

### Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ BUILDS                GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <enter> View Logs     <q>      Quit        <k>       Up         <:>   Command   │
│ <d>     Detail        </>      Filter      <g>       Top                        │
│ <y>     YAML          <:>      Command     <G>       Bottom                     │
│ <c>     Copy Build ID                      <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```

---

## Level 2: Builds --> Build Logs (Cross-Service to CloudWatch)

### Navigation

- **Entry:** Press Enter on a build in the Builds list
- **Frame title:** `build-logs(240) — #139`
- **View stack:** CodeBuild --> Builds --> Build Logs --> (detail via d)
- **Esc** returns to Builds list
- **New key bindings:**
  - `w` — toggle word wrap

### views.yaml

```yaml
cb_build_logs:
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

Note: Same structure as other log views. `width: 0` fills remaining width.

### AWS API

- The build's `Logs.GroupName` and `Logs.StreamName` fields (from BatchGetBuilds response) point to the CloudWatch log stream
- `logs:GetLogEvents` with the extracted `logGroupName` and `logStreamName`
- Paginated via `nextForwardToken`/`nextBackwardToken`
- **Latency warning:** Build logs can be large (thousands of lines). Initial fetch returns ~1MB. Show spinner.
- **Edge case:** If `Logs.GroupName` is empty (custom log config or logs disabled), show: "Build logs not available in CloudWatch."

### ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌────────────────── build-logs(240) — #139 ──────────────────────────────────────┐
│ TIMESTAMP              MESSAGE                                                  │
│ 2026-03-21 15:45:00    [Container] 2026/03/21 15:45:00 Entering phase DOWNLO…  │
│ 2026-03-21 15:45:02    [Container] 2026/03/21 15:45:02 Phase complete: DOWNLO…  │
│ 2026-03-21 15:45:02    [Container] 2026/03/21 15:45:02 Entering phase INSTALL   │
│ 2026-03-21 15:45:05    [Container] 2026/03/21 15:45:05 Running command npm ci   │
│ 2026-03-21 15:45:30    [Container] 2026/03/21 15:45:30 Phase complete: INSTAL…  │
│ 2026-03-21 15:45:30    [Container] 2026/03/21 15:45:30 Entering phase BUILD    │
│ 2026-03-21 15:45:31    [Container] 2026/03/21 15:45:31 Running command npm te…  │
│ 2026-03-21 15:46:00    FAIL src/payment.test.ts                                 │
│ 2026-03-21 15:46:00      Expected: 200                                          │
│ 2026-03-21 15:46:00      Received: 500                                          │
│ 2026-03-21 15:46:03    [Container] 2026/03/21 15:46:03 Command did not exit s…  │
│   · · · (229 more)                                                              │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Row coloring by log content (entire row):
- Lines containing `FAIL`, `ERROR`, `error`, `Error`, `did not exit successfully`: RED `#f7768e`
- Lines containing `Phase complete`, `SUCCEEDED`: GREEN `#9ece6a`
- Lines containing `Entering phase`, `Running command`: YELLOW `#e0af68`
- All other lines: PLAIN `#c0caf5`

### Copy Behavior

`c` copies the full message text of the selected log line.

### Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ BUILD LOGS            GENERAL              NAVIGATION           HOTKEYS         │
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
