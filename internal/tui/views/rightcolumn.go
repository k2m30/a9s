// Package views — RightColumnModel renders the RELATED panel in the detail view.
package views

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
)

type rightColumnRow struct {
	targetType  string
	displayName string
	count       int               // -1 = loading, 0+ = resolved
	resourceIDs []string          // IDs from checker result (for navigation in US3)
	fetchFilter map[string]string // server-side filter for filtered paginated fetcher
	loading     bool
	err         error
	approximate bool                    // true when count was derived from a truncated cache; UI renders "N+"
	checker     resource.RelatedChecker // originating RelatedDef.Checker — carried forward for re-apply on load-more
}

// RightColumnModel manages the RELATED panel rendered next to a detail view.
// Exported so tui.rendererState can hold one without importing internal view
// model types into the forbidden files.
type RightColumnModel struct {
	rows               []rightColumnRow
	cursor             int
	focused            bool
	width              int
	height             int
	scrollOffset       int
	filterQuery        string
	filterActive       bool
	parentRes          resource.Resource // stored for RelatedNavigateMsg construction
	sourceResourceType string            // short name of the resource type being detailed (e.g. "ct-events")
	keys               keys.Map
}

// newRightColumn constructs a RightColumnModel from related definitions and a parent resource.
// sourceType is the short name of the resource type being detailed (e.g. "ct-events").
// All rows start in loading state; checkers are dispatched by app.go.
func newRightColumn(defs []resource.RelatedDef, parentRes resource.Resource, sourceType string) RightColumnModel {
	rows := make([]rightColumnRow, len(defs))
	for i, def := range defs {
		rows[i] = rightColumnRow{
			targetType:  def.TargetType,
			displayName: def.DisplayName,
			count:       -1,
			loading:     true,
			checker:     def.Checker,
		}
	}
	return RightColumnModel{
		rows:               rows,
		parentRes:          parentRes,
		sourceResourceType: sourceType,
		keys:               keys.Default(),
	}
}

// Init implements the sub-component init pattern. No async work — checkers are dispatched by app.go.
func (m RightColumnModel) Init() (RightColumnModel, tea.Cmd) {
	return m, nil
}

// Update handles key navigation and result delivery.
func (m RightColumnModel) Update(msg tea.Msg) (RightColumnModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateKeyMsg(msg)

	case messages.RelatedCheckResult:
		// Match rows by DefDisplayName: it is unique per RelatedDef and handles
		// the ct-events self-pivot case where 4 rows all share
		// TargetType="ct-events" but carry distinct DisplayNames ("CT events by
		// AccessKeyId/Username/EventName/SharedEventId"). Production messages
		// always carry DefDisplayName (app_related.go sets it in every dispatch).
		targetIdx := -1
		for i := range m.rows {
			if m.rows[i].displayName == msg.DefDisplayName {
				targetIdx = i
				break
			}
		}
		// Tight unambiguous fallback for messages without DefDisplayName: match
		// by TargetType ONLY when exactly one row carries that TargetType. If
		// multiple rows share the TargetType, we refuse to bind rather than
		// silently pick the wrong row — that ambiguity is a contract violation
		// and must be surfaced, not hidden behind "whichever row happens to be
		// loading first". Production code always populates DefDisplayName, so
		// this branch is test-surface only.
		if targetIdx < 0 && msg.DefDisplayName == "" {
			matches := 0
			firstIdx := -1
			for i := range m.rows {
				if m.rows[i].targetType == msg.Result.TargetType {
					if firstIdx < 0 {
						firstIdx = i
					}
					matches++
				}
			}
			if matches == 1 {
				targetIdx = firstIdx
			}
		}
		if targetIdx >= 0 {
			m.rows[targetIdx].loading = false
			m.rows[targetIdx].err = msg.Result.Err
			m.rows[targetIdx].count = msg.Result.Count
			m.rows[targetIdx].resourceIDs = msg.Result.ResourceIDs
			m.rows[targetIdx].fetchFilter = msg.Result.FetchFilter
			m.rows[targetIdx].approximate = msg.Result.Approximate
		}
		// Keep selection on an actionable row when possible.
		m.ensureCursorValid()
	}
	return m, nil
}

