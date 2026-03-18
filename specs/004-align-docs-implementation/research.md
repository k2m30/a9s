# Research: Align Documentation With Implementation

**Date**: 2026-03-18
**Method**: Automated code audit via agent exploration of all spec FRs, CLAUDE.md claims, agent definitions, and stale reference scan.

## Spec 001 — AWS TUI Manager (19 FRs)

| FR | Requirement | Status | Notes |
|----|-------------|--------|-------|
| FR-001 | TUI launch, full terminal | Implemented | Alt-screen mode via BT v2 |
| FR-002 | Read AWS profiles from config+credentials | Implemented | Reads both ~/.aws/config and ~/.aws/credentials |
| FR-003 | Persistent header (profile, region, version) | Implemented | layout.RenderHeader() |
| FR-004 | Colon-command with auto-suggestions | Partial | Commands work, but NO auto-suggestions/completion |
| FR-005 | Navigation commands | Implemented | All listed commands work |
| FR-006 | k9s-style keybindings | Partial | `y` shows YAML not JSON (spec says JSON); `[`/`]` history keys NOT implemented |
| FR-007 | Filter mode via `/` | Implemented | Real-time across all columns |
| FR-008 | Tabular resource display | Implemented | Resource-specific columns |
| FR-009 | Detail view, scrollable | Implemented | Via bubbles/viewport |
| FR-010 | S3 hierarchical browsing | Implemented | Bucket → objects → prefixes |
| FR-011 | Profile switching via `:ctx` | Implemented | Reloads AWS clients |
| FR-012 | Region switching via `:region` | Implemented | Pass-through to APIs |
| FR-013 | Breadcrumbs | Not implemented | Centered frame titles instead |
| FR-014 | Help overlay via `?` | Implemented | Context-sensitive per view |
| FR-015 | Back/forward history `[`/`]` | Not implemented | Only stack-based Escape pop |
| FR-016 | Graceful API error handling | Implemented | Flash error in header |
| FR-017 | Column sorting N/S/A | Implemented | Toggle asc/desc with indicators |
| FR-018 | Async API calls, manual refresh | Implemented | tea.Cmd + Ctrl-R |
| FR-019 | Read-only enforcement | Implemented | No write operations |

**Summary**: 15 implemented, 2 partial, 2 not implemented. Spec needs annotation for FR-004, FR-006, FR-013, FR-015.

## Spec 002 — Configurable Views (16 FRs)

| FR | Requirement | Status | Notes |
|----|-------------|--------|-------|
| FR-001 | Load list+detail from YAML | Implemented | |
| FR-002 | Lookup chain (cwd, env, home) | Implemented | |
| FR-003 | Fall back to defaults if no config | Implemented | |
| FR-004 | Partial config support | Implemented | |
| FR-005 | Error message on invalid YAML | Partial | Silently falls back, no user-facing error |
| FR-006 | All 8 resource types configurable | Implemented | |
| FR-007 | Column def: name, path, width | Implemented | |
| FR-008 | Detail: ordered paths, scalar+subtree | Implemented | |
| FR-009 | Reflection on AWS SDK structs | Implemented | |
| FR-010 | Auto-format (time, bool, numeric) | Implemented | |
| FR-011 | Ship views.yaml with defaults | Implemented | |
| FR-012 | Refgen tool exists | Implemented | cmd/refgen/ works |
| FR-013 | Ship views_reference.yaml artifact | Not implemented | Tool exists but file not in repo |
| FR-014 | Reference file: simple path list | N/A | File not shipped |
| FR-015 | Config columns sortable | Implemented | |
| FR-016 | Detail fallback to defaults | Implemented | |

**Summary**: 13 implemented, 1 partial, 1 not implemented, 1 N/A.

## Spec 003 — Fix UI Bugs (19 FRs)

| FR | Requirement | Status | Notes |
|----|-------------|--------|-------|
| FR-001 | Filter on main menu | Implemented | |
| FR-002 | S3 back-nav restores cursor | Not implemented | No cursor position stack |
| FR-003 | `y` key shows YAML | Implemented | |
| FR-004 | `c` copies ID in list, YAML in detail | Implemented | |
| FR-005 | Status bar at bottom | Implemented | |
| FR-006 | Status messages clear on nav | Partial | Clears on timeout, not explicitly on nav |
| FR-007 | "main" only on main menu | Implemented | |
| FR-008 | Resource count in frame title | Partial | Works for resource lists, S3 objects uncertain |
| FR-009 | Remove duplicate title line | Implemented | |
| FR-010 | Vertical scrolling all views | Implemented | |
| FR-011 | Horizontal scrolling all views | Implemented | |
| FR-012 | Horizontal scroll clamped | Partial | Left works, right can overflow |
| FR-013 | Horizontal scroll reset on nav | Not implemented | Scroll offset persists |
| FR-014 | Context-sensitive help | Implemented | |
| FR-015 | Header no blue bg, version right | Implemented | |
| FR-016 | Detail title no " - Detail" | Implemented | |
| FR-017 | `w` toggles word wrap | Implemented | |
| FR-018 | Detail YAML `Key: value` format | Implemented | |
| FR-019 | Config column widths applied | Implemented | |

**Summary**: 14 implemented, 2 partial, 2 not implemented.

## CLAUDE.md Audit

| Claim | Status | Issue |
|-------|--------|-------|
| Project Structure "src/ tests/" | WRONG | Actual: `cmd/`, `internal/`, `tests/`, `docs/`, `specs/` |
| Go 1.25+ | Correct | go.mod says 1.25.0 |
| "Go 1.22+" in Code Style | Stale | Should be Go 1.25+ |
| Build command | Correct | Works |
| Test command | Correct | Works |
| Refgen command | Correct | Works |
| Bubble Tea v2 | Correct | v2.0.2 |

## Agent Definitions Audit

| Agent | Status | Issue |
|-------|--------|-------|
| a9s-architect.md | Accurate | Old code correctly marked as forbidden |
| a9s-coder.md | Accurate | BT v2 API references correct |
| a9s-integrator.md | Partially stale | Describes cleanup steps already completed |
| a9s-pm.md | Accurate | |
| a9s-qa.md | Accurate | |
| a9s-qa-stories.md | Accurate | |
| a9s-fixtures.md | Accurate | |
| a9s-tui-reviewer.md | Accurate | |
| test-coverage-analyzer.md | Accurate | |
| tui-ux-auditor.md | INACCURATE | Line 20 references "src/" — should be "internal/" |

## Stale Reference Scan

| File | Reference | Severity |
|------|-----------|----------|
| CLAUDE.md (lines 15-16) | "src/" directory | Medium — doesn't exist |
| tui-ux-auditor.md (line 20) | "src/" directory | Medium — agent looks in wrong place |
| docs/rewrite-tasks.md | "internal/app/" | OK — historical context (rewrite tracking) |
| docs/architecture-review-v2.md | "internal/app/" | OK — historical context (review of old code) |
| specs/001/tasks.md | "internal/app/" | OK — historical context (completed tasks) |

## Decisions

- Decision: Annotate each spec FR inline with status suffix
  - Rationale: Clarified during /speckit.clarify — user chose inline text suffix format
  - Alternatives: Separate verification table (rejected — harder to maintain)

- Decision: Spot-check QA stories for known-changed areas only
  - Rationale: 350KB of stories; full audit has diminishing returns
  - Alternatives: Full line-by-line audit (rejected — 10x effort, low marginal value)

- Decision: Unimplemented features get "Future Work" section in their spec
  - Rationale: Preserves context for later without misleading current readers
  - Alternatives: Delete entirely (rejected — loses design context)
