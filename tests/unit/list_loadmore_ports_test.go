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
// insertion order and ascending-name sort order.
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

	// ls.LoadingMore stays set until the fetch result lands, so repeat presses are inert.
	for i := range 5 {
		_, tasks := c.Apply(app.Action{Kind: app.ActionLoadMore})
		if len(tasks) != 0 {
			t.Errorf("ActionLoadMore press %d while already loading must be a no-op, got %d tasks", i+2, len(tasks))
		}
	}
}

func TestActionLoadMore_ErrorClearsLoading_PreservesRowsAndAllowsRetry(t *testing.T) {
	c := openListController(t, "ec2")
	seed := wave3LoadMoreResources(200, 0)
	c.ApplyResourcesLoaded("ec2", seed, &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-p2"}, false)

	_, tasks := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(tasks) == 0 {
		t.Fatal("precondition: ActionLoadMore must produce a task before the simulated error")
	}

	// loadMore=true clears only the load-more request's own flag, LoadingMore.
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

func TestListSort_PreservedAfterLoadMoreAppend(t *testing.T) {
	c := openListController(t, "ec2")

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

// errLoadMoreAPIFailed is a fixed sentinel used to simulate a failed
// KindFetchMore/KindFetchResources execution delivered as a real
// messages.APIError, mirroring errTUIFetchFailed in
// tui_error_marker_parity_test.go (different package, not importable here).
type errLoadMoreAPIFailed struct{}

func (errLoadMoreAPIFailed) Error() string { return "load-more ports pin: simulated fetch failure" }

// A failed load-more on the headless/web lane clears LoadingMore; otherwise
// handleActionLoadMore's debounce guard swallows every later 'm' press and the
// loading indicator never clears.
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

	// The failure arrives as a messages.APIError through Controller.Handle,
	// shaped as executor.go builds a KindFetchMore failure. LoadingMore records
	// which flag the request raised; the clear reads it rather than inferring it
	// from Append, which says how rows would merge.
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

	_, retry := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(retry) == 0 {
		t.Error("after the failure clears LoadingMore, ActionLoadMore must produce a task again (retry) — a stranded LoadingMore=true would debounce this forever")
	}
}

// Each landed fetch installs its own error marker text, replacing a marker
// from an earlier attempt (docs/design/cache-requirements.md: a fetch failure
// swaps the marker for an error marker), and its rows still land.
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
	vs, _ := handlePage(c, messages.ResourcesLoaded{
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

// ClearListLoading clears Refreshing itself, whether or not a
// SetListFetchError call follows it.
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

	// loadMore=false: the completing request is a plain refresh fetch, which
	// clears Loading and Refreshing.
	c.ClearListLoading(false)

	lb := c.Snapshot().Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after ClearListLoading")
	}
	if lb.Refreshing {
		t.Error("Refreshing = true after ClearListLoading, want false — ClearListLoading must stop every in-flight fetch indicator, not just Loading/LoadingMore")
	}
}

// A failed load-more clears only LoadingMore. An in-flight Ctrl+R refresh
// keeps Refreshing: ActionRefresh has no debounce guard of its own, so
// clearing it would let a second refresh dispatch while the first is still in
// flight.
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

// A failed Ctrl+R refresh leaves LoadingMore set; clearing it would let
// handleActionLoadMore's debounce guard pass a duplicate continuation for the
// same page.
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

// LoadingMore and Refreshing are legitimately simultaneous (Ctrl+R during an
// in-flight load-more), and resourcelist.go renders their indicator lines
// independently.
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
