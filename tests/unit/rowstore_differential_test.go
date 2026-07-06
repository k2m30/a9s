// rowstore_differential_test.go — task #17 wave 1 stage 2 retired the legacy
// session.ProbeResources/ProbeTruncated maps this file's harness used to
// compare RowStore against (originally a Stage 1 dual-write EQUIVALENCE
// harness per rowstore-unification-plan.md). Per that plan's own Stage 5
// ("Remove scaffolding") instruction, the ProbeResources-vs-store comparisons
// are now meaningless — the map they compared against no longer exists — so
// those cases are converted to STORE-ONLY assertions of the same semantics
// RowStore was always meant to hold. The remaining legacy-map comparisons
// against session.ResourceCache/session.LazyResourceCache stay intact: those
// maps still exist (Stage 3 has not yet retired them), so the
// ListState.Rows/LazyResourceCache differential checks here still catch a
// genuine RowStore drift against a live sibling source.
//
// Event coverage this harness drives (see individual Test funcs below):
//  1. cache-loaded seed              (AvailabilityCacheLoaded, real disk rows AND placeholder-fallback/counts-only)
//  2. prefetch                       (AvailabilityPrefetched)
//  3. per-type probe result          (AvailabilityChecked)
//  4. list open + load-more append   (ActionCommand open, then ResourcesLoaded Append=false/true)
//  5. enrichment completion w/ FieldUpdates (EnrichmentChecked)
//  6. related lazy-add               (RelatedCheckResult.LazyAddedResources, both dual-write lanes)
//  7. pair switch                    (Session.Rotate)
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
)

// newDifferentialTestController mirrors newSeededTestController in
// app_cache_first_seeding_test.go (same package, same isolated-disk-store
// precondition) — duplicated as a small variant so this file has no
// cross-file coupling to another test file's helper lifetime.
func newDifferentialTestController(t *testing.T) (*session.Session, *runtime.Core, *app.Controller) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	return s, core, app.New(core)
}

func idSet(rows []resource.Resource) map[string]bool {
	out := make(map[string]bool, len(rows))
	for _, r := range rows {
		out[r.ID] = true
	}
	return out
}

func idSetsEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for id := range a {
		if !b[id] {
			return false
		}
	}
	return true
}

// assertRowStoreHasIDs asserts session.RowStore's retained rows for shortName
// have exactly the given ID set — the store-only replacement for the retired
// legacy-map differential check (task #17 wave 1 stage 2: session.
// ProbeResources/ProbeTruncated no longer exist to compare against).
func assertRowStoreHasIDs(t *testing.T, s *session.Session, shortName string, wantIDs ...string) {
	t.Helper()
	snap := s.RowStore.Snapshot(shortName)

	want := make(map[string]bool, len(wantIDs))
	for _, id := range wantIDs {
		want[id] = true
	}
	got := idSet(snap.Rows)
	if !idSetsEqual(want, got) {
		t.Errorf("type %q: RowStore.Snapshot(%q).Rows ID set = %v, want %v", shortName, shortName, got, want)
	}
}

// assertRowStoreMatchesListRows compares session.RowStore's rows for
// shortName against the Controller-visible ListState.Rows for that type's
// open list screen — the dual-write drift alarm for the canonical
// list-screen lane at Stage 1 (session.ResourceCache itself is a
// TUI-adapter-only write-through this Controller-level harness never
// populates; Stage 4 is what unifies ListState.Rows onto the store).
func assertRowStoreMatchesListRows(t *testing.T, s *session.Session, c *app.Controller, shortName string) {
	t.Helper()
	snap := s.RowStore.Snapshot(shortName)
	lb := c.Snapshot().Body.List
	if lb == nil {
		t.Fatalf("type %q: Snapshot().Body.List is nil — test must open the list screen before comparing", shortName)
	}
	legacyIDs := make(map[string]bool, len(lb.Rows))
	for _, r := range lb.Rows {
		legacyIDs[r.ResourceID] = true
	}
	storeIDs := idSet(snap.Rows)
	if !idSetsEqual(legacyIDs, storeIDs) {
		t.Errorf("type %q: RowStore rows %v != Controller ListState.Rows %v — dual-write drift", shortName, storeIDs, legacyIDs)
	}
}

// -----------------------------------------------------------------------
// Event 1 — cache-loaded seed (AvailabilityCacheLoaded)
// -----------------------------------------------------------------------

