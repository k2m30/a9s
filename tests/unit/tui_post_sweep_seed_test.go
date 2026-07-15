// tui_post_sweep_seed_test.go — RED regression tests for DEF-15 (C1
// systemic follow-on to DEF-12).
//
// Root cause: handleEnrichmentChecked (internal/runtime/handlers_availability.go,
// "All enrichment done" branch, ~L494-507) nils c.session.ProbeResources /
// c.session.ProbeTruncated once Wave-2 enrichment completes (EnrichChecked >=
// EnrichTotal), AFTER snapshotting them for the cache save. HandleNavigate's
// cache-MISS seed branch (handlers_navigate.go NavigateTargetResourceList
// case, ~L160-167) reads ONLY session.ProbeResources[canon] to populate
// NavigateResult.CachedEntry — it never falls back to the on-disk per-type
// Store (session.EnsureCacheStore()/store.Type(canon)), even though that
// Store outlives the post-sweep free and holds the exact same rows the sweep
// itself just persisted via TaskKindSaveCache.
//
// Symptom: any list opened AFTER the background availability+enrichment
// sweep has fully completed renders a bare "Loading..." shell, even though
// the disk cache for that type is fully populated and fresh (it was written
// by this very sweep).
//
// Target behavior (coder, parallel dispatch): HandleNavigate's miss-branch
// seed falls back ProbeResources -> loaded per-type disk store rows
// (equivalent of rowsFromCacheRows + IsTruncated: !tf.Exact) when
// ProbeResources holds nothing for the type. All navigation entry points
// (menu Enter, TUI colon-command, -c armed command) funnel through this same
// HandleNavigate seed.
//
// Harness precedents:
//   - tui_warm_open_seed_test.go (DEF-12): HandleNavigate-level and
//     TUI-Update-level warm-open seed pins; newWarmOpenApp/rootApplyMsg/
//     rootViewContent/stripANSI patterns reused verbatim here.
//   - tui_savecache_routing_test.go (DEF-11): driveSweepCompletion pattern
//     for driving a real AvailabilityChecked-queue-drained completion
//     through the actual TUI renderer seam; runCmdTree for draining the
//     resulting tea.Cmd tree.
//   - qa_root_command_test.go: interactive colon-command entry pattern
//     (rootKeyPress(":") + chars + tea.KeyEnter) for driving the TUI
//     command-mode lane exactly as a live keystroke sequence would.
//   - qa_mainmenu_test.go: menu Enter-key navigation pattern (default cursor
//     is on EC2 Instances, index 0 — TestQA_MainMenu_MoveDownWithJ).
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

// ────────────────────────────────────────────────────────────────────────────
// Shared fixtures
// ────────────────────────────────────────────────────────────────────────────

// seedDiskStoreWithS3Rows writes a real on-disk per-type cache file for
// profile/region carrying 2 s3 rows via cache.Store.Put + SaveType, mirroring
// seedDiskCacheWithS3Rows in command_navigation_after_seed_test.go. Returns
// the freshly-reloaded *cache.Store, matching what EnsureCacheStore would
// see on a real read after the sweep's own TaskKindSaveCache wrote it.
func seedDiskStoreWithS3Rows(t *testing.T, profile, region string) *cache.Store {
	t.Helper()
	store := cache.LoadDir(profile, region)
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        2,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "arn:aws:s3:::def15-store-bucket-1", Name: "def15-store-bucket-1", Fields: map[string]string{"region": region}},
			{ID: "arn:aws:s3:::def15-store-bucket-2", Name: "def15-store-bucket-2", Fields: map[string]string{"region": region}},
		},
	})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("seed fixture SaveType(s3): %v", err)
	}
	return cache.LoadDir(profile, region)
}

// ────────────────────────────────────────────────────────────────────────────
// Test 1 — runtime level: HandleNavigate post-sweep must seed CachedEntry
// from RowStore's retained rows (RowStore is never freed post-sweep, task
// #17 wave 1 stage 2), which in this fixture match the disk store's rows.
// ────────────────────────────────────────────────────────────────────────────

