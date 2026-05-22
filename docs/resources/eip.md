---
shortName: eip
name: Elastic IPs
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Address.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# eip — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `eip`
- **Display name**: Elastic IPs
- **AWS API reference**: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Address.html>
- **List API**: `DescribeAddresses`
- **Describe API (if any)**: not used — Wave 2 is `None` in `docs/attention-signals.md`. `DescribeAddressesAttribute` (reverse-DNS) is listed but explicitly Wave 3 and out of scope.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `asg`, `cfn`, `ec2`, `ecs`, `ecs-svc`, `ecs-task`, `eni`, `logs`, `nat`, `ct-events`.

### `alarm`

- **Why related**: CloudWatch alarms that fire on traffic through this EIP — operator needs to see alarm state next to the IP when investigating connectivity.
- **How discovered**: cross-reference the already-loaded `alarm` list by `MetricAlarm.Dimensions[]` containing `{Name: "AllocationId"|"NetworkInterfaceId", Value: <Address.AllocationId | Address.NetworkInterfaceId>}`. CloudWatch has no native EIP-scoped metric namespace, so in practice alarms attach to the backing ENI or to a NAT gateway consuming the EIP — persona (a9s-devops): alarms dimensioned on `NetworkInterfaceId` are the reliable pivot because EIP traffic shows up under the interface it is associated with.
- **Count shown**: yes.

### `asg`

- **Why related**: When the EIP is attached to an instance launched by an Auto Scaling Group, the ASG owns the replacement lifecycle of that IP's target — persona (a9s-devops): operators watching for IP churn need to see whether the attached instance will be rotated by an ASG.
- **How discovered**: read `Address.InstanceId`; follow to the already-loaded `ec2` list to read that instance's `Tags[]` for `aws:autoscaling:groupName`; cross-reference the already-loaded `asg` list by `AutoScalingGroupName`. Two-hop pivot via `ec2`.
- **Count shown**: yes.

### `cfn`

- **Why related**: The CFN stack that allocated this EIP — provenance and blast radius for IP-address changes.
- **How discovered**: read `Address.Tags[]` for key `aws:cloudformation:stack-name`; cross-reference the already-loaded `cfn` list by `Stack.StackName` — persona (a9s-devops): CFN writes reserved tags on every stack-managed resource, so tag lookup is the standard cheap pivot.
- **Count shown**: yes.

### `ec2`

- **Why related**: The EC2 instance this EIP routes traffic to — the workload the address exposes.
- **How discovered**: read `Address.InstanceId`; cross-reference the already-loaded `ec2` list by `Instance.InstanceId`.
- **Count shown**: yes.

### `ecs`

- **Why related**: ECS cluster running a task that terminates on this EIP. Relevant only in the narrow case where an ECS task on EC2 launch type has a user-assigned EIP via its ENI — persona (a9s-devops): common ECS deployments rely on ALB or auto-assigned public IPs, so this pivot is rare but still valid for legacy task-per-EIP patterns.
- **How discovered**: read `Address.NetworkInterfaceId`; cross-reference the already-loaded `ecs-task` list for `attachments[].details[]` entries whose `networkInterfaceId` matches; from matched task, follow `clusterArn` to the already-loaded `ecs` list by `Cluster.clusterArn`.
- **Count shown**: yes.

### `ecs-svc`

- **Why related**: ECS service owning the task that terminates on this EIP — service-level context around a task-EIP binding.
- **How discovered**: via the `ecs-task` chain (see `ecs` above); from the matched task read `group` (format `service:<svc>`) and cross-reference the already-loaded `ecs-svc` list by `Service.serviceName` — persona (a9s-devops): this is the same two-hop ENI→task→service chain ECS uses internally; skip when `group` doesn't begin with `service:`.
- **Count shown**: yes.

### `ecs-task`

- **Why related**: The ECS task whose ENI carries this EIP — direct workload attribution for the address.
- **How discovered**: read `Address.NetworkInterfaceId`; cross-reference the already-loaded `ecs-task` list for any task with `attachments[].details[]` entry where `name == "networkInterfaceId"` and `value == <NetworkInterfaceId>`.
- **Count shown**: yes.

