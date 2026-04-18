// Package views — rightColumnModel renders the RELATED panel in the detail view.
package views

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

type rightColumnRow struct {
	targetType  string
	displayName string
	count       int               // -1 = loading, 0+ = resolved
	resourceIDs []string          // IDs from checker result (for navigation in US3)
	fetchFilter map[string]string // server-side filter for filtered paginated fetcher
	loading     bool
	err         error
	approximate bool // true when count was derived from a truncated cache; UI renders "N+"
}

type rightColumnModel struct {
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

// newRightColumn constructs a rightColumnModel from related definitions and a parent resource.
// sourceType is the short name of the resource type being detailed (e.g. "ct-events").
// All rows start in loading state; checkers are dispatched by app.go.
func newRightColumn(defs []resource.RelatedDef, parentRes resource.Resource, sourceType string) rightColumnModel {
	rows := make([]rightColumnRow, len(defs))
	for i, def := range defs {
		rows[i] = rightColumnRow{
			targetType:  def.TargetType,
			displayName: def.DisplayName,
			count:       -1,
			loading:     true,
		}
	}
	return rightColumnModel{
		rows:               rows,
		parentRes:          parentRes,
		sourceResourceType: sourceType,
		keys:               keys.Default(),
	}
}

// Init implements the sub-component init pattern. No async work — checkers are dispatched by app.go.
func (m rightColumnModel) Init() (rightColumnModel, tea.Cmd) {
	return m, nil
}

// Update handles key navigation and result delivery.
func (m rightColumnModel) Update(msg tea.Msg) (rightColumnModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.updateKeyMsg(msg)

	case messages.RelatedCheckResultMsg:
		// When multiple rows share the same TargetType (e.g. the 4 ct-events
		// self-pivot rows all have TargetType="ct-events"), results arrive as
		// separate messages. To resolve all rows, prefer the first STILL-LOADING
		// row with the matching TargetType (FIFO). Only fall back to the first row
		// of that type if all are already resolved (late-arriving duplicate).
		// Match by DefDisplayName which is unique per def (multiple defs can share TargetType,
		// e.g. the 4 ct-events self-pivots all have TargetType="ct-events" but distinct
		// display names "CT events by AccessKeyId/Username/EventName/SharedEventId").
		targetIdx := -1
		for i := range m.rows {
			if m.rows[i].displayName == msg.DefDisplayName {
				targetIdx = i
				break
			}
		}
		// Fallback for messages without DefDisplayName (legacy/test injection):
		// use the old loading-first-match-by-TargetType logic.
		if targetIdx < 0 && msg.DefDisplayName == "" {
			for i := range m.rows {
				if m.rows[i].targetType == msg.Result.TargetType {
					if m.rows[i].loading {
						targetIdx = i
						break
					}
					if targetIdx < 0 {
						targetIdx = i
					}
				}
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

func (m rightColumnModel) updateKeyMsg(msg tea.KeyMsg) (rightColumnModel, tea.Cmd) {
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
				return messages.RelatedNavigateMsg{
					TargetType:     row.targetType,
					SourceResource: m.parentRes,
					RelatedIDs:     row.resourceIDs,
					FetchFilter:    row.fetchFilter,
				}
			}
		}
	}
	return m, nil
}

// View renders the right column content (no frame — frame is added externally).
func (m rightColumnModel) View() string {
	if m.width <= 0 {
		return ""
	}

	lines := make([]string, 0, m.height)

	// Header: "RELATED" centered.
	header := "RELATED"
	padLeft := (m.width - lipgloss.Width(header)) / 2
	padLeft = max(padLeft, 0)
	centeredHeader := strings.Repeat(" ", padLeft) + header
	lines = append(lines, styles.DimText.Render(centeredHeader))

	visible := m.visibleIndexes()
	switch {
	case len(m.rows) == 0:
		lines = append(lines, styles.DimText.Render("  No related types registered"))
	case len(visible) == 0:
		lines = append(lines, styles.DimText.Render("  No matches"))
	default:
		usableHeight := max(m.height-1, 1) // after header

		start := m.scrollOffset
		end := min(start+usableHeight, len(visible))

		for _, idx := range visible[start:end] {
			row := m.rows[idx]
			var rowText string
			var rowStyle lipgloss.Style

			switch {
			case row.loading:
				rowText = "  " + row.displayName
				rowStyle = styles.DimText
			case row.err != nil:
				rowText = "  " + row.displayName + "  \u2014" // em dash
				rowStyle = styles.DimText
			case row.count == -1 && len(row.fetchFilter) > 0:
				rowText = "  " + row.displayName
				rowStyle = styles.RowNormal
			case row.count == -1:
				rowText = "  " + row.displayName
				rowStyle = styles.DimText
			case row.count == 0 && row.approximate:
				rowText = "  " + row.displayName + " (0+)"
				rowStyle = styles.RowNormal
			case row.count == 0:
				rowText = "  " + row.displayName + " (0)"
				rowStyle = styles.DimText
			case row.approximate:
				rowText = "  " + row.displayName + " (" + fmt.Sprintf("%d", row.count) + "+)"
				rowStyle = styles.RowNormal
			default:
				rowText = "  " + row.displayName + " (" + fmt.Sprintf("%d", row.count) + ")"
				rowStyle = styles.RowNormal
			}

			if m.focused && m.cursor == idx {
				lines = append(lines, styles.RowSelected.Width(m.width).Render(rowText))
			} else {
				lines = append(lines, rowStyle.Render(rowText))
			}
		}
	}

	// Pad remaining height with empty strings.
	for len(lines) < m.height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// SetSize sets the rendering dimensions.
func (m *rightColumnModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// SetFocused sets whether this column has keyboard focus.
func (m *rightColumnModel) SetFocused(focused bool) {
	m.focused = focused
	if focused {
		m.ensureCursorValid()
	}
}

// IsFocused reports whether this column has keyboard focus.
func (m rightColumnModel) IsFocused() bool {
	return m.focused
}

// SelectedRow returns a pointer to the currently selected row, or nil if the cursor is out of range.
func (m rightColumnModel) SelectedRow() *rightColumnRow {
	if m.cursor >= 0 && m.cursor < len(m.rows) {
		return &m.rows[m.cursor]
	}
	return nil
}

// SelectedTypeName returns the display name of the currently selected row, or "" if none.
func (m rightColumnModel) SelectedTypeName() string {
	row := m.SelectedRow()
	if row == nil {
		return ""
	}
	return row.displayName
}

func isActionableRow(row rightColumnRow) bool {
	if row.loading || row.err != nil {
		return false
	}
	if len(row.fetchFilter) > 0 {
		// Any row with a server-side filter is navigable — the fetch determines
		// the real count.
		return true
	}
	if row.count == -1 {
		// Unknown without a fetchFilter → not drillable.
		return false
	}
	if row.approximate {
		// 0+ or N+ — reverse-scan lower bound. Navigable to the target-type
		// list; the target list view can filter client-side.
		return true
	}
	return row.count > 0
}

// isSelfPivotZeroRow reports whether a row is a self-pivot row (its TargetType equals
// the source resource type) that has resolved with count=0 and no error.
// Self-pivot rows are filters (navigate to a filtered self-list), not counts —
// showing "(0)" for a self-pivot is semantically meaningless and must be hidden.
// Non-self target types (e.g. "ec2" rows visible on a different source type) always
// remain visible even when their count is 0.
func (m rightColumnModel) isSelfPivotZeroRow(row rightColumnRow) bool {
	return !row.loading &&
		row.err == nil &&
		row.count == 0 &&
		m.sourceResourceType != "" &&
		row.targetType == m.sourceResourceType
}

func (m rightColumnModel) visibleIndexes() []int {
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

func (m *rightColumnModel) ensureCursorValid() {
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

func (m *rightColumnModel) ensureScrollVisible() {
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

func (m *rightColumnModel) moveCursor(dir int) {
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
	hasActionable := false
	for _, idx := range visible {
		if isActionableRow(m.rows[idx]) {
			hasActionable = true
			break
		}
	}
	for {
		next := pos + dir
		if next < 0 || next >= len(visible) {
			return
		}
		pos = next
		idx := visible[pos]
		if !hasActionable || isActionableRow(m.rows[idx]) {
			m.cursor = idx
			m.ensureScrollVisible()
			return
		}
	}
}

func (m rightColumnModel) IsFiltering() bool {
	return m.filterActive
}

func (m rightColumnModel) FilterQuery() string {
	return m.filterQuery
}

func (m rightColumnModel) HasFilter() bool {
	return strings.TrimSpace(m.filterQuery) != ""
}

// HasActionableRows reports whether the right column is worth focusing.
// Loading rows remain focusable so users can inspect and filter while checks run.
// Fully-resolved all-zero rows are not focusable.
func (m rightColumnModel) HasActionableRows() bool {
	for _, idx := range m.visibleIndexes() {
		if m.rows[idx].loading || isActionableRow(m.rows[idx]) {
			return true
		}
	}
	return false
}
