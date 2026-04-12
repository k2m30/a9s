package unit

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// findNavigateMsg walks a tea.Cmd (including nested BatchMsg) and returns the
// first messages.NavigateMsg found, or nil if none is found.
// Handles nil cmd and non-batch cases gracefully.
func findNavigateMsg(cmd tea.Cmd) *messages.NavigateMsg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if nav, ok := msg.(messages.NavigateMsg); ok {
		return &nav
	}
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return nil
	}
	for _, subCmd := range batch {
		if subCmd == nil {
			continue
		}
		subMsg := subCmd()
		if nav, ok := subMsg.(messages.NavigateMsg); ok {
			return &nav
		}
		if subBatch, ok := subMsg.(tea.BatchMsg); ok {
			for _, innerCmd := range subBatch {
				if innerCmd == nil {
					continue
				}
				innerMsg := innerCmd()
				if nav, ok := innerMsg.(messages.NavigateMsg); ok {
					return &nav
				}
			}
		}
	}
	return nil
}

// TestQA_CLICommand_ClientsReady_EmitsNavigateMsg verifies that when a model is
// constructed with WithCommand("ec2") and a ClientsReadyMsg is received, the
// returned cmd batch contains a NavigateMsg targeting the ec2 resource list.
//
// Uses demo clients + WithNoCache to route handleClientsReady through the
// demo branch, which skips fetchIdentity (safe to walk with extractMsg).
func TestQA_CLICommand_ClientsReady_EmitsNavigateMsg(t *testing.T) {
	m := tui.New(
		"testprofile",
		"us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithNoCache(true),
		tui.WithCommand("ec2"),
	)
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	_, cmd := rootApplyMsg(m, messages.ClientsReadyMsg{
		Clients: demo.NewServiceClients(),
		Region:  "us-east-1",
	})

	nav := extractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.NavigateMsg)
		return ok
	})

	navMsg, ok := nav.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected messages.NavigateMsg, got %T", nav)
	}
	if navMsg.Target != messages.TargetResourceList {
		t.Errorf("NavigateMsg.Target should be TargetResourceList, got %v", navMsg.Target)
	}
	if navMsg.ResourceType != "ec2" {
		t.Errorf("NavigateMsg.ResourceType should be %q, got %q", "ec2", navMsg.ResourceType)
	}
}

// TestQA_CLICommand_ClientsReady_NoNavigateMsg_WhenUnset verifies that when a
// model is constructed WITHOUT WithCommand, receiving ClientsReadyMsg does NOT
// produce a NavigateMsg in the returned batch.
func TestQA_CLICommand_ClientsReady_NoNavigateMsg_WhenUnset(t *testing.T) {
	m := tui.New(
		"testprofile",
		"us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithNoCache(true),
	)
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	_, cmd := rootApplyMsg(m, messages.ClientsReadyMsg{
		Clients: demo.NewServiceClients(),
		Region:  "us-east-1",
	})

	nav := findNavigateMsg(cmd)
	if nav != nil {
		t.Errorf("expected no NavigateMsg when WithCommand is not set, got NavigateMsg{Target:%v, ResourceType:%q}", nav.Target, nav.ResourceType)
	}
}

// TestQA_CLICommand_ClientsReady_ClearedAfterFirstUse verifies that the command
// stored via WithCommand is cleared after the first ClientsReadyMsg, so that
// a subsequent ClientsReadyMsg (e.g. profile/region switch) does NOT re-emit
// a NavigateMsg.
func TestQA_CLICommand_ClientsReady_ClearedAfterFirstUse(t *testing.T) {
	m := tui.New(
		"testprofile",
		"us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithNoCache(true),
		tui.WithCommand("s3"),
	)
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	crm := messages.ClientsReadyMsg{
		Clients: demo.NewServiceClients(),
		Region:  "us-east-1",
	}

	// First ClientsReadyMsg — should emit NavigateMsg for "s3".
	var firstCmd tea.Cmd
	m, firstCmd = rootApplyMsg(m, crm)

	_ = extractMsg(t, firstCmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.NavigateMsg)
		return ok
	})

	// Second ClientsReadyMsg — command should have been cleared; no NavigateMsg.
	_, secondCmd := rootApplyMsg(m, crm)

	nav := findNavigateMsg(secondCmd)
	if nav != nil {
		t.Errorf("expected no NavigateMsg on second ClientsReadyMsg (command should have been cleared), got NavigateMsg{Target:%v, ResourceType:%q}", nav.Target, nav.ResourceType)
	}
}

// TestQA_CLICommand_DemoMode_EmitsNavigateMsg verifies that the WithCommand
// option also triggers a NavigateMsg on the demo/no-cache path in
// handleClientsReady (the `if m.noCache` branch).
func TestQA_CLICommand_DemoMode_EmitsNavigateMsg(t *testing.T) {
	m := tui.New(
		"demo",
		"us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithNoCache(true),
		tui.WithCommand("s3"),
	)
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	_, cmd := rootApplyMsg(m, messages.ClientsReadyMsg{
		Clients: demo.NewServiceClients(),
		Region:  "us-east-1",
	})

	nav := extractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.NavigateMsg)
		return ok
	})

	navMsg, ok := nav.(messages.NavigateMsg)
	if !ok {
		t.Fatalf("expected messages.NavigateMsg, got %T", nav)
	}
	if navMsg.Target != messages.TargetResourceList {
		t.Errorf("NavigateMsg.Target should be TargetResourceList, got %v", navMsg.Target)
	}
	if navMsg.ResourceType != "s3" {
		t.Errorf("NavigateMsg.ResourceType should be %q, got %q", "s3", navMsg.ResourceType)
	}
}

// TestQA_CLICommand_SkippedWhenUserNavigatedAway verifies that if the initial
// AWS connection is slow and the user navigates away from the main menu before
// ClientsReadyMsg arrives, the -c auto-navigation is suppressed to avoid
// pushing a view on top of whatever the user is doing.
func TestQA_CLICommand_SkippedWhenUserNavigatedAway(t *testing.T) {
	m := tui.New(
		"testprofile",
		"us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithNoCache(true),
		tui.WithCommand("ec2"),
	)
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Simulate user navigating to help before ClientsReadyMsg arrives.
	m, _ = rootApplyMsg(m, messages.NavigateMsg{Target: messages.TargetHelp})

	// Now ClientsReadyMsg arrives — stack depth is 2 (menu + help).
	_, cmd := rootApplyMsg(m, messages.ClientsReadyMsg{
		Clients: demo.NewServiceClients(),
		Region:  "us-east-1",
	})

	nav := findNavigateMsg(cmd)
	if nav != nil {
		t.Errorf("expected no NavigateMsg when user already navigated away from menu, got NavigateMsg{Target:%v, ResourceType:%q}", nav.Target, nav.ResourceType)
	}
}
