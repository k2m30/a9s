// costs_ui_polish_test.go — Cost Explorer: regression pins for the user's
// screenshot-review batch.
//
// package unit_test (not unit): none of these items need TUI-level helpers —
// a pure views.RenderCosts call, a headless Controller via newCostsController
// (costs_state_test.go, same package), or a pure costs.BuildGrid/
// domain.HelpGroupsFor call cover every case.
package unit_test

import (
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// costsUIPolishBody builds a CostsBody with n synthetic time columns/cells —
// enough to force a fractional/partial trailing column at several widths if
// the broken-right-border guard (RenderCosts's costsFittingCols) regressed.
func costsUIPolishBody(n int) app.CostsBody {
	cols := make([]app.CostColumn, n)
	cells := make([]app.CostCell, n)
	for i := range cols {
		cols[i] = app.CostColumn{Label: "Col" + string(rune('A'+i%26))}
		cells[i] = app.CostCell{Amount: "1,234.5"}
	}
	return app.CostsBody{
		Pivot:       "SERVICE",
		Metric:      "invoice",
		Granularity: "month",
		Columns:     cols,
		Rows: []app.CostRow{
			{Label: "Amazon EC2", Cells: cells},
			{Label: "Amazon RDS", Cells: cells},
		},
		Totals:      cells,
		FooterNote:  "Δ vs prev: +12.4%",
		DataThrough: "2026-07-09",
	}
}

// ===========================================================================
// 1 — every RenderCosts grid line is exactly the requested width; no line
// ever exceeds it (the broken-right-border overflow bug).
// ===========================================================================

func TestCostsUIPolish_RenderCosts_GridLinesExactWidth_NeverOverflow(t *testing.T) {
	body := costsUIPolishBody(15)
	// 60/79/137 are deliberately NOT round multiples of a column width, so a
	// regression in the fitting-column guard would previously have produced
	// a partial trailing column that overflowed the requested width.
	for _, width := range []int{60, 79, 137} {
		out := tuitest.StripANSI(views.RenderCosts(body, width, 32))
		lines := strings.Split(out, "\n")
		if len(lines) < 3 {
			t.Fatalf("width=%d: expected at least header+data+TOTAL lines, got %d lines", width, len(lines))
		}
		// header, every data row, and TOTAL are the grid lines the frame's
		// right border sits against — each must be EXACTLY width.
		gridLines := lines[:len(lines)-2] // last two are the blank separator + footer
		for i, l := range gridLines {
			if w := lipgloss.Width(l); w != width {
				t.Errorf("width=%d line=%d: got width %d, want exactly %d — overflow would break the frame's right border:\n%q", width, i, w, width, l)
			}
		}
		// No line, anywhere, may exceed the requested width.
		for i, l := range lines {
			if w := lipgloss.Width(l); w > width {
				t.Errorf("width=%d line=%d: got width %d, exceeds requested width %d:\n%q", width, i, w, width, l)
			}
		}
	}
}

// ===========================================================================
// 2 — the footer renders no "CE calls" element.
// ===========================================================================

func TestCostsUIPolish_Footer_NeverContainsCECalls(t *testing.T) {
	body := costsUIPolishBody(3)
	body.APICalls = 7
	body.APICostUSD = "$0.07"
	out := tuitest.StripANSI(views.RenderCosts(body, 120, 32))
	if strings.Contains(out, "CE calls") {
		t.Errorf("rendered output contains the removed \"CE calls\" footer element:\n%s", out)
	}
	if strings.Contains(out, body.APICostUSD) {
		t.Errorf("rendered output contains the removed API cost estimate %q:\n%s", body.APICostUSD, out)
	}
}

// ===========================================================================
// 3 — ViewState.Footer for the costs screen carries CostsFooterHintsFor's
// hint set (b/+/-/digits/enter/esc/ctrl+r); single source, no renderer-side
// duplicate (cross-checked: internal/tui has no hardcoded "Metric"/"Zoom"/
// "Pivot"/"Drill" footer-hint literal anywhere outside viewstate.go).
// ===========================================================================

func TestCostsUIPolish_Footer_CarriesCostsFooterHintsFor_SingleSource(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	vs := c.Snapshot()

	want := app.CostsFooterHintsFor("") // newCostsController never sets uiMode -> "" (TUI)
	if !reflect.DeepEqual(vs.Footer, want) {
		t.Errorf("Snapshot().Footer = %+v, want exactly CostsFooterHintsFor(\"\") = %+v (Footer must be sourced from CostsFooterHintsFor, never a hand-rolled duplicate)", vs.Footer, want)
	}

	wantKeys := []string{"b", "+/-", "0-9", "Enter", "Esc", "ctrl+r"}
	for _, k := range wantKeys {
		if !slices.ContainsFunc(vs.Footer, func(h app.KeyHint) bool { return h.Key == k }) {
			t.Errorf("Snapshot().Footer missing key hint %q", k)
		}
	}
}

// ===========================================================================
// 4 — FR-002 "open at today": the cursor sits on the newest period and is
// VISIBLE (ScrollX positions it) on open, after pivot reset, and after
// zoom-out.
// ===========================================================================

func TestCostsUIPolish_CursorOpensOnNewestPeriod_AndIsVisible(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)

	const viewportCols = 3
	assertCursorVisibleAtNewest := func(label string) {
		t.Helper()
		top := topDrill(t, c)
		windowLen := len(top.Window)
		if windowLen == 0 {
			t.Fatalf("%s: window is empty", label)
		}
		if top.Cursor.Col != windowLen-1 {
			t.Errorf("%s: Cursor.Col got %d want %d (newest/rightmost period)", label, top.Cursor.Col, windowLen-1)
		}
		wantScrollX := max(windowLen-viewportCols, 0)
		if top.ScrollX != wantScrollX {
			t.Errorf("%s: ScrollX got %d want %d (the cursor's column must be visible in the viewport)", label, top.ScrollX, wantScrollX)
		}
		if top.Cursor.Col < top.ScrollX || top.Cursor.Col >= top.ScrollX+viewportCols {
			t.Errorf("%s: cursor column %d is outside the visible viewport [%d,%d)", label, top.Cursor.Col, top.ScrollX, top.ScrollX+viewportCols)
		}
	}

	// On open.
	c.SetCostsViewportCols(viewportCols)
	assertCursorVisibleAtNewest("on open")

	// After pivot reset (digit 0).
	c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // move off default first, so the reset is observable
	c.Apply(app.Action{Kind: app.ActionCostPivot, N: 0})
	assertCursorVisibleAtNewest("after pivot reset")

	// After zoom-out: zoom IN (month -> week, bounded) then back OUT
	// (week -> month, trailing-anchored) — the FR-002 re-land applies only
	// to the trailing-anchored (month/year) landing.
	c.Apply(app.Action{Kind: app.ActionCostZoomIn})
	c.Apply(app.Action{Kind: app.ActionCostZoomOut})
	assertCursorVisibleAtNewest("after zoom-out")
}

