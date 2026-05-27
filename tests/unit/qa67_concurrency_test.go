package unit

// qa67_concurrency_test.go — §F Concurrency & Timing (issue #67)
//
// Bugs caught:
//   - F.3: resource deleted between list and detail — app must not crash
//   - F.4: resource deleted between list and child view — shows error, no crash
//   - F.6: rapid esc presses through deeply nested views do not leave ghost state
//   - F.7: navigating away during loading state cancels cleanly, no stale data

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// F.3 — Resource deleted between list load and detail open: app renders without panic.
// The detail view shows last-known data from the list (no second AWS call needed).
func TestQa67_F3_ResourceDeletedBeforeDetailOpen_NoPanic(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	resources := []resource.Resource{
		{
			ID:     "i-deleted-later",
			Name:   "soon-deleted",
			Fields: map[string]string{
				"instance_id": "i-deleted-later",
				"name":        "soon-deleted",
				"state":       "running",
				"type":        "t3.small",
				"private_ip":  "10.0.0.1",
				"public_ip":   "",
				"launch_time": "2025-01-01",
				"lifecycle":   "",
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: resources})

	// Simulate: resource "deleted" in AWS, but user pressed d to view detail
	// using the stale list data — this is the list-cached resource, no new API call.
	res := &resources[0]
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: res,
	})

	out := rootViewContent(m)
	// Must not crash; should show the stale cached data
	if out == "" {
		t.Error("F.3: detail view should render with last-known data after resource is deleted")
	}

	// Esc returns to the list
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	out = rootViewContent(m)
	plain := stripANSI(out)
	// Should be back at the EC2 list
	if plain == "" {
		t.Error("F.3: after Esc from detail, should render the EC2 list")
	}
}

// F.4 — When APIErrorMsg arrives for a child view (resource deleted), error is shown and nav works.
func TestQa67_F4_ResourceDeletedBeforeChildView_ShowsError(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})
	buckets := []resource.Resource{
		{
			ID:     "deleted-bucket",
			Name:   "deleted-bucket",
			Fields: map[string]string{
				"name":          "deleted-bucket",
				"region":        "us-east-1",
				"creation_date": "2025-01-01",
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "s3", Resources: buckets})

	// Navigate into the bucket (child view)
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		// Execute the child fetch cmd — it will fail since bucket is "deleted"
		_ = cmd() // returns APIErrorMsg in real scenario
	}

	// Simulate the APIErrorMsg from the child fetcher (bucket was deleted)
	m, _ = rootApplyMsg(m, messages.APIError{
		ResourceType: "s3_objects",
		Err:          errNoSuchBucket("deleted-bucket"),
	})

	out := rootViewContent(m)
	plain := stripANSI(out)
	// Should show an error indication, not crash
	if plain == "" {
		t.Error("F.4: View() should not be empty after APIErrorMsg for deleted bucket")
	}
	// Should NOT show "Loading"
	if strings.Contains(plain, "Loading...") {
		t.Error("F.4: after APIErrorMsg, should not still show Loading indicator")
	}
}

func errNoSuchBucket(bucket string) error {
	return fmt.Errorf("NoSuchBucket: the specified bucket does not exist: %s", bucket)
}

// F.6 — Rapid Esc presses through nested views do not leave ghost views or panic.
func TestQa67_F6_RapidEscPresses_DoNotPanic(t *testing.T) {
	m := newRootSizedModel()

	// Build up a view stack: menu -> list -> detail
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	resources := []resource.Resource{
		{
			ID:     "i-deep",
			Name:   "deep-nav-instance",
			Fields: map[string]string{
				"instance_id": "i-deep",
				"name":        "deep-nav-instance",
				"state":       "running",
				"type":        "t3.micro",
				"private_ip":  "10.0.1.1",
				"public_ip":   "",
				"launch_time": "2025-01-01",
				"lifecycle":   "",
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: resources})
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: &resources[0],
	})

	// Rapid Esc presses — simulate user pressing 3× quickly
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	out := rootViewContent(m)
	if out == "" {
		t.Error("F.6: View() should not be empty after rapid Esc presses")
	}
}

// F.7 — Navigating away during loading state (before ResourcesLoadedMsg) does not corrupt view.
func TestQa67_F7_EscDuringLoading_NavigatesBackCleanly(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Verify loading state
	out := rootViewContent(m)
	plain := stripANSI(out)
	if !strings.Contains(plain, "Loading") {
		t.Errorf("F.7: loading state should show 'Loading' text, got: %s", plain[:min(200, len(plain))])
	}

	// Esc before resources arrive — should return to main menu
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	out = rootViewContent(m)
	if out == "" {
		t.Error("F.7: after Esc during loading, View() should not be empty")
	}

	// Now if late ResourcesLoadedMsg arrives, it should be discarded or benign
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources: []resource.Resource{
			{ID: "i-late", Name: "late-instance", Fields: map[string]string{
				"instance_id": "i-late",
				"name":        "late-instance",
				"state":       "running",
				"type":        "t3.micro",
				"private_ip":  "10.0.0.1",
				"public_ip":   "",
				"launch_time": "2025-01-01",
				"lifecycle":   "",
			}},
		},
	})
	// App should still be functional
	out = rootViewContent(m)
	if out == "" {
		t.Error("F.7: after late ResourcesLoadedMsg, View() should not be empty")
	}
}

// F.5 — Refresh after state change shows updated data.
func TestQa67_F5_RefreshAfterStateChange_ShowsUpdatedData(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	// Load initial state: instance is running
	initial := []resource.Resource{
		{
			ID:     "i-state-change",
			Name:   "changeable-instance",
			Fields: map[string]string{
				"instance_id": "i-state-change",
				"name":        "changeable-instance",
				"state":       "running",
				"type":        "t3.small",
				"private_ip":  "10.0.0.7",
				"public_ip":   "",
				"launch_time": "2025-01-01",
				"lifecycle":   "",
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: initial})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "running") {
		t.Fatalf("F.5: initial state should show 'running', got: %s", plain[:min(200, len(plain))])
	}

	// Ctrl+R triggers refresh — then new data arrives with stopped status
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	updated := []resource.Resource{
		{
			ID:     "i-state-change",
			Name:   "changeable-instance",
			Fields: map[string]string{
				"instance_id": "i-state-change",
				"name":        "changeable-instance",
				"state":       "stopped",
				"type":        "t3.small",
				"private_ip":  "10.0.0.7",
				"public_ip":   "",
				"launch_time": "2025-01-01",
				"lifecycle":   "",
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: updated})

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "stopped") {
		t.Errorf("F.5: after refresh with updated data, should show 'stopped' status, got: %s", plain[:min(200, len(plain))])
	}
}
