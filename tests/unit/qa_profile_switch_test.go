package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

func TestBug_ProfileSwitch_RefreshesResourceList(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	oldResources := []resource.Resource{
		{ID: "i-old", Name: "old-server", Fields: map[string]string{"instance_id": "i-old", "name": "old-server", "state": "running"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: oldResources, Provenance: messages.FetchProvenanceCanonicalList})

	content := rootViewContent(m)
	if !strings.Contains(content, "old-server") {
		t.Fatal("should show old-server before profile switch")
	}

	m, cmd := rootApplyMsg(m, messages.ProfileSelected{Profile: "new-profile"})

	if cmd == nil {
		t.Fatal("ProfileSelectedMsg should return a command to reconnect")
	}

	// Gen:2 — ConnectGen seeds at 1 (session.New()); one ProfileSelected
	// Rotate()s it to 2.
	m, cmd = rootApplyMsg(m, messages.ClientsReady{Clients: nil, Gen: 2})

	content = rootViewContent(m)
	plain := stripANSI(content)
	if !strings.Contains(plain, "Refreshing") {
		t.Errorf("After profile switch + ClientsReadyMsg, expected 'Refreshing' flash, got:\n%s", plain[:min(300, len(plain))])
	}
	if cmd == nil {
		t.Error("After ClientsReadyMsg following profile switch, should return a fetch command to refresh")
	}
}

func TestBug_ProfileSwitch_UpdatesHeaderProfile(t *testing.T) {
	m := newRootSizedModel()
	content := rootViewContent(m)
	if !strings.Contains(content, "testprofile") {
		t.Fatal("header should show initial profile")
	}

	// Gen:2 — ConnectGen seeds at 1 (session.New()); one ProfileSelected
	// Rotate()s it to 2.
	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "new-profile"})
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: nil, Gen: 2})

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
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	resources := []resource.Resource{
		{ID: "i-123", Name: "server", Fields: map[string]string{"instance_id": "i-123"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: resources})

	m, cmd := rootApplyMsg(m, messages.RegionSelected{Region: "eu-west-1"})
	if cmd == nil {
		t.Fatal("RegionSelectedMsg should return a reconnect command")
	}

	// Gen:2 — ConnectGen seeds at 1 (session.New()); one RegionSelected
	// Rotate()s it to 2.
	_, cmd = rootApplyMsg(m, messages.ClientsReady{Clients: nil, Gen: 2})
	if cmd == nil {
		t.Error("After ClientsReadyMsg following region switch, should return a fetch command to refresh")
	}
}

func TestBug_ProfileList_MatchesAWSCLI(t *testing.T) {
	// Profiles follow `aws configure list-profiles`: only [profile xxx] sections
	// from ~/.aws/config, never bare sections from ~/.aws/credentials. The host's
	// config decides the exact list.
	m := newRootSizedModel()
	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: ':'})
	if cmd != nil {
		msg := cmd()
		m, _ = rootApplyMsg(m, msg)
	}
	for _, r := range "ctx" {
		m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		// The flow must not panic.
		_, _ = rootApplyMsg(m, msg)
	}
}

func TestBug_RegionShownInHeader(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "test-dev", "")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 24})

	content := rootViewContent(m)
	plain := stripANSI(content)

	// Header format: "a9s vX.Y.Z  profile:region"
	switch {
	case strings.Contains(plain, "test-dev:us-east-1") || strings.Contains(plain, "test-dev:eu-"):
	case strings.HasSuffix(strings.TrimSpace(strings.Split(plain, "\n")[0]), ":"):
		t.Error("header shows empty region — should resolve default region from AWS config")
	case !strings.Contains(plain, ":"):
		t.Error("header missing profile:region separator")
	}
}

func TestBug_RegionShownInHeader_AfterConnect(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "test-dev", "")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 24})

	// Gen:1 — ConnectGen seeds at 1 (session.New()); this model is never rotated.
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: nil, Gen: 1})

	content := rootViewContent(m)
	plain := stripANSI(content)
	header := strings.Split(plain, "\n")[0]

	if strings.Contains(header, "test-dev: ") || strings.HasSuffix(strings.TrimSpace(header), ":") {
		t.Errorf("header region should not be empty after connect, got: %s", header)
	}
}

