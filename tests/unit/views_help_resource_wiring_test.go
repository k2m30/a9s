package unit

// The help screen is built via NewHelpWithResource(m.keys, ctx,
// activeShortName), so the CloudTrail Events legend shows for ct-events only.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

func TestHelpWiring_NewHelpWithResource_HasLegend_RegressionGuard(t *testing.T) {
	h := views.NewHelpWithResource(keys.Default(), views.HelpFromResourceListPaginated, "ct-events")
	h.SetSize(120, 40)
	out := h.View()
	plain := stripANSI(out)

	if !strings.Contains(plain, "CloudTrail") {
		t.Error("NewHelpWithResource(ct-events) must show CloudTrail legend — regression guard")
	}
}

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
