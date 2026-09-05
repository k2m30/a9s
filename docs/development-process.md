# a9s Development Process

This is the **single source of truth** for how work flows through a9s — from request to release. If a rule here conflicts with an ad-hoc instruction, this document wins until it is updated.

It is short on purpose. Process rot starts the moment a doc becomes too long to re-read in five minutes.

## Goals

1. **Quality is gated, not hoped for.** Each stage has explicit entry and exit criteria. Nothing advances without its gate.
2. **TDD is non-negotiable.** Tests exist before implementation merges, full stop.
3. **Failures fail loudly and locally.** CI is verification, not a debugger. The pre-push and pre-release gate suites catch regressions before they reach `main`.
4. **The process measures itself.** Track a small set of release metrics and run a periodic retro to keep the process alive.

## Subagents and skills are tools, not owners

Work is done inside a Claude Code session. The developer may delegate scoped sub-tasks to **subagents** (`.claude/agents/*.md`) or invoke **skills** as tools. They are tools, not approvers — they hold no authority, sign off on nothing, and never own a stage. The developer owns the work end to end.

Surviving subagents and their write boundaries:

| Subagent | Writes to | Use for |
|---|---|---|
| `a9s-dev` | `core/`, `internal/`, `cmd/`, `.a9s/`, `scripts/`, generated docs | Go production code, fixtures, catalog, doc regeneration — **no tests** |
| `a9s-qa` | `tests/` | Red tests first, adversarial verify, findings / sign-off — **no production code** |
| `a9s-facilitator` | `TASKDIR/spec.md`, `TASKDIR/log.md` | Binding ruling when dev/qa log `OFF` / `LOOP` / `BLOCKED` or pass round 3 |
| `a9s-acceptance` | `TASKDIR/` | Skeptical end-user acceptance on rendered surfaces, docs and gates |
| `a9s-qa-stories` | Nothing (read-only) | Given/when/then stories from the design spec, zero source knowledge |
| `a9s-consistency-checker` | Nothing (read-only) | Cross-file drift: code ↔ docs ↔ website ↔ config |
| `a9s-fixtures` | `core/demo/` | Test/demo fixtures from real AWS via the AWS MCP tool |
| `a9s-devops` | All | AWS-practitioner consult: resource priorities, real-world workflows |
| `tui-designer` | Design artifacts | TUI wireframes, color schemes, preview mockups |

The dev/QA write split is the TDD guardrail: dev cannot edit tests, QA cannot edit production code. Keep it. The loop itself — task workspace, round log, statuses, escalation to the facilitator, final acceptance — is defined once in `.claude/skills/a9s-team-loop/SKILL.md`.

## Definitions

### Definition of Ready (DoR) — required before implementation begins

A unit of work is Ready when **all** are true:

- One-sentence problem statement.
- Acceptance criteria written (or a pointer to a spec doc / PR spec).
- Sized: `XS` (≤30 LOC, single file) · `S` (≤200 LOC, ≤3 files) · `M` (≤600 LOC) · `L` (≤1500 LOC) · `XL` (split first — see Splitting below).
- Linked to a goal, refactor phase, or release milestone if applicable.

If any of those is missing, do not start fuzzy — resolve it first.

### Definition of Done (DoD) — required before work is considered complete

- Acceptance criteria demonstrably met (test, screenshot, or live run).
- Stage 6 (`make ready-to-push`) gates green locally.
- Docs sync respected: README is regenerated when `docs/shared/` changes; `CHANGELOG.md` updated for any user-visible change; `docs/architecture.md` aligned for cross-cutting changes.
- Single-source-of-truth invariants intact (no dual-authoring, no permanent dual API surface).
- Conventional commit message on every commit.

### Splitting

`XL` (>1500 LOC, or touches >3 packages, or no clear single concern) is **always** split before work starts. Size by one mechanical concern per PR, with stabilization checkpoints between phases.

## Lifecycle — stages

Every unit of work goes through these stages. Stages 2, 4, 6.5 may be **skipped** for trivial bug fixes (`XS`, single file, no behavior change visible to users). Stages 3, 5, 6 never skip — except in the Trivial-docs fast-path below.

```text
1. Intake → 2. Spec → 3. Tests → 4. Impl → 5. Review → 6. Validate → 6.5 Post-merge AWS → 7. Release → 8. Retro
```

### Trivial-docs fast-path (`XS`-docs)

**When the lane applies — all of the following must hold:**

