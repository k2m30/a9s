// costs_state_test.go — Cost Explorer controller-side drill/pivot/metric/zoom
// state machine (specs/021-cost-explorer/data-model.md §"Controller &
// runtime additions").
//
//   - app.CostsState, app.ActionCostZoomIn/Out, app.ActionCostMetric,
//     app.ActionCostPivot and runtime.ScreenCosts are given verbatim in
//     data-model.md.
//   - app.Controller.EnsureCostsState(now time.Time) follows the Ensure*State
//     family (EnsureDetailState, EnsureTextState, EnsureSelectorState all
//     take the screen's seed data; Costs' only seed data is the injected
//     clock used to compute the default 12-month window, matching this
//     package's "now is injected, never time.Now() internally" convention).
//   - messages.CostsLoaded{Query, Grid, Attrs, Anomalies, Requests, Err} —
//     Grid costs.GridResult{Fetched, Records, Err} is symmetric with
//     costs.AnomalyResult — is routed through app.Controller's real
//     Handle(runtime.Event) dispatch (not a test-only seam) so these tests
//     exercise the same path production code uses.
//   - Grid-cursor movement reuses ActionMoveUp/ActionMoveDown for the row
//     axis (row dimension) and ActionScrollLeft/ActionScrollRight for the
//     column axis (time axis), per data-model.md ("movement reuses
//     ActionMoveUp/Down etc.").
//
// Grid-seeding trick used throughout: every seeded costs.Record uses a
// Period equal to the injected "now"'s own month. Query.CacheKey() only
// derives from Granularity+GroupBy+Filter (Range is explicitly excluded per
// data-model.md), so the seeded Query never needs to reproduce
// EnsureCostsState's exact window math — and using "now"'s own month as the
// record Period guarantees it lands inside ANY reasonable trailing-N-
// months-ending-at-now window, regardless of N.
package unit_test

import (
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// fixedCostsNow is the injected clock for every test in this file — a fixed
// instant (not time.Now()) so window/period math is fully deterministic.
var fixedCostsNow = time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

// newCostsController builds a Controller with ScreenCosts pushed and
// EnsureCostsState seeded, ready to drive costs actions and inspect
// Snapshot().Body.Costs / the controller's CostsState.
func newCostsController(t *testing.T, now time.Time) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{
		runtime.PushScreen{ID: runtime.ScreenCosts},
	})
	c.EnsureCostsState(now)
	return c
}

// topDrill reads the top-of-stack DrillLevel via the exported
// GetCostsDrillStack accessor — mirrors the GetMenu*/GetList* family
// (e.g. GetMenuAvailability, GetListSelectedRow) since Snapshot() only
// exposes the flattened CostsBody, not the raw per-frame drill state
// individual actions in this file need to assert on.
func topDrill(t *testing.T, c *app.Controller) costs.DrillLevel {
	t.Helper()
	vs := c.Snapshot()
	if vs.Body.Kind != app.BodyKindCosts {
		t.Fatalf("expected BodyKindCosts, got %q", vs.Body.Kind)
	}
	stack := c.GetCostsDrillStack()
	if len(stack) == 0 {
		t.Fatal("GetCostsDrillStack returned an empty stack")
	}
	return stack[len(stack)-1]
}

// monthRecord builds one costs.Record for rowKey, priced amount, within
// now's own month — see file header for why this guarantees the record
// lands inside whatever window EnsureCostsState(now) computes.
func monthRecord(now time.Time, rowKey string, amount float64) costs.Record {
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	return costs.Record{
		Period:  costs.Period{Start: start.Format("2006-01-02"), End: end.Format("2006-01-02")},
		Keys:    []string{rowKey},
		Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: amount, Unit: "USD"}},
	}
}

