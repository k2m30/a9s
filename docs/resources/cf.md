---
shortName: cf
name: CloudFront Distributions
awsApiRef: https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_Distribution.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# cf — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `cf`
- **Display name**: CloudFront Distributions
- **AWS API reference**: https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_Distribution.html
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

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `Status == Deployed`.
  - **State bucket**: Healthy.
  - **How obtained**: `DistributionSummary.Status` field on the list response.
- **Signal**: `Status == InProgress`.
  - **State bucket**: Warning.
  - **How obtained**: `DistributionSummary.Status` field on the list response.
- **Signal**: `Enabled == false`.
  - **State bucket**: Dim.
  - **How obtained**: `DistributionSummary.Enabled` field on the list response.
- **Signal**: `ViewerCertificate.CloudFrontDefaultCertificate == false` AND `MinimumProtocolVersion` in `SSLv3` / `TLSv1` / `TLSv1_2016` / `TLSv1.1_2016`.
  - **State bucket**: Warning.
  - **How obtained**: `DistributionSummary.ViewerCertificate.MinimumProtocolVersion` field on the list response.
- **Signal**: `WebACLId == ""` (no WAF attached).
  - **State bucket**: Warning.
  - **How obtained**: `DistributionSummary.WebACLId` field on the list response.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: `LoggingConfig.Enabled == false` on the full distribution config.
  - **State bucket**: Warning.
  - **API call**: `GetDistributionConfig` — one call per distribution.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `5xxErrorRate` / `TotalErrorRate` metric-based attention.
- OUT OF SCOPE: Origin-deleted cross-check (detecting that an origin bucket / ALB / custom origin referenced by the distribution no longer exists).

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing" — e.g. maintenance scheduled, certificate expiring soon. `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `stopping: Server.SpotInstanceShutdown`, `expires in 7d`). **Healthy rows render blank** — no `OK` / `available` / `ACTIVE` / `running`. Empty means "nothing to see." |
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
| `Status == InProgress` | 1 | Warning | n/a | S2, S4 | `deploying: config propagating` | `Distribution config change is still propagating to edge locations.` |
| `Enabled == false` | 1 | Dim | n/a | S2, S4 | `disabled (admin-off)` | `Distribution is administratively disabled — not serving traffic.` |
| Weak TLS policy on aliased distribution | 1 | Warning | n/a | S2, S4 | `weak TLS: MinimumProtocolVersion=<v>` | `Viewer TLS policy allows deprecated protocols (SSLv3 / TLSv1 / TLSv1_2016 / TLSv1.1_2016).` |
| `WebACLId == ""` | 1 | Warning | n/a | S2, S4 | `no WAF attached` | `Distribution has no Web ACL in front of it — public surface without rate limiting.` |
| `LoggingConfig.Enabled == false` | 2 | Warning (on Healthy row) | `~` | S3, S4, S5 | `access logs off` | `Standard access logs are disabled — no S3 log trail for this distribution.` |

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
- a9s golden doc — Wave 1 signals for `cf` — `docs/attention-signals.md` § "DNS, CDN, Certs" row `cf` Wave 1.
- a9s golden doc — Wave 2 signal `Logging.Enabled==false` — `docs/attention-signals.md` § "DNS, CDN, Certs" row `cf` Wave 2.
- a9s golden doc — Wave 3 exclusions (5xxErrorRate, origin-deleted) — `docs/attention-signals.md` § "DNS, CDN, Certs" row `cf` Wave 3.
- a9s golden doc — read-only invariant — `docs/architecture.md` § "What is a9s?".
- a9s golden doc — universal pivot policy — `docs/related-resources.md` § Policy.
- AWS Go SDK v2 — `Status` field on list response — `AWS SDK Go v2 — cloudfront/types.DistributionSummary § Status`.
- AWS Go SDK v2 — `Enabled` field on list response — `AWS SDK Go v2 — cloudfront/types.DistributionSummary § Enabled`.
- AWS Go SDK v2 — `WebACLId` field on list response — `AWS SDK Go v2 — cloudfront/types.DistributionSummary § WebACLId`.
- AWS Go SDK v2 — `ViewerCertificate` + `CloudFrontDefaultCertificate` + `MinimumProtocolVersion` — `AWS SDK Go v2 — cloudfront/types.ViewerCertificate § CloudFrontDefaultCertificate, MinimumProtocolVersion, ACMCertificateArn`.
- AWS Go SDK v2 — `Origins.Items[].DomainName` (cross-reference target) — `AWS SDK Go v2 — cloudfront/types.Origin § DomainName`.
- AWS Go SDK v2 — `LoggingConfig.Enabled` + `Bucket` (Wave 2 access-log check and s3 sink pivot) — `AWS SDK Go v2 — cloudfront/types.LoggingConfig § Enabled, Bucket`.
- AWS API Reference (fallback) — `ListDistributions` return shape — AWS API Reference: ListDistributions § DistributionList (https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_ListDistributions.html).
- AWS API Reference (fallback) — `GetDistributionConfig` return shape — AWS API Reference: GetDistributionConfig § DistributionConfig (https://docs.aws.amazon.com/cloudfront/latest/APIReference/API_GetDistributionConfig.html).
- a9s-devops consultation — `acm` pivot only meaningful when not using CloudFront default cert — a9s-devops (2026-04-20): possible=yes, worth=yes. Default-cert distributions have no ACM ARN to follow.
- a9s-devops consultation — `alarm` discovery via CW `AWS/CloudFront` namespace + `DistributionId` dimension — a9s-devops (2026-04-20): possible=yes, worth=yes. Standard CW namespace, no extra API.
- a9s-devops consultation — `elb` discovery via name-match of `Origins[].DomainName` against ELB `DNSName` — a9s-devops (2026-04-20): possible=yes, worth=yes. No direct ARN field on the origin; name match is the standard cross-reference.
- a9s-devops consultation — `lambda` discovery via `LambdaFunctionAssociations` across default + all cache behaviors — a9s-devops (2026-04-20): possible=yes, worth=yes. Dedupe by function ARN since the same function may attach to multiple viewer events.
- a9s-devops consultation — `r53` discovery via reverse scan of loaded record sets where `AliasTarget.DNSName` matches the distribution's `DomainName` — a9s-devops (2026-04-20): possible=yes, worth=yes. Case-fold the comparison and tolerate trailing dot.
- a9s-devops consultation — `s3` discovery via `Origins[].DomainName` suffix match plus `LoggingConfig.Bucket` — a9s-devops (2026-04-20): possible=yes, worth=yes. Covers both the OAI/OAC origin case and the standard-log sink.
- a9s-devops consultation — `logs` discovery left as TBD — a9s-devops (2026-04-20): possible=partial, worth=yes-in-principle. Standard logging goes to S3 not CW Logs; real-time log configs require `GetRealtimeLogConfig` per configuration, which exceeds the bounded Wave 2 budget.
