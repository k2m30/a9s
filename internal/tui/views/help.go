package views

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/messages"
	"github.com/k2m30/a9s/internal/tui/styles"
	"github.com/k2m30/a9s/internal/tui/text"
)

// HelpContext identifies which view opened help so keys can be filtered.
type HelpContext int

const (
	HelpFromMainMenu     HelpContext = iota // main menu view
	HelpFromResourceList                    // resource list (non-secrets)
	HelpFromSecretsList                     // secrets resource list (includes reveal)
	HelpFromDetail                          // detail view
	HelpFromYAML                            // yaml view
	HelpFromSelector                        // profile or region selector
	HelpFromReveal                          // reveal view
)

// HelpModel renders context-sensitive keybinding reference inside the frame.
// Any key press closes help (parent pops the view).
type HelpModel struct {
	keys    keys.Map
	context HelpContext
	width   int
	height  int
}

// NewHelp returns a HelpModel with the given view context.
func NewHelp(k keys.Map, ctx HelpContext) HelpModel {
	return HelpModel{keys: k, context: ctx}
}

// Init implements tea.Model.
func (m HelpModel) Init() (HelpModel, tea.Cmd) {
	return m, nil
}

// Update handles any key press by sending PopViewMsg.
func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	switch msg.(type) {
	case tea.KeyMsg:
		return m, func() tea.Msg {
			return messages.PopViewMsg{}
		}
	}
	return m, nil
}

// Help view styles — created once, not per frame.
var (
	helpCatStyle  = lipgloss.NewStyle().Foreground(styles.ColHelpCat).Bold(true)
	helpKeyStyle  = lipgloss.NewStyle().Foreground(styles.ColHelpKey).Bold(true)
	helpDescStyle = lipgloss.NewStyle().Foreground(styles.ColDetailVal)
)

// helpBinding is a single key-description pair for rendering.
type helpBinding struct {
	key  string
	desc string
}

// View renders context-sensitive keybinding layout.
func (m HelpModel) View() string {
	catStyle := helpCatStyle
	hkStyle := helpKeyStyle
	descStyle := helpDescStyle

	bind := func(k, d string) string {
		return hkStyle.Render(text.PadOrTrunc(k, 9)) + descStyle.Render(d)
	}

	groups := m.buildGroups()

	// Determine number of columns from groups
	numCols := len(groups)
	if numCols == 0 {
		return ""
	}

	colW := (m.width - 6) / numCols
	if colW < 12 {
		colW = 12
	}

	// Build category header row
	var catParts []string
	for i, g := range groups {
		if i < numCols-1 {
			catParts = append(catParts, catStyle.Render(text.PadOrTrunc(g.title, colW)))
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
				parts = append(parts, text.PadOrTrunc(cell, colW))
			} else {
				parts = append(parts, cell)
			}
		}
		sb.WriteString(" " + strings.Join(parts, ""))
	}

	sb.WriteString("\n\n")
	closeHint := styles.DimText.Render("Press any key to close")
	sb.WriteString(lipgloss.Place(m.width, 1, lipgloss.Center, lipgloss.Top, closeHint))

	return sb.String()
}

// helpGroup is a titled column of key bindings.
type helpGroup struct {
	title    string
	bindings []helpBinding
}

// buildGroups returns the column groups appropriate for the current context.
func (m HelpModel) buildGroups() []helpGroup {
	switch m.context {
	case HelpFromMainMenu:
		return m.mainMenuGroups()
	case HelpFromResourceList:
		return m.resourceListGroups(false)
	case HelpFromSecretsList:
		return m.resourceListGroups(true)
	case HelpFromDetail:
		return m.detailGroups()
	case HelpFromYAML:
		return m.yamlGroups()
	case HelpFromSelector:
		return m.selectorGroups()
	case HelpFromReveal:
		return m.revealGroups()
	default:
		return m.mainMenuGroups()
	}
}

