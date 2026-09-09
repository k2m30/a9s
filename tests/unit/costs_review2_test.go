// costs_review2_test.go — Cost Explorer external review round 2: findings
// R2-R9 (R1, the visible-window-missing-period gap, was independently
// verified fixed on HEAD by the F4-tightening commit — see
// TestCostsReview_F4_VisibleWindow_MissingClosedPeriod_TriggersRefetch_NeverRenderedAsSilentZero
// in costs_review_findings_test.go, which now passes; no test for it here).
//
// package unit (not unit_test): R7 needs the full TUI Model
// (newRootSizedModel/rootApplyMsg/ctrlR/drainCmds, package-unit-only)
// alongside the seven headless-Controller/Store/aws-layer findings, and Go
// permits only one package per file. Every review*-prefixed helper reused
// below (reviewNow, reviewCostsController, reviewCostsControllerNoIsolation,
// reviewTopDrill, reviewBaseServiceQuery, reviewFullMetricRecord,
// reviewFindFetchCostsTask) already lives in this package, defined in
// costs_review_findings_test.go.
//
// *** New API surface this file pins (does NOT exist on disk yet — these
// *** two tests are compile-red until the coder adds the symbols below).
// *** Every other test in this file compiles clean today and is a genuine
// *** runtime red/green against EXISTING production surface only.
//
//  1. core/costs/store.go: Store.MergeCoverage(q Query, covered []Period,
//     now time.Time) — R2. ADDITIVE (existing 3-arg Merge is untouched, so
//     every current Merge call site keeps compiling unchanged): stamps every
//     period in `covered` as fetched-at-now, including periods with zero
//     matching records — Merge alone can only learn a period exists from its
//     own records' Period field, so a CE result with zero groups for a
//     period (a young account, or spend fully filtered out) is invisible to
//     it today and Lookup reports that period missing forever. Follows
//     Merge's own closed-immutable/open-always-refreshes rule.
//  2. core/costs/grid.go: ApplyRowAttrs(g Grid, attrs map[string]string)
//     Grid — R8. A NEW package-level function operating on an
//     already-built Grid (relabels Rows[i].Label to "name (id)" when attrs
//     has an entry for Rows[i].Key, leaving unmatched keys as the raw ID) —
//     deliberately NOT a new BuildGrid parameter, since BuildGrid has many
//     existing call sites (production liveCostGrid plus several tests) that
//     a required-signature-widening would break; a separate post-processing
//     step is additive and keeps that blast radius at zero.
//
// Contract: specs/021-cost-explorer/data-model.md, spec.md FR-007/FR-012/
// FR-014/FR-017, and the current production code in
// core/app/costs_state.go, core/app/costs_body.go,
// core/runtime/handlers_navigate.go, core/app/navigate.go,
// core/runtime/executor.go, internal/tui/runtime_adapter_navigate.go,
// core/costs/{store,grid,drill}.go — each finding below was
// independently re-verified against current code before being scoped here.
package unit

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/costexplorer"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// review2NewDemoCostsClient mirrors costs_demo_test.go's newDemoCostsClient
// (package unit_test, not importable from here) — a *costexplorer.Client
// routed through the demo transport.
func review2NewDemoCostsClient() *costexplorer.Client {
	return costexplorer.NewFromConfig(demo.NewDemoAWSConfig())
}

// review2WindowContainsPeriod mirrors costs_interaction_test.go's
// windowContainsPeriod (package unit_test, not importable from here).
func review2WindowContainsPeriod(window []costs.Period, target costs.Period) bool {
	for _, p := range window {
		if p == target {
			return true
		}
	}
	return false
}

// ===========================================================================
// R2 (P2) — a successfully fetched period with ZERO records (young account,
// or spend fully filtered out) must be remembered as covered.
// ===========================================================================

