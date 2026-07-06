// tui_warm_open_seed_test.go — RED pin for DEF-12 (C1 + Goal 4 of
// docs/design/cache-requirements.md).
//
// Root cause: on a cache-MISS (no session.ResourceCache entry for the type
// yet, but session.RowStore DOES hold retained first-page rows from a
// prior Wave-1 probe / disk-cache replay), the two renderer adapters diverge:
//
//   - internal/app/navigate.go's applyNavResult, NavigateKindPushResourceList
//     branch (controller/headless/web lane) seeds the pushed list straight
//     from c.core.Session().RowStore.Snapshot(res.ResolvedType) (Rows +
//     Pagination.IsTruncated → synthetic PaginationMeta), so the list renders
//     instantly with Refreshing=true instead of Loading=true.
//   - internal/tui/runtime_adapter_navigate.go's NavigateKindPushResourceList
//     case (live TUI lane) does no such seeding — it only calls
//     views.NewResourceList(...).Init() and dispatches the fetch, so the
//     screen renders the bare "Loading..." shell (resourcelist.go View(),
//     "snap.Body.List == nil") until the live fetch round-trip lands, even
//     though the exact same probe rows are sitting in session state.
//
// The fix (coder, parallel dispatch) moves the seeding decision up to
// runtime.Core.HandleNavigate: on the NavigateTargetResourceList cache-MISS
// branch, when session.RowStore.Snapshot(canon) is non-empty, the runtime
// attaches a synthetic session.ResourceCacheEntry on NavigateResult.CachedEntry
// (reusing the same field the cache-HIT branch already populates) while STILL
// returning the KindFetchResources task (C1 "show what you know, then verify
// on sight" — the seed never substitutes for the live fetch). Both adapters
// then consume CachedEntry uniformly instead of the TUI adapter doing nothing
// and the controller re-deriving its own local seed.
//
// Test 1 (HandleNavigate_MissWithProbeRows_AttachesSeedAndFetchTask) pins the
// runtime-level contract directly against Core.HandleNavigate.
//
// Test 2 (TUI_WarmOpen_RendersSeededRows_NotLoading) drives the real Bubble
// Tea renderer seam (tuitest.Step/Render, exactly as tui_savecache_routing_test.go
// and tui_error_marker_parity_test.go do) and asserts the rendered View()
// shows the seeded row instead of the bare "Loading..." shell.
//
// Test 8 in app_cache_wave_regressions_test.go
// (TestWarmListOpen_TruncatedDiskSeed_ShowsNPlus_BeforeRefetch) is the
// existing, already-green parity anchor for the controller/headless lane —
// it is NOT duplicated here.
package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// ────────────────────────────────────────────────────────────────────────────
// Test 1 — runtime-level: HandleNavigate must attach the probe-row seed on
// a cache miss, without dropping the verify-on-sight fetch task.
// ────────────────────────────────────────────────────────────────────────────

