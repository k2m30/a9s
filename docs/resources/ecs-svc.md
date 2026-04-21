---
shortName: ecs-svc
name: ECS Services
awsApiRef: https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Service.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# ecs-svc — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `ecs-svc`
- **Display name**: ECS Services
- **AWS API reference**: https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Service.html
- **List API**: `DescribeServices` (invoked per cluster after `ListServices` returns ARNs; `ListServices` itself returns ARNs only — every field a9s renders on the list row comes from the `DescribeServices` response).
- **Describe API (if any)**: Same `DescribeServices` output also carries Wave 2 fields (`deployments[].rolloutState`, `events[]`) — no additional per-service API call. a9s-devops-persona: ECS exposes service state only through `DescribeServices`, batched up to 10 service ARNs per call, so "list" and "describe" are the same wire call in practice.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `cfn`, `eb-rule`, `ecr`, `ecs`, `ecs-task`, `elb`, `logs`, `role`, `secrets`, `sfn`, `sg`, `subnet`, `tg`, `vpc`, `ct-events`.

### `alarm`

- **Why related**: CloudWatch alarms watching the service's CPU, memory, or PendingTaskCount — first signal of capacity/throttling impact.
- **How discovered**: cross-reference the already-loaded `alarm` list by `MetricAlarm.Dimensions[]` containing `{Name: "ClusterName", Value: <cluster>}` AND `{Name: "ServiceName", Value: <service>}` — a9s-devops-persona: ECS service alarms always carry this dimension pair in the `AWS/ECS` namespace, so a cache-scan is sufficient; no extra API call.
- **Count shown**: yes.

### `cfn`

- **Why related**: CloudFormation stack that created the service — infra-as-code provenance.
- **How discovered**: read field `Service.Tags[]` for key `aws:cloudformation:stack-name` or `aws:cloudformation:stack-id`; cross-reference the already-loaded `cfn` list by `StackName` — a9s-devops-persona: CloudFormation stamps these reserved tags on every stack-managed resource including ECS services, so tag lookup is the correct pivot; no API call when `cfn` is already loaded.
- **Count shown**: yes.

### `eb-rule`

- **Why related**: EventBridge scheduled-task rules that target this ECS service — most "cron on ECS" workflows are EB rules with an ECS target.
- **How discovered**: cross-reference the already-loaded `eb-rule` list where `ListTargetsByRule` response contains a target with `EcsParameters.TaskDefinitionArn` or `Arn` pointing at this service's cluster, and (for service-linked scheduled tasks) `Service.ServiceName` matching — a9s-devops-persona: EventBridge is the canonical way to schedule ECS tasks; the target carries the cluster ARN and (for service-launched runs) the service reference. When `eb-rule` is loaded with targets, this is a pure cache scan.
- **Count shown**: yes.

### `ecr`

- **Why related**: Container images the service's tasks pull — upstream supply chain for every task launched by this service.
- **How discovered**: resolve `Service.TaskDefinition` → `DescribeTaskDefinition` → `ContainerDefinitions[].Image`; parse ECR URIs of shape `<acct>.dkr.ecr.<region>.amazonaws.com/<repo>[:tag|@digest]`; cross-reference the already-loaded `ecr` list by `Repository.repositoryName` — a9s-devops-persona: the task definition is the only place that names the images, and `DescribeTaskDefinition` is one call per distinct `TaskDefinition` ARN (often one per service). Worth it because operators routinely pivot from a failing service to "is the image still there? when was it pushed?".
- **Count shown**: yes.

### `ecs`

- **Why related**: Parent cluster — context for capacity, container-instance pool, cluster-wide settings.
- **How discovered**: read field `Service.ClusterArn`; cross-reference the already-loaded `ecs` list by `Cluster.clusterArn` or `Cluster.clusterName`.
- **Count shown**: yes.

### `ecs-task`

- **Why related**: Running tasks for this service — drill-down to actual container health and exit codes.
- **How discovered**: `ListTasks(cluster=<clusterArn>, serviceName=<serviceName>)` returns task ARNs scoped to the service; cross-reference the already-loaded `ecs-task` list by `Task.Group == "service:<serviceName>"` (Tasks launched by a service carry that group marker) — a9s-devops-persona: `Task.Group` is the reliable link and is returned by `DescribeTasks`; when tasks are preloaded, this is a cache scan with zero extra calls.
- **Count shown**: yes.

