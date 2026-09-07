---
shortName: kms
name: KMS Keys
awsApiRef: https://docs.aws.amazon.com/kms/latest/APIReference/API_KeyMetadata.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# kms — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `kms`
- **Display name**: KMS Keys
- **AWS API reference**: <https://docs.aws.amazon.com/kms/latest/APIReference/API_KeyMetadata.html>
- **List API**: `ListKeys` — returns `KeyListEntry{KeyId, KeyArn}` only (no state, no manager, no rotation info).
- **Describe API (if any)**: `DescribeKey` per key (returns `KeyMetadata`) plus `GetKeyRotationStatus` per key (returns `KeyRotationEnabled`, `RotationPeriodInDays`, `NextRotationDate`). Both are per-key N+1 calls. `DescribeKey` runs as the row is built, so every `KeyState` signal is Wave 1; `GetKeyRotationStatus` and the key policy are the Wave 2 pass.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` § Per-type contract: `ct-events`, `dbi`, `ebs`, `role`, `secrets`.

KMS is a **reverse-index pivot**: `KeyMetadata` carries no references to consumer resources. Every target below is discovered by scanning the already-loaded sibling-type list for entries whose `KmsKeyId` / `KmsKeyArn` / `SseKmsKeyId` / equivalent encryption-key field matches this key's `KeyId` or `Arn`. When the sibling list has not been loaded in the current sweep, the panel shows the target with an unknown count rather than a zero.

### `dbi`

- **Why related**: RDS instances using this key for at-rest storage encryption — answers "who encrypts with this key?" during a rotation or deletion review.
- **How discovered**: cross-reference the already-loaded `dbi` list by `DBInstance.KmsKeyId` matching this key's `Arn` (RDS stores the full key ARN, not the KeyId).
- **Count shown**: yes.

### `ebs`

- **Why related**: EBS volumes using this key — same rotation/deletion-impact question, scoped to block storage.
- **How discovered**: cross-reference the already-loaded `ebs` list by `Volume.KmsKeyId` matching this key's `Arn`.
- **Count shown**: yes.

### `secrets`

- **Why related**: Secrets Manager secrets encrypted with this key — a customer-managed KMS key protecting credentials is sensitive blast radius.
- **How discovered**: cross-reference the already-loaded `secrets` list by `SecretListEntry.KmsKeyId` matching this key's `KeyId` (the secrets listing carries only the UUID suffix; `docs/related-resources.md` § `kms` notes the "UUID suffix matched against KMS key cache").
- **Count shown**: yes.

### `role`

- **Why related**: IAM roles that the key policy trusts — answers "who can use this key?" during a permissions audit.
- **How discovered**: requires `GetKeyPolicy` per key and JSON-parsing the `Principal` / `AWS` entries to extract role ARNs, then cross-reference the already-loaded `role` list. No `kms` signal makes that call — key-policy analysis is deferred, `docs/attention-signals.md § Not yet implemented`. **Count unknown** until key-policy enrichment is wired; the panel shows the `role` target label with no number rather than hiding it.
- **Count shown**: unknown.

### `ct-events`

- **Why related**: audit trail for key usage — Encrypt / Decrypt / GenerateDataKey / ScheduleKeyDeletion calls against this key show who touched it and when.
- **How discovered**: universal pivot — applies to every registered type; see `related-resources.md` §Policy. Filtered by `resources[].ARN` matching this key's `Arn`.
- **Count shown**: unknown (event stream, not a bounded collection).

## 3. Attention / Issues Algorithm

**Source API**: [DescribeKey](https://docs.aws.amazon.com/kms/latest/APIReference/API_DescribeKey.html)

Transcribed from `docs/attention-signals.md § Signals § SECRETS & CONFIG` row `kms`.

### 3.1 Wave 1 — zero extra API calls

`ListKeys` returns `{KeyId, KeyArn}` only, so the fetcher reads each key with `DescribeKey` before it builds the row. The signals below are computed from that response as the row is built, with no second pass.

- **Signal**: `KeyState==Disabled`.
  - **State bucket**: Warning.
  - **API call**: `DescribeKey` — one per key (N+1).
  - **Cost shape**: per-resource.

- **Signal**: `KeyState==Creating`.
  - **State bucket**: Broken.
  - **API call**: `DescribeKey` — one per key (N+1).
  - **Cost shape**: per-resource.

- **Signal**: `KeyState==Updating`.
  - **State bucket**: Broken.
  - **API call**: `DescribeKey` — one per key (N+1).
  - **Cost shape**: per-resource.

- **Signal**: `KeyState==PendingDeletion`.
  - **State bucket**: Broken.
  - **API call**: `DescribeKey` — one per key (N+1).
  - **Cost shape**: per-resource.

- **Signal**: `KeyState==PendingImport`.
  - **State bucket**: Broken.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `KeyState==PendingReplicaDeletion`.
  - **State bucket**: Broken.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `KeyState==Unavailable`.
  - **State bucket**: Broken.
  - **API call**: `DescribeKey` — one per key (N+1).
  - **Cost shape**: per-resource.

- **Signal**: `DescribeKey` was denied for this key.
  - **State bucket**: Broken.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

### 3.2 Wave 2 — bounded extra API calls

`GetKeyRotationStatus` and the key policy read on the second pass; `KeyState` is already on the row by then and its signals are in §3.1.

- **Signal**: `KeyRotationEnabled==false` on CMK.
  - **State bucket**: Warning.
  - **API call**: `DescribeKey` — one per key (N+1).
  - **Cost shape**: per-resource.

- **Signal**: Default key policy allows a wildcard principal with no restrictive condition.
  - **State bucket**: Broken.
  - **API call**: `DescribeKey` — one per key (N+1).
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: Key-policy analysis per key (`Principal:*` detection).

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `kms`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `KeyState==Disabled` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `disabled` |
| `KeyState==PendingDeletion` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `pending deletion` |
| `KeyState==Creating` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `<key state>` |
| `KeyState==Updating` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `<key state>` |
| `KeyState==PendingImport` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `<key state>` |
| `KeyState==PendingReplicaDeletion` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `<key state>` |
| `KeyState==Unavailable` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `<key state>` |
| `DescribeKey` was denied for this key | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `access denied (kms:DescribeKey)` |
| `KeyRotationEnabled==false` on CMK | 2 | Warning | `~` | S2, S3, S4, S5 | `key rotation disabled` |
| Default key policy allows a wildcard principal with no restrictive condition | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `key policy open to anyone` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`Disabled`, `PendingDeletion`, `Unavailable`) in the List text column is not acceptable. Pair it with a cause the operator can act on (`disabled: admin off`, `pending deletion`, `unavailable: custom key store offline`).
- For signals that legitimately have no operator-actionable cause (pure `Enabled` Healthy), omit the row from this table entirely; §3 still describes it.
- Keep the List text short enough to fit: ≤ 40 chars. The Detail cell quotes the finding's Detail constant verbatim, however long it is.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes: every non-Healthy row carries a short cause in S4 — red for `pending deletion`, `key policy open to anyone` and `access denied (kms:DescribeKey)`, yellow for `disabled` and `key rotation disabled` — so the operator can triage "which key is about to strand encrypted data?" without pressing detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (key-policy `Principal:*` analysis).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- `role` related-panel count until key-policy enrichment is wired (recorded as `count: unknown` in §2, not a gap to backfill via reflection).

## 6. Citations

- List API returns `KeyId`/`KeyArn` only — `AWS SDK Go v2 — service/kms/types.KeyListEntry § KeyArn, KeyId`.
- Wave 1 `DescribeKey` per key for `KeyState` buckets (Enabled / Creating / Updating / Disabled / PendingDeletion / PendingImport / PendingReplicaDeletion / Unavailable) — `docs/attention-signals.md § Signals § SECRETS & CONFIG` row `kms`. Field confirmed: `AWS SDK Go v2 — service/kms/types.KeyMetadata § KeyState` (`KeyState` enum values match).
- Wave 2 `GetKeyRotationStatus` per key, `KeyRotationEnabled==false` on CMK → Warning — `docs/attention-signals.md § Signals § SECRETS & CONFIG` row `kms`. Field confirmed: `AWS SDK Go v2 — service/kms.GetKeyRotationStatusOutput § KeyRotationEnabled`.
- CMK = customer-managed key (`KeyManager==CUSTOMER`); AWS-managed keys excluded from rotation check because AWS rotates them automatically — `AWS SDK Go v2 — service/kms/types.KeyManagerType § KeyManagerTypeAws, KeyManagerTypeCustomer` (enum values AWS and CUSTOMER). a9s-devops persona (2026-04-20, persona fallback per skill §"Handling gaps"): possible=yes, worth=yes. Rationale: surfacing rotation-off on AWS-managed keys would be noise because the operator cannot change it and AWS has already taken responsibility; the signal is actionable only for keys the account owns.
- Related target discovery is reverse-index (sibling-list cross-reference on `KmsKeyId`/`KmsKeyArn`) for `dbi`, `ebs`, `secrets` — `docs/related-resources.md` § `kms` reasoning bullets (`StreamDescription.KeyId`, `Volume.KmsKeyId`, `SecretListEntry.KmsKeyId — UUID suffix matched against KMS key cache`). a9s-devops persona (2026-04-20, persona fallback): possible=yes, worth=yes. Rationale: `KeyMetadata` holds no consumer refs, so the pivot must traverse the other direction; these consumer types list their KMS key on the list-response shape, so no extra API call is needed when the sibling list is already loaded.
- `s3` budget exclusion — bucket encryption config is not on `ListBuckets` and not cached; per-bucket `GetBucketEncryption` fan-out exceeds the checker budget — `docs/related-resources.md` § Policy rule 7.
- `role` target count is unknown pending key-policy enrichment — `docs/attention-signals.md § Not yet implemented`. a9s-devops persona (2026-04-20, persona fallback): possible=yes (via `GetKeyPolicy`), worth=deferred. Rationale: the key-policy JSON parse is the same work the deferred grant-level analysis scopes; surfacing the role list requires that enrichment to land first, so the panel shows the target with no count rather than hiding it.
- `ct-events` is the universal pivot — `docs/related-resources.md` § Policy (universal cross-reference via `resources[].ARN`).
- Read-only invariant — `docs/architecture.md` § opening paragraph ("a9s is a read-only terminal UI for AWS").
- S1–S5 surface rules and glyph constraints — `a9s-resource-spec` skill § "Allowed visualization surfaces (exactly five)".

<!-- BEGIN GENERATED: header -->
kms — SECRETS & CONFIG. Lifecycle key: `status`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| kms.state.pending\_deletion | pending deletion | broken | wave1 | — |
| kms.state.disabled | disabled | warn | wave1 | — |
| kms.state.unavailable | <key state> | broken | wave1 | — |
| kms.access-denied | access denied (kms:DescribeKey) | broken | wave1 | — |
| kms.rotation-disabled | key rotation disabled | warn | wave2 | This customer-managed key never rotates its backing material, so every ciphertext ever written under it depends on one key that has been in use since creation. Enable automatic key rotation on the key. |
| kms.public-policy | key policy open to anyone | broken | wave2 | The key policy allows a wildcard principal, so any AWS account can use this key to decrypt data encrypted with it. Replace the "*" principal with the specific accounts or roles that need the key, or add a condition that requires the caller's account or ARN to equal one you expect; a condition that only names the service the request comes through, or only says whether a key is set, scopes nothing. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| ebs | EBS Volumes | yes |
| dbi | RDS Instances | yes |
| secrets | Secrets Manager | yes |
| role | IAM Roles (grants) | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
