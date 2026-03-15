package unit

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/app"
	awsclient "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/internal/resource"
	"github.com/k2m30/a9s/internal/views"
)

// ===========================================================================
// Helper: create a key press message for a single character key.
// ===========================================================================

func charKey(ch string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: ch}
}

// ===========================================================================
// Helper: build mock resources for a given resource type with populated Fields.
// ===========================================================================

func mockResourcesForType(shortName string, count int) []resource.Resource {
	rt := resource.FindResourceType(shortName)
	if rt == nil {
		return nil
	}
	resources := make([]resource.Resource, count)
	for i := range resources {
		fields := make(map[string]string)
		for _, col := range rt.Columns {
			fields[col.Key] = fmt.Sprintf("%s-%d", col.Key, i)
		}
		resources[i] = resource.Resource{
			ID:     fmt.Sprintf("id-%s-%d", shortName, i),
			Name:   fmt.Sprintf("name-%s-%d", shortName, i),
			Status: fmt.Sprintf("status-%d", i),
			Fields: fields,
		}
	}
	return resources
}

// ===========================================================================
// Helper: set up an AppState in ResourceListView with mock data.
// ===========================================================================

func setupResourceListState(shortName string, count int) app.AppState {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = shortName
	state.Resources = mockResourcesForType(shortName, count)
	state.Width = 160
	state.Height = 40
	state.SelectedIndex = 0
	return state
}

// ===========================================================================
// QA-060: EC2 list shows correct column headers
// ===========================================================================

func TestQA_060_EC2ColumnHeaders(t *testing.T) {
	state := setupResourceListState("ec2", 3)
	view := state.View()
	content := view.Content

	expectedHeaders := []string{"Instance ID", "Name", "State", "Type", "Private IP", "Public IP", "Launch Time"}
	for _, header := range expectedHeaders {
		if !strings.Contains(content, header) {
			t.Errorf("EC2 list view should contain column header %q", header)
		}
	}
}

// ===========================================================================
// QA-061: S3 list shows correct column headers
// ===========================================================================

func TestQA_061_S3ColumnHeaders(t *testing.T) {
	state := setupResourceListState("s3", 3)
	view := state.View()
	content := view.Content

	expectedHeaders := []string{"Bucket Name", "Creation Date"}
	for _, header := range expectedHeaders {
		if !strings.Contains(content, header) {
			t.Errorf("S3 list view should contain column header %q", header)
		}
	}
}

// ===========================================================================
// QA-062: RDS list shows correct column headers
// ===========================================================================

func TestQA_062_RDSColumnHeaders(t *testing.T) {
	state := setupResourceListState("rds", 3)
	view := state.View()
	content := view.Content

	expectedHeaders := []string{"DB Identifier", "Engine", "Version", "Status", "Class", "Endpoint", "Multi-AZ"}
	for _, header := range expectedHeaders {
		if !strings.Contains(content, header) {
			t.Errorf("RDS list view should contain column header %q", header)
		}
	}
}

// ===========================================================================
// QA-063: Redis list shows correct column headers
// ===========================================================================

func TestQA_063_RedisColumnHeaders(t *testing.T) {
	state := setupResourceListState("redis", 3)
	view := state.View()
	content := view.Content

	expectedHeaders := []string{"Cluster ID", "Version", "Node Type", "Status", "Nodes", "Endpoint"}
	for _, header := range expectedHeaders {
		if !strings.Contains(content, header) {
			t.Errorf("Redis list view should contain column header %q", header)
		}
	}
}

// ===========================================================================
// QA-064: DocumentDB list shows correct column headers
// ===========================================================================

func TestQA_064_DocDBColumnHeaders(t *testing.T) {
	state := setupResourceListState("docdb", 3)
	view := state.View()
	content := view.Content

	expectedHeaders := []string{"Cluster ID", "Version", "Status", "Instances", "Endpoint"}
	for _, header := range expectedHeaders {
		if !strings.Contains(content, header) {
			t.Errorf("DocumentDB list view should contain column header %q", header)
		}
	}
}

// ===========================================================================
// QA-065: EKS list shows correct column headers
// ===========================================================================

func TestQA_065_EKSColumnHeaders(t *testing.T) {
	state := setupResourceListState("eks", 3)
	view := state.View()
	content := view.Content

	expectedHeaders := []string{"Cluster Name", "Version", "Status", "Endpoint", "Platform Version"}
	for _, header := range expectedHeaders {
		if !strings.Contains(content, header) {
			t.Errorf("EKS list view should contain column header %q", header)
		}
	}
}

