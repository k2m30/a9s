// costs_round5_test.go — Cost Explorer: external reviewer's third pass (7
// findings, none already covered by rounds 3/4's red tests — fold, the
// zero-value neutral-color pin, and -c startup are all already pinned
// elsewhere and are NOT repeated here).
//
// package unit_test (not unit): every finding here is reachable via the
// headless app.Controller / pure core/costs package — no TUI-level
// helper is needed, so this file reuses costs_state_test.go's
// newCostsController/topDrill/fixedCostsNow/monthRecord and
// costs_interaction_test.go's findFetchCostsTask directly (same package).
//
// *** New/pinned observable surface (finding E only — flagged per the
// *** dispatch): no navigation mechanism exists yet for Enter on a
// *** RESOURCE_ID row (applyCostsSelect is a hard no-op there today —
// *** NextDim returns "" at the bottom of the chain). Rather than invent a
// *** full TaskKindEmitNavigate-shaped round trip (a much larger surface
// *** than this file should pin blind), E asserts against CostsBody's
// *** EXISTING FooterNote seam (the same field DrillRefusedReason already
// *** uses for "Enter did something observable, not a silent no-op") —
// *** loosely: non-empty and resource-ID-bearing for a supported service,
// *** non-empty for an unsupported one. The coder is free to choose the
// *** exact wording and the exact supported-service table; this only pins
// *** that Enter on a RESOURCE_ID row must never again be silent.
package unit_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	a9saws "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// round5FullDailyRecords tiles every day of every period in window with one
// costs.Record each — the shape Store.Lookup's lookupContained requires to
// resolve a week/year column at all (it demands the matched native buckets
// "tile w edge-to-edge with no gap", per store.go's own doc comment): a
// single day inside a week is not enough for that week's column to resolve.
func round5FullDailyRecords(t *testing.T, window []costs.Period, rowKey string, amount float64) []costs.Record {
	t.Helper()
	var recs []costs.Record
	for _, p := range window {
		start, err := time.Parse("2006-01-02", p.Start)
		if err != nil {
			t.Fatalf("parsing period start %q: %v", p.Start, err)
		}
		end, err := time.Parse("2006-01-02", p.End)
		if err != nil {
			t.Fatalf("parsing period end %q: %v", p.End, err)
		}
		for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
			recs = append(recs, costs.Record{
				Period:  costs.Period{Start: d.Format("2006-01-02"), End: d.AddDate(0, 0, 1).Format("2006-01-02")},
				Keys:    []string{rowKey},
				Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: amount, Unit: "USD"}},
			})
		}
	}
	return recs
}

// ===========================================================================
// A (P1) — ApplyCostsLoaded must not stamp week/year DISPLAY buckets as
// covered via MergeCoverage when ev.Records are native DAILY/MONTHLY: this
// is the traced root cause of round 4's "empty week grid" live bug (a
// full CostsLoaded round-trip through the real apply path, not a synthetic
// Store seed — round 4's dead end).
// ===========================================================================

