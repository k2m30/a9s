// SPDX-License-Identifier: GPL-3.0-or-later

package views

import (
	"regexp"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// Regex patterns for JSON syntax coloring.
var (
	jsonKeyValueRe = regexp.MustCompile(`^(\s*)("([^"]+)")\s*:(.*)$`)
	jsonStringRe   = regexp.MustCompile(`^"[^"]*"$`)
	jsonNumRe      = regexp.MustCompile(`^-?\d+(\.\d+)?([eE][+-]?\d+)?$`)
	jsonBoolRe     = regexp.MustCompile(`^(true|false)$`)
	jsonNullRe     = regexp.MustCompile(`^null$`)
)

// colorizeJSON applies Tokyo Night syntax colors to JSON text line by line.
func colorizeJSON(raw string) string {
	keyStyle := styles.YAMLKeyStyle
	strStyle := styles.YAMLStrStyle
	numStyle := styles.YAMLNumStyle
	boolStyle := styles.YAMLBoolStyle
	nullStyle := styles.YAMLNullStyle

	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	result := make([]string, len(lines))

	for i, line := range lines {
		matches := jsonKeyValueRe.FindStringSubmatch(line)
		if matches != nil {
			indent := matches[1]
			keyPart := matches[2] // includes quotes
			valPart := strings.TrimSpace(matches[4])

			coloredLine := indent + keyStyle.Render(keyPart) + ":"
			if valPart != "" {
				coloredLine += " " + colorizeJSONValue(valPart, strStyle, numStyle, boolStyle, nullStyle)
			}
			result[i] = coloredLine
		} else {
			// Value-only lines (array elements, closing brackets, etc.)
			trimmed := strings.TrimSpace(line)
			// Strip trailing comma for matching, re-add after coloring
			val := strings.TrimSuffix(trimmed, ",")
			suffix := ""
			if strings.HasSuffix(trimmed, ",") {
				suffix = ","
			}
			colored := colorizeJSONValue(val, strStyle, numStyle, boolStyle, nullStyle)
			if colored != val {
				// It was colorized — rebuild with original indent
				indent := line[:len(line)-len(trimmed)]
				result[i] = indent + colored + suffix
			} else {
				result[i] = line
			}
		}
	}

	return strings.Join(result, "\n")
}

// colorizeJSONValue applies the appropriate color to a JSON value token.
// The value may have a trailing comma which is preserved uncolored.
func colorizeJSONValue(val string, strStyle, numStyle, boolStyle, nullStyle lipgloss.Style) string {
	// Strip trailing comma for matching
	suffix := ""
	if strings.HasSuffix(val, ",") {
		suffix = ","
		val = strings.TrimSuffix(val, ",")
	}

	var colored string
	switch {
	case jsonNullRe.MatchString(val):
		colored = nullStyle.Render(val)
	case jsonBoolRe.MatchString(val):
		colored = boolStyle.Render(val)
	case jsonNumRe.MatchString(val):
		colored = numStyle.Render(val)
	case jsonStringRe.MatchString(val):
		colored = strStyle.Render(val)
	default:
		// Structural tokens ({, }, [, ]) — leave uncolored
		return val + suffix
	}
	return colored + suffix
}
