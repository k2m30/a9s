---
name: a9s-linkedin-post
description: Create the next LinkedIn post from the content plan — writes post text, VHS tape, overlay script, and produces MP4 with captions. Picks a random color theme per video.
---

## Overview

Produces one LinkedIn post folder at a time under `docs/demos/social/`. Each invocation picks the next unprocessed topic from the content plan, creates all assets, and (for dark themes) renders the final MP4.

## Prerequisites

- `vhs` — `brew install vhs`
- `ffmpeg-full` — `brew install ffmpeg-full` (keg-only, needed for `drawtext` filter)
- Built binary: `make build` (run before first invocation)

## Source Files

| File | Purpose |
|------|---------|
| `docs/demos/social/linkedin-content-plan.md` | Master list of 77 topics with titles and hooks |
| `docs/demos/social/linkedin.tape` | Reference VHS tape (1080x1080 square, LinkedIn format) |
| `docs/demos/social/add-overlays.sh` | Reference ffmpeg overlay script |

## Output Structure

Each post produces a numbered folder:

```
docs/demos/social/
  NN-slug-name/
    post.md              # LinkedIn post text + hashtags
    demo.tape            # VHS scenario for this topic
    add-overlays.sh      # ffmpeg caption overlay script
    demo.mp4             # Raw VHS output (intermediate, do NOT commit)
    final.mp4            # Final video with captions (commit this)
```

## Step-by-Step Procedure

### Step 1: Find the next topic

1. Read `docs/demos/social/linkedin-content-plan.md`
2. List existing `docs/demos/social/[0-9][0-9]-*/` folders to find which topics are done
3. Pick the next sequential topic from the plan that has no folder yet
4. The folder number is two-digit zero-padded: `01`, `02`, ..., `77`
5. The slug is the topic title lowercased, spaces→hyphens, stripped of quotes/punctuation, max 40 chars

### Step 2: Pick a random theme

**All 11 themes:**
- Dark: `tokyo-night.yaml`, `catppuccin-mocha.yaml`, `dracula.yaml`, `nord.yaml`, `gruvbox-dark.yaml`, `solarized-dark.yaml`
- Light: `catppuccin-latte.yaml`, `gruvbox-light.yaml`, `nord-light.yaml`, `solarized-light.yaml`, `tokyo-night-light.yaml`

1. Pick one theme at random (uniform across all 11)
2. Write `~/.a9s/config.yaml` with `theme: <picked>.yaml`
3. Record which theme was picked — it determines the VHS `Set Theme` value

**The a9s theme name belongs ONLY in `~/.a9s/config.yaml`**, never in the `.tape` file. The a9s app reads its theme from config — VHS does not control app colors.

**In the `.tape` file, `Set Theme` has only two valid values:**

- `Set Theme "Builtin Dark"` — if the picked a9s theme is dark (tokyo-night, catppuccin-mocha, dracula, nord, gruvbox-dark, solarized-dark)
- `Set Theme "Builtin Light"` — if the picked a9s theme is light (catppuccin-latte, gruvbox-light, nord-light, solarized-light, tokyo-night-light)

VHS only controls the terminal chrome (background behind the app frame). Match it to dark/light so the chrome doesn't clash with the app palette. Do NOT try to match VHS theme names to a9s theme names — that couples two unrelated color systems and produces inconsistent results.

Header comment in the `.tape` file: `# Theme: <picked-a9s-theme>` (e.g. `# Theme: gruvbox-dark`) — this documents which a9s theme was used; the actual rendering comes from `~/.a9s/config.yaml`.

### Step 3: Write post.md

Create `docs/demos/social/NN-slug/post.md` with this structure:

```markdown
# Post NN: <Title from content plan>

## Theme

<theme-name> (dark|light)

## Post Text

<4-10 sentences. Start from the DevOps pain point, not the tool.
Mention a9s naturally mid-post. End with the repo link.
Tone: engineer talking to engineers. No AI-speak, no marketing.
No "revolutionize", "seamlessly", "unlock", "empower", "leverage".>

brew install k2m30/a9s/a9s
a9s --demo

https://github.com/k2m30/a9s

#AWS #TUI #DevOps #OpenSource #Go #SRE #DeveloperTools #BubbleTea

## Video Description

<1 sentence: what the video shows>

## Captions

| Start | End | Description | Key Hint |
|-------|-----|-------------|----------|
| 3.0 | 7.0 | Caption text | key hint |
| ... | ... | ... | ... |
| <last> | <end> | github.com/k2m30/a9s | brew install k2m30/a9s/a9s |
```

**Caption rules:**
- The last caption is ALWAYS the end card: `github.com/k2m30/a9s` + `brew install k2m30/a9s/a9s`
- End card duration: 3-4 seconds
- NO Unicode arrows or symbols in captions (render as empty boxes in ffmpeg). Use `Up / Down`, `|` as separator
- Use `--` instead of em-dash
- Colons in ffmpeg text must be escaped with backslash: `\:s3` not `:s3`

### Step 3.5: Verify the video path with the scenario harness