// ===========================================================================
// QA-066: Secrets Manager list shows correct column headers
// ===========================================================================

func TestQA_066_SecretsColumnHeaders(t *testing.T) {
	state := setupResourceListState("secrets", 3)
	view := state.View()
	content := view.Content

	expectedHeaders := []string{"Secret Name", "Description", "Last Accessed", "Last Changed", "Rotation"}
	for _, header := range expectedHeaders {
		if !strings.Contains(content, header) {
			t.Errorf("Secrets list view should contain column header %q", header)
		}
	}
}

// ===========================================================================
// QA-067: Empty resource list shows "No resources found" message
// ===========================================================================

func TestQA_067_EmptyResourceListMessage(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Resources = []resource.Resource{} // empty
	state.Width = 120
	state.Height = 24

	view := state.View()
	content := view.Content

	if !strings.Contains(content, "No ec2 resources found") {
		t.Errorf("Empty resource list should show 'No ec2 resources found', got content:\n%s", content)
	}
}

// ===========================================================================
// QA-068: Large list (100+ items) renders without panic
// ===========================================================================

func TestQA_068_LargeListRendersWithoutPanic(t *testing.T) {
	state := setupResourceListState("ec2", 150)
	state.Height = 24

	// Should not panic
	view := state.View()
	content := view.Content

	// Verify row count is displayed
	if !strings.Contains(content, "EC2 Instances (150)") {
		t.Errorf("Large list should show count (150), got content:\n%s", content)
	}

	// Navigate to the bottom
	for i := 0; i < 149; i++ {
		updated, _ := state.Update(charKey("j"))
		state = updated.(app.AppState)
	}

	if state.SelectedIndex != 149 {
		t.Errorf("Should be able to navigate to last item (149), got %d", state.SelectedIndex)
	}

	// Render again at bottom -- should not panic
	view = state.View()
	if !strings.Contains(view.Content, "instance_id-149") {
		t.Error("Last item should be visible when cursor is at the bottom")
	}
}

// ===========================================================================
// QA-069: Column truncation on narrow terminal (Width=40)
// ===========================================================================

func TestQA_069_NarrowTerminalRendersWithoutPanic(t *testing.T) {
	state := setupResourceListState("ec2", 5)
	state.Width = 40
	state.Height = 20

	// Should not panic even though columns are wider than terminal
	view := state.View()
	content := view.Content

	if content == "" {
		t.Error("Narrow terminal should still render content, got empty string")
	}

	// The header text should still be present
	if !strings.Contains(content, "EC2 Instances") {
		t.Error("Narrow terminal should still show resource type name")
	}
}

// ===========================================================================
// QA-070: j/k navigation with bounds checking
// ===========================================================================

func TestQA_070_JKNavigationBoundsChecking(t *testing.T) {
	state := setupResourceListState("ec2", 5)

	// Verify cursor starts at 0
	if state.SelectedIndex != 0 {
		t.Errorf("Cursor should start at 0, got %d", state.SelectedIndex)
	}

	// Press j three times
	for i := 0; i < 3; i++ {
		updated, _ := state.Update(charKey("j"))
		state = updated.(app.AppState)
	}
	if state.SelectedIndex != 3 {
		t.Errorf("After 3x j, cursor should be at 3, got %d", state.SelectedIndex)
	}

	// Press k once
	updated, _ := state.Update(charKey("k"))
	state = updated.(app.AppState)
	if state.SelectedIndex != 2 {
		t.Errorf("After 1x k, cursor should be at 2, got %d", state.SelectedIndex)
	}

	// Press k many times -- should stop at 0
	for i := 0; i < 10; i++ {
		updated, _ := state.Update(charKey("k"))
		state = updated.(app.AppState)
	}
	if state.SelectedIndex != 0 {
		t.Errorf("Cursor should stop at 0, got %d", state.SelectedIndex)
	}

	// Press j many times -- should stop at 4 (last index for 5 items)
	for i := 0; i < 10; i++ {
		updated, _ := state.Update(charKey("j"))
		state = updated.(app.AppState)
	}
	if state.SelectedIndex != 4 {
		t.Errorf("Cursor should stop at 4, got %d", state.SelectedIndex)
	}
}

// ===========================================================================
// QA-071: Row count display
// ===========================================================================