func TestCostsRound5_A_WeekViewCoverageStamp_DoesNotShadowNativeDailyRecords(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)

	targetIdx := len(root.Window) - 3
	if targetIdx < 0 {
		t.Fatalf("precondition: window too short (%d columns)", len(root.Window))
	}
	for range len(root.Window) - 1 - targetIdx {
		c.Apply(app.Action{Kind: app.ActionScrollLeft})
	}

	_, tasks := c.Apply(app.Action{Kind: app.ActionCostZoomIn}) // month -> week
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: zoom-in to week did not emit a KindFetchCosts task")
	}
	weekTop := topDrill(t, c)
	if weekTop.Granularity != costs.GranularityWeek {
		t.Fatalf("precondition: expected Granularity week, got %q", weekTop.Granularity)
	}

	// The REAL apply path: native DAILY records (exactly what CE returns
	// for a week-granularity display request) PLUS the display-level
	// Window (payload.Window == top.Window, the clipped week buckets) —
	// exactly what ensureCostsShapeFetched dispatches and ApplyCostsLoaded
	// receives back via messages.CostsLoaded.
	recs := round5FullDailyRecords(t, weekTop.Window, "Amazon EC2", 10)
	c.Handle(messages.CostsLoaded{
		Query:  payload.Query,
		Grid:   costs.GridResult{Fetched: true, Records: recs},
		Window: payload.Window,
		// A non-nil (even empty) Anomalies slice is an authoritative fetch
		// result — seeded fresh so re-evaluating the same shape below stays
		// a true zero-task cache hit, not just SkipGrid=true.
		Anomalies: []costs.AnomalyMark{},
		Requests:  1,
	})

	vs := c.Snapshot()
	if vs.Body.Costs.Loading {
		t.Error("Loading still true after a matching CostsLoaded delivery")
	}
	if len(vs.Body.Costs.Rows) == 0 {
		t.Fatal("week-granularity body has ZERO rows after a full CostsLoaded round-trip with native daily records and the display Window — MergeCoverage's empty display-bucket stamp is shadowing the real native daily data (round 4's live \"empty week grid\" bug)")
	}
	nonZero := false
	for _, row := range vs.Body.Costs.Rows {
		for _, cell := range row.Cells {
			if cell.Amount != "" && cell.Amount != "0.0" {
				nonZero = true
			}
		}
	}
	if !nonZero {
		t.Errorf("week-granularity body's rows are all zero after a full CostsLoaded round-trip with native daily records (rows: %+v)", vs.Body.Costs.Rows)
	}

	// Second half: re-evaluating the SAME base query shape (invoice ->
	// unblended -> amortized cycles back to invoice's own CacheKey, per
	// costsQueryForFrame's doc) must NOT re-emit a fetch — Lookup must
	// resolve the native daily records already merged, not treat the shape
	// as still missing because of an empty display-bucket coverage stamp
	// shadowing them.
	c.Apply(app.Action{Kind: app.ActionCostMetric})              // invoice -> unblended (distinct shape, ignore)
	_, tasks2 := c.Apply(app.Action{Kind: app.ActionCostMetric}) // unblended -> amortized (back to invoice's base shape)
	if _, found := findFetchCostsTask(tasks2); found {
		t.Error("re-evaluating the same base query shape re-emitted a KindFetchCosts task — a subsequent Lookup must not prefer an empty display-bucket coverage entry over the real merged native records")
	}
}

// ===========================================================================
// B (P1) — a CostsLoaded arriving while Help (or Identity) is stacked
// ABOVE the costs screen must still merge into the costs screen beneath.
// ===========================================================================

func TestCostsRound5_B_CostsLoadedWhileOverlayStacked_StillMergesIntoScreenBeneath(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)

	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE, already current — dispatch against the empty store
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: SERVICE pivot did not emit a KindFetchCosts task")
	}
	if !c.Snapshot().Body.Costs.Loading {
		t.Fatal("precondition: fetch dispatch did not set Loading")
	}

	// Push an overlay (Help) ON TOP of the costs screen — mirrors a user
	// pressing '?' while a fetch is outstanding.
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenHelp}})

	// Deliver the fetch result WHILE Help is stacked above.
	c.Handle(messages.CostsLoaded{
		Query:    payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{monthRecord(fixedCostsNow, "Amazon EC2", 1200.0)}},
		Requests: 1,
	})

	// Pop the overlay back to the costs screen.
	c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})

	vs := c.Snapshot()
	if vs.Body.Kind != app.BodyKindCosts {
		t.Fatalf("expected to be back on the costs screen after popping Help, got Body.Kind=%q", vs.Body.Kind)
	}
	if vs.Body.Costs.Loading {
		t.Error("costs screen still shows Loading after popping Help — the CostsLoaded delivered while Help was stacked above must have merged into the costs screen beneath, not been silently dropped")
	}
	if len(vs.Body.Costs.Rows) == 0 {
		t.Error("costs screen has zero rows after popping Help — the fetch result delivered while Help was on top was never merged")
	}
}

// ===========================================================================
// C (P1) — in-flight matching must include the requested RANGE, not just
// CacheKey (extends round 2's shape-only R4 pin to same-shape different-
// range deliveries): fetch May (week-granularity), zoom to June (same
// CacheKey, different Range) before May's result lands, deliver May's
// result -> June must stay Loading; deliver June's -> renders.
// ===========================================================================

