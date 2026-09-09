---
shortName: elb
name: Load Balancers
awsApiRef: https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_LoadBalancer.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
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

Expected targets from `docs/related-resources.md` § Per-type contract: `acm`, `alarm`, `cf`, `cfn`, `ct-events`, `eni`, `s3`, `sg`, `subnet`, `tg`, `vpc`, `waf`.

### `acm`

- **Why related**: HTTPS listener certificate — the ACM cert that terminates TLS on this LB's HTTPS/TLS listeners.
- **How discovered**: call `elbv2:DescribeListeners(LoadBalancerArn=<this>)` and collect `Certificates[].CertificateArn` from each listener; cross-reference against the already-loaded `acm` list by ARN. `docs/related-resources.md` § `elb` does not say whether discovery happens at detail-open time or earlier; the contract row does not fix the moment.
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

### `s3`

- **Why related**: Access-log S3 destination — lets the operator jump to the bucket receiving access logs when debugging.
- **How discovered**: `TBD — a9s-devops: not available in AWS surface without a per-LB Describe call.` The access-log bucket lives in `DescribeLoadBalancerAttributes`, not on the list response, and that read is deferred — `docs/attention-signals.md § Not yet implemented`. a9s-devops: possible=yes via `DescribeLoadBalancerAttributes` (N+1), worth=no at list time — the related panel would require a bounded fan-out this type has explicitly deferred. Related-panel pivot is still documented as a contract target; discovery is deferred until this resource performs that read.
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
- **How discovered**: universal pivot — applies to every registered type; see docs/related-resources.md §Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeLoadBalancers](https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_DescribeLoadBalancers.html)

Transcribed from `docs/attention-signals.md § Signals § NETWORKING` row `elb`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: ELBv2 `State.Code == provisioning`.
  - **State bucket**: Warning.
  - **How obtained**: `DescribeLoadBalancers` response field `LoadBalancer.State.Code`.

- **Signal**: ELBv2 `State.Code == active_impaired`.
  - **State bucket**: Warning.
  - **How obtained**: `DescribeLoadBalancers` response field `LoadBalancer.State.Code`.

- **Signal**: ELBv2 `State.Code == failed`; surface `State.Reason` as the Broken detail.
  - **State bucket**: Broken.
  - **How obtained**: `DescribeLoadBalancers` response fields `LoadBalancer.State.Code` and `LoadBalancer.State.Reason`.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal. Each runs on the type's bounded second pass, after the rows are on screen. Target health is not among them: it lives on `tg`, and that Wave 2 belongs to the target-group spec.

- **Signal**: ALB `routing.http.desync_mitigation_mode == monitor`.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: ALB `routing.http.drop_invalid_header_fields.enabled != true`.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: ALB `HTTP` listener with no redirect to HTTPS, or NLB `TCP` listener on 443.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: `HTTPS`/`TLS` listener on a policy outside the `TLS13-`/`TLS-1-2-`/`FS-1-2-` families.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: `deletion_protection.enabled` or `access_logs.s3.enabled` is `false`.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `HTTPCode_ELB_5XX_Count` per LB.
- OUT OF SCOPE: `DescribeLoadBalancerAttributes` per LB (deletion-protection, access-logs).

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `elb`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

Lifecycle findings render their phrase only — the state IS the whole fact, and a Detail
sentence would restate it. Their S5 cell reads `—`.

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `State.Code == provisioning` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `provisioning` |
| `State.Code == active_impaired` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `active impaired` |
| `State.Code == failed` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `failed` |
| ALB `routing.http.desync_mitigation_mode == monitor` | 2 | Warning | `~` | S2, S3, S4, S5 | `HTTP desync mitigation off` |
| ALB `routing.http.drop_invalid_header_fields.enabled != true` | 2 | Warning | `~` | S2, S3, S4, S5 | `invalid HTTP headers not dropped` |
| ALB `HTTP` listener with no redirect to HTTPS, or NLB `TCP` listener on 443 | 2 | Warning | `~` | S2, S3, S4, S5 | `<port(s) LIST> in the clear` |
| `HTTPS`/`TLS` listener on a policy outside the `TLS13-`/`TLS-1-2-`/`FS-1-2-` families | 2 | Warning | `~` | S2, S3, S4, S5 | `weak TLS policy on <port(s) LIST>` |
| `deletion_protection.enabled` or `access_logs.s3.enabled` is `false` | 2 | Warning | `~` | S2, S3, S4, S5 | `deletion protection or access logs disabled` |

Healthy ELBv2 rows (`State.Code == active`) and Classic (ELBv1) rows are omitted from this table per the §4 rule: Healthy renders green with a blank Status column.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — yellow for `provisioning` and `active impaired` and red for `failed`, so the operator reads the state inline. The four posture signals read the same way: `HTTP desync mitigation off`, `invalid HTTP headers not dropped`, `<port(s) LIST> in the clear` and `weak TLS policy on <port(s) LIST>` each name the setting and every port it applies to — the operator opens detail only for the remedy sentence. The AWS-provided `State.Reason` is not part of the phrase; it lives in the detail view, where an empty one costs nothing.

## 5. Out of Scope

- CloudWatch `HTTPCode_ELB_5XX_Count` (§3.3). The `DescribeLoadBalancerAttributes` and `DescribeListeners` signals listed there now ship as Wave 2 and appear in §4.
- Target-health signals (healthy/unhealthy target counts) — those live on `tg`, `docs/attention-signals.md § Signals § NETWORKING` row `tg`.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- `s3` related-panel discovery at list time — a9s-devops: not worth it, requires an N+1 `DescribeLoadBalancerAttributes` fan-out that is still deferred, `docs/attention-signals.md § Not yet implemented`; revisit if/when this resource performs that read.

