package unit

import (
	"regexp"

	lipgloss "charm.land/lipgloss/v2"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI removes ANSI escape sequences from a string for plain-text comparison.
func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// lipglossWidth measures the visible width of a string (ANSI-aware).
func lipglossWidth(s string) int {
	return lipgloss.Width(s)
}
