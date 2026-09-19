package unit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
	smithy "github.com/aws/smithy-go"

	a9saws "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/costs"
)

type mockCostsGetCostAndUsageClient struct {
	pages []*costexplorer.GetCostAndUsageOutput
	err   error
	calls int
}

func (m *mockCostsGetCostAndUsageClient) GetCostAndUsage(
	ctx context.Context,
	params *costexplorer.GetCostAndUsageInput,
	optFns ...func(*costexplorer.Options),
) (*costexplorer.GetCostAndUsageOutput, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	idx := m.calls - 1
	if idx >= len(m.pages) {
		return &costexplorer.GetCostAndUsageOutput{}, nil
	}
	return m.pages[idx], nil
}

type mockCostsGetCostAndUsageWithResourcesClient struct {
	pages []*costexplorer.GetCostAndUsageWithResourcesOutput
	calls int
}

func (m *mockCostsGetCostAndUsageWithResourcesClient) GetCostAndUsageWithResources(
	ctx context.Context,
	params *costexplorer.GetCostAndUsageWithResourcesInput,
	optFns ...func(*costexplorer.Options),
) (*costexplorer.GetCostAndUsageWithResourcesOutput, error) {
	m.calls++
	idx := m.calls - 1
	if idx >= len(m.pages) {
		return &costexplorer.GetCostAndUsageWithResourcesOutput{}, nil
	}
	return m.pages[idx], nil
}

type mockCostsGetAnomaliesClient struct {
	output *costexplorer.GetAnomaliesOutput
}

func (m *mockCostsGetAnomaliesClient) GetAnomalies(
	ctx context.Context,
	params *costexplorer.GetAnomaliesInput,
	optFns ...func(*costexplorer.Options),
) (*costexplorer.GetAnomaliesOutput, error) {
	return m.output, nil
}

func ceGroup(key string, amounts map[string]string) cetypes.Group {
	metrics := make(map[string]cetypes.MetricValue, len(amounts))
	for k, v := range amounts {
		val := v
		metrics[k] = cetypes.MetricValue{Amount: &val, Unit: strPtr("USD")}
	}
	return cetypes.Group{Keys: []string{key}, Metrics: metrics}
}

func TestFetchCostAndUsage_PaginatesAcrossPages_AccumulatesRecordsAndCountsRequests(t *testing.T) {
	page := func(token *string, key string) *costexplorer.GetCostAndUsageOutput {
		return &costexplorer.GetCostAndUsageOutput{
			NextPageToken: token,
			ResultsByTime: []cetypes.ResultByTime{
				{
					TimePeriod: &cetypes.DateInterval{Start: strPtr("2026-06-01"), End: strPtr("2026-07-01")},
					Groups: []cetypes.Group{
						ceGroup(key, map[string]string{
							"UnblendedCost":    "10.0000000000",
							"BlendedCost":      "10.0000000000",
							"AmortizedCost":    "10.0000000000",
							"NetAmortizedCost": "10.0000000000",
							"NetUnblendedCost": "10.0000000000",
						}),
					},
				},
			},
		}
	}

	mock := &mockCostsGetCostAndUsageClient{
		pages: []*costexplorer.GetCostAndUsageOutput{
			page(strPtr("tok-1"), "Amazon Elastic Compute Cloud - Compute"),
			page(strPtr("tok-2"), "Amazon Simple Storage Service"),
			page(nil, "EC2 - Other"),
		},
	}

	q := costs.Query{Granularity: "MONTHLY", GroupBy: []costs.Dimension{costs.Dimension("SERVICE")}, Range: costs.Period{Start: "2026-06-01", End: "2026-07-01"}}
	result, err := a9saws.FetchCostAndUsage(context.Background(), mock, q)
	if err != nil {
		t.Fatalf("FetchCostAndUsage() error = %v", err)
	}
	if mock.calls != 3 {
		t.Errorf("mock.calls = %d, want 3", mock.calls)
	}
	if result.RequestCount != 3 {
		t.Errorf("RequestCount = %d, want 3", result.RequestCount)
	}
	if len(result.Records) != 3 {
		t.Fatalf("len(Records) = %d, want 3 (one per page)", len(result.Records))
	}
}

