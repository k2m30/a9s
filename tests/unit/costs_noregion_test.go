// costs_noregion_test.go — Cost Explorer: the REGION="NoRegion" drill defect,
// live-verified against a real account (aws ce get-dimension-values):
// CE returns region-less charges under GROUPING key "NoRegion", but the
// FILTERABLE value for the REGION dimension is the EMPTY STRING — filtering
// REGION=["NoRegion"] returns $0.00 at every granularity, REGION=[""]
// returns exactly the grid's own monthly amount (5,207.90 for March 2026,
// all Tax). Our drill pins the display key verbatim, so the drilled child
// frame's fetch is filtered on a value CE never matches — zeros forever.
// Second layer: Tax has no daily-granularity records at all, so even a
// correctly-filtered week drill comes back empty — the fix is a
// granularity fallback, not just the filter translation.
//
// package unit_test: reuses newCostsController/topDrill/fixedCostsNow/
// fullMetricRecord/findFetchCostsTask/codexMoveCursorToNewestColumn/
// codexMoveCursorToRow from costs_state_test.go/costs_interaction_test.go/
// costs_codex_test.go, same convention as those files.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/costs/screen"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// ===========================================================================
// N1 — filter translation: screen.Select on a REGION-pivot row keyed
// "NoRegion" must pin the CE FILTERABLE value ("") for REGION, not the
// display key ("NoRegion") verbatim — the root cause of the live defect.
// ===========================================================================

func TestCostsNoRegion_N1_Select_PushDrillPinsCEFilterableEmptyValue(t *testing.T) {
	cell := screen.PeriodRef{Period: costs.Period{Start: "2026-07-01", End: "2026-08-01"}}
	state := screen.ScreenState{
		RowDim:      costs.DimensionRegion,
		Filter:      costs.Filter{},
		Granularity: costs.GranularityMonth,
		Now:         fixedCostsNow,
	}
	row := screen.GridRowRef{Present: true, Value: "NoRegion"}

	out := screen.Select(state, row, cell)
	pd, ok := out.(screen.PushDrill)
	if !ok {
		t.Fatalf("screen.Select on a REGION row keyed %q = %#v (%T), want screen.PushDrill", row.Value, out, out)
	}
	if pd.Value != "" {
		t.Errorf("PushDrill.Value = %q, want \"\" — CE's own filterable value for a region-less charge; pinning the display key %q verbatim filters on a value CE never matches, returning $0.00 forever", pd.Value, row.Value)
	}

	// The resulting fetch shape must differ from a "NoRegion"-valued one —
	// otherwise the cache/fetch layer can't tell the (broken) old shape from
	// the (correct) new one.
	qNoRegion := costs.Query{
		Granularity: pd.Granularity.APIGranularity(),
		GroupBy:     []costs.Dimension{pd.Dim},
		Filter:      costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionRegion: {"NoRegion"}}},
	}
	qTranslated := costs.Query{
		Granularity: pd.Granularity.APIGranularity(),
		GroupBy:     []costs.Dimension{pd.Dim},
		Filter:      costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionRegion: {pd.Value}}},
	}
	if qTranslated.CacheKey() == qNoRegion.CacheKey() {
		t.Errorf("Query.CacheKey() unchanged after translation (%q) — the drill still fetches under the same shape as the broken REGION=%q filter", qTranslated.CacheKey(), "NoRegion")
	}
}

// TestCostsNoRegion_N1_Breadcrumb_RendersDisplayFormNeverBlank drives the
// SAME drill through the full controller: the pinned CE filter value is
// legitimately "" (N1 above), but the breadcrumb/frame-title segment naming
// this drill step must still show the human-readable "NoRegion" — never a
// blank segment a user can't make sense of.
func TestCostsNoRegion_N1_Breadcrumb_RendersDisplayFormNeverBlank(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	newestCol := root.Window[len(root.Window)-1]

	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 2}) // REGION pivot
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: REGION pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query:    payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newestCol, "NoRegion", 5207.90)}},
		Requests: 1,
	})
	if !codexMoveCursorToRow(c, "NoRegion") {
		t.Fatal("precondition: no REGION row for the region-less Tax charge")
	}

	c.Apply(app.Action{Kind: app.ActionSelect})

	vs := c.Snapshot()
	if len(vs.Body.Costs.Breadcrumb) == 0 || vs.Body.Costs.Breadcrumb[0] != "NoRegion" {
		t.Errorf("Breadcrumb = %v, want first segment %q — the pinned CE filter value is legitimately \"\", but the breadcrumb must render the display form, never a blank segment", vs.Body.Costs.Breadcrumb, "NoRegion")
	}
}

