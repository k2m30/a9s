# Feature Specification: Align Documentation With Implementation

**Feature Branch**: `004-align-docs-implementation`
**Created**: 2026-03-18
**Status**: Draft
**Input**: User description: "align documentation with implementation to be able to move forward"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Developer Reads Specs and Understands Current State (Priority: P1)

A developer (or AI agent) opens the project specs (001, 002, 003) and reads them to understand what the application does. Every claim in the specs matches the actual running code — there are no phantom features described that don't exist, no implemented features that are missing from specs, and no behaviors described differently from how they actually work. The developer can trust the specs as a source of truth for the current state of the project.

**Why this priority**: Without trustworthy specs, any future development work starts from a wrong baseline. Every subsequent feature, bug fix, or architecture decision risks building on false assumptions.

**Independent Test**: Can be tested by reading each spec requirement and verifying it against the actual code behavior — either via existing tests, code inspection, or manual verification.

**Acceptance Scenarios**:

1. **Given** spec 001 describes feature behaviors, **When** a developer reads each acceptance scenario, **Then** every scenario accurately describes how the application currently works (or is explicitly marked as "not yet implemented").
2. **Given** spec 002 describes configurable views, **When** a developer reads the YAML structure and field path conventions, **Then** the documented format matches the actual `views.yaml` and the code that parses it.
3. **Given** spec 003 describes bug fixes, **When** a developer reads each fix description, **Then** every fix is either confirmed implemented or explicitly marked as still open.
4. **Given** all three specs have a Status field, **When** a developer checks the status, **Then** completed features show "Complete" and any unfinished items are clearly listed.

---

### User Story 2 - Developer Reads CLAUDE.md and Gets Correct Build/Test Commands (Priority: P1)

A developer reads CLAUDE.md to understand how to build, test, and work with the project. The project structure description, technology versions, commands, and rules all match the actual project.

**Why this priority**: CLAUDE.md is the first file AI agents read. Incorrect information here causes cascading errors in every AI-assisted task.

**Independent Test**: Run every command listed in CLAUDE.md and verify they succeed. Check every factual claim against the actual project.

**Acceptance Scenarios**:

1. **Given** CLAUDE.md describes the project structure, **When** a developer looks at the filesystem, **Then** the described directories and layout match reality.
2. **Given** CLAUDE.md lists build and test commands, **When** a developer runs each command, **Then** they all succeed.
3. **Given** CLAUDE.md states technology versions, **When** a developer checks go.mod and dependencies, **Then** the versions match.

---

### User Story 3 - Developer Reads Design Spec and Understands Visual Layout (Priority: P2)

A developer reads the design spec (docs/design/design.md) to understand the visual structure, color palette, and component specifications. The design spec accurately describes the current rendering behavior, not aspirational designs.

**Why this priority**: The design spec guides all visual work. Drift between spec and implementation means visual changes are made against wrong expectations.

**Independent Test**: Compare each wireframe section in the design spec against actual application screenshots or rendered test output.

**Acceptance Scenarios**:

1. **Given** the design spec describes the header bar format, **When** the application renders, **Then** the header matches the spec description.
2. **Given** the design spec describes key bindings, **When** a developer presses each listed key, **Then** the described action occurs.
3. **Given** the design spec describes view layouts, **When** each view renders, **Then** the visual structure matches the spec wireframes.

---

### User Story 4 - Agent Definitions Reflect Current Architecture (Priority: P2)

AI agent definitions in `.claude/agents/` accurately describe the current project architecture, package structure, and development patterns. An agent spawned from these definitions has correct context to do its job.

**Why this priority**: Agents with stale context waste time, produce wrong code, and need constant correction.

**Independent Test**: Read each agent definition and verify every factual claim about the codebase is current.

**Acceptance Scenarios**:

1. **Given** agent definitions reference package paths, **When** those paths are checked, **Then** they exist and contain what the agent expects.
2. **Given** agent definitions describe architectural patterns, **When** those patterns are checked in code, **Then** the described patterns are actually used.

---

### User Story 5 - Stale Documentation Removed (Priority: P3)

Documents that describe abandoned approaches, deleted code paths, or superseded designs are either removed or clearly marked as historical. No document references `internal/app/` (deleted package), old god-object patterns, or features that were designed but never implemented.

**Why this priority**: Stale docs are worse than no docs — they actively mislead.

