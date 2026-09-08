// list_loadmore_ports_test.go — live-seam port pins for the ActionLoadMore
// no-op/debounce/error-recovery mechanism and the
// sort/cursor stability of a plain (non-checker) load-more append, ported
// from the doomed legacy qa_pagination_stories_test.go funcs onto the LIVE
// Controller seam (core/app/actions_list.go handleActionLoadMore,
// core/app/list_state.go ClearListLoading, core/app/list_body.go
// buildListBody's per-render sort/select-clamp) so those legacy funcs can be
// deleted without losing coverage:
//
//   - TestStoryH1_DemoMode_PaginationForLargeTypes / _ChildViews_Pagination
//     (M-key no-op when not truncated, produces a cmd when truncated) and
//     TestStoryI1_EmptyLoadMore_MBecomesNoop / TestStoryI2_RapidMPresses_Debounced:
//     core/app/headless_regression_test.go's TestActionLoadMore_* funcs
//     only cover the "truncated -> produces a KindFetchMore task" branch
//     (payload shape). Nothing exercises the "!HasPagination -> nil tasks"
//     or "already LoadingMore -> nil tasks (debounce)" guard branches.
//   - TestStoryI4_LoadMoreAfterSort_PreservesSortOrder: core/app/list_test.go's
//     TestListSort_* never combines an active sort with an append; the only
//     append+sort coverage anywhere (list_ports_test.go's
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
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
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

func TestActionLoadMore_NotTruncated_Noop(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(3, 0), &resource.PaginationMeta{IsTruncated: false}, false)

	_, tasks := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(tasks) != 0 {
		t.Errorf("ActionLoadMore on a non-truncated list must be a no-op, got %d tasks", len(tasks))
	}
}

func TestActionLoadMore_AlreadyLoading_Debounced(t *testing.T) {
	c := openListController(t, "ec2")
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

func TestActionLoadMore_ErrorClearsLoading_PreservesRowsAndAllowsRetry(t *testing.T) {
	c := openListController(t, "ec2")
	seed := wave3LoadMoreResources(200, 0)
	c.ApplyResourcesLoaded("ec2", seed, &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-p2"}, false)

	_, tasks := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(tasks) == 0 {
		t.Fatal("precondition: ActionLoadMore must produce a task before the simulated error")
	}

	// Simulate the app-level error handler: a load-more fetch failed. loadMore=true
	// selects the load-more request's own flag (LoadingMore) — this scenario never
	// started a concurrent refresh, so there is nothing else to preserve here.
	c.ClearListLoading(true)

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

func TestListSort_PreservedAfterLoadMoreAppend(t *testing.T) {
	c := openListController(t, "ec2")

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

func TestListCursor_StableAfterLoadMoreAppend(t *testing.T) {
	c := openListController(t, "ec2")

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

func TestRenderList_LoadMoreHint_ShownWhenTruncated(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(5, 0), &resource.PaginationMeta{
		IsTruncated: true, NextToken: "tok",
	}, false)

	out := wave3RenderListBody(t, c, "ec2")
	if !strings.Contains(out, "m: load more") {
		t.Errorf("RenderList for a truncated list must show the 'm: load more' hint, got:\n%s", out)
	}
}

func TestRenderList_LoadMoreHint_FilterAwareVariant(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(5, 0), &resource.PaginationMeta{
		IsTruncated: true, NextToken: "tok",
	}, false)
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "z-instance"})

	out := wave3RenderListBody(t, c, "ec2")
	if !strings.Contains(out, "m: load more (filter applies to loaded data only)") {
		t.Errorf("RenderList with an active filter must show the filter-aware hint, got:\n%s", out)
	}
}

func TestRenderList_LoadMoreHint_HiddenWhenNotTruncated(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(5, 0), &resource.PaginationMeta{
		IsTruncated: false,
	}, false)

	out := wave3RenderListBody(t, c, "ec2")
	if strings.Contains(out, "load more") {
		t.Errorf("RenderList for a non-truncated list must NOT show the load-more hint, got:\n%s", out)
	}
}