// TestPostSweepWarmOpen_SeedsFromStore pins DEF-15 at the Core.HandleNavigate
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
// task #17 wave 1 stage 2 removed the legacy free that branch used to
// perform (session.ProbeResources/ProbeTruncated no longer exist; RowStore
// retains its rows for the session unconditionally — see RowStore.Amend's
// doc comment). The precondition below instead confirms RowStore has
// genuinely been observed for s3 with the AvailabilityChecked-landed rows,
// so the seed the assertions inspect provably reflects a completed
// post-sweep state.
//
// A post-sweep list-open must seed CachedEntry from RowStore's retained
// rows immediately (C1 "show what you know, then verify on sight"),
// matching disk-store data by construction in this fixture (the sweep wrote
// the same row IDs to both).
func TestPostSweepWarmOpen_SeedsFromStore(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	profile, region := "def15-store-profile", "us-east-1"
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

	// Confirm the pre-sweep seed landed (sanity check, not the assertion
	// under test): RowStore must be populated before we drive the sweep to
	// completion.
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
			{ID: "arn:aws:s3:::def15-store-bucket-1", Name: "def15-store-bucket-1", Type: "s3", Fields: map[string]string{"region": region}},
			{ID: "arn:aws:s3:::def15-store-bucket-2", Name: "def15-store-bucket-2", Type: "s3", Fields: map[string]string{"region": region}},
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

	// Precondition: task #17 wave 1 stage 2 removed the legacy
	// "all-enrichment-done" free this test used to prove fired here (RowStore
	// now retains its rows for the session unconditionally — see RowStore.
	// Amend's doc comment). Confirm instead that RowStore has genuinely been
	// observed for s3 (Gen != 0) with the same rows the sweep landed, so the
	// seed the assertions below inspect is demonstrably sourced from a
	// completed post-sweep state, not an untouched fixture.
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
		t.Fatal("result.CachedEntry = nil, want a synthetic entry seeded from the on-disk per-type store — DEF-15: a post-sweep list-open must not render a bare Loading shell when the disk cache holds complete, fresh rows")
	}
	if len(result.CachedEntry.Resources) != 2 {
		t.Fatalf("len(result.CachedEntry.Resources) = %d, want 2 (the disk store's persisted rows)", len(result.CachedEntry.Resources))
	}
	gotIDs := map[string]bool{}
	for _, r := range result.CachedEntry.Resources {
		gotIDs[r.ID] = true
	}
	for _, want := range []string{"arn:aws:s3:::def15-store-bucket-1", "arn:aws:s3:::def15-store-bucket-2"} {
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

// ────────────────────────────────────────────────────────────────────────────
// Test 2 — TUI level: post-sweep list-open via messages.Navigate must render
// disk-seeded rows, not the bare Loading shell.
// ────────────────────────────────────────────────────────────────────────────

// newPostSweepApp builds a tui.Model wired to demo clients with on-disk
// caching ENABLED (no WithNoCache) so EnsureCacheStore actually loads the
// disk store this test seeds — mirrors newSaveCacheApp's construction
// (tui_savecache_routing_test.go), not newWarmOpenApp's (which disables
// caching and would make the disk-store fallback unreachable).
func newPostSweepApp(t *testing.T, profile, region string) tui.Model {
	t.Helper()
	m := tui.New(profile, region,
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithProfile(profile),
		tui.WithRegion(region))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	t.Cleanup(m.CloseController)
	return m
}

// driveToPostSweepState drives m through the identical sequence Test 1 drives
// core through (AvailabilityCacheLoaded seed -> AvailabilityChecked -> N
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
			{ID: "arn:aws:s3:::def15-tui-bucket-1", Name: "def15-tui-bucket-1", Type: "s3", Fields: map[string]string{"region": region}},
			{ID: "arn:aws:s3:::def15-tui-bucket-2", Name: "def15-tui-bucket-2", Type: "s3", Fields: map[string]string{"region": region}},
		},
	})
	for i := 0; i < 5; i++ {
		m, _ = rootApplyMsg(m, messages.EnrichmentChecked{ResourceType: "s3"})
	}
	return m
}

