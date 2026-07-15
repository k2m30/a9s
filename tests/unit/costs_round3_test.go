// costs_round3_test.go — Cost Explorer: user feedback round 3 (live usage).
//
// package unit (not unit_test): item 1 (CLI -c costs/ce) needs the full TUI
// Model (tui.New/rootApplyMsg/extractMsg, package-unit-only, following
// qa_cli_command_flag_test.go's exact convention) alongside the three
// headless-Controller/pure-costs findings, and Go permits only one package
// per file — everything here lives in package unit with small
// locally-prefixed (round3*) helpers mirroring costs_state_test.go's
// unit_test helpers, to avoid implying they are the same functions across
// packages (same pattern as costs_review_findings_test.go/
// costs_review2_test.go).
//
// Item 1 tests only the runtime/TUI navigation half of "-c costs"/"-c ce"
// (tui.WithCommand onward, matching qa_cli_command_flag_test.go's
// convention); cmd/a9s/main.go's own flag VALIDATION is package main and
// covered separately by main_wiring_test.go.
package unit

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

var round3Now = time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

// newCostsScreenController is the single shared package-unit costs
// controller builder. Builds a Controller with ScreenCosts pushed and
// EnsureCostsState seeded under a fresh, isolated A9S_CONFIG_FOLDER.
// Blessed in qa_controller_construction_discipline_test.go's
// ccdBlessedHelpers.
//
// package unit_test's equivalent is costs_state_test.go's
// newCostsController — the two cannot be merged across the package
// boundary (package unit needs TUI helpers unavailable to unit_test).
func newCostsScreenController(t *testing.T, now time.Time) *app.Controller {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "test-profile"
	s.Region = "us-east-1"
	core := runtime.New(s, nil)
	c := app.New(core)
	t.Cleanup(c.Close)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c.EnsureCostsState(now)
	return c
}

// round3MonthRecord builds one costs.Record for rowKey, priced amount,
// within now's own month.
func round3MonthRecord(now time.Time, rowKey string, amount float64) costs.Record {
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	return costs.Record{
		Period:  costs.Period{Start: start.Format("2006-01-02"), End: end.Format("2006-01-02")},
		Keys:    []string{rowKey},
		Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: amount, Unit: "USD"}},
	}
}

// ===========================================================================
// 1 (P1) — CLI entry: "-c costs" and "-c ce" auto-open the Cost Explorer
// screen on start.
// ===========================================================================

func TestCostsRound3_CLICommand_EmitsNavigateTargetCosts(t *testing.T) {
	for _, cliCmd := range []string{"costs", "ce"} {
		t.Run(cliCmd, func(t *testing.T) {
			m := tui.New(
				"demo", "us-east-1",
				tui.WithClients(demo.NewServiceClients()),
				tui.WithNoCache(true),
				tui.WithCommand(cliCmd),
			)
			m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})

			_, cmd := rootApplyMsg(m, messages.ClientsReady{
				Clients: demo.NewServiceClients(),
				Region:  "us-east-1",
			})

			nav := extractMsg(t, cmd, func(msg tea.Msg) bool {
				_, ok := msg.(messages.Navigate)
				return ok
			})
			navMsg, ok := nav.(messages.Navigate)
			if !ok {
				t.Fatalf("expected messages.Navigate, got %T", nav)
			}
			if navMsg.Target != messages.TargetCosts {
				t.Errorf("-c %s: NavigateMsg.Target got %v want messages.TargetCosts (got ResourceType=%q — today \"costs\" falls through to the generic NavigateTargetResourceList path, which has no such registered type)", cliCmd, navMsg.Target, navMsg.ResourceType)
			}
		})
	}
}

// ===========================================================================
// 2 (P2) — SERVICE-pivot rows render without the "Amazon "/"AWS " vendor
// prefix; the label column stays wide enough that a full un-prefixed
// service name survives untruncated.
// ===========================================================================

func TestCostsRound3_ServiceLabel_StripsVendorPrefix(t *testing.T) {
	c := newCostsScreenController(t, round3Now)
	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}
	c.Handle(messages.CostsLoaded{
		Query: q,
		Grid: costs.GridResult{Fetched: true, Records: []costs.Record{
			round3MonthRecord(round3Now, "Amazon Elastic Compute Cloud - Compute", 1200.0),
			round3MonthRecord(round3Now, "AWS Lambda", 400.0),
			round3MonthRecord(round3Now, "Amazon Simple Storage Service", 300.0),
		}},
		Requests: 1,
	})

	vs := c.Snapshot()
	gotLabels := make([]string, len(vs.Body.Costs.Rows))
	for i, r := range vs.Body.Costs.Rows {
		gotLabels[i] = r.Label
	}

	wantLabels := []string{"Elastic Compute Cloud - Compute", "Lambda", "Simple Storage Service"}
	for _, want := range wantLabels {
		if !slices.Contains(gotLabels, want) {
			t.Errorf("expected a row labeled %q (vendor prefix stripped), got labels: %v", want, gotLabels)
		}
	}
	for _, got := range gotLabels {
		if strings.HasPrefix(got, "Amazon ") || strings.HasPrefix(got, "AWS ") {
			t.Errorf("row label %q still carries the vendor prefix", got)
		}
	}
}

