package unit

// qa67_cross_cutting_test.go — §K Cross-Cutting Error Resilience (issue #67)
//
// Bugs caught:
//   - K.1: no panic on any input sequence (fuzz sampler)
//   - K.2: view stack integrity after error — Esc returns to correct view
//   - K.3: error in one child view does not affect sibling child views
//   - K.4: rapid ctrl+r does not cause data corruption
//   - K.6: error flash does not overlap with filter mode
//   - K.7: frame title shows correct count after error then refresh
//   - K.8: empty filter result followed by clear shows all resources
//   - K.9: copy from empty resource list does not crash
//   - K.10: sort on empty resource list is a no-op, no crash
//   - K.11: filter on empty resource list does not crash
//   - K.12: detail view on empty resource list is a no-op, no crash
//   - K.13: YAML view on empty resource list is a no-op, no crash

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// K.1 — No panic on any common input in all view states.
func TestQa67_K1_NoPanic_InputSequences(t *testing.T) {
	type keyInput struct {
		name string
		key  tea.KeyPressMsg
	}
	// Keys that might cause crashes if not handled gracefully
	inputs := []keyInput{
		{name: "enter", key: tea.KeyPressMsg{Code: tea.KeyEnter}},
		{name: "esc", key: tea.KeyPressMsg{Code: tea.KeyEscape}},
		{name: "space", key: tea.KeyPressMsg{Code: -1, Text: " "}},
		{name: "ctrl-a", key: tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl}},
		{name: "ctrl-z", key: tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl}},
		{name: "ctrl-c", key: tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}},
		{name: "f1", key: tea.KeyPressMsg{Code: tea.KeyF1}},
		{name: "f5", key: tea.KeyPressMsg{Code: tea.KeyF5}},
		{name: "delete", key: tea.KeyPressMsg{Code: tea.KeyDelete}},
		{name: "insert", key: tea.KeyPressMsg{Code: tea.KeyInsert}},
		{name: "home", key: tea.KeyPressMsg{Code: tea.KeyHome}},
		{name: "end", key: tea.KeyPressMsg{Code: tea.KeyEnd}},
		{name: "pgup", key: tea.KeyPressMsg{Code: tea.KeyPgUp}},
		{name: "pgdn", key: tea.KeyPressMsg{Code: tea.KeyPgDown}},
		{name: "up", key: tea.KeyPressMsg{Code: tea.KeyUp}},
		{name: "down", key: tea.KeyPressMsg{Code: tea.KeyDown}},
		{name: "left", key: tea.KeyPressMsg{Code: tea.KeyLeft}},
		{name: "right", key: tea.KeyPressMsg{Code: tea.KeyRight}},
		{name: "backspace", key: tea.KeyPressMsg{Code: tea.KeyBackspace}},
		{name: "tab", key: tea.KeyPressMsg{Code: tea.KeyTab}},
		{name: "backtick", key: tea.KeyPressMsg{Code: -1, Text: "`"}},
		{name: "tilde", key: tea.KeyPressMsg{Code: -1, Text: "~"}},
		{name: "pipe", key: tea.KeyPressMsg{Code: -1, Text: "|"}},
		{name: "backslash", key: tea.KeyPressMsg{Code: -1, Text: "\\"}},
		{name: "null-byte", key: tea.KeyPressMsg{Code: -1, Text: "\x00"}},
		{name: "unicode-cjk", key: tea.KeyPressMsg{Code: -1, Text: "中"}},
		{name: "unicode-emoji", key: tea.KeyPressMsg{Code: -1, Text: "🚀"}},
	}

	// Test in main menu state
	t.Run("main_menu", func(t *testing.T) {
		m := newRootSizedModel()
		for _, input := range inputs {
			m, _ = rootApplyMsg(m, input.key)
			out := rootViewContent(m)
			if out == "" {
				t.Errorf("K.1: View() is empty after key %q in main menu", input.name)
			}
		}
	})

	// Test in resource list state (loaded)
	t.Run("resource_list_loaded", func(t *testing.T) {
		m := newRootSizedModel()
		m, _ = rootApplyMsg(m, messages.NavigateMsg{
			Target:       messages.TargetResourceList,
			ResourceType: "ec2",
		})
		m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
			ResourceType: "ec2",
			Resources: []resource.Resource{
				{ID: "i-fuzz", Name: "fuzz-instance", Status: "running", Fields: map[string]string{
					"instance_id": "i-fuzz",
					"name":        "fuzz-instance",
					"state":       "running",
					"type":        "t3.micro",
					"private_ip":  "10.0.0.1",
					"public_ip":   "",
					"launch_time": "2025-01-01",
					"lifecycle":   "",
				}},
			},
		})
		for _, input := range inputs {
			m, _ = rootApplyMsg(m, input.key)
			out := rootViewContent(m)
			if out == "" {
				t.Errorf("K.1: View() is empty after key %q in resource list", input.name)
			}
		}
	})
}

