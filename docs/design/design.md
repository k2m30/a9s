# a9s TUI Design Specification

Version: 2.1 — header-only layout (no status bar)
Target: Lipgloss v2 + Bubble Tea v2 + Bubbles v2
Minimum width: 80 columns
Comfortable width: 120 columns

---

## 1. Color Palette (Tokyo Night Dark)

All colors are specified as `lipgloss.Color("#hex")`. The palette is designed for
dark terminals (the dominant case for AWS tooling).

| Element                | Foreground  | Background | Style        | Notes                        |
|------------------------|-------------|------------|--------------|------------------------------|
| **Header bar**         | `#c0caf5`   | —          | Bold         | App name + profile + region  |
| **Header accent**      | `#7aa2f7`   | —          | Bold         | "a9s" text                   |
| **Header dim**         | `#565f89`   | —          | —            | " v0.x.x" version text       |
| **Header context**     | `#c0caf5`   | —          | Bold         | "  profile:region"           |
| **Header hint**        | `#565f89`   | —          | —            | "? for help" on far right    |
| **Header filter**      | `#e0af68`   | —          | Bold         | "/search-text█" on right     |
| **Header command**     | `#e0af68`   | —          | Bold         | ":ec2█" on right             |
| **Header flash ok**    | `#9ece6a`   | —          | Bold         | "Copied!" transient message  |
| **Header flash err**   | `#f7768e`   | —          | Bold         | Error transient message      |
| **Key hint key**       | `#7aa2f7`   | `#24283b`  | Bold         | `<d>` part of hint           |
| **Key hint desc**      | `#565f89`   | —          | —            | "describe" part of hint      |
| **Table header**       | `#7aa2f7`   | —          | Bold         | Column titles (no sep below) |
| **Table row normal**   | `#c0caf5`   | —          | —            | Unselected rows              |
| **Table row selected** | `#1a1b26`   | `#7aa2f7`  | Bold         | Full-width cursor row        |
| **Table row alt**      | `#c0caf5`   | `#1e2030`  | —            | Alternating row bg (subtle)  |
| **Table row error**    | `#f7768e`   | —          | —            | Entire row in red (failed)   |
| **Table row dim**      | `#565f89`   | —          | Dim          | Entire row dim (terminated)  |
| **Status running**     | `#9ece6a`   | —          | Bold         | running, available, active   |
| **Status stopped**     | `#f7768e`   | —          | —            | stopped, terminated, failed  |
| **Status pending**     | `#e0af68`   | —          | —            | pending, starting, creating  |
| **Status unknown**     | `#565f89`   | —          | Dim          | unknown, n/a, —              |
| **Detail key**         | `#7aa2f7`   | —          | —            | Left side of "key: value"    |
| **Detail value**       | `#c0caf5`   | —          | —            | Right side                   |
| **Detail section**     | `#e0af68`   | —          | Bold         | Section headings (YELLOW)    |
| **YAML key**           | `#7aa2f7`   | —          | —            | key:                         |
| **YAML value str**     | `#9ece6a`   | —          | —            | "string value"               |
| **YAML value num**     | `#ff9e64`   | —          | —            | 42, 3.14                     |
| **YAML value bool**    | `#bb9af7`   | —          | —            | true, false                  |
| **YAML value null**    | `#565f89`   | —          | Dim          | null, ~                      |
| **YAML indent line**   | `#414868`   | —          | Dim          | │ tree connector             |
| **Table border**       | `#414868`   | —          | —            | Thin NormalBorder()          |
| **Border focused**     | `#7aa2f7`   | —          | —            | Active panel border          |
| **Border unfocused**   | `#414868`   | —          | —            | Inactive panel border        |
| **Error text**         | `#f7768e`   | —          | Bold         | Error messages               |
| **Warning text**       | `#e0af68`   | —          | —            | Warning messages             |
| **Success text**       | `#9ece6a`   | —          | —            | Success, copied messages     |
| **Overlay bg**         | `#c0caf5`   | `#1a1b26`  | —            | Help overlay box             |
| **Overlay border**     | `#7aa2f7`   | —          | —            | Help overlay border          |
| **Help key**           | `#9ece6a`   | —          | Bold         | Key in help screen (GREEN)   |
| **Help category**      | `#e0af68`   | —          | Bold         | Category header (ORANGE)     |
| **Spinner**            | `#7aa2f7`   | —          | —            | Loading spinner dots         |
| **Scroll indicator**   | `#414868`   | —          | Dim          | "↑ 12 lines above"           |

