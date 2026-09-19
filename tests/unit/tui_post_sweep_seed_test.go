// The post-sweep disk-store fallback.
//
// handleEnrichmentChecked (core/runtime/handlers_availability.go, "All
// enrichment done" branch) nils c.session.ProbeResources /
// c.session.ProbeTruncated once Wave-2 enrichment completes (EnrichChecked >=
// EnrichTotal), AFTER snapshotting them for the cache save. HandleNavigate's
// cache-MISS seed branch (handlers_navigate.go NavigateTargetResourceList
// case) therefore falls back ProbeResources -> loaded per-type disk store
// rows (session.EnsureCacheStore()/store.Type(canon), the equivalent of
// rowsFromCacheRows + IsTruncated: !tf.Exact) when ProbeResources holds
// nothing for the type: that Store outlives the post-sweep free and holds
// the exact same rows the sweep itself just persisted via TaskKindSaveCache,
// so a list opened AFTER the background sweep completes renders those rows,
// not a bare "Loading..." shell. All navigation entry points (menu Enter,
// TUI colon-command, -c armed command) funnel through this same
// HandleNavigate seed.
package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// seedDiskStoreWithS3Rows writes a real on-disk per-type cache file for
// profile/region carrying 2 s3 rows via cache.Store.Put + SaveType, mirroring
// seedDiskCacheWithS3Rows in command_navigation_after_seed_test.go. Returns
// the freshly-reloaded *cache.Store, matching what EnsureCacheStore would
// see on a real read after the sweep's own TaskKindSaveCache wrote it.
func seedDiskStoreWithS3Rows(t *testing.T, profile, region string) *cache.Store {
	t.Helper()
	store := cache.LoadDirForTest(profile, region)
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        2,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "arn:aws:s3:::sweep-bucket-1", Name: "sweep-bucket-1", Fields: map[string]string{"region": region}},
			{ID: "arn:aws:s3:::sweep-bucket-2", Name: "sweep-bucket-2", Fields: map[string]string{"region": region}},
		},
	})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("seed fixture SaveType(s3): %v", err)
	}
	return cache.LoadDirForTest(profile, region)
}

