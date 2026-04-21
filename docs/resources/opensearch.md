---
shortName: opensearch
name: OpenSearch Domains
awsApiRef: https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_DomainStatus.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# opensearch — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `opensearch`
- **Display name**: OpenSearch Domains
- **AWS API reference**: https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_DomainStatus.html
- **List API**: `ListDomainNames` (returns per-domain `DomainName` + `EngineType` only — no state, no config).
- **Describe API (if any)**: `DescribeDomains` (bounded fan-out, up to 5 domain names per call; returns full `DomainStatus[]`). Used in Wave 2.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `acm`, `alarm`, `cfn`, `kms`, `logs`, `sg`, `subnet`, `vpc`, `ct-events`.

### `acm`

- **Why related**: A custom domain endpoint (e.g. `search.example.com`) is terminated by an ACM certificate — operator chasing a TLS expiry, a browser handshake error, or a cert-rotation lands on the cert from the domain.
- **How discovered**: Read `DomainStatus.DomainEndpointOptions.CustomEndpointCertificateArn` (set only when `CustomEndpointEnabled==true`); look the ARN up in the already-loaded `acm` list.
- **Count shown**: yes.

### `alarm`

- **Why related**: CloudWatch alarms on `AWS/ES` (legacy namespace preserved for OpenSearch) metrics — `ClusterStatus.red`, `ClusterStatus.yellow`, `FreeStorageSpace`, `JVMMemoryPressure`, `CPUUtilization` — are how a daily-driver operator learns the cluster is sick. The panel jumps straight to the firing/ok alarm.
- **How discovered**: Reverse-scan the already-loaded `alarm` list — an alarm belongs to this domain when `Namespace=="AWS/ES"` (OpenSearch Service keeps the ES namespace for backwards compatibility) AND `Dimensions[]` contains `Name=="DomainName"` with `Value==<this.DomainName>`. — a9s-devops: AWS/ES namespace + `DomainName` dimension is the documented CloudWatch surface; one pass over the loaded alarm list, zero extra API calls.
- **Count shown**: yes.

### `cfn`

- **Why related**: The CloudFormation stack that provisioned the domain is the fastest route to the template, parameters, and stack events — critical for drift triage and change history.
- **How discovered**: Call `opensearch:ListTags(ARN=DomainStatus.ARN)` and look for the `aws:cloudformation:stack-name` tag (set automatically on CFN-managed resources); look the stack up in the already-loaded `cfn` list. No tag → not CFN-managed (skip). — a9s-devops: the CFN-stack tag is the canonical managed-by marker across AWS; domain config itself has no stack-reference field.
- **Count shown**: yes.

### `kms`

- **Why related**: Encryption-at-rest uses a customer-managed or AWS-managed KMS key — operator wants to confirm the key is enabled and not pending deletion before declaring the domain usable (a disabled key breaks reads/writes).
- **How discovered**: Read `DomainStatus.EncryptionAtRestOptions.KmsKeyId` (only set when `EncryptionAtRestOptions.Enabled==true`); look the key ID up in the already-loaded `kms` list.
- **Count shown**: yes.

### `logs`

- **Why related**: Slow, index-slow, error, and audit logs publish to CloudWatch Logs groups — an operator chasing a slow query or a cluster restart reads the groups listed here.
- **How discovered**: Read `DomainStatus.LogPublishingOptions` map entries (keys `SEARCH_SLOW_LOGS`, `INDEX_SLOW_LOGS`, `ES_APPLICATION_LOGS`, `AUDIT_LOGS`); for each entry with `Enabled==true` take `CloudWatchLogsLogGroupArn` and look it up in the already-loaded `logs` list.
- **Count shown**: yes.

### `sg`

- **Why related**: When the domain is VPC-attached, its ENIs live behind customer-controlled security groups — operator debugging connection timeouts or 403/ACCESS_DENIED from a client jumps from the domain to the SGs to inspect ingress rules.
- **How discovered**: Read `DomainStatus.VPCOptions.SecurityGroupIds[]` (empty/absent when the domain is public-endpoint-only); look each SG ID up in the already-loaded `sg` list.
- **Count shown**: yes.

### `subnet`

- **Why related**: VPC-attached domains place ENIs in specific subnets across AZs — operator reviewing multi-AZ health, subnet exhaustion, or route-table failures needs the subnet list one keypress away.
- **How discovered**: Read `DomainStatus.VPCOptions.SubnetIds[]`; look each subnet ID up in the already-loaded `subnet` list.
- **Count shown**: yes.

### `vpc`