---

## 2. Layout Structure

Two elements, top to bottom. No status bar.

```
 a9s v0.5.0  prod:us-east-1                                         ? for help
┌──────────────────── ec2-instances(42) ────────────────────────────────────────┐
│ NAME↑                STATUS      TYPE       AZ           LAUNCH TIME          │
│ api-prod-01          running     t3.medium  us-east-1a   2024-01-15 09:22     │
│ ...                                                                           │
└───────────────────────────────────────────────────────────────────────────────┘
```

Key structural rules:
1. **Header** — 1 unframed line. Left side: `a9s v0.x.x  profile:region`. Right
   side: `? for help` (dim) in the normal state. The right side changes based on
   mode — see section 3.1 for all variants. No separator below it.
2. **Frame** — fills all remaining vertical space. The resource title is
   centered in the top border: `┌──────── ec2-instances(42) ────────┐` with
   equal dashes on both sides of the title. There is NO separate context line,
   NO command/filter line, and NO status bar below the frame.

Line budget:
- Header: 1 line
- Frame top border (with embedded centered title): 1 line
- Frame content rows: `termHeight - 3` lines
- Frame bottom border: 1 line

Lipgloss composition:

```go
lipgloss.JoinVertical(lipgloss.Left,
    header,    // full width, accent+dim text, right: hint/input/flash, Padding(0,1)
    frameBox,  // manually built: ┌──── title ────┐ + │rows│ + └───┘
)
```

Frame construction (manual — lipgloss.Border() cannot embed a title):

```go
// Top border with centered title: ┌─────── title ───────┐
totalDashes := w - 2 - titleVis - 2  // minus corners, spaces around title
leftDashes  := totalDashes / 2
rightDashes := totalDashes - leftDashes
prefix    := "┌" + strings.Repeat("─", leftDashes) + " "
suffix    := " " + strings.Repeat("─", rightDashes) + "┐"
topBorder := borderStyle.Render(prefix) + titleStyle.Render(title) +
             borderStyle.Render(suffix)

// Each content row (content padded to innerW = w-2)
row := borderStyle.Render("│") + paddedContent + borderStyle.Render("│")

// Bottom border
bottom := borderStyle.Render("└" + strings.Repeat("─", w-2) + "┘")
```

---

## 3. Component Specifications

### 3.1 Header Bar

Border: none. Placed directly above the frame — no separator line between them.
Style: `lipgloss.NewStyle().Padding(0, 1).Width(termWidth)`

Left side: `a9s` (accent bold) + `v0.x.x` (dim) + `profile:region` (bold).
Right side: context-sensitive, right-aligned. Variants:

| Mode           | Right side content              | Color           |
|----------------|---------------------------------|-----------------|
| Normal         | `? for help`                    | `#565f89` (dim) |
| Filter active  | `/search-text█`                 | `#e0af68` bold  |
| Command active | `:ec2█`                         | `#e0af68` bold  |
| Flash success  | `Copied!` (auto-clears 2s)      | `#9ece6a` bold  |
| Flash error    | `Error: msg` (auto-clears 2s)   | `#f7768e` bold  |

```
Normal:   a9s v0.5.0  prod:us-east-1                                   ? for help
Filter:   a9s v0.5.0  prod:us-east-1                                   /prod█
Command:  a9s v0.5.0  prod:us-east-1                                   :ec2█
Flash ok: a9s v0.5.0  prod:us-east-1                                   Copied!
Flash err:a9s v0.5.0  prod:us-east-1                          Error: no credentials
```

