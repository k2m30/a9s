// list_one_apply_point_test.go — round 2 of listgen: one apply point, one
// stale decision, one population decision.
//
//  4. The list view does not apply list results. It used to, in its own
//     Update, unstamped and unreachable — a second apply point is exactly what
//     the request sequence forbids.
//  5. A superseded result is rejected once, by its sequence, on every lane. On
//     the headless lane the runtime's row-store write ran before the
//     controller's guard, so a superseded page still reached the shared store
//     and only a content heuristic partly caught it.
//  6. A seed never reports itself complete while holding fewer rows than the
//     population its source knows.
package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ───────────────────────────────────────────────────────────────────────────
// 4. The list view is not an apply point
// ───────────────────────────────────────────────────────────────────────────

// TestListView_DoesNotApplyResourcesLoaded pins that handing a list result
// straight to the view changes nothing. Every list result reaches the screen
// through the controller, where the request sequence is checked; a view that
// applies one itself would bypass that check with no way to see it.
func TestListView_DoesNotApplyResourcesLoaded(t *testing.T) {
	ctrl, _ := newDetailParityHeadlessController(t)
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: listGenType})

	td := resource.FindResourceType(listGenType)
	if td == nil {
		t.Fatalf("resource type %q is not registered", listGenType)
	}
	rl := views.NewResourceList(*td, nil, keys.Default(), ctrl)
	rl.SetSize(80, 24)

	rl.Update(messages.ResourcesLoaded{
		ResourceType: listGenType,
		Resources:    listGenRows(3, "i-view"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	if got := len(ctrl.GetListAllResources()); got != 0 {
		t.Fatalf("the list view applied a result on its own: %d rows landed on the screen, want 0", got)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// 5. A superseded result is rejected on every lane, by its sequence alone
// ───────────────────────────────────────────────────────────────────────────

// TestListFetch_SupersededResultNeverReachesRowStore drives the headless lane,
// where Controller.Handle runs the runtime's row-store write before its own
// guard. The superseded page carries entirely different rows of the same size,
// the shape no content heuristic can recognise, so the store is the only place
// the damage shows.
func TestListFetch_SupersededResultNeverReachesRowStore(t *testing.T) {
	ctrl, core := newDetailParityHeadlessController(t)

	_, entryTasks := ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: listGenType})
	entrySeq, entryScreen := listGenFetchSeq(t, entryTasks, "on-entry verification")
	_, refreshTasks := ctrl.Apply(app.Action{Kind: app.ActionRefresh})
	refreshSeq, refreshScreen := listGenFetchSeq(t, refreshTasks, "ctrl+R refresh")

	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: listGenType,
		Resources:    listGenRows(3, "i-refresh"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
		ListSeq:      refreshSeq,
		ScreenID:     refreshScreen,
	})
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: listGenType,
		Resources:    listGenRows(3, "i-entry"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
		ListSeq:      entrySeq,
		ScreenID:     entryScreen,
	})

	entry, ok := core.ResourceCache(listGenType)
	if !ok {
		t.Fatalf("the row store holds nothing for %s after two results landed", listGenType)
	}
	for _, r := range entry.Resources {
		if strings.HasPrefix(r.ID, "i-entry") {
			t.Fatalf("row %q from the superseded on-entry verification reached the shared row store", r.ID)
		}
	}
}

// TestListFetch_CtrlRResetWinsOverAnExactScreen is the case the content
// heuristic got wrong and the sequence gets right: a list paged to an exact
// total, then refreshed. The refresh legitimately replays page 1 — smaller,
// still truncated, every ID already on screen — which is precisely the shape
// the heuristic called stale. It is the newest request, so it wins.
func TestListFetch_CtrlRResetWinsOverAnExactScreen(t *testing.T) {
	ctrl, _ := newDetailParityHeadlessController(t)

	_, entryTasks := ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: listGenType})
	entrySeq, entryScreen := listGenFetchSeq(t, entryTasks, "on-entry verification")
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: listGenType,
		Resources:    listGenRows(6, "i-page"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
		ListSeq:      entrySeq,
		ScreenID:     entryScreen,
	})

	_, refreshTasks := ctrl.Apply(app.Action{Kind: app.ActionRefresh})
	refreshSeq, refreshScreen := listGenFetchSeq(t, refreshTasks, "ctrl+R refresh")
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: listGenType,
		Resources:    listGenRows(3, "i-page"),
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "next"},
		Provenance:   messages.FetchProvenanceCanonicalList,
		ListSeq:      refreshSeq,
		ScreenID:     refreshScreen,
	})

	if got := len(ctrl.GetListAllResources()); got != 3 {
		t.Fatalf("after Ctrl+R the list holds %d rows, want the refresh's 3 — the reset was discarded as a stale subset", got)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// 6. A seed never claims to be complete while it is short of the population
// ───────────────────────────────────────────────────────────────────────────

// TestListSeed_RowStoreBranchReportsTruncatedBelowPopulation covers the seed
// branch that reads the session's own retained rows. A counts-only observation
// can raise the population above an exact row set; the seed built from that
// entry must say it is truncated, or the screen renders a complete list that
// is missing rows.
func TestListSeed_RowStoreBranchReportsTruncatedBelowPopulation(t *testing.T) {
	_, core := newDetailParityHeadlessController(t)

	core.ObserveRows(listGenType, listGenRows(4, "i-live"), &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)
	core.ObserveCountRows(listGenType, 9)

	res, _ := core.HandleNavigate(runtime.NavigateEvent{
		Target:       runtime.NavigateTargetResourceList,
		ResourceType: listGenType,
	})
	if res.CachedEntry == nil {
		t.Fatalf("navigating to a retained list produced no seed")
	}
	if res.CachedEntry.Pagination == nil || !res.CachedEntry.Pagination.IsTruncated {
		t.Fatalf("a 4-row seed of a type known to have 9 reports itself complete: pagination %+v", res.CachedEntry.Pagination)
	}
}
