package app

import "github.com/k2m30/a9s/v3/internal/runtime"

// Screen is one entry on the controller's view stack. It pairs the
// runtime-issued screen identity and context with the per-screen view
// state owned by the controller (not by a Bubble Tea model).
type Screen struct {
	ID    runtime.ScreenID      `json:"id"`
	Ctx   runtime.ScreenContext `json:"ctx"`
	State ScreenState           `json:"state"`
}

// ScreenState is the per-screen view state union. Exactly one of the
// pointer fields is non-nil, determined by the screen kind.
type ScreenState struct {
	List   *ListState   `json:"list,omitempty"`
	Detail *DetailState `json:"detail,omitempty"`
	Text   *TextState   `json:"text,omitempty"`
	Menu   *MenuState   `json:"menu,omitempty"`
}

// ListState holds the mutable display state for a resource-list screen.
type ListState struct {
	Filter          string `json:"filter,omitempty"`
	SortCol         string `json:"sort_col,omitempty"`
	SortDir         string `json:"sort_dir,omitempty"` // "asc" | "desc"
	SelectedRow     int    `json:"selected_row"`
	ScrollX         int    `json:"scroll_x"`
	ScrollY         int    `json:"scroll_y"`
	AttentionOnly   bool   `json:"attention_only,omitempty"`
	PaginationCursor string `json:"pagination_cursor,omitempty"`
}

// DetailState holds the mutable display state for a resource-detail screen.
type DetailState struct {
	SearchQuery  string `json:"search_query,omitempty"`
	SearchCursor int    `json:"search_cursor"`
	Wrap         bool   `json:"wrap,omitempty"`
	RelatedFocus bool   `json:"related_focus,omitempty"`
	ScrollY      int    `json:"scroll_y"`
}

// TextState holds the mutable display state for a YAML/JSON text screen.
type TextState struct {
	Search  string `json:"search,omitempty"`
	Wrap    bool   `json:"wrap,omitempty"`
	ScrollY int    `json:"scroll_y"`
}

// MenuState holds the mutable display state for the main-menu screen.
// Maps the CONTROLLER bucket from docs/web-ui-state-inventory.md §MainMenuModel.
type MenuState struct {
	Filter         string          `json:"filter,omitempty"`
	Cursor         int             `json:"cursor"`
	ScrollOffset   int             `json:"scroll_offset"`
	AttentionOnly  bool            `json:"attention_only,omitempty"`
	Availability   map[string]int  `json:"availability,omitempty"`
	Truncated      map[string]bool `json:"truncated,omitempty"`
	IssueCounts    map[string]int  `json:"issue_counts,omitempty"`
	IssueKnown     map[string]bool `json:"issue_known,omitempty"`
	IssueTruncated map[string]bool `json:"issue_truncated,omitempty"`

	// Progress fields for FrameTitle indicator (DERIVED at Snapshot, stored here
	// so intents can update them without re-computing from task state).
	AvailChecked  int `json:"avail_checked,omitempty"`
	AvailTotal    int `json:"avail_total,omitempty"`
	EnrichChecked int `json:"enrich_checked,omitempty"`
	EnrichTotal   int `json:"enrich_total,omitempty"`
}