// TestPostSweepWarmOpen_TUI_RendersRows pins DEF-15 at the real Bubble Tea
// Update/View seam. Same post-sweep state as Test 1, driven through
// tui.Model.Update; navigating via a messages.Navigate (exactly what pressing
// Enter on the main menu emits) must render the RowStore-seeded row names
// (driveToPostSweepState's own AvailabilityChecked delivery), not the bare
// "Loading..." shell.
//
// task #17 wave 1 stage 2 removed the legacy free HandleNavigate's
// disk-store fallback depended on (session.ProbeResources/ProbeTruncated no
// longer exist; RowStore retains its rows for the session unconditionally —
// see RowStore.Amend's doc comment). RowStore's own retained rows (the
// AvailabilityChecked-landed "def15-tui-bucket-*" rows) are therefore always
// what a post-sweep list-open seeds from; the on-disk store seeded here
// carries deliberately DIFFERENT row names ("def15-store-bucket-*") so this
// test can distinguish "rendered from RowStore" from "rendered from disk" —
// see TestPostSweepWarmOpen_SeedsFromStore for the disk-fallback path
// (reachable only when RowStore was never observed this session).
func TestPostSweepWarmOpen_TUI_RendersRows(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "def15-tui-profile", "us-east-1"
	seedDiskStoreWithS3Rows(t, profile, region)

	m := newPostSweepApp(t, profile, region)
	m = driveToPostSweepState(m, region)

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})

	content := stripANSI(rootViewContent(m))
	if strings.Contains(content, "Loading...") {
		t.Errorf("rendered view after post-sweep opening s3 (RowStore fully seeded) still shows the bare Loading shell — DEF-15:\n%s", content)
	}
	if !strings.Contains(content, "def15-tui-bucket-1") {
		t.Errorf("rendered view after post-sweep opening s3 does not contain the RowStore-seeded row %q:\n%s", "def15-tui-bucket-1", content)
	}
	if !strings.Contains(content, "def15-tui-bucket-2") {
		t.Errorf("rendered view after post-sweep opening s3 does not contain the RowStore-seeded row %q:\n%s", "def15-tui-bucket-2", content)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 3 — TUI level: the colon-command lane must render seeded rows too,
// mid-sweep (before the free) — per live tmux observation this path showed
// Loading even while ProbeResources was still populated.
// ────────────────────────────────────────────────────────────────────────────

// TestColonCommand_WarmOpen_RendersRows drives the actual interactive
// colon-command entry lane (":" + "s3" + Enter, exactly as
// qa_root_command_test.go's :root/:main tests do) while ProbeResources["s3"]
// is still populated (mid-sweep — AvailabilityCacheLoaded has seeded it, but
// enrichment has NOT yet reached completion, so the free has not fired).
// This isolates whether the Loading regression observed live in tmux
// (":s3" at 39/47, mid-sweep) shares DEF-15's root cause or is a distinct
// lane-divergence bug: if this test passes at HEAD, the live symptom's cause
// is NOT "seed never read" (Test 1/2 already cover that at completion) but
// something specific to the colon-command key-mode input path diverging from
// the messages.Navigate path Test 2 drives directly.
//
// Expected RED per the live observation. If GREEN at HEAD: the resolution
// (per dispatch instructions) is to report the finding rather than pin a
// false mechanism — see the test body's diagnostic branch.
func TestColonCommand_WarmOpen_RendersRows(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "def15-colon-profile", "us-east-1"
	seedDiskStoreWithS3Rows(t, profile, region)

	m := newPostSweepApp(t, profile, region)

	// Mid-sweep: AvailabilityCacheLoaded seeds ProbeResources; enrichment is
	// deliberately NOT driven to completion, so the free has not fired and
	// ProbeResources["s3"] is still populated — mirrors the live tmux
	// snapshot (39/47, sweep still in flight).
	m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"s3": 2},
	})
	m, _ = rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: true,
		Count:        2,
		Gen:          1,
		Resources: []resource.Resource{
			{ID: "arn:aws:s3:::def15-tui-bucket-1", Name: "def15-tui-bucket-1", Type: "s3", Fields: map[string]string{"region": region}},
			{ID: "arn:aws:s3:::def15-tui-bucket-2", Name: "def15-tui-bucket-2", Type: "s3", Fields: map[string]string{"region": region}},
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
		t.Errorf("rendered view after :s3 mid-sweep still shows the bare Loading shell — DEF-15 live-tmux symptom:\n%s", content)
	}
	if !strings.Contains(content, "def15-tui-bucket-1") && !strings.Contains(content, "def15-store-bucket-1") {
		t.Errorf("rendered view after :s3 mid-sweep does not contain a seeded row (neither ProbeResources' def15-tui-bucket-1 nor a disk-store fallback row):\n%s", content)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 4 — TUI level: menu Enter (the primary navigation entry point) must
// render seeded rows post-sweep too.
// ────────────────────────────────────────────────────────────────────────────

// TestMenuEnter_PostSweep_RendersRows drives Enter on the main-menu row for
// "ec2" (default cursor position, index 0 — TestQA_MainMenu_MoveDownWithJ in
// qa_mainmenu_test.go establishes this), post-sweep for ec2, and asserts the
// rendered view shows the disk-seeded rows rather than Loading.
func TestMenuEnter_PostSweep_RendersRows(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "def15-menu-profile", "us-east-1"

	store := cache.LoadDir(profile, region)
	store.Put("ec2", cache.TypeFile{
		HasResources: true,
		Count:        2,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "i-def15menu1", Name: "def15-menu-instance-1", Fields: map[string]string{"region": region}},
			{ID: "i-def15menu2", Name: "def15-menu-instance-2", Fields: map[string]string{"region": region}},
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
			{ID: "i-def15menu1", Name: "def15-menu-instance-1", Type: "ec2", Fields: map[string]string{"region": region}},
			{ID: "i-def15menu2", Name: "def15-menu-instance-2", Type: "ec2", Fields: map[string]string{"region": region}},
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
		t.Errorf("rendered view after menu-Enter on ec2 post-sweep still shows the bare Loading shell — DEF-15:\n%s", content)
	}
	if !strings.Contains(content, "def15-menu-instance-1") {
		t.Errorf("rendered view after menu-Enter on ec2 post-sweep does not contain the disk-seeded row %q:\n%s", "def15-menu-instance-1", content)
	}
	if !strings.Contains(content, "def15-menu-instance-2") {
		t.Errorf("rendered view after menu-Enter on ec2 post-sweep does not contain the disk-seeded row %q:\n%s", "def15-menu-instance-2", content)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 5 — C9 guard: a pair switch must NOT let the disk-store fallback leak
// the OLD pair's rows into the NEW pair's post-sweep seed.
// ────────────────────────────────────────────────────────────────────────────

// TestPairSwitch_PostSweep_NoStaleSeed guards the fix against a C9
// regression: profile A's disk store carries s3 rows; after Rotate() (a
// profile switch to B, whose disk store carries NO s3 data at all),
// HandleNavigate("s3") must NOT seed CachedEntry from A's now-stale store —
// EnsureCacheStore is pair-scoped (Session.EnsureCacheStore reloads
// cache.LoadDir(profile, region) whenever the memoized store's pair differs
// from session.Profile/Region), so a correct fallback implementation reads
// B's (empty) store, not A's. This is a non-regression guard, not a RED pin:
// it must be green both before and after the DEF-15 fix lands, proving the
// fix does not introduce a stale cross-pair leak.
func TestPairSwitch_PostSweep_NoStaleSeed(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	profileA, profileB, region := "def15-pairswitch-a", "def15-pairswitch-b", "us-east-1"
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

// ────────────────────────────────────────────────────────────────────────────
// Test 6/7 — Codex P2 on DEF-15's own seed: an observed-empty probe result
// this session must NOT fall back to stale disk-store rows.
//
// Storage-shape finding (dispatch item 3): handleAvailabilityChecked
// (internal/runtime/handlers_availability.go L233-274) stores into
// session.ProbeResources[canonType] whenever "msg.Err == nil ||
// len(msg.Resources) > 0" (L240). For a genuinely-empty type the live probe
// sends Err=nil, Resources=nil/empty, HasResources=false, Count=0 — Err==nil
// alone satisfies the guard, so the store DOES run:
// session.ProbeResources[canon] = msg.Resources (nil/empty slice). The map
// key is therefore PRESENT with a zero-length slice, not absent. This is the
// exact shape HandleNavigate's fallback must distinguish via a two-value map
// read (observed bool) rather than a bare "len(rows) > 0" truthiness check.
// No gap: the zero-resource path is reachable through the real handler with
// real message fields, driven below via Core.HandleEvent(AvailabilityChecked)
// exactly as production code would emit it.
//
// P2 defect (pre-fix, working tree at time of writing): HandleNavigate read
// "if rows := c.session.ProbeResources[canon]; len(rows) > 0" — an
// observed-empty slice fails that truthiness check identically to an absent
// key, so execution falls through to the disk-store branch and seeds stale
// rows for a type the current session just confirmed is empty.
// ────────────────────────────────────────────────────────────────────────────

// TestObservedEmpty_DoesNotSeedStaleDiskRows pins the P2 fix at the
// Core.HandleNavigate seam: the disk store for "s3" carries 3 stale rows
// (written before this session started, e.g. a prior session's sweep), but
// THIS session's live Wave-1 probe already confirmed s3 has zero resources.
// HandleNavigate must not seed CachedEntry from the stale disk rows — a
// current-session observed-empty result is fresher than any disk row (C2).
func TestObservedEmpty_DoesNotSeedStaleDiskRows(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	profile, region := "def15p2-store-profile", "us-east-1"

	store := cache.LoadDir(profile, region)
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        3,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "arn:aws:s3:::def15p2-stale-bucket-1", Name: "def15p2-stale-bucket-1", Fields: map[string]string{"region": region}},
			{ID: "arn:aws:s3:::def15p2-stale-bucket-2", Name: "def15p2-stale-bucket-2", Fields: map[string]string{"region": region}},
			{ID: "arn:aws:s3:::def15p2-stale-bucket-3", Name: "def15p2-stale-bucket-3", Fields: map[string]string{"region": region}},
		},
	})
	if err := store.SaveType("s3"); err != nil {
		t.Fatalf("seed fixture SaveType(s3): %v", err)
	}

	sess := session.New()
	sess.Profile = profile
	sess.Region = region
	core := runtime.New(sess, catalog.All())

	// Drive the real live-probe event for a genuinely-empty type, exactly as
	// production fires it: Err=nil, HasResources=false, Count=0,
	// Resources=nil. Per the storage-shape finding above, this DOES populate
	// session.ProbeResources["s3"] with a zero-length slice (Err==nil alone
	// satisfies handleAvailabilityChecked's storage guard).
	_, _ = core.HandleEvent(messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: false,
		Count:        0,
		Gen:          1,
		Err:          nil,
		Resources:    nil,
	})

	// Precondition: prove RowStore observed s3 this session (Gen != 0 — the
	// observed-at-all discriminator per handlers_navigate.go's own doc
	// comment) with a zero-length Rows slice — this is not a trivially-true
	// setup. If this fails, the storage-shape assumption above is wrong and
	// the test cannot validate the observed-empty fallback distinction.
	//
	// Note: Core.ProbeResources()'s own two-value return cannot be used for
	// this precondition — it reports ok=false whenever len(Rows)==0
	// regardless of Gen, so it cannot distinguish "never observed" from
	// "observed empty" (unlike HandleNavigate's own RowStore.Snapshot(canon).
	// Gen != 0 check, which this precondition mirrors directly instead).
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
		t.Fatalf("result.CachedEntry = %+v, want nil — this session observed s3 as empty via a live probe; seeding from the stale on-disk store rows (%d rows) would render resources the current session already knows do not exist (Codex P2 on DEF-15)", result.CachedEntry, len(result.CachedEntry.Resources))
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
	const profile, region = "def15p2-tui-profile", "us-east-1"

	store := cache.LoadDir(profile, region)
	store.Put("s3", cache.TypeFile{
		HasResources: true,
		Count:        3,
		Exact:        true,
		Rows: []cache.Row{
			{ID: "arn:aws:s3:::def15p2-tui-stale-1", Name: "def15p2-tui-stale-1", Fields: map[string]string{"region": region}},
			{ID: "arn:aws:s3:::def15p2-tui-stale-2", Name: "def15p2-tui-stale-2", Fields: map[string]string{"region": region}},
			{ID: "arn:aws:s3:::def15p2-tui-stale-3", Name: "def15p2-tui-stale-3", Fields: map[string]string{"region": region}},
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
	for _, stale := range []string{"def15p2-tui-stale-1", "def15p2-tui-stale-2", "def15p2-tui-stale-3"} {
		if strings.Contains(content, stale) {
			t.Errorf("rendered view after opening s3 (observed empty this session via live probe) contains stale disk-store row %q — Codex P2 on DEF-15:\n%s", stale, content)
		}
	}
}

// TestUnobserved_StillSeedsFromStore is the regression guard for DEF-15
// itself: when ProbeResources lacks the "s3" key entirely (never observed
// this session — the map is nil, e.g. before any probe or after the
// post-sweep free), HandleNavigate must still fall back to the on-disk
// per-type store and seed CachedEntry from its rows. Must stay green both
// before and after the P2 fix — it pins the DEF-15 base behavior the P2 fix
// must not regress.
func TestUnobserved_StillSeedsFromStore(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	profile, region := "def15p2-unobserved-profile", "us-east-1"
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
		t.Fatal("result.CachedEntry = nil, want a synthetic entry seeded from the on-disk per-type store — DEF-15 base behavior: an unobserved type (key absent) must still fall back to disk-store rows")
	}
	if len(result.CachedEntry.Resources) != 2 {
		t.Fatalf("len(result.CachedEntry.Resources) = %d, want 2 (the disk store's persisted rows)", len(result.CachedEntry.Resources))
	}
	gotIDs := map[string]bool{}
	for _, r := range result.CachedEntry.Resources {
		gotIDs[r.ID] = true
	}
	for _, want := range []string{"arn:aws:s3:::def15-store-bucket-1", "arn:aws:s3:::def15-store-bucket-2"} {
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