func TestCostsRound3_ServiceLabel_ColumnWideEnoughToAvoidTruncation(t *testing.T) {
	longLabel := "Elastic Compute Cloud - Compute" // 32 chars, post vendor-prefix-strip
	body := app.CostsBody{
		Pivot:       "SERVICE",
		Metric:      "invoice",
		Granularity: "month",
		Columns:     []app.CostColumn{{Label: "Jul'26", Open: true}},
		Rows:        []app.CostRow{{Label: longLabel, Cells: []app.CostCell{{Amount: "1,200.0"}}}},
		Totals:      []app.CostCell{{Amount: "1,200.0"}},
	}

	out := tuitest.StripANSI(views.RenderCosts(body, 160, 32))
	if !strings.Contains(out, longLabel) {
		t.Errorf("rendered output at width=160 does not contain the full untruncated label %q (%d chars) — the label column must be wide enough to fit it:\n%s", longLabel, len(longLabel), out)
	}
}

// ===========================================================================
// 3 (P2) — fold REMOVED (spec.md Edge Cases "Many small rows"): every
// non-zero row renders individually (no "… others" rollup, regardless of
// magnitude), zero-only rows are hidden, TOTAL stays pinned; vertical
// scroll keeps the cursor row visible and TOTAL pinned as the cursor moves
// past the initial viewport.
// ===========================================================================

func TestCostsRound3_Fold_Removed_AllNonZeroRowsRenderIndividually_ZeroOnlyHidden(t *testing.T) {
	window := []costs.Period{{Start: "2026-07-01", End: "2026-08-01"}}
	var recs []costs.Record
	const smallRowCount = 30
	for i := range smallRowCount {
		recs = append(recs, costs.Record{
			Period:  window[0],
			Keys:    []string{fmt.Sprintf("Service%02d", i)},
			Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 1.0 + float64(i)*0.1, Unit: "USD"}},
		})
	}
	for i := range 3 {
		recs = append(recs, costs.Record{
			Period:  window[0],
			Keys:    []string{fmt.Sprintf("ZeroService%d", i)},
			Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 0, Unit: "USD"}},
		})
	}

	grid := costs.BuildGrid(recs, costs.MetricInvoice, window)

	if len(grid.Rows) != smallRowCount {
		t.Fatalf("len(Rows) = %d, want %d (all 30 non-zero rows render individually, no fold, 3 zero-only rows hidden)", len(grid.Rows), smallRowCount)
	}
	for _, r := range grid.Rows {
		if strings.Contains(r.Label, "others") {
			t.Errorf("found a rollup row %q — fold was removed per spec.md Edge Cases \"Many small rows\"", r.Label)
		}
		if strings.HasPrefix(r.Key, "ZeroService") {
			t.Errorf("zero-only row %q must be hidden, not rendered", r.Key)
		}
	}
}

// TestCostsRound3_VerticalScroll_CursorVisible_TotalPinned_AsCursorMovesBeyondViewport
// tests the OBSERVABLE rendered behavior, not the (dead, never-mutated)
// DrillLevel.ScrollY field: RenderCosts's own clipCostsRows already
// cursor-centers the visible data-row window and keeps header/TOTAL pinned,
// independent of ScrollY — if this is already correct today it is a green
// regression pin; this test's row-count precondition still requires fold
// removal (item 3's other half) to have landed first.
func TestCostsRound3_VerticalScroll_CursorVisible_TotalPinned_AsCursorMovesBeyondViewport(t *testing.T) {
	c := newCostsScreenController(t, round3Now)
	const rowCount = 30
	var recs []costs.Record
	for i := range rowCount {
		// Strictly decreasing amounts so sort order (desc by total) is
		// deterministic and distinct from insertion order.
		recs = append(recs, round3MonthRecord(round3Now, fmt.Sprintf("Service%02d", i), float64(rowCount-i)))
	}
	c.Handle(messages.CostsLoaded{
		Query:    costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}},
		Grid:     costs.GridResult{Fetched: true, Records: recs},
		Requests: 1,
	})

	vs := c.Snapshot()
	if len(vs.Body.Costs.Rows) != rowCount {
		t.Fatalf("precondition: expected %d rows (requires fold removal to have landed), got %d", rowCount, len(vs.Body.Costs.Rows))
	}

	const height = 12 // small viewport: header + fewer than 30 data rows + TOTAL
	for range 20 {
		c.Apply(app.Action{Kind: app.ActionMoveDown})
	}
	vs = c.Snapshot()
	body := *vs.Body.Costs
	cursorRow := body.CursorRow
	if cursorRow < 0 || cursorRow >= len(body.Rows) {
		t.Fatalf("cursor row %d out of range [0,%d)", cursorRow, len(body.Rows))
	}
	cursorLabel := body.Rows[cursorRow].Label

	out := tuitest.StripANSI(views.RenderCosts(body, 160, height))
	if !strings.Contains(out, cursorLabel) {
		t.Errorf("after moving the cursor to row %d (%q), the rendered output does not show that row — the cursor must remain visible as the grid vertically scrolls:\n%s", cursorRow, cursorLabel, out)
	}
	if !strings.Contains(out, "TOTAL") {
		t.Errorf("TOTAL row missing from the rendered output after scrolling:\n%s", out)
	}
}

