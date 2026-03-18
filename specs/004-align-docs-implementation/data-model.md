# Data Model: Align Documentation With Implementation

No new data entities are introduced by this feature. This is a documentation-only task.

## Entities Modified (documentation only)

### Spec File
- **Location**: `specs/NNN-name/spec.md`
- **Fields modified**: Status (Draft → Complete/Partial), FR annotations (inline suffix added)
- **New section**: "Future Work" for unimplemented features

### Project Guide (CLAUDE.md)
- **Fields modified**: Project Structure section, technology version references
- **No new fields**

### Agent Definition
- **Location**: `.claude/agents/*.md`
- **Fields modified**: Package path references, codebase structure descriptions
- **Affected files**: tui-ux-auditor.md, a9s-integrator.md

### Design Spec
- **Location**: `docs/design/design.md`
- **Fields modified**: Key binding tables, component descriptions where they diverge from implementation

### QA Story
- **Location**: `docs/qa/*.md`
- **Fields modified**: Individual given/when/then scenarios in known-changed areas
