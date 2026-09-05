---
name: a9s-team-loop
description: The dev / qa / facilitator / acceptance loop — shared protocol every team agent follows (task workspace, round log, statuses, escalation, shell rules)
---

# Team loop protocol

Four roles, one loop. The orchestrator (the main session) dispatches; it writes no code and no tests.

```text
          ┌──────────── findings ────────────┐
          ▼                                  │
 spec ─► a9s-qa (tests first) ─► a9s-dev (impl) ─► a9s-qa (verify) ─► QA SIGN-OFF ─► a9s-acceptance ─► ACCEPT
                                  │                     │                                     │
                                  └── OFF / LOOP / BLOCKED ──► a9s-facilitator (ruling) ◄─────┘ REJECT
```

- **a9s-qa** writes the failing tests from the spec, then verifies every dev round adversarially.
- **a9s-dev** makes the tests pass with the smallest correct change and keeps every gate green.
- **a9s-facilitator** is called only on `OFF`, `LOOP`, `BLOCKED`, or when a task passes round 3. It rules; it never codes.
- **a9s-acceptance** is the skeptical end user. It sees the finished tree, not the log, and accepts or rejects with evidence.

## Task workspace

Every dispatch names two absolute paths:

- `WORKTREE` — the git worktree to build in. Every `go` / `make` invocation targets it: `go -C $WORKTREE build ./...`, `make -C $WORKTREE lint`. Reference files by absolute path under it. Never touch any other checkout.
- `TASKDIR` — the task folder. It holds `spec.md` (the contract, written by the orchestrator or rewritten by the facilitator) and `log.md` (the append-only round log). Scratch output (`gate.txt`, captures, patches) goes here too, never in `/tmp`.

## Round log — `TASKDIR/log.md`

Append one entry per round, never edit earlier entries:

```markdown
## a9s-dev · round 2 · DONE
- changed: core/aws/sqs_issue_enrichment.go:41-88 (policy verdict via iampolicy.Evaluate), core/demo/fixtures/sqs.go:120 (PublicQueueName witness)
- gates: go build OK · go vet OK · go test ./tests/unit -run 'SQS' OK (14 tests) · make lint OK
- skipped: none
- open: none
```

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
- **Report what you ran.** Gate results are pasted from captured output (`make -C $WORKTREE test > $TASKDIR/gate.txt 2>&1; echo "EXIT=$?" >> $TASKDIR/gate.txt`), then read with `tail`/`grep`. Never relay a number you did not reproduce.
- **Shell discipline.** One command per Bash call. No `&&`, `;`, `|`, `$()` or backticks; write intermediates to `TASKDIR` files and read them in the next call. Use `/usr/bin/grep` for exhaustive enumeration (the proxied `grep` drops matches in bulk mode). Run `graphify query "<question>"` before grepping raw source when `graphify-out/graph.json` exists.
- **Interleaving with a teammate.** dev and qa work in the same worktree concurrently. A compile error in a file you do not own is the other role mid-edit: wait 30 s and retry, up to 10 times, then log `BLOCKED` naming the file. Never "fix" the other role's file.
- **Never push, never open a PR, never merge, never tag.** Commits stay local in the worktree; the orchestrator integrates.

## Ending a round

The last thing a round does is append its log entry. The agent's final message to the orchestrator is a copy of that entry — nothing else.
