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

Related panel, demo (deterministic, asserted, part of `make ready-to-push`):

```
make smoke-related        # scripts/smoke-related-demo.sh, ~30s
```

Dedicated RELATED-panel edge cases: exact fixture witness badges (CloudTrail
Trails, KMS Key), the bare CloudTrail Events pivot, the zero-count-row
cursor skip, a count-1 drill landing on the target detail (not a list), a
circular drill (bucket -> trail -> bucket) re-showing cached counts with the
depth badge intact, Esc unwinding back to the same detail, and the ec2 IAM
Role pivot.

Cost Explorer, demo (deterministic, asserted, part of `make ready-to-push`):

```
make smoke-costs          # scripts/smoke-costs-demo.sh, ~60s
```

Grid opens at the current month (open-period marker, stripped vendor
prefixes, no fold/no counter/no negative zero, key-hint bar), month → week
→ day zoom carries data, zoom-out past year without a CE validation error,
metric cycle (unblended drops Tax), account pivot, the planted growth-story
drill to usage types, the 14-day resource-boundary message on old cells,
synthetic resource rows on the current month, the Cost Explorer help
section, Esc back to the menu, and `-c costs` startup. The cost fixtures
anchor to the current month at process start, so assertions never rot.

Live (data-independent patterns, credentials required, not in the gate):

```
make smoke-live PROFILE=<readonly-profile> REGION=<region>
```

Sweep reaches issue badges in-session, no whole-cell raw enums anywhere,
issue-titled lists carry cause phrases, an ec2 detail's related checks
settle to counts and its drills produce no fetch errors. Refuses non-readonly
profiles.

Related panel, live (data-independent, credentials required, not in the gate):

```
make smoke-related-live PROFILE=<readonly-profile> REGION=<region>
```

Pattern-based RELATED-panel checks against a real account: the first
candidate type (ec2, lambda, s3, sg) that settles a counted badge, a drill
of the first actionable pivot landing on a detail or list frame with Esc
returning to the same title, and — if a "(?)" row is visible — Enter
navigates and the row never dead-ends. Refuses non-readonly profiles.

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
