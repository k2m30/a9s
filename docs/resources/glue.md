---
shortName: glue
name: Glue Jobs
awsApiRef: https://docs.aws.amazon.com/glue/latest/webapi/API_Job.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# glue — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `glue`
- **Display name**: Glue Jobs
- **AWS API reference**: <https://docs.aws.amazon.com/glue/latest/webapi/API_Job.html>
- **List API**: `GetJobs` — returns `Job[]`. Per `attention-signals.md` §Data & Analytics, this is a **configuration-only** shape: it carries the job definition (`Name`, `Role`, `Command`, `Connections`, `DefaultArguments`, `SecurityConfiguration`, `LogUri`, `CreatedOn`, `LastModifiedOn`, etc.) but no runtime state. Every attention signal therefore lives in Wave 2.
- **Describe API (if any)**: `GetJobRuns(JobName, MaxResults=1)` per job — used in Wave 2 to read the latest `JobRun.JobRunState` and `JobRun.ErrorMessage`. The API returns runs ordered by `StartedOn` descending, so `MaxResults=1` yields the most recent execution.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `athena`, `cfn`, `kms`, `logs`, `role`, `s3`, `secrets`, `ct-events`.

### `alarm`

- **Why related**: CloudWatch alarms that watch this job's run failures so on-call gets paged when a nightly ETL breaks — this is the hand-off from "a9s shows it broken" to "pager fires."
- **How discovered**: cross-reference the already-loaded `alarm` list by `AlarmActions`/`Dimensions` referencing the Glue job name; fall back to name-contains match on `AlarmDescription` — a9s-devops persona (2026-04-20): possible=yes, worth=yes. Glue does not expose an inverse index of alarms; CloudWatch metric-alarm dimensions for Glue use `JobName` as the dimension key, which is the field the already-loaded alarm list can be filtered on client-side without extra API calls.
- **Count shown**: yes.

### `athena`

- **Why related**: Athena workgroups whose queries consume the Glue Data Catalog that this Glue job populates — when the job fails, downstream Athena dashboards go stale and the operator needs the pivot to know which consumers to warn.
- **How discovered**: show the full already-loaded `athena` list — a9s-devops persona (2026-04-20): possible=yes, worth=yes. Glue jobs do not store a per-job list of consuming Athena workgroups (the Catalog is a shared-namespace resource), and there is no cheap inverse index. A9s therefore links `glue` → `athena` as an account-wide pivot rather than a per-job filter; this mirrors the `related-resources.md` rationale that Athena queries Glue Catalog.
- **Count shown**: yes (the full account-wide workgroup count).

### `cfn`

- **Why related**: CloudFormation stack that created this Glue job — the IaC source of truth; operator pivots here to see what the stack wanted the job to look like vs. what it is now.
- **How discovered**: read tags on the Glue job; pivot on the `aws:cloudformation:stack-name` tag value and cross-reference the loaded `cfn` list by `StackName` — a9s-devops persona (2026-04-20): possible=yes, worth=yes. CloudFormation stamps `aws:cloudformation:stack-name` and `aws:cloudformation:stack-id` tags on every resource it creates, and Glue jobs carry tags retrievable via `GetTags(ResourceArn)`. If the tag is absent the job was created outside CFN and the pivot shows zero.
- **Count shown**: yes (typically 0 or 1).

### `kms`

- **Why related**: KMS key that encrypts the job bookmark, the S3 data at rest, and the CloudWatch Logs output — a rotated or revoked key is the classic "job suddenly failing with AccessDenied" root cause.
- **How discovered**: read `Job.SecurityConfiguration` (name); resolve to a `SecurityConfiguration` via `GetSecurityConfiguration`, then walk `EncryptionConfiguration.{S3Encryption[].KmsKeyArn, CloudWatchEncryption.KmsKeyArn, JobBookmarksEncryption.KmsKeyArn}` and cross-reference the loaded `kms` list by key ARN — a9s-devops persona (2026-04-20): possible=yes, worth=yes. Glue carries KMS references only through a named SecurityConfiguration; the SDK confirms the three encryption sub-fields. If `SecurityConfiguration` is empty the job uses AWS-owned keys and the pivot correctly shows zero.
- **Count shown**: yes (0–3 keys).

### `logs`

- **Why related**: CloudWatch Logs group where Spark driver/executor output lands — the first place an operator goes when a run fails, independent of a9s's own cause summary.
- **How discovered**: two sources combined — (a) the Glue default log group `/aws-glue/jobs/output` and `/aws-glue/jobs/error` (continuous-logging convention), (b) `Job.DefaultArguments["--continuous-log-logGroup"]` when set. Cross-reference the loaded `logs` list by log-group name — a9s-devops persona (2026-04-20): possible=yes, worth=yes. The default groups are Glue convention; the argument override is the documented way to rename them. `Job.LogUri` on the `Job` shape is the *deprecated* S3-based log path and should not drive the `logs` pivot.
- **Count shown**: yes.

