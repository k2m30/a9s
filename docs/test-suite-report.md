# a9s Test Suite Assessment Report

**Date:** 2026-03-18
**Analyzer:** Static analysis + `go test` coverage profiling
**Overall Coverage:** 88.5% of statements

---

## A. Architecture Summary

a9s is a terminal-based AWS resource browser built with Go and the Bubble Tea v2 framework. It provides a TUI (Terminal User Interface) for viewing AWS resources across seven service types: S3, EC2, RDS, ElastiCache Redis, DocumentDB, EKS, and Secrets Manager. The application follows a view-stack architecture where users navigate through main menu, resource lists, detail views, YAML dumps, help overlays, and profile/region selectors. All rendering is driven by configurable `views.yaml` field paths that extract data from real AWS SDK structs via reflection.

### Package Structure

| Package | Lines | Purpose |
|---------|-------|---------|
| `internal/tui/` (app.go) | 871 | Root model: view stack, navigation, input modes, AWS integration |
| `internal/tui/views/` | 1,464 | 8 view models: mainmenu, resourcelist, detail, yaml, reveal, profile, region, help |
| `internal/tui/layout/` | 148 | Frame rendering, header, border drawing |
| `internal/tui/styles/` | 147 | Lipgloss style definitions, status coloring |
| `internal/tui/keys/` | 82 | Key binding definitions |
| `internal/tui/messages/` | 114 | Tea message types for inter-view communication |
| `internal/aws/` | 1,259 | AWS SDK wrappers for all 7 services + profiles + regions |
| `internal/fieldpath/` | 400 | Reflection-based struct field extraction and formatting |
| `internal/config/` | 314 | YAML config parsing for view definitions |
| `internal/resource/` | 171 | Resource type definitions and column specs |
| **Total source** | **5,235** | |

---

## B. Test Suite Structure

### B.1 Key Metrics

| Metric | Value |
|--------|-------|
| Total test runs (incl. subtests) | 1,311 |
| Top-level test functions | 915 |
| Passing | 915 (100%) |
| Failing | 0 |
| Skipped | 1 (`TestQA_Profile_FrameTitle`) |
| Benchmark functions | 2 |
| Test files | 46 (43 with tests + 3 support) |
| Total test code lines | 22,562 |
| Test:source ratio | 4.3:1 |

### B.2 Test File Inventory

#### AWS Client Tests (7 files, 29 tests, 1,467 lines)
| File | Tests | Coverage Target |
|------|-------|-----------------|
| `aws_s3_test.go` | 6 | S3 bucket/object fetching, pagination, error handling |
| `aws_ec2_test.go` | 4 | EC2 instance parsing, detail data, empty/error responses |
| `aws_rds_test.go` | 3 | RDS instance parsing, error handling |
| `aws_redis_test.go` | 3 | ElastiCache cluster parsing |
| `aws_docdb_test.go` | 4 | DocumentDB cluster parsing, engine filter verification |
| `aws_eks_test.go` | 3 | EKS cluster listing via two-phase API |
| `aws_secrets_test.go` | 6 | Secrets listing, reveal, detail data, error handling |
| `aws_errors_test.go` | 6 | AWS error classification (expired, denied, throttle, etc.) |
| `aws_profile_test.go` | 9 | AWS profile file parsing, config/credentials merge |

#### Fieldpath Tests (4 files, 45 tests, 900 lines)
| File | Tests | Coverage Target |
|------|-------|-----------------|
| `fieldpath_extract_test.go` | 21 | Field extraction from structs, pointers, nil handling |
| `fieldpath_enumerate_test.go` | 8 | Path enumeration on AWS SDK types |
| `fieldpath_format_test.go` | 15 | Value formatting (timestamps, booleans, sizes) |
| `fieldpath_unexported_test.go` | 1 | Unexported field handling edge case |

#### Config Tests (1 file, 10 tests, 374 lines)
| File | Tests | Coverage Target |
|------|-------|-----------------|
| `config_test.go` | 10 | YAML config loading, parsing, defaults, column specs |

