//go:build integration

package integration

// scenario_truncated_lower_bound_test.go — the rendered witness for a list read
// in part.
//
// The related panel distinguishes three answers, and only two of them had a
// screen on the demo bench: an exact count, and a question mark for a list
// nobody read. The third — a resolved lower bound, "what matched is real and a
// page nobody read may hold more" — had none, because every demo list fitted in
// its first page. These walks pin it on both sides: a pivot that matched inside
// the partial list, and one that matched nothing in it. Without both, a
// regression to a bare count or back to a question mark changes no pixel.

import (
	"testing"

	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
)

func TestScenario_TruncatedListRendersALowerBound(t *testing.T) {
	total := len(demofixtures.NewCWLogsFixtures().LogGroups)
	if total <= demofixtures.LogGroupsPageSize {
		t.Fatal("no demo list is paginated; the lower bound has no rendered witness")
	}

	s := fullIntegrationNewDemoScenario(t)

	// The list frame reports the page it read, marked as a lower bound.
	s.OpenList("logs")
	s.ExpectFrameContains(fullIntegrationFrameCount("logs", fullIntegrationCountExpectation{
		count:     demofixtures.LogGroupsPageSize,
		truncated: true,
	}))

	// A pivot whose groups are on the page that was read: real matches, still a
	// lower bound because a later page may name more.
	dbc := fullIntegrationMustFindResourceByID(t, s.clients, "dbc", "acme-docdb-prod")
	s.OpenDetailResource("dbc", dbc)
	s.ExpectRelatedCount("Log Groups", 2)
	s.ExpectViewContains("Log Groups (2+)")

	// A pivot whose groups are not on it: a zero that is real so far, which is
	// the answer a question mark would throw away.
	s.Back()
	unenc := fullIntegrationMustFindResourceByID(t, s.clients, "dbc", "warn-dbc-unenc")
	s.OpenDetailResource("dbc", unenc)
	s.ExpectRelatedCount("Log Groups", 0)
	s.ExpectViewContains("Log Groups (0+)")
}

// TestScenario_TruncatedListLoadMoreSettles pins the other end of the same
// fixture: the "+" is a promise the list can keep. Loading the next page brings
// the rest and the frame stops marking a lower bound, so the marker reports a
// state the user can leave rather than a permanent decoration.
func TestScenario_TruncatedListLoadMoreSettles(t *testing.T) {
	total := len(demofixtures.NewCWLogsFixtures().LogGroups)
	if total <= demofixtures.LogGroupsPageSize {
		t.Fatal("no demo list is paginated; there is nothing to load more of")
	}

	s := fullIntegrationNewDemoScenario(t)
	s.OpenList("logs")
	s.ExpectFrameContains(fullIntegrationFrameCount("logs", fullIntegrationCountExpectation{
		count:     demofixtures.LogGroupsPageSize,
		truncated: true,
	}))

	s.LoadMore()

	s.ExpectFrameContains(fullIntegrationFrameCount("logs", fullIntegrationCountExpectation{
		count:     total,
		truncated: false,
	}))
}
