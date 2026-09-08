// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package messages

import (
	"errors"
	"time"

	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchProvenance identifies which fetch pipeline produced a ResourcesLoaded
// event: the type's own top-level population, a server-side filtered drill,
// a single by-ID lookup, or a nested child-resource fetch. A consumer that
// needs to know whether a result IS the type's canonical population — the
// shared per-type RowStore, the persisted disk cache, the menu badge sync,
// the C6 FilteredRowsSet seed — calls CanonicalList() instead of inferring
// it from fields a given consumer happens to have access to (ScreenID,
// ListState.EscPops/ParentContext) and that another consumer of the same
// event (runtime.Core, which owns no screen stack) never can.
type FetchProvenance uint8

const (
	// FetchProvenanceUnknown is the zero value — a ResourcesLoaded literal
	// that never set Provenance. CanonicalList() returns false for it: an
	// omitted provenance fails closed rather than being silently read as the
	// type's canonical population.
	FetchProvenanceUnknown FetchProvenance = iota
	// FetchProvenanceCanonicalList is the type's own top-level list fetch, or
	// its load-more continuation — the only provenance eligible to replace
	// the shared per-type RowStore entry or the persisted disk cache.
	FetchProvenanceCanonicalList
	// FetchProvenanceFilteredList is a server-side filtered drill into the
	// same resource type, or its load-more continuation.
	FetchProvenanceFilteredList
	// FetchProvenanceByID is a single-resource by-ID lookup.
	FetchProvenanceByID
	// FetchProvenanceChild is a nested child-resource-type fetch, or its
	// load-more continuation.
	FetchProvenanceChild
)

// CanonicalList reports whether a ResourcesLoaded event carrying this
// provenance is the resource type's canonical top-level population.
func (p FetchProvenance) CanonicalList() bool { return p == FetchProvenanceCanonicalList }

// ProvenanceForContinuation resolves the FetchProvenance for a KindFetchMore
// continuation from the same ParentContext/FetchFilter mutual-exclusivity
// FetchMorePayload documents: a non-empty parentContext continues a child
// list, a non-empty fetchFilter continues a filtered list, neither continues
// the canonical top-level list. Shared by the executor and the TUI adapter's
// own KindFetchMore construction so the two producers of the same
// continuation cannot resolve it differently.
func ProvenanceForContinuation(parentContext, fetchFilter map[string]string) FetchProvenance {
	if len(parentContext) > 0 {
		return FetchProvenanceChild
	}
	if len(fetchFilter) > 0 {
		return FetchProvenanceFilteredList
	}
	return FetchProvenanceCanonicalList
}

// ResourcesLoaded is sent when AWS resources have been fetched.
type ResourcesLoaded struct {
	ResourceType string
	Resources    []resource.Resource
	Pagination   *resource.PaginationMeta // nil when result has no pagination info
	Append       bool                     // true = append to existing list
	// Provenance identifies which fetch pipeline produced this event. Every
	// production construction site sets it explicitly (executor.go,
	// internal/tui/fetch_adapter.go) — see FetchProvenance for why the zero
	// value must never be read as canonical.
	Provenance FetchProvenance
	// TypeGen is the enrichment-rerun token. 0 on normal fetches (no rerun
	// intent). Non-zero only when the message originates from the
	// Ctrl+R-for-rerun wrapped fetch: it carries the per-type enrichment
	// generation captured at dispatch time. The handler applies the list
	// update unconditionally, then — after its existing write-through block
	// — checks this field; if it matches the current per-type gen, it seeds
	// probeResources and dispatches probeEnrichment.
	TypeGen domain.Gen
	// Gen is the session AvailabilityGen captured at dispatch time. A
	// ResourcesLoaded whose Gen no longer matches the current session gen is
	// silently discarded (profile/region switch happened between dispatch and
	// delivery). AvailabilityGen is seeded at 1 and every production dispatch
	// site stamps the live value, so Gen==0 only ever originates from a
	// synthetic/unstamped construction (never a real fetch) — AcceptZeroGen=true
	// so those messages still pass the guard rather than requiring every such
	// caller to round-trip a live session gen.
	Gen domain.Gen
	// ListSeq is the per-type canonical-list fetch sequence the dispatch that
	// produced this result was stamped with (runtime.TaskRequest.ListSeq).
	// The apply point discards a canonical list result whose ListSeq is no
	// longer the latest one handed out for its type — an on-entry
	// verification overtaken by a later Ctrl+R, a load-more, or another
	// refresh. Zero means the result carries no ordering claim (a cache-seed
	// replay, a filtered/child/by-ID fetch, a synthetic construction) and is
	// applied as-is.
	ListSeq domain.Gen
	// Err is non-nil when the paginated fetcher returned a partial-success
	// composite error: SOME resources made it back AND something failed
	// (e.g. one inline-group-policy enumeration call timed out). The handler
	// renders Resources as usual AND routes Err through Flash so the `!`
	// log records the partial failure.
	Err error
}

func (ResourcesLoaded) isEvent()               {}
func (m ResourcesLoaded) GenStamp() domain.Gen { return m.Gen }
func (ResourcesLoaded) GenAspect() Aspect      { return AspectAvailability }
func (ResourcesLoaded) AcceptZeroGen() bool    { return true }

// APIError is sent when an AWS API call fails.
type APIError struct {
	ResourceType string
	Err          error
	// Gen is the session AvailabilityGen captured at dispatch time. A stale
	// APIError (from a prior profile/region) is silently discarded.
	// AvailabilityGen is seeded at 1 and every production dispatch site
	// stamps the live value, so Gen==0 only ever originates from a
	// synthetic/unstamped construction — AcceptZeroGen=true so those messages
	// still pass the guard.
	Gen domain.Gen
	// Append mirrors ResourcesLoaded.Append's meaning applied to the failure
	// path: true when this error is the outcome of a KindFetchMore
	// continuation, false for every other fetch kind (initial load, refresh,
	// filtered, child, by-ID). LoadingMore and Refreshing are allowed to be
	// simultaneously in flight on the same screen (a Ctrl+R issued while a
	// load-more continuation is still outstanding), so the failure handler
	// must know which of the two in-flight requests this is, to clear only
	// that one's indicator rather than stranding or prematurely releasing
	// the sibling request's flag (core/app/list_state.go's
	// clearFetchInFlight).
	Append bool
	// Provenance mirrors ResourcesLoaded.Provenance's meaning applied to the
	// failure path: every production construction site pairs an APIError
	// branch with a ResourcesLoaded success branch and stamps this field with
	// the exact same value the paired success would have carried (executor.go,
	// internal/tui/fetch_adapter.go). This lets a failure be routed to the
	// SAME screen its paired success would have landed on — the scan
	// handleResourcesLoadedEvent already performs by ResourceType +
	// CanonicalList() agreement (core/app/handle.go) — instead of blindly
	// applying to whatever screen happens to be topmost, which would let a
	// navigate-away-before-failure race mark an unrelated screen with this
	// request's error. The zero value (FetchProvenanceUnknown) means the
	// caller predates this contract (e.g. a hand-built APIError with no paired
	// fetch, or the ClientsReady-wrong-client-type path in
	// internal/tui/runtime_adapter.go's emitAPIErrorCmd, which carries no
	// ResourceType either) — ClearActiveListLoadingIntent's consumer falls
	// back to the pre-existing "active list screen" behavior for those,
	// exactly like FetchResourcesPayload.Provenance's own zero-value grace.
	Provenance FetchProvenance
}

func (APIError) isEvent()               {}
func (m APIError) GenStamp() domain.Gen { return m.Gen }
func (APIError) GenAspect() Aspect      { return AspectAvailability }
func (APIError) AcceptZeroGen() bool    { return true }

// Flash sets a transient message in the header right side.
type Flash struct {
	Text    string
	IsError bool
}

func (Flash) isEvent() {}

// ByIDFetchFailed is the typed outcome of a by-ID resource fetch (the TUI's
// fetchByIDDetail adapter) that found nothing or errored — TargetType/ID
// name exactly which fetch failed, so a consumer holding a placeholder for
// that same (TargetType, ID) can act on it unambiguously, never by sniffing
// an unrelated error Flash that happens to arrive while a placeholder is on
// screen (S3: the msg.IsError + GetListAutoOpenSingle heuristic it replaces
// could not tell "my own fetch failed" from "something else failed").
type ByIDFetchFailed struct {
	TargetType string
	ID         string
	Reason     string
}

func (ByIDFetchFailed) isEvent() {}

// ClearFlash is sent after the flash auto-clear timer expires.
type ClearFlash struct {
	Gen domain.Gen // only clear if this matches current flash generation
}

func (ClearFlash) isEvent() {}

// ValueRevealed is sent when a resource value has been fetched via reveal (x key).
type ValueRevealed struct {
	ResourceType string // e.g., "secrets", "ssm"
	ResourceID   string // secret name or parameter name
	Value        string
	Err          error
	// Gen is the session ConnectGen captured at dispatch time. A stale
	// ValueRevealed (secret from a prior profile) is silently discarded to
	// prevent cross-account secret display. ConnectGen is seeded at 1 and
	// every production dispatch site stamps the live value, so Gen==0 only
	// ever originates from a synthetic/unstamped construction — AcceptZeroGen=true
	// so those messages still pass the guard.
	Gen domain.Gen
}

func (ValueRevealed) isEvent()               {}
func (m ValueRevealed) GenStamp() domain.Gen { return m.Gen }
func (ValueRevealed) GenAspect() Aspect      { return AspectConnect }
func (ValueRevealed) AcceptZeroGen() bool    { return true }

// ClientsReady is sent when AWS clients are initialized.
// Clients is typed as any to avoid importing aws/ from the messages package.
// The adapter type-asserts it to *awsclient.ServiceClients.
type ClientsReady struct {
	Clients any
	Err     error
	Region  string     // resolved region from AWS config (set on success)
	Gen     domain.Gen // connect generation — ignore if != current connectGen
}

func (ClientsReady) isEvent()               {}
func (m ClientsReady) GenStamp() domain.Gen { return m.Gen }
func (ClientsReady) GenAspect() Aspect      { return AspectConnect }

// AcceptZeroGen is true: ConnectGen is seeded at 1 and every production
// dispatch site (internal/tui/fetch_adapter.go's connectAWS,
// core/runtime/executor.go's TaskKindConnect case, the pre-supplied-clients
// bootstrap in internal/tui/app.go's Init) stamps the live ConnectGen, so
// Gen==0 only ever originates from a synthetic/unstamped construction, never
// a real connect result.
func (ClientsReady) AcceptZeroGen() bool { return true }

// RelatedCheckResult delivers one checker's async result back to the detail view.
// The adapter delegates this to the active view (detail model's rightColumnModel).
type RelatedCheckResult struct {
	ResourceType     string
	SourceResourceID string // ID of the source resource (for cache keying)
	DefDisplayName   string // unique def.DisplayName — disambiguates multiple defs sharing a TargetType (e.g. ct-events self-pivots)
	Result           resource.RelatedCheckResult
	// OperationID is the core/runtime.DetailOperation.ID this result was
	// dispatched under. Accepted only when it matches the session's current
	// DetailOpGen (see messages.AspectDetailOp) — a fresh detail open or an
	// explicit refresh begins a new operation, so a result from any earlier
	// one is discarded regardless of how long it was in flight. DetailOpGen
	// is seeded at 1 and every OperationID is minted by domain.Gen.Bump()
	// (core/runtime.BeginDetailOperation), which can never return 0 — so a
	// zero OperationID only ever originates from a synthetic/unstamped
	// construction, never a real detail operation.
	OperationID domain.Gen
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

func (RelatedCheckResult) isEvent()               {}
func (m RelatedCheckResult) GenStamp() domain.Gen { return m.OperationID }
func (RelatedCheckResult) GenAspect() Aspect      { return AspectDetailOp }
func (RelatedCheckResult) AcceptZeroGen() bool    { return true }

// RelatedCheckBatch is the headless-executor counterpart to the per-def
// RelatedCheckResult messages the TUI fan-out emits. The executor runs
// checkers concurrently (bounded by runtime.MaxConcurrentProbes) and bundles
// all per-def results into one event so DrainSync can route them through
// Controller.Handle in a single call. OperationID mirrors the
// DetailOperation.ID captured at dispatch time — stale batches are dropped
// by the same IsStale guard used for individual RelatedCheckResult messages.
type RelatedCheckBatch struct {
	ResourceType     string
	SourceResourceID string
	Results          []RelatedCheckResult
	OperationID      domain.Gen
}

func (RelatedCheckBatch) isEvent()               {}
func (m RelatedCheckBatch) GenStamp() domain.Gen { return m.OperationID }
func (RelatedCheckBatch) GenAspect() Aspect      { return AspectDetailOp }
func (RelatedCheckBatch) AcceptZeroGen() bool    { return true }

// AvailabilityCacheLoaded delivers cached availability data loaded from disk.
// Entries maps resource short names to resource counts.
// Only entries with a successful check (no error) are included.
type AvailabilityCacheLoaded struct {
	Entries        map[string]int  // shortName -> resource count
	Truncated      map[string]bool // shortName -> true if truncated
	IssueCounts    map[string]int  // shortName -> cached issue count
	IssueTruncated map[string]bool // shortName -> true if issue count was truncated
	IssueKnown     map[string]bool // shortName -> true if issue count was probed (vs unknown)
	// Profile and Region name the pair this load describes, taken from the
	// Store it was read from. The load is dispatched asynchronously and can
	// be delivered after the operator has switched pairs; the handler drops
	// it rather than painting one account's counts onto another's menu (C9).
	// Empty on a synthetic construction that names no pair — such an event
	// carries no claim about which pair it belongs to and is applied as-is.
	Profile string
	Region  string
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
	// PrefetchErr is the composite error aggregating HARD per-type fetch
	// failures (the type yielded no rows) during the synchronous availability
	// prefetch. Non-nil when any paginated fetcher errored row-less; the app
	// handler surfaces it as a Flash so operators see permission/throttle
	// issues rather than silently missing types.
	PrefetchErr error
	// PrefetchSoftErr aggregates PARTIAL per-type failures (rows arrived
	// alongside a composite per-item error — the E5 contract). Recorded in
	// the `!` error log only, never as a blocking banner: the rows already
	// carry their degraded-state findings on screen.
	PrefetchSoftErr error
}

func (AvailabilityPrefetched) isEvent()               {}
func (m AvailabilityPrefetched) GenStamp() domain.Gen { return m.Gen }
func (AvailabilityPrefetched) GenAspect() Aspect      { return AspectAvailability }

// AcceptZeroGen returns false so an in-flight AvailabilityPrefetched stamped
// with the pre-rotation session counter (e.g. captured before AvailabilityGen
// was bumped) cannot bypass the staleness guard once Rotate() has advanced
// AvailabilityGen past it. Session.New() seeds AvailabilityGen=1 so the
// legitimate first prefetch on a fresh session is still applied; the
// hazard is a zero-stamped message arriving after Rotate(), which the
// stale-drop guard must reject. Mirrors AvailabilityChecked.
func (AvailabilityPrefetched) AcceptZeroGen() bool { return false }

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
	// Duration is the wall time ExecuteTaskAt spent inside
	// ProbeResourceAvailability for this probe (runtime.ProbeStatus.Duration, #462).
	Duration time.Duration
}

func (AvailabilityChecked) isEvent()               {}
func (m AvailabilityChecked) GenStamp() domain.Gen { return m.Gen }
func (AvailabilityChecked) GenAspect() Aspect      { return AspectAvailability }
func (AvailabilityChecked) AcceptZeroGen() bool    { return false } // AvailabilityGen is seeded at 1; zero stamp is always stale

// EnrichmentChecked reports one resource type's Wave 2 enrichment result.
type EnrichmentChecked struct {
	ResourceType string
	Truncated    bool // whether the enrichment count is a lower bound
	// Findings carries every independently-evaluated Wave-2 Finding per
	// Resource.ID (IssueEnricherResult.Findings, unfiltered), keyed by
	// resource.Resource.ID. Populated on success AND on partial-success (Err
	// non-nil but partial results present). May include findings for resources
	// off-page (account-wide enrichers). A resource with more than one
	// independently-evaluated condition carries every one of them — the
	// row-level fold (applyEnrichment/ApplyWave2ToRow) uses this so a
	// multi-condition resource retains every Finding on its cached row,
	// unifiedIssueCount uses this to count a resource once if ANY entry is
	// SevBroken, and PatchDetail.EnrichmentFindings is built from this so
	// every independently-evaluated finding reaches an already-open detail.
	// A render-boundary consumer that needs a single representative (a row's
	// one-character glyph) reduces via domain.WorstSeverityFinding — never
	// stored pre-reduced here.
	Findings map[string][]domain.Finding
	// AttentionDetails carries the supporting rows for each per-resource
	// Finding, keyed by Resource.ID then by the owning Finding's Code
	// (IssueEnricherResult.AttentionDetails, unfiltered) — every
	// independently-evaluated condition on a resource keeps its own rows. The
	// row-level fold (runtime.Core.applyEnrichment) reads it directly against
	// the matching r.Findings entry when writing onto cached rows.
	AttentionDetails map[string]map[domain.FindingCode]domain.AttentionDetail
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
	// Duration is the wall time ExecuteTaskAt spent inside ProbeEnrichment for
	// this probe (runtime.ProbeStatus.Duration, #462) — summed onto the
	// type's availability-probe duration, not tracked separately.
	Duration time.Duration
}

func (EnrichmentChecked) isEvent()               {}
func (m EnrichmentChecked) GenStamp() domain.Gen { return m.Gen }
func (EnrichmentChecked) GenAspect() Aspect      { return AspectEnrichment }
func (EnrichmentChecked) AcceptZeroGen() bool    { return true }

// IdentityLoaded is sent when the caller identity has been fetched.
// Identity is typed as any to avoid importing aws/ from the messages package.
// The adapter type-asserts it to *awsclient.CallerIdentity.
type IdentityLoaded struct {
	Identity any
	// Gen is the session ConnectGen captured at dispatch time. A stale
	// IdentityLoaded (account ID from a prior profile) is silently discarded
	// to prevent stale identity from appearing in the header after a switch.
	// ConnectGen is seeded at 1 and every production dispatch site stamps the
	// live value, so Gen==0 only ever originates from a synthetic/unstamped
	// construction — AcceptZeroGen=true so those messages still pass the guard.
	Gen domain.Gen
}

func (IdentityLoaded) isEvent()               {}
func (m IdentityLoaded) GenStamp() domain.Gen { return m.Gen }
func (IdentityLoaded) GenAspect() Aspect      { return AspectConnect }
func (IdentityLoaded) AcceptZeroGen() bool    { return true }

// IdentityError is sent when the caller identity fetch fails.
type IdentityError struct {
	Err string
	// Gen is the session ConnectGen captured at dispatch time. A stale
	// IdentityError (from a prior profile's fetch) is silently discarded to
	// avoid clearing IdentityFetching for the new session's in-flight fetch.
	// ConnectGen is seeded at 1 and every production dispatch site stamps the
	// live value, so Gen==0 only ever originates from a synthetic/unstamped
	// construction — AcceptZeroGen=true so those messages still pass the guard.
	Gen domain.Gen
}

func (IdentityError) isEvent()               {}
func (m IdentityError) GenStamp() domain.Gen { return m.Gen }
func (IdentityError) GenAspect() Aspect      { return AspectConnect }
func (IdentityError) AcceptZeroGen() bool    { return true }

// EnrichDetailResult delivers an enriched resource back to the detail view.
// On success, the detail view replaces its resource and rebuilds the field
// list. OperationID is the core/runtime.DetailOperation.ID stamped by the
// dispatcher; discarded when it no longer matches the session's active
// operation (a Ctrl+R refresh or navigating to a different resource begins a
// new operation). DetailOpGen is seeded at 1 and every OperationID is minted
// by domain.Gen.Bump() (never 0 in production), so a zero OperationID only
// ever originates from a synthetic/unstamped construction.
type EnrichDetailResult struct {
	ResourceType string
	ResourceID   string
	EnrichedRes  resource.Resource
	Err          error
	OperationID  domain.Gen
}

func (EnrichDetailResult) isEvent()               {}
func (m EnrichDetailResult) GenStamp() domain.Gen { return m.OperationID }
func (EnrichDetailResult) GenAspect() Aspect      { return AspectDetailOp }
func (EnrichDetailResult) AcceptZeroGen() bool    { return true }

// CostsLoaded delivers one Cost Explorer fetch result: the query shape that
// was fetched, the mapped grid/attrs/anomalies, and the request count for
// the session $-counter (FR-013 — counted even when Err is set).
type CostsLoaded struct {
	Query costs.Query
	// Grid carries the grid (GetCostAndUsage[WithResources]) fetch's own
	// outcome — Fetched/Records/Err — the ONE home for grid data on this
	// event; ApplyCostsLoaded reads Grid.Fetched/Grid.Records exclusively,
	// never a bare top-level Records field whose nil-ness alone cannot
	// distinguish "skipped" from "CE confirmed zero groups" (symmetric with
	// Anomalies below via AnomalyResult's own Requested convention).
	Grid  costs.GridResult
	Attrs map[string]string
	// Window is the exact visible-window periods Query.Range was built to
	// cover (FetchCostsPayload.Window, threaded through unchanged) —
	// ApplyCostsLoaded stamps Store.MergeCoverage against this, not against
	// whichever periods Grid.Records happens to mention, so a period CE
	// genuinely returned zero groups for (R2) is remembered as covered.
	Window    []costs.Period
	Anomalies []costs.AnomalyMark
	// AnomaliesTruncated mirrors Grid.Truncated for the anomaly overlay: true
	// when awsclient.FetchCostAnomalies' page cap fired before GetAnomalies'
	// own NextPageToken exhaustion, so Anomalies is a lower bound rather than
	// CE's own authoritative complete list for the window — the same signal
	// AnomaliesFetchResult.Truncated carries, threaded through so
	// ApplyCostsLoaded/costsAnomalyResultFromEvent can avoid caching a
	// page-capped result as an authoritative 24h-TTL snapshot.
	AnomaliesTruncated bool
	Requests           int
	Err                error
	// Gen is the session ConnectGen captured at dispatch time. A stale
	// CostsLoaded (a fetch dispatched under a prior profile/region) is
	// dropped by the same IsStale guard every other ConnectGen-stamped
	// event uses — CostsState is per-screen but the Store it merges into
	// is loaded per-profile, so an in-flight fetch surviving a profile
	// switch must never merge into the new profile's cache. ConnectGen is
	// seeded at 1 and every production dispatch site stamps the live value,
	// so Gen==0 only ever originates from a synthetic/unstamped construction
	// — AcceptZeroGen=true so those messages still pass the guard.
	Gen domain.Gen
}

func (CostsLoaded) isEvent()               {}
func (m CostsLoaded) GenStamp() domain.Gen { return m.Gen }
func (CostsLoaded) GenAspect() Aspect      { return AspectConnect }
func (CostsLoaded) AcceptZeroGen() bool    { return true }

// ThemeFileRead delivers the bytes of a theme YAML file read from disk
// in response to a TaskKindReadThemeFile dispatch. Theme is the theme
// filename the user selected; Bytes is the raw YAML payload (nil on
// read error); Err is non-nil when the read failed. This event lets the
// runtime split the theme-selected flow into a Core-side handler
// (HandleThemeSelected, emits the read task) and a second Core-side handler
// (HandleThemeFileRead, emits Apply/Pop/Flash + Save task) without
// performing file I/O inside Core.
type ThemeFileRead struct {
	Theme string
	Bytes []byte
	Err   error
}

func (ThemeFileRead) isEvent() {}

// AllEventSamples returns exactly one minimal-but-valid instance of every
// concrete type implementing Event, in this file's declaration order — the
// enumerable registry the cross-renderer routing contract test iterates.
// ADDING A NEW EVENT TYPE WITHOUT A SAMPLE HERE MUST FAIL THE CONTRACT
// TEST's count check, so keep this adjacent to the type definitions above.
//
// Fields are left at their zero value except where a non-zero value is
// needed to route safely through Controller.Handle without a nil-deref or
// panic (each such field is commented at its literal below) or to exercise
// the type's real branch instead of a graceful-degrade no-op (e.g. a
// registered ResourceType instead of an unregistered empty string). Target:
// zero exclusions — every concrete Event type below has a sample.
func AllEventSamples() []Event {
	return []Event{
		ResourcesLoaded{ResourceType: "ec2"},
		// Err must be non-nil: HandleAPIError's routing path
		// (core/runtime/handlers.go) unconditionally calls ev.Err.Error() in
		// its non-classified branch — a nil Err panics.
		APIError{ResourceType: "ec2", Err: errors.New("sample api error")},
		Flash{Text: "sample"},
		ByIDFetchFailed{TargetType: "ec2", ID: "i-sample", Reason: "sample reason"},
		ClearFlash{},
		ValueRevealed{ResourceType: "secrets", ResourceID: "sample"},
		ClientsReady{Region: "us-east-1"},
		RelatedCheckResult{ResourceType: "ec2", SourceResourceID: "i-sample", DefDisplayName: "sample"},
		RelatedCheckBatch{ResourceType: "ec2", SourceResourceID: "i-sample"},
		AvailabilityCacheLoaded{},
		AvailabilityPrefetched{},
		AvailabilityChecked{ResourceType: "ec2"},
		EnrichmentChecked{ResourceType: "ec2"},
		IdentityLoaded{},
		IdentityError{Err: "sample"},
		// EnrichedRes carries a Type/ID so a routing path keying off the
		// resource (cache keys, matching a stacked detail screen) exercises
		// its real branch instead of an empty-string no-op.
		EnrichDetailResult{ResourceType: "ec2", ResourceID: "i-sample", EnrichedRes: resource.Resource{Type: "ec2", ID: "i-sample"}},
		CostsLoaded{},
		ThemeFileRead{Theme: "sample"},
	}
}
