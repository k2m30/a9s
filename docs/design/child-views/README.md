# Child View Design Specifications

## Overview

All MUST-HAVE, SHOULD-HAVE, and Structural child views from the [devops research](devops-research.md).
Two child views already implemented: S3 Objects and R53 Records.

### Status Key

- **Implemented** -- shipped and tested
- **Planned** -- design spec complete, ready for implementation

---

## Child View Inventory

| # | Parent | Child View | Tier | Status | Design File | Nesting |
|---|--------|-----------|------|--------|-------------|---------|
| — | S3 Buckets (s3) | Objects | MUST-HAVE | Implemented | [s3-objects.md](s3-objects.md) | No |
| — | Route 53 Zones (r53) | Records | MUST-HAVE | Implemented | [r53-records.md](r53-records.md) | No |
| 1 | Log Groups (logs) | Log Streams | MUST-HAVE | Planned | [logs-streams.md](logs-streams.md) | Yes: Log Events (level 2) |
| 2 | Lambda (lambda) | Invocations | MUST-HAVE | Planned | [lambda-invocations.md](lambda-invocations.md) | Yes: Log Lines (level 2) |
| 3 | ECS Services (ecs-svc) | Service Events | MUST-HAVE | Planned | [ecs-svc-events.md](ecs-svc-events.md) | No |
| 4 | ECS Services (ecs-svc) | Tasks | MUST-HAVE | Planned | [ecs-svc-tasks.md](ecs-svc-tasks.md) | No |
| 5 | ECS Services (ecs-svc) | Container Logs | MUST-HAVE | Planned | [ecs-svc-logs.md](ecs-svc-logs.md) | No |
| 6 | CFN Stacks (cfn) | Stack Events | MUST-HAVE | Planned | [cfn-events.md](cfn-events.md) | No |
| 7 | CFN Stacks (cfn) | Stack Resources | MUST-HAVE | Planned | [cfn-resources.md](cfn-resources.md) | No |
| 8 | Target Groups (tg) | Target Health | MUST-HAVE | Planned | [tg-health.md](tg-health.md) | No |
| 9 | ASG (asg) | Scaling Activities | MUST-HAVE | Planned | [asg-activities.md](asg-activities.md) | No |
| 10 | Load Balancers (elb) | Listeners | MUST-HAVE | Planned | [elb-listeners.md](elb-listeners.md) | Yes: Rules (level 2) |
| 11 | CW Alarms (alarm) | Alarm History | SHOULD-HAVE | Planned | [alarm-history.md](alarm-history.md) | No |
| 12 | Step Functions (sfn) | Executions | SHOULD-HAVE | Planned | [sfn-executions.md](sfn-executions.md) | Yes: History (level 2) |
| 13 | CodeBuild (cb) | Builds | SHOULD-HAVE | Planned | [cb-builds.md](cb-builds.md) | Yes: Build Logs (level 2) |
| 14 | CodePipeline (pipeline) | Pipeline Stages | SHOULD-HAVE | Planned | [pipeline-stages.md](pipeline-stages.md) | No |
| 15 | ECR (ecr) | Images | SHOULD-HAVE | Planned | [ecr-images.md](ecr-images.md) | No |
| 16 | RDS Instances (dbi) | RDS Events | SHOULD-HAVE | Planned | [dbi-events.md](dbi-events.md) | No |
| 17 | IAM Roles (role) | Attached Policies | SHOULD-HAVE | Planned | [role-policies.md](role-policies.md) | No |
| 18 | SNS Topics (sns) | Subscriptions | SHOULD-HAVE | Planned | [sns-subscriptions.md](sns-subscriptions.md) | No |
| 19 | EB Rules (eb-rule) | Targets | SHOULD-HAVE | Planned | [eb-rule-targets.md](eb-rule-targets.md) | No |
| 20 | Glue Jobs (glue) | Job Runs | SHOULD-HAVE | Planned | [glue-runs.md](glue-runs.md) | No |
| 21 | IAM Groups (iam-group) | Group Members | SHOULD-HAVE | Planned | [iam-group-members.md](iam-group-members.md) | No |

**Total: 21 parent-child relationships, 25 view levels (4 have nested level 2)**

---

