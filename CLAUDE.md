# a9s Development Guidelines

Your work will be review by Codex.

## Process — single source of truth

**Read [`docs/development-process.md`](docs/development-process.md) first.** It defines the 8-stage lifecycle (Stages 1–8, with Stage 6.5 as an optional post-merge real-AWS gate), Definition of Ready, Definition of Done, agent ownership per stage, and the canonical pre-push and pre-release gates. If a rule below conflicts with that document, the document wins until updated.

Quick reference:

- Pre-push gate (Stage 6): `make ready-to-push`
- Pre-release gate (Stage 7): `make ready-to-release`
- Under CAE-1, CTO auto-pulls `todo, unassigned` issues in the active project every heartbeat (see [`docs/development-process.md`](docs/development-process.md) §"Continuous Autonomous Execution"). Other agents (Architect, QA, Coder, DevOps, etc.) act only on explicit dispatch from CTO or Architect — they never browse the backlog or pick up undispatched work.
- **Issue-creation discipline (mandatory, AS-446 retro).** Before creating any new Paperclip issue, check it against the five rules in [`docs/development-process.md`](docs/development-process.md) §"Issue-creation discipline": (1) one review issue per PR (not one per reviewer); (2) heartbeat ticks do not create issues; (3) drift-detection cap of ≤1 new issue per heartbeat; (4) recovery is a comment, not a child; (5) API probes are never issues. Violations are caught at retro and reverted.

## GitHub

- Repository: `k2m30/a9s` — always use this owner/repo for GitHub API calls, issues, and PRs

## Active Technologies

- Go 1.26+ + Bubble Tea v2.0.2, Lipgloss v2.0.2, Bubbles v2, AWS SDK Go v2 (autoscaling, codeartifact, codebuild, codepipeline, dynamodb, ec2, ecr, ecs, efs, elasticbeanstalk, elbv2, events, iam, kms, lambda, rds, secretsmanager, ses, sesv2, sfn, sns, ssm, eventbridge, backup), yaml.v3, clipboard (020-architecture-refactor)
- YAML config on disk (`~/.a9s/config.yaml`, `~/.a9s/themes/*.yaml`, `~/.a9s/views/`); YAML cache on disk (`~/.a9s/cache/<profile>--<region>.yaml`); session-scoped in-memory state owned by `internal/session.Session` after Phase 02 (020-architecture-refactor)

- Go 1.26+, Bubble Tea v2.0.2, Lipgloss v2.0.2, Bubbles v2, AWS SDK Go v2, yaml.v3, clipboard
- YAML config on disk (`~/.a9s/config.yaml`, `~/.a9s/themes/*.yaml`, `~/.a9s/views/`)
- YAML cache on disk (`~/.a9s/cache/<profile>--<region>.yaml`), in-memory maps
- In-process demo fixture store (per resource type, loaded at startup)
- In-memory session-scoped maps on root `Model` (findings cleared on profile/region switch; no disk persistence for findings themselves — cache format unchanged)
- Go 1.26+ (CLAUDE.md) + AWS SDK Go v2 (service clients for autoscaling, codeartifact, codebuild, codepipeline, dynamodb, ec2, ecr, ecs, efs, elasticbeanstalk, elbv2, events, iam, kms, lambda, rds, secretsmanager, ses, sesv2, sfn, sns, ssm, events/eventbridge, backup), Bubble Tea v2.0.2, Lipgloss v2.0.2, yaml.v3 (019-related-panel-checkers)
- In-memory `resource.ResourceCache` (`map[string]ResourceCacheEntry`, each with `Resources []Resource` + `IsTruncated bool`) built by the background fetcher pool; no on-disk state changes in this feature (019-related-panel-checkers)

## Project Structure