### `eni`

- **Why related**: The network interface the EIP is currently associated with — routing target one level below the instance. Associations to secondary ENIs are the common case a user misses.
- **How discovered**: read `Address.NetworkInterfaceId`; cross-reference the already-loaded `eni` list by `NetworkInterface.NetworkInterfaceId`.
- **Count shown**: yes.

### `logs`

- **Why related**: CloudWatch Logs groups receiving VPC Flow Logs that include traffic for this IP — the only log surface that directly references an EIP by value — persona (a9s-devops): flow-log capture is enabled at the VPC/subnet/ENI level, so the operator pivot is to log groups destined from the attached ENI, not from the EIP itself. Value here is narrow; surface it only when an ENI association exists.
- **How discovered**: read `Address.NetworkInterfaceId` → follow to `eni` in the already-loaded `eni` list → read `NetworkInterface.VpcId` → filter the already-loaded `logs` list by log-group-name convention for VPC Flow Logs (`/aws/vpc/flowlogs/<vpc-id>` or operator-defined). No direct AWS cross-reference API — persona (a9s-devops): this pivot is best-effort; when flow logs target S3 or Kinesis instead of CloudWatch Logs, the count is 0.
- **Count shown**: yes.

### `nat`

- **Why related**: NAT gateway consuming this EIP as its public address — critical when the EIP is the egress IP for a private subnet.
- **How discovered**: cross-reference the already-loaded `nat` list where `NatGateway.NatGatewayAddresses[].AllocationId == Address.AllocationId`.
- **Count shown**: yes.

### `ct-events`

- **Why related**: CloudTrail audit trail for EIP allocation, association, disassociation, release, and tagging — who changed what, when.
- **How discovered**: universal pivot — applies to every registered type; see `docs/related-resources.md` §Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `AssociationId` absent AND `InstanceId` absent AND `NetworkInterfaceId` absent → Warning (unattached, billed hourly).
  - **State bucket**: Warning.
  - **How obtained**: three fields on the list-response `Address` shape: `AssociationId`, `InstanceId`, `NetworkInterfaceId`. No extra call.
- **Signal**: Cross-ref `ec2` — attached to instance with `State.Name==stopped` → Warning (zombie billing).
  - **State bucket**: Warning.
  - **How obtained**: read `Address.InstanceId`; look up the already-loaded `ec2` list by `Instance.InstanceId` and check `Instance.State.Name`. Zero extra API calls (sibling-list cross-reference).

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `DescribeAddressesAttribute` per EIP (reverse-DNS).

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing" — e.g. maintenance scheduled, certificate expiring soon. `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `unattached — billed hourly`). **Healthy rows render blank** — no `OK` / `available` / `in-use`. Empty means "nothing to see." |
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
| unattached EIP (no association/instance/ENI) | 1 | Warning | n/a | S2, S4 | `unattached — billed hourly` | — (Wave 1, no S5) |
| attached to stopped EC2 instance | 1 | Warning | n/a | S2, S4 | `attached to stopped instance` | — (Wave 1, no S5) |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`DORMANT`, `stopped`, `available`, `failed`) in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column. Tests will assert the cause is present.
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), you may omit the row from this table entirely; §3 still describes it.
- Keep both columns short enough to fit: List text ≤ 40 chars, Detail text ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — both Wave 1 signals render a yellow row with a self-explanatory cause in the Status column (`unattached — billed hourly`, `attached to stopped instance`), so the operator can triage an EIP without pressing detail. All problem rows are self-explanatory in the list — operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): `DescribeAddressesAttribute` per EIP (reverse-DNS lookup).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- `alarm` via dimensions other than `AllocationId` / `NetworkInterfaceId` — persona (a9s-devops): not worth it, no EIP-scoped CloudWatch namespace exists; alarms surface via the ENI or NAT pivots already listed.
- `logs` cross-reference beyond best-effort VPC-flow-log-name matching — persona (a9s-devops): not worth a live API call; flow logs are configured out-of-band and may land in S3 or Kinesis rather than CloudWatch Logs.
- Direct ECS discovery without an ENI association — persona (a9s-devops): not worth it, an EIP with no `NetworkInterfaceId` cannot be tied to a task.

