package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"context"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

func TestWiring_CopyInResourceList_ReturnsFlashMsg(t *testing.T) {
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-abc123", Name: "web-server", Fields: map[string]string{"instance_id": "i-abc123"}},
		},
	})

	_, cmd := rootApplyMsg(m, rootKeyPress("c"))

	if cmd == nil {
		t.Fatal("pressing 'c' in resource list should return a command for clipboard copy")
	}

	msg := cmd()
	switch v := msg.(type) {
	case messages.Flash:
		if v.IsError {
			// Clipboard may fail in CI, but should still produce a FlashMsg
			t.Logf("clipboard copy returned error flash: %s (expected in headless env)", v.Text)
		}
	default:
		t.Errorf("expected FlashMsg, got %T", msg)
	}
}

func TestWiring_CopyInDetailView_ReturnsFlashMsg(t *testing.T) {
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:     "i-abc123",
		Name:   "web-server",
		Fields: map[string]string{"instance_id": "i-abc123"},
	}

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: res,
	})

	_, cmd := rootApplyMsg(m, rootKeyPress("c"))

	if cmd == nil {
		t.Fatal("pressing 'c' in detail view should return a command for clipboard copy")
	}

	msg := cmd()
	switch msg.(type) {
	case messages.Flash:
		// OK — clipboard may succeed or fail
	default:
		t.Errorf("expected FlashMsg, got %T", msg)
	}
}

func TestWiring_CopyInDetailView_UsesActiveFieldValue(t *testing.T) {
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:   "i-abc123",
		Name: "web-server",
		Fields: map[string]string{
			"InstanceId": "i-abc123",
			"VpcId":      "vpc-123",
		},
	}

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     res,
	})

	_, cmd := rootApplyMsg(m, rootKeyPress("c"))
	if cmd == nil {
		t.Fatal("pressing 'c' in detail view should return a command")
	}

	msg := cmd()
	switch v := msg.(type) {
	case messages.Flash:
		if !strings.Contains(v.Text, "i-abc123") && !strings.HasPrefix(v.Text, "Copy failed:") {
			t.Errorf("flash should mention copied field value, got %q", v.Text)
		}
	default:
		t.Errorf("expected FlashMsg, got %T", msg)
	}
}

func TestWiring_EscapeOnFocusedRightColumn_StaysInDetail(t *testing.T) {
	replaceEC2Related(t, []resource.RelatedDef{
		{TargetType: "tg", DisplayName: "Target Groups", Checker: noopChecker},
	})

	m := newRootSizedModel()
	res := &resource.Resource{
		ID:   "i-abc123",
		Name: "web-server",
		Fields: map[string]string{
			"InstanceId": "i-abc123",
		},
	}

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     res,
	})

	beforeTab := m.View().Content
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyTab))
	afterTab := m.View().Content
	if beforeTab == afterTab {
		t.Skip("right column did not focus via Tab; skipping root Esc wiring check")
	}

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	afterEsc := m.View().Content

	if !strings.Contains(afterEsc, "web-server") && !strings.Contains(afterEsc, "i-abc123") {
		t.Errorf("Esc on focused right column should stay in detail view, got: %s", afterEsc)
	}
}

func TestWiring_CopyInYAMLView_ReturnsFlashMsg(t *testing.T) {
	m := newRootSizedModel()

	res := &resource.Resource{
		ID:     "i-abc123",
		Name:   "web-server",
		Fields: map[string]string{"instance_id": "i-abc123", "name": "web-server"},
	}

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetYAML,
		Resource: res,
	})

	_, cmd := rootApplyMsg(m, rootKeyPress("c"))

	if cmd == nil {
		t.Fatal("pressing 'c' in YAML view should return a command for clipboard copy")
	}

	msg := cmd()
	switch msg.(type) {
	case messages.Flash:
		// OK
	default:
		t.Errorf("expected FlashMsg, got %T", msg)
	}
}

func TestWiring_CopyInRevealView_ReturnsFlashMsg(t *testing.T) {
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.ValueRevealed{
		ResourceID: "my-secret",
		Value:      "s3cr3t-value",
	})

	_, cmd := rootApplyMsg(m, rootKeyPress("c"))

	if cmd == nil {
		t.Fatal("pressing 'c' in reveal view should return a command for clipboard copy")
	}

	msg := cmd()
	switch msg.(type) {
	case messages.Flash:
		// OK
	default:
		t.Errorf("expected FlashMsg, got %T", msg)
	}
}

