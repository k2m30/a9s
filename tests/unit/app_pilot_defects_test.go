// app_pilot_defects_test.go — RED pins for the seven root-caused defects
// found by the live S3 pilot (docs/design/cache-requirements.md §4), which
// FAILED 6/12 acceptance steps. Each defect below is pinned at the
// controller/runtime seam the coordinator's handoff identified. The contract
// doc (docs/design/cache-requirements.md) is authoritative; comments below
// cite the specific rule (C3/C4/C5/C6/C7) each pin locks in.
//
// Investigation note (DEF-3): a static trace of the current committed
// production wiring (internal/app/handle.go's maybeSaveResourceListCache,
// internal/runtime/probes.go's SaveResourceListCache, and
// internal/runtime/handlers_availability.go's rowsFromCacheRows) shows
// Findings ARE threaded through save (resource.Resource.Findings ->
// cache.Row.Findings) and seed-back (cache.Row.Findings ->
// resource.Resource.Findings) on the straightforward top-level list-open
// path — this differs from the dispatch's literal description ("production
// save writes rows as id/name/fields only"). TestSaveResourceListCache_
// FindingsSurviveWiredSaveAndColdBootReseed below pins the full contract
// end-to-end (save via the real wiring seam, reload from disk, reseed a
// fresh controller, and assert both the raw Findings AND the render-derived
// glyph/severity survive). If this comes back green, it stands as a
// regression guard for a already-correct contract, not a defect pin, and
// this is reported explicitly rather than pinning a fabricated red.
package unit_test

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// -----------------------------------------------------------------------
// DEF-1 (C4) — a cold list open with a connected client must not block the
// transport when the target screen is already renderable (cached rows
// seeded, or the Loading shell already present).
//
// Pinned seam: a NEW function app.IsBackgroundFetchTask(req, screenAlready
// Renderable) — a context-aware sibling of app.IsBackgroundTaskKind, taking
// the full runtime.TaskRequest plus an explicit "the screen the fetch
// targets is already showing content" bool, so a KindFetchResources task can
// be classified background exactly when the pilot's step 7 requires it
// (cached rows already on screen, Refreshing=true) while staying blocking
// for the genuinely first-ever cold render (step 2's `Loading…` case, where
// nothing renderable exists yet and the response must still carry SOME
// shell — DrainSyncPartition already returns that shell synchronously
// before background tasks are deferred, so this does not regress step 2).
//
// This keeps app.IsBackgroundTaskKind (and the existing
// TestIsBackgroundTaskKind_TableAllKnownKinds pin, and DrainSyncPartition's
// isBackground func(runtime.TaskKind) bool parameter used for the other 4
// kinds) completely untouched — no ripple into the generic partition
// machinery's signature. A future coder wires web's handleAction to consult
// IsBackgroundFetchTask specifically for KindFetchResources requests (kept
// out of the generic isBackground callback passed to DrainSyncPartition,
// since that callback only receives a TaskKind, not screen-renderability
// context) before/instead of the existing IsBackgroundTaskKind check.
// -----------------------------------------------------------------------

// TestIsBackgroundFetchTask_CachedRowsSeeded_IsBackground pins the new
// classifier's core case: a KindFetchResources task targeting a screen that
// already has cached rows on it (Refreshing=true is the seeded-list signal,
// per ListState/ListBody's documented contract) must be classified
// background so the caller can defer it instead of blocking the response.
func TestIsBackgroundFetchTask_CachedRowsSeeded_IsBackground(t *testing.T) {
	req := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "s3"}}
	if !app.IsBackgroundFetchTask(req, true) {
		t.Error("IsBackgroundFetchTask(KindFetchResources, screenAlreadyRenderable=true) = false, want true — DEF-1/C4: a list-content fetch over an already-renderable screen (cached rows or Loading shell present) must never block the transport")
	}
}

// TestIsBackgroundFetchTask_ColdNoRenderableContent_StaysBlocking pins the
// non-regression half: a truly cold fetch (nothing renderable yet) must
// remain blocking, since DrainSyncPartition's synchronous half is what
// produces the `Loading…` shell in the same response (pilot step 2) — a
// background-only cold fetch would return an EMPTY response with no shell.
func TestIsBackgroundFetchTask_ColdNoRenderableContent_StaysBlocking(t *testing.T) {
	req := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "s3"}}
	if app.IsBackgroundFetchTask(req, false) {
		t.Error("IsBackgroundFetchTask(KindFetchResources, screenAlreadyRenderable=false) = true, want false — a genuinely cold fetch (no Loading shell, no cached rows) must stay blocking so the response carries the shell")
	}
}

// TestIsBackgroundFetchTask_NonFetchKind_UnaffectedByRenderability pins that
// the new classifier only changes behavior for KindFetchResources — every
// other TaskKind's classification must be identical to
// app.IsBackgroundTaskKind's existing verdict regardless of the
// screenAlreadyRenderable argument, so this new function is additive, not a
// silent behavior change for the 4 already-background kinds or any other
// blocking kind.
func TestIsBackgroundFetchTask_NonFetchKind_UnaffectedByRenderability(t *testing.T) {
	kinds := []runtime.TaskKind{
		runtime.KindRelatedCheck, runtime.KindEnrichDetail,
		runtime.TaskKindProbeEnrich, runtime.TaskKindSaveCache,
		runtime.KindFetchFiltered, runtime.KindFetchMore,
		runtime.TaskKindProbeAvailability, runtime.TaskKindConnect,
	}
	for _, k := range kinds {
		req := runtime.TaskRequest{Key: runtime.TaskKey{Kind: k}}
		want := app.IsBackgroundTaskKind(k)
		if got := app.IsBackgroundFetchTask(req, true); got != want {
			t.Errorf("IsBackgroundFetchTask(%q, true) = %v, want %v (must match IsBackgroundTaskKind for non-fetch kinds)", k, got, want)
		}
		if got := app.IsBackgroundFetchTask(req, false); got != want {
			t.Errorf("IsBackgroundFetchTask(%q, false) = %v, want %v (must match IsBackgroundTaskKind for non-fetch kinds)", k, got, want)
		}
	}
}

