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

// ---------------------------------------------------------------------------
// QA-195: Built-in commands must be case-insensitive
// Spec (contracts/commands.md): "Commands are case-insensitive."
// Bug: :main, :ctx, :region, :q, :quit, :root only match lowercase.
// ---------------------------------------------------------------------------

func TestCommandCaseInsensitive_Main(t *testing.T) {
	for _, cmd := range []string{"MAIN", "Main", "mAiN"} {
		state := app.NewAppState("", "")
		state.CurrentView = app.ResourceListView
		state.CommandMode = true
		state.CommandText = cmd

		updated, _ := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		s := updated.(app.AppState)
		if s.CurrentView != app.MainMenuView {
			t.Errorf(":%s should navigate to MainMenu, got view %d", cmd, s.CurrentView)
		}
	}
}

func TestCommandCaseInsensitive_Quit(t *testing.T) {
	for _, cmd := range []string{"Q", "QUIT", "Quit"} {
		state := app.NewAppState("", "")
		state.CommandMode = true
		state.CommandText = cmd

		_, teaCmd := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		if teaCmd == nil {
			t.Errorf(":%s should return tea.Quit cmd, got nil", cmd)
		}
	}
}

func TestCommandCaseInsensitive_Ctx(t *testing.T) {
	for _, cmd := range []string{"CTX", "Ctx"} {
		state := app.NewAppState("", "")
		state.CommandMode = true
		state.CommandText = cmd

		updated, _ := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		s := updated.(app.AppState)
		if s.CurrentView != app.ProfileSelectView {
			t.Errorf(":%s should navigate to ProfileSelectView, got view %d (status: %s)",
				cmd, s.CurrentView, s.StatusMessage)
		}
	}
}

func TestCommandCaseInsensitive_Region(t *testing.T) {
	for _, cmd := range []string{"REGION", "Region"} {
		state := app.NewAppState("", "")
		state.CommandMode = true
		state.CommandText = cmd

		updated, _ := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		s := updated.(app.AppState)
		if s.CurrentView != app.RegionSelectView {
			t.Errorf(":%s should navigate to RegionSelectView, got view %d (status: %s)",
				cmd, s.CurrentView, s.StatusMessage)
		}
	}
}

func TestCommandCaseInsensitive_Root(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CommandMode = true
	state.CommandText = "ROOT"

	updated, _ := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	s := updated.(app.AppState)
	if s.CurrentView != app.MainMenuView {
		t.Errorf(":ROOT should navigate to MainMenu, got view %d", s.CurrentView)
	}
}

// ---------------------------------------------------------------------------
// QA-196/197: g/G (top/bottom) must work in detail, JSON, reveal,
// profile select, and region select views.
// Spec (FR-006): "g/G for top/bottom" — applies to all list/scrollable views.
// Bug: Only handled in main menu and resource list views.
// ---------------------------------------------------------------------------

func TestDetailView_GoTopBottom(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.DetailView
	state.Detail = views.NewDetailModel("test", map[string]string{
		"A": "1", "B": "2", "C": "3", "D": "4", "E": "5",
		"F": "6", "H": "8", "I": "9", "J": "10", "K": "11",
	})
	state.Detail.Height = 3

	// Scroll down first
	for i := 0; i < 5; i++ {
		updated, _ := state.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		state = updated.(app.AppState)
	}

	// g → go to top
	updated, _ := state.Update(tea.KeyPressMsg{Code: -1, Text: "g"})
	state = updated.(app.AppState)
	if state.Detail.Offset != 0 {
		t.Errorf("g in detail view should set offset to 0, got %d", state.Detail.Offset)
	}

	// G → go to bottom
	updated, _ = state.Update(tea.KeyPressMsg{Code: -1, Text: "G"})
	state = updated.(app.AppState)
	if state.Detail.Offset == 0 {
		t.Error("G in detail view should scroll to bottom, offset is still 0")
	}
}

