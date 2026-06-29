package views

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

// MainMenuModel is a thin delegating renderer. The app.Controller is the single
// source of truth for all menu data (items, filter, cursor, availability, issues).
// MainMenuModel owns only renderer state: terminal dimensions, scroll offset, and
// the key map used to translate key presses into controller actions.
type MainMenuModel struct {
	scrollOffset int
	width        int
	height       int
	keys         keys.Map
	ctrl         *app.Controller
}

// NewMainMenu returns an initialized MainMenuModel. The ctrl argument is variadic
// for backward-compatibility: callers that pass no controller (e.g. isolated unit
// tests) get an auto-constructed stub controller backed by the full resource
// catalog so all Set*/Get* methods work identically to the production path.
// In production tui.New() always passes an explicit controller.
func NewMainMenu(k keys.Map, ctrl ...*app.Controller) MainMenuModel {
	var c *app.Controller
	if len(ctrl) > 0 {
		c = ctrl[0]
	}
	if c == nil {
		c = app.New(runtime.Bootstrap("", "", resource.AllResourceTypes()))
	}
	return MainMenuModel{
		keys: k,
		ctrl: c,
	}
}

// Init implements tea.Model. No async work needed.
func (m MainMenuModel) Init() (MainMenuModel, tea.Cmd) {
	return m, nil
}

// Update handles navigation keys by translating them into controller actions.
// Enter emits a Navigate message directly (navigation stays TUI-side).
func (m MainMenuModel) Update(msg tea.Msg) (MainMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveUp})
		case key.Matches(msg, m.keys.Down):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveDown})
		case key.Matches(msg, m.keys.Top):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveTop})
		case key.Matches(msg, m.keys.Bottom):
			m.ctrl.Apply(app.Action{Kind: app.ActionMoveBottom})
		case key.Matches(msg, m.keys.PageUp):
			m.ctrl.Apply(app.Action{Kind: app.ActionPageUp, N: max(m.height-1, 1)})
		case key.Matches(msg, m.keys.PageDown):
			m.ctrl.Apply(app.Action{Kind: app.ActionPageDown, N: max(m.height-1, 1)})
		case key.Matches(msg, m.keys.Enter):
			selected, navigable := m.ctrl.MenuSelected()
			if !navigable {
				return m, nil
			}
			return m, func() tea.Msg {
				return messages.Navigate{
					Target:       messages.TargetResourceList,
					ResourceType: selected.ShortName,
				}
			}
		case key.Matches(msg, m.keys.ToggleAttentionOnly):
			m.ctrl.Apply(app.Action{Kind: app.ActionToggleAttention})
		}
		// Adjust scroll to keep cursor visible after any key action.
		body := m.ctrl.Snapshot().Body.Menu
		if body != nil {
			m.adjustScrollForBody(*body)
		}
	}
	return m, nil
}

// adjustScrollForBody ensures the cursor is visible within the viewport,
// accounting for category header lines. Mirrors old adjustScroll but uses
// controller-supplied body data.
func (m *MainMenuModel) adjustScrollForBody(body app.MenuBody) {
	if m.height <= 0 {
		return
	}
	lines := buildRenderLinesFromEntries(body.Entries)
	cursorLine := 0
	for i, rl := range lines {
		if !rl.isHeader && rl.itemIndex == body.Selected {
			cursorLine = i
			break
		}
	}
	if cursorLine < m.scrollOffset {
		m.scrollOffset = cursorLine
		if m.scrollOffset > 0 && lines[m.scrollOffset-1].isHeader {
			m.scrollOffset--
		}
	}
	if cursorLine >= m.scrollOffset+m.height {
		m.scrollOffset = cursorLine - m.height + 1
	}
}

// View renders the menu by delegating entirely to the controller snapshot.
// The controller is the single source of truth; no data is read from the model.
func (m *MainMenuModel) View() string {
	body := m.ctrl.Snapshot().Body.Menu
	if body == nil {
		return "No resource types"
	}
	m.adjustScrollForBody(*body)
	return m.RenderBody(*body)
}

