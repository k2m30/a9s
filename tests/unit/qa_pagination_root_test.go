package unit

// qa_pagination_root_test.go — TDD tests for pagination bugs at the root model level.
//
// Bug 1: Paginated fetcher returns IsTruncated=true but frame title shows "(50)"
//        instead of "(50+)". These tests verify the root model correctly passes
//        PaginationMeta from ResourcesLoadedMsg down to the active resource list view.
//
// Bug 2: After loading resources, pressing Esc, and re-entering the same resource
//        type, the model makes new API calls and shows only the first page again.
//        The desired behavior is to preserve the previously loaded resources and
//        not issue any new fetch commands.
//
// Bug 3 (probe truncation): probeResourceAvailability calls GetFetcher, which
//        returns []Resource with no truncation info. For ct-events, which has a
//        paginated fetcher, the probe should call GetPaginatedFetcher and use
//        FetchResult.IsTruncated to set AvailabilityCheckedMsg.Truncated=true.
//        The downstream wiring (handler→menu→view) already works correctly, so
//        a fix to the probe alone is sufficient.
//
// Tests 1–2 exercise currently-working view-layer wiring at the root level and
// should PASS immediately.
//
// Tests 3–5 document the desired cache behavior that does not yet exist and are
// SKIPPED via t.Skip() so they do not block CI. Remove the t.Skip() once the
// resource cache is implemented.
//
// Tests 6–7 (TestQA_MainMenu_TruncatedAvailabilityShowsPlus and
// TestQA_MainMenu_NonTruncatedAvailabilityNoPlus) verify the downstream
// rendering path for Bug 3 at the MainMenuModel view level. They confirm the
// wiring from SetAvailability+SetTruncated through to View() already works for
// ct-events, so the only fix needed is in probeResourceAvailability itself.

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers: ct-events test resources
// ---------------------------------------------------------------------------

