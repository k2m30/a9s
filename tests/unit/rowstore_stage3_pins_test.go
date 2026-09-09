// rowstore_stage3_pins_test.go — pins for task #17 wave 1 Stage 3 (row-store
// unification): session.ResourceCache + session.LazyResourceCache die;
// PatchResourceCache/PatchLazyResourceCache intents target session.RowStore
// (Observe/ObservePartial) instead of the two legacy maps; related lanes read
// the store union (full-beats-partial). See
// core/session/rowstore.go and the row-store unification plan
// (rowstore-unification-plan.md, Stage 3 bullet).
//
// Verified against HEAD 6d4ef1ce (Stage 2 landed, Stage 3 NOT yet landed):
// session.ResourceCache/session.LazyResourceCache still exist and are still
// the read path for core/runtime's related-navigation helpers
// (relatedCacheSnapshot/relatedFetchTasks in handlers_related.go read those
// two maps directly, never session.RowStore). Every pin below states its
// honest RED/GREEN status at HEAD in its own doc comment; several are GREEN
// today via Stage-1/Stage-2 groundwork and exist here purely as regression
// guards against Stage 3 regressing what already works.
//
// Architect clarification incorporated (mid-dispatch correction): Partial is
// a TYPE-LEVEL property on TypeRows, not a per-row tag.
//   - (a) A type NEVER canonically observed (Observe/ObserveCount), whose only
//     rows arrived via ObservePartial (the lazy-related-add lane): the whole
//     TypeRows entry is Partial=true, and SnapshotAll(false) — the input to
//     BuildResourceCacheSnapshot and the disk-save lane — skips it entirely.
//     Pin exactly this "never poisons a type never canonically listed" case.
//   - (b) A type ALREADY canonically observed (a full Observe has landed,
//     Partial=false) that later receives a lazy add via ObservePartial: those
//     rows are legitimate deeper-page rows of the SAME canonical list. They
//     APPEND (dedup by ID) onto the existing Rows, Partial flips back to true
//     per ObservePartial's own contract (RowStore.ObservePartial always sets
//     Partial=true on its own written entry — see rowstore.go's Observe vs
//     ObservePartial doc comments), but the merged Rows themselves — including
//     the newly-appended lazy IDs — legitimately continue to appear in any
//     later SnapshotAll(true) or Amend fold same as a load-more page would.
//     This file does NOT pin case (b)'s absence anywhere; only case (a).
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// newRowStorePinsTestController mirrors newSeededTestController /
// newDifferentialTestController (same package, same t.TempDir()-isolated
// A9S_CONFIG_FOLDER precondition) — duplicated as its own small variant per
// this test package's convention (each file avoids depending on another
// file's helper lifetime).
func newRowStorePinsTestController(t *testing.T) (*session.Session, *runtime.Core, *app.Controller) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	return s, core, c
}

// =============================================================================
// Pin 1 — generic scope-poison: a type NEVER canonically listed, whose only
// rows came from a lazy related-add (ObservePartial), never appears in
// canonical navigation seeds, never enters BuildEnrichQueue, and never
// reaches SnapshotAll(false) (the disk-save lane's own input).
// =============================================================================

// TestScopePoison_PartialOnlyType_NeverInEnrichQueue drives a lazy-add
// (ObservePartialRows) for "ec2" — a real Wave-2-enricher-registered type
// (core/aws/catalog_compute.go: Wave2: IssueEnricher{Fn:
// EnrichEC2InstanceStatus}) — with NO prior canonical Observe/ObserveCount for
// that type, then asserts "ec2" is absent from BuildEnrichQueue.
//
// HONEST STATUS AT HEAD (6d4ef1ce): RED. Core.BuildEnrichQueue
// (core/runtime/probes.go:709-719) gates solely on
// `c.session.RowStore.Snapshot(e.ShortName).Gen == 0` — it does NOT check
// TypeRows.Partial. RowStore.ObservePartial (rowstore.go:272-289) always
// bumps Gen (existing.Gen+1) even for a type that has never been Observed, so
// a lazy-add-only type's Gen is 1, not 0, and it wrongly PASSES the
// Gen==0 guard and enters the queue today. This is a genuine bug this pin
// catches independent of Stage 3 landing — Stage 3's stated goal ("Partial
// never enters BuildEnrichQueue... now enforced by one flag instead of a
// separate map") is exactly the fix this pin expects.
func TestScopePoison_PartialOnlyType_NeverInEnrichQueue(t *testing.T) {
	_, core, _ := newRowStorePinsTestController(t)

	core.ObservePartialRows("ec2", []resource.Resource{
		{ID: "i-lazy-only-1", Name: "lazy-only", Type: "ec2"},
	})

	queue := core.BuildEnrichQueue()
	for _, rt := range queue {
		if rt == "ec2" {
			t.Fatalf("BuildEnrichQueue = %v, must not contain %q — a type never canonically observed, only lazy-added, must never enter the Wave-2 enrich queue (C6 scope boundary)", queue, "ec2")
		}
	}
}

