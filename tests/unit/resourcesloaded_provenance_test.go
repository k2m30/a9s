// resourcesloaded_provenance_test.go — behavior pins for
// messages.ResourcesLoaded.Provenance (FetchProvenance), the fix for the
// defect where a filtered/by-ID/child fetch result was indistinguishable
// from a canonical top-level list fetch, so the RowStore write accepted
// EVERY one into the shared per-type RowStore as a canonical full replace.
// Reproduced: an EC2 list paged to 200 exact rows, then a filtered related
// drill returning 3 rows replaced the entry — 3 rows, TotalCount 200→3 —
// surviving to disk and restart.
//
// Every non-canonical-provenance pin here drives the REAL Controller.Handle
// -> handleResourcesLoadedEvent -> RowStore path (never pokes the
// provenance gate or the RowStore directly), and reads back
// BOTH the in-memory RowStore snapshot AND the persisted on-disk TypeFile —
// this is the regression that reached disk and survived restart, so a
// unit-level stub on the gate function would have passed happily while the
// bug was live.
// NOTE on delivery: a page reaches the screen whose instance it names, at any
// depth in the stack. Most deliveries here are for the screen on top and use
// handlePage, which stamps that instance; the two aimed at a screen further
// down name it explicitly, captured when that screen was opened. Neither is a
// by-type shortcut — the identity is what the routing reads, and a test that
// left it off would be asserting about a page nothing dispatched.
package unit