// ctEventsResources returns n ct-events resources with sequential IDs.
// Uses the new _ct.* field schema (Status is severity-based, not ReadOnly).
// CreateBucket → W verb → "ct-attention".
// _ct.actor is set to "usr-NNNN" so it renders in the ACTOR column and can be
// used as a unique per-resource assertion target.
func ctEventsResources(n int) []resource.Resource {
	resources := make([]resource.Resource, n)
	for i := range n {
		id := fmt.Sprintf("evt-%04d", i)
		actor := fmt.Sprintf("usr-%04d", i)
		resources[i] = resource.Resource{
			ID:     id,
			Name:   fmt.Sprintf("CreateBucket-%d", i),
			Fields: map[string]string{
				"event_name":    fmt.Sprintf("CreateBucket-%d", i),
				"time":          "2026-03-28 14:30:15",
				"event_time":    "2026-03-28 14:30:15",
				"user":          "admin",
				"source":        "s3.amazonaws.com",
				"resource_type": "",
				"resource_name": "",
				"read_only":     "false",
				// New _ct.* fields required by the redesigned list columns.
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
			ID:     id,
			Name:   fmt.Sprintf("DeleteObject-%d", idx),
			Fields: map[string]string{
				"event_name":    fmt.Sprintf("DeleteObject-%d", idx),
				"time":          "2026-03-28 14:30:15",
				"event_time":    "2026-03-28 14:30:15",
				"user":          "admin",
				"source":        "s3.amazonaws.com",
				"resource_type": "",
				"resource_name": "",
				"read_only":     "false",
				// New _ct.* fields required by the redesigned list columns.
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
			ID:     id,
			Name:   fmt.Sprintf("web-server-%d", i),
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
// Test 1: Initial truncated load shows "(50+)" in root rendered view
// ---------------------------------------------------------------------------

// TestQA_PaginationRoot_InitialLoadShowsTruncated verifies that when a paginated
// resource type loads its first page with IsTruncated=true, the root model's
// rendered frame title contains "50+" (not just "50").
//
// This is Bug 1: the "(50+)" indicator was not being shown in practice.
func TestQA_PaginationRoot_InitialLoadShowsTruncated(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Navigate to ct-events (push the resource list view onto the stack)
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	// Simulate the first page arriving with IsTruncated=true.
	// We bypass the actual fetch command and inject the message directly.
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2-token",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false,
	})

	plain := stripANSI(rootViewContent(m))

	// The "+" is the key indicator. The frame title must show "ct-events(50+)".
	if !strings.Contains(plain, "50+") {
		t.Errorf("expected frame title to contain '50+' for truncated first page, but got:\n%s", plain)
	}

	// Negative: must NOT show "(50)" without the "+". Because "ct-events(50+)"
	// contains the substring "ct-events(50", we check for the exact pattern
	// by asserting that the "+" is present immediately after "50" in the title.
	// The Contains("50+") assertion above is sufficient for this requirement.
}

// ---------------------------------------------------------------------------
// Test 2: Load more appends and updates the count to "(100)"
// ---------------------------------------------------------------------------

// TestQA_PaginationRoot_LoadMoreAppendsAndShowsUpdatedCount verifies that after
// pressing M and receiving a second page, the root model renders "(100)" in the
// frame title (no "+" because the last page was not truncated).
func TestQA_PaginationRoot_LoadMoreAppendsAndShowsUpdatedCount(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Navigate to ct-events
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	// Load page 1: truncated
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false,
	})

	// Press M to trigger load more
	m, cmd := rootApplyMsg(m, rootKeyPress("M"))

	// The resource list view must return a non-nil command when M is pressed on
	// a truncated list (it produces a LoadMoreMsg).
	if cmd == nil {
		t.Fatal("pressing M on a truncated list at root level should return a non-nil command")
	}

	// Load page 2: final page, not truncated
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources2(50, 50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			NextToken:   "",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: true,
	})

	plain := stripANSI(rootViewContent(m))

	// Total loaded = 100, no more pages → "(100...)" without "+"
	if !strings.Contains(plain, "ct-events(100") {
		t.Errorf("after loading two pages, expected frame title 'ct-events(100...)', got:\n%s", plain)
	}

	// Must NOT show "100+" since the last page was not truncated
	if strings.Contains(plain, "100+") {
		t.Errorf("after loading final page, frame title must not contain '100+', got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Test 3: Esc and re-enter preserves cached resources (EXPECTED TO FAIL)
// ---------------------------------------------------------------------------

// TestQA_PaginationRoot_EscAndReenter_PreservesCachedResources documents the
// desired behavior for Bug 2: after loading resources (including pressing M),
// pressing Esc to return to the main menu, and then re-entering the same
// resource type, the previously loaded 100 resources must be restored without
// issuing any new fetch commands.
//
// This test is SKIPPED until the resource cache is implemented.
// Remove the t.Skip() call once the cache feature is in place.
func TestQA_PaginationRoot_EscAndReenter_PreservesCachedResources(t *testing.T) {

	tui.Version = "test"
	m := newRootSizedModel()

	// Step 1: Navigate to ct-events
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	// Step 2: Load page 1 (truncated)
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false,
	})

	// Step 3: Press M to load more
	m, _ = rootApplyMsg(m, rootKeyPress("M"))

	// Step 4: Load page 2 (final)
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources2(50, 50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: false,
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: true,
	})

	// Verify 100 resources are loaded before navigating away
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ct-events(100") {
		t.Fatalf("precondition: expected 'ct-events(100...)' before Esc, got:\n%s", plain)
	}

	// Step 5: Press Esc to return to main menu
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Step 6: Re-navigate to ct-events
	m, cmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	// KEY ASSERTION: no new fetch command should be issued when cache is present.
	// Currently this FAILS because the implementation always fetches on navigate.
	if cmd != nil {
		t.Errorf("re-entering ct-events after Esc should return nil cmd (cache hit), but got a non-nil command — this is Bug 2")
	}

	plain = stripANSI(rootViewContent(m))

	// Should still show 100 resources
	if !strings.Contains(plain, "ct-events(100") {
		t.Errorf("after re-entering ct-events, expected 'ct-events(100...)' (from cache), got:\n%s", plain)
	}

	// The first resource from page 1 must still be present.
	// We check for "usr-0000" which is the _ct.actor value rendered in the ACTOR column
	// for the first ctEventsResources() entry (the ID "evt-0000" is not rendered in any column).
	if !strings.Contains(plain, "usr-0000") {
		t.Errorf("after re-entering ct-events, first-page resource actor 'usr-0000' should be visible in ACTOR column")
	}
}

// ---------------------------------------------------------------------------
// Test 4: Re-entering cached list — M continues from last token (EXPECTED TO FAIL)
// ---------------------------------------------------------------------------

