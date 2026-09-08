---
shortName: sg
name: Security Groups
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_SecurityGroup.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# sg — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `sg`
- **Display name**: Security Groups
- **AWS API reference**: <https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_SecurityGroup.html>
- **List API**: `DescribeSecurityGroups`
- **Describe API (if any)**: not used — `DescribeSecurityGroups` returns the full `SecurityGroup` shape, including `IpPermissions[]` and `IpPermissionsEgress[]`. No Wave 2 call required.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `cfn`, `ct-events`, `ec2`, `elb`, `eni`, `lambda`, `sg`, `vpc`.

### `cfn`

- **Why related**: CloudFormation stack that created the SG (infra-as-code provenance; answers "who owns this SG and which template to edit to change it").
- **How discovered**: read `Tags[]` on the SG for key `aws:cloudformation:stack-name` — value is the parent stack name. No direct field; tag-heuristic only — a9s-devops: standard CFN-managed-resource convention, present on every CFN-created SG.
- **Count shown**: yes (0 or 1).

### `ec2`

- **Why related**: EC2 instances with this SG attached — "what workloads will this rule change affect?" is the first question an SRE asks before touching an SG.
- **How discovered**: cross-reference the already-loaded `ec2` list by `Instance.SecurityGroups[].GroupId` == this SG's `GroupId`.
- **Count shown**: yes.

### `elb`

- **Why related**: Load balancers with this SG attached — SG change can sever public traffic into an ALB; NLB v2 supports SGs too.
- **How discovered**: cross-reference the already-loaded `elb` list by `LoadBalancer.SecurityGroups[]` containing this SG's `GroupId`. a9s-devops: ALB always has SGs; NLB has them only when explicitly attached (v2 feature); CLB uses a separate `SecurityGroups` field on the classic shape.
- **Count shown**: yes.

### `eni`

- **Why related**: The ENI-level view of "who's using this SG" — covers Lambda, RDS, VPC endpoints, Fargate tasks, and any other service that provisions an ENI. An SG with zero ENIs is an orphan.
- **How discovered**: cross-reference the already-loaded `eni` list by `NetworkInterface.Groups[].GroupId` == this SG's `GroupId`.
- **Count shown**: yes.

### `lambda`

- **Why related**: Lambda functions in a VPC attach SGs to their ENIs — a rule change can break outbound calls from the function.
- **How discovered**: cross-reference the already-loaded `lambda` list by `FunctionConfiguration.VpcConfig.SecurityGroupIds[]` containing this SG's `GroupId`. a9s-devops: the canonical Lambda→SG field, used daily by SREs debugging VPC-Lambda egress failures.
- **Count shown**: yes.

### `sg`

- **Why related**: Other security groups referenced in this SG's ingress/egress rules — SG-to-SG references are the normal way to chain tiers ("app SG allows from web SG"), and tracing the chain is how operators reason about reachability.
- **How discovered**: read `IpPermissions[].UserIdGroupPairs[].GroupId` and `IpPermissionsEgress[].UserIdGroupPairs[].GroupId` on this SG.
- **Count shown**: yes.

### `vpc`

- **Why related**: Parent VPC — every SG lives in exactly one VPC and rules are scoped to its CIDR space.
- **How discovered**: read `VpcId` field on this SG.
- **Count shown**: yes (exactly 1).

### `ct-events`

- **Why related**: Audit trail for rule changes — "who opened port 22 last Tuesday?" is the classic SG forensic question.
- **How discovered**: call CloudTrail `LookupEvents` scoped to this SG's ARN / GroupId (universal pivot — applies to every registered type; see `docs/related-resources.md` §Policy).
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeSecurityGroups](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeSecurityGroups.html)

Transcribed from `docs/attention-signals.md § Signals § NETWORKING` row `sg`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `0.0.0.0/0` on any port in `sensitivePorts`.
  - **State bucket**: Broken.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `IpPermissions[]` with `IpRanges[].CidrIp == 0.0.0.0/0` (or `Ipv6Ranges[].CidrIpv6 == ::/0`) covering any port in the set {20, 21, 22, 23, 25, 445, 1433, 1521, 2483, 3306, 3389, 5432, 5601, 6379, 7199, 8888, 9092, 9160, 9200, 11211, 27017}. An all-protocols (`-1`) rule open to the internet is the same signal one step wider.
  - **Finding**: `sg.ingress.dangerous-ports`, phrase `<port(s) LIST> open to 0.0.0.0/0`; the all-protocols case is `sg.ingress.wide-open`, phrase `all ports open to 0.0.0.0/0`.
  - **State bucket**: Broken.
  - **How obtained**: read `IpPermissions[]` on the SG, inspect each rule's `FromPort`/`ToPort`/`IpProtocol` against `IpRanges[].CidrIp` — the list API returns the full ingress rule set, no extra call. a9s-devops: port list is the standard "admin/database exposed to the internet" set. 8080 and 8443 are deliberately excluded — they front ordinary public applications far more often than anything worth paging on.

