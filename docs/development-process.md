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

## Continuous Autonomous Execution (CAE-1)

Effective immediately, the development process is **self-driving**. The board sets direction and reviews outcomes. The board does not gate execution. Agents make tradeoff calls, document them, and continue.

The five rules of CAE-1:

1. **No plan-approval `request_confirmation` gates.** Architect spec is published → QA writes failing tests → Coder ships. Reviewers (CodeReviewer + CodexReviewer + Architect-for-size≥M) catch issues at Stage 5. Board can override post-hoc by commenting on issue or PR.

2. **CTO auto-pulls `todo, unassigned` work in active projects.** Every CTO heartbeat scans the active project (currently `refactoring`) for unassigned ready work, runs Stage 1 (size, AC, owner), and dispatches. The earlier anti-pattern "CTO does not browse the backlog" is removed for in-scope work. CEO still owns *net-new strategic direction*.

3. **No "routed to CEO" parking.** When an agent faces a technical tradeoff, the agent makes the call, documents the reason in a comment, and continues. The board reviews outcomes, not pre-decisions. The CTO is the highest in-process technical authority.

4. **Idle is forbidden when ready work exists.** When all of an agent's `in_progress` is parked on a real wait (PR review, blocker, monitor), the agent picks up its next assigned `todo` or any unblocked `blocked` work and starts. No heartbeat ends with idle status when ready assigned work remains.

5. **Manual intervention survives only for**: net-new role hiring, destructive AWS/git actions, real-money spend, and Paperclip-platform changes outside this repo.

**Board override clause.** The board (CEO / user) can override any in-process decision post-hoc by commenting on the issue or PR. Agents must respect that override the moment they observe it but **must never wait for it** before acting. Idle-pending-board is a process violation under CAE-1; the only valid waits are technical (PR review, CI, real blocker) — never governance.

**Why this is safe.** The Stage 5 reviewer matrix (CodeReviewer + CodexReviewer always; Architect ≥ M; CTO final sign-off) and the Stage 6 / 6.5 / 7 gates are unchanged. CAE-1 removes *gates that wait on the board*, not gates that wait on technical correctness. The board retains full visibility (every comment, every PR) and full override (a comment instantly redirects).

**Heartbeat §0 — fast-path live-work exit (AS-591 / AS-592).** Every routine-fired CTO heartbeat runs a single API call FIRST before any sweep: `GET /api/companies/{companyId}/issues?status=in_progress&limit=200` (per AS-630, `in_review` is banned, so the status set is `in_progress` only). Filter out the execution issue itself and other open CAE-1 routine execution issues (parent `d7aeab7a-…`). If the filtered count ≥ 1, the heartbeat closes `done` with a single-line "Fast-path exit (AS-591/AS-592): N live issues open …" comment and runs **none** of the §1–§10 sweeps. Only on a true-idle tick (filtered count == 0) does the heartbeat run the full sweep (stale-execution, auto-pull, `in_review` straggler migration AS-630, cascade blockers, null-project, Agent-Health Audit, PR-Gate Audit, E2E heartbeat, open-issue liveness, pre-exit numbers). The principle: live work is its own liveness signal; multi-table drift detection adds zero value while other agents are mid-flight, and the token cost is non-trivial. Direct-assigned issues (`assigneeAgentId` = CTO, not the routine parent) are unaffected — those still get full Stage 1 / Stage 5 / Stage 7 work.

## `in_review` is PROHIBITED · no board decisions on technical issues (AS-630, 2026-05-12)

CEO directive on AS-630 — overrides any later contradicting text in this doc.

1. **`in_review` is a banned issue status.** Do not transition any Paperclip issue to `in_review`. Do not query `status=in_review` in any sweep — replace with `status=in_progress` (or `blocked` for governance waits). The board found the status confusing: agents conflated `in_review` (a Paperclip status field) with "code review" (Stage 5 of the SDLC, which lives on the PR thread via labels + reviewer verdicts). Valid statuses: **`todo`, `in_progress`, `blocked`, `done`, `cancelled`**.

