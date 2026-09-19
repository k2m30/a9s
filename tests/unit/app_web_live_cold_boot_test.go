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

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// newLiveWebStyleController builds a Controller the way core/web/construct.go's
// newSession does for a live (non-demo) session: no pre-supplied clients and
// s.NoCache false.
func newLiveWebStyleController(t *testing.T, profile, region string) (*runtime.Core, *app.Controller) {
	t.Helper()
	core := runtime.Bootstrap(profile, region, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)
	ctrl.SetUIMode("web")
	return core, ctrl
}

func TestWebBoot_AvailabilityCacheLoaded_AppliesCountsAndIssuesToMenu(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	_, ctrl := newLiveWebStyleController(t, "", "us-east-1")

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

func TestWebBoot_Refreshing_TrueDuringCacheSeededSweep_FalseOnComplete(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	core, ctrl := newLiveWebStyleController(t, "webboot-refreshing-prof", "us-east-1")

	store := core.EnsureCacheStore()
	if store == nil {
		t.Fatal("core.EnsureCacheStore() = nil — test fixture requires a live disk store to seed rows into")
	}
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        57,
		Exact:        false,
		Rows: []cache.Row{
			{ID: "bucket-webboot-1", Name: "bucket-webboot-1"},
		},
	})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("seed fixture SaveType(s3): %v", err)
	}

	vs, tasks := ctrl.Handle(messages.AvailabilityCacheLoaded{
		Entries:    map[string]int{"s3": 57, "ec2": 3},
		Truncated:  map[string]bool{"s3": true},
		IssueKnown: map[string]bool{},
	})
	if vs.Body.Menu == nil {
		t.Fatal("Handle(AvailabilityCacheLoaded) returned nil Body.Menu")
	}

	// A disk-cache load against nil clients dispatches no probe tasks; it only
	// latches Session.AvailSweepPending, and ClientsReady drains the first batch.
	// Gen:1 — ConnectGen seeds at 1 and this session is never rotated.
	vs, readyTasks := ctrl.Handle(messages.ClientsReady{Gen: 1})
	tasks = append(tasks, readyTasks...)

	hasProbeTask := false
	for _, tk := range tasks {
		if tk.Key.Kind == runtime.TaskKindProbeAvailability {
			hasProbeTask = true
			break
		}
	}
	if !hasProbeTask {
		t.Fatal("Handle(AvailabilityCacheLoaded)+Handle(ClientsReady) returned no TaskKindProbeAvailability tasks — the background sweep this test's Refreshing assertion depends on was never queued; test assumption broken")
	}

	if !vs.Body.Menu.Refreshing {
		t.Error("MenuBody.Refreshing = false, want true — a cache-seeded live sweep with outstanding availability probes must show Refreshing=true so a polling browser sees it (C3), matching the live-smoke gap")
	}

	// AvailabilityChecked.AcceptZeroGen() is false, so Gen must carry the live
	// AvailabilityGen or Core.HandleEvent drops the event as stale.
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

func TestWebBoot_ColdListOpen_ControllerLevel_ReturnsLoadingShellAndFetchTask(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	_, ctrl := newLiveWebStyleController(t, "", "us-east-1")

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
		t.Error("IsBackgroundTaskKind(KindFetchResources) = true, want false — documents today's classification (always blocking); core/web handleAction drains this synchronously before writing the response even though the snapshot above already shows a renderable Loading shell, which is Contract C's actual (unreachable-hermetically) red")
	}
}

func TestWebBoot_AvailabilityCacheLoaded_CountsOnlyFallback_KeepsLoadingTrue(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	_, ctrl := newLiveWebStyleController(t, "", "us-east-1")

	_, _ = ctrl.Handle(messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"ec2": 1},
	})

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	snap := ctrl.Snapshot()
	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening ec2 list post-cache-load")
	}
	if !lb.Loading {
		t.Error("Loading = false, want true — C6a: a counts-only cache-load with no real per-type disk rows must never fabricate placeholder Rows, so the list-open still shows the ordinary Loading shell")
	}
	if len(lb.Rows) != 0 {
		t.Errorf("len(Rows) = %d, want 0 — C6a: no real disk row data exists for this type, so RowStore must not seed any rows", len(lb.Rows))
	}
}

// perTypeCacheDir returns the directory cache.DirForTest(profile, region)
// resolves to under the redirected A9S_CONFIG_FOLDER.
func perTypeCacheDir(t *testing.T, profile, region string) string {
	t.Helper()
	return cache.DirForTest(profile, region)
}

