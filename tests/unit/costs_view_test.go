// costs_view_test.go — Cost Explorer Phase 2: TUI renderer parity with
// wireframe.md (specs/021-cost-explorer/wireframe.md).
//
// Target: views.RenderCosts(body app.CostsBody, width, height int) string —
// signature frozen by the coordinator's execute dispatch. A thin renderer
// per the CostsBody doc comment ("renderers consume verbatim, never
// recompute — same contract as ListRow.Color"), so every test here builds
// an app.CostsBody literal directly rather than routing through a
// Controller: RenderCosts's contract is "given this body, produce this
// text", independent of how the body was assembled.
//
// "Frame title" (the breadcrumb text shown in the box's top border, e.g.
// "Costs: by service · invoice · monthly · Aug'25-Jul'26*") is NOT
// exercised here: every other screen kind (list/detail/menu) keeps its
// title in a separate FrameTitle()-style accessor rather than baking it
// into the body-render function, and RenderCosts's signature
// (body, width, height) carries no title-construction inputs beyond what
// CostsBody itself exposes — this file only asserts what is verifiably
// RenderCosts's job: the grid body, cursor highlight, pinned TOTAL,
// footer line, and the FR-017 error block.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// baseCostsBody returns a realistic default-view body: 3 monthly columns
// (the last open), 2 rows (EC2 growing, RDS dropping), a pinned TOTAL row —
// mirrors wireframe.md's default grid shape.
func baseCostsBody() app.CostsBody {
	return app.CostsBody{
		Pivot:       "SERVICE",
		Metric:      "invoice",
		Granularity: "month",
		Columns: []app.CostColumn{
			{Label: "May'26", Open: false},
			{Label: "Jun'26", Open: false},
			{Label: "Jul'26", Open: true},
		},
		Rows: []app.CostRow{
			{
				Label: "Amazon EC2",
				Cells: []app.CostCell{
					{Amount: "1,190.0"},
					{Amount: "1,000.0"},
					{Amount: "1,234.5", DeltaTag: "growth"},
				},
			},
			{
				Label: "Amazon RDS",
				Cells: []app.CostCell{
					{Amount: "899.1"},
					{Amount: "800.0"},
					{Amount: "750.0", DeltaTag: "drop"},
				},
			},
		},
		Totals: []app.CostCell{
			{Amount: "2,089.1"},
			{Amount: "1,800.0"},
			{Amount: "1,984.5"},
		},
		CursorRow:   0,
		CursorCol:   2,
		FooterNote:  "Δ vs prev: +23.4%",
		APICalls:    3,
		APICostUSD:  "$0.03",
		DataThrough: "2026-07-09",
	}
}

// ---------------------------------------------------------------------------
// Cursor cell highlight
// ---------------------------------------------------------------------------

func TestRenderCosts_CursorCellIsHighlighted(t *testing.T) {
	atRow0 := baseCostsBody()
	atRow0.CursorRow, atRow0.CursorCol = 0, 2

	atRow1 := baseCostsBody()
	atRow1.CursorRow, atRow1.CursorCol = 1, 2

	out0 := views.RenderCosts(atRow0, 120, 32)
	out1 := views.RenderCosts(atRow1, 120, 32)

	if out0 == out1 {
		t.Fatal("moving the cursor from row 0 to row 1 produced byte-identical output — cursor cell is not visibly highlighted")
	}
	if tuitest.StripANSI(out0) != tuitest.StripANSI(out1) {
		t.Error("moving the cursor changed the VISIBLE text, not just styling — RenderCosts must render the same amounts regardless of cursor position")
	}
}

// ---------------------------------------------------------------------------
// Pinned TOTAL row
// ---------------------------------------------------------------------------

func TestRenderCosts_TotalRowPresent(t *testing.T) {
	body := baseCostsBody()
	out := tuitest.StripANSI(views.RenderCosts(body, 120, 32))

	if !strings.Contains(out, "TOTAL") {
		t.Fatalf("rendered output missing the pinned TOTAL row label:\n%s", out)
	}
	for _, cell := range body.Totals {
		if !strings.Contains(out, cell.Amount) {
			t.Errorf("rendered output missing TOTAL cell amount %q:\n%s", cell.Amount, out)
		}
	}
	// TOTAL is pinned LAST (below every data row), per wireframe.md.
	totalIdx := strings.Index(out, "TOTAL")
	lastRowIdx := strings.LastIndex(out, "Amazon RDS")
	if lastRowIdx == -1 || totalIdx < lastRowIdx {
		t.Errorf("TOTAL row must render after every data row; TOTAL at %d, last data row at %d", totalIdx, lastRowIdx)
	}
}

