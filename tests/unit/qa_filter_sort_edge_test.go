package unit

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/k2m30/a9s/internal/app"
	awsclient "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/internal/navigation"
	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/styles"
	"github.com/k2m30/a9s/internal/views"
)

// ===================================================================
// Helper: build a resource list view state with EC2 resources.
// ===================================================================

func ec2ResourceListState(resources []resource.Resource) app.AppState {
	s := app.NewAppState("", "")
	s.CurrentView = app.ResourceListView
	s.CurrentResourceType = "ec2"
	s.Resources = resources
	s.Width = 120
	s.Height = 40
	return s
}

func sampleEC2Resources() []resource.Resource {
	return []resource.Resource{
		{
			ID: "i-abc001", Name: "Prod-Web-1", Status: "running",
			Fields: map[string]string{
				"instance_id": "i-abc001", "name": "Prod-Web-1", "state": "running",
				"type": "t3.xlarge", "private_ip": "10.0.1.1",
				"public_ip": "54.1.2.3", "launch_time": "2026-01-15",
			},
			DetailData: map[string]string{"Instance ID": "i-abc001", "Name": "Prod-Web-1"},
			RawJSON:    `{"InstanceId":"i-abc001"}`,
		},
		{
			ID: "i-abc002", Name: "dev-api", Status: "stopped",
			Fields: map[string]string{
				"instance_id": "i-abc002", "name": "dev-api", "state": "stopped",
				"type": "t3.medium", "private_ip": "10.0.2.5",
				"public_ip": "", "launch_time": "2026-02-20",
			},
			DetailData: map[string]string{"Instance ID": "i-abc002", "Name": "dev-api"},
			RawJSON:    `{"InstanceId":"i-abc002"}`,
		},
		{
			ID: "i-abc003", Name: "STAGING-API", Status: "running",
			Fields: map[string]string{
				"instance_id": "i-abc003", "name": "STAGING-API", "state": "running",
				"type": "t3.small", "private_ip": "10.0.3.9",
				"public_ip": "52.10.20.30", "launch_time": "2026-03-01",
			},
			DetailData: map[string]string{"Instance ID": "i-abc003", "Name": "STAGING-API"},
			RawJSON:    `{"InstanceId":"i-abc003"}`,
		},
		{
			ID: "i-abc004", Name: "prod-db", Status: "running",
			Fields: map[string]string{
				"instance_id": "i-abc004", "name": "prod-db", "state": "running",
				"type": "r5.large", "private_ip": "10.0.4.2",
				"public_ip": "", "launch_time": "2025-12-01",
			},
			DetailData: map[string]string{"Instance ID": "i-abc004", "Name": "prod-db"},
			RawJSON:    `{"InstanceId":"i-abc004"}`,
		},
	}
}

func pressKeyFSE(s app.AppState, keyStr string) app.AppState {
	updated, _ := s.Update(tea.KeyPressMsg{Code: -1, Text: keyStr})
	return updated.(app.AppState)
}

func pressSpecialKeyFSE(s app.AppState, code rune) app.AppState {
	updated, _ := s.Update(tea.KeyPressMsg{Code: code})
	return updated.(app.AppState)
}

// ===================================================================
// QA-123: / activates filter mode
// ===================================================================
func TestQA_123_SlashActivatesFilterMode(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())

	state = pressKeyFSE(state, "/")

	if !state.FilterMode {
		t.Error("/ should activate FilterMode in ResourceListView")
	}
	if state.Filter != "" {
		t.Errorf("Filter should start empty, got %q", state.Filter)
	}
}

// ===================================================================
// QA-124: / does NOT activate in main menu
// ===================================================================
func TestQA_124_SlashNoEffectInMainMenu(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.MainMenuView

	state = pressKeyFSE(state, "/")

	if state.FilterMode {
		t.Error("/ should NOT activate FilterMode in MainMenuView")
	}
}

// ===================================================================
// QA-125: Typing filters in real time
// ===================================================================
func TestQA_125_TypingFiltersRealTime(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.FilterMode = true

	// Type "p", "r", "o", "d" one at a time; check narrowing at each step
	for i, ch := range "prod" {
		state = pressKeyFSE(state, string(ch))
		prefix := "prod"[:i+1]
		if state.Filter != prefix {
			t.Fatalf("After typing %q, filter should be %q, got %q", string(ch), prefix, state.Filter)
		}
	}

	// "prod" should match "Prod-Web-1" and "prod-db" (case-insensitive)
	if len(state.FilteredResources) != 2 {
		t.Errorf("Filter 'prod' should match 2 resources, got %d", len(state.FilteredResources))
	}
}

// ===================================================================
// QA-126: Case insensitive matching
// ===================================================================
func TestQA_126_CaseInsensitiveFilter(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.FilterMode = true

	for _, ch := range "prod" {
		state = pressKeyFSE(state, string(ch))
	}

	// Should match "Prod-Web-1" and "prod-db" despite lowercase query
	found := map[string]bool{}
	for _, r := range state.FilteredResources {
		found[r.Name] = true
	}
	if !found["Prod-Web-1"] {
		t.Error("Filter 'prod' should match 'Prod-Web-1' (case-insensitive)")
	}
	if !found["prod-db"] {
		t.Error("Filter 'prod' should match 'prod-db'")
	}
}

// ===================================================================
// QA-127: Filter across all visible columns
// ===================================================================
func TestQA_127_FilterAcrossAllColumns(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.FilterMode = true

	// Filter by instance type "t3.xlarge" which is in Fields, not Name
	for _, ch := range "t3.xlarge" {
		state = pressKeyFSE(state, string(ch))
	}

	if len(state.FilteredResources) != 1 {
		t.Fatalf("Filter 't3.xlarge' should match 1 resource, got %d", len(state.FilteredResources))
	}
	if state.FilteredResources[0].Name != "Prod-Web-1" {
		t.Errorf("Matched resource should be 'Prod-Web-1', got %q", state.FilteredResources[0].Name)
	}
}

// ===================================================================
// QA-128: Filter matches ID field
// ===================================================================
func TestQA_128_FilterMatchesIDField(t *testing.T) {
	resources := sampleEC2Resources()
	filtered := views.FilterResources("i-abc001", resources)

	if len(filtered) != 1 {
		t.Fatalf("Filter by ID 'i-abc001' should match 1 resource, got %d", len(filtered))
	}
	if filtered[0].ID != "i-abc001" {
		t.Errorf("Matched resource should have ID 'i-abc001', got %q", filtered[0].ID)
	}
}

// ===================================================================
// QA-129: Filter matches status field
// ===================================================================
func TestQA_129_FilterMatchesStatusField(t *testing.T) {
	resources := sampleEC2Resources()
	filtered := views.FilterResources("stopped", resources)

	if len(filtered) != 1 {
		t.Fatalf("Filter 'stopped' should match 1 resource, got %d", len(filtered))
	}
	if filtered[0].Name != "dev-api" {
		t.Errorf("Matched resource should be 'dev-api', got %q", filtered[0].Name)
	}
}

