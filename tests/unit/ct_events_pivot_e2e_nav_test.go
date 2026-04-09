package unit_test

// ct_events_pivot_e2e_nav_test.go — Layer 5 end-to-end regression test for
// demo-mode ct-events self-pivot navigation.
//
// All tests need rewrite onto the cold-cache harness (T047-T049, Phase 5 rewrite).

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
)

// TestCtEventsPivotNavigation_DemoMode_LandsOnFilteredList verifies that in
// demo mode, navigating via a ct-events self-pivot row (Username or EventName)
// produces a non-empty ResourcesLoadedMsg rather than an APIErrorMsg.
func TestCtEventsPivotNavigation_DemoMode_LandsOnFilteredList(t *testing.T) {
	t.Skip("needs rewrite onto cold-cache harness (T047-T049)")
}
