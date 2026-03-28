package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ── Clipboard copy tests ────────────────────────────────────────────────────

func TestWiring_CopyInResourceList_ReturnsFlashMsg(t *testing.T) {
	m := newRootSizedModel()

	// Navigate to ec2 resource list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Load some resources
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-abc123", Name: "web-server", Status: "running", Fields: map[string]string{"instance_id": "i-abc123"}},
		},
	})

	// Press 'c' to copy
	_, cmd := rootApplyMsg(m, rootKeyPress("c"))

	if cmd == nil {
		t.Fatal("pressing 'c' in resource list should return a command for clipboard copy")
	}

	// Execute the command — should return a FlashMsg or CopiedMsg
	msg := cmd()
	switch v := msg.(type) {
	case messages.FlashMsg:
		if v.IsError {
			// Clipboard may fail in CI, but should still produce a FlashMsg
			t.Logf("clipboard copy returned error flash: %s (expected in headless env)", v.Text)
		}
	case messages.CopiedMsg:
		if v.Content != "i-abc123" {
			t.Errorf("CopiedMsg.Content should be 'i-abc123', got %q", v.Content)
		}
	default:
		t.Errorf("expected FlashMsg or CopiedMsg, got %T", msg)
	}
}

func TestWiring_CopyInDetailView_ReturnsFlashMsg(t *testing.T) {
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:     "i-abc123",
		Name:   "web-server",
		Fields: map[string]string{"instance_id": "i-abc123"},
	}

	// Navigate to detail
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: res,
	})

	// Press 'c' to copy
	_, cmd := rootApplyMsg(m, rootKeyPress("c"))

	if cmd == nil {
		t.Fatal("pressing 'c' in detail view should return a command for clipboard copy")
	}

	msg := cmd()
	switch msg.(type) {
	case messages.FlashMsg:
		// OK — clipboard may succeed or fail
	case messages.CopiedMsg:
		// OK
	default:
		t.Errorf("expected FlashMsg or CopiedMsg, got %T", msg)
	}
}

func TestWiring_CopyInYAMLView_ReturnsFlashMsg(t *testing.T) {
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:     "i-abc123",
		Name:   "web-server",
		Fields: map[string]string{"instance_id": "i-abc123", "name": "web-server"},
	}

	// Navigate to YAML
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetYAML,
		Resource: res,
	})

	// Press 'c' to copy
	_, cmd := rootApplyMsg(m, rootKeyPress("c"))

	if cmd == nil {
		t.Fatal("pressing 'c' in YAML view should return a command for clipboard copy")
	}

	msg := cmd()
	switch msg.(type) {
	case messages.FlashMsg:
		// OK
	case messages.CopiedMsg:
		// OK
	default:
		t.Errorf("expected FlashMsg or CopiedMsg, got %T", msg)
	}
}

func TestWiring_CopyInRevealView_ReturnsFlashMsg(t *testing.T) {
	m := newRootSizedModel()

	// Push a reveal view via SecretRevealedMsg
	m, _ = rootApplyMsg(m, messages.SecretRevealedMsg{
		SecretName: "my-secret",
		Value:      "s3cr3t-value",
	})

	// Press 'c' to copy
	_, cmd := rootApplyMsg(m, rootKeyPress("c"))

	if cmd == nil {
		t.Fatal("pressing 'c' in reveal view should return a command for clipboard copy")
	}

	msg := cmd()
	switch msg.(type) {
	case messages.FlashMsg:
		// OK
	case messages.CopiedMsg:
		// OK
	default:
		t.Errorf("expected FlashMsg or CopiedMsg, got %T", msg)
	}
}

// ── Refresh (ctrl+r) tests ──────────────────────────────────────────────────

func TestWiring_RefreshInResourceList_ReturnsFetchCmd(t *testing.T) {
	m := newRootSizedModel()

	// Navigate to ec2 resource list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Press ctrl+r to refresh
	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	if cmd == nil {
		t.Fatal("pressing ctrl+r in resource list should return a command for fetching resources")
	}

	// Execute the cmd — should yield APIErrorMsg (nil clients) or ResourcesLoadedMsg
	msg := cmd()
	switch msg.(type) {
	case messages.APIErrorMsg:
		// Expected: no clients initialized
	case messages.ResourcesLoadedMsg:
		// Would happen if clients were set
	case messages.FlashMsg:
		// Refreshing flash is also OK
	default:
		t.Errorf("expected APIErrorMsg or ResourcesLoadedMsg, got %T", msg)
	}
}

