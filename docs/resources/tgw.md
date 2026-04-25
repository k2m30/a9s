---
shortName: tgw
name: Transit Gateways
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_TransitGateway.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
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

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `State == available` → Healthy.
  - **State bucket**: Healthy.
  - **How obtained**: `TransitGateway.State` on the `DescribeTransitGateways` list response (`AWS SDK Go v2 — ec2/types.TransitGateway § State`).
- **Signal**: `State` in `pending` / `modifying` / `deleting` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `TransitGateway.State` on the list response.
- **Signal**: `State == deleted` → Dim.
  - **State bucket**: Dim.
  - **How obtained**: `TransitGateway.State` on the list response.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: Any attachment `State` in `failed` / `failing` / `rejected` / `rejecting` → Broken.
  - **State bucket**: Broken.
  - **API call**: `DescribeTransitGatewayAttachments` — one per TGW, filtered by `transit-gateway-id`.
  - **Cost shape**: per-resource.
- **Signal**: Any attachment `State == pendingAcceptance` with age >24h → Warning.
  - **State bucket**: Warning.
  - **API call**: same `DescribeTransitGatewayAttachments` call; combine `State` with `CreationTime` (`AWS SDK Go v2 — ec2/types.TransitGatewayAttachment § State, CreationTime`).
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `PacketDropCountBlackhole` / `PacketDropCountNoRoute`.

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
| `State==pending` | 1 | Warning | n/a | S2, S4 | `pending: provisioning` | `TGW is still provisioning — wait for state to reach available.` |
| `State==modifying` | 1 | Warning | n/a | S2, S4 | `modifying: config change` | `TGW configuration change in progress — attachments may flap briefly.` |
| `State==deleting` | 1 | Warning | n/a | S2, S4 | `deleting` | `TGW is being deleted — attachments are being torn down.` |
| `State==deleted` | 1 | Dim | n/a | S2, S4 | `deleted` | `TGW has been deleted; record will age out shortly.` |
| attachment `State==failed`/`failing` | 2 | Broken | `!` | S1, S4, S5 (S3 suppressed on red row) | `attachment failed` | `One or more TGW attachments failed — check VPC, Direct Connect, or peer status.` |
| attachment `State==rejected`/`rejecting` | 2 | Broken | `!` | S1, S4, S5 (S3 suppressed on red row) | `attachment rejected` | `Cross-account attachment request was rejected by the accepter account.` |
| attachment `State==pendingAcceptance` >24h | 2 | Warning | `~` | S3, S4, S5 | `attachment awaiting accept` | `Cross-account VPC attachment request pending acceptance for more than 24h.` |

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for Wave 1 (color + cause word covers `pending`/`modifying`/`deleting`/`deleted`). For Wave 2, the list text `attachment failed` / `attachment rejected` / `attachment awaiting accept` names what to chase in one glance; the operator still presses detail only to find out *which* attachment — that trade (list stays narrow, detail carries the IDs) is acceptable because a TGW typically has few attachments and the next click is always "show me the attachments list".

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- `DescribeTransitGateways` as list API — `docs/attention-signals.md` § Networking → `tgw` row § Source column points to `DescribeTransitGatewayAttachments` (Wave 2); list API is the standard `DescribeTransitGateways` per SDK (`AWS SDK Go v2 — ec2 § DescribeTransitGateways`).
- `TransitGateway.State` enum values — `AWS SDK Go v2 — ec2/types.TransitGatewayState` § `pending, available, modifying, deleting, deleted`.
- Wave 1 state buckets — `docs/attention-signals.md` § Networking → `tgw` Wave 1 cell.
- Wave 2 attachment-state signals — `docs/attention-signals.md` § Networking → `tgw` Wave 2 cell, and `AWS SDK Go v2 — ec2/types.TransitGatewayAttachmentState` § `failed, failing, rejected, rejecting, pendingAcceptance`.
- Wave 3 exclusion list — `docs/attention-signals.md` § Networking → `tgw` Wave 3 cell.
- Related targets `ct-events, role, rtb, subnet, vpc` — `docs/related-resources.md` § Per-type contract table, row `tgw`, and § `tgw` subsection.
- `vpc` discovery via `DescribeTransitGatewayVpcAttachments.VpcId` — `AWS SDK Go v2 — ec2/types.TransitGatewayVpcAttachment § VpcId`. a9s-devops (2026-04-20): possible=yes, worth=yes. Operators follow TGW → attached VPCs constantly during connectivity debugging; the VPC-attachment API returns the IDs in one call.
- `subnet` discovery via `TransitGatewayVpcAttachment.SubnetIds` — `AWS SDK Go v2 — ec2/types.TransitGatewayVpcAttachment § SubnetIds`. a9s-devops (2026-04-20): possible=yes, worth=yes. TGW subnets pin which AZs are reachable; operators need this when cross-AZ traffic misbehaves.
- `rtb` discovery via sibling-list cross-reference on `Routes[].TransitGatewayId` — a9s-devops (2026-04-20): possible=yes, worth=yes. Zero extra API calls (sibling list already loaded); "which subnets actually route through this TGW?" is the primary daily question, and route-table records carry the target-ID fields.
- `role` discovery mechanism — a9s-devops (2026-04-20): possible=no, worth=yes (in mesh-TGW multi-account setups, but no AWS surface exposes the accepter role directly on the TGW record). TBD — recorded as `Count shown: unknown` in §2; the golden doc's "Cross-account RAM share roles" text describes intent rather than a field path.
- `ct-events` universal pivot — `docs/related-resources.md` § Policy (universal ct-events entry applies to every registered type).
- S1–S5 surface mechanics and Wave→surface mapping — a9s-resource-spec skill SKILL.md § "Allowed visualization surfaces (exactly five)" and § "Mapping rules".
- Read-only invariant (Out of Scope write operations) — `docs/architecture.md` § "What is a9s?".
