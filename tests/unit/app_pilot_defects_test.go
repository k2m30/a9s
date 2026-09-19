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

func TestIsBackgroundFetchTask_CachedRowsSeeded_IsBackground(t *testing.T) {
	req := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "s3"}}
	if !app.IsBackgroundFetchTask(req, true) {
		t.Error("IsBackgroundFetchTask(KindFetchResources, screenAlreadyRenderable=true) = false, want true — C4: a list-content fetch over an already-renderable screen (cached rows or Loading shell present) must never block the transport")
	}
}

// A cold fetch stays blocking: DrainSyncPartition's synchronous half is what
// returns the Loading… shell in the same response.
func TestIsBackgroundFetchTask_ColdNoRenderableContent_StaysBlocking(t *testing.T) {
	req := runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: "s3"}}
	if app.IsBackgroundFetchTask(req, false) {
		t.Error("IsBackgroundFetchTask(KindFetchResources, screenAlreadyRenderable=false) = true, want false — a genuinely cold fetch (no Loading shell, no cached rows) must stay blocking so the response carries the shell")
	}
}

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

func TestWebBoot_WarmListOpen_FetchTaskDeferredAsBackground(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	core, ctrl := newLiveWebStyleController(t, "pilot-warmboot-prof", "us-east-1")

	// A counts-only AvailabilityCacheLoaded never fabricates placeholder rows, so
	// a renderable warm open needs a real per-type file on disk.
	store := core.EnsureCacheStore()
	if store == nil {
		t.Fatal("core.EnsureCacheStore() = nil — test fixture requires a live disk store to seed rows into")
	}
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        3,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "bucket-pilot-warmboot-1", Name: "bucket-pilot-warmboot-1"},
			{ID: "bucket-pilot-warmboot-2", Name: "bucket-pilot-warmboot-2"},
			{ID: "bucket-pilot-warmboot-3", Name: "bucket-pilot-warmboot-3"},
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
		t.Error("KindFetchResources was not deferred to the background partition for a warm (already-renderable) list open — C4: the transport must not block on this fetch when cached rows are already on screen")
	}
}

func TestAvailabilityChecked_TruncatedProbe_NeverDowngradesExactMenuTotal(t *testing.T) {
	core, ctrl := newLiveWebStyleController(t, "", "us-east-1")

	ctrl.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "s3", Count: 55, Truncated: false},
	})

	before := ctrl.Snapshot()
	beforeEntry := findMenuEntryPilot(t, before, "s3")
	if beforeEntry.Availability != 55 || beforeEntry.AvailTruncated {
		t.Fatalf("test setup: s3 menu entry = %+v, want Availability=55 AvailTruncated=false before the truncated sweep result lands", beforeEntry)
	}

	// Gen must match the session's AvailabilityGen (session.New seeds it at 1):
	// AvailabilityChecked.AcceptZeroGen() is false, so Gen 0 is dropped as stale.
	vs, _ := ctrl.Handle(messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: true,
		Count:        50,
		Truncated:    true,
		Gen:          core.Session().AvailabilityGen,
	})

	entry := findMenuEntryPilot(t, vs, "s3")
	if entry.Availability != 55 {
		t.Errorf("s3 menu Availability = %d, want unchanged 55 — C5: a truncated sweep probe must never downgrade an already-exact stored total", entry.Availability)
	}
	if entry.AvailTruncated {
		t.Error("s3 menu AvailTruncated = true, want false — the exact badge must stay intact (no '+' suffix) when a truncated probe lands after an exact observation")
	}
}

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

