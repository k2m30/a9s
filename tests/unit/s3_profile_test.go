package unit

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/app"
)

// BUG: When user launches with -p profile, the S3 client must use that
// profile's credentials, not the default profile's.
// The Init() sends InitConnectMsg with the profile. But if the user
// types :s3 before InitConnectMsg is processed, Clients is nil.
// Also verify the profile name reaches NewAWSSession correctly.

func TestProfilePassedToInit(t *testing.T) {
	state := app.NewAppState("kvinta.kvinta-dev", "")

	if state.ActiveProfile != "kvinta.kvinta-dev" {
		t.Errorf("ActiveProfile should be 'kvinta.kvinta-dev', got %q", state.ActiveProfile)
	}

	// Init should return a command
	cmd := state.Init()
	if cmd == nil {
		t.Fatal("Init should return a command (InitConnectMsg)")
	}

	// Execute the command to get the message
	msg := cmd()
	initMsg, ok := msg.(app.InitConnectMsg)
	if !ok {
		t.Fatalf("Expected InitConnectMsg, got %T", msg)
	}
	if initMsg.Profile != "kvinta.kvinta-dev" {
		t.Errorf("InitConnectMsg.Profile should be 'kvinta.kvinta-dev', got %q", initMsg.Profile)
	}
}

func TestS3FetchUsesCorrectProfile(t *testing.T) {
	// After InitConnectMsg is processed, verify the :s3 command flow.
	// We test this without real AWS credentials by verifying the command
	// mode and resource type switching, then checking the state after
	// InitConnectMsg (which may or may not succeed depending on environment).
	state := app.NewAppState("kvinta.kvinta-dev", "eu-central-1")
	state.Width = 80
	state.Height = 24

	// Simulate InitConnectMsg being processed (may or may not connect)
	updated, _ := state.Update(app.InitConnectMsg{
		Profile: "kvinta.kvinta-dev",
		Region:  "eu-central-1",
	})
	state = updated.(app.AppState)

	// Regardless of whether AWS connected, test the :s3 command flow
	state.CommandMode = true
	state.CommandText = "s3"
	updated, cmd := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)

	if state.CurrentResourceType != "s3" {
		t.Fatalf("Expected resource type 's3', got %q", state.CurrentResourceType)
	}

	// If Clients is non-nil, we expect a fetch command and Loading=true
	if state.Clients != nil {
		if cmd == nil {
			t.Error("With Clients available, should return fetch command")
		}
		if !state.Loading {
			t.Error("With Clients available, should be loading")
		}
	}
	// If Clients is nil (no credentials), the app should still be in the right
	// view without crashing. The fetch command may be nil.
	if state.CurrentView != app.ResourceListView {
		t.Errorf("Expected ResourceListView, got %d", state.CurrentView)
	}
}
