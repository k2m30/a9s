# Plan: Surface Background Rendering for Themes

> Note: references to `styles.RowColorStyle` in this file are historical. Current API: `styles.ColorStyle(resource.Color)` via `ResourceTypeDef.ResolveColor(r)`. See `docs/architecture.md` Row Coloring.

## Context

Light themes look wrong because the app does not own the terminal background. Verified: Lipgloss v2's `Render()` sets outer bg once — inner `\x1b[m]` resets kill it for subsequent characters. Outer wrapping is unreliable. Per-fragment background is required at every composition site.

## Goal

Every visible character in the steady-state UI gets background from one of: `SurfaceStyle()`, `WithSurfaceBg(style)`, or an existing explicit bg (RowSelected, RowAlt, overlay, search). Transient pre-ready states ("Initializing...") may show plain text briefly.

---

## Part 1: Theme Model

**`internal/tui/styles/theme.go`**: Add `Background color.Color` (after `Name`). DefaultTheme: `"#1a1b26"`. Add YAML parsing.

**`internal/tui/styles/palette.go`**: Add `ColBg color.Color`. Wire in `applyPalette()`.

No Foreground field.

## Part 2: Background Helpers — `internal/tui/styles/styles.go`

No composed style changes. Add two helpers (both no-op when `NoColorActive()`):

```go
func SurfaceStyle() lipgloss.Style      // NewStyle().Background(ColBg)
func WithSurfaceBg(s lipgloss.Style) lipgloss.Style  // s.Background(ColBg)
```

No `WithSelectedBg` helper needed — selected rows in menu/selector already work because inner styles (DimText etc.) only set foreground, so the outer RowSelected bg persists. Detail selected rows use plain text + outer wrapper. Adding a selected-bg helper would change ANSI output without fixing any visible gap.

## Part 3: Header Kind Enum — `internal/tui/layout/` (NOT `tui/`)

Define the kind enum in the `layout` package to avoid import cycles (`tui` → `layout` already exists; `layout` cannot import `tui`):

**`internal/tui/layout/header.go`** (new file, or in frame.go):
```go
type HeaderRightKind int
const (
    HeaderHelp HeaderRightKind = iota
    HeaderFilter
    HeaderCommand
    HeaderSearch
    HeaderFlashSuccess
    HeaderFlashError
    HeaderRevealWarn
)
```

## Part 4: Header Refactor

**`internal/tui/app.go` — `headerRight()`** (line 679):
- Return `(string, layout.HeaderRightKind)` instead of styled `string`
- Each branch returns raw text + kind:
  - `modeFilter` → `("/" + m.cmdInput.Value(), layout.HeaderFilter)`
  - `modeCommand` → `(":" + m.cmdInput.Value(), layout.HeaderCommand)`
  - search active → `(info, layout.HeaderSearch)`
  - flash error → `(text, layout.HeaderFlashError)`
  - flash success → `(text, layout.HeaderFlashSuccess)`
  - reveal warning → `(rv.HeaderWarningText(), layout.HeaderRevealWarn)` (add `HeaderWarningText() string` to RevealModel)
  - default → `("? for help", layout.HeaderHelp)`

**`internal/tui/app.go` — `View()`** (line 521):
- Update call: `rightText, rightKind := m.headerRight()`
- Pass both to `layout.RenderHeader(..., rightText, rightKind)`
- Update `headerCacheKey` to include `rightText + rightKind` instead of `rightContent`

**`internal/tui/layout/frame.go` — `RenderHeader()`** (line 268):
- Accept `rightText string, rightKind HeaderRightKind` instead of `rightContent string`
- Style right side per kind, each with `WithSurfaceBg`:
  ```go
  switch rightKind {
  case HeaderHelp:         right = styles.WithSurfaceBg(styles.DimText).Render(rightText)
  case HeaderFilter, HeaderCommand, HeaderSearch:
                           right = styles.WithSurfaceBg(styles.FilterActive).Render(rightText)
  case HeaderFlashSuccess: right = styles.WithSurfaceBg(styles.FlashSuccess).Render(rightText)
  case HeaderFlashError:   right = styles.WithSurfaceBg(styles.FlashError).Render(rightText)
  case HeaderRevealWarn:   right = styles.WithSurfaceBg(styles.FlashError).Render(rightText)
  }
  ```
