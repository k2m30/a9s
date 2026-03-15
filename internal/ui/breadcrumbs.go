package ui

import (
	"strings"

	"github.com/k2m30/a9s/internal/styles"
)

// RenderBreadcrumbs renders a breadcrumb navigation line from the given
// segments, joined with " > " separators. The result fills the full width.
func RenderBreadcrumbs(segments []string, width int) string {
	crumbs := strings.Join(segments, " \u203A ")
	if width > 0 {
		return styles.BreadcrumbStyle.Width(width).Render(crumbs)
	}
	return styles.BreadcrumbStyle.Render(crumbs)
}
