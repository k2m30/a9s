---
shortName: eip
name: Elastic IPs
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Address.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# eip — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `eip`
- **Display name**: Elastic IPs
- **AWS API reference**: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Address.html>
- **List API**: `DescribeAddresses`
- **Describe API (if any)**: not used — every `eip` signal reads the list response, `docs/attention-signals.md § Signals § NETWORKING` row `eip`. `DescribeAddressesAttribute` (reverse-DNS) is deferred — `docs/attention-signals.md § Not yet implemented`.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `alarm`, `asg`, `cfn`, `ct-events`, `ec2`, `ecs`, `ecs-svc`, `ecs-task`, `eni`, `nat`.

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

### `nat`

- **Why related**: NAT gateway consuming this EIP as its public address — critical when the EIP is the egress IP for a private subnet.
- **How discovered**: cross-reference the already-loaded `nat` list where `NatGateway.NatGatewayAddresses[].AllocationId == Address.AllocationId`.
- **Count shown**: yes.

### `ct-events`

- **Why related**: CloudTrail audit trail for EIP allocation, association, disassociation, release, and tagging — who changed what, when.
- **How discovered**: universal pivot — applies to every registered type; see `docs/related-resources.md` §Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeAddresses](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeAddresses.html)

Transcribed from `docs/attention-signals.md § Signals § NETWORKING` row `eip`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `AssociationId` absent AND `InstanceId` absent AND `NetworkInterfaceId` absent → Warning (unattached, billed hourly).
  - **State bucket**: Warning.
  - **How obtained**: three fields on the list-response `Address` shape: `AssociationId`, `InstanceId`, `NetworkInterfaceId`. No extra call.

- **Signal**: Cross-ref `ec2` — attached to instance with `State.Name==stopped` → Warning (zombie billing). — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06)
  - **State bucket**: Warning.
  - **How obtained**: read `Address.InstanceId`; look up the already-loaded `ec2` list by `Instance.InstanceId` and check `Instance.State.Name`. Zero extra API calls (sibling-list cross-reference).

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `DescribeAddressesAttribute` per EIP (reverse-DNS).

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `eip`: Wave 1 issue-colored rows only — this type registers no Wave 2 enricher, so nothing else bumps the count.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| unattached EIP (no association/instance/ENI) | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `unassociated` |
| attached to stopped EC2 instance — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | 1 | Warning | n/a | S2, S4 | `attached to stopped instance` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`DORMANT`, `stopped`, `available`, `failed`) in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column. Tests will assert the cause is present.
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), you may omit the row from this table entirely; §3 still describes it.
- Keep the List text short enough to fit: ≤ 40 chars. The Detail cell quotes the finding's Detail constant verbatim, however long it is.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — the one Wave 1 signal renders a yellow row reading `unassociated`, which is the whole triage: the address is billed and nothing is using it. All problem rows are self-explanatory in the list — operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): `DescribeAddressesAttribute` per EIP (reverse-DNS lookup).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- `alarm` via dimensions other than `AllocationId` / `NetworkInterfaceId` — persona (a9s-devops): not worth it, no EIP-scoped CloudWatch namespace exists; alarms surface via the ENI or NAT pivots already listed.
- `logs` cross-reference beyond best-effort VPC-flow-log-name matching — persona (a9s-devops): not worth a live API call; flow logs are configured out-of-band and may land in S3 or Kinesis rather than CloudWatch Logs.
- Direct ECS discovery without an ENI association — persona (a9s-devops): not worth it, an EIP with no `NetworkInterfaceId` cannot be tied to a task.

## 6. Citations

- a9s golden doc — per-type contract (`alarm`, `asg`, `cfn`, `ct-events`, `ec2`, `ecs`, `ecs-svc`, `ecs-task`, `eni`, `nat`) — `docs/related-resources.md` § Per-type contract / `eip`.
- a9s golden doc — `nat` pivot direction (`NatGatewayAddresses[].AllocationId`) — `docs/related-resources.md` § Per-target reasoning / `nat` / `eip`.
- a9s golden doc — `ct-events` universal-pivot policy — `docs/related-resources.md` § Policy #4.
- a9s golden doc — Wave 1 signals (unattached EIP; zombie-billing cross-ref to stopped `ec2`) — `docs/attention-signals.md § Signals § NETWORKING` row `eip`.
- a9s golden doc — Wave 2 `None`; Wave 3 `DescribeAddressesAttribute` (reverse-DNS) — `docs/attention-signals.md § Not yet implemented`.
- a9s golden doc — read-only invariant used in §5 — `docs/architecture.md` § What is a9s?.
- AWS Go SDK v2 — `Address.AllocationId`, `Address.AssociationId`, `Address.InstanceId`, `Address.NetworkInterfaceId`, `Address.Tags` — `AWS SDK Go v2 — service/ec2/types.Address § AllocationId, AssociationId, InstanceId, NetworkInterfaceId, Tags`.
- AWS API Reference — `Address` response shape — `AWS API Reference: API_Address` (<https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Address.html>).
- a9s-devops persona — `alarm` discovered via CloudWatch `Dimensions[]` on `AllocationId`/`NetworkInterfaceId` — persona (2026-04-20): possible=partial, worth=yes-narrow. CloudWatch has no EIP-scoped metric namespace; alarms in practice dimension on the ENI or NAT that carries the traffic, so cache-scan of loaded alarms is the correct pivot.
- a9s-devops persona — `asg` via two-hop `ec2` lookup (`Address.InstanceId` → `Instance.Tags[aws:autoscaling:groupName]`) — persona (2026-04-20): possible=yes, worth=yes. Operator needs to know whether the underlying instance is replaceable by an ASG.
- a9s-devops persona — `cfn` via `Address.Tags[aws:cloudformation:stack-name]` — persona (2026-04-20): possible=yes, worth=yes. CFN writes reserved tags on stack-managed resources; cheap cache pivot.
- a9s-devops persona — `ecs` / `ecs-svc` / `ecs-task` via `NetworkInterfaceId` match on `attachments[].details[]` — persona (2026-04-20): possible=yes, worth=yes-narrow. Pattern is rare (ALB/Fargate auto-IP is more common) but valid for legacy task-per-EIP setups; skip when no ENI association.
- `logs` budget exclusion — `docs/related-resources.md` § Explicitly excluded.
- a9s-devops persona — `alarm` non-ENI/NAT dimensions, `logs` beyond best-effort, ECS without ENI recorded in §5 — persona (2026-04-20): possible=no / partial, worth=no. AWS surface does not expose a direct cross-reference and the operator benefit is below the Wave 1 cost budget.

<!-- BEGIN GENERATED: header -->
eip — NETWORKING. Lifecycle key: none (the list API returns no lifecycle field).
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| eip.unassociated | unassociated | warn | wave1 | — |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| ec2 | EC2 Instances | no |
| eni | Network Interfaces | no |
| nat | NAT Gateways | yes |
| alarm | CloudWatch Alarms | yes |
| asg | Auto Scaling Groups | no |
| cfn | CloudFormation | no |
| ecs | ECS Clusters | yes |
| ecs-svc | ECS Services | yes |
| ecs-task | ECS Tasks | yes |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
