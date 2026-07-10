---
name: a9s-traverse
description: Full live traversal of the a9s TUI against a real *readonly* AWS account — every resource type walked end to end. Use when the user asks to "traverse", "walk everything", "full sweep", "check all types live", or wants a comprehensive audit that main-menu issue counts match the lists, every detail opens with a working related panel, related pivots drill correctly, and no error surfaces anywhere. Broader and more exhaustive than `a9s-smoke` (which samples a few types).
---

# a9s full traversal

Drive the **compiled binary** (`./a9s`) in a headless tmux session against a
real `*readonly` profile and walk EVERY non-empty resource type end to end.
The unit/integration suites and the demo smoke can't catch account-shaped
defects: an issue count that disagrees between the menu and a list, a related
pivot stuck unresolved, a `Tab`+`Enter` that no-ops on a deferred pivot, a raw
enum that only leaks on real data.

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
   present, and every row has **settled** — a count `(N)`/`(N+)`, a proven `(0)`,
   an errored `—`, or a blank deferred/unknown row — with none left dimmed
   "checking". (A blank row is a valid settled state; `(?)` is retired and is
   never rendered.)
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
- **Counts (step 2)** — extract each list title (`sed -n '2p'`) and compare its `(count) !N` to the settled menu's `(count) issues:N`. **PRESERVE the trailing `+`** — grep `issues:[0-9]+\+?` / `!?[0-9]+\+?`, never `issues:[0-9]+` (which silently drops the `+`). A `+` marks a **truncated lower bound**: the menu's availability probe and the list paginate to different depths, so `issues:50+` (menu) vs `!51+` (list) are BOTH "≥N, more exist" and are **not a mismatch** — only compare when NEITHER side carries a `+`. Dropping the `+` turns two honest lower bounds into a phantom off-by-one (this cost a long false-positive chase). For **exact** (no-`+`) counts they must match: a menu that undercounts (or a list that over-counts) is a bug — the badge aggregates Wave-1 issue-colored rows + Wave-2 `!`-severity findings only; `~` warnings do NOT bump (docs/attention-signals.md S1). A stable exact mismatch is usually **duplicate resource IDs**: the menu badge (`unifiedIssueCount`) and the list title (`listIssueCount`) count per resource, so two DISTINCT resources sharing an ID (e.g. two ACM certs for one domain — ACM keys on the domain) each count — a fetcher that emits non-unique IDs is the root.
- **Details (step 3)** — every detail capture has `detail --` in its title, a `RELATED` block, and no row left dimmed "checking" (a blank deferred/unknown row is fine — it is not an error, and `(?)` is never rendered).
- **Drills (step 4) — exhaustive.** "Follow ALL related resources" means drill
  **every** actionable pivot, not a sample. `Esc` back from a drill preserves the
  related focus and cursor, so you can walk the whole panel in place:

  ```sh
  # per type, in its detail. K = actionable-row count. A row is actionable
  # UNLESS it is a proven-zero (0) dead-end or an errored "—" row. Actionable
  # therefore includes (N>=1), ANY truncated (N+) INCLUDING (0+), and BLANK
  # deferred/unknown rows (a named pivot with NO count — "(?)" is retired and
  # never rendered; see related-resources-engine.md §2/§7). Counting only
  # positive (N) undercounts and skips exactly the deferred no-op pivots this
  # skill exists to catch. The RELATED panel is the right column (it shares
  # lines with the left detail fields), so read the last cell of each row:
  tmux send-keys -t $S d; sleep 6; tmux capture-pane -t $S -p > detail.txt
  K=$(awk -F'│' '
    /RELATED/{inrel=1; next}
    inrel && NF>=3 {
      c=$(NF-1); gsub(/^ +| +$/,"",c)
      if (c=="")               next          # below the panel
      if (c ~ /—/)             next          # errored row — dead end
      if (c ~ /\([0-9]+\+\)/)  {n++; next}    # (N+)/(0+) truncated — actionable
      if (c ~ /\([1-9][0-9]*\)/) {n++; next}  # (N>=1) resolved — actionable
      if (c ~ /\([0-9]+\)/)    next          # (0) proven-zero — dead end
      if (c ~ /^[A-Z]/)        n++            # blank named pivot — deferred, actionable
    }
    END{print n+0}' detail.txt)
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
  of N — verify N matches the badge); staying on the source detail = a **blank
  deferred/unknown** pivot re-dispatching in place (expected, not a dead-end).
  Any `FetchByIDs failed` or a `(0)`-count (non-truncated) row that navigated is
  a defect. `Tab` lands on the first **drillable** pivot, stepping over blank
  deferred rows (alarms/ebs-snap/backup/ct-events) that only re-dispatch on
  Enter — but they are still actionable, so the Down walk above must reach every
  one of them (that is why K counts blank and `(0+)` rows).

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
- **Menu ≠ list — rule out the `+` FIRST, then instrument, then theorize.**
  Before calling it a bug, confirm the grep kept the trailing `+`: `50+` vs
  `51+` are truncated lower bounds, not a divergence (see step 2). For a stable
  EXACT mismatch, do NOT theorize about `Wave1Only` / severity / color from the
  captures — **instrument** both `unifiedIssueCount` (menu) and `listIssueCount`
  (list) to dump each resource's id + counted decision, run live with the env
  gate, and diff. That diff nailed the ACM case in one shot (two certs sharing
  the domain-id: the menu's set-by-id collapsed them, the list's per-row counter
  did not) after three from-the-capture theories were each refuted. The two
  aggregations must count per-resource identically (both strip `runtime.Wave1Only`,
  both count per row not per id).
- **`Tab`+`Enter` no-op = wrong landing.** If it stays on the source, the
  cursor parked on a `(0)` dead-end or a blank deferred pivot; the fix is
  `detailSkipToDrillable` (see docs/related-resources-engine.md §6). Instrument
  the live Enter handler (`internal/tui/app_stack.go`) before theorizing — two
  cursor-lane theories were refuted that way.

## Make findings permanent

A traversal that surfaces a class of defect should leave a guard behind: a
unit pin (e.g. `tests/unit/app_related_focus_entry_test.go` for the drillable
landing, the issue-count pins for menu/list parity) or an assertion in the
demo smoke (`scripts/smoke-demo.sh` / `smoke-related-demo.sh`), so the class
can't silently return.
