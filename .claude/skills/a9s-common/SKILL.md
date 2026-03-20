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
go run ./cmd/refgen/ > .a9s/views_reference.yaml    # regenerate views reference (after SDK changes)
```

## Pre-Push Checklist (MANDATORY)

Before ANY `git push`, ALL of these must pass locally:
1. `go build ./...`
2. `go test ./tests/unit/ -count=1 -timeout 120s`
3. `golangci-lint run ./...`

CI is NOT a debugging tool. Never push to see if CI passes. Fix locally first.

## Lint Fix Rules

- NEVER delete code just to satisfy a linter. Understand the code's purpose first.
- Dead code (genuinely unused, no callers) → remove it.
- Intentionally unused (crash-verification tests, scaffolding) → `//nolint:lintername // reason`
- If a linter rule produces widespread false positives → disable the rule in `.golangci.yml`, not per-line.
- NEVER blindly iterate (change `m` → `_` → `_, _`). Read the surrounding context, understand the intent, fix it right the first time.

## Version Bumping

After ANY code change, bump the version in `cmd/a9s/main.go` and rebuild:
```
const version = "X.Y.Z"
```
