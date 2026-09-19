package unit

// Enrichment completion must update
// EVERY view of the matching type on m.stack, not only the active one.
//
// If the user has navigated from ResourceListModel (RDS list) to DetailModel
// (detail view for one RDS instance), and Wave 2 enrichment completes while
// the detail is active, the ResourceListModel below it in m.stack must not
// keep stale findingsByID: when the user presses Esc, the revealed
// ResourceListModel shows the markers and banner.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// ─────────────────────────────────────────────────────────────────────────────
// EnrichmentCheckedMsg while Detail active → stacked ResourceList updated
// ─────────────────────────────────────────────────────────────────────────────

// TestEnrichment_UpdatesStackedResourceListWhenDetailActive verifies that when
// the user navigates from the RDS list to a detail view, and Wave 2 enrichment
// completes while the detail is active, the ResourceListModel below the detail
// on the stack is updated with findings. After popping back to the list, row
// markers and banner must be visible.
func TestEnrichment_UpdatesStackedResourceListWhenDetailActive(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "rds",
	})

	// "db_identifier" is the dbi type's DisplayNameKey
	// (core/aws/catalog_databases.go); the identity column reads it.
	rdsResources := []resource.Resource{
		{ID: "db-stacked-a-001", Name: "db-stacked-a-001", Fields: map[string]string{"db_identifier": "db-stacked-a-001", "status": "available"}},
		{ID: "db-stacked-b-001", Name: "db-stacked-b-001", Fields: map[string]string{"db_identifier": "db-stacked-b-001", "status": "available"}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "rds",
		Resources:    rdsResources, Provenance: messages.FetchProvenanceCanonicalList,
	})

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "rds",
		Resource:     &rdsResources[0],
	})

	plainDetail := stripANSI(rootViewContent(m))
	if !strings.Contains(plainDetail, "db-stacked-a-001") {
		t.Fatalf("expected to be in detail view showing 'db-stacked-a-001', got: %s", plainDetail[:min(200, len(plainDetail))])
	}

	enrichMsg := messages.EnrichmentChecked{
		ResourceType: "rds",
		Truncated:    false,
		Findings: map[string][]domain.Finding{
			"db-stacked-a-001": {{Code: "rds.pending-maintenance", Phrase: "pending maintenance: system-update", Severity: domain.SevBroken, Source: "wave2:rds"}},
		},
		Gen:     0, // fresh model: enrichmentGen=0
		TypeGen: 0, // startup probe: enrichmentTypeGen["rds"]=0
	}
	m, _ = rootApplyMsg(m, enrichMsg)

	m, _ = rootApplyMsg(m, messages.PopView{})

	plainList := stripANSI(rootViewContent(m))
	if !strings.Contains(plainList, "rds") {
		t.Fatalf("expected to be back at RDS list after pop, got: %s", plainList[:min(200, len(plainList))])
	}

	// Checked via ctrl+z survival, not the literal "! " glyph text: colorDBI
	// prefers colorFromAnyFinding (core/aws/catalog_databases.go), so once the
	// SevBroken Finding is applied no glyph is produced (the glyph only fires
	// when ResolveColor()==ColorHealthy; see core/app/list_columns.go).
	_, ctrlZVisible := isVisibleUnderCtrlZ(m, "db-stacked-a-001")
	if !ctrlZVisible {
		t.Errorf("after pop from detail to RDS list, db-stacked-a-001 must survive ctrl+z (finding applied). "+
			"Pre-fix: absent because handleEnrichmentChecked only updates activeView() (the DetailModel), "+
			"leaving the stacked ResourceListModel's findingsByID empty. "+
			"View excerpt: %s", plainList[:min(400, len(plainList))])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// EnrichmentCheckedMsg while Detail-B active → stacked Detail-A updated
// ─────────────────────────────────────────────────────────────────────────────

// TestEnrichment_UpdatesStackedDetailWhenAnotherDetailActive verifies that when
// the user has two detail views stacked (detail-A below, detail-B active), and
// Wave 2 enrichment completes with findings for BOTH resources, detail-A must also
// receive its finding. After popping to detail-A, the "Attention" section
// must appear.
func TestEnrichment_UpdatesStackedDetailWhenAnotherDetailActive(t *testing.T) {
	tui.Version = "test"
	m := newRootSizedModel()

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "rds",
	})

	resourceA := resource.Resource{
		ID: "db-stacked-a-001", Name: "db-stacked-a-001",
		Fields: map[string]string{"db_instance_id": "db-stacked-a-001", "status": "available"},
	}
	resourceB := resource.Resource{
		ID: "db-stacked-b-001", Name: "db-stacked-b-001",
		Fields: map[string]string{"db_instance_id": "db-stacked-b-001", "status": "available"},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "rds",
		Resources:    []resource.Resource{resourceA, resourceB},
	})

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

	// Stack: [MainMenu, ResourceList, DetailA, DetailB]
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "rds",
		Resource:     &resourceB,
	})

	plainB := stripANSI(rootViewContent(m))
	if !strings.Contains(plainB, "db-stacked-b-001") {
		t.Fatalf("expected detail for db-stacked-b-001, got: %s", plainB[:min(200, len(plainB))])
	}

	enrichMsg := messages.EnrichmentChecked{
		ResourceType: "rds",
		Truncated:    false,
		Findings: map[string][]domain.Finding{
			"db-stacked-a-001": {{Code: "rds.pending-maintenance", Phrase: "pending maintenance: system-update on A", Severity: domain.SevBroken, Source: "wave2:rds"}},
			"db-stacked-b-001": {{Code: "rds.pending-maintenance", Phrase: "pending maintenance: minor-version-upgrade on B", Severity: domain.SevWarn, Source: "wave2:rds"}},
		},
		Gen:     0,
		TypeGen: 0,
	}
	m, _ = rootApplyMsg(m, enrichMsg)

	plainB2 := stripANSI(rootViewContent(m))
	if !strings.Contains(plainB2, "Attention") {
		t.Logf("note: detail-B (active) also missing Pending Maintenance; model may not have enrichment configured correctly")
	}

	// Stack: [MainMenu, ResourceList, DetailA]
	m, _ = rootApplyMsg(m, messages.PopView{})

	plainA2 := stripANSI(rootViewContent(m))

	if !strings.Contains(plainA2, "Attention") {
		t.Errorf("after pop from detail-B to detail-A, the 'Pending Maintenance' section must appear "+
			"in detail-A's view because enrichment found an issue for db-stacked-a-001. "+
			"Pre-fix: absent because handleEnrichmentChecked only updates activeView() (detail-B), "+
			"leaving the stacked detail-A's enrichmentFinding nil. "+
			"View excerpt: %s", plainA2[:min(500, len(plainA2))])
	}
}
