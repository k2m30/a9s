//go:build integration

package integration

import (
	"testing"
)

// Types flagged ExcludeFromIssueBadge (e.g. CloudTrail Events) never produce a
// menu badge, so ctrl+z hides them even when issueKnown is false.
func TestCtrlZ_HidesExcludeFromIssueBadgeTypes(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)

	scenario.Press("ctrl+z")

	scenario.ExpectViewNotContains("CloudTrail Events")
}
