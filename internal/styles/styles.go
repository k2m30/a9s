package styles

import (
	"image/color"
	"os"

	lipgloss "charm.land/lipgloss/v2"
)

// Status color constants for resource state indicators.
var (
	StatusRunning   color.Color = lipgloss.Color("#00ff00")
	StatusStopped   color.Color = lipgloss.Color("#ff0000")
	StatusPending   color.Color = lipgloss.Color("#ffff00")
	StatusAvailable color.Color = lipgloss.Color("#00ff00")
)

// Style variables used throughout the application.
var (
	HeaderStyle      lipgloss.Style
	BreadcrumbStyle  lipgloss.Style
	TableCursorStyle lipgloss.Style
	TableHeaderStyle lipgloss.Style
	StatusBarStyle   lipgloss.Style
	ErrorStyle       lipgloss.Style
	SpinnerStyle     lipgloss.Style
)

func init() {
	InitStyles()
}

// InitStyles initializes all application styles. It checks the NO_COLOR
// environment variable and adjusts accordingly.
func InitStyles() {
	noColor := os.Getenv("NO_COLOR") != ""

	if noColor {
		// NO_COLOR means no ANSI escape codes at all — no colors,
		// no bold, no reverse, no faint. Plain text only.
		HeaderStyle = lipgloss.NewStyle().Padding(0, 1)
		BreadcrumbStyle = lipgloss.NewStyle()
		TableCursorStyle = lipgloss.NewStyle()
		TableHeaderStyle = lipgloss.NewStyle()
		StatusBarStyle = lipgloss.NewStyle()
		ErrorStyle = lipgloss.NewStyle()
		SpinnerStyle = lipgloss.NewStyle()

		StatusRunning = lipgloss.NoColor{}
		StatusStopped = lipgloss.NoColor{}
		StatusPending = lipgloss.NoColor{}
		StatusAvailable = lipgloss.NoColor{}
	} else {
		HeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Bold(true).
			Padding(0, 1)

		BreadcrumbStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Faint(true)

		TableCursorStyle = lipgloss.NewStyle().
			Reverse(true)

		TableHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Bold(true)

		StatusBarStyle = lipgloss.NewStyle().
			Faint(true)

		ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff0000"))

		SpinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00ffff"))

		StatusRunning = lipgloss.Color("#00ff00")
		StatusStopped = lipgloss.Color("#ff0000")
		StatusPending = lipgloss.Color("#ffff00")
		StatusAvailable = lipgloss.Color("#00ff00")
	}
}
