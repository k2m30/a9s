---
shortName: sg
name: Security Groups
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_SecurityGroup.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
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

Expected targets from `docs/related-resources.md` Per-type contract: `cfn`, `ct-events`, `ec2`, `elb`, `eni`, `lambda`, `sg`, `vpc`.

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

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `IpPermissions[]` with `IpRanges[].CidrIp == 0.0.0.0/0` covering any port in the set {22, 23, 21, 3389, 1433, 3306, 5432, 6379, 27017, 11211, 9200}.
  - **State bucket**: Broken.
  - **How obtained**: read `IpPermissions[]` on the SG, inspect each rule's `FromPort`/`ToPort`/`IpProtocol` against `IpRanges[].CidrIp` — the list API returns the full ingress rule set, no extra call. a9s-devops: port list is the standard "admin/database exposed to the internet" set (SSH/telnet/FTP/RDP/SQL/MySQL/Postgres/Redis/Mongo/memcached/Elasticsearch).
- **Signal**: Cross-ref `eni` — this SG's `GroupId` is not referenced by any `NetworkInterface.Groups[].GroupId` in the loaded `eni` list.
  - **State bucket**: Warning.
  - **How obtained**: cross-reference the already-loaded `eni` list by `Groups[].GroupId`. Skip the rule if the `eni` list wasn't loaded in this sweep (cannot distinguish "no users" from "didn't look").

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: SG-referencing-deleted-SG detection.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `open: 22 to 0.0.0.0/0`). **Healthy rows render blank** — no `OK` / `in-use`. Empty means "nothing to see." |
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
| `0.0.0.0/0` on admin/db port | 1 | Broken | n/a | S2 + S4 | `open: 22 to 0.0.0.0/0` | `Ingress rule allows TCP 22 from 0.0.0.0/0 — SSH is reachable from the entire internet.` |
| Not referenced by any ENI (orphan) | 1 | Warning | n/a | S2 + S4 | `orphan: no ENIs attached` | `This security group is not attached to any network interface — candidate for cleanup.` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`open`, `orphan`, `unused`) in the List text column is not acceptable. Pair it with the cause (port+CIDR for exposure, reason for orphan).
- For `0.0.0.0/0`-on-admin-port findings that trip multiple rules on the same SG (e.g. both 22 and 3389), the list text should show the lowest/most-famous port first and pluralize (`open: 22, 3389 to 0.0.0.0/0`). Detail text enumerates all offending rules, one per line.
- Keep both columns short enough to fit: List text ≤ 40 chars, Detail text ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for both signals — a red row with `open: 22 to 0.0.0.0/0` tells the on-call engineer immediately which port and which side of the rule is the problem, and a yellow row with `orphan: no ENIs attached` says the SG is cruft. UX gap worth flagging for implementation: the admin-port rule as written in `docs/attention-signals.md` covers only IPv4 `0.0.0.0/0`; SDK `IpPermission` also exposes `Ipv6Ranges[].CidrIpv6`, and an IPv6 `::/0` on port 22 is equally exposed — recommend extending the check to IPv6 and rendering the list text as `open: 22 to ::/0` in that case. Flagged here per a9s-devops; the golden doc can be amended separately if the team accepts.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`docs/architecture.md` §"What is a9s?").
- Egress `0.0.0.0/0` rules — a9s-devops: not worth flagging at the row level; wide egress is the default VPC behavior and flagging it would drown the list in false positives. Revisit only as an explicit opt-in governance check.

## 6. Citations

- Contract targets `cfn, ct-events, ec2, elb, eni, lambda, sg, vpc` — `docs/related-resources.md` § Per-type contract row `sg` (line 99) and detail block `### \`sg\`` (lines 902–913).
- Wave 1 admin-port signal and port set — `docs/attention-signals.md` § Networking row `sg` (line 60).
- Wave 1 orphan-SG signal (cross-ref `eni`) — `docs/attention-signals.md` § Networking row `sg` (line 60).
- Wave 3 SG-referencing-deleted-SG — `docs/attention-signals.md` § Networking row `sg` (line 60).
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
- IPv6 `::/0` gap — `a9s-devops (2026-04-20): possible=yes, worth=yes. AWS SDK exposes IpPermission.Ipv6Ranges[].CidrIpv6; the current attention-signals.md rule mentions only 0.0.0.0/0 and misses an equally dangerous exposure path. Noted in §4.1 as a UX gap rather than amending the golden doc unilaterally.`
- Egress-wide-open omission — `a9s-devops (2026-04-20): possible=yes, worth=no. Default VPC SG permits all egress; flagging egress 0.0.0.0/0 would paint half the list yellow with no actionable value. Recorded in §5 Out of Scope.`
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
