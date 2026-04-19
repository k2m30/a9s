package tui

import (
	"github.com/k2m30/a9s/v3/internal/resource"
)

// sessionRuntime owns the in-memory orchestration state for the active
// profile/region session: fetch-availability queue, Wave 2 enrichment queue,
// per-type caches, and the generation counters that invalidate stale async
// results on profile/region switch or refresh.
//
// It is embedded into the root tui.Model by value. Field promotion means all
// existing access sites (e.g. m.resourceCache, m.probeResources, m.relatedGen)
// continue to work unchanged; the value split exists to make ownership
// explicit: everything on sessionRuntime is session-scoped data that
// handleProfileSelected / handleRegionSelected MUST invalidate before wiring
// the next session.
//
// Rules of ownership:
//
//   - Only session-scoped orchestration state belongs here. UI shell concerns
//     (view stack, header, input mode, theme) stay on the surrounding Model.
//   - Maps and slices must be constructed by newSessionRuntime — zero-value
//     maps are not acceptable for handler paths that write into them.
//   - Session rotation (profile/region switch) MUST bump every generation and
//     replace/clear the caches, so in-flight messages tagged with old gens are
//     discarded by the handlers' gen guards.
type sessionRuntime struct {
	// Wave 1 availability scan.
	availabilityGen int      // bumped on profile/region switch to cancel stale probes
	availQueue      []string // resource short names remaining to probe
	availChecked    int      // number probed so far in current gen
	availTotal      int      // total types to probe in current gen

	// Wave 2 issue-enrichment dispatch.
	probeResources map[string][]resource.Resource // retained first-page resources from Wave 1
	enrichQueue    []string                       // resource types pending Wave 2 enrichment
	enrichmentGen  int                            // session-wide gen counter for Wave 2
	enrichChecked  int                            // number of enrichment probes completed in current gen
	enrichTotal    int                            // total enrichment probes to run in current gen

	// Per-type Wave 2 finding state (feature 018-enrichment-visibility).
	enrichmentFindings     map[string]map[string]resource.EnrichmentFinding
	enrichmentRan          map[string]bool
	enrichmentTypeGen      map[string]int
	enrichmentTruncatedIDs map[string]map[string]bool

	// Session-scoped caches + stale-result guards.
	resourceCache map[string]*resourceCacheEntry
	relatedCache  *relatedCacheLRU
	relatedGen    uint64 // bumped on refresh/profile/region switch
	enrichGen     uint64 // bumped on refresh/profile/region switch (detail-enrichment only)
	enrichResKey  string // "resourceType:resourceID" of last detail-enrichment dispatch
}

// newSessionRuntime constructs a fresh sessionRuntime with all maps initialized
// and generation counters seeded at 1. The seed=1 convention makes Generation=0
// (unset) always stale, so synthetic test messages or early-return paths that
// leave Gen at its zero value are rejected by the gen guards.
func newSessionRuntime() sessionRuntime {
	return sessionRuntime{
		probeResources:         nil, // initialized lazily on first probe retention
		enrichmentFindings:     make(map[string]map[string]resource.EnrichmentFinding),
		enrichmentRan:          make(map[string]bool),
		enrichmentTypeGen:      make(map[string]int),
		enrichmentTruncatedIDs: make(map[string]map[string]bool),
		resourceCache:          make(map[string]*resourceCacheEntry),
		relatedCache:           newRelatedCacheLRU(maxRelatedCacheEntries),
		relatedGen:             1,
		enrichGen:              1,
	}
}

// resetForSessionSwitch rotates the session runtime when the user switches
// profile or region. Every generation counter is bumped so that in-flight
// async messages tagged with the pre-switch gens are rejected by the
// handlers' gen guards; all cached rows, findings, and queues are cleared so
// the next session wires up on a clean slate.
//
// Callers (handleProfileSelected / handleRegionSelected) retain responsibility
// for UI shell state (header flash, view stack pop, menu availability reset)
// — this method touches only sessionRuntime-owned fields.
func (r *sessionRuntime) resetForSessionSwitch() {
	r.relatedCache.clear()
	r.relatedGen++
	r.enrichGen++
	r.availabilityGen++
	r.enrichmentGen++

	r.enrichQueue = nil
	r.probeResources = nil
	r.availQueue = nil
	r.availChecked = 0
	r.availTotal = 0
	r.enrichChecked = 0
	r.enrichTotal = 0
	r.resourceCache = make(map[string]*resourceCacheEntry)
	r.enrichmentFindings = make(map[string]map[string]resource.EnrichmentFinding)
	r.enrichmentRan = make(map[string]bool)
	r.enrichmentTypeGen = make(map[string]int)
	r.enrichmentTruncatedIDs = make(map[string]map[string]bool)
}