// SetSize updates terminal dimensions.
func (m *MainMenuModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// FrameTitle delegates to the controller.
func (m MainMenuModel) FrameTitle() string {
	return m.ctrl.MenuFrameTitle()
}

// BottomHints implements Hintable for MainMenuModel.
func (m MainMenuModel) BottomHints() []layout.KeyHint {
	// Single-sourced in app.MenuFooterHints, shared with the web footer
	// (ViewState.Footer) so the two renderers cannot drift.
	src := app.MenuFooterHints()
	hints := make([]layout.KeyHint, len(src))
	for i, h := range src {
		hints[i] = layout.KeyHint{Key: h.Key, Desc: h.Help}
	}
	return hints
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
	item, _ := m.ctrl.MenuSelected()
	return item
}

// SetFilter delegates filter updates to the controller.
func (m *MainMenuModel) SetFilter(filterText string) {
	m.ctrl.Apply(app.Action{Kind: app.ActionSetFilter, Arg: filterText})
	m.scrollOffset = 0
}

// GetFilter returns the current filter text from the controller snapshot.
func (m *MainMenuModel) GetFilter() string {
	body := m.ctrl.Snapshot().Body.Menu
	if body == nil {
		return ""
	}
	return body.Filter
}

// SetAvailability updates the resource count for a resource type.
// Paired with the current truncated value from the controller to satisfy the
// PatchMenuAvailability pairing requirement (Count + Truncated travel together).
func (m *MainMenuModel) SetAvailability(shortName string, count int) {
	truncated := m.ctrl.GetMenuTruncated()[shortName]
	m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PatchMenuAvailability{
		ResourceType: shortName,
		Count:        count,
		Truncated:    truncated,
	}})
}

// ClearAvailability resets all availability and issue state via the controller.
func (m *MainMenuModel) ClearAvailability() {
	m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.MenuClearAvailabilityIntent{}})
}

// GetAvailability returns a copy of the availability map from the controller.
func (m *MainMenuModel) GetAvailability() map[string]int {
	return m.ctrl.GetMenuAvailability()
}

// SetTruncated records whether a resource type's count is truncated.
// Paired with the current availability count from the controller (PatchMenuAvailability
// carries Count + Truncated together).
func (m *MainMenuModel) SetTruncated(shortName string, truncated bool) {
	count := m.ctrl.GetMenuAvailability()[shortName]
	m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PatchMenuAvailability{
		ResourceType: shortName,
		Count:        count,
		Truncated:    truncated,
	}})
}

// GetTruncated returns a copy of the truncated map from the controller.
func (m *MainMenuModel) GetTruncated() map[string]bool {
	return m.ctrl.GetMenuTruncated()
}

// SetCheckProgress updates the background check progress indicator.
func (m *MainMenuModel) SetCheckProgress(checked, total int) {
	m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PatchMenuCheckProgress{
		Checked: checked,
		Total:   total,
	}})
}

// SetIssues updates the issue count for a resource type.
func (m *MainMenuModel) SetIssues(shortName string, count int, truncated bool) {
	m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PatchMenu{
		ResourceType: shortName,
		Issues:       count,
		Truncated:    truncated,
	}})
}

// SetIssuesFromCache bulk-loads issue counts from cache.
// A nil known map is a no-op; an empty (non-nil) known map initializes
// the issue maps to empty, matching the pre-controller behavior.
func (m *MainMenuModel) SetIssuesFromCache(counts map[string]int, truncated map[string]bool, known map[string]bool) {
	if known == nil {
		return
	}
	m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PatchMenuIssueBatch{
		Counts:    counts,
		Truncated: truncated,
		Known:     known,
	}})
}

// GetIssueCounts returns a copy of the issue-count map from the controller.
func (m *MainMenuModel) GetIssueCounts() map[string]int {
	return m.ctrl.GetMenuIssueCounts()
}

// GetIssueTruncated returns a copy of the issue-truncated map from the controller.
func (m *MainMenuModel) GetIssueTruncated() map[string]bool {
	return m.ctrl.GetMenuIssueTruncated()
}

