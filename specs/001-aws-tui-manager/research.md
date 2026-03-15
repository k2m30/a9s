# Research: a9s — Terminal UI AWS Resource Manager

**Branch**: `001-aws-tui-manager` | **Date**: 2026-03-15

## Decision 1: TUI Framework

**Decision**: Bubble Tea v2 (v2.0.2) + Bubbles v2 + Lip Gloss v2

**Rationale**:
- k9s (the inspiration) is built in Go with tview/tcell, proving
  Go is the right language for this problem domain.
- Bubble Tea v2 is the most popular TUI framework (40k+ stars),
  with the "Cursed Renderer" delivering orders-of-magnitude faster
  rendering.
- The Charm ecosystem (Bubbles components, Lip Gloss styling)
  provides table, list, textinput, viewport, spinner, and help
  components out of the box.
- Import path: `charm.land/bubbletea/v2`

**Alternatives considered**:
- tview/tcell (what k9s uses): k9s forked both libraries to
  maintain custom modifications. Less actively maintained than
  Bubble Tea. Would require more low-level work.
- Rust + Ratatui: Superior raw performance but significantly
  slower development velocity and steeper learning curve.
- Python + Textual: Fastest development but distribution
  requires Python runtime or bloated PyInstaller bundles.

## Decision 2: Enhanced Table Component

**Decision**: evertras/bubble-table v0.19.2

**Rationale**:
- The built-in `bubbles/table` does NOT support sorting or
  filtering — both required by FR-007 and FR-017.
- evertras/bubble-table provides built-in sorting (single and
  multiple columns, numeric-aware), built-in filtering via `/`,
  pagination, row selection with j/k navigation, horizontal
  scrolling, and per-row/cell styling.
- 562 GitHub stars, actively maintained (latest release Sep 2025).
- Directly satisfies the k9s-style table interaction model.

**Alternatives considered**:
- Built-in bubbles/table: Lacks sorting and filtering. Would
  require implementing both from scratch.
- Custom table implementation: High effort, low benefit when
  evertras/bubble-table covers the requirements.

## Decision 3: AWS SDK

**Decision**: aws-sdk-go-v2 with per-service packages

**Rationale**:
- GA, production-ready, actively maintained.
- Modular — import only the service packages needed.
- Built-in paginators for all list operations.
- Standard Go error handling with typed errors via smithy.
- All 7 target services have Go SDK support with paginators.

**Required service packages**:
- `github.com/aws/aws-sdk-go-v2/config`
- `github.com/aws/aws-sdk-go-v2/service/s3`
- `github.com/aws/aws-sdk-go-v2/service/ec2`
- `github.com/aws/aws-sdk-go-v2/service/rds`
- `github.com/aws/aws-sdk-go-v2/service/elasticache`
- `github.com/aws/aws-sdk-go-v2/service/docdb`
- `github.com/aws/aws-sdk-go-v2/service/eks`
- `github.com/aws/aws-sdk-go-v2/service/secretsmanager`

## Decision 4: AWS Profile Enumeration

**Decision**: gopkg.in/ini.v1 for parsing ~/.aws/config

**Rationale**:
- The AWS SDK v2 does NOT provide a public API to enumerate
  profiles. You must parse the INI files directly.
- gopkg.in/ini.v1 is the standard Go INI parser.
- Profiles in `~/.aws/config` are prefixed with `profile ` in
  section names (except `[default]`). Profiles in
  `~/.aws/credentials` use bare section names.

**Alternatives considered**:
- Manual INI parsing: Fragile, no benefit over a well-tested library.
- Shelling out to `aws configure list-profiles`: Requires AWS CLI
  installed, adds external dependency.

## Decision 5: Application Architecture

**Decision**: State machine pattern with composable child models

**Rationale**:
- The root model holds a `currentView` enum and delegates to
  view-specific update/render functions.
- Child models (table, viewport, textinput) live inside the root
  model and receive messages selectively.
- Persistent layout: header + breadcrumbs + content + status bar
  composed via lipgloss.JoinVertical.
- Async operations use `tea.Cmd` — the Bubble Tea runtime
  executes commands asynchronously and delivers results as messages.
- This matches the official Bubble Tea patterns (views example,
  composable-views example).

## Decision 6: Testing Strategy

