package styles

import (
	"os"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// Composed styles built from the Tokyo Night Dark palette.
var (
	HeaderStyle   lipgloss.Style
	TableHeader   lipgloss.Style
	RowSelected   lipgloss.Style
	RowNormal     lipgloss.Style
	RowAlt        lipgloss.Style
	BorderNormal  lipgloss.Style
	BorderFocused lipgloss.Style
	DetailKey     lipgloss.Style
	DetailVal     lipgloss.Style
	DetailSection lipgloss.Style
	FlashSuccess  lipgloss.Style
	FlashError    lipgloss.Style
	FilterActive  lipgloss.Style
	DimText       lipgloss.Style
	SpinnerStyle  lipgloss.Style
)

// NoColorActive reports whether NO_COLOR is set in the environment.
func NoColorActive() bool {
	val, ok := os.LookupEnv("NO_COLOR")
	return ok && val != ""
}

// rowColorCache maps lowercase status strings to pre-built styles.
var rowColorCache map[string]lipgloss.Style

// RowColorStyle returns a style for a full row based on resource status.
// Uses a pre-built cache to avoid allocating new styles on every call.
func RowColorStyle(status string) lipgloss.Style {
	if NoColorActive() {
		return lipgloss.NewStyle()
	}
	if s, ok := rowColorCache[strings.ToLower(status)]; ok {
		return s
	}
	return lipgloss.NewStyle().Foreground(ColHeaderFg)
}

func init() {
	initStyles()
}

// Reinit re-initializes all composed styles. Useful for tests that toggle NO_COLOR.
func Reinit() {
	initStyles()
}

func initStyles() {
	// Reset all styles to zero values first.
	HeaderStyle = lipgloss.Style{}
	TableHeader = lipgloss.Style{}
	RowSelected = lipgloss.Style{}
	RowNormal = lipgloss.Style{}
	RowAlt = lipgloss.Style{}
	BorderNormal = lipgloss.Style{}
	BorderFocused = lipgloss.Style{}
	DetailKey = lipgloss.Style{}
	DetailVal = lipgloss.Style{}
	DetailSection = lipgloss.Style{}
	FlashSuccess = lipgloss.Style{}
	FlashError = lipgloss.Style{}
	FilterActive = lipgloss.Style{}
	DimText = lipgloss.Style{}
	SpinnerStyle = lipgloss.Style{}
	rowColorCache = nil

	if NoColorActive() {
		return
	}

	// Pre-build row color styles by status string.
	rowColorCache = map[string]lipgloss.Style{
		"running":      lipgloss.NewStyle().Foreground(ColRunning),
		"available":    lipgloss.NewStyle().Foreground(ColRunning),
		"active":       lipgloss.NewStyle().Foreground(ColRunning),
		"in-use":       lipgloss.NewStyle().Foreground(ColRunning),
		"stopped":      lipgloss.NewStyle().Foreground(ColStopped),
		"failed":       lipgloss.NewStyle().Foreground(ColStopped),
		"error":        lipgloss.NewStyle().Foreground(ColStopped),
		"deleting":     lipgloss.NewStyle().Foreground(ColStopped),
		"deleted":      lipgloss.NewStyle().Foreground(ColStopped),
		"pending":      lipgloss.NewStyle().Foreground(ColPending),
		"creating":     lipgloss.NewStyle().Foreground(ColPending),
		"modifying":    lipgloss.NewStyle().Foreground(ColPending),
		"updating":     lipgloss.NewStyle().Foreground(ColPending),
		"terminated":   lipgloss.NewStyle().Foreground(ColTerminated),
		"shutting-down": lipgloss.NewStyle().Foreground(ColTerminated),
	}

	HeaderStyle = lipgloss.NewStyle().Padding(0, 1)
	TableHeader = lipgloss.NewStyle().Foreground(ColAccent).Bold(true)
	RowSelected = lipgloss.NewStyle().Background(ColRowSelectedBg).Foreground(ColRowSelectedFg).Bold(true)
	RowNormal = lipgloss.NewStyle().Foreground(ColHeaderFg)
	RowAlt = lipgloss.NewStyle().Foreground(ColHeaderFg).Background(ColRowAltBg)
	BorderNormal = lipgloss.NewStyle().Foreground(ColBorder)
	BorderFocused = lipgloss.NewStyle().Foreground(ColAccent)
	DetailKey = lipgloss.NewStyle().Foreground(ColDetailKey)
	DetailVal = lipgloss.NewStyle().Foreground(ColDetailVal)
	DetailSection = lipgloss.NewStyle().Foreground(ColDetailSec).Bold(true)
	FlashSuccess = lipgloss.NewStyle().Foreground(ColSuccess).Bold(true)
	FlashError = lipgloss.NewStyle().Foreground(ColError).Bold(true)
	FilterActive = lipgloss.NewStyle().Foreground(ColFilter).Bold(true)
	DimText = lipgloss.NewStyle().Foreground(ColDim)
	SpinnerStyle = lipgloss.NewStyle().Foreground(ColSpinner)
}
