---
shortName: dbi-snap
name: DB Instance Snapshots
awsApiRef: https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBSnapshot.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# dbi-snap — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `dbi-snap`
- **Display name**: DB Instance Snapshots
- **AWS API reference**: https://docs.aws.amazon.com/AmazonRDS/latest/APIReference/API_DBSnapshot.html
- **List API**: `DescribeDBSnapshots`
- **Describe API (if any)**: not used — all Wave 1 signals are carried on the list response; Wave 2 is `None` per `attention-signals.md § Databases & Storage § dbi-snap`.

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
`DBCluster` in real AWS), which is dead UX. See `internal/aws/dbi_snap.go` for
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

Transcribed from `docs/attention-signals.md § Databases & Storage § dbi-snap`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `Status == "available"` → Healthy.
  - **State bucket**: Healthy.
  - **How obtained**: `DBSnapshot.Status` on the `DescribeDBSnapshots` response.

- **Signal**: `Status == "creating"` → Warning.
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

- **Signal**: cross-ref `dbi` — source DB instance no longer present in the already-loaded `dbi` list → Warning (orphan snapshot whose parent was deleted).
  - **State bucket**: Warning.
  - **How obtained**: read `DBSnapshot.DBInstanceIdentifier`; treat as orphan when the identifier is absent from the loaded `dbi` list. Skip the rule when the `dbi` list has not been loaded in this session (avoids false-positive orphan flags).

- **Signal**: cross-ref `dbi` — when the parent DB is present in the already-loaded `dbi` list, `SnapshotCreateTime` older than the parent `DBInstance.BackupRetentionPeriod` (in days) AND `SnapshotType == "automated"` → Warning (automated snapshot kept past its retention window — signals retention-policy drift or a stuck automated cycle).
  - **State bucket**: Warning.
  - **How obtained**: compute age from `DBSnapshot.SnapshotCreateTime` on the list response, cross-reference against the already-loaded `dbi` list by `DBInstanceIdentifier`, compare to `DBInstance.BackupRetentionPeriod`. Skip the rule when the parent DB is not in the loaded sibling list.
  - **Threshold note**: dbi-snap fires on `age > retention` (no multiplier). The sister type `docdb-snap` uses `age > retention × 1.5` to suppress chatter on DocumentDB clusters whose automated cleanup runs less aggressively. The thresholds were authored at different times and the divergence is intentional, not an inconsistency to reconcile — RDS automated snapshots are evicted on a tight schedule, so any overshoot is operator-actionable; DocumentDB tolerates a half-cycle slip.

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `DescribeDBSnapshotAttributes` per snapshot (public-snapshot detection; per-row fan-out).

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
- **Wave 1 Warning / Broken / Dim, fetcher-detected** → S2 (color) + S4 (cause text). No S1, S3, S5. Applies to the four signals computable from the `DescribeDBSnapshots` row alone (creating / failed / incompatible / unencrypted).
- **Wave 1 Warning, cross-ref-detected** → S1 (counted as an issue instance), S2 (color), S4 (cause text), S5 (Attention entry with the same phrase plus structured Rows). Applies to `orphan: source DB deleted` and `automated, <N>d past retention`. Wave classification is unchanged (zero AWS API calls — the enricher only reads the in-memory dbi cache); the surface set is broader because the cross-ref enricher is the only enricher-output channel that reaches S5, so the implementation routes these phrases through the `Findings` map for S5 visibility while simultaneously emitting `FieldUpdates["status"]` for S4. See `internal/aws/rds_snap_issue_enrichment.go` for the contract.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4 (short cause), S5 (full sentence).
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph on green row. S3, S4 (short cause), S5 (full sentence). No S1.
- **Wave 2 finding on an already yellow/red/dim row** → redundant with color; S3 suppressed, S4 deduplicates with existing cause, S5 still carries the full sentence, S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `Status == creating` | 1 | Warning | n/a | S2, S4 | `creating: <pct>%` | — |
| `Status == failed` | 1 | Broken | n/a | S2, S4 | `failed` | — |
| `Status == incompatible-*` | 1 | Broken | n/a | S2, S4 | `incompatible-restore` (or current keyword) | — |
| `Encrypted == false` | 1 | Warning | n/a | S2, S4 | `unencrypted` | — |
| orphan: source DB deleted | 1 (cross-ref) | Warning | n/a | S1, S2, S4, S5 | `orphan: source DB deleted` | `orphan: source DB deleted` + Source DB row |
| automated age > parent `BackupRetentionPeriod` | 1 (cross-ref) | Warning | n/a | S1, S2, S4, S5 | `automated, <N>d past retention` | `automated, <N>d past retention` + Source DB / Retention / Created rows |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`DORMANT`, `stopped`, `available`, `failed`) in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column. Tests will assert the cause is present.
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), you may omit the row from this table entirely; §3 still describes it.
- Keep both columns short enough to fit: List text ≤ 40 chars, Detail text ≤ 100 chars.

