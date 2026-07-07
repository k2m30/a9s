package unit_test

// rightcolumn_no_actionable_focus_test.go — Bug F regression tests.
//
// Bug F: The right column remains focusable (Tab moves focus there) even when every
// row has been fully resolved and no row is actionable (Count=0, the only
// resolved-and-never-actionable state — see resource.IsRelatedActionable).
// HasActionableRows() should return false in this state.
//
// The current HasActionableRows() implementation returns true for loading rows
// (rows where no RelatedCheckResultMsg has been delivered yet). The bug is that
// after ALL rows are delivered as Count=0, the right column must NOT accept focus.
//
// Owner decision #38 (2026-07-06) retired Count=-1/no-FetchFilter as a
// non-actionable state: a transient "(?)" row is now a drillable pivot into
// the target type's plain top-level list, so it counts as actionable here.
// The "MixedNonActionable" subtests below inject a mix of Count=0 and
// Count=-1/no-FetchFilter rows and — despite the historical helper name —
// now assert that focus IS granted, since the panel has actionable rows.
//
// Test strategy: Use ct-events detail model (17+ registered related defs).
// Inject non-actionable (Count=0) results for every registered related def
// to exercise the true all-non-actionable guard. Verify:
//  1. HasActionableRows() == false (read from View output / Tab behaviour).
//  2. Tab key does NOT transfer focus to right column.

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// helpers local to this file
// ---------------------------------------------------------------------------

// buildAllZeroDetail creates a ct-events DetailModel and injects Count=0 for
// every registered related def. When all rows are resolved as Count=0, the
// right column must not be focusable.
func buildAllZeroDetail(t *testing.T, fixture resource.Resource) views.DetailModel {
	t.Helper()
	k := keys.Default()
	cfg := config.DefaultConfig()
	d := views.NewDetail(fixture, "ct-events", cfg, k)
	d.SetSize(200, 40)

	defs := resource.GetRelated("ct-events")
	for _, def := range defs {
		// Inject Count=0, no FetchFilter — non-actionable, resolved.
		d, _ = d.Update(messages.RelatedCheckResult{
			ResourceType:   "ct-events",
			DefDisplayName: def.DisplayName,
			Result: resource.RelatedCheckResult{
				TargetType:  def.TargetType,
				Count:       0,
				FetchFilter: nil,
			},
		})
	}
	return d
}

// buildMixedNonActionableDetail creates a ct-events DetailModel and injects a
// mix of Count=0 (never actionable) and Count=-1/no-FetchFilter rows. Despite
// the name, the Count=-1/no-FetchFilter half IS actionable as of owner
// decision #38 (2026-07-06): a transient "(?)" row is now a drillable pivot
// into the target type's plain top-level list (see
// qa_related_transient_unknown_drill_test.go), so this panel is no longer
// all-non-actionable — see TestRightColumnNoActionableRowsBlocksFocus's
// "MixedNonActionable" subtests below, which now assert focus IS granted.
func buildMixedNonActionableDetail(t *testing.T, fixture resource.Resource) views.DetailModel {
	t.Helper()
	k := keys.Default()
	cfg := config.DefaultConfig()
	d := views.NewDetail(fixture, "ct-events", cfg, k)
	d.SetSize(200, 40)

	defs := resource.GetRelated("ct-events")
	for i, def := range defs {
		state := domain.RelatedResolved
		if i%2 != 0 {
			state = domain.RelatedUnknown // odd indices: resolved-unknown, no FetchFilter
		}
		d, _ = d.Update(messages.RelatedCheckResult{
			ResourceType:   "ct-events",
			DefDisplayName: def.DisplayName,
			Result: resource.RelatedCheckResult{
				TargetType:  def.TargetType,
				State:       state,
				Count:       0,
				FetchFilter: nil, // no FetchFilter — not a pivot row
			},
		})
	}
	return d
}

