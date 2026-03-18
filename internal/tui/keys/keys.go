package keys

import "charm.land/bubbles/v2/key"

// Map holds all application key bindings. Single source of truth.
type Map struct {
	// Navigation
	Up     key.Binding
	Down   key.Binding
	Top    key.Binding
	Bottom key.Binding
	Enter  key.Binding
	Escape key.Binding

	// App
	Quit      key.Binding
	ForceQuit key.Binding
	Help      key.Binding
	Refresh   key.Binding

	// Input modes
	Colon  key.Binding
	Filter key.Binding
	Tab    key.Binding

	// Resource list actions
	Describe    key.Binding
	YAML        key.Binding
	Reveal      key.Binding
	Copy        key.Binding
	ScrollLeft  key.Binding
	ScrollRight key.Binding

	// Sort
	SortByName   key.Binding
	SortByStatus key.Binding
	SortByAge    key.Binding

	// Pagination
	PageUp   key.Binding
	PageDown key.Binding

	// Detail / YAML
	ToggleWrap key.Binding
}

// Default returns the canonical key bindings.
func Default() Map {
	return Map{
		Up:     key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("↑/k", "up")),
		Down:   key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("↓/j", "down")),
		Top:    key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
		Bottom: key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Escape: key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),

		Quit:      key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
		ForceQuit: key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "force quit")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Refresh:   key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "refresh")),

		Colon:  key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command")),
		Filter: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Tab:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "complete")),

		Describe:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "detail")),
		YAML:        key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yaml")),
		Reveal:      key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "reveal")),
		Copy:        key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy id")),
		ScrollLeft:  key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h/←", "scroll left")),
		ScrollRight: key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/→", "scroll right")),

		SortByName:   key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "sort name")),
		SortByStatus: key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "sort status")),
		SortByAge:    key.NewBinding(key.WithKeys("A"), key.WithHelp("A", "sort age")),

		PageUp:   key.NewBinding(key.WithKeys("pgup", "ctrl+u"), key.WithHelp("pgup", "page up")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", "ctrl+d"), key.WithHelp("pgdn", "page down")),

		ToggleWrap: key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "toggle wrap")),
	}
}