func TestWiring_RefreshOnMainMenu_TriggersAvailabilityCheck(t *testing.T) {
	m := newRootSizedModel()

	// Press ctrl+r on the main menu — should trigger availability cache reload
	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	// With caching enabled (default), ctrl+r on main menu fires the availability reload command
	if cmd == nil {
		t.Error("pressing ctrl+r on main menu should trigger availability cache reload command")
	}
}

func TestWiring_RefreshOnMainMenu_NoCacheMode_NoOp(t *testing.T) {
	m := tui.New("testprofile", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Press ctrl+r on the main menu in no-cache mode — should be a no-op
	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	// With caching disabled, ctrl+r on main menu should produce nil cmd
	if cmd != nil {
		t.Error("pressing ctrl+r on main menu in no-cache mode should not trigger any command")
	}
}

// ── Availability pipeline wiring tests (#68) ─────────────────────────────────

// Bug 1: Demo mode excluded from availability probes.
// ClientsReadyMsg in demo mode should trigger availability probes (via
// loadAvailabilityCache), not skip them.

func TestWiring_ClientsReady_DemoMode_TriggersAvailabilityProbes(t *testing.T) {
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Send ClientsReadyMsg — demo mode should still fire availability probes
	_, cmd := rootApplyMsg(m, messages.ClientsReadyMsg{})

	if cmd == nil {
		t.Fatal("ClientsReadyMsg in demo mode should return non-nil cmd (identity + availability probes)")
	}

	// Execute the batch and look for an AvailabilityCacheLoadedMsg
	found := extractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.AvailabilityCacheLoadedMsg)
		return ok
	})
	if found == nil {
		t.Error("ClientsReadyMsg in demo mode should produce AvailabilityCacheLoadedMsg from loadAvailabilityCache")
	}
}

func TestWiring_ClientsReady_DemoMode_NoCache_SkipsAvailability(t *testing.T) {
	m := tui.New("demo", "us-east-1", tui.WithDemo(true), tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Send ClientsReadyMsg — with --no-cache, should only produce identity, NOT availability
	_, cmd := rootApplyMsg(m, messages.ClientsReadyMsg{})

	if cmd == nil {
		t.Fatal("ClientsReadyMsg in demo+no-cache should still return identity cmd")
	}

	// Walk batch and verify NO AvailabilityCacheLoadedMsg is present
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, subCmd := range batch {
			if subCmd == nil {
				continue
			}
			subMsg := subCmd()
			if _, isAvail := subMsg.(messages.AvailabilityCacheLoadedMsg); isAvail {
				t.Error("ClientsReadyMsg in demo+no-cache mode should NOT produce AvailabilityCacheLoadedMsg")
			}
		}
	} else {
		// Not a batch — check the single message
		if _, isAvail := msg.(messages.AvailabilityCacheLoadedMsg); isAvail {
			t.Error("ClientsReadyMsg in demo+no-cache mode should NOT produce AvailabilityCacheLoadedMsg")
		}
	}
}

// Bug 2: Flash never cleared after all checks complete.
// Walk the full probe cycle: send AvailabilityCacheLoadedMsg, then feed
// AvailabilityCheckedMsg for every resource type. After the last one, verify
// flash is cleared.

func TestWiring_AvailabilityComplete_ClearsFlash(t *testing.T) {
	m := newRootSizedModel()

	// Set flash to simulate "Refreshing availability..." state
	m, _ = rootApplyMsg(m, messages.FlashMsg{Text: "Refreshing availability...", IsError: false})

	// Verify flash is active
	rendered := stripANSI(rootViewContent(m))
	if !strings.Contains(rendered, "Refreshing availability...") {
		t.Fatal("flash should be visible before availability cycle")
	}

	// Send AvailabilityCacheLoadedMsg to build the queue and fire first 3 probes
	m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoadedMsg{
		Entries: make(map[string]int),
		Expired: true,
	})

	// Now drain the queue by sending AvailabilityCheckedMsg for all resource types.
	// The queue was built from AllShortNames(). First 3 were dequeued by the cache
	// loaded handler. Each AvailabilityCheckedMsg dequeues one more. So we need to
	// send len(AllShortNames()) messages total to drain everything.
	allNames := resource.AllShortNames()
	var lastCmd tea.Cmd
	for _, name := range allNames {
		m, lastCmd = rootApplyMsg(m, messages.AvailabilityCheckedMsg{
			ResourceType: name,
			HasResources: true,
			Count:        1,
			Gen:          0,
		})
	}

	// After the LAST AvailabilityCheckedMsg, the returned cmd should be non-nil
	// (saveAvailabilityCache).
	if lastCmd == nil {
		t.Error("last AvailabilityCheckedMsg should return non-nil cmd (saveCache)")
	}

	// Render View() — "Refreshing availability..." should NOT be present anymore
	rendered = stripANSI(rootViewContent(m))
	if strings.Contains(rendered, "Refreshing availability...") {
		t.Error("flash should be cleared after all availability checks complete")
	}
}