2. **No technical issue ever requires a board decision.** Technical tradeoffs — architecture, design, scope, library choice, gate findings, regressions, reviewer disputes, refactor sequencing — are **CTO + CEO** responsibility (CTO is highest in-process technical authority; CEO is the escalation above CTO). Do not file `request_confirmation` interactions on the board for technical questions. Do not write "routed to board" / "escalate to board" on technical issues. CAE-1 already removed plan-approval gates; AS-630 extends the rule across every technical decision class.

3. **What still goes to the board** (governance-only, narrow): net-new role hiring, destructive AWS/git actions, real-money spend, Paperclip-platform changes outside this repo. These remain `request_confirmation` candidates, with the source issue sitting in `blocked` (with the interaction id named in the blocker text) — never `in_review`.

4. **Replacement table for every prior use of `in_review`:**
   - PR open, awaiting Stage 5 reviewers → issue stays `in_progress`; PR labels (`stage5-pending` / `stage5-complete` / `stage5-changes-requested`) + GitHub PR review state ARE the gating signal.
   - Awaiting a governance `request_confirmation` (hiring, spend, destructive op) → `blocked` with the interaction id in the blocker text.
   - Awaiting E2ETester real-AWS sign-off → `in_progress` with a comment naming the wait.
   - Awaiting a child issue to complete → `blocked` with `blockedByIssueIds` set.
   - Reviewer (CodeReviewer / CodexReviewer / Architect-for-≥M) Stage 5 issue → `in_progress` while reviewing; **`done` once the verdict is posted on both Paperclip and the PR**.

5. **Stragglers migration (one-time).** CTO Step C1.5 now sweeps any pre-AS-630 `in_review` issue and transitions it: `done` if the linked PR merged and the deliverable shipped; `in_progress` if the PR is still open; `blocked` only if a genuine governance interaction is pending. The sweep becomes a no-op once the platform is clean.

## Issue-creation discipline (anti-proliferation, mandatory)

Paperclip issues are the unit of *engineering work the user expects to see in their inbox*. They are **not** the audit log of every governance act, every heartbeat tick, every recovery action. Conflating those two roles is what turned the 40-PR refactor program into 432 issues. The following five rules are mandatory and apply to every agent.

1. **One review issue per PR (not one per reviewer).** Stage 5 creates exactly **one** Paperclip issue per PR — "Stage 5 review of PR #N". CodeReviewer, CodexReviewer, and (for size ≥ M) Architect post their verdicts as **comments on that single issue** plus the mandatory GitHub PR cross-post. The Architect's `make ready-to-push` and CTO final sign-off are comments on the same issue. **Never** spawn separate `Stage 5 — CodeReviewer / CodexReviewer / Architect` issues. A rework round (R2, R3, …) appends comments on the existing review issue — it does not create a new issue.

2. **Heartbeat ticks do not create issues.** Routine cron firings are *agent wakes*, not persistent records. Each routine has at most one running umbrella issue ("CAE-1 heartbeat — current cycle"); successive ticks **append comments** to that single issue. New Paperclip issues are created only when a tick takes a concrete engineering action (e.g. dispatches a real Coder task) — and then the new issue tracks that action, not the tick.

3. **Drift-detection cap: at most 1 new issue per heartbeat per agent.** When a sweep detects drift (`in_review` straggler per AS-630, blocked-without-blocker, stale lock, projectId=null, etc.), record **every** finding as a comment on the existing affected issue. At most **one** new follow-up issue per heartbeat may be filed, and only when the drift requires bounded work that cannot be expressed as a status change on an existing issue. Multiple drift types in a single tick collapse into one summary comment on the heartbeat umbrella issue plus, at most, one new issue.

4. **Recovery is a comment, not a child issue.** When an agent is found stalled or in `error`, post a comment on the **stalled issue itself** (and escalate to CTO/CEO if the platform-level recovery is needed). **Never** create a "Recover stalled issue AS-X" child. The stalled issue's `assigneeAgentId` is already correct; status transitions remain that assignee's call (recovery agents are advisory, see Stage 5 §"Recovery agents — no parent status transitions").

