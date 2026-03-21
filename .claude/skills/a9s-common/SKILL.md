---
name: a9s-common
description: Shared rules for all a9s agents — shell rules, package access, build/test commands
---

## Shell Rules

- NEVER chain commands with `&&`, `;`, `|`, or `cd` — use single standalone commands with absolute paths
- NEVER use subshell expressions like `$(...)` or backtick substitution
- NEVER use interactive flags like `-i`, `read -p`, `select`
- When intermediate results are needed, write output to /tmp files and read them in subsequent commands

## Package Access Rules

- `internal/aws/` — ADD new fetcher files. Do NOT modify existing fetchers.
- `internal/resource/types.go` — ADD new ResourceTypeDef entries only. Do NOT modify existing entries.
- `internal/config/defaults.go` — ADD new default view definitions only. Do NOT modify existing entries.
- `internal/fieldpath/` — FROZEN. Never modify.
- `internal/tui/` — Modify views, styles, layout, keys, messages as needed.

## Build & Test Commands

```
go build -o a9s ./cmd/a9s/                        # build binary
go test ./tests/unit/ -count=1 -timeout 120s       # run all unit tests
golangci-lint run ./...                            # lint (MUST pass before any push)
govulncheck ./...                                  # vulnerability check (MUST pass before any push)
go run ./cmd/refgen/ > .a9s/views_reference.yaml    # regenerate views reference (after SDK changes)
```

## Pre-Push Checklist (MANDATORY)

Before ANY `git push`, ALL of these must pass locally:
1. `go build ./...`
2. `go test ./tests/unit/ -count=1 -timeout 120s`
3. `golangci-lint run ./...`
4. `govulncheck ./...`

CI is NOT a debugging tool. Never push to see if CI passes. Fix locally first.

**Before pushing, also run these agents:**
5. `a9s-consistency-checker` — verify code/docs/website alignment (must be all PASS, no FAIL)
6. `test-coverage-analyzer` — check for test coverage gaps
7. `a9s-architect` — verify architecture against `docs/go-codebase-checklist.md` (target: 8.5+/10)

**Exception**: Docs-only changes (*.md, docs/, website/, specs/, .claude/, LICENSE) do NOT trigger CI.
The pre-push checklist is only required when Go source code, go.mod, go.sum, .golangci.yml, Makefile,
Dockerfile, .goreleaser.yaml, or .github/workflows/ files are modified.

## CI Path Filtering

CI and CodeQL workflows skip docs-only changes via `paths-ignore`. The website has its own deploy workflow
triggered by `website/**` changes. Release workflow triggers only on `v*` tags.

| Workflow | Trigger |
|----------|---------|
| CI (lint, test, build, security, verify-readonly, install-test) | Code changes to main or PRs |
| CodeQL | Code changes to main or PRs, plus weekly schedule |
| Deploy Website | `website/**` changes pushed to main |
| Release (GoReleaser) | `v*` tag push only |

## Lint Fix Rules

- NEVER delete code just to satisfy a linter. Understand the code's purpose first.
- Dead code (genuinely unused, no callers) → remove it.
- Intentionally unused (crash-verification tests, scaffolding) → `//nolint:lintername // reason`
- If a linter rule produces widespread false positives → disable the rule in `.golangci.yml`, not per-line.
- NEVER blindly iterate (change `m` → `_` → `_, _`). Read the surrounding context, understand the intent, fix it right the first time.

## Docs Sync Rule

When code changes affect any of the following, you MUST update README.md AND the website (`website/content/`) in the same PR:
- Resource types added/removed/renamed → update README services table + `website/content/resources.md`
- Key bindings added/removed/changed → update README key bindings tables + `website/content/docs/_index.md`
- Commands added/removed/changed → update README commands table + `website/content/docs/_index.md`
- CLI flags changed → update README Quick Start + `website/content/install.md`
- Install methods changed → update README Installation + `website/content/install.md`
- Go version bumped → update README, CONTRIBUTING.md, `website/content/install.md`

## Version Bumping

After ANY code change, bump the version in `cmd/a9s/main.go` and rebuild:
```
const version = "X.Y.Z"
```
