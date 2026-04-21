---
shortName: subnet
name: Subnets
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Subnet.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# subnet — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `subnet`
- **Display name**: Subnets
- **AWS API reference**: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Subnet.html
- **List API**: `DescribeSubnets`
- **Describe API (if any)**: not used — all Wave 1 signals derive from the list response

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `asg`, `cfn`, `ct-events`, `ec2`, `efs`, `eks`, `elb`, `eni`, `nat`, `rtb`, `vpc`, `vpce`.

### `vpc`

- **Why related**: Every subnet belongs to exactly one VPC — operator diagnosing a subnet issue almost always needs to confirm the VPC-level context (CIDR, flow logs, empty-VPC warnings).
- **How discovered**: read field `Subnet.VpcId` on the resource.
- **Count shown**: yes (always 1).

### `rtb`

- **Why related**: Subnet's effective routing is governed by the associated route table — or the VPC main route table if no explicit association exists. The spec already uses this cross-reference for the `MapPublicIpOnLaunch` misconfigured-public-subnet check, so the operator needs the pivot to inspect routes.
- **How discovered**: cross-reference the already-loaded `rtb` list by `RouteTable.Associations[].SubnetId == Subnet.SubnetId`; fall back to the VPC main route table (`Associations[].Main == true` with matching `VpcId`) when there is no explicit association.
- **Count shown**: yes.

### `ec2`

- **Why related**: Operator's most common subnet question is "what is running in here?" — list instances placed in this subnet to triage capacity, ENI allocation, or routing issues.
- **How discovered**: cross-reference the already-loaded `ec2` list by `Instance.SubnetId == Subnet.SubnetId` (primary ENI). Cited use of `Instance.SubnetId` appears at related-resources.md line 414.
- **Count shown**: yes.

### `eni`

- **Why related**: Subnet's `AvailableIpAddressCount` depletes as ENIs are provisioned; operator needs to see the ENI population directly to understand what's consuming IPs (instances, NAT GWs, VPC endpoints, Lambda, etc.).
- **How discovered**: cross-reference the already-loaded `eni` list by `NetworkInterface.SubnetId == Subnet.SubnetId`.
- **Count shown**: yes.

### `nat`

- **Why related**: NAT gateways live inside a subnet and must be placed in a public subnet; the NAT's health and presence are routinely cross-checked against subnet configuration.
- **How discovered**: cross-reference the already-loaded `nat` list by `NatGateway.SubnetId == Subnet.SubnetId`.
- **Count shown**: yes.

### `elb`

- **Why related**: ELBv2 load balancers attach to subnets per AZ; operator pivots here to see which LBs depend on this subnet (e.g. before changes or when diagnosing a 5xx that tracks back to one AZ).
- **How discovered**: cross-reference the already-loaded `elb` list by `LoadBalancer.AvailabilityZones[].SubnetId == Subnet.SubnetId`.
- **Count shown**: yes.

### `eks`

- **Why related**: EKS cluster control plane and workloads declare subnets in `ResourcesVpcConfig.SubnetIds`; pivot lets operator confirm cluster subnet mapping during a connectivity or AZ-rebalance investigation.
- **How discovered**: cross-reference the already-loaded `eks` list by `Cluster.ResourcesVpcConfig.SubnetIds[] contains Subnet.SubnetId` (the `SubnetIds` field is only populated after `eks` Wave 2 `DescribeCluster`).
- **Count shown**: yes.

### `asg`

- **Why related**: Auto Scaling groups launch instances into one or more subnets via `VPCZoneIdentifier` — operator needs this to understand which ASG will replace capacity in the affected AZ.
- **How discovered**: cross-reference the already-loaded `asg` list by parsing `AutoScalingGroup.VPCZoneIdentifier` (comma-separated subnet IDs) for `Subnet.SubnetId`.
- **Count shown**: yes.

### `efs`

