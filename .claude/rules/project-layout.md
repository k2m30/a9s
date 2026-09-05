---
paths:
  - "core/**/*.go"
  - "internal/**/*.go"
  - "cmd/**/*.go"
  - "tests/**/*.go"
---

# Project layout

Where each package sits and what it is allowed to know. Loaded when Go source is in play.

```text
cmd/
  a9s/           # main binary (TUI, --demo, --web)
  readmegen/     # README.md generator from docs/README.tmpl.md + docs/shared/
  refgen/        # views_reference.yaml generator
  viewsgen/      # .a9s/views/*.yaml generator from core/config defaults
  preview/       # static TUI design mockups (no AWS)
  catalogen/     # catalog codegen
  snapshot/      # web-e2e snapshot collector
  checklist/     # web-e2e checklist oracle
core/            # platform-agnostic core — importable by external modules; dual-licensed (GPL-3.0-or-later OR commercial)
  app/           # headless controller — shared list/detail/menu/cost state+render for tui/ and web/
  aws/           # AWS service clients, fetchers, related checkers, enrichers, catalog_<category>.go type defs
  buildinfo/     # version resolution from ldflags / go install
  cache/         # per-type on-disk availability cache (TypeFile schema v2, no TTL)
  catalog/       # canonical resource catalog (ResourceTypeDef, installed via aws.Install)
  config/        # YAML config loading, per-category view defaults (defaults_<category>.go)
  costs/         # Cost Explorer domain state machine
  demo/          # synthetic data for demo mode (fixtures/ + fakes/)
  domain/        # leaf types: Resource, Finding, AttentionDetail, Severity, Color
  fieldpath/     # struct field extraction via reflection (frozen)
  jsonyaml/      # renderer-free JSON→YAML helpers
  resource/      # backward-compat alias layer over domain/ + catalog/
  runtime/       # platform-agnostic app core (Core) + messages/ Cmd/Event taxonomy
  semantics/     # projection, ctevent, selector helpers
  session/       # session.Session — session-scoped state, RowStore, Rotate()
  web/           # web mode HTTP server (internal-only)
internal/
  tui/           # Bubble Tea adapter shell — GPL-3.0-or-later only
    keys/        # key bindings (including child-view triggers: e, L, r, s)
    layout/      # frame rendering
    styles/      # Tokyo Night Dark palette + themes/*.yaml
    views/       # view models (menu, list, detail, yaml, help, etc.)
tests/
  unit/          # unit tests
  integration/   # integration tests
docs/
  design/        # visual design spec (incl. child-views/ with 24 view levels)
  qa/            # QA user stories
specs/           # feature specifications
```
