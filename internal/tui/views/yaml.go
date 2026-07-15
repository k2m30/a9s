package views

import (
	"reflect"
	"regexp"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"charm.land/bubbles/v2/viewport"

	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"

	"gopkg.in/yaml.v3"
)

// YAMLModel renders YAML with syntax coloring using bubbles/viewport for scroll.
// ctrl is non-nil when the model is constructed by the TUI navigator; when
// ctrl is set, View() delegates to RenderText(ctrl.Snapshot().Body.Text) so
// the headless/web renderer and the TUI renderer share one code path.
// Unit tests and isolated callers leave ctrl nil; View() falls back to the
// direct viewport path in that case so parity tests remain unaffected.
type YAMLModel struct {
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

// NewYAMLWithCtrl creates a YAMLModel backed by the given controller.
// The controller stack must already have ScreenYAML pushed and EnsureTextState
// called before the first View() so that Snapshot().Body.Text is non-nil.
func NewYAMLWithCtrl(res resource.Resource, resourceType string, k keys.Map, ctrl *app.Controller) YAMLModel {
	return YAMLModel{
		ctrl:         ctrl,
		res:          res,
		resourceType: resourceType,
		keys:         k,
	}
}

// SetSize initializes or resizes the viewport.
func (m *YAMLModel) SetSize(w, h int) {
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
func (m *YAMLModel) refreshViewportContent() {
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

// FrameTitle returns e.g. "i-0abc123 yaml".
func (m YAMLModel) FrameTitle() string {
	id := m.res.ID
	if m.res.Name != "" {
		id = m.res.Name
	}
	return id + " yaml"
}

// ContentLines returns the syntax-colored YAML content as a slice of lines,
// matching exactly what refreshViewportContent passes to the viewport.
// Used by the TUI navigator at push time to seed EnsureTextState.
func (m YAMLModel) ContentLines() []string {
	content := m.renderContent()
	return strings.Split(content, "\n")
}

// RawContent returns the uncolored YAML text for clipboard copy.
func (m YAMLModel) RawContent() string {
	var data []byte
	var err error

	if m.res.RawStruct != nil {
		safe := fieldpath.ToSafeValue(reflect.ValueOf(m.res.RawStruct))
		data, err = yaml.Marshal(safe)
	} else if len(m.res.Fields) > 0 {
		data, err = yaml.Marshal(m.res.Fields)
	}

	if err != nil || len(data) == 0 {
		return ""
	}
	return string(data)
}

// ResourceID returns the resource ID for clipboard copy.
func (m YAMLModel) ResourceID() string {
	return m.res.ID
}

// renderContent marshals the resource to YAML and applies syntax coloring.
func (m YAMLModel) renderContent() string {
	var data []byte
	var err error

	if m.res.RawStruct != nil {
		safe := fieldpath.ToSafeValue(reflect.ValueOf(m.res.RawStruct))
		data, err = yaml.Marshal(safe)
	} else if len(m.res.Fields) > 0 {
		data, err = yaml.Marshal(m.res.Fields)
	}

	if err != nil || len(data) == 0 {
		return styles.DimText.Render("  No YAML data available")
	}

	return colorizeYAML(string(data))
}

// RenderText renders a YAML text screen from a controller-supplied TextBody,
// byte-identical to View() for the same logical state. The Lines in body are
// the syntax-colored content strings set at push time. Scroll, wrap, and
// search highlights are applied from body fields; width and height come from
// the model's viewport (set by SetSize).
//
// If the model is not yet ready (SetSize not called), returns "Initializing..."
// matching View()'s pre-ready behaviour.
func (m *YAMLModel) RenderText(body app.TextBody) string {
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