func (m RightColumnModel) updateKeyMsg(msg tea.KeyMsg) (RightColumnModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	if key.Matches(msg, m.keys.Escape) && strings.TrimSpace(m.filterQuery) != "" {
		m.filterActive = false
		m.filterQuery = ""
		m.scrollOffset = 0
		m.ensureCursorValid()
		return m, nil
	}

	if m.filterActive {
		k := msg.Key()
		switch {
		case key.Matches(msg, m.keys.Escape):
			m.filterActive = false
			m.filterQuery = ""
			m.scrollOffset = 0
			m.ensureCursorValid()
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			m.filterActive = false
			return m, nil
		case key.Matches(msg, m.keys.Up):
			m.moveCursor(-1)
			return m, nil
		case key.Matches(msg, m.keys.Down):
			m.moveCursor(1)
			return m, nil
		}
		if k.Code == tea.KeyBackspace {
			if len(m.filterQuery) > 0 {
				m.filterQuery = m.filterQuery[:len(m.filterQuery)-1]
				m.scrollOffset = 0
				m.ensureCursorValid()
			}
			return m, nil
		}
		if k.Text != "" {
			m.filterQuery += k.Text
			m.scrollOffset = 0
			m.ensureCursorValid()
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Search):
		m.filterActive = true
		m.filterQuery = ""
		m.scrollOffset = 0
		return m, nil
	case key.Matches(msg, m.keys.Down):
		m.moveCursor(1)
	case key.Matches(msg, m.keys.Up):
		m.moveCursor(-1)
	case key.Matches(msg, m.keys.Enter):
		if row := m.SelectedRow(); row != nil && isActionableRow(*row) {
			return m, func() tea.Msg {
				return messages.RelatedNavigate{
					TargetType:     row.targetType,
					SourceResource: m.parentRes,
					RelatedIDs:     row.resourceIDs,
					FetchFilter:    row.fetchFilter,
					Checker:        row.checker,
				}
			}
		}
	}
	return m, nil
}

// View renders the right column content (no frame - frame is added externally).
// It is a thin adapter over the shared renderRelatedPanel (detail_helpers.go):
// visible rows are mapped to []app.RelatedBlock in visibleIndexes() order,
// count badge and actionability come from the same shared rules
// (resource.FormatRelatedCount / isActionableRow) so the TUI, the headless
// controller, and the web renderer cannot drift - this is the single render
// implementation; renderDetailRelatedFromBody calls the same function.
func (m RightColumnModel) View() string {
	if m.width <= 0 {
		return ""
	}

	visible := m.visibleIndexes()
	rows := make([]app.RelatedBlock, len(visible))
	cursor := -1
	for i, idx := range visible {
		row := m.rows[idx]
		rows[i] = app.RelatedBlock{
			Name:         row.displayName,
			Loading:      row.loading,
			Err:          row.err != nil,
			CountDisplay: resource.FormatRelatedCount(row.count),
			Actionable:   isActionableRow(row),
		}
		if idx == m.cursor {
			cursor = i
		}
	}

	// filterActive here means "rows exist but the visible/filtered set is
	// empty" - the same condition RightColumnModel used to render "No
	// matches" (as opposed to "No related types registered" when there were
	// no rows at all).
	filterActive := len(m.rows) > 0 && len(visible) == 0

	return renderRelatedPanel(rows, filterActive, cursor, m.scrollOffset, m.focused, m.width, m.height)
}

// SetSize sets the rendering dimensions.
func (m *RightColumnModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetFocused sets whether this column has keyboard focus.
func (m *RightColumnModel) SetFocused(focused bool) {
	m.focused = focused
	if focused {
		m.ensureCursorValid()
	}
}

// IsFocused reports whether this column has keyboard focus.
func (m RightColumnModel) IsFocused() bool {
	return m.focused
}

// SelectedRow returns a pointer to the currently selected row, or nil if the cursor is out of range.
func (m RightColumnModel) SelectedRow() *rightColumnRow {
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		return &m.rows[m.cursor]
	}
	return nil
}

// SelectedTypeName returns the display name of the currently selected row, or "" if none.
func (m RightColumnModel) SelectedTypeName() string {
	row := m.SelectedRow()
	if row == nil {
		return ""
	}
	return row.displayName
}

// isActionableRow reports whether the right-column row is drillable. The rule
// itself lives in resource.IsRelatedActionable so the TUI, the headless
// controller (ActionRelatedSelect / RelatedBlock.Actionable), and the web
// renderer all share one definition and cannot drift (see that func for the
// per-case rationale).
func isActionableRow(row rightColumnRow) bool {
	return resource.IsRelatedActionable(row.count, row.approximate, len(row.fetchFilter) > 0, row.loading, row.err != nil)
}

