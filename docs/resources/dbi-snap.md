---
shortName: dbi-snap
name: DB Instance Snapshots
awsApiRef: https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBSnapshot.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# dbi-snap — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `dbi-snap`
- **Display name**: DB Instance Snapshots
- **AWS API reference**: <https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBSnapshot.html>
- **List API**: `DescribeDBSnapshots`
- **Describe API (if any)**: not used — all Wave 1 signals are carried on the list response; Wave 2 is `None` per `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `dbi-snap`.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `backup`, `dbi`, `kms`, `ct-events`.

### `dbi`

- **Why related**: the source DB instance this snapshot was taken from — the operator's first question ("where did this come from, is it still alive?") is always about the parent instance. Citation: `related-resources.md § dbi-snap` ("Source DB instance").
- **How discovered**: read `DBSnapshot.DBInstanceIdentifier` from the list response, then cross-reference the already-loaded `dbi` list by that identifier. No extra API call. Citation: `AWS SDK Go v2 — rds/types.DBSnapshot § DBInstanceIdentifier`.
- **Count shown**: yes (0 or 1 — a snapshot has exactly one source instance; 0 when the parent has been deleted, which is itself the orphan signal in §3.1).

### `kms`

- **Why related**: the encryption key protecting the snapshot. If the key is disabled or pending deletion, the snapshot cannot be restored — a silent restore-blocker the operator needs to catch early. Citation: `related-resources.md § dbi-snap` ("Encryption key").
- **How discovered**: read `DBSnapshot.KmsKeyId` from the list response, then cross-reference the already-loaded `kms` list by KeyId/KeyArn. No extra API call. Citation: `AWS SDK Go v2 — rds/types.DBSnapshot § KmsKeyId`.
- **Count shown**: yes (0 or 1 — one key per encrypted snapshot; 0 when `Encrypted==false`).

### `dbc` (intentionally absent)

`dbi-snap` does NOT register a `dbc` pivot. Real AWS rejects `CreateDBSnapshot`
on Aurora cluster members — Aurora cluster snapshots live in `dbc-snap`
(`DBClusterSnapshot`), which has its own pivots. A registered `dbi-snap → dbc`
pivot would always resolve `Count=0` (an `dbi-snap` is never associated with a
`DBCluster` in real AWS), which is dead UX. See `core/aws/dbi_snap.go` for
the structural exclusion.

### `backup`

- **Why related**: AWS Backup can create RDS snapshots on behalf of a backup plan; knowing whether a snapshot was produced by AWS Backup (vs automated by the DB instance or manual) tells the operator which retention policy governs its lifecycle and which audit trail applies. Citation: `related-resources.md § dbi-snap` ("Snapshots covered by AWS Backup").
- **How discovered**: a9s-devops persona (2026-04-20): possible=yes, worth=yes (narrow). AWS Backup-created RDS snapshots carry the identifier prefix `awsbackup:job-<uuid>` on `DBSnapshotIdentifier`; AWS Backup records the snapshot ARN on its recovery-point list (`backup:ListRecoveryPointsByResource` with the snapshot or parent-instance ARN). The cheap Wave-1-safe path is a string-prefix match on `DBSnapshotIdentifier` — no extra API call required. Rationale (per `docs/historical/019-related-panel/related-panel-devops-consensus.md § dbi-snap → backup`): AWS Backup tracks the parent DB instance rather than each manual snapshot individually, so a live cross-API call is high-cost for thin value; the identifier prefix is free on the list response and answers the same operator question.
- **Count shown**: yes (0 or 1 — a snapshot is either a Backup-created recovery point or it is not).

### `ct-events`

- **Why related**: universal pivot — every registered type carries a CloudTrail pivot for audit. For RDS snapshots the canonical operator questions are "who deleted this snapshot" (only backup lost), "who shared this snapshot" (data exfiltration via `ModifyDBSnapshotAttribute`), and "who copied this snapshot" (`CopyDBSnapshot` — cross-region DR is fine, cross-account needs scrutiny). See `related-resources.md §Policy`.
- **How discovered**: `LookupEvents` with `LookupAttributes=[{AttributeKey=ResourceName,AttributeValue=<DBSnapshotIdentifier>}]` — universal pivot, applies to every registered type; see `related-resources.md §Policy`. Called on demand when the operator opens the pivot, not on list load.
- **Count shown**: unknown — a9s-devops persona (2026-04-20): `LookupEvents` returns windowed results; the panel typically shows a page rather than a total count, so "N" would be misleading.

## 3. Attention / Issues Algorithm

**Source API**: [DescribeDBSnapshots](https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBSnapshots.html)

Transcribed from `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `dbi-snap`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

