package layout

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/internal/tui/styles"
)

// PadOrTrunc pads s to exactly w visible columns, or truncates with "…".
// Uses a fast path for plain ASCII strings (no ANSI escapes) to avoid
// lipgloss.Width overhead. Falls back to ANSI-aware measurement otherwise.
func PadOrTrunc(s string, w int) string {
	if w <= 0 {
		return ""
	}
	// Fast path: no ANSI escapes → len(s) == visible width for ASCII
	if !strings.Contains(s, "\x1b") {
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

// CenterTitle returns the top border line with title centered between corners.
//
//	┌─── title ───┐
//
// Empty title produces a plain top border: ┌──────┐
// Title too long is truncated to fit.
func CenterTitle(title string, w int) string {
	borderStyle := lipgloss.NewStyle().Foreground(styles.ColBorder)

	if title == "" {
		return borderStyle.Render("\u250c" + strings.Repeat("\u2500", w-2) + "\u2510")
	}

	titleRendered := lipgloss.NewStyle().Foreground(styles.ColAccent).Bold(true).Render(title)
	titleVis := lipgloss.Width(titleRendered)

	// Layout: ┌ + leftDashes + " " + title + " " + rightDashes + ┐
	// totalDashes = (w - 2) - titleVis - 2  (minus corners, minus spaces around title)
	totalDashes := w - 2 - titleVis - 2
	if totalDashes < 2 {
		// Title too long — truncate it to fit
		maxTitleVis := w - 2 - 2 - 2 // corners(2) + spaces(2) + min 2 dashes
		if maxTitleVis < 1 {
			// Extremely narrow — just do plain border
			return borderStyle.Render("\u250c" + strings.Repeat("\u2500", w-2) + "\u2510")
		}
		titleRendered = ansi.Truncate(titleRendered, maxTitleVis, "\u2026")
		titleVis = lipgloss.Width(titleRendered)
		totalDashes = w - 2 - titleVis - 2
	}

	leftDashes := totalDashes / 2
	rightDashes := totalDashes - leftDashes

	prefix := "\u250c" + strings.Repeat("\u2500", leftDashes) + " "
	suffix := " " + strings.Repeat("\u2500", rightDashes) + "\u2510"

	return borderStyle.Render(prefix) + titleRendered + borderStyle.Render(suffix)
}

// RenderFrame produces the complete framed box:
//
//	┌─────── title ───────┐
//	│ content             │
//	└─────────────────────┘
//
// w is total width including border characters.
// h is total height including top and bottom borders.
// lines are pre-rendered content rows.
// If fewer content lines than h-2, pad with empty lines.
func RenderFrame(lines []string, title string, w, h int) string {
	borderStyle := lipgloss.NewStyle().Foreground(styles.ColBorder)
	borderV := borderStyle.Render("\u2502") // render once, reuse for all rows
	innerW := w - 2                         // space between left │ and right │

	// Top border with centered title
	topBorder := CenterTitle(title, w)

	var sb strings.Builder
	sb.WriteString(topBorder)

	// Content lines: h-2 rows (minus top and bottom borders)
	contentRows := h - 2
	for i := 0; i < contentRows; i++ {
		sb.WriteString("\n")
		var content string
		if i < len(lines) {
			content = lines[i]
		}

		visW := lipgloss.Width(content)
		var padded string
		if visW < innerW {
			padded = content + strings.Repeat(" ", innerW-visW)
		} else {
			padded = content
		}

		sb.WriteString(borderV)
		sb.WriteString(padded)
		sb.WriteString(borderV)
	}

	// Bottom border
	sb.WriteString("\n")
	sb.WriteString(borderStyle.Render("\u2514" + strings.Repeat("\u2500", w-2) + "\u2518"))

	return sb.String()
}

// RenderHeader produces the 1-line unframed header:
//
//	a9s v0.5.0  profile:region                       ? for help
//
// "a9s" is in ColAccent bold, version in ColDim, profile:region in ColHeaderFg bold.
// rightContent is right-aligned. Gap filled with spaces to terminal width w.
func RenderHeader(profile, region, version string, w int, rightContent string) string {
	accent := lipgloss.NewStyle().
		Foreground(styles.ColAccent).Bold(true).Render("a9s")
	ver := lipgloss.NewStyle().
		Foreground(styles.ColDim).Render(" v" + version)
	ctx := lipgloss.NewStyle().
		Foreground(styles.ColHeaderFg).Bold(true).
		Render("  " + profile + ":" + region)

	left := accent + ver + ctx
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(rightContent)

	// padding: 1 char on each side (via Padding(0,1))
	innerW := w - 2
	gap := innerW - leftW - rightW
	if gap < 1 {
		gap = 1
	}

	content := left + strings.Repeat(" ", gap) + rightContent
	return lipgloss.NewStyle().
		Foreground(styles.ColHeaderFg).
		Width(w).Padding(0, 1).Render(content)
}
