package unit

// wipfix_list_lifecycle_test.go — rows 2, 3 and 4: who a fetch belongs to, and
// which activity flag it owns.
//
// Row 2: a client-side related list (a detail's related panel handing a set of
// IDs to a type whose scan was truncated) has neither a ParentContext nor a
// FetchFilter — only EscPops says it is not the type's canonical list. A
// continuation that re-derives its provenance from those two empty maps calls
// itself a canonical-list fetch, and handle.go's symmetric gate then refuses
// to land it on the very screen that asked for it.
//
// Row 3: Ctrl+R while a load-more is outstanding supersedes the continuation.
// The discard happens before any flag is cleared, and the refresh clears only
// Loading/Refreshing — so LoadingMore stays true and the m key is dead for the
// rest of the session.
//
// Row 4: an exact related-ID drill with no cached match but another cached page
// begins with a KindFetchMore. Nothing raised LoadingMore for it (the screen is
// in its initial Loading), yet its result carries Append=true and clears
// LoadingMore, leaving the fetched rows behind a loading screen.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws" // resource-type registrations
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// wipfixRelatedDrillList pushes a client-side related ec2 list on top of a
// detail view, exactly as selecting a related-panel row does: EscPops is set,
// and neither ParentContext nor FetchFilter is ever populated. Returns nothing
// — the drill list is the controller's top screen afterwards.
func wipfixRelatedDrillList(t *testing.T, ctrl *app.Controller, ids []string) {
	t.Helper()
	ctrl.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	ctrl.EnsureDetailState(resource.Resource{ID: "sg-wipfix-src", Name: "sg-wipfix-src", Type: "sg"}, "sg")
	ctrl.ApplyDetailRelated([]app.DetailRelatedRow{
		{TargetType: provenancePinType, DisplayName: "EC2 Instances", Count: len(ids), ResourceIDs: ids},
	})
	ctrl.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})
	if !ctrl.GetListEscPops() {
		t.Fatal("test setup problem: the related drill list must have EscPops=true")
	}
}

// wipfixFetchMoreTask returns the KindFetchMore payload in tasks.
func wipfixFetchMoreTask(t *testing.T, tasks []runtime.TaskRequest) (runtime.TaskRequest, runtime.FetchMorePayload) {
	t.Helper()
	for _, tr := range tasks {
		if tr.Key.Kind != runtime.KindFetchMore {
			continue
		}
		p, ok := tr.Payload.(runtime.FetchMorePayload)
		if !ok {
			t.Fatalf("KindFetchMore task carries %T, want runtime.FetchMorePayload", tr.Payload)
		}
		return tr, p
	}
	t.Fatalf("no KindFetchMore task among %d tasks", len(tasks))
	return runtime.TaskRequest{}, runtime.FetchMorePayload{}
}

// ---------------------------------------------------------------------------
// Row 2 — a continuation belongs to the list that issued it
// ---------------------------------------------------------------------------

// TestLoadMore_OnClientSideRelatedList_AppendsToThatList pins row 2: the
// continuation a related drill issues must carry the drill's own lane, so its
// result lands on the drill and appends there.
func TestLoadMore_OnClientSideRelatedList_AppendsToThatList(t *testing.T) {
	ctrl, core, profile, region := newProvenancePinController(t)
	seedCanonical200(t, ctrl, core, profile, region)
	wipfixRelatedDrillList(t, ctrl, []string{"i-drill-0", "i-drill-1"})

	// The drill's first page: two rows and more to come.
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(1, "i-drill"),
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "page-2"},
		Provenance:   messages.FetchProvenanceFilteredList,
	})
	if got := len(ctrl.Snapshot().Body.List.Rows); got != 1 {
		t.Fatalf("precondition: drill list has %d rows, want 1", got)
	}

	_, tasks := ctrl.Apply(app.Action{Kind: app.ActionLoadMore})
	task, payload := wipfixFetchMoreTask(t, tasks)
	if payload.Provenance.CanonicalList() {
		t.Errorf("the continuation calls itself a canonical-list fetch — its owner is a related drill (EscPops), and handle.go's symmetric gate refuses a canonical result on a non-canonical screen")
	}
	if task.ListSeq != 0 {
		t.Errorf("the continuation was stamped list sequence %d — only a canonical-list fetch takes one, or a drill's load-more supersedes the verification of the list beneath it", task.ListSeq)
	}

	// The continuation's own result, carrying the lane the task recorded.
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(2, "i-drill")[1:],
		Append:       true,
		LoadingMore:  !payload.ContinuesInitialLoad,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   payload.Provenance,
		ListSeq:      task.ListSeq,
	})

	body := ctrl.Snapshot().Body.List
	if len(body.Rows) != 2 {
		t.Errorf("drill list has %d rows after load-more, want 2 — the second page never landed on the list that asked for it", len(body.Rows))
	}
	if body.LoadingMore {
		t.Errorf("the drill list is still marked loading-more after its own continuation landed")
	}
	assertStillCanonical200(t, ctrl, core, profile, region, "related-drill load-more")
}