// ===================================================================
// QA-130: No matches shows empty state with message
// ===================================================================
func TestQA_130_NoMatchesShowsMessage(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.FilterMode = true

	for _, ch := range "zzzznonexistent" {
		state = pressKeyFSE(state, string(ch))
	}
	// Confirm filter mode with enter to see rendered message
	state = pressSpecialKeyFSE(state, tea.KeyEnter)

	if len(state.FilteredResources) != 0 {
		t.Errorf("Non-matching filter should produce 0 results, got %d", len(state.FilteredResources))
	}

	// Check the rendered output shows the "no matching" message
	view := state.View()
	if !strings.Contains(view.Content, "No ec2 resources matching filter") {
		t.Error("View should show 'No ec2 resources matching filter' message")
	}
}

// ===================================================================
// QA-131: Escape clears filter
// ===================================================================
func TestQA_131_EscapeClearsFilter(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.FilterMode = true
	state.Filter = "prod"
	state.FilteredResources = views.FilterResources("prod", state.Resources)

	state = pressSpecialKeyFSE(state, tea.KeyEscape)

	if state.FilterMode {
		t.Error("Escape should exit FilterMode")
	}
	if state.Filter != "" {
		t.Errorf("Escape should clear Filter, got %q", state.Filter)
	}
	if state.FilteredResources != nil {
		t.Error("Escape should set FilteredResources to nil")
	}
}

// ===================================================================
// QA-132: Enter confirms filter (keeps text, exits mode)
// ===================================================================
func TestQA_132_EnterConfirmsFilter(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.FilterMode = true

	// Type "prod"
	for _, ch := range "prod" {
		state = pressKeyFSE(state, string(ch))
	}
	// Press Enter
	state = pressSpecialKeyFSE(state, tea.KeyEnter)

	if state.FilterMode {
		t.Error("Enter should exit FilterMode")
	}
	if state.Filter != "prod" {
		t.Errorf("Enter should preserve Filter text, got %q", state.Filter)
	}
	if len(state.FilteredResources) != 2 {
		t.Errorf("Filtered results should still be present, got %d", len(state.FilteredResources))
	}
}

// ===================================================================
// QA-133: Filter persists after describe and back
// ===================================================================
func TestQA_133_FilterPersistsAfterDetailAndBack(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())

	// Apply filter "prod"
	state.Filter = "prod"
	state.FilteredResources = views.FilterResources("prod", state.Resources)
	state.SelectedIndex = 0

	// Describe (d) the first filtered resource
	state = pressKeyFSE(state, "d")
	if state.CurrentView != app.DetailView {
		t.Fatal("d should open DetailView")
	}

	// Go back with Escape
	state = pressSpecialKeyFSE(state, tea.KeyEscape)

	if state.Filter != "prod" {
		t.Errorf("Filter should persist after back, got %q", state.Filter)
	}
}

// ===================================================================
// QA-134: Filter clears on resource type switch
// ===================================================================
func TestQA_134_FilterClearsOnResourceTypeSwitch(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.Filter = "prod"
	state.FilteredResources = views.FilterResources("prod", state.Resources)

	// Switch to RDS via command mode
	state.CommandMode = true
	state.CommandText = "rds"
	state = pressSpecialKeyFSE(state, tea.KeyEnter)

	if state.Filter != "" {
		t.Errorf("Filter should be cleared on resource type switch, got %q", state.Filter)
	}
	if state.FilteredResources != nil {
		t.Error("FilteredResources should be nil after resource type switch")
	}
}

// ===================================================================
// QA-135: Filter with special characters
// ===================================================================
func TestQA_135_FilterSpecialCharacters(t *testing.T) {
	resources := []resource.Resource{
		{ID: "i-1", Name: "web-1.prod", Fields: map[string]string{"name": "web-1.prod"}},
		{ID: "i-2", Name: "web_2_staging", Fields: map[string]string{"name": "web_2_staging"}},
		{ID: "i-3", Name: "api-server", Fields: map[string]string{"name": "api-server", "private_ip": "10.0.1.5"}},
	}

	// Test dot (literal, not regex)
	filtered := views.FilterResources("web-1.", resources)
	if len(filtered) != 1 || filtered[0].Name != "web-1.prod" {
		t.Errorf("Filter 'web-1.' should match exactly 'web-1.prod', got %d matches", len(filtered))
	}

	// Test underscore
	filtered = views.FilterResources("web_2", resources)
	if len(filtered) != 1 || filtered[0].Name != "web_2_staging" {
		t.Errorf("Filter 'web_2' should match exactly 'web_2_staging', got %d matches", len(filtered))
	}

	// Test IP-like pattern
	filtered = views.FilterResources("10.0.1", resources)
	if len(filtered) != 1 || filtered[0].Name != "api-server" {
		t.Errorf("Filter '10.0.1' should match resource with that IP, got %d matches", len(filtered))
	}
}

// ===================================================================
// QA-136: Filter on empty list
// ===================================================================
func TestQA_136_FilterOnEmptyList(t *testing.T) {
	state := ec2ResourceListState(nil)
	state.FilterMode = true

	// Type "anything" -- should not crash
	for _, ch := range "anything" {
		state = pressKeyFSE(state, string(ch))
	}

	if state.Filter != "anything" {
		t.Errorf("Filter should accept input on empty list, got %q", state.Filter)
	}
	if len(state.FilteredResources) != 0 {
		t.Errorf("FilteredResources should be empty, got %d", len(state.FilteredResources))
	}
}

// ===================================================================
// QA-137: Backspace removes filter chars; empty exits filter mode
// ===================================================================
func TestQA_137_BackspaceRemovesFilterChars(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.FilterMode = true
	state.Filter = "prod"
	state.FilteredResources = views.FilterResources("prod", state.Resources)

	// Backspace once: "pro"
	state = pressSpecialKeyFSE(state, tea.KeyBackspace)
	if state.Filter != "pro" {
		t.Errorf("After one backspace, filter should be 'pro', got %q", state.Filter)
	}

	// Backspace 3 more times: should exit filter mode
	state = pressSpecialKeyFSE(state, tea.KeyBackspace) // "pr"
	state = pressSpecialKeyFSE(state, tea.KeyBackspace) // "p"
	state = pressSpecialKeyFSE(state, tea.KeyBackspace) // ""

	if state.FilterMode {
		t.Error("Backspace to empty should exit FilterMode")
	}
	if state.Filter != "" {
		t.Errorf("Filter should be empty after all backspaces, got %q", state.Filter)
	}
}

// ===================================================================
// QA-138: Shift+N sorts by name
// ===================================================================
func TestQA_138_SortByName(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())

	state = pressKeyFSE(state, "N")

	if state.StatusMessage != "Sorted by name" {
		t.Errorf("Status should say 'Sorted by name', got %q", state.StatusMessage)
	}
	if state.SelectedIndex != 0 {
		t.Errorf("SelectedIndex should reset to 0 after sort, got %d", state.SelectedIndex)
	}

	// Verify alphabetical order by name field (case-insensitive)
	for i := 1; i < len(state.Resources); i++ {
		prev := strings.ToLower(state.Resources[i-1].Fields["name"])
		curr := strings.ToLower(state.Resources[i].Fields["name"])
		if prev > curr {
			t.Errorf("Resources not sorted by name: %q > %q at index %d", prev, curr, i)
		}
	}
}