**Independent Test**: Search all documentation for references to deleted packages, old patterns, or unimplemented features.

**Acceptance Scenarios**:

1. **Given** the old `internal/app/` package was deleted, **When** searching all docs for "internal/app", **Then** zero results are found (or references are clearly marked as historical).
2. **Given** some planned features were never implemented, **When** reading specs, **Then** unimplemented items are explicitly marked rather than appearing as current functionality.

---

### Edge Cases

- What happens when a spec requirement was partially implemented? Mark it with the specific gaps remaining.
- What happens when the implementation diverges intentionally from the spec (e.g., a better approach was found)? Update the spec to match the implementation, documenting why.
- What happens when documentation references features that were designed but deprioritized? Move them to a "Future Work" section rather than deleting, so context is preserved.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: All spec files (001, 002, 003) MUST have their Status field updated to reflect actual implementation state (Complete, Partial, or Draft with notes).
- **FR-002**: Every functional requirement in specs 001-003 MUST be verified against the implementation and annotated with an inline text suffix: `— *Implemented*`, `— *Partial: missing X*`, or `— *Not implemented*`.
- **FR-003**: CLAUDE.md MUST accurately describe the current project structure, matching actual directories (`internal/`, `cmd/`, `tests/`).
- **FR-004**: CLAUDE.md MUST list correct technology versions matching `go.mod` (Go 1.25+, Bubble Tea v2.0.2, Lipgloss v2.0.2).
- **FR-005**: CLAUDE.md MUST list only commands that currently work when executed.
- **FR-006**: The design spec (docs/design/design.md) MUST match the current rendering behavior for all described components.
- **FR-007**: All agent definitions in `.claude/agents/` MUST reference only packages and patterns that currently exist.
- **FR-008**: No documentation file MUST contain references to `internal/app/` unless explicitly marked as historical context.
- **FR-009**: Spec 001 MUST accurately reflect that profiles are read only from `~/.aws/config` (not `~/.aws/credentials`).
- **FR-010**: Spec 001 MUST accurately reflect that `y` key shows YAML format (not JSON).
- **FR-011**: Spec 001 MUST accurately reflect which key bindings are actually implemented (e.g., `[`/`]` for history navigation is not implemented).
- **FR-012**: QA stories in `docs/qa/` MUST be verified via targeted spot-check of known-changed areas (key bindings, S3 navigation, detail view, copy functionality, profile switching). Obviously wrong stories are updated or removed; unchanged areas are left as-is.

### Key Entities

- **Spec File**: A feature specification document in `specs/NNN-name/spec.md` describing requirements, acceptance scenarios, and success criteria for a feature.
- **Design Spec**: The visual design document at `docs/design/design.md` describing layout, colors, components, and wireframes.
- **Agent Definition**: An AI agent configuration file in `.claude/agents/` describing an agent's role, tools, and project context.
- **Project Guide**: The `CLAUDE.md` file containing build commands, project structure, and development rules.

## Assumptions

- The implementation is the source of truth. Where docs and code disagree, the code is correct and docs should be updated to match.
- Specs should reflect what IS, not what SHOULD BE. Future work items belong in a clearly labeled section.
- No code changes are needed — this is purely a documentation alignment task.
- The 1,045 existing tests serve as verification of current behavior.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer reading specs 001-003 can determine exactly which features are implemented, partially implemented, and not yet implemented — without needing to read the code.
- **SC-002**: Every command in CLAUDE.md executes successfully when run verbatim.
- **SC-003**: Zero documentation files reference `internal/app/` without historical context markers.
- **SC-004**: A new AI agent spawned with updated agent definitions produces correct code on the first attempt for at least basic tasks (adding a test, modifying a view).
- **SC-005**: The design spec describes only behaviors that match the current application — a developer comparing screenshots to wireframes finds no contradictions.
- **SC-006**: All spec Status fields are either "Complete" or "Partial" with explicit gap lists — no specs remain in "Draft" status for implemented features.

## Clarifications

### Session 2026-03-18

- Q: How should requirement verification be recorded in specs 001-003? → A: Inline text suffix on each FR line (e.g., `— *Implemented*`, `— *Partial: missing X*`, `— *Not implemented*`).
- Q: How deeply should QA stories (docs/qa/) be verified? → A: Targeted spot-check of known-changed areas (key bindings, S3 nav, detail view, copy, profile switching). Flag obviously wrong stories; leave unchanged areas as-is.
