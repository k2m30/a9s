---
name: a9s-implement-issue
description: End-to-end workflow for implementing a GitHub issue — from analysis through QA stories, design, scoped tasks, implementation, pre-release checks, and release prep. Use for any issue that is NOT a new-resource or child-view (those have their own skills).
disable-model-invocation: true
---

# Implement Issue Workflow

End-to-end pipeline for taking a GitHub issue from open to released. Covers analysis, QA stories, design, scoped task generation, implementation, verification, docs, and release prep.

**Not for:** Issues tagged `new-resource` or `child-view` — use `a9s-add-resource` or `a9s-add-child-view` skills instead.

## Phase Overview

| Phase | Who | Output | Gate |
|-------|-----|--------|------|
| 1. Analyze | Architect | Issue analysis + complexity assessment | User approves scope |
| 2. QA Stories | `a9s-qa-stories` agent | `docs/qa/issue-{N}-{slug}.md` | Stories exist |
| 3. Design | `tui-designer` agent (if needed) | Design spec update or new design doc | Design approved |
| 4. Scope | Architect | CODER TASK + QA TASK with exact files | User approves tasks |
| 5. Implement | `a9s-qa` + `a9s-coder` agents | Tests + production code | All tests pass |
| 6. Verify | Pre-release checks | Clean lint, tests, arch score | All green |
| 7. Docs | Architect + coder | Updated shared docs, README, website | Docs in sync |
| 8. Release | `release` skill | Tag, release notes, changelog | User triggers |

---

## Phase 1: Analyze Issue

Read the issue and all its comments. Classify it and assess scope.

### Step 1.1: Fetch issue

```
gh issue view {N} --json title,body,labels,comments,milestone
```

Read every comment — they often contain design decisions, scope changes, or blockers.

### Step 1.2: Classify

Determine issue type (exactly one):

| Type | Description | Examples |
|------|-------------|---------|
| **ui-enhancement** | Changes to existing TUI views — new keys, columns, styling | #61 status colors, #57 human-readable formats |
| **new-feature** | New TUI component or capability that doesn't exist yet | #89 cross-view search, #22 color themes, #81 configurable menu |
| **data-layer** | Changes to fetchers, config, or resource model without TUI changes | #68 cache availability, #21 cache log sources |
| **milestone** | Multi-issue epic requiring breakdown before implementation | #63 resource actions, #64 cross-resource navigation |
| **infra** | CI/CD, build, signing, non-code | #76 Windows code signing |
| **docs-only** | Documentation, QA stories, branding | #54 brand graphics, #67 QA milestone |

### Step 1.3: Assess complexity

| Complexity | Criteria | Parallelization |
|------------|----------|-----------------|
| **S** (small) | 1-3 files changed, pattern exists, no new components | `parallel-safe` |
| **M** (medium) | 4-10 files, may need design clarification, touches multiple packages | `parallel-safe` if interfaces locked |
| **L** (large) | 10+ files, new component/pattern, needs design phase | `sequential` — QA first |
| **XL** (milestone) | Must be broken into sub-issues first | STOP — break down, create sub-issues |

### Step 1.4: Check prerequisites

- [ ] QA stories exist in `docs/qa/`? (grep for issue number)
- [ ] Design spec covers this feature? (check `docs/design/`)
- [ ] Any blocking issues mentioned?
- [ ] Does the issue body contain acceptance criteria?

**Output:** Issue analysis summary with type, complexity, missing prerequisites, and recommended next phases.

**GATE: Present analysis to user. Get approval before proceeding.**

---

## Phase 2: QA Stories

If QA stories don't already exist for this issue, generate them.

### Step 2.1: Check for existing stories

```
grep -r "#{N}" docs/qa/
```

If stories exist and cover the acceptance criteria, skip to Phase 3.

### Step 2.2: Run `a9s-qa-stories` agent

Dispatch with this prompt:

```
Write QA stories for GitHub issue #{N}: {title}

Issue description:
{paste issue body}

Acceptance criteria:
{paste from issue}

Write stories to: docs/qa/issue-{N}-{slug}.md

Cover:
- Every acceptance criterion as at least one story
- Happy path for each user interaction
- Error/edge cases: empty state, nil values, terminal resize, boundary widths
- Key binding conflicts with existing bindings
- Interaction with existing features (filter, sort, copy, help overlay)
- Accessibility: NO_COLOR support, minimum terminal size
```

