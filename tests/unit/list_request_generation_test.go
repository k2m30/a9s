// list_request_generation_test.go — behaviour pins for the per-list fetch
// request sequence and the re-entry re-verification, both raised by the
// 2026-09-08 cache review.
//
//  1. A list fetch carries the dispatch-order sequence its screen was at when
//     it was dispatched, and the apply point accepts only the latest one. The
//     reviewed defect: the only stale-result guard was a content heuristic
//     (a smaller, still-truncated, strict-ID-subset replace), so an on-entry
//     verification overlapping a later Ctrl+R could land AFTER the refresh and
//     replace the newer screen with its own older rows.
//  2. Re-entering a list that the row store still holds in full re-verifies it
//     on both adapters. The reviewed defect: HandleNavigate returned no task
//     for that branch and only the headless controller synthesised one, so the
//     TUI rendered retained rows with no refreshing marker and no AWS call.
//  3. A verify-refetch that comes back shallower than the type's known
//     population does not regress the rendered total.
package unit

import (
	"strconv"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

const listGenType = "ec2"

// listGenRows builds n synthetic ec2 rows whose IDs carry prefix, so two
// pages of the same size are still distinguishable by content.
func listGenRows(n int, prefix string) []resource.Resource {
	rows := make([]resource.Resource, n)
	for i := range rows {
		id := prefix + "-" + strconv.Itoa(i)
		rows[i] = resource.Resource{ID: id, Name: id, Type: listGenType}
	}
	return rows
}

// listGenFetchSeq returns the list-fetch sequence stamped on the first
// KindFetchResources task in tasks — what the executor echoes onto the
// messages.ResourcesLoaded that fetch eventually produces.
func listGenFetchSeq(t *testing.T, tasks []runtime.TaskRequest, what string) domain.Gen {
	t.Helper()
	for _, task := range tasks {
		if task.Key.Kind == runtime.KindFetchResources {
			if task.ListSeq == 0 {
				t.Fatalf("%s: KindFetchResources task carries ListSeq 0 — every canonical list fetch is stamped at dispatch", what)
			}
			return task.ListSeq
		}
	}
	t.Fatalf("%s: no KindFetchResources task dispatched, got %d task(s)", what, len(tasks))
	return 0
}

// seedListGenDiskFile writes the on-disk type file the reviewer described: a
// type whose population is known to be count while only rowCount rows fitted
// in the file.
func seedListGenDiskFile(t *testing.T, count, rowCount int) {
	t.Helper()
	stored := listGenRows(rowCount, "i-disk")
	rows := make([]cache.Row, len(stored))
	for i, r := range stored {
		rows[i] = cache.Row{ID: r.ID, Name: r.Name}
	}
	store := cache.LoadDirForTest("detail-parity-prof", "us-east-1")
	store.Put(listGenType, cache.TypeFile{
		HasResources: true,
		Count:        count,
		Exact:        true,
		Rows:         rows,
	})
	if err := store.SaveType(listGenType); err != nil {
		t.Fatalf("seeding the on-disk type file: %v", err)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// 1. The overlapped on-entry verification loses to the later Ctrl+R
// ───────────────────────────────────────────────────────────────────────────

// TestListFetch_StaleEntryVerificationLosesToLaterRefresh reproduces the
// reviewer's scenario end to end through the real Controller: opening a list
// dispatches its on-entry verification, the operator hits Ctrl+R before that
// result arrives, and the refresh completes FIRST. The verification's own
// result then lands last, carrying a full same-sized page of different rows —
// exactly the shape the content heuristic cannot recognise as stale.
//
// The newer screen must survive.
func TestListFetch_StaleEntryVerificationLosesToLaterRefresh(t *testing.T) {
	ctrl, _ := newDetailParityHeadlessController(t)

	_, entryTasks := ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: listGenType})
	entrySeq := listGenFetchSeq(t, entryTasks, "on-entry verification")

	_, refreshTasks := ctrl.Apply(app.Action{Kind: app.ActionRefresh})
	refreshSeq := listGenFetchSeq(t, refreshTasks, "ctrl+R refresh")

	if refreshSeq == entrySeq {
		t.Fatalf("refresh reused the entry verification's sequence %d — a later dispatch must outrank an earlier one", entrySeq)
	}

	// The refresh wins the race and lands first.
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: listGenType,
		Resources:    listGenRows(3, "i-refresh"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
		ListSeq:      refreshSeq,
	})

	// The superseded on-entry verification arrives afterwards.
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: listGenType,
		Resources:    listGenRows(3, "i-entry"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
		ListSeq:      entrySeq,
	})

	rows := ctrl.GetListAllResources()
	if len(rows) != 3 {
		t.Fatalf("list holds %d rows, want the refresh's 3", len(rows))
	}
	for _, r := range rows {
		if !strings.HasPrefix(r.ID, "i-refresh") {
			t.Fatalf("row %q came from the superseded on-entry verification — the older request replaced the newer screen", r.ID)
		}
	}
}

