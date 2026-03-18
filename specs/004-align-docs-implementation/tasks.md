# Tasks: Align Documentation With Implementation

**Input**: Design documents from `/specs/004-align-docs-implementation/`
**Prerequisites**: plan.md (required), spec.md (required), research.md

**Tests**: Not applicable — documentation-only feature, no code changes.

**Organization**: Tasks grouped by user story. Each story is independently completable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: No setup needed — all target files already exist. Proceed directly to user stories.

---

## Phase 2: Foundational

**Purpose**: Read research.md audit results so all subsequent edits are based on verified findings.

- [ ] T001 Read /Users/k2m30/projects/a9s/specs/004-align-docs-implementation/research.md to confirm FR audit results before editing any spec

**Checkpoint**: Research findings confirmed — spec editing can begin.

---

## Phase 3: User Story 2 - Fix CLAUDE.md (Priority: P1) MVP

**Goal**: CLAUDE.md accurately describes project structure, versions, and commands.

**Independent Test**: Run every command listed in CLAUDE.md. Check project structure matches filesystem.

- [ ] T002 [US2] Fix project structure in /Users/k2m30/projects/a9s/CLAUDE.md — replace "src/ tests/" with actual layout: cmd/, internal/ (with subpackages aws/, config/, fieldpath/, resource/, tui/), tests/, docs/, specs/
- [ ] T003 [US2] Normalize "Go 1.22+" references to "Go 1.25+" in /Users/k2m30/projects/a9s/CLAUDE.md to match go.mod
- [ ] T004 [US2] Remove duplicate/conflicting Active Technologies entries in /Users/k2m30/projects/a9s/CLAUDE.md — consolidate to single entry: Go 1.25+, Bubble Tea v2.0.2, Lipgloss v2.0.2, Bubbles v2, AWS SDK Go v2, yaml.v3, clipboard

**Checkpoint**: CLAUDE.md is accurate. All commands work. Structure matches filesystem.

---

## Phase 4: User Story 1 - Annotate Specs 001-003 (Priority: P1)

**Goal**: Every FR in specs 001-003 has an inline status annotation. Status fields updated. Future Work sections added.

**Independent Test**: Read any spec FR and immediately know if it's implemented, partial, or missing.

### Spec 001: AWS TUI Manager (19 FRs)

- [ ] T005 [P] [US1] Annotate FR-001 through FR-019 in /Users/k2m30/projects/a9s/specs/001-aws-tui-manager/spec.md with inline status suffixes per research.md findings: 15 Implemented, 2 Partial (FR-004 no auto-suggestions, FR-006 y=YAML not JSON + no [/] keys), 2 Not implemented (FR-013 breadcrumbs, FR-015 history nav)
- [ ] T006 [P] [US1] Update Status field from "Draft" to "Partial" in /Users/k2m30/projects/a9s/specs/001-aws-tui-manager/spec.md
- [ ] T007 [P] [US1] Add "## Future Work" section at end of /Users/k2m30/projects/a9s/specs/001-aws-tui-manager/spec.md listing: breadcrumb navigation (FR-013), back/forward history with [/] keys (FR-015), command auto-suggestions (FR-004)
- [ ] T008 [US1] Fix FR-002 text in /Users/k2m30/projects/a9s/specs/001-aws-tui-manager/spec.md — currently says reads from ~/.aws/credentials, verify if implementation actually reads both or only config
- [ ] T009 [US1] Fix FR-006 text in /Users/k2m30/projects/a9s/specs/001-aws-tui-manager/spec.md — change "raw JSON view" to "YAML view" for y key description

### Spec 002: Configurable Views (16 FRs)

