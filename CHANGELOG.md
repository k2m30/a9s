# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [3.11.1] - 2026-03-25

### Fixed
- Error flash messages now auto-dismiss after 5 seconds instead of persisting forever (#86)
- Long error messages truncated in header to prevent multi-line wrapping that pushed content off-screen (#84)
- KMS fetcher gracefully skips keys that can't be described (e.g. permission denied) instead of failing entirely (#85)
- DynamoDB fetcher gracefully skips tables that can't be described instead of failing entirely

## [3.9.0] - 2026-03-25

### Added
- Child view: CodeBuild Projects → Builds — drill into any project to see recent builds with status, duration, source version, and initiator (#41)
- Child view: CodeBuild Builds → Build Logs — full build output from CloudWatch Logs with phase/error highlighting (#42)
- Build log status classification: ERROR (red), SUCCEEDED (green), IN_PROGRESS (blue) for quick visual scanning
- `DrillCondition` on build logs: shows "Build logs not available in CloudWatch" when logs are disabled

## [3.8.0] - 2026-03-25

### Added
- Child view: Step Functions → Executions — drill into any state machine to see executions with status, duration, start/stop times (#39)
- Child view: SFN Executions → Execution History — step-by-step state machine trace with event classification, state tracking, and error details (#40)
- `DrillBlockMessage` on `ChildViewDef` — shows flash message when drill is blocked (EXPRESS state machines show explanation)
- `CopyField` on `ResourceTypeDef` — child views can specify which field `c` copies (execution ARN, event detail)
- Status colors for SFN execution states: succeeded, timed_out, aborted, pending_redrive

## [3.4.2] - 2026-03-24

### Fixed
- NO_COLOR cursor visibility: use ANSI reverse video for selected row (#74)

## [3.4.1] - 2026-03-24

### Changed
- Split oversized source files to comply with go-codebase-checklist size constraints (#71):
  - `types.go` (1093→108 lines): split into 12 category files + `columns.go`
  - `defaults.go` (1105→54 lines): split into 12 category files
  - `app.go` (953→345 lines): split into `app_handlers.go`, `app_input.go`, `app_fetchers.go`
  - 7 AWS fetcher functions: extracted per-item `convertXxx()` helpers (all under 50 lines)
  - Demo fixtures: split 6 oversized files into smaller category files
  - Test files: split top 3 oversized files (`qa_redis_docdb`, `qa_mainmenu`, `mocks`)
- Website UX/UI audit — 18 fixes (#52)

### Added
- "Why a9s?" section with real-life use cases in README and website
- Documentation for `NO_COLOR`, `AWS_PROFILE`, `AWS_REGION` environment variables
- CFN Stacks added to README feature summary drill-down list
- v3.3.0 and v3.4.0 release notes to website

### Added
- `--demo` mode with synthetic fixtures for all 62 resource types (no AWS credentials needed)
- Demo GIF embedded in README and website
- Credential hardening: app never reads `~/.aws/credentials`
- GPL-3.0-or-later license
- Full README with features, installation, key bindings, and service catalog
- CONTRIBUTING.md, CODE_OF_CONDUCT.md, SECURITY.md, SUPPORT.md, ROADMAP.md
- GitHub Actions CI pipeline (lint, test, build, security, read-only verification)
- CodeQL security scanning
- GoReleaser configuration for multi-platform releases
- Homebrew tap (k2m30/homebrew-a9s)
- Docker image (ghcr.io/k2m30/a9s) running in demo mode
- Cosign binary signing and SBOMs
- Dependabot configuration for Go modules and GitHub Actions
- Issue templates and PR template
- Stale issue/PR automation
- Hugo website with landing page, install guide, and docs
- S3 drill-down into bucket objects (demo mode)
- Route 53 drill-down into zone records (demo mode)

### Changed
- Version now injected via build-time ldflags (no longer hardcoded)
- Expanded linter configuration with gosec, gocritic, bodyclose, noctx
- Enhanced Makefile with security scanning, coverage, and read-only API verification
- Bumped Go to 1.26.1 (resolved 14 stdlib CVEs)
- golangci-lint v2 config migration

## [0.5.0] - 2026-03-16

### Fixed
- 15 UI bugs: filter, navigation, YAML, scroll, help, header, detail views

## [0.4.5] - 2026-03-16

### Added
- Configurable views via YAML config (`~/.a9s/views.yaml`)

## [0.3.2] - 2026-03-15

### Added
- Horizontal scroll for wide tables
- 517 comprehensive QA tests
- CI pipeline
- Consolidated mock infrastructure

### Fixed
- Layout, navigation, and S3 bugs
- Unicode separator, duplicate version constant

## [0.3.0] - 2026-03-15

### Added
- Horizontal scroll for wide tables
- Removed all skipped tests

## [0.2.0] - 2026-03-15

### Added
- 517 QA tests covering layout, navigation, S3 bugs

### Fixed
- Layout and navigation bugs
- S3 view issues

## [0.1.0] - 2026-03-15

### Added
- Initial release
- S3, EC2, RDS, Redis, DocumentDB, EKS, Secrets Manager resource types
- YAML detail view
- Multi-profile and multi-region support
- Tokyo Night Dark theme
- Clipboard support
- Help overlay

[Unreleased]: https://github.com/k2m30/a9s/compare/v3.0.0-alpha.4...HEAD
[0.5.0]: https://github.com/k2m30/a9s/compare/v0.4.5...v0.5.0
[0.4.5]: https://github.com/k2m30/a9s/compare/v0.3.2...v0.4.5
[0.3.2]: https://github.com/k2m30/a9s/releases/tag/v0.3.2
