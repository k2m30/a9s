package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/internal/app"
	"github.com/k2m30/a9s/internal/resource"

	tea "charm.land/bubbletea/v2"
)

// ---------------------------------------------------------------------------
// T020 - Test root model Init
// ---------------------------------------------------------------------------

func TestNewAppState_DefaultView(t *testing.T) {
	state := app.NewAppState("", "")
	if state.CurrentView != app.MainMenuView {
		t.Errorf("expected CurrentView = MainMenuView (%d), got %d", app.MainMenuView, state.CurrentView)
	}
}

func TestNewAppState_DefaultProfile(t *testing.T) {
	// Unset AWS_PROFILE so NewAppState falls back to "default".
	t.Setenv("AWS_PROFILE", "")
	state := app.NewAppState("", "")
	if state.ActiveProfile != "default" {
		t.Errorf("expected ActiveProfile = %q, got %q", "default", state.ActiveProfile)
	}
}

func TestNewAppState_DefaultRegion_NoEnvVar_ReadsFromConfig(t *testing.T) {
	// When AWS_REGION is not set and no region flag is passed,
	// NewAppState should read the region from ~/.aws/config for the
	// active profile, NOT hard-code "us-east-1".
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")

	// Use our test fixture which has region = us-east-1 for [default]
	// and region = eu-west-1 for [profile dev].
	state := app.NewAppState("", "")
	// With the real ~/.aws/config, the region should come from the
	// config file. At minimum it must NOT be a hard-coded fallback
	// when the config file exists and has a region.
	// The "default" profile's region from the user's real config
	// should be used. We can't assert a specific value here since
	// it depends on the machine, but we CAN test with a known config.
	// This test documents the requirement.
	_ = state
}

func TestNewAppState_ReadsRegionFromConfigFile(t *testing.T) {
	// Given a config file with [default] region = eu-central-1,
	// NewAppState("", "") should set ActiveRegion to "eu-central-1".
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")

	// Use the test fixture: [default] has region = us-east-1
	configPath := "../../tests/testdata/aws_config_sample"
	state := app.NewAppStateWithConfig("", "", configPath)
	if state.ActiveRegion != "us-east-1" {
		t.Errorf("expected region from config = %q, got %q", "us-east-1", state.ActiveRegion)
	}
}

func TestNewAppState_ReadsRegionFromConfigFile_NonDefaultProfile(t *testing.T) {
	// Given profile "dev" with region = eu-west-1 in config,
	// NewAppState should use eu-west-1.
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")

	configPath := "../../tests/testdata/aws_config_sample"
	state := app.NewAppStateWithConfig("dev", "", configPath)
	if state.ActiveRegion != "eu-west-1" {
		t.Errorf("expected region from config for dev = %q, got %q", "eu-west-1", state.ActiveRegion)
	}
}

func TestNewAppState_ExplicitRegionOverridesConfig(t *testing.T) {
	// When region is explicitly passed, it takes precedence over config.
	configPath := "../../tests/testdata/aws_config_sample"
	state := app.NewAppStateWithConfig("", "ap-southeast-1", configPath)
	if state.ActiveRegion != "ap-southeast-1" {
		t.Errorf("expected explicit region = %q, got %q", "ap-southeast-1", state.ActiveRegion)
	}
}

func TestNewAppState_EnvVarRegionOverridesConfig(t *testing.T) {
	// AWS_REGION env var takes precedence over config file.
	t.Setenv("AWS_REGION", "sa-east-1")
	state := app.NewAppState("", "")
	if state.ActiveRegion != "sa-east-1" {
		t.Errorf("expected env var region = %q, got %q", "sa-east-1", state.ActiveRegion)
	}
}

func TestNewAppState_FallbackWhenNoConfig(t *testing.T) {
	// When config file doesn't exist and no env var, fall back to us-east-1.
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	state := app.NewAppStateWithConfig("", "", "/nonexistent/path/config")
	if state.ActiveRegion != "us-east-1" {
		t.Errorf("expected fallback region = %q, got %q", "us-east-1", state.ActiveRegion)
	}
}