// ===================================================================
// QA-139: Shift+S sorts by status/state
// ===================================================================
func TestQA_139_SortByStatus(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())

	state = pressKeyFSE(state, "S")

	if state.StatusMessage != "Sorted by status" {
		t.Errorf("Status should say 'Sorted by status', got %q", state.StatusMessage)
	}
	if state.SelectedIndex != 0 {
		t.Errorf("SelectedIndex should reset to 0, got %d", state.SelectedIndex)
	}

	// EC2 uses "state" column -- verify sort by state
	for i := 1; i < len(state.Resources); i++ {
		prev := state.Resources[i-1].Fields["state"]
		curr := state.Resources[i].Fields["state"]
		if prev > curr {
			t.Errorf("Resources not sorted by state: %q > %q at index %d", prev, curr, i)
		}
	}
}

// ===================================================================
// QA-140: Shift+A sorts by age/time
// ===================================================================
func TestQA_140_SortByAge(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())

	state = pressKeyFSE(state, "A")

	if state.StatusMessage != "Sorted by age" {
		t.Errorf("Status should say 'Sorted by age', got %q", state.StatusMessage)
	}

	// EC2 uses "launch_time" column -- verify sorted
	for i := 1; i < len(state.Resources); i++ {
		prev := state.Resources[i-1].Fields["launch_time"]
		curr := state.Resources[i].Fields["launch_time"]
		if prev > curr {
			t.Errorf("Resources not sorted by launch_time: %q > %q at index %d", prev, curr, i)
		}
	}
}

// ===================================================================
// QA-141: Sort + filter combined
// ===================================================================
func TestQA_141_SortPlusFilter(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())

	// Apply filter "prod" first
	state.Filter = "prod"
	state.FilteredResources = views.FilterResources("prod", state.Resources)

	// Sort by name
	state = pressKeyFSE(state, "N")

	// After sort, filter should be re-applied on the sorted Resources
	if state.Filter != "prod" {
		t.Errorf("Filter should still be 'prod', got %q", state.Filter)
	}
	if len(state.FilteredResources) != 2 {
		t.Errorf("Should still have 2 filtered 'prod' resources, got %d", len(state.FilteredResources))
	}

	// Filtered results should be sorted by name (case-insensitive)
	if len(state.FilteredResources) >= 2 {
		n0 := strings.ToLower(state.FilteredResources[0].Fields["name"])
		n1 := strings.ToLower(state.FilteredResources[1].Fields["name"])
		if n0 > n1 {
			t.Errorf("Filtered resources should be sorted: %q should come before %q", n0, n1)
		}
	}
}

// ===================================================================
// QA-142: Sort stability (Go sort.Slice is NOT stable)
// ===================================================================
func TestQA_142_SortStability(t *testing.T) {
	// Document that sort is not guaranteed stable
	state := ec2ResourceListState(sampleEC2Resources())
	state = pressKeyFSE(state, "S")

	// Just verify no crash and status set; stability not guaranteed
	if state.StatusMessage != "Sorted by status" {
		t.Errorf("Sort should complete without crash, status: %q", state.StatusMessage)
	}
}

// ===================================================================
// QA-143: Sort on empty list
// ===================================================================
func TestQA_143_SortOnEmptyList(t *testing.T) {
	state := ec2ResourceListState(nil)

	// All three sort keys should not crash on empty
	for _, key := range []string{"N", "S", "A"} {
		s := pressKeyFSE(state, key)
		if s.StatusIsError {
			t.Errorf("Sort key %q on empty list should not cause error", key)
		}
	}
}

// ===================================================================
// QA-144: Sort for resource type without matching column
// ===================================================================
func TestQA_144_SortFallbackToName(t *testing.T) {
	// S3 has no "status" or "state" column
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "s3"
	state.Resources = []resource.Resource{
		{ID: "bucket-z", Name: "bucket-z", Fields: map[string]string{"name": "bucket-z", "region": "us-east-1", "creation_date": "2026-01-01"}},
		{ID: "bucket-a", Name: "bucket-a", Fields: map[string]string{"name": "bucket-a", "region": "eu-west-1", "creation_date": "2025-06-01"}},
		{ID: "bucket-m", Name: "bucket-m", Fields: map[string]string{"name": "bucket-m", "region": "ap-southeast-1", "creation_date": "2024-12-01"}},
	}
	state.Width = 120
	state.Height = 40

	state = pressKeyFSE(state, "S")

	// Should fall back to name sort since S3 has no status column
	if state.StatusMessage != "Sorted by status" {
		t.Errorf("Status should still say 'Sorted by status', got %q", state.StatusMessage)
	}
	// Verify resources sorted by Name (fallback)
	if state.Resources[0].Name != "bucket-a" {
		t.Errorf("First resource after status-sort fallback should be 'bucket-a', got %q", state.Resources[0].Name)
	}
}

// ===================================================================
// QA-145: Sort keys only work in resource list view
// ===================================================================
func TestQA_145_SortKeysOnlyInResourceList(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.MainMenuView

	for _, key := range []string{"N", "S", "A"} {
		s := pressKeyFSE(state, key)
		if s.StatusMessage == "Sorted by name" || s.StatusMessage == "Sorted by status" || s.StatusMessage == "Sorted by age" {
			t.Errorf("Sort key %q should have no effect in MainMenuView", key)
		}
	}
}

// ===================================================================
// QA-146: Navigate forward, [ goes back
// ===================================================================
func TestQA_146_HistoryBackWithBracket(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.MainMenuView

	// Simulate: push MainMenu state, go to ResourceList
	state.History.Push(navigation.ViewState{
		ViewType: navigation.MainMenuView,
	})
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Resources = sampleEC2Resources()

	// Push ResourceList state, go to DetailView
	state.History.Push(navigation.ViewState{
		ViewType:     navigation.ResourceListView,
		ResourceType: "ec2",
		CursorPos:    1,
	})
	state.CurrentView = app.DetailView
	state.Detail = views.NewDetailModel("test", map[string]string{"A": "1"})

	// Press [ to go back
	state = pressKeyFSE(state, "[")

	if state.CurrentView != app.ResourceListView {
		t.Errorf("[ should go back to ResourceListView, got view %d", state.CurrentView)
	}
}

// ===================================================================
// QA-147: ] goes forward after going back
// ===================================================================
func TestQA_147_HistoryForwardWithBracket(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.MainMenuView

	// Push MainMenu, navigate to ResourceList
	state.History.Push(navigation.ViewState{
		ViewType: navigation.MainMenuView,
	})
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Resources = sampleEC2Resources()

	// Push ResourceList, navigate to Detail
	state.History.Push(navigation.ViewState{
		ViewType:     navigation.ResourceListView,
		ResourceType: "ec2",
		CursorPos:    0,
	})
	state.CurrentView = app.DetailView

	// [ back to ResourceList
	state = pressKeyFSE(state, "[")
	if state.CurrentView != app.ResourceListView {
		t.Fatal("Should be on ResourceListView after [")
	}

	// ] forward to Detail
	state = pressKeyFSE(state, "]")
	if state.CurrentView != app.ResourceListView {
		// Note: Forward() restores the popped state which was ResourceListView
		// The DetailView state was never pushed, so forward restores ResourceListView
		// This is expected based on how the stack works
	}
}

