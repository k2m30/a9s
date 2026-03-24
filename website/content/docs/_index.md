---
title: "Documentation"
---

## Getting Started

1. [Install a9s](/a9s/install/)
2. Ensure you have [AWS credentials configured](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html)
3. Run `a9s` (or `a9s -p myprofile`)

## Key Bindings

{{< include "keybindings.md" >}}

## Child Views (Drill-Downs)

Press `Enter` on a resource to explore its nested children. Press `Esc` to go back.

| Parent | Child View | Key | Description |
|--------|-----------|-----|-------------|
| Lambda Functions | Invocations | `Enter` | Recent invocations with status, duration, memory, cold start |
| Lambda Invocations | Log Lines | `Enter` | Full log output for a specific invocation (START → app logs → END) |
| S3 Buckets | Objects | `Enter` | Browse bucket contents, drill into folders |
| Route 53 Zones | DNS Records | `Enter` | View A, CNAME, MX, and other record types |
| Log Groups | Log Streams | `Enter` | Streams sorted by most recent event |
| Log Streams | Log Events | `Enter` | Color-coded log lines (ERROR=red, WARN=yellow) |
| Target Groups | Target Health | `Enter` | Health status per target (healthy/unhealthy/draining) |
| ECS Services | Service Events | `e` | Event timeline (steady state, placement failures, deployments) |
| ECS Services | Tasks | `Enter` | Running and stopped tasks with status, health, stopped reason |
| ECS Services | Container Logs | `L` | Application logs from CloudWatch (resolved from task definition) |

## Commands

{{< include "commands.md" >}}

## Configuration

{{< include "config.md" >}}

## AWS Permissions

{{< include "permissions.md" >}}
