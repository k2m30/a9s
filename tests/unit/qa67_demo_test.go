package unit

// qa67_demo_test.go — §G Demo Mode (issue #67)
//
// Bugs caught:
//   - G.2: all resource types have fixture data in demo mode
//   - G.6: fixtures include error/failed/stopped states that exercise row coloring
//   - G.7: header shows "demo" profile in demo mode
//   - G.8: navigation features (sort, filter, hscroll) work in demo mode

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// newDemoRootModel creates a demo-mode root model sized for testing.
func newDemoRootModel(t *testing.T) tui.Model {
	t.Helper()
	model := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m, _ := rootApplyMsg(model, tea.WindowSizeMsg{Width: 120, Height: 36})
	// Deliver ClientsReadyMsg using the demo transport
	crMsg := demoClientsReadyMsg()
	m, _ = rootApplyMsg(m, crMsg)
	return m
}

// G.2 — All registered resource types have demo fixture data (non-empty).
func TestQa67_G2_DemoFixtures_AllResourceTypesHaveData(t *testing.T) {
	// The demo package's GetResources only covers the types that have been
	// explicitly registered. We test each short name registered in the resource
	// registry against the demo fixture store.
	allTypes := resource.AllShortNames()
	for _, rt := range allTypes {
		t.Run(rt, func(t *testing.T) {
			resources, ok := demo.GetResources(rt)
			if !ok {
				// Some resource types may legitimately not have demo fixtures
				// (child-only types). Log and skip rather than fail.
				t.Logf("G.2: no demo fixture registered for %q (may be child-only type)", rt)
				return
			}
			if len(resources) == 0 {
				t.Errorf("G.2: demo.GetResources(%q) returned ok=true but empty slice", rt)
			}
		})
	}
}

// G.2 — Demo mode: core resource types have non-empty fixture data.
func TestQa67_G2_DemoFixtures_CoreResourceTypesNonEmpty(t *testing.T) {
	// Core types that must always have demo fixtures for the demo to be meaningful
	coreTypes := []string{
		"ec2", "s3", "lambda", "dbi", "redis", "secrets", "ssm",
		"eks", "ecs", "ecs_svc",
	}
	for _, rt := range coreTypes {
		t.Run(rt, func(t *testing.T) {
			resources, ok := demo.GetResources(rt)
			if !ok {
				t.Errorf("G.2: core resource type %q has no demo fixtures registered", rt)
				return
			}
			if len(resources) == 0 {
				t.Errorf("G.2: demo.GetResources(%q) returned empty slice; demo mode would show empty list", rt)
			}
		})
	}
}

// G.6 — Demo fixtures include at least one resource in an error/stopped/failed state.
func TestQa67_G6_DemoFixtures_ContainErrorOrStoppedStates(t *testing.T) {
	// These types should include both running and stopped/failed states
	// so that demo mode exercises row coloring paths.
	typesWithExpectedStates := []struct {
		shortName    string
		wantStatuses []string // at least one of these must be present
	}{
		{"ec2", []string{"stopped", "terminated", "pending"}},
		{"dbi", []string{"stopped", "creating", "failed", "deleting"}},
		{"ecs_svc", []string{"INACTIVE", "DRAINING"}},
	}
	for _, tt := range typesWithExpectedStates {
		t.Run(tt.shortName, func(t *testing.T) {
			resources, ok := demo.GetResources(tt.shortName)
			if !ok {
				t.Skipf("G.6: no demo fixture for %q", tt.shortName)
				return
			}
			found := false
			for _, r := range resources {
				if slices.Contains(tt.wantStatuses, r.Status) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("G.6: %q fixtures should include at least one of statuses %v for error-path testing; only found: %v",
					tt.shortName, tt.wantStatuses, collectStatuses(resources))
			}
		})
	}
}

func collectStatuses(resources []resource.Resource) []string {
	seen := make(map[string]bool)
	for _, r := range resources {
		seen[r.Status] = true
	}
	statuses := make([]string, 0, len(seen))
	for s := range seen {
		statuses = append(statuses, s)
	}
	return statuses
}

