# EC2 Instance Status Checks — Design Spec

Issue: [#188](https://github.com/k2m30/a9s/issues/188)
Version: 1.2
Related: [#196](https://github.com/k2m30/a9s/issues/196) (resource issues overlay),
[`deterministic-issue-signals.md`](deterministic-issue-signals.md)

---

## 1. Problem

The AWS Console shows Status Checks as a prominent column in the EC2 instance
list — "2/2 checks passed", "initializing", etc. a9s lacks this. A user sees
an instance as "running" but can't SSH in. In the console, the answer is
obvious — status checks show "initializing" or "1/2 checks passed". In a9s,
there's no indication.

---

## 2. Design Principle

**Silence means healthy.** When all checks pass (2/2), show nothing extra.
When checks fail or are initializing, show an inline warning indicator in
the list row. Full details are in the detail view.

This aligns with the issue-signals framework (#196): EC2 status checks are
an L2 enrichment signal (requires one extra API call). The visual indicator
in the list view is the first instance of row-level issue marking.

---

## 3. UX Decisions

### 3.1 List View — Inline Warning Indicator

No extra column. Instead, prepend a warning glyph to the **STATE** cell
when status checks are not fully passing on a `running` instance.

| Condition                          | STATE Cell Renders As | Color     |
|------------------------------------|-----------------------|-----------|
| running, 2/2 checks passed        | `running`             | GREEN     |
| running, 1/2 or 0/2 checks        | `! running`           | RED       |
| running, initializing              | `~ running`           | YELLOW    |
| running, insufficient-data         | `running`             | GREEN     |
| pending (any checks)               | `pending`             | YELLOW    |
| stopped (checks not applicable)    | `stopped`             | RED       |
| terminated                         | `terminated`          | DIM       |
| checks not yet fetched             | `running`             | GREEN     |

Key decisions:
- **`!`** prefix for failed checks — ASCII, visible in any terminal, no Unicode risk
- **`~`** prefix for initializing — lighter than `!`, signals "not yet ready"
- No indicator when healthy or when checks haven't loaded yet (avoids flash of
  warning followed by disappearing indicator)
- No indicator for non-running states — stopped/terminated instances don't have
  meaningful status checks
- `insufficient-data` is treated as healthy for display — it's a transient AWS
  state that resolves quickly, not an actionable signal

The `!` and `~` glyphs use their own foreground color (RED or YELLOW),
independent of the row status color. On a selected row (blue background),
the glyph retains its color for contrast.

The STATE column width stays at 12 — `! running` is 9 chars, fits with room.

### 3.2 Detail View — Status Checks Section

Add a **Status Checks** section to the EC2 detail view, placed immediately
after the `State` section:

```
 State:
     Code: "16"
     Name: running
 Status Checks:
     System:   ok
     Instance: ok
```

Section is omitted entirely when:
- Status check data hasn't been fetched yet (keep detail clean, no placeholders)
- Instance is not running (stopped/terminated have no meaningful checks)

Status values are color-coded:
- `ok` — GREEN `#9ece6a`
- `impaired` — RED `#f7768e`
- `initializing` — YELLOW `#e0af68`
- `insufficient-data` — DIM `#565f89`
- `not-applicable` — DIM `#565f89`

### 3.3 Fetch Strategy — Per-Page Background Enrichment

After each page of `DescribeInstances` loads, fire a background
`DescribeInstanceStatus` call with the current page's instance IDs.
Merge results directly into `Resource.Fields`.

```
DescribeInstanceStatus
  InstanceIds: [current page instance IDs]
  IncludeAllInstances: true
```

`IncludeAllInstances: true` ensures we get results for all instances on the
page, not just running ones. Stopped/terminated instances return
`not-applicable` — we simply ignore those during rendering.

Two API calls per page total: `DescribeInstances` (already exists) +
`DescribeInstanceStatus` (new, background). At most 200 instance IDs per
page — if that exceeds the 100-instance limit per `DescribeInstanceStatus`
call, paginate with `NextToken`.

### 3.4 Fetch Flow

```
1. DescribeInstances returns page → list renders immediately, no indicators

2. Background Cmd fires:
     DescribeInstanceStatus(InstanceIds: pageInstanceIDs, IncludeAllInstances: true)

3. StatusChecksLoadedMsg arrives → merge system_status + instance_status
   into Resource.Fields for each instance on the page

4. List re-renders:
   - Fields["system_status"] or Fields["instance_status"] == "impaired" → ! prefix
   - Fields["system_status"] or Fields["instance_status"] == "initializing" → ~ prefix
   - Otherwise → no indicator
```

### 3.5 Refresh Behavior

- `Ctrl+R`: re-fetches both `DescribeInstances` and `DescribeInstanceStatus`.
  Indicators disappear during refresh (correct — silence means healthy until
  proven otherwise).
- Page navigation: new page triggers a new `DescribeInstanceStatus` call for
  that page's instance IDs.
- No auto-poll. Manual refresh only.

---

## 4. ASCII Wireframes

### 4.1 List View — Mixed Status Checks

```
 a9s v0.8.0  prod:us-east-1                                                                          ? for help
┌─────────────────────────────────── ec2-instances(42) ──────────────────────────────────────────────────────────┐
│ NAME                    STATE        LIFECYCLE     TYPE           PRIVATE IP        INSTANCE ID                │
│ api-prod-01             running      on-demand     t3.medium      10.0.1.42         i-0abc123def456789a        │
│ api-prod-02             running      on-demand     t3.medium      10.0.1.43         i-0abc123def456789b        │
│ worker-01               ! running    on-demand     t3.large       10.0.2.10         i-0def456789abcdef0        │
│ worker-02               ~ running    on-demand     t3.large       10.0.2.11         i-0def456789abcdef1        │
│ bastion                 running      on-demand     t2.micro       10.0.0.5          i-0111222333aaabbb0        │
│ old-worker              stopped      on-demand     t3.medium      10.0.1.99         i-0aaa111222bbbccc0        │
│ legacy-app              terminated   on-demand     t2.small       -                 i-0000111222cccddd0        │
│   · · · (35 more)                                                                                              │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

Row colors:
- `api-prod-01`, `api-prod-02`, `bastion`: GREEN row, no indicator (healthy or not in problem set)
- `worker-01`: GREEN row text, but `!` prefix in **RED** — system or instance check impaired
- `worker-02`: GREEN row text, but `~` prefix in **YELLOW** — checks still initializing
- `old-worker`: RED row (stopped), no indicator
- `legacy-app`: DIM row (terminated), no indicator

### 4.2 List View — All Healthy (common case)

```
┌─────────────────────────────────── ec2-instances(42) ──────────────────────────────────────────────────────────┐
│ NAME                    STATE        LIFECYCLE     TYPE           PRIVATE IP        INSTANCE ID                │
│ api-prod-01             running      on-demand     t3.medium      10.0.1.42         i-0abc123def456789a        │
│ api-prod-02             running      on-demand     t3.medium      10.0.1.43         i-0abc123def456789b        │
│ worker-01               running      on-demand     t3.large       10.0.2.10         i-0def456789abcdef0        │
│ worker-02               running      on-demand     t3.large       10.0.2.11         i-0def456789abcdef1        │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

No visual noise. Identical to the current view when everything is healthy.

### 4.3 List View — Selected Row with Failed Check

When `worker-01` is selected (blue background):

```
│ api-prod-01             running      on-demand     t3.medium      10.0.1.42         i-0abc123def456789a        │
│ worker-01               ! running    on-demand     t3.large       10.0.2.10         i-0def456789abcdef0        │
│ worker-02               ~ running    on-demand     t3.large       10.0.2.11         i-0def456789abcdef1        │
```

- `worker-01` selected: blue background, dark foreground, but `!` retains RED foreground
- Ensures the warning is visible even on the selected row

### 4.4 Detail View — Failed Checks

```
┌──────────────────── i-0def456789abcdef0 ───────────────────────────────────┐
│ InstanceId:           i-0def456789abcdef0                                   │
│ State:                                                                       │
│     Code: "16"                                                               │
│     Name: running                                                            │
│ Status Checks:                                                               │
│     System:   ok                                                             │
│     Instance: impaired                                                       │
│ InstanceType:         t3.large                                               │
│ ImageId:              ami-0abcdef01234567                                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

- `System: ok` in GREEN
- `Instance: impaired` in RED

### 4.5 Detail View — Initializing

```
│ Status Checks:                                                               │
│     System:   initializing                                                   │
│     Instance: initializing                                                   │
```

Both in YELLOW.

### 4.6 Detail View — Healthy (no section shown)

```
┌──────────────────── i-0abc123def456789a ───────────────────────────────────┐
│ InstanceId:           i-0abc123def456789a                                   │
│ State:                                                                       │
│     Code: "16"                                                               │
│     Name: running                                                            │
│ InstanceType:         t3.medium                                              │
│ ImageId:              ami-0abcdef01234567                                    │
└─────────────────────────────────────────────────────────────────────────────┘
```

No `Status Checks` section. Silence means healthy.

---

## 5. Color Palette

No new colors. Reuses existing Tokyo Night Dark palette:

| Element                        | Foreground | Style  | Constant     |
|--------------------------------|------------|--------|--------------|
| `!` glyph (failed checks)     | `#f7768e`  | bold   | `ColStopped` |
| `~` glyph (initializing)      | `#e0af68`  | normal | `ColPending` |
| Detail `ok`                    | `#9ece6a`  | normal | `ColRunning` |
| Detail `impaired`              | `#f7768e`  | normal | `ColStopped` |
| Detail `initializing`          | `#e0af68`  | normal | `ColPending` |
| Detail `insufficient-data`     | `#565f89`  | dim    | `ColDim`     |
| Detail `not-applicable`        | `#565f89`  | dim    | `ColDim`     |

---

## 6. Data Model — Merge Into Resource.Fields

Status check results are merged directly into `Resource.Fields` for each
instance on the current page. No separate data structure.

| Field Key          | Source                            | Stored In          |
|--------------------|-----------------------------------|--------------------|
| `system_status`    | `SystemStatus.Status`             | `Resource.Fields`  |
| `instance_status`  | `InstanceStatus.Status`           | `Resource.Fields`  |

These fields are populated by `StatusChecksLoadedMsg` and read by:
- List view: to decide whether to prepend `!` or `~` to the STATE cell
- Detail view: to render the `Status Checks` section

On page navigation, the new page's instances start without these fields
(no indicators), then the background call populates them.

---

## 7. New Message Type

| Msg                       | Payload                          | Trigger                        |
|---------------------------|----------------------------------|--------------------------------|
| `StatusChecksLoadedMsg`   | `map[string]StatusCheckResult`   | Background fetch completes     |

```
StatusCheckResult {
    SystemStatus   string  // "ok", "impaired", "initializing", "not-applicable", ...
    InstanceStatus string  // same enum
}
```

The map contains entries for all instances on the current page.

---

## 8. State Transitions

```
ResourcesLoadedMsg (EC2 page)
  → List renders normally (no indicators)
  → Background Cmd fires: fetchStatusChecks(pageInstanceIDs)
      DescribeInstanceStatus(InstanceIds: [...], IncludeAllInstances: true)

StatusChecksLoadedMsg
  → Merges system_status + instance_status into Resource.Fields
  → List re-renders; rows with impaired/initializing gain ! or ~ prefix

KeyMsg(ctrl+r)
  → Re-fetches DescribeInstances (clears Fields)
  → Background Cmd re-fires for status checks on the new page

Page navigation
  → New page loads → new background status check call for that page's IDs
```

---

## 9. Rendering Logic (List View)

When rendering the STATE cell for an EC2 resource:

```
state      = resource.Fields["state"]
sysStatus  = resource.Fields["system_status"]
instStatus = resource.Fields["instance_status"]

if state == "running" {
    if sysStatus == "impaired" || instStatus == "impaired" {
        return RED_BOLD("!") + " " + state
    }
    if sysStatus == "initializing" || instStatus == "initializing" {
        return YELLOW("~") + " " + state
    }
}
return state  // healthy, non-running, or not yet enriched
```

The `!` / `~` glyph is rendered with its own `lipgloss.Style` foreground,
separate from the row-level status color style.

The detail view reads the same fields:

```
sysStatus  = resource.Fields["system_status"]
instStatus = resource.Fields["instance_status"]
if sysStatus != "" {
    // Render "Status Checks:" section with sysStatus, instStatus
}
// Otherwise: omit the section entirely (not yet fetched or not applicable)
```

---

## 10. Relationship to Issue Signals (#196)

This feature is the first concrete instance of the L2 enrichment pattern
from the resource-issues-overlay design:

- Signal family: Health-Check Aggregate (section 3.3 of deterministic-issue-signals.md)
- Detection level: L2 Enriched (one extra API call)
- Severity: `Issue` for impaired checks, `Warning` for initializing

When #196 ships its `!` toggle and `issues:N` frame title, EC2 status checks
should plug in naturally:
- `!` mode filters to instances with impaired checks (hard issue)
- `!!` mode adds instances with initializing checks (warning)
- `issues:N` count includes instances with `!` prefix

The `!` glyph prefix chosen here is intentionally the same character as the
issues toggle key, reinforcing the visual language.

---

## 11. Demo Mode

Demo fixtures set `system_status` and `instance_status` fields on EC2 instances:

| Instance      | State       | system_status  | instance_status | List View       |
|---------------|-------------|----------------|-----------------|-----------------|
| api-prod-01   | running     | ok             | ok              | `running`       |
| api-prod-02   | running     | ok             | ok              | `running`       |
| worker-01     | running     | ok             | impaired        | `! running`     |
| worker-02     | running     | initializing   | initializing    | `~ running`     |
| bastion       | running     | ok             | ok              | `running`       |
| old-worker    | stopped     | (empty)        | (empty)         | `stopped`       |
| legacy-app    | terminated  | (empty)        | (empty)         | `terminated`    |

---

## 12. Scope Boundaries

This design does NOT:
- Add a new column (too expensive in horizontal space)
- Auto-poll status checks (manual Ctrl+R only)
- Show indicators for non-running instances
- Show a loading state (`...`) — silence until data arrives
- Require changes to `app.go`, message routing, or key bindings
- Affect any resource type other than EC2
