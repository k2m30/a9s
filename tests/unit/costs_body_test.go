// costs_body_test.go — Cost Explorer Phase 2: Snapshot() -> CostsBody
// (specs/021-cost-explorer/data-model.md §ViewState, wireframe.md).
//
// CostColumn/CostRow/CostCell are referenced by data-model.md's CostsBody
// struct ("Columns []CostColumn // label + open marker", "Rows []CostRow //
// label + cells (pre-formatted amount, delta tag, anomaly flag)") but their
// own field shapes are not spelled out — data-model.md leaves them to the
// implementation. This file fixes that contract (TDD: tests define it where
// the architect doc doesn't) as:
//
//	type CostColumn struct { Label string; Open bool }
//	type CostRow    struct { Label string; Cells []CostCell }
//	type CostCell   struct { Amount string; DeltaTag string; Anomaly bool; Estimated bool; Negative bool }
//
// Amount format (comma-grouped thousands, exactly one decimal place, no
// currency symbol) is read directly off wireframe.md's grid ("1,204.1",
// "2,971.3", "714.8" — every value shown uses this exact shape). DeltaTag
// is this file's own invented three-way bucket name ("growth"/"drop"/
// "neutral"/"" for no baseline), matching wireframe.md's "cells colored by
// delta bucket: growth red shades, drop green shades, |Δ| < threshold
// neutral" rule; every seeded delta here is far outside any plausible
// neutral threshold so the growth/drop assertions hold regardless of the
// exact neutral threshold in production.
package unit_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// fmtCostAmount reproduces the exact display format wireframe.md uses for
// every grid amount: comma-grouped thousands, one decimal place, no symbol.
func fmtCostAmount(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := strconv.FormatFloat(v, 'f', 1, 64)
	intPart, decPart, _ := strings.Cut(s, ".")
	var grouped strings.Builder
	n := len(intPart)
	for i, r := range intPart {
		if i > 0 && (n-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(r)
	}
	out := grouped.String() + "." + decPart
	if neg {
		out = "-" + out
	}
	return out
}

// findCostRow returns the CostRow with the given label, failing the test if
// absent.
func findCostRow(t *testing.T, rows []app.CostRow, label string) app.CostRow {
	t.Helper()
	for _, r := range rows {
		if r.Label == label {
			return r
		}
	}
	t.Fatalf("no CostRow with Label %q among %d rows", label, len(rows))
	return app.CostRow{}
}

// seedTwoMonthGrid loads a two-column (prior closed month + current open
// month), two-row (EC2 growing, RDS dropping) SERVICE grid via the real
// production event path. Both periods use fixedCostsNow's own month and the
// month before it, guaranteeing they land inside whatever window
// EnsureCostsState computed (see costs_state_test.go's file header for why
// this sidesteps needing to reproduce that window math).
func seedTwoMonthGrid(t *testing.T, c *app.Controller, now time.Time) {
	t.Helper()
	curStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	curEnd := curStart.AddDate(0, 1, 0)
	prevStart := curStart.AddDate(0, -1, 0)
	prevEnd := curStart

	rec := func(rowKey string, period costs.Period, amount float64) costs.Record {
		return costs.Record{
			Period:  period,
			Keys:    []string{rowKey},
			Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: amount, Unit: "USD"}},
		}
	}
	curPeriod := costs.Period{Start: curStart.Format("2006-01-02"), End: curEnd.Format("2006-01-02")}
	prevPeriod := costs.Period{Start: prevStart.Format("2006-01-02"), End: prevEnd.Format("2006-01-02")}

	c.Handle(messages.CostsLoaded{
		Query: costs.Query{
			Granularity: costs.GranularityMonth.APIGranularity(),
			GroupBy:     []costs.Dimension{costs.DimensionService},
		},
		Grid: costs.GridResult{Fetched: true, Records: []costs.Record{
			rec("Amazon EC2", prevPeriod, 1000.0),
			rec("Amazon EC2", curPeriod, 1234.5), // +23.45% -> growth
			rec("Amazon RDS", prevPeriod, 800.0),
			rec("Amazon RDS", curPeriod, 750.0), // -6.25% -> drop
		}},
		Requests: 1,
	})
}

