---
shortName: mwaa
name: Managed Airflow
awsApiRef: https://docs.aws.amazon.com/mwaa/latest/API/API_Environment.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# mwaa — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `mwaa`
- **Display name**: Managed Airflow
- **AWS API reference**: <https://docs.aws.amazon.com/mwaa/latest/API/API_Environment.html>
- **List API**: `ListEnvironments` (returns environment name strings only, paginated)
- **Describe API (if any)**: `GetEnvironment` (per environment, Wave 2)

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `ct-events`, `kms`, `logs`, `role`, `s3`, `sg`, `subnet`.

### `alarm`

- **Why related**: CloudWatch alarms in the `AWS/MWAA` namespace carry the `EnvironmentName` dimension — first triage stop during an incident.
- **How discovered**: cross-reference the already-loaded `alarm` list by dimension `EnvironmentName` == environment name (workflow pivot; no ARN field on `Environment`).
- **Count shown**: yes.

### `kms`

- **Why related**: `KmsKey` encrypts the metadata database, logs, and queue.
- **How discovered**: read field `KmsKey` on the environment.
- **Count shown**: yes.

### `logs`

- **Why related**: five per-component log groups; a failed DAG run sends the operator straight to TaskLogs/SchedulerLogs.
- **How discovered**: read fields `LoggingConfiguration.{DagProcessingLogs,SchedulerLogs,WebserverLogs,WorkerLogs,TaskLogs}.CloudWatchLogGroupArn` on the environment.
- **Count shown**: yes.

### `role`

- **Why related**: `ExecutionRoleArn` is the role Airflow tasks assume for AWS access ("why can't my DAG write to S3").
- **How discovered**: read field `ExecutionRoleArn` on the environment.
- **Count shown**: yes.

### `s3`

- **Why related**: `SourceBucketArn` holds DAGs, requirements.txt, and plugins ("why isn't my DAG showing up").
- **How discovered**: read field `SourceBucketArn` on the environment.
- **Count shown**: yes.

### `sg`

- **Why related**: `NetworkConfiguration.SecurityGroupIds` — "why can't Airflow reach RDS / my internal API".
- **How discovered**: read field `NetworkConfiguration.SecurityGroupIds` on the environment.
- **Count shown**: yes.

### `subnet`

- **Why related**: `NetworkConfiguration.SubnetIds` — where the environment's ENIs live; AZ and routing debugging.
- **How discovered**: read field `NetworkConfiguration.SubnetIds` on the environment.
- **Count shown**: yes.

### `ct-events`

- **Why related**: audit trail for environment changes ("who ran UpdateEnvironment"). Universal pivot — applies to every registered type; see related-resources.md §Policy.
- **How discovered**: CloudTrail LookupEvents by resource name.
- **Count shown**: yes.

Explicitly excluded (per `docs/related-resources.md` §`mwaa`): `vpc` (no direct field; one hop via subnet), `sqs` (`CeleryExecutorQueue` lives in an AWS-owned account — pivot dead-ends in AccessDenied; detail-view fact only), `vpce` (`WebserverVpcEndpointService`/`DatabaseVpcEndpointService` are endpoint-service names, not customer `vpce-*` IDs; detail text only).

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — the list API does not return fields usable for attention. (`ListEnvironments` returns environment name strings only; the pattern matches `eks`/`ng`/`ddb`.)

AccessDenied contract: when the profile's role denies `mwaa:ListEnvironments`, the main-menu row shows the error state — never `0`. An operator who reads "0 environments" during an incident wrongly concludes the account runs no Airflow.

### 3.2 Wave 2 — bounded extra API calls

All signals come from `GetEnvironment` per environment (N+1; accounts run 1–5 environments, so the fan-out is small).

- **Signal**: `Status == AVAILABLE` → Healthy.
  - **State bucket**: Healthy.
  - **API call**: `GetEnvironment`, one per environment.
  - **Cost shape**: per-resource.
- **Signal**: `Status` in `CREATING` / `CREATING_SNAPSHOT` / `PENDING` / `UPDATING` / `ROLLING_BACK` / `MAINTENANCE`.
  - **State bucket**: Warning.
  - **API call**: `GetEnvironment`, one per environment.
  - **Cost shape**: per-resource.
- **Signal**: `Status` in `CREATE_FAILED` / `UPDATE_FAILED` / `UNAVAILABLE`.
  - **State bucket**: Broken.
  - **API call**: `GetEnvironment`, one per environment.
  - **Cost shape**: per-resource.
- **Signal**: `Status` in `DELETING` / `DELETED`.
  - **State bucket**: Dim.
  - **API call**: `GetEnvironment`, one per environment.
  - **Cost shape**: per-resource.
- **Signal**: `LastUpdate.Status == FAILED` on an `AVAILABLE` environment — the last update silently failed while the environment keeps serving the previous configuration ("my change didn't take"). Surface `LastUpdate.Error.ErrorMessage` (and a humanized `ErrorCode`) as the cause.
  - **State bucket**: Warning.
  - **API call**: `GetEnvironment`, one per environment (same call as above).
  - **Cost shape**: per-resource.
