// costs_interaction_test.go — Cost Explorer interaction defects (D1-D5)
// found via a live tmux smoke of ./a9s --demo, not covered by
// costs_state_test.go/costs_body_test.go (which assert state fields but
// never fetch dispatch or window anchoring). Contract sources:
// specs/021-cost-explorer/spec.md FR-004/FR-005/FR-010/FR-011/FR-017,
// data-model.md, and the current production code in
// internal/app/costs_state.go + internal/costs/window.go.
//
// New API this file assumes (none of it exists on disk yet — the whole
// file is a compile-red TDD pin until the coder adds it):
//
//   - Controller.SetCostsViewportCols(n int) — the D4/D5 renderer-supplied
//     visible-column-count seam, named and shaped after the existing
//     DetailState.ViewportHeight / Controller.SetDetailViewportHeight
//     precedent (internal/app/screenstate.go, internal/app/detail_state.go).
//     Reconciling CostsBody.ScrollX against CursorCol once ViewportCols is
//     known mirrors detail_cursor.go's reconcileDetailScrollToCursor.
//
// Everything else (D1's fetch-task assertions via Controller.Apply's
// existing []runtime.TaskRequest return, D2/D3's CostsBody/DrillLevel
// field assertions) drives fixes into the EXISTING costs_state.go/
// costs_body.go surface — runtime.KindFetchCosts and
// runtime.FetchCostsPayload already exist (used today only by
// HandleNavigate's initial NavigateTargetCosts dispatch); D1's job is
// wiring the SAME mechanism into handleActionCostPivot/Metric/ZoomIn.
package unit_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// fullMetricRecord builds a realistic multi-metric record for period p:
// invoice plus the three metrics that share invoice's Query shape
// (amortized/net-amortized/blended all read a different metric KEY from
// the SAME fetch — data-model.md's display mapping). unblended is
// deliberately absent: it is fetched under a DISTINCT, tax-excluded
// Query/CacheKey per data-model.md, never mixed into the base fetch.
func fullMetricRecord(p costs.Period, rowKey string, amount float64) costs.Record {
	return costs.Record{
		Period: p,
		Keys:   []string{rowKey},
		Metrics: map[costs.Metric]costs.Amount{
			costs.MetricInvoice:      {Value: amount, Unit: "USD"},
			costs.MetricAmortized:    {Value: amount * 1.02, Unit: "USD"},
			costs.MetricNetAmortized: {Value: amount * 0.98, Unit: "USD"},
			costs.MetricBlended:      {Value: amount * 1.01, Unit: "USD"},
		},
	}
}

// baseServiceQuery is the default root-frame query shape: monthly, grouped
// by SERVICE, invoice mode (no filter).
func baseServiceQuery() costs.Query {
	return costs.Query{
		Granularity: costs.GranularityMonth.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionService},
	}
}

// findFetchCostsTask returns the FetchCostsPayload of the first
// KindFetchCosts TaskRequest in tasks, or ok=false when none is present.
func findFetchCostsTask(tasks []runtime.TaskRequest) (runtime.FetchCostsPayload, bool) {
	for _, tr := range tasks {
		if tr.Key.Kind == runtime.KindFetchCosts {
			if p, ok := tr.Payload.(runtime.FetchCostsPayload); ok {
				return p, true
			}
		}
	}
	return runtime.FetchCostsPayload{}, false
}

// windowContainsPeriod reports whether some column in window fully covers
// target's Start (string comparison is safe: both are "YYYY-MM-DD").
func windowContainsPeriod(window []costs.Period, target costs.Period) bool {
	for _, col := range window {
		if col.Start <= target.Start && target.Start < col.End {
			return true
		}
	}
	return false
}

// ===========================================================================
// D1 — fetch-on-shape-miss
// ===========================================================================

func TestCostsInteraction_D1_MetricSwitchToUnblended_ShapeMiss_SetsLoadingAndEmitsFetchTask(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	// Fresh store: nothing fetched yet, not even the default invoice shape.

	vs, tasks := c.Apply(app.Action{Kind: app.ActionCostMetric}) // invoice -> unblended

	if !vs.Body.Costs.Loading {
		t.Error("switching to unblended (a distinct tax-excluded query shape not yet in the store) must set CostsBody.Loading — never silently render a zero grid (FR-017)")
	}
	if _, found := findFetchCostsTask(tasks); !found {
		t.Fatal("switching metric to unblended (shape miss) did not emit a KindFetchCosts TaskRequest")
	}
}