// TestWebBoot_WarmListOpen_FetchTaskDeferredAsBackground pins DEF-1
// end-to-end at the seam a web request handler would actually use: a
// controller seeded with cache-first rows for s3 (mirroring pilot step 7 —
// "cached rows... render < 100ms... ⟟ marker") opens the s3 list, and the
// resulting KindFetchResources task — run through the real
// DrainSyncPartition machinery with an isBackground callback built from
// IsBackgroundFetchTask bound to the post-Apply screen-renderable state —
// must land in the DEFERRED slice, never executed synchronously.
func TestWebBoot_WarmListOpen_FetchTaskDeferredAsBackground(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	core, ctrl := newLiveWebStyleController(t, "pilot-def1-prof", "us-east-1")

	// Seed a REAL on-disk per-type file for s3 so the cache-first list-open
	// path (NavigateKindPushResourceList's RowStore/OriginDisk seed) has
	// genuine row data to seed from — mirrors a warm boot per pilot step 5.
	// task #17 wave 1 stage 2 / C6a: a counts-only AvailabilityCacheLoaded
	// with no real disk row data never fabricates placeholder Rows (see
	// TestWebBoot_AvailabilityCacheLoaded_CountsOnlyFallback_KeepsLoadingTrue
	// in app_web_live_cold_boot_test.go), so this test needs a real store to
	// exercise a genuinely-renderable warm open.
	store := core.EnsureCacheStore()
	if store == nil {
		t.Fatal("core.EnsureCacheStore() = nil — test fixture requires a live disk store to seed rows into")
	}
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        3,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "bucket-pilot-def1-1", Name: "bucket-pilot-def1-1"},
			{ID: "bucket-pilot-def1-2", Name: "bucket-pilot-def1-2"},
			{ID: "bucket-pilot-def1-3", Name: "bucket-pilot-def1-3"},
		},
	})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("seed fixture SaveType(s3): %v", err)
	}

	ctrl.Handle(messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"s3": 3},
	})

	_, tasks := ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	snap := ctrl.Snapshot()
	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening a warm-seeded s3 list")
	}
	if lb.Loading {
		t.Fatal("test setup: Loading=true — expected a warm (cache-seeded) list open with rows already renderable, not a cold Loading shell")
	}

	renderable := !lb.Loading
	isBackground := func(kind runtime.TaskKind) bool {
		return app.IsBackgroundFetchTask(runtime.TaskRequest{Key: runtime.TaskKey{Kind: kind}}, renderable)
	}

	deferred := app.DrainSyncPartition(context.Background(), ctrl, tasks, isBackground, nil)
	hasDeferredFetch := false
	for _, d := range deferred {
		if d.Key.Kind == runtime.KindFetchResources {
			hasDeferredFetch = true
			break
		}
	}
	if !hasDeferredFetch {
		t.Error("KindFetchResources was not deferred to the background partition for a warm (already-renderable) list open — DEF-1/C4: the transport must not block on this fetch when cached rows are already on screen")
	}
}

// -----------------------------------------------------------------------
// DEF-2 (C5) — the availability sweep's truncated probe result must never
// downgrade an already-exact stored total in the LIVE menu view-state
// (session.MenuState via PatchMenuAvailability), mirroring the guard that
// already exists on the disk-persist path (SaveAvailabilityCache /
// SaveResourceListCache in internal/runtime/probes.go).
//
// Pinned seam: internal/app/intents.go's PatchMenuAvailability case (called
// from internal/runtime/handlers_availability.go's handleAvailabilityChecked)
// currently overwrites ms.Availability[type]/ms.Truncated[type]
// unconditionally with the incoming Count/Truncated — no comparison against
// the current stored exactness. This test drives the REAL AvailabilityChecked
// event (what a background sweep's probe emits) after an already-exact 55
// through Controller.Handle and asserts the exact 55 (and its exactness)
// survive a truncated 50 landing mid-sweep.
// -----------------------------------------------------------------------