func TestCostsReview2_R2_ZeroRecordPeriod_MergeCoverage_NotReportedMissing(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	now := reviewNow
	window := costs.BuildWindow(costs.GranularityMonth, now)
	if len(window) < 3 {
		t.Fatalf("precondition: default window has %d columns, need at least 3", len(window))
	}
	zeroRecordIdx := 2 // a closed month CE genuinely returned zero groups for — not a bug, a real empty result

	q := reviewBaseServiceQuery()
	store := costs.LoadStore("test-profile")

	var recs []costs.Record
	for i, p := range window {
		if i == zeroRecordIdx {
			continue
		}
		recs = append(recs, reviewFullMetricRecord(p, "Amazon EC2", 1000.0+float64(i)))
	}
	store.Merge(q, recs, now)

	if _, missing := store.Lookup(q, window, now); !review2WindowContainsPeriod(missing, window[zeroRecordIdx]) {
		t.Fatalf("precondition broken: Merge alone already treats the zero-record period %+v as covered — MergeCoverage's fix would be untestable", window[zeroRecordIdx])
	}

	// The fetch executor always knows the FULL requested window regardless
	// of how many records CE actually returned for it — MergeCoverage
	// stamps that coverage explicitly, independent of Merge's per-record
	// period derivation.
	store.MergeCoverage(q, window, now)

	_, missing := store.Lookup(q, window, now)
	if review2WindowContainsPeriod(missing, window[zeroRecordIdx]) {
		t.Errorf("Store.Lookup still reports the zero-record period %+v as missing after MergeCoverage stamped the full window as covered — this loops the fetch forever for a young account / fully-filtered-out period", window[zeroRecordIdx])
	}
	if len(missing) != 0 {
		t.Errorf("Store.Lookup reports %d missing periods after MergeCoverage covered the full window: %+v", len(missing), missing)
	}
}

// ===========================================================================
// R3 (P1) — opening the costs screen with a warm cache must perform ZERO CE
// fetches: HandleNavigate must not emit an unconditional KindFetchCosts —
// ensureCostsShapeFetched decides (SC-002).
// ===========================================================================

func TestCostsReview2_R3_MainMenuNavigateToCosts_WarmCache_ZeroFetches(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	// NavigateKindPushCosts (core/app/navigate.go) seeds CostsState via
	// c.ensureCostsState(time.Now()) — a hard-coded wall clock, not
	// injectable — so the seed window below must be built from the same
	// real clock the navigation path will use.
	now := time.Now().UTC()

	window := costs.BuildWindow(costs.GranularityMonth, now)
	seed := costs.LoadStore("test-profile")
	var recs []costs.Record
	for i, p := range window {
		recs = append(recs, reviewFullMetricRecord(p, "Amazon EC2", 1000.0+float64(i)))
	}
	seed.Merge(reviewBaseServiceQuery(), recs, now) // every closed + the fresh open period covered
	// Also seed a fresh anomaly slot (screen.PlanFetch's Grid/Anomalies
	// freshness derive independently now) so this test's "zero fetches"
	// expectation still holds unconditionally, not just for the grid half.
	// covered is the window under test — the exact range this scenario's
	// anomaly snapshot is standing in for.
	seed.PutAnomalies(nil, now, costs.Period{Start: window[0].Start, End: window[len(window)-1].End})
	if err := seed.Save(); err != nil {
		t.Fatalf("seeding on-disk cost cache: %v", err)
	}

	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)

	// Navigate to Costs via the real headless menu-select path (the same
	// code HandleNavigate/applyNavResult chain a web/headless client uses —
	// SC-002), not the ApplyIntents/EnsureCostsState test-seam shortcut
	// reviewCostsControllerNoIsolation uses (that seam bypasses HandleNavigate
	// entirely, so it cannot exercise this bug).
	c.Apply(app.Action{Kind: app.ActionSetFilter, Arg: "costs"})
	c.Apply(app.Action{Kind: app.ActionMoveTop})
	vs, tasks := c.Apply(app.Action{Kind: app.ActionSelect})

	if vs.Body.Kind != app.BodyKindCosts {
		t.Fatalf("Select on the synthetic costs entry did not open the costs screen, got Body.Kind=%q", vs.Body.Kind)
	}
	if vs.Body.Costs.Loading {
		t.Error("opening the costs screen with a fully warm cache (every closed + fresh open period already cached) must not show Loading=true")
	}
	if _, found := reviewFindFetchCostsTask(tasks); found {
		t.Error("opening the costs screen with a fully warm cache emitted a KindFetchCosts task — HandleNavigate must not fetch unconditionally; ensureCostsShapeFetched must be the sole decider")
	}
}