### `elb`

- **Why related**: Load balancer fronting the service — user-facing traffic path.
- **How discovered**: resolve via `Service.LoadBalancers[].TargetGroupArn` → the already-loaded `tg` list → `TargetGroup.LoadBalancerArns[]` → cross-reference the already-loaded `elb` list by `LoadBalancer.LoadBalancerArn` — a9s-devops-persona: ECS does not record the LB ARN on the service object; only the target group is attached, so the LB is one hop away through `tg`. No extra API call when `tg` and `elb` are already loaded.
- **Count shown**: yes.

### `logs`

- **Why related**: CloudWatch Log Groups receiving container stdout/stderr — primary runtime diagnostic for task failures.
- **How discovered**: resolve `Service.TaskDefinition` → `DescribeTaskDefinition` → `ContainerDefinitions[].LogConfiguration` where `LogDriver == "awslogs"`; extract `Options["awslogs-group"]`; cross-reference the already-loaded `logs` list by `logGroupName` — a9s-devops-persona: awslogs is the overwhelmingly common driver for ECS; `Options["awslogs-group"]` names the log group directly. Re-uses the same `DescribeTaskDefinition` call made for `ecr` discovery.
- **Count shown**: yes.

### `role`

- **Why related**: Service role + task-level execution and task roles — IAM surface that gates service operations and in-container AWS access.
- **How discovered**: read field `Service.RoleArn` (legacy ELB-registration role); additionally resolve `Service.TaskDefinition` → `TaskDefinition.ExecutionRoleArn` and `TaskDefinition.TaskRoleArn`; cross-reference the already-loaded `role` list by `Role.Arn` — a9s-devops-persona: all three role ARNs matter for troubleshooting; the task definition is already being fetched for `ecr` and `logs`, so no additional calls.
- **Count shown**: yes.

### `secrets`

- **Why related**: Secrets Manager secrets injected as container env vars — rotation or deletion here surfaces as task-launch failures.
- **How discovered**: resolve `Service.TaskDefinition` → `ContainerDefinitions[].Secrets[]` where `ValueFrom` is a Secrets Manager ARN (shape `arn:aws:secretsmanager:...:secret:<name>-<6char>`); cross-reference the already-loaded `secrets` list by `SecretListEntry.ARN` — a9s-devops-persona: ECS resolves these at task start; a deleted-or-scheduled-for-deletion secret is a common failure cause. `ValueFrom` also accepts SSM ARNs, which are handled under the `ssm` pivot on `ecs-task`.
- **Count shown**: yes.

### `sfn`

- **Why related**: Step Functions state machines that launch tasks in this service via the `ecs:runTask.sync` integration — relevant for orchestrated batch/pipeline workloads.
- **How discovered**: no direct field on the service; detection requires scanning already-loaded `sfn` state-machine definitions for `ecs:runTask.sync` steps referencing this cluster + task definition — a9s-devops-persona: the linkage lives inside the state-machine definition JSON (not in a tag or structured field on the service). Worth it because when a pipeline breaks, operators ask "what state machine is driving this?". Implementation depends on whether `sfn` definitions are already materialised; if not, fall back to cache-miss (pivot shown with count unknown).
- **Count shown**: unknown — a9s-devops-persona: depends on whether SFN definitions are preloaded in the cross-reference cache; if only the list is loaded, count is 0 or omitted rather than fetched live.

### `sg`

- **Why related**: Security groups attached to the service's awsvpc-mode ENIs — network-reachability troubleshooting.
- **How discovered**: read field `Service.NetworkConfiguration.AwsvpcConfiguration.SecurityGroups[]`; cross-reference the already-loaded `sg` list by `SecurityGroup.GroupId`. a9s-devops-persona: only populated for services using awsvpc networking (Fargate is always awsvpc; EC2 launch type may be bridge/host, in which case the count is 0).
- **Count shown**: yes.

### `subnet`

- **Why related**: Subnets the service's tasks launch into — AZ coverage and IP-exhaustion troubleshooting.
- **How discovered**: read field `Service.NetworkConfiguration.AwsvpcConfiguration.Subnets[]`; cross-reference the already-loaded `subnet` list by `Subnet.SubnetId`. Populated only for awsvpc networking.
- **Count shown**: yes.

### `tg`

