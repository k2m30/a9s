---
shortName: tg
name: Target Groups
awsApiRef: https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_TargetGroup.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# tg — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `tg`
- **Display name**: Target Groups
- **AWS API reference**: <https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_TargetGroup.html>
- **List API**: `elbv2:DescribeTargetGroups` (ELBv2 — Application, Network, and Gateway Load Balancers). Classic (ELBv1) does not have target groups; instances are registered directly on the LB.
- **Describe API (if any)**: `elbv2:DescribeTargetHealth(TargetGroupArn=<this>)` — one call per target group (Wave 2, bounded per-resource fan-out).

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `asg`, `backup`, `cfn`, `ct-events`, `dbc`, `dbi`, `ec2`, `ecs-svc`, `elb`, `lambda`, `logs`, `dbi-snap`, `sg`, `subnet`, `vpc`.

### `alarm`

- **Why related**: CloudWatch alarms on TG target-health (`UnHealthyHostCount`, `HealthyHostCount`) — the operator wants to know which alarms are watching this TG before deciding "healthy targets = site healthy."
- **How discovered**: cross-reference the already-loaded `alarm` list by `Dimensions[]` containing `{Name: "TargetGroup", Value: <targetgroup/<name>/<id>>}` (and, when paired, `{Name: "LoadBalancer", Value: <app/<name>/<id>>}`). — a9s-devops: possible=yes, worth=yes. CloudWatch publishes `AWS/ApplicationELB` and `AWS/NetworkELB` per-TG metrics under the `TargetGroup` dimension using the ARN-suffix form; this is the standard SRE join.
- **Count shown**: yes.

### `asg`

- **Why related**: Auto Scaling Groups register/deregister instances into this TG — answers "who owns the instances behind this TG?" during a 3am target-health incident.
- **How discovered**: cross-reference the already-loaded `asg` list by `AutoScalingGroup.TargetGroupARNs[]` containing this TG's `TargetGroupArn`.
- **Count shown**: yes.

### `backup`

- **Why related**: listed by related-resources.md contract. In practice, AWS Backup does not support target groups as a backup resource — TGs are configuration, not stateful data, and do not appear in AWS Backup's supported-services matrix.
- **How discovered**: no AWS field or API links a target group to a Backup plan or recovery point. — a9s-devops: possible=no, worth=no. This is recorded in §5 Out of Scope as an unfillable contract entry; removing it from the contract needs a separate amendment to `docs/related-resources.md`.
- **Count shown**: unknown.

### `cfn`

- **Why related**: CloudFormation stack that created the TG — lets the operator see whether the TG is IaC-managed and which stack owns it, the standard first question when TG config looks wrong.
- **How discovered**: call `elbv2:DescribeTags(ResourceArns=[<TG.TargetGroupArn>])` and read the `aws:cloudformation:stack-name` / `aws:cloudformation:stack-id` tags; cross-reference the already-loaded `cfn` list by stack name. — a9s-devops: possible=yes, worth=yes. CloudFormation stamps `aws:cloudformation:*` tags on every resource it creates, including ELBv2 target groups; this is the canonical CFN-ownership pivot.
- **Count shown**: yes.

### `dbc`

- **Why related**: listed by related-resources.md contract (1/6 audit mention). In practice, TG target types are `instance`, `ip`, `lambda`, and `alb` (per `types.TargetTypeEnum`) — DocumentDB clusters are not a routable target; clients reach DocDB via its own endpoint, not a load balancer.
- **How discovered**: no AWS field on `TargetGroup` or `DescribeTargetHealth` references a DocumentDB cluster. — a9s-devops: possible=no, worth=no. Recorded in §5.
- **Count shown**: unknown.

### `dbi`

- **Why related**: listed by related-resources.md contract (1/6 audit mention). Same reasoning as `dbc` — RDS DB instances are not a valid TG target type.
- **How discovered**: no AWS field on `TargetGroup` or `DescribeTargetHealth` references an RDS instance. — a9s-devops: possible=no, worth=no. Recorded in §5.
- **Count shown**: unknown.

### `ec2`