// ===========================================================================
// R4 (P1) — a CostsLoaded whose Query does not match the awaited top-frame
// query must not clear Loading or install ErrorMsg for the active shape
// (its records may still merge); only the matching result clears Loading.
// ===========================================================================

func TestCostsReview2_R4_ApplyCostsLoaded_MismatchedQuery_DoesNotClearLoadingOrError_ButStillMerges(t *testing.T) {
	c := newCostsScreenController(t, reviewNow)
	window := reviewTopDrill(t, c).Window

	// Shape A: the default root frame (SERVICE pivot) against an empty
	// store — re-evaluating it dispatches (and awaits) its fetch.
	_, tasksA := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE, already the current pivot
	if _, found := reviewFindFetchCostsTask(tasksA); !found {
		t.Fatal("precondition: shape A (SERVICE) did not emit a KindFetchCosts task against the empty store")
	}
	if !c.Snapshot().Body.Costs.Loading {
		t.Fatal("precondition: shape A's fetch dispatch did not set Loading")
	}
	shapeAQuery := reviewBaseServiceQuery()

	// Switch pivot to shape B (REGION) BEFORE shape A's result arrives —
	// still Loading, now awaiting a DIFFERENT query shape.
	_, tasksB := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 2}) // REGION
	if _, found := reviewFindFetchCostsTask(tasksB); !found {
		t.Fatal("precondition: shape B (REGION) did not emit a KindFetchCosts task")
	}
	if !c.Snapshot().Body.Costs.Loading {
		t.Fatal("precondition: shape B's fetch dispatch did not set Loading")
	}
	shapeBQuery := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionRegion}}

	// Deliver shape A's now-stale (from the controller's current
	// perspective) result. Its records may still merge, but since the top
	// frame has since moved on to shape B, this delivery must NOT clear
	// Loading or set an error for the still-awaited shape B.
	var recsA []costs.Record
	for i, p := range window {
		recsA = append(recsA, reviewFullMetricRecord(p, "Amazon EC2", 1000.0+float64(i)))
	}
	c.Handle(messages.CostsLoaded{Query: shapeAQuery, Grid: costs.GridResult{Fetched: true, Records: recsA}, Requests: 1})

	vs := c.Snapshot()
	if !vs.Body.Costs.Loading {
		t.Error("delivering shape A's result while shape B is still awaited cleared Loading — ApplyCostsLoaded must only resolve Loading/ErrorMsg for the query shape it actually awaits (matched via CacheKey), not unconditionally on every arrival")
	}
	if vs.Body.Costs.ErrorMsg != "" {
		t.Errorf("delivering a successful (non-error) mismatched result unexpectedly set ErrorMsg: %q", vs.Body.Costs.ErrorMsg)
	}

	// Confirm shape A's records genuinely merged into the Store (not
	// silently dropped) by pivoting back to shape A: with every window
	// period now cached, this must render instantly, no new fetch.
	_, tasksBackToA := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1})
	if payload, found := reviewFindFetchCostsTask(tasksBackToA); found && !payload.SkipGrid {
		t.Error("pivoting back to shape A after its CostsLoaded delivery re-emitted a GRID fetch (SkipGrid=false) — shape A's records were not actually merged into the Store during the mismatched-query delivery (an anomalies-only SkipGrid=true task is fine: the anomaly slot may still be absent/stale)")
	}
	if c.Snapshot().Body.Costs.Loading {
		t.Error("pivoting back to the now-fully-cached shape A still shows Loading=true")
	}

	// Finally, re-select (still-uncached) shape B and deliver its own
	// matching result — Loading must now clear and the grid render it.
	_, tasksBackToB := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 2})
	if _, found := reviewFindFetchCostsTask(tasksBackToB); !found {
		t.Fatal("precondition: re-selecting shape B (still uncached) did not re-emit a KindFetchCosts task")
	}
	var recsB []costs.Record
	for i, p := range window {
		recsB = append(recsB, reviewFullMetricRecord(p, "us-east-1", 2000.0+float64(i)))
	}
	c.Handle(messages.CostsLoaded{Query: shapeBQuery, Grid: costs.GridResult{Fetched: true, Records: recsB}, Requests: 1})

	vs = c.Snapshot()
	if vs.Body.Costs.Loading {
		t.Error("delivering shape B's own (matching) result did not clear Loading")
	}
	if len(vs.Body.Costs.Rows) == 0 {
		t.Error("after shape B's matching result landed, the grid has zero rows — shape B's data did not render")
	}
}

