package views

import (
	gocolor "image/color"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

// ctEventsLegend renders the CloudTrail Events glyph + row-tint + cell-color legend
// per design doc §8a. Called only when context is HelpFromResourceList* and
// resourceShortName is "ct-events".
func (m HelpModel) ctEventsLegend() string {
	catStyle := styles.HelpCatStyle
	descStyle := styles.HelpDescStyle

	verbStyle := func(col gocolor.Color, bold bool) lipgloss.Style {
		s := lipgloss.NewStyle().Foreground(col)
		if bold {
			s = s.Bold(true)
		}
		return s
	}

	var sb strings.Builder

	// Header.
	sb.WriteString(" " + catStyle.Render("CloudTrail Events Legend"))
	sb.WriteString("\n\n")

	// --- Verb glyphs ---
	sb.WriteString(" " + catStyle.Render("VERB GLYPHS"))
	sb.WriteString("\n")
	verbRows := []struct {
		glyph string
		style lipgloss.Style
		desc  string
	}{
		{"R", verbStyle(styles.ColTerminated, false), "Read  (Describe*, Get*, List*, Head*)"},
		{"W", verbStyle(styles.ColPending, true), "Write (Create*, Put*, Update*, Attach*)"},
		{"D", verbStyle(styles.ColStopped, true), "Destructive (Delete*, Terminate*, Revoke*)"},
		{"S", verbStyle(styles.ColTerminated, false), "Service event (eventType=AwsServiceEvent)"},
		{"I", verbStyle(styles.ColTerminated, false), "Insight event (eventCategory=Insight)"},
		{"N", verbStyle(styles.ColTerminated, false), "NetworkActivity (eventCategory=NetworkActivity)"},
		{"?", verbStyle(styles.ColTerminated, false), "Ambiguous (no classifier match)"},
	}
	for _, row := range verbRows {
		glyph := row.style.Render(text.PadOrTrunc(row.glyph, 3))
		sb.WriteString(" " + glyph + descStyle.Render(row.desc) + "\n")
	}

	sb.WriteString("\n")

	// --- Row tints ---
	sb.WriteString(" " + catStyle.Render("SEVERITY TIERS"))
	sb.WriteString("\n")
	tintRows := []struct {
		label string
		col   gocolor.Color
		desc  string
	}{
		{"ct-info", styles.ColTerminated, "routine reads — normal-volume noise"},
		{"ct-attention", styles.ColPending, "worth a glance — writes, ROOT, sensitive reads, cross-account"},
		{"ct-danger", styles.ColStopped, "worth investigating — destructive ops or failures"},
	}
	for _, row := range tintRows {
		label := lipgloss.NewStyle().Foreground(row.col).Render(text.PadOrTrunc(row.label, 14))
		sb.WriteString(" " + label + descStyle.Render(row.desc) + "\n")
	}

	return sb.String()
}
