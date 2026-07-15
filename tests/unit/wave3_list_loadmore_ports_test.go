// wave3_list_loadmore_ports_test.go — Wave 3 (022-codebase-cleanup) PORT pins
// for the ActionLoadMore no-op/debounce/error-recovery mechanism and the
// sort/cursor stability of a plain (non-checker) load-more append, ported
// from the doomed legacy qa_pagination_stories_test.go funcs onto the LIVE
// Controller seam (internal/app/actions_list.go handleActionLoadMore,
// internal/app/list_state.go ClearListLoading, internal/app/list_body.go
// buildListBody's per-render sort/select-clamp) so those legacy funcs can be
// deleted without losing coverage:
//
//   - TestStoryH1_DemoMode_PaginationForLargeTypes / _ChildViews_Pagination
//     (M-key no-op when not truncated, produces a cmd when truncated) and
//     TestStoryI1_EmptyLoadMore_MBecomesNoop / TestStoryI2_RapidMPresses_Debounced:
//     internal/app/headless_regression_test.go's TestActionLoadMore_* funcs
//     only cover the "truncated -> produces a KindFetchMore task" branch
//     (payload shape). Nothing exercises the "!HasPagination -> nil tasks"
//     or "already LoadingMore -> nil tasks (debounce)" guard branches.
//   - TestStoryI4_LoadMoreAfterSort_PreservesSortOrder: internal/app/list_test.go's
//     TestListSort_* never combines an active sort with an append; the only
//     append+sort coverage anywhere (wave3_list_ports_test.go's
//     RelatedCheckerCarry_PreservesSortAfterMerge) drives the reapplyCheckerAgainst
//     merge path, not a plain non-checker ActionLoadMore append.
//   - TestStoryI5_LoadMoreAtBottom_CursorStays: nothing pins that ls.SelectedRow
//     is left untouched by an append (buildListBody clamps it but does not
//     reset it) when the cursor sits at the pre-append last row.
//   - TestStoryE4_ErrorDuringLoadMore_PreservesData / _AllResourceTypes: these
//     drove the DEAD views.ResourceListModel.ClearLoading() (zero production
//     callers). The live equivalent is Controller.ClearListLoading(), and
//     nothing pins that it preserves rows/pagination and clears LoadingMore
//     so a retry ActionLoadMore produces a task again.
//   - TestStory_LoadMoreIndicator_* (5 funcs) and qa_pagination_hint_test.go:
//     these all drove the DEAD views.ResourceListModel.View()/Update() legacy
//     harness. The hint text ("m: load more" / filter-aware variant /
//     "loading...") is real production behavior — internal/tui/views/
//     resourcelist.go's RenderList (LIVE, called from internal/tui/renderer.go's
//     renderList) reads it straight off body.Truncated/body.Filter/
//     body.LoadingMore — but nothing anywhere drives RenderList itself (the
//     live rendering entry point) with a truncated/loading/filtered ListBody
//     and asserts on the hint text. Ported directly against RenderList via
//     views.NewTransientResourceList, mirroring renderer.go's own call shape.
package unit_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// wave3LoadMoreResources returns n synthetic EC2 resources named so that
// reverse-alpha insertion order (highest suffix first) differs from both
// insertion order and ascending-name sort order — the same shape the ported
// legacy I.4/I.5 stories used.
func wave3LoadMoreResources(n int, idOffset int) []resource.Resource {
	out := make([]resource.Resource, n)
	for i := range n {
		id := fmt.Sprintf("i-%05d", idOffset+i)
		name := fmt.Sprintf("z-instance-%05d", idOffset+n-1-i)
		out[i] = resource.Resource{
			ID: id, Name: name, Type: "ec2",
			Fields: map[string]string{"instance_id": id, "name": name, "state": "running"},
		}
	}
	return out
}

// ===========================================================================
// ActionLoadMore no-op / debounce guard branches
// ===========================================================================

func TestWave3ActionLoadMore_NotTruncated_Noop(t *testing.T) {
	c := wave3ListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(3, 0), &resource.PaginationMeta{IsTruncated: false}, false)

	_, tasks := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(tasks) != 0 {
		t.Errorf("ActionLoadMore on a non-truncated list must be a no-op, got %d tasks", len(tasks))
	}
}

func TestWave3ActionLoadMore_AlreadyLoading_Debounced(t *testing.T) {
	c := wave3ListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(200, 0), &resource.PaginationMeta{
		IsTruncated: true, NextToken: "tok",
	}, false)

	_, first := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(first) == 0 {
		t.Fatal("precondition: first ActionLoadMore on a truncated list must produce a task")
	}

	// Rapid repeats before the fetch result lands: ls.LoadingMore is now true,
	// so every subsequent press must be inert.
	for i := range 5 {
		_, tasks := c.Apply(app.Action{Kind: app.ActionLoadMore})
		if len(tasks) != 0 {
			t.Errorf("ActionLoadMore press %d while already loading must be a no-op, got %d tasks", i+2, len(tasks))
		}
	}
}