Before writing the tape, you MUST verify the entire navigation path works in demo mode using the integration test scenario harness. This catches missing fixtures, broken child views, empty lists, and wrong row indices BEFORE wasting time on VHS + ffmpeg.

**Write a temporary integration test** at `tests/integration/local_repro_linkedin_test.go`:

```go
//go:build integration

package integration

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestLinkedInPostScenario(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	var debug strings.Builder

	// Replay every navigation step the tape will perform.
	// Example: open a list, navigate down, enter child views, press keys.

	scenario.OpenList("lambda")
	scenario.ExpectCurrentListType("lambda")
	scenario.ExpectNoAPIError()

	// Log all rows to find exact indices
	for i, r := range scenario.currentListResources {
		debug.WriteString(fmt.Sprintf("[%d] %s\n", i, r.Name))
	}

	// Navigate to the target row
	scenario.Press("down")
	scenario.Press("down")

	// Enter child view
	scenario.Press("enter")
	scenario.ExpectNoAPIError()
	scenario.ExpectCurrentListType("lambda_invocations")

	// Log child rows
	for i, r := range scenario.currentListResources {
		debug.WriteString(fmt.Sprintf("  child[%d] %s (Status: %s)\n", i, r.ID, r.Status))
	}

	// ... continue for every step in the planned tape ...

	// Write debug output to /tmp for inspection
	os.WriteFile("/tmp/linkedin_scenario_debug.txt", []byte(debug.String()), 0o644)
}
```

**Run it:**

```sh
go test -tags integration ./tests/integration/ -run TestLinkedInPostScenario -count=1 -v -timeout 30s
```

**From the debug output, extract:**

