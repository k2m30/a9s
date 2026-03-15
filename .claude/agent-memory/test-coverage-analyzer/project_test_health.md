---
name: a9s test health baseline (2026-03-15)
description: Comprehensive test coverage analysis of the a9s Go TUI AWS resource manager project, including per-package coverage, test counts, and identified gaps
type: project
---

Test health baseline as of 2026-03-15.

**Why:** Establishes a reference point for test quality improvements and identifies high-risk untested areas.

**How to apply:** Reference this when adding features or refactoring -- ensure new code maintains or improves the 80.6% overall coverage, and prioritize closing the gaps identified below (especially internal/ui at 22.5% and internal/views at 58.3%).

Key facts:
- Overall coverage: 80.6% (unit tests only, via `-cover -coverpkg=./internal/...`)
- Per-package coverage: app=86.5%, aws=91.3%, navigation=100%, resource=100%, styles=100%, views=58.3%, ui=22.5%
- 778 test cases total (773 pass, 5 fail as of this date)
- All tests are in external test package (tests/unit, tests/integration) -- no in-package _test.go files exist
- Go 1.25 `covdata` tool issue prevents `-coverprofile` output; use `-cover -coverpkg=` instead
- 5 failing tests: TestQA_012_VersionFlag (stale version constant), 3 horizontal_scroll tests (layout issues), 1 qa_layout_rendering test
- Test framework: stdlib testing only (no testify, no gomock)
- Mocking pattern: hand-written interface mocks for each AWS service (well-structured via interfaces.go)
- CI: No GitHub Actions or CI config found (.github/ absent)
- Integration tests use build tag `integration` and compile the binary to test CLI flags
