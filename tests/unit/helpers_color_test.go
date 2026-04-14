package unit

import (
	"image/color"
	"strings"
)

// colorsEqual compares two color.Color values by their RGBA components.
func colorsEqual(a, b color.Color) bool {
	r1, g1, b1, a1 := a.RGBA()
	r2, g2, b2, a2 := b.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}

// findLineContaining returns the first line of view that contains needle, or "".
func findLineContaining(view, needle string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}
