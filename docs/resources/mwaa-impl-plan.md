# mwaa — Implementation Plan

Derived from [`docs/resources/mwaa.md`](mwaa.md). Spec has zero TBDs; deliberate exclusions carry §6 citations.

## 0. Architecture decision — in-fetcher N+1 (the EKS pattern)

`ListEnvironments` returns names only; every signal AND every related-panel field comes from the same `GetEnvironment` call. Accounts run 1–5 environments. Therefore the fetcher does `ListEnvironments` + `GetEnvironment` per name (both wrapped in `RetryOnThrottle`, partial failure aggregated per rules E3/E5) and emits ALL findings fetcher-side (`Source: "wave1"`). There is NO `mwaa_issue_enrichment.go` and NO `mwaa_detail_enrichment.go` — same shape as `eks` ("types without a `Wave2` enricher still perform in-fetcher Wave 2 work"). The spec's §3.2 "Wave 2" labels the *cost class* (per-resource describe), not a separate enricher artifact.

`Resource.ID` = environment **name** (bare, display-friendly). MWAA APIs address environments by name — no ARN params exist in the API surface (rule E7 trivially satisfied; `Fields["arn"]` still populated from `Environment.Arn` for ct-events).

AccessDenied contract: `ListEnvironments` error → fetcher returns `(FetchResult{}, err)`. NEVER `(empty, nil)`. The framework renders the menu error state — the operator must be able to distinguish "can't see" from "isn't there".

