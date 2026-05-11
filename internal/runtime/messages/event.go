package messages

import (
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ResourcesLoaded is sent when AWS resources have been fetched.
type ResourcesLoaded struct {
	ResourceType string
	Resources    []resource.Resource
	Pagination   *resource.PaginationMeta // nil when result has no pagination info
	Append       bool                     // true = append to existing list
	// TypeGen is the enrichment-rerun token. 0 on normal fetches (no rerun
	// intent). Non-zero only when the message originates from the
	// Ctrl+R-for-rerun wrapped fetch: it carries the per-type enrichment
	// generation captured at dispatch time. The handler applies the list
	// update unconditionally, then — after its existing write-through block
	// — checks this field; if it matches the current per-type gen, it seeds
	// probeResources and dispatches probeEnrichment.
	TypeGen domain.Gen
	// Err is non-nil when the paginated fetcher returned a partial-success
	// composite error: SOME resources made it back AND something failed
	// (e.g. one inline-group-policy enumeration call timed out). The handler
	// renders Resources as usual AND routes Err through Flash so the `!`
	// log records the partial failure.
	Err error
}

func (ResourcesLoaded) isEvent()              {}
func (m ResourcesLoaded) GenStamp() domain.Gen { return m.TypeGen }

// APIError is sent when an AWS API call fails.
type APIError struct {
	ResourceType string
	Err          error
}

func (APIError) isEvent() {}

// Flash sets a transient message in the header right side.
type Flash struct {
	Text    string
	IsError bool
}

func (Flash) isEvent() {}

// ClearFlash is sent after the flash auto-clear timer expires.
type ClearFlash struct {
	Gen domain.Gen // only clear if this matches current flash generation
}

func (ClearFlash) isEvent()              {}
func (m ClearFlash) GenStamp() domain.Gen { return m.Gen }

// ValueRevealed is sent when a resource value has been fetched via reveal (x key).
type ValueRevealed struct {
	ResourceType string // e.g., "secrets", "ssm"
	ResourceID   string // secret name or parameter name
	Value        string
	Err          error
}

func (ValueRevealed) isEvent() {}

// Copied is sent after a successful clipboard copy.
type Copied struct {
	Content string
}

func (Copied) isEvent() {}

// ClientsReady is sent when AWS clients are initialized.
// Clients is typed as any to avoid importing aws/ from the messages package.
// The adapter type-asserts it to *awsclient.ServiceClients.
type ClientsReady struct {
	Clients any
	Err     error
	Region  string     // resolved region from AWS config (set on success)
	Gen     domain.Gen // connect generation — ignore if != current connectGen
}

func (ClientsReady) isEvent()              {}
func (m ClientsReady) GenStamp() domain.Gen { return m.Gen }

// RelatedCheckResult delivers one checker's async result back to the detail view.
// The adapter delegates this to the active view (detail model's rightColumnModel).
type RelatedCheckResult struct {
	ResourceType     string
	SourceResourceID string // ID of the source resource (for cache keying)
	DefDisplayName   string // unique def.DisplayName — disambiguates multiple defs sharing a TargetType (e.g. ct-events self-pivots)
	Result           resource.RelatedCheckResult
	Generation       domain.Gen // dispatch generation — discard if != current RelatedGen
	// CachedPages contains full top-level resource pages fetched from AWS on a
	// cold cache miss, keyed by target resource short name. Non-nil only when
	// the NeedsTargetCache prefetch executed a live fetch (i.e., target was
	// absent from the ResourceCache snapshot passed to the checker). These
	// pages represent authoritative first-page results from the paginated
	// top-level fetcher and replace any absent cache entry verbatim.
	// Nil on cache hit or in demo mode — the app handler skips nil maps.
	CachedPages map[string]resource.ResourceCacheEntry
	// LazyAddedResources contains resources pulled via FetchByIDs when a
	// checker emitted target IDs outside the top-level fetcher's filter (KMS
	// customer-managed, AMI Owners=self, EBS snapshot Owners=self, IAM Policy
	// Scope=Local). Unlike CachedPages, these are NOT a complete first page —
	// they are a sparse set of IDs. The app handler merges them (append dedup
	// by ID) into any existing cache entry; if no entry exists, creates one
	// marked IsTruncated=true so the next top-level navigation still fetches
	// the full list authoritatively. Nil when no lazy-add occurred.
	LazyAddedResources map[string][]resource.Resource
	// LazyAddError is non-nil when FetchByIDs partially or fully failed during
	// the lazy-add path. The partial results (if any) are still present in
	// LazyAddedResources. The app handler converts this into a Flash so
	// operators see a visible error rather than a silent skip.
	LazyAddError error
}

func (RelatedCheckResult) isEvent()              {}
func (m RelatedCheckResult) GenStamp() domain.Gen { return m.Generation }

// AvailabilityCacheLoaded delivers cached availability data loaded from disk.
// Entries maps resource short names to resource counts.
// Only entries with a successful check (no error) are included.
type AvailabilityCacheLoaded struct {
	Entries        map[string]int  // shortName -> resource count
	Truncated      map[string]bool // shortName -> true if truncated
	Expired        bool            // true if cache was beyond TTL
	IssueCounts    map[string]int  // shortName -> cached issue count
	IssueTruncated map[string]bool // shortName -> true if issue count was truncated
	IssueKnown     map[string]bool // shortName -> true if issue count was probed (vs unknown)
}

func (AvailabilityCacheLoaded) isEvent() {}

// AvailabilityPrefetched is returned by the synchronous prefetch path in
// no-cache mode (e.g. demo with pre-supplied clients). Unlike
// AvailabilityCacheLoaded it does NOT trigger background probes — all counts
// are already populated.
type AvailabilityPrefetched struct {
	Entries        map[string]int                      // shortName -> resource count
	Truncated      map[string]bool                     // shortName -> true if truncated
	IssueCounts    map[string]int                      // shortName -> issue-status resource count
	IssueTruncated map[string]bool                     // shortName -> true if issue count is lower bound
	Resources      map[string][]resource.Resource      // shortName -> retained first-page resources for Wave 2
	Pagination     map[string]*resource.PaginationMeta // shortName -> full pagination meta (NextToken, etc.) for cache seeding
	Gen            domain.Gen                          // availabilityGen captured at dispatch — stale if != current
	// PrefetchErr is the composite error aggregating per-type fetch failures
	// during the synchronous availability prefetch. Non-nil when any paginated
	// fetcher errored; the app handler surfaces it as a Flash so operators
	// see permission/throttle issues rather than silently missing types.
	PrefetchErr error
}

func (AvailabilityPrefetched) isEvent()              {}
func (m AvailabilityPrefetched) GenStamp() domain.Gen { return m.Gen }

// AvailabilityChecked reports one resource type's background probe result.
type AvailabilityChecked struct {
	ResourceType string
	HasResources bool
	Count        int                 // number of resources found
	Truncated    bool                // true if count is from a truncated first page
	Err          error               // non-nil means "couldn't check" -- treat as unknown, don't grey out
	Gen          domain.Gen          // generation counter -- ignore if != current availabilityGen
	Issues       int                 // count of IsIssueRowColor() resources (red/yellow only)
	Resources    []resource.Resource // Populated on success AND on partial-success (Err non-nil but partial results present)
}

func (AvailabilityChecked) isEvent()              {}
func (m AvailabilityChecked) GenStamp() domain.Gen { return m.Gen }

// EnrichmentChecked reports one resource type's Wave 2 enrichment result.
type EnrichmentChecked struct {
	ResourceType string
	Issues       int  // updated issue count after enrichment (menu badge — ! severity only)
	Truncated    bool // whether the enrichment count is a lower bound
	// Findings is the per-resource finding map for this type, keyed by
	// resource.Resource.ID. Populated on success AND on partial-success (Err
	// non-nil but partial results present). May include findings for resources
	// off-page (account-wide enrichers).
	Findings map[string]resource.EnrichmentFinding
	// FieldUpdates carries per-resource Fields[] mutations to merge into
	// cached rows. Keyed by resource ID then by field key. Populated on success
	// AND on partial-success (Err non-nil but partial results present).
	FieldUpdates map[string]map[string]string
	// TruncatedIDs carries the per-resource truncation signal from the enricher.
	// Keyed by Resource.ID. Rows in this set are rendered as "?" because the
	// enricher could not fully inspect them (per-resource API error or page cap).
	TruncatedIDs map[string]bool
	Err          error      // enrichment error (nil on success)
	Gen          domain.Gen // session-wide generation counter (stale probe protection; profile/region switch)
	TypeGen      domain.Gen // per-type generation counter; bumped on every rerun for that type. Stale
	// results whose TypeGen doesn't match the current per-type gen are discarded.
}

func (EnrichmentChecked) isEvent()              {}
func (m EnrichmentChecked) GenStamp() domain.Gen { return m.Gen }

// IdentityLoaded is sent when the caller identity has been fetched.
// Identity is typed as any to avoid importing aws/ from the messages package.
// The adapter type-asserts it to *awsclient.CallerIdentity.
type IdentityLoaded struct {
	Identity any
}

func (IdentityLoaded) isEvent() {}

// IdentityError is sent when the caller identity fetch fails.
type IdentityError struct {
	Err string
}

func (IdentityError) isEvent() {}

// EnrichDetailResult delivers an enriched resource back to the detail view.
// On success, the detail view replaces its resource and rebuilds the field list.
// Generation is stamped by the dispatcher and validated by the adapter to
// discard stale results after Ctrl+R or navigation away.
type EnrichDetailResult struct {
	ResourceType string
	ResourceID   string
	EnrichedRes  resource.Resource
	Err          error
	Generation   domain.Gen
}

func (EnrichDetailResult) isEvent()              {}
func (m EnrichDetailResult) GenStamp() domain.Gen { return m.Generation }
