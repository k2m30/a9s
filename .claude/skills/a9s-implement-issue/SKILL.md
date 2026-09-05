---
name: a9s-implement-issue
description: End-to-end workflow for implementing a GitHub issue — interview, spec, one approval, then autonomous through the team loop to acceptance and release prep. Use for any issue that is NOT a new-resource or child-view (those have their own skills).
disable-model-invocation: true
---

# Implement Issue Workflow

Take a GitHub issue from open to accepted. There is exactly **one** human gate: the user approves `spec.md`. Everything after it runs autonomously through the team loop in `.claude/skills/a9s-team-loop/SKILL.md`.

**Not for:** issues tagged `new-resource` or `child-view` — use `a9s-resource-spec` (spec) + `a9s-implement-resource` (implementation) instead.

| Phase | Who | Output |
|-------|-----|--------|
| 1. Interview & spec | Orchestrator | `TASKDIR/spec.md` |
| 2. **Approval** | User | the only gate |
| 3. Team loop | `a9s-dev` + `a9s-qa` + `a9s-facilitator` | stubs → red tests → implementation → sign-off |
| 4. Acceptance | `a9s-acceptance` | ACCEPT or REJECT on rendered surfaces |
| 5. Docs & release prep | Orchestrator | shared docs, README, CHANGELOG, release notes |

---

## Phase 1: Interview and write the spec

### Read the issue

```bash
gh issue view {N} --json title,body,labels,comments,milestone
```

Read every comment — they carry design decisions, scope changes, and blockers the body does not.

### Classify it

| Type | Description |
|------|-------------|
| **ui-enhancement** | Changes to existing TUI views — new keys, columns, styling |
| **new-feature** | New TUI component or capability that does not exist yet |
| **data-layer** | Fetchers, config, or resource model without TUI changes |
| **milestone** | Multi-issue epic — break into sub-issues first, then run this skill per sub-issue |
| **infra** | CI/CD, build, signing, non-code |
| **docs-only** | Documentation, QA stories, branding |

Size it: `S` (1-3 files, pattern exists) · `M` (4-10 files, multiple packages) · `L` (10+ files, new component or pattern) · `XL` (break down first).

### Interview the user

Ask what the issue does not answer, in one batch: the acceptance criteria if they are missing, the UX decision if the issue is ambiguous, what is explicitly out of scope. Ask once, then write. Do not interview a second time to confirm the spec — that is the approval gate's job.

### Write `TASKDIR/spec.md`

The spec is the contract. It must:

- state the problem in one sentence and the acceptance criteria as a numbered list;
- name the exact files and interfaces involved, and the exact symbols the implementation adds (signatures, finding codes, phrases, severities) — `a9s-dev` stubs these in round 0 and `a9s-qa` writes tests against these names, so a missing signature costs a round;
- state what is out of scope;
- end with an end-to-end verification step: the rendered surface a user would check.

Include, as steps the agents run rather than as sign-offs:

- **Story coverage** — every acceptance criterion, the happy path per interaction, and the edge cases: empty state, nil fields, terminal resize, narrow terminal, `NO_COLOR`, key-binding conflicts with existing bindings, interaction with filter/sort/copy/help.
- **Affected surfaces** — which views (`internal/tui/views/`), styles (`internal/tui/styles/`), keys (`internal/tui/keys/keys.go`), messages (`core/runtime/messages/`), config (`core/config/`), and existing tests (`tests/unit/`) the change touches.
- **Design** — run `tui-designer` and `go run ./cmd/preview/` only when the issue needs a new TUI component, a layout change, or a new interaction pattern. Fold the resulting wireframe into the spec rather than raising a second gate.

**GATE: present `spec.md` to the user. This is the only approval.**

---

## Phase 2 onward: run the loop

Dispatch through `a9s-team-loop`. The orchestrator writes no code and no tests.

1. `a9s-dev` round 0 — compile-clean stubs for every symbol the spec pins.
2. `a9s-qa` round 1 — red tests that fail on assertions.
3. `a9s-dev` round 1 — the implementation.
4. `a9s-qa` verify — findings or `SIGN-OFF`.
5. `a9s-facilitator` on `OFF` / `LOOP` / `BLOCKED` or past round 3.
6. `a9s-acceptance` against the issue's acceptance criteria.

Findings and rejects route back to the loop, not to the user. Escalate to the user only for a `BLOCKED` the facilitator cannot rule on, or a scope change the spec did not cover.

---

## Phase 3: Docs and release prep

Update the shared source and regenerate — never edit `README.md` directly:

| What changed | Update |
|-------------|--------|
| Key bindings added/removed | `docs/shared/keybindings.md` |
| Child views added/removed | `docs/shared/keybindings.md` + `docs/design/child-views/` |
| Commands added/removed | `docs/shared/commands.md` |
| CLI flags changed | `docs/shared/quickstart.md` |
| Config options changed | `docs/shared/config.md` |
| Resource types changed | `docs/README.tmpl.md` services table + `website/content/resources.md` |

```bash
go run ./cmd/readmegen/ > README.md
```

Then `CHANGELOG.md` for the user-visible change, `make ready-to-push` green from captured output, and a conventional commit referencing the issue:

```text
feat: {short description} (#{N})
```

Release notes and tagging follow `docs/development-process.md` Stage 7 and the `release` skill. Close the issue after the release:

```bash
gh issue close {N} --comment "Implemented in {commit}. Released in vX.Y.Z."
```

---

## Milestone breakdown (XL issues)

1. Read the milestone body for phasing hints.
2. Break it into independently implementable sub-issues via `gh issue create`, titled `{milestone title}: {sub-feature}`, each with its own scoped acceptance criteria and the parent's labels plus `milestone:{N}`.
3. Comment the breakdown on the parent issue.
4. Run this skill per sub-issue. Sub-issues are separate tasks with separate worktrees, and at most four run at a time.
