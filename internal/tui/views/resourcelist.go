package views

import (
	"maps"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

// ResourceListModel is a tea.Model for the resource table view.
type ResourceListModel struct {
	typeDef    resource.ResourceTypeDef
	viewConfig *config.ViewsConfig

	allResources      []resource.Resource
	filteredResources []resource.Resource

	scroll        ScrollState
	hScrollOffset int

	sortColIdx int // -1 = no sort, 0-based column index
	sortAsc    bool
	sortColKey string // exact column key carrying the sort glyph; set when sort changes (§6)

	filterText           string
	AttentionFilter                          // §7: ctrl+z toggle — when enabled, hide dim/neutral rows
	issueCount           int                 // count of IsIssueRowColor() resources; recomputed in applySortAndFilter
	showIssueBadge       bool                // when true, FrameTitle shows "issues:N" badge; set by main menu navigation
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

	// enrichment state — populated by SetEnrichmentState.
	enrichmentIssueCount int                                   // unified Wave-1 + Wave-2 distinct-instance count
	enrichmentTruncated  bool                                  // true if enrichment count is a lower bound
	findingsByID         map[string]domain.Finding             // this type's per-resource Wave-2 findings (AS-1395 typed model)
	// truncatedByID is populated by SetTruncatedIDs. Resources in this set
	// had their enrichment truncated (per-resource API error or page cap) and
	// are rendered with a "?" prefix in the identity column.
	truncatedByID map[string]bool

	// reapplyChecker + reapplySource — carried forward from a related-panel
	// navigation. On each fresh page of target resources (ResourcesLoadedMsg),
	// the checker re-runs against the page and new matching IDs merge into
	// relatedIDSet. This makes approximate pivots (0+, 10+, 25+) behave
	// symmetrically: the ID set extends organically as the target type's
	// pagination is consumed. Nil on non-related-navigated lists (no-op).
	reapplyChecker resource.RelatedChecker
	reapplySource  resource.Resource
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
		sortColIdx: SortColNone,
	}
	// ct-events: default sort is by event_time DESC (newest first).
	// Only apply when a viewConfig is present (full app mode); unit tests
	// that pass nil viewConfig work with synthetic data and are not affected.
	if typeDef.ShortName == "ct-events" && viewConfig != nil {
		cols := m.resolveColumns()
		for i, c := range cols {
			if c.sortKey == "event_time" {
				m.sortColIdx = i
				m.sortAsc = false
				m.sortColKey = colSortKey(c)
				break
			}
		}
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
		sortColIdx:    SortColNone,
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
	sortColIdx int,
	sortAsc bool,
	cursorPos int,
	hScrollOffset int,
	attentionOnly bool,
) ResourceListModel {
	m := ResourceListModel{
		typeDef:         typeDef,
		viewConfig:      viewConfig,
		allResources:    resources,
		pagination:      pagination,
		filterText:      filterText,
		sortColIdx:      sortColIdx,
		sortAsc:         sortAsc,
		hScrollOffset:   hScrollOffset,
		loading:         false,
		keys:            k,
		AttentionFilter: AttentionFilter{enabled: attentionOnly},
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
	case messages.ResourcesLoaded:
		// Drop loads for a different active resource type. A late EC2 fetch
		// returning after the user has opened S3 must not mutate the S3
		// list or its loading state — let the root model route the message
		// to its write-through cache (which keys by msg.ResourceType).
		//
		// Compare via the registry so the alias the fetcher carried (e.g.
		// "rds") still matches the list's canonical ShortName ("dbi"). Empty
		// ResourceType ("unstamped") falls through: production fetch results
		// always stamp the type via internal/tui/fetch_adapter.go, so an
		// empty type only appears in unit-test fixtures that synthesise a
		// ResourcesLoaded without a ResourceType. AS-648-h2 will tighten
		// this further by carrying a session generation alongside the type.
		//
		// Child views (parentContext != nil) must still be guarded: production
		// child fetches stamp ResourceType = childType (canonical, e.g.
		// "s3_objects") at internal/tui/fetch_adapter.go:103, so the same
		// canonicalised comparison rejects a late ResourcesLoaded from a
		// different child whose Gen guard happens to still match.
		if msg.ResourceType != "" {
			canon := msg.ResourceType
			if td := resource.FindResourceType(msg.ResourceType); td != nil {
				canon = td.ShortName
			}
			if canon != m.typeDef.ShortName {
				return m, nil
			}
		}
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
			// Mirror Enter: if the type registers a child under Key="enter"
			// and its DrillCondition (if any) admits this row, jump straight
			// into the child view. Otherwise fall back to the generic detail.
			// This keeps related-panel pivots consistent with manual drill
			// from the same list — a pivot that narrows to one row must not
			// strand the operator on bucket metadata when Enter would have
			// opened bucket contents.
			if enterChild := m.enterChildFor(r); enterChild != nil {
				ctx := m.buildChildContext(*enterChild, &r)
				displayName := ctx[enterChild.DisplayNameKey]
				childType := enterChild.ChildType
				return m, func() tea.Msg {
					return messages.EnterChildView{
						ChildType:     childType,
						ParentContext: ctx,
						DisplayName:   displayName,
					}
				}
			}
			return m, func() tea.Msg {
				return messages.Navigate{
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
					return messages.LoadMore{
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
					return messages.Navigate{
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
			pageSize := max(m.height-1, 1)
			m.scroll.PageUp(pageSize)
		case key.Matches(msg, m.keys.PageDown):
			pageSize := max(m.height-1, 1)
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
					return messages.Navigate{
						Target:   messages.TargetDetail,
						Resource: r,
					}
				}
			}
		case key.Matches(msg, m.keys.Describe):
			// d key always opens detail view (never drills into S3)
			if r := m.SelectedResource(); r != nil {
				return m, func() tea.Msg {
					return messages.Navigate{
						Target:   messages.TargetDetail,
						Resource: r,
					}
				}
			}
		case key.Matches(msg, m.keys.YAML):
			if r := m.SelectedResource(); r != nil {
				return m, func() tea.Msg {
					return messages.Navigate{
						Target:   messages.TargetYAML,
						Resource: r,
					}
				}
			}
		case key.Matches(msg, m.keys.JSON):
			if r := m.SelectedResource(); r != nil {
				return m, func() tea.Msg {
					return messages.Navigate{
						Target:   messages.TargetJSON,
						Resource: r,
					}
				}
			}
		case key.Matches(msg, m.keys.ToggleAttentionOnly):
			m.Toggle()
			m.applySortAndFilter()
			m.scroll.SetCursor(0)
			m.styledRowCache = nil
			return m, nil
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
		case key.Matches(msg, m.keys.CloudTrail):
			if m.typeDef.CloudTrailKey == "" || m.parentContext != nil {
				break
			}
			if r := m.SelectedResource(); r != nil {
				filter := resource.BuildCloudTrailFilter(*r, m.typeDef.ShortName)
				if filter != nil {
					return m, func() tea.Msg {
						return messages.RelatedNavigate{
							TargetType:     "ct-events",
							SourceResource: *r,
							SourceType:     m.typeDef.ShortName,
							FetchFilter:    filter,
						}
					}
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
					return messages.LoadMore{
						ResourceType:      rt,
						ContinuationToken: token,
						ParentContext:     pc,
						FetchFilter:       ff,
					}
				}
			}
		default:
			if m.handleSortByCol(msg) {
				return m, nil
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

	fullCols := m.resolveColumns()

	// Resolve marker column index on the FULL (unsliced) column list so that
	// horizontal scrolling cannot move the marker to a different semantic column
	// (e.g. State.Name path matching "Name" when Name itself is scrolled off).
	fullMarkerColIdx := resolveIdentityColumn(fullCols, m.typeDef)

	cols := fullCols

	// Apply horizontal scroll: skip hScrollOffset columns from the left.
	if m.hScrollOffset > 0 && m.hScrollOffset < len(cols) {
		cols = cols[m.hScrollOffset:]
	} else if m.hScrollOffset >= len(cols) {
		cols = nil
	}

	// Pre-widen the lifecycle/status column to the max natural phrase width
	// across ALL rows BEFORE fitColumns and renderHeaderRow. This ensures the
	// header and all data rows use the same (widened) column width; per-row
	// widening in renderDataRow is therefore no longer needed.
	cols = m.widenLifecycleColumn(cols, m.filteredResources)

	// Hide rightmost columns that don't fit in width.
	cols = m.fitColumns(cols)

	if len(cols) == 0 {
		return "No resources found"
	}

	// Render header row.
	headerLine := m.renderHeaderRow(cols)

	// Translate the full-column marker index to the visible (post-hscroll, post-fit)
	// column index. If the identity column is scrolled off-screen or truncated out,
	// set to -1 so renderDataRow skips the marker entirely rather than falling back
	// to the first visible column (which would jump the dot to State/Type/etc.).
	// Must be computed here (before VisibleWindow) so findingsBanner can use it.
	markerColIdx := -1
	if fullMarkerColIdx >= m.hScrollOffset {
		candidate := fullMarkerColIdx - m.hScrollOffset
		if candidate < len(cols) && cols[candidate].key == fullCols[fullMarkerColIdx].key && cols[candidate].path == fullCols[fullMarkerColIdx].path {
			markerColIdx = candidate
		}
	}

	// Determine visible row count: height minus column header row (1).
	// Frame borders are already excluded from m.height by the root model.
	visibleRows := max(m.height-1, 1)

	// Reserve one row for the "load more" indicator when paginated and truncated.
	showLoadMore := m.pagination != nil && m.pagination.IsTruncated
	if showLoadMore && visibleRows > 2 {
		visibleRows--
	}

	// Spec §4: only S1–S5 surfaces are rendered. No findings banner.
	// Determine the window of rows to display, keeping cursor centered.
	startRow, endRow := m.scroll.VisibleWindow(visibleRows)

	var sb strings.Builder
	_ = markerColIdx // column index reserved for per-row glyph rendering downstream.

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
				base = styles.ColorStyle(resolveRowColor(m.typeDef, r))
			}
			styled = m.renderDataRow(cols, r, base, m.width, isSelected, markerColIdx)
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

// RenderList renders the list body from a controller-supplied ListBody,
// byte-identical to View()'s output. The renderer owns scrollOffset/width/height
// (read from m); all data comes from body.
//
// Mapping from body fields to View() state:
//   - body.Loading            → m.loading
//   - body.Rows               → m.filteredResources (cells pre-extracted by controller)
//   - body.Selected           → m.scroll.Cursor()
//   - body.Columns            → resolved listCol slice (width/title/key from body)
//   - body.Sort               → m.sortColKey / m.sortAsc
//   - body.ScrollX            → m.hScrollOffset
//   - body.Truncated          → m.pagination.IsTruncated
//   - body.LoadingMore        → m.loadingMore
//   - body.Filter             → m.filterText (for load-more hint text)
//   - body.MarkerCol          → fullMarkerColIdx (identity column, before hscroll)
//   - body.EnrichmentFindings → m.findingsByID (for glyph prepend on identity col)
//   - body.Rows[i].Color      → resolveRowColor(m.typeDef, r) → styles.ColorStyle
//   - body.Rows[i].Decorator  → "!" / "~" glyph prefix on identity cell
func (m *ResourceListModel) RenderList(body app.ListBody) string {
	if body.Loading {
		return m.spinner.View() + " Loading..."
	}
	if len(body.Rows) == 0 {
		return "No resources found"
	}

	// Build listCol slice from body.Columns, mirroring resolveColumns output.
	fullCols := make([]listCol, len(body.Columns))
	for i, cd := range body.Columns {
		fullCols[i] = listCol{
			title: cd.Title,
			width: cd.Width,
			key:   cd.Key,
			path:  cd.Path,
		}
	}
	// Apply sort-key prefix widths so header titles match View() exactly.
	fullCols = applySortKeyPrefixWidths(fullCols)

	// The marker column index is pre-computed on the full column list by the controller.
	fullMarkerColIdx := body.MarkerCol

	cols := fullCols
	scrollX := body.ScrollX
	if scrollX > 0 && scrollX < len(cols) {
		cols = cols[scrollX:]
	} else if scrollX >= len(cols) {
		cols = nil
	}

	// Widen lifecycle/status column to the max natural phrase width across all rows.
	// body.Rows[i].Cells are indexed by the full (pre-scroll) column list, so fullCols
	// is passed to resolve the correct cell index regardless of the scroll offset.
	cols = renderListWidenLifecycleColumn(cols, fullCols, body.Rows, m.typeDef)

	cols = m.fitColumns(cols)

	if len(cols) == 0 {
		return "No resources found"
	}

	// Reconstruct sort state from body for header rendering.
	savedSortColKey := m.sortColKey
	savedSortAsc := m.sortAsc
	savedHScrollOffset := m.hScrollOffset
	m.sortColKey = renderListSortColKey(body.Sort, fullCols, m.typeDef)
	m.sortAsc = body.Sort.Dir != "desc"
	m.hScrollOffset = scrollX
	headerLine := m.renderHeaderRow(cols)
	m.sortColKey = savedSortColKey
	m.sortAsc = savedSortAsc
	m.hScrollOffset = savedHScrollOffset

	// Translate full-column marker index to visible (post-hscroll, post-fit) index.
	markerColIdx := -1
	if fullMarkerColIdx >= scrollX {
		candidate := fullMarkerColIdx - scrollX
		if candidate < len(cols) && candidate < len(fullCols[scrollX:]) {
			origIdx := scrollX + candidate
			if origIdx < len(fullCols) && cols[candidate].key == fullCols[origIdx].key {
				markerColIdx = candidate
			}
		}
	}

	visibleRows := max(m.height-1, 1)
	showLoadMore := body.Truncated
	if showLoadMore && visibleRows > 2 {
		visibleRows--
	}

	// Compute visible window using a synthetic ScrollState keyed on body.Selected.
	total := len(body.Rows)
	startRow, endRow := renderListVisibleWindow(body.Selected, total, visibleRows)

	var sb strings.Builder
	sb.WriteString(headerLine)

	for i := startRow; i < endRow; i++ {
		sb.WriteString("\n")
		row := body.Rows[i]
		isSelected := i == body.Selected
		base := renderListRowStyle(row, isSelected)
		styled := renderListDataRow(cols, row, base, m.width, isSelected, markerColIdx, body.EnrichmentFindings, scrollX)
		sb.WriteString(styled)
	}

	if showLoadMore {
		sb.WriteString("\n")
		var hint string
		switch {
		case body.LoadingMore:
			hint = "── loading... ──"
		case body.Filter != "":
			hint = "── m: load more (filter applies to loaded data only) ──"
		default:
			hint = "── m: load more ──"
		}
		sb.WriteString(styles.DimText.Render(hint))
	}

	return sb.String()
}

// renderListSortColKey returns the sort column key matching body.Sort.Col against
// the full resolved column list, mirroring how m.sortColKey is set via updateSortColKey.
//
// body.Sort.Col may be a td.Columns key (e.g. "workgroup_name") while fullCols
// contains path-based view-config columns (e.g. key="" title="Workgroup"). The
// cross-reference via td.Columns bridges the two: find the td column whose Key
// matches sort.Col, then find the fullCols column whose Title matches that td
// column's Title, and return its canonical colSortKey.
func renderListSortColKey(sort app.SortSpec, fullCols []listCol, td resource.ResourceTypeDef) string {
	if sort.Col == "" {
		return ""
	}
	sortColLower := strings.ToLower(sort.Col)
	for _, c := range fullCols {
		if c.key == sort.Col || c.path == sort.Col || c.title == sort.Col {
			return colSortKey(c)
		}
		// Title-underscore match: "Plan Name" → "plan_name" to bridge td.Columns
		// key identifiers with view-config path-only columns.
		titleUnder := strings.ToLower(strings.ReplaceAll(c.title, " ", "_"))
		if titleUnder == sortColLower {
			return colSortKey(c)
		}
	}
	// Cross-reference via td.Columns: sort.Col may be a td.Columns key (e.g.
	// "workgroup_name"). Find the td column with that key, then look up the
	// fullCols column by matching Title to get the canonical colSortKey.
	for _, tc := range td.Columns {
		if tc.Key == sort.Col {
			for _, c := range fullCols {
				if c.title == tc.Title {
					return colSortKey(c)
				}
			}
		}
	}
	return sort.Col
}

// renderListWidenLifecycleColumn mirrors widenLifecycleColumn but operates on
// pre-extracted cell strings from ListRow.Cells rather than resource.Resource.
// The lifecycle/status column is identified by key "status" or the type's LifecycleKey.
//
// fullCols is the pre-scroll full column list used to resolve the correct cell index in
// ListRow.Cells (which is always indexed by full-column position). cols is the
// post-scroll visible slice whose matching entry gets widened.
func renderListWidenLifecycleColumn(cols []listCol, fullCols []listCol, rows []app.ListRow, td resource.ResourceTypeDef) []listCol {
	if len(cols) == 0 || len(rows) == 0 {
		return cols
	}
	lifecycleKey := lifecycleColumnKey(td)

	// Find the lifecycle column's index in fullCols for correct row.Cells lookup.
	fullIdx := -1
	for i, c := range fullCols {
		if c.key == "status" || c.key == lifecycleKey {
			fullIdx = i
			break
		}
	}
	if fullIdx < 0 {
		return cols
	}

	// Find the same column in the visible (post-scroll) slice for widening.
	visIdx := -1
	for i, c := range cols {
		if c.key == "status" || c.key == lifecycleKey {
			visIdx = i
			break
		}
	}
	if visIdx < 0 {
		// Lifecycle column is scrolled off; nothing to widen.
		return cols
	}

	maxW := cols[visIdx].width
	for _, row := range rows {
		if fullIdx < len(row.Cells) {
			if nat := lipgloss.Width(row.Cells[fullIdx]); nat > maxW {
				maxW = nat
			}
		}
	}
	if maxW == cols[visIdx].width {
		return cols
	}
	out := make([]listCol, len(cols))
	copy(out, cols)
	out[visIdx].width = maxW
	return out
}

// renderListRowStyle returns the lipgloss.Style for a row, mirroring the base
// style selection in View(). Uses ListRow.Color (pre-resolved by the controller)
// to reconstruct the exact style that resolveRowColor + styles.ColorStyle would produce.
func renderListRowStyle(row app.ListRow, isSelected bool) lipgloss.Style {
	if isSelected {
		return styles.RowSelected
	}
	return styles.ColorStyle(colorTagToDomain(row.Color))
}

// colorTagToDomain converts a ListRow.Color string tag back to domain.Color.
func colorTagToDomain(tag string) domain.Color {
	switch tag {
	case "healthy":
		return domain.ColorHealthy
	case "warning":
		return domain.ColorWarning
	case "broken":
		return domain.ColorBroken
	case "dim":
		return domain.ColorDim
	}
	return domain.ColorHealthy
}

// renderListVisibleWindow mirrors ScrollState.VisibleWindow for RenderList,
// computing the centered visible window from the selected row index.
func renderListVisibleWindow(selected, total, viewHeight int) (int, int) {
	if total <= viewHeight {
		return 0, total
	}
	half := viewHeight / 2
	start := max(selected-half, 0)
	end := start + viewHeight
	if end > total {
		end = total
		start = max(end-viewHeight, 0)
	}
	return start, end
}

// renderListDataRow renders a single data row from pre-extracted ListRow.Cells,
// mirroring renderDataRow but reading cells from the body instead of resource.Resource.
// The decorator glyph ("! "/"~ ") is prepended to the identity cell (markerColIdx)
// for ColorHealthy rows with enrichment findings, exactly as renderDataRow does.
func renderListDataRow(cols []listCol, row app.ListRow, base lipgloss.Style, totalWidth int, isSelected bool, markerColIdx int, findings map[string]domain.Finding, cellOffset int) string {
	var b strings.Builder
	b.WriteString(base.Render(" "))
	used := 1
	for i, c := range cols {
		if i > 0 {
			b.WriteString(base.Render("  "))
			used += 2
		}
		var val string
		if cellOffset+i < len(row.Cells) {
			val = row.Cells[cellOffset+i]
		}
		// Enrichment glyph on identity column: mirrors renderDataRow's marker logic.
		// Only applies when the row is ColorHealthy (Decorator carries "!"/"~" only
		// for healthy rows per resolveListDecoratorFull).
		if i == markerColIdx && row.Color == "healthy" {
			if f, ok := findings[row.ResourceID]; ok {
				switch f.Severity {
				case domain.SevBroken:
					val = "! " + val
				case domain.SevWarn:
					val = "~ " + val
				}
			}
		}
		padded := text.PadOrTrunc(val, c.width)
		used += c.width
		b.WriteString(base.Render(padded))
	}
	if isSelected && totalWidth > used {
		b.WriteString(base.Render(strings.Repeat(" ", totalWidth-used)))
	}
	return b.String()
}

// SetEnrichmentState stores Wave 2 enrichment results for this resource type.
// issueCount is the unified Wave-1 + Wave-2 distinct-instance count (R2/R3 source of truth).
// truncated indicates a lower-bound count; findings is the per-resource finding map.
// Invalidates the styled row cache because findings affect marker rendering and
// re-runs applySortAndFilter so that the ctrl+z attention filter picks up newly
// flagged rows immediately — otherwise enabling ctrl+z before Wave 2 completes
// would leave the enriched rows hidden until the next filter edit.
func (m *ResourceListModel) SetEnrichmentState(issueCount int, truncated bool, findings map[string]domain.Finding) {
	m.enrichmentIssueCount = issueCount
	m.enrichmentTruncated = truncated
	m.findingsByID = findings
	m.styledRowCache = nil
	m.applySortAndFilter()
}

// ApplyFieldUpdates merges Wave-2-derived field values into the in-memory
// resource slices. Keyed by resource ID, then by field key. Invalidates the
// styled row cache so the new values appear on the next render.
func (m *ResourceListModel) ApplyFieldUpdates(updates map[string]map[string]string) {
	if len(updates) == 0 {
		return
	}
	for i := range m.allResources {
		if kvMap, ok := updates[m.allResources[i].ID]; ok {
			if m.allResources[i].Fields == nil {
				m.allResources[i].Fields = make(map[string]string, len(kvMap))
			}
			maps.Copy(m.allResources[i].Fields, kvMap)
		}
	}
	for i := range m.filteredResources {
		if kvMap, ok := updates[m.filteredResources[i].ID]; ok {
			if m.filteredResources[i].Fields == nil {
				m.filteredResources[i].Fields = make(map[string]string, len(kvMap))
			}
			maps.Copy(m.filteredResources[i].Fields, kvMap)
		}
	}
	m.styledRowCache = nil
}

// SetTruncatedIDs stores the per-resource truncation set for this resource type.
func (m *ResourceListModel) SetTruncatedIDs(truncatedIDs map[string]bool) {
	m.truncatedByID = truncatedIDs
	m.styledRowCache = nil
}

// InvalidateStyleCache clears the styled row cache, forcing re-render
// with current styles. Called after theme changes.
func (m *ResourceListModel) InvalidateStyleCache() {
	m.styledRowCache = nil
}
