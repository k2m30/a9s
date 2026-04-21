---
shortName: kms
name: KMS Keys
awsApiRef: https://docs.aws.amazon.com/kms/latest/APIReference/API_KeyMetadata.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# kms — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `kms`
- **Display name**: KMS Keys
- **AWS API reference**: <https://docs.aws.amazon.com/kms/latest/APIReference/API_KeyMetadata.html>
- **List API**: `ListKeys` — returns `KeyListEntry{KeyId, KeyArn}` only (no state, no manager, no rotation info).
- **Describe API (if any)**: `DescribeKey` per key (returns `KeyMetadata`) plus `GetKeyRotationStatus` per key (returns `KeyRotationEnabled`, `RotationPeriodInDays`, `NextRotationDate`). Both are per-key N+1 calls — all KMS signals are Wave 2.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `ct-events`, `dbi`, `ebs`, `role`, `s3`, `secrets`.

KMS is a **reverse-index pivot**: `KeyMetadata` carries no references to consumer resources. Every target below is discovered by scanning the already-loaded sibling-type list for entries whose `KmsKeyId` / `KmsKeyArn` / `SseKmsKeyId` / equivalent encryption-key field matches this key's `KeyId` or `Arn`. When the sibling list has not been loaded in the current sweep, the panel shows the target with an unknown count rather than a zero.

### `dbi`

- **Why related**: RDS instances using this key for at-rest storage encryption — answers "who encrypts with this key?" during a rotation or deletion review.
- **How discovered**: cross-reference the already-loaded `dbi` list by `DBInstance.KmsKeyId` matching this key's `Arn` (RDS stores the full key ARN, not the KeyId).
- **Count shown**: yes.

### `ebs`

- **Why related**: EBS volumes using this key — same rotation/deletion-impact question, scoped to block storage.
- **How discovered**: cross-reference the already-loaded `ebs` list by `Volume.KmsKeyId` matching this key's `Arn`.
- **Count shown**: yes.

### `s3`

- **Why related**: S3 buckets using this key for SSE-KMS default encryption — one of the highest-blast-radius consumers; deleting the key bricks every object encrypted with it.
- **How discovered**: cross-reference the already-loaded `s3` list by the bucket's `ServerSideEncryptionConfiguration.Rules[].ApplyServerSideEncryptionByDefault.KMSMasterKeyID` matching this key's `Arn`. Bucket encryption config is not on `ListBuckets`; this pivot is meaningful only after buckets have been enriched via `GetBucketEncryption` (Wave 3 for `s3`).
- **Count shown**: yes.

### `secrets`

- **Why related**: Secrets Manager secrets encrypted with this key — a customer-managed KMS key protecting credentials is sensitive blast radius.
- **How discovered**: cross-reference the already-loaded `secrets` list by `SecretListEntry.KmsKeyId` matching this key's `KeyId` (the secrets listing carries only the UUID suffix; related-resources.md notes the "UUID suffix matched against KMS key cache").
- **Count shown**: yes.

### `role`

- **Why related**: IAM roles that the key policy trusts — answers "who can use this key?" during a permissions audit.
- **How discovered**: requires `GetKeyPolicy` per key and JSON-parsing the `Principal` / `AWS` entries to extract role ARNs, then cross-reference the already-loaded `role` list. `GetKeyPolicy` is not part of the Wave 2 set in `docs/attention-signals.md` (that document treats key-policy analysis as Wave 3: "Key-policy analysis per key (`Principal:*` detection)"). **Count unknown** until key-policy enrichment is wired; the panel shows the `role` target label with no number rather than hiding it.
- **Count shown**: unknown.

### `ct-events`

- **Why related**: audit trail for key usage — Encrypt / Decrypt / GenerateDataKey / ScheduleKeyDeletion calls against this key show who touched it and when.
- **How discovered**: universal pivot — applies to every registered type; see `related-resources.md` §Policy. Filtered by `resources[].ARN` matching this key's `Arn`.
- **Count shown**: unknown (event stream, not a bounded collection).

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — `ListKeys` returns `{KeyId, KeyArn}` only; neither state, manager type, nor rotation status is exposed by the list API.