- **Why related**: Target groups receiving the service's tasks as targets — load-balancer health and routing.
- **How discovered**: read field `Service.LoadBalancers[].TargetGroupArn`; cross-reference the already-loaded `tg` list by `TargetGroup.TargetGroupArn`.
- **Count shown**: yes.

### `vpc`

- **Why related**: Parent VPC of the service's subnets — network-boundary context.
- **How discovered**: derive via `Service.NetworkConfiguration.AwsvpcConfiguration.Subnets[0]` → already-loaded `subnet` entry → `Subnet.VpcId`; cross-reference the already-loaded `vpc` list by `Vpc.VpcId`. a9s-devops-persona: VPC is never on the Service object directly; the subnet list is the only reliable hop.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Audit trail for service changes (UpdateService, CreateService, DeleteService, force-new-deployment). Universal pivot — applies to every registered type; see related-resources.md §Policy.
- **How discovered**: `LookupEvents(ResourceName=<serviceName>)` or filter by `EventSource=ecs.amazonaws.com` and `Resources[].ResourceName`.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

> Note: every Wave 1 signal below reads a field from the `DescribeServices` response body. ECS's `ListServices` returns ARNs only, so a9s already issues `DescribeServices` as part of the "list load" for this type; Wave 1 here means "readable from the list-load response without any additional API call."

- **Signal**: `Service.status == "ACTIVE"`.
  - **State bucket**: Healthy.
  - **How obtained**: `Service.Status` field on the list-load response.

- **Signal**: `Service.status == "DRAINING"`.
  - **State bucket**: Dim.
  - **How obtained**: `Service.Status` field on the list-load response.

- **Signal**: `Service.status == "INACTIVE"`.
  - **State bucket**: Broken.
  - **How obtained**: `Service.Status` field on the list-load response.

- **Signal**: `Service.runningCount < Service.desiredCount`.
  - **State bucket**: Warning.
  - **How obtained**: compare `Service.RunningCount` and `Service.DesiredCount` on the list-load response. No awareness of active deployments at this stage — a running deployment is the common benign cause and is distinguished in Wave 2.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

> Note: these signals reuse the same `DescribeServices` response body Wave 1 reads (deployments, events are on the Service object) — no additional API call is required. The Wave-1/Wave-2 split here is about *what a9s interprets*, not about extra calls; see attention-signals.md for the canonical classification.

- **Signal**: `Service.deployments[].rolloutState == FAILED`.
  - **State bucket**: Broken.
  - **API call**: none beyond list-load (`DescribeServices` response already carries `deployments[]`).
  - **Cost shape**: per-resource (amortised — `DescribeServices` batches 10 services per call).

- **Signal**: `Service.runningCount < Service.desiredCount` AND no active `deployments[]` entry with `status == "PRIMARY"` and `rolloutState == "IN_PROGRESS"`.
  - **State bucket**: Broken.
  - **API call**: none beyond list-load.
  - **Cost shape**: per-resource (amortised).

- **Signal**: `Service.events[]` contains a message matching `"unable to place"` OR `"ELB health checks failed"` with `createdAt` within the last 10 minutes.
  - **State bucket**: Broken.
  - **API call**: none beyond list-load (`Events[]` is a rolling buffer of up to 100 most-recent events on the Service object).
  - **Cost shape**: per-resource (amortised).