// TestScopePoison_PartialOnlyType_VisibleInResourceCacheSnapshotButTruncated
// pins the arbitrated boundary of BuildResourceCacheSnapshot, the merged view
// RELATED-lane checkers (NeedsTargetCache) consume: unlike BuildEnrichQueue
// and SnapshotAll(false) (the disk-save lane), a Partial-only type MUST be
// visible here, carrying its rows with the truncated marker set — this is
// the load-bearing replacement for the deleted session.LazyResourceCache
// merge that related-panel checkers relied on (see the sibling pin
// TestBuildResourceCacheSnapshot_LazyOnlyTruncated in
// tests/unit/qa_lazy_cache_snapshot_truncated_test.go, which exercises the
// same contract end-to-end through a real NeedsTargetCache checker).
//
// Architect arbitration (final, this file's original HONEST STATUS was
// wrong): BuildResourceCacheSnapshot reads
// `s.RowStore.SnapshotAll(true)` (includePartial), not SnapshotAll(false) —
// see probes.go's own doc comment ("Backed entirely by RowStore... a type's
// rows live in exactly one RowStore entry regardless of which lane wrote
// them"). It skips only Gen == 0 (never observed at all); ObservePartial
// always bumps Gen off zero, so a Partial-only type's Gen != 0 and it is
// NOT skipped. The scope-poison boundary that IS enforced lives at three
// other seams, each pinned separately in this file/package: navigation
// seeds (BuildEnrichQueue, TestScopePoison_PartialOnlyType_NeverInEnrichQueue
// above) and disk saves
// (SnapshotAll(false), TestScopePoison_PartialOnlyType_ExcludedFromSnapshotAllFalse
// below).
//
// HONEST STATUS AT HEAD: GREEN. BuildResourceCacheSnapshot's Gen-only skip
// and its own IsTruncated derivation (`tr.Partial || ...Pagination.IsTruncated`,
// probes.go:806) already produce exactly this shape at HEAD; kept as a
// regression guard against a future change narrowing the Gen==0 skip into a
// Partial-aware skip, which would silently break every NeedsTargetCache
// related-panel checker that depends on lazy-added rows being visible here.
func TestScopePoison_PartialOnlyType_VisibleInResourceCacheSnapshotButTruncated(t *testing.T) {
	_, core, _ := newRowStorePinsTestController(t)

	core.ObservePartialRows("ec2", []resource.Resource{
		{ID: "i-lazy-only-2", Name: "lazy-only-2", Type: "ec2"},
	})

	snap := core.BuildResourceCacheSnapshot()
	entry, ok := snap["ec2"]
	if !ok {
		t.Fatalf("BuildResourceCacheSnapshot[%q] absent, want present — a lazy-only type's rows must feed RELATED-lane checkers (NeedsTargetCache) exactly like the deleted session.LazyResourceCache merge did", "ec2")
	}
	if !entry.IsTruncated {
		t.Errorf("BuildResourceCacheSnapshot[%q].IsTruncated = false, want true — a Partial (lazy-add) entry is sparse (FetchByIDs, not a full first page) and must be marked truncated", "ec2")
	}
	if len(entry.Resources) != 1 || entry.Resources[0].ID != "i-lazy-only-2" {
		t.Errorf("BuildResourceCacheSnapshot[%q].Resources = %+v, want one row i-lazy-only-2", "ec2", entry.Resources)
	}
}

