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
	Identity  key.Binding

	// Input modes
	Colon  key.Binding
	Filter key.Binding
	Tab    key.Binding

	// Resource list actions
	Describe    key.Binding
	YAML        key.Binding
	JSON        key.Binding
	Reveal      key.Binding
	Copy        key.Binding
	ScrollLeft  key.Binding
	ScrollRight key.Binding

	// Child-view triggers
	Events    key.Binding
	Logs      key.Binding
	Resources key.Binding
	Source    key.Binding

	// CloudTrail shortcut
	CloudTrail key.Binding

	// Sort
	SortByCol [10]key.Binding // 1-9,0 → sort by column position

	// Pagination
	PageUp   key.Binding
	PageDown key.Binding

	// Paginated fetch
	LoadMore key.Binding

	// Detail / YAML
	ToggleWrap key.Binding

	// Resource list attention filter
	ToggleAttentionOnly key.Binding

	// Related views
	ToggleRelated key.Binding

	// Search (detail/YAML views)
	Search     key.Binding
	SearchNext key.Binding
	SearchPrev key.Binding

	// Error log
	ErrorLog key.Binding
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
		Refresh:   key.NewBinding(key.WithKeys("ctrl+r", "\x12"), key.WithHelp("ctrl+r", "refresh")),
		Identity:  key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "identity")),

		Colon:  key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command")),
		Filter: key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Tab:    key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "complete")),

		Describe:    key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "detail")),
		YAML:        key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "yaml")),
		JSON:        key.NewBinding(key.WithKeys("J"), key.WithHelp("J", "json")),
		Reveal:      key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "reveal")),
		Copy:        key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy id")),
		ScrollLeft:  key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h/←", "scroll left")),
		ScrollRight: key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l/→", "scroll right")),

		Events:    key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "events")),
		Logs:      key.NewBinding(key.WithKeys("L"), key.WithHelp("L", "logs")),
		Resources: key.NewBinding(key.WithKeys("r", "R"), key.WithHelp("r/R", "resources")),
		Source:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "source")),

		CloudTrail: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "cloudtrail")),

		SortByCol: [10]key.Binding{
			key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "sort col 1")),
			key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "sort col 2")),
			key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "sort col 3")),
			key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "sort col 4")),
			key.NewBinding(key.WithKeys("5"), key.WithHelp("5", "sort col 5")),
			key.NewBinding(key.WithKeys("6"), key.WithHelp("6", "sort col 6")),
			key.NewBinding(key.WithKeys("7"), key.WithHelp("7", "sort col 7")),
			key.NewBinding(key.WithKeys("8"), key.WithHelp("8", "sort col 8")),
			key.NewBinding(key.WithKeys("9"), key.WithHelp("9", "sort col 9")),
			key.NewBinding(key.WithKeys("0"), key.WithHelp("0", "sort col 10")),
		},

		PageUp:   key.NewBinding(key.WithKeys("pgup", "ctrl+u", "\x15"), key.WithHelp("pgup", "page up")),
		PageDown: key.NewBinding(key.WithKeys("pgdown", "ctrl+d", "\x04"), key.WithHelp("pgdn", "page down")),

		LoadMore: key.NewBinding(key.WithKeys("m", "M"), key.WithHelp("m", "load more")),

		ToggleWrap: key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "toggle wrap")),

		ToggleAttentionOnly: key.NewBinding(key.WithKeys("ctrl+z"), key.WithHelp("ctrl+z", "show only attention-worthy rows")),

		ToggleRelated: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "related")),

		Search:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		SearchNext: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next match")),
		SearchPrev: key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "prev match")),

		ErrorLog: key.NewBinding(key.WithKeys("!"), key.WithHelp("!", "error log")),
	}
}
