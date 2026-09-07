---
shortName: cf
name: CloudFront Distributions
awsApiRef: https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_Distribution.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# cf — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `cf`
- **Display name**: CloudFront Distributions
- **AWS API reference**: <https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_Distribution.html>
- **List API**: `ListDistributions`
- **Describe API (if any)**: `GetDistributionConfig` per distribution (Wave 2: access-log enablement).

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `acm`, `alarm`, `ct-events`, `elb`, `lambda`, `logs`, `r53`, `s3`, `waf`.

### `acm`

- **Why related**: `Distribution.ViewerCertificate.ACMCertificateArn` — the TLS cert presented to viewers for aliased domains. When the cert is expiring or failed renewal, the distribution is the blast radius.
- **How discovered**: read `ViewerCertificate.ACMCertificateArn` on the distribution — a direct ARN; the cert lives in `us-east-1` regardless of distribution region — a9s-devops: only pivot shown when `CloudFrontDefaultCertificate==false`, otherwise CF is using its built-in cert and there is no ACM pivot.
- **Count shown**: yes (0 or 1).

### `alarm`

- **Why related**: Error-rate / latency alarms scoped to this distribution. When the CF list shows 5xx climbing, the operator wants to know which alarms are already watching.
- **How discovered**: cross-reference the already-loaded `alarm` list by `Namespace==AWS/CloudFront` and `Dimensions[].Name==DistributionId, Value==<Distribution.Id>` — a9s-devops: standard CW namespace for CF; no extra API call needed when alarms are already in the sweep.
- **Count shown**: yes.

### `elb`

- **Why related**: ALB configured as a CloudFront origin. When a CF distribution returns `5xxErrorRate` and the origin is an ALB, operator pivots straight to the load balancer to check target health and backend state.
- **How discovered**: cross-reference the already-loaded `elb` list by matching `Distribution.Origins.Items[].DomainName` against the ALB's `DNSName` (ELBv2 DNS names have the form `<name>-<id>.<region>.elb.amazonaws.com`) — a9s-devops: there is no direct LB ARN field on the origin; name-match against the loaded ELB list is the standard cross-reference.
- **Count shown**: yes.

### `lambda`

- **Why related**: Lambda@Edge / CloudFront Functions attached to cache behaviors. A misbehaving edge function is a common cause of distribution-wide 5xx spikes.
- **How discovered**: read `Distribution.DefaultCacheBehavior.LambdaFunctionAssociations.Items[].LambdaFunctionARN` plus every `Distribution.CacheBehaviors.Items[].LambdaFunctionAssociations.Items[].LambdaFunctionARN`; deduplicate by function ARN — a9s-devops: ARNs include a function version; pivot targets the function, not the specific version.
- **Count shown**: yes.

### `logs`

- **Why related**: Access-log or real-time-log destinations for this distribution. Operator opens `logs` to grep recent requests when debugging cache hit/miss or a bad origin response.
- **How discovered**: TBD — a9s-devops: not directly available from `ListDistributions`. Standard logging targets S3 (`LoggingConfig.Bucket`), not CloudWatch Logs — that surfaces under `s3` below, not `logs`. Real-time log configs (CW Logs destinations) require `GetRealtimeLogConfig` per configuration, which is outside the Wave 2 budget for `cf`. possible=partial, worth=yes in principle but would push cf into N+1 real-time-config fans-out; left as discovery TBD until a bounded mechanism exists.
- **Count shown**: unknown.

### `r53`

- **Why related**: Route 53 alias records pointing `A`/`AAAA` at this distribution's CloudFront domain — the human-readable hostnames operators use to reach the distribution.
- **How discovered**: cross-reference the already-loaded `r53` record sets by matching `AliasTarget.DNSName` against the distribution's `DomainName` (e.g. `d111111abcdef8.cloudfront.net`) — a9s-devops: CF alias targets drop the trailing dot and are case-insensitive; compare case-folded.
- **Count shown**: yes.

### `s3`

- **Why related**: S3 bucket configured as a CloudFront origin, and/or the bucket receiving standard access logs.
- **How discovered**: cross-reference the already-loaded `s3` list by matching `Distribution.Origins.Items[].DomainName` against `<bucket>.s3.amazonaws.com` / `<bucket>.s3.<region>.amazonaws.com` / `<bucket>.s3-website-<region>.amazonaws.com`, and by matching the standard-log bucket from `GetDistributionConfig` (`LoggingConfig.Bucket`) — a9s-devops: covers both the OAI/OAC static-site case and the S3 access-log sink.
- **Count shown**: yes.

