---
shortName: elb
name: Load Balancers
awsApiRef: https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_LoadBalancer.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# elb — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `elb`
- **Display name**: Load Balancers
- **AWS API reference**: <https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_LoadBalancer.html>
- **List API**: `DescribeLoadBalancers` (ELBv2). Classic (ELBv1) uses its own `DescribeLoadBalancers` on the `elasticloadbalancing` (v1) endpoint and returns a separate shape with no `State` field.
- **Describe API (if any)**: not used (Wave 2 is `None` for this type).

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `acm`, `alarm`, `cf`, `cfn`, `ct-events`, `eni`, `r53`, `s3`, `sg`, `subnet`, `tg`, `vpc`, `waf`.

### `acm`

- **Why related**: HTTPS listener certificate — the ACM cert that terminates TLS on this LB's HTTPS/TLS listeners.
- **How discovered**: call `elbv2:DescribeListeners(LoadBalancerArn=<this>)` and collect `Certificates[].CertificateArn` from each listener; cross-reference against the already-loaded `acm` list by ARN. `TBD — not specified in related-resources.md` whether discovery is at detail-open time or earlier; the contract row does not fix the moment.
- **Count shown**: yes.

### `alarm`

- **Why related**: CloudWatch alarms on LB metrics (4xx/5xx/latency) — the operator wants to know which alarms watch this LB's health before deciding the LB is fine.
- **How discovered**: cross-reference the already-loaded `alarm` list by `Dimensions[].Value` matching this LB's `LoadBalancerArn` suffix (`app/<name>/<id>`) or Classic `LoadBalancerName`. — a9s-devops: possible=yes, worth=yes. CloudWatch alarm dimensions for `AWS/ApplicationELB`/`AWS/NetworkELB` use `LoadBalancer` dimension with the `app/<name>/<id>` suffix; for Classic it is `LoadBalancerName`.
- **Count shown**: yes.

### `cf`

- **Why related**: ALB as CloudFront origin — the LB may be fronted by a CDN, which changes the blast radius when it is unhealthy.
- **How discovered**: cross-reference the already-loaded `cf` list by `Origins.Items[].DomainName` matching this LB's `DNSName`. — a9s-devops: possible=yes, worth=yes. CloudFront `Distribution.Origins.Items[].DomainName` holds a fully-qualified DNS name; the a9s `cf` list already includes origin data per the `cf` contract row pointing at `elb`.
- **Count shown**: yes.

### `cfn`

- **Why related**: CloudFormation stack that created the LB — lets the operator see whether the LB is managed IaC and which stack owns it.
- **How discovered**: read the `Tags` on the LB for `aws:cloudformation:stack-name` / `aws:cloudformation:stack-id`; cross-reference the already-loaded `cfn` list by stack name. — a9s-devops: possible=yes, worth=yes. CloudFormation stamps `aws:cloudformation:*` tags on every resource it creates, including ELBs.
- **Count shown**: yes.

### `eni`

- **Why related**: LB creates ENIs per AZ — the ENIs are the actual IPs clients connect to; they reveal AZ placement and whether the LB is really wired up.
- **How discovered**: cross-reference the already-loaded `eni` list by `Description` starting with `ELB app/<name>/<id>` (ALB) or `ELB net/<name>/<id>` (NLB) or `ELB <name>` (Classic) and/or `RequesterId` indicating the ELB service. — a9s-devops: possible=yes, worth=yes. ELB-owned ENIs have a well-known Description prefix that references the LB; this is the standard pivot used by SREs today.
- **Count shown**: yes.

### `r53`

- **Why related**: Route 53 alias/records pointing at this LB — answers "which hostname resolves here?", the #1 question an operator asks when an LB misbehaves.
- **How discovered**: cross-reference the already-loaded `r53` list: within each hosted zone, record sets where `AliasTarget.DNSName` equals this LB's `DNSName` (with/without trailing dot). — a9s-devops: possible=yes, worth=yes. Route 53 alias records to ELBs carry the LB's `DNSName` in `AliasTarget.DNSName`; this is the canonical join.
- **Count shown**: yes.

### `s3`

- **Why related**: Access-log S3 destination — lets the operator jump to the bucket receiving access logs when debugging.
- **How discovered**: `TBD — a9s-devops: not available in AWS surface without a per-LB Describe call.` The access-log bucket lives in `DescribeLoadBalancerAttributes` (Wave 3 per `attention-signals.md`), not on the list response. a9s-devops: possible=yes via `DescribeLoadBalancerAttributes` (N+1), worth=no at list time — the related panel would require a bounded fan-out this type has explicitly deferred to Wave 3. Related-panel pivot is still documented as a contract target; discovery is deferred until this resource moves to Wave 2/3.
- **Count shown**: unknown.

