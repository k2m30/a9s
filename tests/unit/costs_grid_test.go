package unit_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/costs"
)

const testBlendedMetric = costs.Metric("blended")

func dayPeriod(start, end string) costs.Period { return costs.Period{Start: start, End: end} }

func TestBuildGrid_WeeklyAggregation_SumsSevenDaysPerISOWeek(t *testing.T) {
	// ISO weeks: [2026-06-15, 2026-06-22) and [2026-06-22, 2026-06-29), both
	// Monday-anchored.
	weekA := dayPeriod("2026-06-15", "2026-06-22")
	weekB := dayPeriod("2026-06-22", "2026-06-29")

	dayStarts := []string{
		"2026-06-15", "2026-06-16", "2026-06-17", "2026-06-18", "2026-06-19", "2026-06-20", "2026-06-21",
		"2026-06-22", "2026-06-23", "2026-06-24", "2026-06-25", "2026-06-26", "2026-06-27", "2026-06-28",
	}
	var recs []costs.Record
	for i, start := range dayStarts {
		end := "2026-06-29"
		if i < len(dayStarts)-1 {
			end = dayStarts[i+1]
		}
		recs = append(recs, costs.Record{
			Period: dayPeriod(start, end),
			Keys:   []string{"Amazon Elastic Compute Cloud - Compute"},
			Metrics: map[costs.Metric]costs.Amount{
				testBlendedMetric: {Value: 10, Unit: "USD"},
			},
		})
	}

	grid := costs.BuildGrid(recs, testBlendedMetric, []costs.Period{weekA, weekB})

	if len(grid.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(grid.Rows))
	}
	row := grid.Rows[0]
	if len(row.Cells) != 2 {
		t.Fatalf("len(Cells) = %d, want 2", len(row.Cells))
	}
	if row.Cells[0].Amount.Value != 70 {
		t.Errorf("week A total = %v, want 70 (7 days x 10)", row.Cells[0].Amount.Value)
	}
	if row.Cells[1].Amount.Value != 70 {
		t.Errorf("week B total = %v, want 70 (7 days x 10)", row.Cells[1].Amount.Value)
	}
}

func TestBuildGrid_YearlyAggregation_SumsTwelveMonths(t *testing.T) {
	year := dayPeriod("2025-01-01", "2026-01-01")
	months := []string{
		"2025-01-01", "2025-02-01", "2025-03-01", "2025-04-01", "2025-05-01", "2025-06-01",
		"2025-07-01", "2025-08-01", "2025-09-01", "2025-10-01", "2025-11-01", "2025-12-01",
	}
	monthEnds := []string{
		"2025-02-01", "2025-03-01", "2025-04-01", "2025-05-01", "2025-06-01", "2025-07-01",
		"2025-08-01", "2025-09-01", "2025-10-01", "2025-11-01", "2025-12-01", "2026-01-01",
	}
	var recs []costs.Record
	for i, start := range months {
		recs = append(recs, costs.Record{
			Period: dayPeriod(start, monthEnds[i]),
			Keys:   []string{"Amazon Simple Storage Service"},
			Metrics: map[costs.Metric]costs.Amount{
				testBlendedMetric: {Value: 100, Unit: "USD"},
			},
		})
	}

	grid := costs.BuildGrid(recs, testBlendedMetric, []costs.Period{year})

	if len(grid.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(grid.Rows))
	}
	if got := grid.Rows[0].Cells[0].Amount.Value; got != 1200 {
		t.Errorf("year total = %v, want 1200 (12 months x 100)", got)
	}
}

func TestBuildGrid_Delta_NaNWhenNoBaseOrPrevZero(t *testing.T) {
	monthA := dayPeriod("2026-05-01", "2026-06-01")
	monthB := dayPeriod("2026-06-01", "2026-07-01")

	recs := []costs.Record{
		{Period: monthA, Keys: []string{"Amazon Elastic Compute Cloud - Compute"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 100, Unit: "USD"}}},
		{Period: monthB, Keys: []string{"Amazon Elastic Compute Cloud - Compute"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 150, Unit: "USD"}}},
		{Period: monthA, Keys: []string{"Tax"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 0, Unit: "USD"}}},
		{Period: monthB, Keys: []string{"Tax"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 50, Unit: "USD"}}},
	}

	grid := costs.BuildGrid(recs, testBlendedMetric, []costs.Period{monthA, monthB})

	byKey := make(map[string]costs.GridRow, len(grid.Rows))
	for _, r := range grid.Rows {
		byKey[r.Key] = r
	}

	ec2 := byKey["Amazon Elastic Compute Cloud - Compute"]
	if !math.IsNaN(ec2.Cells[0].Delta) {
		t.Errorf("EC2 col0 (no previous column) Delta = %v, want NaN", ec2.Cells[0].Delta)
	}
	if got := ec2.Cells[1].Delta; math.Abs(got-0.5) > 1e-9 {
		t.Errorf("EC2 col1 Delta = %v, want 0.5 ((150-100)/100)", got)
	}

	tax := byKey["Tax"]
	if !math.IsNaN(tax.Cells[0].Delta) {
		t.Errorf("Tax col0 (no previous column) Delta = %v, want NaN", tax.Cells[0].Delta)
	}
	if !math.IsNaN(tax.Cells[1].Delta) {
		t.Errorf("Tax col1 Delta = %v, want NaN (prev == 0)", tax.Cells[1].Delta)
	}
}

