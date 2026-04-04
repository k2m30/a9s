package views

import (
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
)

// DetailModel renders the key-value describe view using bubbles/viewport for scroll.
type DetailModel struct {
	res                 resource.Resource
	resourceType        string // e.g. "ec2", "s3", "rds" — used to look up correct ViewDef
	viewConfig          *config.ViewsConfig
	viewport            viewport.Model
	ready               bool
	wrap                bool
	width               int
	height              int
	keys                keys.Map
	search              SearchModel
	rightCol            rightColumnModel
	rightColVisible     bool                  // true when explicitly toggled on
	rightColAutoShown   bool                  // true when right column was auto-shown on SetSize (wide terminal + registered defs)
	rightColUserToggled bool                  // true after user explicitly toggles related visibility
	rightColWidth       int                   // width of right column panel (default 32)
	fieldList           []fieldpath.FieldItem // structured field data; nil = not yet computed
	fieldCursor         int                   // index into fieldList for navigable cursor
}

// NewDetail creates a DetailModel for the given resource.
// resourceType identifies which ViewDef to use from the config (e.g. "ec2", "rds").
func NewDetail(res resource.Resource, resourceType string, viewConfig *config.ViewsConfig, k keys.Map) DetailModel {
	if resourceType == "" {
		resourceType = inferDetailResourceType(res)
	}
	return DetailModel{
		resourceType:  resourceType,
		res:           res,
		viewConfig:    viewConfig,
		keys:          k,
		rightColWidth: 32,
	}
}

// inferDetailResourceType provides a conservative fallback for routes that
// navigate to detail without an explicit type. This prevents losing related
// and navigable behavior for common top-level resources.
func inferDetailResourceType(res resource.Resource) string {
	has := func(k string) bool {
		v, ok := res.Fields[k]
		return ok && strings.TrimSpace(v) != ""
	}
	// EC2 signature: infer only from EC2-shaped key sets.
	// Anchor on instance id key plus at least one EC2-specific companion field.
	hasInstanceID := has("InstanceId") || has("instance_id")
	hasEC2Companion := has("ImageId") || has("image_id") ||
		has("VpcId") || has("vpc_id") ||
		has("SubnetId") || has("subnet_id") ||
		has("PrivateIpAddress") || has("private_ip") ||
		has("PublicIpAddress") || has("public_ip") ||
		has("KeyName") || has("key_name") ||
		has("InstanceLifecycle") || has("lifecycle") ||
		has("LaunchTime") || has("launch_time") ||
		has("IamInstanceProfile") || has("iam_instance_profile") ||
		has("SecurityGroups") || has("security_groups")
	if hasInstanceID && hasEC2Companion {
		return "ec2"
	}
	return ""
}

// Init implements tea.Model. No async work.
func (m DetailModel) Init() (DetailModel, tea.Cmd) {
	return m, nil
}