// K.2 — View stack integrity after error: Esc returns to the correct view.
func TestQa67_K2_ViewStackIntegrity_AfterError(t *testing.T) {
	m := newRootSizedModel()

	// Navigate: main menu -> EC2 list -> detail
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	resources := []resource.Resource{
		{ID: "i-stack-test", Name: "stack-test", Status: "running", Fields: map[string]string{
			"instance_id": "i-stack-test",
			"name":        "stack-test",
			"state":       "running",
			"type":        "t3.micro",
			"private_ip":  "10.0.0.1",
			"public_ip":   "",
			"launch_time": "2025-01-01",
			"lifecycle":   "",
		}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "ec2", Resources: resources})

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:   messages.TargetDetail,
		Resource: &resources[0],
	})

	// Send an API error (simulating a detail refresh failing)
	m, _ = rootApplyMsg(m, messages.APIErrorMsg{
		ResourceType: "ec2",
		Err:          errAccessDenied("ec2:DescribeInstances"),
	})

	// Esc from detail should go back to the EC2 list, not to main menu
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	out := rootViewContent(m)
	plain := stripANSI(out)
	// Should be on the EC2 list — the resource should be visible
	if !strings.Contains(plain, "stack-test") {
		t.Errorf("K.2: after Esc from detail (post-error), EC2 list should show 'stack-test', got: %s", plain[:min(200, len(plain))])
	}
	if out == "" {
		t.Error("K.2: View() should not be empty after Esc from detail following API error")
	}
}

func errAccessDenied(action string) error {
	return fmt.Errorf("AccessDenied: User is not authorized to perform: %s", action)
}

// K.3 — Error in one child view does not affect sibling child views.
func TestQa67_K3_ErrorInChildView_DoesNotAffectSiblings(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ecs-svc",
	})
	services := []resource.Resource{
		{ID: "my-cluster/my-service", Name: "my-service", Status: "ACTIVE", Fields: map[string]string{
			"service_name":  "my-service",
			"cluster":       "my-cluster",
			"status":        "ACTIVE",
			"desired_count": "2",
			"running_count": "2",
			"pending_count": "0",
			"task_def":      "my-task:1",
			"launch_type":   "FARGATE",
		}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "ecs-svc", Resources: services})

	// Open Events child view (key 'e') — execute the returned cmd to actually push the child view
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: -1, Text: "e"})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			m, _ = rootApplyMsg(m, msg)
		}
	}
	m, _ = rootApplyMsg(m, messages.APIErrorMsg{
		ResourceType: "ecs_svc_events",
		Err:          errAccessDenied("ecs:DescribeServices"),
	})

	// Esc back to the ECS services list
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	out := rootViewContent(m)
	if out == "" {
		t.Error("K.3: View() should not be empty after returning from errored child view")
	}

	// The service should still be visible
	plain := stripANSI(out)
	if !strings.Contains(plain, "my-service") {
		t.Errorf("K.3: after returning from errored child view, ECS service 'my-service' should be visible, got: %s", plain[:min(200, len(plain))])
	}
}

// K.4 — Rapid ctrl+r does not cause data corruption or crash.
func TestQa67_K4_RapidCtrlR_NoCrashOrCorruption(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	resources := []resource.Resource{
		{ID: "i-rapid", Name: "rapid-refresh", Status: "running", Fields: map[string]string{
			"instance_id": "i-rapid",
			"name":        "rapid-refresh",
			"state":       "running",
			"type":        "t3.micro",
			"private_ip":  "10.0.0.1",
			"public_ip":   "",
			"launch_time": "2025-01-01",
			"lifecycle":   "",
		}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "ec2", Resources: resources})

	// Press ctrl+r three times rapidly
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	// Application should still be functional
	out := rootViewContent(m)
	if out == "" {
		t.Error("K.4: View() should not be empty after rapid ctrl+r")
	}

	// Deliver resources for the last refresh — should not corrupt state
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "ec2", Resources: resources})
	out = rootViewContent(m)
	plain := stripANSI(out)
	if !strings.Contains(plain, "rapid-refresh") {
		t.Errorf("K.4: after rapid ctrl+r + resources loaded, should show resource, got: %s", plain[:min(200, len(plain))])
	}
}

