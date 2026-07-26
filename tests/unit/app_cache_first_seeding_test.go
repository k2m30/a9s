// app_cache_first_seeding_test.go — RED tests for the CACHE-FIRST LIST UX epic,
// the cache-first seeding contract (in-session seeding) and the
// menu-refreshing contract (menu Refreshing signal).
//
// Cache-first seeding: when a list screen opens and the session already holds rows for
// that type (RowStore — first-page rows retained by availability probes, task
// #17 wave 1 stage 2's replacement for the removed session.ProbeResources/
// ProbeTruncated maps), the list must render those rows IMMEDIATELY:
// ListBody.Loading=false, rows visible, and ListBody.Refreshing=true while the
// fresh fetch runs. On ResourcesLoaded the rows swap in place and
// Refreshing=false. When no rows are known, today's behavior (Loading=true, no
// Refreshing) stays unchanged.
//
// Menu-refreshing: MenuBody gains Refreshing=true while a background availability
// sweep is running after a cache-seeded startup. It flips false when the sweep
// completes.
//
// AMBIGUITY RESOLUTIONS (stated, not deferred):
//   - Seeding source: driven via core.Session().RowStore.Observe(type, rows,
//     ..., session.OriginProbe, false) directly. Session() and RowStore are
//     already public/exported today (core/runtime.Core.Session,
//     core/session.Session.RowStore) — only the NEW ListBody.Refreshing
//     field and the seed-on-open behavior are red.
//   - List-open trigger: app.Action{Kind: app.ActionCommand, Arg: shortName}
//     is the existing, precedented list-open path (see
//     TestController_Apply_PRB_Command_ResourceShortName_PushesListScreen in
//     app_controller_pr_b_test.go) — driving applyNavResult's
//     NavigateKindPushResourceList branch.
//   - "Fresh fetch running / completes" is modeled as the existing
//     ResourcesLoaded task-result lane (Handle), which already swaps
//     ls.Rows and clears ls.Loading/LoadingMore — Refreshing must join that
//     same clear-on-load contract.
//   - Two resource types pinned per the generic-ness requirement: "ec2"
//     (Path-based columns) and "s3" (Key-based columns) — both go through the
//     identical seeding path with no per-type special-casing.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// newSeededTestController builds a Controller + its backing Core, exactly
// like newTestControllerWithCore in app_controller_pr_b_test.go (same
// package, precedented helper) — duplicated here as a small variant so this
// file has no cross-file coupling to another test file's helper lifetime.
//
// A9S_CONFIG_FOLDER is redirected to t.TempDir() so the disk-store fallback
// HandleNavigate now consults reads/writes an isolated per-test
// directory instead of the developer's real ~/.a9s/cache — a leftover
// ec2.yaml from a prior manual run (or another test package sharing the same
// "demo"/"us-east-1" pair) would otherwise leak rows into
// TestListOpen_NoProbeResources_KeepsTodaysLoadingBehavior's "nothing seeded"
// precondition.
func newSeededTestController(t *testing.T) (*runtime.Core, *app.Controller) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	return core, c
}

// -----------------------------------------------------------------------
// Cache-first seeding — in-session seeding from ProbeResources
// -----------------------------------------------------------------------

// TestListOpen_SeedsFromProbeResources_EC2 pins cache-first seeding for "ec2": when
// the session already holds ProbeResources for ec2, opening the ec2 list
// must render those rows immediately with Loading=false and Refreshing=true.
func TestListOpen_SeedsFromProbeResources_EC2(t *testing.T) {
	core, c := newSeededTestController(t)

	seeded := []resource.Resource{
		{ID: "i-0aaaa1111bbbb2222", Name: "web-1", Type: "ec2", Fields: map[string]string{"state": "running"}},
		{ID: "i-0ccccc3333dddd444", Name: "web-2", Type: "ec2", Fields: map[string]string{"state": "stopped"}},
	}
	core.Session().RowStore.Observe("ec2", seeded, nil, session.OriginProbe, false)

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	snap := c.Snapshot()

	if snap.Body.Kind != app.BodyKindList {
		t.Fatalf("Body.Kind = %q, want %q", snap.Body.Kind, app.BodyKindList)
	}
	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening ec2 list with seeded RowStore rows")
	}
	if lb.Loading {
		t.Error("Loading = true, want false — seeded rows must render immediately, not show a loading spinner")
	}
	if !lb.Refreshing {
		t.Error("Refreshing = false, want true — a background fetch is still expected to confirm/replace seeded rows")
	}
	if len(lb.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2 — seeded RowStore rows must be visible immediately", len(lb.Rows))
	}
	gotIDs := map[string]bool{lb.Rows[0].ResourceID: true, lb.Rows[1].ResourceID: true}
	for _, want := range []string{"i-0aaaa1111bbbb2222", "i-0ccccc3333dddd444"} {
		if !gotIDs[want] {
			t.Errorf("seeded row ID %q missing from rendered rows: %v", want, gotIDs)
		}
	}
}

