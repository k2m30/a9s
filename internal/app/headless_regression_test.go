// headless_regression_test.go — regression tests for four headless-controller
// bugs fixed in commit 56910d32.
//
// Fix 3 (related ResourceIDs): mergeDetailRelatedRow now propagates ResourceIDs
//   from the RelatedCheckBatch result into DetailState.RelatedRows. Pre-fix the
//   field was silently dropped on the existing-row update path.
//
// Fix 4 (load-more context): ActionLoadMore now includes ParentContext and
//   FetchFilter in the emitted FetchMorePayload. Pre-fix only ContinuationToken
//   was set, causing the executor to call the wrong (top-level) fetcher for
//   child / filtered lists.
//
// Fix 5 (filtered related-nav payload): applyRelatedNavResult now returns a
//   payload-bearing KindFetchFiltered task (FetchFilteredPayload{Filter: ...}).
//   Pre-fix HandleRelatedNavigate returned a no-payload task, which ExecuteTask
//   could not route to the filtered fetcher, producing an empty list.
//
// Fix 6 (live connect): not tested here — see TestLiveConnect_NotTested below.
package app_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ─────────────────────────────────────────────────────────────────────────────
// Fix 3: mergeDetailRelatedRow must propagate ResourceIDs
// ─────────────────────────────────────────────────────────────────────────────

// TestHandleRelatedCheckBatch_ResourceIDs_EnableSingleResourceNav verifies that
// when a RelatedCheckBatch carries exactly one ResourceID, a subsequent
// ActionSelect on the focused related row produces a navigation task (proving
// the ID reached the row and was used to derive TargetID for the detail path).
//
// Strategy: use newControllerAtDetail to arrive at a known detail screen, seed
// the related panel with a row via ApplyDetailRelated, then send a
// RelatedCheckBatch that updates the same DisplayName with ResourceIDs=[Y].
// ActionSelect on the focused row must emit at least one task.  If ResourceIDs
// were dropped (pre-fix), the update path set ResourceIDs to nil, so targetID
// would be "" and navigation would fall through without emitting a fetch task.
//
// Pre-fix failure: mergeDetailRelatedRow's existing-row branch assigned
// Count/Loading/Err/Approximate/FetchFilter but omitted
// `ds.RelatedRows[i].ResourceIDs = resourceIDs`.
func TestHandleRelatedCheckBatch_ResourceIDs_EnableSingleResourceNav(t *testing.T) {
	res := fakeEC2Resources()[0]
	c := newControllerAtDetail(res, "ec2")

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
				Result: resource.RelatedCheckResult{
					Count:       1,
					TargetType:  "sg",
					ResourceIDs: []string{updatedID},
				},
			},
		},
	}
	c.Handle(batch) //nolint:ineffassign,staticcheck // asserting via ActionSelect tasks, not Handle return value

	// Enable related focus so ActionSelect navigates via the related row.
	c.Apply(app.Action{Kind: app.ActionToggleFocus}) //nolint:ineffassign,staticcheck // focus state observed via Snapshot

	snap = c.Snapshot()
	if snap.Body.Detail == nil {
		t.Fatal("detail body nil after ToggleFocus")
	}
	if !snap.Body.Detail.RelatedFocused {
		t.Skip("related panel did not accept focus — cannot drive related navigation in this test env")
	}

	// ActionSelect on the focused related row.  If ResourceIDs survived the
	// batch update, navigation resolves to a single resource (targetID ==
	// updatedID) and emits at least one task.  If ResourceIDs were dropped,
	// targetID=="" and no task is emitted (pre-fix behavior).
	_, navTasks := c.Apply(app.Action{Kind: app.ActionSelect})

	if len(navTasks) == 0 {
		t.Error("ActionSelect on related row with ResourceIDs=[sg-updated-0002] returned no tasks — " +
			"pre-fix bug: mergeDetailRelatedRow update path dropped ResourceIDs, leaving targetID=empty")
	}
}

