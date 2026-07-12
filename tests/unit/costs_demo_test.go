// costs_demo_test.go — Cost Explorer Phase 2: demo transport serves CE data
// (specs/021-cost-explorer/spec.md SC-001, FR-003).
//
// Phase-1 production code (already landed, used as-is): internal/costs,
// internal/aws.FetchCostAndUsage/FetchCostAnomalies, and
// internal/demo.NewDemoAWSConfig (routes any *costexplorer.Client through
// the demo transport, exactly like tests/unit/demo_app_test.go's pattern
// for other services via internal/aws.CreateServiceClients).
//
// Phase-2, not yet landed (RED until the coder registers "ce:GetCostAndUsage"
// / "ce:GetAnomalies" handlers backed by internal/demo/fixtures/costs.go):
// every assertion below currently fails with a 501 "no handler for ce:*"
// transport error, not a compile error — this file compiles clean today
// against Phase-1 symbols alone.
//
// The growth-story numbers asserted here (service/usage-type pinned via
// fixtures.CostsGrowthService/CostsGrowthUsageType, a sharp single-month
// jump) are lifted verbatim from wireframe.md's default-view example row,
// which is explicitly the coordinator-cited source for the planted story —
// so this is not a guess at fixture values, it is the documented visual
// truth the fixture is built to match. The exact jump magnitude is asserted
// as a ratio threshold ("roughly doubling", per the coordinator's own
// wording), not an exact float, since the fixture's precise numbers are the
// coder's to choose. Pinned via the constants, not literals (Codex X1): the
// story must re-plant under a service with a registered a9s detail-view
// mapping (CostsResourceRowsByService) so SC-001's spike -> usage type ->
// resource chain can actually resolve a resource; whichever service the
// coder re-plants it under, this file needs no edit.
package unit_test

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/costexplorer"

	a9saws "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/costs"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// demoCostsQuery builds a wide-enough Range to comfortably contain whatever
// calendar window the demo fixture covers, grouped by dims and optionally
// filtered.
func demoCostsQuery(filter costs.Filter, dims ...costs.Dimension) costs.Query {
	return costs.Query{
		Granularity: costs.GranularityMonth.APIGranularity(),
		GroupBy:     dims,
		Filter:      filter,
		Range:       costs.Period{Start: "2024-01-01", End: "2027-01-01"},
	}
}

func newDemoCostsClient() *costexplorer.Client {
	return costexplorer.NewFromConfig(demo.NewDemoAWSConfig())
}

// ---------------------------------------------------------------------------
// 13 months of monthly SERVICE-grouped data
// ---------------------------------------------------------------------------

func TestCostsDemo_FetchCostAndUsage_ServiceGrouped_ThirteenMonths(t *testing.T) {
	client := newDemoCostsClient()
	q := demoCostsQuery(costs.Filter{}, costs.DimensionService)

	result, err := a9saws.FetchCostAndUsage(context.Background(), client, q)
	if err != nil {
		t.Fatalf("FetchCostAndUsage(SERVICE): %v", err)
	}
	if len(result.Records) == 0 {
		t.Fatal("FetchCostAndUsage(SERVICE) returned no records")
	}

	periods := make(map[string]struct{})
	for _, rec := range result.Records {
		periods[rec.Period.Start+"/"+rec.Period.End] = struct{}{}
	}
	if got := len(periods); got != 13 {
		t.Errorf("distinct monthly periods in the SERVICE-grouped fixture: got %d want 13", got)
	}
}

// ---------------------------------------------------------------------------
// FR-003: invoice totals include a Tax row
// ---------------------------------------------------------------------------

func TestCostsDemo_InvoiceTotals_IncludeTaxRow(t *testing.T) {
	client := newDemoCostsClient()
	// No RECORD_TYPE filter -> invoice mode (FR-003: everything incl. Tax).
	q := demoCostsQuery(costs.Filter{}, costs.DimensionService)

	result, err := a9saws.FetchCostAndUsage(context.Background(), client, q)
	if err != nil {
		t.Fatalf("FetchCostAndUsage(SERVICE, invoice): %v", err)
	}

	found := false
	for _, rec := range result.Records {
		if len(rec.Keys) == 1 && rec.Keys[0] == "Tax" {
			found = true
			amt, ok := rec.Metrics[costs.MetricInvoice]
			if !ok {
				t.Errorf("Tax record for period %+v has no %q metric: %+v", rec.Period, costs.MetricInvoice, rec.Metrics)
				continue
			}
			if amt.Value <= 0 {
				t.Errorf("Tax record for period %+v has non-positive invoice amount %v", rec.Period, amt.Value)
			}
		}
	}
	if !found {
		t.Fatal("SERVICE-grouped invoice-mode fetch has no \"Tax\" row (FR-003)")
	}
}

