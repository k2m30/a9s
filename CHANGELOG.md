# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [3.45.0] - 2026-05-12

### Changed (BREAKING — resource type renames, no aliases)

- **`rds-snap` → `dbi-snap`** — the RDS DBSnapshot resource type was
  renamed to match its parent shortName (`dbi`). The shortName is the
  primary identifier for views, themes, scripts, etc; users who scripted
  against the old name must update. No backward-compat alias is provided.
- **`docdb-snap` → `dbc-snap`** — the cluster-snapshot resource type was
  renamed to match its parent shortName (`dbc`). This resource type covers
  BOTH DocumentDB cluster snapshots AND Aurora cluster snapshots — they
  share the AWS API (`DescribeDBClusterSnapshots`). Display name is now
  `DB Cluster Snapshots`. No backward-compat alias.
- **Display names** — `RDS Snapshots` → `DB Instance Snapshots`,
  `DocDB Snapshots` → `DB Cluster Snapshots`. Aligns with parent display
  names (`DB Instances` / `DB Clusters`).

### Added

- **Generic `SnapshotCrossRef` enricher helper**
  (`internal/aws/snapshot_cross_ref.go`) parameterized by parent shortName,
  parent-ID extractor, parent-retention extractor, and a retention-rule
  flag. Covers the orphan + past-retention pattern shared across snapshot
  types. Both `dbi-snap` and `dbc-snap` enrichers are now thin
  configuration wrappers (~60 lines each); the future `ebs-snap` consumer
  can disable the retention rule (`ec2.Volume` has no
  `BackupRetentionPeriod`).
- **`dbc-snap` cross-ref enricher activated** — was `NoOpIssueEnricher`.
  Orphan and past-retention signals now fire for both DocumentDB and
  Aurora cluster snapshots whose parent cluster is missing or past its
  declared retention period.
