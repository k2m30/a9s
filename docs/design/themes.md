# Themes — Implementation Plan

**Status:** Roadmap
**Priority:** Medium
**Depends on:** None (standalone feature)

## Overview

Add configurable color themes to a9s. The current Tokyo Night Dark palette is hardcoded in `internal/tui/styles/palette.go`. This plan introduces a theme abstraction layer so users can switch between built-in themes or define custom ones via config.

## Current Architecture

```
palette.go    → 33 lipgloss.Color constants (hardcoded hex values)
styles.go     → 15 composed lipgloss.Style variables, init at startup
                RowColorStyle() cache for status→color mapping
                NO_COLOR env var support (zeroes all styles)
views/*.go    → import styles package, use composed styles directly
config.go     → loads views.yaml (column layout only, no theme config)
```

**Key observations:**
- Colors are Go constants — cannot be changed at runtime
- Composed styles are package-level `var`s, rebuilt via `Reinit()`
- `NO_COLOR` already proves the "reinit all styles" path works
- Config system already supports `~/.a9s/` lookup chain

## Design Goals

1. **Zero breaking changes** — default behavior unchanged, Tokyo Night Dark remains default
2. **Config-driven** — theme selection via `~/.a9s/config.yaml`
3. **Built-in themes** — ship 4-6 popular themes out of the box
4. **Custom themes** — users can define their own palette in YAML
5. **NO_COLOR preserved** — overrides any theme when set
6. **Testable** — theme switching covered by unit tests
7. **k9s parity** — k9s supports custom skins; a9s should too

## Theme Palette Structure

A theme defines all 33 color slots currently in `palette.go`. The YAML representation:

```yaml
# ~/.a9s/themes/my-theme.yaml
name: "My Custom Theme"

colors:
  # Structural
  header_fg:        "#c0caf5"
  accent:           "#7aa2f7"
  dim:              "#565f89"
  border:           "#414868"

  # Row selection
  row_selected_bg:  "#7aa2f7"
  row_selected_fg:  "#1a1b26"
  row_alt_bg:       "#1e2030"

  # Status (row coloring)
  running:          "#9ece6a"
  stopped:          "#f7768e"
  pending:          "#e0af68"
  terminated:       "#565f89"

  # Detail view
  detail_key:       "#7aa2f7"
  detail_val:       "#c0caf5"
  detail_sec:       "#e0af68"

  # YAML syntax
  yaml_key:         "#7aa2f7"
  yaml_str:         "#9ece6a"
  yaml_num:         "#ff9e64"
  yaml_bool:        "#bb9af7"
  yaml_null:        "#565f89"
  yaml_tree:        "#414868"

  # Help
  help_key:         "#9ece6a"
  help_cat:         "#e0af68"

  # UI feedback
  filter:           "#e0af68"
  success:          "#9ece6a"
  error:            "#f7768e"
  spinner:          "#7aa2f7"
  scroll:           "#414868"
  warning:          "#e0af68"

  # Key hints
  key_hint_key:     "#7aa2f7"
  key_hint_bg:      "#24283b"
  key_hint_fg:      "#565f89"

  # Overlays
  overlay_bg:       "#1a1b26"
  overlay_border:   "#7aa2f7"
```

## Built-in Themes

Ship these themes embedded in the binary:

| Theme | Source | Character |
|-------|--------|-----------|
| **Tokyo Night Dark** | Current palette (default) | Cool blues, soft contrast |
| **Tokyo Night Light** | tokyonight.com | Light variant — warm whites, blue accents |
| **Catppuccin Mocha** | catppuccin.com | Warm pastels, easy on eyes (dark) |
| **Catppuccin Latte** | catppuccin.com | Warm pastels, light background |
| **Dracula** | draculatheme.com | Purple accent, high contrast |
| **Nord** | nordtheme.com | Arctic blues, muted tones (dark) |
| **Nord Light** | nordtheme.com | Snow Storm palette, arctic light |
| **Gruvbox Dark** | github.com/morhetz/gruvbox | Retro warm, earthy tones |
| **Gruvbox Light** | github.com/morhetz/gruvbox | Retro warm, cream background |
| **Solarized Dark** | ethanschoonover.com/solarized | Precision colors, low contrast |
| **Solarized Light** | ethanschoonover.com/solarized | Same 16 colors, light background |

## Config Integration

Add a top-level `config.yaml` alongside the existing `views.yaml`:

```yaml
# ~/.a9s/config.yaml
theme: "tokyo-night"          # built-in theme name
# OR
theme: "~/.a9s/themes/my-theme.yaml"  # path to custom theme file
```

**Lookup chain** (same as views.yaml):
1. `.a9s/config.yaml` (per-project)
2. `~/.a9s/config.yaml` (user home)
3. Built-in defaults (tokyo-night)

## Implementation Phases

### Phase 1: Theme Abstraction Layer

**Goal:** Replace hardcoded constants with a `Theme` struct without changing any visual behavior.

**Files to create:**
- `internal/tui/styles/theme.go` — `Theme` struct with all 33 color fields, `DefaultTheme()` returning Tokyo Night values

**Files to modify:**
- `internal/tui/styles/palette.go` — convert constants to `var`s populated from active theme
- `internal/tui/styles/styles.go` — `initStyles()` reads from active theme, add `ApplyTheme(t Theme)`

**Acceptance criteria:**
- All existing tests pass with zero changes
- `DefaultTheme()` produces identical colors to current constants
- `ApplyTheme()` + `Reinit()` rebuilds all composed styles
- NO_COLOR still overrides everything

### Phase 2: Built-in Theme Definitions

**Goal:** Define 5-6 built-in themes as Go structs.

