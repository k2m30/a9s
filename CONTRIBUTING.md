# Contributing to a9s

a9s is built entirely with [Claude Code](https://docs.anthropic.com/en/docs/claude-code). We encourage contributors to do the same. This guide covers setup, workflow, and the do's and don'ts of working on this codebase with Claude Code.

## Prerequisites

- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) CLI installed and authenticated
- Go 1.26+ (`brew install go`)
- golangci-lint v2.11+ (`brew install golangci-lint`)
- govulncheck (`go install golang.org/x/vuln/cmd/govulncheck@latest`)

## Getting Started

```sh
git clone https://github.com/k2m30/a9s.git
cd a9s
claude          # start Claude Code — it reads CLAUDE.md automatically
```

That's it. Claude Code reads `CLAUDE.md` at the project root which contains all build commands, project structure, architecture rules, available agents, and skills. You don't need to memorize anything.

## Development Workflow

1. Fork and clone the repo
2. Create a feature branch from `main`
3. Start Claude Code: `claude`
4. Describe what you want to do in plain English
5. Claude Code writes tests first, then implementation (TDD is enforced)
6. Before pushing, Claude Code runs the pre-push checklist automatically (tests, lint, vulncheck, consistency checker, coverage analyzer, architect review)
7. Submit a pull request against `main`

## Claude Code Setup

### CLAUDE.md

The project's `CLAUDE.md` is the single source of truth for how Claude Code operates in this repo. It defines:

- **Commands** — build, test, lint, vulncheck, refgen
- **Project structure** — where everything lives
- **Rules** — TDD, pre-push checks, docs sync, no CI debugging
- **Agents** — specialized sub-agents for architecture, coding, QA, review, etc.
- **Skills** — reusable workflows (a9s-common, a9s-bt-v2, a9s-add-resource, a9s-add-child-view)

You don't need to edit it to contribute. Just start Claude Code and it knows the rules.

### Agents

Claude Code dispatches specialized agents for different tasks. Key ones for contributors:

| Agent | What it does | When to use |
|-------|-------------|-------------|
| `a9s-coder` | Writes implementation code (TDD) | Features, bug fixes |
| `a9s-qa` | Writes test code | Test coverage gaps |
| `a9s-architect` | Reviews architecture (scores against checklist) | Before pushing |
| `a9s-tui-reviewer` | Reviews Bubble Tea v2 correctness | After TUI changes |
| `a9s-consistency-checker` | Verifies code/docs/website alignment | Before pushing |
| `test-coverage-analyzer` | Finds test coverage gaps | Before pushing |

You don't invoke these manually — Claude Code uses them when appropriate. But you can ask for them explicitly: "run the architect agent" or "check test coverage".

### Skills

Skills are reusable workflows loaded by Claude Code:

- **`a9s-common`** — shared rules for all agents (shell rules, build commands)
- **`a9s-bt-v2`** — Bubble Tea v2 / Lipgloss v2 API patterns
- **`a9s-add-resource`** — 12-step blueprint for adding new AWS resource types
- **`a9s-add-child-view`** — step-by-step blueprint for adding child views (child fetcher + parent wiring + tests)

## Do's

- **Do describe intent, not implementation.** Say "add CloudWatch Metrics as a resource type" not "create a file internal/aws/cloudwatch_metrics.go with a function..."
- **Do let Claude Code run the pre-push checklist.** It runs tests, lint, vulncheck, consistency checker, coverage analyzer, and architect review. Don't skip it.
- **Do ask Claude Code to explain code** before modifying it. It has full context of the codebase.
- **Do use `--demo` mode** to verify UI changes without AWS credentials: `./a9s --demo`
- **Do commit frequently.** Small, focused commits with conventional commit messages.
- **Do test all resource types**, not just the one you're working on. If you add a feature to EC2, verify it works for S3, Lambda, RDS, etc.

## Don'ts

- **Don't use CI as a debugging tool.** Run `make test`, `make lint`, and `govulncheck ./...` locally before pushing. Push once.
- **Don't delete code to make linters happy.** Understand why the code exists first. If it's dead, remove it. If it has a purpose, use a targeted `//nolint` with a reason comment.
- **Don't skip TDD.** Write failing tests first. Claude Code enforces this.
- **Don't edit docs manually.** When code changes affect resource types, key bindings, commands, or CLI flags, tell Claude Code to update README and website in the same PR.
- **Don't push without the pre-push checklist.** No exceptions.
- **Don't fight Claude Code's architecture decisions.** The project scores 9.5/10 on the architecture checklist. If you disagree with a pattern, open a discussion first.

## Adding a New AWS Resource Type

This is the most common contribution. Just tell Claude Code:

> "Add CloudWatch Metrics as a new resource type"

It will use the `a9s-add-resource` skill which covers:
1. Fetcher in `internal/aws/`
2. Type definition in `internal/resource/types.go`
3. Default view config in `internal/config/defaults.go`
4. Demo fixtures in `internal/demo/`
5. Unit tests for fetcher, view rendering, and demo fixtures
6. README and website updates

## Project Structure

```
cmd/a9s/            main binary
cmd/refgen/         views_reference.yaml generator
internal/aws/       AWS service clients and resource fetchers (read-only)
internal/config/    YAML config loading
internal/demo/      synthetic fixture data for --demo mode
internal/fieldpath/ struct field extraction via reflection
internal/resource/  generic resource model and registry
internal/tui/       Bubble Tea views, keys, layout, styles, messages
tests/unit/         unit tests (1,045+)
tests/integration/  integration tests
```

## Architecture

- **Read-only by design** — a9s never makes write calls to AWS. Enforced by CI.
- **No credential access** — a9s never reads `~/.aws/credentials`.
- **Bubble Tea v2** — all I/O in `tea.Cmd` closures, views are pure functions.
- **Single source of truth** — key bindings in `keys/keys.go`, types in `types.go`, styles in `styles/`.
- **Message-driven** — views communicate via typed messages, never import each other.

## Code of Conduct

Please read and follow our [Code of Conduct](CODE_OF_CONDUCT.md).
