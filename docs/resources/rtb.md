---
shortName: rtb
name: Route Tables
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RouteTable.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# rtb — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `rtb`
- **Display name**: Route Tables
- **AWS API reference**: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_RouteTable.html
- **List API**: `DescribeRouteTables`
- **Describe API (if any)**: not used — Wave 1 only; `DescribeRouteTables` already returns the full `Routes[]`, `Associations[]`, and `Tags[]` needed for every signal and related-panel pivot.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `cfn`, `eni`, `igw`, `nat`, `subnet`, `tgw`, `vpc`, `vpce`, `ct-events`.

### `cfn`

- **Why related**: CloudFormation stack that created the route table — jump to the stack to see the template, parameters, and sibling resources managed alongside this RTB.
- **How discovered**: read `Tags[]` on the route table; match the value of the tag whose key is `aws:cloudformation:stack-name` against the already-loaded `cfn` list — a9s-devops: the `aws:cloudformation:*` tag set is applied automatically by CFN to every resource it creates, so this is the canonical cross-reference for IaC-provenance pivots.
- **Count shown**: unknown.

### `eni`

- **Why related**: ENI route targets (for example, a firewall/NVA appliance hop) — operator wants to inspect the target interface's state, SG, and owning instance.
- **How discovered**: read `Routes[].NetworkInterfaceId` on the route table; cross-reference the already-loaded `eni` list by `NetworkInterfaceId`.
- **Count shown**: unknown.

### `igw`

- **Why related**: Internet Gateway route targets — the default-route target for public subnets; operator pivots to confirm IGW attachment.
- **How discovered**: read `Routes[].GatewayId` on the route table, filter to values with the `igw-` prefix; cross-reference the already-loaded `igw` list by gateway ID.
- **Count shown**: unknown.

### `nat`

- **Why related**: NAT Gateway route targets — default-route target for private subnets; operator pivots to confirm the NAT is `available` and to inspect its EIP.
- **How discovered**: read `Routes[].NatGatewayId` on the route table; cross-reference the already-loaded `nat` list by NAT gateway ID.
- **Count shown**: unknown.

### `subnet`

- **Why related**: Explicitly-associated subnets — which subnets actually use this RTB (and, for the VPC main RTB, which subnets implicitly fall back to it).
- **How discovered**: read `Associations[].SubnetId` on the route table; cross-reference the already-loaded `subnet` list by subnet ID. (For the main RTB, `Associations[].Main==true` covers every subnet in the VPC that has no explicit association — the reverse join is done on the `subnet` side.)
- **Count shown**: unknown.

### `tgw`

- **Why related**: Transit Gateway route targets — inter-VPC/on-prem hops; operator pivots to confirm TGW attachment and routes.
- **How discovered**: read `Routes[].TransitGatewayId` on the route table; cross-reference the already-loaded `tgw` list by transit gateway ID.
- **Count shown**: unknown.

### `vpc`

- **Why related**: Parent VPC — the route table belongs to exactly one VPC; the operator pivots for VPC-level context (CIDR, flow logs, siblings).
- **How discovered**: read `VpcId` on the route table; cross-reference the already-loaded `vpc` list by VPC ID.
- **Count shown**: unknown.

### `vpce`

- **Why related**: Gateway-endpoint routes (S3 and DynamoDB gateway endpoints) attach to route tables by adding prefix-list routes; operator pivots to confirm endpoint state and policy.
- **How discovered**: read `Routes[].GatewayId` on the route table, filter to values with the `vpce-` prefix (gateway endpoints appear as a gateway target on the route); cross-reference the already-loaded `vpce` list by endpoint ID.
- **Count shown**: unknown.

### `ct-events`

- Universal pivot — applies to every registered type; see `docs/related-resources.md` §Policy.
- **Why related**: Audit trail for route changes (CreateRoute, DeleteRoute, ReplaceRoute, AssociateRouteTable, etc.) — explains "who changed what, when" on this RTB.
- **How discovered**: call the CloudTrail `LookupEvents` API filtered by the route table ID.
- **Count shown**: unknown.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `Routes[].State == blackhole` — at least one route in the table has a dead target (gateway detached, NAT deleted, ENI gone, peering torn down).
  - **State bucket**: Broken.
  - **How obtained**: `DescribeRouteTables` response — inspect each `Route.State` on every `RouteTable.Routes[]`.