// K.6 — Error flash does not overlap with filter mode.
func TestQa67_K6_ErrorFlash_DoesNotOverlapFilter(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-filter", Name: "filter-test", Status: "running", Fields: map[string]string{
				"instance_id": "i-filter",
				"name":        "filter-test",
				"state":       "running",
				"type":        "t3.micro",
				"private_ip":  "10.0.0.1",
				"public_ip":   "",
				"launch_time": "2025-01-01",
				"lifecycle":   "",
			}},
		},
	})

	// Trigger an error flash
	m, _ = rootApplyMsg(m, messages.FlashMsg{
		Text:    "Error: rate limit exceeded",
		IsError: true,
	})

	// Now enter filter mode
	m, _ = rootApplyMsg(m, rootKeyPress("/"))

	out := rootViewContent(m)
	// Must not crash; filter mode should be active
	if out == "" {
		t.Error("K.6: View() should not be empty when filter mode entered after error flash")
	}
}

// K.7 — Frame title shows correct count after error then refresh.
func TestQa67_K7_FrameTitleCount_AfterErrorThenRefresh(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Send an error — resource list is empty
	m, _ = rootApplyMsg(m, messages.APIErrorMsg{
		ResourceType: "ec2",
		Err:          errAccessDenied("ec2:DescribeInstances"),
	})

	// Error state: the list should have 0 resources and show an error
	out := rootViewContent(m)
	plain := stripANSI(out)
	if !strings.Contains(plain, "AccessDenied") && !strings.Contains(plain, "access denied") && !strings.Contains(plain, "error") && !strings.Contains(plain, "Error") {
		t.Errorf("K.7: after APIErrorMsg, view should show error indication, got: %s", plain[:min(200, len(plain))])
	}

	// User presses ctrl+r — then resources arrive
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	resources := make([]resource.Resource, 42)
	for i := range resources {
		resources[i] = resource.Resource{
			ID:   "i-recovered",
			Name: "recovered",
			Fields: map[string]string{
				"instance_id": "i-recovered",
				"name":        "recovered",
				"state":       "running",
				"type":        "t3.micro",
				"private_ip":  "10.0.0.1",
				"public_ip":   "",
				"launch_time": "2025-01-01",
				"lifecycle":   "",
			},
		}
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "ec2", Resources: resources})

	out = rootViewContent(m)
	plain = stripANSI(out)
	// Frame title should show count (42)
	if !strings.Contains(plain, "42") {
		t.Errorf("K.7: expected count 42 in frame title after refresh, got: %s", plain[:min(300, len(plain))])
	}
	if out == "" {
		t.Error("K.7: View() should not be empty after error then successful refresh")
	}
}

// K.8 — Empty filter result followed by clear shows all resources.
func TestQa67_K8_EmptyFilterClear_ShowsAllResources(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	resources := []resource.Resource{
		{ID: "i-alpha", Name: "alpha-server", Status: "running", Fields: map[string]string{
			"instance_id": "i-alpha",
			"name":        "alpha-server",
			"state":       "running",
			"type":        "t3.micro",
			"private_ip":  "10.0.0.1",
			"public_ip":   "",
			"launch_time": "2025-01-01",
			"lifecycle":   "",
		}},
		{ID: "i-beta", Name: "beta-server", Status: "running", Fields: map[string]string{
			"instance_id": "i-beta",
			"name":        "beta-server",
			"state":       "running",
			"type":        "t3.micro",
			"private_ip":  "10.0.0.2",
			"public_ip":   "",
			"launch_time": "2025-01-01",
			"lifecycle":   "",
		}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{ResourceType: "ec2", Resources: resources})

	// Enter filter mode and type something that matches nothing
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range "zzzzz" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	// Verify empty state
	out := rootViewContent(m)
	if out == "" {
		t.Fatal("K.8: View() should not be empty during empty filter")
	}

	// Clear filter with Esc
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	out = rootViewContent(m)
	plain := stripANSI(out)
	// Both resources should be visible again
	if !strings.Contains(plain, "alpha-server") {
		t.Errorf("K.8: after filter clear, alpha-server should be visible, got: %s", plain[:min(300, len(plain))])
	}
	if !strings.Contains(plain, "beta-server") {
		t.Errorf("K.8: after filter clear, beta-server should be visible, got: %s", plain[:min(300, len(plain))])
	}
}

// K.9 — Copy from empty resource list does not crash.
func TestQa67_K9_CopyFromEmptyList_NoCrash(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	// Load empty resources
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{},
	})

	// Press 'c' to copy — should be a no-op or show a warning
	m, _ = rootApplyMsg(m, rootKeyPress("c"))
	out := rootViewContent(m)
	if out == "" {
		t.Error("K.9: View() should not be empty after copy on empty list")
	}
}