// ---------------------------------------------------------------------------
// Kind, pre-resolved cells, open-period marker, TOTAL row alignment
// ---------------------------------------------------------------------------

func TestCostsBody_Snapshot_KindCostsWithPreResolvedCells(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	seedTwoMonthGrid(t, c, fixedCostsNow)

	body := c.Snapshot().Body
	if body.Kind != app.BodyKindCosts {
		t.Fatalf("Body.Kind: got %q want %q", body.Kind, app.BodyKindCosts)
	}
	if body.Costs == nil {
		t.Fatal("Body.Costs is nil")
	}
	cb := body.Costs

	if len(cb.Columns) < 2 {
		t.Fatalf("CostsBody.Columns: got %d want >= 2", len(cb.Columns))
	}
	lastIdx := len(cb.Columns) - 1
	prevIdx := lastIdx - 1

	// Row Label is the vendor-prefix-stripped display name
	// (stripCostsServiceVendorPrefix, costs_body.go) — "Amazon EC2" ->
	// "EC2", "Amazon RDS" -> "RDS". seedTwoMonthGrid's rowKey arguments
	// stay the raw CE service name (the Filter/Key value, unaffected by
	// display stripping — see the Breadcrumb test below, which still
	// expects the raw name).
	ec2 := findCostRow(t, cb.Rows, "EC2")
	rds := findCostRow(t, cb.Rows, "RDS")

	if got, want := ec2.Cells[lastIdx].Amount, fmtCostAmount(1234.5); got != want {
		t.Errorf("EC2 current-month Amount: got %q want %q", got, want)
	}
	// costs_quality_test.go, item 1: DeltaTag gained a 4-tier scale
	// (neutral / soft / strong per direction) — +23.45% falls in the
	// soft-growth band ([5%,25%)).
	if got := ec2.Cells[lastIdx].DeltaTag; got != "growth-soft" {
		t.Errorf("EC2 current-month DeltaTag: got %q want %q (+23.45%% vs prior month)", got, "growth-soft")
	}
	if got, want := ec2.Cells[prevIdx].Amount, fmtCostAmount(1000.0); got != want {
		t.Errorf("EC2 prior-month Amount: got %q want %q", got, want)
	}

	if got, want := rds.Cells[lastIdx].Amount, fmtCostAmount(750.0); got != want {
		t.Errorf("RDS current-month Amount: got %q want %q", got, want)
	}
	// -6.25% falls in the soft-drop band ([5%,25%)) under item 1's 4-tier scale.
	if got := rds.Cells[lastIdx].DeltaTag; got != "drop-soft" {
		t.Errorf("RDS current-month DeltaTag: got %q want %q (-6.25%% vs prior month)", got, "drop-soft")
	}
}

func TestCostsBody_Snapshot_OpenPeriodMarker(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	seedTwoMonthGrid(t, c, fixedCostsNow)

	cb := c.Snapshot().Body.Costs
	if len(cb.Columns) == 0 {
		t.Fatal("CostsBody.Columns is empty")
	}
	lastIdx := len(cb.Columns) - 1
	for i, col := range cb.Columns {
		want := i == lastIdx
		if col.Open != want {
			t.Errorf("Columns[%d].Open: got %v want %v (only the last/current column is open)", i, col.Open, want)
		}
	}
}

