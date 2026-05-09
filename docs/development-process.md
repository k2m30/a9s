# a9s Development Process

This is the **single source of truth** for how work flows through a9s — from request to release. Everyone (CEO, CTO, every Paperclip agent) follows it. If a rule here conflicts with an ad-hoc instruction, this document wins until it is updated.

It is short on purpose. Process rot starts the moment a doc becomes too long to re-read in five minutes.

## Goals

1. **Quality is gated, not hoped for.** Each stage has explicit entry and exit criteria. Nothing advances without its gate.
2. **Roles are explicit.** Each stage names exactly one owning **Paperclip company agent**. No ambiguity about who picks this up next.
3. **TDD is non-negotiable.** Tests exist before implementation merges, full stop.
4. **Failures fail loudly and locally.** CI is verification, not a debugger. The pre-push and pre-release gate suites catch regressions before they reach `main`.
5. **The process measures itself.** We track DORA-style metrics and run a weekly retro to keep the process alive.

## Two layers — never confused

There are **two distinct layers** in this codebase, and only one of them can be assigned issues or sign off on PRs:

| Layer | Where it lives | Has heartbeats? | Can be assigned a Paperclip issue? | Can sign off on a PR? |
|---|---|---|---|---|
| **Paperclip company agent** | The hired roster (this section, below) | Yes | Yes | Yes |
| **Claude Code subagent** | `.claude/agents/*.md` (`a9s-*`, `tui-*`, `test-coverage-analyzer`) | No | No | No — they are *tools* a Paperclip agent invokes inside its own Claude Code session |

Throughout this document, every stage owner and every reviewer is a **Paperclip agent name** (capitalized, single word). When that agent uses a Claude Code subagent or skill as a tool, it is listed in the "Tools" column — never in the "Owner" or "Reviewer" column.

Concretely: "the Coder agent invokes the `a9s-coder` subagent during Stage 4." Never "`a9s-coder` owns Stage 4."

## Agents — the 9 hired Paperclip roster

This is the ground-truth list. If a stage names anyone else as an owner or reviewer, the doc is wrong.

| Agent | Role | Charter |
|---|---|---|
| **CEO** | Strategic direction | Files issues, sets priorities, makes final calls on scope and trade-offs. |
| **CTO** | Technical direction | Owns this process. Triage, Stage 5 final sign-off, Stage 7 release ownership, Stage 8 retro. Does not write code. |
| **Architect** | Spec & design | Owns Stage 2. Reviewer at Stage 5 for size ≥ `M`. |
| **Coder** | Implementation | Owns Stage 4. Writes production code, wires packages, builds fixtures. |
| **QA** | Test plans & tests | Owns Stage 3. Translates spec to stories, then to failing tests. |
| **E2ETester** | Real-AWS scenario validation | Owns Stage 6.5 (post-merge real-AWS smoke) and the Stage 7 integration sign-off against a real profile. |
| **CodeReviewer** | Diff-level code review | Reviewer at Stage 5 (always runs). Local-commit and PR-diff review. |
| **CodexReviewer** | Independent pair-review | Reviewer at Stage 5 (always runs). Second-opinion review using Codex. |
| **DevOps** | Consultative only | Provides operational priority input, AWS-practitioner advice, incident-response guidance. **Does not** write code, push branches, or own deploys. Never an implementation owner. |

If a task's deliverable is a PR, branch, fixture, test, doc edit, or release artifact, it is **not** a DevOps task. Route it to Coder (or QA / Architect / CTO as the stage demands).

## Definitions

### Definition of Ready (DoR) — required before Stage 2 begins

An issue is Ready when **all** are true:

- One-sentence problem statement.
- Acceptance criteria written (or pointer to a spec doc / refactor PR spec).
- Owning Paperclip agent assigned (from the roster above).
- Sized: `XS` (≤30 LOC, single file) · `S` (≤200 LOC, ≤3 files) · `M` (≤600 LOC) · `L` (≤1500 LOC) · `XL` (split first — see Splitting below).
- Linked to a goal, refactor phase, or release milestone if applicable.

If any of those is missing, the issue is parked in `todo` and the assignee asks one focused question. Never start fuzzy.

