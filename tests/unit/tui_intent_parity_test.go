// Internal/tui/app_dispatch.go's applyIntents
// forwards the whole intent slice to m.ctrl.ApplyIntents
// (Controller.applyIntents, core/app/intents.go) in one call, keeping a local
// switch only for adapter-only concerns (FlashIntent re-emit, ClearFlash,
// PushScreen/PopScreen renderer sync, PopSelectorIntent, ApplyThemeIntent,
// SetIdentityIntent, HeaderInvalidateIntent, PatchDetail).
//
// A local case for an intent kind the controller also handles applies it
// twice. For an ADDITIVE intent (PatchRelatedCache appends to a slice) that
// is a duplicated cache entry. For ABSOLUTE-SET intents
// (PatchMenuAvailability, PatchMenu — both assign ms.X = v.Y) a double-apply
// of the same value is idempotent and invisible in final state, so those are
// pinned by TUI-lane/headless-lane final-state parity.
//
// A sized tui.Model driven via rootApplyMsg (tuitest.Step) is the only
// reachable seam into internal/tui.Model's unexported applyIntents; the
// headless side drives core/app.Controller directly. Both lanes go through
// the entry points their production code uses (m.coreUpdate / c.Handle for
// AvailabilityPrefetched; Core.HandleRelatedCheckResult then applyIntents /
// c.ApplyIntents for RelatedCheckResult).
package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// newIntentParityHeadlessController builds a Controller+Core pair configured
// exactly like newTestControllerAndCore (app_patch_cache_intents_test.go)
// and newMenuController (app_menu_test.go): a fresh in-memory session, no
// disk cache interaction (no ApplyIntents/Handle call in this file reaches
// TaskKindSaveCache), so this stays hermetic.
func newIntentParityHeadlessController(t *testing.T) (*app.Controller, *runtime.Core) {
	t.Helper()
	s := session.New()
	s.Profile = "intent-parity-prof"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	return newBlessedController(t, core), core
}

// newIntentParityTUIModel builds a sized, demo-independent tui.Model exactly
// like newSaveCacheApp (tui_savecache_routing_test.go), minus the on-disk
// cache redirection this file's tests never need (no TaskKindSaveCache path
// is driven here).
func newIntentParityTUIModel(t *testing.T) tui.Model {
	t.Helper()
	m := newBlessedModel(t, "intent-parity-prof", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 100, Height: 40})
	return m
}

// relatedParityEvent builds a messages.RelatedCheckResult that, once run
// through Core.HandleRelatedCheckResult, produces all three cache-patch
// intents in a SINGLE call: PatchRelatedCache (SourceResourceID set),
// PatchResourceCache (CachedPages carries a "sg" page not yet cached), and
// PatchLazyResourceCache (LazyAddedResources carries a "kms" sparse add).
// Generation is left at the zero value deliberately —
// messages.RelatedCheckResult.AcceptZeroGen() is true (event.go), so a
// zero Gen is never treated as stale regardless of the session's seeded
// RelatedGen (session.New seeds RelatedGen at 1, not 0 — see
// docs/architecture.md "Test Architecture" gen-seed pitfall).
func relatedParityEvent() messages.RelatedCheckResult {
	return messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: "i-parity0001",
		DefDisplayName:   "Security Groups",
		Result:           resource.KnownRelated("sg", []string{"sg-parity1", "sg-parity2"}, false),
		CachedPages: map[string]resource.ResourceCacheEntry{
			"sg": {
				Resources: []resource.Resource{
					{ID: "sg-parity1", Type: "sg"},
					{ID: "sg-parity2", Type: "sg"},
				},
			},
		},
		LazyAddedResources: map[string][]resource.Resource{
			"kms": {
				{ID: "key-parity0001", Type: "kms"},
			},
		},
	}
}

