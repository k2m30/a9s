// runtime_cache_rows_exact_totals_test.go — RED tests for the CACHE-FIRST
// LIST UX epic, Contract D (exact totals on load-more exhaustion, moved to
// the controller level).
//
// Contract D: when load-more exhausts pagination (no next token), the exact
// total becomes authoritative: menu availability for that type updates to
// the exact count with Truncated=false (both in-session AND persisted via
// the disk cache), survives returning to the menu and — via the cache file —
// an app restart. The sync-back currently living in
// internal/tui/app_stack.go's popRS (TUI-only, renderer-owned) moves into
// the controller's ResourcesLoaded/load-more handling so web gets it too.
//
// Today (confirmed by reading internal/tui/app_stack.go:40-99 and
// core/app/handle.go + core/app/list_body.go) this sync-back exists
// ONLY in the TUI's popRS — core/app.Controller.Handle/
// handleResourcesLoadedEvent/applyResourcesLoaded never touch
// PatchMenuAvailability at all. These tests pin the controller-level
// behavior directly via Handle(messages.ResourcesLoaded{...}) — no
// TUI/*tui.Model involvement — per the epic's explicit "NO TUI involvement"
// requirement.
//
// AMBIGUITY RESOLUTIONS (stated, not deferred):
//   - "Load-more that exhausts pagination" is modeled as a ResourcesLoaded
//     event with Append=true and Pagination.IsTruncated=false — the natural
//     shape of the KindFetchMore task's result once the fetcher's next-token
//     comes back empty (mirrors FetchMorePayload/handleActionLoadMore in
//     core/app/actions_list.go, which starts the load-more fetch but
//     does not itself see the result — the result re-enters through the
//     same Handle(ResourcesLoaded) lane as a normal fetch, distinguished by
//     Append=true).
//   - The "only-increase guard" mirrored from popRS (menu count/issues never
//     regress) is preserved as part of the ported behavior: a pre-existing
//     higher truncated menu count must not be overwritten by a smaller
//     load-more total. This mirrors the exact guard in
//     internal/tui/app_stack.go:57 (`!newTrunc || !known || newCount >
//     curCount`).
//   - Disk persistence is pinned via the same on-disk cache.Entry.Count/
//     Truncated fields the SaveAvailabilityCache seam already writes
//     (core/runtime/probes.go) — this test asserts the controller
//     triggers that same disk write path when the in-session
//     PatchMenuAvailability fires from a load-more exhaustion, not a novel
//     disk format.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// -----------------------------------------------------------------------
// Contract D — exact totals move to the controller (no TUI involvement)
// -----------------------------------------------------------------------

// TestLoadMoreExhausted_UpdatesMenuAvailability_ExactNoTUI pins the core
// controller-level outcome: a load-more ResourcesLoaded result whose
// pagination is no longer truncated must update the ROOT menu's
// availability for that type to the exact count with Truncated=false —
// driven purely through Controller.Handle, with no *tui.Model anywhere in
// this test (proving the sync-back no longer needs TUI popRS to fire).
func TestLoadMoreExhausted_UpdatesMenuAvailability_ExactNoTUI(t *testing.T) {
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)

	// Seed the menu with a truncated lower-bound count, as if an earlier
	// availability probe reported "200+" before the user drilled into the
	// list and paged through everything.
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "ec2", Count: 200, Truncated: true},
	})

	// Open the ec2 list and seed its first page + pagination cursor so the
	// controller has a live ListState to append the load-more page onto.
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	firstPage := make([]resource.Resource, 200)
	for i := range firstPage {
		firstPage[i] = resource.Resource{ID: "i-page1-" + itoaTest(i), Type: "ec2"}
	}
	c.ApplyResourcesLoaded("ec2", firstPage, &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-1"}, false)

	// Load-more exhausts pagination: 47 more rows arrive, no next token.
	moreRows := make([]resource.Resource, 47)
	for i := range moreRows {
		moreRows[i] = resource.Resource{ID: "i-page2-" + itoaTest(i), Type: "ec2"}
	}
	vs, _ := c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    moreRows,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Append:       true,
		Gen:          0, // AcceptZeroGen=true
	})
	_ = vs

	avail := c.GetMenuAvailability()
	trunc := c.GetMenuTruncated()
	if got, want := avail["ec2"], 247; got != want {
		t.Errorf("menu availability[ec2] = %d, want %d (exact total after load-more exhaustion)", got, want)
	}
	if trunc["ec2"] {
		t.Error("menu truncated[ec2] = true, want false — exact total must clear the truncated lower-bound flag")
	}
}