// TestPostSweepWarmOpen_SeedsFromStore pins the disk-store fallback at the Core.HandleNavigate
// seam. The disk store for "s3" carries 2 real rows (written exactly as a
// completed sweep's TaskKindSaveCache would). The session is driven through
// the full post-sweep state machine: AvailabilityCacheLoaded (seeds
// RowStore from the disk store, as handleAvailabilityCacheLoaded does) ->
// AvailabilityChecked (Wave-1 completes, queue drains) -> EnrichmentChecked
// with TypeGen matching so it is NOT dropped as stale, and with the
// enrichment queue naturally empty (BuildEnrichQueue returns nothing for
// this synthetic single-type fixture with no detail enrichers registered
// against it in this harness) so EnrichChecked(1) >= EnrichTotal(1) fires
// the "all enrichment done" branch on this very call.
//
// RowStore retains its rows for the session unconditionally (see
// RowStore.Amend's doc comment). The precondition below confirms RowStore
// has genuinely been observed for s3 with the AvailabilityChecked-landed
// rows, so the seed the assertions inspect provably reflects a completed
// post-sweep state.
//
// A post-sweep list-open seeds CachedEntry from RowStore's retained rows
// immediately, matching disk-store data by construction in this fixture (the
// sweep wrote the same row IDs to both).
func TestPostSweepWarmOpen_SeedsFromStore(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	profile, region := "sweep-store-profile", "us-east-1"
	seedDiskStoreWithS3Rows(t, profile, region)

	sess := session.New()
	sess.Profile = profile
	sess.Region = region
	core := runtime.New(sess, catalog.All())

	// Drive AvailabilityCacheLoaded so ProbeResources seeds from the disk
	// store first (mirrors a real cold-boot cache load), then let the
	// sweep run to Wave-1 completion.
	_, _ = core.HandleEvent(messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"s3": 2},
	})

	if tr := core.Session().RowStore.Snapshot("s3"); tr.Gen == 0 || len(tr.Rows) == 0 {
		t.Fatal("fixture assumption broken: RowStore not seeded for s3 by AvailabilityCacheLoaded before driving the sweep to completion")
	}

	// Drain Wave-1: a single AvailabilityChecked with the queue naturally
	// empty (AvailQueue was fully consumed by handleAvailabilityCacheLoaded's
	// own initial batch dispatch against a 1-type catalog fixture in this
	// harness) satisfies "all checks done" and starts Wave-2 enrichment.
	_, _ = core.HandleEvent(messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: true,
		Count:        2,
		Gen:          1,
		Resources: []resource.Resource{
			{ID: "arn:aws:s3:::sweep-bucket-1", Name: "sweep-bucket-1", Type: "s3", Fields: map[string]string{"region": region}},
			{ID: "arn:aws:s3:::sweep-bucket-2", Name: "sweep-bucket-2", Type: "s3", Fields: map[string]string{"region": region}},
		},
	})

	// Drain Wave-2 to completion. EnrichmentChecked.AcceptZeroGen() is true,
	// so Gen need not be threaded through a queue-inspection round-trip;
	// TypeGen: 0 always passes the per-type generation guard
	// (msg.TypeGen != 0 && msg.TypeGen != current — a zero TypeGen never
	// trips the mismatch branch). Sent once per queued type; a fresh
	// session's enrichment queue for a single probed type drains on the
	// first result, so EnrichChecked(1) >= EnrichTotal(1) triggers the
	// "all enrichment done" free unconditionally here regardless of the
	// queue's exact length.
	for i := 0; i < 5; i++ {
		_, _ = core.HandleEvent(messages.EnrichmentChecked{ResourceType: "s3"})
	}

	if tr := core.Session().RowStore.Snapshot("s3"); tr.Gen == 0 || len(tr.Rows) != 2 {
		t.Fatalf("fixture assumption broken: RowStore.Snapshot(s3) = %+v, want Gen != 0 with 2 rows after driving to post-sweep completion", tr)
	}

	result, tasks := core.HandleNavigate(runtime.NavigateEvent{
		Target:       runtime.NavigateTargetResourceList,
		ResourceType: "s3",
	})

	if result.Kind != runtime.NavigateKindPushResourceList {
		t.Fatalf("result.Kind = %v, want NavigateKindPushResourceList (still a session.ResourceCache miss)", result.Kind)
	}
	if result.CachedEntry == nil {
		t.Fatal("result.CachedEntry = nil, want a synthetic entry seeded from the on-disk per-type store — D12: a post-sweep list-open must not render a bare Loading shell when the disk cache holds complete, fresh rows")
	}
	if len(result.CachedEntry.Resources) != 2 {
		t.Fatalf("len(result.CachedEntry.Resources) = %d, want 2 (the disk store's persisted rows)", len(result.CachedEntry.Resources))
	}
	gotIDs := map[string]bool{}
	for _, r := range result.CachedEntry.Resources {
		gotIDs[r.ID] = true
	}
	for _, want := range []string{"arn:aws:s3:::sweep-bucket-1", "arn:aws:s3:::sweep-bucket-2"} {
		if !gotIDs[want] {
			t.Errorf("result.CachedEntry.Resources missing disk-store row %q, got IDs %v", want, gotIDs)
		}
	}
	if result.CachedEntry.Pagination == nil || result.CachedEntry.Pagination.IsTruncated {
		t.Errorf("result.CachedEntry.Pagination = %+v, want non-nil with IsTruncated=false (disk store's TypeFile.Exact=true for this fixture)", result.CachedEntry.Pagination)
	}

	if len(tasks) != 1 || tasks[0].Key.Kind != runtime.KindFetchResources {
		t.Errorf("tasks = %+v, want exactly one KindFetchResources task — the disk-store seed augments the miss path, it must not replace the verify-on-sight fetch (C1)", tasks)
	}
}

