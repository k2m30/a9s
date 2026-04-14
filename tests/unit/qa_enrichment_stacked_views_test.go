package unit

// qa_enrichment_stacked_views_test.go — RED tests for Bug 3: Enrichment completion
// updates ONLY the active view, leaving stacked views stale.
//
// Bug: In app_handlers_navigate.go, handleEnrichmentChecked (lines 638-650) only
// calls SetEnrichmentState/SetEnrichmentFinding on the ACTIVE view:
//
//	if rl, ok := m.activeView().(*views.ResourceListModel); ok && rl.ResourceType() == msg.ResourceType {
//	    rl.SetEnrichmentState(...)
//	}
//	if d, ok := m.activeView().(*views.DetailModel); ok && d.ResourceType() == msg.ResourceType {
//	    d.SetEnrichmentFinding(...)
//	}
//
// If the user has navigated from ResourceListModel (EC2 list) to DetailModel
// (detail view for one EC2 instance), and Wave 2 enrichment completes while the
// detail is active, only the DetailModel is updated. The ResourceListModel below it
// in m.stack keeps stale findingsByID. When the user presses Esc, the revealed
// ResourceListModel shows no markers or banner despite findings being available.
//
// Demanded behavior (post-fix): iterate m.stack and update ALL views of matching type.
//
// Tests T068–T069:
//   T068 — Wave 2 completes while DetailModel is active: stacked ResourceListModel
//           must reflect findings after pop.
//   T069 — Wave 2 completes while DetailModel-B is active: stacked DetailModel-A
//           must show "Background Check" section after pop-to-A.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// ─────────────────────────────────────────────────────────────────────────────
// T068 — EnrichmentCheckedMsg while Detail active → stacked ResourceList updated
// ─────────────────────────────────────────────────────────────────────────────

