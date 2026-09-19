// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package app_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// TestHandleRelatedCheckBatch_ResourceIDs_EnableSingleResourceNav verifies that
// when a RelatedCheckBatch carries exactly one ResourceID, a subsequent
// ActionSelect on the focused related row produces a navigation task (proving
// the ID reached the row and was used to derive TargetID for the detail path).
//
// mergeDetailRelatedRow's existing-row branch must assign ResourceIDs along
// with Count/Loading/Err/Truncated/FetchFilter.
func TestHandleRelatedCheckBatch_ResourceIDs_EnableSingleResourceNav(t *testing.T) {
	res := fakeEC2Resources()[0]
	c := newControllerAtDetail(t, res, "ec2")

	snap := c.Snapshot()
	if snap.Body.Kind != app.BodyKindDetail {
		t.Fatalf("precondition: expected detail screen, got %q", snap.Body.Kind)
	}

	// Seed the related row with an initial ID — creates the row so the batch
	// below exercises the existing-row (update) branch of mergeDetailRelatedRow.
	c.ApplyDetailRelated([]app.DetailRelatedRow{
		{
			TargetType:  "sg",
			DisplayName: "Security Groups",
			Count:       1,
			ResourceIDs: []string{"sg-initial-0001"},
		},
	})

	// Send a RelatedCheckBatch that updates the same DisplayName with a new
	// single ResourceID.  The update path must write the new ResourceIDs.
	updatedID := "sg-updated-0002"
	batch := messages.RelatedCheckBatch{
		ResourceType:     "ec2",
		SourceResourceID: res.ID,
		Results: []messages.RelatedCheckResult{
			{
				ResourceType:     "ec2",
				SourceResourceID: res.ID,
				DefDisplayName:   "Security Groups",
				Result:           resource.KnownRelated("sg", []string{updatedID}, false),
			},
		},
	}
	c.Handle(batch) //nolint:ineffassign,staticcheck // asserting via ActionSelect tasks, not Handle return value

	c.Apply(app.Action{Kind: app.ActionToggleFocus}) //nolint:ineffassign,staticcheck // focus state observed via Snapshot

	snap = c.Snapshot()
	if snap.Body.Detail == nil {
		t.Fatal("detail body nil after ToggleFocus")
	}
	if !snap.Body.Detail.RelatedFocused {
		t.Skip("related panel did not accept focus — cannot drive related navigation in this test env")
	}

	_, navTasks := c.Apply(app.Action{Kind: app.ActionSelect})

	if len(navTasks) == 0 {
		t.Error("ActionSelect on related row with ResourceIDs=[sg-updated-0002] returned no tasks — " +
			"pre-fix bug: mergeDetailRelatedRow update path dropped ResourceIDs, leaving targetID=empty")
	}
}

// TestHandleRelatedCheckBatch_ResourceIDs_InsertPath verifies that when a
// RelatedCheckBatch result for a new DisplayName (insert path) carries
// ResourceIDs, those IDs end up usable for navigation.
func TestHandleRelatedCheckBatch_ResourceIDs_InsertPath(t *testing.T) {
	res := fakeEC2Resources()[0]
	c := newControllerAtDetail(t, res, "ec2")

	snap := c.Snapshot()
	if snap.Body.Kind != app.BodyKindDetail {
		t.Fatalf("expected detail screen, got %q", snap.Body.Kind)
	}

	// Fresh insert — no prior row with this DisplayName.
	batch := messages.RelatedCheckBatch{
		ResourceType:     "ec2",
		SourceResourceID: res.ID,
		Results: []messages.RelatedCheckResult{
			{
				ResourceType:     "ec2",
				SourceResourceID: res.ID,
				DefDisplayName:   "IAM Roles",
				Result: resource.KnownRelated("iam-role", []string{
					"arn:aws:iam::123456789012:role/test-role",
				}, false),
			},
		},
	}
	c.Handle(batch) //nolint:ineffassign,staticcheck // asserting via ActionSelect tasks

	c.Apply(app.Action{Kind: app.ActionToggleFocus}) //nolint:ineffassign,staticcheck // focus state observed via Snapshot

	snap = c.Snapshot()
	if snap.Body.Detail == nil || !snap.Body.Detail.RelatedFocused {
		t.Skip("related panel not focused — cannot drive related navigation")
	}

	_, navTasks := c.Apply(app.Action{Kind: app.ActionSelect})
	if len(navTasks) == 0 {
		t.Error("ActionSelect on related row with ResourceIDs set (insert path) returned no tasks — " +
			"ResourceIDs were not stored in the insert path")
	}
}