- **`dbc-snap` drill-through coverage** —
  `tests/integration/scenario_related_drill_through_test.go` now includes
  Aurora and DocumentDB graph-roots; the `dbc-snap → backup` pivot was
  rewritten to use cache scan + plan-IDs (was returning recovery-point
  ARNs that didn't drill).

### Removed (no aliases)

- The `rds-snap → dbc` (now `dbi-snap → dbc`) related-pivot registration
  and its checker are gone. Aurora cluster snapshots are not stored as
  `DBSnapshot`s in real AWS (`CreateDBSnapshot` is rejected on Aurora
  cluster members), so the pivot had no realistic non-zero case. Aurora
  cluster snapshots live in `dbc-snap`.
- The bogus `ProdRDSSnapAuroraID` / `ARN` demo fixture and its associated
  backup recovery points. CodeRabbit flagged this in a prior review; we
  waved it away with a "legal at SDK type level" comment. The fixture
  shape is rejected by real AWS — drop it. The `dbi-snap` graph-root is
  now `ProdDBISnapID`.

### Fixed

- **Retention threshold reconciled to 1.0×** for both `dbi-snap` and
  `dbc-snap`. The previous `dbc-snap` 1.5× multiplier was authoring drift —
  `BackupRetentionPeriod` IS the operator's declared retention policy; any
  snapshot kept past it is policy drift regardless of engine. The N4
  "intentional divergence" note in the spec doc has been deleted.
- **`dbc-snap → backup` pivot** now resolves plan IDs via cache scan
  instead of recovery-point ARNs via API call. Drilling into the pivot
  lands on a non-empty backup-plan list (was empty).
- **`dbc-snap` fetcher now populates `Resource.Issues` and computes the
  §4 phrase via `ComputeDBCSnapStatusAndIssues`** — mirroring the
  `dbi-snap` pattern. Adds `failed`, `incompatible-*` (Broken),
  `creating` (Warning), and `manual age > 365d` (Warning) signals.
  Previously the fetcher set `Status` to a raw AWS keyword passthrough
  and left `Issues=nil`, breaking universal rule U7f and the detail-view
  Attention section.
- **`computeMergedStatus` reverted to honor the fetcher's §4 phrase** —
  removed a defensive gate added in ab1d7c1 that masked the missing-Issues
  bug above by silently letting cross-ref phrases override
  fetcher-emitted Broken phrases (a `failed + orphan` row would render
  as just `orphan: source cluster deleted`, losing the Broken signal).
  The fetcher contract (Issues populated for every active Wave-1 phrase)
  is now load-bearing; the helper trusts it.
- **`SnapshotCrossRefConfig.OrphanRowLabel` renamed to `ParentRowLabel`** —
  the field is used both for the orphan citation row AND the
  past-retention parent-cite row; the original name implied
  orphan-specific use.
- **dbc-snap fetcher now calls both DocDB and RDS SDKs and merges results** —
  reverses an earlier wrong claim that the DocDB-side
  `DescribeDBClusterSnapshots` returned Aurora rows via a "shared backend".
  AWS SDK docstrings are explicit: docdb-side is DocDB-scoped
  (docdb@v1.48.12/api_op_DescribeDBClusterSnapshots.go:14); rds-side
  covers Aurora + Multi-AZ
  (rds@v1.116.3/api_op_DescribeDBClusterSnapshots.go:19-25). Without the
  merge, every Aurora cluster snapshot would (a) be invisible to a9s
  entirely, or (b) once visible via demo fixtures only, register as orphan
  in real accounts because the dbc parent cache was DocDB-only too. Same
  merge applied to the dbc parent fetcher. `dbc_snap_issue_enrichment.go`
  and `dbc_snap_related.go` regain the `rdstypes.DBClusterSnapshot`
  extractor branches that were wrongly deleted in commit a07331e.
- **`checkDbcSnapBackup` truncation handling** — when the dbc cache is
  truncated AND the parent ARN cannot be resolved from the visible
  window, the checker now returns `UnknownRelated("backup")` instead
  of `Count: 0`. Previously the pivot lied with a definitive zero
  for snapshots whose parent cluster fell past page 1 of `dbc`.
- **`dbc → subnet` and `dbc → vpc` pivots now resolve correctly for Aurora
  clusters** — the subnet-group lookup now dispatches by RawStruct shape:
  Aurora rows (`rdstypes.DBCluster`) call `c.RDS.DescribeDBSubnetGroups`,
  DocDB rows (`docdbtypes.DBCluster`) call `c.DocDB.DescribeDBSubnetGroups`.
  Previously the DocDB-only call returned empty for Aurora rows.

### Spec

- `docs/resources/dbc-snap.md` §3.1 + §4 now list the `incompatible-*`
  Broken signal (defensive parity with the documented `DBSnapshot`
  status family).

### Internal (Phase-05 architecture refactor)

- **`internal/runtime` package** — platform-agnostic app core (`Core`,
  handlers, probes, fetchers, orchestrator) extracted from `internal/tui`.
  `tui.Model` shrunk from ~1200 LOC to ~420 LOC; all behavior preserved.
- **`session.Session` owns all session-scoped mutable state** — resource
  cache, related cache, enrichment findings, generation counters, identity.
  Accessed via `m.core.Session()`; `Rotate()` invalidates in-flight gens
  on profile/region switch. Replaces the former `sessionRuntime` embed.
- **`domain.Gen` unified generation counters** — `AvailabilityGen`,
  `EnrichmentGen`, `ConnectGen` are now typed `domain.Gen` values with
  shared `IsStale` / `Stamp` helpers (AS-73).
- **Typed Cmd/Event message taxonomy** — `runtime/messages/cmd.go` (user
  intent) and `event.go` (async results) with `GenStamped` embed for
  compile-time gen-guard guarantees (AS-74).
- **`KindFetchMore` carries token via `FetchMorePayload`** — pagination
  token bundled with type + gen to eliminate token-mismatch races (AS-270).
- **Wave-2 enrichers migrated to Findings API** — database enrichers no
  longer write `FieldUpdates["status"]`; they emit `Findings` with
  `Source="wave2"`. The status column applies a two-layer priority rule:
  Wave-1 issue phrases first, Wave-2 findings appended only when no Wave-1
  issue is present (AS-140).
- **Glyphs only on `ColorHealthy` rows** — `"! "` and `"~ "` decorators
  are applied only when `ResolveColor(r) == ColorHealthy`; non-Healthy
  rows are colored instead.
- **S3 cross-region related-defs soft-truncate to 0+** — out-of-region
  buckets show empty count instead of error flash (AS-489).
- **Related-panel literal `(N)` count suffix** — avoids ANSI-width
  miscalculation when Lipgloss measures badge glyphs (AS-378).
- **`verify-readonly` Makefile grep widened** to cover `internal/runtime/`
  (AS-236).
- **DBC/DBC-snap dual-API dedup** — rows appearing in both standard and
  Aurora-specific API responses are deduplicated (AS-145).

## [3.44.0] - 2026-04-25

### Changed

- **Partial-success contract end-to-end** — paginated fetchers, the synchronous demo prefetch, the per-type availability probe, and Wave-2 enrichers all now preserve partial results when an error accompanies them. `ResourcesLoadedMsg`, `AvailabilityCheckedMsg`, and `EnrichmentCheckedMsg` carry the partial slice + composite error simultaneously; the app surfaces the error via `FlashMsg` while still applying the partial state. Previously a single per-item failure inside any of these layers blanked the menu badge or list view for that resource type.
- **Never-silent-skip rule** — every AWS SDK call across the codebase now wraps in `RetryOnThrottle` and surfaces per-item failures through the operator-visible error channel. Previously, ~50 files silently swallowed per-item describe/get errors, so a resource list could look complete while permission or throttling issues silently degraded the view. Operators now see aggregated composite errors via the `!` error log.
- New shared helper `internal/aws/partial_errors.go` (`AggregateFailures`, `AggregateMissing`) standardizes composite-error formatting across fetchers, related checkers, and Wave-2 enrichers.
- Wave-2 enrichers now set `Truncated`/`TruncatedIDs` (per-row `?` marker) AND return an aggregated top-level error (feeds `EnrichmentCheckedMsg.Err` → FlashMsg) — both channels fire on per-item failure.
- Related-panel checkers (`*_related.go`) set `RelatedCheckResult.Err` to the aggregated composite; the app layer converts it to `FlashMsg{IsError:true}` so the error log captures the reason the pivot shows `?`.
- Top-level fetchers (`backup.go`, `dbc.go`, `ddb.go`, `efs.go`, `eks.go`, `kms.go`, `ng.go`, `opensearch.go`, `redis.go`, `redshift.go`, `s3.go`, `sg.go`, `sqs.go`, plus the lazy-add fetchers) aggregate per-item describe failures; `ResourcesLoadedMsg.Err` already surfaces as a flash.
- ECS-task fetcher decouples join-failure signaling from `Pagination.IsTruncated` — per-task `Fields["task_def_join_error"]="true"` marks affected tasks; `checkEFSECSTask` reports `Approximate=true` without the fetcher advertising a bogus "m: load more" footer.
- `a9s-implement-resource` skill: new **Error handling and throttle rules** section (E1–E6) with banned-pattern list, category/surface-channel table, and enforcement hooks in phases 5, 7.5, 8, 9. Coverage matrix gains U12 (partial-failure FlashMsg) and U13 (throttle-wrap static audit).
- Lazy-add cache snapshot now marks lazy-only entries as `IsTruncated:true` (sparse, not authoritative). The related-panel prefetch decision uses a `mainCacheKeys` set built from `resourceCache` only — a lazy-only entry no longer suppresses the real first-page fetch when a `NeedsTargetCache:true` checker scans it.
- IAM policy `FetchIAMPoliciesByIDsFull` builds also index entries by ARN alongside `PolicyName` so checkers that emit ARN form (e.g. role → managed policy) drill into the cached row.

### Added

- `tui` package split into smaller files to keep each under the 500-line file-size budget: `app_probes.go` (availability + enrichment probes), `app_handlers_availability.go` (their handlers + `unifiedIssueCount`), `app_handlers_related_navigate.go` (`handleRelatedNavigate` and child variant), `related_cache.go` (LRU + replay helpers).
- `EnrichmentCheckedMsg.Err` (existing) and new `ResourcesLoadedMsg.Err` carry partial-success errors alongside resources.
- Lazy-only fast path in related navigation now requires ALL requested IDs to short-circuit (`len(filtered) == len(result.RelatedIDs)`); partial coverage falls through to a full fetch instead of rendering only the lazily-cached subset.
- Lazy-add path for related-panel drill-through: when a checker emits IDs outside the top-level fetcher's scope filter (KMS customer-managed, AMI `Owners=self`, EBS snapshot `OwnerIds=self`, IAM Policy `Scope=Local`), the orchestrator calls `FetchByIDs` to resolve them and populates `lazyResourceCache`. Drilling into a KMS pivot for an `aws/rds` key, an AMI pivot for a public marketplace AMI, or an IAM role's `AdministratorAccess` now lands on a real entry instead of an empty list.
- `lazyResourceCache` session map (separate from `resourceCache`) consulted only by related-navigation, never by main-menu top-level list. Prevents lazy-added out-of-scope entries from polluting the scope-filtered top-level view.
- `ResetIAMPoliciesCache()` exported and wired into `sessionRuntime.resetForSessionSwitch` — IAM policy memoization now clears on profile/region switch so account-A policies never leak into account-B drills.
- `RelatedCheckResultMsg.LazyAddError` field routes FetchByIDs failures into FlashMsg without masking partial successes.
- `AvailabilityPrefetchedMsg.PrefetchErr` surfaces per-type failures from the synchronous availability prefetch in no-cache / demo mode.
- User-story set `tests/stories/lazy_add.md` — 44 given/when/then stories across 9 sections covering cross-scope drill-through, session lifecycle, idempotence, race/timing, and size extremes.

### Fixed

- `probeEnrichment` no longer drops the `IssueEnricherResult` when the enricher returns a composite error alongside partial findings — Wave-2 menu badges, FieldUpdates, and per-row `?` markers now survive across per-item failures.
- `probeResourceAvailability` no longer drops the partial resource list when the fetcher returns a composite error — the menu count + `Issues` badge update from the partial slice and a FlashMsg surfaces the error.
- `handleEnrichmentChecked` and `handleAvailabilityChecked` now apply state changes when the message carries partial success (Err+Resources/Findings populated). Pure failure (Err alone) still leaves the menu entry as "unknown".
- `FetchAMIsByIDs` now sets `IncludeDeprecated: true` to match the single-ID `FetchAMIByID` path; deprecated AMIs referenced from EC2 / ASG / EKS pivots no longer silently vanish from batch drills.
- `efs_issue_enrichment` uses the typed `efstypes.LifeCycleStateAvailable` constant instead of the string literal `"available"` for safer future SDK upgrades.
- `EnrichEC2InstanceStatus` no longer panics when called with a nil `*ServiceClients` — `probeEnrichment` re-instates the nil-clients guard and returns a clear "AWS clients not initialized" error.
- `handleEnrichmentChecked` no longer silently swallows `EnrichmentCheckedMsg.Err` — failures now emit `FlashMsg{IsError:true}` so the error log (`!` key) captures them.
- `handleRelatedNavigate` cache-hit path now consults `lazyResourceCache` alongside `resourceCache`, so a drill through a cache-hit pivot finds lazy-added out-of-scope targets.
- ENI list no longer renders `m: load more` on a fully-resolved exact-ID filter — `handleRelatedNavigate` strips `IsTruncated` on the pagination passed to the list view when every `RelatedIDs` entry matched.
- `FetchKMSKeysPage` `DescribeKey` and `ListAliases` per-item failures now aggregate into a composite returned error instead of silently skipping keys or stopping the alias-page loop.
- `FetchDynamoDBTablesPage` per-table `DescribeTable` failures aggregate into a composite error (previously: silent `continue`).
- `sfnDescribe` signature returns `(out, err)` — `checkSFNRole`, `checkSFNKMS`, `checkSFNLambda`, and `ecs-svc` cross-references now set `Result.Err` on API failure instead of collapsing to bare `Count=-1`.
- `redshiftLoggingStatus` signature returns `(status, err)` — `checkRedshiftLogs` and `checkRedshiftS3` surface the underlying error via `Result.Err`.
- `checkEbTG` per-LB `DescribeLoadBalancers`/`DescribeListeners` failures aggregate into `Result.Err` (previously silent `continue`, undercounting TG relationships).
- `secrets-related: DescribeTaskDefinition` treats ECS `ClientException` ("task definition does not exist") as definitive absence rather than a real error — mirrors the `ecs_task.go` carve-out so demo fixtures and real environments with soft-deleted task definitions don't spuriously flash.
- Enricher tests pinning the old `err == nil` silent-skip contract updated to accept the new aggregated-error contract across ASG, CFN, ECR, ECS-svc, EFS, Logs, MSK, R53, SFN, SQS, TG, TGW, WAF, plus EKS top-level fetcher.

## [3.43.0] - 2026-04-24

### Changed

- Redis, DynamoDB, DBI (RDS instance), DBC (RDS cluster), S3, SES, and Backup resources re-implemented from spec (#295, #296) — spec-compliant §4 phrases, Wave-2 findings, graph-root pivot accuracy
- Detail view: unified `Attention (N)` section replaces per-type issue sections (Pending Maintenance, Target Health, Latest Build, etc.) — one consistent format for every resource type
- Redis fetcher now lists `ReplicationGroup` (was `CacheCluster`); engine filter excludes Valkey/Memcached
- Backup enricher surfaces Wave-2 job-state findings via the unified Status column instead of a parallel column

### Added

- Shard-level Wave-1 phrases for cluster-mode-enabled Redis (e.g. `shard ng-1: modifying (+2)`)
- Table-driven drill-through integration tests pin every registered related-panel pivot + navigable field across nine graph-root fixtures
- Central `NavIDFromValue` registry normalizes ARNs to bare IDs at navigation time for KMS, IAM role, ECS cluster, CloudWatch Logs, S3, and IAM user
- `ApproximateZero` / `Approximate=true` contract for truncated reverse-scan related checkers (DDB, S3, SES, Redis)
- Backup plan coverage detection honors `arn:aws:<service>:*:*:<type>/*` wildcards, `NotResources` exclusions, and tag-based selections (#296)
- API Gateway REST v1 APIs now appear in the `apigw` list alongside HTTP/WebSocket APIs
- API Gateway detail view resolves ACM certificates attached via custom domain mappings

### Fixed

- Navigable fields in detail view now land on the actual resource — highlighting a KMS ARN, role ARN, bucket name, or ECS cluster ARN and pressing enter previously produced empty landing lists
- SES `DescribeActiveReceiptRuleSet` cache clears on profile/region switch — previously stale Lambda/S3 related-panel counts persisted after `Ctrl+R` or profile switch
- SES related pivots: lambda/eb-rule ID-format mismatch, kinesis type mismatch, Ctrl+R staleness (#295)
- S3 related-panel joins now use boundary-safe resource-ID matching (no more substring collisions)
- Attention entry color capped at row's S2 color bucket (no more green glyphs on red rows)
- Redis color function matches §4 phrases via `StripFindingSuffix` so the Status column paints correctly under multi-finding rows
- EKS clusters whose `DescribeCluster` call fails now surface as a `DescribeFailed` row instead of silently vanishing from the list
- KMS paginated fetcher fully paginates `ListAliases` so customer-managed keys on later alias pages no longer render with a blank alias
- API Gateway enricher emits the documented "no deployed stages" warning when `GetStages` returns empty, matching `docs/attention-signals.md`

## [3.42.0] - 2026-04-14

### Added

- Inter-navigation between detail, JSON, and YAML views — press `d`/`y`/`J` to switch freely between them without going back first (#269)
- Sort by column position with `1`–`0` keys — pressing the same key toggles sort direction (#267)
- Error log view with `!` key — shows session errors with timestamps (#268)

### Fixed

- View-switch keys (`d`/`J`) are blocked in raw-text viewer mode (error log) to prevent navigation with empty resource context (#269)

### Changed

- Architecture guide updated with all recent features: full message catalog, ReplaceCurrent navigation pattern, sorting, error log, resource type categories

## [3.41.0] - 2026-04-13

### Added

- `:root` / `:main` colon command to navigate back to the main menu from any view depth (#258)
- COMMANDS section in all help screens showing available colon commands (`:q`, `:ctx`, `:profile`, `:region`, `:theme`, `:help`, `:root`, `:main`, `:<resource>`) (#258)
- Tab completion for `:root` and `:main` commands (#258)
- Auto-detect and pretty-print JSON in detail view field values — top-level and sub-field JSON strings expand as indented YAML sub-fields (#262)
- Syntax-colored JSON pretty-printing in secret/parameter reveal view (`x` key) with raw copy preserved (#262)
- AWS tags render as flat `Key: Value` pairs in detail views instead of verbose `Key/Value` struct fields; rich tag structs (e.g., ASG with PropagateAtLaunch) are left unflattened (#210)

## [3.40.0] - 2026-04-13

### Fixed

- Detail view nested structures (arrays, maps, sub-objects) now render with proper YAML hierarchy instead of flat 5-space indentation (#265)
- Bare YAML list items like `- '*'` no longer misrendered as key:value pairs in detail view
- Fields-map and RawStruct data sources now produce identical YAML list format for array sections (e.g., SecurityGroups)
- Detail search matches are cursor-position-independent — canonical `": "` spacing on all sub-field types
- EC2 status-check sub-fields store raw values; cursor row no longer leaks ANSI color escapes

### Changed

- `make test` runs without `-race` for fast local iteration (4s); `make test-race` added for pre-push race detection
- Unit test suite optimized: removed real timer waits, replaced full-app journeys with direct model setup, sampled representative resource types in exhaustive sweeps (86.3% coverage maintained)

### Added

- Shared YAML line tokenizer (`yaml_line.go`) ensuring markers and spacing stay identical across cursor states and views
- `"impaired"` and `"initializing"` status colors in row color cache for EC2 status checks

## [3.39.0] - 2026-04-12

### Added

- Auto-create default view config files (`~/.a9s/views/*.yaml`) on first launch — no source checkout needed to customize views
- Auto-create and auto-refresh `~/.a9s/views_reference.yaml` field reference on each launch — always up-to-date with the binary version
- `--reset-views` flag to delete all view configs and regenerate defaults on next launch (with confirmation prompt)
- `--reset-themes` flag to delete all theme files and regenerate defaults on next launch (with confirmation prompt)
- Synthetic child view entries (`lambda_invocations`, `lambda_invocation_logs`, `pipeline_stages`) in `views_reference.yaml`
- View Customization and Color Themes wiki pages

## [3.38.0] - 2026-04-12

### Added

- JSON view for raw resource data (`J` key). Opens from resource list and detail views, complementing the existing YAML view (`y`). Marshals the AWS SDK response struct directly via `json.MarshalIndent` with 2-space indentation, preserving native JSON types (booleans, numbers, nulls) for direct use in AWS CLI `--cli-input-json`, jq, and other tooling. Supports search (`/`), scroll, line wrap (`w`), copy (`c`), and CloudTrail jump (`t`). Help screen (`?`) lists the new binding.

### Fixed

- Lambda invocation logs fetcher now paginates through empty CloudWatch `FilterLogEvents` pages. Previously made a single API call, returning zero results whenever matching events lived in a later log stream page.

## [3.37.0] - 2026-04-12

### Added

- `-c` / `--command` CLI flag to open a resource list directly on startup, skipping the main menu (k9s-style). Accepts any resource short name or alias (e.g. `a9s -c ec2`, `a9s -c events`). Combinable with `-p`, `-r`, `--demo`, `--no-cache`. Resolves via `resource.FindResourceType`; unknown names fail fast with exit 1 before the TUI starts.

### Fixed

- Auto-navigation from `-c` is guarded to startup-only: if the initial AWS connection is slow and the user navigates away from the main menu before `ClientsReadyMsg` arrives, the flag is silently consumed without pushing a view on top of the user's current screen.

## [3.36.1] - 2026-04-11

### Fixed

- `cache.Save` is now crash-atomic and concurrent-safe. Previously wrote directly to the target path with `os.WriteFile`, which truncated the file mid-write — concurrent readers could observe a zero-length cache, and two a9s processes sharing the same `~/.a9s/cache/<profile>--<region>.yaml` raced on a deterministic `.tmp` name and silently lost updates. Save now uses `os.CreateTemp` for a unique per-invocation temp file, followed by `os.Rename`.
- `cache.Path` now sanitizes backslash in addition to `/` and space. On Windows, profile names containing `\` could previously create unintended subdirectories.
- App context cancellation now fires reliably on exit. `defer model.Cancel()` was added around `tea.Program.Run` via a `main` → `runProgram` refactor, so in-flight AWS fetcher goroutines are cancelled on normal return, error path, and panic — previously only normal `QuitMsg` dispatched the cancel.
- `ClientsReadyMsg` with an unexpected `Clients` type now surfaces an internal error via `APIErrorMsg` instead of silently falling back. The demo-mode `nil` path still correctly routes to pre-supplied clients.

## [3.36.0] - 2026-04-10

### Added

- Configurable color themes: 11 built-in themes (Tokyo Night Dark/Light, Catppuccin Mocha/Latte, Dracula, Nord/Nord Light, Gruvbox Dark/Light, Solarized Dark/Light) shipped as embedded YAML files, auto-extracted to `~/.a9s/themes/` on first run.
- `:theme` command opens a selector overlay (same UX as `:region`) for runtime theme switching with immediate re-render and config persistence.
- `~/.a9s/config.yaml` configuration file for app-level settings, starting with `theme:` key.
- Custom themes: copy any built-in YAML file, edit colors, point config at it. Partial themes inherit missing colors from the Tokyo Night Dark default.
- Active theme name displayed in help view (`?`).
- Path traversal protection on theme filename validation (rejects `../`, absolute paths, path separators).

### Changed

- Extracted 33 hardcoded Tokyo Night Dark palette colors into a `Theme` struct with `DefaultTheme()`, `ApplyTheme()`, `ThemeFromYAML()`, and `ActiveTheme()` API.
- Migrated 13 package-level style captures from 4 view files (help, identity, yaml, search) into centralized `styles.go` composed styles, rebuilt on every `ApplyTheme()` call.
- `NoColorActive()` now correctly returns `true` for `NO_COLOR=` (empty value) per the NO_COLOR spec. Previously required a non-empty value.
- `:theme` command persists before applying — if config save fails, the change is aborted and the user sees an error flash instead of a silent partial state.
- Theme file operations guard against missing config directory (`$HOME` unset, no `$A9S_CONFIG_FOLDER`) — no accidental writes to the working directory.
- `:theme` selector always marks the active theme as "(current)", including the default Tokyo Night Dark on first run with no config.

## [3.35.0] - 2026-04-10

### Added

- CloudTrail Events related view extended to all 66 resource types. Every resource now shows "CloudTrail Events" in the right-column related panel with correct per-service filters.
- `t` keyboard shortcut for direct CloudTrail Events navigation from resource list, detail, and YAML views.
- Deterministic `CloudTrailKey` per-type configuration on `ResourceTypeDef` — no heuristics, no reflection. Each resource type explicitly declares its CloudTrail lookup attribute and value source.
- ARN-based CloudTrail filters for Lambda, RDS, EKS, Secrets Manager, DocumentDB, and SQS (types where CloudTrail indexes by ARN, not name).
- SQS resources now expose `Fields["arn"]` extracted from queue attributes.
- 9 new demo CloudTrail fixture events covering Lambda, RDS, ECS, DynamoDB, Secrets Manager, EKS, and CloudFormation.
- Demo CloudTrail fake now supports suffix matching on ResourceName (bare name matches ARN-prefixed events).
- `t` / `cloudtrail` added to help overlay (`?`) for resource list, detail, and YAML views.
- `TestFullRelatedViewValidation` comprehensive integration test: validates all related-resource entries and navigable fields across all resource types in both demo and live AWS modes (1,133 demo subtests, 610 live subtests).
- 15 live integration scenario tests for CloudTrail `t` key against real AWS.
- 40+ new unit tests covering filter logic, key handlers, hints, help overlay, coverage gaps.

### Changed

- File splits: `resourcelist.go`, `detail.go`, `app_handlers.go`, `help.go` each split into two files to stay under 500 lines per the codebase checklist.
- YAML view (`NewYAML`) now accepts `resourceType` parameter for child-type detection.
- `t` hint and key suppressed on ct-events lists (no self-reference), child resource lists, child detail/YAML views, and types with no CloudTrail support.
- Pre-push rules now require `TestFullRelatedViewValidation` against a real AWS profile before any push.
- Test count updated from 4,400+ to 4,500+ in README and website.
- `website/content/resources.md`: CloudTrail Events short name corrected to `ct-events`.

### Removed

- Reflection-based ARN extraction (`extractARNFromRawStruct`) replaced by deterministic `CloudTrailKey` config.

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

[Unreleased]: https://github.com/k2m30/a9s/compare/v3.39.0...HEAD
[3.39.0]: https://github.com/k2m30/a9s/compare/v3.38.0...v3.39.0
[3.38.0]: https://github.com/k2m30/a9s/compare/v3.37.0...v3.38.0
[3.37.0]: https://github.com/k2m30/a9s/compare/v3.36.1...v3.37.0
[3.36.1]: https://github.com/k2m30/a9s/compare/v3.36.0...v3.36.1
[3.36.0]: https://github.com/k2m30/a9s/compare/v3.35.0...v3.36.0
[3.35.0]: https://github.com/k2m30/a9s/compare/v3.34.0...v3.35.0
[3.34.0]: https://github.com/k2m30/a9s/compare/v3.33.0...v3.34.0
[3.33.0]: https://github.com/k2m30/a9s/compare/v3.32.3...v3.33.0
[3.32.3]: https://github.com/k2m30/a9s/compare/v3.32.2...v3.32.3
[3.32.2]: https://github.com/k2m30/a9s/compare/v3.32.1...v3.32.2
[3.32.1]: https://github.com/k2m30/a9s/compare/v3.32.0...v3.32.1
[3.32.0]: https://github.com/k2m30/a9s/releases/tag/v3.32.0