func (m HelpModel) mainMenuGroups() []helpGroup {
	return []helpGroup{
		{
			title: "NAVIGATION",
			bindings: []helpBinding{
				{"j/k", "up/down"},
				{"g", "top"},
				{"G", "bottom"},
				{"pgup", "page up"},
				{"pgdn", "page down"},
			},
		},
		{
			title: "ACTIONS",
			bindings: []helpBinding{
				{"enter", "select"},
				{"/", "filter"},
				{":", "command"},
				{"q", "quit"},
				{"ctrl+c", "force quit"},
			},
		},
		{
			title: "OTHER",
			bindings: []helpBinding{
				{"?", "help"},
				{"esc", "back"},
			},
		},
	}
}

func (m HelpModel) resourceListGroups(secrets bool) []helpGroup {
	nav := helpGroup{
		title: "NAVIGATION",
		bindings: []helpBinding{
			{"j/k", "up/down"},
			{"g/G", "top/bottom"},
			{"pgup", "page up"},
			{"pgdn", "page down"},
			{"h/l", "scroll cols"},
		},
	}

	actions := helpGroup{
		title: "ACTIONS",
		bindings: []helpBinding{
			{"enter/d", "detail"},
			{"y", "yaml"},
			{"c", "copy id"},
			{"/", "filter"},
			{":", "command"},
		},
	}
	if secrets {
		actions.bindings = append(actions.bindings, helpBinding{"x", "reveal"})
	}

	sortGroup := helpGroup{
		title: "SORT",
		bindings: []helpBinding{
			{"N", "sort name"},
			{"I", "sort id"},
			{"A", "sort date"},
		},
	}

	other := helpGroup{
		title: "OTHER",
		bindings: []helpBinding{
			{"ctrl+r", "refresh"},
			{"esc", "back"},
			{"q", "quit"},
			{"?", "help"},
		},
	}

	return []helpGroup{nav, actions, sortGroup, other}
}

func (m HelpModel) detailGroups() []helpGroup {
	return []helpGroup{
		{
			title: "SCROLL",
			bindings: []helpBinding{
				{"j/k", "up/down"},
				{"g", "top"},
				{"G", "bottom"},
			},
		},
		{
			title: "ACTIONS",
			bindings: []helpBinding{
				{"y", "yaml"},
				{"c", "copy yaml"},
				{"w", "wrap toggle"},
			},
		},
		{
			title: "OTHER",
			bindings: []helpBinding{
				{"esc", "back"},
				{"?", "help"},
			},
		},
	}
}

func (m HelpModel) yamlGroups() []helpGroup {
	return []helpGroup{
		{
			title: "SCROLL",
			bindings: []helpBinding{
				{"j/k", "up/down"},
				{"g", "top"},
				{"G", "bottom"},
			},
		},
		{
			title: "ACTIONS",
			bindings: []helpBinding{
				{"c", "copy yaml"},
				{"w", "wrap toggle"},
			},
		},
		{
			title: "OTHER",
			bindings: []helpBinding{
				{"esc", "back"},
				{"?", "help"},
			},
		},
	}
}

func (m HelpModel) selectorGroups() []helpGroup {
	return []helpGroup{
		{
			title: "NAVIGATION",
			bindings: []helpBinding{
				{"j/k", "up/down"},
				{"g", "top"},
				{"G", "bottom"},
				{"/", "filter"},
			},
		},
		{
			title: "ACTIONS",
			bindings: []helpBinding{
				{"enter", "select"},
				{"esc", "cancel"},
			},
		},
		{
			title: "OTHER",
			bindings: []helpBinding{
				{"?", "help"},
			},
		},
	}
}

func (m HelpModel) revealGroups() []helpGroup {
	return []helpGroup{
		{
			title: "ACTIONS",
			bindings: []helpBinding{
				{"c", "copy secret"},
				{"esc", "close"},
			},
		},
		{
			title: "OTHER",
			bindings: []helpBinding{
				{"?", "help"},
			},
		},
	}
}

// CopyContent returns empty — nothing to copy from the help view.
func (m HelpModel) CopyContent() (string, string) {
	return "", ""
}

// GetHelpContext returns the context this help was opened from.
func (m HelpModel) GetHelpContext() HelpContext {
	return m.context
}

// SetSize updates layout dimensions.
func (m *HelpModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// FrameTitle returns "help".
func (m HelpModel) FrameTitle() string {
	return "help"
}
