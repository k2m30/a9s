package ui

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/internal/styles"
)

// RenderHeader renders the top header line showing app name, version, profile,
// and region. If loading is true, a spinner character is appended at the right.
func RenderHeader(appName, version, profile, region string, loading bool, width int) string {
	left := fmt.Sprintf("%s v%s | profile: %s | %s", appName, version, profile, region)

	if loading {
		spinner := " \u28BE" // braille spinner character ⣾
		if width > 0 {
			// Place spinner at the right edge: pad the left text so spinner lands at the end
			available := width - lipgloss.Width(spinner)
			if available > lipgloss.Width(left) {
				left = left + strings.Repeat(" ", available-lipgloss.Width(left)) + spinner
			} else {
				left = left + spinner
			}
			return styles.HeaderStyle.Width(width).Render(left)
		}
		return styles.HeaderStyle.Render(left + spinner)
	}

	if width > 0 {
		return styles.HeaderStyle.Width(width).Render(left)
	}
	return styles.HeaderStyle.Render(left)
}