// TestDifferential_AvailabilityCacheLoaded_RealDiskRows_MatchesProbeResources
// drives the disk-cache-loaded seed event with a populated on-disk per-type
// file (real row data, not the placeholder fallback) and asserts RowStore
// retains exactly those rows (OriginDisk).
func TestDifferential_AvailabilityCacheLoaded_RealDiskRows_MatchesProbeResources(t *testing.T) {
	s, core, c := newDifferentialTestController(t)

	store := core.EnsureCacheStore()
	if store == nil {
		t.Fatal("core.EnsureCacheStore() = nil — test fixture requires a live disk store")
	}
	store.Put("ec2", cache.TypeFile{
		HasResources: true,
		Count:        1,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "i-diskrow-1", Name: "disk-row-1", Fields: map[string]string{"state": "running"}},
		},
	})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("seed fixture SaveType(ec2): %v", err)
	}

	_, _ = c.Handle(messages.AvailabilityCacheLoaded{
		Entries:   map[string]int{"ec2": 1},
		Truncated: map[string]bool{"ec2": false},
	})

	assertRowStoreHasIDs(t, s, "ec2", "i-diskrow-1")
	snap := s.RowStore.Snapshot("ec2")
	if len(snap.Rows) != 1 || snap.Rows[0].ID != "i-diskrow-1" {
		t.Errorf("RowStore.Snapshot(ec2).Rows = %+v, want [i-diskrow-1] seeded from the real on-disk row data", snap.Rows)
	}
}

// TestDifferential_AvailabilityCacheLoaded_PlaceholderFallback_IsCountsOnly
// drives the disk-cache-loaded seed event with NO on-disk per-type file (the
// placeholder-row fallback path) and asserts RowStore treats it as a
// counts-only observation (C6a: TotalCount set, Rows untouched/empty) — a
// placeholder-only fallback must never fabricate Rows in the store.
func TestDifferential_AvailabilityCacheLoaded_PlaceholderFallback_IsCountsOnly(t *testing.T) {
	s, _, c := newDifferentialTestController(t)

	_, _ = c.Handle(messages.AvailabilityCacheLoaded{
		Entries:   map[string]int{"ec2": 3},
		Truncated: map[string]bool{"ec2": false},
	})

	snap := s.RowStore.Snapshot("ec2")
	if snap.TotalCount != 3 {
		t.Errorf("RowStore.Snapshot(ec2).TotalCount = %d, want 3 to mirror the cache-loaded count (C6a)", snap.TotalCount)
	}
	if len(snap.Rows) != 0 {
		t.Errorf("RowStore.Snapshot(ec2).Rows = %+v, want empty — a placeholder-only fallback must never fabricate Rows in the store (C6a)", snap.Rows)
	}
}

// -----------------------------------------------------------------------
// Event 2 — prefetch (AvailabilityPrefetched)
// -----------------------------------------------------------------------

// TestDifferential_AvailabilityPrefetched_MatchesProbeResources drives the
// synchronous no-cache-mode prefetch path and asserts RowStore retains
// exactly the prefetched rows for the type (OriginFetch).
func TestDifferential_AvailabilityPrefetched_MatchesProbeResources(t *testing.T) {
	s, _, c := newDifferentialTestController(t)

	rows := []resource.Resource{
		{ID: "i-prefetch-1", Name: "prefetch-1", Type: "ec2"},
		{ID: "i-prefetch-2", Name: "prefetch-2", Type: "ec2"},
	}
	_, _ = c.Handle(messages.AvailabilityPrefetched{
		Entries:    map[string]int{"ec2": 2},
		Truncated:  map[string]bool{"ec2": false},
		Resources:  map[string][]resource.Resource{"ec2": rows},
		Pagination: map[string]*resource.PaginationMeta{"ec2": {IsTruncated: false}},
		Gen:        s.AvailabilityGen,
	})

	assertRowStoreHasIDs(t, s, "ec2", "i-prefetch-1", "i-prefetch-2")
}

// -----------------------------------------------------------------------
// Event 3 — per-type probe result (AvailabilityChecked)
// -----------------------------------------------------------------------

// TestDifferential_AvailabilityChecked_MatchesProbeResources drives one
// type's Wave 1 background-probe completion and asserts RowStore retains
// exactly the probed rows (OriginProbe).
func TestDifferential_AvailabilityChecked_MatchesProbeResources(t *testing.T) {
	s, _, c := newDifferentialTestController(t)

	rows := []resource.Resource{{ID: "i-probed-1", Name: "probed-1", Type: "ec2"}}
	_, _ = c.Handle(messages.AvailabilityChecked{
		ResourceType: "ec2",
		HasResources: true,
		Count:        1,
		Truncated:    false,
		Gen:          s.AvailabilityGen,
		Resources:    rows,
	})

	assertRowStoreHasIDs(t, s, "ec2", "i-probed-1")
}

