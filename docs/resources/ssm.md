---
shortName: ssm
name: SSM Parameters
awsApiRef: https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_ParameterMetadata.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# ssm — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `ssm`
- **Display name**: SSM Parameters
- **AWS API reference**: <https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_ParameterMetadata.html>
- **List API**: `DescribeParameters`
- **Describe API (if any)**: not used (the list response is `[]ParameterMetadata` — name, type, tier, KeyId, LastModifiedDate, LastModifiedUser all present; no per-parameter Describe needed for Wave 1/2)

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `ct-events`, `kms`.

### `kms`

- **Why related**: `KeyId` — KMS key that encrypts a SecureString parameter. Operators rotating or restricting a KMS key need to see which parameters depend on it; operators opening a SecureString parameter need to know which key unlocks it.
- **How discovered**: read `ParameterMetadata.KeyId` on the parameter, then cross-reference the already-loaded `kms` list by key id or alias — a9s-devops: for SecureString parameters the KMS alias (`alias/aws/ssm` by default, or a customer-managed alias) is the direct field; for non-SecureString parameters the field is empty and no `kms` row is shown.
- **Count shown**: yes (either 0 or 1).

### `ct-events`

- **Why related**: Audit trail for parameter reads and writes — `GetParameter` / `PutParameter` / `DeleteParameter` / `GetParametersByPath` events are the forensic record of who touched this parameter and when. Critical for rotating a leaked SecureString or tracing a surprise change.
- **How discovered**: universal pivot — applies to every registered type; see docs/related-resources.md §Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeParameters](https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_DescribeParameters.html)

Transcribed from `docs/attention-signals.md § Signals § SECRETS & CONFIG` row `ssm`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `Type==SecureString` AND `LastModifiedDate` >365d → Warning (stale secret — rotation overdue).
  - **State bucket**: Warning.
  - **How obtained**: `ParameterMetadata.Type` and `ParameterMetadata.LastModifiedDate` on the `DescribeParameters` list response.

- **Signal**: `Type==String` AND name suffix matches `-password` / `-secret` / `-token` or name contains `/secret` / `/password` / `/token` → Broken (should be SecureString — plaintext credential).
  - **State bucket**: Broken.
  - **How obtained**: `ParameterMetadata.Type` and `ParameterMetadata.Name` on the list response; pure string match against the name.

- **Signal**: `Tier==Advanced` AND `LastModifiedDate` >90d → Warning (cost — aged Advanced parameter; `$0.05/month` vs free Standard). — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06)
  - **State bucket**: Warning.
  - **How obtained**: `ParameterMetadata.Tier` and `ParameterMetadata.LastModifiedDate` on the list response.

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `GetParameterHistory` per parameter (true access-age — distinguishing "aged" from "unused" requires the history API, per-resource unbounded fan-out).

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `ssm`: Wave 1 issue-colored rows only — this type registers no Wave 2 enricher, so nothing else bumps the count.
<!-- END GENERATED: badge -->

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `SecureString not rotated >365d` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `not modified in over 365 days` |
| `String name looks like a secret` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `plaintext value looks like a credential` |
| `Advanced tier aged >90d` — NOT IMPLEMENTED (backlog; no emission in code as of 2026-07-06) | 1 | Warning | n/a | S2 + S4 | `advanced: aged 90d+ ($0.05/mo)` |

Rules for filling list and detail text:

- Banned words: `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- Keep the List text short enough to fit: ≤ 40 chars (all three list entries above fit). The Detail cell quotes the finding's Detail constant verbatim, however long it is.
- Bare state keywords are not acceptable. Every row above pairs the condition with a cause.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for both signals — the Status column explicitly names the cause (`not modified in over 365 days`, `plaintext value looks like a credential`) so the operator can triage (rotate or retype) without pressing detail. The detail line (S5) is a fuller sentence for operators who want the remediation action spelled out; the list line is the 3am-glance primary.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Per-parameter `GetParameter` / `GetParameterHistory` to read values or last-access timestamps — a9s-devops: not worth it for the default list view; belongs to a dedicated "reveal" action governed by the read-only invariant and explicit user intent, not to the attention/issue surfacing.
- Cross-account sharing, parameter-policy expiry, advanced policies (`ParameterInlinePolicy[]`) — a9s-devops: possible=yes (field is on `ParameterMetadata.Policies`) but worth=no for the default list view; policy expiry alerting is a niche feature better delivered via EventBridge / a dedicated advanced-policies view than via a finding on the parameter row.
- Any UI element not listed in §4 — no new columns, no new icons, no new views, no new key bindings.
- Any write operation. a9s is read-only by design (`docs/architecture.md` § "What is a9s?").

## 6. Citations

- Per-type contract `ssm` → `ct-events`, `kms` — `docs/related-resources.md` § Per-type contract, row `ssm`.
- AWS API URL for `ssm` — `docs/related-resources.md` § `ssm` (`https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_ParameterMetadata.html`).
- `kms` relation reason ("KeyId — KMS key for SecureString") — `docs/related-resources.md` § `ssm`.
- `ct-events` relation reason ("Audit trail for parameter reads/writes") — `docs/related-resources.md` § `ssm`.
- `KeyId` field presence on `ParameterMetadata` — `AWS SDK Go v2 — service/ssm/types.ParameterMetadata § KeyId`.
- `Type`, `Tier`, `LastModifiedDate`, `Name` field presence on `ParameterMetadata` — `AWS SDK Go v2 — service/ssm/types.ParameterMetadata § Type, Tier, LastModifiedDate, Name`.
- `ParameterType` enum (`String`, `StringList`, `SecureString`) — `AWS SDK Go v2 — service/ssm/types.ParameterType`.
- `ParameterTier` enum (includes `Standard`, `Advanced`, `Intelligent-Tiering`) — `AWS SDK Go v2 — service/ssm/types.ParameterTier`.
- The `ssm` signals — `docs/attention-signals.md § Signals § SECRETS & CONFIG` row `ssm`; the deferred `GetParameterHistory` read — `docs/attention-signals.md § Not yet implemented`.
- List API `DescribeParameters` — `core/aws/ssm.go`.
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
- ct-events universal-pivot policy — `docs/related-resources.md` § Policy, item 4.
- Rephrasing `Tier==Advanced unused >90d` → `Tier==Advanced AND LastModifiedDate >90d` — `a9s-devops (2026-04-20): possible=no for "unused", possible=yes for "aged". Rationale: the DescribeParameters response (ParameterMetadata) exposes LastModifiedDate but not a last-access timestamp; true access tracking lives in GetParameterHistory. The signal as originally worded could not be implemented from the list response. Rephrasing to LastModifiedDate-age preserves the cost-hygiene intent.` The `GetParameterHistory` read stays deferred — `docs/attention-signals.md § Not yet implemented`.
- `kms` discovery mechanism (cross-reference already-loaded `kms` list by `KeyId` / alias) — `a9s-devops (2026-04-20): possible=yes, worth=yes. Rationale: ParameterMetadata.KeyId is populated only for SecureString; the default key alias alias/aws/ssm is well-known, and customer-managed aliases are resolvable against the kms list when loaded. Operators rotating a customer-managed key need the inverse lookup (which parameters depend on this key) which the related panel delivers.`
- Policies scope-out — `a9s-devops (2026-04-20): possible=yes (ParameterMetadata.Policies[] is present on the list response), worth=no for the default list view. Rationale: parameter policies (expiration, notification, no-change) are a niche power-user feature; alerting on policy expiry is better served by EventBridge or a dedicated advanced-policies view than by occupying S3/S4 real estate that all SSM rows would otherwise lose to a rarely-used signal.`

<!-- BEGIN GENERATED: header -->
ssm — SECRETS & CONFIG. Lifecycle key: none (the list API returns no lifecycle field).
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| ssm.value.plaintext-sensitive | plaintext value looks like a credential | broken | wave1 | — |
| ssm.value.stale | not modified in over 365 days | warn | wave1 | — |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| kms | KMS Key | yes |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
