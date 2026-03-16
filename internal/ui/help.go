package ui

import (
	"fmt"
	"os"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// HelpViewType identifies which set of keybindings to show.
type HelpViewType int

const (
	// GlobalHelp shows all keybindings (default/fallback).
	GlobalHelp HelpViewType = iota
	// MainMenuHelp shows main-menu-specific keys.
	MainMenuHelp
	// ListViewHelp shows list-view-specific keys.
	ListViewHelp
	// DetailViewHelp shows detail-view-specific keys.
	DetailViewHelp
	// JSONViewHelp shows JSON/YAML-view-specific keys.
	JSONViewHelp
)

// HelpModel holds the dimensions and context used to render the help overlay.
type HelpModel struct {
	Width    int
	Height   int
	ViewType HelpViewType
}

// NewHelpModel creates a new HelpModel with default dimensions.
func NewHelpModel() HelpModel {
	return HelpModel{}
}

// helpSection groups a category title with its keybinding entries.
type helpSection struct {
	Title    string
	Bindings []helpBinding
}

// helpBinding is a single key/description pair.
type helpBinding struct {
	Key  string
	Desc string
}

// View renders the help overlay as a bordered box with keybinding sections.
func (m HelpModel) View() string {
	sections := m.sectionsForView()

	noColor := os.Getenv("NO_COLOR") != ""

	var b strings.Builder
	b.WriteString("  Keybindings\n")

	for i, sec := range sections {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("  %s\n", sec.Title))
		for _, kb := range sec.Bindings {
			b.WriteString(fmt.Sprintf("    %-10s %s\n", kb.Key, kb.Desc))
		}
	}

	b.WriteString("\n  Press any key to close help\n")

	content := b.String()

	boxWidth := 42
	if m.Width > 0 && boxWidth > m.Width-4 {
		boxWidth = m.Width - 4
	}
	if boxWidth < 30 {
		boxWidth = 30
	}

	var boxStyle lipgloss.Style
	if noColor {
		boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 2).
			Width(boxWidth)
	} else {
		boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00ffff")).
			Padding(1, 2).
			Width(boxWidth)
	}

	return boxStyle.Render(content)
}

// sectionsForView returns the keybinding sections appropriate for the current view.
func (m HelpModel) sectionsForView() []helpSection {
	switch m.ViewType {
	case MainMenuHelp:
		return []helpSection{
			{
				Title: "Navigation",
				Bindings: []helpBinding{
					{Key: "j/\u2193", Desc: "down"},
					{Key: "k/\u2191", Desc: "up"},
					{Key: "g", Desc: "top"},
					{Key: "G", Desc: "bottom"},
					{Key: "Enter", Desc: "select"},
					{Key: "/", Desc: "filter"},
				},
			},
			{
				Title: "Commands",
				Bindings: []helpBinding{
					{Key: ":", Desc: "command"},
					{Key: "?", Desc: "help"},
					{Key: "q", Desc: "quit"},
					{Key: "Ctrl-C", Desc: "exit"},
				},
			},
		}

	case ListViewHelp:
		return []helpSection{
			{
				Title: "Navigation",
				Bindings: []helpBinding{
					{Key: "j/\u2193", Desc: "down"},
					{Key: "k/\u2191", Desc: "up"},
					{Key: "g", Desc: "top"},
					{Key: "G", Desc: "bottom"},
					{Key: "h/\u2190", Desc: "scroll left"},
					{Key: "l/\u2192", Desc: "scroll right"},
					{Key: "Enter", Desc: "select"},
					{Key: "Esc", Desc: "back"},
				},
			},
			{
				Title: "Actions",
				Bindings: []helpBinding{
					{Key: "d", Desc: "describe"},
					{Key: "y", Desc: "YAML view"},
					{Key: "x", Desc: "reveal secret"},
					{Key: "c", Desc: "copy ID"},
					{Key: "/", Desc: "filter"},
					{Key: ":", Desc: "command"},
					{Key: "?", Desc: "help"},
					{Key: "Ctrl-R", Desc: "refresh"},
				},
			},
			{
				Title: "Sorting",
				Bindings: []helpBinding{
					{Key: "N", Desc: "by name"},
					{Key: "S", Desc: "by status"},
					{Key: "A", Desc: "by age"},
				},
			},
		}

	case DetailViewHelp:
		return []helpSection{
			{
				Title: "Navigation",
				Bindings: []helpBinding{
					{Key: "j/\u2193", Desc: "down"},
					{Key: "k/\u2191", Desc: "up"},
					{Key: "g", Desc: "top"},
					{Key: "G", Desc: "bottom"},
					{Key: "h/\u2190", Desc: "scroll left"},
					{Key: "l/\u2192", Desc: "scroll right"},
					{Key: "Esc", Desc: "back"},
				},
			},
			{
				Title: "Actions",
				Bindings: []helpBinding{
					{Key: "w", Desc: "toggle wrap"},
					{Key: "c", Desc: "copy detail"},
					{Key: "?", Desc: "help"},
				},
			},
		}

	case JSONViewHelp:
		return []helpSection{
			{
				Title: "Navigation",
				Bindings: []helpBinding{
					{Key: "j/\u2193", Desc: "down"},
					{Key: "k/\u2191", Desc: "up"},
					{Key: "g", Desc: "top"},
					{Key: "G", Desc: "bottom"},
					{Key: "h/\u2190", Desc: "scroll left"},
					{Key: "l/\u2192", Desc: "scroll right"},
					{Key: "Esc", Desc: "back"},
				},
			},
			{
				Title: "Actions",
				Bindings: []helpBinding{
					{Key: "c", Desc: "copy content"},
					{Key: "?", Desc: "help"},
				},
			},
		}

	default:
		// GlobalHelp - show everything
		return []helpSection{
			{
				Title: "Global",
				Bindings: []helpBinding{
					{Key: ":", Desc: "command"},
					{Key: "/", Desc: "filter"},
					{Key: "?", Desc: "help"},
					{Key: "Esc", Desc: "back"},
					{Key: "[", Desc: "history back"},
					{Key: "]", Desc: "history forward"},
					{Key: "Ctrl-R", Desc: "refresh"},
					{Key: "Ctrl-C", Desc: "exit"},
				},
			},
			{
				Title: "Navigation",
				Bindings: []helpBinding{
					{Key: "j/\u2193", Desc: "down"},
					{Key: "k/\u2191", Desc: "up"},
					{Key: "g", Desc: "top"},
					{Key: "G", Desc: "bottom"},
					{Key: "Enter", Desc: "select"},
				},
			},
			{
				Title: "Actions",
				Bindings: []helpBinding{
					{Key: "d", Desc: "describe"},
					{Key: "y", Desc: "YAML view"},
					{Key: "x", Desc: "reveal secret"},
					{Key: "c", Desc: "copy"},
				},
			},
			{
				Title: "Sorting",
				Bindings: []helpBinding{
					{Key: "N", Desc: "by name"},
					{Key: "S", Desc: "by status"},
					{Key: "A", Desc: "by age"},
				},
			},
		}
	}
}