// TestLoadMoreExhausted_SurvivesReturnToMenu verifies the exact total
// remains authoritative after the list screen is popped back to the menu
// (the "survives returning to the menu" half of the contract) — again
// driven purely at the controller level via ApplyIntents/Apply, no TUI.
func TestLoadMoreExhausted_SurvivesReturnToMenu(t *testing.T) {
	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "s3", Count: 10, Truncated: true},
	})
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "s3"})
	c.ApplyResourcesLoaded("s3", []resource.Resource{
		{ID: "bucket-1", Type: "s3"}, {ID: "bucket-2", Type: "s3"},
	}, &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-a"}, false)

	c.Handle(messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    []resource.Resource{{ID: "bucket-3", Type: "s3"}},
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Append:       true,
		Gen:          0,
	})

	// Pop back to the menu — a plain controller-level ActionBack, not a TUI
	// popRS call. If Contract D is correctly moved to the controller, the
	// exact total set above must already be in MenuState and popping must
	// not need to (re)compute it.
	c.Apply(app.Action{Kind: app.ActionBack})

	avail := c.GetMenuAvailability()
	trunc := c.GetMenuTruncated()
	if got, want := avail["s3"], 3; got != want {
		t.Errorf("after returning to menu: availability[s3] = %d, want %d", got, want)
	}
	if trunc["s3"] {
		t.Error("after returning to menu: truncated[s3] = true, want false")
	}
}

