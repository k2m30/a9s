// Package domain — see contracts.go for the package overview.
//
// resource_cache.go owns the platform-agnostic list-view cache entry shape
// used to restore a top-level resource list when the user re-enters it
// from the main menu. The concrete store (session.Session.RowStore) lives
// in internal/session and is mutated by runtime handlers (via
// Core.SetResourceCache and friends); this file owns the per-entry value
// type so renderer adapters and the session package can both reference it
// without an import cycle.
//
// ListViewCacheEntry disambiguates against domain.ResourceCacheEntry above
// (related-checker cache snapshot — different shape, different purpose).
//
// Resources / Pagination drive the next render; FilterText, AttentionOnly,
// SortColIdx, SortAsc, CursorPos, HScrollOffset preserve the list view's
// interactive state across re-entry.
package domain

// ListViewCacheEntry stores the state of a previously-viewed resource list.
// Used to restore the list when the user re-enters the same resource type
// from the main menu, avoiding redundant API calls.
type ListViewCacheEntry struct {
	Resources     []Resource
	Pagination    *PaginationMeta
	FilterText    string
	AttentionOnly bool // §7.3: ctrl+z toggle persisted across view re-entry
	SortColIdx    int
	SortAsc       bool
	CursorPos     int
	HScrollOffset int
	// TotalCount is the authoritative total known for this list at seed time,
	// when it may exceed len(Resources) (item B, #17 wave 2, DEF-21: the C6a
	// reconstructable disk pair can hold Count > len(Rows) — Rows are only the
	// last-known pages, Count is the authoritative total). Zero means
	// "unknown/not applicable" — len(Resources) IS the authoritative count for
	// every seed source except the on-disk C6a fallback, so callers only set
	// this in that one branch. The seeded list's title prefers TotalCount over
	// len(Rows) until the next real fetch result lands and clears it.
	TotalCount int
}
