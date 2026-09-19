// A Wave-2 issue badge set in a live web session (core/web/construct.go
// newSession, live branch) survives Escape back to the menu. The events travel
// the production chain — ctrl.Handle(messages.EnrichmentChecked{...}) ->
// Core.handleEnrichmentChecked -> PatchMenu/PatchResourceList intents ->
// Controller.applyIntents — so both core/runtime's intent construction and
// core/app's intent consumption are covered.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// newLiveShapedBadgeReproController builds a *runtime.Core + *app.Controller
// the way core/web/construct.go's newSession does for a live (non-demo, no
// pre-supplied clients) web session: runtime.Bootstrap + app.New +
// SetUIMode("web").
func newLiveShapedBadgeReproController(t *testing.T, profile, region string) (*runtime.Core, *app.Controller) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	core := runtime.Bootstrap(profile, region, resource.AllResourceTypes())
	ctrl := newBlessedController(t, core)
	t.Cleanup(ctrl.Close)
	ctrl.SetUIMode("web")
	return core, ctrl
}

// seedReadonlyS3DiskCache writes an on-disk s3 TypeFile for profile/region with
// availability known (Count 50, truncated) and issues unknown: the disk seed a
// readonly-profile session has after a prior run that only completed a Wave-1
// availability sweep.
func seedReadonlyS3DiskCache(t *testing.T, profile, region string) *cache.Store {
	t.Helper()
	store := cache.LoadDirForTest(profile, region)
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        50,
		Exact:        false,
	})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("seed fixture SaveType(s3): %v", err)
	}
	return cache.LoadDirForTest(profile, region)
}

// liveShapedBadgeReproS3Resources builds 5 fictional S3 bucket resources,
// matching the shape ResourcesLoaded carries for a top-level s3 list fetch.
func liveShapedBadgeReproS3Resources(n int) []resource.Resource {
	buckets := make([]resource.Resource, 0, n)
	for i := range n {
		id := "arn:aws:s3:::a9s-repro-bucket-" + string(rune('a'+i))
		buckets = append(buckets, resource.Resource{
			ID:   id,
			Name: "a9s-repro-bucket-" + string(rune('a'+i)),
			Type: "s3",
			Fields: map[string]string{
				"region": "us-east-1",
			},
		})
	}
	return buckets
}

// liveShapedBadgeReproWave2Findings builds n wave2-sourced findings keyed to
// the first n bucket IDs from liveShapedBadgeReproS3Resources, matching the
// shape EnrichmentChecked.Findings carries in production
// (core/runtime/handlers_availability.go handleEnrichmentChecked).
//
// Severity is SevBroken: unifiedIssueCount (core/runtime/handlers_availability.go)
// counts only SevBroken findings toward the "!" badge, so a SevWarn fixture
// would give a count of 0.
func liveShapedBadgeReproWave2Findings(n int) map[string][]domain.Finding {
	findings := make(map[string][]domain.Finding, n)
	for i := range n {
		id := "arn:aws:s3:::a9s-repro-bucket-" + string(rune('a'+i))
		findings[id] = []domain.Finding{{
			Code:     "s3.public_access",
			Phrase:   "public access not blocked",
			Severity: domain.SevBroken,
			Source:   "wave2:s3",
		}}
	}
	return findings
}

// liveShapedBadgeReproMenuEntry returns the MenuEntry for shortName from a
// MenuBody, or nil.
func liveShapedBadgeReproMenuEntry(mb *app.MenuBody, shortName string) *app.MenuEntry {
	if mb == nil {
		return nil
	}
	for i := range mb.Entries {
		if mb.Entries[i].ShortName == shortName {
			return &mb.Entries[i]
		}
	}
	return nil
}

