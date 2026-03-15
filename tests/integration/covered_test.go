//go:build integration

package integration

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/app"
	awsclient "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/internal/views"
)

// Helper functions for key simulation.
func iKeyPress(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: s}
}

func iSpecialKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func iSendKey(state app.AppState, msg tea.KeyPressMsg) app.AppState {
	model, _ := state.Update(msg)
	return model.(app.AppState)
}

func iExecuteCommand(state app.AppState, cmd string) (app.AppState, tea.Cmd) {
	state.CommandMode = true
	state.CommandText = cmd
	model, teaCmd := state.Update(iSpecialKey(tea.KeyEnter))
	return model.(app.AppState), teaCmd
}

// QA-195: Case-insensitive commands -- full integration verification
func TestQA_195_CaseInsensitiveCommands(t *testing.T) {
	commands := []struct {
		input    string
		wantView app.ViewType
		wantType string
	}{
		{"EC2", app.ResourceListView, "ec2"},
		{"Ec2", app.ResourceListView, "ec2"},
		{"S3", app.ResourceListView, "s3"},
		{"RDS", app.ResourceListView, "rds"},
		{"REDIS", app.ResourceListView, "redis"},
		{"DocDB", app.ResourceListView, "docdb"},
		{"EKS", app.ResourceListView, "eks"},
		{"SECRETS", app.ResourceListView, "secrets"},
		{"MAIN", app.MainMenuView, ""},
		{"Quit", app.MainMenuView, ""}, // returns tea.Quit cmd
	}

	for _, tc := range commands {
		t.Run(tc.input, func(t *testing.T) {
			state := app.NewAppState("", "")
			// Navigate away from main for MAIN command test
			if strings.ToLower(tc.input) == "main" {
				state, _ = iExecuteCommand(state, "ec2")
			}

			updated, cmd := iExecuteCommand(state, tc.input)

			if strings.ToLower(tc.input) == "quit" || strings.ToLower(tc.input) == "q" {
				if cmd == nil {
					t.Error("expected tea.Quit command for :quit")
				}
				return
			}

			if updated.CurrentView != tc.wantView {
				t.Errorf(":%s -> expected view %d, got %d", tc.input, tc.wantView, updated.CurrentView)
			}
			if tc.wantType != "" && updated.CurrentResourceType != tc.wantType {
				t.Errorf(":%s -> expected type %q, got %q", tc.input, tc.wantType, updated.CurrentResourceType)
			}
		})
	}
}

// QA-196: g/G support in scrollable views (Detail, JSON, Reveal)
func TestQA_196_GoTopBottomInScrollableViews(t *testing.T) {
	// Test DetailView scrolling with g/G
	t.Run("DetailView", func(t *testing.T) {
		state := app.NewAppState("", "")
		state.Width = 80
		state.Height = 10 // Small height to enable scrolling

		// Create a detail model with many entries to ensure scrollable content
		data := make(map[string]string)
		for i := 0; i < 30; i++ {
			data[strings.Repeat("Key", 1)+string(rune('A'+i%26))] = strings.Repeat("Value", 5)
		}
		state.Detail = views.NewDetailModel("Test Detail", data)
		state.Detail.Width = state.Width
		state.Detail.Height = state.Height
		state.CurrentView = app.DetailView

		// Press G to go to bottom
		state = iSendKey(state, iKeyPress("G"))
		// Press g to go to top
		state = iSendKey(state, iKeyPress("g"))

		// Should not panic -- that's the main assertion
		view := state.View()
		if view.Content == "" {
			t.Error("expected non-empty view after g/G navigation in DetailView")
		}
	})

	// Test JSONView scrolling with g/G
	t.Run("JSONView", func(t *testing.T) {
		state := app.NewAppState("", "")
		state.Width = 80
		state.Height = 10
		state.JSONData = views.NewJSONView("Test JSON", `{"key1":"value1","key2":"value2","key3":"value3"}`)
		state.JSONData.Width = state.Width
		state.JSONData.Height = state.Height
		state.CurrentView = app.JSONView

		state = iSendKey(state, iKeyPress("G"))
		state = iSendKey(state, iKeyPress("g"))

		view := state.View()
		if view.Content == "" {
			t.Error("expected non-empty view after g/G navigation in JSONView")
		}
	})

	// Test RevealView scrolling with g/G
	t.Run("RevealView", func(t *testing.T) {
		state := app.NewAppState("", "")
		state.Width = 80
		state.Height = 10
		state.Reveal = views.NewRevealView("Test Secret", strings.Repeat("long secret content\n", 20))
		state.Reveal.Width = state.Width
		state.Reveal.Height = state.Height
		state.CurrentView = app.RevealView

		state = iSendKey(state, iKeyPress("G"))
		state = iSendKey(state, iKeyPress("g"))

		view := state.View()
		if view.Content == "" {
			t.Error("expected non-empty view after g/G navigation in RevealView")
		}
	})
}

