package views

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// applySortAndFilter re-applies filter and then sorts the filtered results.
func (m *ResourceListModel) applySortAndFilter() {
	m.applyFilter()
	m.sortFiltered()
	m.styledRowCache = nil
}

// updateSortColKey sets m.sortColKey to the canonical column key for the current
// sort mode on the current resource type. This is the single source of truth for
// which column header receives the sort glyph (§6).
//
// Matching is exact against resolved column keys:
//   - SortName → first column whose key/title is "name" or contains "Name"
//   - SortID   → first column whose key contains "id" or title contains "ID"
//   - SortAge  → first column whose key or path matches isAgeKey
//
// If no column matches, sortColKey is set to "" (no glyph rendered).
func (m *ResourceListModel) updateSortColKey() {
	cols := m.resolveColumns()
	for _, c := range cols {
		switch m.sort {
		case SortName:
			if strings.EqualFold(c.key, "name") || strings.Contains(strings.ToLower(c.title), "name") {
				m.sortColKey = colSortKey(c)
				return
			}
		case SortID:
			if strings.Contains(strings.ToLower(c.key), "id") || strings.Contains(c.title, "ID") {
				m.sortColKey = colSortKey(c)
				return
			}
		case SortAge:
			if isAgeKey(c.key) || isAgeKey(c.path) {
				m.sortColKey = colSortKey(c)
				return
			}
		}
	}
	m.sortColKey = ""
}

func (m ResourceListModel) exactRelatedTargetID() (string, bool) {
	if len(m.relatedIDSet) != 1 {
		return "", false
	}
	for id := range m.relatedIDSet {
		if id == "" {
			return "", false
		}
		return id, true
	}
	return "", false
}

// SetFilter applies a filter; cursor resets to 0.
func (m *ResourceListModel) SetFilter(text string) {
	m.filterText = text
	m.applyFilter()
	m.styledRowCache = nil
	m.scroll.SetCursor(0)
}

// SetPendingFilter stores filter text to be applied when resources are loaded.
// Used by related-resource navigation to pre-filter the list on first load.
func (m *ResourceListModel) SetPendingFilter(text string) {
	m.pendingFilter = text
}

// SetFetchFilter sets server-side filter parameters used for both initial fetch and load-more.
// When non-nil, the list bypasses relatedIDSet and uses a registered FilteredPaginatedFetcher.
func (m *ResourceListModel) SetFetchFilter(filter map[string]string) {
	m.fetchFilter = filter
}

// FetchFilter returns the server-side filter parameters used for initial fetch and load-more.
func (m ResourceListModel) FetchFilter() map[string]string {
	return m.fetchFilter
}

// SetRelatedIDFilter constrains the list to an exact set of resource IDs.
// Used by related-resource navigation flows to preserve checker result IDs
// even when the destination type must be fetched first.
func (m *ResourceListModel) SetRelatedIDFilter(ids []string) {
	if len(ids) == 0 {
		m.relatedIDSet = nil
		return
	}
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		set[id] = struct{}{}
	}
	m.relatedIDSet = set
}

// SetAutoOpenSingleDetail configures one-shot auto-navigation to detail when
// a ResourcesLoaded update leaves exactly one filtered row.
func (m *ResourceListModel) SetAutoOpenSingleDetail(v bool) {
	m.autoOpenSingleDetail = v
}

// GetFilter returns the current filter text.
func (m *ResourceListModel) GetFilter() string {
	return m.filterText
}

// ShortName returns the resource type's short name (for cross-view state sync).
func (m *ResourceListModel) ShortName() string { return m.typeDef.ShortName }

// LoadedCount returns the total number of resources currently loaded into this view
// (sum of all pages fetched so far).
func (m *ResourceListModel) LoadedCount() int { return len(m.allResources) }

