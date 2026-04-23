---
shortName: ddb
generatedBy: a9s-implement-resource skill
inputs:
  - docs/resources/ddb.md
---

# ddb — Implementation Plan

Derived from `docs/resources/ddb.md`. Authoritative for phase 6 dispatch. The spec doc is the contract; this plan translates it into pseudocode tests, fixtures, and a contract-surface delta.

## Framing

- §3.1 Wave 1 is **empty** — `ListTables` returns names only, so the fetcher has no zero-extra-call signals.
- All §3 signals are Wave 2. `DescribeTable` is a mandatory per-table call for the list itself (otherwise no Status/Items/Size are available) — it is treated as fetcher-local here and the TableStatus → §4 phrase mapping lands in the fetcher. `DescribeContinuousBackups` for PITR stays in the enricher.
- Because there are no Wave 1 signals, universal matrix rows U7a / U7b / U7e / U7f are **N/A** for this resource (they all presuppose Wave-1 warnings being added to `Resource.Issues`). Multi-finding stacking still happens between two Wave-2 findings (e.g. ARCHIVED + PITR off) and is covered by a U7c-equivalent detail-view test.
- `Resource.Issues` IS still populated — by the fetcher — when TableStatus maps to a non-empty §4 phrase. Otherwise the detail-view Attention section would hide it.

## 1. Pseudocode test spec