An `available` snapshot with nothing else wrong raises no signal and renders
green and blank.

- **Signal**: `Status == "creating"` → Warning.
  - **State bucket**: Warning.
  - **How obtained**: `DBSnapshot.Status` on the `DescribeDBSnapshots` response.

- **Signal**: `Status` is any other value that is neither `available` nor an enumerated broken state (e.g. `copying`, `pending`) → Warning. The snapshot cannot be restored from yet, and passing the keyword through keeps a state AWS adds later visible instead of reading as ready.
  - **State bucket**: Warning.
  - **How obtained**: `DBSnapshot.Status` on the `DescribeDBSnapshots` response.

- **Signal**: `Status == "failed"` → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `DBSnapshot.Status` on the `DescribeDBSnapshots` response.

- **Signal**: `Status` matches `incompatible-*` (e.g. `incompatible-restore`, `incompatible-parameters`) → Broken.
  - **State bucket**: Broken.
  - **How obtained**: `DBSnapshot.Status` on the `DescribeDBSnapshots` response.

- **Signal**: `Encrypted == false` → Warning (CIS RDS.4).
  - **State bucket**: Warning.
  - **How obtained**: `DBSnapshot.Encrypted` on the `DescribeDBSnapshots` response.

### 3.2 Wave 2 — bounded extra API calls

The two cross-reference signals below make no AWS call of their own — they read
the already-loaded `dbi` list — but they run in the enrichment pass, after the
list is on screen, so they are Wave 2 like the attribute read.

- **Signal**: cross-ref `dbi` — source DB instance no longer present in the already-loaded `dbi` list (orphan snapshot whose parent was deleted).
  - **State bucket**: Broken.
  - **How obtained**: read `DBSnapshot.DBInstanceIdentifier`; treat as orphan when the identifier is absent from the loaded `dbi` list. Skip the rule when the `dbi` list has not been loaded in this session (avoids false-positive orphan flags).

- **Signal**: cross-ref `dbi` — when the parent DB is present in the already-loaded `dbi` list, `SnapshotCreateTime` older than the parent `DBInstance.BackupRetentionPeriod` (in days) AND `SnapshotType == "automated"` (automated snapshot kept past its retention window — retention-policy drift or a stuck automated cycle).
  - **State bucket**: Broken.
  - **How obtained**: compute age from `DBSnapshot.SnapshotCreateTime` on the list response, cross-reference against the already-loaded `dbi` list by `DBInstanceIdentifier`, compare to `DBInstance.BackupRetentionPeriod`. Skip the rule when the parent DB is not in the loaded sibling list.
  - **Threshold**: fires on `age > retention` (1.0× — no multiplier). `BackupRetentionPeriod` IS the operator's declared retention policy; any snapshot kept past it is policy drift regardless of engine. Same threshold applies to `dbc-snap`.

- **Signal**: the snapshot's `restore` attribute lists the `all` group → **Broken** (`shared with all AWS accounts`).
  - **State bucket**: Broken.
  - **API call**: `DescribeDBSnapshotAttributes` — one call per snapshot.
  - **Cost shape**: per-resource.
  - **How obtained**: `core/aws/dbi_snap_issue_enrichment.go` reads the `restore` attribute and flags the snapshot when the shared list holds `all`.

### 3.3 Wave 3 — OUT OF SCOPE

Nothing is recorded out of scope for `dbi-snap`.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `dbi-snap`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `Status == creating` | 1 | Warning | n/a | S1, S2, S4 | `creating: <pct>%` |
| `Status` neither `available` nor an enumerated state | 1 | Warning | n/a | S1, S2, S4 | `<status>` |
| `Status == failed` | 1 | Broken | n/a | S1, S2, S4 | `failed` |
| `Status == incompatible-*` | 1 | Broken | n/a | S1, S2, S4 | `<incompatible-* status>` |
| `Encrypted == false` | 1 | Warning | n/a | S1, S2, S4 | `unencrypted` |
| orphan: source DB deleted | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `orphan: source DB deleted` |
| automated age > parent `BackupRetentionPeriod` | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `automated, <N>d past retention` |
| `restore` attribute lists the `all` group | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `shared with all AWS accounts` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`DORMANT`, `stopped`, `available`, `failed`) in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column. Tests will assert the cause is present.
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), you may omit the row from this table entirely; §3 still describes it.
- List text ≤ 40 chars. The Detail sentence lives on the finding definition and is generated into the Findings table below; it is never written here.