func TestCostsRound5_C_InFlightMatch_IncludesRange_NotJustCacheKey(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)

	mayIdx, juneIdx := -1, -1
	for i, p := range root.Window {
		if strings.HasPrefix(p.Start, "2026-05") {
			mayIdx = i
		}
		if strings.HasPrefix(p.Start, "2026-06") {
			juneIdx = i
		}
	}
	if mayIdx == -1 || juneIdx == -1 {
		t.Fatalf("precondition: default window does not include both May and June 2026, window=%+v", root.Window)
	}

	for range len(root.Window) - 1 - mayIdx {
		c.Apply(app.Action{Kind: app.ActionScrollLeft})
	}
	_, mayTasks := c.Apply(app.Action{Kind: app.ActionCostZoomIn}) // month -> week, anchored on May
	mayPayload, found := findFetchCostsTask(mayTasks)
	if !found {
		t.Fatal("precondition: May zoom-in did not emit a fetch task")
	}
	if !c.Snapshot().Body.Costs.Loading {
		t.Fatal("precondition: May zoom-in did not set Loading")
	}

	// Zoom back out (trailing default, lands at the newest column per
	// FR-002) WITHOUT ever delivering May's result, then move to June and
	// zoom in again — same base query shape, different Range.
	c.Apply(app.Action{Kind: app.ActionCostZoomOut})
	monthTop := topDrill(t, c)
	if monthTop.Granularity != costs.GranularityMonth {
		t.Fatalf("precondition: expected Granularity month after zoom-out, got %q", monthTop.Granularity)
	}
	for range len(monthTop.Window) - 1 - juneIdx {
		c.Apply(app.Action{Kind: app.ActionScrollLeft})
	}
	_, juneTasks := c.Apply(app.Action{Kind: app.ActionCostZoomIn}) // month -> week, anchored on June
	junePayload, found := findFetchCostsTask(juneTasks)
	if !found {
		t.Fatal("precondition: June zoom-in did not emit a fetch task")
	}
	if !c.Snapshot().Body.Costs.Loading {
		t.Fatal("precondition: June zoom-in did not set Loading")
	}
	juneWeekTop := topDrill(t, c)

	if mayPayload.Query.CacheKey() != junePayload.Query.CacheKey() {
		t.Fatalf("precondition broken: May/June queries have different CacheKeys (%q vs %q) — this test needs the SAME shape, different Range", mayPayload.Query.CacheKey(), junePayload.Query.CacheKey())
	}
	if mayPayload.Query.Range == junePayload.Query.Range {
		t.Fatalf("precondition broken: May/June queries have the SAME Range %+v — test needs them to differ", mayPayload.Query.Range)
	}

	// Deliver MAY's result while June is the awaited shape+range — must
	// NOT clear Loading.
	c.Handle(messages.CostsLoaded{
		Query:    mayPayload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{{Period: costs.Period{Start: "2026-05-04", End: "2026-05-05"}, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 5, Unit: "USD"}}}}},
		Requests: 1,
	})
	if !c.Snapshot().Body.Costs.Loading {
		t.Error("delivering May's result while June (same CacheKey, different Range) is awaited cleared Loading — in-flight matching must include the requested RANGE, not just CacheKey")
	}

	// Deliver JUNE's own (matching) result, fully tiling June's week window
	// (Store.Lookup's lookupContained requires edge-to-edge daily coverage
	// to resolve a week column at all) — Loading clears, grid renders.
	c.Handle(messages.CostsLoaded{
		Query:    junePayload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: round5FullDailyRecords(t, juneWeekTop.Window, "Amazon EC2", 7)},
		Requests: 1,
	})
	vs := c.Snapshot()
	if vs.Body.Costs.Loading {
		t.Error("delivering June's own matching result did not clear Loading")
	}
	if len(vs.Body.Costs.Rows) == 0 {
		t.Error("after June's result landed, the grid has zero rows")
	}
}

// ===========================================================================
// D (P2) — the demo transport must serve GetCostAndUsageWithResources:
// resource-level drill in --demo returns synthetic per-resource costs
// whose resource IDs reference existing demo fixture resources (e.g. the
// demo EC2 instance IDs), so a future resource-detail jump lands on real
// demo entities.
// ===========================================================================