- **Why related**: EFS file systems project into a subnet via one mount target per AZ; losing a subnet (or running out of IPs) breaks EFS reachability from that AZ. Related-resources.md §subnet lists `efs` as a DevOps audit pivot.
- **How discovered**: cross-reference `efs` mount-target metadata by subnet ID — mount targets are returned by `DescribeMountTargets` (per file system), carrying `MountTargetDescription.SubnetId`. Because a9s does not persist mount-target data in the top-level `efs` list, the pivot surfaces an `efs` count only when mount-target data has been populated via the `efs` resource's own Wave 2 fetch.
- **Count shown**: yes (subject to the mount-target data being loaded).

### `vpce`

- **Why related**: Interface-type VPC endpoints materialize an ENI in each configured subnet; when endpoints fail or land in an AZ-down subnet, this pivot shows which endpoints are affected.
- **How discovered**: cross-reference the already-loaded `vpce` list by `VpcEndpoint.SubnetIds[] contains Subnet.SubnetId` (Interface endpoints only; Gateway endpoints attach to route tables, not subnets).
- **Count shown**: yes.

### `cfn`

- **Why related**: When a subnet was created by CloudFormation, operator needs one-keypress access to the stack to understand drift, ownership, and change history.
- **How discovered**: read the subnet's `Tags[]` for `aws:cloudformation:stack-name` (and/or `aws:cloudformation:stack-id`); cross-reference with the loaded `cfn` list. Tag-based discovery is the conventional CFN back-reference — no dedicated AWS API field exists on `Subnet`.
- **Count shown**: yes (0 or 1 — a subnet is owned by at most one stack).

### `ct-events`