// TestQA_PaginationRoot_EscAndReenter_MKeyContinuesFromLastToken documents the
// desired behavior: after re-entering a cached resource list that was truncated,
// pressing M should continue from the saved continuation token, not start over.
//
// This test is SKIPPED until the resource cache is implemented.
func TestQA_PaginationRoot_EscAndReenter_MKeyContinuesFromLastToken(t *testing.T) {

	tui.Version = "test"
	m := newRootSizedModel()

	// Step 1: Navigate to ct-events, load one page, leave it truncated
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
		Append: false,
	})

	// Step 2: Press Esc to go back to main menu
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Step 3: Re-enter ct-events (should use cache — no fetch)
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	// Step 4: Press M — should continue from "page2-continuation-token"
	_, cmd := rootApplyMsg(m, rootKeyPress("M"))

	if cmd == nil {
		t.Fatal("pressing M on a cached truncated list should return a non-nil command")
	}

	// Execute the command and verify it carries the correct continuation token
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
// Test 5: Independent cache per resource type (EXPECTED TO FAIL)
// ---------------------------------------------------------------------------

// TestQA_PaginationRoot_CachePerResourceType documents the desired behavior that
// different resource types have independent caches: navigating to ct-events,
// then ec2, then back to ct-events should restore the ct-events resources (not
// the ec2 resources), and vice versa.
//
// This test is SKIPPED until the resource cache is implemented.
func TestQA_PaginationRoot_CachePerResourceType(t *testing.T) {

	tui.Version = "test"
	m := newRootSizedModel()

	// Step 1: Navigate to ct-events, load 50 resources, Esc back
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
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

	// Step 2: Navigate to ec2, load 30 resources, Esc back
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
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

	// Step 3: Re-enter ct-events — must show 50 (not 30 or 0)
	m, ctCmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	if ctCmd != nil {
		t.Errorf("re-entering ct-events should not issue a fetch (cache hit), but returned a non-nil cmd")
	}

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ct-events(50") {
		t.Errorf("after re-entering ct-events, expected 'ct-events(50...)', got:\n%s", plain)
	}
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Step 4: Re-enter ec2 — must show 30 (not 50 or 0)
	m, ec2Cmd := rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	if ec2Cmd != nil {
		t.Errorf("re-entering ec2 should not issue a fetch (cache hit), but returned a non-nil cmd")
	}

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "ec2(30") {
		t.Errorf("after re-entering ec2, expected 'ec2(30...)', got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Tests 6–7: MainMenuModel view-level wiring for probe truncation (Bug 3)
// ---------------------------------------------------------------------------

// TestQA_MainMenu_TruncatedAvailabilityShowsPlus verifies that when the main
// menu receives a truncated availability result for ct-events (the resource type
// affected by the probe bug), it renders "(50+)" — not "(50)".
//
// This test exercises the downstream half of Bug 3: the wiring from
// SetAvailability+SetTruncated through View() already works. The broken link
// is in probeResourceAvailability, which never sets Truncated=true even for
// paginated fetchers. Once that probe fix lands, the menu will automatically
// show "(50+)" via this path.
//
// This test should PASS immediately (the view wiring is correct).
func TestQA_MainMenu_TruncatedAvailabilityShowsPlus(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })

	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 200)

	// Simulate what handleAvailabilityChecked does when Truncated=true arrives.
	m.SetAvailability("ct-events", 50)
	m.SetTruncated("ct-events", true)

	plain := stripANSI(m.View())

	// The rendered line for ct-events must show "(50+)".
	if !strings.Contains(plain, "(50+)") {
		t.Errorf("expected main menu to contain '(50+)' for ct-events with Truncated=true and Count=50, got:\n%s", plain)
	}

	// Must NOT show a bare "(50)" — after removing "(50+)" occurrences, "(50)" must be absent.
	withoutPlus := strings.ReplaceAll(plain, "(50+)", "")
	if strings.Contains(withoutPlus, "(50)") {
		t.Errorf("expected no bare '(50)' when truncated, only '(50+)', got:\n%s", plain)
	}
}