## Nested View Chains (multi-level drill-downs)

These child views themselves have children, forming 3-level navigation chains:

| Chain | Level 1 | Level 2 | Design File |
|-------|---------|---------|-------------|
| Log viewing | Log Streams | Log Events | [logs-streams.md](logs-streams.md) |
| Lambda debugging | Invocations | Log Lines | [lambda-invocations.md](lambda-invocations.md) |
| ALB routing | Listeners | Listener Rules | [elb-listeners.md](elb-listeners.md) |
| Workflow debugging | Executions | Execution History | [sfn-executions.md](sfn-executions.md) |
| CI debugging | Builds | Build Logs | [cb-builds.md](cb-builds.md) |

---

## Dependency Order

Some child views depend on other views or infrastructure being implemented first.

### Hard Dependencies

| Child View | Depends On | Reason |
|-----------|------------|--------|
| Lambda Invocations | Log viewer infrastructure | Cross-service: parses CloudWatch Logs REPORT lines |
| Lambda Log Lines | Log viewer infrastructure | Reuses log event display component |
| ECS Container Logs | Log viewer infrastructure | Cross-service: fetches from CloudWatch Logs |
| CodeBuild Build Logs | Log viewer infrastructure | Cross-service: fetches from CloudWatch Logs |

### Soft Dependencies (share patterns but not code)

| Child View | Benefits From | Reason |
|-----------|---------------|--------|
| ECS Tasks | ECS Events | Same parent (ecs-svc), shared key binding docs |
| CFN Resources | CFN Events | Same parent (cfn), shared key binding docs |
| SFN Execution History | SFN Executions | Nested child, must exist first |
| Listener Rules | ELB Listeners | Nested child, must exist first |
| Build Logs | CodeBuild Builds | Nested child, must exist first |

### No Dependencies (independent)

These can be implemented in any order:
- Target Group Health
- ASG Scaling Activities
- Alarm History
- Pipeline Stages
- ECR Images
- RDS Events
- IAM Role Policies
- SNS Subscriptions
- EventBridge Rule Targets
- Glue Job Runs
- IAM Group Members

---

## Recommended Implementation Order

Ordered by incident value (what helps most at 2 AM), respecting hard dependencies.

### Phase 1: Foundation + "Show me what happened" (highest incident value)

| Order | View | Key Insight | API Call(s) |
|-------|------|-------------|-------------|
| 1 | **logs --> Log Streams --> Log Events** | Foundation. All cross-service log views depend on this. | `DescribeLogStreams`, `GetLogEvents` |
| 2 | **lambda --> Invocations --> Log Lines** | Highest single-resource value. Depends on log viewer. | `FilterLogEvents` (CW Logs) |
| 3 | **ecs-svc --> Service Events** | First check when ECS is unhealthy. Data already in DescribeServices. | `DescribeServices` (re-fetch) |
| 4 | **ecs-svc --> Tasks** | Second check. Shows stopped reasons for crash-loop debugging. | `ListTasks`, `DescribeTasks` |
| 5 | **tg --> Target Health** | "Why 502s?" Answers ALB routing failures instantly. | `DescribeTargetHealth` |
| 6 | **cfn --> Stack Events** | "What failed in this deployment?" Real-time deployment timeline. | `DescribeStackEvents` |

### Phase 2: "Show me the timeline" (deployment and debugging workflows)

| Order | View | Key Insight | API Call(s) |
|-------|------|-------------|-------------|
| 7 | **cfn --> Stack Resources** | "What is stuck?" Shares parent key bindings with cfn-events. | `ListStackResources` |
| 8 | **asg --> Scaling Activities** | "Why did we scale?" Independent, no dependencies. | `DescribeScalingActivities` |
| 9 | **alarm --> Alarm History** | "When did it fire? Is it flapping?" | `DescribeAlarmHistory` |
| 10 | **elb --> Listeners** | "How is traffic routed?" Prerequisite for listener rules. | `DescribeListeners` |
| 11 | **ecs-svc --> Container Logs** | "Show me application output." Depends on log viewer. | `DescribeTaskDefinition`, `FilterLogEvents` |

### Phase 3: "Show me the context" (CI/CD and investigation)