5. **API probes are never issues.** Test API endpoints with throwaway curl / scripts. Issues whose title contains "probe", "test", "delete me", "_probe" etc. are a violation of this rule, full stop.

**Why these rules.** AS-446 (2026-05-11) audit: 432 total issues, of which ~145 were legitimate engineering work and ~287 were process apparatus. The four levers that caused the proliferation map 1-to-1 to rules 1–4 above; rule 5 captures the residual probe-spam. Both the CEO and CTO audits converged on the same conclusion: process was followed; the process was the bug.

**Enforcement.** This is part of the Definition of Done for every issue. A PR may carry a Stage-5-tagged label only if exactly one review issue exists for it. The CTO retro (Stage 8) MUST include a per-week count of issues created per PR shipped; a value > 1.5 trips a process-fix entry in the same retro.

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

Every unit of work goes through these stages. Stages 1, 2, 4, 6, 6.5 may be **skipped** for trivial bug fixes (`XS`, single file, no behavior change visible to users). Stages 3, 5, 7 never skip — **except in the Trivial-docs fast-path below, which is the only lane that collapses Stages 2–5 into a single CTO authorship step.**

```text
1. Intake → 2. Spec → 3. Tests → 4. Impl → 5. Review → 6. Validate → 6.5 Post-merge AWS → 7. Release → 8. Retro
```

### Trivial-docs fast-path (`XS`-docs) — AS-595

**Why this lane exists.** AS-592 (2026-05-12) was a `+2 / -0` one-file addition to this document. Filed as P0 with full feature-style acceptance criteria, it triggered Architect spec → QA dispatch → Coder impl → CodeReviewer + CodexReviewer + Architect Stage 5 reviews → PR #368 → AS-594 as a separate Stage 5 review tracker. For two lines of markdown. The board (AS-595) was furious; the chain was process-correct under the rules as written; the rules were wrong. This lane fixes that.

**When the lane applies — all of the following must hold:**

1. The change touches **only** `*.md`, `docs/`, `website/`, `specs/`, `.claude/`, `LICENSE`, or `CHANGELOG.md`. No Go source, no tests, no fixtures, no Makefile, no `.github/workflows/`, no `internal/`, no `cmd/`.
2. Size is `XS`: ≤ 30 LOC added/changed across ≤ 2 files.
3. The change is one of: typo fix, link fix, formatting/style fix, clarification, or **codifies a CEO/CTO policy decision already made on the issue thread** (the diff IS the decision).
4. The change does **not** reverse an existing rule. Reversing a rule is a policy change and routes through normal Stage 1–5.

**Lane procedure (single Paperclip agent, single heartbeat):**

| Step | Owner | Action |
|---|---|---|
| 1 | CTO | Confirm the four conditions above hold. If any fails, route through normal Stages 1–5. |
| 2 | CTO | Author the diff directly. **No** Architect spec, **no** QA dispatch, **no** Coder dispatch. |
| 3 | CTO | Branch from `main`. Single commit. Conventional commit message (`docs(scope): ...`). `Co-Authored-By: Paperclip <noreply@paperclip.ing>`. |
| 4 | CTO | Run `make mdlint` locally. This is the **only** gate. |
| 5 | CTO | Open PR with `[trivial-docs]` prefix in the title. PR body is one sentence plus the rationale. |
| 6 | CTO | Wait for CI to go green (Markdown lint, Detect changes, the skip-aware short path). |
| 7 | CTO | Self-merge (squash) after CI green. **No** separate Paperclip Stage 5 review-issue, **no** Architect/QA/Coder dispatch, **no** CodeReviewer/CodexReviewer Stage 5 verdict required (they may still comment on the PR; their comments are advisory, not gating). |

**CEO direct-authorship (narrow):** when the CEO is codifying a CEO policy decision they have just made on the issue thread, the CEO MAY author the trivial-docs PR themselves under this lane (CEO replaces CTO in steps 2–7 above). This is the exception to "CEO does not write doc edits"; it exists because routing a CEO-made policy decision through CTO authorship for a 2-line MD edit is exactly the over-process this lane fixes. The CEO's own `AGENTS.md` carries the matching authorization.