### `role`

- **Why related**: IAM role the Glue job assumes when it runs — wrong permissions here are the single most common cause of `FAILED` runs (S3 access denied, Secrets Manager unreadable, KMS decrypt blocked).
- **How discovered**: read `Job.Role` directly (role ARN or role name); cross-reference the loaded `role` list by ARN or name — AWS SDK Go v2 — glue/types.Job § Role confirms the field exists on the list response.
- **Count shown**: yes (exactly 1).

### `s3`

- **Why related**: S3 buckets holding the job script, the temp directory Spark spills to, and the source/sink datasets — a `NoSuchBucket` or lifecycle-expired script object silently kills job startup.
- **How discovered**: parse `Job.Command.ScriptLocation` (always `s3://…`) and the well-known Glue arguments in `Job.DefaultArguments`: `--TempDir`, `--spark-event-logs-path`, `--extra-py-files`, `--extra-jars`, `--extra-files`. Extract bucket names and cross-reference the loaded `s3` list — a9s-devops persona (2026-04-20): possible=yes, worth=yes. These are the documented Special Parameters on the Glue arguments page; Glue does not offer a first-class "buckets this job reads" list, so argument parsing is the idiomatic path. User data paths inside the script itself are invisible at this layer and are legitimately out of scope.
- **Count shown**: yes (typically 1–4).

### `secrets`