1. The change touches **only** `*.md`, `docs/`, `website/`, `specs/`, `.claude/`, `LICENSE`, or `CHANGELOG.md`. No Go source, tests, fixtures, Makefile, `.github/workflows/`, `core/`, `internal/`, or `cmd/`.
2. Size is `XS`: ≤ 30 LOC added/changed across ≤ 2 files.
3. The change is one of: typo fix, link fix, formatting/style fix, or clarification.
4. The change does **not** reverse an existing rule. Reversing a rule routes through normal Stages 1–5.

**Lane procedure:** author the diff directly (no spec/tests/review ceremony), branch from `main`, single conventional commit (`docs(scope): ...`), run `make mdlint` (the only gate), open a PR prefixed `[trivial-docs]`, merge after CI is green.

### Stage 1 — Intake

- **Trigger**: a request, bug report, or ready backlog item.
- **Action**: triage type (bug · feature · refactor · ops · docs), set priority, set size, draft acceptance criteria.
- **Exit**: the work meets DoR.

### Stage 2 — Spec & Design

- **Trigger**: DoR met and size ≥ `M`. Skipped for `XS`/`S` bug fixes.
- **Tools**: `a9s-resource-spec` skill (writes `docs/resources/<short>.md`); `a9s-devops` for AWS-practitioner priority sanity.
- **Action**: produce a spec doc. Resources use `a9s-resource-spec`. Refactor PRs reference the per-PR spec in `docs/historical/refactor/`. Features write to `specs/<n>-<feature>.md`.
- **Exit**: spec doc committed to the feature branch. The spec is the contract; existing implementation is disposable.
- **Anti-pattern**: skipping the spec for "obvious" features. If it is so obvious, the spec is one paragraph — write it anyway.

### Stage 3 — Tests

- **Trigger**: spec published (size ≥ M) or scoped task (`XS`/`S`).
- **Tools**: `a9s-qa-stories` (given/when/then, zero source knowledge), `a9s-qa` (failing Go tests).
- **Action**: translate spec to stories, then to failing Go tests. Tests land on the feature branch and **fail as expected**. The QA subagent rejects tasks without an exact file scope.
- **Exit**: failing tests committed.
- **Anti-pattern**: "test along with implementation." That is not TDD. Tests precede implementation in time and in commit history.

### Stage 4 — Implementation

- **Trigger**: Stage 3 tests landed and red.
- **Tools**: `a9s-coder` (Go production code), `a9s-fixtures` (demo/test fixtures via `a9s-create-demo-fixture`).
- **Action**: make the failing tests pass. Touch only files in scope. Rebuild the binary (`make build`) after every change.
- **Exit**: tests pass; `make build && make test && make lint && make gofix && make security` green locally.
- **Anti-pattern**: writing new tests in the coder pass instead of routing back to QA. Editing files outside scope. Skipping `make gofix`.

### Stage 5 — Review

Review the diff before it merges. Use the available lenses as tools:

- `a9s-consistency-checker` — cross-file drift (code ↔ docs ↔ website ↔ config).
- Direct review for Bubble Tea v2 / Lipgloss v2 correctness (see the `a9s-bt-v2` skill), security (read-only AWS invariant, no secrets), and test-coverage gaps.
- `arch-review` skill — architecture checklist for size ≥ `M`.
- CodeRabbit / Codex (external) — one pass per push; batch fixes into one push. Use `@coderabbitai ignore` when no re-review is needed.

- **Exit**: review lenses clean; CodeRabbit either resolved or `@coderabbitai ignore`d with a reason.
- **Anti-pattern**: multiple push-fix-push cycles to chase a reviewer. Get it right locally; push once.

### Stage 6 — Pre-push Validation (single command)

```bash
make ready-to-push
```

This target is the canonical gate. It MUST pass locally with zero edits before any push. It runs, in Makefile order:

1. `make verify-hooks` — aborts unless `git config core.hooksPath` is `.githooks`. The sensitive-term half of `check-no-real-data` lives in the git hooks; an unhooked clone would silently lose it.
2. `make check-no-real-data` — blocks real AWS/environment identifiers (account IDs in ARNs, previously-leaked terms) from tracked files; tree mode also term-scans NEW content against the merge-base with `origin/main`.
3. `make test-race` — unit tests with race detector and `-shuffle=on` (CI shuffles test order; a local gate that runs in declaration order cannot catch order-dependent leaks, per the v3.54.0 macOS TempDir incident).
4. `make lint` — golangci-lint.
5. `make security` — govulncheck.
6. `make gofix` — `//go:fix inline` directives applied.
7. `make verify-readonly` — read-only invariant via `cmd/readonlycheck`: an AST-based scan that flags write-verb method call nodes in `core/aws/` and `core/runtime/`, so comments, strings, and formatting are structurally irrelevant; SDK-typed receivers are always flagged, and exemptions are exact method names on non-SDK receivers.
8. `make verify-zero-init` — zero `init()` bodies in `core/aws/` and `core/catalog/`: registration lives in catalog literals, not package `init()` (design record: `docs/historical/refactor/landed/AS-795-init-cycle-break.md`).
9. `make verify-renderer-free` — runs `go list -deps` over `core/` and fails on any `charm.land` or `internal/` dependency (the renderer-agnostic boundary, architecture invariant 1).
10. `make check-readme` — README in sync with `docs/shared/`.
11. `make check-catalogen` — the generated blocks in `docs/attention-signals.md`, `docs/related-resources.md`, and `docs/resources/*.md` are in sync with the catalog declarations (runs `cmd/catalogen` and fails if regeneration changes anything).
12. `make snapshot` — golden-file render checks.
13. `make mdlint` — markdown lint across `docs/`, `CLAUDE.md`, `CONTRIBUTING.md`, `CHANGELOG.md`.
14. `make smoke` — tmux-driven demo smoke over the compiled binary (`scripts/smoke-demo.sh`): rendered menu counts, humanized statuses, per-row issue causes, the reference bucket's related panel. Requires tmux. When demo fixtures legitimately change, update the script's assertions in the same PR.
15. `make smoke-related` — tmux-driven demo smoke dedicated to the RELATED panel (`scripts/smoke-related-demo.sh`): exact fixture witness badges, the zero-count-row cursor skip, a count-1 drill landing on the target detail, a circular drill re-showing cached counts, and the ec2 IAM Role pivot. Requires tmux.
16. `make smoke-costs` — tmux-driven demo smoke of the Cost Explorer (`scripts/smoke-costs-demo.sh`): month→week→day zoom, metric cycle, account pivot, the planted growth-story drill to usage types, the 14-day resource boundary message. Fixtures anchor to the current month, so assertions stay evergreen. Requires tmux.
17. `make smoke-enrichers` — tmux-driven demo smoke dedicated to on-demand detail enrichment (`scripts/smoke-enrichers-demo.sh`): every catalog-registered detail enricher's fetched payload asserted on the rendered surface — SFN ASL definition, CFN template body, Lambda deploy-state fields, EC2 decoded user data, SNS topic attributes, S3 policy/CORS/lifecycle documents, IAM policy documents, and the two child-view enrichers (role-policy documents, transfer agreement profile resolution). Session-cache hit behavior is unit-tested, deliberately not asserted here. Requires tmux.

This enumeration is pinned against the Makefile's `ready-to-push` dependency list by a unit test (`tests/unit/docs_gate_sync_test.go`) — when the target changes, that test fails until this list is updated to match.

For changes that touch `core/aws/` real-account behavior, additionally run the live integration test against a real AWS profile (this is also the entry to Stage 6.5):

```bash
A9S_CT_PROFILE=<profile> go test -tags integration ./tests/integration/ \
  -run TestFullRelatedViewValidation -count=1 -v -timeout 1800s
```

The 1800s budget is sized from practice: the full per-type walk takes
~19 minutes against a moderately populated account, so a 600s budget
panics mid-walk with zero assertion failures.

- **Exit**: all green locally. CI is verification, not debugging.
- **Anti-pattern**: "I'll let CI tell me if it's broken." That is a budget leak and an etiquette violation against reviewers.

#### Docs-only exception

For pure docs changes (`*.md`, `docs/`, `website/`, `specs/`, `.claude/`, `LICENSE`), `make ready-to-push` is **not** required. `make mdlint` is.

### Stage 6.5 — Post-merge real-AWS validation

- **Trigger**: a merge to `main` touches `core/aws/`, fetchers, child views, related-resource pivots, or fixtures. Skipped for pure-docs and pure-tooling changes. For a multi-PR refactor program, also run a batch pass at each phase boundary (≥ 3 PRs merged since the last real-AWS sign-off), since mocks cannot fully cover large refactor surfaces.
- **Tools**: the integration test binaries under `tests/integration/`; `make smoke-live PROFILE=<profile>-readonly [REGION=...]` — a data-independent TUI sweep over every non-empty resource type in the account (raw-enum and vacuous-status forbids, related-panel settle checks); `make smoke-related-live PROFILE=<profile>-readonly [REGION=...]` — the RELATED-panel-dedicated companion (settled counted badge, drill-and-return, a surviving "(?)" row staying actionable). Live smoke runs only on `*readonly` profiles by construction.
- **Action**: run the integration suite against a real AWS profile (`A9S_CT_PROFILE=<profile>`), run `make smoke-live` and `make smoke-related-live`, exercise the changed surface (list → detail → child view → related view) across ≥ 4 distinct resource types for a phase-boundary pass, and capture pass/fail per scenario.
- **Exit**: all real-AWS scenarios green, or a scoped regression note with a follow-up fix.
- **Anti-pattern**: treating Stage 6 (`make ready-to-push`) as sufficient for changes that depend on real AWS API behavior.

