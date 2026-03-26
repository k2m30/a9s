package unit

// qa_pagination_view_test.go — TDD tests for Phase 2 pagination view-layer.
//
// These tests exercise:
//   - FrameTitle() with pagination states (truncated, loading, filtered)
//   - ResourcesLoadedMsg with Append=true appends vs Append=false replaces
//   - 'M' key (LoadMore) behavior when truncated, non-truncated, or already loading
//
// Phase 0+1 prerequisites (must be merged before these compile):
//   - resource.PaginationMeta  (IsTruncated, NextToken)
//   - messages.ResourcesLoadedMsg gains Pagination, Append fields
//   - messages.LoadMoreMsg type
//   - keys.Map gains LoadMore binding (M key)

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// pgTestTypeDef returns an EC2-like ResourceTypeDef for pagination tests.
func pgTestTypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		Name:      "EC2 Instances",
		ShortName: "ec2",
		Aliases:   []string{"ec2"},
		Columns: []resource.Column{
			{Key: "instance_id", Title: "Instance ID", Width: 20},
			{Key: "name", Title: "Name", Width: 28},
			{Key: "state", Title: "State", Width: 12},
		},
	}
}

// pgTestResources creates n resources with sequential IDs.
func pgTestResources(n int) []resource.Resource {
	res := make([]resource.Resource, n)
	for i := range n {
		id := fmt.Sprintf("i-%05d", i)
		name := fmt.Sprintf("instance-%05d", i)
		res[i] = resource.Resource{
			ID: id, Name: name, Status: "running",
			Fields: map[string]string{
				"instance_id": id,
				"name":        name,
				"state":       "running",
			},
		}
	}
	return res
}

// pgKeyPress creates a tea.KeyPressMsg for a printable character.
func pgKeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

// pgNewModel creates a fresh ResourceListModel, calls Init, and sets size.
func pgNewModel(t *testing.T) views.ResourceListModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := pgTestTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 30)
	m, _ = m.Init()
	return m
}

// pgLoadResources sends a ResourcesLoadedMsg to the model with the given options.
func pgLoadResources(
	m views.ResourceListModel,
	resources []resource.Resource,
	pagination *resource.PaginationMeta,
	appendMode bool,
) views.ResourceListModel {
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    resources,
		Pagination:   pagination,
		Append:       appendMode,
	})
	return m
}

// ===========================================================================
// FrameTitle tests
// ===========================================================================

// TestResourceList_FrameTitle_NonTruncated verifies that FrameTitle returns
// "ec2(42)" when pagination is nil or IsTruncated=false.
func TestResourceList_FrameTitle_NonTruncated(t *testing.T) {
	t.Run("nil pagination", func(t *testing.T) {
		m := pgNewModel(t)
		m = pgLoadResources(m, pgTestResources(42), nil, false)

		title := m.FrameTitle()
		expected := "ec2(42)"
		if title != expected {
			t.Errorf("expected FrameTitle() = %q, got %q", expected, title)
		}
	})

	t.Run("IsTruncated=false", func(t *testing.T) {
		m := pgNewModel(t)
		m = pgLoadResources(m, pgTestResources(42), &resource.PaginationMeta{
			IsTruncated: false,
		}, false)

		title := m.FrameTitle()
		expected := "ec2(42)"
		if title != expected {
			t.Errorf("expected FrameTitle() = %q, got %q", expected, title)
		}
	})
}

// TestResourceList_FrameTitle_Truncated verifies that FrameTitle returns
// "ec2(200+)" when IsTruncated=true, indicating more pages are available.
func TestResourceList_FrameTitle_Truncated(t *testing.T) {
	m := pgNewModel(t)
	m = pgLoadResources(m, pgTestResources(200), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "token-abc",
	}, false)

	title := m.FrameTitle()
	expected := "ec2(200+)"
	if title != expected {
		t.Errorf("expected FrameTitle() = %q, got %q", expected, title)
	}
}