```text
TEST: healthy_active_blank
GIVEN: a DynamoDB table with TableStatus = ACTIVE, PITR enabled, no SSE CMK
WHEN:  FetchDynamoDBTablesPage runs and the row is rendered
THEN:
  - Resource.Status == ""
  - Resource.Issues is empty
  - list row S2 color = green (Healthy)
  - list row S4 = blank
  - list row S3 = no glyph prefix
  - S1 menu badge does NOT bump (no !-severity finding)

TEST: warn_creating_phrase
GIVEN: a DynamoDB table with TableStatus = CREATING
WHEN:  FetchDynamoDBTablesPage runs
THEN:
  - Resource.Status == "creating"
  - Resource.Issues == ["creating"]
  - list row S2 = yellow (Warning)
  - list row S4 = "creating"
  - list row S3 = no glyph (non-green)

TEST: warn_updating_phrase
GIVEN: a DynamoDB table with TableStatus = UPDATING
WHEN:  FetchDynamoDBTablesPage runs
THEN:
  - Resource.Status == "updating"
  - Resource.Issues == ["updating"]
  - list row S2 = yellow, S4 = "updating", no glyph

TEST: warn_deleting_phrase
GIVEN: a DynamoDB table with TableStatus = DELETING
THEN:  Resource.Status == "deleting"; Issues == ["deleting"]; yellow; no glyph

TEST: warn_archiving_phrase
GIVEN: a DynamoDB table with TableStatus = ARCHIVING
THEN:  Resource.Status == "archiving"; Issues == ["archiving"]; yellow; no glyph

TEST: broken_kms_inaccessible
GIVEN: a DynamoDB table with TableStatus = INACCESSIBLE_ENCRYPTION_CREDENTIALS
WHEN:  FetchDynamoDBTablesPage runs
THEN:
  - Resource.Status == "kms key inaccessible"
  - Resource.Issues == ["kms key inaccessible"]
  - list row S2 = red (Broken)
  - list row S4 = "kms key inaccessible"
  - no glyph

TEST: broken_archived
GIVEN: a DynamoDB table with TableStatus = ARCHIVED, ArchivalSummary.ArchivalReason
       = "INACCESSIBLE_ENCRYPTION_CREDENTIALS"
WHEN:  FetchDynamoDBTablesPage runs
THEN:
  - Resource.Status == "archived: kms key lost"
  - Resource.Issues == ["archived: kms key lost"]
  - list row S2 = red, S4 = "archived: kms key lost", no glyph

TEST: healthy_pitr_off_glyph_and_phrase   (covers U3)
GIVEN: a DynamoDB table with TableStatus = ACTIVE and
       DescribeContinuousBackups → PointInTimeRecoveryStatus = DISABLED
WHEN:  FetchDynamoDBTablesPage runs, then EnrichDynamoDBPITR runs over the result
THEN:
  - Enricher FieldUpdates[id]["status"] == "PITR off"
  - Enricher Findings[id].Severity == "~"
  - Enricher Findings[id].Summary  == "PITR off"
  - After merge: Resource.Fields["status"] == "PITR off"
  - list row S2 = green (Healthy — ~ finding does not downgrade)
  - list row S3 = "~ " prefix
  - list row S4 = "PITR off"
  - S1 menu badge does NOT bump (~ never bumps)

TEST: enricher_summary_not_rows           (covers U11 — enrichment contract)
GIVEN: the same fixture as healthy_pitr_off_glyph_and_phrase
THEN:
  - Findings[id].Summary == "PITR off" (stable short phrase, ≤4 words)
  - For every Row r on the finding: !strings.Contains(Summary, r.Value)

TEST: broken_archived_plus_pitr_off_stacks   (covers multi-W2 +N suffix)
GIVEN: a DynamoDB table with TableStatus = ARCHIVED AND PITR = DISABLED
WHEN:  Fetcher runs (Status="archived: kms key lost"), then enricher runs
THEN:
  - Enricher FieldUpdates[id]["status"] == "archived: kms key lost (+1)"
  - Enricher Findings[id].Severity == "~"  (PITR itself is still ~)
  - After merge: Resource.Fields["status"] == "archived: kms key lost (+1)"
  - list row S2 = red (Broken — severity precedence wins)
  - list row S4 = "archived: kms key lost (+1)"
  - list row S3 = no glyph (non-green)
  - Detail view Attention section contains BOTH "Archived: kms key lost"
    AND "PITR off" verbatim (capitalized-first per capitalizeFirst) — the
    Wave-2 finding is not swallowed by the Wave-1-style phrase (covers U7c).
  - S1 menu badge does NOT bump (~ finding only)

TEST: fetcher_populates_resource_issues     (covers U7f adapted for Wave-2-only)
GIVEN: every §3.2 TableStatus fixture (Healthy, each transitional, each broken)
WHEN:  FetchDynamoDBTablesPage runs
THEN:
  - Healthy          → Issues == []   (nil or empty)
  - CREATING         → Issues == ["creating"]
  - UPDATING         → Issues == ["updating"]
  - DELETING         → Issues == ["deleting"]
  - ARCHIVING        → Issues == ["archiving"]
  - INACCESSIBLE_EC* → Issues == ["kms key inaccessible"]
  - ARCHIVED         → Issues == ["archived: kms key lost"]

TEST: anti_throttle_not_surfaced            (§3.3 Wave 3 OUT OF SCOPE)
GIVEN: a fixture that includes CloudWatch ReadThrottleEvents > 0 metadata
WHEN:  FetchDynamoDBTablesPage + EnrichDynamoDBPITR run
THEN:  no finding, no S4 phrase, no S5 line mentions "throttle" or "5xx"

TEST: anti_systemerrors_not_surfaced        (§3.3 Wave 3 OUT OF SCOPE)
GIVEN: a fixture that includes SystemErrors > 0 metadata
THEN:  no finding, no S4 phrase about 5xx/error-rate

TEST: related_alarm_via_dimension
GIVEN: an alarm cache containing one alarm with Dimensions = [{Name:"TableName", Value:<table>}]
WHEN:  checkDdbAlarm runs on this table
THEN:  result.TargetType == "alarm"; result.Count == 1; the alarm's ID is in result.IDs

TEST: related_backup_via_plan_selection
GIVEN: a backup cache containing one plan whose Fields["resources"] CSV
       includes this table's ARN (arn:aws:dynamodb:<region>:<acct>:table/<name>)
WHEN:  checkDdbBackup runs
THEN:  result.TargetType == "backup"; result.Count == 1; the plan's ID is in result.IDs
       — must NOT call ListRecoveryPointsByResource (per §2 backup discovery note)

TEST: related_kinesis_via_describe_kinesis_streaming_destination
GIVEN: DynamoDBDescribeKinesisStreamingDestinationAPI that returns one
       active destination with StreamArn ending in /<stream-name>
WHEN:  checkDdbKinesis runs
THEN:  result.TargetType == "kinesis"; result.Count == 1; result.IDs contains <stream-name>

TEST: related_kms_via_sse_description
GIVEN: TableDescription.SSEDescription.KMSMasterKeyArn =
       "arn:aws:kms:us-east-1:123456789012:key/abcd-1234-…"
WHEN:  checkDdbKMS runs
THEN:  result.Count == 1; result.IDs == ["abcd-1234-…"]

TEST: related_kms_absent_when_aws_owned
GIVEN: TableDescription.SSEDescription == nil   (AWS-owned key)
WHEN:  checkDdbKMS runs
THEN:  result.Count == 0

TEST: related_lambda_via_event_source_mappings
GIVEN: TableDescription.LatestStreamArn set, Lambda ListEventSourceMappings
       returns one mapping with FunctionArn = "arn:...:function:<fn>"
WHEN:  checkDdbLambda runs
THEN:  result.Count == 1; result.IDs == ["<fn>"]

TEST: related_lambda_zero_when_no_stream
GIVEN: TableDescription.LatestStreamArn == nil
WHEN:  checkDdbLambda runs
THEN:  result.Count == 0 (not -1 — streams-disabled is not a failure)
       — no ListEventSourceMappings call is made

TEST: related_logs_via_prefix_match
GIVEN: a logs cache containing a group named "/aws/dynamodb/tables/<name>/insights/default"
WHEN:  checkDdbLogs runs
THEN:  result.Count == 1
       AND (guard) a log group named "/aws/lambda/<name>" does NOT match
       — matching is a prefix match on "/aws/dynamodb/tables/<name>/", not substring.

TEST: related_vpce_via_service_name_match
GIVEN: a vpce cache containing one entry with
       service_name == "com.amazonaws.us-east-1.dynamodb"
       AND vpc_endpoint_type == "Gateway";
       plus one entry with service_name == "com.amazonaws.us-east-1.s3" (decoy)
WHEN:  checkDdbVPCE runs with table region = us-east-1
THEN:  result.Count == 1; the DDB endpoint's ID is in result.IDs; s3 decoy is excluded

TEST: related_ct_events_universal_pivot
GIVEN: ddb is registered in RelatedDefs
WHEN:  the related panel is populated for a ddb resource
THEN:  ct-events appears in the panel (universal pivot); no registration drift
```

