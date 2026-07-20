// tui_toplevel_list_persistence_test.go — pin tests for #17 wave 2,
// verifying the coder's fix in internal/tui/runtime_adapter_navigate.go: a
// top-level, menu-driven list open now pushes runtime.ScreenResourceList
// (via app.Controller.ApplyIntents + EnsureListState), not
// PushChildListScreen's runtime.ScreenChildList. Before that fix, EVERY
// top-level TUI list open silently disabled Controller.maybeSaveResourceListCache's
// C6 disk-cache save gate (core/app/handle.go's syncExactTotalToMenu,
// which only persists when screen.ID == runtime.ScreenResourceList) — a
// genuine navigate-and-append session in the running TUI never wrote a
// per-type cache file to disk, RED at commit effdd465 (confirmed by reading
// PushChildListScreen's runtime.ScreenChildList push at that commit).
//
// See core/app/handle.go, core/app/list_body.go,
// core/app/navigate.go, core/runtime/handlers_navigate.go for the
// production-side contracts (top-level list save gate, seed-time provisional
// total, silent-swap findings carry) these tests pin against.
//
// SCOPE NOTE on the screen-ID guard: internal/tui.Model.ctrl is
// unexported and Model exposes no accessor onto app.Controller (only
// Model.Core() for *runtime.Core, which does not carry ScreenIDs()) — so a
// tests/unit black-box test cannot directly assert
// (*app.Controller).ScreenIDs() through the TUI layer. maybeSaveResourceListCache
// and syncExactTotalToMenu share the exact same single gate
// (screen.ID == runtime.ScreenResourceList — core/app/handle.go), so
// TestTopLevelListOpen_TUI_PersistsAppendedRowsToDisk below (which CAN only
// pass if that gate is satisfied) is an indirect but airtight proof that the
// TUI's pushed screen carries ScreenResourceList, not ScreenChildList.
// TestScreenIDGuard_TopLevelCommandVsPushChildListScreen additionally pins
// the underlying invariant directly at the core/app layer (both call
// paths are reachable from tests/unit), guarding the semantic the TUI
// depends on even though this package cannot observe *tui.Model's own
// screen stack.
package unit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// -----------------------------------------------------------------------
// Pin (a): a top-level TUI list open + append persists rows to disk.
// -----------------------------------------------------------------------

// TestTopLevelListOpen_TUI_PersistsAppendedRowsToDisk drives a genuine
// navigate-then-append session entirely through the TUI layer
// (tuitest.Sized -> messages.Navigate -> messages.ResourcesLoaded, exactly
// the shape internal/tui/app.go's Update dispatches for a real fetch/load-more
// result) and asserts the per-type disk cache file ends up holding the FULL,
// accumulated row set — not just the first page, and not nothing at all (the
// HEAD-effdd465 regression: the gate never fired for this call path, so
// s3.yaml was never created).
func TestTopLevelListOpen_TUI_PersistsAppendedRowsToDisk(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	const profile, region = "toplevel-persist-prof", "us-east-1"

	m := tuitest.Sized(profile, region)
	t.Cleanup(func() { m.CloseController() })

	// Open the s3 list (top-level, menu-driven — the exact ":command open"
	// shape, since messages.Navigate{TargetResourceList} is what both the
	// main-menu Enter key and the ":command <type>" path dispatch).
	m, _ = tuitest.Step(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})

	// First page lands (Append: false — the initial fetch result shape).
	firstPage := []resource.Resource{
		{ID: "bucket-1", Name: "bucket-1", Type: "s3", Fields: map[string]string{"name": "bucket-1"}},
		{ID: "bucket-2", Name: "bucket-2", Type: "s3", Fields: map[string]string{"name": "bucket-2"}},
	}
	m, _ = tuitest.Step(m, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    firstPage,
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-1"},
		Append:       false,
	})

	// Load-more appends a second page and exhausts pagination (the "open +
	// append" shape the pin requires).
	secondPage := []resource.Resource{
		{ID: "bucket-3", Name: "bucket-3", Type: "s3", Fields: map[string]string{"name": "bucket-3"}},
	}
	m, _ = tuitest.Step(m, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    secondPage,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Append:       true,
	})
	_ = m

	store := cache.LoadDirForTest(profile, region)
	if store == nil {
		t.Fatal("cache.LoadDirForTest returned nil — expected a persisted s3 type file")
	}
	tf, ok := store.Type("s3")
	if !ok {
		t.Fatalf("cache.LoadDirForTest(%q, %q).Type(\"s3\") missing — top-level TUI list open+append never persisted to disk (HEAD-effdd465 regression: the C6 save gate never fired for a TUI-pushed list)", profile, region)
	}
	if len(tf.Rows) != 3 {
		t.Fatalf("persisted s3 Rows count = %d, want 3 (both loaded pages, not just the first)", len(tf.Rows))
	}
	gotIDs := map[string]bool{}
	for _, r := range tf.Rows {
		gotIDs[r.ID] = true
	}
	for _, want := range []string{"bucket-1", "bucket-2", "bucket-3"} {
		if !gotIDs[want] {
			t.Errorf("persisted s3 Rows missing ID %q — got %v", want, tf.Rows)
		}
	}
	if !tf.Exact {
		t.Error("persisted s3 Exact = false, want true — load-more exhausted pagination (IsTruncated=false)")
	}
	if tf.Count != 3 {
		t.Errorf("persisted s3 Count = %d, want 3", tf.Count)
	}

	// Confirm the file that ended up on disk is exactly where cache.DirForTest says
	// it should be, and that it is genuinely readable YAML, not merely an
	// in-memory Store observation independent of a real file write.
	wantDir := filepath.Join(tmp, "cache", profile+"--"+region)
	if _, err := os.Stat(filepath.Join(wantDir, "s3.yaml")); err != nil {
		t.Errorf("expected s3.yaml under %s: %v", wantDir, err)
	}
}

