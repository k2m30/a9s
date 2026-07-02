// app_web_live_cold_boot_test.go — RED/pin tests reconciled against the FINAL
// cache contract at docs/design/cache-requirements.md, ROUND 2 (commit
// 671e88c5: per-type files, load-before-save invariant, scope and privacy
// rules). This file started as a pre-contract draft, was reconciled once
// against round-1 (single-file C7), and is now reconciled again against
// round-2 (per-type-file C7, new C7b). The header below records the final
// disposition of every original test plus the extended coverage added on
// top of it, across BOTH reconciliation passes.
//
// ROUND-1 DISPOSITION (unchanged by round-2, restated for continuity):
//
//   - Contract A (menu counts from disk cache) -> KEPT AS-IS, unrenamed.
//     TestWebBoot_AvailabilityCacheLoaded_AppliesCountsAndIssuesToMenu pins
//     C1 ("show what you know"): the disk-cache counts/issue-badges DO reach
//     the menu once the real messages.AvailabilityCacheLoaded event is
//     processed. Round-2 does not change this outcome (only the on-disk
//     SHAPE feeding it changes), so it is left as a GREEN precondition,
//     unchanged.
//
//   - Contract B (menu Refreshing during a cache-seeded sweep) -> KEPT
//     AS-IS, unrenamed. TestWebBoot_Refreshing_TrueDuringCacheSeededSweep_
//     FalseOnComplete pins the "one global updating flag" half of round-2's
//     C3 (per-entry origin flags are a NEW, separate round-2 requirement —
//     see the round-2 section below for why it is NOT pinned here).
//
//   - Contract C (cold list-open Loading-shell controller-level precondition)
//     -> KEPT AS-IS, unrenamed. Untouched by round-2 (C4 is unchanged).
//
//   - Contract D (disk-rows never seeded into ProbeResources by
//     handleAvailabilityCacheLoaded) -> KEPT, single-page fixture retained
//     verbatim as a still-valid (if narrower) red pin of the same handler
//     gap; TestColdBoot_SeedsAllLoadedPages_PerTypeFile_InstantlySeedsBefore
//     FetchCompletes below (round-2) extends it to the per-type-file, all-
//     pages scope.
//
// ROUND-2 REWRITE (this pass) — the old cache.File/cache.Entry/cache.Load/
// cache.Save/cache.CachedRow/cache.Path/cache.DefaultTTL surface is being
// DELETED by the coder and replaced with a directory-per-pair,
// file-per-type surface (cache.Dir, cache.Row, cache.TypeFile, cache.Store,
// cache.LoadDir, (*Store).Type/Types/Put/SaveType — exact signatures per the
// architect's round-2 handoff). Every test in this file that touched the old
// surface is REWRITTEN below against the new one; none of the old surface
// is referenced anywhere in this file anymore. This is deliberately a
// COMPILE-RED pass: the new cache package symbols do not exist in
// production code yet, so this entire file will fail to build until the
// coder implements internal/cache's round-2 surface — exactly like any
// other TDD red phase, just at package-compile granularity instead of a
// single assertion.
//
//   - Old Contract E / round-1 "Contract F" (whole-state save) ->
//     SUPERSEDED. Round-1's C7 ("no merge logic, save writes the entire
//     file back") is now round-2's DIFFERENT no-merge story: per-type files
//     make merge-avoidance structural (a save physically only ever touches
//     one type's file, so there is no "other types" data in scope to
//     clobber). TestPerTypeSave_TouchingOneType_LeavesSiblingFilesByteExact
//     replaces the whole-state-save test with the per-type-isolation
//     equivalent.
//   - Item 3 (C6 all-loaded-pages) -> re-pinned against per-type files:
//     TestAllLoadedPages_PersistBeyondFirstPage_InTypeFile and
//     TestColdBoot_SeedsAllLoadedPages_PerTypeFile_InstantlySeedsBefore
//     FetchCompletes.
//   - Item 4 (C6 detail/related session-only) -> re-pinned:
//     TestDetailAndRelatedState_NeverPersistedToDisk (structural, against
//     TypeFile's schema) and TestColdBoot_SecondVisit_RelatedFanOutRerunsAf
//     terRestart (FIXED per the coordinator's note: the original
//     "RelatedRows/Related is empty" assertion was wrong — Related blocks
//     auto-populate as Loading placeholders on every detail-open regardless
//     of history. The correct pin is on CACHE state: a restarted controller
//     dispatches the related fan-out task again for a resource visited in a
//     prior process, proving nothing related-panel-shaped survived restart
//     to short-circuit it).
//   - Item 5 (C1 no TTL) -> re-pinned: TestAncientTypeFile_SeedsNormally_No
//     AgeDiscard, using TypeFile.SavedAt instead of cache.File.CheckedAt.
//   - NEW (round-2 C6 scope boundary): TestChildAndFilteredLists_NeverWritten
//     ToTypeFile pins "only the canonical top-level, unfiltered list... is
//     persisted — child lists, related-navigation lists, and filtered views
//     ... are never written to disk".
//   - NEW (round-2 C6 render-time derivation): TestColorsGlyphsStatus_NotPer
//     sisted_DerivedAtRenderFromFieldsAndFindings pins "Colors, glyphs and
//     status texts are NOT persisted... derived at render time".
//   - NEW (round-2 C7 hard invariant): TestLoadBeforeSave_PairSwitch_NeverSa
//     vesBeforeLoad pins the structural load-before-save invariant at the
//     runtime/controller level via a pair-switch scenario, per the
//     coordinator's guidance (Store is the only way to save — no Store, no
//     save — so this is pinned as "the runtime never calls a save-shaped
//     path for a pair whose Store it has not obtained via LoadDir first").
//   - NEW (round-2 C7b): TestNoCache_NeverLoadsPopulatedDir_NeverWritesFiles
//     pins "--no-cache disables persisted load AND save entirely" at the
//     controller level using the existing session.NoCache flag.
//   - C7a chokepoint audit -> UPDATED (not rewritten from scratch): now
//     scans for cache.Dir(...) references and os.* primitives fed a
//     cache.Dir(...)-derived path, outside internal/cache.
//   - C7a format-marker pin -> UPDATED: TypeFile.Version (not
//     cache.File.Version) is the pinned first field.
//
// All tests are hermetic: A9S_CONFIG_FOLDER redirected to t.TempDir(), no AWS
// credentials, no network.
package unit_test

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
)