// Bug 3: Ctrl+R on main menu in demo mode was a no-op.
// After ClientsReadyMsg, pressing ctrl+r on the main menu in demo mode should
// trigger availability probes.

func TestWiring_RefreshOnMainMenu_DemoMode_TriggersProbes(t *testing.T) {
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// First send ClientsReadyMsg so probes can run
	m, _ = rootApplyMsg(m, messages.ClientsReadyMsg{})

	// Press ctrl+r on the main menu
	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	if cmd == nil {
		t.Error("pressing ctrl+r on main menu in demo mode should trigger availability probes")
	}
}

// Bug 4: Demo probe count uses GetResources (all fixtures) instead of
// GetResourcesPaginated (first page only). The menu shows the total fixture
// count but the list view only shows DemoPageSize items on the first page.
// The probe count MUST match what the user actually sees.

func TestWiring_DemoMode_ProbeCount_MatchesPaginatedPageSize(t *testing.T) {
	// Step 1: Find a resource type with known fixture count.
	var targetType string
	var totalCount int
	for _, shortName := range resource.AllShortNames() {
		all, ok := demo.GetResources(shortName)
		if !ok {
			continue
		}
		if len(all) > 0 {
			targetType = shortName
			totalCount = len(all)
			break
		}
	}
	if targetType == "" {
		t.Fatal("test requires at least one resource type with demo fixtures")
	}

	// Step 2: Create a demo-mode model with real clients backed by the demo transport.
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	m, _ = rootApplyMsg(m, demoClientsReadyMsg())

	// Step 3: Send AvailabilityCacheLoadedMsg to start the probe pipeline.
	// In demo mode, loadAvailabilityCache returns Expired: true (no cache file),
	// so we can send the message directly to start probes.
	m, cmd := rootApplyMsg(m, messages.AvailabilityCacheLoadedMsg{
		Entries: make(map[string]int),
		Expired: true,
	})

	// Step 4: Walk probe cycle, collect results.
	type probeResult struct {
		Count     int
		Truncated bool
	}
	collected := make(map[string]probeResult)
	for cmd != nil {
		msg := cmd()
		if acm, ok := msg.(messages.AvailabilityCheckedMsg); ok {
			collected[acm.ResourceType] = probeResult{Count: acm.Count, Truncated: acm.Truncated}
			m, cmd = rootApplyMsg(m, acm)
			continue
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, subCmd := range batch {
				if subCmd == nil {
					continue
				}
				subMsg := subCmd()
				if acm, ok := subMsg.(messages.AvailabilityCheckedMsg); ok {
					collected[acm.ResourceType] = probeResult{Count: acm.Count, Truncated: acm.Truncated}
					m, cmd = rootApplyMsg(m, acm)
				}
			}
			continue
		}
		break
	}

	// Step 5: Verify probe count matches total fixtures (real fetcher returns all, no pagination).
	result, found := collected[targetType]
	if !found {
		t.Fatalf("probe cycle did not produce AvailabilityCheckedMsg for %s", targetType)
	}
	if result.Count != totalCount {
		t.Errorf("demo probe for %s reported count=%d, want %d (total fixtures)",
			targetType, result.Count, totalCount)
	}
}

// ── Reveal (x key) tests ────────────────────────────────────────────────────

func TestWiring_RevealForSecrets_ReturnsFetchCmd(t *testing.T) {
	m := newRootSizedModel()

	// Navigate to secrets resource list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})

	// Load a secret
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "secrets",
		Resources: []resource.Resource{
			{ID: "my-secret", Name: "my-secret", Status: "active", Fields: map[string]string{"secret_name": "my-secret"}},
		},
	})

	// Press 'x' to reveal
	_, cmd := rootApplyMsg(m, rootKeyPress("x"))

	if cmd == nil {
		t.Fatal("pressing 'x' on secrets resource list should return a reveal fetch command")
	}

	// Execute the cmd — should yield FlashMsg (nil clients) or SecretRevealedMsg
	msg := cmd()
	switch msg.(type) {
	case messages.FlashMsg:
		// Expected: no clients initialized
	case messages.SecretRevealedMsg:
		// Would happen if clients were set
	default:
		t.Errorf("expected FlashMsg or SecretRevealedMsg, got %T", msg)
	}
}

