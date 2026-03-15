<!--
Sync Impact Report
===================
Version change: [TEMPLATE] → 1.0.0 (MAJOR: initial constitution)
Modified principles: N/A (first version)
Added sections:
  - Principle I: Test-Driven Development (NON-NEGOTIABLE)
  - Principle II: Code Quality
  - Principle III: Testing Standards
  - Principle IV: User Experience Consistency
  - Principle V: Performance Requirements
  - Section: Quality Gates
  - Section: Development Workflow
  - Section: Governance
Removed sections: None
Templates requiring updates:
  - .specify/templates/plan-template.md ✅ aligned (Constitution Check
    section is generic and will be filled per-feature)
  - .specify/templates/spec-template.md ✅ aligned (acceptance scenarios
    and success criteria support TDD and UX consistency)
  - .specify/templates/tasks-template.md ✅ aligned (test-first task
    ordering and checkpoint gates match constitution principles)
Follow-up TODOs: None
-->

# a9s Constitution

## Core Principles

### I. Test-Driven Development (NON-NEGOTIABLE)

All production code MUST be written using the Red-Green-Refactor cycle:

1. **Red**: Write a failing test that defines the desired behavior.
2. **Green**: Write the minimal code required to make the test pass.
3. **Refactor**: Improve the code while keeping all tests green.

Rules:
- No production code may be written without a corresponding failing
  test first.
- Tests MUST fail for the right reason before implementation begins.
- Each TDD cycle MUST be small: one behavior per cycle.
- Test names MUST describe the behavior under test, not the
  implementation (e.g., `test_returns_404_when_user_not_found` not
  `test_get_user`).
- Skipping or disabling tests (`skip`, `xfail`, `pending`) requires
  a linked issue and MUST NOT persist beyond one release cycle.

Rationale: TDD produces code that is correct by construction, keeps
scope under control, and ensures every line of production code is
exercised by at least one test.

### II. Code Quality

All code MUST meet these non-negotiable standards:

- **Single Responsibility**: Every module, class, and function MUST
  have one clearly defined purpose.
- **No Dead Code**: Unused imports, variables, functions, and commented-
  out code MUST be removed before merge.
- **Naming**: Names MUST be descriptive and unambiguous. Abbreviations
  are prohibited unless they are universally understood domain terms.
- **Formatting**: All code MUST pass the project's configured linter
  and formatter with zero warnings. Formatting MUST be enforced by
  pre-commit hooks or CI.
- **Complexity Limits**: Functions exceeding a cyclomatic complexity
  of 10 MUST be refactored. Files exceeding 300 lines SHOULD be
  split unless a justified exception is documented.
- **No Magic Values**: Literal numbers and strings MUST be extracted
  into named constants with descriptive names.

Rationale: Consistent, clean code reduces cognitive load, speeds up
reviews, and prevents defect accumulation over time.

### III. Testing Standards

The project MUST maintain a layered testing strategy:

- **Unit Tests**: MUST cover all business logic in isolation. External
  dependencies MUST be stubbed or mocked at the boundary. Target:
  ≥90% line coverage on business logic modules.
- **Integration Tests**: MUST verify that components interact correctly
  across boundaries (database, API, file system). Real dependencies
  SHOULD be used via test containers or fixtures where feasible.
- **Contract Tests**: MUST exist for every public API endpoint and
  every inter-service interface. Contract tests MUST validate request
  and response schemas.
- **End-to-End Tests**: MUST cover each critical user journey defined
  in the feature specification (P1 stories at minimum).

Test quality rules:
- Tests MUST be deterministic — no flaky tests allowed. A test that
  fails intermittently MUST be fixed or removed within 48 hours.
- Tests MUST be independent — no test may depend on the execution
  order or side effects of another test.
- Test data MUST be created within the test or via explicit fixtures.
  Shared mutable test state is prohibited.
- Test assertions MUST be specific. A single assertion per logical
  behavior is preferred over asserting multiple unrelated outcomes.

Rationale: A rigorous, layered test suite provides confidence for
refactoring, documents system behavior, and catches regressions at
the earliest possible stage.

### IV. User Experience Consistency

All user-facing interfaces MUST follow these standards:

- **Design Patterns**: Reuse existing UI components and interaction
  patterns before introducing new ones. A new pattern MUST be
  justified in the feature spec and approved before implementation.
- **Responsive Behavior**: All interfaces MUST function correctly
  across the project's supported viewport sizes and devices.