### `waf`

- **Why related**: `Distribution.WebACLId` — the WAFv2 / WAF-Classic Web ACL in front of the distribution. When rate-limited / blocked traffic explains a drop, operator pivots to the ACL to inspect rules and blocked-request counts.
- **How discovered**: read `WebACLId` on the distribution (empty string means no ACL attached).
- **Count shown**: yes (0 or 1).

### `ct-events`

- **Why related**: Universal pivot — audit trail for distribution config changes (`CreateDistribution`, `UpdateDistribution`, `DeleteDistribution`, `TagResource`, etc.). When an operator wonders "why did this change at 2am?", ct-events is the answer.
- **How discovered**: universal pivot — applies to every registered type; see `docs/related-resources.md` §Policy. Filter by `resources[].ARN == Distribution.ARN` or by distribution ID in event detail.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

**Source API**: [ListDistributions](https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_ListDistributions.html)

Transcribed from `docs/attention-signals.md § Signals § DNS & CDN` row `cf`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `Status == InProgress`.
  - **State bucket**: Warning.
  - **How obtained**: `DistributionSummary.Status` field on the list response.

- **Signal**: `Enabled == false`.
  - **State bucket**: Dim.
  - **How obtained**: `DistributionSummary.Enabled` field on the list response.

- **Signal**: `WebACLId == ""` (no WAF attached). — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06)
  - **State bucket**: Warning.
  - **How obtained**: `DistributionSummary.WebACLId` field on the list response.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: `ViewerCertificate.CloudFrontDefaultCertificate == false` AND `MinimumProtocolVersion` in `SSLv3` / `TLSv1` / `TLSv1_2016` / `TLSv1.1_2016`.
  - **State bucket**: Warning.
  - **How obtained**: `DistributionSummary.ViewerCertificate.MinimumProtocolVersion` field on the list response.

- **Signal**: viewer allows plain HTTP / origin `http-only` (`cf.insecure-protocol`).
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: `LoggingConfig.Enabled == false` on the full distribution config.
  - **State bucket**: Warning.
  - **API call**: `GetDistributionConfig` — one call per distribution.
  - **Cost shape**: per-resource.

- **Signal**: an S3 origin naming a bucket absent from the account.
  - **State bucket**: Broken.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: `DefaultRootObject` empty.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: an S3 origin with neither an origin access control nor a legacy origin access identity.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: the default CloudFront certificate on a distribution with custom aliases.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: `GeoRestriction.RestrictionType == none`.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `5xxErrorRate` / `TotalErrorRate` metric-based attention.
- OUT OF SCOPE: Origin-deleted cross-check (detecting that an origin bucket / ALB / custom origin referenced by the distribution no longer exists).

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `cf`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `Status == InProgress` | 1 | Warning | n/a | S1, S2, S4 | `deploying: config propagating` |
| `Enabled == false` | 1 | Dim | n/a | S2, S4 | `disabled (admin-off)` |
| Weak TLS policy on aliased distribution | 2 | Warning | `~` | S3, S4, S5 | `minimum TLS below 1.2` |
| viewer allows plain HTTP / origin `http-only` (`cf.insecure-protocol`) | 2 | Warning | `~` | S3, S4, S5 | `traffic allowed without TLS` |
| `LoggingConfig.Enabled == false` | 2 | Warning | `~` | S3, S4, S5 | `access logging off` |
| an S3 origin naming a bucket absent from the account | 2 | Broken | `!` | S1, S3, S4, S5 | `S3 origin bucket does not exist` |
| `DefaultRootObject` empty | 2 | Warning | `~` | S3, S4, S5 | `no default root object` |
| an S3 origin with neither an origin access control nor a legacy origin access identity | 2 | Warning | `~` | S3, S4, S5 | `S3 origin without origin access control` |
| the default CloudFront certificate on a distribution with custom aliases | 2 | Warning | `~` | S3, S4, S5 | `uses the default CloudFront certificate` |
| `GeoRestriction.RestrictionType == none` | 2 | Warning | `~` | S3, S4, S5 | `no geo restriction` |
| `WebACLId == ""` — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | 1 | Warning | n/a | S2, S4 | `no WAF attached` |

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for every §3 signal: each yellow/red/dim row carries a self-explanatory cause in the Status column (`deploying: config propagating`, `disabled (admin-off)`, `weak TLS: MinimumProtocolVersion=TLSv1`, `no WAF attached`, `access logs off`) so the operator can triage without opening detail; the `~` glyph on the logging-disabled case is intentionally soft because it is a hygiene concern, not an outage.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- Real-time log config discovery for `logs` pivot — a9s-devops: not worth per-distribution `GetRealtimeLogConfig` fan-out today; revisit when a bounded batch API exists or when the feature graduates to Wave 2 budget.