func TestSaveResourceListCache_FindingsSurviveWiredSaveAndColdBootReseed(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	s := session.New()
	s.Profile = "pilot-coldreseed-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)

	finding := domain.Finding{
		Code:     "s3-public-read",
		Phrase:   "publicly readable",
		Severity: domain.SevBroken,
		Source:   "wave2:s3",
	}

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{
		{ID: "bucket-coldreseed-1", Name: "coldreseed-bucket", Type: "s3", Fields: map[string]string{"region": "us-east-1"}, Findings: []domain.Finding{finding}},
	}, nil, false)

	ctrl.WaitForCacheWrites()
	store := cache.LoadDirForTest("pilot-coldreseed-prof", "us-east-1")
	tf, ok := store.Type("s3")
	if !ok {
		t.Fatal(`store.Type("s3") missing after a production-path list-open + ApplyResourcesLoaded save`)
	}
	if len(tf.Rows) != 1 {
		t.Fatalf("persisted s3 TypeFile.Rows has %d entries, want 1", len(tf.Rows))
	}
	if len(tf.Rows[0].Findings) != 1 || tf.Rows[0].Findings[0].Code != "s3-public-read" {
		t.Errorf("persisted s3 TypeFile.Rows[0].Findings = %+v, want 1 finding with Code=%q — C6: per-row findings must survive the production save wiring, not just a manually-constructed cache.Row", tf.Rows[0].Findings, "s3-public-read")
	}

	s2 := session.New()
	s2.Profile = "pilot-coldreseed-prof"
	s2.Region = "us-east-1"
	core2 := runtime.New(s2, resource.AllResourceTypes())
	ctrl2 := newBlessedController(t, core2)
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
		if lb.Rows[i].ResourceID == "bucket-coldreseed-1" {
			found = true
			if lb.Rows[i].Severity == "" {
				t.Error("cold-boot seeded row for bucket-coldreseed-1 has empty Severity — C6: the seeded row's persisted Findings must drive render-time severity classification, not just the raw Fields")
			}
		}
	}
	if !found {
		t.Error("cold-boot seeded rows missing bucket-coldreseed-1 — the persisted row was not seeded back at all")
	}
}

func TestProductionRefresh_OneType_LeavesSiblingTypeFilesByteIdentical(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	seed := cache.LoadDirForTest("pilot-siblingrefresh-prof", "us-east-1")
	seed.Put("ec2", cache.TypeFile{
		HasResources: true,
		Count:        12,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "i-0siblingrefreshpre01", Name: "pre-existing-instance", Fields: map[string]string{"state": "running"}},
		},
	})
	if err := seed.SaveType("ec2"); err != nil {
		t.Fatalf("SaveType (ec2 fixture): %v", err)
	}
	ec2Path := cache.DirForTest("pilot-siblingrefresh-prof", "us-east-1") + "/ec2.yaml"
	before, err := readFileForAuditPilot(t, ec2Path)
	if err != nil {
		t.Fatalf("reading ec2 fixture file before the s3-only production refresh: %v", err)
	}

	s := session.New()
	s.Profile = "pilot-siblingrefresh-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	ctrl.ApplyResourcesLoaded("s3", []resource.Resource{
		{ID: "bucket-siblingrefresh-1", Type: "s3", Fields: map[string]string{"region": "us-east-1"}},
	}, nil, false)

	after, err := readFileForAuditPilot(t, ec2Path)
	if err != nil {
		t.Fatalf("reading ec2 fixture file after the s3-only production refresh: %v", err)
	}
	if before != after {
		t.Error("ec2.yaml bytes changed after refreshing ONLY s3 through the production controller path — sibling-file isolation (C7): a per-type refresh must leave every sibling type file byte-identical")
	}
}

