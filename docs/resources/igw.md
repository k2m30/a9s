---
shortName: igw
name: Internet Gateways
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InternetGateway.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# igw — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `igw`
- **Display name**: Internet Gateways
- **AWS API reference**: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InternetGateway.html
- **List API**: `DescribeInternetGateways` (returns the full `InternetGateway` shape per gateway, including `InternetGatewayId`, `OwnerId`, `Attachments[]` and `Tags[]`).
- **Describe API (if any)**: not used — list response already carries everything the Wave 1 signals need.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `rtb`, `vpc`, `ct-events`.

### `rtb`

- **Why related**: Route tables reveal whether this IGW is actually carrying internet traffic — an IGW with no route table pointing `0.0.0.0/0` at it is paid-for, attached, and unused. This is also how the operator answers "which subnets go to the internet through this gateway?".
- **How discovered**: Reverse-scan the already-loaded `rtb` list — walk each route table's `Routes[]` and match any route whose `GatewayId` equals this gateway's `InternetGatewayId`. No extra AWS call needed. — a9s-devops: the route target lives on the `Route` object as `GatewayId`; reverse-scan against the already-loaded `rtb` list is the cheapest approach. Possible=yes, worth=yes.
- **Count shown**: yes.

### `vpc`

- **Why related**: The VPC this gateway is attached to — operator's first pivot when they see an IGW is "which network does this actually belong to?".
- **How discovered**: Read `Attachments[0].VpcId` on this IGW and open the matching entry in the already-loaded `vpc` list. A detached IGW (`len(Attachments)==0`) has no VPC pivot and the panel shows the target as empty. — a9s-devops: IGW→VPC is a 1:1 relationship carried directly on the IGW response. Possible=yes, worth=yes.
- **Count shown**: yes (0 or 1 — an IGW attaches to at most one VPC).

### `ct-events`

- **Why related**: Audit trail for attach/detach events and tag changes — universal pivot for "who changed this, and when?". Typical CloudTrail event names to filter on: `AttachInternetGateway`, `DetachInternetGateway`, `CreateInternetGateway`, `DeleteInternetGateway`.
- **How discovered**: Call CloudTrail `LookupEvents` filtered by `ResourceName == InternetGatewayId` (and/or event-name filter). Universal pivot — applies to every registered type; see `related-resources.md` § Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `Attachments[].State == attached` → Healthy.
  - **State bucket**: Healthy.
  - **How obtained**: `Attachments[0].State` on the `InternetGateway` returned by `DescribeInternetGateways`.

- **Signal**: `Attachments[].State == attaching` or `detaching` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `Attachments[0].State` on the list-response IGW.

- **Signal**: `Attachments[].State == detached` → Warning (orphan).
  - **State bucket**: Warning.
  - **How obtained**: `Attachments[0].State` on the list-response IGW.

- **Signal**: `len(Attachments) == 0` → Warning (orphan — never attached or fully detached).
  - **State bucket**: Warning.
  - **How obtained**: size of the `Attachments[]` slice on the list-response IGW.

- **Signal**: IGW attached to a VPC but no route table in that VPC has a `0.0.0.0/0 → igw` route → Warning (unused — operator is paying for a gateway nothing routes through).
  - **State bucket**: Warning.
  - **How obtained**: Take this IGW's `Attachments[0].VpcId`; cross-reference the already-loaded `rtb` list and filter to route tables whose `VpcId` equals that value; scan their `Routes[]` for any route where `DestinationCidrBlock == "0.0.0.0/0"` and `GatewayId == InternetGatewayId`. If none match, raise the signal.

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: None.

(Attention-signals.md lists `None` for Wave 3 on this row; there are no CloudWatch metrics or deep-probe checks planned for internet gateways.)

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". Never appears on yellow/red/dim rows. |
| S4 | Status / description column text | Short human-readable cause (e.g. `detached`, `attached but unused`). Healthy rows render blank. |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping for this resource:

