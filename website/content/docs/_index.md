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
| CFN Stacks | Stack Events | `Enter` | Event timeline showing stack operation progress and status |
| CFN Stacks | Stack Resources | `r` | Logical and physical resources in the stack with status |
| Auto Scaling Groups | Scaling Activities | `Enter` | Scaling activity history with status, description, cause |
| CloudWatch Alarms | Alarm History | `Enter` | State transitions, configuration updates, and action events |
| Load Balancers | Listeners | `Enter` | Port, protocol, action, target, SSL policy, certificate |
| Step Functions | Executions | `Enter` | Execution list with status, duration, start/stop times |
| SFN Executions | Execution History | `Enter` | Step-by-step state machine trace with errors and state names |
| CodeBuild Projects | Builds | `Enter` | Recent builds with status, duration, source version, initiator |
| CodeBuild Builds | Build Logs | `Enter` | Full build output from CloudWatch Logs with phase highlighting |
| CodePipelines | Pipeline Stages | `Enter` | Flattened stage→action view with status coloring and external URLs |
| ECR Repositories | Images | `Enter` | Tags, digest, size, push time, scan findings with severity counts |

## Commands

{{< include "commands.md" >}}

## Configuration

{{< include "config.md" >}}

## Environment Variables

{{< include "env-vars.md" >}}

## AWS Permissions

{{< include "permissions.md" >}}
