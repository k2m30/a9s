// tui_intent_parity_test.go — parity pins for the Wave-1 convergence of
// internal/tui/app_dispatch.go's applyIntents onto the headless controller's
// Controller.applyIntents (core/app/intents.go).
//
// Wave-1 change under pin: applyIntents forwards the ENTIRE intent slice to
// m.ctrl.ApplyIntents in one call (Controller.applyIntents already handles
// PatchMenuAvailability/PatchMenu/PatchResourceList/PatchResourceCache/
// PatchRelatedCache/PatchLazyResourceCache identically to the TUI-local
// cases it replaces), with a local switch surviving only for adapter-only
// concerns (FlashIntent re-emit, ClearFlash, PushScreen/PopScreen renderer
// sync, PopSelectorIntent, ApplyThemeIntent, SetIdentityIntent,
// HeaderInvalidateIntent, PatchDetail).
//
// Hazard under test: forwarding the whole batch to m.ctrl.ApplyIntents WHILE
// also keeping a leftover local case for the same intent kind would apply
// that intent twice. For an ADDITIVE intent (PatchRelatedCache appends to a
// slice) a double-apply is directly observable as a duplicated cache entry;
// this file's first test pins exactly that. For ABSOLUTE-SET intents
// (PatchMenuAvailability, PatchMenu — both assign ms.X = v.Y rather than
// accumulate) a double-apply of the SAME intent value is idempotent and
// therefore invisible via final state; the second test instead pins
// TUI-lane/headless-lane FINAL-STATE PARITY for those intents (the actual
// correctness property the collapse must preserve) and documents why a
// literal double-invocation cannot be distinguished from a single one this
// way.
//
// Harness: mirrors tests/unit/tui_savecache_routing_test.go — a sized
// tui.Model driven via rootApplyMsg (tuitest.Step) is the ONLY reachable
// seam into internal/tui.Model's unexported applyIntents; the headless
// comparison side drives core/app.Controller directly (mirrors
// tests/unit/app_patch_cache_intents_test.go's newTestControllerAndCore and
// tests/unit/app_menu_test.go's newMenuController). Both lanes are driven
// through the SAME entry-point shape their respective production code
// uses (m.coreUpdate / c.Handle for AvailabilityPrefetched;
// m.handleRelatedCheckResult's Core.HandleRelatedCheckResult call /
// c.core.HandleRelatedCheckResult+c.ApplyIntents for RelatedCheckResult),
// so both actually exercise applyIntents/Controller.applyIntents rather than
// a synthetic hand-built intent slice.
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
	return app.New(core), core
}

// newIntentParityTUIModel builds a sized, demo-independent tui.Model exactly
// like newSaveCacheApp (tui_savecache_routing_test.go), minus the on-disk
// cache redirection this file's tests never need (no TaskKindSaveCache path
// is driven here).
func newIntentParityTUIModel(t *testing.T) tui.Model {
	t.Helper()
	m := tui.New("intent-parity-prof", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 100, Height: 40})
	return m
}

