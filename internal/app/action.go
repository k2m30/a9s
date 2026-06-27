// Package app is the headless controller layer that sits between the
// platform-agnostic runtime core (internal/runtime) and any renderer
// (TUI, web, test). It owns the screen stack, per-screen view state,
// the semantic Action vocabulary, and the serialisable ViewState snapshot.
//
// PR-A creates the contract types and a skeleton Controller. No behavior
// is moved from internal/tui in this PR; that happens in PR-B (event
// migration) and PR-C (state lift).
package app

// ActionKind is the semantic verb that a renderer translates its native
// input (keystroke, HTTP POST, test call) into before handing it to the
// controller. Using a named string makes the set greppable and
// round-trips cleanly through JSON without an iota → string map.
type ActionKind string

const (
	ActionMoveUp     ActionKind = "move-up"
	ActionMoveDown   ActionKind = "move-down"
	ActionMoveTop    ActionKind = "move-top"
	ActionMoveBottom ActionKind = "move-bottom"
	ActionPageUp     ActionKind = "page-up"
	ActionPageDown   ActionKind = "page-down"

	ActionSelect ActionKind = "select"
	ActionBack   ActionKind = "back"

	ActionOpenDetail   ActionKind = "open-detail"
	ActionOpenYAML     ActionKind = "open-yaml"
	ActionOpenJSON     ActionKind = "open-json"
	ActionOpenHelp     ActionKind = "open-help"
	ActionOpenIdentity ActionKind = "open-identity"

	ActionReveal ActionKind = "reveal"

	// ActionSetFilter carries the filter string in Arg.
	ActionSetFilter ActionKind = "set-filter"

	// ActionSort carries the column key in Arg.
	ActionSort ActionKind = "sort"

	// ActionSearch carries the query string in Arg.
	ActionSearch      ActionKind = "search"
	ActionSearchNext  ActionKind = "search-next"
	ActionSearchPrev  ActionKind = "search-prev"
	ActionSearchClear ActionKind = "search-clear"

	ActionCopy ActionKind = "copy"

	ActionToggleRelated   ActionKind = "toggle-related"
	ActionToggleWrap      ActionKind = "toggle-wrap"
	ActionToggleAttention ActionKind = "toggle-attention"

	// ActionChildView carries the trigger key in Arg (e, L, r, s, Enter, t).
	ActionChildView ActionKind = "child-view"

	ActionLoadMore ActionKind = "load-more"
	ActionRefresh  ActionKind = "refresh"

	// ActionCommand carries the resource short-name in Arg (from the -c flag path).
	ActionCommand ActionKind = "command"

	// ActionSelectProfile carries the profile name in Arg.
	ActionSelectProfile ActionKind = "select-profile"
	// ActionSelectRegion carries the region name in Arg.
	ActionSelectRegion ActionKind = "select-region"
	// ActionSelectTheme carries the theme name in Arg.
	ActionSelectTheme ActionKind = "select-theme"

	ActionQuit ActionKind = "quit"
)

// Action is a single semantic input from a renderer to the controller.
// Arg carries the string parameter for parameterised actions (SetFilter,
// Sort, Search, ChildView, Command, SelectProfile, SelectRegion); it is
// empty for zero-arity actions.
type Action struct {
	Kind ActionKind `json:"kind"`
	Arg  string     `json:"arg,omitempty"`
	// N carries a numeric parameter. Currently the renderer's page size for
	// PageUp/PageDown, so page movement tracks the live viewport height instead
	// of a constant. Zero means "use the controller default".
	N int `json:"n,omitempty"`
}