// seedServiceGrid loads a 3-row SERVICE-pivoted grid into the root drill
// frame via the real production event path.
func seedServiceGrid(t *testing.T, c *app.Controller, now time.Time) {
	t.Helper()
	q := costs.Query{
		Granularity: costs.GranularityMonth.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionService},
	}
	c.Handle(messages.CostsLoaded{
		Query: q,
		Grid: costs.GridResult{Fetched: true, Records: []costs.Record{
			monthRecord(now, "Amazon EC2", 1200.0),
			monthRecord(now, "Amazon RDS", 900.0),
			monthRecord(now, "Amazon S3", 150.0),
		}},
		Requests: 1,
	})
}

// ---------------------------------------------------------------------------
// EnsureCostsState seeds the root drill frame
// ---------------------------------------------------------------------------

func TestCostsState_EnsureCostsState_SeedsRootDrillFrame(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)

	stack := c.GetCostsDrillStack()
	if len(stack) != 1 {
		t.Fatalf("DrillStack length after EnsureCostsState: got %d want 1", len(stack))
	}
	root := stack[0]
	if root.RowDim != costs.DimensionService {
		t.Errorf("root RowDim: got %q want %q (SERVICE is the default pivot)", root.RowDim, costs.DimensionService)
	}
	if root.Granularity != costs.GranularityMonth {
		t.Errorf("root Granularity: got %q want %q (default view is monthly per wireframe.md)", root.Granularity, costs.GranularityMonth)
	}
	if len(root.Filter.Equals) != 0 {
		t.Errorf("root Filter.Equals: got %v want empty (no drill has happened yet)", root.Filter.Equals)
	}
	windowLen := len(root.Window)
	if windowLen == 0 {
		t.Fatal("precondition: root drill Window is empty")
	}
	if root.Cursor.Row != 0 || root.Cursor.Col != windowLen-1 {
		t.Errorf("root Cursor: got {%d,%d} want {0,%d} (FR-002 \"open at today\": the cursor starts on the newest/rightmost period, not column 0)", root.Cursor.Row, root.Cursor.Col, windowLen-1)
	}
	if root.ScrollX != 0 || root.ScrollY != 0 {
		t.Errorf("root Scroll: got {%d,%d} want {0,0} (ScrollX reconciliation to show the cursor's column happens once ViewportCols is known, via SetCostsViewportCols)", root.ScrollX, root.ScrollY)
	}

	vs := c.Snapshot()
	if vs.Body.Kind != app.BodyKindCosts {
		t.Fatalf("Snapshot().Body.Kind: got %q want %q", vs.Body.Kind, app.BodyKindCosts)
	}
	if vs.Body.Costs == nil {
		t.Fatal("Snapshot().Body.Costs is nil after EnsureCostsState")
	}
	if vs.Body.Costs.Pivot != string(costs.DimensionService) {
		t.Errorf("CostsBody.Pivot: got %q want %q", vs.Body.Costs.Pivot, costs.DimensionService)
	}
	if vs.Body.Costs.Metric != string(costs.MetricInvoice) {
		t.Errorf("CostsBody.Metric: got %q want %q (invoice is the default display metric)", vs.Body.Costs.Metric, costs.MetricInvoice)
	}
	if vs.Body.Costs.Granularity != string(costs.GranularityMonth) {
		t.Errorf("CostsBody.Granularity: got %q want %q", vs.Body.Costs.Granularity, costs.GranularityMonth)
	}
}

// ---------------------------------------------------------------------------
// A freshly-pushed child drill frame also opens on the newest column
// ---------------------------------------------------------------------------

