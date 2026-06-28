package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// handleSortByCol checks all 10 positional sort bindings against msg.
// Key "1" = absolute column 0, "2" = column 1, …, "0" = column 9.
// Returns true if a sort binding was matched (key consumed).
func (m *ResourceListModel) handleSortByCol(msg tea.KeyMsg) bool {
	for colIdx, kb := range m.keys.SortByCol {
		if key.Matches(msg, kb) {
			// Use ResolveColumnsForType so that viewConfig and fallback typeDefs
			// are applied — this makes sort-by-key consistent with buildListBody.
			cols := m.ctrl.ResolveColumnsForType(m.typeDef.ShortName)
			if colIdx >= len(cols) {
				return true // key pressed but no such column — absorb it
			}
			// Derive sort key the same way colSortKey does for listCol:
			// prefer Key, then Path, then Title.
			col := cols[colIdx]
			colKey := col.Key
			if colKey == "" {
				colKey = col.Path
			}
			if colKey == "" {
				colKey = col.Title
			}
			m.ctrl.Apply(app.Action{Kind: app.ActionSort, Arg: colKey})
			m.styledRowCache = nil
			return true
		}
	}
	return false
}

// SetFilter applies a filter; cursor resets to 0.
func (m *ResourceListModel) SetFilter(text string) {
	m.ctrl.Apply(app.Action{Kind: app.ActionSetFilter, Arg: text})
	m.styledRowCache = nil
}

// SetPendingFilter stores filter text to be applied when resources are loaded.
// In the controller-driven path this applies the filter immediately (there is no
// pending state — the controller owns the filter).
func (m *ResourceListModel) SetPendingFilter(text string) {
	if text != "" {
		m.ctrl.Apply(app.Action{Kind: app.ActionSetFilter, Arg: text})
		m.styledRowCache = nil
	}
}

// SetFetchFilter sets server-side filter parameters for initial fetch and load-more.
func (m *ResourceListModel) SetFetchFilter(filter map[string]string) {
	m.ctrl.PatchListFetchFilter(filter)
}

// FetchFilter returns the server-side filter parameters used for initial fetch and load-more.
func (m ResourceListModel) FetchFilter() map[string]string {
	return m.ctrl.GetListFetchFilter()
}

// SetRelatedIDFilter constrains the list to an exact set of resource IDs.
func (m *ResourceListModel) SetRelatedIDFilter(ids []string) {
	m.ctrl.PatchListRelatedIDSet(ids)
	m.styledRowCache = nil
}

// SetReapplyChecker registers the originating RelatedDef.Checker for re-application
// on subsequent resource loads.
func (m *ResourceListModel) SetReapplyChecker(checker resource.RelatedChecker, src resource.Resource) {
	m.ctrl.PatchListReapplyChecker(checker, src)
	m.styledRowCache = nil
}

// ReapplyCheckerAgainst re-runs the carried checker against newPage and merges
// any newly matched IDs into the related-ID filter set.
func (m *ResourceListModel) ReapplyCheckerAgainst(newPage []resource.Resource) {
	m.ctrl.ApplyReapplyCheckerAgainst(newPage)
	m.styledRowCache = nil
}

// RelatedIDFilterSize returns the size of the current related-ID filter.
func (m *ResourceListModel) RelatedIDFilterSize() int {
	return len(m.ctrl.GetListRelatedIDSet())
}

// VisibleResources returns the currently-visible (post-filter, post-sort)
// resources by delegating to the controller snapshot.
func (m *ResourceListModel) VisibleResources() []resource.Resource {
	snap := m.ctrl.Snapshot()
	if snap.Body.List == nil {
		return nil
	}
	// Reconstruct resource slice from the snapshot rows via the cache.
	// For test compatibility, return all cached resources filtered through
	// the same pipeline the controller uses.
	return m.ctrl.GetListVisibleResources()
}

// AppendResourcesForTest appends resources to the controller cache and
// re-triggers the filter pipeline.
func (m *ResourceListModel) AppendResourcesForTest(page []resource.Resource) {
	m.ctrl.ApplyResourcesLoaded(m.typeDef.ShortName, page, nil, true)
	m.styledRowCache = nil
}