func TestWiring_RefreshInResourceList_ReturnsFetchCmd(t *testing.T) {
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	if cmd == nil {
		t.Fatal("pressing ctrl+r in resource list should return a command for fetching resources")
	}

	msg := cmd()
	switch msg.(type) {
	case messages.APIError:
		// Expected: no clients initialized
	case messages.ResourcesLoaded:
		// Would happen if clients were set
	case messages.Flash:
		// Refreshing flash is also OK
	default:
		t.Errorf("expected APIErrorMsg or ResourcesLoadedMsg, got %T", msg)
	}
}

func TestWiring_RefreshOnMainMenu_TriggersAvailabilityCheck(t *testing.T) {
	m := newRootSizedModel()

	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	if cmd == nil {
		t.Error("pressing ctrl+r on main menu should trigger availability cache reload command")
	}
}

func TestWiring_RefreshOnMainMenu_NoCacheMode_NoOp(t *testing.T) {
	m := newBlessedModel(t, "testprofile", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	if cmd != nil {
		t.Error("pressing ctrl+r on main menu in no-cache mode should not trigger any command")
	}
}

// ClientsReady in demo mode triggers availability probes (via
// loadAvailabilityCache).

func TestWiring_ClientsReady_DemoMode_TriggersAvailabilityProbes(t *testing.T) {
	m := newBlessedModel(t, "demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Gen:1 — ConnectGen seeds at 1 (session.New()); this model is never rotated.
	_, cmd := rootApplyMsg(m, messages.ClientsReady{Gen: 1})

	if cmd == nil {
		t.Fatal("ClientsReadyMsg in demo mode should return non-nil cmd (identity + availability probes)")
	}

	found := extractMsg(t, cmd, func(msg tea.Msg) bool {
		_, ok := msg.(messages.AvailabilityCacheLoaded)
		return ok
	})
	if found == nil {
		t.Error("ClientsReadyMsg in demo mode should produce AvailabilityCacheLoadedMsg from loadAvailabilityCache")
	}
}

func TestWiring_ClientsReady_DemoMode_NoCache_SkipsAvailability(t *testing.T) {
	m := newBlessedModel(t, "demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Send ClientsReadyMsg — with --no-cache, should only produce identity, NOT
	// availability. Gen:1 — ConnectGen seeds at 1 (session.New()); this model
	// is never rotated.
	_, cmd := rootApplyMsg(m, messages.ClientsReady{Gen: 1})

	if cmd == nil {
		t.Fatal("ClientsReadyMsg in demo+no-cache should still return identity cmd")
	}

	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, subCmd := range batch {
			if subCmd == nil {
				continue
			}
			subMsg := subCmd()
			if _, isAvail := subMsg.(messages.AvailabilityCacheLoaded); isAvail {
				t.Error("ClientsReadyMsg in demo+no-cache mode should NOT produce AvailabilityCacheLoadedMsg")
			}
		}
	} else {
		if _, isAvail := msg.(messages.AvailabilityCacheLoaded); isAvail {
			t.Error("ClientsReadyMsg in demo+no-cache mode should NOT produce AvailabilityCacheLoadedMsg")
		}
	}
}

// TestWiring_AvailabilityComplete_ClearsFlash walks the full probe cycle
// after ClientsReady: a disk-cache load with nil clients dequeues no probes
// upfront — it latches Session.AvailSweepPending instead, and
// HandleClientsReady's drain (fireNextAvailabilityProbes(4)) is what
// dequeues the first batch. Without a ClientsReady in between, the queue
// starts at len(AllShortNames()) undiminished, so exactly
// len(AllShortNames()) AvailabilityCheckedMsg only reaches AvailChecked==
// AvailTotal with the queue NOT yet empty (each message that finds
// len(AvailQueue)>0 dequeues one more and returns before reaching the
// ClearFlash branch — see handleAvailabilityChecked's queue-then-total
// check order in core/runtime/handlers_availability.go) — one message
// short of ever emitting ClearFlash. Sending ClientsReady first (dequeuing
// 4) is the real production sequence: the cache-loaded handler dispatches
// nothing, ClientsReady dequeues 4, and exactly len(AllShortNames())
// AvailabilityCheckedMsg then drains the remaining len(AllShortNames())-4
// queue entries plus reaches the terminal branch.
func TestWiring_AvailabilityComplete_ClearsFlash(t *testing.T) {
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Flash{Text: "Refreshing availability...", IsError: false})

	rendered := stripANSI(rootViewContent(m))
	if !strings.Contains(rendered, "Refreshing availability...") {
		t.Fatal("flash should be visible before availability cycle")
	}

	// Send AvailabilityCacheLoadedMsg to build the queue. With nil clients
	// (this model's state), no probes are dispatched yet — AvailSweepPending
	// is latched instead.
	m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoaded{
		Entries: make(map[string]int),
	})

	// ClientsReady drains the latched sweep's first batch of 4 — mirroring
	// the real boot sequence where the disk-cache load races ahead of the
	// AWS connect. Gen:1 — ConnectGen seeds at 1 (session.New()); this model
	// is never rotated.
	m, _ = rootApplyMsg(m, messages.ClientsReady{Gen: 1})

	// Now drain the queue by sending AvailabilityCheckedMsg for all resource
	// types. The queue was built from AllShortNames() minus the 4 ClientsReady
	// dequeued. Each AvailabilityCheckedMsg dequeues one more. So we need to
	// send len(AllShortNames()) messages total to drain everything.
	allNames := resource.AllShortNames()
	var lastCmd tea.Cmd
	// session.New seeds AvailabilityGen=1 — capture and reuse so the
	// AvailabilityChecked stale guard (AcceptZeroGen=false) accepts each msg.
	gen := m.Core().Session().AvailabilityGen
	for _, name := range allNames {
		m, lastCmd = rootApplyMsg(m, messages.AvailabilityChecked{
			ResourceType: name,
			HasResources: true,
			Count:        1,
			Gen:          gen,
		})
	}

	// (saveAvailabilityCache).
	if lastCmd == nil {
		t.Error("last AvailabilityCheckedMsg should return non-nil cmd (saveCache)")
	}

	rendered = stripANSI(rootViewContent(m))
	if strings.Contains(rendered, "Refreshing availability...") {
		t.Error("flash should be cleared after all availability checks complete")
	}
}

