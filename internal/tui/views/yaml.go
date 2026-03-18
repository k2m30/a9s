package views

import (
	"reflect"
	"regexp"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/fieldpath"
	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/styles"

	"gopkg.in/yaml.v3"
)

// YAMLModel renders YAML with syntax coloring using bubbles/viewport for scroll.
type YAMLModel struct {
	res      resource.Resource
	viewport viewport.Model
	ready    bool
	wrap     bool
	width    int
	height   int
	keys     keys.Map
}

// NewYAML creates a YAMLModel for the given resource.
func NewYAML(res resource.Resource, k keys.Map) YAMLModel {
	return YAMLModel{
		res:  res,
		keys: k,
	}
}

// Init implements tea.Model. No async work.
func (m YAMLModel) Init() (YAMLModel, tea.Cmd) {
	return m, nil
}

// Update delegates scroll to viewport; handles c (copy), esc (back).
func (m YAMLModel) Update(msg tea.Msg) (YAMLModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.ToggleWrap):
			m.wrap = !m.wrap
			m.viewport.SoftWrap = m.wrap
			m.viewport.SetContent(m.renderContent())
			return m, nil
		}
	}

	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

// View renders YAML content via viewport.
func (m YAMLModel) View() string {
	if !m.ready {
		return "Initializing..."
	}
	return m.viewport.View()
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
	m.viewport.SetContent(m.renderContent())
}

// FrameTitle returns e.g. "i-0abc123 yaml".
func (m YAMLModel) FrameTitle() string {
	id := m.res.ID
	if m.res.Name != "" {
		id = m.res.Name
	}
	return id + " yaml"
}

// CopyContent returns the raw YAML text for clipboard copy.
func (m YAMLModel) CopyContent() (string, string) {
	content := m.RawContent()
	if content == "" {
		return "", ""
	}
	return content, "Copied YAML to clipboard"
}

// GetHelpContext returns HelpFromYAML.
func (m YAMLModel) GetHelpContext() HelpContext {
	return HelpFromYAML
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

// Regex patterns for YAML syntax coloring.
var (
	yamlKeyRe  = regexp.MustCompile(`^(\s*(?:- )?)([^\s:][^:]*):(.*)$`)
	yamlNumRe  = regexp.MustCompile(`^-?\d+(\.\d+)?$`)
	yamlBoolRe = regexp.MustCompile(`^(true|false|yes|no|Yes|No)$`)
	yamlNullRe = regexp.MustCompile(`^(null|~)$`)
)

// YAML syntax color styles — created once, not per call.
var (
	yamlKeyStyle  = lipgloss.NewStyle().Foreground(styles.ColYAMLKey)
	yamlStrStyle  = lipgloss.NewStyle().Foreground(styles.ColYAMLStr)
	yamlNumStyle  = lipgloss.NewStyle().Foreground(styles.ColYAMLNum)
	yamlBoolStyle = lipgloss.NewStyle().Foreground(styles.ColYAMLBool)
	yamlNullStyle = lipgloss.NewStyle().Foreground(styles.ColYAMLNull)
)

// colorizeYAML applies Tokyo Night syntax colors to YAML text line by line.
func colorizeYAML(raw string) string {
	keyStyle := yamlKeyStyle
	strStyle := yamlStrStyle
	numStyle := yamlNumStyle
	boolStyle := yamlBoolStyle
	nullStyle := yamlNullStyle

	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	result := make([]string, len(lines))

	for i, line := range lines {
		matches := yamlKeyRe.FindStringSubmatch(line)
		if matches != nil {
			indent := matches[1]
			keyPart := matches[2]
			valPart := strings.TrimSpace(matches[3])

			coloredLine := indent + keyStyle.Render(keyPart) + ":"
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