// ===========================================================================
// N2 — guard scope: the "never pin a blank value" invariant (screen.go's
// PushDrill doc, X7's own "NeverPinsEmptyValue" contract) must not collide
// with N1's now-legitimate empty pinned FILTER value. It must still refuse a
// genuinely UNRESOLVED row exactly as before.
// ===========================================================================

func TestCostsNoRegion_N2_GuardScope_TranslatedEmptyValueSurvives_UnresolvedRowStillRefused(t *testing.T) {
	cell := screen.PeriodRef{Period: costs.Period{Start: "2026-07-01", End: "2026-08-01"}}
	state := screen.ScreenState{
		RowDim:      costs.DimensionRegion,
		Filter:      costs.Filter{},
		Granularity: costs.GranularityMonth,
		Now:         fixedCostsNow,
	}

	t.Run("translated REGION=\"\" filter value is not refused", func(t *testing.T) {
		row := screen.GridRowRef{Present: true, Value: "NoRegion"}
		out := screen.Select(state, row, cell)
		if _, ok := out.(screen.PushDrill); !ok {
			t.Errorf("Select(...) = %#v (%T), want screen.PushDrill — the guard must not mistake the translated, legitimate REGION=\"\" filter value for an invalid blank pin", out, out)
		}
	})

	t.Run("a genuinely unresolved row is still refused", func(t *testing.T) {
		row := screen.GridRowRef{} // Present: false — nothing to select.
		out := screen.Select(state, row, cell)
		if _, ok := out.(screen.NoSelection); !ok {
			t.Errorf("Select(...) = %#v (%T), want screen.NoSelection (formerly NoSelectableRow, merged with WaitForRows) — an unresolved row must stay refused; N1's translation must not weaken this guard", out, out)
		}
	})
}

// ===========================================================================
// N3 — granularity fallback: Tax has no daily-granularity records at all,
// so even a correctly REGION=""-filtered week drill comes back empty. A
// drilled frame whose fetch delivers zero records while the selected PARENT
// cell was non-zero must re-plan ONCE at the parent's own granularity
// (window narrowed to exactly the selected period) rather than leave a
// silent, permanently empty grid.
// ===========================================================================

// pushNoRegionDrill drives the controller through the REGION pivot, seeds a
// NoRegion row priced amount at period, moves the cursor onto it, and
// presses Enter — returning the resulting week-granularity fetch payload.
// Shared setup for the N3 fallback scenarios below.
func pushNoRegionDrill(t *testing.T, c *app.Controller, period costs.Period, amount float64) messages.CostsLoaded {
	t.Helper()
	_, tasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 2}) // REGION pivot
	payload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: REGION pivot did not emit a fetch task")
	}
	c.Handle(messages.CostsLoaded{
		Query:    payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(period, "NoRegion", amount)}},
		Requests: 1,
	})
	if !codexMoveCursorToRow(c, "NoRegion") {
		t.Fatal("precondition: no REGION row for the region-less Tax charge")
	}
	codexMoveCursorToNewestColumn(c)

	_, tasks = c.Apply(app.Action{Kind: app.ActionSelect}) // -> SERVICE, week granularity
	weekPayload, found := findFetchCostsTask(tasks)
	if !found {
		t.Fatal("precondition: drilling the NoRegion row emitted no fetch task")
	}
	if weekPayload.Query.Granularity != "DAILY" {
		t.Fatalf("precondition: expected the drilled frame's own week-granularity fetch (DAILY), got %q", weekPayload.Query.Granularity)
	}
	return messages.CostsLoaded{Query: weekPayload.Query, Grid: costs.GridResult{Fetched: true}, Requests: 1}
}