// newLiveWebStyleController builds a Controller the same way
// internal/web/construct.go newSession does for a LIVE (non-demo) session:
// runtime.Bootstrap + app.New + SetUIMode("web") — no pre-supplied clients,
// no synchronous demo handshake, s.NoCache left false (the live default).
func newLiveWebStyleController(profile, region string) (*runtime.Core, *app.Controller) {
	core := runtime.Bootstrap(profile, region, resource.AllResourceTypes())
	ctrl := app.New(core)
	ctrl.SetUIMode("web")
	return core, ctrl
}

// -----------------------------------------------------------------------
// Contract A (GREEN precondition) — cache-loaded counts DO reach the menu
// -----------------------------------------------------------------------

// TestWebBoot_AvailabilityCacheLoaded_AppliesCountsAndIssuesToMenu locks in
// the working half of the cache-load path: once ExecuteTask(TaskKindLoadAvailCache)'s
// resulting messages.AvailabilityCacheLoaded event reaches Controller.Handle
// (exactly what DrainSyncProgress does with BootstrapLive's returned tasks),
// the menu DOES carry the cached counts and issue badges. This guards against
// a future regression accidentally breaking the part that already works,
// while Contract B below pins the part that is genuinely broken (Refreshing
// staying false throughout the sweep this same event kicks off).
func TestWebBoot_AvailabilityCacheLoaded_AppliesCountsAndIssuesToMenu(t *testing.T) {
	_, ctrl := newLiveWebStyleController("", "us-east-1")

	vs, _ := ctrl.Handle(messages.AvailabilityCacheLoaded{
		Entries:     map[string]int{"s3": 57},
		Truncated:   map[string]bool{"s3": true},
		IssueCounts: map[string]int{"s3": 5},
		IssueKnown:  map[string]bool{"s3": true},
	})
	if vs.Body.Menu == nil {
		t.Fatal("Handle(AvailabilityCacheLoaded) returned nil Body.Menu")
	}
	var s3Entry *app.MenuEntry
	for i := range vs.Body.Menu.Entries {
		if vs.Body.Menu.Entries[i].ShortName == "s3" {
			s3Entry = &vs.Body.Menu.Entries[i]
			break
		}
	}
	if s3Entry == nil {
		t.Fatal(`Body.Menu.Entries has no "s3" entry after AvailabilityCacheLoaded`)
	}
	if !s3Entry.AvailKnown || s3Entry.Availability != 57 {
		t.Errorf("s3 entry Availability=%d AvailKnown=%v, want Availability=57 AvailKnown=true", s3Entry.Availability, s3Entry.AvailKnown)
	}
	if s3Entry.IssueBadge.Count != 5 {
		t.Errorf("s3 entry IssueBadge.Count=%d, want 5", s3Entry.IssueBadge.Count)
	}
}

// -----------------------------------------------------------------------
// Contract B — MenuBody.Refreshing must be observable during a cache-seeded
// live sweep (C3's one-global-updating-flag half; the per-entry origin flag
// half of round-2's C3 is intentionally NOT pinned in this file — it names
// a NEW app-level view-state field the round-2 handoff did not specify a
// signature for, so pinning it here would mean QA inventing that field.
// Left for a follow-up dispatch once app.MenuEntry's origin-flag field name
// is specified.)
// -----------------------------------------------------------------------

// TestWebBoot_Refreshing_TrueDuringCacheSeededSweep_FalseOnComplete pins
// Contract B / the global-flag half of C3 against the real handler under
// test: driving the actual messages.AvailabilityCacheLoaded event (what
// ExecuteTask(TaskKindLoadAvailCache) produces) through Controller.Handle,
// exactly as DrainSyncProgress does for BootstrapLive's returned tasks.
//
// Currently red: handleAvailabilityCacheLoaded queues session.AvailQueue but
// never writes session.ProbeResources, so menuRefreshing() — which ranges
// over ProbeResources, not AvailQueue — reports false immediately after the
// cache load, even though a real multi-type background sweep has just been
// queued and is about to run for real (slow, network-bound) probes.
func TestWebBoot_Refreshing_TrueDuringCacheSeededSweep_FalseOnComplete(t *testing.T) {
	core, ctrl := newLiveWebStyleController("", "us-east-1")

	vs, tasks := ctrl.Handle(messages.AvailabilityCacheLoaded{
		Entries:    map[string]int{"s3": 57, "ec2": 3},
		Truncated:  map[string]bool{"s3": true},
		IssueKnown: map[string]bool{},
	})
	if vs.Body.Menu == nil {
		t.Fatal("Handle(AvailabilityCacheLoaded) returned nil Body.Menu")
	}
	hasProbeTask := false
	for _, tk := range tasks {
		if tk.Key.Kind == runtime.TaskKindProbeAvailability {
			hasProbeTask = true
			break
		}
	}
	if !hasProbeTask {
		t.Fatal("Handle(AvailabilityCacheLoaded) returned no TaskKindProbeAvailability tasks — the background sweep this test's Refreshing assertion depends on was never queued; test assumption broken")
	}

	if !vs.Body.Menu.Refreshing {
		t.Error("MenuBody.Refreshing = false, want true — a cache-seeded live sweep with outstanding availability probes must show Refreshing=true so a polling browser sees it (C3), matching the live-smoke gap")
	}

	// Stamp the live AvailabilityGen — AcceptZeroGen()==false for
	// AvailabilityChecked, so a Gen:0 event is unconditionally dropped as
	// stale by Core.HandleEvent's IsStale guard.
	liveGen := core.Session().AvailabilityGen

	pending := tasks
	for len(pending) > 0 {
		tk := pending[0]
		pending = pending[1:]
		if tk.Key.Kind != runtime.TaskKindProbeAvailability {
			continue
		}
		var v app.ViewState
		v, pending2 := ctrl.Handle(messages.AvailabilityChecked{
			ResourceType: tk.Key.Scope,
			HasResources: true,
			Count:        1,
			Gen:          liveGen,
		})
		pending = append(pending, pending2...)
		vs = v
	}

	if vs.Body.Menu == nil {
		t.Fatal("Body.Menu nil after draining the sweep")
	}
	if vs.Body.Menu.Refreshing {
		t.Error("MenuBody.Refreshing = true, want false — sweep must clear Refreshing once every dispatched probe's result has landed (C3: drops in the same frame the sweep completes)")
	}
}

