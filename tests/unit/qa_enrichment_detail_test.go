package unit

// qa_enrichment_detail_test.go — T046–T048: US3 enrichment detail view behavioral tests.
//
// Tests verify what the user observes in the detail view (section presence/absence,
// summary text) when SetEnrichmentFinding is called on DetailModel.
// All assertions are behavioral string-contains checks against rendered output.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers local to this file
// ---------------------------------------------------------------------------

// rdsDetailResource returns a simple RDS resource for detail view testing.
func rdsDetailResource() resource.Resource {
	return resource.Resource{
		ID:   "db-prod-1",
		Name: "prod-db-1",
		Fields: map[string]string{
			"db_instance_id": "db-prod-1",
			"engine":         "mysql",
			"status":         "available",
			"instance_class": "db.t3.medium",
		},
	}
}

// newRDSDetailModel builds a DetailModel for an RDS resource with a fixed size.
func newRDSDetailModel(t *testing.T) views.DetailModel {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	styles.Reinit()
	t.Cleanup(func() {
		styles.Reinit()
	})
	k := keys.Default()
	m := views.NewDetail(rdsDetailResource(), "rds", nil, k)
	m.SetSize(120, 40)
	return m
}

// detailRenderOutput returns the detail view's rendered content.
// DetailModel.View() is the string-returning method on the view layer.
func detailRenderOutput(m views.DetailModel) string {
	return m.PlainContent()
}

// ---------------------------------------------------------------------------
// T046-a: No "Background Check" section when finding is nil
// ---------------------------------------------------------------------------

