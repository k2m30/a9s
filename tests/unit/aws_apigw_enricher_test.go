package unit

// aws_apigw_enricher_test.go — Behavioral tests for EnrichAPIGatewayStage.
//
// Contract assertions:
//   - GetStages is called once per API Gateway resource (keyed by API ID).
//   - Stages with ThrottlingBurstLimit > 0 AND AccessLogSettings non-nil → 0 findings.
//   - A stage with ThrottlingBurstLimit=0 → 1 finding sev "~" "throttling" for that API.
//   - A stage with AccessLogSettings=nil → 1 finding sev "~" "access logs" for that API.
//   - clients.APIGatewayV2 == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error for a resource → 0 findings for that resource, Truncated=true, no error returned.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// apigwGetStagesFake implements APIGatewayV2API for enrichment testing.
// It embeds the aggregate interface and overrides only GetStages.
// The results map is keyed by API ID so the fake can serve different
// responses per resource.
type apigwGetStagesFake struct {
	awsclient.APIGatewayV2API
	// results maps API ID → slice of Stage.
	results map[string][]apigwtypes.Stage
	// errByID maps API ID → error; overrides results when set.
	errByID map[string]error
}

func (f *apigwGetStagesFake) GetStages(
	_ context.Context,
	in *apigatewayv2.GetStagesInput,
	_ ...func(*apigatewayv2.Options),
) (*apigatewayv2.GetStagesOutput, error) {
	id := ""
	if in != nil && in.ApiId != nil {
		id = *in.ApiId
	}
	if f.errByID != nil {
		if err, ok := f.errByID[id]; ok {
			return nil, err
		}
	}
	stages, ok := f.results[id]
	if !ok {
		return &apigatewayv2.GetStagesOutput{Items: []apigwtypes.Stage{}}, nil
	}
	return &apigatewayv2.GetStagesOutput{Items: stages}, nil
}

// Compile-time check: apigwGetStagesFake satisfies APIGatewayV2API.
var _ awsclient.APIGatewayV2API = (*apigwGetStagesFake)(nil)

// apigwResources returns a slice of API Gateway Resource stubs with the given API IDs.
func apigwResources(ids ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(ids))
	for _, id := range ids {
		res = append(res, resource.Resource{
			ID:     id,
			Name:   "api-" + id,
			Status: "ACTIVE",
			Fields: map[string]string{
				"api_id":       id,
				"name":         "api-" + id,
				"protocol":     "HTTP",
				"created_date": "2024-01-01",
			},
		})
	}
	return res
}

// apigwStageWithThrottlingAndLogs builds a Stage with throttling and access log settings configured.
func apigwStageWithThrottlingAndLogs(name string) apigwtypes.Stage {
	return apigwtypes.Stage{
		StageName: aws.String(name),
		DefaultRouteSettings: &apigwtypes.RouteSettings{
			ThrottlingBurstLimit: aws.Int32(500),
			ThrottlingRateLimit:  aws.Float64(1000),
		},
		AccessLogSettings: &apigwtypes.AccessLogSettings{
			DestinationArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/apigateway/" + name),
			Format:         aws.String(`{"requestId":"$context.requestId"}`),
		},
	}
}

const (
	apigwAPIID1 = "api1abc123"
	apigwAPIID2 = "api2def456"
)

// TestEnrichAPIGatewayStage_ThrottledWithLogsProducesNoFindings verifies that when all
// API stages have throttling configured (ThrottlingBurstLimit > 0) AND access log settings
// (AccessLogSettings non-nil), no findings are produced.
func TestEnrichAPIGatewayStage_ThrottledWithLogsProducesNoFindings(t *testing.T) {
	fake := &apigwGetStagesFake{
		results: map[string][]apigwtypes.Stage{
			apigwAPIID1: {apigwStageWithThrottlingAndLogs("$default")},
			apigwAPIID2: {apigwStageWithThrottlingAndLogs("prod")},
		},
	}
	clients := &awsclient.ServiceClients{APIGatewayV2: fake}
	resources := apigwResources(apigwAPIID1, apigwAPIID2)

	result, err := awsclient.EnrichAPIGatewayStage(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0", result.IssueCount)
	}
}

