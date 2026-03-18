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

// RowColorStyle returns a style for a full row based on resource status.
func RowColorStyle(status string) lipgloss.Style {
	if NoColorActive() {
		return lipgloss.NewStyle()
	}
	s := strings.ToLower(status)
	switch {
	case s == "running" || s == "available" || s == "active" || s == "in-use":
		return lipgloss.NewStyle().Foreground(ColRunning)
	case s == "stopped" || s == "failed" || s == "error" || s == "deleting" || s == "deleted":
		return lipgloss.NewStyle().Foreground(ColStopped)
	case s == "pending" || s == "creating" || s == "modifying" || s == "updating":
		return lipgloss.NewStyle().Foreground(ColPending)
	case s == "terminated" || s == "shutting-down":
		return lipgloss.NewStyle().Foreground(ColTerminated)
	default:
		return lipgloss.NewStyle().Foreground(ColHeaderFg)
	}
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

	if NoColorActive() {
		return
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
