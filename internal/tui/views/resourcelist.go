package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// ResourceListModel is a tea.Model for the resource table view.
type ResourceListModel struct {
	typeDef    resource.ResourceTypeDef
	viewConfig *config.ViewsConfig

	allResources      []resource.Resource
	filteredResources []resource.Resource

	scroll        ScrollState
	hScrollOffset int

	sort       SortField
	sortAsc    bool
	sortColKey string // exact column key carrying the sort glyph; set when sort changes (§6)

	filterText           string
	attentionOnly        bool                // §7: ctrl+z toggle — when true, hide dim/neutral rows
	pendingFilter        string              // auto-applied after ResourcesLoadedMsg arrives
	relatedIDSet         map[string]struct{} // optional exact-ID prefilter (used by related navigation)
	fetchFilter          map[string]string   // server-side filter params for filtered paginated fetcher
	autoOpenSingleDetail bool                // when true, ResourcesLoaded auto-navigates to detail if exactly one row remains
	parentContext        map[string]string   // context from parent view for child fetchers
	displayName          string              // custom display name for frame title
	titleSuffix          string              // optional suffix appended after count, e.g. " -- i-abc (web)"
	escPops              bool                // when true, Esc pops view instead of clearing filter first

	pagination  *resource.PaginationMeta // nil = unpaginated
	loadingMore bool                     // true while fetching next page

	loading bool
	spinner spinner.Model

	width  int
	height int
	keys   keys.Map

	// styledRowCache caches fully styled row strings (with cursor highlight
	// or status color applied). On cursor move, only the old and new cursor
	// positions are invalidated. Full invalidation happens when data, filter,
	// sort, width, or hScroll changes.
	styledRowCache map[int]string
}

// NewResourceList creates a ResourceListModel in loading state.
func NewResourceList(typeDef resource.ResourceTypeDef, viewConfig *config.ViewsConfig, k keys.Map) ResourceListModel {
	sp := spinner.New()
	m := ResourceListModel{
		typeDef:    typeDef,
		viewConfig: viewConfig,
		loading:    true,
		spinner:    sp,
		keys:       k,
	}
	// ct-events: default sort is by event_time DESC (newest first).
	// Only apply when a viewConfig is present (full app mode); unit tests
	// that pass nil viewConfig work with synthetic data and are not affected.
	if typeDef.ShortName == "ct-events" && viewConfig != nil {
		m.sort = SortAge
		m.sortAsc = false
		m.sortColKey = "time" // §6: TIME column is bound to SortAge for ct-events
	}
	return m
}

// NewChildResourceList creates a ResourceListModel for a child resource type.
// parentCtx provides parameters from the parent view (e.g., bucket name, zone ID).
// displayName is used for the frame title instead of the type's ShortName.
func NewChildResourceList(childType resource.ResourceTypeDef, parentCtx map[string]string, displayName string, viewConfig *config.ViewsConfig, k keys.Map) ResourceListModel {
	sp := spinner.New()
	return ResourceListModel{
		typeDef:       childType,
		viewConfig:    viewConfig,
		parentContext: parentCtx,
		displayName:   displayName,
		loading:       true,
		spinner:       sp,
		keys:          k,
	}
}

// NewResourceListFromCache creates a ResourceListModel pre-populated with cached data.
// No loading state, no spinner — the view is immediately ready to render.
func NewResourceListFromCache(
	typeDef resource.ResourceTypeDef,
	viewConfig *config.ViewsConfig,
	k keys.Map,
	resources []resource.Resource,
	pagination *resource.PaginationMeta,
	filterText string,
	sortField SortField,
	sortAsc bool,
	cursorPos int,
	hScrollOffset int,
	attentionOnly bool,
) ResourceListModel {
	m := ResourceListModel{
		typeDef:       typeDef,
		viewConfig:    viewConfig,
		allResources:  resources,
		pagination:    pagination,
		filterText:    filterText,
		sort:          sortField,
		sortAsc:       sortAsc,
		hScrollOffset: hScrollOffset,
		loading:       false,
		keys:          k,
		attentionOnly: attentionOnly,
	}
	m.applySortAndFilter()
	m.updateSortColKey()
	// Restore cursor position after filter application resets scroll.
	if cursorPos >= 0 && cursorPos < len(m.filteredResources) {
		m.scroll.SetCursor(cursorPos)
	}
	return m
}

// Init starts the spinner tick cycle.
func (m ResourceListModel) Init() (ResourceListModel, tea.Cmd) {
	return m, m.spinner.Tick
}