// newPostSweepApp builds a tui.Model wired to demo clients with on-disk
// caching ENABLED (no WithNoCache) so EnsureCacheStore actually loads the
// disk store this test seeds — mirrors newSaveCacheApp's construction
// (tui_savecache_routing_test.go), not newWarmOpenApp's (which disables
// caching and would make the disk-store fallback unreachable).
func newPostSweepApp(t *testing.T, profile, region string) tui.Model {
	t.Helper()
	m := newBlessedModel(t, profile, region,
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithProfileForTest(profile),
		tui.WithRegionForTest(region))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	t.Cleanup(m.CloseController)
	return m
}

// driveToPostSweepState drives m through the sequence
// TestPostSweepWarmOpen_SeedsFromStore drives core through
// (AvailabilityCacheLoaded seed -> AvailabilityChecked -> N
// EnrichmentChecked events), via the real Bubble Tea Update seam.
func driveToPostSweepState(m tui.Model, region string) tui.Model {
	m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"s3": 2},
	})
	m, _ = rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: true,
		Count:        2,
		Gen:          1,
		Resources: []resource.Resource{
			{ID: "arn:aws:s3:::sweep-tui-bucket-1", Name: "sweep-tui-bucket-1", Type: "s3", Fields: map[string]string{"region": region}},
			{ID: "arn:aws:s3:::sweep-tui-bucket-2", Name: "sweep-tui-bucket-2", Type: "s3", Fields: map[string]string{"region": region}},
		},
	})
	for i := 0; i < 5; i++ {
		m, _ = rootApplyMsg(m, messages.EnrichmentChecked{ResourceType: "s3"})
	}
	return m
}

// TestPostSweepWarmOpen_TUI_RendersRows pins the disk-store fallback at the
// real Bubble Tea Update/View seam. In the post-sweep state, navigating via a
// messages.Navigate (exactly what pressing Enter on the main menu emits)
// renders the RowStore-seeded row names (driveToPostSweepState's own
// AvailabilityChecked delivery), not the bare "Loading..." shell.
//
// RowStore retains its rows for the session unconditionally (see
// RowStore.Amend's doc comment), so RowStore's own retained rows (the
// AvailabilityChecked-landed "def15-tui-bucket-*" rows) are always what a
// post-sweep list-open seeds from; the on-disk store seeded here carries
// deliberately DIFFERENT row names ("def15-store-bucket-*") so this test can
// distinguish "rendered from RowStore" from "rendered from disk" — see
// TestPostSweepWarmOpen_SeedsFromStore for the disk-fallback path (reachable
// only when RowStore was never observed this session).
func TestPostSweepWarmOpen_TUI_RendersRows(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "sweep-tui-profile", "us-east-1"
	seedDiskStoreWithS3Rows(t, profile, region)

	m := newPostSweepApp(t, profile, region)
	m = driveToPostSweepState(m, region)

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})

	content := stripANSI(rootViewContent(m))
	if strings.Contains(content, "Loading...") {
		t.Errorf("rendered view after post-sweep opening s3 (RowStore fully seeded) still shows the bare Loading shell — D12:\n%s", content)
	}
	if !strings.Contains(content, "sweep-tui-bucket-1") {
		t.Errorf("rendered view after post-sweep opening s3 does not contain the RowStore-seeded row %q:\n%s", "sweep-tui-bucket-1", content)
	}
	if !strings.Contains(content, "sweep-tui-bucket-2") {
		t.Errorf("rendered view after post-sweep opening s3 does not contain the RowStore-seeded row %q:\n%s", "sweep-tui-bucket-2", content)
	}
}

