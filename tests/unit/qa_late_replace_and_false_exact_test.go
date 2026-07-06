// qa_late_replace_and_false_exact_test.go — RED pins for DEF-18.
//
// Defect (observed live; coder root-causing in parallel — coordinate via
// the mechanisms below):
//
//  A) A staler page-1 REPLACE landing after a deeper load-more append
//     stomps the 55-row list back to 50+ (C2: older results must be
//     discarded).
//
//  B) The persisted file got count:50, exact:true, rows:0 on a 55-bucket
//     account: a page-1 session.ResourceCache entry stored WITHOUT its
//     Pagination (HandleResourcesLoaded's PatchResourceCache at
//     internal/runtime/handlers_resources.go builds
//     Entry{Resources: ev.Resources} — no Pagination) makes
//     availabilityFromResourceCache derive truncated=false for a page-1-
//     shaped entry, producing a false-exact 50 that SaveAvailabilityCache
//     (internal/runtime/probes.go) then accepts as a downgrade of the
//     previously-stored true-exact 55, dropping the fuller Rows.
//
// Pins (harnesses: qa_load_more_dedup_test.go's poisoning-sequence shape +
// runtime_executor_depth_refetch_test.go's seedCachedRows/bucketID/
// page1Resources/page2Resources package-level helpers, reused directly —
// both already live in this package, unit_test):
//
//  1. FalseExact_PageOneEntryWithoutPagination_NeverDowngradesExact
//  2. NilPaginationEntry_IsNotExact
//  3. LateReplace_DoesNotStompDeeperList
//  4. FreshReplace_StillWins
package unit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
)

// ────────────────────────────────────────────────────────────────────────────
// Test 1 — a page-1 entry saved without Pagination must never downgrade an
// already-persisted true-exact total.
// ────────────────────────────────────────────────────────────────────────────

