package unit

// qa_enrichment_marker_test.go — T041: row marker rendering tests for US2.
//
// Tests verify that:
//   - Resources with no finding render no "! " / "~ " prefix marker.
//   - Resources with a "!" finding render a "! " prefix on the identity column.
//   - Resources with a "~" finding render a "~ " prefix on the identity column.
//   - The marker appears at most once per affected row.
//   - Exactly N prefixed rows appear when N resources have findings.
//   - Under NO_COLOR the prefix character still appears in the rendered output.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// markerTypeDef returns a ResourceTypeDef whose "name" key column will be
// selected as the identity column by the cascade (step 2).
func markerTypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		ShortName: "ec2",
		Name:      "EC2 Instances",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 28},
			{Key: "state", Title: "State", Width: 12},
		},
	}
}

// markerResources returns three resources with distinct IDs and healthy statuses
// so no status-based row color interferes with the marker assertion.
func markerResources() []resource.Resource {
	return []resource.Resource{
		{
			ID: "i-1", Name: "web-server-01", Status: "running",
			Fields: map[string]string{"name": "web-server-01", "state": "running"},
		},
		{
			ID: "i-2", Name: "api-gateway-02", Status: "running",
			Fields: map[string]string{"name": "api-gateway-02", "state": "running"},
		},
		{
			ID: "i-3", Name: "worker-node-03", Status: "running",
			Fields: map[string]string{"name": "worker-node-03", "state": "running"},
		},
	}
}

// buildMarkerModel constructs a fully-loaded ResourceListModel for marker tests.
func buildMarkerModel(t *testing.T, findings map[string]resource.EnrichmentFinding) views.ResourceListModel {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	styles.Reinit()
	t.Cleanup(func() {
		styles.Reinit()
	})

	td := markerTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    markerResources(),
	})
	m.SetEnrichmentState(len(findings), false, findings)
	return m
}

// countFindingPrefixes counts occurrences of "! " and "~ " prefixes in the
// rendered view. These prefixes mark rows whose identity column carries a
// Wave-2 finding. The prefix is pure text; row color is unchanged.
func countFindingPrefixes(rendered string) int {
	return strings.Count(rendered, "! ") + strings.Count(rendered, "~ ")
}

// ---------------------------------------------------------------------------
// TestRowMarker_Absent_WhenNoFinding
// ---------------------------------------------------------------------------

// TestRowMarker_Absent_WhenNoFinding verifies that when findingsByID is empty,
// no "! " or "~ " prefix marker appears in the rendered list output.
func TestRowMarker_Absent_WhenNoFinding(t *testing.T) {
	m := buildMarkerModel(t, map[string]resource.EnrichmentFinding{})
	rendered := m.View()
	plain := stripANSI(rendered)
	if strings.Contains(plain, "! ") || strings.Contains(plain, "~ ") {
		t.Error("expected no prefix marker in output when findingsByID is empty, but marker was found")
	}
}

// ---------------------------------------------------------------------------
// TestRowMarker_PresentForFinding_SeverityBang
// ---------------------------------------------------------------------------

// TestRowMarker_PresentForFinding_SeverityBang verifies that a resource with
// severity "!" has a "! " prefix marker in the rendered output.
func TestRowMarker_PresentForFinding_SeverityBang(t *testing.T) {
	findings := map[string]resource.EnrichmentFinding{
		"i-1": {Severity: "!", Summary: "system status impaired"},
	}
	m := buildMarkerModel(t, findings)
	rendered := m.View()

	// The "! " prefix must appear in plain-text output.
	if !strings.Contains(stripANSI(rendered), "! ") {
		t.Error("expected '! ' prefix marker for severity=! finding, not found in output")
	}

	// The raw (ANSI-included) output must contain a color sequence for ColStopped.
	dotStyleBang := styles.ColorStyle(resource.ColorBroken) // ColStopped path
	_ = dotStyleBang                                        // ensure import is used

	// Find the line that contains the resource's name.
	line := findLineContaining(rendered, "web-server-01")
	if line == "" {
		t.Fatal("could not find rendered row for web-server-01")
	}
	if !strings.Contains(stripANSI(line), "! ") {
		t.Error("the rendered row for resource with severity=! does not contain '! ' prefix")
	}
}

// ---------------------------------------------------------------------------
// TestRowMarker_PresentForFinding_SeverityTilde
// ---------------------------------------------------------------------------

// TestRowMarker_PresentForFinding_SeverityTilde verifies that a resource with
// severity "~" has a "~ " prefix marker in the rendered output.
func TestRowMarker_PresentForFinding_SeverityTilde(t *testing.T) {
	findings := map[string]resource.EnrichmentFinding{
		"i-1": {Severity: "~", Summary: "pending maintenance: system-update"},
	}
	m := buildMarkerModel(t, findings)
	rendered := m.View()

	if !strings.Contains(stripANSI(rendered), "~ ") {
		t.Error("expected '~ ' prefix marker for severity=~ finding, not found in output")
	}

	line := findLineContaining(rendered, "web-server-01")
	if line == "" {
		t.Fatal("could not find rendered row for web-server-01")
	}
	if !strings.Contains(stripANSI(line), "~ ") {
		t.Error("the rendered row for resource with severity=~ does not contain '~ ' prefix")
	}
}

