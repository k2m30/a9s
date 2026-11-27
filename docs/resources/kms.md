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
- **How discovered**: `GetKeyPolicy` (default policy) and `ListGrants` per opened key: the roles the key policy grants plus each grant's `GranteePrincipal` and `RetiringPrincipal` roles, cross-referenced against the already-loaded `role` list. Principals are those the policy's Allow statements grant, less any an unconditional Deny takes away; another account's principal is never matched against a local row, and a policy that does not parse leaves the pivot unknown.
- **Count shown**: yes.

### `ct-events`

- **Why related**: audit trail for key usage — Encrypt / Decrypt / GenerateDataKey / ScheduleKeyDeletion calls against this key show who touched it and when.
- **How discovered**: universal pivot — applies to every registered type; see `docs/related-resources.md` §Policy. `LookupEvents` by the key ID, and by the key's `Arn` beside it, since KMS records most calls against a key under its ARN.
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

- **Signal**: `KeyState==PendingDeletion` or `PendingReplicaDeletion`.
  - **State bucket**: Broken. The key is leaving: Wave 2 does not read its rotation or its policy.
  - **API call**: `DescribeKey` — one per key (N+1).
  - **Cost shape**: per-resource.

- **Signal**: `KeyState==Creating` or `Updating`.
  - **State bucket**: Warning. The state ends on its own.
  - **API call**: `DescribeKey` — one per key (N+1).
  - **Cost shape**: per-resource.

- **Signal**: `KeyState==PendingImport` or `Unavailable`.
  - **State bucket**: Broken.
  - **API call**: `DescribeKey` — one per key (N+1).
  - **Cost shape**: per-resource.

- **Signal**: `KeyState` is a value outside `KeyState.Values()`.
  - **State bucket**: Warning. The row shows the value AWS reported.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `DescribeKey` was denied for this key.
  - **State bucket**: Broken.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `DescribeKey` failed for this key for any reason but a denial (throttling past the retries, a service fault, an answer without `KeyMetadata`).
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call. `ListKeys` proved the key exists, so it keeps a row; a key `DescribeKey` reports gone was deleted after the list call and is dropped without a failure.

### 3.2 Wave 2 — bounded extra API calls

`GetKeyRotationStatus` and the key policy read on the second pass; `KeyState` is already on the row by then and its signals are in §3.1.

- **Signal**: `KeyRotationEnabled==false` on CMK that is not a multi-Region replica (`MultiRegionConfiguration.MultiRegionKeyType=REPLICA`): "You can enable and disable automatic key rotation only on a primary key" (AWS KMS Developer Guide, "Multi-Region keys in AWS KMS"). The detail of every multi-Region key shows `MultiRegionKeyType` (primary or replica), so the operator sees why one key carries this finding and another does not.
  - **State bucket**: Warning.
  - **API call**: `GetKeyRotationStatus` — one per key that can rotate automatically (`KeySpec=SYMMETRIC_DEFAULT`, `Origin=AWS_KMS`, no custom key store). A customer key that cannot rotate automatically gets the signal without the call.
  - **Cost shape**: per-resource.

- **Signal**: Key policy allows a wildcard principal with no restrictive condition.
  - **Explicit Deny**: a Deny statement that takes the grant from every caller on every granted resource clears the signal, and so do Deny statements that together fence both AWS service principals and every other caller to an account, organisation, VPC endpoint, address range or the principals a NotPrincipal block names. A conditional Deny that spares AWS service principals, or that names only part of the granted resources, does not clear it and is shown in the `Deny when` row. A condition that holds for a request without the key (a `ForAllValues:` operator), or that names the resource being called (`aws:ResourceAccount`, `aws:ResourceOrgID`, `aws:ResourceOrgPaths`, `s3:ResourceAccount`), scopes nobody. A policy that does not parse leaves the row not inspected, never flagged.
  - **State bucket**: Broken.
  - **API call**: `DescribeKey` — one per key (N+1).
  - **Cost shape**: per-resource.

- **Signal**: Key policy admits another account — names a principal in it, or a condition on a `*` or service-principal grant names it.
  - **Own account**: principals of the owning account are not foreign, and AWS service principals are never counted. When STS cannot name the session's account, the owning account is the one the key's ARN names; only when neither does is the row marked not inspected.
  - **Explicit Deny**: an account an unconditional Deny takes every granted action from is not counted.
  - **State bucket**: Warning.
  - **API call**: `GetKeyPolicy` — the same read as the wildcard-principal signal.
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
| `KeyState==PendingDeletion` or `PendingReplicaDeletion` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `pending deletion` |
| `KeyState==Creating` or `Updating` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `<key state> in progress` |
| `KeyState==PendingImport` or `Unavailable` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `<key state>` |
| `KeyState` outside `KeyState.Values()` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `<key state> (unrecognised)` |
| `DescribeKey` was denied for this key | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `access denied (kms:DescribeKey)` |
| `DescribeKey` failed for this key for any reason but a denial | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `details unavailable` |
| `KeyRotationEnabled==false` on CMK, not a replica | 2 | Warning | `~` | S2, S3, S4, S5 | `key rotation disabled` |
| Key policy allows a wildcard principal with no restrictive condition | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `key policy open to anyone` |
| Key policy admits another account — names a principal in it, or a condition on a `*` or service-principal grant names it | 2 | Warning | `~` | S2, S3, S4, S5 | `key policy grants another account` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`Disabled`, `PendingDeletion`, `Unavailable`) in the List text column is not acceptable. Pair it with a cause the operator can act on (`disabled: admin off`, `pending deletion`, `unavailable: custom key store offline`).
- For signals that legitimately have no operator-actionable cause (pure `Enabled` Healthy), omit the row from this table entirely; §3 still describes it.
- Keep the List text short enough to fit: ≤ 40 chars. The Detail cell quotes the finding's Detail constant verbatim, however long it is.

