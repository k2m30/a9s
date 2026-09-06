---
name: a9s-team-loop
description: The dev / qa / facilitator / acceptance loop — shared protocol every team agent follows (task workspace, round log, statuses, escalation, shell rules)
---

# Team loop protocol

Four roles, one loop. The orchestrator (the main session) dispatches; it writes no code and no tests.

```text
                        ┌──────────── findings ────────────┐
                        ▼                                  │
 spec ─► a9s-dev (round 0: stubs) ─► a9s-qa (red tests) ─► a9s-dev (impl) ─► a9s-qa (verify) ─► QA SIGN-OFF ─► a9s-acceptance ─► ACCEPT
                                                             │                     │                                     │
                                                             └── OFF / LOOP / BLOCKED ──► a9s-facilitator (ruling) ◄─────┘ REJECT
```

- **a9s-qa** writes the failing tests from the spec, then verifies every dev round adversarially.
- **a9s-dev** lands the stubs QA writes against, then makes the tests pass with the smallest correct change and keeps every gate green.
- **a9s-facilitator** is called only on `OFF`, `LOOP`, `BLOCKED`, or when a task passes round 3. It rules; it never codes.
- **a9s-acceptance** is the skeptical end user. It sees the finished tree, not the log, and accepts or rejects with evidence.

## Task workspace

Every dispatch names two absolute paths:

- `WORKTREE` — the git worktree to build in. Every `go` / `make` invocation targets it: `go -C $WORKTREE build ./...`, `make -C $WORKTREE lint`. Reference files by absolute path under it. Never touch any other checkout.
- `TASKDIR` — the task folder. It holds `spec.md` (the contract, written by the orchestrator or rewritten by the facilitator) and `log.md` (the append-only round log). Scratch output (`gate.txt`, captures, patches) goes here too, never in `/tmp`.

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
- from: 4f1a9c2 (a9s-qa round 1 red tests)
- changed: core/aws/sqs_issue_enrichment.go:41-88 (policy verdict via iampolicy.Evaluate), core/demo/fixtures/sqs.go:120 (PublicQueueName witness)
- gates: go build OK · go vet OK · go test ./tests/unit -run 'SQS' OK (14 tests) · make lint OK
- deferred: core/aws/sqs.go:77 — Deleting queues still reach the posture pass — not this batch's row — owner: spec w5 gone-resource ruling
- simplified: ponytail-review on 4f1a9c2..HEAD — dropped the per-queue result struct (one map suffices), kept the cap guard (it is the read-only bound, not ceremony)
```

The `from:` line names the commit the round started from — the other role's last commit. It is how the next agent knows the tree it inherits. The `TASKDIR=` and `WORKTREE=` lines are what the stop hook reads to find `gate.txt`; without them the hook refuses the round.

The `deferred:` line is mandatory: `none`, or one item per line as `file:line — what — why left — owner`. Anything you noticed and did not fix goes here — a duplicated fact, a raw value, a gate that cannot see a surface, a fixture that lies — whether or not it is "this batch's". An item you noticed and did not write is a defect of the round. "Pre-existing", "out of batch" and "add when" are routings for the orchestrator, never dispositions: every deferred line is moved to the spec of the batch that owns the type, or to the backlog with an owner, before the next dispatch into the worktree.

The `simplified:` line is mandatory on every dev and QA round entry. Before the round's gates, apply the ponytail ladder to the round's own diff (`from:` to the working tree, never the whole tree) and apply what survives the rules below; the `/ponytail-review` skill is not loadable in an agent session, so the ladder is applied by hand (does it need to exist, is it already here, does stdlib do it, can it be one line) and the orchestrator runs the skill itself on the integrated diff; the line names the range reviewed and what was cut, or what the review proposed and why it was refused. `none` is not an answer: a review that proposes nothing says "nothing proposed" with the range. Ponytail hunts over-engineering only; its proposals go through the same rules as any other change — a deletion is a hypothesis proven by grepping callers and running the gates, and no test, guard or `//nolint` is removed to look simpler.

Status values, exactly one per entry:

| Status | Meaning | Next |
|---|---|---|
| `DONE` | this round's work is complete and verified as stated | the other role's turn |
| `FINDINGS` | (qa only) numbered defects with file:line and a failing test each | dev round |
| `SIGN-OFF` | (qa only) every spec row has a passing behavioural test, gates green; valid only when its `from:` is the hash the orchestrator dispatched | acceptance |
| `ACCEPT` / `REJECT` | (acceptance only) with numbered evidence | done / dev round with facilitator |
| `BLOCKED` | cannot proceed without something outside the worktree (credentials, a missing SDK field, a decision) | facilitator |
| `OFF` | "something's off": the spec contradicts the code, the test encodes a defect as intent, a fix would need a second truth source, the change is growing past its size | facilitator |
| `LOOP` | the same finding has bounced dev↔qa twice without converging | facilitator |

Read the whole log before starting a round. Rounds are numbered per role; round 1 is the first entry of that role.

