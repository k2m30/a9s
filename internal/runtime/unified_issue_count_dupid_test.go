package runtime

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// alwaysBrokenTD returns an issue color for every resource regardless of its
// fields/findings, so a test can control which resources count purely by how
// many it passes.
func alwaysBrokenTD() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		ShortName: "dupidtest",
		Color:     func(domain.Resource) domain.Color { return domain.ColorBroken },
	}
}

// TestUnifiedIssueCount_DistinctResourcesSharingID_CountEach pins the menu
// badge to count per resource, matching the list title (listIssueCount, which
// counts per row). Two DISTINCT resources that share an ID — e.g. two ACM
// certificates for one domain, both expired (ACM keys on the domain name) —
// must each bump the badge. Before this fix unifiedIssueCount accumulated into
// a set keyed by r.ID, collapsing the pair to one and undercounting the menu
// relative to the list.
func TestUnifiedIssueCount_DistinctResourcesSharingID_CountEach(t *testing.T) {
	resources := []resource.Resource{
		{ID: "artifacts.example.com", Name: "cert-expired-2023"},
		{ID: "artifacts.example.com", Name: "cert-active-2026"},
	}
	if got := unifiedIssueCount(resources, alwaysBrokenTD(), nil); got != 2 {
		t.Errorf("unifiedIssueCount with two distinct same-ID issue resources = %d, want 2 (each counts, matching the list title)", got)
	}
}

// TestUnifiedIssueCount_SameResourceBothWaves_CountsOnce guards the intended
// dedup the fix must preserve: a single resource flagged by BOTH its Wave-1
// color AND a Wave-2 "!" finding counts once, never twice.
func TestUnifiedIssueCount_SameResourceBothWaves_CountsOnce(t *testing.T) {
	resources := []resource.Resource{{ID: "i-solo", Name: "one"}}
	findings := map[string][]domain.Finding{
		"i-solo": {{Severity: domain.SevBroken, Source: "wave2:dupidtest"}},
	}
	if got := unifiedIssueCount(resources, alwaysBrokenTD(), findings); got != 1 {
		t.Errorf("unifiedIssueCount same resource in both waves = %d, want 1 (no double-count)", got)
	}
}
