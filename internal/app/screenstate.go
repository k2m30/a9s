package app

import (
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

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
	List     *ListState     `json:"list,omitempty"`
	Detail   *DetailState   `json:"detail,omitempty"`
	Text     *TextState     `json:"text,omitempty"`
	Menu     *MenuState     `json:"menu,omitempty"`
	Selector *SelectorState `json:"selector,omitempty"`
}

// ListState holds the mutable display state for a resource-list screen.
type ListState struct {
	// Rows holds the fetched resource page for THIS screen. Storing rows here
	// rather than in a Controller-level type-keyed map prevents two stacked
	// list screens of the same resource type from corrupting each other's data.
	Rows []resource.Resource `json:"rows,omitempty"`

	Filter           string `json:"filter,omitempty"`
	SortCol          string `json:"sort_col,omitempty"`
	SortDir          string `json:"sort_dir,omitempty"` // "asc" | "desc"
	SelectedRow      int    `json:"selected_row"`
	ScrollX          int    `json:"scroll_x"`
	ScrollY          int    `json:"scroll_y"`
	AttentionOnly    bool   `json:"attention_only,omitempty"`
	PaginationCursor string `json:"pagination_cursor,omitempty"`

	// Inventory fields from docs/web-ui-state-inventory.md §ResourceListModel.
	HasPagination    bool                `json:"has_pagination,omitempty"`
	AutoOpenSingle   bool                `json:"auto_open_single,omitempty"`
	RelatedIDSet     map[string]struct{} `json:"related_id_set,omitempty"`
	FetchFilter      map[string]string   `json:"fetch_filter,omitempty"`
	ParentContext    map[string]string   `json:"parent_context,omitempty"`
	DisplayName      string              `json:"display_name,omitempty"`
	TitleSuffix      string              `json:"title_suffix,omitempty"`
	EscPops          bool                `json:"esc_pops,omitempty"`
	ShowIssueBadge   bool                `json:"show_issue_badge,omitempty"`

	// Loading tracks whether the initial fetch is still in flight.
	Loading bool `json:"loading,omitempty"`
	// LoadingMore tracks whether an m-key load-more fetch is in flight.
	LoadingMore bool `json:"loading_more,omitempty"`
}

// DetailState holds the mutable display state for a resource-detail screen.
// Controller-owned fields (per docs/web-ui-state-inventory.md §DetailModel).
type DetailState struct {
	// Display-interaction state
	SearchQuery  string `json:"search_query,omitempty"`
	SearchCursor int    `json:"search_cursor"`
	Wrap         bool   `json:"wrap,omitempty"`
	ScrollY      int    `json:"scroll_y"`
	FieldCursor  int    `json:"field_cursor"`

	// Related panel state
	RelatedVisible bool   `json:"related_visible,omitempty"`
	RelatedFocus   bool   `json:"related_focus,omitempty"`
	RelatedCursor  int    `json:"related_cursor"`
	RelatedScroll  int    `json:"related_scroll"`
	RelatedFilter  string `json:"related_filter,omitempty"`
	RelatedFilterActive bool `json:"related_filter_active,omitempty"`

	// Per-screen data: set once at push via EnsureDetailState, updated by enrichment.
	Resource resource.Resource `json:"resource,omitzero"`
	// ResourceType is the canonical short name (e.g. "ec2", "rds").
	ResourceType string `json:"resource_type,omitempty"`
	// RelatedRows holds the resolved related-panel rows (populated by ApplyDetailRelated).
	RelatedRows []DetailRelatedRow `json:"related_rows,omitempty"`
	// Findings holds wave-2 enrichment findings for this resource (set by ApplyDetailFinding).
	Findings []domain.Finding `json:"findings,omitempty"`
	// AttentionDetails holds per-finding detail rows (set by ApplyDetailFinding).
	AttentionDetails map[domain.FindingCode]domain.AttentionDetail `json:"attention_details,omitempty"`
}

// DetailRelatedRow is one row in the detail screen's related panel, mirroring
// rightColumnRow but as a serialisable value type (no funcs, no checker).
type DetailRelatedRow struct {
	TargetType  string            `json:"target_type"`
	DisplayName string            `json:"display_name"`
	Count       int               `json:"count"`      // -1 = loading
	Loading     bool              `json:"loading,omitempty"`
	Err         string            `json:"err,omitempty"`
	Approximate bool              `json:"approximate,omitempty"`
	ResourceIDs []string          `json:"resource_ids,omitempty"`
	FetchFilter map[string]string `json:"fetch_filter,omitempty"`
}

// TextState holds the mutable display state for a YAML/JSON text screen.
// Lines is the syntax-colored content set once at push time (set by
// EnsureTextState) and never mutated; all other fields are updated by Apply.
type TextState struct {
	Lines        []string `json:"lines,omitempty"`
	Search       string   `json:"search,omitempty"`
	SearchCursor int      `json:"search_cursor"`
	Wrap         bool     `json:"wrap,omitempty"`
	ScrollY      int      `json:"scroll_y"`
}

// SelectorState holds the mutable display state for a profile/region/theme
// selector screen. Items, ActiveItem, and Title are set once at push time and
// never mutate; Filter and Cursor are updated by Apply actions.
type SelectorState struct {
	Items      []string `json:"items,omitempty"`
	ActiveItem string   `json:"active_item,omitempty"`
	Title      string   `json:"title,omitempty"`
	Filter     string   `json:"filter,omitempty"`
	Cursor     int      `json:"cursor"`
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