- **Signal**: `WebserverAccessMode` in `PUBLIC_ONLY` / `PUBLIC_AND_PRIVATE` — the Airflow webserver is reachable from the internet (still IAM-authed, but exposed; analogous to `dbi` `PubliclyAccessible`).
  - **State bucket**: Warning.
  - **API call**: `GetEnvironment`, one per environment (same call as above).
  - **Cost shape**: per-resource.

- **Signal**: `airflow:GetEnvironment` denied for a listed environment name — the row is KEPT with the name only ("you can't see it" ≠ "it isn't there"; live witness 2026-07-14: a readonly role allowed `ListEnvironments` but denied `GetEnvironment`, which must not fake an empty list). The per-name failures also aggregate into the fetch error (flash + `!` log).
  - **State bucket**: Warning.
  - **API call**: `GetEnvironment`, one per environment (the denial IS the response).
  - **Cost shape**: per-resource.

Deliberately not signals (a9s-devops 2026-07-14): any `LoggingConfiguration` component disabled (logging is opt-in per component and cost-driven; a warning would fire on most environments — noise, not signal; detail-view fact only) and `AirflowVersion` EOL (no stable AWS source for the deprecation schedule; a hardcoded table goes stale and lies; fact only).

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `AWS/MWAA` metrics: `SchedulerHeartbeat` (the classic MWAA-outage canary), DAG-processing `ImportErrors`/`TotalParseTime`, `QueuedTasks`/`RunningTasks`, worker/scheduler CPU+memory.
- OUT OF SCOPE: AirflowVersion-EOL check (no stable API source).

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count + list frame title `!N` suffix | Aggregated count of `!`-severity findings. `~` findings do not bump. The list frame title appends a space-separated `!N` after the count parentheses when the current list has N > 0 issues (`s3(50+) !5`, `ec2(17) !1`), or `!N+` when N is a truncated lower bound; N uses the same aggregation as the menu badge (Wave 1 issue-colored rows + Wave 2 `!`-severity findings). No suffix when N = 0, and omitted in attention-only mode (`ctrl+z`) — the filtered count already is the issue count, so `name(5 of 50+) [!]` stays as-is. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing". **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause. **Healthy rows render blank.** |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit).
- **Wave 1 Warning / Broken / Dim** → S2 + S4.
- **Wave 2 finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4, S5.
- **Wave 2 finding on a Healthy row, informational** → `~` glyph on green row. S3, S4, S5. No S1.
- **Wave 2 finding on an already yellow/red/dim row** → S3 suppressed, S4 deduplicates with existing cause, S5 carries the full sentence, S1 still counts if `!`.

One row per signal from §3. All mwaa signals are Wave 2 because `ListEnvironments` is opaque; the `GetEnvironment` pass sets the row color, so the state-bucket rows behave like Wave 1 colors to the operator (yellow/red is the attention signal, S3 suppressed because the row is not green):

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `Status == CREATING` | 2 | Warning | n/a | S2, S4 | `creating` | `Environment is being provisioned; Airflow is not yet reachable.` |
| `Status == CREATING_SNAPSHOT` | 2 | Warning | n/a | S2, S4 | `creating snapshot` | `The environment is snapshotting its metadata database before an update or upgrade.` |
| `Status == PENDING` | 2 | Warning | n/a | S2, S4 | `pending: awaiting VPC endpoints` | `Creation is paused until the required VPC endpoints exist in your VPC.` |
| `Status == UPDATING` | 2 | Warning | n/a | S2, S4 | `updating` | `Environment update in progress; workers may be replaced.` |
| `Status == ROLLING_BACK` | 2 | Warning | n/a | S2, S4 | `rolling back: update failed` | `Update or upgrade failed; the environment is restoring the latest metadata snapshot.` |
| `Status == MAINTENANCE` | 2 | Warning | n/a | S2, S4 | `maintenance in progress` | `Scheduled maintenance is running; the environment may be briefly unavailable.` |
| `Status == CREATE_FAILED` | 2 | Broken | n/a | S2, S4 | `create failed` | `Environment creation failed and the environment was not created.` |
| `Status == UPDATE_FAILED` | 2 | Broken | n/a | S2, S4, S5 | `update failed: rolled back` | `Update failed; environment was restored to its previous state and is usable.` |
| `Status == UNAVAILABLE` | 2 | Broken | n/a | S2, S4 | `unavailable: not stable` | `Environment failed and did not return to a stable state; contact AWS support.` |
| `Status == DELETING` | 2 | Dim | n/a | S2, S4 | `deleting` | `Environment is being deleted.` |
| `Status == DELETED` | 2 | Dim | n/a | S2, S4 | `deleted` | `Environment has been deleted.` |
| `LastUpdate.Status == FAILED` on `AVAILABLE` | 2 | Warning | n/a | S2, S4, S5 | `last update failed` | `Last update failed: <LastUpdate.Error.ErrorMessage>. Still on previous config.` |
| `WebserverAccessMode` public | 2 | Warning | n/a | S2, S4, S5 | `webserver public` | `Airflow webserver is reachable from the internet (access mode: <humanized mode>).` |
| `GetEnvironment` denied | 2 | Warning | n/a | S2, S4, S5 | `details denied` | `Access to environment details was denied; only the name is visible.` |

