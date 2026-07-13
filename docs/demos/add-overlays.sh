#!/bin/bash
# Add text overlays to demo GIF
# Usage: bash docs/demos/add-overlays.sh
# Requires: ffmpeg-full (brew install ffmpeg-full)
#
# Timing trace from demo.tape (calculated):
#
# Act 1: Main Menu
#   0.0  Show → Sleep 2s → ends 2.0
#   2.0  Down×3 @300ms → ends 2.9
#   2.9  Sleep 3s → ends 5.9
#
# Act 1b: Issues at a glance
#   5.9  Ctrl+Z → Sleep 3s → ends 8.95
#   8.95 Ctrl+Z → Sleep 1.5s → ends 10.5
#
# Act 2: EC2 List
#  10.5  Up×3 @300ms → ends 11.4
#  11.4  Enter → Sleep 2.5s → ends 13.9
#  13.9  Down×2 @300ms → ends 14.5
#  14.5  Sleep 1s → ends 15.5
#  15.5  Type@300ms "/web" (4×300ms=1.2s) → ends 16.7
#  16.7  Sleep 3s → ends 19.7
#  19.7  Escape → Sleep 3s → ends 22.7
#
# Act 3: Detail + YAML
#  22.7  Type "d" → Sleep 3s → ends 25.75
#  25.75 Down×3 @200ms → ends 26.35
#  26.35 Sleep 1s → ends 27.35
#  27.35 Type "y" → Sleep 3s → ends 30.4
#  30.4  Escape×3 @500ms → ends 31.9
#  31.9  Sleep 3s → ends 34.9
#
# Act 4: Related Views (panel auto-shows on detail)
#  34.9  Enter → Sleep 2s → ends 36.9
#  36.9  Down → Sleep 300ms → ends 37.2
#  37.2  Type "d" → Sleep 3s → ends 40.25
#  40.25 Tab → Sleep 1s → ends 41.25
#  41.25 Down×3 @400ms → ends 42.45
#  42.45 Sleep 2s → ends 44.45
#  44.45 Enter → Sleep 3s → ends 47.45
#  47.45 Escape×3 @500ms → ends 48.95
#  48.95 Sleep 3s → ends 51.95
#
# Act 5: S3 Drill-Down
#  51.95 Type@200ms ":s3" (3×200ms=0.6s) → ends 52.55
#  52.55 Sleep 1s → ends 53.55
#  53.55 Enter → Sleep 2.5s → ends 56.05
#  56.05 Enter → Sleep 2.5s → ends 58.55
#  58.55 Escape×2 @500ms → ends 59.55
#  59.55 Sleep 3s → ends 62.55
#
# Act 6: Quick Tour
#  62.55 Type@200ms ":lambda" (7×200ms=1.4s) → ends 63.95
#  63.95 Sleep 1s → ends 64.95
#  64.95 Enter → Sleep 3s → ends 67.95
#  67.95 Type@200ms ":rds" (4×200ms=0.8s) → ends 68.75
#  68.75 Sleep 1s → ends 69.75
#  69.75 Enter → Sleep 3s → ends 72.75
#  72.75 Escape → Sleep 3s → ends 75.75

INPUT="docs/demos/demo-raw.gif"
OUTPUT="/tmp/demo-annotated.gif"
FFMPEG="/opt/homebrew/opt/ffmpeg-full/bin/ffmpeg"
FONT="/System/Library/Fonts/Supplemental/Arial Unicode.ttf"

STYLE="fontfile=${FONT}:fontsize=44:fontcolor=white:box=1:boxcolor=black@0.7:boxborderw=12:x=(w-text_w)/2"
STYLE_KEY="fontfile=${FONT}:fontsize=32:fontcolor=#7aa2f7:box=1:boxcolor=black@0.7:boxborderw=10:x=(w-text_w)/2"

Y_DESC="y=h-100"
Y_KEY="y=h-48"

