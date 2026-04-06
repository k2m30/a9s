package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
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

	// availability tracks resource counts per type.
	// Absent key = unknown (not yet checked, render normally).
	// Value 0 = empty (dimmed, skip-navigable).
	// Value > 0 = has resources (normal style, count shown).
	availability map[string]int

	// truncated tracks which resource types have truncated counts.
	// true means the count is from a first page only ("5+" style).
	truncated map[string]bool

	// availChecked / availTotal track background check progress.
	// Both zero means "not checking" or "done".
	availChecked int
	availTotal   int
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
			m.skipUnavailable(-1)
		case key.Matches(msg, m.keys.Down):
			m.scroll.Down()
			m.skipUnavailable(+1)
		case key.Matches(msg, m.keys.Top):
			m.scroll.Top()
			m.skipUnavailable(+1)
		case key.Matches(msg, m.keys.Bottom):
			m.scroll.Bottom()
			m.skipUnavailable(-1)
		case key.Matches(msg, m.keys.PageUp):
			pageSize := m.height - 1
			if pageSize < 1 {
				pageSize = 1
			}
			m.scroll.PageUp(pageSize)
			m.skipUnavailable(-1)
		case key.Matches(msg, m.keys.PageDown):
			pageSize := m.height - 1
			if pageSize < 1 {
				pageSize = 1
			}
			m.scroll.PageDown(pageSize)
			m.skipUnavailable(+1)
		case key.Matches(msg, m.keys.Enter):
			c := m.scroll.Cursor()
			if len(m.filteredItems) > 0 && c < len(m.filteredItems) {
				selected := m.filteredItems[c]
				// Block navigation only to confirmed-empty types.
				// Truncated-zero is not confirmed empty — more pages may exist.
				if m.availability != nil {
					isTruncated := m.truncated != nil && m.truncated[selected.ShortName]
					if count, known := m.availability[selected.ShortName]; known && count == 0 && !isTruncated {
						return m, nil
					}
				}
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

// skipUnavailable advances the cursor past empty resource types.
// direction is +1 (forward) or -1 (backward).
// If no navigable item is found in the given direction, tries the opposite.
// If ALL items are empty, gives up (avoids infinite loop).
func (m *MainMenuModel) skipUnavailable(direction int) {
	if m.availability == nil || len(m.filteredItems) == 0 {
		return
	}

	total := len(m.filteredItems)
	start := m.scroll.Cursor()

	// Try to find a navigable item in the given direction
	cur := start
	for cur >= 0 && cur < total {
		item := m.filteredItems[cur]
		isTruncated := m.truncated != nil && m.truncated[item.ShortName]
		if count, known := m.availability[item.ShortName]; !known || count > 0 || isTruncated {
			// This item is navigable: unknown, has resources, or page was truncated
			// (truncated-zero is not confirmed empty — more pages may exist).
			m.scroll.SetCursor(cur)
			return
		}
		cur += direction
	}

	// No navigable item found in that direction — try the opposite
	cur = start - direction
	for cur >= 0 && cur < total {
		item := m.filteredItems[cur]
		if count, known := m.availability[item.ShortName]; !known || count > 0 {
			m.scroll.SetCursor(cur)
			return
		}
		cur -= direction
	}

	// ALL items are empty — leave cursor where it is
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
		if rl.itemIndex == m.scroll.Cursor() {
			// Build name with count suffix if known
			nameStr := item.Name
			if m.availability != nil {
				if count, known := m.availability[item.ShortName]; known {
					countSuffix := " (" + itoa(count) + ")"
					if m.truncated != nil && m.truncated[item.ShortName] {
						countSuffix = " (" + itoa(count) + "+)"
					}
					nameStr += countSuffix
				}
			}
			namePadded := text.PadOrTrunc(nameStr, nameFieldW)

			// Selected row: full highlight, alias stays dimmed.
			dimAlias := styles.DimText.Render(aliasPadded)
			selectedName := "    " + namePadded + " "
			line := styles.RowSelected.Width(m.width).Render(selectedName + dimAlias)
			sb.WriteString(line)
		} else {
			// Check availability count
			count, known := -1, false
			if m.availability != nil {
				count, known = m.availability[item.ShortName]
			}

			// Build name with count suffix if known
			nameStr := item.Name
			if known {
				countSuffix := " (" + itoa(count) + ")"
				if m.truncated != nil && m.truncated[item.ShortName] {
					countSuffix = " (" + itoa(count) + "+)"
				}
				nameStr += countSuffix
			}
			namePadded := text.PadOrTrunc(nameStr, nameFieldW)

			isTruncated := m.truncated != nil && m.truncated[item.ShortName]
			if known && count == 0 && !isTruncated {
				// Confirmed-empty resource type — fully dimmed.
				// Truncated-zero is not dimmed; more pages may exist.
				dimAlias := styles.DimText.Render(aliasPadded)
				dimName := styles.DimText.Render("    " + namePadded + " ")
				sb.WriteString(dimName + dimAlias)
			} else {
				dimAlias := styles.DimText.Render(aliasPadded)
				name := styles.RowNormal.Render("    " + namePadded + " ")
				sb.WriteString(name + dimAlias)
			}
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

// BottomHints implements Hintable for MainMenuModel.
func (m MainMenuModel) BottomHints() []layout.KeyHint {
	return []layout.KeyHint{
		{Key: "ctrl+r", Desc: "Refresh"},
	}
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

// SetAvailability sets the resource count for a resource type.
func (m *MainMenuModel) SetAvailability(shortName string, count int) {
	if m.availability == nil {
		m.availability = make(map[string]int)
	}
	m.availability[shortName] = count
}

// ClearAvailability resets all availability state (e.g., on profile/region switch).
func (m *MainMenuModel) ClearAvailability() {
	m.availability = nil
	m.truncated = nil
	m.availChecked = 0
	m.availTotal = 0
}

// GetAvailability returns a copy of the availability map for cache persistence.
// Returns nil if no availability data has been set.
func (m *MainMenuModel) GetAvailability() map[string]int {
	if m.availability == nil {
		return nil
	}
	cp := make(map[string]int, len(m.availability))
	for k, v := range m.availability {
		cp[k] = v
	}
	return cp
}

// SetTruncated records whether a resource type's count is truncated.
func (m *MainMenuModel) SetTruncated(shortName string, truncated bool) {
	if m.truncated == nil {
		m.truncated = make(map[string]bool)
	}
	m.truncated[shortName] = truncated
}

// GetTruncated returns a copy of the truncated map for cache persistence.
// Returns nil if no truncation data has been set.
func (m *MainMenuModel) GetTruncated() map[string]bool {
	if m.truncated == nil {
		return nil
	}
	cp := make(map[string]bool, len(m.truncated))
	for k, v := range m.truncated {
		cp[k] = v
	}
	return cp
}

// SetCheckProgress updates the background check progress indicator.
// checked=0, total=0 means "not checking" (hides the indicator).
func (m *MainMenuModel) SetCheckProgress(checked, total int) {
	m.availChecked = checked
	m.availTotal = total
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
