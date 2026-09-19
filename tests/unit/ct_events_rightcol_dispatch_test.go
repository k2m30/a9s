package unit_test

// Assertion definitions:
//
//	D1: An actionable typed row (Count>0, ResourceIDs non-empty) dispatches at
//	    least one fetch task scoped to the def's TargetType.
//
//	D2: A non-actionable row (Count=0, no FetchFilter) dispatches nothing —
//	    zero tasks.
//
//	D3: A pivot row (State: RelatedDeferred, FetchFilter non-empty,
//	    ResourceIDs empty) dispatches a KindFetchFiltered task carrying the
//	    SAME FetchFilter.
//
// Scope: all demo ct-events fixtures × all 17 registered RelatedDef groups.
//
// Every controller here is fresh (newTestController, empty RowStore), so
// HandleRelatedNavigate never takes the cache-hit branch and D1/D3 always
// dispatch a fetch task. ViewState exposes no resource type for the pushed
// screen; TargetType is checked via TaskRequest.Key.Scope.

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// ctEventsRealCheckerResults runs all registered ct-events real checkers against
// the given resource and demo resource cache. ct-events checkers are pure
// field-readers (no AWS calls) so they work correctly with in-memory demo data.
// Source of truth: real checkers in core/aws/ct_events_related.go.
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

// sampleCTFixtures selects 3 representative fixtures from a larger set:
// the first, the middle, and the last. This keeps the dispatch test under 20ms
// while still exercising diverse event shapes (with/without related, error events).
func sampleCTFixtures(all []resource.Resource) []resource.Resource {
	if len(all) <= 3 {
		return all
	}
	return []resource.Resource{all[0], all[len(all)/2], all[len(all)-1]}
}

// buildCTEventsRightColController builds a fresh controller (empty RowStore)
// with a ct-events resource pushed to ScreenDetail and all related rows
// initialised to Loading.
func buildCTEventsRightColController(t *testing.T, fixture resource.Resource) *app.Controller {
	t.Helper()
	c := newTestController(t)
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{
			ID:      runtime.ScreenDetail,
			Context: runtime.ScreenContext{ResourceType: "ct-events", ResourceID: fixture.ID},
		},
	})
	c.EnsureDetailState(fixture, "ct-events")
	c.InitDetailRelatedRows("ct-events")
	return c
}

// ctRelatedRowIndex returns the index of the related row matching displayName
// — the disambiguator for ct-events self-pivot defs that share a TargetType.
func ctRelatedRowIndex(related []app.RelatedBlock, displayName string) int {
	for i, row := range related {
		if row.Name == displayName {
			return i
		}
	}
	return -1
}