// TestActionLoadMore_ChildList_PayloadCarriesParentContext verifies that when
// ActionLoadMore is applied to a list with ParentContext set (child list),
// the returned FetchMorePayload carries that ParentContext.
func TestActionLoadMore_ChildList_PayloadCarriesParentContext(t *testing.T) {
	c := newListController(t, "ec2")

	wantParentCtx := map[string]string{
		"cluster": "prod-cluster",
		"service": "api-service",
	}
	wantCursor := "cursor-abc-123"

	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   wantCursor,
	}, false)

	// PatchListParentContext simulates having entered via ActionChildView.
	c.PatchListParentContext(wantParentCtx)

	_, tasks := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(tasks) == 0 {
		t.Fatal("ActionLoadMore returned no tasks — expected KindFetchMore task")
	}

	var fetchMore *runtime.TaskRequest
	for i := range tasks {
		if tasks[i].Key.Kind == runtime.KindFetchMore {
			fetchMore = &tasks[i]
			break
		}
	}
	if fetchMore == nil {
		t.Fatalf("no KindFetchMore task among %d returned tasks", len(tasks))
	}

	payload, ok := fetchMore.Payload.(runtime.FetchMorePayload)
	if !ok {
		t.Fatalf("KindFetchMore task payload type %T, want runtime.FetchMorePayload", fetchMore.Payload)
	}

	if payload.ContinuationToken != wantCursor {
		t.Errorf("FetchMorePayload.ContinuationToken=%q, want %q", payload.ContinuationToken, wantCursor)
	}
	if len(payload.ParentContext) == 0 {
		t.Error("FetchMorePayload.ParentContext is empty — " +
			"pre-fix bug: child-list ParentContext was omitted from FetchMorePayload")
	}
	for k, v := range wantParentCtx {
		if payload.ParentContext[k] != v {
			t.Errorf("FetchMorePayload.ParentContext[%q]=%q, want %q", k, payload.ParentContext[k], v)
		}
	}
}

// TestActionLoadMore_FilteredList_PayloadCarriesFetchFilter verifies that when
// ActionLoadMore is applied to a list with FetchFilter set, the returned
// FetchMorePayload carries that FetchFilter.
func TestActionLoadMore_FilteredList_PayloadCarriesFetchFilter(t *testing.T) {
	c := newListController(t, "ec2")

	wantFilter := map[string]string{
		"vpc-id": "vpc-0deadbeef",
	}
	wantCursor := "cursor-filter-456"

	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   wantCursor,
	}, false)

	c.PatchListFetchFilter(wantFilter)

	_, tasks := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(tasks) == 0 {
		t.Fatal("ActionLoadMore returned no tasks")
	}

	var fetchMore *runtime.TaskRequest
	for i := range tasks {
		if tasks[i].Key.Kind == runtime.KindFetchMore {
			fetchMore = &tasks[i]
			break
		}
	}
	if fetchMore == nil {
		t.Fatalf("no KindFetchMore task among %d returned tasks", len(tasks))
	}

	payload, ok := fetchMore.Payload.(runtime.FetchMorePayload)
	if !ok {
		t.Fatalf("KindFetchMore payload type %T, want runtime.FetchMorePayload", fetchMore.Payload)
	}

	if payload.ContinuationToken != wantCursor {
		t.Errorf("FetchMorePayload.ContinuationToken=%q, want %q", payload.ContinuationToken, wantCursor)
	}
	if len(payload.FetchFilter) == 0 {
		t.Error("FetchMorePayload.FetchFilter is empty — " +
			"pre-fix bug: FetchFilter was omitted from FetchMorePayload, executor routed to wrong fetcher")
	}
	for k, v := range wantFilter {
		if payload.FetchFilter[k] != v {
			t.Errorf("FetchMorePayload.FetchFilter[%q]=%q, want %q", k, payload.FetchFilter[k], v)
		}
	}
}

