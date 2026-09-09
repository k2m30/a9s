// costs_delivery_matrix_test.go — M1: the delivery-state completeness net
// over messages.CostsLoaded's honest GridResult/anomaly shapes crossed with
// the frame states that give each combination distinct, observable
// behavior. package unit_test (headless-only; reuses newCostsController /
// topDrill / findFetchCostsTask / fullMetricRecord / codexMoveCursorToRow,
// same helpers costs_review3_test.go and siblings already share).
//
// Grid and Anomalies are INDEPENDENT axes at the executor (SkipGrid and
// SkipAnomalies are two separate booleans, FetchCostsPayload), so the
// dispatch's nominal {grid}x{anomalies} cross product is real — except for
// "failed", which is NOT a per-half state: messages.CostsLoaded carries one
// Err field for the WHOLE delivery (ApplyCostsLoaded returns before ever
// looking at Grid/Anomalies when ev.Err != nil), so a "grid failed, anomaly
// fetched" cell does not exist in the current wire shape. Err is tested as
// its own axis (matching/not-matching frame), not crossed into grid x
// anomaly. "both skipped" (grid AND anomalies skipped in the same delivery)
// is marked unreachable: ensureCostsShapeFetched never dispatches a fetch
// with nothing to fetch.
//
// Each of the 8 reachable non-error grid x anomaly combinations is paired
// with the ONE frame state that makes its distinguishing behavior
// observable (a full 8x6-frame cross product would mostly re-assert the
// same "Loading clears, Rows update" shape already pinned per-combination
// here) — this is the completeness net over the DISTINCT combinations the
// architecture actually produces, not a mechanical enumeration of every
// frame against every combination.
package unit_test

import (
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

func m1AnomalyMark(period costs.Period, service string, impact float64) costs.AnomalyMark {
	return costs.AnomalyMark{
		Impact:    costs.Amount{Value: impact, Unit: "USD"},
		Period:    period,
		Dimension: map[costs.Dimension]string{costs.DimensionService: service},
	}
}

// ===========================================================================
// Cell 1 — grid:fetched-with-records x anomalies:fetched-with-marks
// @ fresh-loading (root shape-miss). Both halves genuinely ran together.
// ===========================================================================

func TestCostsDeliveryMatrix_Grid_RecordsAndAnomalies_Marks_FreshLoading(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	newestCol := root.Window[len(root.Window)-1]

	c.Handle(messages.CostsLoaded{
		Query: costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}},
		Grid: costs.GridResult{
			Fetched: true,
			Records: []costs.Record{fullMetricRecord(newestCol, "Amazon EC2", 1200.0)},
		},
		Window:    root.Window,
		Anomalies: []costs.AnomalyMark{m1AnomalyMark(newestCol, "Amazon EC2", 405)},
		Requests:  2,
	})

	vs := c.Snapshot()
	if vs.Body.Costs.Loading {
		t.Error("Loading still true after a matching fetched+marks delivery")
	}
	if len(vs.Body.Costs.Rows) == 0 {
		t.Error("no rows after a matching fetched-with-records delivery")
	}
	if !strings.Contains(vs.Body.Costs.FooterNote, "405") {
		t.Errorf("FooterNote %q does not mention the delivered anomaly's impact (405)", vs.Body.Costs.FooterNote)
	}
}

// ===========================================================================
// Cell 2 — grid:fetched-with-records x anomalies:fetched-empty
// @ fresh-loading. Grid data lands; anomalies authoritatively found none —
// FooterNote must not fabricate an anomaly mention.
// ===========================================================================

func TestCostsDeliveryMatrix_Grid_Records_AnomaliesEmpty_FreshLoading(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	newestCol := root.Window[len(root.Window)-1]

	c.Handle(messages.CostsLoaded{
		Query: costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}},
		Grid: costs.GridResult{
			Fetched: true,
			Records: []costs.Record{fullMetricRecord(newestCol, "Amazon EC2", 1200.0)},
		},
		Window:    root.Window,
		Anomalies: []costs.AnomalyMark{},
		Requests:  2,
	})

	vs := c.Snapshot()
	if len(vs.Body.Costs.Rows) == 0 {
		t.Error("no rows after a matching fetched-with-records delivery")
	}
	if strings.Contains(vs.Body.Costs.FooterNote, "$") {
		t.Errorf("FooterNote %q mentions an anomaly amount after an authoritative zero-anomalies delivery", vs.Body.Costs.FooterNote)
	}
}