// TestScopePoison_PartialOnlyType_ExcludedFromSnapshotAllFalse pins the C6
// scope boundary directly at the RowStore level: SnapshotAll(false) — the
// exact input the disk-save lane (snapshotRowStoreForSave /
// rowStoreResourcesAndTruncated, both unexported in core/runtime) reads —
// never surfaces a Partial-only type, so no disk save can ever be seeded from
// lazy-add-only rows.
//
// HONEST STATUS AT HEAD: GREEN already (regression guard). Documents the
// disk-save leg of the scope-poison triad (enrich-queue / cache-snapshot /
// disk-save) that pins 1a-1c together cover; snapshotRowStoreForSave itself
// is unexported and untestable directly from tests/unit, so this asserts its
// documented input contract instead (rowstore.go SnapshotAll doc comment +
// handlers_availability.go:578-601 doc comment, both citing the same
// SnapshotAll(false) call).
func TestScopePoison_PartialOnlyType_ExcludedFromSnapshotAllFalse(t *testing.T) {
	s, core, _ := newRowStorePinsTestController(t)

	core.ObservePartialRows("ec2", []resource.Resource{
		{ID: "i-lazy-only-3", Name: "lazy-only-3", Type: "ec2"},
	})

	all := s.RowStore.SnapshotAll(false)
	if entry, ok := all["ec2"]; ok {
		t.Fatalf("RowStore.SnapshotAll(false)[%q] = %+v, want absent — the disk-save lane's own input must never see a lazy-only type's rows", "ec2", entry)
	}

	// Regression companion: includePartial=true (the related-lane's own read,
	// per the plan's "related lanes read the store union" directive) MUST see
	// it — the type-level Partial flag hides it from canonical saves/enrich,
	// not from the related lookup lane itself.
	allWithPartial := s.RowStore.SnapshotAll(true)
	if entry, ok := allWithPartial["ec2"]; !ok || len(entry.Rows) != 1 || entry.Rows[0].ID != "i-lazy-only-3" {
		t.Fatalf("RowStore.SnapshotAll(true)[%q] = %+v, want one row i-lazy-only-3 — related lanes must still see partial-only rows", "ec2", entry)
	}
}

// =============================================================================
// Pin 2 — precedence: ObservePartial rows for a type, then a full-fetch
// Observe. Colliding IDs must keep the richer (full) row, Partial flips
// false, and a related-drill lookup resolves against the full row.
//
// Ports the intent of TestRelatedCacheSnapshot_MergePrecedence
// (core/runtime/handlers_related_test.go:26-56, "on collision ResourceCache
// must win over LazyResourceCache") onto the store union: on collision, a
// full Observe (session.ResourceCache's Stage-3 replacement) must win over a
// prior ObservePartial (session.LazyResourceCache's Stage-3 replacement).
// =============================================================================