// ---------------------------------------------------------------------------
// Row 3 — a superseded request retires its own flag
// ---------------------------------------------------------------------------

// TestRefreshDuringLoadMore_LeavesLoadMoreUsable pins row 3: when Ctrl+R
// supersedes an outstanding continuation, the continuation's discarded result
// must still retire the flag it raised, or the m key never works again.
func TestRefreshDuringLoadMore_LeavesLoadMoreUsable(t *testing.T) {
	ctrl, core, profile, region := newProvenancePinController(t)
	_ = profile
	_ = region
	_ = core
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: provenancePinType})
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(3, "i-page1"),
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "page-2"},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	_, moreTasks := ctrl.Apply(app.Action{Kind: app.ActionLoadMore})
	moreTask, morePayload := wipfixFetchMoreTask(t, moreTasks)
	if !ctrl.Snapshot().Body.List.LoadingMore {
		t.Fatal("precondition: the list must be marked loading-more after the m key")
	}

	// Ctrl+R while the continuation is still outstanding: a newer canonical
	// list fetch is dispatched, superseding it.
	ctrl.Apply(app.Action{Kind: app.ActionRefresh})

	// The superseded continuation lands and is discarded.
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(3, "i-page2"),
		Append:       true,
		LoadingMore:  !morePayload.ContinuesInitialLoad,
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "page-3"},
		Provenance:   morePayload.Provenance,
		ListSeq:      moreTask.ListSeq,
	})

	if ctrl.Snapshot().Body.List.LoadingMore {
		t.Errorf("the list is still marked loading-more after its continuation was discarded as superseded — nothing else will ever clear it")
	}
	if _, tasks := ctrl.Apply(app.Action{Kind: app.ActionLoadMore}); len(tasks) == 0 {
		t.Errorf("the m key produced no task after a refresh superseded the previous continuation — load-more is dead for the rest of the session")
	}
}

// ---------------------------------------------------------------------------
// Row 4 — a completion clears the flag its own request raised
// ---------------------------------------------------------------------------

// TestExactRelatedDrill_StartingWithFetchMore_RendersItsRows pins row 4: a
// drill that opens with a continuation (its target is not cached, but the
// cached page it would come after is) raised Loading, not LoadingMore — so the
// result must retire Loading, whatever Append says about how the rows merge.
func TestExactRelatedDrill_StartingWithFetchMore_RendersItsRows(t *testing.T) {
	ctrl, core, profile, region := newProvenancePinController(t)
	_ = core
	_ = profile
	_ = region

	// A canonical page that is truncated: the RowStore entry has more pages,
	// which is what makes the drill below start with a continuation.
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: provenancePinType})
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(3, "i-cached"),
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "page-2"},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	// The drill's targets are NOT among the cached rows. Two of them: a
	// single-ID drill resolves to a by-ID detail fetch instead, which is a
	// different lane entirely.
	const target = "i-beyond-0"
	wipfixRelatedDrillList(t, ctrl, []string{target, "i-beyond-1"})

	body := ctrl.Snapshot().Body.List
	if body == nil {
		t.Fatal("test setup problem: the drill did not push a list screen")
	}
	if !body.Loading {
		t.Fatalf("precondition: the drill list opens in its initial load (Loading), got Loading=%v LoadingMore=%v", body.Loading, body.LoadingMore)
	}

	// The continuation lands: Append=true, because that is how the rows merge.
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    []resource.Resource{{ID: target, Name: target, Type: provenancePinType}},
		Append:       true,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceFilteredList,
	})

	body = ctrl.Snapshot().Body.List
	if body.Loading {
		t.Errorf("the drill list is still loading after its own fetch landed — the completion cleared LoadingMore, which nothing had raised, and left the loading screen over the rows it just fetched")
	}
	if len(body.Rows) != 1 || body.Rows[0].ResourceID != target {
		t.Errorf("the drill list shows %d rows, want the one target %q", len(body.Rows), target)
	}
}