## 2. Fixture list (plain language)

Fixtures live in `internal/demo/fixtures/ddb.go` (single source, retiring the current `dynamodb.go`). Every non-adversarial entry below is reachable from both `./a9s --demo` and the unit tests. Sibling fixture files are updated so every §2 pivot resolves non-zero on the nominated graph-root.

### 2.1 Per-signal fixtures

- **`orders-prod`** *(graph-root, Healthy baseline + every pivot non-zero)*
  - `TableName = "orders-prod"`.
  - `TableArn = "arn:aws:dynamodb:us-east-1:123456789012:table/orders-prod"`.
  - `TableStatus = ACTIVE`.
  - `ItemCount = 12_345_678`, `TableSizeBytes = 4_294_967_296` (4 GiB).
  - `BillingModeSummary.BillingMode = PAY_PER_REQUEST`.
  - `SSEDescription.SSEType = KMS`, `SSEDescription.KMSMasterKeyArn = "arn:aws:kms:us-east-1:123456789012:key/orders-prod-cmk-0001"`.
  - `LatestStreamArn = "arn:aws:dynamodb:us-east-1:123456789012:table/orders-prod/stream/2026-01-01T00:00:00.000"`.
  - `StreamSpecification.StreamEnabled = true`, `StreamViewType = NEW_AND_OLD_IMAGES`.
  - `DeletionProtectionEnabled = true`, `ProvisionedThroughput = nil` (on-demand).
  - PITR: ENABLED (no finding).
  - **Sibling pivots expected to resolve ≥ 1 against this fixture**:
    - `alarm`: one CloudWatch alarm `orders-prod-throttle` with `Dimensions=[{Name:"TableName", Value:"orders-prod"}]`.
    - `backup`: one backup plan `acme-weekly-full-backup` whose `resources` CSV contains `arn:aws:dynamodb:us-east-1:123456789012:table/orders-prod`.
    - `kinesis`: `DescribeKinesisStreamingDestination(orders-prod)` → `KinesisDataStreamDestinations=[{StreamArn:"arn:aws:kinesis:us-east-1:123456789012:stream/orders-prod-cdc"}]`; the `kinesis` list includes `orders-prod-cdc`.
    - `kms`: the `kms` list contains a key with ID `orders-prod-cmk-0001`.
    - `lambda`: `ListEventSourceMappings(EventSourceArn=<orders-prod stream>)` → one mapping `FunctionArn = "arn:aws:lambda:us-east-1:123456789012:function:orders-projector"`; the `lambda` list contains `orders-projector`.
    - `logs`: the `logs` list contains `/aws/dynamodb/tables/orders-prod/insights/default`.
    - `vpce`: the `vpce` list contains one gateway endpoint with `service_name = "com.amazonaws.us-east-1.dynamodb"`, `vpc_endpoint_type = "Gateway"`.
    - `ct-events`: universal — harness drains a ct-events fixture where the `ResourceName`/`ResourceType` filter matches `orders-prod`.