// -----------------------------------------------------------------------
// Event 4 — list open + load-more append (ResourcesLoaded)
// -----------------------------------------------------------------------

// TestDifferential_ListOpen_ThenLoadMore_MatchesListRows opens a list screen,
// drives a first-page ResourcesLoaded, then a load-more append, and asserts
// RowStore's rows match the Controller-visible ListState.Rows at each step.
// Both lanes are driven by the SAME Controller.Handle call: Controller.Handle
// -> core.HandleEvent's messages.ResourcesLoaded case dual-writes RowStore
// (OriginFetch, Append-aware), while Controller.Handle's own
// handleResourcesLoadedEvent populates ListState.Rows.
func TestDifferential_ListOpen_ThenLoadMore_MatchesListRows(t *testing.T) {
	s, _, c := newDifferentialTestController(t)

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	page1 := []resource.Resource{
		{ID: "bucket-1", Name: "bucket-1", Type: "s3"},
		{ID: "bucket-2", Name: "bucket-2", Type: "s3"},
	}
	_, _ = c.Handle(messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    page1,
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-1"},
		Append:       false,
		Gen:          0,
	})
	assertRowStoreMatchesListRows(t, s, c, "s3")

	page2 := []resource.Resource{
		{ID: "bucket-3", Name: "bucket-3", Type: "s3"},
	}
	_, _ = c.Handle(messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    page2,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Append:       true,
		Gen:          0,
	})
	assertRowStoreMatchesListRows(t, s, c, "s3")

	final := s.RowStore.Snapshot("s3")
	if len(final.Rows) != 3 {
		t.Fatalf("after load-more append, RowStore.Snapshot(s3).Rows = %+v, want 3 rows total", final.Rows)
	}
}

// -----------------------------------------------------------------------
// Event 5 — enrichment completion with FieldUpdates (EnrichmentChecked)
// -----------------------------------------------------------------------

// rowstoreDiffSentinelWave2Type is a catalog-absent Wave-2 short name used to
// keep the enrichment queue open past the "ec2" completion in
// TestDifferential_EnrichmentChecked_FieldUpdates_MatchesProbeResources. A
// single-type queue would make that one EnrichmentChecked delivery the FINAL
// one (EnrichChecked >= EnrichTotal) — see
// TestDifferential_EnrichmentChecked_AllDone_RowStoreSurvivesLegacyFree for
// the dedicated all-done-completion pin. Registering this second sentinel
// keeps EnrichTotal at 2 so the ec2 delivery is a genuine partial-completion,
// matching this test's stated intent (assert RowStore's own amended rows
// mid-sweep, before the queue drains).
const rowstoreDiffSentinelWave2Type = "rowstore-diff-sentinel-wave2"

// TestDifferential_EnrichmentChecked_FieldUpdates_MatchesProbeResources
// drives a Wave 2 enrichment completion carrying FieldUpdates and asserts
// RowStore's amended rows (via AmendRows, task #17 wave 1 stage 2's
// replacement for the removed session.ProbeResources applyEnrichment fold)
// carry the merged FieldUpdates values — the copy-on-write Amend must
// reproduce applyEnrichment's FieldUpdates maps.Copy behavior.
func TestDifferential_EnrichmentChecked_FieldUpdates_MatchesProbeResources(t *testing.T) {
	awsclient.SetWave2EnricherForTest(t, rowstoreDiffSentinelWave2Type, awsclient.IssueEnricher{
		Fn:       awsclient.InFetcherWave2Sentinel,
		Priority: 100,
	})

	s, _, c := newDifferentialTestController(t)

	seed := []resource.Resource{
		{ID: "i-enrich-1", Name: "enrich-1", Type: "ec2", Fields: map[string]string{"state": "running"}},
	}
	_, _ = c.Handle(messages.AvailabilityChecked{
		ResourceType: "ec2",
		HasResources: true,
		Count:        1,
		Gen:          s.AvailabilityGen,
		Resources:    seed,
	})

	// Seed the sentinel type's own RowStore entry too — BuildEnrichQueue
	// (internal/runtime/probes.go) only enqueues a Wave-2 entry when
	// RowStore.Snapshot(type).Gen != 0 (observed-at-all).
	_, _ = c.Handle(messages.AvailabilityChecked{
		ResourceType: rowstoreDiffSentinelWave2Type,
		HasResources: true,
		Count:        1,
		Gen:          s.AvailabilityGen,
		Resources: []resource.Resource{
			{ID: "sentinel-1", Name: "sentinel-1", Type: rowstoreDiffSentinelWave2Type},
		},
	})

	if s.EnrichTotal != 2 {
		t.Fatalf("precondition: want EnrichTotal=2 (ec2 + sentinel) after both AvailabilityChecked deliveries drained the avail queue and startEnrichment ran, got %d", s.EnrichTotal)
	}

	_, _ = c.Handle(messages.EnrichmentChecked{
		ResourceType: "ec2",
		Issues:       1,
		Gen:          s.EnrichmentGen,
		TypeGen:      s.EnrichmentTypeGen["ec2"],
		FieldUpdates: map[string]map[string]string{
			"i-enrich-1": {"cost_estimate": "12.50"},
		},
	})

	if s.EnrichChecked >= s.EnrichTotal {
		t.Fatalf("precondition: want a partial enrichment completion (EnrichChecked < EnrichTotal) so handleEnrichmentChecked's all-done free does not fire; got EnrichChecked=%d EnrichTotal=%d", s.EnrichChecked, s.EnrichTotal)
	}

	assertRowStoreHasIDs(t, s, "ec2", "i-enrich-1")

	snap := s.RowStore.Snapshot("ec2")
	if len(snap.Rows) != 1 || snap.Rows[0].Fields["cost_estimate"] != "12.50" {
		t.Errorf("RowStore.Snapshot(ec2).Rows = %+v, want Fields[cost_estimate]=12.50 from the applyEnrichment FieldUpdates fold", snap.Rows)
	}
}