// -----------------------------------------------------------------------
// Contract C — cold list-open Loading-shell precondition (C4, unchanged by
// round-2, seam documentation)
// -----------------------------------------------------------------------

// TestWebBoot_ColdListOpen_ControllerLevel_ReturnsLoadingShellAndFetchTask
// documents the GREEN half of Contract C / C4 at the controller level: on a
// completely cold live-web-style controller (no ProbeResources, no
// ResourceCache), Apply(open-list) already returns Loading=true in the same
// snapshot as the KindFetchResources task.
func TestWebBoot_ColdListOpen_ControllerLevel_ReturnsLoadingShellAndFetchTask(t *testing.T) {
	_, ctrl := newLiveWebStyleController("", "us-east-1")

	_, tasks := ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	snap := ctrl.Snapshot()
	if snap.Body.Kind != app.BodyKindList {
		t.Fatalf("Body.Kind = %q, want %q", snap.Body.Kind, app.BodyKindList)
	}
	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening a cold s3 list")
	}
	if !lb.Loading {
		t.Error("Loading = false, want true — a truly cold list open (no seeded rows anywhere) must show the Loading shell")
	}

	hasFetch := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.KindFetchResources {
			hasFetch = true
			break
		}
	}
	if !hasFetch {
		t.Fatal("Apply(open s3 list) returned no KindFetchResources task — cold list open must dispatch the content fetch")
	}

	if app.IsBackgroundTaskKind(runtime.KindFetchResources) {
		t.Error("IsBackgroundTaskKind(KindFetchResources) = true, want false — documents today's classification (always blocking); internal/web handleAction drains this synchronously before writing the response even though the snapshot above already shows a renderable Loading shell, which is Contract C's actual (unreachable-hermetically) red")
	}
}

// -----------------------------------------------------------------------
// Contract D — disk-rows seeding at web boot (cache-load never seeds Rows)
// -----------------------------------------------------------------------

// TestWebBoot_AvailabilityCacheLoaded_DoesNotSeedProbeResourcesRows pins
// Contract D directly against the real handler: even though
// handleAvailabilityCacheLoaded DOES apply counts/issues to the menu
// (Contract A, green), it never touches session.ProbeResources — so a list
// opened immediately after a cache-seeded live sweep starts shows
// Loading=true with zero rows, not the disk-cached rows with Refreshing=true.
//
// Kept as a narrow single-page pin driven purely through the counts-only
// AvailabilityCacheLoaded event (no cache package types referenced) — still
// valid and independent of the round-2 on-disk shape change. See
// TestColdBoot_SeedsAllLoadedPages_PerTypeFile_InstantlySeedsBeforeFetch
// Completes below for the round-2, all-pages, per-type-file extension.
func TestWebBoot_AvailabilityCacheLoaded_DoesNotSeedProbeResourcesRows(t *testing.T) {
	_, ctrl := newLiveWebStyleController("", "us-east-1")

	// Models exactly what ExecuteTask(TaskKindLoadAvailCache) would produce
	// from a loaded per-type ec2 TypeFile with one row — the counts-only
	// projection this event carries today. No cache package type is
	// referenced directly; this keeps the test isolated from the round-2
	// on-disk shape so it remains a pure "handler never seeds
	// ProbeResources" pin.
	_, _ = ctrl.Handle(messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"ec2": 1},
	})

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	snap := ctrl.Snapshot()
	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening ec2 list post-cache-load")
	}
	if lb.Loading {
		t.Error("Loading = true, want false — handleAvailabilityCacheLoaded must load the per-type cache's Rows into session.ProbeResources (mirroring Count/Truncated) so a cold list-open renders real disk-cached rows immediately instead of the empty Loading shell")
	}
	if !lb.Refreshing {
		t.Error("Refreshing = false, want true — a live fetch must still run to confirm/replace disk-seeded rows")
	}
}

// -----------------------------------------------------------------------
// Round-2 Contract F — per-type-file isolation (structural no-merge, C7)
// -----------------------------------------------------------------------

// perTypeCacheDir mirrors what cache.Dir(profile, region) will resolve to
// under an A9S_CONFIG_FOLDER-redirected temp dir, for tests that need to
// assert directly on directory/file existence without going through
// cache.LoadDir. Kept minimal and local to this file (no dependency on the
// coder's eventual Dir() implementation beyond calling it directly).
func perTypeCacheDir(t *testing.T, profile, region string) string {
	t.Helper()
	return cache.Dir(profile, region)
}