// ---------------------------------------------------------------------------
// Grid content: row labels + formatted amounts
// ---------------------------------------------------------------------------

func TestRenderCosts_RendersEveryRowLabelAndAmount(t *testing.T) {
	body := baseCostsBody()
	out := tuitest.StripANSI(views.RenderCosts(body, 120, 32))

	for _, row := range body.Rows {
		if !strings.Contains(out, row.Label) {
			t.Errorf("rendered output missing row label %q:\n%s", row.Label, out)
		}
		for _, cell := range row.Cells {
			if !strings.Contains(out, cell.Amount) {
				t.Errorf("rendered output missing cell amount %q for row %q:\n%s", cell.Amount, row.Label, out)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Open-period marker
// ---------------------------------------------------------------------------

func TestRenderCosts_OpenPeriodColumnGetsAsteriskSuffix(t *testing.T) {
	body := baseCostsBody()
	out := tuitest.StripANSI(views.RenderCosts(body, 120, 32))

	if !strings.Contains(out, "Jul'26*") {
		t.Errorf("open (current) period column %q must render with a %q suffix, got:\n%s", "Jul'26", "*", out)
	}
	if strings.Contains(out, "May'26*") || strings.Contains(out, "Jun'26*") {
		t.Errorf("closed period columns must NOT render the open-period asterisk:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Footer: anomaly/delta slot + data-through date (amended FR-013: the "CE
// calls" API counter/cost-estimate element was deleted from the footer
// entirely — this test now also pins its absence).
// ---------------------------------------------------------------------------

func TestRenderCosts_FooterHasDeltaSlotAndDataThrough(t *testing.T) {
	body := baseCostsBody()
	out := tuitest.StripANSI(views.RenderCosts(body, 120, 32))

	if !strings.Contains(out, body.FooterNote) {
		t.Errorf("footer missing FooterNote %q:\n%s", body.FooterNote, out)
	}
	if !strings.Contains(out, body.DataThrough) {
		t.Errorf("footer missing data-through date %q:\n%s", body.DataThrough, out)
	}
	if strings.Contains(out, "CE calls") {
		t.Errorf("footer must not render the removed \"CE calls\" element (amended FR-013):\n%s", out)
	}
	if strings.Contains(out, body.APICostUSD) {
		t.Errorf("footer must not render the removed API cost estimate %q (amended FR-013):\n%s", body.APICostUSD, out)
	}
}

// ---------------------------------------------------------------------------
// Error state (FR-017): centered explicit message block, never an empty grid
// ---------------------------------------------------------------------------

func TestRenderCosts_ErrorState_RendersMessageBlock_NotEmptyGrid(t *testing.T) {
	body := app.CostsBody{
		ErrorMsg: "Cost Explorer unavailable: AccessDenied — profile needs ce:GetCostAndUsage",
	}
	out := tuitest.StripANSI(views.RenderCosts(body, 120, 32))

	if strings.TrimSpace(out) == "" {
		t.Fatal("RenderCosts produced an empty/blank block for an error body — FR-017 forbids an empty grid standing in for the error")
	}
	if !strings.Contains(out, "AccessDenied") {
		t.Errorf("rendered error block missing the classified error text, got:\n%s", out)
	}
	if strings.Contains(out, "TOTAL") {
		t.Errorf("error body must not also render a (necessarily empty) grid with a TOTAL row, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Every metric / granularity combination renders without panicking
// ---------------------------------------------------------------------------

func TestRenderCosts_AllMetricsAndGranularities_RenderWithoutPanic(t *testing.T) {
	metrics := []string{"invoice", "unblended", "amortized", "net-amortized", "blended"}
	granularities := []string{"year", "month", "week", "day"}

	for _, metric := range metrics {
		for _, gran := range granularities {
			body := baseCostsBody()
			body.Metric = metric
			body.Granularity = gran
			out := tuitest.StripANSI(views.RenderCosts(body, 120, 32))
			if strings.TrimSpace(out) == "" {
				t.Errorf("metric=%s granularity=%s: RenderCosts produced empty output", metric, gran)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Terminal width/height edge cases — must not panic
// ---------------------------------------------------------------------------

func TestRenderCosts_TerminalSizeEdgeCases_DoNotPanic(t *testing.T) {
	sizes := []struct{ w, h int }{
		{40, 10},
		{80, 24},
		{200, 50},
		{0, 0},
	}
	for _, sz := range sizes {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("RenderCosts panicked at %dx%d: %v", sz.w, sz.h, r)
				}
			}()
			views.RenderCosts(baseCostsBody(), sz.w, sz.h)
		}()
	}
}