// Update handles messages: ResourcesLoadedMsg, spinner ticks, key events.
func (m ResourceListModel) Update(msg tea.Msg) (ResourceListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.ResourcesLoadedMsg:
		m.loading = false
		m.loadingMore = false
		if msg.Append {
			m.allResources = append(m.allResources, msg.Resources...)
		} else {
			m.allResources = msg.Resources
		}
		m.pagination = msg.Pagination
		if m.pendingFilter != "" {
			m.filterText = m.pendingFilter
			m.pendingFilter = ""
		}
		m.applySortAndFilter()
		m.styledRowCache = nil
		if m.autoOpenSingleDetail && len(m.filteredResources) == 1 {
			r := m.filteredResources[0]
			m.autoOpenSingleDetail = false
			return m, func() tea.Msg {
				return messages.NavigateMsg{
					Target:         messages.TargetDetail,
					ResourceType:   m.typeDef.ShortName,
					Resource:       &r,
					ReplaceCurrent: true,
				}
			}
		}
		if m.autoOpenSingleDetail && len(m.filteredResources) == 0 && m.pagination != nil && m.pagination.IsTruncated && !m.loadingMore {
			if _, ok := m.exactRelatedTargetID(); ok {
				m.loadingMore = true
				rt := m.typeDef.ShortName
				token := m.pagination.NextToken
				pc := m.parentContext
				return m, func() tea.Msg {
					return messages.LoadMoreMsg{
						ResourceType:      rt,
						ContinuationToken: token,
						ParentContext:     pc,
					}
				}
			}
		}
		if m.autoOpenSingleDetail && len(m.filteredResources) == 0 && m.typeDef.StubCreator != nil {
			if targetID, ok := m.exactRelatedTargetID(); ok {
				m.autoOpenSingleDetail = false
				stub := m.typeDef.StubCreator(targetID)
				return m, func() tea.Msg {
					return messages.NavigateMsg{
						Target:         messages.TargetDetail,
						ResourceType:   m.typeDef.ShortName,
						Resource:       &stub,
						ReplaceCurrent: true,
					}
				}
			}
		}
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case tea.KeyMsg:
		oldCursor := m.scroll.Cursor()
		switch {
		case key.Matches(msg, m.keys.Up):
			m.scroll.Up()
		case key.Matches(msg, m.keys.Down):
			m.scroll.Down()
		case key.Matches(msg, m.keys.Top):
			m.scroll.Top()
		case key.Matches(msg, m.keys.Bottom):
			m.scroll.Bottom()
		case key.Matches(msg, m.keys.PageUp):
			pageSize := m.height - 1
			if pageSize < 1 {
				pageSize = 1
			}
			m.scroll.PageUp(pageSize)
		case key.Matches(msg, m.keys.PageDown):
			pageSize := m.height - 1
			if pageSize < 1 {
				pageSize = 1
			}
			m.scroll.PageDown(pageSize)
		case key.Matches(msg, m.keys.ScrollLeft):
			if m.hScrollOffset > 0 {
				m.hScrollOffset--
				m.styledRowCache = nil
			}
		case key.Matches(msg, m.keys.ScrollRight):
			cols := m.resolveColumns()
			visible := cols
			if m.hScrollOffset < len(cols) {
				visible = cols[m.hScrollOffset:]
			}
			fitted := m.fitColumns(visible)
			// Allow scroll if: columns were dropped, OR last column was shrunk.
			canScroll := len(fitted) < len(visible)
			if !canScroll && len(fitted) > 0 && len(visible) > 0 {
				lastFit := fitted[len(fitted)-1]
				lastOrig := visible[len(fitted)-1]
				canScroll = lastFit.width < lastOrig.width
			}
			if canScroll {
				m.hScrollOffset++
				m.styledRowCache = nil
			}
		case key.Matches(msg, m.keys.Enter):
			if r := m.SelectedResource(); r != nil {
				// Data-driven child view routing
				if updated, cmd := m.handleChildKey("enter", r); cmd != nil {
					return updated, cmd
				}
				// No child matched — default to detail view
				return m, func() tea.Msg {
					return messages.NavigateMsg{
						Target:   messages.TargetDetail,
						Resource: r,
					}
				}
			}
		case key.Matches(msg, m.keys.Describe):
			// d key always opens detail view (never drills into S3)
			if r := m.SelectedResource(); r != nil {
				return m, func() tea.Msg {
					return messages.NavigateMsg{
						Target:   messages.TargetDetail,
						Resource: r,
					}
				}
			}
		case key.Matches(msg, m.keys.YAML):
			if r := m.SelectedResource(); r != nil {
				return m, func() tea.Msg {
					return messages.NavigateMsg{
						Target:   messages.TargetYAML,
						Resource: r,
					}
				}
			}
		case key.Matches(msg, m.keys.ToggleAttentionOnly):
			m.attentionOnly = !m.attentionOnly
			m.applySortAndFilter()
			m.scroll.SetCursor(0)
			m.styledRowCache = nil
			return m, nil
		case key.Matches(msg, m.keys.SortByName):
			if m.sort == SortName {
				m.sortAsc = !m.sortAsc
			} else {
				m.sort = SortName
				m.sortAsc = true
			}
			m.updateSortColKey()
			m.applySortAndFilter()
		case key.Matches(msg, m.keys.SortByID):
			if m.sort == SortID {
				m.sortAsc = !m.sortAsc
			} else {
				m.sort = SortID
				m.sortAsc = true
			}
			m.updateSortColKey()
			m.applySortAndFilter()
		case key.Matches(msg, m.keys.SortByAge):
			if m.sort == SortAge {
				m.sortAsc = !m.sortAsc
			} else {
				m.sort = SortAge
				m.sortAsc = true
			}
			m.updateSortColKey()
			m.applySortAndFilter()
		case key.Matches(msg, m.keys.Events):
			if r := m.SelectedResource(); r != nil {
				if updated, cmd := m.handleChildKey("e", r); cmd != nil {
					return updated, cmd
				}
			}
		case key.Matches(msg, m.keys.Logs):
			if r := m.SelectedResource(); r != nil {
				if updated, cmd := m.handleChildKey("L", r); cmd != nil {
					return updated, cmd
				}
			}
		case key.Matches(msg, m.keys.Resources):
			if r := m.SelectedResource(); r != nil {
				if updated, cmd := m.handleChildKey("R", r); cmd != nil {
					return updated, cmd
				}
			}
		case key.Matches(msg, m.keys.Source):
			if r := m.SelectedResource(); r != nil {
				if updated, cmd := m.handleChildKey("s", r); cmd != nil {
					return updated, cmd
				}
			}
		case key.Matches(msg, m.keys.LoadMore):
			if m.pagination != nil && m.pagination.IsTruncated && !m.loadingMore {
				m.loadingMore = true
				rt := m.typeDef.ShortName
				token := m.pagination.NextToken
				pc := m.parentContext
				ff := m.fetchFilter
				return m, func() tea.Msg {
					return messages.LoadMoreMsg{
						ResourceType:      rt,
						ContinuationToken: token,
						ParentContext:     pc,
						FetchFilter:       ff,
					}
				}
			}
		}
		// Invalidate styled row cache for old and new cursor positions.
		if oldCursor != m.scroll.Cursor() {
			delete(m.styledRowCache, oldCursor)
			delete(m.styledRowCache, m.scroll.Cursor())
		}
	}
	return m, nil
}

