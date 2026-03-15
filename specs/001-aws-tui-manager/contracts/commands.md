# Command Contracts: a9s

**Branch**: `001-aws-tui-manager` | **Date**: 2026-03-15

## Colon Commands

All colon commands are activated by pressing `:` followed by
the command name. Commands are case-insensitive.

| Command           | Action                            | View Target     |
|-------------------|-----------------------------------|-----------------|
| `:main` / `:root` | Navigate to resource types list   | MainMenu        |
| `:ctx`            | List/switch AWS profiles          | ProfileSelect   |
| `:region`         | List/switch AWS regions           | RegionSelect    |
| `:s3`             | List S3 buckets                   | ResourceList    |
| `:ec2`            | List EC2 instances                | ResourceList    |
| `:rds`            | List RDS instances                | ResourceList    |
| `:redis`          | List ElastiCache Redis clusters   | ResourceList    |
| `:docdb`          | List DocumentDB clusters          | ResourceList    |
| `:eks`            | List EKS clusters                 | ResourceList    |
| `:secrets`        | List Secrets Manager secrets      | ResourceList    |
| `:q` / `:quit`    | Exit the application              | N/A             |

### Command Input Behavior

- `:` activates command mode, showing a text input in the footer.
- Auto-suggestions appear as the user types, matching known commands
  (the first known command starting with the typed prefix is shown
  as a completion hint).
- Enter executes the command.
- Escape cancels command input and returns to normal mode.
- Backspace deletes the last character; if the text becomes empty,
  command mode is automatically exited.
- Unknown commands display "Unknown command: :<input>" in the
  status bar (as an error that auto-clears after 5 seconds).

## Keybinding Contracts

### Global (all views)

| Key       | Action                              |
|-----------|-------------------------------------|
| `:`       | Enter command mode                  |
| `/`       | Enter filter mode (resource list only) |
| `?`       | Show help overlay                   |
| `Escape`  | Back / cancel / clear filter        |
| `q`       | Quit (from main menu) / back (from other views) |
| `[`       | Navigate back in history            |
| `]`       | Navigate forward in history         |
| `Ctrl-R`  | Refresh current view (reload data)  |
| `Ctrl-C`  | Force quit application              |

### Navigation (table/list views)

| Key            | Action                            |
|----------------|-----------------------------------|
| `j` / Down     | Move cursor down                  |
| `k` / Up       | Move cursor up                    |
| `g`            | Jump to top of list               |
| `G`            | Jump to bottom of list            |
| `h` / Left     | Scroll table left (horizontal)    |
| `l` / Right    | Scroll table right (horizontal)   |
| `Enter`        | Select / drill into resource      |
| `N`            | Sort by name column               |
| `S`            | Sort by status column             |
| `A`            | Sort by age/time column           |

### Resource Actions (resource list views)

| Key | Action                                         |
|-----|------------------------------------------------|
| `d` | Describe — show all resource attributes        |
| `y` | JSON view — show raw API response as JSON      |
| `x` | Reveal — fetch and show sensitive values       |
| `c` | Copy — copy resource identifier to clipboard   |

### Scrollable Views (detail, JSON, reveal)

| Key            | Action                              |
|----------------|-------------------------------------|
| `j` / Down     | Scroll down                         |
| `k` / Up       | Scroll up                           |
| `g`            | Scroll to top                       |
| `G`            | Scroll to bottom                    |
| `c`            | Copy content to clipboard (reveal view only) |
| `Escape`       | Return to previous view             |

### Filter Mode

| Key         | Action                                       |
|-------------|----------------------------------------------|
| Any text    | Filter rows matching text (real-time)        |
| `Enter`     | Accept filter and exit filter mode (filter stays active) |
| `Escape`    | Clear filter text and exit filter mode       |
| `Backspace` | Delete last character (exits filter mode if text becomes empty) |

Filter matches across all visible columns. Matching is
case-insensitive substring search.

**Status bar display**: When filter mode is first activated with no
text, shows `/  (type to filter)`. As the user types, shows
`/<text> (<matched>/<total>)` — e.g., `/prod (3/50)`.

## CLI Interface Contract

### Invocation

```
a9s [flags]
```

### Flags

| Flag               | Type   | Default                  | Description                |
|--------------------|--------|--------------------------|----------------------------|
| `--profile`, `-p`  | string | AWS_PROFILE or "default" | AWS profile to use         |
| `--region`, `-r`   | string | Profile's default region | AWS region override        |
| `--version`, `-v`  | bool   | false                    | Print version and exit     |
| `--help`, `-h`     | bool   | false                    | Print usage and exit       |

### Exit Codes

| Code | Meaning                                    |
|------|--------------------------------------------|
| 0    | Normal exit (`:q` or Ctrl-C)               |
| 1    | Fatal error (no AWS config, terminal error) |

### Environment Variables

| Variable             | Description                                        |
|----------------------|----------------------------------------------------|
| `AWS_PROFILE`        | Default AWS profile (overridden by `--profile`)    |
| `AWS_REGION`         | Default AWS region (overridden by `--region`)      |
| `AWS_DEFAULT_REGION` | Fallback region if `AWS_REGION` is not set         |
| `NO_COLOR`           | Disable colors when set (standard convention)      |

**Region resolution order**: `--region` flag > `AWS_REGION` env >
`AWS_DEFAULT_REGION` env > profile's configured region in
`~/.aws/config` > `us-east-1` (final fallback).