// A truncated refetch keeps the prior exact row set until a new exact
// observation replaces it; Count, len(Rows) and Exact stay mutually consistent.
func TestProductionRefresh_TruncatedRefetch_NeverShrinksPersistedRows_HeaderStaysConsistent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	rows55 := make([]cache.Row, 55)
	for i := range rows55 {
		id := "bucket-truncrefetch-" + itoaPilot(i)
		rows55[i] = cache.Row{ID: id, Name: id, Fields: map[string]string{"region": "us-east-1"}}
	}
	seed := cache.LoadDirForTest("pilot-truncrefetch-prof", "us-east-1")
	seed.Put("s3", cache.TypeFile{HasResources: true, Count: 55, Exact: true, Rows: rows55})
	if err := seed.SaveType("s3"); err != nil {
		t.Fatalf("SaveType (s3 exact-55 fixture): %v", err)
	}

	s := session.New()
	s.Profile = "pilot-truncrefetch-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)

	rows50 := make([]resource.Resource, 50)
	for i := range rows50 {
		id := "bucket-truncrefetch-" + itoaPilot(i)
		rows50[i] = resource.Resource{ID: id, Name: id, Type: "s3", Fields: map[string]string{"region": "us-east-1"}}
	}
	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	ctrl.ApplyResourcesLoaded("s3", rows50, &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-truncrefetch"}, false)

	ctrl.WaitForCacheWrites()
	reloaded := cache.LoadDirForTest("pilot-truncrefetch-prof", "us-east-1")
	tf, ok := reloaded.Type("s3")
	if !ok {
		t.Fatal(`reloaded.Type("s3") missing after the truncated refetch`)
	}
	if !tf.Exact {
		t.Error("s3 TypeFile.Exact = false after a truncated refetch over a prior EXACT 55 — persisted-pair invariant: exactness (like C5) must only ever advance, a truncated observation must not downgrade it")
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

func TestAPIError_OverCachedList_ClearsRefreshing_SetsErrorMarker(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	core, ctrl := newLiveWebStyleController(t, "pilot-apierror-prof", "us-east-1")

	// A counts-only AvailabilityCacheLoaded never fabricates placeholder rows, so
	// a renderable warm open needs a real per-type file on disk.
	store := core.EnsureCacheStore()
	if store == nil {
		t.Fatal("core.EnsureCacheStore() = nil — test fixture requires a live disk store to seed rows into")
	}
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        2,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "bucket-pilot-apierror-1", Name: "bucket-pilot-apierror-1"},
			{ID: "bucket-pilot-apierror-2", Name: "bucket-pilot-apierror-2"},
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
		t.Error("Refreshing = true after APIError landed, want false — C4: a fetch failure over cached content must stop the refreshing marker")
	}
	if lb.LastFetchError == "" {
		t.Error("LastFetchError is empty after APIError landed, want a non-empty error marker — C4: the marker must swap to an error marker, not just disappear")
	}
	if len(lb.Rows) != seededRowCount {
		t.Errorf("len(Rows) = %d after APIError, want unchanged %d — C4: cached content must remain on screen, nothing goes blank on a fetch failure", len(lb.Rows), seededRowCount)
	}
}

var errPilotFetchFailed = &pilotFetchError{}

type pilotFetchError struct{}

func (*pilotFetchError) Error() string { return "pilot: simulated fetch failure" }

func TestMenuEntry_Origin_CacheBeforeVerification_FlipsOnAvailabilityChecked(t *testing.T) {
	core, ctrl := newLiveWebStyleController(t, "", "us-east-1")

	vs, _ := ctrl.Handle(messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"s3": 55},
	})
	seeded := findMenuEntryPilot(t, vs, "s3")
	if seeded.Origin != "cache" {
		t.Errorf("s3 menu entry Origin = %q immediately after AvailabilityCacheLoaded, want %q — C3: a cache-seeded, not-yet-verified entry must be distinguishable from a verified one", seeded.Origin, "cache")
	}

	vs2, _ := ctrl.Handle(messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: true,
		Count:        55,
		Gen:          core.Session().AvailabilityGen,
	})
	verified := findMenuEntryPilot(t, vs2, "s3")
	if verified.Origin != "verified" {
		t.Errorf("s3 menu entry Origin = %q after AvailabilityChecked landed, want %q — C3: origin must flip once a live probe confirms the type this session", verified.Origin, "verified")
	}
}

func TestAvailabilitySweepAndEnrichment_PersistsRowsPerType_WithoutAnyListOpen(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	s := session.New()
	s.Profile = "pilot-sweepenrich-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)

	finding := domain.Finding{Code: "s3-public-read", Phrase: "publicly readable", Severity: domain.SevBroken, Source: "wave2:s3"}
	sweepResources := []resource.Resource{
		{ID: "bucket-sweepenrich-1", Name: "sweepenrich-bucket", Type: "s3", Fields: map[string]string{"region": "us-east-1"}, Findings: []domain.Finding{finding}},
	}

	// With the queue already drained, handleAvailabilityChecked's all-checks-done
	// path dispatches TaskKindSaveCache. Gen must match the session's
	// AvailabilityGen (session.New seeds it at 1), or the event is dropped as stale.
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

	store := cache.LoadDirForTest("pilot-sweepenrich-prof", "us-east-1")
	tf, ok := store.Type("s3")
	if !ok {
		t.Fatal(`store.Type("s3") missing after a sweep+enrichment completion — C7: per-type persistence must not require a list screen to have been opened`)
	}
	if len(tf.Rows) == 0 {
		t.Error("s3 TypeFile.Rows is empty after a sweep completion with no list ever opened — C7: the sweep's fetched rows (with findings) must persist per-type without requiring a list-open, so a corrupt file self-heals on the next sweep, not only on the next list visit")
	}
	found := false
	for _, r := range tf.Rows {
		if r.ID == "bucket-sweepenrich-1" {
			found = true
			if len(r.Findings) != 1 || r.Findings[0].Code != "s3-public-read" {
				t.Errorf("persisted sweep-only row Findings = %+v, want 1 finding with Code=%q", r.Findings, "s3-public-read")
			}
		}
	}
	if !found {
		t.Error("persisted s3 TypeFile.Rows missing bucket-sweepenrich-1 — the sweep-fetched row was not persisted at all")
	}
}