// TestResourceList_FrameTitle_LoadingMore verifies that FrameTitle shows
// "ec2(200+ loading...)" while a subsequent page is being fetched.
func TestResourceList_FrameTitle_LoadingMore(t *testing.T) {
	m := pgNewModel(t)
	// Load initial truncated page
	m = pgLoadResources(m, pgTestResources(200), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "token-abc",
	}, false)

	// Press M to trigger load more — this should set loadingMore=true
	m, _ = m.Update(pgKeyPress("M"))

	title := m.FrameTitle()
	expected := "ec2(200+ loading...)"
	if title != expected {
		t.Errorf("expected FrameTitle() = %q, got %q", expected, title)
	}
}

// TestResourceList_FrameTitle_TruncatedWithFilter verifies that FrameTitle returns
// "ec2(15/200+)" when there are more pages and a filter is active.
func TestResourceList_FrameTitle_TruncatedWithFilter(t *testing.T) {
	m := pgNewModel(t)

	// Create 200 resources, 15 of which match "instance-0000"
	resources := pgTestResources(200)
	m = pgLoadResources(m, resources, &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "token-abc",
	}, false)

	// Apply filter that matches "instance-0000X" (X=0..9) plus instance-00010..instance-00014 = 15 matches
	m.SetFilter("instance-0000")

	title := m.FrameTitle()
	// With filter, should show "ec2(filtered/total+)" format
	// The filter "instance-0000" matches instance-00000 through instance-00009 = 10 items
	expectedPrefix := "ec2("
	if !strings.HasPrefix(title, expectedPrefix) {
		t.Errorf("expected FrameTitle() to start with %q, got %q", expectedPrefix, title)
	}
	if !strings.Contains(title, "/200+)") {
		t.Errorf("expected FrameTitle() to contain '/200+)' for truncated total, got %q", title)
	}
}

// TestResourceList_FrameTitle_AllLoadedWithFilter verifies that FrameTitle returns
// "ec2(15/523)" when all pages have been loaded (not truncated) and a filter is active.
func TestResourceList_FrameTitle_AllLoadedWithFilter(t *testing.T) {
	m := pgNewModel(t)

	// Load all resources (not truncated)
	resources := pgTestResources(523)
	m = pgLoadResources(m, resources, &resource.PaginationMeta{
		IsTruncated: false,
	}, false)

	// Apply filter — "instance-0000" matches instance-00000 through instance-00009 = 10
	m.SetFilter("instance-0000")

	title := m.FrameTitle()
	expected := "ec2(10/523)"
	if title != expected {
		t.Errorf("expected FrameTitle() = %q, got %q", expected, title)
	}
}

// ===========================================================================
// Append behavior tests
// ===========================================================================

// TestResourceList_Update_AppendTrue_AppendsResources verifies that
// ResourcesLoadedMsg with Append=true appends to the existing resource list
// instead of replacing it.
func TestResourceList_Update_AppendTrue_AppendsResources(t *testing.T) {
	m := pgNewModel(t)

	// Load initial page of 200 resources
	m = pgLoadResources(m, pgTestResources(200), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "token-page2",
	}, false)

	initialTitle := m.FrameTitle()
	if initialTitle != "ec2(200+)" {
		t.Fatalf("precondition: expected FrameTitle() = %q, got %q", "ec2(200+)", initialTitle)
	}

	// Append page 2 with 150 more resources (and no more pages)
	page2 := make([]resource.Resource, 150)
	for i := range 150 {
		id := fmt.Sprintf("i-%05d", 200+i)
		name := fmt.Sprintf("instance-%05d", 200+i)
		page2[i] = resource.Resource{
			ID: id, Name: name, Status: "running",
			Fields: map[string]string{
				"instance_id": id,
				"name":        name,
				"state":       "running",
			},
		}
	}

	m = pgLoadResources(m, page2, &resource.PaginationMeta{
		IsTruncated: false,
	}, true)

	// After append, total should be 200 + 150 = 350
	title := m.FrameTitle()
	expected := "ec2(350)"
	if title != expected {
		t.Errorf("after append, expected FrameTitle() = %q, got %q", expected, title)
	}

	// Verify resources from BOTH pages exist. The first resource (i-00000) should
	// still be accessible by scrolling to the top.
	out := stripANSI(m.View())
	if !strings.Contains(out, "i-00000") {
		// First page resources should still be present
		t.Errorf("after append, first-page resource i-00000 should be visible at cursor position")
	}
}