// ===========================================================================
// R5 (P2) — after a drill, the body's Pivot/row-label header must reflect
// the TOP DrillLevel's actual RowDim, not the stale digit-key pivot from
// before the drill; popping restores the outer label.
// ===========================================================================

func TestCostsReview2_R5_Drill_PivotHeaderReflectsTopFrameRowDim_NotStaleDigitPivot(t *testing.T) {
	c := newCostsScreenController(t, reviewNow)

	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 2}) // REGION at the root
	vs := c.Snapshot()
	if vs.Body.Costs.Pivot != string(costs.DimensionRegion) {
		t.Fatalf("precondition: root pivot header got %q want %q", vs.Body.Costs.Pivot, costs.DimensionRegion)
	}

	// Enter is a no-op while its shape is still awaited (drilling into an
	// invisible row is never correct) — a real session always has the
	// CostsLoaded delivery in between.
	payload, found := reviewFindFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: REGION pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query:    payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{round8MonthRecord(reviewNow, "us-east-1", 1200.0)}},
		Requests: 1,
	})

	// Drill (Enter) into the cursor's cell — FR-006 pivots the CHILD
	// frame's RowDim to the next dimension in the chain, without touching
	// cs.Pivot (only the digit-key handler writes that field).
	c.Apply(app.Action{Kind: app.ActionSelect})

	top := reviewTopDrill(t, c)
	vs = c.Snapshot()
	if vs.Body.Costs.Pivot != string(top.RowDim) {
		t.Errorf("after drilling in, header Pivot got %q but the TOP drill frame's actual RowDim is %q — the header must reflect what the grid is actually grouped by, not the stale digit-key pivot from before the drill", vs.Body.Costs.Pivot, top.RowDim)
	}

	// Pop back out — the outer (root) frame's RowDim (REGION) must be
	// reflected in the header again.
	c.Apply(app.Action{Kind: app.ActionBack})
	top = reviewTopDrill(t, c)
	vs = c.Snapshot()
	if vs.Body.Costs.Pivot != string(top.RowDim) {
		t.Errorf("after popping the drill, header Pivot got %q but the restored top frame's RowDim is %q", vs.Body.Costs.Pivot, top.RowDim)
	}
}

// ===========================================================================
// R6 (P2) — anomaly marks must flow end to end through the demo transport:
// fetch -> CostsLoaded.Anomalies -> Store -> grid CELL mark -> footer root
// cause when the cursor is on the flagged cell.
// ===========================================================================

