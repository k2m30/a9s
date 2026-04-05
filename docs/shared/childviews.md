Press `Enter` on a resource to explore its nested children. Press `Esc` to go back.

| Parent | Child View | Key | Description |
|--------|-----------|-----|-------------|
| Lambda Functions | Invocations | `Enter` | Recent invocations with status, duration, memory, cold start |
| Lambda Invocations | Log Lines | `Enter` | Full log output for a specific invocation |
| S3 Buckets | Objects | `Enter` | Browse bucket contents, drill into folders |
| Route 53 Zones | DNS Records | `Enter` | View A, CNAME, MX, and other record types |
| Log Groups | Log Streams | `Enter` | Streams sorted by most recent event |
| Log Streams | Log Events | `Enter` | Color-coded log lines (ERROR=red, WARN=yellow) |
| Target Groups | Target Health | `Enter` | Health status per target (healthy/unhealthy/draining) |
| ECS Services | Service Events | `e` | Event timeline (steady state, placement failures, deployments) |
| ECS Services | Tasks | `Enter` | Running and stopped tasks with status, health, stopped reason |
| ECS Services | Container Logs | `L` | Application logs from CloudWatch |
| CFN Stacks | Stack Events | `Enter` | Event timeline showing stack operation progress and status |
| CFN Stacks | Stack Resources | `R` | Logical and physical resources in the stack with status |
| Auto Scaling Groups | Scaling Activities | `Enter` | Scaling activity history with status, description, cause |
| CloudWatch Alarms | Alarm History | `Enter` | State transitions, configuration updates, and action events |
| Load Balancers | Listeners | `Enter` | Port, protocol, action, target, SSL policy, certificate |
| Step Functions | Executions | `Enter` | Execution list with status, duration, start/stop times |
| SFN Executions | Execution History | `Enter` | Step-by-step state machine trace with errors and state names |
| CodeBuild Projects | Builds | `Enter` | Recent builds with status, duration, source version, initiator |
| CodeBuild Builds | Build Logs | `Enter` | Full build output from CloudWatch Logs with phase highlighting |
| CodePipelines | Pipeline Stages | `Enter` | Flattened stage→action view with status coloring and external URLs |
| ECR Repositories | Images | `Enter` | Tags, digest, size, push time, scan findings with severity counts |
| IAM Roles | Attached Policies | `Enter` | Managed and inline policies with type, ARN, admin highlight |
| IAM Groups | Group Members | `Enter` | Member users with ID, creation date, password last used |
| ELB Listeners | Listener Rules | `Enter` | Priority, conditions, action type, target (nested level 2) |
| RDS Instances | RDS Events | `Enter` | Failovers, maintenance, reboots, configuration changes |
| SNS Topics | Subscriptions | `Enter` | Subscribers with protocol, endpoint, confirmation status |
| EventBridge Rules | Targets | `Enter` | Target ARN, input configuration, IAM role |
| Glue Jobs | Job Runs | `Enter` | Execution history with status, duration, DPU usage, errors |