// TestListOpen_SeedsFromProbeResources_S3 pins cache-first seeding for a second,
// differently-shaped resource type (s3, Key-based columns vs ec2's
// Path-based columns) to guard against a seeding path that only works for
// one column-resolution style.
func TestListOpen_SeedsFromProbeResources_S3(t *testing.T) {
	core, c := newSeededTestController(t)

	seeded := []resource.Resource{
		{ID: "my-bucket-one", Name: "my-bucket-one", Type: "s3", Fields: map[string]string{"region": "us-east-1"}},
	}
	core.Session().RowStore.Observe("s3", seeded, nil, session.OriginProbe, false)

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	snap := c.Snapshot()

	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening s3 list with seeded RowStore rows")
	}
	if lb.Loading {
		t.Error("Loading = true, want false for seeded s3 list")
	}
	if !lb.Refreshing {
		t.Error("Refreshing = false, want true for seeded s3 list")
	}
	if len(lb.Rows) != 1 || lb.Rows[0].ResourceID != "my-bucket-one" {
		t.Fatalf("Rows = %+v, want single row with ResourceID=my-bucket-one", lb.Rows)
	}
}

// TestListOpen_NoProbeResourcesNoDiskStore_KeepsTodaysLoadingBehavior
// verifies the "genuinely nothing known" branch is unchanged: Loading=true,
// Refreshing not set, no rows. This is the regression guard that stops
// cache-first seeding from firing unconditionally.
//
// Renamed from TestListOpen_NoProbeResources_KeepsTodaysLoadingBehavior
// once HandleNavigate grew a disk-store fallback for warm
// list-opens, "no ProbeResources" alone no longer implies Loading=true — a
// populated on-disk per-type cache for this pair now seeds the list too (see
// TestListOpen_NoProbeResourcesButDiskStoreHasRows_SeedsFromDiskStore below).
// This test's actual precondition is narrower than its old name claimed: NO
// ProbeResources AND NO on-disk cache for the pair (newSeededTestController's
// t.TempDir()-isolated A9S_CONFIG_FOLDER guarantees the disk side is empty
// here) — the true "nothing anywhere" cold-start case.
func TestListOpen_NoProbeResourcesNoDiskStore_KeepsTodaysLoadingBehavior(t *testing.T) {
	_, c := newSeededTestController(t)
	// No RowStore rows seeded, and newSeededTestController's isolated
	// A9S_CONFIG_FOLDER guarantees no on-disk per-type cache exists either.

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	snap := c.Snapshot()

	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening ec2 list with no seeded rows")
	}
	if !lb.Loading {
		t.Error("Loading = false, want true — with no RowStore rows and no disk-store data known, today's Loading=true behavior must be preserved")
	}
	if lb.Refreshing {
		t.Error("Refreshing = true, want false — Refreshing must not fire when there was nothing to seed from")
	}
	if len(lb.Rows) != 0 {
		t.Errorf("len(Rows) = %d, want 0 with no seeded data", len(lb.Rows))
	}
}

// TestListOpen_NoProbeResourcesButDiskStoreHasRows_SeedsFromDiskStore pins
// the post-sweep disk-store fallback contract at the Controller/list-open layer
// (a layer above the runtime.HandleNavigate-level and TUI-Update-level pins
// in tests/unit/tui_post_sweep_seed_test.go): when RowStore holds nothing for
// the type but the on-disk per-type cache for the current profile/region pair
// does, opening the list must still seed
// Loading=false/Refreshing=true/rows-populated from that disk data — exactly
// as if RowStore had held it via a probe/disk seed. This is the behavior that
// makes the renamed guard above's narrower precondition ("no RowStore rows"
// is no longer sufficient by itself) correct.
func TestListOpen_NoProbeResourcesButDiskStoreHasRows_SeedsFromDiskStore(t *testing.T) {
	core, c := newSeededTestController(t)
	// No RowStore rows seeded — this isolates the disk-store fallback from
	// the RowStore seeding source pinned in
	// TestListOpen_SeedsFromProbeResources_EC2.

	store := core.EnsureCacheStore()
	if store == nil {
		t.Fatal("core.EnsureCacheStore() = nil — test fixture requires a live disk store to seed rows into")
	}
	store.Put("ec2", cache.TypeFile{
		HasResources: true,
		Count:        1,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "i-0diskstore0001", Name: "disk-seeded-1", Fields: map[string]string{"state": "running"}},
		},
	})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("seed fixture SaveType(ec2): %v", err)
	}

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	snap := c.Snapshot()

	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening ec2 list with a disk-store-only seed")
	}
	if lb.Loading {
		t.Error("Loading = true, want false — a populated on-disk per-type cache must seed the list immediately even with no RowStore rows known (the disk-store fallback)")
	}
	if !lb.Refreshing {
		t.Error("Refreshing = false, want true — a fresh fetch must still run to confirm/replace the disk-seeded rows")
	}
	if len(lb.Rows) != 1 || lb.Rows[0].ResourceID != "i-0diskstore0001" {
		t.Fatalf("Rows = %+v, want single row with ResourceID=i-0diskstore0001", lb.Rows)
	}
}