Composition:

```go
left  := accentStyle.Render("a9s") + dimStyle.Render(" v"+version) +
         boldStyle.Render("  "+profile+":"+region)
right := dimStyle.Render("? for help")  // or filter/cmd/flash variant
gap   := (w - 2) - lipgloss.Width(left) - lipgloss.Width(right)
header := left + strings.Repeat(" ", gap) + right
```

### 3.2 Table Component

Frame: manually constructed (not `lipgloss.NormalBorder()`), border color
`#414868` (thin, dim). The resource title is **centered** in the top border
line. The frame fills the remaining vertical space after the header.

Column headers:
- NOT separated by pipes — space-aligned only
- Sort indicator: `↑` (asc) or `↓` (desc) appended to sorted column title
- Bold, color `#7aa2f7`
- NO underline/separator row below the column headers

```
┌──────────────── ec2-instances(42) ──────────────────────────────────────────┐
│ NAME↑                STATUS      TYPE       AZ           LAUNCH TIME        │
│ api-prod-01          running     t3.medium  us-east-1a   2024-01-15 09:22   │
└─────────────────────────────────────────────────────────────────────────────┘
```

Row coloring strategy (entire row, not just status cell):
- `running` / `available` / `active`: entire row in green `#9ece6a`
- `failed`, and `stopped` where AWS stopped the instance itself: entire row in red `#f7768e`
- `stopped` by request: entire row in yellow `#e0af68`
- `terminated`: entire row dimmed `#565f89`
- `pending` / `creating`: entire row in yellow `#e0af68`
- Selected row: full-width `#7aa2f7` background, `#1a1b26` foreground, bold

This matches k9s where the row color follows the resource state.

Sort indicator: `↑` or `↓` appended directly to the column header, no space.

```go
func colHeader(title string, sortedAsc, sortedDesc bool) string {
    if sortedAsc  { return title + "↑" }
    if sortedDesc { return title + "↓" }
    return title
}
```

### 3.5 Detail / Describe View

Border: `lipgloss.NormalBorder()`, color `#414868`, same thin box as table.
The box replaces the table box — same position in the layout.

Key-value layout:
- Keys: color `#7aa2f7` (cyan-blue), left-aligned, fixed width (~22 chars)
- Values: color `#c0caf5` (plain white)
- Section headers: color `#e0af68` (YELLOW/ORANGE), bold, 1-space indent with trailing colon
- Sub-fields: 5-space indent

Top-level keys use 1-space indent, colon immediately after key name (`Key:`),
value padded to 22-char key column. Section headers (multi-line subtrees) are
rendered at 1-space indent; their child lines are indented 5 spaces.

```
│ Name:                  datalayer-service-prod-on-demand                    │
│ Namespace:             backend                                              │
│ Priority:              0                                                    │
│                                                                             │
│ Containers:                                                                 │
│      app:                                                                   │
│      Image: python:3.11-slim                                                │
│      CPU/Memory: 100m / 256Mi                                               │
```

### 3.6 Help Screen (k9s multi-column style)

Not an overlay — replaces the table box content.
Title rendered as: `── Help ──` centered with dim line decorators.
Four-column layout matching k9s exactly.

```
                              ── Help ──
│ RESOURCE              GENERAL              NAVIGATION           HOTKEYS     │
│ <esc>  Back           <ctrl-a> Aliases     <j>      Down        <?>  Help   │
│ <q>    Quit           <q>      Quit        <k>      Up          <:>  Cmd    │
│                       <ctrl-r> Refresh     <g>      Top                     │
│                                            <G>      Bottom                  │
│                                            <h/l>    Cols                    │
│                                            <Enter>  Open                    │
```

Key color: `#9ece6a` (GREEN) — matches k9s style
Description: `#c0caf5` (plain white)
Category headers: `#e0af68` (ORANGE/YELLOW), uppercase, bold

---

## 4. ASCII Wireframes

### 4.1 View 1 — Main Menu