```text
cmd/
  a9s/           # main binary
  readmegen/     # README.md generator from docs/README.tmpl.md + docs/shared/
  refgen/        # views_reference.yaml generator
internal/
  aws/           # AWS service clients & resource fetchers (top-level + child)
  buildinfo/     # version resolution from ldflags / go install
  config/        # YAML config loading
  demo/          # synthetic fixture data for demo mode
  fieldpath/     # struct field extraction via reflection
  resource/      # generic resource model, registry, child-view definitions
  tui/           # root Bubble Tea app model
    keys/        # key bindings (including child-view triggers: e, L, r, s)
    layout/      # frame rendering
    messages/    # inter-view message types (NavigateMsg, EnterChildViewMsg, etc.)
    styles/      # Tokyo Night Dark palette
    views/       # view models (menu, list, detail, yaml, help, etc.)
tests/
  unit/          # unit tests
  integration/   # integration tests
docs/
  design/        # visual design spec (incl. child-views/ with 24 view levels)
  qa/            # QA user stories
specs/           # feature specifications
```

## Commands

- `make build` — build the binary
- `make test` — run all unit tests (fast, no race detector)
- `make test-race` — run all unit tests with `-race` (CI and pre-push)
- `go test ./tests/unit/ -run TestResourceList -count=1 -v` — run a single test by name
- `make lint` — run golangci-lint (MUST pass locally before any push). Note: do NOT include the `run` subcommand when calling golangci-lint directly — rtk treats it as a package path, causing a spurious `/run: directory not found` error.
- `make security` — check for known vulnerabilities via govulncheck (MUST pass locally before any push)
- `make gofix` — check for unfixed `//go:fix inline` directives (e.g. `reflect.Ptr` → `reflect.Pointer`). If it fails, run `go fix -inline ./...` to apply fixes.
- `go run ./cmd/readmegen/ > README.md` — regenerate README.md from template + shared docs (run after any changes to docs/shared/ or docs/README.tmpl.md)
- `go run ./cmd/viewsgen/` — regenerate per-resource YAML files in .a9s/views/ from built-in defaults (run after any changes to defaults.go)
- `go run ./cmd/refgen/ > .a9s/views_reference.yaml` — regenerate the views reference file from AWS SDK struct reflection (dev-time only, no AWS credentials needed). Must be re-run after AWS SDK version updates.
- `go run ./cmd/preview/` — render static TUI design mockups using Lipgloss v2 (no AWS credentials needed). Used as visual truth for design spec compliance.
- `make mdlint` — run markdownlint on docs (MUST pass locally before any push that touches .md files)
- `./a9s --demo` — run the app with synthetic fixture data (no AWS credentials needed)

## Prerequisites

- Go 1.26+ (`brew install go`)
- golangci-lint v2.11+ (`brew install golangci-lint`)
- govulncheck (`go install golang.org/x/vuln/cmd/govulncheck@latest`)
- markdownlint-cli2 (`brew install markdownlint-cli2`) — for markdown linting

## CodeRabbit (AI Code Reviewer)

- Use `@coderabbitai ignore` on PRs where you don't need further review
- Use `[skip ci]` in commit messages for trivial follow-ups (CodeRabbit still reviews unless ignored)
- CodeRabbit reviews are triggered per-push, not per-commit — batch small fixes into one push

## Architecture Principles

> **Full architecture guide**: [`docs/architecture.md`](docs/architecture.md) — covers all concepts, patterns, caching layers, key handling, test philosophy, and design decisions. Read it first when onboarding.
>
> ⚠️ **Related-resource panel is governed by [`docs/related-resources.md`](docs/related-resources.md) — SINGLE SOURCE OF TRUTH, DO NOT EDIT AD-HOC.** Every `RegisterRelated` call must match that contract. Adding/removing pivots requires an AWS API field citation or a documented DevOps workflow reason in the same PR that touches the registration.

- **Read-only by design** — a9s never makes write calls to AWS
- **Bubble Tea v2** — all I/O in `tea.Cmd` closures, views are pure functions
- **Message-driven** — views communicate via typed messages, never import each other
- **Single source of truth** — key bindings in `keys/keys.go`, types in `types.go`, styles in `styles/`, **related-panel contract in `docs/related-resources.md`**

## Skills and Subagents — in-session tooling, **not** Paperclip assignees