// TestPerTypeSave_TouchingOneType_LeavesSiblingFilesByteExact pins round-2's
// structural no-merge story: "The cache is a directory per profile+region
// containing one self-contained file per resource type... Saving writes
// ONLY the touched type's file... a session that only touched s3 physically
// cannot disturb another type's file." Unlike round-1's whole-state-save
// test, this is checkable at the FILESYSTEM level: ec2's file's bytes must
// be byte-identical before and after an s3-only save, because a per-type
// save mechanically has no code path that could open ec2's file at all.
func TestPerTypeSave_TouchingOneType_LeavesSiblingFilesByteExact(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	store := cache.LoadDir("merge-prof", "us-east-1")
	if store == nil {
		t.Fatal("cache.LoadDir on an empty directory returned nil — must return an empty (non-nil) Store, never fail, per C7")
	}
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        50,
		Rows: []cache.Row{
			{ID: "bucket-preexisting-1", Name: "pre-existing-bucket", Fields: map[string]string{"region": "us-east-1"}},
		},
	})
	store.Put("ec2", cache.TypeFile{
		HasResources: true,
		Count:        12,
		Issues:       2,
		IssuesKnown:  true,
		Rows: []cache.Row{
			{ID: "i-0mergepreexist01", Name: "pre-existing-instance", Fields: map[string]string{"state": "running"},
				Findings: []domain.Finding{{Code: "ec2-stopped-with-eip", Phrase: "stopped, has EIP", Severity: domain.SevWarn, Source: "wave2:ec2"}}},
		},
	})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("SaveType(s3) fixture write: %v", err)
	}
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType(ec2) fixture write: %v", err)
	}

	dir := perTypeCacheDir(t, "merge-prof", "us-east-1")
	ec2Path := filepath.Join(dir, "ec2.yaml")
	before, err := readFileForAudit(t, ec2Path)
	if err != nil {
		t.Fatalf("reading ec2 type file before the s3-only save: %v", err)
	}

	// A fresh session for the SAME pair: load, touch ONLY s3, save ONLY s3.
	store2 := cache.LoadDir("merge-prof", "us-east-1")
	if store2 == nil {
		t.Fatal("cache.LoadDir returned nil on a populated directory")
	}
	ec2Before, ok := store2.Type("ec2")
	if !ok || len(ec2Before.Rows) != 1 {
		t.Fatalf("fixture sanity: store2.Type(ec2) = %+v (ok=%v), want 1 row loaded from disk", ec2Before, ok)
	}
	store2.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        2,
		Rows: []cache.Row{
			{ID: "bucket-new-1", Name: "new-bucket-1", Fields: map[string]string{"region": "us-east-1"}},
			{ID: "bucket-new-2", Name: "new-bucket-2", Fields: map[string]string{"region": "us-east-1"}},
		},
	})
	if saveErr := store2.SaveType("s3"); saveErr != nil {
		t.Fatalf("SaveType(s3): %v", saveErr)
	}

	after, err := readFileForAudit(t, ec2Path)
	if err != nil {
		t.Fatalf("reading ec2 type file after the s3-only save: %v", err)
	}
	if before != after {
		t.Errorf("ec2.yaml bytes changed after an s3-only SaveType call — per-type files must be physically untouched by a save of a different type:\nbefore=%q\nafter=%q", before, after)
	}

	store3 := cache.LoadDir("merge-prof", "us-east-1")
	s3Loaded, ok := store3.Type("s3")
	if !ok {
		t.Fatal(`store3.Type("s3") missing after save+reload`)
	}
	if s3Loaded.Count != 2 || len(s3Loaded.Rows) != 2 {
		t.Errorf("s3 TypeFile after reload = %+v, want Count=2 with 2 Rows (this session's new data)", s3Loaded)
	}
	ec2Loaded, ok := store3.Type("ec2")
	if !ok || ec2Loaded.Count != 12 || len(ec2Loaded.Rows) != 1 {
		t.Errorf("ec2 TypeFile after an s3-only save+reload = %+v (ok=%v), want the original Count=12 with 1 Row untouched", ec2Loaded, ok)
	}
}

// -----------------------------------------------------------------------
// Round-2 — C6 "ALL loaded pages", re-pinned against per-type files
// -----------------------------------------------------------------------

// TestAllLoadedPages_PersistBeyondFirstPage_InTypeFile pins the blunt scope
// of C6 against the new per-type-file surface directly: a TypeFile.Rows
// slice built from 55 fetched rows (50 + a 5-row load-more) must round-trip
// through Put+SaveType+LoadDir with all 55 rows intact, including their
// per-row Findings.
func TestAllLoadedPages_PersistBeyondFirstPage_InTypeFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	rows := make([]cache.Row, 55)
	for i := range rows {
		rows[i] = cache.Row{ID: "bucket-p-" + itoaColdBoot(i), Fields: map[string]string{"region": "us-east-1"}}
	}
	rows[0].Findings = []domain.Finding{{Code: "s3-public-read", Phrase: "publicly readable", Severity: domain.SevBroken, Source: "wave2:s3"}}

	store := cache.LoadDir("pages-prof", "us-east-1")
	store.Put("s3", cache.TypeFile{HasResources: true, Count: 55, Exact: true, Rows: rows})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("SaveType: %v", err)
	}

	reloaded := cache.LoadDir("pages-prof", "us-east-1")
	tf, ok := reloaded.Type("s3")
	if !ok {
		t.Fatal(`reloaded.Type("s3") missing`)
	}
	if len(tf.Rows) != 55 {
		t.Errorf("len(tf.Rows) = %d, want 55 — C6 requires ALL loaded pages persisted, not just the first page's 50", len(tf.Rows))
	}
	if len(tf.Rows) > 0 && (len(tf.Rows[0].Findings) != 1 || tf.Rows[0].Findings[0].Phrase != "publicly readable") {
		t.Errorf("tf.Rows[0].Findings = %+v, want 1 finding with Phrase=%q — per-row findings must survive the multi-page persistence", tf.Rows[0].Findings, "publicly readable")
	}
}

