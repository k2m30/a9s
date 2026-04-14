package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
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

// updateSortColKey sets m.sortColKey to the column sort key for the current
// sort column index. This is the single source of truth for which column
// header receives the sort glyph (§6).
func (m *ResourceListModel) updateSortColKey() {
	if m.sortColIdx == SortColNone {
		m.sortColKey = ""
		return
	}
	cols := m.resolveColumns()
	if m.sortColIdx >= len(cols) {
		m.sortColKey = ""
		return
	}
	m.sortColKey = colSortKey(cols[m.sortColIdx])
}

// handleSortByCol checks all 10 positional sort bindings against msg.
// Key "1" = absolute column 0, "2" = column 1, …, "0" = column 9.
// Column numbers are absolute: key "7" always sorts the 7th column regardless
// of horizontal scroll. Keys for columns that don't exist are absorbed.
// Returns true if a sort binding was matched (key consumed).
func (m *ResourceListModel) handleSortByCol(msg tea.KeyMsg) bool {
	for colIdx, kb := range m.keys.SortByCol {
		if key.Matches(msg, kb) {
			cols := m.resolveColumns()
			if colIdx >= len(cols) {
				return true // key pressed but no such column — absorb it
			}
			if m.sortColIdx == colIdx {
				m.sortAsc = !m.sortAsc
			} else {
				m.sortColIdx = colIdx
				m.sortAsc = true
			}
			m.updateSortColKey()
			m.applySortAndFilter()
			return true
		}
	}
	return false
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

// SortState returns the current sort column index and direction.
func (m ResourceListModel) SortState() (int, bool) {
	return m.sortColIdx, m.sortAsc
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
// AttentionOnly reports whether the ctrl+z attention filter is active.
func (m ResourceListModel) AttentionOnly() bool {
	return m.IsEnabled()
}

// IssueCount returns the number of resources with issue status (red/yellow).
// Recomputed whenever allResources changes via applySortAndFilter().
func (m ResourceListModel) IssueCount() int {
	return m.issueCount
}

// SetShowIssueBadge enables the "issues:N" badge in FrameTitle.
// Set by main menu navigation for top-level resource lists.
func (m *ResourceListModel) SetShowIssueBadge(v bool) {
	m.showIssueBadge = v
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

	isAttention := m.IsEnabled()
	hasTextFilter := m.filterText != "" && filtered != total
	ic := m.issueCount
	issueStr := itoa(ic)
	if truncated && ic > 0 {
		issueStr = itoa(ic) + "+"
	}

	var title string
	switch {
	case hasTextFilter && isAttention:
		// text filter + ctrl+z: name(filtered of total) [!]
		// filtered is already the intersection of both filters
		title = name + "(" + itoa(filtered) + " of " + totalStr + ")"
	case hasTextFilter:
		// text filter only: name(filtered/total) — no issue badge
		title = name + "(" + itoa(filtered) + "/" + totalStr + ")"
	case isAttention:
		// ctrl+z only: name(N of total) [!]
		attentionVisible := len(m.filteredResources)
		title = name + "(" + itoa(attentionVisible) + " of " + totalStr + ")"
	default:
		// No filters: show issue count if > 0 and badge is enabled.
		if ic > 0 && m.showIssueBadge {
			issueWord := "issues"
			if ic == 1 {
				issueWord = "issue"
			}
			title = name + "(" + totalStr + "/" + issueStr + " " + issueWord + ")"
		} else {
			title = name + "(" + totalStr + ")"
		}
	}

	if m.titleSuffix != "" {
		title += m.titleSuffix
	}
	if isAttention {
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
	hints = append(hints, layout.KeyHint{Key: "J", Desc: "JSON"})

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

	// §7 attention filter — show only attention-worthy rows when toggle is on.
	// "Attention-worthy" = issue-colored rows (stopped/failed/pending/etc. that
	// feed the "N issues" badge) PLUS ct-event severities (ct-attention /
	// ct-danger). Previously this used !IsDimRowColor which kept healthy
	// running rows too, producing a "25 of 27 [!]" display when the badge
	// claimed "11 issues" on the same data — a user-visible inconsistency.
	// The badge-count invariant: on any resource list showing "N issues",
	// ctrl+z reveals exactly N rows.
	if m.IsEnabled() {
		kept := make([]resource.Resource, 0, len(result))
		for _, r := range result {
			if m.typeDef.ResolveColor(r).IsIssue() {
				kept = append(kept, r)
			}
		}
		result = kept
	}

	m.filteredResources = result
	m.scroll.SetTotal(len(m.filteredResources))

	// Recompute issue count from allResources (not filtered — represents the full page).
	ic := 0
	for _, r := range m.allResources {
		if m.typeDef.ResolveColor(r).IsIssue() {
			ic++
		}
	}
	m.issueCount = ic

	// Recompute visible finding count from filteredResources (severity-agnostic).
	vfc := 0
	for _, r := range m.filteredResources {
		if _, ok := m.findingsByID[r.ID]; ok {
			vfc++
		}
	}
	m.visibleFindingCount = vfc
}

// renderEnrichmentBanner returns a styled banner line when all three conditions hold:
//  1. len(findingsByID) > 0 (severity-agnostic — includes ~ findings that don't count in menu badge)
//  2. enrichmentRanThisSession is true (Wave 2 completed for this type)
//  3. visibleIssueCount == 0 (no issue-colored rows visible in the filtered set)
//
// Returns an empty string when the banner should not be shown.
func (m *ResourceListModel) renderEnrichmentBanner() string {
	findingCount := len(m.findingsByID)
	if findingCount == 0 || !m.enrichmentRanThisSession {
		return ""
	}
	// Compute visible issue count from filtered resources.
	visibleIssueCount := 0
	for _, r := range m.filteredResources {
		if m.typeDef.ResolveColor(r).IsIssue() {
			visibleIssueCount++
		}
	}
	if visibleIssueCount > 0 {
		return ""
	}

	// Format N or N+ depending on truncation.
	n := itoa(findingCount)
	if m.enrichmentTruncated {
		n += "+"
	}

	// Select long or short text based on visibleFindingCount.
	var text string
	if m.visibleFindingCount > 0 {
		text = "⚠ " + n + " issues detected by background checks"
	} else {
		text = "⚠ " + n + " issues detected by background checks — not visible on this page"
	}

	return styles.EnrichmentBannerStyle.Render(text)
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