```
 [ACCENT]a9s[/] [DIM]v0.5.0[/]  [BOLD]default:us-east-1[/]                                    [DIM]? for help[/]
┌───────────────────── resource-types(7) ──────────────────────────────────────┐
│  [SELECTED]  EC2 Instances                                                    :ec2    [/]│
│    S3 Buckets                                                       :s3       │
│    RDS Instances                                                    :rds      │
│    ElastiCache Redis                                                :redis    │
│    DocumentDB Clusters                                              :docdb    │
│    EKS Clusters                                                     :eks      │
│    Secrets Manager                                                  :secrets  │
│                                                                              │
│  [DIM]7 resource types[/]                                                            │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 4.2 View 2 — Resource List (EC2 Instances, 120 columns)

Normal state (no filter, no command):

```
 [ACCENT]a9s[/] [DIM]v0.5.0[/]  [BOLD]prod:us-east-1[/]                                                                                                [DIM]? for help[/]
┌──────────────────────────────────── ec2-instances(42) ────────────────────────────────────────────────────────────────────┐
│ [BOLD]NAME↑                   STATUS      TYPE           AZ              AMI                    LAUNCH TIME      [/]        │
│ [SELECTED]  api-prod-01           running     t3.medium      us-east-1a      ami-0abcdef012345   2024-01-15 09:22          [/]│
│ [GREEN]  api-prod-02           running     t3.medium      us-east-1b      ami-0abcdef012345   2024-01-15 09:25[/]           │
│ [GREEN]  worker-01             running     t3.large       us-east-1a      ami-0abcdef012345   2024-01-10 14:30[/]           │
│ [YELLOW]  worker-02             pending     t3.large       us-east-1b      ami-0abcdef012345   2024-03-17 08:00[/]          │
│ [GREEN]  bastion               running     t2.micro       us-east-1a      ami-0zzz111222333   2023-11-01 10:00[/]          │
│ [YELLOW]  old-worker            stopped     t3.medium      us-east-1c      ami-0abcdef012345   2023-06-20 16:45[/]        │
│ [DIM]  legacy-app            terminated  t2.small       us-east-1a      ami-0000111222333   2022-12-01 12:00[/]           │
│   · · · (35 more)                                                                                                         │
└───────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

Notes on row coloring:
- Selected row: full-width blue background regardless of status
- `running` rows: entire row text in GREEN `#9ece6a`
- `pending` rows: entire row text in YELLOW `#e0af68`
- `stopped by AWS` rows: entire row text in RED `#f7768e`
- `stopped` rows (stopped by request): entire row text in YELLOW `#e0af68`
- `terminated` rows: entire row DIM `#565f89`
- Column headers have NO underline/separator row below them

When filter is active: header right shows `/text█` (amber bold), frame title
shows `(matched/total)` e.g. `ec2-instances(3/42)`, only matching rows are
displayed. Everything else — colors, layout, columns, selection — is unchanged.
No matched-text highlighting inside row cells.

Command mode active:

```
 [ACCENT]a9s[/] [DIM]v0.5.0[/]  [BOLD]prod:us-east-1[/]                                       [YELLOW]:ec2█[/]
┌──────────────────── ec2-instances(42) ────────────────────────────────────┐
│ ...content unchanged...                                                   │
└───────────────────────────────────────────────────────────────────────────┘
```

Flash message (transient, auto-clears after ~2s):

```
 [ACCENT]a9s[/] [DIM]v0.5.0[/]  [BOLD]prod:us-east-1[/]                                       [GREEN]Copied![/]
┌──────────────────── ec2-instances(42) ────────────────────────────────────┐
│ ...content unchanged...                                                   │
└───────────────────────────────────────────────────────────────────────────┘
```

Loading state:

```
 [ACCENT]a9s[/] [DIM]v0.5.0[/]  [BOLD]prod:us-east-1[/]                                    [DIM]? for help[/]
┌─────────────────── ec2-instances ─────────────────────────────────────────┐
│                                                                           │
│        [SPINNER]⠿[/] Fetching EC2 instances...                                     │
│                                                                           │
└───────────────────────────────────────────────────────────────────────────┘
```