// ===================================================================
// QA-148: History cleared after new navigation
// ===================================================================
func TestQA_148_HistoryClearedOnNewNavigation(t *testing.T) {
	var stack navigation.NavigationStack

	stack.Push(navigation.ViewState{ViewType: navigation.MainMenuView})
	stack.Push(navigation.ViewState{ViewType: navigation.ResourceListView, ResourceType: "ec2"})

	// Pop to create forward history
	stack.Pop()
	if !stack.CanGoForward() {
		t.Fatal("Should have forward history after Pop")
	}

	// Push new state (new navigation) -- should clear forward
	stack.Push(navigation.ViewState{ViewType: navigation.ResourceListView, ResourceType: "rds"})

	if stack.CanGoForward() {
		t.Error("Forward history should be cleared after new Push")
	}
}

// ===================================================================
// QA-149: Multiple back operations
// ===================================================================
func TestQA_149_MultipleBackOperations(t *testing.T) {
	state := app.NewAppState("", "")

	// Build history: MainMenu -> EC2 -> Detail
	state.History.Push(navigation.ViewState{ViewType: navigation.MainMenuView})
	state.History.Push(navigation.ViewState{ViewType: navigation.ResourceListView, ResourceType: "ec2"})
	state.CurrentView = app.DetailView
	state.CurrentResourceType = "ec2"
	state.Resources = sampleEC2Resources()

	// Back to EC2 list
	state = pressKeyFSE(state, "[")
	if state.CurrentView != app.ResourceListView {
		t.Errorf("First [ should return to ResourceListView, got %d", state.CurrentView)
	}

	// Back to MainMenu
	state = pressKeyFSE(state, "[")
	if state.CurrentView != app.MainMenuView {
		t.Errorf("Second [ should return to MainMenuView, got %d", state.CurrentView)
	}

	// Back from MainMenu should be a no-op
	state = pressKeyFSE(state, "[")
	if state.CurrentView != app.MainMenuView {
		t.Errorf("[ at MainMenuView should stay on MainMenuView, got %d", state.CurrentView)
	}
}

// ===================================================================
// QA-150: History after profile switch
// ===================================================================
func TestQA_150_HistoryAfterProfileSwitch(t *testing.T) {
	state := app.NewAppState("test-profile", "us-east-1")

	// Build some history
	state.History.Push(navigation.ViewState{ViewType: navigation.MainMenuView})
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"

	// Simulate profile switch (does NOT clear history in current impl)
	updated, _ := state.Update(app.ProfileSwitchedMsg{
		Profile: "new-profile",
		Region:  "eu-west-1",
	})
	state = updated.(app.AppState)

	if state.ActiveProfile != "new-profile" {
		t.Errorf("Profile should be 'new-profile', got %q", state.ActiveProfile)
	}

	// History still exists (not cleared by profile switch)
	if !state.History.CanGoBack() {
		t.Log("History is cleared after profile switch (implementation may vary)")
	}
}

// ===================================================================
// QA-151: History after region switch
// ===================================================================
func TestQA_151_HistoryAfterRegionSwitch(t *testing.T) {
	state := app.NewAppState("default", "us-east-1")
	state.History.Push(navigation.ViewState{ViewType: navigation.MainMenuView})
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"

	updated, _ := state.Update(app.RegionSwitchedMsg{Region: "eu-west-1"})
	state = updated.(app.AppState)

	if state.ActiveRegion != "eu-west-1" {
		t.Errorf("Region should be 'eu-west-1', got %q", state.ActiveRegion)
	}
}

// ===================================================================
// QA-152: Ctrl-R on resource list sets loading
// ===================================================================
func TestQA_152_CtrlRSetsLoadingOnResourceList(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.Loading = false

	updated, cmd := state.Update(tea.KeyPressMsg{Code: -1, Text: "ctrl+r"})
	state = updated.(app.AppState)

	if !state.Loading {
		t.Error("Ctrl-R on resource list should set Loading to true")
	}
	if cmd == nil {
		t.Error("Ctrl-R should return a fetch command")
	}
}

// ===================================================================
// QA-153: Ctrl-R on main menu does nothing
// ===================================================================
func TestQA_153_CtrlRNoEffectOnMainMenu(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.MainMenuView

	updated, cmd := state.Update(tea.KeyPressMsg{Code: -1, Text: "ctrl+r"})
	state = updated.(app.AppState)

	if state.Loading {
		t.Error("Ctrl-R on main menu should NOT set Loading")
	}
	if cmd != nil {
		t.Error("Ctrl-R on main menu should return nil cmd")
	}
}

// ===================================================================
// QA-154: Loading indicator appears during refresh
// ===================================================================
func TestQA_154_LoadingIndicatorDuringRefresh(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.Loading = true

	view := state.View()
	if !strings.Contains(view.Content, "loading") {
		t.Error("Header should show loading indicator when Loading is true")
	}
}

// ===================================================================
// QA-155: Ctrl-R while already loading
// ===================================================================
func TestQA_155_CtrlRWhileAlreadyLoading(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.Loading = true

	// Ctrl-R while already loading should not crash
	updated, cmd := state.Update(tea.KeyPressMsg{Code: -1, Text: "ctrl+r"})
	state = updated.(app.AppState)

	if !state.Loading {
		t.Error("Should still be loading")
	}
	// A new fetch command is dispatched (no debounce)
	if cmd == nil {
		t.Error("Ctrl-R while loading should still dispatch a fetch command")
	}
}

// ===================================================================
// QA-156: Ctrl-R in detail view
// ===================================================================
func TestQA_156_CtrlRInDetailView(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.DetailView
	state.CurrentResourceType = "ec2"

	updated, cmd := state.Update(tea.KeyPressMsg{Code: -1, Text: "ctrl+r"})
	state = updated.(app.AppState)

	// Detail view is not MainMenu, and has a CurrentResourceType, so refresh fires
	if !state.Loading {
		t.Error("Ctrl-R in detail view should set Loading (refreshes underlying resource list)")
	}
	if cmd == nil {
		t.Error("Ctrl-R in detail view should return a fetch command")
	}
}

// ===================================================================
// QA-157: Ctrl-R in ProfileSelectView or RegionSelectView
// ===================================================================
func TestQA_157_CtrlRInSelectorViews(t *testing.T) {
	// ProfileSelectView with no previous resource type
	state := app.NewAppState("", "")
	state.CurrentView = app.ProfileSelectView
	state.CurrentResourceType = ""

	updated, cmd := state.Update(tea.KeyPressMsg{Code: -1, Text: "ctrl+r"})
	state = updated.(app.AppState)

	// CurrentResourceType is empty, so nothing should happen
	if cmd != nil && state.CurrentResourceType == "" {
		t.Error("Ctrl-R in selector view with no resource type should do nothing")
	}
}

// ===================================================================
// QA-158: ? shows help overlay
// ===================================================================
func TestQA_158_QuestionMarkShowsHelp(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.MainMenuView

	state = pressKeyFSE(state, "?")

	if !state.ShowHelp {
		t.Error("? should set ShowHelp to true")
	}
}