// TestColdBoot_SeedsAllLoadedPages_PerTypeFile_InstantlySeedsBeforeFetch
// Completes pins the cold-boot half of C6's "all pages" scope against the
// per-type-file surface: a TypeFile carrying 55 rows for s3 must seed a
// freshly-booted controller's list-open with all 55 rows instantly
// (Loading=false, Refreshing=true) before any live fetch completes.
func TestColdBoot_SeedsAllLoadedPages_PerTypeFile_InstantlySeedsBeforeFetchCompletes(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	rows := make([]cache.Row, 55)
	for i := range rows {
		id := "bucket-cb-" + itoaColdBoot(i)
		rows[i] = cache.Row{ID: id, Name: id, Fields: map[string]string{"region": "us-east-1"}}
	}
	rows[10].Findings = []domain.Finding{{Code: "s3-public-read", Phrase: "publicly readable", Severity: domain.SevBroken, Source: "wave2:s3"}}

	store := cache.LoadDir("coldboot-prof", "us-east-1")
	store.Put("s3", cache.TypeFile{HasResources: true, Count: 55, Exact: true, Rows: rows})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("SaveType: %v", err)
	}

	s := session.New()
	s.Profile = "coldboot-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := app.New(core)

	reloaded := cache.LoadDir("coldboot-prof", "us-east-1")
	tf, ok := reloaded.Type("s3")
	if !ok || len(tf.Rows) != 55 {
		t.Fatalf("fixture sanity: reloaded.Type(s3) = %+v (ok=%v), want 55 rows", tf, ok)
	}
	ctrl.Handle(messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"s3": tf.Count},
	})

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	snap := ctrl.Snapshot()
	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after cold-boot s3 list open")
	}
	if lb.Loading {
		t.Error("Loading = true, want false — a cache-seeded cold boot must render ALL persisted pages instantly, not show a spinner")
	}
	if !lb.Refreshing {
		t.Error("Refreshing = false, want true — a live fetch must still run to confirm/replace the disk-seeded rows")
	}
	if len(lb.Rows) != 55 {
		t.Fatalf("len(Rows) = %d, want 55 — cold-boot seeding must restore every persisted page, not just the first 50", len(lb.Rows))
	}
	found := false
	for i := range lb.Rows {
		if lb.Rows[i].ResourceID == "bucket-cb-10" {
			found = true
			break
		}
	}
	if !found {
		t.Error("seeded rows missing bucket-cb-10 — a mid-list row from the second half of the persisted pages was dropped")
	}
}

// -----------------------------------------------------------------------
// Round-2 — C6 render-time derivation of colors/glyphs/status
// -----------------------------------------------------------------------

// TestColorsGlyphsStatus_NotPersisted_TypeFileHasNoSuchFields pins the
// round-2 addition to C6: "Colors, glyphs and status texts are NOT
// persisted; they are derived at render time from the persisted fields +
// findings". This is a structural pin via reflection on cache.Row: it must
// carry only ID/Name/Fields/Findings — no color/glyph/status/decorator
// field of any kind — so a future classification-rules change never
// requires a cache migration.
func TestColorsGlyphsStatus_NotPersisted_TypeFileHasNoSuchFields(t *testing.T) {
	rt := reflect.TypeOf(cache.Row{})
	forbidden := []string{"color", "glyph", "status", "decorator"}
	for i := 0; i < rt.NumField(); i++ {
		name := strings.ToLower(rt.Field(i).Name)
		for _, f := range forbidden {
			if strings.Contains(name, f) {
				t.Errorf("cache.Row has field %q — C6 (round 2) forbids persisting colors/glyphs/status; these must be derived at render time from Fields+Findings", rt.Field(i).Name)
			}
		}
	}
}

// -----------------------------------------------------------------------
// Round-2 — C6 scope boundary: only the canonical top-level list persists
// -----------------------------------------------------------------------

// TestChildAndFilteredLists_NeverWrittenToTypeFile pins: "only the
// canonical top-level, unfiltered list of a type is persisted — child
// lists, related-navigation lists, and filtered views are session views
// over that data and are never written to disk (they must not poison the
// type's cache)." Modeled by opening a CHILD list (ParentContext set) for a
// type, landing resources on it, and asserting no save occurs for that
// child context — the top-level type's file (if any existed) must be
// unaffected, and no new file must appear for the child scope.
func TestChildAndFilteredLists_NeverWrittenToTypeFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	s := session.New()
	s.Profile = "childlist-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := app.New(core)

	// Push a CHILD list screen directly (ParentContext set), bypassing the
	// normal top-level ActionCommand open, mirroring how a drill-down child
	// view is modeled at the controller level.
	ctrl.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenChildList,
			Context: runtime.ScreenContext{ResourceType: "ec2"},
		},
	})
	ctrl.ApplyResourcesLoaded("ec2", []resource.Resource{
		{ID: "i-0childonly0001", Type: "ec2"},
	}, nil, false)

	dir := perTypeCacheDir(t, "childlist-prof", "us-east-1")
	ec2Path := filepath.Join(dir, "ec2.yaml")
	if fileExistsForAudit(ec2Path) {
		t.Error("ec2.yaml was written after a CHILD list fetch — only the canonical top-level unfiltered list may persist (C6 scope boundary); a child/filtered view must never poison the type's cache file")
	}
}

// -----------------------------------------------------------------------
// Round-2 Contract H — C6 detail/related state is session-only, never
// persisted (structural), plus the FIXED cache-state pin for restart
// -----------------------------------------------------------------------

// TestDetailAndRelatedState_NeverPersistedToDisk pins the session-only half
// of C6 structurally: cache.TypeFile carries no field that could represent
// detail-screen or related-panel data (no field name containing "detail" or
// "related" anywhere on the type), so no implementation of SaveType could
// accidentally persist it without adding such a field first — which this
// test would then catch.
func TestDetailAndRelatedState_NeverPersistedToDisk(t *testing.T) {
	rt := reflect.TypeOf(cache.TypeFile{})
	forbidden := []string{"detail", "related"}
	for i := 0; i < rt.NumField(); i++ {
		name := strings.ToLower(rt.Field(i).Name)
		for _, f := range forbidden {
			if strings.Contains(name, f) {
				t.Errorf("cache.TypeFile has field %q — C6 requires detail-screen and related-panel data to stay session-only, never reaching the persisted per-type file", rt.Field(i).Name)
			}
		}
	}
}