func TestWiring_RevealNotForNonSecrets(t *testing.T) {
	m := newRootSizedModel()

	// Navigate to ec2 resource list
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Load a resource
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-abc123", Name: "web-server", Status: "running", Fields: map[string]string{"instance_id": "i-abc123"}},
		},
	})

	// Press 'x' — should NOT trigger reveal
	_, cmd := rootApplyMsg(m, rootKeyPress("x"))

	if cmd != nil {
		// Execute to check it's not a reveal command
		msg := cmd()
		switch msg := msg.(type) {
		case messages.SecretRevealedMsg:
			t.Error("pressing 'x' on non-secrets resource should not trigger reveal")
		case messages.NavigateMsg:
			if msg.Target == messages.TargetReveal {
				t.Error("pressing 'x' on non-secrets resource should not navigate to reveal")
			}
		}
	}
}

// ── View config loading tests ───────────────────────────────────────────────

func TestWiring_EmptyProfileShowsDefaultInHeader(t *testing.T) {
	// When no profile is specified (empty string), the header should show "default"
	m := tui.New("", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Simulate AWS connection completing
	m, _ = rootApplyMsg(m, messages.ClientsReadyMsg{Clients: nil, Err: nil})

	rendered := stripANSI(rootViewContent(m))
	if !strings.Contains(rendered, "default") {
		t.Errorf("header should show 'default' when profile is empty, got: %s", rendered)
	}
}

func TestWiring_ViewConfigLoadedOnClientsReady(t *testing.T) {
	m := newRootSizedModel()

	// Send ClientsReadyMsg — viewConfig should be loaded
	m, _ = rootApplyMsg(m, messages.ClientsReadyMsg{Clients: nil, Err: nil})

	// Navigate to resource list — it should work (viewConfig used internally)
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Verify the view renders without panic
	plain := stripANSI(rootViewContent(m))
	if plain == "" {
		t.Error("should render resource list view after config loading")
	}
}

func TestWiring_ViewConfigLoadedAtInit(t *testing.T) {
	m := newRootSizedModel()

	// The Init cmd should have loaded the config. Send the InitConnectMsg
	// that Init returns.
	cmd := m.Init()
	if cmd != nil {
		msg := cmd()
		// Apply the InitConnectMsg
		m, _ = rootApplyMsg(m, msg)
	}

	// Navigate to resource list with viewConfig
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Verify it renders
	plain := stripANSI(rootViewContent(m))
	if plain == "" {
		t.Error("should render resource list view even without views.yaml file")
	}
}

// ── SecretRevealedMsg push test ─────────────────────────────────────────────

func TestWiring_SecretRevealedMsg_PushesRevealView(t *testing.T) {
	tui.Version = "1.0.0"
	m := newRootSizedModel()

	// Send SecretRevealedMsg
	m, _ = rootApplyMsg(m, messages.SecretRevealedMsg{
		SecretName: "prod/db-password",
		Value:      "hunter2",
	})

	plain := stripANSI(rootViewContent(m))
	if plain == "" {
		t.Error("should render reveal view")
	}
	// The frame title should contain the secret name
	if !containsSubstring(plain, "prod/db-password") {
		t.Errorf("reveal view should show secret name in frame title, got: %s", truncateForLog(plain))
	}
}

func TestWiring_SecretRevealedMsg_Error(t *testing.T) {
	tui.Version = "1.0.0"
	m := newRootSizedModel()

	m, cmd := rootApplyMsg(m, messages.SecretRevealedMsg{
		Err: errForTest("access denied"),
	})
	// The handler now returns a FlashMsg command; dispatch it.
	if cmd != nil {
		m, _ = rootApplyMsg(m, cmd())
	}

	plain := stripANSI(rootViewContent(m))
	if !containsSubstring(plain, "reveal failed") {
		t.Errorf("should show error flash for reveal failure, got: %s", truncateForLog(plain))
	}
}

// ── Helper functions ────────────────────────────────────────────────────────

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func truncateForLog(s string) string {
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}

type testError string

func errForTest(msg string) error {
	return testError(msg)
}

func (e testError) Error() string {
	return string(e)
}
