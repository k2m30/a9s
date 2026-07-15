// costs_screen_test.go — contract tests for the Cost Explorer domain
// refactor (specs/021-cost-explorer/architecture.md). TDD compile-red: the
// new package core/costs/screen does not exist yet, and core/costs
// (existing) does not yet carry AnomalyResult/FetchResult/
// Store.ApplyFetchResult/Money/TotalOutcome/SumCells/TrailingWindow/
// WindowWithin. This whole test binary is expected to fail to compile until
// the coder lands both. Per the dispatch: do NOT run `make test` against
// this state.
//
// Package attribution (deduced from the doc's own type signatures, not
// merely its package-layout intro, to avoid an import cycle):
//   - core/costs (EXISTING, extended): AnomalyResult, FetchResult,
//     (*Store).ApplyFetchResult, Money, TotalOutcome, SumCells,
//     TrailingWindow, WindowWithin — BuildGrid (existing, core/costs)
//     must call SumCells directly, and (*Store) methods must reference
//     FetchResult/AnomalyResult directly; core/costs cannot import
//     core/costs/screen (screen imports costs), so these types cannot
//     live in screen without a cycle.
//   - core/costs/screen (NEW): CoverageView, FetchPlan, PlanFetch,
//     DrillPath, SelectOutcome (+ NoSelection/PushDrill/OpenResource/
//     Refuse — NoSelection replaces the former WaitForRows/NoSelectableRow
//     pair: the sole consumer (applyCostsSelect) always treated them
//     identically, so the ponytail-review cut merged them into one outcome),
//     ScreenState, GridRowRef, PeriodRef, Select, ResourceLocator,
//     ViewModel, CursorPos, Viewport, BuildViewModel — the screen-level
//     "what should happen"/"what should render" decisions the doc's intro
//     describes moving out of core/app. InitOutcome (one-field bool
//     ceremony wrapping Store.Recovered()) was cut by the same review —
//     callers read Store.Recovered() directly now.
//
// Speculative construction (the doc gives full field-level types for these
// but no constructor/helper signatures — ScreenState/GridRowRef/PeriodRef
// in particular are never typed in the doc beyond their use in Select's own
// signature). Chosen minimally, directly from the doc's prose:
//
//	type ScreenState struct {
//	    RowDim          costs.Dimension   // current frame's pivot/grouping dim
//	    Path            screen.DrillPath  // ancestor dims already pinned (order only)
//	    Filter          costs.Filter      // ancestor dims' actual pinned VALUES
//	    Granularity     costs.Granularity // current frame granularity
//	    Loading         bool              // shape in flight (blocks Select unconditionally —
//	                                       // this is X7's fix: the old "blind chain" exclusion
//	                                       // is gone, WaitForRows fires even for a just-pushed frame)
//	    Now             time.Time
//	    ResourceTypeFor func(service string) (shortName string, ok bool) // catalog lookup, injected for purity
//	}
//	type GridRowRef struct { Present bool; Value string }
//	type PeriodRef struct { Period costs.Period }
//
// PushDrill.Granularity (added this round) — closes the "two homes"
// duplication where core/app kept its own copy of finerGranularity to
// set the pushed frame's Granularity, alongside Select's OWN internal
// finerGranularity call that already built Window at that same step. Field
// added directly to the existing PushDrill struct:
//
//	type PushDrill struct { Dim costs.Dimension; Value string; Window []costs.Period; Granularity costs.Granularity }
//
// Seam 8 (CostsViewModel, added this round) speculative construction —
// CursorPos/Viewport are named in the doc's BuildViewModel signature but
// never typed beyond that. Chosen to carry exactly what the CURRENT adapter
// mechanics being extracted need (core/app/costs_state.go's
// liveCostGrid/costsVisibleColumnRange/costsGridCacheKey,
// core/app/costs_body.go's buildCostsBody cursor-clamp block — read
// directly to ground this hypothesis, not guessed from the doc's prose
// alone):
//
//	type CursorPos struct { Row, Col int } // Col indexes VisibleCols directly
//	                                        // post-clamp (unifies the old adapter's
//	                                        // separate absolute cursorCol + relative
//	                                        // relCursorCol into one valid index)
//	type Viewport struct { Cols int; ScrollX int } // Cols<=0 means "unsliced, show everything"
//	                                                 // (ViewportCols<=0 fallback, costsVisibleColumnRange)
//
// If the coder's real shape differs, these tests are the negotiable part —
// the outcome assertions (which fields, which values) are the contract; the
// input-construction shape is this file's working hypothesis.
package unit_test

import (
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/costs/screen"
)

// ===========================================================================
// Seam 1 — FetchPlan / PlanFetch (core/costs/screen)
//
// "Freshness inputs are computed independently — cost coverage and anomaly
// freshness have different lifecycles and never gate each other." Table
// covers all four quadrants the doc's own bug classes name: grid-missing x
// anomaly-fresh (the C4a/X2 collision's OPPOSITE quadrant — grid needs a
// fetch but anomalies must NOT be re-requested), grid-warm x anomaly-stale
// (X3's exact bug — warm rows must not suppress a still-needed anomaly
// refresh), both fresh (X3's "no task at all" case), both stale (root
// cold-open case).
// ===========================================================================