- **Signal**: `GroupName == "default"` carrying any ingress rule, or egress beyond the single all-protocols rule to `0.0.0.0/0` AWS creates every group with.
  - **Finding**: `sg.default-with-rules`, phrase `default group allows traffic`.
  - **State bucket**: Warning.
  - **How obtained**: read `GroupName`, `IpPermissions[]` and `IpPermissionsEgress[]` on the list response. AWS attaches this group to anything launched without an explicit one, so every rule on it applies to resources nobody chose to put there.

- **Signal**: ingress opens a sensitive port (SSH, RDP, a database port) to `0.0.0.0/0`.
  - **State bucket**: Broken.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: Cross-ref `eni` — this SG's `GroupId` is not referenced by any `NetworkInterface.Groups[].GroupId` in the loaded `eni` list.
  - **Finding**: `sg.unused`, phrase `not attached to anything`.
  - **State bucket**: Warning.
  - **How obtained**: `EnrichSGUsage` cross-references the already-loaded `eni` list by `Groups[].GroupId`. Zero AWS calls. Emits nothing when the `eni` list was not loaded this sweep or was truncated at the first page — an incomplete list cannot distinguish "no users" from "didn't look". Default groups are exempt: AWS creates one per VPC and it cannot be deleted, so unattached is its normal state.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: SG-referencing-deleted-SG detection.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `sg`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `0.0.0.0/0` on any port in `sensitivePorts` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `all ports open to 0.0.0.0/0` |
| an all-protocols (`-1`) rule open to `0.0.0.0/0` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `all ports open to 0.0.0.0/0` |
| `GroupName == "default"` carrying ingress rules, or egress beyond the AWS-created allow-all | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `default group allows traffic` |
| ingress opens a sensitive port (SSH, RDP, a database port) to `0.0.0.0/0` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `<port(s) LIST> open to 0.0.0.0/0` |
| Not referenced by any ENI in the loaded, untruncated ENI list (non-default groups only) | 2 | Warning | `~` | S2, S3, S4, S5 | `not attached to anything` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`open`, `orphan`, `unused`) in the List text column is not acceptable. Pair it with the cause (port+CIDR for exposure, reason for orphan).
- For `0.0.0.0/0`-on-admin-port findings that trip multiple rules on the same SG (e.g. both 22 and 3389), the list text should show the lowest/most-famous port first and pluralize (`open: 22, 3389 to 0.0.0.0/0`). Detail text enumerates all offending rules, one per line.
- Keep the List text short enough to fit: ≤ 40 chars. The Detail cell quotes the finding's Detail constant verbatim, however long it is.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for all four signals — a red row with `<port(s) LIST> open to 0.0.0.0/0` tells the on-call engineer immediately which port is the problem, a red `all ports open to 0.0.0.0/0` row says the group is wide open, a yellow `default group allows traffic` row says the group AWS attaches by default is not empty, and a yellow `not attached to anything` row says the group is cruft. IPv6 is covered the same way: the all-addresses IPv6 range counts as open to the internet, so an IPv6-only exposure on port 22 renders identically.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`docs/architecture.md` §"What is a9s?").
- Egress `0.0.0.0/0` rules — a9s-devops: not worth flagging at the row level; wide egress is the default VPC behavior and flagging it would drown the list in false positives. Revisit only as an explicit opt-in governance check.

## 6. Citations

