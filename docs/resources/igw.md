---
shortName: igw
name: Internet Gateways
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InternetGateway.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# igw — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `igw`
- **Display name**: Internet Gateways
- **AWS API reference**: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_InternetGateway.html>
- **List API**: `DescribeInternetGateways` (returns the full `InternetGateway` shape per gateway, including `InternetGatewayId`, `OwnerId`, `Attachments[]` and `Tags[]`).
- **Describe API (if any)**: not used — list response already carries everything the Wave 1 signals need.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `rtb`, `vpc`, `ct-events`.

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
- **How discovered**: Call CloudTrail `LookupEvents` filtered by `ResourceName == InternetGatewayId` (and/or event-name filter). Universal pivot — applies to every registered type; see `docs/related-resources.md` § Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeInternetGateways](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInternetGateways.html)

Transcribed from `docs/attention-signals.md § Signals § NETWORKING` row `igw`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `Attachments[].State == attaching` or `detaching` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `Attachments[0].State` on the list-response IGW.

- **Signal**: `Attachments[0].State == detaching`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `len(Attachments) == 0` → Warning (orphan — never attached or fully detached).
  - **State bucket**: Warning.
  - **How obtained**: size of the `Attachments[]` slice on the list-response IGW.

- **Signal**: `Attachments[].State == detached` → Warning (orphan). — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06)
  - **State bucket**: Warning.
  - **How obtained**: `Attachments[0].State` on the list-response IGW.

- **Signal**: IGW attached to a VPC but no route table in that VPC has a `0.0.0.0/0 → igw` route → Warning (unused — operator is paying for a gateway nothing routes through). — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06)
  - **State bucket**: Warning.
  - **How obtained**: Take this IGW's `Attachments[0].VpcId`; cross-reference the already-loaded `rtb` list and filter to route tables whose `VpcId` equals that value; scan their `Routes[]` for any route where `DestinationCidrBlock == "0.0.0.0/0"` and `GatewayId == InternetGatewayId`. If none match, raise the signal.

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: None.

(Attention-signals.md lists `None` for Wave 3 on this row; there are no CloudWatch metrics or deep-probe checks planned for internet gateways.)

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `igw`: Wave 1 issue-colored rows only — this type registers no Wave 2 enricher, so nothing else bumps the count.
<!-- END GENERATED: badge -->

One row per §3 signal (Healthy case omitted per rule):

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `Attachments[0].State == attaching` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `attaching` |
| `Attachments[0].State == detaching` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `detaching` |
| `len(Attachments) == 0` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `no VPC attachments` |
| `Attachments[0].State == detached` — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | 1 | Warning | n/a | S2, S4 | `detached: orphan gateway` |
| IGW attached but VPC has no `0.0.0.0/0 → igw` route — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | 1 | Warning | n/a | S2, S4 | `attached but unused: no default route` |

## 4.1 UX review (two sentences)

At 3am, glancing at the list, the operator can distinguish the Warning modes by the Status column: `attaching` / `detaching` reads as in-flight, and `no VPC attachments` reads as orphan billing. All problem rows are self-explanatory in the list — operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (none declared for this resource).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` § "What is a9s?").
- Egress-only internet gateways (separate AWS resource type, not `igw`).
- CloudWatch metrics per IGW — internet gateways emit no standalone CloudWatch namespace; operator-visible throughput and drop signals live on the attached `nat` / `vpc` / flow-log surfaces, not here. — a9s-devops: possible=no (no `AWS/EC2` dimension for IGWs), worth=no.

## 6. Citations

- `shortName`, display name and the `igw` signals — `docs/attention-signals.md § Signals § NETWORKING` row `igw`; list API — `core/aws/igw.go`.
- AWS API reference URL, related targets list — `docs/related-resources.md` § Per-type contract row for `igw` and § `igw` narrative block.
- Read-only invariant — `docs/architecture.md` § "What is a9s?" (lines 13–15).
- `Attachments[]`, `Attachments[].State`, `Attachments[].VpcId`, `InternetGatewayId` field names — `AWS SDK Go v2 — service/ec2/types.InternetGateway § Attachments` and `service/ec2/types.InternetGatewayAttachment § State, VpcId`.
- `AttachmentStatus` enum values (`attaching`, `attached`, `detaching`, `detached`) — `AWS SDK Go v2 — service/ec2/types.AttachmentStatus`.
- `Routes[].GatewayId`, `Routes[].DestinationCidrBlock` for the `rtb` cross-reference — `AWS SDK Go v2 — service/ec2/types.Route § GatewayId, DestinationCidrBlock`.
- List API returns full gateway shape, no Describe needed — `AWS SDK Go v2 — service/ec2.DescribeInternetGatewaysOutput § InternetGateways`.
- `ct-events` as universal pivot — `docs/related-resources.md` § Policy.
- CloudTrail event-name filter (`AttachInternetGateway`, `DetachInternetGateway`, `CreateInternetGateway`, `DeleteInternetGateway`) — `a9s-devops (2026-04-20): possible=yes (CloudTrail records all IGW management-plane calls), worth=yes. These four event names are the filter operators run when investigating an IGW state change.`
- `rtb` discovery via reverse-scan of the already-loaded list — `a9s-devops (2026-04-20): possible=yes, worth=yes. The route target lives on the Route object as GatewayId; scanning the already-loaded rtb list avoids any extra AWS call.`
- `vpc` discovery via `Attachments[0].VpcId` — `a9s-devops (2026-04-20): possible=yes, worth=yes. IGW-to-VPC is a 1:1 relationship carried directly on the list response, no cross-scan needed.`
- CloudWatch metrics per IGW not available — `a9s-devops (2026-04-20): possible=no, worth=no. AWS does not publish an AWS/EC2 or AWS/VPC namespace for internet gateways; flow logs on the VPC are the closest substitute and are VPC-scoped, not gateway-scoped.`

<!-- BEGIN GENERATED: header -->
igw — NETWORKING. Status key: `state` — the key the status cell reads, and the column naming it is the status column.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| igw.state.attaching | attaching | warn | wave1 | The gateway is still being attached, so the VPC has no internet path through it yet. Wait for the attachment to complete before testing egress or public addressing. |
| igw.state.detaching | detaching | warn | wave1 | The gateway is being detached, and when it goes the VPC loses its internet path: public instances stop being reachable and outbound calls fail. Stop the detachment if anything still depends on it. |
| igw.no-attachments | no VPC attachments | warn | wave1 | This gateway belongs to no VPC, so it routes nothing. Attach it to the VPC it was created for, or delete it so it stops appearing as available infrastructure. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| vpc | VPCs | no |
| rtb | Route Tables | yes |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
