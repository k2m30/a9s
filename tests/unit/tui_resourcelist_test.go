package unit

import (
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helper: build a ResourceListModel with test data
// ---------------------------------------------------------------------------

func rlTestTypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		Name:      "EC2 Instances",
		ShortName: "ec2",
		Aliases:   []string{"ec2"},
		Columns: []resource.Column{
			{Key: "instance_id", Title: "Instance ID", Width: 20},
			{Key: "name", Title: "Name", Width: 28},
			{Key: "state", Title: "State", Width: 12},
			{Key: "type", Title: "Type", Width: 14},
		},
	}
}

func rlTestResources() []resource.Resource {
	return []resource.Resource{
		{
			ID: "i-001", Name: "api-prod-01", Status: "running",
			Fields: map[string]string{
				"instance_id": "i-001", "name": "api-prod-01",
				"state": "running", "type": "t3.medium",
			},
		},
		{
			ID: "i-002", Name: "api-prod-02", Status: "running",
			Fields: map[string]string{
				"instance_id": "i-002", "name": "api-prod-02",
				"state": "running", "type": "t3.medium",
			},
		},
		{
			ID: "i-003", Name: "worker-01", Status: "stopped",
			Fields: map[string]string{
				"instance_id": "i-003", "name": "worker-01",
				"state": "stopped", "type": "t3.large",
			},
		},
		{
			ID: "i-004", Name: "bastion", Status: "pending",
			Fields: map[string]string{
				"instance_id": "i-004", "name": "bastion",
				"state": "pending", "type": "t2.micro",
			},
		},
		{
			ID: "i-005", Name: "legacy-app", Status: "terminated",
			Fields: map[string]string{
				"instance_id": "i-005", "name": "legacy-app",
				"state": "terminated", "type": "t2.small",
			},
		},
	}
}

// rlKeyPress creates a tea.KeyPressMsg for a printable character.
func rlKeyPress(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

func rlLoadedModel(t *testing.T) views.ResourceListModel {
	t.Helper()
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rlTestTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    rlTestResources(),
	})
	return m
}

// ===========================================================================
// Test View() when loading shows spinner text
// ===========================================================================

func TestResourceListView_LoadingShowsSpinner(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rlTestTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	out := m.View()
	if !strings.Contains(out, "Loading") {
		t.Errorf("expected loading view to contain 'Loading', got: %q", out)
	}
}

// ===========================================================================
// Test View() with resources shows column headers
// ===========================================================================

func TestResourceListView_ShowsColumnHeaders(t *testing.T) {
	m := rlLoadedModel(t)
	out := m.View()

	for _, title := range []string{"Instance ID", "Name", "State", "Type"} {
		if !strings.Contains(out, title) {
			t.Errorf("expected View to contain column header %q, got:\n%s", title, out)
		}
	}
}

// ===========================================================================
// Test View() with resources shows resource data in rows
// ===========================================================================

func TestResourceListView_ShowsResourceData(t *testing.T) {
	m := rlLoadedModel(t)
	out := m.View()

	for _, name := range []string{"api-prod-01", "api-prod-02", "worker-01", "bastion", "legacy-app"} {
		if !strings.Contains(out, name) {
			t.Errorf("expected View to contain resource name %q, got:\n%s", name, out)
		}
	}
	if !strings.Contains(out, "t3.medium") {
		t.Errorf("expected View to contain 't3.medium'")
	}
}

// ===========================================================================
// Test selected row uses RowSelected style (is present and distinct)
// ===========================================================================

func TestResourceListView_SelectedRowPresent(t *testing.T) {
	m := rlLoadedModel(t)
	out := m.View()

	lines := strings.Split(out, "\n")
	foundSelected := false
	for _, line := range lines {
		if strings.Contains(line, "api-prod-01") {
			foundSelected = true
			break
		}
	}
	if !foundSelected {
		t.Errorf("expected to find selected row containing 'api-prod-01'")
	}
}

// ===========================================================================
// Test status-colored rows (resources with different statuses are rendered)
// ===========================================================================

func TestResourceListView_StatusColoredRows(t *testing.T) {
	m := rlLoadedModel(t)
	out := m.View()

	for _, status := range []string{"running", "stopped", "pending", "terminated"} {
		if !strings.Contains(out, status) {
			t.Errorf("expected View to contain status %q", status)
		}
	}
}

// ===========================================================================
// Test FrameTitle() returns correct format with count
// ===========================================================================

