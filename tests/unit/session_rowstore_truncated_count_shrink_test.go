// RowStore.Observe never
// shrinks a known TotalCount on a non-exact result, the principle
// Controller.syncExactTotalToMenu (core/app/handle.go) and the
// MenuState.Availability guard apply to the root menu badge. Otherwise the eni
// menu badge shows "58" while the eni list's title suffix shows "36+" after a
// truncated page-1 fetch lands over a wider already-known TotalCount.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// TestRowStore_Observe_TruncatedPageDoesNotShrinkWiderKnownTotalCount: a
// truncated incoming page (36 rows, IsTruncated=true) leaves a wider known
// TotalCount (58) seeded by an earlier ObserveCount in place — a truncated
// page does not claim to be the complete list.
func TestRowStore_Observe_TruncatedPageDoesNotShrinkWiderKnownTotalCount(t *testing.T) {
	store := session.NewRowStore()

	seedRows := make([]resource.Resource, 36)
	for i := range seedRows {
		seedRows[i] = resource.Resource{ID: "eni-" + itoaTest(i), Type: "eni"}
	}
	store.Observe("eni", seedRows, &resource.PaginationMeta{IsTruncated: true}, session.OriginProbe, false)
	store.ObserveCount("eni", 58)

	precondition := store.Snapshot("eni")
	if precondition.TotalCount != 58 {
		t.Fatalf("precondition failed: TotalCount = %d, want 58", precondition.TotalCount)
	}

	page1 := make([]resource.Resource, 36)
	for i := range page1 {
		page1[i] = resource.Resource{ID: "eni-" + itoaTest(i), Type: "eni"}
	}
	store.Observe("eni", page1, &resource.PaginationMeta{IsTruncated: true}, session.OriginFetch, false)

	snap := store.Snapshot("eni")
	if snap.TotalCount != 58 {
		t.Errorf("TotalCount = %d, want 58 (unchanged) — a truncated 36-row page must not shrink a wider already-known TotalCount (menu \"58\" vs list \"36+\" drift)", snap.TotalCount)
	}
	if len(snap.Rows) != 36 {
		t.Errorf("len(Rows) = %d, want 36 — the truncated page's rows must still be accepted", len(snap.Rows))
	}
}

// TestRowStore_Observe_ExactPageIsAllowedToShrinkTotalCount: an EXACT
// (untruncated) fetch page is allowed to shrink
// TotalCount below a prior wider known count — resources can genuinely be
// deleted between observations, and an exact result IS authoritative proof
// of the new total, unlike a truncated page.
func TestRowStore_Observe_ExactPageIsAllowedToShrinkTotalCount(t *testing.T) {
	store := session.NewRowStore()

	seedRows := make([]resource.Resource, 36)
	for i := range seedRows {
		seedRows[i] = resource.Resource{ID: "eni-" + itoaTest(i), Type: "eni"}
	}
	store.Observe("eni", seedRows, &resource.PaginationMeta{IsTruncated: true}, session.OriginProbe, false)
	store.ObserveCount("eni", 58)

	precondition := store.Snapshot("eni")
	if precondition.TotalCount != 58 {
		t.Fatalf("precondition failed: TotalCount = %d, want 58", precondition.TotalCount)
	}

	exactPage := make([]resource.Resource, 36)
	for i := range exactPage {
		exactPage[i] = resource.Resource{ID: "eni-" + itoaTest(i), Type: "eni"}
	}
	store.Observe("eni", exactPage, &resource.PaginationMeta{IsTruncated: false}, session.OriginFetch, false)

	snap := store.Snapshot("eni")
	if snap.TotalCount != 36 {
		t.Errorf("TotalCount = %d, want 36 — an EXACT fetch result is authoritative and IS allowed to shrink a prior wider known count (resources really got deleted)", snap.TotalCount)
	}
}
