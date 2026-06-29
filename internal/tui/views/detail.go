package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
)

// DetailModel renders the key-value describe view using bubbles/viewport for scroll.
// ctrl is non-nil when the model is constructed by the TUI navigator via
// NewDetailWithCtrl; in that case View() delegates to
// RenderDetail(ctrl.Snapshot().Body.Detail) and key actions route to ctrl.Apply.
// Unit tests and isolated callers leave ctrl nil; View() builds a live body and
// delegates to RenderDetail so parity tests remain unaffected.
type DetailModel struct {
	ctrl                   *app.Controller             // non-nil = controller-backed TUI path
	res                    resource.Resource
	resourceType           string // e.g. "ec2", "s3", "rds" — used to look up correct ViewDef
	viewConfig             *config.ViewsConfig
	navProvider            func(string) []resource.NavigableField // returns navigable fields for a resource type; defaults to GetActiveNavigableFields
	viewport               viewport.Model
	ready                  bool
	wrap                   bool
	width                  int
	height                 int
	keys                   keys.Map
	search                 SearchModel
	rightCol               rightColumnModel
	rightColVisible        bool                        // true when explicitly toggled on
	rightColAutoShown      bool                        // true when right column was auto-shown on SetSize (wide terminal + registered defs)
	rightColUserToggled    bool                        // true after user explicitly toggles related visibility
	rightColWidth          int                         // width of right column panel (default 32)
	pendingRelatedDispatch bool                        // true when a narrow→wide resize should dispatch RelatedCheckStartedMsg
	fieldList              []fieldpath.FieldItem       // structured field data; nil = not yet computed
	fieldCursor            int                         // index into fieldList for navigable cursor
	plainMode              bool                        // true only during PlainContent(); causes Attention entries to render Key: Value (full text for clipboard/search)
}

