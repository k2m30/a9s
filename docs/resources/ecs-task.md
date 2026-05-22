---
shortName: ecs-task
name: ECS Tasks
awsApiRef: https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Task.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# ecs-task — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `ecs-task`
- **Display name**: ECS Tasks
- **AWS API reference**: <https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Task.html>
- **List API**: `ListTasks` + `DescribeTasks` (list-mode enumeration requires both; `ListTasks` returns ARNs only, `DescribeTasks` returns the full `Task` shape. The task list is scoped to a parent cluster — tasks are a child view under `ecs`/`ecs-svc`.)
- **Describe API (if any)**: `DescribeTasks` used for Wave 2 (same shape as list-mode; Wave 2 simply re-reads `StopCode` / container `ExitCode` on the already-fetched Task). `DescribeTaskDefinition` is required for related-panel pivots to `ecr`, `logs`, `role`, `secrets`, `ssm` (see §2).

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `ct-events`, `ec2`, `ecr`, `ecs`, `ecs-svc`, `eni`, `logs`, `role`, `secrets`, `sg`, `ssm`, `subnet`.

### `alarm`

- **Why related**: CloudWatch alarms scoped to `ClusterName` + `ServiceName` dimensions fire on task-level symptoms (CPU/memory saturation, task count deltas). The alarm entry is the fastest route from a broken task row to "what page is ringing right now?" — a9s-devops: an operator responding to a task failure almost always wants to see whether the on-call is already paged.
- **How discovered**: cross-reference the already-loaded `alarm` list by `Dimensions[].Name == "ClusterName"` matching `Task.ClusterArn` suffix and, when the task belongs to a service, `Dimensions[].Name == "ServiceName"` matching `Task.Group` (stripped of the `service:` prefix). No extra API call.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Audit trail for task start/stop events — `RunTask`, `StartTask`, `StopTask`, and the `ecs.amazonaws.com` service principal actions that terminated or rescheduled the task.
- **How discovered**: universal pivot — applies to every registered type; see related-resources.md §Policy.
- **Count shown**: unknown.

### `ec2`

- **Why related**: For EC2-launch-type tasks the container instance is a real EC2 host; terminating or rebooting the instance kills every task on it. The EC2 pivot answers "which box is this task on, and is that box healthy?".
- **How discovered**: call `ecs:DescribeContainerInstances(cluster=Task.ClusterArn, containerInstances=[Task.ContainerInstanceArn])` to resolve `Ec2InstanceId`, then cross-reference the already-loaded `ec2` list. Only applies when `Task.LaunchType==EC2` and `Task.ContainerInstanceArn` is non-nil (Fargate tasks have no container instance) — a9s-devops: Fargate tasks omit this pivot cleanly; no empty row.
- **Count shown**: yes (0 or 1).

### `ecr`

- **Why related**: Containers pull images from ECR; when an image tag is moved or a repository deleted, subsequent task launches fail to pull. The ECR pivot lets the operator jump from a failing task to the repository that owns the image.
- **How discovered**: call `ecs:DescribeTaskDefinition(taskDefinition=Task.TaskDefinitionArn)`, iterate `ContainerDefinitions[].Image`, parse each as `{account}.dkr.ecr.{region}.amazonaws.com/{repo}[:tag|@digest]`, and cross-reference the already-loaded `ecr` list by `repo`. Images not matching the ECR URI pattern (Docker Hub, public ECR, gcr.io) are ignored.
- **Count shown**: yes.

### `ecs`

- **Why related**: Parent cluster. Cluster state (`ACTIVE` vs `INACTIVE`) and capacity providers are the first context for any task issue.
- **How discovered**: read `Task.ClusterArn` on the resource; cross-reference the already-loaded `ecs` list by ARN.
- **Count shown**: yes (always 1).

### `ecs-svc`