- Contract targets `cfn, ct-events, ec2, elb, eni, lambda, sg, vpc` — `docs/related-resources.md` § Per-type contract, row `sg`, and `docs/related-resources.md` § `sg`.
- Wave 1 admin-port signal and port set — `docs/attention-signals.md § Signals § NETWORKING` row `sg`.
- Wave 2 orphan-SG signal (cross-ref `eni`) — `docs/attention-signals.md § Signals § NETWORKING` row `sg`.
- Wave 3 SG-referencing-deleted-SG — `docs/attention-signals.md § Not yet implemented`.
- `SecurityGroup` struct has no `State` field (config-only; SGs are always "healthy" unless a rule-level or usage-level signal fires) — `AWS SDK Go v2 — ec2/types.SecurityGroup`.
- `IpPermissions[].IpRanges[].CidrIp` — `AWS SDK Go v2 — ec2/types.IpPermission § IpRanges` and `ec2/types.IpRange § CidrIp`.
- `IpPermissions[].UserIdGroupPairs[].GroupId` for SG-to-SG pivot — `AWS SDK Go v2 — ec2/types.IpPermission § UserIdGroupPairs` and `ec2/types.UserIdGroupPair § GroupId`.
- `NetworkInterface.Groups[].GroupId` for ENI→SG reverse cross-ref — `AWS SDK Go v2 — ec2/types.NetworkInterface § Groups` (field type `[]GroupIdentifier`).
- `cfn` discovery via `aws:cloudformation:stack-name` tag — `a9s-devops (2026-04-20): possible=yes, worth=yes. CFN writes this tag on every resource it creates; it is the only reliable SG→stack link because SecurityGroup has no StackId field.`
- `ec2` discovery via reverse-ref on `Instance.SecurityGroups[].GroupId` — `a9s-devops (2026-04-20): possible=yes, worth=yes. First-asked question in an SG investigation is "what breaks if I change this rule"; cross-ref the already-loaded ec2 list is cheap.`
- `elb` discovery via reverse-ref on `LoadBalancer.SecurityGroups[]` — `a9s-devops (2026-04-20): possible=yes, worth=yes. ALBs always have SGs; NLB v2 supports attached SGs; CLB carries its own SGs list. Operator pivots from SG to LB when debugging public traffic.`
- `lambda` discovery via `VpcConfig.SecurityGroupIds[]` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Canonical Lambda→SG field; daily-driver workflow for debugging VPC-Lambda egress.`
- `sg` self-reference via `UserIdGroupPairs[].GroupId` — `a9s-devops (2026-04-20): possible=yes, worth=yes. Tiered SG chaining (web→app→db) is the standard AWS pattern; tracing references is how operators reason about reachability.`
- `vpc` discovery via `VpcId` field on SG — `AWS SDK Go v2 — ec2/types.SecurityGroup § VpcId`.
- `ct-events` as universal pivot — `docs/related-resources.md` § Policy (universal pivot applies to every registered type).
- S4 List text wording (`open: 22 to 0.0.0.0/0`, `orphan: no ENIs attached`) — `user (2026-04-20): decide. Matches the skill's S4 rule that bare state keywords are banned; paired port+CIDR gives operator enough context at the list level.`
- S5 Detail text wording — same derivation as S4; human-readable expansion of the same facts.
- IPv6 `::/0` — `a9s-devops (2026-04-20): possible=yes, worth=yes.` Shipped: `isInternetFacing` checks `Ipv6Ranges[].CidrIpv6` alongside `0.0.0.0/0`.
- Egress-wide-open omission — `a9s-devops (2026-04-20): possible=yes, worth=no. Default VPC SG permits all egress; flagging egress 0.0.0.0/0 would paint half the list yellow with no actionable value. Recorded in §5 Out of Scope.`
- Read-only invariant — `docs/architecture.md` § "What is a9s?".

<!-- BEGIN GENERATED: header -->
sg — NETWORKING. Lifecycle key: none (the list API returns no lifecycle field).
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| sg.ingress.wide-open | all ports open to 0.0.0.0/0 | broken | wave1 | One ingress rule opens every port and protocol to the whole internet, so nothing this group protects is reachable only from where you intended. Replace it with rules naming the ports each workload actually serves and the addresses allowed to reach them. |
| sg.ingress.dangerous-ports | <port(s) LIST> open to 0.0.0.0/0 | broken | wave1 | An administrative or database port on this group accepts connections from any address on the internet, which is how credential-stuffing and direct database access start. Narrow the rule to the addresses that need it, or move the access behind a bastion or private link. |
| sg.default-with-rules | default group allows traffic | warn | wave1 | The VPC's default security group still carries rules, and AWS attaches it to any resource launched without an explicit group. Remove every ingress rule and every egress rule other than the AWS-created allow-all, and give each workload its own group. |
| sg.unused | not attached to anything | warn | wave2 | No network interface in this account references this group, so its rules protect nothing and its name still gets picked from the console list. Delete it, or attach it to the workload it was written for. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| vpc | VPC | no |
| ec2 | EC2 Instances | yes |
| eni | Network Interfaces | yes |
| elb | Load Balancers | yes |
| lambda | Lambda Functions | yes |
| cfn | CloudFormation | no |
| sg | Referencing SGs | yes |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
