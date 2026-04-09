---
name: a9s-demo-gif
description: Record and annotate the a9s demo GIF using VHS and ffmpeg overlays. Use when updating the demo GIF for README/website.
---

## Overview

The demo GIF is built in two stages:
1. **VHS** records terminal actions into a raw GIF (`demo-raw.gif`)
2. **ffmpeg** adds timed text overlay captions, producing the final `docs/demos/demo.gif`

## Prerequisites

- `vhs` — `brew install vhs`
- `ffmpeg-full` — `brew install ffmpeg-full` (keg-only, needed for `drawtext` filter)
- Built binary: `make build`

## Files

| File | Purpose |
|------|---------|
| `docs/demos/demo.tape` | VHS script — actions, timing, keystrokes |
| `docs/demos/add-overlays.sh` | ffmpeg overlay script — captions with timestamps |
| `docs/demos/demo.gif` | Final output (committed, used by README and website) |
| `docs/demos/demo-scenario.md` | Human-readable scenario with act descriptions |

## Recording Workflow

### 1. Build the binary
```
make build
```

### 2. Run VHS
```
vhs docs/demos/demo.tape
```
This produces `docs/demos/demo-raw.gif` (intermediate, do NOT commit).

### 3. Apply overlays
```
bash docs/demos/add-overlays.sh
```
This reads `demo-raw.gif`, applies timed captions, outputs final `docs/demos/demo.gif`.

### 4. Clean up intermediate
```
rm docs/demos/demo-raw.gif
```

### 5. Review
Open `docs/demos/demo.gif` and verify caption sync. Adjust timestamps in `add-overlays.sh` if needed, then re-run step 3.

## VHS Tape Syntax

Key patterns used in the tape:

```tape
Set TypingSpeed 50ms          # Global default (fast, for single-key commands)
Type@300ms "/web"             # Per-command override (visible typing for demos)
Down@300ms 3                  # Press Down 3 times with 300ms between each
Escape@500ms 3                # Press Escape 3 times with 500ms between each
Tab                           # Single keypress
Hide / Show                   # Hide launch command from recording
```

- `Key@<delay> <count>` — repeat a key N times with delay between presses
- `Type@<delay> "<text>"` — type text with per-character delay (overrides global TypingSpeed)
- Single-character commands (`d`, `y`, `r`) use the global 50ms speed — they appear instant, which is correct
- Multi-character inputs (`/web`, `:s3`, `:lambda`) MUST use `Type@200ms` or `Type@300ms` so the viewer sees them being typed
- After command-mode inputs (`:s3`, `:lambda`, `:rds`), add `Sleep 1s` before `Enter` so the viewer sees the typed command

## Overlay Timestamp Calculation

**Critical:** Overlay timestamps in `add-overlays.sh` must match the actual timing from the tape. Every change to the tape shifts all subsequent timestamps.

### How to calculate

Trace through the tape line by line, accumulating time:
- `Sleep Xs` — adds X seconds
- `Down@300ms 3` — adds 3 * 300ms = 0.9s
- `Type@300ms "/web"` — adds 4 * 300ms = 1.2s (character count * per-char delay)
- `Type "d"` — adds 1 * 50ms = 0.05s (uses global TypingSpeed)
- `Escape@500ms 3` — adds 3 * 500ms = 1.5s
- `Enter`, `Tab`, single keys — near-instant, ~0.05s

The timing trace MUST be kept in the header comment of `add-overlays.sh` so future edits can verify sync.

### Overlay format

Two lines per caption:
- **Description line** — white, 44pt, describes what's happening
- **Key hint line** — blue (#7aa2f7), 32pt, shows the keybinding

Both are bottom-centered with semi-transparent black background boxes.

### Text constraints

- NO Unicode arrows or symbols (render as empty boxes in ffmpeg). Use `Up / Down`, `|` as separator
- Use `--` instead of em-dash
- Colons in text must be escaped: `\:s3` not `:s3`
- Font: Arial Unicode (`/System/Library/Fonts/Supplemental/Arial Unicode.ttf`)
- ffmpeg binary: `/opt/homebrew/opt/ffmpeg-full/bin/ffmpeg`

## App Behavior Notes

- **Related panel auto-shows** on detail view. Do NOT press `r` (that closes it). Use `Tab` to focus the right column, `Down` to browse, `Enter` to navigate.
- `--demo` mode uses synthetic fixture data, no AWS credentials needed.
- The GIF should start and end on the main menu for seamless looping.

## Current Act Structure

| Act | Feature | Key sequence |
|-----|---------|-------------|
| 1 | Main menu | Down x3, pause |
| 2 | EC2 list + filter | Enter, Down x2, `/web`, Esc |
| 3 | Detail + YAML | `d`, Down x3, `y`, Esc x3 |
| 4 | Related views | Enter, Down, `d`, Tab, Down x3, Enter, Esc x3 |
| 5 | S3 drill-down | `:s3`, Enter x2, Esc x2 |
| 6 | Quick tour | `:lambda`, `:rds`, Esc |

## File Size

GIF size is dominated by resolution (1200x500) and color depth, not framerate. The ffmpeg pass downsamples to 10fps via `fps=10` filter. Expect ~3MB for a 70s recording. This is acceptable for README embedding.
