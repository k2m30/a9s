---
shortName: acm
name: ACM Certificates
awsApiRef: https://docs.aws.amazon.com/acm/latest/APIReference/API_CertificateDetail.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# acm — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `acm`
- **Display name**: ACM Certificates
- **AWS API reference**: <https://docs.aws.amazon.com/acm/latest/APIReference/API_CertificateDetail.html>
- **List API**: `ListCertificates` — returns `CertificateSummary[]`. The SDK confirms `Status`, `NotAfter`, `NotBefore`, `InUse`, `DomainName`, `CertificateArn` are all on the summary shape, so every Wave 1 signal is reachable with zero extra calls.
- **Describe API (if any)**: `DescribeCertificate` per cert — used in Wave 2 to read `RenewalSummary.RenewalStatus` and `DomainValidationOptions[].ValidationStatus`, which are **not** on the summary shape.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `apigw`, `cf`, `elb`, `r53`, `ct-events`.

### `apigw`

- **Why related**: API Gateway custom domain using this certificate for TLS.
- **How discovered**: call `DescribeCertificate`, read `CertificateDetail.InUseBy[]`, keep entries whose ARN prefix is `arn:aws:apigateway:` — a9s-devops: `InUseBy` is the only ACM field that lists consuming resources; filter by ARN service prefix to split apigw/cf/elb without extra API calls. Cross-reference loaded `apigw` list by ARN.
- **Count shown**: yes.

### `cf`

- **Why related**: CloudFront distribution using this certificate as its SSL cert.
- **How discovered**: call `DescribeCertificate`, read `CertificateDetail.InUseBy[]`, keep entries whose ARN prefix is `arn:aws:cloudfront:` — a9s-devops: same `InUseBy` split as apigw. Cross-reference loaded `cf` list by distribution ARN.
- **Count shown**: yes.

### `elb`

- **Why related**: Load balancer listener using this certificate on an HTTPS listener.
- **How discovered**: call `DescribeCertificate`, read `CertificateDetail.InUseBy[]`, keep entries whose ARN prefix is `arn:aws:elasticloadbalancing:` — a9s-devops: same `InUseBy` split. Cross-reference loaded `elb` list by LoadBalancer ARN (the InUseBy entry references the listener, the LB ARN is the prefix of the listener ARN).
- **Count shown**: yes.

### `r53`

- **Why related**: Route 53 hosted zone that owns the certificate's domain — the zone where DNS validation records live and where the operator looks when validation stalls.
- **How discovered**: read `CertificateSummary.DomainName` on the resource; cross-reference the already-loaded `r53` list by hosted-zone `Name` using longest-suffix match (e.g. cert `*.api.example.com` pivots to zone `example.com` if `api.example.com` is not itself a zone) — a9s-devops: ACM surfaces the domain but not the owning zone; suffix matching against loaded hosted zones is the idiomatic pivot and requires no extra API call. For certificates with DNS validation, `DomainValidationOptions[].ResourceRecord.Name` from the describe response is a more precise hint when multiple zones could match.
- **Count shown**: yes (typically 1).

### `ct-events`

