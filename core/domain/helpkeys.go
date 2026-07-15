// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package domain

// HelpContext identifies which screen opened the help overlay so the
// keybinding table can be filtered to the bindings that screen actually
// honors. Mirrors internal/tui/views.HelpContext one-for-one; this is the
// single source of truth for the key/description content, while the TUI
// HelpContext values remain the renderer-side selector the TUI's key-press
// handler already computes (helpContext() in internal/tui/app_stack.go).
type HelpContext int

const (
	HelpFromMainMenu HelpContext = iota
	HelpFromResourceList
	HelpFromSecretsList
	HelpFromDetail
	HelpFromYAML
	HelpFromJSON
	HelpFromSelector
	HelpFromReveal
	HelpFromResourceListPaginated
	HelpFromSecretsListPaginated
	HelpFromCosts
)

// HelpHint is one key/description pair.
type HelpHint struct {
	Key  string
	Help string
}

// HelpSection is one titled column of key hints.
type HelpSection struct {
	Title string
	Hints []HelpHint
}

// commandsSection is appended to every context — the colon-command
// reference is identical regardless of which screen opened help.
func commandsSection() HelpSection {
	return HelpSection{
		Title: "COMMANDS",
		Hints: []HelpHint{
			{":q", "exit"},
			{":ctx", "switch profile"},
			{":profile", "switch profile"},
			{":region", "switch region"},
			{":theme", "switch theme"},
			{":help", "show help"},
			{":root", "main menu"},
			{":main", "main menu"},
			{":<res>", "e.g. :ec2 :s3 :lambda"},
		},
	}
}

// HelpGroupsFor returns the titled key-hint sections for the given context,
// terminated with the shared COMMANDS section. toggleAttentionKey is the
// live key.Binding string for keys.Map.ToggleAttentionOnly — the one
// context-independent binding whose key text is configurable rather than a
// literal, so callers (both the TUI and the controller's buildHelpBody) must
// supply it rather than this table hardcoding a key.
//
// This is the single source of the help-overlay key/description content: the
// TUI's internal/tui/views/help.go renders these sections directly, and
// core/app/snapshot.go's buildHelpBody consumes the same table for the
// web renderer's HelpBody. Do not re-list these bindings anywhere else.
func HelpGroupsFor(ctx HelpContext, toggleAttentionKey string) []HelpSection {
	switch ctx {
	case HelpFromResourceList:
		return resourceListSections(false, false, toggleAttentionKey)
	case HelpFromSecretsList:
		return resourceListSections(true, false, toggleAttentionKey)
	case HelpFromResourceListPaginated:
		return resourceListSections(false, true, toggleAttentionKey)
	case HelpFromSecretsListPaginated:
		return resourceListSections(true, true, toggleAttentionKey)
	case HelpFromDetail:
		return detailSections()
	case HelpFromYAML:
		return yamlSections()
	case HelpFromJSON:
		return jsonSections()
	case HelpFromSelector:
		return selectorSections()
	case HelpFromReveal:
		return revealSections()
	case HelpFromCosts:
		return costsSections()
	default:
		return mainMenuSections()
	}
}

func mainMenuSections() []HelpSection {
	return []HelpSection{
		{
			Title: "NAVIGATION",
			Hints: []HelpHint{
				{"j/k", "up/down"},
				{"g", "top"},
				{"G", "bottom"},
				{"pgup", "page up"},
				{"pgdn", "page down"},
			},
		},
		{
			Title: "ACTIONS",
			Hints: []HelpHint{
				{"enter", "select"},
				{"/", "filter"},
				{":", "command"},
				{"q", "quit"},
				{"ctrl+c", "force quit"},
			},
		},
		{
			Title: "OTHER",
			Hints: []HelpHint{
				{"i", "identity"},
				{"!", "error log"},
				{"?", "help"},
				{"esc", "back"},
			},
		},
		commandsSection(),
	}
}

func resourceListSections(secrets, paginated bool, toggleAttentionKey string) []HelpSection {
	nav := HelpSection{
		Title: "NAVIGATION",
		Hints: []HelpHint{
			{"j/k", "up/down"},
			{"g/G", "top/bottom"},
			{"pgup", "page up"},
			{"pgdn", "page down"},
			{"h/l", "scroll cols"},
		},
	}

	actions := HelpSection{
		Title: "ACTIONS",
		Hints: []HelpHint{
			{"enter/d", "detail"},
			{"y", "yaml"},
			{"J", "json"},
			{"t", "cloudtrail events"},
			{"c", "copy id"},
			{"/", "filter"},
			{":", "command"},
		},
	}
	if paginated {
		actions.Hints = append(actions.Hints, HelpHint{"M", "load more"})
	}
	if secrets {
		actions.Hints = append(actions.Hints, HelpHint{"x", "reveal"})
	}

	sortSection := HelpSection{
		Title: "SORT",
		Hints: []HelpHint{
			{"1-0", "sort col 1-10"},
		},
	}

	filter := HelpSection{
		Title: "FILTER",
		Hints: []HelpHint{
			{toggleAttentionKey, "Toggle attention filter (hide healthy/dim rows)"},
		},
	}

	other := HelpSection{
		Title: "OTHER",
		Hints: []HelpHint{
			{"ctrl+r", "refresh"},
			{"esc", "back"},
			{"q", "quit"},
			{"i", "identity"},
			{"!", "error log"},
			{"?", "help"},
		},
	}

	return append([]HelpSection{nav, actions, sortSection, filter, other}, commandsSection())
}