### `sg`

- **Why related**: Attached security groups (ALB only) — the SGs that gate traffic to the LB's listeners.
- **How discovered**: read `LoadBalancer.SecurityGroups[]` on the list response (ELBv2 ALB) and cross-reference the already-loaded `sg` list. NLB/GWLB and Classic do not use SGs in the same way; the field is present on ALB only.
- **Count shown**: yes.

### `subnet`

- **Why related**: AZ subnets the LB listens in — shows the network surface and where the ENIs are placed.
- **How discovered**: read `LoadBalancer.AvailabilityZones[].SubnetId` on the list response and cross-reference the already-loaded `subnet` list.
- **Count shown**: yes.

### `tg`

- **Why related**: Target groups attached to this LB — where traffic actually goes; target health lives on `tg`, not on `elb`.
- **How discovered**: cross-reference the already-loaded `tg` list by `TargetGroup.LoadBalancerArns[]` containing this LB's `LoadBalancerArn`.
- **Count shown**: yes.

### `vpc`

- **Why related**: `LoadBalancer.VpcId` — the VPC this LB lives in.
- **How discovered**: read `LoadBalancer.VpcId` on the list response and cross-reference the already-loaded `vpc` list.
- **Count shown**: yes.

### `waf`

- **Why related**: WebACL associated with ALB — lets the operator see whether incoming traffic is filtered by WAF before it reaches targets.
- **How discovered**: cross-reference the already-loaded `waf` list; WAFv2 `WebACL` associations to ALBs are resolved via `wafv2:ListResourcesForWebACL(WebACLArn, ResourceType=APPLICATION_LOAD_BALANCER)`. — a9s-devops: possible=yes, worth=yes. The `waf` contract row explicitly lists `elb` as a related target, and `ListResourcesForWebACL` is the documented reverse pivot.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Audit trail for LB config changes — universal "who changed what, when" pivot.
- **How discovered**: universal pivot — applies to every registered type; see related-resources.md §Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: ELBv2 `State.Code == active`.
  - **State bucket**: Healthy.
  - **How obtained**: `DescribeLoadBalancers` response field `LoadBalancer.State.Code`.
- **Signal**: ELBv2 `State.Code == provisioning`.
  - **State bucket**: Warning.
  - **How obtained**: `DescribeLoadBalancers` response field `LoadBalancer.State.Code`.
- **Signal**: ELBv2 `State.Code == active_impaired`.
  - **State bucket**: Warning.
  - **How obtained**: `DescribeLoadBalancers` response field `LoadBalancer.State.Code`.
- **Signal**: ELBv2 `State.Code == failed`; surface `State.Reason` as the Broken detail.
  - **State bucket**: Broken.
  - **How obtained**: `DescribeLoadBalancers` response fields `LoadBalancer.State.Code` and `LoadBalancer.State.Reason`.
- **Signal**: Classic (ELBv1) Load Balancer — the list response has no `State` field, so no state-derived signal is available.
  - **State bucket**: Healthy (default — nothing to flag from the list response).
  - **How obtained**: absence of a state field on the ELBv1 `DescribeLoadBalancers` response. Target-health signals for Classic LBs live on `tg` (Wave 2 there), not here.

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals. (Target health lives on `tg`; that Wave 2 belongs to the target-group spec.)

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `HTTPCode_ELB_5XX_Count` per LB.
- OUT OF SCOPE: `DescribeLoadBalancerAttributes` per LB (deletion-protection, access-logs).

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
| `State.Code == provisioning` | 1 | Warning | n/a | S2, S4 | `provisioning: <State.Reason>` (fallback `provisioning: coming up`) | `Load balancer still provisioning — not yet accepting traffic.` |
| `State.Code == active_impaired` | 1 | Warning | n/a | S2, S4 | `impaired: <State.Reason>` (fallback `impaired: scaling behind`) | `Load balancer is routing but lacks resources to scale — AWS is degraded.` |
| `State.Code == failed` | 1 | Broken | n/a | S2, S4 | `failed: <State.Reason>` | `Load balancer could not be set up — see State.Reason for cause.` |