// TestAvailabilityChecked_TruncatedProbe_NeverDowngradesExactMenuTotal pins
// DEF-2 directly: a menu holding an exact 55 for s3 (Truncated=false) that
// then receives an AvailabilityChecked{Count:50, Truncated:true} for s3 (the
// availability sweep's truncated first-page result) must retain
// Availability["s3"]==55 and Truncated["s3"]==false — not regress to 50/true.
func TestAvailabilityChecked_TruncatedProbe_NeverDowngradesExactMenuTotal(t *testing.T) {
	core, ctrl := newLiveWebStyleController(t, "", "us-east-1")

	// Establish the exact baseline the pilot's step 3/4 produce ("s3(55)",
	// exact, no "+") via the same intent PatchMenuAvailability the runtime
	// emits for an untruncated result.
	ctrl.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "s3", Count: 55, Truncated: false},
	})

	before := ctrl.Snapshot()
	beforeEntry := findMenuEntryPilot(t, before, "s3")
	if beforeEntry.Availability != 55 || beforeEntry.AvailTruncated {
		t.Fatalf("test setup: s3 menu entry = %+v, want Availability=55 AvailTruncated=false before the truncated sweep result lands", beforeEntry)
	}

	// The availability sweep's truncated first-page probe result — this is
	// exactly the live-observed defect: mid-sweep, s3's badge drops from
	// exact 55 to "50+". Gen must match the session's live AvailabilityGen
	// (seeded at 1 by session.New, per its own "seed=1 makes Gen=0 always
	// stale" doc comment) — AvailabilityChecked.AcceptZeroGen() is false, so
	// a Gen:0 event here would be silently dropped as stale rather than
	// exercising the downgrade path this test targets.
	vs, _ := ctrl.Handle(messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: true,
		Count:        50,
		Truncated:    true,
		Gen:          core.Session().AvailabilityGen,
	})

	entry := findMenuEntryPilot(t, vs, "s3")
	if entry.Availability != 55 {
		t.Errorf("s3 menu Availability = %d, want unchanged 55 — DEF-2/C5: a truncated sweep probe must never downgrade an already-exact stored total", entry.Availability)
	}
	if entry.AvailTruncated {
		t.Error("s3 menu AvailTruncated = true, want false — the exact badge must stay intact (no '+' suffix) when a truncated probe lands after an exact observation")
	}
}

// findMenuEntryPilot is a small local helper mirroring the inline
// menu-entry-lookup loop already duplicated across this package's other
// cache/menu tests (kept file-local per this package's existing convention
// of not sharing helpers across test files).
func findMenuEntryPilot(t *testing.T, vs app.ViewState, shortName string) app.MenuEntry {
	t.Helper()
	if vs.Body.Menu == nil {
		t.Fatalf("ViewState.Body.Menu is nil (looking for %q)", shortName)
	}
	for i := range vs.Body.Menu.Entries {
		if vs.Body.Menu.Entries[i].ShortName == shortName {
			return vs.Body.Menu.Entries[i]
		}
	}
	t.Fatalf("Body.Menu.Entries has no %q entry", shortName)
	return app.MenuEntry{}
}

// -----------------------------------------------------------------------
// DEF-3 (C6) — per-row Findings must survive the production save+reload+
// reseed round trip (not just the cache package's own manually-constructed
// Row round-trip tests), and cold-boot seeding must render glyphs/status
// derived from them.
//
// See the file-level investigation note above: this pin exercises the real
// wiring seam end-to-end. Kept as a single comprehensive test rather than
// several narrower ones, since the concern is specifically the WIRING
// (do the real production call sites thread Findings through, not just the
// cache package's own struct), not the cache package's marshal/unmarshal
// correctness (already covered by TestAllLoadedPages_PersistBeyondFirstPage_
// InTypeFile in app_web_live_cold_boot_test.go).
// -----------------------------------------------------------------------

// TestSaveResourceListCache_FindingsSurviveWiredSaveAndColdBootReseed pins
// DEF-3 against the production wiring: a list screen holding a
// resource.Resource row with a Wave-2 Finding, saved via the SAME call path
// production code uses (Controller.applyResourcesLoaded -> syncExactTotalToMenu
// -> maybeSaveResourceListCache -> Core.SaveResourceListCache — driven here
// via the public ApplyResourcesLoaded test seam used elsewhere in this
// package, e.g. TestChildAndFilteredLists_NeverWrittenToTypeFile), must
// leave the persisted TypeFile.Rows[].Findings populated, and a brand-new
// controller cold-booted against that same on-disk pair must seed rows whose
// Findings are non-empty (so buildListBody's render-time classification can
// derive the correct glyph/severity for the seeded row before any live
// fetch confirms it).
func TestSaveResourceListCache_FindingsSurviveWiredSaveAndColdBootReseed(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	s := session.New()
	s.Profile = "pilot-def3-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := app.New(core)
	t.Cleanup(ctrl.Close)

	finding := domain.Finding{
		Code:     "s3-public-read",
		Phrase:   "publicly readable",
		Severity: domain.SevBroken,
		Source:   "wave2:s3",
	}

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{
		{ID: "bucket-def3-1", Name: "def3-bucket", Type: "s3", Fields: map[string]string{"region": "us-east-1"}, Findings: []domain.Finding{finding}},
	}, nil, false)

	store := cache.LoadDir("pilot-def3-prof", "us-east-1")
	tf, ok := store.Type("s3")
	if !ok {
		t.Fatal(`store.Type("s3") missing after a production-path list-open + ApplyResourcesLoaded save`)
	}
	if len(tf.Rows) != 1 {
		t.Fatalf("persisted s3 TypeFile.Rows has %d entries, want 1", len(tf.Rows))
	}
	if len(tf.Rows[0].Findings) != 1 || tf.Rows[0].Findings[0].Code != "s3-public-read" {
		t.Errorf("persisted s3 TypeFile.Rows[0].Findings = %+v, want 1 finding with Code=%q — DEF-3: per-row findings must survive the production save wiring, not just a manually-constructed cache.Row", tf.Rows[0].Findings, "s3-public-read")
	}

	// Cold-boot half: a brand-new controller for the SAME pair must seed the
	// list-open with the persisted row's Findings intact, so render-time
	// classification (buildListBody -> td.ResolveColor) produces the correct
	// glyph/severity before any live fetch lands.
	s2 := session.New()
	s2.Profile = "pilot-def3-prof"
	s2.Region = "us-east-1"
	core2 := runtime.New(s2, resource.AllResourceTypes())
	ctrl2 := app.New(core2)
	t.Cleanup(ctrl2.Close)

	ctrl2.Handle(messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"s3": tf.Count},
	})
	_, _ = ctrl2.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	snap := ctrl2.Snapshot()
	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after cold-boot s3 list open")
	}
	if len(lb.Rows) == 0 {
		t.Fatal("cold-boot seeded zero rows for s3 — cannot assert on findings-derived rendering")
	}
	found := false
	for i := range lb.Rows {
		if lb.Rows[i].ResourceID == "bucket-def3-1" {
			found = true
			if lb.Rows[i].Severity == "" {
				t.Error("cold-boot seeded row for bucket-def3-1 has empty Severity — DEF-3: the seeded row's persisted Findings must drive render-time severity classification, not just the raw Fields")
			}
		}
	}
	if !found {
		t.Error("cold-boot seeded rows missing bucket-def3-1 — the persisted row was not seeded back at all")
	}
}

