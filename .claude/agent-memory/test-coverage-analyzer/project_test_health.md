---
name: a9s test health baseline (2026-03-15, updated)
description: Comprehensive test coverage analysis of the a9s Go TUI AWS resource manager project -- 87.3% coverage, 779 tests, per-package breakdown, and identified gaps including 0% coverage on extractCellValue/findColumnPathBySubstr
type: project
---

Test health baseline as of 2026-03-15 (updated after configurable views feature).

**Why:** Establishes a reference point for test quality improvements and identifies high-risk untested areas.

**How to apply:** Reference this when adding features or refactoring -- ensure new code maintains or improves the 87.3% overall coverage, and prioritize closing the gaps identified below (especially extractCellValue at 0%, fetchResources at 12%).

Key facts:
- Overall coverage: 87.3% (unit tests only, via `-coverprofile` with `-coverpkg=./internal/...`)
- 779 test functions, 2 benchmarks, 0 failures, 0 skipped unit tests
- Test-to-source ratio: 3.7:1 (19,164 test LOC / 5,151 source LOC)
- Per-package: app=~89%, aws=~92%, config=~84%, fieldpath=~80%, navigation=100%, resource=100%, views=~96%, ui=~96%, styles=~50%
- New packages (fieldpath, config) have dedicated test files with good coverage of core paths
- CI: GitHub Actions runs unit tests + build on push/PR (.github/workflows/test.yml)
- Critical gaps: extractCellValue 0%, findColumnPathBySubstr 0%, fetchResources 12%, fetchS3Objects 18%, lowerFirst 0% (dead code), DefaultConfig 0%
- Test framework: stdlib testing only, hand-written interface mocks
- All tests in external test packages (tests/unit/, tests/integration/)