// TestListFetch_LatestResultStillApplies is the negated form: a result whose
// sequence IS the latest dispatch is applied, so the guard above cannot be
// satisfied by rejecting everything.
func TestListFetch_LatestResultStillApplies(t *testing.T) {
	ctrl, _ := newDetailParityHeadlessController(t)

	_, entryTasks := ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: listGenType})
	entrySeq := listGenFetchSeq(t, entryTasks, "on-entry verification")

	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: listGenType,
		Resources:    listGenRows(2, "i-entry"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
		ListSeq:      entrySeq,
	})

	if got := len(ctrl.GetListAllResources()); got != 2 {
		t.Fatalf("list holds %d rows, want the 2 the latest fetch returned", got)
	}
}

// ───────────────────────────────────────────────────────────────────────────
// 2. A re-entered list is re-verified on both adapters
// ───────────────────────────────────────────────────────────────────────────

// TestListReEntry_RuntimeReturnsVerificationTask pins the decision where it
// belongs: HandleNavigate itself returns the fetch task for the row-store hit,
// so no adapter has to synthesise one and both behave identically.
func TestListReEntry_RuntimeReturnsVerificationTask(t *testing.T) {
	ctrl, core := newDetailParityHeadlessController(t)

	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: listGenType})
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: listGenType,
		Resources:    listGenRows(4, "i-first"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	res, tasks := core.HandleNavigate(runtime.NavigateEvent{
		Target:       runtime.NavigateTargetResourceList,
		ResourceType: listGenType,
	})
	if res.Kind != runtime.NavigateKindPushResourceListCached {
		t.Fatalf("re-entry resolved to %v, want the row-store cache hit", res.Kind)
	}
	found := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.KindFetchResources {
			found = true
		}
	}
	if !found {
		t.Fatalf("re-opening a retained list returned %d task(s) and no KindFetchResources — the list is treated as permanently fresh", len(tasks))
	}
}

// TestListReEntry_HeadlessMarksRefreshing pins the headless lane's rendered
// consequence: the re-entered list carries the refreshing marker state.
func TestListReEntry_HeadlessMarksRefreshing(t *testing.T) {
	ctrl, _ := newDetailParityHeadlessController(t)

	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: listGenType})
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: listGenType,
		Resources:    listGenRows(4, "i-first"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	ctrl.Apply(app.Action{Kind: app.ActionBack})

	_, tasks := ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: listGenType})
	found := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.KindFetchResources {
			found = true
		}
	}
	if !found {
		t.Fatalf("headless re-entry dispatched no list fetch, got %d task(s)", len(tasks))
	}
	if ls := ctrl.Snapshot().Body.List; ls == nil || !ls.Refreshing {
		t.Fatalf("headless re-entry did not mark the list refreshing")
	}
}

// ───────────────────────────────────────────────────────────────────────────
// 3. A shallower verify-refetch does not regress the rendered total
// ───────────────────────────────────────────────────────────────────────────

// TestListTotal_ShallowVerifyKeepsKnownPopulation seeds the on-disk file the
// reviewer described — a type whose population is 55 while only 50 rows were
// stored — opens the list, and lands a still-truncated 50-row verify-refetch
// on it. The rendered total must stay the type's known population; regressing
// to the row depth is the "50+" frame the reviewer saw.
func TestListTotal_ShallowVerifyKeepsKnownPopulation(t *testing.T) {
	ctrl, _ := newDetailParityHeadlessController(t)

	seedListGenDiskFile(t, 55, 50)

	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: listGenType})
	if got := ctrl.Snapshot().FrameTitle; !strings.Contains(got, "55") {
		t.Fatalf("seeded list title %q does not show the known population 55", got)
	}

	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: listGenType,
		Resources:    listGenRows(50, "i-disk"),
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "next"},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	got := ctrl.Snapshot().FrameTitle
	if strings.Contains(got, "50+") || !strings.Contains(got, "55") {
		t.Fatalf("after a still-truncated 50-row verify-refetch the title is %q — the known population 55 was replaced by the row depth", got)
	}
}

// TestListTotal_ExactVerifyRetiresKnownPopulation is the other side of the
// retirement rule: an EXACT result is authoritative proof of the new total, so
// a type whose population was known to be 55 and now returns exactly 50 rows
// renders 50. Without this the seeded number would outlive the evidence
// against it.
func TestListTotal_ExactVerifyRetiresKnownPopulation(t *testing.T) {
	ctrl, _ := newDetailParityHeadlessController(t)
	seedListGenDiskFile(t, 55, 50)

	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: listGenType})
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: listGenType,
		Resources:    listGenRows(50, "i-disk"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	if got := ctrl.Snapshot().FrameTitle; got != "ec2(50)" {
		t.Fatalf("after an exact 50-row verify the title is %q, want ec2(50) — the seeded population outlived the evidence against it", got)
	}
}

// TestListTotal_VerifyReachingPopulationRetiresIt covers the boundary: a
// still-truncated result that has itself reached the seeded population has
// nothing left to preserve, so the screen goes back to reporting its own rows.
func TestListTotal_VerifyReachingPopulationRetiresIt(t *testing.T) {
	ctrl, _ := newDetailParityHeadlessController(t)
	seedListGenDiskFile(t, 55, 50)

	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: listGenType})
	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: listGenType,
		Resources:    listGenRows(55, "i-disk"),
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "next"},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	if got := ctrl.Snapshot().FrameTitle; got != "ec2(55+)" {
		t.Fatalf("after a 55-row truncated verify the title is %q, want ec2(55+)", got)
	}
}
