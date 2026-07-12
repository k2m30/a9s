# a9s Cost Explorer demo recording — loopable GIF
# Run: vhs docs/demos/costs.tape
# Requires: vhs, ./a9s binary built
# Then: bash docs/demos/add-costs-overlays.sh

Output docs/demos/costs-raw.gif

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

# ── Act 1: Open the Cost Explorer ──
# t=0
Sleep 1.5s
Type@200ms ":costs"
Sleep 1s
Enter
Sleep 4s

# ── Act 2: Cost metrics ──
# t≈7.8
Type "b"
Sleep 3s
Type "0"
Sleep 2s

# ── Act 3: Pivots ──
# t≈12.9
Type "2"
Sleep 3s
Type "3"
Sleep 3s
Type "0"
Sleep 2.5s

# ── Act 4: The anomaly — why did the bill jump? ──
# t≈21.6
Left@300ms 6
Sleep 2.5s
Enter
Sleep 3.5s
Escape
Sleep 1s

# ── Act 5: Drill to the machine behind the number ──
# t≈30.5
Right@300ms 6
Sleep 1s
Enter
Sleep 2.5s
Enter
Sleep 3s
Enter
Sleep 4s

# ── Act 6: Back out to the menu ──
# t≈44.9
Escape@600ms 3
Sleep 1.5s
Escape
Sleep 2s
