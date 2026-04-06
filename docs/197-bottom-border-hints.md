# Feature Specification: Bottom Border Key Hints (#197, closes #190)

**Issue**: https://github.com/k2m30/a9s/issues/197
**Also closes**: https://github.com/k2m30/a9s/issues/190
**Created**: 2026-04-06
**Status**: Ready for implementation

## Context

As a9s grows (60+ resource types, child views, related views), key discoverability is a real problem. Non-obvious bindings (`L` Logs, `x` Reveal, `d` Detail when enter is overridden, `ctrl+r` refresh) go undiscovered — users must press `?` to find them. This feature embeds view-specific key hints into the bottom frame border line, costing zero extra lines.

This also fully satisfies #190 (feedforward navigation hints in detail header). The bottom border dynamically shows `enter VPC` when the detail cursor lands on a navigable field — a separate header hint would be pure duplication.

## Design Principle: Show What's Non-Obvious

`enter` is the primary action key. Its meaning changes per context, and the bottom hints must reflect what it does RIGHT NOW:

- **Resource list, no enter-child** (EC2, VPC, EKS...): `enter` = detail → don't show `d Detail` (redundant)
- **Resource list, has enter-child** (S3, RDS, ELB, SNS...): `enter` is overridden → show `enter {Child}` + `d Detail` (now the only way to detail)
- **Detail view, cursor on navigable field**: `enter` navigates → show `enter {TargetType}`
- **Detail view, cursor on plain field**: `enter` does nothing → don't show it
- **Detail view, right column focused**: `enter` opens related → show `enter {SelectedType}`

Standard/intuitive keys (`j/k`, `g/G`, `/`, `?`, `q`, `esc`) are NEVER shown — they're universal.

`ctrl+r` (refresh) is a global key handled at the root level — it works on main menu (restart availability checks), resource list (re-fetch resources), and detail (re-trigger related checks). It's non-obvious and should appear in hints for all three views.

---

## Visual Spec

```
└──enter Objects──d Detail──y YAML──ctrl+r Refresh──────────┘
   ^^^^^^^^^^^^   ^^^^^^^^   ^^^^^^  ^^^^^^^^^^^^^^
   Key=AccentBold Key       Key      Key
       Desc=Dim       Desc      Desc       Desc
```

- Key character(s): `ColAccent` (`#7aa2f7`) bold
- Description text: `ColDim` (`#565f89`)
- Dashes and corners: `ColBorder` (`#414868`)
- Separator between hints: `──` (2 border dashes minimum)
- Hints that don't fit at current width are dropped from the right (never overflow/wrap)
- Corner characters `└` and `┘` always present

Width behavior:
- 60 cols (minimum): 1-2 hints or plain border if none fit
- 80 cols: ~3-4 hints
- 120+ cols: all hints fit

---

## Architecture

### New type: `layout.KeyHint`

```go
// KeyHint is a single key+description hint for the bottom border.
type KeyHint struct {
    Key  string // "r", "d", "esc", "enter"
    Desc string // "Related", "Detail", "Back", "Objects"
}
```

### New functions in `layout` package

```go
// BottomBorderWithHints renders the bottom border line with embedded key hints.
// Hints are placed left-to-right after └──. Hints that don't fit at width w
// are dropped from the right. Empty/nil hints produces a plain └───┘ border.
func BottomBorderWithHints(hints []KeyHint, w int) string

// RenderFrameWithHints is like RenderFrame but uses BottomBorderWithHints
// for the bottom border. When hints is nil/empty, identical to RenderFrame.
func RenderFrameWithHints(lines []string, title string, hints []KeyHint, w, h int) string
```

`RenderFrame` and `RenderFramePrepadded` signatures are unchanged. `RenderFrameWithHints` is a new function — avoids breaking `twocolumn.go` (2 internal calls) and 13 test call sites.

### New interface: `views.Hintable`

```go
// Hintable is an optional interface for views that provide bottom border key hints.
type Hintable interface {
    BottomHints() []layout.KeyHint
}
```

Not added to the `View` interface — views that don't benefit (Help, Identity, Reveal, Selector) simply don't implement it. `app.go` type-asserts.

### Plumbing in `app.go`

In `View()` (~line 471), replace `layout.RenderFrame(...)` with:

```go
var hints []layout.KeyHint
if h, ok := active.(views.Hintable); ok {
    hints = h.BottomHints()
}
frame := layout.RenderFrameWithHints(lines, frameTitle, hints, m.width, frameHeight)
```

---

## Complete View Hint Tables

### Main Menu

Most keys are standard (enter=select, j/k, /). `ctrl+r` is the only non-obvious key.

