# Roadmap

This document outlines the planned direction for a9s. Priorities may shift based on community feedback.

## Already Implemented

- **66 AWS resource types** across 12 categories
- **Search and filter** -- `/` to filter resource lists; `/` to search within YAML, detail, and JSON views with `n`/`N` for next/prev match
- **Column sorting** -- `N`/`I`/`A` to sort by name, ID, or date; `1`-`0` keys to sort by any column position
- **Customizable columns** -- `~/.a9s/views/` overrides which fields are displayed per resource type
- **Multi-account** -- works out of the box via assume-role in `~/.aws/config`
- **Demo mode** -- `--demo` flag runs the full UI with synthetic data, no AWS needed
- **Child views** -- drill-down screens for resources that contain sub-entities (e.g., Lambda → Invocations, CloudWatch Log Groups → Log Streams, ECS Clusters → Services/Tasks, IAM Groups → Users)
- **Resource relationships** -- navigate from EC2 to its VPC, Security Groups, EBS Volumes; related panel via `r` key in detail views
- **11 color themes** -- Tokyo Night Dark (default), Dracula, Nord, Catppuccin, and more; custom theme support via `~/.a9s/themes/`
- **Command mode** -- `:` opens a command prompt with tab completion for profile/region switching, navigation (`:root`, `:help`), and theme changes
- **CloudTrail integration** -- press `t` from any resource detail to view recent API activity for that resource
- **YAML view** -- full AWS API response with syntax coloring, search, word wrap toggle
- **JSON view** -- raw JSON export with auto-detect and pretty-print in detail and reveal views
- **Tag flattening** -- AWS tags rendered as `Key: Value` pairs in detail views for easy reading
- **Clipboard support** -- `c` to copy resource IDs, YAML, JSON, ARNs, and secret values
- **Error log** -- `!` to view all session errors with timestamps; scrollable, searchable
- **Identity view** -- `i` to inspect current IAM caller identity, account, and role
- **Help view** -- `?` for context-sensitive keybinding reference
- **Horizontal scrolling** -- `h`/`l` to scroll wide tables
- **Pagination** -- `M` to load more for large result sets (demo mode showcases this)
- **8,700+ unit tests**

## Short-Term

- **Issues shown in UI** -- highlight potential issues to narrow down incidents

## Medium-Term

- **Cost overlay** -- show estimated monthly cost per resource (via Cost Explorer API)
- **Live tail** -- stream CloudWatch Logs in a split pane

## Long-Term

- **More AWS resource types** -- Cognito, AppSync, Config Rules, GuardDuty, Neptune, SageMaker, and more
- **Tag editor** -- view and edit resource tags inline
- **Bookmarks** -- save frequently accessed resources for quick access
- **Resource actions** -- start/stop/reboot instances, invoke lambdas (opt-in, off by default). Gated on project maturity (10k+ stars). a9s must prove itself as a safe, trusted read-only tool before introducing write operations.

## Non-Goals

- **Plugin system** -- adds complexity without clear value; new resource types are easy to add via PR
- **Terraform/IaC integration** -- a9s is a viewer, not a provisioning tool
- **Telemetry or analytics** -- a9s will never phone home
- **Web UI** -- terminal-first, always