## 6. Citations

- a9s golden doc — per-type contract targets for `cf` — `docs/related-resources.md` § "Per-type contract" row `cf`.
- a9s golden doc — `acm` reason — `docs/related-resources.md` § `cf` → `acm` ("Distribution.ViewerCertificate.AcmCertificateArn").
- a9s golden doc — `alarm` reason — `docs/related-resources.md` § `cf` → `alarm` ("Distribution error-rate alarms").
- a9s golden doc — `ct-events` reason — `docs/related-resources.md` § `cf` → `ct-events`.
- a9s golden doc — `elb` reason — `docs/related-resources.md` § `cf` → `elb` ("ALB origins").
- a9s golden doc — `lambda` reason — `docs/related-resources.md` § `cf` → `lambda` ("Lambda@Edge associations").
- a9s golden doc — `logs` reason — `docs/related-resources.md` § `cf` → `logs` ("Realtime / access logs").
- a9s golden doc — `r53` reason — `docs/related-resources.md` § `cf` → `r53` ("Route 53 alias records pointing here").
- a9s golden doc — `s3` reason — `docs/related-resources.md` § `cf` → `s3` ("S3 origins").
- a9s golden doc — `waf` reason — `docs/related-resources.md` § `cf` → `waf` ("Distribution.WebACLId").
- a9s golden doc — Wave 1 signals for `cf` — `docs/attention-signals.md § Signals § DNS & CDN` row `cf`.
- a9s golden doc — Wave 2 signal `Logging.Enabled==false` — `docs/attention-signals.md § Signals § DNS & CDN` row `cf`.
- a9s golden doc — Wave 3 exclusions (5xxErrorRate, origin-deleted) — `docs/attention-signals.md § Not yet implemented`.
- a9s golden doc — read-only invariant — `docs/architecture.md` § "What is a9s?".
- a9s golden doc — universal pivot policy — `docs/related-resources.md` § Policy.
- AWS Go SDK v2 — `Status` field on list response — `AWS SDK Go v2 — cloudfront/types.DistributionSummary § Status`.
- AWS Go SDK v2 — `Enabled` field on list response — `AWS SDK Go v2 — cloudfront/types.DistributionSummary § Enabled`.
- AWS Go SDK v2 — `WebACLId` field on list response — `AWS SDK Go v2 — cloudfront/types.DistributionSummary § WebACLId`.
- AWS Go SDK v2 — `ViewerCertificate` + `CloudFrontDefaultCertificate` + `MinimumProtocolVersion` — `AWS SDK Go v2 — cloudfront/types.ViewerCertificate § CloudFrontDefaultCertificate, MinimumProtocolVersion, ACMCertificateArn`.
- AWS Go SDK v2 — `Origins.Items[].DomainName` (cross-reference target) — `AWS SDK Go v2 — cloudfront/types.Origin § DomainName`.
- AWS Go SDK v2 — `LoggingConfig.Enabled` + `Bucket` (Wave 2 access-log check and s3 sink pivot) — `AWS SDK Go v2 — cloudfront/types.LoggingConfig § Enabled, Bucket`.
- AWS API Reference (fallback) — `ListDistributions` return shape — AWS API Reference: ListDistributions § DistributionList (<https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_ListDistributions.html>).
- AWS API Reference (fallback) — `GetDistributionConfig` return shape — AWS API Reference: GetDistributionConfig § DistributionConfig (<https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_GetDistributionConfig.html>).
- a9s-devops consultation — `acm` pivot only meaningful when not using CloudFront default cert — a9s-devops (2026-04-20): possible=yes, worth=yes. Default-cert distributions have no ACM ARN to follow.
- a9s-devops consultation — `alarm` discovery via CW `AWS/CloudFront` namespace + `DistributionId` dimension — a9s-devops (2026-04-20): possible=yes, worth=yes. Standard CW namespace, no extra API.
- a9s-devops consultation — `elb` discovery via name-match of `Origins[].DomainName` against ELB `DNSName` — a9s-devops (2026-04-20): possible=yes, worth=yes. No direct ARN field on the origin; name match is the standard cross-reference.
- a9s-devops consultation — `lambda` discovery via `LambdaFunctionAssociations` across default + all cache behaviors — a9s-devops (2026-04-20): possible=yes, worth=yes. Dedupe by function ARN since the same function may attach to multiple viewer events.
- a9s-devops consultation — `r53` discovery via reverse scan of loaded record sets where `AliasTarget.DNSName` matches the distribution's `DomainName` — a9s-devops (2026-04-20): possible=yes, worth=yes. Case-fold the comparison and tolerate trailing dot.
- a9s-devops consultation — `s3` discovery via `Origins[].DomainName` suffix match plus `LoggingConfig.Bucket` — a9s-devops (2026-04-20): possible=yes, worth=yes. Covers both the OAI/OAC origin case and the standard-log sink.
- a9s-devops consultation — `logs` discovery left as TBD — a9s-devops (2026-04-20): possible=partial, worth=yes-in-principle. Standard logging goes to S3 not CW Logs; real-time log configs require `GetRealtimeLogConfig` per configuration, which exceeds the bounded Wave 2 budget.

