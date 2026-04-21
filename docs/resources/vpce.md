---
shortName: vpce
name: VPC Endpoints
awsApiRef: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_VpcEndpoint.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# vpce — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `vpce`
- **Display name**: VPC Endpoints
- **AWS API reference**: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_VpcEndpoint.html
- **List API**: `DescribeVpcEndpoints` (returns the full `VpcEndpoint` shape per endpoint, including `VpcEndpointId`, `VpcEndpointType` (`Interface` | `Gateway` | `GatewayLoadBalancer` | `Resource` | `ServiceNetwork`), `State`, `VpcId`, `ServiceName`, `RouteTableIds[]`, `SubnetIds[]`, `Groups[].GroupId`, `NetworkInterfaceIds[]`, `DnsEntries[].{DnsName, HostedZoneId}`, `PrivateDnsEnabled`, `LastError.{Code, Message}`, `FailureReason`, `PolicyDocument`, `CreationTimestamp`, `Tags[]`).
- **Describe API (if any)**: not used. Wave 1 signals are sufficient; there is no per-endpoint Describe call in the Wave 2 budget (endpoint-policy semantic analysis and DNS-resolution probes are Wave 3 — see §3.3).

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `acm`, `alarm`, `cf`, `eni`, `logs`, `r53`, `rtb`, `s3`, `sg`, `subnet`, `tg`, `vpc`, `waf`, `ct-events`.

### `acm`

- **Why related**: The ACM certificate presented by a custom-domain endpoint (interface endpoints that front a private API Gateway, PrivateLink service, or a user-owned service that terminates TLS with a private cert) — operator pivots here when a client is getting a TLS validation error against `*.vpce-svc-...amazonaws.com` or a custom private hostname. — a9s-devops (2026-04-20): possible=yes, worth=yes. PrivateLink-backed interface endpoints that terminate TLS with a customer cert are a real (though narrow) incident path; the related-resources.md row reflects this rather than a generic pivot.
- **How discovered**: No direct field on `VpcEndpoint` links to an ACM ARN. For `Interface` endpoints where `PrivateDnsEnabled==true` and `ServiceName` begins with `com.amazonaws.vpce.` (private service), open the already-loaded `acm` list and let the operator visually confirm by common name; no automatic cross-reference. Alternative: call `DescribeVpcEndpointServiceConfigurations` and read `PrivateDnsNameConfiguration`, but this is Wave-3-budget. Show the full `acm` list as candidate pivots. — a9s-devops (2026-04-20): possible=partial (no FK field on the endpoint itself), worth=yes for the pivot even when automatic count is unavailable.
- **Count shown**: unknown (no deterministic cross-reference; pivot opens the full `acm` list).

### `alarm`

- **Why related**: CloudWatch alarms watching interface-endpoint connectivity and data volume (for example `AWS/PrivateLinkEndpoints` namespace: `ActiveConnections`, `BytesProcessed`, `PacketsDropped`). Operator pivots here when an endpoint is marked Available but consumers are getting timeouts, or when auditing what monitoring exists on a critical endpoint. — a9s-devops (2026-04-20): possible=yes, worth=yes. `AWS/PrivateLinkEndpoints` publishes per-endpoint metrics keyed by `VPC Endpoint Id`, and alarms written against that dimension are the standard operational signal.
- **How discovered**: Filter the already-loaded `alarm` list client-side where any entry in `MetricAlarm.Dimensions[]` has `Name=="VPC Endpoint Id"` (or legacy `VpcEndpointId`) and `Value==this.VpcEndpointId`. For `MetricAlarms` keyed by `Namespace==AWS/PrivateLinkEndpoints`. — a9s-devops (2026-04-20): possible=yes, worth=yes. Dimension-based scan of the already-loaded alarm list is the cheap path and mirrors how engineers actually find endpoint alarms.
- **Count shown**: yes (0 or more).

### `cf`

