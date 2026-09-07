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

**Source API**: [GetEnvironment](https://docs.aws.amazon.com/mwaa/latest/API/API_GetEnvironment.html)

Transcribed from `docs/attention-signals.md § Signals § DATA & ANALYTICS` row `mwaa`.

### 3.1 Wave 1 — zero extra API calls

The list API returns environment name strings only, so the fetcher reads each environment with `GetEnvironment` before it builds the row. Every signal below is computed from that response as the row is built, with no second pass. The pattern matches `eks`, `ng` and `ddb`.

AccessDenied contract: when the profile's role denies `mwaa:ListEnvironments`, the main-menu row shows the error state — never `0`. An operator who reads "0 environments" during an incident wrongly concludes the account runs no Airflow.

- **Signal**: `Status` in `CREATING` / `CREATING_SNAPSHOT` / `PENDING` / `UPDATING` / `ROLLING_BACK` / `MAINTENANCE`.
  - **State bucket**: Warning.
  - **API call**: `GetEnvironment`, one per environment.
  - **Cost shape**: per-resource.

- **Signal**: `Status == CREATING_SNAPSHOT`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `Status == PENDING`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `Status == UPDATING`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `Status == ROLLING_BACK`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `Status == MAINTENANCE`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `Status` in `CREATE_FAILED` / `UPDATE_FAILED` / `UNAVAILABLE`.
  - **State bucket**: Broken.
  - **API call**: `GetEnvironment`, one per environment.
  - **Cost shape**: per-resource.

- **Signal**: `LastUpdate.Status == FAILED` on an `AVAILABLE` environment — the last update silently failed while the environment keeps serving the previous configuration ("my change didn't take"). Surface `LastUpdate.Error.ErrorMessage` (and a humanized `ErrorCode`) as the cause.
  - **State bucket**: Broken.
  - **API call**: `GetEnvironment`, one per environment (same call as above).
  - **Cost shape**: per-resource.

- **Signal**: `Status == UNAVAILABLE`.
  - **State bucket**: Broken.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `Status` in `DELETING` / `DELETED`.
  - **State bucket**: Dim.
  - **API call**: `GetEnvironment`, one per environment.
  - **Cost shape**: per-resource.

- **Signal**: `Status == DELETED`.
  - **State bucket**: Dim.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `LastUpdate.Status == FAILED` on `AVAILABLE`.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

- **Signal**: `WebserverAccessMode` in `PUBLIC_ONLY` / `PUBLIC_AND_PRIVATE` — the Airflow webserver is reachable from the internet (still IAM-authed, but exposed; analogous to `dbi` `PubliclyAccessible`).
  - **State bucket**: Warning.
  - **API call**: `GetEnvironment`, one per environment (same call as above).
  - **Cost shape**: per-resource.

- **Signal**: `airflow:GetEnvironment` denied for a listed environment name — the row is KEPT with the name only ("you can't see it" ≠ "it isn't there"; live witness 2026-07-14: a readonly role allowed `ListEnvironments` but denied `GetEnvironment`, which must not fake an empty list). The per-name failures also aggregate into the fetch error (flash + `!` log).
  - **State bucket**: Warning.
  - **API call**: `GetEnvironment`, one per environment (the denial IS the response).
  - **Cost shape**: per-resource.

Deliberately not signals (a9s-devops 2026-07-14): any `LoggingConfiguration` component disabled (logging is opt-in per component and cost-driven; a warning would fire on most environments — noise, not signal; detail-view fact only) and `AirflowVersion` EOL (no stable AWS source for the deprecation schedule; a hardcoded table goes stale and lies; fact only).

- **Signal**: `GetEnvironment` answered with nothing usable.
  - **State bucket**: Warning.
  - **How obtained**: read off what the fetcher already holds for the row, with no extra call.

### 3.2 Wave 2 — bounded extra API calls

All signals come from `GetEnvironment` per environment (N+1; accounts run 1–5 environments, so the fan-out is small).

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `AWS/MWAA` metrics: `SchedulerHeartbeat` (the classic MWAA-outage canary), DAG-processing `ImportErrors`/`TotalParseTime`, `QueuedTasks`/`RunningTasks`, worker/scheduler CPU+memory.
- OUT OF SCOPE: AirflowVersion-EOL check (no stable API source).

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `mwaa`: Wave 1 issue-colored rows only — this type registers no Wave 2 enricher, so nothing else bumps the count.
<!-- END GENERATED: badge -->

One row per signal from §3. All mwaa signals are Wave 2 because `ListEnvironments` is opaque; the `GetEnvironment` pass sets the row color, so the state-bucket rows behave like Wave 1 colors to the operator (yellow/red is the attention signal, S3 suppressed because the row is not green):

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| `Status == CREATING` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `creating` |
| `Status == CREATING_SNAPSHOT` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `creating snapshot` |
| `Status == PENDING` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `pending: awaiting VPC endpoints` |
| `Status == UPDATING` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `updating` |
| `Status == ROLLING_BACK` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `rolling back: update failed` |
| `Status == MAINTENANCE` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `maintenance in progress` |
| `Status == CREATE_FAILED` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `create failed` |
| `Status == UPDATE_FAILED` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `update failed: rolled back` |
| `Status == UNAVAILABLE` | 1 | Broken | `!` | S1, S2, S3, S4, S5 | `unavailable: not stable` |
| `Status == DELETING` | 1 | Dim | n/a | S2, S4 | `deleting` |
| `Status == DELETED` | 1 | Dim | n/a | S2, S4 | `deleted` |
| `LastUpdate.Status == FAILED` on `AVAILABLE` | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `last update failed` |
| `WebserverAccessMode` public | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `webserver public` |
| `GetEnvironment` denied | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `details denied` |
| `GetEnvironment` answered with nothing usable | 1 | Warning | `~` | S1, S2, S3, S4, S5 | `details unavailable` |

Notes:

- No glyph case exists for mwaa: every signal moves the row off green (the color is the signal; per the color-findings conformance contract, any issue-severity finding is color-bearing). S1 is driven by all issue-colored rows under the standard aggregation.
- When `LastUpdate.Status == FAILED` coincides with another signal row (e.g. `ROLLING_BACK`), S4 keeps the state cause with the `(+N)` suffix; the failed-update sentence still appears in S5.
- No raw AWS enum ever reaches a rendered surface — `ErrorCode` and `WebserverAccessMode` values are humanized (`INCORRECT_CONFIGURATION` → `Incorrect configuration`) per the issue-text style gate.
- AccessDenied on `mwaa:ListEnvironments`: the main-menu row carries the error state, never `0` — "you can't see it" must be distinguishable from "it isn't there".
- `GetEnvironment` denied: an authorization failure renders `details denied`; a non-authorization describe failure (nil body, transient error) renders the neutral `details unavailable`.

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
- Names-only list response — `AWS SDK Go v2 — mwaa § ListEnvironments`.
- Status enum and bucket mapping — `docs/attention-signals.md § Signals § DATA & ANALYTICS` row `mwaa`; `AWS SDK Go v2 — mwaa/types.Environment § Status` (all twelve values documented on the SDK shape, incl. PENDING "paused until you create the required VPC endpoints", UPDATE_FAILED "restored to its previous state … ready to use", UNAVAILABLE "did not return to its previous state and is not stable").
- `LastUpdate.Status == FAILED` finding + error surface — `docs/attention-signals.md § Signals § DATA & ANALYTICS` row `mwaa`; `AWS SDK Go v2 — mwaa/types.Environment § LastUpdate` (`LastUpdate.Status`, `LastUpdate.Error.ErrorMessage`); `a9s-devops (2026-07-14): possible=yes, worth=yes. "My change didn't take" case; Warning not Broken because the environment still serves.`
- `WebserverAccessMode` public finding — `docs/attention-signals.md § Signals § DATA & ANALYTICS` row `mwaa`; `AWS SDK Go v2 — mwaa/types.Environment § WebserverAccessMode`; `a9s-devops (2026-07-14): possible=yes, worth=yes (low-severity). Analogous to dbi PubliclyAccessible.`
- Logging-disabled and AirflowVersion-EOL exclusions — `a9s-devops (2026-07-14): possible=yes, worth=no. Majority-fire noise / no stable EOL source.`
- AccessDenied honest-degradation contract — `a9s-devops (2026-07-14): worth=yes. "0 environments" during an incident misdirects triage; error state, never 0.`
- Wave 3 metric names — `docs/attention-signals.md § Not yet implemented`.
- S1–S5 surface definitions — `docs/attention-signals.md § Visualization Surfaces`.
- Read-only invariant — `docs/architecture.md` § "What is a9s?".

<!-- BEGIN GENERATED: header -->
mwaa — DATA & ANALYTICS. Lifecycle key: `status`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| mwaa.warn.creating | creating | warn | wave1 | Environment is being provisioned; Airflow is not yet reachable. |
| mwaa.warn.creating\_snapshot | creating snapshot | warn | wave1 | The environment is snapshotting its metadata database before an update or upgrade. |
| mwaa.warn.pending | pending: awaiting VPC endpoints | warn | wave1 | Creation is paused until the required VPC endpoints exist in your VPC. |
| mwaa.warn.updating | updating | warn | wave1 | Environment update in progress; workers may be replaced. |
| mwaa.warn.rolling\_back | rolling back: update failed | warn | wave1 | Update or upgrade failed; the environment is restoring the latest metadata snapshot. |
| mwaa.warn.maintenance | maintenance in progress | warn | wave1 | Scheduled maintenance is running; the environment may be briefly unavailable. |
| mwaa.broken.create\_failed | create failed | broken | wave1 | Environment creation failed and the environment was not created. |
| mwaa.broken.update\_failed | update failed: rolled back | broken | wave1 | Update failed; environment was restored to its previous state and is usable. |
| mwaa.broken.unavailable | unavailable: not stable | broken | wave1 | Environment failed and did not return to a stable state; contact AWS support. |
| mwaa.dim.deleting | deleting | dim | wave1 | Environment is being deleted. |
| mwaa.dim.deleted | deleted | dim | wave1 | Environment has been deleted. |
| mwaa.warn.last\_update\_failed | last update failed | warn | wave1 | The last update to this environment failed, so it is still running its previous configuration; the error code and message are listed below. Fix the cause and update again. |
| mwaa.warn.webserver\_public | webserver public | warn | wave1 | The Airflow web server answers from the public internet, so its login page is reachable by anyone; the access mode is listed below. Switch the environment to private-only access from your VPC. |
| mwaa.warn.details\_denied | details denied | warn | wave1 | Access to environment details was denied; only the name is visible. |
| mwaa.warn.details\_unavailable | details unavailable | warn | wave1 | Details could not be retrieved; only the name is visible. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| alarm | CW Alarms | yes |
| kms | KMS Key | no |
| logs | Log Groups | no |
| role | IAM Roles | no |
| s3 | S3 Buckets | no |
| sg | Security Groups | no |
| subnet | Subnets | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
