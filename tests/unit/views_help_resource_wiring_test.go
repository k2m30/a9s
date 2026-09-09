package unit

// The help screen is built via NewHelpWithResource(m.keys, ctx,
// activeShortName), so the CloudTrail Events legend shows for ct-events;
// HW2/HW4 below are the regression guards.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// HW2/HW4 below guard the NewHelpWithResource routing at the help call site.

// ===========================================================================
// HW2: NewHelpWithResource("ct-events") — legend IS present (target state)
//
// This test currently PASSES (the constructor wiring is correct).
// It serves as a regression guard to ensure the constructor keeps working.
// ===========================================================================

func TestHelpWiring_NewHelpWithResource_HasLegend_RegressionGuard(t *testing.T) {
	// The fix at app_handlers.go:51 and :440 must produce this result.
	h := views.NewHelpWithResource(keys.Default(), views.HelpFromResourceListPaginated, "ct-events")
	h.SetSize(120, 40)
	out := h.View()
	plain := stripANSI(out)

	if !strings.Contains(plain, "CloudTrail") {
		t.Error("NewHelpWithResource(ct-events) must show CloudTrail legend — regression guard")
	}
}

// HW3 (the second NewHelp call-site bug at app_handlers.go:440) is retired
// for the same reason as HW1 above — NewHelp is dead, the wiring fix landed.

// ===========================================================================
// HW4: Non-ct-events resource types never show the legend
//
// Verify NewHelpWithResource produces a non-empty view without the legend
// for every non-ct-events resource type.
// ===========================================================================

func TestHelpWiring_NewHelpWithResource_NonCT_NeverShowsLegend(t *testing.T) {
	nonCTTypes := []string{"ec2", "s3", "rds", "lambda", "role", "iam-user", "eks", "sg", "vpc"}
	for _, shortName := range nonCTTypes {
		h := views.NewHelpWithResource(keys.Default(), views.HelpFromResourceListPaginated, shortName)
		h.SetSize(120, 40)
		out := h.View()
		plain := stripANSI(out)

		if strings.Contains(plain, "CloudTrail") {
			t.Errorf("NewHelpWithResource(%q, HelpFromResourceListPaginated): "+
				"must NOT show CloudTrail legend for non-ct-events resource type", shortName)
		}
		if plain == "" {
			t.Errorf("NewHelpWithResource(%q): produced empty view", shortName)
		}
	}
}
