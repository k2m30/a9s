// app_live_shaped_badge_repro_test.go — deterministic controller-level
// reproduction of a LIVE web-session bug report:
//
//	readonly-profile web session -> open s3 list -> wave-2 enrichment flags
//	5 rows (EnrichmentChecked processed, list shows flags) -> Escape to menu
//	-> s3 menu entry has issue_badge {count:0, truncated:true} and
//	origin:"cache" — the badge count is LOST, while the same flow in the TUI
//	shows the badge.
//
// This file mirrors the live web lane's own construction sequence
// (internal/web/construct.go newSession, live/non-demo branch) at the
// headless Controller level, matching the patterns already established by
// app_web_lane_menu_badge_test.go (PatchResourceList Issues-discard defect)
// and app_enrichment_menu_badge_test.go (ApplyEnrichmentState menu-sync
// contract) — but driven through the REAL production call chain
// (ctrl.Handle(messages.EnrichmentChecked{...}) -> Core.handleEnrichmentChecked
// -> PatchMenu/PatchResourceList intents -> Controller.applyIntents) instead
// of calling ApplyEnrichmentState or ApplyIntents directly, so a defect
// living anywhere in that chain (including internal/runtime's intent
// construction, not just internal/app's intent consumption) would surface
// here too.
//
// Both a GREEN and a RED outcome are informative here: GREEN would mean the
// bug does not reproduce at the Controller level and must live in
// internal/web's session/HTTP layer; RED pins the Controller-level mechanism
// precisely (which assertion fails, at which step, with which values).
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// newLiveShapedBadgeReproController builds a *runtime.Core + *app.Controller
// exactly the way internal/web/construct.go's newSession does for a LIVE
// (non-demo, no pre-supplied clients) web session: runtime.Bootstrap +
// app.New + SetUIMode("web") — mirrors newLiveWebStyleController in
// app_web_live_cold_boot_test.go, duplicated locally per that file's own
// stated precedent (no cross-file coupling to another test file's helper
// lifetime).
func newLiveShapedBadgeReproController(t *testing.T, profile, region string) (*runtime.Core, *app.Controller) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	core := runtime.Bootstrap(profile, region, resource.AllResourceTypes())
	ctrl := app.New(core)
	ctrl.SetUIMode("web")
	return core, ctrl
}