// ---------------------------------------------------------------------------
// Planted growth story: fixtures.CostsGrowthService, driven by
// fixtures.CostsGrowthUsageType — pinned via the constants (not literals) so
// a re-plant under a different service/usage type (Codex X1: the story must
// live under a service with a registered resource-row mapping, e.g. "Amazon
// Elastic Compute Cloud - Compute") only requires editing the fixture, never
// this test.
// ---------------------------------------------------------------------------

func TestCostsDemo_GrowthStory_Doubling(t *testing.T) {
	client := newDemoCostsClient()
	q := demoCostsQuery(
		costs.Filter{Equals: map[costs.Dimension][]string{costs.DimensionService: {fixtures.CostsGrowthService}}},
		costs.DimensionUsageType,
	)

	result, err := a9saws.FetchCostAndUsage(context.Background(), client, q)
	if err != nil {
		t.Fatalf("FetchCostAndUsage(%s x USAGE_TYPE): %v", fixtures.CostsGrowthService, err)
	}

	var growthRecs []costs.Record
	for _, rec := range result.Records {
		if len(rec.Keys) == 1 && rec.Keys[0] == fixtures.CostsGrowthUsageType {
			growthRecs = append(growthRecs, rec)
		}
	}
	if len(growthRecs) < 2 {
		t.Fatalf("expected >= 2 monthly %s records under %s, got %d", fixtures.CostsGrowthUsageType, fixtures.CostsGrowthService, len(growthRecs))
	}
	sort.Slice(growthRecs, func(i, j int) bool { return growthRecs[i].Period.Start < growthRecs[j].Period.Start })

	const doublingThreshold = 1.5 // "roughly doubling" per the coordinator's own wording
	maxRatio := 0.0
	for i := 1; i < len(growthRecs); i++ {
		prev := growthRecs[i-1].Metrics[costs.MetricInvoice].Value
		cur := growthRecs[i].Metrics[costs.MetricInvoice].Value
		if prev <= 0 {
			continue
		}
		if ratio := cur / prev; ratio > maxRatio {
			maxRatio = ratio
		}
	}
	if maxRatio < doublingThreshold {
		t.Errorf("planted growth story not found: largest month-over-month %s ratio under %s is %.2fx, want >= %.1fx", fixtures.CostsGrowthUsageType, fixtures.CostsGrowthService, maxRatio, doublingThreshold)
	}
}

// ---------------------------------------------------------------------------
// Planted anomaly names the same service + usage type
// ---------------------------------------------------------------------------

func TestCostsDemo_Anomaly_RootCauseNamesServiceAndUsageType(t *testing.T) {
	client := newDemoCostsClient()
	window := costs.Period{Start: "2024-01-01", End: "2027-01-01"}

	marks, err := a9saws.FetchCostAnomalies(context.Background(), client, window)
	if err != nil {
		t.Fatalf("FetchCostAnomalies: %v", err)
	}
	if len(marks) == 0 {
		t.Fatal("FetchCostAnomalies returned no anomaly marks")
	}

	var match *costs.AnomalyMark
	for i := range marks {
		if marks[i].Dimension[costs.DimensionService] == fixtures.CostsGrowthService &&
			marks[i].Dimension[costs.DimensionUsageType] == fixtures.CostsGrowthUsageType {
			match = &marks[i]
			break
		}
	}
	if match == nil {
		t.Fatalf("no anomaly mark names service %q + usage type %q", fixtures.CostsGrowthService, fixtures.CostsGrowthUsageType)
	}
	if !strings.Contains(match.RootCause, fixtures.CostsGrowthService) {
		t.Errorf("anomaly RootCause missing the service name, got %q", match.RootCause)
	}
	if !strings.Contains(match.RootCause, fixtures.CostsGrowthUsageType) {
		t.Errorf("anomaly RootCause missing the usage type, got %q", match.RootCause)
	}
	if match.Impact.Value <= 0 {
		t.Errorf("planted anomaly Impact.Value: got %v, want a positive spend impact", match.Impact.Value)
	}
}