### Definition of Done (DoD) — required before an issue closes `done`

- Acceptance criteria demonstrably met (test, screenshot, or live run).
- Stage 6 (`make ready-to-push`) gates green locally.
- Stage 5 reviewers (per matrix below) signed off.
- Docs sync respected: README is regenerated when `docs/shared/` changes; `CHANGELOG.md` updated for any user-visible change; `docs/architecture.md` aligned for cross-cutting changes.
- Single-source-of-truth invariants intact (no dual-authoring, no permanent dual API surface).
- Conventional commit message, `Co-Authored-By: Paperclip <noreply@paperclip.ing>` on every commit.

### Splitting

`XL` (>1500 LOC, or touches >3 packages, or no clear single owner) is **always** split before work starts. **Architect** owns the split. The 40-PR refactor program is the canonical example of how to size: one mechanical concern per PR, stabilization checkpoints between phases.

## Lifecycle — 8 stages

Every unit of work goes through these stages. Stages 1, 2, 4, 6, 6.5 may be **skipped** for trivial bug fixes (`XS`, single file, no behavior change visible to users). Stages 3, 5, 7 never skip.

```text
1. Intake → 2. Spec → 3. Tests → 4. Impl → 5. Review → 6. Validate → 6.5 Post-merge AWS → 7. Release → 8. Retro
```

### Stage 1 — Intake

- **Owner**: **CTO**.
- **Trigger**: CEO/board files an issue, or a regression is observed.
- **Action**: Triage type (bug · feature · refactor · ops · docs), set priority, set size, name the owning Paperclip agent, draft acceptance criteria.
- **Exit**: Issue meets DoR.
- **Anti-pattern**: Self-assigning unassigned work. CTO does not browse the backlog; only acts on what the CEO assigns. Other agents do not pick up backlog without explicit delegation.

### Stage 2 — Spec & Design

- **Owner**: **Architect**.
- **Tools the Architect invokes**: `a9s-resource-spec` skill (writes `docs/resources/<short>.md`), `a9s-architect` subagent (design output only — writes nothing into source).
- **Optional consult**: **DevOps** for AWS-practitioner priority sanity ("which 10 resources next?", "is CWL more important than Lambda?").
- **Trigger**: DoR met and size ≥ `M`. Skipped for `XS`/`S` bug fixes.
- **Action**: Produce a spec doc. Resources use `a9s-resource-spec`. Refactor PRs reference the per-PR spec in `docs/refactor/`. Features write to `specs/<n>-<feature>.md`.
- **Exit**: Spec doc + CTO sign-off comment on the issue. The spec is the contract; existing implementation is disposable.
- **Anti-pattern**: Skipping the spec for "obvious" features. If it is so obvious, the spec is one paragraph — write it anyway.

### Stage 3 — Tests

- **Owner**: **QA**.
- **Tools the QA agent invokes**: `a9s-qa-stories` (given/when/then with zero source-code knowledge), `a9s-qa` (failing Go tests), `a9s-related-qa` (related-view scope).
- **Trigger**: Stage 2 sign-off (or Stage 1 sign-off for `XS`/`S`).
- **Action**: Translate spec to stories, then to failing Go tests. Tests land on the feature branch and **fail as expected**. Architect provides exact file scope; QA rejects tasks without scope.
- **Exit**: Failing tests committed. The Coder's job is to make them pass.
- **Anti-pattern**: "Test along with implementation." That is not TDD. Tests precede implementation in time and in commit history.

### Stage 4 — Implementation

- **Owner**: **Coder**.
- **Tools the Coder invokes**: `a9s-coder` (Go production code), `a9s-integrator` (cross-package wiring, `internal/tui/app.go`, message flow), `a9s-fixtures` (demo/test fixtures via `a9s-create-demo-fixture` skill).
- **Trigger**: Stage 3 tests landed and red.
- **Action**: Make the failing tests pass. Touch only files in the Architect's scope. Rebuild the binary (`make build`) after every change.
- **Exit**: Tests pass; `make build && make test && make lint && make gofix && make security` green locally.
- **Anti-pattern**: Coder writes new tests instead of routing back to QA. Coder edits files outside the Architect's scope. Coder skips `make gofix`.

