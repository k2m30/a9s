package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

func TestAttentionFilter_DefaultDisabled(t *testing.T) {
	var af views.AttentionFilter
	if af.IsEnabled() {
		t.Error("new AttentionFilter should be disabled by default")
	}
}

func TestAttentionFilter_Toggle(t *testing.T) {
	var af views.AttentionFilter

	af.Toggle()
	if !af.IsEnabled() {
		t.Error("after first Toggle, should be enabled")
	}

	af.Toggle()
	if af.IsEnabled() {
		t.Error("after second Toggle, should be disabled")
	}
}

func TestAttentionFilter_SetEnabled(t *testing.T) {
	var af views.AttentionFilter

	af.SetEnabled(true)
	if !af.IsEnabled() {
		t.Error("SetEnabled(true) should enable")
	}

	af.SetEnabled(false)
	if af.IsEnabled() {
		t.Error("SetEnabled(false) should disable")
	}
}

func TestAttentionFilter_SetEnabledIdempotent(t *testing.T) {
	var af views.AttentionFilter

	af.SetEnabled(true)
	af.SetEnabled(true)
	if !af.IsEnabled() {
		t.Error("double SetEnabled(true) should still be enabled")
	}
}

// TestAttentionFilter_IncludesResourcesWithFindings verifies that when the
// attention filter (ctrl+z) is enabled, a resource whose Wave 1 Color always
// returns Healthy is still shown when it has a Wave 2 enrichment finding.
//
// CodeRabbit PR-273 finding: internal/tui/views/resourcelist_helpers.go:508-515
// filters with only m.typeDef.ResolveColor(r).IsIssue(), which drops resources
// whose issues come exclusively from Wave 2 findings. The fix must add
// "|| findingsByID has r.ID" to the kept predicate.
func TestAttentionFilter_IncludesResourcesWithFindings(t *testing.T) {
	t.Helper()
	t.Setenv("NO_COLOR", "")
	styles.Reinit()
	t.Cleanup(func() { styles.Reinit() })

	// s3 Color always returns ColorHealthy regardless of resource fields,
	// so all three resources below will fail the IsIssue() Wave-1 gate.
	td := resource.FindResourceType("s3")
	if td == nil {
		t.Fatal("s3 type not registered")
	}

	resources := []resource.Resource{
		{
			ID: "res-0", Name: "bucket-alpha", Status: "active",
			Fields: map[string]string{"name": "bucket-alpha"},
		},
		{
			ID: "res-1", Name: "bucket-beta", Status: "active",
			Fields: map[string]string{"name": "bucket-beta"},
		},
		{
			ID: "res-2", Name: "bucket-gamma", Status: "active",
			Fields: map[string]string{"name": "bucket-gamma"},
		},
	}

	k := keys.Default()
	m := views.NewResourceList(*td, nil, k)
	m.SetSize(120, 24)
	m, _ = m.Init()
	m, _ = m.Update(messages.ResourcesLoadedMsg{
		ResourceType: "s3",
		Resources:    resources,
	})

	// Wave 2 finding only for res-0.
	findings := map[string]resource.EnrichmentFinding{
		"res-0": {Severity: "warning", Summary: "public access enabled"},
	}
	m.SetEnrichmentState(1, false, findings)

	// Enable attention filter — mirrors ctrl+z. SetFilter("") re-runs applyFilter()
	// so the attention predicate sees the freshly-set findingsByID map.
	m.SetEnabled(true)
	m.SetFilter("")

	rendered := stripANSI(m.View())

	// res-0 MUST be visible (it has a Wave 2 finding).
	if !strings.Contains(rendered, "bucket-alpha") {
		t.Error("attention filter must show res-0 (bucket-alpha) which has a Wave 2 finding, but it was hidden")
	}

	// res-1 and res-2 must NOT be visible (no finding, Wave-1 Color is Healthy).
	if strings.Contains(rendered, "bucket-beta") {
		t.Error("attention filter must hide res-1 (bucket-beta) which has no finding and Healthy Wave-1 color")
	}
	if strings.Contains(rendered, "bucket-gamma") {
		t.Error("attention filter must hide res-2 (bucket-gamma) which has no finding and Healthy Wave-1 color")
	}
}