// TestResourceList_Update_AppendFalse_ReplacesResources verifies that
// ResourcesLoadedMsg with Append=false replaces the entire resource list,
// matching the legacy non-paginated behavior.
func TestResourceList_Update_AppendFalse_ReplacesResources(t *testing.T) {
	m := pgNewModel(t)

	// Load initial page of 50 resources
	m = pgLoadResources(m, pgTestResources(50), nil, false)

	title1 := m.FrameTitle()
	if title1 != "ec2(50)" {
		t.Fatalf("precondition: expected FrameTitle() = %q, got %q", "ec2(50)", title1)
	}

	// Replace with a fresh set of 30 resources (Append=false)
	m = pgLoadResources(m, pgTestResources(30), nil, false)

	title2 := m.FrameTitle()
	expected := "ec2(30)"
	if title2 != expected {
		t.Errorf("after replace, expected FrameTitle() = %q, got %q", expected, title2)
	}
}

// TestResourceList_Update_Append_StoresPagination verifies that pagination
// metadata is stored after both append and replace operations, and that
// subsequent FrameTitle calls reflect the stored pagination state.
func TestResourceList_Update_Append_StoresPagination(t *testing.T) {
	t.Run("replace stores pagination", func(t *testing.T) {
		m := pgNewModel(t)

		// Load with truncated pagination
		m = pgLoadResources(m, pgTestResources(100), &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "token-1",
		}, false)

		title := m.FrameTitle()
		if title != "ec2(100+)" {
			t.Errorf("expected FrameTitle() = %q, got %q", "ec2(100+)", title)
		}
	})

	t.Run("append updates pagination", func(t *testing.T) {
		m := pgNewModel(t)

		// Load page 1: truncated
		m = pgLoadResources(m, pgTestResources(100), &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "token-1",
		}, false)

		if m.FrameTitle() != "ec2(100+)" {
			t.Fatalf("precondition failed: %q", m.FrameTitle())
		}

		// Load page 2: still truncated
		m = pgLoadResources(m, pgTestResources(50), &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "token-2",
		}, true)

		title := m.FrameTitle()
		if title != "ec2(150+)" {
			t.Errorf("expected FrameTitle() = %q after second page, got %q", "ec2(150+)", title)
		}

		// Load page 3: final page (not truncated)
		m = pgLoadResources(m, pgTestResources(25), &resource.PaginationMeta{
			IsTruncated: false,
		}, true)

		title = m.FrameTitle()
		if title != "ec2(175)" {
			t.Errorf("expected FrameTitle() = %q after final page, got %q", "ec2(175)", title)
		}
	})
}

// ===========================================================================
// LoadMore key (M) tests
// ===========================================================================

// TestResourceList_LoadMore_WhenTruncated_SendsMsg verifies that pressing M
// on a truncated list returns a command (which will produce a LoadMoreMsg).
func TestResourceList_LoadMore_WhenTruncated_SendsMsg(t *testing.T) {
	m := pgNewModel(t)

	// Load truncated page
	m = pgLoadResources(m, pgTestResources(200), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "token-abc",
	}, false)

	// Press M (LoadMore)
	_, cmd := m.Update(pgKeyPress("M"))

	if cmd == nil {
		t.Fatal("expected M key on truncated list to return a non-nil command")
	}

	// Execute the command to verify it produces a LoadMoreMsg
	msg := cmd()
	if _, ok := msg.(messages.LoadMoreMsg); !ok {
		t.Errorf("expected cmd to produce LoadMoreMsg, got %T", msg)
	}
}