func TestCostsScreen_PlanFetch_GridAndAnomalyFreshnessDeriveIndependently(t *testing.T) {
	const profile = "screen-planfetch"
	now := fixedCostsNow
	q := baseServiceQuery()
	window := costs.BuildWindow(costs.GranularityMonth, now)

	tests := []struct {
		name          string
		seedGrid      bool // fully cover window, fresh open period
		seedAnomalies bool // Put marks at `now` (fresh, within 24h TTL)
		wantGrid      bool
		wantAnomalies bool
	}{
		{"grid missing, anomalies fresh", false, true, true, false},
		{"grid warm, anomalies stale", true, false, false, true},
		{"both fresh", true, true, false, false},
		{"both stale", false, false, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := costs.LoadStore(profile + "-" + tt.name)
			if tt.seedGrid {
				var recs []costs.Record
				for _, p := range window {
					recs = append(recs, costs.Record{Period: p, Keys: []string{"Amazon EC2"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 100, Unit: "USD"}}})
				}
				store.Merge(q, recs, now)
				store.MergeCoverage(q, window, now)
			}
			if tt.seedAnomalies {
				store.PutAnomalies([]costs.AnomalyMark{{Period: window[len(window)-1], Dimension: map[costs.Dimension]string{costs.DimensionService: "Amazon EC2"}}}, now, costs.Period{Start: window[0].Start, End: window[len(window)-1].End})
			}

			plan := screen.PlanFetch(store, q, window, now)
			if plan.Grid != tt.wantGrid {
				t.Errorf("FetchPlan.Grid = %v, want %v", plan.Grid, tt.wantGrid)
			}
			if plan.Anomalies != tt.wantAnomalies {
				t.Errorf("FetchPlan.Anomalies = %v, want %v", plan.Anomalies, tt.wantAnomalies)
			}
			if plan.Grid && plan.Query.CacheKey() != q.CacheKey() {
				t.Errorf("FetchPlan.Query shape = %+v, want a Query matching %+v's CacheKey when Grid is true", plan.Query, q)
			}
		})
	}
}

// ===========================================================================
// Seam 2 + Seam 6 — AnomalyResult through (*costs.Store).ApplyFetchResult
// (core/costs, extended)
//
// Reconciles the X2-vs-C4a collision: TestCostsSelfReview_C4a's intent
// (authoritative-empty clears) is case "requested, empty" below; the X2
// codex pin's intent (skip preserves) is case "not requested" below. Both
// now express as distinct, non-overlapping AnomalyResult shapes instead of
// a single ambiguous nil slice.
// ===========================================================================

func TestCostsScreen_ApplyFetchResult_AnomalyResult_WriteSemantics(t *testing.T) {
	const profile = "screen-applyfetchresult"
	t0 := fixedCostsNow
	q := baseServiceQuery()
	window := costs.BuildWindow(costs.GranularityMonth, t0)
	seedMark := costs.AnomalyMark{Period: window[len(window)-1], Dimension: map[costs.Dimension]string{costs.DimensionService: "Amazon EC2"}}
	newMark := costs.AnomalyMark{Period: window[len(window)-1], Dimension: map[costs.Dimension]string{costs.DimensionService: "Amazon RDS"}}

	tests := []struct {
		name          string
		anomalies     costs.AnomalyResult
		wantMarksLen  int
		wantTTLRenews bool // whether FetchedAt should now read as t0 (renewed) vs the original seed time
	}{
		{
			name:          "requested, empty (authoritative zero result) — C4a's intent: clears",
			anomalies:     costs.AnomalyResult{Requested: true, Marks: nil, Err: nil},
			wantMarksLen:  0,
			wantTTLRenews: true,
		},
		{
			name:          "requested, marks present — replaces and renews",
			anomalies:     costs.AnomalyResult{Requested: true, Marks: []costs.AnomalyMark{newMark}, Err: nil},
			wantMarksLen:  1,
			wantTTLRenews: true,
		},
		{
			name:          "not requested (skip) — X2's intent: never touches marks nor TTL",
			anomalies:     costs.AnomalyResult{Requested: false, Marks: nil, Err: nil},
			wantMarksLen:  1, // the pre-seeded mark survives untouched
			wantTTLRenews: false,
		},
		{
			name:          "requested but errored — non-blocking, but must not clear/renew either",
			anomalies:     costs.AnomalyResult{Requested: true, Marks: nil, Err: errCostsScreenAnomalyFetch},
			wantMarksLen:  1,
			wantTTLRenews: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := costs.LoadStore(profile + "-" + tt.name)
			seedTime := t0.Add(-2 * time.Hour) // distinguishably before t0
			store.PutAnomalies([]costs.AnomalyMark{seedMark}, seedTime, costs.Period{Start: window[0].Start, End: window[len(window)-1].End})

			store.ApplyFetchResult(costs.FetchResult{
				Query:     q,
				Records:   []costs.Record{fullMetricRecord(window[len(window)-1], "Amazon EC2", 100)},
				Anomalies: tt.anomalies,
				Requests:  1,
			}, t0)

			marks, fresh := store.Anomalies(t0)
			if len(marks) != tt.wantMarksLen {
				t.Errorf("marks after ApplyFetchResult: got %d, want %d (%+v)", len(marks), tt.wantMarksLen, marks)
			}
			// TTL check: at t0 exactly, a mark stamped at seedTime (2h before
			// t0) is still within the 24h TTL either way, so "fresh" alone
			// can't distinguish renewed-vs-not. Advance past the ORIGINAL
			// seed's TTL horizon (seedTime+24h) but stay within a
			// hypothetically-renewed-at-t0 horizon (t0+24h) to discriminate.
			_, freshAfterOriginalTTL := store.Anomalies(seedTime.Add(24 * time.Hour))
			if freshAfterOriginalTTL != tt.wantTTLRenews {
				t.Errorf("fresh at original-seed-TTL-horizon = %v, want %v (renewed=%v)", freshAfterOriginalTTL, tt.wantTTLRenews, tt.wantTTLRenews)
			}
			_ = fresh
		})
	}
}

func TestCostsScreen_ApplyFetchResult_AnomalyErr_DoesNotBlockGridMerge(t *testing.T) {
	store := costs.LoadStore("screen-applyfetchresult-err-nonblocking")
	now := fixedCostsNow
	q := baseServiceQuery()
	window := costs.BuildWindow(costs.GranularityMonth, now)

	store.ApplyFetchResult(costs.FetchResult{
		Query:     q,
		Records:   []costs.Record{fullMetricRecord(window[len(window)-1], "Amazon EC2", 250)},
		Anomalies: costs.AnomalyResult{Requested: true, Err: errCostsScreenAnomalyFetch},
	}, now)

	recs, missing := store.Lookup(q, window, now)
	found := false
	for _, r := range recs {
		if len(r.Keys) == 1 && r.Keys[0] == "Amazon EC2" {
			found = true
		}
	}
	if !found {
		t.Errorf("grid Records did not merge when Anomalies.Err was set — an anomaly-fetch failure must not block the accompanying grid data (Requests=%d, missing=%+v)", len(recs), missing)
	}
}

