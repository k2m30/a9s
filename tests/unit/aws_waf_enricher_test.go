package unit

// aws_waf_enricher_test.go — Behavioral tests for EnrichWAFLogging.
//
// Contract assertions:
//   - GetLoggingConfiguration is called once per WAF WebACL resource (keyed by ARN).
//   - ListResourcesForWebACL is called once per WebACL to check associations.
//   - Logging enabled AND resources associated → 0 findings.
//   - WAFNonexistentItemException from GetLoggingConfiguration → 1 finding sev "~" "no logging" for that WebACL.
//   - ListResourcesForWebACL returns empty list → 1 finding sev "~" "not associated" for that WebACL.
//   - clients.WAFv2 == nil → (EnricherResult{Findings: non-nil empty}, nil).
//   - API error (generic) → 0 findings, Truncated=true, no error returned.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// wafLoggingFake implements WAFv2API for enrichment testing.
// It embeds the interface and overrides only GetLoggingConfiguration and
// ListResourcesForWebACL so the fake only needs to serve the two methods
// used by EnrichWAFLogging.  Result maps are keyed by WebACL ARN.
type wafLoggingFake struct {
	awsclient.WAFv2API
	// loggingResults maps ARN → GetLoggingConfigurationOutput.
	loggingResults map[string]*wafv2.GetLoggingConfigurationOutput
	// loggingErrByARN maps ARN → error; overrides loggingResults when set.
	loggingErrByARN map[string]error
	// resourcesResults maps ARN → ListResourcesForWebACLOutput.
	resourcesResults map[string]*wafv2.ListResourcesForWebACLOutput
	// resourcesErrByARN maps ARN → error; overrides resourcesResults when set.
	resourcesErrByARN map[string]error
}

func (f *wafLoggingFake) GetLoggingConfiguration(
	_ context.Context,
	in *wafv2.GetLoggingConfigurationInput,
	_ ...func(*wafv2.Options),
) (*wafv2.GetLoggingConfigurationOutput, error) {
	arn := ""
	if in != nil && in.ResourceArn != nil {
		arn = *in.ResourceArn
	}
	if f.loggingErrByARN != nil {
		if err, ok := f.loggingErrByARN[arn]; ok {
			return nil, err
		}
	}
	out, ok := f.loggingResults[arn]
	if !ok {
		return &wafv2.GetLoggingConfigurationOutput{}, nil
	}
	return out, nil
}

func (f *wafLoggingFake) ListResourcesForWebACL(
	_ context.Context,
	in *wafv2.ListResourcesForWebACLInput,
	_ ...func(*wafv2.Options),
) (*wafv2.ListResourcesForWebACLOutput, error) {
	arn := ""
	if in != nil && in.WebACLArn != nil {
		arn = *in.WebACLArn
	}
	if f.resourcesErrByARN != nil {
		if err, ok := f.resourcesErrByARN[arn]; ok {
			return nil, err
		}
	}
	out, ok := f.resourcesResults[arn]
	if !ok {
		return &wafv2.ListResourcesForWebACLOutput{}, nil
	}
	return out, nil
}

// Compile-time check: wafLoggingFake satisfies WAFv2API.
var _ awsclient.WAFv2API = (*wafLoggingFake)(nil)

// wafWebACLResources returns a slice of WAF Resource stubs with the given ARNs.
// The ID field is set to the ARN to match how the enricher keys resources.
func wafWebACLResources(arns ...string) []resource.Resource {
	res := make([]resource.Resource, 0, len(arns))
	for i, arn := range arns {
		id := "webacl-id-" + strings.TrimPrefix(arn, "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/")
		_ = i
		res = append(res, resource.Resource{
			ID:   arn,
			Name: "webacl-" + id,
			Fields: map[string]string{
				"name":        "webacl-" + id,
				"id":          id,
				"arn":         arn,
				"description": "",
			},
		})
	}
	return res
}

// wafLoggingOutput returns a GetLoggingConfigurationOutput indicating logging is configured.
func wafLoggingOutput(arn string) *wafv2.GetLoggingConfigurationOutput {
	return &wafv2.GetLoggingConfigurationOutput{
		LoggingConfiguration: &wafv2types.LoggingConfiguration{
			ResourceArn:           &arn,
			LogDestinationConfigs: []string{"arn:aws:logs:us-east-1:123456789012:log-group:aws-waf-logs-example"},
		},
	}
}

// wafResourcesOutput returns a ListResourcesForWebACLOutput with the given resource ARNs.
func wafResourcesOutput(resourceARNs ...string) *wafv2.ListResourcesForWebACLOutput {
	return &wafv2.ListResourcesForWebACLOutput{
		ResourceArns: resourceARNs,
	}
}

const (
	wafACLARN1 = "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-acl-1/aaaabbbb-1111-2222-3333-444444444444"
	wafACLARN2 = "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-acl-2/bbbbcccc-1111-2222-3333-444444444444"
)

