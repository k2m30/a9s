package unit_test

// demo_paginated_list_witness_test.go — the demo bench needs one list that does
// not fit on its first page.
//
// A list read in part is a resolved lower bound: what matched is real, and the
// truncation flag carries "there may be more". A rule that renders nowhere on
// the bench cannot be checked there: a regression back to a question mark, or
// to a bare count, would leave every screen identical. The log
// groups are the paginated one, and these pins hold the three facts the rendered
// witnesses rest on: the first page is short of the whole fixture set, it says
// so, and the count oracle reports that page rather than the fixture total.

import (
	"context"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestDemoLogGroups_FirstPageIsTruncated pins the fixture arrangement the
// lower-bound witnesses depend on. Nothing else on the bench pages, so if this
// list stops paging the "+" disappears from every screen at once.
func TestDemoLogGroups_FirstPageIsTruncated(t *testing.T) {
	fetch := resource.GetPaginatedFetcher("logs")
	if fetch == nil {
		t.Fatal("no paginated fetcher registered for logs")
	}
	total := len(demofixtures.NewCWLogsFixtures().LogGroups)
	if total <= demofixtures.LogGroupsPageSize {
		t.Fatalf("the log-group fixtures (%d) fit inside one page of %d, so no demo list is paginated and the lower bound has no witness",
			total, demofixtures.LogGroupsPageSize)
	}

	page, err := fetch(context.Background(), demo.NewServiceClients(), "")
	if err != nil {
		t.Fatalf("first page: %v", err)
	}

	if len(page.Resources) != demofixtures.LogGroupsPageSize {
		t.Errorf("first page holds %d groups, want %d", len(page.Resources), demofixtures.LogGroupsPageSize)
	}
	if page.Pagination == nil || !page.Pagination.IsTruncated {
		t.Fatalf("first page reports no truncation (%+v); the cache entry would read complete and every pivot into logs would render an exact count",
			page.Pagination)
	}
}

// TestDemoLogGroups_SecondPageCompletesTheList pins the other half: the pages
// add up. A first page that says "more" and a second that repeats it, or that
// returns nothing, would make the list unreadable rather than paginated.
func TestDemoLogGroups_SecondPageCompletesTheList(t *testing.T) {
	fetch := resource.GetPaginatedFetcher("logs")
	if fetch == nil {
		t.Fatal("no paginated fetcher registered for logs")
	}
	clients := demo.NewServiceClients()

	first, err := fetch(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	if first.Pagination == nil || first.Pagination.NextToken == "" {
		t.Fatal("first page carries no continuation token")
	}

	second, err := fetch(context.Background(), clients, first.Pagination.NextToken)
	if err != nil {
		t.Fatalf("second page: %v", err)
	}

	total := len(demofixtures.NewCWLogsFixtures().LogGroups)
	if got := len(first.Resources) + len(second.Resources); got != total {
		t.Errorf("the two pages hold %d groups, want %d — the fixture set is not covered exactly once",
			got, total)
	}

	// Page two holds the one group no pivot matches, and nothing else. Every
	// other group in the file is some pivot's witness, so a group that drifts
	// onto page two takes a count a scenario pins down with it.
	if len(second.Resources) != 1 || second.Resources[0].ID != demofixtures.LogGroupSecondPageOnly {
		var ids []string
		for _, r := range second.Resources {
			ids = append(ids, r.ID)
		}
		t.Errorf("page two holds %v, want only %q — any other group there costs its pivot a count",
			ids, demofixtures.LogGroupSecondPageOnly)
	}
	if second.Pagination != nil && second.Pagination.IsTruncated {
		t.Error("second page reports truncation; the list would never settle")
	}
}

// TestDemoCountsOracle_ReportsTheFirstPage pins what the oracle is for. The
// main menu and the list frame show the page the app has read, so an oracle
// counting the whole fixture set would disagree with every screen the moment a
// list pages.
func TestDemoCountsOracle_ReportsTheFirstPage(t *testing.T) {
	fetch := resource.GetPaginatedFetcher("logs")
	if fetch == nil {
		t.Fatal("no paginated fetcher registered for logs")
	}
	page, err := fetch(context.Background(), demo.NewServiceClients(), "")
	if err != nil {
		t.Fatalf("first page: %v", err)
	}

	counts := demofixtures.ExpectedTopLevelCountsForTest()
	got, ok := counts["logs"]
	if !ok {
		t.Fatal("no counts entry for logs")
	}
	if got != len(page.Resources) {
		t.Errorf("counts oracle says %d, first page holds %d", got, len(page.Resources))
	}
	if total := len(demofixtures.NewCWLogsFixtures().LogGroups); got == total {
		t.Errorf("counts oracle says %d, which is the whole fixture set — it must report the first page", got)
	}
}