- **Why related**: For service-managed tasks, the owning service holds the desired count, deployment state, and circuit breaker — most "why did my task stop?" answers live on the service, not the task.
- **How discovered**: read `Task.Group` on the resource; if it starts with `service:`, strip the prefix to get the service name and cross-reference the already-loaded `ecs-svc` list. Standalone tasks (Group does not start with `service:`) omit this pivot.
- **Count shown**: yes (0 or 1).

### `eni`

- **Why related**: `awsvpc` network-mode tasks (all Fargate, optional EC2) attach their own ENI; the ENI carries the private IP, flow logs, and any attached security groups. When networking is broken, the ENI is where the real evidence lives.
- **How discovered**: read `Task.Attachments[]` where `Type=="ElasticNetworkInterface"`, extract `Details[].Value` for the `Name=="networkInterfaceId"` entry, and cross-reference the already-loaded `eni` list by ID.
- **Count shown**: yes.

### `logs`

- **Why related**: awslogs-driver log groups receive the container's stdout/stderr — when a task died from `EssentialContainerExited` the next step is almost always "tail the log group".
- **How discovered**: call `ecs:DescribeTaskDefinition(taskDefinition=Task.TaskDefinitionArn)`, iterate `ContainerDefinitions[].LogConfiguration` where `LogDriver=="awslogs"`, read `Options["awslogs-group"]`, and cross-reference the already-loaded `logs` list by log-group name.
- **Count shown**: yes.

### `role`

- **Why related**: Task role (application IAM) and execution role (agent IAM — pulls images, writes logs, reads secrets) are the two principals the task acts as. An `AccessDenied` in the task logs usually resolves to one of these two roles.
- **How discovered**: call `ecs:DescribeTaskDefinition(taskDefinition=Task.TaskDefinitionArn)`, read `TaskRoleArn` and `ExecutionRoleArn`, and cross-reference the already-loaded `role` list by ARN.
- **Count shown**: yes (0, 1, or 2).

### `secrets`

- **Why related**: Task definitions inject Secrets Manager secrets as environment variables via `ContainerDefinitions[].Secrets[].ValueFrom`. When a task fails to start with "unable to pull secret", the secret itself is the next click — a9s-devops: secret-injection failures are a common Fargate startup failure mode, and the link is deterministic from the task definition.
- **How discovered**: call `ecs:DescribeTaskDefinition(taskDefinition=Task.TaskDefinitionArn)`, iterate `ContainerDefinitions[].Secrets[]` and `ContainerDefinitions[].RepositoryCredentials.CredentialsParameter`, filter `ValueFrom` values whose ARN service prefix is `secretsmanager`, and cross-reference the already-loaded `secrets` list. Also covered by the reverse-scan documented under `secrets` in related-resources.md (`TaskDefinition.ContainerDefinitions[].Secrets[].ValueFrom==ARN`).
- **Count shown**: yes.

### `sg`

- **Why related**: `awsvpc` task ENIs carry security groups that gate all ingress/egress for the task's containers — "connection refused" and "timeout" symptoms trace here first.
- **How discovered**: derive the task's ENI via `Task.Attachments[]` (see `eni` above), then cross-reference the already-loaded `eni` list to read `Groups[].GroupId` on that ENI, and cross-reference the already-loaded `sg` list by ID. No extra API call beyond what `eni` already consumes — a9s-devops: the SG membership is carried on the ENI, not the Task, so the chain is Task → ENI → SG.
- **Count shown**: yes.

### `ssm`

- **Why related**: Parallel to `secrets`, task definitions inject SSM Parameter Store values (including SecureString parameters) via `ContainerDefinitions[].Secrets[].ValueFrom`. Config drift from a parameter rotation is a frequent "worked yesterday, broken today" cause — a9s-devops: SSM-mediated config is cheaper than Secrets Manager and shows up especially in non-prod environments.
- **How discovered**: same call as `secrets` — `ecs:DescribeTaskDefinition`, iterate `ContainerDefinitions[].Secrets[]`, filter `ValueFrom` values whose ARN service prefix is `ssm` (or a bare parameter name, resolved to the account/region SSM namespace), and cross-reference the already-loaded `ssm` list.
- **Count shown**: yes.

