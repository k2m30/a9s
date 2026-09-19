package unit

// Pagination at the root model: a truncated
// first page renders "(N+)", load-more appends, re-entering a list restores
// its rows per resource type, and loadingMore clears on error and on
// re-entry.

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// ---------------------------------------------------------------------------
// Helpers: ct-events test resources
// ---------------------------------------------------------------------------

// ctEventsResources returns n ct-events resources with sequential IDs.
// Uses the _ct.* field schema (Status is severity-based, not ReadOnly).
// CreateBucket → W verb → "ct-attention".
// _ct.actor is set to "usr-NNNN" so it renders in the ACTOR column and can be
// used as a unique per-resource assertion target.
func ctEventsResources(n int) []resource.Resource {
	resources := make([]resource.Resource, n)
	for i := range n {
		id := fmt.Sprintf("evt-%04d", i)
		actor := fmt.Sprintf("usr-%04d", i)
		resources[i] = resource.Resource{
			ID:   id,
			Name: fmt.Sprintf("CreateBucket-%d", i),
			Fields: map[string]string{
				"event_name":    fmt.Sprintf("CreateBucket-%d", i),
				"time":          "2026-03-28 14:30:15",
				"event_time":    "2026-03-28 14:30:15",
				"user":          "admin",
				"source":        "s3.amazonaws.com",
				"resource_type": "",
				"resource_name": "",
				"read_only":     "false",
				// _ct.* fields the list columns read.
				"_ct.verb":    "W",
				"_ct.actor":   actor,
				"_ct.origin":  "CLI",
				"_ct.target":  "(none)",
				"_ct.outcome": "OK",
			},
		}
	}
	return resources
}

// ctEventsResources2 returns n additional ct-events resources whose IDs start
// at offset, so they can be distinguished from the first page.
// DeleteObject → D verb → "ct-danger".
func ctEventsResources2(n, offset int) []resource.Resource {
	resources := make([]resource.Resource, n)
	for i := range n {
		idx := offset + i
		id := fmt.Sprintf("evt-%04d", idx)
		actor := fmt.Sprintf("usr-%04d", idx)
		resources[i] = resource.Resource{
			ID:   id,
			Name: fmt.Sprintf("DeleteObject-%d", idx),
			Fields: map[string]string{
				"event_name":    fmt.Sprintf("DeleteObject-%d", idx),
				"time":          "2026-03-28 14:30:15",
				"event_time":    "2026-03-28 14:30:15",
				"user":          "admin",
				"source":        "s3.amazonaws.com",
				"resource_type": "",
				"resource_name": "",
				"read_only":     "false",
				// _ct.* fields the list columns read.
				"_ct.verb":    "D",
				"_ct.actor":   actor,
				"_ct.origin":  "CLI",
				"_ct.target":  "(none)",
				"_ct.outcome": "OK",
			},
		}
	}
	return resources
}

// ec2TestResources returns n ec2-like resources.
func ec2TestResources(n int) []resource.Resource {
	resources := make([]resource.Resource, n)
	for i := range n {
		id := fmt.Sprintf("i-%05d", i)
		resources[i] = resource.Resource{
			ID:   id,
			Name: fmt.Sprintf("web-server-%d", i),
			Fields: map[string]string{
				"instance_id":   id,
				"instance_type": "t3.micro",
				"state":         "running",
				"name":          fmt.Sprintf("web-server-%d", i),
			},
		}
	}
	return resources
}

// ---------------------------------------------------------------------------
// Initial truncated load shows "(50+)" in root rendered view
// ---------------------------------------------------------------------------

// TestQA_PaginationRoot_InitialLoadShowsTruncated verifies that when a paginated
// resource type loads its first page with IsTruncated=true, the root model's
// rendered frame title contains "50+" (not just "50").
func TestQA_PaginationRoot_InitialLoadShowsTruncated(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2-token",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false, Provenance: messages.FetchProvenanceCanonicalList,
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "50+") {
		t.Errorf("expected frame title to contain '50+' for truncated first page, but got:\n%s", plain)
	}

}

// ---------------------------------------------------------------------------
// Load more appends and updates the count to "(100)"
// ---------------------------------------------------------------------------