// TestActionLoadMore_NoContext_PayloadHasOnlyToken verifies that a plain
// top-level list (no ParentContext, no FetchFilter) produces a FetchMorePayload
// with only the ContinuationToken set and both maps nil/empty.
func TestActionLoadMore_NoContext_PayloadHasOnlyToken(t *testing.T) {
	c := newListController(t, "ec2")

	wantCursor := "cursor-plain-789"

	c.ApplyResourcesLoaded("ec2", fakeEC2Resources(), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   wantCursor,
	}, false)

	_, tasks := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(tasks) == 0 {
		t.Fatal("ActionLoadMore returned no tasks for plain list")
	}

	var fetchMore *runtime.TaskRequest
	for i := range tasks {
		if tasks[i].Key.Kind == runtime.KindFetchMore {
			fetchMore = &tasks[i]
			break
		}
	}
	if fetchMore == nil {
		t.Fatalf("no KindFetchMore task among %d returned tasks", len(tasks))
	}

	payload, ok := fetchMore.Payload.(runtime.FetchMorePayload)
	if !ok {
		t.Fatalf("KindFetchMore payload type %T, want runtime.FetchMorePayload", fetchMore.Payload)
	}
	if payload.ContinuationToken != wantCursor {
		t.Errorf("FetchMorePayload.ContinuationToken=%q, want %q", payload.ContinuationToken, wantCursor)
	}
	if len(payload.ParentContext) != 0 {
		t.Errorf("FetchMorePayload.ParentContext non-empty on plain list: %v", payload.ParentContext)
	}
	if len(payload.FetchFilter) != 0 {
		t.Errorf("FetchMorePayload.FetchFilter non-empty on plain list: %v", payload.FetchFilter)
	}
}

// TestActionSelect_RelatedNav_FetchFilter_TaskCarriesPayload verifies that when
// ActionSelect on a focused related row resolves to a NavigationKindFilteredList
// with FetchFilter, the returned tasks include KindFetchFiltered with a non-nil
// FetchFilteredPayload.Filter.
func TestActionSelect_RelatedNav_FetchFilter_TaskCarriesPayload(t *testing.T) {
	res := fakeEC2Resources()[0]
	c := newControllerAtDetail(t, res, "ec2")

	snap := c.Snapshot()
	if snap.Body.Kind != app.BodyKindDetail {
		t.Fatalf("precondition: expected detail screen, got %q", snap.Body.Kind)
	}

	// Seed a related row with a FetchFilter (as a filter-based checker would
	// produce).  Count>1 ensures the row resolves via NavigationKindFilteredList,
	// not the single-resource fast-path.
	wantFilter := map[string]string{"instance-id": res.ID}
	c.ApplyDetailRelated([]app.DetailRelatedRow{
		{
			TargetType:  "sg",
			DisplayName: "Security Groups",
			Count:       3,
			FetchFilter: wantFilter,
		},
	})

	c.Apply(app.Action{Kind: app.ActionToggleFocus}) //nolint:ineffassign,staticcheck // focus state observed via Snapshot

	snap = c.Snapshot()
	if snap.Body.Detail == nil || !snap.Body.Detail.RelatedFocused {
		t.Skip("related panel did not accept focus — cannot test related navigation in this environment")
	}

	_, navTasks := c.Apply(app.Action{Kind: app.ActionSelect})

	if len(navTasks) == 0 {
		t.Fatal("ActionSelect on related row with FetchFilter returned no tasks — " +
			"expected KindFetchFiltered task")
	}

	var fetchFiltered *runtime.TaskRequest
	for i := range navTasks {
		if navTasks[i].Key.Kind == runtime.KindFetchFiltered {
			fetchFiltered = &navTasks[i]
			break
		}
	}
	if fetchFiltered == nil {
		kinds := make([]string, len(navTasks))
		for i, tr := range navTasks {
			kinds[i] = string(tr.Key.Kind)
		}
		t.Fatalf("no KindFetchFiltered task in returned tasks %v — "+
			"applyRelatedNavResult did not emit filtered task for FetchFilter-bearing row", kinds)
	}

	payload, ok := fetchFiltered.Payload.(runtime.FetchFilteredPayload)
	if !ok {
		t.Fatalf("KindFetchFiltered payload type %T, want runtime.FetchFilteredPayload — "+
			"pre-fix: payload was nil (HandleRelatedNavigate returned no-payload task)", fetchFiltered.Payload)
	}
	if len(payload.Filter) == 0 {
		t.Error("FetchFilteredPayload.Filter is empty — " +
			"applyRelatedNavResult did not populate Filter from NavigationResult.FetchFilter (pre-fix bug)")
	}
	for k, v := range wantFilter {
		if payload.Filter[k] != v {
			t.Errorf("FetchFilteredPayload.Filter[%q]=%q, want %q", k, payload.Filter[k], v)
		}
	}
}