// -----------------------------------------------------------------------
// DEF-4 (C7) — (a) a refresh of one type must leave sibling per-type files
// byte-identical when driven through the PRODUCTION refresh path end-to-end
// (not just Store.SaveType directly, which TestPerTypeSave_TouchingOneType_
// LeavesSiblingFilesByteExact in app_web_live_cold_boot_test.go already
// covers at the Store level); (b) a save after a truncated first-page
// refetch must not shrink previously-persisted rows nor leave count/rows
// inconsistent with the header's exact flag.
//
// Contract choice for (b), stated explicitly per the dispatch's instruction:
// mirroring C5's exactness rule, the FULLER row set (from a prior exact
// fetch) is kept until a new EXACT refetch replaces it — a truncated refetch
// must not truncate previously-persisted rows, and Count/len(Rows)/Exact
// must remain mutually consistent (Exact=true implies len(Rows)==Count when
// rows are being persisted at all).
// -----------------------------------------------------------------------

// TestProductionRefresh_OneType_LeavesSiblingTypeFilesByteIdentical drives
// the refresh of s3 through the real controller-level path (list-open +
// ApplyResourcesLoaded, exactly as TestSaveResourceListCache_
// FindingsSurviveWiredSaveAndColdBootReseed above does, mirroring a Ctrl+R
// refresh's data flow) and asserts a SIBLING type's on-disk file
// (pre-populated directly via cache.LoadDir/SaveType, mirroring an earlier
// session's save) is byte-identical before and after — pinning DEF-4a
// end-to-end through the production wiring rather than only through
// Store.SaveType directly.
func TestProductionRefresh_OneType_LeavesSiblingTypeFilesByteIdentical(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	seed := cache.LoadDir("pilot-def4a-prof", "us-east-1")
	seed.Put("ec2", cache.TypeFile{
		HasResources: true,
		Count:        12,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "i-0def4apreexist01", Name: "pre-existing-instance", Fields: map[string]string{"state": "running"}},
		},
	})
	if err := seed.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType (ec2 fixture): %v", err)
	}
	ec2Path := cache.Dir("pilot-def4a-prof", "us-east-1") + "/ec2.yaml"
	before, err := readFileForAuditPilot(t, ec2Path)
	if err != nil {
		t.Fatalf("reading ec2 fixture file before the s3-only production refresh: %v", err)
	}

	s := session.New()
	s.Profile = "pilot-def4a-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := app.New(core)
	t.Cleanup(ctrl.Close)

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{
		{ID: "bucket-def4a-1", Type: "s3", Fields: map[string]string{"region": "us-east-1"}},
	}, nil, false)

	after, err := readFileForAuditPilot(t, ec2Path)
	if err != nil {
		t.Fatalf("reading ec2 fixture file after the s3-only production refresh: %v", err)
	}
	if before != after {
		t.Error("ec2.yaml bytes changed after refreshing ONLY s3 through the production controller path — DEF-4a: a per-type refresh must leave every sibling type file byte-identical")
	}
}

