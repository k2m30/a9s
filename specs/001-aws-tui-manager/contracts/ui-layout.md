# UI Layout Contract: a9s

**Branch**: `001-aws-tui-manager` | **Date**: 2026-03-15

## Screen Layout

The terminal screen is divided into 4 persistent zones:

```
┌──────────────────────────────────────────────────┐
│ HEADER: a9s v0.1.0 | profile: prod | us-east-1  │
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

Fixed at the top. Always visible. Contains:
- Application name and version (left)
- Active AWS profile name (center-left)
- Active AWS region code (center-right)
- Loading indicator (right, when active)

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

**ResourceList view**: Table with column headers, data rows,
cursor highlight. Shows row count in bottom-right of table.

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
- Normal mode: keybinding hints (e.g., `? help  : command  / filter`)
- Command mode: `:` prefix + text input with suggestions
- Filter mode: `/` prefix + filter text + match count
- Error state: error message (auto-clears after 5 seconds)
- Loading: spinner + "Loading <resource type>..."

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