func TestResourceListView_FrameTitle(t *testing.T) {
	m := rlLoadedModel(t)
	title := m.FrameTitle()

	expected := "ec2(5)"
	if title != expected {
		t.Errorf("expected FrameTitle() = %q, got %q", expected, title)
	}
}

// ===========================================================================
// Test FrameTitle() with filter shows "type(filtered/total)"
// ===========================================================================

func TestResourceListView_FrameTitleFiltered(t *testing.T) {
	m := rlLoadedModel(t)
	m.SetFilter("api")

	title := m.FrameTitle()
	expected := "ec2(2/5)"
	if title != expected {
		t.Errorf("expected FrameTitle() = %q, got %q", expected, title)
	}
}

// ===========================================================================
// Test SetFilter() filters resources
// ===========================================================================

func TestResourceListView_SetFilterFilters(t *testing.T) {
	m := rlLoadedModel(t)
	m.SetFilter("worker")

	out := m.View()
	if !strings.Contains(out, "worker-01") {
		t.Errorf("expected filtered View to contain 'worker-01'")
	}
	if strings.Contains(out, "api-prod-01") {
		t.Errorf("expected filtered View to NOT contain 'api-prod-01'")
	}
}

// ===========================================================================
// Test horizontal scroll changes visible output
// ===========================================================================

func TestResourceListView_HorizontalScroll(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rlTestTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(50, 20) // very narrow
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    rlTestResources(),
	})

	outBefore := m.View()

	// Scroll right using 'l' key
	m, _ = m.Update(rlKeyPress("l"))

	outAfter := m.View()

	if outBefore == outAfter {
		t.Errorf("expected horizontal scroll to change the visible output")
	}
}

func TestResourceListView_HorizontalScroll_ClampsAtLastColumn(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rlTestTypeDef() // 4 columns
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(50, 20) // narrow enough that not all columns fit
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    rlTestResources(),
	})

	// Scroll right many times — more than the number of columns
	for range 20 {
		m, _ = m.Update(rlKeyPress("l"))
	}

	out := m.View()
	// Should still show data, NOT "No resources found"
	if strings.Contains(out, "No resources found") {
		t.Error("scrolling right past last column should not show 'No resources found'")
	}
	// Should have at least one column header visible
	// The last column is "Type" — it should be visible at max scroll
	if !strings.Contains(out, "Type") && !strings.Contains(out, "State") &&
		!strings.Contains(out, "Name") && !strings.Contains(out, "Instance ID") {
		t.Errorf("at least one column header should remain visible after max scroll, got:\n%s", out)
	}
}

func TestResourceListView_HorizontalScroll_CannotScrollPastEnd(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rlTestTypeDef() // 4 columns
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(200, 20) // wide enough to see ALL columns
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    rlTestResources(),
	})

	outBefore := m.View()

	// When all columns already fit, scrolling right should have no effect
	m, _ = m.Update(rlKeyPress("l"))

	outAfter := m.View()
	if outBefore != outAfter {
		t.Error("scrolling right should have no effect when all columns fit in viewport")
	}
}

// ===========================================================================
// Test empty resource list shows appropriate message
// ===========================================================================

func TestResourceListView_EmptyList(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rlTestTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{},
	})

	out := m.View()
	if !strings.Contains(out, "No resources found") {
		t.Errorf("expected empty list to show 'No resources found', got: %q", out)
	}
}

// ===========================================================================
// Test config-driven columns (ViewsConfig)
// ===========================================================================

func TestResourceListView_ConfigDrivenColumns(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rlTestTypeDef()
	k := keys.Default()

	cfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {
				List: []config.ListColumn{
					{Title: "ID", Path: "InstanceId", Width: 20},
					{Title: "MyName", Path: "Tags", Width: 28},
				},
			},
		},
	}

	m := views.NewResourceList(td, cfg, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    rlTestResources(),
	})

	out := m.View()

	if !strings.Contains(out, "ID") {
		t.Errorf("expected config-driven column 'ID' in output")
	}
	if !strings.Contains(out, "MyName") {
		t.Errorf("expected config-driven column 'MyName' in output")
	}
}

// ===========================================================================
// Test vertical scroll: only visible rows fit in height
// ===========================================================================

func TestResourceListView_VerticalScrollLimitsRows(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rlTestTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 4)
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    rlTestResources(),
	})

	out := m.View()
	lines := strings.Split(out, "\n")
	nonEmpty := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty++
		}
	}
	if nonEmpty > 4 {
		t.Errorf("expected at most 4 non-empty lines with height=4, got %d", nonEmpty)
	}
}