// TestIntentParity_CachePatches_TUIEqualsHeadless drives the SAME
// RelatedCheckResult through the TUI lane (rootApplyMsg -> Model.Update ->
// handleRelatedCheckResult -> Core.HandleRelatedCheckResult ->
// m.applyIntents) and the headless lane (Controller.ApplyIntents fed the SAME
// intents the same Core.HandleRelatedCheckResult call produces), then asserts
// both lanes converge on identical observable cache state with exactly one
// RelatedCache entry under the key.
func TestIntentParity_CachePatches_TUIEqualsHeadless(t *testing.T) {
	tm := newIntentParityTUIModel(t)
	tm, _ = rootApplyMsg(tm, relatedParityEvent())

	tuiCore := tm.Core()
	tuiRelated, tuiRelatedHit := tuiCore.RelatedCacheGet(runtime.RelatedCacheKey("ec2", "i-parity0001"))
	tuiSG, tuiSGHit := tuiCore.ResourceCache("sg")
	tuiKMS, tuiKMSHit := tuiCore.LazyResourceCache("kms")

	ctrl, hCore := newIntentParityHeadlessController(t)
	intents, _ := hCore.HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
		ResourceType:       "ec2",
		SourceResourceID:   "i-parity0001",
		DefDisplayName:     "Security Groups",
		Result:             relatedParityEvent().Result,
		CachedPages:        relatedParityEvent().CachedPages,
		LazyAddedResources: relatedParityEvent().LazyAddedResources,
	})
	ctrl.ApplyIntents(intents)

	hRelated, hRelatedHit := hCore.RelatedCacheGet(runtime.RelatedCacheKey("ec2", "i-parity0001"))
	hSG, hSGHit := hCore.ResourceCache("sg")
	hKMS, hKMSHit := hCore.LazyResourceCache("kms")

	if !tuiRelatedHit {
		t.Fatal("TUI lane: RelatedCacheGet hit=false after RelatedCheckResult — PatchRelatedCache did not apply")
	}
	if len(tuiRelated) != 1 {
		t.Fatalf("TUI lane: RelatedCache entry count = %d, want exactly 1 — a count of 2 means applyIntents applied PatchRelatedCache twice (forwarded batch pass + a leftover local case)", len(tuiRelated))
	}
	if !hRelatedHit {
		t.Fatal("headless lane: RelatedCacheGet hit=false after ApplyIntents — PatchRelatedCache did not apply")
	}
	if len(hRelated) != 1 {
		t.Fatalf("headless lane: RelatedCache entry count = %d, want exactly 1 — a count of 2 means Controller.applyIntents applied PatchRelatedCache twice", len(hRelated))
	}

	if tuiRelated[0].DefDisplayName != hRelated[0].DefDisplayName {
		t.Errorf("RelatedCache[0].DefDisplayName: TUI=%q headless=%q, want equal", tuiRelated[0].DefDisplayName, hRelated[0].DefDisplayName)
	}
	if tuiRelated[0].Result.Count() != hRelated[0].Result.Count() {
		t.Errorf("RelatedCache[0].Result.Count: TUI=%d headless=%d, want equal", tuiRelated[0].Result.Count(), hRelated[0].Result.Count())
	}
	if tuiRelated[0].Result.Count() != 2 {
		t.Errorf("RelatedCache[0].Result.Count = %d, want 2 (exact mapping, not just non-zero)", tuiRelated[0].Result.Count())
	}

	if tuiSGHit != hSGHit {
		t.Fatalf("ResourceCache(\"sg\") hit: TUI=%v headless=%v, want equal", tuiSGHit, hSGHit)
	}
	if !tuiSGHit {
		t.Fatal("ResourceCache(\"sg\") hit=false on both lanes — PatchResourceCache (from CachedPages) did not apply")
	}
	if len(tuiSG.Resources) != len(hSG.Resources) || len(tuiSG.Resources) != 2 {
		t.Errorf("ResourceCache(\"sg\").Resources length: TUI=%d headless=%d, want both 2", len(tuiSG.Resources), len(hSG.Resources))
	}

	if tuiKMSHit != hKMSHit {
		t.Fatalf("LazyResourceCache(\"kms\") hit: TUI=%v headless=%v, want equal", tuiKMSHit, hKMSHit)
	}
	if !tuiKMSHit {
		t.Fatal("LazyResourceCache(\"kms\") hit=false on both lanes — PatchLazyResourceCache did not apply")
	}
	if len(tuiKMS) != len(hKMS) || len(tuiKMS) != 1 {
		t.Fatalf("LazyResourceCache(\"kms\") length: TUI=%d headless=%d, want both 1 — a length of 2 on either lane means PatchLazyResourceCache double-applied", len(tuiKMS), len(hKMS))
	}
	if tuiKMS[0].ID != "key-parity0001" || hKMS[0].ID != "key-parity0001" {
		t.Errorf("LazyResourceCache(\"kms\")[0].ID: TUI=%q headless=%q, want %q", tuiKMS[0].ID, hKMS[0].ID, "key-parity0001")
	}
}