// TestHandleNavigate_MissWithProbeRows_AttachesSeedAndFetchTask pins DEF-12
// at the runtime.Core.HandleNavigate seam: session.ResourceCache has NO entry
// for "s3" (a genuine cache miss), but session.RowStore holds retained
// first-page rows for "s3" from a prior Wave-1 probe, with
// Pagination.IsTruncated = true. HandleNavigate(NavigateTargetResourceList,
// "s3") must:
//
//  1. still return NavigateKindPushResourceList (not the Cached variant —
//     this is genuinely a cache miss, not a cache hit),
//  2. populate NavigateResult.CachedEntry with the probe rows and an
//     IsTruncated=true pagination marker, so the adapter can seed the list
//     immediately instead of rendering a blank Loading shell,
//  3. still return the KindFetchResources task — the seed augments the
//     miss path, it does not replace the live verify-on-sight fetch (C1).
//
// RED today: CachedEntry is nil on the miss branch (handlers_navigate.go's
// NavigateTargetResourceList case only populates CachedEntry on the
// cache-HIT branch, lines ~140-145).
func TestHandleNavigate_MissWithProbeRows_AttachesSeedAndFetchTask(t *testing.T) {
	sess := session.New()
	probeRows := []resource.Resource{
		{ID: "arn:aws:s3:::def12-warm-bucket-1", Name: "def12-warm-bucket-1", Type: "s3", Fields: map[string]string{"region": "us-east-1"}},
		{ID: "arn:aws:s3:::def12-warm-bucket-2", Name: "def12-warm-bucket-2", Type: "s3", Fields: map[string]string{"region": "us-east-1"}},
	}
	sess.RowStore.Observe("s3", probeRows, &resource.PaginationMeta{IsTruncated: true}, session.OriginProbe, false)

	core := runtime.New(sess, catalog.All())

	// Explicitly confirm the fixture is a genuine cache MISS. handlers_navigate.go's
	// own doc comment (the NavigateTargetResourceList case, "this check above
	// (c.ResourceCache(canon)) only hits a FULL (non-Partial, OriginFetch)
	// RowStore entry") states that an OriginProbe-only entry must NOT satisfy
	// core.HasResourceCache. If this precondition fails, RowStore.Observe's
	// Partial-clearing rule (session/rowstore.go: "a plain Observe is by
	// definition a full, non-partial observation", applied identically
	// regardless of Origin) has drifted from that stated contract — a
	// production defect in the Origin/Partial semantics, not a stale test.
	if core.HasResourceCache("s3") {
		t.Fatal("test setup: core.HasResourceCache(s3) unexpectedly true after only an OriginProbe Observe — this test requires a genuine cache miss per handlers_navigate.go's own documented contract")
	}

	result, tasks := core.HandleNavigate(runtime.NavigateEvent{
		Target:       runtime.NavigateTargetResourceList,
		ResourceType: "s3",
	})

	if result.Kind != runtime.NavigateKindPushResourceList {
		t.Fatalf("result.Kind = %v, want NavigateKindPushResourceList — this is a genuine cache miss, not a cache hit", result.Kind)
	}
	if result.CachedEntry == nil {
		t.Fatal("result.CachedEntry = nil, want a synthetic entry seeded from session.RowStore.Snapshot(s3) — DEF-12: warm list-open must not render a bare Loading shell when probe rows are already known")
	}
	if len(result.CachedEntry.Resources) != 2 {
		t.Fatalf("len(result.CachedEntry.Resources) = %d, want 2 (the retained probe rows)", len(result.CachedEntry.Resources))
	}
	if result.CachedEntry.Resources[0].ID != "arn:aws:s3:::def12-warm-bucket-1" {
		t.Errorf("result.CachedEntry.Resources[0].ID = %q, want %q", result.CachedEntry.Resources[0].ID, "arn:aws:s3:::def12-warm-bucket-1")
	}
	if result.CachedEntry.Resources[1].ID != "arn:aws:s3:::def12-warm-bucket-2" {
		t.Errorf("result.CachedEntry.Resources[1].ID = %q, want %q", result.CachedEntry.Resources[1].ID, "arn:aws:s3:::def12-warm-bucket-2")
	}
	if result.CachedEntry.Pagination == nil || !result.CachedEntry.Pagination.IsTruncated {
		t.Errorf("result.CachedEntry.Pagination = %+v, want non-nil with IsTruncated=true (mirrors RowStore.Snapshot(s3).Pagination.IsTruncated)", result.CachedEntry.Pagination)
	}

	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1 — the seed must NOT replace the verify-on-sight fetch task (C1)", len(tasks))
	}
	if tasks[0].Key.Kind != runtime.KindFetchResources {
		t.Errorf("tasks[0].Key.Kind = %v, want KindFetchResources", tasks[0].Key.Kind)
	}
	if tasks[0].Key.Scope != "s3" {
		t.Errorf("tasks[0].Key.Scope = %q, want %q", tasks[0].Key.Scope, "s3")
	}
}

