package app

import (
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
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
	Costs    *CostsState    `json:"costs,omitempty"`
}

// ListState holds the mutable display state for a resource-list screen.
type ListState struct {
	// Rows holds the fetched resource page for THIS screen. Storing rows here
	// rather than in a Controller-level type-keyed map prevents two stacked
	// list screens of the same resource type from corrupting each other's data.
	Rows []resource.Resource `json:"rows,omitempty"`
	// RowsGen pins the RowStore generation this screen's Rows were adopted
	// from, for the canonical top-level list only (task #17 wave 1 stage 4).
	// Zero for a screen that has never routed through Core.ObserveRows (a
	// freshly-pushed screen, or a non-canonical child/filtered list, whose
	// Rows are written locally without ever touching RowStore). Carries no
	// behavior today — reserved for a future conformance check that a
	// canonical screen's Rows never regress behind a fresher store
	// generation without an explicit re-adopt.
	RowsGen domain.Gen `json:"rows_gen,omitempty"`

	Filter           string `json:"filter,omitempty"`
	SortCol          string `json:"sort_col,omitempty"`
	SortDir          string `json:"sort_dir,omitempty"` // "asc" | "desc"
	SelectedRow      int    `json:"selected_row"`
	ScrollX          int    `json:"scroll_x"`
	ScrollY          int    `json:"scroll_y"`
	AttentionOnly    bool   `json:"attention_only,omitempty"`
	PaginationCursor string `json:"pagination_cursor,omitempty"`

	// Inventory fields from docs/historical/analysis/web-ui-state-inventory.md §ResourceListModel.
	HasPagination  bool                `json:"has_pagination,omitempty"`
	AutoOpenSingle bool                `json:"auto_open_single,omitempty"`
	RelatedIDSet   map[string]struct{} `json:"related_id_set,omitempty"`

	// reapplyChecker + reapplySource belong to THIS related-list screen: for a
	// truncated reverse-scan pivot, each loaded page is re-run through the checker
	// to extend RelatedIDSet with newly matched IDs. Held per-screen (not in a
	// controller type-keyed map) so popping the related list drops the checker —
	// a later normal list of the same type can never inherit it. In-memory only
	// (a func is not serialisable); the web renderer runs the checker controller-
	// side and never needs it in the JSON snapshot.
	reapplyChecker resource.RelatedChecker
	reapplySource  resource.Resource
	FetchFilter    map[string]string `json:"fetch_filter,omitempty"`
	ParentContext  map[string]string `json:"parent_context,omitempty"`
	DisplayName    string            `json:"display_name,omitempty"`
	TitleSuffix    string            `json:"title_suffix,omitempty"`
	EscPops        bool              `json:"esc_pops,omitempty"`

	// Loading tracks whether the initial fetch is still in flight.
	Loading bool `json:"loading,omitempty"`
	// LoadingMore tracks whether an m-key load-more fetch is in flight.
	LoadingMore bool `json:"loading_more,omitempty"`
	// Refreshing is true when the screen opened with rows seeded from a
	// cache-first source (session ProbeResources, a previous visit's
	// ResourceCache, or the on-disk availability cache) while a fresh fetch
	// is still in flight to confirm/replace them. Cleared once the fetch's
	// ResourcesLoaded result lands. Distinct from Loading, which gates the
	// no-rows-known spinner path.
	Refreshing bool `json:"refreshing,omitempty"`
	// LastFetchError is the error marker text for the most recent failed
	// fetch over this screen, or "" when no error is outstanding (DEF-5/C4).
	// Set by a messages.APIError landing while this screen is active; cleared
	// on the next successful ResourcesLoaded for this screen.
	LastFetchError string `json:"last_fetch_error,omitempty"`
	// TotalCount is the authoritative total for a seeded-but-unverified list
	// (item B, #17 wave 2, DEF-21), set by cache-first seeding callers AFTER
	// applyResourcesLoaded — mirroring Refreshing's set-after-seed ordering —
	// when the seed source's known total exceeds len(Rows) (the C6a
	// reconstructable disk pair: Count may outrun the last-known Rows).
	// buildListFrameTitle prefers this over len(Rows) while it is set. Zero
	// means "not applicable" — every other seed source already has
	// len(Rows) == the authoritative total. Cleared by the next genuine
	// fetch-result landing in applyResourcesLoaded, same trigger as Refreshing.
	TotalCount int `json:"total_count,omitempty"`
}

