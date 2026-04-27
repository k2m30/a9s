package domain

import "context"

// ─── Column / view types ───────────────────────────────────────────────────

// Column defines a column in a resource table view.
// Moved from internal/resource/types.go in Phase 01.
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
// resource list. Moved from internal/resource/types.go in Phase 01.
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
// Moved from internal/resource/related.go in Phase 01.
type RelatedDef struct {
	TargetType       string // target resource short name (e.g., "tg", "alarm")
	DisplayName      string // right-column row label (e.g., "Target Groups")
	Checker          RelatedChecker
	NeedsTargetCache bool
}

// NavigableField associates a detail view field path with a target resource type.
// Moved from internal/resource/related.go in Phase 01.
type NavigableField struct {
	FieldPath  string // matches a path in ViewDef.Detail (e.g., "VpcId")
	TargetType string // resource short name (e.g., "vpc")
}

// ResourceCacheEntry holds a snapshot of one resource type's list plus
// truncation state. Moved from internal/resource/related.go in Phase 01.
type ResourceCacheEntry struct {
	Resources   []Resource
	IsTruncated bool
	Pagination  *PaginationMeta
}

// ResourceCache is a read-only snapshot of already-loaded resource lists,
// keyed by resource short name. Moved from internal/resource/related.go in Phase 01.
type ResourceCache map[string]ResourceCacheEntry

// ─── Pagination types ──────────────────────────────────────────────────────

// PaginationMeta holds cursor state for paginated fetches.
// Moved from internal/resource/pagination.go in Phase 01.
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
// Moved from internal/resource/pagination.go in Phase 01.
type FetchResult struct {
	Resources  []Resource
	Pagination *PaginationMeta // nil when pagination info is not available
}

// ─── Parent context ────────────────────────────────────────────────────────

// ParentContext holds key-value pairs passed from a parent view to a child
// fetcher. Moved from internal/resource/registry.go in Phase 01.
type ParentContext map[string]string

// ─── Function signatures ───────────────────────────────────────────────────
//
// These are MOVED here from internal/resource/* and internal/aws/* in Phase 01.
// The original sites keep `type X = domain.X` re-export aliases.
// Current signatures use `any` for clients and `string` for tokens.
// The aspirational Capabilities/Cursor redesign is a later phase.

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
// Moved from internal/resource/enricher.go in Phase 01.
type DetailEnricher func(ctx context.Context, clients any, res Resource) (Resource, error)

// RelatedChecker returns a count of related resources of a specific type.
// Moved from internal/resource/related.go in Phase 01.
// Note: returns RelatedCheckResult which remains in internal/resource/ for now.
type RelatedChecker func(ctx context.Context, clients any, res Resource, cache ResourceCache) RelatedCheckResult

// RelatedCheckResult is returned by a RelatedChecker.
// Kept here alongside RelatedChecker to avoid a circular dependency.
type RelatedCheckResult struct {
	TargetType  string
	Count       int      // -1 = unknown; 0+ = count
	ResourceIDs []string // IDs of found related resources
	Err         error
	FetchFilter map[string]string
	Approximate bool
}

// ─── Capability IDs ────────────────────────────────────────────────────────

// CapabilityID identifies a named capability a resource type may declare.
type CapabilityID string

const (
	CapLogs           CapabilityID = "logs"
	CapCloudTrailScan CapabilityID = "ct.scan"
	CapCost           CapabilityID = "cost"
)

// ─── Query spec types ──────────────────────────────────────────────────────

// QueryFilter is a string-based filter hint passed to capability modules.
type QueryFilter string

// TimeRange specifies a time window in unix seconds. Zero means open-ended.
type TimeRange struct {
	Start, End int64
}

// Cursor is an opaque pagination handle.
type Cursor string

// QueryLimit is a row cap. Zero means unlimited.
type QueryLimit int

// QuerySpec bundles the query parameters passed to capability modules.
type QuerySpec struct {
	Filter    QueryFilter
	TimeRange TimeRange
	Cursor    Cursor
	Limit     QueryLimit
}
