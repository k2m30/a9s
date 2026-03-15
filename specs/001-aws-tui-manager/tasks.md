# Tasks: a9s — Terminal UI AWS Resource Manager

**Input**: Design documents from `/specs/001-aws-tui-manager/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Included — constitution mandates TDD (Principle I). Tests MUST be written and FAIL before implementation.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Entry point**: `cmd/a9s/main.go`
- **Internal packages**: `internal/app/`, `internal/ui/`, `internal/views/`, `internal/aws/`, `internal/resource/`, `internal/navigation/`
- **Tests**: `tests/unit/`, `tests/integration/`, `tests/testdata/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [x] T001 Initialize Go module (go mod init), add all dependencies to go.mod: bubbletea v2, bubbles v2, lipgloss v2, evertras/bubble-table, aws-sdk-go-v2 (config + 7 service packages), gopkg.in/ini.v1, atotto/clipboard
- [x] T002 Create project directory structure per plan.md: cmd/a9s/, internal/app/, internal/ui/, internal/views/, internal/aws/, internal/resource/, internal/navigation/, tests/unit/, tests/integration/, tests/testdata/
- [x] T003 [P] Create Makefile with targets: build, test, lint, fmt, run, clean
- [x] T004 [P] Configure golangci-lint in .golangci.yml with default linters (govet, errcheck, staticcheck, gosimple, unused)
- [x] T005 [P] Create test fixtures in tests/testdata/: aws_config_sample (3 profiles: default, dev, prod-sso) and aws_credentials_sample (2 profiles: default, dev)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core framework, shared types, and AWS client layer that MUST be complete before ANY user story

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T006 Define shared message types (ResourcesLoadedMsg, APIErrorMsg, ProfileSwitchedMsg, RegionSwitchedMsg, StatusMsg) in internal/app/messages.go
- [x] T007 [P] Define global KeyMap with all keybindings from contracts/commands.md in internal/app/keys.go
- [x] T008 [P] Define Lip Gloss styles (header, breadcrumbs, table cursor, status bar, error, spinner colors per contracts/ui-layout.md color scheme) in internal/app/styles.go
- [x] T009 Define Resource interface (ID, Name, Status, Fields, RawJSON, DetailData) and generic Resource struct in internal/resource/resource.go
- [x] T010 [P] Define per-resource-type column configurations (7 types: S3, EC2, RDS, Redis, DocumentDB, EKS, Secrets with columns from data-model.md) in internal/resource/types.go
- [x] T011 Define AWS service interfaces for mocking (one interface per service with methods used: EC2DescribeInstancesAPI, S3ListBucketsAPI, S3ListObjectsV2API, S3GetBucketLocationAPI, RDSDescribeDBInstancesAPI, ElastiCacheDescribeCacheClustersAPI, DocDBDescribeDBClustersAPI, EKSListClustersAPI, EKSDescribeClusterAPI, SecretsManagerListSecretsAPI, SecretsManagerGetSecretValueAPI) in internal/aws/interfaces.go

### Foundational Tests (TDD: write FIRST, verify FAIL, then implement)

- [x] T012 [P] Test AWS error classification: given smithy.APIError with codes ExpiredToken, AccessDenied, Throttling, and unknown, verify classifyAWSError returns correct (code, message, retryable) tuples in tests/unit/aws_errors_test.go
- [x] T013 [P] Test NavigationStack: push 3 states, pop returns most recent, forward after pop returns popped state, clear empties stack, CanGoBack/CanGoForward report correctly in tests/unit/navigation_test.go
- [x] T014 [P] Test AWS client factory: NewAWSSession with profile "dev" and region "eu-west-1" returns config with correct profile/region; verify CreateServiceClients returns non-nil clients in tests/unit/aws_client_test.go

### Foundational Implementation