// DetailState holds the mutable display state for a resource-detail screen.
// Controller-owned fields (per docs/historical/analysis/web-ui-state-inventory.md §DetailModel).
type DetailState struct {
	// Display-interaction state
	SearchQuery  string `json:"search_query,omitempty"`
	SearchCursor int    `json:"search_cursor"`
	Wrap         bool   `json:"wrap,omitempty"`
	ScrollY      int    `json:"scroll_y"`
	FieldCursor  int    `json:"field_cursor"`
	// ViewportHeight is the renderer-supplied usable field-viewport clip
	// height, set once per render via Controller.SetDetailViewportHeight so
	// move actions (ActionMoveUp/Down/Bottom) can reconcile ScrollY without
	// every call site needing to pass it through Action.N.
	ViewportHeight int `json:"viewport_height,omitempty"`

	// Related panel state
	RelatedVisible bool `json:"related_visible,omitempty"`
	// RelatedHidden is set when the user explicitly hides the panel (overrides
	// auto-show logic). Distinct from RelatedVisible so that the default
	// "show when defs exist" behaviour is preserved until the user acts.
	RelatedHidden bool `json:"related_hidden,omitempty"`
	// RelatedUserVisible is true only when the user explicitly toggled the panel
	// ON (mirrors rightColVisible in the TUI). Auto-show (initDetailRelatedRows)
	// does NOT set this flag. Used by buildDetailFooterHints to gate the
	// "tab: Cols" hint, which matches DetailModel.BottomHints checking m.rightColVisible.
	RelatedUserVisible  bool   `json:"related_user_visible,omitempty"`
	RelatedFocus        bool   `json:"related_focus,omitempty"`
	RelatedCursor       int    `json:"related_cursor"`
	RelatedScroll       int    `json:"related_scroll"`
	RelatedFilter       string `json:"related_filter,omitempty"`
	RelatedFilterActive bool   `json:"related_filter_active,omitempty"`

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
	TargetType  string `json:"target_type"`
	DisplayName string `json:"display_name"`
	// State classifies how Count should be interpreted; see domain.RelatedRowState.
	State       domain.RelatedRowState `json:"state,omitempty"`
	Count       int                    `json:"count"` // authoritative only when State == RelatedResolved
	Loading     bool                   `json:"loading,omitempty"`
	Err         string                 `json:"err,omitempty"`
	Truncated   bool                   `json:"truncated,omitempty"`
	ResourceIDs []string               `json:"resource_ids,omitempty"`
	FetchFilter map[string]string      `json:"fetch_filter,omitempty"`
}

// TextState holds the mutable display state for a YAML/JSON text screen.
// Lines is the syntax-colored content set once at push time (set by
// EnsureTextState) and never mutated; all other fields are updated by Apply.
// Resource is the resource this text screen was rendered from — set at push
// time and replaced by SetTextResource when async detail enrichment lands,
// mirroring DetailState.Resource. GetTextResource reads it directly instead
// of re-resolving through the row cache, so a y/J toggle (or any other
// GetTextResource caller) sees enriched fields even when no Detail screen is
// on the stack to have received them via ApplyDetailEnrichmentForResource.
type TextState struct {
	Lines        []string          `json:"lines,omitempty"`
	Search       string            `json:"search,omitempty"`
	SearchCursor int               `json:"search_cursor"`
	Wrap         bool              `json:"wrap,omitempty"`
	ScrollY      int               `json:"scroll_y"`
	Resource     resource.Resource `json:"resource,omitzero"`
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
// Maps the CONTROLLER bucket from docs/historical/analysis/web-ui-state-inventory.md §MainMenuModel.
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
	// IssueTruncAuthoritative tracks, per resource type, whether the current
	// IssueTruncated[canon]=true was set by an authoritative Wave-2
	// enrichment result (true) as opposed to a non-authoritative rows-derived
	// sync (false/absent) — see syncMenuIssueCount's doc comment. Only an
	// authoritative caller may clear a truncation flag it did not itself set
	// without authority; a rows-derived equal-count resync may still clear a
	// flag that was itself seeded non-authoritatively (e.g. by PatchMenu or a
	// prior rows-derived sync).
	IssueTruncAuthoritative map[string]bool `json:"issue_trunc_authoritative,omitempty"`
	// Origin tracks, per resource type, whether the stored availability count
	// is disk-cache-seeded ("cache") or confirmed by a live probe this
	// session ("verified") — DEF-6/C3.
	Origin map[string]string `json:"origin,omitempty"`

	// Progress fields for FrameTitle indicator (DERIVED at Snapshot, stored here
	// so intents can update them without re-computing from task state).
	AvailChecked  int `json:"avail_checked,omitempty"`
	AvailTotal    int `json:"avail_total,omitempty"`
	EnrichChecked int `json:"enrich_checked,omitempty"`
	EnrichTotal   int `json:"enrich_total,omitempty"`
}