// TestHandleRelatedCheckBatch_ResourceIDs_InsertPath verifies that when a
// RelatedCheckBatch result for a new DisplayName (insert path) carries
// ResourceIDs, those IDs end up usable for navigation.
//
// The actual bug was only on the update path; the insert path always assigned
// the full struct.  This test guards both paths against regression.
func TestHandleRelatedCheckBatch_ResourceIDs_InsertPath(t *testing.T) {
	res := fakeEC2Resources()[0]
	c := newControllerAtDetail(res, "ec2")

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
				Result: resource.RelatedCheckResult{
					Count:      1,
					TargetType: "iam-role",
					ResourceIDs: []string{
						"arn:aws:iam::123456789012:role/test-role",
					},
				},
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

// ─────────────────────────────────────────────────────────────────────────────
// Fix 4: ActionLoadMore must carry ParentContext and FetchFilter in payload
// ─────────────────────────────────────────────────────────────────────────────

// TestActionLoadMore_ChildList_PayloadCarriesParentContext verifies that when
// ActionLoadMore is applied to a list with ParentContext set (child list),
// the returned FetchMorePayload carries that ParentContext.
//
// Pre-fix failure: the FetchMorePayload was constructed with only
// ContinuationToken.  ParentContext and FetchFilter were omitted, so the
// executor routed the request to the top-level fetcher instead of the child
// fetcher, returning incorrect results.
func TestActionLoadMore_ChildList_PayloadCarriesParentContext(t *testing.T) {
	c := newListController("ec2")

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
//
// Pre-fix failure: same as TestActionLoadMore_ChildList_PayloadCarriesParentContext —
// FetchFilter was omitted, causing the executor to call the wrong fetcher for
// filtered lists (e.g., related-navigation filtered by vpc-id).
func TestActionLoadMore_FilteredList_PayloadCarriesFetchFilter(t *testing.T) {
	c := newListController("ec2")

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
// with only the ContinuationToken set and both maps nil/empty.  This is the
// baseline case — it must still work correctly after the fix.
func TestActionLoadMore_NoContext_PayloadHasOnlyToken(t *testing.T) {
	c := newListController("ec2")

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
	// Plain list: neither context map should be populated.
	if len(payload.ParentContext) != 0 {
		t.Errorf("FetchMorePayload.ParentContext non-empty on plain list: %v", payload.ParentContext)
	}
	if len(payload.FetchFilter) != 0 {
		t.Errorf("FetchMorePayload.FetchFilter non-empty on plain list: %v", payload.FetchFilter)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Fix 5: applyRelatedNavResult must emit FetchFilteredPayload on KindFetchFiltered
// ─────────────────────────────────────────────────────────────────────────────

// TestActionSelect_RelatedNav_FetchFilter_TaskCarriesPayload verifies that when
// ActionSelect on a focused related row resolves to a NavigationKindFilteredList
// with FetchFilter, the returned tasks include KindFetchFiltered with a non-nil
// FetchFilteredPayload.Filter.
//
// Strategy: use newControllerAtDetail with a known resource, seed a related row
// with FetchFilter set and Count>1 (so the row resolves via
// NavigationKindFilteredList rather than the single-resource fast-path).  Enable
// RelatedFocus, call ActionSelect, and assert the returned tasks include a
// payload-bearing KindFetchFiltered task.
//
// Pre-fix failure: HandleRelatedNavigate emitted a KindFetchFiltered task with
// nil Payload.  ExecuteTask type-asserted payload to FetchFilteredPayload and got
// a zero-value struct with nil Filter, so the filtered fetcher received no filter
// and returned all resources (or errored), leaving the list empty.
func TestActionSelect_RelatedNav_FetchFilter_TaskCarriesPayload(t *testing.T) {
	res := fakeEC2Resources()[0]
	c := newControllerAtDetail(res, "ec2")

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

	// Enable related focus so ActionSelect navigates via the related row.
	c.Apply(app.Action{Kind: app.ActionToggleFocus}) //nolint:ineffassign,staticcheck // focus state observed via Snapshot

	snap = c.Snapshot()
	if snap.Body.Detail == nil || !snap.Body.Detail.RelatedFocused {
		t.Skip("related panel did not accept focus — cannot test related navigation in this environment")
	}

	// ActionSelect on the focused row triggers HandleRelatedNavigate →
	// applyRelatedNavResult.  With FetchFilter set and Count>1, the result is
	// NavigationKindFilteredList → KindFetchFiltered with FetchFilteredPayload.
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

// TestRelatedNav_MultiID_SeedsRelatedIDSet verifies the [P2] fix: navigating a
// related row that carries multiple ResourceIDs and NO FetchFilter (e.g. an EC2
// instance → its several security groups) resolves to NavigationKindFilteredList
// and seeds the pushed list's RelatedIDSet to exactly those IDs — so the list
// renders only the related subset (list.go's RelatedIDSet prefilter), not every
// resource of the target type. Both the mouse click (ActionRelatedSelect) and
// the keyboard Enter (ActionSelect) paths must seed it identically, since Fix #6
// routes both through the shared dispatchRelatedNavigate.
//
// Pre-fix, applyRelatedNavResult's filtered-list branch handled FilterText and
// FetchFilter but never seeded RelatedIDSet for the multi-ID case, so the web
// showed all resources of the type.
func TestRelatedNav_MultiID_SeedsRelatedIDSet(t *testing.T) {
	ids := []string{"sg-aaa111", "sg-bbb222", "sg-ccc333"}
	c := newControllerAtDetail(fakeEC2Resources()[0], "ec2")
	c.ApplyDetailRelated([]app.DetailRelatedRow{{
		TargetType:  "sg",
		DisplayName: "Security Groups",
		Count:       len(ids), // >1 → filtered list, not the single-resource fast-path
		ResourceIDs: ids,
		// no FetchFilter → the multi-ID subset path
	}})

	// Click the related row. Fix #6 routes the click (ActionRelatedSelect) and
	// the keyboard Enter (ActionSelect) through the same dispatchRelatedNavigate,
	// so this exercises the shared applyRelatedNavResult seeding that both use.
	vs, _ := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})

	// [Codex P2] Apply must return the POST-navigation snapshot. c.snapshot() now
	// runs AFTER dispatchRelatedNavigate pushes the filtered list; pre-fix the
	// snapshot was taken first (Go evaluates return operands left-to-right), so a
	// caller trusting Apply's ViewState got the stale source detail.
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

// ─────────────────────────────────────────────────────────────────────────────
// Fix 6: live connect — not tested (see rationale)
// ─────────────────────────────────────────────────────────────────────────────

// TestLiveConnect_NotTested documents why Fix 6 has no automated unit test.
//
// The live-connect branch in construct.go initiates a real TCP connection to
// AWS endpoints.  There is no injectable failure seam (dial func, transport
// override) that would let a unit test exercise the "connection failed →
// controller stays on menu" path without real credentials or network access.
// A test that opens a live connection would be flaky in CI (network-dependent,
// credential-dependent, timing-sensitive).
//
// The demo-mode path exercised by TestHeadless_FetchPopulatesListRows
// (headless_drain_test.go) is the structurally parallel path: controller
// initialises with fake clients, tasks execute synchronously, and list rows
// are populated.  If a dial-failure injection seam is added later, a unit test
// that asserts Body.Kind==BodyKindMenu after a failing connect (no panic, no
// hang, no goroutine leak) should be written here.
func TestLiveConnect_NotTested(t *testing.T) {
	t.Skip("live-connect fix requires real AWS credentials; no injectable failure seam — see comment")
}