func TestCostsInteraction_D1_MetricCycleAmongCachedShapes_NoFetchTask(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	window := topDrill(t, c).Window
	// Seed EVERY period in the visible window (closed months and the fresh
	// open one), not just the open column — a single-period seed would let
	// this test pass even if the shape check ignored missing closed periods.
	var recs []costs.Record
	for i, p := range window {
		recs = append(recs, fullMetricRecord(p, "Amazon EC2", 1000.0+float64(i)))
	}
	c.Handle(messages.CostsLoaded{
		Query: baseServiceQuery(),
		Grid:  costs.GridResult{Fetched: true, Records: recs},
		// A non-nil (even empty) Anomalies slice is an authoritative fetch
		// result (costsAnomalyResultFromEvent) — seeded fresh so the
		// unconditional "no fetch task" expectation below holds for the
		// anomaly half too, not just the grid.
		Anomalies: []costs.AnomalyMark{},
		Requests:  1,
	})

	// invoice -> unblended: distinct shape, not cached (covered by the test
	// above) — advance past it without asserting here.
	c.Apply(app.Action{Kind: app.ActionCostMetric})

	// unblended -> amortized -> net-amortized -> blended: every one of these
	// reads a different metric KEY from the SAME already-cached base query
	// shape (same Granularity/GroupBy/Filter) — must render instantly, never
	// re-fetch.
	for _, want := range []costs.Metric{costs.MetricAmortized, costs.MetricNetAmortized, costs.MetricBlended} {
		_, tasks := c.Apply(app.Action{Kind: app.ActionCostMetric})
		if _, found := findFetchCostsTask(tasks); found {
			t.Errorf("switching to metric %q (same cached query shape as invoice) emitted an unnecessary KindFetchCosts task", want)
		}
		if got := c.Snapshot().Body.Costs.Metric; got != string(want) {
			t.Fatalf("metric cycle: got %q want %q", got, want)
		}
	}
}

func TestCostsInteraction_D1_PivotToLinkedAccount_ShapeMiss_EmitsFetchTaskWithNewGroupBy(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	window := topDrill(t, c).Window
	// Seed only the default SERVICE-pivot shape.
	c.Handle(messages.CostsLoaded{
		Query:    baseServiceQuery(),
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(window[len(window)-1], "Amazon EC2", 1200.0)}},
		Requests: 1,
	})

	vs, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 3}) // LINKED_ACCOUNT

	if !vs.Body.Costs.Loading {
		t.Error("pivoting to LINKED_ACCOUNT (a GroupBy shape not in the store) must set CostsBody.Loading — never render an all-zero grid")
	}
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("pivoting to LINKED_ACCOUNT (shape miss) did not emit a KindFetchCosts TaskRequest")
	}
	if len(payload.Query.GroupBy) != 1 || payload.Query.GroupBy[0] != costs.DimensionLinkedAccount {
		t.Errorf("fetch task Query.GroupBy: got %v want [LINKED_ACCOUNT]", payload.Query.GroupBy)
	}
}

func TestCostsInteraction_D1_ZoomInToWeek_ShapeMiss_EmitsFetchTaskWithDailyGranularity(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	window := topDrill(t, c).Window
	c.Handle(messages.CostsLoaded{
		Query:    baseServiceQuery(),
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(window[len(window)-1], "Amazon EC2", 1200.0)}},
		Requests: 1,
	})

	vs, tasks := c.Apply(app.Action{Kind: app.ActionCostZoomIn}) // month -> week (DAILY-backed)

	if !vs.Body.Costs.Loading {
		t.Error("zooming into week (a DAILY-backed shape not in the store) must set CostsBody.Loading — never render an all-zero grid")
	}
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("zooming in to week (shape miss) did not emit a KindFetchCosts TaskRequest")
	}
	if payload.Query.Granularity != "DAILY" {
		t.Errorf("fetch task Query.Granularity: got %q want %q", payload.Query.Granularity, "DAILY")
	}
}

