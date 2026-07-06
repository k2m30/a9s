// qa_load_more_dedup_test.go — RED pins for DEF-17.
//
// Defect (observed live, TUI cold pilot): cold `:s3` open (fresh pair) lands
// on page 1 (50 rows, truncated, NextToken="p2"). Pressing 'm' (load more)
// appends a FULL DUPLICATE of page 1 instead of fetching page 2 — title
// grows 50+ -> 100+, row IDs repeat consecutively, the More hint persists.
// The persisted per-type cache file then carries Count:100 with only 50
// distinct rows — a mismatched pair the DEF-4b matched-pair rule in
// Core.SaveResourceListCache (internal/runtime/probes.go) forbids.
//
// A coder is root-causing the mechanism in parallel. These pins target the
// symptom at four seams:
//
//  1. LoadMore_TUI_ColdOpen_NoDuplicates — real Bubble Tea Update loop, real
//     'm' keypress through internal/tui/views/resourcelist.go's
//     key.Matches(msg, m.keys.LoadMore) branch.
//  2. LoadMore_TokenPresent_AfterColdOpen — the controller getter the 'm'
//     path reads (Controller.GetListPaginationCursor) must equal the
//     fetcher-returned NextToken after a cold open settles.
//  3. AppendDedup_Backstop — Controller.ApplyResourcesLoaded(append=true)
//     must not duplicate rows whose IDs already exist on the screen.
//  4. PersistedPair_NeverMismatched — after driving the poisoning sequence
//     through the real save wiring, the persisted TypeFile must never leave
//     Count > len(Rows) via a double-append (DEF-4b).
//
// Harness precedents:
//   - tests/unit/runtime_executor_depth_refetch_test.go: page1(50,
//     truncated, "p2") / page2(5, exact) SetPaginatedForTest fixture shape,
//     reused verbatim here (bucketID/page1Resources/page2Resources are
//     package-level helpers already defined there — reused directly).
//   - tests/unit/tui_post_sweep_seed_test.go: newPostSweepApp / rootApplyMsg
//     / rootViewContent / stripANSI / rootKeyPress / rootSpecialKey TUI-drive
//     pattern (real tui.Model, on-disk caching enabled).
//   - tests/unit/app_pilot_defects_test.go: app.New(core) + ctrl.Apply
//     (app.Action{Kind: app.ActionCommand, Arg: "s3"}) headless colon-command
//     equivalent, plus ctrl.ApplyResourcesLoaded test seam.
package unit

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// ────────────────────────────────────────────────────────────────────────────
// Shared fixture: page1 (50, truncated, token "p2") / page2 (5, exact, 55
// total) registered against the REAL "s3" short name (not a synthetic type)
// so the fixture exercises the exact live-observed code path (`:s3`).
// SetPaginatedForTest overrides the catalog's real s3 fetcher for the
// duration of the test (GetPaginatedFetcher is legacy-first); t.Cleanup
// restores the catalog fallback.
// ────────────────────────────────────────────────────────────────────────────

func registerDef17S3Fetcher(t *testing.T) {
	t.Helper()
	resource.SetPaginatedForTest("s3", func(_ context.Context, _ any, token string) (resource.FetchResult, error) {
		switch token {
		case "":
			return resource.FetchResult{
				Resources: def17Page1Resources(50),
				Pagination: &resource.PaginationMeta{
					IsTruncated: true,
					NextToken:   "p2",
					TotalHint:   -1,
					PageSize:    50,
				},
			}, nil
		case "p2":
			return resource.FetchResult{
				Resources: def17Page2Resources(50, 5),
				Pagination: &resource.PaginationMeta{
					IsTruncated: false,
					NextToken:   "",
					TotalHint:   55,
					PageSize:    5,
				},
			}, nil
		default:
			return resource.FetchResult{}, nil
		}
	})
	t.Cleanup(func() { resource.CleanupPaginatedForTest("s3") })
}

func def17BucketID(i int) string {
	const digits = "0123456789"
	n := i + 1
	s := ""
	for n > 0 {
		s = string(digits[n%10]) + s
		n /= 10
	}
	for len(s) < 3 {
		s = "0" + s
	}
	return "arn:aws:s3:::def17-bucket-" + s
}

func def17Page1Resources(n int) []resource.Resource {
	out := make([]resource.Resource, n)
	for i := range out {
		out[i] = resource.Resource{ID: def17BucketID(i), Name: def17BucketID(i), Type: "s3"}
	}
	return out
}