// ===========================================================================
// Test sort indicators appear in column headers
// ===========================================================================

func TestResourceListView_SortIndicator(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := rlTestTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()

	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    rlTestResources(),
	})

	// Trigger sort by name with '2' key (column 1 = name, 1-indexed key "2")
	m, _ = m.Update(rlKeyPress("2"))

	out := m.View()
	if !strings.Contains(out, "\u2191") && !strings.Contains(out, "\u2193") {
		t.Errorf("expected sort indicator (arrow) in View output after sort, got:\n%s", out)
	}
}

// ===========================================================================
// Test no separator row below headers
// ===========================================================================

func TestResourceListView_NoSeparatorBelowHeaders(t *testing.T) {
	m := rlLoadedModel(t)
	out := m.View()

	lines := strings.SplitSeq(out, "\n")
	for line := range lines {
		stripped := strings.TrimSpace(line)
		if stripped == "" {
			continue
		}
		allDash := true
		for _, ch := range stripped {
			if ch != '-' && ch != '_' && ch != '=' && ch != ' ' {
				allDash = false
				break
			}
		}
		if allDash && len(stripped) > 5 {
			t.Errorf("found what looks like a separator row: %q", stripped)
		}
	}
}

// ===========================================================================
// Bug: pressing Down past last item should NOT move cursor off-screen.
// After pressing Down 10 times past the end, pressing Up once should
// immediately show the previous item (not require 10 Ups to return).
// ===========================================================================

func TestResourceList_DownPastEnd_CursorStaysAtLast(t *testing.T) {
	m := rlLoadedModel(t) // 5 resources, height 20

	// Move to last item
	m, _ = m.Update(rlKeyPress("G")) // jump to bottom

	// Verify we're at the last item
	view1 := stripANSI(m.View())
	if !strings.Contains(view1, "legacy-app") {
		t.Fatalf("after G, should see last item 'legacy-app' in view")
	}

	// Press Down 10 times past the end
	for range 10 {
		m, _ = m.Update(rlKeyPress("j"))
	}

	// View should still show the last item highlighted
	view2 := stripANSI(m.View())
	if !strings.Contains(view2, "legacy-app") {
		t.Errorf("after 10x Down past end, last item should still be visible:\n%s", view2)
	}

	// Now press Up ONCE — should immediately show the previous item
	m, _ = m.Update(rlKeyPress("k"))
	view3 := stripANSI(m.View())

	// After one Up from the last item, "bastion" (second-to-last) should
	// be where the cursor is. Verify it's visible.
	if !strings.Contains(view3, "bastion") {
		t.Errorf("after 1x Up from last item, should see 'bastion':\n%s", view3)
	}

	// The critical check: view should be DIFFERENT from the "at end" view
	// because the cursor moved. If pressing Up doesn't change the view,
	// the cursor was stuck somewhere invisible.
	if view3 == view2 {
		t.Error("pressing Up once from last position should change the view (cursor should move to previous item)")
	}
}

// TestResourceList_DownPastEnd_ManyItems tests the same bug with more items
// than fit on screen, which requires actual scrolling.
func TestResourceList_DownPastEnd_ManyItems(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := resource.ResourceTypeDef{
		Name:      "Log Streams",
		ShortName: "log_streams",
		Columns: []resource.Column{
			{Key: "stream_name", Title: "Stream Name", Width: 48},
			{Key: "last_event", Title: "Last Event", Width: 22},
		},
	}
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 15) // Only 14 data rows visible (15 minus header)

	// Create 30 resources — more than fit on screen
	var resources []resource.Resource
	for i := range 30 {
		resources = append(resources, resource.Resource{
			ID: fmt.Sprintf("stream-%02d", i), Name: fmt.Sprintf("stream-%02d", i),
			Fields: map[string]string{
				"stream_name": fmt.Sprintf("stream-%02d", i),
				"last_event":  fmt.Sprintf("2026-03-%02d 12:00", i+1),
			},
		})
	}

	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "log_streams",
		Resources:    resources,
	})

	// Jump to bottom (last item = stream-29)
	m, _ = m.Update(rlKeyPress("G"))

	viewAtEnd := stripANSI(m.View())
	if !strings.Contains(viewAtEnd, "stream-29") {
		t.Fatalf("after G, last item 'stream-29' should be visible:\n%s", viewAtEnd)
	}

	// Press Down 10 more times past the end
	for range 10 {
		m, _ = m.Update(rlKeyPress("j"))
	}

	viewAfterExtraDown := stripANSI(m.View())
	if !strings.Contains(viewAfterExtraDown, "stream-29") {
		t.Errorf("after 10x Down past end, last item should still be visible:\n%s", viewAfterExtraDown)
	}

	// Press Up ONCE — cursor should move to stream-28
	m, _ = m.Update(rlKeyPress("k"))
	viewAfterOneUp := stripANSI(m.View())

	if !strings.Contains(viewAfterOneUp, "stream-28") {
		t.Errorf("after 1x Up, stream-28 should be visible:\n%s", viewAfterOneUp)
	}

	// View must change — if it doesn't, cursor is stuck
	if viewAfterOneUp == viewAfterExtraDown {
		t.Error("pressing Up once should change the view — cursor appears stuck after pressing Down past end")
	}
}