### `subnet`

- **Why related**: `awsvpc` task ENIs are placed in a specific subnet; the subnet's free-IP count, route table, and NAT attachment determine whether the task can reach the internet or a VPC endpoint.
- **How discovered**: read `Task.Attachments[]` where `Type=="ElasticNetworkInterface"`, extract `Details[].Value` for the `Name=="subnetId"` entry, and cross-reference the already-loaded `subnet` list by ID.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `lastStatus==RUNNING`.
  - **State bucket**: Healthy.
  - **How obtained**: `Task.LastStatus` on the `DescribeTasks` response (list-mode uses `DescribeTasks` because `ListTasks` returns ARNs only).

- **Signal**: `lastStatus` in transitional states (`PROVISIONING`, `PENDING`, `ACTIVATING`, `DEACTIVATING`, `STOPPING`, `DEPROVISIONING`).
  - **State bucket**: Warning.
  - **How obtained**: `Task.LastStatus`.

- **Signal**: `lastStatus==STOPPED` with `StopCode != UserInitiated`.
  - **State bucket**: Broken.
  - **How obtained**: `Task.LastStatus` and `Task.StopCode` (both on the `DescribeTasks` response; `StopCode` is one of `TaskFailedToStart`, `EssentialContainerExited`, `UserInitiated`, `ServiceSchedulerInitiated`, `SpotInterruption`, `TerminationNotice`).

- **Signal**: `healthStatus==UNHEALTHY`.
  - **State bucket**: Broken.
  - **How obtained**: `Task.HealthStatus` (enum `HEALTHY` / `UNHEALTHY` / `UNKNOWN`).

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: `DescribeTasks`: `StopCode` in `TaskFailedToStart`/`EssentialContainerExited` → Broken.
  - **State bucket**: Broken.
  - **API call**: `DescribeTasks` (same call as list-mode; no additional network call — this is the same data surfaced again with a stricter rule on the StopCode value).
  - **Cost shape**: per-resource (already paid in list-mode).