| State | Bottom hints |
|-------|-------------|
| Normal | `ctrl+r Refresh` |

### Resource List (top-level, escPops=false)

Hint construction pseudocode:
```
hints = []
enterChild = first child where Key == "enter"
if enterChild != nil:
    hints += {enter, childDisplayName}
    hints += {d, Detail}
if hasReveal:
    hints += {x, Reveal}
hints += {y, YAML}
for each child where Key != "enter":
    hints += {child.Key, childLabel}   // e.g. "e Events", "L Logs", "R Resources"
hints += {ctrl+r, Refresh}
if paginatedAndTruncated:
    hints += {m, More}
```

NOTE: `r` in the resource list is a **child view trigger** (`keys.Resources` → `handleChildKey("r")`), NOT the related panel toggle. The related panel (`r` / `ToggleRelated`) exists only in detail view. Currently no resource types define `ChildViewDef` with `Key: "r"`, so if one is added in the future, the child key loop will pick it up automatically.

| Resource type example | Enter behavior | Bottom hints |
|-----------------------|---------------|--------------|
| EC2 (no enter-child) | enter=detail | `y YAML`, `ctrl+r Refresh` |
| VPC (no enter-child) | enter=detail | `y YAML`, `ctrl+r Refresh` |
| S3 (enter→objects) | enter=child | `enter Objects`, `d Detail`, `y YAML`, `ctrl+r Refresh` |
| ELB (enter→listeners) | enter=child | `enter Listeners`, `d Detail`, `y YAML`, `ctrl+r Refresh` |
| ECS-svc (enter→tasks, e/L children) | enter=child | `enter Tasks`, `d Detail`, `y YAML`, `e Events`, `L Logs`, `ctrl+r Refresh` |
| CFN (enter→events, R resources) | enter=child | `enter Events`, `d Detail`, `y YAML`, `R Resources`, `ctrl+r Refresh` |
| Secrets (no enter-child, has reveal) | enter=detail | `x Reveal`, `y YAML`, `ctrl+r Refresh` |
| RDS (enter→events) | enter=child | `enter Events`, `d Detail`, `y YAML`, `ctrl+r Refresh` |

### Resource List (child/related, escPops=true)

Same algorithm but prepend `esc Back`:

| Context | Bottom hints |
|---------|-------------|
| Simple child list (e.g. S3 objects) | `esc Back`, `y YAML`, `ctrl+r Refresh` |
| Child with own enter-child (rare) | `esc Back`, `enter {Child}`, `d Detail`, `y YAML`, `ctrl+r Refresh` |

### Detail View (cursor-dependent)

```
hints = []
if rightCol.IsFocused():
    selectedType = rightCol.SelectedTypeName()
    if selectedType != "":
        hints += {enter, selectedType}
    hints += {tab, Fields}
    hints += {y, YAML}
    hints += {ctrl+r, Refresh}
    return hints

if fieldList != nil && fieldCursor in range && fieldList[fieldCursor].IsNavigable:
    targetType = fieldList[fieldCursor].TargetType
    displayName = lookupDisplayName(targetType)
    hints += {enter, displayName}

hints += {y, YAML}
if hasRelated:
    hints += {r, Related}
    if rightColVisible:
        hints += {tab, Cols}
hints += {ctrl+r, Refresh}
hints += {w, Wrap}
return hints
```

| Detail state | Bottom hints |
|-------------|-------------|
| Plain field, no related | `y YAML`, `ctrl+r Refresh`, `w Wrap` |
| Plain field, has related | `y YAML`, `r Related`, `ctrl+r Refresh`, `w Wrap` |
| Navigable field (e.g. VpcId), no related | `enter VPC`, `y YAML`, `ctrl+r Refresh`, `w Wrap` |
| Navigable field, has related | `enter VPC`, `y YAML`, `r Related`, `ctrl+r Refresh`, `w Wrap` |
| Related panel visible, left focused | `enter {Target}*`, `y YAML`, `r Related`, `tab Cols`, `ctrl+r Refresh`, `w Wrap` |
| Right column focused, row selected | `enter {SelectedType}`, `tab Fields`, `y YAML`, `ctrl+r Refresh` |
| Right column focused, no selection | `tab Fields`, `y YAML`, `ctrl+r Refresh` |

\* Only when cursor is on a navigable field.

### YAML View

| State | Bottom hints |
|-------|-------------|
| Normal | `w Wrap`, `c Copy` |

### Not Hintable

- **Help**: renders own `"Press any key to close"` inline
- **Identity**: renders own `"Press any key to close | c to copy ARN"` inline
- **Reveal**: renders own close hint inline
- **Selector**: all keys are standard (enter, j/k, /)

---

## Display Name Resolution

