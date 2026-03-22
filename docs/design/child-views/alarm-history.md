# Child View: CloudWatch Alarms --> Alarm History

**Status:** Planned
**Tier:** SHOULD-HAVE (Structural)

---

## Navigation

- **Entry:** Press Enter on an alarm in the CloudWatch Alarms list
- **Frame title:** `alarm-history(34) — api-error-rate-high`
- **View stack:** Alarms --> Alarm History --> (detail/YAML via d/y)
- **Esc** returns to Alarms list
- **No new key bindings** beyond the standard set

## views.yaml

```yaml
alarm_history:
  list:
    Timestamp:
      path: Timestamp
      width: 22
    Type:
      path: HistoryItemType
      width: 18
    Summary:
      path: HistorySummary
      width: 60
  detail:
    - Timestamp
    - HistoryItemType
    - HistorySummary
    - HistoryData
    - AlarmName
    - AlarmType
```

Source struct: `cwtypes.AlarmHistoryItem`

## AWS API

- `cloudwatch:DescribeAlarmHistory` with `AlarmName`, `AlarmTypes=["MetricAlarm","CompositeAlarm"]`
- Paginated via `NextToken`
- **Retention:** AWS retains alarm history for 30 days
- **Latency:** Fast (<1 second). Typical alarms have 10-100 history items.
- **Optional filter:** `HistoryItemType` can filter to `StateUpdate` only (most useful during incidents) or `Action` (to see notification deliveries)

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌──────────── alarm-history(34) — api-error-rate-high ───────────────────────────┐
│ TIMESTAMP              TYPE               SUMMARY                               │
│ 2026-03-22 02:52       StateUpdate        Alarm updated from ALARM to OK        │
│ 2026-03-22 02:52       Action             Successfully executed action arn:aw…  │
│ 2026-03-22 02:47       StateUpdate        Alarm updated from OK to ALARM        │
│ 2026-03-22 02:47       Action             Successfully executed action arn:aw…  │
│ 2026-03-21 14:30       StateUpdate        Alarm updated from ALARM to OK        │
│ 2026-03-21 14:30       Action             Successfully executed action arn:aw…  │
│ 2026-03-21 14:22       StateUpdate        Alarm updated from OK to ALARM        │
│   · · · (27 more)                                                               │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Row coloring by history item type and content (entire row):
- `StateUpdate` containing "to ALARM": RED `#f7768e`
- `StateUpdate` containing "to OK": GREEN `#9ece6a`
- `StateUpdate` containing "to INSUFFICIENT_DATA": YELLOW `#e0af68`
- `Action` containing "Successfully": PLAIN `#c0caf5`
- `Action` containing "Failed": RED `#f7768e`
- `ConfigurationUpdate`: DIM `#565f89`

Selected row: full-width blue background overrides content-based coloring.

## Copy Behavior

`c` copies the HistorySummary text of the selected entry.

## Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ ALARM HISTORY         GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy Summary                       <G>       Bottom                     │
│                                            <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```
