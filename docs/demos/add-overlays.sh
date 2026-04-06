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
# Act 2: EC2 List
#   5.9  Up×3 @300ms → ends 6.8
#   6.8  Enter → Sleep 2.5s → ends 9.3
#   9.3  Down×2 @300ms → ends 9.9
#   9.9  Sleep 1s → ends 10.9
#  10.9  Type@300ms "/web" (4×300ms=1.2s) → ends 12.1
#  12.1  Sleep 3s → ends 15.1
#  15.1  Escape → Sleep 3s → ends 18.1
#
# Act 3: Detail + YAML
#  18.1  Type "d" → Sleep 3s → ends 21.15
#  21.15 Down×3 @200ms → ends 21.75
#  21.75 Sleep 1s → ends 22.75
#  22.75 Type "y" → Sleep 3s → ends 25.8
#  25.8  Escape×3 @500ms → ends 27.3
#  27.3  Sleep 3s → ends 30.3
#
# Act 4: Related Views (panel auto-shows on detail)
#  30.3  Enter → Sleep 2s → ends 32.3
#  32.3  Down → Sleep 300ms → ends 32.6
#  32.6  Type "d" → Sleep 3s → ends 35.65
#  35.65 Tab → Sleep 1s → ends 36.65
#  36.65 Down×3 @400ms → ends 37.85
#  37.85 Sleep 2s → ends 39.85
#  39.85 Enter → Sleep 3s → ends 42.85
#  42.85 Escape×3 @500ms → ends 44.35
#  44.35 Sleep 3s → ends 47.35
#
# Act 5: S3 Drill-Down
#  47.35 Type@200ms ":s3" (3×200ms=0.6s) → ends 47.95
#  47.95 Sleep 1s → ends 48.95
#  48.95 Enter → Sleep 2.5s → ends 51.45
#  51.45 Enter → Sleep 2.5s → ends 53.95
#  53.95 Escape×2 @500ms → ends 54.95
#  54.95 Sleep 3s → ends 57.95
#
# Act 6: Quick Tour
#  57.95 Type@200ms ":lambda" (7×200ms=1.4s) → ends 59.35
#  59.35 Sleep 1s → ends 60.35
#  60.35 Enter → Sleep 3s → ends 63.35
#  63.35 Type@200ms ":rds" (4×200ms=0.8s) → ends 64.15
#  64.15 Sleep 1s → ends 65.15
#  65.15 Enter → Sleep 3s → ends 68.15
#  68.15 Escape → Sleep 3s → ends 71.15

INPUT="docs/demos/demo-raw.gif"
OUTPUT="/tmp/demo-annotated.gif"
FFMPEG="/opt/homebrew/opt/ffmpeg-full/bin/ffmpeg"
FONT="/System/Library/Fonts/Supplemental/Arial Unicode.ttf"

STYLE="fontfile=${FONT}:fontsize=44:fontcolor=white:box=1:boxcolor=black@0.7:boxborderw=12:x=(w-text_w)/2"
STYLE_KEY="fontfile=${FONT}:fontsize=32:fontcolor=#7aa2f7:box=1:boxcolor=black@0.7:boxborderw=10:x=(w-text_w)/2"

Y_DESC="y=h-100"
Y_KEY="y=h-48"

OVERLAYS="\
drawtext=text='66 AWS resource types':${STYLE}:${Y_DESC}:enable='between(t,0.5,5.5)',\
drawtext=text='Up / Down  navigate':${STYLE_KEY}:${Y_KEY}:enable='between(t,0.5,5.5)',\
\
drawtext=text='EC2 Instances':${STYLE}:${Y_DESC}:enable='between(t,7,10.5)',\
drawtext=text='Enter  open resource list':${STYLE_KEY}:${Y_KEY}:enable='between(t,7,10.5)',\
\
drawtext=text='Filter resources instantly':${STYLE}:${Y_DESC}:enable='between(t,11,17.5)',\
drawtext=text='/  search  |  Esc  clear':${STYLE_KEY}:${Y_KEY}:enable='between(t,11,17.5)',\
\
drawtext=text='Detail View -- all fields':${STYLE}:${Y_DESC}:enable='between(t,18.5,22.5)',\
drawtext=text='d  detail  |  Down  scroll':${STYLE_KEY}:${Y_KEY}:enable='between(t,18.5,22.5)',\
\
drawtext=text='Full YAML -- raw AWS API response':${STYLE}:${Y_DESC}:enable='between(t,23,27)',\
drawtext=text='y  yaml view':${STYLE_KEY}:${Y_KEY}:enable='between(t,23,27)',\
\
drawtext=text='Related Resources':${STYLE}:${Y_DESC}:enable='between(t,33,39.5)',\
drawtext=text='Tab  focus  |  Down  browse':${STYLE_KEY}:${Y_KEY}:enable='between(t,33,39.5)',\
\
drawtext=text='Navigate to related resource':${STYLE}:${Y_DESC}:enable='between(t,40,44)',\
drawtext=text='Enter  jump to resource':${STYLE_KEY}:${Y_KEY}:enable='between(t,40,44)',\
\
drawtext=text='S3 Buckets':${STYLE}:${Y_DESC}:enable='between(t,48,51)',\
drawtext=text='\:s3  jump to any service':${STYLE_KEY}:${Y_KEY}:enable='between(t,48,51)',\
\
drawtext=text='Drill into bucket objects':${STYLE}:${Y_DESC}:enable='between(t,51.5,54.5)',\
drawtext=text='Enter  child view':${STYLE_KEY}:${Y_KEY}:enable='between(t,51.5,54.5)',\
\
drawtext=text='Lambda Functions':${STYLE}:${Y_DESC}:enable='between(t,58,63)',\
drawtext=text='\:lambda  jump to service':${STYLE_KEY}:${Y_KEY}:enable='between(t,58,63)',\
\
drawtext=text='RDS Databases':${STYLE}:${Y_DESC}:enable='between(t,63.5,68)',\
drawtext=text='\:rds  jump to service':${STYLE_KEY}:${Y_KEY}:enable='between(t,63.5,68)'"

# Two-pass GIF encoding
$FFMPEG -y -i "$INPUT" -vf "${OVERLAYS},fps=10,palettegen=stats_mode=diff" /tmp/demo_palette.png

$FFMPEG -y -i "$INPUT" -i /tmp/demo_palette.png -lavfi "[0:v]${OVERLAYS},fps=10[v];[v][1:v]paletteuse=dither=floyd_steinberg" "$OUTPUT"

# Copy final result to repo
cp "$OUTPUT" "docs/demos/demo.gif"

echo ""
echo "Output: docs/demos/demo.gif"
ls -lh "docs/demos/demo.gif"