**Report to the orchestrator only.** Your final message goes to the orchestrator; never message the other role, and never start a round because the other role asked. The orchestrator hands the worktree from one role to the next; a round started on a teammate's request is void, and a sign-off that answers a teammate's message instead of the orchestrator's dispatch does not count. A message from the orchestrator that says "hold" means no edits and no runs in that worktree, on anyone's request, until the next hand-over.

## Non-negotiables (all roles)

- **Read before you climb.** Trace the real flow end to end (fetcher → Fields/RawStruct → findings function → catalog `FindingDef` → color classifier → demo fixture → fake → rendered surface) before choosing the smallest change. A small diff in the wrong place is a second bug.
- **One truth source.** Never compute the same fact twice. If two places would need the same condition, move it to one function and call it from both.
- **Fix the class, not the instance.** A defect found in one type is checked in its siblings before the round closes — every wave-1 fetcher and every wave-2 enricher of every type in the batch, listed with `file:line` in the round entry. A class closed on one wave, one type or one surface is still open.
- **A stale pin never blocks a round.** When a spec row deletes behaviour an existing test asserts, dev inverts that test in the same round with a comment naming the row, lists it in the log, and QA verifies the inversion. It is the one test dev may touch; the gate stays green and neither role waits on the other.
- **A criterion is met as written or logged as OFF.** Verifying a weaker check ("is a prefix of" for "equals") and signing off is a finding against the verifier, not a verification.
- **A deletion is a hypothesis.** "Dead", "unreachable", "nothing calls it" is proven by grepping the symbol across every caller and running the gates on the deletion, not by reasoning from the path you were editing.
- **Deferral is written, never carried in your head.** See the `deferred:` line above.
- **Every round is simplified before it is handed on.** See the `simplified:` line above: ponytail-review on the round's diff, its proposals judged by the deletion rule, the result written down.
- **No confidence filter.** Every finding is fixed or disproved with `file:line` evidence in the log. "Pre-existing", "minor", "probably fine" are not dispositions.
- **Nothing real.** No real AWS account IDs, profile names, bucket/secret/DNS names, or e-mails anywhere — synthetic `123456789012`, `example-readonly`, `acme-*`. `scripts/check-no-real-data.sh` is the gate.
- **Comments earn their place.** No comment that restates the code, narrates a change, or argues with a reviewer. Rationale, constraints, gotchas, external context only.
- **Shell discipline.** One command per Bash call. No `&&`, `;`, `|`, `$()` or backticks; write intermediates to `TASKDIR` files and read them in the next call. Use `/usr/bin/grep` for exhaustive enumeration (the proxied `grep` drops matches in bulk mode). Run `graphify query "<question>"` before grepping raw source when `graphify-out/graph.json` exists.
- **Report what you ran, in the shape the gate hook reads.** Gate results are pasted from captured output, never relayed. A dev round that logs `DONE` must leave `$TASKDIR/gate.txt` holding an `EXIT=` line under both `make test` and `make lint`, captured after the last edit:

  ```bash
  : > $TASKDIR/gate.txt
  printf '## gate: make test\n' >> $TASKDIR/gate.txt
  make -C $WORKTREE test >> $TASKDIR/gate.txt 2>&1
  printf 'EXIT=%s\n' $? >> $TASKDIR/gate.txt
  printf '## gate: make lint\n' >> $TASKDIR/gate.txt
  make -C $WORKTREE lint >> $TASKDIR/gate.txt 2>&1
  printf 'EXIT=%s\n' $? >> $TASKDIR/gate.txt
  ```

  This is the whole contract, and it is stated here only. A `## gate: <name>` marker names the gate, because a command's own text never appears in its output; the exit line the recipe appends last is the command's, so a stray `EXIT=0` inside a test transcript cannot rescue a red gate. The first line truncates, so the file always describes exactly one run: a gate appearing twice means it was appended across two runs, and the hook refuses it. Read it back with `tail`/`grep` and paste the exit lines into the log. `SubagentStop` parses this shape and hands the round back when the file is missing, stale, or red.
- **Never push, never open a PR, never merge, never tag.** Commits stay local in the worktree; the orchestrator integrates.

## One agent in a worktree at a time

Parallelism lives across tasks, not inside one. Two agents editing one worktree overwrite each other, and `go vet ./...` compiles `tests/` — a half-written test file blinds vet for production code.

So each task runs its rounds in sequence, each starting from the previous round's commit:

| Round | Agent | Starts from | Commits |
|---|---|---|---|
| 0 | `a9s-dev` | the task's base commit | compile-clean zero-value stubs for every symbol the spec pins — `stub(<scope>): symbols for <task>` |
| 1 | `a9s-qa` | dev's stub commit | red tests that fail on assertions, not on compile |
| 1 | `a9s-dev` | qa's red commit | the implementation that turns them green |
| verify | `a9s-qa` | dev's commit | the adversarial round: findings or sign-off |

Findings send it back to dev, from QA's commit. The `from:` line of every log entry names the commit it started from.

