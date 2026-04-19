package messages

import (
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ViewTarget identifies a destination view for NavigateMsg.
type ViewTarget int

const (
	TargetMainMenu ViewTarget = iota
	TargetResourceList
	TargetDetail
	TargetYAML
	TargetJSON
	TargetReveal
	TargetProfile
	TargetRegion
	TargetTheme
	TargetHelp
)

// NavigateMsg requests a view transition. The root model handles push/pop.
type NavigateMsg struct {
	Target         ViewTarget
	ResourceType   string
	Resource       *resource.Resource
	ReplaceCurrent bool // when true, pop current view before pushing target (used by auto-open flows)
}

// PopViewMsg requests popping the current view from the stack.
type PopViewMsg struct{}

// ResourcesLoadedMsg is sent when AWS resources have been fetched.
type ResourcesLoadedMsg struct {
	ResourceType string
	Resources    []resource.Resource
	Pagination   *resource.PaginationMeta // nil when result has no pagination info
	Append       bool                     // true = append to existing list
	// TypeGen is the enrichment-rerun token. 0 on normal fetches (no rerun
	// intent). Non-zero only when the message originates from the
	// Ctrl+R-for-rerun wrapped fetch: it carries the per-type enrichment
	// generation captured at dispatch time. The ResourcesLoadedMsg handler
	// applies the list update unconditionally, then — after its existing
	// write-through block — checks this field; if it matches the current
	// per-type gen, it seeds probeResources and dispatches probeEnrichment.
	TypeGen int
}

// LoadMoreMsg triggers loading the next page of a paginated resource list.
type LoadMoreMsg struct {
	ResourceType      string
	ContinuationToken string
	ParentContext     map[string]string // non-nil for child views
	FetchFilter       map[string]string
}

// APIErrorMsg is sent when an AWS API call fails.
type APIErrorMsg struct {
	ResourceType string
	Err          error
}

// FlashMsg sets a transient message in the header right side.
type FlashMsg struct {
	Text    string
	IsError bool
}

// ClearFlashMsg is sent after the flash auto-clear timer expires.
type ClearFlashMsg struct {
	Gen int // only clear if this matches current flash generation
}

// ProfileSelectedMsg is sent when the user confirms a profile selection.
type ProfileSelectedMsg struct {
	Profile string
}

// RegionSelectedMsg is sent when the user confirms a region selection.
type RegionSelectedMsg struct {
	Region string
}

// ThemeSelectedMsg is sent when the user confirms a theme selection.
type ThemeSelectedMsg struct {
	Theme string
}

// ValueRevealedMsg is sent when a resource value has been fetched via reveal (x key).
type ValueRevealedMsg struct {
	ResourceType string // e.g., "secrets", "ssm"
	ResourceID   string // secret name or parameter name
	Value        string
	Err          error
}

// CopiedMsg is sent after a successful clipboard copy.
type CopiedMsg struct {
	Content string
}

// InitConnectMsg triggers the initial AWS session setup.
type InitConnectMsg struct {
	Profile string
	Region  string
}

// ClientsReadyMsg is sent when AWS clients are initialized.
// Clients is typed as any to avoid importing aws/ from the messages package.
// The root model type-asserts it to *awsclient.ServiceClients.
type ClientsReadyMsg struct {
	Clients any
	Err     error
	Region  string // resolved region from AWS config (set on success)
	Gen     int    // connect generation — ignore if != current connectGen
}

// EnterChildViewMsg signals that the user has triggered a child view navigation.
// The root model uses ChildType to look up the child type definition and fetcher,
// ParentContext to provide parameters to the child fetcher, and DisplayName
// for the child view's frame title.
type EnterChildViewMsg struct {
	ChildType     string
	ParentContext map[string]string
	DisplayName   string
}

// LoadResourcesMsg triggers an async fetch of resources for a given type.
type LoadResourcesMsg struct {
	ResourceType  string
	ParentContext map[string]string
}

// RefreshMsg triggers a re-fetch of the current resource list.
type RefreshMsg struct{}

// RelatedCheckStartedMsg requests that the root model dispatch related-resource checkers.
// Emitted by DetailModel when user presses 'r'. The root model handles this because
// it owns m.clients and m.resourceCache — views cannot dispatch AWS calls directly.
type RelatedCheckStartedMsg struct {
	ResourceType   string
	SourceResource resource.Resource // the resource being viewed
}

// RelatedCheckResultMsg delivers one checker's async result back to the detail view.
// The root model delegates this to the active view (detail model's rightColumnModel).
type RelatedCheckResultMsg struct {
	ResourceType     string
	SourceResourceID string // ID of the source resource (for cache keying)
	DefDisplayName   string // unique def.DisplayName — disambiguates multiple defs sharing a TargetType (e.g. ct-events self-pivots)
	Result           resource.RelatedCheckResult
	Generation       uint64 // dispatch generation — discard if != Model.relatedGen
	// CachedPages contains resource pages fetched from AWS on a cold cache miss,
	// keyed by target resource short name. Non-nil only when FetchRelatedTarget
	// executed a live fetch (i.e., target was absent from the ResourceCache snapshot
	// passed to the checker). The app handler writes these entries into m.resourceCache
	// so subsequent detail views for any resource type get a cache hit.
	// Nil on cache hit or in demo mode — the app handler skips nil maps.
	CachedPages map[string]resource.ResourceCacheEntry
}

// RelatedNavigateMsg requests navigation to a related resource type.
// Emitted by: (a) detail view when Enter pressed on navigable field,
//
//	(b) rightColumnModel when Enter pressed on selected row.
//
// Handled by: root model in app_related.go (handleRelatedNavigate).
type RelatedNavigateMsg struct {
	TargetType     string            // resource short name to navigate to (e.g., "vpc")
	SourceResource resource.Resource // the resource being viewed
	SourceType     string            // source resource short name (e.g., "ec2")
	TargetID       string            // specific ID for navigable field case (e.g., "vpc-0abc")
	RelatedIDs     []string          // IDs from checker for right-column case
	FetchFilter    map[string]string
}

// AvailabilityCacheLoadedMsg delivers cached availability data loaded from disk.
// Entries maps resource short names to resource counts.
// Only entries with a successful check (no error) are included.
type AvailabilityCacheLoadedMsg struct {
	Entries        map[string]int  // shortName -> resource count
	Truncated      map[string]bool // shortName -> true if truncated
	Expired        bool            // true if cache was beyond TTL
	IssueCounts    map[string]int  // shortName -> cached issue count
	IssueTruncated map[string]bool // shortName -> true if issue count was truncated
	IssueKnown     map[string]bool // shortName -> true if issue count was probed (vs unknown)
}

// AvailabilityPrefetchedMsg is returned by the synchronous prefetch path in
// no-cache mode (e.g. demo with pre-supplied clients). Unlike
// AvailabilityCacheLoadedMsg it does NOT trigger background probes — all counts
// are already populated.
type AvailabilityPrefetchedMsg struct {
	Entries        map[string]int                 // shortName -> resource count
	Truncated      map[string]bool                // shortName -> true if truncated
	IssueCounts    map[string]int                 // shortName -> issue-status resource count
	IssueTruncated map[string]bool                // shortName -> true if issue count is lower bound
	Resources      map[string][]resource.Resource // shortName -> retained first-page resources for Wave 2
	Gen            int                            // availabilityGen captured at dispatch — stale if != current
}

// AvailabilityCheckedMsg reports one resource type's background probe result.
type AvailabilityCheckedMsg struct {
	ResourceType string
	HasResources bool
	Count        int   // number of resources found
	Truncated    bool  // true if count is from a truncated first page
	Err          error // non-nil means "couldn't check" -- treat as unknown, don't grey out
	Gen          int   // generation counter -- ignore if != current availabilityGen
	Issues       int                 // count of IsIssueRowColor() resources (red/yellow only)
	Resources    []resource.Resource // retained first-page resources for Wave 2 enricher consumption
}

// EnrichmentCheckedMsg reports one resource type's Wave 2 enrichment result.
type EnrichmentCheckedMsg struct {
	ResourceType string
	Issues       int  // updated issue count after enrichment (menu badge — ! severity only)
	Truncated    bool // whether the enrichment count is a lower bound
	// Findings is the per-resource finding map for this type, keyed by
	// resource.Resource.ID. Populated on success; nil/empty when Err != nil.
	// May include findings for resources off-page (account-wide enrichers).
	Findings map[string]resource.EnrichmentFinding
	// FieldUpdates carries per-resource Fields[] mutations to merge into
	// cached rows. Keyed by resource ID then by field key. Nil when the
	// enricher produced no field updates.
	FieldUpdates map[string]map[string]string
	// TruncatedIDs carries the per-resource truncation signal from the enricher.
	// Keyed by Resource.ID. Rows in this set are rendered as "?" because the
	// enricher could not fully inspect them (per-resource API error or page cap).
	TruncatedIDs map[string]bool
	Err          error // enrichment error (nil on success)
	Gen          int   // session-wide generation counter (stale probe protection; profile/region switch)
	TypeGen      int   // per-type generation counter; bumped on every rerun for that type. Stale
	// results whose TypeGen doesn't match the current per-type gen are discarded.
}

// IdentityLoadedMsg is sent when the caller identity has been fetched.
// Identity is typed as any to avoid importing aws/ from the messages package.
// The root model type-asserts it to *awsclient.CallerIdentity.
type IdentityLoadedMsg struct {
	Identity any
}

// IdentityErrorMsg is sent when the caller identity fetch fails.
type IdentityErrorMsg struct {
	Err string
}

// EnrichDetailMsg signals that the active detail view's resource should be
// enriched with additional data (e.g., policy document fetched on demand).
type EnrichDetailMsg struct {
	ResourceType string
	Resource     resource.Resource
}

// EnrichDetailResultMsg delivers an enriched resource back to the detail view.
// On success, the detail view replaces its resource and rebuilds the field list.
// Generation is stamped by the dispatcher and validated in app.go to discard
// stale results after Ctrl+R or navigation away.
type EnrichDetailResultMsg struct {
	ResourceType string
	ResourceID   string
	EnrichedRes  resource.Resource
	Err          error
	Generation   uint64
}