// ===========================================================================
// 5 — the costs context's help sections include the Cost Explorer section
// with the documented keys (HelpFromCosts / domain.costsSections via
// HelpGroupsFor).
// ===========================================================================

func TestCostsUIPolish_HelpSections_IncludeCostExplorerKeys(t *testing.T) {
	sections := domain.HelpGroupsFor(domain.HelpFromCosts, "ctrl+z")

	var costSection *domain.HelpSection
	for i := range sections {
		if sections[i].Title == "COST EXPLORER" {
			costSection = &sections[i]
			break
		}
	}
	if costSection == nil {
		t.Fatalf("HelpGroupsFor(HelpFromCosts, ...) has no \"COST EXPLORER\" section, got sections: %+v", sections)
	}

	wantKeys := []string{"b", "+/-", "0-9", "enter", "h/l", "j/k"}
	for _, k := range wantKeys {
		if !slices.ContainsFunc(costSection.Hints, func(hint domain.HelpHint) bool { return hint.Key == k }) {
			t.Errorf("COST EXPLORER help section missing key %q, got hints: %+v", k, costSection.Hints)
		}
	}

	// esc and ctrl+r live in the "OTHER" section for every other context —
	// confirm the costs context follows the same shape rather than
	// inventing a separate layout.
	var otherSection *domain.HelpSection
	for i := range sections {
		if sections[i].Title == "OTHER" {
			otherSection = &sections[i]
			break
		}
	}
	if otherSection == nil {
		t.Fatal("HelpGroupsFor(HelpFromCosts, ...) has no \"OTHER\" section")
	}
	for _, k := range []string{"ctrl+r", "esc"} {
		if !slices.ContainsFunc(otherSection.Hints, func(hint domain.HelpHint) bool { return hint.Key == k }) {
			t.Errorf("OTHER help section (costs context) missing key %q, got hints: %+v", k, otherSection.Hints)
		}
	}
}

// ===========================================================================
// 6 — sort/determinism: rows sort desc by ABSOLUTE total (a large negative
// credit row ranks by its true magnitude); NaN-producing rows keep
// deterministic order across repeated runs. The noise-floor fold itself was
// removed (spec.md Edge Cases "Many small rows" — see costs_round3_test.go).
// ===========================================================================