- Style left segments per-fragment:
  - `styles.WithSurfaceBg(lipgloss.NewStyle().Foreground(styles.ColAccent).Bold(true)).Render("a9s")`
  - `styles.WithSurfaceBg(lipgloss.NewStyle().Foreground(styles.ColDim)).Render(" v" + version)`
  - etc.
- Gap spaces: `styles.SurfaceStyle().Render(strings.Repeat(" ", gap))`
- Padding (from old `.Width(w).Padding(0,1)`): replace with explicit left/right padding via `styles.SurfaceStyle().Render(" ")`

## Part 5: Frame Rendering — `internal/tui/layout/frame.go`

**`CenterTitle()`** (line 38): Each segment individually:
- `WithSurfaceBg(borderStyle).Render("┌" + dashes + " ")`
- `WithSurfaceBg(titleStyle).Render(title)`
- `WithSurfaceBg(borderStyle).Render(" " + dashes + "┐")`

**`RenderFrame()` (line 82), `RenderFramePrepadded()` (line 126), `RenderFrameWithHints()` (line 224)**:
- Border: `WithSurfaceBg(borderStyle).Render("│")`
- Content: pass through as-is (views handle their own bg per Parts 6-11)
- Trailing padding: `SurfaceStyle().Render(strings.Repeat(" ", innerW-visW))`
- Empty lines: `SurfaceStyle().Render(strings.Repeat(" ", innerW))`

**`BottomBorderWithHints()`** (line 159):
- Preserve the current visual contract and width accounting:
  - each hint stays `key + " " + desc`
  - separators between hints stay `──`
- Apply bg per fragment without changing layout:
  - `WithSurfaceBg(keyStyle).Render(key)`
  - `SurfaceStyle().Render(" ")`
  - `WithSurfaceBg(descStyle).Render(desc)`
  - hint separators remain `WithSurfaceBg(borderStyle).Render("──")`
  - leading/trailing border runs remain `WithSurfaceBg(borderStyle).Render(...)`

## Part 6: Resource List — `views/table_render.go`, `views/resourcelist.go`

**`renderDataRow()`** (table_render.go:123):
- Non-selected: `base = styles.WithSurfaceBg(base)` before rendering cells
- Selected: `base` is `RowSelected` (has own bg), no change
- Keep `isSelected` trailing-pad guard — remove only after visual verification

**`resourcelist.go View()`** — table header row (line 383+):
- Header cells rendered with `TableHeader` — wrap: `styles.WithSurfaceBg(styles.TableHeader)`
- Column header padding: `styles.SurfaceStyle().Render(padding)`

**Loading / empty states** (resourcelist.go:384-405):
- `return m.spinner.View() + " Loading..."` and both `return "No resources found"` branches are steady-state list content once the view is mounted.
- Render them through surface-aware styles instead of leaving them plain, for example:
  - loading: surface-style the spinner fragment if possible, and surface-style the `" Loading..."` suffix
  - empty: `styles.WithSurfaceBg(styles.DimText).Render("No resources found")`

**"load more / loading" hint** (resourcelist.go:449-461):
- Line 460: `styles.WithSurfaceBg(styles.DimText).Render(hint)` — currently fg-only, leaks terminal bg on light themes.

## Part 7: Detail View — `views/detail_fields.go`

Lines 303-384 have TWO rendering branches:
- **Selected rows (lines 303-330)**: already render as PLAIN TEXT (no styled spans, no ANSI resets). The outer `Background(ColRowSelectedBg).Render(line)` at line 383 works correctly because there are no inner resets to kill the bg. **No changes needed.**
- **Non-selected rows (lines 332-371)**: use styled spans (DetailKey, DetailVal, NavigableField). These need per-fragment `WithSurfaceBg`.

**Non-selected rows** — every fragment gets `WithSurfaceBg`:
- `styles.SurfaceStyle().Render("     ")` for spacers (lines 343, 349, 356)
- `styles.WithSurfaceBg(styles.DetailKey).Render(key + ":")` (lines 343, 349, 356, 361, 363)
- `styles.WithSurfaceBg(styles.DetailVal).Render(val)` (lines 356, 358, 368)
- `styles.WithSurfaceBg(styles.NavigableField).Render(val)` (lines 343, 361)
- `styles.WithSurfaceBg(styles.RowColorStyle(tier)).Render(val)` (line 366)
- `styles.WithSurfaceBg(styles.DetailSection).Render(item.Key + ":")` (line 336)
- Leading space `" "`: `styles.SurfaceStyle().Render(" ")` (lines 334, 336, 361, 370)
- Inter-field gap `"  "`: `styles.SurfaceStyle().Render("  ")` (lines 343, 349)

