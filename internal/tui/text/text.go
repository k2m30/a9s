package text

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	lipgloss "charm.land/lipgloss/v2"
)

// PadOrTrunc pads s to exactly w visible columns, or truncates with "...".
// Uses a fast path for plain ASCII strings (no ANSI escapes) to avoid
// lipgloss.Width overhead. Falls back to ANSI-aware measurement otherwise.
func PadOrTrunc(s string, w int) string {
	if w <= 0 {
		return ""
	}
	// Fast path: pure ASCII without ANSI escapes -> len(s) == visible width.
	// Any multi-byte rune (len != rune count) or ANSI escape falls through.
	if len(s) == len([]rune(s)) && !strings.Contains(s, "\x1b") {
		if len(s) == w {
			return s
		}
		if len(s) > w {
			return s[:w-1] + "\u2026"
		}
		return s + strings.Repeat(" ", w-len(s))
	}
	// Slow path: ANSI-aware measurement
	visible := lipgloss.Width(s)
	if visible == w {
		return s
	}
	if visible > w {
		return ansi.Truncate(s, w, "\u2026")
	}
	return s + strings.Repeat(" ", w-visible)
}