// TestEnrichWAFLogging_LoggedAndAssociatedProducesNoFindings verifies that when both
// WebACLs have logging configured and are associated with resources, no findings are produced.
func TestEnrichWAFLogging_LoggedAndAssociatedProducesNoFindings(t *testing.T) {
	fake := &wafLoggingFake{
		loggingResults: map[string]*wafv2.GetLoggingConfigurationOutput{
			wafACLARN1: wafLoggingOutput(wafACLARN1),
			wafACLARN2: wafLoggingOutput(wafACLARN2),
		},
		resourcesResults: map[string]*wafv2.ListResourcesForWebACLOutput{
			wafACLARN1: wafResourcesOutput("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-1/aabbccdd"),
			wafACLARN2: wafResourcesOutput("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-2/aabbccdd"),
		},
	}
	clients := &awsclient.ServiceClients{WAFv2: fake}
	resources := wafWebACLResources(wafACLARN1, wafACLARN2)

	result, err := awsclient.EnrichWAFLogging(context.Background(), clients, resources)
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

// TestEnrichWAFLogging_NoLoggingProducesFindingSevTilde verifies that when acl-1
// returns WAFNonexistentItemException (logging not configured), a finding with
// severity "~" containing "no logging" is produced for acl-1 only.
func TestEnrichWAFLogging_NoLoggingProducesFindingSevTilde(t *testing.T) {
	notExistErr := &wafv2types.WAFNonexistentItemException{
		Message: stringPtr("logging configuration not found"),
	}
	fake := &wafLoggingFake{
		loggingErrByARN: map[string]error{
			wafACLARN1: notExistErr,
		},
		loggingResults: map[string]*wafv2.GetLoggingConfigurationOutput{
			wafACLARN2: wafLoggingOutput(wafACLARN2),
		},
		resourcesResults: map[string]*wafv2.ListResourcesForWebACLOutput{
			wafACLARN1: wafResourcesOutput("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-1/aabbccdd"),
			wafACLARN2: wafResourcesOutput("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-2/aabbccdd"),
		},
	}
	clients := &awsclient.ServiceClients{WAFv2: fake}
	resources := wafWebACLResources(wafACLARN1, wafACLARN2)

	result, err := awsclient.EnrichWAFLogging(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings[wafACLARN1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (no logging)", wafACLARN1)
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
	if !strings.Contains(strings.ToLower(f.Summary), "no logging") {
		t.Errorf("summary %q must contain \"no logging\"", f.Summary)
	}
	if _, ok := result.Findings[wafACLARN2]; ok {
		t.Error("acl-2 must NOT appear in Findings — it has logging configured")
	}
	// "~" findings do NOT contribute to IssueCount per the EnricherResult contract.
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
}

// TestEnrichWAFLogging_OrphanACLProducesFindingSevTilde verifies that when acl-1
// has no associated resources (ListResourcesForWebACL returns empty), a finding with
// severity "~" containing "not associated" is produced for acl-1 only.
func TestEnrichWAFLogging_OrphanACLProducesFindingSevTilde(t *testing.T) {
	fake := &wafLoggingFake{
		loggingResults: map[string]*wafv2.GetLoggingConfigurationOutput{
			wafACLARN1: wafLoggingOutput(wafACLARN1),
			wafACLARN2: wafLoggingOutput(wafACLARN2),
		},
		resourcesResults: map[string]*wafv2.ListResourcesForWebACLOutput{
			wafACLARN1: wafResourcesOutput(), // empty — orphan
			wafACLARN2: wafResourcesOutput("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-2/aabbccdd"),
		},
	}
	clients := &awsclient.ServiceClients{WAFv2: fake}
	resources := wafWebACLResources(wafACLARN1, wafACLARN2)

	result, err := awsclient.EnrichWAFLogging(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings[wafACLARN1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (orphan ACL)", wafACLARN1)
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
	if !strings.Contains(strings.ToLower(f.Summary), "not associated") {
		t.Errorf("summary %q must contain \"not associated\"", f.Summary)
	}
	if _, ok := result.Findings[wafACLARN2]; ok {
		t.Error("acl-2 must NOT appear in Findings — it is associated with a resource")
	}
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
}

// TestEnrichWAFLogging_NilClientReturnsEmptyFindingsNoError verifies that when
// clients.WAFv2 is nil the enricher returns a non-nil empty Findings map and no error.
func TestEnrichWAFLogging_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{WAFv2: nil}

	result, err := awsclient.EnrichWAFLogging(context.Background(), clients, wafWebACLResources(wafACLARN1, wafACLARN2))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when WAFv2 client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}

// TestEnrichWAFLogging_APIErrorSetsTruncatedNoError verifies that when the API call
// returns a generic error, the enricher sets Truncated=true, produces 0 findings,
// and does not propagate the error.
func TestEnrichWAFLogging_APIErrorSetsTruncatedNoError(t *testing.T) {
	apiErr := errors.New("wafv2: GetLoggingConfiguration throttled")
	fake := &wafLoggingFake{
		loggingErrByARN: map[string]error{
			wafACLARN1: apiErr,
			wafACLARN2: apiErr,
		},
	}
	clients := &awsclient.ServiceClients{WAFv2: fake}
	resources := wafWebACLResources(wafACLARN1, wafACLARN2)

	result, err := awsclient.EnrichWAFLogging(context.Background(), clients, resources)
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

// stringPtr is a local helper — avoids importing aws just for a test helper.
func stringPtr(s string) *string { return &s }