A compile error in a file you do not own is not a teammate mid-edit — nobody else is in the worktree. It is a real break in the tree you inherited: report it as a finding or log `BLOCKED` naming the file. Never "fix" the other role's file.

## Orchestrator checklist

The orchestrator dispatches, integrates, and writes nothing else — with one fast path. A change with no behaviour (a comment, a doc cell or sentence, a changelog line) or a one-line code change whose correctness is evident from reading and already covered by a gate is done by the orchestrator directly on `main`, gated by what covers it (`make mdlint`; `make test` and `make lint`), and committed with a clear message. Anything that adds a symbol, changes a rendered value, moves a fixture count or golden, or needs a witness goes through the loop. When in doubt, the loop. This is the user's rule (2026-09-05): a two-round cycle for an em-dash cell is waste.

- **One task in flight by default, two at most, and two only with disjoint file sets** (the user's rule, 2026-09-06, after the round census: four of one day's rounds were rebases and every batch shared the catalog files). Queue the rest. One dev and one QA per live task, one acceptance agent for all of them, no standing facilitator (the orchestrator rules; a facilitator is dispatched only for a stalled loop); an agent is retired when its task lands, and no idle agent is kept around to be woken later.
- **Pilot first.** When several batches share a shape, one goes through QA sign-off and acceptance before its siblings are dispatched; every ruling it produces is folded into the common spec first.
- **A gate change lands alone.** A batch that extends a test gate lands before its siblings sign off, and every in-flight branch is rebased onto it (or re-cut) before its next verify round; a branch green against its own base proves nothing about a gate it does not carry.
- **No two live tasks touch the same file.** This is the user's rule (2026-09-06: "you must prevent same files clashes"), and globs are not enough to keep it: every task ends up in `CHANGELOG.md`, `docs/attention-signals.md`, the count pins and the coverage allowlists. Before dispatching, compute the new task's file set from the spec (the files it names plus the per-type files of every type it touches, including `docs/resources/<type>.md`, `core/demo/fixtures/<type>.go` and the tests that pin that type) and intersect it with `git diff --name-only origin/main...task/<x>` for every live branch. A non-empty intersection means the second task waits for the first to land, or is cut from the first's tip and lands after it in that order; it never runs beside it. A task that discovers mid-flight that it needs a file another live task holds logs `BLOCKED` naming the file, and the orchestrator serialises the two. The shared hubs that force this on unrelated work (`CHANGELOG.md`, the demo count pins in `tests/integration/four_rules_demo_test.go`, `tests/unit/qa_demo_state_coverage_test.go`, `docs/attention-signals.md`) are a defect in the tree's shape, tracked as per-task changelog fragments and per-type pin files; until those land, `CHANGELOG.md` is the one file exempt from the rule and is resolved at landing by keeping both sides.
- **Worktrees are cut from committed HEAD.** Never with uncommitted work applied; a worktree that carries someone's WIP leaks it into commits.
- **One agent per worktree at a time.** Dispatch the next role only after the previous one's round entry is in `log.md` **and** `ListAgents` shows no running agent for that worktree — the log is necessary, not sufficient. Check the branch tip before dispatching and name it in the dispatch.
- **Write `$WORKTREE/.claude/task-context.md`** with `WORKTREE=`, `TASKDIR=` and `ROLE=` on every dispatch.
- **Facilitator on the first unruled deviation.** When QA or dev names a spec gap or a conflict between a ruling and a standing test, dispatch `a9s-facilitator` the first time, not the third; a ruling given by message is not a ruling.
- **Acceptance after every sign-off**, on a clean detached checkout of the landed tip, with the spec and its rulings as criteria. Never one integrated pass at the end.
- **After a usage-limit reset, resume by name.** The agent keeps its transcript; `git status` in its worktree and the log say where it stopped. Never re-dispatch a fresh agent into a tree with half a round in it.
- **Route every `deferred:` line** to the owning spec or the backlog with an owner before the next dispatch into that worktree.

### Landing checklist

1. The sign-off's `from:` equals the hash you dispatched, and `git log <signed-off>..<branch>` is empty; triage anything above it first.
2. `ListAgents` shows no running agent for the worktree.
3. Cherry-pick the branch onto a landing branch cut from `main` in a clean worktree; resolve conflicts there; if the branch was cut with WIP present, reverse-apply every WIP hunk and diff the result against the WIP file list.
4. On the landing tree: `go vet ./...`, `make test`, `make lint` (retry once on the cross-worktree lock), `make check-catalogen`, `make mdlint`, `make security`.
5. Fast-forward `main`; if the primary checkout has uncommitted work on a file the branch touches, set that file aside (`git diff` to a file, `git checkout --`), fast-forward, restore with `git apply --3way`, unstage, and confirm the primary tree still builds.
6. Acceptance on a fresh detached worktree at the new tip; delete the batch branch and worktree only after that dispatch.
7. `graphify update .` on the fast-forwarded main tip, so the graph every dispatch is told to query describes the tree the next task starts from.

## Ending a round

The last thing a round does is append its log entry. The agent's final message to the orchestrator is a copy of that entry — nothing else.
