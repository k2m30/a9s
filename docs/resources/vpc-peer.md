---
shortName: vpc-peer
name: VPC Peering
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_VpcPeeringConnection.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# vpc-peer — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `vpc-peer` (aliases: `pcx`, `peering`)
- **Display name**: VPC Peering
- **AWS API reference**: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_VpcPeeringConnection.html>
- **List API**: `DescribeVpcPeeringConnections` (paginated; full detail in one call — `Status`, `ExpirationTime`, both `VpcInfo` sides)
- **Describe API (if any)**: not used — no per-connection describe exists.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `rtb`, `vpc`, `ct-events`.

### `rtb`

- **Why related**: "who actually routes to this peer" — a peering connection without a route is dead weight; the route tables ARE the traffic path.
- **How discovered**: cross-reference the already-loaded `rtb` list for `Routes[].VpcPeeringConnectionId == <pcx-id>`. Zero extra API calls. Unknown (`?`) when the rtb cache is absent; a truncated cache renders the count as an honest lower bound (`N+`, the fleet cache-scan convention shared with lt/ng) — never a fake exact 0.
- **Count shown**: yes.

### `vpc`

- **Why related**: the local end of the tunnel — reachability debugging starts at your own VPC.
- **How discovered**: symmetric CACHE-MEMBERSHIP GATE — pivot for whichever side's `VpcId` (`RequesterVpcInfo`/`AccepterVpcInfo`) is present in the loaded local `vpc` cache. A cross-account remote side is never in the cache → rendered as a plain fact (`VpcId + OwnerId + Region`), not a pivot. Never hardcode requester-is-local.
- **Count shown**: yes.

### `ct-events`

- **Why related**: audit trail — Create/Accept/Reject/Delete/ModifyVpcPeeringConnection*; "who accepted this" is the first cross-account incident question. Universal pivot — applies to every registered type; see docs/related-resources.md §Policy.
- **How discovered**: CloudTrail LookupEvents by VpcPeeringConnectionId.
- **Count shown**: yes.

Explicitly excluded: `sg` — `docs/related-resources.md` § `vpc-peer`.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeVpcPeeringConnections](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeVpcPeeringConnections.html)

Transcribed from `docs/attention-signals.md § Signals § NETWORKING` row `vpc-peer`.

Load-bearing SDK fact (doc comment on `Requester/AccepterVpcInfo`): "CIDR block information is only returned when describing an active VPC peering connection" — `CidrBlock`/`CidrBlockSet` are nil for every non-active state. Nil-safe rendering mandatory; the overlap check is active-only.

### 3.1 Wave 1 — zero extra API calls

- `PeeringOptions.AllowDnsResolutionFromRemoteVpc` per side = config fact. ClassicLink fields are SDK-deprecated — ignored.

- Cross-account / cross-region are FACTS (neutral detail badges), never signals — all seven live-witnessed rows are cross-account.

- **Signal**: `Status.Code == deleting` → Warning; `deleted` → Dim (AWS keeps it listed for a window).

- **Signal**: `Status.Code == active` → Healthy.

- **Signal**: provisioning.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `Status.Code` in `provisioning` / `initiating-request` (transient).
  - **State bucket**: Warning. — **How obtained**: `VpcPeeringConnection.Status.Code`.

- **Signal**: `Status.Code == pending-acceptance` — the other side has not accepted; AWS expires the request at 7 days. The countdown to `ExpirationTime` is the actionable bit.
  - **State bucket**: Warning. — **How obtained**: `Status.Code` + `ExpirationTime`.

- **Signal**: `Status.Code == expired` — the request died unaccepted.
  - **State bucket**: Warning.
  - **How obtained**: `VpcPeeringConnection.Status.Code`.

- **Signal**: `Status.Code == rejected` — the peer said no; `Status.Message` carries the cause verbatim.
  - **State bucket**: Broken.
  - **How obtained**: `VpcPeeringConnection.Status.Code` and `Status.Message`.

- **Signal**: `Status.Code == failed` — `Status.Message` verbatim.
  - **State bucket**: Broken.

- **Signal**: deleting.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: deleted.
  - **State bucket**: Dim.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: active-only IPv4 CIDR overlap — `RequesterVpcInfo.CidrBlockSet` × `AccepterVpcInfo.CidrBlockSet` intersection (≈15 lines of prefix math). Overlapping ranges silently blackhole traffic subsets.
  - **State bucket**: Warning. Ranked below missing-route.

### 3.2 Wave 2 — bounded extra API calls

No API-calling Wave 2 exists (no per-connection describe). Two derived zero-API cache-scan signals (the cache-scan enricher layer — snapshot_cross_ref.go precedent):

- **Signal**: active pcx with NO loaded-`rtb` route referencing it → "no local route to peer" (an accepted tunnel nobody routes into). Guards: rtb cache present AND not truncated — otherwise no signal, never a guess.
  - **State bucket**: Warning. — **Cost shape**: zero API calls (cache scan).

- **Signal**: a route references the pcx but `Route.State == blackhole` → "route to peer blackholed" (the other side tore down; your route now eats packets).
  - **State bucket**: Warning. — **Cost shape**: zero API calls (cache scan).

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: SG cross-referencing.
- OUT OF SCOPE: IPv6 CIDR overlap.
- OUT OF SCOPE: DNS effective-state / ClassicLink (SDK-deprecated).
- OUT OF SCOPE: flow-logs traffic verification.
- OUT OF SCOPE: remote-side health (invisible by construction — the API shows one side).
- OUT OF SCOPE: TGW-migration advice.

