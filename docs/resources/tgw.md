---
shortName: tgw
name: Transit Gateways
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_TransitGateway.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# tgw — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `tgw`
- **Display name**: Transit Gateways
- **AWS API reference**: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_TransitGateway.html>
- **List API**: `DescribeTransitGateways`
- **Describe API (if any)**: `DescribeTransitGatewayAttachments` (Wave 2 — one call per TGW, filtered by `transit-gateway-id`)

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `ct-events`, `role`, `rtb`, `subnet`, `vpc`.

### `vpc`

- **Why related**: VPCs attached to this TGW — the primary operator question when tracing connectivity: "which VPCs can talk through this gateway?" (`related-resources.md` § `tgw`).
- **How discovered**: call `DescribeTransitGatewayVpcAttachments` filtered by `transit-gateway-id`, then read `TransitGatewayVpcAttachment.VpcId` for each attachment — a9s-devops: the VPC-attachment API returns `VpcId` directly, no further hop needed.
- **Count shown**: yes.

### `subnet`

- **Why related**: The specific subnets carrying the TGW ENI per AZ; when cross-AZ traffic misbehaves, the operator needs to see which AZs the TGW is actually anchored in (`related-resources.md` § `tgw`).
- **How discovered**: same `DescribeTransitGatewayVpcAttachments` response — read `TransitGatewayVpcAttachment.SubnetIds[]` across attachments (`AWS SDK Go v2 — ec2/types.TransitGatewayVpcAttachment § SubnetIds`).
- **Count shown**: yes.

### `rtb`

- **Why related**: VPC route tables that direct traffic into this TGW — answers "which subnets actually send traffic through here?" (`related-resources.md` § `tgw`).
- **How discovered**: cross-reference the already-loaded `rtb` list client-side; match `Routes[].TransitGatewayId == this.TransitGatewayId`. Zero extra AWS calls — a9s-devops: this is the standard same-sweep sibling-list pivot used elsewhere (e.g. `subnet` ↔ `rtb`), and rtb's list response does include `Routes[]` with the target-ID fields populated.
- **Count shown**: yes.

### `role`

- **Why related**: Cross-account RAM-share IAM roles associated with this TGW in multi-account network hubs (`related-resources.md` § `tgw`).
- **How discovered**: TBD — a9s-devops: not available cleanly on the AWS surface. The `TransitGateway` response carries no `Role` ARN. RAM resource shares are reachable via `GetResourceShares` / `ListResources`, but those reference managed policies, not IAM roles. Discovering "the role used to accept a cross-account attachment" requires correlating CloudTrail `AcceptTransitGatewayVpcAttachment` events with principal ARNs — Wave 3 territory.
- **Count shown**: unknown.

### `ct-events`

- **Why related**: Audit trail for TGW attachment changes, RAM shares, and route-table edits — universal pivot applies to every registered type; see related-resources.md §Policy.
- **How discovered**: universal pivot — `LookupEvents` filtered by TGW ID.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeTransitGatewayAttachments](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeTransitGatewayAttachments.html)

Transcribed from `docs/attention-signals.md § Signals § NETWORKING` row `tgw`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `State` in `pending` / `modifying` / `deleting` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `TransitGateway.State` on the list response.

- **Signal**: `State==modifying`.
  - **State bucket**: Warning.
  - **How obtained**: `TransitGateway.State` on the list response.

- **Signal**: `State==deleting`.
  - **State bucket**: Warning.
  - **How obtained**: `TransitGateway.State` on the list response.

- **Signal**: `State == deleted` → Dim.
  - **State bucket**: Dim.
  - **How obtained**: `TransitGateway.State` on the list response.

- **Signal**: `Options.AutoAcceptSharedAttachments == enable` (not on a deleting/deleted gateway).
  - **State bucket**: Warning.
  - **How obtained**: `TransitGateway.State` on the list response.

- **Signal**: `State == failed`.
  - **State bucket**: Broken.
  - **How obtained**: `TransitGateway.State` on the list response.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: Any attachment `State` in `failed` / `failing` / `rejected` / `rejecting` → Broken.
  - **State bucket**: Broken.
  - **API call**: `DescribeTransitGatewayAttachments` — one per TGW, filtered by `transit-gateway-id`.
  - **Cost shape**: per-resource.

- **Signal**: attachment `State==rejected`/`rejecting`.
  - **State bucket**: Broken.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: Any attachment `State == pendingAcceptance` with age >24h → Warning.
  - **State bucket**: Warning.
  - **API call**: same `DescribeTransitGatewayAttachments` call; combine `State` with `CreationTime` (`AWS SDK Go v2 — ec2/types.TransitGatewayAttachment § State, CreationTime`).
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `PacketDropCountBlackhole` / `PacketDropCountNoRoute`.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `tgw`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

