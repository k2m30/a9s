---
shortName: vpc
name: VPCs
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Vpc.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# vpc — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `vpc`
- **Display name**: VPCs
- **AWS API reference**: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Vpc.html
- **List API**: `DescribeVpcs` (returns the full `Vpc` shape per VPC, including `VpcId`, `State`, `CidrBlock`, `IsDefault`, `OwnerId`, `DhcpOptionsId`, `InstanceTenancy`, `CidrBlockAssociationSet[]`, `Ipv6CidrBlockAssociationSet[]`, and `Tags[]`).
- **Describe API (if any)**: not used for Wave 1 — list response already carries everything needed. Wave 2 uses `DescribeFlowLogs` (account-wide, client-side filter by `ResourceType=VPC`), not a per-VPC Describe call.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `cfn`, `ec2`, `elb`, `eni`, `igw`, `nat`, `rtb`, `sg`, `subnet`, `tgw`, `vpce`, `ct-events`.

### `cfn`

- **Why related**: The CloudFormation stack that created this VPC — operator's first pivot for "who owns this network and where is it declared in IaC?". Also the path to see the full set of resources the stack manages if a VPC change is in flight.
- **How discovered**: Read the `aws:cloudformation:stack-name` tag from this VPC's `Tags[]` (AWS writes it automatically on any resource created by a stack); open the matching entry in the already-loaded `cfn` list by `StackName`. No extra AWS call needed when the tag is present. — a9s-devops (2026-04-20): possible=yes (CloudFormation stamps the `aws:cloudformation:stack-name` tag on every resource it creates, including VPCs), worth=yes. Tag-based lookup is the standard IaC-ownership pivot — `DescribeStackResources` per VPC works too but costs one API call and is only needed when the tag is stripped.
- **Count shown**: yes (0 or 1 — a VPC belongs to at most one stack).

### `ec2`

- **Why related**: EC2 instances running inside this VPC — the answer to "what compute is in this network right now?" and the first thing an operator looks at during a VPC-level incident or before deleting a VPC.
- **How discovered**: Filter the already-loaded `ec2` list client-side where `Instance.VpcId == this.VpcId`. No extra AWS call needed. — a9s-devops (2026-04-20): possible=yes (`Instance.VpcId` is on every `DescribeInstances` response), worth=yes. Reverse-scan of the already-loaded list is the cheapest path and matches how operators mentally model the VPC → instance parent-child relationship.
- **Count shown**: yes.

### `elb`

- **Why related**: Load balancers (ELBv2 ALB/NLB/GWLB) inside this VPC — operator pivots here when a public endpoint is unreachable or when auditing what the VPC exposes.
- **How discovered**: Filter the already-loaded `elb` list where `LoadBalancer.VpcId == this.VpcId`. Classic (ELBv1) load balancers also carry `VPCId` on `DescribeLoadBalancers`. — a9s-devops (2026-04-20): possible=yes, worth=yes. ELBv2 carries `VpcId` directly on the list response.
- **Count shown**: yes.

### `eni`

- **Why related**: Every managed attachment into the VPC (RDS, Lambda-in-VPC, EFS mount target, VPC endpoint, NAT gateway, load balancer, ECS awsvpc task) surfaces as an ENI. This is the operator's catch-all for "what else, besides EC2 instances, is sitting in this network?" — especially useful when IP space is filling up or when a VPC won't delete.
- **How discovered**: Filter the already-loaded `eni` list where `NetworkInterface.VpcId == this.VpcId`. — a9s-devops (2026-04-20): possible=yes (`NetworkInterface.VpcId` is on every `DescribeNetworkInterfaces` response), worth=yes. ENIs are the single view that explains which AWS services have injected themselves into the VPC.
- **Count shown**: yes.

### `igw`

