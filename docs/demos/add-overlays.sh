#!/bin/bash
# Add text overlays to demo GIF
# Usage: bash docs/demos/add-overlays.sh
# Adjust timestamps if VHS output timing drifts

INPUT="docs/demos/demo.gif"
OUTPUT="docs/demos/demo-annotated.gif"
FFMPEG="/opt/homebrew/opt/ffmpeg-full/bin/ffmpeg"
FONT="/System/Library/Fonts/Supplemental/Arial Unicode.ttf"

# Text style: bottom-center, semi-transparent box
# GIF is 1200x500 — use smaller fonts than the MP4 version
STYLE="fontfile=${FONT}:fontsize=22:fontcolor=white:box=1:boxcolor=black@0.7:boxborderw=8:x=(w-text_w)/2"
STYLE_KEY="fontfile=${FONT}:fontsize=16:fontcolor=#7aa2f7:box=1:boxcolor=black@0.7:boxborderw=6:x=(w-text_w)/2"

# Y positions (from bottom)
Y_DESC="y=h-56"
Y_KEY="y=h-28"

# Two-pass GIF encoding for quality
# Pass 1: generate palette
$FFMPEG -y -i "$INPUT" -vf "\
drawtext=text='62 AWS resource types with live counts':${STYLE}:${Y_DESC}:enable='between(t,0.5,4)',\
drawtext=text='↑ ↓  navigate':${STYLE_KEY}:${Y_KEY}:enable='between(t,0.5,4)',\
\
drawtext=text='EC2 Instances':${STYLE}:${Y_DESC}:enable='between(t,5,9.5)',\
drawtext=text='Enter  open resource list':${STYLE_KEY}:${Y_KEY}:enable='between(t,5,9.5)',\
\
drawtext=text='Filter resources instantly':${STYLE}:${Y_DESC}:enable='between(t,10,13)',\
drawtext=text='/  search · Esc  clear':${STYLE_KEY}:${Y_KEY}:enable='between(t,10,13)',\
\
drawtext=text='Detail View — all instance fields':${STYLE}:${Y_DESC}:enable='between(t,14.5,18.5)',\
drawtext=text='d  detail · ↓  scroll':${STYLE_KEY}:${Y_KEY}:enable='between(t,14.5,18.5)',\
\
drawtext=text='Full YAML — raw AWS API response':${STYLE}:${Y_DESC}:enable='between(t,19.5,23)',\
drawtext=text='y  yaml view':${STYLE_KEY}:${Y_KEY}:enable='between(t,19.5,23)',\
\
drawtext=text='S3 Buckets':${STYLE}:${Y_DESC}:enable='between(t,27.5,30)',\
drawtext=text='\:s3  jump to any service':${STYLE_KEY}:${Y_KEY}:enable='between(t,27.5,30)',\
\
drawtext=text='Drill into bucket objects':${STYLE}:${Y_DESC}:enable='between(t,30,33)',\
drawtext=text='Enter  child view':${STYLE_KEY}:${Y_KEY}:enable='between(t,30,33)',\
\
drawtext=text='Lambda Functions':${STYLE}:${Y_DESC}:enable='between(t,38,40.5)',\
drawtext=text='\:lambda  jump to service':${STYLE_KEY}:${Y_KEY}:enable='between(t,38,40.5)',\
\
drawtext=text='RDS Databases':${STYLE}:${Y_DESC}:enable='between(t,43,46)',\
drawtext=text='\:rds  jump to service':${STYLE_KEY}:${Y_KEY}:enable='between(t,43,46)',\
palettegen=stats_mode=diff" /tmp/demo_palette.png

# Pass 2: apply palette
$FFMPEG -y -i "$INPUT" -i /tmp/demo_palette.png -lavfi "\
[0:v]drawtext=text='62 AWS resource types with live counts':${STYLE}:${Y_DESC}:enable='between(t,0.5,4)',\
drawtext=text='↑ ↓  navigate':${STYLE_KEY}:${Y_KEY}:enable='between(t,0.5,4)',\
\
drawtext=text='EC2 Instances':${STYLE}:${Y_DESC}:enable='between(t,5,9.5)',\
drawtext=text='Enter  open resource list':${STYLE_KEY}:${Y_KEY}:enable='between(t,5,9.5)',\
\
drawtext=text='Filter resources instantly':${STYLE}:${Y_DESC}:enable='between(t,10,13)',\
drawtext=text='/  search · Esc  clear':${STYLE_KEY}:${Y_KEY}:enable='between(t,10,13)',\
\
drawtext=text='Detail View — all instance fields':${STYLE}:${Y_DESC}:enable='between(t,14.5,18.5)',\
drawtext=text='d  detail · ↓  scroll':${STYLE_KEY}:${Y_KEY}:enable='between(t,14.5,18.5)',\
\
drawtext=text='Full YAML — raw AWS API response':${STYLE}:${Y_DESC}:enable='between(t,19.5,23)',\
drawtext=text='y  yaml view':${STYLE_KEY}:${Y_KEY}:enable='between(t,19.5,23)',\
\
drawtext=text='S3 Buckets':${STYLE}:${Y_DESC}:enable='between(t,27.5,30)',\
drawtext=text='\:s3  jump to any service':${STYLE_KEY}:${Y_KEY}:enable='between(t,27.5,30)',\
\
drawtext=text='Drill into bucket objects':${STYLE}:${Y_DESC}:enable='between(t,30,33)',\
drawtext=text='Enter  child view':${STYLE_KEY}:${Y_KEY}:enable='between(t,30,33)',\
\
drawtext=text='Lambda Functions':${STYLE}:${Y_DESC}:enable='between(t,38,40.5)',\
drawtext=text='\:lambda  jump to service':${STYLE_KEY}:${Y_KEY}:enable='between(t,38,40.5)',\
\
drawtext=text='RDS Databases':${STYLE}:${Y_DESC}:enable='between(t,43,46)',\
drawtext=text='\:rds  jump to service':${STYLE_KEY}:${Y_KEY}:enable='between(t,43,46)'\
[v];[v][1:v]paletteuse=dither=floyd_steinberg" "$OUTPUT"

echo ""
echo "Output: $OUTPUT"
ls -lh "$OUTPUT"
