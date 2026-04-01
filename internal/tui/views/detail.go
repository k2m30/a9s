package views

import (
	"reflect"
	"sort"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/charmbracelet/x/ansi"

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
	search       SearchModel
	rightCol          rightColumnModel
	rightColVisible   bool // true when explicitly toggled on
	rightColAutoShown bool // true when right column was auto-shown on SetSize (wide terminal + registered defs)
	rightColWidth     int  // width of right column panel (default 32)
	fieldList    []fieldpath.FieldItem // structured field data; nil = not yet computed
	fieldCursor  int                   // index into fieldList for navigable cursor
}

// NewDetail creates a DetailModel for the given resource.
// resourceType identifies which ViewDef to use from the config (e.g. "ec2", "rds").
func NewDetail(res resource.Resource, resourceType string, viewConfig *config.ViewsConfig, k keys.Map) DetailModel {
	return DetailModel{
		resourceType:  resourceType,
		res:           res,
		viewConfig:    viewConfig,
		keys:          k,
		rightColWidth: 32,
	}
}

// buildFieldList computes m.fieldList from the view config and navigable field registry.
// Sets m.fieldList to nil when no config or detail paths are available (falls through to renderFromConfig).
func (m *DetailModel) buildFieldList() {
	if m.viewConfig == nil {
		m.fieldList = nil
		return
	}
	vd := config.GetViewDef(m.viewConfig, m.resourceType)
	if len(vd.Detail) == 0 {
		m.fieldList = nil
		return
	}
	navFields := resource.GetNavigableFields(m.resourceType)
	navMap := make(map[string]string, len(navFields))
	for _, nf := range navFields {
		navMap[nf.FieldPath] = nf.TargetType
	}
	m.fieldList = fieldpath.ExtractFieldList(m.res.RawStruct, m.res.Fields, vd.Detail, navMap)
}