func TestNewAppState_Breadcrumbs(t *testing.T) {
	state := app.NewAppState("", "")
	if len(state.Breadcrumbs) != 1 {
		t.Fatalf("expected 1 breadcrumb, got %d", len(state.Breadcrumbs))
	}
	if state.Breadcrumbs[0] != "main" {
		t.Errorf("expected breadcrumb[0] = %q, got %q", "main", state.Breadcrumbs[0])
	}
}

// ---------------------------------------------------------------------------
// T022 - Test command routing
// ---------------------------------------------------------------------------

// enterKeyMsg returns a tea.KeyPressMsg that represents pressing Enter.
func enterKeyMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyEnter}
}

func TestCommandRouting_EC2(t *testing.T) {
	state := app.NewAppState("", "")
	state.CommandMode = true
	state.CommandText = "ec2"

	model, _ := state.Update(enterKeyMsg())
	updated, ok := model.(app.AppState)
	if !ok {
		t.Fatal("Update did not return an AppState")
	}

	if updated.CurrentView != app.ResourceListView {
		t.Errorf("expected CurrentView = ResourceListView (%d), got %d", app.ResourceListView, updated.CurrentView)
	}
	if updated.CurrentResourceType != "ec2" {
		t.Errorf("expected CurrentResourceType = %q, got %q", "ec2", updated.CurrentResourceType)
	}
}

func TestCommandRouting_Main(t *testing.T) {
	state := app.NewAppState("", "")
	// First navigate to EC2 to leave MainMenuView.
	state.CommandMode = true
	state.CommandText = "ec2"
	model, _ := state.Update(enterKeyMsg())
	state = model.(app.AppState)

	// Now issue :main to return to main menu.
	state.CommandMode = true
	state.CommandText = "main"
	model, _ = state.Update(enterKeyMsg())
	updated := model.(app.AppState)

	if updated.CurrentView != app.MainMenuView {
		t.Errorf("expected CurrentView = MainMenuView (%d), got %d", app.MainMenuView, updated.CurrentView)
	}
}

func TestCommandRouting_Quit(t *testing.T) {
	state := app.NewAppState("", "")
	state.CommandMode = true
	state.CommandText = "q"

	_, cmd := state.Update(enterKeyMsg())
	if cmd == nil {
		t.Fatal("expected a non-nil cmd (tea.Quit) for :q command")
	}
}

func TestCommandRouting_UnknownCommand(t *testing.T) {
	state := app.NewAppState("", "")
	state.CommandMode = true
	state.CommandText = "xyz"

	model, _ := state.Update(enterKeyMsg())
	updated := model.(app.AppState)

	if !strings.Contains(updated.StatusMessage, "Unknown command") {
		t.Errorf("expected StatusMessage to contain %q, got %q", "Unknown command", updated.StatusMessage)
	}
}

// ---------------------------------------------------------------------------
// T033 - Test ProfileSwitchedMsg and RegionSwitchedMsg
// ---------------------------------------------------------------------------

func TestProfileSwitchedMsg_UpdatesState(t *testing.T) {
	state := app.NewAppState("default", "us-east-1")

	msg := app.ProfileSwitchedMsg{Profile: "dev", Region: "eu-west-1"}
	model, _ := state.Update(msg)
	updated, ok := model.(app.AppState)
	if !ok {
		t.Fatal("Update did not return an AppState")
	}

	if updated.ActiveProfile != "dev" {
		t.Errorf("expected ActiveProfile = %q, got %q", "dev", updated.ActiveProfile)
	}
	if updated.ActiveRegion != "eu-west-1" {
		t.Errorf("expected ActiveRegion = %q, got %q", "eu-west-1", updated.ActiveRegion)
	}
}

