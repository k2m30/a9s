---
name: a9s-team-loop
description: The implementer / acceptance loop — shared protocol every team agent follows (task workspace, round log, statuses, escalation, shell rules)
---

# Team loop protocol

Three roles, one loop. The orchestrator (the main session) writes the spec, dispatches, rules, and lands; it writes no code and no tests beyond the fast path below.

```text
 spec ─► a9s-qa (red test per row, blind to the code) ─► a9s-dev (every test green → checked probes → the suite, itself) ─► a9s-acceptance (once) ─► ACCEPT ─► landing
                     │                                                    │                                                   │
                     └── OFF / BLOCKED / LOOP ──► a9s-facilitator (ruling) ◄──────────────────────────────────────────────────┘ REJECT on behaviour (QA adds the red test, dev round 2)
                                                                                                                              REJECT on wording (orchestrator fast path)
```

- **a9s-qa** writes the failing test for every spec row before the implementer exists in the worktree: from the spec, the given/when/then stories and the failure scenario, never from the implementation. Its red output is pasted in the log and its tests are the definition of done for the row. QA has no verify round: dev runs the suite itself (the user, 2026-09-08: "DEV CAN AND SHOULD RUN TEST HIMSELF!!!! NOT TO PASS IT BACK AND FORTH!!! QA writes tests!!! dev do the job until all tests pass").
- **a9s-dev** is the implementer: makes every QA test pass with the smallest correct change, adds its own edge-case probes (`checked:`), and runs the whole suite itself until green — one round per task in the common case, another only after an acceptance REJECT or a facilitator ruling. Dev never edits, inverts or deletes a QA test; when a QA test encodes a wrong expectation, dev logs it with evidence and the orchestrator has QA rewrite it.
- **a9s-facilitator** is called on `OFF`, `LOOP` or `BLOCKED`, and whenever a task passes round 3. On round 3 its first ruling is spec growth: every row added after dispatch that has no operator-observable witness is struck. It rules; it never codes. Every other deviation is ruled by the orchestrator as a bullet under the spec table, which is the only form a ruling takes.
- **a9s-acceptance** is the skeptical end user. It sees the finished tree, not the log, and accepts or rejects with evidence.

## Task workspace

Every dispatch names two absolute paths:

- `WORKTREE` — the git worktree to build in. Every `go` / `make` invocation targets it: `go -C $WORKTREE build ./...`, `make -C $WORKTREE lint`. Reference files by absolute path under it. Never touch any other checkout.
- `TASKDIR` — the task folder. It holds `spec.md` (the contract, written by the orchestrator or rewritten by the facilitator) and `log.md` (the append-only round log). Scratch output (captures, patches, gate logs) goes here too, never in `/tmp`.

Both paths are given in the dispatch prompt, and a prompt does not survive compaction. The orchestrator therefore also writes them to `$WORKTREE/.claude/task-context.md` per dispatch:

```text
WORKTREE=/private/tmp/a9s-wt/w7
TASKDIR=/private/tmp/.../tasks/w7
ROLE=a9s-dev
```

A `SessionStart` hook with the `compact` matcher prints that file back into context after a compaction. If you ever find yourself without the two paths, read it.

## Round log — `TASKDIR/log.md`

Append one entry per round, never edit earlier entries:

```markdown
## a9s-dev · round 2 · DONE
- TASKDIR=/private/tmp/.../tasks/w7
- WORKTREE=/private/tmp/a9s-wt/w7
- from: 4f1a9c2 (the task's base)
- changed: core/aws/sqs_issue_enrichment.go:41-88 (policy verdict via iampolicy.Evaluate), core/demo/fixtures/sqs.go:120 (PublicQueueName witness)
- red: tests/unit/sqs_public_policy_test.go (3 tests, red output pasted below before the fix)
- checked: empty policy → no finding; policy with Deny only → no finding; two statements, second public → finding once
- gates: go build OK · go vet OK · go test ./tests/unit -run 'SQS' OK (14 tests) · make lint OK
- deferred: core/aws/sqs.go:77 — Deleting queues still reach the posture pass — not this batch's row — owner: spec w5 gone-resource ruling
- simplified: ponytail-review on 4f1a9c2..HEAD — dropped the per-queue result struct (one map suffices), kept the cap guard (it is the read-only bound, not ceremony)
```

