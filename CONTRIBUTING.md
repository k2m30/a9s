# Contributing to a9s

Thank you for your interest in contributing to a9s. This document explains how
to get started.

## Prerequisites

- Go 1.25 or later
- golangci-lint
- make

## Getting Started

1. Fork the repository
2. Clone your fork
3. Build: `make build`
4. Run tests: `make test`
5. Run linter: `make lint`

## Development Workflow

1. Create a feature branch from `main`
2. Write failing tests first (TDD is required)
3. Implement the feature or fix
4. Ensure all tests pass: `make test`
5. Ensure linting passes: `make lint`
6. Submit a pull request against `main`

## Testing

- **Unit tests**: `make test`
- **Integration tests**: `make integration`
- **Coverage report**: `make coverage`
- Test ALL resource types, not just one
- Write tests before implementation code

## Commit Messages

Use conventional commit format:

- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation only
- `test:` adding or updating tests
- `refactor:` code change that neither fixes a bug nor adds a feature
- `chore:` maintenance tasks
- `perf:` performance improvement

## Project Structure

```
cmd/a9s/           main binary
internal/aws/      AWS service clients and resource fetchers (read-only)
internal/config/   YAML config loading
internal/resource/ generic resource model and registry
internal/tui/      Bubble Tea views, keys, layout, styles, messages
tests/unit/        unit tests (1,045+)
tests/integration/ integration tests
```

## Architecture Notes

- a9s is **read-only by design** -- it never makes write calls to AWS
- The TUI is built with Bubble Tea v2, Lipgloss v2, and Bubbles v2
- Each AWS resource type has a fetcher in `internal/aws/` and a type definition
  in `internal/resource/types.go`
- All key bindings are defined in `internal/tui/keys/keys.go`

## Good First Issues

Look for issues labeled
[good first issue](https://github.com/k2m30/a9s/labels/good%20first%20issue)
for beginner-friendly tasks.

## Code of Conduct

Please read and follow our [Code of Conduct](CODE_OF_CONDUCT.md).
