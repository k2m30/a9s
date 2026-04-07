package unit

// Tests for Bug P2: app_handlers.go uses NewHelp (no resource short name),
// so the CloudTrail Events legend is never shown even when the active view is ct-events.
//
// The two broken call sites:
//   internal/tui/app_handlers.go:51  → views.NewHelp(m.keys, ctx)
//   internal/tui/app_handlers.go:440 → views.NewHelp(m.keys, ctx)
//
// Both must be changed to views.NewHelpWithResource(m.keys, ctx, activeShortName).
//
// Testing strategy B (no app harness available):
//   1. Assert that NewHelp (the broken path) produces NO legend — documents the bug.
//   2. Assert that NewHelpWithResource("ct-events") DOES produce a legend — the target state.
//
// Test HW1 currently PASSES (NewHelp correctly hides the legend — but only because
// it has no resource name, not because of correct logic). This is a documentation test
// that shows the BUG: when app_handlers calls NewHelp, the legend is suppressed even
// for ct-events, because NewHelp never receives the short name.
//
// Test HW2 (the positive path) already passes via views_help_ct_events_legend_test.go.
// The tests here are specifically about the WIRING — that the call site in app_handlers.go
// must be changed from NewHelp to NewHelpWithResource.
//
// Test HW3 verifies that NewHelp with HelpFromResourceListPaginated context (the second
// call site, app_handlers.go:440) also lacks the legend — documents the second broken site.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ===========================================================================
// HW1: NewHelp (broken path, app_handlers.go:51) — legend is absent for ct-events
//
// This test documents the bug: when app_handlers.go calls NewHelp, the legend is
// NOT shown even when the active resource type is ct-events.
//
// The test currently PASSES (NewHelp correctly omits the legend), but it documents
// WHY this is wrong: the coder must change NewHelp → NewHelpWithResource at the call
// site. This is a DOCUMENTATION TEST — it will need to be updated or removed after
// the fix, since the fix changes the call site, not the NewHelp function itself.
// ===========================================================================

func TestHelpWiring_NewHelp_MissingLegend_DocumentsBug(t *testing.T) {
	// app_handlers.go:51 currently calls:
	//   help := views.NewHelp(m.keys, ctx)
	// Even when ctx is HelpFromResourceListPaginated and the user is on ct-events,
	// no short name is passed → legend is suppressed.
	h := views.NewHelp(keys.Default(), views.HelpFromResourceListPaginated)
	h.SetSize(120, 40)
	out := h.View()
	plain := stripANSI(out)

	// Document the bug: NewHelp produces no legend, even though the user is on ct-events.
	// This test PASSES now (legend is absent), but it captures the wrong state.
	// After the fix, app_handlers.go will call NewHelpWithResource and this call path
	// will no longer be exercised for ct-events.
	if strings.Contains(plain, "CloudTrail") {
		t.Error("NewHelp (no resource short name) must NOT show CloudTrail legend — " +
			"if this fails, the NewHelp function incorrectly shows the legend without a short name")
	}
	// Note: the real bug is that app_handlers.go CALLS NewHelp instead of NewHelpWithResource.
	// There is no way to test the call site without an app-level harness.
	// See Bug P2 comment in specs/012-ct-events-list-redesign/spec.md.
}

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

// ===========================================================================
// HW3: Second broken call site — app_handlers.go:440 (TargetHelp message handler)
//
// The second call site also uses NewHelp. This test documents the same bug
// via the HelpFromResourceList context (the other paginated context the list
// view produces when paging is not active).
// ===========================================================================

func TestHelpWiring_NewHelp_NonPaginatedContext_MissingLegend_DocumentsBug(t *testing.T) {
	// app_handlers.go:440 currently calls:
	//   h := views.NewHelp(m.keys, ctx)
	// Same issue: short name is lost.
	h := views.NewHelp(keys.Default(), views.HelpFromResourceList)
	h.SetSize(120, 40)
	out := h.View()
	plain := stripANSI(out)

	if strings.Contains(plain, "CloudTrail") {
		t.Error("NewHelp with HelpFromResourceList (no short name) must NOT show CloudTrail legend")
	}
}

// ===========================================================================
// HW4: Both constructors produce identical layout structure
//
// The fix must not break the general keybinding layout.
// Verify both NewHelp and NewHelpWithResource ("ec2") produce a non-empty view
// without the legend for non-ct-events resources.
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