func TestQA_071_RowCountDisplay(t *testing.T) {
	// Without filter
	state := setupResourceListState("ec2", 10)
	view := state.View()
	if !strings.Contains(view.Content, "EC2 Instances (10)") {
		t.Errorf("Should display 'EC2 Instances (10)', content:\n%s", view.Content)
	}

	// With filter -- activate filter mode and type "3"
	state.FilterMode = true
	updated, _ := state.Update(charKey("3"))
	state = updated.(app.AppState)

	// Commit filter
	updated, _ = state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)

	view = state.View()
	content := view.Content

	// The filtered view should show count in title and filter in status bar
	if state.Filter != "" && len(state.FilteredResources) > 0 {
		countStr := fmt.Sprintf("(%d)", len(state.FilteredResources))
		if !strings.Contains(content, countStr) {
			t.Errorf("Should display filtered count %q in title", countStr)
		}
	}
}

// ===========================================================================
// QA-072: :ctx shows profile list
// ===========================================================================

func TestQA_072_CtxShowsProfileList(t *testing.T) {
	profiles := []string{"default", "dev", "staging", "prod"}
	selector := views.NewProfileSelect(profiles, "default")

	viewOutput := selector.View()

	if !strings.Contains(viewOutput, "Select AWS Profile") {
		t.Error(":ctx view should show 'Select AWS Profile' title")
	}

	for _, p := range profiles {
		if !strings.Contains(viewOutput, p) {
			t.Errorf(":ctx view should list profile %q", p)
		}
	}

	// Active profile should be marked with "*"
	if !strings.Contains(viewOutput, "* default") {
		t.Error("Active profile 'default' should be marked with '* '")
	}
}

// ===========================================================================
// QA-073: Selecting profile sends ProfileSwitchedMsg
// ===========================================================================

func TestQA_073_SelectProfileSendsProfileSwitchedMsg(t *testing.T) {
	state := app.NewAppState("default", "us-east-1")
	state.ProfileSelector = views.NewProfileSelect(
		[]string{"default", "dev", "staging"},
		"default",
	)
	state.CurrentView = app.ProfileSelectView
	state.Width = 80
	state.Height = 24

	// Navigate down to "dev" (index 1)
	updated, _ := state.Update(charKey("j"))
	state = updated.(app.AppState)

	if state.ProfileSelector.SelectedProfile() != "dev" {
		t.Errorf("After j, selected profile should be 'dev', got %q", state.ProfileSelector.SelectedProfile())
	}

	// Press Enter to select
	updated, cmd := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)

	// The view should transition back to MainMenuView
	if state.CurrentView != app.MainMenuView {
		t.Errorf("After selecting profile, view should return to MainMenuView, got %d", state.CurrentView)
	}

	// The cmd should produce a ProfileSwitchedMsg
	if cmd == nil {
		t.Fatal("Selecting a profile should return a non-nil command")
	}
	msg := cmd()
	if psm, ok := msg.(app.ProfileSwitchedMsg); ok {
		if psm.Profile != "dev" {
			t.Errorf("ProfileSwitchedMsg should have Profile='dev', got %q", psm.Profile)
		}
	} else {
		t.Errorf("Expected ProfileSwitchedMsg, got %T", msg)
	}
}

// ===========================================================================
// QA-074: Switch to profile with SSO (expired token) -- skip, needs real AWS
// ===========================================================================

func TestQA_074_SSOExpiredToken(t *testing.T) {
	// Test that when APIErrorMsg contains "ExpiredToken", the status message
	// suggests "aws sso login".
	state := app.NewAppState("", "us-east-1")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Loading = true

	updated, cmd := state.Update(app.APIErrorMsg{
		Err:          fmt.Errorf("operation error EC2: DescribeInstances, ExpiredToken: the security token included in the request is expired"),
		ResourceType: "ec2",
	})
	s := updated.(app.AppState)

	if !s.StatusIsError {
		t.Error("expected StatusIsError=true for expired token error")
	}
	if !strings.Contains(s.StatusMessage, "aws sso login") {
		t.Errorf("expected status message to suggest 'aws sso login', got %q", s.StatusMessage)
	}
	if !strings.Contains(s.StatusMessage, "expired") {
		t.Errorf("expected status message to contain 'expired', got %q", s.StatusMessage)
	}
	if s.Loading {
		t.Error("expected Loading=false after expired token error")
	}
	if cmd == nil {
		t.Error("expected a timer command for auto-clear")
	}
}

// ===========================================================================
// QA-075: :region shows region list
// ===========================================================================

