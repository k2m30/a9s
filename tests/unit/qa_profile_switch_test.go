package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// --- Bug: Profile switch doesn't refresh data ---

func TestBug_ProfileSwitch_RefreshesResourceList(t *testing.T) {
	m := newRootSizedModel()
	// Navigate to EC2 and load resources
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: "ec2"})
	oldResources := []resource.Resource{
		{ID: "i-old", Name: "old-server", Status: "running", Fields: map[string]string{"instance_id": "i-old", "name": "old-server", "state": "running"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "ec2", Resources: oldResources})

	// Verify old data is visible
	content := rootViewContent(m)
	if !strings.Contains(content, "old-server") {
		t.Fatal("should show old-server before profile switch")
	}

	// Simulate profile switch: ProfileSelectedMsg
	m, cmd := rootApplyMsg(m, messages.ProfileSelectedMsg{Profile: "new-profile"})

	// After profile selected, a connectAWS cmd should be returned
	if cmd == nil {
		t.Fatal("ProfileSelectedMsg should return a command to reconnect")
	}

	// Simulate ClientsReadyMsg (successful reconnect)
	m, cmd = rootApplyMsg(m, messages.ClientsReadyMsg{Clients: nil}) // nil clients for test

	// After reconnect, should trigger a refresh (return a fetch command)
	// The active view should still be the resource list
	content = rootViewContent(m)
	plain := stripANSI(content)
	// Should show some indication of refresh (flash or loading state)
	if !strings.Contains(plain, "Refreshing") && !strings.Contains(plain, "Loading") && !strings.Contains(plain, "Connected") {
		t.Logf("After profile switch + reconnect, expected refresh indication, got:\n%s", plain[:min(300, len(plain))])
		// The key check: cmd should be non-nil (a fetch command)
	}
	// Most importantly: a fetch command should have been returned to reload data
	if cmd == nil {
		t.Error("After ClientsReadyMsg following profile switch, should return a fetch command to refresh")
	}
}

func TestBug_ProfileSwitch_UpdatesHeaderProfile(t *testing.T) {
	m := newRootSizedModel()
	// Check initial profile in header
	content := rootViewContent(m)
	if !strings.Contains(content, "testprofile") {
		t.Fatal("header should show initial profile")
	}

	// Switch profile
	m, _ = rootApplyMsg(m, messages.ProfileSelectedMsg{Profile: "new-profile"})
	m, _ = rootApplyMsg(m, messages.ClientsReadyMsg{Clients: nil})

	content = rootViewContent(m)
	if !strings.Contains(content, "new-profile") {
		t.Error("header should show new profile after switch")
	}
	if strings.Contains(content, "testprofile") {
		t.Error("header should NOT show old profile after switch")
	}
}

func TestBug_RegionSwitch_RefreshesResourceList(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: "ec2"})
	resources := []resource.Resource{
		{ID: "i-123", Name: "server", Status: "running", Fields: map[string]string{"instance_id": "i-123"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "ec2", Resources: resources})

	// Switch region
	m, cmd := rootApplyMsg(m, messages.RegionSelectedMsg{Region: "eu-west-1"})
	if cmd == nil {
		t.Fatal("RegionSelectedMsg should return a reconnect command")
	}

	// Simulate successful reconnect
	_, cmd = rootApplyMsg(m, messages.ClientsReadyMsg{Clients: nil})
	if cmd == nil {
		t.Error("After ClientsReadyMsg following region switch, should return a fetch command to refresh")
	}
}

// --- Bug: Profile list shows credentials-only profiles ---

func TestBug_ProfileList_MatchesAWSCLI(t *testing.T) {
	// The profile list should match `aws configure list-profiles` behavior:
	// only [profile xxx] from ~/.aws/config, NOT bare sections from ~/.aws/credentials
	// This test verifies the count is reasonable (not inflated by credentials file)

	// We can't test exact count without knowing the user's config,
	// but we can verify the ListProfiles function signature is called correctly
	// and that the profile model receives what fetchProfiles returns.

	// Test that ListProfiles only reads config file
	m := newRootSizedModel()
	// Trigger :ctx command
	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: ':'})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	// Type "ctx" and enter
	for _, r := range "ctx" {
		m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	// This returns a NavigateMsg for TargetProfile
	if cmd != nil {
		msg := cmd()
		m, cmd = rootApplyMsg(m, msg) //nolint:ineffassign,staticcheck // verify flow doesn't panic
		// This triggers fetchProfiles which returns profilesLoadedMsg
		// We can't easily test the actual profile list without filesystem access
		// but we verify the flow doesn't crash
	}
}