#### TUI Core Tests (5 files, 119 tests, 2,288 lines)
| File | Tests | Coverage Target |
|------|-------|-----------------|
| `tui_root_test.go` | 33 | Root model: view rendering, navigation, command mode, header, S3 drill-in |
| `tui_content_views_test.go` | 28 | All 8 view models: View(), FrameTitle(), field rendering |
| `tui_resourcelist_test.go` | 14 | Resource list: columns, filter, sort, scroll, config-driven columns |
| `tui_wiring_test.go` | 12 | Cross-cutting: clipboard copy, refresh, reveal, config loading |
| `tui_styles_test.go` | 19 | Style initialization, status coloring, NO_COLOR support |
| `layout_frame_test.go` | 26 | Frame rendering, title centering, padding, header layout |

#### QA Scenario Tests (16 files, 703 tests, 13,483 lines)
| File | Tests | Coverage Target |
|------|-------|-----------------|
| `qa_mainmenu_test.go` | 78 | Main menu: all 7 types, navigation, filter, command, help, quit, header, frame, terminal size, edge cases |
| `qa_ec2_test.go` | 76 | EC2: list columns, frame, header, status color, selection, navigation, sort, filter, command, actions, detail, YAML, cross-view navigation, FilterResources unit |
| `qa_s3_test.go` | 61 | S3: bucket list, object list, drill-in, detail, YAML, navigation, filter, sort, copy |
| `qa_rds_test.go` | 66 | RDS: list, detail, YAML, filter, sort, navigation, cross-view |
| `qa_redis_docdb_test.go` | 59 | Redis + DocumentDB: list, detail, YAML, filter, sort, cross-service switching |
| `qa_eks_secrets_test.go` | 28 | EKS + Secrets: list, detail, YAML, reveal workflow |
| `qa_detail_test.go` | 72 | Detail view across all 7 resource types with real AWS SDK structs |
| `qa_yaml_test.go` | 50 | YAML view across all 7 types: syntax coloring, scroll, wrap, copy |
| `qa_help_profile_region_test.go` | 38 | Help overlay, profile selector, region selector, flash messages, resize |
| `qa_configurable_views_test.go` | 37 | Config-driven columns and detail fields for all resource types |
| `qa_filtering_test.go` | 27 | Filter behavior across all 7 resource types |
| `qa_coverage_gaps_test.go` | 22 | Targeted tests for propagateSize, updateActiveView delegation, colorizeValue |
| `qa_fetch_test.go` | 12 | End-to-end fetch pipeline with mock HTTP for all 7 types |
| `qa_profile_update_test.go` | 30 | ProfileModel.Update: navigation, selection, filter |
| `qa_profile_switch_test.go` | 9 | Profile/region switch: data refresh, header update |
| `qa_architect_bugs_test.go` | 4 | Regression tests for specific bugs: S3 folder navigation, detail ViewDef selection |

#### Specialized Tests (5 files, 12 tests, 281 lines)
| File | Tests | Coverage Target |
|------|-------|-----------------|
| `filter_test.go` | 7 | FilterResources: case-insensitive, field match, edge cases |
| `filter_bench_test.go` | 2+benchmarks | Filter performance with large datasets |
| `s3_pagination_test.go` | 1 | S3 bucket pagination across multiple pages |
| `s3_object_pagination_test.go` | 1 | S3 object pagination with continuation tokens |
| `s3_perf_test.go` | 1+benchmark | S3 fetch performance with 500 buckets |
| `column_key_mismatch_test.go` | 1 | Config column key vs resource Fields key mismatch |
| `qa_detail_paths_test.go` | 1 | Verify detail config paths match AWS SDK struct fields |
| `qa_s3_object_detail_test.go` | 3 | S3 object detail with realistic SDK structs |

#### Support Files (3 files, 0 tests, 663 lines)
| File | Purpose |
|------|---------|
| `fixtures_test.go` | Real AWS data fixtures for S3, EC2, RDS, Redis, DocDB, EKS, Secrets |
| `mocks_test.go` | Interface mocks for all 7 AWS service APIs |
| `helpers_test.go` | stripANSI, lipglossWidth utilities |

### B.3 QA Story Coverage