func TestCostsState_PushDrill_ChildFrame_OpensOnCurrentColumn(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	seedServiceGrid(t, c, fixedCostsNow)

	c.Apply(app.Action{Kind: app.ActionSelect}) // SERVICE -> USAGE_TYPE
	if depth := len(c.GetCostsDrillStack()); depth != 2 {
		t.Fatalf("precondition: ActionSelect must push a child drill frame — DrillStack depth got %d want 2", depth)
	}

	child := topDrill(t, c)
	if child.RowDim != costs.DimensionUsageType {
		t.Fatalf("pushed child frame's RowDim: got %q want %q", child.RowDim, costs.DimensionUsageType)
	}
	if len(child.Window) == 0 {
		t.Fatal("precondition: pushed child frame's Window is empty")
	}

	// FR-002 "open at today": the child opens on the CURRENT column — the
	// newest whose period has already begun at now — NOT the raw last column
	// (week/day child windows tile the whole parent period, so their last
	// column can be an empty future bucket) and NOT column 0 (the oldest
	// often falls outside the RESOURCE_ID 14-day retention window).
	cur := child.Window[child.Cursor.Col]
	curStart, err := time.Parse("2006-01-02", cur.Start)
	if err != nil {
		t.Fatalf("parsing cursor period start %q: %v", cur.Start, err)
	}
	if curStart.After(fixedCostsNow.UTC()) {
		t.Errorf("child opened on a FUTURE column (col %d, %s) — the cursor must land on a period that has already begun by now", child.Cursor.Col, cur.Start)
	}
	if child.Cursor.Col < len(child.Window)-1 {
		next := child.Window[child.Cursor.Col+1]
		if nextStart, nerr := time.Parse("2006-01-02", next.Start); nerr == nil && !nextStart.After(fixedCostsNow.UTC()) {
			t.Errorf("child did not open on the NEWEST non-future column — col %d (%s) is non-future but so is the next col (%s)", child.Cursor.Col, cur.Start, next.Start)
		}
	}
}

// ---------------------------------------------------------------------------
// Zoom walks year -> month -> week -> day with boundary no-ops
// ---------------------------------------------------------------------------

func TestCostsState_Zoom_WalksGranularityChain_WithBoundaryNoops(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)

	granAfter := func(kind app.ActionKind) costs.Granularity {
		c.Apply(app.Action{Kind: kind})
		return topDrill(t, c).Granularity
	}

	// Root starts at month (the default). Zoom out walks toward year;
	// zoom in walks toward day. Boundary no-ops verified in both directions.
	steps := []struct {
		name string
		kind app.ActionKind
		want costs.Granularity
	}{
		{"month->year (zoom out)", app.ActionCostZoomOut, costs.GranularityYear},
		{"year->year (zoom out at top boundary, no-op)", app.ActionCostZoomOut, costs.GranularityYear},
		{"year->month (zoom in)", app.ActionCostZoomIn, costs.GranularityMonth},
		{"month->week (zoom in)", app.ActionCostZoomIn, costs.GranularityWeek},
		{"week->day (zoom in)", app.ActionCostZoomIn, costs.GranularityDay},
		{"day->day (zoom in at bottom boundary, no-op)", app.ActionCostZoomIn, costs.GranularityDay},
		{"day->week (zoom out)", app.ActionCostZoomOut, costs.GranularityWeek},
		{"week->month (zoom out)", app.ActionCostZoomOut, costs.GranularityMonth},
	}
	for _, step := range steps {
		got := granAfter(step.kind)
		if got != step.want {
			t.Errorf("%s: Granularity got %q want %q", step.name, got, step.want)
		}
		if depth := len(c.GetCostsDrillStack()); depth != 1 {
			t.Errorf("%s: zoom must mutate the top frame in place, not push — DrillStack depth got %d want 1", step.name, depth)
		}
	}
}

// ---------------------------------------------------------------------------
// Zoom on a RESOURCE_ID frame must stay within the 14-day CE resource-level
// retention window — the same bound PushDrill enforces via
// costs.ClampResourceDrillWindow (screen.go's Select) the moment a RESOURCE_ID
// frame is first pushed. applyCostZoom has no RESOURCE_ID branch: zooming out
// either re-anchors on the cursor's current period (week/day) or rebuilds the
// ordinary trailing month/year window (costs.BuildWindow(g, cs.Now)) with no
// clamp at all — a follow-up GetCostAndUsageWithResources fetch over that
// window would span far more than 14 days and CE would reject it.
// ---------------------------------------------------------------------------