// TestIntentParity_CachePatches_RepeatedApply_StaysAdditiveOnce:
// PatchRelatedCache is additive across separate related-check results, so a
// SECOND, distinct RelatedCheckResult for the same source resource yields
// exactly 2 entries — one per delivery, not one per intent-application pass.
func TestIntentParity_CachePatches_RepeatedApply_StaysAdditiveOnce(t *testing.T) {
	tm := newIntentParityTUIModel(t)
	tm, _ = rootApplyMsg(tm, relatedParityEvent())

	second := messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: "i-parity0001",
		DefDisplayName:   "IAM Roles",
		Result:           resource.KnownRelated("iam-role", []string{"role-parity1"}, false),
	}
	tm, _ = rootApplyMsg(tm, second)

	related, hit := tm.Core().RelatedCacheGet(runtime.RelatedCacheKey("ec2", "i-parity0001"))
	if !hit {
		t.Fatal("RelatedCacheGet hit=false after two distinct RelatedCheckResult deliveries")
	}
	if len(related) != 2 {
		t.Fatalf("RelatedCache entry count = %d, want exactly 2 (one per delivery) — got either a missing delivery or a double-apply within one delivery", len(related))
	}
	names := map[string]bool{related[0].DefDisplayName: true, related[1].DefDisplayName: true}
	if !names["Security Groups"] || !names["IAM Roles"] {
		t.Errorf("RelatedCache DefDisplayNames = %v, want both %q and %q present", []string{related[0].DefDisplayName, related[1].DefDisplayName}, "Security Groups", "IAM Roles")
	}
}

// availabilityPrefetchedParityEvent produces both PatchMenuAvailability and
// PatchMenu in one HandleEvent call (handleAvailabilityPrefetched,
// core/runtime/handlers_availability.go), reached identically by the
// TUI lane (m.coreUpdate) and the headless lane (Controller.Handle) since
// both call HandleEvent -> applyIntents. Gen is set to 1 to match
// session.New()'s seeded AvailabilityGen (AvailabilityPrefetched.AcceptZeroGen()
// is false, so Gen=0 here would be silently dropped as stale).
func availabilityPrefetchedParityEvent() messages.AvailabilityPrefetched {
	return messages.AvailabilityPrefetched{
		Entries:        map[string]int{"ec2": 7},
		Truncated:      map[string]bool{"ec2": false},
		IssueCounts:    map[string]int{"ec2": 3},
		IssueTruncated: map[string]bool{"ec2": false},
		Gen:            1,
	}
}

// TestIntentParity_MenuPatches_SingleApplication drives the same
// AvailabilityPrefetched event through the TUI lane (rootApplyMsg ->
// m.coreUpdate -> m.applyIntents) and the headless lane (ctrl.Handle ->
// c.applyIntents), then asserts both converge on the SAME menu availability
// + issue counts.
//
// PatchMenuAvailability and PatchMenu are ABSOLUTE-SET intents
// (Controller.applyIntents assigns ms.Availability[rt] = v.Count /
// ms.IssueCounts[rt] = v.Issues), so applying the same value twice is
// idempotent and invisible in final state. The double-apply hazard for
// additive intents is pinned by TestIntentParity_CachePatches_TUIEqualsHeadless.
func TestIntentParity_MenuPatches_SingleApplication(t *testing.T) {
	tm := newIntentParityTUIModel(t)
	tm, _ = rootApplyMsg(tm, availabilityPrefetchedParityEvent())
	tuiPlain := stripANSI(rootViewContent(tm))

	ctrl, _ := newIntentParityHeadlessController(t)
	ctrl.Handle(availabilityPrefetchedParityEvent())
	snap := ctrl.Snapshot()
	if snap.Body.Kind != app.BodyKindMenu || snap.Body.Menu == nil {
		t.Fatalf("headless lane: Snapshot().Body.Kind = %q, want BodyKindMenu with non-nil Menu", snap.Body.Kind)
	}
	var hEntry *app.MenuEntry
	for i := range snap.Body.Menu.Entries {
		if snap.Body.Menu.Entries[i].ShortName == "ec2" {
			hEntry = &snap.Body.Menu.Entries[i]
			break
		}
	}
	if hEntry == nil {
		t.Fatal("headless lane: ec2 entry not present in MenuBody.Entries after AvailabilityPrefetched")
	}

	if hEntry.Availability != 7 {
		t.Errorf("headless lane: ec2 entry Availability = %d, want 7", hEntry.Availability)
	}
	if hEntry.IssueBadge.Count != 3 {
		t.Errorf("headless lane: ec2 entry IssueBadge.Count = %d, want 3", hEntry.IssueBadge.Count)
	}

	if !strings.Contains(tuiPlain, "EC2 Instances (7)") {
		t.Errorf("TUI lane: rendered menu should contain %q after AvailabilityPrefetched(Count=7), got: %s", "EC2 Instances (7)", tuiPlain)
	}
	if !strings.Contains(tuiPlain, "issues:3") {
		t.Errorf("TUI lane: rendered menu should contain %q after AvailabilityPrefetched(IssueCounts=3), got: %s", "issues:3", tuiPlain)
	}
}