// TestEnrichment_UpdatesStackedResourceListWhenDetailActive verifies that when
// the user navigates from the EC2 list to a detail view, and Wave 2 enrichment
// completes while the detail is active, the ResourceListModel below the detail
// on the stack is updated with findings. After popping back to the list, row
// markers and banner must be visible.
//
// Pre-fix: only the DetailModel (active) is updated. ResourceListModel gets no
// SetEnrichmentState call. After pop, the list shows no markers or banner.
//
// Post-fix: handleEnrichmentChecked iterates m.stack and calls SetEnrichmentState
// on every *ResourceListModel whose ResourceType() matches the enrichment type.
func TestEnrichment_UpdatesStackedResourceListWhenDetailActive(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Step 1: Navigate to EC2 list.
	m = navigateToEC2List(m)

	// Step 2: Load EC2 resources.
	ec2Resources := []resource.Resource{
		{ID: "i-abc1111111111111", Name: "web-server-1", Status: "running", Fields: map[string]string{"name": "web-server-1", "state": "running"}},
		{ID: "i-abc2222222222222", Name: "web-server-2", Status: "running", Fields: map[string]string{"name": "web-server-2", "state": "running"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    ec2Resources,
	})

	// Step 3: Navigate to detail view for the first instance.
	// DetailModel is now active; ResourceListModel is below it in m.stack.
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Resources[0],
	})

	// Verify we're in detail view (sanity check).
	plainDetail := stripANSI(rootViewContent(m))
	if !strings.Contains(plainDetail, "web-server-1") {
		t.Fatalf("expected to be in detail view showing 'web-server-1', got: %s", plainDetail[:min(200, len(plainDetail))])
	}

	// Step 4: Wave 2 enrichment completes while detail is active.
	// Findings: i-abc1111111111111 has a finding (the same instance we're viewing).
	// We use TypeGen=0 (startup probe, not a rerun).
	enrichMsg := messages.EnrichmentCheckedMsg{
		ResourceType: "ec2",
		Issues:       1,
		Truncated:    false,
		Findings: map[string]resource.EnrichmentFinding{
			"i-abc1111111111111": {Severity: "!", Summary: "system status check failed"},
		},
		Gen:     0, // fresh model: enrichmentGen=0
		TypeGen: 0, // startup probe: enrichmentTypeGen["ec2"]=0
	}
	m, _ = rootApplyMsg(m, enrichMsg)

	// Step 5: Pop back to the ResourceListModel.
	m, _ = rootApplyMsg(m, messages.PopViewMsg{})

	// Verify we're back at the EC2 list.
	plainList := stripANSI(rootViewContent(m))
	if !strings.Contains(plainList, "ec2") {
		t.Fatalf("expected to be back at EC2 list after pop, got: %s", plainList[:min(200, len(plainList))])
	}

	// ASSERTION: The ResourceListModel must show the dot marker (·) for i-abc1111111111111.
	// Pre-fix: The marker is absent because handleEnrichmentChecked only updated the
	// DetailModel (active at the time). The ResourceListModel below on the stack was
	// never called with SetEnrichmentState(findings).
	// Post-fix: The stack is iterated; SetEnrichmentState was called on the
	// ResourceListModel, so the marker (·) appears for the affected row.
	if !strings.Contains(plainList, "·") {
		t.Errorf("after pop from detail to EC2 list, the dot marker '·' must be visible for the enrichment-affected row. "+
			"Pre-fix: absent because handleEnrichmentChecked only updates activeView() (the DetailModel), "+
			"leaving the stacked ResourceListModel's findingsByID empty. "+
			"View excerpt: %s", plainList[:min(400, len(plainList))])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// T069 — EnrichmentCheckedMsg while Detail-B active → stacked Detail-A updated
// ─────────────────────────────────────────────────────────────────────────────

// TestEnrichment_UpdatesStackedDetailWhenAnotherDetailActive verifies that when
// the user has two detail views stacked (detail-A below, detail-B active), and
// Wave 2 enrichment completes with findings for BOTH resources, detail-A must also
// receive its finding. After popping to detail-A, the "⚠ Background Check" section
// must appear.
//
// Pre-fix: only detail-B (active) is updated. Detail-A is never updated. After pop
// to detail-A, no "Background Check" section appears.
//
// Post-fix: handleEnrichmentChecked iterates m.stack and calls SetEnrichmentFinding
// on every *DetailModel whose ResourceType() matches.
func TestEnrichment_UpdatesStackedDetailWhenAnotherDetailActive(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Step 1: Navigate to EC2 list.
	m = navigateToEC2List(m)

	// Step 2: Load two EC2 resources.
	resourceA := resource.Resource{
		ID: "i-detail-a-001", Name: "instance-A", Status: "running",
		Fields: map[string]string{"name": "instance-A", "state": "running"},
	}
	resourceB := resource.Resource{
		ID: "i-detail-b-001", Name: "instance-B", Status: "running",
		Fields: map[string]string{"name": "instance-B", "state": "running"},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    []resource.Resource{resourceA, resourceB},
	})

	// Step 3: Navigate to detail view for instance A.
	// Stack: [MainMenu, ResourceList, DetailA]
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &resourceA,
	})

	plainA := stripANSI(rootViewContent(m))
	if !strings.Contains(plainA, "instance-A") {
		t.Fatalf("expected detail for instance-A, got: %s", plainA[:min(200, len(plainA))])
	}

	// Step 4: Navigate to detail view for instance B.
	// Stack: [MainMenu, ResourceList, DetailA, DetailB]
	// DetailB is now active; DetailA is stacked below it.
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &resourceB,
	})

	plainB := stripANSI(rootViewContent(m))
	if !strings.Contains(plainB, "instance-B") {
		t.Fatalf("expected detail for instance-B, got: %s", plainB[:min(200, len(plainB))])
	}

	// Step 5: Wave 2 enrichment completes with findings for BOTH A and B.
	enrichMsg := messages.EnrichmentCheckedMsg{
		ResourceType: "ec2",
		Issues:       2,
		Truncated:    false,
		Findings: map[string]resource.EnrichmentFinding{
			"i-detail-a-001": {Severity: "!", Summary: "system status check failed on A"},
			"i-detail-b-001": {Severity: "~", Summary: "pending maintenance on B"},
		},
		Gen:     0,
		TypeGen: 0,
	}
	m, _ = rootApplyMsg(m, enrichMsg)

	// Verify detail-B (currently active) shows its finding (this should work pre-fix too).
	plainB2 := stripANSI(rootViewContent(m))
	if !strings.Contains(plainB2, "Background Check") {
		// Detail-B not updated either — something more fundamental is broken.
		t.Logf("note: detail-B (active) also missing Background Check; model may not have enrichment configured correctly")
	}

	// Step 6: Pop back to detail-A.
	// Stack: [MainMenu, ResourceList, DetailA]
	m, _ = rootApplyMsg(m, messages.PopViewMsg{})

	plainA2 := stripANSI(rootViewContent(m))

	// ASSERTION: detail-A must show "Background Check" section with the finding
	// for i-detail-a-001.
	// Pre-fix: absent because handleEnrichmentChecked only updated detail-B (the
	// active view). Detail-A (stacked below) was never called with SetEnrichmentFinding.
	// Post-fix: the stack iteration calls SetEnrichmentFinding on every *DetailModel
	// of matching type, so detail-A is updated even while not active.
	if !strings.Contains(plainA2, "Background Check") {
		t.Errorf("after pop from detail-B to detail-A, the '⚠ Background Check' section must appear "+
			"in detail-A's view because enrichment found an issue for instance-A. "+
			"Pre-fix: absent because handleEnrichmentChecked only updates activeView() (detail-B), "+
			"leaving the stacked detail-A's enrichmentFinding nil. "+
			"View excerpt: %s", plainA2[:min(500, len(plainA2))])
	}
}
