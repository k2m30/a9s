# EC2 Instance Status Checks — Design Spec

Issue: [#188](https://github.com/k2m30/a9s/issues/188)
Version: 1.1
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

### 3.3 Fetch Strategy — Filter-Based Background Enrichment

`DescribeInstanceStatus` supports **server-side filters** on status values.
Instead of fetching checks for every instance, we ask AWS to return only
instances with problems.

**Two targeted calls, fired in parallel after the instance list loads:**

```
Call 1: DescribeInstanceStatus
  Filters: [{ Name: "instance-status.status",  Values: ["impaired"] },
            { Name: "system-status.status",     Values: ["impaired"] }]
  → Returns only instances where at least one check is impaired

Call 2: DescribeInstanceStatus
  Filters: [{ Name: "instance-status.status",  Values: ["initializing"] },
            { Name: "system-status.status",     Values: ["initializing"] }]
  → Returns only instances where checks are still initializing
```

**Why this is better than fetching all instance IDs:**
- **Common case is free**: healthy fleet → both calls return 0 results
- **No instance ID list needed**: filters are account-wide, no dependency on
  which page of instances is loaded
- **Tiny response**: typically 0-5 instances, not hundreds
- **No pagination headaches**: result set is almost always under the page limit
- **Works with paginated instance list**: enrichment is independent of which
  DescribeInstances page the user is viewing

**Single call alternative (simpler):**

Actually, a single call with `IncludeAllInstances: false` and **no filters**
could also work — it returns all instances where status is NOT `ok`. But the
filter approach is more explicit and avoids returning `insufficient-data`
instances that we'd discard anyway.

**Chosen approach: single call with exclusion filter.**

```
DescribeInstanceStatus
  IncludeAllInstances: false   (default — only running instances)
  Filters: [{ Name: "instance-status.status", Values: ["impaired", "initializing"] }]

  + second call:
  Filters: [{ Name: "system-status.status", Values: ["impaired", "initializing"] }]
```

Wait — the filters are OR within a single filter name but AND across filter
names. To catch "system OR instance is bad" we need either:
- Two separate calls (one filtering system-status, one filtering instance-status)
- One call with no filters, then client-side filtering

**Final decision: one unfiltered call, client-side filter.**

```
DescribeInstanceStatus
  IncludeAllInstances: false
  (no filters — returns all running instances with ANY non-ok status)
```

This returns instances where system OR instance status is not `ok`. The
response typically contains only instances with `impaired` or `initializing`
status. In a healthy fleet, this returns an empty list. We filter out
`insufficient-data` client-side (treated as healthy).

This is the simplest approach: one API call, zero results for healthy fleets,
a handful of results when something is wrong.

### 3.4 Fetch Flow

```
1. DescribeInstances returns page → list renders immediately, no indicators

2. Background Cmd fires:
     DescribeInstanceStatus(IncludeAllInstances: false)
   This returns ONLY instances where system or instance status != ok.

3. StatusChecksLoadedMsg arrives with map[instanceID]→{system, instance}
   Only problematic instances are in the map.

4. List re-renders:
   - Instance ID in the map with "impaired" → ! prefix
   - Instance ID in the map with "initializing" → ~ prefix
   - Instance ID NOT in the map → no indicator (healthy)
```

### 3.5 Refresh Behavior

- `Ctrl+R`: re-fetches both `DescribeInstances` and `DescribeInstanceStatus`.
  Indicators disappear during refresh (correct — silence means healthy until
  proven otherwise).
- No auto-poll. Manual refresh only.

### 3.6 Cost Analysis

| Scenario                     | API Calls | Response Size |
|------------------------------|-----------|---------------|
| Healthy fleet (common)       | 1         | 0 instances   |
| 2 instances with issues      | 1         | 2 instances   |
| 50 instances initializing    | 1 (maybe paginated) | 50 instances |

Compare with the "fetch all" approach: 1 call per page × N pages, returning
all running instances regardless of health. The filter approach is strictly
cheaper for the common case.

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

## 6. Data Model — Separate Problem Set (Not Merged Into Resources)

The status checks result is **not merged** into `Resource.Fields`. Instead,
it is stored as a separate lookup map on the view model:

```
statusChecks map[string]StatusCheckResult   // instance ID → problem info
```

This map only contains entries for problematic instances (typically 0-50).
The paginated resource list is untouched.

**Why separate, not merged:**
- The EC2 list is paginated. Merging into `Resource.Fields` would only
  annotate instances on the current page. But status checks are account-wide
  and cheap — the problem set applies across all pages.
- On page navigation, merged fields would be lost and need re-merging.
- Keeping the problem set separate means it survives page changes and only
  needs re-fetching on `Ctrl+R`.
- The detail view reads from the same map when rendering the Status Checks
  section.

**Lookup keys:**

| Field               | Source                          | Stored In                          |
|---------------------|---------------------------------|------------------------------------|
| `SystemStatus`      | `SystemStatus.Status`           | `statusChecks[id].SystemStatus`    |
| `InstanceStatus`    | `InstanceStatus.Status`         | `statusChecks[id].InstanceStatus`  |

---

## 7. New Message Type

| Msg                       | Payload                          | Trigger                        |
|---------------------------|----------------------------------|--------------------------------|
| `StatusChecksLoadedMsg`   | `map[string]StatusCheckResult`   | Background fetch completes     |

```
StatusCheckResult {
    SystemStatus   string  // "impaired", "initializing"
    InstanceStatus string  // "impaired", "initializing"
}
```

The map only contains entries for problematic instances. An empty map means
the entire fleet is healthy.

---

## 8. State Transitions

```
ResourcesLoadedMsg (EC2 page)
  → List renders normally (no indicators)
  → Background Cmd fires: fetchProblematicStatusChecks()
      DescribeInstanceStatus(IncludeAllInstances: false)
      Returns only instances where status != ok

StatusChecksLoadedMsg
  → Stores the problem map as a separate lookup: statusChecks map[id]→result
  → List re-renders; renderer checks each row's instance ID against the map
  → Rows found in the map gain ! or ~ prefix

KeyMsg(ctrl+r)
  → Clears the statusChecks map
  → Re-fetches DescribeInstances AND re-fires status checks background Cmd

Page navigation (next/prev page of EC2 instances)
  → statusChecks map is RETAINED — it's account-wide, not page-specific
  → New page rows are checked against the existing map immediately
```

---

## 9. Rendering Logic (List View)

When rendering the STATE cell for an EC2 resource:

```
state = resource.Fields["state"]
id    = resource.ID

// Look up in the separate problem set (not in Resource.Fields)
check, hasProblem = statusChecks[id]

if state == "running" && hasProblem {
    if check.SystemStatus == "impaired" || check.InstanceStatus == "impaired" {
        return RED_BOLD("!") + " " + state    // failed check
    }
    if check.SystemStatus == "initializing" || check.InstanceStatus == "initializing" {
        return YELLOW("~") + " " + state       // still initializing
    }
}
return state  // healthy (not in map), non-running, or not yet enriched
```

The `!` / `~` glyph is rendered with its own `lipgloss.Style` foreground,
separate from the row-level status color style.

The detail view uses the same lookup:

```
check, hasProblem = statusChecks[resource.ID]
if hasProblem {
    // Render "Status Checks:" section with check.SystemStatus, check.InstanceStatus
}
// Otherwise: omit the section entirely (silence means healthy)
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

Demo mode provides a separate `statusChecks` map containing only problematic
instances. Healthy instances are absent from the map.

**Problem set map (2 entries):**

| Instance ID              | System Status  | Instance Status |
|--------------------------|----------------|-----------------|
| i-0def456789abcdef0      | ok             | impaired        |
| i-0def456789abcdef1      | initializing   | initializing    |

**EC2 instance list (unchanged):**

| Instance      | State       | Effect in List View             |
|---------------|-------------|---------------------------------|
| api-prod-01   | running     | No indicator (not in map)       |
| api-prod-02   | running     | No indicator (not in map)       |
| worker-01     | running     | `! running` (found in map: impaired) |
| worker-02     | running     | `~ running` (found in map: initializing) |
| bastion       | running     | No indicator (not in map)       |
| old-worker    | stopped     | No indicator (not in map)       |
| legacy-app    | terminated  | No indicator (not in map)       |

---

## 12. Scope Boundaries

This design does NOT:
- Add a new column (too expensive in horizontal space)
- Auto-poll status checks (manual Ctrl+R only)
- Show indicators for non-running instances
- Show a loading state (`...`) — silence until data arrives
- Require changes to `app.go`, message routing, or key bindings
- Affect any resource type other than EC2
