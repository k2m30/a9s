---
shortName: dbc-snap
name: DB Cluster Snapshots
awsApiRef: https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DBClusterSnapshot.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# dbc-snap — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `dbc-snap`
- **Display name**: DB Cluster Snapshots
- **AWS API reference**: https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DBClusterSnapshot.html
- **List API**: `DescribeDBClusterSnapshots`
- **Describe API (if any)**: not used — all Wave 1 signals are carried on the list response.
- **Coverage**: this resource type covers BOTH DocumentDB cluster snapshots
  AND Aurora cluster snapshots (RDS Aurora — `aurora-postgresql`,
  `aurora-mysql`). They share the AWS API: `DescribeDBClusterSnapshots`
  returns both engines, with the engine name on each row. Real AWS rejects
  `CreateDBSnapshot` on Aurora cluster members; Aurora cluster-level
  snapshots only exist as `DBClusterSnapshot`s, which is why they live
  here and not in `dbi-snap`.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `backup`, `dbc`, `kms`, `vpc`, `ct-events`.

### `dbc`

- **Why related**: the source cluster this snapshot was taken from — the operator's first question ("where did this come from, is it still alive?") is always about the parent cluster. Citation: `related-resources.md § dbc-snap` ("Source cluster").
- **How discovered**: read `DBClusterSnapshot.DBClusterIdentifier` from the list response, then cross-reference the already-loaded `dbc` list by that identifier. No extra API call. Citation: `AWS SDK Go v2 — docdb/types.DBClusterSnapshot § DBClusterIdentifier`.
- **Count shown**: yes (0 or 1 — a snapshot has exactly one source cluster; 0 when the parent has been deleted, which is itself the orphan signal in §3.1).

### `kms`

- **Why related**: the encryption key protecting the snapshot. If the key is disabled or pending deletion, the snapshot cannot be restored — a silent restore-blocker the operator needs to catch early. Citation: `related-resources.md § dbc-snap` ("Encryption key").
- **How discovered**: read `DBClusterSnapshot.KmsKeyId` from the list response, then cross-reference the already-loaded `kms` list by KeyId/KeyArn. No extra API call. Citation: `AWS SDK Go v2 — docdb/types.DBClusterSnapshot § KmsKeyId`.
- **Count shown**: yes (0 or 1 — one key per encrypted snapshot; 0 when `StorageEncrypted==false`).

### `vpc`

- **Why related**: the VPC the source cluster lived in when the snapshot was taken — orients the operator when planning a restore into the same or a sibling network. Citation: `related-resources.md § dbc-snap` ("Mentioned by 1/6 independent DevOps audits as an AWS-API or operational pivot").
- **How discovered**: read `DBClusterSnapshot.VpcId` from the list response, then cross-reference the already-loaded `vpc` list by VPC ID. No extra API call. Citation: `AWS SDK Go v2 — docdb/types.DBClusterSnapshot § VpcId`.
- **Count shown**: yes (0 or 1) — a9s-devops persona: the snapshot records the VPC of the source cluster at snapshot time; on restore, operator can choose a different VPC, so this is orienting context rather than a hard binding. possible=yes, worth=yes (weak). Marginal pivot but the field is free on the list response.

### `backup`

- **Why related**: AWS Backup can produce DocDB cluster snapshots on behalf of a backup plan; knowing whether a snapshot was created by Backup (vs manual/automated by the cluster) tells the operator which retention policy governs its lifecycle. Citation: `related-resources.md § dbc-snap` ("Snapshots covered by Backup vaults").
- **How discovered**: a9s-devops persona (2026-04-20): possible=yes, worth=yes (narrow). AWS Backup-created snapshots carry the identifier prefix `awsbackup:job-<uuid>` on `DBClusterSnapshotIdentifier`, and AWS Backup records the snapshot ARN on its recovery-point list (`ListRecoveryPointsByResource` with the cluster ARN). The cheap Wave-1-safe path is a string match on the snapshot identifier prefix — no extra API call required. Rationale: most DocDB operators split "restore from a DocDB-native snapshot" vs "restore from an AWS Backup recovery point" as different workflows with different audit trails; surfacing the pivot without a per-row API call is the right cost shape.
- **Count shown**: yes (0 or 1 — a snapshot is either a Backup-created recovery point or it is not).

