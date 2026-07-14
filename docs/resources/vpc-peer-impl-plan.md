# vpc-peer — Implementation Plan

Derived from [`docs/resources/vpc-peer.md`](vpc-peer.md). Spec has zero TBDs; two DECIDE-at-phase-7 flags are marked inline (§0 blackhole severity plumbing, §0 navigable set).

## 0. Architecture decisions

**Single-call fetcher — NO N+1.** DescribeVpcPeeringConnections returns full detail (Status, ExpirationTime, both VpcInfo sides) in the list call. internal/aws/vpcpeer.go: one paginated call (RetryOnThrottle, DefaultPageSize), RawStruct = the ec2types.VpcPeeringConnection (value/pointer per transfer convention), Resource.ID = VpcPeeringConnectionId. NO describe, NO details-denied path (no per-item call to be denied) — the ONLY type of the four without a degraded-row story; a DescribeVpcPeeringConnections denial is a whole-list error (menu shows the error state, never 0).

**Nil-safe CIDR (load-bearing SDK fact).** CidrBlock/CidrBlockSet on Requester/AccepterVpcInfo are nil unless Status.Code == active. The overlap check runs active-only; rendering never dereferences without checks.

**Fetcher-written findings** (Source "wave1"), §4 precedence: state → CIDR overlap:
- vpc-peer.warn.provisioning "provisioning" / vpc-peer.warn.initiating "initiating" SevWarn
- vpc-peer.warn.pending_acceptance — phrase "pending acceptance: expires in <N>d" (N from ExpirationTime; the countdown IS the phrase; Detail carries the date)
- vpc-peer.warn.expired "expired: never accepted" SevWarn
- vpc-peer.broken.rejected "rejected" SevBroken (Detail = Status.Message verbatim)
- vpc-peer.broken.failed "failed" SevBroken (Detail = Status.Message verbatim)
- vpc-peer.warn.deleting "deleting" SevWarn; vpc-peer.dim.deleted "deleted" SevDim
- vpc-peer.warn.cidr_overlap "CIDR overlap with peer" SevWarn (active-only; IPv4 CidrBlockSet×CidrBlockSet intersection ~15 lines of prefix math in the fetcher; Detail names the ranges)

