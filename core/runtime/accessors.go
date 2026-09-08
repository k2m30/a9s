// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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
// core/session.
package runtime

import (
	"maps"
	"time"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// Bootstrap constructs a fresh *Core seeded with a new session.Session
// configured for the given profile/region pair. Used by the renderer's
// model constructor (tui.New) so it can build the Core without importing
// core/session.
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
func (c *Core) Profile() string {
	p, _ := c.session.CurrentPair()
	return p
}

// SetProfile sets the active session profile. Used by the WithProfile
// constructor option only, before any goroutine other than the caller's own
// can observe the session — goes through SetProfileRegion regardless, so a
// later WithProfile/WithRegion combination (both constructor options run
// during the same single-threaded construction) never risks a torn pair.
func (c *Core) SetProfile(p string) {
	_, r := c.session.CurrentPair()
	c.session.SetProfileRegion(p, r)
}

// Region returns the active session region.
func (c *Core) Region() string {
	_, r := c.session.CurrentPair()
	return r
}

// SetRegion sets the active session region. Used by the WithRegion
// constructor option only. See SetProfile's doc comment.
func (c *Core) SetRegion(r string) {
	p, _ := c.session.CurrentPair()
	c.session.SetProfileRegion(p, r)
}

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
// cache.LoadDirIn(root, profile, region) whenever the memoized store (if any) was
// not loaded for the CURRENT session.Profile/session.Region pair — this
// covers both the first call since the last Rotate (C9) or process start,
// and a pair switch that lands between two calls without an intervening
// Rotate observation. NoCache=true always returns nil without ever calling
// LoadDirIn (C7b: --no-cache disables persisted load entirely). An unresolved
// Profile or Region (pair not yet resolved) returns nil WITHOUT memoizing, so
// a pre-connect call never pins the store to the wrong "<profile>--"
// directory. Session.EnsureCacheStore reads the pair itself under
// session.pairMu — this method no longer reads session.Profile/Region at all,
// closing the cross-goroutine race a profile/region switch (which writes
// those fields via SetProfileRegion, also under pairMu) used to have against
// a concurrent caller here (CI run 28839454135).
func (c *Core) EnsureCacheStore() *cache.Store {
	if c.session.NoCache {
		return nil
	}
	return c.session.EnsureCacheStore()
}

// WithCacheStore runs fn against the current pair's *cache.Store with
// session.pairMu held for fn's ENTIRE duration, including any disk I/O fn
// performs — the coarse, general-purpose primitive. See
// Session.WithCacheStore's doc comment for why SaveResourceListCache/
// SaveAvailabilityCache use the narrower WithCacheStoreSave below instead.
// No-op (fn not called) when NoCache is set, mirroring EnsureCacheStore.
func (c *Core) WithCacheStore(fn func(store *cache.Store) error) error {
	if c.session.NoCache {
		return nil
	}
	return c.session.WithCacheStore(fn)
}

// WithCacheStoreSave runs fn against pair's *cache.Store with
// session.pairMu held for the pair read, the store decision, and fn itself —
// fn's own store.Type read and store.Put write for one type file can never
// interleave with another such sequence running concurrently (the store-lock
// serialization, D13), and the Profile/Region pair itself cannot be read torn
// or racing a concurrent profile/region switch. fn stages its writes as
// cache.WritePlan values instead of touching disk itself; the actual write
// happens after pairMu is released — see Session.WithCacheStoreSave's doc
// comment for the full design and its trade-offs. No-op (fn not called) when
// NoCache is set, mirroring EnsureCacheStore.
func (c *Core) WithCacheStoreSave(pair session.Pair, fn func(store *cache.Store) ([]cache.WritePlan, error)) error {
	if c.session.NoCache {
		return nil
	}
	return c.session.WithCacheStoreSave(pair, fn)
}

// Pair returns the session's current profile/region as one value — what a
// save path stamps onto work it is about to hand to another goroutine, and
// what a synchronous saver passes straight back in.
func (c *Core) Pair() session.Pair { return c.session.CurrentPairValue() }

// ReadCacheStore runs fn against the current pair's *cache.Store with
// session.pairMu held, for read-only callers (store.Type/store.Types). See
// Session.ReadCacheStore for why a read call site must not bypass this lock
// even though it never mutates the store itself.
func (c *Core) ReadCacheStore(fn func(store *cache.Store) error) error {
	if c.session.NoCache {
		return nil
	}
	return c.session.ReadCacheStore(fn)
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
// renderer adapters need not import core/aws.
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

// EnrichmentTypeGen returns the per-type Wave-2 enrichment counter for the
// given resource short name. Zero when no enrichment has run yet for the
// type.
func (c *Core) EnrichmentTypeGen(rt string) domain.Gen { return c.session.EnrichmentTypeGenGet(rt) }

// BumpEnrichmentTypeGen increments the per-type Wave-2 counter and returns
// the new value. Used by the refresh path to invalidate the prior batch's
// per-type findings before re-dispatching.
func (c *Core) BumpEnrichmentTypeGen(rt string) domain.Gen {
	return c.session.EnrichmentTypeGenBump(rt)
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
	c.session.EnrichmentTypeGenReset()
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
// given resource short name, or (nil, false) when no FULL (non-Partial),
// OriginFetch entry is cached. Renderer adapters use this in place of
// indexing a session map directly. Backed by RowStore (task #17 wave 1
// stage 3 — the former session.ResourceCache map is gone; a type's rows
// live in exactly one RowStore entry). The returned entry is freshly built
// from the store's defensive-copy Snapshot on every call — mutating it does
// not write back into RowStore (callers wishing to mutate content use
// SetResourceCache, AmendRows, or the Observe* family).
//
// Origin-gated to OriginFetch only (not Probe/Disk): a Wave-1
// availability-probe or disk-seeded entry is knowledge the probe/disk
// gathered, not a verified live top-level fetch, so it must never satisfy a
// "this list is already cached" check — HandleNavigate's own promotion
// decision (NavigateKindPushResourceListCached vs. a miss that still
// dispatches KindFetchResources) depends on this distinction. Callers that
// want an any-origin lookup (e.g. related-navigate's cache-hit resolution,
// which already consulted an any-origin RowStore snapshot to decide the hit)
// must read RowStore directly rather than through this accessor.
func (c *Core) ResourceCache(rt string) (*domain.ListViewCacheEntry, bool) {
	tr := c.session.RowStore.Snapshot(rt)
	if tr.Gen == 0 || tr.Partial || tr.Origin != session.OriginFetch {
		return nil, false
	}
	return listViewCacheEntryFromTypeRows(tr), true
}

// AnyOriginResourceCache returns the cached top-level resource-list entry
// for the given resource short name regardless of which lane populated it
// (OriginFetch, OriginProbe, or OriginDisk), or (nil, false) when no FULL
// (non-Partial) entry exists at all. Renderer adapters use this instead of
// ResourceCache when the caller has already established a cache hit against
// an any-origin RowStore snapshot (e.g. ResolveRelatedNavigate's
// NavigationKindDetail/NavigationKindFilteredList resolution, which reads
// RowStore.SnapshotAll(true) directly) and merely needs to re-fetch that
// same entry's rows — narrowing to OriginFetch here would make the lookup
// miss for a Probe/Disk-origin entry the caller already confirmed exists,
// silently dropping a related-navigate cache hit back to a flash error.
func (c *Core) AnyOriginResourceCache(rt string) (*domain.ListViewCacheEntry, bool) {
	tr := c.session.RowStore.Snapshot(rt)
	if tr.Gen == 0 || tr.Partial {
		return nil, false
	}
	return listViewCacheEntryFromTypeRows(tr), true
}

// AnyOriginResourceCacheGen returns the RowStore generation for the full
// any-origin cache entry for rt, or zero when no such entry exists. Unlike
// AnyOriginResourceCache it does not clone rows, so render memo keys can check
// invalidation cheaply on cache hits.
func (c *Core) AnyOriginResourceCacheGen(rt string) domain.Gen {
	gen, partial := c.session.RowStore.SnapshotMeta(rt)
	if gen == 0 || partial {
		return 0
	}
	return gen
}

// AnyLaneResources returns a type's RowStore rows from EITHER lane — full or
// Partial — ignoring the Gen/Partial gate AnyOriginResourceCache applies. It is
// the render-time row source for a related-FILTERED list: because the list is
// scoped to a RelatedIDSet, surfacing the Partial (lazy/by-ID) lane is safe —
// everything outside the set is filtered out — so a cache-hit filtered list
// renders its rows without a fetch, identically on both renderers.
func (c *Core) AnyLaneResources(rt string) []domain.Resource {
	return c.session.RowStore.Snapshot(rt).Rows
}

// AnyLaneResourceByID returns the single row of type rt whose ID matches id,
// scanning either lane (full or Partial) via a linear search over
// AnyLaneResources — the row set per type is small enough (list-page-sized,
// not account-wide) that a scan beats maintaining a second by-ID index.
// Used to resolve a full cached resource for a related-panel row before
// falling back to a StubCreator/ID-only synthesis (see core/app ConsoleTarget).
func (c *Core) AnyLaneResourceByID(rt, id string) (domain.Resource, bool) {
	for _, r := range c.AnyLaneResources(rt) {
		if r.ID == id {
			return r, true
		}
	}
	return domain.Resource{}, false
}

// listViewCacheEntryFromTypeRows builds the renderer-facing
// domain.ListViewCacheEntry from a RowStore TypeRows snapshot, threading
// through the retained ListViewState (filter/sort/cursor/h-scroll) alongside
// Resources/Pagination/TotalCount — shared by ResourceCache and
// AnyOriginResourceCache so both cache-hit paths restore the exact same
// warm-reentry view state a list was left in.
func listViewCacheEntryFromTypeRows(tr session.TypeRows) *domain.ListViewCacheEntry {
	return &domain.ListViewCacheEntry{
		Resources:     tr.Rows,
		Pagination:    tr.Pagination,
		TotalCount:    tr.TotalCount,
		FilterText:    tr.ViewState.FilterText,
		AttentionOnly: tr.ViewState.AttentionOnly,
		SortColIdx:    tr.ViewState.SortColIdx,
		SortAsc:       tr.ViewState.SortAsc,
		CursorPos:     tr.ViewState.CursorPos,
		HScrollOffset: tr.ViewState.HScrollOffset,
	}
}

// SetResourceCache stores the cached top-level resource-list entry for the
// given resource short name as a full (non-Partial), OriginFetch RowStore
// observation — replacing the rows wholesale (mirrors the former map's bare
// assignment semantics; Observe's own stale-replace guard still applies for
// a smaller/truncated/subset replace). A nil entry drops the cached entry
// entirely so the next ResourceCache/HasResourceCache call reports a miss,
// matching the former map's `m[rt] = nil` behavior (which HasResourceCache
// already treated as absent).
//
// e's interactive-state fields (FilterText/AttentionOnly/SortColIdx/
// SortAsc/CursorPos/HScrollOffset) are written to the entry's ListViewState
// via SetViewState — a separate RowStore write from the rows-carrying
// Observe/ObserveCount above (TypeRows.ViewState's doc comment explains why
// the two are orthogonal). Without this, a warm re-entry into a cached list
// would restore the rows but silently drop the sort column/direction and
// cursor position the user left the list in.
func (c *Core) SetResourceCache(rt string, e *domain.ListViewCacheEntry) {
	if e == nil {
		c.session.RowStore.Delete(rt)
		return
	}
	c.session.RowStore.Observe(rt, e.Resources, e.Pagination, session.OriginFetch, false)
	if e.TotalCount != 0 {
		c.session.RowStore.ObserveCount(rt, e.TotalCount)
	}
	c.session.RowStore.SetViewState(rt, session.ListViewState{
		FilterText:    e.FilterText,
		AttentionOnly: e.AttentionOnly,
		SortColIdx:    e.SortColIdx,
		SortAsc:       e.SortAsc,
		CursorPos:     e.CursorPos,
		HScrollOffset: e.HScrollOffset,
	})
}

// DeleteResourceCache drops the cached resource-list entry for the given
// resource short name, so the next list-open re-fetches. Backed by
// RowStore.Delete — the entry is removed entirely (Gen resets to 0), not
// merely emptied, so a subsequent Snapshot reports "never observed" rather
// than "observed empty".
func (c *Core) DeleteResourceCache(rt string) { c.session.RowStore.Delete(rt) }

// HasResourceCache reports whether a full (non-Partial), OriginFetch cached
// entry exists for the given resource short name (without exposing the
// entry itself or any other type). Renderer adapters that only need the
// existence signal use this in place of `m.core.ResourceCache(rt)` to avoid
// binding to the entry shape. Origin-gated identically to ResourceCache —
// see its doc comment for why a Probe/Disk-origin entry must not report a
// hit here.
func (c *Core) HasResourceCache(rt string) bool {
	tr := c.session.RowStore.Snapshot(rt)
	return tr.Gen != 0 && !tr.Partial && tr.Origin == session.OriginFetch
}

// ResourceCacheKeys returns the set of resource short names that currently
// have a full (non-Partial) cached top-level list entry. Snapshot semantics
// — the returned slice is decoupled from the underlying store.
func (c *Core) ResourceCacheKeys() []string {
	all := c.session.RowStore.SnapshotAll(false)
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	return keys
}

// FetchOriginCacheKeys returns the set of resource short names whose cached
// top-level list entry passes the same freshness gate as HasResourceCache
// (Gen != 0, !Partial, Origin == OriginFetch) — i.e. a verified live
// top-level fetch, never a Wave-1 availability probe or disk-cache seed.
// Related-checker fan-out (relatedCheckCmd / runRelatedCheckers) uses this to
// decide whether a NeedsTargetCache pivot can skip its own fresh target
// fetch: a Probe/Disk-origin entry is knowledge gathered for the menu
// counts/availability sweep, not a verified-fresh target list, so treating it
// as "already cached" here would silently suppress the live fetch and let
// the pivot serve stale or incomplete rows. ResourceCacheKeys (which includes
// Probe/Disk origins) remains the correct source for the enrichment-fold
// consumer in app_enrich_fold.go, whose membership semantics are unrelated to
// freshness-gating and must not change.
func (c *Core) FetchOriginCacheKeys() []string {
	all := c.session.RowStore.SnapshotAll(false)
	keys := make([]string, 0, len(all))
	for k, tr := range all {
		if tr.Gen != 0 && !tr.Partial && tr.Origin == session.OriginFetch {
			keys = append(keys, k)
		}
	}
	return keys
}

// ForEachResourceCache invokes fn for every full (non-Partial) cached
// resource-list entry. Each entry is freshly built from the store's
// defensive-copy SnapshotAll, so the callback's in-place mutation of
// entry.Resources[i] fields is safe (it mutates the copy, not RowStore's
// backing array) but does NOT propagate back into the store — callers
// needing the mutation to stick call SetResourceCache/AmendRows explicitly
// afterward (mirrors the former map's shared-backing-array semantics for
// the read side only; the write-back is now explicit, not implicit).
func (c *Core) ForEachResourceCache(fn func(rt string, entry *domain.ListViewCacheEntry)) {
	for rt, tr := range c.session.RowStore.SnapshotAll(false) {
		entry := &domain.ListViewCacheEntry{
			Resources:  tr.Rows,
			Pagination: tr.Pagination,
			TotalCount: tr.TotalCount,
		}
		fn(rt, entry)
	}
}

// NewFindingPairsSincePrev returns a deep copy of every resource type's
// new-finding-pair counts recorded by the most recent on-disk cache save for
// that type — the (row, finding-code) pairs absent from the previous
// on-disk generation, per domain.FindingCode (#463). Empty for a type that
// has never been saved this session, or when caching is disabled. Backed by
// session.AllNewFindingPairs, itself already a deep copy — this wrapper
// exists only so app/ and other renderer-facing callers never reach through
// Core into session.Session directly.
func (c *Core) NewFindingPairsSincePrev() map[string]map[domain.FindingCode]int {
	return c.session.AllNewFindingPairs()
}

// FindingFirstSeenForType returns a defensive-copy map of rowID ->
// domain.FindingCode -> first-observation time, read from the on-disk
// availability cache for shortName's canonical type (#463). Empty when
// caching is disabled (NoCache/demo), no cache entry exists yet for
// shortName, or shortName carries no findings — callers treat a missing
// entry as "unknown" (zero time), never as an error. This is the single
// accessor through which core/app reads FirstSeen, so *cache.Store never
// leaks past this package.
func (c *Core) FindingFirstSeenForType(shortName string) map[string]map[domain.FindingCode]time.Time {
	canon := canonShortName(shortName)
	out := make(map[string]map[domain.FindingCode]time.Time)
	_ = c.ReadCacheStore(func(store *cache.Store) error {
		if store == nil {
			return nil
		}
		tf, ok := store.Type(canon)
		if !ok {
			return nil
		}
		for _, r := range tf.Rows {
			if len(r.FindingFirstSeen) == 0 {
				continue
			}
			out[r.ID] = maps.Clone(r.FindingFirstSeen)
		}
		return nil
	})
	return out
}

// LazyResourceCache returns the lazy-cache slice for the given resource
// short name (resources pulled via FetchByIDs for filtered-target drills).
// The bool reports whether a Partial RowStore entry exists for the type —
// distinct from a non-nil empty slice. Backed by RowStore (task #17 wave 1
// stage 3 — the former session.LazyResourceCache map is gone).
func (c *Core) LazyResourceCache(rt string) ([]domain.Resource, bool) {
	tr := c.session.RowStore.Snapshot(rt)
	if tr.Gen == 0 || !tr.Partial {
		return nil, false
	}
	return tr.Rows, true
}

// ForEachLazyResourceCache invokes fn for every Partial RowStore entry. The
// slice is a defensive copy (RowStore.SnapshotAll); the callback's in-place
// mutation of rows[i] fields does not propagate back into the store.
func (c *Core) ForEachLazyResourceCache(fn func(rt string, rows []resource.Resource)) {
	for rt, tr := range c.session.RowStore.SnapshotAll(true) {
		if !tr.Partial {
			continue
		}
		fn(rt, tr.Rows)
	}
}

// ExtendLazyResourceCache merges the given per-type rows into the lazy
// cache. Used by the PatchLazyResourceCache intent dispatcher. Backed
// entirely by RowStore.ObservePartial (task #17 wave 1 stage 3 — the former
// session.LazyResourceCache map dual-write is gone; ObservePartial is now
// the sole write path). Each adds[rt] is already the full merged slice
// HandleRelatedCheckResult computed (dedup-appended against the prior
// lazy-cache entry) — ObservePartial's own dedup-append makes re-merging it
// against the store idempotent.
func (c *Core) ExtendLazyResourceCache(adds map[string][]resource.Resource) {
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

// ProbeResources returns the canonical retained rows for the given resource
// short name, read from RowStore (task #17 wave 1 stage 2 — replaces the
// removed session.ProbeResources map). Any full-population origin qualifies —
// Disk, Probe, or Fetch: a top-level list fetch REPLACES the probe/disk entry
// for its type (one RowStore entry per type), so an origin gate here would
// blind every probe-lane reader (ProbeEnrichment's enricher input,
// handleEnrichmentChecked's unifiedIssueCount, handleAvailabilityChecked's
// D17 Wave-2 carry) for exactly the type whose list is open on screen — the
// open type's Wave-2 findings would silently vanish while every other type
// enriches normally. Mirrors BuildEnrichQueue's observed-at-all membership
// rule (see its Gen != 0 doc comment). Only a Partial (sparse lazy-add)
// entry is excluded: it is not the type's canonical population, matching
// SnapshotAll(false)'s scope.
//
// The returned slice is a defensive copy (RowStore.Snapshot), so callers
// wishing to mutate row content must go through AmendRows instead of writing
// into the returned slice in place.
func (c *Core) ProbeResources(rt string) ([]resource.Resource, bool) {
	tr := c.session.RowStore.Snapshot(rt)
	if len(tr.Rows) == 0 || tr.Partial {
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

// PendingDetailRefreshGet returns the DetailOperation ID of the most recent
// unsatisfied explicit refresh recorded for the given resource key (see
// session.Session.PendingDetailRefresh), and whether one is recorded at all.
func (c *Core) PendingDetailRefreshGet(key string) (domain.Gen, bool) {
	op, ok := c.session.PendingDetailRefresh[key]
	return op, ok
}

// PendingDetailRefreshSet records opID as the most recent explicit-refresh
// demand for the given resource key.
func (c *Core) PendingDetailRefreshSet(key string, opID domain.Gen) {
	c.session.PendingDetailRefresh[key] = opID
}

// PendingDetailRefreshClear drops the recorded refresh demand for the given
// resource key — called once its enrichment has folded successfully.
func (c *Core) PendingDetailRefreshClear(key string) {
	delete(c.session.PendingDetailRefresh, key)
}

// FilteredRowsGet returns the cached rows for a server-side-filtered
// related drill of resourceType under the given fetch filter.
func (c *Core) FilteredRowsGet(resourceType string, filter map[string]string) (session.FilteredRowsEntry, bool) {
	return c.session.FilteredRows.Get(session.FilteredRowsKey(resourceType, filter))
}

// FilteredRowsSet stores a filtered drill's accumulated rows + pagination
// state under its (type + filter) key. Rows are copied — the caller's
// slice is screen-owned and mutated in place by later field updates.
func (c *Core) FilteredRowsSet(resourceType string, filter map[string]string, rows []resource.Resource, truncated bool, cursor string) {
	c.session.FilteredRows.Set(session.FilteredRowsKey(resourceType, filter), session.FilteredRowsEntry{
		Rows:      append([]resource.Resource(nil), rows...),
		Truncated: truncated,
		Cursor:    cursor,
	})
}

// HasIssueEnricher reports whether a Wave-2 issue enricher is registered
// for the given resource short name. Renderer adapters use this in place
// of probing awsclient.Wave2EnricherFor / awsclient.IssueEnricherRegistry
// directly.
func (c *Core) HasIssueEnricher(shortName string) bool {
	_, ok := awsclient.Wave2EnricherFor(shortName)
	return ok
}

// ObserveRows is the sole chokepoint for a rows-carrying write to the
// session-scoped RowStore. canon must already be the canonicalized resource
// short name — callers resolve aliases before calling, matching every other
// canon-keyed write in this package. Returns the accepted rows (a deep copy,
// independent of the store's own retained slice — see RowStore.Observe) plus
// the resulting Gen; core/app's applyResourcesLoaded assigns the returned
// rows directly onto the calling screen's ListState.Rows, so the value is
// load-bearing, not a diagnostic-only by-product.
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