### `ct-events`

- **Why related**: universal pivot — every registered type carries a CloudTrail pivot for audit ("who deleted this snapshot", "why was it created"). See `related-resources.md §Policy`.
- **How discovered**: `LookupEvents` with `ResourceName = DBClusterSnapshotIdentifier` and `ResourceType = AWS::RDS::DBClusterSnapshot` (DocDB shares the RDS CloudTrail resource type). Called on demand when the operator opens the pivot, not on list load.
- **Count shown**: unknown (lazy — the pivot is opened, not counted pre-emptively).

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `Status == "available"`.
  - **State bucket**: Healthy.
  - **How obtained**: `DBClusterSnapshot.Status` on the list response.

- **Signal**: `Status == "creating"`.
  - **State bucket**: Warning.
  - **How obtained**: `DBClusterSnapshot.Status` on the list response.

- **Signal**: `Status == "failed"`.
  - **State bucket**: Broken.
  - **How obtained**: `DBClusterSnapshot.Status` on the list response.

- **Signal**: `Status` matches `incompatible-*` (e.g. `incompatible-restore`) — the snapshot exists but cannot be restored without manual intervention.
  - **State bucket**: Broken.
  - **How obtained**: `DBClusterSnapshot.Status` prefix match on the list response. AWS does not officially enumerate this status family for `DBClusterSnapshot` in the public API reference, but it surfaces in practice (mirrors the documented `DBSnapshot.Status` family) and `dbi-snap` handles it identically. Defensive parity — keep the keyword verbatim per the §4 table rules.

- **Signal**: manual snapshot age > 365d (cost drift — forgotten long-lived manual snapshot).
  - **State bucket**: Warning.
  - **How obtained**: compute `now() - DBClusterSnapshot.SnapshotCreateTime` on the list response; gate on `SnapshotType == "manual"`.

- **Signal**: cross-ref `dbc` — source cluster no longer present in the already-loaded `dbc` list → Warning (orphan snapshot whose parent was deleted).
  - **State bucket**: Warning.
  - **How obtained**: read `DBClusterSnapshot.DBClusterIdentifier`; treat as orphan when the identifier is absent from the loaded `dbc` list. Skip the rule when the `dbc` list has not been loaded in this session (avoids false-positive orphan flags).

- **Signal**: cross-ref `dbc` — when the parent cluster is present in the already-loaded `dbc` list, `SnapshotCreateTime` older than `DBCluster.BackupRetentionPeriod` AND `SnapshotType == "automated"` → Warning (automated snapshot kept past its retention window — signals retention-policy drift or a stuck automated cycle).
  - **State bucket**: Warning.
  - **How obtained**: compute age from `DBClusterSnapshot.SnapshotCreateTime` on the list response, cross-referenced against the already-loaded `dbc` list by `DBClusterIdentifier`. Skip the rule when the parent cluster is not in the loaded sibling list.
  - **Threshold**: fires on `age > retention` (1.0× — no multiplier). `BackupRetentionPeriod` IS the operator's declared retention policy; any snapshot kept past it is policy drift regardless of engine. Same threshold applies to `dbi-snap`.

### 3.2 Wave 2 — bounded extra API calls

No Wave 2 signals.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `DescribeDBClusterSnapshotAttributes` per snapshot (public-snapshot detection).

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
| `Status == creating` | 1 | Warning | n/a | S2, S4 | `creating` | — |
| `Status == failed` | 1 | Broken | n/a | S2, S4 | `failed` | — |
| `Status` matches `incompatible-*` | 1 | Broken | n/a | S2, S4 | `incompatible-restore` (keyword verbatim) | — |
| manual age > 365d | 1 | Warning | n/a | S2, S4 | `manual, unused 400d` | — |
| orphan: source cluster deleted | 1 (cross-ref) | Warning | n/a | S1, S2, S4, S5 | `orphan: source cluster deleted` | `orphan: source cluster deleted` + Source Cluster row |
| automated age > parent `BackupRetentionPeriod` | 1 (cross-ref) | Warning | n/a | S1, S2, S4, S5 | `automated, <N>d past retention` | `automated, <N>d past retention` + Source Cluster / Retention / Created rows |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`DORMANT`, `stopped`, `available`, `failed`) in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column. Tests will assert the cause is present.
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), you may omit the row from this table entirely; §3 still describes it.
- Keep both columns short enough to fit: List text ≤ 40 chars, Detail text ≤ 100 chars.