import (
	"context"
	"strconv"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws" // ensure all related/fetcher registrations run
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// ────────────────────────────────────────────────────────────────────────────
// helpers
// ────────────────────────────────────────────────────────────────────────────

const provenancePinType = "ec2"

// newProvenancePinController builds a fresh Controller/Core pair via the
// blessed newDetailParityHeadlessController helper (tui_detail_parity_test.go,
// same "unit" package) rather than calling app.New directly — each
// provenance pin gets its own isolated temp cache dir (that helper's own
// t.Setenv/t.TempDir), so subtests never share RowStore/disk state.
// profile/region mirror the helper's own "detail-parity-prof"/"us-east-1"
// so provenancePinReadTypeFile reads the same pair.
func newProvenancePinController(t *testing.T) (*app.Controller, *runtime.Core, string, string) {
	t.Helper()
	ctrl, core := newDetailParityHeadlessController(t)
	return ctrl, core, "detail-parity-prof", "us-east-1"
}

// provenancePinEC2Rows builds n synthetic ec2 resources with distinct IDs.
func provenancePinEC2Rows(n int, prefix string) []resource.Resource {
	rows := make([]resource.Resource, n)
	for i := range rows {
		id := prefix + "-" + strconv.Itoa(i)
		rows[i] = resource.Resource{ID: id, Name: id, Type: provenancePinType}
	}
	return rows
}

// provenancePinReadTypeFile re-reads the on-disk TypeFile for shortName,
// failing the test if missing — the disk-reached half of every assertion
// here, since a corrupted count/rows persisted to disk survives restart.
// The per-type save does not run on the goroutine that triggered it: a
// 6000-row type file's marshal would sit in the latency of the fetch that
// produced it, so the cache writer owns it. A test that reads the file a
// call it just made produces therefore waits for that writer first —
// re-reading the directory without the barrier returns whatever the
// PREVIOUS save left, which reads as a passing assertion against stale
// bytes.
func provenancePinReadTypeFile(t *testing.T, ctrl *app.Controller, profile, region, shortName string) cache.TypeFile {
	t.Helper()
	ctrl.WaitForCacheWrites()
	store := cache.LoadDirForTest(profile, region)
	tf, ok := store.Type(shortName)
	if !ok {
		t.Fatalf("cache.LoadDirForTest(%q, %q).Type(%q) missing — expected a persisted TypeFile", profile, region, shortName)
	}
	return tf
}

// seedCanonical200 opens the ec2 list and delivers a 200-exact-row canonical
// ResourcesLoaded through the real Controller.Handle path, then asserts both
// the in-memory RowStore snapshot and the persisted disk TypeFile show
// exactly 200 rows with TotalCount/Count 200 — the precondition every
// subtest below builds on.
func seedCanonical200(t *testing.T, ctrl *app.Controller, core *runtime.Core, profile, region string) domain.Gen {
	t.Helper()
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: provenancePinType})
	handlePage(ctrl, messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(200, "i-canon"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	snap := core.Session().RowStore.Snapshot(provenancePinType)
	if len(snap.Rows) != 200 || snap.TotalCount != 200 {
		t.Fatalf("precondition failed: RowStore snapshot after canonical seed = %d rows, TotalCount=%d, want 200/200", len(snap.Rows), snap.TotalCount)
	}
	tf := provenancePinReadTypeFile(t, ctrl, profile, region, provenancePinType)
	if len(tf.Rows) != 200 || tf.Count != 200 {
		t.Fatalf("precondition failed: disk TypeFile after canonical seed = %d rows, Count=%d, want 200/200", len(tf.Rows), tf.Count)
	}
	// The identity of the screen this seed opened, for a subtest that later
	// delivers to it from underneath something else.
	return ctrl.GetListInstance()
}

// assertStillCanonical200 re-reads both the RowStore snapshot and the disk
// TypeFile and fails if either has moved away from the 200-row canonical
// seed — the exact shape of the regression that reached disk.
func assertStillCanonical200(t *testing.T, ctrl *app.Controller, core *runtime.Core, profile, region, label string) {
	t.Helper()
	snap := core.Session().RowStore.Snapshot(provenancePinType)
	if len(snap.Rows) != 200 || snap.TotalCount != 200 {
		t.Errorf("%s: RowStore entry = %d rows, TotalCount=%d, want 200/200 unchanged — a non-canonical result must never replace the canonical population", label, len(snap.Rows), snap.TotalCount)
	}
	tf := provenancePinReadTypeFile(t, ctrl, profile, region, provenancePinType)
	if len(tf.Rows) != 200 || tf.Count != 200 {
		t.Errorf("%s: disk TypeFile = %d rows, Count=%d, want 200/200 unchanged — this is the regression that reached disk and survived restart", label, len(tf.Rows), tf.Count)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Real 200-then-3 repro: FilteredList does not replace the canonical entry
// ────────────────────────────────────────────────────────────────────────────

// TestObserveResourcesLoadedRows_FilteredDrill_RealRepro_DoesNotReplaceCanonical
// drives the ACTUAL bug repro end to end: a top-level ec2 list loads 200
// exact rows, then a real stacked related-filtered ec2 list is pushed via
// the production ActionRelatedSelect -> dispatchRelatedNavigate ->
// applyRelatedNavResult seam (mirrors rowstore_stage4_pins_test.go's
// pushStackedRelatedFilteredS3List), and a 3-row filtered result lands
// through the real Controller.Handle path. Before the fix this collapsed
// the shared "ec2" RowStore entry (and its disk persistence) from 200 to 3.
func TestObserveResourcesLoadedRows_FilteredDrill_RealRepro_DoesNotReplaceCanonical(t *testing.T) {
	ctrl, core, profile, region := newProvenancePinController(t)
	seedCanonical200(t, ctrl, core, profile, region)

	// Push a stacked, related, filtered ec2 list — same production seam
	// pushStackedRelatedFilteredS3List uses: a detail view (an arbitrary
	// different source type, "sg") with one actionable DetailRelatedRow
	// targeting "ec2" with a multi-ID, cache-miss RelatedIDs set, selected
	// via ActionRelatedSelect. This is NavigationKindFilteredList per
	// ResolveRelatedNavigate (handlers_related.go), and sets ls.EscPops=true
	// (core/app/navigate.go's applyRelatedNavResult).
	ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	ctrl.EnsureDetailState(resource.Resource{ID: "sg-provenance-src", Name: "sg-provenance-src", Type: "sg"}, "sg")
	ctrl.ApplyDetailRelated([]app.DetailRelatedRow{
		{TargetType: provenancePinType, DisplayName: "EC2 Instances", Count: 3, ResourceIDs: []string{"i-filtered-1", "i-filtered-2", "i-filtered-3"}},
	})
	ctrl.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})

	if !ctrl.GetListEscPops() {
		t.Fatal("test setup problem: EscPops must be true after pushing a related-filtered ec2 list — the stacked screen is not registering as non-canonical")
	}

	// The filtered fetch result: same resourceType "ec2", 3 rows, the
	// provenance a real KindFetchFiltered task would carry.
	handlePage(ctrl, messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(3, "i-filtered"),
		Provenance:   messages.FetchProvenanceFilteredList,
	})

	assertStillCanonical200(t, ctrl, core, profile, region, "filtered-drill 3-row result")
}

// ────────────────────────────────────────────────────────────────────────────
// The pin that distinguishes `continue` from `return` at handle.go:298: a
// non-canonical result whose real target screen is BURIED UNDER a fresh
// canonical screen of the SAME type must still reach it.
// ────────────────────────────────────────────────────────────────────────────