// updateKeyMsgWithCtrl handles tea.KeyMsg events when m.ctrl is non-nil.
// Stateful actions (wrap, search, scroll, cursor, related panel) are routed to
// m.ctrl.Apply so the controller owns all mutable state. Navigation keys that
// open new screens (YAML, JSON, Enter on navigable fields, CloudTrail, Copy)
// still emit messages unchanged — they have no effect on detail state.
func (m DetailModel) updateKeyMsgWithCtrl(msg tea.KeyMsg) (DetailModel, tea.Cmd) {
	// Search input mode: let the search widget accumulate keystrokes, then sync
	// the resulting query to the controller on every update.
	if m.search.IsInputMode() {
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		// Sync query into controller so Snapshot().Body.Detail.Search is live.
		m.ctrl.Apply(app.Action{Kind: app.ActionSearch, Arg: m.search.Query()})
		// Match computation/highlight is renderer-side: feed m.search the rendered
		// content so MatchCount()/the N/M header populate (incl. the enter that
		// confirms the search).
		m.refreshViewportContent()
		return m, cmd
	}

	// When related panel is focused and filtering, delegate to the right column
	// widget for text input, then sync filter to controller.
	if m.rightColShowing() && m.rightCol.IsFocused() && m.rightCol.IsFiltering() {
		var cmd tea.Cmd
		m.rightCol, cmd = m.rightCol.Update(msg)
		m.ctrl.Apply(app.Action{Kind: app.ActionSetFilter, Arg: m.rightCol.FilterQuery()})
		return m, cmd
	}

	pageSize := max(m.height-4, 1)

	switch {
	// --- Related panel focused: Up/Down/Enter/Search/Tab/Esc handled by ctrl ---
	case m.rightColShowing() && m.rightCol.IsFocused():
		switch {
		case key.Matches(msg, m.keys.Up):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveUp})
			if body := m.ctrl.Snapshot().Body.Detail; body != nil {
				m.rightCol.cursor = body.RelatedCursor
			}
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveDown})
			if body := m.ctrl.Snapshot().Body.Detail; body != nil {
				m.rightCol.cursor = body.RelatedCursor
			}
			return m, nil
		case key.Matches(msg, m.keys.Search):
			// Start filtering the related panel.
			m.rightCol, _ = m.rightCol.Update(msg)
			m.ctrl.Apply(app.Action{Kind: app.ActionSetFilter, Arg: m.rightCol.FilterQuery()})
			return m, nil
		case key.Matches(msg, m.keys.Tab):
			m.ctrl.Apply(app.Action{Kind: app.ActionToggleFocus})
			m.rightCol.SetFocused(false)
			return m, nil
		case key.Matches(msg, m.keys.Escape):
			if m.rightCol.HasFilter() {
				m.rightCol, _ = m.rightCol.Update(msg)
				m.ctrl.Apply(app.Action{Kind: app.ActionSetFilter, Arg: ""})
				return m, nil
			}
			// Unfocus right column.
			m.ctrl.Apply(app.Action{Kind: app.ActionToggleFocus})
			m.rightCol.SetFocused(false)
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			// Enter on a related row → navigate (same as legacy).
			var cmd tea.Cmd
			m.rightCol, cmd = m.rightCol.Update(msg)
			return m, cmd
		}
		// For any other key fall through to the global cases below (ToggleRelated etc.)
		fallthrough

	default:
		switch {
		case key.Matches(msg, m.keys.ToggleWrap):
			m.ctrl.Apply(app.Action{Kind: app.ActionToggleWrap})
			return m, nil

		case key.Matches(msg, m.keys.Search):
			// When the related pane is focused, '/' activates the rightcol filter,
			// not the main detail search (renderer-side; query synced to controller).
			if m.rightColShowing() && m.rightCol.IsFocused() {
				m.rightCol, _ = m.rightCol.Update(msg)
				m.ctrl.Apply(app.Action{Kind: app.ActionSetFilter, Arg: m.rightCol.FilterQuery()})
				return m, nil
			}
			m.search.Activate()
			m.ctrl.Apply(app.Action{Kind: app.ActionSearch, Arg: ""})
			return m, nil

		case key.Matches(msg, m.keys.SearchNext):
			// Match navigation/highlight is renderer-side (depends on rendered
			// content): drive m.search so the N/M header + viewport follow.
			if m.search.IsActive() && m.search.MatchCount() > 0 {
				m.search.NextMatch()
				m.refreshViewportContent()
			}
			m.ctrl.Apply(app.Action{Kind: app.ActionSearchNext})
			return m, nil

		case key.Matches(msg, m.keys.SearchPrev):
			if m.search.IsActive() && m.search.MatchCount() > 0 {
				m.search.PrevMatch()
				m.refreshViewportContent()
			}
			m.ctrl.Apply(app.Action{Kind: app.ActionSearchPrev})
			return m, nil

		case key.Matches(msg, m.keys.Escape):
			if m.search.IsActive() {
				m.search.Deactivate()
				m.ctrl.Apply(app.Action{Kind: app.ActionSearchClear})
				return m, nil
			}

		case key.Matches(msg, m.keys.PageDown):
			m.ctrl.Apply(app.Action{Kind: app.ActionPageDown, N: pageSize})
			if body := m.ctrl.Snapshot().Body.Detail; body != nil {
				m.fieldCursor = body.FieldCursor
			}
			return m, nil

		case key.Matches(msg, m.keys.PageUp):
			m.ctrl.Apply(app.Action{Kind: app.ActionPageUp, N: pageSize})
			if body := m.ctrl.Snapshot().Body.Detail; body != nil {
				m.fieldCursor = body.FieldCursor
			}
			return m, nil

		case key.Matches(msg, m.keys.ToggleRelated):
			// Mirror the legacy flag management so SetSize auto-show respects
			// explicit hides: rightColUserToggled gates the auto-show path.
			m.rightColUserToggled = true
			if m.width < layout.MinInnerContentWidth {
				return m, nil
			}
			if m.rightColAutoShown {
				// First explicit toggle: hide the auto-shown column.
				m.rightColAutoShown = false
				m.rightColVisible = false
				m.rightCol.SetFocused(false)
				m.recalcViewportWidth()
				// Sync controller: hidden=true suppresses auto-show in buildDetailBody.
				m.ctrl.SetDetailRelatedVisible(false, true)
				return m, nil
			}
			// Normal toggle: flip visible state.
			m.rightColVisible = !m.rightColVisible
			// Sync controller to the resolved state; hidden=true so it stays controlled.
			m.ctrl.SetDetailRelatedVisible(m.rightColVisible, true)
			if m.rightColVisible {
				defs := resource.GetRelated(m.resourceType)
				m.rightCol = newRightColumn(defs, m.res, m.resourceType)
				m.rightCol.keys = m.keys
				m.rightCol.SetSize(m.currentRightColWidth(), m.height)
				m.recalcViewportWidth()
				return m, func() tea.Msg {
					return messages.RelatedCheckStarted{
						ResourceType:   m.resourceType,
						SourceResource: m.res,
					}
				}
			}
			m.rightCol.SetFocused(false)
			m.recalcViewportWidth()
			return m, nil

		case key.Matches(msg, m.keys.Tab):
			m.ctrl.Apply(app.Action{Kind: app.ActionToggleFocus})
			if m.rightCol.IsFocused() {
				m.rightCol.SetFocused(false)
			} else if m.rightColShowing() {
				m.rightCol.SetFocused(true)
			}
			return m, nil

		case key.Matches(msg, m.keys.ScrollRight):
			// l: focus right column.
			if m.rightColShowing() && !m.rightCol.IsFocused() {
				m.ctrl.Apply(app.Action{Kind: app.ActionToggleFocus})
				m.rightCol.SetFocused(true)
			}
			return m, nil

		case key.Matches(msg, m.keys.ScrollLeft):
			// h: focus left column.
			if m.rightCol.IsFocused() {
				m.ctrl.Apply(app.Action{Kind: app.ActionToggleFocus})
				m.rightCol.SetFocused(false)
			}
			return m, nil

		case key.Matches(msg, m.keys.Down):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveDown})
			// Keep local fieldCursor in sync so Enter/Copy can index m.fieldList.
			if body := m.ctrl.Snapshot().Body.Detail; body != nil {
				m.fieldCursor = body.FieldCursor
			}
			return m, nil

		case key.Matches(msg, m.keys.Up):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveUp})
			if body := m.ctrl.Snapshot().Body.Detail; body != nil {
				m.fieldCursor = body.FieldCursor
			}
			return m, nil

		case key.Matches(msg, m.keys.Top):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveTop})
			if body := m.ctrl.Snapshot().Body.Detail; body != nil {
				m.fieldCursor = body.FieldCursor
			}
			return m, nil

		case key.Matches(msg, m.keys.Bottom):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveBottom})
			if body := m.ctrl.Snapshot().Body.Detail; body != nil {
				m.fieldCursor = body.FieldCursor
			}
			return m, nil

		// --- Navigation keys: emit messages, no ctrl.Apply ---
		case key.Matches(msg, m.keys.Copy):
			if m.rightCol.IsFocused() {
				name := m.rightCol.SelectedTypeName()
				if name != "" {
					return m, func() tea.Msg {
						return messages.Copied{Content: name}
					}
				}
				return m, nil
			}
			// Left column: copy the field value at cursor.
			// Use fieldList (has full field metadata) mirroring the legacy path.
			if m.fieldList != nil && m.fieldCursor >= 0 && m.fieldCursor < len(m.fieldList) {
				item := m.fieldList[m.fieldCursor]
				val := item.Value
				if val == "" {
					val = item.Key
				}
				return m, func() tea.Msg {
					return messages.Copied{Content: val}
				}
			}
			return m, nil

		case key.Matches(msg, m.keys.Enter):
			if m.rightCol.IsFocused() {
				break
			}
			// Use fieldList for nav info (FieldRow in body lacks NavID/TargetType).
			if m.fieldList != nil && m.fieldCursor >= 0 && m.fieldCursor < len(m.fieldList) {
				item := m.fieldList[m.fieldCursor]
				if item.IsNavigable {
					targetID := item.Value
					if item.NavID != "" {
						targetID = item.NavID
					}
					res := m.res
					rt := m.resourceType
					return m, func() tea.Msg {
						return messages.RelatedNavigate{
							TargetType:     item.TargetType,
							SourceResource: res,
							SourceType:     rt,
							TargetID:       targetID,
						}
					}
				}
			}
			return m, nil

		case key.Matches(msg, m.keys.YAML):
			return m, func() tea.Msg {
				return messages.Navigate{
					Target:       messages.TargetYAML,
					Resource:     &m.res,
					ResourceType: m.resourceType,
				}
			}

		case key.Matches(msg, m.keys.JSON):
			return m, func() tea.Msg {
				return messages.Navigate{
					Target:       messages.TargetJSON,
					Resource:     &m.res,
					ResourceType: m.resourceType,
				}
			}

		case key.Matches(msg, m.keys.CloudTrail):
			if ff := resource.BuildCloudTrailFilter(m.res, m.resourceType); ff != nil {
				res := m.res
				rt := m.resourceType
				return m, func() tea.Msg {
					return messages.RelatedNavigate{
						TargetType:     "ct-events",
						SourceResource: res,
						SourceType:     rt,
						FetchFilter:    ff,
					}
				}
			}
			return m, nil
		}
	}
	return m, nil
}

