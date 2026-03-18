# Implementation Plan: Align Documentation With Implementation

**Branch**: `004-align-docs-implementation` | **Date**: 2026-03-18 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/004-align-docs-implementation/spec.md`

## Summary

Update all project documentation to match the current v1.2.0 implementation. Three feature specs (001-003) need FR-by-FR verification annotations and status updates. CLAUDE.md has a wrong project structure. Two agent definitions have stale references. QA stories need targeted spot-checks in known-changed areas. No code changes — documentation only.

## Technical Context

**Language/Version**: Go 1.25.0 (go.mod)
**Primary Dependencies**: Bubble Tea v2.0.2, Lipgloss v2.0.2, AWS SDK Go v2
**Storage**: N/A (documentation only)
**Testing**: `go test ./tests/unit/ -count=1 -timeout 120s` (1,045 tests verify current behavior)
**Target Platform**: macOS (development), documentation artifacts
**Project Type**: CLI / TUI application
**Performance Goals**: N/A
**Constraints**: No code changes; documentation only
**Scale/Scope**: ~20 documentation files, ~54 FRs to annotate, ~5 files to fix

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Applicable? | Status |
|-----------|-------------|--------|
| I. TDD | No — no production code changes | N/A |
| II. Code Quality | No — no code changes | N/A |
| III. Testing Standards | No — no new tests needed | N/A |
| IV. UX Consistency | No — no UI changes | N/A |
| V. Performance | No — no runtime changes | N/A |

**Gate result**: PASS — all principles are N/A for a documentation-only feature.

## Project Structure

### Documentation (this feature)

```text
specs/004-align-docs-implementation/
├── plan.md              # This file
├── research.md          # FR-by-FR audit results (complete)
├── data-model.md        # Entity descriptions (complete)
├── quickstart.md        # Execution guide (complete)
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
# No new code files. Documents to modify:
CLAUDE.md                           # Fix project structure, normalize versions
specs/001-aws-tui-manager/spec.md   # Annotate 19 FRs, update Status, add Future Work
specs/002-configurable-views/spec.md # Annotate 16 FRs, update Status
specs/003-fix-ui-bugs/spec.md       # Annotate 19 FRs, update Status, add Future Work
docs/design/design.md               # Verify key bindings, fix discrepancies
.claude/agents/tui-ux-auditor.md    # Fix "src/" → "internal/"
.claude/agents/a9s-integrator.md    # Update stale cleanup references
docs/qa/02-s3-views.md              # Spot-check S3 navigation stories
docs/qa/08-detail-all-types.md      # Spot-check detail view stories
docs/qa/01-main-menu.md             # Spot-check key binding stories
```

**Structure Decision**: No new directories or files beyond spec artifacts. All changes are edits to existing documentation.

## Execution Phases

### Phase 1: Fix CLAUDE.md (FR-003, FR-004, FR-005)

Update project structure from "src/ tests/" to actual layout. Normalize Go version to 1.25+. Verify all commands still work.

### Phase 2: Annotate Spec 001 (FR-001, FR-002, FR-009, FR-010, FR-011)

Add inline status suffix to each of 19 FRs per research.md findings. Update Status from "Draft" to "Partial". Add "Future Work" section listing: breadcrumbs (FR-013), history navigation (FR-015), command auto-suggestions (FR-004). Fix factual errors: FR-006 `y` key is YAML not JSON, `[`/`]` not implemented.

### Phase 3: Annotate Spec 002 (FR-001, FR-002)

Add inline status to 16 FRs. Update Status to "Partial". Note: views_reference.yaml not shipped (FR-013), config error not shown to user (FR-005).

### Phase 4: Annotate Spec 003 (FR-001, FR-002)

Add inline status to 19 FRs. Update Status to "Partial". Note: S3 cursor restoration (FR-002), horizontal scroll clamping (FR-012, FR-013), flash clear on nav (FR-006).

### Phase 5: Fix Agent Definitions (FR-007)

Fix tui-ux-auditor.md "src/" → "internal/". Update a9s-integrator.md to reflect that old package cleanup is already complete.

### Phase 6: Verify Design Spec (FR-006)

Check key binding tables in design.md against actual keys.go implementation. Fix any discrepancies.

### Phase 7: Spot-Check QA Stories (FR-012)

Target known-changed areas: S3 folder navigation, `c` copy behavior, `y` YAML rendering, profile switching, detail view layout. Flag or update obviously wrong stories.

### Phase 8: Stale Reference Cleanup (FR-008)

Final sweep: search all .md files for uncontextualized "internal/app/" and "src/" references.

## Complexity Tracking

No constitution violations to justify — this is a documentation-only feature.