// TestObserveResourcesLoadedRows_FilteredScreenBuriedUnderFreshCanonical_StillReceivesResult
// builds a THREE-deep same-type stack — [screen1: canonical ec2 200 rows]
// -> [screen2: related-filtered ec2, pushed via the real ActionRelatedSelect
// seam, EscPops=true] -> [screen3: a FRESH canonical ec2 list, pushed via
// ActionCommand on top of screen2, exactly as a user pressing ":ec2" again
// while deep in a related-panel drill would] — then delivers a
// FilteredList-provenance result for screen2's 3 target IDs.
//
// This is the one scenario the earlier FilteredDrill repro above cannot
// cover: there, the filtered screen was already on top, so both `continue`
// (skip a mismatched canonical screen and keep scanning) and a naive
// `return` (stop at the first type match, mismatched or not) would reach
// it identically — nothing distinguishes them when the target is already
// topmost. Here, screen3's canonical Provenance mismatch MUST be skipped via
// `continue` for the scan to ever reach screen2 at all; a `return` at
// handle.go:298 (treating the topLevelCanonical mismatch as "stop scanning,
// nothing more to do" instead of "this screen isn't it, keep looking")
// would swallow the message entirely — screen2 would strand on whatever it
// showed before, and screen3's own placeholder would incorrectly gain
// nothing either (msg.Provenance still isn't CanonicalList, so it can never
// legitimately apply to screen3 regardless of scan direction).
func TestObserveResourcesLoadedRows_FilteredScreenBuriedUnderFreshCanonical_StillReceivesResult(t *testing.T) {
	ctrl, core, profile, region := newProvenancePinController(t)
	seedCanonical200(t, ctrl, core, profile, region)

	// screen2: related-filtered ec2, pushed via the same real seam as the
	// FilteredDrill repro above.
	ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	ctrl.EnsureDetailState(resource.Resource{ID: "sg-buried-src", Name: "sg-buried-src", Type: "sg"}, "sg")
	// provenancePinEC2Rows 0-indexes its generated IDs (prefix-0, prefix-1,
	// prefix-2 for n=3) — ResourceIDs must match exactly, or the
	// RelatedIDSet display prefilter (list_filter.go) hides the mismatched
	// row and this test would misreport a delivery bug that is actually
	// just an ID-numbering mismatch in the test's own setup.
	ctrl.ApplyDetailRelated([]app.DetailRelatedRow{
		{TargetType: provenancePinType, DisplayName: "EC2 Instances", Count: 3, ResourceIDs: []string{"i-buried-0", "i-buried-1", "i-buried-2"}},
	})
	ctrl.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})
	if !ctrl.GetListEscPops() {
		t.Fatal("test setup problem: screen2 (related-filtered ec2) must have EscPops=true")
	}
	screen2 := ctrl.GetListInstance()

	// screen3: a FRESH canonical ec2 list pushed on top of screen2 via the
	// same command-palette seam seedCanonical200 used to build screen1 —
	// the exact "user jumps straight to :ec2 again while deep in a drill"
	// scenario handle.go's own doc comment describes.
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: provenancePinType})
	if ctrl.GetListEscPops() {
		t.Fatal("test setup problem: screen3 (fresh :ec2 command navigation) must be topLevelCanonical (EscPops=false)")
	}
	// Cache-first seeding immediately shows screen3 with the RowStore's
	// existing 200-row canonical entry (same session, same type) — its own
	// fresh fetch has not landed yet, but the cache-seeded rows are already
	// there. That is fine for this pin: what matters below is that this
	// 200-row seed stays put, proving the FilteredList delivery never
	// touched screen3.
	preDeliverySnap := ctrl.Snapshot()
	if preDeliverySnap.Body.List == nil || len(preDeliverySnap.Body.List.Rows) != 200 {
		got := 0
		if preDeliverySnap.Body.List != nil {
			got = len(preDeliverySnap.Body.List.Rows)
		}
		t.Fatalf("test setup problem: screen3 must start cache-seeded from the shared 200-row canonical entry, got %d rows", got)
	}

	// Deliver the FilteredList result for screen2's buried target IDs while
	// screen3 (mismatched canonical) sits on top.
	buriedRows := provenancePinEC2Rows(3, "i-buried")
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    buriedRows,
		Provenance:   messages.FetchProvenanceFilteredList,
		ScreenID:     screen2,
	})

	// screen3 (still topmost) must be completely untouched: a
	// FilteredList-provenance result is never legitimate data for a
	// topLevelCanonical screen, scan direction aside — its cache-seeded
	// 200 rows must survive unchanged (never overwritten by the 3 buried-
	// target rows, never merged with them).
	afterDeliverySnap := ctrl.Snapshot()
	if afterDeliverySnap.Body.List == nil || len(afterDeliverySnap.Body.List.Rows) != 200 {
		got := 0
		if afterDeliverySnap.Body.List != nil {
			got = len(afterDeliverySnap.Body.List.Rows)
		}
		t.Errorf("screen3 (fresh canonical, on top) rows = %d, want 200 unchanged — a FilteredList result must never populate a topLevelCanonical screen", got)
	}

	// Pop screen3 and confirm screen2 (buried at delivery time) actually
	// received the 3 rows — this is the assertion a `return` at
	// handle.go:298 would fail: the message would have been swallowed at
	// screen3 and never reached screen2 at all.
	ctrl.Apply(app.Action{Kind: app.ActionBack})
	screen2Snap := ctrl.Snapshot()
	if screen2Snap.Body.List == nil || len(screen2Snap.Body.List.Rows) != 3 {
		got := 0
		if screen2Snap.Body.List != nil {
			got = len(screen2Snap.Body.List.Rows)
		}
		t.Fatalf("screen2 (buried filtered ec2 list) rows = %d, want 3 — the FilteredList result never reached its real target screen", got)
	}
	gotIDs := map[string]bool{}
	for _, r := range screen2Snap.Body.List.Rows {
		gotIDs[r.ResourceID] = true
	}
	for _, want := range buriedRows {
		if !gotIDs[want.ID] {
			t.Errorf("screen2 rows missing expected buried-target ID %q, got %v", want.ID, gotIDs)
		}
	}

	// Pop screen2 and confirm screen1 (the original canonical 200-row list)
	// was never touched by any of the above — both the per-screen Rows and
	// the shared RowStore/disk entry.
	ctrl.Apply(app.Action{Kind: app.ActionBack})
	assertStillCanonical200(t, ctrl, core, profile, region, "after a buried-screen FilteredList delivery two levels down")
}

