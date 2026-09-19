package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// Two writers touch one TypeFile for a single ResourcesLoaded event: the
// synchronous row save writes Rows and Count from the result, and the menu
// availability save queued by the same event writes the menu's count
// asynchronously. If the menu count disagrees with the rows, whichever writer
// lands last leaves a TypeFile whose Rows and Count contradict each other,
// and the next cold boot badges it wrong.
//
// Close flushes the pending snapshot, which makes the write order
// deterministic here.
func TestPendingAvailabilitySave_NeverResurrectsASupersededCount(t *testing.T) {
	ctrl, core, profile, region := newProvenancePinController(t)
	seedCanonical200(t, ctrl, core, profile, region)

	handlePage(ctrl, messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(5, "i-refresh"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	if tf := provenancePinReadTypeFile(t, ctrl, profile, region, provenancePinType); len(tf.Rows) != 5 || tf.Count != 5 {
		t.Fatalf("precondition: TypeFile right after the second result = %d rows, Count=%d, want 5/5", len(tf.Rows), tf.Count)
	}

	// Flush the writer: every snapshot queued before this point is now on
	// disk, in the order the goroutine would have written it anyway.
	ctrl.Close()

	tf := provenancePinReadTypeFile(t, ctrl, profile, region, provenancePinType)
	if len(tf.Rows) != 5 || tf.Count != 5 {
		t.Errorf("TypeFile after the pending availability save landed = %d rows, Count=%d, want 5/5 — the availability writer put back a count the same event had already replaced", len(tf.Rows), tf.Count)
	}
	if snap := core.Session().RowStore.Snapshot(provenancePinType); len(snap.Rows) != 5 || snap.TotalCount != 5 {
		t.Errorf("in-memory RowStore = %d rows, TotalCount=%d, want 5/5", len(snap.Rows), snap.TotalCount)
	}
}

// One canonical, untruncated list result is the authoritative answer for its
// type: 5 rows means the account has 5, so it lowers the menu count. A
// truncated or partial observation only raises it — a first page of 50 must
// not lower a known total of 200.
//
// The count is more than a badge: persistMenuAvailabilityCache hands it to
// the availability writer, which writes it into the TypeFile whose Rows the
// same event replaced.
func TestExactCanonicalShrink_LowersTheMenuCount(t *testing.T) {
	ctrl, core, profile, region := newProvenancePinController(t)
	_, _ = profile, region
	seedCanonical200(t, ctrl, core, profile, region)

	handlePage(ctrl, messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(5, "i-refresh"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	if snap := core.Session().RowStore.Snapshot(provenancePinType); snap.TotalCount != 5 {
		t.Fatalf("precondition: RowStore TotalCount = %d, want 5", snap.TotalCount)
	}
	if got := ctrl.GetMenuAvailability()[provenancePinType]; got != 5 {
		t.Errorf("menu availability for %s = %d after an exact canonical result of 5 rows, want 5 — the row list and the badge disagree in the same session", provenancePinType, got)
	}
}

// The count and its lower-bound marker are one fact and move together. A
// stored 200 from a truncated page means "at least 200"; a later result that
// reached the end of the type and found 5 makes the count exactly 5. Leaving
// the marker set would render "5+" for a list enumerated to the end.
func TestExactCanonicalShrink_ClearsTheLowerBoundMarker(t *testing.T) {
	ctrl, _, _, _ := newProvenancePinController(t)
	ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: provenancePinType})

	handlePage(ctrl, messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(200, "i-canon"),
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-more"},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})
	if got := ctrl.GetMenuAvailability()[provenancePinType]; got != 200 {
		t.Fatalf("precondition: menu availability = %d, want 200", got)
	}
	if !ctrl.GetMenuTruncated()[provenancePinType] {
		t.Fatal("precondition: the seeded count must be a truncated lower bound")
	}

	handlePage(ctrl, messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(5, "i-refresh"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	if got := ctrl.GetMenuAvailability()[provenancePinType]; got != 5 {
		t.Errorf("menu availability = %d after an exact result of 5 rows, want 5", got)
	}
	if ctrl.GetMenuTruncated()[provenancePinType] {
		t.Error("menu truncated = true after an exact result, want false — the list was enumerated to the end, so the count is not a lower bound")
	}
}