OVERLAYS="\
drawtext=text='66 resource types + Cost Explorer':${STYLE}:${Y_DESC}:enable='between(t,0.5,5.5)',\
drawtext=text='Up / Down  navigate':${STYLE_KEY}:${Y_KEY}:enable='between(t,0.5,5.5)',\
\
drawtext=text='Spot issues instantly':${STYLE}:${Y_DESC}:enable='between(t,6.2,8.8)',\
drawtext=text='Ctrl+Z  show only resources with findings':${STYLE_KEY}:${Y_KEY}:enable='between(t,6.2,8.8)',\
\
drawtext=text='EC2 Instances':${STYLE}:${Y_DESC}:enable='between(t,11.6,15.1)',\
drawtext=text='Enter  open resource list':${STYLE_KEY}:${Y_KEY}:enable='between(t,11.6,15.1)',\
\
drawtext=text='Filter resources instantly':${STYLE}:${Y_DESC}:enable='between(t,15.6,22.1)',\
drawtext=text='/  search  |  Esc  clear':${STYLE_KEY}:${Y_KEY}:enable='between(t,15.6,22.1)',\
\
drawtext=text='Detail View -- all fields':${STYLE}:${Y_DESC}:enable='between(t,23.1,27.1)',\
drawtext=text='d  detail  |  Down  scroll':${STYLE_KEY}:${Y_KEY}:enable='between(t,23.1,27.1)',\
\
drawtext=text='Full YAML -- raw AWS API response':${STYLE}:${Y_DESC}:enable='between(t,27.6,31.6)',\
drawtext=text='y  yaml view':${STYLE_KEY}:${Y_KEY}:enable='between(t,27.6,31.6)',\
\
drawtext=text='Related Resources':${STYLE}:${Y_DESC}:enable='between(t,37.6,44.1)',\
drawtext=text='Tab  focus  |  Down  browse':${STYLE_KEY}:${Y_KEY}:enable='between(t,37.6,44.1)',\
\
drawtext=text='Navigate to related resource':${STYLE}:${Y_DESC}:enable='between(t,44.6,47.3)',\
drawtext=text='Enter  jump to resource':${STYLE_KEY}:${Y_KEY}:enable='between(t,44.6,47.3)',\
\
drawtext=text='S3 Buckets':${STYLE}:${Y_DESC}:enable='between(t,52.6,55.6)',\
drawtext=text='\:s3  jump to any service':${STYLE_KEY}:${Y_KEY}:enable='between(t,52.6,55.6)',\
\
drawtext=text='Drill into bucket objects':${STYLE}:${Y_DESC}:enable='between(t,56.1,59.1)',\
drawtext=text='Enter  child view':${STYLE_KEY}:${Y_KEY}:enable='between(t,56.1,59.1)',\
\
drawtext=text='Lambda Functions':${STYLE}:${Y_DESC}:enable='between(t,62.6,67.6)',\
drawtext=text='\:lambda  jump to service':${STYLE_KEY}:${Y_KEY}:enable='between(t,62.6,67.6)',\
\
drawtext=text='RDS Databases':${STYLE}:${Y_DESC}:enable='between(t,68.1,72.6)',\
drawtext=text='\:rds  jump to service':${STYLE_KEY}:${Y_KEY}:enable='between(t,68.1,72.6)'"

# Two-pass GIF encoding
$FFMPEG -y -i "$INPUT" -vf "${OVERLAYS},fps=10,palettegen=stats_mode=diff" /tmp/demo_palette.png

$FFMPEG -y -i "$INPUT" -i /tmp/demo_palette.png -lavfi "[0:v]${OVERLAYS},fps=10[v];[v][1:v]paletteuse=dither=floyd_steinberg" "$OUTPUT"

# Copy final result to repo
cp "$OUTPUT" "docs/demos/demo.gif"

echo ""
echo "Output: docs/demos/demo.gif"
ls -lh "docs/demos/demo.gif"