var errCostsScreenAnomalyFetch = &costsScreenTestError{"GetAnomalies: throttled"}

type costsScreenTestError struct{ msg string }

func (e *costsScreenTestError) Error() string { return e.msg }

// ===========================================================================
// Seam 3 — DrillPath.Next (core/costs/screen)
//
// "skips pinned" is the X5 behavior: from usage-type pivot, USAGE_TYPE
// pinned, drilled to SERVICE and pinned that too — Next() must not offer
// USAGE_TYPE again. Production already carries this fix in costs.NextDim
// (round of pins ago); this pins it green on the NEW seam once the coder
// ports the algorithm — a passing result here is expected, not a
// regression signal.
// ===========================================================================

func TestCostsScreen_DrillPath_Next(t *testing.T) {
	tests := []struct {
		name    string
		pinned  []costs.Dimension
		wantDim costs.Dimension
		wantOK  bool
	}{
		{"nothing pinned: SERVICE first", nil, costs.DimensionService, true},
		{
			// Ported from the retired costs.NextDim's TestNextDim_Chain ("pivot
			// SERVICE with SERVICE already pinned in the filter drills to
			// USAGE_TYPE") — the one case not already covered by the X5/
			// canonical-order/leaf cases below (022-codebase-cleanup re-audit).
			"SERVICE alone pinned -> USAGE_TYPE",
			[]costs.Dimension{costs.DimensionService},
			costs.DimensionUsageType, true,
		},
		{
			"X5: USAGE_TYPE then SERVICE both pinned -> RESOURCE_ID, not a redundant USAGE_TYPE",
			[]costs.Dimension{costs.DimensionUsageType, costs.DimensionService},
			costs.DimensionResourceID, true,
		},
		{
			"SERVICE then USAGE_TYPE pinned (canonical chain order) -> RESOURCE_ID",
			[]costs.Dimension{costs.DimensionService, costs.DimensionUsageType},
			costs.DimensionResourceID, true,
		},
		{
			"RESOURCE_ID already pinned: leaf, false",
			[]costs.Dimension{costs.DimensionService, costs.DimensionUsageType, costs.DimensionResourceID},
			"", false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := screen.DrillPath{Pinned: tt.pinned}
			dim, ok := p.Next()
			if dim != tt.wantDim || ok != tt.wantOK {
				t.Errorf("DrillPath{Pinned: %v}.Next() = (%q, %v), want (%q, %v)", tt.pinned, dim, ok, tt.wantDim, tt.wantOK)
			}
		})
	}
}

// ===========================================================================
// Seam 4 — Select outcomes (core/costs/screen)
//
// Reconciles the X6/X7 codex pins where their mechanism moves into this
// seam: X7 (fast Enter through a still-loading level pins an empty value)
// is the "loading -> NoSelection" case (state.Loading now gates
// UNCONDITIONALLY — the old "blind chain" exclusion this seam removes).
// X6 (stale cursor beyond the filtered row count no-ops instead of
// drilling the clamped row) is orthogonal to this seam — clamping happens
// in the reducer BEFORE Select ever sees a row (per the doc's "Selection/
// rendering single source" note), so X6 stays a controller-level pin: by
// the time Select runs, row is already the clamped one.
//
// NoSelection (ponytail-review cut) replaces the former two-outcome
// WaitForRows/NoSelectableRow pair — applyCostsSelect's own switch always
// treated both as the identical no-op, so the two scenarios below assert
// the SAME outcome type now; only the input (Loading vs an unresolved row)
// still distinguishes them, kept for its own documentation value.
// ===========================================================================

// screenTestScreenState/screenTestGridRow/screenTestPeriodRef are this
// file's working hypothesis for the doc's unspecified ScreenState/
// GridRowRef/PeriodRef shapes — see the file-level doc comment.
func screenTestState(rowDim costs.Dimension, pinned []costs.Dimension, filter costs.Filter, gran costs.Granularity, loading bool, now time.Time, resourceTypeFor func(string) (string, bool)) screen.ScreenState {
	return screen.ScreenState{
		RowDim:          rowDim,
		Path:            screen.DrillPath{Pinned: pinned},
		Filter:          filter,
		Granularity:     gran,
		Loading:         loading,
		Now:             now,
		ResourceTypeFor: resourceTypeFor,
	}
}

func TestCostsScreen_Select_LoadingShape_AlwaysWaitForRows(t *testing.T) {
	now := fixedCostsNow
	state := screenTestState(costs.DimensionService, nil, costs.Filter{}, costs.GranularityMonth, true, now, nil)
	row := screen.GridRowRef{Present: true, Value: "Amazon EC2"} // even a resolvable row must not bypass Loading
	cell := screen.PeriodRef{Period: costs.BuildWindow(costs.GranularityMonth, now)[0]}

	got := screen.Select(state, row, cell)
	if _, ok := got.(screen.NoSelection); !ok {
		t.Errorf("Select on a Loading shape = %T, want screen.NoSelection{} — this is X7's fix: a still-loading frame must never let Enter through blind, even a just-pushed one (WaitForRows/NoSelectableRow merged into one outcome — the sole consumer always treated them identically)", got)
	}
}

func TestCostsScreen_Select_CoveredEmpty_NoSelectableRow(t *testing.T) {
	now := fixedCostsNow
	state := screenTestState(costs.DimensionService, nil, costs.Filter{}, costs.GranularityMonth, false, now, nil)
	row := screen.GridRowRef{Present: false}
	cell := screen.PeriodRef{Period: costs.BuildWindow(costs.GranularityMonth, now)[0]}

	got := screen.Select(state, row, cell)
	if _, ok := got.(screen.NoSelection); !ok {
		t.Errorf("Select on a covered-but-empty grid = %T, want screen.NoSelection{} (formerly NoSelectableRow, merged with WaitForRows into one outcome)", got)
	}
}

