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
go run ./cmd/refgen/ > views_reference.yaml        # regenerate views reference (after SDK changes)
```

## Version Bumping

After ANY code change, bump the version in `cmd/a9s/main.go` and rebuild:
```
const version = "X.Y.Z"
```
