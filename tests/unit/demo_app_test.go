package unit

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/internal/tui"
	"github.com/k2m30/a9s/internal/tui/messages"
)

// ═══════════════════════════════════════════════════════════════════════════
// Demo mode app.go integration tests — verify root model demo-mode behavior.
// ═══════════════════════════════════════════════════════════════════════════

// ---------------------------------------------------------------------------
// 1. TestDemoMode_Init_NoAWSConnection
// ---------------------------------------------------------------------------

func TestDemoMode_Init_NoAWSConnection(t *testing.T) {
	model := tui.New("demo", "us-east-1", tui.WithDemo(true))
	cmd := model.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil cmd; expected a cmd that produces ClientsReadyMsg")
	}
	msg := cmd()
	crm, ok := msg.(messages.ClientsReadyMsg)
	if !ok {
		t.Fatalf("Init() cmd produced %T; expected messages.ClientsReadyMsg", msg)
	}
	if crm.Clients != nil {
		t.Error("ClientsReadyMsg.Clients should be nil in demo mode")
	}
	if crm.Err != nil {
		t.Errorf("ClientsReadyMsg.Err should be nil in demo mode; got %v", crm.Err)
	}
}

// ---------------------------------------------------------------------------
// 2. TestDemoMode_FetchResources_EC2
// ---------------------------------------------------------------------------

func TestDemoMode_FetchResources_EC2(t *testing.T) {
	model := tui.New("demo", "us-east-1", tui.WithDemo(true))

	// Send ClientsReadyMsg to move past initialization
	var m tea.Model = model
	m, _ = m.Update(messages.ClientsReadyMsg{})

	// Navigate to EC2 resource list
	_, cmd := m.Update(messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	if cmd == nil {
		t.Fatal("NavigateMsg for EC2 returned nil cmd; expected a fetch command")
	}

	// The cmd may be a tea.BatchMsg (from Init + fetchResources). We need to find
	// the ResourcesLoadedMsg by executing all returned cmds.
	msg := extractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoadedMsg)
		return ok
	})
	rlm, ok := msg.(messages.ResourcesLoadedMsg)
	if !ok {
		t.Fatalf("expected ResourcesLoadedMsg; got %T", msg)
	}
	if len(rlm.Resources) == 0 {
		t.Error("ResourcesLoadedMsg.Resources is empty; expected demo EC2 fixtures")
	}
}

// ---------------------------------------------------------------------------
// 3. TestDemoMode_FetchResources_Unknown
// ---------------------------------------------------------------------------

func TestDemoMode_FetchResources_Unknown(t *testing.T) {
	model := tui.New("demo", "us-east-1", tui.WithDemo(true))

	// Send ClientsReadyMsg to move past initialization
	var m tea.Model = model
	m, _ = m.Update(messages.ClientsReadyMsg{})

	// Navigate to a non-demo resource type (redis)
	_, cmd := m.Update(messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "redis",
	})
	if cmd == nil {
		t.Fatal("NavigateMsg for redis returned nil cmd; expected a fetch command")
	}

	msg := extractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoadedMsg)
		return ok
	})
	rlm, ok := msg.(messages.ResourcesLoadedMsg)
	if !ok {
		t.Fatalf("expected ResourcesLoadedMsg; got %T", msg)
	}
	// Unknown demo type should return empty (not an error)
	if len(rlm.Resources) != 0 {
		t.Errorf("expected 0 resources for non-demo type; got %d", len(rlm.Resources))
	}
}

// ---------------------------------------------------------------------------
// 4. TestDemoMode_BlockedCommand_Ctx
// ---------------------------------------------------------------------------

func TestDemoMode_BlockedCommand_Ctx(t *testing.T) {
	model := tui.New("demo", "us-east-1", tui.WithDemo(true))

	var m tea.Model = model
	m, _ = m.Update(messages.ClientsReadyMsg{})

	// Execute :ctx command via NavigateMsg (same path as executeCommand)
	_, cmd := m.Update(messages.NavigateMsg{Target: messages.TargetProfile})
	if cmd == nil {
		t.Fatal("NavigateMsg for TargetProfile returned nil cmd; expected flash message")
	}
	msg := cmd()
	flash, ok := msg.(messages.FlashMsg)
	if !ok {
		t.Fatalf("expected FlashMsg; got %T", msg)
	}
	if !flash.IsError {
		t.Error("expected IsError=true for blocked profile command")
	}
	if flash.Text == "" {
		t.Error("expected non-empty flash text for blocked profile command")
	}
}

// ---------------------------------------------------------------------------
// 5. TestDemoMode_BlockedCommand_Region
// ---------------------------------------------------------------------------

