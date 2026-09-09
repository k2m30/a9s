// SPDX-License-Identifier: GPL-3.0-or-later

package text

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/domain"
)

// Width returns the number of terminal columns s occupies. It is the measure
// PadOrTrunc pads and truncates to, exported so that a caller reserving room
// for a string cannot disagree with the renderer that fills it: a rune-count
// reservation is half the room a CJK title needs.
func Width(s string) int {
	return domain.Width(s)
}

// unpaintable reports whether r occupies no terminal column yet survives as a
// byte through PadOrTrunc's ASCII fast path — every C0 control and DEL except
// the ESC that introduces an ANSI sequence.
func unpaintable(r rune) bool {
	return (r < 0x20 && r != 0x1b) || r == 0x7f
}

// PadOrTrunc pads s to exactly w visible columns, or truncates with "...".
// Uses a fast path for plain ASCII strings (no ANSI escapes) to avoid
// lipgloss.Width overhead. Falls back to ANSI-aware measurement otherwise.
func PadOrTrunc(s string, w int) string {
	if w <= 0 {
		return ""
	}
	// A control character the terminal does not paint breaks a fixed-width
	// cell twice over: a newline puts the rest of the row on a second line,
	// and every other one is a byte the ASCII path below counts as a column
	// that is never drawn, so the cell comes up short and the column beside
	// it slides left. ESC is the exception — it opens the styling sequences
	// the measure already reads.
	if strings.ContainsFunc(s, unpaintable) {
		s = strings.ReplaceAll(s, "\r\n", "\n")
		s = strings.Map(func(r rune) rune {
			if unpaintable(r) {
				return ' '
			}
			return r
		}, s)
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
	visible := Width(s)
	if visible == w {
		return s
	}
	if visible > w {
		// A cut that lands inside a double-width rune cannot keep that rune,
		// so the truncation comes back a column short of what the caller
		// reserved. Filling the gap here is what lets one reservation serve
		// both sides: every caller gets the w columns it asked for, and the
		// cell to its right starts where its header says it does.
		s = ansi.Truncate(s, w, "\u2026")
		// A cluster the cut cannot split — an emoji built from several runes,
		// a mark that combines with the one before it — can leave the result
		// no narrower than the cell. There is nothing to pad then, and
		// trimming further would break the cluster the truncation kept whole.
		if visible = Width(s); visible > w {
			s = ansi.Truncate(s, w-1, "\u2026")
			visible = Width(s)
		}
		if visible >= w {
			return s
		}
	}
	return s + strings.Repeat(" ", w-visible)
}
