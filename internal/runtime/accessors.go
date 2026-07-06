// accessors.go — typed Core accessors.
//
// Exports a typed read/write surface on *Core for every session-state read or
// mutation, so the renderer never reaches through Core into Session — the
// accessors here, the ServiceClients alias in transport.go, and the
// related-cache helpers in relatedcache.go are the entire renderer-facing
// surface that replaces direct session-shape coupling.
//
// The accessor list covers the field set the renderer actually touches.
// Convenience constructors that internalise session.New() also live here so
// the renderer's Model construction path does not need to import
// internal/session.
package runtime

import (
	"maps"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/cache"
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

// CacheStore returns the loaded per-type disk cache for the CURRENT
// Profile+Region pair, or nil when no LoadDir has run yet for this pair
// (cold start before TaskKindLoadAvailCache completes, or --no-cache). Goes
// through EnsureCacheStore so this getter never returns a store memoized for
// a stale pair (see Session.EnsureCacheStore).
func (c *Core) CacheStore() *cache.Store { return c.EnsureCacheStore() }

// EnsureCacheStore returns the current pair's *cache.Store, reloading via
// cache.LoadDir(profile, region) whenever the memoized store (if any) was
// not loaded for the CURRENT session.Profile/session.Region pair — this
// covers both the first call since the last Rotate (C9) or process start,
// and a pair switch that lands between two calls without an intervening
// Rotate observation. NoCache=true always returns nil without ever calling
// LoadDir (C7b: --no-cache disables persisted load entirely). session == ""
// Profile or Region (pair not yet resolved) returns nil WITHOUT memoizing,
// so a pre-connect call never pins the store to the wrong "<profile>--"
// directory. All access serializes on session.cacheStoreMu.
func (c *Core) EnsureCacheStore() *cache.Store {
	if c.session.NoCache {
		return nil
	}
	return c.session.EnsureCacheStore(c.session.Profile, c.session.Region)
}

// WithCacheStore runs fn against the current pair's *cache.Store with
// session.cacheStoreMu held for fn's entire duration, so a caller's own
// store.Type/Put/SaveType read-modify-write sequence for one type file can
// never interleave with another such sequence running concurrently (DEF-17).
// No-op (fn not called) when NoCache is set, mirroring EnsureCacheStore.
func (c *Core) WithCacheStore(fn func(store *cache.Store) error) error {
	if c.session.NoCache {
		return nil
	}
	return c.session.WithCacheStore(c.session.Profile, c.session.Region, fn)
}

// ReadCacheStore runs fn against the current pair's *cache.Store with
// session.cacheStoreMu held, for read-only callers (store.Type/store.Types).
// See Session.ReadCacheStore for why a read call site must not bypass this
// lock even though it never mutates the store itself.
func (c *Core) ReadCacheStore(fn func(store *cache.Store) error) error {
	if c.session.NoCache {
		return nil
	}
	return c.session.ReadCacheStore(c.session.Profile, c.session.Region, fn)
}

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

// ResetProbeMaps clears the Wave-1 retained-probe row-store entries so the
// next probe round populates fresh (used by the global refresh / Ctrl+R
// path). Prior to task #17 wave 1 stage 2 this reset session.ProbeResources/
// ProbeTruncated directly; those fields are gone, so this now clears every
// RowStore entry whose Origin is OriginProbe or OriginDisk (the two origins
// a Wave-1 probe/disk-seed populate) while leaving OriginFetch (top-level
// list fetch) rows untouched — a menu-only refresh must not blank an
// already-open resource list's own fetched rows.
func (c *Core) ResetProbeMaps() {
	c.session.RowStore.ClearProbeOrigin()
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
	// Dual-write (task #17 wave 1): each adds[rt] is already the full
	// merged slice HandleRelatedCheckResult computed (dedup-appended against
	// the prior LazyResourceCache entry) — ObservePartial's own dedup-append
	// makes re-merging it against the store idempotent.
	for rt, rows := range adds {
		c.ObservePartialRows(rt, rows)
	}
}

// ProbeOriginTypeNames returns the canonical short names of every type
// currently retaining an OriginProbe/OriginDisk row-carrying entry in
// RowStore (task #17 wave 1 stage 2 — the membership test the removed
// session.ProbeResources map used to provide via range-over-map).
func (c *Core) ProbeOriginTypeNames() []string {
	return c.session.RowStore.ProbeOriginTypeNames()
}

// ProbeResources returns the Wave-1/disk-seed retained rows for the given
// resource short name, read from RowStore (task #17 wave 1 stage 2 —
// replaces the removed session.ProbeResources map). ok reports whether
// RowStore currently holds an OriginProbe/OriginDisk entry with rows for rt;
// the returned slice is a defensive copy (RowStore.Snapshot), so callers
// wishing to mutate row content must go through AmendRows instead of writing
// into the returned slice in place.
func (c *Core) ProbeResources(rt string) ([]resource.Resource, bool) {
	tr := c.session.RowStore.Snapshot(rt)
	if len(tr.Rows) == 0 || (tr.Origin != session.OriginProbe && tr.Origin != session.OriginDisk) {
		return nil, false
	}
	return tr.Rows, true
}

// ForEachProbeResources invokes fn for every retained OriginProbe/OriginDisk
// row set (task #17 wave 1 stage 2 — replaces the removed
// session.ProbeResources map). Each rows slice is a defensive copy
// (RowStore.SnapshotAll); the callback MUST NOT rely on in-place mutation
// propagating back into the store — use AmendRows for that.
func (c *Core) ForEachProbeResources(fn func(rt string, rows []resource.Resource)) {
	for rt, tr := range c.session.RowStore.SnapshotAll(true) {
		if len(tr.Rows) == 0 || (tr.Origin != session.OriginProbe && tr.Origin != session.OriginDisk) {
			continue
		}
		fn(rt, tr.Rows)
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

// ObserveRows is the thin dual-write chokepoint every rows-carrying write to
// ProbeResources/ResourceCache feeds alongside its existing map write (task
// #17 wave 1 — row-store unification, Stage 1: dual-write scaffolding, zero
// behavior change). canon must already be the canonicalized resource short
// name — callers resolve aliases before calling, matching every other
// canon-keyed write in this package. Returns the accepted rows + Gen so a
// future caller can compare it against the legacy map write during the
// Stage-1 differential harness; today's callers only need the side effect
// and may discard the result.
func (c *Core) ObserveRows(canon string, rows []resource.Resource, pagination *resource.PaginationMeta, origin session.Origin, appendPage bool) ([]resource.Resource, domain.Gen) {
	return c.session.RowStore.Observe(canon, rows, pagination, origin, appendPage)
}

// ObserveCountRows is the dual-write chokepoint for a counts-only observation
// (C6a: never touches Rows — e.g. the disk-cache-loaded seed when no
// per-type disk row data is available and the legacy map falls back to
// placeholder rows, which must never be fed into RowStore).
func (c *Core) ObserveCountRows(canon string, totalCount int) domain.Gen {
	return c.session.RowStore.ObserveCount(canon, totalCount)
}

// ObservePartialRows is ObserveRows' counterpart for the LazyResourceCache
// dual-write lane (sparse FetchByIDs adds, never a full first page).
func (c *Core) ObservePartialRows(canon string, rows []resource.Resource) ([]resource.Resource, domain.Gen) {
	return c.session.RowStore.ObservePartial(canon, rows)
}

// AmendRows is the dual-write counterpart for a copy-on-write enrichment
// fold over canon's currently retained row set. fn receives the existing
// slice and returns its replacement — see RowStore.Amend's doc comment for
// the copy-on-write invariant this enforces.
func (c *Core) AmendRows(canon string, fn func([]resource.Resource) []resource.Resource) domain.Gen {
	return c.session.RowStore.Amend(canon, fn)
}
