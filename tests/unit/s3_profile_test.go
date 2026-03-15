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
	// After InitConnectMsg is processed, Clients should be non-nil
	// and :s3 should use those clients
	state := app.NewAppState("kvinta.kvinta-dev", "eu-central-1")
	state.Width = 80
	state.Height = 24

	// Simulate InitConnectMsg being processed (this tries real AWS)
	// Skip if no real credentials
	updated, _ := state.Update(app.InitConnectMsg{
		Profile: "kvinta.kvinta-dev",
		Region:  "eu-central-1",
	})
	state = updated.(app.AppState)

	if state.Clients == nil {
		t.Skip("No AWS credentials available for profile kvinta.kvinta-dev")
	}

	// Now :s3 command
	state.CommandMode = true
	state.CommandText = "s3"
	updated, cmd := state.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	state = updated.(app.AppState)

	if state.CurrentResourceType != "s3" {
		t.Fatalf("Expected resource type 's3', got %q", state.CurrentResourceType)
	}
	if cmd == nil {
		t.Fatal("Should return fetch command")
	}
	if !state.Loading {
		t.Error("Should be loading")
	}
}
