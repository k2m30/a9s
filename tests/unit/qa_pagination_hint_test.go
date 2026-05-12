package unit

// qa_pagination_hint_test.go — TDD tests for the "load more" hint text
// shown at the bottom of a paginated resource list.
//
// Tests verify three hint variants:
//   1. Standard hint when no filter is active: "m: load more"
//   2. Filter-aware hint when a filter is set: "m: load more (filter applies to loaded data only)"
//   3. Loading state while fetching next page: "loading..." (not "m: load more")
//
// Test 1 and 3 exercise existing behaviour and should PASS immediately.
// Test 2 documents new behaviour (T012) and is EXPECTED TO FAIL until the
// filter-aware hint is implemented in ResourceListModel.View().

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// newHintTestResourceList returns a ResourceListModel loaded with n resources
// and IsTruncated=true so the "load more" hint area is active.
func newHintTestResourceList(n int) views.ResourceListModel {
	k := keys.Default()
	typeDef := resource.ResourceTypeDef{
		Name:      "Hint Test",
		ShortName: "hint-test",
		Columns: []resource.Column{
			{Key: "id", Title: "ID", Width: 20},
			{Key: "name", Title: "Name", Width: 30},
		},
	}
	rl := views.NewResourceList(typeDef, nil, k)
	rl.SetSize(80, 20)

	resources := make([]resource.Resource, n)
	for i := range n {
		resources[i] = resource.Resource{
			ID:   "r-0001",
			Name: "res-item",
			Fields: map[string]string{
				"id":   "r-0001",
				"name": "res-item",
			},
		}
	}

	rl, _ = rl.Update(messages.ResourcesLoaded{
		ResourceType: "hint-test",
		Resources:    resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "next-page-token",
			PageSize:    n,
		},
		Append: false,
	})
	return rl
}

// ---------------------------------------------------------------------------
// TestQA_PaginationHint_NoFilter_ShowsStandard
// ---------------------------------------------------------------------------

// TestQA_PaginationHint_NoFilter_ShowsStandard verifies that when a resource
// list has a truncated pagination state and NO filter is active, the hint line
// contains "m: load more" but does NOT contain the filter-warning suffix.
//
// This test exercises existing behaviour and should PASS immediately.
func TestQA_PaginationHint_NoFilter_ShowsStandard(t *testing.T) {
	rl := newHintTestResourceList(5)

	output := stripANSI(rl.View())

	if !strings.Contains(output, "m: load more") {
		t.Errorf("expected hint to contain 'm: load more' with no active filter, got:\n%s", output)
	}

	if strings.Contains(output, "filter applies to loaded data only") {
		t.Errorf("expected NO filter warning when no filter is active, but got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// TestQA_PaginationHint_FilterActive_ShowsFilterWarning
// ---------------------------------------------------------------------------

// TestQA_PaginationHint_FilterActive_ShowsFilterWarning verifies that when a
// resource list has a truncated pagination state AND an active filter, the hint
// line contains the filter-aware warning so users understand the filter only
// applies to already-loaded data, not the unfetched pages.
//
// This test documents new behaviour (T012) and is EXPECTED TO FAIL until the
// filter-aware hint is implemented.
func TestQA_PaginationHint_FilterActive_ShowsFilterWarning(t *testing.T) {
	rl := newHintTestResourceList(5)

	// Apply a filter — this simulates the user typing "/" then "res".
	rl.SetFilter("res")

	output := stripANSI(rl.View())

	if !strings.Contains(output, "m: load more") {
		t.Errorf("expected hint to contain 'm: load more' even with active filter, got:\n%s", output)
	}

	// KEY ASSERTION: the filter warning must be appended to the hint.
	if !strings.Contains(output, "filter applies to loaded data only") {
		t.Errorf("expected hint to contain 'filter applies to loaded data only' when filter is active and list is truncated, got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// TestQA_PaginationHint_Loading_ShowsLoadingText
// ---------------------------------------------------------------------------

// TestQA_PaginationHint_Loading_ShowsLoadingText verifies that while the next
// page is being fetched (after pressing M), the hint line shows "loading..."
// and does NOT show "m: load more".
//
// This test exercises existing behaviour and should PASS immediately.
func TestQA_PaginationHint_Loading_ShowsLoadingText(t *testing.T) {
	rl := newHintTestResourceList(5)

	// Send the "M" key press to trigger loadingMore=true.
	// key.Matches in Update uses the text/code of the KeyPressMsg against the LoadMore binding.
	rl, _ = rl.Update(rootKeyPress("M"))

	output := stripANSI(rl.View())

	if !strings.Contains(output, "loading...") {
		t.Errorf("expected hint to contain 'loading...' after pressing M, got:\n%s", output)
	}

	// The "m: load more" prompt must be hidden while loading is in progress.
	// Strip "loading..." occurrences first to avoid false substring matches.
	withoutLoading := strings.ReplaceAll(output, "loading...", "")
	if strings.Contains(withoutLoading, "m: load more") {
		t.Errorf("expected 'm: load more' to be hidden while loading, but it was still present:\n%s", output)
	}
}