// ===========================================================================
// Bug: narrow screen drops wide columns entirely instead of shrinking them.
// Log events with Timestamp (22) + Message (120) on a 80-col terminal
// should show both columns, with Message shrunk to fill remaining space.
// ===========================================================================

func TestResourceList_NarrowScreen_ShowsAllColumns(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	td := resource.ResourceTypeDef{
		Name:      "Log Events",
		ShortName: "log_events",
		Columns: []resource.Column{
			{Key: "timestamp", Title: "Timestamp", Width: 22},
			{Key: "message", Title: "Message", Width: 120},
		},
	}
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(80, 20) // narrow terminal — 80 cols can't fit 22+120

	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "log_events",
		Resources: []resource.Resource{
			{
				ID: "evt-1", Name: "test event",
				Fields: map[string]string{
					"timestamp": "2025-07-25 16:05",
					"message":   "Downloading snowflake_connector_python-3.2.1",
				},
			},
		},
	})

	view := stripANSI(m.View())

	// Both columns should be visible
	if !strings.Contains(view, "Timestamp") {
		t.Errorf("Timestamp header should be visible on narrow screen:\n%s", view)
	}
	if !strings.Contains(view, "Message") {
		t.Errorf("Message header should be visible on narrow screen (shrunk to fit):\n%s", view)
	}
	// Message content should be visible (even if truncated)
	if !strings.Contains(view, "Downloading") || !strings.Contains(view, "snowflake") {
		t.Errorf("Message content should be visible (truncated) on narrow screen:\n%s", view)
	}
}

// ===========================================================================
// SetDisplayName + FrameTitle coverage
// ===========================================================================

func TestResourceListView_SetDisplayName(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	k := keys.Default()
	m := views.NewResourceList(rlTestTypeDef(), nil, k)
	m.SetSize(80, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    rlTestResources(),
	})

	// Before SetDisplayName, FrameTitle uses typeDef.ShortName ("ec2")
	title := m.FrameTitle()
	if !strings.Contains(title, "ec2") {
		t.Errorf("FrameTitle before SetDisplayName should contain 'ec2'; got: %q", title)
	}

	// After SetDisplayName, FrameTitle uses the custom name
	m.SetDisplayName("Custom Name")
	title = m.FrameTitle()
	if !strings.Contains(title, "Custom Name") {
		t.Errorf("FrameTitle after SetDisplayName should contain 'Custom Name'; got: %q", title)
	}
	// Should still include the count
	if !strings.Contains(title, fmt.Sprintf("%d", len(rlTestResources()))) {
		t.Errorf("FrameTitle should include resource count; got: %q", title)
	}
}

func TestResourceListView_SetDisplayName_EmptyRestoresDefault(t *testing.T) {
	k := keys.Default()
	m := views.NewResourceList(rlTestTypeDef(), nil, k)
	m.SetSize(80, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    rlTestResources(),
	})

	m.SetDisplayName("Temporary")
	m.SetDisplayName("") // reset to default
	title := m.FrameTitle()
	if !strings.Contains(title, "ec2") {
		t.Errorf("FrameTitle after clearing displayName should use ShortName; got: %q", title)
	}
}

// ===========================================================================
// exactRelatedTargetID — indirect coverage via ResourcesLoadedMsg with
// pagination + autoOpenSingleDetail + relatedIDSet (single non-empty ID)
// ===========================================================================