func TestQA_075_RegionShowsRegionList(t *testing.T) {
	regions := awsclient.AllRegions()
	selector := views.NewRegionSelect(regions, "us-east-1")

	viewOutput := selector.View()

	if !strings.Contains(viewOutput, "Select AWS Region") {
		t.Error(":region view should show 'Select AWS Region' title")
	}

	// Check a sample of regions are listed
	sampleRegions := []string{"us-east-1", "eu-west-1", "ap-southeast-1"}
	for _, code := range sampleRegions {
		if !strings.Contains(viewOutput, code) {
			t.Errorf(":region view should list region %q", code)
		}
	}

	// Check display names are present
	if !strings.Contains(viewOutput, "US East (N. Virginia)") {
		t.Error(":region view should show display names like 'US East (N. Virginia)'")
	}

	// Active region should be marked with "*"
	if !strings.Contains(viewOutput, "* us-east-1") {
		t.Error("Active region 'us-east-1' should be marked with '* '")
	}

	// Verify all 27 regions are listed
	if len(regions) != 27 {
		t.Errorf("AllRegions should return 27 regions, got %d", len(regions))
	}
}

// ===========================================================================
// QA-076: Selecting region sends RegionSwitchedMsg
// ===========================================================================

func TestQA_076_SelectRegionSendsRegionSwitchedMsg(t *testing.T) {
	regions := awsclient.AllRegions()
	state := app.NewAppState("default", "us-east-1")
	state.RegionSelector = views.NewRegionSelect(regions, "us-east-1")
	state.CurrentView = app.RegionSelectView
	state.Width = 80
	state.Height = 40

	// Navigate down to a different region (eu-west-1 is at some index)
	// Find the index of eu-west-1
	targetIdx := -1
	for i, r := range regions {
		if r.Code == "eu-west-1" {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		t.Fatal("Could not find eu-west-1 in AllRegions")
	}

	// Navigate to the target region
	for i := 0; i < targetIdx; i++ {
		updated, _ := state.Update(charKey("j"))
		state = updated.(app.AppState)
	}

	selected := state.RegionSelector.SelectedRegion()
	if selected.Code != "eu-west-1" {
		t.Errorf("After navigating, selected region should be 'eu-west-1', got %q", selected.Code)
	}

	// Press Enter to select
	updated, cmd := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)

	if state.CurrentView != app.MainMenuView {
		t.Errorf("After selecting region, view should return to MainMenuView, got %d", state.CurrentView)
	}

	if cmd == nil {
		t.Fatal("Selecting a region should return a non-nil command")
	}
	msg := cmd()
	if rsm, ok := msg.(app.RegionSwitchedMsg); ok {
		if rsm.Region != "eu-west-1" {
			t.Errorf("RegionSwitchedMsg should have Region='eu-west-1', got %q", rsm.Region)
		}
	} else {
		t.Errorf("Expected RegionSwitchedMsg, got %T", msg)
	}
}

// ===========================================================================
// QA-077: Profile switch updates header
// ===========================================================================

func TestQA_077_ProfileSwitchUpdatesHeader(t *testing.T) {
	state := app.NewAppState("default", "us-east-1")
	state.Width = 120
	state.Height = 24

	// Simulate receiving a ProfileSwitchedMsg
	// Note: recreateClients will fail without real AWS, but we test header update
	updated, _ := state.Update(app.ProfileSwitchedMsg{Profile: "dev", Region: "eu-west-1"})
	state = updated.(app.AppState)

	if state.ActiveProfile != "dev" {
		t.Errorf("ActiveProfile should be 'dev' after switch, got %q", state.ActiveProfile)
	}
	if state.ActiveRegion != "eu-west-1" {
		t.Errorf("ActiveRegion should be 'eu-west-1' after profile switch, got %q", state.ActiveRegion)
	}

	view := state.View()
	content := view.Content
	if !strings.Contains(content, "profile: dev") {
		t.Error("Header should show 'profile: dev' after profile switch")
	}
	if !strings.Contains(content, "eu-west-1") {
		t.Error("Header should show 'eu-west-1' after profile switch")
	}
}

// ===========================================================================
// QA-078: Region switch updates header
// ===========================================================================

func TestQA_078_RegionSwitchUpdatesHeader(t *testing.T) {
	state := app.NewAppState("default", "us-east-1")
	state.Width = 120
	state.Height = 24

	// Simulate receiving a RegionSwitchedMsg
	updated, _ := state.Update(app.RegionSwitchedMsg{Region: "ap-southeast-1"})
	state = updated.(app.AppState)

	if state.ActiveRegion != "ap-southeast-1" {
		t.Errorf("ActiveRegion should be 'ap-southeast-1' after switch, got %q", state.ActiveRegion)
	}

	view := state.View()
	content := view.Content
	if !strings.Contains(content, "ap-southeast-1") {
		t.Error("Header should show 'ap-southeast-1' after region switch")
	}
}

