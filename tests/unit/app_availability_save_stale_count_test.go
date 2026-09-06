package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// TestPendingAvailabilitySave_NeverResurrectsASupersededCount pins that the
// queued menu-availability save cannot put a count back on disk that the very
// event which queued it has already replaced.
//
// Two writers touch one TypeFile for a single ResourcesLoaded event. The
// synchronous row save writes Rows and Count from the result itself (5/5).
// The menu availability save, queued by the same event, carries the menu's
// own count — which is still 200, because the availability map keeps the
// larger of what it knows and what it is told (see the sibling test below).
// That save is counts-only and asynchronous, so when its writer goroutine is
// scheduled after the row save it leaves a TypeFile whose Rows are the 5 and
// whose Count says 200: a stored pair that contradicts itself, and a menu
// badge that reads 200 for a list of 5 on the next cold boot.
//
// Close is what makes the timing deterministic here, not what causes the
// corruption: it flushes the snapshot that is already pending. Left to the
// scheduler the same write lands whenever the goroutine next runs, which is
// what makes TestObserveResourcesLoadedRows_CanonicalList_DoesReplace fail
// about one run in twenty under the full shuffled tree.
func TestPendingAvailabilitySave_NeverResurrectsASupersededCount(t *testing.T) {
	ctrl, core, profile, region := newProvenancePinController(t)
	seedCanonical200(t, ctrl, core, profile, region)

	ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: provenancePinType,
		Resources:    provenancePinEC2Rows(5, "i-refresh"),
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	// Precondition: the synchronous row save already agrees with itself.
	if tf := provenancePinReadTypeFile(t, profile, region, provenancePinType); len(tf.Rows) != 5 || tf.Count != 5 {
		t.Fatalf("precondition: TypeFile right after the second result = %d rows, Count=%d, want 5/5", len(tf.Rows), tf.Count)
	}

	// Flush the writer: every snapshot queued before this point is now on
	// disk, in the order the goroutine would have written it anyway.
	ctrl.Close()

	tf := provenancePinReadTypeFile(t, profile, region, provenancePinType)
	if len(tf.Rows) != 5 || tf.Count != 5 {
		t.Errorf("TypeFile after the pending availability save landed = %d rows, Count=%d, want 5/5 — the availability writer put back a count the same event had already replaced", len(tf.Rows), tf.Count)
	}
	if snap := core.Session().RowStore.Snapshot(provenancePinType); len(snap.Rows) != 5 || snap.TotalCount != 5 {
		t.Errorf("in-memory RowStore = %d rows, TotalCount=%d, want 5/5", len(snap.Rows), snap.TotalCount)
	}
}

// TestExactCanonicalShrink_LowersTheMenuCount pins the cause of the
// contradiction above.
//
// One canonical, untruncated list result is the authoritative answer for its
// type: 5 rows means the account has 5. The availability map has two writers
// and they do not agree on that. The probe lane states the rule and follows
// it — "an untruncated observation always wins" (core/app/intents.go:91-101).
// The list-open lane keeps the larger of what it knows and what it is told
// (core/app/handle.go:378), which is right for a truncated or partial
// observation — a first page of 50 must not lower a known total of 200 — and
// wrong for this one, which is neither.
//
// The stale count is not only a badge: persistMenuAvailabilityCache hands it
// to the availability writer, which writes it into the very TypeFile whose
// Rows the same event just replaced. Two writers, one file, opposite numbers,
// and the scheduler picks the winner.
func TestExactCanonicalShrink_LowersTheMenuCount(t *testing.T) {
	ctrl, core, profile, region := newProvenancePinController(t)
	_, _ = profile, region
	seedCanonical200(t, ctrl, core, profile, region)

	ctrl.Handle(messages.ResourcesLoaded{
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
