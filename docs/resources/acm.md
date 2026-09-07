---
shortName: acm
name: ACM Certificates
awsApiRef: https://docs.aws.amazon.com/acm/latest/APIReference/API_CertificateDetail.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
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

**Source API**: [ListCertificates](https://docs.aws.amazon.com/acm/latest/APIReference/API_ListCertificates.html)

Transcribed from `docs/attention-signals.md § Signals § DNS & CDN` row `acm`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `Status == PENDING_VALIDATION` — emits `acm.status.pending-validation`
  - **State bucket**: Warning.
  - **How obtained**: `CertificateSummary.Status` from `ListCertificates`.

- **Signal**: `Status == EXPIRED` — emits `acm.status.failed`
  - **State bucket**: Broken.
  - **How obtained**: `CertificateSummary.Status` from `ListCertificates`.

- **Signal**: `Status == REVOKED` — emits `acm.status.failed`
  - **State bucket**: Broken.
  - **How obtained**: `CertificateSummary.Status` from `ListCertificates`.

- **Signal**: `Status == FAILED` — emits `acm.status.failed`
  - **State bucket**: Broken.
  - **How obtained**: `CertificateSummary.Status` from `ListCertificates`.

- **Signal**: `Status == VALIDATION_TIMED_OUT` — emits `acm.status.failed`
  - **State bucket**: Broken.
  - **How obtained**: `CertificateSummary.Status` from `ListCertificates`.

- **Signal**: `Status == INACTIVE` — emits `acm.status.inactive`
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

- **Signal**: `KeyAlgorithm` is RSA below 2048 bits.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: `RenewalSummary.RenewalStatus == FAILED`. — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06)
  - **State bucket**: Broken.
  - **API call**: `DescribeCertificate` per cert — one call per AMAZON_ISSUED certificate (field exists only when cert type is AMAZON_ISSUED).
  - **Cost shape**: per-resource.

- **Signal**: any `DomainValidationOptions[].ValidationStatus == FAILED`. — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06)
  - **State bucket**: Broken.
  - **API call**: same `DescribeCertificate` per cert as above — no additional call beyond what the renewal check already pays for.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

Nothing is recorded out of scope for `acm`. Nothing to copy.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `acm`: Wave 1 issue-colored rows only — this type registers no Wave 2 enricher, so nothing else bumps the count.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `Status == PENDING_VALIDATION` — emits `acm.status.pending-validation` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `pending validation` |
| `Status == EXPIRED` — emits `acm.status.failed` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `expired` |
| `Status == REVOKED` — emits `acm.status.failed` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `<status, in words>` |
| `Status == FAILED` — emits `acm.status.failed` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `<status, in words>` |
| `Status == VALIDATION_TIMED_OUT` — emits `acm.status.failed` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `<status, in words>` |
| `Status == INACTIVE` — emits `acm.status.inactive` | 1 | Dim | n/a | S2, S4 | `inactive` |
| `NotAfter within 30 days` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `expires in <N> days` |
| `NotAfter within 7 days` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `expires in <N> days` |
| `InUse == false on non-expired cert` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `certificate not in use (orphan)` |
| `KeyAlgorithm` is RSA below 2048 bits | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `weak key algorithm` |
| `RenewalSummary.RenewalStatus == FAILED` — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | 2 | Broken | `!` | S1, S3, S4, S5 | `auto-renewal failed` |
| `DomainValidationOptions[].ValidationStatus == FAILED` — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | 2 | Broken | n/a | S4, S5 | `validation failed: <domain>` |

Notes:

- The `RenewalSummary.RenewalStatus == FAILED` condition is the ACM landmine: a cert reads `ISSUED` while its next auto-renewal has already failed silently. Were it implemented it would colour the row on its own, bump the menu count, and carry `!` and its sentence in the detail view.
- The `DomainValidationOptions` failure typically lands on a cert whose Wave 1 `Status` is already `PENDING_VALIDATION` (yellow). S4 replaces the generic `validating DNS` wording with the specific `validation failed: <domain>` cause.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — every non-healthy cert carries a specific cause in S4 (`expires in 5d`, `issuance failed`, `validation timed out`, `auto-renewal failed`), and the detail view carries the full sentence for each, one keypress away. The only wording that could be tightened is the generic `issuance failed` for `Status == FAILED` — when `CertificateDetail.FailureReason` is available from a prior describe, the Wave 2 pass may refine S4 to `issuance failed: <FailureReason>`.

## 5. Out of Scope

