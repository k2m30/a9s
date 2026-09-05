---
paths:
  - "internal/tui/keys/**"
  - "core/aws/catalog_*.go"
  - "core/config/**"
  - "cmd/**"
  - "docs/shared/**"
  - "docs/README.tmpl.md"
  - "README.md"
  - "website/**"
  - "CONTRIBUTING.md"
---

# Docs sync

`docs/shared/` is the single source of truth for content shared between README and website.
- README is generated: edit `docs/README.tmpl.md` or `docs/shared/*.md`, then run `go run ./cmd/readmegen/ > README.md`
- Website uses Hugo `{{< include >}}` shortcodes that resolve to `docs/shared/` via module mount
- **Never edit README.md directly** — it will be overwritten by readmegen

When code changes affect any of the following, update the shared source and regenerate:
- Key bindings added/removed/changed → `docs/shared/keybindings.md`
- Child views added/removed → `docs/shared/keybindings.md` (child-view trigger keys) + `docs/design/child-views/`
- Commands added/removed/changed → `docs/shared/commands.md`
- CLI flags changed → `docs/shared/quickstart.md`
- Install methods changed → `docs/shared/install.md`
- Resource types added/removed/renamed → `docs/README.tmpl.md` services table + `website/content/resources.md`
- Go version bumped → `docs/shared/install.md`, CONTRIBUTING.md
- Keybindings, child views, IAM policy changed → **GitHub wiki** (see below)

GitHub wiki — out-of-repo surface, updated MANUALLY on release:
- The wiki pages (Key Bindings, Child Views, View Customization, Color Themes, Minimal IAM Profile, Environment Variables) have no repo-tracked source; **no gate catches their drift**
- Any keybinding, child-view, or IAM-policy change requires a manual wiki edit as part of the release
- README's keybindings/child-views links point at the website; the wiki remains authoritative only for the pages without a repo-tracked source
