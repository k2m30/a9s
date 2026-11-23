// SPDX-License-Identifier: GPL-3.0-or-later

package views

import (
	"regexp"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"charm.land/bubbles/v2/viewport"

	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// TextModel renders a YAML or JSON text screen from the controller's
// TextBody; the renderer builds one per frame (NewTransientText) and discards
// it.
type TextModel struct {
	viewport viewport.Model
	ready    bool
	width    int
	height   int
}

// ColourText applies the syntax colouring of screen id (YAML or JSON) to the
// plain lines core built. Stripping the styling gives back the same lines.
func ColourText(id runtime.ScreenID, lines []string) []string {
	if len(lines) == 0 {
		return nil
	}
	raw := strings.Join(lines, "\n")
	if id == runtime.ScreenJSON {
		return strings.Split(colorizeJSON(raw), "\n")
	}
	return strings.Split(colorizeYAML(raw), "\n")
}

// TextLines is what a YAML or JSON screen of res is seeded with when it is
// pushed; RegenerateTextScreens reseeds it with the same colouring.
func TextLines(id runtime.ScreenID, res resource.Resource) []string {
	return ColourText(id, app.ResourceTextLines(id, res))
}

// TextContent is the text RenderText hands the viewport; tests read a
// screen's content through it so they see what the terminal draws.
func TextContent(body app.TextBody) string {
	if len(body.Lines) == 0 && body.Notice != "" {
		return styles.DimText.Render("  " + body.Notice)
	}
	return strings.Join(body.Lines, "\n")
}

// RenderText renders a YAML or JSON text screen from a controller-supplied
// TextBody. The Lines in body are the syntax-coloured lines set at push time
// (TextLines). Scroll, wrap and search highlights are applied from body
// fields; width and height come from the viewport NewTransientText was given.
//
// A model not built by NewTransientText returns "Initializing...".
func (m *TextModel) RenderText(body app.TextBody) string {
	if !m.ready {
		return "Initializing..."
	}

	content := TextContent(body)

	if body.Search != "" {
		// The body already carries the match set the controller computed, in
		// display columns. Painting from it is what keeps the highlight on the
		// screen and the offsets the web lane reads describing one thing.
		plain := ansi.Strip(content)
		var sm SearchModel
		sm.active = true
		sm.query = body.Search
		sm.content = plain
		sm.setMatches(body.SearchMatches)
		if body.SearchCursor >= 0 && body.SearchCursor < len(sm.matches) {
			sm.currentIdx = body.SearchCursor
		}
		var matchLine int
		content, matchLine = sm.Apply(content)
		_ = matchLine
	}

	m.viewport.SoftWrap = body.Wrap
	m.viewport.SetContent(content)
	m.viewport.GotoTop()
	m.viewport.SetYOffset(body.ScrollY)
	return m.viewport.View()
}

// Regex patterns for YAML syntax coloring.
var (
	yamlIndentRe = regexp.MustCompile(`^(\s*(?:- )?)(.*)$`)
	yamlNumRe    = regexp.MustCompile(`^-?\d+(\.\d+)?$`)
	yamlBoolRe   = regexp.MustCompile(`^(true|false|yes|no|Yes|No)$`)
	yamlNullRe   = regexp.MustCompile(`^(null|~)$`)
)

// colorizeYAML applies Tokyo Night syntax colors to YAML text line by line.
func colorizeYAML(raw string) string {
	keyStyle := styles.YAMLKeyStyle
	strStyle := styles.YAMLStrStyle
	numStyle := styles.YAMLNumStyle
	boolStyle := styles.YAMLBoolStyle
	nullStyle := styles.YAMLNullStyle

	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	result := make([]string, len(lines))

	for i, line := range lines {
		indentMatch := yamlIndentRe.FindStringSubmatch(line)
		indent := indentMatch[1]
		rest := indentMatch[2]

		if key, val, ok := splitYAMLKeyValue(rest); ok && key != "" {
			valPart := strings.TrimSpace(val)

			coloredLine := indent + keyStyle.Render(key) + ":"
			if valPart != "" {
				coloredLine += " " + colorizeValue(valPart, strStyle, numStyle, boolStyle, nullStyle)
			}
			result[i] = coloredLine
		} else {
			// Lines without keys (list items with scalar values, etc.)
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- ") {
				// List item
				prefix := line[:len(line)-len(trimmed)]
				valStr := strings.TrimPrefix(trimmed, "- ")
				result[i] = prefix + "- " + colorizeValue(valStr, strStyle, numStyle, boolStyle, nullStyle)
			} else {
				result[i] = line
			}
		}
	}

	return strings.Join(result, "\n")
}

// splitYAMLKeyValue splits a YAML mapping line (with indent/dash already
// stripped) into its key and value parts, matching how a YAML parser locates
// the key/value separator: a colon that is quoted-delimited, or an unquoted
// colon followed by a space or end-of-line. Bare colons inside an unquoted
// key (e.g. "aws:autoscaling:groupName") are never split on, so coloring
// never alters the text a YAML parser would see — stripANSI(colorize(x)) == x.
func splitYAMLKeyValue(s string) (key, val string, ok bool) {
	if s == "" {
		return "", "", false
	}
	if s[0] == '\'' || s[0] == '"' {
		quote := s[0]
		i := 1
		for i < len(s) {
			if s[i] == quote {
				if quote == '\'' && i+1 < len(s) && s[i+1] == '\'' {
					i += 2
					continue
				}
				if quote == '"' {
					backslashes := 0
					for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
						backslashes++
					}
					if backslashes%2 == 1 {
						i++
						continue
					}
				}
				i++
				break
			}
			i++
		}
		if i < len(s) && s[i] == ':' {
			return s[:i], s[i+1:], true
		}
		return "", "", false
	}
	for i := 0; i < len(s); i++ {
		if s[i] == ':' && (i == len(s)-1 || s[i+1] == ' ') {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}

// colorizeValue applies the appropriate color to a YAML value.
func colorizeValue(val string, strStyle, numStyle, boolStyle, nullStyle lipgloss.Style) string {
	if yamlNullRe.MatchString(val) {
		return nullStyle.Render(val)
	}
	if yamlBoolRe.MatchString(val) {
		return boolStyle.Render(val)
	}
	if yamlNumRe.MatchString(val) {
		return numStyle.Render(val)
	}
	return strStyle.Render(val)
}