## 6. Citations

- a9s golden doc — related panel contract (12 targets: `acm`, `alarm`, `cf`, `cfn`, `ct-events`, `eni`, `s3`, `sg`, `subnet`, `tg`, `vpc`, `waf`) — `docs/related-resources.md` § Per-type contract, row `elb`, and `docs/related-resources.md` § `elb`.
- a9s golden doc — universal pivot `ct-events` — `docs/related-resources.md` § Policy (universal pivots clause).
- a9s golden doc — the `elb` signals — `docs/attention-signals.md § Signals § NETWORKING` row `elb`; the deferred CloudWatch metric — `docs/attention-signals.md § Not yet implemented`.
- a9s golden doc — read-only invariant — `docs/architecture.md` § "What is a9s?".
- AWS Go SDK v2 — `LoadBalancer.State.Code` / `State.Reason` field names and the state-machine description (`provisioning` → `active` → `active_impaired` → `failed`) — `AWS SDK Go v2 — elasticloadbalancingv2/types.LoadBalancer § State` and `elasticloadbalancingv2/types.LoadBalancerState § Code, Reason`.
- AWS Go SDK v2 — `LoadBalancer.VpcId`, `SecurityGroups[]`, `AvailabilityZones[].SubnetId` field names for related-panel pivots — `AWS SDK Go v2 — elasticloadbalancingv2/types.LoadBalancer § VpcId, SecurityGroups, AvailabilityZones`.
- a9s-devops consultation — `alarm` discovery via CloudWatch dimension `LoadBalancer` with `app/<name>/<id>` suffix — `a9s-devops (2026-04-20): possible=yes, worth=yes. Standard CloudWatch dimension schema for AWS/ApplicationELB / AWS/NetworkELB.`
- a9s-devops consultation — `cf` discovery via `Distribution.Origins.Items[].DomainName == LB.DNSName` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Matches the reverse pivot from the cf contract row.`
- a9s-devops consultation — `cfn` discovery via `aws:cloudformation:stack-name` tag — `a9s-devops (2026-04-20): possible=yes, worth=yes. CFN stamps this tag on every created resource.`
- a9s-devops consultation — `eni` discovery via Description prefix `ELB app/...` / `ELB net/...` / `ELB <name>` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Canonical SRE pivot for ELB-owned ENIs.`
- `r53` budget exclusion — `docs/related-resources.md` § Explicitly excluded.
- a9s-devops consultation — `waf` discovery via `wafv2:ListResourcesForWebACL(ResourceType=APPLICATION_LOAD_BALANCER)` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Documented reverse pivot; matches waf contract row listing elb.`
- a9s-devops consultation — `s3` (access-log bucket) discovery deferred — `a9s-devops (2026-04-20): possible=yes via DescribeLoadBalancerAttributes, worth=no at list time. Would require an N+1 fan-out.`
- a9s-devops consultation — Classic (ELBv1) default Healthy bucket when no State field — implicit from `docs/attention-signals.md § Signals § NETWORKING` row `elb`; no state signal available, so the row defaults to Healthy and target-health signalling moves to `tg`. No separate devops dispatch.

<!-- BEGIN GENERATED: header -->
elb — NETWORKING. Status key: `state` — the key the status cell reads, and the column naming it is the status column.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| elb.state.provisioning | provisioning | warn | wave1 | The load balancer is still being built and is not yet accepting traffic. This normally clears in a few minutes; if it does not, its subnets are usually out of free IP addresses. |
| elb.state.active\_impaired | active impaired | warn | wave1 | The load balancer is serving traffic but could not set up or scale in at least one availability zone, so capacity there is degraded. Check that every attached subnet has spare IP addresses. |
| elb.state.failed | failed | broken | wave1 | The load balancer could not be created and will not recover on its own. It has to be deleted and recreated; nothing routes through it in the meantime. |
| elb.misconfigured | deletion protection or access logs disabled | warn | wave2 | This load balancer can be deleted with a single call, or is not writing access logs, so an outage is one call away, or leaves no record of the requests that hit it. Turn deletion protection on and point access logs at a bucket. |
| elb.desync-mitigation-off | HTTP desync mitigation off | warn | wave2 | The load balancer forwards requests it knows are ambiguous instead of rejecting them, so a crafted request can be interpreted one way by the balancer and another by the target. Set the desync mitigation mode to defensive or strictest. |
| elb.invalid-headers-kept | invalid HTTP headers not dropped | warn | wave2 | Headers that are not valid HTTP are passed through to the targets instead of being dropped, which is how request smuggling reaches an application. Turn on dropping of invalid header fields. |
| elb.plain-http-listener | <port(s) LIST> in the clear | warn | wave2 | A listener on this load balancer carries traffic in the clear, so credentials and session cookies cross the network readable by anyone on the path; the ports are listed below. Terminate TLS on the listener, or redirect it to an HTTPS listener. |
| elb.weak-tls-policy | weak TLS policy on <port(s) LIST> | warn | wave2 | A listener's security policy still negotiates older protocol versions or ciphers without forward secrecy, so a client can be steered onto a breakable connection; the ports are listed below. Move the listener to one of the modern security policies that require version 1.2 or later. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| tg | Target Groups | yes |
| alarm | CW Alarms | yes |
| sg | Security Groups | no |
| vpc | VPC | no |
| cfn | CloudFormation | no |
| acm | ACM Certificates | no |
| cf | CloudFront | yes |
| eni | Network Interfaces | yes |
| s3 | S3 Buckets | no |
| subnet | Subnets | no |
| waf | WAF Web ACLs | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