- **Why related**: The parent VPC frames everything else — flow logs, VPC endpoints, NAT — and is the root of network triage.
- **How discovered**: Read `DomainStatus.VPCOptions.VPCId`; look the VPC ID up in the already-loaded `vpc` list.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Universal pivot — applies to every registered type; see `related-resources.md` §Policy.
- **How discovered**: CloudTrail `LookupEvents` with `LookupAttribute=ResourceName` = domain name (and/or `ResourceARN` = `DomainStatus.ARN`).
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — the list API does not return fields usable for attention. `ListDomainNames` returns only `DomainName` and `EngineType`; there is no state, processing flag, or config field to classify from.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: `DomainStatus.Deleted == true`.
  - **State bucket**: Dim.
  - **API call**: `DescribeDomains` — bounded fan-out (up to 5 domain names per call; effectively one-per-N-resources).
  - **Cost shape**: hybrid (batched per-resource).

- **Signal**: `DomainStatus.Processing == true` OR `DomainStatus.UpgradeProcessing == true`.
  - **State bucket**: Warning.
  - **API call**: `DescribeDomains` — bounded fan-out (shared with other Wave 2 signals; one DescribeDomains call returns all the fields).
  - **Cost shape**: hybrid.

- **Signal**: `DomainStatus.DomainProcessingStatus == "Isolated"`.
  - **State bucket**: Broken.
  - **API call**: `DescribeDomains` — bounded fan-out (shared).
  - **Cost shape**: hybrid.

- **Signal**: `DomainStatus.ServiceSoftwareOptions.UpdateAvailable == true` AND `ServiceSoftwareOptions.AutomatedUpdateDate` in the past.
  - **State bucket**: Warning.
  - **API call**: `DescribeDomains` — bounded fan-out (shared).
  - **Cost shape**: hybrid.

- **Signal**: `DomainStatus.EncryptionAtRestOptions.Enabled == false`.
  - **State bucket**: Warning.
  - **API call**: `DescribeDomains` — bounded fan-out (shared).
  - **Cost shape**: hybrid.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: Cluster-health (Red/Yellow/Green) is CloudWatch-only: `AWS/ES` namespace, `ClusterStatus.red`/`yellow`.
- OUT OF SCOPE: `FreeStorageSpace` (CloudWatch metric).
- OUT OF SCOPE: `JVMMemoryPressure` (CloudWatch metric).

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `isolated: cluster quarantined by AWS`). **Healthy rows render blank** — no `OK` / `Active` / `available`. Empty means "nothing to see." |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping applied here:

- OpenSearch has **no Wave 1** signal at all (the list API is config-thin), so on first paint every row is green with a blank Status. The attention picture appears only after the Wave 2 `DescribeDomains` fan-out returns.
- Wave 2 hard-state signals (`Deleted`, `Processing/UpgradeProcessing`, `Isolated`) drive **S2 + S4** (color + cause). They do not bump S1 and are not candidates for S3 — color is already the signal.
- Wave 2 background-check signals (`UpdateAvailable` past `AutomatedUpdateDate`, `EncryptionAtRestOptions.Enabled==false`) land on a still-green row → **S3 + S4 + S5** (glyph + short cause + detail sentence). `UpdateAvailable` past the auto-update cutoff is a pressing background concern (`!`); missing at-rest encryption is a posture finding (`~`) — see §6 user decision.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `Deleted==true` | 2 | Dim | n/a | S2, S4 | `deleting: removal in progress` | `Domain is being deleted — awaiting AWS to tear down ENIs and release the endpoint.` |
| `Processing==true` or `UpgradeProcessing==true` | 2 | Warning | n/a | S2, S4 | `processing: config change in flight` | `AWS is applying a configuration or version change — writes continue, brief brownouts possible.` |
| `DomainProcessingStatus=="Isolated"` | 2 | Broken | n/a | S2, S4 | `isolated: quarantined by AWS` | `AWS has quarantined the domain (billing, policy, or health) — no reads/writes until resolved.` |
| `ServiceSoftwareOptions.UpdateAvailable==true` AND `AutomatedUpdateDate` past | 2 | Healthy | `!` | S1, S3, S4, S5 | `software update forced soon` | `Service-software update is available; AWS will apply it automatically any day — plan the window.` |
| `EncryptionAtRestOptions.Enabled==false` | 2 | Healthy | `~` | S3, S4, S5 | `encryption at rest off` | `Indexes are stored unencrypted on disk — enable at-rest encryption for compliance.` |