func TestCostsInteraction_D1_ZoomOutBackToCachedMonthShape_NoFetchTask(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	window := topDrill(t, c).Window
	// Seed EVERY period in the visible month window (closed months and the
	// fresh open one), not just the open column — a single-period seed
	// would let this test pass even if the shape check ignored missing
	// closed periods when re-evaluating the month shape on zoom-out.
	var recs []costs.Record
	for i, p := range window {
		recs = append(recs, fullMetricRecord(p, "Amazon EC2", 1000.0+float64(i)))
	}
	c.Handle(messages.CostsLoaded{
		Query: baseServiceQuery(),
		Grid:  costs.GridResult{Fetched: true, Records: recs},
		// A non-nil (even empty) Anomalies slice is an authoritative fetch
		// result — seeded fresh so zooming back out stays a true zero-task
		// cache hit, not just SkipGrid=true.
		Anomalies: []costs.AnomalyMark{},
		Requests:  1,
	})

	c.Apply(app.Action{Kind: app.ActionCostZoomIn})              // month -> week (uncached; not the focus here)
	_, tasks := c.Apply(app.Action{Kind: app.ActionCostZoomOut}) // week -> month (cached)

	if _, found := findFetchCostsTask(tasks); found {
		t.Error("zooming back out to the already-cached month shape re-emitted a KindFetchCosts task")
	}
}

// ===========================================================================
// D2 — zoom anchoring
// ===========================================================================

// TestCostsInteraction_D2_ZoomIn_AnchorsOnCursorPeriod is the one retained
// zoom-IN pin (already correctly implemented by applyCostZoom — this guards
// against a regression, it is not the live defect).
func TestCostsInteraction_D2_ZoomIn_AnchorsOnCursorPeriod(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	if len(root.Window) < 6 {
		t.Fatalf("precondition: root window has %d columns, need enough for a mid-window month", len(root.Window))
	}
	midIdx := len(root.Window) / 2
	// FR-002 "open at today": the cursor starts on the newest (last)
	// column, not column 0 — scroll LEFT from there to reach the
	// mid-window target column.
	stepsLeft := (len(root.Window) - 1) - midIdx
	for range stepsLeft {
		c.Apply(app.Action{Kind: app.ActionScrollLeft})
	}
	if got := topDrill(t, c).Cursor.Col; got != midIdx {
		t.Fatalf("precondition: cursor did not land on the mid-window column: got %d want %d", got, midIdx)
	}
	cursorPeriod := topDrill(t, c).Window[midIdx]

	c.Apply(app.Action{Kind: app.ActionCostZoomIn}) // month -> week

	weekWindow := topDrill(t, c).Window
	if len(weekWindow) == 0 {
		t.Fatal("zoom-in produced an empty window")
	}
	cursorMonth, err := time.Parse("2006-01-02", cursorPeriod.Start)
	if err != nil {
		t.Fatalf("parsing cursor period start %q: %v", cursorPeriod.Start, err)
	}
	firstWeekStart, err := time.Parse("2006-01-02", weekWindow[0].Start)
	if err != nil {
		t.Fatalf("parsing week window start %q: %v", weekWindow[0].Start, err)
	}
	if firstWeekStart.Year() != cursorMonth.Year() || firstWeekStart.Month() != cursorMonth.Month() {
		t.Errorf("zoom-in must anchor on the cursor's period (%s), got week window starting %s — not the same calendar month",
			cursorPeriod.Start, weekWindow[0].Start)
	}
}

// TestCostsInteraction_D2_ZoomOut_WindowContainsCursorPeriod is the live
// defect's first half: the restored coarser window must contain the
// cursor's period, not merely be "some 12-month window".
func TestCostsInteraction_D2_ZoomOut_WindowContainsCursorPeriod(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	midIdx := len(root.Window) / 2
	for range midIdx {
		c.Apply(app.Action{Kind: app.ActionScrollRight})
	}
	cursorMonthPeriod := topDrill(t, c).Window[midIdx]

	c.Apply(app.Action{Kind: app.ActionCostZoomIn})  // month -> week, anchored on cursorMonthPeriod
	c.Apply(app.Action{Kind: app.ActionCostZoomOut}) // week -> month

	restored := topDrill(t, c).Window
	if !windowContainsPeriod(restored, cursorMonthPeriod) {
		t.Errorf("zoom-out must restore a window containing the cursor's original period %+v, got window %+v", cursorMonthPeriod, restored)
	}
}