func TestCostsBody_Snapshot_TotalRowAlignment(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	seedTwoMonthGrid(t, c, fixedCostsNow)

	cb := c.Snapshot().Body.Costs
	if len(cb.Totals) != len(cb.Columns) {
		t.Fatalf("len(Totals)=%d must equal len(Columns)=%d (index-aligned pinned TOTAL row)", len(cb.Totals), len(cb.Columns))
	}
	lastIdx := len(cb.Columns) - 1
	prevIdx := lastIdx - 1

	if got, want := cb.Totals[lastIdx].Amount, fmtCostAmount(1234.5+750.0); got != want {
		t.Errorf("TOTAL current-month Amount: got %q want %q", got, want)
	}
	if got, want := cb.Totals[prevIdx].Amount, fmtCostAmount(1000.0+800.0); got != want {
		t.Errorf("TOTAL prior-month Amount: got %q want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// Breadcrumb
// ---------------------------------------------------------------------------

func TestCostsBody_Snapshot_Breadcrumb_EmptyAtRoot_PopulatedAfterDrill(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)
	seedTwoMonthGrid(t, c, fixedCostsNow)

	if bc := c.Snapshot().Body.Costs.Breadcrumb; len(bc) != 0 {
		t.Errorf("root Breadcrumb: got %v want empty (no drill has happened yet)", bc)
	}

	// Cursor defaults to row 0 == "Amazon EC2" (rows sorted desc by total;
	// EC2's 1000+1234.5 total exceeds RDS's 800+750).
	c.Apply(app.Action{Kind: app.ActionSelect})

	// buildCostsBreadcrumb strips the "Amazon "/"AWS " vendor prefix off a
	// SERVICE-dimension segment (stripCostsServiceVendorPrefix,
	// costs_body.go), matching the wireframe's own breadcrumb example
	// ("Costs: EC2 - Compute ▸ ...", specs/021-cost-explorer/wireframe.md) —
	// so the drilled segment is "EC2", not the raw filter value "Amazon EC2".
	bc := c.Snapshot().Body.Costs.Breadcrumb
	found := false
	for _, seg := range bc {
		if strings.Contains(seg, "EC2") {
			found = true
		}
	}
	if !found {
		t.Errorf("Breadcrumb after drilling into the EC2 row: got %v, want a segment naming %q", bc, "EC2")
	}
}

// ---------------------------------------------------------------------------
// APICalls counter — session-scoped, accumulates, survives errors (FR-013)
// ---------------------------------------------------------------------------

func TestCostsBody_Snapshot_APICallsCounter_AccumulatesAcrossFetches(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)

	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}
	c.Handle(messages.CostsLoaded{Grid: costs.GridResult{Fetched: true}, Query: q, Requests: 3})
	if got := c.Snapshot().Body.Costs.APICalls; got != 3 {
		t.Fatalf("APICalls after first fetch (Requests=3): got %d want 3", got)
	}

	c.Handle(messages.CostsLoaded{Grid: costs.GridResult{Fetched: true}, Query: q, Requests: 2})
	if got := c.Snapshot().Body.Costs.APICalls; got != 5 {
		t.Fatalf("APICalls after second fetch (+2): got %d want 5 (session counter accumulates)", got)
	}

	// FR-013: the counter increments by Requests even on a failed fetch.
	c.Handle(messages.CostsLoaded{
		Grid: costs.GridResult{Fetched: true}, Query: q,
		Requests: 1,
		Err:      fmt.Errorf("%w: throttled", awsclient.ErrCostsThrottled),
	})
	if got := c.Snapshot().Body.Costs.APICalls; got != 6 {
		t.Errorf("APICalls after a failed fetch (+1): got %d want 6 (FR-013: counts even on error)", got)
	}
}

// ---------------------------------------------------------------------------
// Explicit error body — FR-017: never an empty grid
// ---------------------------------------------------------------------------

func TestCostsBody_Snapshot_ErrorMsg_NeverEmptyGrid(t *testing.T) {
	c := newCostsController(t, fixedCostsNow)

	q := costs.Query{Granularity: costs.GranularityMonth.APIGranularity(), GroupBy: []costs.Dimension{costs.DimensionService}}
	c.Handle(messages.CostsLoaded{
		Grid: costs.GridResult{Fetched: true}, Query: q,
		Requests: 1,
		Err:      fmt.Errorf("%w: profile needs ce:GetCostAndUsage", awsclient.ErrCostsAccessDenied),
	})

	cb := c.Snapshot().Body.Costs
	if cb == nil {
		t.Fatal("Body.Costs is nil after a failed fetch — FR-017 requires an explicit error body, not a missing one")
	}
	if cb.ErrorMsg == "" {
		t.Fatal("CostsBody.ErrorMsg is empty after a failed fetch — FR-017: an empty grid must never masquerade as zero spend")
	}
	if !strings.Contains(strings.ToLower(cb.ErrorMsg), "access denied") {
		t.Errorf("CostsBody.ErrorMsg: got %q, want it to name the AccessDenied failure", cb.ErrorMsg)
	}
}
