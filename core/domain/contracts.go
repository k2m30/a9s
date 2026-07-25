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
//
// Every field is unexported. This is deliberate, not incidental: a
// RelatedChecker must report one of four outcomes — a proven count (0..N,
// optionally a truncated lower bound), "could not determine", "the lookup
// errored", or "resolved via a deferred server-side filter" — and those four
// are mutually exclusive by construction only if nothing outside this package
// can populate the fields directly. With exported fields, any of the ~136
// checker files could (and did) write `RelatedCheckResult{TargetType: t,
// Count: 0}` after a swallowed error or a denied call, reporting a confident
// zero for what was actually "we don't know". Unexporting the fields makes
// that literal a compile error from every other package; the only way to
// build a value is through KnownRelated / UnknownRelated / ErrorRelated /
// DeferredRelated (see related_result.go), each of which can express exactly
// one outcome. No constructor accepts both a count/ids and an error, so
// "count alongside a failure" has no expressible shape.
type RelatedCheckResult struct {
	targetType string
	// state classifies how count should be interpreted; see RelatedRowState.
	// The zero value (RelatedResolved) means count (0..N) is authoritative.
	state       RelatedRowState
	count       int      // authoritative only when state == RelatedResolved
	resourceIDs []string // IDs of found related resources
	err         error
	fetchFilter map[string]string
	// truncated stays orthogonal to state: it modifies a RelatedResolved
	// result derived from a truncated cache page, or a partial-success union
	// of calls where some failed ("N+"), never the other states.
	truncated bool
}

// TargetType returns the related resource type this result describes.
func (r RelatedCheckResult) TargetType() string { return r.targetType }

// State returns the raw resolution state the checker (or constructor) set.
// Consumers deriving a row disposition should prefer EffectiveState.
func (r RelatedCheckResult) State() RelatedRowState { return r.state }

// Count returns the resolved count. Authoritative only when EffectiveState
// returns RelatedResolved.
func (r RelatedCheckResult) Count() int { return r.count }

// ResourceIDs returns the IDs of found related resources.
func (r RelatedCheckResult) ResourceIDs() []string { return r.resourceIDs }

// Err returns the error that caused RelatedError, or nil.
func (r RelatedCheckResult) Err() error { return r.err }

// FetchFilter returns the server-side filter for a RelatedDeferred result, or nil.
func (r RelatedCheckResult) FetchFilter() map[string]string { return r.fetchFilter }

// Truncated reports whether Count is a lower bound rather than an exact count.
func (r RelatedCheckResult) Truncated() bool { return r.truncated }

// WithTargetType returns a copy of r retargeted to targetType. Checkers built
// from shared, target-agnostic logic (e.g. a two-hop lookup reused across
// pivots) may leave targetType unset; the executor stamps it from the
// RelatedDef that invoked the checker. Every other field is preserved
// unchanged, so this cannot be used to smuggle a count alongside an error.
func (r RelatedCheckResult) WithTargetType(targetType string) RelatedCheckResult {
	r.targetType = targetType
	return r
}

// WithFetchFilter returns a copy of r carrying filter as its server-side
// FetchFilter, alongside whatever count/truncated state r already has. A
// windowed pivot (e.g. ct-events) can be both locally resolved AND carry a
// filter so Enter can re-fetch the full, unwindowed answer server-side —
// this is a legitimate composite, unlike count-alongside-an-error, which no
// constructor can express.
func (r RelatedCheckResult) WithFetchFilter(filter map[string]string) RelatedCheckResult {
	r.fetchFilter = filter
	return r
}

// EffectiveState returns the row-disposition state consumers must act on. No
// exported constructor can build a result carrying both a count and an err
// (a partial-success union of calls where some failed is expressed via
// KnownRelated's truncated flag instead — see related_result.go), so err != nil
// implies count == 0 in every value this package can produce. The err != nil
// → RelatedError branch below is a defensive fallback, not a live path: err
// dominates whatever state is set, so the row renders blank and dimmed — a
// dead end — rather than showing a count that could not be trusted. The
// failure is surfaced separately (Flash{IsError:true} + "!" log) and the user
// retries with Ctrl+R. Every site that derives a mirror-row state or
// actionability from a result must go through this rather than reading State
// directly, so IsRelatedActionable stays the single source of truth.
func (r RelatedCheckResult) EffectiveState() RelatedRowState {
	if r.err != nil {
		return RelatedError
	}
	return r.state
}