// GetIssueKnown returns a copy of the issue-known map from the controller.
func (m *MainMenuModel) GetIssueKnown() map[string]bool {
	return m.ctrl.GetMenuIssueKnown()
}

// SetEnrichProgress updates Wave 2 enrichment progress counters.
func (m *MainMenuModel) SetEnrichProgress(checked, total int) {
	m.ctrl.ApplyIntents([]runtime.UIIntent{runtime.PatchMenuEnrichProgress{
		Checked: checked,
		Total:   total,
	}})
}

// Toggle flips the attention-only filter via the controller.
func (m *MainMenuModel) Toggle() {
	m.ctrl.Apply(app.Action{Kind: app.ActionToggleAttention})
}

// IsEnabled reports whether the attention-only filter is currently active.
func (m MainMenuModel) IsEnabled() bool {
	body := m.ctrl.Snapshot().Body.Menu
	if body == nil {
		return false
	}
	return body.AttentionOnly
}

// SetEnabled sets the attention-only filter to the given state via the controller.
// Calling SetEnabled(true) when already true (or false when already false) is a
// no-op because ActionToggleAttention flips the current state; this method reads
// the current state and only applies the toggle when it would change the value.
func (m *MainMenuModel) SetEnabled(enabled bool) {
	if m.IsEnabled() != enabled {
		m.ctrl.Apply(app.Action{Kind: app.ActionToggleAttention})
	}
}

// renderLine represents a single line in the menu: either a category header or a selectable item.
type renderLine struct {
	isHeader  bool
	header    string
	itemIndex int
}

// RenderBody renders the menu from a controller-supplied MenuBody, byte-identical
// to the old View(). The controller owns the logical state (visible entries,
// selection, availability/issue badges); the renderer owns scrollOffset and dimensions.
func (m *MainMenuModel) RenderBody(body app.MenuBody) string {
	if len(body.Entries) == 0 {
		return "No resource types"
	}

	const aliasW = 15

	lines := buildRenderLinesFromEntries(body.Entries)

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

		item := body.Entries[rl.itemIndex]
		aliasPadded := text.PadOrTrunc(item.Alias, aliasW)
		nameFieldW := max(m.width-4-aliasW-3, 10)

		nameStr := item.Display
		if item.AvailKnown {
			countSuffix := " (" + itoa(item.Availability) + ")"
			if item.AvailTruncated {
				countSuffix = " (" + itoa(item.Availability) + "+)"
			}
			nameStr += countSuffix
		}
		nameStr += entryIssueBadge(item)
		namePadded := text.PadOrTrunc(nameStr, nameFieldW)

		if rl.itemIndex == body.Selected {
			dimAlias := styles.DimText.Render(aliasPadded)
			selectedName := "    " + namePadded + " "
			sb.WriteString(styles.RowSelected.Width(m.width).Render(selectedName + dimAlias))
			continue
		}

		dimAlias := styles.DimText.Render(aliasPadded)
		if item.AvailKnown && item.Availability == 0 && !item.AvailTruncated {
			sb.WriteString(styles.DimText.Render("    "+namePadded+" ") + dimAlias)
		} else {
			sb.WriteString(styles.RowNormal.Render("    "+namePadded+" ") + dimAlias)
		}
	}

	return sb.String()
}

// buildRenderLinesFromEntries builds the flat header+item line list from
// controller-supplied entries, inserting a category header when Category changes.
func buildRenderLinesFromEntries(entries []app.MenuEntry) []renderLine {
	lines := make([]renderLine, 0, len(entries)+12)
	lastCat := ""
	for i, item := range entries {
		if item.Category != lastCat {
			lines = append(lines, renderLine{isHeader: true, header: item.Category})
			lastCat = item.Category
		}
		lines = append(lines, renderLine{itemIndex: i})
	}
	return lines
}

// entryIssueBadge mirrors issueBadge() for a controller-supplied entry: the
// " issues:N" suffix, only when the count is positive.
func entryIssueBadge(e app.MenuEntry) string {
	if e.IssueBadge.Count <= 0 {
		return ""
	}
	return " issues:" + itoa(e.IssueBadge.Count)
}