- **Why related**: The internet gateway attached to this VPC — operator opens this to answer "does this VPC have internet egress at all?" and to spot an attached-but-unused IGW.
- **How discovered**: Filter the already-loaded `igw` list where any entry in `InternetGateway.Attachments[].VpcId` equals this VPC's `VpcId`. — a9s-devops (2026-04-20): possible=yes, worth=yes. `Attachments[].VpcId` is the canonical pivot; a VPC usually has 0 or 1 IGW.
- **Count shown**: yes (usually 0 or 1).

### `nat`

- **Why related**: NAT gateways inside this VPC — operator pivots here when private-subnet egress is broken, or when auditing NAT cost (each NAT ~$32/mo plus data processing).
- **How discovered**: Filter the already-loaded `nat` list where `NatGateway.VpcId == this.VpcId`. — a9s-devops (2026-04-20): possible=yes (`NatGateway.VpcId` is on every `DescribeNatGateways` response), worth=yes.
- **Count shown**: yes.

### `rtb`

- **Why related**: Route tables scoped to this VPC — operator reaches here to trace "where does traffic from subnet X actually go?" and to spot blackhole routes after a gateway/ENI was deleted.
- **How discovered**: Filter the already-loaded `rtb` list where `RouteTable.VpcId == this.VpcId`. — a9s-devops (2026-04-20): possible=yes, worth=yes. The VPC main route table is included in the same filter; the `Associations[].Main` flag identifies it.
- **Count shown**: yes.

### `sg`

- **Why related**: Security groups scoped to this VPC — operator reaches here for connectivity debugging ("what SGs exist in this VPC, and which one allows / denies port X?") and for exposure audits.
- **How discovered**: Filter the already-loaded `sg` list where `SecurityGroup.VpcId == this.VpcId`. — a9s-devops (2026-04-20): possible=yes (`SecurityGroup.VpcId` is on every `DescribeSecurityGroups` response), worth=yes.
- **Count shown**: yes.

### `subnet`

- **Why related**: Subnets in this VPC — the fundamental breakdown of the VPC's IP plan across AZs. Operator pivots here to check AZ coverage, free IP addresses, and public/private layout.
- **How discovered**: Filter the already-loaded `subnet` list where `Subnet.VpcId == this.VpcId`. — a9s-devops (2026-04-20): possible=yes (`Subnet.VpcId` is on every `DescribeSubnets` response), worth=yes. This cross-reference is also the input to the Wave 1 "empty VPC" signal in §3.1.
- **Count shown**: yes.

### `tgw`

- **Why related**: Transit gateways this VPC is attached to — operator pivots here when inter-VPC or hybrid-network connectivity is broken, or when auditing what networks this VPC can reach.
- **How discovered**: TGW list responses do not carry child VPC IDs. Call `DescribeTransitGatewayAttachments` and filter client-side where `ResourceType == vpc` and `ResourceId == this.VpcId`; the matching entries' `TransitGatewayId` values are the TGWs this VPC attaches to. Open each one in the already-loaded `tgw` list. — a9s-devops (2026-04-20): possible=yes, worth=yes. Most VPCs have 0 or 1 TGW attachment, so the extra call is cheap and the operational value is high (TGW debugging is one of the harder incident paths).
- **Count shown**: yes (typically 0 or 1).

### `vpce`

- **Why related**: VPC endpoints inside this VPC (Gateway and Interface) — operator pivots here when a private call to S3/DynamoDB/an AWS service is failing, or when auditing how this VPC reaches AWS APIs without going to the internet.
- **How discovered**: Filter the already-loaded `vpce` list where `VpcEndpoint.VpcId == this.VpcId`. — a9s-devops (2026-04-20): possible=yes (`VpcEndpoint.VpcId` is on every `DescribeVpcEndpoints` response), worth=yes.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Audit trail for VPC-level changes — universal pivot for "who changed this, and when?". Typical CloudTrail event names for VPC operations: `CreateVpc`, `DeleteVpc`, `ModifyVpcAttribute`, `AssociateVpcCidrBlock`, `CreateFlowLogs`, `DeleteFlowLogs`.
- **How discovered**: Call CloudTrail `LookupEvents` filtered by `ResourceName == VpcId` (and/or event-name filter). Universal pivot — applies to every registered type; see `related-resources.md` § Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `State == available` → Healthy.
  - **State bucket**: Healthy.
  - **How obtained**: `State` field (type `VpcState`) on the `Vpc` returned by `DescribeVpcs`.

