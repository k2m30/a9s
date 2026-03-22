# Child View: IAM Groups --> Group Members

**Status:** Planned
**Tier:** SHOULD-HAVE (Structural)

---

## Navigation

- **Entry:** Press Enter on a group in the IAM Groups list
- **Frame title:** `group-members(4) — Admins`
- **View stack:** IAM Groups --> Group Members --> (detail/YAML via d/y)
- **Esc** returns to IAM Groups list
- **No new key bindings** beyond the standard set

## views.yaml

```yaml
iam_group_members:
  list:
    User Name:
      path: UserName
      width: 28
    User ID:
      path: UserId
      width: 24
    Created:
      path: CreateDate
      width: 22
    Password Last Used:
      path: PasswordLastUsed
      width: 22
  detail:
    - UserName
    - UserId
    - Arn
    - Path
    - CreateDate
    - PasswordLastUsed
    - Tags
```

Source struct: `iamtypes.User` (returned by `GetGroup`)

## AWS API

- `iam:GetGroup` with `GroupName`
- Paginated via `Marker` / `IsTruncated` — returns the group info and a list of `Users[]`
- **Latency:** Fast (<1 second). Groups typically have 1-20 members.

## ASCII Wireframe

```
 a9s v0.5.0  prod:us-east-1                                              ? for help
┌────────────────── group-members(4) — Admins ───────────────────────────────────┐
│ USER NAME                    USER ID                  CREATED                P…  │
│ jsmith                       AIDA1234567890ABCDEF1    2024-06-15 09:00       2…  │
│ akumar                       AIDA1234567890ABCDEF2    2024-08-20 14:30       2…  │
│ mchen                        AIDA1234567890ABCDEF3    2025-01-10 10:00       2…  │
│ departed-employee            AIDA1234567890ABCDEF4    2023-03-01 08:00       2…  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Scrolled right to show Password Last Used:
```
│ CREATED                PASSWORD LAST USED     │
│ 2024-06-15 09:00       2026-03-22 01:30       │
│ 2024-08-20 14:30       2026-03-21 18:00       │
│ 2025-01-10 10:00       2026-03-22 02:15       │
│ 2023-03-01 08:00       2025-09-15 11:00       │
```

Row coloring:
- Users whose `PasswordLastUsed` is more than 90 days ago: YELLOW `#e0af68` (stale credentials, security concern)
- Users whose `PasswordLastUsed` is null/never: DIM `#565f89` (service accounts or never-used credentials)
- All other users: PLAIN `#c0caf5`

The 90-day stale credential highlighting is a deliberate security-conscious design choice. During an audit, the question "who in this group hasn't logged in recently?" is answered instantly by the row color.

Selected row: full-width blue background overrides all coloring.

## Copy Behavior

`c` copies the User Name — the IAM user identifier for cross-referencing or searching.

## Help Screen

```
┌──────────────────────────────── Help ───────────────────────────────────────────┐
│ GROUP MEMBERS         GENERAL              NAVIGATION           HOTKEYS         │
│                                                                                 │
│ <esc>   Back          <ctrl-r> Refresh     <j>       Down       <?>   Help      │
│ <d>     Detail        </>      Filter      <k>       Up         <:>   Command   │
│ <y>     YAML          <:>      Command     <g>       Top                        │
│ <c>     Copy User                          <G>       Bottom                     │
│                                            <h/l>     Cols                       │
│                                            <pgup/dn> Page                       │
│                                                                                 │
│                       Press any key to close                                    │
└─────────────────────────────────────────────────────────────────────────────────┘
```