**What the lane explicitly removes (vs. normal Stages 1–5):**

- Stage 2 spec — no `[spec-published]` comment, no `docs/refactor/…` or `specs/…` doc.
- Stage 3 tests — there is nothing to test; tests are not a hazing ritual.
- Stage 4 implementation — collapses into the authorship step.
- Stage 5 review-issue — **never** spawn a separate `Stage 5 review of PR #N — trivial-docs` Paperclip issue. The PR is its own record.
- Stage 5 mandatory CR/CXR/Arch verdicts and PR cross-posts — not required. (They are required for **every other** PR; see Stage 5 below.)

**What the lane preserves:**

- Stage 6 docs-only exception still applies (`make ready-to-push` not required; `make mdlint` is).
- Stage 6.5 not triggered (no `internal/aws/` touch by definition).
- Stage 7 release flow unchanged for downstream releases; trivial-docs changes do not bump the release artifact unless `CHANGELOG.md` is involved.
- Stage 8 retro: trivial-docs PRs count toward the per-week issues-per-PR audit at the favorable ratio (0 spawned issues per PR is the target).

**Anti-patterns (this lane fails when):**

- The CEO files a trivial-docs change with feature-style acceptance criteria (1–N), files-to-touch, done-when. That shape is the SDLC-routing signal; CTO will treat it as a feature. **Mark trivial-docs issues `[trivial-docs]` in the title and limit the body to one sentence plus the exact diff.**
- The CTO dispatches Architect, QA, or Coder for a trivial-docs change. The CTO is the author.
- A separate Stage 5 review-issue is spawned (`Stage 5 review of PR #N — trivial-docs (XS)`). The PR is the record; no Paperclip issue is needed for the review.
- A CodeReviewer/CodexReviewer/Architect verdict is treated as gating on a trivial-docs PR. Their comments are advisory only on this lane.
- The lane is stretched to cover policy reversals or non-docs files. If in doubt, route through normal Stage 1–5.

### Stage 1 — Intake

- **Owner**: **CTO**.
- **Trigger**: CEO/board files an issue, *or* a `todo, unassigned` issue exists in the active project (under CAE-1, the CTO auto-pulls these every heartbeat).
- **Action**: Triage type (bug · feature · refactor · ops · docs), set priority, set size, name the owning Paperclip agent, draft acceptance criteria. Dispatch immediately; no waiting for explicit per-issue CEO approval on in-scope work.
- **Exit**: Issue meets DoR and is dispatched to the owning agent (or to Architect when size ≥ M).
- **Anti-pattern (post-CAE-1)**: Self-assigning `todo, unassigned` issues *outside* the active project — CTO auto-pull is in-scope only. Other agents (Architect, QA, Coder, etc.) browsing the backlog or picking up undispatched work — they still act only on explicit dispatch from CTO or Architect. Waiting on CEO/board approval before dispatching in-scope ready work.

### Stage 2 — Spec & Design

- **Owner**: **Architect**.
- **Tools the Architect invokes**: `a9s-resource-spec` skill (writes `docs/resources/<short>.md`), `a9s-architect` subagent (design output only — writes nothing into source).
- **Optional consult**: **DevOps** for AWS-practitioner priority sanity ("which 10 resources next?", "is CWL more important than Lambda?").
- **Trigger**: DoR met and size ≥ `M`. Skipped for `XS`/`S` bug fixes.
- **Action**: Produce a spec doc. Resources use `a9s-resource-spec`. Refactor PRs reference the per-PR spec in `docs/refactor/`. Features write to `specs/<n>-<feature>.md`.
- **Exit (CAE-1)**: Spec doc committed to the feature branch with a `[spec-published]` comment on the issue and a scoped dispatch to **QA** for Stage 3 in the same heartbeat. **No `request_confirmation` plan-approval gate.** The spec is the contract; existing implementation is disposable. Stage 5 reviewers (CodeReviewer + CodexReviewer + Architect-for-size≥M + CTO final) catch spec-vs-diff mismatches; the board can override post-hoc by commenting on the issue or PR.
- **Anti-pattern**: Skipping the spec for "obvious" features. If it is so obvious, the spec is one paragraph — write it anyway.
- **Anti-pattern (post-CAE-1)**: Posting the spec and then waiting for CEO/CTO/board approval before dispatching QA. Spec is published → QA picks up. Approval is *not* an entry condition for Stage 3.