// IsTruncated reports whether more pages remain unfetched on the AWS side.
func (m *ResourceListModel) IsTruncated() bool {
	return m.pagination != nil && m.pagination.IsTruncated
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
// When the resource type defines a CopyField, that field value is used instead.
func (m ResourceListModel) CopyContent() (string, string) {
	if r := m.SelectedResource(); r != nil {
		if m.typeDef.CopyField != "" {
			if val, ok := r.Fields[m.typeDef.CopyField]; ok && val != "" {
				return val, "Copied: " + val
			}
		}
		return r.ID, "Copied: " + r.ID
	}
	return "", ""
}

// GetHelpContext returns the appropriate help context based on resource type and pagination state.
func (m ResourceListModel) GetHelpContext() HelpContext {
	truncated := m.pagination != nil && m.pagination.IsTruncated
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
	c := m.scroll.Cursor()
	if c >= 0 && c < len(m.filteredResources) {
		r := m.filteredResources[c]
		return &r
	}
	return nil
}

// ResourceType returns the short name of the resource type (e.g., "ec2", "secrets").
func (m ResourceListModel) ResourceType() string {
	return m.typeDef.ShortName
}

// ParentContext returns the parent context map, or nil for top-level lists.
func (m ResourceListModel) ParentContext() map[string]string {
	return m.parentContext
}

// AllResources returns all loaded resources (across all pages).
func (m ResourceListModel) AllResources() []resource.Resource {
	return m.allResources
}

// PaginationState returns the current pagination metadata, or nil if unpaginated.
func (m ResourceListModel) PaginationState() *resource.PaginationMeta {
	return m.pagination
}

// CursorPosition returns the current cursor index in the filtered list.
func (m ResourceListModel) CursorPosition() int {
	return m.scroll.Cursor()
}

// SortState returns the current sort field and direction.
func (m ResourceListModel) SortState() (SortField, bool) {
	return m.sort, m.sortAsc
}

// HScrollOffset returns the current horizontal scroll offset.
func (m ResourceListModel) HScrollOffset() int {
	return m.hScrollOffset
}

// FilterText returns the current filter text.
func (m ResourceListModel) FilterText() string {
	return m.filterText
}

// AttentionOnly returns the current state of the ctrl+z attention filter.
func (m ResourceListModel) AttentionOnly() bool {
	return m.attentionOnly
}

// handleChildKey iterates through the typeDef's Children looking for a match
// on keyName. If found, checks DrillCondition, builds context, and returns
// an EnterChildViewMsg command. Returns the model and nil cmd if no child matched.
func (m ResourceListModel) handleChildKey(keyName string, r *resource.Resource) (ResourceListModel, tea.Cmd) {
	for _, child := range m.typeDef.Children {
		if child.Key != keyName {
			continue
		}
		// Check drill condition
		if child.DrillCondition != nil && !child.DrillCondition(*r) {
			if child.DrillBlockMessage != "" {
				msg := child.DrillBlockMessage
				return m, func() tea.Msg {
					return messages.FlashMsg{Text: msg, IsError: true}
				}
			}
			continue
		}
		// Build context and create message
		ctx := m.buildChildContext(child, r)
		displayName := ctx[child.DisplayNameKey]
		childType := child.ChildType

		return m, func() tea.Msg {
			return messages.EnterChildViewMsg{
				ChildType:     childType,
				ParentContext: ctx,
				DisplayName:   displayName,
			}
		}
	}
	return m, nil
}

// buildChildContext resolves ContextKeys for a ChildViewDef given the selected resource.
// Resolution rules:
//   - "ID"         → r.ID
//   - "Name"       → r.Name
//   - "@parent.x"  → m.parentContext["x"]
//   - anything else → r.Fields[key]
func (m ResourceListModel) buildChildContext(child resource.ChildViewDef, r *resource.Resource) map[string]string {
	ctx := make(map[string]string, len(child.ContextKeys))
	for param, source := range child.ContextKeys {
		switch {
		case source == "ID":
			ctx[param] = r.ID
		case source == "Name":
			ctx[param] = r.Name
		case strings.HasPrefix(source, "@parent."):
			parentKey := strings.TrimPrefix(source, "@parent.")
			if m.parentContext != nil {
				ctx[param] = m.parentContext[parentKey]
			}
		default:
			ctx[param] = r.Fields[source]
		}
	}
	return ctx
}

// ClearLoading clears the loading state so the view no longer shows a spinner.
func (m *ResourceListModel) ClearLoading() {
	m.loading = false
	m.loadingMore = false
}

// FrameTitle returns e.g. "ec2(42)" or "ec2(3/42)" when filtered.
// For child views with a display name, shows that name instead of the short name.
// During loading, returns just the name without count.
// When pagination indicates truncation:
//   - "ec2(200+)"             — truncated, no filter
//   - "ec2(200+ loading...)"  — truncated, loadingMore in progress
//   - "ec2(15/200+)"          — truncated, filter active
func (m ResourceListModel) FrameTitle() string {
	name := m.typeDef.ShortName
	if m.typeDef.ListTitle != "" {
		name = m.typeDef.ListTitle
	}
	if m.displayName != "" {
		name = m.displayName
	}
	if m.loading {
		return name
	}

	total := len(m.allResources)
	filtered := len(m.filteredResources)
	truncated := m.pagination != nil && m.pagination.IsTruncated

	totalStr := itoa(total)
	if truncated {
		totalStr = itoa(total) + "+"
	}

	if m.loadingMore {
		return name + "(" + totalStr + " loading...)"
	}

	if m.filterText != "" && filtered != total {
		title := name + "(" + itoa(filtered) + "/" + totalStr + ")"
		if m.titleSuffix != "" {
			title += m.titleSuffix
		}
		if m.attentionOnly {
			title += " [!]"
		}
		return title
	}
	title := name + "(" + totalStr + ")"
	if m.titleSuffix != "" {
		title += m.titleSuffix
	}
	if m.attentionOnly {
		title += " [!]"
	}
	return title
}

// BottomHints implements Hintable for ResourceListModel.
func (m ResourceListModel) BottomHints() []layout.KeyHint {
	var hints []layout.KeyHint

	// Child/related lists show esc Back first
	if m.escPops {
		hints = append(hints, layout.KeyHint{Key: "esc", Desc: "Back"})
	}

	// Enter-child awareness
	var enterChild *resource.ChildViewDef
	for i := range m.typeDef.Children {
		if m.typeDef.Children[i].Key == "enter" {
			enterChild = &m.typeDef.Children[i]
			break
		}
	}
	if enterChild != nil {
		// Evaluate DrillCondition against selected resource — when the condition
		// is false (e.g., S3 file, SFN Express), enter goes to detail, not child.
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

	// Reveal key (Secrets Manager, etc.)
	if resource.HasRevealFetcher(m.typeDef.ShortName) {
		hints = append(hints, layout.KeyHint{Key: "x", Desc: "Reveal"})
	}

	hints = append(hints, layout.KeyHint{Key: "y", Desc: "YAML"})

	// Non-enter child keys (e, L, R, etc.)
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

	if m.typeDef.CloudTrailKey != "" && m.parentContext == nil {
		hints = append(hints, layout.KeyHint{Key: "t", Desc: "CloudTrail"})
	}

	hints = append(hints, layout.KeyHint{Key: "ctrl+r", Desc: "Refresh"})
	hints = append(hints, layout.KeyHint{Key: "ctrl+z", Desc: "Only !"})

	// Pagination "more" hint
	if m.pagination != nil && m.pagination.IsTruncated {
		hints = append(hints, layout.KeyHint{Key: "m", Desc: "More"})
	}

	return hints
}

// SetTitleSuffix sets a suffix appended to the frame title after count rendering.
func (m *ResourceListModel) SetTitleSuffix(s string) {
	m.titleSuffix = s
}

// SetDisplayName overrides the base title name used in FrameTitle.
func (m *ResourceListModel) SetDisplayName(name string) {
	m.displayName = name
}

// SetEscPops configures Esc behavior for this list.
// Related-navigation lists set this to true so Esc returns to source detail view.
func (m *ResourceListModel) SetEscPops(v bool) {
	m.escPops = v
}

// EscPops reports whether Esc should pop this view immediately.
func (m ResourceListModel) EscPops() bool {
	return m.escPops
}

// applyFilter filters allResources into filteredResources.
func (m *ResourceListModel) applyFilter() {
	base := m.allResources
	if len(m.relatedIDSet) > 0 {
		subset := make([]resource.Resource, 0, len(base))
		for _, r := range base {
			if _, ok := m.relatedIDSet[r.ID]; ok {
				subset = append(subset, r)
			}
		}
		base = subset
	}
	result := FilterResources(m.filterText, base)

	// §7 attention filter — hide dim rows when toggle is on.
	if m.attentionOnly {
		kept := make([]resource.Resource, 0, len(result))
		for _, r := range result {
			if !styles.IsDimRowColor(r.Status) {
				kept = append(kept, r)
			}
		}
		result = kept
	}

	m.filteredResources = result
	m.scroll.SetTotal(len(m.filteredResources))
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
			strings.Contains(strings.ToLower(r.Name), q) ||
			strings.Contains(strings.ToLower(r.Status), q) {
			result = append(result, r)
			continue
		}
		for _, v := range r.Fields {
			if strings.Contains(strings.ToLower(v), q) {
				result = append(result, r)
				break
			}
		}
	}
	return result
}