// View renders the table content. Caller wraps in RenderFrame.
// Pointer receiver so that row caches persist across frames.
func (m *ResourceListModel) View() string {
	if m.loading {
		return m.spinner.View() + " Loading..."
	}
	if len(m.filteredResources) == 0 {
		return "No resources found"
	}

	cols := m.resolveColumns()

	// Apply horizontal scroll: skip hScrollOffset columns from the left.
	if m.hScrollOffset > 0 && m.hScrollOffset < len(cols) {
		cols = cols[m.hScrollOffset:]
	} else if m.hScrollOffset >= len(cols) {
		cols = nil
	}

	// Hide rightmost columns that don't fit in width.
	cols = m.fitColumns(cols)

	if len(cols) == 0 {
		return "No resources found"
	}

	// Render header row.
	headerLine := m.renderHeaderRow(cols)

	// Determine visible row count: height minus column header row (1).
	// Frame borders are already excluded from m.height by the root model.
	visibleRows := m.height - 1
	if visibleRows < 1 {
		visibleRows = 1
	}

	// Reserve one row for the "load more" indicator when paginated and truncated.
	showLoadMore := m.pagination != nil && m.pagination.IsTruncated
	if showLoadMore && visibleRows > 2 {
		visibleRows--
	}

	// Determine the window of rows to display, keeping cursor centered.
	startRow, endRow := m.scroll.VisibleWindow(visibleRows)

	var sb strings.Builder
	sb.WriteString(headerLine)

	for i := startRow; i < endRow; i++ {
		sb.WriteString("\n")
		r := m.filteredResources[i]

		styled, ok := m.styledRowCache[i]
		if !ok {
			isSelected := i == m.scroll.Cursor()
			var base lipgloss.Style
			if isSelected {
				base = styles.RowSelected
			} else {
				base = styles.RowColorStyle(r.Status)
			}
			styled = m.renderDataRow(cols, r, base, m.width, isSelected)
			if m.styledRowCache == nil {
				m.styledRowCache = make(map[int]string)
			}
			m.styledRowCache[i] = styled
		}
		sb.WriteString(styled)
	}

	// Append "load more" or "loading..." indicator when truncated.
	if showLoadMore {
		sb.WriteString("\n")
		var hint string
		switch {
		case m.loadingMore:
			hint = "── loading... ──"
		case m.filterText != "":
			hint = "── m: load more (filter applies to loaded data only) ──"
		default:
			hint = "── m: load more ──"
		}
		sb.WriteString(styles.DimText.Render(hint))
	}

	return sb.String()
}

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











