// costs_demo_daily_test.go — Cost Explorer week/day zoom over the demo
// transport: the demo CE handler (core/demo/costs_handlers.go) honours the
// request's Granularity, so a DAILY GetCostAndUsage request returns one
// bucket per day, never month-sized buckets (or nothing, once TimePeriod
// narrows to a sub-month range).
//
// Reuses newDemoCostsClient from costs_demo_test.go for the real
// *costexplorer.Client-over-demo-transport construction; queries here build
// their own costs.Query literals rather than demoCostsQuery, since that
// helper's wide "cover the whole 13-month history" Range is the wrong shape
// for the narrow month/week ranges these tests need.
//
// core/demo/fixtures/costs.go's month generation is relative to
// fixtures.CostsAnchorMonth (computed from "now" at process start), so every
// month literal below is derived from that exported anchor (and from
// fixtures.CostsGrowthMonth for the growth-story-specific case) via
// costsDemoDailyMonth.
package unit_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	a9saws "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/costs"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// dailyRecordKey identifies one (group-key-tuple, period) record for
// cross-fetch comparison.
func dailyRecordKey(rec costs.Record) string {
	return strings.Join(rec.Keys, "/") + "@" + rec.Period.Start + "/" + rec.Period.End
}

// costsDemoDailyMonth returns the first day ("YYYY-MM-01") of the month
// monthsBeforeAnchor months before fixtures.CostsAnchorMonth — a closed
// month relative to the evergreen anchor, never a stale absolute literal.
func costsDemoDailyMonth(t *testing.T, monthsBeforeAnchor int) time.Time {
	t.Helper()
	anchor, err := time.Parse("2006-01-02", fixtures.CostsAnchorMonth)
	if err != nil {
		t.Fatalf("parsing fixtures.CostsAnchorMonth %q: %v", fixtures.CostsAnchorMonth, err)
	}
	return anchor.AddDate(0, -monthsBeforeAnchor, 0)
}

// ---------------------------------------------------------------------------
// DAILY SERVICE-grouped fetch over one closed month: one bucket per day,
// each service's daily amounts summing to its monthly fixture amount.
// ---------------------------------------------------------------------------

func TestCostsDemoDaily_ServiceGrouped_DailyBucketsSumToMonthlyFixtureAmount(t *testing.T) {
	client := newDemoCostsClient()
	monthTime := costsDemoDailyMonth(t, 4) // closed month, well before the anchor
	nextMonthTime := costsDemoDailyMonth(t, 3)
	month := monthTime.Format("2006-01-02")
	nextMonth := nextMonthTime.Format("2006-01-02")
	wantDays := int(nextMonthTime.Sub(monthTime).Hours() / 24)

	monthlyResult, err := a9saws.FetchCostAndUsage(context.Background(), client, costs.Query{
		Granularity: costs.GranularityMonth.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionService},
		Range:       costs.Period{Start: month, End: nextMonth},
	})
	if err != nil {
		t.Fatalf("FetchCostAndUsage(MONTHLY, %s): %v", month, err)
	}
	monthlyAmount := make(map[string]float64)
	for _, rec := range monthlyResult.Records {
		if rec.Period.Start != month {
			continue
		}
		monthlyAmount[strings.Join(rec.Keys, "/")] = rec.Metrics[costs.MetricInvoice].Value
	}
	if len(monthlyAmount) == 0 {
		t.Fatalf("monthly baseline fetch for %s returned no records", month)
	}

	dailyResult, err := a9saws.FetchCostAndUsage(context.Background(), client, costs.Query{
		Granularity: costs.GranularityDay.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionService},
		Range:       costs.Period{Start: month, End: nextMonth},
	})
	if err != nil {
		t.Fatalf("FetchCostAndUsage(DAILY, %s): %v", month, err)
	}
	if len(dailyResult.Records) == 0 {
		t.Fatal("DAILY fetch returned no records — the demo CE handler must honor Granularity, not just serve monthly buckets")
	}

	dailySum := make(map[string]float64)
	dayPeriods := make(map[string]struct{})
	for _, rec := range dailyResult.Records {
		key := strings.Join(rec.Keys, "/")
		dailySum[key] += rec.Metrics[costs.MetricInvoice].Value
		dayPeriods[rec.Period.Start+"/"+rec.Period.End] = struct{}{}
	}

	if got := len(dayPeriods); got != wantDays {
		t.Errorf("distinct DAILY periods in %s: got %d want %d — one bucket per day, not one monthly bucket", month, got, wantDays)
	}

	const tolerance = 0.01
	for key, monthlyVal := range monthlyAmount {
		gotSum, ok := dailySum[key]
		if !ok {
			t.Errorf("service %q present in the monthly fetch has no DAILY records at all", key)
			continue
		}
		diff := gotSum - monthlyVal
		if diff > tolerance || diff < -tolerance {
			t.Errorf("service %q: daily amounts sum to %.4f, monthly fixture amount is %.4f (diff %.4f exceeds tolerance %.2f)",
				key, gotSum, monthlyVal, diff, tolerance)
		}
	}
}

