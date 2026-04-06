package layout

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

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

// RenderFramePrepadded is like RenderFrame but assumes all content lines are
// already padded to innerW (w-2). This avoids the per-line lipgloss.Width()
// call, which is the most expensive operation in RenderFrame.
// Empty/missing lines are still padded with spaces to innerW.
func RenderFramePrepadded(lines []string, title string, w, h int) string {
	borderStyle := lipgloss.NewStyle().Foreground(styles.ColBorder)
	borderV := borderStyle.Render("\u2502")
	innerW := w - 2

	topBorder := CenterTitle(title, w)

	emptyPad := strings.Repeat(" ", innerW)

	var sb strings.Builder
	sb.WriteString(topBorder)

	contentRows := h - 2
	for i := 0; i < contentRows; i++ {
		sb.WriteString("\n")
		var padded string
		if i < len(lines) {
			padded = lines[i]
		} else {
			padded = emptyPad
		}

		sb.WriteString(borderV)
		sb.WriteString(padded)
		sb.WriteString(borderV)
	}

	sb.WriteString("\n")
	sb.WriteString(borderStyle.Render("\u2514" + strings.Repeat("\u2500", w-2) + "\u2518"))

	return sb.String()
}

// KeyHint is a single key+description hint for the bottom border.
type KeyHint struct {
	Key  string // e.g. "r", "d", "esc", "enter", "ctrl+r"
	Desc string // e.g. "Related", "Detail", "Back"
}

// BottomBorderWithHints renders the bottom border line with embedded key hints.
// Hints are placed left-to-right after └──. Hints that don't fit are dropped from the right.
// Empty/nil hints produce a plain └───┘ border.
// Total visual width of output equals w.
func BottomBorderWithHints(hints []KeyHint, w int) string {
	borderStyle := lipgloss.NewStyle().Foreground(styles.ColBorder)
	keyStyle := lipgloss.NewStyle().Foreground(styles.ColAccent).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(styles.ColDim)
	dashSep := borderStyle.Render("\u2500\u2500")

	if w < 4 || len(hints) == 0 {
		return borderStyle.Render("\u2514" + strings.Repeat("\u2500", w-2) + "\u2518")
	}

	// Start with └── (corner + 2 dashes)
	prefix := borderStyle.Render("\u2514\u2500\u2500")
	// Reserve 1 for └, 2 for initial ──, 1 for ┘
	usedWidth := 3 + 1 // corner(1) + 2 dashes + corner(1)

	var parts []string
	for i, hint := range hints {
		// Render the hint: "key desc"
		rendered := keyStyle.Render(hint.Key) + " " + descStyle.Render(hint.Desc)
		hintVis := lipgloss.Width(rendered)

		// Between hints: ── separator
		sepVis := 0
		if i > 0 {
			sepVis = lipgloss.Width(dashSep)
		}

		if usedWidth+sepVis+hintVis > w-1 { // w-1 to leave room for ┘
			break
		}

		if i > 0 {
			parts = append(parts, dashSep)
			usedWidth += sepVis
		}
		parts = append(parts, rendered)
		usedWidth += hintVis
	}

	// Fill remaining width with dashes, then ┘
	// usedWidth = 3 (prefix) + 1 (reserved for ┘) + hints_width
	// remaining = w - usedWidth = space for dashes only (┘ already accounted for)
	remaining := w - usedWidth
	if remaining < 0 {
		remaining = 0
	}
	suffix := borderStyle.Render(strings.Repeat("\u2500", remaining) + "\u2518")

	var sb strings.Builder
	sb.WriteString(prefix)
	for _, p := range parts {
		sb.WriteString(p)
	}
	sb.WriteString(suffix)
	return sb.String()
}

// RenderFrameWithHints is like RenderFrame but uses BottomBorderWithHints
// for the bottom border. When hints is nil/empty, output is identical to RenderFrame.
func RenderFrameWithHints(lines []string, title string, hints []KeyHint, w, h int) string {
	borderStyle := lipgloss.NewStyle().Foreground(styles.ColBorder)
	borderV := borderStyle.Render("\u2502")
	innerW := w - 2

	topBorder := CenterTitle(title, w)

	var sb strings.Builder
	sb.WriteString(topBorder)

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

	sb.WriteString("\n")
	sb.WriteString(BottomBorderWithHints(hints, w))

	return sb.String()
}

// RenderHeader produces the 1-line unframed header with optional identity info:
//
//	a9s v0.5.0  profile:region (alias) role          ? for help
//
// "a9s" is in ColAccent bold, version in ColDim, profile:region in ColHeaderFg bold.
// accountBadge and roleName are appended in dim style when non-empty and width >= 80.
// rightContent is right-aligned. Gap filled with spaces to terminal width w.
func RenderHeader(profile, region, version string, w int, rightContent, accountBadge, roleName string) string {
	accent := lipgloss.NewStyle().
		Foreground(styles.ColAccent).Bold(true).Render("a9s")
	ver := lipgloss.NewStyle().
		Foreground(styles.ColDim).Render(" v" + version)
	ctx := lipgloss.NewStyle().
		Foreground(styles.ColHeaderFg).Bold(true).
		Render("  " + profile + ":" + region)

	left := accent + ver + ctx

	dimStyle := lipgloss.NewStyle().Foreground(styles.ColDim)
	rightW := lipgloss.Width(rightContent)
	innerW := w - 2

	// Add identity parts only if they fit on one line with >=2 char gap
	if accountBadge != "" || roleName != "" {
		candidate := left
		if accountBadge != "" {
			candidate += dimStyle.Render(" (" + accountBadge + ")")
		}
		if roleName != "" {
			candidate += dimStyle.Render(" " + roleName)
		}
		if lipgloss.Width(candidate)+rightW+2 <= innerW {
			left = candidate
		}
	}

	leftW := lipgloss.Width(left)
	gap := innerW - leftW - rightW
	if gap < 1 {
		// Content too wide — truncate left side to fit
		maxLeftW := innerW - rightW - 1
		if maxLeftW < 3 {
			maxLeftW = 3
		}
		left = ansi.Truncate(left, maxLeftW, "\u2026")
		leftW = lipgloss.Width(left)
		gap = innerW - leftW - rightW
		if gap < 1 {
			gap = 1
		}
	}

	content := left + strings.Repeat(" ", gap) + rightContent
	return lipgloss.NewStyle().
		Foreground(styles.ColHeaderFg).
		Width(w).Padding(0, 1).Render(content)
}