## 4. Issue Visualization

Surfaces S1–S5 per `docs/attention-signals.md § Visualization Surfaces`; wave→surface mapping as standard. Every signal is color-bearing — the fleet color invariant ("color derives from findings", the conformance-gate owner rule) applies to the two cache-scan route checks too: their rows render Warning-colored like any other finding. Their `~` class affects only the S1 aggregation (no badge bump) and S5 ordering.

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| provisioning | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `provisioning` |
| initiating-request | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `initiating` |
| pending-acceptance | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `pending acceptance: expires in <N>d` |
| expired | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `expired: never accepted` |
| rejected | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `rejected` |
| failed | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `failed` |
| deleting | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `deleting` |
| deleted | 1 | Dim | n/a | S2, S4 | `deleted` |
| CIDR overlap (active) | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `CIDR overlap with peer` |
| no local route (active) | 2 | Warning | `~` | S2, S3, S4, S5 | `no local route to peer` |
| route blackholed | 2 | Warning | `~` | S2, S3, S4, S5 | `route to peer blackholed` |

Notes:

- No raw AWS enum reaches a rendered surface.
- Multiple findings stack with the framework `(+N)` suffix; S5 enumerates each.
- pending-acceptance escalation: wording carries the countdown; under 48h the S4 phrase stays the same with the smaller `<N>` — the number IS the escalation.
- Both derived route checks ship as `~`-class background checks: Warning-colored rows per the fleet color invariant, excluded from the S1 badge (the badge counts state rows and `!`-class checks) — the color plus the S4 phrase carry the cause. Owner ruling 2026-07-15: no glyph-on-green special path exists; color derives from findings uniformly.

## 4.1 UX review (two sentences)

Every problem row names its cause in the Status column (`pending acceptance: expires in 3d`, `rejected`, `no local route to peer`), so the 3am operator triages without opening detail. The rtb pivot answers "who routes into this tunnel" one keypress away, and the remote side renders as plain OwnerId/Region facts — a9s never pretends to see the other account.

## 5. Out of Scope

- All §3.3 Wave 3 items.
- `sg` pivot (§2 exclusion with citation).
- Any UI element not in S1–S5; any write operation (`architecture.md` §"What is a9s?").

## 6. Citations

- Pivot set and exclusions — `docs/related-resources.md` § `vpc-peer`.
- CIDR active-only fact — `AWS SDK Go v2 — ec2/types.VpcPeeringConnection § RequesterVpcInfo/AccepterVpcInfo` (doc comment verbatim).
- State machine + ExpirationTime — `AWS SDK Go v2 — ec2/types.VpcPeeringConnection § Status, § ExpirationTime`; `docs/attention-signals.md § Signals § NETWORKING` row `vpc-peer`.
- rtb pivot + missing-route/blackhole signals — `a9s-devops (2026-07-14): possible=yes (Routes[].VpcPeeringConnectionId, Route.State), worth=yes — the pivot that makes the type worth adding.`
- Cross-account-as-facts — `a9s-devops (2026-07-14): all 7 live rows cross-account; flagging normality is alarm fatigue.` Live witness w_vpcpeer.json.
- Cache-scan enricher layer — snapshot_cross_ref.go / EnrichLTDeprecatedAMI precedent (fleet mechanism since v3.50.x).
- `~` vs Warning split for missing-route vs blackholed — spec decision 2026-07-15 mirroring the lt deprecated-AMI treatment (background concern vs active breakage).
- Read-only invariant — `docs/architecture.md` § "What is a9s?".

<!-- BEGIN GENERATED: header -->
vpc-peer — NETWORKING. Status key: `status` — the key the status cell reads, and the column naming it is the status column.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| vpc-peer.warn.provisioning | provisioning | warn | wave1 | Peering connection is being provisioned. |
| vpc-peer.warn.initiating | initiating | warn | wave1 | Peering request is being initiated. |
| vpc-peer.warn.pending\_acceptance | pending acceptance: expires in <N>d | warn | wave1 | The peer has not accepted this request yet, and AWS expires it a week after creation; the countdown is in the status and the date is listed below. Ask the accepter to approve it. |
| vpc-peer.warn.expired | expired: never accepted | warn | wave1 | The peering request expired unaccepted; recreate it if still needed. |
| vpc-peer.broken.rejected | rejected | broken | wave1 | The accepter rejected this peering request, so nothing will ever route across it; AWS keeps the record listed for a while. Delete it and request again once the other side agrees. |
| vpc-peer.broken.failed | failed | broken | wave1 | The peering connection failed to establish and will not recover on its own; the status message is listed below. Delete it and request a new one. |
| vpc-peer.warn.deleting | deleting | warn | wave1 | Peering connection is being deleted. |
| vpc-peer.dim.deleted | deleted | dim | wave1 | AWS keeps deleted connections listed for a window. |
| vpc-peer.warn.cidr\_overlap | CIDR overlap with peer | warn | wave1 | The requester and accepter VPCs have overlapping address ranges, so routes into the overlap are blackholed; the range is listed below. Re-address one side, or peer a VPC that does not overlap. |
| vpc-peer.warn.no\_local\_route | no local route to peer | warn | wave2 | No loaded route table routes to this peering connection. |
| vpc-peer.warn.route\_blackholed | route to peer blackholed | warn | wave2 | A route references this connection but its state is blackhole. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| rtb | Route Tables | yes |
| vpc | VPC | yes |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