// NewDetail creates a DetailModel for the given resource.
// resourceType identifies which ViewDef to use from the config (e.g. "ec2", "rds").
// By default, navProvider is resource.GetActiveNavigableFields (ACTIVE-only).
// TUI handlers that need merged DEFAULT+ACTIVE should call d.SetNavProvider(resource.GetNavigableFields).
func NewDetail(res resource.Resource, resourceType string, viewConfig *config.ViewsConfig, k keys.Map) DetailModel {
	if resourceType == "" {
		resourceType = inferDetailResourceType(res)
	}
	return DetailModel{
		resourceType:  resourceType,
		res:           res,
		viewConfig:    viewConfig,
		navProvider:   resource.GetActiveNavigableFields,
		keys:          k,
		rightColWidth: 32,
	}
}

// NewDetailWithCtrl creates a DetailModel backed by the given controller.
// The controller stack must already have ScreenDetail pushed and EnsureDetailState
// called before the first View() so that Snapshot().Body.Detail is non-nil from
// the first render.
//
// Key actions in Update route to ctrl.Apply. Enrichment findings and related
// results must be applied via ctrl.ApplyDetailFinding / ctrl.ApplyDetailRelated
// (not via SetEnrichmentFinding / ApplyRelatedResults) so the controller
// remains the single source of truth.
func NewDetailWithCtrl(res resource.Resource, resourceType string, viewConfig *config.ViewsConfig, k keys.Map, ctrl *app.Controller) DetailModel {
	if resourceType == "" {
		resourceType = inferDetailResourceType(res)
	}
	return DetailModel{
		ctrl:          ctrl,
		resourceType:  resourceType,
		res:           res,
		viewConfig:    viewConfig,
		navProvider:   resource.GetActiveNavigableFields,
		keys:          k,
		rightColWidth: 32,
	}
}