// ===========================================================================
// 4 (P2) — zero-valued cells carry NO delta color tag (neutral), regardless
// of the period-over-period delta math.
// ===========================================================================

func TestCostsRound3_ZeroValueCell_NeverColoredAsGrowthOrDrop(t *testing.T) {
	c := newCostsScreenController(t, round3Now)
	topSnap := c.Snapshot()
	if topSnap.Body.Kind != app.BodyKindCosts {
		t.Fatalf("expected BodyKindCosts, got %q", topSnap.Body.Kind)
	}
	stack := c.GetCostsDrillStack()
	if len(stack) == 0 {
		t.Fatal("GetCostsDrillStack returned an empty stack")
	}
	window := stack[len(stack)-1].Window
	if len(window) < 2 {
		t.Fatalf("precondition: window has %d columns, need at least 2", len(window))
	}
	prev := window[len(window)-2]
	last := window[len(window)-1]

	recs := []costs.Record{
		// A row with real spend that drops to EXACTLY zero in the last
		// (current) column — today this computes a -100% delta and gets
		// tagged "drop"; the fix must pin a zero-value cell as neutral
		// regardless of the delta math.
		{Period: prev, Keys: []string{"EC2 - Other"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 500, Unit: "USD"}}},
		{Period: last, Keys: []string{"EC2 - Other"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 0, Unit: "USD"}}},
		// Control: a genuine (non-zero) growth transition must still be
		// tagged — the fix must not suppress delta coloring entirely.
		{Period: prev, Keys: []string{"Elastic Load Balancing"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 100, Unit: "USD"}}},
		{Period: last, Keys: []string{"Elastic Load Balancing"}, Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 200, Unit: "USD"}}},
	}
	c.Handle(messages.CostsLoaded{
		Query:    costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}},
		Grid:     costs.GridResult{Fetched: true, Records: recs},
		Requests: 1,
	})

	vs := c.Snapshot()
	var zeroRow, growthRow *app.CostRow
	for i := range vs.Body.Costs.Rows {
		switch vs.Body.Costs.Rows[i].Label {
		case "EC2 - Other":
			zeroRow = &vs.Body.Costs.Rows[i]
		case "Elastic Load Balancing":
			growthRow = &vs.Body.Costs.Rows[i]
		}
	}
	if zeroRow == nil {
		t.Fatal("precondition: no \"EC2 - Other\" row found")
	}
	if growthRow == nil {
		t.Fatal("precondition: no \"Elastic Load Balancing\" row found")
	}

	zeroCell := zeroRow.Cells[len(zeroRow.Cells)-1]
	if zeroCell.Amount != "0.0" {
		t.Fatalf("precondition: EC2 - Other's last cell Amount got %q want \"0.0\"", zeroCell.Amount)
	}
	// costs_quality_test.go, item 1: DeltaTag is now one of 5 values
	// (neutral / growth-soft / growth-strong / drop-soft / drop-strong) —
	// a zero-value cell must be exactly "neutral", not any growth/drop tier.
	if zeroCell.DeltaTag != "neutral" {
		t.Errorf("a zero-value cell got DeltaTag %q — a cell whose own amount is 0.0 must be neutral (no color) regardless of the period-over-period delta math (a $500 -> $0.0 transition is not a meaningful \"drop\" signal)", zeroCell.DeltaTag)
	}

	// $100 -> $200 is +100%, unambiguously the "strong" growth tier
	// (>=25%) under item 1's 4-tier scale.
	growthCell := growthRow.Cells[len(growthRow.Cells)-1]
	if growthCell.DeltaTag != "growth-strong" {
		t.Errorf("control failed: Elastic Load Balancing's genuine $100 -> $200 growth got DeltaTag %q want %q — the fix must not suppress delta coloring for non-zero cells", growthCell.DeltaTag, "growth-strong")
	}
}
