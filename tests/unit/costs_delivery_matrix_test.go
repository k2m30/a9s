// Grid and Anomalies are independent axes at the executor (SkipGrid and
// SkipAnomalies are separate booleans on FetchCostsPayload), so the
// {grid}x{anomalies} cross product is real. Failure is not a per-half state:
// messages.CostsLoaded carries one Err for the whole delivery, and
// ApplyCostsLoaded returns before reading Grid or Anomalies when Err is set,
// so Err is its own axis. Both halves skipped is unreachable:
// ensureCostsShapeFetched never dispatches a fetch with nothing to fetch.
//
// Each reachable grid x anomaly combination is paired with the one frame
// state that makes its distinguishing behaviour observable.
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

// Anomalies authoritatively found none, so FooterNote must not mention an
// anomaly.

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

// A same-shape refresh that skips the anomaly slot leaves any existing
// anomaly FooterNote untouched.

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

	// Anomalies nil: the anomaly slot is skipped while its TTL is fresh.
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

// A grid re-confirmed zero at an eligible child frame fires the coarser
// fallback even when anomalies landed marks in the same delivery; grid
// authority decides it.

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

// A grid re-confirmed zero for an already-warm bucket still stamps coverage
// (an authoritative zero is authoritative), the row clears, and the untouched
// anomaly slot leaves any existing note alone.

func TestCostsDeliveryMatrix_GridEmpty_AnomaliesSkipped_WarmBucket_StampsCoverage(t *testing.T) {
	// The seed is fetched strictly earlier than fixedCostsNow, still within the
	// open-period TTL: MergeCoverage's `!existing.FetchedAt.Equal(now)` guard
	// needs two different "now" stamps to tell an earlier warm fetch from this
	// same round, and one fixed clock for both deliveries would mask the
	// zero-clear.
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

// An anomaly-only refresh must not touch MergeCoverage: warm records survive
// and the new marks land.

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

// An anomaly-only delivery that authoritatively found zero anomalies clears a
// stale FooterNote: PutAnomalies runs unconditionally on success.

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

func TestCostsDeliveryMatrix_GridSkipped_AnomaliesSkipped_Unreachable(t *testing.T) {
	t.Skip("unreachable: ensureCostsShapeFetched/screen.FetchPlan never dispatches a KindFetchCosts task with SkipGrid=true AND SkipAnomalies=true — there is nothing left for such a delivery to have fetched, so this (grid, anomalies) combination never arrives on the wire")
}

// Err short-circuits both halves: ApplyCostsLoaded returns before reading
// Grid or Anomalies.

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

// With the costs screen fully left before the delivery arrives (cs == nil),
// ApplyCostsLoaded treats Err as a no-op: no disk write, no panic.

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