// Update delegates scroll to viewport; handles y (yaml), c (copy), esc (back).
func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.RelatedCheckResultMsg:
		// Ignore results for a different resource type or source resource.
		if msg.ResourceType != m.resourceType || (msg.SourceResourceID != "" && msg.SourceResourceID != m.res.ID) {
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
			if m.rightCol.IsFiltering() {
				var cmd tea.Cmd
				m.rightCol, cmd = m.rightCol.Update(msg)
				m.refreshViewportContent()
				return m, cmd
			}
			switch {
			case key.Matches(msg, m.keys.Up), key.Matches(msg, m.keys.Down),
				key.Matches(msg, m.keys.Enter):
				var cmd tea.Cmd
				m.rightCol, cmd = m.rightCol.Update(msg)
				m.refreshViewportContent()
				return m, cmd
			case key.Matches(msg, m.keys.Search):
				var cmd tea.Cmd
				m.rightCol, cmd = m.rightCol.Update(msg)
				m.refreshViewportContent()
				return m, cmd
			case key.Matches(msg, m.keys.Tab):
				m.rightCol.SetFocused(false)
				m.refreshViewportContent() // update cursor highlight after focus change
				return m, nil
			case key.Matches(msg, m.keys.Escape):
				if m.rightCol.IsFiltering() || m.rightCol.HasFilter() {
					var cmd tea.Cmd
					m.rightCol, cmd = m.rightCol.Update(msg)
					m.refreshViewportContent()
					return m, cmd
				}
				// Esc from focused right column: unfocus (don't pop view)
				m.rightCol.SetFocused(false)
				m.refreshViewportContent() // update cursor highlight after focus change
				return m, nil
			}
			// Other keys (like ToggleRelated, Search, etc.) still handled by detail
		}
		switch {
		case key.Matches(msg, m.keys.ScrollRight):
			// l: focus right column (if showing and not already focused)
			if m.rightColShowing() && !m.rightCol.IsFocused() && m.rightCol.HasActionableRows() {
				m.rightCol.SetFocused(true)
				m.refreshViewportContent()
				return m, nil
			}
			return m, nil
		case key.Matches(msg, m.keys.ScrollLeft):
			// h: focus left column (if right is focused)
			if m.rightCol.IsFocused() {
				m.rightCol.SetFocused(false)
				m.refreshViewportContent()
				return m, nil
			}
			return m, nil
		case key.Matches(msg, m.keys.Copy):
			if m.rightCol.IsFocused() {
				name := m.rightCol.SelectedTypeName()
				if name != "" {
					return m, func() tea.Msg {
						return messages.CopiedMsg{Content: name}
					}
				}
				return m, nil
			}
			// Left column: copy the field value at cursor
			if m.fieldList != nil && m.fieldCursor >= 0 && m.fieldCursor < len(m.fieldList) {
				item := m.fieldList[m.fieldCursor]
				val := item.Value
				if val == "" {
					val = item.Key
				}
				return m, func() tea.Msg {
					return messages.CopiedMsg{Content: val}
				}
			}
			return m, nil
		case key.Matches(msg, m.keys.Refresh):
			// Ctrl+R: re-trigger related checks if right column is showing
			if m.rightColShowing() {
				defs := resource.GetRelated(m.resourceType)
				m.rightCol = newRightColumn(defs, m.res)
				m.rightCol.keys = m.keys
				m.rightCol.SetSize(m.currentRightColWidth(), m.height)
				return m, func() tea.Msg {
					return messages.RelatedCheckStartedMsg{
						ResourceType:   m.resourceType,
						SourceResource: m.res,
					}
				}
			}
			return m, nil
		case key.Matches(msg, m.keys.PageDown):
			if !m.rightCol.IsFocused() && m.fieldList != nil {
				pageSize := max(m.height-4, 1)
				m.fieldCursor += pageSize
				if m.fieldCursor >= len(m.fieldList) {
					m.fieldCursor = len(m.fieldList) - 1
				}
				m.syncViewportToCursor()
				m.refreshViewportContent()
				return m, nil
			}
			// No fieldList — scroll viewport directly
			if m.ready {
				m.viewport.HalfPageDown()
				return m, nil
			}
			return m, nil
		case key.Matches(msg, m.keys.PageUp):
			if !m.rightCol.IsFocused() && m.fieldList != nil {
				pageSize := max(m.height-4, 1)
				m.fieldCursor = max(m.fieldCursor-pageSize, 0)
				m.syncViewportToCursor()
				m.refreshViewportContent()
				return m, nil
			}
			// No fieldList — scroll viewport directly
			if m.ready {
				m.viewport.HalfPageUp()
				return m, nil
			}
			return m, nil
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
			if m.rightColShowing() && (m.rightCol.IsFocused() || m.rightCol.HasActionableRows()) {
				if m.rightCol.IsFocused() {
					m.rightCol.SetFocused(false)
				} else {
					m.rightCol.SetFocused(true)
				}
				m.refreshViewportContent() // update cursor highlight after focus change
				return m, nil
			}
		case key.Matches(msg, m.keys.ToggleRelated):
			m.rightColUserToggled = true
			if m.width < 60 {
				return m, nil // silently ignore on narrow terminals
			}
			if m.rightColAutoShown {
				// First explicit toggle hides the auto-shown column.
				m.rightColAutoShown = false
				m.rightColVisible = false
				m.rightCol.SetFocused(false)
				m.recalcViewportWidth()
				return m, nil
			}
			// Normal toggle: flip visible state.
			m.rightColVisible = !m.rightColVisible
			if m.rightColVisible {
				defs := resource.GetRelated(m.resourceType)
				m.rightCol = newRightColumn(defs, m.res)
				m.rightCol.keys = m.keys
				m.rightCol.SetSize(m.currentRightColWidth(), m.height)
				m.recalcViewportWidth()
				return m, func() tea.Msg {
					return messages.RelatedCheckStartedMsg{
						ResourceType:   m.resourceType,
						SourceResource: m.res,
					}
				}
			}
			m.rightCol.SetFocused(false)
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
		case key.Matches(msg, m.keys.Top):
			if !m.rightCol.IsFocused() && m.fieldList != nil && m.fieldCursor > 0 {
				m.fieldCursor = 0
				m.syncViewportToCursor()
				m.refreshViewportContent()
				return m, nil
			}
			return m, nil
		case key.Matches(msg, m.keys.Bottom):
			if !m.rightCol.IsFocused() && m.fieldList != nil && m.fieldCursor < len(m.fieldList)-1 {
				m.fieldCursor = len(m.fieldList) - 1
				m.syncViewportToCursor()
				m.refreshViewportContent()
				return m, nil
			}
			return m, nil
		case key.Matches(msg, m.keys.Down):
			if !m.rightCol.IsFocused() && m.fieldList != nil && m.fieldCursor < len(m.fieldList)-1 {
				m.fieldCursor++
				m.syncViewportToCursor()
				m.refreshViewportContent()
				return m, nil
			}
			return m, nil // Bug6 fix: clamp at boundary, don't fall through to viewport scroll
		case key.Matches(msg, m.keys.Up):
			if !m.rightCol.IsFocused() && m.fieldList != nil && m.fieldCursor > 0 {
				m.fieldCursor--
				m.syncViewportToCursor()
				m.refreshViewportContent()
				return m, nil
			}
			return m, nil // Bug6 fix: clamp at boundary, don't fall through to viewport scroll
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
// When the right column is showing and width >= 60, renders left and right columns side by side
// with a │ separator.
func (m DetailModel) View() string {
	if !m.ready {
		return "Initializing..."
	}
	if m.rightColShowing() && m.width >= 60 {
		// Keep RELATED visible at medium widths by using side-by-side layout.
		rightW := m.currentRightColWidth()
		sep := styles.ColSepDim.Render("│")
		if m.rightCol.IsFocused() {
			sep = styles.ColSepAccent.Render("│")
		}
		leftContent := m.viewport.View()
		rightContent := m.rightCol.View()
		leftLines := strings.Split(leftContent, "\n")
		rightLines := strings.Split(rightContent, "\n")
		// Normalise to same number of lines.
		maxLines := max(len(leftLines), len(rightLines))
		leftW := m.width - rightW - 1 // -1 for separator character
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
				right = ansi.Truncate(rightLines[i], rightW, "")
			}
			// Pad left column to its fixed width so right column aligns correctly.
			padded := left
			leftVisible := lipgloss.Width(left)
			if leftVisible < leftW {
				padded = left + strings.Repeat(" ", leftW-leftVisible)
			}
			sb.WriteString(padded)
			sb.WriteString(sep)
			sb.WriteString(right)
		}
		return sb.String()
	}
	return m.viewport.View()
}