func TestCostsScreen_Select_RowPresent_PushDrill_NonEmptyValue_WindowWithinSelectedPeriod(t *testing.T) {
	now := fixedCostsNow
	window := costs.BuildWindow(costs.GranularityMonth, now)
	newestMonth := window[len(window)-1]
	state := screenTestState(costs.DimensionService, nil, costs.Filter{}, costs.GranularityMonth, false, now, nil)
	row := screen.GridRowRef{Present: true, Value: "Amazon EC2"}
	cell := screen.PeriodRef{Period: newestMonth}

	got := screen.Select(state, row, cell)
	push, ok := got.(screen.PushDrill)
	if !ok {
		t.Fatalf("Select on a resolvable SERVICE row = %T, want screen.PushDrill", got)
	}
	if push.Value == "" {
		t.Error("PushDrill.Value is empty — a resolved row must never push an empty-value filter pin")
	}
	if push.Value != "Amazon EC2" {
		t.Errorf("PushDrill.Value = %q, want %q", push.Value, "Amazon EC2")
	}
	if push.Dim != costs.DimensionUsageType {
		t.Errorf("PushDrill.Dim = %q, want %q (SERVICE pinned -> next is USAGE_TYPE)", push.Dim, costs.DimensionUsageType)
	}
	if len(push.Window) == 0 {
		t.Fatal("PushDrill.Window is empty")
	}
	first, last := push.Window[0], push.Window[len(push.Window)-1]
	if first.Start < newestMonth.Start || last.End > newestMonth.End {
		t.Errorf("PushDrill.Window %+v is not contained within the selected period %+v (WindowWithin must never trail outside it)", push.Window, newestMonth)
	}
	if push.Granularity != costs.GranularityWeek {
		t.Errorf("PushDrill.Granularity = %q, want %q (drilling a month cell steps one finer: month -> week, same chain the Window itself was built at — Select must never leave the caller to re-derive it)", push.Granularity, costs.GranularityWeek)
	}
	wantWindow := costs.WindowWithin(newestMonth, push.Granularity, now)
	if len(push.Window) != len(wantWindow) {
		t.Fatalf("PushDrill.Window has %d periods, want %d (WindowWithin at PushDrill's own Granularity) — Granularity and Window must describe the SAME chain, not two independently derived ones", len(push.Window), len(wantWindow))
	}
	for i := range wantWindow {
		if push.Window[i] != wantWindow[i] {
			t.Errorf("PushDrill.Window[%d] = %+v, want %+v (WindowWithin(selected period, PushDrill.Granularity, now)) — Window must be built AT the Granularity PushDrill itself reports, not a mismatched pair", i, push.Window[i], wantWindow[i])
		}
	}
}

// TestCostsScreen_Select_RowPresent_PushDrill_Granularity_YearCellStepsToMonth
// pins the OTHER end of the finerGranularity chain the previous test covers
// (month -> week): a year-granularity frame's PushDrill must report
// GranularityMonth, with Window built at that same granularity — the one
// place this decision is made, never re-derived by the caller (the "two
// homes" duplication this pin closes: core/app used to keep its own
// copy of finerGranularity to set the pushed frame's Granularity field,
// even though Select already computed the equivalent chain step internally
// to build Window).
func TestCostsScreen_Select_RowPresent_PushDrill_Granularity_YearCellStepsToMonth(t *testing.T) {
	now := fixedCostsNow
	window := costs.BuildWindow(costs.GranularityYear, now)
	newestYear := window[len(window)-1]
	state := screenTestState(costs.DimensionService, nil, costs.Filter{}, costs.GranularityYear, false, now, nil)
	row := screen.GridRowRef{Present: true, Value: "Amazon EC2"}
	cell := screen.PeriodRef{Period: newestYear}

	got := screen.Select(state, row, cell)
	push, ok := got.(screen.PushDrill)
	if !ok {
		t.Fatalf("Select on a resolvable SERVICE row (year frame) = %T, want screen.PushDrill", got)
	}
	if push.Granularity != costs.GranularityMonth {
		t.Errorf("PushDrill.Granularity = %q, want %q (drilling a year cell steps one finer: year -> month)", push.Granularity, costs.GranularityMonth)
	}
	wantWindow := costs.WindowWithin(newestYear, push.Granularity, now)
	if len(push.Window) != len(wantWindow) {
		t.Fatalf("PushDrill.Window has %d periods, want %d (WindowWithin at PushDrill's own Granularity)", len(push.Window), len(wantWindow))
	}
	for i := range wantWindow {
		if push.Window[i] != wantWindow[i] {
			t.Errorf("PushDrill.Window[%d] = %+v, want %+v (WindowWithin(selected period, PushDrill.Granularity, now))", i, push.Window[i], wantWindow[i])
		}
	}
}

func TestCostsScreen_Select_ResourceLeaf_CatalogMapped_OpenResource(t *testing.T) {
	now := fixedCostsNow
	window := costs.BuildWindow(costs.GranularityMonth, now)
	newestMonth := window[len(window)-1]
	filter := costs.Filter{Equals: map[costs.Dimension][]string{
		costs.DimensionService:   {"Amazon Elastic Compute Cloud - Compute"},
		costs.DimensionUsageType: {"USE1-BoxUsage:m5.large"},
	}}
	resourceTypeFor := func(service string) (string, bool) {
		if service == "Amazon Elastic Compute Cloud - Compute" {
			return "ec2", true
		}
		return "", false
	}
	state := screenTestState(costs.DimensionResourceID, []costs.Dimension{costs.DimensionService, costs.DimensionUsageType}, filter, costs.GranularityDay, false, now, resourceTypeFor)
	row := screen.GridRowRef{Present: true, Value: "i-0a1b2c3d4e5f60001"}
	cell := screen.PeriodRef{Period: newestMonth}

	got := screen.Select(state, row, cell)
	open, ok := got.(screen.OpenResource)
	if !ok {
		t.Fatalf("Select on a resource-leaf row with a catalog-mapped service = %T, want screen.OpenResource", got)
	}
	if open.Locator.Type != "ec2" {
		t.Errorf("Locator.Type = %q, want %q", open.Locator.Type, "ec2")
	}
	if open.Locator.ID != "i-0a1b2c3d4e5f60001" {
		t.Errorf("Locator.ID = %q, want the selected row's Value", open.Locator.ID)
	}
}