### Stage 5 — Review

Reviewers run **in parallel**. The PR cannot proceed until every applicable reviewer signs off. Every reviewer in this matrix is a **Paperclip agent**; the "Tools they invoke" column lists the in-session subagents/skills they use during the review.

| Reviewer (Paperclip agent) | Always runs? | Trigger condition | Tools they invoke in-session |
|---|---|---|---|
| **CodeReviewer** | Yes | Any code change | `a9s-tui-reviewer` (TUI files), `a9s-consistency-checker`, `test-coverage-analyzer`, `a9s-docs-reviewer` (docs files), `tui-ux-auditor` (UX) |
| **CodexReviewer** | Yes | Any code change | Codex pair-review pass — independent of CodeReviewer |
| **Architect** | Yes for size ≥ `M` | Architecture checklist score; target ≥ 8.5 / 10 | `a9s-architect` (design re-read), `a9s-arch-review` skill |
| **CTO** | Yes | Final sign-off; all above must be green first | `a9s-security-auditor` (when `internal/aws/` or deps changed) |
| CodeRabbit (external) | Yes | One pass per push (batch fixes into one push) | n/a — external service, not a Paperclip agent |

Notes:

- A subagent id appearing in the "Tools" column is a tool the human-side Paperclip reviewer invokes. It does **not** sign off on its own. Sign-off is the Paperclip agent's act.
- Docs-only PRs still run CodeReviewer (with `a9s-docs-reviewer` and `a9s-consistency-checker`) and CodexReviewer; Architect is skipped if size < `M`; CTO final sign-off still required.
- **Exit**: All applicable Paperclip reviewers thumbs-up; CodeRabbit either resolved or `@coderabbitai ignore`d with reason.
- **Anti-pattern**: Multiple push-fix-push cycles to chase reviewers. Get it right locally; push once.

### Stage 6 — Pre-push Validation (single command)

Run by **whoever pushes** the branch (typically Coder, occasionally CTO for hotfix or doc PRs).

```bash
make ready-to-push
```

This target is the canonical gate. It MUST pass locally with zero edits before any push. It runs:

1. `make test-race` — unit tests with race detector.
2. `make lint` — golangci-lint.
3. `make security` — govulncheck.
4. `make gofix` — `//go:fix inline` directives applied.
5. `make verify-readonly` — read-only invariant.
6. `make check-readme` — README in sync with `docs/shared/`.
7. `make snapshot` — golden-file render checks.
8. `make mdlint` — markdown lint across `docs/`, `CLAUDE.md`, `CONTRIBUTING.md`, `CHANGELOG.md`.

For changes that touch `internal/aws/` real-account behavior, additionally run the live integration test against a real AWS profile (this is also the entry to Stage 6.5):

```bash
A9S_CT_PROFILE=<profile> go test -tags integration ./tests/integration/ \
  -run TestFullRelatedViewValidation -count=1 -v -timeout 600s
```

- **Exit**: All green locally. CI is verification, not debugging.
- **Anti-pattern**: "I'll let CI tell me if it's broken." That is a budget leak and an etiquette violation against reviewers.

#### Docs-only exception

For pure docs changes (`*.md`, `docs/`, `website/`, `specs/`, `.claude/`, `LICENSE`), `make ready-to-push` is **not** required. `make mdlint` is.

### Stage 6.5 — Post-merge real-AWS validation

- **Owner**: **E2ETester**.
- **Trigger**: Any merge to `main` that touches `internal/aws/`, fetchers, child views, related-resource pivots, or fixtures. Skipped for pure-docs and pure-tooling changes.
- **Action**: Run the integration suite against a real AWS profile (`A9S_CT_PROFILE=<profile>`), exercise the changed surface (golden-path: list → detail → child view → related view), and capture pass/fail per scenario. File a P1 incident issue (assignee CTO) if any real-AWS scenario regresses.
- **Tools the E2ETester invokes**: `a9s-bug-hunt-real-profile` skill, the integration test binaries under `tests/integration/`.
- **Exit**: All real-AWS scenarios green, or an incident issue exists with the regression scoped and a follow-up Coder issue created.
- **Anti-pattern**: Treating Stage 6 (`make ready-to-push`) as sufficient for changes that depend on real AWS API behavior.