func TestCostsState_Zoom_ResourceIDFrame_ZoomOut_StaysWithinRetentionWindow(t *testing.T) {
	const demoEC2InstanceID = "i-0a1b2c3d4e5f60001"
	c := round5DrillToResourceRow(t, "Amazon Elastic Compute Cloud - Compute", demoEC2InstanceID)

	if got := topDrill(t, c).RowDim; got != costs.DimensionResourceID {
		t.Fatalf("precondition: expected a RESOURCE_ID top frame, got RowDim=%q", got)
	}
	if got := topDrill(t, c).Granularity; got != costs.GranularityDay {
		t.Fatalf("precondition: expected the RESOURCE_ID frame to start at day granularity, got %q", got)
	}

	_, tasks := c.Apply(app.Action{Kind: app.ActionCostZoomOut})

	// Rendered result: RowDim staying RESOURCE_ID after a zoom-out already
	// proves the frame was mutated in place rather than popped/replaced —
	// pinning len(GetCostsDrillStack()) on top of this would just encode the
	// same fact as an internal-state magic number.
	top := topDrill(t, c)
	if top.RowDim != costs.DimensionResourceID {
		t.Fatalf("zoom must mutate the top frame in place — RowDim changed to %q", top.RowDim)
	}
	if len(top.Window) == 0 {
		t.Fatal("zoom-out on a RESOURCE_ID frame left an empty Window")
	}

	start, err := costs.ParseDate(top.Window[0].Start)
	if err != nil {
		t.Fatalf("Window[0].Start = %q, ParseDate: %v", top.Window[0].Start, err)
	}
	end, err := costs.ParseDate(top.Window[len(top.Window)-1].End)
	if err != nil {
		t.Fatalf("Window[-1].End = %q, ParseDate: %v", top.Window[len(top.Window)-1].End, err)
	}
	if span := end.Sub(start).Hours() / 24; span > costs.ResourceDrillWindowRetentionDays {
		t.Errorf("RESOURCE_ID frame zoom-out produced a %.0f-day window %s..%s — exceeds the %d-day CE resource-level retention bound the push path enforces",
			span, top.Window[0].Start, top.Window[len(top.Window)-1].End, costs.ResourceDrillWindowRetentionDays)
	}

	// Dispatched fetch payload: on a cache miss for this zoom's shape, the
	// CE fetch it triggers must itself carry a window within the same
	// retention bound. Empirically this particular zoom hits cache (no task
	// fires) — "if found" keeps the assertion honest instead of requiring a
	// task the production path doesn't actually dispatch here.
	if payload, found := findFetchCostsTask(tasks); found {
		if len(payload.Window) == 0 {
			t.Fatal("dispatched fetch task carries an empty Window")
		}
		pStart, errS := costs.ParseDate(payload.Window[0].Start)
		pEnd, errE := costs.ParseDate(payload.Window[len(payload.Window)-1].End)
		if errS != nil || errE != nil {
			t.Fatalf("fetch task Window bounds unparseable: start err=%v end err=%v", errS, errE)
		}
		if span := pEnd.Sub(pStart).Hours() / 24; span > costs.ResourceDrillWindowRetentionDays {
			t.Errorf("dispatched fetch task Window spans %.0f days — a GetCostAndUsageWithResources call over %d days is invalid against CE", span, costs.ResourceDrillWindowRetentionDays)
		}
	}
}

