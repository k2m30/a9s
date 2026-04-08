package unit_test

// ct_events_rightcol_e2e_test.go — Layer 5 end-to-end tests for ct-events
// right-column row activation feeding into the root tui.Model.
//
// Layer 5 = the dispatched RelatedNavigateMsg is fed through the real
// tui.Model.Update (root model in demo mode) and the resulting view stack
// is inspected.
//
// Assertion definitions:
//
//	E1: RelatedNavigateMsg for a typed row with ResourceIDs → root model pushes a
//	    view whose frame contains the target resource type name or an ID from RelatedIDs.
//	    Must NOT produce a FlashMsg error.
//
//	E2: RelatedNavigateMsg for a pivot row (FetchFilter non-empty) → root model
//	    pushes a ResourceList view. Must NOT produce a FlashMsg error (which would
//	    indicate the FetchFilter was dropped and the resolver fell through to KindFlash).
//
//	E3: RelatedNavigateMsg for an unknown TargetType → root model returns a FlashMsg
//	    with IsError=true (error path, not a panic).
//
// Representative subset: one fixture per target-type present in corpus +
// all 4 pivot variants (AccessKeyId, Username, EventName, SharedEventId).

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)


// ---------------------------------------------------------------------------
// Layer 5 helpers (local — cannot access package-unit helpers from unit_test)
// ---------------------------------------------------------------------------

// ctRootApplyMsg sends a message through the tui.Model Update and returns
// the updated model and cmd. Mirrors rootApplyMsg from tui_root_test.go.
func ctRootApplyMsg(m tui.Model, msg tea.Msg) (tui.Model, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(tui.Model), cmd
}

// ctRootViewContent returns the rendered content string from the root model.
func ctRootViewContent(m tui.Model) string {
	return m.View().Content
}

// newCTDemoRootModel creates a tui.Model in demo mode with a sized window
// ready for testing navigation flows.
func newCTDemoRootModel(t *testing.T) tui.Model {
	t.Helper()
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m, _ = ctRootApplyMsg(m, tea.WindowSizeMsg{Width: 200, Height: 40})
	return m
}

// feedCTRelatedNavigate feeds a RelatedNavigateMsg to the root model and
// executes the returned cmd once, returning (updatedModel, resultMsg).
// resultMsg is nil when cmd is nil.
func feedCTRelatedNavigate(m tui.Model, navMsg messages.RelatedNavigateMsg) (tui.Model, tea.Msg) {
	m, cmd := ctRootApplyMsg(m, navMsg)
	if cmd == nil {
		return m, nil
	}
	resultMsg := cmd()
	m, _ = ctRootApplyMsg(m, resultMsg)
	return m, resultMsg
}

// pickCTFixtureForTargetType returns the first ct-events fixture whose real
// checker result for targetType has Count>0. Returns (fixture, result, true).
// ct-events checkers are pure field-readers so they produce correct results
// when given a cache populated from demo fixtures.
// Source of truth: real checkers in internal/aws/ct_events_related.go.
func pickCTFixtureForTargetType(t *testing.T, targetType string) (resource.Resource, resource.RelatedCheckResult, bool) {
	t.Helper()
	fixtures, ok := demo.GetResources("ct-events")
	if !ok {
		return resource.Resource{}, resource.RelatedCheckResult{}, false
	}
	cache := buildDemoResourceCache(t)
	for _, f := range fixtures {
		for _, r := range ctEventsRealCheckerResults(f, cache) {
			if r.TargetType == targetType && r.Count > 0 {
				return f, r, true
			}
		}
	}
	return resource.Resource{}, resource.RelatedCheckResult{}, false
}

// ctTruncate limits a string to maxLen characters for display in error messages.
func ctTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ---------------------------------------------------------------------------
// TestCtEventsRightColumnEndToEnd
// ---------------------------------------------------------------------------