// TestPrecedence_FullObserveWinsOverPriorPartial_OnIDCollision pins that once
// a colliding ID has been observed by BOTH ObservePartial (first) and Observe
// (second, richer, full-beats-partial), the store's retained row for that ID
// is the FULL observation's version (Name/Fields intact), not the earlier
// partial's zero-value stand-in — mirroring
// TestRelatedCacheSnapshot_MergePrecedence's "ResourceCache must win on
// collision" assertion (got["i-shared"] != "from-cache" fails there).
//
// HONEST STATUS AT HEAD: GREEN at the RowStore level (RowStore.Observe already
// implements full-beats-partial — Observe unconditionally accepts and always
// clears Partial per its own doc comment). RED at the runtime-helper level:
// core/runtime/handlers_related.go's relatedCacheSnapshot/relatedFetchTasks
// still read session.ResourceCache/session.LazyResourceCache directly (not
// RowStore) at HEAD, so seeding ONLY via core.ObservePartialRows +
// core.ObserveRows produces no observable change in
// Core.RelatedCachedResource/Core.HandleRelatedNavigate until Stage 3
// re-points those helpers onto the store union — this is the porting target
// the dispatch calls out ("port the expectations... onto the store union").
func TestPrecedence_FullObserveWinsOverPriorPartial_OnIDCollision(t *testing.T) {
	_, core, _ := newRowStorePinsTestController(t)

	// Partial arrives first (lazy related-add), carrying only a bare ID.
	core.ObservePartialRows("s3", []resource.Resource{
		{ID: "shared-bucket"},
	})

	// A full fetch lands second, with the richer Name/Fields payload.
	core.ObserveRows("s3", []resource.Resource{
		{ID: "shared-bucket", Name: "from-full-fetch", Fields: map[string]string{"region": "us-east-1"}},
	}, nil, session.OriginFetch, false)

	tr := core.Session().RowStore.Snapshot("s3")
	if tr.Partial {
		t.Error("RowStore.Snapshot(s3).Partial = true after a full Observe, want false — full-beats-partial must clear Partial")
	}
	var got *resource.Resource
	for i := range tr.Rows {
		if tr.Rows[i].ID == "shared-bucket" {
			got = &tr.Rows[i]
			break
		}
	}
	if got == nil {
		t.Fatal("RowStore.Snapshot(s3).Rows missing shared-bucket after full Observe")
	}
	if got.Name != "from-full-fetch" {
		t.Errorf("shared-bucket.Name = %q, want %q — full Observe must win over the earlier partial's zero-value row on ID collision", got.Name, "from-full-fetch")
	}

	// Related-drill parity: Core.RelatedCachedResource (the exported lookup
	// HandleRelatedNavigate's cache-hit path consults) must resolve the FULL
	// row, not the earlier partial's bare-ID stand-in. At HEAD this reads
	// session.ResourceCache/LazyResourceCache, neither of which was touched by
	// this test's ObservePartialRows/ObserveRows calls — so this assertion is
	// RED at HEAD (found=false) until Stage 3 re-points the lookup onto
	// RowStore's union.
	cached, found := core.RelatedCachedResource("s3", "shared-bucket")
	if !found {
		t.Fatalf("RelatedCachedResource(s3, shared-bucket) found=false — RED at HEAD: the related lookup still reads session.ResourceCache/LazyResourceCache, not RowStore's union (Stage 3 not yet landed)")
	}
	if cached.Name != "from-full-fetch" {
		t.Errorf("RelatedCachedResource(s3, shared-bucket).Name = %q, want %q (full-beats-partial)", cached.Name, "from-full-fetch")
	}
}

// =============================================================================
// Pin 3 — related-drill parity: a related navigation with RelatedIDs that
// exist ONLY as Partial rows in RowStore resolves identically to today's
// LazyResourceCache-backed path (NavigationKindDetail on a single-ID cache
// hit, matching TestHandleRelatedNavigate_DetailCacheHit_NoTask's shape in
// core/runtime/handlers_related_test.go:236-254, and
// TestRelatedCacheSnapshot_LazyOnly's "lazy-only entries are visible"
// expectation at handlers_related_test.go:58-68).
// =============================================================================

// TestRelatedDrillParity_PartialOnlyRows_ResolvesSameAsLazyResourceCacheToday
// drives HandleRelatedNavigate with a single RelatedIDs entry that exists ONLY
// as a store-side Partial row (never in session.ResourceCache, and — the
// Stage-3 point — no longer expected to require session.LazyResourceCache
// either). Asserts the navigation resolves to NavigationKindDetail (a cache
// hit) with no fetch task, exactly like TestHandleRelatedNavigate_
// DetailCacheHit_NoTask's shape when the same row is seeded via
// session.ResourceCache today.
//
// HONEST STATUS AT HEAD: RED. handlers_related.go's ResolveRelatedNavigate
// resolves a single-RelatedIDs cache hit via relatedCacheHit(cache, ...),
// fed by relatedCacheSnapshot(s), which unions session.LazyResourceCache +
// session.ResourceCache only — RowStore is never consulted. Seeding solely
// via core.ObservePartialRows (no session.LazyResourceCache write) therefore
// produces a MISS (NavigationKindFilteredList, not NavigationKindDetail) at
// HEAD. Stage 3's "related lanes read the store union (full-beats-partial)"
// is exactly what flips this green.
func TestRelatedDrillParity_PartialOnlyRows_ResolvesSameAsLazyResourceCacheToday(t *testing.T) {
	_, core, _ := newRowStorePinsTestController(t)

	core.ObservePartialRows("kms", []resource.Resource{
		{ID: "alias/aws/managed-parity", Name: "alias/aws/managed-parity", Type: "kms"},
	})

	result, tasks := core.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: "kms",
		RelatedIDs: []string{"alias/aws/managed-parity"},
	})

	if result.Kind != runtime.NavigationKindDetail {
		t.Fatalf("HandleRelatedNavigate(kms, RelatedIDs=[alias/aws/managed-parity]).Kind = %v, want NavigationKindDetail — RED at HEAD: relatedCacheSnapshot does not read RowStore, only session.ResourceCache/LazyResourceCache", result.Kind)
	}
	if tasks != nil {
		t.Errorf("tasks = %v, want nil — a store-side Partial-row cache hit must not dispatch a fetch, matching the LazyResourceCache-seeded cache-hit path today", tasks)
	}
}