// pressTab sends a Tab key to the detail model and returns the updated model.
func pressTab(d views.DetailModel) views.DetailModel {
	updated, _ := d.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	return updated
}

// pressScrollRight sends the "l" key (ScrollRight) to focus the right column.
func pressScrollRight(d views.DetailModel) views.DetailModel {
	updated, _ := d.Update(tea.KeyPressMsg{Code: -1, Text: "l"})
	return updated
}

// ---------------------------------------------------------------------------
// TestRightColumnNoActionableRowsBlocksFocus — Bug F regression
// ---------------------------------------------------------------------------

// TestRightColumnNoActionableRowsBlocksFocus verifies that after all related rows
// are resolved as Count=0 (or Count=-1 without FetchFilter), the right column
// must not receive focus via Tab or "l" key.
func TestRightColumnNoActionableRowsBlocksFocus(t *testing.T) {
	ensureNoColor(t)

	ctClient := fakes.NewCloudTrail()
	fixtures, fetchErr := awsclient.FetchCloudTrailEvents(context.Background(), ctClient)
	if fetchErr != nil || len(fixtures) == 0 {
		t.Fatalf("demo ct-events fixtures missing (err=%v, len=%d)", fetchErr, len(fixtures))
	}

	defs := resource.GetRelated("ct-events")
	if len(defs) == 0 {
		t.Fatal("resource.GetRelated(\"ct-events\") returned no defs — SetRelatedForTest not called?")
	}

	fixture := fixtures[0]

	// --- Subtest: all rows Count=0 ---
	t.Run("AllCountZero/TabDoesNotFocus", func(t *testing.T) {
		d := buildAllZeroDetail(t, fixture)

		// Capture view before Tab to compare after.
		viewBefore := stripAnsi(d.View())

		// Press Tab — should NOT transfer focus to right column.
		dAfterTab := pressTab(d)
		viewAfterTab := stripAnsi(dAfterTab.View())

		// Bug F: if Tab focused the right column, the view changes (cursor highlight
		// appears in the right column). If the right column is correctly not focusable,
		// the view must be unchanged (no cursor highlight added).
		//
		// We cannot call HasActionableRows() directly (unexported type). We infer it
		// from Tab not changing the view.
		if viewBefore != viewAfterTab {
			t.Errorf("Bug F: Tab changed view when all rows are Count=0 — right column gained focus illegally."+
				"\nevent=%s defs=%d"+
				"\nView diff: before=%d chars after=%d chars",
				fixture.ID, len(defs), len(viewBefore), len(viewAfterTab))
		}
	})

	t.Run("AllCountZero/ScrollRightDoesNotFocus", func(t *testing.T) {
		d := buildAllZeroDetail(t, fixture)

		viewBefore := stripAnsi(d.View())
		dAfterL := pressScrollRight(d)
		viewAfterL := stripAnsi(dAfterL.View())

		// Bug F: "l" must not focus the right column when HasActionableRows()==false.
		if viewBefore != viewAfterL {
			t.Errorf("Bug F: 'l' (ScrollRight) changed view when all rows are Count=0 — right column gained focus illegally."+
				"\nevent=%s defs=%d",
				fixture.ID, len(defs))
		}
	})

	// --- Subtest: mix of Count=0 (never actionable) and Count=-1/no-FetchFilter
	// (actionable per owner decision #38) — the panel now DOES have actionable
	// rows, so focus must be granted, mirroring WithOneActionableRow below. ---
	t.Run("MixedNonActionable/TabFocuses", func(t *testing.T) {
		d := buildMixedNonActionableDetail(t, fixture)

		viewBefore := stripAnsi(d.View())
		dAfterTab := pressTab(d)
		viewAfterTab := stripAnsi(dAfterTab.View())

		if viewBefore == viewAfterTab {
			t.Errorf("Tab did not change view when the panel has actionable Count=-1-no-filter rows (owner decision #38: transient unknown rows are now actionable) — right column should have gained focus."+
				"\nevent=%s defs=%d",
				fixture.ID, len(defs))
		}
	})

	t.Run("MixedNonActionable/ScrollRightFocuses", func(t *testing.T) {
		d := buildMixedNonActionableDetail(t, fixture)

		viewBefore := stripAnsi(d.View())
		dAfterL := pressScrollRight(d)
		viewAfterL := stripAnsi(dAfterL.View())

		if viewBefore == viewAfterL {
			t.Errorf("'l' (ScrollRight) did not change view when the panel has actionable Count=-1-no-filter rows (owner decision #38: transient unknown rows are now actionable) — right column should have gained focus."+
				"\nevent=%s defs=%d",
				fixture.ID, len(defs))
		}
	})

	// --- Subtest: verify focus IS available when at least one row is actionable ---
	// This ensures we aren't testing a broken model that never allows focus.
	t.Run("WithOneActionableRow/TabDoesFocus", func(t *testing.T) {
		// Start with all-zero model, then inject one actionable row (Count=1).
		d := buildAllZeroDetail(t, fixture)

		// Override first def to be actionable.
		if len(defs) == 0 {
			t.Skip("no defs")
		}
		firstDef := defs[0]
		d, _ = d.Update(messages.RelatedCheckResult{
			ResourceType: "ct-events",
			Result: resource.RelatedCheckResult{
				TargetType:  firstDef.TargetType,
				Count:       1,
				ResourceIDs: []string{"test-resource-id"},
			},
		})

		viewBefore := stripAnsi(d.View())
		dAfterTab := pressTab(d)
		viewAfterTab := stripAnsi(dAfterTab.View())

		// With one actionable row, Tab MUST change the view (focus is accepted).
		if viewBefore == viewAfterTab {
			t.Errorf("Control check FAIL: Tab did not change view even when one actionable row exists"+
				" — either right column never accepts focus or the model is broken."+
				"\nevent=%s actionable targetType=%q",
				fixture.ID, firstDef.TargetType)
		}
	})
}