func TestDemoMode_BlockedCommand_Region(t *testing.T) {
	model := tui.New("demo", "us-east-1", tui.WithDemo(true))

	var m tea.Model = model
	m, _ = m.Update(messages.ClientsReadyMsg{})

	// Execute :region command via NavigateMsg
	_, cmd := m.Update(messages.NavigateMsg{Target: messages.TargetRegion})
	if cmd == nil {
		t.Fatal("NavigateMsg for TargetRegion returned nil cmd; expected flash message")
	}
	msg := cmd()
	flash, ok := msg.(messages.FlashMsg)
	if !ok {
		t.Fatalf("expected FlashMsg; got %T", msg)
	}
	if !flash.IsError {
		t.Error("expected IsError=true for blocked region command")
	}
	if flash.Text == "" {
		t.Error("expected non-empty flash text for blocked region command")
	}
}

// ---------------------------------------------------------------------------
// 6. TestDemoMode_BlockedReveal
// ---------------------------------------------------------------------------

func TestDemoMode_BlockedReveal(t *testing.T) {
	model := tui.New("demo", "us-east-1", tui.WithDemo(true))

	var m tea.Model = model
	m, _ = m.Update(messages.ClientsReadyMsg{})

	// Navigate to secrets resource list
	m, cmd := m.Update(messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})

	// Execute only the ResourcesLoadedMsg from fetch (skip timer cmds)
	if cmd != nil {
		msg := extractMsg(t, cmd, func(msg tea.Msg) bool {
			_, ok := msg.(messages.ResourcesLoadedMsg)
			return ok
		})
		m, _ = m.Update(msg)
	}

	// Simulate pressing 'x' key — this triggers handleReveal via handleKeyMsg.
	// In demo mode, handleReveal returns a FlashMsg immediately (before checking
	// if we're on a secrets view or have a selected resource).
	xKey := tea.KeyPressMsg{Code: -1, Text: "x"}
	_, cmd = m.Update(xKey)
	if cmd == nil {
		t.Fatal("reveal key in demo mode returned nil cmd; expected FlashMsg")
	}
	revealMsg := cmd()
	flash, ok := revealMsg.(messages.FlashMsg)
	if !ok {
		t.Fatalf("expected FlashMsg from blocked reveal; got %T", revealMsg)
	}
	if !flash.IsError {
		t.Error("expected IsError=true for blocked reveal command")
	}
	if flash.Text == "" {
		t.Error("expected non-empty flash text for blocked reveal")
	}
}

// ---------------------------------------------------------------------------
// 7. TestDemoMode_RefreshReturnsSameData
// ---------------------------------------------------------------------------

func TestDemoMode_RefreshReturnsSameData(t *testing.T) {
	model := tui.New("demo", "us-east-1", tui.WithDemo(true))

	var m tea.Model = model
	m, _ = m.Update(messages.ClientsReadyMsg{})

	// Navigate to EC2
	m, cmd := m.Update(messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Execute first fetch
	msg := extractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.ResourcesLoadedMsg)
		return ok
	})
	rlm1 := msg.(messages.ResourcesLoadedMsg)
	firstCount := len(rlm1.Resources)

	// Deliver the resources to the model
	m, _ = m.Update(rlm1)

	// Now trigger a LoadResourcesMsg (refresh path)
	_, cmd = m.Update(messages.LoadResourcesMsg{ResourceType: "ec2"})
	if cmd == nil {
		t.Fatal("LoadResourcesMsg returned nil cmd; expected fetch command")
	}

	msg2 := cmd()
	rlm2, ok := msg2.(messages.ResourcesLoadedMsg)
	if !ok {
		t.Fatalf("expected ResourcesLoadedMsg on refresh; got %T", msg2)
	}
	if len(rlm2.Resources) != firstCount {
		t.Errorf("refresh returned %d resources; expected %d (same as initial)", len(rlm2.Resources), firstCount)
	}
}

// ---------------------------------------------------------------------------
// 8. TestNonDemoMode_Unchanged
// ---------------------------------------------------------------------------

func TestNonDemoMode_Unchanged(t *testing.T) {
	model := tui.New("", "")
	cmd := model.Init()
	if cmd == nil {
		t.Fatal("Init() returned nil cmd; expected InitConnectMsg")
	}
	msg := cmd()
	_, ok := msg.(messages.InitConnectMsg)
	if !ok {
		t.Fatalf("Init() produced %T; expected messages.InitConnectMsg", msg)
	}
}

// ---------------------------------------------------------------------------
// Helper: extractMsg walks batch commands to find a message matching pred.
// ---------------------------------------------------------------------------

func extractMsg(t *testing.T, cmd tea.Cmd, pred func(tea.Msg) bool) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("extractMsg: cmd is nil")
		return nil
	}
	msg := cmd()
	if pred(msg) {
		return msg
	}
	// If it's a BatchMsg, recurse into each sub-cmd.
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, subCmd := range batch {
			if subCmd == nil {
				continue
			}
			subMsg := subCmd()
			if pred(subMsg) {
				return subMsg
			}
			// Handle nested batches
			if subBatch, ok := subMsg.(tea.BatchMsg); ok {
				for _, innerCmd := range subBatch {
					if innerCmd == nil {
						continue
					}
					innerMsg := innerCmd()
					if pred(innerMsg) {
						return innerMsg
					}
				}
			}
		}
	}
	t.Fatalf("extractMsg: no message matched predicate (got %T)", msg)
	return nil
}