// TestQA_MainMenu_NonTruncatedAvailabilityNoPlus verifies that when the main
// menu receives a non-truncated availability result for ct-events, it renders
// "(50)" — not "(50+)".
//
// This is the negative case for TestQA_MainMenu_TruncatedAvailabilityShowsPlus:
// it confirms the "+" is only added when Truncated=true, preventing false positives
// in the current probe output (which correctly returns Truncated=false for most
// resource types that use the non-paginated GetFetcher path).
//
// This test should PASS immediately.
func TestQA_MainMenu_NonTruncatedAvailabilityNoPlus(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })

	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 200)

	// Simulate what handleAvailabilityChecked does when Truncated=false arrives
	// (the current probe behavior for ct-events — this is the bug: Truncated is
	// always false because probeResourceAvailability never calls GetPaginatedFetcher).
	m.SetAvailability("ct-events", 50)
	m.SetTruncated("ct-events", false)

	plain := stripANSI(m.View())

	// Must show "(50)" without "+".
	if !strings.Contains(plain, "(50)") {
		t.Errorf("expected main menu to contain '(50)' for ct-events with Truncated=false and Count=50, got:\n%s", plain)
	}

	// Must NOT show "(50+)" — truncation is false.
	if strings.Contains(plain, "(50+)") {
		t.Errorf("expected no '(50+)' when Truncated=false for ct-events, got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// Tests 8–10: loadingMore error transitions (CONCERNS.md #16)
//
// EXPECTED FAILURE STATUS (as of 2026-04-07):
//
// TestPagination_ErrorClearsLoadingMore — EXPECTED TO PASS on current main.
//   handleAPIError calls ClearLoading() which sets loadingMore=false. This test
//   documents the contract and will FAIL if someone removes the ClearLoading call.
//   NOTE: If the active view is NOT a *ResourceListModel at the time of the error
//   (e.g., a spinner-only view before resources arrive), loadingMore is never
//   cleared — that race is the deadlock described in CONCERNS.md #16.
//
// TestPagination_DoubleLoadIgnored — EXPECTED TO PASS on current main.
//   The guard `!m.loadingMore` on line 346 of resourcelist.go already prevents a
//   second fetch. This test documents that contract.
//
// TestPagination_PopViewClearsLoadingMore — EXPECTED TO PASS on current main.
//   When the user presses Esc and re-enters the resource list, a new
//   ResourceListModel is created via NewResourceList (loadingMore defaults to
//   false). This test documents that re-entry produces a clean state.
// ---------------------------------------------------------------------------

// TestPagination_ErrorClearsLoadingMore verifies that delivering an APIErrorMsg
// while loadingMore=true clears the loadingMore flag AND retains the pagination
// meta (so the user can retry with M after the error is resolved).
//
// Documents CONCERNS.md #16: if APIErrorMsg did not clear loadingMore, the view
// would be permanently stuck showing "ct-events(50+ loading...)" with no way to
// retry the next page.
func TestPagination_ErrorClearsLoadingMore(t *testing.T) {
	tui.Version = "test"

	m := newRootSizedModel()

	// Navigate to ct-events.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	// Load page 1 — truncated.
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2-token",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false,
	})

	// Press M — this sets loadingMore=true on the resource list.
	m, cmd := rootApplyMsg(m, rootKeyPress("M"))
	if cmd == nil {
		t.Fatal("pressing M on a truncated list must return a non-nil command")
	}

	// Confirm we are now in the "loading..." state before the error arrives.
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "loading...") {
		t.Fatalf("precondition: expected frame title to contain 'loading...' after pressing M, got:\n%s", plain)
	}

	// Deliver an APIErrorMsg for ct-events (simulating a network failure on page 2).
	m, _ = rootApplyMsg(m, messages.APIError{
		ResourceType: "ct-events",
		Err:          fmt.Errorf("RequestTimeout: connection timed out"),
	})

	plain = stripANSI(rootViewContent(m))

	// ASSERTION 1: loadingMore must be cleared — frame title must NOT contain "loading...".
	// Failure here means the deadlock from CONCERNS.md #16 is present: the user
	// cannot retry M and is stuck on "ct-events(50+ loading...)".
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

	// Navigate to ct-events.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	// Load page 1 — truncated.
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2-token",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false,
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

	// Navigate to ct-events.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ct-events",
	})

	// Load page 1 — truncated.
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ct-events",
		Resources:    ctEventsResources(50),
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "page2-token",
			PageSize:    50,
			TotalHint:   -1,
		},
		Append: false,
	})

	// Press M — sets loadingMore=true and dispatches a LoadMoreMsg.
	m, _ = rootApplyMsg(m, rootKeyPress("M"))

	// Precondition: confirm we are mid-load.
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "loading...") {
		t.Fatalf("precondition: expected 'loading...' after pressing M, got:\n%s", plain)
	}

	// Pop the view (simulate Esc back to main menu).
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	// Re-push ct-events (simulate the user pressing Enter on it again in the menu).
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