- **Why related**: Universal pivot — applies to every registered type; see related-resources.md §Policy.
- **How discovered**: `LookupEvents(ResourceName=<SubnetId>)`.
- **Count shown**: unknown (ct-events is event-stream, not count-oriented).

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md` §Networking — `subnet` row.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `State == available`.
  - **State bucket**: Healthy.
  - **How obtained**: `Subnet.State` on the `DescribeSubnets` response.

- **Signal**: `State == pending`.
  - **State bucket**: Warning.
  - **How obtained**: `Subnet.State` on the list response.

- **Signal**: `State == unavailable`.
  - **State bucket**: Broken.
  - **How obtained**: `Subnet.State` on the list response.

- **Signal**: `State == failed`.
  - **State bucket**: Broken.
  - **How obtained**: `Subnet.State` on the list response. Per AWS SDK: "The underlying infrastructure to support the subnet failed to provision as expected."

- **Signal**: `State == failed-insufficient-capacity`.
  - **State bucket**: Broken.
  - **How obtained**: `Subnet.State` on the list response. Per AWS SDK: "The underlying infrastructure to support the subnet failed to provision due to a shortage of EC2 instance capacity." Operator implication: ENI provisioning into this subnet will fail — move workloads to another AZ.

- **Signal**: `AvailableIpAddressCount / CIDR-size < 0.1` (IP pool running low).
  - **State bucket**: Warning.
  - **How obtained**: compute on the list response — `Subnet.AvailableIpAddressCount` against the host count of `Subnet.CidrBlock`.

- **Signal**: `AvailableIpAddressCount / CIDR-size < 0.02` (IP pool nearly exhausted).
  - **State bucket**: Broken.
  - **How obtained**: compute on the list response — same fields as above.

- **Signal**: `MapPublicIpOnLaunch == true` AND the effective route table for this subnet has no `0.0.0.0/0 → IGW` default route (where "effective" = the explicitly associated route table, or the VPC main route table when no explicit association exists).
  - **State bucket**: Warning (misconfigured public subnet).
  - **How obtained**: read `Subnet.MapPublicIpOnLaunch`; cross-reference the already-loaded `rtb` list by `Associations[].SubnetId` (falling back to the VPC main route table); scan `Routes[]` for a `DestinationCidrBlock == "0.0.0.0/0"` with `GatewayId` starting `igw-`.

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: None.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". Never appears on yellow/red/dim rows. |
| S4 | Status / description column text | Short human-readable cause. Healthy rows render blank. |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!`: S1, S3, S4, S5.
- **Wave 2 background finding on a Healthy row, informational** → `~`: S3, S4, S5. No S1.
- **Wave 2 finding on an already yellow/red/dim row** → S3 suppressed, S4 deduplicates, S5 still carries the full sentence, S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `State == pending` | 1 | Warning | n/a | S2, S4 | `pending: provisioning` | — |
| `State == unavailable` | 1 | Broken | n/a | S2, S4 | `unavailable` | — |
| `State == failed` | 1 | Broken | n/a | S2, S4 | `failed: infrastructure` | — |
| `State == failed-insufficient-capacity` | 1 | Broken | n/a | S2, S4 | `failed: AZ out of capacity` | — |
| IP pool low (`< 10%` free) | 1 | Warning | n/a | S2, S4 | `IPs low: N free of M` | — |
| IP pool exhausted (`< 2%` free) | 1 | Broken | n/a | S2, S4 | `IPs exhausted: N free of M` | — |
| Misconfigured public subnet (auto-assign public IP, no IGW default route) | 1 | Warning | n/a | S2, S4 | `public IP on launch, no IGW route` | — |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`pending`, `available`, `failed`) in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column.
- For signals that legitimately have no operator-actionable cause (pure Healthy `State == available`), the row is omitted from this table.
- Keep both columns short: List text ≤ 40 chars, Detail text ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? All problem rows are self-explanatory in the list — a red `failed: AZ out of capacity` row tells the operator to move workloads to another AZ, a red `IPs exhausted: 3 free of 256` row tells them the subnet has run out of addresses, and a yellow `public IP on launch, no IGW route` row tells them the subnet is misconfigured — operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (none for this resource).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- AZ-level health (which AZ a subnet sits in is informational; the AZ's operational status is not a subnet signal in Wave 1/2 — confirmed unavailable from Wave 1 AWS surface on `Subnet`).

## 6. Citations

- Related-panel targets list (`asg, cfn, ct-events, ec2, efs, eks, elb, eni, nat, rtb, vpc, vpce`) — `docs/related-resources.md` § Per-type contract, `subnet` row.
- Related-panel `vpc` discovery (`Subnet.VpcId`) — `AWS SDK Go v2 — ec2/types.Subnet § VpcId`.
- Related-panel `rtb` discovery (explicit association via `Associations[].SubnetId`, fallback to VPC main) — `docs/related-resources.md` § `subnet` Wave 1 row ("including the VPC main RTB when subnet has no explicit association"); `AWS SDK Go v2 — ec2/types.RouteTableAssociation § SubnetId, Main`.
- Related-panel `ec2` discovery (`Instance.SubnetId`) — `docs/related-resources.md` line 414 ("Instance.SubnetId — primary ENI's subnet"); `AWS SDK Go v2 — ec2/types.Instance § SubnetId`.
- Related-panel `eni` discovery (`NetworkInterface.SubnetId`) — `AWS SDK Go v2 — ec2/types.NetworkInterface § SubnetId`.
- Related-panel `nat` discovery (`NatGateway.SubnetId`) — `AWS SDK Go v2 — ec2/types.NatGateway § SubnetId`.
- Related-panel `elb` discovery (`LoadBalancer.AvailabilityZones[].SubnetId`) — `docs/related-resources.md` line 550 ("AZ subnets the LB listens in"); `AWS SDK Go v2 — elasticloadbalancingv2/types.AvailabilityZone § SubnetId`.
- Related-panel `eks` discovery (`Cluster.ResourcesVpcConfig.SubnetIds`) — `docs/related-resources.md` line 534 ("Cluster.ResourcesVpcConfig.SubnetIds — cluster subnets"); `AWS SDK Go v2 — eks/types.VpcConfigResponse § SubnetIds`.
- Related-panel `asg` discovery (parse `VPCZoneIdentifier`) — `docs/related-resources.md` line 191 ("AutoScalingGroup.VPCZoneIdentifier — subnets the ASG launches into"); `AWS SDK Go v2 — autoscaling/types.AutoScalingGroup § VPCZoneIdentifier`.
- Related-panel `efs` discovery (`MountTargetDescription.SubnetId`) — `docs/related-resources.md` line 500 ("MountTarget subnets"); `AWS SDK Go v2 — efs/types.MountTargetDescription § SubnetId`.
- Related-panel `vpce` discovery (Interface endpoints' `VpcEndpoint.SubnetIds`) — `docs/related-resources.md` line 1044 ("Interface endpoint subnets"); `AWS SDK Go v2 — ec2/types.VpcEndpoint § SubnetIds`.
- Related-panel `cfn` discovery (tag-based, `aws:cloudformation:stack-name`) — `AWS SDK Go v2 — ec2/types.Subnet § Tags`. Tag convention is an AWS-wide CFN behavior cited from related-resources.md's general use of CFN tag-based pivots.
- Related-panel `ct-events` universal pivot — `docs/related-resources.md` § Policy (universal pivot).
- §3 Wave 1 signals (`State` enum, `AvailableIpAddressCount`, `MapPublicIpOnLaunch`, `rtb` cross-ref) — `docs/attention-signals.md` § Networking, `subnet` row.
- §3 `State` enum values (`pending`, `available`, `unavailable`, `failed`, `failed-insufficient-capacity`) match exactly — `AWS SDK Go v2 — ec2/types.SubnetState` (const `SubnetStatePending`, `SubnetStateAvailable`, `SubnetStateUnavailable`, `SubnetStateFailed`, `SubnetStateFailedInsufficientCapacity`).
- §3 Wave 1 `failed-insufficient-capacity` operator meaning — `AWS SDK Go v2 — ec2/types.Subnet § State` (inline comment: "The underlying infrastructure to support the subnet failed to provision due to a shortage of EC2 instance capacity.").
- §3 Wave 1 `AvailableIpAddressCount` field — `AWS SDK Go v2 — ec2/types.Subnet § AvailableIpAddressCount` ("The number of unused private IPv4 addresses in the subnet. The IPv4 addresses for any stopped instances are considered unavailable.").
- §3 Wave 1 `MapPublicIpOnLaunch` field — `AWS SDK Go v2 — ec2/types.Subnet § MapPublicIpOnLaunch` ("Indicates whether instances launched in this subnet receive a public IPv4 address.").
- §3.2 "No Wave 2 signals" — `docs/attention-signals.md` § Networking, `subnet` row (Wave 2 cell = "None").
- §3.3 "None" (no Wave 3 items listed) — `docs/attention-signals.md` § Networking, `subnet` row (Wave 3 cell = "None").
- §5 read-only invariant — `docs/architecture.md` § "What is a9s?" (line 13: "a9s is a read-only terminal UI for AWS.").
- `efs` count caveat (mount-target data is not on the top-level `efs` list response — it requires `DescribeMountTargets` per FS) — `AWS SDK Go v2 — efs/types.FileSystemDescription` (no subnet field on `FileSystemDescription`; subnet data only on `MountTargetDescription`).
- `cfn` tag-based discovery is conventional (no direct field on `Subnet`) — a9s-devops (2026-04-20): possible=yes, worth=yes. Real-world subnets that are CFN-managed carry `aws:cloudformation:stack-name`; operator commonly pivots from a subnet to its stack during drift or change investigations. No AWS surface carries a direct Stack reference on `Subnet` itself, so tags are the only read-only discovery path.
- `efs` pivot worth surfacing despite indirect discovery — a9s-devops (2026-04-20): possible=yes, worth=yes. Subnet IP exhaustion or AZ failure directly breaks EFS mount-target reachability in that AZ; operator routinely asks "which FS mounts through this subnet?" during AZ incident triage. Surfaced as count-when-loaded, accepting that the count is present only when mount-target data has been fetched by the `efs` resource's own Wave 2.