// TestProductionRefresh_TruncatedRefetch_NeverShrinksPersistedRows_HeaderStaysConsistent
// pins DEF-4b: a type whose disk file already holds an EXACT 55 rows (a
// prior full-depth session), refreshed via the production list-open path
// with a TRUNCATED 50-row result (e.g. a first-page-only refetch), must
// leave the persisted file's Count/Rows/Exact mutually consistent — per the
// stated contract choice, the prior fuller (55-row, exact) state is kept
// until a new EXACT observation replaces it, exactly mirroring C5.
func TestProductionRefresh_TruncatedRefetch_NeverShrinksPersistedRows_HeaderStaysConsistent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	rows55 := make([]cache.Row, 55)
	for i := range rows55 {
		id := "bucket-def4b-" + itoaPilot(i)
		rows55[i] = cache.Row{ID: id, Name: id, Fields: map[string]string{"region": "us-east-1"}}
	}
	seed := cache.LoadDir("pilot-def4b-prof", "us-east-1")
	seed.Put("s3", cache.TypeFile{HasResources: true, Count: 55, Exact: true, Rows: rows55})
	if err := seed.SaveType("s3"); err != nil {
		t.Fatalf("SaveType (s3 exact-55 fixture): %v", err)
	}

	s := session.New()
	s.Profile = "pilot-def4b-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := app.New(core)
	t.Cleanup(ctrl.Close)

	// A truncated 50-row refetch landing on the SAME type — mirrors the
	// pilot's observed "truncated refetch save SHRANK rows 55->50" defect.
	rows50 := make([]resource.Resource, 50)
	for i := range rows50 {
		id := "bucket-def4b-" + itoaPilot(i)
		rows50[i] = resource.Resource{ID: id, Name: id, Type: "s3", Fields: map[string]string{"region": "us-east-1"}}
	}
	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	ctrl.ApplyResourcesLoaded("s3", rows50, &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-def4b"}, false)

	reloaded := cache.LoadDir("pilot-def4b-prof", "us-east-1")
	tf, ok := reloaded.Type("s3")
	if !ok {
		t.Fatal(`reloaded.Type("s3") missing after the truncated refetch`)
	}
	if !tf.Exact {
		t.Error("s3 TypeFile.Exact = false after a truncated refetch over a prior EXACT 55 — DEF-4b: exactness (like C5) must only ever advance, a truncated observation must not downgrade it")
	}
	if tf.Count != 55 {
		t.Errorf("s3 TypeFile.Count = %d, want unchanged 55 — a truncated 50-row refetch must not shrink an already-exact stored total", tf.Count)
	}
	if len(tf.Rows) != tf.Count {
		t.Errorf("s3 TypeFile: len(Rows)=%d but Count=%d — header and rows must agree; a truncated refetch must not leave them inconsistent", len(tf.Rows), tf.Count)
	}
	if len(tf.Rows) != 55 {
		t.Errorf("s3 TypeFile.Rows has %d entries, want the fuller 55-row set preserved (contract choice: keep the fuller row set until an exact refetch replaces it, mirroring C5)", len(tf.Rows))
	}
}

// -----------------------------------------------------------------------
// DEF-5 (C4) — after a fetch failure over cached content, Refreshing must
// stop, and an error marker must replace it (nothing goes blank, per C4:
// "keeps the content, swaps the marker for an error marker, and logs once").
//
// Pinned seam: app.ListBody has no error-surface field at all today (only
// Loading/LoadingMore/Refreshing) — this is a genuine field-existence gap,
// compile-red like StatusCol was when it was first introduced. Named
// LastFetchError (string, empty = no error) to mirror the existing
// enrichment-findings naming style on ListBody and stay orthogonal to the
// transient header Flash (which clears on the next Apply and is not
// per-surface/sticky, so it cannot satisfy C4's "marker" requirement on its
// own).
//
// Additionally: runtime.Core.HandleEvent's switch (internal/runtime/
// orchestrator.go) has NO case for messages.APIError at all — only the TUI
// adapter's shim calls Core.HandleAPIError directly, bypassing
// Controller.Handle/Core.HandleEvent entirely. A web/headless caller feeding
// a messages.APIError through Controller.Handle (exactly what
// ExecuteTask(KindFetchResources) returns on full failure, per
// internal/runtime/executor.go) currently gets nil intents/tasks back — the
// list's Refreshing flag never clears and no error reaches ListBody. Both
// halves are pinned below.
// -----------------------------------------------------------------------

// TestAPIError_OverCachedList_ClearsRefreshing_SetsErrorMarker pins DEF-5's
// full behavioral contract at the Controller.Handle seam a web/headless
// caller actually uses: a list screen seeded from cache (Refreshing=true,
// rows on screen) that then receives the real messages.APIError event a
// failed KindFetchResources execution produces must end with
// Refreshing=false and ListBody.LastFetchError populated — rows must remain
// on screen (nothing blanks).
func TestAPIError_OverCachedList_ClearsRefreshing_SetsErrorMarker(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	core, ctrl := newLiveWebStyleController(t, "pilot-def5-prof", "us-east-1")

	// Seed a REAL on-disk per-type file for s3 so the cache-first list-open
	// has genuine row data to seed from (C6a: a counts-only
	// AvailabilityCacheLoaded with no real disk row data never fabricates
	// placeholder Rows — see
	// TestWebBoot_AvailabilityCacheLoaded_CountsOnlyFallback_KeepsLoadingTrue
	// in app_web_live_cold_boot_test.go).
	store := core.EnsureCacheStore()
	if store == nil {
		t.Fatal("core.EnsureCacheStore() = nil — test fixture requires a live disk store to seed rows into")
	}
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        2,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "bucket-pilot-def5-1", Name: "bucket-pilot-def5-1"},
			{ID: "bucket-pilot-def5-2", Name: "bucket-pilot-def5-2"},
		},
	})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("seed fixture SaveType(s3): %v", err)
	}

	ctrl.Handle(messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"s3": 2},
	})
	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	seeded := ctrl.Snapshot()
	seededLB := seeded.Body.List
	if seededLB == nil {
		t.Fatal("Body.List is nil after a warm s3 list open")
	}
	if !seededLB.Refreshing {
		t.Fatal("test setup: Refreshing=false after a warm cache-seeded list open — expected the live fetch still in flight")
	}
	seededRowCount := len(seededLB.Rows)
	if seededRowCount == 0 {
		t.Fatal("test setup: zero seeded rows — cannot assert rows survive the failure")
	}

	vs, _ := ctrl.Handle(messages.APIError{
		ResourceType: "s3",
		Err:          errPilotFetchFailed,
		Gen:          0,
	})

	lb := vs.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after APIError over a cached list")
	}
	if lb.Refreshing {
		t.Error("Refreshing = true after APIError landed, want false — DEF-5/C4: a fetch failure over cached content must stop the refreshing marker")
	}
	if lb.LastFetchError == "" {
		t.Error("LastFetchError is empty after APIError landed, want a non-empty error marker — DEF-5/C4: the marker must swap to an error marker, not just disappear")
	}
	if len(lb.Rows) != seededRowCount {
		t.Errorf("len(Rows) = %d after APIError, want unchanged %d — DEF-5/C4: cached content must remain on screen, nothing goes blank on a fetch failure", len(lb.Rows), seededRowCount)
	}
}