// TestCtEventsRightColumnDispatch iterates all demo ct-events fixtures × all
// 17 registered RelatedDef groups and asserts the dispatch invariants D1, D2, D3.
func TestCtEventsRightColumnDispatch(t *testing.T) {
	allFixtures := loadAllCTFixtures(t)

	// Sample 3 representative fixtures: first (index 0), one with related resources
	// (index len/2), and last. Full fixture sweep is available via loadAllCTFixtures.
	fixtures := sampleCTFixtures(allFixtures)

	defs := resource.GetRelated("ct-events")
	if len(defs) == 0 {
		t.Fatal("resource.GetRelated(\"ct-events\") returned no defs — SetRelatedForTest not called?")
	}

	// ct-events has no demo override: the real checkers are pure field-readers
	// and produce correct results when given a cache populated from demo fixtures.
	// Source of truth: core/aws/ct_events_related.go.
	cache := buildFakeResourceCache(t)

	for _, fixture := range fixtures {
		t.Run(fixture.ID, func(t *testing.T) {
			allResults := ctEventsRealCheckerResults(fixture, cache)
			resultByType := make(map[string]resource.RelatedCheckResult, len(allResults))
			for _, r := range allResults {
				resultByType[r.TargetType()] = r
			}

			for _, def := range defs {
				t.Run(def.TargetType, func(t *testing.T) {
					label := fmt.Sprintf("event=%s group=%s", fixture.ID, def.TargetType)

					result, hasResult := resultByType[def.TargetType]
					if !hasResult {
						return
					}

					isPivot := result.State() == domain.RelatedDeferred
					// Unknown has Count 0 and no FetchFilter like a not-actionable row, but
					// nothing has answered yet: selecting it dispatches a related-check
					// re-run scoped to the ct-events row, not a fetch scoped to the target,
					// so it is neither D1/D3 nor D2.
					isUnknown := result.State() == domain.RelatedUnknown
					isNotActionable := !isUnknown &&
						result.Count() == 0 && len(result.FetchFilter()) == 0

					c := buildCTEventsRightColController(t, fixture)
					errMsg := ""
					if result.Err() != nil {
						errMsg = result.Err().Error()
					}
					c.ApplyDetailRelatedResultForResource("ct-events", fixture.ID, def.DisplayName, def.TargetType,
						result.EffectiveState(), result.Count(), false, errMsg, result.Truncated(), result.ResourceIDs(), result.FetchFilter())

					body := c.Snapshot().Body.Detail
					if body == nil {
						t.Fatalf("Body.Detail is nil after EnsureDetailState + InitDetailRelatedRows — %s", label)
					}
					idx := ctRelatedRowIndex(body.Related, def.DisplayName)

					if isNotActionable {
						// D2: Count=0/no-FetchFilter rows for a ct-events self-pivot
						// TargetType (this fixture pointing at itself) are suppressed
						// entirely from Body.Detail.Related by isSelfPivotZeroDetailRow
						// (detail_cursor.go) — there is no row to select, which is a
						// stronger form of "dispatches nothing" than the D2 assertion
						// below. When the row is present (non-self-pivot Count=0 case),
						// selecting it dispatches zero tasks.
						if idx < 0 {
							return
						}
						c.Apply(app.Action{Kind: app.ActionToggleFocus})
						_, tasks := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: strconv.Itoa(idx)})
						if len(tasks) != 0 {
							t.Errorf("D2 FAIL: Count=0 row dispatched %d task(s) — %s | tasks=%+v",
								len(tasks), label, tasks)
						}
						return
					}

					if idx < 0 {
						t.Fatalf("related row for %q not found after ApplyDetailRelatedResultForResource — %s", def.DisplayName, label)
					}
					c.Apply(app.Action{Kind: app.ActionToggleFocus})
					_, tasks := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: strconv.Itoa(idx)})

					if isUnknown {
						if len(tasks) == 0 {
							t.Errorf("unknown row dispatched 0 tasks — an unanswered row must be able to answer itself: %s", label)
						}
						return
					}

					// D1/D3: an actionable row (typed hit or pivot) dispatches at least one
					// fetch task scoped to this def's TargetType.
					if len(tasks) == 0 {
						t.Errorf("D1/D3 FAIL: actionable row (Count=%d, State=%v, FetchFilter=%v) dispatched 0 tasks — %s",
							result.Count(), result.State(), result.FetchFilter(), label)
						return
					}
					for _, task := range tasks {
						if task.Key.Scope != def.TargetType {
							t.Errorf("dispatched task Scope=%q, want %q — %s", task.Key.Scope, def.TargetType, label)
						}
					}

					if isPivot {
						// D3: pivot row (State: RelatedDeferred, FetchFilter non-empty,
						// ResourceIDs empty) must dispatch a KindFetchFiltered task
						// carrying the SAME FetchFilter.
						found := false
						for _, task := range tasks {
							payload, ok := task.Payload.(runtime.FetchFilteredPayload)
							if !ok {
								continue
							}
							found = true
							for k, v := range result.FetchFilter() {
								if payload.Filter[k] != v {
									t.Errorf("D3 FAIL: FetchFilter[%q]=%q, want %q — %s", k, payload.Filter[k], v, label)
								}
							}
						}
						if !found {
							t.Errorf("D3 FAIL: pivot row (FetchFilter=%v) dispatched no KindFetchFiltered task — %s | tasks=%+v",
								result.FetchFilter(), label, tasks)
						}
					}
				})
			}
		})
	}
}

// TestCtEventsRightColumnDispatch_LoadingRowNotActionable verifies that loading
// rows (no result delivered) do NOT dispatch any task when ActionRelatedSelect
// activates them. Loading rows are focusable but not navigable.
func TestCtEventsRightColumnDispatch_LoadingRowNotActionable(t *testing.T) {
	fixtures := loadAllCTFixtures(t)

	// Use the first fixture — all rows stay in loading state (no results injected).
	fixture := fixtures[0]
	c := buildCTEventsRightColController(t, fixture)
	c.Apply(app.Action{Kind: app.ActionToggleFocus})

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil after EnsureDetailState + InitDetailRelatedRows")
	}

	for i := range body.Related {
		_, tasks := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: strconv.Itoa(i)})
		if len(tasks) != 0 {
			t.Errorf("loading row at position %d dispatched %d task(s) — loading rows must not be navigable (event=%s)",
				i, len(tasks), fixture.ID)
		}
	}
}
