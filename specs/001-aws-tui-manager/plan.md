# Implementation Plan: a9s — Terminal UI AWS Resource Manager

**Branch**: `001-aws-tui-manager` | **Date**: 2026-03-15 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-aws-tui-manager/spec.md`

## Summary

Build a read-only terminal UI for browsing AWS resources (S3, EC2,
RDS, Redis, DocumentDB, EKS, Secrets Manager), inspired by k9s. The
app uses k9s-style colon commands for navigation, vim-style keybindings,
and supports multi-profile/multi-region switching. Built with Go using
Bubble Tea v2 for the TUI framework and aws-sdk-go-v2 for AWS API calls.
Single binary distribution, async data loading, no auto-refresh.

## Technical Context

**Language/Version**: Go 1.22+
**Primary Dependencies**: Bubble Tea v2 (charm.land/bubbletea/v2),
Bubbles v2 (charm.land/bubbles/v2), Lip Gloss v2
(charm.land/lipgloss/v2), evertras/bubble-table v0.19.2,
aws-sdk-go-v2, gopkg.in/ini.v1, atotto/clipboard
**Storage**: N/A (read-only, no local storage)
**Testing**: Go standard `testing` package + teatest
(charmbracelet/x/exp/teatest) for TUI testing
**Target Platform**: macOS, Linux (cross-platform terminal, 256-color)
**Project Type**: CLI/TUI application
**Performance Goals**: <2s startup, <5s resource list load,
<200ms filter response, responsive UI during async API calls
**Constraints**: Single binary, read-only (no CUD operations),
manual refresh only (Ctrl-R), relies on existing AWS credential chain
**Scale/Scope**: 7 resource types, up to 500 resources per type,
10 views (main, 7 resource lists, profile select, region select)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### I. Test-Driven Development (NON-NEGOTIABLE) — PASS

- **Strategy**: Interface-based AWS client design enables mock
  injection for all service calls.
- **TUI testing**: teatest package for model testing with simulated
  key presses and output assertions.
- **Pure function tests**: Update() and View() are pure functions
  that can be unit tested directly.
- **TDD cycle**: Each behavior (keybinding, view transition, API
  response handling) gets a failing test first.

### II. Code Quality — PASS

- **Formatting**: gofmt (Go standard formatter, zero config).
- **Linting**: golangci-lint with default linters (govet, errcheck,
  staticcheck, gosimple, unused).
- **Single responsibility**: Clear package separation — aws/,
  views/, ui/, app/, resource/, navigation/.
- **Complexity**: Functions kept small by delegating view logic
  to per-view update/render functions.
- **Pre-commit hooks**: Makefile targets for fmt, lint, test.

### III. Testing Standards — PASS

- **Unit tests**: Model update logic, AWS response parsing,
  navigation history, filter matching. Interface-based mocks
  for AWS clients.
- **Integration tests**: Optional — real AWS API calls with a test
  profile in CI (requires AWS credentials in CI environment).
- **Contract tests**: AWS response parsing contracts — verify that
  domain types are correctly extracted from SDK response types.
- **E2E tests**: teatest for full TUI interaction flows (launch →
  navigate → describe → back).
- **Coverage target**: ≥90% on internal/aws/ and internal/app/
  packages.

### IV. User Experience Consistency — PASS (with adaptations)

- **Terminal accessibility adaptation**: WCAG 2.1 AA applies to
  web/GUI. For terminal apps, this translates to:
  - Sufficient color contrast (tested against 256-color palette)
  - Full keyboard navigation (the only input method)
  - `NO_COLOR` environment variable support
  - Status-based color coding with shape/text redundancy
    (not color-only indicators)
- **Design patterns**: k9s interaction patterns applied consistently
  across all 7 resource types.
- **Error states**: Specific, actionable messages in status bar
  (no generic errors).
- **Loading states**: Spinner in status bar during all API calls.

### V. Performance Requirements — PASS (with adaptations)

- **Terminal-specific metrics** (web metrics LCP/FID/JS bundles
  do not apply):
  - Startup: <2s to main screen (SC-001)
  - Resource load: <5s for 500 resources (SC-003)
  - Filter: <200ms per keystroke for 1000 items (SC-005)
  - UI responsiveness: keyboard input accepted during API calls (SC-007)
- **No database queries**: AWS API call latency is network-bound,
  not application-bound.
- **CI benchmarks**: Benchmark tests for filter performance on
  large datasets. Startup time benchmark.
- **Single binary**: No bundle size concern — Go binary ~15-20MB.

## Project Structure

### Documentation (this feature)

```text
specs/001-aws-tui-manager/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── commands.md
│   └── ui-layout.md
└── tasks.md             (created by /speckit.tasks)
```

### Source Code (repository root)

```text
cmd/
└── a9s/
    └── main.go                  # Entry point: parse flags, create model, run program