Notes on the table above:

- `Status == creating` is transient; the row text gives the state verbatim because there is no deeper cause field on the list response (`DBClusterSnapshot` has no `StatusReason`-style field — a9s-devops persona: possible=no on the SDK shape, so no cause text is available at list time).
- `Status == failed` likewise has no per-row cause field on the SDK shape. The bare keyword is the most informative thing the list response carries; the operator presses detail for the full record and typically pivots to `ct-events` for the `ModifyDBClusterSnapshot`/`CreateDBClusterSnapshot` failure event.
- The two age-based rows show concrete numeric causes so the operator can triage without opening detail.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? `creating` and `failed` rows carry only the state keyword because the DocDB SDK `DBClusterSnapshot` shape exposes no per-snapshot failure-reason field — the implementation should still surface these yellow/red rows, but for `failed` the operator must open detail and pivot to `ct-events` to get the cause, which is an acceptable design limit given AWS's own surface is thin here; the two age rows are self-explanatory.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- `vpc` as anything more than orienting context — the snapshot records the source-cluster VPC, but restore-time VPC selection is independent. a9s-devops persona: possible=yes (field is on the SDK shape), worth=weak (marginal pivot). Kept because the field is free on the list response.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

One bullet per claim in §§2–4.1. Citation sources, in order of authority:

- a9s golden doc — related-panel contract for `dbc-snap` — `docs/related-resources.md § dbc-snap` (targets `backup`, `ct-events`, `dbc`, `kms`, `vpc`; per-type contract table row).
- a9s golden doc — Wave 1 signals (`Status` buckets, manual-age cost rule, automated cross-ref with `dbc` retention) — `docs/attention-signals.md § Databases & Storage § dbc-snap`.
- a9s golden doc — Wave 3 exclusion (`DescribeDBClusterSnapshotAttributes`) — `docs/attention-signals.md § Databases & Storage § dbc-snap` Wave 3 cell.
- a9s golden doc — read-only invariant — `docs/architecture.md § What is a9s?`.
- a9s golden doc — `ct-events` universal-pivot policy — `docs/related-resources.md § Policy`.
- AWS Go SDK v2 — `DBClusterIdentifier`, `KmsKeyId`, `VpcId`, `Status`, `SnapshotType`, `SnapshotCreateTime`, `StorageEncrypted` fields — `AWS SDK Go v2 — docdb/types.DBClusterSnapshot`.
- AWS Go SDK v2 — no `StatusReason`-style cause field exists on `DBClusterSnapshot` — `AWS SDK Go v2 — docdb/types.DBClusterSnapshot` (field enumeration).
- AWS API Reference (authoritative list-API page) — `DescribeDBClusterSnapshots` — `https://docs.aws.amazon.com/documentdb/latest/developerguide/API_DescribeDBClusterSnapshots.html`.
- a9s-devops persona (2026-04-20) — `backup` discovery via snapshot-identifier prefix `awsbackup:job-<uuid>` on `DBClusterSnapshotIdentifier` — possible=yes, worth=yes (narrow). Rationale: DocDB operators treat AWS Backup-created recovery points as a separate workflow from DocDB-native snapshots; the identifier prefix is free on the list response, so no per-row API cost.
- a9s-devops persona (2026-04-20) — `vpc` pivot worth-assessment — possible=yes (field on SDK shape), worth=weak. Rationale: kept because the field is free on the list response and orients the operator at a glance; restore-time VPC selection is independent.
- a9s-devops persona (2026-04-20) — no per-row cause text for `creating`/`failed` Status values — possible=no on the DocDB SDK shape. Rationale: the operator must pivot to `ct-events` to get the failure cause; this is an acceptable design limit given AWS's own surface is thin here.