// TestQA_PaginationRoot_LoadMoreAppendsAndShowsUpdatedCount verifies that after
// pressing M and receiving a second page, the root model renders "(100)" in the
// frame title (no "+" because the last page was not truncated).
func TestQA_PaginationRoot_LoadMoreAppendsAndShowsUpdatedCount(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false, Provenance: messages.FetchProvenanceCanonicalList,
	})

	m, cmd := rootApplyMsg(m, rootKeyPress("M"))

	// The resource list view must return a non-nil command when M is pressed on
	// a truncated list (it produces a LoadMoreMsg).
	if cmd == nil {
		t.Fatal("pressing M on a truncated list at root level should return a non-nil command")
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources2(50, 50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			NextToken:   "",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: true, Provenance: messages.FetchProvenanceCanonicalList,
	})

	plain := stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "ct-events(100") {
		t.Errorf("after loading two pages, expected frame title 'ct-events(100...)', got:\n%s", plain)
	}

	if strings.Contains(plain, "100+") {
		t.Errorf("after loading final page, frame title must not contain '100+', got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Esc and re-enter preserves cached resources
// ---------------------------------------------------------------------------

// TestQA_PaginationRoot_EscAndReenter_PreservesCachedResources verifies that
// after loading resources (including pressing M), pressing Esc to return to
// the main menu, and re-entering the same resource type, the previously
// loaded 100 resources are restored.
func TestQA_PaginationRoot_EscAndReenter_PreservesCachedResources(t *testing.T) {

	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false, Provenance: messages.FetchProvenanceCanonicalList,
	})

	m, _ = rootApplyMsg(m, rootKeyPress("M"))

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources2(50, 50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: true, Provenance: messages.FetchProvenanceCanonicalList,
	})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ct-events(100") {
		t.Fatalf("precondition: expected 'ct-events(100...)' before Esc, got:\n%s", plain)
	}

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	m, cmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	// A warm re-entry re-verifies: HandleNavigate returns the KindFetchResources
	// task for the row-store hit, so both adapters seed the retained rows AND
	// fetch; a list is never fresh forever.
	if cmd == nil {
		t.Errorf("re-entering ct-events after Esc should seed the retained rows and re-verify them, but issued no command")
	}

	plain = stripANSI(rootViewContent(m))

	if !strings.Contains(plain, "ct-events(100") {
		t.Errorf("after re-entering ct-events, expected 'ct-events(100...)' (from cache), got:\n%s", plain)
	}

	// We check for "usr-0000" which is the _ct.actor value rendered in the ACTOR column
	// for the first ctEventsResources() entry (the ID "evt-0000" is not rendered in any column).
	if !strings.Contains(plain, "usr-0000") {
		t.Errorf("after re-entering ct-events, first-page resource actor 'usr-0000' should be visible in ACTOR column")
	}
}

// ---------------------------------------------------------------------------
// Re-entering cached list — M continues from last token
// ---------------------------------------------------------------------------

// TestQA_PaginationRoot_EscAndReenter_MKeyContinuesFromLastToken verifies that
// after re-entering a cached resource list that was truncated, pressing M
// continues from the saved continuation token instead of starting over.
func TestQA_PaginationRoot_EscAndReenter_MKeyContinuesFromLastToken(t *testing.T) {

	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2-continuation-token",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false, Provenance: messages.FetchProvenanceCanonicalList,
	})

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	_, cmd := rootApplyMsg(m, rootKeyPress("M"))

	if cmd == nil {
		t.Fatal("pressing M on a cached truncated list should return a non-nil command")
	}

	msg := cmd()
	loadMore, ok := msg.(messages.LoadMore)
	if !ok {
		t.Fatalf("expected LoadMoreMsg from M key on cached truncated list, got %T", msg)
	}

	if loadMore.ContinuationToken != "page2-continuation-token" {
		t.Errorf("LoadMoreMsg should carry continuation token 'page2-continuation-token', got %q", loadMore.ContinuationToken)
	}
}

// ---------------------------------------------------------------------------
// Independent cache per resource type
// ---------------------------------------------------------------------------

