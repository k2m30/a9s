// SPDX-License-Identifier: GPL-3.0-or-later

package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
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
			colKey := cols[sortColIdx].SortColKey()
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

// Update handles messages: spinner ticks drive the loading animation; key
// events are translated to controller Actions or emitted as navigation
// messages. A list result is not among them — every one reaches the screen
// through the controller, where the request sequence is checked, and a view
// that applied one itself would be a second apply point past that check.
func (m ResourceListModel) Update(msg tea.Msg) (ResourceListModel, tea.Cmd) {
	switch msg := msg.(type) {
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

// RenderList renders the list body from a controller-supplied ListBody. Every
// value it paints comes from body — the cells, their colours, the sort, the
// horizontal scroll, the pagination hint. The model supplies only the terminal
// geometry the controller cannot know: width, height, and the scroll offset.
//
// Nothing here re-derives a presentation decision the controller already made.
// body.EnrichmentFindings in particular is not consulted: the status cell and
// the row colour are resolved once, in core/app, and arrive already settled
// (tests/unit/tui_viewstate_purity_list_test.go holds this).
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
			title:   cd.Title,
			width:   cd.Width,
			key:     cd.Key,
			path:    cd.Path,
			sortKey: cd.SortColKey(),
		}
	}
	// Apply sort-key prefix widths so header titles match View() exactly.
	fullCols = applySortKeyPrefixWidths(fullCols)

	// The marker column index is pre-computed on the full column list by the controller.

	cols := fullCols
	scrollX := body.ScrollX
	if scrollX > 0 && scrollX < len(cols) {
		cols = cols[scrollX:]
	} else if scrollX >= len(cols) {
		cols = nil
	}

	// Widen lifecycle/status column to the max natural phrase width across all rows.
	// body.Rows[i].Cells are indexed by the full (pre-scroll) column list, so
	// body.StatusCol (also full-column-space) is passed to resolve the correct
	// cell index regardless of the scroll offset. The widen pass measures
	// row.Cells verbatim — the S4 status-column override is already baked into
	// Cells by buildListBody, so no separate findings-phrase measurement is
	// needed.
	cols = renderListWidenLifecycleColumn(cols, fullCols, body.Rows, body.StatusCol, scrollX)

	cols = m.fitColumns(cols)

	if len(cols) == 0 {
		return "No resources found"
	}

	headerLine := renderHeaderRow(cols, body.Sort.Col, body.Sort.Dir != "desc", scrollX)

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
		styled := renderListDataRow(cols, row, base, m.width, isSelected, scrollX)
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

	// Cache-first seeding: the list opened with rows already
	// visible while a fresh fetch confirms/replaces them. Additive-only — this
	// line never renders when Refreshing is false (the default), so it does
	// not affect the byte-parity gate against the legacy View() path.
	if body.Refreshing {
		sb.WriteString("\n")
		sb.WriteString(styles.DimText.Render("── refreshing... ──"))
	}

	// Per cache contract C4: a fetch failure over cached content swaps the refreshing
	// marker for an error marker — cached rows stay on screen, nothing goes
	// blank. LastFetchError is consumed verbatim (already-classified text
	// from HandleAPIError); this view performs no further formatting.
	if body.LastFetchError != "" {
		sb.WriteString("\n")
		sb.WriteString(styles.FlashError.Render("── error: " + body.LastFetchError + " ──"))
	}

	return sb.String()
}

// renderListWidenLifecycleColumn widens the status/lifecycle column to the
// max natural width of its baked cell text (body.Rows[i].Cells[statusCol]) —
// a pure consumer of the pre-resolved body.StatusCol index. No re-measurement
// of EnrichmentFindings phrases happens here: buildListBody has already baked
// any S4 status-column override into Cells, so measuring Cells verbatim is
// sufficient.
//
// statusCol is the full-column-space index (body.StatusCol); -1 means the
// type has no status column, a no-op. cols is the post-scroll visible slice
// whose matching entry gets widened; fullCols is the pre-scroll full column
// list used to translate statusCol into a visible index.
func renderListWidenLifecycleColumn(cols []listCol, fullCols []listCol, rows []app.ListRow, statusCol int, scrollX int) []listCol {
	if len(cols) == 0 || len(rows) == 0 || statusCol < 0 {
		return cols
	}

	// Translate the full-column-space status index into the visible
	// (post-scroll, post-fit) index.
	visIdx := -1
	if statusCol >= scrollX {
		candidate := statusCol - scrollX
		if candidate < len(cols) && candidate < len(fullCols[scrollX:]) {
			origIdx := scrollX + candidate
			if origIdx < len(fullCols) && cols[candidate].key == fullCols[origIdx].key {
				visIdx = candidate
			}
		}
	}
	if visIdx < 0 {
		// Lifecycle column is scrolled off; nothing to widen.
		return cols
	}

	maxW := cols[visIdx].width
	for _, row := range rows {
		if statusCol < len(row.Cells) {
			if nat := text.Width(row.Cells[statusCol]); nat > maxW {
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

// renderListDataRow renders a single data row as a pure consumer of
// app.ListRow: cell text comes from row.Cells verbatim. The status-column
// override is already baked into the cell by core/app's buildListBody, which
// is where the rule that docs/resources/*.md §4 states for each type lives.
// No re-derivation from an enrichment findings map happens here, and no
// marker is prepended to any cell — a row's colour already carries its worst
// finding.
func renderListDataRow(cols []listCol, row app.ListRow, base lipgloss.Style, totalWidth int, isSelected bool, cellOffset int) string {
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
		padded := text.PadOrTrunc(val, c.width)
		used += c.width
		b.WriteString(base.Render(padded))
	}
	if isSelected && totalWidth > used {
		b.WriteString(base.Render(strings.Repeat(" ", totalWidth-used)))
	}
	return b.String()
}

// SetEnrichmentState stores Wave 2 enrichment results for this resource
// type. findings/details are per-ID, slice/nested-valued — every
// independently-evaluated Wave-2 condition a resource carries (the cache-hit
// navigation seed via wave2FindingsByID/wave2DetailsByID, and the live
// enrichment fold) reaches Controller.ApplyEnrichmentState's plural storage
// layer unreduced. Invalidates the render cache.
func (m *ResourceListModel) SetEnrichmentState(issueCount int, truncated bool, findings map[string][]domain.Finding, details map[string]map[domain.FindingCode]domain.AttentionDetail) {
	m.ctrl.ApplyEnrichmentState(m.typeDef.ShortName, issueCount, truncated, findings, details)
	m.styledRowCache = nil
}
