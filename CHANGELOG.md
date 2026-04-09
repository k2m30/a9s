# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- Demo mode (`--demo`) fully migrated from legacy HTTP transport to per-service typed fakes. All 66 resource types now served by 42 typed fakes in `internal/demo/fakes/` backed by SDK-typed fixture data in `internal/demo/fixtures/`. Deleted the legacy fixture stores (`demoData`, `childDemoData`, `RegisterRelatedDemo`), all `fixtures_*.go` category files, all `handlers_*.go` transport handlers, and the parallel demo-related registry. Only the STS handler remains for `GetCallerIdentity` probes.
- IAM Policies list now includes inline group policies alongside managed policies. Replaced "Policy ID" column with "Type" (managed/inline). Related panel for inline policies shows the parent group.
- CloudTrail navigable fields fixed: AWSService events no longer pollute `Fields["user"]` and `Fields["role_name"]` with service principals; AssumedRole events no longer copy role names into the user navigation field.
- Demo main menu now shows resource counts on startup (availability probes run in noCache mode).
- Go toolchain bumped from 1.26.1 to 1.26.2 (fixes 4 crypto/x509 + crypto/tls stdlib vulnerabilities).

## [3.33.0] - 2026-04-07

### Changed
- CloudTrail events list view redesigned (v2): one row one color, severity-based tinting (`ct-info` dim / `ct-attention` yellow / `ct-danger` red) replacing the per-cell ANSI composition pipeline.
- Verb classification table updated: `BatchGet*`, `Decrypt`, `Encrypt`, `Sign`, `GenerateDataKey*` now correctly classified as read (`R`).
- `AssumeRole` and `AssumeRoleWithSAML` reclassified from write (`W`) to read (`R`) — STS session vending is identity exchange, not a state mutation. Ordinary AssumeRole events now render as `ct-info` instead of `ct-attention`; cross-account, Root, and error paths still escalate.
- Cross-account events in ct-events now show the counterparty account ID inline in the ACTOR column (`999988887777/alice`) and TARGET column (for ARNs from other accounts).
- TIME column now renders as `Apr 07 17:00:59` (15 chars) instead of ISO timestamp.
- TARGET column strips ARN prefix to show just the resource portion.
- Sort indicator glyph now bound to exactly one column per sort mode (fixes double-glyph on ct-events TIME/EVENT columns).
- IAM `ListEntitiesForPolicy` consolidated from three filtered calls per policy to one unfiltered call partitioned client-side, with a 5-second per-policy cache. Reduces API quota use on policy detail views by 3×.
- Help screen ct-events legend now shows the three real row-tint colors (dim/yellow/red) instead of seven decorative palette colors that contradicted the design spec.
- `cache.Load` now validates resource keys against the registry and skips unknown keys instead of silently retaining them.

### Added
- `ctrl+z` — global "show only attention-worthy rows" filter on every resource list view. Hides dim/neutral rows (e.g. ct-info events, terminated EC2 instances).
- CloudTrail target fallback table for management events with empty `resources[]` (`DescribeInstances`, `GetParameter`, `GetSecretValue`, `AssumeRole`, etc.).
- Sensitive-reads allowlist: events reading secret material (Secrets Manager, SSM Parameters, STS AssumeRole, IAM credential reports, ACM exports) escalate to `ct-attention` severity.
- App-wide cancellation context. `tui.Model` now owns an `appCtx` created in `New()` and cancelled on quit; fetchers and IAM related-checkers use it instead of `context.Background()`, so navigating away or quitting actually cancels in-flight AWS calls.

### Fixed
- Demo-mode related counters now match the navigation target lists. Previously a `policy` detail could show "5 Roles" but pressing Enter opened an empty list; affected `policy → role/iam-user/iam-group` and `role → lambda/glue` fixtures.
- Demo handlers no longer swallow `Sscanf`/`Unmarshal` errors — malformed pagination tokens or request bodies now produce HTTP 400 instead of zero-offset silent success.
- Demo fixture `mustParseTime` panic on bad RFC3339 literal replaced with `ParseTime` returning an error.
- Removed deprecated `SecretName` field from `ValueRevealedMsg`; all callers use `ResourceID`.

### Removed
- `ListColumn.Color` field — per-cell color classifiers are no longer supported.
- Per-cell ANSI composition helpers (`ApplyCellColor`, `applyVerbColor`, `applyActorColor`, `applyOutcomeColor`, `applyOriginColor`, etc.) and their tests.
- Legacy `ct-write`/`ct-read` status values and `[cross]` actor prefix.

## [3.32.3] - 2026-04-07

### Fixed
- CloudTrail Events `User` column now shows `invokedBy` (e.g., `ec2.amazonaws.com`) for `AWSService` identity events (EC2 instance profile credential refresh via STS)

## [3.32.2] - 2026-04-07