- **Why related**: EC2 instances registered as `instance`-type targets — the actual workloads behind the TG. When target-health flips to `unhealthy`, the operator pivots to the instance to check what crashed.
- **How discovered**: call `elbv2:DescribeTargetHealth(TargetGroupArn=<this>)`, collect `TargetHealthDescriptions[].Target.Id` where the target group's `TargetType == instance`, and cross-reference the already-loaded `ec2` list by instance ID.
- **Count shown**: yes.

### `ecs-svc`

- **Why related**: ECS services that register tasks into this TG — when TG target health drops, the usual owner is the ECS service fronting it.
- **How discovered**: cross-reference the already-loaded `ecs-svc` list by `Service.loadBalancers[].targetGroupArn` containing this TG's `TargetGroupArn`.
- **Count shown**: yes.

### `elb`

- **Why related**: the Load Balancers forwarding traffic to this TG — the other side of the traffic path. An orphan TG (no LB) shows up here as "no entries".
- **How discovered**: read `TargetGroup.LoadBalancerArns[]` on the list response and cross-reference the already-loaded `elb` list by `LoadBalancerArn`.
- **Count shown**: yes.

### `lambda`

- **Why related**: Lambda function registered as a `lambda`-type target (ALB → Lambda integration) — the actual handler behind the TG.
- **How discovered**: call `elbv2:DescribeTargetHealth(TargetGroupArn=<this>)` when the TG's `TargetType == lambda`, collect `TargetHealthDescriptions[].Target.Id` (which is the Lambda function ARN), and cross-reference the already-loaded `lambda` list. — a9s-devops: possible=yes, worth=yes. The `TargetType == lambda` registration is the documented ALB→Lambda path; `Target.Id` is the function ARN.
- **Count shown**: yes.

### `logs`

- **Why related**: listed by related-resources.md contract (2/6 audit mention). In practice target groups do not emit CloudWatch Logs — access logs from the parent ELB go to S3 (`DescribeLoadBalancerAttributes`), not CloudWatch Logs.
- **How discovered**: no AWS field on `TargetGroup` references a CloudWatch log group. — a9s-devops: possible=no, worth=no. Recorded in §5.
- **Count shown**: unknown.

### `dbi-snap`

- **Why related**: listed by related-resources.md contract (2/6 audit mention). Same reasoning as `dbi`/`dbc` — RDS snapshots are not a TG target and share no AWS-API field with TGs.
- **How discovered**: no AWS field on `TargetGroup` or `DescribeTargetHealth` references an RDS snapshot. — a9s-devops: possible=no, worth=no. Recorded in §5.
- **Count shown**: unknown.

### `sg`

- **Why related**: listed by related-resources.md contract (1/6 audit mention). Security groups attach to ENIs (LB listeners, instances), not to target groups — `TargetGroup` has no `SecurityGroups` field. The right pivot for SG inspection is the parent `elb` (ALB has `SecurityGroups[]`) or the registered instances.
- **How discovered**: no AWS field on `TargetGroup` references a security group. — a9s-devops: possible=no, worth=no at the TG level; use `elb` → `sg` instead. Recorded in §5.
- **Count shown**: unknown.

### `subnet`

- **Why related**: listed by related-resources.md contract (1/6 audit mention). Target groups are not subnet-scoped — the parent LB occupies subnets via `AvailabilityZones[].SubnetId`; `TargetGroup` has no subnet field.
- **How discovered**: no AWS field on `TargetGroup` references a subnet. — a9s-devops: possible=no, worth=no at the TG level; use `elb` → `subnet` instead. Recorded in §5.
- **Count shown**: unknown.

### `vpc`

- **Why related**: `TargetGroup.VpcId` — the VPC this TG is scoped to (TGs with `TargetType == lambda` are not VPC-scoped and may omit this field).
- **How discovered**: read `TargetGroup.VpcId` on the list response and cross-reference the already-loaded `vpc` list.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Audit trail for target group config changes (registrations/deregistrations, health-check settings) — universal "who changed what, when" pivot.
- **How discovered**: universal pivot — applies to every registered type; see related-resources.md §Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `LoadBalancerArns == []` (orphan — TG is not attached to any load balancer).
  - **State bucket**: Warning.
  - **How obtained**: `elbv2:DescribeTargetGroups` response field `TargetGroup.LoadBalancerArns` is an empty slice.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: any target `TargetHealth.State == unhealthy` (at least one registered target is failing health checks, but not all).
  - **State bucket**: Warning.
  - **API call**: `elbv2:DescribeTargetHealth(TargetGroupArn=<this>)` — one call per target group.
  - **Cost shape**: per-resource.