// ===========================================================================
// Cell 3 — grid:fetched-with-records x anomalies:skipped (TTL-fresh)
// @ warm-covered re-fetch. A same-shape refresh that skips the anomaly slot
// must leave any existing anomaly FooterNote untouched (never clobbered by
// the absent anomaly half).
// ===========================================================================

func TestCostsDeliveryMatrix_Grid_Records_AnomaliesSkipped_WarmRefresh(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	newestCol := root.Window[len(root.Window)-1]
	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}

	c.Handle(messages.CostsLoaded{
		Query:     q,
		Grid:      costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newestCol, "Amazon EC2", 1200.0)}},
		Window:    root.Window,
		Anomalies: []costs.AnomalyMark{m1AnomalyMark(newestCol, "Amazon EC2", 405)},
		Requests:  2,
	})
	if got := c.Snapshot().Body.Costs.FooterNote; !strings.Contains(got, "405") {
		t.Fatalf("precondition: first delivery's anomaly did not land in FooterNote, got %q", got)
	}

	// Second delivery: grid re-fetched with fresh records, anomaly slot
	// skipped (Anomalies left nil — TTL still fresh, X2's own convention).
	c.Handle(messages.CostsLoaded{
		Query:    q,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newestCol, "Amazon EC2", 1300.0)}},
		Window:   root.Window,
		Requests: 1,
	})

	if got := c.Snapshot().Body.Costs.FooterNote; !strings.Contains(got, "405") {
		t.Errorf("FooterNote lost the still-fresh $405 anomaly after a grid-only refresh (Anomalies skipped) — got %q", got)
	}
}

// ===========================================================================
// Cell 4 — grid:fetched-empty x anomalies:fetched-with-marks
// @ fallback-eligible-drill. The N3 trigger: grid genuinely re-confirmed
// zero at an eligible child frame, and separately anomalies fetched marks
// in the SAME delivery — the fallback must still fire (grid authority is
// what matters, not whether anomalies also rode along).
// ===========================================================================

func TestCostsDeliveryMatrix_GridEmpty_AnomaliesMarks_FallbackFires(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	newestCol := root.Window[len(root.Window)-1]

	c.Handle(messages.CostsLoaded{
		Query:    costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}},
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newestCol, "Amazon EC2", 250)}},
		Requests: 1,
	})
	if !codexMoveCursorToRow(c, "EC2") {
		t.Fatal("precondition: no SERVICE row for the seeded EC2 record")
	}
	codexMoveCursorToNewestColumn(c)

	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect}) // -> USAGE_TYPE child, Fallback.Eligible=true
	childPayload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: drilling did not dispatch a fetch task")
	}

	_, fallbackTasks := c.Handle(messages.CostsLoaded{
		Query:     childPayload.Query,
		Grid:      costs.GridResult{Fetched: true},
		Window:    childPayload.Window,
		Anomalies: []costs.AnomalyMark{m1AnomalyMark(newestCol, "Amazon EC2", 50)},
		Requests:  2,
	})

	if _, found := findFetchCostsTask(fallbackTasks); !found {
		t.Error("grid:fetched-empty + anomalies:fetched-with-marks in the SAME delivery did not fire the N3 fallback — grid authority must decide this independent of the anomaly half riding along")
	}
}

// ===========================================================================
// Cell 5 — grid:fetched-empty x anomalies:skipped @ warm-covered. Grid
// genuinely re-confirmed zero for an already-warm bucket — MergeCoverage
// still stamps (an authoritative zero is still authoritative), the row
// clears, and the untouched anomaly slot leaves any existing note alone.
// ===========================================================================