Healthy ELBv2 rows (`State.Code == active`) and Classic (ELBv1) rows are omitted from this table per the §4 rule: Healthy renders green with a blank Status column.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — yellow for `provisioning`/`active_impaired` and red for `failed` are paired with the AWS-provided `State.Reason` in the Status column, so the operator reads the cause inline. One UX gap: when `State.Reason` is empty (common during very early `provisioning`), the Status column falls back to a generic phrase; implementation should take the reason verbatim when non-empty and never show a bare state keyword.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): CloudWatch `HTTPCode_ELB_5XX_Count`; `DescribeLoadBalancerAttributes` per LB (deletion-protection, access-logs).
- Target-health signals (healthy/unhealthy target counts) — those live on `tg` per `attention-signals.md`, not on `elb`.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- `s3` related-panel discovery at list time — a9s-devops: not worth it, requires N+1 `DescribeLoadBalancerAttributes` fan-out that `attention-signals.md` explicitly defers to Wave 3; revisit if/when this resource adopts a Wave 2.

## 6. Citations

- a9s golden doc — related panel contract (13 targets: `acm`, `alarm`, `cf`, `cfn`, `ct-events`, `eni`, `r53`, `s3`, `sg`, `subnet`, `tg`, `vpc`, `waf`) — `docs/related-resources.md` § "Per-type contract" table row for `elb` and § `### elb`.
- a9s golden doc — universal pivot `ct-events` — `docs/related-resources.md` § "Policy" (universal pivots clause).
- a9s golden doc — Wave 1 / Wave 2 / Wave 3 signals and source API — `docs/attention-signals.md` § "Networking" table row for `elb`.
- a9s golden doc — read-only invariant — `docs/architecture.md` § "What is a9s?".
- AWS Go SDK v2 — `LoadBalancer.State.Code` / `State.Reason` field names and the state-machine description (`provisioning` → `active` → `active_impaired` → `failed`) — `AWS SDK Go v2 — elasticloadbalancingv2/types.LoadBalancer § State` and `elasticloadbalancingv2/types.LoadBalancerState § Code, Reason`.
- AWS Go SDK v2 — `LoadBalancer.VpcId`, `SecurityGroups[]`, `AvailabilityZones[].SubnetId` field names for related-panel pivots — `AWS SDK Go v2 — elasticloadbalancingv2/types.LoadBalancer § VpcId, SecurityGroups, AvailabilityZones`.
- a9s-devops consultation — `alarm` discovery via CloudWatch dimension `LoadBalancer` with `app/<name>/<id>` suffix — `a9s-devops (2026-04-20): possible=yes, worth=yes. Standard CloudWatch dimension schema for AWS/ApplicationELB / AWS/NetworkELB.`
- a9s-devops consultation — `cf` discovery via `Distribution.Origins.Items[].DomainName == LB.DNSName` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Matches the reverse pivot from the cf contract row.`
- a9s-devops consultation — `cfn` discovery via `aws:cloudformation:stack-name` tag — `a9s-devops (2026-04-20): possible=yes, worth=yes. CFN stamps this tag on every created resource.`
- a9s-devops consultation — `eni` discovery via Description prefix `ELB app/...` / `ELB net/...` / `ELB <name>` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Canonical SRE pivot for ELB-owned ENIs.`
- a9s-devops consultation — `r53` discovery via `AliasTarget.DNSName == LB.DNSName` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Standard Route 53 alias join.`
- a9s-devops consultation — `waf` discovery via `wafv2:ListResourcesForWebACL(ResourceType=APPLICATION_LOAD_BALANCER)` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Documented reverse pivot; matches waf contract row listing elb.`
- a9s-devops consultation — `s3` (access-log bucket) discovery deferred — `a9s-devops (2026-04-20): possible=yes via DescribeLoadBalancerAttributes, worth=no at list time. Would require N+1 fan-out attention-signals.md explicitly defers to Wave 3.`
- a9s-devops consultation — Classic (ELBv1) default Healthy bucket when no State field — implicit from attention-signals.md note "Classic (ELBv1) has no State field"; no state signal available, so the row defaults to Healthy and target-health signalling moves to `tg`. No separate devops dispatch.

<!-- BEGIN GENERATED: header -->
elb — NETWORKING. Lifecycle key: `state`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Approximate? |
| --- | --- | --- |
| tg | Target Groups | yes |
| alarm | CW Alarms | yes |
| sg | Security Groups | no |
| vpc | VPC | no |
| cfn | CloudFormation | no |
| r53 | Route 53 Records | no |
| acm | ACM Certificates | no |
| cf | CloudFront | no |
| eni | Network Interfaces | yes |
| s3 | S3 Buckets | no |
| subnet | Subnets | no |
| waf | WAF Web ACLs | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
