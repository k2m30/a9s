# Implementation Plan: Fix UI Bugs

**Branch**: `003-fix-ui-bugs` | **Date**: 2026-03-16 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/003-fix-ui-bugs/spec.md`

## Summary

Fix 15 UI bugs across the a9s TUI application: filter on main menu, S3 navigation position preservation, YAML rendering, context-aware copy, status bar positioning, breadcrumbs, scrolling, help screen, header styling, detail view improvements, and config column width application.

## Technical Context

**Language/Version**: Go 1.25+
**Primary Dependencies**: Bubble Tea v2, lipgloss v2, yaml.v3, clipboard
**Testing**: Standard Go `testing`, TDD mandatory (tests FIRST)
**Target Platform**: macOS/Linux terminal
**Project Type**: CLI/TUI application
**Constraints**: All fixes must be backward compatible. All 8 resource types must work. Existing tests must not break.

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. TDD (NON-NEGOTIABLE) | PASS | Tests written FIRST for every bug fix. User explicitly demanded this. |
| II. Code Quality | PASS | Bug fixes only — no new abstractions, no scope creep. |
| III. Testing Standards | PASS | Each bug gets at least one test that would have caught it. |
| IV. UX Consistency | PASS | Fixes align existing UI to consistent behavior patterns. |
| V. Performance | PASS | No performance-impacting changes. Scroll clamping is O(1). |

## Files to Modify

All changes are in existing files — no new packages.

```text
internal/app/app.go          # Main app: filter, navigation, breadcrumbs, status, scroll, copy, header
internal/views/detail.go     # Detail view: title, wrap toggle, YAML formatting, horizontal scroll
internal/views/jsonview.go   # Rename/convert to YAML view
internal/ui/header.go        # Header bar styling, version placement
internal/ui/help.go          # Context-sensitive help
internal/ui/statusbar.go     # Status bar positioning
internal/ui/breadcrumbs.go   # Breadcrumb formatting, "main" removal, count
internal/app/keys.go         # Add 'w' key binding for wrap toggle
```

## Bug-to-File Mapping

| Bug # | Description | Primary File(s) |
|-------|-------------|-----------------|
| 1 | Filter on main menu | app.go |
| 2 | S3 back navigation position | app.go |
| 3 | Y key → YAML | app.go, jsonview.go → yamlview |
| 4 | Context-aware copy | app.go |
| 5 | Status bar at bottom | app.go (View method) |
| 6 | Status clear on nav | app.go |
| 7 | Breadcrumbs no "main" | app.go, breadcrumbs.go |
| 8 | All views scrollable | detail.go, app.go |
| 9 | Scroll clamp | app.go |
| 10 | Scroll reset on nav | app.go |
| 11 | Context help | help.go, app.go |
| 12 | Header styling | header.go, app.go |
| 13 | Detail improvements | detail.go, app.go |
| 14 | Count in breadcrumbs | app.go, breadcrumbs.go |
| 15 | Config widths | app.go (renderResourceList) |
