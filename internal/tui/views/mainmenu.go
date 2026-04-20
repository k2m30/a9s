package views

import (
	"maps"
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

	// Issue tracking — parallel to availability maps.
	AttentionFilter                 // ctrl+z toggle for issue filter
	issueCounts     map[string]int  // per-type issue counts (red/yellow statuses)
	issueKnown      map[string]bool // per-type: true = probed, absent = unknown
	issueTruncated  map[string]bool // per-type: true = issue count is lower bound
	enrichChecked   int             // Wave 2 enrichment progress
	enrichTotal     int             // Wave 2 total enrichment probes
}

// NewMainMenu returns an initialized MainMenuModel with all registered resource types.
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
			pageSize := max(m.height-1, 1)
			m.scroll.PageUp(pageSize)
			m.skipUnavailable(-1)
		case key.Matches(msg, m.keys.PageDown):
			pageSize := max(m.height-1, 1)
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
		case key.Matches(msg, m.keys.ToggleAttentionOnly):
			m.Toggle()
			m.applyFilter()
			m.scroll.SetCursor(0)
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
		isTruncated := m.truncated != nil && m.truncated[item.ShortName]
		if count, known := m.availability[item.ShortName]; !known || count > 0 || isTruncated {
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
func (m *MainMenuModel) View() string {
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
		if len(item.Aliases) > 0 {
			aliasStr = ":" + item.Aliases[0]
		}
		aliasPadded := text.PadOrTrunc(aliasStr, aliasW)

		// Name field fills remaining width: total - 4 leading - aliasW - 3 trailing.
		nameFieldW := max(m.width-4-aliasW-3, 10)
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
			nameStr += m.issueBadge(item.ShortName)
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
			nameStr += m.issueBadge(item.ShortName)
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
	var title string
	switch {
	case m.filterText != "" || m.IsEnabled():
		title = "resource-types(" + itoa(filtered) + "/" + itoa(total) + ")"
	default:
		title = "resource-types(" + itoa(total) + ")"
	}
	if m.IsEnabled() {
		title += " [!]"
	}
	if m.enrichTotal > 0 && m.enrichChecked < m.enrichTotal {
		title += " [enriching " + itoa(m.enrichChecked) + "/" + itoa(m.enrichTotal) + "]"
	}
	return title
}

// BottomHints implements Hintable for MainMenuModel.
func (m MainMenuModel) BottomHints() []layout.KeyHint {
	return []layout.KeyHint{
		{Key: "ctrl+z", Desc: "Issues only"},
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

// ClearAvailability resets all availability and issue state (e.g., on profile/region switch).
func (m *MainMenuModel) ClearAvailability() {
	m.availability = nil
	m.truncated = nil
	m.availChecked = 0
	m.availTotal = 0
	// Clear issue state too — stale badges from a previous account must not survive.
	m.issueCounts = nil
	m.issueKnown = nil
	m.issueTruncated = nil
	m.enrichChecked = 0
	m.enrichTotal = 0
	m.applyFilter() // re-apply so ctrl+z visibility reflects the cleared state
}

// GetAvailability returns a copy of the availability map for cache persistence.
// Returns nil if no availability data has been set.
func (m *MainMenuModel) GetAvailability() map[string]int {
	if m.availability == nil {
		return nil
	}
	cp := make(map[string]int, len(m.availability))
	maps.Copy(cp, m.availability)
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
	maps.Copy(cp, m.truncated)
	return cp
}

// SetCheckProgress updates the background check progress indicator.
// checked=0, total=0 means "not checking" (hides the indicator).
func (m *MainMenuModel) SetCheckProgress(checked, total int) {
	m.availChecked = checked
	m.availTotal = total
}

// SetIssues updates the issue count for a resource type and marks it as known.
func (m *MainMenuModel) SetIssues(shortName string, count int, truncated bool) {
	if m.issueCounts == nil {
		m.issueCounts = make(map[string]int)
	}
	if m.issueKnown == nil {
		m.issueKnown = make(map[string]bool)
	}
	if m.issueTruncated == nil {
		m.issueTruncated = make(map[string]bool)
	}
	m.issueCounts[shortName] = count
	m.issueKnown[shortName] = true
	m.issueTruncated[shortName] = truncated
	// Reapply filter so ctrl+z quad-state reflects the new issue data.
	m.applyFilter()
}

// SetIssuesFromCache bulk-loads issue counts from cache, respecting known flags.
func (m *MainMenuModel) SetIssuesFromCache(counts map[string]int, truncated map[string]bool, known map[string]bool) {
	if m.issueCounts == nil {
		m.issueCounts = make(map[string]int)
	}
	if m.issueKnown == nil {
		m.issueKnown = make(map[string]bool)
	}
	if m.issueTruncated == nil {
		m.issueTruncated = make(map[string]bool)
	}
	for name, k := range known {
		if k {
			m.issueCounts[name] = counts[name]
			m.issueKnown[name] = true
			m.issueTruncated[name] = truncated[name]
		}
	}
	// Reapply filter so ctrl+z quad-state reflects the cached issue data.
	m.applyFilter()
}

// GetIssueCounts returns a copy of the per-type issue count map for cache persistence.
// Returns nil if no issue data has been set.
func (m *MainMenuModel) GetIssueCounts() map[string]int {
	if m.issueCounts == nil {
		return nil
	}
	cp := make(map[string]int, len(m.issueCounts))
	maps.Copy(cp, m.issueCounts)
	return cp
}

// GetIssueTruncated returns a copy of the per-type issue truncation map for cache persistence.
// Returns nil if no truncation data has been set.
func (m *MainMenuModel) GetIssueTruncated() map[string]bool {
	if m.issueTruncated == nil {
		return nil
	}
	cp := make(map[string]bool, len(m.issueTruncated))
	maps.Copy(cp, m.issueTruncated)
	return cp
}

// GetIssueKnown returns a copy of the per-type issue known map for cache persistence.
// Returns nil if no known data has been set.
func (m *MainMenuModel) GetIssueKnown() map[string]bool {
	if m.issueKnown == nil {
		return nil
	}
	cp := make(map[string]bool, len(m.issueKnown))
	maps.Copy(cp, m.issueKnown)
	return cp
}

// SetEnrichProgress updates the Wave 2 enrichment progress counters.
func (m *MainMenuModel) SetEnrichProgress(checked, total int) {
	m.enrichChecked = checked
	m.enrichTotal = total
}

// issueBadge returns the " issues:N" or " issues:N+" suffix for a resource type.
// Returns empty string if no issues are known.
func (m MainMenuModel) issueBadge(shortName string) string {
	var badge string
	if m.issueKnown[shortName] {
		count := m.issueCounts[shortName]
		if count > 0 {
			badge = " issues:" + itoa(count)
			if m.issueTruncated[shortName] {
				badge += "+"
			}
		}
	}
	return badge
}

// isVisibleUnderIssueFilter determines whether a resource type should be
// visible when the ctrl+z issue filter is active.
//
// Tri-state visibility (evaluated in order):
//  1. ExcludeFromIssueBadge types (e.g. ct-events) — never probed; hide always.
//  2. Unknown (not yet probed) — show ONLY while no type has been probed yet
//     (true cold-start). Once any probe has reported, unknown → hide so the
//     user sees a focused issues-only view instead of the whole unknown menu.
//  3. Has issues → show; zero + not truncated → hide (CONFIRMED zero);
//     zero + truncated → show (LOWER BOUND — unread pages may hold issues).
//
// Per docs/attention-signals.md, every registered resource type has at least
// a Wave 1 or Wave 2 signal; there is no "always healthy" class.
func (m MainMenuModel) isVisibleUnderIssueFilter(shortName string) bool {
	td := resource.FindResourceType(shortName)
	if td != nil && td.ExcludeFromIssueBadge {
		return false
	}
	if !m.issueKnown[shortName] {
		// Cold-start: no probe has reported anywhere → keep everything visible
		// so the user isn't greeted with an empty menu. Once any probe lands,
		// the filter tightens to "known-issue only".
		return len(m.issueKnown) == 0
	}
	if m.issueCounts[shortName] > 0 {
		return true
	}
	// Zero issues — truncated count is a LOWER BOUND; unread pages may carry
	// issues, so keep the type visible so the user can drill in.
	return m.issueTruncated[shortName]
}

// applyFilter filters allItems into filteredItems by case-insensitive substring match.
// Requires at least 2 characters to actually filter; single chars are too ambiguous
// for the short list of resource types.
func (m *MainMenuModel) applyFilter() {
	m.renderLinesCache = nil

	// First pass: text filter.
	var result []resource.ResourceTypeDef
	if len(m.filterText) < 2 {
		result = m.allItems
	} else {
		q := strings.ToLower(m.filterText)
		result = make([]resource.ResourceTypeDef, 0)
		for _, item := range m.allItems {
			if strings.Contains(strings.ToLower(item.Name), q) ||
				strings.Contains(strings.ToLower(item.ShortName), q) {
				result = append(result, item)
			}
		}
	}

	// Second pass: issue filter (ctrl+z) — quad-state visibility.
	if m.IsEnabled() {
		filtered := make([]resource.ResourceTypeDef, 0, len(result))
		for _, item := range result {
			if m.isVisibleUnderIssueFilter(item.ShortName) {
				filtered = append(filtered, item)
			}
		}
		result = filtered
	}

	m.filteredItems = result
	m.scroll.SetTotal(len(m.filteredItems))
}