A policy document that does not parse, or a policy read that answers without a document, says nothing about who may call: the row reads `not inspected` instead of a verdict, and its detail names the reason, `policy does not parse` or `no policy document returned`.

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
- `s3` budget exclusion — `docs/related-resources.md` § Explicitly excluded.
- `role` target count is unknown pending key-policy enrichment — `docs/attention-signals.md § Not yet implemented`. a9s-devops persona (2026-04-20, persona fallback): possible=yes (via `GetKeyPolicy`), worth=deferred. Rationale: the key-policy JSON parse is the same work the deferred grant-level analysis scopes; surfacing the role list requires that enrichment to land first, so the panel shows the target with no count rather than hiding it.
- `ct-events` is the universal pivot — `docs/related-resources.md` § Policy (universal cross-reference via `resources[].ARN`).
- Read-only invariant — `docs/architecture.md` § opening paragraph ("a9s is a read-only terminal UI for AWS").
- S1–S5 surface rules and glyph constraints — `a9s-resource-spec` skill § "Allowed visualization surfaces (exactly five)".

<!-- BEGIN GENERATED: header -->
kms — SECRETS & CONFIG. Status key: `status` — the key the status cell reads, and the column naming it is the status column.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Applies to | Detail |
| --- | --- | --- | --- | --- | --- |
| kms.state.pending_deletion | pending deletion | broken | wave1 | all | The key is scheduled for destruction, and when the waiting period ends everything encrypted under it becomes permanently unreadable — snapshots, buckets, secrets and volumes alike. Cancel the deletion unless you have proven nothing still depends on it. |
| kms.state.disabled | disabled | warn | wave1 | all | The key cannot be used while it is disabled, so any read or write needing it fails now even though nothing has been destroyed. Re-enable it if that was not deliberate; the failures show up as permission errors in the services that use it. |
| kms.state.unavailable | <key state> | broken | wave1 | all | The key is not usable in the state it reports, and what to do depends on which state that is. A key awaiting imported material needs the material imported; one whose custom key store is disconnected needs the store reconnected before anything encrypted under it can be read. |
| kms.state.transitional | <key state> in progress | warn | wave1 | all | The key is being created or updated, for example a multi-Region replica being copied or a key in a custom key store being set up, and becomes usable on its own once AWS finishes. Wait it out before treating a failed encrypt or decrypt as a permission problem. |
| kms.state.unrecognised | <key state> (unrecognised) | warn | wave1 | all | AWS reports a key state a9s does not recognise, so a9s cannot tell whether the key can be used. Check the key in the AWS console before relying on it. |
| kms.access-denied | access denied (kms:DescribeKey) | broken | wave1 | all | This key's state could not be read because the key policy or your own IAM policy denies it, so nothing here can be judged — the key may be healthy or pending deletion and this view cannot tell you which. Grant the role you browse with permission to describe the key. |
| kms.warn.details_unavailable | details unavailable | warn | wave1 | all | The per-item describe call for this row failed, so a9s can show its name and nothing about its posture — the row is unjudged, not healthy. Retry the refresh; if it persists, check the service's health and whether the call is being throttled. |
| kms.rotation-disabled | key rotation disabled | warn | wave2 | automatic rotation, manual rotation, metadata not read | The key material never changes on a schedule, so anything that has ever been able to decrypt with this key still can. Turn on automatic rotation if this is a symmetric customer managed key with AWS-generated material, the only kind that supports it; for an asymmetric key, a message-authentication key, one with imported material, or one in a custom key store, plan a manual rotation to a new key and re-encrypt what depends on it. |
| kms.public-policy | key policy open to anyone | broken | wave2 | automatic rotation, manual rotation, multi-Region replica, metadata not read | The key policy allows any principal, so anyone can use this key, depending on the actions allowed. Replace the wildcard principal with the accounts or roles that need it, or add a condition that requires the caller's account or ARN to equal one you expect; a condition that only says whether a key is set scopes nothing. |
| kms.cross-account-policy | key policy grants another account | warn | wave2 | automatic rotation, manual rotation, multi-Region replica, metadata not read | The key policy admits a caller from another AWS account, so that account can use this key, depending on the actions allowed. Confirm each account belongs to a partner you meant to share with, and remove the rest from the key policy. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| ebs | EBS Volumes | yes |
| dbi | RDS Instances | yes |
| secrets | Secrets Manager | yes |
| role | IAM Roles (grants) | yes |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