| QA Doc | File | Test File | Status |
|--------|------|-----------|--------|
| `01-main-menu.md` | Main menu behavior | `qa_mainmenu_test.go` | Covered (78 tests) |
| `02-s3-views.md` | S3 bucket/object views | `qa_s3_test.go` | Covered (61 tests) |
| `03-ec2-views.md` | EC2 instance views | `qa_ec2_test.go` | Covered (76 tests) |
| `04-rds-views.md` | RDS instance views | `qa_rds_test.go` | Covered (66 tests) |
| `05-redis-docdb-views.md` | Redis + DocDB views | `qa_redis_docdb_test.go` | Covered (59 tests) |
| `06-eks-secrets-views.md` | EKS + Secrets views | `qa_eks_secrets_test.go` | Covered (28 tests) |
| `07-help-profile-region.md` | Help, profile, region | `qa_help_profile_region_test.go` | Covered (38 tests) |
| `08-detail-all-types.md` | Detail view all types | `qa_detail_test.go` | Covered (72 tests) |
| `09-yaml-all-types.md` | YAML view all types | `qa_yaml_test.go` | Covered (50 tests) |
| `10-configurable-views.md` | Config-driven columns | `qa_configurable_views_test.go` | Covered (37 tests) |
| `11-filtering.md` | Filter across all types | `qa_filtering_test.go` | Covered (27 tests) |

**Result: 11/11 QA story documents have corresponding test files.**

---

## C. Coverage Analysis

### C.1 Overall: 88.5%

### C.2 Per-Package Breakdown (estimated from function-level data)

| Package | Estimated Coverage | Notes |
|---------|-------------------|-------|
| `internal/tui/` | ~88% | `fetchProfiles` 11.1%, `fetchSecretValue` 44.4% drag it down |
| `internal/tui/views/` | ~92% | Several `Init()` methods at 0% (no-ops) |
| `internal/tui/layout/` | 100% | Fully covered |
| `internal/tui/styles/` | 100% | Fully covered |
| `internal/tui/keys/` | 100% | Fully covered |
| `internal/aws/` | ~89% | `parseCredentialsProfiles` 0%, `DefaultCredentialsPath` 0% |
| `internal/fieldpath/` | ~88% | `ToSafeValue` 70.7%, `FormatValue` 75%, `EnumeratePaths` 76.1% |
| `internal/config/` | ~91% | `UnmarshalYAML` 76.5%, `parseListColumns` 83.3% |
| `internal/resource/` | ~96% | Near-complete |

### C.3 Functions at 0% Coverage

| Function | File | Risk |
|----------|------|------|
| `DefaultCredentialsPath` | aws/profile.go:22 | Low -- simple path resolution |
| `parseCredentialsProfiles` | aws/profile.go:98 | Medium -- credentials-file profile parsing untested |
| `Help.Init()` | views/help.go:29 | None -- no-op function |
| `MainMenu.Init()` | views/mainmenu.go:38 | None -- no-op function |
| `Region.Init()` | views/region.go:37 | None -- no-op function |
| `YAML.Init()` | views/yaml.go:42 | None -- no-op function |
| `Profile.SetFilter` | views/profile.go:110 | Low -- profile filtering unused in UI |
| `Profile.applyFilter` | views/profile.go:117 | Low -- profile filtering unused in UI |

### C.4 Functions Below 50% Coverage

| Function | Coverage | Risk | Why |
|----------|----------|------|-----|
| `fetchProfiles` | 11.1% | **HIGH** | Reads filesystem, creates profile view -- nearly untested because it requires AWS config files on disk |
| `fetchSecretValue` | 44.4% | **HIGH** | Secret reveal pipeline -- only error path partially tested, success path requires real AWS clients |

### C.5 Functions at 50-80% (Notable)

| Function | Coverage | Gap |
|----------|----------|-----|
| `ToSafeValue` | 70.7% | Missing: channel, func, complex types |
| `innerSize` | 71.4% | Missing: size clamping edge cases |
| `FormatValue` | 75.0% | Missing: some value type branches |
| `EnumeratePaths` | 76.1% | Missing: some reflect.Kind branches |
| `UnmarshalYAML` | 76.5% | Missing: some YAML parsing edge cases |
| `GetDefaultRegion` | 73.9% | Missing: env var and config file region sources |
| `ResourceList.Update` | 72.3% | Missing: PageUp/PageDown, some key combos |
| `visibleWindow` | 71.4% | Missing: cursor-at-boundary scrolling |
| `buildSecretDetailData` | 71.4% | Missing: rotation rules, version stages |

---

## D. Real Value Assessment

### D.1 High-Value Tests (genuinely catch bugs)