// -----------------------------------------------------------------------
// Pin (b): the underlying screen-ID invariant the TUI fix relies on.
// -----------------------------------------------------------------------

// TestScreenIDGuard_TopLevelCommandVsPushChildListScreen pins, at the
// core/app layer, the exact invariant the TUI adapter's fix depends on:
// a top-level command-driven list open (the app.Controller path
// handleActionCommand -> applyNavResult mirrors what the TUI's fixed
// handleNavigate now drives via ApplyIntents{PushScreen{ScreenResourceList}})
// pushes runtime.ScreenResourceList, while PushChildListScreen (the OLD,
// wrong call the TUI used to make for a top-level open, still correct for a
// genuine child/related list) pushes runtime.ScreenChildList. Both screen
// IDs are asserted in the SAME test so it cannot pass merely because
// ScreenIDs() always reports one fixed value.
func TestScreenIDGuard_TopLevelCommandVsPushChildListScreen(t *testing.T) {
	newCtrl := func(profile string) *app.Controller {
		s := session.New()
		s.Profile = profile
		s.Region = "us-east-1"
		core := runtime.New(s, nil)
		c := app.New(core)
		t.Cleanup(c.Close)
		return c
	}

	t.Run("top_level_command_open_pushes_ScreenResourceList", func(t *testing.T) {
		c := newCtrl("screenid-cmd-prof")
		c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
		ids := c.ScreenIDs()
		if len(ids) == 0 {
			t.Fatal("ScreenIDs() empty after ActionCommand open")
		}
		if got := ids[len(ids)-1]; got != runtime.ScreenResourceList {
			t.Errorf("top ScreenID = %q, want %q", got, runtime.ScreenResourceList)
		}
	})

	t.Run("PushChildListScreen_pushes_ScreenChildList", func(t *testing.T) {
		c := newCtrl("screenid-child-prof")
		c.PushChildListScreen("ec2")
		ids := c.ScreenIDs()
		if len(ids) == 0 {
			t.Fatal("ScreenIDs() empty after PushChildListScreen")
		}
		if got := ids[len(ids)-1]; got != runtime.ScreenChildList {
			t.Errorf("top ScreenID = %q, want %q — PushChildListScreen must still be usable for genuine child/related lists", got, runtime.ScreenChildList)
		}
	})
}

// -----------------------------------------------------------------------
// Pin (c): a seeded C6a pair (Count > len(Rows)) titles with the authoritative
// total, and a genuine fetch clears the override.
// -----------------------------------------------------------------------