| Order | View | Key Insight | API Call(s) |
|-------|------|-------------|-------------|
| 12 | **sfn --> Executions** | "Did the workflow run?" Prerequisite for execution history. | `ListExecutions` |
| 13 | **cb --> Builds** | "Is CI passing?" Prerequisite for build logs. | `ListBuildsForProject`, `BatchGetBuilds` |
| 14 | **pipeline --> Pipeline Stages** | "Where is my deploy?" Single API call. | `GetPipelineState` |
| 15 | **ecr --> Images** | "What tags exist? Is the right version deployed?" | `DescribeImages` |
| 16 | **dbi --> RDS Events** | "Was there a failover? Maintenance?" | `DescribeEvents` |

### Phase 4: "Show me the structure" (security, audit, nested views)

| Order | View | Key Insight | API Call(s) |
|-------|------|-------------|-------------|
| 17 | **role --> Attached Policies** | "What permissions does this role have?" | `ListAttachedRolePolicies`, `ListRolePolicies` |
| 18 | **sns --> Subscriptions** | "Who gets notified?" | `ListSubscriptionsByTopic` |
| 19 | **eb-rule --> Targets** | "What does this rule trigger?" | `ListTargetsByRule` |
| 20 | **glue --> Job Runs** | "Did the ETL run?" | `GetJobRuns` |
| 21 | **iam-group --> Members** | "Who is in this group?" | `GetGroup` |
| 22 | **elb --> Listeners --> Rules** | Nested child of listeners. | `DescribeRules` |
| 23 | **sfn --> Executions --> History** | Nested child of executions. | `GetExecutionHistory` |
| 24 | **cb --> Builds --> Build Logs** | Nested child of builds. Cross-service (CW Logs). | `GetLogEvents` |

---

## New Key Bindings Introduced

Most child views use only the standard key binding set (j/k/g/G/Enter/d/y/c/Esc/filter/sort). The following are the only NEW bindings required:

| Key | Context | Action | Design File |
|-----|---------|--------|-------------|
| `e` | ECS Services list | Open Service Events | [ecs-svc-events.md](ecs-svc-events.md) |
| `L` | ECS Services list | Open Container Logs | [ecs-svc-logs.md](ecs-svc-logs.md) |
| `r` | CFN Stacks list | Open Stack Resources | [cfn-resources.md](cfn-resources.md) |
| `t` | Log Events / Invocation Logs | Toggle timestamp display | [logs-streams.md](logs-streams.md) |
| `w` | Log Events / Build Logs / Container Logs | Toggle word wrap | Various |

Notes:
- `Enter` on ECS Services opens Tasks (the primary child). `e` opens Events (the secondary child).
- `Enter` on CFN Stacks opens Stack Events (the primary child). `r` opens Stack Resources (the secondary child).
- `w` for word wrap is reused from the existing Detail View (see design.md section 5, Detail View key bindings).
- `t` for timestamp toggle is new but intuitive for log views.

---

## Cross-Service Views

These child views fetch data from a different AWS service than the parent:

| Parent | Child | Source Service | Pattern |
|--------|-------|----------------|---------|
| Lambda | Invocations | CloudWatch Logs | Parse REPORT lines from log group |
| Lambda | Log Lines | CloudWatch Logs | Filter by RequestId |
| ECS Services | Container Logs | CloudWatch Logs (via task def logConfiguration) | FilterLogEvents on awslogs group |
| CodeBuild | Build Logs | CloudWatch Logs (via build Logs.groupName) | GetLogEvents on build log stream |
| RDS Instances | RDS Events | RDS Events API (separate from DescribeDBInstances) | DescribeEvents with source filter |
| CW Alarms | Alarm History | CW Alarm History API (separate from DescribeAlarms) | DescribeAlarmHistory |
| ASG | Scaling Activities | Auto Scaling Activities API (separate from DescribeASGs) | DescribeScalingActivities |

The cross-service log views all follow the same pattern:
1. Extract log group name/stream from parent resource metadata
2. Call CloudWatch Logs API with that group/stream
3. Display results as a log event list

This argues for a **shared log viewer component** that all cross-service log views can reuse. Implement it once with the Log Groups --> Streams --> Events chain, then reuse it for Lambda, ECS, and CodeBuild.