- [ ] T010 [P] [US1] Annotate FR-001 through FR-016 in /Users/k2m30/projects/a9s/specs/002-configurable-views/spec.md with inline status suffixes per research.md: 13 Implemented, 1 Partial (FR-005 no user error on bad YAML), 1 Not implemented (FR-013 reference artifact not shipped), 1 N/A (FR-014)
- [ ] T011 [P] [US1] Update Status field from "Draft" to "Partial" in /Users/k2m30/projects/a9s/specs/002-configurable-views/spec.md
- [ ] T012 [P] [US1] Add "## Future Work" section at end of /Users/k2m30/projects/a9s/specs/002-configurable-views/spec.md listing: ship views_reference.yaml artifact (FR-013), user-facing error on invalid YAML (FR-005)

### Spec 003: Fix UI Bugs (19 FRs)

- [ ] T013 [P] [US1] Annotate FR-001 through FR-019 in /Users/k2m30/projects/a9s/specs/003-fix-ui-bugs/spec.md with inline status suffixes per research.md: 14 Implemented, 2 Partial (FR-008 S3 count uncertain, FR-012 right scroll unbounded), 2 Not implemented (FR-002 cursor restore, FR-013 scroll reset), 1 Partial (FR-006 flash not cleared on nav)
- [ ] T014 [P] [US1] Update Status field from "Draft" to "Partial" in /Users/k2m30/projects/a9s/specs/003-fix-ui-bugs/spec.md
- [ ] T015 [P] [US1] Add "## Future Work" section at end of /Users/k2m30/projects/a9s/specs/003-fix-ui-bugs/spec.md listing: S3 cursor position restore (FR-002), horizontal scroll clamping (FR-012), scroll reset on nav (FR-013), flash clear on nav (FR-006)

**Checkpoint**: All 54 FRs annotated. All specs have correct Status. Future Work sections document gaps.

---

## Phase 5: User Story 3 - Verify Design Spec (Priority: P2)

**Goal**: Design spec key binding tables and component descriptions match current implementation.

**Independent Test**: Compare key binding table in design.md against keys.go — no contradictions.

- [ ] T016 [US3] Verify key binding tables in /Users/k2m30/projects/a9s/docs/design/design.md Section 5 against /Users/k2m30/projects/a9s/internal/tui/keys/keys.go — remove [/] history keys if listed, confirm y is described as YAML not JSON, confirm all listed keys actually exist
- [ ] T017 [US3] Verify Section 3.5 (Detail View) in /Users/k2m30/projects/a9s/docs/design/design.md matches current detail.go rendering — check indentation rules (1-space top-level, 5-space sub-fields), colon format, section headers

**Checkpoint**: Design spec matches implementation for all described components.

---

## Phase 6: User Story 4 - Fix Agent Definitions (Priority: P2)

**Goal**: All agent definitions reference only packages and patterns that currently exist.

**Independent Test**: Read each agent definition — no references to nonexistent paths.

- [ ] T018 [P] [US4] Fix /Users/k2m30/projects/a9s/.claude/agents/tui-ux-auditor.md line 20 — change "src/" to "internal/" and "tests/"
- [ ] T019 [P] [US4] Update /Users/k2m30/projects/a9s/.claude/agents/a9s-integrator.md — note that old package cleanup (internal/app/, internal/views/, etc.) is already complete, update language from "will remove" to "removed"

**Checkpoint**: All 10 agent definitions are accurate.

---

## Phase 7: User Story 5 - Stale References & QA Spot-Check (Priority: P3)

**Goal**: No uncontextualized stale references. QA stories accurate in known-changed areas.

**Independent Test**: grep all .md files for "internal/app" and "src/" — zero uncontextualized hits.

- [x] T020 [US5] Search all .md files under /Users/k2m30/projects/a9s/ for uncontextualized "internal/app/" references — any found in specs/, docs/design/, CLAUDE.md, or agent files must be removed or marked historical
- [x] T021 [US5] Search all .md files for "src/" references (outside .specify/ templates) — fix any that reference nonexistent src/ directory
- [x] T022 [P] [US5] Spot-check /Users/k2m30/projects/a9s/docs/qa/02-s3-views.md — verify S3 folder navigation stories match implementation (Enter on folder sends S3NavigatePrefixMsg, d opens detail)
- [x] T023 [P] [US5] Spot-check /Users/k2m30/projects/a9s/docs/qa/08-detail-all-types.md — verify detail view stories match implementation (colon format, indentation, config-driven fields)
- [x] T024 [P] [US5] Spot-check /Users/k2m30/projects/a9s/docs/qa/01-main-menu.md — verify key binding references match actual keys (y=YAML, c=context-aware copy, no [/] history)