func TestResourceListView_ExactRelatedTargetID_SingleID_TriggersLoadMore(t *testing.T) {
	// When: ResourceListModel has relatedIDSet = {"vol-target-123"} (exactly one non-empty ID),
	//       autoOpenSingleDetail = true, ResourcesLoadedMsg arrives with IsTruncated=true
	//       and no resources match the filter.
	// Then: exactRelatedTargetID returns ("vol-target-123", true) → LoadMoreMsg is emitted.
	k := keys.Default()
	td := resource.ResourceTypeDef{
		Name:      "EBS Volumes",
		ShortName: "ebs",
		Columns: []resource.Column{
			{Key: "volume_id", Title: "Volume ID", Width: 20},
		},
	}
	m := views.NewResourceList(td, nil, k)
	m.SetSize(80, 20)
	m, _ = m.Init()

	m.SetRelatedIDFilter([]string{"vol-target-123"})
	m.SetAutoOpenSingleDetail(true)

	// Resources that do NOT match the filter (non-matching IDs)
	nonMatching := []resource.Resource{
		{ID: "vol-other-1", Fields: map[string]string{"volume_id": "vol-other-1"}},
		{ID: "vol-other-2", Fields: map[string]string{"volume_id": "vol-other-2"}},
	}
	var got tea.Cmd
	m, got = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ebs",
		Resources:    nonMatching,
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "token-page-2",
		},
	})

	if got == nil {
		t.Fatal("exactRelatedTargetID with single non-empty ID + truncated page should emit LoadMoreMsg cmd")
	}
	msg := got()
	loadMore, ok := msg.(messages.LoadMoreMsg)
	if !ok {
		t.Fatalf("expected LoadMoreMsg, got %T: %+v", msg, msg)
	}
	if loadMore.ResourceType != "ebs" {
		t.Errorf("LoadMoreMsg.ResourceType should be 'ebs'; got %q", loadMore.ResourceType)
	}
	if loadMore.ContinuationToken != "token-page-2" {
		t.Errorf("LoadMoreMsg.ContinuationToken should be 'token-page-2'; got %q", loadMore.ContinuationToken)
	}
}

func TestResourceListView_ExactRelatedTargetID_MultipleIDs_NoLoadMore(t *testing.T) {
	// When: relatedIDSet has 2 IDs, exactRelatedTargetID returns false → no LoadMoreMsg.
	k := keys.Default()
	td := resource.ResourceTypeDef{
		Name:      "EBS Volumes",
		ShortName: "ebs",
		Columns: []resource.Column{
			{Key: "volume_id", Title: "Volume ID", Width: 20},
		},
	}
	m := views.NewResourceList(td, nil, k)
	m.SetSize(80, 20)
	m, _ = m.Init()

	m.SetRelatedIDFilter([]string{"vol-id-1", "vol-id-2"}) // 2 IDs — exactRelatedTargetID returns false
	m.SetAutoOpenSingleDetail(true)

	nonMatching := []resource.Resource{
		{ID: "vol-other-x", Fields: map[string]string{"volume_id": "vol-other-x"}},
	}
	var got tea.Cmd
	m, got = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ebs",
		Resources:    nonMatching,
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "token-page-2",
		},
	})

	if got != nil {
		msg := got()
		if _, isLoadMore := msg.(messages.LoadMoreMsg); isLoadMore {
			t.Error("exactRelatedTargetID with 2 IDs should NOT emit LoadMoreMsg (ambiguous target)")
		}
	}
	// The model should still exist and not panic
	_ = m.FrameTitle()
}

func TestResourceListView_ExactRelatedTargetID_EmptyID_NoLoadMore(t *testing.T) {
	// When: relatedIDSet = {""} (one empty-string ID),
	//       exactRelatedTargetID returns ("", false) → no LoadMoreMsg.
	k := keys.Default()
	td := resource.ResourceTypeDef{
		Name:      "EBS Volumes",
		ShortName: "ebs",
		Columns: []resource.Column{
			{Key: "volume_id", Title: "Volume ID", Width: 20},
		},
	}
	m := views.NewResourceList(td, nil, k)
	m.SetSize(80, 20)
	m, _ = m.Init()

	// SetRelatedIDFilter skips empty strings when building the set,
	// so the set ends up empty — exactRelatedTargetID returns false.
	m.SetRelatedIDFilter([]string{""})
	m.SetAutoOpenSingleDetail(true)

	nonMatching := []resource.Resource{
		{ID: "vol-other-y", Fields: map[string]string{"volume_id": "vol-other-y"}},
	}
	var got tea.Cmd
	m, got = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ebs",
		Resources:    nonMatching,
		Pagination: &resource.PaginationMeta{
			IsTruncated: true,
			NextToken:   "token-page-3",
		},
	})

	if got != nil {
		msg := got()
		if _, isLoadMore := msg.(messages.LoadMoreMsg); isLoadMore {
			t.Error("empty-string relatedID should NOT trigger LoadMoreMsg")
		}
	}
}
