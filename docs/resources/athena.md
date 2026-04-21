---
shortName: athena
name: Athena Workgroups
awsApiRef: https://docs.aws.amazon.com/athena/latest/APIReference/API_WorkGroup.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# athena — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `athena`
- **Display name**: Athena Workgroups
- **AWS API reference**: https://docs.aws.amazon.com/athena/latest/APIReference/API_WorkGroup.html
- **List API**: `ListWorkGroups` — returns `WorkGroupSummary[]`. The SDK confirms `Name`, `State`, `CreationTime`, `Description`, `EngineVersion` are on the summary shape, so the Wave 1 `State` signal is reachable with zero extra calls.
- **Describe API (if any)**: `GetWorkGroup` per workgroup — used in Wave 2 to read `Configuration.EnforceWorkGroupConfiguration`, `Configuration.ResultConfiguration.EncryptionConfiguration`, and `Configuration.BytesScannedCutoffPerQuery`, none of which are on `WorkGroupSummary`.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `glue`, `kms`, `logs`, `role`, `s3`, `ct-events`.

### `glue`

- **Why related**: Athena queries the account's Glue Data Catalog; when a query fails with a schema or table error, the operator pivots to Glue Jobs that populate those tables.
- **How discovered**: the WorkGroup shape carries no Glue reference — the Athena↔Glue link is namespace-level (Glue Catalog is account- and region-scoped, one default catalog per region). Cross-reference the already-loaded `glue` (Jobs) list for the current profile/region with no filter. a9s-devops: no per-workgroup Glue field exists on `WorkGroup` or `WorkGroupConfiguration`; showing the full region-local Glue Jobs list is the idiomatic pivot.
- **Count shown**: yes (total Glue Jobs in region).

### `kms`

- **Why related**: KMS key encrypting Athena query results written to S3 (workgroup result encryption).
- **How discovered**: call `GetWorkGroup`; read `Configuration.ResultConfiguration.EncryptionConfiguration.KmsKey` (set when `EncryptionOption` is `SSE_KMS` or `CSE_KMS`). Also read `Configuration.CustomerContentEncryptionConfiguration.KmsKey` for Spark-enabled workgroups. Cross-reference the loaded `kms` list by key ARN or ID.
- **Count shown**: yes (0 or 1 — result-encryption is a single key per workgroup; Spark data-store encryption adds at most one more).

### `logs`

- **Why related**: CloudWatch Log group where the workgroup publishes query/session logs (Spark workgroups) — first stop when investigating why a query or session failed.
- **How discovered**: call `GetWorkGroup`; read `Configuration.MonitoringConfiguration.CloudWatchLoggingConfiguration.LogGroup` (set only when `Enabled==true`). Cross-reference the loaded `logs` list by log-group name.
- **Count shown**: yes (0 or 1).

### `role`

- **Why related**: Execution role the workgroup assumes to access user data (Spark sessions and IAM Identity Center enabled workgroups). For SQL-only workgroups the role is user-policy-based and not stored on the workgroup.
- **How discovered**: call `GetWorkGroup`; read `Configuration.ExecutionRole` (an IAM role ARN). Cross-reference the loaded `role` list by ARN. a9s-devops: `ExecutionRole` is populated only for Spark / IAM IC workgroups; for SQL-only workgroups the pivot is suppressed (count=0) and operators go to CloudTrail events instead. related-resources.md itself flags `role` as a 1/6-audit borderline pivot, consistent with this partial-field reality.
- **Count shown**: yes (0 or 1 depending on workgroup type).

### `s3`

- **Why related**: S3 bucket where query results are written (`OutputLocation`). Operators pivot here to find the actual result CSV/Parquet, check result retention/lifecycle, or diagnose "result not found" errors.
- **How discovered**: call `GetWorkGroup`; read `Configuration.ResultConfiguration.OutputLocation` (an `s3://bucket/path` URI), parse out the bucket name, and cross-reference the loaded `s3` list by bucket name.
- **Count shown**: yes (0 or 1 — one output location per workgroup).