func TestCostsRound5_D_DemoTransport_ServesGetCostAndUsageWithResources_RealFixtureIDs(t *testing.T) {
	client := newDemoCostsClient()

	result, err := a9saws.FetchCostAndUsageWithResources(context.Background(), client, costs.Query{
		Granularity: costs.GranularityDay.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionResourceID},
		Filter:      costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionService: {"Amazon Elastic Compute Cloud - Compute"}}},
		Range:       costs.Period{Start: "2026-06-01", End: "2026-06-08"},
	})
	if err != nil {
		t.Fatalf("FetchCostAndUsageWithResources against the demo transport: %v — the demo transport must register a ce:GetCostAndUsageWithResources handler", err)
	}
	if len(result.Records) == 0 {
		t.Fatal("FetchCostAndUsageWithResources returned zero records against the demo transport")
	}

	// core/demo/fixtures/ec2.go's "web-prod-01" instance — a real demo
	// EC2 entity, not a synthetic never-referenced ID.
	const demoEC2InstanceID = "i-0a1b2c3d4e5f60001"
	found := false
	for _, rec := range result.Records {
		for _, key := range rec.Keys {
			if key == demoEC2InstanceID {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("no record's Keys reference the real demo EC2 instance ID %q — resource-level drill must return IDs that match existing demo fixture resources so a future resource-detail jump lands on real entities", demoEC2InstanceID)
	}
}

// ===========================================================================
// E (P2) — Enter on a RESOURCE_ID row must not be a dead end. See the file
// header comment for the exact (deliberately minimal, flagged) observable
// this pins: CostsBody.FooterNote, mirroring DrillRefusedReason's existing
// "Enter did something observable" seam.
// ===========================================================================

// round5DrillToResourceRow drills SERVICE -> USAGE_TYPE -> RESOURCE_ID
// (pinning serviceName at the root) and seeds exactly one RESOURCE_ID row
// (resourceKey) at the bottom frame, cursor positioned on it.
func round5DrillToResourceRow(t *testing.T, serviceName, resourceKey string) *app.Controller {
	t.Helper()
	c := newCostsController(t, fixedCostsNow)

	c.Handle(messages.CostsLoaded{
		Query:    costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}},
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{monthRecord(fixedCostsNow, serviceName, 1200.0)}},
		Requests: 1,
	})
	_, drill1Tasks := c.Apply(app.Action{Kind: app.ActionSelect}) // pins SERVICE=serviceName, -> USAGE_TYPE frame

	if got := topDrill(t, c).RowDim; got != costs.DimensionUsageType {
		t.Fatalf("precondition: expected USAGE_TYPE frame after first drill, got RowDim=%q", got)
	}

	// Loading gates Select unconditionally now (screen.Select's WaitForRows) —
	// the USAGE_TYPE frame's own fetch must land before the next Enter, or
	// it strictly no-ops instead of advancing.
	drill1Payload, found := findFetchCostsTask(drill1Tasks)
	if !found {
		t.Fatal("precondition: SERVICE -> USAGE_TYPE drill did not emit a fetch task")
	}
	usageTypeTop := topDrill(t, c)
	c.Handle(messages.CostsLoaded{
		Query:    drill1Payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(usageTypeTop.Window[0], "USE1-BoxUsage:m5.large", 1200.0)}},
		Window:   usageTypeTop.Window,
		Requests: 1,
	})

	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect}) // pins USAGE_TYPE=the delivered row's key, -> RESOURCE_ID frame
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: RESOURCE_ID drill did not emit a fetch task")
	}
	resourceTop := topDrill(t, c)
	if resourceTop.RowDim != costs.DimensionResourceID {
		t.Fatalf("precondition: expected RESOURCE_ID frame after second drill, got RowDim=%q", resourceTop.RowDim)
	}

	c.Handle(messages.CostsLoaded{
		Query:    payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{{Period: resourceTop.Window[0], Keys: []string{resourceKey}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 3, Unit: "USD"}}}}},
		Requests: 1,
	})
	return c
}

