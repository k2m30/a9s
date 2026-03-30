# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [3.26.0] - 2026-03-30

### Added
- Resource cache — loaded resources preserved on Esc+re-enter, selective invalidation on profile/region switch (#111)
- Cross-view search with `/` activation, `n`/`N` navigation, ANSI-aware highlighting across list, detail, and YAML views (#89)
- Search bindings added to detail and YAML help screens (#89)
- 68 QA story tests for search covering activation, navigation, highlighting, edge cases (#89)

### Fixed
- Availability probes retry up to 3x with exponential backoff on ThrottlingException instead of silently failing (#186)
- `/` key routing delegates to Searchable views correctly (#89)
- `recomputeMatches` no longer resets `currentIdx` on every call (#89)
- Enter on empty search query deactivates instead of showing 0/0 (#89)

## [3.21.0] - 2026-03-28

### Added
- Status color-coding expanded from 20 to 52 mappings across 16 resource types (#61)
- CloudFormation pattern-based color matching — `*_COMPLETE` green, `*_IN_PROGRESS` yellow, `*_FAILED` red (#61)
- Target Group Health colors: healthy/unhealthy/draining/initial/unused/unavailable (#61)
- CloudWatch Alarm colors: OK/ALARM/INSUFFICIENT_DATA (#61)
- ACM, CloudFront, EventBridge, KMS, MSK, Redshift, SES, Athena, VPC Endpoint status colors (#61)
- Elastic Beanstalk health-based row coloring (Green/Yellow/Red/Grey) (#61)

### Changed
- SES row coloring now uses verification status (was empty) (#61)
- CloudFront disabled distributions show dim/grey rows instead of deployed green (#61)
- Elastic Beanstalk row coloring uses Health field instead of operational Status (#61)

## [3.20.0] - 2026-03-28

### Added
- Resource counts in main menu — each service shows live count, e.g. "EC2 Instances (25)" (#68)
- Resource availability caching — grey out empty resource types, cursor skips them (#68)
- Pagination UX — help context shows load-more hint, availability truncation indicator (#68)
- Demo mode via mock AWS transport — app code is 100% identical in demo and real modes (#103)
- 63 SDK-level transport tests covering all 62 resource types through real AWS SDK clients (#103)
- S3 folder drill-down with prefix-filtered fixtures for all 6 demo buckets (#103)
- Annotated demo GIF with 9 scene overlays explaining navigation and key bindings

### Changed
- Demo mode now uses real SDK clients with an in-process mock `http.RoundTripper` instead of separate fetch code paths (#103)
- Removed 140+ lines of `if m.demoMode` branches from app.go, app_fetchers.go, app_handlers.go (#103)
- Deleted old demo pagination engine (pagination.go) — SDK handles pagination natively now (#103)

### Fixed
- S3 folder navigation in demo mode — drilling into folders no longer shows the same top-level listing (#103)
- ECS services/tasks duplicate IDs — handlers now filter by cluster from request body (#103)
- KMS DescribeKey returns matching key instead of always the first one (#103)
- API Gateway handler uses correct camelCase keys for SDK deserialization (#103)
- ASG handler no longer emits empty `<NextToken>` that caused infinite pagination loops (#103)
- EC2 instances missing names — tag serialization added to XML response (#103)
- Demo probe counts match paginated page size (#68)
- "Refreshing availability..." flash auto-clears when all checks complete (#68)
- Cursor skips greyed-out empty resource types in main menu (#68)

## [3.17.0] - 2026-03-27

### Added
- Unified pagination for all 62 top-level fetchers and 28 child fetchers (#83, #80)
- `M` key for load-more in paginated child views
- RetryOnThrottle infrastructure for AWS API rate limiting
- Demo mode pagination — ~30% of resources demonstrate load-more UX with 5-item pages
- 65 QA stories with full test coverage for pagination

## [3.16.0] - 2026-03-26

### Added
- IAM identity in header and detail view — press `i` to see account, caller, and session info; header shows account badge and role/user name (#77)
- EC2 Spot vs On-Demand instance lifecycle column — shows "spot" or "on-demand" with filter support (#72)
- Design spec and preview renders for the identity view

### Fixed
- Name column is now first in all default list views — 14 resource types reordered (sg, vpc, subnet, rtb, nat, igw, eip, vpce, tgw, eni, r53, cf, apigw, efs) (#23)
- Header identity badge gracefully drops on narrow terminals instead of wrapping to a second line

## [3.15.0] - 2026-03-26

### Added
- Child view: Glue Jobs → Job Runs — drill into job execution history with status, duration, DPU usage, and error messages (#49)

## [3.14.0] - 2026-03-26

### Added
- Child view: EventBridge Rules → Targets — drill into rule targets showing ARN, input config, and role (#48)

## [3.13.0] - 2026-03-26

### Added
- Child view: SNS Topics → Subscriptions — drill into topic subscribers showing protocol, endpoint, and confirmation status (#47)

## [3.12.0] - 2026-03-25

### Added
- Child view: RDS Instances → RDS Events — drill into DB instance events showing failovers, maintenance, and reboots (#45)
- 68 QA stories for cross-view search component (#89)
- 138 QA stories for Policy Document grandchild view (#87)

## [3.11.3] - 2026-03-25

### Added
- ECS Services list view now shows cluster name column (#90)

### Fixed
- Demo mode uses short cluster names in ECS service fixtures
- Architecture review issues and coverage gaps

## [3.11.2] - 2026-03-25

### Added
- Respect `AWS_CONFIG_FILE` environment variable for profile discovery, enabling per-project AWS config via direnv (#88)

### Fixed
- Test count in README and website updated to reflect actual count (2,300+)

## [3.11.1] - 2026-03-25

### Fixed
- Error flash messages now auto-dismiss after 5 seconds instead of persisting forever (#86)
- Long error messages truncated in header to prevent multi-line wrapping that pushed content off-screen (#84)
- KMS fetcher gracefully skips keys that can't be described (e.g. permission denied) instead of failing entirely (#85)
- DynamoDB fetcher gracefully skips tables that can't be described instead of failing entirely

## [3.11.0] - 2026-03-25

### Added
- Child view: IAM Roles → Attached Policies — drill into a role to see managed and inline policies; AdministratorAccess and PowerUserAccess highlighted in red (#46)
- Child view: IAM Groups → Group Members — drill into a group to see member users with username, user ID, and creation date (#50)
- Child view: ELB Listeners → Listener Rules — nested level-2 child (ELB → Listeners → Rules) with routing priority, conditions, action type, and target (#51)

## [3.10.0] - 2026-03-25

### Added
- Child view: CodePipeline → Pipeline Stages — drill into a pipeline to see stage/action status with visual grouping and deep links (#43)
- Child view: ECR Repositories → Images — browse container images with tags, digest, size, push date, and vulnerability scan results (#44)

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

## [3.7.0] - 2026-03-25

### Added
- Child view: Load Balancers → Listeners — drill into any ALB/NLB to see listeners with port, protocol, default action, target group, SSL policy, and certificate (#37)

## [3.6.0] - 2026-03-24

### Added
- Child view: Auto Scaling Groups → Scaling Activities — drill into any ASG to see recent scaling events with status, cause, and timestamps (#36)
- Child view: CloudWatch Alarms → Alarm History — drill into any alarm to see state transition history (#38)

## [3.5.1] - 2026-03-24

### Fixed
- Docker container home directory permissions — `COPY` created `/home/a9s` owned by root (#78)
- Docker theme color degradation — `FROM scratch` ships zero env vars, so Lipgloss degrades hex colors to ANSI-256; now sets `TERM` and `COLORTERM` (#79)

## [3.5.0] - 2026-03-24

### Added
- Windows support (amd64 and arm64) (#55)
- Scoop package manager — install on Windows with `scoop bucket add a9s && scoop install a9s`
- Windows added to CI test matrix

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

## [3.4.0] - 2026-03-24

### Added
- Child view: CFN Stacks → Stack Events — real-time deployment timeline showing resource creation/update/failure (#35)
- Child view: CFN Stacks → Stack Resources — logical-to-physical resource mapping with drift detection; access via `r` key (#35)
- CFN Stacks is the second multi-child parent (after ECS Services): `Enter` = events, `r` = resources

## [3.3.0] - 2026-03-24

### Added
- Child view: ECS Services → Tasks (`Enter`) — running and recently stopped tasks with status, health check, task definition revision (#33)
- Child view: ECS Services → Service Events (`e`) — event timeline with steady state, placement failures, deployment progress (#33)
- Child view: ECS Services → Container Logs (`L`) — application logs from CloudWatch, resolved from task definition's awslogs config (#34)
- ECS Services is the first multi-child parent: `Enter` = tasks, `e` = events, `L` = logs
- `cmd/viewsgen/` tool to auto-generate `.a9s/views.yaml` from built-in defaults

## [3.2.0] - 2026-03-24

### Added
- Child view: Lambda → Invocations — recent invocations parsed from CloudWatch Logs REPORT lines with duration, memory, cold start (#31)
- Child view: Lambda Invocations → Log Lines — full log output for a single invocation, color-coded by severity (#32)
- 3-level drill chain: Lambda Functions → Invocations → Log Lines

## [3.1.0] - 2026-03-23

### Added
- Child view: Log Groups → Log Streams → Log Events — 3-level drill-down for CloudWatch Logs with color-coded severity
- Child view: Target Groups → Target Health — per-target health status with reason and description
- Human-readable formatting for byte sizes and timestamps across all views
- Narrow screen support — wide columns shrink to fit instead of disappearing
- Sort by age now works for child view fields

## [3.0.1] - 2026-03-23

### Fixed
- `a9s --version` now resolves commit hash and build date from Go's embedded VCS metadata when ldflags aren't set
- Docker semver tags — images now tagged with `v3`, `v3.0`, `v3.0.1`, and `latest`
- Release pipeline OOM during cross-compilation fixed by limiting GoReleaser parallelism

## [3.0.0] - 2026-03-23

### Added
- `--demo` mode with synthetic fixtures for all 62 resource types (no AWS credentials needed)
- 62 AWS resource types across 12 service categories
- Child view architecture — drill into S3 objects, Route 53 records, and more
- Tokyo Night Dark theme with status-aware row coloring
- Configurable columns via `~/.a9s/views.yaml`
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
- Hugo website with landing page, install guide, and docs

### Changed
- Version now injected via build-time ldflags (no longer hardcoded)
- Expanded linter configuration with gosec, gocritic, bodyclose, noctx
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

[Unreleased]: https://github.com/k2m30/a9s/compare/v3.16.0...HEAD
[3.16.0]: https://github.com/k2m30/a9s/compare/v3.15.0...v3.16.0
[3.15.0]: https://github.com/k2m30/a9s/compare/v3.14.0...v3.15.0
[3.14.0]: https://github.com/k2m30/a9s/compare/v3.13.0...v3.14.0
[3.13.0]: https://github.com/k2m30/a9s/compare/v3.12.0...v3.13.0
[3.12.0]: https://github.com/k2m30/a9s/compare/v3.11.3...v3.12.0
[3.11.3]: https://github.com/k2m30/a9s/compare/v3.11.2...v3.11.3
[3.11.2]: https://github.com/k2m30/a9s/compare/v3.11.1...v3.11.2
[3.11.1]: https://github.com/k2m30/a9s/compare/v3.11.0...v3.11.1
[3.11.0]: https://github.com/k2m30/a9s/compare/v3.10.0...v3.11.0
[3.10.0]: https://github.com/k2m30/a9s/compare/v3.9.0...v3.10.0
[3.9.0]: https://github.com/k2m30/a9s/compare/v3.8.0...v3.9.0
[3.8.0]: https://github.com/k2m30/a9s/compare/v3.7.0...v3.8.0
[3.7.0]: https://github.com/k2m30/a9s/compare/v3.6.0...v3.7.0
[3.6.0]: https://github.com/k2m30/a9s/compare/v3.5.1...v3.6.0
[3.5.1]: https://github.com/k2m30/a9s/compare/v3.5.0...v3.5.1
[3.5.0]: https://github.com/k2m30/a9s/compare/v3.4.2...v3.5.0
[3.4.2]: https://github.com/k2m30/a9s/compare/v3.4.1...v3.4.2
[3.4.1]: https://github.com/k2m30/a9s/compare/v3.4.0...v3.4.1
[3.4.0]: https://github.com/k2m30/a9s/compare/v3.3.0...v3.4.0
[3.3.0]: https://github.com/k2m30/a9s/compare/v3.2.0...v3.3.0
[3.2.0]: https://github.com/k2m30/a9s/compare/v3.1.0...v3.2.0
[3.1.0]: https://github.com/k2m30/a9s/compare/v3.0.1...v3.1.0
[3.0.1]: https://github.com/k2m30/a9s/compare/v3.0.0...v3.0.1
[3.0.0]: https://github.com/k2m30/a9s/compare/v0.5.0...v3.0.0
[0.5.0]: https://github.com/k2m30/a9s/compare/v0.4.5...v0.5.0
[0.4.5]: https://github.com/k2m30/a9s/compare/v0.3.2...v0.4.5
[0.3.2]: https://github.com/k2m30/a9s/releases/tag/v0.3.2