- **Why related**: Rare, but real: CloudFront distributions that use a VPC Origin (launched 2024) target an interior resource through a PrivateLink-style attachment backed by a VPC endpoint or endpoint service. Operator pivots here when a VPC-origin distribution is returning `504 ErrorCode: OriginDNSError` and wants to confirm which endpoint CloudFront is reaching. — a9s-devops (2026-04-20): possible=yes, worth=yes for VPC-origin distributions (niche but high-signal when it applies).
- **How discovered**: No direct FK field on `VpcEndpoint` — CloudFront stores the origin's VPC attachment on the distribution side (`Origin.VpcOriginConfig.VpcOriginId`). The pivot shows the full `cf` list; automatic linkage requires a Wave-2 `ListVpcOrigins` call against CloudFront which is out of scope. — a9s-devops (2026-04-20): possible=partial (no FK on vpce), worth=yes for the pivot (operator can scan a small list of VPC-origin distributions).
- **Count shown**: unknown (no deterministic cross-reference).

### `eni`

- **Why related**: For an interface endpoint, AWS provisions one ENI per subnet — these ENIs carry the private IPs that clients reach. Operator pivots here when DNS resolves the endpoint to an IP but connections time out (firewall at the ENI level), or when auditing which IPs a consumer is actually talking to.
- **How discovered**: Filter the already-loaded `eni` list where `NetworkInterface.NetworkInterfaceId` appears in `this.NetworkInterfaceIds[]` — direct FK from the endpoint to its ENIs.
- **Count shown**: yes (equal to the number of subnets the interface endpoint occupies; gateway endpoints have no ENIs).

### `logs`

