---
name: Test Suite Health Assessment
description: Comprehensive test coverage analysis (3,058 test runs, 1,813 func Test*, 132 files, 93.5% statement coverage) - buildinfo covered at 85.7%, CI path filter gap for Dockerfile/.goreleaser.yaml
type: project
---

a9s test suite assessed 2026-03-22: 1,813 top-level test functions producing 3,058 test runs (including subtests), ~49,800 lines of test code across 132 unit test files + 5 integration test files. All tests pass (0 FAIL) in ~5.3s. Only 1 top-level skip (TestQA_Profile_FrameTitle) + 4 sub-test skips. Overall instrumented statement coverage: 93.5% across all internal packages.

**Buildinfo package (new):**
- `internal/buildinfo/buildinfo.go` has 85.7% coverage from unit tests in `tests/unit/version_test.go` (3 tests).
- Uncovered branch: the `return info.Main.Version` path at line 13 (only reachable when binary installed via `go install` with tagged module -- cannot be unit tested).
- The ldflags flow (goreleaser, Makefile, release.yml all inject `main.version`, `main.commit`, `main.date`) is consistent with `main.go` var declarations.

**CI/Infrastructure gaps:**
- CI workflow triggers on `pull_request` only -- direct pushes to main skip CI entirely (was `push` + `pull_request` previously).
- CI `paths` filter does NOT include `Dockerfile`, `.goreleaser.yaml`, or `.github/workflows/release.yml`. Changes to these files never trigger CI, even on PRs.
- CLAUDE.md mentions an `install-test` CI job that does not exist in ci.yml.
- Integration test `TestQA_012_VersionFlag` has a potentially flawed assertion: checks `strings.Contains(output, ".")` but when built without ldflags the output is "a9s dev (commit: none, built: unknown)" which contains no dots. (Behind integration build tag, so not routinely run.)

**Gaps closed since last assessment (2026-03-21):**
- R53 records config defaults: 6 new tests in `qa_r53_records_config_test.go` -- fully covered.
- R53 records TUI drill-down: 41 test functions in `qa_r53_records_test.go` covering R53EnterZoneMsg at view and root model levels.
- demo.GetR53Records(): now thoroughly tested (14+ call sites in tests).
- Skips reduced from 5 to 1 top-level + 4 sub-test skips (same 5 total, but previously reported as 5 separate skips).
- Test count grew from 1,763 to 1,813 func Test* (3,058 total runs, was 3,002).

**Remaining 0% coverage functions (11 total):**
1. `CreateServiceClients` (client.go:115) -- integration-only, needs real AWS config
2. `s3ObjWebapp`, `s3ObjMLTraining`, `s3ObjTerraform`, `s3ObjCloudtrail`, `s3ObjBackups` (demo/fixtures.go) -- 5 S3 object fixture helpers that are registered but never fetched in any test
3. `handleProfilesLoaded` (app.go:359) -- profile loading pipeline, no unit test
4. `PlainContent` (detail.go:149) -- clipboard-related method
5. `Init` methods for help, mainmenu, yaml views -- no-ops per BT v2 pattern, not tested

**Why:** The suite is in excellent health at 93.5% coverage with strong breadth. The buildinfo package is adequately covered. The primary risk is in CI infrastructure: Dockerfile and goreleaser changes can silently break releases.

**How to apply:** When reviewing release/infrastructure changes, note that CI does not validate them. The --demo CLI integration test is still missing. The 5 uncovered S3 object fixture helpers represent mild risk (data quality of 5 S3 buckets' demo objects is untested).