// ────────────────────────────────────────────────────────────────────────────
// The SYMMETRIC direction: a genuine CanonicalList result must not overwrite
// a topmost filtered/child screen, and must fall through to the canonical
// screen beneath it instead. handle.go's gate is symmetric
// (topLevelCanonical != msg.Provenance.CanonicalList()), not a
// one-directional "reject non-canonical on a canonical screen" check; a late
// canonical refresh landing while the user sits in a related-panel drill
// must not match the topmost (filtered) screen — the first same-type screen
// the top-down scan finds — and overwrite the drill with the unrelated
// canonical population.
// ────────────────────────────────────────────────────────────────────────────

// TestObserveResourcesLoadedRows_CanonicalResult_TopmostFilteredScreen_FallsThroughToCanonicalBeneath
// pushes a stacked related-filtered ec2 drill (same production seam as the
// FilteredDrill repro above) on top of the 200-row canonical seed, seeds the
// drill with its OWN distinct 3-row result, then delivers a fresh
// CanonicalList result (250 rows) while the drill is still topmost. Asserts
// the drill is untouched and the canonical result instead reaches screen1 —
// both in the per-screen Snapshot after popping back, and in the shared
// RowStore/disk TypeFile.
func TestObserveResourcesLoadedRows_CanonicalResult_TopmostFilteredScreen_FallsThroughToCanonicalBeneath(t *testing.T) {
	ctrl, core, profile, region := newProvenancePinController(t)
	screen1 := seedCanonical200(t, ctrl, core, profile, region)

	ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	ctrl.EnsureDetailState(resource.Resource{ID: "sg-reverse-src", Name: "sg-reverse-src", Type: "sg"}, "sg")
	ctrl.ApplyDetailRelated([]app.DetailRelatedRow{
		{TargetType: provenancePinType, DisplayName: "EC2 Instances", Count: 3, ResourceIDs: []string{"i-drill-0", "i-drill-1", "i-drill-2"}},
	})
	ctrl.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})
	if !ctrl.GetListEscPops() {
		t.Fatal("test setup problem: the pushed related-filtered ec2 list must have EscPops=true")
	}
	handlePage(ctrl, messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(3, "i-drill"),
		Provenance:   messages.FetchProvenanceFilteredList,
	})
	drillSnap := ctrl.Snapshot()
	if drillSnap.Body.List == nil || len(drillSnap.Body.List.Rows) != 3 {
		got := 0
		if drillSnap.Body.List != nil {
			got = len(drillSnap.Body.List.Rows)
		}
		t.Fatalf("test setup problem: the drill screen must show its own 3 rows before the reverse pin fires, got %d", got)
	}

	// A late CanonicalList result for "ec2" lands while the filtered drill is
	// still topmost — e.g. a stale top-level refresh continuation that was
	// already in flight when the user navigated into the drill.
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(250, "i-late-canon"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
		ScreenID:     screen1,
	})

	// The drill (still topmost) must be completely untouched.
	afterSnap := ctrl.Snapshot()
	if afterSnap.Body.List == nil || len(afterSnap.Body.List.Rows) != 3 {
		got := 0
		if afterSnap.Body.List != nil {
			got = len(afterSnap.Body.List.Rows)
		}
		t.Errorf("drill screen (still topmost) rows = %d after a late CanonicalList result landed, want 3 unchanged — a CanonicalList result must never overwrite the filtered drill the user is sitting in", got)
	}

	// It must instead have fallen through PAST the drill to the canonical
	// screen beneath it: pop back and confirm screen1 picked up the fresh
	// 250-row canonical population, both per-screen and on disk.
	ctrl.Apply(app.Action{Kind: app.ActionBack}) // pop drill -> detail
	ctrl.Apply(app.Action{Kind: app.ActionBack}) // pop detail -> canonical screen1

	screen1Snap := ctrl.Snapshot()
	if screen1Snap.Body.List == nil || len(screen1Snap.Body.List.Rows) != 250 {
		got := 0
		if screen1Snap.Body.List != nil {
			got = len(screen1Snap.Body.List.Rows)
		}
		t.Errorf("canonical screen1 rows after popping back = %d, want 250 — the late CanonicalList result must fall through past the topmost drill and land on the canonical screen beneath it", got)
	}

	snap := core.Session().RowStore.Snapshot(provenancePinType)
	if len(snap.Rows) != 250 || snap.TotalCount != 250 {
		t.Errorf("RowStore entry after the late CanonicalList result = %d rows, TotalCount=%d, want 250/250", len(snap.Rows), snap.TotalCount)
	}
	tf := provenancePinReadTypeFile(t, ctrl, profile, region, provenancePinType)
	if len(tf.Rows) != 250 || tf.Count != 250 {
		t.Errorf("disk TypeFile after the late CanonicalList result = %d rows, Count=%d, want 250/250", len(tf.Rows), tf.Count)
	}
}