func def17Page2Resources(offset, n int) []resource.Resource {
	out := make([]resource.Resource, n)
	for i := range out {
		out[i] = resource.Resource{ID: def17BucketID(offset + i), Name: def17BucketID(offset + i), Type: "s3"}
	}
	return out
}

// ────────────────────────────────────────────────────────────────────────────
// Test 1 — real TUI Update loop, real 'm' keypress.
// ────────────────────────────────────────────────────────────────────────────

// TestLoadMore_TUI_ColdOpen_NoDuplicates drives the LIVE sequence: a fresh
// pair, on-disk caching enabled (newPostSweepApp mirrors the real ./a9s
// wiring, not a NoCache test double), opened via the colon-command lane
// (":" + "s3" + Enter — the live pilot symptom occurred via `:s3`, not menu
// Enter), then a real 'm' keypress routed through
// internal/tui/views/resourcelist.go's key.Matches(msg, m.keys.LoadMore)
// branch and internal/tui/fetch_adapter.go's fetchMoreResources.
//
// RED today (per the live observation): pressing 'm' after the cold open
// re-fetches page 1 instead of page 2 (or double-appends it), so the
// rendered view shows a duplicate consecutive row ID and the title reflects
// ~100 rows instead of 55.
func TestLoadMore_TUI_ColdOpen_NoDuplicates(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	registerDef17S3Fetcher(t)
	const profile, region = "def17-tui-profile", "us-east-1"

	m := tui.New(profile, region,
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithProfile(profile),
		tui.WithRegion(region))
	// Height is tall enough to render all 55 rows without viewport
	// scrolling — the assertions below check row-ID occurrences and the
	// frame title text, both of which must be visible in the rendered
	// output for the checks to be meaningful (a truncated viewport would
	// otherwise produce a false RED from scrolled-off rows, not the
	// DEF-17 defect itself).
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 70})

	// Drive Init() so the pre-supplied demo clients actually land on
	// m.core (via messages.ClientsReady) — without this, m.core.Clients()
	// stays nil and the load-more fetch fails with "AWS clients not
	// initialized" instead of exercising the real fetcher.
	initCmd := m.Init()
	if initCmd == nil {
		t.Fatal("Init() should return a command producing ClientsReady")
	}
	initMsg := initCmd()
	if _, ok := initMsg.(messages.ClientsReady); !ok {
		t.Fatalf("Init() cmd produced %T, want messages.ClientsReady", initMsg)
	}
	m, _ = rootApplyMsg(m, initMsg)

	// Cold open via the colon-command lane: ":" + "s3" + Enter.
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
	m, fetchCmd := rootApplyMsg(m, nav)
	if fetchCmd == nil {
		t.Fatal("opening s3 should return a fetch command")
	}

	// Drain the resulting fetch command tree to land page 1 (mirrors the
	// real async round trip: the tea.Cmd closure runs and its returned
	// tea.Msg is fed back through Update).
	m = def17DrainOneLevel(t, m, fetchCmd)

	content := stripANSI(rootViewContent(m))
	if strings.Contains(content, "Loading...") {
		t.Fatalf("cold s3 open did not settle to rendered rows before pressing 'm':\n%s", content)
	}
	if !strings.Contains(content, def17BucketID(0)) {
		t.Fatalf("cold s3 open did not render page 1's first row %q:\n%s", def17BucketID(0), content)
	}

	// Press 'm' — the real load-more keypress. The key handler
	// (internal/tui/views/resourcelist.go's key.Matches(msg, m.keys.LoadMore)
	// branch) returns a cmd producing messages.LoadMore, which app.go's
	// `case messages.LoadMore:` routes to m.fetchMoreResources — a SECOND
	// cmd hop before the actual ResourcesLoaded lands. Both hops must be
	// drained.
	m, loadMoreCmd := rootApplyMsg(m, rootKeyPress("m"))
	if loadMoreCmd == nil {
		t.Fatal("pressing 'm' on a truncated cold-opened list should return a command")
	}
	loadMoreMsg := loadMoreCmd()
	if _, ok := loadMoreMsg.(messages.LoadMore); !ok {
		t.Fatalf("pressing 'm' should produce messages.LoadMore, got %T", loadMoreMsg)
	}
	m, fetchMoreCmd := rootApplyMsg(m, loadMoreMsg)
	if fetchMoreCmd == nil {
		t.Fatal("messages.LoadMore should return a fetch-more command")
	}
	m = def17DrainOneLevel(t, m, fetchMoreCmd)

	content = stripANSI(rootViewContent(m))

	// Bug-catching: a naive re-fetch-page-1 or double-append would still
	// pass a bare substring-presence check, so count row-ID occurrences.
	firstID := def17BucketID(0)
	occurrences := strings.Count(content, firstID)
	if occurrences > 1 {
		t.Errorf("row ID %q appears %d times in the rendered list after 'm' — DEF-17: load-more duplicated page 1 instead of fetching page 2:\n%s", firstID, occurrences, content)
	}
	if !strings.Contains(content, def17BucketID(54)) {
		t.Errorf("rendered list after 'm' is missing page 2's last row %q — page 2 was never actually fetched:\n%s", def17BucketID(54), content)
	}
	if !strings.Contains(content, "s3(55)") {
		t.Errorf("rendered frame title after 'm' does not contain \"s3(55)\" — DEF-17: the title must reflect exactly 55 total rows (50 + 5), not a duplicated ~100:\n%s", content)
	}
	totalOccurrences := 0
	for i := range 55 {
		totalOccurrences += strings.Count(content, def17BucketID(i))
	}
	if totalOccurrences != 55 {
		t.Errorf("rendered list after 'm' contains %d total row-ID occurrences across the 55 distinct IDs, want exactly 55 (no duplicates) — DEF-17:\n%s", totalOccurrences, content)
	}
}