func TestFetchCostAndUsage_DimensionValueAttributesMapIntoAttrs(t *testing.T) {
	mock := &mockCostsGetCostAndUsageClient{
		pages: []*costexplorer.GetCostAndUsageOutput{
			{
				DimensionValueAttributes: []cetypes.DimensionValuesWithAttributes{
					{Value: strPtr("123456789012"), Attributes: map[string]string{"description": "prod-account"}},
					{Value: strPtr("210987654321"), Attributes: map[string]string{"description": "staging-account"}},
				},
				ResultsByTime: []cetypes.ResultByTime{
					{
						TimePeriod: &cetypes.DateInterval{Start: strPtr("2026-06-01"), End: strPtr("2026-07-01")},
						Groups:     []cetypes.Group{ceGroup("123456789012", map[string]string{"UnblendedCost": "5.0000000000"})},
					},
				},
			},
		},
	}

	q := costs.Query{Granularity: "MONTHLY", GroupBy: []costs.Dimension{costs.Dimension("LINKED_ACCOUNT")}, Range: costs.Period{Start: "2026-06-01", End: "2026-07-01"}}
	result, err := a9saws.FetchCostAndUsage(context.Background(), mock, q)
	if err != nil {
		t.Fatalf("FetchCostAndUsage() error = %v", err)
	}
	want := map[string]string{"123456789012": "prod-account", "210987654321": "staging-account"}
	if len(result.Attrs) != len(want) {
		t.Fatalf("Attrs = %v, want %v", result.Attrs, want)
	}
	for k, v := range want {
		if result.Attrs[k] != v {
			t.Errorf("Attrs[%q] = %q, want %q", k, result.Attrs[k], v)
		}
	}
}

// CE returns NetUnblendedCost alongside the four metrics a9s maps.
func TestFetchCostAndUsage_ParsesAllFourMetrics(t *testing.T) {
	mock := &mockCostsGetCostAndUsageClient{
		pages: []*costexplorer.GetCostAndUsageOutput{
			{
				ResultsByTime: []cetypes.ResultByTime{
					{
						TimePeriod: &cetypes.DateInterval{Start: strPtr("2026-06-01"), End: strPtr("2026-07-01")},
						Groups: []cetypes.Group{
							ceGroup("Amazon Elastic Compute Cloud - Compute", map[string]string{
								"UnblendedCost":    "100.0000000000",
								"BlendedCost":      "200.0000000000",
								"AmortizedCost":    "300.0000000000",
								"NetAmortizedCost": "400.0000000000",
								"NetUnblendedCost": "500.0000000000",
							}),
						},
					},
				},
			},
		},
	}

	// No RECORD_TYPE filter is the "invoice" query shape, so UnblendedCost lands
	// under Metric("invoice").
	q := costs.Query{Granularity: "MONTHLY", GroupBy: []costs.Dimension{costs.Dimension("SERVICE")}, Range: costs.Period{Start: "2026-06-01", End: "2026-07-01"}}
	result, err := a9saws.FetchCostAndUsage(context.Background(), mock, q)
	if err != nil {
		t.Fatalf("FetchCostAndUsage() error = %v", err)
	}
	if len(result.Records) != 1 {
		t.Fatalf("len(Records) = %d, want 1", len(result.Records))
	}
	metrics := result.Records[0].Metrics
	if len(metrics) != 4 {
		t.Fatalf("len(Metrics) = %d, want 4 (the net-unblended metric dropped), got %+v", len(metrics), metrics)
	}
	var sum float64
	for _, amt := range metrics {
		sum += amt.Value
	}
	if sum != 1000 {
		t.Errorf("sum of all metric values = %v, want 1000 (100+200+300+400, NetUnblendedCost's 500 correctly dropped)", sum)
	}
	if got := metrics[costs.Metric("invoice")].Value; got != 100 {
		t.Errorf("Metrics[invoice] = %v, want 100 (UnblendedCost, no RECORD_TYPE filter)", got)
	}
	if got := metrics[costs.Metric("blended")].Value; got != 200 {
		t.Errorf("Metrics[blended] = %v, want 200 (BlendedCost)", got)
	}
	if got := metrics[costs.Metric("amortized")].Value; got != 300 {
		t.Errorf("Metrics[amortized] = %v, want 300 (AmortizedCost)", got)
	}
	if got := metrics[costs.Metric("net-amortized")].Value; got != 400 {
		t.Errorf("Metrics[net-amortized] = %v, want 400 (NetAmortizedCost)", got)
	}
}