func TestRenderList_LoadMoreHint_ShowsLoadingWhileInFlight(t *testing.T) {
	c := openListController(t, "ec2")
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

func TestRenderList_LoadMoreHint_HiddenAfterAllPagesLoaded(t *testing.T) {
	c := openListController(t, "ec2")
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

// ===========================================================================
// ResourcesLoaded-provenance fixes (branch fix/resourcesloaded-provenance,
// wave C): clearFetchInFlight choke point + hadErr-gated LastFetchError clear.
// ===========================================================================

// errLoadMoreAPIFailed is a fixed sentinel used to simulate a failed
// KindFetchMore/KindFetchResources execution delivered as a real
// messages.APIError, mirroring errTUIFetchFailed in
// tui_error_marker_parity_test.go (different package, not importable here).
type errLoadMoreAPIFailed struct{}

func (errLoadMoreAPIFailed) Error() string { return "load-more ports pin: simulated fetch failure" }

// TestActionLoadMore_APIError_ClearsLoadingMoreAndRefreshing_HeadlessLane is a
// BUG CATCH. Before core/app/list_state.go's clearFetchInFlight choke point,
// the headless/web ClearActiveListLoadingIntent case (core/app/intents.go)
// only cleared ls.Loading (and ls.Refreshing conditionally on v.Err != "") —
// it never touched ls.LoadingMore. A failed load-more on the headless/web
// lane left the "── loading... ──" indicator stuck forever with no user
// recovery: handleActionLoadMore's own debounce guard (core/app/actions_list.go)
// is `if ls.LoadingMore { return nil, nil }`, so every subsequent 'm' press
// after the failure was silently swallowed. This is the highest-value pin in
// this file — a permanently stuck UI state, not a transient glitch.
func TestActionLoadMore_APIError_ClearsLoadingMoreAndRefreshing_HeadlessLane(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(50, 0), &resource.PaginationMeta{
		IsTruncated: true, NextToken: "tok-p2",
	}, false)

	_, tasks := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(tasks) == 0 {
		t.Fatal("precondition: ActionLoadMore must produce a task before the simulated failure")
	}
	pre := c.Snapshot().Body.List
	if pre == nil || !pre.LoadingMore {
		t.Fatal("precondition: LoadingMore must be true after ActionLoadMore dispatched a fetch")
	}

	// Simulate the headless/web task-result lane: the in-flight load-more
	// fetch failed, delivered as a real messages.APIError through
	// Controller.Handle — the same event HandleAPIError/
	// ClearActiveListLoadingIntent produce for any failed
	// KindFetchResources/KindFetchMore execution (core/runtime/handlers.go).
	// Append/LoadingMore mirror executor.go's own KindFetchMore-failure
	// construction — this failure is the outcome of the load-more
	// continuation dispatched above, so only LoadingMore may clear.
	// LoadingMore is the field the clear reads: the request records which
	// flag it raised instead of the handler inferring it from Append, which
	// answers how the rows would have merged. Do not drop it back to
	// Append-only — that is the shape the drill-opens-on-a-continuation
	// defect lived in.
	vs, _ := c.Handle(messages.APIError{ResourceType: "ec2", Err: errLoadMoreAPIFailed{}, Append: true, LoadingMore: true})

	got := vs.Body.List
	if got == nil {
		t.Fatal("Body.List is nil after APIError landed")
	}
	if got.LoadingMore {
		t.Error("LoadingMore = true after a failed load-more's APIError landed, want false — the load-more indicator would be stuck forever with ActionLoadMore permanently debounced")
	}
	if got.Loading {
		t.Error("Loading = true after APIError landed, want false")
	}
	if got.Refreshing {
		t.Error("Refreshing = true after APIError landed, want false")
	}

	// Confirm the stuck state is actually gone: a retry must produce a task.
	_, retry := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(retry) == 0 {
		t.Error("after the failure clears LoadingMore, ActionLoadMore must produce a task again (retry) — a stranded LoadingMore=true would debounce this forever")
	}
}

// TestApplyResourcesLoaded_PartialSuccessErr_InstallsCurrentFetchError pins
// the CURRENT contract: per cache contract C4 ("a fetch failure ... swaps
// the marker for an error marker" — docs/design/cache-requirements.md), each
// landed fetch's outcome — success or partial-success — installs ITS OWN
// marker text, never leaving a stale marker from an unrelated earlier
// attempt in place. applyResourcesLoaded's hadErr bool became a fetchErr
// error for exactly this reason: a bare bool could only say "an error
// happened", not carry which one, so the prior implementation could only
// ever leave whatever marker was already there. This test previously
// asserted the opposite — that an existing marker survives a new
// partial-success error unchanged — which would let a retry's genuinely new
// failure hide silently behind stale, possibly unrelated error text forever.
// Pins both halves: the new rows still land, AND the marker reflects the
// failure that just happened.
func TestApplyResourcesLoaded_PartialSuccessErr_InstallsCurrentFetchError(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(2, 0), &resource.PaginationMeta{
		IsTruncated: true, NextToken: "tok",
	}, false)

	const priorErr = "prior fetch failure: throttled"
	c.SetListFetchError(priorErr)
	if got := c.Snapshot().Body.List.LastFetchError; got != priorErr {
		t.Fatalf("precondition: LastFetchError = %q, want %q", got, priorErr)
	}

	partialErr := errors.New("partial: 1 of 3 IDs failed: throttled")
	newRows := wave3LoadMoreResources(3, 100)
	vs, _ := c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    newRows,
		Provenance:   messages.FetchProvenanceCanonicalList,
		Err:          partialErr,
	})

	lb := vs.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after a partial-success ResourcesLoaded landed")
	}
	if len(lb.Rows) != len(newRows) {
		t.Errorf("rows must land despite the partial error: got %d rows, want %d", len(lb.Rows), len(newRows))
	}
	if lb.LastFetchError != partialErr.Error() {
		t.Errorf("LastFetchError = %q after a partial-success (Err-carrying) result landed, want it replaced with the fresh failure %q — a stale unrelated marker must not survive a new fetch's own error", lb.LastFetchError, partialErr.Error())
	}
}

