package views

import (
	"reflect"
	"regexp"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"

	"gopkg.in/yaml.v3"
)

// YAMLModel renders YAML with syntax coloring using bubbles/viewport for scroll.
type YAMLModel struct {
	res          resource.Resource
	resourceType string
	viewport     viewport.Model
	ready        bool
	wrap         bool
	width        int
	height       int
	keys         keys.Map
	search       SearchModel
	rawText      string // non-empty = raw text mode (no YAML marshaling)
	rawTitle     string // frame title for raw text mode
}

// NewYAML creates a YAMLModel for the given resource.
func NewYAML(res resource.Resource, resourceType string, k keys.Map) YAMLModel {
	return YAMLModel{
		res:          res,
		resourceType: resourceType,
		keys:         k,
	}
}

// NewTextViewer creates a read-only text viewer using the YAML viewport infrastructure.
func NewTextViewer(title, content string, k keys.Map) YAMLModel {
	return YAMLModel{
		rawText:  content,
		rawTitle: title,
		keys:     k,
	}
}

// IsTextViewer reports whether this YAMLModel is in raw-text mode (e.g. error log).
func (m YAMLModel) IsTextViewer() bool {
	return m.rawText != ""
}

// Init implements tea.Model. No async work.
func (m YAMLModel) Init() (YAMLModel, tea.Cmd) {
	return m, nil
}

// Update delegates scroll to viewport; handles c (copy), esc (back).
func (m YAMLModel) Update(msg tea.Msg) (YAMLModel, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.EnrichDetailResult:
		// Accept enriched resource when type and ID match.
		if msg.ResourceType != m.resourceType || msg.ResourceID != m.res.ID {
			return m, nil
		}
		m.res = msg.EnrichedRes
		m.refreshViewportContent()
		return m, nil
	case tea.PasteMsg:
		if m.search.IsInputMode() {
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.refreshViewportContent()
			return m, cmd
		}
	case searchPasteMsg:
		if m.search.IsInputMode() {
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.refreshViewportContent()
			return m, cmd
		}
	case tea.KeyMsg:
		// Search input mode captures all keys.
		if m.search.IsInputMode() {
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.refreshViewportContent()
			return m, cmd
		}
		switch {
		case key.Matches(msg, m.keys.Search):
			m.search.Activate()
			return m, nil
		case key.Matches(msg, m.keys.SearchNext):
			if m.search.IsActive() && m.search.MatchCount() > 0 {
				m.search.NextMatch()
				m.refreshViewportContent()
				return m, nil
			}
		case key.Matches(msg, m.keys.SearchPrev):
			if m.search.IsActive() && m.search.MatchCount() > 0 {
				m.search.PrevMatch()
				m.refreshViewportContent()
				return m, nil
			}
		case key.Matches(msg, m.keys.Escape):
			if m.search.IsActive() {
				m.search.Deactivate()
				m.refreshViewportContent()
				return m, nil
			}
		case key.Matches(msg, m.keys.ToggleWrap):
			m.wrap = !m.wrap
			m.viewport.SoftWrap = m.wrap
			m.refreshViewportContent()
			return m, nil
		case key.Matches(msg, m.keys.CloudTrail):
			if ff := resource.BuildCloudTrailFilter(m.res, m.resourceType); ff != nil {
				res := m.res
				return m, func() tea.Msg {
					return messages.RelatedNavigate{
						TargetType:     "ct-events",
						SourceResource: res,
						SourceType:     m.resourceType,
						FetchFilter:    ff,
					}
				}
			}
			return m, nil
		case key.Matches(msg, m.keys.Describe):
			if m.rawText != "" {
				return m, nil
			}
			res := m.res
			return m, func() tea.Msg {
				return messages.Navigate{
					Target:         messages.TargetDetail,
					Resource:       &res,
					ResourceType:   m.resourceType,
					ReplaceCurrent: true,
				}
			}
		case key.Matches(msg, m.keys.JSON):
			if m.rawText != "" {
				return m, nil
			}
			res := m.res
			return m, func() tea.Msg {
				return messages.Navigate{
					Target:         messages.TargetJSON,
					Resource:       &res,
					ResourceType:   m.resourceType,
					ReplaceCurrent: true,
				}
			}
		case key.Matches(msg, m.keys.YAML):
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

// IsSearchActive returns true when search is active (input mode or confirmed highlights).
func (m YAMLModel) IsSearchActive() bool { return m.search.IsActive() }

// IsSearchInputMode returns true when the search input is capturing keystrokes.
func (m YAMLModel) IsSearchInputMode() bool { return m.search.IsInputMode() }

// SearchInfo returns the search state string for the header.
// Input mode: "/query" (or "/" when query is empty), Confirmed: "N/M matches", Inactive: "".
func (m YAMLModel) SearchInfo() string {
	if !m.search.IsActive() {
		return ""
	}
	if m.search.IsInputMode() {
		q := m.search.Query()
		return "/" + q
	}
	return m.search.MatchInfo()
}

// FrameTitle returns e.g. "i-0abc123 yaml".
func (m YAMLModel) FrameTitle() string {
	if m.rawTitle != "" {
		return m.rawTitle
	}
	id := m.res.ID
	if m.res.Name != "" {
		id = m.res.Name
	}
	return id + " yaml"
}

// BottomHints implements Hintable for YAMLModel.
func (m YAMLModel) BottomHints() []layout.KeyHint {
	if m.rawText != "" {
		return []layout.KeyHint{
			{Key: "w", Desc: "Wrap"},
			{Key: "c", Desc: "Copy"},
		}
	}
	hints := []layout.KeyHint{
		{Key: "w", Desc: "Wrap"},
		{Key: "c", Desc: "Copy"},
	}
	if resource.BuildCloudTrailFilter(m.res, m.resourceType) != nil {
		hints = append(hints, layout.KeyHint{Key: "t", Desc: "CloudTrail"})
	}
	return hints
}

// CopyContent returns the raw YAML text for clipboard copy.
func (m YAMLModel) CopyContent() (string, string) {
	if m.rawText != "" {
		return m.rawText, "Copied to clipboard"
	}
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
	if m.rawText != "" {
		return m.rawText
	}
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
	if m.rawText != "" {
		return m.rawText
	}
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