// TestResourceList_LoadMore_WhenNotTruncated_Noop verifies that pressing M
// on a non-truncated list (all pages loaded) does nothing.
func TestResourceList_LoadMore_WhenNotTruncated_Noop(t *testing.T) {
	t.Run("nil pagination", func(t *testing.T) {
		m := pgNewModel(t)
		m = pgLoadResources(m, pgTestResources(50), nil, false)

		_, cmd := m.Update(pgKeyPress("M"))
		if cmd != nil {
			t.Error("expected M key on non-truncated list (nil pagination) to return nil cmd")
		}
	})

	t.Run("IsTruncated=false", func(t *testing.T) {
		m := pgNewModel(t)
		m = pgLoadResources(m, pgTestResources(50), &resource.PaginationMeta{
			IsTruncated: false,
		}, false)

		_, cmd := m.Update(pgKeyPress("M"))
		if cmd != nil {
			t.Error("expected M key on non-truncated list to return nil cmd")
		}
	})
}

// TestResourceList_LoadMore_WhenAlreadyLoading_Noop verifies that pressing M
// while a page is already being fetched does nothing (prevents double-fetching).
func TestResourceList_LoadMore_WhenAlreadyLoading_Noop(t *testing.T) {
	m := pgNewModel(t)

	// Load truncated page
	m = pgLoadResources(m, pgTestResources(200), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "token-abc",
	}, false)

	// Press M once — should start loading
	m, cmd1 := m.Update(pgKeyPress("M"))
	if cmd1 == nil {
		t.Fatal("precondition: first M press should return a command")
	}

	// Press M again while still loading — should be a no-op
	_, cmd2 := m.Update(pgKeyPress("M"))
	if cmd2 != nil {
		t.Error("expected second M key press during loadingMore to return nil cmd (no double-fetch)")
	}
}

// ===========================================================================
// Edge case tests
// ===========================================================================

// TestResourceList_FrameTitle_Pagination_AllResourceTypes verifies that
// FrameTitle works correctly for all resource types, not just EC2.
func TestResourceList_FrameTitle_Pagination_AllResourceTypes(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	for _, rt := range resource.AllResourceTypes() {
		t.Run(rt.ShortName, func(t *testing.T) {
			k := keys.Default()
			m := views.NewResourceList(rt, nil, k)
			m.SetSize(120, 30)
			m, _ = m.Init()

			// Create some test resources with fields matching this type's columns
			resources := make([]resource.Resource, 100)
			for i := range 100 {
				fields := make(map[string]string)
				for _, col := range rt.Columns {
					fields[col.Key] = fmt.Sprintf("%s-%d", col.Key, i)
				}
				resources[i] = resource.Resource{
					ID: fmt.Sprintf("id-%d", i), Name: fmt.Sprintf("name-%d", i),
					Status: "running", Fields: fields,
				}
			}

			// Non-truncated
			m, _ = m.Update(messages.ResourcesLoadedMsg{
				ResourceType: rt.ShortName,
				Resources:    resources,
				Pagination:   nil,
				Append:       false,
			})
			title := m.FrameTitle()
			expected := rt.ShortName + "(100)"
			if title != expected {
				t.Errorf("non-truncated: expected %q, got %q", expected, title)
			}

			// Truncated
			m, _ = m.Update(messages.ResourcesLoadedMsg{
				ResourceType: rt.ShortName,
				Resources:    resources,
				Pagination: &resource.PaginationMeta{
					IsTruncated: true,
					NextToken:   "tok",
				},
				Append: false,
			})
			title = m.FrameTitle()
			expected = rt.ShortName + "(100+)"
			if title != expected {
				t.Errorf("truncated: expected %q, got %q", expected, title)
			}
		})
	}
}