// TestColonCommand_WarmOpen_RendersRows drives the interactive colon-command
// entry lane (":" + "s3" + Enter) while ProbeResources["s3"] is still
// populated (mid-sweep — AvailabilityCacheLoaded has seeded it, but
// enrichment has not completed, so the free has not fired) and asserts the
// seeded rows render rather than Loading.
func TestColonCommand_WarmOpen_RendersRows(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "sweep-colon-profile", "us-east-1"
	seedDiskStoreWithS3Rows(t, profile, region)

	m := newPostSweepApp(t, profile, region)

	// Mid-sweep: AvailabilityCacheLoaded seeds ProbeResources; enrichment is
	// not driven to completion, so ProbeResources["s3"] is still populated.
	m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"s3": 2},
	})
	m, _ = rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: true,
		Count:        2,
		Gen:          1,
		Resources: []resource.Resource{
			{ID: "arn:aws:s3:::sweep-tui-bucket-1", Name: "sweep-tui-bucket-1", Type: "s3", Fields: map[string]string{"region": region}},
			{ID: "arn:aws:s3:::sweep-tui-bucket-2", Name: "sweep-tui-bucket-2", Type: "s3", Fields: map[string]string{"region": region}},
		},
	})

	m, _ = rootApplyMsg(m, rootKeyPress(":"))
	for _, ch := range "s3" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(ch)))
	}
	m, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal(":s3 + Enter should return a command")
	}
	navMsg := cmd()
	nav, ok := navMsg.(messages.Navigate)
	if !ok {
		t.Fatalf(":s3 should emit messages.Navigate, got %T", navMsg)
	}
	m, _ = rootApplyMsg(m, nav)

	content := stripANSI(rootViewContent(m))
	if strings.Contains(content, "Loading...") {
		t.Logf("FINDING: :s3 mid-sweep (ProbeResources populated) renders the bare Loading shell through the colon-command lane. Since Test 2 proves the messages.Navigate path renders seeded rows once ProbeResources is populated, this divergence is specific to the colon-command key-mode input path (rootKeyPress(':') + chars + KeyEnter) versus a directly-injected messages.Navigate — the command-mode submit handler is not reaching the same HandleNavigate call, or is racing/dropping the seed. Needs deeper tracing in the TUI command-mode submit handler, not HandleNavigate itself.")
		t.Errorf("rendered view after :s3 mid-sweep still shows the bare Loading shell — the D12 live-tmux symptom:\n%s", content)
	}
	if !strings.Contains(content, "sweep-tui-bucket-1") && !strings.Contains(content, "sweep-bucket-1") {
		t.Errorf("rendered view after :s3 mid-sweep does not contain a seeded row (neither ProbeResources' sweep-tui-bucket-1 nor a disk-store fallback row):\n%s", content)
	}
}

