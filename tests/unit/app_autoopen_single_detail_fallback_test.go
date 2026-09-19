package unit

// autoOpenSingleDetail (core/app/handle.go) has two zero-row fallbacks for a
// placeholder list whose single related-ID target row has not loaded: while
// pagination remains it chases via a KindFetchMore task and marks the list
// LoadingMore; once pagination is exhausted and the target type has a catalog
// StubCreator it synthesizes a stub resource and opens its detail. Otherwise
// the placeholder list stays put.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

func TestApply_AutoOpenSingleDetail_ZeroRowsWithPagination_QueuesFetchMore(t *testing.T) {
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.SetListAutoOpenSingle(true)
	c.PatchListRelatedIDSet([]string{"i-0target00000001"})
	// A by-ID/related placeholder is never the canonical top-level list —
	// PatchListEscPops mirrors the EscPops stamp navigate.go's
	// applyRelatedNavResult sets on every real by-ID/filtered/child
	// placeholder it pushes, which isTopLevelCanonicalList (list_state.go)
	// keys on to exempt it from the canonical-only Provenance gate (handle.go).
	c.PatchListEscPops(true)

	_, tasks := handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    nil,
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "chase-tok-1"}, Provenance: messages.FetchProvenanceFilteredList,
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

// "ami" is the catalog type with a StubCreator (core/aws/catalog_compute.go).
func TestApply_AutoOpenSingleDetail_ZeroRowsNoPaginationWithStubCreator_OpensSynthesizedDetail(t *testing.T) {
	const targetID = "ami-0stub00000000001"
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ami"})
	c.SetListAutoOpenSingle(true)
	c.PatchListRelatedIDSet([]string{targetID})
	c.PatchListEscPops(true)

	handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ami",
		Resources:    nil,
		Pagination:   nil, Provenance: messages.FetchProvenanceByID,
	})

	snap := c.Snapshot()
	if snap.Body.Kind != app.BodyKindDetail {
		t.Fatalf("Body.Kind = %v after zero-row/no-pagination auto-open with a StubCreator, want BodyKindDetail", snap.Body.Kind)
	}
	if got := c.GetDetailResource().ID; got != targetID {
		t.Errorf("GetDetailResource().ID = %q, want the synthesized stub's target ID %q", got, targetID)
	}
}