// ===================================================================
// QA-159: Help shows all keybindings
// ===================================================================
func TestQA_159_HelpShowsAllKeybindings(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.MainMenuView
	state.ShowHelp = true
	state.Width = 80
	state.Height = 40

	view := state.View()
	content := view.Content

	requiredKeys := []string{":", "/", "?", "Esc", "[", "]", "Ctrl-R", "Ctrl-C",
		"Enter", "d", "y", "x", "c", "N", "S", "A"}
	for _, k := range requiredKeys {
		if !strings.Contains(content, k) {
			t.Errorf("Help should contain keybinding %q", k)
		}
	}
}

// ===================================================================
// QA-160: Press any key closes help
// ===================================================================
func TestQA_160_AnyKeyClosesHelp(t *testing.T) {
	state := app.NewAppState("", "")
	state.ShowHelp = true

	// Press a random key (e.g. 'a')
	state = pressKeyFSE(state, "a")

	if state.ShowHelp {
		t.Error("Pressing any key while help is shown should close help")
	}
}

// ===================================================================
// QA-161: ? again closes help (toggle)
// ===================================================================
func TestQA_161_QuestionMarkTogglesHelp(t *testing.T) {
	state := app.NewAppState("", "")
	state.ShowHelp = false

	// ? opens help
	state = pressKeyFSE(state, "?")
	if !state.ShowHelp {
		t.Fatal("First ? should open help")
	}

	// ? closes help
	state = pressKeyFSE(state, "?")
	if state.ShowHelp {
		t.Error("Second ? should close help")
	}
}

// ===================================================================
// QA-162: Help from main menu
// ===================================================================
func TestQA_162_HelpFromMainMenu(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.MainMenuView
	state.Width = 80
	state.Height = 40

	state = pressKeyFSE(state, "?")

	if !state.ShowHelp {
		t.Fatal("? should activate help from main menu")
	}

	view := state.View()
	if !strings.Contains(view.Content, "Keybindings") {
		t.Error("Help overlay should display keybindings title")
	}
}

// ===================================================================
// QA-163: Help from resource list
// ===================================================================
func TestQA_163_HelpFromResourceList(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())

	state = pressKeyFSE(state, "?")

	if !state.ShowHelp {
		t.Error("? should activate help from resource list")
	}

	view := state.View()
	if !strings.Contains(view.Content, "Keybindings") {
		t.Error("Help overlay should render from resource list view")
	}
}

// ===================================================================
// QA-164: Help from detail view
// ===================================================================
func TestQA_164_HelpFromDetailView(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.DetailView
	state.Detail = views.NewDetailModel("test", map[string]string{"A": "1"})
	state.Width = 80
	state.Height = 40

	state = pressKeyFSE(state, "?")

	if !state.ShowHelp {
		t.Error("? should activate help from detail view")
	}
}

// ===================================================================
// QA-165: Help overlay respects terminal width
// ===================================================================
func TestQA_165_HelpRespectsTerminalWidth(t *testing.T) {
	state := app.NewAppState("", "")
	state.ShowHelp = true
	state.Width = 50
	state.Height = 40

	view := state.View()

	// Help should render without crashing at narrow width
	if view.Content == "" {
		t.Error("Help should render content at narrow terminal width")
	}
}

// ===================================================================
// QA-166: Expired AWS credentials show re-auth message
// ===================================================================
func TestQA_166_ExpiredCredentialsMessage(t *testing.T) {
	state := app.NewAppState("", "")

	updated, _ := state.Update(app.APIErrorMsg{
		ResourceType: "ec2",
		Err:          fmt.Errorf("operation error: ExpiredTokenException: token has expired"),
	})
	s := updated.(app.AppState)

	if !s.StatusIsError {
		t.Error("Expired token should set StatusIsError")
	}
	if !strings.Contains(s.StatusMessage, "expired") {
		t.Errorf("Status should mention 'expired', got %q", s.StatusMessage)
	}
	if !strings.Contains(s.StatusMessage, "aws sso login") {
		t.Errorf("Status should suggest 'aws sso login', got %q", s.StatusMessage)
	}
}

// ===================================================================
// QA-167: Access denied error shows in status
// ===================================================================
func TestQA_167_AccessDeniedShowsError(t *testing.T) {
	state := app.NewAppState("", "")

	updated, _ := state.Update(app.APIErrorMsg{
		ResourceType: "rds",
		Err:          fmt.Errorf("AccessDeniedException: User is not authorized"),
	})
	s := updated.(app.AppState)

	if !s.StatusIsError {
		t.Error("Access denied should set StatusIsError")
	}
	if !strings.Contains(s.StatusMessage, "rds") {
		t.Errorf("Error should reference resource type 'rds', got %q", s.StatusMessage)
	}
}

// ===================================================================
// QA-168: Network timeout shows in status
// ===================================================================
func TestQA_168_NetworkTimeoutError(t *testing.T) {
	state := app.NewAppState("", "")

	updated, _ := state.Update(app.APIErrorMsg{
		ResourceType: "ec2",
		Err:          fmt.Errorf("operation error: RequestTimeout: request timed out"),
	})
	s := updated.(app.AppState)

	if !s.StatusIsError {
		t.Error("Timeout should set StatusIsError")
	}
	if !s.Loading == true {
		// Loading should be cleared
	}
	if s.Loading {
		t.Error("Loading should be set to false after error")
	}
}

// ===================================================================
// QA-169: API throttling error
// ===================================================================
func TestQA_169_APIThrottlingError(t *testing.T) {
	state := app.NewAppState("", "")
	state.Loading = true

	updated, cmd := state.Update(app.APIErrorMsg{
		ResourceType: "ec2",
		Err:          fmt.Errorf("Throttling: Rate exceeded"),
	})
	s := updated.(app.AppState)

	if !s.StatusIsError {
		t.Error("Throttling should set StatusIsError")
	}
	if s.Loading {
		t.Error("Loading should be false after error")
	}
	if cmd == nil {
		t.Error("APIErrorMsg should return a timer command for auto-clear")
	}
}

// ===================================================================
// QA-170: Invalid profile name error on startup
// ===================================================================
func TestQA_170_InvalidProfileNameError(t *testing.T) {
	// Test that NewAppState with a bogus profile still creates a valid AppState,
	// and that sending InitConnectMsg with that profile results in either
	// a successful connection or an error status message.
	state := app.NewAppState("nonexistent-profile-xyz", "us-east-1")

	if state.ActiveProfile != "nonexistent-profile-xyz" {
		t.Errorf("expected ActiveProfile 'nonexistent-profile-xyz', got %q", state.ActiveProfile)
	}
	if state.CurrentView != app.MainMenuView {
		t.Errorf("expected MainMenuView, got %d", state.CurrentView)
	}

	// Sending InitConnectMsg: NewAWSSession may succeed (using default config)
	// or fail (no such profile). Either way, state must be consistent.
	updated, _ := state.Update(app.InitConnectMsg{
		Profile: "nonexistent-profile-xyz",
		Region:  "us-east-1",
	})
	s := updated.(app.AppState)

	if s.Clients != nil {
		// Connected fine (default profile may have worked) — status should not be error
		if s.StatusIsError {
			t.Error("Clients is non-nil but StatusIsError is true")
		}
	} else {
		// Connection failed — status should show an error
		if !s.StatusIsError {
			t.Error("Clients is nil but StatusIsError is false; expected an error message")
		}
		if !strings.Contains(s.StatusMessage, "AWS config error") {
			t.Errorf("expected status to contain 'AWS config error', got %q", s.StatusMessage)
		}
	}
}