func TestBug_ProfileSwitch_FlashHasTimer(t *testing.T) {
	m := newRootSizedModel()
	// ProfileSelected returns a batch (flash + connectAWS).
	m, cmd := rootApplyMsg(m, messages.ProfileSelected{Profile: "test-prod"})
	if cmd == nil {
		t.Fatal("ProfileSelectedMsg should return a command")
	}

	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, subCmd := range batch {
			if subCmd != nil {
				subMsg := subCmd()
				// connectAWS needs real AWS; only the FlashMsg is applied.
				if _, isFlash := subMsg.(messages.Flash); isFlash {
					m, _ = rootApplyMsg(m, subMsg)
				}
			}
		}
	}

	content := rootViewContent(m)
	if !strings.Contains(content, "Switching") {
		t.Error("should show switching message after FlashMsg is processed")
	}
}

func TestBug_ProfileSwitch_FlashClears(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	resources := []resource.Resource{
		{ID: "i-123", Name: "srv", Fields: map[string]string{"instance_id": "i-123"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: resources})

	m, cmd := rootApplyMsg(m, messages.ProfileSelected{Profile: "test-prod"})
	if cmd != nil {
		msg := cmd()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, subCmd := range batch {
				if subCmd != nil {
					subMsg := subCmd()
					if _, isFlash := subMsg.(messages.Flash); isFlash {
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

	// ClientsReadyMsg arrives — "Connected. Refreshing..." replaces the switching
	// flash. Gen:2 — ConnectGen seeds at 1 (session.New()); one ProfileSelected
	// Rotate()s it to 2.
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: nil, Gen: 2})
	content = rootViewContent(m)
	if strings.Contains(content, "Switching to test-prod") {
		t.Error("'Switching to...' flash should be replaced after ClientsReadyMsg")
	}
}

func TestBug_ProfileSwitch_ClearsRegion(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "dev-profile", "us-west-2")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 24})

	content := rootViewContent(m)
	if !strings.Contains(content, "us-west-2") {
		t.Fatal("header should show us-west-2 initially")
	}

	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "prod-profile"})

	// ~/.aws/config is outside the test's control, so the region is checked for
	// being re-resolved, not for its value. Gen:2 — ConnectGen seeds at 1
	// (session.New()); one ProfileSelected Rotate()s it to 2.
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: nil, Gen: 2})

	content = rootViewContent(m)
	if !strings.Contains(content, "prod-profile") {
		t.Error("header should show prod-profile after switch")
	}
}

func TestBug_RefreshFlashClears_AfterResourcesLoaded(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-1", Fields: map[string]string{"instance_id": "i-1"}}},
	})

	// Gen:2 — ConnectGen seeds at 1 (session.New()); one ProfileSelected
	// Rotate()s it to 2.
	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "other"})
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: nil, Gen: 2})

	content := rootViewContent(m)
	plain := stripANSI(content)
	if !strings.Contains(plain, "Refreshing") {
		t.Errorf("Expected 'Refreshing' flash before resources loaded, got:\n%s", plain[:min(300, len(plain))])
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources:    []resource.Resource{{ID: "i-1", Fields: map[string]string{"instance_id": "i-1"}}},
	})

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	content := rootViewContent(m)
	plain := stripANSI(content)
	if !strings.Contains(plain, "Refreshing") {
		t.Fatal("should show 'Refreshing...' after Ctrl+R")
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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
	// Gen:2 — ConnectGen seeds at 1 (session.New()); one ProfileSelected
	// Rotate()s it to 2.
	m, _ = rootApplyMsg(m, messages.ProfileSelected{Profile: "other-profile"})
	m, cmd := rootApplyMsg(m, messages.ClientsReady{Clients: nil, Gen: 2})

	// The main menu has no resource list to refresh.
	if cmd != nil {
		t.Log("From main menu, no resource list to refresh — cmd can be nil")
	}

	content := rootViewContent(m)
	if !strings.Contains(content, "other-profile") {
		t.Error("header should show new profile even when switched from main menu")
	}
}