// TestCostsInteraction_D2_ZoomOut_NeverSlidesBeforeEarliestFetchedPeriod is
// the live defect's core: zooming in on the OLDEST fetched month then back
// out re-anchors on the cursor's (now-clipped) week start, which can walk
// the restored window's start date before any data this session has ever
// fetched.
func TestCostsInteraction_D2_ZoomOut_NeverSlidesBeforeEarliestFetchedPeriod(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	earliestFetched := root.Window[0] // oldest column of the default window

	recs := make([]costs.Record, len(root.Window))
	for i, p := range root.Window {
		recs[i] = fullMetricRecord(p, "Amazon EC2", 1000.0+float64(i))
	}
	c.Handle(messages.CostsLoaded{Query: baseServiceQuery(), Grid: costs.GridResult{Fetched: true, Records: recs}, Requests: 1})

	// Cursor stays at column 0 — the oldest fetched month.
	c.Apply(app.Action{Kind: app.ActionCostZoomIn})  // month -> week, anchored on earliestFetched's month
	c.Apply(app.Action{Kind: app.ActionCostZoomOut}) // week -> month: must not slide before earliestFetched

	restored := topDrill(t, c).Window
	if len(restored) == 0 {
		t.Fatal("zoom-out produced an empty window")
	}
	if restored[0].Start < earliestFetched.Start {
		t.Errorf("zoom-out window slid before the earliest fetched period: got window starting %s, earliest fetched was %s",
			restored[0].Start, earliestFetched.Start)
	}
}

func TestCostsInteraction_D2_ZoomIn_AtDayBoundary_NoOp(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	c.Apply(app.Action{Kind: app.ActionCostZoomIn}) // month -> week
	c.Apply(app.Action{Kind: app.ActionCostZoomIn}) // week -> day
	before := topDrill(t, c).Window

	c.Apply(app.Action{Kind: app.ActionCostZoomIn}) // day -> day (no-op)

	after := topDrill(t, c).Window
	if !reflect.DeepEqual(before, after) {
		t.Errorf("zoom-in at the day boundary must no-op, window changed: before=%+v after=%+v", before, after)
	}
	if g := topDrill(t, c).Granularity; g != costs.GranularityDay {
		t.Errorf("granularity after boundary no-op: got %q want %q", g, costs.GranularityDay)
	}
}

func TestCostsInteraction_D2_ZoomOut_AtYearBoundary_NoOp(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	c.Apply(app.Action{Kind: app.ActionCostZoomOut}) // month -> year
	before := topDrill(t, c).Window

	c.Apply(app.Action{Kind: app.ActionCostZoomOut}) // year -> year (no-op)

	after := topDrill(t, c).Window
	if !reflect.DeepEqual(before, after) {
		t.Errorf("zoom-out at the year boundary must no-op, window changed: before=%+v after=%+v", before, after)
	}
	if g := topDrill(t, c).Granularity; g != costs.GranularityYear {
		t.Errorf("granularity after boundary no-op: got %q want %q", g, costs.GranularityYear)
	}
}

// ===========================================================================
// D3 — data-through
// ===========================================================================