// TestCostsState_Zoom_NonResourceFrame_ZoomOut_KeepsTrailingWindowShape pins
// the sibling case a RESOURCE_ID-specific fix must not disturb: a
// SERVICE-pivoted (non-RESOURCE_ID) root frame's zoom-out keeps building the
// ordinary trailing window byte-for-byte — the 14-day resource retention
// clamp applies only to RESOURCE_ID frames.
func TestCostsState_Zoom_NonResourceFrame_ZoomOut_KeepsTrailingWindowShape(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)

	if got := topDrill(t, c).RowDim; got != costs.DimensionService {
		t.Fatalf("precondition: expected the default root frame to be SERVICE-pivoted, got RowDim=%q", got)
	}

	c.Apply(app.Action{Kind: app.ActionCostZoomOut}) // month -> year

	top := topDrill(t, c)
	if top.Granularity != costs.GranularityYear {
		t.Fatalf("expected month->year zoom-out, got Granularity=%q", top.Granularity)
	}
	want := costs.BuildWindow(costs.GranularityYear, fixedCostsNow)
	if len(top.Window) != len(want) {
		t.Fatalf("non-resource frame zoom-out Window length = %d, want %d (costs.BuildWindow(year, now) unchanged)", len(top.Window), len(want))
	}
	for i := range want {
		if top.Window[i] != want[i] {
			t.Errorf("non-resource frame zoom-out Window[%d] = %+v, want %+v", i, top.Window[i], want[i])
		}
	}
}

// ---------------------------------------------------------------------------
// Metric cycles the five display metrics
// ---------------------------------------------------------------------------

func TestCostsState_Metric_CyclesFiveDisplayModes(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)

	// Cycle order per data-model.md's display mapping list: invoice ->
	// unblended -> amortized -> net-amortized -> blended -> back to invoice.
	// net-unblended is stored-only and MUST NOT appear in the cycle.
	wantOrder := []costs.Metric{
		costs.MetricUnblended,
		costs.MetricAmortized,
		costs.MetricNetAmortized,
		costs.MetricBlended,
		costs.MetricInvoice, // full cycle back to the start
	}

	vs := c.Snapshot()
	if vs.Body.Costs.Metric != string(costs.MetricInvoice) {
		t.Fatalf("precondition: default Metric got %q want %q", vs.Body.Costs.Metric, costs.MetricInvoice)
	}

	for i, want := range wantOrder {
		c.Apply(app.Action{Kind: app.ActionCostMetric})
		got := c.Snapshot().Body.Costs.Metric
		if got != string(want) {
			t.Errorf("cycle step %d: Metric got %q want %q", i+1, got, want)
		}
		// net-unblended is stored-only and must never surface in the 'b'
		// cycle — wantOrder above never includes it, so the cycle itself
		// (only ever comparing got against wantOrder's 5 entries) already
		// makes that structurally true; no separate guard needed.
	}
}

// ---------------------------------------------------------------------------
// Pivot presets (digit keys 0-9)
// ---------------------------------------------------------------------------

func TestCostsState_Pivot_DigitPresetsSwitchRowDim(t *testing.T) {
	cases := []struct {
		digit int
		want  costs.Dimension
	}{
		{1, costs.DimensionService},
		{2, costs.DimensionRegion},
		{3, costs.DimensionLinkedAccount},
		{4, costs.DimensionUsageType},
		{5, costs.DimensionPurchaseType},
		{6, costs.DimensionRecordType},
	}
	for _, tc := range cases {
		c := newCostsController(t, fixedCostsNow)
		c.Apply(app.Action{Kind: app.ActionCostPivot, N: tc.digit})
		got := topDrill(t, c).RowDim
		if got != tc.want {
			t.Errorf("digit %d: root RowDim got %q want %q", tc.digit, got, tc.want)
		}
		if pivot := c.Snapshot().Body.Costs.Pivot; pivot != string(tc.want) {
			t.Errorf("digit %d: CostsBody.Pivot got %q want %q", tc.digit, pivot, tc.want)
		}
	}
}