## 6. Citations

- a9s golden doc — per-type contract (`alarm`, `asg`, `cfn`, `ct-events`, `ec2`, `ecs`, `ecs-svc`, `ecs-task`, `eni`, `logs`, `nat`) — `docs/related-resources.md` § Per-type contract / `eip`.
- a9s golden doc — `nat` pivot direction (`NatGatewayAddresses[].AllocationId`) — `docs/related-resources.md` § Per-target reasoning / `nat` / `eip`.
- a9s golden doc — `ct-events` universal-pivot policy — `docs/related-resources.md` § Policy #4.
- a9s golden doc — Wave 1 signals (unattached EIP; zombie-billing cross-ref to stopped `ec2`) — `docs/attention-signals.md` § Networking / `eip`.
- a9s golden doc — Wave 2 `None`; Wave 3 `DescribeAddressesAttribute` (reverse-DNS) — `docs/attention-signals.md` § Networking / `eip`.
- a9s golden doc — read-only invariant used in §5 — `docs/architecture.md` § What is a9s?.
- AWS Go SDK v2 — `Address.AllocationId`, `Address.AssociationId`, `Address.InstanceId`, `Address.NetworkInterfaceId`, `Address.Tags` — `AWS SDK Go v2 — service/ec2/types.Address § AllocationId, AssociationId, InstanceId, NetworkInterfaceId, Tags`.
- AWS API Reference — `Address` response shape — `AWS API Reference: API_Address` (<https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Address.html>).
- a9s-devops persona — `alarm` discovered via CloudWatch `Dimensions[]` on `AllocationId`/`NetworkInterfaceId` — persona (2026-04-20): possible=partial, worth=yes-narrow. CloudWatch has no EIP-scoped metric namespace; alarms in practice dimension on the ENI or NAT that carries the traffic, so cache-scan of loaded alarms is the correct pivot.
- a9s-devops persona — `asg` via two-hop `ec2` lookup (`Address.InstanceId` → `Instance.Tags[aws:autoscaling:groupName]`) — persona (2026-04-20): possible=yes, worth=yes. Operator needs to know whether the underlying instance is replaceable by an ASG.
- a9s-devops persona — `cfn` via `Address.Tags[aws:cloudformation:stack-name]` — persona (2026-04-20): possible=yes, worth=yes. CFN writes reserved tags on stack-managed resources; cheap cache pivot.
- a9s-devops persona — `ecs` / `ecs-svc` / `ecs-task` via `NetworkInterfaceId` match on `attachments[].details[]` — persona (2026-04-20): possible=yes, worth=yes-narrow. Pattern is rare (ALB/Fargate auto-IP is more common) but valid for legacy task-per-EIP setups; skip when no ENI association.
- a9s-devops persona — `logs` via VPC-flow-log group-name convention keyed off the attached ENI's VPC — persona (2026-04-20): possible=partial, worth=yes-narrow. No direct AWS reference API; returns 0 when flow logs target S3 or Kinesis.
- a9s-devops persona — `alarm` non-ENI/NAT dimensions, `logs` beyond best-effort, ECS without ENI recorded in §5 — persona (2026-04-20): possible=no / partial, worth=no. AWS surface does not expose a direct cross-reference and the operator benefit is below the Wave 1 cost budget.

<!-- BEGIN GENERATED: header -->
eip — NETWORKING. Lifecycle key: `state`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Approximate? |
| --- | --- | --- |
| ec2 | EC2 Instances | no |
| eni | Network Interfaces | no |
| nat | NAT Gateways | yes |
| alarm | CloudWatch Alarms | no |
| asg | Auto Scaling Groups | no |
| cfn | CloudFormation | no |
| ecs | ECS Clusters | no |
| ecs-svc | ECS Services | no |
| ecs-task | ECS Tasks | no |
| logs | Log Groups | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