- **Wave 1 Healthy** (`attached`) → no §4 row. S2 renders green, S4 renders blank. Silence is the UX.
- **Wave 1 Warning** signals → S2 (yellow) + S4 (cause). No S1, S3, S5.
- No Wave 2 signals on this resource, so S1/S3/S5 are unused here.

One row per §3 signal (Healthy case omitted per rule):

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `Attachments[0].State == attaching` | 1 | Warning | n/a | S2, S4 | `attaching to VPC` | n/a (Wave 1 Warning has no S5) |
| `Attachments[0].State == detaching` | 1 | Warning | n/a | S2, S4 | `detaching from VPC` | n/a |
| `Attachments[0].State == detached` | 1 | Warning | n/a | S2, S4 | `detached: orphan gateway` | n/a |
| `len(Attachments) == 0` | 1 | Warning | n/a | S2, S4 | `unattached: no VPC` | n/a |
| IGW attached but VPC has no `0.0.0.0/0 → igw` route | 1 | Warning | n/a | S2, S4 | `attached but unused: no default route` | n/a |

## 4.1 UX review (two sentences)

At 3am, glancing at the list, the operator can distinguish the four Warning modes by the Status column: `attaching` / `detaching` reads as in-flight, `detached` / `unattached` reads as orphan billing, and `attached but unused: no default route` reads as misconfiguration. All problem rows are self-explanatory in the list — operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (none declared for this resource).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` § "What is a9s?").
- Egress-only internet gateways (separate AWS resource type, not `igw`).
- CloudWatch metrics per IGW — internet gateways emit no standalone CloudWatch namespace; operator-visible throughput and drop signals live on the attached `nat` / `vpc` / flow-log surfaces, not here. — a9s-devops: possible=no (no `AWS/EC2` dimension for IGWs), worth=no.

## 6. Citations

- `shortName`, display name, signal cells, list API — `docs/attention-signals.md` § Networking row for `igw` (line 55).
- AWS API reference URL, related targets list — `docs/related-resources.md` § Per-type contract row for `igw` (line 78) and § `igw` narrative block (lines 600–606).
- Read-only invariant — `docs/architecture.md` § "What is a9s?" (lines 13–15).
- `Attachments[]`, `Attachments[].State`, `Attachments[].VpcId`, `InternetGatewayId` field names — `AWS SDK Go v2 — service/ec2/types.InternetGateway § Attachments` and `service/ec2/types.InternetGatewayAttachment § State, VpcId`.
- `AttachmentStatus` enum values (`attaching`, `attached`, `detaching`, `detached`) — `AWS SDK Go v2 — service/ec2/types.AttachmentStatus`.
- `Routes[].GatewayId`, `Routes[].DestinationCidrBlock` for the `rtb` cross-reference — `AWS SDK Go v2 — service/ec2/types.Route § GatewayId, DestinationCidrBlock`.
- List API returns full gateway shape, no Describe needed — `AWS SDK Go v2 — service/ec2.DescribeInternetGatewaysOutput § InternetGateways`.
- `ct-events` as universal pivot — `docs/related-resources.md` § Policy (line 34).
- CloudTrail event-name filter (`AttachInternetGateway`, `DetachInternetGateway`, `CreateInternetGateway`, `DeleteInternetGateway`) — `a9s-devops (2026-04-20): possible=yes (CloudTrail records all IGW management-plane calls), worth=yes. These four event names are the filter operators run when investigating an IGW state change.`
- `rtb` discovery via reverse-scan of the already-loaded list — `a9s-devops (2026-04-20): possible=yes, worth=yes. The route target lives on the Route object as GatewayId; scanning the already-loaded rtb list avoids any extra AWS call.`
- `vpc` discovery via `Attachments[0].VpcId` — `a9s-devops (2026-04-20): possible=yes, worth=yes. IGW-to-VPC is a 1:1 relationship carried directly on the list response, no cross-scan needed.`
- CloudWatch metrics per IGW not available — `a9s-devops (2026-04-20): possible=no, worth=no. AWS does not publish an AWS/EC2 or AWS/VPC namespace for internet gateways; flow logs on the VPC are the closest substitute and are VPC-scoped, not gateway-scoped.`
