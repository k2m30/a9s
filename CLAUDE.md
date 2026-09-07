# a9s Development Guidelines

Your work will be review by Codex.

## Process — single source of truth

**Read [`docs/development-process.md`](docs/development-process.md) first.** It defines the lifecycle stages, Definition of Ready, Definition of Done, and the canonical pre-push and pre-release gates. If a rule below conflicts with that document, the document wins until updated.

Topic rules load from `.claude/rules/`: project layout, commands, toolchain, agent file access, docs sync.

Quick reference:

- Pre-push gate (Stage 6): `make ready-to-push`
- Pre-release gate (Stage 7): `make ready-to-release`

## GitHub

- Repository: `k2m30/a9s` — always use this owner/repo for GitHub API calls, issues, and PRs

## Active Technologies

- Go 1.26+, Bubble Tea v2.0.6, Lipgloss v2.0.3, Bubbles v2.1.0 (all under `charm.land/*/v2`), AWS SDK Go v2 (one service module per supported AWS service), yaml.v3, clipboard
- YAML config on disk (`~/.a9s/config.yaml`, `~/.a9s/themes/*.yaml`, `~/.a9s/views/`); YAML cache on disk, per-type layout: `~/.a9s/cache/<profile>--<region>/<shortName>.yaml` (TypeFile schema v2, no TTL)
- Session-scoped in-memory state owned by `core/session.Session` (RowStore, capability stores, generation counters; cleared on profile/region `Rotate()`)
- In-process demo fixture store (per resource type, typed fakes in `core/demo/fixtures/` + `fakes/`, loaded at startup)

## Bug Protocol

Applies to every defect — one you found, one a review reported, one a test caught.

1. **Ask whether the bug is architectural BEFORE writing the red test or the fix.** If two places compute the same fact, if a comment asserts an invariant nothing enforces, or if the same mistake is possible at a site nobody has visited yet, the instance fix is the wrong fix. Fix the shape.
2. **Decide autonomously.** Architectural changes do not need approval. Estimate, choose, execute, and report what you chose and why.
3. **Order fixes so no work gets rewritten.** If an architectural change will rewrite the lines a smaller fix touches, do the architectural one first or fold them together. Never fix the same lines twice.
4. **No confidence filter.** Every finding is either fixed or explicitly disproved with `file:line` evidence. "Probably fine", "pre-existing", and "minor" are not dispositions. A stale or misattributed finding is *disproved*, not dropped.
5. **Fix the class, not the report.** A reviewer names the symptom it happened to see. Before closing, check whether the same defect exists in sibling fields, sibling call sites, or the other lane.
6. **A test that encodes a defect as intent is worse than no test** — it makes the fix look like a regression. When a test must be inverted, say so in its comment so the next reader does not "restore" it.
7. **Re-derive numbers, never relay them.** Counts from a grep, a subagent, or a prior report are unverified until you reproduce them. Report only what you have run.

## External Review Protocol

**Nothing merges to `main` without an external pass resolved.** Local gates prove the code runs; they do not prove it is right. Three independent reviewers are available and they find different classes of defect:

```bash
codex exec --skip-git-repo-check "<review prompt naming the range and the production files>"
coderabbit review --plain --type committed --base-commit <base>
```

Plus `/ponytail-review` for over-engineering (delete/simplify only — it does not hunt correctness).

Rules:

- **Batch. Never per fix.** Codex is expensive: one pass per phase boundary or pre-merge.
- **Review the committed range**, not the working tree — `--base-commit <base>` for CodeRabbit, an explicit range for Codex.
- **A reviewer's suggested patch is a proposal, not verified code.** Read every snippet against the actual file before applying it; patches routinely reference helpers that do not exist in that file's package. The finding can be correct while the patch does not compile.
- **Point reviewers at production code.** A large mechanical test migration will drown the signal otherwise.
- Never tag a release without CodeRabbit and Codex resolved on the range.

On PRs: `@coderabbitai ignore` where no further review is wanted; `[skip ci]` for trivial follow-ups; reviews trigger per-push, so batch small fixes into one push.

## Gate Results Are Samples, Not Verdicts

- `make test-race` runs `-shuffle=on` with a **fresh random seed each invocation**. One green run is one ordering. When something order-dependent is suspected, sweep 10+ seeds and report the rate.
- Comparing "the same seed" across two trees is **not** a controlled experiment — shuffle permutes the whole test list, so a different test count yields a different order. Compare the same tests in the same order.
- Read the gate's exit code from captured output (`make ... > /tmp/x.txt; echo "EXIT=$?" >> /tmp/x.txt`), not from a wrapper's status. Never `tail -5` a gate log; the diagnostic is usually above the fold.

## Architecture Principles

> **Full architecture guide**: [`docs/architecture.md`](docs/architecture.md) — covers all concepts, patterns, caching layers, key handling, test philosophy, and design decisions. Read it first when onboarding.
>
> ⚠️ **Related-resource panel is governed by [`docs/related-resources.md`](docs/related-resources.md) — SINGLE SOURCE OF TRUTH, DO NOT EDIT AD-HOC.** Every `Related` entry on a catalog literal must match that contract. Adding/removing pivots requires an AWS API field citation or a documented DevOps workflow reason in the same PR that touches the registration.

- **Read-only by design** — a9s never makes write calls to AWS
- **Bubble Tea v2** — all I/O in `tea.Cmd` closures, views are pure functions
- **Message-driven** — views communicate via typed messages, never import each other
- **Single source of truth** — key bindings in `keys/keys.go`, types in `types.go`, styles in `styles/`, **related-panel contract in `docs/related-resources.md`**

## Skills and Subagents — in-session tooling