### Step 2.3: Review stories

Read the generated file. Verify:
- [ ] Every acceptance criterion from the issue is covered
- [ ] Edge cases include: empty state, nil fields, resize, narrow terminal
- [ ] No stories reference implementation details (pure black-box)
- [ ] AWS CLI comparisons included where applicable

---

## Phase 3: Design (if needed)

Skip this phase if:
- The issue is type `ui-enhancement` with clear spec (e.g., #61 just adds color mappings)
- The design spec already covers the feature completely
- The issue body contains complete wireframes/layouts

Run this phase if:
- New TUI component needed (e.g., #89 search component, #22 theme selector)
- Layout changes needed (e.g., #81 filtered menu)
- Confirmation dialogs or new interaction patterns needed (e.g., #63 actions)
- The issue says "needs design" or has ambiguous UX

### Step 3.1: Run `tui-designer` agent

Dispatch with this prompt:

```
Design the TUI interface for GitHub issue #{N}: {title}

Context:
- a9s is a TUI AWS resource manager using Bubble Tea v2 + Lipgloss v2
- Current design spec: docs/design/design.md
- Tokyo Night Dark palette: internal/tui/styles/palette.go
- Existing views: {list affected views}

Requirements from issue:
{paste relevant requirements}

QA stories to satisfy:
{paste key stories from Phase 2}

Output:
1. Update docs/design/{appropriate-file}.md with wireframes
2. Update cmd/preview/main.go if new visual elements
3. Specify which existing views are affected and how
```

### Step 3.2: Visual approval

Ask user to run `go run ./cmd/preview/` and approve the design.

**GATE: User approves design before proceeding.**

---

## Phase 4: Scope Tasks

The architect reads the issue, QA stories, and design spec, then produces exact scoped tasks for coder and QA.

### Step 4.1: Identify all affected files

Read the codebase to determine exact file paths:

1. **Which views are affected?** — grep for relevant types/functions in `internal/tui/views/`
2. **Which styles change?** — check `internal/tui/styles/`
3. **Which keys change?** — check `internal/tui/keys/keys.go`
4. **Which messages change?** — check `internal/runtime/messages/`
5. **Which config changes?** — check `internal/config/`
6. **Which tests exist?** — grep in `tests/unit/` for existing coverage
7. **Which docs need updates?** — check `docs/shared/` for affected content

### Step 4.2: Produce CODER TASK

```
## CODER TASK: {title} (#{N})
Parallelization: {parallel-safe | sequential (after QA)}

### Files to create:
- `{path}` — {description}

### Files to modify:
- `{path}` — `{function/struct}` — {what to change}
  - Append point / edit location: {grep pattern or line reference}

### Expected behavior:
- {bullet points matching acceptance criteria}

### Type signatures (if new):
```go
{exact signatures}
```

### Context files (read-only):
- `{path}` — {why}
```

### Step 4.3: Produce QA TASK

```
## QA TASK: {title} (#{N})
Parallelization: {parallel-safe | sequential (before coder)}

### Test files to create:
- `{path}` — {what it tests}

### Test files to modify:
- `{path}` — {what to add/change}
  - Append point: {grep pattern or function name}

### What to test:
- Function: `{package}.{Func}({params}) ({returns})`
- Happy path: {expected behavior}
- Error path: {expected error behavior}
- Edge cases: {specific to this issue}

### Mock structure (if needed):
```go
{exact mock}
```

### Type signatures:
```go
{types needed for compilable tests}
```

### Context files (read-only):
- `{path}` — {why}
```

**GATE: Present both tasks to user. Get approval before dispatching.**

---

## Phase 5: Implement

**The architect NEVER writes code in this phase.** The architect spins off `a9s-coder` and `a9s-qa` agents with the scoped task specs. The user should not have to manually pass tasks to agents — dispatching IS the architect's job.

Dispatch tasks to agents. Order depends on parallelization assessment.

### For `parallel-safe` tasks:
Spin off `a9s-qa` and `a9s-coder` simultaneously with their respective task specs.

### For `sequential` tasks:
1. Spin off `a9s-qa` first — write tests
2. Verify tests compile (or fail with expected missing-implementation errors)
3. Spin off `a9s-coder` — write implementation to make tests pass

### Step 5.1: Verify after both complete

```bash
make test
make lint
make security
make gofix
make build
```

All four must pass. If tests fail, identify whether it's a coder bug or a QA bug and dispatch a fix to the appropriate agent with exact scope.

---

## Phase 6: Pre-Release Verification

Run the full pre-push checklist:

### Step 6.1: Automated checks
```bash
bash .claude/scripts/arch-review.sh > /tmp/arch-review.txt 2>&1
```

### Step 6.2: Agent checks
Run these agents (read-only, safe to parallel):
- `a9s-consistency-checker` — code/docs/website alignment
- `test-coverage-analyzer` — coverage gaps for the new code
- `a9s-architect` with `a9s-arch-review` skill — architecture score (target: 8.5+/10)

### Step 6.3: Fix any findings

If any check fails, produce a scoped fix task for the appropriate agent. Do NOT proceed until all checks pass.

---

## Phase 7: Docs Update

Determine which shared docs need updates based on what changed:

| What changed | Update |
|-------------|--------|
| Key bindings added/removed | `docs/shared/keybindings.md` |
| Child views added/removed | `docs/shared/childviews.md` |
| Commands added/removed | `docs/shared/commands.md` |
| CLI flags changed | `docs/shared/quickstart.md` |
| Config options changed | `docs/shared/config.md` |
| Resource types changed | `docs/README.tmpl.md` services table + `website/content/resources.md` |

### Step 7.1: Update shared docs

Edit the relevant `docs/shared/*.md` files.

### Step 7.2: Regenerate README

```bash
go run ./cmd/readmegen/ > README.md
```

### Step 7.3: Update website (if applicable)

Website uses Hugo `{{< include >}}` shortcodes resolving to `docs/shared/`. If shared docs changed, verify the website renders correctly.

### Step 7.4: Update design spec

If the implementation revealed design decisions or the design spec has ambiguities that were resolved, update `docs/design/`.

---

## Phase 8: Release Prep

**GATE: All previous phases must be complete. Present summary to user and get explicit approval before committing.**

### Step 8.1: Commit

Stage all changed files. Write a commit message referencing the issue:

```
feat: {short description} (#{N})

{1-2 sentence summary of what changed and why}
```

### Step 8.2: Prepare release notes

Determine version bump:
- **Patch** — bug fix, style tweak, doc update
- **Minor** — new feature, new key binding, new view capability
- **Major** — breaking change (config format, removed feature)

Draft `releases/vX.Y.Z.md` following the format in existing release notes.

### Step 8.3: Close the issue

After merge, close with a comment linking to the PR/commit:

```
gh issue close {N} --comment "Implemented in {commit/PR}. Released in vX.Y.Z."
```

---

## Milestone Breakdown (XL issues only)

For issues classified as `milestone` in Phase 1:

1. Read the milestone issue body for phasing hints
2. Break into sub-issues, each independently implementable
3. Create sub-issues via `gh issue create` with:
   - Title: `{milestone title}: {sub-feature}`
   - Body: Scoped acceptance criteria for just this piece
   - Labels: inherit parent labels + `milestone:{N}`
4. Comment on the parent issue with the breakdown
5. Implement each sub-issue using this same skill (recursive)

---

## Checklist Summary

Copy this to track progress:

```
- [ ] Phase 1: Issue analyzed, complexity assessed, user approved scope
- [ ] Phase 2: QA stories written (or confirmed existing)
- [ ] Phase 3: Design approved (or skipped — not needed)
- [ ] Phase 4: CODER TASK + QA TASK scoped with exact files, user approved
- [ ] Phase 5: Implementation complete, all tests pass
- [ ] Phase 6: Pre-release checks pass (lint, vuln, arch score, consistency)
- [ ] Phase 7: Docs updated, README regenerated
- [ ] Phase 8: Committed, release notes drafted, user approved
```