### 3.2 Wave 2 — bounded extra API calls

All KMS attention signals are Wave 2. Two per-key calls are needed: `DescribeKey` (for `KeyState`) and `GetKeyRotationStatus` (for `KeyRotationEnabled`).

- **Signal**: `KeyState==Enabled`.
  - **State bucket**: Healthy.
  - **API call**: `DescribeKey` — one per key (N+1).
  - **Cost shape**: per-resource.
- **Signal**: `KeyState==Creating` or `KeyState==Updating`.
  - **State bucket**: Warning.
  - **API call**: `DescribeKey` — one per key (N+1).
  - **Cost shape**: per-resource.
- **Signal**: `KeyState==Disabled`.
  - **State bucket**: Warning.
  - **API call**: `DescribeKey` — one per key (N+1).
  - **Cost shape**: per-resource.
- **Signal**: `KeyState==PendingDeletion` or `KeyState==PendingImport` or `KeyState==PendingReplicaDeletion`.
  - **State bucket**: Broken.
  - **API call**: `DescribeKey` — one per key (N+1).
  - **Cost shape**: per-resource.
- **Signal**: `KeyState==Unavailable`.
  - **State bucket**: Broken.
  - **API call**: `DescribeKey` — one per key (N+1).
  - **Cost shape**: per-resource.
- **Signal**: `KeyRotationEnabled==false` on a customer-managed key (`KeyManager==CUSTOMER`).
  - **State bucket**: Healthy row + background concern (`!` glyph — rotation is a security must-have for CMKs; AWS-managed keys rotate automatically and are excluded).
  - **API call**: `GetKeyRotationStatus` — one per key (N+1).
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: Key-policy analysis per key (`Principal:*` detection).

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing" — e.g. rotation disabled, maintenance scheduled. `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `disabled: admin off`, `pending deletion in 7d`). **Healthy rows render blank** — no `OK` / `Enabled`. Empty means "nothing to see." |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping:

- **Wave 1 Healthy** → n/a (no Wave 1 signals for kms).
- **Wave 2 Healthy** (`KeyState==Enabled`, rotation on) → omit from §4; S2 renders green, S4 renders blank. Silence is the UX.
- **Wave 2 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5 for the state-bucket row itself.
- **Wave 2 background finding on a Healthy row, important** (`KeyRotationEnabled==false` on CMK with `KeyState==Enabled`) → `!` glyph on green row. S1, S3, S4, S5.
- **Wave 2 finding on an already yellow/red row** (e.g. rotation disabled on a `Disabled` key) → redundant with color; S3 suppressed, S4 deduplicates with existing cause, S5 still carries the full sentence, S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `KeyState==Creating` | 2 | Warning | n/a | S2, S4 | `creating` | `Key is being created; not yet usable for cryptographic operations.` |
| `KeyState==Updating` | 2 | Warning | n/a | S2, S4 | `updating` | `Key material is being updated; brief availability window.` |
| `KeyState==Disabled` | 2 | Warning | n/a | S2, S4 | `disabled: admin off` | `Key is disabled by an administrator; cannot encrypt or decrypt until re-enabled.` |
| `KeyState==PendingDeletion` | 2 | Broken | n/a | S2, S4 | `pending deletion` | `Key is scheduled for deletion; resources encrypted with it will become unrecoverable.` |
| `KeyState==PendingImport` | 2 | Broken | n/a | S2, S4 | `awaiting key material` | `Key has no material imported yet; cannot encrypt or decrypt.` |
| `KeyState==PendingReplicaDeletion` | 2 | Broken | n/a | S2, S4 | `pending replica deletion` | `Multi-Region replica is scheduled for deletion; primary still holds the material.` |
| `KeyState==Unavailable` | 2 | Broken | n/a | S2, S4 | `unavailable: custom key store offline` | `Custom key store (CloudHSM or external) is disconnected; key cannot be used.` |
| `KeyRotationEnabled==false` on CMK | 2 | Healthy + `!` | `!` | S1, S3, S4, S5 | `rotation off` | `Customer-managed key has automatic rotation disabled; enable annual rotation for compliance.` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`Disabled`, `PendingDeletion`, `Unavailable`) in the List text column is not acceptable. Pair it with a cause the operator can act on (`disabled: admin off`, `pending deletion`, `unavailable: custom key store offline`).
- For signals that legitimately have no operator-actionable cause (pure `Enabled` Healthy), omit the row from this table entirely; §3 still describes it.
- Keep both columns short enough to fit: List text ≤ 40 chars, Detail text ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes: every non-Healthy row carries a short cause in S4 (`pending deletion`, `disabled: admin off`, `unavailable: custom key store offline`) and every green `!` row carries `rotation off` — the operator can triage "which key is about to strand encrypted data?" without pressing detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (key-policy `Principal:*` analysis).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- `role` related-panel count until key-policy enrichment is wired (recorded as `count: unknown` in §2, not a gap to backfill via reflection).