**`qa_detail_test.go` -- EXCELLENT (9/10)**
Uses *real AWS SDK types* (`ec2types.Instance`, `rdstypes.DBInstance`, `s3types.Bucket`, etc.) with realistic field values. Tests config-driven rendering with `DefaultConfig()`. Tests nil pointer handling (nil PublicIpAddress, nil ConfigurationEndpoint). Would catch rendering regressions because assertions check for specific field names AND values in the View() output. The `realisticEC2Instance()` helper constructs an Instance with nested State, SecurityGroups array, and Tags -- this is close to what the real AWS SDK returns.

**`qa_fetch_test.go` -- EXCELLENT (9/10)**
Constructs real AWS SDK clients backed by a mock HTTP transport that returns valid XML/JSON responses matching each service's API contract. Tests the full fetch pipeline: NavigateMsg -> fetchResources -> HTTP call -> response parsing -> ResourcesLoadedMsg. Covers all 7 resource types plus error paths (nil clients, unsupported type). This is the closest thing to an integration test without touching AWS.

**`qa_architect_bugs_test.go` -- EXCELLENT (10/10)**
Regression tests for specific bugs that shipped to production. Tests that S3 folder Enter navigates into prefix (not detail), that `d` key on S3 bucket shows detail (not enters bucket), and that detail view uses the correct ViewDef for the resource type when the full config has all 8 types loaded. These tests exist because bugs were found *despite* other tests passing.

**`aws_*_test.go` (7 files) -- GOOD (7/10)**
Test AWS service wrappers with mock API clients. Verify correct parsing of multiple resources, error handling, empty responses, and specific data mapping (e.g., DocumentDB engine filter). Use real AWS SDK request/response types. Weakness: mocks only return canned single-page responses; don't test complex scenarios like instances with nested VPCs or security groups.

**`fieldpath_extract_test.go` -- GOOD (8/10)**
Tests reflection-based field extraction with nested structs, pointer chaining, nil handling, arrays, and maps. Uses both simple test structs and real AWS SDK types. Assertions check exact extracted values, not just "non-empty". Would catch bugs in the data pipeline between AWS responses and view rendering.

### D.2 Medium-Value Tests (provide safety net but limited depth)

**`qa_mainmenu_test.go`, `qa_ec2_test.go`, `qa_s3_test.go`, etc. (QA scenario files) -- MEDIUM-HIGH (6/10)**

These tests are thorough in covering *behavioral flows* (navigate, filter, sort, Esc back) and check frame titles, header content, and data presence in rendered output. However, there is a critical limitation:

**The fixture data uses `resource.Resource` with flat `Fields` maps (not RawStruct).** This means:
- Config-driven column paths that reference AWS SDK struct fields (`InstanceId`, `PrivateIpAddress`) cannot extract values from `Fields` maps. They fall back to title-matching, which bridges some gaps but misses others.
- The tests confirm the *UI chrome* works correctly (navigation, frame titles, filter counts) but don't fully test whether *real AWS data* renders correctly in list columns.
- Example: `TestQA_EC2_A1_1_ListColumns_AllSixPresent` verifies column HEADERS exist but cannot verify that data appears in the right columns because fixture resources lack RawStruct.

This is a significant false-confidence risk. The `qa_detail_test.go` file (which DOES use RawStruct with real AWS SDK types) partially compensates.

**`tui_root_test.go` -- MEDIUM (6/10)**
Tests the root model's view stack, navigation, command mode, filter mode, and header rendering. Good coverage of the happy path. Assertions check `strings.Contains` on stripped-ANSI output, which catches presence but not position. Tests cannot verify that "Instance ID" appears as a column header and not somewhere else in the output.

**`tui_resourcelist_test.go` -- MEDIUM (6/10)**
Tests resource list rendering at the view level (not through root model). Good: verifies column headers, data rows, filter narrowing, sort indicators, empty list message. Limitation: uses test data with simple Fields maps, so config-driven column extraction is not deeply tested.

### D.3 Low-Value Tests (exist but weak coverage)

**`qa_coverage_gaps_test.go` -- LOW-MEDIUM (4/10)**
Explicitly written to hit uncovered lines (propagateSize, updateActiveView delegation, colorizeValue). These tests do exercise the code paths, but assertions are mostly "did it not crash" or "did the output change." For example, `TestQA_Coverage_PropagateSize_AllViewTypes` verifies that `beforeResize != afterResize` -- a legitimate check, but it would pass even if the resize produced garbage output.

**`tui_styles_test.go` -- LOW (3/10)**
Tests that style functions return non-empty strings and that NO_COLOR mode produces unstyled output. These tests confirm the styling infrastructure works but cannot catch visual regressions (wrong color for a status, misaligned columns). They are essentially "does the style system not crash" tests.