### Stage 3 — Tests

- **Owner**: **QA**.
- **Tools the QA agent invokes**: `a9s-qa-stories` (given/when/then with zero source-code knowledge), `a9s-qa` (failing Go tests), `a9s-related-qa` (related-view scope).
- **Trigger**: Spec-published comment from Architect (size ≥ M) or scoped dispatch from CTO (`XS`/`S`). **No plan-approval `request_confirmation` gate** under CAE-1; QA proceeds on the published spec.
- **Action**: Translate spec to stories, then to failing Go tests. Tests land on the feature branch and **fail as expected**. Architect provides exact file scope; QA rejects tasks without scope.
- **Exit (CAE-1)**: Failing tests committed and a scoped dispatch to **Coder** for Stage 4 in the same heartbeat. The Coder's job is to make them pass.
- **Anti-pattern**: "Test along with implementation." That is not TDD. Tests precede implementation in time and in commit history.
- **Anti-pattern (post-CAE-1)**: Pre-circulating a test plan for board/CTO sign-off before writing the failing tests. The test code itself is the test plan; missing coverage is caught at Stage 5 by CodeReviewer/`test-coverage-analyzer`.

### Stage 4 — Implementation

- **Owner**: **Coder**.
- **Tools the Coder invokes**: `a9s-coder` (Go production code), `a9s-integrator` (cross-package wiring, `internal/tui/app.go`, message flow), `a9s-fixtures` (demo/test fixtures via `a9s-create-demo-fixture` skill).
- **Trigger**: Stage 3 tests landed and red. **No implementation-approach `request_confirmation` gate** under CAE-1; Coder proceeds on the scoped dispatch.
- **Action**: Make the failing tests pass. Touch only files in the Architect's scope. Rebuild the binary (`make build`) after every change.
- **Exit**: Tests pass; `make build && make test && make lint && make gofix && make security` green locally.
- **Anti-pattern**: Coder writes new tests instead of routing back to QA. Coder edits files outside the Architect's scope. Coder skips `make gofix`.
- **Anti-pattern (post-CAE-1)**: Posting an implementation plan and waiting for approval before writing code. The diff itself is the plan; design issues are caught at Stage 5 by Architect-for-size≥M.

### Stage 5 — Review

Reviewers run **in parallel** against **a single review issue per PR** (see §"Issue-creation discipline" rule 1). The PR cannot proceed until every applicable reviewer signs off. Every reviewer in this matrix is a **Paperclip agent**; the "Tools they invoke" column lists the in-session subagents/skills they use during the review. **Each reviewer posts their verdict as a comment on the shared review issue plus the GitHub PR cross-post — they do not spawn a separate Paperclip issue for their own role.**

| Reviewer (Paperclip agent) | Always runs? | Trigger condition | Tools they invoke in-session |
|---|---|---|---|
| **CodeReviewer** | Yes | Any PR | `a9s-tui-reviewer` (TUI files), `a9s-consistency-checker`, `test-coverage-analyzer`, `a9s-docs-reviewer` (docs files), `tui-ux-auditor` (UX) |
| **CodexReviewer** | Yes | Any PR | Codex pair-review pass — independent of CodeReviewer |
| **Architect** | Yes for size ≥ `M` | Architecture checklist score; target ≥ 8.5 / 10 | `a9s-architect` (design re-read), `a9s-arch-review` skill |
| **CTO** | Yes | Final sign-off; all above must be green first | `a9s-security-auditor` (when `internal/aws/` or deps changed) |
| CodeRabbit (external) | Yes | One pass per push (batch fixes into one push) | n/a — external service, not a Paperclip agent |

