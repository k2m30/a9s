package unit

// qa_pagination_view_test.go — the 'M' key (LoadMore) Update() behavior when
// truncated, non-truncated, or already loading. The FrameTitle()/Append
// format and cursor-stability pins are on the live Controller seam
// (pagination_frametitle_ports_test.go and list_loadmore_ports_test.go).

import (
	"fmt"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
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

// pgTestResources creates n resources with sequential IDs starting at 0.
func pgTestResources(n int) []resource.Resource {
	return pgTestResourcesFrom(0, n)
}

// pgTestResourcesFrom creates n resources with sequential IDs starting at
// offset, so a caller simulating a second/third page of a paginated list can
// generate IDs disjoint from an earlier page's pgTestResources(n) or
// pgTestResourcesFrom(0, n) call — AWS pagination never repeats an ID across
// pages, so a synthetic page-2 fixture that starts back at index 0 does not
// mirror real pagination and defeats an append-time ID-dedup guard.
func pgTestResourcesFrom(offset, n int) []resource.Resource {
	res := make([]resource.Resource, n)
	for i := range n {
		id := fmt.Sprintf("i-%05d", offset+i)
		name := fmt.Sprintf("instance-%05d", offset+i)
		res[i] = resource.Resource{
			ID: id, Name: name,
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
func pgNewModel(t *testing.T) (views.ResourceListModel, *app.Controller) {
	t.Helper()
	tuitest.ForceColor(t)

	td := pgTestTypeDef()
	k := keys.Default()
	ctrl := newListViewCtrl(t, td)
	m := views.NewResourceList(td, nil, k, ctrl)
	m.SetSize(120, 30)
	m, _ = m.Init()
	return m, ctrl
}

// pgLoadResources sends a ResourcesLoadedMsg to the model with the given options.
func pgLoadResources(
	ctrl *app.Controller,
	m views.ResourceListModel,
	resources []resource.Resource,
	pagination *resource.PaginationMeta,
	appendMode bool,
) views.ResourceListModel {
	ctrl.ApplyResourcesLoaded("ec2", resources, pagination, appendMode)
	return m
}

// ===========================================================================
// LoadMore key (M) tests
// ===========================================================================

// TestResourceList_LoadMore_WhenTruncated_SendsMsg verifies that pressing M
// on a truncated list returns a command (which will produce a LoadMoreMsg).
func TestResourceList_LoadMore_WhenTruncated_SendsMsg(t *testing.T) {
	m, ctrl := pgNewModel(t)

	// Load truncated page
	m = pgLoadResources(ctrl, m, pgTestResources(200), &resource.PaginationMeta{
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
	if _, ok := msg.(messages.LoadMore); !ok {
		t.Errorf("expected cmd to produce LoadMoreMsg, got %T", msg)
	}
}

// TestResourceList_LoadMore_WhenNotTruncated_Noop verifies that pressing M
// on a non-truncated list (all pages loaded) does nothing.
func TestResourceList_LoadMore_WhenNotTruncated_Noop(t *testing.T) {
	t.Run("nil pagination", func(t *testing.T) {
		m, ctrl := pgNewModel(t)
		m = pgLoadResources(ctrl, m, pgTestResources(50), nil, false)

		_, cmd := m.Update(pgKeyPress("M"))
		if cmd != nil {
			t.Error("expected M key on non-truncated list (nil pagination) to return nil cmd")
		}
	})

	t.Run("IsTruncated=false", func(t *testing.T) {
		m, ctrl := pgNewModel(t)
		m = pgLoadResources(ctrl, m, pgTestResources(50), &resource.PaginationMeta{
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
	m, ctrl := pgNewModel(t)

	// Load truncated page
	m = pgLoadResources(ctrl, m, pgTestResources(200), &resource.PaginationMeta{
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

// TestResourceList_LoadMore_KeyBinding_Exists verifies that the LoadMore
// binding is registered in the key map.
func TestResourceList_LoadMore_KeyBinding_Exists(t *testing.T) {
	k := keys.Default()
	// Verify that LoadMore binding exists and matches "M"
	if !key.Matches(pgKeyPress("M"), k.LoadMore) {
		t.Error("expected 'M' key to match keys.Map.LoadMore binding")
	}
}
