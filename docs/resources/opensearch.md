---
shortName: opensearch
name: OpenSearch Domains
awsApiRef: https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_DomainStatus.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# opensearch — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `opensearch`
- **Display name**: OpenSearch Domains
- **AWS API reference**: <https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_DomainStatus.html>
- **List API**: `ListDomainNames` (returns per-domain `DomainName` + `EngineType` only — no state, no config).
- **Describe API (if any)**: `DescribeDomains` (bounded fan-out, up to 5 domain names per call; returns full `DomainStatus[]`). Called by the fetcher, since `ListDomainNames` returns nothing classifiable.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `acm`, `alarm`, `cfn`, `kms`, `logs`, `sg`, `subnet`, `vpc`, `ct-events`.

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

- **Why related**: Universal pivot — applies to every registered type; see `docs/related-resources.md` §Policy.
- **How discovered**: CloudTrail `LookupEvents` with `LookupAttribute=ResourceName` = domain name (and/or `ResourceARN` = `DomainStatus.ARN`).
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeDomains](https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_DescribeDomains.html)

Transcribed from `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `opensearch`.

### 3.1 Wave 1 — zero extra API calls

`ListDomainNames` returns only `DomainName` and `EngineType`, so the fetcher pairs it with `DescribeDomains` (bounded fan-out, up to 5 names per call). Every signal below is readable from the `DomainStatus` that call returns, so all of them are decided in the fetcher and none costs an extra call. A domain being deleted reports nothing else.

One bullet per distinct signal.

- **Signal**: `DomainStatus.Deleted == true`.
  - **State bucket**: Dim.
  - **API call**: `DescribeDomains` — bounded fan-out (up to 5 domain names per call; effectively one-per-N-resources).
  - **Cost shape**: hybrid (batched per-resource).

- **Signal**: `DomainStatus.Processing == true` OR `DomainStatus.UpgradeProcessing == true`.
  - **State bucket**: Warning.
  - **API call**: `DescribeDomains` — bounded fan-out (shared; one DescribeDomains call returns all the fields).
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

- **Signal**: no `VPCOptions` AND the access policy allows any principal.
  - **State bucket**: Broken.
  - **API call**: `DescribeDomains` — bounded fan-out (shared).
  - **Cost shape**: hybrid.

- **Signal**: `DomainStatus.DomainEndpointOptions.EnforceHTTPS` not true.
  - **State bucket**: Warning.
  - **API call**: `DescribeDomains` — bounded fan-out (shared).
  - **Cost shape**: hybrid.

- **Signal**: `DomainStatus.NodeToNodeEncryptionOptions.Enabled` not true.
  - **State bucket**: Warning.
  - **API call**: `DescribeDomains` — bounded fan-out (shared).
  - **Cost shape**: hybrid.

- **Signal**: `DescribeDomains` was denied for this domain.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: the domain is absent from the `DescribeDomains` response.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

### 3.2 Wave 2 — bounded extra API calls

None. `DescribeDomains` is the fetcher's own call and its `DomainStatus`
carries every signal in §3.1, so opensearch registers no Wave 2 enricher.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: Cluster-health (Red/Yellow/Green) is CloudWatch-only: `AWS/ES` namespace, `ClusterStatus.red`/`yellow`.
- OUT OF SCOPE: `FreeStorageSpace` (CloudWatch metric).
- OUT OF SCOPE: `JVMMemoryPressure` (CloudWatch metric).

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `opensearch`: Wave 1 issue-colored rows only — this type registers no Wave 2 enricher, so nothing else bumps the count.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `Deleted==true` | 1 | Dim | n/a | S2, S4 | `deleting: removal in progress` |
| `Processing==true` or `UpgradeProcessing==true` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `processing: config change in flight` |
| `DomainProcessingStatus=="Isolated"` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `isolated: quarantined by AWS` |
| `ServiceSoftwareOptions.UpdateAvailable==true` AND `AutomatedUpdateDate` past | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `software update forced soon` |
| `EncryptionAtRestOptions.Enabled==false` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `encryption at rest off` |
| No `VPCOptions` AND access policy allows any principal | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `reachable outside a VPC` |
| `DomainEndpointOptions.EnforceHTTPS` not true | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `HTTPS not enforced` |
| `NodeToNodeEncryptionOptions.Enabled` not true | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `node-to-node encryption off` |
| `DescribeDomains` was denied for this domain | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `details denied` |
| the domain is absent from the `DescribeDomains` response | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `details unavailable` |

## 4.1 UX review

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every non-green row carries an explicit cause in the Status column (`isolated: quarantined by AWS`, `processing: config change in flight`, `deleting: removal in progress`), and the posture signals do the same (`software update forced soon`, `encryption at rest off`), so detail-view navigation is optional. All problem rows are self-explanatory in the list — operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): CloudWatch `ClusterStatus.red`/`yellow`, `FreeStorageSpace`, `JVMMemoryPressure`.
- `role` related-panel entry — `docs/related-resources.md` § Explicitly excluded.
- Per-node runtime state (individual shard/node health) — requires OpenSearch data-plane API (not AWS control plane); out of scope for a9s.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- a9s golden doc — Per-type related contract for `opensearch` — `docs/related-resources.md` § Per-type contract, row `opensearch`, and `docs/related-resources.md` § `opensearch`.
- a9s golden doc — the `opensearch` signals — `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `opensearch`; the deferred CloudWatch cluster health — `docs/attention-signals.md § Not yet implemented`.
- a9s golden doc — `role` intentionally excluded — `docs/related-resources.md` § Explicitly excluded.
- a9s golden doc — `ct-events` universal pivot — `docs/related-resources.md` § `Policy` point 4.
- a9s golden doc — read-only invariant — `docs/architecture.md` § `What is a9s?`.
- AWS SDK Go v2 — `ListDomainNames` returns only `DomainName` + `EngineType` per entry — `AWS SDK Go v2 — opensearch/types.DomainInfo § DomainName, EngineType`.
- AWS SDK Go v2 — `DomainStatus` fields the fetcher reads — `AWS SDK Go v2 — opensearch/types.DomainStatus § Deleted, Processing, UpgradeProcessing, DomainProcessingStatus, ServiceSoftwareOptions, EncryptionAtRestOptions, VPCOptions, DomainEndpointOptions, LogPublishingOptions`.
- AWS SDK Go v2 — `ServiceSoftwareOptions.UpdateAvailable` and `AutomatedUpdateDate` — `AWS SDK Go v2 — opensearch/types.ServiceSoftwareOptions § UpdateAvailable, AutomatedUpdateDate`.
- AWS SDK Go v2 — `DomainProcessingStatusType` enum values (incl. `Isolated`) — `AWS SDK Go v2 — opensearch/types.DomainProcessingStatusType`.
- AWS SDK Go v2 — `EncryptionAtRestOptions` field — `AWS SDK Go v2 — opensearch/types.EncryptionAtRestOptions § Enabled, KmsKeyId`.
- AWS SDK Go v2 — `VPCDerivedInfo` fields — `AWS SDK Go v2 — opensearch/types.VPCDerivedInfo § VPCId, SubnetIds, SecurityGroupIds`.
- AWS SDK Go v2 — `DomainEndpointOptions.CustomEndpointCertificateArn` — `AWS SDK Go v2 — opensearch/types.DomainEndpointOptions § CustomEndpointCertificateArn, CustomEndpointEnabled`.
- AWS SDK Go v2 — `LogPublishingOption.CloudWatchLogsLogGroupArn` — `AWS SDK Go v2 — opensearch/types.LogPublishingOption § CloudWatchLogsLogGroupArn, Enabled`.
- AWS API Reference — `DescribeDomains` (bounded fan-out, up to 5 domain names per call) — `AWS API Reference: DescribeDomains` (<https://docs.aws.amazon.com/opensearch-service/latest/APIReference/API_DescribeDomains.html>).
- AWS API Reference — CloudWatch metrics for OpenSearch use the `AWS/ES` namespace with `DomainName` dimension — `AWS Developer Guide: Monitoring OpenSearch Service cluster metrics with Amazon CloudWatch` (<https://docs.aws.amazon.com/opensearch-service/latest/developerguide/managedomains-cloudwatchmetrics.html>).
- a9s-devops consultation — `acm` discovery via `DomainEndpointOptions.CustomEndpointCertificateArn` — `a9s-devops (2026-04-20, persona): possible=yes, worth=yes. Cert is surfaced only when CustomEndpointEnabled; no cert for default *.es.amazonaws.com endpoints.`
- a9s-devops consultation — `alarm` discovery via reverse-scan on `AWS/ES` namespace + `DomainName` dimension — `a9s-devops (2026-04-20, persona): possible=yes, worth=yes. OpenSearch retains the AWS/ES namespace for backward compatibility; the DomainName dimension is the join key.`
- a9s-devops consultation — `cfn` discovery via `aws:cloudformation:stack-name` tag (requires `opensearch:ListTags`) — `a9s-devops (2026-04-20, persona): possible=yes, worth=yes. CFN tag is the canonical managed-by marker; DomainStatus carries no stack reference.`
- a9s-devops consultation — batching with `DescribeDomains` (up to 5 domain names per call; the fetcher's own call, wave 1) — `a9s-devops (2026-04-20, persona): possible=yes, worth=yes. DescribeDomains is the only API that returns full DomainStatus; batching caps fan-out at N/5 calls, well within Wave 2 bounds.`
- user decision — severity for `ServiceSoftwareOptions.UpdateAvailable` past `AutomatedUpdateDate` — `user (2026-04-20): decide →`!`. Rationale: AWS will auto-apply any day, causing a rolling restart; operator wants this flagged so the window can be planned. Consistent with ACM`!`for imminent cert expiry.`
- orchestrator ruling (2026-09-06) — supersedes the decision above: severity is the one axis with no exemptions, and a pending forced update is a warning, not an outage, so the finding is `~` (Warning) in the fetcher, the catalog and the tables here.
- user decision — severity for `EncryptionAtRestOptions.Enabled==false` — `user (2026-04-20): decide →`~`. Rationale: posture/compliance finding, not an outage risk. Consistent with RDS`StorageEncrypted==false`and S3 encryption defaults treated as background annotations.`

<!-- BEGIN GENERATED: header -->
opensearch — DATABASES & STORAGE. Lifecycle key: `status`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| opensearch.dim.deleting | deleting: removal in progress | dim | wave1 | — |
| opensearch.broken.isolated | isolated: quarantined by AWS | broken | wave1 | — |
| opensearch.warn.processing | processing: config change in flight | warn | wave1 | — |
| opensearch.update-forced | software update forced soon | warn | wave1 | AWS will apply this update automatically once the scheduled date passes; upgrade on your own schedule before then to control the maintenance window. |
| opensearch.encryption-off | encryption at rest off | warn | wave1 | Data at rest is stored unencrypted. Enabling encryption at rest requires creating a new domain and migrating data — it cannot be turned on in place. |
| opensearch.public | reachable outside a VPC | broken | wave1 | The domain sits outside a VPC and its access policy allows any principal, so the search endpoint is reachable from the internet. Move the domain into a VPC, or scope the access policy to named principals. |
| opensearch.https-not-enforced | HTTPS not enforced | warn | wave1 | The domain accepts plaintext HTTP, so queries and results can be read off the wire. Turn on Require HTTPS in the domain's endpoint options. |
| opensearch.node-to-node-tls-off | node-to-node encryption off | warn | wave1 | Traffic between the domain's own nodes is unencrypted. Node-to-node encryption can only be enabled on a domain that already has it configured at creation — recreate the domain if this data is sensitive. |
| opensearch.warn.details\_denied | details denied | warn | wave1 | — |
| opensearch.warn.details\_unavailable | details unavailable | warn | wave1 | Details could not be retrieved; only the name is visible. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| alarm | CW Alarms | yes |
| logs | Log Groups | no |
| sg | Security Groups | no |
| vpc | VPC | no |
| kms | KMS Key | no |
| cfn | CloudFormation | yes |
| subnet | Subnets | no |
| acm | ACM Certificates | yes |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
