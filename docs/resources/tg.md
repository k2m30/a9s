---
shortName: tg
name: Target Groups
awsApiRef: https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_TargetGroup.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
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

Expected targets from `docs/related-resources.md` § Per-type contract: `alarm`, `asg`, `cfn`, `ct-events`, `ec2`, `ecs-svc`, `elb`, `lambda`, `vpc`.

### `alarm`

- **Why related**: CloudWatch alarms on TG target-health (`UnHealthyHostCount`, `HealthyHostCount`) — the operator wants to know which alarms are watching this TG before deciding "healthy targets = site healthy."
- **How discovered**: cross-reference the already-loaded `alarm` list by `Dimensions[]` containing `{Name: "TargetGroup", Value: <targetgroup/<name>/<id>>}` (and, when paired, `{Name: "LoadBalancer", Value: <app/<name>/<id>>}`). — a9s-devops: possible=yes, worth=yes. CloudWatch publishes `AWS/ApplicationELB` and `AWS/NetworkELB` per-TG metrics under the `TargetGroup` dimension using the ARN-suffix form; this is the standard SRE join.
- **Count shown**: yes.

### `asg`

- **Why related**: Auto Scaling Groups register/deregister instances into this TG — answers "who owns the instances behind this TG?" during a 3am target-health incident.
- **How discovered**: cross-reference the already-loaded `asg` list by `AutoScalingGroup.TargetGroupARNs[]` containing this TG's `TargetGroupArn`.
- **Count shown**: yes.

### `cfn`

- **Why related**: CloudFormation stack that created the TG — lets the operator see whether the TG is IaC-managed and which stack owns it, the standard first question when TG config looks wrong.
- **How discovered**: call `elbv2:DescribeTags(ResourceArns=[<TG.TargetGroupArn>])` and read the `aws:cloudformation:stack-name` / `aws:cloudformation:stack-id` tags; cross-reference the already-loaded `cfn` list by stack name. — a9s-devops: possible=yes, worth=yes. CloudFormation stamps `aws:cloudformation:*` tags on every resource it creates, including ELBv2 target groups; this is the canonical CFN-ownership pivot.
- **Count shown**: yes.

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

### `vpc`

- **Why related**: `TargetGroup.VpcId` — the VPC this TG is scoped to (TGs with `TargetType == lambda` are not VPC-scoped and may omit this field).
- **How discovered**: read `TargetGroup.VpcId` on the list response and cross-reference the already-loaded `vpc` list.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Audit trail for target group config changes (registrations/deregistrations, health-check settings) — universal "who changed what, when" pivot.
- **How discovered**: universal pivot — applies to every registered type; see docs/related-resources.md §Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeTargetHealth](https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_DescribeTargetHealth.html)

Transcribed from `docs/attention-signals.md § Signals § NETWORKING` row `tg`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

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

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `tg`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| any target `unhealthy` (not all) | 2 | Warning | `~` | S2, S3, S4, S5 | `unhealthy targets: <N>/<M>` |
| all targets `unhealthy` | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `all <N target(s)> unhealthy` |

Notes:

- Both Wave 2 findings colour the row themselves: yellow for unhealthy targets, red when every target is down. S2 is the attention signal in the list, and the Attention section carries the tier — `~` for the Warning row, `!` for the Broken one. Only the Broken one bumps S1, which is right for a user-facing outage.
- List-text `<K>/<N>` is derived from the `DescribeTargetHealth` response: `K = count(TargetHealthDescriptions where TargetHealth.State == "unhealthy")`, `N = len(TargetHealthDescriptions)`. The detail lists one `Unhealthy target` row per failing target, `<id>:<port> — <reason>`, from `Target.Id`, `Target.Port` and `TargetHealth.Reason` (e.g. `Target.Timeout`, `Target.ResponseCodeMismatch`, `Target.FailedHealthChecks`). The ratio is the phrase's, so no row repeats it.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — a partial outage shows a yellow row reading `unhealthy targets: <N>/<M>`, and a total outage shows a red row reading `all <N target(s)> unhealthy`; the operator knows the scope (how many) and the next pivot (`ec2` or `lambda` for target failures) without opening detail. A target group with no load balancer carries no finding, so it reads as healthy — the `elb` pivot is the way to see that.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): CloudWatch `UnHealthyHostCount` / `HealthyHostCount` ratio trends per TG.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- `backup`, `dbc`, `dbi`, `dbi-snap`, `logs`, `sg` and `subnet` as related targets — `docs/related-resources.md` § Explicitly excluded.

## 6. Citations