> **The two tables below describe Claude Code skills and subagents.** They are tools invoked from within the Claude Code session. Work runs as the team loop, whose round order is defined once in `.claude/skills/a9s-team-loop/SKILL.md` — read it there. The main session orchestrates only. One implementer (`a9s-dev`) owns a task end to end, red test before each fix; acceptance is the independent check. The separate QA role was retired on 2026-09-07: it re-ran the suite the implementer had just run and handed the same findings back.

## Skills

| Skill | Scope | Usage |
|-------|-------|-------|
| `a9s-common` | All work | Shell rules, package access rules, build/test commands |
| `a9s-bt-v2` | TUI-touching work | Bubble Tea v2 / Lipgloss v2 / Bubbles v2 API patterns |
| `a9s-add-attention-column` | impl + tests | Add a list-view attention column (Tier A/B decision tree) |
| `a9s-implement-issue` | Orchestrator | Interview → `spec.md` → one approval → autonomous through the team loop to acceptance |
| `a9s-resource-spec` | Main session | Generate `docs/resources/<shortName>.md` implementation-blind from the four golden docs |
| `a9s-implement-resource` | Main session (orchestrator) | Implement resource from its spec: TBDs → impl-plan → fixtures → dev/QA handoff |
| `a9s-create-demo-fixture` | Fixtures | Build the single-source fixture file at `core/demo/fixtures/<shortName>.go` — graph-connected, demo + tests share it |

## Agents

| Agent | Role | Writes to | Rejects without |
|-------|------|-----------|-----------------|
| `a9s-dev` | The implementer in the team loop — red test per spec row, then the fix, then its own edge-case probes; production code, tests, fixtures, fakes, catalog, generated docs | `core/`, `internal/`, `cmd/`, `.a9s/`, `scripts/`, `tests/`, docs it regenerates | `WORKTREE` + `TASKDIR/spec.md` |
| `a9s-facilitator` | Rules when dev logs `OFF`, `LOOP`, `BLOCKED`, or a task stalls — rewrites the spec or names the fix; no code | `TASKDIR/spec.md`, `TASKDIR/log.md` | A stalled loop |
| `a9s-acceptance` | Skeptical end user — final acceptance on rendered surfaces, docs, gates; blind to the log until verdict | `TASKDIR/` only | Criteria + integrated worktree |
| `a9s-qa-stories` | Given/when/then stories from design spec (no source code) | Nothing (read-only) | N/A |
| `a9s-devops` | AWS practitioner — resource priorities, feature advice | All | N/A |
| `a9s-consistency-checker` | Verifies consistency across code, tests, README, website, config | Nothing (read-only) | N/A |
| `tui-designer` | TUI wireframes, color schemes, preview mockups | Design artifacts | N/A |

## Rules

- ALWAYS rebuild binary (`make build`) after ANY code change — version is resolved at build time via `core/buildinfo`
- Resolve ambiguity from the code and the spec; ask only when different readings lead to materially different work
- TDD is non-negotiable: `a9s-dev` writes the failing test for a spec row, pastes its red output into the log, then makes it pass. One agent in a worktree at a time.
- ALWAYS test ALL resource types (S3, EC2, RDS, Redis, DocumentDB, EKS, Secrets Manager, VPC, SG, Node Groups, etc), not just one
- NEVER delete code, tests, or helpers just to make a linter happy. Understand WHY the code exists first. If it's genuinely dead, remove it. If it serves a purpose (scaffolding, crash-verification tests), use a targeted `//nolint` with a reason comment. If a linter rule produces widespread false positives, fix the rule in `.golangci.yml`.
- NEVER make multiple push-and-check cycles. Get it right locally, push once.
- NEVER commit real environment identifiers — real AWS account IDs, profile names, secret/bucket/DNS names, personal emails. Use synthetic values (`123456789012`, `example-readonly`) or `<placeholders>`. Enforced by `scripts/check-no-real-data.sh`: a generic account-ID-in-ARN heuristic (committed — a pattern, no real value stored) plus the exact terms from the local, git-ignored `.githooks/sensitive_patterns.txt` (never committed — not even hashed; a 12-digit ID or short name is brute-forced from a hash in seconds). Term enforcement is on NEW content only. Modes: tree (`make ready-to-push`), `--staged` (`.githooks/pre-commit`), `--diff` (`.githooks/pre-push`, aborts the push before it reaches the remote), `--audit` (hunt existing leaks). CI is deliberately NOT used — it runs after the push, and a push to a public repo exposes the data immediately. Run `make install-hooks` once per clone. New real term → add it to your local `.githooks/sensitive_patterns.txt`.
- BEFORE any push, the canonical gate is **`make ready-to-push`** — see [`docs/development-process.md`](docs/development-process.md) §"Stage 6 — Pre-push Validation" for the gate contents and the `core/aws/` live-integration sub-rule. Review the diff first (Stage 5): `a9s-consistency-checker` for cross-file drift, direct BT v2 / security / coverage review, and CodeRabbit / Codex as external passes.
- BEFORE any release, the canonical gate is **`make ready-to-release`** — see [`docs/development-process.md`](docs/development-process.md) §"Stage 7 — Merge & Release" for the manual checklist (`CHANGELOG.md`, `releases/vX.Y.Z.md`, `docs/architecture.md` alignment, busywork audit on tests added/modified in the release).
- **Exception**: Docs-only changes (`*.md`, `docs/`, `website/`, `specs/`, `.claude/`, `LICENSE`) skip `ready-to-push`; `make mdlint` is required.

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

Rules:
- For codebase questions, first run `graphify query "<question>"` when graphify-out/graph.json exists. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).