- **Accessibility**: All interfaces MUST meet WCAG 2.1 Level AA.
  Keyboard navigation, screen reader support, and sufficient color
  contrast are mandatory.
- **Error States**: Every user action that can fail MUST display a
  clear, actionable error message. Generic messages like "Something
  went wrong" are prohibited.
- **Loading States**: Every asynchronous operation MUST provide
  visual feedback (spinner, skeleton, progress bar) within 200ms
  of initiation.
- **Consistency Verification**: New UI work MUST be visually
  reviewed against existing screens to ensure consistent spacing,
  typography, color usage, and interaction patterns.

Rationale: Inconsistent UX erodes user trust and increases support
burden. Enforcing consistency from the start is far cheaper than
retroactive alignment.

### V. Performance Requirements

All features MUST meet measurable performance budgets:

- **Response Time**: API endpoints MUST respond within 200ms at p95
  under normal load. Pages MUST achieve Largest Contentful Paint
  (LCP) ≤2.5s and First Input Delay (FID) ≤100ms.
- **Resource Budgets**: JavaScript bundles MUST NOT exceed 250KB
  gzipped per route. Database queries MUST NOT exceed 100ms at p95.
- **Scalability**: Features MUST be load-tested against the defined
  scale targets before release. Performance MUST NOT degrade by more
  than 10% at 2x the expected load.
- **Monitoring**: Every new feature MUST include performance
  instrumentation (metrics, traces, or structured logs) that enables
  detection of regressions in production.
- **Regression Prevention**: Performance benchmarks MUST be included
  in CI. A merge MUST be blocked if it causes a measurable
  regression beyond the defined thresholds.

Rationale: Performance is a feature. Budgets set upfront prevent
the gradual degradation that makes systems unusable and expensive
to remediate.

## Quality Gates

All code changes MUST pass through these gates before merge:

1. **Test Gate**: All tests (unit, integration, contract) MUST pass.
   No new test may be skipped without a linked issue.
2. **Coverage Gate**: Overall line coverage MUST NOT decrease. New
   code MUST meet or exceed the 90% threshold on business logic.
3. **Lint Gate**: Zero linter warnings. Zero formatter violations.
4. **Performance Gate**: CI performance benchmarks MUST NOT regress
   beyond defined thresholds.
5. **Review Gate**: At least one reviewer MUST approve. The reviewer
   MUST verify TDD discipline (tests committed before or with
   implementation, not after).
6. **Accessibility Gate**: New UI changes MUST pass automated
   accessibility checks (axe-core or equivalent).

Exceptions to any gate require written justification in the PR
description and explicit reviewer approval.

## Development Workflow

The standard development workflow enforces constitution principles:

1. **Spec Review**: Read the feature spec and confirm understanding
   of acceptance scenarios before writing any code.
2. **Branch**: Create a feature branch from `main` using the naming
   convention `###-feature-name`.
3. **TDD Loop** (repeat per behavior):
   a. Write a failing test.
   b. Verify it fails for the expected reason.
   c. Write minimal production code to pass.
   d. Refactor while green.
   e. Commit the test and implementation together.
4. **Self-Review**: Run the full test suite, linter, and formatter
   locally before pushing.
5. **Pull Request**: Open a PR with a clear description mapping
   changes to spec requirements. Include evidence of TDD discipline
   (test history in commits).
6. **Review & Merge**: Address review feedback. All quality gates
   MUST pass before merge.
7. **Verify**: Confirm the feature works in the deployed environment
   against the acceptance scenarios from the spec.

## Governance

This constitution is the authoritative source for development
standards in the a9s project. It supersedes conflicting guidance
in any other document.

- **Amendments**: Any change to this constitution MUST be proposed
  as a PR, reviewed by at least one team member, and documented
  with a version bump and rationale.
- **Versioning**: The constitution follows semantic versioning:
  - MAJOR: Principle removed, redefined, or governance restructured.
  - MINOR: New principle or section added, or existing guidance
    materially expanded.
  - PATCH: Wording clarifications, typo fixes, non-semantic changes.
- **Compliance Review**: At the start of each feature, the
  implementation plan MUST include a Constitution Check section
  confirming alignment with all active principles.
- **Exceptions**: Any deviation from a constitution principle MUST
  be documented in the PR description with justification and
  reviewer approval. Recurring exceptions signal the need for a
  constitution amendment.

**Version**: 1.0.0 | **Ratified**: 2026-03-15 | **Last Amended**: 2026-03-15