- **Signal**: container `exitCode` non-zero with `essential=true` → Broken.
  - **State bucket**: Broken.
  - **API call**: `DescribeTasks` gives `Task.Containers[].ExitCode`; the `essential` flag lives on the task definition, so a paired `DescribeTaskDefinition(taskDefinition=Task.TaskDefinitionArn)` is needed to confirm essentiality.
  - **Cost shape**: per-resource (one `DescribeTaskDefinition` per distinct `TaskDefinitionArn` in the list — task definitions are immutable per revision, so a per-session cache collapses this to roughly one call per unique revision).

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: Cross-cluster outlier detection.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing" — e.g. maintenance scheduled, certificate expiring soon. `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `stopping: Server.SpotInstanceShutdown`, `expires in 7d`). **Healthy rows render blank** — no `OK` / `available` / `ACTIVE` / `running`. Empty means "nothing to see." |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank. Silence is the UX.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4 (short cause), S5 (full sentence).
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph on green row. S3, S4 (short cause), S5 (full sentence). No S1.
- **Wave 2 finding on an already yellow/red/dim row** → redundant with color; S3 suppressed, S4 deduplicates with existing cause, S5 still carries the full sentence, S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `lastStatus` transitional (`PROVISIONING`/`PENDING`/`ACTIVATING`/`STOPPING`/`DEPROVISIONING`) | 1 | Warning | n/a | S2, S4 | `starting` or `stopping` (match phase) | `Task is transitioning — last status PENDING, started 00:00:14 ago.` |
| `lastStatus==STOPPED`, `StopCode==EssentialContainerExited` | 1 | Broken | n/a | S2, S4 | `stopped: essential container exited` | `Essential container exited with code N — see logs for the last lines before exit.` |
| `lastStatus==STOPPED`, `StopCode==TaskFailedToStart` | 1 | Broken | n/a | S2, S4 | `stopped: failed to start` | `Task never reached RUNNING — check image pull, secrets, and IAM permissions.` |
| `lastStatus==STOPPED`, `StopCode==SpotInterruption` | 1 | Broken | n/a | S2, S4 | `stopped: spot reclaimed` | `Spot capacity was reclaimed by AWS — task will be rescheduled if managed by a service.` |
| `lastStatus==STOPPED`, `StopCode==ServiceSchedulerInitiated` | 1 | Broken | n/a | S2, S4 | `stopped: scheduler replaced` | `Service scheduler stopped this task as part of a deployment or scaling action.` |
| `lastStatus==STOPPED`, `StopCode==TerminationNotice` | 1 | Broken | n/a | S2, S4 | `stopped: termination notice` | `Host received a termination notice — task was drained before shutdown.` |
| `lastStatus==STOPPED`, `StopCode==UserInitiated` | 1 | Dim | n/a | S2, S4 | `stopped: user initiated` | `Task was stopped intentionally — no action needed.` |
| `healthStatus==UNHEALTHY` | 1 | Broken | n/a | S2, S4 | `running but unhealthy` | `Task is RUNNING but an essential container failed its health check.` |
| Wave 2: `ExitCode!=0` on essential container (on an already-Broken row) | 2 | Broken | `!` (counted) | S1, S4 (dedup), S5 | `stopped: exit N` (merged with existing cause) | `Essential container <name> exited with code <N> — reason: "<Container.Reason or StoppedReason>".` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`DORMANT`, `stopped`, `available`, `failed`) in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column. Tests will assert the cause is present.
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), you may omit the row from this table entirely; §3 still describes it.
- Keep both columns short enough to fit: List text ≤ 40 chars, Detail text ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for every non-healthy state: every red / yellow row in §4 carries the StopCode translated into plain words in S4 (`stopped: essential container exited`, `stopped: failed to start`, `stopped: spot reclaimed`), so the operator can triage from the list alone; detail is only needed when they want the specific container name and `Reason` text. The one UX gap to enforce in the implementation: never show a bare `STOPPED` / `PENDING` word without its cause — `list-attention-coverage.md` grades this resource "A-" and calls for adding a `Stop Code` column, which S4 delivers by concatenating the translated cause into the existing Status column rather than adding a second column.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above) — cross-cluster outlier detection.
- Container-level sub-views (per-container exit codes, network bindings) beyond the single enrichment line — the full `Task.Containers[]` list is already visible in the detail view via existing field rendering; no new panel.
- Execution-level CloudWatch metrics (CPU/memory per task) — Wave 3.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

One bullet per claim in §§2–4.1.