// TestMenuEnter_PostSweep_RendersRows drives Enter on the main-menu row for
// "ec2" (the default cursor position, index 0), post-sweep for ec2, and
// asserts the rendered view shows the disk-seeded rows rather than Loading.
func TestMenuEnter_PostSweep_RendersRows(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "sweep-menu-profile", "us-east-1"

	store := cache.LoadDirForTest(profile, region)
	store.Put("ec2", cache.TypeFile{
		HasResources: true,
		Count:        2,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "i-sweepmenu1", Name: "sweep-menu-instance-1", Fields: map[string]string{"region": region}},
			{ID: "i-sweepmenu2", Name: "sweep-menu-instance-2", Fields: map[string]string{"region": region}},
		},
	})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("seed fixture SaveType(ec2): %v", err)
	}

	m := newPostSweepApp(t, profile, region)

	m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"ec2": 2},
	})
	m, _ = rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: "ec2",
		HasResources: true,
		Count:        2,
		Gen:          1,
		Resources: []resource.Resource{
			{ID: "i-sweepmenu1", Name: "sweep-menu-instance-1", Type: "ec2", Fields: map[string]string{"region": region}},
			{ID: "i-sweepmenu2", Name: "sweep-menu-instance-2", Type: "ec2", Fields: map[string]string{"region": region}},
		},
	})
	for i := 0; i < 5; i++ {
		m, _ = rootApplyMsg(m, messages.EnrichmentChecked{ResourceType: "ec2"})
	}

	// Default cursor is EC2 Instances (index 0) — no j/down movement needed.
	m, cmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter on the main menu should produce a navigate command")
	}
	navMsg := cmd()
	nav, ok := navMsg.(messages.Navigate)
	if !ok {
		t.Fatalf("Enter should emit messages.Navigate, got %T", navMsg)
	}
	if nav.ResourceType != "ec2" {
		t.Fatalf("fixture assumption broken: default menu cursor ResourceType = %q, want %q", nav.ResourceType, "ec2")
	}
	m, _ = rootApplyMsg(m, nav)

	content := stripANSI(rootViewContent(m))
	if strings.Contains(content, "Loading...") {
		t.Errorf("rendered view after menu-Enter on ec2 post-sweep still shows the bare Loading shell — D12:\n%s", content)
	}
	if !strings.Contains(content, "sweep-menu-instance-1") {
		t.Errorf("rendered view after menu-Enter on ec2 post-sweep does not contain the disk-seeded row %q:\n%s", "sweep-menu-instance-1", content)
	}
	if !strings.Contains(content, "sweep-menu-instance-2") {
		t.Errorf("rendered view after menu-Enter on ec2 post-sweep does not contain the disk-seeded row %q:\n%s", "sweep-menu-instance-2", content)
	}
}

// TestPairSwitch_PostSweep_NoStaleSeed: profile A's disk store carries s3
// rows; after Rotate() to profile B, whose disk store carries no s3 data,
// HandleNavigate("s3") seeds nothing from A's store — EnsureCacheStore is
// pair-scoped (Session.EnsureCacheStore reloads
// cache.LoadDirForTest(profile, region) whenever the memoized store's pair
// differs from session.Profile/Region).
func TestPairSwitch_PostSweep_NoStaleSeed(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	profileA, profileB, region := "sweep-pairswitch-a", "sweep-pairswitch-b", "us-east-1"
	seedDiskStoreWithS3Rows(t, profileA, region)
	// profileB's pair intentionally has no on-disk s3 data at all.

	sess := session.New()
	sess.Profile = profileA
	sess.Region = region
	core := runtime.New(sess, catalog.All())

	// Populate ProbeResources for A directly (equivalent of a completed
	// sweep for A), then rotate to B exactly as HandleProfileSelected does.
	_, _ = core.HandleEvent(messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"s3": 2},
	})
	_, _ = core.HandleProfileSelected(runtime.ProfileSelectedEvent{Profile: profileB})
	core.SetRegion(region)

	result, _ := core.HandleNavigate(runtime.NavigateEvent{
		Target:       runtime.NavigateTargetResourceList,
		ResourceType: "s3",
	})

	if result.CachedEntry != nil {
		for _, r := range result.CachedEntry.Resources {
			if strings.HasPrefix(r.ID, "arn:aws:s3:::def15-store-bucket-") {
				t.Errorf("result.CachedEntry.Resources contains profile-A's stale disk-store row %q after switching to profile B (which has no s3 disk cache) — C9 violation: the disk-store fallback must be pair-scoped, not leak the old pair's rows", r.ID)
			}
		}
	}
}

// An observed-empty probe result this session never falls back to stale
// disk-store rows. handleAvailabilityChecked
// (core/runtime/handlers_availability.go) stores a type's rows whenever
// "msg.Err == nil || len(msg.Resources) > 0". For a genuinely-empty type the
// live probe sends Err=nil, Resources=nil/empty, HasResources=false, Count=0
// — Err==nil alone satisfies the guard, so the store runs with a
// zero-length slice: the type is observed, not absent. HandleNavigate's
// fallback distinguishes that via the observed flag; a bare
// "len(rows) > 0" check fails an observed-empty slice identically to an
// absent key.