### Stage 7 — Merge & Release

- **Merge to `main`**: only with green CI plus Stage 5 sign-offs.
- **Release path** (when cutting a tagged version):
  - **Owner**: **CTO**.
  - **Tools the CTO invokes**: `a9s-release-validator` subagent (pre-tag checklist — GoReleaser config, multi-arch builds, changelog, CI), `release.md` skill.
  - **E2ETester** runs the full real-AWS pass before the tag is pushed and signs off.
  - Steps:
    1. CTO runs `make ready-to-release` (Stage 6 gates + integration). All green.
    2. `a9s-release-validator` reports the pre-tag checklist; CTO resolves any flag.
    3. `CHANGELOG.md` updated with a Keep-a-Changelog entry; `releases/vX.Y.Z.md` written.
    4. `docs/architecture.md` aligned with the codebase. Outdated architecture docs are a release blocker.
    5. **Busywork audit**: every test added or modified in the release is reviewed and deleted if it is a tautology, a mock asserting its own input, a struct-shape pin instead of a behavior pin, or duplicate coverage. Coverage earned by busywork is a liability.
    6. E2ETester signs off after a real-AWS pass on the release commit.
    7. Tag pushed; GoReleaser publishes; release notes go live.
- **Exit**: Tag exists, artifacts published, `releases/vX.Y.Z.md` committed.

### Stage 8 — Retro (weekly, ~15 minutes)

- **Owner**: **CTO**.
- Every week, the CTO writes a single-comment retro on the program tracking issue covering:
  - DORA snapshot: PRs merged, lead time median, change-failure rate (PRs requiring a follow-up fix), MTTR for any bug landed in `main`.
  - One thing the process did well.
  - One thing the process slipped on, and the **specific** rule change that prevents it next time. Update this document in the same heartbeat; do not "remember to fix it later."