func TestCostsDeliveryMatrix_GridEmpty_AnomaliesSkipped_WarmBucket_StampsCoverage(t *testing.T) {
	// The seed is fetched STRICTLY EARLIER than fixedCostsNow (still within
	// the open-period TTL) — MergeCoverage's own
	// `!existing.FetchedAt.Equal(now)` guard needs two genuinely different
	// "now" stamps to tell "an earlier warm fetch" from "this same round"
	// apart; reusing one fixed clock for both deliveries would silently
	// mask the very zero-clear behavior this cell exists to pin (mirrors
	// costs_review3_test.go's own P1 R2 setup, same reason).
	profile := "test-profile-matrix-cell5"
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	seedNow := fixedCostsNow.Add(-2 * time.Hour)
	window := costs.BuildWindow(costs.GranularityMonth, fixedCostsNow)
	newestCol := window[len(window)-1]
	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}

	seed := costs.LoadStore(profile)
	seed.Merge(q, []costs.Record{fullMetricRecord(newestCol, "Amazon EC2", 1200.0)}, seedNow)
	seed.MergeCoverage(q, window, seedNow)
	if err := seed.Save(); err != nil {
		t.Fatalf("seeding on-disk cost cache: %v", err)
	}

	s := session.New()
	s.Profile = profile
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c.EnsureCostsState(fixedCostsNow) // LATER "now" than the seed

	if len(c.Snapshot().Body.Costs.Rows) == 0 {
		t.Fatal("precondition: warm grid loaded from disk did not render the seeded EC2 row")
	}

	// A later re-check of the SAME shape genuinely re-runs the grid and
	// authoritatively finds zero this time (e.g. the resource was deleted).
	c.Handle(messages.CostsLoaded{
		Query:    q,
		Grid:     costs.GridResult{Fetched: true},
		Window:   window,
		Requests: 1,
	})

	if len(c.Snapshot().Body.Costs.Rows) != 0 {
		t.Error("grid:fetched-empty for an already-warm bucket did not clear the now-stale EC2 row — an authoritative re-confirmed zero must overwrite warm data, not be ignored as a non-event")
	}
}

// ===========================================================================
// Cell 6 — grid:skipped x anomalies:fetched-with-marks @ warm-covered
// (P1's own canonical cell, placed here as the matrix's authoritative
// anomaly-only-refresh entry). MergeCoverage must NOT be touched; warm
// records survive; the new marks land.
// ===========================================================================

func TestCostsDeliveryMatrix_GridSkipped_AnomaliesMarks_WarmBucket_NeverClearsGrid(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	newestCol := root.Window[len(root.Window)-1]
	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}

	c.Handle(messages.CostsLoaded{
		Query:    q,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newestCol, "Amazon EC2", 1200.0)}},
		Window:   root.Window,
		Requests: 1,
	})

	// Anomaly-only refresh: grid slot untouched (Fetched:false, Records
	// nil), anomalies genuinely re-fetched with fresh marks.
	c.Handle(messages.CostsLoaded{
		Query:     q,
		Grid:      costs.GridResult{Fetched: false},
		Window:    root.Window,
		Anomalies: []costs.AnomalyMark{m1AnomalyMark(newestCol, "Amazon EC2", 75)},
		Requests:  1,
	})

	vs := c.Snapshot()
	if len(vs.Body.Costs.Rows) == 0 || vs.Body.Costs.Rows[0].Label != "EC2" {
		t.Errorf("an anomaly-only delivery wiped the warm EC2 row — got %+v", vs.Body.Costs.Rows)
	}
	if !strings.Contains(vs.Body.Costs.FooterNote, "75") {
		t.Errorf("FooterNote %q does not mention the anomaly-only delivery's own $75 mark", vs.Body.Costs.FooterNote)
	}
}

// ===========================================================================
// Cell 7 — grid:skipped x anomalies:fetched-empty @ warm-covered. An
// anomaly-only delivery that authoritatively found ZERO anomalies must
// clear a stale FooterNote — PutAnomalies runs unconditionally on success,
// per costs_selfreview_test.go's own X-series pin, placed here as its
// matrix-cell counterpart.
// ===========================================================================