func TestProfileSelectView_GoTopBottom(t *testing.T) {
	profiles := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	state := app.NewAppState("", "")
	state.CurrentView = app.ProfileSelectView
	state.ProfileSelector = views.NewProfileSelect(profiles, "alpha")

	// G → last item
	updated, _ := state.Update(tea.KeyPressMsg{Code: -1, Text: "G"})
	state = updated.(app.AppState)
	if state.ProfileSelector.SelectedProfile() != "epsilon" {
		t.Errorf("G should select last profile 'epsilon', got %q",
			state.ProfileSelector.SelectedProfile())
	}

	// g → first item
	updated, _ = state.Update(tea.KeyPressMsg{Code: -1, Text: "g"})
	state = updated.(app.AppState)
	if state.ProfileSelector.SelectedProfile() != "alpha" {
		t.Errorf("g should select first profile 'alpha', got %q",
			state.ProfileSelector.SelectedProfile())
	}
}

func TestRegionSelectView_GoTopBottom(t *testing.T) {
	regions := awsclient.AllRegions()
	state := app.NewAppState("", "")
	state.CurrentView = app.RegionSelectView
	state.RegionSelector = views.NewRegionSelect(regions, "us-east-1")

	// G → last item
	updated, _ := state.Update(tea.KeyPressMsg{Code: -1, Text: "G"})
	state = updated.(app.AppState)
	last := regions[len(regions)-1]
	if state.RegionSelector.SelectedRegion().Code != last.Code {
		t.Errorf("G should select last region %q, got %q",
			last.Code, state.RegionSelector.SelectedRegion().Code)
	}

	// g → first item
	updated, _ = state.Update(tea.KeyPressMsg{Code: -1, Text: "g"})
	state = updated.(app.AppState)
	if state.RegionSelector.SelectedRegion().Code != "us-east-1" {
		t.Errorf("g should select first region 'us-east-1', got %q",
			state.RegionSelector.SelectedRegion().Code)
	}
}

// ---------------------------------------------------------------------------
// QA-199: Error messages must auto-clear after 5 seconds.
// Spec (contracts/ui-layout.md): "Error state: error message
// (auto-clears after 5 seconds)"
// Bug: No timer command returned; errors persist indefinitely.
// ---------------------------------------------------------------------------

func TestErrorAutoClear_ReturnsTimerCmd(t *testing.T) {
	state := app.NewAppState("", "")

	updated, cmd := state.Update(app.APIErrorMsg{
		ResourceType: "ec2",
		Err:          fmt.Errorf("access denied"),
	})
	s := updated.(app.AppState)

	if !s.StatusIsError {
		t.Fatal("StatusIsError should be true after APIErrorMsg")
	}
	if s.StatusMessage == "" {
		t.Fatal("StatusMessage should not be empty after APIErrorMsg")
	}
	if cmd == nil {
		t.Error("APIErrorMsg should return a delayed cmd to auto-clear the error after 5 seconds")
	}
}

// ---------------------------------------------------------------------------
// QA-183: Stale API response must not overwrite current view data.
// If user switches from :ec2 to :rds while EC2 is still loading,
// the late EC2 response should be discarded.
// ---------------------------------------------------------------------------

func TestStaleResponse_Discarded(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.ResourceListView
	state.CurrentResourceType = "rds"
	state.Loading = true

	staleMsg := app.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-stale", Name: "stale-instance"},
		},
	}

	updated, _ := state.Update(staleMsg)
	s := updated.(app.AppState)

	if s.CurrentResourceType != "rds" {
		t.Errorf("CurrentResourceType should still be 'rds', got %q", s.CurrentResourceType)
	}
	if len(s.Resources) > 0 {
		t.Errorf("Stale EC2 resources should be discarded, got %d resources", len(s.Resources))
	}
}

// ---------------------------------------------------------------------------
// QA-198: Command auto-suggestions must render in the status bar.
// Spec (FR-004): "auto-suggestions for known commands"
// Bug: CommandInput.View() has suggestions but renderStatusBar
// builds command mode display manually without using it.
// ---------------------------------------------------------------------------

func TestCommandMode_ShowsAutoSuggestion(t *testing.T) {
	state := app.NewAppState("", "")
	state.CurrentView = app.MainMenuView
	state.CommandMode = true
	state.CommandText = "ec"
	state.Width = 80
	state.Height = 24

	view := state.View()

	// The rendered output should contain the suggestion "ec2" somewhere
	if !strings.Contains(view.Content, "ec2") {
		t.Error("Command mode with 'ec' typed should show 'ec2' auto-suggestion in the rendered output")
	}
}
