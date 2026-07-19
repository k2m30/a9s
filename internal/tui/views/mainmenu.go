// SPDX-License-Identifier: GPL-3.0-or-later

package views

import (
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
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
			if selected.ShortName == app.CostsMenuShortName {
				return m, func() tea.Msg {
					return messages.Navigate{Target: messages.TargetCosts}
				}
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

// SetSize updates terminal dimensions.
func (m *MainMenuModel) SetSize(w, h int) {
	m.width = w
	m.height = h
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
			countSuffix := " (" + strconv.Itoa(item.Availability) + ")"
			if item.AvailTruncated {
				countSuffix = " (" + strconv.Itoa(item.Availability) + "+)"
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
		// DEF-6/C3: a disk-cache-seeded, not-yet-re-verified count (Origin ==
		// "cache") dims the same as a confirmed-empty entry — both are "not
		// yet a confirmed answer this session" states the operator should be
		// able to tell apart from a verified one at a glance.
		confirmedEmpty := item.AvailKnown && item.Availability == 0 && !item.AvailTruncated
		cacheOrigin := item.Origin == "cache"
		if confirmedEmpty || cacheOrigin {
			sb.WriteString(styles.DimText.Render("    "+namePadded+" ") + dimAlias)
		} else {
			sb.WriteString(styles.RowNormal.Render("    "+namePadded+" ") + dimAlias)
		}
	}

	// Contract C: a background availability sweep is still confirming/
	// replacing cache-seeded startup counts. Additive-only — never renders
	// when Refreshing is false (the default), so it does not affect existing
	// render-parity assertions.
	if body.Refreshing {
		sb.WriteString("\n")
		sb.WriteString(styles.DimText.Render("  refreshing…"))
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

// entryIssueBadge renders the " issues:N" suffix for a controller-supplied
// entry, only when the count is positive. A truncated count is a lower bound
// (the list has unfetched pages, so more issue rows may exist) and gets a "+"
// suffix — mirroring the availability "(N+)" marker and the web menu template.
// (Owner decision 2026-07-08 supersedes the earlier "truncation is behavioral,
// never rendered" rule; the list-view ⓘ banner stays forbidden — that is a
// separate mechanism.)
func entryIssueBadge(e app.MenuEntry) string {
	if e.IssueBadge.Count <= 0 {
		return ""
	}
	suffix := " issues:" + strconv.Itoa(e.IssueBadge.Count)
	if e.IssueBadge.Truncated {
		suffix += "+"
	}
	return suffix
}
