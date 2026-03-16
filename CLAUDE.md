# a9s Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-16

## Active Technologies
- Go 1.25+ (go.mod) + Bubble Tea v2, bubble-table, AWS SDK Go v2, `gopkg.in/yaml.v3` (new) (002-configurable-views)
- YAML config files (filesystem) (002-configurable-views)
- Go 1.25+ + Bubble Tea v2, lipgloss v2, yaml.v3, clipboard (003-fix-ui-bugs)

- Go 1.22+ + Bubble Tea v2 (charm.land/bubbletea/v2), (001-aws-tui-manager)

## Project Structure

```text
src/
tests/
```

## Commands

- `go build -o a9s ./cmd/a9s/` — build the binary
- `go test ./tests/unit/ -count=1 -timeout 120s` — run unit tests
- `go run ./cmd/refgen/ > views_reference.yaml` — regenerate the views reference file from AWS SDK struct reflection (dev-time only, no AWS credentials needed). Must be re-run after AWS SDK version updates.

## Code Style

Go 1.22+: Follow standard conventions

## Recent Changes
- 003-fix-ui-bugs: Added Go 1.25+ + Bubble Tea v2, lipgloss v2, yaml.v3, clipboard
- 002-configurable-views: Added Go 1.25+ (go.mod) + Bubble Tea v2, bubble-table, AWS SDK Go v2, `gopkg.in/yaml.v3` (new)

- 001-aws-tui-manager: Added Go 1.22+ + Bubble Tea v2 (charm.land/bubbletea/v2),

<!-- MANUAL ADDITIONS START -->

## Rules

- ALWAYS bump version in `cmd/a9s/main.go` and rebuild binary after ANY code change
- ALWAYS write failing tests BEFORE writing implementation code (TDD is non-negotiable)
- ALWAYS test ALL resource types (S3, EC2, RDS, Redis, DocumentDB, EKS, Secrets Manager), not just one

<!-- MANUAL ADDITIONS END -->