1. **Exact row indices** for every Down/Up navigation (don't guess — the list order may not match fixture declaration order)
2. **Child view type names** after each Enter (confirms the child view actually triggers)
3. **Row content** to verify data is populated and visually interesting
4. **Status values** to find the specific row you want to highlight (e.g., the ERROR row)

**If anything is missing or broken:**

1. Fix fixtures in `internal/demo/fixtures/` — add missing data, fix format mismatches
2. Rebuild: `make build`
3. Re-run the scenario test to confirm the fix
4. Document what you fixed in the Step 7 report

**After the scenario test passes**, delete `tests/integration/local_repro_linkedin_test.go` — it's a throwaway verification, not a permanent test.

**Only proceed to Step 4 (writing the tape) once every navigation step is verified.** Use the exact row indices from the debug output — never guess cursor positions.

### Step 4: Write demo.tape

Create `docs/demos/social/NN-slug/demo.tape` — a VHS script tailored to this topic.

**Fixed header (always the same):**

```tape
# a9s LinkedIn Post NN: <Title>
# Theme: <theme-name>
# Run: vhs docs/demos/social/NN-slug/demo.tape

Output docs/demos/social/NN-slug/demo.mp4

Set Shell bash
Set FontSize 22
Set Width 1080
Set Height 1080
Set Padding 20
Set TypingSpeed 50ms
Set Framerate 10
```

**ALWAYS set `Set Theme` in the tape** using the VHS theme from the mapping table in Step 2. VHS `Set Theme` controls the terminal background/chrome colors. The a9s app colors come from `~/.a9s/config.yaml`. Both must be set for the video to look correct — especially for light themes.

**Launch block (always the same):**

```tape
Hide
Type "./a9s --demo"
Enter
Sleep 1s
Show
```

**Body:** Write keystrokes that demonstrate the topic's feature. Follow these timing rules:
- `Sleep Xs` between acts for the viewer to absorb
- Single keys (Enter, Escape, `d`, `y`, `t`, `J`) use global 50ms TypingSpeed — near instant
- Multi-character typed input (`:s3`, `/web`, `:lambda`) MUST use `Type@300ms` so viewer sees typing
- After typing a command (`:s3`), add `Sleep 1s` before `Enter` so viewer reads it
- `Down@400ms 3` — press Down 3 times with 400ms gaps (LinkedIn pacing is slower than README demo)
- Total video length: 20-50 seconds depending on topic complexity

**End:** Always return to a clean state (Escape back or quit). End with `Sleep 2s` for the end card overlay.

**Timing trace:** Add a comment block at the top of the tape body calculating cumulative timestamps for each act. This is critical for writing accurate overlay timestamps.

```tape
# Timing trace:
# 0.0s  — Show (after hide+launch)
# 2.0s  — Act 1 starts (Sleep 2s)
# ...
# 25.0s — End card
# 28.0s — Video ends
```

### Step 5: Write add-overlays.sh

Create `docs/demos/social/NN-slug/add-overlays.sh`:

```bash
#!/bin/bash
# Overlay captions for Post NN: <Title>
# Usage: bash docs/demos/social/NN-slug/add-overlays.sh

INPUT="docs/demos/social/NN-slug/demo.mp4"
OUTPUT="docs/demos/social/NN-slug/final.mp4"
FFMPEG="/opt/homebrew/opt/ffmpeg-full/bin/ffmpeg"
FONT="/System/Library/Fonts/Supplemental/Arial Unicode.ttf"

STYLE="fontfile=${FONT}:fontsize=48:fontcolor=white:box=1:boxcolor=black@0.65:boxborderw=16:x=(w-text_w)/2"
STYLE_KEY="fontfile=${FONT}:fontsize=36:fontcolor=#7aa2f7:box=1:boxcolor=black@0.65:boxborderw=12:x=(w-text_w)/2"

Y_DESC="y=h-120"
Y_KEY="y=h-60"

$FFMPEG -y -i "$INPUT" -vf "\
drawtext=text='<description>':${STYLE}:${Y_DESC}:enable='between(t,<start>,<end>)',\
drawtext=text='<key hint>':${STYLE_KEY}:${Y_KEY}:enable='between(t,<start>,<end>)',\
\
<... more caption pairs ...>\
\
drawtext=text='github.com/k2m30/a9s':fontfile=${FONT}:fontsize=52:fontcolor=#7aa2f7:box=1:boxcolor=black@0.75:boxborderw=20:x=(w-text_w)/2:y=(h-text_h)/2:enable='between(t,<start>,<end>)',\
drawtext=text='brew install k2m30/a9s/a9s':fontfile=${FONT}:fontsize=38:fontcolor=#9ece6a:box=1:boxcolor=black@0.75:boxborderw=16:x=(w-text_w)/2:y=(h+80)/2:enable='between(t,<start>,<end>)'\
" -codec:a copy "$OUTPUT"

echo ""
echo "Output: $OUTPUT"
ls -lh "$OUTPUT"
```

**Timestamp rules:**
- Overlay timestamps MUST match the timing trace from the tape
- Each caption pair: description line (white, 48pt, y=h-120) + key hint (blue #7aa2f7, 36pt, y=h-60)
- End card is centered vertically: `y=(h-text_h)/2` and `y=(h+80)/2`
- Escape colons in drawtext: `\:s3` not `:s3`
- NO Unicode arrows — use plain text like `Up / Down`

### Step 6: Run VHS + overlay

1. Rebuild binary: `make build`
2. Run VHS: `vhs docs/demos/social/NN-slug/demo.tape`
3. Run overlay: `bash docs/demos/social/NN-slug/add-overlays.sh`
4. Verify `final.mp4` exists and report file size
5. Clean up: delete `demo.mp4` (intermediate)

### Step 7: Report

Print a summary:

```
Post NN: <Title>
Theme: <theme-name> (dark|light)
Folder: docs/demos/social/NN-slug/
Files: post.md, demo.tape, add-overlays.sh, final.mp4
Status: ✓ complete | ⚠ light theme — manual VHS needed
Next: Post NN+1: <next title>
```

## VHS Scenario Guidelines by Content Category

### AWS Console Pain posts
- Show the a9s workflow that replaces the painful console path
- Start with the shortest path: `:resource` or `-c resource` to jump in
- Emphasize speed: fast navigation, instant results

### Feature Demo posts
- Focus on ONE feature per video
- Show the feature in context: navigate to it, use it, show the result
- Use multiple resource types if the feature is cross-cutting (e.g., `t` for CloudTrail)

### Engineering Credibility posts
- Video is secondary — can show `make test` running, or the app version, or a code grep
- Keep videos short (15-20s) with more emphasis on the post text

### Security & Trust posts
- Show the IAM policy, or grep the codebase for write calls, or show `--demo` mode
- Trust-building: show what the tool does NOT do

### Hot Takes / Opinion posts
- Video shows the a9s workflow that proves the point
- E.g., "TUI vs web" → show a9s doing in 5 seconds what the console does in 30

### Community posts
- Show `--demo` mode, `docker run`, or the contributing workflow
- Welcoming tone in post text

## Text Style Rules for post.md

- 4-10 sentences. Short paragraphs.
- Start from the problem, not the tool
- Mention a9s by name once, naturally
- No exclamation marks in the first sentence
- No "I'm excited to announce" or "Thrilled to share"
- No "check it out" — the link speaks for itself
- End with the GitHub link on its own line
- Hashtags on last line: `#AWS #DevOps #OpenSource #Go #SRE #DeveloperTools`
- Write like you're explaining something to a colleague, not selling something to a stranger

## App Behavior Notes for VHS Scripts

- `--demo` mode uses synthetic fixture data, no AWS credentials needed
- Related panel auto-shows on detail view. Do NOT press `r` (that closes it). Use `Tab` to focus the right column, `Down` to browse, `Enter` to navigate.
- Filter mode: `/` opens, Escape clears and closes
- Command mode: `:` opens, type resource name, Enter to jump
- YAML view: `y` from list or detail. Escape to close.
- JSON view: `J` from list or detail. Escape to close.
- Detail view: `d` or Enter from list. Escape to close. `w` toggles text wrap (essential for showing long truncated values).
- CloudTrail: `t` from list, detail, or YAML. Escape to close.
- Attention filter: `Ctrl+z` toggles. Shows only attention-worthy rows.
- Pagination: `M` loads more results (only works where pagination exists)
- Theme switching: `:theme` opens selector
- Profile switching: `:ctx` opens selector
- Region switching: `:region` opens selector