func TestCostsNoRegion_N3_GranularityFallback_ReplansOnceAtParentGranularity(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	newestCol := root.Window[len(root.Window)-1]

	zeroWeekDelivery := pushNoRegionDrill(t, c, newestCol, 5207.90)

	// The week-granularity fetch succeeds but genuinely returns zero
	// records — Tax has no daily-granularity records at all, even correctly
	// filtered. The selected parent cell (5207.90) was non-zero, so this
	// must re-plan a coarser fetch rather than stand as a silent empty grid.
	_, fallbackTasks := c.Handle(zeroWeekDelivery)

	monthPayload, found := findFetchCostsTask(fallbackTasks)
	if !found {
		t.Fatal("a zero finer-grain result for a NON-zero parent cell did not re-plan a coarser fetch — the grid would stay silently empty forever")
	}
	if monthPayload.Query.Granularity != "MONTHLY" {
		t.Errorf("fallback re-fetch Granularity = %q, want %q (the parent REGION frame's own granularity)", monthPayload.Query.Granularity, "MONTHLY")
	}
	if monthPayload.Query.Range != newestCol {
		t.Errorf("fallback re-fetch Range = %+v, want exactly the selected period %+v — a fallback must not widen back to a full trailing window", monthPayload.Query.Range, newestCol)
	}
	if got := monthPayload.Query.Filter.Equals[costs.DimensionRegion]; len(got) != 1 || got[0] != "" {
		t.Errorf("fallback re-fetch REGION filter = %v, want [\"\"] — the N1 translated pin must survive the fallback re-plan", got)
	}

	stack := c.GetCostsDrillStack()
	top := stack[len(stack)-1]
	if len(top.Window) != 1 || top.Window[0] != newestCol {
		t.Errorf("frame Window after fallback = %v, want exactly [%+v] (the selected period alone)", top.Window, newestCol)
	}
	if top.Granularity != costs.GranularityMonth {
		t.Errorf("frame Granularity after fallback = %q, want %q", top.Granularity, costs.GranularityMonth)
	}

	// The whole point of the fallback is that the data actually renders —
	// not just a note explaining its absence.
	c.Handle(messages.CostsLoaded{
		Query:    monthPayload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(newestCol, "Tax", 5207.90)}},
		Requests: 1,
	})

	vs := c.Snapshot()
	if vs.Body.Costs.Loading {
		t.Error("post-fallback: still Loading after the coarser fetch's own delivery landed")
	}
	if len(vs.Body.Costs.Rows) != 1 || vs.Body.Costs.Rows[0].Label != "Tax" {
		t.Fatalf("post-fallback Rows = %+v, want exactly one Tax row", vs.Body.Costs.Rows)
	}
	if vs.Body.Costs.FooterNote == "" {
		t.Error("post-fallback FooterNote is empty — the user is shown monthly data with no explanation of why the columns aren't at the drilled (weekly) granularity")
	}
	if !strings.Contains(strings.ToLower(vs.Body.Costs.FooterNote), "month") {
		t.Errorf("post-fallback FooterNote = %q, does not explain the coarser (monthly) granularity actually shown", vs.Body.Costs.FooterNote)
	}
}

func TestCostsNoRegion_N3_GranularityFallback_FiresAtMostOnce(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	newestCol := root.Window[len(root.Window)-1]

	zeroWeekDelivery := pushNoRegionDrill(t, c, newestCol, 5207.90)
	_, fallbackTasks := c.Handle(zeroWeekDelivery)
	monthPayload, found := findFetchCostsTask(fallbackTasks)
	if !found {
		t.Fatal("precondition: the fallback did not dispatch a coarser re-fetch")
	}

	// The coarser re-fetch ALSO comes back empty — the fallback must not
	// chain a second, ever-coarser re-plan.
	_, secondTasks := c.Handle(messages.CostsLoaded{Query: monthPayload.Query, Grid: costs.GridResult{Fetched: true}, Requests: 1})
	if _, found := findFetchCostsTask(secondTasks); found {
		t.Error("a second zero-record delivery after the fallback already fired dispatched YET ANOTHER re-fetch — the fallback must fire at most once per frame")
	}

	vs := c.Snapshot()
	if vs.Body.Costs.FooterNote == "" {
		t.Error("after the fallback's own coarser re-fetch also comes back empty, FooterNote must still explain the empty grid, not go silent")
	}
}

func TestCostsNoRegion_N3_ZeroParentCell_NoFallback(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	olderCol := root.Window[0]

	// The only NoRegion record is priced on an OLDER column — the row
	// survives (its total isn't all-zero), but the SELECTED (newest) cell
	// is genuinely zero: an ordinary empty cell, not a monthly-only charge.
	zeroWeekDelivery := pushNoRegionDrill(t, c, olderCol, 5207.90)

	_, fallbackTasks := c.Handle(zeroWeekDelivery)
	if _, found := findFetchCostsTask(fallbackTasks); found {
		t.Error("a zero finer-grain result for a cell that was ALREADY zero at the parent level triggered a coarser re-fetch — the fallback must only fire when the parent cell was genuinely non-zero")
	}

	vs := c.Snapshot()
	if vs.Body.Costs.FooterNote == "" {
		t.Error("a zero-parent empty finer-grain drill must still explain itself (the pre-existing X11 honesty note)")
	}
}