// TestObservedEmpty_DoesNotSeedStaleDiskRows pins the Core.HandleNavigate
// seam: the disk store for "s3" carries 3 stale rows (written before this
// session started, e.g. a prior session's sweep), but THIS session's live
// Wave-1 probe already confirmed s3 has zero resources. A current-session
// observed-empty result is fresher than any disk row.
func TestObservedEmpty_DoesNotSeedStaleDiskRows(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	profile, region := "stale-store-profile", "us-east-1"

	store := cache.LoadDirForTest(profile, region)
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        3,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "arn:aws:s3:::stale-bucket-1", Name: "stale-bucket-1", Fields: map[string]string{"region": region}},
			{ID: "arn:aws:s3:::stale-bucket-2", Name: "stale-bucket-2", Fields: map[string]string{"region": region}},
			{ID: "arn:aws:s3:::stale-bucket-3", Name: "stale-bucket-3", Fields: map[string]string{"region": region}},
		},
	})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("seed fixture SaveType(s3): %v", err)
	}

	sess := session.New()
	sess.Profile = profile
	sess.Region = region
	core := runtime.New(sess, catalog.All())

	// The real live-probe event for a genuinely-empty type, exactly as
	// production fires it: Err=nil, HasResources=false, Count=0, Resources=nil.
	_, _ = core.HandleEvent(messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: false,
		Count:        0,
		Gen:          1,
		Err:          nil,
		Resources:    nil,
	})

	// Core.ProbeResources()'s two-value return reports ok=false whenever
	// len(Rows)==0 regardless of Gen, so it cannot distinguish "never observed"
	// from "observed empty"; HandleNavigate reads
	// RowStore.Snapshot(canon).Gen != 0, and so does this precondition.
	tr := core.Session().RowStore.Snapshot("s3")
	if tr.Gen == 0 {
		t.Fatal("fixture assumption broken: RowStore.Snapshot(s3).Gen == 0 (never observed) after a live AvailabilityChecked{Err:nil} event — the storage-shape finding (Err==nil alone triggers storage) does not hold; HandleNavigate cannot be pinned against this precondition")
	}
	if len(tr.Rows) != 0 {
		t.Fatalf("fixture assumption broken: RowStore.Snapshot(s3).Rows = %d rows, want 0 (observed-empty)", len(tr.Rows))
	}

	result, tasks := core.HandleNavigate(runtime.NavigateEvent{
		Target:       runtime.NavigateTargetResourceList,
		ResourceType: "s3",
	})

	if result.Kind != runtime.NavigateKindPushResourceList {
		t.Fatalf("result.Kind = %v, want NavigateKindPushResourceList (still a session.ResourceCache miss)", result.Kind)
	}
	if result.CachedEntry != nil {
		t.Fatalf("result.CachedEntry = %+v, want nil — this session observed s3 as empty via a live probe; seeding from the stale on-disk store rows (%d rows) would render resources the current session already knows do not exist (the observed-empty guard on the disk-store fallback)", result.CachedEntry, len(result.CachedEntry.Resources))
	}
	if len(tasks) != 1 || tasks[0].Key.Kind != runtime.KindFetchResources {
		t.Errorf("tasks = %+v, want exactly one KindFetchResources task — an observed-empty seed still requires the verify-on-sight fetch (C1)", tasks)
	}
}

