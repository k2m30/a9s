---
shortName: eb
name: Elastic Beanstalk
awsApiRef: https://docs.aws.amazon.com/elasticbeanstalk/latest/api/API_EnvironmentDescription.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# eb — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `eb`
- **Display name**: Elastic Beanstalk
- **AWS API reference**: <https://docs.aws.amazon.com/elasticbeanstalk/latest/api/API_EnvironmentDescription.html>
- **List API**: `DescribeEnvironments`
- **Describe API (if any)**: `DescribeEnvironmentHealth` (Wave 2, per-environment); `DescribeEnvironmentResources` (related-target discovery for `elb` / `tg`); `DescribeConfigurationSettings` (related-target discovery for `role` / `sg`); `DescribeApplicationVersions` (related-target discovery for `s3`).

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `alarm`, `asg`, `cfn`, `ec2`, `elb`, `logs`, `role`, `s3`, `sg`, `tg`, `ct-events`.

### `alarm`

- **Why related**: Health alarms the operator relies on to be notified when this environment degrades.
- **How discovered**: Cross-reference the already-loaded `alarm` list — filter `AlarmName` / `Dimensions` referencing the environment. The golden doc does not pin a specific field citation for this pivot, so the exact filter is an operator heuristic — a9s-devops: possible=yes, worth=yes. Typical EB alarms are auto-created with `Dimensions=[{Name:EnvironmentName,Value:<env>}]`, so match on that.
- **Count shown**: unknown.

### `asg`

- **Why related**: The Auto Scaling Group backing this environment's EC2 fleet — the place to look when the fleet is not scaling, failing launches, or stuck at desired capacity.
- **How discovered**: Call `DescribeEnvironmentResources` and read `EnvironmentResources.AutoScalingGroups[].Name`; alternatively, cross-reference the already-loaded `asg` list by the `elasticbeanstalk:environment-name` tag.
- **Count shown**: unknown.

### `cfn`

- **Why related**: Beanstalk provisions a CloudFormation stack per environment; the stack is where low-level provisioning errors surface.
- **How discovered**: Cross-reference the already-loaded `cfn` list by stack-name prefix `awseb-{EnvironmentId}-stack`.
- **Count shown**: unknown.

### `ec2`

- **Why related**: The individual EC2 instances running the application — the place to look for a specific node failing health checks or stuck in an impaired state.
- **How discovered**: Call `DescribeEnvironmentResources` and read `EnvironmentResources.Instances[].Id`; alternatively, cross-reference the already-loaded `ec2` list by the `elasticbeanstalk:environment-name` tag.
- **Count shown**: unknown.

### `elb`

- **Why related**: The load balancer fronting the environment; target-group health, 5xx rates, and TLS config live here.
- **How discovered**: Call `DescribeEnvironmentResources` and read `EnvironmentResources.LoadBalancers[].Name`.
- **Count shown**: unknown.

### `logs`

- **Why related**: CloudWatch Logs groups collect the application and environment logs; first stop for tailing what the platform saw.
- **How discovered**: Cross-reference the already-loaded `logs` list by log-group name prefix `/aws/elasticbeanstalk/{EnvironmentName}/`.
- **Count shown**: unknown.

### `role`

- **Why related**: The environment's EC2 instance profile and service role — the IAM identities that calls out to AWS APIs on the environment's behalf. Broken permissions here show up as environment health drops.
- **How discovered**: Call `DescribeConfigurationSettings` and read `OptionSettings[]` where `Namespace=aws:autoscaling:launchconfiguration` `OptionName=IamInstanceProfile` (→ `GetInstanceProfile` for attached roles) and `Namespace=aws:elasticbeanstalk:environment` `OptionName=ServiceRole`.
- **Count shown**: unknown.

### `s3`

- **Why related**: S3 buckets that hold the environment's application-version bundles; lifecycle, encryption, and public-access posture of the deploy artifacts.
- **How discovered**: Call `DescribeApplicationVersions` for the environment's `ApplicationName` and read `ApplicationVersions[].SourceBundle.S3Bucket`.
- **Count shown**: unknown.

### `sg`

- **Why related**: Security groups attached to the EC2 fleet and the load balancer; governs exposure, ingress, and egress.
- **How discovered**: Call `DescribeConfigurationSettings` and read `OptionSettings[]` where `Namespace=aws:autoscaling:launchconfiguration` `OptionName=SecurityGroups` and `Namespace=aws:elbv2:loadbalancer` `OptionName=SecurityGroups`.
- **Count shown**: unknown.

### `tg`

- **Why related**: Target groups the environment's ALB listener forwards to; target-health per instance lives here.
- **How discovered**: Call `DescribeEnvironmentResources` to get `LoadBalancers[].Name`, then `elbv2:DescribeListeners` and read `DefaultActions[].ForwardConfig.TargetGroups[].TargetGroupArn` (or `DefaultActions[].TargetGroupArn`).
- **Count shown**: unknown.

### `ct-events`