### Stage 7 — Merge & Release

- **Merge to `main`**: only with green CI plus Stage 5 review clean.
- **Release path** (when cutting a tagged version):
  1. `make ready-to-release PROFILE=<readonly-profile> REGION=<region>` — ADDITIVE on top of Stage 6, it does NOT re-run the push gate: prerequisite is a green `make ready-to-push` on this same tree, and this target adds only integration + the live read-only smokes `smoke-live` and `smoke-related-live`. All green. Requires read-only AWS credentials and tmux; the live smokes require an explicit `PROFILE`/`REGION` (no default) and refuse any profile whose name is not `*readonly*`.
  2. `CHANGELOG.md` updated with a Keep-a-Changelog entry; `releases/vX.Y.Z.md` written.
  3. `docs/architecture.md` aligned with the codebase. Outdated architecture docs are a release blocker.
  4. **Busywork audit**: every test added or modified in the release is reviewed and deleted if it is a tautology, a mock asserting its own input, a struct-shape pin instead of a behavior pin, or duplicate coverage. Coverage earned by busywork is a liability.
  5. The real-AWS pass is now a mandatory automated gate, not a manual step: the live read-only smokes run as part of `ready-to-release` (step 1) and must be green against a real account before the tag is cut.
  6. Tag pushed; CI (GoReleaser) publishes; release notes go live.
- **Exit**: tag exists, artifacts published, `releases/vX.Y.Z.md` committed.

> Releases are cut from tags by CI (GoReleaser). Never create a GitHub release manually.

### Stage 8 — Retro (periodic)

After each release (or on a regular cadence), write a short retro covering: a release-metrics snapshot, one thing the process did well, and one thing it slipped on plus the **specific** rule change that prevents it next time. Update this document in the same change — do not "remember to fix it later." A retro that produces no rule change is a smell.

## Branching, Commits, PRs

- **Trunk-based**: `main` is always releasable. Feature branches are short-lived (≤ 1 week) and named `<area>/<short-slug>`.
- **One PR per concern**. Refactor PRs follow the per-PR spec in `docs/historical/refactor/<phase>.md`.
- **Conventional Commits**: `feat:`, `fix:`, `refactor:`, `docs:`, `chore:`, `test:`, `ci:`.
- **`@coderabbitai ignore`** on PRs that don't need a re-review; `[skip ci]` on trivial follow-ups.
- **Never `--no-verify`, never `--no-gpg-sign`**. Hook failures are diagnosed, not bypassed.
- **Never amend a published commit**. Always create a new commit.

## Bug Triage

- **P0 (critical)**: data corruption, crash on launch, security regression. Drop everything. Hotfix branch off `main`.
- **P1 (high)**: broken core workflow, real AWS profile fails. Schedule into the current sprint.
- **P2 (medium)**: edge case, demo-only, cosmetic. Schedule against the next release.
- **P3 (low)**: nice-to-have. Backlog only; never auto-promoted.

A bug fix follows the same lifecycle. The cheap path for `XS` bug fixes is Stages 1 → 3 → 4 → 5 → 6 → 7 (skip 2 and 6.5 if the change does not touch real-AWS surface). "I just changed one line" is how regressions hide.

## Incident & Rollback

If a regression lands in `main`:

1. Record an incident note: timestamp, symptoms, suspected commit.
2. If user-visible and not safely forward-fixable in <60 minutes: revert the offending commit. Reverts are commits; they go through Stage 5 (lightweight) and Stage 6.
3. After mitigation: a written post-incident note covering root cause, blast radius, what gate failed to catch it, and the gate change that prevents recurrence. The gate change lands the same week.

A regression that the process did not catch is a process bug, not a coder bug.

## Metrics

Tracked per release in the release notes file:

- **Deployment frequency**: tagged releases per week.
- **Lead time for changes**: median commit-to-merge time on the release's PRs.
- **Change-failure rate**: PRs in the release that required a follow-up fix.
- **MTTR**: time from incident open to mitigation merged for any P0/P1 in the release.

Any value that drifts the wrong way for two consecutive periods is a retro topic.

## Anti-patterns (call them out, fix them)

- **Naming a subagent as an owner or approver.** Subagents are tools; the developer owns the work.
- **Routing implementation work to a consult-only tool.** `a9s-devops` is advisory; PRs, branches, fixtures, tests, doc edits are real work, not consults.
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
