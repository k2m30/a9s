package views

import (
	"reflect"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"

	"gopkg.in/yaml.v3"
)

// DetailModel renders the key-value describe view using bubbles/viewport for scroll.
type DetailModel struct {
	res          resource.Resource
	resourceType string // e.g. "ec2", "s3", "rds" — used to look up correct ViewDef
	viewConfig   *config.ViewsConfig
	viewport     viewport.Model
	ready        bool
	wrap         bool
	width        int
	height       int
	keys         keys.Map
}

// NewDetail creates a DetailModel for the given resource.
// resourceType identifies which ViewDef to use from the config (e.g. "ec2", "rds").
func NewDetail(res resource.Resource, resourceType string, viewConfig *config.ViewsConfig, k keys.Map) DetailModel {
	return DetailModel{
		resourceType: resourceType,
		res:        res,
		viewConfig: viewConfig,
		keys:       k,
	}
}

// Init implements tea.Model. No async work.
func (m DetailModel) Init() (DetailModel, tea.Cmd) {
	return m, nil
}

// Update delegates scroll to viewport; handles y (yaml), c (copy), esc (back).
func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.YAML):
			return m, func() tea.Msg {
				return messages.NavigateMsg{
					Target:   messages.TargetYAML,
					Resource: &m.res,
				}
			}
		case key.Matches(msg, m.keys.ToggleWrap):
			m.wrap = !m.wrap
			m.viewport.SoftWrap = m.wrap
			m.viewport.SetContent(m.renderContent())
			return m, nil
		}
	}

	// Delegate to viewport for scroll
	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

// View renders detail content via viewport.
func (m DetailModel) View() string {
	if !m.ready {
		return "Initializing..."
	}
	return m.viewport.View()
}

// SetSize initializes or resizes the viewport. Must be called before View().
func (m *DetailModel) SetSize(w, h int) {
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

// FrameTitle returns the resource identifier.
func (m DetailModel) FrameTitle() string {
	if m.res.Name != "" {
		return m.res.Name
	}
	return m.res.ID
}

// CopyContent returns the resource as YAML for clipboard copy.
func (m DetailModel) CopyContent() (string, string) {
	content := m.RawYAML()
	if content == "" {
		return "", ""
	}
	return content, "Copied YAML to clipboard"
}

// GetHelpContext returns HelpFromDetail.
func (m DetailModel) GetHelpContext() HelpContext {
	return HelpFromDetail
}

// ResourceID returns the resource ID for clipboard copy.
func (m DetailModel) ResourceID() string {
	return m.res.ID
}

// RawYAML returns the resource as YAML for clipboard copy (same format as YAML view).
func (m DetailModel) RawYAML() string {
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

// PlainContent returns the detail content as plain text (no ANSI) for clipboard copy.
func (m DetailModel) PlainContent() string {
	content := m.renderContent()
	// Strip ANSI escape codes
	result := make([]byte, 0, len(content))
	i := 0
	for i < len(content) {
		if content[i] == '\x1b' && i+1 < len(content) && content[i+1] == '[' {
			// Skip until we hit a letter
			j := i + 2
			for j < len(content) && (content[j] < 'a' || content[j] > 'z') && (content[j] < 'A' || content[j] > 'Z') {
				j++
			}
			if j < len(content) {
				j++ // skip the letter
			}
			i = j
		} else {
			result = append(result, content[i])
			i++
		}
	}
	return string(result)
}

// renderContent builds the styled key-value lines from the resource.
func (m DetailModel) renderContent() string {
	const keyW = 22

	kv := func(key, val string) string {
		return " " + styles.DetailKey.Render(text.PadOrTrunc(key+":", keyW)) + styles.DetailVal.Render(val)
	}

	// Try config-driven rendering.
	if m.viewConfig != nil {
		if lines := m.renderFromConfig(kv); len(lines) > 0 {
			return strings.Join(lines, "\n")
		}
	}

	// Fallback: render from Fields map (no config or no matching ViewDef).
	if len(m.res.Fields) == 0 {
		return styles.DimText.Render("  No detail data available")
	}

	// Sort keys for stable output.
	fieldKeys := make([]string, 0, len(m.res.Fields))
	for k := range m.res.Fields {
		fieldKeys = append(fieldKeys, k)
	}
	sort.Strings(fieldKeys)

	var lines []string
	for _, k := range fieldKeys {
		lines = append(lines, kv(k, m.res.Fields[k]))
	}
	return strings.Join(lines, "\n")
}

// renderFromConfig looks up the correct ViewDef by resource type and renders detail lines.
// Tries RawStruct extraction first, then falls back to Fields map for each path.
// Empty/nil fields are shown as "-" (not skipped).
func (m DetailModel) renderFromConfig(kv func(string, string) string) []string {
	vd := config.GetViewDef(m.viewConfig, m.resourceType)
	if len(vd.Detail) == 0 {
		return nil
	}
	var lines []string
	for _, path := range vd.Detail {
		// Try extracting from RawStruct via fieldpath
		val := ""
		if m.res.RawStruct != nil {
			val = fieldpath.ExtractSubtree(m.res.RawStruct, path)
		}
		// Fallback: try Fields map (case-insensitive key match)
		if val == "" && len(m.res.Fields) > 0 {
			for k, v := range m.res.Fields {
				if strings.EqualFold(k, path) {
					val = v
					break
				}
			}
			// Also try underscore-separated version: "InstanceId" → "instance_id"
			if val == "" {
				snakeKey := toSnakeCase(path)
				if v, ok := m.res.Fields[snakeKey]; ok {
					val = v
				}
			}
		}
		if val == "" {
			val = "-"
		}
		if strings.Contains(val, "\n") {
			lines = append(lines, " "+styles.DetailSection.Render(path+":"))
			for _, subLine := range strings.Split(val, "\n") {
				lines = append(lines, "     "+styles.DetailVal.Render(subLine))
			}
		} else {
			lines = append(lines, kv(path, val))
		}
	}
	return lines
}

// toSnakeCase converts PascalCase to snake_case: "InstanceId" → "instance_id".
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(r + 32) // toLower
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}