// ===========================================================================
// QA-079: Cancel profile/region selection with Escape returns to previous view
// ===========================================================================

func TestQA_079_CancelProfileSelectWithEscape(t *testing.T) {
	state := app.NewAppState("default", "us-east-1")
	state.ProfileSelector = views.NewProfileSelect(
		[]string{"default", "dev"},
		"default",
	)
	state.CurrentView = app.ProfileSelectView
	state.Width = 80
	state.Height = 24

	// Navigate to "dev" but then Escape
	updated, _ := state.Update(charKey("j"))
	state = updated.(app.AppState)

	updated, _ = state.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	state = updated.(app.AppState)

	// Should return to previous view (MainMenuView as fallback)
	if state.CurrentView == app.ProfileSelectView {
		t.Error("Escape should leave ProfileSelectView")
	}
	// Profile should remain unchanged
	if state.ActiveProfile != "default" {
		t.Errorf("ActiveProfile should remain 'default' after cancel, got %q", state.ActiveProfile)
	}
}

func TestQA_079_CancelRegionSelectWithEscape(t *testing.T) {
	regions := awsclient.AllRegions()
	state := app.NewAppState("default", "us-east-1")
	state.RegionSelector = views.NewRegionSelect(regions, "us-east-1")
	state.CurrentView = app.RegionSelectView
	state.Width = 80
	state.Height = 40

	// Navigate down but then Escape
	updated, _ := state.Update(charKey("j"))
	state = updated.(app.AppState)

	updated, _ = state.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	state = updated.(app.AppState)

	if state.CurrentView == app.RegionSelectView {
		t.Error("Escape should leave RegionSelectView")
	}
	// Region should remain unchanged
	if state.ActiveRegion != "us-east-1" {
		t.Errorf("ActiveRegion should remain 'us-east-1' after cancel, got %q", state.ActiveRegion)
	}
}

// ===========================================================================
// QA-080: :ctx when no profiles are configured
// ===========================================================================

func TestQA_080_CtxWithNoProfiles(t *testing.T) {
	// Simulate the executeCommand("ctx") path when ListProfiles returns empty
	// We test via the profile selector: NewProfileSelect with empty list
	selector := views.NewProfileSelect([]string{}, "default")
	if selector.SelectedProfile() != "" {
		t.Errorf("Empty profile list should return empty SelectedProfile, got %q", selector.SelectedProfile())
	}

	// When the app runs :ctx with no profiles, it should show an error status.
	// We cannot easily call executeCommand directly (it reads real config),
	// but we can verify the behavior when no profiles are found by testing
	// that the app handles the empty profiles case.
	viewOutput := selector.View()
	if !strings.Contains(viewOutput, "Select AWS Profile") {
		t.Error("Empty profile selector should still render")
	}
}

// ===========================================================================
// QA-081: Navigate region list with g/G
// ===========================================================================

func TestQA_081_RegionListGGNavigation(t *testing.T) {
	regions := awsclient.AllRegions()
	state := app.NewAppState("default", "us-east-1")
	state.RegionSelector = views.NewRegionSelect(regions, "us-east-1")
	state.CurrentView = app.RegionSelectView
	state.Width = 80
	state.Height = 40

	// Press G to jump to bottom
	updated, _ := state.Update(charKey("G"))
	state = updated.(app.AppState)

	lastIdx := len(regions) - 1
	if state.RegionSelector.Cursor != lastIdx {
		// Note: QA-081 mentions that g/G may NOT be handled in region select view.
		// The handleRegionSelectKeys only handles Up, Down, Top, Bottom, Enter.
		// Top/Bottom are bound to "g"/"G", so they SHOULD work.
		t.Errorf("After G, cursor should be at last region (%d), got %d", lastIdx, state.RegionSelector.Cursor)
	}

	// Press g to jump to top
	updated, _ = state.Update(charKey("g"))
	state = updated.(app.AppState)

	if state.RegionSelector.Cursor != 0 {
		t.Errorf("After g, cursor should be at 0, got %d", state.RegionSelector.Cursor)
	}

	// Verify the first and last regions
	if state.RegionSelector.SelectedRegion().Code != "us-east-1" {
		t.Errorf("First region should be 'us-east-1', got %q", state.RegionSelector.SelectedRegion().Code)
	}
}
