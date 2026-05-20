package views

import (
	"maps"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
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
	findingsByID         map[string]resource.EnrichmentFinding // this type's per-resource findings
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

// SetEnrichmentState stores Wave 2 enrichment results for this resource type.
// issueCount is the unified Wave-1 + Wave-2 distinct-instance count (R2/R3 source of truth).
// truncated indicates a lower-bound count; findings is the per-resource finding map.
// Invalidates the styled row cache because findings affect marker rendering and
// re-runs applySortAndFilter so that the ctrl+z attention filter picks up newly
// flagged rows immediately — otherwise enabling ctrl+z before Wave 2 completes
// would leave the enriched rows hidden until the next filter edit.
func (m *ResourceListModel) SetEnrichmentState(issueCount int, truncated bool, findings map[string]resource.EnrichmentFinding) {
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