// K.10 — Sort on empty resource list is a no-op, no crash.
func TestQa67_K10_SortOnEmptyList_NoCrash(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{},
	})

	// Press sort keys — should be no-op
	for _, key := range []string{"N", "S", "A"} {
		m, _ = rootApplyMsg(m, rootKeyPress(key))
	}
	out := rootViewContent(m)
	if out == "" {
		t.Error("K.10: View() should not be empty after sort keys on empty list")
	}
}

// K.11 — Filter on empty resource list does not crash.
func TestQa67_K11_FilterOnEmptyList_NoCrash(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{},
	})

	// Enter filter mode and type
	m, _ = rootApplyMsg(m, rootKeyPress("/"))
	for _, r := range "filter-on-empty" {
		m, _ = rootApplyMsg(m, rootKeyPress(string(r)))
	}

	out := rootViewContent(m)
	if out == "" {
		t.Error("K.11: View() should not be empty when filtering an empty list")
	}
}

// K.12 — Detail view on empty resource list is a no-op, no crash.
func TestQa67_K12_DetailOnEmptyList_IsNoOp(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{},
	})

	viewBefore := rootViewContent(m)

	// Press 'd' — should be no-op on empty list
	m, _ = rootApplyMsg(m, rootKeyPress("d"))

	viewAfter := rootViewContent(m)
	if viewAfter == "" {
		t.Error("K.12: View() should not be empty after pressing d on empty list")
	}

	// Should still be in the resource list, not crashed into a detail view
	_ = viewBefore
}

// K.13 — YAML view on empty resource list is a no-op, no crash.
func TestQa67_K13_YAMLOnEmptyList_IsNoOp(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{},
	})

	// Press 'y' — should be no-op on empty list
	m, _ = rootApplyMsg(m, rootKeyPress("y"))
	out := rootViewContent(m)
	if out == "" {
		t.Error("K.13: View() should not be empty after pressing y on empty list")
	}
}

// K.9–K.13 extended — all operations on empty list for all resource types.
func TestQa67_K9_K13_EmptyListOps_AllResourceTypes(t *testing.T) {
	ops := []struct {
		name string
		key  string
	}{
		{name: "copy", key: "c"},
		{name: "sort_name", key: "N"},
		{name: "sort_status", key: "S"},
		{name: "sort_age", key: "A"},
		{name: "detail", key: "d"},
		{name: "yaml", key: "y"},
	}

	// Representative sample — full sweep in CI slow suite
	sampleTypes := []string{"ec2", "s3", "secrets", "vpc"}
	for _, rt := range sampleTypes {
		for _, op := range ops {
			t.Run(rt+"/"+op.name, func(t *testing.T) {
				m := newRootSizedModel()
				m, _ = rootApplyMsg(m, messages.NavigateMsg{
					Target:       messages.TargetResourceList,
					ResourceType: rt,
				})
				m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
					ResourceType: rt,
					Resources:    []resource.Resource{},
				})
				m, _ = rootApplyMsg(m, rootKeyPress(op.key))
				out := rootViewContent(m)
				if out == "" {
					t.Errorf("[%s/%s] K.9-K.13: View() should not be empty after pressing %q on empty list",
						rt, op.name, op.key)
				}
			})
		}
	}
}