func TestBuildGrid_RowsSortedDescByRowTotal(t *testing.T) {
	month := dayPeriod("2026-06-01", "2026-07-01")
	recs := []costs.Record{
		{Period: month, Keys: []string{"Amazon Simple Storage Service"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 300, Unit: "USD"}}},
		{Period: month, Keys: []string{"Amazon Elastic Compute Cloud - Compute"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 500, Unit: "USD"}}},
		{Period: month, Keys: []string{"EC2 - Other"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 100, Unit: "USD"}}},
	}

	grid := costs.BuildGrid(recs, testBlendedMetric, []costs.Period{month})

	if len(grid.Rows) != 3 {
		t.Fatalf("len(Rows) = %d, want 3", len(grid.Rows))
	}
	wantOrder := []string{"Amazon Elastic Compute Cloud - Compute", "Amazon Simple Storage Service", "EC2 - Other"}
	for i, want := range wantOrder {
		if grid.Rows[i].Key != want {
			t.Errorf("Rows[%d].Key = %q, want %q (order: %v)", i, grid.Rows[i].Key, want, wantOrder)
		}
	}
}

// TestBuildGrid_NoFold_AllNonZeroRows_RenderIndividually_NoOthersRollup: a
// dataset with many small rows renders every row individually — no "… others"
// rollup, regardless of magnitude (spec.md Edge Cases "Many small rows").
func TestBuildGrid_NoFold_AllNonZeroRows_RenderIndividually_NoOthersRollup(t *testing.T) {
	month := dayPeriod("2026-06-01", "2026-07-01")
	var recs []costs.Record
	recs = append(recs,
		costs.Record{Period: month, Keys: []string{"Amazon Elastic Compute Cloud - Compute"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 100000, Unit: "USD"}}},
		costs.Record{Period: month, Keys: []string{"Amazon Simple Storage Service"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 50000, Unit: "USD"}}},
	)
	const tinyRowCount = 20
	for i := 0; i < tinyRowCount; i++ {
		recs = append(recs, costs.Record{
			Period: month,
			Keys:   []string{fmt.Sprintf("USE1-DataTransfer-Regional-Bytes-%02d", i)},
			Metrics: map[costs.Metric]costs.Amount{
				testBlendedMetric: {Value: 1, Unit: "USD"},
			},
		})
	}

	grid := costs.BuildGrid(recs, testBlendedMetric, []costs.Period{month})

	wantRows := 2 + tinyRowCount
	if len(grid.Rows) != wantRows {
		t.Fatalf("len(Rows) = %d, want %d (fold removed — every non-zero row renders individually, no rollup), rows: %+v", len(grid.Rows), wantRows, grid.Rows)
	}
	for _, r := range grid.Rows {
		if strings.Contains(r.Label, "others") {
			t.Errorf("found a rollup row %q — fold was removed, every row must render individually", r.Label)
		}
	}
}

func TestBuildGrid_TotalRow_IndexAlignedWithColumnsAndEqualsColumnSums(t *testing.T) {
	monthA := dayPeriod("2026-05-01", "2026-06-01")
	monthB := dayPeriod("2026-06-01", "2026-07-01")

	recs := []costs.Record{
		{Period: monthA, Keys: []string{"Amazon Elastic Compute Cloud - Compute"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 100, Unit: "USD"}}},
		{Period: monthB, Keys: []string{"Amazon Elastic Compute Cloud - Compute"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 120, Unit: "USD"}}},
		{Period: monthA, Keys: []string{"Amazon Simple Storage Service"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 30, Unit: "USD"}}},
		{Period: monthB, Keys: []string{"Amazon Simple Storage Service"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 45, Unit: "USD"}}},
	}

	grid := costs.BuildGrid(recs, testBlendedMetric, []costs.Period{monthA, monthB})

	if len(grid.Totals) != 2 {
		t.Fatalf("len(Totals) = %d, want 2 (index-aligned with 2 Columns)", len(grid.Totals))
	}
	if got := grid.Totals[0].Amount.Value; got != 130 {
		t.Errorf("Totals[0] = %v, want 130 (100+30)", got)
	}
	if got := grid.Totals[1].Amount.Value; got != 165 {
		t.Errorf("Totals[1] = %v, want 165 (120+45)", got)
	}
}

