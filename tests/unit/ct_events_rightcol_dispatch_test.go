package unit_test

// ct_events_rightcol_dispatch_test.go — Layer 3 dispatch tests for the
// ct-events right-column row activation path.
//
// Assertion definitions:
//
//	D1: Pressing Enter on an actionable typed row (Count>0, ResourceIDs non-empty)
//	    dispatches a RelatedNavigateMsg with non-empty RelatedIDs and matching TargetType.
//
//	D2: Pressing Enter on a non-actionable row (Count=0, no FetchFilter)
//	    dispatches nothing — cmd is nil.
//
//	D3: Pressing Enter on a pivot row (Count=-1, FetchFilter non-empty, ResourceIDs
//	    empty) dispatches a RelatedNavigateMsg with non-empty FetchFilter and empty
//	    RelatedIDs. This catches the actionability guard bug where len(resourceIDs)>0
//	    was checked instead of len(fetchFilter)>0.
//
// Scope: all demo ct-events fixtures × all 17 registered RelatedDef groups.
// The test uses demo.GetResources("ct-events") — no hardcoded event IDs.

import (
	"context"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ctEventsRealCheckerResults runs all registered ct-events real checkers against
// the given resource and demo resource cache. ct-events checkers are pure
// field-readers (no AWS calls) so they work correctly with in-memory demo data.
// Source of truth: real checkers in internal/aws/ct_events_related.go.
func ctEventsRealCheckerResults(res resource.Resource, cache resource.ResourceCache) []resource.RelatedCheckResult {
	defs := resource.GetRelated("ct-events")
	results := make([]resource.RelatedCheckResult, 0, len(defs))
	for _, def := range defs {
		if def.Checker != nil {
			r := def.Checker(context.Background(), nil, res, cache)
			results = append(results, r)
		}
	}
	return results
}

// ---------------------------------------------------------------------------
// Dispatch-test helpers (local to this file)
// ---------------------------------------------------------------------------

// newCTEventsDetail creates a DetailModel for a ct-events resource with the
// right column auto-shown (width=200 triggers auto-show for registered defs).
func newCTEventsDetail(res resource.Resource) views.DetailModel {
	k := keys.Default()
	cfg := config.DefaultConfig()
	d := views.NewDetail(res, "ct-events", cfg, k)
	d.SetSize(200, 40)
	return d
}

// injectCTResult delivers a single RelatedCheckResultMsg into a ct-events
// DetailModel and returns the updated model.
func injectCTResult(d views.DetailModel, result resource.RelatedCheckResult) views.DetailModel {
	updated, _ := d.Update(messages.RelatedCheckResultMsg{
		ResourceType: "ct-events",
		Result:       result,
	})
	return updated
}

// ctFocusRightColumn focuses the right column using the ScrollRight ("l") key.
// The right column receives focus when HasActionableRows() is true, which is
// satisfied by any loading row.
func ctFocusRightColumn(d views.DetailModel) views.DetailModel {
	// "l" = ScrollRight — focuses right column when HasActionableRows
	updated, _ := d.Update(tea.KeyPressMsg{Code: -1, Text: "l"})
	return updated
}

// ctPressDownN presses the Down key n times on a focused right column.
func ctPressDownN(d views.DetailModel, n int) views.DetailModel {
	for range n {
		d, _ = d.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	return d
}

// ctExecuteCmd runs the tea.Cmd and returns the produced tea.Msg.
// Returns nil when cmd is nil.
func ctExecuteCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// ---------------------------------------------------------------------------
// TestCtEventsRightColumnDispatch
// ---------------------------------------------------------------------------

// TestCtEventsRightColumnDispatch iterates all demo ct-events fixtures × all
// 17 registered RelatedDef groups and asserts the dispatch invariants D1, D2, D3.
//
// For each (fixture, group) pair:
//  1. Build a DetailModel with that fixture.
//  2. Inject the demo checker result for that group only (other rows loading).
//  3. Focus the right column — ensureCursorValid() lands on the one actionable row.
//  4. Press Enter and inspect the dispatched cmd.
//
// D1: Count>0 → cmd returns RelatedNavigateMsg with non-empty RelatedIDs or FetchFilter.
// D2: Count=0 and no FetchFilter → pressing Enter at each row position dispatches no
//
//	RelatedNavigateMsg for this target type.
//
// D3: Count=-1 and FetchFilter non-empty → cmd returns RelatedNavigateMsg with
//
//	non-empty FetchFilter and EMPTY RelatedIDs.
func TestCtEventsRightColumnDispatch(t *testing.T) {
	ensureNoColor(t)

	fixtures := loadAllCTFixtures(t)

	defs := resource.GetRelated("ct-events")
	if len(defs) == 0 {
		t.Fatal("resource.GetRelated(\"ct-events\") returned no defs — RegisterRelated not called?")
	}

	// ct-events has no demo override: the real checkers are pure field-readers
	// and produce correct results when given a cache populated from demo fixtures.
	// Source of truth: internal/aws/ct_events_related.go.
	cache := buildFakeResourceCache(t)

	for _, fixture := range fixtures {
		t.Run(fixture.ID, func(t *testing.T) {
			// Get all real checker results for this fixture.
			allResults := ctEventsRealCheckerResults(fixture, cache)
			// Build a map targetType → result for O(1) lookup.
			resultByType := make(map[string]resource.RelatedCheckResult, len(allResults))
			for _, r := range allResults {
				resultByType[r.TargetType] = r
			}

			for _, def := range defs {
				t.Run(def.TargetType, func(t *testing.T) {
					label := fmt.Sprintf("event=%s group=%s", fixture.ID, def.TargetType)

					// Determine which result to inject for this specific group.
					result, hasResult := resultByType[def.TargetType]
					if !hasResult {
						// Demo checker returned no result for this group — skip.
						return
					}

					// Classify the result.
					isTypedHit := result.Count > 0
					isPivot := result.Count == -1 && len(result.FetchFilter) > 0
					isNotActionable := result.Count == 0 && len(result.FetchFilter) == 0

					if isNotActionable {
						// D2: Row with Count=0 and no FetchFilter must not dispatch
						// a RelatedNavigateMsg to this target type on Enter.
						// All rows remain at loading state except this one (Count=0).
						// ensureCursorValid skips Count=0 rows in favour of loading rows.
						// Navigate through all positions and verify no navigation to target type.
						d := newCTEventsDetail(fixture)
						d = injectCTResult(d, result)
						d = ctFocusRightColumn(d)
						for i := range len(defs) {
							if i > 0 {
								d = ctPressDownN(d, 1)
							}
							_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
							msg := ctExecuteCmd(cmd)
							if navMsg, ok2 := msg.(messages.RelatedNavigateMsg); ok2 {
								if navMsg.TargetType == def.TargetType {
									t.Errorf("D2 FAIL: Count=0 row dispatched RelatedNavigateMsg — %s"+
										" | dispatched TargetType=%q RelatedIDs=%v FetchFilter=%v",
										label, navMsg.TargetType, navMsg.RelatedIDs, navMsg.FetchFilter)
								}
							}
						}
						return
					}

					// For actionable rows: build model, inject only this result,
					// then focus. ensureCursorValid() moves cursor to the one
					// actionable row.
					d := newCTEventsDetail(fixture)
					d = injectCTResult(d, result)
					d = ctFocusRightColumn(d)

					// Press Enter to activate the selected row.
					_, cmd := d.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

					if isTypedHit {
						// D1: Count>0 typed row must dispatch RelatedNavigateMsg with
						// non-empty RelatedIDs (or FetchFilter for cache-miss path).
						if cmd == nil {
							t.Errorf("D1 FAIL: Count=%d typed row dispatched nil cmd — %s",
								result.Count, label)
							return
						}
						msg := ctExecuteCmd(cmd)
						navMsg, navOK := msg.(messages.RelatedNavigateMsg)
						if !navOK {
							t.Errorf("D1 FAIL: Count=%d typed row dispatched %T, want RelatedNavigateMsg — %s",
								result.Count, msg, label)
							return
						}
						if navMsg.TargetType != def.TargetType {
							t.Errorf("D1 FAIL: RelatedNavigateMsg.TargetType=%q, want %q — %s",
								navMsg.TargetType, def.TargetType, label)
						}
						if len(navMsg.RelatedIDs) == 0 && len(navMsg.FetchFilter) == 0 {
							t.Errorf("D1 FAIL: RelatedNavigateMsg has empty RelatedIDs AND empty FetchFilter — %s"+
								" | result.Count=%d result.ResourceIDs=%v",
								label, result.Count, result.ResourceIDs)
						}
						if navMsg.SourceResource.ID != fixture.ID {
							t.Errorf("D1 FAIL: RelatedNavigateMsg.SourceResource.ID=%q, want %q — %s",
								navMsg.SourceResource.ID, fixture.ID, label)
						}
					}

					if isPivot {
						// D3: Pivot row (Count=-1, FetchFilter non-empty, ResourceIDs empty)
						// must dispatch RelatedNavigateMsg with non-empty FetchFilter and
						// EMPTY RelatedIDs.
						// This catches the actionability guard bug where len(resourceIDs)>0
						// was checked instead of len(fetchFilter)>0.
						if cmd == nil {
							t.Errorf("D3 FAIL: pivot row (Count=-1, FetchFilter=%v) dispatched nil cmd"+
								" — right column actionability guard may be checking resourceIDs instead of fetchFilter — %s",
								result.FetchFilter, label)
							return
						}
						msg := ctExecuteCmd(cmd)
						navMsg, navOK := msg.(messages.RelatedNavigateMsg)
						if !navOK {
							t.Errorf("D3 FAIL: pivot row dispatched %T, want RelatedNavigateMsg — %s"+
								" | FetchFilter=%v",
								msg, label, result.FetchFilter)
							return
						}
						if navMsg.TargetType != def.TargetType {
							t.Errorf("D3 FAIL: RelatedNavigateMsg.TargetType=%q, want %q — %s",
								navMsg.TargetType, def.TargetType, label)
						}
						if len(navMsg.FetchFilter) == 0 {
							t.Errorf("D3 FAIL: pivot RelatedNavigateMsg.FetchFilter is empty, want non-empty — %s"+
								" | result.FetchFilter=%v",
								label, result.FetchFilter)
						}
						if len(navMsg.RelatedIDs) != 0 {
							t.Errorf("D3 FAIL: pivot RelatedNavigateMsg.RelatedIDs=%v, want empty — %s",
								navMsg.RelatedIDs, label)
						}
						// Verify FetchFilter keys/values are preserved exactly.
						for k, v := range result.FetchFilter {
							got := navMsg.FetchFilter[k]
							if got != v {
								t.Errorf("D3 FAIL: FetchFilter[%q]=%q, want %q — %s",
									k, got, v, label)
							}
						}
					}
				})
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestCtEventsRightColumnDispatch_LoadingRowNotActionable
// ---------------------------------------------------------------------------

// TestCtEventsRightColumnDispatch_LoadingRowNotActionable verifies that loading
// rows (no RelatedCheckResultMsg delivered) do NOT dispatch a RelatedNavigateMsg
// when Enter is pressed. Loading rows are focusable but not navigable.
func TestCtEventsRightColumnDispatch_LoadingRowNotActionable(t *testing.T) {
	ensureNoColor(t)

	fixtures := loadAllCTFixtures(t)

	// Use the first fixture — all rows stay in loading state (no results injected).
	fixture := fixtures[0]
	d := newCTEventsDetail(fixture)
	// Do NOT inject any results — all rows stay in loading state.
	d = ctFocusRightColumn(d)

	defs := resource.GetRelated("ct-events")
	for i := range len(defs) {
		current := d
		if i > 0 {
			current = ctPressDownN(d, i)
		}
		_, cmd := current.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		msg := ctExecuteCmd(cmd)
		if _, ok2 := msg.(messages.RelatedNavigateMsg); ok2 {
			t.Errorf("loading row at position %d dispatched RelatedNavigateMsg — loading rows must not be navigable (event=%s)",
				i, fixture.ID)
		}
	}
}