// renderFromFieldList renders the structured field list to a string.
// Each FieldItem is rendered according to its type: header, sub-field, navigable, or normal.
func (m DetailModel) renderFromFieldList() string {
	if len(m.fieldList) == 0 {
		return styles.DimText.Render("  No detail data available")
	}
	// Collect top-level field paths for key width calculation.
	var topPaths []string
	for _, item := range m.fieldList {
		if !item.IsHeader && !item.IsSubField {
			topPaths = append(topPaths, item.Key)
		}
	}
	keyW := computeKeyWidth(topPaths)

	var lines []string
	for _, item := range m.fieldList {
		var line string
		switch {
		case item.IsHeader:
			line = " " + styles.DetailSection.Render(item.Key+":")
		case item.IsSubField:
			line = "     " + styles.DetailVal.Render(item.Value)
		case item.IsNavigable:
			line = " " + styles.DetailKey.Render(text.PadOrTrunc(item.Key+":", keyW)) + styles.NavigableField.Render(item.Value)
		default:
			line = " " + styles.DetailKey.Render(text.PadOrTrunc(item.Key+":", keyW)) + styles.DetailVal.Render(item.Value)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// Init implements tea.Model. No async work.
func (m DetailModel) Init() (DetailModel, tea.Cmd) {
	return m, nil
}

// Update delegates scroll to viewport; handles y (yaml), c (copy), esc (back).
func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.RelatedCheckResultMsg:
		// Ignore results for a different resource type.
		if msg.ResourceType != m.resourceType {
			return m, nil
		}
		m.rightCol, _ = m.rightCol.Update(msg)
		return m, nil
	case tea.KeyMsg:
		// Search input mode captures all keys.
		if m.search.IsInputMode() {
			m.search, _ = m.search.Update(msg)
			m.refreshViewportContent()
			return m, nil
		}
		// When right column is focused, delegate navigation keys to it.
		if m.rightColShowing() && m.rightCol.IsFocused() {
			switch {
			case key.Matches(msg, m.keys.Up), key.Matches(msg, m.keys.Down),
				key.Matches(msg, m.keys.Enter):
				var cmd tea.Cmd
				m.rightCol, cmd = m.rightCol.Update(msg)
				return m, cmd
			case key.Matches(msg, m.keys.Tab):
				m.rightCol.SetFocused(false)
				return m, nil
			case key.Matches(msg, m.keys.Escape):
				// Esc from focused right column: unfocus (don't pop view)
				m.rightCol.SetFocused(false)
				return m, nil
			}
			// Other keys (like ToggleRelated, Search, etc.) still handled by detail
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
		case key.Matches(msg, m.keys.Tab):
			if m.rightColShowing() {
				if m.rightCol.IsFocused() {
					m.rightCol.SetFocused(false)
				} else {
					m.rightCol.SetFocused(true)
				}
				return m, nil
			}
		case key.Matches(msg, m.keys.ToggleRelated):
			if m.width < 100 {
				return m, nil // silently ignore on narrow terminals
			}
			if m.rightColAutoShown {
				// Transition from auto-shown → explicitly on (still visible).
				// The first toggle after auto-show does NOT hide the column.
				m.rightColAutoShown = false
				m.rightColVisible = true
				defs := resource.GetRelated(m.resourceType)
				m.rightCol = newRightColumn(defs, m.res)
				m.rightCol.keys = m.keys
				m.rightCol.SetSize(m.rightColWidth, m.height)
				m.recalcViewportWidth()
				return m, func() tea.Msg {
					return messages.RelatedCheckStartedMsg{
						ResourceType:   m.resourceType,
						SourceResource: m.res,
					}
				}
			}
			// Normal toggle: flip visible state.
			m.rightColVisible = !m.rightColVisible
			if m.rightColVisible {
				defs := resource.GetRelated(m.resourceType)
				m.rightCol = newRightColumn(defs, m.res)
				m.rightCol.keys = m.keys
				m.rightCol.SetSize(m.rightColWidth, m.height)
				m.recalcViewportWidth()
				return m, func() tea.Msg {
					return messages.RelatedCheckStartedMsg{
						ResourceType:   m.resourceType,
						SourceResource: m.res,
					}
				}
			}
			m.recalcViewportWidth()
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			// Navigate to a related resource when pressing Enter on a navigable field.
			// Skip if the right column has focus — it handles its own Enter.
			if m.rightCol.IsFocused() {
				break
			}
			if m.fieldList != nil && m.fieldCursor >= 0 && m.fieldCursor < len(m.fieldList) {
				item := m.fieldList[m.fieldCursor]
				if item.IsNavigable {
					return m, func() tea.Msg {
						return messages.RelatedNavigateMsg{
							TargetType:     item.TargetType,
							SourceResource: m.res,
							SourceType:     m.resourceType,
							TargetID:       item.Value,
						}
					}
				}
			}
			return m, nil
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
			m.refreshViewportContent()
			return m, nil
		case key.Matches(msg, m.keys.Down):
			if !m.rightCol.IsFocused() && m.fieldList != nil && m.fieldCursor < len(m.fieldList)-1 {
				m.fieldCursor++
				m.syncViewportToCursor()
				return m, nil
			}
		case key.Matches(msg, m.keys.Up):
			if !m.rightCol.IsFocused() && m.fieldList != nil && m.fieldCursor > 0 {
				m.fieldCursor--
				m.syncViewportToCursor()
				return m, nil
			}
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
// When the right column is showing and width >= 100, renders left and right columns side by side.
func (m DetailModel) View() string {
	if !m.ready {
		return "Initializing..."
	}
	if m.rightColShowing() && m.width >= 100 {
		leftContent := m.viewport.View()
		rightContent := m.rightCol.View()
		leftLines := strings.Split(leftContent, "\n")
		rightLines := strings.Split(rightContent, "\n")
		// Normalise to same number of lines.
		maxLines := len(leftLines)
		if len(rightLines) > maxLines {
			maxLines = len(rightLines)
		}
		leftW := m.width - m.rightColWidth
		var sb strings.Builder
		for i := range maxLines {
			if i > 0 {
				sb.WriteString("\n")
			}
			left := ""
			if i < len(leftLines) {
				left = leftLines[i]
			}
			right := ""
			if i < len(rightLines) {
				right = rightLines[i]
			}
			// Pad left column to its fixed width so right column aligns correctly.
			padded := left
			leftVisible := lipgloss.Width(left)
			if leftVisible < leftW {
				padded = left + strings.Repeat(" ", leftW-leftVisible)
			}
			sb.WriteString(padded)
			sb.WriteString(right)
		}
		return sb.String()
	}
	return m.viewport.View()
}

// SetSize initializes or resizes the viewport. Must be called before View().
// On first call, if width >= 100 and related defs are registered, the right
// column is auto-shown (rightColAutoShown = true). The first explicit toggle
// transitions from auto-shown to explicitly-on (still visible). A second toggle
// hides the column.
func (m *DetailModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	// Auto-show right column on first SetSize call when terminal is wide enough
	// and there are registered related defs for this resource type.
	if !m.ready && w >= 100 && len(resource.GetRelated(m.resourceType)) > 0 {
		m.rightColAutoShown = true
		m.rightCol = newRightColumn(resource.GetRelated(m.resourceType), m.res)
		m.rightCol.keys = m.keys
	}

	viewportW := w
	if m.rightColShowing() && w >= 100 {
		viewportW = w - m.rightColWidth
		m.rightCol.SetSize(m.rightColWidth, h)
	}

	if !m.ready {
		m.viewport = viewport.New(viewport.WithWidth(viewportW), viewport.WithHeight(h))
		m.ready = true
	} else {
		m.viewport.SetWidth(viewportW)
		m.viewport.SetHeight(h)
	}
	m.refreshViewportContent()
}

// rightColShowing returns true when the right column should be rendered.
// The column shows when explicitly toggled on OR when auto-shown on entry.
func (m DetailModel) rightColShowing() bool {
	return m.rightColVisible || m.rightColAutoShown
}

// recalcViewportWidth adjusts the viewport width based on the right column visibility.
func (m *DetailModel) recalcViewportWidth() {
	if m.rightColShowing() && m.width >= 100 {
		leftW := m.width - m.rightColWidth
		if m.ready {
			m.viewport.SetWidth(leftW)
		}
	} else if m.ready {
		m.viewport.SetWidth(m.width)
	}
	m.refreshViewportContent()
}

// syncViewportToCursor adjusts the viewport scroll to keep fieldCursor visible.
func (m *DetailModel) syncViewportToCursor() {
	if !m.ready {
		return
	}
	yOffset := m.viewport.YOffset()
	visibleLines := m.viewport.Height()
	if m.fieldCursor < yOffset {
		m.viewport.SetYOffset(m.fieldCursor)
	} else if m.fieldCursor >= yOffset+visibleLines {
		m.viewport.SetYOffset(m.fieldCursor - visibleLines + 1)
	}
}

// refreshViewportContent re-renders content and applies search highlights.
func (m *DetailModel) refreshViewportContent() {
	if m.fieldList == nil {
		m.buildFieldList()
	}
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

// IsSearchActive returns true when search is active (input mode or confirmed highlights).
func (m DetailModel) IsSearchActive() bool {
	return m.search.IsActive()
}

// IsSearchInputMode returns true when the search input is capturing keystrokes.
func (m DetailModel) IsSearchInputMode() bool {
	return m.search.IsInputMode()
}

// SearchInfo returns the search state string for the header.
// Input mode: "/query" (or "/" when query is empty), Confirmed: "N/M matches", Inactive: "".
func (m DetailModel) SearchInfo() string {
	if !m.search.IsActive() {
		return ""
	}
	if m.search.IsInputMode() {
		q := m.search.Query()
		return "/" + q
	}
	return m.search.MatchInfo()
}

// ResourceID returns the resource ID for clipboard copy.
func (m DetailModel) ResourceID() string {
	return m.res.ID
}

// ResourceType returns the resource type short name.
func (m DetailModel) ResourceType() string {
	return m.resourceType
}

// SourceResource returns the resource being viewed.
func (m DetailModel) SourceResource() resource.Resource {
	return m.res
}

// NeedsRelatedCheck returns true when the right column was auto-shown
// and checkers have not yet been dispatched. The root model checks this
// after pushing the detail view to emit RelatedCheckStartedMsg.
func (m DetailModel) NeedsRelatedCheck() bool {
	return m.rightColAutoShown
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
	// Use structured field list when available.
	if m.fieldList != nil {
		return m.renderFromFieldList()
	}

	// Try config-driven rendering.
	if m.viewConfig != nil {
		vd := config.GetViewDef(m.viewConfig, m.resourceType)
		if len(vd.Detail) > 0 {
			keyW := computeKeyWidth(vd.Detail)
			kv := func(key, val string) string {
				return " " + styles.DetailKey.Render(text.PadOrTrunc(key+":", keyW)) + styles.DetailVal.Render(val)
			}
			if lines := m.renderFromConfig(kv); len(lines) > 0 {
				return strings.Join(lines, "\n")
			}
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

	keyW := computeKeyWidth(fieldKeys)
	kv := func(key, val string) string {
		return " " + styles.DetailKey.Render(text.PadOrTrunc(key+":", keyW)) + styles.DetailVal.Render(val)
	}

	var lines []string
	for _, k := range fieldKeys {
		lines = append(lines, kv(k, m.res.Fields[k]))
	}
	return strings.Join(lines, "\n")
}

// computeKeyWidth returns the width needed for the key column: longest key + 1 (for colon), minimum 22.
func computeKeyWidth(keys []string) int {
	w := 22
	for _, k := range keys {
		if len(k)+1 > w {
			w = len(k) + 1
		}
	}
	return w
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
		val := ""
		// Try Fields map first — fetchers populate Fields with pre-formatted
		// values (e.g., formatted timestamps instead of raw epoch ms).
		if len(m.res.Fields) > 0 {
			// Try exact case-insensitive match
			for k, v := range m.res.Fields {
				if strings.EqualFold(k, path) {
					val = v
					break
				}
			}
			// Try underscore-separated version: "InstanceId" → "instance_id"
			if val == "" {
				snakeKey := toSnakeCase(path)
				if v, ok := m.res.Fields[snakeKey]; ok {
					val = v
				}
			}
		}
		// Fall back to RawStruct extraction for fields not in Fields map
		if val == "" && m.res.RawStruct != nil {
			val = fieldpath.ExtractSubtree(m.res.RawStruct, path)
		}
		if val == "" {
			val = "-"
		}
		if strings.Contains(val, "\n") {
			lines = append(lines, " "+styles.DetailSection.Render(path+":"))
			for subLine := range strings.SplitSeq(val, "\n") {
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
