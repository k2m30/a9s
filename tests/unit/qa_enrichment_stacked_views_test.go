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
// If the user has navigated from ResourceListModel (RDS list) to DetailModel
// (detail view for one RDS instance), and Wave 2 enrichment completes while the
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
//           must show "Attention" section after pop-to-A.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

// ─────────────────────────────────────────────────────────────────────────────
// T068 — EnrichmentCheckedMsg while Detail active → stacked ResourceList updated
// ─────────────────────────────────────────────────────────────────────────────

// TestEnrichment_UpdatesStackedResourceListWhenDetailActive verifies that when
// the user navigates from the RDS list to a detail view, and Wave 2 enrichment
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

	// Step 1: Navigate to RDS list.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "rds",
	})

	// Step 2: Load RDS resources.
	rdsResources := []resource.Resource{
		{ID: "db-stacked-a-001", Name: "db-stacked-a-001", Fields: map[string]string{"db_instance_id": "db-stacked-a-001", "status": "available"}},
		{ID: "db-stacked-b-001", Name: "db-stacked-b-001", Fields: map[string]string{"db_instance_id": "db-stacked-b-001", "status": "available"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "rds",
		Resources:    rdsResources,
	})

	// Step 3: Navigate to detail view for the first instance.
	// DetailModel is now active; ResourceListModel is below it in m.stack.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "rds",
		Resource:     &rdsResources[0],
	})

	// Verify we're in detail view (sanity check).
	plainDetail := stripANSI(rootViewContent(m))
	if !strings.Contains(plainDetail, "db-stacked-a-001") {
		t.Fatalf("expected to be in detail view showing 'db-stacked-a-001', got: %s", plainDetail[:min(200, len(plainDetail))])
	}

	// Step 4: Wave 2 enrichment completes while detail is active.
	// Findings: db-stacked-a-001 has a finding (the same instance we're viewing).
	// We use TypeGen=0 (startup probe, not a rerun).
	enrichMsg := messages.EnrichmentChecked{
		ResourceType: "rds",
		Issues:       1,
		Truncated:    false,
		Findings: map[string]domain.Finding{
			"db-stacked-a-001": {Code: "rds.pending-maintenance", Phrase: "pending maintenance: system-update", Severity: domain.SevBroken, Source: "wave2:rds"},
		},
		Gen:     0, // fresh model: enrichmentGen=0
		TypeGen: 0, // startup probe: enrichmentTypeGen["rds"]=0
	}
	m, _ = rootApplyMsg(m, enrichMsg)

	// Step 5: Pop back to the ResourceListModel.
	m, _ = rootApplyMsg(m, messages.PopView{})

	// Verify we're back at the RDS list.
	plainList := stripANSI(rootViewContent(m))
	if !strings.Contains(plainList, "rds") {
		t.Fatalf("expected to be back at RDS list after pop, got: %s", plainList[:min(200, len(plainList))])
	}

	// ASSERTION: The ResourceListModel must show the "! " prefix marker for db-stacked-a-001.
	// Pre-fix: The marker is absent because handleEnrichmentChecked only updated the
	// DetailModel (active at the time). The ResourceListModel below on the stack was
	// never called with SetEnrichmentState(findings).
	// Post-fix: The stack is iterated; SetEnrichmentState was called on the
	// ResourceListModel, so the "! " prefix appears for the affected row.
	if !strings.Contains(plainList, "! ") {
		t.Errorf("after pop from detail to RDS list, the '! ' prefix marker must be visible for the enrichment-affected row. "+
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
// receive its finding. After popping to detail-A, the "Attention" section
// must appear.
//
// Pre-fix: only detail-B (active) is updated. Detail-A is never updated. After pop
// to detail-A, no "Attention" section appears.
//
// Post-fix: handleEnrichmentChecked iterates m.stack and calls SetEnrichmentFinding
// on every *DetailModel whose ResourceType() matches.
func TestEnrichment_UpdatesStackedDetailWhenAnotherDetailActive(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	// Step 1: Navigate to RDS list.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "rds",
	})

	// Step 2: Load two RDS resources.
	resourceA := resource.Resource{
		ID: "db-stacked-a-001", Name: "db-stacked-a-001",
		Fields: map[string]string{"db_instance_id": "db-stacked-a-001", "status": "available"},
	}
	resourceB := resource.Resource{
		ID: "db-stacked-b-001", Name: "db-stacked-b-001",
		Fields: map[string]string{"db_instance_id": "db-stacked-b-001", "status": "available"},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "rds",
		Resources:    []resource.Resource{resourceA, resourceB},
	})

	// Step 3: Navigate to detail view for instance A.
	// Stack: [MainMenu, ResourceList, DetailA]
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "rds",
		Resource:     &resourceA,
	})

	plainA := stripANSI(rootViewContent(m))
	if !strings.Contains(plainA, "db-stacked-a-001") {
		t.Fatalf("expected detail for db-stacked-a-001, got: %s", plainA[:min(200, len(plainA))])
	}

	// Step 4: Navigate to detail view for instance B.
	// Stack: [MainMenu, ResourceList, DetailA, DetailB]
	// DetailB is now active; DetailA is stacked below it.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "rds",
		Resource:     &resourceB,
	})

	plainB := stripANSI(rootViewContent(m))
	if !strings.Contains(plainB, "db-stacked-b-001") {
		t.Fatalf("expected detail for db-stacked-b-001, got: %s", plainB[:min(200, len(plainB))])
	}

	// Step 5: Wave 2 enrichment completes with findings for BOTH A and B.
	enrichMsg := messages.EnrichmentChecked{
		ResourceType: "rds",
		Issues:       2,
		Truncated:    false,
		Findings: map[string]domain.Finding{
			"db-stacked-a-001": {Code: "rds.pending-maintenance", Phrase: "pending maintenance: system-update on A", Severity: domain.SevBroken, Source: "wave2:rds"},
			"db-stacked-b-001": {Code: "rds.pending-maintenance", Phrase: "pending maintenance: minor-version-upgrade on B", Severity: domain.SevWarn, Source: "wave2:rds"},
		},
		Gen:     0,
		TypeGen: 0,
	}
	m, _ = rootApplyMsg(m, enrichMsg)

	// Verify detail-B (currently active) shows its finding (this should work pre-fix too).
	plainB2 := stripANSI(rootViewContent(m))
	if !strings.Contains(plainB2, "Attention") {
		// Detail-B not updated either — something more fundamental is broken.
		t.Logf("note: detail-B (active) also missing Pending Maintenance; model may not have enrichment configured correctly")
	}

	// Step 6: Pop back to detail-A.
	// Stack: [MainMenu, ResourceList, DetailA]
	m, _ = rootApplyMsg(m, messages.PopView{})

	plainA2 := stripANSI(rootViewContent(m))

	// ASSERTION: detail-A must show "Attention" section with the finding
	// for db-stacked-a-001.
	// Pre-fix: absent because handleEnrichmentChecked only updated detail-B (the
	// active view). Detail-A (stacked below) was never called with SetEnrichmentFinding.
	// Post-fix: the stack iteration calls SetEnrichmentFinding on every *DetailModel
	// of matching type, so detail-A is updated even while not active.
	if !strings.Contains(plainA2, "Attention") {
		t.Errorf("after pop from detail-B to detail-A, the 'Pending Maintenance' section must appear "+
			"in detail-A's view because enrichment found an issue for db-stacked-a-001. "+
			"Pre-fix: absent because handleEnrichmentChecked only updates activeView() (detail-B), "+
			"leaving the stacked detail-A's enrichmentFinding nil. "+
			"View excerpt: %s", plainA2[:min(500, len(plainA2))])
	}
}