- **Why related**: Universal pivot — who issued, renewed, revoked, or imported this certificate.
- **How discovered**: pre-built CloudTrail query scoped to `CertificateArn` as the resource identifier.
- **Count shown**: unknown (CloudTrail queries are windowed; a reliable total isn't available without a separate count call).
- Universal pivot — applies to every registered type; see `related-resources.md` §Policy.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `Status == ISSUED`.
  - **State bucket**: Healthy.
  - **How obtained**: `CertificateSummary.Status` from `ListCertificates`.

- **Signal**: `Status == PENDING_VALIDATION`.
  - **State bucket**: Warning.
  - **How obtained**: `CertificateSummary.Status` from `ListCertificates`.

- **Signal**: `Status == EXPIRED`.
  - **State bucket**: Broken.
  - **How obtained**: `CertificateSummary.Status` from `ListCertificates`.

- **Signal**: `Status == REVOKED`.
  - **State bucket**: Broken.
  - **How obtained**: `CertificateSummary.Status` from `ListCertificates`.

- **Signal**: `Status == FAILED`.
  - **State bucket**: Broken.
  - **How obtained**: `CertificateSummary.Status` from `ListCertificates`.

- **Signal**: `Status == VALIDATION_TIMED_OUT`.
  - **State bucket**: Broken.
  - **How obtained**: `CertificateSummary.Status` from `ListCertificates`.

- **Signal**: `Status == INACTIVE`.
  - **State bucket**: Dim.
  - **How obtained**: `CertificateSummary.Status` from `ListCertificates`.

- **Signal**: `NotAfter - now() < 30 days` (and not already covered by the `< 7d` rule below).
  - **State bucket**: Warning.
  - **How obtained**: `CertificateSummary.NotAfter` from `ListCertificates`, compared to wall-clock time.

- **Signal**: `NotAfter - now() < 7 days`.
  - **State bucket**: Broken.
  - **How obtained**: `CertificateSummary.NotAfter` from `ListCertificates`, compared to wall-clock time. Overrides the `< 30d` Warning when both apply.

- **Signal**: `InUse == false` on a non-expired cert (orphan).
  - **State bucket**: Warning.
  - **How obtained**: `CertificateSummary.InUse` and `CertificateSummary.NotAfter` from `ListCertificates`.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: `RenewalSummary.RenewalStatus == FAILED`.
  - **State bucket**: Broken.
  - **API call**: `DescribeCertificate` per cert — one call per AMAZON_ISSUED certificate (field exists only when cert type is AMAZON_ISSUED).
  - **Cost shape**: per-resource.

- **Signal**: any `DomainValidationOptions[].ValidationStatus == FAILED`.
  - **State bucket**: Broken.
  - **API call**: same `DescribeCertificate` per cert as above — no additional call beyond what the renewal check already pays for.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

The golden doc's Wave 3 cell is `None` for this resource. Nothing to copy.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause. **Healthy rows render blank.** |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit).
- **Wave 1 Warning / Broken / Dim** → S2 + S4.
- **Wave 2 finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4, S5.
- **Wave 2 finding on a Healthy row, informational** → `~` glyph on green row. S3, S4, S5. No S1.
- **Wave 2 finding on an already yellow/red/dim row** → S3 suppressed, S4 deduplicates with existing cause, S5 carries the full sentence, S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `Status == PENDING_VALIDATION` | 1 | Warning | n/a | S2, S4 | `validating DNS` | `Certificate is waiting for DNS validation records to be published.` |
| `Status == EXPIRED` | 1 | Broken | n/a | S2, S4 | `expired` | `Certificate is past its NotAfter date and no longer valid for TLS.` |
| `Status == REVOKED` | 1 | Broken | n/a | S2, S4 | `revoked` | `Certificate was revoked by the CA; clients will reject it.` |
| `Status == FAILED` | 1 | Broken | n/a | S2, S4 | `issuance failed` | `Certificate issuance failed; see FailureReason on the detail view.` |
| `Status == VALIDATION_TIMED_OUT` | 1 | Broken | n/a | S2, S4 | `validation timed out` | `DNS validation records were not added within 72 hours; re-request the cert.` |
| `Status == INACTIVE` | 1 | Dim | n/a | S2, S4 | `inactive` | `Certificate is inactive — not used for any live resources.` |
| `NotAfter within 30 days` | 1 | Warning | n/a | S2, S4 | `expires in <N>d` | `Certificate expires in <N> days on <NotAfter>; renew or replace before then.` |
| `NotAfter within 7 days` | 1 | Broken | n/a | S2, S4 | `expires in <N>d` | `Certificate expires in <N> days on <NotAfter>; renew immediately.` |
| `InUse == false on non-expired cert` | 1 | Warning | n/a | S2, S4 | `not in use` | `Certificate is not attached to any resource; consider deleting if no longer needed.` |
| `RenewalSummary.RenewalStatus == FAILED` | 2 | Broken | `!` | S1, S3, S4, S5 | `auto-renewal failed` | `Automatic renewal failed — the cert will expire unless you re-validate or re-issue.` |
| `DomainValidationOptions[].ValidationStatus == FAILED` | 2 | Broken | n/a | S4, S5 | `validation failed: <domain>` | `Validation failed for domain <domain>; check the DNS record or request a new cert.` |

Notes:

- The `RenewalSummary.RenewalStatus == FAILED` finding is the canonical `!`-on-green case: a cert is currently `ISSUED` (green) but its next auto-renewal has already failed silently. Operators want the menu count, the glyph, and the detail line — classic ACM landmine.
- The `DomainValidationOptions` failure typically lands on a cert whose Wave 1 `Status` is already `PENDING_VALIDATION` (yellow). The row is already yellow, so S3 is suppressed. S4 replaces the generic `validating DNS` wording with the specific `validation failed: <domain>` cause.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every non-healthy cert carries a specific cause in S4 (`expires in 5d`, `issuance failed`, `validation timed out`, `auto-renewal failed`), and the `!` glyph on an otherwise-green cert is the only case where the cause lives in S5, reachable via the standard detail keypress. The only wording that could be tightened is the generic `issuance failed` for `Status == FAILED` — when `CertificateDetail.FailureReason` is available from a prior describe, the Wave 2 pass may refine S4 to `issuance failed: <FailureReason>`.

## 5. Out of Scope

- No Wave 3 signals are defined for this resource.
- Any UI element not listed in §4 — no new columns, icons, views, or key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- acm related-panel targets `apigw`, `cf`, `elb`, `r53`, `ct-events` — `docs/related-resources.md` § Per-type contract, row `acm`.
- acm Wave 1 signal set (`Status` enum mapping, `NotAfter` thresholds, `InUse` orphan) — `docs/attention-signals.md` § Signals, row `acm` Wave 1 cell.
- acm Wave 2 signals (`RenewalSummary.RenewalStatus`, `DomainValidationOptions[].ValidationStatus`) — `docs/attention-signals.md` § Signals, row `acm` Wave 2 cell.
- `Status`, `NotAfter`, `NotBefore`, `InUse`, `DomainName`, `CertificateArn` present on `ListCertificates` response — `AWS SDK Go v2 — service/acm/types.CertificateSummary § Status, NotAfter, InUse, DomainName, CertificateArn`.
- `InUseBy []string` field on describe response lists ARNs of consuming resources — `AWS SDK Go v2 — service/acm/types.CertificateDetail § InUseBy`.
- `RenewalSummary` present only on `AMAZON_ISSUED` certs — `AWS SDK Go v2 — service/acm/types.CertificateDetail § RenewalSummary`.
- `DomainValidationOptions []DomainValidation` present only on `AMAZON_ISSUED` certs — `AWS SDK Go v2 — service/acm/types.CertificateDetail § DomainValidationOptions`.
- Discovery of `apigw`, `cf`, `elb` via `InUseBy[]` ARN-prefix split — `a9s-devops (2026-04-20): possible=yes, worth=yes. InUseBy is the only ACM field listing consuming resources; splitting by ARN service prefix avoids extra API calls.`
- Discovery of `r53` via longest-suffix match of `DomainName` against loaded hosted-zone `Name` — `a9s-devops (2026-04-20): possible=yes, worth=yes. ACM surfaces the domain but not the owning zone; suffix matching is the idiomatic pivot and operators rely on it when validation stalls.`
- `ct-events` is the universal pivot applied to every registered type — `docs/related-resources.md` § Policy, rule 4.
- a9s is read-only — `docs/architecture.md` § "What is a9s?".
- Superseded HOW ignored — row middle-dot `·` marker, `⚠ Background Check` detail header, and derived list-level banner in `docs/enrichment-visibility.md` are not cited or reproduced per the skill's S1–S5 rules.

<!-- BEGIN GENERATED: header -->
acm — DNS & CDN. Lifecycle key: `status`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Approximate? |
| --- | --- | --- |
| cf | CloudFront Distros | yes |
| elb | Load Balancers | no |
| apigw | API Gateways | no |
| r53 | Route 53 Zones | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
