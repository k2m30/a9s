# a9s Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-18

## Active Technologies
- Go 1.26+ + Bubble Tea v2.0.2, Lipgloss v2.0.2, Bubbles v2, AWS SDK Go v2, yaml.v3, clipboard

## Project Structure

```text
cmd/
  a9s/           # main binary
  refgen/        # views_reference.yaml generator
internal/
  aws/           # AWS service clients & resource fetchers
  config/        # YAML config loading
  fieldpath/     # struct field extraction via reflection
  resource/      # generic resource model & registry
  tui/           # root Bubble Tea app model
    keys/        # key bindings
    layout/      # frame rendering
    messages/    # inter-view message types
    styles/      # Tokyo Night Dark palette
    views/       # view models (menu, list, detail, yaml, help, etc.)
tests/
  unit/          # 1,045+ unit tests
  integration/   # integration tests
docs/
  design/        # visual design spec
  qa/            # QA user stories
specs/           # feature specifications
```

## Commands

- `go build -o a9s ./cmd/a9s/` — build the binary
- `go test ./tests/unit/ -count=1 -timeout 120s` — run unit tests
- `golangci-lint run ./...` — run linter (MUST pass locally before any push)
- `govulncheck ./...` — check for known vulnerabilities (MUST pass locally before any push)
- `go run ./cmd/refgen/ > .a9s/views_reference.yaml` — regenerate the views reference file from AWS SDK struct reflection (dev-time only, no AWS credentials needed). Must be re-run after AWS SDK version updates.
- `go run ./cmd/preview/` — render static TUI design mockups using Lipgloss v2 (no AWS credentials needed). Used as visual truth for design spec compliance.

## Prerequisites

- Go 1.26+ (`brew install go`)
- golangci-lint v2.11+ (`brew install golangci-lint`)
- govulncheck (`go install golang.org/x/vuln/cmd/govulncheck@latest`)

## Code Style

Go 1.26+: Follow standard conventions

<!-- MANUAL ADDITIONS START -->

## Skills

| Skill | Scope | Usage |
|-------|-------|-------|
| `a9s-common` | All agents | Shell rules, package access rules, build/test commands |
| `a9s-bt-v2` | TUI-touching agents | Bubble Tea v2 / Lipgloss v2 / Bubbles v2 API patterns |
| `a9s-add-resource` | Coder + QA (on-demand) | 12-step blueprint for adding new AWS resource types (fetcher + view-layer tests) |

## Agents

| Agent | Role | Tools |
|-------|------|-------|
| `a9s-architect` | Architecture + resource type specs | Read-only |
| `a9s-coder` | Implementation — views, fetchers, resource types (TDD) | All |
| `a9s-tui-reviewer` | Code review — BT v2 correctness, design compliance | Read-only |
| `a9s-qa` | Writes test code for all resource types | All |
| `a9s-qa-stories` | Given/when/then stories from design spec (no source code) | Read/Glob/Grep |
| `a9s-pm` | Progress tracking, dependency management, releases | Read-only |
| `a9s-integrator` | Cross-package wiring, message flow, app.go | All |
| `a9s-fixtures` | Test fixtures from gobubble-dev via AWS MCP | Read/Write/Bash |
| `test-coverage-analyzer` | Test suite analysis, coverage gaps | Read-only |
| `tui-ux-auditor` | UX review, k9s comparison, design guidelines | Read + Web |
| `a9s-devops` | AWS practitioner — resource priorities, feature advice, real workflows | All |

## Rules

- ALWAYS bump version in `cmd/a9s/main.go` and rebuild binary after ANY code change
- ALWAYS write failing tests BEFORE writing implementation code (TDD is non-negotiable)
- ALWAYS test ALL resource types (S3, EC2, RDS, Redis, DocumentDB, EKS, Secrets Manager, VPC, SG, Node Groups, etc), not just one
- ALWAYS run `go test`, `golangci-lint run ./...`, and `govulncheck ./...` locally BEFORE pushing. CI is not a debugging tool.
- NEVER delete code, tests, or helpers just to make a linter happy. Understand WHY the code exists first. If it's genuinely dead, remove it. If it serves a purpose (scaffolding, crash-verification tests), use a targeted `//nolint` with a reason comment.
- NEVER make multiple push-and-check cycles. Get it right locally, push once.

<!-- MANUAL ADDITIONS END -->