- [x] T015 Implement AWS client factory: NewAWSSession(profile, region) returning aws.Config, CreateServiceClients(cfg) returning all 7 service clients in internal/aws/client.go
- [x] T016 [P] Implement AWS error classification: classifyAWSError(err) returning (code, message, retryable) for ExpiredToken, AccessDenied, Throttling, and unknown errors in internal/aws/errors.go
- [x] T017 Implement NavigationStack with Push, Pop, Forward, Clear, CanGoBack, CanGoForward methods and ViewState struct in internal/navigation/history.go
- [x] T018 Implement root model skeleton: AppState struct (per data-model.md), Init() loads AWS config and returns MainMenu, Update() delegates by currentView, View() composes header+breadcrumbs+content+statusbar via lipgloss.JoinVertical in internal/app/app.go
- [x] T019 Create entry point: parse --profile/-p, --region/-r, --version/-v flags, create root model, run tea.NewProgram with AltScreen in cmd/a9s/main.go

**Checkpoint**: Foundation ready — user story implementation can now begin

---

## Phase 3: User Story 1 - Launch and Browse Resource Types (Priority: P1) MVP

**Goal**: User launches app, sees resource types, navigates to a resource list via Enter or :command, returns with Escape/:main

**Independent Test**: Launch with valid AWS config → main screen shows 7 resource types → select EC2 → table appears → Escape → back to main

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T020 [P] [US1] Test root model Init returns MainMenu view with 7 resource types, header shows default profile and region in tests/unit/app_test.go
- [x] T021 [P] [US1] Test EC2 response parsing: given DescribeInstances output with 3 instances, verify Resource structs have correct ID, Name (from tags), State, Type, IPs, LaunchTime in tests/unit/aws_ec2_test.go
- [x] T022 [P] [US1] Test command routing: `:ec2` sets currentView to ResourceList with EC2 type, `:main` returns to MainMenu, `:q` returns tea.Quit in tests/unit/app_test.go

### Implementation for User Story 1

- [x] T023 [US1] Implement header component: render app name/version, profile, region, loading spinner using lipgloss styles in internal/ui/header.go
- [x] T024 [P] [US1] Implement breadcrumbs component: render path segments joined by " > " using lipgloss styles in internal/ui/breadcrumbs.go
- [x] T025 [P] [US1] Implement status bar component: render mode-dependent content (hints, command input, filter text, error, loading) in internal/ui/statusbar.go
- [x] T026 [US1] Implement main menu view: bubbles/list showing 7 resource types with names and icons, j/k navigation, Enter to select in internal/views/mainmenu.go
- [x] T027 [US1] Implement generic resource list view: evertras/bubble-table with dynamic columns from ResourceType config, j/k navigation, row count display in internal/views/resourcelist.go
- [x] T028 [US1] Implement EC2 fetcher: FetchEC2Instances(ctx, api) using DescribeInstancesPaginator, parse to []Resource with columns from data-model.md in internal/aws/ec2.go
- [x] T029 [US1] Implement command input mode: `:` activates textinput with inline autocomplete showing best matching command as dimmed text, Enter executes, Escape cancels in internal/ui/command.go
- [x] T030 [US1] Wire command routing in root model Update: dispatch `:ec2` → fetch + show ResourceList, `:main`/`:root` → MainMenu, `:q`/`:quit` → tea.Quit, unknown → error status in internal/app/app.go
- [x] T031 [US1] Integration test: teatest launch → verify main menu output → send `:ec2` → verify table headers → send Escape → verify main menu in tests/integration/tui_test.go

**Checkpoint**: User Story 1 fully functional — app launches, shows resource types, navigates to EC2 list, returns to main

---

## Phase 4: User Story 2 - AWS Profile and Region Switching (Priority: P2)

**Goal**: User switches AWS profiles via :ctx and regions via :region, header updates, subsequent views use new credentials

**Independent Test**: Launch → :ctx → see profiles → select different profile → header updates → :ec2 → resources from new profile

### Tests for User Story 2

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T032 [P] [US2] Test profile enumeration: given sample config with 3 profiles (default, dev, prod-sso) and credentials with 2, verify merged unique list of profile names in tests/unit/aws_profile_test.go
- [x] T033 [P] [US2] Test ProfileSwitchedMsg updates AppState.ActiveProfile and header re-renders with new name in tests/unit/app_test.go

### Implementation for User Story 2

