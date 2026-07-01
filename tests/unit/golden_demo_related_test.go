package unit

// golden_demo_related_test.go — Golden tests for related views.

import (
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Test: Every registered RelatedDef MUST have a live Checker.
//
// If Checker is nil the right column shows "—" in live mode while demo
// mode happily shows a count. That's a bug requiring a fix, not a deferral.
// See: https://github.com/k2m30/a9s/issues/243
// ---------------------------------------------------------------------------

func TestGolden_LiveCheckerCompleteness(t *testing.T) {
	for _, shortName := range resource.AllShortNames() {
		defs := resource.GetRelated(shortName)
		for _, def := range defs {
			t.Run(shortName+"->"+def.TargetType, func(t *testing.T) {
				if def.Checker == nil {
					t.Errorf("Checker is nil — shows dash in live mode, count in demo. Implement it. (#243)")
				}
			})
		}
	}
}