func TestCostsReview2_R6_DemoTransport_AnomalyFlowsToCellMarkAndFooter(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	s.Clients = &awsclient.ServiceClients{CostExplorer: review2NewDemoCostsClient()}
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	// The controller's `now` is taken from the demo fixture's OWN anchor
	// month, never a date literal: the trailing month window built from it
	// then ends at the anchor and is therefore guaranteed to contain the
	// fixture's planted anomaly month (core/demo/fixtures/costs.go's
	// CostsGrowthMonth, a fixed offset inside that same window).
	c.EnsureCostsState(costsFixtureAnchorNow(t))

	// Force the default root frame's shape fetch and execute it for real
	// against the demo transport — the exact code path FR-014's anomaly
	// overlay must flow through end to end.
	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE, already current
	payload, found := reviewFindFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: default root frame did not emit a KindFetchCosts task against the empty store")
	}
	req := runtime.TaskRequest{
		Key:     runtime.TaskKey{Kind: runtime.KindFetchCosts, Scope: "costs"},
		Payload: payload,
	}
	event, err := core.ExecuteTask(context.Background(), req)
	if err != nil {
		t.Fatalf("ExecuteTask(KindFetchCosts) against the demo transport: %v", err)
	}
	loaded, ok := event.(messages.CostsLoaded)
	if !ok {
		t.Fatalf("ExecuteTask(KindFetchCosts) returned %T, want messages.CostsLoaded", event)
	}
	if loaded.Err != nil {
		t.Fatalf("CostsLoaded.Err: %v", loaded.Err)
	}
	if len(loaded.Anomalies) == 0 {
		t.Fatalf("the demo-transport fetch for the default SERVICE-grouped shape returned zero Anomalies — FR-014's planted anomaly (%s / %s) never left the fetch path (executor's KindFetchCosts case must call FetchCostAnomalies alongside the grid query)", fixtures.CostsGrowthService, fixtures.CostsGrowthUsageType)
	}

	c.Handle(loaded)

	// The story's own label after stripCostsServiceVendorPrefix strips any
	// "Amazon "/"AWS " vendor prefix off fixtures.CostsGrowthService — the
	// grid row label the story actually renders under.
	growthRowLabel := strings.TrimPrefix(strings.TrimPrefix(fixtures.CostsGrowthService, "Amazon "), "AWS ")

	vs := c.Snapshot()
	// The cursor's column is preserved across a pivot now (it no longer
	// resets to 0) — the root frame opened at the newest column (FR-002),
	// so that's where we start from; the scroll arithmetic below must be a
	// DELTA from here, not an absolute count of right-scrolls from 0.
	startCol := vs.Body.Costs.CursorCol
	rowIdx, colIdx := -1, -1
	for ri, row := range vs.Body.Costs.Rows {
		if row.Label == growthRowLabel {
			rowIdx = ri
			break
		}
	}
	if rowIdx == -1 {
		t.Fatalf("no grid row labeled %q found after the demo fetch landed", growthRowLabel)
	}
	// The flagged column is named by deriving the label from the fixture's
	// own planted-anomaly month, so this pin follows the dataset's anchor
	// instead of asserting whichever month the literal was written in.
	growthColLabel := costsFixtureMonthLabel(t, fixtures.CostsGrowthMonth)
	for ci, col := range vs.Body.Costs.Columns {
		if col.Label == growthColLabel {
			colIdx = ci
			break
		}
	}
	if colIdx == -1 {
		t.Fatalf("no visible column labeled %q (the planted anomaly month) found", growthColLabel)
	}

	if !vs.Body.Costs.Rows[rowIdx].Cells[colIdx].Anomaly {
		t.Errorf("CostCell.Anomaly is false at the planted anomaly's own (row=%q, col=%q) cell — the fetched Anomalies never reached the rendered grid cell (buildCostsBody/costs.BuildGrid must set it on matching cells)", growthRowLabel, growthColLabel)
	}

	// Move the cursor onto that exact cell and confirm the footer surfaces
	// the anomaly's root cause, not just a period-over-period delta.
	for range rowIdx {
		c.Apply(app.Action{Kind: app.ActionMoveDown})
	}
	switch delta := colIdx - startCol; {
	case delta > 0:
		for range delta {
			c.Apply(app.Action{Kind: app.ActionScrollRight})
		}
	case delta < 0:
		for range -delta {
			c.Apply(app.Action{Kind: app.ActionScrollLeft})
		}
	}
	vs = c.Snapshot()
	footer := vs.Body.Costs.FooterNote
	if !review2ContainsAll(footer, growthRowLabel, fixtures.CostsGrowthUsageType) {
		t.Errorf("footer with cursor on the flagged cell: got %q, want it to name the anomaly root cause (service + usage type)", footer)
	}
}

// review2ContainsAll reports whether s contains every one of subs.
func review2ContainsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}

// ===========================================================================
// R7 (P2) — Ctrl+R on the costs screen must force-refresh: the open period
// is re-fetched even though the cache is fresh (FR-012).
// ===========================================================================

