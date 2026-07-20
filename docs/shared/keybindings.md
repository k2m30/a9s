### Navigation

| Key | Action |
|-----|--------|
| `j` / `Down` | Move down |
| `k` / `Up` | Move up |
| `g` | Go to top |
| `G` | Go to bottom |
| `Enter` | Open / select |
| `Esc` | Back / close |
| `h` / `Left` | Scroll left |
| `l` / `Right` | Scroll right |
| `PgUp` / `Ctrl+U` | Page up |
| `PgDn` / `Ctrl+D` | Page down |

### Actions

| Key | Action |
|-----|--------|
| `d` | Detail view |
| `y` | YAML view |
| `J` | JSON view |
| `x` | Reveal (expand) |
| `c` | Copy resource ID to clipboard |
| `o` | Open the selected resource in the AWS console (browser) |
| `O` | Copy the AWS console URL to clipboard |
| `i` | IAM identity view |
| `/` | Filter |
| `Ctrl+Z` | Show only attention-worthy rows (resource lists) / filter to types with issues (main menu) |
| `:` | Command mode |
| `?` | Help |
| `Ctrl+R` | Refresh |
| `t` | Jump to CloudTrail Events for the selected resource (all resource types) |
| `e` | Open Service Events (ECS Services) / Agreements (Transfer Family) |
| `L` | Open Container Logs (ECS Services) |
| `m` | Load more (paginated lists, also in demo mode) |
| `R` | Open Stack Resources (CFN Stacks) |
| `s` | Open source view (reserved for future child views) |
| `w` | Toggle line wrap (in YAML, JSON, detail, and reveal views) |
| `Tab` | Autocomplete (in command mode) / Switch focus (in detail view with related panel) |

### Cost Explorer (`:costs`)

| Key | Action |
|-----|--------|
| `Enter` | Drill into the selected cell (service → usage type → resources → resource detail) |
| `Esc` | Back out one drill level, then back to the menu |
| `+` / `=` | Zoom in (year → month → week → day, anchored at the cursor) |
| `-` / `_` | Zoom out |
| `b` | Cycle cost metric (invoice → unblended → amortized → net amortized → blended) |
| `1`-`6` | Pivot rows: service, region, account, usage type, purchase option, charge category |
| `0` | Reset to the default view |
| `Ctrl+R` | Force-refresh the open period |

Rows sort by total spend across the visible window, largest absolute value
first. Scrolling left past the oldest loaded column fetches older history
(up to the Cost Explorer 13-month horizon). Resource-level drilling covers
the last 14 days and, per AWS, EC2 only — other services explain the
limitation in the footer.

### Related Resources (Detail View)

| Key | Action |
|-----|--------|
| `r` | Toggle related resources panel |
| `Tab` | Switch focus between detail content and related panel |
| `Enter` | Navigate to related resource (on navigable field or panel row) |

### Search (Detail, YAML, and JSON Views)

| Key | Action |
|-----|--------|
| `/` | Start search |
| `n` | Next match |
| `N` | Previous match |
| `Enter` | Confirm search (keep highlights) |
| `Esc` | Clear search |

### Sorting

| Key | Action |
|-----|--------|
| `1`-`0` | Sort by column position (1=first, 0=tenth) |

### General

| Key | Action |
|-----|--------|
| `!` | Error log (session errors with timestamps) |
| `q` | Quit |
| `Ctrl+C` | Force quit |

### Visual Indicators

a9s surfaces background-health findings without making write calls. Markers and badges let you spot resources that need attention at a glance.

| Marker | Meaning |
|--------|---------|
| `!` prefix on a row | Broken / degraded (e.g. failed build, impaired volume, unhealthy target) |
| `~` prefix on a row | Informational / scheduled (e.g. pending maintenance, non-urgent event) |
| `issues:N` on main menu | `N` distinct resources of this type have an active finding |

In the detail view, every Wave-1 warning and Wave-2 enrichment finding for the selected resource renders as an entry in a unified `Attention (N)` section at the top of the view. Each entry is prefixed with `!` (Broken) or `~` (Warning) and lists its supporting rows beneath. Press `Ctrl+Z` on the main menu to filter to only types with findings, or on a list to show only affected rows.