// SetSize initializes or resizes the viewport. Must be called before View().
// On first call, if width >= 60 and related defs are registered, the right
// column is auto-shown (rightColAutoShown = true). The first explicit toggle
// hides the auto-shown column. A subsequent toggle re-shows it.
func (m *DetailModel) SetSize(w, h int) {
	m.width = w
	m.height = h

	wasShowing := m.rightColShowing()

	// Auto-show right column when wide enough and related defs exist:
	// - on first SetSize call, and
	// - on later resizes only if user hasn't explicitly toggled visibility.
	if w >= 60 && len(resource.GetRelated(m.resourceType)) > 0 &&
		(!m.ready || (!m.rightColShowing() && !m.rightColUserToggled)) {
		m.rightColAutoShown = true
		m.rightCol = newRightColumn(resource.GetRelated(m.resourceType), m.res)
		m.rightCol.keys = m.keys
	} else if w < 60 && wasShowing {
		m.rightColAutoShown = false
		m.rightColVisible = false
		m.rightCol.SetFocused(false)
	}

	viewportW := w
	if m.rightColShowing() && w >= 60 {
		rightW := m.currentRightColWidth()
		viewportW = w - rightW - 1 // -1 for separator character
		m.rightCol.SetSize(rightW, h)
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
	if m.rightColShowing() && m.width >= 60 {
		leftW := m.width - m.currentRightColWidth() - 1 // -1 for separator
		if m.ready {
			m.viewport.SetWidth(leftW)
		}
	} else if m.ready {
		m.viewport.SetWidth(m.width)
	}
	m.refreshViewportContent()
}

func (m DetailModel) currentRightColWidth() int {
	// Keep right panel readable at medium widths while preserving left detail space.
	if m.width <= 0 {
		return m.rightColWidth
	}
	if m.width >= 100 {
		return m.rightColWidth
	}
	w := max(24, m.width/3)
	maxAllowed := max(16, m.width-40) // keep at least 40 cols for left pane
	if w > maxAllowed {
		w = maxAllowed
	}
	return w
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

// FieldCursor returns the current field cursor index for testing.
func (m DetailModel) FieldCursor() int {
	return m.fieldCursor
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

// CopyContent returns column-aware clipboard content for the active selection.
func (m DetailModel) CopyContent() (string, string) {
	if m.rightCol.IsFocused() {
		name := m.rightCol.SelectedTypeName()
		if name == "" {
			return "", ""
		}
		return name, "Copied: " + name
	}
	if m.fieldList != nil && m.fieldCursor >= 0 && m.fieldCursor < len(m.fieldList) {
		item := m.fieldList[m.fieldCursor]
		content := item.Value
		if content == "" {
			content = item.Key
		}
		if content == "" {
			return "", ""
		}
		return content, "Copied: " + content
	}
	content := m.RawYAML()
	if content == "" {
		return "", ""
	}
	return content, "Copied detail to clipboard"
}

// GetHelpContext returns HelpFromDetail.
func (m DetailModel) GetHelpContext() HelpContext {
	return HelpFromDetail
}

// IsSearchActive returns true when search is active (input mode or confirmed highlights).
func (m DetailModel) IsSearchActive() bool {
	return m.search.IsActive() || m.rightCol.IsFiltering()
}

// IsSearchInputMode returns true when the search input is capturing keystrokes.
func (m DetailModel) IsSearchInputMode() bool {
	return m.search.IsInputMode() || m.rightCol.IsFiltering()
}

// SearchInfo returns the search state string for the header.
// Input mode: "/query" (or "/" when query is empty), Confirmed: "N/M matches", Inactive: "".
func (m DetailModel) SearchInfo() string {
	if m.rightCol.IsFiltering() {
		return "/" + m.rightCol.FilterQuery()
	}
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

// ApplyRelatedResults injects cached related check results into the right column,
// avoiding re-dispatch of async checkers. Called by root model on detail re-entry.
func (m *DetailModel) ApplyRelatedResults(results []resource.RelatedCheckResult) {
	for _, r := range results {
		m.rightCol, _ = m.rightCol.Update(messages.RelatedCheckResultMsg{
			ResourceType: m.resourceType,
			Result:       r,
		})
	}
}

// ResetRightColumn resets the right column to its initial loading state,
// discarding any loaded counts. Called by handleRefresh before re-dispatching
// async checks so stale counts are not shown during reload.
func (m *DetailModel) ResetRightColumn() {
	if !m.rightColShowing() {
		return
	}
	defs := resource.GetRelated(m.resourceType)
	m.rightCol = newRightColumn(defs, m.res)
	m.rightCol.keys = m.keys
	m.rightCol.SetSize(m.currentRightColWidth(), m.height)
}

// ConsumesEscapeLocally reports whether Escape should be handled inside the
// detail view instead of by the root view-stack pop logic.
func (m DetailModel) ConsumesEscapeLocally() bool {
	return m.rightCol.IsFocused() || m.rightCol.IsFiltering()
}
