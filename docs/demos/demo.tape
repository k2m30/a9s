# a9s demo recording — loopable GIF
# Run: vhs docs/demos/demo.tape
# Requires: vhs, ./a9s binary built

Output docs/demos/demo.gif

Set Shell bash
Set FontSize 18
Set Width 1200
Set Height 500
Set Padding 10
Set TypingSpeed 500ms
Set Framerate 5
Set Theme "TokyoNight"

# Launch a9s in demo mode (hidden)
Hide
Type "./a9s --demo"
Enter
Sleep 1s
Show

# ── Act 1: Main Menu ──
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms

# ── Act 2: EC2 List View ──
Up
Sleep 300ms
Up
Sleep 300ms
Up
Sleep 300ms

Enter
Sleep 2s

# Browse rows
Down
Sleep 300ms
Down
Sleep 600ms

# Filter by "web"
Type "/web"
Sleep 2s

Escape
Sleep 800ms

# ── Act 3: EC2 Detail + YAML ──
Type "d"
Sleep 2s

# Scroll detail
Down
Sleep 200ms
Down
Sleep 200ms
Down
Sleep 800ms

# YAML view
Type "y"
Sleep 2s

# Back: YAML → detail → list → menu
Escape
Sleep 500ms
Escape
Sleep 500ms
Escape
Sleep 800ms

# ── Act 4: S3 Drill-Down ──
Type ":s3"
Enter
Sleep 2s

# Enter first bucket
Enter
Sleep 2s

# Back: objects → buckets → menu
Escape
Sleep 500ms
Escape
Sleep 800ms

# ── Act 5: Quick Tour ──
Type ":lambda"
Enter
Sleep 2s

Type ":rds"
Enter
Sleep 2s

# Back to main menu (loop point)
Escape
Sleep 2s
