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
- from: 4f1a9c2 (a9s-qa round 1 red tests)
- changed: core/aws/sqs_issue_enrichment.go:41-88 (policy verdict via iampolicy.Evaluate), core/demo/fixtures/sqs.go:120 (PublicQueueName witness)
- gates: go build OK · go vet OK · go test ./tests/unit -run 'SQS' OK (14 tests) · make lint OK
- skipped: none
- open: none
```

The `from:` line names the commit the round started from — the other role's last commit. It is how the next agent knows the tree it inherits.

Status values, exactly one per entry:

| Status | Meaning | Next |
|---|---|---|
| `DONE` | this round's work is complete and verified as stated | the other role's turn |
| `FINDINGS` | (qa only) numbered defects with file:line and a failing test each | dev round |
| `SIGN-OFF` | (qa only) every spec row has a passing behavioural test, gates green | acceptance |
| `ACCEPT` / `REJECT` | (acceptance only) with numbered evidence | done / dev round with facilitator |
| `BLOCKED` | cannot proceed without something outside the worktree (credentials, a missing SDK field, a decision) | facilitator |
| `OFF` | "something's off": the spec contradicts the code, the test encodes a defect as intent, a fix would need a second truth source, the change is growing past its size | facilitator |
| `LOOP` | the same finding has bounced dev↔qa twice without converging | facilitator |

Read the whole log before starting a round. Rounds are numbered per role; round 1 is the first entry of that role.

## Non-negotiables (all roles)

- **Read before you climb.** Trace the real flow end to end (fetcher → Fields/RawStruct → findings function → catalog `FindingDef` → color classifier → demo fixture → fake → rendered surface) before choosing the smallest change. A small diff in the wrong place is a second bug.
- **One truth source.** Never compute the same fact twice. If two places would need the same condition, move it to one function and call it from both.
- **Fix the class, not the instance.** A defect found in one type is checked in its siblings before the round closes.
- **No confidence filter.** Every finding is fixed or disproved with `file:line` evidence in the log. "Pre-existing", "minor", "probably fine" are not dispositions.
- **Nothing real.** No real AWS account IDs, profile names, bucket/secret/DNS names, or e-mails anywhere — synthetic `123456789012`, `example-readonly`, `acme-*`. `scripts/check-no-real-data.sh` is the gate.
- **Comments earn their place.** No comment that restates the code, narrates a change, or argues with a reviewer. Rationale, constraints, gotchas, external context only.
- **Shell discipline.** One command per Bash call. No `&&`, `;`, `|`, `$()` or backticks; write intermediates to `TASKDIR` files and read them in the next call. Use `/usr/bin/grep` for exhaustive enumeration (the proxied `grep` drops matches in bulk mode). Run `graphify query "<question>"` before grepping raw source when `graphify-out/graph.json` exists.
- **Report what you ran, in the shape the gate hook reads.** Gate results are pasted from captured output, never relayed. A dev round that logs `DONE` must leave `$TASKDIR/gate.txt` holding an `EXIT=` line under both `make test` and `make lint`, captured after the last edit:

  ```bash
  printf '## gate: make test\n' >> $TASKDIR/gate.txt
  make -C $WORKTREE test >> $TASKDIR/gate.txt 2>&1
  printf 'EXIT=%s\n' $? >> $TASKDIR/gate.txt
  printf '## gate: make lint\n' >> $TASKDIR/gate.txt
  make -C $WORKTREE lint >> $TASKDIR/gate.txt 2>&1
  printf 'EXIT=%s\n' $? >> $TASKDIR/gate.txt
  ```

  This is the whole contract, and it is stated here only. A `## gate: <name>` marker names the gate, because a command's own text never appears in its output; the exit line the recipe appends last is the command's, so a stray `EXIT=0` inside a test transcript cannot rescue a red gate. Truncate the file at the start of the round (`: > $TASKDIR/gate.txt`), read it back with `tail`/`grep`, and paste the exit lines into the log. `SubagentStop` parses this shape and hands the round back when the file is missing, stale, or red.
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

The orchestrator dispatches, integrates, and writes nothing else.

- **At most 4 tasks in flight.** Queue the rest. Review throughput is the real limit, and conflict rate climbs with the number of similar tasks running at once.
- **Every dispatch lists the task's owned file globs.** Before dispatching, check them against the globs of every in-flight task; if two share a glob, the second waits.
- **One agent per worktree at a time.** Dispatch the next role only after the previous one's round entry is in `log.md`.
- **Write `$WORKTREE/.claude/task-context.md`** with `WORKTREE=`, `TASKDIR=` and `ROLE=` on every dispatch.

## Ending a round

The last thing a round does is append its log entry. The agent's final message to the orchestrator is a copy of that entry — nothing else.