### 4.3 View 3 — Detail View – extracted to a separate document

### 4.4 View 4 — YAML View

```
 [ACCENT]a9s[/] [DIM]v0.5.0[/]  [BOLD]prod:us-east-1[/]                                    [DIM]? for help[/]
┌──────────────── i-0abc123def456789a yaml ───────────────────────────────────┐
│ [YAMLKEY]AmiLaunchIndex[/]: [YAMLNUM]0[/]                                                   │
│ [YAMLKEY]Architecture[/]: [YAMLSTR]x86_64[/]                                               │
│ [YAMLKEY]BlockDeviceMappings[/]:                                                     │
│ [DIM]│[/]   - [YAMLKEY]DeviceName[/]: [YAMLSTR]/dev/xvda[/]                                    │
│ [DIM]│[/]     [YAMLKEY]Ebs[/]:                                                           │
│ [DIM]│[/]       [YAMLKEY]AttachTime[/]: [YAMLSTR]2024-01-15T09:22:45Z[/]                       │
│ [DIM]│[/]       [YAMLKEY]DeleteOnTermination[/]: [YAMLBOOL]true[/]                             │
│ [DIM]│[/]       [YAMLKEY]Status[/]: [YAMLSTR]attached[/]                                       │
│ [YAMLKEY]ImageId[/]: [YAMLSTR]ami-0abcdef01234567[/]                                       │
│ [YAMLKEY]InstanceId[/]: [YAMLSTR]i-0abc123def456789a[/]                                   │
│ [YAMLKEY]InstanceType[/]: [YAMLSTR]t3.medium[/]                                           │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.5 View 5 — Help Screen (k9s multi-column)

```
 [ACCENT]a9s[/] [DIM]v0.5.0[/]  [BOLD]prod:us-east-1[/]                                    [DIM]? for help[/]
┌───────────────────────────── Help ──────────────────────────────────────────┐
│ [CATEGORY]RESOURCE             [/] [CATEGORY]GENERAL              [/] [CATEGORY]NAVIGATION          [/] [CATEGORY]HOTKEYS[/]          │
│                                                                             │
│ [HELPKEY]<esc>[/]  Back           [HELPKEY]<ctrl-r>[/] Refresh      [HELPKEY]<j>[/]       Down        [HELPKEY]<?>[/]   Help       │
│ [HELPKEY]<q>[/]    Quit           [HELPKEY]<q>[/]      Quit         [HELPKEY]<k>[/]       Up          [HELPKEY]<:>[/]   Command    │
│                            [HELPKEY]<:[/]       Command      [HELPKEY]<g>[/]       Top                          │
│                            [HELPKEY]</>[/]       Filter       [HELPKEY]<G>[/]       Bottom                       │
│                                                 [HELPKEY]<h/l>[/]     Cols                          │
│                                                 [HELPKEY]<enter>[/]   Open                          │
│                                                 [HELPKEY]<d>[/]       Detail                        │
│                                                 [HELPKEY]<y>[/]       YAML                          │
│                                                 [HELPKEY]<c>[/]       Copy ID                       │
│                                                 [HELPKEY]<N/S/A>[/]   Sort                          │
│                                                                             │
│                      [DIM]Press any key to close[/]                               │
└─────────────────────────────────────────────────────────────────────────────┘
```

Column width: each of the 4 columns is `(tableWidth - 4) / 4` characters wide.
Category headers: `#e0af68` bold, uppercase.
Keys: `#9ece6a` bold (GREEN — matches k9s help colors).
Descriptions: `#c0caf5` plain.

### 4.6 View 6 — Profile Selector