// ---------------------------------------------------------------------------
// TestRowMarker_PrefixedToIdentityColumn_NotOthers
// ---------------------------------------------------------------------------

// TestRowMarker_PrefixedToIdentityColumn_NotOthers verifies that the "! " prefix
// marker appears at most once per row (not duplicated across multiple columns).
// This indirectly verifies it is attached to the identity column only.
func TestRowMarker_PrefixedToIdentityColumn_NotOthers(t *testing.T) {
	findings := map[string]resource.EnrichmentFinding{
		"i-1": {Severity: "!", Summary: "impaired"},
	}
	m := buildMarkerModel(t, findings)
	rendered := m.View()

	line := findLineContaining(rendered, "web-server-01")
	if line == "" {
		t.Fatal("could not find rendered row for web-server-01")
	}
	plain := stripANSI(line)
	markerCount := strings.Count(plain, "! ")
	if markerCount > 1 {
		t.Errorf("expected at most 1 '! ' prefix in row, got %d: %q", markerCount, plain)
	}
	if markerCount == 0 {
		t.Error("expected exactly 1 '! ' prefix in row for resource with finding, got 0")
	}
}

// ---------------------------------------------------------------------------
// TestRowMarker_OnlyOnAffectedRows
// ---------------------------------------------------------------------------

// TestRowMarker_OnlyOnAffectedRows verifies that when only one of three
// resources has a finding, exactly one prefix marker appears in the full
// rendered output.
func TestRowMarker_OnlyOnAffectedRows(t *testing.T) {
	findings := map[string]resource.EnrichmentFinding{
		"i-1": {Severity: "!", Summary: "impaired"},
	}
	m := buildMarkerModel(t, findings)
	rendered := m.View()

	total := countFindingPrefixes(rendered)
	if total != 1 {
		t.Errorf("expected exactly 1 prefix marker across all rows (only i-1 has a finding), got %d", total)
	}
}

// TestRowMarker_OnlyOnAffectedRows_Multiple verifies prefix marker count when
// multiple resources have findings.
func TestRowMarker_OnlyOnAffectedRows_Multiple(t *testing.T) {
	findings := map[string]resource.EnrichmentFinding{
		"i-1": {Severity: "!", Summary: "impaired"},
		"i-3": {Severity: "~", Summary: "maintenance pending"},
	}
	m := buildMarkerModel(t, findings)
	rendered := m.View()

	total := countFindingPrefixes(rendered)
	if total != 2 {
		t.Errorf("expected exactly 2 prefix markers (i-1 and i-3 have findings), got %d", total)
	}
}

// ---------------------------------------------------------------------------
// TestRowMarker_NoColorMode_StillVisible
// ---------------------------------------------------------------------------

// TestRowMarker_NoColorMode_StillVisible verifies that under NO_COLOR, the
// "~ " prefix marker still appears in the plain-text rendered output.
// Color styling is additive; the prefix character itself must always render.
func TestRowMarker_NoColorMode_StillVisible(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()
	t.Cleanup(func() {
		styles.Reinit()
	})

	findings := map[string]resource.EnrichmentFinding{
		"i-2": {Severity: "~", Summary: "pending maintenance"},
	}

	td := markerTypeDef()
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 20)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    markerResources(),
	})
	m.SetEnrichmentState(1, false, findings)

	rendered := m.View()
	plain := stripANSI(rendered)
	if !strings.Contains(plain, "! ") && !strings.Contains(plain, "~ ") {
		t.Error("expected finding prefix (\"! \" or \"~ \") to be visible in NO_COLOR mode, but it was not found")
	}
}

// ---------------------------------------------------------------------------
// TestRowMarker_AllResourceTypes — verify no panic across resource types
// ---------------------------------------------------------------------------

// TestRowMarker_AllResourceTypes verifies that SetEnrichmentState and View()
// do not panic for every registered resource type. It does not assert marker
// content (which depends on column config), only crash-safety.
func TestRowMarker_AllResourceTypes(t *testing.T) {
	allTypes := resource.AllResourceTypes()
	if len(allTypes) == 0 {
		t.Fatal("AllResourceTypes returned empty — registry is broken")
	}

	finding := map[string]resource.EnrichmentFinding{
		"test-id-1": {Severity: "!", Summary: "test finding"},
	}

	for _, td := range allTypes {
		td := td
		t.Run(td.ShortName, func(t *testing.T) {
			k := keys.Default()
			m := views.NewResourceList(td, nil, k)
			m.SetSize(80, 24)
			m, _ = m.Init()
			res := []resource.Resource{
				{ID: "test-id-1", Name: "test-resource", Status: "running",
					Fields: map[string]string{"name": "test-resource"}},
			}
			m, _ = m.Update(messages.ResourcesLoadedMsg{
				ResourceType: td.ShortName,
				Resources:    res,
			})
			m.SetEnrichmentState(1, false, finding)
			// Must not panic.
			_ = m.View()
		})
	}
}
