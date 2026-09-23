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
	// Path is the RawStruct field path the cell reads when the row carries no
	// Fields entry for Key — the value source a view file spells as "path:".
	Path string
	// Width is the fixed column width; 0 means flexible.
	Width int
	// SortKey is the Fields key the comparator reads when the displayed value
	// does not sort the way the value does (a size rendered "900 B", a status
	// rendered as a finding phrase).
	SortKey string
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

// RefContext is what a target type's reference resolver may consult besides
// the reference itself. An empty AccountID or Region means "not known yet",
// never "foreign". Targets are the target type's cached rows, nil when the
// list has not been loaded.
type RefContext struct {
	AccountID, Region string
	Targets           []Resource
}

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
	// Mirror marks this direction and its reverse as one relationship: both
	// read the same AWS fact from its two ends, so on any data A lists B
	// exactly when B lists A. Set on both directions or neither.
	Mirror bool
	// Distinct names, in one phrase, the AWS fact this direction reads, for a
	// direction whose reverse reads a different one: the two are two
	// relationships between the same pair of types, and a row one side lists
	// while the other does not is the answer, not a defect. Empty on a Mirror
	// direction; a direction that is neither is undecided, which the pair
	// gate rejects.
	Distinct string
}

// NavigableField associates a detail view field path with a target resource type.
type NavigableField struct {
	FieldPath  string // matches a path in ViewDef.Detail (e.g., "VpcId")
	TargetType string // resource short name (e.g., "vpc")
	// Resolve, when set, names the target row the field opens for src, given
	// the field's value and the loaded target rows (nil when that list is not
	// loaded); "" leaves the field not navigable. It is for a field whose value
	// alone does not identify the row, such as a name another row may have
	// taken since, or a route target a blackholed route still names.
	Resolve func(src Resource, value string, targets []Resource) string
}

// ResourceCacheEntry holds a snapshot of one resource type's list plus
// truncation state.
type ResourceCacheEntry struct {
	Resources   []Resource
	IsTruncated bool
	Pagination  *PaginationMeta
	// FieldsOnly marks rows restored from the disk cache, which carry ID,
	// Name and Fields but no RawStruct. A reader that matches on the SDK
	// struct treats such an entry as absent and fetches the type live; a
	// reader that matches on Fields uses it as is.
	FieldsOnly bool
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
	// LowerBoundOnly marks an IsTruncated=true, NextToken="" pairing as
	// deliberate: a fetcher that knows on its own it can never see the full
	// picture (e.g. a cheap probe that skips an expensive sub-resource
	// sweep on purpose) and reports a permanent, non-resumable lower-bound
	// count. Without this flag, that same pairing is indistinguishable from
	// a fetcher that hit a local cap and forgot to wire a cursor — the
	// accidental case sanitizeFetchResult (core/resource/accessors.go)
	// downgrades to exact. Leave false for every ordinary paginated result;
	// only a one-shot, never-resumed probe sets it true.
	LowerBoundOnly bool
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
type RelatedChecker func(ctx context.Context, clients any, res Resource, cache ResourceCache) RelatedCheckResult

// RelatedCheckResult is returned by a RelatedChecker.
// Kept here alongside RelatedChecker to avoid a circular dependency.
//
// Every field is unexported. This is deliberate, not incidental: a
// RelatedChecker must report one of four outcomes — a proven count (0..N,
// optionally a truncated lower bound), "could not determine", "the lookup
// errored", or "resolved via a deferred server-side filter" — and those four
// are mutually exclusive by construction only if nothing outside this package
// can populate the fields directly. With exported fields, any checker could
// write `RelatedCheckResult{TargetType: t, Count: 0}` after a swallowed error
// or a denied call, reporting a confident zero for what is actually "we don't
// know". Unexporting the fields makes
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
	// coverage is how far a RelatedResolved lookup searched; Coverage derives
	// the answer for every other state.
	coverage RelatedCoverage
	// region is the Region the lookup read its answer in, when that is not
	// the session's. A drill into the row has to read the same Region, or it
	// lists another one and finds none of what the count names.
	region string
}

// RelatedCoverage states how far a related lookup searched, and so whether a
// zero it reports is evidence of absence.
type RelatedCoverage uint8

const (
	// CoverageComplete: every place the relation is recorded was read; a zero
	// is a proven dead end.
	CoverageComplete RelatedCoverage = iota
	// CoveragePartial: the search stopped short (a capped page walk, a
	// truncated or degraded target list, a failed call beside successful
	// ones); the count is a lower bound.
	CoveragePartial
	// CoverageNoPath: nothing was searched — AWS records no link the lookup
	// could read, or the lookup had nothing to search with.
	CoverageNoPath
	// CoverageHeuristic: the matches share a property with the source rather
	// than a link AWS records; they are candidates, not a count.
	CoverageHeuristic
)

// Coverage returns how far the lookup behind r searched. Only a
// RelatedResolved result searched at all; a truncated one is partial. A
// heuristic match rule offers candidates however much of the list was read,
// so it outranks a partial scan.
func (r RelatedCheckResult) Coverage() RelatedCoverage {
	switch {
	case r.state != RelatedResolved:
		return CoverageNoPath
	case r.coverage == CoverageHeuristic:
		return CoverageHeuristic
	case r.truncated:
		return CoveragePartial
	default:
		return r.coverage
	}
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

// Region returns the Region this result was read in, or "" for the session's
// own. Whatever reads a Region off a result must fetch its rows there.
func (r RelatedCheckResult) Region() string { return r.region }

// WithRegion returns a copy of r marked as read in region. Only where the
// count came from, never what it counted: a Region carries no rows of its
// own, so this cannot turn an unknown into a number.
func (r RelatedCheckResult) WithRegion(region string) RelatedCheckResult {
	r.region = region
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
//
// A resolved result that searched nothing renders as RelatedUnknown, and one
// holding heuristic candidates as RelatedDeferred (blank, navigating to the
// candidates): neither count is a number the panel may show.
func (r RelatedCheckResult) EffectiveState() RelatedRowState {
	if r.err != nil {
		return RelatedError
	}
	switch r.Coverage() {
	case CoverageNoPath:
		if r.state == RelatedResolved {
			return RelatedUnknown
		}
	case CoverageHeuristic:
		return RelatedDeferred
	}
	return r.state
}