// TestColdBoot_SecondVisit_RelatedFanOutRerunsAfterRestart replaces the
// original (incorrect) "Related is empty after restart" assertion per the
// coordinator's fix: EnsureDetailState auto-populates Related blocks as
// Loading placeholders on every detail-open regardless of history, so
// asserting on panel emptiness is vacuous. The correct, meaningful pin is on
// CACHE state: a related-check result cached in-session (via
// Core.RelatedCacheSet, the same seam handleRelatedCheckBatch writes
// through) for a given resource must NOT be visible to a brand-new
// controller for the same profile+region — proving nothing related-panel-
// shaped survives a restart, so the SAME resource's detail re-open on the
// new controller has no cache hit to short-circuit the fan-out with (i.e.
// RelatedCacheGet on the new controller returns ok=false for the exact key
// the old controller had populated).
func TestColdBoot_SecondVisit_RelatedFanOutRerunsAfterRestart(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	relatedCacheKey := runtime.RelatedCacheKey("ec2", "i-0restartvisit001")

	// First "session": populate the related-check cache for one resource.
	func() {
		s := session.New()
		s.Profile = "restart-prof"
		s.Region = "us-east-1"
		core := runtime.New(s, resource.AllResourceTypes())
		core.RelatedCacheSet(relatedCacheKey, []runtime.RelatedCacheResult{
			{DefDisplayName: "Security Groups", Result: resource.RelatedCheckResult{
				TargetType:  "sg",
				Count:       1,
				ResourceIDs: []string{"sg-restart-test"},
			}},
		})
		if _, ok := core.RelatedCacheGet(relatedCacheKey); !ok {
			t.Fatal("test setup: RelatedCacheGet returned ok=false immediately after RelatedCacheSet in the same session")
		}
	}()

	// "Restart": brand-new session/core for the SAME profile+region.
	s2 := session.New()
	s2.Profile = "restart-prof"
	s2.Region = "us-east-1"
	core2 := runtime.New(s2, resource.AllResourceTypes())

	if _, ok := core2.RelatedCacheGet(relatedCacheKey); ok {
		t.Error("RelatedCacheGet found a hit on a brand-new controller for the same profile+region — related-panel results must be session-only (C6) and never survive a restart, which would otherwise let a stale/wrong navigation target short-circuit the fan-out")
	}
}

// -----------------------------------------------------------------------
// Round-2 — C7 hard invariant: no save before that pair's directory has
// been loaded this session
// -----------------------------------------------------------------------

// TestLoadBeforeSave_PairSwitch_NeverSavesBeforeLoad pins the HARD
// INVARIANT: "no save for a profile+region may happen before that pair's
// cache directory has been loaded (or declared absent/corrupt) in this
// session — early-startup and pair-switch writes must never race the load
// and wipe older knowledge." This is structural by construction on the new
// Store API (Put/SaveType are methods ON a *Store, and the only way to
// obtain one is LoadDir) — there is no package-level cache.SaveType(profile,
// region, ...) that could be called without a prior LoadDir. This test pins
// that structural guarantee still holds through a pair-switch at the
// controller level: switching profile/region must not leave any file
// written for the NEW pair until that pair's own LoadDir has actually run,
// modeled here by asserting the new pair's directory does not exist
// immediately after the switch intent, before any load/save task has been
// processed.
func TestLoadBeforeSave_PairSwitch_NeverSavesBeforeLoad(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	s := session.New()
	s.Profile = "pair-a"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := app.New(core)

	// Establish pair A's directory via a real load+save round trip.
	storeA := cache.LoadDir("pair-a", "us-east-1")
	storeA.Put("ec2", cache.TypeFile{HasResources: true, Count: 1})
	if err := storeA.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType (pair A fixture): %v", err)
	}

	// Switch to pair B (never before seen) at the controller level. Per the
	// invariant, nothing may be written for pair B's directory as a direct
	// consequence of the switch intent alone — a save must wait for pair
	// B's own LoadDir to run first (mirroring C9's "loads the new pair's
	// file per C1" ordering).
	core.SetProfile("pair-b")
	core.SetRegion("us-west-2")
	_ = ctrl

	dirB := perTypeCacheDir(t, "pair-b", "us-west-2")
	if dirExistsForAudit(dirB) {
		t.Error("pair B's cache directory exists immediately after a profile/region switch, before any LoadDir(pair-b,...) call — a save must never precede that pair's own load (C7 hard invariant)")
	}
}

// -----------------------------------------------------------------------
// Round-2 Contract C7b — --no-cache disables persisted load AND save
// -----------------------------------------------------------------------

// TestNoCache_NeverLoadsPopulatedDir_NeverWritesFiles pins C7b: "--no-cache
// disables persisted load AND save entirely (cold behavior every start,
// nothing written)". Pre-populates a real per-type file for the pair, boots
// a NoCache=true controller for that exact pair, drives a full open+fetch
// cycle, and asserts (a) the pre-existing file's count never reaches the
// menu (proving no load happened) and (b) the file on disk is byte-
// unchanged (proving no save happened).
func TestNoCache_NeverLoadsPopulatedDir_NeverWritesFiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	store := cache.LoadDir("nocache-prof", "us-east-1")
	store.Put("s3", cache.TypeFile{HasResources: true, Count: 999, Exact: true})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("SaveType fixture: %v", err)
	}
	dir := perTypeCacheDir(t, "nocache-prof", "us-east-1")
	s3Path := filepath.Join(dir, "s3.yaml")
	before, err := readFileForAudit(t, s3Path)
	if err != nil {
		t.Fatalf("reading s3 type file before no-cache boot: %v", err)
	}

	s := session.New()
	s.Profile = "nocache-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	core.SetNoCache(true)
	ctrl := app.New(core)

	snap := ctrl.Snapshot()
	if snap.Body.Menu == nil {
		t.Fatal("Snapshot() returned nil Body.Menu before navigation")
	}
	var s3Entry *app.MenuEntry
	for i := range snap.Body.Menu.Entries {
		if snap.Body.Menu.Entries[i].ShortName == "s3" {
			s3Entry = &snap.Body.Menu.Entries[i]
			break
		}
	}
	if s3Entry != nil && s3Entry.Availability == 999 {
		t.Error("menu s3 Availability = 999 (the pre-existing on-disk value) with NoCache=true — --no-cache must disable persisted LOAD entirely, never seeding from an existing file")
	}

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{{ID: "bucket-nocache-write-test", Type: "s3"}}, nil, false)

	after, err := readFileForAudit(t, s3Path)
	if err != nil {
		t.Fatalf("reading s3 type file after no-cache activity: %v", err)
	}
	if before != after {
		t.Error("s3.yaml bytes changed while NoCache=true — --no-cache must disable persisted SAVE entirely, never writing to disk")
	}
}