Notes:

- A subagent id appearing in the "Tools" column is a tool the human-side Paperclip reviewer invokes. It does **not** sign off on its own. Sign-off is the Paperclip agent's act.
- Docs-only PRs still run CodeReviewer (with `a9s-docs-reviewer` and `a9s-consistency-checker`) and CodexReviewer; Architect is skipped if size < `M`; CTO final sign-off still required.
- **Verdict cross-post (CodeReviewer + CodexReviewer, mandatory).** Both reviewers MUST cross-post their final verdict (APPROVED / NEEDS CHANGES / REJECTED) as a `gh pr comment` on the GitHub PR thread, in addition to whatever they post on the Paperclip review-issue thread. The Stage 5 audit rule presumes a human-side reviewer can read verdicts on the PR itself; a verdict that lives only inside Paperclip violates that contract. The PR-thread comment must repeat the verdict label, the gate findings (with `file:line` citations), and the next owner — i.e. it is the same content as the Paperclip-thread post, not a "see Paperclip" pointer. CodeRabbit comments still do not count as Stage 5. Example invocation:

  ```bash
  gh pr comment <n> --repo k2m30/a9s --body "$(cat <<'EOF'
  CodexReviewer Stage 5 verdict: NEEDS CHANGES
  - [GATE] path/to/file.go:42 — finding + required fix
  - Ruling: rule cited
  Next owner: Coder
  EOF
  )"
  ```

  The cross-post is part of the Stage 5 deliverable; the PR-Gate audit treats the absence of a cross-post the same as the absence of a verdict — a missing cross-post means the PR stays `stage5-pending` for that reviewer regardless of what the Paperclip thread says.
- **Exit**: All applicable Paperclip reviewers thumbs-up (Paperclip-thread verdict **and** GitHub PR cross-post present for CodeReviewer + CodexReviewer); CodeRabbit either resolved or `@coderabbitai ignore`d with reason.
- **Anti-pattern**: Multiple push-fix-push cycles to chase reviewers. Get it right locally; push once.
- **Anti-pattern**: Posting a Stage 5 verdict only on the Paperclip thread. The PR is the human-readable record of who signed off on what; an audit reading only the GitHub PR cannot reconstruct Stage 5 unless the cross-post is present.
- **Anti-pattern**: Filing one Paperclip issue per reviewer. Stage 5 is **one** review issue per PR; verdicts are comments on that issue. See §"Issue-creation discipline" rule 1.

#### Recovery is a comment, not a child issue

When an agent is detected stalled or in `error`, the response is:

1. **Post a comment on the stalled issue itself.** Name the agent, the symptom (heartbeat stale, lock held, status frozen), and the recommended unblock action (resume agent, reassign, escalate).
2. **Escalate to CTO/CEO** if a platform-level action (`PATCH /api/agents/{id}` status reset, force-release) is required — the platform agent that can act on that is the CEO (bearer-scope).
3. **Do NOT create a "Recover stalled issue AS-X" child issue.** That pattern multiplies the work-tracker without adding signal; the stalled issue is the canonical record.

When a comment-only recovery is genuinely insufficient (e.g. a bounded follow-up engineering task must be done by a different agent before the original can resume), the comment may say so and a single delegated follow-up issue may be filed under §"Issue-creation discipline" rule 3.

##### Recovery agents — no parent status transitions

A Paperclip child whose role is "recover stalled issue X" — or any agent operating on a parent it does not own as `assigneeAgentId` — **may post comments recommending a status transition on the parent, but MUST NOT call `PATCH /api/issues/{parentId}` to change `status`**. Only the parent's currently-assigned agent transitions status.

Exceptions:

- The CEO or CTO transitioning issues they own.
- An agent operating on its own (assigned) issue.
- The board acting via `/api/issues/{id}` directly (governance-level override).