// TestObserveResourcesLoadedRows_CanonicalResult_TopmostFilteredScreen_DoesNotSeedFilteredCache
// covers the second half of the symmetric gate: a CanonicalList result must
// not populate a topmost filtered screen's OWN FilteredRowsSet cache either
// (handle.go only calls FilteredRowsSet when !topLevelCanonical — a
// CanonicalList result reaching this screen at all would be the bug). Uses
// the FetchFilter-carrying drill shape (mirrors
// TestWebLane_FilteredDrill_SetsEscPops_AndFilteredRowsSetFires) since
// FilteredRowsSet only fires for that sub-case, not the RelatedIDs one above.
func TestObserveResourcesLoadedRows_CanonicalResult_TopmostFilteredScreen_DoesNotSeedFilteredCache(t *testing.T) {
	ctrl, core, _, _ := newProvenancePinController(t)

	filter := map[string]string{"Username": "bob-reverse-provenance-test"}

	ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	ctrl.EnsureDetailState(resource.Resource{ID: "ec2-reverse-src", Name: "ec2-reverse-src", Type: "ec2"}, "ec2")
	ctrl.ApplyDetailRelated([]app.DetailRelatedRow{
		{TargetType: "ct-events", DisplayName: "CloudTrail Events", State: domain.RelatedDeferred, FetchFilter: filter},
	})
	ctrl.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})
	if !ctrl.GetListEscPops() {
		t.Fatal("test setup problem: a FetchFilter-carrying related drill must have EscPops=true")
	}

	// A CanonicalList-provenance result for "ct-events" lands while the
	// FetchFilter drill is still topmost. No canonical ct-events screen was
	// ever opened, so this page names none — deliberately unstamped, and it
	// reaches no screen. It is still observed into the row store; what it must
	// not do is seed the drill's filtered cache.
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources: []resource.Resource{
			{ID: "evt-reverse-1", Type: "ct-events"},
		},
		Provenance: messages.FetchProvenanceCanonicalList,
	})

	if _, ok := core.FilteredRowsGet("ct-events", filter); ok {
		t.Error("FilteredRowsGet(\"ct-events\", filter) ok=true after a CanonicalList-provenance result landed — a canonical result must never populate a filtered drill's own filtered-rows cache")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// ByID and Child provenance: same property, but the isolation is DIFFERENT
// from the filtered-drill case above and that difference is deliberate, not
// an oversight — see the comment on popTopScreenForProvenancePin.
// ────────────────────────────────────────────────────────────────────────────

// popTopScreenForProvenancePin pops the top-level ec2 list screen (back to
// the main menu) via the real ActionBack seam. This is required for the
// ByID/Child/Unknown-provenance pins below because applyResourcesLoaded
// (list_body.go) is a SECOND, independent writer into the same shared
// RowStore: whenever handleResourcesLoadedEvent finds an OPEN
// ScreenResourceList/ScreenChildList screen matching msg.ResourceType, it
// routes accepted rows through Core.ObserveRows gated by topLevelCanonical
// (isTopLevelCanonicalList — EscPops/ParentContext on THAT screen), a
// pre-existing gate this fix does not touch and msg.Provenance does not
// reach. Leaving the canonical ec2 list open while delivering a by-ID/child
// result would exercise THAT gate, not the one under test here
// (the Provenance gate in handleResourcesLoadedEvent). A real
// by-ID or child continuation legitimately lands while its originating
// screen is no longer on top (handle.go's own comment: "a late fetch ...
// lands on X's screen even when it is not currently on top" — or, as here,
// not present at all, e.g. the user backed out before a background
// continuation resolved) — so popping the screen first is not a shortcut
// around the real scenario, it is what makes the scenario the one this fix
// actually protects: a non-canonical result for a type that has NO open
// list screen to (mis)apply itself to except through the shared RowStore.
func popTopScreenForProvenancePin(t *testing.T, ctrl *app.Controller) {
	t.Helper()
	ctrl.Apply(app.Action{Kind: app.ActionBack})
}

func TestObserveResourcesLoadedRows_ByIDFetch_DoesNotReplaceCanonical(t *testing.T) {
	ctrl, core, profile, region := newProvenancePinController(t)
	seedCanonical200(t, ctrl, core, profile, region)
	popTopScreenForProvenancePin(t, ctrl)

	// Deliberately unstamped: popTopScreenForProvenancePin left no list screen
	// open, so this page names none and reaches none. It is still observed
	// into the row store — what the assertion below is about is the shared
	// per-type entry, never that the message wrote nothing.
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(1, "i-byid"),
		Provenance:   messages.FetchProvenanceByID,
	})

	assertStillCanonical200(t, ctrl, core, profile, region, "by-ID single-resource result")
}