// def17DrainOneLevel runs cmd and feeds its resulting tea.Msg back through
// Update exactly once (per feedback_dont_recurse_past_tea_tick.md: a single
// level is sufficient here since fetchResources/fetchMoreResources each
// resolve to one ResourcesLoaded message, not a cascade of tea.Tick-based
// follow-ups).
func def17DrainOneLevel(t *testing.T, m tui.Model, cmd tea.Cmd) tui.Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			if c == nil {
				continue
			}
			bm := c()
			if bm != nil {
				m, _ = rootApplyMsg(m, bm)
			}
		}
		return m
	}
	m, _ = rootApplyMsg(m, msg)
	return m
}

// ────────────────────────────────────────────────────────────────────────────
// Test 2 — pagination cursor at the controller getter the 'm' path reads.
// ────────────────────────────────────────────────────────────────────────────

// TestLoadMore_TokenPresent_AfterColdOpen pins Controller.GetListPaginationCursor
// (internal/app/list_state.go) — the exact accessor
// internal/tui/views/resourcelist.go's LoadMore branch calls
// (m.ctrl.GetListPaginationCursor()) to build messages.LoadMore.ContinuationToken.
// After a cold s3 open lands page 1 (NextToken="p2"), the cursor must equal
// the fetcher-returned token. RED if the token got lost or overwritten
// (suspected mechanism: the second 'm' press re-requesting token "" instead
// of "p2" would explain the observed full-page-1 duplicate).
func TestLoadMore_TokenPresent_AfterColdOpen(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	s := session.New()
	s.Profile = "def17-cursor-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := app.New(core)
	t.Cleanup(ctrl.Close)

	// Cold open via the -c/ActionCommand lane (same handler as the
	// colon-command lane per handleActionCommand's ":s3"-equivalent switch).
	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	// Land page 1 exactly as production's fetchResources -> ResourcesLoaded
	// round trip would, via the public test seam.
	ctrl.ApplyResourcesLoaded("s3", def17Page1Resources(50), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "p2",
		TotalHint:   -1,
		PageSize:    50,
	}, false)

	got := ctrl.GetListPaginationCursor()
	if got != "p2" {
		t.Errorf("GetListPaginationCursor() = %q, want %q — DEF-17: the 'm' key path reads this exact accessor to build messages.LoadMore.ContinuationToken; a lost/blank token here would cause the next load-more fetch to re-request token \"\" (page 1 again) instead of \"p2\"", got, "p2")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 3 — headless append-dedup backstop.
// ────────────────────────────────────────────────────────────────────────────

// TestLoadMore_AppendDedup_Backstop pins the coder's ID-dedup backstop:
// Controller.ApplyResourcesLoaded(append=true) with rows whose IDs already
// exist on the screen must not duplicate them. Drives the exact poisoning
// shape observed live — page 1 landing, then an append call that (as if by
// the DEF-17 bug) resends page 1's own rows instead of page 2's.
//
// RED today: internal/app/list_body.go's applyResourcesLoaded appends
// unconditionally (`ls.Rows = append(ls.Rows, resources...)`) with no ID
// dedup, so a poisoned append lands 100 rows (50 originals + 50 duplicates)
// instead of the intended distinct set.
func TestLoadMore_AppendDedup_Backstop(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	s := session.New()
	s.Profile = "def17-dedup-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := app.New(core)
	t.Cleanup(ctrl.Close)

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	page1 := def17Page1Resources(50)
	ctrl.ApplyResourcesLoaded("s3", page1, &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "p2",
		TotalHint:   -1,
		PageSize:    50,
	}, false)

	// Poisoning sequence: an append landing page 1's own rows again (the
	// exact DEF-17 symptom shape), instead of page 2's distinct rows.
	ctrl.ApplyResourcesLoaded("s3", page1, &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "p2",
		TotalHint:   -1,
		PageSize:    50,
	}, true)

	snap := ctrl.Snapshot()
	lb := snap.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after append-dedup drive")
	}
	seen := map[string]int{}
	for _, r := range lb.Rows {
		seen[r.ResourceID]++
	}
	dupCount := 0
	for id, n := range seen {
		if n > 1 {
			dupCount++
			t.Errorf("row ID %q appears %d times after an append carrying already-present IDs — DEF-17: ApplyResourcesLoaded(append=true) must dedup by ID, not blindly concatenate", id, n)
		}
	}
	if dupCount == 0 && len(lb.Rows) != 50 {
		t.Errorf("len(Body.List.Rows) = %d, want 50 (dedup backstop should have discarded all 50 already-present IDs from the poisoned append)", len(lb.Rows))
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 4 — persisted TypeFile must never end up with a mismatched pair.
// ────────────────────────────────────────────────────────────────────────────

// TestLoadMore_PersistedPair_NeverMismatched drives the full flow (cold open
// -> poisoned double-append -> save) through the real production wiring
// (Controller.applyResourcesLoaded -> syncExactTotalToMenu ->
// maybeSaveResourceListCache -> Core.SaveResourceListCache) and asserts the
// on-disk TypeFile for s3 stays internally consistent.
//
// Note on the DEF-4b matched-pair rule cited in the dispatch: Count and Rows
// are ALWAYS written as len(ls.Rows)/ls.Rows together (maybeSaveResourceListCache,
// internal/app/handle.go), so Count==len(Rows) holds by construction even
// when ls.Rows itself has been poisoned with duplicate IDs — a bare
// Count-vs-len(Rows) check can never catch that shape of corruption. The
// live-observed "count:100 with 50 rows" symptom must instead mean 100
// PERSISTED rows of which only 50 are distinct IDs — i.e. the persisted
// Rows slice itself contains duplicate IDs. That is what this pin checks.
func TestLoadMore_PersistedPair_NeverMismatched(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)
	const profile, region = "def17-pair-profile", "us-east-1"

	s := session.New()
	s.Profile = profile
	s.Region = region
	core := runtime.New(s, resource.AllResourceTypes())
	ctrl := app.New(core)
	t.Cleanup(ctrl.Close)

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})

	page1 := def17Page1Resources(50)
	ctrl.ApplyResourcesLoaded("s3", page1, &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "p2",
		TotalHint:   -1,
		PageSize:    50,
	}, false)

	// Poisoning sequence: the DEF-17 symptom shape — an append that lands
	// page 1's rows again instead of page 2's distinct rows, with a
	// pagination result claiming exhaustion (title 50+ -> 100).
	ctrl.ApplyResourcesLoaded("s3", page1, &resource.PaginationMeta{
		IsTruncated: false,
		NextToken:   "",
		TotalHint:   50,
		PageSize:    50,
	}, true)

	store := cache.LoadDir(profile, region)
	tf, ok := store.Type("s3")
	if !ok {
		t.Fatal(`store.Type("s3") missing after the poisoning sequence — cannot validate the persisted-pair invariant`)
	}
	if tf.Exact && tf.Count != len(tf.Rows) {
		t.Errorf("persisted s3 TypeFile: Exact=true but Count=%d != len(Rows)=%d — DEF-4b violation: a double-append must never leave the persisted pair mismatched", tf.Count, len(tf.Rows))
	}
	if tf.Count > len(tf.Rows) {
		t.Errorf("persisted s3 TypeFile: Count=%d > len(Rows)=%d — the on-disk pair claims more rows than are actually stored (the live-observed \"count:100 with 50 rows\" symptom)", tf.Count, len(tf.Rows))
	}

	seen := map[string]int{}
	for _, r := range tf.Rows {
		seen[r.ID]++
	}
	distinct := len(seen)
	if distinct != len(tf.Rows) {
		t.Errorf("persisted s3 TypeFile.Rows has %d entries but only %d distinct IDs — DEF-17: the persisted rows contain duplicates (the live-observed \"count:100 with 50 [distinct] rows\" symptom: Count/len(Rows) matched at %d, but only %d of those rows are actually unique buckets)", len(tf.Rows), distinct, tf.Count, distinct)
	}
}