- **Signal**: all targets `TargetHealth.State == unhealthy` (every registered target is failing — TG is effectively down, and if this TG fronts user traffic the site is down).
  - **State bucket**: Broken.
  - **API call**: `elbv2:DescribeTargetHealth(TargetGroupArn=<this>)` — one call per target group.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `UnHealthyHostCount` / `HealthyHostCount` ratio trends per TG.

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
| `LoadBalancerArns == []` | 1 | Warning | n/a | S2, S4 | `orphan: no load balancer` | `Target group is not attached to any load balancer — receiving no traffic.` |
| any target `unhealthy` (not all) | 2 | Warning | n/a | S2, S4, S5 | `unhealthy: <K>/<N> targets` | `<K> of <N> targets failing health checks — see detail for per-target reason.` |
| all targets `unhealthy` | 2 | Broken | n/a | S2, S4, S5 | `all targets unhealthy` | `Every registered target is failing health checks — TG is down.` |

Notes:

- The Wave 2 findings land on rows that are Healthy (green) at Wave 1 unless Wave 1 already flagged them (e.g. orphan). For the Warning row, severity is `n/a` rather than `~` because the row color is already yellow — S2 is the attention signal; S3 would be redundant and is suppressed. For the Broken row (all targets unhealthy), the row color is already red; S1 still counts this as an `!`-equivalent issue because it represents a user-facing outage, but the glyph is suppressed per the "already red" rule.
- List-text `<K>/<N>` is derived from the `DescribeTargetHealth` response: `K = count(TargetHealthDescriptions where TargetHealth.State == "unhealthy")`, `N = len(TargetHealthDescriptions)`. Detail-text source for per-target reason: `TargetHealth.Reason` (e.g. `Target.Timeout`, `Target.ResponseCodeMismatch`, `Target.FailedHealthChecks`) and `TargetHealth.Description`.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — an orphan TG reads `orphan: no load balancer` (yellow), a partial outage reads `unhealthy: K/N targets` (yellow), and a total outage reads `all targets unhealthy` (red); the operator knows both the scope (how many) and the next pivot (`elb` for orphan, `ec2`/`lambda` for target failures) without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): CloudWatch `UnHealthyHostCount` / `HealthyHostCount` ratio trends per TG.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- `backup` as a related target — a9s-devops: possible=no, worth=no. AWS Backup does not list target groups as a supported resource; TGs are configuration, not stateful data. Contract-level entry in `related-resources.md` appears to be audit-pattern inertia.
- `dbc`, `dbi`, `dbi-snap` as related targets — a9s-devops: possible=no, worth=no. TG target types (`instance`, `ip`, `lambda`, `alb` per `types.TargetTypeEnum`) do not include RDS/DocumentDB; no AWS field links a TG to a DB instance, DB cluster, or RDS snapshot.
- `logs` as a related target — a9s-devops: possible=no, worth=no. Target groups do not emit CloudWatch Logs; ELB access logs go to S3 via `DescribeLoadBalancerAttributes` on the parent `elb`, not to a log group on the TG.
- `sg` as a related target **at the TG level** — a9s-devops: possible=no, worth=no. `TargetGroup` has no `SecurityGroups` field; the SG pivot belongs to the parent `elb` (ALB `SecurityGroups[]`) or to the registered instances/ENIs.
- `subnet` as a related target **at the TG level** — a9s-devops: possible=no, worth=no. `TargetGroup` has no subnet field; the subnet pivot belongs to the parent `elb` via `AvailabilityZones[].SubnetId`.

## 6. Citations