internal/
├── app/
│   ├── app.go                   # Root model: Init, Update, View, view routing
│   ├── keys.go                  # Global KeyMap definitions
│   ├── styles.go                # Lip Gloss style constants
│   └── messages.go              # Shared message types (resourcesLoaded, apiError, etc.)
├── ui/
│   ├── header.go                # Header component (profile, region, version)
│   ├── statusbar.go             # Status bar (hints, command input, errors)
│   ├── breadcrumbs.go           # Breadcrumb trail renderer
│   ├── help.go                  # Help overlay (keybinding reference)
│   └── command.go               # Command input mode (: prefix, suggestions)
├── views/
│   ├── mainmenu.go              # Resource type list (main/root view)
│   ├── resourcelist.go          # Generic resource table view
│   ├── detail.go                # Describe view (key-value attributes)
│   ├── jsonview.go              # Raw JSON view (y key)
│   ├── reveal.go                # Secret reveal view (x key)
│   ├── profile.go               # Profile selector (:ctx)
│   └── region.go                # Region selector (:region)
├── aws/
│   ├── client.go                # AWS client factory (config loading, profile switching)
│   ├── profile.go               # Profile enumeration from INI files
│   ├── regions.go               # Hardcoded region list
│   ├── interfaces.go            # Service interfaces for mocking
│   ├── ec2.go                   # EC2 fetch + parse
│   ├── s3.go                    # S3 bucket + object fetch + parse
│   ├── rds.go                   # RDS fetch + parse
│   ├── redis.go                 # ElastiCache Redis fetch + parse
│   ├── docdb.go                 # DocumentDB fetch + parse
│   ├── eks.go                   # EKS fetch + parse
│   ├── secrets.go               # Secrets Manager fetch + parse
│   └── errors.go                # AWS error classification
├── resource/
│   ├── resource.go              # Resource interface + generic Resource type
│   └── types.go                 # Per-resource-type column definitions
└── navigation/
    └── history.go               # Back/forward navigation stack

tests/
├── unit/
│   ├── app_test.go              # Root model update/view tests
│   ├── aws_ec2_test.go          # EC2 response parsing tests
│   ├── aws_s3_test.go           # S3 response parsing tests
│   ├── aws_rds_test.go          # RDS response parsing tests
│   ├── aws_redis_test.go        # ElastiCache response parsing tests
│   ├── aws_docdb_test.go        # DocumentDB response parsing tests
│   ├── aws_eks_test.go          # EKS response parsing tests
│   ├── aws_secrets_test.go      # Secrets Manager response parsing tests
│   ├── aws_profile_test.go      # Profile enumeration tests
│   ├── aws_errors_test.go       # Error classification tests
│   ├── navigation_test.go       # History stack tests
│   └── filter_test.go           # Filter matching tests
├── integration/
│   └── tui_test.go              # teatest-based TUI interaction tests
└── testdata/
    ├── aws_config_sample         # Sample ~/.aws/config for tests
    └── aws_credentials_sample    # Sample ~/.aws/credentials for tests

go.mod
go.sum
Makefile
.golangci.yml
```

**Structure Decision**: Single Go project with `cmd/` entry point
and `internal/` packages. This is the standard Go layout for a
single-binary CLI application. No need for multi-project structure
— the app has no separate frontend/backend or API layer.

## Complexity Tracking

No constitution violations. All principles pass with documented
terminal-specific adaptations for Principle IV (UX) and
Principle V (Performance).
