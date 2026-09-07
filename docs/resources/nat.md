---
shortName: nat
name: NAT Gateways
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NatGateway.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# nat — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `nat`
- **Display name**: NAT Gateways
- **AWS API reference**: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_NatGateway.html>
- **List API**: `DescribeNatGateways`
- **Describe API (if any)**: not used — `DescribeNatGateways` returns the full `NatGateway` shape (including `State`, `FailureCode`, `FailureMessage`, `NatGatewayAddresses[]`), so Wave 2 is empty.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `alarm`, `eip`, `eni`, `rtb`, `subnet`, `vpc`, `ct-events`.

### `alarm`

- **Why related**: NAT bandwidth/error alarms — operators pivot here when investigating NAT throughput, port exhaustion, or packet-drop incidents.
- **How discovered**: cross-reference the already-loaded `alarm` list by `MetricAlarm.Dimensions[]` entries where `Name==NatGatewayId` and `Value==<this NatGatewayId>` (CloudWatch `AWS/NATGateway` namespace) — a9s-devops persona: standard CloudWatch cross-ref pattern, the same mechanism the zombie-alarm check uses, `docs/attention-signals.md § Signals § MONITORING` row `alarm`.
- **Count shown**: unknown.

### `eip`

- **Why related**: NAT gateway consuming this EIP — operators pivot to check EIP association health, billing, and allocation ownership.
- **How discovered**: read `NatGatewayAddresses[].AllocationId` on the NAT and cross-reference the already-loaded `eip` list by `Address.AllocationId`.
- **Count shown**: unknown.

### `eni`

- **Why related**: NAT backing ENI — the requester-managed ENI that carries NAT traffic; operators need this to trace flow-log entries, inspect private IPs, and confirm AZ placement.
- **How discovered**: read `NatGatewayAddresses[].NetworkInterfaceId` on the NAT and cross-reference the already-loaded `eni` list by `NetworkInterface.NetworkInterfaceId`.
- **Count shown**: unknown.

### `rtb`

- **Why related**: route tables with default routes pointing at this NAT — confirms which private subnets actually egress through this gateway; orphaned NATs (no route targets) are cost waste.
- **How discovered**: cross-reference the already-loaded `rtb` list by any `Route.NatGatewayId==<this NatGatewayId>`.
- **Count shown**: unknown.

### `subnet`

- **Why related**: subnet the NAT lives in (must be public for a public NAT) — operators check that the placement subnet has a `0.0.0.0/0 → igw` route and correct AZ.
- **How discovered**: read `NatGateway.SubnetId` on the NAT and cross-reference the already-loaded `subnet` list by `Subnet.SubnetId`.
- **Count shown**: unknown.

### `vpc`

- **Why related**: parent VPC — operators pivot to the VPC to review flow logs, tenancy, and overall network topology.
- **How discovered**: read `NatGateway.VpcId` on the NAT and cross-reference the already-loaded `vpc` list by `Vpc.VpcId`.
- **Count shown**: unknown.

### `ct-events`

- **Why related**: audit trail for NAT changes — who created/deleted/modified this NAT and when.
- **How discovered**: universal pivot — applies to every registered type; see `docs/related-resources.md` §Policy point 4.
- **Count shown**: unknown.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeNatGateways](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeNatGateways.html)

Transcribed from `docs/attention-signals.md § Signals § NETWORKING` row `nat`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `State==pending`.
  - **State bucket**: Warning.
  - **How obtained**: `NatGateway.State` on the `DescribeNatGateways` response.

- **Signal**: `State==deleting`.
  - **State bucket**: Warning.
  - **How obtained**: `NatGateway.State` on the `DescribeNatGateways` response.

- **Signal**: `State==failed`.
  - **State bucket**: Broken.
  - **How obtained**: `NatGateway.State` on the `DescribeNatGateways` response.

- **Signal**: `State == deleted`.
  - **State bucket**: Dim.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `ErrorPortAllocation`.
- OUT OF SCOPE: CloudWatch `PacketsDropCount`.
- OUT OF SCOPE: CloudWatch `BytesOutToDestination==0` cost-waste detection.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `nat`: Wave 1 issue-colored rows only — this type registers no Wave 2 enricher, so nothing else bumps the count.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `State==pending` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `pending` |
| `State==deleting` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `deleting` |
| `State==failed` + `FailureCode` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `failed` |
| `State == deleted` | 1 | Dim | n/a | S2, S4 | `deleted` |

Notes on filling list and detail text:

- The `<FailureCode>` placeholder in S4 is replaced verbatim with the SDK value (e.g. `failed: InsufficientFreeAddressesInSubnet`, `failed: Gateway.NotAttached`, `failed: InvalidAllocationID.NotFound`, `failed: Resource.AlreadyAssociated`, `failed: InternalError`, `failed: InvalidSubnetID.NotFound`). Codes are short enough that S4 stays ≤ 40 chars for every documented code.
- `<FailureMessage>` in S5 is replaced with the verbatim AWS message (e.g. "Subnet has insufficient free addresses to create this NAT gateway.").
- `State==available` is Healthy — omitted from §4 (S4 renders blank).
- Banned jargon check: none of `Wave 1`, `Wave 2`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity` appears in any S4/S5 cell.
- Bare-keyword check: no row uses a bare `pending`/`failed`/`deleting` — each pairs the state with a cause fragment.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, the operator sees a red row for a failed NAT reading `failed` in the Status column, and opens detail for the AWS failure code that says why. All problem rows are self-explanatory in the list — operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- `State==deleted` terminal-tombstone rendering — not specified in `docs/attention-signals.md § Signals § NETWORKING` row `nat`. a9s-devops persona: possible=yes (SDK exposes it), worth=low for daily-driver (deleted NAT gateways age out of the list response quickly). Flagged here rather than invented.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- Per-type contract row for `nat` — `docs/related-resources.md` § Per-type contract, row `nat`.
- `alarm`, `ct-events`, `eip`, `eni`, `rtb`, `subnet`, `vpc` pivots with reasoning — `docs/related-resources.md` § `nat`.
- `eip` discovery via `NatGatewayAddresses[].AllocationId` — `docs/related-resources.md` § `nat` bullet `eip`; confirmed by `AWS SDK Go v2 — ec2/types.NatGatewayAddress § AllocationId`.
- `eni` discovery via `NatGatewayAddresses[].NetworkInterfaceId` — `docs/related-resources.md` § `nat` bullet `eni`; confirmed by `AWS SDK Go v2 — ec2/types.NatGatewayAddress § NetworkInterfaceId`.
- `rtb` discovery via `Route.NatGatewayId` — `docs/related-resources.md` § `nat` bullet `rtb`; confirmed by `AWS SDK Go v2 — ec2/types.Route § NatGatewayId`.
- `subnet` discovery via `NatGateway.SubnetId` — `AWS SDK Go v2 — ec2/types.NatGateway § SubnetId`.
- `vpc` discovery via `NatGateway.VpcId` — `AWS SDK Go v2 — ec2/types.NatGateway § VpcId`.
- `alarm` discovery via CloudWatch `AWS/NATGateway` namespace + `NatGatewayId` dimension — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. CloudWatch NAT alarms conventionally carry NatGatewayId dimension`, which is the cross-reference the zombie-alarm check makes — `docs/attention-signals.md § Signals § MONITORING` row `alarm`.
- `ct-events` universal pivot — `docs/related-resources.md` § Policy point 4.
- Wave 1 `State` transitions (`available`/`pending`/`failed`/`deleting`) — `docs/attention-signals.md § Signals § NETWORKING` row `nat`; confirmed by `AWS SDK Go v2 — ec2/types.NatGateway § State` (documented enum values `pending`, `failed`, `available`, `deleting`, `deleted`).
- `FailureCode` and `FailureMessage` as Broken-detail pair — `docs/attention-signals.md § Signals § NETWORKING` row `nat`; field shapes confirmed by `AWS SDK Go v2 — ec2/types.NatGateway § FailureCode` and `§ FailureMessage`. Documented failure codes `InsufficientFreeAddressesInSubnet`, `Gateway.NotAttached`, `InvalidAllocationID.NotFound`, `Resource.AlreadyAssociated`, `InternalError`, `InvalidSubnetID.NotFound` come from the same SDK comment.
- Wave 2 is empty for `nat` — `docs/attention-signals.md § Signals § NETWORKING` row `nat`.
- Wave 3 signals — `docs/attention-signals.md § Not yet implemented`.
- `Count shown: unknown` for every related target — `docs/related-resources.md` § `nat` is silent on per-target counts. a9s-devops persona (2026-04-20): possible=no to cite from golden docs, worth=no to guess. Remaining gap is documented, not invented.
- `State==deleted` treated as Dim but not surfaced — `a9s-devops persona (2026-04-20): possible=yes (SDK enum), worth=no for daily-driver use.` Recorded in §5 Out of Scope rather than §4, because `docs/attention-signals.md § Signals § NETWORKING` row `nat` carries no finding for it.
- Read-only invariant — `docs/architecture.md` § "What is a9s?".

<!-- BEGIN GENERATED: header -->
nat — NETWORKING. Lifecycle key: `state`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| nat.state.pending | pending | warn | wave1 | — |
| nat.state.deleting | deleting | warn | wave1 | — |
| nat.state.failed | failed | broken | wave1 | — |
| nat.state.deleted | deleted | dim | wave1 | — |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| vpc | VPCs | no |
| subnet | Subnets | no |
| rtb | Route Tables | yes |
| alarm | CloudWatch Alarms | yes |
| eip | Elastic IPs | no |
| eni | Network Interfaces | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