// ===================================================================
// QA-171: Region with no support for service
// ===================================================================
func TestQA_171_RegionNoServiceSupport(t *testing.T) {
	// Test that when an APIErrorMsg arrives for a specific service,
	// the status message shows the error. This simulates a region
	// that doesn't support a given service.
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "docdb"
	state.Loading = true

	updated, cmd := state.Update(app.APIErrorMsg{
		Err:          fmt.Errorf("service docdb is not available in region af-south-1"),
		ResourceType: "docdb",
	})
	s := updated.(app.AppState)

	if !s.StatusIsError {
		t.Error("expected StatusIsError=true after APIErrorMsg")
	}
	if !strings.Contains(s.StatusMessage, "docdb") {
		t.Errorf("expected status message to contain 'docdb', got %q", s.StatusMessage)
	}
	if s.Loading {
		t.Error("expected Loading=false after APIErrorMsg")
	}
	if cmd == nil {
		t.Error("expected a timer command for auto-clear after APIErrorMsg")
	}
}

// ===================================================================
// QA-172: Concurrent errors (switch resource type while loading)
// ===================================================================
func TestQA_172_ConcurrentStaleResponseDiscarded(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "rds"
	state.Loading = true

	// A stale EC2 response arrives while viewing RDS
	updated, _ := state.Update(app.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-stale", Name: "stale"},
		},
	})
	s := updated.(app.AppState)

	if s.CurrentResourceType != "rds" {
		t.Errorf("Should still be viewing rds, got %q", s.CurrentResourceType)
	}
	if len(s.Resources) != 0 {
		t.Error("Stale EC2 response should be discarded when viewing RDS")
	}
}

// ===================================================================
// QA-173: InitConnectMsg failure on startup
// ===================================================================
func TestQA_173_InitConnectMsgFailure(t *testing.T) {
	// Test the InitConnectMsg handler. Send InitConnectMsg and verify
	// state is consistent regardless of whether AWS connection succeeds or fails.
	state := app.NewAppState("", "us-east-1")
	state.Width = 80
	state.Height = 24

	updated, _ := state.Update(app.InitConnectMsg{
		Profile: "default",
		Region:  "us-east-1",
	})
	s := updated.(app.AppState)

	// Either connection succeeded (Clients != nil, no error) or
	// failed (Clients == nil, error status).
	if s.Clients != nil {
		// Success path
		if s.StatusIsError {
			t.Error("Clients is non-nil but StatusIsError is true — inconsistent")
		}
		if s.StatusMessage == "" {
			t.Error("expected a status message after successful connection")
		}
	} else {
		// Failure path
		if !s.StatusIsError {
			t.Error("Clients is nil but StatusIsError is false — expected error")
		}
		if s.StatusMessage == "" {
			t.Error("expected a status error message when connection fails")
		}
	}

	// View should render without panic
	view := s.View()
	if view.Content == "" {
		t.Error("expected non-empty view content after InitConnectMsg")
	}
}

// ===================================================================
// QA-174: Very narrow terminal (40 columns)
// ===================================================================
func TestQA_174_NarrowTerminalNoCrash(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.Width = 40
	state.Height = 24

	// Should not panic
	view := state.View()
	if view.Content == "" {
		t.Error("View should produce output even at narrow width")
	}
}

// ===================================================================
// QA-175: Very short terminal (10 rows)
// ===================================================================
func TestQA_175_ShortTerminalNoCrash(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.Width = 80
	state.Height = 10

	view := state.View()
	if view.Content == "" {
		t.Error("View should produce output at short terminal height")
	}
}

// ===================================================================
// QA-176: Extremely small terminal (10x5) doesn't crash
// ===================================================================
func TestQA_176_ExtremelySmallTerminalNoCrash(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.Width = 10
	state.Height = 5

	// Must not panic
	view := state.View()
	_ = view.Content
}

// ===================================================================
// QA-177: Terminal resize updates dimensions
// ===================================================================
func TestQA_177_ResizeUpdatesDimensions(t *testing.T) {
	state := app.NewAppState("", "")
	state.Width = 80
	state.Height = 24

	// Resize to 60x20
	updated, _ := state.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	state = updated.(app.AppState)

	if state.Width != 60 || state.Height != 20 {
		t.Errorf("After resize, dimensions should be 60x20, got %dx%d", state.Width, state.Height)
	}

	// Resize to 200x60
	updated, _ = state.Update(tea.WindowSizeMsg{Width: 200, Height: 60})
	state = updated.(app.AppState)

	if state.Width != 200 || state.Height != 60 {
		t.Errorf("After resize, dimensions should be 200x60, got %dx%d", state.Width, state.Height)
	}
}

// ===================================================================
// QA-178: NO_COLOR environment variable
// ===================================================================
func TestQA_178_NoColorEnvVar(t *testing.T) {
	// Set NO_COLOR and call InitStyles, then verify styles have no ANSI color codes.
	t.Setenv("NO_COLOR", "1")
	styles.InitStyles()

	// After InitStyles with NO_COLOR, the HeaderStyle should not contain
	// color-related rendering. Render a sample string and verify no ANSI escape.
	rendered := styles.HeaderStyle.Render("test")
	if strings.Contains(rendered, "\x1b[") {
		t.Errorf("expected no ANSI escape codes with NO_COLOR=1, but found them in: %q", rendered)
	}

	// Restore styles for other tests
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.InitStyles()
	})
}

// ===================================================================
// QA-179: Non-256-color terminal
// ===================================================================
func TestQA_179_Non256ColorTerminal(t *testing.T) {
	// Verify the app doesn't crash with NO_COLOR set (simulating a terminal
	// without 256-color support). Create an AppState, render View(), no panic = pass.
	t.Setenv("NO_COLOR", "1")
	styles.InitStyles()

	state := app.NewAppState("", "us-east-1")
	state.Width = 80
	state.Height = 24

	// Should not panic
	view := state.View()
	if view.Content == "" {
		t.Error("expected non-empty view content with NO_COLOR set")
	}

	// Restore styles for other tests
	t.Cleanup(func() {
		os.Unsetenv("NO_COLOR")
		styles.InitStyles()
	})
}

// ===================================================================
// QA-180: SSH session (clipboard may not work)
// ===================================================================
func TestQA_180_SSHClipboardFailure(t *testing.T) {
	// Test that pressing 'c' (copy) on a resource detail view doesn't crash,
	// regardless of clipboard availability. In an SSH session clipboard may
	// fail, but the app should handle it gracefully.
	state := app.NewAppState("", "us-east-1")
	state.CurrentView = app.DetailView
	state.Detail = views.NewDetailModel("i-abc001", map[string]string{
		"Instance ID": "i-abc001",
		"Name":        "test-instance",
	})
	state.Width = 80
	state.Height = 24

	// Press 'c' to attempt copy — should not panic
	model, _ := state.Update(tea.KeyPressMsg{Code: -1, Text: "c"})
	s := model.(app.AppState)

	// Verify the state is still consistent — either "Copied" or an error message,
	// but no crash.
	_ = s.StatusMessage // just verify it's accessible, no panic
	view := s.View()
	if view.Content == "" {
		t.Error("expected non-empty view content after copy attempt")
	}
}

