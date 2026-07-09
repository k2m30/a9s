---
name: a9s-traverse
description: Full live traversal of the a9s TUI against a real *readonly* AWS account — every resource type walked end to end. Use when the user asks to "traverse", "walk everything", "full sweep", "check all types live", or wants a comprehensive audit that main-menu issue counts match the lists, every detail opens with a working related panel, related pivots drill correctly, and no error surfaces anywhere. Broader and more exhaustive than `a9s-smoke` (which samples a few types).
---

# a9s full traversal

Drive the **compiled binary** (`./a9s`) in a headless tmux session against a
real `*readonly` profile and walk EVERY non-empty resource type end to end.
The unit/integration suites and the demo smoke can't catch account-shaped
defects: an issue count that disagrees between the menu and a list, a related
pivot stuck `(?)`, a `Tab`+`Enter` that no-ops on a deferred pivot, a raw enum
that only leaks on real data.

Never launch `./a9s` interactively in the foreground — always tmux + staged
`capture-pane`. Live runs use `*readonly` profiles only. **Never paste a real
account ID, profile name, or resource name into a file, commit, or report** —
captures live in `/tmp`; mask identifiers (`<redacted>`, `123456789012`) in
anything shown to the user (`scripts/check-no-real-data.sh` guards commits).

## The five checks (the contract)

1. **Main menu** — issue counts and cache/enrichment. Wait for enrichment to
   **settle** (title reads `resource-types(N)` with NO `[enriching X/Y]`)
   before trusting any `issues:N` badge.
2. **List per type** — for every non-zero type, the list `(count)` and the
   `!N` issue suffix must match the settled menu's `(count)` and `issues:N`.
3. **Detail per type** — open one resource; fields render, a RELATED panel is
   present, and there are **zero `(?)`** rows (all checks settled to a count).
4. **Follow related** — the counts are right, and drilling works: `Tab` lands
   on the first **drillable** pivot and `Enter` navigates to it; a count-1
   drill lands on the target detail.
5. **No errors** — no `FetchByIDs failed`, `panic`, `AccessDenied`,
   `unresolved reference`, `map[`, or raw `UPPER_SNAKE` cell anywhere.

## Setup

```sh
PROFILE=<readonly-profile>; REGION=<region>     # discover: aws configure list-profiles | grep readonly
aws sts get-caller-identity --profile "$PROFILE" >/dev/null   # creds valid?
make build
```

## Walk (adapt the type list from the settled menu)

```sh
S=trav; OUT=/tmp/trav; rm -f ${OUT}_*.txt
tmux new-session -d -s $S -x 230 -y 55 "./a9s --profile $PROFILE --region $REGION"
sleep 30                                    # enrichment settle — do NOT read issue counts before this
tmux capture-pane -t $S -p > ${OUT}_menuA.txt
tmux send-keys -t $S G; sleep 1; tmux capture-pane -t $S -p > ${OUT}_menuB.txt; tmux send-keys -t $S g

for t in <non-zero types from the menu>; do
  tmux send-keys -t $S ":$t" Enter; sleep 5; tmux send-keys -t $S C-r; sleep 4   # live: Ctrl+R + settle
  tmux capture-pane -t $S -p > ${OUT}_list_${t}.txt
  tmux send-keys -t $S d; sleep 6
  tmux capture-pane -t $S -p > ${OUT}_detail_${t}.txt
  tmux send-keys -t $S Escape; sleep 1
done
tmux kill-session -t $S
```

## Judge from the captures

- **Errors (step 5)** — `grep -lE 'FetchByIDs failed|panic|AccessDenied|not authoriz|unresolved reference|map\[' /tmp/trav_*.txt`. Any hit is a defect; quote the line.
- **Counts (step 2)** — extract each list title (`sed -n '2p'`) and compare its `(count) !N` to the settled menu's `(count) issues:N`. **They must match.** A menu that undercounts (or a list that over-counts) is a bug — the badge aggregates Wave-1 issue-colored rows + Wave-2 `!`-severity findings only; `~` warnings do NOT bump (docs/attention-signals.md S1).
- **Details (step 3)** — every detail capture has `detail --` in its title, a `RELATED` block, and zero `(?)`.
- **Drills (step 4) — exhaustive.** "Follow ALL related resources" means drill
  **every** actionable pivot, not a sample. `Esc` back from a drill preserves the
  related focus and cursor, so you can walk the whole panel in place:

  ```sh
  # per type, in its detail:
  tmux send-keys -t $S d; sleep 6; tmux capture-pane -t $S -p > detail.txt
  K=$(awk '/RELATED/{f=1} f' detail.txt | grep -coE '\([1-9][0-9]*\+?\)|\(\?\)')  # actionable rows
  tmux send-keys -t $S Tab; sleep 2                # lands on the first DRILLABLE pivot
  i=1; while [ "$i" -le "$K" ]; do
    tmux send-keys -t $S Enter; sleep 4
    tmux capture-pane -t $S -p > drill_${i}.txt     # classify the landing
    tmux send-keys -t $S Escape; sleep 2            # back to the same detail, cursor preserved
    tmux send-keys -t $S Down; sleep 1              # next actionable pivot
    i=$((i + 1)); done
  ```

  Classify each landing: a `detail -- <target>` or `type(N)` list frame = the
  pivot navigated (a count-1 drill lands on the target detail; count-N on a list
  of N — verify N matches the badge); staying on the source detail = a deferred
  `(?)` pivot re-dispatching (expected, not a dead-end). Any `FetchByIDs failed`
  or a `(0)`-count row that navigated is a defect. `Tab` lands on the first
  **drillable** pivot, skipping deferred `(?)` rows (alarm/ebs-snap/backup/
  ct-events) that only re-dispatch on Enter.

## Gotchas learned the hard way

- **Enrichment settle first.** The menu's `issues:N` is only final after
  `[enriching X/Y]` clears (~20-30s live). Reading it early gives false
  undercounts.
- **Distrust the first render.** Live rows seed from cache instantly then swap;
  `Ctrl+R` and wait 8-10s before judging a list, longer for `s3`/`role`/
  `event` (paginated / slow APIs).
- **The nav stack goes deep.** Each `:type` pushes a screen; `Esc` unwinds ONE
  level (header shows `[N]` depth). To get a clean settled menu, re-launch
  fresh rather than spamming `Esc`.
- **Menu ≠ list is a real bug.** If the counts disagree, the two aggregations
  have drifted (`unifiedIssueCount` for the menu vs `listIssueCount` for the
  list) — both must run `runtime.Wave1Only` the same way.
- **`Tab`+`Enter` no-op = wrong landing.** If it stays on the source, the
  cursor parked on a `(0)` dead-end or a deferred `(?)` pivot; the fix is
  `detailSkipToDrillable` (see docs/related-resources-engine.md §6). Instrument
  the live Enter handler (`internal/tui/app_stack.go`) before theorizing — two
  cursor-lane theories were refuted that way.

## Make findings permanent

A traversal that surfaces a class of defect should leave a guard behind: a
unit pin (e.g. `tests/unit/app_related_focus_entry_test.go` for the drillable
landing, the issue-count pins for menu/list parity) or an assertion in the
demo smoke (`scripts/smoke-demo.sh` / `smoke-related-demo.sh`), so the class
can't silently return.
