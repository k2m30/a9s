# VHS demo script for a9s
# Run: vhs docs/demos/demo.tape
# Requires: vhs (brew install vhs), a9s binary, valid AWS credentials

Output assets/demo.gif
Set Theme "Tokyo Night"
Set FontSize 14
Set Width 1400
Set Height 800
Set Padding 20
Set TypingSpeed 80ms
Set Shell zsh

# Launch a9s
Type "a9s"
Enter
Sleep 3s

# Navigate through menu categories
Down 3
Sleep 500ms
Down 2
Sleep 500ms

# Select a resource type
Enter
Sleep 2s

# Scroll through resource list
Down
Sleep 300ms
Down
Sleep 300ms
Down
Sleep 300ms

# Open YAML detail view
Enter
Sleep 2s

# Scroll in detail view
Down 5
Sleep 1s

# Go back
Escape
Sleep 1s

# Switch region
Type "R"
Sleep 2s

# Go back to menu
Escape
Sleep 1s

# Show help
Type "?"
Sleep 2s

# Close help
Escape
Sleep 1s