Per-name `GetEnvironment` failure (live witness 2026-07-14: readonly role allows List, denies `airflow:GetEnvironment`) → the row is KEPT as a name-only degraded resource: `Resource{ID: name, Fields: name-only}` + finding `mwaa.warn.details_denied` (Phrase `details denied`, Warning), AND the failure still aggregates into the composite fetch error (flash + `!` log). Dropping the row fakes an empty/partial list — banned. Demo witness: `warn-airflow-details-denied` (the mwaa fake returns AccessDeniedException for that name's GetEnvironment — a legitimate operational state, not an adversarial fixture). Menu badge 12→13.

## 1. Behavioral test spec (pseudocode)

```text
TEST: healthy_available_silence   (U1)
GIVEN: environment prod-airflow-reporting, Status=AVAILABLE, LastUpdate SUCCESS, WebserverAccessMode=PRIVATE_ONLY
WHEN:  the list is fetched and rendered
THEN:  row green; S4 blank; Findings empty; no glyph; no S1 bump

TEST: status_warning_phrases   (U2, one case per status)
GIVEN: environments with Status=CREATING / CREATING_SNAPSHOT / PENDING / UPDATING / ROLLING_BACK / MAINTENANCE
WHEN:  list fetched
THEN:  row yellow; S4 = exact §4 phrase:
       creating | creating snapshot | pending: awaiting VPC endpoints | updating
       | rolling back: update failed | maintenance in progress
       one SevWarn Finding each (Source wave1); no glyph (non-green)

TEST: status_broken_phrases   (U2)
GIVEN: Status=CREATE_FAILED / UPDATE_FAILED / UNAVAILABLE
WHEN:  list fetched
THEN:  row red; S4 = create failed | update failed: rolled back | unavailable: not stable
       one SevBroken Finding each; no glyph; S1 counts each instance once

TEST: status_dim_deleting   (U2)
GIVEN: Status=DELETING
WHEN:  list fetched
THEN:  row gray; S4 = deleting; SevDim finding; no S1 bump

TEST: status_mapping_table_unit
GIVEN: the status→bucket mapping function, table-driven over all 12 enum values (incl. DELETED)
THEN:  buckets per spec §3.2 (DELETED→Dim covered here; no demo fixture — a deleted
       environment does not appear in ListEnvironments in practice)

TEST: last_update_failed_on_available   (U3)
GIVEN: warn-airflow-stale-update: Status=AVAILABLE, LastUpdate.Status=FAILED,
       LastUpdate.Error = {ErrorCode:"INCORRECT_CONFIGURATION", ErrorMessage:"Scheduler failed to launch: requirements.txt install failed"}
WHEN:  list fetched
THEN:  row GREEN (stays healthy); `~ ` glyph prefix; S4 = "last update failed"
       detail Attention contains "Last update failed:" and the ErrorMessage text
       S1 does NOT bump (~ finding)

TEST: webserver_public_on_available   (U3)
GIVEN: warn-airflow-public: Status=AVAILABLE, WebserverAccessMode=PUBLIC_ONLY
WHEN:  list fetched
THEN:  row GREEN; `~ ` glyph; S4 = "webserver public"
       detail contains "reachable from the internet"; no S1 bump

TEST: multi_findings_on_green_suffix   (U7a analog — framework (+N))
GIVEN: warn-airflow-multi: AVAILABLE + LastUpdate.Status=FAILED + WebserverAccessMode=PUBLIC_ONLY
WHEN:  list fetched
THEN:  Findings = [last update failed, webserver public] in §4 order
       S4 renders "last update failed (+1)"; row green; `~ ` glyph

TEST: finding_on_nongreen_row_stacks   (U7b/U7c analog)
GIVEN: warn-airflow-rollback: Status=ROLLING_BACK AND LastUpdate.Status=FAILED (+Error)
WHEN:  list fetched / detail opened
THEN:  row yellow; NO glyph (non-green); S4 = "rolling back: update failed (+1)"
       detail Attention lists BOTH entries: the state phrase and the failed-update
       sentence with its ErrorMessage rows — no finding silently disappears

TEST: detail_surfaces_every_phrase   (U7e)
GIVEN: warn-airflow-multi
WHEN:  detail opened
THEN:  rendered detail contains "Last update failed" AND "Webserver public"
       (capitalizeFirst applied at render; no (+N) suffix in detail)

TEST: fetcher_populates_findings   (U7f)
GIVEN: each fixture above
WHEN:  FetchMWAAEnvironmentsPage runs
THEN:  ordered Phrase slice of got.Findings deep-equals expectation per fixture
       (Healthy → empty; multi → [last update failed, webserver public])

TEST: access_denied_is_an_error_not_zero
GIVEN: fake ListEnvironments returns AccessDeniedException
WHEN:  fetch runs
THEN:  (FetchResult{}, err) with err non-nil containing "AccessDenied"
       — NEVER an empty successful result

TEST: partial_get_failure   (U12 / E3 / E5)
GIVEN: 5 environments listed; GetEnvironment errors on 2 (AccessDenied, NotFound)
WHEN:  fetch runs
THEN:  3 rows returned AND composite error "mwaa: GetEnvironment failed for 2 of 5..."
       scenario: list renders 3 rows, FlashMsg{IsError:true}, errorHistory has both IDs

TEST: related_targets   (one per §2 pivot)
GIVEN: prod-airflow-etl (graph root)
THEN:  kms→1 (KmsKey), logs→5 (five LoggingConfiguration ARNs), role→1 (ExecutionRoleArn),
       s3→1 (SourceBucketArn), sg→2 (SecurityGroupIds), subnet→2 (SubnetIds),
       alarm→2 (AWS/MWAA alarms with EnvironmentName dimension), ct-events→drillable

TEST: wave3_anti_tests
GIVEN: any fixture
THEN:  no CloudWatch metric strings (SchedulerHeartbeat etc.) surface anywhere;
       AirflowVersion renders as a plain detail fact, never as a finding;
       disabled logging components (prod-airflow-reporting has WebserverLogs.Enabled=false)
       produce NO finding
```

## 2. Fixture list (single source: `core/demo/fixtures/mwaa.go`)

All values synthetic (account `123456789012`, region of the demo profile). `<name>` = environment name = `Resource.ID`.

```text
FIXTURE: prod-airflow-etl   (GRAPH ROOT)
AVAILABLE, AirflowVersion 2.10.3, EnvironmentClass mw1.large, MinWorkers 1 / MaxWorkers 5,
Schedulers 2, MinWebservers/MaxWebservers 2, WebserverAccessMode PRIVATE_ONLY,
EndpointManagement SERVICE, WeeklyMaintenanceWindowStart WED:22:30, LastUpdate SUCCESS.
KmsKey = existing kms fixture key ARN.
LoggingConfiguration: all five components Enabled INFO, CloudWatchLogGroupArn
  arn:aws:logs:...:log-group:airflow-prod-airflow-etl-{DAGProcessing,Scheduler,WebServer,Worker,Task}
  → five matching `logs` fixture entries.
ExecutionRoleArn → existing role fixture; ServiceRoleArn = service-linked role ARN (fact only).
SourceBucketArn → existing s3 fixture bucket; DagS3Path dags; RequirementsS3Path requirements/requirements.txt.
NetworkConfiguration: 2 subnets + 2 security groups from existing vpc-graph fixtures.
CeleryExecutorQueue = arn:aws:sqs:...airflow-celery-<uuid> (detail fact only — no pivot).
WebserverUrl <uuid>-vpce.c1.airflow.<region>.on.aws.
Alarm siblings: 2 CloudWatch alarms, namespace AWS/MWAA, dimension EnvironmentName=prod-airflow-etl.
Pivot expectation: kms 1, logs 5, role 1, s3 1, sg 2, subnet 2, alarm 2 → ≥2 on 4/7 (57%).

FIXTURE: prod-airflow-reporting
AVAILABLE, healthy silence row. PRIVATE_ONLY, LastUpdate SUCCESS.
WebserverLogs.Enabled=false + TaskLogs INFO (wave-3 anti-test: no finding for disabled component).
mw1.small, 1 scheduler.

FIXTURE: warn-airflow-creating        Status=CREATING
FIXTURE: warn-airflow-snapshotting    Status=CREATING_SNAPSHOT
FIXTURE: warn-airflow-pending         Status=PENDING
FIXTURE: warn-airflow-updating        Status=UPDATING
FIXTURE: warn-airflow-maintenance     Status=MAINTENANCE
FIXTURE: warn-airflow-rollback        Status=ROLLING_BACK + LastUpdate{Status:FAILED,
         Error{ErrorCode:"ROLLBACK_IN_PROGRESS", ErrorMessage:"Update to 2.11.0 failed; restoring metadata snapshot"}}
         → two findings on a yellow row (state + failed update): S4 "rolling back: update failed (+1)"
FIXTURE: broken-airflow-create-failed Status=CREATE_FAILED
FIXTURE: broken-airflow-update-failed Status=UPDATE_FAILED + LastUpdate FAILED
         (Error{ErrorCode:"INCORRECT_CONFIGURATION", ErrorMessage:"Environment update failed; rolled back to previous state"})
FIXTURE: broken-airflow-unavailable   Status=UNAVAILABLE
FIXTURE: dim-airflow-deleting         Status=DELETING
FIXTURE: warn-airflow-stale-update    AVAILABLE + LastUpdate{Status:FAILED, Error{ErrorCode:"INCORRECT_CONFIGURATION",
         ErrorMessage:"Scheduler failed to launch: requirements.txt install failed"}}
FIXTURE: warn-airflow-public          AVAILABLE + WebserverAccessMode=PUBLIC_ONLY, WebserverUrl public hostname
FIXTURE: warn-airflow-multi           AVAILABLE + LastUpdate FAILED (as stale-update) + PUBLIC_ONLY
         → multi-issue instance for 9.2/9.5: S4 "last update failed (+1)"
```

Skips (recorded, not silent): DELETED has no fixture — a deleted environment does not appear in `ListEnvironments`; the mapping is covered by the table-driven unit test. U4 (`!`-on-green) N/A — spec has no `!`-on-green signal; S1 is driven by the three Broken fixtures. U7d N/A — only `~` background findings exist.

Adversarial fixtures (inline in tests only, NEVER in the demo file): nil `Environment` in GetEnvironment output, AccessDenied on ListEnvironments, per-name GetEnvironment failures for the U12 case, environment with nil LoggingConfiguration/NetworkConfiguration.

Menu badge expectation: issues:12 — unifiedIssueCount counts issue-COLORED rows (Warning + Broken): 6 Warning-state fixtures + 3 Broken + 3 background-warning fixtures (stale-update, public, multi; per the color-findings conformance gate every issue-severity finding is color-bearing, so these rows render yellow — no glyph-on-green exists for mwaa). Dim doesn't bump. Menu count moves 67→68; type count 66→67. A `dim-airflow-deleted` fixture (Status=DELETED) witnesses the `mwaa.dim.deleted` finding (demo-state coverage gate requires every registered FindingDef to fire).

## 3. Contract surface gap analysis

New service — all four contract-surface files absent; no existing fetcher; no catalog entry; SDK module `service/mwaa v1.42.1` already in go.mod (uncommitted-clean: committed with the spec).

Full file scope union (supersedes the skill's default list — new-service wiring included):

| File | Action | Owner |
|------|--------|-------|
| `core/demo/fixtures/mwaa.go` | create (§2) | 6a coder |
| `core/demo/fixtures/{alarm,logs,kms,role,s3,sg/vpc-graph}` siblings | targeted adds so every §2 pivot resolves | 6a coder |
| `core/demo/fakes/mwaa.go` | create — typed fake over the two APIs; validates unknown names with ResourceNotFoundException; permissive-free (rule U15 spirit; MWAA has no ARN params) | 6a coder |
| `core/demo/client.go` | `clients.MWAA = fakes.NewMWAA()` | 6a coder |
| `core/aws/mwaa.go` | create — `FetchMWAAEnvironmentsPage` (ListEnvironments + GetEnvironment N+1, RetryOnThrottle both, E3/E5 aggregation, findings per §1, fields incl. arn/airflow_version/environment_class/workers/schedulers/webserver_access_mode/endpoint_management/weekly_maintenance_window/webserver_url/source_bucket/kms_key/execution_role/log-group ARNs/subnets/sgs/celery_queue) | 7 coder |
| `core/aws/mwaa_interfaces.go` | create — `MWAAAPI` narrow interface (ListEnvironments, GetEnvironment) | 7 coder |
| `core/aws/mwaa_related.go` | create — checkers per §2: kms/logs/role/s3/sg/subnet field-driven, alarm via sibling-cache dimension match, ct-events via `ctEventsCheckerFor("mwaa")` | 7 coder |
| `core/aws/client.go` | `MWAA MWAAAPI` field + `mwaa.NewFromConfig(cfg)` | 7 coder |
| `core/aws/catalog_data.go` | `ResourceTypeDef` literal: ShortName `mwaa`, Aliases `["mwaa","airflow"]` (uniqueness gate: `airflow` free as of 2026-07-14), Category DATA, columns per defaults, Color from findings helpers, Fetcher, Related, Navigable (role/kms/s3/logs/sg/subnet field paths via NavIDFromValue-compatible extractors), FieldKeys, Findings []catalog.FindingDef, CloudTrailKey | 7 coder |
| `core/config/defaults_data.go` | one Status column + identity columns: Name, Airflow (version), Class, Workers, Schedulers, Access, Created; Status column flagged per humanized-enum convention | 7 coder |
| `.a9s/views/mwaa.yaml` | generated via `go run ./cmd/viewsgen/` | 7 coder |
| `tests/unit/aws_mwaa_test.go` | fetcher tests §1 | 6b QA |
| `tests/unit/aws_mwaa_related_test.go` | checker tests §2 | 6b QA |
| `tests/integration/scenario_mwaa_visual_test.go` | render gate | runner (phase 8) |
| `tests/integration/scenario_related_drill_through_test.go` | +1 row `{"mwaa graph root", "mwaa", fixtures.ProdAirflowEtlID}` | runner (phase 8) |
| `tests/integration/demo_full_integration_test.go` | expectedTopLevel: mwaa entry (15 rows, issues:3); menu count 67→68 | runner (phase 8) |
| `docs/README.tmpl.md` + `README.md` (regen) + `website/content/resources.md` | counts 66→67 + services-table row | runner (phase 9 docs) |
| `.a9s/views_reference.yaml` | regen via `go run ./cmd/refgen/` (SDK module added) | runner |
| `core/demo/handlers.go` | only if typed-fake path insufficient (expected: no change) | 6a coder |

No `mwaa_issue_enrichment.go`, no `mwaa_detail_enrichment.go` (§0). `Wave2` field omitted on the catalog literal.

## 4. Coverage matrix mapping

| ID | Status | Fixture / justification |
|----|--------|--------------------------|
| U1 | covered | prod-airflow-reporting |
| U2 | covered | 6 warning + 3 broken + 1 dim fixtures |
| U3 | N/A | no glyph-on-green: background findings are color-bearing Warning (conformance gate) |
| U4 | N/A | spec has no `!`-on-green signal |
| U5 | covered | any warn-/broken- fixture |
| U6 | covered | issues:12 (9 state-colored + 3 background-warning rows) |
| U7a | covered | warn-airflow-multi → "last update failed (+1)" |
| U7b | covered (analog) | warn-airflow-rollback → "rolling back: update failed (+1)" (all findings fetcher-local; no enricher exists) |
| U7c | covered | rollback detail shows failed-update sentence + rows |
| U7d | N/A | only `~` background findings |
| U7e | covered | warn-airflow-multi detail enumerates both phrases |
| U7f | covered | fetcher_populates_findings deep-equals |
| U7f' | N/A | no cross-ref signals; no enricher |
| U8 | N/A | no Wave-1/Wave-2 severity collision possible |
| U9 | covered | prod-airflow-etl pivots (≥2 ratio 4/7) |
| U10 | covered | scenario ExpectViewNotContains |
| U11 | N/A (adapted) | no enricher; fetcher findings keep Phrase ≠ AttentionDetails rows — unit-asserted on stale-update fixture |
| U12 | covered | partial_get_failure scenario |
| U13 | covered | phase-9.6.a grep |
| U14 | N/A | MWAA API has no ARN params; ID=name is the API's own addressing |
| U15 | covered | fake rejects unknown environment names |
| U16 | covered | scenario asserts AssertNoEnrichmentErrors (vacuous but pinned) |