The `from:` line names the commit the round started from: the task's base, or the previous round's commit after an acceptance reject. The `TASKDIR=` and `WORKTREE=` lines name where the round ran.

The `deferred:` line is optional and stays in the log. It is for a defect an operator would see (a wrong screen, wrong data, a crash, a documented behaviour that does not hold) that this task's rows do not cover, one per line as `file:line — what the operator sees`. Nobody routes it: the orchestrator reads the log when the next task is cut and takes only what has a witness onto the backlog. Gate precision, hypothetical sites, comment wording and "could drift" are not deferred lines; they are nothing.

The `checked:` line is mandatory on every dev round entry. For each rule the round changed, it names the edge cases dev enumerated and exercised before the gates: the negated form, the empty input, the boundary value, the other partition or family, the second caller. These are throwaway probes (an ad-hoc table in a scratch file, a mutation in a disposable copy) deleted before the gates run; the line records the input and the observed result for each, and a probe that finds a defect becomes a pinned test before the fix. "The suite is green" is not a check: the suite covers what a test pins, and the cases a rewrite breaks are the ones nobody pinned yet (the user's question, 2026-09-07: "why dev cannot ensure he didn't break anything himself?"). A round entry without it is refused like one without gates.

The `simplified:` line is mandatory on every dev round entry. Before the round's gates, apply the ponytail ladder to the round's own diff (`from:` to the working tree, never the whole tree) and apply what survives the rules below; the `/ponytail-review` skill is not loadable in an agent session, so the ladder is applied by hand (does it need to exist, is it already here, does stdlib do it, can it be one line) and the orchestrator runs the skill itself on the integrated diff; the line names the range reviewed and what was cut, or what the review proposed and why it was refused. `none` is not an answer: a review that proposes nothing says "nothing proposed" with the range. Ponytail hunts over-engineering only; its proposals go through the same rules as any other change — a deletion is a hypothesis proven by grepping callers and running the gates, and no test, guard or `//nolint` is removed to look simpler.

Status values, exactly one per entry:

| Status | Meaning | Next |
|---|---|---|
| `DONE` | (dev) every spec row has its red-then-green test, `checked:` probes done, gates green | acceptance |
| `ACCEPT` / `REJECT` | (acceptance only) with numbered evidence | landing / a dev round on a behaviour reject, the orchestrator's fast path on a wording reject |
| `BLOCKED` | cannot proceed without something outside the worktree (credentials, a missing SDK field, a decision) | facilitator |
| `OFF` | "something's off": the spec contradicts the code, an existing test encodes a defect as intent, a fix would need a second truth source, the change is growing past its size | facilitator |
| `LOOP` | the same reject has bounced dev↔acceptance twice without converging | facilitator |

Read the whole log before starting a round. Round 1 is the task's first dev entry; round 2 exists only after a reject or a ruling.

**Report to the orchestrator only.** Your final output goes to the orchestrator; never start a round because another agent asked. The orchestrator hands the worktree from one agent to the next; a round started on a teammate's request is void. A message from the orchestrator that says "hold" means no edits and no runs in that worktree until the next hand-over.

## Non-negotiables (all roles)

- **Read before you climb.** Trace the real flow end to end (fetcher → Fields/RawStruct → findings function → catalog `FindingDef` → color classifier → demo fixture → fake → rendered surface) before choosing the smallest change. A small diff in the wrong place is a second bug.
- **One truth source.** Never compute the same fact twice. If two places would need the same condition, move it to one function and call it from both.
- **Fix the class, not the instance.** A defect found in one type is checked in its siblings before the round closes — every wave-1 fetcher and every wave-2 enricher of every type in the batch, listed with `file:line` in the round entry. A sibling in the same file through the same helper is fixed in the same round; any other sibling is a `deferred:` line, never a new spec row mid-task. The spec is frozen at dispatch: only a facilitator ruling changes a row, and an orchestrator bullet under the table only clarifies one (a phrase, a severity, an edge) and never adds one.
- **A stale pin never blocks a round.** When a spec row deletes behaviour an existing test asserts, dev inverts that test in the same round with a comment naming the row and lists it in the log; acceptance reads the inversion.
- **A criterion is met as written or logged as OFF.** Verifying a weaker check ("is a prefix of" for "equals") and signing off is a finding against the verifier, not a verification.
- **A sweep is its command and its full output, never a retyped list.** A row whose completion includes "every site" carries the exact grep (`/usr/bin/grep -rn ... core/ internal/ cmd/`) and its complete output pasted into the log, with the verdict written beside each line; a hand-enumerated list dropped two of fourteen sites on 2026-09-07 while claiming completeness twice.
- **A deletion is a hypothesis.** "Dead", "unreachable", "nothing calls it" is proven by grepping the symbol across every caller and running the gates on the deletion, not by reasoning from the path you were editing.
- **Every round is simplified before it is handed on.** See the `simplified:` line above.
- **No confidence filter.** Every finding is fixed or disproved with `file:line` evidence in the log. "Pre-existing", "minor", "probably fine" are not dispositions.
- **Nothing real.** No real AWS account IDs, profile names, bucket/secret/DNS names, or e-mails anywhere — synthetic `123456789012`, `example-readonly`, `acme-*`. `scripts/check-no-real-data.sh` is the gate.
- **Comments earn their place.** No comment that restates the code, narrates a change, or argues with a reviewer. Rationale, constraints, gotchas, external context only.
- **Shell discipline.** One command per Bash call. No `&&`, `;`, `|`, `$()` or backticks; write intermediates to `TASKDIR` files and read them in the next call. Use `/usr/bin/grep` for exhaustive enumeration (the proxied `grep` drops matches in bulk mode). Run `graphify query "<question>"` before grepping raw source when `graphify-out/graph.json` exists.
- **Report what you ran.** Gate results are pasted from captured output, never relayed: run each gate with its output and exit code captured to a file under `TASKDIR`, read the exit line back, and paste it into the log. A round that logs `DONE` names green `make test` and `make lint` captured after its last edit.
- **Never push, never open a PR, never merge, never tag.** Commits stay local in the worktree; the orchestrator integrates.

## One agent in a worktree at a time

Two agents editing one worktree overwrite each other, and `go vet ./...` compiles `tests/` — a half-written test file blinds vet for production code. So a task runs one agent at a time: dev, then acceptance on a detached checkout of dev's commit, then dev again only on a reject. The `from:` line of every log entry names the commit it started from.

A compile error in a file you did not touch is a real break in the tree you inherited, not a teammate mid-edit: log `BLOCKED` naming the file.

## Orchestrator checklist

The orchestrator dispatches, integrates, and writes nothing else — with one fast path. A change with no behaviour (a comment, a doc cell or sentence, a changelog line) or a one-line code change whose correctness is evident from reading and already covered by a gate is done by the orchestrator directly on `main`, gated by what covers it (`make mdlint`; `make test` and `make lint`), and committed with a clear message. Anything that adds a symbol, changes a rendered value, moves a fixture count or golden, or needs a witness goes through the loop. When in doubt, the loop. This is the user's rule (2026-09-05): a two-round cycle for an em-dash cell is waste.

- **A spec is at most eight rows at dispatch**, each a backlog row with a witness, and the header names the count. More than eight, or an estimate above size `L` in `docs/development-process.md`, splits into two tasks before QA starts. A dev `DONE` at round 4 or later without a facilitator ruling in the log is refused by the orchestrator.
- **One task in flight, several issues per task, grouped by area** (the user, 2026-09-07: one issue per task paid the landing overhead per issue; 2026-09-06: parallel tasks paid rebases on shared files). One dev per task, one acceptance agent for all tasks, a facilitator on `OFF`/`BLOCKED`/`LOOP` and at round 3; the dev is retired when its task lands.
- **No two live tasks touch the same file.** This is the user's rule (2026-09-06: "you must prevent same files clashes"), and globs are not enough to keep it: every task ends up in `CHANGELOG.md`, `docs/attention-signals.md`, the count pins and the coverage allowlists. Before dispatching, compute the new task's file set from the spec (the files it names plus the per-type files of every type it touches, including `docs/resources/<type>.md`, `core/demo/fixtures/<type>.go` and the tests that pin that type) and intersect it with `git diff --name-only origin/main...task/<x>` for every live branch. A non-empty intersection means the second task waits for the first to land, or is cut from the first's tip and lands after it in that order; it never runs beside it. A task that discovers mid-flight that it needs a file another live task holds logs `BLOCKED` naming the file, and the orchestrator serialises the two. `CHANGELOG.md` is the one file exempt from the rule: every task adds its lines under Unreleased and the landing keeps both sides. The other shared hubs (the demo count pins in `tests/integration/four_rules_demo_test.go`, `tests/unit/qa_demo_state_coverage_test.go`, `docs/attention-signals.md`) are a defect in the tree's shape, tracked as per-type pin files.
- **Worktrees are cut from committed HEAD.** Never with uncommitted work applied; a worktree that carries someone's WIP leaks it into commits.
- **One agent per worktree at a time.** Dispatch the next role only after the previous one's round entry is in `log.md` **and** `ListAgents` shows no running agent for that worktree — the log is necessary, not sufficient. Check the branch tip before dispatching and name it in the dispatch.
- **Write `$WORKTREE/.claude/task-context.md`** with `WORKTREE=`, `TASKDIR=` and `ROLE=` on every dispatch.
- **Rule in the spec, not by message.** A deviation dev names is ruled by the orchestrator as a bullet under the spec table before the next dispatch, with the file and the behaviour; only `OFF`, `BLOCKED`, `LOOP` and the round-3 threshold go to the facilitator. A ruling given only by message is not a ruling. The orchestrator never adds a row after dispatch: a found item without a witness stays a `deferred:` line in the log, and one with a witness is a backlog row for a later task.
- **Acceptance once per task**, after the last dev `DONE`, on a clean detached checkout of dev's commit, with the spec and its rulings as criteria, in parallel with the landing gate; nothing runs acceptance again after the fast-forward. A reject on wording (a changelog line, a doc sentence, a comment) is the orchestrator's fast path on the task branch; a reject on behaviour is a dev round.
- **After a usage-limit reset, resume by name.** The agent keeps its transcript; `git status` in its worktree and the log say where it stopped. Never re-dispatch a fresh agent into a tree with half a round in it.
- **A round's gates match the round's diff, not the task.** The per-round contract is `make test` and `make lint`, plus `make mdlint` when a `.md` moved and `make check-catalogen` when a doc or catalog literal moved. `make integration`, `make snapshot`, the smokes and `make test-race` run once per task, in `make ready-to-push` at landing, and nowhere else: acceptance proves a rendered claim with the scoped test and a demo capture of that row, not the integration suite. A dispatch never lists the long gates (the user's rule, 2026-09-07: "reevaluate running long integration test for every one liner", then "qa run took 13m 40s" for a verify round I had given the long gates to).

### Landing checklist

1. The accepted commit is the branch tip (`git log <accepted>..<branch>` is empty); triage anything above it first.
2. `ListAgents` shows no running agent for the worktree.
3. Cherry-pick the branch onto a landing branch cut from `main` in a clean worktree; resolve conflicts there; if the branch was cut with WIP present, reverse-apply every WIP hunk and diff the result against the WIP file list.
4. On the landing tree: `go vet ./...`, `make test`, `make lint` (retry once on the cross-worktree lock), `make check-catalogen`, `make mdlint`, `make security`.
5. Fast-forward `main`; if the primary checkout has uncommitted work on a file the branch touches, set that file aside (`git diff` to a file, `git checkout --`), fast-forward, restore with `git apply --3way`, unstage, and confirm the primary tree still builds.
6. Delete the batch branch and worktree, and stop the task's QA, dev and facilitator agents (`TaskStop` by name); only the shared acceptance agent outlives a task.
7. `graphify update .` on the fast-forwarded main tip, so the graph every dispatch is told to query describes the tree the next task starts from.

## Ending a round

The last thing a round does is append its log entry. The agent's final output is a copy of that entry — nothing else, and it is not also sent as a message: the harness delivers the final output to the orchestrator when the agent goes idle, so a `SendMessage` of the same report arrives twice (the user asked why every agent reports twice, 2026-09-07). A message mid-round is for a question only the orchestrator can answer, never for the report.