// TestFalseExact_PageOneEntryWithoutPagination_NeverDowngradesExact drives
// the REAL Core.HandleResourcesLoaded handler (internal/runtime/
// handlers_resources.go) on a fresh Core with no cached entry for "s3" yet
// — the exact !alreadyCached branch that builds the PatchResourceCache
// intent — with ev.Pagination reporting a truncated 50-row first page (the
// live-observed shape). The returned intent is applied exactly as
// production applies it (Controller.ApplyIntents), then TaskKindSaveCache
// is run through the real executor (Core.ExecuteTask), mirroring the real
// background-sweep save path.
//
// The on-disk TypeFile for "s3" is pre-seeded (via seedCachedRows, reused
// from runtime_executor_depth_refetch_test.go) with a true-exact 55-row
// state, as if an earlier full-pagination sweep had already completed.
//
// RED today: the false-exact 50-row derivation downgrades the persisted
// 55-row/exact state to 50/exact, dropping 5 rows.
func TestFalseExact_PageOneEntryWithoutPagination_NeverDowngradesExact(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	// Pre-seed the disk with a true-exact 55-row state. seedCachedRows
	// (reused from runtime_executor_depth_refetch_test.go) writes under the
	// fixed depthTestProfile/depthTestRegion pair — use the same pair below
	// so the save/read-back in this test targets the same on-disk file.
	seedCachedRows(t, "s3", 55, true)

	s := session.New()
	s.Profile = depthTestProfile
	s.Region = depthTestRegion
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := app.New(core)
	t.Cleanup(ctrl.Close)

	// Drive the real handler: a page-1 fetch result for "s3", truncated,
	// with no existing session.ResourceCache entry (!alreadyCached) — the
	// exact condition that builds Entry{Resources: ev.Resources} at
	// handlers_resources.go:74-82.
	intents, _ := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: "s3",
		Resources:    page1Resources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "p2",
			TotalHint:   -1,
			PageSize:    50,
		},
		Append: false,
	})
	foundPatch := false
	for _, in := range intents {
		if _, ok := in.(runtime.PatchResourceCache); ok {
			foundPatch = true
		}
	}
	if !foundPatch {
		t.Fatal("Core.HandleResourcesLoaded did not emit a PatchResourceCache intent for a first-time cache entry — cannot exercise the false-exact derivation without it")
	}
	ctrl.ApplyIntents(intents)

	got, ok := core.ResourceCache("s3")
	if !ok || got == nil {
		t.Fatal(`core.ResourceCache("s3") missing after applying PatchResourceCache — session.ResourceCache was not populated`)
	}
	if len(got.Resources) != 50 {
		t.Fatalf("session.ResourceCache[s3].Resources has %d entries, want 50 (page 1) — cannot exercise the derivation with the wrong fixture shape", len(got.Resources))
	}

	// Run the real save-cache executor path (mirrors the background sweep
	// completion dispatching TaskKindSaveCache).
	ctx := context.Background()
	if _, err := core.ExecuteTask(ctx, runtime.TaskRequest{
		Key: runtime.TaskKey{Kind: runtime.TaskKindSaveCache},
	}); err != nil {
		t.Fatalf("ExecuteTask(TaskKindSaveCache): %v", err)
	}

	store := cache.LoadDir(depthTestProfile, depthTestRegion)
	tf, ok := store.Type("s3")
	if !ok {
		t.Fatal(`cache.LoadDir(...).Type("s3") missing after TaskKindSaveCache`)
	}
	if tf.Count != 55 || !tf.Exact || len(tf.Rows) != 55 {
		t.Errorf("persisted s3 TypeFile after a page-1-without-Pagination entry raced a save = {Count:%d Exact:%v len(Rows):%d}, want {Count:55 Exact:true len(Rows):55} — DEF-18: a page-1 entry stored without its Pagination must never be treated as an exact observation that downgrades an already-persisted true-exact total", tf.Count, tf.Exact, len(tf.Rows))
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 2 — availabilityFromResourceCache-derived Exact must never come from
// a nil-Pagination entry.
// ────────────────────────────────────────────────────────────────────────────

// TestNilPaginationEntry_IsNotExact isolates the derivation itself (as
// opposed to test 1's full HandleResourcesLoaded-to-disk round trip):
// directly seed a session.ResourceCache entry with a nil Pagination field
// (the exact shape HandleResourcesLoaded's PatchResourceCache intent
// carries) and confirm the persisted save never marks that type Exact,
// preserving whatever was already stored on disk (or leaving it unknown
// when nothing was stored).
//
// RED today: SaveAvailabilityCache treats the nil-Pagination entry's
// derived truncated=false as a genuine exact observation and persists
// Exact=true with the entry's own (possibly incomplete) count — even when
// no prior exact state existed to protect, this proves the derivation
// itself, not just the downgrade-guard interaction, is wrong.
func TestNilPaginationEntry_IsNotExact(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	// Seed a true-exact 55-row prior state so a false Exact=true here is
	// observable as a downgrade, matching the live-observed symptom shape.
	// seedCachedRows writes under the fixed depthTestProfile/depthTestRegion
	// pair (reused from runtime_executor_depth_refetch_test.go) — use the
	// same pair below.
	seedCachedRows(t, "s3", 55, true)

	s := session.New()
	s.Profile = depthTestProfile
	s.Region = depthTestRegion
	core := runtime.New(s, resource.AllResourceTypes())

	// Directly seed session.ResourceCache with the exact shape
	// HandleResourcesLoaded's PatchResourceCache intent carries: Resources
	// set, Pagination nil.
	core.SetResourceCache("s3", &domain.ListViewCacheEntry{
		Resources: page1Resources(50),
	})

	ctx := context.Background()
	if _, err := core.ExecuteTask(ctx, runtime.TaskRequest{
		Key: runtime.TaskKey{Kind: runtime.TaskKindSaveCache},
	}); err != nil {
		t.Fatalf("ExecuteTask(TaskKindSaveCache): %v", err)
	}

	store := cache.LoadDir(depthTestProfile, depthTestRegion)
	tf, ok := store.Type("s3")
	if !ok {
		t.Fatal(`cache.LoadDir(...).Type("s3") missing after TaskKindSaveCache`)
	}
	if tf.Exact && tf.Count == 50 {
		t.Errorf("persisted s3 TypeFile = {Count:%d Exact:%v}, want the prior exact 55 preserved (Exact=true, Count=55) — DEF-18: a nil-Pagination session.ResourceCache entry must never be treated as an exact observation", tf.Count, tf.Exact)
	}
	if tf.Count != 55 || !tf.Exact || len(tf.Rows) != 55 {
		t.Errorf("persisted s3 TypeFile after a nil-Pagination entry raced a save = {Count:%d Exact:%v len(Rows):%d}, want {Count:55 Exact:true len(Rows):55} (prior exact state preserved)", tf.Count, tf.Exact, len(tf.Rows))
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 3 — a late page-1 replace must not stomp a deeper, already-loaded
// list (DEF-18 mechanism A).
// ────────────────────────────────────────────────────────────────────────────

// TestLateReplace_DoesNotStompDeeperList drives the headless controller via
// Controller.Handle(messages.ResourcesLoaded{...}) — the real task-result
// lane (per runtime_cache_rows_exact_totals_test.go's Contract D precedent),
// not the ApplyResourcesLoaded test-only bypass, so syncExactTotalToMenu
// fires and the menu-availability/title-derived count is also exercised:
// page 1 (50, truncated, token) lands with Append=false, then page 2 (5,
// exact) with Append=true — landing the list at 55 exact, matching the
// pre-existing DEF-17 dedup contract.
//
// Then a LATE page-1 replace arrives (Append=false) carrying the SAME 50
// page-1 IDs, still truncated — the exact shape a straggling background
// verify-refetch racing behind the foreground load-more would produce
// (C2: a result older than a later invalidation must be discarded). The
// list must REMAIN at 55 rows/exact — the late replace must be rejected,
// not silently accepted as a fresher truth.
//
// RED today: internal/app/list_body.go's applyResourcesLoaded replaces
// ls.Rows unconditionally on append=false (`ls.Rows = resources`), with no
// check for whether the incoming page is an older, shallower subset of
// what is already on screen.
func TestLateReplace_DoesNotStompDeeperList(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	const profile, region = "def18-late-replace-profile", "us-east-1"

	s := session.New()
	s.Profile = profile
	s.Region = region
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := app.New(core)
	t.Cleanup(ctrl.Close)

	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	page1 := page1Resources(50)
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    page1,
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "p2",
			TotalHint:   -1,
			PageSize:    50,
		},
		Append: false,
		Gen:    0, // AcceptZeroGen=true
	})

	page2 := page2Resources(50, 5)
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    page2,
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			NextToken:   "",
			TotalHint:   55,
			PageSize:    5,
		},
		Append: true,
		Gen:    0,
	})

	preSnap := ctrl.Snapshot()
	preLB := preSnap.Body.List
	if preLB == nil || len(preLB.Rows) != 55 {
		gotLen := -1
		if preLB != nil {
			gotLen = len(preLB.Rows)
		}
		t.Fatalf("setup: after page1+page2, Body.List.Rows has %d entries, want 55 — cannot exercise the late-replace stomp without the deeper list already loaded", gotLen)
	}
	if got, want := ctrl.GetMenuAvailability()["s3"], 55; got != want {
		t.Fatalf("setup: GetMenuAvailability()[\"s3\"] = %d after page1+page2, want %d — cannot exercise the regression check without the exact total already synced", got, want)
	}

	// The late replace: a straggling background result carrying the SAME
	// page-1 IDs again, still truncated — exactly what a verify-refetch
	// dispatched before the load-more (but resolved after it) would
	// produce.
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    page1Resources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "p2",
			TotalHint:   -1,
			PageSize:    50,
		},
		Append: false,
		Gen:    0,
	})

	snap := ctrl.Snapshot()
	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after the late-replace sequence")
	}
	if len(lb.Rows) != 55 {
		t.Errorf("Body.List.Rows has %d entries after a late page-1 replace, want 55 (unchanged) — DEF-18: a staler page-1 replace must not stomp a deeper, already-loaded list", len(lb.Rows))
	}
	avail := ctrl.GetMenuAvailability()
	trunc := ctrl.GetMenuTruncated()
	if got, want := avail["s3"], 55; got != want {
		t.Errorf(`GetMenuAvailability()["s3"] = %d after a late page-1 replace, want %d (unchanged) — the menu/frame-title-derived count must not regress either`, got, want)
	}
	if trunc["s3"] {
		t.Error(`GetMenuTruncated()["s3"] = true after a late page-1 replace, want false — the exact 55 total must not be re-marked truncated by a stale replace`)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 4 — a FRESH replace carrying genuinely new content must still win
// (guard against over-blocking replaces).
// ────────────────────────────────────────────────────────────────────────────

// TestFreshReplace_StillWins guards against a fix for test 3 that is too
// aggressive — e.g. rejecting every smaller/truncated append=false replace
// regardless of content. A replace whose incoming IDs are NOT a subset of
// what is already on screen (e.g. after a manual refresh where the
// underlying account's bucket set changed) must still swap the list
// normally.
//
// This models a refresh landing a page 1 with different IDs than the
// previously-shown 55 rows — e.g. after buckets were added/removed
// upstream between the two fetches — so the replace is genuinely fresh
// content, not a stale straggler.
func TestFreshReplace_StillWins(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	const profile, region = "def18-fresh-replace-profile", "us-east-1"

	s := session.New()
	s.Profile = profile
	s.Region = region
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := app.New(core)
	t.Cleanup(ctrl.Close)

	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	page1 := page1Resources(50)
	ctrl.ApplyResourcesLoaded("s3", page1, &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "p2",
		TotalHint:   -1,
		PageSize:    50,
	}, false)

	page2 := page2Resources(50, 5)
	ctrl.ApplyResourcesLoaded("s3", page2, &resource.PaginationMeta{
		IsTruncated: false,
		NextToken:   "",
		TotalHint:   55,
		PageSize:    5,
	}, true)

	preSnap := ctrl.Snapshot()
	if preSnap.Body.List == nil || len(preSnap.Body.List.Rows) != 55 {
		t.Fatal("setup: list did not reach 55 rows before the fresh-refresh replace")
	}

	// A fresh manual refresh: page 1 again, but with 30 DIFFERENT bucket
	// IDs (disjoint from the prior page1+page2 IDs) and exact (untruncated)
	// — a genuine smaller result, not a stale straggler duplicate.
	freshIDs := make([]resource.Resource, 30)
	for i := range freshIDs {
		freshIDs[i] = resource.Resource{ID: "fresh-bucket-" + bucketID(i), Name: "fresh-bucket-" + bucketID(i), Type: "s3"}
	}
	ctrl.ApplyResourcesLoaded("s3", freshIDs, &resource.PaginationMeta{
		IsTruncated: false,
		NextToken:   "",
		TotalHint:   30,
		PageSize:    30,
	}, false)

	snap := ctrl.Snapshot()
	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after the fresh-replace sequence")
	}
	if len(lb.Rows) != 30 {
		t.Errorf("Body.List.Rows has %d entries after a fresh exact 30-row refresh replaced the prior 55, want 30 — a genuinely fresh replace (disjoint IDs, exact) must still swap the list normally, not be blocked by an over-aggressive late-replace guard", len(lb.Rows))
	}
	for _, r := range lb.Rows {
		if !strings.HasPrefix(r.ResourceID, "fresh-bucket-") {
			t.Errorf("Body.List row ID %q after the fresh replace does not carry the fresh-bucket- prefix — stale page1/page2 rows leaked through instead of being replaced", r.ResourceID)
			break
		}
	}
}