func TestCostsState_Pivot_SevenToNine_AreNoOps(t *testing.T) {
	for _, digit := range []int{7, 8, 9} {
		c := newCostsController(t, fixedCostsNow)
		// Establish a known non-default pivot first so a no-op is observable.
		c.Apply(app.Action{Kind: app.ActionCostPivot, N: 3})
		before := topDrill(t, c).RowDim
		if before != costs.DimensionLinkedAccount {
			t.Fatalf("precondition: RowDim after digit 3 got %q want %q", before, costs.DimensionLinkedAccount)
		}

		c.Apply(app.Action{Kind: app.ActionCostPivot, N: digit})
		after := topDrill(t, c).RowDim
		if after != before {
			t.Errorf("digit %d: reserved preset must no-op — RowDim changed from %q to %q", digit, before, after)
		}
	}
}

func TestCostsState_Pivot_Zero_ResetsToDefaultView(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)

	// Move away from every default: non-default pivot, non-default metric,
	// and drill one level deep — 0 must undo all three at once. Enter is a
	// no-op while its shape is still awaited (drilling into an invisible
	// row is never correct) — a real session always has the CostsLoaded
	// delivery in between, so this test needs one too, matching the
	// EXACT awaited shape (USAGE_TYPE pivot, unblended metric), not
	// seedServiceGrid's unconditional SERVICE-shaped delivery.
	c.Apply(app.Action{Kind: app.ActionCostPivot, N: 4})              // USAGE_TYPE
	_, metricTasks := c.Apply(app.Action{Kind: app.ActionCostMetric}) // -> unblended
	payload, found := findFetchCostsTask(metricTasks)
	if !found {
		t.Fatal("precondition: metric switch to unblended did not emit a fetch task")
	}
	// The active display metric is unblended: the awaited query carries
	// unblended's NotEquals[RECORD_TYPE] filter, and invoiceMetricKey
	// (core/aws/costs.go) stores that shape's UnblendedCost under
	// Metrics[costs.MetricUnblended], not MetricInvoice — monthRecord's
	// invoice-only Metrics map would leave the row valueless for this
	// display metric, so the record is built inline instead.
	rec := monthRecord(fixedCostsNow, "BoxUsage:m5.2xlarge", 1200.0)
	rec.Metrics = map[costs.Metric]costs.Amount{costs.MetricUnblended: {Value: 1200.0, Unit: "USD"}}
	c.Handle(messages.CostsLoaded{
		Query:    payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{rec}},
		Requests: 1,
	})
	c.Apply(app.Action{Kind: app.ActionSelect}) // drill one level

	if depth := len(c.GetCostsDrillStack()); depth < 2 {
		t.Fatalf("precondition: expected a drilled frame before reset, DrillStack depth got %d", depth)
	}

	c.Apply(app.Action{Kind: app.ActionCostPivot, N: 0})

	stack := c.GetCostsDrillStack()
	if len(stack) != 1 {
		t.Errorf("digit 0: DrillStack must pop back to the root frame — depth got %d want 1", len(stack))
	}
	root := stack[0]
	if root.RowDim != costs.DimensionService {
		t.Errorf("digit 0: root RowDim got %q want %q", root.RowDim, costs.DimensionService)
	}
	if len(root.Filter.Equals) != 0 {
		t.Errorf("digit 0: root Filter must be cleared, got %v", root.Filter.Equals)
	}
	vs := c.Snapshot()
	if vs.Body.Costs.Metric != string(costs.MetricInvoice) {
		t.Errorf("digit 0: Metric got %q want %q (invoice)", vs.Body.Costs.Metric, costs.MetricInvoice)
	}
}

// ---------------------------------------------------------------------------
// Movement actions clamp cursor to grid bounds
// ---------------------------------------------------------------------------

func TestCostsState_Movement_ClampsRowToGridBounds(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	seedServiceGrid(t, c, fixedCostsNow) // 3 rows: index 0..2

	// Move down past the last row repeatedly — must clamp at the last index,
	// never run past it or wrap.
	for i := 0; i < 5; i++ {
		c.Apply(app.Action{Kind: app.ActionMoveDown})
	}
	if row := topDrill(t, c).Cursor.Row; row != 2 {
		t.Errorf("after 5x move-down over 3 rows: CursorRow got %d want 2 (clamped to last row)", row)
	}

	// Move up past the first row repeatedly — must clamp at 0.
	for i := 0; i < 5; i++ {
		c.Apply(app.Action{Kind: app.ActionMoveUp})
	}
	if row := topDrill(t, c).Cursor.Row; row != 0 {
		t.Errorf("after 5x move-up from row 2: CursorRow got %d want 0 (clamped to first row)", row)
	}
}

