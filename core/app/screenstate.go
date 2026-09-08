// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app

import (
	"maps"

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
	HasPagination bool `json:"has_pagination,omitempty"`
	// PopulationUnconfirmed is true when this list cannot assert that its rows
	// are the type's whole population. HasPagination implies it — a page left
	// behind is a population not seen — but so does a partial-success fetch
	// that DID reach the last page while a sibling enumeration failed.
	//
	// The two are separate fields because they drive opposite surfaces.
	// HasPagination is "another page exists": the "+" on the count, the "m"
	// hint, the load-more dispatch, all of which need a cursor to be true of.
	// PopulationUnconfirmed is "do not record this as an exact total": the
	// disk-cache exactness flag and the menu's count sync-back. Carrying both
	// on one field titled a fully-loaded list as "N+" and offered a load-more
	// with no cursor to follow.
	PopulationUnconfirmed bool `json:"population_unconfirmed,omitempty"`

	// instance identifies THIS list screen for the length of its life on the
	// stack. Two screens of one resource type are otherwise indistinguishable
	// to a result coming back: an AMI's EC2 drill stacked on another drill of
	// the same type share their type and their lane, so a continuation the
	// deeper one asked for lands on whichever is topmost. Every fetch a screen
	// dispatches carries this id, and the result and the failure carry it
	// back, so a page always returns to the screen that asked for it — and a
	// body built off the lock for one screen cannot install into another.
	//
	// Session-local and never serialised: a screen restored from a snapshot is
	// a new instance, and nothing outside this process can name one.
	instance domain.Gen

	// loadingSeq and loadingMoreSeq name the request that raised each of the
	// two activity flags — Loading/Refreshing on one lane, LoadingMore on the
	// other. A completion retires the flag it owns and no other: a Ctrl+R
	// issued while an earlier refresh is still out supersedes it, and when the
	// older one returns it is discarded, so letting it clear the marker would
	// tell the operator the loading is over while the newer fetch is still in
	// flight. Zero means the flag was raised without a sequence (a lane that
	// draws none, a synthetic seed), and any completion may retire it.
	loadingSeq     domain.Gen
	loadingMoreSeq domain.Gen

	// FetchIncomplete records that some component of this list's fetch did not
	// enumerate everything it was asked for — a partial-success result, where
	// rows landed AND something failed. It is STICKY across the pages of one
	// list: a non-append fetch starts the list over and so resets it, an
	// appended page ORs its own answer in. Page two saying "no more pages" is
	// a different fact from page one's failure to enumerate, and letting it
	// overwrite the failure recorded the list as the type's exact population
	// on the strength of a page that never saw what was missing.
	FetchIncomplete bool                `json:"fetch_incomplete,omitempty"`
	AutoOpenSingle  bool                `json:"auto_open_single,omitempty"`
	RelatedIDSet    map[string]struct{} `json:"related_id_set,omitempty"`

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

	// Loading tracks whether the initial fetch is still in flight. Renderers
	// treat it as the top-priority gate (RenderList/buildListFrameTitle return
	// early on Loading before looking at LoadingMore/Refreshing).
	Loading bool `json:"loading,omitempty"`
	// LoadingMore tracks whether an m-key/target-chase load-more fetch is in
	// flight. NOT mutually exclusive with Refreshing: a Ctrl+R refresh can
	// land while a load-more page is still outstanding (activeListRefreshTasks
	// sets Refreshing without checking LoadingMore), and RenderList renders
	// both the load-more hint and the refreshing marker in that case.
	LoadingMore bool `json:"loading_more,omitempty"`
	// Refreshing is true when the screen opened with rows seeded from a
	// cache-first source (session ProbeResources, a previous visit's
	// ResourceCache, or the on-disk availability cache) while a fresh fetch
	// is still in flight to confirm/replace them, or when an explicit Ctrl+R
	// refresh is confirming the currently-shown rows. Cleared once the fetch's
	// ResourcesLoaded result lands. Distinct from Loading, which gates the
	// no-rows-known spinner path; see LoadingMore for why this is not
	// mutually exclusive with it.
	Refreshing bool `json:"refreshing,omitempty"`
	// LastFetchError is the error marker text for the most recent failed
	// fetch over this screen, or "" when no error is outstanding (C4).
	// Set by a messages.APIError landing while this screen is active; cleared
	// on the next successful ResourcesLoaded for this screen.
	LastFetchError string `json:"last_fetch_error,omitempty"`
	// TotalCount is the population a cache-first seed source reported for this
	// type (cache.TypeFile.Population / the row store's own TotalCount) while
	// the seed itself carried only the rows that source retained. Set by
	// cache-first seeding callers AFTER applyResourcesLoaded, mirroring
	// Refreshing's set-after-seed ordering. buildListFrameTitle prefers it
	// over len(Rows) while it is set. Zero means "not applicable" — every
	// other seed source already has len(Rows) == the authoritative total.
	// Retired by the first fetch result that supersedes it (applyResourcesLoaded).
	TotalCount int `json:"total_count,omitempty"`
	// rowsVersion counts every content-changing mutation of Rows/RelatedIDSet
	// on this screen: applyResourcesLoaded's non-stale write branches,
	// seedRelatedExactRows, seedFilteredListFromCache, applyListFieldUpdates,
	// applyRowFindings, clearRowFindings, PatchListRelatedIDSet,
	// patchListReapplyChecker, and reapplyCheckerAgainst (core/app/list_body.go,
	// list_state.go, list_filter.go, navigate.go). buildListBody's memo
	// (list_body.go) keys on this rather than RowsGen because RowsGen only
	// advances for the canonical top-level list path (Core.ObserveRows) and
	// stays zero for a child/related/filtered screen or an in-place
	// Fields/Findings mutation — exactly the cases a RowsGen-only key would
	// miss. In-memory only, like reapplyChecker/reapplySource below — not
	// part of the JSON snapshot.
	rowsVersion uint64

	// bodyMemo caches the expensive part of the last buildListBody result for
	// this screen (resolved columns, the filtered+sorted+decorated row set,
	// and the marker/status column indices) so a frame whose row-affecting
	// inputs are unchanged — the common cursor-move/spinner-tick render —
	// skips the O(n log n) sort and O(n·cols) cell extraction. See
	// listBodyMemo in list_body.go.
	bodyMemo listBodyMemo
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
	// ViewportWidth is the renderer-supplied usable field-panel width, set on
	// resize via Controller.SetDetailViewportWidth. The body builder wraps the
	// Attention sentence at it so the one line a reader must act on is never
	// cut at the panel edge; zero means no renderer has reported a width and
	// the builder falls back to defaultAttentionWrapWidth.
	ViewportWidth int `json:"viewport_width,omitempty"`

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
	// cursorLayout is what the LAST BUILD of the body — the one the operator
	// is looking at — observed about the row under the cursor and the size of
	// the Attention block it was indexed against. snapshot() is the only writer,
	// because it is the only build that reaches a screen; applyFindingToState is
	// the only reader, relocating the cursor into the rebuilt layout by the
	// identity recorded here.
	//
	// Recorded rather than recomputed: the block depends on more than
	// ds.Findings — the "not inspected" entry comes from the session
	// truncated-ID set, which the runtime writes before the controller applies
	// the intent — so anything derived later describes a layout that was never
	// on screen. Unexported, and so never serialised: it is a record of what a
	// builder emitted, not state a client supplies.
	cursorLayout detailLayout
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
	// session ("verified") — C3.
	Origin map[string]string `json:"origin,omitempty"`
	// ProbeCause tracks, per resource type, the error class of the last
	// availability probe that FAILED for it (classifyProbeErr's vocabulary:
	// "access-denied", "expired", "throttled", a raw AWS code), cleared the
	// moment a probe answers successfully. A cached count on a row whose probe
	// was refused is not the same fact as a cached count nobody has re-checked
	// yet, and this is what tells them apart.
	ProbeCause map[string]string `json:"probe_cause,omitempty"`

	// Progress fields for FrameTitle indicator (DERIVED at Snapshot, stored here
	// so intents can update them without re-computing from task state).
	AvailChecked  int `json:"avail_checked,omitempty"`
	AvailTotal    int `json:"avail_total,omitempty"`
	EnrichChecked int `json:"enrich_checked,omitempty"`
	EnrichTotal   int `json:"enrich_total,omitempty"`
}

