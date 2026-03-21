---
title: "Documentation"
---

## Getting Started

1. [Install a9s](/a9s/install/)
2. Ensure you have [AWS credentials configured](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html)
3. Run `a9s` (or `a9s -p myprofile`)

## Key Bindings

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
| `x` | Reveal (expand) |
| `c` | Copy resource ID to clipboard |
| `/` | Filter |
| `:` | Command mode |
| `?` | Help |
| `Ctrl+R` | Refresh |
| `w` | Toggle line wrap (in YAML view) |
| `Tab` | Autocomplete (in command mode) |

### Sorting

| Key | Action |
|-----|--------|
| `N` | Sort by name |
| `I` | Sort by ID |
| `A` | Sort by date |

### General

| Key | Action |
|-----|--------|
| `q` | Quit |
| `Ctrl+C` | Force quit |

## Commands

Press `:` to enter command mode, then type a command:

| Command | Action |
|---------|--------|
| `:q` / `:quit` | Exit a9s |
| `:ctx` / `:profile` | Switch AWS profile |
| `:region` | Switch AWS region |
| `:help` | Show help |
| `:<resource>` | Jump to resource type (e.g., `:ec2`, `:s3`, `:lambda`) |

All resource shortnames from the [Resources](/a9s/resources/) page work as commands.

## Configuration

a9s stores configuration in `~/.a9s/config.yaml`. AWS profiles and regions are read from your standard AWS configuration (`~/.aws/config` and `~/.aws/credentials`).

## AWS Permissions

a9s uses **read-only** AWS API calls exclusively. The following managed policies provide sufficient access:

- `ReadOnlyAccess` (broad read-only access to all services)
- Or individual service policies like `AmazonEC2ReadOnlyAccess`, `AmazonS3ReadOnlyAccess`, etc.

a9s will gracefully handle permission errors -- resources you don't have access to will show an error message instead of crashing.