**Decision**: Interface-based mocking + teatest

**Rationale**:
- AWS SDK v2 does not ship interface packages. Define narrow
  interfaces per operation, which real clients satisfy implicitly.
- Mock implementations use Go function types for concise test setup.
- teatest (charmbracelet/x/exp/teatest) provides test model
  creation, key press simulation, output assertions, and golden
  file comparison.
- Pure function testing of Update/View for unit tests.
- Set `lipgloss.SetColorProfile(termenv.Ascii)` in tests for
  deterministic output.

## Decision 7: Clipboard

**Decision**: Bubble Tea v2 native OSC52 clipboard (primary),
atotto/clipboard v0.1.4 (fallback)

**Rationale**:
- Bubble Tea v2 includes native clipboard integration via OSC52
  escape sequences, which works over SSH.
- atotto/clipboard as fallback for terminals that don't support
  OSC52. No Cgo dependency, text-only (which is all a9s needs).

## Decision 8: Project Structure

**Decision**: Standard Go layout with cmd/ and internal/

```
a9s/
  cmd/a9s/main.go
  internal/
    app/        (root model, keys, styles, messages)
    ui/         (header, statusbar, help, command input)
    views/      (mainmenu, resourcelist, detail, profile, region)
    aws/        (client factory, per-service fetchers)
    resource/   (resource interface, column definitions)
    navigation/ (history stack)
  go.mod
  go.sum
  Makefile
```

**Rationale**:
- `cmd/a9s/main.go` is minimal — creates root model, runs program.
- `internal/` prevents external imports of internal packages.
- `internal/aws/` isolates all AWS SDK calls behind interfaces
  for testability.
- Each `internal/aws/*.go` returns domain types, not raw SDK types.

## Decision 9: Linting and Formatting

**Decision**: gofmt + golangci-lint

**Rationale**:
- gofmt is Go's standard formatter — zero configuration.
- golangci-lint aggregates multiple linters (govet, errcheck,
  staticcheck, gosimple, unused, etc.) with sane defaults.
- Enforced via Makefile targets and pre-commit hooks.

## Decision 10: Distribution

**Decision**: Single binary via `go build`, cross-compiled

**Rationale**:
- Go produces static binaries with no runtime dependencies.
- Cross-compilation is trivial: `GOOS=linux GOARCH=amd64 go build`.
- Future: goreleaser for automated multi-platform releases.

## API Call Patterns

### Pagination

All list operations use the SDK's built-in paginator pattern:
```
paginator := service.New<Operation>Paginator(client, input)
for paginator.HasMorePages() {
    page, _ := paginator.NextPage(ctx)
    // process page
}
```

Available paginators: ListBuckets, ListObjectsV2,
DescribeInstances, DescribeDBInstances, DescribeCacheClusters,
DescribeDBClusters (docdb), ListClusters (eks), ListSecrets.

Single-item calls (no paginator needed): DescribeCluster (eks),
GetSecretValue.

### Error Classification

AWS SDK v2 errors unwrap via `errors.As` to `smithy.APIError`
with `ErrorCode()` and `ErrorMessage()`. Key error codes:
- Credentials: `ExpiredToken`, `ExpiredTokenException`
- Access: `AccessDenied`, `AccessDeniedException`
- Throttling: `Throttling`, `ThrottlingException`,
  `TooManyRequestsException`
- Not found: Service-specific typed errors (e.g., `NoSuchBucket`)

### ElastiCache Redis Filtering

The DescribeCacheClusters API has no server-side engine filter.
Redis clusters must be filtered client-side by checking
`cluster.Engine == "redis"`.

### DocumentDB vs Aurora

Use the `docdb` service package (not `rds`) for DocumentDB.
Filter with `engine=docdb` in the Filters parameter to exclude
Aurora clusters.

### EKS Two-Step Pattern

EKS requires ListClusters (returns names only) followed by
DescribeCluster for each cluster to get details (version, status,
endpoint, platform version).

### SSO Profile Handling

SSO profiles work transparently with `config.LoadDefaultConfig`
+ `WithSharedConfigProfile`. The SDK reads cached SSO tokens
from `~/.aws/sso/cache`. If the token is expired, the SDK
returns an error — the app should suggest running
`aws sso login --profile <name>`.
