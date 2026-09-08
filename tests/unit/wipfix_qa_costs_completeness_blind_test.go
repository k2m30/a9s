// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_qa_costs_completeness_blind_test.go reproduces the reviewer's two
// cost-cache scenarios literally, from the screen the operator uses:
//
//   - a grid whose CE walk hit its page cap is partial. Leaving the screen and
//     coming back must not turn it into a complete answer, and a later
//     complete fetch must be able to replace the numbers it left behind.
//   - an anomaly walk cut short still found real anomalies. They must render,
//     with the screen saying the overlay is incomplete.
package unit

import (
	"strings"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

var wipfixCostsNow = time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

// wipfixOpenCosts pushes the costs screen and returns the fetch the screen
// issues for its own frame — the request every CostsLoaded below answers.
func wipfixOpenCosts(t *testing.T, c *app.Controller) runtime.FetchCostsPayload {
	t.Helper()
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenCosts}})
	c.EnsureCostsState(wipfixCostsNow)
	tasks := c.EnsureCostsFetch()
	if len(tasks) != 1 {
		t.Fatalf("the costs screen issued %d fetch task(s), want 1", len(tasks))
	}
	p, ok := tasks[0].Payload.(runtime.FetchCostsPayload)
	if !ok {
		t.Fatalf("costs fetch payload type = %T, want runtime.FetchCostsPayload", tasks[0].Payload)
	}
	if len(p.Window) == 0 {
		t.Fatal("the costs fetch carries no window")
	}
	return p
}

// wipfixCostRecords bills one service the same amount in every period of the
// window, so every grid cell has a value to lose.
func wipfixCostRecords(window []costs.Period, amount float64) []costs.Record {
	out := make([]costs.Record, 0, len(window))
	for _, p := range window {
		out = append(out, costs.Record{
			Period: p,
			Keys:   []string{"Amazon Simple Storage Service"},
			Metrics: map[costs.Metric]costs.Amount{
				costs.MetricInvoice:      {Value: amount, Unit: "USD"},
				costs.MetricUnblended:    {Value: amount, Unit: "USD"},
				costs.MetricAmortized:    {Value: amount, Unit: "USD"},
				costs.MetricNetAmortized: {Value: amount, Unit: "USD"},
				costs.MetricBlended:      {Value: amount, Unit: "USD"},
			},
		})
	}
	return out
}

// wipfixCostsScreenText is every string the costs screen renders that could
// carry a completeness warning.
func wipfixCostsScreenText(t *testing.T, c *app.Controller) string {
	t.Helper()
	body := c.Snapshot().Body.Costs
	if body == nil {
		t.Fatal("no costs body on the snapshot")
	}
	return body.FooterNote + "\n" + body.ErrorMsg + "\n" + strings.Join(body.Breadcrumb, " ")
}

func wipfixSaysPartial(s string) bool {
	l := strings.ToLower(s)
	return strings.Contains(l, "partial") || strings.Contains(l, "incomplete") ||
		strings.Contains(l, "lower bound") || strings.Contains(l, "truncated")
}

// TestCostsPageCappedGrid_StillWarnsAfterLeavingAndReopening pins row 1's
// first half: the truncation belongs to the data, not to the frame that
// happened to be on screen when it arrived. Recording it on the frame alone
// means walking away and back turns a page-capped answer into a confident one.
func TestCostsPageCappedGrid_StillWarnsAfterLeavingAndReopening(t *testing.T) {
	c := newTestController(t)
	p := wipfixOpenCosts(t, c)

	_, _ = c.Handle(messages.CostsLoaded{
		Query:  p.Query,
		Window: p.Window,
		Grid: costs.GridResult{
			Fetched:   true,
			Records:   wipfixCostRecords(p.Window, 100),
			Truncated: true,
		},
	})

	if got := wipfixCostsScreenText(t, c); !wipfixSaysPartial(got) {
		t.Errorf("the costs screen says nothing about the page cap right after the fetch: %q", got)
	}

	amount := wipfixFirstCostCellText(t, c)

	// Leave the screen and come back — the same operator, the same window.
	_, _ = c.Apply(app.Action{Kind: app.ActionBack})
	wipfixOpenCosts(t, c)

	// The numbers themselves survive the round trip, so anything lost here was
	// lost because it was kept somewhere the reopen does not read.
	if got := wipfixFirstCostCellText(t, c); got != amount {
		t.Fatalf("precondition: the reopened screen shows %q, not the %q it was left with — "+
			"this scenario cannot say anything about the warning", got, amount)
	}
	if got := wipfixCostsScreenText(t, c); !wipfixSaysPartial(got) {
		t.Errorf("after reopening the costs screen the page-cap warning is gone: %q — "+
			"completeness must travel with the data, not with the frame that was on screen", got)
	}
}