- [x] T034 [US2] Implement profile enumeration: ListProfiles() parsing ~/.aws/config (profile sections) and ~/.aws/credentials (bare sections) using ini.v1 in internal/aws/profile.go
- [x] T035 [P] [US2] Implement hardcoded region list (all current AWS regions with codes and display names) and GetDefaultRegion(profile) in internal/aws/regions.go
- [x] T036 [US2] Implement profile selector view: bubbles/list showing profiles, current highlighted, Enter selects and triggers ProfileSwitchedMsg in internal/views/profile.go
- [x] T037 [US2] Implement region selector view: bubbles/list showing regions, current highlighted, Enter selects and triggers RegionSwitchedMsg in internal/views/region.go
- [x] T038 [US2] Wire :ctx and :region commands in root model: on ProfileSwitchedMsg/RegionSwitchedMsg, recreate AWS clients, update header, navigate to MainMenu/previous view in internal/app/app.go

**Checkpoint**: User Story 2 functional — :ctx shows profiles, :region shows regions, switching updates header and credentials

---

## Phase 5: User Story 3 - Resource Detail and Interaction (Priority: P3)

**Goal**: User presses d for describe, x for reveal (secrets), c for copy, Enter drills into S3 buckets, y for JSON view

**Independent Test**: Navigate to EC2 list → press d → see all attributes → Escape → press c → ID copied → :s3 → Enter bucket → see objects

### Tests for User Story 3

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T039 [P] [US3] Test detail view renders key-value pairs: given Resource with 10 DetailData entries, verify viewport contains all keys and values in tests/unit/detail_test.go
- [x] T040 [P] [US3] Test JSON view renders formatted JSON from Resource.RawJSON in tests/unit/jsonview_test.go
- [x] T041 [P] [US3] Test S3 bucket listing with region: given ListBuckets output with 3 buckets + GetBucketLocation responses, verify Resource structs with Name, Region, CreationDate in tests/unit/aws_s3_test.go
- [x] T042 [P] [US3] Test S3 object listing with prefix/delimiter: given ListObjectsV2 output with 2 CommonPrefixes and 3 Contents, verify folder rows and file rows in tests/unit/aws_s3_test.go
- [x] T043 [P] [US3] Test Secrets Manager GetSecretValue: given secret name, verify SecretString returned as plain text in tests/unit/aws_secrets_test.go

### Implementation for User Story 3

- [x] T044 [US3] Implement describe view: scrollable viewport rendering key-value pairs from Resource.DetailData with lipgloss styling, j/k scroll, Escape back in internal/views/detail.go
- [x] T045 [P] [US3] Implement JSON view: scrollable viewport rendering Resource.RawJSON with indentation, j/k scroll, Escape back in internal/views/jsonview.go
- [x] T046 [P] [US3] Implement reveal view: scrollable viewport rendering plain text secret value, j/k scroll, Escape back in internal/views/reveal.go
- [x] T047 [US3] Implement S3 fetcher: FetchS3Buckets (ListBucketsPaginator + GetBucketLocation per bucket for Region column) and FetchS3Objects(bucket, prefix) using ListObjectsV2Paginator with "/" delimiter, page size 100 in internal/aws/s3.go
- [x] T048 [US3] Implement Secrets Manager fetcher: FetchSecrets (ListSecretsPaginator) and RevealSecret(name) using GetSecretValue in internal/aws/secrets.go
- [x] T049 [US3] Wire d/y/x/c keybindings in root model: d → DetailView, y → JSONView, x → RevealView (secrets only), c → clipboard copy via OSC52 with atotto fallback in internal/app/app.go
- [x] T050 [US3] Wire Enter drill-down for S3: S3 bucket Enter → S3ObjectList, S3 prefix Enter → deeper prefix, Escape → parent prefix or bucket list in internal/app/app.go

**Checkpoint**: Resource details, JSON view, secret reveal, clipboard copy, and S3 browsing all functional

---

## Phase 6: User Story 4 - Filter and Search Resources (Priority: P4)

**Goal**: User presses / to filter, types text, list filters in real time, Escape clears filter

**Independent Test**: Navigate to EC2 list → press / → type "prod" → only matching instances shown → Escape → full list restored

### Tests for User Story 4

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T051 [P] [US4] Test filter matching: case-insensitive substring across all Fields values, verify matching and non-matching rows in tests/unit/filter_test.go
- [x] T052 [P] [US4] Test filter state preservation: apply filter, drill into detail, Escape back, verify filter still active in tests/unit/app_test.go
- [x] T053 [P] [US4] Test empty filter result shows "No resources match filter" message in tests/unit/app_test.go