// errPilotFetchFailed is a fixed sentinel error for DEF-5's APIError pin,
// avoiding a dependency on any specific AWS SDK error type.
var errPilotFetchFailed = &pilotFetchError{}

type pilotFetchError struct{}

func (*pilotFetchError) Error() string { return "pilot: simulated fetch failure" }

// -----------------------------------------------------------------------
// DEF-6 (C3) — MenuEntry has no per-entry origin field distinguishing
// "cache" (seeded, not yet re-verified) from "verified" (confirmed this
// session by a live AvailabilityChecked landing). Compile-red field-
// existence pin, mirroring StatusCol's introduction.
// -----------------------------------------------------------------------

// TestMenuEntry_Origin_CacheBeforeVerification_FlipsOnAvailabilityChecked
// pins DEF-6 end-to-end: a menu entry seeded purely from
// AvailabilityCacheLoaded (disk cache, not yet re-verified this session)
// must report Origin=="cache"; once the matching AvailabilityChecked result
// lands for that type, Origin must flip to "verified".
func TestMenuEntry_Origin_CacheBeforeVerification_FlipsOnAvailabilityChecked(t *testing.T) {
	core, ctrl := newLiveWebStyleController(t, "", "us-east-1")

	vs, _ := ctrl.Handle(messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"s3": 55},
	})
	seeded := findMenuEntryPilot(t, vs, "s3")
	if seeded.Origin != "cache" {
		t.Errorf("s3 menu entry Origin = %q immediately after AvailabilityCacheLoaded, want %q — DEF-6/C3: a cache-seeded, not-yet-verified entry must be distinguishable from a verified one", seeded.Origin, "cache")
	}

	vs2, _ := ctrl.Handle(messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: true,
		Count:        55,
		Gen:          core.Session().AvailabilityGen,
	})
	verified := findMenuEntryPilot(t, vs2, "s3")
	if verified.Origin != "verified" {
		t.Errorf("s3 menu entry Origin = %q after AvailabilityChecked landed, want %q — DEF-6/C3: origin must flip once a live probe confirms the type this session", verified.Origin, "verified")
	}
}

// -----------------------------------------------------------------------
// DEF-7 (C7/C8) — a background availability sweep's probe + Wave-2
// enrichment completion must persist that type's per-row rows/findings to
// disk WITHOUT any list screen ever having been opened. Today only
// SaveAvailabilityCache (counts-only, no rows) runs from the
// TaskKindSaveCache executor case; SaveResourceListCache (which carries
// rows) is only ever called from maybeSaveResourceListCache, reachable
// solely through the list-open -> applyResourcesLoaded -> syncExactTotalToMenu
// chain in internal/app/handle.go.
//
// The sanity-gap addendum (a TypeFile whose Rows length contradicts Count)
// is SKIPPED per the dispatch's explicit instruction: C7 describes each
// type's file as self-contained and self-healing on the next successful
// save (a corrupt/inconsistent file degrades to "no cache" for that type
// only, per C7's own unreadable-file rule) — a live mismatch CHECK inside a
// TypeFile is implementation-defensive scope the contract doc does not
// require, and there is no existing seam (encode/decode chokepoint) this QA
// pass is scoped to touch to add one. Noting the decision here rather than
// silently omitting it.
// -----------------------------------------------------------------------