**Files to create:**
- `internal/tui/styles/themes/` directory
- `internal/tui/styles/themes/registry.go` — `BuiltinThemes` map, `Get(name) (Theme, bool)`
- `internal/tui/styles/themes/tokyo_night.go` (dark + light)
- `internal/tui/styles/themes/catppuccin.go` (mocha + latte)
- `internal/tui/styles/themes/dracula.go`
- `internal/tui/styles/themes/nord.go` (dark + light)
- `internal/tui/styles/themes/gruvbox.go` (dark + light)
- `internal/tui/styles/themes/solarized.go` (dark + light)

**Acceptance criteria:**
- Each theme (11 total) passes palette completeness check (all 33 slots filled)
- `Get("tokyo-night")` returns the default theme
- Light themes use appropriate contrast (dark text on light backgrounds)
- Unknown names return `false`

### Phase 3: Config Loading

**Goal:** Read theme selection from `config.yaml` and apply at startup.

**Files to create:**
- `internal/config/appconfig.go` — `AppConfig` struct with `Theme` field, `LoadAppConfig()` function

**Files to modify:**
- `internal/tui/app.go` — load app config at init, call `ApplyTheme()` before first render
- `internal/config/config.go` — share lookup chain logic

**Acceptance criteria:**
- `theme: "catppuccin"` in config switches colors
- Missing config file defaults to tokyo-night
- Invalid theme name logs warning, falls back to default

### Phase 4: Custom Theme Files

**Goal:** Allow user-defined YAML theme files.

**Files to modify:**
- `internal/config/appconfig.go` — detect file path vs built-in name, parse YAML theme file
- `internal/tui/styles/theme.go` — `ThemeFromYAML(data []byte) (Theme, error)` with validation

**Acceptance criteria:**
- `theme: "~/.a9s/themes/my-theme.yaml"` loads custom palette
- Missing color keys fall back to tokyo-night defaults (partial themes work)
- Invalid hex values produce clear error messages
- Example theme file shipped in `examples/themes/`

### Phase 5: Documentation and Tests

**Goal:** Full test coverage and user docs.

**Tests to write:**
- Theme struct completeness (no zero-value colors)
- Round-trip: Theme → YAML → Theme produces identical result
- Each built-in theme renders all view types without panics
- ApplyTheme swaps colors in composed styles
- Config loading with each theme name
- Partial custom theme merges with defaults
- NO_COLOR still wins over any theme

**Docs to update:**
- `README.md` — themes section in configuration
- `website/content/docs/_index.md` — theme configuration guide
- `docs/design/design.md` — note that palette is now theme-driven

## Architecture Diagram

```
┌─────────────────┐     ┌──────────────────┐
│  config.yaml    │────▶│  LoadAppConfig() │
│  theme: "nord"  │     └────────┬─────────┘
└─────────────────┘              │
                                 ▼
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  Built-in       │────▶│  ApplyTheme(t)   │────▶│  palette vars   │
│  themes/        │     │                  │     │  (33 colors)    │
│  registry.go    │     └────────┬─────────┘     └────────┬────────┘
└─────────────────┘              │                        │
                                 ▼                        ▼
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  Custom YAML    │────▶│  Reinit()        │────▶│  composed styles│
│  theme file     │     │  (rebuild all)   │     │  (15 styles)    │
└─────────────────┘     └──────────────────┘     └─────────────────┘
                                                          │
                                 ┌────────────────────────┘
                                 ▼
                        ┌──────────────────┐
                        │  Views render    │
                        │  using styles.*  │
                        └──────────────────┘
```

## Key Design Decisions

### Why not runtime theme switching (`:theme nord`)?

Not in initial scope. The `Reinit()` path supports it technically, but it requires:
- A command-mode command to switch themes
- Redrawing the entire screen after switch
- Persisting the choice back to config

This can be added later as a follow-up. For v1, theme is set at startup via config.

### Why a separate `config.yaml` instead of adding to `views.yaml`?

Separation of concerns:
- `views.yaml` = what data to show (columns, fields)
- `config.yaml` = how the app looks and behaves (theme, future settings like refresh interval, default region, etc.)

This also avoids breaking existing `views.yaml` files.

### Why partial themes merge with defaults?

Users shouldn't need to define all 33 colors to customize. If someone just wants to change the accent color, they should be able to:

```yaml
name: "My Accent"
colors:
  accent: "#ff0000"
```

All other colors inherit from tokyo-night. This lowers the barrier to customization.

### Why embed themes in the binary instead of shipping YAML files?

- Simpler distribution (single binary, no asset directory)
- No file-not-found errors for built-in themes
- Go structs are type-safe and validated at compile time
- Custom themes use YAML; built-in themes don't need the overhead

## Effort Estimate

| Phase | Scope | Files |
|-------|-------|-------|
| Phase 1 | Theme abstraction | 3 modified, 1 new |
| Phase 2 | Built-in themes (11 themes, 7 files) | 7 new files |
| Phase 3 | Config loading | 2 new, 2 modified |
| Phase 4 | Custom themes | 2 modified, 1 example |
| Phase 5 | Tests + docs | 3-5 test files, 3 docs |

## Open Questions

1. ~~**Should there be a light theme?**~~ **Resolved:** Yes — shipping 5 light variants (Tokyo Night Light, Catppuccin Latte, Nord Light, Gruvbox Light, Solarized Light).
2. **Theme preview command?** Something like `a9s --theme catppuccin` to preview without changing config. Low effort, nice UX.
3. **Per-status custom colors?** Allow themes to define custom status→color mappings beyond the standard 4 (running/stopped/pending/terminated). Useful but increases complexity.
