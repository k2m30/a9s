package unit

// app_autoopen_single_detail_fallback_test.go — coverage for
// autoOpenSingleDetail's two new zero-row fallbacks (core/app/handle.go):
// when the placeholder list's single related-ID target row hasn't loaded and
// the list came back empty, either (1) pagination remains — chase it via a
// KindFetchMore task, marking the list LoadingMore, or (2) pagination is
// exhausted and the target type has a catalog StubCreator — synthesize a
// stub resource and pop straight to its detail. When neither applies, the
// function must no-op cleanly: no panic, the placeholder list stays put.
//
// Driven entirely through the real production entry points a web/headless
// host uses: Apply(ActionCommand) to open a real catalog type's list (the
// same construction the whitebox core/app/handle_autoopen_test.go uses,
// translated to this package's exported surface — SetListAutoOpenSingle /
// PatchListRelatedIDSet are the exported equivalents of that test's direct
// ls.AutoOpenSingle/ls.RelatedIDSet field pokes, unreachable from here), then
// Handle(messages.ResourcesLoaded) with zero resources to trigger the chase.
//
// Transplanted verbatim from ref/detail-enrichment-261-attempt1 (git show) —
// every symbol it references (newTestController, SetListAutoOpenSingle,
// PatchListRelatedIDSet, GetDetailResource, BodyKindDetail/BodyKindList,
// KindFetchMore/FetchMorePayload) is still current v2 API; no adaptation
// required.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// TestApply_AutoOpenSingleDetail_ZeroRowsWithPagination_QueuesFetchMore
// covers the paginated-chase fallback: a zero-row page with an unexhausted
// pagination token must queue KindFetchMore (scoped to the list's type,
// carrying the continuation token) and mark the list LoadingMore.
func TestApply_AutoOpenSingleDetail_ZeroRowsWithPagination_QueuesFetchMore(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.SetListAutoOpenSingle(true)
	c.PatchListRelatedIDSet([]string{"i-0target00000001"})

	_, tasks := c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    nil,
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "chase-tok-1"},
	})

	var fetchMore *runtime.TaskRequest
	for i := range tasks {
		if tasks[i].Key.Kind == runtime.KindFetchMore {
			fetchMore = &tasks[i]
		}
	}
	if fetchMore == nil {
		t.Fatalf("expected a KindFetchMore task for the zero-row paginated auto-open chase; tasks: %+v", tasks)
	}
	if fetchMore.Key.Scope != "ec2" {
		t.Errorf("KindFetchMore Scope = %q, want %q", fetchMore.Key.Scope, "ec2")
	}
	payload, ok := fetchMore.Payload.(runtime.FetchMorePayload)
	if !ok {
		t.Fatalf("KindFetchMore Payload = %T, want runtime.FetchMorePayload", fetchMore.Payload)
	}
	if payload.ContinuationToken != "chase-tok-1" {
		t.Errorf("FetchMorePayload.ContinuationToken = %q, want %q", payload.ContinuationToken, "chase-tok-1")
	}

	snap := c.Snapshot()
	if snap.Body.List == nil || !snap.Body.List.LoadingMore {
		t.Error("list LoadingMore not set after the zero-row paginated auto-open chase")
	}
}

// TestApply_AutoOpenSingleDetail_ZeroRowsNoPaginationWithStubCreator_OpensSynthesizedDetail
// covers the StubCreator-synthesis fallback using the one real catalog
// StubCreator (core/aws/catalog_compute.go's "ami" entry): pagination
// exhausted, target row never loaded — the function must synthesize a stub
// resource for the target ID and pop straight to its detail.
func TestApply_AutoOpenSingleDetail_ZeroRowsNoPaginationWithStubCreator_OpensSynthesizedDetail(t *testing.T) {
	const targetID = "ami-0stub00000000001"
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ami"})
	c.SetListAutoOpenSingle(true)
	c.PatchListRelatedIDSet([]string{targetID})

	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ami",
		Resources:    nil,
		Pagination:   nil,
	})

	snap := c.Snapshot()
	if snap.Body.Kind != app.BodyKindDetail {
		t.Fatalf("Body.Kind = %v after zero-row/no-pagination auto-open with a StubCreator, want BodyKindDetail", snap.Body.Kind)
	}
	if got := c.GetDetailResource().ID; got != targetID {
		t.Errorf("GetDetailResource().ID = %q, want the synthesized stub's target ID %q", got, targetID)
	}
}

// TestApply_AutoOpenSingleDetail_ZeroRowsNoPaginationNoStubCreator_PlaceholderRemains
// covers the give-up path using a real catalog type with no StubCreator
// ("ec2" — the only StubCreator registered in the whole catalog is "ami"):
// nothing left to chase or synthesize, so the function must not panic and
// must leave the placeholder list in place (never pop to a detail screen).
func TestApply_AutoOpenSingleDetail_ZeroRowsNoPaginationNoStubCreator_PlaceholderRemains(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.SetListAutoOpenSingle(true)
	c.PatchListRelatedIDSet([]string{"i-0nostub0000000001"})

	var snap app.ViewState
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Handle panicked on zero-row/no-pagination/no-StubCreator auto-open: %v", r)
			}
		}()
		c.Handle(messages.ResourcesLoaded{
			ResourceType: "ec2",
			Resources:    nil,
			Pagination:   nil,
		})
		snap = c.Snapshot()
	}()

	if snap.Body.Kind != app.BodyKindList {
		t.Errorf("Body.Kind = %v, want BodyKindList — the placeholder list must remain when there's nothing left to chase or synthesize", snap.Body.Kind)
	}
	if snap.Body.List == nil || len(snap.Body.List.Rows) != 0 {
		t.Errorf("Body.List.Rows = %+v, want empty (still the zero-row placeholder)", snap.Body.List.Rows)
	}
}