// TestQA_PaginationRoot_CachePerResourceType verifies that
// different resource types have independent caches: navigating to ct-events,
// then ec2, then back to ct-events should restore the ct-events resources (not
// the ec2 resources), and vice versa.
func TestQA_PaginationRoot_CachePerResourceType(t *testing.T) {

	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false,
	})
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources:    ec2TestResources(30),
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			PageSize:    30,
			TotalHint:   -1,
		},
		Append: false,
	})
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	m, ctCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	// A warm re-entry re-verifies: HandleNavigate returns the KindFetchResources
	// task for the row-store hit, so both adapters seed the retained rows AND
	// fetch; a list is never fresh forever.
	if ctCmd == nil {
		t.Errorf("re-entering ct-events should seed the retained rows and re-verify them, but issued no command")
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ct-events(50") {
		t.Errorf("after re-entering ct-events, expected 'ct-events(50...)', got:\n%s", plain)
	}
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	m, ec2Cmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// A warm re-entry re-verifies: HandleNavigate returns the KindFetchResources
	// task for the row-store hit, so both adapters seed the retained rows AND
	// fetch; a list is never fresh forever.
	if ec2Cmd == nil {
		t.Errorf("re-entering ec2 should seed the retained rows and re-verify them, but issued no command")
	}

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ec2(30") {
		t.Errorf("after re-entering ec2, expected 'ec2(30...)', got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// loadingMore transitions
// ---------------------------------------------------------------------------

// TestPagination_ErrorClearsLoadingMore verifies that delivering an APIErrorMsg
// while loadingMore=true clears the loadingMore flag AND retains the pagination
// meta (so the user can retry with M after the error is resolved).
//
// If APIErrorMsg left loadingMore set, the view would stay on
// "ct-events(50+ loading...)" with no way to retry the next page.
func TestPagination_ErrorClearsLoadingMore(t *testing.T) {
	tui.Version = "test"

	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2-token",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false, Provenance: messages.FetchProvenanceCanonicalList,
	})

	// Press M — this sets loadingMore=true on the resource list.

	m, cmd := rootApplyMsg(m, rootKeyPress("M"))
	if cmd == nil {
		t.Fatal("pressing M on a truncated list must return a non-nil command")
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "loading...") {
		t.Fatalf("precondition: expected frame title to contain 'loading...' after pressing M, got:\n%s", plain)
	}

	// Deliver an APIErrorMsg for ct-events (simulating a network failure on page 2).
	// Append/LoadingMore mirror the real KindFetchMore failure shape
	// (executor.go's own APIError construction for that case) — this failure
	// is the outcome of the M-key load-more continuation just pressed above,
	// not an initial load/refresh, so only LoadingMore (not Loading/
	// Refreshing) may clear. LoadingMore is the field the clear reads: the
	// request records which flag it raised instead of the handler inferring
	// it from Append.
	m, _ = rootApplyMsg(m, messages.APIError{
		ResourceType: "ct-events",
		Err:          fmt.Errorf("RequestTimeout: connection timed out"),
		Append:       true,
		LoadingMore:  true,
	})

	plain = stripANSI(rootViewContent(m))

	// A stale loadingMore leaves the user stuck on "ct-events(50+ loading...)",
	// unable to retry M.
	if strings.Contains(plain, "loading...") {
		t.Errorf("APIErrorMsg should clear loadingMore — frame title must not contain 'loading...' after error, got:\n%s", plain)
	}

	// ASSERTION 2: pagination must be retained (IsTruncated still true) so the "+"
	// indicator remains and the user knows they can retry M.
	if !strings.Contains(plain, "50+") {
		t.Errorf("APIErrorMsg must not clear pagination — frame title must still contain '50+' after error, got:\n%s", plain)
	}
}

// TestPagination_DoubleLoadIgnored verifies that pressing M a second time while
// loadingMore=true produces no command (the duplicate fetch is suppressed).
//
// This prevents issuing two concurrent page-2 fetches when the user double-taps M.
func TestPagination_DoubleLoadIgnored(t *testing.T) {
	tui.Version = "test"

	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2-token",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false, Provenance: messages.FetchProvenanceCanonicalList,
	})

	// First M press — must produce a command (LoadMoreMsg) and set loadingMore=true.

	m, firstCmd := rootApplyMsg(m, rootKeyPress("M"))
	if firstCmd == nil {
		t.Fatal("first M press on truncated list must return a non-nil command")
	}

	// Precondition: view must show "loading..." to confirm loadingMore=true.
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "loading...") {
		t.Fatalf("precondition: expected 'loading...' in frame title after first M press, got:\n%s", plain)
	}

	// Second M press while loadingMore=true — must produce NO command.
	// The guard `!m.loadingMore` in the LoadMore key handler suppresses the duplicate.
	_, secondCmd := rootApplyMsg(m, rootKeyPress("M"))

	// ASSERTION: the second M press must be a no-op at the root command level.
	// A non-nil command here means a second page-2 fetch would be dispatched,
	// which could cause duplicate rows or token confusion on arrival.
	if secondCmd != nil {
		t.Errorf("second M press while loadingMore=true should produce no command (duplicate fetch suppressed), but got a non-nil command")
	}
}

// TestPagination_PopViewClearsLoadingMore verifies that pressing Esc to leave a
// resource list that was mid-load (loadingMore=true), then re-entering it,
// produces a view with loadingMore=false (fresh state, no leftover spinner).
//
// Without this property, re-entering a resource type would show
// "ct-events(0+ loading...)" permanently even though no fetch is in flight.
func TestPagination_PopViewClearsLoadingMore(t *testing.T) {
	tui.Version = "test"

	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2-token",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false, Provenance: messages.FetchProvenanceCanonicalList,
	})

	// Press M — sets loadingMore=true and dispatches a LoadMoreMsg.

	m, _ = rootApplyMsg(m, rootKeyPress("M"))

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "loading...") {
		t.Fatalf("precondition: expected 'loading...' after pressing M, got:\n%s", plain)
	}

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	plain = stripANSI(rootViewContent(m))

	// ASSERTION: the re-entered view must NOT show "loading..." — the new
	// ResourceListModel is created fresh by NewResourceList with loadingMore=false.
	// A failure here means the stale loadingMore=true leaked from the previous
	// visit into the new view, permanently blocking M retries.
	if strings.Contains(plain, "loading...") {
		t.Errorf("re-entering ct-events after Esc should show a fresh view (no 'loading...'), got:\n%s", plain)
	}
}