func TestFetchCostAndUsage_PropagatesEstimatedFlag(t *testing.T) {
	tests := []struct {
		name      string
		estimated bool
	}{
		{name: "estimated result", estimated: true},
		{name: "final (non-estimated) result", estimated: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockCostsGetCostAndUsageClient{
				pages: []*costexplorer.GetCostAndUsageOutput{
					{
						ResultsByTime: []cetypes.ResultByTime{
							{
								Estimated:  tt.estimated,
								TimePeriod: &cetypes.DateInterval{Start: strPtr("2026-07-01"), End: strPtr("2026-08-01")},
								Groups:     []cetypes.Group{ceGroup("Amazon Elastic Compute Cloud - Compute", map[string]string{"UnblendedCost": "10.0000000000"})},
							},
						},
					},
				},
			}
			q := costs.Query{Granularity: "MONTHLY", GroupBy: []costs.Dimension{costs.Dimension("SERVICE")}, Range: costs.Period{Start: "2026-07-01", End: "2026-08-01"}}
			result, err := a9saws.FetchCostAndUsage(context.Background(), mock, q)
			if err != nil {
				t.Fatalf("FetchCostAndUsage() error = %v", err)
			}
			if got := result.Records[0].Estimated; got != tt.estimated {
				t.Errorf("Records[0].Estimated = %v, want %v", got, tt.estimated)
			}
		})
	}
}

func TestFetchCostAndUsage_ClassifiesErrors(t *testing.T) {
	q := costs.Query{Granularity: "MONTHLY", GroupBy: []costs.Dimension{costs.Dimension("SERVICE")}, Range: costs.Period{Start: "2026-06-01", End: "2026-07-01"}}

	t.Run("AccessDeniedException maps to ErrCostsAccessDenied", func(t *testing.T) {
		mock := &mockCostsGetCostAndUsageClient{err: &boundaryAPIError{Code: "AccessDeniedException", Message: "User is not authorized to perform: ce:GetCostAndUsage", Fault: smithy.FaultClient}}
		_, err := a9saws.FetchCostAndUsage(context.Background(), mock, q)
		if !errors.Is(err, a9saws.ErrCostsAccessDenied) {
			t.Errorf("FetchCostAndUsage() error = %v, want errors.Is(err, ErrCostsAccessDenied)", err)
		}
	})

	t.Run("DataUnavailableException maps to ErrCostsDataUnavailable", func(t *testing.T) {
		mock := &mockCostsGetCostAndUsageClient{err: &cetypes.DataUnavailableException{Message: strPtr("Cost Explorer data is not available yet for this account")}}
		_, err := a9saws.FetchCostAndUsage(context.Background(), mock, q)
		if !errors.Is(err, a9saws.ErrCostsDataUnavailable) {
			t.Errorf("FetchCostAndUsage() error = %v, want errors.Is(err, ErrCostsDataUnavailable)", err)
		}
	})

	t.Run("ThrottlingException maps to ErrCostsThrottled", func(t *testing.T) {
		mock := &mockCostsGetCostAndUsageClient{err: &boundaryAPIError{Code: "ThrottlingException", Message: "Rate exceeded", Fault: smithy.FaultClient}}
		_, err := a9saws.FetchCostAndUsage(context.Background(), mock, q)
		if !errors.Is(err, a9saws.ErrCostsThrottled) {
			t.Errorf("FetchCostAndUsage() error = %v, want errors.Is(err, ErrCostsThrottled)", err)
		}
	})

	t.Run("other errors pass through unclassified", func(t *testing.T) {
		mock := &mockCostsGetCostAndUsageClient{err: errors.New("boom-network-timeout")}
		_, err := a9saws.FetchCostAndUsage(context.Background(), mock, q)
		if err == nil {
			t.Fatal("FetchCostAndUsage() error = nil, want a passthrough error")
		}
		if errors.Is(err, a9saws.ErrCostsAccessDenied) || errors.Is(err, a9saws.ErrCostsDataUnavailable) || errors.Is(err, a9saws.ErrCostsThrottled) {
			t.Errorf("FetchCostAndUsage() error = %v, want none of the typed sentinels for an unrelated failure", err)
		}
	})
}

