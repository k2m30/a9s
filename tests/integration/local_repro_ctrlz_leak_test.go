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

// Post-AlwaysHealthy-purge: every registered type has at least a Wave 1 or Wave 2
// signal per docs/attention-signals.md, so no type is unconditionally hidden under
// ctrl+z. Types with zero known issues (and not truncated) are hidden — but that
// is the "confirmed zero" rule, not an AlwaysHealthy rule. The former
// TestCtrlZ_HidesAlwaysHealthyTypes asserted AlwaysHealthy types were always
// hidden; that invariant is gone. Visibility under ctrl+z for those types is now
// driven by their per-type probe state like any other type.
