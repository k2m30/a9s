# Tasks: Configurable Views

**Input**: Design documents from `/specs/002-configurable-views/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Included per constitution (Principle I: TDD is NON-NEGOTIABLE).

**Organization**: Tasks grouped by user story. US1+US2 combined (config loading is inseparable from list view configurability).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup

**Purpose**: Add dependency, create package structure

- [x] T001 Add `gopkg.in/yaml.v3` dependency via `go get gopkg.in/yaml.v3`
- [x] T002 [P] Create directory structure: `internal/config/`, `internal/fieldpath/`, `cmd/refgen/`
- [x] T003 [P] Create test fixture files: `tests/testdata/views_valid.yaml`, `tests/testdata/views_invalid.yaml`, `tests/testdata/views_partial.yaml`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Reflection engine and config types that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

### Tests for Foundational

> **Write these tests FIRST, ensure they FAIL before implementation**

- [x] T004 [P] Test dot-path extraction on simple structs (string, *string, int, nested struct) in `tests/unit/fieldpath_extract_test.go`
- [x] T005 [P] Test auto-formatting: time.Time→readable date, bool→Yes/No, *string→deref, nil→empty, named string types→string in `tests/unit/fieldpath_format_test.go`
- [x] T006 [P] Test dot-path extraction with pointer fields, nil pointers, missing fields returning empty in `tests/unit/fieldpath_extract_test.go`
- [x] T007 [P] Test YAML subtree extraction: scalar returns value, nested struct returns YAML-marshaled string, slice returns YAML-marshaled string in `tests/unit/fieldpath_extract_test.go`

### Implementation for Foundational

- [x] T008 [P] Implement `ExtractValue(obj interface{}, dotPath string) (reflect.Value, error)` — resolves dot-notation path via JSON tags, dereferences pointers, in `internal/fieldpath/extract.go`
- [x] T009 [P] Implement `FormatValue(val reflect.Value) string` — auto-formats by Go type per data-model type mapping in `internal/fieldpath/format.go`
- [x] T010 Implement `ExtractScalar(obj interface{}, dotPath string) string` — combines ExtractValue + FormatValue for list view use, returns empty string on error, in `internal/fieldpath/extract.go`
- [x] T011 Implement `ExtractSubtree(obj interface{}, dotPath string) string` — for detail view: scalars formatted, nested objects/arrays YAML-marshaled, in `internal/fieldpath/extract.go`
- [x] T012 Define config types: `ViewsConfig`, `ViewDef`, `ListColumnDef` structs with YAML tags in `internal/config/config.go`
- [x] T013 Add `RawStruct interface{}` field to `Resource` struct in `internal/resource/resource.go`
- [x] T014 Update all 7 AWS fetchers to store raw SDK struct in `Resource.RawStruct`: `internal/aws/ec2.go`, `internal/aws/s3.go`, `internal/aws/rds.go`, `internal/aws/redis.go`, `internal/aws/docdb.go`, `internal/aws/eks.go`, `internal/aws/secrets.go`

**Checkpoint**: Reflection engine works on any Go struct. Config types defined. Resources carry raw SDK structs. All existing tests still pass.

---

## Phase 3: User Story 1 + User Story 2 — Configurable List View with Lookup Chain (Priority: P1) MVP

**Goal**: List view columns loaded from `views.yaml` found via the 3-location lookup chain. Falls back to built-in defaults when no config exists or for unconfigured resource types.

**Independent Test**: Place `views.yaml` in current directory with custom EC2 columns → launch a9s → verify only those columns appear with specified widths.

### Tests for US1 + US2

> **Write these tests FIRST, ensure they FAIL before implementation**

- [x] T015 [P] [US1] Test YAML parsing: valid config returns correct ViewDef with list columns per resource type in `tests/unit/config_test.go`
- [x] T016 [P] [US2] Test lookup chain: finds `./views.yaml` first, then `$A9S_CONFIG_FOLDER/views.yaml`, then `~/.a9s/views.yaml` using temp dirs in `tests/unit/config_test.go`
- [x] T017 [P] [US1] Test fallback: no config file returns built-in defaults in `tests/unit/config_test.go`
- [x] T018 [P] [US1] Test partial config: config defines S3 only → S3 uses config, EC2 uses defaults in `tests/unit/config_test.go`
- [x] T019 [P] [US1] Test invalid YAML: returns error + falls back to defaults in `tests/unit/config_test.go`
- [x] T020 [P] [US1] Test list view rendering with config-driven columns: columns from config appear with correct widths, values extracted via reflection in `tests/unit/views_config_test.go`
- [x] T020b [P] [US1] Test sorting by config-driven column: sort by a reflected scalar column (string, date, numeric) produces correct order in `tests/unit/views_config_test.go`

### Implementation for US1 + US2

- [x] T021 [US1] Implement built-in default view definitions matching current hardcoded values for all 7 resource types in `internal/config/defaults.go`
- [x] T022 [US2] Implement `Load() (*ViewsConfig, error)` — lookup chain (`./views.yaml` → `$A9S_CONFIG_FOLDER/views.yaml` → `~/.a9s/views.yaml`), parse YAML, return config or error in `internal/config/config.go`
- [x] T023 [US1] Implement `GetViewDef(cfg *ViewsConfig, shortName string) ViewDef` — returns config view if present, falls back to defaults, handles partial config in `internal/config/config.go`
- [x] T024 [US1] Implement error handling: invalid YAML logs error message with file path and parse error, returns defaults in `internal/config/config.go`
- [x] T025 [US1] Update `internal/app/app.go` renderResourceList to use config-driven columns with fieldpath.ExtractScalar for value extraction
- [x] T026 [US1] Update `internal/app/app.go` to call `config.Load()` at startup and pass config to views
- [x] T026b [US1] Update sort logic in `internal/app/app.go` to sort by reflection-extracted values instead of Fields map lookup
- [x] T027 [US1] Create default `views.yaml` file at project root pre-populated with current hardcoded column definitions for all resource types

**Checkpoint**: List view columns driven by `views.yaml`. Lookup chain works across 3 locations. Falls back to defaults when no config. All existing tests pass.

---

## Phase 4: User Story 1b — Configurable Detail View (Priority: P1)

**Goal**: Detail view shows only configured fields in configured order. Scalars render as key:value, nested objects/arrays as indented YAML subtrees.

**Independent Test**: Add `detail` section to `views.yaml` for EC2 with 5 paths including `securityGroups` → open EC2 detail → verify only those 5 fields shown in order, with security groups rendered as YAML.

### Tests for US1b

> **Write these tests FIRST, ensure they FAIL before implementation**

- [x] T028 [P] [US1b] Test detail view renders configured fields in order (not alphabetical) in `tests/unit/detail_config_test.go`
- [x] T029 [P] [US1b] Test detail view renders scalar path as `key : value` in `tests/unit/detail_config_test.go`
- [x] T030 [P] [US1b] Test detail view renders nested object/array path as indented YAML subtree in `tests/unit/detail_config_test.go`
- [x] T031 [P] [US1b] Test detail view falls back to built-in defaults when no detail section configured in `tests/unit/detail_config_test.go`

### Implementation for US1b

- [x] T032 [US1b] Update `internal/views/detail.go` to accept `ViewDef.Detail` path list and render fields in configured order using `fieldpath.ExtractSubtree()`
- [x] T033 [US1b] Implement YAML subtree rendering in detail view: detect scalar vs nested via reflection, format scalars as `key : value`, marshal nested to indented YAML in `internal/views/detail.go`
- [x] T034 [US1b] Update `internal/app/app.go` detail view initialization to pass config detail paths when available, fall back to current behavior (all fields alphabetical) when not configured
- [x] T035 [US1b] Add default detail field paths to `internal/config/defaults.go` for all 7 resource types matching current hardcoded DetailData fields

**Checkpoint**: Detail view driven by config. Scalar and nested rendering works. Falls back to current behavior when unconfigured. All tests pass.

---

## Phase 5: User Story 3 — Reference File Generator (Priority: P2)

**Goal**: Development-time tool that reflects on AWS SDK structs to enumerate all available field paths. Output is `views_reference.yaml`.

**Independent Test**: Run `go run ./cmd/refgen` → verify output contains known paths like `instanceId`, `state.name` for EC2 and `name`, `creationDate` for S3.

### Tests for US3

> **Write these tests FIRST, ensure they FAIL before implementation**

- [x] T036 [P] [US3] Test path enumeration on a simple test struct: returns flat paths for scalars, dot-paths for nested, `[]` suffix for slices in `tests/unit/fieldpath_enumerate_test.go`
- [x] T037 [P] [US3] Test path enumeration handles pointer types, time.Time (leaf), and named string types (leaf) in `tests/unit/fieldpath_enumerate_test.go`

### Implementation for US3

- [x] T038 [US3] Implement `EnumeratePaths(t reflect.Type, prefix string) []string` — recursive struct walker using JSON tags, handles pointers, slices, nested structs in `internal/fieldpath/enumerate.go`
- [x] T039 [US3] Create `cmd/refgen/main.go` — imports all AWS SDK response types, calls `EnumeratePaths` for each, outputs YAML with resource type as key and paths as list
- [x] T040 [US3] Run reference generator and commit output as `views_reference.yaml` at project root

**Checkpoint**: `views_reference.yaml` generated and committed. Paths from reference file work in `views.yaml`.

---

## Phase 6: User Story 4 — S3 Objects View Configuration (Priority: P2)

**Goal**: S3 object/prefix browsing view (inside a bucket) also configurable via `views.yaml` under `s3_objects` key.

**Independent Test**: Add `s3_objects` section to `views.yaml` with 2 columns → browse inside bucket → verify only those 2 columns shown.

### Tests for US4

- [x] T041 [P] [US4] Test config loading recognizes `s3_objects` as a valid view key and returns correct columns in `tests/unit/config_test.go`
- [x] T042 [P] [US4] Test S3 object list view uses config-driven columns when `s3_objects` section present in `tests/unit/config_test.go`

### Implementation for US4

- [x] T043 [US4] Add `s3_objects` default view definition to `internal/config/defaults.go` matching current `S3ObjectColumns()` values
- [x] T044 [US4] Update S3 object browsing view in `internal/app/app.go` to use config-driven columns for `s3_objects` view key
- [x] T045 [US4] Update `internal/aws/s3.go` `FetchS3Objects` to store raw SDK struct in `Resource.RawStruct` for objects and common prefixes
- [x] T046 [US4] Add `s3_objects` section to default `views.yaml` at project root

**Checkpoint**: All 8 view types (7 resource types + S3 objects) configurable. Full feature complete.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Cleanup, edge cases, documentation

- [ ] T047 Remove old hardcoded column definitions from `internal/resource/types.go` once all views use config-driven rendering (DEFERRED: legacy path still used by tests with nil RawStruct)
- [x] T048 [P] Verify all edge cases from spec: invalid YAML error message, unknown view names ignored, width=0 is flexible, missing env var dir skipped, array path in list view shows empty
- [x] T049 [P] Run `go vet ./...` — zero warnings (golangci-lint not installed)
- [x] T050 [P] Run `go test ./tests/unit/ -count=1` — all tests pass
- [x] T051 Run quickstart.md validation: views.yaml + views_reference.yaml shipped, config loading works, fieldpath extraction works

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup — BLOCKS all user stories
- **US1+US2 (Phase 3)**: Depends on Foundational
- **US1b (Phase 4)**: Depends on Foundational (can run in parallel with Phase 3)
- **US3 (Phase 5)**: Depends on Foundational (reflection enumerate)
- **US4 (Phase 6)**: Depends on Phase 3 (list view config infrastructure)
- **Polish (Phase 7)**: Depends on all user stories complete

### User Story Dependencies

- **US1+US2 (P1)**: Depends on Foundational only — MVP story
- **US1b (P1)**: Depends on Foundational only — can parallelize with US1+US2
- **US3 (P2)**: Depends on Foundational only — can parallelize with US1/US1b
- **US4 (P2)**: Depends on US1+US2 (reuses list view config infrastructure)

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Config/types before rendering logic
- Core implementation before integration with app.go

### Parallel Opportunities

- T002, T003 in Setup (different directories)
- T004, T005, T006, T007 in Foundational tests (different test functions)
- T008, T009 in Foundational impl (different files)
- T015–T020 all test tasks (different test functions)
- T028–T031 all detail test tasks
- T036, T037 enumerate test tasks
- Phases 3, 4, 5 can run in parallel after Foundational

---

## Parallel Example: Foundational Phase

```bash
# Launch all foundational tests together:
Task: T004 "Test dot-path extraction on simple structs"
Task: T005 "Test auto-formatting"
Task: T006 "Test pointer/nil/missing field extraction"
Task: T007 "Test YAML subtree extraction"

# After tests fail, launch parallel implementations:
Task: T008 "Implement ExtractValue in internal/fieldpath/extract.go"
Task: T009 "Implement FormatValue in internal/fieldpath/format.go"
```

---

## Implementation Strategy

### MVP First (US1 + US2)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: US1 + US2 (config-driven list view with lookup chain)
4. **STOP and VALIDATE**: Place custom `views.yaml` → verify list columns change
5. Deploy/demo if ready

### Incremental Delivery

1. Setup + Foundational → Reflection engine ready
2. US1+US2 → Configurable list view (MVP!)
3. US1b → Configurable detail view
4. US3 → Reference file for field discovery
5. US4 → S3 objects view
6. Polish → Cleanup and edge cases

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story
- Constitution mandates TDD: write failing tests before implementation
- Commit after each task or logical group
- Stop at any checkpoint to validate independently
- Existing 676 unit tests must continue passing throughout