**Cache-scan enricher** internal/aws/vpcpeer_issue_enrichment.go (EnrichLTDeprecatedAMI/snapshot_cross_ref.go layer, zero SDK calls, scans the "rtb" cache):
- vpc-peer.warn.route_blackholed "route to peer blackholed" — tier "~"→SevWarn? NO: spec pins blackholed as COLOR-BEARING Warning vs missing-route as `~`. Emission tiers: blackholed = color-bearing (verify how a wave2-emitted finding can be color-bearing — the lt precedent showed wave2 `~` stays green; if the enrichment fold cannot color rows, DOWNGRADE DECISION at implementation: both signals ship as `~` background checks and the spec §4 is amended to match the fold's actual capability — flag to the release owner in the PR).
- vpc-peer.warn.no_local_route "no local route to peer" `~` background check.
- Guards: rtb cache present AND not truncated; active connections only.

**Related checkers** internal/aws/vpcpeer_related.go:
- rtb — cache-scan via cachedTypedRows[ec2types.RouteTable](cache, "rtb"): Routes[].VpcPeeringConnectionId == pcx-id; UnknownRelated on absent/truncated.
- vpc — CACHE-MEMBERSHIP GATE: collect the VpcIds of BOTH sides present in cachedTypedRows[ec2types.Vpc](cache, "vpc"); ids that resolve = pivot entries; remote side absent = fact only. Never assume requester-is-local.
- ct-events — ctEventsCheckerFor("vpc-peer").
Navigable: {FieldPath: "RequesterVpcInfo.VpcId", TargetType: "vpc"} + {FieldPath: "AccepterVpcInfo.VpcId", TargetType: "vpc"}? Only if the drill-through tolerates a remote-side id that's not in the local vpc list (empty landing = FAIL) — the SAFE navigable set is empty or gated; DECIDE at phase 7 against the drill-through gate; prefer the panel pivot.

## 1. Behavioral test spec (pseudocode)

```text
TEST: active_silence            active + disjoint CIDRs + routed  THEN green, blank S4, 0 findings
TEST: state_phrases             one subtest per provisioning/initiating-request/pending-acceptance/
                                expired/rejected/failed/deleting/deleted THEN exact §4 phrase+severity;
                                rejected/failed Detail == Status.Message verbatim; no raw enum
TEST: pending_countdown         ExpirationTime = now+72h THEN "pending acceptance: expires in 3d"
TEST: cidr_overlap_active       overlapping CidrBlockSets on active THEN warn + ranges in Detail
TEST: cidr_nil_safe             non-active with nil CIDR fields THEN no panic, no overlap finding
TEST: enricher_missing_route    active pcx, rtb cache has no route to it THEN ~ "no local route to peer"
TEST: enricher_blackhole        route references pcx with State==blackhole THEN "route to peer blackholed"
TEST: enricher_guards           rtb cache absent OR truncated THEN no derived findings
TEST: related_rtb               graph root resolves rtb 2 (two tables route to it)
TEST: related_vpc_gate          local side resolves vpc 1; remote side (not in cache) = no pivot entry
TEST: truncated_rtb_unknown     rtb truncated THEN rtb pivot Unknown, never 0
TEST: wave3_anti                cross-account/cross-region produce no finding; DNS options are facts
```

## 2. Fixture list (internal/demo/fixtures/vpcpeer.go; synthetic account 123456789012 + remote 210987654321)

```text
prod-peer-shared     (GRAPH ROOT) active; requester = local vpc fixture (vpc cache member),
                     accepter = remote OwnerId 210987654321 (NOT in cache — the gate witness);
                     disjoint CIDRs; TWO existing rtb fixtures gain Routes[] entries with
                     VpcPeeringConnectionId → this pcx (rtb pivot 2 — the ≥2 witness)
warn-peer-pending    pending-acceptance, ExpirationTime now+3d (evergreen: relative date pattern)
warn-peer-expired    expired
broken-peer-rejected rejected + Status.Message "Rejected by accepter: CIDR conflict"
broken-peer-failed   failed + Status.Message
warn-peer-deleting   deleting
dim-peer-deleted     deleted
warn-peer-overlap    active + overlapping CidrBlockSets (10.0.0.0/16 both sides)
warn-peer-noroute    active, valid CIDRs, NO rtb route references it (~ witness)
warn-peer-blackhole  active + one rtb fixture route to it with State blackhole
```

Menu badge: pending, expired, rejected, failed, deleting, overlap, blackhole = 5 Warning + 2 Broken = issues:7 (noroute `~` excluded, deleted Dim excluded); rows 10. Counts: type 70 (menu 71) — THE GOAL TOTALS (66→70, menu 67→71); docs 69→70; smoke resource-types(71); filter pins 70→71.

## 3. File scope union

| File | Owner |
|---|---|
| internal/demo/fixtures/vpcpeer.go + rtb/vpc sibling route entries | 6a coder |
| internal/demo/fakes/ec2.go — DescribeVpcPeeringConnections handler | 6a coder |
| fixtures/counts.go "vpc-peer" entry | 6a coder |
| internal/aws/vpcpeer.go — fetcher + CIDR overlap math | 7 coder |
| internal/aws/vpcpeer_issue_enrichment.go — rtb cache-scan | 7 coder |
| internal/aws/vpcpeer_related.go — rtb/vpc/ct-events | 7 coder |
| internal/aws/lt_interfaces.go-style narrow iface if needed (EC2API likely already carries the op — check ec2_interfaces.go first) | 7 coder |
| internal/aws/catalog_networking.go — ResourceTypeDef (Aliases [vpc-peer pcx peering] — uniqueness gate) | 7 coder |
| internal/config/defaults_networking.go — columns: Pcx Id, Status, Requester VPC, Requester Owner, Accepter VPC, Accepter Owner, Expires | 7 coder |
| .a9s/views/vpc-peer.yaml (viewsgen) | 7 coder |
| tests/unit/aws_vpcpeer_test.go + aws_vpcpeer_related_test.go | 6b QA |
| scenario_vpcpeer_visual_test.go + drill-through row + counts pins (menu 70→71, filter, smoke resource-types(71)) + CR273/state-coverage/filter registries | runner + QA |
| docs counts 69→70 (README.tmpl ×3 + Networking row, website + row), README regen | runner |

## 4. Coverage matrix deltas vs lt

No child views, no N+1, no degraded rows (single-call type). U9 graph-root ≥2: rtb 2 on root; vpc 1 (structural — one local side), ct-events exempt → 1/2 = 50% exactly, gate met. Broken/Dim buckets EXIST (rejected/failed/deleted) — no state-coverage allowlist needed. Novelty risks: (a) CIDR overlap math — pure function, unit-test heavy; (b) color-bearing-vs-~ for blackhole (see §0 DOWNGRADE DECISION); (c) evergreen ExpirationTime fixture (relative date — redshift precedent).