// TestAvailabilitySweepAndEnrichment_PersistsRowsPerType_WithoutAnyListOpen
// pins DEF-7: driving a full availability-sweep-and-enrichment completion
// for s3 through Controller.Handle — WITHOUT ever calling
// Apply(ActionCommand, "s3") or otherwise opening the s3 list screen — must
// still leave a readable per-type file on disk carrying the rows the sweep
// fetched (with findings), so a corrupt/missing file self-heals on the very
// next background sweep rather than only on the next list open.
func TestAvailabilitySweepAndEnrichment_PersistsRowsPerType_WithoutAnyListOpen(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	s := session.New()
	s.Profile = "pilot-def7-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := app.New(core)
	t.Cleanup(ctrl.Close)

	finding := domain.Finding{Code: "s3-public-read", Phrase: "publicly readable", Severity: domain.SevBroken, Source: "wave2:s3"}
	sweepResources := []resource.Resource{
		{ID: "bucket-def7-1", Name: "def7-bucket", Type: "s3", Fields: map[string]string{"region": "us-east-1"}, Findings: []domain.Finding{finding}},
	}

	// Drive exactly one AvailabilityChecked landing for s3, with the queue
	// already drained (so handleAvailabilityChecked's "all checks done" path
	// fires TaskKindSaveCache) — no list screen is opened anywhere in this
	// test. Gen must match the session's live AvailabilityGen (seeded at 1 by
	// session.New — see the DEF-2 test above for the same gotcha), or the
	// event is silently dropped as stale.
	_, tasks := ctrl.Handle(messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: true,
		Count:        1,
		Resources:    sweepResources,
		Gen:          core.Session().AvailabilityGen,
	})

	hasSaveCacheTask := false
	for _, tk := range tasks {
		if tk.Key.Kind == runtime.TaskKindSaveCache {
			hasSaveCacheTask = true
			break
		}
	}
	if !hasSaveCacheTask {
		t.Fatal("handleAvailabilityChecked (queue drained) returned no TaskKindSaveCache task — test assumption broken, cannot exercise the sweep-completion save path")
	}

	// Execute the real save-cache task the sweep dispatched, exactly as
	// DrainSync would, still without ever opening a list screen.
	for _, tk := range tasks {
		if tk.Key.Kind != runtime.TaskKindSaveCache {
			continue
		}
		ev, err := core.ExecuteTask(context.Background(), tk)
		if err != nil {
			t.Fatalf("ExecuteTask(TaskKindSaveCache): %v", err)
		}
		if ev != nil {
			ctrl.Handle(ev)
		}
	}

	store := cache.LoadDir("pilot-def7-prof", "us-east-1")
	tf, ok := store.Type("s3")
	if !ok {
		t.Fatal(`store.Type("s3") missing after a sweep+enrichment completion — DEF-7/C7: per-type persistence must not require a list screen to have been opened`)
	}
	if len(tf.Rows) == 0 {
		t.Error("s3 TypeFile.Rows is empty after a sweep completion with no list ever opened — DEF-7: the sweep's fetched rows (with findings) must persist per-type without requiring a list-open, so a corrupt file self-heals on the next sweep, not only on the next list visit")
	}
	found := false
	for _, r := range tf.Rows {
		if r.ID == "bucket-def7-1" {
			found = true
			if len(r.Findings) != 1 || r.Findings[0].Code != "s3-public-read" {
				t.Errorf("persisted sweep-only row Findings = %+v, want 1 finding with Code=%q", r.Findings, "s3-public-read")
			}
		}
	}
	if !found {
		t.Error("persisted s3 TypeFile.Rows missing bucket-def7-1 — the sweep-fetched row was not persisted at all")
	}
}

// -----------------------------------------------------------------------
// DEF-8 (C6) — Wave-2 findings applied to an OPEN live list must reach the
// persisted per-type cache. Verified live (last gap before the S3-pilot
// rerun): applyEnrichment (internal/runtime/helpers.go, driven from
// Core.handleEnrichmentChecked via the real messages.EnrichmentChecked
// event) mutates findings into session.ResourceCache / LazyResourceCache /
// ProbeResources only. Separately, PatchResourceList's intent handler
// (internal/app/intents.go) stores the SAME findings into the controller's
// own c.enrichmentStore map (applyEnrichmentState) — a parallel
// id->domain.Finding lookup used only by GetListEnrichmentFindings for glyph
// rendering. Neither of those two writes ever touches ls.Rows[i].Findings or
// c.resourceCache[type][i].Findings, which is what
// maybeSaveResourceListCache (internal/app/handle.go) reads when persisting
// (Findings: r.Findings, copied straight from ls.Rows). Net effect on a real
// account: s3.yaml carries issues:5 in the header and every row with ZERO
// findings — a cold-boot reseed then has nothing for the already-green DEF-3
// render-time classification to classify, so no glyphs render.
//
// This differs from DEF-3 above: DEF-3 pins findings that arrive ALREADY
// baked onto the Resource passed to ApplyResourcesLoaded (the Wave-1 initial
// load). DEF-8 pins the Wave-2 path: rows land with NO findings, enrichment
// is applied afterward through the production EnrichmentChecked seam, and
// only THEN is the list-open save re-triggered — exactly the sequence a live
// account produces (fetch, then a later enrichment probe).
//
// Contract note for the fix: the controller's enrichment-application seam
// (PatchResourceList's intent case in internal/app/intents.go, alongside its
// existing applyEnrichmentState + applyListFieldUpdates calls) must ALSO
// write findings onto the controller's own row stores (ls.Rows +
// c.resourceCache) — the same explicit dual-store propagation pattern
// ClearRowFindings (list_body.go) already established for the clearing
// direction. Persisting then needs no special logic: maybeSaveResourceListCache
// already reads Findings straight off ls.Rows.
// -----------------------------------------------------------------------