// TestRelatedDrillParity_PartialOnlyRows_MultiIDCoverage_NoFetchTask extends
// the parity pin to the multi-RelatedIDs coverage path (relatedFetchTasks'
// "full coverage → no task" branch, mirroring
// TestRelatedFetchTasks_LazyFullCoverage_NoTask at
// handlers_related_test.go:82-92, which seeds full coverage via
// session.LazyResourceCache alone).
//
// HONEST STATUS AT HEAD: RED for the same reason as the single-ID pin above —
// relatedFetchTasks (core/runtime/handlers_related.go:184-226) reads
// s.ResourceCache[targetType] and s.LazyResourceCache[targetType] directly;
// neither is touched by ObservePartialRows, so "missing" is computed as 2 (both
// IDs uncovered) at HEAD and a KindFetchResources task is wrongly emitted for
// IDs the store already fully covers as Partial rows.
func TestRelatedDrillParity_PartialOnlyRows_MultiIDCoverage_NoFetchTask(t *testing.T) {
	_, core, _ := newRowStorePinsTestController(t)

	core.ObservePartialRows("kms", []resource.Resource{
		{ID: "alias/aws/managed-1", Name: "alias/aws/managed-1", Type: "kms"},
		{ID: "alias/aws/managed-2", Name: "alias/aws/managed-2", Type: "kms"},
	})

	result, tasks := core.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: "kms",
		RelatedIDs: []string{"alias/aws/managed-1", "alias/aws/managed-2"},
	})

	if result.Kind != runtime.NavigationKindFilteredList {
		t.Fatalf("HandleRelatedNavigate(kms, 2 RelatedIDs).Kind = %v, want NavigationKindFilteredList (multi-ID filtered view)", result.Kind)
	}
	if tasks != nil {
		t.Errorf("tasks = %v, want nil — RED at HEAD: relatedFetchTasks does not see RowStore's Partial rows as coverage, so it wrongly emits a fetch task for IDs the store already has", tasks)
	}
}

// =============================================================================
// Pin 4 — menuRefreshing counts-only pin (Stage-2 deferred gap, closed in
// Stage 3 per the coordinator's mandate): a counts-only AvailabilityCacheLoaded
// seed (no per-type disk row data) must still show the menu's
// refreshing/updating indicator until the sweep completes for that type.
// =============================================================================

// TestMenuRefreshing_CountsOnlySeed_StaysRefreshingUntilSweepCompletes drives
// a real AvailabilityCacheLoaded event through Controller.Handle with a
// non-zero Entries count for a type that has NO on-disk per-type row data
// (newRowStorePinsTestController's t.TempDir()-isolated A9S_CONFIG_FOLDER
// guarantees the disk store is empty), forcing
// handleAvailabilityCacheLoaded's counts-only fallback
// (c.ObserveCountRows(shortName, count), handlers_availability.go:126) rather
// than the rows-carrying c.ObserveRows branch. Asserts
// ViewState.Body.Menu.Refreshing is true immediately after the seed (sweep
// still in flight), then flips false once the matching AvailabilityChecked
// result lands (Gen=1, matching session.New()'s AvailabilityGen seed).
//
// HONEST STATUS AT HEAD (6d4ef1ce): RED for the "stays true" half.
// Controller.menuRefreshing() (core/app/menu.go:196-207) reports true iff
// core.ProbeOriginTypeNames() contains an un-acked type.
// RowStore.ProbeOriginTypeNames() (session/rowstore.go:384-395) filters on
// `len(tr.Rows) > 0` — but ObserveCountRows/ObserveCount (rowstore.go:248-258)
// deliberately never touches Rows (C6a). A counts-only-seeded type therefore
// NEVER appears in ProbeOriginTypeNames(), so menuRefreshing() returns false
// immediately even while that type's own Wave-1 sweep result has not yet
// landed — exactly the "Stage-2 deferred gap" the plan/coordinator flagged for
// Stage 3 to close (menuRefreshing must track counts-only-seeded types too,
// not just rows-carrying ones).
func TestMenuRefreshing_CountsOnlySeed_StaysRefreshingUntilSweepCompletes(t *testing.T) {
	s, _, c := newRowStorePinsTestController(t)

	c.Handle(messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"lambda": 7},
	})

	snap := c.Snapshot()
	if snap.Body.Menu == nil {
		t.Fatal("Body.Menu is nil after AvailabilityCacheLoaded")
	}
	if !snap.Body.Menu.Refreshing {
		t.Error("Body.Menu.Refreshing = false immediately after a counts-only AvailabilityCacheLoaded seed, want true — the background sweep for this type has not completed yet (Stage-2 deferred gap: menuRefreshing only tracks RowStore.ProbeOriginTypeNames, which a counts-only ObserveCount write never populates since it never touches Rows)")
	}

	// Sweep completes: the matching AvailabilityChecked result lands.
	c.Handle(messages.AvailabilityChecked{
		ResourceType: "lambda",
		HasResources: true,
		Count:        7,
		Gen:          s.AvailabilityGen,
	})

	snap = c.Snapshot()
	if snap.Body.Menu == nil {
		t.Fatal("Body.Menu is nil after AvailabilityChecked")
	}
	if snap.Body.Menu.Refreshing {
		t.Error("Body.Menu.Refreshing = true after the sweep's AvailabilityChecked result landed, want false — the sweep has completed for every seeded type")
	}
}