## 6. Citations

- List API returns `KeyId`/`KeyArn` only — `docs/attention-signals.md` § Secrets & Config row `kms` ("None — `ListKeys` returns `{KeyId, KeyArn}` only"). Confirmed: `AWS SDK Go v2 — service/kms/types.KeyListEntry § KeyArn, KeyId`.
- Wave 2 `DescribeKey` per key for `KeyState` buckets (Enabled / Creating / Updating / Disabled / PendingDeletion / PendingImport / PendingReplicaDeletion / Unavailable) — `docs/attention-signals.md` § Secrets & Config row `kms`. Field confirmed: `AWS SDK Go v2 — service/kms/types.KeyMetadata § KeyState` (`KeyState` enum values match).
- Wave 2 `GetKeyRotationStatus` per key, `KeyRotationEnabled==false` on CMK → Warning — `docs/attention-signals.md` § Secrets & Config row `kms`. Field confirmed: `AWS SDK Go v2 — service/kms.GetKeyRotationStatusOutput § KeyRotationEnabled`.
- CMK = customer-managed key (`KeyManager==CUSTOMER`); AWS-managed keys excluded from rotation check because AWS rotates them automatically — `AWS SDK Go v2 — service/kms/types.KeyManagerType § KeyManagerTypeAws, KeyManagerTypeCustomer` (enum values AWS and CUSTOMER). a9s-devops persona (2026-04-20, persona fallback per skill §"Handling gaps"): possible=yes, worth=yes. Rationale: surfacing rotation-off on AWS-managed keys would be noise because the operator cannot change it and AWS has already taken responsibility; the signal is actionable only for keys the account owns.
- Related target discovery is reverse-index (sibling-list cross-reference on `KmsKeyId`/`KmsKeyArn`) for `dbi`, `ebs`, `s3`, `secrets` — `docs/related-resources.md` § `kms` reasoning bullets (`StreamDescription.KeyId`, `Volume.KmsKeyId`, `Bucket SSE-KMS key`, `SecretListEntry.KmsKeyId — UUID suffix matched against KMS key cache`). a9s-devops persona (2026-04-20, persona fallback): possible=yes, worth=yes. Rationale: `KeyMetadata` holds no consumer refs, so the pivot must traverse the other direction; all four consumer types list their KMS key on the list-response shape, so no extra API call is needed when the sibling list is already loaded.
- `role` target count is unknown pending key-policy enrichment — `docs/attention-signals.md` § Secrets & Config row `kms` Wave 3 ("Key-policy analysis per key (`Principal:*` detection)"). a9s-devops persona (2026-04-20, persona fallback): possible=yes (via `GetKeyPolicy`), worth=deferred. Rationale: the key-policy JSON parse is the same work Wave 3 already scopes; surfacing the role list requires that enrichment to land first, so the panel shows the target with no count rather than hiding it.
- `ct-events` is the universal pivot — `docs/related-resources.md` § Policy (universal cross-reference via `resources[].ARN`).
- Read-only invariant — `docs/architecture.md` § opening paragraph ("a9s is a read-only terminal UI for AWS").
- S1–S5 surface rules and glyph constraints — `a9s-resource-spec` skill § "Allowed visualization surfaces (exactly five)".