func TestObserveResourcesLoadedRows_ChildFetch_DoesNotReplaceCanonical(t *testing.T) {
	ctrl, core, profile, region := newProvenancePinController(t)
	seedCanonical200(t, ctrl, core, profile, region)
	popTopScreenForProvenancePin(t, ctrl)

	// Deliberately unstamped: popTopScreenForProvenancePin left no list screen
	// open, so this page names none and reaches none. It is still observed
	// into the row store — what the assertion below is about is the shared
	// per-type entry, never that the message wrote nothing.
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(4, "i-child"),
		Provenance:   messages.FetchProvenanceChild,
	})

	assertStillCanonical200(t, ctrl, core, profile, region, "child-fetch result")
}

// ────────────────────────────────────────────────────────────────────────────
// Positive control: a genuine CanonicalList result DOES replace — the fix
// gates the write, it does not disable it.
// ────────────────────────────────────────────────────────────────────────────

func TestObserveResourcesLoadedRows_CanonicalList_DoesReplace(t *testing.T) {
	ctrl, core, profile, region := newProvenancePinController(t)
	seedCanonical200(t, ctrl, core, profile, region)

	handlePage(ctrl, messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(5, "i-refresh"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	snap := core.Session().RowStore.Snapshot(provenancePinType)
	if len(snap.Rows) != 5 || snap.TotalCount != 5 {
		t.Errorf("RowStore entry after a second CanonicalList result = %d rows, TotalCount=%d, want 5/5 — a genuine canonical result must still replace", len(snap.Rows), snap.TotalCount)
	}
	tf := provenancePinReadTypeFile(t, ctrl, profile, region, provenancePinType)
	if len(tf.Rows) != 5 || tf.Count != 5 {
		t.Errorf("disk TypeFile after a second CanonicalList result = %d rows, Count=%d, want 5/5 — a genuine canonical result must still persist", len(tf.Rows), tf.Count)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Fail-safe: the zero-value Provenance (a construction site that never
// declared it — the exact shape of a future fifth fetch kind whose author
// forgets to set Provenance) must NOT be treated as canonical.
// ────────────────────────────────────────────────────────────────────────────

func TestObserveResourcesLoadedRows_UnknownProvenance_FailSafe_DoesNotReplaceCanonical(t *testing.T) {
	ctrl, core, profile, region := newProvenancePinController(t)
	seedCanonical200(t, ctrl, core, profile, region)
	popTopScreenForProvenancePin(t, ctrl)

	// Provenance intentionally omitted — the zero value, FetchProvenanceUnknown.
	// Deliberately unstamped: popTopScreenForProvenancePin left no list screen
	// open, so this page names none and reaches none. It is still observed
	// into the row store — what the assertion below is about is the shared
	// per-type entry, never that the message wrote nothing.
	ctrl.Handle(messages.ResourcesLoaded{Provenance: messages.FetchProvenanceUnknown,
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(7, "i-unknown"),
	})

	assertStillCanonical200(t, ctrl, core, profile, region, "zero-value (undeclared) Provenance result")
}

// ────────────────────────────────────────────────────────────────────────────
// Web/headless lane: a filtered-drill push sets EscPops so
// isTopLevelCanonicalList is false, AND FilteredRowsSet (the C6 seed) fires
// for real.
// ────────────────────────────────────────────────────────────────────────────

// TestWebLane_FilteredDrill_SetsEscPops_AndFiltersRowsSetFires drives a
// FetchFilter-carrying related drill (not the RelatedIDs-based one above):
// FilteredRowsSet (handle.go) only fires when ls.FetchFilter is non-empty,
// which only the FetchFilter sub-case of applyRelatedNavResult populates.
// "ct-events" is the one demo-registered type with a FilteredPaginatedFetcher
// (resolve_related_navigate_test.go), so ResolveRelatedNavigate classifies
// this as NavigationKindFilteredList with FetchFilter preserved.
func TestWebLane_FilteredDrill_SetsEscPops_AndFilteredRowsSetFires(t *testing.T) {
	ctrl, core, _, _ := newProvenancePinController(t)
	ctrl.SetUIMode("web")

	filter := map[string]string{"Username": "alice-provenance-test"}

	ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	ctrl.EnsureDetailState(resource.Resource{ID: "ec2-provenance-src", Name: "ec2-provenance-src", Type: "ec2"}, "ec2")
	ctrl.ApplyDetailRelated([]app.DetailRelatedRow{
		{TargetType: "ct-events", DisplayName: "CloudTrail Events", State: domain.RelatedDeferred, FetchFilter: filter},
	})
	ctrl.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})

	if !ctrl.GetListEscPops() {
		t.Fatal("EscPops must be true after a FetchFilter-carrying related drill — isTopLevelCanonicalList must exclude this screen")
	}

	rows := []resource.Resource{
		{ID: "evt-provenance-1", Type: "ct-events"},
		{ID: "evt-provenance-2", Type: "ct-events"},
	}
	handlePage(ctrl, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    rows,
		Provenance:   messages.FetchProvenanceFilteredList,
	})

	entry, ok := core.FilteredRowsGet("ct-events", filter)
	if !ok {
		t.Fatal("FilteredRowsGet(\"ct-events\", filter) ok=false — FilteredRowsSet did not fire for the filtered-drill result")
	}
	if len(entry.Rows) != len(rows) {
		t.Errorf("FilteredRowsGet rows = %d, want %d — FilteredRowsSet fired with the wrong row set", len(entry.Rows), len(rows))
	}
}

// ────────────────────────────────────────────────────────────────────────────
// ProvenanceForContinuation: the shared classifier, pinned directly, PLUS
// both lanes that call it for a real KindFetchMore/LoadMore continuation —
// the whole point of sharing it is that the two lanes cannot drift apart,
// so a pin that only exercises one lane leaves the actual failure mode
// (the two lanes computing different provenance for the same continuation)
// uncovered.
// ────────────────────────────────────────────────────────────────────────────

func TestProvenanceForContinuation_ClassifiesChildFilteredCanonical(t *testing.T) {
	cases := []struct {
		name          string
		parentContext map[string]string
		fetchFilter   map[string]string
		want          messages.FetchProvenance
	}{
		{"child parent context wins", map[string]string{"bucket": "b"}, nil, messages.FetchProvenanceChild},
		{"filter with no parent context", nil, map[string]string{"vpc-id": "vpc-1"}, messages.FetchProvenanceFilteredList},
		{"child wins over filter when both set", map[string]string{"bucket": "b"}, map[string]string{"vpc-id": "vpc-1"}, messages.FetchProvenanceChild},
		{"neither set is the canonical continuation", nil, nil, messages.FetchProvenanceCanonicalList},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := messages.ProvenanceForContinuation(tc.parentContext, tc.fetchFilter)
			if got != tc.want {
				t.Errorf("ProvenanceForContinuation(%v, %v) = %v, want %v", tc.parentContext, tc.fetchFilter, got, tc.want)
			}
		})
	}
}