// TestClearListLoading_AlsoClearsRefreshing_ContractLock is a CONTRACT LOCK,
// not a live-bug catch. ClearListLoading's only production caller
// (internal/tui/runtime_adapter.go's ClearActiveListLoadingIntent case)
// always follows this call with SetListFetchError(v.Err), and the intent's
// one construction site (core/runtime/handlers.go's HandleAPIError) never
// leaves text empty — so SetListFetchError already forces Refreshing=false on
// that lane today regardless of what ClearListLoading itself does. This pin
// exists so a FUTURE caller that invokes ClearListLoading without a following
// SetListFetchError (e.g. a bare "stop everything, nothing to report" clear)
// cannot reintroduce a stranded Refreshing marker — not because the
// divergence is live now.
func TestClearListLoading_AlsoClearsRefreshing_ContractLock(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(3, 0), &resource.PaginationMeta{
		IsTruncated: true, NextToken: "tok",
	}, false)
	c.SetListRefreshing(true)

	pre := c.Snapshot().Body.List
	if pre == nil || !pre.Refreshing {
		t.Fatal("precondition: Refreshing must be true before ClearListLoading")
	}

	// loadMore=false: this scenario never started a load-more (only
	// SetListRefreshing(true) above), so the completing request is a plain
	// (non-load-more) fetch — false selects the Loading+Refreshing pair,
	// which is what a real refresh-failure completion would pass.
	c.ClearListLoading(false)

	lb := c.Snapshot().Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after ClearListLoading")
	}
	if lb.Refreshing {
		t.Error("Refreshing = true after ClearListLoading, want false — ClearListLoading must stop every in-flight fetch indicator, not just Loading/LoadingMore")
	}
}

// TestClearListLoading_LoadMoreFailure_LeavesRefreshingSet is a BUG CATCH for
// the exact regression this branch's clearFetchInFlight choke point fixes:
// before the loadMore parameter existed, ANY completion (load-more or
// refresh) cleared all three flags unconditionally, so a failed load-more
// silently cleared an in-flight Ctrl+R refresh that had not itself completed
// — stopping its "refreshing..." indicator and, worse, letting a second
// refresh be dispatched while the first was still genuinely in flight
// (ActionRefresh has no debounce guard of its own the way ActionLoadMore
// does). Drives both actions through the real Apply seam (mirrors
// TestListState_LoadingMoreAndRefreshing_CoexistAndRenderIndependently
// below) so LoadingMore and Refreshing are both genuinely true before the
// simulated load-more failure, then asserts ClearListLoading(true) clears
// only the flag belonging to the request that actually completed.
func TestClearListLoading_LoadMoreFailure_LeavesRefreshingSet(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(5, 0), &resource.PaginationMeta{
		IsTruncated: true, NextToken: "tok",
	}, false)

	_, loadMoreTasks := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(loadMoreTasks) == 0 {
		t.Fatal("precondition: ActionLoadMore must produce a task")
	}
	_, refreshTasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	if len(refreshTasks) == 0 {
		t.Fatal("precondition: ActionRefresh must produce a task even with a load-more in flight")
	}
	pre := c.Snapshot().Body.List
	if pre == nil || !pre.LoadingMore || !pre.Refreshing {
		t.Fatal("precondition: both LoadingMore and Refreshing must be true before the simulated load-more failure")
	}

	// The in-flight load-more's own fetch failed — the completing request is
	// the load-more, not the concurrent Ctrl+R refresh that is still
	// genuinely outstanding.
	c.ClearListLoading(true)

	lb := c.Snapshot().Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after ClearListLoading(true)")
	}
	if lb.LoadingMore {
		t.Error("LoadingMore = true after ClearListLoading(true), want false — the failed load-more's own flag must clear")
	}
	if !lb.Refreshing {
		t.Error("Refreshing = false after ClearListLoading(true), want true — a load-more failure must not clobber a concurrent in-flight refresh that has not itself completed")
	}
}

