// accessors.go — PR-05a-h4-c (AS-963) typed Core accessors.
//
// Exports a typed read/write surface on *Core that lifts every session-state
// read or mutation the renderer used to do via `m.core.Session().<Field>`.
// After h4-c the renderer never reaches through Core into Session — the
// accessors here, the ServiceClients alias in transport.go, and the
// related-cache helpers in relatedcache.go are the entire renderer-facing
// surface that replaces direct session-shape coupling.
//
// The accessor list covers the field set the renderer actually touches
// (see docs/refactor/05-pr-05a-h4.md §2 and CodeReviewer's AS-963 verdict).
// Convenience constructors that internalise session.New() also live here so
// the renderer's Model construction path does not need to import
// internal/session.
package runtime

import (
	"maps"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
)

// Bootstrap constructs a fresh *Core seeded with a new session.Session
// configured for the given profile/region pair. Used by the renderer's
// model constructor (tui.New) so it can build the Core without importing
// internal/session.
func Bootstrap(profile, region string, types []catalog.ResourceTypeDef) *Core {
	s := session.New()
	s.Profile = profile
	s.Region = region
	return New(s, types)
}

// CurrentGenFor implements messages.GenSource by delegating to the
// session. Lets `messages.IsStale(ev, m.core)` work without the renderer
// reaching through Core into Session.
func (c *Core) CurrentGenFor(a messages.Aspect) domain.Gen {
	return c.session.CurrentGenFor(a)
}

// Profile returns the active session profile.
func (c *Core) Profile() string { return c.session.Profile }

// SetProfile sets the active session profile. Used by the WithProfile
// constructor option only.
func (c *Core) SetProfile(p string) { c.session.Profile = p }

// Region returns the active session region.
func (c *Core) Region() string { return c.session.Region }

// SetRegion sets the active session region. Used by the WithRegion
// constructor option only.
func (c *Core) SetRegion(r string) { c.session.Region = r }

// NoCache reports whether the --no-cache / --demo CLI flags disabled
// on-disk availability caching and background probes.
func (c *Core) NoCache() bool { return c.session.NoCache }

// SetNoCache sets the NoCache policy flag. Constructor-option only.
func (c *Core) SetNoCache(v bool) { c.session.NoCache = v }

// Command returns the one-shot resource short name to navigate to on the
// first ClientsReady (from the -c CLI flag).
func (c *Core) Command() string { return c.session.Command }

// SetCommand sets the one-shot -c CLI flag command. Constructor-option only.
func (c *Core) SetCommand(s string) { c.session.Command = s }

// ClearCommand clears the one-shot -c CLI flag command after the first
// ClientsReady has consumed it.
func (c *Core) ClearCommand() { c.session.Command = "" }

// Clients returns the active session-scoped AWS transport (set by
// HandleClientsReady). The return type is the runtime-exported alias so
// renderer adapters need not import internal/aws.
func (c *Core) Clients() *ServiceClients { return c.session.Clients }

// PreSuppliedClients returns the bootstrap-channel transport supplied by
// tests or demo mode via WithClients.
func (c *Core) PreSuppliedClients() *ServiceClients { return c.session.PreSuppliedClients }

// SetPreSuppliedClients sets the bootstrap-channel transport. Used by the
// WithClients constructor option only.
func (c *Core) SetPreSuppliedClients(s *ServiceClients) { c.session.PreSuppliedClients = s }

// ConnectGen returns the staleness counter for AWS connect attempts.
func (c *Core) ConnectGen() domain.Gen { return c.session.ConnectGen }

// AvailabilityGen returns the Wave-1 availability staleness counter.
func (c *Core) AvailabilityGen() domain.Gen { return c.session.AvailabilityGen }

// BumpAvailabilityGen increments the Wave-1 availability counter so
// in-flight stale probes are rejected by the gen guard.
func (c *Core) BumpAvailabilityGen() { c.session.AvailabilityGen++ }

// EnrichmentGen returns the session-wide Wave-2 enrichment counter.
func (c *Core) EnrichmentGen() domain.Gen { return c.session.EnrichmentGen }

// BumpEnrichmentGen increments the Wave-2 enrichment counter.
func (c *Core) BumpEnrichmentGen() { c.session.EnrichmentGen++ }

// RelatedGen returns the related-cache staleness counter.
func (c *Core) RelatedGen() domain.Gen { return c.session.RelatedGen }