func TestCostsInteraction_D3_DataThrough_IsInclusiveLastDay_NotExclusiveQueryEnd(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	window := topDrill(t, c).Window
	// A CLOSED period (round7, item 3: an OPEN/current period's DataThrough
	// now caps at cs.Now's own date instead of End-1 — window[len-1] is
	// fixedCostsNow's own, still-open month, so this test uses the
	// second-to-last (already-closed) column to keep pinning the
	// exclusive-End-vs-inclusive-day contract this test exists for, without
	// tripping the open-period cap a different test now owns).
	last := window[len(window)-2] // e.g. Start="2026-06-01" End="2026-07-01" (exclusive), closed relative to fixedCostsNow (2026-07-15)

	c.Handle(messages.CostsLoaded{
		Query:    baseServiceQuery(),
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(last, "Amazon EC2", 1200.0)}},
		Requests: 1,
	})

	dataThrough := c.Snapshot().Body.Costs.DataThrough
	if dataThrough == last.End {
		t.Errorf("DataThrough is the query range's EXCLUSIVE End (%q) — must be the inclusive last covered day instead", last.End)
	}
	end, err := time.Parse("2006-01-02", last.End)
	if err != nil {
		t.Fatalf("parsing period end %q: %v", last.End, err)
	}
	wantInclusive := end.AddDate(0, 0, -1).Format("2006-01-02")
	if dataThrough != wantInclusive {
		t.Errorf("DataThrough: got %q want %q (inclusive last day covered by the fetched record)", dataThrough, wantInclusive)
	}
}

func TestCostsInteraction_D3_DataThrough_StaysPinnedToLatestAcrossOutOfOrderFetches(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	window := topDrill(t, c).Window
	// Both CLOSED periods (round7, item 3's open-period cap doesn't apply —
	// see the sibling test above for why window[len-1], fixedCostsNow's own
	// open month, is avoided here).
	older := window[len(window)-4]
	newer := window[len(window)-2]
	q := baseServiceQuery()

	// The newer period lands FIRST, then an older period's fetch result
	// arrives second — DataThrough must not regress backward.
	c.Handle(messages.CostsLoaded{Query: q, Grid: costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newer, "Amazon EC2", 1200.0)}}, Requests: 1})
	c.Handle(messages.CostsLoaded{Query: q, Grid: costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(older, "Amazon EC2", 900.0)}}, Requests: 1})

	end, err := time.Parse("2006-01-02", newer.End)
	if err != nil {
		t.Fatalf("parsing period end %q: %v", newer.End, err)
	}
	want := end.AddDate(0, 0, -1).Format("2006-01-02")
	if got := c.Snapshot().Body.Costs.DataThrough; got != want {
		t.Errorf("DataThrough after an older period's fetch result arrives second: got %q want %q (must stay pinned to the latest period's inclusive last day)", got, want)
	}
}

// ===========================================================================
// D4/D5 — horizontal window scroll
//
// RECONCILED (architecture.md Seam 8, CostsViewModel): the viewport-slice
// mechanism this group exercises (costsVisibleColumnRange's [start,count)
// window, ScrollX reconciliation) is now BuildViewModel's VisibleCols
// output — pinned at the typed seam in costs_screen_test.go
// (TestCostsScreen_BuildViewModel_ViewportSlice_AppliesScrollWindow). This
// group (D4/D5's four tests, including TestRenderCosts_D4D5_
// LabelColumnStaysWhileTimeColumnsShift below) stays as the full-stack
// (controller -> render) acceptance pins.
// ===========================================================================

func TestCostsInteraction_D4_ScrollRight_AdvancesWindowToNewestPeriod_ClampsThere(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	c.SetCostsViewportCols(4)
	window := topDrill(t, c).Window
	if len(window) < 5 {
		t.Fatalf("precondition: need more columns (%d) than the 4-column viewport to exercise scrolling", len(window))
	}

	for range window {
		c.Apply(app.Action{Kind: app.ActionScrollRight})
	}

	body := c.Snapshot().Body.Costs
	wantScrollX := len(window) - 4
	if body.ScrollX != wantScrollX {
		t.Errorf("ScrollX after scrolling to the newest column: got %d want %d (clamped so the newest period stays visible)", body.ScrollX, wantScrollX)
	}
	if len(body.Columns) != 4 {
		t.Fatalf("visible Columns count: got %d want 4 (ViewportCols)", len(body.Columns))
	}
	if lastVisible := body.Columns[len(body.Columns)-1]; !lastVisible.Open {
		t.Errorf("the newest (rightmost) visible column must be the open period, got Open=false for %+v", lastVisible)
	}
}

