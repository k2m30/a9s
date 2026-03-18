package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/layout"
	"github.com/k2m30/a9s/internal/tui/messages"
	"github.com/k2m30/a9s/internal/tui/styles"
)

// MainMenuModel displays the resource type selection list.
type MainMenuModel struct {
	allItems      []resource.ResourceTypeDef
	filteredItems []resource.ResourceTypeDef
	filterText    string
	cursor        int
	width         int
	height        int
	keys          keys.Map
}

// NewMainMenu returns an initialized MainMenuModel with all resource types.
func NewMainMenu(k keys.Map) MainMenuModel {
	all := resource.AllResourceTypes()
	return MainMenuModel{
		allItems:      all,
		filteredItems: all,
		keys:          k,
	}
}

// Init implements tea.Model. No async work needed.
func (m MainMenuModel) Init() (MainMenuModel, tea.Cmd) {
	return m, nil
}

// Update handles navigation keys. Enter sends NavigateMsg to push resource list.
func (m MainMenuModel) Update(msg tea.Msg) (MainMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.filteredItems)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Top):
			m.cursor = 0
		case key.Matches(msg, m.keys.Bottom):
			if len(m.filteredItems) > 0 {
				m.cursor = len(m.filteredItems) - 1
			}
		case key.Matches(msg, m.keys.Enter):
			if len(m.filteredItems) > 0 && m.cursor < len(m.filteredItems) {
				selected := m.filteredItems[m.cursor]
				return m, func() tea.Msg {
					return messages.NavigateMsg{
						Target:       messages.TargetResourceList,
						ResourceType: selected.ShortName,
					}
				}
			}
		}
	}
	return m, nil
}

// View renders the menu items. Caller wraps in RenderFrame.
func (m MainMenuModel) View() string {
	if len(m.filteredItems) == 0 {
		return "No resource types"
	}

	// Alias column width: widest alias is ":secrets" = 8, plus trailing pad.
	const aliasW = 9

	var sb strings.Builder
	for i, item := range m.filteredItems {
		if i > 0 {
			sb.WriteString("\n")
		}

		aliasStr := ":" + item.ShortName
		aliasPadded := layout.PadOrTrunc(aliasStr, aliasW)

		// Name field fills remaining width: total - 2 leading - aliasW - 5 trailing.
		nameFieldW := m.width - 2 - aliasW - 5
		if nameFieldW < 10 {
			nameFieldW = 10
		}
		namePadded := layout.PadOrTrunc(item.Name, nameFieldW)

		if i == m.cursor {
			// Selected row: full highlight, alias stays dimmed.
			dimAlias := styles.DimText.Render(aliasPadded)
			selectedName := "  " + namePadded + " "
			line := styles.RowSelected.Width(m.width).Render(selectedName + dimAlias)
			sb.WriteString(line)
		} else {
			dimAlias := styles.DimText.Render(aliasPadded)
			name := styles.RowNormal.Render("  " + namePadded + " ")
			sb.WriteString(name + dimAlias)
		}
	}

	return sb.String()
}

// SetSize updates terminal dimensions.
func (m *MainMenuModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// FrameTitle returns the frame border title.
func (m MainMenuModel) FrameTitle() string {
	total := len(m.allItems)
	filtered := len(m.filteredItems)
	if m.filterText != "" {
		return "resource-types(" + itoa(filtered) + "/" + itoa(total) + ")"
	}
	return "resource-types(" + itoa(total) + ")"
}

// CopyContent returns empty — nothing to copy from the main menu.
func (m MainMenuModel) CopyContent() (string, string) {
	return "", ""
}

// GetHelpContext returns HelpFromMainMenu.
func (m MainMenuModel) GetHelpContext() HelpContext {
	return HelpFromMainMenu
}

// SelectedItem returns the resource type at the current cursor.
func (m MainMenuModel) SelectedItem() resource.ResourceTypeDef {
	if len(m.filteredItems) == 0 {
		return resource.ResourceTypeDef{}
	}
	return m.filteredItems[m.cursor]
}

// SetFilter applies a filter to the menu items; cursor resets to 0.
func (m *MainMenuModel) SetFilter(text string) {
	m.filterText = text
	m.applyFilter()
	m.cursor = 0
}

// applyFilter filters allItems into filteredItems by case-insensitive substring match.
// Requires at least 2 characters to actually filter; single chars are too ambiguous
// for the short list of resource types.
func (m *MainMenuModel) applyFilter() {
	if len(m.filterText) < 2 {
		m.filteredItems = m.allItems
		return
	}
	q := strings.ToLower(m.filterText)
	result := make([]resource.ResourceTypeDef, 0)
	for _, item := range m.allItems {
		if strings.Contains(strings.ToLower(item.Name), q) ||
			strings.Contains(strings.ToLower(item.ShortName), q) {
			result = append(result, item)
		}
	}
	m.filteredItems = result
}
