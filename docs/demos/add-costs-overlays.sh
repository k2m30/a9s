#!/bin/bash
# Add text overlays to the Cost Explorer demo GIF
# Usage: bash docs/demos/add-costs-overlays.sh
# Requires: ffmpeg-full (brew install ffmpeg-full)
#
# Timing trace from costs.tape (calculated):
#
# Act 1: Open the Cost Explorer
#   0.0   Show → Sleep 1.5s → ends 1.5
#   1.5   Type@200ms ":costs" (6×200ms=1.2s) → ends 2.7
#   2.7   Sleep 1s → ends 3.7
#   3.7   Enter → Sleep 4s → ends 7.75
#
# Act 2: Cost metrics
#   7.75  Type "b" → Sleep 3s → ends 10.8
#  10.8   Type "0" → Sleep 2s → ends 12.85
#
# Act 3: Pivots
#  12.85  Type "2" → Sleep 3s → ends 15.9
#  15.9   Type "3" → Sleep 3s → ends 18.95
#  18.95  Type "0" → Sleep 2.5s → ends 21.5
#
# Act 4: The anomaly
#  21.5   Left×6 @300ms (1.8s) → ends 23.3
#  23.3   Sleep 2.5s → ends 25.8
#  25.8   Enter → Sleep 3.5s → ends 29.35
#  29.35  Escape → Sleep 1s → ends 30.4
#
# Act 5: Drill to the machine
#  30.4   Right×6 @300ms (1.8s) → ends 32.2
#  32.2   Sleep 1s → ends 33.2
#  33.2   Enter → Sleep 2.5s → ends 35.75
#  35.75  Enter → Sleep 3s → ends 38.8
#  38.8   Enter → Sleep 4s → ends 42.85
#
# Act 6: Back out
#  42.85  Escape×3 @600ms (1.8s) → ends 44.65
#  44.65  Sleep 1.5s → ends 46.15
#  46.15  Escape → Sleep 2s → ends 48.2

INPUT="docs/demos/costs-raw.gif"
OUTPUT="/tmp/costs-annotated.gif"
FFMPEG="/opt/homebrew/opt/ffmpeg-full/bin/ffmpeg"
FONT="/System/Library/Fonts/Supplemental/Arial Unicode.ttf"

STYLE="fontfile=${FONT}:fontsize=44:fontcolor=white:box=1:boxcolor=black@0.7:boxborderw=12:x=(w-text_w)/2"
STYLE_KEY="fontfile=${FONT}:fontsize=32:fontcolor=#7aa2f7:box=1:boxcolor=black@0.7:boxborderw=10:x=(w-text_w)/2"

Y_DESC="y=h-100"
Y_KEY="y=h-48"

OVERLAYS="\
drawtext=text='Your AWS bill as a grid -- matches the invoice':${STYLE}:${Y_DESC}:enable='between(t,0.5,7.3)',\
drawtext=text='\:costs  open Cost Explorer':${STYLE_KEY}:${Y_KEY}:enable='between(t,0.5,7.3)',\
\
drawtext=text='Cycle cost metrics':${STYLE}:${Y_DESC}:enable='between(t,8,12.4)',\
drawtext=text='b  invoice | unblended | amortized':${STYLE_KEY}:${Y_KEY}:enable='between(t,8,12.4)',\
\
drawtext=text='Pivot by region or account':${STYLE}:${Y_DESC}:enable='between(t,13.2,21)',\
drawtext=text='2-6  pivots  |  0  reset':${STYLE_KEY}:${Y_KEY}:enable='between(t,13.2,21)',\
\
drawtext=text='Anomaly -- root cause in the footer':${STYLE}:${Y_DESC}:enable='between(t,23.5,25.7)',\
drawtext=text='Left  move to the spike':${STYLE_KEY}:${Y_KEY}:enable='between(t,23.5,25.7)',\
\
drawtext=text='Why? The usage type behind the jump':${STYLE}:${Y_DESC}:enable='between(t,26.2,29.2)',\
drawtext=text='Enter  drill into the cell':${STYLE_KEY}:${Y_KEY}:enable='between(t,26.2,29.2)',\
\
drawtext=text='Drill the current month to resources':${STYLE}:${Y_DESC}:enable='between(t,33.5,38.6)',\
drawtext=text='Enter  usage types, then instances':${STYLE_KEY}:${Y_KEY}:enable='between(t,33.5,38.6)',\
\
drawtext=text='The machine behind the number':${STYLE}:${Y_DESC}:enable='between(t,39.2,42.7)',\
drawtext=text='Enter  open the EC2 detail view':${STYLE_KEY}:${Y_KEY}:enable='between(t,39.2,42.7)',\
\
drawtext=text='Back to the grid, back to the menu':${STYLE}:${Y_DESC}:enable='between(t,45,47.9)',\
drawtext=text='Esc  back':${STYLE_KEY}:${Y_KEY}:enable='between(t,45,47.9)'"

# Two-pass GIF encoding
$FFMPEG -y -i "$INPUT" -vf "${OVERLAYS},fps=10,palettegen=stats_mode=diff" /tmp/costs_palette.png

$FFMPEG -y -i "$INPUT" -i /tmp/costs_palette.png -lavfi "[0:v]${OVERLAYS},fps=10[v];[v][1:v]paletteuse=dither=floyd_steinberg" "$OUTPUT"

# Copy final result to repo
cp "$OUTPUT" "docs/demos/costs.gif"

echo ""
echo "Output: docs/demos/costs.gif"
ls -lh "docs/demos/costs.gif"