func TestCostsScreen_Select_ResourceLeaf_UnmappedService_Refuse(t *testing.T) {
	now := fixedCostsNow
	window := costs.BuildWindow(costs.GranularityMonth, now)
	newestMonth := window[len(window)-1]
	filter := costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionService: {"Tax"}}}
	resourceTypeFor := func(string) (string, bool) { return "", false }
	state := screenTestState(costs.DimensionResourceID, []costs.Dimension{costs.DimensionService, costs.DimensionUsageType}, filter, costs.GranularityDay, false, now, resourceTypeFor)
	row := screen.GridRowRef{Present: true, Value: "some-resource-id"}
	cell := screen.PeriodRef{Period: newestMonth}

	got := screen.Select(state, row, cell)
	refuse, ok := got.(screen.Refuse)
	if !ok {
		t.Fatalf("Select on a resource-leaf row for an unmapped service = %T, want screen.Refuse", got)
	}
	if refuse.Reason == "" {
		t.Error("Refuse.Reason is empty — must be an honest, rendered reason")
	}
}

func TestCostsScreen_Select_ResourceLeaf_OutOfWindow_Refuse(t *testing.T) {
	now := fixedCostsNow
	// A period 6 months before now: far outside CE's 14-day resource-level
	// retention window, so WindowWithin's own clamp leaves nothing.
	staleMonth := costs.BuildWindow(costs.GranularityMonth, now)[0]
	filter := costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionService: {"Amazon Elastic Compute Cloud - Compute"}}}
	resourceTypeFor := func(string) (string, bool) { return "ec2", true }
	state := screenTestState(costs.DimensionResourceID, []costs.Dimension{costs.DimensionService, costs.DimensionUsageType}, filter, costs.GranularityWeek, false, now, resourceTypeFor)
	row := screen.GridRowRef{Present: true, Value: "USE1-BoxUsage:m5.large"}
	cell := screen.PeriodRef{Period: staleMonth}

	got := screen.Select(state, row, cell)
	refuse, ok := got.(screen.Refuse)
	if !ok {
		t.Fatalf("Select on a resource-leaf row whose window falls entirely outside the 14-day resource-drill horizon = %T, want screen.Refuse", got)
	}
	if refuse.Reason == "" {
		t.Error("Refuse.Reason is empty")
	}
}

// ===========================================================================
// Seam 8 — ResourceLocator construction (core/costs/screen)
//
// Region/AccountID were cut from ResourceLocator by a later ponytail-review
// pass (written-never-read — no consumer ever read either field off the
// locator OpenResource carries). The only surviving locator contract is
// Type/ID, already pinned by TestCostsScreen_Select_ResourceLeaf_
// CatalogMapped_OpenResource above; this seam's own
// PopulatesRegionAndAccountFromPinnedFilter test is deleted outright rather
// than trimmed — nothing about the cut fields was worth re-pinning.
// ===========================================================================

// ===========================================================================
// Seam 4 (window half) — TrailingWindow vs WindowWithin (core/costs,
// extended: BuildGrid/ClampResourceDrillWindow, existing core/costs
// functions, need these directly without an import cycle through screen).
//
// X4a landed as a green pin via the new constructor: WindowWithin(year,
// month, now) must return Jan..Dec of the SELECTED year, not a trailing
// 12-month window ending at the year's own January (the exact bug the old
// BuildWindow(Month, anchor) had, per the architecture doc's own framing).
// ===========================================================================

func TestCostsScreen_WindowWithin_YearToMonths_InsideSelectedYear(t *testing.T) {
	now := fixedCostsNow
	yearPeriod := costs.Period{Start: "2026-01-01", End: "2027-01-01"}

	got := costs.WindowWithin(yearPeriod, costs.GranularityMonth, now)
	if len(got) == 0 {
		t.Fatal("WindowWithin(year, month) returned no periods")
	}
	first, last := got[0], got[len(got)-1]
	if first.Start < yearPeriod.Start || last.End > yearPeriod.End {
		t.Errorf("WindowWithin(year=%+v, month) = [%s, %s) — outside the selected year (X4a: must never be a trailing window ending at the year's own January)", yearPeriod, first.Start, last.End)
	}
	if first.Start != yearPeriod.Start {
		t.Errorf("WindowWithin(year, month) first period Start = %q, want the year's own Start %q (Jan of the selected year)", first.Start, yearPeriod.Start)
	}
}

func TestCostsScreen_WindowWithin_MonthToWeeks_InsideSelectedMonth(t *testing.T) {
	now := fixedCostsNow
	monthPeriod := costs.Period{Start: "2026-07-01", End: "2026-08-01"}

	got := costs.WindowWithin(monthPeriod, costs.GranularityWeek, now)
	if len(got) == 0 {
		t.Fatal("WindowWithin(month, week) returned no periods")
	}
	first, last := got[0], got[len(got)-1]
	if first.Start < monthPeriod.Start || last.End > monthPeriod.End {
		t.Errorf("WindowWithin(month=%+v, week) = [%s, %s) — outside the selected month", monthPeriod, first.Start, last.End)
	}
}

func TestCostsScreen_WindowWithin_ClampsToHistoryHorizonAndNextMonthEnd(t *testing.T) {
	// A far-future "now" makes the CE 13-month history horizon bite even for
	// a recent-looking selected year; the open period's own End must never
	// exceed the first of the month after now.
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	openYear := costs.Period{Start: "2026-01-01", End: "2027-01-01"}

	got := costs.WindowWithin(openYear, costs.GranularityMonth, now)
	if len(got) == 0 {
		t.Fatal("WindowWithin(open year, month) returned no periods")
	}
	last := got[len(got)-1]
	cutoff := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
	end, err := costs.ParseDate(last.End)
	if err != nil {
		t.Fatalf("parsing last period End %q: %v", last.End, err)
	}
	if end.After(cutoff) {
		t.Errorf("WindowWithin's last period End %v is after the first-of-next-month ceiling %v", end, cutoff)
	}
}

