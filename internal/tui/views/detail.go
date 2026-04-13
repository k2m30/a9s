package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
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
	rightColVisible          bool // true when explicitly toggled on
	rightColAutoShown        bool // true when right column was auto-shown on SetSize (wide terminal + registered defs)
	rightColUserToggled      bool // true after user explicitly toggles related visibility
	rightColWidth            int  // width of right column panel (default 32)
	pendingRelatedDispatch   bool // true when a narrow→wide resize should dispatch RelatedCheckStartedMsg
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

// TakePendingRelatedDispatch returns true and clears the resize-dispatch flag.
// Called by the root model's tea.WindowSizeMsg handler (after propagateSize)
// so RelatedCheckStartedMsg is emitted from the correct place.
func (m *DetailModel) TakePendingRelatedDispatch() bool {
	if m.pendingRelatedDispatch {
		m.pendingRelatedDispatch = false
		return true
	}
	return false
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
	case messages.EnrichDetailResultMsg:
		// Guard: ignore results for a different resource type or resource ID.
		if msg.ResourceType != m.resourceType || msg.ResourceID != m.res.ID {
			return m, nil
		}
		m.res = msg.EnrichedRes
		m.fieldList = nil // force rebuild on next render
		m.refreshViewportContent()
		return m, nil
	case tea.PasteMsg:
		// Route bracketed paste to search when in search input mode.
		if m.search.IsInputMode() {
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.refreshViewportContent()
			return m, cmd
		}
		return m, nil
	case searchPasteMsg:
		// Route ctrl+V clipboard result to search when in search input mode.
		if m.search.IsInputMode() {
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.refreshViewportContent()
			return m, cmd
		}
		return m, nil
	case tea.KeyMsg:
		// Search input mode captures all keys.
		if m.search.IsInputMode() {
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.refreshViewportContent()
			return m, cmd
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
			if m.width < layout.MinInnerContentWidth {
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
				m.rightCol = newRightColumn(defs, m.res, m.resourceType)
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
					targetID := item.Value
					if item.NavID != "" {
						targetID = item.NavID
					}
					return m, func() tea.Msg {
						return messages.RelatedNavigateMsg{
							TargetType:     item.TargetType,
							SourceResource: m.res,
							SourceType:     m.resourceType,
							TargetID:       targetID,
						}
					}
				}
			}
			return m, nil
		case key.Matches(msg, m.keys.YAML):
			return m, func() tea.Msg {
				return messages.NavigateMsg{
					Target:       messages.TargetYAML,
					Resource:     &m.res,
					ResourceType: m.resourceType,
				}
			}
		case key.Matches(msg, m.keys.JSON):
			return m, func() tea.Msg {
				return messages.NavigateMsg{
					Target:       messages.TargetJSON,
					Resource:     &m.res,
					ResourceType: m.resourceType,
				}
			}
		case key.Matches(msg, m.keys.ToggleWrap):
			m.wrap = !m.wrap
			m.viewport.SoftWrap = m.wrap
			m.refreshViewportContent()
			return m, nil
		case key.Matches(msg, m.keys.CloudTrail):
			if ff := resource.BuildCloudTrailFilter(m.res, m.resourceType); ff != nil {
				res := m.res
				rt := m.resourceType
				return m, func() tea.Msg {
					return messages.RelatedNavigateMsg{
						TargetType:     "ct-events",
						SourceResource: res,
						SourceType:     rt,
						FetchFilter:    ff,
					}
				}
			}
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
				// Skip IsSection rows (section headers for ct-events should not receive cursor focus).
				for m.fieldCursor < len(m.fieldList)-1 && m.fieldList[m.fieldCursor].IsSection {
					m.fieldCursor++
				}
				m.syncViewportToCursor()
				m.refreshViewportContent()
				return m, nil
			}
			return m, nil // Bug6 fix: clamp at boundary, don't fall through to viewport scroll
		case key.Matches(msg, m.keys.Up):
			if !m.rightCol.IsFocused() && m.fieldList != nil && m.fieldCursor > 0 {
				m.fieldCursor--
				// Skip IsSection rows (section headers for ct-events should not receive cursor focus).
				for m.fieldCursor > 0 && m.fieldList[m.fieldCursor].IsSection {
					m.fieldCursor--
				}
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

