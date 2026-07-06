---
name: a9s-smoke
description: Run the a9s TUI smoke test — tmux-driven, real binary, rendered-surface assertions. Use whenever the user says "смоук"/"smoke", asks to verify the app end to end, after any UI-affecting change, or before a push (the demo smoke is part of `make ready-to-push`). Covers the scripted BASE walk plus checks you compose for whatever THIS session changed.
---

# a9s smoke testing

The unit and integration suites drive models and controllers; the smoke drives
the **compiled binary** in a headless tmux session and checks what a user
actually sees rendered. Whole bug classes live only there: a raw enum leaking
into a column, a phrase emitted on the demo path but not the live one, a
panel rendered from the wrong stacked screen.

A smoke run is always TWO parts:

1. **Base walk** — scripted, below.
2. **Session checks** — composed by you, for what the current session
   changed. Never skip this part.

Never launch `./a9s` interactively in the foreground — always tmux with
staged `capture-pane` snapshots. Live runs use `*readonly` profiles only.

## Base walk

Demo (deterministic, asserted, part of `make ready-to-push`):

```
make smoke            # scripts/smoke-demo.sh, ~60s
```

Menu catalog, list counts and humanized statuses, owner-worded sg risks,
per-row causes, filter narrowing, detail + Attention, related witnesses,
YAML view, circular drill re-showing cached badges, version+depth header.
When demo fixtures legitimately change, update the script's assertions in
the same PR.

Live (data-independent patterns, credentials required, not in the gate):

```
make smoke-live PROFILE=gobubble-dev-readonly REGION=eu-west-2
```

Sweep reaches issue badges in-session, no whole-cell raw enums anywhere,
issue-titled lists carry cause phrases, an ec2 detail's related checks
settle to counts and its drills produce no fetch errors. Refuses non-readonly
profiles.

## Session checks — compose them every run

Enumerate what the session changed (`git diff --stat main...` plus the
conversation) and write a tmux check per touched surface. Map:

| You changed | You verify |
|---|---|
| classifier / findings / phrases | the type's list: status column carries the new phrase; no raw token; title `!N` matches explained rows |
| enricher (wave-2) | open the list on live, wait/Ctrl+R, re-capture — demo warmth masks dispatch bugs |
| related checker / registration | open a source detail, panel shows the pivot with a sane badge; drill count-1 lands on the target DETAIL |
| navigation / stack | walk the exact chain (including circular A→B→A) and assert the landing frame title |
| renderer / columns / badges | capture the surface at 220 columns and grep the exact expected text |
| fetcher / pagination | list title count vs menu count consistency; load-more keeps the wider total |

Recipe skeleton:

```sh
tmux new-session -d -s smoke -x 220 -y 50 '<binary> --demo'   # or --profile <ro> --region <r>
sleep 4
tmux send-keys -t smoke ':<type>' Enter
sleep 3                       # live: 8-10s, and Ctrl+R before judging
tmux capture-pane -t smoke -p > /tmp/smoke_<type>.txt
tmux kill-session -t smoke
```

Judging: a capture is evidence — quote the offending line. Distrust the
first render on live (cached rows seed instantly and swap silently). A
rendered surface disagreeing with a green unit suite means the gap is
between the model layer and the renderer: fix the wiring, then make the
class permanent — extend `scripts/smoke-demo.sh`, the visibility/style
gates (`tests/unit/qa_issue_*_gate_test.go`), or both.