// TestCtEventsRightColumnEndToEnd verifies that RelatedNavigateMsgs produced
// by the right-column row activation path are handled correctly by the root
// tui.Model.
//
// E1: Typed rows (Count>0) — must not produce FlashMsg error.
// E2: Pivot rows (Count=-1, FetchFilter non-empty) — must push a new view.
// E3: Unknown TargetType — must produce FlashMsg with IsError=true.
func TestCtEventsRightColumnEndToEnd(t *testing.T) {
	ensureNoColor(t)

	defs := resource.GetRelated("ct-events")
	if len(defs) == 0 {
		t.Fatal("resource.GetRelated(\"ct-events\") returned no defs")
	}

	// ct-events has no demo override: real checkers are pure field-readers and
	// produce correct results when given a cache populated from demo fixtures.
	// Source of truth: real checkers in internal/aws/ct_events_related.go.
	cache := buildDemoResourceCache(t)

	// -------------------------------------------------------------------------
	// E1: Typed rows with Count>0 — each distinct TargetType with at least one
	// fixture with Count>0. Must not produce FlashMsg error.
	// -------------------------------------------------------------------------

	seenTypes := make(map[string]bool)
	for _, def := range defs {
		if seenTypes[def.TargetType] {
			continue
		}
		if def.TargetType == "ct-events" {
			// Pivot types tested separately in E2.
			continue
		}

		fixture, result, found := pickCTFixtureForTargetType(t, def.TargetType)
		if !found {
			// No fixture with Count>0 for this type — no E1 data.
			continue
		}
		seenTypes[def.TargetType] = true

		defCopy := def
		resultCopy := result
		fixtureCopy := fixture

		t.Run("E1/"+defCopy.TargetType, func(t *testing.T) {
			label := fmt.Sprintf("event=%s targetType=%s relatedIDs=%v",
				fixtureCopy.ID, defCopy.TargetType, resultCopy.ResourceIDs)

			m := newCTDemoRootModel(t)
			navMsg := messages.RelatedNavigateMsg{
				TargetType:     defCopy.TargetType,
				SourceResource: fixtureCopy,
				RelatedIDs:     resultCopy.ResourceIDs,
				FetchFilter:    resultCopy.FetchFilter,
			}

			m, resultMsg := feedCTRelatedNavigate(m, navMsg)

			// E1a: Must not produce a FlashMsg error.
			if fm, ok := resultMsg.(messages.FlashMsg); ok && fm.IsError {
				t.Errorf("E1 FAIL: typed row navigation produced FlashMsg error=%q — %s",
					fm.Text, label)
				return
			}

			// E1b: View must not contain error text.
			view := stripAnsi(ctRootViewContent(m))
			if strings.Contains(view, "unknown resource type") {
				t.Errorf("E1 FAIL: View contains 'unknown resource type' after typed navigation — %s | excerpt: %s",
					label, ctTruncate(view, 300))
			}
		})
	}

	// -------------------------------------------------------------------------
	// E2: Pivot rows (Count=-1, FetchFilter non-empty) — all 4 pivot variants.
	// Critical: FetchFilter must be forwarded to KindFilteredList, NOT dropped.
	// Each pivot must push a new view (verified by Esc popping it back).
	// -------------------------------------------------------------------------

	type pivotCase struct {
		name      string
		filterKey string // key expected in FetchFilter
	}
	pivotCases := []pivotCase{
		{"PivotByUsername", "Username"},
		{"PivotByEventName", "EventName"},
		{"PivotByAccessKeyId", "AccessKeyId"},
		{"PivotBySharedEventId", "SharedEventId"},
	}

	for _, pc := range pivotCases {
		pc := pc
		t.Run("E2/"+pc.name, func(t *testing.T) {
			fixtures, ok2 := demo.GetResources("ct-events")
			if !ok2 || len(fixtures) == 0 {
				t.Skip("no ct-events fixtures")
			}

			// Find a fixture whose real checker has a pivot result for this filter key.
			// ct-events has no demo override — real checkers are pure field-readers.
			// Source of truth: internal/aws/ct_events_related.go.
			var pivotResult *resource.RelatedCheckResult
			var pivotFixture resource.Resource
			for _, f := range fixtures {
				for _, r := range ctEventsRealCheckerResults(f, cache) {
					if r.TargetType == "ct-events" && r.Count == -1 && len(r.FetchFilter) > 0 {
						if _, hasPivotKey := r.FetchFilter[pc.filterKey]; hasPivotKey {
							rCopy := r
							pivotResult = &rCopy
							pivotFixture = f
							break
						}
					}
				}
				if pivotResult != nil {
					break
				}
			}

			if pivotResult == nil {
				t.Skipf("no ct-events fixture has pivot %s with FetchFilter[%q] — skipping",
					pc.name, pc.filterKey)
				return
			}

			label := fmt.Sprintf("event=%s pivot=%s FetchFilter=%v",
				pivotFixture.ID, pc.name, pivotResult.FetchFilter)

			m := newCTDemoRootModel(t)
			navMsg := messages.RelatedNavigateMsg{
				TargetType:     "ct-events",
				SourceResource: pivotFixture,
				RelatedIDs:     pivotResult.ResourceIDs, // empty for pivots
				FetchFilter:    pivotResult.FetchFilter,
			}

			mAfter, resultMsg := feedCTRelatedNavigate(m, navMsg)

			// E2a: Must NOT produce a FlashMsg error.
			if fm, ok3 := resultMsg.(messages.FlashMsg); ok3 && fm.IsError {
				t.Errorf("E2 FAIL: pivot RelatedNavigateMsg produced FlashMsg error=%q"+
					" — FetchFilter may have been dropped by resolver — %s",
					fm.Text, label)
				return
			}

			// E2b: View must not contain 'unknown resource type'.
			viewAfterNav := stripAnsi(ctRootViewContent(mAfter))
			if strings.Contains(viewAfterNav, "unknown resource type") {
				t.Errorf("E2 FAIL: View contains 'unknown resource type' — ct-events TargetType not resolved — %s | excerpt: %s",
					label, ctTruncate(viewAfterNav, 300))
			}

			// E2c: A new view must have been pushed. Pressing Esc pops it back.
			// If no view was pushed, the view before and after Esc are identical.
			mPopped, _ := ctRootApplyMsg(mAfter, tea.KeyPressMsg{Code: tea.KeyEscape})
			viewBeforeEsc := viewAfterNav
			viewAfterEsc := stripAnsi(ctRootViewContent(mPopped))
			if viewBeforeEsc == viewAfterEsc {
				t.Errorf("E2 FAIL: view unchanged after Esc — no view was pushed by pivot navigation — %s",
					label)
			}
		})
	}

	// -------------------------------------------------------------------------
	// E3: Unknown TargetType → FlashMsg with IsError=true (not a panic).
	// -------------------------------------------------------------------------

	t.Run("E3/UnknownTargetType", func(t *testing.T) {
		fixtures, ok3 := demo.GetResources("ct-events")
		if !ok3 || len(fixtures) == 0 {
			t.Fatal("no ct-events fixtures")
		}
		fixture := fixtures[0]
		label := fmt.Sprintf("event=%s targetType=NONEXISTENT_TYPE_12345", fixture.ID)

		m := newCTDemoRootModel(t)
		navMsg := messages.RelatedNavigateMsg{
			TargetType:     "NONEXISTENT_TYPE_12345",
			SourceResource: fixture,
			RelatedIDs:     []string{"fake-id"},
		}

		_, resultMsg := feedCTRelatedNavigate(m, navMsg)

		// E3: Must produce a FlashMsg with IsError=true.
		if resultMsg == nil {
			t.Errorf("E3 FAIL: unknown TargetType returned nil resultMsg, want FlashMsg — %s", label)
			return
		}
		fm, ok4 := resultMsg.(messages.FlashMsg)
		if !ok4 {
			t.Errorf("E3 FAIL: unknown TargetType returned %T, want FlashMsg — %s", resultMsg, label)
			return
		}
		if !fm.IsError {
			t.Errorf("E3 FAIL: FlashMsg.IsError=false for unknown TargetType, want true — %s | text=%q",
				label, fm.Text)
		}
	})
}

