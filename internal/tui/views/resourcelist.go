package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

// ResourceListModel is a thin delegating renderer for the resource table view.
// The app.Controller is the single source of truth for all list data (resources,
// filter, sort, cursor, pagination, enrichment). ResourceListModel owns only
// renderer state: terminal dimensions, spinner, the key map, and the typeDef
// needed for child-view routing and column rendering. All data reads go through
// m.ctrl.Snapshot().Body.List; all writes go through m.ctrl.Apply().
type ResourceListModel struct {
	typeDef    resource.ResourceTypeDef
	viewConfig *config.ViewsConfig

	spinner spinner.Model
	width   int
	height  int
	keys    keys.Map

	// styledRowCache caches fully styled row strings. Keyed by row index in the
	// visible (post-filter) set. Invalidated whenever the controller snapshot
	// changes the visible set or selection.
	styledRowCache map[int]string

	ctrl *app.Controller

	// Ephemeral render-time fields — populated from ctrl.Snapshot().Body.List
	// during RenderList and renderHeaderRow only. Never written by Update();
	// they carry zero values at all other times.
	sortColKey    string
	sortAsc       bool
	hScrollOffset int
}

// newResourceListCtrl creates a stub controller for a top-level resource-list
// screen. Used by NewResourceList for backward-compat paths and unit tests.
// Routes via ActionCommand so the menu stack is correctly initialised when the
// type is a registered menu entry. Falls back to PushChildListScreen for types
// that are not menu entries (unit-test types, ad-hoc types).
func newResourceListCtrl(typeDef resource.ResourceTypeDef, core *runtime.Core) *app.Controller {
	if core == nil {
		core = runtime.Bootstrap("", "", resource.AllResourceTypes())
	}
	c := app.New(core)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: typeDef.ShortName})
	// If ActionCommand did not push a list screen (type not in menu), fall back
	// to a direct push so topListState() is non-nil for all subsequent operations.
	if c.GetListSelectedRow() == 0 && len(c.GetListAllResources()) == 0 {
		snap := c.Snapshot()
		if snap.Body.List == nil {
			c.PushChildListScreen(typeDef.ShortName)
		}
	}
	return c
}

// newChildListCtrl creates a stub controller for a child resource-list screen
// (e.g., s3_objects, r53_records). These types are not menu entries so they
// cannot be reached via ActionCommand; PushChildListScreen bypasses the menu.
func newChildListCtrl(typeDef resource.ResourceTypeDef, core *runtime.Core) *app.Controller {
	if core == nil {
		core = runtime.Bootstrap("", "", resource.AllResourceTypes())
	}
	c := app.New(core)
	c.PushChildListScreen(typeDef.ShortName)
	return c
}

// NewResourceList creates a ResourceListModel in loading state.
// ctrl is optional — when nil a stub controller is constructed for backward
// compatibility with callers that do not yet pass a controller.
func NewResourceList(typeDef resource.ResourceTypeDef, viewConfig *config.ViewsConfig, k keys.Map, ctrl ...*app.Controller) ResourceListModel {
	sp := spinner.New()
	var c *app.Controller
	if len(ctrl) > 0 {
		c = ctrl[0]
	}
	if c == nil {
		c = newResourceListCtrl(typeDef, nil)
	}
	c.SetViewConfig(viewConfig)
	// For unregistered types (unit-test typeDefs) the catalog has no entry.
	// Register the typeDef's columns as a fallback so buildListBody can render rows.
	c.RegisterFallbackTypeDef(typeDef)
	m := ResourceListModel{
		typeDef:    typeDef,
		viewConfig: viewConfig,
		spinner:    sp,
		keys:       k,
		ctrl:       c,
	}
	// ct-events default sort (event_time DESC) is seeded once by the controller
	// at list-state creation (app.applyListDefaults), not here — this constructor
	// runs on every keystroke and must not re-apply it.
	return m
}