### `ct-events`

- **Why related**: Universal pivot — who created, updated, enabled, or disabled this workgroup; which principals ran queries or changed its configuration.
- **How discovered**: pre-built CloudTrail query scoped to the workgroup name (Athena events reference workgroups by `Name`, not ARN, in CloudTrail event payloads).
- **Count shown**: unknown (CloudTrail queries are windowed; a reliable total isn't available without a separate count call).
- Universal pivot — applies to every registered type; see `related-resources.md` §Policy.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

One bullet per distinct signal. Keep AWS field names verbatim.

- **Signal**: `State == ENABLED`.
  - **State bucket**: Healthy.
  - **How obtained**: `WorkGroupSummary.State` from `ListWorkGroups`.

- **Signal**: `State == DISABLED`.
  - **State bucket**: Warning.
  - **How obtained**: `WorkGroupSummary.State` from `ListWorkGroups`. Admin-off — the workgroup exists but will reject new query submissions.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: `Configuration.EnforceWorkGroupConfiguration == false` AND `Configuration.ResultConfiguration.EncryptionConfiguration == nil`.
  - **State bucket**: Warning.
  - **API call**: `GetWorkGroup` per workgroup.
  - **Cost shape**: per-resource. Governance gap — workgroup does not force its settings on clients, and no result-encryption is configured, so query results can be written unencrypted to S3.

- **Signal**: `Configuration.BytesScannedCutoffPerQuery` unset (nil).
  - **State bucket**: Warning.
  - **API call**: same `GetWorkGroup` per workgroup — no additional call beyond the governance check above.
  - **Cost shape**: per-resource. Cost-control gap — a runaway query in this workgroup has no scan-bytes ceiling and can bill unbounded dollars.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `ListQueryExecutions` + `BatchGetQueryExecution` failure-rate per workgroup.

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
| `State == DISABLED` | 1 | Warning | n/a | S2, S4 | `disabled: no new queries accepted` | `Workgroup is disabled — queries submitted here will be rejected until re-enabled.` |
| `EnforceWorkGroupConfiguration==false && ResultConfiguration.EncryptionConfiguration==nil` | 2 | Healthy | `~` | S3, S4, S5 | `results unencrypted, not enforced` | `Query results written to S3 without encryption; workgroup does not force client-side settings.` |
| `BytesScannedCutoffPerQuery` unset | 2 | Healthy | `~` | S3, S4, S5 | `no per-query scan limit` | `No data-scan ceiling — a runaway query can bill unbounded dollars.` |

Rules for filling list and detail text:

- Banned words (internal jargon must never appear here): `Wave 1`, `Wave 2`, `Wave 3`, `finding`, `enrichment`, `probe`, `truncated`, `lower bound`, `bucket`, `severity`.
- A bare state keyword (`DORMANT`, `stopped`, `available`, `failed`) in the List text column is not acceptable. Pair it with the cause, or put the cause in the adjacent description column. Tests will assert the cause is present.
- For signals that legitimately have no operator-actionable cause (e.g. pure `Healthy`), you may omit the row from this table entirely; §3 still describes it.
- Keep both columns short enough to fit: List text ≤ 40 chars, Detail text ≤ 100 chars.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? All problem rows are self-explanatory in the list — a yellow row reading `disabled: no new queries accepted` tells the operator the workgroup is admin-off, and a green row prefixed with `~` and `results unencrypted, not enforced` tells them the workgroup has a governance gap without needing to press detail. Operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- `role` pivot for SQL-only workgroups — a9s-devops: possible=partial (ExecutionRole is nil on SQL workgroups), worth=marginal; surface only when the field is populated, do not synthesize a role by reading IAM policies.

## 6. Citations

- a9s golden doc — athena contract row (related targets and AWS API URL) — `docs/related-resources.md` § Per-type contract, row `athena`, and § `athena` subsection.
- a9s golden doc — Wave 1 `State==ENABLED`/`DISABLED` mapping — `docs/attention-signals.md` § Data & Analytics, row `athena`, Wave 1 cell.
- a9s golden doc — Wave 2 `EnforceWorkGroupConfiguration==false && ResultConfiguration.EncryptionConfiguration==nil` and `BytesScannedCutoffPerQuery` unset — `docs/attention-signals.md` § Data & Analytics, row `athena`, Wave 2 cell.
- a9s golden doc — Wave 3 `ListQueryExecutions`/`BatchGetQueryExecution` is out of scope — `docs/attention-signals.md` § Data & Analytics, row `athena`, Wave 3 cell.
- a9s golden doc — read-only invariant — `docs/architecture.md` § "a9s is a read-only terminal UI for AWS".
- AWS Go SDK v2 — `ListWorkGroups` returns `WorkGroupSummary` with `Name`, `State`, `CreationTime`, `Description`, `EngineVersion` — `AWS SDK Go v2 — service/athena/types.WorkGroupSummary § State`.
- AWS Go SDK v2 — `State` enum values `ENABLED`/`DISABLED` — `AWS SDK Go v2 — service/athena/types.WorkGroupState § WorkGroupStateEnabled`.
- AWS Go SDK v2 — `GetWorkGroup` returns `WorkGroup.Configuration` (not on `WorkGroupSummary`) — `AWS SDK Go v2 — service/athena/types.WorkGroup § Configuration`.
- AWS Go SDK v2 — `EnforceWorkGroupConfiguration`, `BytesScannedCutoffPerQuery`, `ResultConfiguration`, `ExecutionRole`, `CustomerContentEncryptionConfiguration`, `MonitoringConfiguration` all on `WorkGroupConfiguration` — `AWS SDK Go v2 — service/athena/types.WorkGroupConfiguration § EnforceWorkGroupConfiguration, § BytesScannedCutoffPerQuery, § ResultConfiguration, § ExecutionRole`.
- AWS Go SDK v2 — `OutputLocation` (S3 URI) and `EncryptionConfiguration` on `ResultConfiguration` — `AWS SDK Go v2 — service/athena/types.ResultConfiguration § OutputLocation, § EncryptionConfiguration`.
- AWS Go SDK v2 — `KmsKey` on `EncryptionConfiguration` (populated when `EncryptionOption` is `SSE_KMS` or `CSE_KMS`) — `AWS SDK Go v2 — service/athena/types.EncryptionConfiguration § KmsKey`.
- AWS Go SDK v2 — `CloudWatchLoggingConfiguration.LogGroup` — `AWS SDK Go v2 — service/athena/types.CloudWatchLoggingConfiguration § LogGroup`.
- a9s-devops consultation — glue discovery via region-local Glue Jobs list (no per-workgroup field) — `a9s-devops (2026-04-20): possible=yes, worth=yes. Athena↔Glue binding is namespace-level; Glue Data Catalog is account/region-scoped. Operator workflow: failed query → pivot to Glue Jobs that populate the referenced tables.`
- a9s-devops consultation — role pivot is partial (Spark/IAM IC only) via `Configuration.ExecutionRole` — `a9s-devops (2026-04-20): possible=yes (partial), worth=yes. Spark workgroups have ExecutionRole; SQL-only workgroups fall back to ct-events for audit. Matches related-resources.md 1/6-audit borderline note.`
- a9s-devops consultation — count shown = yes for kms/logs/role/s3 (singular fields when set); glue = yes (region count); ct-events = unknown (windowed CloudTrail queries) — `a9s-devops (2026-04-20): possible=yes, worth=yes. Per-target singular or account-wide; consistent with acm/s3/ec2 specs in docs/resources/.`