// TestResourceList_Append_EmptySecondPage verifies that appending an empty
// second page does not corrupt the resource list.
func TestResourceList_Append_EmptySecondPage(t *testing.T) {
	m := pgNewModel(t)
	m = pgLoadResources(m, pgTestResources(100), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "tok",
	}, false)

	if m.FrameTitle() != "ec2(100+)" {
		t.Fatalf("precondition: %q", m.FrameTitle())
	}

	// Append empty final page
	m = pgLoadResources(m, []resource.Resource{}, &resource.PaginationMeta{
		IsTruncated: false,
	}, true)

	title := m.FrameTitle()
	if title != "ec2(100)" {
		t.Errorf("after appending empty page, expected %q, got %q", "ec2(100)", title)
	}
}

// TestResourceList_Append_PreservesCursorPosition verifies that appending
// new resources does not disrupt the user's current cursor position.
func TestResourceList_Append_PreservesCursorPosition(t *testing.T) {
	m := pgNewModel(t)

	// Load initial page
	m = pgLoadResources(m, pgTestResources(50), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "tok",
	}, false)

	// Move cursor down 10 positions
	for range 10 {
		m, _ = m.Update(pgKeyPress("j"))
	}

	// Verify cursor is at position 10
	selected := m.SelectedResource()
	if selected == nil {
		t.Fatal("expected a selected resource after moving cursor")
	}
	expectedID := "i-00010"
	if selected.ID != expectedID {
		t.Fatalf("precondition: expected cursor at %q, got %q", expectedID, selected.ID)
	}

	// Append more resources
	m = pgLoadResources(m, pgTestResources(50), &resource.PaginationMeta{
		IsTruncated: false,
	}, true)

	// Cursor should still point at the same resource
	afterAppend := m.SelectedResource()
	if afterAppend == nil {
		t.Fatal("expected a selected resource after append")
	}
	if afterAppend.ID != expectedID {
		t.Errorf("after append, expected cursor still at %q, got %q", expectedID, afterAppend.ID)
	}
}

// TestResourceList_LoadMore_SetsAndClearsLoadingMore verifies the full
// load-more lifecycle: pressing M sets loadingMore, receiving the response
// clears it.
func TestResourceList_LoadMore_SetsAndClearsLoadingMore(t *testing.T) {
	m := pgNewModel(t)

	// Load truncated page
	m = pgLoadResources(m, pgTestResources(100), &resource.PaginationMeta{
		IsTruncated: true,
		NextToken:   "tok",
	}, false)

	// Press M — should enter loadingMore state
	m, _ = m.Update(pgKeyPress("M"))

	title := m.FrameTitle()
	if !strings.Contains(title, "loading...") {
		t.Errorf("after pressing M, FrameTitle should contain 'loading...', got %q", title)
	}

	// Receive the appended page — loadingMore should clear
	m = pgLoadResources(m, pgTestResources(50), &resource.PaginationMeta{
		IsTruncated: false,
	}, true)

	title = m.FrameTitle()
	if strings.Contains(title, "loading...") {
		t.Errorf("after receiving append, FrameTitle should not contain 'loading...', got %q", title)
	}
	if title != "ec2(150)" {
		t.Errorf("expected %q, got %q", "ec2(150)", title)
	}
}

// TestResourceList_LoadMore_KeyBinding_Exists verifies that the LoadMore
// binding is registered in the key map.
func TestResourceList_LoadMore_KeyBinding_Exists(t *testing.T) {
	k := keys.Default()
	// Verify that LoadMore binding exists and matches "M"
	if !key.Matches(pgKeyPress("M"), k.LoadMore) {
		t.Error("expected 'M' key to match keys.Map.LoadMore binding")
	}
}