// TestExecuteTask_FetchMore_ChildContinuation_SetsChildProvenance drives the
// EXECUTOR lane's KindFetchMore case (core/runtime/executor.go) for a
// continuation carrying ParentContext, and asserts the resulting
// ResourcesLoaded.Provenance is FetchProvenanceChild.
func TestExecuteTask_FetchMore_ChildContinuation_SetsChildProvenance(t *testing.T) {
	c := newExecutorCore(t)
	const childType = "test_provenance_child"
	resource.SetChildTypeForTest(resource.ResourceTypeDef{
		Name:      "Test Provenance Child",
		ShortName: childType,
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
	})
	resource.SetPaginatedChildForTest(childType, func(_ context.Context, _ any, _ resource.ParentContext, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{}, nil
	})
	t.Cleanup(func() {
		resource.CleanupChildTypeForTest(childType)
		resource.CleanupPaginatedChildForTest(childType)
	})

	p := runtime.FetchMorePayload{ParentContext: map[string]string{"bucket": "my-bucket"}}
	ev, err := c.ExecuteTask(context.Background(), reqP(runtime.KindFetchMore, childType, p))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(messages.ResourcesLoaded)
	if !ok {
		t.Fatalf("expected messages.ResourcesLoaded, got %T", ev)
	}
	if got.Provenance != messages.FetchProvenanceChild {
		t.Errorf("executor lane KindFetchMore with ParentContext: Provenance = %v, want FetchProvenanceChild", got.Provenance)
	}
}

// TestExecuteTask_FetchMore_FilteredContinuation_SetsFilteredProvenance
// drives the EXECUTOR lane's KindFetchMore case for a continuation carrying
// FetchFilter, and asserts Provenance is FetchProvenanceFilteredList.
func TestExecuteTask_FetchMore_FilteredContinuation_SetsFilteredProvenance(t *testing.T) {
	c := newExecutorCore(t)
	const filteredType = "test_provenance_filtered"
	resource.SetChildTypeForTest(resource.ResourceTypeDef{
		Name:      "Test Provenance Filtered",
		ShortName: filteredType,
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
	})
	resource.SetFilteredPaginatedForTest(filteredType, func(_ context.Context, _ any, _ map[string]string, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{}, nil
	})
	t.Cleanup(func() {
		resource.CleanupChildTypeForTest(filteredType)
		resource.CleanupFilteredPaginatedForTest(filteredType)
	})

	p := runtime.FetchMorePayload{FetchFilter: map[string]string{"vpc-id": "vpc-1"}}
	ev, err := c.ExecuteTask(context.Background(), reqP(runtime.KindFetchMore, filteredType, p))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(messages.ResourcesLoaded)
	if !ok {
		t.Fatalf("expected messages.ResourcesLoaded, got %T", ev)
	}
	if got.Provenance != messages.FetchProvenanceFilteredList {
		t.Errorf("executor lane KindFetchMore with FetchFilter: Provenance = %v, want FetchProvenanceFilteredList", got.Provenance)
	}
}