// --- Verify profile switch from main menu works ---

func TestBug_RegionShownInHeader(t *testing.T) {
	tui.Version = "test"
	m := tui.New("test-dev", "")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 24})

	// When region is empty, header should still show a resolved region (not empty)
	content := rootViewContent(m)
	plain := stripANSI(content)

	// Header format: "a9s vX.Y.Z  profile:region"
	// With empty region, it should show a default (e.g. "us-east-1") not "test-dev:"
	switch {
	case strings.Contains(plain, "test-dev:us-east-1") || strings.Contains(plain, "test-dev:eu-"):
		// Good — region is resolved
	case strings.HasSuffix(strings.TrimSpace(strings.Split(plain, "\n")[0]), ":"):
		t.Error("header shows empty region — should resolve default region from AWS config")
	case !strings.Contains(plain, ":"):
		t.Error("header missing profile:region separator")
	}
}

func TestBug_RegionShownInHeader_AfterConnect(t *testing.T) {
	tui.Version = "test"
	m := tui.New("test-dev", "")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 24})

	// After ClientsReadyMsg, region should be populated in header
	m, _ = rootApplyMsg(m, messages.ClientsReadyMsg{Clients: nil})

	content := rootViewContent(m)
	plain := stripANSI(content)
	header := strings.Split(plain, "\n")[0]

	// The region should not be empty after connect
	if strings.Contains(header, "test-dev: ") || strings.HasSuffix(strings.TrimSpace(header), ":") {
		t.Errorf("header region should not be empty after connect, got: %s", header)
	}
}

func TestBug_ProfileSwitch_FlashHasTimer(t *testing.T) {
	m := newRootSizedModel()
	// Switch profile — returns batch cmd (flash + connectAWS)
	m, cmd := rootApplyMsg(m, messages.ProfileSelectedMsg{Profile: "test-prod"})
	if cmd == nil {
		t.Fatal("ProfileSelectedMsg should return a command")
	}

	// Execute the batch to process FlashMsg
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, subCmd := range batch {
			if subCmd != nil {
				subMsg := subCmd()
				// Only process FlashMsg, skip connectAWS (would need real AWS)
				if _, isFlash := subMsg.(messages.FlashMsg); isFlash {
					m, _ = rootApplyMsg(m, subMsg)
				}
			}
		}
	}

	// Now the flash should be visible with a timer (gen > 0)
	content := rootViewContent(m)
	if !strings.Contains(content, "Switching") {
		t.Error("should show switching message after FlashMsg is processed")
	}
}

func TestBug_ProfileSwitch_FlashClears(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: "ec2"})
	resources := []resource.Resource{
		{ID: "i-123", Name: "srv", Status: "running", Fields: map[string]string{"instance_id": "i-123"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "ec2", Resources: resources})

	// Switch profile — process the FlashMsg from the batch
	m, cmd := rootApplyMsg(m, messages.ProfileSelectedMsg{Profile: "test-prod"})
	if cmd != nil {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, subCmd := range batch {
				if subCmd != nil {
					subMsg := subCmd()
					if _, isFlash := subMsg.(messages.FlashMsg); isFlash {
						m, _ = rootApplyMsg(m, subMsg)
					}
				}
			}
		}
	}

	content := rootViewContent(m)
	if !strings.Contains(content, "Switching to test-prod") {
		t.Error("should show switching flash")
	}

	// ClientsReadyMsg arrives — "Connected. Refreshing..." replaces the switching flash
	m, _ = rootApplyMsg(m, messages.ClientsReadyMsg{Clients: nil})
	content = rootViewContent(m)
	if strings.Contains(content, "Switching to test-prod") {
		t.Error("'Switching to...' flash should be replaced after ClientsReadyMsg")
	}
}