<!-- BEGIN GENERATED: header -->
cf — DNS & CDN. Lifecycle key: `status`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| cf.disabled | disabled (admin-off) | dim | wave1 | The distribution is switched off, so it serves nothing and the edge locations answer with an error. Nothing here needs fixing unless it was meant to be serving; enable it, or delete it once you are sure. |
| cf.status.in-progress | deploying: config propagating | warn | wave1 | A configuration change is still reaching the edge locations, so viewers may get the old behaviour or the new one depending on where they are. Wait for it to finish before judging anything else about the distribution. |
| cf.insecure-protocol | traffic allowed without TLS | warn | wave2 | — |
| cf.origin-bucket-missing | S3 origin bucket does not exist | broken | wave2 | The distribution forwards requests to a bucket that no longer exists, so those paths fail and anyone who creates a bucket with that name starts serving your traffic. Repoint the origin at a bucket you own, or remove it. |
| cf.deprecated-tls | minimum TLS below 1.2 | warn | wave2 | Viewers may negotiate a protocol version with known weaknesses, which modern browsers already refuse. Raise the distribution's minimum protocol version to TLS 1.2 or later. |
| cf.logging-off | access logging off | warn | wave2 | The distribution records no request logs, so an attack or abuse pattern at the edge leaves nothing to investigate. Turn on standard logging and give it a destination. |
| cf.no-default-root-object | no default root object | warn | wave2 | A request for the distribution root returns whatever the origin serves there, which can expose object names you did not mean to publish. Set a default root object such as index.html. |
| cf.s3-origin-no-oac | S3 origin without origin access control | warn | wave2 | The bucket behind this origin must be open to reach it through CloudFront, so viewers can bypass the distribution and read from the bucket directly. Attach an origin access control and restrict the bucket policy to it. |
| cf.default-certificate | uses the default CloudFront certificate | warn | wave2 | The distribution serves custom domains with the default CloudFront certificate, so viewers reaching those names get a certificate mismatch warning. Attach a certificate that covers the aliases. |
| cf.no-geo-restriction | no geo restriction | warn | wave2 | Content is served to every country, including any the account is not meant to serve. Add a geographic restriction if the distribution should be limited. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| s3 | S3 Buckets (origin) | yes |
| elb | Load Balancers (origin) | yes |
| waf | WAF Web ACLs | yes |
| acm | ACM Certificates | yes |
| r53 | Route 53 Zones | yes |
| alarm | CloudWatch Alarms | yes |
| lambda | Lambda@Edge | no |
| logs | Log Groups | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