// =============================================================================
// Pin 5 — C9 rotate: after a profile/region pair switch, the store holds
// nothing for the old pair, INCLUDING rows that were only ever Partial.
// Extends TestDifferential_PairSwitch_ClearsRowStoreLikeLegacyMaps
// (rowstore_differential_test.go:478-501, which only exercises a
// fully-Observed row) to the Partial-only case.
// =============================================================================

// TestRotate_ClearsPartialOnlyRows pins that a type observed ONLY via
// ObservePartial (never a full Observe/ObserveCount) is gone from RowStore
// after Session.Rotate — Gen resets to 0 (never observed this new session)
// and Rows is empty, exactly like the existing pair-switch pin's assertion
// shape for a fully-Observed row.
//
// HONEST STATUS AT HEAD: GREEN already (regression guard). RowStore.Clear()
// (rowstore.go:354-359, called from Session.Rotate at session.go:413)
// unconditionally replaces the whole `types` map — it has no Partial-aware
// carve-out, so a Partial-only entry is cleared exactly like any other. Kept
// here because Stage 3 deletes session.LazyResourceCache (whose own Rotate
// clear at session.go:415 is Partial's legacy counterpart) — this guards that
// RowStore.Clear() alone remains sufficient once that map is gone.
func TestRotate_ClearsPartialOnlyRows(t *testing.T) {
	s, core, _ := newRowStorePinsTestController(t)

	core.ObservePartialRows("efs", []resource.Resource{
		{ID: "fs-partial-preswitch", Type: "efs"},
	})
	pre := s.RowStore.Snapshot("efs")
	if !pre.Partial || len(pre.Rows) != 1 {
		t.Fatalf("precondition: RowStore.Snapshot(efs) = %+v, want Partial=true with 1 row before rotate", pre)
	}

	s.Profile = "other-profile-pin5"
	s.Region = "eu-west-1"
	s.Rotate()

	post := s.RowStore.Snapshot("efs")
	if post.Gen != 0 {
		t.Errorf("RowStore.Snapshot(efs).Gen after pair switch = %d, want 0 (never observed this new session) — C9 must clear Partial-only entries too", post.Gen)
	}
	if len(post.Rows) != 0 {
		t.Errorf("RowStore.Snapshot(efs).Rows after pair switch = %+v, want empty — C9 must clear Partial-only entries too", post.Rows)
	}
	if post.Partial {
		t.Error("RowStore.Snapshot(efs).Partial after pair switch = true, want false (zero-value TypeRows for a never-observed-this-session type)")
	}

	// Regression companion: ProbeOriginTypeNames/SnapshotAll must also show no
	// trace of the old pair's Partial-only type post-rotate — a related-lane
	// read for "efs" after rotate must not resurrect the pre-switch lazy rows.
	all := s.RowStore.SnapshotAll(true)
	if _, ok := all["efs"]; ok {
		t.Errorf("RowStore.SnapshotAll(true) still contains %q after rotate, want absent", "efs")
	}
}