// ─────────────────────────────────────────────────────────────────────────
// 1. IntentParity_CachePatches_TUIEqualsHeadless
// ─────────────────────────────────────────────────────────────────────────

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
		Result: resource.RelatedCheckResult{
			TargetType:  "sg",
			Count:       2,
			ResourceIDs: []string{"sg-parity1", "sg-parity2"},
		},
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
// m.applyIntents, the exact function the Wave-1 collapse rewrites) and the
// headless lane (Controller.ApplyIntents fed the SAME intents the same
// Core.HandleRelatedCheckResult call produces), then asserts both lanes
// converge on identical observable cache state — and, critically, that
// neither lane double-applies the additive PatchRelatedCache intent: a
// double-apply would leave 2 RelatedCache entries under the same key
// instead of 1.
func TestIntentParity_CachePatches_TUIEqualsHeadless(t *testing.T) {
	// --- TUI lane ---
	tm := newIntentParityTUIModel(t)
	tm, _ = rootApplyMsg(tm, relatedParityEvent())

	tuiCore := tm.Core()
	tuiRelated, tuiRelatedHit := tuiCore.RelatedCacheGet(runtime.RelatedCacheKey("ec2", "i-parity0001"))
	tuiSG, tuiSGHit := tuiCore.ResourceCache("sg")
	tuiKMS, tuiKMSHit := tuiCore.LazyResourceCache("kms")

	// --- Headless lane ---
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

	// --- Single-application pins (the load-bearing assertions) ---
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

	// --- Parity: both lanes must converge on identical observable state ---
	if tuiRelated[0].DefDisplayName != hRelated[0].DefDisplayName {
		t.Errorf("RelatedCache[0].DefDisplayName: TUI=%q headless=%q, want equal", tuiRelated[0].DefDisplayName, hRelated[0].DefDisplayName)
	}
	if tuiRelated[0].Result.Count != hRelated[0].Result.Count {
		t.Errorf("RelatedCache[0].Result.Count: TUI=%d headless=%d, want equal", tuiRelated[0].Result.Count, hRelated[0].Result.Count)
	}
	if tuiRelated[0].Result.Count != 2 {
		t.Errorf("RelatedCache[0].Result.Count = %d, want 2 (exact mapping, not just non-zero)", tuiRelated[0].Result.Count)
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

// TestIntentParity_CachePatches_RepeatedApply_StaysAdditiveOnce guards the
// companion property to the double-apply hazard above: PatchRelatedCache is
// intentionally additive ACROSS separate related-check results (e.g. two
// different related-def checks for the same source resource both append),
// so the "exactly 1" pin above must not be read as "RelatedCache can never
// grow" — it grows once per distinct RelatedCheckResult delivery, not per
// intent-application pass within a single delivery. Sending a SECOND,
// distinct RelatedCheckResult for the same source resource must yield
// exactly 2 entries (one per delivery), proving the earlier "exactly 1"
// pin is measuring double-application-per-delivery, not accidentally
// pinning a cap that would mask a legitimate second delivery.
func TestIntentParity_CachePatches_RepeatedApply_StaysAdditiveOnce(t *testing.T) {
	tm := newIntentParityTUIModel(t)
	tm, _ = rootApplyMsg(tm, relatedParityEvent())

	second := messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: "i-parity0001",
		DefDisplayName:   "IAM Roles",
		Result: resource.RelatedCheckResult{
			TargetType:  "iam-role",
			Count:       1,
			ResourceIDs: []string{"role-parity1"},
		},
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

// ─────────────────────────────────────────────────────────────────────────
// 2. IntentParity_MenuPatches_SingleApplication
// ─────────────────────────────────────────────────────────────────────────

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
// Double-apply caveat: PatchMenuAvailability and PatchMenu are ABSOLUTE-SET
// intents (Controller.applyIntents assigns ms.Availability[rt] = v.Count /
// ms.IssueCounts[rt] = v.Issues, never accumulates) — applying the identical
// intent value twice within one applyIntents call is idempotent and
// therefore produces the SAME final count as applying it once. This test
// therefore cannot itself distinguish "applied once" from "applied twice
// with identical inputs" via final state; what it DOES pin is TUI/headless
// final-state parity, which is the actual correctness contract the Wave-1
// forward-everything collapse must preserve (a regression that routed menu
// patches through a DIFFERENT path on one lane, or dropped them on one
// lane, would show up here as a mismatch or a missing entry). The
// double-apply hazard for the collapse's additive intents is pinned by
// TestIntentParity_CachePatches_TUIEqualsHeadless above.
func TestIntentParity_MenuPatches_SingleApplication(t *testing.T) {
	// --- TUI lane ---
	tm := newIntentParityTUIModel(t)
	tm, _ = rootApplyMsg(tm, availabilityPrefetchedParityEvent())
	tuiPlain := stripANSI(rootViewContent(tm))

	// --- Headless lane ---
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

	// --- Exact-value pins (not just non-zero) ---
	if hEntry.Availability != 7 {
		t.Errorf("headless lane: ec2 entry Availability = %d, want 7", hEntry.Availability)
	}
	if hEntry.IssueBadge.Count != 3 {
		t.Errorf("headless lane: ec2 entry IssueBadge.Count = %d, want 3", hEntry.IssueBadge.Count)
	}

	// --- TUI/headless parity via the rendered menu row ---
	if !strings.Contains(tuiPlain, "EC2 Instances (7)") {
		t.Errorf("TUI lane: rendered menu should contain %q after AvailabilityPrefetched(Count=7), got: %s", "EC2 Instances (7)", tuiPlain)
	}
	if !strings.Contains(tuiPlain, "issues:3") {
		t.Errorf("TUI lane: rendered menu should contain %q after AvailabilityPrefetched(IssueCounts=3), got: %s", "issues:3", tuiPlain)
	}
}