**Selected rows** — keep the rendering path (lines 303-330 plain text + line 383 outer wrapper). One change: add NO_COLOR guard at line 383:
```go
if styles.NoColorActive() {
    line = lipgloss.NewStyle().Reverse(true).Render(line)
} else {
    line = lipgloss.NewStyle().Background(styles.ColRowSelectedBg).Render(line)
}
```
This is needed because the current `Background(ColRowSelectedBg)` emits ANSI bg even under NO_COLOR (it's an ad-hoc style, not a composed style gated by initStyles). The reverse-video branch matches RowSelected's NO_COLOR behavior (`tui_styles_test.go:230-246`).

Tests in `detail_selected_row_visibility_test.go` continue to pass: no key tint, no underline, has bg (or reverse under NO_COLOR).

**Detail side-by-side** (`detail_helpers.go:20`):
- Left column padding (line 55): `styles.SurfaceStyle().Render(strings.Repeat(" ", leftW-leftVisible))`
- Separator: `styles.WithSurfaceBg(ColSepDim/ColSepAccent style)` (line 27-29)

## Part 8: Right Column — `views/rightcolumn.go`

Lines 220-250:
- Non-selected rows (line 249): `styles.WithSurfaceBg(rowStyle).Render(rowText)`
- Selected rows (line 247): already uses `RowSelected.Width(m.width)` — no change
- Header "RELATED" (line 206): `styles.WithSurfaceBg(styles.DimText).Render(centeredHeader)`
- Status messages (lines 211-213): `styles.WithSurfaceBg(styles.DimText).Render(...)`
- Empty padding lines (line 256): `styles.SurfaceStyle().Render("")` or keep as `""`  (frame trailing-pad handles it)

## Part 9: Main Menu — `views/mainmenu.go`

**Category headers** (line 236-238): `styles.WithSurfaceBg(styles.DimText).Render(headerText)` — currently fg-only.

**Non-selected item rows** (line 293+):
- Name: `styles.WithSurfaceBg(nameStyle).Render(namePadded)`
- Alias: `styles.WithSurfaceBg(styles.DimText).Render(aliasPadded)`
- Leading spaces: `styles.SurfaceStyle().Render("    ")`
- Shortname: `styles.WithSurfaceBg(dimStyle).Render(shortname)`

**Selected rows** (line 267-269): NO CHANGES. `DimText.Render(alias)` inside `RowSelected.Width.Render()` works because DimText only sets foreground — it doesn't reset RowSelected's bg. The `\x1b[m` from DimText.Render is at the end, right before Lipgloss Width padding re-applies RowSelected style. No visible gap.

## Part 10: Selector — `views/selector.go`

**Non-selected** (line 122):
- `(current)` indicator at line 116: `styles.WithSurfaceBg(styles.DimText).Render("(current)")`
- Label: `styles.WithSurfaceBg(styles.RowNormal).Width(m.width).Render(label)`

**Selected** (line 120): NO CHANGES. `DimText.Render("(current)")` inside `RowSelected.Width.Render(label)` works — DimText only sets fg, outer RowSelected bg persists. No visible gap.

## Part 11: Help View — `views/help.go`

**Style aliases** (line 88-90): wrap with surface bg:
- `catStyle := styles.WithSurfaceBg(styles.HelpCatStyle)`
- `hkStyle := styles.WithSurfaceBg(styles.HelpKeyStyle)`
- `descStyle := styles.WithSurfaceBg(styles.HelpDescStyle)`

**`padCell()`** (line 92-96): the padding spaces at line 96 are raw `strings.Repeat(" ", ...)`. Change to:
```go
return s + styles.SurfaceStyle().Render(strings.Repeat(" ", w-visible))
```

**`bind()`** (line 92): uses `text.PadOrTrunc` which pads with spaces — those need bg. Do not wrap the final concatenated `bind()` string in `SurfaceStyle()`; that would still be outer wrapping around pre-rendered ANSI fragments. Render the padded key through `WithSurfaceBg(hkStyle)` with explicit width, then append `descStyle.Render(...)`.

**Leading spaces** (line 129 `" " + catRow`, line 146 `" " + ...`): `styles.SurfaceStyle().Render(" ") + catRow`

**`lipgloss.Place()`** calls (lines 158, 161): Place generates padding spaces that won't have bg. Replace with manual centering:
```go
padLeft := (m.width - lipgloss.Width(themeLine)) / 2
line := styles.SurfaceStyle().Render(strings.Repeat(" ", padLeft)) + themeLine + styles.SurfaceStyle().Render(strings.Repeat(" ", m.width-padLeft-lipgloss.Width(themeLine)))
```

**CT events legend** (`help_ct_legend.go:16-75`):
- Style aliases (line 17-18): `catStyle := styles.WithSurfaceBg(styles.HelpCatStyle)`, `descStyle := styles.WithSurfaceBg(styles.HelpDescStyle)`
- `verbStyle()` helper (line 20-26): add `.Background(styles.ColBg)` to the returned style (guarded by NoColorActive check, or use WithSurfaceBg)
- Leading spaces `" "` (lines 31, 35, 52, 58, 71): `styles.SurfaceStyle().Render(" ")`
- Inline fg styles at lines 42-48 (verbRows) and lines 70 (tintRows): wrap in `WithSurfaceBg`: `styles.WithSurfaceBg(row.style).Render(...)`
- `descStyle.Render(row.desc)` at lines 52 and 71: already covered by alias
- `text.PadOrTrunc` padding inside `row.style.Render(...)`: covered because the entire padded string goes through Render with bg

## Part 12: YAML View — `views/yaml.go`

**`colorizeYAML()`** (line 281+): style aliases at line 282-286:
- `keyStyle := styles.WithSurfaceBg(styles.YAMLKeyStyle)`
- Same for str/num/bool/null
- Tree connectors using `YAMLTree`: `styles.WithSurfaceBg(styles.YAMLTreeStyle)` (if exposed) or inline `WithSurfaceBg`
- YAML indentation (leading spaces): these are part of the rendered line. Since each line is a mix of styled spans + raw indent, the indent spaces need bg. Wrap the indent: `styles.SurfaceStyle().Render(indent)`

**`renderContent()`** (line 248-261):
- Empty-data path (line 256-257): `return styles.WithSurfaceBg(styles.DimText).Render("  No YAML data available")` — currently returns foreground-only styled text

**Viewport content**: `m.viewport.View()` returns the pre-styled content. The viewport itself doesn't add bg. Frame trailing-padding handles the right edge, but the left-side indent spaces need bg from `colorizeYAML()`.

## Part 13: Identity View — `views/identity.go`

**All three states** must be covered:

**`renderLoaded()`** (line 132):
- Style aliases (line 133-135): `secStyle := styles.WithSurfaceBg(styles.IdentitySectionStyle)`, etc.
- `line()` helper (line 139-143): spacers `"  "` → `styles.SurfaceStyle().Render("  ")`
- Section headers (lines 150, 159, 175): `styles.SurfaceStyle().Render("  ") + secStyle.Render("ACCOUNT")`
- `lipgloss.Place()` (line 185): replace with manual centering using `SurfaceStyle()` padding
- Newlines between sections: the `\n` characters don't carry bg, but the frame trailing-padding covers the right edge

**`renderLoading()`** (line 111-117):
- Line 114-115: `styles.SurfaceStyle().Render("  ") + styles.WithSurfaceBg(styles.DimText).Render("Fetching identity...")`

**`renderError()`** (line 120-129):
- Line 123-124: `styles.SurfaceStyle().Render("  ") + styles.WithSurfaceBg(styles.FlashError).Render("Error: " + m.errorMsg)`
- Line 126-127: `styles.SurfaceStyle().Render("  ") + styles.WithSurfaceBg(styles.DimText).Render("Press any key to close")`

## Part 14: Reveal View — `views/reveal.go`

**Pre-ready state** (line 60-61): `return "Initializing..."` — leave as plain text. Tests assert this exact string (`qa_reveal_test.go:33-40`, `qa_profile_update_test.go:394-397`). This is a transient state before SetSize, disappears immediately. Frame trailing-padding provides bg for the rest of the line.

**Ready state** (line 63): `m.viewport.View()` — viewport content is plain text. When setting content in the reveal model, apply `WithSurfaceBg` to each line so the viewport renders with theme bg. Frame trailing-padding covers the right edge.

## Part 15: YAML Theme Files

Add `background:` to all 11 built-in themes:

| Theme | `background` |
|-------|-------------|
| tokyo-night.yaml | `#1a1b26` |
| tokyo-night-light.yaml | `#e1e2e7` |
| catppuccin-mocha.yaml | `#1e1e2e` |
| catppuccin-latte.yaml | `#eff1f5` |
| dracula.yaml | `#282a36` |
| nord.yaml | `#2e3440` |
| nord-light.yaml | `#eceff4` |
| gruvbox-dark.yaml | `#282828` |
| gruvbox-light.yaml | `#fbf1c7` |
| solarized-dark.yaml | `#002b36` |
| solarized-light.yaml | `#fdf6e3` |

## Part 16: Tests

- Theme parsing: 36 fields (was 35), ColBg assertion in palette test
- `TestSurfaceBg_DetailSelectedRow_NoHoles` — selected detail line with Key+gap+Val, verify selection-bg ANSI at every character including gaps
- `TestSurfaceBg_DetailNormalRow_FullBg` — normal detail line, verify surface-bg at every character
- `TestSurfaceBg_MenuSelected_NoHoles` — menu selected with dim alias, full selection-bg
- `TestSurfaceBg_Header_LightTheme` — header with all segment types, no terminal-bg gaps
- `TestSurfaceBg_SelectorCurrent_NoHoles` — selector with (current) indicator, bg consistent
- `TestNoColor_NoBgEscapes` — NO_COLOR → both helpers return no-op, zero bg ANSI from SurfaceStyle/WithSurfaceBg; detail cursor uses reverse video not bg
- Regenerate golden files

## Verification

1. `make test` + `make lint` — green
2. `./a9s --demo` default theme — no regression
3. `theme: "gruvbox-light.yaml"` on dark terminal — full cream surface, zero dark gaps
4. Detail selected row — full highlight through Key/gap/Value
5. Main menu selected — full highlight through name/alias
6. Header flash/filter/help — consistent bg across all segments
7. Help view centered lines — bg fills full width
8. `:theme` dark↔light — immediate full re-render
9. `NO_COLOR=1 ./a9s --demo` — no bg escapes from surface helpers; detail cursor uses reverse video; selection in menu/selector uses RowSelected reverse

## Non-Goals

- No Foreground theme field
- No Background on any global composed style
- No speculative row-padding guard removal
- No changes to detail selected-row rendering path (lines 303-330) — only the NO_COLOR guard at line 383
- No changes to menu/selector selected-row rendering — existing fg-only inner styles work with outer RowSelected bg

## Files

| File | Changes |
|------|---------|
| `styles/theme.go` | +1 field, +1 YAML, +1 parse |
| `styles/palette.go` | +1 var, +1 assign |
| `styles/styles.go` | +2 helpers (SurfaceStyle, WithSurfaceBg). No composed style changes. |
| `layout/frame.go` | Per-segment bg in borders/padding/title/hints while preserving existing bottom-hint layout. RenderHeader refactored to accept raw text + kind. HeaderRightKind enum defined here. |
| `app.go` | headerRight() returns (string, layout.HeaderRightKind). View() updated. |
| `views/detail_fields.go` | Per-fragment WithSurfaceBg on non-selected rows only. Selected rows unchanged. NO_COLOR reverse-video fix at line 383. |
| `views/detail_helpers.go` | SurfaceBg on left-column padding and separator. |
| `views/table_render.go` | WithSurfaceBg(base) for non-selected rows. |
| `views/resourcelist.go` | WithSurfaceBg on table header, load-more hint, and loading/empty states. |
| `views/rightcolumn.go` | WithSurfaceBg on all non-selected rows and header. |
| `views/mainmenu.go` | WithSurfaceBg on non-selected rows only. Selected rows unchanged. |
| `views/selector.go` | WithSurfaceBg on non-selected rows + (current) indicator. Selected rows unchanged. |
| `views/help.go` | WithSurfaceBg aliases, bg-aware bind/padding, replace lipgloss.Place. |
| `views/help_ct_legend.go` | WithSurfaceBg aliases. |
| `views/identity.go` | WithSurfaceBg aliases, SurfaceStyle spacers, replace lipgloss.Place. |
| `views/yaml.go` | WithSurfaceBg aliases for colorizeYAML styles. |
| `views/reveal.go` | SurfaceStyle viewport content. |
| 11 YAML files | +`background` key |
| tests/ | Per-fragment bg verification, NO_COLOR, goldens |