func detailSections() []HelpSection {
	return []HelpSection{
		{
			Title: "SCROLL",
			Hints: []HelpHint{
				{"j/k", "up/down"},
				{"g", "top"},
				{"G", "bottom"},
			},
		},
		{
			Title: "ACTIONS",
			Hints: []HelpHint{
				{"y", "yaml"},
				{"J", "json"},
				{"t", "cloudtrail events"},
				{"c", "copy value"},
				{"w", "wrap toggle"},
				{"r", "related"},
				{"tab", "focus switch"},
				{"h/l", "focus cols"},
			},
		},
		{
			Title: "SEARCH",
			Hints: []HelpHint{
				{"/", "search"},
				{"n", "next match"},
				{"N", "prev match"},
			},
		},
		{
			Title: "RELATED",
			Hints: []HelpHint{
				{"/", "filter list"},
				{"c", "copy type"},
				{"tab", "focus switch"},
				{"r", "related"},
				{"esc", "unfocus"},
			},
		},
		{
			Title: "OTHER",
			Hints: []HelpHint{
				{"esc", "back"},
				{"i", "identity"},
				{"!", "error log"},
				{"?", "help"},
			},
		},
		commandsSection(),
	}
}

func yamlSections() []HelpSection {
	return []HelpSection{
		{
			Title: "SCROLL",
			Hints: []HelpHint{
				{"j/k", "up/down"},
				{"g", "top"},
				{"G", "bottom"},
			},
		},
		{
			Title: "ACTIONS",
			Hints: []HelpHint{
				{"c", "copy yaml"},
				{"t", "cloudtrail events"},
				{"w", "wrap toggle"},
			},
		},
		{
			Title: "SEARCH",
			Hints: []HelpHint{
				{"/", "search"},
				{"n", "next match"},
				{"N", "prev match"},
			},
		},
		{
			Title: "OTHER",
			Hints: []HelpHint{
				{"esc", "back"},
				{"i", "identity"},
				{"!", "error log"},
				{"?", "help"},
			},
		},
		commandsSection(),
	}
}

func jsonSections() []HelpSection {
	return []HelpSection{
		{
			Title: "SCROLL",
			Hints: []HelpHint{
				{"j/k", "up/down"},
				{"g", "top"},
				{"G", "bottom"},
			},
		},
		{
			Title: "ACTIONS",
			Hints: []HelpHint{
				{"c", "copy json"},
				{"t", "cloudtrail events"},
				{"w", "wrap toggle"},
			},
		},
		{
			Title: "SEARCH",
			Hints: []HelpHint{
				{"/", "search"},
				{"n", "next match"},
				{"N", "prev match"},
			},
		},
		{
			Title: "OTHER",
			Hints: []HelpHint{
				{"esc", "back"},
				{"i", "identity"},
				{"!", "error log"},
				{"?", "help"},
			},
		},
		commandsSection(),
	}
}

func selectorSections() []HelpSection {
	return []HelpSection{
		{
			Title: "NAVIGATION",
			Hints: []HelpHint{
				{"j/k", "up/down"},
				{"g", "top"},
				{"G", "bottom"},
				{"/", "filter"},
			},
		},
		{
			Title: "ACTIONS",
			Hints: []HelpHint{
				{"enter", "select"},
				{"esc", "cancel"},
			},
		},
		{
			Title: "OTHER",
			Hints: []HelpHint{
				{"i", "identity"},
				{"!", "error log"},
				{"?", "help"},
			},
		},
		commandsSection(),
	}
}

func costsSections() []HelpSection {
	return []HelpSection{
		{
			Title: "COST EXPLORER",
			Hints: []HelpHint{
				{"b", "metric"},
				{"+/-", "zoom"},
				{"0-9", "pivot"},
				{"enter", "drill"},
				{"h/l", "scroll cols"},
				{"j/k", "move row"},
				{"sort", "rows sort by total spend across the visible window, largest absolute first"},
			},
		},
		{
			Title: "OTHER",
			Hints: []HelpHint{
				{"ctrl+r", "refresh"},
				{"esc", "back"},
				{"i", "identity"},
				{"!", "error log"},
				{"?", "help"},
			},
		},
		commandsSection(),
	}
}

func revealSections() []HelpSection {
	return []HelpSection{
		{
			Title: "ACTIONS",
			Hints: []HelpHint{
				{"c", "copy value"},
				{"w", "wrap toggle"},
				{"esc", "close"},
			},
		},
		{
			Title: "OTHER",
			Hints: []HelpHint{
				{"i", "identity"},
				{"!", "error log"},
				{"?", "help"},
			},
		},
		commandsSection(),
	}
}