### Fixed
- Cmd+V and ctrl+V paste now work in filter mode (`/`) — pasted text is applied as a filter immediately
- Cmd+V and ctrl+V paste now work in search mode (`/` in detail/YAML views) — pasted text is appended to the search query
- CloudTrail Events `User` column is no longer empty for `AssumeRole` events — the role name from `userIdentity.sessionContext.sessionIssuer.userName` is shown as a fallback when `Username` is nil

## [3.32.1] - 2026-04-07

### Fixed
- IAM User → CloudTrail Events counter in the related panel no longer shows a misleading partial count (e.g., `(4)`) when the event cache is truncated — the panel now shows the entry without a count and navigates to the full server-side filtered result
- EC2 → CloudTrail Events has the same fix — truncated cache no longer produces a wrong counter
- CloudTrail AssumedRole events now correctly identify the associated IAM Role via `userIdentity.sessionContext.sessionIssuer.userName` in the raw event JSON; the role name is stored as `Fields["role_name"]` and the `IAM Roles` related panel now shows a non-zero count
- `ct-events` detail view now registers `user` → `iam-user` and `role_name` → `role` as navigable fields, making usernames and role names clickable
- IAM User → IAM Policies, IAM Role → Policies, IAM Group → Policies were navigating to empty lists because the policy ID comparison was using `PolicyArn` while the policy fetcher stores `PolicyName` as the resource ID

## [3.32.0] - 2026-04-06