### Implementation for User Story 4

- [x] T054 [US4] Implement filter mode: / activates text input in status bar, each keystroke triggers evertras/bubble-table SetFilter, Escape clears and exits in internal/views/resourcelist.go
- [x] T055 [US4] Wire filter state in root model: preserve filter across detail drill-in/back, clear on view switch, show match count in status bar in internal/app/app.go

**Checkpoint**: Filter mode functional — real-time filtering across all columns, state preserved across navigation

---

## Phase 7: User Story 5 - All MVP Resource Types (Priority: P5)

**Goal**: All 7 resource types display correctly with type-specific columns and colon commands

**Independent Test**: For each of :s3, :ec2, :rds, :redis, :docdb, :eks, :secrets, verify correct columns and data

### Tests for User Story 5

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T056 [P] [US5] Test RDS response parsing: given DescribeDBInstances output, verify Resource with DBIdentifier, Engine, EngineVersion, Status, Class, Endpoint, MultiAZ in tests/unit/aws_rds_test.go
- [x] T057 [P] [US5] Test ElastiCache Redis response parsing with client-side engine=="redis" filter, verify Memcached clusters excluded in tests/unit/aws_redis_test.go
- [x] T058 [P] [US5] Test DocumentDB response parsing with engine=docdb filter, verify ClusterID, EngineVersion, Status, InstanceCount, Endpoint in tests/unit/aws_docdb_test.go
- [x] T059 [P] [US5] Test EKS two-step fetch: given ListClusters with 2 names + DescribeCluster responses, verify Resources with Name, Version, Status, Endpoint, PlatformVersion in tests/unit/aws_eks_test.go
- [x] T060 [P] [US5] Test Secrets Manager list parsing: verify SecretName, Description, LastAccessed, LastChanged, RotationEnabled columns in tests/unit/aws_secrets_test.go

### Implementation for User Story 5

- [x] T061 [P] [US5] Implement RDS fetcher: FetchRDSInstances using DescribeDBInstancesPaginator, parse to []Resource in internal/aws/rds.go
- [x] T062 [P] [US5] Implement ElastiCache Redis fetcher: FetchRedisClusters using DescribeCacheClustersPaginator with client-side Engine=="redis" filter in internal/aws/redis.go
- [x] T063 [P] [US5] Implement DocumentDB fetcher: FetchDocDBClusters using DescribeDBClustersPaginator with engine=docdb server-side filter in internal/aws/docdb.go
- [x] T064 [P] [US5] Implement EKS fetcher: FetchEKSClusters using ListClustersPaginator + DescribeCluster per name in internal/aws/eks.go
- [x] T065 [US5] Wire all remaining resource commands (:s3, :rds, :redis, :docdb, :eks, :secrets) in root model Update, each triggering async fetch + ResourceList display in internal/app/app.go

**Checkpoint**: All 7 resource types browsable with correct columns — :s3, :ec2, :rds, :redis, :docdb, :eks, :secrets

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [x] T066 [P] Implement help overlay: ? toggles overlay showing all keybindings grouped by category (Global, Navigation, Actions, Filter) from contracts/commands.md in internal/ui/help.go
- [x] T067 [P] Implement column sorting keybindings: Shift-N sorts by name, Shift-S sorts by status, Shift-A sorts by age/time using evertras/bubble-table SortByAsc/Desc in internal/views/resourcelist.go
- [x] T068 [P] Implement manual refresh: Ctrl-R re-triggers current view's fetch command and shows loading spinner in internal/app/app.go
- [x] T069 [P] Implement back/forward history navigation: [ pops history back, ] pushes history forward, using NavigationStack in internal/app/app.go
- [x] T070 [P] Add NO_COLOR environment variable support: detect os.Getenv("NO_COLOR"), set lipgloss.SetColorProfile(termenv.Ascii) when set in internal/app/styles.go
- [x] T071 Implement all edge case error handling: expired credentials → suggest re-auth, empty results → "No X found in region" message, unknown commands → "Unknown command" status, API timeout → error in status bar with retry hint, no profiles → setup instructions in internal/app/app.go
- [x] T072 [P] Add benchmark test for filter performance: generate 1000-row table, verify filter completes in <200ms per keystroke (SC-005) in tests/unit/filter_bench_test.go
- [x] T073 [P] Add benchmark tests for startup time (<2s, SC-001), resource list load (<5s for 500 resources, SC-003), and profile switch (<1s, SC-004) in tests/unit/bench_test.go
- [x] T074 [P] Configure coverage threshold: add `go test -coverprofile` to Makefile test target, add coverage check enforcing ≥90% on internal/aws/ and internal/app/ in Makefile
- [x] T075 Run full quickstart.md smoke test checklist manually and fix any failures

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can proceed in priority order (P1 → P2 → P3 → P4 → P5)
  - Some stories can overlap: US4 (filter) and US5 (resource types) are independent
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