func TestCostsReview2_R7_CtrlR_OnCostsScreen_ForcesOpenPeriodRefetch(t *testing.T) {
	m := newRootSizedModel()

	now := time.Now().UTC()
	window := costs.BuildWindow(costs.GranularityMonth, now)
	seed := costs.LoadStore("testprofile") // matches newRootSizedModel's tuitest.Sized profile
	var recs []costs.Record
	for i, p := range window {
		recs = append(recs, reviewFullMetricRecord(p, "Amazon EC2", 1000.0+float64(i)))
	}
	seed.Merge(reviewBaseServiceQuery(), recs, now) // fully warm — every closed + fresh open period
	if err := seed.Save(); err != nil {
		t.Fatalf("seeding on-disk cost cache: %v", err)
	}

	m, navCmd := rootApplyMsg(m, messages.Navigate{Target: messages.TargetCosts})
	// Drain the navigation's own cmd chain first, so whatever it does or
	// does not fetch (a separate concern from R7) is resolved before
	// isolating Ctrl+R's own effect.
	m, _ = drainCmds(t, m, navCmd, 10)

	m, refreshCmd := rootApplyMsg(m, ctrlR())
	_, chainMsgs := drainCmds(t, m, refreshCmd, 10)

	found := false
	for _, msg := range chainMsgs {
		if _, ok := msg.(messages.CostsLoaded); ok {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Ctrl+R on the costs screen with a fully warm cache produced no CostsLoaded-triggering fetch — force-refresh must re-fetch the open period regardless of cache freshness (FR-012); handleRefresh currently no-ops for every rendererState kind except menu/detail/list")
	}
}

// ===========================================================================
// R8 (P2) — LINKED_ACCOUNT pivot rows must label as "name (id)" using the
// store's attrs; raw ID only when no attr exists for that key.
// ===========================================================================

func TestCostsReview2_R8_LinkedAccountPivot_RowLabel_NameParensID(t *testing.T) {
	g := costs.Grid{
		RowDim: costs.DimensionLinkedAccount,
		Rows: []costs.GridRow{
			{Key: "123456789012", Label: "123456789012"},
			{Key: "999999999999", Label: "999999999999"}, // no attrs entry for this account
		},
	}
	attrs := map[string]string{"123456789012": "prod-account"}

	got := costs.ApplyRowAttrs(g, attrs)

	if len(got.Rows) != 2 {
		t.Fatalf("ApplyRowAttrs changed the row count: got %d want 2", len(got.Rows))
	}
	if got.Rows[0].Label != "prod-account (123456789012)" {
		t.Errorf("row with a known account-name attr: Label got %q, want %q", got.Rows[0].Label, "prod-account (123456789012)")
	}
	if got.Rows[1].Label != "999999999999" {
		t.Errorf("row with NO attrs entry: Label got %q, want the raw ID unchanged %q", got.Rows[1].Label, "999999999999")
	}
}

// ===========================================================================
// R9 (P1) — the resource-drill 14-day gate must validate the WINDOW the
// drill will actually query (BuildWindow's finer-granularity output, which
// can start earlier than the clipped selected period), not merely the
// selected cell's own period. Contract (explicitly pinned): CLAMP the
// expanded window to now-14d and allow the drill; refuse only when nothing
// remains after clamping.
// ===========================================================================

func TestCostsReview2_R9_ResourceDrillGate_NearBoundary_ClampsExpandedWindow_NotRefuse(t *testing.T) {
	// Midnight UTC (not reviewNow's noon) so the 14-day cutoff falls on a
	// clean date boundary that lines up exactly with the clipped week
	// period's Start below — isolates the window-expansion bug from any
	// time-of-day rounding.
	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)
	cutoff := now.AddDate(0, 0, -14) // 2026-07-01, 00:00 UTC

	c := newCostsScreenController(t, now)

	// Move the cursor to the last (open) column — July 2026 — of the
	// default 12-month root window, then drill in twice: SERVICE ->
	// USAGE_TYPE (week granularity) -> RESOURCE_ID (day granularity, the
	// gated drill).
	rootWindow := reviewTopDrill(t, c).Window

	// Seed the root SERVICE-pivoted grid so the first Enter pins a REAL
	// service, not "" — round7's ResourceDrillAllowed now requires the
	// pinned SERVICE to be exactly EC2 (CE's GetCostAndUsageWithResources
	// hard constraint), and an empty string is no longer vacuously
	// accepted by a count-only check. Cursor starts at (row 0, col
	// len-1==newest), and this is the ONLY seeded row, so row 0 resolves
	// to it regardless of which column the cursor is later scrolled to.
	c.Handle(messages.CostsLoaded{
		Query:    reviewBaseServiceQuery(),
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{reviewFullMetricRecord(rootWindow[len(rootWindow)-1], "Amazon Elastic Compute Cloud - Compute", 1200.0)}},
		Requests: 1,
	})

	for range len(rootWindow) - 1 {
		c.Apply(app.Action{Kind: app.ActionScrollRight})
	}
	_, drill1Tasks := c.Apply(app.Action{Kind: app.ActionSelect}) // -> USAGE_TYPE, week granularity, anchored on Jul 2026, SERVICE pinned to EC2

	weekTop := reviewTopDrill(t, c)
	// weekTop.Window[0] is CLIPPED to the month boundary ("2026-07-01"),
	// which is exactly AT the 14-day cutoff — the coarse check the current
	// gate performs passes. The finer (day) window BuildWindow builds for
	// the next drill is anchored on this SAME clipped Start, but expands to
	// the full Mon-Sun ISO week containing it, which starts on 2026-06-29 —
	// two days BEFORE the retention cutoff.
	if weekTop.Window[0].Start != "2026-07-01" {
		t.Fatalf("precondition: week[0].Start got %q want %q — the boundary scenario depends on this exact clip", weekTop.Window[0].Start, "2026-07-01")
	}

	// The pushed child frame now opens with the cursor on its own newest
	// column (FR-002, applyCostsSelect's PushDrill case) — scroll it back
	// to col 0 so the record delivered below, and the next Select, target
	// week[0], the exact boundary-adjacent cell this test is about.
	// ActionScrollLeft clamps at col 0 (WEEK granularity never triggers the
	// month-only scroll-to-load extension), so over-scrolling is safe.
	for range weekTop.Window {
		c.Apply(app.Action{Kind: app.ActionScrollLeft})
	}

	// Loading gates Select unconditionally now (screen.Select's
	// WaitForRows) — the USAGE_TYPE frame's own fetch must land before the
	// next Enter. Seeded at week[0] (the cursor's cell after scrolling back
	// to col 0) so the grid still has a real row there after landing.
	drill1Payload, found := reviewFindFetchCostsTask(drill1Tasks)
	if !found {
		t.Fatal("precondition: SERVICE -> USAGE_TYPE drill did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query:    drill1Payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{reviewFullMetricRecord(weekTop.Window[0], "USE1-BoxUsage:m5.large", 100.0)}},
		Window:   weekTop.Window,
		Requests: 1,
	})

	// Drill again: USAGE_TYPE -> RESOURCE_ID. Cursor is scrolled back to
	// col 0, pointing at week[0].
	c.Apply(app.Action{Kind: app.ActionSelect})

	stack := c.GetCostsDrillStack()
	if len(stack) < 3 {
		t.Fatalf("resource drill was refused (drill stack depth %d, want 3) — the contract is CLAMP the expanded window and allow, not refuse, when only the coarse selected period (not the expanded finer window) is within the 14-day retention limit", len(stack))
	}
	top := stack[len(stack)-1]
	if top.RowDim != costs.DimensionResourceID {
		t.Fatalf("expected the pushed child frame's RowDim to be RESOURCE_ID, got %v", top.RowDim)
	}
	if len(top.Window) == 0 {
		t.Fatal("resource drill's child frame has an empty Window after clamping — CLAMP must leave at least the in-retention portion, not drop everything")
	}
	for _, p := range top.Window {
		start, err := time.Parse("2006-01-02", p.Start)
		if err != nil {
			t.Fatalf("parsing window period start %q: %v", p.Start, err)
		}
		if start.Before(cutoff) {
			t.Errorf("resource-drill child frame's Window contains period %+v starting before the 14-day retention cutoff (%s) — the gate validated only the coarse selected period, not the expanded (finer-granularity) window BuildWindow actually produces", p, cutoff.Format("2006-01-02"))
		}
	}
	if got := top.Window[len(top.Window)-1].End; got != "2026-07-06" {
		t.Errorf("resource-drill child frame's last window period End got %q want %q — only the too-early START should be clamped, the End must stay exactly what the unclamped day-window would have produced", got, "2026-07-06")
	}
}
