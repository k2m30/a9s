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
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// apigwGetStagesFake implements APIGatewayV2API for enrichment testing.
// It embeds the aggregate interface and answers the two calls the enricher
// makes. The results map is keyed by API ID so the fake can serve different
// responses per resource.
//
// GetAuthorizers is answered rather than left to the embedded nil interface:
// the enricher calls every method of the aggregate it is given, and a nil
// embedded field dereferences into a SIGSEGV that takes the whole unit
// package down before any other test reports. The static
// `var _ APIGatewayV2API` assertion cannot catch it, because embedding
// satisfies the interface at compile time whether or not the field is set.
type apigwGetStagesFake struct {
	awsclient.APIGatewayV2API
	// results maps API ID → slice of Stage.
	results map[string][]apigwtypes.Stage
	// errByID maps API ID → error; overrides results when set.
	errByID map[string]error
	// authorizers maps API ID → slice of Authorizer. A nil map means every
	// API has one: these tests predate apigw.no-authorizer and are about the
	// stage-config rows, so an unauthorized API would add a finding none of
	// them is asking about. Set it to exercise row 18.
	authorizers map[string][]apigwtypes.Authorizer
}

func (f *apigwGetStagesFake) GetAuthorizers(
	_ context.Context,
	in *apigatewayv2.GetAuthorizersInput,
	_ ...func(*apigatewayv2.Options),
) (*apigatewayv2.GetAuthorizersOutput, error) {
	id := ""
	if in != nil && in.ApiId != nil {
		id = *in.ApiId
	}
	if f.authorizers == nil {
		return &apigatewayv2.GetAuthorizersOutput{Items: []apigwtypes.Authorizer{{
			AuthorizerId: aws.String("auth-default"),
			Name:         aws.String("acme-jwt"),
		}}}, nil
	}
	return &apigatewayv2.GetAuthorizersOutput{Items: f.authorizers[id]}, nil
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
			ID:   id,
			Name: "api-" + id,
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
	fs, ok := result.Findings[apigwAPIID1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (no throttling)", apigwAPIID1)
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	// Inverted for spec row "phrase": the wording belongs to the code and the
	// offending item is a supporting row. Do not restore the old assertion.
	if want := catalog.Phrase("apigw.stage-config-issues"); f.Phrase != want {
		t.Errorf("Phrase = %q, want the catalog's %q", f.Phrase, want)
	}
	if rows := fmt.Sprintf("%v", result.AttentionDetails[apigwAPIID1]["apigw.stage-config-issues"].Rows); !strings.Contains(strings.ToLower(rows), "throttling") {
		t.Errorf("no supporting row names the throttling issue: %s", rows)
	}
	if _, ok := result.Findings[apigwAPIID2]; ok {
		t.Error("api-2 must NOT appear in Findings — it has throttling configured")
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
	fs, ok := result.Findings[apigwAPIID1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (no access logs)", apigwAPIID1)
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	// Inverted for spec row "phrase": the wording belongs to the code and the
	// offending item is a supporting row. Do not restore the old assertion.
	if want := catalog.Phrase("apigw.stage-config-issues"); f.Phrase != want {
		t.Errorf("Phrase = %q, want the catalog's %q", f.Phrase, want)
	}
	if rows := fmt.Sprintf("%v", result.AttentionDetails[apigwAPIID1]["apigw.stage-config-issues"].Rows); !strings.Contains(strings.ToLower(rows), "access log") {
		t.Errorf("no supporting row names the access-log issue: %s", rows)
	}
	if _, ok := result.Findings[apigwAPIID2]; ok {
		t.Error("api-2 must NOT appear in Findings — it has access logs configured")
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
	fs, ok := result.Findings[emptyAPIID]
	if !ok {
		t.Fatalf(
			"expected a finding keyed by %q when 0 stages, got none — "+
				"per docs/attention-signals.md a deployed-stage check must emit sev \"~\"",
			emptyAPIID,
		)
	}
	f := fs[0]

	if f.Severity != domain.SevWarn {
		t.Errorf("finding Severity = %v, want SevWarn", f.Severity)
	}

	if !strings.Contains(strings.ToLower(f.Phrase), "no deployed") {
		t.Errorf("finding Summary = %q, must contain \"no deployed\"", f.Phrase)
	}
}

// TestEnrichAPIGatewayStage_APIErrorMarksRowTruncatedIDNotBadge verifies that when the
// API call for api-1 returns an error, the enricher marks that API's row via
// TruncatedIDs (a per-row "?" coverage gap), produces 0 findings for that API,
// and does not propagate the error. apigw is a "~"-only enricher (IssueCount
// always 0), so the coverage gap must never lower-bound the aggregate issue
// badge — Truncated stays false.
func TestEnrichAPIGatewayStage_APIErrorMarksRowTruncatedIDNotBadge(t *testing.T) {
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
	if result.Truncated {
		t.Error("Truncated must stay false: apigw is a \"~\"-only enricher, so an API error marks the row via TruncatedIDs, never the aggregate issue badge")
	}
	if !result.TruncatedIDs[apigwAPIID1] {
		t.Errorf("TruncatedIDs[%q] must be true — the GetStages error must mark that API's row with a \"?\" coverage gap", apigwAPIID1)
	}
}