func TestEnrichmentChecked_OpenList_FindingsReachPersistedCacheAndColdBootGlyph(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	s := session.New()
	s.Profile = "pilot-enrichchecked-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)

	finding := domain.Finding{
		Code:     "s3-public-read",
		Phrase:   "publicly readable",
		Severity: domain.SevBroken,
		Source:   "wave2:s3",
	}

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	baseRows := []resource.Resource{
		{ID: "bucket-enrichchecked-1", Name: "enrichchecked-bucket", Type: "s3", Fields: map[string]string{"region": "us-east-1"}},
	}
	ctrl.ApplyResourcesLoaded("s3", baseRows, nil, false)

	ctrl.WaitForCacheWrites()
	preStore := cache.LoadDirForTest("pilot-enrichchecked-prof", "us-east-1")
	preTF, ok := preStore.Type("s3")
	if !ok || len(preTF.Rows) != 1 {
		t.Fatalf("test setup: pre-enrichment s3 TypeFile missing or wrong row count: ok=%v rows=%+v", ok, preTF.Rows)
	}
	if len(preTF.Rows[0].Findings) != 0 {
		t.Fatalf("test setup: pre-enrichment s3 TypeFile.Rows[0].Findings = %+v, want empty (findings must not exist before enrichment runs)", preTF.Rows[0].Findings)
	}

	// TypeGen stays zero: EnrichmentChecked.AcceptZeroGen() is true and the
	// per-type gen guard only fires when TypeGen != 0.
	ctrl.Handle(messages.EnrichmentChecked{
		ResourceType: "s3",
		Findings:     map[string][]domain.Finding{"bucket-enrichchecked-1": {finding}},
	})

	// maybeSaveResourceListCache reads findings from ls.Rows and these rows carry
	// none, so any saved finding comes from the applied enrichment.
	ctrl.ApplyResourcesLoaded("s3", baseRows, nil, false)

	ctrl.WaitForCacheWrites()
	store := cache.LoadDirForTest("pilot-enrichchecked-prof", "us-east-1")
	tf, ok := store.Type("s3")
	if !ok {
		t.Fatal(`store.Type("s3") missing after enrichment + list-open save`)
	}
	if len(tf.Rows) != 1 {
		t.Fatalf("persisted s3 TypeFile.Rows has %d entries, want 1", len(tf.Rows))
	}
	if len(tf.Rows[0].Findings) != 1 || tf.Rows[0].Findings[0].Code != "s3-public-read" {
		t.Errorf("persisted s3 TypeFile.Rows[0].Findings = %+v, want 1 finding with Code=%q — C6: Wave-2 findings applied to an OPEN live list via the real EnrichmentChecked seam must reach ls.Rows/resourceCache so the list-open save path persists them, not just the session-side stores applyEnrichment writes to", tf.Rows[0].Findings, "s3-public-read")
	}

	s2 := session.New()
	s2.Profile = "pilot-enrichchecked-prof"
	s2.Region = "us-east-1"
	core2 := runtime.New(s2, resource.AllResourceTypes())
	ctrl2 := newBlessedController(t, core2)
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
		if lb.Rows[i].ResourceID == "bucket-enrichchecked-1" {
			found = true
			if lb.Rows[i].Severity == "" {
				t.Error("cold-boot seeded row for bucket-enrichchecked-1 has empty Severity — C6: the seeded row's persisted Findings must drive render-time severity classification, closing the loop back to render-time glyph classification")
			}
		}
	}
	if !found {
		t.Error("cold-boot seeded rows missing bucket-enrichchecked-1 — the persisted row was not seeded back at all")
	}
}

func readFileForAuditPilot(t *testing.T, path string) (string, error) {
	t.Helper()
	return readFileForAudit(t, path)
}

func itoaPilot(n int) string {
	return itoaColdBoot(n)
}