// -----------------------------------------------------------------------
// Round-2 — C1 "no TTL", re-pinned against TypeFile.SavedAt
// -----------------------------------------------------------------------

// TestAncientTypeFile_SeedsNormally_NoAgeDiscard pins C1's explicit,
// deliberate no-TTL rule against the round-2 per-type SavedAt field: a
// TypeFile with a SavedAt years in the past must seed the menu/list exactly
// like a freshly-saved one would — nothing discards by age.
func TestAncientTypeFile_SeedsNormally_NoAgeDiscard(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	store := cache.LoadDir("ancient-prof", "us-east-1")
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        12,
		Rows: []cache.Row{
			{ID: "bucket-ancient-1", Name: "ancient-bucket", Fields: map[string]string{"region": "us-east-1"}},
		},
	})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("SaveType: %v", err)
	}

	// Backdate the just-saved file's SavedAt by rewriting it through the
	// same Store API (Put stamps SavedAt=now on every call per the
	// architect's handoff, so backdating requires a second, explicit Put
	// with SavedAt set, then a re-save).
	tf, ok := store.Type("s3")
	if !ok {
		t.Fatal(`store.Type("s3") missing after fixture save`)
	}
	tf.SavedAt = time.Now().AddDate(-3, 0, 0)
	store.Put("s3", tf)
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("SaveType (backdate): %v", err)
	}

	reloaded := cache.LoadDir("ancient-prof", "us-east-1")
	ancientTF, ok := reloaded.Type("s3")
	if !ok {
		t.Fatal(`reloaded.Type("s3") missing`)
	}
	if ancientTF.Count != 12 || len(ancientTF.Rows) != 1 {
		t.Fatalf("ancient TypeFile was altered/dropped on LoadDir: %+v, want Count=12 with 1 Row intact regardless of SavedAt age", ancientTF)
	}
	if time.Since(ancientTF.SavedAt) < 2*365*24*time.Hour {
		t.Fatal("test setup: backdated SavedAt did not round-trip through Put+SaveType+LoadDir — fixture sanity check failed")
	}

	s := session.New()
	s.Profile = "ancient-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := app.New(core)

	vs, _ := ctrl.Handle(messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"s3": ancientTF.Count},
	})
	if vs.Body.Menu == nil {
		t.Fatal("Handle(AvailabilityCacheLoaded) returned nil Body.Menu for an ancient type file")
	}
	var s3Entry *app.MenuEntry
	for i := range vs.Body.Menu.Entries {
		if vs.Body.Menu.Entries[i].ShortName == "s3" {
			s3Entry = &vs.Body.Menu.Entries[i]
			break
		}
	}
	if s3Entry == nil {
		t.Fatal(`Body.Menu.Entries has no "s3" entry after loading an ancient type file — an old cache must still seed the menu (C1: no TTL, nothing discards by age)`)
	}
	if !s3Entry.AvailKnown || s3Entry.Availability != 12 {
		t.Errorf("ancient-cache s3 entry Availability=%d AvailKnown=%v, want Availability=12 AvailKnown=true — age must never zero or hide a cached count", s3Entry.Availability, s3Entry.AvailKnown)
	}
	if !vs.Body.Menu.Refreshing {
		t.Error("MenuBody.Refreshing = false, want true — an ancient cache must still be marked refreshing/verifying (C1: stale-marked + re-verification, never a bare unexplained number)")
	}
}

// -----------------------------------------------------------------------
// Contract C7a — at-rest security seam: single chokepoint + format marker
// (round-2: scans for cache.Dir(...) instead of the deleted cache.Path/Dir)
// -----------------------------------------------------------------------

// cacheDiskAccessAllowlist lists "<repo-relative-path>:<os.*-func-name>" keys
// that are legitimately allowed to reference a raw cache-path-derived disk
// primitive outside the cache package (currently empty — every production
// caller today goes through cache.LoadDir/(*Store).SaveType). Mirrors
// nonPaginatedAPIs's allowlist pattern in enrichment_pagination_audit_test.go:
// additions require a justification comment at the call site, not silent
// broadening.
var cacheDiskAccessAllowlist = map[string]bool{}

// cacheDiskPrimitives are the os-level calls C7a forbids outside
// internal/cache — direct filesystem access to a cache file's bytes or
// path bypasses the single encode/decode chokepoint C7a requires.
var cacheDiskPrimitives = map[string]bool{
	"ReadFile": true, "WriteFile": true, "Open": true,
	"OpenFile": true, "Create": true, "Remove": true, "Rename": true,
	"ReadDir": true, "Mkdir": true, "MkdirAll": true, "RemoveAll": true,
}