- **`sessions-creating`** — `TableStatus = CREATING`. Everything else minimal.

- **`sessions-updating`** — `TableStatus = UPDATING`. Minimal.

- **`analytics-deleting`** — `TableStatus = DELETING`. Minimal.

- **`legacy-archiving`** — `TableStatus = ARCHIVING`, `ArchivalSummary` not yet finalized.

- **`legacy-kms-lost`** — `TableStatus = INACCESSIBLE_ENCRYPTION_CREDENTIALS`. SSEDescription points at a now-deleted CMK (`arn:aws:kms:us-east-1:123456789012:key/legacy-prod-cmk-deleted`).

- **`legacy-archived`** — `TableStatus = ARCHIVED`. `ArchivalSummary.ArchivalReason = "INACCESSIBLE_ENCRYPTION_CREDENTIALS"`. `ArchivalSummary.ArchivalDateTime = 2024-11-02T03:15:00Z`. `ArchivalSummary.ArchivalBackupArn` set. **ALSO** PITR = DISABLED on this table → this fixture exercises the multi-W2 stacking (+1 suffix) path and the U7c detail-view-shows-both case.

- **`audit-pitr-off`** — `TableStatus = ACTIVE`, PITR = DISABLED, billing PROVISIONED. Covers the Healthy-with-~ case and U3/U11.

### 2.2 Sibling fixture edits (same PR, needed so `orders-prod` has non-zero pivots)

- `internal/demo/fixtures/alarm.go` — add `orders-prod-throttle` alarm with TableName dimension.
- `internal/demo/fixtures/backup.go` — add `acme-weekly-full-backup` plan whose selection resources CSV includes the orders-prod table ARN.
- `internal/demo/fixtures/kinesis.go` — add `orders-prod-cdc` stream.
- `internal/demo/fixtures/kms.go` — add `orders-prod-cmk-0001` key.
- `internal/demo/fixtures/lambda.go` — add `orders-projector` function and its event-source-mapping in the typed fake so `ListEventSourceMappings(orders-prod stream)` returns it.
- `internal/demo/fixtures/logs.go` — add `/aws/dynamodb/tables/orders-prod/insights/default` log group.
- `internal/demo/fixtures/vpce.go` — add a gateway endpoint with `service_name = "com.amazonaws.us-east-1.dynamodb"`.
- `internal/demo/fixtures/cloudtrail.go` (or equivalent ct-events fixture source) — ensure at least one event with `ResourceName = "orders-prod"`.

