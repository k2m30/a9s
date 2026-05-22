// Package domain — see contracts.go for the package overview.
//
// resource_cache.go owns the platform-agnostic list-view cache entry shape
// used to restore a top-level resource list when the user re-enters it
// from the main menu. The concrete map (`Session.ResourceCache`) lives in
// internal/session and is mutated by runtime handlers; this file owns the
// per-entry value type so renderer adapters and the session package can
// both reference it without an import cycle.
//
// ListViewCacheEntry — moved from internal/session/session.go in
// PR-05a-h4-c (AS-963). The original session.ResourceCacheEntry is now
// a type alias to this struct (see internal/session/session.go) so the
// existing read paths (resource.ResourceCache snapshots,
// runtime.HandleResourcesLoaded, etc.) keep compiling. The intentional
// name change from ResourceCacheEntry → ListViewCacheEntry disambiguates
// against the pre-existing domain.ResourceCacheEntry above (related-
// checker cache snapshot — different shape, different purpose).
//
// Field set mirrors the original session.ResourceCacheEntry verbatim:
// Resources / Pagination drive the next render; FilterText, AttentionOnly,
// SortColIdx, SortAsc, CursorPos, HScrollOffset preserve the list view's
// interactive state across re-entry.
package domain

// ListViewCacheEntry stores the state of a previously-viewed resource list.
// Used to restore the list when the user re-enters the same resource type
// from the main menu, avoiding redundant API calls. Moved from
// internal/session/session.go in PR-05a-h4-c (AS-963) so renderer adapters
// can construct entries without importing internal/session.
//
// The session-side type alias (`type ResourceCacheEntry = domain.ListViewCacheEntry`)
// keeps the existing `session.ResourceCacheEntry` name available for
// callers (tests, runtime handlers) that already reference it.
type ListViewCacheEntry struct {
	Resources     []Resource
	Pagination    *PaginationMeta
	FilterText    string
	AttentionOnly bool // §7.3: ctrl+z toggle persisted across view re-entry
	SortColIdx    int
	SortAsc       bool
	CursorPos     int
	HScrollOffset int
}