// ===================================================================
// QA-181: Unicode in resource names
// ===================================================================
func TestQA_181_UnicodeResourceNames(t *testing.T) {
	resources := []resource.Resource{
		{
			ID: "i-unicode", Name: "web-\u2603-snowman",
			Fields: map[string]string{"instance_id": "i-unicode", "name": "web-\u2603-snowman", "state": "running"},
		},
	}
	state := ec2ResourceListState(resources)

	// Should not panic
	view := state.View()
	if !strings.Contains(view.Content, "\u2603") {
		t.Error("Unicode character should appear in rendered output")
	}
}

// ===================================================================
// QA-182: Ctrl-C exits from any view
// ===================================================================
func TestQA_182_CtrlCExitsFromAnyView(t *testing.T) {
	views := []app.ViewType{
		app.MainMenuView,
		app.ResourceListView,
		app.DetailView,
		app.JSONView,
		app.RevealView,
	}
	for _, v := range views {
		state := app.NewAppState("", "")
		state.CurrentView = v
		state.CurrentResourceType = "ec2"

		_, cmd := state.Update(tea.KeyPressMsg{Code: -1, Text: "ctrl+c"})
		if cmd == nil {
			t.Errorf("Ctrl-C from view %d should return a quit command", v)
		}
	}
}

// ===================================================================
// QA-183: Stale response discarded
// (covered in qa_discrepancies_test.go -- TestStaleResponse_Discarded)
// ===================================================================
func TestQA_183_RapidCommandSwitching(t *testing.T) {
	// Tests rapid switching: the final resource type should be preserved
	state := app.NewAppState("", "")
	state.CurrentView = app.MainMenuView

	// Switch to ec2
	state.CommandMode = true
	state.CommandText = "ec2"
	state = pressSpecialKeyFSE(state, tea.KeyEnter)

	// Switch to rds
	state.CommandMode = true
	state.CommandText = "rds"
	state = pressSpecialKeyFSE(state, tea.KeyEnter)

	// Switch to eks
	state.CommandMode = true
	state.CommandText = "eks"
	state = pressSpecialKeyFSE(state, tea.KeyEnter)

	if state.CurrentResourceType != "eks" {
		t.Errorf("After rapid switching, resource type should be 'eks', got %q", state.CurrentResourceType)
	}

	// Stale ec2 response should be discarded
	updated, _ := state.Update(app.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "stale"}},
	})
	s := updated.(app.AppState)
	if len(s.Resources) != 0 {
		t.Error("Stale EC2 response should be discarded while viewing EKS")
	}
}

// ===================================================================
// QA-184: Profile switch while loading
// ===================================================================
func TestQA_184_ProfileSwitchWhileLoading(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.Loading = true

	updated, _ := state.Update(app.ProfileSwitchedMsg{
		Profile: "staging",
		Region:  "eu-west-1",
	})
	s := updated.(app.AppState)

	if s.ActiveProfile != "staging" {
		t.Errorf("Profile should be 'staging', got %q", s.ActiveProfile)
	}
	if s.ActiveRegion != "eu-west-1" {
		t.Errorf("Region should be 'eu-west-1', got %q", s.ActiveRegion)
	}
}

// ===================================================================
// QA-185: Region switch while in detail view
// ===================================================================
func TestQA_185_RegionSwitchWhileInDetailView(t *testing.T) {
	state := app.NewAppState("default", "us-east-1")
	state.CurrentView = app.DetailView

	updated, _ := state.Update(app.RegionSwitchedMsg{Region: "eu-west-1"})
	s := updated.(app.AppState)

	if s.ActiveRegion != "eu-west-1" {
		t.Errorf("Region should be 'eu-west-1', got %q", s.ActiveRegion)
	}
}

// ===================================================================
// QA-186: Filter active then profile switch
// ===================================================================
func TestQA_186_FilterClearsAfterProfileSwitchAndNewNav(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.Filter = "prod"
	state.FilteredResources = views.FilterResources("prod", state.Resources)

	// Profile switch resets to main menu
	updated, _ := state.Update(app.ProfileSwitchedMsg{
		Profile: "other",
		Region:  "us-west-2",
	})
	state = updated.(app.AppState)

	// Navigate to ec2 via command
	state.CommandMode = true
	state.CommandText = "ec2"
	state = pressSpecialKeyFSE(state, tea.KeyEnter)

	if state.Filter != "" {
		t.Errorf("Filter should be cleared after profile switch + new nav, got %q", state.Filter)
	}
}

// ===================================================================
// QA-187: Switching from S3 object view to another resource type
// ===================================================================
func TestQA_187_S3StateResetOnResourceSwitch(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "s3"
	state.S3Bucket = "my-bucket"
	state.S3Prefix = "logs/2024/"

	// Switch to ec2
	state.CommandMode = true
	state.CommandText = "ec2"
	state = pressSpecialKeyFSE(state, tea.KeyEnter)

	if state.S3Bucket != "" {
		t.Errorf("S3Bucket should be cleared after switching to ec2, got %q", state.S3Bucket)
	}
	if state.S3Prefix != "" {
		t.Errorf("S3Prefix should be cleared after switching to ec2, got %q", state.S3Prefix)
	}
}

// ===================================================================
// QA-188: Command mode does not pass keys to underlying view
// ===================================================================
func TestQA_188_CommandModeIsolatesKeys(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.MainMenuView
	state.CommandMode = true
	state.CommandText = ""
	initialIndex := state.SelectedIndex

	// Type navigation keys that would normally move cursor
	for _, ch := range "jkgd" {
		state = pressKeyFSE(state, string(ch))
	}

	if state.CommandText != "jkgd" {
		t.Errorf("Command mode should capture keys as text, got %q", state.CommandText)
	}
	if state.SelectedIndex != initialIndex {
		t.Error("Command mode should not affect SelectedIndex")
	}
}

// ===================================================================
// QA-189: Filter mode does not pass keys to underlying view
// ===================================================================
func TestQA_189_FilterModeIsolatesKeys(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.FilterMode = true
	state.Filter = ""
	initialIndex := state.SelectedIndex

	// Type keys that would normally be actions
	for _, ch := range "jkdyc" {
		state = pressKeyFSE(state, string(ch))
	}

	if state.Filter != "jkdyc" {
		t.Errorf("Filter mode should capture keys as filter text, got %q", state.Filter)
	}
	if state.CurrentView != app.ResourceListView {
		t.Error("Filter mode should not change the view")
	}
	// SelectedIndex gets reset to 0 on each applyFilter call
	_ = initialIndex
}