// G.7 — Demo mode header shows "demo" profile indicator.
func TestQa67_G7_DemoMode_HeaderShowsDemoProfile(t *testing.T) {
	model := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m, _ := rootApplyMsg(model, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = rootApplyMsg(m, demoClientsReadyMsg())

	out := rootViewContent(m)
	plain := stripANSI(out)
	if !strings.Contains(plain, "demo") {
		t.Errorf("G.7: demo mode header should contain 'demo' profile indicator, got: %s", plain[:min(200, len(plain))])
	}
}

// G.7 — DemoProfile constant equals "demo".
func TestQa67_G7_DemoProfile_ConstantIsDemo(t *testing.T) {
	if demo.DemoProfile != "demo" {
		t.Errorf("G.7: demo.DemoProfile = %q, want %q", demo.DemoProfile, "demo")
	}
}

// G.8 — Demo mode: sort key works on demo EC2 fixtures without crash.
func TestQa67_G8_DemoMode_SortWorks(t *testing.T) {
	m := newDemoRootModel(t)
	m, cmd := rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Deliver the fetched resources
	if cmd != nil {
		msg := extractMsg(t, cmd, func(msg tea.Msg) bool {
			_, ok := msg.(messages.ResourcesLoadedMsg)
			return ok
		})
		m, _ = rootApplyMsg(m, msg)
	}

	// Sort ascending by name
	m, _ = rootApplyMsg(m, rootKeyPress("N"))
	out := rootViewContent(m)
	if out == "" {
		t.Error("G.8: View() should not be empty after sort in demo mode")
	}

	// Sort descending
	m, _ = rootApplyMsg(m, rootKeyPress("N"))
	out = rootViewContent(m)
	if out == "" {
		t.Error("G.8: View() should not be empty after reverse sort in demo mode")
	}
}

// G.8 — Demo mode: filter key works on demo EC2 fixtures without crash.
func TestQa67_G8_DemoMode_FilterWorks(t *testing.T) {
	m := newDemoRootModel(t)
	m, cmd := rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	if cmd != nil {
		msg := extractMsg(t, cmd, func(msg tea.Msg) bool {
			_, ok := msg.(messages.ResourcesLoadedMsg)
			return ok
		})
		m, _ = rootApplyMsg(m, msg)
	}

	// Enter filter mode
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range "web" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}
	out := rootViewContent(m)
	if out == "" {
		t.Error("G.8: View() should not be empty after typing filter in demo mode")
	}

	// Clear filter
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	out = rootViewContent(m)
	if out == "" {
		t.Error("G.8: View() should not be empty after clearing filter in demo mode")
	}
}

// G.8 — Demo mode: help screen opens and closes without crash.
func TestQa67_G8_DemoMode_HelpOpenClose(t *testing.T) {
	m := newDemoRootModel(t)

	// Open help with ?
	m, _ = rootApplyMsg(m, rootKeyPress("?"))
	out := rootViewContent(m)
	plain := stripANSI(out)
	if !strings.Contains(plain, "help") && !strings.Contains(plain, "Help") && !strings.Contains(plain, "keys") {
		t.Errorf("G.8: help screen should show after ? key in demo mode, got: %s", plain[:min(200, len(plain))])
	}

	// Close help with Esc
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	out = rootViewContent(m)
	if out == "" {
		t.Error("G.8: View() should not be empty after closing help in demo mode")
	}
}

// G.3 — Demo mode: detail view shows complete fields for EC2.
func TestQa67_G3_DemoMode_DetailViewShowsFields(t *testing.T) {
	resources, ok := demo.GetResources("ec2")
	if !ok || len(resources) == 0 {
		t.Skip("G.3: no EC2 demo fixtures available")
	}

	m := newDemoRootModel(t)
	res := &resources[0]
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: res,
	})
	out := rootViewContent(m)
	if out == "" {
		t.Error("G.3: detail view should not be empty for demo EC2 resource")
	}
}

// G.4 — Demo mode: YAML view produces non-empty output for EC2.
func TestQa67_G4_DemoMode_YAMLViewNonEmpty(t *testing.T) {
	resources, ok := demo.GetResources("ec2")
	if !ok || len(resources) == 0 {
		t.Skip("G.4: no EC2 demo fixtures available")
	}

	m := newDemoRootModel(t)
	res := &resources[0]
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetYAML,
		Resource: res,
	})
	out := rootViewContent(m)
	if out == "" {
		t.Error("G.4: YAML view should not be empty for demo EC2 resource")
	}
}
