---
shortName: ssm
name: SSM Parameters
awsApiRef: https://docs.aws.amazon.com/systems-manager/latest/APIReference/API_ParameterMetadata.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
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

Expected targets from `docs/related-resources.md` Per-type contract: `ct-events`, `kms`.

### `kms`

- **Why related**: `KeyId` — KMS key that encrypts a SecureString parameter. Operators rotating or restricting a KMS key need to see which parameters depend on it; operators opening a SecureString parameter need to know which key unlocks it.
- **How discovered**: read `ParameterMetadata.KeyId` on the parameter, then cross-reference the already-loaded `kms` list by key id or alias — a9s-devops: for SecureString parameters the KMS alias (`alias/aws/ssm` by default, or a customer-managed alias) is the direct field; for non-SecureString parameters the field is empty and no `kms` row is shown.
- **Count shown**: yes (either 0 or 1).

### `ct-events`

- **Why related**: Audit trail for parameter reads and writes — `GetParameter` / `PutParameter` / `DeleteParameter` / `GetParametersByPath` events are the forensic record of who touched this parameter and when. Critical for rotating a leaked SecureString or tracing a surprise change.
- **How discovered**: universal pivot — applies to every registered type; see related-resources.md §Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `Type==SecureString` AND `LastModifiedDate` >365d → Warning (stale secret — rotation overdue).
  - **State bucket**: Warning.
  - **How obtained**: `ParameterMetadata.Type` and `ParameterMetadata.LastModifiedDate` on the `DescribeParameters` list response.
- **Signal**: `Type==String` AND name suffix matches `-password` / `-secret` / `-token` or name contains `/secret` / `/password` / `/token` → Warning (should be SecureString — plaintext credential).
  - **State bucket**: Warning.
  - **How obtained**: `ParameterMetadata.Type` and `ParameterMetadata.Name` on the list response; pure string match against the name.
- **Signal**: `Tier==Advanced` AND `LastModifiedDate` >90d → Warning (cost — aged Advanced parameter; `$0.05/month` vs free Standard).
  - **State bucket**: Warning.
  - **How obtained**: `ParameterMetadata.Tier` and `ParameterMetadata.LastModifiedDate` on the list response.

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `GetParameterHistory` per parameter (true access-age — distinguishing "aged" from "unused" requires the history API, per-resource unbounded fan-out).

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". Never appears on yellow/red/dim rows. |
| S4 | Status / description column text | Short human-readable cause. Healthy rows render blank. |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4, S5.
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph on green row. S3, S4, S5. No S1.
- **Wave 2 finding on an already yellow/red/dim row** → S3 suppressed, S4 deduplicates, S5 carries full sentence, S1 still counts if `!`.

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `SecureString not rotated >365d` | 1 | Warning | n/a | S2 + S4 | `stale: not rotated in 365d+` | `SecureString parameter has not been modified in over a year — rotate or confirm still in use.` |
| `String name looks like a secret` | 1 | Warning | n/a | S2 + S4 | `plaintext: name looks like a secret` | `Parameter is type String but name suggests a credential — switch to SecureString.` |
| `Advanced tier aged >90d` | 1 | Warning | n/a | S2 + S4 | `advanced: aged 90d+ ($0.05/mo)` | `Advanced-tier parameter has not changed in 90d+ — downgrade to Standard if no Advanced features are used.` |

Rules for filling list and detail text:

- Banned words: `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- List text ≤ 40 chars, Detail text ≤ 100 chars. (All three list entries above fit.)
- Bare state keywords are not acceptable. Every row above pairs the condition with a cause.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for all three signals — the Status column explicitly names the cause (`stale: not rotated in 365d+`, `plaintext: name looks like a secret`, `advanced: aged 90d+ ($0.05/mo)`) so the operator can triage (rotate / retype / downgrade) without pressing detail. The detail line (S5) is a fuller sentence for operators who want the remediation action spelled out; the list line is the 3am-glance primary.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Per-parameter `GetParameter` / `GetParameterHistory` to read values or last-access timestamps — a9s-devops: not worth it for the default list view; belongs to a dedicated "reveal" action governed by the read-only invariant and explicit user intent, not to the attention/issue surfacing.
- Cross-account sharing, parameter-policy expiry, advanced policies (`ParameterInlinePolicy[]`) — a9s-devops: possible=yes (field is on `ParameterMetadata.Policies`) but worth=no for the default list view; policy expiry alerting is a niche feature better delivered via EventBridge / a dedicated advanced-policies view than via a list-row glyph.
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
- Wave 1 / Wave 2 / Wave 3 signal rows — `docs/attention-signals.md` § Secrets & Config, row `ssm`.
- List API `DescribeParameters` — `docs/attention-signals.md` § Secrets & Config, row `ssm` Source column.
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
- ct-events universal-pivot policy — `docs/related-resources.md` § Policy, item 4.
- Rephrasing `Tier==Advanced unused >90d` → `Tier==Advanced AND LastModifiedDate >90d` — `a9s-devops (2026-04-20): possible=no for "unused" at Wave 1, possible=yes for "aged". Rationale: DescribeParameters response (ParameterMetadata) exposes LastModifiedDate but not last-access timestamp; true access tracking lives in GetParameterHistory which is Wave 3. The Wave 1 signal as originally worded could not be implemented from the list response. Rephrasing to LastModifiedDate-age preserves the cost-hygiene intent and is achievable at Wave 1.` Amendment recorded in `docs/attention-signals.md` with an HTML comment on the same row.
- `kms` discovery mechanism (cross-reference already-loaded `kms` list by `KeyId` / alias) — `a9s-devops (2026-04-20): possible=yes, worth=yes. Rationale: ParameterMetadata.KeyId is populated only for SecureString; the default key alias alias/aws/ssm is well-known, and customer-managed aliases are resolvable against the kms list when loaded. Operators rotating a customer-managed key need the inverse lookup (which parameters depend on this key) which the related panel delivers.`
- Policies scope-out — `a9s-devops (2026-04-20): possible=yes (ParameterMetadata.Policies[] is present on the list response), worth=no for the default list view. Rationale: parameter policies (expiration, notification, no-change) are a niche power-user feature; alerting on policy expiry is better served by EventBridge or a dedicated advanced-policies view than by occupying S3/S4 real estate that all SSM rows would otherwise lose to a rarely-used signal.`