// TestClearListLoading_RefreshFailure_LeavesLoadingMoreSet is the mirror-image
// BUG CATCH: a failed Ctrl+R refresh must not clear a still-outstanding
// load-more continuation. Before the loadMore parameter existed, this
// direction was just as broken — a refresh failure would have cleared
// LoadingMore, and handleActionLoadMore's `if ls.LoadingMore { return nil,
// nil }` debounce guard would then let a second, duplicate continuation
// request fire for the same page while the original was still in flight.
func TestClearListLoading_RefreshFailure_LeavesLoadingMoreSet(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(5, 0), &resource.PaginationMeta{
		IsTruncated: true, NextToken: "tok",
	}, false)

	_, loadMoreTasks := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(loadMoreTasks) == 0 {
		t.Fatal("precondition: ActionLoadMore must produce a task")
	}
	_, refreshTasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	if len(refreshTasks) == 0 {
		t.Fatal("precondition: ActionRefresh must produce a task even with a load-more in flight")
	}
	pre := c.Snapshot().Body.List
	if pre == nil || !pre.LoadingMore || !pre.Refreshing {
		t.Fatal("precondition: both LoadingMore and Refreshing must be true before the simulated refresh failure")
	}

	// The concurrent Ctrl+R refresh's own fetch failed — the completing
	// request is the refresh, not the still-outstanding load-more
	// continuation.
	c.ClearListLoading(false)

	lb := c.Snapshot().Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after ClearListLoading(false)")
	}
	if lb.Refreshing {
		t.Error("Refreshing = true after ClearListLoading(false), want false — the failed refresh's own flag must clear")
	}
	if !lb.LoadingMore {
		t.Error("LoadingMore = false after ClearListLoading(false), want true — a refresh failure must not clobber a still-outstanding load-more continuation, or a duplicate continuation request could fire while the original is still in flight")
	}
}

// TestListState_LoadingMoreAndRefreshing_CoexistAndRenderIndependently is a
// DESIGN-DECISION LOCK, not a bug catch. The collapse of Loading/LoadingMore/
// Refreshing into a single enum was proposed and refused: LoadingMore and
// Refreshing are legitimately simultaneous (Ctrl+R fired while an m-key
// load-more is still in flight), and internal/tui/views/resourcelist.go
// renders both indicator lines independently and deliberately — the
// load-more hint block ("── loading... ──" / "m: load more", gated on
// body.LoadingMore/body.Truncated) and the separate "── refreshing... ──"
// line (gated on body.Refreshing) never interact. This pin exists so the
// next person who looks at three booleans and reaches for an enum does not
// silently drop one indicator.
//
// NOT RED-revertable against the current diff: there is no single line in
// this branch's change to revert, since this invariant predates it and the
// pin guards a hypothetical future refactor, not something wave C touched.
// Verified instead by temporarily editing core/app/actions_list.go's
// activeListRefreshTasks to add `ls.LoadingMore = false` right after
// `ls.Refreshing = true` — simulating the exact clobber a naive single-enum
// collapse would introduce — confirming this test failed (LoadingMore=false
// where it must be true), then restoring the file byte-identically (verified
// via `git diff`, exit 0). See the QA session notes for that run; the edit
// was never left in the tree.
func TestListState_LoadingMoreAndRefreshing_CoexistAndRenderIndependently(t *testing.T) {
	c := openListController(t, "ec2")
	c.ApplyResourcesLoaded("ec2", wave3LoadMoreResources(5, 0), &resource.PaginationMeta{
		IsTruncated: true, NextToken: "tok",
	}, false)

	_, loadMoreTasks := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(loadMoreTasks) == 0 {
		t.Fatal("precondition: ActionLoadMore must produce a task")
	}

	_, refreshTasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	if len(refreshTasks) == 0 {
		t.Fatal("precondition: ActionRefresh must produce a task even with a load-more in flight")
	}

	lb := c.Snapshot().Body.List
	if lb == nil {
		t.Fatal("Body.List is nil")
	}
	if !lb.LoadingMore {
		t.Error("LoadingMore = false after ActionRefresh landed on top of an in-flight load-more, want true — LoadingMore and Refreshing must coexist, neither may clobber the other")
	}
	if !lb.Refreshing {
		t.Error("Refreshing = false after ActionRefresh, want true")
	}

	out := stripAnsi(wave3RenderListBody(t, c, "ec2"))
	if !strings.Contains(out, "loading...") {
		t.Errorf("RenderList must show the load-more 'loading...' hint while LoadingMore=true, got:\n%s", out)
	}
	if !strings.Contains(out, "── refreshing... ──") {
		t.Errorf("RenderList must ALSO show the separate refreshing marker while Refreshing=true — both indicators must render independently, got:\n%s", out)
	}
}