// SetNavProvider overrides the nav field provider used by buildFieldList.
// TUI construction paths call this with resource.GetNavigableFields (merged
// ACTIVE+DEFAULT) so that prod code sees all registered navigable fields.
// Test-direct paths retain the ACTIVE-only default to stay isolated from
// init-time registrations.
func (m *DetailModel) SetNavProvider(p func(string) []resource.NavigableField) {
	m.navProvider = p
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

// wave2FindingFromResource extracts the first wave-2 Finding and its companion
// AttentionDetail from r.Findings / r.AttentionDetails. Returns (nil, nil) when
// no wave-2 finding is present. Mirrors findingFromResource in tui/app_enrich_fold.go
// but is local to the views package to avoid a cross-package import cycle.
func wave2FindingFromResource(r resource.Resource) (*domain.Finding, *domain.AttentionDetail) {
	for _, f := range r.Findings {
		if strings.HasPrefix(string(f.Source), "wave2:") {
			finding := f
			var ad *domain.AttentionDetail
			if r.AttentionDetails != nil {
				if got, ok := r.AttentionDetails[f.Code]; ok && len(got.Rows) > 0 {
					adVal := got
					ad = &adVal
				}
			}
			return &finding, ad
		}
	}
	return nil, nil
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
	case messages.RelatedCheckResult:
		// Ignore results for a different resource type or source resource.
		if msg.ResourceType != m.resourceType || (msg.SourceResourceID != "" && msg.SourceResourceID != m.res.ID) {
			return m, nil
		}
		if m.ctrl != nil {
			// Controller-backed path: merge one checker result into the controller's
			// DetailState.RelatedRows by DisplayName (mirrors rightColumnModel.Update).
			errMsg := ""
			if msg.Result.Err != nil {
				errMsg = msg.Result.Err.Error()
			}
			m.ctrl.ApplyDetailRelatedResult(
				msg.DefDisplayName,
				msg.Result.TargetType,
				msg.Result.Count,
				false,
				errMsg,
				msg.Result.Approximate,
				msg.Result.FetchFilter,
			)
		}
		// Keep the local right-column model in sync in both paths: Enter/copy still
		// delegate to m.rightCol.SelectedRow/Update, so without this the visible
		// resolved row stays a loading, non-actionable placeholder even though the
		// rendered count came from DetailState.
		m.rightCol, _ = m.rightCol.Update(msg)
		return m, nil
	case messages.EnrichDetailResult:
		if m.ctrl != nil {
			// Controller-backed path: route the enriched resource AND its wave-2
			// finding to the matching detail screen BY RESOURCE ID — it may be a
			// STACKED detail, not the active one. The enriched resource carries
			// fetched data (e.g. IAM policy documents in RawStruct) that the field
			// projection and subsequent YAML/JSON opens need; dropping it would keep
			// showing the pre-enrichment resource. nil finding clears prior findings.
			ef, ad := wave2FindingFromResource(msg.EnrichedRes)
			m.ctrl.ApplyDetailEnrichmentForResource(msg.ResourceType, msg.ResourceID, msg.EnrichedRes, ef, ad)
			return m, nil
		}
		// Legacy path: only the active view, so ignore results for another resource.
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
		// Controller-backed path: route stateful keys to ctrl.Apply.
		// Navigation keys (YAML/JSON/Enter/CloudTrail/Copy) still emit messages.
		if m.ctrl != nil {
			return m.updateKeyMsgWithCtrl(msg)
		}
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
						return messages.Copied{Content: name}
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
					return messages.Copied{Content: val}
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
					return messages.RelatedCheckStarted{
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
						return messages.RelatedNavigate{
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
				return messages.Navigate{
					Target:       messages.TargetYAML,
					Resource:     &m.res,
					ResourceType: m.resourceType,
				}
			}
		case key.Matches(msg, m.keys.JSON):
			return m, func() tea.Msg {
				return messages.Navigate{
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
					return messages.RelatedNavigate{
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
				// Skip non-selectable rows (section headers, spacers).
				for m.fieldCursor < len(m.fieldList)-1 && (m.fieldList[m.fieldCursor].IsSection || m.fieldList[m.fieldCursor].IsSpacer) {
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
				// Skip non-selectable rows (section headers, spacers).
				for m.fieldCursor > 0 && (m.fieldList[m.fieldCursor].IsSection || m.fieldList[m.fieldCursor].IsSpacer) {
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