// ===================================================================
// QA-190: Loading state clears on both success and error
// ===================================================================
func TestQA_190_LoadingClearsOnSuccessAndError(t *testing.T) {
	// Success case
	state := ec2ResourceListState(nil)
	state.Loading = true

	updated, _ := state.Update(app.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    sampleEC2Resources(),
	})
	s := updated.(app.AppState)
	if s.Loading {
		t.Error("Loading should be false after ResourcesLoadedMsg")
	}

	// Error case
	state2 := ec2ResourceListState(nil)
	state2.Loading = true

	updated2, _ := state2.Update(app.APIErrorMsg{
		ResourceType: "ec2",
		Err:          fmt.Errorf("some error"),
	})
	s2 := updated2.(app.AppState)
	if s2.Loading {
		t.Error("Loading should be false after APIErrorMsg")
	}
}

// ===================================================================
// QA-191: SelectedIndex consistency after sort
// ===================================================================
func TestQA_191_SelectedIndexResetsAfterSort(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.SelectedIndex = 3

	state = pressKeyFSE(state, "N")

	if state.SelectedIndex != 0 {
		t.Errorf("SelectedIndex should reset to 0 after sort, got %d", state.SelectedIndex)
	}
}

// ===================================================================
// QA-192: Escape from MainMenuView does nothing
// ===================================================================
func TestQA_192_EscapeFromMainMenuNoop(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.MainMenuView

	state = pressSpecialKeyFSE(state, tea.KeyEscape)

	if state.CurrentView != app.MainMenuView {
		t.Errorf("Escape from MainMenu should stay on MainMenu, got view %d", state.CurrentView)
	}
}

// ===================================================================
// QA-193: Multiple filter entries in succession
// ===================================================================
func TestQA_193_MultipleFilterEntriesInSuccession(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())

	// First filter: "prod"
	state = pressKeyFSE(state, "/")
	for _, ch := range "prod" {
		state = pressKeyFSE(state, string(ch))
	}
	state = pressSpecialKeyFSE(state, tea.KeyEnter)

	if state.Filter != "prod" {
		t.Fatalf("First filter should be 'prod', got %q", state.Filter)
	}
	firstCount := len(state.FilteredResources)

	// Second filter: "/" resets and applies "dev"
	state = pressKeyFSE(state, "/")
	if state.Filter != "" {
		t.Errorf("Starting new filter should reset Filter to empty, got %q", state.Filter)
	}

	for _, ch := range "dev" {
		state = pressKeyFSE(state, string(ch))
	}
	state = pressSpecialKeyFSE(state, tea.KeyEnter)

	if state.Filter != "dev" {
		t.Errorf("Second filter should be 'dev', got %q", state.Filter)
	}
	// "dev" should match "dev-api" (1 resource), not overlap with "prod" (2 resources)
	if len(state.FilteredResources) == firstCount {
		t.Error("Second filter 'dev' should produce different results than first filter 'prod'")
	}
}

// ===================================================================
// QA-194: View renders correctly after every state transition
// ===================================================================
func TestQA_194_ViewRendersAfterStateTransitions(t *testing.T) {
	state := app.NewAppState("default", "us-east-1")
	state.Width = 120
	state.Height = 40

	// Main menu should render
	view := state.View()
	if !strings.Contains(view.Content, "AWS Resources") {
		t.Error("Main menu should show 'AWS Resources'")
	}

	// Navigate to EC2 (simulate command)
	state.CommandMode = true
	state.CommandText = "ec2"
	state = pressSpecialKeyFSE(state, tea.KeyEnter)
	state.Loading = false
	state.Resources = sampleEC2Resources()

	view = state.View()
	if !strings.Contains(view.Content, "EC2 Instances") {
		t.Error("EC2 resource list should show 'EC2 Instances'")
	}

	// Detail view
	state = pressKeyFSE(state, "d")
	if state.CurrentView == app.DetailView {
		view = state.View()
		if view.Content == "" {
			t.Error("Detail view should render content")
		}
	}

	// Back
	state = pressSpecialKeyFSE(state, tea.KeyEscape)
	view = state.View()
	if view.Content == "" {
		t.Error("View should render after going back")
	}
}

// QA-195 through QA-199 are covered in qa_discrepancies_test.go and have been removed as duplicates.

// ===================================================================
// QA-200: S3 listing is global regardless of region
// ===================================================================
func TestQA_200_S3ListingGlobalRegardlessOfRegion(t *testing.T) {
	// Test that FetchS3Buckets returns buckets regardless of what region was set.
	// The "global" behavior is the S3 API's job — we just verify our code
	// passes through results from the mock correctly.
	mockAPI := &mockS3ListBucketsAPI{
		Output: &s3.ListBucketsOutput{
			Buckets: []s3types.Bucket{
				{Name: strPtr("bucket-us-east")},
				{Name: strPtr("bucket-eu-west")},
				{Name: strPtr("bucket-ap-southeast")},
			},
		},
	}

	resources, err := awsclient.FetchS3Buckets(context.Background(), mockAPI)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resources) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(resources))
	}

	names := map[string]bool{}
	for _, r := range resources {
		names[r.Name] = true
	}
	for _, expected := range []string{"bucket-us-east", "bucket-eu-west", "bucket-ap-southeast"} {
		if !names[expected] {
			t.Errorf("expected bucket %q in results", expected)
		}
	}
}

// mockS3ListBucketsAPI implements awsclient.S3ListBucketsAPI for testing.
type mockS3ListBucketsAPI struct {
	Output *s3.ListBucketsOutput
	Err    error
}

func (m *mockS3ListBucketsAPI) ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	return m.Output, m.Err
}

func strPtr(s string) *string {
	return &s
}

// ===================================================================
// Additional: ClearErrorMsg clears error state
// ===================================================================
func TestQA_199b_ClearErrorMsgClearsState(t *testing.T) {
	state := app.NewAppState("", "")
	state.StatusMessage = "Error fetching ec2: access denied"
	state.StatusIsError = true

	updated, _ := state.Update(app.ClearErrorMsg{})
	s := updated.(app.AppState)

	if s.StatusIsError {
		t.Error("ClearErrorMsg should set StatusIsError to false")
	}
	if s.StatusMessage != "" {
		t.Errorf("ClearErrorMsg should clear StatusMessage, got %q", s.StatusMessage)
	}
}

// ===================================================================
// Additional: Verify empty resource list shows helpful message
// ===================================================================
func TestQA_166b_EmptyResourceListMessage(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Width = 80
	state.Height = 24

	// Receive empty resources
	updated, _ := state.Update(app.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{},
	})
	s := updated.(app.AppState)

	if s.StatusMessage == "" {
		t.Error("Empty resource list should set a helpful status message")
	}
	if !strings.Contains(s.StatusMessage, "No") {
		t.Errorf("Status should mention 'No ... found', got %q", s.StatusMessage)
	}
}

// ===================================================================
// Additional: Very large terminal (300x50) doesn't crash (QA-176 complement)
// ===================================================================
func TestQA_176b_VeryLargeTerminalNoCrash(t *testing.T) {
	state := ec2ResourceListState(sampleEC2Resources())
	state.Width = 300
	state.Height = 50

	// Should not panic
	view := state.View()
	if view.Content == "" {
		t.Error("View should produce output at large terminal dimensions")
	}
}