> **The two tables below describe Claude Code skills and subagents.** They are tools invoked from within an agent's Claude Code session. **They are not Paperclip company agents.** They have no heartbeats, cannot be assigned issues, and cannot sign off on PRs. The Paperclip roster (CEO, CTO, Architect, Coder, QA, E2ETester, DevOps, CodexReviewer, CodeReviewer) is the only set of names that can own a stage or sign off on a PR — see [`docs/development-process.md`](docs/development-process.md) §"Agents". When this file refers to an "agent" by an `a9s-*` / `tui-*` / `test-coverage-analyzer` id, it means a tool a Paperclip agent invokes — never an issue assignee.

## Skills

| Skill | Scope | Usage |
|-------|-------|-------|
| `a9s-common` | All agents | Shell rules, package access rules, build/test commands |
| `a9s-bt-v2` | TUI-touching agents | Bubble Tea v2 / Lipgloss v2 / Bubbles v2 API patterns |
| `a9s-add-resource` | Coder (steps 1-7), QA (steps 8-12) | Split blueprint: coder=implementation, QA=tests |
| `a9s-add-child-view` | Coder (phase 3), QA (phase 2) | Split blueprint: architect scopes, QA tests, coder implements |
| `a9s-add-related-view` | Coder (steps 1-6), QA (steps 7-11) | Split blueprint: add related-resource views per resource type |
| `a9s-implement-issue` | Architect (orchestrator) | End-to-end: analyze → QA stories → design → scope → implement → verify → docs → release |
| `a9s-resource-spec` | Main session | Generate `docs/resources/<shortName>.md` implementation-blind from the four golden docs |
| `a9s-implement-resource` | Main session (orchestrator) | Implement resource from its spec: TBDs → impl-plan → fixtures → QA + coder handoff |
| `a9s-create-demo-fixture` | Coder (during impl-resource 6a) | Build the single-source fixture file at `internal/demo/fixtures/<shortName>.go` — graph-connected, demo + tests share it |

## Agents

| Agent | Role | Writes to | Rejects without |
|-------|------|-----------|-----------------|
| `a9s-architect` | Scopes tasks, design decisions, interfaces | Nothing (design output only) | N/A (owns scoping) |
| `a9s-coder` | Implementation only — no tests | `internal/`, `cmd/`, `.a9s/` | Exact file scope from architect |
| `a9s-qa` | Tests only — no production code | `tests/unit/` | Exact file scope from architect |
| `a9s-tui-reviewer` | Code review — BT v2 correctness, design compliance | Nothing (read-only) | N/A |
| `a9s-qa-stories` | Given/when/then stories from design spec (no source code) | Nothing (read-only) | N/A |
| `a9s-integrator` | Cross-package wiring, message flow, app.go | `internal/tui/app.go`, `messages/` | N/A |
| `a9s-fixtures` | Test fixtures from dev-account via AWS MCP | `internal/demo/` | N/A |
| `test-coverage-analyzer` | Test suite analysis, coverage gaps | Nothing (read-only) | N/A |
| `tui-ux-auditor` | UX review, k9s comparison, design guidelines | Nothing (read-only) | N/A |
| `a9s-devops` | AWS practitioner — resource priorities, feature advice | All | N/A |
| `a9s-consistency-checker` | Verifies consistency across code, tests, README, website, config | Nothing (read-only) | N/A |

## Agent File Access Rules

Agents MUST use targeted file access — never broad globs on large directories.

### DO

- Use Explore agent wherever reasonable
- `Glob("internal/aws/{resource}*.go")` — find a specific fetcher
- `Glob("tests/unit/*{resource}*")` — find tests for a specific resource
- `Grep("mock.*{InterfaceName}", "tests/unit/mocks_test.go")` — find a specific mock
- `Glob("internal/demo/fixtures/*.go")` — find a per-service fixture file
- `Glob("internal/demo/fakes/*.go")` — find a typed-fake implementation
- `Grep("func Test.*{Resource}", "tests/unit/qa_detail_child_views_test.go")` — find append point

### DON'T

- `Glob("tests/unit/*.go")` — returns 148 files, most irrelevant
- `Glob("internal/aws/*.go")` — returns 77 files, most irrelevant
- `Glob("internal/demo/*.go")` — only 3 files remain (client.go, handlers.go, transport.go)
- Reading entire cross-cutting files (mocks_test.go, qa_detail_test.go) — grep for the section first

