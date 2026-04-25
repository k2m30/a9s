---
shortName: eni
name: Network Interfaces
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NetworkInterface.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# eni — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `eni`
- **Display name**: Network Interfaces
- **AWS API reference**: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NetworkInterface.html>
- **List API**: `DescribeNetworkInterfaces`
- **Describe API (if any)**: not used — all Wave 1 signals live on the list response shape (`NetworkInterface`); no Wave 2 enricher is registered.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `ec2`, `eip`, `elb`, `lambda`, `nat`, `sg`, `subnet`, `vpc`, `vpce`, `ct-events`.

### `ec2`

- **Why related**: The instance this ENI is attached to — the primary hop when an ENI looks wrong.
- **How discovered**: read field `NetworkInterface.Attachment.InstanceId` on the resource; cross-reference the already-loaded `ec2` list by `Instance.InstanceId`. Absent when the ENI is not attached (`Status == available`) or is attached to a non-EC2 service (lambda, nat, vpce).
- **Count shown**: yes.

### `eip`

- **Why related**: Public IP or allocated Elastic IP bound to this ENI — tells the operator whether the interface is internet-reachable and whether the EIP is billed.
- **How discovered**: read field `NetworkInterface.Association.AllocationId` (or `Association.PublicIp`) on the resource; cross-reference the already-loaded `eip` list by `Address.AllocationId` / `Address.PublicIp`. Absent when `Association == nil`.
- **Count shown**: yes.

### `elb`

- **Why related**: When the ENI backs a load balancer AZ, the operator needs to pivot to the LB to see listener / target-group state.
- **How discovered**: read field `NetworkInterface.InterfaceType` — values `load_balancer`, `network_load_balancer`, `gateway_load_balancer` identify ELBv2-managed ENIs. Extract the LB name from `NetworkInterface.Description` (AWS writes `ELB app/<name>/<id>` or `ELB net/<name>/<id>`) and cross-reference the already-loaded `elb` list — a9s-devops: the LB name embedded in `Description` is AWS-stable and is the standard pivot when no formal cross-reference field exists on the ENI shape.
- **Count shown**: yes.

### `lambda`

- **Why related**: Lambda-in-VPC functions provision ENIs; when one is orphaned or stuck the operator wants to jump to the function.
- **How discovered**: read field `NetworkInterface.InterfaceType == lambda`. The function name is embedded in `NetworkInterface.Description` (AWS format `AWS Lambda VPC ENI-<functionName>-<uuid>`); cross-reference the already-loaded `lambda` list by `FunctionConfiguration.FunctionName` — a9s-devops: parsing the description is the only in-band way to reach the function from the ENI; the AWS SDK does not expose a direct owning-function field on the ENI shape.
- **Count shown**: yes.

### `nat`

- **Why related**: NAT gateway backing ENI — when the ENI is misbehaving the fault almost always lives on the NAT gateway (FailureCode, EIP).
- **How discovered**: read field `NetworkInterface.InterfaceType == natGateway`. The NAT gateway ID is embedded in `NetworkInterface.Description` (AWS format `Interface for NAT Gateway nat-<id>`); cross-reference the already-loaded `nat` list by `NatGateway.NatGatewayId` — a9s-devops: description-parse is the supported pivot here.
- **Count shown**: yes.

### `sg`

- **Why related**: Security groups attached to this ENI — the rules that actually govern this interface's traffic.
- **How discovered**: read field `NetworkInterface.Groups[].GroupId` on the resource; cross-reference the already-loaded `sg` list by `SecurityGroup.GroupId`.
- **Count shown**: yes.

### `subnet`

- **Why related**: Subnet the ENI lives in — needed to assess IP pressure, AZ, and routing for this interface.
- **How discovered**: read field `NetworkInterface.SubnetId` on the resource; cross-reference the already-loaded `subnet` list by `Subnet.SubnetId`.
- **Count shown**: yes.

### `vpc`

- **Why related**: Parent VPC — the broader network boundary this ENI belongs to.
- **How discovered**: read field `NetworkInterface.VpcId` on the resource; cross-reference the already-loaded `vpc` list by `Vpc.VpcId`.
- **Count shown**: yes.

### `vpce`

- **Why related**: Interface endpoints provision one ENI per AZ; operator pivots to the endpoint to see `State`, `LastError`, and policy.
- **How discovered**: read field `NetworkInterface.InterfaceType == vpc_endpoint`. The endpoint ID is embedded in `NetworkInterface.Description` (AWS format `VPC Endpoint Interface vpce-<id>`); cross-reference the already-loaded `vpce` list by `VpcEndpoint.VpcEndpointId` — a9s-devops: AWS records the owning endpoint in the description string; this is the standard cross-link.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Universal pivot — CloudTrail audit trail scoped to this ENI (`AttachNetworkInterface`, `DetachNetworkInterface`, `CreateNetworkInterface`, `DeleteNetworkInterface`, `ModifyNetworkInterfaceAttribute`). Answers "who changed this, and when?".
- **How discovered**: universal pivot — applies to every registered type; see related-resources.md §Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `Status` in `in-use` or `associated` → Healthy.
  - **State bucket**: Healthy.
  - **How obtained**: `NetworkInterface.Status` on the `DescribeNetworkInterfaces` list response.

- **Signal**: `Status` in `attaching` or `detaching` → Warning (transitional).
  - **State bucket**: Warning.
  - **How obtained**: `NetworkInterface.Status` on the list response.