func costsUIPolishRecord(period costs.Period, key string, amount float64) costs.Record {
	return costs.Record{
		Period:  period,
		Keys:    []string{key},
		Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: amount, Unit: "USD"}},
	}
}

func TestCostsUIPolish_LargeNegativeRow_SortsByAbsoluteTotal_NoFold(t *testing.T) {
	window := []costs.Period{{Start: "2026-07-01", End: "2026-08-01"}}
	recs := []costs.Record{
		costsUIPolishRecord(window[0], "Dominant", 10000),
		costsUIPolishRecord(window[0], "BigCredit", -8000), // large negative, ranks by |total|
		costsUIPolishRecord(window[0], "TinyA", 5),
		costsUIPolishRecord(window[0], "TinyB", 3),
	}

	grid := costs.BuildGrid(recs, costs.MetricInvoice, window)

	wantOrder := []string{"Dominant", "BigCredit", "TinyA", "TinyB"}
	if len(grid.Rows) != len(wantOrder) {
		t.Fatalf("expected %d rows (fold removed — every row renders individually), got %d: %+v", len(wantOrder), len(grid.Rows), grid.Rows)
	}
	for i, want := range wantOrder {
		if grid.Rows[i].Key != want {
			t.Errorf("row %d: got Key %q want %q (rows sort desc by ABSOLUTE total: a large negative row ranks by its true magnitude, never folded)", i, grid.Rows[i].Key, want)
		}
	}
}

func TestCostsUIPolish_NaNRow_DeterministicOrder_AcrossRepeatedRuns(t *testing.T) {
	window := []costs.Period{{Start: "2026-07-01", End: "2026-08-01"}}
	recs := []costs.Record{
		costsUIPolishRecord(window[0], "NaNRow", math.NaN()),
		costsUIPolishRecord(window[0], "A", 500),
		costsUIPolishRecord(window[0], "B", 300),
	}

	rowOrder := func() []string {
		grid := costs.BuildGrid(recs, costs.MetricInvoice, window)
		out := make([]string, len(grid.Rows))
		for i, r := range grid.Rows {
			out[i] = r.Key
		}
		return out
	}

	order1 := rowOrder()
	order2 := rowOrder()

	if !reflect.DeepEqual(order1, order2) {
		t.Fatalf("BuildGrid row order is not deterministic across repeated runs on identical input: run1=%v run2=%v", order1, order2)
	}
	want := []string{"A", "B", "NaNRow"}
	if !reflect.DeepEqual(order1, want) {
		t.Errorf("row order got %v want %v (NaN total pinned last, never folded away, never poisoning the sort of the well-defined rows)", order1, want)
	}
}

// ===========================================================================
// 7 — "-0.0" never appears in any rendered amount.
// ===========================================================================

func TestCostsUIPolish_NegativeZero_NeverRendered(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	q := costs.Query{
		Granularity: costs.GranularityMonth.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionService},
	}
	// The -0.04 cell must sit alongside a real-spend cell IN THE SAME ROW
	// (a second, prior-month period): the display-level zero-row filter
	// (round6, item 9) hides a row only when EVERY one of its own visible
	// cells formats as "0.0" — a lone -0.04 record would now be hidden
	// outright before ever reaching the "-0.0" rendering check this test
	// exists to pin. A real prior-month amount keeps the row (and its
	// current-month -0.04 cell) visible without changing what's asserted.
	curStart := time.Date(fixedCostsNow.Year(), fixedCostsNow.Month(), 1, 0, 0, 0, 0, time.UTC)
	curEnd := curStart.AddDate(0, 1, 0)
	prevStart := curStart.AddDate(0, -1, 0)
	curPeriod := costs.Period{Start: curStart.Format("2006-01-02"), End: curEnd.Format("2006-01-02")}
	prevPeriod := costs.Period{Start: prevStart.Format("2006-01-02"), End: curStart.Format("2006-01-02")}
	rec := func(period costs.Period, amount float64) costs.Record {
		return costs.Record{
			Period:  period,
			Keys:    []string{"Amazon EC2"},
			Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: amount, Unit: "USD"}},
		}
	}
	c.Handle(messages.CostsLoaded{
		Query: q,
		Grid: costs.GridResult{Fetched: true, Records: []costs.Record{
			rec(prevPeriod, 500.0),
			rec(curPeriod, -0.04), // rounds to "0.0" at 1 decimal, but is negative
		}},
		Requests: 1,
	})

	vs := c.Snapshot()
	if len(vs.Body.Costs.Rows) == 0 {
		t.Fatal("precondition: no rows after seeding the -0.04 record")
	}
	for _, row := range vs.Body.Costs.Rows {
		for _, cell := range row.Cells {
			if cell.Amount == "-0.0" {
				t.Errorf("row %q rendered a cell as \"-0.0\" — a tiny negative rounding to zero at one decimal must render as \"0.0\"", row.Label)
			}
		}
	}
	for _, cell := range vs.Body.Costs.Totals {
		if cell.Amount == "-0.0" {
			t.Error("TOTAL row rendered a cell as \"-0.0\"")
		}
	}

	out := tuitest.StripANSI(views.RenderCosts(*vs.Body.Costs, 120, 32))
	if strings.Contains(out, "-0.0") {
		t.Errorf("rendered costs grid contains the literal substring \"-0.0\":\n%s", out)
	}
}