// SetAutoOpenSingleDetail configures one-shot auto-navigation to detail when
// a ResourcesLoaded update leaves exactly one filtered row.
func (m *ResourceListModel) SetAutoOpenSingleDetail(v bool) {
	m.ctrl.SetListAutoOpenSingle(v)
}

// GetFilter returns the current filter text.
func (m *ResourceListModel) GetFilter() string {
	return m.ctrl.GetListFilter()
}

// ShortName returns the resource type's short name.
func (m *ResourceListModel) ShortName() string { return m.typeDef.ShortName }

// LoadedCount returns the total number of resources currently loaded into this view.
func (m *ResourceListModel) LoadedCount() int { return len(m.ctrl.GetListAllResources()) }

// IsTruncated reports whether more pages remain unfetched on the AWS side.
func (m *ResourceListModel) IsTruncated() bool {
	truncated, _ := m.ctrl.GetListPagination()
	return truncated
}

// SetSize updates dimensions.
func (m *ResourceListModel) SetSize(w, h int) {
	if m.width != w {
		m.styledRowCache = nil
	}
	m.width = w
	m.height = h
}

// CopyContent returns the selected resource's ID for clipboard copy.
func (m ResourceListModel) CopyContent() (string, string) {
	r, ok := m.ctrl.ListSelected()
	if !ok {
		return "", ""
	}
	if m.typeDef.CopyField != "" {
		if val, ok := r.Fields[m.typeDef.CopyField]; ok && val != "" {
			return val, "Copied: " + val
		}
	}
	return r.ID, "Copied: " + r.ID
}

// GetHelpContext returns the appropriate help context based on resource type and pagination state.
func (m ResourceListModel) GetHelpContext() HelpContext {
	truncated, _ := m.ctrl.GetListPagination()
	hasReveal := resource.HasRevealFetcher(m.typeDef.ShortName)
	if hasReveal {
		if truncated {
			return HelpFromRevealListPaginated
		}
		return HelpFromRevealList
	}
	if truncated {
		return HelpFromResourceListPaginated
	}
	return HelpFromResourceList
}

// SelectedResource returns the resource at cursor, or nil.
func (m ResourceListModel) SelectedResource() *resource.Resource {
	r, ok := m.ctrl.ListSelected()
	if !ok {
		return nil
	}
	return &r
}

// ResourceType returns the short name of the resource type (e.g., "ec2", "secrets").
func (m ResourceListModel) ResourceType() string {
	return m.typeDef.ShortName
}

// ParentContext returns the parent context map, or nil for top-level lists.
func (m ResourceListModel) ParentContext() map[string]string {
	return m.ctrl.GetListParentContext()
}

// AllResources returns all loaded resources (across all pages).
func (m ResourceListModel) AllResources() []resource.Resource {
	return m.ctrl.GetListAllResources()
}

// PaginationState returns the current pagination metadata, or nil if unpaginated.
func (m ResourceListModel) PaginationState() *resource.PaginationMeta {
	truncated, cursor := m.ctrl.GetListPagination()
	if !truncated && cursor == "" {
		return nil
	}
	return &resource.PaginationMeta{
		IsTruncated: truncated,
		NextToken:   cursor,
	}
}

// CursorPosition returns the current cursor index in the filtered list.
func (m ResourceListModel) CursorPosition() int {
	return m.ctrl.GetListSelectedRow()
}

// SortState returns the current sort column index and direction.
// Returns (-1, true) when no sort is active.
func (m ResourceListModel) SortState() (int, bool) {
	col, dir := m.ctrl.GetListSort()
	if col == "" {
		return -1, true
	}
	// Translate column key back to index using the same column set and sort-key
	// derivation as handleSortByCol (Key → Path → Title priority).
	cols := m.ctrl.ResolveColumnsForType(m.typeDef.ShortName)
	for i, c := range cols {
		// Derive effective sort key from ColumnDef the same way handleSortByCol does.
		effectiveKey := c.Key
		if effectiveKey == "" {
			effectiveKey = c.Path
		}
		if effectiveKey == "" {
			effectiveKey = c.Title
		}
		if effectiveKey == col {
			return i, dir != "desc"
		}
	}
	return -1, true
}