Lifecycle findings render their phrase only — the state IS the whole fact, and a Detail
sentence would restate it. Their S5 cell reads `—`.

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `State==pending` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `pending` |
| `State==modifying` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `modifying` |
| `State==deleting` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `deleting` |
| `State==deleted` | 1 | Dim | n/a | S2, S4 | `deleted` |
| `Options.AutoAcceptSharedAttachments == enable` (not on a deleting/deleted gateway) | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `auto-accepts shared attachments` |
| `State == failed` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `failed` |
| attachment `State==failed`/`failing` | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `attachment failed` |
| attachment `State==rejected`/`rejecting` | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `attachment failed` |
| attachment `State==pendingAcceptance` >24h | 2 | Warning | `~` | S2, S3, S4, S5 | `attachment between states` |

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for Wave 1 (color + cause word covers `pending`/`modifying`/`deleting`/`deleted`). For Wave 2, the list text `attachment failed` / `attachment rejected` / `attachment awaiting accept` names what to chase in one glance; the operator still presses detail only to find out *which* attachment — that trade (list stays narrow, detail carries the IDs) is acceptable because a TGW typically has few attachments and the next click is always "show me the attachments list". The `auto-accepts shared attachments` row is Wave 1 and needs no detail at all — the phrase is the whole finding.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- `DescribeTransitGateways` as list API — `AWS SDK Go v2 — ec2 § DescribeTransitGateways`.
- `TransitGateway.State` enum values — `AWS SDK Go v2 — ec2/types.TransitGatewayState` § `pending, available, modifying, deleting, deleted`.
- Wave 1 state buckets — `docs/attention-signals.md § Signals § NETWORKING` row `tgw`.
- Wave 2 attachment-state signals — `docs/attention-signals.md § Signals § NETWORKING` row `tgw`, and `AWS SDK Go v2 — ec2/types.TransitGatewayAttachmentState` § `failed, failing, rejected, rejecting, pendingAcceptance`.
- Wave 3 exclusion list — `docs/attention-signals.md § Not yet implemented`.
- Related targets `ct-events, role, rtb, subnet, vpc` — `docs/related-resources.md` § Per-type contract table, row `tgw`, and § `tgw` subsection.
- `vpc` discovery via `DescribeTransitGatewayVpcAttachments.VpcId` — `AWS SDK Go v2 — ec2/types.TransitGatewayVpcAttachment § VpcId`. a9s-devops (2026-04-20): possible=yes, worth=yes. Operators follow TGW → attached VPCs constantly during connectivity debugging; the VPC-attachment API returns the IDs in one call.
- `subnet` discovery via `TransitGatewayVpcAttachment.SubnetIds` — `AWS SDK Go v2 — ec2/types.TransitGatewayVpcAttachment § SubnetIds`. a9s-devops (2026-04-20): possible=yes, worth=yes. TGW subnets pin which AZs are reachable; operators need this when cross-AZ traffic misbehaves.
- `rtb` discovery via sibling-list cross-reference on `Routes[].TransitGatewayId` — a9s-devops (2026-04-20): possible=yes, worth=yes. Zero extra API calls (sibling list already loaded); "which subnets actually route through this TGW?" is the primary daily question, and route-table records carry the target-ID fields.
- `role` discovery mechanism — a9s-devops (2026-04-20): possible=no, worth=yes (in mesh-TGW multi-account setups, but no AWS surface exposes the accepter role directly on the TGW record). TBD — recorded as `Count shown: unknown` in §2; the golden doc's "Cross-account RAM share roles" text describes intent rather than a field path.
- `ct-events` universal pivot — `docs/related-resources.md` § Policy (universal ct-events entry applies to every registered type).
- S1–S5 surface mechanics and Wave→surface mapping — a9s-resource-spec skill SKILL.md § "Allowed visualization surfaces (exactly five)" and § "Mapping rules".
- Read-only invariant (Out of Scope write operations) — `docs/architecture.md` § "What is a9s?".

<!-- BEGIN GENERATED: header -->
tgw — NETWORKING. Lifecycle key: `state`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| tgw.state.pending | pending | warn | wave1 | The gateway is still being created and does not route yet. Attachments created now stay pending until it comes up. |
| tgw.state.modifying | modifying | warn | wave1 | A configuration change is being applied. Routing across the gateway can be inconsistent until it settles. |
| tgw.state.deleting | deleting | warn | wave1 | The gateway is being torn down. Every attachment on it goes away and any traffic still routed through it will stop. |
| tgw.state.failed | failed | broken | wave1 | The gateway could not be created and will not recover. It has to be recreated, and anything routed through it has no path. |
| tgw.state.deleted | deleted | dim | wave1 | This gateway is gone. AWS keeps returning it for a while after deletion, so route tables that still point at it are dead references worth cleaning up. |
| tgw.attachment-failed | attachment failed | broken | wave2 | The network behind this attachment has no path across the gateway. Failed attachments do not retry; delete and recreate the attachment. |
| tgw.attachment-transitional | attachment between states | warn | wave2 | The attachment is between states — being modified, rolled back, or waiting for the gateway owner to accept it — and traffic across it is not reliable until it settles. Pending acceptance is the one state that needs a person: the owning account has to approve it. |
| tgw.auto-accept-attachments | auto-accepts shared attachments | warn | wave1 | Any account this gateway is shared with can attach a VPC to it without review, putting that VPC on your routed network the moment it asks. Turn auto-accept off and approve each attachment explicitly. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| vpc | VPCs | no |
| rtb | Route Tables | yes |
| role | IAM Role | no |
| subnet | Subnets | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
