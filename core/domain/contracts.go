// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package domain

import "context"

// ─── Column / view types ───────────────────────────────────────────────────

// Column defines a column in a resource table view.
type Column struct {
	// Key is the field key used to extract the value from Resource.Fields.
	Key string
	// Title is the column header display text.
	Title string
	// Width is the fixed column width; 0 means flexible.
	Width int
	// Sortable indicates whether this column supports sorting.
	Sortable bool
}

// ChildViewDef describes a child view that can be drilled into from a parent
// resource list.
type ChildViewDef struct {
	// ChildType is the registered child type short name (e.g., "s3_objects").
	ChildType string
	// Key is the trigger key name (e.g., "enter", "e", "L").
	Key string
	// ContextKeys maps child-fetcher parameter names to source expressions.
	ContextKeys map[string]string
	// DisplayNameKey is the context key whose value becomes the child view's
	// display name (frame title).
	DisplayNameKey string
	// DrillCondition is an optional predicate. When non-nil, the child view
	// is only entered if the predicate returns true for the selected resource.
	DrillCondition func(Resource) bool
	// DrillBlockMessage is the flash text shown when DrillCondition returns false.
	DrillBlockMessage string
}

// ─── Related types ─────────────────────────────────────────────────────────

// RelatedDef defines one related resource class for a given resource type.
type RelatedDef struct {
	TargetType       string // target resource short name (e.g., "tg", "alarm")
	DisplayName      string // right-column row label (e.g., "Target Groups")
	Checker          RelatedChecker
	NeedsTargetCache bool
	// Truncated is true when this pivot's checker can return a truncated
	// lower-bound ("N+") count — i.e. it scans a cache page that may itself
	// be truncated and returns relatedResultTrunc(...). It is orthogonal to
	// NeedsTargetCache (a prefetch flag), and is the single source of truth
	// for the per-resource docs' "Truncated?" column.
	Truncated bool
}

// NavigableField associates a detail view field path with a target resource type.
type NavigableField struct {
	FieldPath  string // matches a path in ViewDef.Detail (e.g., "VpcId")
	TargetType string // resource short name (e.g., "vpc")
}

// ResourceCacheEntry holds a snapshot of one resource type's list plus
// truncation state.
type ResourceCacheEntry struct {
	Resources   []Resource
	IsTruncated bool
	Pagination  *PaginationMeta
}

// ResourceCache is a read-only snapshot of already-loaded resource lists,
// keyed by resource short name.
type ResourceCache map[string]ResourceCacheEntry

// ─── Pagination types ──────────────────────────────────────────────────────

// PaginationMeta holds cursor state for paginated fetches.
type PaginationMeta struct {
	// IsTruncated is true when more pages exist beyond what was returned.
	IsTruncated bool
	// NextToken is an opaque continuation token for the next page.
	NextToken string
	// TotalHint is the known or estimated total count. -1 means unknown.
	TotalHint int
	// PageSize is the number of items returned in this page.
	PageSize int
}

// FetchResult wraps a resource page with pagination state.
type FetchResult struct {
	Resources  []Resource
	Pagination *PaginationMeta // nil when pagination info is not available
}

// ─── Parent context ────────────────────────────────────────────────────────

// ParentContext holds key-value pairs passed from a parent view to a child
// fetcher.
type ParentContext map[string]string

// ─── Function signatures ───────────────────────────────────────────────────
//
// The core/resource sites keep `type X = domain.X` re-export aliases.
// Current signatures use `any` for clients and `string` for tokens.

// PaginatedFetcher returns a single page of resources.
type PaginatedFetcher func(ctx context.Context, clients any, continuationToken string) (FetchResult, error)

// FilteredPaginatedFetcher returns a single page of resources filtered server-side.
type FilteredPaginatedFetcher func(ctx context.Context, clients any, filter map[string]string, continuationToken string) (FetchResult, error)

// PaginatedChildFetcher returns a single page of child resources.
type PaginatedChildFetcher func(ctx context.Context, clients any, parentCtx ParentContext, continuationToken string) (FetchResult, error)

// RevealFetcher fetches a reveal value for a resource by ID.
type RevealFetcher func(ctx context.Context, clients any, resourceID string) (string, error)

// FetchByIDsFunc fetches specific resource instances by ID, bypassing any
// filter the top-level paginated fetcher applies.
type FetchByIDsFunc func(ctx context.Context, clients any, ids []string) ([]Resource, error)

// DetailEnricher enriches a single resource on demand for detail views.
type DetailEnricher func(ctx context.Context, clients any, res Resource) (Resource, error)

// RelatedChecker returns a count of related resources of a specific type.
// Note: returns RelatedCheckResult which remains in core/resource/ for now.
type RelatedChecker func(ctx context.Context, clients any, res Resource, cache ResourceCache) RelatedCheckResult

// RelatedCheckResult is returned by a RelatedChecker.
// Kept here alongside RelatedChecker to avoid a circular dependency.
type RelatedCheckResult struct {
	TargetType string
	// State classifies how Count should be interpreted; see RelatedRowState.
	// The zero value (RelatedResolved) means Count (0..N) is authoritative.
	State       RelatedRowState
	Count       int      // authoritative only when State == RelatedResolved
	ResourceIDs []string // IDs of found related resources
	Err         error
	FetchFilter map[string]string
	// Truncated stays orthogonal to State: it modifies a RelatedResolved
	// result derived from a truncated cache page ("N+"), never the other states.
	Truncated bool
}

// EffectiveState returns the row-disposition state consumers must act on. A
// checker can return a partial success — a positive Count with real
// ResourceIDs — alongside an aggregate Err (e.g. the lambda→eb-rule checker
// when some ListTargetsByRule calls fail). The error dominates: Err != nil
// forces RelatedError over whatever State the checker left (typically the
// zero-value RelatedResolved), so the row renders blank and dimmed — a dead
// end — rather than showing a Count that could not be trusted. The failure is
// surfaced separately (Flash{IsError:true} + "!" log) and the user retries with
// Ctrl+R. Results already constructed as RelatedError are unaffected. Every
// site that derives a mirror-row State or actionability from a result must go
// through this rather than reading State directly, so IsRelatedActionable stays
// the single source of truth.
func (r RelatedCheckResult) EffectiveState() RelatedRowState {
	if r.Err != nil {
		return RelatedError
	}
	return r.State
}