### D.4 Assessment of Assertion Quality

| Pattern | Usage | Quality |
|---------|-------|---------|
| `strings.Contains(plain, "exact value")` | ~80% of assertions | **Good** -- checks for specific content |
| `strings.Contains(plain, "ec2(6)")` | Frame title checks | **Very good** -- exact format match |
| `output != ""` | ~5% of assertions | **Weak** -- only tests non-empty |
| `beforeOutput != afterOutput` | ~5% of assertions | **Weak** -- tests change but not correctness |
| Exact equality (`title == "ec2(5)"`) | ~5% of assertions | **Excellent** -- precise |
| `len(result) == N` | Filter tests | **Good** -- checks exact count |
| Type assertions (`msg.(messages.X)`) | Cmd result tests | **Good** -- verifies message flow |

---

## E. Gaps and Weaknesses

### E.1 Critical Untested Areas

1. **`fetchProfiles` (11.1% coverage)**: The profile loading pipeline that reads `~/.aws/config` and `~/.aws/credentials` is nearly untested in the TUI context. If profile loading fails silently or returns unexpected data, users would see empty profile selectors or crashes.

2. **`fetchSecretValue` (44.4% coverage)**: The secret reveal success path through the TUI is only partially tested. The test in `tui_wiring_test.go` tests with nil clients (error path). The happy path (client returns a secret value -> SecretRevealedMsg -> push reveal view) is tested at the message level but not through the actual fetch pipeline.

3. **`parseCredentialsProfiles` (0% coverage)**: The function that parses `~/.aws/credentials` for profiles is completely untested. If users have profiles only in credentials (not config), they would not appear in the profile selector.

4. **Real AWS struct data in list views**: QA scenario tests for list views (qa_ec2_test.go, qa_s3_test.go, etc.) use `resource.Resource` with flat `Fields` maps rather than RawStruct with real SDK types. This means config-driven column extraction (which uses reflection on RawStruct) is NOT tested in list view context for most resource types. The detail view tests (`qa_detail_test.go`) compensate partially, but there is a gap in testing whether list columns correctly extract from RawStruct.

5. **ResourceList.Update PageUp/PageDown (72.3%)**: The resource list update handler for Page Up/Page Down keys is under-tested. These are important for navigating long resource lists.

### E.2 Tests That Give False Confidence

1. **QA scenario tests with Fields-only fixtures**: As noted above, tests like `TestQA_EC2_A1_1_ListColumns_AllSixPresent` verify column headers exist but cannot verify data appears correctly because the fixture data lacks RawStruct. A bug in config-driven column extraction would NOT be caught by these tests. **This is exactly the class of bug that the `qa_architect_bugs_test.go` TestBug_Detail_UsesCorrectViewDefForResourceType was written to catch** -- a bug where detail views used the wrong ViewDef shipped despite tests passing.

2. **Status coloring tests**: Tests like `TestQA_EC2_A4_StatusColoring_RunningRowHasANSI` verify that running rows have *some* ANSI codes, but don't verify the specific color. A row could be styled red instead of green and these tests would still pass.

3. **Sort order tests**: Tests verify sort indicators appear (arrow characters) but don't verify the actual data order after sorting. `TestQA_EC2_A7_1_SortByNameAscending` checks for an up-arrow but doesn't verify "api-prod-01" appears before "bastion" in the output.

### E.3 Structural Weaknesses

1. **Helper duplication**: `stripANSI` is defined in `helpers_test.go` (package `unit`) and redefined as `stripAnsi` in `qa_detail_test.go` (package `unit_test`). This suggests the test files were written by different agents without full coordination.

2. **Package split**: `qa_detail_test.go` uses `package unit_test` (external test) while all other files use `package unit` (internal test). This prevents `qa_detail_test.go` from accessing the helpers in `helpers_test.go` directly, leading to redefinition.

3. **No test for `cmd/a9s/main.go`**: The CLI entry point, argument parsing, and version display are completely untested.

---

## F. Recommendations

### Top 5 Highest-Value Tests to Add

1. **HIGH: List view with RawStruct fixtures** -- Create fixtures that include real AWS SDK RawStruct values (like `qa_detail_test.go` does) but test them in the *list view* context. This would catch column extraction bugs that currently slip through because list tests only use Fields maps. Estimated impact: Would have caught the ViewDef iteration bug before it shipped.

