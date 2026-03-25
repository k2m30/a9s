# Adding Text Overlays to Demo Videos

## Prerequisites

The default `brew install ffmpeg` does **not** include the `drawtext` filter (no libfreetype). Install `ffmpeg-full`:

```sh
brew install ffmpeg-full
```

It's keg-only (won't conflict with regular ffmpeg). Use the full path:

```
/opt/homebrew/opt/ffmpeg-full/bin/ffmpeg
```

## Font

ffmpeg's default font cannot render Unicode symbols (↑ ↓ · — etc.). Specify a font file with full glyph coverage:

```
/System/Library/Fonts/Supplemental/Arial Unicode.ttf
```

## drawtext Pattern

```sh
FONT="/System/Library/Fonts/Supplemental/Arial Unicode.ttf"
STYLE="fontfile=${FONT}:fontsize=48:fontcolor=white:box=1:boxcolor=black@0.65:boxborderw=16:x=(w-text_w)/2"

/opt/homebrew/opt/ffmpeg-full/bin/ffmpeg -y -i input.mp4 -vf "\
drawtext=text='Description line':${STYLE}:y=h-120:enable='between(t,3,9)',\
drawtext=text='Key hint line':${STYLE}:y=h-60:enable='between(t,3,9)'\
" -codec:a copy output.mp4
```

Key parameters:
- `fontfile` — path to .ttf with required glyphs
- `fontsize` — size in pixels
- `fontcolor` — hex (`#7aa2f7`) or named
- `box=1:boxcolor=black@0.65:boxborderw=16` — semi-transparent background box
- `x=(w-text_w)/2` — horizontally centered
- `y=h-120` — distance from bottom
- `enable='between(t,3,9)'` — show between seconds 3 and 9

## Escaping

Colons in text need a backslash: `\:s3` not `:s3`

## VHS Outputs MP4 Directly

No need for GIF→MP4 conversion. In the .tape file:

```
Output path/to/output.mp4
```

## Working Example

See `docs/demos/social/add-overlays.sh` for a full implementation with timed overlays across a 55-second demo video.