// EnrichmentChecked carries Gen/TypeGen 0: a fresh session's first enrichment
// cycle for s3 never bumped EnrichmentTypeGen["s3"] off zero, so production
// stamps 0 here. GetMenuIssueCounts()["s3"] is checked after enrichment and
// the menu entry again after ActionBack, which separates a badge never set from
// one set and then lost on back.
func TestLiveShapedBadgeRepro_S3_SurvivesEscapeToMenu(t *testing.T) {
	const profile, region = "repro-readonly-prof", "us-east-1"

	store := seedReadonlyS3DiskCache(t, profile, region)
	core, ctrl := newLiveShapedBadgeReproController(t, profile, region)

	ctrl.Handle(runtime.CacheStoreToEvent(store))

	ctrl.Handle(messages.ClientsReady{
		Clients: nil,
		Region:  region,
		Gen:     core.ConnectGen(),
		Err:     nil,
	})

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	snapAfterList := ctrl.Snapshot()
	if snapAfterList.Body.Kind != app.BodyKindList {
		t.Fatalf("after ActionCommand(s3): Body.Kind = %q, want %q", snapAfterList.Body.Kind, app.BodyKindList)
	}

	buckets := liveShapedBadgeReproS3Resources(5)
	handlePage(ctrl, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    buckets,
		Gen:          0, Provenance: messages.FetchProvenanceCanonicalList,
	})

	snapAfterLoad := ctrl.Snapshot()
	if snapAfterLoad.Body.List == nil || len(snapAfterLoad.Body.List.Rows) != 5 {
		gotLen := -1
		if snapAfterLoad.Body.List != nil {
			gotLen = len(snapAfterLoad.Body.List.Rows)
		}
		t.Fatalf("after ResourcesLoaded: len(Body.List.Rows) = %d, want 5 — test setup failed to seed the list", gotLen)
	}

	findings := liveShapedBadgeReproWave2Findings(4)
	ctrl.Handle(messages.EnrichmentChecked{
		ResourceType: "s3",
		Truncated:    true,
		Findings:     findings,
		Gen:          0,
		TypeGen:      0,
	})

	if got := ctrl.GetMenuIssueCounts()["s3"]; got != 4 {
		t.Errorf("after EnrichmentChecked, before back-navigation: GetMenuIssueCounts()[s3] = %d, want 4 — badge was never set by EnrichmentChecked in the first place (fails BEFORE any back-navigation is involved)", got)
	}
	if got := ctrl.GetMenuIssueKnown()["s3"]; !got {
		t.Errorf("after EnrichmentChecked, before back-navigation: GetMenuIssueKnown()[s3] = %v, want true", got)
	}

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionBack})

	snapAfterBack := ctrl.Snapshot()
	if snapAfterBack.Body.Kind != app.BodyKindMenu {
		t.Fatalf("after ActionBack: Body.Kind = %q, want %q", snapAfterBack.Body.Kind, app.BodyKindMenu)
	}

	if got := ctrl.GetMenuIssueCounts()["s3"]; got != 4 {
		t.Errorf("after ActionBack: GetMenuIssueCounts()[s3] = %d, want 4 — this is the live bug: badge count lost on Escape-to-menu", got)
	}
	if got := ctrl.GetMenuIssueKnown()["s3"]; !got {
		t.Errorf("after ActionBack: GetMenuIssueKnown()[s3] = %v, want true", got)
	}

	entry := liveShapedBadgeReproMenuEntry(snapAfterBack.Body.Menu, "s3")
	if entry == nil {
		t.Fatal("after ActionBack: menu has no entry for s3")
	}
	if entry.IssueBadge.Count != 4 {
		t.Errorf("after ActionBack: s3 MenuEntry.IssueBadge.Count = %d, want 4 — this is the missing badge reported live (5 flagged rows, menu shows issue_badge count:0)", entry.IssueBadge.Count)
	}
	if !entry.IssueBadge.Truncated {
		t.Error("after ActionBack: s3 MenuEntry.IssueBadge.Truncated = false, want true")
	}
}

// The disk-cache seed's background load (TaskKindLoadAvailCache) can land after
// Wave-2 enrichment has set a fresh badge. The replayed store still reports
// issues unknown for s3, and replaying it keeps the in-session badge.
func TestLiveShapedBadgeRepro_S3_LateDiskCacheReplayDoesNotClobberFreshBadge(t *testing.T) {
	const profile, region = "repro-readonly-prof-late", "us-east-1"

	// Disk cache is written to the SAME pair the controller will load from,
	// but NOT delivered to the controller yet — that happens after
	// EnrichmentChecked, below.
	store := seedReadonlyS3DiskCache(t, profile, region)
	core, ctrl := newLiveShapedBadgeReproController(t, profile, region)

	ctrl.Handle(messages.ClientsReady{
		Clients: nil,
		Region:  region,
		Gen:     core.ConnectGen(),
		Err:     nil,
	})

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	buckets := liveShapedBadgeReproS3Resources(5)
	handlePage(ctrl, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    buckets,
		Gen:          0, Provenance: messages.FetchProvenanceCanonicalList,
	})

	findings := liveShapedBadgeReproWave2Findings(4)
	ctrl.Handle(messages.EnrichmentChecked{
		ResourceType: "s3",
		Truncated:    true,
		Findings:     findings,
		Gen:          0,
		TypeGen:      0,
	})

	if got := ctrl.GetMenuIssueCounts()["s3"]; got != 4 {
		t.Fatalf("precondition failed: GetMenuIssueCounts()[s3] = %d, want 4 before the late disk-cache replay is delivered", got)
	}

	// The disk seed replays late, with issues unknown for s3.
	ctrl.Handle(runtime.CacheStoreToEvent(store))

	if got := ctrl.GetMenuIssueCounts()["s3"]; got != 4 {
		t.Errorf("after late CacheStoreToEvent replay: GetMenuIssueCounts()[s3] = %d, want 4 — a stale disk-cache replay with issues UNKNOWN must not clobber the fresher in-session Wave-2 badge", got)
	}
	if got := ctrl.GetMenuIssueKnown()["s3"]; !got {
		t.Errorf("after late CacheStoreToEvent replay: GetMenuIssueKnown()[s3] = %v, want true — must not be knocked back to unknown", got)
	}

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionBack})

	snapAfterBack := ctrl.Snapshot()
	if snapAfterBack.Body.Kind != app.BodyKindMenu {
		t.Fatalf("after ActionBack: Body.Kind = %q, want %q", snapAfterBack.Body.Kind, app.BodyKindMenu)
	}
	entry := liveShapedBadgeReproMenuEntry(snapAfterBack.Body.Menu, "s3")
	if entry == nil {
		t.Fatal("after ActionBack: menu has no entry for s3")
	}
	if entry.IssueBadge.Count != 4 {
		t.Errorf("after ActionBack (late-replay variant): s3 MenuEntry.IssueBadge.Count = %d, want 4 — late disk-cache replay clobbered the fresh badge down to the on-disk unknown state", entry.IssueBadge.Count)
	}
}
