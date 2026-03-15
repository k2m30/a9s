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
- Auto-suggestions appear as the user types, matching known commands.
- Enter executes the command.
- Escape cancels command input and returns to normal mode.
- Unknown commands display "Unknown command: :<input>" in the
  status bar.

## Keybinding Contracts

### Global (all views)

| Key       | Action                              |
|-----------|-------------------------------------|
| `:`       | Enter command mode                  |
| `/`       | Enter filter mode                   |
| `?`       | Show help overlay                   |
| `Escape`  | Back / cancel / clear filter        |
| `[`       | Navigate back in history            |
| `]`       | Navigate forward in history         |
| `Ctrl-R`  | Refresh current view (reload data)  |
| `Ctrl-C`  | Exit application                    |

### Navigation (table/list views)

| Key            | Action                            |
|----------------|-----------------------------------|
| `j` / Down     | Move cursor down                  |
| `k` / Up       | Move cursor up                    |
| `g`            | Jump to top of list               |
| `G`            | Jump to bottom of list            |
| `Enter`        | Select / drill into resource      |
| `Shift-N`      | Sort by name column               |
| `Shift-S`      | Sort by status column             |
| `Shift-A`      | Sort by age/time column           |

### Resource Actions (resource list views)

| Key | Action                                         |
|-----|------------------------------------------------|
| `d` | Describe — show all resource attributes        |
| `y` | JSON view — show raw API response as JSON      |
| `x` | Reveal — fetch and show sensitive values       |
| `c` | Copy — copy resource identifier to clipboard   |

### Scrollable Views (detail, JSON, reveal)

| Key            | Action                   |
|----------------|--------------------------|
| `j` / Down     | Scroll down              |
| `k` / Up       | Scroll up                |
| `g`            | Scroll to top            |
| `G`            | Scroll to bottom         |
| `Escape`       | Return to previous view  |

### Filter Mode

| Key       | Action                               |
|-----------|--------------------------------------|
| Any text  | Filter rows matching text            |
| `Escape`  | Clear filter and exit filter mode    |

Filter matches across all visible columns. Matching is
case-insensitive substring search.

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

| Variable      | Description                                 |
|---------------|---------------------------------------------|
| `AWS_PROFILE` | Default AWS profile (overridden by `--profile`) |
| `AWS_REGION`  | Default AWS region (overridden by `--region`) |
| `NO_COLOR`    | Disable colors when set (standard convention) |