// TestHandleNavigate_MissWithoutProbeRows_NoCachedEntry is the non-regression
// half of Test 1: a genuine cache miss with NO retained probe rows (the
// common cold-open case) must NOT synthesize a CachedEntry out of thin air —
// CachedEntry stays nil and the adapter falls back to the ordinary Loading
// shell, exactly as before this fix.
func TestHandleNavigate_MissWithoutProbeRows_NoCachedEntry(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())

	result, tasks := core.HandleNavigate(runtime.NavigateEvent{
		Target:       runtime.NavigateTargetResourceList,
		ResourceType: "s3",
	})

	if result.Kind != runtime.NavigateKindPushResourceList {
		t.Fatalf("result.Kind = %v, want NavigateKindPushResourceList", result.Kind)
	}
	if result.CachedEntry != nil {
		t.Errorf("result.CachedEntry = %+v, want nil — no probe rows were retained, nothing to seed from", result.CachedEntry)
	}
	if len(tasks) != 1 || tasks[0].Key.Kind != runtime.KindFetchResources {
		t.Errorf("tasks = %+v, want exactly one KindFetchResources task", tasks)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 2 — TUI-level: a warm list-open must render seeded rows, not the bare
// Loading shell.
// ────────────────────────────────────────────────────────────────────────────

// newWarmOpenApp builds a tui.Model wired to demo clients (no real AWS
// calls), sized so View() renders real content. Mirrors newErrorMarkerApp /
// newSaveCacheApp's construction pattern from sibling test files in this
// package.
func newWarmOpenApp(t *testing.T) tui.Model {
	t.Helper()
	m := tui.New("def12-warmopen-demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile("def12-warmopen-demo"),
		tui.WithRegion("us-east-1"))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// TestTUI_WarmOpen_RendersSeededRows_NotLoading pins DEF-12 at the real
// Bubble Tea Update/View seam. A prior Wave-1 availability probe for "s3" is
// driven through the exact same seam tui_savecache_routing_test.go's
// driveSweepCompletion uses (a real messages.AvailabilityChecked with
// Gen: 1 — session.New() seeds AvailabilityGen at 1, not 0, and
// AvailabilityChecked.AcceptZeroGen() is false), which naturally populates
// session.RowStore's "s3" entry (Rows + Pagination.IsTruncated = true) via
// handleAvailabilityChecked — exactly as a live warm-open scenario would
// have them populated from a completed background sweep. Crucially, this
// probe result does NOT populate session.ResourceCache — only a subsequent
// list-open (Navigate) would, and that is the cache-miss path under test.
//
// Navigating to "s3" (a Navigate message, exactly as pressing Enter on the
// main menu would send) must render the seeded row and pagination-truncated
// "2+" marker immediately, WITHOUT waiting for a fetch round-trip — the
// rendered View() must NOT show the bare "Loading..." shell.
//
// RED today: runtime_adapter_navigate.go's NavigateKindPushResourceList case
// only calls rl.Init() and dispatches the fetch; it never seeds from
// RowStore, so the list starts empty (Loading=true) until the fetch
// completes.
func TestTUI_WarmOpen_RendersSeededRows_NotLoading(t *testing.T) {
	m := newWarmOpenApp(t)

	sweepRows := []resource.Resource{
		{ID: "arn:aws:s3:::def12-tui-warm-bucket-1", Name: "def12-tui-warm-bucket-1", Type: "s3", Fields: map[string]string{"region": "us-east-1"}},
		{ID: "arn:aws:s3:::def12-tui-warm-bucket-2", Name: "def12-tui-warm-bucket-2", Type: "s3", Fields: map[string]string{"region": "us-east-1"}},
	}
	m, _ = rootApplyMsg(m, messages.AvailabilityChecked{
		ResourceType: "s3",
		HasResources: true,
		Count:        len(sweepRows),
		Truncated:    true,
		Resources:    sweepRows,
		Gen:          1,
	})

	m, navCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})
	_ = navCmd

	content := stripANSI(rootViewContent(m))
	if strings.Contains(content, "Loading...") {
		t.Errorf("rendered view after warm-opening s3 (with probe rows already retained) still shows the bare Loading shell — DEF-12: known rows must render instantly:\n%s", content)
	}
	if !strings.Contains(content, "def12-tui-warm-bucket-1") {
		t.Errorf("rendered view after warm-opening s3 does not contain the seeded row %q:\n%s", "def12-tui-warm-bucket-1", content)
	}
	if !strings.Contains(content, "def12-tui-warm-bucket-2") {
		t.Errorf("rendered view after warm-opening s3 does not contain the seeded row %q:\n%s", "def12-tui-warm-bucket-2", content)
	}
	if !strings.Contains(content, "2+") {
		t.Errorf("rendered view after warm-opening a truncated probe seed does not contain the %q lower-bound title marker:\n%s", "2+", content)
	}
}