### Delegate to Explore for broad investigations

When a single task would require reading 5+ files totaling >500 lines, OR when you need to trace a feature across multiple packages (fetcher → view → related → test), dispatch an `Explore` agent and ask for a summarized report rather than reading everything into main context. Direct Grep/Glob/Read remain correct for targeted lookups (known file, specific symbol, < 3 queries). This protects the main context window for synthesis and decision-making.

#### Per-service fixture files are here (`internal/demo/fixtures/`)

## Rules

- ALWAYS rebuild binary (`make build`) after ANY code change — version is resolved at build time via `internal/buildinfo`
- Do not make any changes until you have 95%+ confidence in what you need to build. Ask me follow up questions until you reach that confidence
- TDD is non-negotiable: architect scopes both QA and coder tasks; QA writes tests, coder writes implementation. For rigid patterns (resource types, child views) they run in parallel. For novel features, QA goes first.
- ALWAYS test ALL resource types (S3, EC2, RDS, Redis, DocumentDB, EKS, Secrets Manager, VPC, SG, Node Groups, etc), not just one
- NEVER delete code, tests, or helpers just to make a linter happy. Understand WHY the code exists first. If it's genuinely dead, remove it. If it serves a purpose (scaffolding, crash-verification tests), use a targeted `//nolint` with a reason comment. If a linter rule produces widespread false positives, fix the rule in `.golangci.yml`.
- NEVER make multiple push-and-check cycles. Get it right locally, push once.
- BEFORE any push, the canonical gate is **`make ready-to-push`** — see [`docs/development-process.md`](docs/development-process.md) §"Stage 6 — Pre-push Validation" for the gate contents and the `internal/aws/` live-integration sub-rule. Stage 5 reviewers (Paperclip agents: **CodeReviewer**, **CodexReviewer**, **Architect** for size ≥ M, **CTO** as final) must sign off before this gate runs. Those reviewers invoke the subagent tools (`a9s-consistency-checker`, `test-coverage-analyzer`, `a9s-tui-reviewer`, `a9s-security-auditor`, `a9s-docs-reviewer`, `tui-ux-auditor`) in-session — see the §"Skills and Subagents" banner below.
- BEFORE any release, the canonical gate is **`make ready-to-release`** — see [`docs/development-process.md`](docs/development-process.md) §"Stage 7 — Merge & Release" for the manual checklist (`CHANGELOG.md`, `releases/vX.Y.Z.md`, `docs/architecture.md` alignment, busywork audit on tests added/modified in the release).
- **Exception**: Docs-only changes (`*.md`, `docs/`, `website/`, `specs/`, `.claude/`, `LICENSE`) skip `ready-to-push`; `make mdlint` is required.

## Docs Sync Rule

`docs/shared/` is the single source of truth for content shared between README and website.
- README is generated: edit `docs/README.tmpl.md` or `docs/shared/*.md`, then run `go run ./cmd/readmegen/ > README.md`
- Website uses Hugo `{{< include >}}` shortcodes that resolve to `docs/shared/` via module mount
- **Never edit README.md directly** — it will be overwritten by readmegen

When code changes affect any of the following, update the shared source and regenerate:
- Key bindings added/removed/changed → `docs/shared/keybindings.md`
- Child views added/removed → `docs/shared/childviews.md`
- Commands added/removed/changed → `docs/shared/commands.md`
- CLI flags changed → `docs/shared/quickstart.md`
- Install methods changed → `docs/shared/install.md`
- Resource types added/removed/renamed → `docs/README.tmpl.md` services table + `website/content/resources.md`
- Go version bumped → `docs/shared/install.md`, CONTRIBUTING.md

## Recent Changes

- 020-architecture-refactor: Added Go 1.26+ + Bubble Tea v2.0.2, Lipgloss v2.0.2, Bubbles v2, AWS SDK Go v2 (autoscaling, codeartifact, codebuild, codepipeline, dynamodb, ec2, ecr, ecs, efs, elasticbeanstalk, elbv2, events, iam, kms, lambda, rds, secretsmanager, ses, sesv2, sfn, sns, ssm, eventbridge, backup), yaml.v3, clipboard
