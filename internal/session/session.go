// Package session owns the in-memory orchestration state for the active
// profile/region session: fetch-availability queue, Wave 2 enrichment queue,
// per-type caches, and the generation counters that invalidate stale async
// results on profile/region switch or refresh.
//
// Session is embedded as *session.Session into tui.Model. Field promotion
// means access sites like m.ResourceCache, m.ProbeResources, m.RelatedGen
// resolve transparently to the embedded Session.
//
// Rules of ownership:
//
//   - Only session-scoped orchestration state belongs here. UI shell concerns
//     (view stack, header, input mode, theme) stay on the surrounding Model.
//   - Maps that handler paths write into directly (ResourceCache,
//     EnrichmentFindings, EnrichmentRan, EnrichmentTypeGen,
//     EnrichmentTruncatedIDs) MUST be constructed by New().
//     ProbeResources and the availability/enrich queues stay nil until a
//     probe retains its first batch — they are built in place.
//   - Session rotation (profile/region switch) MUST bump every generation and
//     replace/clear the caches, so in-flight messages tagged with old gens are
//     discarded by the handlers' gen guards.
package session

import (
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ResourceCacheEntry stores the state of a previously-viewed resource list.
// Used to restore the list when the user re-enters the same resource type
// from the main menu, avoiding redundant API calls.
type ResourceCacheEntry struct {
	Resources     []resource.Resource
	Pagination    *resource.PaginationMeta
	FilterText    string
	AttentionOnly bool // §7.3: ctrl+z toggle persisted across view re-entry
	SortColIdx    int
	SortAsc       bool
	CursorPos     int
	HScrollOffset int
}

// Session owns the in-memory orchestration state for the active
// profile/region session.
type Session struct {
	// Wave 1 availability scan.
	AvailabilityGen int      // bumped on profile/region switch to cancel stale probes
	AvailQueue      []string // resource short names remaining to probe
	AvailChecked    int      // number probed so far in current gen
	AvailTotal      int      // total types to probe in current gen

	// Wave 2 issue-enrichment dispatch.
	ProbeResources  map[string][]resource.Resource // retained first-page resources from Wave 1
	ProbeTruncated  map[string]bool                // per-type truncation signal from Wave 1 probe
	EnrichQueue     []string                       // resource types pending Wave 2 enrichment
	EnrichmentGen   int                            // session-wide gen counter for Wave 2
	EnrichChecked   int                            // number of enrichment probes completed in current gen
	EnrichTotal     int                            // total enrichment probes to run in current gen

	// Per-type Wave 2 finding state (feature 018-enrichment-visibility).
	EnrichmentFindings     map[string]map[string]resource.EnrichmentFinding
	EnrichmentRan          map[string]bool
	EnrichmentTypeGen      map[string]int
	EnrichmentTruncatedIDs map[string]map[string]bool

	// Session-scoped caches + stale-result guards.
	ResourceCache map[string]*ResourceCacheEntry
	// LazyResourceCache holds resources pulled via FetchByIDs for filtered-target
	// drills. Consulted by related-navigation only; NEVER by top-level list
	// navigation. Ensures lazy-added out-of-scope entries (e.g. AWS-managed KMS
	// keys) do not pollute the scope-filtered main-menu list.
	LazyResourceCache map[string][]resource.Resource
	RelatedCache      *RelatedCacheLRU
	RelatedGen        uint64 // bumped on refresh/profile/region switch
	EnrichGen         uint64 // bumped on refresh/profile/region switch (detail-enrichment only)
	EnrichResKey      string // "resourceType:resourceID" of last detail-enrichment dispatch

	// Feature-specific session caches. These used to hang off *ServiceClients
	// but that blurred the AWS-transport/session-state boundary; they live
	// here instead and are passed to detail enrichers via DetailEnrichmentCtx.
	PolicyDocCache *awsclient.PolicyDocumentCache
}

// New constructs a fresh Session with all maps initialized and generation
// counters seeded at 1. The seed=1 convention makes Generation=0 (unset)
// always stale, so synthetic test messages or early-return paths that leave
// Gen at its zero value are rejected by the gen guards.
func New() *Session {
	return &Session{
		ProbeResources:         nil, // initialized lazily on first probe retention
		EnrichmentFindings:     make(map[string]map[string]resource.EnrichmentFinding),
		EnrichmentRan:          make(map[string]bool),
		EnrichmentTypeGen:      make(map[string]int),
		EnrichmentTruncatedIDs: make(map[string]map[string]bool),
		ResourceCache:          make(map[string]*ResourceCacheEntry),
		LazyResourceCache:      make(map[string][]resource.Resource),
		RelatedCache:           NewRelatedCacheLRU(MaxRelatedCacheEntries),
		RelatedGen:             1,
		EnrichGen:              1,
		EnrichmentGen:          1,
		PolicyDocCache:         &awsclient.PolicyDocumentCache{},
	}
}

// Rotate rotates the session when the user switches profile or region. Every
// generation counter is bumped so that in-flight async messages tagged with
// the pre-switch gens are rejected by the handlers' gen guards; all cached
// rows, findings, and queues are cleared so the next session wires up on a
// clean slate.
//
// Callers (handleProfileSelected / handleRegionSelected) retain responsibility
// for UI shell state (header flash, view stack pop, menu availability reset)
// — this method touches only Session-owned fields.
func (s *Session) Rotate() {
	s.RelatedCache.Clear()
	s.RelatedGen++
	s.EnrichGen++
	s.AvailabilityGen++
	s.EnrichmentGen++

	s.EnrichQueue = nil
	s.ProbeResources = nil
	s.ProbeTruncated = nil
	s.AvailQueue = nil
	s.AvailChecked = 0
	s.AvailTotal = 0
	s.EnrichChecked = 0
	s.EnrichTotal = 0
	s.ResourceCache = make(map[string]*ResourceCacheEntry)
	s.LazyResourceCache = make(map[string][]resource.Resource)
	s.EnrichmentFindings = make(map[string]map[string]resource.EnrichmentFinding)
	s.EnrichmentRan = make(map[string]bool)
	s.EnrichmentTypeGen = make(map[string]int)
	s.EnrichmentTruncatedIDs = make(map[string]map[string]bool)

	// Feature caches: swap the PolicyDocumentCache for a fresh instance so
	// documents fetched in the previous account cannot leak into the next.
	s.PolicyDocCache = &awsclient.PolicyDocumentCache{}

	// Process-wide lazy-add caches in internal/aws. These cache AWS names
	// across drills within a session; they must be reset on session switch
	// so stale entries from the prior account cannot satisfy FetchByIDs
	// calls in the next one.
	awsclient.ResetIAMPoliciesCache()
}
