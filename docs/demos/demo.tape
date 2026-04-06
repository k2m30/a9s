# a9s demo recording — loopable GIF
# Run: vhs docs/demos/demo.tape
# Requires: vhs, ./a9s binary built
# Then: bash docs/demos/add-overlays.sh

Output docs/demos/demo-raw.gif

Set Shell bash
Set FontSize 18
Set Width 1200
Set Height 500
Set Padding 10
Set TypingSpeed 50ms
Set Framerate 3
Set Theme "TokyoNight"

# Launch a9s in demo mode (hidden)
Hide
Type "./a9s --demo"
Enter
Sleep 1s
Show

# ── Act 1: Main Menu ──
# t=0
Sleep 2s
Down@300ms 3
Sleep 3s

# ── Act 2: EC2 List ──
# t≈6
Up@300ms 3
Enter
Sleep 2.5s
Down@300ms 2
Sleep 1s
Type@300ms "/web"
Sleep 3s
Escape
Sleep 3s

# ── Act 3: Detail + YAML ──
# t≈17
Type "d"
Sleep 3s
Down@200ms 3
Sleep 1s
Type "y"
Sleep 3s
Escape@500ms 3
Sleep 3s

# ── Act 4: Related Views ──
# t≈29
# Related panel auto-shows on detail view. No need to press r.
Enter
Sleep 2s
Down
Sleep 300ms
Type "d"
Sleep 3s
Tab
Sleep 1s
Down@400ms 3
Sleep 2s
Enter
Sleep 3s
Escape@500ms 3
Sleep 3s

# ── Act 5: S3 Drill-Down ──
# t≈48
Type@200ms ":s3"
Sleep 1s
Enter
Sleep 2.5s
Enter
Sleep 2.5s
Escape@500ms 2
Sleep 3s

# ── Act 6: Quick Tour ──
# t≈58
Type@200ms ":lambda"
Sleep 1s
Enter
Sleep 3s
Type@200ms ":rds"
Sleep 1s
Enter
Sleep 3s
Escape
Sleep 3s