Notes on the table above:

- `Status == creating` pairs the state with `PercentProgress` so the operator can tell at a glance how far along the snapshot is — the SDK shape exposes the field on the list response (`AWS SDK Go v2 — rds/types.DBSnapshot § PercentProgress`).
- `Status == failed` and `Status == incompatible-*` carry only the state keyword because the RDS SDK `DBSnapshot` shape exposes no per-snapshot failure-reason field — a9s-devops persona: `DBSnapshot` has no `StatusInfos`/`StatusReason`/`FailureMessage` field (confirmed by `AWS SDK Go v2 — rds/types.DBSnapshot` field enumeration). The operator must pivot to `ct-events` (`CreateDBSnapshot`/`ModifyDBSnapshot` failure events) for the root cause, which is the documented RDS workflow for snapshot failures.
- The orphan row is self-describing; the automated-past-retention row shows the exact number of days past the parent's `BackupRetentionPeriod` so the operator can triage without opening detail.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Mostly yes — `unencrypted`, `orphan: source DB deleted`, and `automated, Nd past retention` are self-describing; `creating: <pct>%` tells the operator progress is in flight; the one gap is `failed` / `incompatible-*`, which carry only the state keyword because AWS exposes no structured failure-reason field on `DBSnapshot` — the operator must open detail and pivot to `ct-events` for the underlying cause, which is an acceptable design limit given the thinness of AWS's own surface here.

## 5. Out of Scope

