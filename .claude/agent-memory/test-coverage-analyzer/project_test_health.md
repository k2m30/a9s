---
name: Test Suite Health Assessment
description: Comprehensive test coverage analysis after TUI rewrite - 88.5% coverage, 915 tests, critical gaps in RawStruct list fixtures and profile loading
type: project
---

a9s test suite assessed 2026-03-18: 88.5% statement coverage, 915 top-level tests (1311 including subtests), 22,562 lines of test code across 46 files. All 11 QA story documents have corresponding test files.

**Critical gaps identified:**
- List-view QA tests (qa_ec2_test.go, qa_s3_test.go, etc.) use flat Fields maps, not RawStruct with real AWS SDK types. This means config-driven column extraction is NOT tested in list context. This is the exact class of bug that shipped before (ViewDef iteration bug).
- `fetchProfiles` at 11.1% coverage, `fetchSecretValue` at 44.4%, `parseCredentialsProfiles` at 0%.
- Status color tests check for "has ANSI codes" not "has correct color."
- Sort tests check for arrow indicators not actual data order.

**Why:** The test suite looks comprehensive by metrics but has a blind spot in how list views render data from AWS SDK structs. Detail view tests (qa_detail_test.go) use real SDK types and are excellent; the same pattern needs to be applied to list-level QA tests.

**How to apply:** When writing new tests for list views, always use fixtures with RawStruct populated from real AWS SDK types. When assessing test coverage, don't just look at the percentage -- verify that assertions check correctness, not just presence.