func TestCostsScreen_TrailingWindow_AnchorAndNow_ClampCeilingComesFromNow(t *testing.T) {
	// TrailingWindow separates the trailing window's END (anchor) from the
	// clamp CEILING (now) — a zoom-out re-anchored on an older cursor period
	// must still clamp against the REAL now, not the (possibly older) anchor.
	anchor := time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC)

	got := costs.TrailingWindow(costs.GranularityMonth, anchor, now)
	if len(got) == 0 {
		t.Fatal("TrailingWindow returned no periods")
	}
	last := got[len(got)-1]
	if last.Start != "2026-01-01" || last.End != "2026-02-01" {
		t.Errorf("TrailingWindow's last (open) period = %+v, want the anchor's own month (Jan 2026), not now's month (Jul 2026)", last)
	}
}

// ===========================================================================
// Seam 5 — Money / TotalOutcome / SumCells (core/costs, extended:
// BuildGrid, existing core/costs, must call SumCells directly)
//
// Absorbs the X8 pin at the domain level (a USD+EUR sum cannot leave the
// domain layer as a bare number); the body-level X8 pin (costs_codex_test.go,
// asserting the RENDERED note) stays as the controller/render acceptance
// test.
// ===========================================================================

func TestCostsScreen_SumCells_SingleUnit_TotalWithUnit(t *testing.T) {
	cells := []costs.Money{{Value: 100, Unit: "USD"}, {Value: 80, Unit: "USD"}}
	got := costs.SumCells(cells)
	if got.Mixed {
		t.Fatal("SumCells over a single unit reported Mixed=true")
	}
	if got.Total.Value != 180 {
		t.Errorf("SumCells Total.Value = %v, want 180", got.Total.Value)
	}
	if got.Total.Unit != "USD" {
		t.Errorf("SumCells Total.Unit = %q, want %q", got.Total.Unit, "USD")
	}
}

func TestCostsScreen_SumCells_MixedUnits_NoTotalValueConsumed(t *testing.T) {
	cells := []costs.Money{{Value: 100, Unit: "USD"}, {Value: 80, Unit: "EUR"}}
	got := costs.SumCells(cells)
	if !got.Mixed {
		t.Error("SumCells over USD+EUR did not report Mixed=true")
	}
}

// ===========================================================================
// Seam 6 (init half) — InitOutcome DELETED (ponytail-review cut: one-field
// bool ceremony wrapping Store.Recovered() with no logic of its own).
// Callers read Store.Recovered() directly now — the real behavior pin lives
// where the corrupt/alien-version recovery actually happens, over a REAL
// Store: TestStore_CorruptYAML_RenamedToBakFreshStoreNoPanic and
// TestStore_AlienVersion_RenamedToBakFreshStoreNoPanic (costs_store_test.go).
// This file's own former InitOutcome test asserted nothing but a struct
// literal's own field — a real coverage gap never opened by deleting it.
// The controller-level X9 flash pin (costs_codex_test.go) stays unchanged
// as the acceptance test that the flash actually renders.
// ===========================================================================

// ===========================================================================
// Seam 8 — CostsViewModel / BuildViewModel (core/costs/screen)
//
// "The renderer and Enter handler share one clamped view-model" — the
// adapter's display filter, cursor clamp, viewport slice, and grid-cache
// key all move here as one pure BuildViewModel. Traced against the CURRENT
// adapter mechanics being extracted (core/app/costs_state.go's
// liveCostGrid/filterCostsZeroDisplayGridRows/costsVisibleColumnRange/
// costsGridCacheKey, core/app/costs_body.go's buildCostsBody
// cursor-clamp block):
//   - display filter: filterCostsZeroDisplayGridRows drops rows whose
//     VISIBLE cells all round to "0.0" (costsAmountRoundsToZero) — EXCEPT
//     liveCostGrid skips this filter entirely for RowDim==LINKED_ACCOUNT
//     (applies costs.ApplyRowAttrs instead), the existing pivot exemption.
//   - cursor clamp: cursorRow clamps to len(grid.Rows)-1 (the FILTERED
//     rows), cursorCol clamps to the window, then a separate relCursorCol
//     re-clamps into the visible slice — Seam 8 unifies this into one
//     Cursor whose Col already indexes VisibleCols.
//   - viewport slice: costsVisibleColumnRange's [start,count) window.
//   - memoization key: costsGridCacheKey folds in grid identity (Query
//     shape + window + metric), cursor/viewport (ViewportCols/ScrollX —
//     the zero-row filter reads the visible range), and Store.Revision().
// ===========================================================================

func costsScreenViewModelGrid(rowDim costs.Dimension, rows []costs.GridRow, columns []costs.Period) costs.Grid {
	return costs.Grid{RowDim: rowDim, Rows: rows, Columns: columns, Totals: make([]costs.CellValue, len(columns))}
}

// ---------------------------------------------------------------------------
// 1. Display filter: sub-cent hidden on SERVICE, kept on LINKED_ACCOUNT;
// offsetting non-zero rows always kept regardless of pivot.
// ---------------------------------------------------------------------------