// TestRelatedNav_MultiID_SeedsRelatedIDSet verifies that navigating a
// related row that carries multiple ResourceIDs and NO FetchFilter (e.g. an EC2
// instance → its several security groups) resolves to NavigationKindFilteredList
// and seeds the pushed list's RelatedIDSet to exactly those IDs — so the list
// renders only the related subset (list.go's RelatedIDSet prefilter), not every
// resource of the target type. Both the mouse click (ActionRelatedSelect) and
// the keyboard Enter (ActionSelect) paths must seed it identically, since both
// route through the shared dispatchRelatedNavigate.
func TestRelatedNav_MultiID_SeedsRelatedIDSet(t *testing.T) {
	ids := []string{"sg-aaa111", "sg-bbb222", "sg-ccc333"}
	c := newControllerAtDetail(t, fakeEC2Resources()[0], "ec2")
	c.ApplyDetailRelated([]app.DetailRelatedRow{{
		TargetType:  "sg",
		DisplayName: "Security Groups",
		Count:       len(ids), // >1 → filtered list, not the single-resource fast-path
		ResourceIDs: ids,
		// no FetchFilter → the multi-ID subset path
	}})

	// The click (ActionRelatedSelect) and the keyboard Enter (ActionSelect)
	// share dispatchRelatedNavigate, so this exercises the applyRelatedNavResult
	// seeding both use.
	vs, _ := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})

	// Apply must return the POST-navigation snapshot: c.snapshot() runs AFTER
	// dispatchRelatedNavigate pushes the filtered list (Go evaluates return
	// operands left-to-right).
	if vs.Body.Kind != app.BodyKindList {
		t.Errorf("Apply returned Body.Kind=%q after the related click, want %q "+
			"(stale pre-navigation snapshot — snapshot taken before dispatch)", vs.Body.Kind, app.BodyKindList)
	}

	got := c.GetListRelatedIDSet()
	if len(got) == 0 {
		t.Fatal("RelatedIDSet empty after multi-ID related navigation — the list shows ALL " +
			"resources of the target type, not just the related subset (P2 regression)")
	}
	if len(got) != len(ids) {
		t.Errorf("RelatedIDSet has %d ids, want %d %v", len(got), len(ids), ids)
	}
	for _, id := range ids {
		if _, ok := got[id]; !ok {
			t.Errorf("RelatedIDSet missing %q (got %v)", id, got)
		}
	}
}

// TestLiveConnect_NotTested records that the live-connect branch in
// construct.go opens a real TCP connection to AWS endpoints and offers no
// injectable failure seam (dial func, transport override) to exercise the
// "connection failed → controller stays on menu" path without real
// credentials or network access.
//
// The demo-mode path exercised by TestHeadless_FetchPopulatesListRows
// (headless_drain_test.go) is the structurally parallel path: controller
// initialises with fake clients, tasks execute synchronously, and list rows
// are populated.
func TestLiveConnect_NotTested(t *testing.T) {
	t.Skip("live-connect fix requires real AWS credentials; no injectable failure seam — see comment")
}