// TestListOpen_SeedsFromResourceCache_PreviousVisit pins the second half of
// the cache-first seeding contract's second seeding source: "or a previous visit's ResourceCache" — not
// just the availability-probe ProbeResources map. Simulates a user who
// already opened the ec2 list once this session (populating session
// ResourceCache), popped back to the menu, then re-opens the list — the
// second open must seed instantly.
func TestListOpen_SeedsFromResourceCache_PreviousVisit(t *testing.T) {
	core, c := newSeededTestController(t)

	priorVisitRows := []resource.Resource{
		{ID: "i-0prevvisit0001", Name: "cached-1", Type: "ec2", Fields: map[string]string{"state": "running"}},
	}
	core.Session().RowStore.Observe("ec2", priorVisitRows, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	snap := c.Snapshot()

	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening ec2 list with a prior-visit ResourceCache entry")
	}
	if lb.Loading {
		t.Error("Loading = true, want false — a previous visit's ResourceCache must seed the list immediately")
	}
	if !lb.Refreshing {
		t.Error("Refreshing = false, want true — a fresh fetch must still run to confirm/replace cached rows")
	}
	if len(lb.Rows) != 1 || lb.Rows[0].ResourceID != "i-0prevvisit0001" {
		t.Fatalf("Rows = %+v, want single row with ResourceID=i-0prevvisit0001", lb.Rows)
	}
}

// TestListOpen_ResourcesLoaded_ClearsRefreshingAndSwapsRows pins the
// "on ResourcesLoaded the rows swap in place and Refreshing=false" half of
// the cache-first seeding contract.
func TestListOpen_ResourcesLoaded_ClearsRefreshingAndSwapsRows(t *testing.T) {
	core, c := newSeededTestController(t)

	seeded := []resource.Resource{
		{ID: "i-0seeded0001", Name: "stale-seed", Type: "ec2", Fields: map[string]string{"state": "pending"}},
	}
	core.Session().RowStore.Observe("ec2", seeded, nil, session.OriginProbe, false)

	_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	if lb := c.Snapshot().Body.List; lb == nil || !lb.Refreshing {
		t.Fatal("precondition failed: expected Refreshing=true immediately after seeded list-open")
	}

	fresh := []resource.Resource{
		{ID: "i-0fresh00001", Name: "fresh-row", Type: "ec2", Fields: map[string]string{"state": "running"}},
		{ID: "i-0fresh00002", Name: "fresh-row-2", Type: "ec2", Fields: map[string]string{"state": "running"}},
	}
	vs, _ := c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    fresh,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Gen:          0, Provenance: // AcceptZeroGen=true — always passes the staleness guard
		messages.FetchProvenanceCanonicalList,
	})

	lb := vs.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after ResourcesLoaded")
	}
	if lb.Refreshing {
		t.Error("Refreshing = true, want false — ResourcesLoaded must clear Refreshing once the fresh fetch lands")
	}
	if lb.Loading {
		t.Error("Loading = true, want false after ResourcesLoaded")
	}
	if len(lb.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2 — rows must swap to the fresh fetch result, not stay on the seeded page", len(lb.Rows))
	}
	for _, r := range lb.Rows {
		if r.ResourceID == "i-0seeded0001" {
			t.Errorf("stale seeded row %q still present after ResourcesLoaded swap", r.ResourceID)
		}
	}
}

// -----------------------------------------------------------------------
// Menu-refreshing signal during background availability sweep
// -----------------------------------------------------------------------