func TestCostsScreen_BuildViewModel_DisplayFilter_SubCentHidden_LinkedAccountExempt(t *testing.T) {
	columns := []costs.Period{{Start: "2026-07-01", End: "2026-08-01"}}
	subCentRow := costs.GridRow{Key: "Amazon EC2", Label: "Amazon EC2", Cells: []costs.CellValue{{Amount: costs.Amount{Value: 0.004, Unit: "USD"}}}}
	offsettingRow := costs.GridRow{Key: "Amazon RDS", Label: "Amazon RDS", Cells: []costs.CellValue{{Amount: costs.Amount{Value: 100, Unit: "USD"}}}}

	t.Run("SERVICE pivot hides the sub-cent row, keeps the offsetting one", func(t *testing.T) {
		g := costsScreenViewModelGrid(costs.DimensionService, []costs.GridRow{subCentRow, offsettingRow}, columns)
		vm := screen.BuildViewModel(g, costs.DimensionService, screen.CursorPos{}, screen.Viewport{}, 1)
		if len(vm.Rows) != 1 || vm.Rows[0].Key != "Amazon RDS" {
			t.Errorf("SERVICE pivot: got rows %+v, want exactly [Amazon RDS] (sub-cent EC2 row hidden)", vm.Rows)
		}
	})

	t.Run("LINKED_ACCOUNT pivot keeps the sub-cent row (the existing pivot exemption)", func(t *testing.T) {
		g := costsScreenViewModelGrid(costs.DimensionLinkedAccount, []costs.GridRow{subCentRow, offsettingRow}, columns)
		vm := screen.BuildViewModel(g, costs.DimensionLinkedAccount, screen.CursorPos{}, screen.Viewport{}, 1)
		if len(vm.Rows) != 2 {
			t.Errorf("LINKED_ACCOUNT pivot: got %d rows, want 2 (the sub-cent-display filter is exempted on this pivot)", len(vm.Rows))
		}
	})
}

// ---------------------------------------------------------------------------
// 2. Cursor clamp: the returned Cursor always indexes Rows/VisibleCols
// validly, even when the raw input is out of bounds (X6's mechanism).
// ---------------------------------------------------------------------------

func TestCostsScreen_BuildViewModel_CursorClamp_AlwaysValidIndex(t *testing.T) {
	columns := []costs.Period{
		{Start: "2026-05-01", End: "2026-06-01"},
		{Start: "2026-06-01", End: "2026-07-01"},
		{Start: "2026-07-01", End: "2026-08-01"},
	}
	rows := []costs.GridRow{
		{Key: "Amazon EC2", Label: "Amazon EC2", Cells: []costs.CellValue{{Amount: costs.Amount{Value: 100, Unit: "USD"}}, {Amount: costs.Amount{Value: 100, Unit: "USD"}}, {Amount: costs.Amount{Value: 100, Unit: "USD"}}}},
	}
	g := costsScreenViewModelGrid(costs.DimensionService, rows, columns)

	// Cursor far beyond both axes.
	vm := screen.BuildViewModel(g, costs.DimensionService, screen.CursorPos{Row: 99, Col: 99}, screen.Viewport{}, 1)
	if vm.Cursor.Row < 0 || vm.Cursor.Row >= len(vm.Rows) {
		t.Errorf("Cursor.Row = %d does not validly index Rows (len %d)", vm.Cursor.Row, len(vm.Rows))
	}
	if vm.Cursor.Col < 0 || vm.Cursor.Col >= len(vm.VisibleCols) {
		t.Errorf("Cursor.Col = %d does not validly index VisibleCols (len %d)", vm.Cursor.Col, len(vm.VisibleCols))
	}

	// Cursor with negative components — same invariant.
	vmNeg := screen.BuildViewModel(g, costs.DimensionService, screen.CursorPos{Row: -5, Col: -5}, screen.Viewport{}, 1)
	if vmNeg.Cursor.Row < 0 || vmNeg.Cursor.Row >= len(vmNeg.Rows) {
		t.Errorf("negative Cursor.Row = %d does not validly index Rows (len %d)", vmNeg.Cursor.Row, len(vmNeg.Rows))
	}
	if vmNeg.Cursor.Col < 0 || vmNeg.Cursor.Col >= len(vmNeg.VisibleCols) {
		t.Errorf("negative Cursor.Col = %d does not validly index VisibleCols (len %d)", vmNeg.Cursor.Col, len(vmNeg.VisibleCols))
	}

	// Empty Rows (every row filtered/no data): Cursor must still not panic
	// a caller indexing Rows — reported as an explicitly out-of-range zero
	// value is acceptable ONLY when Rows itself is empty.
	emptyGrid := costsScreenViewModelGrid(costs.DimensionService, nil, columns)
	vmEmpty := screen.BuildViewModel(emptyGrid, costs.DimensionService, screen.CursorPos{Row: 3, Col: 1}, screen.Viewport{}, 1)
	if len(vmEmpty.Rows) != 0 {
		t.Fatalf("precondition: expected zero rows, got %d", len(vmEmpty.Rows))
	}
	if vmEmpty.Cursor.Row != 0 {
		t.Errorf("Cursor.Row over an empty Rows set = %d, want 0 (clamped floor)", vmEmpty.Cursor.Row)
	}
}

// ---------------------------------------------------------------------------
// 3. Viewport slice: VisibleCols is the scroll window applied — contents,
// not copies (compare values, not pointer identity — Period is a plain
// value type, so this asserts the SLICE CONTENTS match the expected window,
// not that BuildViewModel reuses the same backing array).
// ---------------------------------------------------------------------------