// TestDifferential_EnrichmentChecked_AllDone_RowStoreSurvivesLegacyFree pins
// the D12-class survival behavior RowStore exists for: when a single-type
// enrichment queue drains on the FIRST EnrichmentChecked delivery
// (handleEnrichmentChecked's "all done" branch,
// internal/runtime/handlers_availability.go), RowStore's rows for that type
// are retained after the sweep completes — task #17 wave 1 stage 2 removed
// the legacy session.ProbeResources/ProbeTruncated free this branch used to
// perform, so there is no map-nil race left to survive; the row set the
// enrichment fold merged into RowStore via AmendRows earlier in the same
// handler call must simply still be there.
func TestDifferential_EnrichmentChecked_AllDone_RowStoreSurvivesLegacyFree(t *testing.T) {
	s, _, c := newDifferentialTestController(t)

	seed := []resource.Resource{
		{ID: "i-enrich-2", Name: "enrich-2", Type: "ec2", Fields: map[string]string{"state": "running"}},
	}
	_, _ = c.Handle(messages.AvailabilityChecked{
		ResourceType: "ec2",
		HasResources: true,
		Count:        1,
		Gen:          s.AvailabilityGen,
		Resources:    seed,
	})

	if s.EnrichTotal != 1 {
		t.Fatalf("precondition: want EnrichTotal=1 (ec2 only) so the next EnrichmentChecked is the terminal one, got %d", s.EnrichTotal)
	}

	_, _ = c.Handle(messages.EnrichmentChecked{
		ResourceType: "ec2",
		Issues:       1,
		Gen:          s.EnrichmentGen,
		TypeGen:      s.EnrichmentTypeGen["ec2"],
		FieldUpdates: map[string]map[string]string{
			"i-enrich-2": {"cost_estimate": "9.99"},
		},
	})

	if s.EnrichChecked < s.EnrichTotal {
		t.Fatalf("precondition: want the all-done branch to have fired (EnrichChecked >= EnrichTotal), got EnrichChecked=%d EnrichTotal=%d", s.EnrichChecked, s.EnrichTotal)
	}

	snap := s.RowStore.Snapshot("ec2")
	if len(snap.Rows) != 1 || snap.Rows[0].Fields["cost_estimate"] != "9.99" {
		t.Errorf("RowStore.Snapshot(ec2).Rows = %+v, want the merged cost_estimate=9.99 row to survive enrichment-sweep completion", snap.Rows)
	}
}

// -----------------------------------------------------------------------
// Event 6 — related lazy-add (RelatedCheckResult.LazyAddedResources)
// -----------------------------------------------------------------------