Notes:

- No glyph case exists for mwaa: every signal moves the row off green (the color is the signal; per the color-findings conformance contract, any issue-severity finding is color-bearing). S1 is driven by all issue-colored rows under the standard aggregation.
- When `LastUpdate.Status == FAILED` coincides with another signal row (e.g. `ROLLING_BACK`), S4 keeps the state cause with the `(+N)` suffix; the failed-update sentence still appears in S5.
- No raw AWS enum ever reaches a rendered surface — `ErrorCode` and `WebserverAccessMode` values are humanized (`INCORRECT_CONFIGURATION` → `Incorrect configuration`) per the issue-text style gate.
- AccessDenied on `mwaa:ListEnvironments`: the main-menu row carries the error state, never `0` — "you can't see it" must be distinguishable from "it isn't there".

## 4.1 UX review (two sentences)

At 3am every problem row names its cause in the Status column — `pending: awaiting VPC endpoints`, `rolling back: update failed`, `last update failed`, `webserver public` — so the operator can triage without opening detail. The one nuance that needs the detail view is the failed-update error message itself (`LastUpdate.Error.ErrorMessage`), which is too long for the list and lives in S5.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Logging-component-disabled and AirflowVersion-EOL as attention signals — detail-view facts only (a9s-devops 2026-07-14: majority-fire noise / no stable EOL source).
- `sqs` and `vpce` related pivots (§2 Explicitly excluded — not drillable from customer credentials / no customer endpoint IDs).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- Related-panel target set (`alarm`, `ct-events`, `kms`, `logs`, `role`, `s3`, `sg`, `subnet`) — `docs/related-resources.md` § Per-type contract, row `mwaa`.
- Per-target discovery fields and exclusions (`vpc`, `sqs`, `vpce`) — `docs/related-resources.md` § `mwaa`.
- `alarm` pivot via `EnvironmentName` dimension — `a9s-devops (2026-07-14): possible=yes, worth=yes. First triage stop; join key is the env name, no ARN field.`
- `sqs` exclusion — `AWS SDK Go v2 — mwaa/types.Environment § CeleryExecutorQueue` (queue ARN in an AWS-owned account) + `a9s-devops (2026-07-14): possible=no (not drillable), worth=no. Pivot dead-ends in AccessDenied.`
- `vpce` exclusion — `AWS SDK Go v2 — mwaa/types.Environment § WebserverVpcEndpointService, § DatabaseVpcEndpointService` (endpoint-service names, not endpoint IDs) + `a9s-devops (2026-07-14): possible=no, worth=no.`
- Wave 1 `None` (names-only list) — `docs/attention-signals.md` § Data & Analytics, row `mwaa`; `AWS SDK Go v2 — mwaa § ListEnvironments`.
- Status enum and bucket mapping — `docs/attention-signals.md` § Data & Analytics, row `mwaa`; `AWS SDK Go v2 — mwaa/types.Environment § Status` (all twelve values documented on the SDK shape, incl. PENDING "paused until you create the required VPC endpoints", UPDATE_FAILED "restored to its previous state … ready to use", UNAVAILABLE "did not return to its previous state and is not stable").
- `LastUpdate.Status == FAILED` finding + error surface — `docs/attention-signals.md` § Data & Analytics, row `mwaa`; `AWS SDK Go v2 — mwaa/types.Environment § LastUpdate` (`LastUpdate.Status`, `LastUpdate.Error.ErrorMessage`); `a9s-devops (2026-07-14): possible=yes, worth=yes. "My change didn't take" case; Warning not Broken because the environment still serves.`
- `WebserverAccessMode` public finding — `docs/attention-signals.md` § Data & Analytics, row `mwaa`; `AWS SDK Go v2 — mwaa/types.Environment § WebserverAccessMode`; `a9s-devops (2026-07-14): possible=yes, worth=yes (low-severity). Analogous to dbi PubliclyAccessible.`
- Logging-disabled and AirflowVersion-EOL exclusions — `a9s-devops (2026-07-14): possible=yes, worth=no. Majority-fire noise / no stable EOL source.`
- AccessDenied honest-degradation contract — `a9s-devops (2026-07-14): worth=yes. "0 environments" during an incident misdirects triage; error state, never 0.`
- Wave 3 metric names — `docs/attention-signals.md` § Data & Analytics, row `mwaa` (`AWS/MWAA` namespace).
- S1–S5 surface definitions — `docs/attention-signals.md` § Visualization Surfaces.
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
