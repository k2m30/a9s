# Repository Guidelines

## Project Structure & Module Organization
- `cmd/` contains entrypoints and helper binaries: `cmd/a9s` (main app), `cmd/refgen`, and preview tools.
- `internal/` holds production code:
  - `internal/aws/` read-only AWS fetchers and service clients.
  - `internal/tui/` Bubble Tea UI (views, layout, keys, styles, messages).
  - `internal/resource/` resource registry/types and pagination metadata.
  - `internal/config/` default and user view configuration loading.
  - `internal/demo/` synthetic fixtures used by `--demo` mode.
- `tests/unit/` is the primary regression suite; `tests/integration/` covers integration-tagged flows.
- `docs/`, `website/`, and `releases/` hold product docs and release notes.

## Build, Test, and Development Commands
- `make build` builds `./a9s` with version metadata.
- `make run` builds then starts the binary locally.
- `make test` runs race-enabled unit tests (`./tests/unit/`).
- `make integration` runs integration tests (`-tags integration`).
- `make lint` runs `golangci-lint` with repo config.
- `make security` runs `govulncheck ./...`.
- `make verify-readonly` fails if write-style AWS API calls are introduced.
- `make coverage` generates `coverage.out` and prints function coverage.

## Coding Style & Naming Conventions
- Go version: `1.26.x` (see `go.mod`).
- Formatting is enforced with `gofmt` (`make fmt`); do not hand-format.
- `.editorconfig`: tabs for `*.go`, 2-space indentation for YAML/Markdown/JSON.
- Keep naming aligned with existing patterns:
  - AWS fetchers in `internal/aws/<service>.go`.
  - Tests as `*_test.go`, often scoped by feature (example: `qa_pagination_view_test.go`).
- Follow read-only architecture: no mutating AWS operations in runtime code.

## Testing Guidelines
- Add/update tests with every behavior change; prefer TDD (failing test first).
- Run at minimum: `make test lint verify-readonly` before opening a PR.
- For cross-package confidence, run `make coverage`.
- Integration tests are opt-in and should be run when touching end-to-end behavior.

## Commit & Pull Request Guidelines
- Use Conventional Commits (`feat:`, `fix:`, `test:`, `docs:`), matching recent history.
- Keep commits focused and atomic; include tests in the same PR when applicable.
- PRs should follow `.github/pull_request_template.md`:
  - clear summary and linked issues (`Fixes #123`)
  - change type checked
  - checklist completed (`make test`, `make lint`, docs updates if needed)
- If UI behavior changes, include a screenshot/GIF or terminal capture for reviewers.
