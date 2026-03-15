package app

import "charm.land/bubbles/v2/key"

// KeyMap defines all keybindings for the application.
type KeyMap struct {
	// Navigation
	Up     key.Binding
	Down   key.Binding
	Top    key.Binding
	Bottom key.Binding
	Enter  key.Binding
	Escape key.Binding

	// Commands
	Colon  key.Binding
	Filter key.Binding
	Help   key.Binding

	// Actions
	Describe key.Binding
	JSON     key.Binding
	Reveal   key.Binding
	Copy     key.Binding

	// History
	HistoryBack    key.Binding
	HistoryForward key.Binding

	// Sorting
	SortByName   key.Binding
	SortByStatus key.Binding
	SortByAge    key.Binding

	// System
	Refresh key.Binding
	Quit    key.Binding
	ForceQuit key.Binding
}

// DefaultKeyMap returns the default set of keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("↓/j", "move down"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "go to top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "go to bottom"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),

		Colon: key.NewBinding(
			key.WithKeys(":"),
			key.WithHelp(":", "command"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),

		Describe: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "describe"),
		),
		JSON: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "json"),
		),
		Reveal: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "reveal"),
		),
		Copy: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "copy"),
		),

		HistoryBack: key.NewBinding(
			key.WithKeys("["),
			key.WithHelp("[", "history back"),
		),
		HistoryForward: key.NewBinding(
			key.WithKeys("]"),
			key.WithHelp("]", "history forward"),
		),

		SortByName: key.NewBinding(
			key.WithKeys("N"),
			key.WithHelp("N", "sort by name"),
		),
		SortByStatus: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "sort by status"),
		),
		SortByAge: key.NewBinding(
			key.WithKeys("A"),
			key.WithHelp("A", "sort by age"),
		),

		Refresh: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "refresh"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		ForceQuit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "force quit"),
		),
	}
}