func TestBuildGrid_NegativeAmounts_FlowIntoTotalsCorrectly(t *testing.T) {
	month := dayPeriod("2026-06-01", "2026-07-01")
	recs := []costs.Record{
		{Period: month, Keys: []string{"Tax"}, Metrics: map[costs.Metric]costs.Amount{costs.Metric("invoice"): {Value: 1000, Unit: "USD"}}},
		{Period: month, Keys: []string{"Credit"}, Metrics: map[costs.Metric]costs.Amount{costs.Metric("invoice"): {Value: -200, Unit: "USD"}}},
	}

	grid := costs.BuildGrid(recs, costs.Metric("invoice"), []costs.Period{month})

	byKey := make(map[string]costs.GridRow, len(grid.Rows))
	for _, r := range grid.Rows {
		byKey[r.Key] = r
	}
	if got := byKey["Credit"].Cells[0].Amount.Value; got != -200 {
		t.Errorf("Credit row amount = %v, want -200 (negative sign preserved)", got)
	}
	if len(grid.Totals) != 1 {
		t.Fatalf("len(Totals) = %d, want 1", len(grid.Totals))
	}
	if got := grid.Totals[0].Amount.Value; got != 800 {
		t.Errorf("Totals[0] = %v, want 800 (1000 + (-200))", got)
	}
}

func TestBuildGrid_Currency(t *testing.T) {
	month := dayPeriod("2026-06-01", "2026-07-01")

	t.Run("uniform currency reports the shared unit", func(t *testing.T) {
		recs := []costs.Record{
			{Period: month, Keys: []string{"Amazon Elastic Compute Cloud - Compute"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 100, Unit: "USD"}}},
			{Period: month, Keys: []string{"Amazon Simple Storage Service"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 30, Unit: "USD"}}},
		}
		grid := costs.BuildGrid(recs, testBlendedMetric, []costs.Period{month})
		if grid.Currency != "USD" {
			t.Errorf("Currency = %q, want %q", grid.Currency, "USD")
		}
	})

	t.Run("mixed currency suppresses TOTAL via empty Currency", func(t *testing.T) {
		recs := []costs.Record{
			{Period: month, Keys: []string{"Amazon Elastic Compute Cloud - Compute"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 100, Unit: "USD"}}},
			{Period: month, Keys: []string{"AWS Support (Business)"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 80, Unit: "EUR"}}},
		}
		grid := costs.BuildGrid(recs, testBlendedMetric, []costs.Period{month})
		if grid.Currency != "" {
			t.Errorf("Currency = %q, want \"\" (mixed-currency grid)", grid.Currency)
		}
	})
}

func TestBuildGrid_EstimatedFlag_PropagatesFromRecordToCell(t *testing.T) {
	month := dayPeriod("2026-07-01", "2026-08-01")

	t.Run("estimated record marks the cell estimated", func(t *testing.T) {
		recs := []costs.Record{
			{Period: month, Keys: []string{"Amazon Elastic Compute Cloud - Compute"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 500, Unit: "USD"}}, Estimated: true},
		}
		grid := costs.BuildGrid(recs, testBlendedMetric, []costs.Period{month})
		if !grid.Rows[0].Cells[0].Estimated {
			t.Error("Cells[0].Estimated = false, want true")
		}
	})

	t.Run("non-estimated record leaves the cell unmarked", func(t *testing.T) {
		recs := []costs.Record{
			{Period: month, Keys: []string{"Amazon Elastic Compute Cloud - Compute"}, Metrics: map[costs.Metric]costs.Amount{testBlendedMetric: {Value: 500, Unit: "USD"}}, Estimated: false},
		}
		grid := costs.BuildGrid(recs, testBlendedMetric, []costs.Period{month})
		if grid.Rows[0].Cells[0].Estimated {
			t.Error("Cells[0].Estimated = true, want false")
		}
	})
}