// TestDetailView_NoBackgroundCheckSection_WhenNoFinding asserts that a DetailModel
// with no enrichment finding (the default state) does NOT render a
// "Pending Maintenance" section.
func TestDetailView_NoBackgroundCheckSection_WhenNoFinding(t *testing.T) {
	m := newRDSDetailModel(t)

	output := detailRenderOutput(m)

	if strings.Contains(output, "Pending Maintenance") {
		t.Errorf("detail view must not show 'Pending Maintenance' section when no finding set, got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// T046-b: "Background Check" section present when finding is set
// ---------------------------------------------------------------------------

// TestDetailView_ShowsBackgroundCheckSection_WhenFindingSet asserts that after
// SetEnrichmentFinding is called with a non-nil finding, the detail view renders
// a section containing "Pending Maintenance" and the finding's Summary text.
func TestDetailView_ShowsBackgroundCheckSection_WhenFindingSet(t *testing.T) {
	m := newRDSDetailModel(t)

	finding := resource.EnrichmentFinding{
		Severity: "!",
		Summary:  "pending maintenance: system-update (New OS patch)",
	}
	m.SetEnrichmentFinding(&finding)

	output := detailRenderOutput(m)

	if !strings.Contains(output, "Pending Maintenance") {
		t.Errorf("detail view must show 'Pending Maintenance' section when finding is set, got:\n%s", output)
	}
	if !strings.Contains(output, "pending maintenance: system-update (New OS patch)") {
		t.Errorf("detail view must show finding summary text, got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// T047: SetEnrichmentFinding triggers viewport refresh (summary visible on render)
// ---------------------------------------------------------------------------

// TestDetailView_SetFindingInvalidatesFieldList asserts that after SetEnrichmentFinding
// is called, the next render reflects the new content (the finding section appears).
// This verifies that SetEnrichmentFinding invalidates the field list and triggers
// refreshViewportContent().
func TestDetailView_SetFindingInvalidatesFieldList(t *testing.T) {
	m := newRDSDetailModel(t)

	// First render: no finding.
	first := detailRenderOutput(m)
	if strings.Contains(first, "Pending Maintenance") {
		t.Error("pre-condition failed: initial render should not contain 'Pending Maintenance'")
	}

	// Set finding and render again.
	finding := resource.EnrichmentFinding{Severity: "~", Summary: "pending maintenance: minor-version-upgrade"}
	m.SetEnrichmentFinding(&finding)

	second := detailRenderOutput(m)
	if !strings.Contains(second, "Pending Maintenance") {
		t.Errorf("after SetEnrichmentFinding, render must show 'Pending Maintenance', got:\n%s", second)
	}
	if !strings.Contains(second, "minor-version-upgrade") {
		t.Errorf("after SetEnrichmentFinding, render must show summary text, got:\n%s", second)
	}
}

// ---------------------------------------------------------------------------
// TestDetailView_FindingSeverityBangPresent: "!" severity renders without panic
// ---------------------------------------------------------------------------

// TestDetailView_FindingSeverityBangRendersWithSummary asserts that a finding
// with severity "!" renders the summary text in the detail view without panicking.
// This covers the broken/degraded severity path.
func TestDetailView_FindingSeverityBangRendersWithSummary(t *testing.T) {
	m := newRDSDetailModel(t)

	finding := resource.EnrichmentFinding{Severity: "!", Summary: "latest build FAILED (2026-04-13)"}
	m.SetEnrichmentFinding(&finding)

	// Must not panic.
	output := detailRenderOutput(m)

	if !strings.Contains(output, "latest build FAILED") {
		t.Errorf("severity '!' finding must render its summary, got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// TestDetailView_FindingSeverityTildeRendersWithSummary: "~" severity renders without panic
// ---------------------------------------------------------------------------

// TestDetailView_FindingSeverityTildeRendersWithSummary asserts that a finding
// with severity "~" renders the summary text in the detail view without panicking.
// This covers the informational/scheduled severity path.
func TestDetailView_FindingSeverityTildeRendersWithSummary(t *testing.T) {
	m := newRDSDetailModel(t)

	finding := resource.EnrichmentFinding{Severity: "~", Summary: "pending maintenance: os-upgrade"}
	m.SetEnrichmentFinding(&finding)

	// Must not panic.
	output := detailRenderOutput(m)

	if !strings.Contains(output, "pending maintenance: os-upgrade") {
		t.Errorf("severity '~' finding must render its summary, got:\n%s", output)
	}
}

// ---------------------------------------------------------------------------
// T046-b complement: SetFindingToNil removes section
// ---------------------------------------------------------------------------

// TestDetailView_SetFindingToNilRemovesSection asserts that after a finding is
// set and then cleared via SetEnrichmentFinding(nil), the "Background Check"
// section disappears from the rendered output.
func TestDetailView_SetFindingToNilRemovesSection(t *testing.T) {
	m := newRDSDetailModel(t)

	// Set finding.
	finding := resource.EnrichmentFinding{Severity: "!", Summary: "system status impaired"}
	m.SetEnrichmentFinding(&finding)

	withFinding := detailRenderOutput(m)
	if !strings.Contains(withFinding, "Pending Maintenance") {
		t.Error("pre-condition failed: finding should appear after SetEnrichmentFinding")
	}

	// Clear finding.
	m.SetEnrichmentFinding(nil)

	withoutFinding := detailRenderOutput(m)
	if strings.Contains(withoutFinding, "Pending Maintenance") {
		t.Errorf("after SetEnrichmentFinding(nil), 'Pending Maintenance' must not appear, got:\n%s", withoutFinding)
	}
}

// ---------------------------------------------------------------------------
// T048: YAML view does not show finding section
// ---------------------------------------------------------------------------

// TestDetailView_YAMLViewDoesNotShowFinding asserts that the YAML/raw output
// (RawYAML) for a resource with a finding does NOT include finding summary text.
// YAML views are raw data views and must not be enriched with findings.
func TestDetailView_YAMLViewDoesNotShowFinding(t *testing.T) {
	m := newRDSDetailModel(t)

	finding := resource.EnrichmentFinding{Severity: "!", Summary: "instance status impaired"}
	m.SetEnrichmentFinding(&finding)

	// RawYAML is the YAML serialization path used for the YAML view.
	yamlOutput := m.RawYAML()
	if strings.Contains(yamlOutput, "instance status impaired") {
		t.Errorf("YAML output must not contain finding summary, got:\n%s", yamlOutput)
	}
}

// ---------------------------------------------------------------------------
// Multiple resource types: detail finding renders for ec2, rds, ddb, sfn, cb
// ---------------------------------------------------------------------------

// TestDetailView_FindingRendersForMultipleResourceTypes asserts that the finding
// section renders correctly for resource types that have per-type enrichers.
// Each type has its own section header injected by the per-type injector.
func TestDetailView_FindingRendersForMultipleResourceTypes(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })

	type tc struct {
		resourceType  string
		res           resource.Resource
		summary       string
		sectionHeader string
	}

	cases := []tc{
		{
			resourceType:  "rds",
			sectionHeader: "Pending Maintenance",
			res: resource.Resource{
				ID: "db-prod", Name: "prod-db",
				Fields: map[string]string{"db_instance_id": "db-prod", "status": "available"},
			},
			summary: "pending maintenance: system-update (New OS patch)",
		},
		{
			resourceType:  "cb",
			sectionHeader: "Latest Build",
			res: resource.Resource{
				ID: "my-project", Name: "my-project",
				Fields: map[string]string{"project_name": "my-project"},
			},
			summary: "latest build FAILED (2026-04-10)",
		},
		{
			resourceType:  "sfn",
			sectionHeader: "Latest Execution",
			res: resource.Resource{
				ID: "arn:aws:states:us-east-1:111:stateMachine:my-machine", Name: "my-machine",
				Fields: map[string]string{"name": "my-machine"},
			},
			summary: "latest execution FAILED",
		},
	}

	k := keys.Default()
	for _, tc := range cases {
		tc := tc
		t.Run(tc.resourceType, func(t *testing.T) {
			m := views.NewDetail(tc.res, tc.resourceType, nil, k)
			m.SetSize(120, 40)

			finding := resource.EnrichmentFinding{Severity: "!", Summary: tc.summary}
			m.SetEnrichmentFinding(&finding)

			output := m.PlainContent()

			if !strings.Contains(output, tc.sectionHeader) {
				t.Errorf("[%s] detail view must show '%s' section, got:\n%s", tc.resourceType, tc.sectionHeader, output)
			}
			if !strings.Contains(output, tc.summary) {
				t.Errorf("[%s] detail view must show summary '%s', got:\n%s", tc.resourceType, tc.summary, output)
			}
		})
	}
}
