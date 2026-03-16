package ui

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/internal/styles"
)

// RenderHeader renders the top header line showing app name, profile, and region
// on the left, with the version on the right. If loading is true, a spinner
// character is appended.
func RenderHeader(appName, version, profile, region string, loading bool, width int) string {
	left := fmt.Sprintf("%s | profile: %s | %s", appName, profile, region)
	if loading {
		left += " [loading...]"
	}
	right := fmt.Sprintf("v%s", version)

	if width > 0 {
		leftLen := lipgloss.Width(left)
		rightLen := lipgloss.Width(right)
		padding := width - leftLen - rightLen - 2 // 2 for Padding(0,1)
		if padding < 1 {
			padding = 1
		}
		headerText := left + strings.Repeat(" ", padding) + right
		return styles.HeaderStyle.Width(width).Render(headerText)
	}
	return styles.HeaderStyle.Render(left + "  " + right)
}