// NewChildResourceList creates a ResourceListModel for a child resource type.
// parentCtx provides parameters from the parent view (e.g., bucket name, zone ID).
// displayName is used for the frame title instead of the type's ShortName.
func NewChildResourceList(childType resource.ResourceTypeDef, parentCtx map[string]string, displayName string, viewConfig *config.ViewsConfig, k keys.Map, ctrl ...*app.Controller) ResourceListModel {
	sp := spinner.New()
	var c *app.Controller
	if len(ctrl) > 0 {
		c = ctrl[0]
	}
	if c == nil {
		// Child types (s3_objects, r53_records, etc.) are not menu entries;
		// use PushChildListScreen to bypass ActionCommand routing.
		c = newChildListCtrl(childType, nil)
	}
	c.SetViewConfig(viewConfig)
	// Register fallback columns for unregistered/child types so buildListBody
	// can render rows even when the type is absent from the catalog.
	c.RegisterFallbackTypeDef(childType)
	if displayName != "" {
		c.PatchListDisplayName(displayName)
	}
	if len(parentCtx) > 0 {
		c.PatchListParentContext(parentCtx)
	}
	return ResourceListModel{
		typeDef:    childType,
		viewConfig: viewConfig,
		spinner:    sp,
		keys:       k,
		ctrl:       c,
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
	ctrl ...*app.Controller,
) ResourceListModel {
	var c *app.Controller
	if len(ctrl) > 0 {
		c = ctrl[0]
	}
	if c == nil {
		c = newResourceListCtrl(typeDef, nil)
	}
	c.SetViewConfig(viewConfig)
	c.RegisterFallbackTypeDef(typeDef)
	c.ApplyResourcesLoaded(typeDef.ShortName, resources, pagination, false)
	if filterText != "" {
		c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: filterText})
	}
	if attentionOnly {
		// Toggle attention if not already on.
		snap := c.Snapshot()
		if snap.Body.List != nil && !snap.Body.List.AttentionOnly {
			c.Apply(app.Action{Kind: app.ActionToggleAttention})
		}
	}
	if hScrollOffset > 0 {
		for range hScrollOffset {
			c.Apply(app.Action{Kind: app.ActionScrollRight})
		}
	}
	// Apply sort: sortColIdx is a 0-based column index; translate to column key.
	// Use the controller's instance resolver so fallback typeDefs (e.g. test
	// types not in the catalog) contribute their columns to the index mapping.
	if sortColIdx >= 0 {
		cols := c.ResolveColumnsForType(typeDef.ShortName)
		if sortColIdx < len(cols) {
			colKey := cols[sortColIdx].Key
			// View-config columns (path-based) may have an empty Key. Fall back to
			// the matching td.Columns entry by title so ActionSort's "Arg != """
			// guard does not silently drop the sort.
			if colKey == "" {
				colTitle := cols[sortColIdx].Title
				for _, tc := range typeDef.Columns {
					if tc.Title == colTitle && tc.Key != "" {
						colKey = tc.Key
						break
					}
				}
			}
			if colKey != "" {
				c.Apply(app.Action{Kind: app.ActionSort, Arg: colKey})
				if !sortAsc {
					// First apply sets asc; second flips to desc.
					c.Apply(app.Action{Kind: app.ActionSort, Arg: colKey})
				}
			}
		}
	}
	// Restore cursor.
	if cursorPos > 0 {
		for range cursorPos {
			c.Apply(app.Action{Kind: app.ActionMoveDown})
		}
	}
	return ResourceListModel{
		typeDef:    typeDef,
		viewConfig: viewConfig,
		spinner:    spinner.New(),
		keys:       k,
		ctrl:       c,
	}
}

// Init starts the spinner tick cycle.
func (m ResourceListModel) Init() (ResourceListModel, tea.Cmd) {
	return m, m.spinner.Tick
}