// ---------------------------------------------------------------------------
// TestRightColumnNoActionableRowsBlocksFocus_AllFixtures
// ---------------------------------------------------------------------------

// TestRightColumnNoActionableRowsBlocksFocus_AllFixtures runs the all-zero
// non-actionable guard against every ct-events fixture, not just the first.
func TestRightColumnNoActionableRowsBlocksFocus_AllFixtures(t *testing.T) {
	ensureNoColor(t)

	ctClient := fakes.NewCloudTrail()
	fixtures, fetchErr := awsclient.FetchCloudTrailEvents(context.Background(), ctClient)
	if fetchErr != nil || len(fixtures) == 0 {
		t.Fatalf("demo ct-events fixtures missing (err=%v, len=%d)", fetchErr, len(fixtures))
	}

	defs := resource.GetRelated("ct-events")
	if len(defs) == 0 {
		t.Fatal("resource.GetRelated(\"ct-events\") returned no defs")
	}

	for _, fixture := range fixtures {
		t.Run(fixture.ID, func(t *testing.T) {
			d := buildAllZeroDetail(t, fixture)

			viewBefore := stripAnsi(d.View())

			// Tab must not focus.
			dTab := pressTab(d)
			viewAfterTab := stripAnsi(dTab.View())
			if viewBefore != viewAfterTab {
				t.Errorf("Bug F: event=%s: Tab focused right column when all rows Count=0"+
					" — expected no view change, got diff (before=%d chars after=%d chars)",
					fixture.ID, len(viewBefore), len(viewAfterTab))
			}

			// "l" must not focus.
			dL := pressScrollRight(d)
			viewAfterL := stripAnsi(dL.View())
			if viewBefore != viewAfterL {
				t.Errorf("Bug F: event=%s: 'l' focused right column when all rows Count=0"+
					" — expected no view change",
					fixture.ID)
			}
		})
	}
}