- No Wave 3 signals are defined for this resource.
- Any UI element not listed in §4 — no new columns, icons, views, or key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- acm related-panel targets `apigw`, `cf`, `elb`, `r53`, `ct-events` — `docs/related-resources.md` § Per-type contract, row `acm`.
- acm Wave 1 signal set (`Status` enum mapping, `NotAfter` thresholds, `InUse` orphan) — `docs/attention-signals.md § Signals § DNS & CDN` row `acm`.
- acm Wave 2 signals (`RenewalSummary.RenewalStatus`, `DomainValidationOptions[].ValidationStatus`) — `docs/attention-signals.md § Signals § DNS & CDN` row `acm`.
- `Status`, `NotAfter`, `NotBefore`, `InUse`, `DomainName`, `CertificateArn` present on `ListCertificates` response — `AWS SDK Go v2 — service/acm/types.CertificateSummary § Status, NotAfter, InUse, DomainName, CertificateArn`.
- `InUseBy []string` field on describe response lists ARNs of consuming resources — `AWS SDK Go v2 — service/acm/types.CertificateDetail § InUseBy`.
- `RenewalSummary` present only on `AMAZON_ISSUED` certs — `AWS SDK Go v2 — service/acm/types.CertificateDetail § RenewalSummary`.
- `DomainValidationOptions []DomainValidation` present only on `AMAZON_ISSUED` certs — `AWS SDK Go v2 — service/acm/types.CertificateDetail § DomainValidationOptions`.
- Discovery of `apigw`, `cf`, `elb` via `InUseBy[]` ARN-prefix split — `a9s-devops (2026-04-20): possible=yes, worth=yes. InUseBy is the only ACM field listing consuming resources; splitting by ARN service prefix avoids extra API calls.`
- Discovery of `r53` via longest-suffix match of `DomainName` against loaded hosted-zone `Name` — `a9s-devops (2026-04-20): possible=yes, worth=yes. ACM surfaces the domain but not the owning zone; suffix matching is the idiomatic pivot and operators rely on it when validation stalls.`
- `ct-events` is the universal pivot applied to every registered type — `docs/related-resources.md` § Policy, rule 4.
- a9s is read-only — `docs/architecture.md` § "What is a9s?".
- Superseded HOW ignored — row middle-dot `·` marker, `⚠ Background Check` detail header, and derived list-level banner in `docs/historical/analysis/enrichment-visibility.md` are not cited or reproduced per the skill's S1–S5 rules.

<!-- BEGIN GENERATED: header -->
acm — DNS & CDN. Lifecycle key: `status`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| acm.expired | expired | broken | wave1 | The certificate has already expired, so every client reaching a listener that serves it refuses the connection. Replace it and confirm the listeners have picked up the new one. |
| acm.expires-critical | expires in <N> days | broken | wave1 | The certificate expires within a week and every client reaching a listener that serves it will then refuse the connection. Renew or replace it now and confirm the listeners have picked up the new one. |
| acm.expires-soon | expires in <N> days | warn | wave1 | The certificate expires within a month, which is enough time to renew it calmly and not enough to forget about it. Check that automatic renewal is configured and that its validation records are still published. |
| acm.orphan | certificate not in use (orphan) | warn | wave1 | Nothing is serving this certificate, so it is renewed and tracked for no traffic, and it clutters the list an operator scans during an incident. Delete it, or attach it to the listener it was requested for. |
| acm.status.pending-validation | pending validation | warn | wave1 | The certificate has been requested but not issued: the domain is still waiting to be proved yours, so nothing can serve TLS with it yet. Publish the validation record in the domain's zone, or answer the validation email, before the request times out. |
| acm.status.failed | <status, in words> | broken | wave1 | The certificate cannot terminate TLS: it has expired, been revoked, failed issuance, or run out of time to validate, and the status says which. Anything still pointing at it is serving a broken handshake, so request a replacement and move the listeners onto it. |
| acm.status.inactive | inactive | dim | wave1 | An imported certificate marked inactive, because nothing is using it to terminate TLS. It costs nothing to keep, so this is a note rather than a fault; delete it once you are sure nothing will need it. |
| acm.weak-key | weak key algorithm | warn | wave1 | The certificate's key is short enough to be worth attacking, and browsers are withdrawing trust from keys this size. Reissue the certificate with a key of 2048 bits or more, or an elliptic-curve key. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| cf | CloudFront Distros | yes |
| elb | Load Balancers | no |
| apigw | API Gateways | no |
| r53 | Route 53 Zones | yes |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