// HScrollOffset returns the current horizontal scroll offset.
func (m ResourceListModel) HScrollOffset() int {
	return m.ctrl.GetListScrollX()
}

// FilterText returns the current filter text.
func (m ResourceListModel) FilterText() string {
	return m.ctrl.GetListFilter()
}

// AttentionOnly reports whether the ctrl+z attention filter is active.
func (m ResourceListModel) AttentionOnly() bool {
	return m.ctrl.GetListAttentionOnly()
}

// IssueCount returns the number of resources with issue status (red/yellow).
func (m ResourceListModel) IssueCount() int {
	return m.ctrl.GetListIssueCount()
}

// SetShowIssueBadge enables the "issues:N" badge in FrameTitle.
func (m *ResourceListModel) SetShowIssueBadge(v bool) {
	m.ctrl.PatchListShowIssueBadge(v)
}

// enterChildFor returns the Children entry registered under Key="enter" for
// this resource type, or nil if none is registered or its DrillCondition
// vetoes the given row.
func (m ResourceListModel) enterChildFor(r resource.Resource) *resource.ChildViewDef {
	for i := range m.typeDef.Children {
		c := &m.typeDef.Children[i]
		if c.Key != "enter" {
			continue
		}
		if c.DrillCondition != nil && !c.DrillCondition(r) {
			return nil
		}
		return c
	}
	return nil
}

// handleChildKey iterates through the typeDef's Children looking for a match
// on keyName. If found, checks DrillCondition, builds context, and returns
// an EnterChildViewMsg command. Returns the model and nil cmd if no child matched.
func (m ResourceListModel) handleChildKey(keyName string, r *resource.Resource) (ResourceListModel, tea.Cmd) {
	for _, child := range m.typeDef.Children {
		if child.Key != keyName {
			continue
		}
		if child.DrillCondition != nil && !child.DrillCondition(*r) {
			if child.DrillBlockMessage != "" {
				msg := child.DrillBlockMessage
				return m, func() tea.Msg {
					return messages.Flash{Text: msg, IsError: true}
				}
			}
			continue
		}
		ctx := m.buildChildContext(child, r)
		displayName := ctx[child.DisplayNameKey]
		childType := child.ChildType

		return m, func() tea.Msg {
			return messages.EnterChildView{
				ChildType:     childType,
				ParentContext: ctx,
				DisplayName:   displayName,
			}
		}
	}
	return m, nil
}

// buildChildContext resolves ContextKeys for a ChildViewDef given the selected resource.
func (m ResourceListModel) buildChildContext(child resource.ChildViewDef, r *resource.Resource) map[string]string {
	ctx := make(map[string]string, len(child.ContextKeys))
	pc := m.ctrl.GetListParentContext()
	for param, source := range child.ContextKeys {
		switch {
		case source == "ID":
			ctx[param] = r.ID
		case source == "Name":
			ctx[param] = r.Name
		case strings.HasPrefix(source, "@parent."):
			parentKey := strings.TrimPrefix(source, "@parent.")
			if pc != nil {
				ctx[param] = pc[parentKey]
			}
		default:
			ctx[param] = r.Fields[source]
		}
	}
	return ctx
}

// ClearLoading clears the loading and load-more flags on the controller's
// top list screen. Called by the app-level error handler when a load-more
// fetch fails, so the title reverts from "name(N+) loading..." to "name(N+)".
func (m *ResourceListModel) ClearLoading() {
	m.ctrl.ClearListLoading()
}

// FrameTitle returns the frame title by delegating to the controller.
func (m ResourceListModel) FrameTitle() string {
	return m.ctrl.ListFrameTitle()
}