- a9s golden doc — related panel contract (9 targets: `alarm`, `asg`, `cfn`, `ct-events`, `ec2`, `ecs-svc`, `elb`, `lambda`, `vpc`) — `docs/related-resources.md` § Per-type contract, row `tg`, and `docs/related-resources.md` § `tg`.
- a9s golden doc — universal pivot `ct-events` — `docs/related-resources.md` § Policy (universal pivots clause).
- a9s golden doc — the `tg` signals, read from `DescribeTargetHealth` — `docs/attention-signals.md § Signals § NETWORKING` row `tg`; the deferred CloudWatch ratios — `docs/attention-signals.md § Not yet implemented`.
- a9s golden doc — read-only invariant — `docs/architecture.md` § "What is a9s?".
- a9s golden doc — `asg` → `tg` discovery via `AutoScalingGroup.TargetGroupARNs` — `docs/related-resources.md` § `asg` ("tg — AutoScalingGroup.TargetGroupARNs").
- a9s golden doc — `ecs-svc` → `tg` discovery via `Service.loadBalancers[].targetGroupArn` — `docs/related-resources.md` § `ecs-svc` ("tg — Service.LoadBalancers[].TargetGroupArn").
- AWS Go SDK v2 — `TargetGroup.LoadBalancerArns`, `TargetGroup.VpcId`, `TargetGroup.TargetGroupArn`, `TargetGroup.TargetType` field names — `AWS SDK Go v2 — elasticloadbalancingv2/types.TargetGroup § LoadBalancerArns, VpcId, TargetGroupArn, TargetType`.
- AWS Go SDK v2 — `TargetType` enum values `instance`, `ip`, `lambda`, `alb` — `AWS SDK Go v2 — elasticloadbalancingv2/types.TargetTypeEnum`.
- AWS Go SDK v2 — `DescribeTargetHealth` response shape and `TargetHealthDescription.Target.Id` / `TargetHealth.State` / `TargetHealth.Reason` / `TargetHealth.Description` — `AWS SDK Go v2 — elasticloadbalancingv2/types.TargetHealthDescription § Target, TargetHealth` and `elasticloadbalancingv2/types.TargetHealth § State, Reason, Description`.
- AWS Go SDK v2 — `TargetHealthStateEnum` values `initial`, `healthy`, `unhealthy`, `unhealthy.draining`, `unused`, `draining`, `unavailable` — `AWS SDK Go v2 — elasticloadbalancingv2/types.TargetHealthStateEnum`.
- a9s-devops consultation — `alarm` discovery via CloudWatch `TargetGroup` dimension with ARN-suffix value — `a9s-devops (2026-04-20): possible=yes, worth=yes. AWS/ApplicationELB and AWS/NetworkELB publish per-TG metrics with the TargetGroup dimension; standard SRE join.`
- a9s-devops consultation — `cfn` discovery via `aws:cloudformation:stack-name` tag fetched with `elbv2:DescribeTags` — `a9s-devops (2026-04-20): possible=yes, worth=yes. CFN stamps this tag on every created resource including ELBv2 TGs.`
- a9s-devops consultation — `lambda` discovery via `DescribeTargetHealth` when `TargetType == lambda`, `Target.Id` is the function ARN — `a9s-devops (2026-04-20): possible=yes, worth=yes. Documented ALB→Lambda path.`
- a9s-devops consultation — `logs` not a real pivot — `a9s-devops (2026-04-20): possible=no, worth=no. TGs do not emit CloudWatch Logs; access logs live on the parent ELB in S3 via DescribeLoadBalancerAttributes.`
- a9s-devops consultation — `sg` not a TG-level pivot — `a9s-devops (2026-04-20): possible=no, worth=no. TargetGroup has no SecurityGroups field; SG pivot belongs to the parent ALB or the registered instances.`
- a9s-devops consultation — `subnet` not a TG-level pivot — `a9s-devops (2026-04-20): possible=no, worth=no. TargetGroup has no subnet field; subnet pivot lives on the parent ELB AvailabilityZones.SubnetId.`
- a9s-devops consultation — Wave 2 severity mapping ("any unhealthy" = Warning, "all unhealthy" = Broken) matches `docs/attention-signals.md § Signals § NETWORKING` row `tg` — `a9s-devops (2026-04-20): possible=yes, worth=yes. "All targets unhealthy" is the user-facing-outage case and justifies the Broken bucket.`

<!-- BEGIN GENERATED: header -->
tg — NETWORKING. Lifecycle key: none (the list API returns no lifecycle field).
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| tg.all-targets-unhealthy | all <N target(s)> unhealthy | broken | wave2 | Every registered target is failing its health check, so the load balancer has nowhere to send a request and whatever sits in front of this group is down. Check the targets themselves, then the health-check path, port and matcher the group is configured with. |
| tg.unhealthy-targets | unhealthy targets: <N>/<M> | warn | wave2 | Some of this group's targets are failing their health checks, so every request is landing on the ones that are left. Each failing target is listed with the reason its health check gave; fix or replace them before the remaining targets run out of headroom. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| elb | Load Balancers | yes |
| ecs-svc | ECS Services | yes |
| asg | Auto Scaling Groups | yes |
| alarm | CW Alarms | yes |
| vpc | VPC | no |
| cfn | CloudFormation | no |
| ec2 | EC2 Instances | no |
| lambda | Lambda Functions | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