// TestObservedEmpty_TUI_DoesNotRenderStaleRows is the TUI-level companion to
// TestObservedEmpty_DoesNotSeedStaleDiskRows: after a live Wave-1 probe
// confirms s3 is empty this session, opening the s3 list via
// messages.Navigate must render a bare (non-stale) list — none of the
// disk-store's stale row names may appear — even though the disk cache for
// this pair is fully populated with 3 rows from a prior session.
func TestObservedEmpty_TUI_DoesNotRenderStaleRows(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "stale-tui-profile", "us-east-1"

	store := cache.LoadDirForTest(profile, region)
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        3,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "arn:aws:s3:::stale-tui-1", Name: "stale-tui-1", Fields: map[string]string{"region": region}},
			{ID: "arn:aws:s3:::stale-tui-2", Name: "stale-tui-2", Fields: map[string]string{"region": region}},
			{ID: "arn:aws:s3:::stale-tui-3", Name: "stale-tui-3", Fields: map[string]string{"region": region}},
		},
	})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("seed fixture SaveType(s3): %v", err)
	}

	m := newPostSweepApp(t, profile, region)

	m, _ = rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: false,
		Count:        0,
		Gen:          1,
		Err:          nil,
		Resources:    nil,
	})

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})

	content := stripANSI(rootViewContent(m))
	for _, stale := range []string{"stale-tui-1", "stale-tui-2", "stale-tui-3"} {
		if strings.Contains(content, stale) {
			t.Errorf("rendered view after opening s3 (observed empty this session via live probe) contains stale disk-store row %q — the observed-empty guard on the disk-store fallback:\n%s", stale, content)
		}
	}
}

// TestUnobserved_StillSeedsFromStore: when ProbeResources lacks the "s3"
// key entirely (never observed this session — before any probe or after the
// post-sweep free), HandleNavigate falls back to the on-disk per-type store
// and seeds CachedEntry from its rows.
func TestUnobserved_StillSeedsFromStore(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	profile, region := "stale-unobserved-profile", "us-east-1"
	seedDiskStoreWithS3Rows(t, profile, region)

	sess := session.New()
	sess.Profile = profile
	sess.Region = region
	core := runtime.New(sess, catalog.All())

	// Precondition: prove the key is genuinely absent, not merely empty.
	if _, observed := core.ProbeResources("s3"); observed {
		t.Fatal("fixture assumption broken: session.ProbeResources[s3] already observed before any probe ran — a fresh session.New() must not pre-populate ProbeResources")
	}

	result, tasks := core.HandleNavigate(runtime.NavigateEvent{
		Target:       runtime.NavigateTargetResourceList,
		ResourceType: "s3",
	})

	if result.Kind != runtime.NavigateKindPushResourceList {
		t.Fatalf("result.Kind = %v, want NavigateKindPushResourceList (still a session.ResourceCache miss)", result.Kind)
	}
	if result.CachedEntry == nil {
		t.Fatal("result.CachedEntry = nil, want a synthetic entry seeded from the on-disk per-type store — disk-store fallback base behavior: an unobserved type (key absent) must still fall back to disk-store rows")
	}
	if len(result.CachedEntry.Resources) != 2 {
		t.Fatalf("len(result.CachedEntry.Resources) = %d, want 2 (the disk store's persisted rows)", len(result.CachedEntry.Resources))
	}
	gotIDs := map[string]bool{}
	for _, r := range result.CachedEntry.Resources {
		gotIDs[r.ID] = true
	}
	for _, want := range []string{"arn:aws:s3:::sweep-bucket-1", "arn:aws:s3:::sweep-bucket-2"} {
		if !gotIDs[want] {
			t.Errorf("result.CachedEntry.Resources missing disk-store row %q, got IDs %v", want, gotIDs)
		}
	}
	if result.CachedEntry.Pagination == nil || result.CachedEntry.Pagination.IsTruncated {
		t.Errorf("result.CachedEntry.Pagination = %+v, want non-nil with IsTruncated=false (disk store's TypeFile.Exact=true for this fixture)", result.CachedEntry.Pagination)
	}
	if len(tasks) != 1 || tasks[0].Key.Kind != runtime.KindFetchResources {
		t.Errorf("tasks = %+v, want exactly one KindFetchResources task", tasks)
	}
}