func TestPerTypeSave_TouchingOneType_LeavesSiblingFilesByteExact(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	store := cache.LoadDirForTest("merge-prof", "us-east-1")
	if store == nil {
		t.Fatal("cache.LoadDirForTest on an empty directory returned nil — must return an empty (non-nil) Store, never fail, per C7")
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

	store2 := cache.LoadDirForTest("merge-prof", "us-east-1")
	if store2 == nil {
		t.Fatal("cache.LoadDirForTest returned nil on a populated directory")
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

	store3 := cache.LoadDirForTest("merge-prof", "us-east-1")
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

func TestAllLoadedPages_PersistBeyondFirstPage_InTypeFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	rows := make([]cache.Row, 55)
	for i := range rows {
		rows[i] = cache.Row{ID: "bucket-p-" + itoaColdBoot(i), Fields: map[string]string{"region": "us-east-1"}}
	}
	rows[0].Findings = []domain.Finding{{Code: "s3-public-read", Phrase: "publicly readable", Severity: domain.SevBroken, Source: "wave2:s3"}}

	store := cache.LoadDirForTest("pages-prof", "us-east-1")
	store.Put("s3", cache.TypeFile{HasResources: true, Count: 55, Exact: true, Rows: rows})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("SaveType: %v", err)
	}

	reloaded := cache.LoadDirForTest("pages-prof", "us-east-1")
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

func TestColdBoot_SeedsAllLoadedPages_PerTypeFile_InstantlySeedsBeforeFetchCompletes(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	rows := make([]cache.Row, 55)
	for i := range rows {
		id := "bucket-cb-" + itoaColdBoot(i)
		rows[i] = cache.Row{ID: id, Name: id, Fields: map[string]string{"region": "us-east-1"}}
	}
	rows[10].Findings = []domain.Finding{{Code: "s3-public-read", Phrase: "publicly readable", Severity: domain.SevBroken, Source: "wave2:s3"}}

	store := cache.LoadDirForTest("coldboot-prof", "us-east-1")
	store.Put("s3", cache.TypeFile{HasResources: true, Count: 55, Exact: true, Rows: rows})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("SaveType: %v", err)
	}

	s := session.New()
	s.Profile = "coldboot-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)

	reloaded := cache.LoadDirForTest("coldboot-prof", "us-east-1")
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

// cache.Row carries only ID/Name/Fields/Findings: colors, glyphs and status are
// derived at render time, so a classification-rules change never requires a
// cache migration.
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

func TestChildAndFilteredLists_NeverWrittenToTypeFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	s := session.New()
	s.Profile = "childlist-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)

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

// EnsureDetailState fills Related blocks with Loading placeholders on every
// detail-open, so panel emptiness proves nothing; the restart check is on
// RelatedCacheGet.
func TestColdBoot_SecondVisit_RelatedFanOutRerunsAfterRestart(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	relatedCacheKey := runtime.RelatedCacheKey("ec2", "i-0restartvisit001")

	func() {
		s := session.New()
		s.Profile = "restart-prof"
		s.Region = "us-east-1"
		core := runtime.New(s, resource.AllResourceTypes())
		core.RelatedCacheSet(relatedCacheKey, []runtime.RelatedCacheResult{
			{DefDisplayName: "Security Groups", Result: resource.KnownRelated("sg", []string{"sg-restart-test"}, false)},
		})
		if _, ok := core.RelatedCacheGet(relatedCacheKey); !ok {
			t.Fatal("test setup: RelatedCacheGet returned ok=false immediately after RelatedCacheSet in the same session")
		}
	}()

	s2 := session.New()
	s2.Profile = "restart-prof"
	s2.Region = "us-east-1"
	core2 := runtime.New(s2, resource.AllResourceTypes())

	if _, ok := core2.RelatedCacheGet(relatedCacheKey); ok {
		t.Error("RelatedCacheGet found a hit on a brand-new controller for the same profile+region — related-panel results must be session-only (C6) and never survive a restart, which would otherwise let a stale/wrong navigation target short-circuit the fan-out")
	}
}

// No save for a profile+region may happen before that pair's cache directory
// has been loaded this session; a *Store is obtainable only through LoadDir.
func TestLoadBeforeSave_PairSwitch_NeverSavesBeforeLoad(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	s := session.New()
	s.Profile = "pair-a"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)

	storeA := cache.LoadDirForTest("pair-a", "us-east-1")
	storeA.Put("ec2", cache.TypeFile{HasResources: true, Count: 1})
	if err := storeA.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType (pair A fixture): %v", err)
	}

	core.SetProfile("pair-b")
	core.SetRegion("us-west-2")
	_ = ctrl

	dirB := perTypeCacheDir(t, "pair-b", "us-west-2")
	if dirExistsForAudit(dirB) {
		t.Error("pair B's cache directory exists immediately after a profile/region switch, before any LoadDir(pair-b,...) call — a save must never precede that pair's own load (C7 hard invariant)")
	}
}