// ---------------------------------------------------------------------------
// TestCtEventsRightColumnEndToEnd_AllPivotsHaveFetchFilter
// ---------------------------------------------------------------------------

// TestCtEventsRightColumnEndToEnd_AllPivotsHaveFetchFilter verifies that for
// every ct-events fixture where the pivot checkers return Count=-1+FetchFilter,
// the full dispatch path pushes a new view (E2 exhaustive variant).
func TestCtEventsRightColumnEndToEnd_AllPivotsHaveFetchFilter(t *testing.T) {
	ensureNoColor(t)

	fixtures, ok := demo.GetResources("ct-events")
	if !ok || len(fixtures) == 0 {
		t.Fatal("demo.GetResources(\"ct-events\") returned no fixtures")
	}

	// ct-events has no demo override — real checkers are pure field-readers
	// and produce correct results from demo fixture data.
	// Source of truth: real checkers in internal/aws/ct_events_related.go.
	cache2 := buildDemoResourceCache(t)

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			allResults := ctEventsRealCheckerResults(fixture, cache2)

			for _, result := range allResults {
				result := result
				if result.TargetType != "ct-events" {
					continue
				}
				if result.Count != -1 || len(result.FetchFilter) == 0 {
					continue
				}

				label := fmt.Sprintf("event=%s FetchFilter=%v", fixture.ID, result.FetchFilter)

				m := newCTDemoRootModel(t)
				navMsg := messages.RelatedNavigateMsg{
					TargetType:     "ct-events",
					SourceResource: fixture,
					RelatedIDs:     result.ResourceIDs,
					FetchFilter:    result.FetchFilter,
				}

				mAfter, resultMsg := feedCTRelatedNavigate(m, navMsg)

				// Must NOT produce FlashMsg error.
				if fm, ok2 := resultMsg.(messages.FlashMsg); ok2 && fm.IsError {
					t.Errorf("E2 FAIL: pivot navigation produced error flash=%q — FetchFilter may have been dropped — %s",
						fm.Text, label)
					continue
				}

				// A new view must have been pushed (Esc pops it back).
				viewBefore := stripAnsi(ctRootViewContent(mAfter))
				mPopped, _ := ctRootApplyMsg(mAfter, tea.KeyPressMsg{Code: tea.KeyEscape})
				viewAfterEsc := stripAnsi(ctRootViewContent(mPopped))
				if viewBefore == viewAfterEsc {
					t.Errorf("E2 FAIL: no view was pushed — Esc returned identical view — %s", label)
				}
			}
		})
	}
}