// isSelfPivotZeroRow reports whether a row is a self-pivot row (its TargetType equals
// the source resource type) that has resolved with count=0 and no error.
// Self-pivot rows are filters (navigate to a filtered self-list), not counts —
// showing "(0)" for a self-pivot is semantically meaningless and must be hidden.
// Non-self target types (e.g. "ec2" rows visible on a different source type) always
// remain visible even when their count is 0.
func (m RightColumnModel) isSelfPivotZeroRow(row rightColumnRow) bool {
	return !row.loading &&
		row.err == nil &&
		row.count == 0 &&
		m.sourceResourceType != "" &&
		row.targetType == m.sourceResourceType
}

func (m RightColumnModel) visibleIndexes() []int {
	if len(m.rows) == 0 {
		return nil
	}
	query := strings.TrimSpace(strings.ToLower(m.filterQuery))
	if query == "" {
		idx := make([]int, 0, len(m.rows))
		for i, row := range m.rows {
			if !m.isSelfPivotZeroRow(row) {
				idx = append(idx, i)
			}
		}
		return idx
	}
	idx := make([]int, 0, len(m.rows))
	for i, row := range m.rows {
		if !m.isSelfPivotZeroRow(row) && strings.Contains(strings.ToLower(row.displayName), query) {
			idx = append(idx, i)
		}
	}
	return idx
}

func (m *RightColumnModel) ensureCursorValid() {
	visible := m.visibleIndexes()
	if len(visible) == 0 {
		m.cursor = 0
		m.scrollOffset = 0
		return
	}
	isVisible := slices.Contains(visible, m.cursor)
	if !isVisible {
		m.cursor = visible[0]
	}
	// Prefer first actionable visible row when actionable rows exist.
	hasActionable := false
	for _, idx := range visible {
		if isActionableRow(m.rows[idx]) {
			hasActionable = true
			break
		}
	}
	if hasActionable {
		if row := m.SelectedRow(); row == nil || !isActionableRow(*row) {
			for _, idx := range visible {
				if isActionableRow(m.rows[idx]) {
					m.cursor = idx
					break
				}
			}
		}
	}
	m.ensureScrollVisible()
}

func (m *RightColumnModel) ensureScrollVisible() {
	visible := m.visibleIndexes()
	if len(visible) == 0 {
		return
	}
	usableHeight := max(m.height-1, 1)
	selectedPos := 0
	for i, idx := range visible {
		if idx == m.cursor {
			selectedPos = i
			break
		}
	}
	if selectedPos < m.scrollOffset {
		m.scrollOffset = selectedPos
	}
	if selectedPos >= m.scrollOffset+usableHeight {
		m.scrollOffset = selectedPos - usableHeight + 1
	}
	m.scrollOffset = max(m.scrollOffset, 0)
	m.scrollOffset = min(m.scrollOffset, len(visible)-1)
}

func (m *RightColumnModel) moveCursor(dir int) {
	visible := m.visibleIndexes()
	if len(visible) == 0 {
		return
	}
	pos := -1
	for i, idx := range visible {
		if idx == m.cursor {
			pos = i
			break
		}
	}
	if pos < 0 {
		pos = 0
	}
	for {
		next := pos + dir
		if next < 0 || next >= len(visible) {
			return
		}
		pos = next
		idx := visible[pos]
		if isActionableRow(m.rows[idx]) {
			m.cursor = idx
			m.ensureScrollVisible()
			return
		}
	}
}

func (m RightColumnModel) IsFiltering() bool {
	return m.filterActive
}

func (m RightColumnModel) FilterQuery() string {
	return m.filterQuery
}

func (m RightColumnModel) HasFilter() bool {
	return strings.TrimSpace(m.filterQuery) != ""
}

// HasActionableRows reports whether the right column is worth focusing.
// Loading rows remain focusable so users can inspect and filter while checks run.
// Fully-resolved all-zero rows are not focusable.
func (m RightColumnModel) HasActionableRows() bool {
	for _, idx := range m.visibleIndexes() {
		if m.rows[idx].loading || isActionableRow(m.rows[idx]) {
			return true
		}
	}
	return false
}
