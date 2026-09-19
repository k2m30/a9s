package unit

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// A resource deleted between list load and detail open renders without panic
// from the last-known list data; no second AWS call is made.
func TestQa67_F3_ResourceDeletedBeforeDetailOpen_NoPanic(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	resources := []resource.Resource{
		{
			ID:   "i-deleted-later",
			Name: "soon-deleted",
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: resources})

	res := &resources[0]
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: res,
	})

	out := rootViewContent(m)
	if out == "" {
		t.Error("F.3: detail view should render with last-known data after resource is deleted")
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	out = rootViewContent(m)
	plain := stripANSI(out)
	if plain == "" {
		t.Error("F.3: after Esc from detail, should render the EC2 list")
	}
}

// An APIError for a child view (resource deleted) is shown and navigation
// still works.
func TestQa67_F4_ResourceDeletedBeforeChildView_ShowsError(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})
	buckets := []resource.Resource{
		{
			ID:   "deleted-bucket",
			Name: "deleted-bucket",
			Fields: map[string]string{
				"name":          "deleted-bucket",
				"region":        "us-east-1",
				"creation_date": "2025-01-01",
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "s3", Resources: buckets})

	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		_ = cmd() // returns APIErrorMsg in real scenario
	}

	m, _ = rootApplyMsg(m, messages.APIError{
		ResourceType: "s3_objects",
		Err:          errNoSuchBucket("deleted-bucket"),
	})

	out := rootViewContent(m)
	plain := stripANSI(out)
	if plain == "" {
		t.Error("F.4: View() should not be empty after APIErrorMsg for deleted bucket")
	}
	if strings.Contains(plain, "Loading...") {
		t.Error("F.4: after APIErrorMsg, should not still show Loading indicator")
	}
}

func errNoSuchBucket(bucket string) error {
	return fmt.Errorf("NoSuchBucket: the specified bucket does not exist: %s", bucket)
}

// Rapid Esc presses through nested views do not leave ghost views or panic.
func TestQa67_F6_RapidEscPresses_DoNotPanic(t *testing.T) {
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	resources := []resource.Resource{
		{
			ID:   "i-deep",
			Name: "deep-nav-instance",
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: resources})
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: &resources[0],
	})

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})
	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	out := rootViewContent(m)
	if out == "" {
		t.Error("F.6: View() should not be empty after rapid Esc presses")
	}
}

// Navigating away before ResourcesLoaded arrives does not corrupt the view.
func TestQa67_F7_EscDuringLoading_NavigatesBackCleanly(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	out := rootViewContent(m)
	plain := stripANSI(out)
	if !strings.Contains(plain, "Loading") {
		t.Errorf("F.7: loading state should show 'Loading' text, got: %s", plain[:min(200, len(plain))])
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	out = rootViewContent(m)
	if out == "" {
		t.Error("F.7: after Esc during loading, View() should not be empty")
	}

	// A late ResourcesLoaded must be discarded or benign.
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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
	out = rootViewContent(m)
	if out == "" {
		t.Error("F.7: after late ResourcesLoadedMsg, View() should not be empty")
	}
}

// Refresh after a state change shows the updated data.
func TestQa67_F5_RefreshAfterStateChange_ShowsUpdatedData(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	initial := []resource.Resource{
		{
			ID:   "i-state-change",
			Name: "changeable-instance",
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: initial, Provenance: messages.FetchProvenanceCanonicalList})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "running") {
		t.Fatalf("F.5: initial state should show 'running', got: %s", plain[:min(200, len(plain))])
	}

	m, _ = rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})

	updated := []resource.Resource{
		{
			ID:   "i-state-change",
			Name: "changeable-instance",
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
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{ResourceType: "ec2", Resources: updated, Provenance: messages.FetchProvenanceCanonicalList})

	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "stopped") {
		t.Errorf("F.5: after refresh with updated data, should show 'stopped' status, got: %s", plain[:min(200, len(plain))])
	}
}