// BumpRelatedGen increments the related-cache counter so in-flight
// related-check results from the prior batch are discarded.
func (c *Core) BumpRelatedGen() { c.session.RelatedGen++ }

// EnrichGen returns the detail-enrichment staleness counter.
func (c *Core) EnrichGen() domain.Gen { return c.session.EnrichGen }

// BumpEnrichGen increments the detail-enrichment counter.
func (c *Core) BumpEnrichGen() { c.session.EnrichGen++ }

// EnrichResKey returns the "resourceType:resourceID" of the last
// detail-enrichment dispatch.
func (c *Core) EnrichResKey() string { return c.session.EnrichResKey }

// ClearEnrichResKey clears the last-dispatched detail-enrichment key so the
// next enrichment dispatch is forced to bump.
func (c *Core) ClearEnrichResKey() { c.session.EnrichResKey = "" }

// EnrichmentTypeGen returns the per-type Wave-2 enrichment counter for the
// given resource short name. Zero when no enrichment has run yet for the
// type.
func (c *Core) EnrichmentTypeGen(rt string) domain.Gen { return c.session.EnrichmentTypeGen[rt] }

// BumpEnrichmentTypeGen increments the per-type Wave-2 counter and returns
// the new value. Used by the refresh path to invalidate the prior batch's
// per-type findings before re-dispatching.
func (c *Core) BumpEnrichmentTypeGen(rt string) domain.Gen {
	c.session.EnrichmentTypeGen[rt]++
	return c.session.EnrichmentTypeGen[rt]
}

// DeleteEnrichmentRan clears the per-type enrichment-ran latch so the next
// enrichment dispatch for the type re-runs from scratch.
func (c *Core) DeleteEnrichmentRan(rt string) { delete(c.session.EnrichmentRan, rt) }

// EnrichmentTruncatedIDs returns the truncated-ID set for the given
// resource type, or nil when no enrichment has retained truncation data
// for the type.
func (c *Core) EnrichmentTruncatedIDs(rt string) map[string]bool {
	return c.session.EnrichmentTruncatedIDs[rt]
}

// DeleteEnrichmentTruncatedIDs clears the per-type truncated-ID set.
func (c *Core) DeleteEnrichmentTruncatedIDs(rt string) {
	delete(c.session.EnrichmentTruncatedIDs, rt)
}

// ResetEnrichmentMaps clears the per-type enrichment latches and counters
// in one shot. Used by the global refresh path (Ctrl+R from main menu)
// where every type must re-enrich from scratch.
func (c *Core) ResetEnrichmentMaps() {
	c.session.EnrichmentRan = make(map[string]bool)
	c.session.EnrichmentTypeGen = make(map[string]domain.Gen)
	c.session.EnrichmentTruncatedIDs = make(map[string]map[string]bool)
}

// ResetProbeMaps clears the Wave-1 retained-probe maps. Used by the global
// refresh path so the next probe round populates fresh.
func (c *Core) ResetProbeMaps() {
	c.session.ProbeResources = make(map[string][]resource.Resource)
	c.session.ProbeTruncated = make(map[string]bool)
}

// SetIdentityFetching sets the session-wide IdentityFetching latch.
func (c *Core) SetIdentityFetching(v bool) { c.session.IdentityFetching = v }

// IdentityFetching reports whether a fetch-identity task is in-flight.
func (c *Core) IdentityFetching() bool { return c.session.IdentityFetching }

// Identity returns the renderer-shaped domain mirror of the session's
// caller identity, or nil if no identity has been resolved yet. The
// awsclient-typed pointer stays inside Core; the adapter renders only
// the domain fields exposed here.
func (c *Core) Identity() *domain.CallerIdentity {
	if c.session.Identity == nil {
		return nil
	}
	return domainCallerIdentityFrom(c.session.Identity)
}

// ResourceCache returns the cached top-level resource-list entry for the
// given resource short name, or (nil, false) when no entry is cached.
// Renderer adapters use this in place of indexing the session map
// directly. The return type is the list-view cache entry shape, distinct
// from the related-checker snapshot's domain.ResourceCacheEntry.
func (c *Core) ResourceCache(rt string) (*domain.ListViewCacheEntry, bool) {
	e, ok := c.session.ResourceCache[rt]
	return e, ok
}

// SetResourceCache stores the cached top-level resource-list entry for the
// given resource short name. nil entries are stored as-is (callers wishing
// to drop an entry should use DeleteResourceCache).
func (c *Core) SetResourceCache(rt string, e *domain.ListViewCacheEntry) {
	c.session.ResourceCache[rt] = e
}

