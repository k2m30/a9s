// SPDX-License-Identifier: GPL-3.0-or-later

package views

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/text"
)

// HelpContext identifies which view opened help so keys can be filtered.
type HelpContext int

const (
	HelpFromMainMenu              HelpContext = iota // main menu view
	HelpFromResourceList                             // resource list (non-secrets)
	HelpFromSecretsList                              // secrets resource list (includes reveal)
	HelpFromDetail                                   // detail view
	HelpFromYAML                                     // yaml view
	HelpFromJSON                                     // json view
	HelpFromSelector                                 // profile or region selector
	HelpFromReveal                                   // reveal view
	HelpFromResourceListPaginated                    // paginated resource list (includes M)
	HelpFromSecretsListPaginated                     // paginated secrets list (includes M and x)
	HelpFromCosts                                    // cost explorer view
)

// Aliases: reveal-list help contexts (resource types with reveal fetchers).
const HelpFromRevealList = HelpFromSecretsList
const HelpFromRevealListPaginated = HelpFromSecretsListPaginated

// HelpModel renders context-sensitive keybinding reference inside the frame.
// Any key press closes help (parent pops the view).
type HelpModel struct {
	keys              keys.Map
	context           HelpContext
	resourceShortName string
	width             int
	height            int
}

// NewHelpWithResource returns a HelpModel scoped to a specific resource type.
// When ctx is HelpFromResourceList or HelpFromResourceListPaginated and
// resourceShortName is "ct-events", a CloudTrail Events legend is appended.
func NewHelpWithResource(k keys.Map, ctx HelpContext, resourceShortName string) HelpModel {
	return HelpModel{keys: k, context: ctx, resourceShortName: resourceShortName}
}

// Update handles any key press by sending PopViewMsg.
func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		return m, func() tea.Msg {
			return messages.PopView{}
		}
	}
	return m, nil
}

// helpBinding is a single key-description pair for rendering.
type helpBinding struct {
	key  string
	desc string
}

// domainContext maps the TUI's HelpContext (which screen opened help) to the
// renderer-neutral domain.HelpContext the shared key table is keyed on.
func (c HelpContext) domainContext() domain.HelpContext {
	switch c {
	case HelpFromResourceList:
		return domain.HelpFromResourceList
	case HelpFromSecretsList:
		return domain.HelpFromSecretsList
	case HelpFromResourceListPaginated:
		return domain.HelpFromResourceListPaginated
	case HelpFromSecretsListPaginated:
		return domain.HelpFromSecretsListPaginated
	case HelpFromDetail:
		return domain.HelpFromDetail
	case HelpFromYAML:
		return domain.HelpFromYAML
	case HelpFromJSON:
		return domain.HelpFromJSON
	case HelpFromSelector:
		return domain.HelpFromSelector
	case HelpFromReveal:
		return domain.HelpFromReveal
	case HelpFromCosts:
		return domain.HelpFromCosts
	default:
		return domain.HelpFromMainMenu
	}
}

// View renders context-sensitive keybinding layout.
func (m HelpModel) View() string {
	catStyle := styles.HelpCatStyle
	hkStyle := styles.HelpKeyStyle
	descStyle := styles.HelpDescStyle

	bind := func(k, d string) string {
		// Keep descriptors intact; truncation of ANSI-styled cells can clip
		// important help words in narrow layouts.
		return hkStyle.Render(text.PadOrTrunc(k, 9)) + descStyle.Render(d)
	}
	// padCell always appends at least a two-space gap, even when s already
	// meets or exceeds w — without it, a cell whose content is as wide as
	// or wider than its column budget (e.g. "page down" abutting the next
	// column's "ctrl+c") fuses directly into the next column with zero
	// separation.
	padCell := func(s string, w int) string {
		visible := lipgloss.Width(s)
		if visible >= w {
			return s + "  "
		}
		return s + strings.Repeat(" ", w-visible)
	}

	groups := m.buildGroups()

	// Determine number of columns from groups
	numCols := len(groups)
	if numCols == 0 {
		return ""
	}

	colW := max((m.width-6)/numCols, 12)

	// Build category header row
	var catParts []string
	for i, g := range groups {
		if i < numCols-1 {
			catParts = append(catParts, padCell(catStyle.Render(g.title), colW))
		} else {
			catParts = append(catParts, catStyle.Render(g.title))
		}
	}
	catRow := strings.Join(catParts, "")

	// Find the maximum number of bindings in any column
	maxRows := 0
	for _, g := range groups {
		if len(g.bindings) > maxRows {
			maxRows = len(g.bindings)
		}
	}

	var sb strings.Builder
	sb.WriteString(" " + catRow)
	sb.WriteString("\n")

	for row := 0; row < maxRows; row++ {
		sb.WriteString("\n")
		var parts []string
		for i, g := range groups {
			var cell string
			if row < len(g.bindings) {
				cell = bind(g.bindings[row].key, g.bindings[row].desc)
			}
			if i < numCols-1 {
				parts = append(parts, padCell(cell, colW))
			} else {
				parts = append(parts, cell)
			}
		}
		sb.WriteString(" " + strings.Join(parts, ""))
	}

	// Append CloudTrail Events legend when context + resource match.
	if (m.context == HelpFromResourceList || m.context == HelpFromResourceListPaginated) &&
		m.resourceShortName == "ct-events" {
		sb.WriteString("\n")
		sb.WriteString(m.ctEventsLegend())
	}

	sb.WriteString("\n\n")
	themeLine := styles.DimText.Render("Theme: " + styles.ActiveTheme().Name)
	sb.WriteString(lipgloss.Place(m.width, 1, lipgloss.Center, lipgloss.Top, themeLine))
	sb.WriteString("\n")
	closeHint := styles.DimText.Render("Press any key to close")
	sb.WriteString(lipgloss.Place(m.width, 1, lipgloss.Center, lipgloss.Top, closeHint))

	return sb.String()
}

// helpGroup is a titled column of key bindings.
type helpGroup struct {
	title    string
	bindings []helpBinding
}

// buildGroups returns the column groups appropriate for the current context,
// sourced from the shared domain.HelpGroupsFor table (single source of truth
// for help-overlay key/description content; see core/domain/helpkeys.go).
func (m HelpModel) buildGroups() []helpGroup {
	sections := domain.HelpGroupsFor(m.context.domainContext(), m.keys.ToggleAttentionOnly.Help().Key)
	groups := make([]helpGroup, len(sections))
	for i, s := range sections {
		bindings := make([]helpBinding, len(s.Hints))
		for j, h := range s.Hints {
			bindings[j] = helpBinding{key: h.Key, desc: h.Help}
		}
		groups[i] = helpGroup{title: s.Title, bindings: bindings}
	}
	return groups
}

// SetSize updates layout dimensions.
func (m *HelpModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}