// TestDifferential_RelatedLazyAdd_MatchesLazyResourceCache drives a related
// lazy-add result (e.g. a KMS customer-managed-key pivot outside the
// top-level fetcher's filter) through BOTH lanes that legitimately exist
// for this message today:
//
//  1. runtime.Core.HandleRelatedCheckResult (the intent-returning method) +
//     Controller.ApplyIntents — this is what populates RowStore's Partial
//     entry (via PatchLazyResourceCache, task #17 wave 1 stage 3: the
//     legacy session.LazyResourceCache map this intent used to write is
//     gone; the intent now feeds RowStore.ObservePartial), exactly as the
//     TUI adapter and the headless RelatedCheckBatch executor do.
//  2. app.Controller.Handle(messages.RelatedCheckResult{...}) — this is what
//     feeds RowStore (via core.HandleEvent's dedicated
//     observeRelatedCheckResultRows case, deliberately side-effect-only so
//     Controller.Handle never double-applies path 1's intents for a bare
//     RelatedCheckResult).
//
// A real production caller normally only exercises ONE of these two paths
// per message (TUI/headless-executor calls HandleRelatedCheckResult
// directly; a hypothetical generic HandleEvent-only caller would only get
// RowStore's side). Driving both here is deliberate: it is the only way to
// observe the two RowStore-writing lanes side by side and catch them
// drifting apart.
func TestDifferential_RelatedLazyAdd_MatchesLazyResourceCache(t *testing.T) {
	s, core, c := newDifferentialTestController(t)

	lazyRows := []resource.Resource{
		{ID: "key-lazy-1", Name: "lazy-key-1", Type: "kms"},
	}

	intents, _ := core.HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
		ResourceType:       "ec2",
		SourceResourceID:   "i-source-1",
		DefDisplayName:     "KMS Keys",
		LazyAddedResources: map[string][]resource.Resource{"kms": lazyRows},
	})
	c.ApplyIntents(intents)

	_, _ = c.Handle(messages.RelatedCheckResult{
		ResourceType:       "ec2",
		SourceResourceID:   "i-source-1",
		DefDisplayName:     "KMS Keys",
		Generation:         s.RelatedGen,
		LazyAddedResources: map[string][]resource.Resource{"kms": lazyRows},
	})

	afterLane1 := s.RowStore.Snapshot("kms")
	if len(afterLane1.Rows) != 1 || afterLane1.Rows[0].ID != "key-lazy-1" {
		t.Fatalf("precondition: RowStore.Snapshot(kms) after lane 1 (PatchLazyResourceCache) = %+v, want [key-lazy-1]", afterLane1.Rows)
	}

	all := s.RowStore.SnapshotAll(true)
	storeSnap := all["kms"]
	storeIDs := idSet(storeSnap.Rows)
	if !storeIDs["key-lazy-1"] {
		t.Errorf("RowStore partial view for kms = %+v, want key-lazy-1 present — dual-write drift against legacy LazyResourceCache", storeSnap.Rows)
	}
	if !storeSnap.Partial {
		t.Error("RowStore SnapshotAll(true)[kms].Partial = false, want true — a lazy-add observation must mark Partial")
	}

	// Scope boundary (C6): a lazy-add row must never poison the canonical
	// (non-partial) view of a type it was added under.
	canonical := s.RowStore.SnapshotAll(false)
	if tr, ok := canonical["kms"]; ok {
		for _, r := range tr.Rows {
			if r.ID == "key-lazy-1" {
				t.Error("lazy-added row leaked into the canonical (non-partial) RowStore view — violates C6 scope boundary")
			}
		}
	}
}

// -----------------------------------------------------------------------
// Event 7 — pair switch (Session.Rotate)
// -----------------------------------------------------------------------

// TestDifferential_PairSwitch_ClearsRowStoreLikeLegacyMaps drives a Wave-1
// probe result (populating RowStore), then rotates the session
// (profile/region switch) and asserts RowStore ends up empty — never
// observed again (Gen==0), not merely empty-Rows (C9 pair isolation).
func TestDifferential_PairSwitch_ClearsRowStoreLikeLegacyMaps(t *testing.T) {
	s, _, c := newDifferentialTestController(t)

	_, _ = c.Handle(messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: true,
		Count:        1,
		Gen:          s.AvailabilityGen,
		Resources:    []resource.Resource{{ID: "bucket-preswitch-1", Type: "s3"}},
	})
	assertRowStoreHasIDs(t, s, "s3", "bucket-preswitch-1")

	s.Profile = "other-profile"
	s.Region = "eu-west-1"
	s.Rotate()

	postSnap := s.RowStore.Snapshot("s3")
	if postSnap.Gen != 0 {
		t.Errorf("RowStore.Snapshot(s3).Gen after pair switch = %d, want 0 (never observed this new session) — C9", postSnap.Gen)
	}
	if len(postSnap.Rows) != 0 {
		t.Errorf("RowStore.Snapshot(s3).Rows after pair switch = %+v, want empty (C9)", postSnap.Rows)
	}
}