- **Why related**: VPC flow logs in this VPC (or subnet-scoped flow logs for endpoint subnets) carry REJECT / ACCEPT records for traffic to the endpoint ENI's IPs — operator pivots here to answer "is traffic even reaching the endpoint, or is an NACL/SG dropping it before it gets there?". — a9s-devops (2026-04-20): possible=yes, worth=yes. Flow logs land in CloudWatch Logs groups (or S3), and an operator chasing "connection refused" wants a one-hop pivot from the endpoint to the log group.
- **How discovered**: No direct FK on `VpcEndpoint`. Call `DescribeFlowLogs` (already an account-wide Wave-2 call for `vpc`; reuse its cached response) and filter entries where `ResourceId==this.VpcId` OR `ResourceId` appears in `this.SubnetIds[]` AND `LogDestinationType==cloud-watch-logs`. Match the resulting `LogGroupName` values to entries in the already-loaded `logs` list. — a9s-devops (2026-04-20): possible=yes, worth=yes. Reuse of the existing `DescribeFlowLogs` cache keeps this at no additional cost; the match is deterministic.
- **Count shown**: yes (0 or more log groups covering this endpoint's traffic).

### `r53`

- **Why related**: Private DNS for an interface endpoint is implemented as a Route 53 private hosted zone that AWS manages inside the VPC (`PrivateDnsEnabled==true`) — operator pivots here when service discovery is broken ("my resource resolves the public AWS IP, not the endpoint IP") to confirm the private zone is present and associated with this VPC.
- **How discovered**: Direct field: `VpcEndpoint.DnsEntries[].HostedZoneId` gives the Route 53 hosted-zone IDs backing this endpoint's DNS. Filter the already-loaded `r53` list by `HostedZone.Id` membership in this set. — a9s-devops (2026-04-20): possible=yes, worth=yes. `DnsEntries[].HostedZoneId` is on every `DescribeVpcEndpoints` response for interface endpoints with private DNS enabled.
- **Count shown**: yes (0 for gateway endpoints and for interface endpoints with `PrivateDnsEnabled==false`; 1+ otherwise).

### `rtb`

- **Why related**: For a Gateway endpoint (S3, DynamoDB), AWS injects a prefix-list route into each associated route table. Operator pivots here when "why can't my private subnet reach S3?" — the answer is almost always "the gateway endpoint is not attached to that subnet's route table." Also the pivot when a blackhole route is the surviving ghost of a deleted endpoint.
- **How discovered**: Direct field: `VpcEndpoint.RouteTableIds[]` gives the IDs of route tables wired to this gateway endpoint. Filter the already-loaded `rtb` list by `RouteTable.RouteTableId` membership in this set. — a9s-devops (2026-04-20): possible=yes, worth=yes. `RouteTableIds[]` is on every `DescribeVpcEndpoints` response for `Gateway` endpoints.
- **Count shown**: yes (0 for interface endpoints; 1+ for gateway endpoints).

### `s3`

- **Why related**: The single highest-value real-world use of VPC endpoints: Gateway endpoint for S3 (private-subnet compute reaching S3 without NAT). Operator pivots here when debugging bucket access ("is my bucket policy allowing this VPC endpoint?") — a bucket policy condition on `aws:SourceVpce` is the standard lockdown pattern. — a9s-devops (2026-04-20): possible=yes, worth=yes. `com.amazonaws.<region>.s3` is the most common gateway endpoint in production; pivoting to the bucket list to inspect policies is a daily workflow.
- **How discovered**: No FK in the endpoint response to individual buckets. When `this.ServiceName` matches `com.amazonaws.<region>.s3`, the pivot opens the full `s3` list so the operator can inspect per-bucket policies that reference `aws:SourceVpce`. — a9s-devops (2026-04-20): possible=partial (no FK, but the pivot is well-defined when `ServiceName` is S3), worth=yes.
- **Count shown**: unknown (pivot opens the full `s3` list; a count would require per-bucket `GetBucketPolicy` calls, which are Wave-3).

### `sg`

- **Why related**: For an interface endpoint, the attached security groups govern inbound TLS (typically 443) to the endpoint ENIs. Operator pivots here when a consumer is getting connection-refused or timeouts through the endpoint — the cause is almost always an SG that doesn't allow the consumer's CIDR on 443.
- **How discovered**: Direct field: `VpcEndpoint.Groups[].GroupId` gives the IDs of security groups on the endpoint ENIs. Filter the already-loaded `sg` list by `SecurityGroup.GroupId` membership in this set. — a9s-devops (2026-04-20): possible=yes, worth=yes. `Groups[].GroupId` is on every `DescribeVpcEndpoints` response for interface endpoints.
- **Count shown**: yes (0 for gateway endpoints; 1+ for interface endpoints).

### `subnet`

- **Why related**: For an interface endpoint, one ENI is provisioned per subnet listed on the endpoint — operator pivots here to confirm the endpoint covers every AZ the consumer workload runs in, and to spot single-AZ endpoints feeding a multi-AZ workload (availability risk).
- **How discovered**: Direct field: `VpcEndpoint.SubnetIds[]` gives the subnet IDs for this interface endpoint. Filter the already-loaded `subnet` list by `Subnet.SubnetId` membership in this set. — a9s-devops (2026-04-20): possible=yes, worth=yes. `SubnetIds[]` is on every `DescribeVpcEndpoints` response for interface endpoints.
- **Count shown**: yes (0 for gateway endpoints; 1+ for interface endpoints).

### `tg`

- **Why related**: A PrivateLink service publisher configures a NLB; the `tg` (target groups) behind that NLB are what actually answer consumer traffic arriving through the endpoint. Operator pivots here on the consumer side when debugging "endpoint is Available, connections timeout" — the real failure is often on the far side (unhealthy targets behind the provider NLB). — a9s-devops (2026-04-20): possible=yes, worth=yes for operator-published PrivateLink services.
- **How discovered**: No FK from a consumer endpoint to provider target groups (that crosses an account boundary). When `this.VpcEndpointType==Interface` and `this.ServiceName` begins `com.amazonaws.vpce.` (customer PrivateLink), the pivot opens the full `tg` list — useful only when the operator owns both the provider and the consumer account. — a9s-devops (2026-04-20): possible=partial, worth=yes for shops that own both ends (common in platform teams).
- **Count shown**: unknown (no deterministic cross-reference).

### `vpc`

- **Why related**: The parent VPC this endpoint belongs to — operator pivots up one level to see sibling networking resources (route tables, subnets, flow logs) and to confirm the endpoint is in the VPC they think it is.
- **How discovered**: Direct field: `VpcEndpoint.VpcId`. Open the matching entry in the already-loaded `vpc` list. — a9s-devops (2026-04-20): possible=yes, worth=yes. `VpcId` is on every `DescribeVpcEndpoints` response.
- **Count shown**: yes (always 1).

### `waf`

- **Why related**: WAF Web ACLs can be associated with API Gateway REST APIs (regional / private) that are fronted by an interface endpoint. Operator pivots here when a private API is returning 403s that don't appear in API Gateway logs — the block is at the WAF edge. — a9s-devops (2026-04-20): possible=yes, worth=yes for private API Gateway endpoints.
- **How discovered**: No FK from `VpcEndpoint` to WAF; association lives on the API Gateway side (`waf:GetWebACLForResource` per API). The pivot opens the full `waf` list so the operator can correlate by ACL scope (`REGIONAL`) and protected resource. — a9s-devops (2026-04-20): possible=partial, worth=yes.
- **Count shown**: unknown (no deterministic cross-reference).

### `ct-events`

- **Why related**: Audit trail for endpoint lifecycle and configuration changes — universal pivot for "who deleted / modified / accepted this endpoint, and when?". Typical CloudTrail event names: `CreateVpcEndpoint`, `DeleteVpcEndpoints`, `ModifyVpcEndpoint`, `AcceptVpcEndpointConnections`, `RejectVpcEndpointConnections`.
- **How discovered**: Call CloudTrail `LookupEvents` filtered by `ResourceName==this.VpcEndpointId` (and/or event-name filter on the names above). Universal pivot — applies to every registered type; see `related-resources.md` § Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `State == Available` → Healthy.
  - **State bucket**: Healthy.
  - **How obtained**: `State` field (type `State`) on the `VpcEndpoint` returned by `DescribeVpcEndpoints`.

- **Signal**: `State == PendingAcceptance` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `State` field on the list-response endpoint (producer hasn't accepted the consumer's PrivateLink request yet).

- **Signal**: `State == Pending` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `State` field on the list-response endpoint (endpoint is provisioning).

- **Signal**: `State == Deleting` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `State` field on the list-response endpoint.

- **Signal**: `State == Failed` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `State` field on the list-response endpoint; surface `LastError.Code` + `LastError.Message` as the cause.

- **Signal**: `State == Rejected` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `State` field on the list-response endpoint (producer rejected the consumer's PrivateLink request).

- **Signal**: `State == Expired` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `State` field on the list-response endpoint.

- **Signal**: `State == Partial` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `State` field on the list-response endpoint (interface endpoint with some AZ ENIs failed to provision — subset of expected ENIs came up).

- **Signal**: `State == Deleted` → Dim.
  - **State bucket**: Dim.
  - **How obtained**: `State` field on the list-response endpoint.

- **Signal**: `LastError` non-empty (any state) → Broken detail.
  - **State bucket**: Broken.
  - **How obtained**: `LastError.Code` and `LastError.Message` pointer-fields on the list-response endpoint. Surface the message as the cause text in S4/S5 when present, regardless of the headline `State` (the error can persist into `Available` states after a transient failure and is the first piece an operator reads).

- **Signal**: Interface endpoint with `NetworkInterfaceIds == []` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `VpcEndpointType == Interface` AND `NetworkInterfaceIds` slice is empty on the list-response endpoint. An interface endpoint without ENIs cannot serve traffic.

- **Signal**: Gateway endpoint with `RouteTableIds == []` → Warning (orphan).
  - **State bucket**: Warning.
  - **How obtained**: `VpcEndpointType == Gateway` AND `RouteTableIds` slice is empty on the list-response endpoint. A gateway endpoint with no associated route tables is configured but routes nothing — the classic "S3 endpoint exists but subnet still egresses through NAT" cause.

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: Endpoint policy analysis — parse `PolicyDocument` and detect policies that are over-broad (`Effect: Allow, Principal: *, Action: *, Resource: *`) or that contradict their referenced bucket/table policies. Requires JSON policy evaluation and cross-resource correlation that is out of the Wave 2 budget.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". Never appears on yellow/red/dim rows. |
| S4 | Status / description column text | Short human-readable cause. Healthy rows render blank. |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping for this resource:

- **Wave 1 Healthy** (`Available`, no `LastError`) → no §4 row. S2 renders green, S4 renders blank. Silence is the UX.
- **Wave 1 Warning** signals (`PendingAcceptance`, `Pending`, `Deleting`, gateway with no route tables) → S2 (yellow) + S4 (cause). No S1, S3, S5.
- **Wave 1 Broken** signals (`Failed`, `Rejected`, `Expired`, `Partial`, interface with no ENIs, non-empty `LastError`) → S2 (red) + S4 (cause text, preferring `LastError.Message` when present). No S1 (Wave 1 does not produce a finding object), no S3 (red rows don't carry glyphs).
- **Wave 1 Dim** (`Deleted`) → S2 (gray) + S4 (`deleted`). No S1, no S3, no S5.
- No Wave 2 findings exist for this resource, so S1 and S3 never fire on a `vpce` row under the current contract.

One row per §3 signal (Healthy case omitted per rule):

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `State == PendingAcceptance` | 1 | Warning | n/a | S2, S4 | `pending acceptance` | n/a (Wave 1 Warning has no S5) |
| `State == Pending` | 1 | Warning | n/a | S2, S4 | `pending: provisioning` | n/a |
| `State == Deleting` | 1 | Warning | n/a | S2, S4 | `deleting` | n/a |
| `State == Failed` | 1 | Broken | n/a | S2, S4 | `failed: <LastError.Message>` | n/a (Wave 1 Broken has no S5) |
| `State == Rejected` | 1 | Broken | n/a | S2, S4 | `rejected by service owner` | n/a |
| `State == Expired` | 1 | Broken | n/a | S2, S4 | `expired` | n/a |
| `State == Partial` | 1 | Broken | n/a | S2, S4 | `partial: some AZ ENIs missing` | n/a |
| `LastError` non-empty | 1 | Broken | n/a | S2, S4 | `<LastError.Code>: <LastError.Message>` | n/a |
| interface, `NetworkInterfaceIds == []` | 1 | Broken | n/a | S2, S4 | `interface: no ENIs — unreachable` | n/a |
| gateway, `RouteTableIds == []` | 1 | Warning | n/a | S2, S4 | `gateway: no route tables attached` | n/a |
| `State == Deleted` | 1 | Dim | n/a | S2, S4 | `deleted` | n/a |

Note: S4 cells pair the state with a cause per the "state keywords are not explanations" rule; bare `Pending` or `Failed` would be insufficient. When `LastError.Message` is present for a `Failed` row, it replaces the generic `failed` cause at render time. Truncate `LastError.Message` at 40 chars for the list view — the full sentence is available in the detail view's field block (which is always rendered for any resource and is not an S5 enrichment line).

## 4.1 UX review (two sentences)

At 3am, glancing at the list, a red vpce row with `interface: no ENIs — unreachable` or `failed: <LastError.Message>` tells the operator exactly why the endpoint is down without opening detail; a yellow row with `gateway: no route tables attached` points them straight at the route-table pivot. All problem rows are self-explanatory in the list — operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (endpoint policy semantic analysis).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` § "What is a9s?").
- `PrivateDnsEnabled==false` on an interface endpoint — surfacing this as a Warning would be presumptuous: private-DNS-off is a deliberate choice for services with conflicting names, not a misconfiguration. — a9s-devops (2026-04-20): possible=yes, worth=no. Show it as a field in the detail view, not as an attention signal.
- CloudWatch `AWS/PrivateLinkEndpoints` metrics (`PacketsDropped`, `BytesProcessed`) per endpoint. Exceeds the Wave 2 budget (one GetMetricStatistics call per endpoint per dimension) and operators reach these via `alarm` pivots instead. — a9s-devops (2026-04-20): possible=yes, worth=no (for Wave 2). Handled out-of-band via CloudWatch alarm integration.
- DNS resolution probe ("does `com.amazonaws.<region>.s3` resolve from inside this VPC?") — a9s does not dial into the VPC. — a9s-devops (2026-04-20): possible=no from outside the VPC, worth=n/a.
- `DescribeVpcEndpointServicePermissions` / `DescribeVpcEndpointConnections` — producer-side views of PrivateLink connections. Not registered as an a9s resource type; outside this spec's scope.
- Endpoint `PolicyDocument` rendered as a parsed-tree surface. Show the raw document in the detail view's field block; semantic analysis belongs in Security Hub / IAM Access Analyzer, not a9s.
- Per-endpoint CloudTrail data events for S3 gateway endpoints (who accessed which bucket through this endpoint). Routed through the `trail` / `ct-events` resources, not through `vpce`.

## 6. Citations

- `shortName`, display name, signal cells (`State` bucket mapping, `LastError`, `NetworkInterfaceIds`, `RouteTableIds` Wave 1 rules), list API — `docs/attention-signals.md` § Networking row for `vpce` (line 57).
- AWS API reference URL, related targets list — `docs/related-resources.md` § Per-type contract row for `vpce` (line 109) and § `vpce` narrative block (lines 1030–1047).
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
- `VpcEndpoint.VpcEndpointId`, `VpcEndpointType`, `State`, `VpcId`, `ServiceName`, `RouteTableIds`, `SubnetIds`, `Groups[].GroupId`, `NetworkInterfaceIds`, `DnsEntries[].{DnsName, HostedZoneId}`, `PrivateDnsEnabled`, `LastError.{Code, Message}`, `FailureReason`, `PolicyDocument`, `CreationTimestamp`, `Tags` field names — `AWS SDK Go v2 — service/ec2/types.VpcEndpoint`.
- `State` enum values (`PendingAcceptance`, `Pending`, `Available`, `Deleting`, `Deleted`, `Rejected`, `Failed`, `Expired`, `Partial`) — `AWS SDK Go v2 — service/ec2/types.State` (`StatePendingAcceptance`, `StatePending`, `StateAvailable`, `StateDeleting`, `StateDeleted`, `StateRejected`, `StateFailed`, `StateExpired`, `StatePartial`).
- `VpcEndpointType` enum values (`Interface`, `Gateway`, `GatewayLoadBalancer`, `Resource`, `ServiceNetwork`) — `AWS SDK Go v2 — service/ec2/types.VpcEndpointType`.
- `LastError.{Code, Message}` shape — `AWS SDK Go v2 — service/ec2/types.LastError § Code, Message`.
- `SecurityGroupIdentifier.GroupId` for the `sg` cross-reference — `AWS SDK Go v2 — service/ec2/types.SecurityGroupIdentifier § GroupId`.
- `DnsEntry.HostedZoneId` for the `r53` cross-reference — `AWS SDK Go v2 — service/ec2/types.DnsEntry § HostedZoneId`.
- `ct-events` as universal pivot — `docs/related-resources.md` § Policy (line 34).
- CloudTrail event-name filter (`CreateVpcEndpoint`, `DeleteVpcEndpoints`, `ModifyVpcEndpoint`, `AcceptVpcEndpointConnections`, `RejectVpcEndpointConnections`) — `a9s-devops (2026-04-20): possible=yes (CloudTrail records all endpoint management-plane calls), worth=yes. These event names are the filter operators run when investigating endpoint lifecycle and PrivateLink connection decisions.`
- `acm` discovery is partial (no FK on VpcEndpoint) — `a9s-devops (2026-04-20): possible=partial, worth=yes for PrivateLink TLS-debug. The related-resources.md row reflects the niche-but-real pivot; the panel shows the full `acm` list without a deterministic count.`
- `alarm` discovery via dimension-scan on `VPC Endpoint Id` — `a9s-devops (2026-04-20): possible=yes, worth=yes. AWS/PrivateLinkEndpoints publishes per-endpoint metrics; dimension-based scan of the already-loaded alarm list is the cheap path.`
- `cf` discovery is partial (FK lives on CloudFront side as VpcOriginConfig) — `a9s-devops (2026-04-20): possible=partial, worth=yes for VPC-origin distributions. Niche but high-signal when the attachment exists.`
- `eni`, `rtb`, `sg`, `subnet`, `vpc` discovered via direct FK fields on `VpcEndpoint` (`NetworkInterfaceIds`, `RouteTableIds`, `Groups[].GroupId`, `SubnetIds`, `VpcId`) — `a9s-devops (2026-04-20): possible=yes, worth=yes. These are first-class FKs on every DescribeVpcEndpoints response and are the standard operator pivots.`
- `logs` discovery via reuse of the vpc-level `DescribeFlowLogs` cache, matching on `ResourceId == this.VpcId` or subnet IDs — `a9s-devops (2026-04-20): possible=yes, worth=yes. Reuses an existing account-wide Wave 2 call from the vpc spec; no new AWS call for the vpce pivot.`
- `r53` discovery via `DnsEntries[].HostedZoneId` — `a9s-devops (2026-04-20): possible=yes, worth=yes. First-class field on interface endpoints with private DNS enabled.`
- `s3` discovery is partial (no FK, but gated on `ServiceName == com.amazonaws.<region>.s3`) — `a9s-devops (2026-04-20): possible=partial, worth=yes. Most common gateway endpoint in production; bucket-policy inspection is a daily workflow.`
- `tg` discovery is partial (cross-account for PrivateLink producer/consumer) — `a9s-devops (2026-04-20): possible=partial, worth=yes for platform teams that own both ends.`
- `waf` discovery is partial (FK lives on API Gateway association) — `a9s-devops (2026-04-20): possible=partial, worth=yes for private API Gateway endpoints where WAF is the hidden 403 source.`
- Truncation of `LastError.Message` at 40 chars for S4 — `a9s-devops (2026-04-20): possible=yes, worth=yes. AWS error messages exceed 40 chars regularly; truncation preserves the identity/state columns and the full message is still in the detail field block.`
- `PrivateDnsEnabled==false` not a signal — `a9s-devops (2026-04-20): possible=yes, worth=no. Deliberate configuration choice for services with DNS conflicts, not a misconfiguration.`
- CloudWatch Wave 3 metrics rationale — `a9s-devops (2026-04-20): possible=yes, worth=no for Wave 2. Per-endpoint GetMetricStatistics exceeds the Wave 2 budget; alarm pivot covers the operational path.`
- DNS-probe out-of-scope — `a9s-devops (2026-04-20): possible=no, worth=n/a. a9s does not dial into the VPC.`
- Wave 1 `State` bucket mapping, `LastError`, `NetworkInterfaceIds==[]`, `RouteTableIds==[]` rules — `docs/attention-signals.md` § Networking row for `vpce` (line 57).
- Wave 3 endpoint-policy analysis out-of-scope — `docs/attention-signals.md` § Networking row for `vpce` Wave 3 cell (line 57).
