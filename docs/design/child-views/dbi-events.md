# Child View: RDS Instances --> Recent Events

**Status:** Planned
**Tier:** SHOULD-HAVE

---

## Navigation

- **Entry:** Press Enter on an RDS instance in the RDS Instances list
- **Frame title:** `rds-events(18) — prod-payments-db`
- **View stack:** RDS Instances --> RDS Events --> (detail/YAML via d/y)
- **Esc** returns to RDS Instances list
- **No new key bindings** beyond the standard set

## views.yaml

```yaml
dbi_events:
  list:
    Timestamp:
      path: Date
      width: 22
    Category:
      key: event_categories
      width: 18
    Message:
      path: Message
      width: 60
  detail:
    - Date
    - SourceIdentifier
    - SourceType
    - EventCategories
    - SourceArn
    - Message
```

Note on computed fields:
- `event_categories`: joined string from `EventCategories[]` (e.g., "availability", "failover", "maintenance")

Source struct: `rdstypes.Event`

## AWS API

- `rds:DescribeEvents` with `SourceIdentifier` = DB instance identifier, `SourceType` = `db-instance`
- Optional: `Duration` parameter (in minutes) to limit time range. Default to 10080 (7 days) for a useful operational window. AWS retains events for up to 14 days.
- Paginated via `Marker`
- **Latency:** Fast (<1 second). Most instances have 5-50 events in a 7-day window.

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌────────────── rds-events(18) — prod-payments-db ───────────────────────────────┐
│ TIMESTAMP              CATEGORY           MESSAGE                               │
│ 2026-03-22 03:00       maintenance        Finished applying modification to d…  │
│ 2026-03-22 02:55       maintenance        Applying modification to database i…  │
│ 2026-03-22 02:50       maintenance        Multi-AZ instance failover started.   │
│ 2026-03-22 02:50       availability       Multi-AZ instance failover complete…  │
│ 2026-03-22 02:45       notification       DB instance is being rebooted for m…  │
│ 2026-03-20 14:00       notification       Automated backup completed.           │
│ 2026-03-19 14:00       notification       Automated backup completed.           │
│ 2026-03-18 08:00       configuration ch…  Updated to use DBParameterGroup pro…  │
│   · · · (10 more)                                                               │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Row coloring by event category (entire row):
- `failover`, `failure`: RED `#f7768e` — something broke or failed over
- `maintenance`, `recovery`: YELLOW `#e0af68` — planned/unplanned maintenance
- `availability` containing "failover complete" or "recovery": GREEN `#9ece6a`
- `notification` (backups, routine): PLAIN `#c0caf5`
- `configuration change`: DIM `#565f89` — informational

Selected row: full-width blue background overrides category-based coloring.

## Copy Behavior

`c` copies the full Message text — the event description is what you paste into an incident report to explain what RDS was doing.

## Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ RDS EVENTS            GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy Message                       <G>       Bottom                     │
│                                            <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```