- a9s golden doc — related panel contract (16 targets: `alarm`, `asg`, `backup`, `cfn`, `ct-events`, `dbc`, `dbi`, `ec2`, `ecs-svc`, `elb`, `lambda`, `logs`, `dbi-snap`, `sg`, `subnet`, `vpc`) — `docs/related-resources.md` § "Per-type contract" table row for `tg` and § `### tg`.
- a9s golden doc — universal pivot `ct-events` — `docs/related-resources.md` § "Policy" (universal pivots clause).
- a9s golden doc — Wave 1 / Wave 2 / Wave 3 signals and source API (`DescribeTargetHealth`) — `docs/attention-signals.md` § "Networking" table row for `tg`.
- a9s golden doc — read-only invariant — `docs/architecture.md` § "What is a9s?".
- a9s golden doc — `asg` → `tg` discovery via `AutoScalingGroup.TargetGroupARNs` — `docs/related-resources.md` § `### asg` ("tg — AutoScalingGroup.TargetGroupARNs").
- a9s golden doc — `ecs-svc` → `tg` discovery via `Service.loadBalancers[].targetGroupArn` — `docs/related-resources.md` § `### ecs-svc` ("tg — Service.LoadBalancers[].TargetGroupArn").
- AWS Go SDK v2 — `TargetGroup.LoadBalancerArns`, `TargetGroup.VpcId`, `TargetGroup.TargetGroupArn`, `TargetGroup.TargetType` field names — `AWS SDK Go v2 — elasticloadbalancingv2/types.TargetGroup § LoadBalancerArns, VpcId, TargetGroupArn, TargetType`.
- AWS Go SDK v2 — `TargetType` enum values `instance`, `ip`, `lambda`, `alb` — `AWS SDK Go v2 — elasticloadbalancingv2/types.TargetTypeEnum`.
- AWS Go SDK v2 — `DescribeTargetHealth` response shape and `TargetHealthDescription.Target.Id` / `TargetHealth.State` / `TargetHealth.Reason` / `TargetHealth.Description` — `AWS SDK Go v2 — elasticloadbalancingv2/types.TargetHealthDescription § Target, TargetHealth` and `elasticloadbalancingv2/types.TargetHealth § State, Reason, Description`.
- AWS Go SDK v2 — `TargetHealthStateEnum` values `initial`, `healthy`, `unhealthy`, `unhealthy.draining`, `unused`, `draining`, `unavailable` — `AWS SDK Go v2 — elasticloadbalancingv2/types.TargetHealthStateEnum`.
- a9s-devops consultation — `alarm` discovery via CloudWatch `TargetGroup` dimension with ARN-suffix value — `a9s-devops (2026-04-20): possible=yes, worth=yes. AWS/ApplicationELB and AWS/NetworkELB publish per-TG metrics with the TargetGroup dimension; standard SRE join.`
- a9s-devops consultation — `cfn` discovery via `aws:cloudformation:stack-name` tag fetched with `elbv2:DescribeTags` — `a9s-devops (2026-04-20): possible=yes, worth=yes. CFN stamps this tag on every created resource including ELBv2 TGs.`
- a9s-devops consultation — `lambda` discovery via `DescribeTargetHealth` when `TargetType == lambda`, `Target.Id` is the function ARN — `a9s-devops (2026-04-20): possible=yes, worth=yes. Documented ALB→Lambda path.`
- a9s-devops consultation — `backup` not a real pivot — `a9s-devops (2026-04-20): possible=no, worth=no. AWS Backup does not support target groups; TGs are configuration, not stateful data.`
- a9s-devops consultation — `dbc` / `dbi` / `dbi-snap` not real pivots — `a9s-devops (2026-04-20): possible=no, worth=no. TG TargetType enum is instance/ip/lambda/alb; databases are not a routable target.`
- a9s-devops consultation — `logs` not a real pivot — `a9s-devops (2026-04-20): possible=no, worth=no. TGs do not emit CloudWatch Logs; access logs live on the parent ELB in S3 via DescribeLoadBalancerAttributes.`
- a9s-devops consultation — `sg` not a TG-level pivot — `a9s-devops (2026-04-20): possible=no, worth=no. TargetGroup has no SecurityGroups field; SG pivot belongs to the parent ALB or the registered instances.`
- a9s-devops consultation — `subnet` not a TG-level pivot — `a9s-devops (2026-04-20): possible=no, worth=no. TargetGroup has no subnet field; subnet pivot lives on the parent ELB AvailabilityZones.SubnetId.`
- a9s-devops consultation — Wave 2 severity mapping ("any unhealthy" = Warning, "all unhealthy" = Broken) matches attention-signals.md verbatim — `a9s-devops (2026-04-20): possible=yes, worth=yes. "All targets unhealthy" is the user-facing-outage case and justifies the Broken bucket.`
