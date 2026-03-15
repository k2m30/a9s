# UI Layout Contract: a9s

**Branch**: `001-aws-tui-manager` | **Date**: 2026-03-15

## Screen Layout

The terminal screen is divided into 4 persistent zones:

```
┌──────────────────────────────────────────────────┐
│ HEADER: a9s v0.3.2 | profile: prod | us-east-1  │
├──────────────────────────────────────────────────┤
│ BREADCRUMBS: main > EC2 > i-abc123               │
├──────────────────────────────────────────────────┤
│                                                  │
│ CONTENT AREA                                     │
│ (table view, detail view, list view, etc.)       │
│                                                  │
│                                                  │
├──────────────────────────────────────────────────┤
│ STATUS BAR: <command input> | <filter> | <error> │
└──────────────────────────────────────────────────┘
```

### Header (1 line)

Fixed at the top. Always visible. Rendered as a single styled line:
`a9s v<version> | profile: <name> | <region>`

When loading is in progress, `[loading...]` is appended:
`a9s v0.3.2 | profile: prod | us-east-1 [loading...]`

### Breadcrumbs (1 line)

Below header. Shows navigation path segments separated
by ` > `. Examples:
- `main`
- `main > EC2`
- `main > S3 > my-bucket > logs/2024/`
- `main > Secrets > my-secret [describe]`

### Content Area (remaining height - 3 lines)

The main content area fills all available space between
breadcrumbs and status bar. Renders the current view:

**MainMenu view**: List of resource type names, cursor
highlight on current selection.

**ResourceList view**: Table with column headers, separator line,
data rows, and cursor highlight (`> ` prefix on selected row).
Shows resource count in the title line as `<Type> (<count>)`.
Supports horizontal scrolling via `h`/`l` keys when the table
is wider than the terminal.

**Detail/Describe view**: Scrollable key-value pairs showing
all resource attributes. Keys left-aligned, values right.

**JSON view**: Scrollable raw JSON with syntax highlighting.

**Reveal view**: Scrollable plain text of secret value.

**ProfileSelect view**: List of AWS profile names, current
profile highlighted differently.

**RegionSelect view**: List of AWS region codes with display
names, current region highlighted.

**Help overlay**: Semi-transparent overlay showing all
keybindings grouped by category.

### Status Bar (1 line)

Fixed at the bottom. Context-dependent content:
- **Normal mode**: displays "Ready" or the most recent status message
- **Command mode**: `:` prefix + text input with auto-completion suggestion
  for known commands (e.g., typing `:re` shows `:region` as suggestion)
- **Filter mode**: When first activated (empty filter), displays
  `/  (type to filter)`. As the user types, displays
  `/<text> (<matched>/<total>)` — e.g., `/prod (3/50)`.
- **Error state**: error message in red (auto-clears after 5 seconds
  via `tea.Tick` + `ClearErrorMsg`)
- **Loading**: header shows `[loading...]` indicator

## Color Scheme

Status-based coloring for resource states:

| State/Status         | Color   |
|----------------------|---------|
| Running / Active     | Green   |
| Stopped / Inactive   | Red     |
| Pending / Creating   | Yellow  |
| Terminated / Deleted | Gray    |
| Error / Failed       | Red     |
| Available            | Green   |
| Modifying            | Yellow  |

UI element colors:
- Header background: dark blue
- Breadcrumbs: dim white
- Table headers: bold white
- Cursor row: reverse video (inverted colors)
- Filter match highlight: yellow
- Error messages: red
- Loading spinner: cyan

Respects `NO_COLOR` environment variable — all colors disabled
when set.