```
 [ACCENT]a9s[/] [DIM]v0.5.0[/]  [BOLD]prod:us-east-1[/]                                    [DIM]? for help[/]
┌──────────────────── aws-profiles(6) ────────────────────────────────────────┐
│                                                                             │
│  [SELECTED]  default                 (current)                                        [/]│
│    prod                                                                     │
│    staging                                                                  │
│    dev                                                                      │
│    personal                                                                 │
│  [DIM]  legacy-account         (no credentials)[/]                                │
│                                                                             │
│  [DIM]6 profiles[/]                                                                │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.7 View 7 — Region Selector

Same structure as Profile Selector with region names and descriptions.

### 4.8 View 8 — Reveal View (Secrets Manager)

```
 [ACCENT]a9s[/] [DIM]v0.5.0[/]  [BOLD]prod:us-east-1[/]                         [RED]Secret visible — press esc to close[/]
┌──────────────── prod/api/database-password ─────────────────────────────────┐
│                                                                             │
│  [BOLD]prod/api/database-password[/]                                              │
│  [DIM]──────────────────────────────────[/]                                      │
│                                                                             │
│  [GREEN]s3cr3t-p@ssw0rd-here![/]                                                 │
│                                                                             │
│  [DIM]Type: SecureString[/]                                                      │
│  [DIM]Last rotated: 2024-01-10T14:23:00Z[/]                                      │
│  [DIM]Rotation enabled: true[/]                                                   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

Note: when a secret is revealed, the "? for help" hint is replaced by the
red warning in the header. This is a persistent warning (not auto-clearing)
that stays until the user presses `esc` to close the reveal view.

---

## 5. Key Binding Reference

### Global (all views)

| Key       | Action                    | Notes                        |
|-----------|---------------------------|------------------------------|
| `q`       | Quit application          | Only on main menu            |
| `ctrl+c`  | Force quit                | Any view                     |
| `?`       | Toggle help screen        | Replaces table box content   |
| `esc`     | Go back / cancel          | Pop view stack               |
| `ctrl+r`  | Refresh current view      | Re-fetches from AWS          |
| `:`       | Enter command mode        | Header right becomes input   |
| `/`       | Enter filter mode         | Header right becomes input   |

### Main Menu

| Key     | Action               | Notes                    |
|---------|----------------------|--------------------------|
| `j`/`↓` | Move cursor down     | Wraps at bottom          |
| `k`/`↑` | Move cursor up       | Wraps at top             |
| `g`     | Jump to top          |                          |
| `G`     | Jump to bottom       |                          |
| `enter` | Open resource list   | For selected type        |

### Resource List

| Key      | Action               | Notes                        |
|----------|----------------------|------------------------------|
| `j`/`↓`  | Move cursor down     |                              |
| `k`/`↑`  | Move cursor up       |                              |
| `g`      | Jump to top          |                              |
| `G`      | Jump to bottom       |                              |
| `h`/`←`  | Scroll columns left  |                              |
| `l`/`→`  | Scroll columns right |                              |
| `enter`  | Open detail view     |                              |
| `d`      | Open detail view     | Config-driven field list     |
| `y`      | Open YAML view       | Full resource YAML           |
| `x`      | Reveal secret value  | Secrets Manager only         |
| `c`      | Copy resource ID     | Copies Name or ARN           |
| `N`      | Sort by name         | Toggles asc/desc; ↑↓ header  |
| `S`      | Sort by status       | Toggles asc/desc             |
| `A`      | Sort by age          | Toggles asc/desc             |
| `pgup`/`ctrl+u` | Page up       |                              |
| `pgdn`/`ctrl+d` | Page down     |                              |

### Detail View

| Key      | Action               | Notes                        |
|----------|----------------------|------------------------------|
| `j`/`↓`  | Scroll down one line |                              |
| `k`/`↑`  | Scroll up one line   |                              |
| `g`      | Jump to top          |                              |
| `G`      | Jump to bottom       |                              |
| `w`      | Toggle word wrap     |                              |
| `c`      | Copy detail content  | Full detail to clipboard     |
| `y`      | Switch to YAML view  |                              |
| `pgup`/`ctrl+u` | Page up       |                              |
| `pgdn`/`ctrl+d` | Page down     |                              |