// After ClientsReady, ctrl+r on the main menu in demo mode triggers
// availability probes.

func TestWiring_RefreshOnMainMenu_DemoMode_TriggersProbes(t *testing.T) {
	m := newBlessedModel(t, "demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// First send ClientsReadyMsg so probes can run. Gen:1 — ConnectGen seeds
	// at 1 (session.New()); this model is never rotated.
	m, _ = rootApplyMsg(m, messages.ClientsReady{Gen: 1})

	_, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	if cmd == nil {
		t.Error("pressing ctrl+r on main menu in demo mode should trigger availability probes")
	}
}

// The demo probe count matches what the user sees.

func TestWiring_DemoMode_ProbeCount_MatchesPaginatedPageSize(t *testing.T) {
	// ec2 always has typed-fake fixtures.
	ec2Client := fakes.NewEC2()
	ec2Res, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(context.Background(), ec2Client, token)
	})
	if err != nil || len(ec2Res) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2Res))
	}
	targetType := "ec2"
	totalCount := len(ec2Res)

	// demo.NewServiceClients() gives the probe the same typed-fake data
	// FetchEC2Instances returned above.
	m := newBlessedModel(t, "demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	// Gen:1 — ConnectGen seeds at 1 (session.New()); this model is never rotated.
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: demo.NewServiceClients(), Gen: 1})

	// In demo mode loadAvailabilityCache finds no cache file, so the message is
	// sent directly to start probes.
	m, cmd := rootApplyMsg(m, messages.AvailabilityCacheLoaded{
		Entries: make(map[string]int),
	})

	type probeResult struct {
		Count     int
		Truncated bool
	}
	collected := make(map[string]probeResult)
	for cmd != nil {
		msg := cmd()
		if acm, ok := msg.(messages.AvailabilityChecked); ok {
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
				if acm, ok := subMsg.(messages.AvailabilityChecked); ok {
					collected[acm.ResourceType] = probeResult{Count: acm.Count, Truncated: acm.Truncated}
					m, cmd = rootApplyMsg(m, acm)
				}
			}
			continue
		}
		break
	}

	// The real fetcher returns every fixture, so the probe count equals the total.
	result, found := collected[targetType]
	if !found {
		t.Fatalf("probe cycle did not produce AvailabilityCheckedMsg for %s", targetType)
	}
	if result.Count != totalCount {
		t.Errorf("demo probe for %s reported count=%d, want %d (total fixtures)",
			targetType, result.Count, totalCount)
	}
}