- **Signal**: `State == pending` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `State` field on the list-response VPC.

- **Signal**: No subnets in this VPC → Warning (empty VPC).
  - **State bucket**: Warning.
  - **How obtained**: Cross-reference the already-loaded `subnet` list; count entries where `Subnet.VpcId == this.VpcId`. If zero, raise the signal.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: No VPC flow logs configured for this VPC → Warning (CIS / Well-Architected SEC — network traffic is unlogged).
  - **State bucket**: Warning.
  - **API call**: `DescribeFlowLogs` — one account-wide call; filter the response client-side for entries where `ResourceId == this.VpcId` (the list-level filter `ResourceType=VPC` is applied client-side on the shared response). Raise the signal when no flow log targets this VPC.
  - **Cost shape**: account-wide (one call covers every VPC in the region).

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `DescribeVpcAttribute(EnableDnsSupport)` per VPC — would detect VPCs that have DNS support or DNS hostnames disabled (a common "why can't my instances resolve names?" cause), but requires one extra API call per VPC and is out of the Wave 2 budget.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". Never appears on yellow/red/dim rows. |
| S4 | Status / description column text | Short human-readable cause. Healthy rows render blank. |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping for this resource:

- **Wave 1 Healthy** (`available`) → no §4 row. S2 renders green, S4 renders blank. Silence is the UX.
- **Wave 1 Warning** signals → S2 (yellow) + S4 (cause). No S1, S3, S5.
- **Wave 2 informational Warning on a Healthy VPC** (no flow logs) → `~` glyph on the green row. S3 + S4 + S5. Does not bump S1 (informational, not an operational outage).

One row per §3 signal (Healthy case omitted per rule):

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `State == pending` | 1 | Warning | n/a | S2, S4 | `pending: VPC being created` | n/a (Wave 1 Warning has no S5) |
| no subnets in VPC | 1 | Warning | n/a | S2, S4 | `empty: no subnets` | n/a |
| no flow logs for this VPC | 2 | Healthy | `~` | S3, S4, S5 | `no flow logs` | `No VPC flow logs configured — network traffic here is unlogged (CIS/Well-Architected SEC).` |

## 4.1 UX review (two sentences)

At 3am, glancing at the list, a yellow row with `pending: VPC being created` or `empty: no subnets` is already self-explanatory, and a green row prefixed `~` with `no flow logs` tells the operator what's missing without requiring a detail pivot. All problem rows are self-explanatory in the list — operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (DNS-support/hostnames per-VPC attribute check).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` § "What is a9s?").
- Default-VPC-present detection. `Vpc.IsDefault == true` exists on the list response, but whether a region still has its default VPC is a one-off account-posture question (CIS 5.1), not a daily-driver signal. — a9s-devops (2026-04-20): possible=yes, worth=no. This belongs in a Security Hub / Config rule, not in the attention column an operator glances at every day.
- VPC peering connections, Client VPN endpoints, Site-to-Site VPN gateways. These are not registered a9s resource types today; the related panel cannot surface them without adding those types first.
- `BlockPublicAccessStates` surfacing. The field exists on `Vpc` but is an AWS-VPC-wide security control whose operational value (detecting a newly-enabled or newly-disabled account posture) is outside the per-row attention contract. — a9s-devops (2026-04-20): possible=yes, worth=no.
- CloudWatch metrics per VPC — AWS does not publish a per-VPC CloudWatch namespace; VPC-level traffic visibility only exists via flow logs (covered in Wave 2 as presence/absence, not as volume metrics). — a9s-devops (2026-04-20): possible=no, worth=no.