// "ec2" registers no StubCreator, so nothing is left to chase or synthesize.
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
		handlePage(c, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceByID,
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

// The chase keys on whether the target was found (matched == nil), not on
// len(ls.Rows) == 0, so a target past the first page of a large listing is
// still reached.
func TestApply_AutoOpenSingleDetail_NonEmptyPageMissingTarget_StillChasesViaFetchMore(t *testing.T) {
	const targetID = "i-0target00000002"
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	c.SetListAutoOpenSingle(true)
	c.PatchListRelatedIDSet([]string{targetID})
	c.PatchListEscPops(true)

	_, tasks := handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-0other0000000001", Type: "ec2"}},
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "chase-tok-2"}, Provenance: messages.FetchProvenanceFilteredList,
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

// While the top-of-stack placeholder's own chase is in flight
// (ls.LoadingMore), autoOpenSingleDetail never falls through to StubCreator
// synthesis for it, even for "ami", which has one. Handle's ResourcesLoaded
// case clears LoadingMore on the screen the incoming event matches
// (handleResourcesLoadedEvent resolves ls by ResourceType, not top of stack),
// so a ResourcesLoaded for "ec2", which is not on the stack, leaves the "ami"
// placeholder's LoadingMore set while autoOpenSingleDetail still evaluates the
// top of stack. Without the guard that event would read "ami"'s in-flight
// chase as exhausted and synthesize a premature stub.
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
		// Deliberately not handlePage: this page names a type the open screen is not,
		// so it belongs to no screen and is delivered exactly as it is — stamping it
		// for the screen on top is the guess the identity exists to remove.
		c.Handle(messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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

// The stub fallback must not fire while the placeholder's OWN fetch is
// still outstanding (its very first response has
// not landed at all yet — LoadingMore is still false, HasPagination is still
// false, exactly the zero-value shape "fetched, empty, exhausted" also has).
// An unrelated ResourcesLoaded arriving in that window must be a pure no-op
// for the placeholder; only its OWN type's response may resolve it.
func TestApply_AutoOpenSingleDetail_UnrelatedResourcesLoaded_NoStubNoDetailOpen(t *testing.T) {
	const targetID = "ami-0pending000000001"
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ami"})
	c.SetListAutoOpenSingle(true)
	c.PatchListRelatedIDSet([]string{targetID})

	// Deliberately not handlePage: this page names a type the open screen is not,
	// so it belongs to no screen and is delivered exactly as it is — stamping it
	// for the screen on top is the guess the identity exists to remove.
	c.Handle(messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-unrelated0001", Type: "ec2"}},
		Pagination:   nil,
	})

	snap := c.Snapshot()
	if snap.Body.Kind != app.BodyKindList {
		t.Errorf("Body.Kind = %v, want BodyKindList — an unrelated type's ResourcesLoaded must never resolve a DIFFERENT type's still-pending by-ID placeholder", snap.Body.Kind)
	}
}

func TestApply_AutoOpenSingleDetail_UnrelatedThenOwnTypeEmptyNoPagination_StubFiresOnOwnType(t *testing.T) {
	const targetID = "ami-0pending000000002"
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ami"})
	c.SetListAutoOpenSingle(true)
	c.PatchListRelatedIDSet([]string{targetID})
	c.PatchListEscPops(true)

	// Deliberately not handlePage: this page names a type the open screen is not,
	// so it belongs to no screen and is delivered exactly as it is — stamping it
	// for the screen on top is the guess the identity exists to remove.
	c.Handle(messages.ResourcesLoaded{ResourceType: "ec2", Resources: nil, Pagination: nil, Provenance: messages.FetchProvenanceCanonicalList})
	handlePage(c, messages.ResourcesLoaded{ResourceType: "ami", Resources: nil, Pagination: nil, Provenance: messages.FetchProvenanceByID})

	snap := c.Snapshot()
	if snap.Body.Kind != app.BodyKindDetail {
		t.Fatalf("Body.Kind = %v after the placeholder's OWN type resolved empty/no-pagination, want BodyKindDetail (stub fallback)", snap.Body.Kind)
	}
	if got := c.GetDetailResource().ID; got != targetID {
		t.Errorf("GetDetailResource().ID = %q, want the synthesized stub's target ID %q", got, targetID)
	}
}

func TestApply_AutoOpenSingleDetail_UnrelatedThenOwnTypePagination_ChaseFallbackStillFires(t *testing.T) {
	const targetID = "ami-0pending000000003"
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ami"})
	c.SetListAutoOpenSingle(true)
	c.PatchListRelatedIDSet([]string{targetID})
	c.PatchListEscPops(true)

	// Deliberately not handlePage: this page names a type the open screen is not,
	// so it belongs to no screen and is delivered exactly as it is — stamping it
	// for the screen on top is the guess the identity exists to remove.
	c.Handle(messages.ResourcesLoaded{ResourceType: "ec2", Resources: nil, Pagination: nil, Provenance: messages.FetchProvenanceCanonicalList})
	_, tasks := handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ami",
		Resources:    nil,
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "chase-tok-guard"}, Provenance: messages.FetchProvenanceFilteredList,
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

// ami's StubCreator sets Name to the bare ID; a real row's distinct Name
// tells the real resource apart from a stub.
func TestApply_AutoOpenSingleDetail_UnrelatedThenOwnTypeRealResult_OpensRealResourceNotStub(t *testing.T) {
	const targetID = "ami-0pending000000004"
	const realName = "real-production-ami"
	c := newTestController(t)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "ami"})
	c.SetListAutoOpenSingle(true)
	c.PatchListRelatedIDSet([]string{targetID})
	c.PatchListEscPops(true)

	// Deliberately not handlePage: this page names a type the open screen is not,
	// so it belongs to no screen and is delivered exactly as it is — stamping it
	// for the screen on top is the guess the identity exists to remove.
	c.Handle(messages.ResourcesLoaded{ResourceType: "ec2", Resources: nil, Pagination: nil, Provenance: messages.FetchProvenanceCanonicalList})
	handlePage(c, messages.ResourcesLoaded{
		ResourceType: "ami",
		Resources:    []resource.Resource{{ID: targetID, Name: realName, Type: "ami"}},
		Pagination:   nil, Provenance: messages.FetchProvenanceByID,
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