// Update handles messages: ResourcesLoaded seeds the controller cache; spinner
// ticks drive the loading animation; key events are translated to controller
// Actions or emitted as navigation messages.
func (m ResourceListModel) Update(msg tea.Msg) (ResourceListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case messages.ResourcesLoaded:
		// Drop loads for a different active resource type.
		if msg.ResourceType != "" {
			canon := msg.ResourceType
			if td := resource.FindResourceType(msg.ResourceType); td != nil {
				canon = td.ShortName
			}
			if canon != m.typeDef.ShortName {
				return m, nil
			}
		}
		m.ctrl.ApplyResourcesLoaded(m.typeDef.ShortName, msg.Resources, msg.Pagination, msg.Append)
		m.styledRowCache = nil

		// Auto-open single detail: mirrors the old logic reading m.filteredResources.
		snap := m.ctrl.Snapshot()
		ls := snap.Body.List
		if ls != nil && m.ctrl.GetListAutoOpenSingle() {
			if len(ls.Rows) == 1 {
				r, ok := m.ctrl.ListSelected()
				if ok {
					m.ctrl.ClearListAutoOpenSingle()
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
					rCopy := r
					return m, func() tea.Msg {
						return messages.Navigate{
							Target:         messages.TargetDetail,
							ResourceType:   m.typeDef.ShortName,
							Resource:       &rCopy,
							ReplaceCurrent: true,
						}
					}
				}
			}
			// Zero rows, paginated, single target ID → load more.
			if len(ls.Rows) == 0 && ls.Truncated && !ls.LoadingMore {
				if _, ok := m.ctrl.GetListExactRelatedTargetID(); ok {
					m.ctrl.SetListLoadingMore(true)
					rt := m.typeDef.ShortName
					token := m.ctrl.GetListPaginationCursor()
					pc := m.ctrl.GetListParentContext()
					return m, func() tea.Msg {
						return messages.LoadMore{
							ResourceType:      rt,
							ContinuationToken: token,
							ParentContext:     pc,
						}
					}
				}
			}
			// Zero rows, StubCreator available → synthesise stub.
			if len(ls.Rows) == 0 && m.typeDef.StubCreator != nil {
				if targetID, ok := m.ctrl.GetListExactRelatedTargetID(); ok {
					m.ctrl.ClearListAutoOpenSingle()
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
		}
		return m, nil

	case spinner.TickMsg:
		snap := m.ctrl.Snapshot()
		if snap.Body.List != nil && snap.Body.List.Loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case tea.KeyMsg:
		oldSel := m.ctrl.GetListSelectedRow()
		switch {
		case key.Matches(msg, m.keys.Up):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveUp})
		case key.Matches(msg, m.keys.Down):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveDown})
		case key.Matches(msg, m.keys.Top):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveTop})
		case key.Matches(msg, m.keys.Bottom):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveBottom})
		case key.Matches(msg, m.keys.PageUp):
			m.ctrl.Apply(app.Action{Kind: app.ActionPageUp, N: max(m.height-1, 1)})
		case key.Matches(msg, m.keys.PageDown):
			m.ctrl.Apply(app.Action{Kind: app.ActionPageDown, N: max(m.height-1, 1)})
		case key.Matches(msg, m.keys.ScrollLeft):
			m.ctrl.Apply(app.Action{Kind: app.ActionScrollLeft})
			m.styledRowCache = nil
		case key.Matches(msg, m.keys.ScrollRight):
			// Guard: only scroll right if columns actually overflow.
			snap := m.ctrl.Snapshot()
			if snap.Body.List != nil {
				scrollX := snap.Body.List.ScrollX
				cols := snap.Body.List.Columns
				visible := cols
				if scrollX < len(cols) {
					visible = cols[scrollX:]
				}
				// Build listCol slice for fitColumns check.
				lcols := make([]listCol, len(visible))
				for i, cd := range visible {
					lcols[i] = listCol{title: cd.Title, width: cd.Width, key: cd.Key, path: cd.Path}
				}
				fitted := m.fitColumns(lcols)
				canScroll := len(fitted) < len(lcols)
				if !canScroll && len(fitted) > 0 && len(lcols) > 0 {
					canScroll = fitted[len(fitted)-1].width < lcols[len(fitted)-1].width
				}
				if canScroll {
					m.ctrl.Apply(app.Action{Kind: app.ActionScrollRight})
					m.styledRowCache = nil
				}
			}
		case key.Matches(msg, m.keys.Enter):
			if r, ok := m.ctrl.ListSelected(); ok {
				if updated, cmd := m.handleChildKey("enter", &r); cmd != nil {
					return updated, cmd
				}
				rCopy := r
				return m, func() tea.Msg {
					return messages.Navigate{
						Target:   messages.TargetDetail,
						Resource: &rCopy,
					}
				}
			}
		case key.Matches(msg, m.keys.Describe):
			if r, ok := m.ctrl.ListSelected(); ok {
				rCopy := r
				return m, func() tea.Msg {
					return messages.Navigate{
						Target:   messages.TargetDetail,
						Resource: &rCopy,
					}
				}
			}
		case key.Matches(msg, m.keys.YAML):
			if r, ok := m.ctrl.ListSelected(); ok {
				rCopy := r
				return m, func() tea.Msg {
					return messages.Navigate{
						Target:   messages.TargetYAML,
						Resource: &rCopy,
					}
				}
			}
		case key.Matches(msg, m.keys.JSON):
			if r, ok := m.ctrl.ListSelected(); ok {
				rCopy := r
				return m, func() tea.Msg {
					return messages.Navigate{
						Target:   messages.TargetJSON,
						Resource: &rCopy,
					}
				}
			}
		case key.Matches(msg, m.keys.ToggleAttentionOnly):
			m.ctrl.Apply(app.Action{Kind: app.ActionToggleAttention})
			m.styledRowCache = nil
			return m, nil
		case key.Matches(msg, m.keys.Events):
			if r, ok := m.ctrl.ListSelected(); ok {
				if updated, cmd := m.handleChildKey("e", &r); cmd != nil {
					return updated, cmd
				}
			}
		case key.Matches(msg, m.keys.Logs):
			if r, ok := m.ctrl.ListSelected(); ok {
				if updated, cmd := m.handleChildKey("L", &r); cmd != nil {
					return updated, cmd
				}
			}
		case key.Matches(msg, m.keys.Resources):
			if r, ok := m.ctrl.ListSelected(); ok {
				if updated, cmd := m.handleChildKey("R", &r); cmd != nil {
					return updated, cmd
				}
			}
		case key.Matches(msg, m.keys.Source):
			if r, ok := m.ctrl.ListSelected(); ok {
				if updated, cmd := m.handleChildKey("s", &r); cmd != nil {
					return updated, cmd
				}
			}
		case key.Matches(msg, m.keys.CloudTrail):
			pc := m.ctrl.GetListParentContext()
			if m.typeDef.CloudTrailKey == "" || pc != nil {
				break
			}
			if r, ok := m.ctrl.ListSelected(); ok {
				filter := resource.BuildCloudTrailFilter(r, m.typeDef.ShortName)
				if filter != nil {
					rCopy := r
					return m, func() tea.Msg {
						return messages.RelatedNavigate{
							TargetType:     "ct-events",
							SourceResource: rCopy,
							SourceType:     m.typeDef.ShortName,
							FetchFilter:    filter,
						}
					}
				}
			}
		case key.Matches(msg, m.keys.LoadMore):
			snap := m.ctrl.Snapshot()
			if snap.Body.List != nil && snap.Body.List.Truncated && !snap.Body.List.LoadingMore {
				m.ctrl.SetListLoadingMore(true)
				rt := m.typeDef.ShortName
				token := m.ctrl.GetListPaginationCursor()
				pc := m.ctrl.GetListParentContext()
				ff := m.ctrl.GetListFetchFilter()
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
		// Invalidate styled row cache when selection moves.
		if newSel := m.ctrl.GetListSelectedRow(); newSel != oldSel {
			delete(m.styledRowCache, oldSel)
			delete(m.styledRowCache, newSel)
		}
	}
	return m, nil
}