// TestMenu_Refreshing_TrueDuringBackgroundSweep_FalseOnComplete pins the
// menu-refreshing signal: MenuBody.Refreshing=true while a background availability sweep runs
// after a cache-seeded startup, flipping false when the sweep completes.
//
// Ambiguity resolution: "sweep running" is modeled the same way the rest of
// the availability pipeline models "in flight" state — via a per-type
// AvailChecked/AvailTotal progress counter already present on MenuState
// (see menuProgressIndicator in menu.go). This test pins the OUTCOME
// (MenuBody.Refreshing) rather than assuming a specific internal counter
// name, so the coder is free to wire it through PatchMenuAvailability/
// AvailabilityChecked intents however is cleanest.
func TestMenu_Refreshing_TrueDuringBackgroundSweep_FalseOnComplete(t *testing.T) {
	core, c := newSeededTestController(t)

	// Cold start with cache-seeded counts already applied (mirrors a disk
	// cache load having already populated menu availability before any live
	// probe has run).
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "ec2", Count: 3, Truncated: false},
	})

	// Simulate the background sweep starting: at least one AvailabilityChecked
	// has NOT yet arrived for all known types. We drive this through the same
	// event lane the runtime uses so this test exercises the real seam, not a
	// hand-built MenuState.
	core.Session().RowStore.Observe("ec2", []resource.Resource{
		{ID: "i-0sweep0001", Type: "ec2"},
	}, nil, session.OriginProbe, false)

	snapDuring := c.Snapshot()
	if snapDuring.Body.Menu == nil {
		t.Fatal("Snapshot().Body.Menu is nil at root menu screen")
	}
	if !snapDuring.Body.Menu.Refreshing {
		t.Error("MenuBody.Refreshing = false, want true — a background availability sweep is in flight after cache-seeded startup")
	}

	// Sweep completes: every registered type has been probed.
	vs, _ := c.Handle(messages.AvailabilityChecked{
		ResourceType: "ec2",
		HasResources: true,
		Count:        3,
		Truncated:    false,
		Gen:          0,
	})
	if vs.Body.Menu == nil {
		t.Fatal("Handle(AvailabilityChecked) returned nil Body.Menu")
	}
	// NOTE: full-sweep completion in production spans every registered
	// resource type; this single-type Handle call pins the transition
	// direction (false once this type's probe result lands and no other
	// probe is outstanding), matching the "no other RowStore sweep signal
	// remains" outcome for a controller seeded with only one type.
	if vs.Body.Menu.Refreshing {
		t.Error("MenuBody.Refreshing = true, want false — sweep must clear Refreshing once its results land")
	}
}

// TestMenu_Refreshing_FalseWithNoSweepInFlight guards against Refreshing
// firing unconditionally on every menu snapshot (the "no probe ever
// started" baseline).
func TestMenu_Refreshing_FalseWithNoSweepInFlight(t *testing.T) {
	_, c := newSeededTestController(t)

	snap := c.Snapshot()
	if snap.Body.Menu == nil {
		t.Fatal("Snapshot().Body.Menu is nil")
	}
	if snap.Body.Menu.Refreshing {
		t.Error("MenuBody.Refreshing = true, want false — no availability sweep has ever started on a fresh controller")
	}
}

// TestListOpen_Refreshing_AllResourceTypes_Generic verifies the seeding
// contract has no per-type special-casing by driving it against every
// registered resource type with RowStore rows seeded. This is the
// "generic-ness pin" required by the epic across ALL 66 types, not just
// the two spot-checked above.
func TestListOpen_Refreshing_AllResourceTypes_Generic(t *testing.T) {
	for _, td := range resource.AllResourceTypes() {
		shortName := td.ShortName
		t.Run(shortName, func(t *testing.T) {
			core, c := newSeededTestController(t)
			core.Session().RowStore.Observe(shortName, []resource.Resource{
				{ID: "generic-seed-1", Name: "generic-seed-1", Type: shortName, Fields: map[string]string{}},
			}, nil, session.OriginProbe, false)

			_, _ = c.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})
			snap := c.Snapshot()

			if snap.Body.Kind != app.BodyKindList {
				t.Fatalf("Body.Kind = %q, want %q for type %q", snap.Body.Kind, app.BodyKindList, shortName)
			}
			lb := snap.Body.List
			if lb == nil {
				t.Fatalf("Body.List is nil for type %q", shortName)
			}
			if lb.Loading {
				t.Errorf("type %q: Loading = true, want false with seeded RowStore rows", shortName)
			}
			if !lb.Refreshing {
				t.Errorf("type %q: Refreshing = false, want true with seeded RowStore rows", shortName)
			}
			if len(lb.Rows) != 1 {
				t.Errorf("type %q: len(Rows) = %d, want 1", shortName, len(lb.Rows))
			}
		})
	}
}

// domain import guard: keep the domain.Gen usage explicit for the Gen field
// on messages so a future refactor of ResourcesLoaded.Gen's type is caught
// at compile time rather than silently accepting an int.
var _ domain.Gen = domain.Gen(0)
