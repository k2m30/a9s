// session_rowstore_truncated_count_shrink_test.go pins a live cross-surface
// defect: the eni menu badge shows "58" while the eni list's own title
// suffix shows "36+" after a truncated page-1 fetch lands over a wider
// already-known TotalCount.
//
// Root cause: RowStore.Observe (core/session/rowstore.go) unconditionally
// sets `next.TotalCount = len(newRows)` on every accepted rows-carrying
// write, regardless of the existing entry's TotalCount. C6a already
// documents "a counts-only observation updates TotalCount without touching
// Rows" and C5 documents "exact only ever advances" for the Pagination-based
// stale-replace guard, but neither rule is applied to TotalCount itself
// inside Observe: a wider TotalCount seeded by ObserveCount (or by an
// earlier, fuller Observe) is silently overwritten the moment ANY later
// rows-carrying Observe lands — even a truncated page that is explicitly NOT
// claiming to be the whole list. This is the same "never shrink a known
// count on a non-exact result" principle already enforced for the root menu
// badge in Controller.syncExactTotalToMenu (core/app/handle.go) and for
// the MenuState.Availability guard, just missing at the RowStore layer.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// TestRowStore_Observe_TruncatedPageDoesNotShrinkWiderKnownTotalCount pins
// Pin B: a truncated incoming page (36 rows, IsTruncated=true) must not
// shrink an existing, wider known TotalCount (58) seeded by an earlier
// ObserveCount — a truncated fetch page explicitly does not claim to be the
// complete list, so it must never be treated as authoritative proof the
// total shrank.
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

// TestRowStore_Observe_ExactPageIsAllowedToShrinkTotalCount is the companion
// non-regression: an EXACT (untruncated) fetch page is allowed to shrink
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
