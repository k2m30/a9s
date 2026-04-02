// Package views — rightColumnModel renders the RELATED panel in the detail view.
package views

import (
	"fmt"
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
	count       int      // -1 = loading, 0+ = resolved
	resourceIDs []string // IDs from checker result (for navigation in US3)
	loading     bool
	err         error
}

type rightColumnModel struct {
	rows         []rightColumnRow
	cursor       int
	focused      bool
	width        int
	height       int
	scrollOffset int
	filterQuery  string
	filterActive bool
	parentRes    resource.Resource // stored for RelatedNavigateMsg construction
	keys         keys.Map
}

// newRightColumn constructs a rightColumnModel from related definitions and a parent resource.
// All rows start in loading state; checkers are dispatched by app.go.
func newRightColumn(defs []resource.RelatedDef, parentRes resource.Resource) rightColumnModel {
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
		rows:      rows,
		parentRes: parentRes,
		keys:      keys.Default(),
	}
}

// Init implements the sub-component init pattern. No async work — checkers are dispatched by app.go.
func (m rightColumnModel) Init() (rightColumnModel, tea.Cmd) {
	return m, nil
}

// Update handles key navigation and result delivery.
func (m rightColumnModel) Update(msg tea.Msg) (rightColumnModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.updateKeyPressMsg(msg)
	case tea.KeyMsg:
		return m.updateKeyMsg(msg)

	case messages.RelatedCheckResultMsg:
		for i := range m.rows {
			if m.rows[i].targetType == msg.Result.TargetType {
				m.rows[i].loading = false
				m.rows[i].err = msg.Result.Err
				m.rows[i].count = msg.Result.Count
				m.rows[i].resourceIDs = msg.Result.ResourceIDs
				break
			}
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
		default:
			// In KeyMsg path, text updates are handled by KeyPressMsg.
			return m, nil
		}
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
				}
			}
		}
	}
	return m, nil
}

func (m rightColumnModel) updateKeyPressMsg(msg tea.KeyPressMsg) (rightColumnModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	if msg.Code == tea.KeyEscape && strings.TrimSpace(m.filterQuery) != "" {
		m.filterActive = false
		m.filterQuery = ""
		m.scrollOffset = 0
		m.ensureCursorValid()
		return m, nil
	}

	if m.filterActive {
		switch msg.Code {
		case tea.KeyEscape:
			m.filterActive = false
			m.filterQuery = ""
			m.scrollOffset = 0
			m.ensureCursorValid()
			return m, nil
		case tea.KeyEnter:
			m.filterActive = false
			return m, nil
		case tea.KeyBackspace:
			if len(m.filterQuery) > 0 {
				m.filterQuery = m.filterQuery[:len(m.filterQuery)-1]
				m.scrollOffset = 0
				m.ensureCursorValid()
			}
			return m, nil
		case tea.KeyUp:
			m.moveCursor(-1)
			return m, nil
		case tea.KeyDown:
			m.moveCursor(1)
			return m, nil
		}
		if msg.Text != "" {
			m.filterQuery += msg.Text
			m.scrollOffset = 0
			m.ensureCursorValid()
		}
		return m, nil
	}

	if msg.Text == "/" {
		m.filterActive = true
		m.filterQuery = ""
		m.scrollOffset = 0
		return m, nil
	}
	if msg.Text == "j" || msg.Code == tea.KeyDown {
		m.moveCursor(1)
		return m, nil
	}
	if msg.Text == "k" || msg.Code == tea.KeyUp {
		m.moveCursor(-1)
		return m, nil
	}
	if msg.Code == tea.KeyEnter {
		if row := m.SelectedRow(); row != nil && isActionableRow(*row) {
			return m, func() tea.Msg {
				return messages.RelatedNavigateMsg{
					TargetType:     row.targetType,
					SourceResource: m.parentRes,
					RelatedIDs:     row.resourceIDs,
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
	if padLeft < 0 {
		padLeft = 0
	}
	centeredHeader := strings.Repeat(" ", padLeft) + header
	lines = append(lines, styles.DimText.Render(centeredHeader))

	visible := m.visibleIndexes()
	if len(m.rows) == 0 {
		lines = append(lines, styles.DimText.Render("  No related types registered"))
	} else if len(visible) == 0 {
		lines = append(lines, styles.DimText.Render("  No matches"))
	} else {
		usableHeight := m.height - 1 // after header
		if usableHeight < 1 {
			usableHeight = 1
		}
		// Keep selected row visible within the rendering window.
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
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}
		if m.scrollOffset > len(visible)-1 {
			m.scrollOffset = len(visible) - 1
		}

		start := m.scrollOffset
		end := start + usableHeight
		if end > len(visible) {
			end = len(visible)
		}

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
			case row.count == -1:
				rowText = "  " + row.displayName
				rowStyle = styles.DimText
			case row.count == 0:
				rowText = "  " + row.displayName + " (0)"
				rowStyle = styles.DimText
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
	return !row.loading && row.err == nil && row.count > 0
}

func (m rightColumnModel) visibleIndexes() []int {
	if len(m.rows) == 0 {
		return nil
	}
	query := strings.TrimSpace(strings.ToLower(m.filterQuery))
	if query == "" {
		idx := make([]int, 0, len(m.rows))
		for i := range m.rows {
			idx = append(idx, i)
		}
		return idx
	}
	idx := make([]int, 0, len(m.rows))
	for i, row := range m.rows {
		if strings.Contains(strings.ToLower(row.displayName), query) {
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
	isVisible := false
	for _, idx := range visible {
		if idx == m.cursor {
			isVisible = true
			break
		}
	}
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