// ===========================================================================
// Error during load-more: Controller.ClearListLoading() preserves data and
// re-arms ActionLoadMore.
// ===========================================================================

func TestWave3ActionLoadMore_ErrorClearsLoading_PreservesRowsAndAllowsRetry(t *testing.T) {
	c := wave3ListController(t, "ec2")
	seed := wave3LoadMoreResources(200, 0)
	c.ApplyResourcesLoaded("ec2", seed, &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-p2"}, false)

	_, tasks := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(tasks) == 0 {
		t.Fatal("precondition: ActionLoadMore must produce a task before the simulated error")
	}

	// Simulate the app-level error handler: a load-more fetch failed.
	c.ClearListLoading()

	lb := c.Snapshot().Body.List
	if lb == nil || len(lb.Rows) != len(seed) {
		got := 0
		if lb != nil {
			got = len(lb.Rows)
		}
		t.Fatalf("ClearListLoading must preserve existing rows: got %d rows, want %d", got, len(seed))
	}
	if !lb.Truncated {
		t.Error("ClearListLoading must preserve pagination (Truncated) state, got false")
	}

	// LoadingMore must be cleared, or ActionLoadMore stays debounced forever.
	_, retryTasks := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(retryTasks) == 0 {
		t.Error("after ClearListLoading, ActionLoadMore must produce a task again (retry), got none")
	}
}

// ===========================================================================
// Sort and cursor stability across a plain (non-checker) load-more append.
// ===========================================================================

func TestWave3ListSort_PreservedAfterLoadMoreAppend(t *testing.T) {
	c := wave3ListController(t, "ec2")

	// Page 1: 200 items named in reverse-alpha order.
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(200, 0), &resource.PaginationMeta{
		IsTruncated: true, NextToken: "tok-p2",
	}, false)

	c.Apply(app.Action{Kind: app.ActionSort, Arg: "name"})

	lb := c.Snapshot().Body.List
	if lb == nil || len(lb.Rows) == 0 {
		t.Fatal("expected rows after sort")
	}
	if lb.Rows[0].Cells[0] != "z-instance-00000" {
		t.Fatalf("precondition: after asc sort, first row should be %q, got %q", "z-instance-00000", lb.Rows[0].Cells[0])
	}

	// Page 2: 100 more items whose names ("a-instance-...") sort BEFORE every
	// page-1 name ("z-instance-..."). If sort were not re-applied on top of
	// the full merged Rows set, the append would land these after page 1.
	page2 := make([]resource.Resource, 100)
	for i := range 100 {
		id := fmt.Sprintf("i-%05d", 200+i)
		name := fmt.Sprintf("a-instance-%05d", i)
		page2[i] = resource.Resource{
			ID: id, Name: name, Type: "ec2",
			Fields: map[string]string{"instance_id": id, "name": name, "state": "running"},
		}
	}
	c.ApplyResourcesLoaded("ec2", page2, &resource.PaginationMeta{IsTruncated: false}, true)

	lb = c.Snapshot().Body.List
	if lb == nil || len(lb.Rows) != 300 {
		got := 0
		if lb != nil {
			got = len(lb.Rows)
		}
		t.Fatalf("expected 300 rows after append, got %d", got)
	}
	if lb.Rows[0].Cells[0] != "a-instance-00000" {
		t.Errorf("sort not re-applied after append: first row = %q, want %q", lb.Rows[0].Cells[0], "a-instance-00000")
	}
}

