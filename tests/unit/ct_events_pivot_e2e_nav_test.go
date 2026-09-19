package unit_test

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
)

// TestCtEventsPivotNavigation_DemoMode_LandsOnFilteredList verifies that in
// demo mode, navigating via a ct-events self-pivot row (Username or EventName)
// produces a non-empty ResourcesLoadedMsg rather than an APIErrorMsg.
func TestCtEventsPivotNavigation_DemoMode_LandsOnFilteredList(t *testing.T) {
	t.Skip("needs rewrite onto cold-cache harness (T047-T049)")
}