**Checkpoint**: No stale references. QA stories accurate in checked areas.

---

## Phase 8: Polish & Verification

**Purpose**: Final sweep and validation across all updated documents.

- [ ] T025 Run `go build -o a9s ./cmd/a9s/` to confirm no code was accidentally changed
- [ ] T026 Run `go test ./tests/unit/ -count=1 -timeout 120s` to confirm all tests still pass
- [ ] T027 Read each spec Status field — confirm none say "Draft"
- [ ] T028 Final review: open each modified file and confirm formatting is consistent (no broken markdown)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: No dependencies — read research.md first
- **US2 CLAUDE.md (Phase 3)**: Depends on Phase 2 — no cross-dependencies with other stories
- **US1 Spec Annotations (Phase 4)**: Depends on Phase 2 — no cross-dependencies with other stories
- **US3 Design Spec (Phase 5)**: Depends on Phase 2 — independent of US1/US2
- **US4 Agent Defs (Phase 6)**: Depends on Phase 2 — independent of other stories
- **US5 Stale Refs (Phase 7)**: Depends on Phases 3-6 (searches for remaining stale refs after fixes)
- **Polish (Phase 8)**: Depends on all previous phases

### User Story Dependencies

- **US1 (Spec Annotations)**: Independent — can start after T001
- **US2 (CLAUDE.md)**: Independent — can start after T001
- **US3 (Design Spec)**: Independent — can start after T001
- **US4 (Agent Defs)**: Independent — can start after T001
- **US5 (Stale Refs)**: Should run LAST among stories — catches anything missed by US1-US4

### Parallel Opportunities

- T005, T006, T007 (spec 001 annotations) can run in parallel
- T010, T011, T012 (spec 002 annotations) can run in parallel
- T013, T014, T015 (spec 003 annotations) can run in parallel
- All three spec annotation groups (T005-T009, T010-T012, T013-T015) can run in parallel with each other
- T018, T019 (agent fixes) can run in parallel
- T022, T023, T024 (QA spot-checks) can run in parallel
- US1, US2, US3, US4 can all proceed in parallel after T001

---

## Parallel Example: Spec Annotations (Phase 4)

```text
# All three specs can be annotated simultaneously:
Agent A: "Annotate 19 FRs in specs/001-aws-tui-manager/spec.md"
Agent B: "Annotate 16 FRs in specs/002-configurable-views/spec.md"
Agent C: "Annotate 19 FRs in specs/003-fix-ui-bugs/spec.md"
```

---

## Implementation Strategy

### MVP First (US1 + US2 Only)

1. Complete Phase 2: Read research findings
2. Complete Phase 3: Fix CLAUDE.md (US2) — agents get correct context immediately
3. Complete Phase 4: Annotate all specs (US1) — specs become trustworthy
4. **STOP and VALIDATE**: Specs are usable, CLAUDE.md is correct
5. Remaining stories (US3-US5) are polish

### Incremental Delivery

1. Fix CLAUDE.md → immediate value for AI agents
2. Annotate specs → specs become trustworthy source of truth
3. Verify design spec → visual work unblocked
4. Fix agent defs → agent reliability improved
5. Clean stale refs → final sweep catches remaining issues

---

## Notes

- No code changes — all tasks edit .md files only
- research.md contains the verified audit results to reference during edits
- Use inline status format: `— *Implemented*`, `— *Partial: missing X*`, `— *Not implemented*`
- Preserve existing spec formatting — only add annotations, don't restructure
- QA spot-checks: only flag obviously wrong stories, don't rewrite entire files
