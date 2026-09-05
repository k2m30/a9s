# Contributing to a9s

a9s is built entirely with [Claude Code](https://docs.anthropic.com/en/docs/claude-code). We encourage contributors to do the same. This guide covers setup, workflow, and the do's and don'ts of working on this codebase with Claude Code.

## Licensing and the Contributor License Agreement (CLA)

a9s is dual-licensed: the whole repository is GPL-3.0-or-later, and the
packages under `core/` are additionally offered by the copyright holder
under a separate commercial license (see
[COMMERCIAL-LICENSE.md](COMMERCIAL-LICENSE.md)). Dual licensing only works
while the copyright holder can relicense every line under `core/`.

Therefore:

- **Any contribution that touches `core/` requires a signed Contributor
  License Agreement** based on the
  [Apache Individual Contributor License Agreement](https://www.apache.org/licenses/icla.pdf)
  (with "the Foundation" read as the a9s copyright holder, Mikhail
  Chuprynski). The CLA grants the copyright holder a perpetual, worldwide,
  irrevocable copyright and patent license to your contribution, including
  the right to sublicense and to distribute it under other license terms.
- **Un-CLA'd contributions cannot be merged into `core/`.** A PR touching
  `core/` from a contributor without a CLA on file will be rewritten by the
  maintainer or closed, however small the diff.
- Contributions limited to `internal/tui/`, `tests/`, `docs/`, or `website/`
  are accepted under the inbound=outbound GPL-3.0-or-later norm and do not
  require a CLA — though signing one is welcome.

To sign, open an issue titled `CLA: <your GitHub handle>` stating that you
accept the Apache ICLA terms with the a9s copyright holder as licensee, or
attach the completed ICLA form to your first PR.

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
- **Skills** — reusable workflows (a9s-common, a9s-bt-v2, a9s-add-resource, a9s-add-child-view, a9s-add-related-view, a9s-implement-issue)

You don't need to edit it to contribute. Just start Claude Code and it knows the rules.

### Agents

Claude Code dispatches specialized agents for different tasks. Key ones for contributors:

| Agent | What it does | When to use |
|-------|-------------|-------------|
| `a9s-dev` | Writes implementation code, fixtures and generated docs — never tests | Features, bug fixes |
| `a9s-qa` | Writes test code — never production code | Test coverage gaps |
| `a9s-facilitator` | Rules when the dev/QA loop stalls | `OFF`, `LOOP`, `BLOCKED` |
| `a9s-acceptance` | Skeptical end-user acceptance on rendered surfaces | Before a release |
| `a9s-qa-stories` | Given/when/then stories from the design spec | New views, before tests |
| `a9s-consistency-checker` | Verifies code/docs/website alignment | Before pushing |
| `a9s-devops` | AWS-practitioner consult | Resource priorities, workflows |
| `tui-designer` | TUI wireframes and color schemes | New/redesigned views |

You don't invoke these manually — Claude Code uses them when appropriate. But you can ask for them explicitly: "use a9s-qa to add tests" or "run the consistency checker". Architecture review is the `arch-review` skill; coverage and BT v2 correctness are reviewed directly in-session.

### Skills

Skills are reusable workflows loaded by Claude Code:

- **`a9s-common`** — shared rules for all agents (shell rules, build commands)
- **`a9s-bt-v2`** — Bubble Tea v2 / Lipgloss v2 API patterns
- **`a9s-add-resource`** — 12-step blueprint for adding new AWS resource types
- **`a9s-add-child-view`** — step-by-step blueprint for adding child views (child fetcher + parent wiring + tests)
- **`a9s-add-related-view`** — blueprint for adding related-resource views per resource type
- **`a9s-implement-issue`** — end-to-end workflow: analyze → design → implement → verify → release

## Do's

- **Do describe intent, not implementation.** Say "add CloudWatch Metrics as a resource type" not "create a file core/aws/cloudwatch_metrics.go with a function..."
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
1. Fetcher in `core/aws/`
2. Type definition — a `catalog.ResourceTypeDef` literal in `core/aws/catalog_<category>.go`
3. Default view config in `core/config/defaults.go`
4. Demo fixtures in `core/demo/`
5. Unit tests for fetcher, view rendering, and demo fixtures
6. README and website updates

## Project Structure

```
cmd/a9s/            main binary
cmd/refgen/         views_reference.yaml generator
core/aws/           AWS service clients, resource fetchers (read-only), catalog_*.go registry literals
core/app/           headless controller (ViewState, actions) shared by TUI and web
core/catalog/       the resource registry (ResourceTypeDef and friends)
core/config/        YAML config loading
core/demo/          synthetic fixture data for --demo mode
core/domain/        core value types (Finding, Resource, Gen, …)
core/fieldpath/     struct field extraction via reflection
core/resource/      backward-compat alias layer over core/catalog
core/runtime/       platform-agnostic app core (runtime.Core, message handlers)
core/session/       session-scoped state (RowStore, caches, generation counters)
core/web/           web-UI adapter
internal/tui/       Bubble Tea views, keys, layout, styles, messages
tests/unit/         unit tests
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