- AWS Go SDK v2 — `Task.LastStatus`, `Task.StopCode`, `Task.HealthStatus`, `Task.Containers[].ExitCode`, `Task.Attachments[]`, `Task.ClusterArn`, `Task.ContainerInstanceArn`, `Task.Group`, `Task.TaskDefinitionArn` — `AWS SDK Go v2 — ecs/types.Task § LastStatus, StopCode, HealthStatus, Containers, Attachments, ClusterArn, ContainerInstanceArn, Group, TaskDefinitionArn`.
- AWS Go SDK v2 — `StopCode` enum values — `AWS SDK Go v2 — ecs/types.TaskStopCode` (`TaskFailedToStart`, `EssentialContainerExited`, `UserInitiated`, `ServiceSchedulerInitiated`, `SpotInterruption`, `TerminationNotice`).
- AWS Go SDK v2 — `HealthStatus` enum values — `AWS SDK Go v2 — ecs/types.HealthStatus` (`HEALTHY`, `UNHEALTHY`, `UNKNOWN`).
- AWS Go SDK v2 — `Container.ExitCode`, `Container.Reason`, `Container.HealthStatus` — `AWS SDK Go v2 — ecs/types.Container § ExitCode, Reason, HealthStatus`.
- AWS Go SDK v2 — Attachment details carry `networkInterfaceId` and `subnetId` for ElasticNetworkInterface attachments — `AWS SDK Go v2 — ecs/types.Attachment § Details, Type`.
- AWS Go SDK v2 — TaskDefinition holds `ExecutionRoleArn`, `TaskRoleArn`, `ContainerDefinitions[].Image`, `ContainerDefinitions[].Secrets[].ValueFrom`, `ContainerDefinitions[].LogConfiguration.Options["awslogs-group"]` — `AWS SDK Go v2 — ecs/types.TaskDefinition § ExecutionRoleArn, TaskRoleArn, ContainerDefinitions`.
- a9s golden doc — Wave 1 / Wave 2 / Wave 3 signals for `ecs-task` — `docs/attention-signals.md` § Compute → `ecs-task` row.
- a9s golden doc — related-panel contract for `ecs-task` (`alarm`, `ct-events`, `ec2`, `ecr`, `ecs`, `ecs-svc`, `eni`, `logs`, `role`, `secrets`, `sg`, `ssm`, `subnet`) — `docs/related-resources.md` § Per-type contract and § `ecs-task`.
- a9s golden doc — `ct-events` is a universal pivot — `docs/related-resources.md` § Policy item 4.
- a9s golden doc — `ecs-task → kms` is explicitly out of the contract (no direct KMS reference on a task) — `docs/related-resources.md` § Unanimous `sometimes` entries.
- a9s golden doc — read-only invariant cited in §5 — `docs/architecture.md` § "What is a9s?".
- a9s golden doc — `list-attention-coverage.md` grades `ecs-task` "A-" and recommends adding `Stop Code` / `Health Status` information to the list, satisfied here via S4 — `docs/list-attention-coverage.md` § `ecs-task` row.
- a9s-devops persona (2026-04-20) — `alarm` pivot discovery via `ClusterName`/`ServiceName` alarm dimensions: possible=yes, worth=yes. Rationale: on-call uses CloudWatch alarm dimensions to scope cluster/service-level saturation alerts; the reverse join is zero-cost against the already-loaded alarm list. (Falling back to persona: agent dispatch unavailable in this session.)
- a9s-devops persona (2026-04-20) — `ec2` pivot requires `DescribeContainerInstances` and is skipped for Fargate: possible=yes, worth=yes. Rationale: EC2-launch-type tasks frequently die because the host went unhealthy; the pivot avoids a mode-switch into the EC2 list to find the host by ARN.
- a9s-devops persona (2026-04-20) — `ecr`, `logs`, `role`, `secrets`, `ssm` pivots require a `DescribeTaskDefinition` call: possible=yes, worth=yes. Rationale: these five pivots are the common "why did the task fail to start?" answers (image pull, log-group permission, IAM, secret rotation, parameter drift); one `DescribeTaskDefinition` per unique revision is a bounded cost and matches the Wave 2 budget already authorized for the `essential=true + ExitCode!=0` signal.
- a9s-devops persona (2026-04-20) — `sg` pivot is indirect via the ENI: possible=yes, worth=yes. Rationale: awsvpc tasks carry security groups on the ENI, not on the Task; once the ENI is loaded for the `eni` pivot, `sg` is a pure cross-reference with no additional call.
- a9s-devops persona (2026-04-20) — severity on Wave 2 `essential+ExitCode!=0` is `!` (counted) because the finding identifies a concrete container-level failure an on-call must address. Rationale: matches the existing `enrichment-visibility.md` example for ECS Tasks (`"task stopped: EssentialContainerExited"` with `!`).

<!-- BEGIN GENERATED: header -->
ecs-task — COMPUTE. Lifecycle key: `status`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Approximate? |
| --- | --- | --- |
| ecs-svc | ECS Services | no |
| ecs | ECS Clusters | no |
| logs | Log Groups | yes |
| role | IAM Role | no |
| alarm | CloudWatch Alarms | yes |
| ct-events | CloudTrail Events | yes |
| ec2 | EC2 Instances | no |
| ecr | ECR Repositories | no |
| eni | Network Interfaces | no |
| secrets | Secrets | no |
| sg | Security Groups | no |
| ssm | SSM Parameters | no |
| subnet | Subnets | no |
<!-- END GENERATED: related -->