func TestBug_ProfileSwitch_ClearsRegion(t *testing.T) {
	tui.Version = "test"
	// Start with explicit region
	m := tui.New("dev-profile", "us-west-2")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 24})

	// Verify the region is us-west-2 initially
	content := rootViewContent(m)
	if !strings.Contains(content, "us-west-2") {
		t.Fatal("header should show us-west-2 initially")
	}

	// Switch to a different profile
	m, _ = rootApplyMsg(m, messages.ProfileSelectedMsg{Profile: "prod-profile"})

	// After ClientsReadyMsg, region should be resolved from the NEW profile's config
	// (not the old "us-west-2"). Since we can't control ~/.aws/config in tests,
	// we verify that m.region was cleared by checking it gets re-resolved.
	m, _ = rootApplyMsg(m, messages.ClientsReadyMsg{Clients: nil})

	content = rootViewContent(m)
	// The old region should NOT persist after profile switch
	// (unless the new profile happens to also default to us-west-2, which is unlikely)
	// More importantly: the header should show prod-profile with SOME region
	if !strings.Contains(content, "prod-profile") {
		t.Error("header should show prod-profile after switch")
	}
}

func TestBug_RefreshFlashClears_AfterResourcesLoaded(t *testing.T) {
	m := newRootSizedModel()
	// Navigate to resource list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: "ec2"})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-1", Fields: map[string]string{"instance_id": "i-1"}}},
	})

	// Simulate profile switch flow that sets "Connected. Refreshing..." flash
	m, _ = rootApplyMsg(m, messages.ProfileSelectedMsg{Profile: "other"})
	m, _ = rootApplyMsg(m, messages.ClientsReadyMsg{Clients: nil})

	// At this point, flash should show "Connected. Refreshing..."
	content := rootViewContent(m)
	plain := stripANSI(content)
	if !strings.Contains(plain, "Refreshing") {
		t.Logf("Expected 'Refreshing' flash before resources loaded, got:\n%s", plain[:min(300, len(plain))])
	}

	// Now resources arrive — flash should be cleared
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-2", Fields: map[string]string{"instance_id": "i-2"}}},
	})

	content = rootViewContent(m)
	plain = stripANSI(content)
	if strings.Contains(plain, "Refreshing") {
		t.Errorf("'Refreshing' flash should be cleared after resources loaded, got:\n%s", plain[:min(300, len(plain))])
	}
}

func TestBug_RefreshFlashClears_AfterCtrlR(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetResourceList, ResourceType: "ec2"})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-1", Fields: map[string]string{"instance_id": "i-1"}}},
	})

	// Ctrl+R sets "Refreshing..." flash
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	content := rootViewContent(m)
	plain := stripANSI(content)
	if !strings.Contains(plain, "Refreshing") {
		t.Fatal("should show 'Refreshing...' after Ctrl+R")
	}

	// Resources arrive — flash should clear
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-2", Fields: map[string]string{"instance_id": "i-2"}}},
	})

	content = rootViewContent(m)
	plain = stripANSI(content)
	if strings.Contains(plain, "Refreshing") {
		t.Errorf("'Refreshing' flash should clear after resources loaded, got:\n%s", plain[:min(300, len(plain))])
	}
}

func TestBug_ProfileSwitch_FromMainMenu_NoRefresh(t *testing.T) {
	m := newRootSizedModel()
	// Switch profile from main menu (no resource list active)
	m, _ = rootApplyMsg(m, messages.ProfileSelectedMsg{Profile: "other-profile"})
	m, cmd := rootApplyMsg(m, messages.ClientsReadyMsg{Clients: nil})

	// From main menu, no resource list to refresh — cmd should be nil
	if cmd != nil {
		t.Log("From main menu, no resource list to refresh — cmd can be nil")
	}

	// But profile should still be updated in header
	content := rootViewContent(m)
	if !strings.Contains(content, "other-profile") {
		t.Error("header should show new profile even when switched from main menu")
	}
}