// TestSeededC6aPair_TitleShowsCountNotRowsLen_ThenClearsOnRealFetch pins the
// seed-time provisional total end-to-end through the TUI: a disk pair reconstructed with
// Count=55 but only 50 Rows (the C6a "counts-only write never touches Rows"
// shape) must seed the s3 list with title "s3(55)", not "s3(50)" — and once
// a genuine fetch result lands, the title must follow the real row count
// again (TotalCount cleared by applyResourcesLoaded).
func TestSeededC6aPair_TitleShowsCountNotRowsLen_ThenClearsOnRealFetch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	const profile, region = "c6a-pair-prof", "us-east-1"

	// Seed the on-disk C6a pair directly: Count=55, only 50 Rows persisted —
	// this is reachable in production via a counts-only reconciling write
	// (reconcileTypeFile) that updates Count without touching Rows.
	s := session.New()
	s.Profile = profile
	s.Region = region
	core := runtime.New(s, nil)
	rows := make([]cache.Row, 50)
	for i := range rows {
		rows[i] = cache.Row{ID: "obj-" + itoaC6a(i), Name: "obj-" + itoaC6a(i)}
	}
	if err := core.WithCacheStore(func(store *cache.Store) error {
		store.Put("s3", cache.TypeFile{Count: 55, Exact: false, Rows: rows})
		return store.SaveType("s3")
	}); err != nil {
		t.Fatalf("seeding C6a pair: %v", err)
	}

	// Fresh TUI model over the SAME profile/region pair, so HandleNavigate's
	// cache-miss fallback (no ProbeResources observed yet) reads the just-seeded
	// disk pair (core/runtime/handlers_navigate.go's ReadCacheStore branch).
	m := tuitest.Sized(profile, region)
	t.Cleanup(func() { m.CloseController() })
	m, _ = tuitest.Step(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})

	// Exact=false (a truncated/inexact C6a pair) renders with the "+" suffix
	// (buildListFrameTitle: totalStr = itoa(total)+"+" when ls.HasPagination),
	// so the expected title is "s3(55+)", not a bare "s3(55)".
	seededPlain := stripANSITL(tuitest.Render(m))
	if !strings.Contains(seededPlain, "s3(55+)") {
		t.Errorf("seeded C6a pair: expected title s3(55+) (Count, not len(Rows)=50), got: %s", seededPlain[:min(200, len(seededPlain))])
	}
	if strings.Contains(seededPlain, "s3(50") {
		t.Errorf("seeded C6a pair: title must not show s3(50...) (len(Rows)) while TotalCount=55 is authoritative, got: %s", seededPlain[:min(200, len(seededPlain))])
	}

	// A genuine fetch result lands with the REAL total (60 rows, exact) —
	// TotalCount must clear and the title must follow the real row count.
	realRows := make([]resource.Resource, 60)
	for i := range realRows {
		realRows[i] = resource.Resource{ID: "obj-real-" + itoaC6a(i), Type: "s3"}
	}
	m, _ = tuitest.Step(m, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    realRows,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Append:       false,
	})

	afterFetchPlain := stripANSITL(tuitest.Render(m))
	if !strings.Contains(afterFetchPlain, "s3(60)") {
		t.Errorf("after real fetch: expected title s3(60) (real row count, TotalCount override cleared), got: %s", afterFetchPlain[:min(200, len(afterFetchPlain))])
	}
	if strings.Contains(afterFetchPlain, "s3(55)") {
		t.Errorf("after real fetch: stale seeded total 's3(55)' must not still be showing, got: %s", afterFetchPlain[:min(200, len(afterFetchPlain))])
	}
}

// -----------------------------------------------------------------------
// Pin (d): the refreshing marker is present on the FIRST rendered frame.
// -----------------------------------------------------------------------

// TestCacheMissSeed_RefreshingMarkerOnFirstFrame pins that a cache-miss-but-
// probe-seeded top-level list open (NavigateKindPushResourceList's
// CachedEntry branch, seeded here via session.RowStore) shows the
// "refreshing..." marker on the VERY FIRST rendered frame after
// messages.Navigate — i.e. before any fetch-result cmd is executed or
// drained. internal/tui/runtime_adapter_navigate.go constructs the list via
// NewResourceListFromCache (which clears Refreshing as part of applying the
// seeded page) and then calls SetListRefreshing(true) AFTER construction, so
// the marker must already be present the moment View() is called — no
// intervening message required.
func TestCacheMissSeed_RefreshingMarkerOnFirstFrame(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	const profile, region = "refresh-marker-prof", "us-east-1"

	m := tuitest.Sized(profile, region)
	t.Cleanup(func() { m.CloseController() })

	// Seed session.RowStore for ec2 BEFORE navigating, exactly as a
	// prior availability probe would have — this is the CachedEntry seed
	// source for the cache-miss branch (NavigateKindPushResourceList).
	core := m.Core()
	core.Session().RowStore.Observe("ec2", []resource.Resource{
		{ID: "i-seed1", Name: "seed-1", Type: "ec2", Fields: map[string]string{"state": "running"}},
		{ID: "i-seed2", Name: "seed-2", Type: "ec2", Fields: map[string]string{"state": "stopped"}},
	}, nil, session.OriginProbe, false)

	m, _ = tuitest.Step(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Render the FIRST frame — no fetch cmd has been executed or drained.
	firstFrame := stripANSITL(tuitest.Render(m))
	if !strings.Contains(firstFrame, "refreshing") {
		t.Errorf("first rendered frame after cache-miss seed should already show the refreshing marker, got:\n%s", firstFrame)
	}
	if !strings.Contains(firstFrame, "seed-1") || !strings.Contains(firstFrame, "seed-2") {
		t.Errorf("first rendered frame should show the seeded rows immediately (render what you know), got:\n%s", firstFrame)
	}
}

// stripANSITL strips ANSI escape sequences the same way tuitest.StripANSI
// does — a local alias avoids a naming collision with this package's other
// files' own stripANSI helpers.
func stripANSITL(s string) string {
	return tuitest.StripANSI(s)
}

// itoaC6a is a tiny local decimal formatter (mirrors itoaTest in
// runtime_cache_rows_exact_totals_test.go) so this file has no extra
// strconv-only import for loop-counter formatting.
func itoaC6a(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