func TestNoCache_NeverLoadsPopulatedDir_NeverWritesFiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	store := cache.LoadDirForTest("nocache-prof", "us-east-1")
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
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)

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

func TestAncientTypeFile_SeedsNormally_NoAgeDiscard(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	store := cache.LoadDirForTest("ancient-prof", "us-east-1")
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

	// Put stamps SavedAt=now, so backdating needs a second Put with SavedAt set,
	// then a re-save.
	tf, ok := store.Type("s3")
	if !ok {
		t.Fatal(`store.Type("s3") missing after fixture save`)
	}
	tf.SavedAt = time.Now().AddDate(-3, 0, 0)
	store.Put("s3", tf)
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("SaveType (backdate): %v", err)
	}

	reloaded := cache.LoadDirForTest("ancient-prof", "us-east-1")
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
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)

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

// cacheDiskAccessAllowlist lists "<repo-relative-path>:<os.*-func-name>" keys
// allowed to reference a raw cache-path-derived disk primitive outside the
// cache package. Each addition needs a justification comment at the call site.
var cacheDiskAccessAllowlist = map[string]bool{}

// cacheDiskPrimitives are the os-level calls forbidden outside core/cache:
// direct access to a cache file's bytes or path bypasses the single
// encode/decode chokepoint.
var cacheDiskPrimitives = map[string]bool{
	"ReadFile": true, "WriteFile": true, "Open": true,
	"OpenFile": true, "Create": true, "Remove": true, "Rename": true,
	"ReadDir": true, "Mkdir": true, "MkdirAll": true, "RemoveAll": true,
}

func TestCacheFileIO_OnlyThroughCachePackage_NoDirectDiskAccessElsewhere(t *testing.T) {
	_, thisFile, _, ok := goruntime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot locate test file")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	cachePkgDir := filepath.Join(repoRoot, "core", "cache")

	var goFiles []string
	for _, scanRoot := range []string{filepath.Join(repoRoot, "core"), filepath.Join(repoRoot, "internal")} {
		walkErr := filepath.WalkDir(scanRoot, func(path string, d fs.DirEntry, err error) error {
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
			t.Fatalf("filepath.WalkDir(%s): %v", scanRoot, walkErr)
		}
	}
	if len(goFiles) == 0 {
		t.Fatal("filepath.WalkDir found zero core/+internal/ .go files — check repo layout")
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
			// Calling cache.DirForTest outside core/cache is itself a violation, even
			// outside an os.* call.
			if pkgIdent, isIdent := sel.X.(*ast.Ident); isIdent && pkgIdent.Name == "cache" && sel.Sel.Name == "DirForTest" {
				key := baseName + ":cache.DirForTest"
				if !cacheDiskAccessAllowlist[key] {
					line := fset.Position(call.Pos()).Line
					violations = append(violations, baseName+":"+itoaColdBoot(line)+
						": calls cache.DirForTest() outside core/cache — only cache.LoadDirForTest/(*Store).SaveType may resolve a cache directory path (C7a single chokepoint)")
				}
				return true
			}

			// An os.* primitive whose arguments reference "cache." touches a cache path
			// directly.
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
					": os."+sel.Sel.Name+"() called with a cache-path-derived argument outside core/cache — C7a requires all cache-file disk I/O to flow through cache.LoadDirForTest/(*Store).SaveType")
			}
			return true
		})
	}

	if len(violations) > 0 {
		t.Errorf("found %d cache-file disk-access violation(s) outside core/cache:\n\n  %s\n\nAll cache-file reads/writes must flow through cache.LoadDirForTest/(*Store).SaveType (C7a).",
			len(violations), strings.Join(violations, "\n  "))
	}
}

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

// Per-finding FirstSeen is part of the on-disk schema at version 2.
func TestCacheSchemaVersion_Exported(t *testing.T) {
	if cache.SchemaVersion != 2 {
		t.Errorf("cache.SchemaVersion = %d, want 2", cache.SchemaVersion)
	}
}

func readFileForAudit(t *testing.T, path string) (string, error) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// fileExistsForAudit treats any stat error as absent.
func fileExistsForAudit(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExistsForAudit(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

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