// ClearAvailability drops every fact this session's probes established about
// individual resource types. It is the one clear point a profile/region
// rotation goes through: what a type's count was, where it came from, and why
// its probe failed are all facts about the OLD profile/region pair, and none
// of them may describe a row under the new one.
func (ms *MenuState) ClearAvailability() {
	ms.Availability = nil
	ms.Truncated = nil
	ms.ProbeCause = nil
	ms.Origin = nil
	ms.IssueTruncAuthoritative = nil
	ms.IssueCounts = nil
	ms.IssueKnown = nil
	ms.IssueTruncated = nil
	ms.AvailChecked = 0
	ms.AvailTotal = 0
	ms.EnrichChecked = 0
	ms.EnrichTotal = 0
}

// cloneForBuild returns a detached copy of ls for an off-lock body build,
// with rows as its row set. Every reference-typed field is copied, so nothing
// the build reads can be written by a locked caller while it runs — a result
// landing under the lock inserts into RelatedIDSet through
// reapplyCheckerAgainst, and a build iterating the same map is a data race Go
// ends the process on.
//
// A new map or slice field on ListState belongs here. Copying the struct
// alone shares it, and the detector only reports it when a reapply and a
// build happen to overlap.
//
// The memo is dropped: the previous generation is not an input to the next,
// and carrying it would keep that generation's rows alive for the build.
func (ls *ListState) cloneForBuild(rows []resource.Resource) ListState {
	out := *ls
	out.bodyMemo = listBodyMemo{}
	out.Rows = make([]resource.Resource, len(rows))
	copy(out.Rows, rows)
	out.RelatedIDSet = maps.Clone(ls.RelatedIDSet)
	out.FetchFilter = maps.Clone(ls.FetchFilter)
	out.ParentContext = maps.Clone(ls.ParentContext)
	return out
}

// canonicalScreenType is the canonical short name of the screen's own
// resource type. A screen opened through an alias ("buckets", "workgroups")
// keeps the alias it was opened with, so the delivery scan resolves it here
// instead of assuming the stack already spells the type the way every
// message does — the message's own name is canonical before it reaches the
// controller (runtime.StampListResult).
func canonicalScreenType(s *Screen) string {
	return resource.CanonicalShortName(s.Ctx.ResourceType)
}