// ===========================================================================
// 8 — Enter on a drill-refused row surfaces the honest reason in
// FooterNote (DrillRefusedReason), not a silent no-op.
// ===========================================================================

func TestCostsUIPolish_DrillRefused_SurfacesHonestReasonInFooter_NotSilentNoOp(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	root := topDrill(t, c)
	if len(root.Window) < 2 {
		t.Fatalf("precondition: root window has %d columns, need at least 2", len(root.Window))
	}

	// Move the cursor to the OLDEST column (index 0) — far more than 14
	// days before fixedCostsNow, so the eventual RESOURCE_ID drill's window
	// is refused even after ClampResourceDrillWindow.
	for range len(root.Window) - 1 {
		c.Apply(app.Action{Kind: app.ActionScrollLeft})
	}
	if got := topDrill(t, c).Cursor.Col; got != 0 {
		t.Fatalf("precondition: cursor did not land on column 0, got %d", got)
	}

	// Loading (screen.Select's WaitForRows) gates Select unconditionally
	// now — the root SERVICE shape's own fetch must land before Enter can
	// drill anywhere at all.
	_, rootTasks := c.Apply(app.Action{Kind: app.ActionCostPivot, N: 1}) // SERVICE, already current — dispatches the shape-miss fetch
	rootPayload, found := findFetchCostsTask(rootTasks)
	if !found {
		t.Fatal("precondition: root SERVICE shape did not emit a fetch task")
	}
	oldestCol := root.Window[0]
	c.Handle(messages.CostsLoaded{
		Query:    rootPayload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(oldestCol, "Amazon EC2", 100)}},
		Requests: 1,
	})
	// The cursor column is preserved across a pivot now (it no longer
	// resets to 0), so it's still on the oldest column from the scroll
	// above.
	if got := topDrill(t, c).Cursor.Col; got != 0 {
		t.Fatalf("precondition: cursor column drifted off the oldest column after the pivot, got %d", got)
	}

	// First Enter: pins SERVICE, drills to USAGE_TYPE (never gated).
	_, drill1Tasks := c.Apply(app.Action{Kind: app.ActionSelect})
	if len(c.GetCostsDrillStack()) != 2 {
		t.Fatalf("precondition: first drill did not reach depth 2, got %d", len(c.GetCostsDrillStack()))
	}
	drill1Payload, found := findFetchCostsTask(drill1Tasks)
	if !found {
		t.Fatal("precondition: SERVICE -> USAGE_TYPE drill did not emit a fetch task")
	}
	usageTypeTop := topDrill(t, c)
	c.Handle(messages.CostsLoaded{
		Query:    drill1Payload.Query,
		Grid:     costs.GridResult{Fetched: true, Records: []costs.Record{fullMetricRecord(usageTypeTop.Window[0], "USE1-BoxUsage:m5.large", 100)}},
		Window:   usageTypeTop.Window,
		Requests: 1,
	})

	// Second Enter: attempts the RESOURCE_ID drill — must be refused, not a
	// silent no-op.
	vs, _ := c.Apply(app.Action{Kind: app.ActionSelect})

	stack := c.GetCostsDrillStack()
	if len(stack) != 2 {
		t.Fatalf("resource drill should have been refused (stack stays at depth 2), got depth %d", len(stack))
	}
	if vs.Body.Costs.FooterNote == "" {
		t.Fatal("drill-refused: FooterNote is empty — the refusal must surface an honest reason, not silently do nothing")
	}
	if !strings.Contains(vs.Body.Costs.FooterNote, "14 days") {
		t.Errorf("drill-refused FooterNote got %q, want it to explain the 14-day retention limit", vs.Body.Costs.FooterNote)
	}
}