2. **HIGH: `parseCredentialsProfiles` unit test** -- Write tests that provide a mock credentials file content and verify profile names are extracted correctly. Currently at 0% coverage, this is a functional gap.

3. **HIGH: Sort order verification** -- Add tests that assert on actual data order after sorting (e.g., after pressing N, verify the first visible row is alphabetically first). Current tests only check for sort indicator arrows.

4. **MEDIUM: `fetchProfiles` integration test** -- Create a temp directory with mock `config` and `credentials` files, set environment variables to point to them, and verify the full pipeline: fetchProfiles -> profilesLoadedMsg -> push ProfileModel.

5. **MEDIUM: Status color specificity** -- Add tests that verify specific ANSI color codes for each status (green for running/available/ACTIVE, red for stopped/failed, yellow for pending/creating, dim for terminated). Current tests only check "has ANSI codes."

### Top 5 Tests to Rewrite for Better Coverage

1. **QA fixture data** -- Enhance `fixtures_test.go` to include RawStruct fields pointing to real AWS SDK types (already done for `qa_detail_test.go`, but not for the list-level QA tests). This is the single highest-leverage change.

2. **`qa_coverage_gaps_test.go`** -- Replace "did the output change" assertions with specific content assertions. For example, after resize from 80x24 to 120x40, verify that more data rows are visible (not just that the output changed).

3. **Status coloring tests** -- Change from `strings.Contains(line, "\x1b[")` to checking for specific color codes (e.g., `\x1b[32m` for green). The `tui_styles_test.go` already knows the color codes; reuse that knowledge.

4. **Sort tests** -- After sorting, extract the data rows and verify order, not just the arrow indicator. Use subtests for ascending vs descending.

5. **Filter precision tests** -- Some filter tests check `ec2(N/M)` in the frame title but don't verify which specific resources are visible. Add assertions that the filtered resources match expectations by name.

### Architectural Test Improvements

1. **Consolidate package declaration**: Move `qa_detail_test.go` from `package unit_test` to `package unit` and remove the duplicated helper functions.

2. **Add a shared `buildRealisticResource(resourceType string)` helper** that returns a `resource.Resource` with RawStruct populated using real AWS SDK types. This would enable list-level QA tests to use realistic data without duplicating construction code.

3. **Add table-driven sub-tests for cross-resource consistency**: A single test function that iterates over all 7 resource types and verifies: (a) navigate to list, (b) load data, (c) verify frame title format, (d) verify column headers, (e) enter detail, (f) verify detail has content, (g) enter YAML, (h) verify YAML has content. This would be the "smoke test" that catches omissions.

4. **Consider adding golden file tests for View() output**: Capture the full rendered output of each view type with known data and compare against stored snapshots. This would catch visual regressions (column alignment, padding, border characters) that `strings.Contains` assertions miss.

---

## Quality Score: 7.5 / 10

**Justification:**

The test suite is impressive in scale (915 top-level tests, 22K lines, 4.3:1 test:source ratio) and demonstrates strong architectural coverage of all view types, navigation flows, and AWS service integrations. The QA story framework with 11 documented test plans shows systematic thinking about what needs testing. The 88.5% statement coverage is solid.

However, there is a meaningful gap between what the tests *appear* to cover and what they *actually* verify. The reliance on Fields-map fixtures in list-view QA tests creates a blind spot for config-driven column extraction bugs -- exactly the class of bug that has escaped to production before. The status coloring and sort order tests check for symptoms (ANSI codes present, arrow visible) rather than correctness (right color, right order). The `fetchProfiles` and `parseCredentialsProfiles` gaps leave the profile selection feature partially untested.

**Strengths (+):**
- Comprehensive behavioral coverage of all 7 resource types across all view types
- Real AWS SDK types used in detail view tests
- Mock HTTP transport for end-to-end fetch pipeline testing
- Regression tests for known shipped bugs
- Excellent test organization with clear naming and QA story traceability
- All QA story documents have corresponding test files

**Weaknesses (-):**
- List-view QA tests use shallow fixtures (Fields maps, not RawStruct)
- Status color, sort order, and filter tests check presence, not correctness
- Profile/credentials pipeline has significant untested paths
- Some tests written purely to hit coverage numbers, not to catch bugs
- Helper duplication between package `unit` and `unit_test`