func TestCostsScreen_BuildViewModel_ViewportSlice_AppliesScrollWindow(t *testing.T) {
	columns := []costs.Period{
		{Start: "2026-04-01", End: "2026-05-01"},
		{Start: "2026-05-01", End: "2026-06-01"},
		{Start: "2026-06-01", End: "2026-07-01"},
		{Start: "2026-07-01", End: "2026-08-01"},
	}
	rows := []costs.GridRow{{Key: "Amazon EC2", Label: "Amazon EC2", Cells: []costs.CellValue{
		{Amount: costs.Amount{Value: 100, Unit: "USD"}}, {Amount: costs.Amount{Value: 100, Unit: "USD"}},
		{Amount: costs.Amount{Value: 100, Unit: "USD"}}, {Amount: costs.Amount{Value: 100, Unit: "USD"}},
	}}}
	g := costsScreenViewModelGrid(costs.DimensionService, rows, columns)

	// A 2-column viewport scrolled to start at index 1: expect columns[1:3].
	vm := screen.BuildViewModel(g, costs.DimensionService, screen.CursorPos{}, screen.Viewport{Cols: 2, ScrollX: 1}, 1)
	if len(vm.VisibleCols) != 2 {
		t.Fatalf("VisibleCols length = %d, want 2", len(vm.VisibleCols))
	}
	if vm.VisibleCols[0] != columns[1] || vm.VisibleCols[1] != columns[2] {
		t.Errorf("VisibleCols = %+v, want %+v (scroll window [1,3))", vm.VisibleCols, columns[1:3])
	}

	// Viewport wider than the grid (or unset, Cols<=0): the whole window is
	// visible — the "unsliced, show everything" fallback.
	vmFull := screen.BuildViewModel(g, costs.DimensionService, screen.CursorPos{}, screen.Viewport{}, 1)
	if len(vmFull.VisibleCols) != len(columns) {
		t.Errorf("VisibleCols length with an unset viewport = %d, want %d (unsliced fallback)", len(vmFull.VisibleCols), len(columns))
	}
}

// ---------------------------------------------------------------------------
// 4. Memoization identity: same inputs -> equal outputs (pure); a revision
// bump (accompanied by genuinely new grid data) yields a rebuilt result.
// Internal caching mechanics (whether/how the seam memoizes) are
// deliberately NOT pinned — only the black-box input/output contract.
// ---------------------------------------------------------------------------

func TestCostsScreen_BuildViewModel_Determinism_SameInputsEqualOutputs(t *testing.T) {
	columns := []costs.Period{{Start: "2026-07-01", End: "2026-08-01"}}
	rows := []costs.GridRow{{Key: "Amazon EC2", Label: "Amazon EC2", Cells: []costs.CellValue{{Amount: costs.Amount{Value: 100, Unit: "USD"}}}}}
	g := costsScreenViewModelGrid(costs.DimensionService, rows, columns)
	cur := screen.CursorPos{Row: 0, Col: 0}
	vp := screen.Viewport{}

	vm1 := screen.BuildViewModel(g, costs.DimensionService, cur, vp, 7)
	vm2 := screen.BuildViewModel(g, costs.DimensionService, cur, vp, 7)

	if len(vm1.Rows) != len(vm2.Rows) || len(vm1.VisibleCols) != len(vm2.VisibleCols) || vm1.Cursor != vm2.Cursor || vm1.Note != vm2.Note {
		t.Errorf("BuildViewModel is not deterministic for identical inputs: %+v vs %+v", vm1, vm2)
	}
	for i := range vm1.Rows {
		if vm1.Rows[i].Key != vm2.Rows[i].Key {
			t.Errorf("Rows[%d] differs across identical calls: %q vs %q", i, vm1.Rows[i].Key, vm2.Rows[i].Key)
		}
	}
}

func TestCostsScreen_BuildViewModel_RevisionBump_WithNewGrid_YieldsRebuiltResult(t *testing.T) {
	columns := []costs.Period{{Start: "2026-07-01", End: "2026-08-01"}}
	cur := screen.CursorPos{Row: 0, Col: 0}
	vp := screen.Viewport{}

	gBefore := costsScreenViewModelGrid(costs.DimensionService, []costs.GridRow{
		{Key: "Amazon EC2", Label: "Amazon EC2", Cells: []costs.CellValue{{Amount: costs.Amount{Value: 100, Unit: "USD"}}}},
	}, columns)
	vmBefore := screen.BuildViewModel(gBefore, costs.DimensionService, cur, vp, 1)

	// A revision bump signals new data landed — the caller passes the
	// resulting NEW grid (a second row now exists) at the bumped revision.
	gAfter := costsScreenViewModelGrid(costs.DimensionService, []costs.GridRow{
		{Key: "Amazon EC2", Label: "Amazon EC2", Cells: []costs.CellValue{{Amount: costs.Amount{Value: 100, Unit: "USD"}}}},
		{Key: "Amazon RDS", Label: "Amazon RDS", Cells: []costs.CellValue{{Amount: costs.Amount{Value: 200, Unit: "USD"}}}},
	}, columns)
	vmAfter := screen.BuildViewModel(gAfter, costs.DimensionService, cur, vp, 2)

	if len(vmAfter.Rows) == len(vmBefore.Rows) {
		t.Errorf("BuildViewModel at a bumped revision with genuinely new grid data returned the same row count (%d) as before — a revision bump must yield a rebuilt result, not a stale one", len(vmBefore.Rows))
	}
}

// ---------------------------------------------------------------------------
// 5. Honesty notes: the empty finer-grain note is sourced from the
// ViewModel now (one source) — pinned for a grid with zero rows.  The
// controller-level X11 pin (costs_codex_test.go) stays as the full-stack
// acceptance test for the depth-gated ("not at root") refinement the
// current adapter also applies; BuildViewModel's own pure signature (grid,
// pivot, cursor, viewport, revision) carries no drill-depth input, so this
// pins the seam's OWN observable contract: an empty grid produces a
// non-empty, granularity-honest Note.
// ---------------------------------------------------------------------------

func TestCostsScreen_BuildViewModel_Note_EmptyFinerGrainHonesty(t *testing.T) {
	columns := []costs.Period{{Start: "2026-07-01", End: "2026-08-01"}}
	// Zero rows: the finer-grain fetch genuinely completed with nothing at
	// this granularity, even though the drilled-into parent cell had money
	// (the scenario this note exists for — a monthly-only charge with no
	// weekly/daily records).
	g := costsScreenViewModelGrid(costs.DimensionUsageType, nil, columns)

	vm := screen.BuildViewModel(g, costs.DimensionUsageType, screen.CursorPos{}, screen.Viewport{}, 1)
	if vm.Note == "" {
		t.Fatal("BuildViewModel over a zero-row grid produced an empty Note — a legitimately empty finer-grain drill must explain itself, not render silently")
	}
	if !strings.Contains(strings.ToLower(vm.Note), "granularity") {
		t.Errorf("Note %q does not explain the granularity mismatch", vm.Note)
	}
}