Retros that produce no rule change are a smell. Either the process is perfect (it isn't) or the retro is performative.

## Agent Orchestration — who runs when

Every "Primary" cell holds a **Paperclip agent name**. The "Tools" column lists Claude Code subagents/skills the Primary invokes in-session.

| Stage | Primary (Paperclip) | Helpers (Paperclip) | Reviewers (Paperclip) | Tools invoked in-session |
|---|---|---|---|---|
| 1 Intake | CTO | — | — | — |
| 2 Spec | Architect | DevOps (consultative, optional) | CTO | `a9s-resource-spec`, `a9s-architect` |
| 3 Tests | QA | — | Architect | `a9s-qa-stories`, `a9s-qa`, `a9s-related-qa` |
| 4 Impl | Coder | — | — | `a9s-coder`, `a9s-integrator`, `a9s-fixtures` |
| 5 Review | (parallel reviewers below) | — | CodeReviewer, CodexReviewer, Architect (≥M), CTO (final), CodeRabbit (external) | `a9s-tui-reviewer`, `a9s-consistency-checker`, `test-coverage-analyzer`, `a9s-security-auditor`, `a9s-docs-reviewer`, `tui-ux-auditor`, `a9s-arch-review` |
| 6 Validate | (whoever pushes — usually Coder) | — | — | `make ready-to-push` |
| 6.5 Post-merge AWS | E2ETester | — | CTO (incident triage) | `a9s-bug-hunt-real-profile`, `tests/integration/` |
| 7 Release | CTO | E2ETester (real-AWS sign-off) | CTO | `a9s-release-validator`, `release.md` skill |
| 8 Retro | CTO | — | — | — |

Skill triggers (invoked in-session by the owning Paperclip agent above):

- New resource type → `a9s-resource-spec` (Stage 2, Architect) → `a9s-implement-resource` (Stages 3–4, QA + Coder).
- New child view → `a9s-add-child-view` (Stage 2 Architect scopes; QA implements tests; Coder implements code).
- New related view → `a9s-add-related-view` (same split).
- New attention column → `a9s-add-attention-column`.
- End-to-end issue → `a9s-implement-issue` (Architect orchestrates across stages 2–7).
- Architecture review → `a9s-arch-review` (Stage 5, Architect).
- Bug from real account → `a9s-bug-hunt-real-profile` (Stage 6.5, E2ETester).

## Branching, Commits, PRs

- **Trunk-based**: `main` is always releasable. Feature branches are short-lived (≤ 1 week) and named `<area>/<short-slug>` (e.g. `process/sustainable-development`, `phase-04-pr04a-catalog-bootstrap`).
- **One PR per concern**. Refactor PRs follow the per-PR spec in `docs/refactor/<phase>.md`.
- **Conventional Commits**. `feat:`, `fix:`, `refactor:`, `docs:`, `chore:`, `test:`, `ci:`. The 40-PR refactor uses `feat(phase-<n>-<pr>): …` consistently.
- **Co-Authored-By**: every commit ends with `Co-Authored-By: Paperclip <noreply@paperclip.ing>`.
- **`@coderabbitai ignore`** on PRs that don't need a re-review; `[skip ci]` on trivial follow-ups.
- **Never `--no-verify`, never `--no-gpg-sign`**. Hook failures are diagnosed, not bypassed.
- **Never amend a published commit**. Always create a new commit.

## Bug Triage

- **P0 (critical)**: data corruption, crash on launch, security regression. Drop everything; CTO owns immediately. Hotfix branch off `main`.
- **P1 (high)**: broken core workflow, real AWS profile fails. Schedule into the current refactor sprint.
- **P2 (medium)**: edge case, demo-only, cosmetic. Schedule against the next release.
- **P3 (low)**: nice-to-have. Backlog only; never auto-promoted.

A bug fix follows the same lifecycle. The cheap path for `XS` bug fixes is Stages 1 → 3 → 4 → 5 → 6 → 7 (skip 2 and 6.5 if the change does not touch real-AWS surface). Even cheaper paths are not allowed; "I just changed one line" is how regressions hide.

## Incident & Rollback

If a regression lands in `main`:

1. CTO files an incident issue with timestamp, symptoms, suspected commit.
2. If user-visible and not safely forward-fixable in <60 minutes: revert the offending commit. Reverts are commits; they go through Stage 5 (lightweight review by CTO + CodeReviewer) and Stage 6.
3. After mitigation: a written post-incident note in the incident issue covering root cause, blast radius, what gate failed to catch it, what gate change prevents recurrence. The gate change lands the same week.

A regression that the process did not catch is a process bug, not a coder bug. If the regression was real-AWS-only, the gate change usually lands in Stage 6.5 (E2ETester scope expansion).

## Metrics (DORA-flavored)

Tracked per release in the release notes file:

- **Deployment frequency**: tagged releases per week.
- **Lead time for changes**: median commit-to-merge time on the release's PRs.
- **Change-failure rate**: PRs in the release that required a follow-up fix.
- **MTTR**: time from incident open to mitigation merged for any P0/P1 in the release.

These are not for vanity. They are the input to the weekly retro: any value that drifts the wrong way for two consecutive weeks is a retro topic.

## Anti-patterns (call them out, fix them)

- **Naming a subagent as a stage owner or reviewer.** Subagents are tools. Owners and reviewers are Paperclip agents from the roster above.
- **Routing implementation work to DevOps.** DevOps is consultative only. PRs, branches, fixtures, tests, doc edits → Coder (or QA / Architect / CTO per stage).
- **Reflexive backlog browsing.** Agents act only on explicit assignments.
- **"Test along with the code."** Tests precede implementation in commit order.
- **Push-fix-push cycles.** `make ready-to-push` runs locally before any push.
- **Skipped gates.** A gate skipped is a gate deleted; either run it or remove it from the gate list.
- **Dual-authoring.** Two sources of truth for the same fact. Always wrong.
- **Documentation drift.** Docs that contradict code are a release blocker, not a backlog item.
- **Heroic merges.** A PR that requires the author to babysit CI is a PR that broke Stage 6.

## Updating this document

When the process changes, the change lands as a single PR that:

1. Edits this document.
2. Edits any enforcement (Makefile target, PR template, CLAUDE.md pointer).
3. Mentions the retro that motivated the change, if applicable.

There is no parallel "process v2" doc. There is one document, and it always reflects the current rule.
