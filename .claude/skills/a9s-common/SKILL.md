---
name: a9s-common
description: Shared rules for all a9s agents — shell rules, package access, build/test commands
---

> Project rules (TDD, version bumps, lint, pre-push checklist, docs sync) live in `CLAUDE.md`. This skill covers agent-specific operational details only.

## Package Access Rules

- `core/aws/` — ADD new fetcher files. Do NOT modify existing fetchers.
- `core/aws/catalog_<category>.go` — ADD new `catalog.ResourceTypeDef` entries to the category slice only. Do NOT modify existing entries.
- `core/config/defaults_<category>.go` — ADD new default view definitions only. Do NOT modify existing entries.
- `core/fieldpath/` — FROZEN. Never modify.
- `internal/tui/` — Modify views, styles, layout, keys as needed; message types live in `core/runtime/messages/`.

## CI Path Filtering

CI and CodeQL workflows skip docs-only changes via `paths-ignore`. The website has its own deploy workflow
triggered by `website/**` changes. Release workflow triggers only on `v*` tags.

| Workflow | Trigger |
|----------|---------|
| CI (lint, test, build, security, verify-readonly, install-test) | Code changes to main or PRs |
| CodeQL | Code changes to main or PRs, plus weekly schedule |
| Deploy Website | `website/**` changes pushed to main |
| Release (GoReleaser) | `v*` tag push only |