// TestEnrichmentChecked_OpenList_FindingsReachPersistedCacheAndColdBootGlyph
// pins DEF-8 end-to-end through the production seams a live app actually
// uses: open the s3 list, land its Wave-1 rows with NO findings (mirrors a
// real fetch, findings are not known yet), apply Wave-2 enrichment through
// the real Controller.Handle(messages.EnrichmentChecked{...}) seam the live
// enrichment probe emits, re-land the same rows (mirrors the next sync that
// re-triggers the list-open save path, e.g. a background re-poll or Ctrl+R),
// then assert the persisted TypeFile's Rows carry the finding for the
// flagged row. A second assertion cold-boots a fresh controller from that
// same on-disk pair and asserts the seeded row renders with a non-empty
// Severity — the already-green DEF-3 machinery, closing the loop end to end.
func TestEnrichmentChecked_OpenList_FindingsReachPersistedCacheAndColdBootGlyph(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	s := session.New()
	s.Profile = "pilot-def8-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := app.New(core)
	t.Cleanup(ctrl.Close)

	finding := domain.Finding{
		Code:     "s3-public-read",
		Phrase:   "publicly readable",
		Severity: domain.SevBroken,
		Source:   "wave2:s3",
	}

	// Open the s3 list and land its Wave-1 rows with NO findings — mirrors a
	// real fetch landing before any enrichment probe has run.
	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	baseRows := []resource.Resource{
		{ID: "bucket-def8-1", Name: "def8-bucket", Type: "s3", Fields: map[string]string{"region": "us-east-1"}},
	}
	ctrl.ApplyResourcesLoaded("s3", baseRows, nil, false)

	// Sanity: the pre-enrichment save must NOT carry the finding yet (setup
	// assumption, not the defect under test).
	preStore := cache.LoadDir("pilot-def8-prof", "us-east-1")
	preTF, ok := preStore.Type("s3")
	if !ok || len(preTF.Rows) != 1 {
		t.Fatalf("test setup: pre-enrichment s3 TypeFile missing or wrong row count: ok=%v rows=%+v", ok, preTF.Rows)
	}
	if len(preTF.Rows[0].Findings) != 0 {
		t.Fatalf("test setup: pre-enrichment s3 TypeFile.Rows[0].Findings = %+v, want empty (findings must not exist before enrichment runs)", preTF.Rows[0].Findings)
	}

	// Apply Wave-2 enrichment through the PRODUCTION seam: the real
	// messages.EnrichmentChecked event via Controller.Handle — exactly what
	// Core.handleEnrichmentChecked processes from a live enrichment probe.
	// TypeGen left at zero: EnrichmentChecked.AcceptZeroGen()==true and the
	// per-type gen guard only fires when msg.TypeGen != 0.
	ctrl.Handle(messages.EnrichmentChecked{
		ResourceType: "s3",
		Issues:       1,
		Findings:     map[string][]domain.Finding{"bucket-def8-1": {finding}},
	})

	// Re-trigger the list-open save path — the same seam
	// maybeSaveResourceListCache uses (ResourcesLoaded landing on the open
	// top-level list), mirroring the next background sync/poll after
	// enrichment has landed. Rows carry no baked-in findings here either —
	// if the fix is missing, this save re-persists the same findings-less
	// rows maybeSaveResourceListCache always reads from ls.Rows.
	ctrl.ApplyResourcesLoaded("s3", baseRows, nil, false)

	store := cache.LoadDir("pilot-def8-prof", "us-east-1")
	tf, ok := store.Type("s3")
	if !ok {
		t.Fatal(`store.Type("s3") missing after enrichment + list-open save`)
	}
	if len(tf.Rows) != 1 {
		t.Fatalf("persisted s3 TypeFile.Rows has %d entries, want 1", len(tf.Rows))
	}
	if len(tf.Rows[0].Findings) != 1 || tf.Rows[0].Findings[0].Code != "s3-public-read" {
		t.Errorf("persisted s3 TypeFile.Rows[0].Findings = %+v, want 1 finding with Code=%q — DEF-8/C6: Wave-2 findings applied to an OPEN live list via the real EnrichmentChecked seam must reach ls.Rows/resourceCache so the list-open save path persists them, not just the session-side stores applyEnrichment writes to", tf.Rows[0].Findings, "s3-public-read")
	}

	// Cold-boot half: a brand-new controller for the SAME pair must seed the
	// list-open with the persisted row's Findings intact, closing the loop to
	// the already-green DEF-3 render-time classification.
	s2 := session.New()
	s2.Profile = "pilot-def8-prof"
	s2.Region = "us-east-1"
	core2 := runtime.New(s2, resource.AllResourceTypes())
	ctrl2 := app.New(core2)
	t.Cleanup(ctrl2.Close)

	ctrl2.Handle(messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"s3": tf.Count},
	})
	_, _ = ctrl2.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	snap := ctrl2.Snapshot()
	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after cold-boot s3 list open")
	}
	if len(lb.Rows) == 0 {
		t.Fatal("cold-boot seeded zero rows for s3 — cannot assert on findings-derived rendering")
	}
	found := false
	for i := range lb.Rows {
		if lb.Rows[i].ResourceID == "bucket-def8-1" {
			found = true
			if lb.Rows[i].Severity == "" {
				t.Error("cold-boot seeded row for bucket-def8-1 has empty Severity — DEF-8: the seeded row's persisted Findings must drive render-time severity classification, closing the loop back to DEF-3's glyph rendering")
			}
		}
	}
	if !found {
		t.Error("cold-boot seeded rows missing bucket-def8-1 — the persisted row was not seeded back at all")
	}
}

// -----------------------------------------------------------------------
// small local helpers (kept file-local per this package's existing
// convention of not sharing helpers across test files, mirrored from
// app_web_live_cold_boot_test.go's readFileForAudit/itoaColdBoot)
// -----------------------------------------------------------------------

func readFileForAuditPilot(t *testing.T, path string) (string, error) {
	t.Helper()
	return readFileForAudit(t, path)
}

func itoaPilot(n int) string {
	return itoaColdBoot(n)
}
