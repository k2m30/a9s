//go:build integration

package integration

import (
	"testing"
)

// TestCtrlZ_HidesExcludeFromIssueBadgeTypes verifies that resource types flagged
// ExcludeFromIssueBadge (e.g. CloudTrail Events) are hidden under ctrl+z even when
// issueKnown is false. Today they leak through because the "unknown → visible"
// fallback in isVisibleUnderIssueFilter does not account for types that can never
// produce a menu badge.
func TestCtrlZ_HidesExcludeFromIssueBadgeTypes(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)

	scenario.Press("ctrl+z")

	scenario.ExpectViewNotContains("CloudTrail Events")
}

// TestCtrlZ_HidesAlwaysHealthyTypes verifies that AlwaysHealthy resource types are
// hidden under ctrl+z regardless of probe state. AlwaysHealthy types always return
// ColorHealthy from their Color func so they can never have issues; they must be
// unconditionally hidden when the issue filter is active.
func TestCtrlZ_HidesAlwaysHealthyTypes(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)

	scenario.Press("ctrl+z")

	scenario.ExpectViewNotContains("S3 Buckets")
	scenario.ExpectViewNotContains("Secrets Manager")
	scenario.ExpectViewNotContains("IAM Users")
	scenario.ExpectViewNotContains("Security Groups")
}
