# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [3.34.0] - 2026-04-10

### Changed
- Demo mode (`--demo`) fully migrated from legacy HTTP transport to per-service typed fakes. All 66 resource types now served by 42 typed fakes in `internal/demo/fakes/` backed by SDK-typed fixture data in `internal/demo/fixtures/`. Deleted the legacy fixture stores (`demoData`, `childDemoData`, `RegisterRelatedDemo`), all `fixtures_*.go` category files, all `handlers_*.go` transport handlers, and the parallel demo-related registry. Only the STS handler remains for `GetCallerIdentity` probes.
- IAM Policies list now includes inline group policies alongside managed policies. Replaced "Policy ID" column with "Type" (managed/inline). Related panel for inline policies shows the parent group.
- CloudTrail navigable fields fixed: AWSService events no longer pollute `Fields["user"]` and `Fields["role_name"]` with service principals; AssumedRole events no longer copy role names into the user navigation field.
- Demo main menu now shows resource counts on startup (availability probes run in noCache mode).
- Main menu command hint shows first alias (e.g., `:event`) instead of canonical short name (`:ct-events`).
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

[Unreleased]: https://github.com/k2m30/a9s/compare/v3.34.0...HEAD
[3.34.0]: https://github.com/k2m30/a9s/compare/v3.33.0...v3.34.0
[3.33.0]: https://github.com/k2m30/a9s/compare/v3.32.3...v3.33.0
[3.32.3]: https://github.com/k2m30/a9s/compare/v3.32.2...v3.32.3
[3.32.2]: https://github.com/k2m30/a9s/compare/v3.32.1...v3.32.2
[3.32.1]: https://github.com/k2m30/a9s/compare/v3.32.0...v3.32.1
[3.32.0]: https://github.com/k2m30/a9s/releases/tag/v3.32.0