// QA-197: g/G support in ProfileSelectView and RegionSelectView
func TestQA_197_GoTopBottomInSelectorViews(t *testing.T) {
	t.Run("ProfileSelectView", func(t *testing.T) {
		profiles := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
		state := app.NewAppState("alpha", "us-east-1")
		state.ProfileSelector = views.NewProfileSelect(profiles, "alpha")
		state.CurrentView = app.ProfileSelectView
		state.Width = 80
		state.Height = 24

		// G to go to bottom
		state = iSendKey(state, iKeyPress("G"))
		if state.ProfileSelector.Cursor != len(profiles)-1 {
			t.Errorf("after G, cursor should be at %d, got %d", len(profiles)-1, state.ProfileSelector.Cursor)
		}

		// g to go to top
		state = iSendKey(state, iKeyPress("g"))
		if state.ProfileSelector.Cursor != 0 {
			t.Errorf("after g, cursor should be at 0, got %d", state.ProfileSelector.Cursor)
		}
	})

	t.Run("RegionSelectView", func(t *testing.T) {
		regions := awsclient.AllRegions()
		state := app.NewAppState("default", "us-east-1")
		state.RegionSelector = views.NewRegionSelect(regions, "us-east-1")
		state.CurrentView = app.RegionSelectView
		state.Width = 80
		state.Height = 40

		// G to go to bottom
		state = iSendKey(state, iKeyPress("G"))
		if state.RegionSelector.Cursor != len(regions)-1 {
			t.Errorf("after G, cursor should be at %d, got %d", len(regions)-1, state.RegionSelector.Cursor)
		}

		// g to go to top
		state = iSendKey(state, iKeyPress("g"))
		if state.RegionSelector.Cursor != 0 {
			t.Errorf("after g, cursor should be at 0, got %d", state.RegionSelector.Cursor)
		}
	})
}

// QA-198: Auto-suggestion rendering
func TestQA_198_AutoSuggestionRendering(t *testing.T) {
	tests := []struct {
		prefix      string
		suggestions []string // at least one should appear
	}{
		{"e", []string{"ec2", "eks"}},
		{"s", []string{"s3", "secrets"}},
		{"r", []string{"rds", "redis", "region", "root"}},
		{"re", []string{"redis", "region"}},
		{"q", []string{"quit"}},
		{"m", []string{"main"}},
		{"c", []string{"ctx"}},
	}

	for _, tc := range tests {
		t.Run("prefix_"+tc.prefix, func(t *testing.T) {
			state := app.NewAppState("", "")
			state.CommandMode = true
			state.CommandText = tc.prefix
			state.Width = 80
			state.Height = 24

			view := state.View()
			content := view.Content

			if !strings.Contains(content, ":"+tc.prefix) {
				t.Errorf("expected view to contain ':%s'", tc.prefix)
			}

			found := false
			for _, suggestion := range tc.suggestions {
				if strings.Contains(content, suggestion) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected one of %v to appear as suggestion for prefix %q", tc.suggestions, tc.prefix)
			}
		})
	}
}

// QA-199: Error auto-clear returns timer cmd
func TestQA_199_ErrorAutoClearTimerCmd(t *testing.T) {
	state := app.NewAppState("", "")

	// Simulate an API error message
	updated, cmd := state.Update(app.APIErrorMsg{
		ResourceType: "ec2",
		Err:          &mockError{msg: "access denied"},
	})
	s := updated.(app.AppState)

	if !s.StatusIsError {
		t.Error("expected StatusIsError=true after APIErrorMsg")
	}
	if s.StatusMessage == "" {
		t.Error("expected non-empty StatusMessage after APIErrorMsg")
	}
	if cmd == nil {
		t.Error("expected non-nil cmd (timer) after APIErrorMsg for auto-clear")
	}

	// Execute the cmd to get the ClearErrorMsg
	if cmd != nil {
		// The cmd is a tea.Tick that will eventually produce ClearErrorMsg.
		// We can verify it's non-nil, which means the timer was created.
		t.Log("auto-clear timer command returned successfully")
	}
}

type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}
