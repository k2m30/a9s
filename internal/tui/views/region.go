package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/messages"
	"github.com/k2m30/a9s/internal/tui/styles"
)

// RegionModel is a tea.Model for the AWS region selector.
type RegionModel struct {
	allRegions      []string
	filteredRegions []string
	filterText      string
	activeRegion    string
	cursor          int
	width           int
	height          int
	keys            keys.Map
}

// NewRegion returns a RegionModel.
func NewRegion(regions []string, activeRegion string, k keys.Map) RegionModel {
	return RegionModel{
		allRegions:      regions,
		filteredRegions: regions,
		activeRegion:    activeRegion,
		keys:            k,
	}
}

// Init implements tea.Model.
func (m RegionModel) Init() (RegionModel, tea.Cmd) {
	return m, nil
}

// Update handles navigation and selection.
func (m RegionModel) Update(msg tea.Msg) (RegionModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.filteredRegions)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Enter):
			if len(m.filteredRegions) > 0 && m.cursor < len(m.filteredRegions) {
				selected := m.filteredRegions[m.cursor]
				return m, func() tea.Msg {
					return messages.RegionSelectedMsg{Region: selected}
				}
			}
		}
	}
	return m, nil
}

// View renders the region list.
func (m RegionModel) View() string {
	if len(m.filteredRegions) == 0 {
		return "No regions available"
	}

	var sb strings.Builder
	for i, r := range m.filteredRegions {
		if i > 0 {
			sb.WriteString("\n")
		}

		label := "  " + r
		if r == m.activeRegion {
			label += " " + styles.DimText.Render("(current)")
		}

		if i == m.cursor {
			sb.WriteString(styles.RowSelected.Width(m.width).Render(label))
		} else {
			sb.WriteString(styles.RowNormal.Render(label))
		}
	}

	return sb.String()
}

// SetSize updates dimensions.
func (m *RegionModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// FrameTitle returns e.g. "aws-regions(17)".
func (m RegionModel) FrameTitle() string {
	total := len(m.allRegions)
	filtered := len(m.filteredRegions)
	if m.filterText != "" && filtered != total {
		return "aws-regions(" + itoa(filtered) + "/" + itoa(total) + ")"
	}
	return "aws-regions(" + itoa(total) + ")"
}

// CopyContent returns empty — nothing to copy from the region selector.
func (m RegionModel) CopyContent() (string, string) {
	return "", ""
}

// GetHelpContext returns HelpFromSelector.
func (m RegionModel) GetHelpContext() HelpContext {
	return HelpFromSelector
}

// SetFilter applies a filter to regions; cursor resets to 0.
func (m *RegionModel) SetFilter(text string) {
	m.filterText = text
	m.applyFilter()
	m.cursor = 0
}

// applyFilter filters allRegions into filteredRegions.
func (m *RegionModel) applyFilter() {
	if m.filterText == "" {
		m.filteredRegions = m.allRegions
		return
	}
	q := strings.ToLower(m.filterText)
	result := make([]string, 0)
	for _, r := range m.allRegions {
		if strings.Contains(strings.ToLower(r), q) {
			result = append(result, r)
		}
	}
	m.filteredRegions = result
}