func TestCostsState_Movement_ClampsColumnToGridBounds(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	seedServiceGrid(t, c, fixedCostsNow)

	// FR-002 "open at today": the cursor starts on the newest (last) column,
	// not column 0.
	windowLen := len(topDrill(t, c).Window)
	if windowLen == 0 {
		t.Fatal("precondition: root drill Window is empty, cannot test column clamping")
	}
	before := topDrill(t, c).Cursor.Col
	if before != windowLen-1 {
		t.Fatalf("precondition: CursorCol got %d want %d (opens on the newest/rightmost column)", before, windowLen-1)
	}

	// Scroll right far past the last column — must stay clamped at
	// len(Window)-1, never run off the end.
	for i := 0; i < windowLen+5; i++ {
		c.Apply(app.Action{Kind: app.ActionScrollRight})
	}
	if col := topDrill(t, c).Cursor.Col; col != windowLen-1 {
		t.Errorf("after scrolling right past the last column: CursorCol got %d want %d (clamped to last column)", col, windowLen-1)
	}

	// Scroll left all the way back past column 0 — must clamp at 0, never
	// go negative.
	for i := 0; i < windowLen+5; i++ {
		c.Apply(app.Action{Kind: app.ActionScrollLeft})
	}
	if col := topDrill(t, c).Cursor.Col; col != 0 {
		t.Errorf("after scrolling left past column 0: CursorCol got %d want 0 (clamped)", col)
	}
}

// ---------------------------------------------------------------------------
// ActionBack pops a drill frame, restoring cursor/scroll exactly
// ---------------------------------------------------------------------------

func TestCostsState_ActionBack_PopsDrillFrame_RestoresCursorScrollExactly(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	seedServiceGrid(t, c, fixedCostsNow)

	// Move the root frame's cursor to a known, non-default position before
	// drilling — this is the exact state ActionBack must restore.
	c.Apply(app.Action{Kind: app.ActionMoveDown}) // row 0 -> 1
	c.Apply(app.Action{Kind: app.ActionScrollRight})
	rootBefore := topDrill(t, c)

	c.Apply(app.Action{Kind: app.ActionSelect}) // drill: pushes a new frame
	if depth := len(c.GetCostsDrillStack()); depth != 2 {
		t.Fatalf("precondition: ActionSelect must push a drill frame — DrillStack depth got %d want 2", depth)
	}

	c.Apply(app.Action{Kind: app.ActionBack})

	stack := c.GetCostsDrillStack()
	if len(stack) != 1 {
		t.Fatalf("after ActionBack: DrillStack depth got %d want 1", len(stack))
	}
	rootAfter := stack[0]
	if rootAfter.Cursor != rootBefore.Cursor {
		t.Errorf("after ActionBack: root Cursor got %+v want %+v (exact restore)", rootAfter.Cursor, rootBefore.Cursor)
	}
	if rootAfter.ScrollX != rootBefore.ScrollX || rootAfter.ScrollY != rootBefore.ScrollY {
		t.Errorf("after ActionBack: root Scroll got {%d,%d} want {%d,%d} (exact restore)",
			rootAfter.ScrollX, rootAfter.ScrollY, rootBefore.ScrollX, rootBefore.ScrollY)
	}
	if rootAfter.RowDim != rootBefore.RowDim {
		t.Errorf("after ActionBack: root RowDim got %q want %q (unaffected by the child drill)", rootAfter.RowDim, rootBefore.RowDim)
	}
}