// View renders the table content. Caller wraps in RenderFrame.
// Pointer receiver so that row caches persist across frames.
func (m *ResourceListModel) View() string {
	snap := m.ctrl.Snapshot()
	if snap.Body.List == nil {
		return m.spinner.View() + " Loading..."
	}
	return m.RenderList(*snap.Body.List)
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

	// Populate ephemeral render-time fields from body for header rendering,
	// then restore after. These fields exist only to satisfy renderHeaderRow's
	// value-receiver reads; they carry no state between frames.
	m.sortColKey = renderListSortColKey(body.Sort, fullCols, m.typeDef)
	m.sortAsc = body.Sort.Dir != "desc"
	m.hScrollOffset = scrollX
	headerLine := m.renderHeaderRow(cols)
	m.sortColKey = ""
	m.sortAsc = false
	m.hScrollOffset = 0

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
// Delegates to the controller's ApplyEnrichmentState and invalidates the render cache.
func (m *ResourceListModel) SetEnrichmentState(issueCount int, truncated bool, findings map[string]domain.Finding) {
	m.ctrl.ApplyEnrichmentState(m.typeDef.ShortName, issueCount, truncated, findings)
	m.styledRowCache = nil
}

// ApplyFieldUpdates merges Wave-2-derived field values into the in-memory
// resource slices via the controller. Invalidates the styled row cache.
func (m *ResourceListModel) ApplyFieldUpdates(updates map[string]map[string]string) {
	if len(updates) == 0 {
		return
	}
	m.ctrl.ApplyListFieldUpdates(m.typeDef.ShortName, updates)
	m.styledRowCache = nil
}

// SetTruncatedIDs stores the per-resource truncation set for this resource type.
// Delegated to the controller.
func (m *ResourceListModel) SetTruncatedIDs(truncatedIDs map[string]bool) {
	m.ctrl.ApplyListTruncatedIDs(m.typeDef.ShortName, truncatedIDs)
	m.styledRowCache = nil
}

// InvalidateStyleCache clears the styled row cache, forcing re-render
// with current styles. Called after theme changes.
func (m *ResourceListModel) InvalidateStyleCache() {
	m.styledRowCache = nil
}