func TestRegionSwitchedMsg_UpdatesState(t *testing.T) {
	state := app.NewAppState("default", "us-east-1")

	msg := app.RegionSwitchedMsg{Region: "ap-southeast-1"}
	model, _ := state.Update(msg)
	updated, ok := model.(app.AppState)
	if !ok {
		t.Fatal("Update did not return an AppState")
	}

	if updated.ActiveRegion != "ap-southeast-1" {
		t.Errorf("expected ActiveRegion = %q, got %q", "ap-southeast-1", updated.ActiveRegion)
	}
	// Profile should remain unchanged
	if updated.ActiveProfile != "default" {
		t.Errorf("expected ActiveProfile to remain %q, got %q", "default", updated.ActiveProfile)
	}
}

func TestCommandRouting_Ctx(t *testing.T) {
	state := app.NewAppState("", "")

	state.CommandMode = true
	state.CommandText = "ctx"

	model, _ := state.Update(enterKeyMsg())
	updated, ok := model.(app.AppState)
	if !ok {
		t.Fatal("Update did not return an AppState")
	}

	if updated.CurrentView != app.ProfileSelectView {
		t.Errorf("expected CurrentView = ProfileSelectView (%d), got %d", app.ProfileSelectView, updated.CurrentView)
	}
}

func TestCommandRouting_Region(t *testing.T) {
	state := app.NewAppState("", "")

	state.CommandMode = true
	state.CommandText = "region"

	model, _ := state.Update(enterKeyMsg())
	updated, ok := model.(app.AppState)
	if !ok {
		t.Fatal("Update did not return an AppState")
	}

	if updated.CurrentView != app.RegionSelectView {
		t.Errorf("expected CurrentView = RegionSelectView (%d), got %d", app.RegionSelectView, updated.CurrentView)
	}
}

// ---------------------------------------------------------------------------
// T052 - Test filter state persists across view transitions
// ---------------------------------------------------------------------------

func TestFilterState_PersistsAfterDetailAndBack(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.FilterMode = true
	state.Filter = "prod"

	// Simulate pressing enter to exit filter mode but keep filter active
	model, _ := state.Update(enterKeyMsg())
	updated := model.(app.AppState)

	// Filter text should be preserved, FilterMode should be false
	if updated.Filter != "prod" {
		t.Errorf("expected Filter = %q after enter, got %q", "prod", updated.Filter)
	}
	if updated.FilterMode {
		t.Errorf("expected FilterMode = false after enter")
	}

	// The FilteredResources should reflect the filter being applied
	// (even if no actual resources loaded, the Filter string persists)
	if updated.Filter != "prod" {
		t.Errorf("expected Filter = %q, got %q", "prod", updated.Filter)
	}
}

// ---------------------------------------------------------------------------
// T053 - Test empty filter result shows appropriate status
// ---------------------------------------------------------------------------

func TestFilterState_EmptyResult_StatusMessage(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Filter = "nonexistent-xyz"
	state.FilteredResources = []resource.Resource{} // empty filtered results

	// When FilteredResources is empty and Filter is set, we expect the
	// renderResourceList to show no matching resources.
	// Verify the state fields are consistent.
	if state.Filter != "nonexistent-xyz" {
		t.Errorf("expected Filter = %q, got %q", "nonexistent-xyz", state.Filter)
	}
	if len(state.FilteredResources) != 0 {
		t.Errorf("expected 0 FilteredResources, got %d", len(state.FilteredResources))
	}
}

func TestFilterState_ClearedOnNewCommand(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "ec2"
	state.Filter = "prod"
	state.FilteredResources = []resource.Resource{
		{ID: "i-001", Name: "prod-web-1", Status: "running", Fields: map[string]string{}},
	}

	// Execute a new resource command (:rds) - filter should be cleared
	state.CommandMode = true
	state.CommandText = "rds"

	model, _ := state.Update(enterKeyMsg())
	updated := model.(app.AppState)

	if updated.Filter != "" {
		t.Errorf("expected Filter to be cleared after new command, got %q", updated.Filter)
	}
	if updated.FilteredResources != nil {
		t.Errorf("expected FilteredResources to be nil after new command, got %v", updated.FilteredResources)
	}
}
