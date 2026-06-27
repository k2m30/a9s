package unit

import (
	"regexp"

	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// ansiRe is kept for direct call sites in search tests that use it inline.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI removes ANSI escape sequences from a string for plain-text comparison.
func stripANSI(s string) string {
	return tuitest.StripANSI(s)
}

// lipglossWidth measures the visible width of a string (ANSI-aware).
func lipglossWidth(s string) int {
	return tuitest.Width(s)
}
