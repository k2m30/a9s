# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