func TestCostsInteraction_D5_ScrollLeft_RetreatsToOldestFetchedPeriod_ClampsThere(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	c.SetCostsViewportCols(4)
	window := topDrill(t, c).Window

	for range window {
		c.Apply(app.Action{Kind: app.ActionScrollRight})
	}
	for range window {
		c.Apply(app.Action{Kind: app.ActionScrollLeft})
	}

	body := c.Snapshot().Body.Costs
	if body.ScrollX != 0 {
		t.Errorf("ScrollX after scrolling all the way back left: got %d want 0 (clamped at the oldest fetched period)", body.ScrollX)
	}
	if len(body.Columns) != 4 {
		t.Fatalf("visible Columns count: got %d want 4 (ViewportCols)", len(body.Columns))
	}
}

func TestCostsInteraction_D4D5_NoScrollNeeded_WhenColumnsFitViewport(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	window := topDrill(t, c).Window
	c.SetCostsViewportCols(len(window) + 5) // viewport wider than the whole window

	for range window {
		c.Apply(app.Action{Kind: app.ActionScrollRight})
	}

	body := c.Snapshot().Body.Costs
	if body.ScrollX != 0 {
		t.Errorf("ScrollX must stay 0 when every column already fits the viewport, got %d", body.ScrollX)
	}
	if len(body.Columns) != len(window) {
		t.Errorf("visible Columns count: got %d want %d (all columns fit)", len(body.Columns), len(window))
	}
}

// TestRenderCosts_D4D5_LabelColumnStaysWhileTimeColumnsShift is the
// renderer-level half of D4/D5: RenderCosts is a thin, stateless renderer
// (per CostsBody's doc comment — "consumed verbatim, never recomputed"), so
// this asserts the CONTRACT the body-layer scroll fix must uphold, using
// two manually pre-sliced CostsBody literals simulating "before" and
// "after" a scroll — it exercises only already-existing symbols and should
// already be green today.
func TestRenderCosts_D4D5_LabelColumnStaysWhileTimeColumnsShift(t *testing.T) {
	before := app.CostsBody{
		Pivot: "SERVICE", Metric: "invoice", Granularity: "month",
		Columns: []app.CostColumn{{Label: "Mar'26"}, {Label: "Apr'26"}, {Label: "May'26"}, {Label: "Jun'26"}},
		Rows: []app.CostRow{
			{Label: "Amazon EC2", Cells: []app.CostCell{{Amount: "1,100.0"}, {Amount: "1,120.0"}, {Amount: "1,150.0"}, {Amount: "1,180.0"}}},
		},
		Totals:  []app.CostCell{{Amount: "1,100.0"}, {Amount: "1,120.0"}, {Amount: "1,150.0"}, {Amount: "1,180.0"}},
		ScrollX: 0,
	}
	after := app.CostsBody{
		Pivot: "SERVICE", Metric: "invoice", Granularity: "month",
		Columns: []app.CostColumn{{Label: "Apr'26"}, {Label: "May'26"}, {Label: "Jun'26"}, {Label: "Jul'26", Open: true}},
		Rows: []app.CostRow{
			{Label: "Amazon EC2", Cells: []app.CostCell{{Amount: "1,120.0"}, {Amount: "1,150.0"}, {Amount: "1,180.0"}, {Amount: "1,234.5"}}},
		},
		Totals:  []app.CostCell{{Amount: "1,120.0"}, {Amount: "1,150.0"}, {Amount: "1,180.0"}, {Amount: "1,234.5"}},
		ScrollX: 1,
	}

	outBefore := tuitest.StripANSI(views.RenderCosts(before, 100, 24))
	outAfter := tuitest.StripANSI(views.RenderCosts(after, 100, 24))

	if !strings.Contains(outBefore, "Amazon EC2") || !strings.Contains(outAfter, "Amazon EC2") {
		t.Fatal("row label column missing from rendered output")
	}
	if strings.Contains(outBefore, "Mar'26") == strings.Contains(outAfter, "Mar'26") {
		t.Error("the oldest column (Mar'26) should have scrolled out of view after the shift")
	}
	if !strings.Contains(outAfter, "Jul'26*") {
		t.Error("the newest (open) column should have scrolled into view, with its open-period marker")
	}
	if !strings.Contains(outBefore, "1,100.0") || strings.Contains(outAfter, "1,100.0") {
		t.Error("the oldest amount cell should have scrolled out of view after the shift")
	}
}