The `DescribeKinesisStreamingDestination` and `ListEventSourceMappings` fakes must be wired to return the expected payloads when called with the `orders-prod` table name / stream ARN. That wiring lives in `internal/demo/fakes/dynamodb.go` and `internal/demo/fakes/lambda.go`; the fixture-creation skill lists exact shapes.

### 2.3 Fixtures explicitly skipped (documented per skill rules)

- **U7a (multi Wave-1 suffix)** — skipped: §3.1 has zero Wave-1 signals, so no Wave-1 phrase can coexist.
- **U7b (Wave-1 + Wave-2 suffix bump)** — skipped: same reason; the multi-W2 stack in `legacy-archived` + PITR disabled is the spiritual equivalent and is covered by `broken_archived_plus_pitr_off_stacks`.
- **U7d (! beats ~ precedence)** — skipped: spec has zero `!`-severity Wave-2 signals (PITR is `~`). Recorded with reason per skill rules.
- **U7e / U7f for Wave-1 phrases** — skipped: no Wave-1 phrases to enumerate. U7f-adapted (Wave-2 phrases populated into `Resource.Issues` by the fetcher for non-Healthy TableStatus) IS tested in `fetcher_populates_resource_issues`.

### 2.4 Adversarial cases (stay inline in test files, never in the fixture file)

- `TableDescription == nil` after DescribeTable (skip table, do not crash).
- `ArchivalSummary == nil` when TableStatus == ARCHIVED (fallback to stock phrase `archived: kms key lost`).
- `DescribeContinuousBackups` errors out (mark truncated, proceed).
- `SSEDescription.KMSMasterKeyArn` with no `/` (malformed; return Count:0 not Count:-1).

## 3. Contract-surface gap analysis (read-only pass, phase 5)

