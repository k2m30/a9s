package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/messages"
	"github.com/k2m30/a9s/internal/tui/styles"
)

// SelectorModel is a generic list selector used for AWS profiles and regions.
// It replaces the separate ProfileModel and RegionModel with a single
// reusable component.
type SelectorModel struct {
	scroll        ScrollState
	allItems      []string
	filteredItems []string
	filterText    string
	activeItem    string // shows "(current)" indicator
	title         string // e.g. "aws-profiles" or "aws-regions"
	onSelect      func(string) tea.Msg
	width, height int
	keys          keys.Map
}

// NewSelector creates a SelectorModel with the given items, active item,
// title, selection callback, and key bindings.
func NewSelector(items []string, activeItem, title string, onSelect func(string) tea.Msg, k keys.Map) SelectorModel {
	return SelectorModel{
		scroll:        NewScrollState(len(items)),
		allItems:      items,
		filteredItems: items,
		activeItem:    activeItem,
		title:         title,
		onSelect:      onSelect,
		keys:          k,
	}
}

// NewProfile returns a SelectorModel configured for AWS profile selection.
func NewProfile(profiles []string, activeProfile string, k keys.Map) SelectorModel {
	return NewSelector(profiles, activeProfile, "aws-profiles", func(s string) tea.Msg {
		return messages.ProfileSelectedMsg{Profile: s}
	}, k)
}

// NewRegion returns a SelectorModel configured for AWS region selection.
func NewRegion(regions []string, activeRegion string, k keys.Map) SelectorModel {
	return NewSelector(regions, activeRegion, "aws-regions", func(s string) tea.Msg {
		return messages.RegionSelectedMsg{Region: s}
	}, k)
}

// Init implements the view initialization pattern.
func (m SelectorModel) Init() (SelectorModel, tea.Cmd) {
	return m, nil
}

// Update handles navigation and selection.
func (m SelectorModel) Update(msg tea.Msg) (SelectorModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
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
			m.scroll.PageUp(m.height)
		case key.Matches(msg, m.keys.PageDown):
			m.scroll.PageDown(m.height)
		case key.Matches(msg, m.keys.Enter):
			if len(m.filteredItems) > 0 && m.scroll.Cursor() < len(m.filteredItems) {
				selected := m.filteredItems[m.scroll.Cursor()]
				onSelect := m.onSelect
				return m, func() tea.Msg {
					return onSelect(selected)
				}
			}
		}
	}
	return m, nil
}

// View renders the selector list with viewport windowing.
func (m SelectorModel) View() string {
	if len(m.filteredItems) == 0 {
		return "No items available"
	}

	startRow, endRow := m.scroll.VisibleWindow(m.height)

	var sb strings.Builder
	for i := startRow; i < endRow; i++ {
		if i > startRow {
			sb.WriteString("\n")
		}

		item := m.filteredItems[i]
		label := "  " + item
		if item == m.activeItem {
			label += " " + styles.DimText.Render("(current)")
		}

		if i == m.scroll.Cursor() {
			sb.WriteString(styles.RowSelected.Width(m.width).Render(label))
		} else {
			sb.WriteString(styles.RowNormal.Render(label))
		}
	}

	return sb.String()
}

// SetSize updates dimensions.
func (m *SelectorModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// FrameTitle returns e.g. "aws-profiles(6)" or "aws-regions(3/17)".
func (m SelectorModel) FrameTitle() string {
	total := len(m.allItems)
	filtered := len(m.filteredItems)
	if m.filterText != "" && filtered != total {
		return m.title + "(" + itoa(filtered) + "/" + itoa(total) + ")"
	}
	return m.title + "(" + itoa(total) + ")"
}

// CopyContent returns empty — nothing to copy from the selector.
func (m SelectorModel) CopyContent() (string, string) {
	return "", ""
}

// GetHelpContext returns HelpFromSelector.
func (m SelectorModel) GetHelpContext() HelpContext {
	return HelpFromSelector
}

// Title returns the selector's title (e.g. "aws-profiles" or "aws-regions").
func (m SelectorModel) Title() string {
	return m.title
}

// SetFilter applies a filter to items; cursor resets to 0.
func (m *SelectorModel) SetFilter(text string) {
	m.filterText = text
	m.applyFilter()
	m.scroll.SetCursor(0)
}

// GetFilter returns the current filter text.
func (m *SelectorModel) GetFilter() string {
	return m.filterText
}

// applyFilter filters allItems into filteredItems.
func (m *SelectorModel) applyFilter() {
	if m.filterText == "" {
		m.filteredItems = m.allItems
		m.scroll.SetTotal(len(m.allItems))
		return
	}
	q := strings.ToLower(m.filterText)
	result := make([]string, 0)
	for _, item := range m.allItems {
		if strings.Contains(strings.ToLower(item), q) {
			result = append(result, item)
		}
	}
	m.filteredItems = result
	m.scroll.SetTotal(len(result))
}