Child and target type names are resolved for hint descriptions:

```go
func hintDisplayName(shortName string) string {
    // Top-level resource types
    if rt := resource.FindResourceType(shortName); rt != nil {
        return rt.Name  // e.g. "EC2 Instances", "VPC"
    }
    // Child-only types
    if ct := resource.GetChildType(shortName); ct != nil {
        return ct.Name  // e.g. "S3 Objects", "Log Streams"
    }
    return shortName  // fallback
}
```

Names may be long ("EC2 Instances" = 13 chars). Consider using `ShortName` or a truncated `Name` when width is tight. Or just let the width-truncation algorithm drop hints naturally.

---

## Files to modify

| File | What changes |
|------|-------------|
| `internal/tui/layout/frame.go` | Add `KeyHint` struct, `BottomBorderWithHints()`, `RenderFrameWithHints()` |
| `internal/tui/layout/twocolumn.go` | **No changes** |
| `internal/tui/views/view.go` | Add `Hintable` interface |
| `internal/tui/app.go` (~line 471) | Type-assert to `Hintable`, call `RenderFrameWithHints` |
| `internal/tui/views/resourcelist.go` | Implement `BottomHints()` — enter-aware, state-dependent |
| `internal/tui/views/detail.go` | Implement `BottomHints()` — cursor-aware, right-column-aware |
| `internal/tui/views/yaml.go` | Implement `BottomHints()` |
| `internal/tui/views/mainmenu.go` | Implement `BottomHints()` — `ctrl+r Refresh` only |
| `tests/unit/layout_frame_test.go` | Tests for `BottomBorderWithHints` |

## Existing code to reuse

| What | Where | Used for |
|------|-------|----------|
| `styles.ColAccent`, `ColDim`, `ColBorder` | `internal/tui/styles/palette.go` | Hint rendering colors |
| `lipgloss.Width()` | lipgloss | Measuring rendered hint widths for truncation |
| `resource.GetRelated(shortName)` | `internal/resource/related.go` | Conditional `r Related` hint (detail view only) |
| `resource.HasRevealFetcher(shortName)` | `internal/resource/registry.go` | Conditional `x Reveal` hint |
| `resource.FindResourceType(name)` | `internal/resource/types.go` | Top-level type display names |
| `resource.GetChildType(name)` | `internal/resource/registry.go` | Child-only type display names |
| `m.typeDef.Children` | `ResourceTypeDef.Children` | Available child view keys per type |
| `m.fieldList[cursor].IsNavigable/.TargetType` | `fieldpath.FieldItem` | Detail cursor navigation target |
| `m.rightCol.IsFocused()/.SelectedTypeName()` | `rightcolumn.go` | Right column enter target |
| `m.escPops` | `ResourceListModel` | Child/related list detection |
| `m.pagination` | `ResourceListModel` | Truncated pagination detection |

## Import safety

`views → layout` for `layout.KeyHint`: **safe, no cycle**. `layout` imports only `styles` + stdlib. `views` already imports `styles`; adding `layout` follows the same pattern.

---

## Width-Aware Ordering Rationale

Hints are ordered left-to-right by importance. When the terminal is too narrow, rightmost hints are dropped first. This ensures:

1. **Escape route** (`esc Back`) — always visible in child/related lists
2. **Enter disambiguation** (`enter {Child}`, `d Detail`) — always visible when enter is overridden
3. **Core actions** (`y YAML`; detail view adds `r Related`) — visible at 80+ cols
4. **Child keys and operations** (`e Events`, `L Logs`, `R Resources`, `ctrl+r Refresh`) — visible at 100+ cols
5. **Auxiliary** (`x Reveal`, `m More`, `w Wrap`, `c Copy`) — visible at 120+ cols

---

## Test Plan

### `BottomBorderWithHints` unit tests

| Test case | Input | Expected |
|-----------|-------|----------|
| Empty hints | `nil, 40` | Plain `└──...──┘` border, same as current |
| Single hint | `[{y, YAML}], 40` | `└──y YAML──...──┘` |
| Multiple hints | `[{d, Detail}, {y, YAML}], 60` | `└──d Detail──y YAML──...──┘` |
| Exact fit | hints that fill exactly `w` | No trailing dashes, just `┘` |
| Truncation | 5 hints at `w=40` | First N hints that fit, rest dropped |
| Very narrow | `[{y, YAML}], 10` | Just `└──...──┘` if hint doesn't fit |
| Key styling | verify AccentBold on key, Dim on desc | ANSI width matches visual width |
| Corner invariant | any input | Always starts with `└`, ends with `┘`, total visual width == `w` |

### Integration smoke tests (manual, `--demo` mode)

See verification list at end of plan file.