Notes on the table above:

- `Status == creating` pairs the state with `PercentProgress` so the operator can tell at a glance how far along the snapshot is — the SDK shape exposes the field on the list response (`AWS SDK Go v2 — rds/types.DBSnapshot § PercentProgress`).
- `Status == failed` and `Status == incompatible-*` carry only the state keyword because the RDS SDK `DBSnapshot` shape exposes no per-snapshot failure-reason field — a9s-devops persona: `DBSnapshot` has no `StatusInfos`/`StatusReason`/`FailureMessage` field (confirmed by `AWS SDK Go v2 — rds/types.DBSnapshot` field enumeration). The operator must pivot to `ct-events` (`CreateDBSnapshot`/`ModifyDBSnapshot` failure events) for the root cause, which is the documented RDS workflow for snapshot failures.
- The orphan row is self-describing; the automated-past-retention row shows the exact number of days past the parent's `BackupRetentionPeriod` so the operator can triage without opening detail.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Mostly yes — `unencrypted`, `orphan: source DB deleted`, and `automated, Nd past retention` are self-describing; `creating: <pct>%` tells the operator progress is in flight; the one gap is `failed` / `incompatible-*`, which carry only the state keyword because AWS exposes no structured failure-reason field on `DBSnapshot` — the operator must open detail and pivot to `ct-events` for the underlying cause, which is an acceptable design limit given the thinness of AWS's own surface here.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Per-row `DescribeDBSnapshotAttributes` for public-snapshot or shared-account detection — a9s-devops persona: possible=yes (the API returns `DBSnapshotAttributes` with `AttributeName=="restore"` listing shared account IDs, `all` meaning public), worth=no as a Wave 2 list-row signal because it's a per-snapshot fan-out; the check belongs in a security-posture view, not on every list load.
- Manual-snapshot cost-drift age rule (> 365d on `SnapshotType=="manual"`) — not present in `attention-signals.md § dbi-snap` for dbi-snap (it applies to `docdb-snap`); out of scope here until the golden doc adds it.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

One bullet per claim in §§2–4.1. Citation sources, in order of authority:

- a9s golden doc — related-panel contract for `dbi-snap` (targets `backup`, `ct-events`, `dbc`, `dbi`, `kms`; per-type contract table row) — `docs/related-resources.md § Per-type contract` (dbi-snap row) and `§ dbi-snap`.
- a9s golden doc — Wave 1 signals (`Status` buckets, `Encrypted==false`, orphan cross-ref `dbi`, automated-past-retention cross-ref `dbi`) — `docs/attention-signals.md § Databases & Storage § dbi-snap` (Wave 1 cell).
- a9s golden doc — Wave 2 cell is `None` — `docs/attention-signals.md § Databases & Storage § dbi-snap` (Wave 2 cell).
- a9s golden doc — Wave 3 exclusion (`DescribeDBSnapshotAttributes` per snapshot, public-snapshot) — `docs/attention-signals.md § Databases & Storage § dbi-snap` (Wave 3 cell).
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