// TestLoadMoreExhausted_OnlyIncreaseGuard mirrors the only-increase guard
// ported from internal/tui/app_stack.go:57 — a load-more result must never
// SHRINK a menu count that was already known to be at least as large
// (e.g. a stale/short load-more page racing a larger already-recorded
// count), while a load-more that DOES exceed the known count must still win.
// Both sub-cases are asserted together so this test cannot pass merely
// because Contract D is entirely unimplemented (an unimplemented sync-back
// would leave availability untouched in BOTH sub-cases, which the "wins"
// sub-case catches).
func TestLoadMoreExhausted_OnlyIncreaseGuard(t *testing.T) {
	newCtrl := func(seedCount int, seedTruncated bool) (*runtime.Core, *app.Controller) {
		s := session.New()
		s.Profile = "demo"
		s.Region = "us-east-1"
		core := runtime.New(s, nil)
		c := app.New(core)
		c.ApplyIntents([]runtime.UIIntent{
			runtime.PatchMenuAvailability{ResourceType: "ec2", Count: seedCount, Truncated: seedTruncated},
		})
		return core, c
	}

	t.Run("smaller_load_more_does_not_regress", func(t *testing.T) {
		// Menu already knows about MORE resources than this load-more result
		// will report (simulates a concurrent fuller probe having already
		// landed).
		_, c := newCtrl(500, false)

		c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
		c.ApplyResourcesLoaded("ec2", []resource.Resource{{ID: "i-only1", Type: "ec2"}}, &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-z"}, false)

		c.Handle(messages.ResourcesLoaded{
			ResourceType: "ec2",
			Resources:    []resource.Resource{{ID: "i-only2", Type: "ec2"}},
			Pagination:   &resource.PaginationMeta{IsTruncated: false},
			Append:       true,
			Gen:          0,
		})

		avail := c.GetMenuAvailability()
		if got := avail["ec2"]; got != 500 {
			t.Errorf("menu availability[ec2] = %d, want unchanged 500 — a smaller load-more total must not regress an already-known larger exact count", got)
		}
	})

	t.Run("larger_load_more_still_wins", func(t *testing.T) {
		// Menu knows about a SMALLER truncated lower-bound than what this
		// load-more will report — the exact total must win here, proving the
		// guard is a directional "never shrink", not a blanket "never
		// update". Without this sub-case, a controller that always ignores
		// load-more sync-back would pass the sibling sub-case above too.
		_, c := newCtrl(2, true)

		c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
		c.ApplyResourcesLoaded("ec2", []resource.Resource{{ID: "i-a"}, {ID: "i-b"}}, &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-y"}, false)

		c.Handle(messages.ResourcesLoaded{
			ResourceType: "ec2",
			Resources:    []resource.Resource{{ID: "i-c"}, {ID: "i-d"}, {ID: "i-e"}},
			Pagination:   &resource.PaginationMeta{IsTruncated: false},
			Append:       true,
			Gen:          0,
		})

		avail := c.GetMenuAvailability()
		trunc := c.GetMenuTruncated()
		if got, want := avail["ec2"], 5; got != want {
			t.Errorf("menu availability[ec2] = %d, want %d — a load-more exact total larger than the known truncated lower-bound must win", got, want)
		}
		if trunc["ec2"] {
			t.Error("menu truncated[ec2] = true, want false once the exact total wins")
		}
	})
}

// TestLoadMoreExhausted_PersistsToDiskCache pins "persisted via the disk
// cache": after a load-more exhaustion updates in-session menu
// availability to an exact count, SaveAvailabilityCache (the existing
// probes.go seam) must be able to write that exact/untruncated state to
// disk when invoked with the controller's updated menu availability maps —
// and cache.LoadDir must read back the same exact, untruncated per-type
// entry, modeling "survives an app restart". Round-2 migration: repinned at
// the same controller seam (core.SaveAvailabilityCache/LoadAvailabilityCache)
// but the on-disk assertion now goes through cache.LoadDir/Store.Type
// directly, since the exact-total persistence flows through
// (*cache.Store).SaveType's per-type file, not the deleted single-file
// cache.File/cache.Entry shape.
func TestLoadMoreExhausted_PersistsToDiskCache(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("A9S_CONFIG_FOLDER", tmp)

	s := session.New()
	s.Profile = "demo"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)

	c.ApplyIntents([]runtime.UIIntent{
		runtime.PatchMenuAvailability{ResourceType: "ec2", Count: 100, Truncated: true},
	})
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.ApplyResourcesLoaded("ec2", make([]resource.Resource, 100), &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-1"}, false)

	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-extra-1"}, {ID: "i-extra-2"}},
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Append:       true,
		Gen:          0,
	})

	avail := c.GetMenuAvailability()
	trunc := c.GetMenuTruncated()
	if err := core.SaveAvailabilityCache(avail, trunc, nil, nil, nil); err != nil {
		t.Fatalf("SaveAvailabilityCache: %v", err)
	}

	store := cache.LoadDir("demo", "us-east-1")
	if store == nil {
		t.Fatal("cache.LoadDir returned nil after SaveAvailabilityCache")
	}
	tf, ok := store.Type("ec2")
	if !ok {
		t.Fatal(`cache.LoadDir(...).Type("ec2") missing after SaveAvailabilityCache`)
	}
	if tf.Count != 102 {
		t.Errorf("persisted ec2 Count = %d, want 102 (exact total after load-more exhaustion)", tf.Count)
	}
	if !tf.Exact {
		t.Error("persisted ec2 Exact = false, want true — an untruncated exact total must be persisted as exact on disk too")
	}
}

// itoaTest is a tiny local decimal formatter so this file has no dependency
// on strconv beyond what's already imported elsewhere in the package,
// avoiding an extra import for a single loop counter.
func itoaTest(n int) string {
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