Why this rule exists: AS-70 (CTO-owned, PR-gate-enforcement program issue) was closed as `done` by a "Recover stalled issue AS-70" child (UUID prefix `b599c3b2`) while two PRs were still open and Stage 5 sign-off was the CTO's call. AS-70 had to be reopened the same heartbeat (2026-05-10). Recovery / supervisor agents' dispositions on a parent are **recommendations**, not decisions.

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

#### Test suite wall budget

`make test` (non-race) MUST complete in ≤ **5 minutes** wall on standard CI hardware (the AS-6 baseline). If a change pushes the suite past that, the author must do one of:

- (a) add `t.Parallel()` + row capture to the new tests;
- (b) split the package into a sub-package (suggested boundary: any single package whose aggregate test wall exceeds 60 s, per the AS-24 finding);
- (c) attach a `go test -json` profile + a written plan to the PR description and request CTO sign-off as part of Stage 5.

Parallelization and shared-fixture techniques are tracked under AS-26 and AS-27; the profile baseline lives in AS-24.

Enforced by the `test-budget` CI job (`scripts/test-budget-gate.sh`, `.github/workflows/ci.yml`). The job fails the build when `make test` (non-race) exceeds 5 minutes wall on `ubuntu-latest`; macOS and Windows are observed via the `test` matrix but not gated. The race-detector wall is not gated here — race timing is non-deterministic across runners and is exercised in the nightly matrix per [AS-25](/AS/issues/AS-25).

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
- **Trigger** — fires when **any** of the following holds:
  1. **Per-PR (surface trigger)**: a merge to `main` touches `internal/aws/`, fetchers, child views, related-resource pivots, or fixtures. Skipped for pure-docs and pure-tooling changes.
  2. **Phase-boundary (batch trigger)**: at least **3 PRs** have merged in an in-flight refactor program (a `docs/refactor/<phase>.md` cluster, or any multi-PR program tracked by a parent issue) since the last Stage 6.5 sign-off on `main`, *regardless of whether they touch `internal/aws/`*. The CTO MUST dispatch a batch real-AWS pass before opening the next phase. Precedent: [AS-133](/AS/issues/AS-133) (Phase-03 batch, 2026-05-10) and [AS-79](/AS/issues/AS-79) (Phase-04 batch). The phase-boundary rule was codified in [AS-363](/AS/issues/AS-363) after CAE-1 4×'d merge throughput exposed that mocks cannot fully cover large refactor surfaces (runtime extraction, session ownership migrations) even when no individual PR touches `internal/aws/`.
- **Action**: Run the integration suite against a real AWS profile (`A9S_CT_PROFILE=<profile>`), exercise the changed surface (golden-path: list → detail → child view → related view) across ≥ 4 distinct resource types when triggered by the phase-boundary rule, and capture pass/fail per scenario. File a P1 incident issue (assignee CTO) if any real-AWS scenario regresses.
- **Tools the E2ETester invokes**: `a9s-bug-hunt-real-profile` skill, the integration test binaries under `tests/integration/`.
- **Exit**: All real-AWS scenarios green, or an incident issue exists with the regression scoped and a follow-up Coder issue created.
- **Anti-patterns**:
  - Treating Stage 6 (`make ready-to-push`) as sufficient for changes that depend on real AWS API behavior.
  - **Over-correcting to per-PR real-AWS** for refactor moves that do not touch `internal/aws/`. The phase-boundary rule is the batch trigger, not "every PR". Per-PR Stage 6.5 for pure-runtime-move PRs is wasteful and slows the loop.
  - Letting `lastHeartbeatAt` for E2ETester drift > 48h while merges to `main` continue. The CAE-1 routine (routineId `4077ee95-6caa-402f-b156-34052fc19e5f`) carries a standing detection check for this gap; see [AS-363](/AS/issues/AS-363) AC3 and CTO `AGENTS.md` §"CAE-1 Heartbeat Scan".

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
| 2 Spec | Architect | DevOps (consultative, optional) | — *(post-hoc only under CAE-1; CTO/board may comment but Stage 2 has no exit gate)* | `a9s-resource-spec`, `a9s-architect` |
| 3 Tests | QA | — | Architect | `a9s-qa-stories`, `a9s-qa`, `a9s-related-qa` |
| 4 Impl | Coder | — | — | `a9s-coder`, `a9s-integrator`, `a9s-fixtures` |
| 5 Review | (parallel reviewers below) | — | CodeReviewer, CodexReviewer, Architect (≥M), CTO (final), CodeRabbit (external) | `a9s-tui-reviewer`, `a9s-consistency-checker`, `test-coverage-analyzer`, `a9s-security-auditor`, `a9s-docs-reviewer`, `tui-ux-auditor`, `a9s-arch-review` |
| 6 Validate | (whoever pushes — usually Coder) | — | — | `make ready-to-push` |
| 6.5 Post-merge AWS *(per-PR surface trigger OR phase-boundary batch — see §Stage 6.5)* | E2ETester | — | CTO (incident triage) | `a9s-bug-hunt-real-profile`, `tests/integration/` |
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

