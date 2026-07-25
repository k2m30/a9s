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

// TestApply_AutoOpenSingleDetail_NonEmptyPageMissingTarget_StillChasesViaFetchMore
// pins #261's Codex-flagged gap: a page that HAS rows but does not contain
// the target must still chase via KindFetchMore when pagination remains —
// keying the decision on "was the target found" (matched == nil) rather than
// len(ls.Rows) == 0. Before the fix, a non-empty page silently stopped the
// chase, stranding a target that never lands on the first page of a large
// listing.
func TestApply_AutoOpenSingleDetail_NonEmptyPageMissingTarget_StillChasesViaFetchMore(t *testing.T) {
	const targetID = "i-0target00000002"
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.SetListAutoOpenSingle(true)
	c.PatchListRelatedIDSet([]string{targetID})

	_, tasks := c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		// Non-empty page, but none of these rows is the target — the target
		// is somewhere on a LATER page.
		Resources:  []resource.Resource{{ID: "i-0other0000000001", Type: "ec2"}},
		Pagination: &resource.PaginationMeta{IsTruncated: true, NextToken: "chase-tok-2"},
	})

	var fetchMore *runtime.TaskRequest
	for i := range tasks {
		if tasks[i].Key.Kind == runtime.KindFetchMore {
			fetchMore = &tasks[i]
		}
	}
	if fetchMore == nil {
		t.Fatalf("expected a KindFetchMore task when a non-empty page does not contain the target; tasks: %+v", tasks)
	}
	payload, ok := fetchMore.Payload.(runtime.FetchMorePayload)
	if !ok {
		t.Fatalf("KindFetchMore Payload = %T, want runtime.FetchMorePayload", fetchMore.Payload)
	}
	if payload.ContinuationToken != "chase-tok-2" {
		t.Errorf("FetchMorePayload.ContinuationToken = %q, want %q", payload.ContinuationToken, "chase-tok-2")
	}

	snap := c.Snapshot()
	if snap.Body.Kind != app.BodyKindList {
		t.Errorf("Body.Kind = %v, want BodyKindList — the chase must still be in flight, no detail opened yet", snap.Body.Kind)
	}
}

// TestApply_AutoOpenSingleDetail_LoadingMoreAlreadyTrue_NoPrematureStubCreation
// pins the LoadingMore guard: while the TOP-of-stack placeholder list's own
// chase is already in flight (ls.LoadingMore == true), autoOpenSingleDetail
// must never fall through to StubCreator synthesis for it — even for a type
// ("ami") that has one registered. Handle's ResourcesLoaded case
// unconditionally clears LoadingMore on the SCREEN THE INCOMING EVENT
// MATCHES (handleResourcesLoadedEvent resolves ls by ResourceType, not
// simply "top of stack" — see Handle's own doc comment), so this drives a
// ResourcesLoaded for a DIFFERENT, unrelated resource type ("ec2", not on
// the stack at all): it never touches the "ami" placeholder's LoadingMore,
// yet autoOpenSingleDetail still evaluates whatever is currently on top
// ("ami") on every ResourcesLoaded event. Without the guard, this unrelated
// event would misread "ami"'s still-in-flight chase as exhausted and
// synthesize a premature stub.
func TestApply_AutoOpenSingleDetail_LoadingMoreAlreadyTrue_NoPrematureStubCreation(t *testing.T) {
	const targetID = "ami-0race0000000001"
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ami"})
	c.SetListAutoOpenSingle(true)
	c.PatchListRelatedIDSet([]string{targetID})
	c.SetListLoadingMore(true)

	var snap app.ViewState
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Handle panicked while a chase was already in flight: %v", r)
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
		t.Errorf("Body.Kind = %v, want BodyKindList — a chase already in flight on the top-of-stack placeholder must never fall through to premature stub synthesis, even when an unrelated ResourcesLoaded event arrives", snap.Body.Kind)
	}
}

// ---------------------------------------------------------------------------
// Codex-flagged regression: the stub fallback must not fire while the
// placeholder's OWN fetch is still outstanding (its very first response has
// not landed at all yet — LoadingMore is still false, HasPagination is still
// false, exactly the zero-value shape "fetched, empty, exhausted" also has).
// An unrelated ResourcesLoaded arriving in that window must be a pure no-op
// for the placeholder; only its OWN type's response may resolve it.
// ---------------------------------------------------------------------------

// TestApply_AutoOpenSingleDetail_UnrelatedResourcesLoaded_NoStubNoDetailOpen
// pins the core regression: a by-ID placeholder pending its OWN fetch (never
// having received any ResourcesLoaded of its own yet) must not react to an
// unrelated type's ResourcesLoaded at all — no stub, no detail open, no
// state change.
func TestApply_AutoOpenSingleDetail_UnrelatedResourcesLoaded_NoStubNoDetailOpen(t *testing.T) {
	const targetID = "ami-0pending000000001"
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ami"})
	c.SetListAutoOpenSingle(true)
	c.PatchListRelatedIDSet([]string{targetID})

	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-unrelated0001", Type: "ec2"}},
		Pagination:   nil,
	})

	snap := c.Snapshot()
	if snap.Body.Kind != app.BodyKindList {
		t.Errorf("Body.Kind = %v, want BodyKindList — an unrelated type's ResourcesLoaded must never resolve a DIFFERENT type's still-pending by-ID placeholder", snap.Body.Kind)
	}
}

