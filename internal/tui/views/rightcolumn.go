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
	rows      []rightColumnRow
	cursor    int
	focused   bool
	width     int
	height    int
	parentRes resource.Resource // stored for RelatedNavigateMsg construction
	keys      keys.Map
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
	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}
		switch {
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Enter):
			if row := m.SelectedRow(); row != nil && !row.loading && row.err == nil && row.count > 0 {
				return m, func() tea.Msg {
					return messages.RelatedNavigateMsg{
						TargetType:     row.targetType,
						SourceResource: m.parentRes,
						RelatedIDs:     row.resourceIDs,
					}
				}
			}
		}

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

	if len(m.rows) == 0 {
		lines = append(lines, styles.DimText.Render("  No related types registered"))
	} else {
		for i, row := range m.rows {
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

			if m.focused && m.cursor == i {
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