// seedReadonlyS3DiskCache writes a real on-disk s3 TypeFile for profile/region
// via cache.Store.Put + SaveType (the standard fixture-writing pattern per
// command_navigation_after_seed_test.go's seedDiskCacheWithS3Rows and
// app_web_live_cold_boot_test.go's inline store.Put/SaveType usage), shaped
// as the bug report describes the pre-existing cache state: availability
// known (Count 50, truncated) but issues UNKNOWN (IssuesKnown left false) —
// the C1 disk-cache seed a readonly-profile session would have from a prior
// run that only ever completed a Wave-1 availability sweep.
func seedReadonlyS3DiskCache(t *testing.T, profile, region string) *cache.Store {
	t.Helper()
	store := cache.LoadDir(profile, region)
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        50,
		Exact:        false,
		// IssuesKnown intentionally left false/zero: "issues UNKNOWN" per the
		// bug report's described pre-existing cache shape.
	})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("seed fixture SaveType(s3): %v", err)
	}
	return cache.LoadDir(profile, region)
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
// (internal/runtime/handlers_availability.go handleEnrichmentChecked).
//
// Severity is deliberately domain.SevBroken, not SevWarn: the real
// production menu-badge count for this path is computed by
// unifiedIssueCount (internal/runtime/handlers_availability.go), which only
// counts SevBroken findings toward the "!" badge — a SevWarn-only fixture
// would silently produce a badge count of 0 regardless of the defect under
// test, which is not what this repro is pinning.
func liveShapedBadgeReproWave2Findings(n int) map[string]domain.Finding {
	findings := make(map[string]domain.Finding, n)
	for i := range n {
		id := "arn:aws:s3:::a9s-repro-bucket-" + string(rune('a'+i))
		findings[id] = domain.Finding{
			Code:     "s3.public_access",
			Phrase:   "public access not blocked",
			Severity: domain.SevBroken,
			Source:   "wave2:s3",
		}
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

// TestLiveShapedBadgeRepro_S3_SurvivesEscapeToMenu reproduces the reported
// live web-session sequence at the headless Controller level:
//
//  1. C1 disk-cache seed (readonly-profile session's pre-existing s3 cache:
//     availability known/truncated, issues UNKNOWN) delivered via
//     ctrl.Handle(runtime.CacheStoreToEvent(store)) — mirrors
//     internal/web/construct.go's live-lane synchronous seed.
//  2. ClientsReady delivered via ctrl.Handle — mirrors BootstrapLive's
//     background connect completing.
//  3. Navigate to the s3 list (ActionCommand "s3") and deliver
//     ResourcesLoaded with 5 fictional buckets.
//  4. Deliver EnrichmentChecked exactly as the runtime emits it in
//     production (4 wave2-sourced findings, Truncated:true, Gen/TypeGen
//     both 0 — a fresh session's first-ever enrichment cycle for s3 has
//     never bumped EnrichmentTypeGen["s3"] off its zero default, so 0 is
//     what production actually stamps here, not a test shortcut).
//  5. Escape back to the menu (ActionBack).
//  6. Snapshot: the s3 menu entry must show IssueBadge.Count==4 and
//     IssueBadge.Known==true — the badge the bug report says is lost.
//
// Mid-flow (after step 4, before step 5) GetMenuIssueCounts()["s3"] must
// already be 4 — this separates "never set" (fails at step 4) from "set
// then lost on back" (passes at step 4, fails at step 6).
func TestLiveShapedBadgeRepro_S3_SurvivesEscapeToMenu(t *testing.T) {
	const profile, region = "repro-readonly-prof", "us-east-1"

	store := seedReadonlyS3DiskCache(t, profile, region)
	core, ctrl := newLiveShapedBadgeReproController(t, profile, region)

	// Step 1: C1 disk-cache seed.
	ctrl.Handle(runtime.CacheStoreToEvent(store))

	// Step 2: ClientsReady (background AWS connect completing).
	ctrl.Handle(messages.ClientsReady{
		Clients: nil,
		Region:  region,
		Gen:     core.ConnectGen(),
		Err:     nil,
	})

	// Step 3: navigate to s3 list, deliver ResourcesLoaded with 5 buckets.
	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	snapAfterList := ctrl.Snapshot()
	if snapAfterList.Body.Kind != app.BodyKindList {
		t.Fatalf("after ActionCommand(s3): Body.Kind = %q, want %q", snapAfterList.Body.Kind, app.BodyKindList)
	}

	buckets := liveShapedBadgeReproS3Resources(5)
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    buckets,
		Gen:          0,
	})

	snapAfterLoad := ctrl.Snapshot()
	if snapAfterLoad.Body.List == nil || len(snapAfterLoad.Body.List.Rows) != 5 {
		gotLen := -1
		if snapAfterLoad.Body.List != nil {
			gotLen = len(snapAfterLoad.Body.List.Rows)
		}
		t.Fatalf("after ResourcesLoaded: len(Body.List.Rows) = %d, want 5 — test setup failed to seed the list", gotLen)
	}

	// Step 4: EnrichmentChecked exactly as the runtime emits it in
	// production — 4 flagged rows out of 5 delivered, Truncated:true.
	findings := liveShapedBadgeReproWave2Findings(4)
	ctrl.Handle(messages.EnrichmentChecked{
		ResourceType: "s3",
		Issues:       4,
		Truncated:    true,
		Findings:     findings,
		Gen:          0,
		TypeGen:      0,
	})

	// Mid-flow assertion: the badge must be visible on the LIST screen's
	// underlying menu-sync state before any back-navigation happens at all.
	if got := ctrl.GetMenuIssueCounts()["s3"]; got != 4 {
		t.Errorf("after EnrichmentChecked, before back-navigation: GetMenuIssueCounts()[s3] = %d, want 4 — badge was never set by EnrichmentChecked in the first place (fails BEFORE any back-navigation is involved)", got)
	}
	if got := ctrl.GetMenuIssueKnown()["s3"]; !got {
		t.Errorf("after EnrichmentChecked, before back-navigation: GetMenuIssueKnown()[s3] = %v, want true", got)
	}

	// Step 5: Escape back to the menu.
	_, _ = ctrl.Apply(app.Action{Kind: app.ActionBack})

	// Step 6: the s3 menu entry must still show the wave-2 badge.
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

// TestLiveShapedBadgeRepro_S3_LateDiskCacheReplayDoesNotClobberFreshBadge
// covers the ordering variant the live session actually has when the C1
// disk-cache seed's background load (TaskKindLoadAvailCache, dispatched from
// handleClientsReadySuccess/handleAvailabilityCacheLoaded's live path) lands
// LATE — AFTER Wave-2 enrichment has already delivered a fresh badge, not
// before it as in the primary scenario above. The replayed disk store still
// reports "issues UNKNOWN" for s3 (no on-disk Wave-2 result was ever saved
// for this profile/region pair), so a naive unconditional
// overwrite-from-cache would clobber the just-landed, more current in-session
// badge back down to unknown/0 — exactly the {count:0, truncated:true}
// shape the bug report describes for the FINAL menu state.
//
// Sequence: seed (Step 1) is SKIPPED up front; instead the identical
// CacheStoreToEvent delivery is moved to AFTER EnrichmentChecked (Step 4) and
// BEFORE ActionBack (Step 5), while ClientsReady/list-open/enrichment (Steps
// 2-4) proceed unchanged.
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
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    buckets,
		Gen:          0,
	})

	findings := liveShapedBadgeReproWave2Findings(4)
	ctrl.Handle(messages.EnrichmentChecked{
		ResourceType: "s3",
		Issues:       4,
		Truncated:    true,
		Findings:     findings,
		Gen:          0,
		TypeGen:      0,
	})

	if got := ctrl.GetMenuIssueCounts()["s3"]; got != 4 {
		t.Fatalf("precondition failed: GetMenuIssueCounts()[s3] = %d, want 4 before the late disk-cache replay is delivered", got)
	}

	// Late-arriving C1 disk-cache seed replay: issues UNKNOWN for s3 in this
	// on-disk snapshot (no Wave-2 result was ever saved to disk for this
	// pair).
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
