// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_qa_codex_costs_test.go pins row 50: a closed period whose records
// came from a page-capped fetch is a lower bound. A later complete fetch that
// finds nothing there has to be able to say so, or the lower bound stays on
// disk and the period is re-fetched every time the screen opens.
package unit_test

import (
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/costs"
)

var wipfixCostsClock = time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)

// wipfixClosedPeriod is a month that ended long before the clock, so the
// store treats it as settled rather than still accruing.
var wipfixClosedPeriod = costs.Period{Start: "2025-01-01", End: "2025-02-01"}

func wipfixCostsQuery() costs.Query {
	return costs.Query{
		Granularity: string(costs.GranularityMonth),
		GroupBy:     []costs.Dimension{costs.DimensionService},
		Range:       wipfixClosedPeriod,
	}
}

// TestCostsStore_CompleteZeroGroupRefetchReplacesTruncatedCoverage pins row
// 50. The capped fetch left records CE never finished enumerating; the
// complete refetch says the period has no spend at all. The complete answer
// wins, whether or not it carries records.
func TestCostsStore_CompleteZeroGroupRefetchReplacesTruncatedCoverage(t *testing.T) {
	store := costs.NewMemoryStore("example-readonly")
	q := wipfixCostsQuery()

	// A capped fetch: some records, coverage stamped truncated.
	store.ApplyFetchResult(costs.FetchResult{
		Query: q,
		Records: []costs.Record{{
			Period: wipfixClosedPeriod,
			Keys:   []string{"Amazon Simple Storage Service"},
			Metrics: map[costs.Metric]costs.Amount{
				costs.MetricInvoice: {Value: 100, Unit: "USD"},
			},
		}},
		Coverage:  []costs.Period{wipfixClosedPeriod},
		Truncated: true,
	}, wipfixCostsClock)

	if seeded, _ := store.Lookup(q, []costs.Period{wipfixClosedPeriod}, wipfixCostsClock); len(seeded) == 0 {
		t.Fatal("precondition: the capped fetch left nothing to look up")
	}

	// The complete refetch, a day later: CE reports no groups for the period.
	later := wipfixCostsClock.AddDate(0, 0, 1)
	store.ApplyFetchResult(costs.FetchResult{
		Query:    q,
		Records:  nil,
		Coverage: []costs.Period{wipfixClosedPeriod},
	}, later)

	records, missing := store.Lookup(q, []costs.Period{wipfixClosedPeriod}, later)
	if len(missing) != 0 {
		t.Errorf("the period is still reported missing after a complete fetch covered it — "+
			"the capped bucket was written post-closure, so nothing replaces it and the "+
			"screen re-fetches this closed month every time it opens (missing=%v)", missing)
	}
	if store.Partial(q, []costs.Period{wipfixClosedPeriod}, later) {
		t.Error("the window is still reported partial after a complete fetch covered it — " +
			"a complete answer with no groups is still a complete answer")
	}
	if len(records) > 0 {
		t.Errorf("the period still holds %d record(s) from the capped fetch after a complete "+
			"refetch found none — the lower bound outlives the answer that replaced it",
			len(records))
	}
}

// TestCostsStore_CompleteRefetchWithRecordsStillReplaces is the negative
// half: replacing a truncated period must not depend on the new fetch being
// empty either.
func TestCostsStore_CompleteRefetchWithRecordsStillReplaces(t *testing.T) {
	store := costs.NewMemoryStore("example-readonly")
	q := wipfixCostsQuery()

	store.ApplyFetchResult(costs.FetchResult{
		Query: q,
		Records: []costs.Record{{
			Period: wipfixClosedPeriod, Keys: []string{"Amazon Simple Storage Service"},
			Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 100, Unit: "USD"}},
		}},
		Coverage:  []costs.Period{wipfixClosedPeriod},
		Truncated: true,
	}, wipfixCostsClock)

	later := wipfixCostsClock.AddDate(0, 0, 1)
	store.ApplyFetchResult(costs.FetchResult{
		Query: q,
		Records: []costs.Record{{
			Period: wipfixClosedPeriod, Keys: []string{"Amazon Simple Storage Service"},
			Metrics: map[costs.Metric]costs.Amount{costs.MetricInvoice: {Value: 900, Unit: "USD"}},
		}},
		Coverage: []costs.Period{wipfixClosedPeriod},
	}, later)

	if store.Partial(q, []costs.Period{wipfixClosedPeriod}, later) {
		t.Error("the window is still partial after a complete fetch with records replaced it")
	}
	records, missing := store.Lookup(q, []costs.Period{wipfixClosedPeriod}, later)
	if len(missing) != 0 {
		t.Errorf("the period is still reported missing after a complete refetch with records "+
			"covered it (missing=%v)", missing)
	}
	if len(records) != 1 || records[0].Metrics[costs.MetricInvoice].Value != 900 {
		t.Errorf("the period holds %v, want the complete refetch's 900", records)
	}
}