- **Signal**: deployment circuit-breaker triggered (inferred from `Service.deployments[]` entry with `rolloutState == "FAILED"` plus `DeploymentConfiguration.DeploymentCircuitBreaker.Enable == true`).
  - **State bucket**: Broken.
  - **API call**: none beyond list-load.
  - **Cost shape**: per-resource (amortised).

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `CPUUtilization`/`MemoryUtilization` p99 per service.

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
| `status == DRAINING` | 1 | Dim | n/a | S2, S4 | `draining` | `Service is draining; tasks shutting down ahead of delete.` |
| `status == INACTIVE` | 1 | Broken | n/a | S2, S4 | `inactive` | `Service is inactive — not scheduling tasks; safe to delete.` |
| `runningCount < desiredCount` (Wave 1, no context) | 1 | Warning | n/a | S2, S4 | `running 2/4` | `Running below desired count — deployment in progress or capacity short.` |
| `deployments[].rolloutState == FAILED` | 2 | Broken | `!` | S2, S4, S5, S1 (via Broken color + finding count) | `deploy failed` | `Latest deployment failed to reach steady state; rollout halted.` |
| `runningCount < desiredCount` AND no IN_PROGRESS deployment | 2 | Broken | `!` | S2, S4, S5, S1 | `running 2/4: no active deploy` | `Task shortfall without an active deployment — placement or health-check blocked.` |
| `events[]` matches `unable to place` ≤10m | 2 | Broken | `!` | S2, S4, S5, S1 | `unable to place` | `Scheduler cannot place tasks — check subnet IPs, capacity providers, constraints.` |
| `events[]` matches `ELB health checks failed` ≤10m | 2 | Broken | `!` | S2, S4, S5, S1 | `ELB health checks failed` | `Tasks killed by load balancer — target group health-check is failing.` |
| deployment circuit-breaker triggered | 2 | Broken | `!` | S2, S4, S5, S1 | `circuit breaker` | `Circuit breaker halted the rollout after repeated task failures.` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`. Applied.
- A bare state keyword (`DORMANT`, `stopped`, `available`, `failed`, `ACTIVE`, `INACTIVE`, `DRAINING`) in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column. Applied — every row above pairs state with the operator-visible reason (e.g. `running 2/4: no active deploy` rather than bare `INACTIVE`).
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), the row is omitted — `ACTIVE` healthy services render S4 blank and do not appear in §4.
- Keep both columns short enough to fit: List text ≤ 40 chars, Detail text ≤ 100 chars. Verified on every row.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every red row carries a specific cause in the Status column (`deploy failed`, `running 2/4: no active deploy`, `unable to place`, `ELB health checks failed`, `circuit breaker`), so the on-call engineer can triage (capacity vs. health-check vs. scheduler) directly from the list; the detail pane is only needed to read the full event message and pivot to `tg` / `ecs-task` / `logs`.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): CloudWatch CPU/Memory p99 per service.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings, derived list-level banners, ceremonial "Background Check" headers.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- Per-resource CloudWatch metric sampling (budget excluded — Wave 3).

## 6. Citations

One bullet per claim in §§2–4.1. Citation sources, in order of authority:

- `ecs-svc` related-targets contract — `docs/related-resources.md` § `Per-type contract` row `ecs-svc` (16 targets: `alarm, cfn, ct-events, eb-rule, ecr, ecs, ecs-task, elb, logs, role, secrets, sfn, sg, subnet, tg, vpc`).
- `sg` discovered via `AwsvpcConfiguration.SecurityGroups` — `docs/related-resources.md` § `ecs-svc`.
- `subnet` discovered via `AwsvpcConfiguration.Subnets` — `docs/related-resources.md` § `ecs-svc`.
- `tg` discovered via `Service.LoadBalancers[].TargetGroupArn` — `docs/related-resources.md` § `ecs-svc`.
- `vpc` implied via `AwsvpcConfiguration` subnets — `docs/related-resources.md` § `ecs-svc`.
- `role` via `Service.RoleArn` — `docs/related-resources.md` § `ecs-svc`.
- `ecs` as parent cluster — `docs/related-resources.md` § `ecs-svc`.
- `ecs-task.Group == "service:<name>"` — `docs/related-resources.md` § `ecs-task`.
- `elb` via TG hop — `docs/related-resources.md` § `ecs-svc` (`Load balancer fronting the service (via TG)`).
- `eb-rule` as scheduled-tasks pivot — `docs/related-resources.md` § `ecs-svc` (`Scheduled tasks are EB-driven`).
- `cfn` as stack provenance — `docs/related-resources.md` § `ecs-svc`.
- `alarm` as CPU/Memory/PendingTasks watchers — `docs/related-resources.md` § `ecs-svc`.
- `logs` as task container logs — `docs/related-resources.md` § `ecs-svc`.
- `ct-events` universal pivot — `docs/related-resources.md` § `Policy` item 4.
- `ecr`, `secrets`, `sfn` listed in contract with audit-count reasoning — `docs/related-resources.md` § `ecs-svc` (`Mentioned by 1/6 independent DevOps audits`).
- a9s-devops persona — `ecr` workflow rationale (task-definition parse, image-URI match) — persona (2026-04-20): possible=yes, worth=yes. Task definition is the only place that names images; operators pivot from failing service to "is the image still there?". No extra calls beyond one `DescribeTaskDefinition` per distinct task-def ARN.
- a9s-devops persona — `logs` workflow rationale (awslogs driver + `awslogs-group` option) — persona (2026-04-20): possible=yes, worth=yes. awslogs is the overwhelmingly common driver; reuses the same `DescribeTaskDefinition` call.
- a9s-devops persona — `secrets` workflow rationale (container env `Secrets[].ValueFrom`) — persona (2026-04-20): possible=yes, worth=yes. Deleted-or-scheduled-for-deletion secrets are a common task-launch failure cause; reuses the task-def call.
- a9s-devops persona — `sfn` pivot is indirect — persona (2026-04-20): possible=yes, worth=yes (if SFN definitions preloaded), else count unknown. No first-class field on the service links to SFN; detection requires scanning state-machine definition JSON for `ecs:runTask.sync` steps. When pipelines break, operators ask "what state machine drove this run?", so the pivot stays in the panel with count=unknown rather than being dropped.
- a9s-devops persona — `alarm` via `ClusterName`+`ServiceName` dimension pair — persona (2026-04-20): possible=yes, worth=yes. `AWS/ECS` namespace alarms use this exact dimension pair; pure cache scan.
- a9s-devops persona — `eb-rule` via `ListTargetsByRule` `EcsParameters` — persona (2026-04-20): possible=yes, worth=yes. EventBridge is the canonical way to schedule ECS tasks; the target carries the cluster ARN.
- a9s-devops persona — `vpc` must route through `subnet` — persona (2026-04-20): possible=yes, worth=yes. No VpcId on the Service object; subnet is the only reliable hop.
- a9s-devops persona — Wave-1/Wave-2 split is interpretive on ECS — persona (2026-04-20): possible=yes, worth=yes. `ListServices` returns ARNs only; `DescribeServices` is always issued as part of list-load, so "Wave 2" for this resource does not add API calls, it adds interpretation. Noted inline in §3 so the spec doesn't mislead a reader about cost shape.
- AWS SDK Go v2 — `Service.Status` string field with values `ACTIVE`/`DRAINING`/`INACTIVE` — `AWS SDK Go v2 — service/ecs/types.Service § Status`.
- AWS SDK Go v2 — `Service.RunningCount`/`Service.DesiredCount` int32 — `AWS SDK Go v2 — service/ecs/types.Service § RunningCount, DesiredCount`.
- AWS SDK Go v2 — `Service.Deployments[]` list of `Deployment` — `AWS SDK Go v2 — service/ecs/types.Service § Deployments`.
- AWS SDK Go v2 — `Deployment.RolloutState` values `IN_PROGRESS`/`COMPLETED`/`FAILED` — `AWS SDK Go v2 — service/ecs/types.Deployment § RolloutState`.
- AWS SDK Go v2 — `Deployment.Status` values `PRIMARY`/`ACTIVE`/`INACTIVE` — `AWS SDK Go v2 — service/ecs/types.Deployment § Status`.
- AWS SDK Go v2 — `Service.Events[]` list of `ServiceEvent{CreatedAt, Id, Message}` capped at most-recent 100 — `AWS SDK Go v2 — service/ecs/types.Service § Events` and `types.ServiceEvent`.
- AWS SDK Go v2 — `Service.NetworkConfiguration.AwsvpcConfiguration.{Subnets, SecurityGroups}` — `AWS SDK Go v2 — service/ecs/types.Service § NetworkConfiguration`.
- AWS SDK Go v2 — `Service.LoadBalancers[].TargetGroupArn` — `AWS SDK Go v2 — service/ecs/types.Service § LoadBalancers`.
- AWS SDK Go v2 — `Service.ClusterArn`, `Service.TaskDefinition`, `Service.RoleArn` — `AWS SDK Go v2 — service/ecs/types.Service § ClusterArn, TaskDefinition, RoleArn`.
- AWS SDK Go v2 — `ListServicesOutput.ServiceArns` returns ARNs only (no status fields) — `AWS SDK Go v2 — service/ecs.ListServicesOutput § ServiceArns`. Justifies the §1 note that `DescribeServices` is the effective list API.
- Read-only invariant — `docs/architecture.md` § `What is a9s?`.
- Wave classification and ECS signal set — `docs/attention-signals.md` § `Containers / Compute` row `ecs-svc`.
