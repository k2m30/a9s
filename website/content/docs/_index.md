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
| `c` | Copy resource ID |
| `/` | Filter |
| `:` | Command mode |
| `?` | Help |
| `Ctrl+R` | Refresh |
| `w` | Toggle wrap |

### Sorting

| Key | Action |
|-----|--------|
| `N` | Sort by name |
| `I` | Sort by ID |
| `A` | Sort by date |

## Configuration

a9s reads your standard AWS configuration from `~/.aws/config` and `~/.aws/credentials`.

Application config is stored in `~/.a9s/config.yaml`.

## AWS Permissions

a9s uses read-only API calls exclusively. The `ReadOnlyAccess` managed policy provides sufficient access. a9s gracefully handles permission errors for services you don't have access to.