- **Signal**: `Status == available` → Warning (orphan — ENI is unattached and billed while it sits idle).
  - **State bucket**: Warning.
  - **How obtained**: `NetworkInterface.Status` on the list response.

- **Signal**: Requester-managed ENI with `Description` referencing a deleted service → Warning (zombie — AWS forgot to reap after the owning service went away).
  - **State bucket**: Warning.
  - **How obtained**: `NetworkInterface.RequesterManaged == true` AND `NetworkInterface.Description` is set AND the referenced owner (parsed from the description string, e.g. NAT gateway id, function name, endpoint id) is not present in the already-loaded sibling list (`nat`, `lambda`, `vpce`, `elb`).

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

The `Wave 3` cell in `docs/attention-signals.md` for `eni` is `None`. No out-of-scope Wave 3 signals are recorded.

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
| `Status == attaching` | 1 | Warning | n/a | S2 + S4 | `attaching` | n/a |
| `Status == detaching` | 1 | Warning | n/a | S2 + S4 | `detaching` | n/a |
| `Status == available` (orphan) | 1 | Warning | n/a | S2 + S4 | `unattached — billed while idle` | n/a |
| Requester-managed, owner gone | 1 | Warning | n/a | S2 + S4 | `zombie: owner <kind> <id> gone` | n/a |

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? All problem rows are self-explanatory in the list — a yellow row with `unattached — billed while idle` or `zombie: owner nat <id> gone` tells the operator exactly which ENI to reclaim and why, no detail press needed; transitional `attaching` / `detaching` rows carry their state word which is enough context because the next refresh will resolve them.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above) — none recorded for `eni`.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- a9s golden doc — expected related targets (`ec2`, `eip`, `elb`, `lambda`, `nat`, `sg`, `subnet`, `vpc`, `vpce`, `ct-events`) — `docs/related-resources.md` § Per-type contract row `eni` and § `eni`.
- a9s golden doc — Wave 1 signals for `eni` (`Status` bucketing, `available` orphan, requester-managed zombie) — `docs/attention-signals.md` § Networking row `eni`.
- a9s golden doc — Wave 2 and Wave 3 are `None` for `eni` — `docs/attention-signals.md` § Networking row `eni`.
- a9s golden doc — read-only invariant — `docs/architecture.md` § "What is a9s?".
- a9s golden doc — `ct-events` universal pivot — `docs/related-resources.md` § Policy item 4.
- AWS SDK Go v2 — `Status` enum values (`available`, `associated`, `attaching`, `in-use`, `detaching`) — `AWS SDK Go v2 — service/ec2/types.NetworkInterfaceStatus`.
- AWS SDK Go v2 — `NetworkInterface.Status` field — `AWS SDK Go v2 — service/ec2/types.NetworkInterface § Status`.
- AWS SDK Go v2 — `NetworkInterface.Attachment.InstanceId` for `ec2` pivot — `AWS SDK Go v2 — service/ec2/types.NetworkInterfaceAttachment § InstanceId`.
- AWS SDK Go v2 — `NetworkInterface.Association.AllocationId` / `PublicIp` for `eip` pivot — `AWS SDK Go v2 — service/ec2/types.NetworkInterfaceAssociation § AllocationId, PublicIp`.
- AWS SDK Go v2 — `NetworkInterface.Groups[].GroupId` for `sg` pivot — `AWS SDK Go v2 — service/ec2/types.NetworkInterface § Groups`.
- AWS SDK Go v2 — `NetworkInterface.SubnetId` for `subnet` pivot — `AWS SDK Go v2 — service/ec2/types.NetworkInterface § SubnetId`.
- AWS SDK Go v2 — `NetworkInterface.VpcId` for `vpc` pivot — `AWS SDK Go v2 — service/ec2/types.NetworkInterface § VpcId`.
- AWS SDK Go v2 — `NetworkInterface.InterfaceType` values (`natGateway`, `load_balancer`, `network_load_balancer`, `gateway_load_balancer`, `vpc_endpoint`, `lambda`) — `AWS SDK Go v2 — service/ec2/types.NetworkInterfaceType`.
- AWS SDK Go v2 — `NetworkInterface.Description` (carrier for owning-service id when `InterfaceType` is a managed variant) — `AWS SDK Go v2 — service/ec2/types.NetworkInterface § Description`.
- AWS SDK Go v2 — `NetworkInterface.RequesterManaged` (zombie detection precondition) — `AWS SDK Go v2 — service/ec2/types.NetworkInterface § RequesterManaged`.
- a9s-devops consultation — pivot from `eni` to `elb`/`lambda`/`nat`/`vpce` via `InterfaceType` + `Description` parsing — `a9s-devops (2026-04-20): possible=yes, worth=yes. The ENI shape exposes no direct owning-resource ID for managed variants; AWS encodes the owner into InterfaceType plus a stable Description prefix, which is the standard cross-link used by operators and the in-console Network Interface detail view.`
- a9s-devops consultation — zombie detection (`RequesterManaged` + cross-reference to already-loaded sibling lists) — `a9s-devops (2026-04-20): possible=yes, worth=yes. Requester-managed ENIs whose owner is gone are a real cleanup item (common after Lambda VPC churn and failed vpce teardown). Sibling-list cross-reference is zero-cost during normal browsing and matches the golden-doc's Wave 1 phrasing.`
