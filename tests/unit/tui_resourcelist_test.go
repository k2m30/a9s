package unit

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/config"
	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/tui/keys"
	"github.com/k2m30/a9s/internal/tui/messages"
	"github.com/k2m30/a9s/internal/tui/styles"
	"github.com/k2m30/a9s/internal/tui/views"
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

	// Trigger sort by name with 'N' key
	m, _ = m.Update(rlKeyPress("N"))

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

	lines := strings.Split(out, "\n")
	for _, line := range lines {
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