// ---------------------------------------------------------------------------
// DAILY USAGE_TYPE-grouped fetch, growth service, partial mid-month range:
// returns exactly those days.
// ---------------------------------------------------------------------------

func TestCostsDemoDaily_UsageTypeGrouped_PartialRangeReturnsExactlyThoseDays(t *testing.T) {
	client := newDemoCostsClient()

	// growthMonth is fixtures.CostsGrowthMonth (the planted growth-story
	// month, fixtures.CostsGrowthService's own single monthly row) — 7 days mid-month
	// (day 10-16 inclusive), a range that starts and ends well inside that
	// row, so a handler that only compares TimePeriod against row.Month
	// cannot serve it correctly (it would exclude the whole month or
	// include it whole).
	growthMonth, err := time.Parse("2006-01-02", fixtures.CostsGrowthMonth)
	if err != nil {
		t.Fatalf("parsing fixtures.CostsGrowthMonth %q: %v", fixtures.CostsGrowthMonth, err)
	}
	rangeStart := growthMonth.AddDate(0, 0, 9) // day 10
	rangeEnd := growthMonth.AddDate(0, 0, 16)  // day 17 (exclusive)
	result, err := a9saws.FetchCostAndUsage(context.Background(), client, costs.Query{
		Granularity: costs.GranularityDay.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionUsageType},
		Filter:      costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionService: {fixtures.CostsGrowthService}}},
		Range:       costs.Period{Start: rangeStart.Format("2006-01-02"), End: rangeEnd.Format("2006-01-02")},
	})
	if err != nil {
		t.Fatalf("FetchCostAndUsage(DAILY, USAGE_TYPE, partial range): %v", err)
	}
	if len(result.Records) == 0 {
		t.Fatal("DAILY partial-range fetch returned no records")
	}

	wantDays := make(map[string]bool, 7)
	for d := rangeStart; d.Before(rangeEnd); d = d.AddDate(0, 0, 1) {
		wantDays[fmt.Sprintf("%s/%s", d.Format("2006-01-02"), d.AddDate(0, 0, 1).Format("2006-01-02"))] = true
	}

	gotDays := make(map[string]bool)
	for _, rec := range result.Records {
		key := rec.Period.Start + "/" + rec.Period.End
		gotDays[key] = true
		if !wantDays[key] {
			t.Errorf("DAILY partial-range fetch returned a period outside the requested range: %s", key)
		}
	}
	if len(gotDays) != len(wantDays) {
		t.Errorf("DAILY partial-range fetch: got %d distinct day periods, want exactly %d — got=%v want=%v", len(gotDays), len(wantDays), gotDays, wantDays)
	}
}

// ---------------------------------------------------------------------------
// Determinism: two identical DAILY fetches return identical records.
// ---------------------------------------------------------------------------

func TestCostsDemoDaily_TwoIdenticalFetches_ReturnIdenticalRecords(t *testing.T) {
	client := newDemoCostsClient()
	monthTime := costsDemoDailyMonth(t, 4)
	nextMonthTime := costsDemoDailyMonth(t, 3)
	q := costs.Query{
		Granularity: costs.GranularityDay.APIGranularity(),
		GroupBy:     []costs.Dimension{costs.DimensionService},
		Range:       costs.Period{Start: monthTime.Format("2006-01-02"), End: nextMonthTime.Format("2006-01-02")},
	}

	first, err := a9saws.FetchCostAndUsage(context.Background(), client, q)
	if err != nil {
		t.Fatalf("first FetchCostAndUsage(DAILY): %v", err)
	}
	second, err := a9saws.FetchCostAndUsage(context.Background(), client, q)
	if err != nil {
		t.Fatalf("second FetchCostAndUsage(DAILY): %v", err)
	}

	if len(first.Records) == 0 {
		t.Fatal("first DAILY fetch returned no records")
	}
	if len(first.Records) != len(second.Records) {
		t.Fatalf("record count differs between two identical DAILY fetches: %d vs %d — the demo day-level split must be deterministic, no randomness in the spread",
			len(first.Records), len(second.Records))
	}

	firstByKey := make(map[string]costs.Record, len(first.Records))
	for _, rec := range first.Records {
		firstByKey[dailyRecordKey(rec)] = rec
	}
	for _, rec := range second.Records {
		key := dailyRecordKey(rec)
		other, ok := firstByKey[key]
		if !ok {
			t.Errorf("second fetch has a record not present in the first: %+v", rec)
			continue
		}
		gotVal := rec.Metrics[costs.MetricInvoice].Value
		wantVal := other.Metrics[costs.MetricInvoice].Value
		if gotVal != wantVal {
			t.Errorf("record %s: invoice amount differs between two identical fetches: %v vs %v — no randomness allowed in the day-level spread",
				key, wantVal, gotVal)
		}
	}
}