## Operations runbook (platform-layer incidents)

Software incidents go through Stage 6.5 / "Incident & Rollback" below. **Platform-layer incidents** — agents stuck in `error`, recovery primitives, bearer-scoped operations — live in `docs/runbook.md`. Read that file first when something is wrong with an agent's lifecycle (not its code).

The runbook is the single home for the CEO-bearer `error → idle` recovery primitive (`PATCH /api/agents/{id}` with `{"status":"idle"}`) and the constraints around it. Future on-call decisions should consult `docs/runbook.md`, not the originating incident issue body.

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
- **Reflexive backlog browsing *outside the active project*.** Under CAE-1, the **CTO auto-pulls** in-scope `todo, unassigned` issues every heartbeat (Stage 1) and dispatches. Other agents (Architect, QA, Coder, etc.) still act only on explicit dispatch from CTO or Architect — they do not browse the backlog.
- **Plan-approval gates.** No `request_confirmation` for spec / test plan / implementation plan. Reviewers catch issues at Stage 5; the board overrides post-hoc by commenting. Manual intervention survives only for: net-new role hiring, destructive AWS/git actions, real-money spend, and Paperclip-platform changes outside this repo.
- **"Routed to CEO" parking.** When an agent faces a technical tradeoff, the agent makes the call, documents the reason in a comment, and continues. The CTO is the highest in-process technical authority.
- **Idle on ready work.** No heartbeat ends with `idle` status when ready assigned `todo` (or unblocked `blocked`) work remains.
- **"Test along with the code."** Tests precede implementation in commit order.
- **Push-fix-push cycles.** `make ready-to-push` runs locally before any push.
- **Skipped gates.** A gate skipped is a gate deleted; either run it or remove it from the gate list.
- **Dual-authoring.** Two sources of truth for the same fact. Always wrong.
- **Documentation drift.** Docs that contradict code are a release blocker, not a backlog item.
- **Heroic merges.** A PR that requires the author to babysit CI is a PR that broke Stage 6.
- **One issue per reviewer per PR.** Stage 5 is **one** review issue per PR; verdicts are comments on that issue (see §"Issue-creation discipline" rule 1). Spawning `Stage 5 — CodeReviewer / CodexReviewer / Architect` triplets is a process-cost multiplier with no signal gain.
- **Heartbeat ticks persisted as issues.** Routine cron firings are wakes, not issues (rule 2). At most one running umbrella issue per routine; subsequent ticks are comments on it.
- **Drift-detection issue spawning.** A sweep that finds N items of drift files **at most one** new issue; the other N-1 findings are comments on the affected issues (rule 3).
- **"Recover stalled X" child issues.** Stalled-agent recovery is a comment on the stalled issue plus an escalation to CTO/CEO; never a new child (rule 4).
- **Filing API probes as issues.** Probes/tests of the Paperclip API go through curl/scripts, not issue creation (rule 5).

## Updating this document

When the process changes, the change lands as a single PR that:

1. Edits this document.
2. Edits any enforcement (Makefile target, PR template, CLAUDE.md pointer).
3. Mentions the retro that motivated the change, if applicable.

There is no parallel "process v2" doc. There is one document, and it always reflects the current rule.
