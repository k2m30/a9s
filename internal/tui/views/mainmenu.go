package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

// MainMenuModel displays the resource type selection list.
type MainMenuModel struct {
	allItems      []resource.ResourceTypeDef
	filteredItems []resource.ResourceTypeDef
	filterText    string
	scroll        ScrollState
	scrollOffset  int
	width         int
	height        int
	keys          keys.Map

	// renderLinesCache caches the flat list of render lines (category headers + items).
	// Invalidated when filteredItems changes (in applyFilter, SetFilter).
	renderLinesCache []renderLine
}

// NewMainMenu returns an initialized MainMenuModel with all resource types.
func NewMainMenu(k keys.Map) MainMenuModel {
	all := resource.AllResourceTypes()
	return MainMenuModel{
		allItems:      all,
		filteredItems: all,
		scroll:        NewScrollState(len(all)),
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
			m.scroll.Up()
		case key.Matches(msg, m.keys.Down):
			m.scroll.Down()
		case key.Matches(msg, m.keys.Top):
			m.scroll.Top()
		case key.Matches(msg, m.keys.Bottom):
			m.scroll.Bottom()
		case key.Matches(msg, m.keys.PageUp):
			pageSize := m.height - 1
			if pageSize < 1 {
				pageSize = 1
			}
			m.scroll.PageUp(pageSize)
		case key.Matches(msg, m.keys.PageDown):
			pageSize := m.height - 1
			if pageSize < 1 {
				pageSize = 1
			}
			m.scroll.PageDown(pageSize)
		case key.Matches(msg, m.keys.Enter):
			c := m.scroll.Cursor()
			if len(m.filteredItems) > 0 && c < len(m.filteredItems) {
				selected := m.filteredItems[c]
				return m, func() tea.Msg {
					return messages.NavigateMsg{
						Target:       messages.TargetResourceList,
						ResourceType: selected.ShortName,
					}
				}
			}
		}
		// Keep cursor visible in the viewport using render-line positions.
		m.adjustScroll()
	}
	return m, nil
}

// adjustScroll ensures the cursor is visible within the viewport, accounting
// for category header lines that occupy space but are not selectable.
func (m *MainMenuModel) adjustScroll() {
	if m.height <= 0 {
		return
	}
	lines := m.buildRenderLines()
	// Find the render-line index of the cursor.
	cursorLine := 0
	for i, rl := range lines {
		if !rl.isHeader && rl.itemIndex == m.scroll.Cursor() {
			cursorLine = i
			break
		}
	}
	if cursorLine < m.scrollOffset {
		m.scrollOffset = cursorLine
		// Include the category header above if the cursor is the first item in a group.
		if m.scrollOffset > 0 && lines[m.scrollOffset-1].isHeader {
			m.scrollOffset--
		}
	}
	if cursorLine >= m.scrollOffset+m.height {
		m.scrollOffset = cursorLine - m.height + 1
	}
}

// renderLine represents a single line in the menu: either a category header or a selectable item.
type renderLine struct {
	isHeader  bool
	header    string // category name, only set when isHeader is true
	itemIndex int    // index into filteredItems, only meaningful when isHeader is false
}

// buildRenderLines builds the flat list of render lines from filteredItems,
// inserting category headers when the category changes between consecutive items.
// Results are cached in renderLinesCache until filteredItems changes.
func (m *MainMenuModel) buildRenderLines() []renderLine {
	if m.renderLinesCache != nil {
		return m.renderLinesCache
	}
	lines := make([]renderLine, 0, len(m.filteredItems)+12)
	lastCat := ""
	for i, item := range m.filteredItems {
		if item.Category != lastCat {
			lines = append(lines, renderLine{isHeader: true, header: item.Category})
			lastCat = item.Category
		}
		lines = append(lines, renderLine{itemIndex: i})
	}
	m.renderLinesCache = lines
	return lines
}

// View renders the menu items. Caller wraps in RenderFrame.
// Only lines within the visible viewport (scrollOffset..scrollOffset+height) are rendered.
func (m MainMenuModel) View() string {
	if len(m.filteredItems) == 0 {
		return "No resource types"
	}

	// Alias column width: widest alias is ":codeartifact" = 13, plus trailing pad.
	const aliasW = 15

	lines := m.buildRenderLines()

	// Calculate visible window
	start := m.scrollOffset
	end := len(lines)
	if m.height > 0 && start+m.height < end {
		end = start + m.height
	}

	var sb strings.Builder
	for li := start; li < end; li++ {
		if li > start {
			sb.WriteString("\n")
		}
		rl := lines[li]

		if rl.isHeader {
			headerText := "  " + rl.header + " "
			sb.WriteString(styles.DimText.Render(headerText))
			continue
		}

		item := m.filteredItems[rl.itemIndex]

		aliasStr := ":" + item.ShortName
		aliasPadded := text.PadOrTrunc(aliasStr, aliasW)

		// Name field fills remaining width: total - 4 leading - aliasW - 3 trailing.
		nameFieldW := m.width - 4 - aliasW - 3
		if nameFieldW < 10 {
			nameFieldW = 10
		}
		namePadded := text.PadOrTrunc(item.Name, nameFieldW)

		if rl.itemIndex == m.scroll.Cursor() {
			// Selected row: full highlight, alias stays dimmed.
			dimAlias := styles.DimText.Render(aliasPadded)
			selectedName := "    " + namePadded + " "
			line := styles.RowSelected.Width(m.width).Render(selectedName + dimAlias)
			sb.WriteString(line)
		} else {
			dimAlias := styles.DimText.Render(aliasPadded)
			name := styles.RowNormal.Render("    " + namePadded + " ")
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
	return m.filteredItems[m.scroll.Cursor()]
}

// SetFilter applies a filter to the menu items; cursor and scroll reset to 0.
func (m *MainMenuModel) SetFilter(text string) {
	m.filterText = text
	m.renderLinesCache = nil
	m.applyFilter()
	m.scroll.SetCursor(0)
	m.scrollOffset = 0
}

// GetFilter returns the current filter text.
func (m *MainMenuModel) GetFilter() string {
	return m.filterText
}



// applyFilter filters allItems into filteredItems by case-insensitive substring match.
// Requires at least 2 characters to actually filter; single chars are too ambiguous
// for the short list of resource types.
func (m *MainMenuModel) applyFilter() {
	m.renderLinesCache = nil
	if len(m.filterText) < 2 {
		m.filteredItems = m.allItems
		m.scroll.SetTotal(len(m.filteredItems))
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
	m.scroll.SetTotal(len(m.filteredItems))
}