## 4.1 UX review

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every non-green row carries an explicit cause in the Status column (`isolated: quarantined by AWS`, `processing: config change in flight`, `deleting: removal in progress`), and the two background-check glyphs (`!` software update forced soon, `~` encryption at rest off) each pair their glyph with a short readable S4 cause so detail-view navigation is optional. All problem rows are self-explanatory in the list — operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): CloudWatch `ClusterStatus.red`/`yellow`, `FreeStorageSpace`, `JVMMemoryPressure`.
- `role` related-panel entry — explicitly excluded by `docs/related-resources.md` §"Majority no" note: advanced-security master user is a policy pivot, not a role field. No IAM role is a first-class attribute on a domain.
- Per-node runtime state (individual shard/node health) — requires OpenSearch data-plane API (not AWS control plane); out of scope for a9s.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- a9s golden doc — Per-type related contract for `opensearch` — `docs/related-resources.md` § `Per-type contract` (line 86) and `### opensearch` (line 715).
- a9s golden doc — Wave 1 / Wave 2 / Wave 3 signal definitions for `opensearch` — `docs/attention-signals.md` § `Databases & Storage` (`opensearch` row).
- a9s golden doc — `role` intentionally excluded — `docs/related-resources.md` § "Majority `no`" (line 1109: "advanced-security master user is a policy pivot, not a role field").
- a9s golden doc — `ct-events` universal pivot — `docs/related-resources.md` § `Policy` point 4.
- a9s golden doc — read-only invariant — `docs/architecture.md` § `What is a9s?`.
- AWS SDK Go v2 — `ListDomainNames` returns only `DomainName` + `EngineType` per entry — `AWS SDK Go v2 — opensearch/types.DomainInfo § DomainName, EngineType`.
- AWS SDK Go v2 — `DomainStatus` Wave-2 fields — `AWS SDK Go v2 — opensearch/types.DomainStatus § Deleted, Processing, UpgradeProcessing, DomainProcessingStatus, ServiceSoftwareOptions, EncryptionAtRestOptions, VPCOptions, DomainEndpointOptions, LogPublishingOptions`.
- AWS SDK Go v2 — `ServiceSoftwareOptions.UpdateAvailable` and `AutomatedUpdateDate` — `AWS SDK Go v2 — opensearch/types.ServiceSoftwareOptions § UpdateAvailable, AutomatedUpdateDate`.
- AWS SDK Go v2 — `DomainProcessingStatusType` enum values (incl. `Isolated`) — `AWS SDK Go v2 — opensearch/types.DomainProcessingStatusType`.
- AWS SDK Go v2 — `EncryptionAtRestOptions` field — `AWS SDK Go v2 — opensearch/types.EncryptionAtRestOptions § Enabled, KmsKeyId`.
- AWS SDK Go v2 — `VPCDerivedInfo` fields — `AWS SDK Go v2 — opensearch/types.VPCDerivedInfo § VPCId, SubnetIds, SecurityGroupIds`.
- AWS SDK Go v2 — `DomainEndpointOptions.CustomEndpointCertificateArn` — `AWS SDK Go v2 — opensearch/types.DomainEndpointOptions § CustomEndpointCertificateArn, CustomEndpointEnabled`.
- AWS SDK Go v2 — `LogPublishingOption.CloudWatchLogsLogGroupArn` — `AWS SDK Go v2 — opensearch/types.LogPublishingOption § CloudWatchLogsLogGroupArn, Enabled`.
- AWS API Reference — `DescribeDomains` (bounded fan-out, up to 5 domain names per call) — `AWS API Reference: DescribeDomains` (https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_DescribeDomains.html).
- AWS API Reference — CloudWatch metrics for OpenSearch use the `AWS/ES` namespace with `DomainName` dimension — `AWS Developer Guide: Monitoring OpenSearch Service cluster metrics with Amazon CloudWatch` (https://docs.aws.amazon.com/opensearch-service/latest/developerguide/managedomains-cloudwatchmetrics.html).
- a9s-devops consultation — `acm` discovery via `DomainEndpointOptions.CustomEndpointCertificateArn` — `a9s-devops (2026-04-20, persona): possible=yes, worth=yes. Cert is surfaced only when CustomEndpointEnabled; no cert for default *.es.amazonaws.com endpoints.`
- a9s-devops consultation — `alarm` discovery via reverse-scan on `AWS/ES` namespace + `DomainName` dimension — `a9s-devops (2026-04-20, persona): possible=yes, worth=yes. OpenSearch retains the AWS/ES namespace for backward compatibility; the DomainName dimension is the join key.`
- a9s-devops consultation — `cfn` discovery via `aws:cloudformation:stack-name` tag (requires `opensearch:ListTags`) — `a9s-devops (2026-04-20, persona): possible=yes, worth=yes. CFN tag is the canonical managed-by marker; DomainStatus carries no stack reference.`
- a9s-devops consultation — Wave 2 batching with `DescribeDomains` (up to 5 domain names per call) — `a9s-devops (2026-04-20, persona): possible=yes, worth=yes. DescribeDomains is the only API that returns full DomainStatus; batching caps fan-out at N/5 calls, well within Wave 2 bounds.`
- user decision — severity for `ServiceSoftwareOptions.UpdateAvailable` past `AutomatedUpdateDate` — `user (2026-04-20): decide → `!`. Rationale: AWS will auto-apply any day, causing a rolling restart; operator wants this flagged so the window can be planned. Consistent with ACM `!` for imminent cert expiry.`
- user decision — severity for `EncryptionAtRestOptions.Enabled==false` — `user (2026-04-20): decide → `~`. Rationale: posture/compliance finding, not an outage risk. Consistent with RDS `StorageEncrypted==false` and S3 encryption defaults treated as background annotations.`
