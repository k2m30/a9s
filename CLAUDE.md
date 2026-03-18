# a9s Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-18

## Active Technologies
- Go 1.25+ + Bubble Tea v2.0.2, Lipgloss v2.0.2, Bubbles v2, AWS SDK Go v2, yaml.v3, clipboard

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
- `go run ./cmd/refgen/ > views_reference.yaml` — regenerate the views reference file from AWS SDK struct reflection (dev-time only, no AWS credentials needed). Must be re-run after AWS SDK version updates.

## Code Style

Go 1.25+: Follow standard conventions

<!-- MANUAL ADDITIONS START -->

## Agents

| Agent | Role |
|-------|------|
| `a9s-architect` | Architecture owner — design decisions, component interfaces, message contracts, dependency boundaries |
| `a9s-coder` | Implementation — writes Go code with TDD, knows BT v2 / Lipgloss v2 / Bubbles v2 APIs |
| `a9s-tui-reviewer` | Code review — verifies BT v2 correctness, architecture compliance, design spec adherence |
| `a9s-qa` | QA — writes/runs tests, verifies all 7 resource types, catches regressions |
| `a9s-qa-stories` | QA stories — generates given/when/then stories from design spec + views.yaml (no implementation knowledge) |
| `a9s-pm` | Project manager — tracks progress, manages dependencies, verifies gates, coordinates releases |
| `a9s-integrator` | Integration — cross-package wiring, message flow, entrypoint switching |
| `a9s-fixtures` | Test fixtures — fetches real AWS data from gobubble-dev via MCP tool |
| `test-coverage-analyzer` | Test analysis — coverage gaps, test quality, structure assessment |
| `tui-ux-auditor` | UX review — design guidelines, interaction patterns, k9s comparison |

## Rules

- ALWAYS bump version in `cmd/a9s/main.go` and rebuild binary after ANY code change
- ALWAYS write failing tests BEFORE writing implementation code (TDD is non-negotiable)
- ALWAYS test ALL resource types (S3, EC2, RDS, Redis, DocumentDB, EKS, Secrets Manager), not just one

<!-- MANUAL ADDITIONS END -->
