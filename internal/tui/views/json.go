package views

import (
	"encoding/json"
	"regexp"
	"strings"

	lipgloss "charm.land/lipgloss/v2"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// JSONModel renders JSON with syntax coloring using bubbles/viewport for scroll.
type JSONModel struct {
	res          resource.Resource
	resourceType string
	viewport     viewport.Model
	ready        bool
	wrap         bool
	width        int
	height       int
	keys         keys.Map
	search       SearchModel
}

// NewJSON creates a JSONModel for the given resource.
func NewJSON(res resource.Resource, resourceType string, k keys.Map) JSONModel {
	return JSONModel{
		res:          res,
		resourceType: resourceType,
		keys:         k,
	}
}

// Init implements tea.Model. No async work.
func (m JSONModel) Init() (JSONModel, tea.Cmd) {
	return m, nil
}

// Update delegates scroll to viewport; handles c (copy), esc (back).
func (m JSONModel) Update(msg tea.Msg) (JSONModel, tea.Cmd) {
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
			res := m.res
			return m, func() tea.Msg {
				return messages.Navigate{
					Target:         messages.TargetDetail,
					Resource:       &res,
					ResourceType:   m.resourceType,
					ReplaceCurrent: true,
				}
			}
		case key.Matches(msg, m.keys.YAML):
			res := m.res
			return m, func() tea.Msg {
				return messages.Navigate{
					Target:         messages.TargetYAML,
					Resource:       &res,
					ResourceType:   m.resourceType,
					ReplaceCurrent: true,
				}
			}
		case key.Matches(msg, m.keys.JSON):
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

// View renders JSON content via viewport.
func (m JSONModel) View() string {
	if !m.ready {
		return "Initializing..."
	}
	return m.viewport.View()
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

// IsSearchActive returns true when search is active (input mode or confirmed highlights).
func (m JSONModel) IsSearchActive() bool { return m.search.IsActive() }

// IsSearchInputMode returns true when the search input is capturing keystrokes.
func (m JSONModel) IsSearchInputMode() bool { return m.search.IsInputMode() }

// SearchInfo returns the search state string for the header.
// Input mode: "/query" (or "/" when query is empty), Confirmed: "N/M matches", Inactive: "".
func (m JSONModel) SearchInfo() string {
	if !m.search.IsActive() {
		return ""
	}
	if m.search.IsInputMode() {
		q := m.search.Query()
		return "/" + q
	}
	return m.search.MatchInfo()
}

// FrameTitle returns e.g. "i-0abc123 json".
func (m JSONModel) FrameTitle() string {
	id := m.res.ID
	if m.res.Name != "" {
		id = m.res.Name
	}
	return id + " json"
}

// BottomHints implements Hintable for JSONModel.
func (m JSONModel) BottomHints() []layout.KeyHint {
	hints := []layout.KeyHint{
		{Key: "w", Desc: "Wrap"},
		{Key: "c", Desc: "Copy"},
	}
	if resource.BuildCloudTrailFilter(m.res, m.resourceType) != nil {
		hints = append(hints, layout.KeyHint{Key: "t", Desc: "CloudTrail"})
	}
	return hints
}

// CopyContent returns the raw JSON text for clipboard copy.
func (m JSONModel) CopyContent() (string, string) {
	content := m.RawContent()
	if content == "" {
		return "", ""
	}
	return content, "Copied JSON to clipboard"
}

// GetHelpContext returns HelpFromJSON.
func (m JSONModel) GetHelpContext() HelpContext {
	return HelpFromJSON
}

// RawContent returns the uncolored JSON text for clipboard copy.
func (m JSONModel) RawContent() string {
	var data []byte
	var err error

	if m.res.RawStruct != nil {
		data, err = json.MarshalIndent(m.res.RawStruct, "", "  ")
	} else if len(m.res.Fields) > 0 {
		data, err = json.MarshalIndent(m.res.Fields, "", "  ")
	}

	if err != nil || len(data) == 0 {
		return ""
	}
	return string(data)
}

// ResourceID returns the resource ID for clipboard copy.
func (m JSONModel) ResourceID() string {
	return m.res.ID
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