func TestWave3ListCursor_StableAfterLoadMoreAppend(t *testing.T) {
	c := wave3ListController(t, "ec2")

	seed := wave3LoadMoreResources(200, 0)
	c.ApplyResourcesLoaded("ec2", seed, &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-p2"}, false)

	// No sort active: move the cursor to the last row.
	c.Apply(app.Action{Kind: app.ActionMoveBottom})
	if got := c.GetListSelectedRow(); got != 199 {
		t.Fatalf("precondition: SelectedRow should be 199, got %d", got)
	}
	preAppend, ok := c.ListSelected()
	if !ok {
		t.Fatal("precondition: expected a selected resource before append")
	}

	page2 := make([]resource.Resource, 100)
	for i := range 100 {
		id := fmt.Sprintf("i-%05d", 200+i)
		name := fmt.Sprintf("instance-%05d", 200+i)
		page2[i] = resource.Resource{
			ID: id, Name: name, Type: "ec2",
			Fields: map[string]string{"instance_id": id, "name": name, "state": "running"},
		}
	}
	c.ApplyResourcesLoaded("ec2", page2, &resource.PaginationMeta{IsTruncated: false}, true)

	if got := c.GetListSelectedRow(); got != 199 {
		t.Errorf("SelectedRow must be untouched by an append; got %d, want 199", got)
	}
	postAppend, ok := c.ListSelected()
	if !ok {
		t.Fatal("expected a selected resource after append")
	}
	if postAppend.ID != preAppend.ID {
		t.Errorf("cursor moved to a different resource after append: got %q, want %q (unchanged)", postAppend.ID, preAppend.ID)
	}

	// The newly appended rows must be reachable by moving down from the
	// preserved cursor position.
	c.Apply(app.Action{Kind: app.ActionMoveDown})
	next, ok := c.ListSelected()
	if !ok || next.ID != "i-00200" {
		gotID := "<none>"
		if ok {
			gotID = next.ID
		}
		t.Errorf("after moving down from the preserved cursor, expected the first appended row %q, got %q", "i-00200", gotID)
	}
}

// ===========================================================================
// RenderList "load more" hint text — driven through the LIVE render seam
// (views.NewTransientResourceList + RenderList, mirroring internal/tui/
// renderer.go's renderList free function) rather than the dead View()/
// Update() legacy harness.
// ===========================================================================

// wave3RenderListBody renders the top list screen exactly as production's
// renderList() free function does: a zero-lifetime ResourceListModel built
// from the Controller's own ListBody snapshot.
func wave3RenderListBody(t *testing.T, c *app.Controller, shortName string) string {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("unknown resource type %q", shortName)
	}
	lb := c.Snapshot().Body.List
	if lb == nil {
		t.Fatal("expected a non-nil list body")
	}
	m := views.NewTransientResourceList(*td, 120, 30)
	return m.RenderList(*lb)
}

func TestWave3RenderList_LoadMoreHint_ShownWhenTruncated(t *testing.T) {
	c := wave3ListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(5, 0), &resource.PaginationMeta{
		IsTruncated: true, NextToken: "tok",
	}, false)

	out := wave3RenderListBody(t, c, "ec2")
	if !strings.Contains(out, "m: load more") {
		t.Errorf("RenderList for a truncated list must show the 'm: load more' hint, got:\n%s", out)
	}
}

func TestWave3RenderList_LoadMoreHint_FilterAwareVariant(t *testing.T) {
	c := wave3ListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(5, 0), &resource.PaginationMeta{
		IsTruncated: true, NextToken: "tok",
	}, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "z-instance"})

	out := wave3RenderListBody(t, c, "ec2")
	if !strings.Contains(out, "m: load more (filter applies to loaded data only)") {
		t.Errorf("RenderList with an active filter must show the filter-aware hint, got:\n%s", out)
	}
}

func TestWave3RenderList_LoadMoreHint_HiddenWhenNotTruncated(t *testing.T) {
	c := wave3ListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(5, 0), &resource.PaginationMeta{
		IsTruncated: false,
	}, false)

	out := wave3RenderListBody(t, c, "ec2")
	if strings.Contains(out, "load more") {
		t.Errorf("RenderList for a non-truncated list must NOT show the load-more hint, got:\n%s", out)
	}
}

func TestWave3RenderList_LoadMoreHint_ShowsLoadingWhileInFlight(t *testing.T) {
	c := wave3ListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(5, 0), &resource.PaginationMeta{
		IsTruncated: true, NextToken: "tok",
	}, false)
	c.Apply(app.Action{Kind: app.ActionLoadMore})

	out := wave3RenderListBody(t, c, "ec2")
	if !strings.Contains(out, "loading...") {
		t.Errorf("RenderList while a load-more is in flight must show 'loading...', got:\n%s", out)
	}
	if strings.Contains(out, "m: load more") {
		t.Errorf("RenderList while loading must NOT show the idle 'm: load more' hint, got:\n%s", out)
	}
}

func TestWave3RenderList_LoadMoreHint_HiddenAfterAllPagesLoaded(t *testing.T) {
	c := wave3ListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(5, 0), &resource.PaginationMeta{
		IsTruncated: true, NextToken: "tok",
	}, false)
	c.Apply(app.Action{Kind: app.ActionLoadMore})
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(3, 5), &resource.PaginationMeta{
		IsTruncated: false,
	}, true)

	out := wave3RenderListBody(t, c, "ec2")
	if strings.Contains(out, "load more") {
		t.Errorf("RenderList after all pages loaded must NOT show the load-more hint, got:\n%s", out)
	}
}