### Added
- EC2 status check indicators in list view — `! running` for impaired, `~ running` for initializing (#188)
- EC2 Status Checks section in detail view with color-coded System/Instance values (#188)
- `DescribeInstanceStatus` API call integrated into EC2 fetcher with graceful degradation
- Demo mode fixtures with impaired and initializing EC2 instances

### Fixed
- Pre-existing lint warning in `cmd/preview/main.go` (if-else chain → switch)

## [3.31.0] - 2026-04-06

### Added
- Related-resource views for **all 66 resource types** — press `r` on any detail view to see cross-resource relationships with navigable fields and background availability checking:
  - **Compute:** EC2, ASG, Lambda, Elastic Beanstalk
  - **Containers:** ECS Clusters, ECS Services, ECS Tasks, ECR, EKS, Node Groups
  - **Databases:** RDS, RDS Snapshots, DocumentDB, DocDB Snapshots, DynamoDB, Redis, Redshift, OpenSearch
  - **Storage:** S3, EBS Volumes, EBS Snapshots, AMIs, EFS, Backup
  - **Networking:** VPC, Subnets, Security Groups, ELB, Target Groups, NAT, IGW, Route Tables, Transit Gateways, VPC Endpoints, Elastic IPs, ENI, CloudFront, API Gateway, ACM, Route 53, WAF
  - **Security:** IAM Roles, IAM Users, IAM Groups, IAM Policies, KMS, Secrets Manager, SSM Parameters
  - **Monitoring:** CloudWatch Alarms, Log Groups, CloudTrail, CloudTrail Events
  - **CI/CD:** CloudFormation, CodeBuild, CodePipeline, CodeArtifact, Athena
  - **Messaging:** SQS, SNS, SNS Subscriptions, EventBridge, Kinesis, Step Functions, MSK, SES, Glue
- All 66 related-view checkers are real implementations — zero nil stubs remain
- `TestGolden_LiveCheckerCompleteness` gate test fails the build if any nil checkers are introduced (#243)
- Demo mode fixtures for all 66 resource types with cross-linked IDs for realistic related-view navigation

### Changed
- Consolidated 19 individual related smoke tests into a single table-driven test file
- Replaced `interface{}` with `any` across 95 files in `internal/aws/`
- Replaced manual contains-loops with `slices.Contains` (3 files)
- Replaced `strings.Index` with `strings.Cut` (2 files)
- Replaced `strP` helper with `aws.String` in Transit Gateway related checker

### Fixed
- All nil stub related-view checkers replaced with real cache-based or API-calling implementations (#243)
- SNS Subscriptions no longer shows "(0+)" on accounts with zero subscriptions — known AWS API quirk where `ListSubscriptions` returns a `NextToken` with empty results
- SNS Topics gets the same defensive guard for empty-page truncation
- Secrets Manager now fetches up to 50 items per page instead of AWS default (~10)
- SSM Parameters now fetches up to 50 items per page instead of AWS default (~10)
- Lowercase `m` now triggers load-more in paginated lists (previously only `M` worked)

## [3.29.0] - 2026-04-04

### Added
- Related-resource views for 23 resource types — press `r` on any detail view to see cross-resource relationships with navigable fields and background availability checking:
  - **Compute:** EC2 Instances, Auto Scaling Groups, Lambda (via ECR), Elastic Beanstalk
  - **Containers:** ECS Clusters, ECR Repositories, EKS Node Groups (via EC2)
  - **Databases:** RDS Instances, DocumentDB Clusters, DocumentDB Snapshots, DynamoDB Tables
  - **Storage:** EBS Volumes, EBS Snapshots, AMIs
  - **Networking:** CloudFront Distributions, API Gateways, ACM Certificates
  - **Monitoring:** CloudWatch Alarms, CloudTrail Events
  - **CI/CD:** CloudFormation Stacks, CodeBuild Projects, CodeArtifact Repositories
  - **Messaging:** EventBridge Rules
- Navigable fields in detail view left column — highlighted fields link directly to related resources (e.g., VpcId → VPC, SubnetId → Subnet, KmsKeyId → KMS)
- Cache-based related checkers with truncation guards — partial-page cache returns "?" instead of false zero
- Demo mode fixtures for all 23 related-view resource types with cross-linked IDs

### Fixed
- CloudWatch Alarm dimension filter now correctly matches InstanceId dimensions (#189)
- Demo fixture RawStructs fully cross-linked for realistic related-view navigation (#189)
- All navigable field paths resolve to non-empty values in demo fixtures (#189)

## [3.28.0] - 2026-04-02

### Added
- Related-views infrastructure — two-column detail view with right panel showing related resource counts and navigation (#198)
- `RelatedDef`, `NavigableField`, and `RelatedChecker` type system in `resource/related.go`
- Generic message-driven dispatch for `RelatedCheckResultMsg` and `RelatedNavigateMsg`
- `r` key to toggle related panel, `Tab` to switch columns, `Enter` to navigate
- Resource cache integration — checkers read from already-loaded resource lists before falling back to live API
- QA stories for related-resources feature (#119, #189)

## [3.27.0] - 2026-04-01

### Fixed
- Config overlay race condition during profile switch (#119)
- Availability probe race condition on rapid view transitions
- Profile rollback on failed AWS connection attempt
- Region resolution edge case with multi-region config files
- CI failures — clipboard tests skip on headless, verify-readonly path fix

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

## [3.25.0] - 2026-03-29

### Added
- CloudTrail Events structured field parsing — JSON `CloudTrailEvent` blob parsed into typed fields (EventName, EventSource, SourceIPAddress, UserAgent, ErrorCode) for detail and YAML views (#117, #118)

## [3.24.0] - 2026-03-29

### Changed
- Migrated all fetchers from legacy `RegisterFetcher` to `RegisterPaginatedFetcher`, removed dual-registry system (#83)

### Fixed
- Demo mode overrides profile/region to "demo" values regardless of CLI flags or environment
- Missing demo transport handlers for EBS Volumes, EBS Snapshots, CloudTrail Events

## [3.23.0] - 2026-03-29

### Added
- EBS Volumes, EBS Snapshots, AMIs, and CloudTrail Events as new resource types with full pagination support

## [3.22.0] - 2026-03-28

### Added
- Generalized secret reveal system — `x` key toggles masked values across any resource type, not just Secrets Manager
- Horizontal scroll wrap toggle — `w` key switches between truncate and wrap modes in list view

### Fixed
- Horizontal scroll offset preserved when toggling wrap mode

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

## [3.19.0] - 2026-03-27

### Added
- Human-readable timestamps across all views — dates shown as "2 hours ago", "3 days ago" instead of raw ISO 8601 (#57)

### Fixed
- Demo fixture timestamps aligned with production format (#57)

## [3.18.0] - 2026-03-27

### Changed
- Split `views.yaml` into per-resource YAML files under `.a9s/views/` for cleaner config management
- Added `cmd/viewsgen/` tool to auto-generate view config files from built-in defaults
- Bumped GitHub Actions versions (#91, #92, #93, #94, #95)

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

[Unreleased]: https://github.com/k2m30/a9s/compare/v3.32.0...HEAD
[3.32.0]: https://github.com/k2m30/a9s/compare/v3.31.0...v3.32.0
[3.31.0]: https://github.com/k2m30/a9s/compare/v3.29.0...v3.31.0
[3.29.0]: https://github.com/k2m30/a9s/compare/v3.28.0...v3.29.0
[3.28.0]: https://github.com/k2m30/a9s/compare/v3.27.0...v3.28.0
[3.27.0]: https://github.com/k2m30/a9s/compare/v3.26.0...v3.27.0
[3.26.0]: https://github.com/k2m30/a9s/compare/v3.25.0...v3.26.0
[3.25.0]: https://github.com/k2m30/a9s/compare/v3.24.0...v3.25.0
[3.24.0]: https://github.com/k2m30/a9s/compare/v3.23.0...v3.24.0
[3.23.0]: https://github.com/k2m30/a9s/compare/v3.22.0...v3.23.0
[3.22.0]: https://github.com/k2m30/a9s/compare/v3.21.0...v3.22.0
[3.21.0]: https://github.com/k2m30/a9s/compare/v3.20.0...v3.21.0
[3.20.0]: https://github.com/k2m30/a9s/compare/v3.19.0...v3.20.0
[3.19.0]: https://github.com/k2m30/a9s/compare/v3.18.0...v3.19.0
[3.18.0]: https://github.com/k2m30/a9s/compare/v3.17.0...v3.18.0
[3.17.0]: https://github.com/k2m30/a9s/compare/v3.16.0...v3.17.0
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