// TestFetchMoreResources_TUI_ChildContinuation_SetsChildProvenance drives
// the TUI ADAPTER lane's fetchMoreResources (internal/tui/fetch_adapter.go)
// via the real messages.LoadMore dispatch (case messages.LoadMore ->
// m.fetchMoreResources), for a continuation carrying ParentContext.
func TestFetchMoreResources_TUI_ChildContinuation_SetsChildProvenance(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()
	m.Core().SetNoCache(true)
	m.Core().SetPreSuppliedClients(demo.NewServiceClients())
	m.Core().HandleClientsReady(runtime.ClientsReadyEvent{ //nolint:errcheck // side-effect only
		Clients:    nil,
		Gen:        m.Core().ConnectGen(),
		StackDepth: 1,
	})

	const childType = "test_provenance_tui_child"
	resource.SetChildTypeForTest(resource.ResourceTypeDef{
		Name:      "Test Provenance TUI Child",
		ShortName: childType,
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
	})
	resource.SetPaginatedChildForTest(childType, func(_ context.Context, _ any, _ resource.ParentContext, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{}, nil
	})
	t.Cleanup(func() {
		resource.CleanupChildTypeForTest(childType)
		resource.CleanupPaginatedChildForTest(childType)
	})

	_, cmd := rootApplyMsg(m, messages.LoadMore{
		ResourceType:      childType,
		ContinuationToken: "next-page-token",
		ParentContext:     map[string]string{"bucket": "my-bucket"},
	})
	if cmd == nil {
		t.Fatal("LoadMore with ParentContext should return a cmd")
	}
	msg := cmd()
	got, ok := msg.(messages.ResourcesLoaded)
	if !ok {
		t.Fatalf("expected messages.ResourcesLoaded, got %T (%+v)", msg, msg)
	}
	if got.Provenance != messages.FetchProvenanceChild {
		t.Errorf("TUI lane fetchMoreResources with ParentContext: Provenance = %v, want FetchProvenanceChild", got.Provenance)
	}
}

// TestFetchMoreResources_TUI_FilteredContinuation_SetsFilteredProvenance
// drives the TUI ADAPTER lane's fetchMoreResources for a continuation
// carrying FetchFilter, and asserts Provenance is FetchProvenanceFilteredList.
func TestFetchMoreResources_TUI_FilteredContinuation_SetsFilteredProvenance(t *testing.T) {
	withTuiVersion(t, "test")
	m := newRootSizedModel()
	m.Core().SetNoCache(true)
	m.Core().SetPreSuppliedClients(demo.NewServiceClients())
	m.Core().HandleClientsReady(runtime.ClientsReadyEvent{ //nolint:errcheck // side-effect only
		Clients:    nil,
		Gen:        m.Core().ConnectGen(),
		StackDepth: 1,
	})

	const filteredType = "test_provenance_tui_filtered"
	resource.SetChildTypeForTest(resource.ResourceTypeDef{
		Name:      "Test Provenance TUI Filtered",
		ShortName: filteredType,
		Columns:   []resource.Column{{Key: "id", Title: "ID", Width: 20}},
	})
	resource.SetFilteredPaginatedForTest(filteredType, func(_ context.Context, _ any, _ map[string]string, _ string) (resource.FetchResult, error) {
		return resource.FetchResult{}, nil
	})
	t.Cleanup(func() {
		resource.CleanupChildTypeForTest(filteredType)
		resource.CleanupFilteredPaginatedForTest(filteredType)
	})

	_, cmd := rootApplyMsg(m, messages.LoadMore{
		ResourceType:      filteredType,
		ContinuationToken: "next-page-token",
		FetchFilter:       map[string]string{"vpc-id": "vpc-1"},
	})
	if cmd == nil {
		t.Fatal("LoadMore with FetchFilter should return a cmd")
	}
	msg := cmd()
	got, ok := msg.(messages.ResourcesLoaded)
	if !ok {
		t.Fatalf("expected messages.ResourcesLoaded, got %T (%+v)", msg, msg)
	}
	if got.Provenance != messages.FetchProvenanceFilteredList {
		t.Errorf("TUI lane fetchMoreResources with FetchFilter: Provenance = %v, want FetchProvenanceFilteredList", got.Provenance)
	}
}