- **Why related**: Secrets Manager secrets referenced by Glue Connections (database passwords, JDBC credentials) — a rotated-but-not-propagated secret turns a green Glue row into a red one the next run.
- **How discovered**: read `Job.Connections.Connections[]` (names); resolve each via `GetConnection`, read `Connection.ConnectionProperties["SECRET_ID"]` when present; cross-reference the loaded `secrets` list by secret ARN/name — a9s-devops persona (2026-04-20): possible=yes, worth=yes. Glue Connections that use Secrets Manager store the secret ID under the `SECRET_ID` connection property; this is the only on-resource path from a Glue job to the consumed secret.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Universal pivot — who created, updated, started, or deleted this job; who invoked `StartJobRun`; who modified its IAM role.
- **How discovered**: pre-built CloudTrail query scoped to the Glue job's `Name` (and to `StartJobRun`/`UpdateJob`/`DeleteJob` event names when the operator wants to narrow).
- **Count shown**: unknown (CloudTrail queries are windowed; a reliable total isn't available without a separate count call).
- Universal pivot — applies to every registered type; see `related-resources.md` §Policy.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md` §Data & Analytics.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — the list API does not return fields usable for attention. `GetJobs` returns job *definitions* only; runtime state lives in `JobRun`, which is a separate API.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: latest `JobRunState` in `FAILED` / `TIMEOUT` / `ERROR` / `EXPIRED` → Broken (excluding user-initiated `STOPPED`).
  - **State bucket**: Broken.
  - **API call**: `GetJobRuns(JobName, MaxResults=1)` — one call per Glue job. Runs are returned in descending `StartedOn` order so `MaxResults=1` is always the latest run.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: DPU-hours trend.
- OUT OF SCOPE: bookmark-stuck detection.

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
| latest run `FAILED` | 2 | Broken | `!` | S1, S2, S4, S5 | `last run failed: <ErrorMessage head>` | `Most recent run FAILED on <StartedOn>: <ErrorMessage>.` |
| latest run `TIMEOUT` | 2 | Broken | `!` | S1, S2, S4, S5 | `last run timed out at <Timeout>m` | `Most recent run hit the configured timeout of <Timeout> minutes on <StartedOn>.` |
| latest run `ERROR` | 2 | Broken | `!` | S1, S2, S4, S5 | `last run errored: <ErrorMessage head>` | `Most recent run ended in ERROR on <StartedOn>: <ErrorMessage>.` |
| latest run `EXPIRED` | 2 | Broken | `!` | S1, S2, S4, S5 | `last run expired (queued too long)` | `Most recent run EXPIRED on <StartedOn> — job was queued past its TTL and never started.` |

Notes on the S4 text:

- `<ErrorMessage head>` is the first ~28 characters of `JobRun.ErrorMessage`, truncated on a word boundary, to stay within the 40-char S4 budget. The full message goes in S5.
- When `ErrorMessage` is empty (Glue occasionally omits it for TIMEOUT/EXPIRED), S4 falls back to the state-plus-cause-kind phrasing above (e.g. `last run timed out at 60m`) — never to a bare `FAILED`/`TIMEOUT` keyword.
- The user-initiated `STOPPED` state is **explicitly not a finding** (per `attention-signals.md`). A Glue job whose latest run is `STOPPED` renders green, blank S4, no glyph — exactly as a never-run or healthy job does.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for `FAILED` and `ERROR` rows where `ErrorMessage` is populated — the head of the message ("AccessDenied on s3://…", "Command failed with exit code 1") is enough to route the incident. For `TIMEOUT` and `EXPIRED`, the list shows the failure *kind* but not the *reason* (timeout in seconds, queue TTL); that is acceptable — those states have only one cause each, so the S4 keyword alone is self-explanatory and the operator can press detail for the full sentence if needed.

## 5. Out of Scope

- All §3.3 Wave 3 signals (DPU-hours trend; bookmark-stuck detection) — copied above.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- **Inner-script data paths (S3 reads/writes inside the PySpark job code)** — a9s-devops persona (2026-04-20): possible=no, not available in AWS surface. Glue does not expose the datasets a script reads; static analysis of user code is out of scope for a read-only TUI.
- **Crawlers, triggers, workflows, development endpoints, data-quality rulesets** — a9s-devops persona (2026-04-20): possible=yes (separate Glue APIs), worth=no for now. The `glue` shortName is scoped to Jobs per `related-resources.md`; these sibling Glue object types would warrant their own shortNames (e.g. `glue-crawler`, `glue-trigger`) in a future iteration.
- **DPU-hours / cost signals** — covered under Wave 3 above; requires CloudWatch metrics and a time-series budget a9s intentionally doesn't spend.

## 6. Citations

- a9s golden doc — related-targets list for `glue` — `docs/related-resources.md` § Per-type contract row `glue` and § `### glue`.
- a9s golden doc — Wave 1/Wave 2/Wave 3 contract for `glue` — `docs/attention-signals.md` § Data & Analytics row `glue`.
- a9s golden doc — read-only invariant — `docs/architecture.md` § "What is a9s?" ("a9s never makes write calls to AWS").
- AWS Go SDK v2 — `Job` is a config-only shape (no state field) — `AWS SDK Go v2 — glue/types.Job` (fields `Name`, `Role`, `Command`, `Connections`, `DefaultArguments`, `SecurityConfiguration`, `LogUri`, `CreatedOn`, `LastModifiedOn`).
- AWS Go SDK v2 — Wave 2 signal fields — `AWS SDK Go v2 — glue/types.JobRun § JobRunState`, `§ ErrorMessage`, `§ StartedOn`.
- AWS Go SDK v2 — IAM role discovery — `AWS SDK Go v2 — glue/types.Job § Role`.
- AWS Go SDK v2 — S3 script/temp paths — `AWS SDK Go v2 — glue/types.JobCommand § ScriptLocation` and `glue/types.Job § DefaultArguments`.
- AWS Go SDK v2 — KMS chain via SecurityConfiguration — `AWS SDK Go v2 — glue/types.Job § SecurityConfiguration` and `glue/types.SecurityConfiguration § EncryptionConfiguration`.
- a9s-devops consultation — `alarm` pivot uses `AlarmActions`/`Dimensions` filter on loaded alarms — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. CloudWatch alarms for Glue use JobName as the standard dimension; no inverse-index API, so filtering the already-loaded alarm list is the correct pivot.`
- a9s-devops consultation — `athena` pivot is account-wide (not per-job filterable) — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. Glue Catalog is shared-namespace; Athena→Glue linkage is one-way only, matching the Athena-queries-Glue-Catalog rationale in related-resources.md.`
- a9s-devops consultation — `cfn` pivot via `aws:cloudformation:stack-name` tag — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. CloudFormation stamps this tag on every created resource; reachable via Glue GetTags.`
- a9s-devops consultation — `kms` pivot walks SecurityConfiguration encryption sub-fields — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. KMS references on Glue jobs live only through the named SecurityConfiguration.`
- a9s-devops consultation — `logs` pivot combines Glue convention groups + `--continuous-log-logGroup` arg — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. Default groups and the continuous-logging argument are the documented log destinations; Job.LogUri is the deprecated S3 path.`
- a9s-devops consultation — `s3` pivot parses ScriptLocation and Glue Special Parameters (`--TempDir`, `--spark-event-logs-path`, etc.) — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. No first-class "buckets this job uses" API; argument parsing is the idiomatic path.`
- a9s-devops consultation — `secrets` pivot via Connection `SECRET_ID` property — `a9s-devops persona (2026-04-20): possible=yes, worth=yes. Glue Connections that wrap Secrets Manager expose the secret ID under this property; this is the only on-resource path.`
- a9s-devops consultation — inner-script data paths are not discoverable — `a9s-devops persona (2026-04-20): possible=no, not available in AWS surface. Glue exposes no "datasets this script reads" API; static code analysis is out of scope.`
- a9s-devops consultation — sibling Glue object types (crawlers, triggers, workflows) deliberately excluded for now — `a9s-devops persona (2026-04-20): possible=yes, worth=no. Would warrant their own shortNames in a future iteration; current glue shortName scopes to Jobs per related-resources.md.`
