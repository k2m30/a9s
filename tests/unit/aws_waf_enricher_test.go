package unit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type wafLoggingFake struct {
	awsclient.WAFv2API
	loggingResults    map[string]*wafv2.GetLoggingConfigurationOutput
	loggingErrByARN   map[string]error
	resourcesResults  map[string]*wafv2.ListResourcesForWebACLOutput
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

var _ awsclient.WAFv2API = (*wafLoggingFake)(nil)

// The enricher keys WebACLs by ARN, so ID is the ARN.
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

func wafLoggingOutput(arn string) *wafv2.GetLoggingConfigurationOutput {
	return &wafv2.GetLoggingConfigurationOutput{
		LoggingConfiguration: &wafv2types.LoggingConfiguration{
			ResourceArn:           &arn,
			LogDestinationConfigs: []string{"arn:aws:logs:us-east-1:123456789012:log-group:aws-waf-logs-example"},
		},
	}
}

func wafResourcesOutput(resourceARNs ...string) *wafv2.ListResourcesForWebACLOutput {
	return &wafv2.ListResourcesForWebACLOutput{
		ResourceArns: resourceARNs,
	}
}

const (
	wafACLARN1 = "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-acl-1/aaaabbbb-1111-2222-3333-444444444444"
	wafACLARN2 = "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-acl-2/bbbbcccc-1111-2222-3333-444444444444"
)

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

	result, err := awsclient.EnrichWAFLogging(context.Background(), clients, resources, nil)
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

	result, err := awsclient.EnrichWAFLogging(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[wafACLARN1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (no logging)", wafACLARN1)
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	if !strings.Contains(strings.ToLower(f.Phrase), "no logging") {
		t.Errorf("summary %q must contain \"no logging\"", f.Phrase)
	}
	if _, ok := result.Findings[wafACLARN2]; ok {
		t.Error("acl-2 must NOT appear in Findings — it has logging configured")
	}
}

func TestEnrichWAFLogging_OrphanACLProducesFindingSevTilde(t *testing.T) {
	fake := &wafLoggingFake{
		loggingResults: map[string]*wafv2.GetLoggingConfigurationOutput{
			wafACLARN1: wafLoggingOutput(wafACLARN1),
			wafACLARN2: wafLoggingOutput(wafACLARN2),
		},
		resourcesResults: map[string]*wafv2.ListResourcesForWebACLOutput{
			wafACLARN1: wafResourcesOutput(),
			wafACLARN2: wafResourcesOutput("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/lb-2/aabbccdd"),
		},
	}
	clients := &awsclient.ServiceClients{WAFv2: fake}
	resources := wafWebACLResources(wafACLARN1, wafACLARN2)

	result, err := awsclient.EnrichWAFLogging(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fs, ok := result.Findings[wafACLARN1]
	if !ok {
		t.Fatalf("expected finding keyed by %q (orphan ACL)", wafACLARN1)
	}
	f := fs[0]
	if f.Severity != domain.SevWarn {
		t.Errorf("severity = %v, want %v", f.Severity, "~")
	}
	// An ACL that is attached to nothing is not a logging gap; it carries its
	// own code and its own wording, not waf.no-logging.
	if f.Code != "waf.orphan" {
		t.Errorf("Code = %q, want waf.orphan", f.Code)
	}
	if want := catalog.Phrase("waf.orphan"); f.Phrase != want {
		t.Errorf("Phrase = %q, want the catalog's %q", f.Phrase, want)
	}
	if rows := fmt.Sprintf("%v", result.AttentionDetails[wafACLARN1]["waf.orphan"].Rows); !strings.Contains(rows, "Associations") {
		t.Errorf("no supporting row names the missing associations: %s", rows)
	}
	if _, ok := result.Findings[wafACLARN2]; ok {
		t.Error("acl-2 must NOT appear in Findings — it is associated with a resource")
	}
}

func TestEnrichWAFLogging_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{WAFv2: nil}

	result, err := awsclient.EnrichWAFLogging(context.Background(), clients, wafWebACLResources(wafACLARN1, wafACLARN2), nil)
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

// waf only emits "~" findings, so a coverage gap never lower-bounds the
// issue badge: Truncated stays false.
func TestEnrichWAFLogging_APIErrorMarksRowTruncatedIDNotBadge(t *testing.T) {
	apiErr := errors.New("wafv2: GetLoggingConfiguration throttled")
	fake := &wafLoggingFake{
		loggingErrByARN: map[string]error{
			wafACLARN1: apiErr,
			wafACLARN2: apiErr,
		},
	}
	clients := &awsclient.ServiceClients{WAFv2: fake}
	resources := wafWebACLResources(wafACLARN1, wafACLARN2)

	result, err := awsclient.EnrichWAFLogging(context.Background(), clients, resources, nil)
	if err == nil {
		t.Fatal("enricher must surface a composite error when an API call fails")
	}
	// A check that issues several calls per resource names the check, never
	// one of its calls: any single call named there is the one that answered
	// as often as the one that failed. The type is not in the label either —
	// it comes from the registry key at the surface, and a type here would
	// render twice ("enrich waf: waf: ...").
	const wafAggregateOp = "web ACL logging and associations"
	if errStr := err.Error(); !strings.Contains(errStr, wafAggregateOp) {
		t.Errorf("composite error must name the check, %q, got: %q", wafAggregateOp, errStr)
	}
	if errStr := err.Error(); !strings.Contains(errStr, wafACLARN1) {
		t.Errorf("composite error must contain the failing WebACL ARN %q, got: %q", wafACLARN1, errStr)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings on API error, got %d", len(result.Findings))
	}
	if result.Truncated {
		t.Error("Truncated must stay false: waf only emits \"~\" findings, so an API error marks the row via TruncatedIDs, never the aggregate issue badge")
	}
	if _, marked := result.TruncatedIDs[wafACLARN1]; !marked {
		t.Errorf("TruncatedIDs[%q] must be true", wafACLARN1)
	}
	if _, marked := result.TruncatedIDs[wafACLARN2]; !marked {
		t.Errorf("TruncatedIDs[%q] must be true", wafACLARN2)
	}
}

func stringPtr(s string) *string { return &s }
