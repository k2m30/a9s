// SPDX-License-Identifier: GPL-3.0-or-later

package views

import (
	"encoding/json"
	"regexp"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"charm.land/bubbles/v2/viewport"

	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// JSONModel renders JSON with syntax coloring using bubbles/viewport for scroll.
// ctrl is non-nil when the model is constructed by the TUI navigator; when
// ctrl is set, View() delegates to RenderText(ctrl.Snapshot().Body.Text) so
// the headless/web renderer and the TUI renderer share one code path.
// Unit tests and isolated callers leave ctrl nil; View() falls back to the
// direct viewport path in that case so parity tests remain unaffected.
type JSONModel struct {
	ctrl         *app.Controller
	res          resource.Resource
	resourceType string
	viewport     viewport.Model
	ready        bool
	width        int
	height       int
	keys         keys.Map
	search       SearchModel
}

// NewJSONWithCtrl creates a JSONModel backed by the given controller.
// The controller stack must already have ScreenJSON pushed and EnsureTextState
// called before the first View() so that Snapshot().Body.Text is non-nil.
func NewJSONWithCtrl(res resource.Resource, resourceType string, k keys.Map, ctrl *app.Controller) JSONModel {
	return JSONModel{
		ctrl:         ctrl,
		res:          res,
		resourceType: resourceType,
		keys:         k,
	}
}

// SetSize initializes or resizes the viewport.
func (m *JSONModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	if !m.ready {
		m.viewport = viewport.New(viewport.WithWidth(w), viewport.WithHeight(h))
		m.ready = true
	} else {
		m.viewport.SetWidth(w)
		m.viewport.SetHeight(h)
	}
	m.refreshViewportContent()
}

// refreshViewportContent re-renders content and applies search highlights.
func (m *JSONModel) refreshViewportContent() {
	content := m.renderContent()
	if m.search.IsActive() && m.search.Query() != "" {
		plain := ansi.Strip(content)
		m.search.SetContent(plain)
		var matchLine int
		content, matchLine = m.search.Apply(content)
		if matchLine >= 0 {
			m.viewport.GotoTop()
			m.viewport.SetYOffset(matchLine)
		}
	}
	m.viewport.SetContent(content)
}

// ContentLines returns the syntax-colored JSON content as a slice of lines,
// matching exactly what refreshViewportContent passes to the viewport.
// Used by the TUI navigator at push time to seed EnsureTextState.
func (m JSONModel) ContentLines() []string {
	content := m.renderContent()
	return strings.Split(content, "\n")
}

// renderContent marshals the resource to JSON and applies syntax coloring.
func (m JSONModel) renderContent() string {
	var data []byte
	var err error

	if m.res.RawStruct != nil {
		data, err = json.MarshalIndent(m.res.RawStruct, "", "  ")
	} else if len(m.res.Fields) > 0 {
		data, err = json.MarshalIndent(m.res.Fields, "", "  ")
	}

	if err != nil || len(data) == 0 {
		return styles.DimText.Render("  No JSON data available")
	}

	return colorizeJSON(string(data))
}

// RenderText renders a JSON text screen from a controller-supplied TextBody,
// byte-identical to View() for the same logical state. The Lines in body are
// the syntax-colored content strings set at push time. Scroll, wrap, and
// search highlights are applied from body fields; width and height come from
// the model's viewport (set by SetSize).
//
// If the model is not yet ready (SetSize not called), returns "Initializing..."
// matching View()'s pre-ready behaviour.
func (m *JSONModel) RenderText(body app.TextBody) string {
	if !m.ready {
		return "Initializing..."
	}

	content := strings.Join(body.Lines, "\n")

	if body.Search != "" {
		plain := ansi.Strip(content)
		var sm SearchModel
		sm.active = true
		sm.SetQuery(body.Search)
		sm.SetContent(plain)
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