- **Why related**: Audit trail for environment config changes — who triggered the last `UpdateEnvironment`, who terminated, who rebuilt.
- **How discovered**: Universal pivot — applies to every registered type; see `docs/related-resources.md` §Policy.
- **Count shown**: unknown.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeEnvironments](https://docs.aws.amazon.com/elasticbeanstalk/latest/api/API_DescribeEnvironments.html)

Transcribed from `docs/attention-signals.md § Signals § MESSAGING` row `eb`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. AWS field names verbatim.

- **Signal**: `Health == Yellow`.
  - **State bucket**: Warning.
  - **How obtained**: `EnvironmentDescription.Health` on the list response. AWS documents this as "something is wrong — two consecutive health-check failures".

- **Signal**: `Health == Grey`.
  - **State bucket**: Warning.
  - **How obtained**: `EnvironmentDescription.Health` on the list response. AWS documents this as "new environment not fully launched, or health checks suspended during an `UpdateEnvironment`/`RestartEnvironment` request".

- **Signal**: `Health == Red`.
  - **State bucket**: Broken.
  - **How obtained**: `EnvironmentDescription.Health` on the list response. AWS documents this as "environment not responsive — three or more consecutive health-check failures".

- **Signal**: `Status == Terminated`.
  - **State bucket**: Dim.
  - **How obtained**: `EnvironmentDescription.Status` on the list response.

- **Signal**: `Status` is `Launching` or `Updating` with no health signal.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `Status == Terminating` with no health signal.
  - **State bucket**: Dim.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: `Causes[]` non-empty on enhanced health.
  - **State bucket**: Warning (cause detail only — does not upgrade an already-Red environment to a different bucket; adds operator-readable reason text to whatever bucket Wave 1 produced).
  - **API call**: `DescribeEnvironmentHealth` — one per environment.
  - **Cost shape**: per-resource.

- **Signal**: managed platform updates not enabled.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: health reporting not `enhanced`.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: log streaming to CloudWatch not enabled.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `DescribeConfigurationSettings` platform-EOL check.
- OUT OF SCOPE: `DescribeEvents` severity filter.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `eb`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `Health == Yellow` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `health: yellow` |
| `Health == Grey` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `health: grey` |
| `Health == Red` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `health: red` |
| `Status == Terminated` | 1 | Dim | n/a | S2, S4 | `terminated` |
| `Status` is `Launching` or `Updating` with no health signal | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `launching` |
| `Status == Terminating` with no health signal | 1 | Dim | n/a | S2, S4 | `terminating` |
| `Causes[]` non-empty | 2 | Warning | `~` | S2, S3, S4, S5 | `environment reports health causes` |
| managed platform updates not enabled | 2 | Warning | `~` | S2, S3, S4, S5 | `managed platform updates off` |
| health reporting not `enhanced` | 2 | Warning | `~` | S2, S3, S4, S5 | `enhanced health reporting off` |
| log streaming to CloudWatch not enabled | 2 | Warning | `~` | S2, S3, S4, S5 | `log streaming to CloudWatch off` |

Notes on the `Causes[]` row:

- `Causes[]` is only populated when enhanced health is enabled; with basic health the field is absent. When absent, the row is omitted entirely from S5.
- On a Yellow/Grey/Red row, S4 already carries a cause from Wave 1. S4 should deduplicate: if the first `Cause` string adds information beyond the Wave 1 text, replace the Wave 1 text; otherwise leave the Wave 1 text and put `Causes[]` in S5 only.
- `Causes[]` is never raised on a Green environment in practice (enhanced-health reports Causes only when degradation is detected). If it ever is, treat as `~` (informational): it colours the row like any other Warning finding.

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`. Note: "truncated to 40 chars" above is a spec-writing instruction to the implementer, not a string the operator ever sees.
- A bare state keyword (`Red`, `Grey`, `Terminated`) in the List text column is not acceptable. Pair it with the cause.
- Keep the List text short enough to fit: ≤ 40 chars. The Detail cell quotes the finding's Detail constant verbatim, however long it is.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every non-healthy row carries a plain-English cause in the Status column (`health: red`, `health: yellow`, `health: grey`, `launching`, `terminated`); the operator knows whether to page, wait, or ignore without opening detail. The specific sub-reasons behind a health colour arrive as `environment reports health causes`, whose full text lives in the detail view as S5, which is the correct place for it.

## 5. Out of Scope

- All §3.3 Wave 3 signals (platform-EOL check, `DescribeEvents` severity filter).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- Contract row for `eb` — `docs/related-resources.md` § Per-type contract (row `eb`).
- `alarm` pivot rationale "Health alarms" — `docs/related-resources.md` § `eb` (bullet `alarm`).
- `asg` pivot discovery via tag and `DescribeEnvironmentResources` — `docs/related-resources.md` § `eb` (bullet `asg`); AWS SDK Go v2 — `elasticbeanstalk/types.EnvironmentResourceDescription § AutoScalingGroups`.
- `cfn` pivot discovery via `awseb-{EnvironmentId}-stack` — `docs/related-resources.md` § `eb` (bullet `cfn`).
- `ec2` pivot discovery via tag and `DescribeEnvironmentResources` — `docs/related-resources.md` § `eb` (bullet `ec2`); AWS SDK Go v2 — `elasticbeanstalk/types.EnvironmentResourceDescription § Instances`.
- `elb` pivot discovery via `DescribeEnvironmentResources.LoadBalancers[].Name` — `docs/related-resources.md` § `eb` (bullet `elb`); AWS SDK Go v2 — `elasticbeanstalk/types.EnvironmentResourceDescription § LoadBalancers`.
- `logs` pivot discovery via log-group prefix — `docs/related-resources.md` § `eb` (bullet `logs`).
- `role` pivot discovery via `DescribeConfigurationSettings` option settings — `docs/related-resources.md` § `eb` (bullet `role`).
- `s3` pivot discovery via `DescribeApplicationVersions.SourceBundle.S3Bucket` — `docs/related-resources.md` § `eb` (bullet `s3`).
- `sg` pivot discovery via `DescribeConfigurationSettings` option settings — `docs/related-resources.md` § `eb` (bullet `sg`).
- `tg` pivot discovery via LB → listener → target-group forwarding — `docs/related-resources.md` § `eb` (bullet `tg`).
- `ct-events` universal pivot — `docs/related-resources.md` § Policy (item 4).
- `alarm` discovery heuristic (match on `Dimensions[EnvironmentName]`) — a9s-devops (2026-04-20): possible=yes, worth=yes. EB-created alarms use `Dimensions=[{Name:EnvironmentName,Value:<env>}]`; non-EB-created alarms referencing the environment use the same dimension if they're useful to the operator. Persona call recorded because no field citation exists in the golden docs for this pivot; adopted because it matches how EB itself tags its own alarms.
- "Count shown: unknown" for every related target — `docs/related-resources.md` § Per-target reasoning does not pin count-display semantics per pivot; the skill records silence as `unknown` rather than inventing a value.
- Wave 1 signals (`Health` Green/Yellow/Grey/Red → Healthy/Warning/Warning/Broken; `Status==Terminated` → Dim) — `docs/attention-signals.md § Signals § MESSAGING` row `eb`.
- SDK field names `Health`, `Status`, `HealthStatus`, and the AWS semantics of Green/Yellow/Grey/Red — `AWS SDK Go v2 — elasticbeanstalk/types.EnvironmentDescription § Health` (doc comment) and `§ Status`.
- `Health` enum values — `AWS SDK Go v2 — elasticbeanstalk/types.EnvironmentHealth`.
- `Status` enum values — `AWS SDK Go v2 — elasticbeanstalk/types.EnvironmentStatus`.
- Wave 2 `Causes[]` non-empty → Warning detail — `docs/attention-signals.md § Signals § MESSAGING` row `eb`; AWS SDK Go v2 — `elasticbeanstalk.DescribeEnvironmentHealthOutput § Causes`.
- `Causes[]` populated only under enhanced health — a9s-devops (2026-04-20): possible=yes, worth=yes. Basic health does not populate `Causes`; implementation must tolerate the empty slice without emitting an S5 line. Persona call because the golden docs are silent on the basic-vs-enhanced distinction but the API contract is explicit.
- Wave 3 items explicitly listed as out of scope — `docs/attention-signals.md § Not yet implemented`.
- Read-only invariant — `docs/architecture.md` § "What is a9s?".

<!-- BEGIN GENERATED: header -->
eb — MESSAGING. Status key: `status` — the key the status cell reads, and the column naming it is the status column.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| eb.health.red | health: red | broken | wave1 | — |
| eb.health.yellow | health: yellow | warn | wave1 | — |
| eb.health.grey | health: grey | warn | wave1 | — |
| eb.status.terminated | terminated | dim | wave1 | — |
| eb.status.launching | launching | warn | wave1 | — |
| eb.status.terminating | terminating | dim | wave1 | — |
| eb.environment-causes | environment reports health causes | warn | wave2 | — |
| eb.managed-updates-off | managed platform updates off | warn | wave2 | The environment never takes platform patches on its own, so it stays on whatever version it was launched with until someone updates it by hand. Turn managed platform updates on and pick a weekly maintenance window. |
| eb.enhanced-health-off | enhanced health reporting off | warn | wave2 | Health is reported from basic checks only, so the environment cannot tell you which instance or which request is failing, or why. Switch health reporting to enhanced. |
| eb.cloudwatch-logs-off | log streaming to CloudWatch off | warn | wave2 | Instance logs stay on the instances and disappear when those instances are replaced, so there is nothing left to read after a failure. Turn on log streaming to CloudWatch Logs. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| cfn | CloudFormation Stack | yes |
| logs | Log Groups | yes |
| asg | Auto Scaling Groups | yes |
| ec2 | EC2 Instances | yes |
| alarm | CloudWatch Alarms | yes |
| elb | Load Balancers | no |
| tg | Target Groups | no |
| sg | Security Groups | no |
| role | IAM Role | no |
| s3 | S3 Buckets | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