// TestEnrichAPIGatewayStage_NoThrottlingProducesFindingSevTilde verifies that when api-1
// has a stage with ThrottlingBurstLimit=0, a finding with severity "~" and a summary
// containing "throttling" is produced for api-1 only.
func TestEnrichAPIGatewayStage_NoThrottlingProducesFindingSevTilde(t *testing.T) {
	stageNoThrottling := apigwtypes.Stage{
		StageName: aws.String("$default"),
		DefaultRouteSettings: &apigwtypes.RouteSettings{
			ThrottlingBurstLimit: aws.Int32(0),
		},
		AccessLogSettings: &apigwtypes.AccessLogSettings{
			DestinationArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/apigateway/api1"),
			Format:         aws.String(`{"requestId":"$context.requestId"}`),
		},
	}
	fake := &apigwGetStagesFake{
		results: map[string][]apigwtypes.Stage{
			apigwAPIID1: {stageNoThrottling},
			apigwAPIID2: {apigwStageWithThrottlingAndLogs("prod")},
		},
	}
	clients := &awsclient.ServiceClients{APIGatewayV2: fake}
	resources := apigwResources(apigwAPIID1, apigwAPIID2)

	result, err := awsclient.EnrichAPIGatewayStage(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings[apigwAPIID1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (no throttling)", apigwAPIID1)
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
	if !strings.Contains(strings.ToLower(f.Summary), "throttling") {
		t.Errorf("summary %q must contain \"throttling\"", f.Summary)
	}
	if _, ok := result.Findings[apigwAPIID2]; ok {
		t.Error("api-2 must NOT appear in Findings — it has throttling configured")
	}
	// "~" findings do NOT contribute to IssueCount per the EnricherResult contract.
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
}

// TestEnrichAPIGatewayStage_NoAccessLogsProducesFindingSevTilde verifies that when api-1
// has a stage with AccessLogSettings=nil, a finding with severity "~" and a summary
// containing "access logs" is produced for api-1 only.
func TestEnrichAPIGatewayStage_NoAccessLogsProducesFindingSevTilde(t *testing.T) {
	stageNoLogs := apigwtypes.Stage{
		StageName: aws.String("$default"),
		DefaultRouteSettings: &apigwtypes.RouteSettings{
			ThrottlingBurstLimit: aws.Int32(500),
			ThrottlingRateLimit:  aws.Float64(1000),
		},
		AccessLogSettings: nil, // no access logs configured
	}
	fake := &apigwGetStagesFake{
		results: map[string][]apigwtypes.Stage{
			apigwAPIID1: {stageNoLogs},
			apigwAPIID2: {apigwStageWithThrottlingAndLogs("prod")},
		},
	}
	clients := &awsclient.ServiceClients{APIGatewayV2: fake}
	resources := apigwResources(apigwAPIID1, apigwAPIID2)

	result, err := awsclient.EnrichAPIGatewayStage(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings[apigwAPIID1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (no access logs)", apigwAPIID1)
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
	if !strings.Contains(strings.ToLower(f.Summary), "access log") {
		t.Errorf("summary %q must contain \"access log\"", f.Summary)
	}
	if _, ok := result.Findings[apigwAPIID2]; ok {
		t.Error("api-2 must NOT appear in Findings — it has access logs configured")
	}
	// "~" findings do NOT contribute to IssueCount per the EnricherResult contract.
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
}

// TestEnrichAPIGatewayStage_NilClientReturnsEmptyFindingsNoError verifies that when
// clients.APIGatewayV2 is nil the enricher returns a non-nil empty Findings map and no error.
func TestEnrichAPIGatewayStage_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{APIGatewayV2: nil}

	result, err := awsclient.EnrichAPIGatewayStage(context.Background(), clients, apigwResources(apigwAPIID1, apigwAPIID2), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when APIGatewayV2 client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichAPIGatewayStage_ZeroStagesEmitsWarning verifies that when GetStages
// returns 0 stages for an API, the enricher emits an EnrichmentFinding with
// severity "~" and a summary containing "no deployed" for that API.
//
// Per docs/attention-signals.md Wave 2: "no deployed stage" is a signal worth
// surfacing to the operator — a REST/HTTP API with no stage is inactive.
//
// CODER NOTE: Currently apigw_issue_enrichment.go `continue`s without
// emitting any finding when stages == 0. This must change. After the fix,
// TestEnrichAPIGatewayStage_ZeroStagesAcrossPages in aws_apigw_v2_pagination_test.go
// (which asserts len(result.Findings)==0 for 0 stages) will need its expectation
// updated to reflect the new behavior — that is the coder's responsibility.
func TestEnrichAPIGatewayStage_ZeroStagesEmitsWarning(t *testing.T) {
	const emptyAPIID = "empty-api-warn-001"

	fake := &apigwGetStagesFake{
		results: map[string][]apigwtypes.Stage{
			// 0 stages for this API (key present but empty slice)
			emptyAPIID: {},
		},
	}
	clients := &awsclient.ServiceClients{APIGatewayV2: fake}
	resources := apigwResources(emptyAPIID)

	result, err := awsclient.EnrichAPIGatewayStage(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A finding must be emitted for the API with 0 stages.
	f, ok := result.Findings[emptyAPIID]
	if !ok {
		t.Fatalf(
			"expected a finding keyed by %q when 0 stages, got none — "+
				"per docs/attention-signals.md a deployed-stage check must emit sev \"~\"",
			emptyAPIID,
		)
	}

	if f.Severity != "~" {
		t.Errorf("finding Severity = %q, want \"~\"", f.Severity)
	}

	if !strings.Contains(strings.ToLower(f.Summary), "no deployed") {
		t.Errorf("finding Summary = %q, must contain \"no deployed\"", f.Summary)
	}
}

// TestEnrichAPIGatewayStage_APIErrorSetsTruncatedNoError verifies that when the API
// call for api-1 returns an error, the enricher sets Truncated=true, produces 0
// findings for that API, and does not propagate the error.
func TestEnrichAPIGatewayStage_APIErrorSetsTruncatedNoError(t *testing.T) {
	apiErr := errors.New("apigatewayv2: GetStages throttled")
	fake := &apigwGetStagesFake{
		errByID: map[string]error{
			apigwAPIID1: apiErr,
		},
		results: map[string][]apigwtypes.Stage{
			apigwAPIID2: {apigwStageWithThrottlingAndLogs("prod")},
		},
	}
	clients := &awsclient.ServiceClients{APIGatewayV2: fake}
	resources := apigwResources(apigwAPIID1, apigwAPIID2)

	result, err := awsclient.EnrichAPIGatewayStage(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings on API error, got %d", len(result.Findings))
	}
	if !result.Truncated {
		t.Error("Truncated must be true when an API call fails")
	}
}