// TestCostsRound5_E_ResourceRowEnter_SupportedService_NotSilentNoOp is
// reconciled to round8's finding 3 (superseding this test's original
// FooterNote-only contract): Enter on a mapped (EC2) resource row must
// now navigate — emit a KindFetchByIDDetail task — rather than merely set
// an observable footer note. See TestCostsRound8_Item3_
// ResourceRowEnter_SupportedService_EmitsFetchByIDDetailTask in
// costs_round8_test.go for the full traced rationale; this test keeps its
// original name (still exercises the same drill-to-RESOURCE_ID scenario)
// but now asserts the navigation task instead.
func TestCostsRound5_E_ResourceRowEnter_SupportedService_NotSilentNoOp(t *testing.T) {
	const demoEC2InstanceID = "i-0a1b2c3d4e5f60001"
	c := round5DrillToResourceRow(t, "Amazon Elastic Compute Cloud - Compute", demoEC2InstanceID)

	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect}) // Enter on the resource row

	if len(c.GetCostsDrillStack()) != 3 {
		t.Fatalf("expected the drill stack to stay at depth 3 (RESOURCE_ID is the bottom of the chain), got %d", len(c.GetCostsDrillStack()))
	}
	var navTask *runtime.TaskRequest
	for i := range tasks {
		if tasks[i].Key.Kind == runtime.KindFetchByIDDetail {
			navTask = &tasks[i]
		}
	}
	if navTask == nil {
		t.Fatal("Enter on a RESOURCE_ID row (EC2, a supported a9s type) emitted no KindFetchByIDDetail navigation task — must not be a silent dead end")
	}
	if navTask.Key.Scope != "ec2" {
		t.Errorf("KindFetchByIDDetail Key.Scope = %q, want %q", navTask.Key.Scope, "ec2")
	}
	payload, ok := navTask.Payload.(runtime.FetchByIDDetailPayload)
	if !ok {
		t.Fatalf("KindFetchByIDDetail Payload type = %T, want runtime.FetchByIDDetailPayload", navTask.Payload)
	}
	if payload.ID != demoEC2InstanceID {
		t.Errorf("Payload.ID = %q, want %q", payload.ID, demoEC2InstanceID)
	}
}

// TestCostsRound5_E_ResourceRowEnter_UnsupportedService_DefinedBehavior_NotSilentNoOp
// pins round7's NEW contract (superseding this test's original scenario):
// CE's GetCostAndUsageWithResources only supports EC2
// ("Amazon Elastic Compute Cloud - Compute" — round7, finding 2), so a
// non-EC2 single-service drill is now refused at the USAGE_TYPE ->
// RESOURCE_ID transition itself — the RESOURCE_ID frame is never reached,
// so the original "seed a RESOURCE_ID row, Enter on it" scenario is
// unreachable by design. round5DrillToResourceRow (still used, unchanged,
// by the EC2 sibling test above) is deliberately NOT reused here, since
// its own precondition (successfully reaching RESOURCE_ID) is exactly what
// a non-EC2 service must now fail.
func TestCostsRound5_E_ResourceRowEnter_UnsupportedService_DefinedBehavior_NotSilentNoOp(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)

	c.Handle(messages.CostsLoaded{
		Query:    costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}},
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{monthRecord(fixedCostsNow, "AWS Support (Business)", 1200.0)}},
		Requests: 1,
	})
	_, drill1Tasks := c.Apply(app.Action{Kind: app.ActionSelect}) // pins SERVICE="AWS Support (Business)", -> USAGE_TYPE frame

	if got := topDrill(t, c).RowDim; got != costs.DimensionUsageType {
		t.Fatalf("precondition: expected USAGE_TYPE frame after the first drill, got RowDim=%q", got)
	}
	if depth := len(c.GetCostsDrillStack()); depth != 2 {
		t.Fatalf("precondition: expected drill stack depth 2 after the first drill, got %d", depth)
	}

	// Loading gates Select unconditionally now (screen.Select's
	// WaitForRows) — the USAGE_TYPE frame's own fetch must land before the
	// next Enter, or it strictly no-ops instead of even reaching the
	// RESOURCE_ID refusal this test targets.
	drill1Payload, found := findFetchCostsTask(drill1Tasks)
	if !found {
		t.Fatal("precondition: SERVICE -> USAGE_TYPE drill did not emit a fetch task")
	}
	usageTypeTop := topDrill(t, c)
	c.Handle(messages.CostsLoaded{
		Query:    drill1Payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(usageTypeTop.Window[0], "Support-Hours", 1200.0)}},
		Window:   usageTypeTop.Window,
		Requests: 1,
	})

	// Enter on the USAGE_TYPE row with a non-EC2 SERVICE pinned: the drill
	// to RESOURCE_ID must be refused honestly (FR-007), not attempted
	// against CE (which would reject it) and not a silent no-op.
	vs, _ := c.Apply(app.Action{Kind: app.ActionSelect})

	if depth := len(c.GetCostsDrillStack()); depth != 2 {
		t.Errorf("drill stack depth after the refused RESOURCE_ID drill: got %d, want 2 (no new frame pushed)", depth)
	}
	if vs.Body.Costs.FooterNote == "" {
		t.Error("Enter on a USAGE_TYPE row with a non-EC2 SERVICE pinned produced no observable FooterNote — the refusal must surface an honest reason, not a silent dead end")
	}
	if !strings.Contains(vs.Body.Costs.FooterNote, "EC2") && !strings.Contains(vs.Body.Costs.FooterNote, "Elastic Compute Cloud") {
		t.Errorf("FooterNote %q does not name the EC2-only constraint that caused the refusal", vs.Body.Costs.FooterNote)
	}
}