## 6. Citations

- `shortName`, display name, signal cells, list API — `docs/attention-signals.md` § Networking row for `vpc` (line 51).
- AWS API reference URL, related targets list — `docs/related-resources.md` § Per-type contract row for `vpc` (line 108) and § `vpc` narrative block (lines 1013–1028).
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
- `Vpc.State`, `VpcId`, `CidrBlock`, `IsDefault`, `OwnerId`, `DhcpOptionsId`, `InstanceTenancy`, `CidrBlockAssociationSet`, `Ipv6CidrBlockAssociationSet`, `Tags`, `BlockPublicAccessStates` field names — `AWS SDK Go v2 — service/ec2/types.Vpc`.
- `VpcState` enum values (`pending`, `available`) — `AWS SDK Go v2 — service/ec2/types.VpcState` (`VpcStatePending`, `VpcStateAvailable`).
- `InternetGateway.Attachments[].VpcId` for the `igw` cross-reference — `AWS SDK Go v2 — service/ec2/types.InternetGateway § Attachments` and `service/ec2/types.InternetGatewayAttachment § VpcId`.
- `TransitGatewayAttachment.ResourceId`, `ResourceType`, `TransitGatewayId` for the `tgw` cross-reference — `AWS SDK Go v2 — service/ec2/types.TransitGatewayAttachment § ResourceId, ResourceType, TransitGatewayId`.
- `FlowLog.ResourceId` for the Wave 2 cross-reference — `AWS SDK Go v2 — service/ec2/types.FlowLog § ResourceId`.
- `ct-events` as universal pivot — `docs/related-resources.md` § Policy (line 34).
- CloudTrail event-name filter (`CreateVpc`, `DeleteVpc`, `ModifyVpcAttribute`, `AssociateVpcCidrBlock`, `CreateFlowLogs`, `DeleteFlowLogs`) — `a9s-devops (2026-04-20): possible=yes (CloudTrail records all VPC management-plane calls), worth=yes. These event names are the filter operators run when investigating a VPC state change.`
- `cfn` discovery via `aws:cloudformation:stack-name` tag — `a9s-devops (2026-04-20): possible=yes, worth=yes. CloudFormation stamps this tag on every resource it creates; tag-based lookup avoids a per-VPC DescribeStackResources call.`
- `ec2`, `elb`, `eni`, `nat`, `rtb`, `sg`, `subnet`, `vpce` discovered via reverse-scan of already-loaded lists on `VpcId` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Every one of these AWS list responses carries `VpcId` (or an equivalent) directly; scanning already-loaded lists is cheaper than any extra AWS call.`
- `tgw` discovery via `DescribeTransitGatewayAttachments` filter — `a9s-devops (2026-04-20): possible=yes, worth=yes. TGW list responses do not carry child VPC IDs, so a single `DescribeTransitGatewayAttachments` call is the only path; most VPCs have ≤1 TGW attachment, so the cost is minimal.`
- Wave 1 signals (`State`, empty-VPC cross-ref) and Wave 2 signal (`DescribeFlowLogs`) — `docs/attention-signals.md` § Networking row for `vpc`.
- Wave 3 DNS-attribute probe out-of-scope — `docs/attention-signals.md` § Networking row for `vpc` Wave 3 cell.
- Default-VPC detection out-of-scope rationale — `a9s-devops (2026-04-20): possible=yes (Vpc.IsDefault is on the list response), worth=no. Not a daily-driver signal; belongs in Security Hub / Config, not in the attention column.`
- `BlockPublicAccessStates` surfacing out-of-scope — `a9s-devops (2026-04-20): possible=yes, worth=no. Account-posture control, not per-row operational attention.`
- No per-VPC CloudWatch namespace — `a9s-devops (2026-04-20): possible=no, worth=no. AWS publishes flow logs (presence covered in Wave 2) but no per-VPC metric namespace.`