### Help Screen

| Key      | Action               |
|----------|----------------------|
| any key  | Close help           |
| `esc`    | Close help           |

### Filter Mode (`/`)

`/` activates filter input in the header right side. Typing filters rows by
case-insensitive substring match — only matching rows are shown and the frame
title updates to `(matched/total)`. `esc` clears the filter and restores all
rows. `backspace` removes the last character. No other special keys.

### Command Mode (`:`)

| Key     | Action               |
|---------|----------------------|
| `enter` | Execute command      |
| `esc`   | Cancel command       |
| `tab`   | Accept autocomplete  |

Known commands: `main`, `root`, `ctx`, `region`, `s3`, `ec2`, `rds`, `redis`,
`docdb`, `eks`, `secrets`, `q`, `quit`.

---

## 6. State Transitions

### Msg Types → State Changes

| Msg                    | From State          | To State              | Notes                         |
|------------------------|---------------------|-----------------------|-------------------------------|
| `ResourcesLoadedMsg`   | Loading:true        | Loading:false         | Populates Resources slice     |
| `APIErrorMsg`          | Loading:true        | Error shown in header | HeaderFlash error, persistent |
| `StatusMsg`            | Any                 | Header right updated  | Auto-clears after 2s          |
| `tea.WindowSizeMsg`    | Any                 | Width/Height updated  | Reflow all views              |
| `KeyMsg(enter)`        | MainMenuView        | ResourceListView      | Pushes view stack             |
| `KeyMsg(enter)`        | ResourceListView    | DetailView            | Pushes view stack             |
| `KeyMsg(y)`            | ResourceListView    | YAMLView              | Pushes view stack             |
| `KeyMsg(x)`            | ResourceListView    | RevealView            | Secrets only                  |
| `KeyMsg(esc)`          | Any non-main        | Previous view         | Pops view stack               |
| `KeyMsg(?)`            | Any                 | HelpView              | Replaces table box            |
| `KeyMsg(:)`            | Any                 | CommandMode=true      | Header right becomes input    |
| `KeyMsg(/)`            | List views          | FilterMode=true       | Header right becomes input    |
| `ProfileSelectedMsg`   | ProfileSelectView   | MainMenuView          | Reloads AWS clients           |
| `RegionSelectedMsg`    | RegionSelectView    | Previous view         | Reloads AWS clients           |
| `CopiedMsg`            | Any                 | Flash success in hdr  | Auto-clears after 2s          |

### View Stack

Navigation uses a stack (`[]ViewState`). `esc` pops the stack.

```
push(MainMenu) → push(ResourceList:ec2) → push(Detail:i-abc) → push(YAML)
                                                             ← esc (YAML → Detail)
                                        ← esc (Detail → ResourceList)
← esc (ResourceList → MainMenu)
```

---

## 7. Component States

### Table Row States

| State      | Visual                                                       |
|------------|--------------------------------------------------------------|
| Normal     | Row text colored by status value                             |
| Selected   | Full-row `#7aa2f7` background, `#1a1b26` foreground, bold    |
| Loading    | Spinner centered in box content area                         |
| Empty      | Centered message with hint to refresh or change region       |
| Error      | Red error text in header right (flash, persistent until nav) |

### Row Color by Status (entire row)

<!-- BEGIN GENERATED: colors -->

A row takes the colour of the finding it carries, and the catalog
decides that. An instance an operator stopped and one AWS stopped
itself are two findings at two severities, so they are two colours.

| Finding | Phrase on the row | Severity | Row colour | Hex |
| --- | --- | --- | --- | --- |
| `ec2.state.pending` | pending | warn | YELLOW | `#e0af68` |
| `ec2.state.stopping` | stopping | warn | YELLOW | `#e0af68` |
| `ec2.state.stopped` | stopped | warn | YELLOW | `#e0af68` |
| `ec2.state.stopped.server` | stopped by AWS | broken | RED | `#f7768e` |
| `ec2.state.terminated` | terminated | dim | DIM | `#565f89` |