- Shared-account detail beyond "shared with all AWS accounts" — the `restore` attribute also names individual account IDs a snapshot is shared with; a9s reports only the public case, because naming accounts belongs in a security-posture view rather than on a list row.
- Manual-snapshot cost-drift age rule (> 365d on `SnapshotType=="manual"`) — no such finding on `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `dbi-snap`; it belongs to `dbc-snap`. Out of scope here until the golden doc adds it.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

One bullet per claim in §§2–4.1. Citation sources, in order of authority:

- a9s golden doc — related-panel contract for `dbi-snap` (targets `backup`, `ct-events`, `dbc`, `dbi`, `kms`; per-type contract table row) — `docs/related-resources.md § Per-type contract` (dbi-snap row) and `§ dbi-snap`.
- a9s golden doc — Wave 1 signals (`Status` buckets, `Encrypted==false`, orphan cross-ref `dbi`, automated-past-retention cross-ref `dbi`) — `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `dbi-snap`.
- a9s golden doc — every `dbi-snap` signal reads the list response — `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `dbi-snap`.
- The public-snapshot finding and its `DescribeDBSnapshotAttributes` read — `docs/attention-signals.md § Signals § DATABASES & STORAGE` row `dbi-snap`; `core/aws/dbi_snap_issue_enrichment.go`.
- a9s golden doc — `ct-events` universal-pivot policy — `docs/related-resources.md § Policy`.
- a9s golden doc — `dbc` marked weak (1/6 DevOps audits) — `docs/related-resources.md § dbi-snap` ("Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot").
- a9s golden doc — read-only invariant — `docs/architecture.md § What is a9s?`.
- AWS Go SDK v2 — `DBSnapshot.DBInstanceIdentifier`, `.KmsKeyId`, `.Encrypted`, `.Status`, `.SnapshotType`, `.SnapshotCreateTime`, `.PercentProgress`, `.DBSnapshotIdentifier`, `.DBSnapshotArn` fields — `AWS SDK Go v2 — rds/types.DBSnapshot`.
- AWS Go SDK v2 — `DBSnapshot` has no `DBClusterIdentifier` field (Aurora cluster pivot is indirect via `dbi`) — `AWS SDK Go v2 — rds/types.DBSnapshot` (field enumeration). `DBClusterSnapshot` is the cluster-level sibling and is surfaced as a separate shortName, not through `dbi-snap`.
- AWS Go SDK v2 — `DBSnapshot` has no `StatusInfos`/`StatusReason`/`FailureMessage` field (operator must pivot to `ct-events` for failure cause) — `AWS SDK Go v2 — rds/types.DBSnapshot` (field enumeration).
- AWS API Reference (authoritative list-API page) — `DescribeDBSnapshots` — `https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBSnapshots.html`.
- AWS API Reference — `DescribeDBSnapshotAttributes` (Wave 3 exclusion context) — `https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DescribeDBSnapshotAttributes.html`.
- AWS API Reference — `LookupEvents` with `ResourceName` attribute for ct-events pivot — `https://docs.aws.amazon.com/awscloudtrail/latest/APIReference/API_LookupEvents.html`.
- S1–S5 surface rules — skill `a9s-resource-spec § Allowed visualization surfaces (exactly five)`.
- a9s-devops persona (2026-04-20) — `dbi` discovery via direct field read (`DBInstanceIdentifier`) — possible=yes, worth=yes. Rationale: the single most important pivot for any snapshot; zero API cost.
- a9s-devops persona (2026-04-20) — `kms` discovery via direct field read (`KmsKeyId`) — possible=yes, worth=yes. Rationale: silent restore-blocker when the key is disabled; free on the list response.
- a9s-devops persona (2026-04-20) — `dbc` discovery via two-hop cross-ref through the already-loaded `dbi` list (`DBSnapshot.DBInstanceIdentifier` → `DBInstance.DBClusterIdentifier` → `dbc` list) — possible=yes (indirect), worth=weak. Rationale: only meaningful for Aurora-member source instances; the pivot is free when both sibling lists are loaded but is otherwise absent. Related-resources.md flags the pivot as 1/6 DevOps audits (weak signal) and this spec keeps it because the cost is zero.
- a9s-devops persona (2026-04-20) — `backup` discovery via `DBSnapshotIdentifier` prefix match on `awsbackup:job-<uuid>` — possible=yes, worth=yes (narrow). Rationale: AWS Backup tracks the parent DB instance rather than each manual snapshot, so a live `ListRecoveryPointsByResource` call is high-cost for thin value; the identifier prefix match is free on the list response and answers the same operator question ("was this a Backup-plan recovery point?").
- a9s-devops persona (2026-04-20) — `ct-events` count shown = unknown (windowed pivot, not pre-counted) — possible=yes (lazy), worth=yes. Rationale: `LookupEvents` paginates over a time window with no documented total; a number would mislead.
- a9s-devops persona (2026-04-20) — no per-row cause text for `failed` / `incompatible-*` Status values on `DBSnapshot` — possible=no on the RDS SDK shape; `DBSnapshot` has no `StatusInfos`/`StatusReason`/`FailureMessage` field. Rationale: bare keyword is the most the list response carries; operator pivots to `ct-events` for `CreateDBSnapshot`/`ModifyDBSnapshot` failure events. Acceptable design limit.
- a9s-devops persona (2026-04-20) — `creating` status paired with `PercentProgress` for S4 text — possible=yes, worth=yes. Rationale: `DBSnapshot.PercentProgress` is on the list response; attaching the percentage converts a bare transitional keyword into an informative progress reading at zero cost.
- a9s-devops persona (2026-04-20) — public-snapshot / shared-account detection (`DescribeDBSnapshotAttributes`) kept out of scope as a Wave 2 list-row signal — possible=yes, worth=no at list-row cost shape. Rationale: per-snapshot fan-out is expensive for a security-audit concern that belongs in a posture view, not on every list refresh; Wave 3 placement matches the golden doc.
- Count-shown values for the related panel — a9s-devops persona (2026-04-20): possible=yes (for cached-sibling lookups), worth=yes. For `dbi`, `kms`, `dbc`, `backup` the counts come from already-loaded siblings and are exact and cheap; `ct-events` is windowed and a count would be misleading, hence `unknown`.

<!-- BEGIN GENERATED: header -->
dbi-snap — DATABASES & STORAGE. Lifecycle key: `status`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| dbi-snap.broken.failed | failed | broken | wave1 | — |
| dbi-snap.broken.incompatible | <incompatible-\* status> | broken | wave1 | — |
| dbi-snap.warn.creating | creating: <pct>% | warn | wave1 | — |
| dbi-snap.warn.transitional | <status> | warn | wave1 | — |
| dbi-snap.warn.unencrypted | unencrypted | warn | wave1 | — |
| dbi-snap.orphan | orphan: source DB deleted | broken | wave2 | — |
| dbi-snap.past-retention | automated, <N>d past retention | broken | wave2 | — |
| dbi-snap.public | shared with all AWS accounts | broken | wave2 | The snapshot is shared with every AWS account, so anyone can restore it and read the database it came from. Remove `all` from the snapshot's restore attribute. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| dbi | DB Instances | yes |
| kms | KMS Keys | yes |
| backup | Backup Plans | yes |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