- **Signal**: `Associations[]` contains no entries AND the route table is not the VPC main RTB (`Associations[].Main != true`) — orphan route table that no subnet and no gateway uses.
  - **State bucket**: Warning.
  - **How obtained**: `DescribeRouteTables` response — inspect `RouteTable.Associations[]` (length, and `Main` flag on each association).

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

`docs/attention-signals.md` lists Wave 3 for `rtb` as `None`. There are no Wave 3 signals to copy.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause. **Healthy rows render blank.** |
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
| `Routes[].State == blackhole` | 1 | Broken | n/a | S2 + S4 | `blackhole route: target gone` | `One or more routes point at a target that no longer exists (gateway detached, NAT/ENI deleted).` |
| no associations AND not VPC main | 1 | Warning | n/a | S2 + S4 | `orphan: no subnet associations` | `No subnet or gateway uses this route table and it is not the VPC main table.` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`DORMANT`, `stopped`, `available`, `failed`) in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column. Tests will assert the cause is present.
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), you may omit the row from this table entirely; §3 still describes it.
- Keep both columns short enough to fit: List text ≤ 40 chars, Detail text ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — a red row with `blackhole route: target gone` tells the on-call exactly which RTB has a dead hop, and a yellow row with `orphan: no subnet associations` flags an unused table for cleanup; both are self-explanatory in the list and operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (documented upstream as `None`; nothing to exclude here beyond the Wave-3 budget itself).
- Per-route propagation health, BGP state, or dynamic route churn — not available from `DescribeRouteTables` alone and would require VGW/Direct-Connect APIs.
- Route-target deep validation beyond `blackhole` state (e.g. IGW detached vs attached, NAT `failed` vs `available`) — those conditions surface on the target's own row (`igw`, `nat`) rather than being re-reported on the RTB.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`docs/architecture.md` §"What is a9s?").

## 6. Citations

- Per-type contract row for `rtb` — `docs/related-resources.md` § Per-type contract table, row `rtb` (targets `cfn`, `ct-events`, `eni`, `igw`, `nat`, `subnet`, `tgw`, `vpc`, `vpce`).
- Per-target reasoning for `rtb` — `docs/related-resources.md` § `rtb` subsection (one-line rationale per target).
- `ct-events` as a universal pivot — `docs/related-resources.md` § Policy (applies to every registered type).
- Wave 1 / Wave 2 / Wave 3 signals for `rtb` — `docs/attention-signals.md` § Networking table, row `rtb`.
- `RouteTable.Routes[]`, `RouteTable.Associations[]`, `RouteTable.VpcId`, `RouteTable.Tags` — `AWS SDK Go v2 — service/ec2/types.RouteTable § Routes, Associations, VpcId, Tags`.
- `Route.State == blackhole` semantics ("the route's target isn't available") — `AWS SDK Go v2 — service/ec2/types.Route § State` (godoc comment) and `AWS SDK Go v2 — service/ec2/types.RouteState` enum (`active`, `blackhole`, `filtered`).
- `Route.GatewayId`, `Route.NatGatewayId`, `Route.TransitGatewayId`, `Route.NetworkInterfaceId` as target fields for `igw`/`nat`/`tgw`/`eni`/`vpce` discovery — `AWS SDK Go v2 — service/ec2/types.Route § GatewayId, NatGatewayId, TransitGatewayId, NetworkInterfaceId`.
- `RouteTableAssociation.SubnetId` and `RouteTableAssociation.Main` as the subnet-pivot and main-RTB flag — `AWS SDK Go v2 — service/ec2/types.RouteTableAssociation § SubnetId, Main`.
- `cfn` discovery via `aws:cloudformation:stack-name` tag — a9s-devops (2026-04-20): possible=yes, worth=yes. CloudFormation automatically stamps this tag on every managed resource; it is the canonical IaC-provenance pivot used throughout a9s and matches the pattern called out explicitly for `secrets` in `docs/related-resources.md` (`SecretListEntry.Tags["aws:cloudformation:stack-name"]`).
- `vpce` discovered via `Routes[].GatewayId` with `vpce-` prefix (gateway endpoints, not interface endpoints) — a9s-devops (2026-04-20): possible=yes, worth=yes. S3 and DynamoDB gateway endpoints install themselves as a route whose target is the `vpce-*` gateway ID; interface endpoints attach via ENI/DNS rather than a route and pivot from elsewhere.
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
- Count-shown values left `unknown` — `docs/related-resources.md` and `docs/enrichment-visibility.md` do not specify per-target count visibility for `rtb`; HOW decision deferred to a per-resource UX review rather than invented here.