// TestApply_AutoOpenSingleDetail_UnrelatedThenOwnTypeEmptyNoPagination_StubFiresOnOwnType
// extends the above: after the unrelated event is correctly ignored, the
// placeholder's OWN type resolving empty with no pagination must still
// trigger the StubCreator fallback exactly as before the guard.
func TestApply_AutoOpenSingleDetail_UnrelatedThenOwnTypeEmptyNoPagination_StubFiresOnOwnType(t *testing.T) {
	const targetID = "ami-0pending000000002"
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ami"})
	c.SetListAutoOpenSingle(true)
	c.PatchListRelatedIDSet([]string{targetID})

	c.Handle(messages.ResourcesLoaded{ResourceType: "ec2", Resources: nil, Pagination: nil})
	c.Handle(messages.ResourcesLoaded{ResourceType: "ami", Resources: nil, Pagination: nil})

	snap := c.Snapshot()
	if snap.Body.Kind != app.BodyKindDetail {
		t.Fatalf("Body.Kind = %v after the placeholder's OWN type resolved empty/no-pagination, want BodyKindDetail (stub fallback)", snap.Body.Kind)
	}
	if got := c.GetDetailResource().ID; got != targetID {
		t.Errorf("GetDetailResource().ID = %q, want the synthesized stub's target ID %q", got, targetID)
	}
}

// TestApply_AutoOpenSingleDetail_UnrelatedThenOwnTypePagination_ChaseFallbackStillFires
// pins that the pagination-chase fallback is unaffected by the type guard:
// after an unrelated event is ignored, the placeholder's OWN type resolving
// with pagination still queues KindFetchMore rather than giving up or
// synthesizing a stub early.
func TestApply_AutoOpenSingleDetail_UnrelatedThenOwnTypePagination_ChaseFallbackStillFires(t *testing.T) {
	const targetID = "ami-0pending000000003"
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ami"})
	c.SetListAutoOpenSingle(true)
	c.PatchListRelatedIDSet([]string{targetID})

	c.Handle(messages.ResourcesLoaded{ResourceType: "ec2", Resources: nil, Pagination: nil})
	_, tasks := c.Handle(messages.ResourcesLoaded{
		ResourceType: "ami",
		Resources:    nil,
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "chase-tok-guard"},
	})

	var fetchMore *runtime.TaskRequest
	for i := range tasks {
		if tasks[i].Key.Kind == runtime.KindFetchMore {
			fetchMore = &tasks[i]
		}
	}
	if fetchMore == nil {
		t.Fatalf("expected a KindFetchMore task once the placeholder's OWN type resolved with pagination remaining; tasks: %+v", tasks)
	}
	if fetchMore.Key.Scope != "ami" {
		t.Errorf("KindFetchMore Scope = %q, want %q", fetchMore.Key.Scope, "ami")
	}

	snap := c.Snapshot()
	if snap.Body.Kind != app.BodyKindList {
		t.Errorf("Body.Kind = %v, want BodyKindList — the chase must still be in flight, no stub or detail opened yet", snap.Body.Kind)
	}
}

// TestApply_AutoOpenSingleDetail_UnrelatedThenOwnTypeRealResult_OpensRealResourceNotStub
// pins the last axis: once the placeholder's OWN type actually delivers the
// target row, the REAL resource opens — not a stub — even after an
// unrelated event was seen first. ami's StubCreator sets Name to the bare
// ID; a real row's distinct Name is the observable proof this is not that
// synthesized shape.
func TestApply_AutoOpenSingleDetail_UnrelatedThenOwnTypeRealResult_OpensRealResourceNotStub(t *testing.T) {
	const targetID = "ami-0pending000000004"
	const realName = "real-production-ami"
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ami"})
	c.SetListAutoOpenSingle(true)
	c.PatchListRelatedIDSet([]string{targetID})

	c.Handle(messages.ResourcesLoaded{ResourceType: "ec2", Resources: nil, Pagination: nil})
	c.Handle(messages.ResourcesLoaded{
		ResourceType: "ami",
		Resources:    []resource.Resource{{ID: targetID, Name: realName, Type: "ami"}},
		Pagination:   nil,
	})

	snap := c.Snapshot()
	if snap.Body.Kind != app.BodyKindDetail {
		t.Fatalf("Body.Kind = %v after the placeholder's OWN type delivered the real target row, want BodyKindDetail", snap.Body.Kind)
	}
	got := c.GetDetailResource()
	if got.ID != targetID {
		t.Errorf("GetDetailResource().ID = %q, want %q", got.ID, targetID)
	}
	if got.Name != realName {
		t.Errorf("GetDetailResource().Name = %q, want the REAL row's %q (a stub's Name would equal the bare ID %q)", got.Name, realName, targetID)
	}
}