- **US1 (P1)**: Can start after Foundational (Phase 2) — No dependencies on other stories. MVP target.
- **US2 (P2)**: Can start after Phase 2 — Adds to US1 (profile/region switching). Independently testable.
- **US3 (P3)**: Can start after US1 — Requires resource list view (T027) to have resources to describe. S3 fetcher needed.
- **US4 (P4)**: Can start after US1 — Requires resource list view (T027) to have rows to filter. Independent of US2/US3.
- **US5 (P5)**: Can start after US1 — Requires generic resource list view (T027) and AWS interfaces (T011). Independent of US2/US3/US4. Note: `:s3` command (T065) depends on S3 fetcher (T047) from US3.

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Messages/types before views
- AWS fetchers before UI wiring
- Components before integration
- Story complete before checkpoint validation

### Parallel Opportunities

- Phase 1: T003, T004, T005 can all run in parallel
- Phase 2 types: T007, T008, T010 can run in parallel; T006, T009 before T011
- Phase 2 tests: T012, T013, T014 all in parallel (before implementation)
- Phase 2 impl: T015, T016 in parallel; T017 after T013; T018 after all
- US1 tests: T020, T021, T022 all in parallel
- US1 impl: T024, T025 in parallel; T023 before T026
- US5 tests: T056-T060 all in parallel
- US5 impl: T061-T064 all in parallel
- Phase 8: T066, T067, T068, T069, T070, T072, T073, T074 all in parallel

---

## Parallel Example: User Story 5

```bash
# Launch all tests for User Story 5 together:
Task: "Test RDS response parsing in tests/unit/aws_rds_test.go"
Task: "Test ElastiCache Redis response parsing in tests/unit/aws_redis_test.go"
Task: "Test DocumentDB response parsing in tests/unit/aws_docdb_test.go"
Task: "Test EKS two-step fetch in tests/unit/aws_eks_test.go"
Task: "Test Secrets Manager list parsing in tests/unit/aws_secrets_test.go"

# Launch all fetcher implementations together (after tests fail):
Task: "Implement RDS fetcher in internal/aws/rds.go"
Task: "Implement ElastiCache Redis fetcher in internal/aws/redis.go"
Task: "Implement DocumentDB fetcher in internal/aws/docdb.go"
Task: "Implement EKS fetcher in internal/aws/eks.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Launch app, verify main menu, navigate to EC2, return
5. Deploy/demo if ready — this is the smallest viable product

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add US1 → Launch + browse EC2 (MVP!)
3. Add US2 → Multi-profile, multi-region
4. Add US3 → Detail views, S3 browsing, secret reveal, copy
5. Add US4 → Filter/search across all views
6. Add US5 → All 7 resource types with correct columns
7. Polish → Help, sorting, refresh, history, edge cases
8. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (MVP — highest priority)
   - After US1 checkpoint passes:
   - Developer A: User Story 3 (depends on US1 resource list)
   - Developer B: User Story 2 (independent of US1 impl details)
   - Developer C: User Story 5 (independent — AWS fetchers only)
   - Developer B: User Story 4 (after US1 table exists)
3. Polish phase: all developers in parallel

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- TDD is NON-NEGOTIABLE: write tests first, verify they fail, then implement
- Commit test + implementation together per TDD cycle
- Stop at any checkpoint to validate story independently
- All AWS fetchers return domain Resource types, not raw SDK types
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