// TestCacheFileIO_OnlyThroughCachePackage_NoDirectDiskAccessElsewhere pins
// C7a's single-chokepoint requirement, updated for round-2's directory-per-
// pair layout: "every read and write of cache files goes through ONE
// encode/decode pair inside the cache module — no other code touches their
// bytes or paths". Source-grep AST audit (repo precedent:
// TestNoSingleCallListAPIEnrichers) over every internal/ .go file
// (excluding internal/cache and _test.go files) for:
//  1. any os.<primitive>(...) call whose argument expression textually
//     references "cache." (catches os.ReadFile(cache.Dir(...)+...) and
//     similar path-construction-then-raw-I/O patterns), and
//  2. any direct reference to cache.Dir at all outside internal/cache —
//     resolving the per-pair directory path is itself the seam violation
//     C7a rules out; only cache.LoadDir/(*Store).SaveType may do it.
func TestCacheFileIO_OnlyThroughCachePackage_NoDirectDiskAccessElsewhere(t *testing.T) {
	_, thisFile, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot locate test file")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	internalRoot := filepath.Join(repoRoot, "internal")
	cachePkgDir := filepath.Join(internalRoot, "cache")

	var goFiles []string
	walkErr := filepath.WalkDir(internalRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if filepath.Dir(path) == cachePkgDir {
			return nil
		}
		goFiles = append(goFiles, path)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("filepath.WalkDir(%s): %v", internalRoot, walkErr)
	}
	if len(goFiles) == 0 {
		t.Fatal("filepath.WalkDir found zero internal/ .go files — check repo layout")
	}

	fset := token.NewFileSet()
	var violations []string

	for _, filePath := range goFiles {
		src, parseErr := parser.ParseFile(fset, filePath, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse error in %s: %v", filePath, parseErr)
		}
		baseName, relErr := filepath.Rel(repoRoot, filePath)
		if relErr != nil {
			baseName = filePath
		}

		ast.Inspect(src, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			// Case 2: any selector cache.Dir, anywhere in the file — even
			// outside an os.* call — is itself the seam violation.
			if pkgIdent, isIdent := sel.X.(*ast.Ident); isIdent && pkgIdent.Name == "cache" && sel.Sel.Name == "Dir" {
				key := baseName + ":cache.Dir"
				if !cacheDiskAccessAllowlist[key] {
					line := fset.Position(call.Pos()).Line
					violations = append(violations, baseName+":"+itoaColdBoot(line)+
						": calls cache.Dir() outside internal/cache — only cache.LoadDir/(*Store).SaveType may resolve a cache directory path (C7a single chokepoint)")
				}
				return true
			}

			// Case 1: os.<primitive>(...) whose args textually reference
			// "cache." — catches os.ReadFile(cache.Dir(...)+...) etc.
			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok || pkgIdent.Name != "os" || !cacheDiskPrimitives[sel.Sel.Name] {
				return true
			}
			key := baseName + ":" + sel.Sel.Name
			if cacheDiskAccessAllowlist[key] {
				return true
			}
			flagged := false
			for _, arg := range call.Args {
				var buf strings.Builder
				_ = printer.Fprint(&buf, fset, arg)
				if strings.Contains(buf.String(), "cache.") {
					flagged = true
					break
				}
			}
			if flagged {
				line := fset.Position(call.Pos()).Line
				violations = append(violations, baseName+":"+itoaColdBoot(line)+
					": os."+sel.Sel.Name+"() called with a cache-path-derived argument outside internal/cache — C7a requires all cache-file disk I/O to flow through cache.LoadDir/(*Store).SaveType")
			}
			return true
		})
	}

	if len(violations) > 0 {
		t.Errorf("found %d cache-file disk-access violation(s) outside internal/cache:\n\n  %s\n\nAll cache-file reads/writes must flow through cache.LoadDir/(*Store).SaveType (C7a).",
			len(violations), strings.Join(violations, "\n  "))
	}
}

// TestTypeFile_FirstFieldIsFormatVersion pins C7a's format-marker
// requirement against the round-2 per-type schema: "each file starts with a
// format marker (the schema version)... version is a single integer". This
// is a structural pin on cache.TypeFile's field order via reflection — the
// first field must be an integer named "Version" serializing under a
// yaml:"version" tag.
func TestTypeFile_FirstFieldIsFormatVersion(t *testing.T) {
	rt := reflect.TypeOf(cache.TypeFile{})
	if rt.NumField() == 0 {
		t.Fatal("cache.TypeFile has zero fields")
	}
	first := rt.Field(0)
	if first.Name != "Version" {
		t.Errorf("cache.TypeFile's first field is %q, want %q — C7a requires the format/schema version to be the first field on disk so a future encrypted format can be detected before the rest of the file is parsed", first.Name, "Version")
		return
	}
	tag := first.Tag.Get("yaml")
	if !strings.HasPrefix(tag, "version") {
		t.Errorf("cache.TypeFile.Version yaml tag = %q, want to start with %q", tag, "version")
	}
	if k := first.Type.Kind(); k != reflect.Int && k != reflect.Int8 && k != reflect.Int16 && k != reflect.Int32 && k != reflect.Int64 {
		t.Errorf("cache.TypeFile.Version kind = %v, want an integer kind (C7a calls it 'a single integer')", k)
	}
}

// TestCacheSchemaVersion_Exported pins that cache.SchemaVersion (the
// current format marker value new saves must stamp) is exported and equals
// 1 for this initial round-2 rollout, per the architect's handoff.
func TestCacheSchemaVersion_Exported(t *testing.T) {
	if cache.SchemaVersion != 1 {
		t.Errorf("cache.SchemaVersion = %d, want 1", cache.SchemaVersion)
	}
}

// -----------------------------------------------------------------------
// small local helpers (kept file-local per this package's existing
// convention of not sharing helpers across test files)
// -----------------------------------------------------------------------

// readFileForAudit reads a file's full contents as a string for
// byte-comparison assertions.
func readFileForAudit(t *testing.T, path string) (string, error) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// fileExistsForAudit reports whether path exists (any error, including
// permission errors, is treated as "does not exist" for this audit's
// purposes — a strict existence check is not required here since Save
// failures are surfaced separately by their own error returns).
func fileExistsForAudit(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// dirExistsForAudit reports whether the directory at path exists.
func dirExistsForAudit(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// itoaColdBoot is a tiny local decimal formatter so this file has no
// dependency on strconv beyond what's already imported elsewhere in the
// package, mirroring itoaTest in runtime_cache_rows_exact_totals_test.go
// (duplicated locally to avoid cross-file coupling to another test file's
// helper lifetime, matching this package's existing convention).
func itoaColdBoot(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