func TestFetchCostAndUsageWithResources_MapsRecordsAndCountsRequests(t *testing.T) {
	mock := &mockCostsGetCostAndUsageWithResourcesClient{
		pages: []*costexplorer.GetCostAndUsageWithResourcesOutput{
			{
				ResultsByTime: []cetypes.ResultByTime{
					{
						Estimated:  true,
						TimePeriod: &cetypes.DateInterval{Start: strPtr("2026-07-05"), End: strPtr("2026-07-06")},
						Groups:     []cetypes.Group{ceGroup("i-0abcd1234ef567890", map[string]string{"UnblendedCost": "3.5000000000"})},
					},
				},
			},
		},
	}

	q := costs.Query{
		Granularity: "DAILY",
		GroupBy:     []costs.Dimension{costs.Dimension("RESOURCE_ID")},
		Filter:      filterPinning(costs.Dimension("SERVICE"), "Amazon Elastic Compute Cloud - Compute"),
		Range:       costs.Period{Start: "2026-07-05", End: "2026-07-06"},
	}
	result, err := a9saws.FetchCostAndUsageWithResources(context.Background(), mock, q)
	if err != nil {
		t.Fatalf("FetchCostAndUsageWithResources() error = %v", err)
	}
	if result.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1", result.RequestCount)
	}
	if len(result.Records) != 1 {
		t.Fatalf("len(Records) = %d, want 1", len(result.Records))
	}
	if !result.Records[0].Estimated {
		t.Error("Records[0].Estimated = false, want true")
	}
	if got := result.Records[0].Keys[0]; got != "i-0abcd1234ef567890" {
		t.Errorf("Records[0].Keys[0] = %q, want the resource id", got)
	}
}

// Dimension and Period are the only mapped representation of a root cause.
func TestFetchCostAnomalies_MapsDimension(t *testing.T) {
	mock := &mockCostsGetAnomaliesClient{
		output: &costexplorer.GetAnomaliesOutput{
			Anomalies: []cetypes.Anomaly{
				{
					AnomalyId:        strPtr("anomaly-1a2b3c"),
					AnomalyScore:     &cetypes.AnomalyScore{CurrentScore: 92.0, MaxScore: 92.0},
					Impact:           &cetypes.Impact{MaxImpact: 452.10, TotalImpact: 452.10},
					MonitorArn:       strPtr("arn:aws:ce::123456789012:anomalymonitor/8f3b2c1a-0000-0000-0000-000000000000"),
					AnomalyStartDate: strPtr("2026-07-01"),
					AnomalyEndDate:   strPtr("2026-07-08"),
					RootCauses: []cetypes.RootCause{
						{
							Service:   strPtr("Amazon Elastic Compute Cloud - Compute"),
							Region:    strPtr("us-east-1"),
							UsageType: strPtr("BoxUsage:m5.2xlarge"),
							Impact:    &cetypes.RootCauseImpact{Contribution: 452.10},
						},
					},
				},
			},
		},
	}

	marks, _, err := a9saws.FetchCostAnomaliesCounted(context.Background(), mock, costs.Period{Start: "2026-07-01", End: "2026-07-08"})
	if err != nil {
		t.Fatalf("FetchCostAnomaliesCounted() error = %v", err)
	}
	if len(marks) != 1 {
		t.Fatalf("len(marks) = %d, want 1", len(marks))
	}
	m := marks[0]
	wantDim := map[costs.Dimension]string{
		costs.Dimension("SERVICE"):    "Amazon Elastic Compute Cloud - Compute",
		costs.Dimension("REGION"):     "us-east-1",
		costs.Dimension("USAGE_TYPE"): "BoxUsage:m5.2xlarge",
	}
	if len(m.Dimension) != len(wantDim) {
		t.Fatalf("Dimension = %v, want %v", m.Dimension, wantDim)
	}
	for k, v := range wantDim {
		if m.Dimension[k] != v {
			t.Errorf("Dimension[%q] = %q, want %q", k, m.Dimension[k], v)
		}
	}
	if m.Period != (costs.Period{Start: "2026-07-01", End: "2026-07-08"}) {
		t.Errorf("Period = %+v, want {2026-07-01 2026-07-08}", m.Period)
	}
}