// DeleteResourceCache drops the cached resource-list entry for the given
// resource short name, so the next list-open re-fetches.
func (c *Core) DeleteResourceCache(rt string) { delete(c.session.ResourceCache, rt) }

// HasResourceCache reports whether a non-nil cached entry exists for the
// given resource short name (without exposing the entry itself or any
// other type). Renderer adapters that only need the existence signal use
// this in place of `m.core.ResourceCache(rt)` to avoid binding to the
// entry shape.
func (c *Core) HasResourceCache(rt string) bool {
	e, ok := c.session.ResourceCache[rt]
	return ok && e != nil
}

// ResourceCacheKeys returns the set of resource short names that currently
// have a cached top-level list entry. Snapshot semantics — the returned
// slice is decoupled from the underlying map.
func (c *Core) ResourceCacheKeys() []string {
	keys := make([]string, 0, len(c.session.ResourceCache))
	for k := range c.session.ResourceCache {
		keys = append(keys, k)
	}
	return keys
}

// ForEachResourceCache invokes fn for every non-nil cached resource-list
// entry. The entry pointer is passed by reference; the callback may mutate
// the entry's slice elements in-place (used by Ctrl+R wave2 cleanup).
func (c *Core) ForEachResourceCache(fn func(rt string, entry *domain.ListViewCacheEntry)) {
	for rt, entry := range c.session.ResourceCache {
		if entry == nil {
			continue
		}
		fn(rt, entry)
	}
}

// LazyResourceCache returns the lazy-cache slice for the given resource
// short name (resources pulled via FetchByIDs for filtered-target
// drills). The bool reports whether any lazy-cache entry exists for
// the type — distinct from a non-nil empty slice.
func (c *Core) LazyResourceCache(rt string) ([]domain.Resource, bool) {
	rows, ok := c.session.LazyResourceCache[rt]
	return rows, ok
}

// ForEachLazyResourceCache invokes fn for every lazy-cache slice. The slice
// is passed by value but the underlying array is shared, so the callback
// may mutate rows[i] fields in-place.
func (c *Core) ForEachLazyResourceCache(fn func(rt string, rows []resource.Resource)) {
	for rt, rows := range c.session.LazyResourceCache {
		fn(rt, rows)
	}
}

// ExtendLazyResourceCache merges the given per-type rows into the lazy
// cache. Used by the PatchLazyResourceCache intent dispatcher in place of
// `maps.Copy(m.core.Session().LazyResourceCache, ...)`.
func (c *Core) ExtendLazyResourceCache(adds map[string][]resource.Resource) {
	maps.Copy(c.session.LazyResourceCache, adds)
}

// ProbeResources returns the Wave-1 retained first-page resources for the
// given resource short name. ok reports whether an entry exists in the
// retained map.
func (c *Core) ProbeResources(rt string) ([]resource.Resource, bool) {
	rows, ok := c.session.ProbeResources[rt]
	return rows, ok
}

// ForEachProbeResources invokes fn for every retained Wave-1 probe slice.
// The slice underlying array is shared, so the callback may mutate rows[i]
// fields in-place.
func (c *Core) ForEachProbeResources(fn func(rt string, rows []resource.Resource)) {
	for rt, rows := range c.session.ProbeResources {
		fn(rt, rows)
	}
}

// RelatedCacheGet returns the cached related-check results for the given
// cache key (built via RelatedCacheKey).
func (c *Core) RelatedCacheGet(key string) ([]RelatedCacheResult, bool) {
	return c.session.RelatedCache.Get(key)
}

// RelatedCacheSet stores the related-check results for the given key.
func (c *Core) RelatedCacheSet(key string, results []RelatedCacheResult) {
	c.session.RelatedCache.Set(key, results)
}

// RelatedCacheDelete drops the cached related-check results for the given
// key so the next related-fanout re-runs the checkers.
func (c *Core) RelatedCacheDelete(key string) { c.session.RelatedCache.Delete(key) }

// HasIssueEnricher reports whether a Wave-2 issue enricher is registered
// for the given resource short name. Renderer adapters use this in place
// of probing awsclient.Wave2EnricherFor / awsclient.IssueEnricherRegistry
// directly.
func (c *Core) HasIssueEnricher(shortName string) bool {
	_, ok := awsclient.Wave2EnricherFor(shortName)
	return ok
}