// ===========================================================================
// F (P2) — anomaly cell-matching by Period.Start containment: a month-long
// anomaly must mark the week/day column containing its start when drilled
// in, not vanish because the anomaly's whole Period no longer fits inside
// any single finer-granularity column.
// ===========================================================================

func TestCostsRound5_F_AnomalyMatchesByPeriodStartContainment_NotFullContainment(t *testing.T) {
	weekColumns := []costs.Period{
		{Start: "2026-01-01", End: "2026-01-05"},
		{Start: "2026-01-05", End: "2026-01-12"},
		{Start: "2026-01-12", End: "2026-01-19"},
		{Start: "2026-01-19", End: "2026-01-26"},
		{Start: "2026-01-26", End: "2026-02-01"},
	}
	g := costs.Grid{
		RowDim: costs.DimensionService,
		Rows: []costs.GridRow{
			{Key: "EC2 - Other", Cells: make([]costs.CellValue, len(weekColumns))},
		},
		Columns: weekColumns,
	}
	mark := costs.AnomalyMark{
		ID:        "anomaly-month-long",
		Period:    costs.Period{Start: "2026-01-01", End: "2026-02-01"}, // month-long, whole
		Dimension: map[costs.Dimension]string{costs.DimensionService: "EC2 - Other"},
		RootCause: "EC2 - Other spend spiked",
	}

	got := costs.ApplyAnomalies(g, []costs.AnomalyMark{mark})

	if got.Rows[0].Cells[0].Anomaly == nil {
		t.Error("month-long anomaly starting 2026-01-01 did not mark the week column containing its start (2026-01-01 - 2026-01-05) — anomaly matching must key off Period.Start containment, not full-Period containment")
	}
	for i := 1; i < len(got.Rows[0].Cells); i++ {
		if got.Rows[0].Cells[i].Anomaly != nil {
			t.Errorf("anomaly incorrectly marked week column %d (%+v) — only the column containing the anomaly's Start should be marked", i, weekColumns[i])
		}
	}
}

// ===========================================================================
// G (P2) — delta semantics for negative rows must reflect the EFFECT ON
// SPEND, not a naive signed percentage: -100 -> -50 (a credit/refund
// shrinking) is spend GROWTH, -50 -> -100 (a credit growing) is a drop;
// zero-crossing must render sanely too.
// ===========================================================================

func TestCostsRound5_G_DeltaSemantics_NegativeRows_ReflectSpendEffect(t *testing.T) {
	window := []costs.Period{
		{Start: "2026-01-01", End: "2026-02-01"},
		{Start: "2026-02-01", End: "2026-03-01"},
	}
	tests := []struct {
		name       string
		prev, cur  float64
		wantGrowth bool
	}{
		{"credit shrinking (-100 -> -50) is spend GROWTH", -100, -50, true},
		{"credit growing (-50 -> -100) is a DROP", -50, -100, false},
		{"credit fully offset to zero (-50 -> 0) is spend GROWTH", -50, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recs := []costs.Record{
				{Period: window[0], Keys: []string{"Refunds"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: tt.prev, Unit: "USD"}}},
				{Period: window[1], Keys: []string{"Refunds"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: tt.cur, Unit: "USD"}}},
			}
			grid := costs.BuildGrid(recs, costs.MetricInvoice, window)
			if len(grid.Rows) != 1 {
				t.Fatalf("precondition: expected 1 row, got %d", len(grid.Rows))
			}
			delta := grid.Rows[0].Cells[1].Delta
			if tt.wantGrowth && !(delta > 0) {
				t.Errorf("%s: Delta = %v, want positive (spend growth)", tt.name, delta)
			}
			if !tt.wantGrowth && !(delta < 0) {
				t.Errorf("%s: Delta = %v, want negative (spend drop)", tt.name, delta)
			}
		})
	}
}