// TestCostsPageCappedGrid_IsRepairedByACompleteRefresh pins row 1's second
// half: a settled period written from a page-capped fetch must never be
// promoted to immutable, or the refresh that could repair it is refused.
func TestCostsPageCappedGrid_IsRepairedByACompleteRefresh(t *testing.T) {
	c := newTestController(t)
	p := wipfixOpenCosts(t, c)

	_, _ = c.Handle(messages.CostsLoaded{
		Query:  p.Query,
		Window: p.Window,
		Grid: costs.GridResult{
			Fetched:   true,
			Records:   wipfixCostRecords(p.Window, 100),
			Truncated: true,
		},
	})
	partial := wipfixFirstCostCellText(t, c)

	// The repair: the same window, walked to the end this time.
	for _, task := range c.ForceRefreshCosts() {
		rp, ok := task.Payload.(runtime.FetchCostsPayload)
		if !ok {
			continue
		}
		_, _ = c.Handle(messages.CostsLoaded{
			Query:  rp.Query,
			Window: rp.Window,
			Grid: costs.GridResult{
				Fetched: true,
				Records: wipfixCostRecords(rp.Window, 900),
			},
		})
	}

	repaired := wipfixFirstCostCellText(t, c)
	if repaired == partial {
		t.Errorf("the first cell still reads %q after a complete refetch replaced the "+
			"page-capped numbers — a bucket written from a truncated fetch was promoted "+
			"to immutable, so nothing can repair it", repaired)
	}
	if got := wipfixCostsScreenText(t, c); wipfixSaysPartial(got) {
		t.Errorf("the screen still calls the window partial after a complete fetch replaced it: %q", got)
	}
}

// wipfixFirstCostCellText returns the first grid cell's rendered amount.
func wipfixFirstCostCellText(t *testing.T, c *app.Controller) string {
	t.Helper()
	body := c.Snapshot().Body.Costs
	if body == nil || len(body.Rows) == 0 || len(body.Rows[0].Cells) == 0 {
		t.Fatalf("the costs grid rendered no cells (body=%v)", body)
	}
	return body.Rows[0].Cells[0].Amount
}

// TestCostsPageCappedAnomalyWalk_MarksStillRender pins row 7: an anomaly the
// walk did find is a confirmed anomaly. Dropping every mark because the walk
// was cut short hides a real cost spike and says nothing about having done so.
func TestCostsPageCappedAnomalyWalk_MarksStillRender(t *testing.T) {
	c := newTestController(t)
	p := wipfixOpenCosts(t, c)

	flagged := p.Window[0]
	_, _ = c.Handle(messages.CostsLoaded{
		Query:  p.Query,
		Window: p.Window,
		Grid: costs.GridResult{
			Fetched: true,
			Records: wipfixCostRecords(p.Window, 100),
		},
		Anomalies: []costs.AnomalyMark{{
			ID:        "example-anomaly-1",
			Score:     0.9,
			Impact:    costs.Amount{Value: 420, Unit: "USD"},
			Period:    flagged,
			RootCause: "Amazon Simple Storage Service",
			Dimension: map[costs.Dimension]string{costs.DimensionService: "Amazon Simple Storage Service"},
		}},
		AnomaliesTruncated: true,
	})

	body := c.Snapshot().Body.Costs
	if body == nil {
		t.Fatal("no costs body on the snapshot")
	}
	marked := 0
	for _, row := range body.Rows {
		for _, cell := range row.Cells {
			if cell.Anomaly {
				marked++
			}
		}
	}
	if marked == 0 {
		t.Errorf("no grid cell carries the anomaly overlay after a page-capped anomaly walk — "+
			"the anomaly it did find is confirmed and must not vanish with the walk (rows=%d)",
			len(body.Rows))
	}
	if got := wipfixCostsScreenText(t, c); !wipfixSaysPartial(got) {
		t.Errorf("the anomaly overlay renders with no warning that it is a lower bound: %q", got)
	}
}

// TestCostsCompleteGrid_NeverWarns is the negative half for both rows: a walk
// that finished must not be described as partial.
func TestCostsCompleteGrid_NeverWarns(t *testing.T) {
	c := newTestController(t)
	p := wipfixOpenCosts(t, c)

	_, _ = c.Handle(messages.CostsLoaded{
		Query:  p.Query,
		Window: p.Window,
		Grid:   costs.GridResult{Fetched: true, Records: wipfixCostRecords(p.Window, 100)},
	})
	if got := wipfixCostsScreenText(t, c); wipfixSaysPartial(got) {
		t.Errorf("a complete costs fetch is described as partial: %q", got)
	}
}