| File | What the spec demands | What exists today | Delta for coder |
|---|---|---|---|
| `internal/aws/ddb.go` (fetcher) | Status carries §4 phrase (lowercase). `Resource.Issues` populated in precedence order. DescribeTable drives the mapping. | Status is the raw AWS enum (`ACTIVE`, `ARCHIVED`, …). `Resource.Issues` never set. | Introduce a `mapDDBTableStatusPhrase(status, archivalSummary) (phrase string, issues []string)` helper; set `Resource.Status` + `Resource.Issues` from it. Do not include Wave-2 PITR phrase here — that's the enricher's job. |
| `internal/aws/ddb_interfaces.go` | Narrow interfaces for ListTables, DescribeTable, DescribeContinuousBackups, DescribeKinesisStreamingDestination. | All four present. | None. |
| `internal/aws/ddb_related.go` — `checkDdbBackup` | Reverse-scan the already-loaded `backup` cache for plans whose selection resources include this table's ARN. `ListRecoveryPointsByResource` explicitly out-of-scope. | Calls `ListRecoveryPointsByResource(ResourceArn=<table>)` directly. Scope violation against spec §2. | Replace with cache reverse-scan: iterate `backup` list entries, split `Fields["resources"]` by `,`, match this table's ARN, collect plan IDs. |
| `internal/aws/ddb_related.go` — `checkDdbKMS` | Read `SSEDescription.KMSMasterKeyArn`; return key ID. | Correct. | None. |
| `internal/aws/ddb_related.go` — `checkDdbAlarm` | Scan alarm cache for Dimensions[].Name=="TableName" match. | Correct. | None. |
| `internal/aws/ddb_related.go` — `checkDdbLambda` | `ListEventSourceMappings(EventSourceArn=<LatestStreamArn>)`; resolve FunctionArn → name. Streams-disabled → Count:0, not -1. | Correct, but the no-stream path returns Count:0 as required; verify RetryOnThrottle wrap and -1 only for true API failures. | None required; accept as-is. |
| `internal/aws/ddb_related.go` — `checkDdbKinesis` | `DescribeKinesisStreamingDestination(TableName)`; resolve StreamArn → name. | Correct. | None. |
| `internal/aws/ddb_related_extra.go` — `checkDdbLogs` | Prefix match on `/aws/dynamodb/tables/<name>/`. | `strings.Contains(logRes.ID, name)` — substring match. | Switch to `strings.HasPrefix(logRes.ID, "/aws/dynamodb/tables/"+name+"/")` |
| `internal/aws/ddb_related_extra.go` — `checkDdbVPCE` | Filter by `service_name == "com.amazonaws.<region>.dynamodb"` AND `vpc_endpoint_type == "Gateway"`. | `strings.Contains(service_name, ".dynamodb")`; no endpoint-type check. | Replace with exact service-name match (suffixed by `.dynamodb`) + `vpc_endpoint_type == "Gateway"` guard. |
| `internal/aws/ddb_issue_enrichment.go` | Summary = `"PITR off"` (spec §4). Bump `(+N)` suffix when Status is non-empty; set `FieldUpdates[id]["status"]` accordingly. Key findings/updates by resource ID. | Summary = `"PITR disabled"`. FieldUpdates only sets `pitr_enabled`, never `status`. Keyed by `name`. | (a) rename Summary to `"PITR off"`; (b) compute new status via `resource.BumpFindingSuffix(existing)` when existing non-empty, else `"PITR off"`; (c) populate `FieldUpdates[id]["status"]` with that value; (d) keep `pitr_enabled` for the view-column fallback if column is retained (we're removing the column, so this can go too); (e) verify keying matches `r.ID` (ddb's ID==Name, so it's OK — switch to `r.ID` to be explicit). |
| `internal/config/defaults_databases.go` — `"ddb"` block | One Status column keyed by `status` (so fetcher's §4 phrase renders). No "PITR" jargon column. Identity columns (Table Name, Items, Size, Billing) retained. | Status column uses `Path: "TableStatus"` (shows raw enum). Extra `PITR` column reads `pitr_enabled` — jargon column per universal UI rules. | (a) change Status column from `Path: "TableStatus"` → `Key: "status"`; (b) delete the PITR column entirely. |
| `.a9s/views/ddb.yaml` | Generated from defaults_databases.go. | Mirrors current defaults (PITR column present, Status uses TableStatus path). | Regenerate via `go run ./cmd/viewsgen/` after defaults are updated. |
| `internal/resource/types_databases.go` — ddb Color func | Switch on fetcher's §4 phrases (lowercase). Must strip `(+N)` first. `""` → Healthy, transitional phrases → Warning, `kms key inaccessible` / `archived: kms key lost` → Broken, `PITR off` → Healthy (Wave-2 ~ on green row). | Switches on raw enum. Will silently break once fetcher emits phrases. | Rewrite to match the §4 phrases, strip suffix via `StripFindingSuffix`, and treat `PITR off` as Healthy (not Warning). |
| `internal/aws/ddb_detail_enrichment.go` | Only if spec §2 demands detail fields beyond the list shape. Spec §2 doesn't. | File does not exist. | None — skip creation. |

### 3.1 Test file replacements

- `tests/unit/aws_dynamodb_test.go` — legacy fetcher test against raw-enum Status; must be deleted (6b rewrites from pseudocode). Renamed target: `aws_ddb_test.go`.
- `tests/unit/aws_ddb_related_test.go` — exists; must be rewritten to assert new backup/logs/vpce semantics and to drop ListRecoveryPointsByResource expectations.
- `tests/unit/aws_ddb_enricher_test.go` — exists; must be rewritten as `aws_ddb_issue_enrichment_test.go` asserting Summary = "PITR off", suffix bumping, Row/Summary separation.
- `tests/unit/qa_rds_ddb_color_fieldkey_test.go` — shared file across rds and ddb; must stay but have its ddb assertions updated to the phrase-based Color func. Handled by QA under phase 6b.

### 3.2 Pre-phase-6a cleanup (runner executes)

Delete the stale test files that phase 6b will rewrite, so the legacy Test* symbols don't clash with the new ones:

```bash
rm -f tests/unit/aws_dynamodb_test.go \
      tests/unit/aws_ddb_related_test.go \
      tests/unit/aws_ddb_enricher_test.go
```

(`qa_rds_ddb_color_fieldkey_test.go` is NOT deleted — it's shared with rds. QA edits its ddb section in-place.)

Also delete the legacy fixture file after the new one ships:

```bash
rm -f internal/demo/fixtures/dynamodb.go
```

## 4. Coverage matrix (per universal UI rules)

| ID | Covered by | Notes |
|---|---|---|
| U1 Healthy blank S4 | `healthy_active_blank` + phase-8 assertion | `orders-prod`, `audit-pitr-off` (pre-enrichment) |
| U2 Warning/Broken §4 phrase | `warn_creating_phrase`, `warn_updating_phrase`, `warn_deleting_phrase`, `warn_archiving_phrase`, `broken_kms_inaccessible`, `broken_archived` | all §3.2 non-Healthy signals |
| U3 `~` glyph on Healthy+~ finding | `healthy_pitr_off_glyph_and_phrase` | `audit-pitr-off` |
| U4 `!` glyph | SKIPPED | spec has zero `!` signals |
| U5 No glyph on non-green | multi-W2 test + warn_* tests | `legacy-archived`, `sessions-creating`, etc. |
| U6 S1 badge counts `!`-instances | phase-8 assertion | expected N=0 (no `!` signals) |
| U7a multi-W1 suffix | SKIPPED | no Wave-1 signals |
| U7b W1+W2 stack | SKIPPED | no Wave-1 signals; multi-W2 equivalent covered below |
| U7c S5 lists every Wave-2 finding | `broken_archived_plus_pitr_off_stacks` | `legacy-archived` carries both phrases |
| U7d `!` beats `~` | SKIPPED | no `!` signals |
| U7e S5 every Wave-1 phrase | SKIPPED | no Wave-1 phrases |
| U7f fetcher populates Resource.Issues | `fetcher_populates_resource_issues` | per-fixture deep-equal |
| U8 Broken > Warning > ~ | implicit in `broken_archived_plus_pitr_off_stacks` | row stays red |
| U9 Related pivot counts | per-checker tests + phase-8 | `orders-prod` resolves every §2 `count shown: yes` pivot ≥ 1 |
| U10 No jargon columns | phase-8 `ExpectViewNotContains` | verifies PITR column is gone |
| U11 Summary ≠ Rows content | `enricher_summary_not_rows` | unit assertion |

## 5. Phase 8 scenario test outline

`tests/integration/scenario_ddb_visual_test.go` — `TestScenario_DDBVisual`:

1. `fullIntegrationNewDemoScenario(t)` → `OpenList("ddb")`.
2. `ExpectViewNotContains("PITR", "pitr", "CIS", "Flags", "Policy", "Issues")` — universal rule 10 + ddb-specific PITR-column gone.
3. For each fixture ID: `ExpectRowStatusBlank` / `ExpectRowStatusEquals` per the table below.
4. `ExpectRowNamePrefix("audit-pitr-off", "~ ")`; for every non-green row, `ExpectRowNoGlyphPrefix`.
5. `ExpectMenuIssueCount("ddb", 0)` — no `!` signals exist.
6. `OpenDetailResource("ddb", "orders-prod")` → for each pivot in §2, `ExpectRelatedRowCountAtLeast(<display name>, 1)`.
7. `OpenDetailResource("ddb", "legacy-archived")` → `ExpectViewContains("Archived: kms key lost")` AND `ExpectViewContains("PITR off")` (U7c).
8. `t.Log("\n" + scenario.currentView())` right before the multi-finding ExpectViewContains asserts — 8.4 rendered-frame sanity.

Expected Status per fixture:

| Fixture | Expected rendered S4 |
|---|---|
| `orders-prod` | (blank) |
| `sessions-creating` | `creating` |
| `sessions-updating` | `updating` |
| `analytics-deleting` | `deleting` |
| `legacy-archiving` | `archiving` |
| `legacy-kms-lost` | `kms key inaccessible` |
| `legacy-archived` | `archived: kms key lost (+1)` |
| `audit-pitr-off` | `PITR off` |