// BottomHints implements Hintable for ResourceListModel.
func (m ResourceListModel) BottomHints() []layout.KeyHint {
	var hints []layout.KeyHint

	if m.ctrl.GetListEscPops() {
		hints = append(hints, layout.KeyHint{Key: "esc", Desc: "Back"})
	}

	var enterChild *resource.ChildViewDef
	for i := range m.typeDef.Children {
		if m.typeDef.Children[i].Key == "enter" {
			enterChild = &m.typeDef.Children[i]
			break
		}
	}
	if enterChild != nil {
		showEnterChild := true
		if enterChild.DrillCondition != nil {
			sel := m.SelectedResource()
			showEnterChild = sel != nil && enterChild.DrillCondition(*sel)
		}
		if showEnterChild {
			desc := enterChild.ChildType
			if ct := resource.GetChildType(enterChild.ChildType); ct != nil {
				desc = ct.Name
			}
			hints = append(hints, layout.KeyHint{Key: "enter", Desc: desc})
			hints = append(hints, layout.KeyHint{Key: "d", Desc: "Detail"})
		}
	}

	if resource.HasRevealFetcher(m.typeDef.ShortName) {
		hints = append(hints, layout.KeyHint{Key: "x", Desc: "Reveal"})
	}

	hints = append(hints, layout.KeyHint{Key: "y", Desc: "YAML"})
	hints = append(hints, layout.KeyHint{Key: "J", Desc: "JSON"})

	for _, child := range m.typeDef.Children {
		if child.Key == "enter" {
			continue
		}
		desc := child.ChildType
		if ct := resource.GetChildType(child.ChildType); ct != nil {
			desc = ct.Name
		}
		hints = append(hints, layout.KeyHint{Key: child.Key, Desc: desc})
	}

	if m.typeDef.CloudTrailKey != "" && m.ctrl.GetListParentContext() == nil {
		hints = append(hints, layout.KeyHint{Key: "t", Desc: "CloudTrail"})
	}

	hints = append(hints, layout.KeyHint{Key: "ctrl+r", Desc: "Refresh"})
	hints = append(hints, layout.KeyHint{Key: "ctrl+z", Desc: "Only !"})

	if truncated, _ := m.ctrl.GetListPagination(); truncated {
		hints = append(hints, layout.KeyHint{Key: "m", Desc: "More"})
	}

	return hints
}

// SetTitleSuffix sets a suffix appended to the frame title after count rendering.
func (m *ResourceListModel) SetTitleSuffix(s string) {
	m.ctrl.PatchListTitleSuffix(s)
}

// SetDisplayName overrides the base title name used in FrameTitle.
func (m *ResourceListModel) SetDisplayName(name string) {
	m.ctrl.PatchListDisplayName(name)
}

// SetEscPops configures Esc behavior for this list.
func (m *ResourceListModel) SetEscPops(v bool) {
	m.ctrl.PatchListEscPops(v)
}

// EscPops reports whether Esc should pop this view immediately.
func (m ResourceListModel) EscPops() bool {
	return m.ctrl.GetListEscPops()
}

// FilterResources returns resources matching the query (case-insensitive).
// Exported so tests can call it directly.
func FilterResources(query string, resources []resource.Resource) []resource.Resource {
	if query == "" {
		return resources
	}
	q := strings.ToLower(query)
	result := make([]resource.Resource, 0)
	for _, r := range resources {
		if strings.Contains(strings.ToLower(r.ID), q) ||
			strings.Contains(strings.ToLower(r.Name), q) {
			result = append(result, r)
			continue
		}
		matched := false
		for _, v := range r.Fields {
			if strings.Contains(strings.ToLower(v), q) {
				matched = true
				break
			}
		}
		if matched {
			result = append(result, r)
			continue
		}
		for _, f := range r.Findings {
			if strings.Contains(strings.ToLower(f.Phrase), q) {
				result = append(result, r)
				break
			}
		}
	}
	return result
}

// Toggle flips the attention-only filter.
func (m *ResourceListModel) Toggle() {
	m.ctrl.Apply(app.Action{Kind: app.ActionToggleAttention})
}

// SetEnabled sets the attention-only filter to the given value without toggling.
func (m *ResourceListModel) SetEnabled(v bool) {
	if v != m.ctrl.GetListAttentionOnly() {
		m.ctrl.Apply(app.Action{Kind: app.ActionToggleAttention})
	}
	m.styledRowCache = nil
}

// IsEnabled reports whether the attention-only filter is active.
func (m ResourceListModel) IsEnabled() bool {
	return m.ctrl.GetListAttentionOnly()
}