Every other row is GREEN `#9ece6a` when its type reports it healthy and
PLAIN `#c0caf5` when the type has nothing to say about it.

<!-- END GENERATED: colors -->

Selected row always overrides row color with full blue background.

### Panel Focus States

| State     | Border color | Border style         |
|-----------|--------------|----------------------|
| Focused   | `#7aa2f7`    | `NormalBorder()`     |
| Unfocused | `#414868`    | `NormalBorder()`     |
| Error     | `#f7768e`    | `NormalBorder()`     |

---

## 8. Responsive Behavior

### Width Breakpoints

| Terminal width | Behavior                                                   |
|----------------|------------------------------------------------------------|
| < 60 cols      | Show error: "Terminal too narrow. Please resize."          |
| 60-79 cols     | 2 columns only (NAME, STATUS). No horizontal scroll hint.  |
| 80-119 cols    | Standard layout: NAME, STATUS, TYPE, AZ columns.           |
| 120+ cols      | Full layout: all configured columns visible.               |

### Height Breakpoints

| Terminal height | Behavior                                                  |
|-----------------|-----------------------------------------------------------|
| < 7 lines       | Show error: "Terminal too short. Please resize."          |
| 7-20 lines      | Full layout, only 3 structural lines overhead.            |
| 20+ lines       | Full layout.                                              |

Note: overhead is now 3 lines (header + frame top border + frame bottom border),
one fewer than before since the status bar was removed.

### Narrow Header (< 80 cols)

```
 [ACCENT]a9s[/] [DIM]v0.6.0[/]  prod:us-east-1
```

(The `? for help` hint may be omitted if there is not enough horizontal space.)

### Column Overflow Strategy

When content width exceeds terminal width:
1. Rightmost columns are hidden (not truncated mid-value)
2. Users can scroll horizontally with `h`/`l` keys (discoverable via `?` help)
3. Horizontal scroll offset (h/l keys) shifts the visible column window
4. Column header scrolls in sync with data

---

## 9. Borders and Spacing Summary

| Component              | Border style                                        | Padding    |
|------------------------|-----------------------------------------------------|------------|
| Header bar             | None (directly above frame, no separator)           | `(0, 1)`   |
| Frame top border       | Manual: `┌──── title ────┐`, centered, `#414868`    | none       |
| Frame content rows     | Manual: `│ content │`, `#414868` borders            | none       |
| Frame bottom border    | Manual: `└──────────────┘`, `#414868`               | none       |
| Table header row       | None (inside frame, first content row)              | `(0, 1)`   |
| Table header separator | NONE — column names only, no separator line         | —          |
| Table data row         | None (inside frame)                                 | `(0, 1)`   |
| Detail view            | Inside frame                                        | `(0, 1)`   |
| YAML view              | Inside frame                                        | `(0, 1)`   |
| Help screen            | Inside frame                                        | `(0, 1)`   |
| Profile / Region list  | Inside frame                                        | `(0, 2)`   |

---

## 10. Bubbles Components to Use

| View / Component   | Bubbles Component      | Notes                                           |
|--------------------|------------------------|-------------------------------------------------|
| Loading spinner    | `bubbles/spinner`      | Dot spinner, `#7aa2f7` color                    |
| Detail scroll      | `bubbles/viewport`     | Renders inside the table box                    |
| YAML scroll        | `bubbles/viewport`     | Same viewport component                         |
| Filter input       | `bubbles/textinput`    | In header right side, no border                 |
| Command input      | `bubbles/textinput`    | In header right side, no border                 |
| Key hints (full)   | custom multi-column    | Rendered inside table box, 4-column layout      |
| Profile list       | `bubbles/list`         | With custom delegate for current indicator      |
| Region list        | `bubbles/list`         | With custom delegate for current indicator      |
| Resource list      | Custom renderer        | Needs per-row status coloring, h-scroll         |
| Progress bars      | `bubbles/progress`     | Optional: loading progress if API supports it   |