func TestWiring_RevealForSecrets_ReturnsFetchCmd(t *testing.T) {
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "secrets",
	})

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "secrets",
		Resources: []resource.Resource{
			{ID: "my-secret", Name: "my-secret", Fields: map[string]string{"secret_name": "my-secret"}},
		},
	})

	_, cmd := rootApplyMsg(m, rootKeyPress("x"))

	if cmd == nil {
		t.Fatal("pressing 'x' on secrets resource list should return a reveal fetch command")
	}

	msg := cmd()
	switch msg.(type) {
	case messages.Flash:
		// Expected: no clients initialized
	case messages.ValueRevealed:
		// Would happen if clients were set
	default:
		t.Errorf("expected FlashMsg or ValueRevealedMsg, got %T", msg)
	}
}

func TestWiring_RevealNotForNonSecrets(t *testing.T) {
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-abc123", Name: "web-server", Fields: map[string]string{"instance_id": "i-abc123"}},
		},
	})

	_, cmd := rootApplyMsg(m, rootKeyPress("x"))

	if cmd != nil {
		msg := cmd()
		switch msg := msg.(type) {
		case messages.ValueRevealed:
			t.Error("pressing 'x' on non-secrets resource should not trigger reveal")
		case messages.Navigate:
			if msg.Target == messages.TargetReveal {
				t.Error("pressing 'x' on non-secrets resource should not navigate to reveal")
			}
		}
	}
}

func TestWiring_EmptyProfileShowsDefaultInHeader(t *testing.T) {
	// When no profile is specified (empty string), the header should show "default"
	m := newBlessedModel(t, "", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

	// Simulate AWS connection completing. Gen:1 — ConnectGen seeds at 1
	// (session.New()); this model is never rotated.
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: nil, Err: nil, Gen: 1})

	rendered := stripANSI(rootViewContent(m))
	if !strings.Contains(rendered, "default") {
		t.Errorf("header should show 'default' when profile is empty, got: %s", rendered)
	}
}

func TestWiring_ViewConfigLoadedOnClientsReady(t *testing.T) {
	m := newRootSizedModel()

	// Send ClientsReadyMsg — viewConfig should be loaded. Gen:1 — ConnectGen
	// seeds at 1 (session.New()); this model is never rotated.
	m, _ = rootApplyMsg(m, messages.ClientsReady{Clients: nil, Err: nil, Gen: 1})

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

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
		m, _ = rootApplyMsg(m, msg)
	}

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	plain := stripANSI(rootViewContent(m))
	if plain == "" {
		t.Error("should render resource list view even without views.yaml file")
	}
}

func TestWiring_ValueRevealedMsg_PushesRevealView(t *testing.T) {
	tui.Version = "1.0.0"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.ValueRevealed{
		ResourceID: "prod/db-password",
		Value:      "hunter2",
	})

	plain := stripANSI(rootViewContent(m))
	if plain == "" {
		t.Error("should render reveal view")
	}
	if !containsSubstring(plain, "prod/db-password") {
		t.Errorf("reveal view should show secret name in frame title, got: %s", truncateForLog(plain))
	}
}

func TestWiring_ValueRevealedMsg_Error(t *testing.T) {
	tui.Version = "1.0.0"
	m := newRootSizedModel()

	m, cmd := rootApplyMsg(m, messages.ValueRevealed{
		Err: errForTest("access denied"),
	})
	// The handler returns a Flash command; dispatch it.
	if cmd != nil {
		m, _ = rootApplyMsg(m, cmd())
	}

	plain := stripANSI(rootViewContent(m))
	// The flash is "reveal: <cause>"; what this test is about — the failure
	// reaches the rendered header — does not depend on the wording.
	if !containsSubstring(plain, "reveal: ") {
		t.Errorf("should show error flash for reveal failure, got: %s", truncateForLog(plain))
	}
}

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
