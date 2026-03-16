# Research: Fix UI Bugs

No unknowns. All 15 bugs are in existing code with clear expected behavior. No new dependencies needed.

## Key Decisions

- **YAML view**: Convert existing `jsonview.go` to render YAML using `yaml.Marshal` with `toSafeValue` (already exists in `fieldpath/extract.go`). Keep the view model name generic.
- **Scroll clamping**: Clamp `HScrollOffset` at assignment time, not at render time. This prevents accumulating dead offset.
- **Navigation position**: Store `SelectedIndex` in the navigation history stack before pushing a new view. Restore on pop.
- **Status clear**: Clear `StatusMessage` in every navigation action (Enter, Escape, command execution).
- **Config widths**: The `renderResourceList` config-driven path already uses `col.Width` from config — bug is likely in the legacy fallback path or the width expansion logic overriding configured widths.