func TestCostsDeliveryMatrix_GridSkipped_AnomaliesEmpty_WarmBucket_ClearsStaleNote(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	newestCol := root.Window[len(root.Window)-1]
	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}

	c.Handle(messages.CostsLoaded{
		Query:     q,
		Grid:      costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newestCol, "Amazon EC2", 1200.0)}},
		Window:    root.Window,
		Anomalies: []costs.AnomalyMark{m1AnomalyMark(newestCol, "Amazon EC2", 500)},
		Requests:  2,
	})
	if got := c.Snapshot().Body.Costs.FooterNote; !strings.Contains(got, "500") {
		t.Fatalf("precondition: first delivery's $500 anomaly did not land in FooterNote, got %q", got)
	}

	c.Handle(messages.CostsLoaded{
		Query:     q,
		Grid:      costs.GridResult{Fetched: false},
		Window:    root.Window,
		Anomalies: []costs.AnomalyMark{},
		Requests:  1,
	})

	if got := c.Snapshot().Body.Costs.FooterNote; strings.Contains(got, "500") {
		t.Errorf("FooterNote %q still shows the stale $500 anomaly after an anomaly-only delivery authoritatively found zero", got)
	}
}

// ===========================================================================
// Cell 8 (unreachable) — grid:skipped x anomalies:skipped. Nothing to
// fetch: ensureCostsShapeFetched never dispatches a task with both halves
// skipped, so this delivery shape never occurs in production. Documented
// rather than silently omitted.
// ===========================================================================

func TestCostsDeliveryMatrix_GridSkipped_AnomaliesSkipped_Unreachable(t *testing.T) {
	t.Skip("unreachable: ensureCostsShapeFetched/screen.FetchPlan never dispatches a KindFetchCosts task with SkipGrid=true AND SkipAnomalies=true — there is nothing left for such a delivery to have fetched, so this (grid, anomalies) combination never arrives on the wire")
}

// ===========================================================================
// Cell 9 — Err (whole-event failure) @ a drilled child frame that matches
// what's awaited. Err short-circuits both halves — Grid/Anomalies carry no
// meaning once Err != nil (ApplyCostsLoaded returns before reading either).
// ===========================================================================

func TestCostsDeliveryMatrix_Err_MatchingChildFrame_SetsErrorMsg(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	newestCol := root.Window[len(root.Window)-1]

	c.Handle(messages.CostsLoaded{
		Query:    costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}},
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newestCol, "Amazon EC2", 500)}},
		Requests: 1,
	})
	if !codexMoveCursorToRow(c, "EC2") {
		t.Fatal("precondition: no SERVICE row for the seeded EC2 record")
	}
	codexMoveCursorToNewestColumn(c)
	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect})
	childPayload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: drilling did not dispatch a fetch task")
	}

	c.Handle(messages.CostsLoaded{Query: childPayload.Query, Err: errWantThrottled, Requests: 1})

	vs := c.Snapshot()
	if vs.Body.Costs.Loading {
		t.Error("Loading still true after a matching failed delivery")
	}
	if vs.Body.Costs.ErrorMsg == "" {
		t.Error("ErrorMsg empty after a matching failed delivery")
	}
}

// ===========================================================================
// Cell 10 — Err @ popped-away (the costs screen was fully left before this
// delivery arrived, cs == nil). ApplyCostsLoaded's own no-cs branch treats
// Err as a pure no-op — no disk write, no panic, nothing left to apply the
// error state to.
// ===========================================================================

func TestCostsDeliveryMatrix_Err_PoppedAway_IsNoOp(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}

	// Pop the costs screen entirely — nothing left with a CostsState
	// (costsStateBeneathOverlay finds no ScreenCosts entry anywhere on the
	// stack), the no-cs branch's own precondition.
	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})

	_, tasks := c.Handle(messages.CostsLoaded{Query: q, Err: errWantThrottled, Requests: 1})
	if len(tasks) != 0 {
		t.Errorf("Err delivery after the costs screen was fully popped returned non-nil tasks: %#v", tasks)
	}
}

// errWantThrottled is a stable sentinel error used by this file's Err cells
// — its exact text is irrelevant, only ErrorMsg's non-emptiness is checked.
var errWantThrottled = costsMatrixThrottledErr{}

type costsMatrixThrottledErr struct{}

func (costsMatrixThrottledErr) Error() string { return "cost explorer: request throttled" }
