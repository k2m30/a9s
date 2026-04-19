package unit

// aws_enrichment_wave4_test.go — Deep-branch coverage for Wave-2 enrichers.
//
// Enrichers covered:
//   - EnrichWAFLogging      (internal/aws/enrichment.go:3460)
//   - EnrichLogsMetricFilters (internal/aws/enrichment.go:4111)
//   - EnrichEBSVolumeStatus  (internal/aws/enrichment.go:406)
//
// This file covers branches that were NOT covered by the existing test files
// (aws_waf_enricher_test.go, aws_logs_enricher_test.go, enrichment_ebs_findings_test.go).

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwlogssvc "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	wafv2svc "github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// =============================================================================
// WAF fakes — wave4
// =============================================================================

// wafFullFake implements WAFv2API AND WAFv2GetWebACLAPI so the enricher's
// type-assertion path for rules_summary is exercised.
type wafFullFake struct {
	awsclient.WAFv2API

	// GetLoggingConfiguration
	loggingResults  map[string]*wafv2svc.GetLoggingConfigurationOutput
	loggingErrByARN map[string]error

	// ListResourcesForWebACL
	resourcesResults  map[string]*wafv2svc.ListResourcesForWebACLOutput
	resourcesErrByARN map[string]error

	// GetWebACL
	getACLOutput *wafv2svc.GetWebACLOutput
	getACLErr    error
}

func (f *wafFullFake) GetLoggingConfiguration(
	_ context.Context,
	in *wafv2svc.GetLoggingConfigurationInput,
	_ ...func(*wafv2svc.Options),
) (*wafv2svc.GetLoggingConfigurationOutput, error) {
	arn := ""
	if in != nil && in.ResourceArn != nil {
		arn = *in.ResourceArn
	}
	if f.loggingErrByARN != nil {
		if err, ok := f.loggingErrByARN[arn]; ok {
			return nil, err
		}
	}
	if out, ok := f.loggingResults[arn]; ok {
		return out, nil
	}
	return &wafv2svc.GetLoggingConfigurationOutput{}, nil
}

func (f *wafFullFake) ListResourcesForWebACL(
	_ context.Context,
	in *wafv2svc.ListResourcesForWebACLInput,
	_ ...func(*wafv2svc.Options),
) (*wafv2svc.ListResourcesForWebACLOutput, error) {
	arn := ""
	if in != nil && in.WebACLArn != nil {
		arn = *in.WebACLArn
	}
	if f.resourcesErrByARN != nil {
		if err, ok := f.resourcesErrByARN[arn]; ok {
			return nil, err
		}
	}
	if out, ok := f.resourcesResults[arn]; ok {
		return out, nil
	}
	return &wafv2svc.ListResourcesForWebACLOutput{}, nil
}

// GetWebACL makes wafFullFake also satisfy WAFv2GetWebACLAPI.
func (f *wafFullFake) GetWebACL(
	_ context.Context,
	_ *wafv2svc.GetWebACLInput,
	_ ...func(*wafv2svc.Options),
) (*wafv2svc.GetWebACLOutput, error) {
	if f.getACLErr != nil {
		return nil, f.getACLErr
	}
	if f.getACLOutput != nil {
		return f.getACLOutput, nil
	}
	return &wafv2svc.GetWebACLOutput{}, nil
}

// Compile-time check: wafFullFake satisfies WAFv2API.
var _ awsclient.WAFv2API = (*wafFullFake)(nil)

// Compile-time check: wafFullFake satisfies WAFv2GetWebACLAPI.
var _ awsclient.WAFv2GetWebACLAPI = (*wafFullFake)(nil)

// wafWebACLResourceWithNameID builds a WAF resource stub with name, id, arn, and
// optional scope fields — required for the GetWebACL path to be reached.
func wafWebACLResourceWithNameID(arn, name, id, scope string) resource.Resource {
	fields := map[string]string{
		"arn":  arn,
		"name": name,
		"id":   id,
	}
	if scope != "" {
		fields["scope"] = scope
	}
	return resource.Resource{
		ID:     arn,
		Name:   name,
		Fields: fields,
	}
}

// =============================================================================
// EnrichWAFLogging — GetWebACL type-assertion path (rules_summary with blocks)
// =============================================================================

// TestEnrichWAFLogging_GetWebACLPathPopulatesBlockRulesSummary verifies that when
// the WAFv2 client also implements WAFv2GetWebACLAPI and the resource has name+id
// fields, the rules_summary FieldUpdate is populated with the block count format
// "N/T BLOCK" rather than the default "0 rules".
// Covers enrichment.go:3522-3545 (GetWebACL type-assertion branch).
func TestEnrichWAFLogging_GetWebACLPathPopulatesBlockRulesSummary(t *testing.T) {
	const arn = "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/prod-acl/aaaa0001"
	const name = "prod-acl"
	const id = "aaaa0001"

	fake := &wafFullFake{
		loggingResults: map[string]*wafv2svc.GetLoggingConfigurationOutput{
			arn: {
				LoggingConfiguration: &wafv2types.LoggingConfiguration{
					ResourceArn:           aws.String(arn),
					LogDestinationConfigs: []string{"arn:aws:logs:us-east-1:123456789012:log-group:aws-waf-logs-prod"},
				},
			},
		},
		resourcesResults: map[string]*wafv2svc.ListResourcesForWebACLOutput{
			arn: {ResourceArns: []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/prod/abc"}},
		},
		getACLOutput: &wafv2svc.GetWebACLOutput{
			WebACL: &wafv2types.WebACL{
				Rules: []wafv2types.Rule{
					{
						Name:   aws.String("BlockBadBots"),
						Action: &wafv2types.RuleAction{Block: &wafv2types.BlockAction{}},
					},
					{
						Name:   aws.String("AllowGoodBots"),
						Action: &wafv2types.RuleAction{Allow: &wafv2types.AllowAction{}},
					},
					{
						Name:   aws.String("RateLimit"),
						Action: &wafv2types.RuleAction{Block: &wafv2types.BlockAction{}},
					},
				},
			},
		},
	}

	clients := &awsclient.ServiceClients{WAFv2: fake}
	resources := []resource.Resource{wafWebACLResourceWithNameID(arn, name, id, "")}

	result, err := awsclient.EnrichWAFLogging(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fu, ok := result.FieldUpdates[arn]
	if !ok {
		t.Fatalf("FieldUpdates missing entry for %q", arn)
	}
	rulesSummary := fu["rules_summary"]
	// Expected: "2/3 BLOCK"
	if !strings.Contains(rulesSummary, "BLOCK") {
		t.Errorf("rules_summary = %q, expected to contain \"BLOCK\" (type-assertion path)", rulesSummary)
	}
	if !strings.Contains(rulesSummary, "2") {
		t.Errorf("rules_summary = %q, expected block count 2", rulesSummary)
	}
	if !strings.Contains(rulesSummary, "3") {
		t.Errorf("rules_summary = %q, expected total count 3", rulesSummary)
	}
	// No findings expected — logging is configured and ACL is associated.
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result.Findings))
	}
}

// TestEnrichWAFLogging_EmptyScopeDefaultsToREGIONAL verifies that when r.Fields["scope"]
// is empty the enricher uses "REGIONAL" as the default scope when calling GetWebACL.
// This is a behavior assertion: the call must succeed (non-error output) which means
// the scope string was accepted. Covers enrichment.go:3523-3526.
func TestEnrichWAFLogging_EmptyScopeDefaultsToREGIONAL(t *testing.T) {
	const arn = "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/scope-test/bbbb0002"
	const name = "scope-test"
	const id = "bbbb0002"

	getACLCalled := false
	fake := &wafFullFake{
		loggingResults: map[string]*wafv2svc.GetLoggingConfigurationOutput{
			arn: {LoggingConfiguration: &wafv2types.LoggingConfiguration{ResourceArn: aws.String(arn)}},
		},
		resourcesResults: map[string]*wafv2svc.ListResourcesForWebACLOutput{
			arn: {ResourceArns: []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/x/y"}},
		},
		getACLOutput: &wafv2svc.GetWebACLOutput{
			WebACL: &wafv2types.WebACL{Rules: []wafv2types.Rule{}},
		},
	}
	// Wrap to detect the call and verify Scope field.
	wrapped := &scopeCaptureFake{wafFullFake: fake, scopeCapture: &getACLCalled}
	clients := &awsclient.ServiceClients{WAFv2: wrapped}
	// Resource has NO "scope" field — should default to REGIONAL.
	resources := []resource.Resource{wafWebACLResourceWithNameID(arn, name, id, "")}

	result, err := awsclient.EnrichWAFLogging(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// rules_summary should be "0 rules" (no rules in the WebACL).
	fu := result.FieldUpdates[arn]
	if fu["rules_summary"] != "0 rules" {
		t.Errorf("rules_summary = %q, want %q (empty WebACL via REGIONAL scope)", fu["rules_summary"], "0 rules")
	}
	if !getACLCalled {
		t.Error("GetWebACL was not called — scope-default branch not reached")
	}
	if wrapped.capturedScope != string(wafv2types.ScopeRegional) {
		t.Errorf("scope passed to GetWebACL = %q, want %q", wrapped.capturedScope, string(wafv2types.ScopeRegional))
	}
}

// scopeCaptureFake wraps wafFullFake and records the Scope argument passed to GetWebACL.
type scopeCaptureFake struct {
	*wafFullFake
	scopeCapture  *bool
	capturedScope string
}

func (f *scopeCaptureFake) GetWebACL(
	ctx context.Context,
	in *wafv2svc.GetWebACLInput,
	opts ...func(*wafv2svc.Options),
) (*wafv2svc.GetWebACLOutput, error) {
	*f.scopeCapture = true
	if in != nil {
		f.capturedScope = string(in.Scope)
	}
	return f.wafFullFake.GetWebACL(ctx, in, opts...)
}

// Compile-time: scopeCaptureFake satisfies WAFv2API and WAFv2GetWebACLAPI.
var _ awsclient.WAFv2API = (*scopeCaptureFake)(nil)
var _ awsclient.WAFv2GetWebACLAPI = (*scopeCaptureFake)(nil)

// TestEnrichWAFLogging_ListResourcesErrorSetsTruncatedNoFindings verifies that when
// ListResourcesForWebACL returns an error, the enricher sets Truncated=true and
// TruncatedIDs[ID]=true, then continues to the next resource without a finding.
// Covers enrichment.go:3505-3509 (ListResourcesForWebACL error branch).
func TestEnrichWAFLogging_ListResourcesErrorSetsTruncatedNoFindings(t *testing.T) {
	assocErr := errors.New("wafv2: ListResourcesForWebACL throttled")
	fake := &wafLoggingFake{
		loggingResults: map[string]*wafv2svc.GetLoggingConfigurationOutput{
			wafACLARN1: {LoggingConfiguration: &wafv2types.LoggingConfiguration{ResourceArn: aws.String(wafACLARN1)}},
			wafACLARN2: {LoggingConfiguration: &wafv2types.LoggingConfiguration{ResourceArn: aws.String(wafACLARN2)}},
		},
		resourcesErrByARN: map[string]error{
			wafACLARN1: assocErr,
			wafACLARN2: assocErr,
		},
	}
	clients := &awsclient.ServiceClients{WAFv2: fake}
	resources := wafWebACLResources(wafACLARN1, wafACLARN2)

	result, err := awsclient.EnrichWAFLogging(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings on ListResourcesForWebACL error, got %d", len(result.Findings))
	}
	if !result.Truncated {
		t.Error("Truncated must be true when ListResourcesForWebACL fails")
	}
	if !result.TruncatedIDs[wafACLARN1] {
		t.Errorf("TruncatedIDs[%q] must be true", wafACLARN1)
	}
	if !result.TruncatedIDs[wafACLARN2] {
		t.Errorf("TruncatedIDs[%q] must be true", wafACLARN2)
	}
}

// TestEnrichWAFLogging_TypeAssertionFailsRulesSummaryZero verifies that when the
// WAFv2 client does NOT implement WAFv2GetWebACLAPI (type assertion fails at
// enrichment.go:3522), rules_summary defaults to "0 rules" without an error.
// This is the wafLoggingFake path — it only implements WAFv2API, not WAFv2GetWebACLAPI.
func TestEnrichWAFLogging_TypeAssertionFailsRulesSummaryZero(t *testing.T) {
	// wafLoggingFake (defined in aws_waf_enricher_test.go) does NOT implement GetWebACL.
	fake := &wafLoggingFake{
		loggingResults: map[string]*wafv2svc.GetLoggingConfigurationOutput{
			wafACLARN1: {LoggingConfiguration: &wafv2types.LoggingConfiguration{ResourceArn: aws.String(wafACLARN1)}},
		},
		resourcesResults: map[string]*wafv2svc.ListResourcesForWebACLOutput{
			wafACLARN1: {ResourceArns: []string{"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/x/y"}},
		},
	}
	// Resource must have name+id for the type-assertion gate to be reached.
	resources := []resource.Resource{
		{
			ID:   wafACLARN1,
			Name: "webacl-1",
			Fields: map[string]string{
				"arn":  wafACLARN1,
				"name": "webacl-1",
				"id":   "aaaabbbb-1111-2222-3333-444444444444",
			},
		},
	}
	clients := &awsclient.ServiceClients{WAFv2: fake}

	result, err := awsclient.EnrichWAFLogging(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fu := result.FieldUpdates[wafACLARN1]
	if fu == nil {
		t.Fatalf("FieldUpdates[%q] must not be nil", wafACLARN1)
	}
	if fu["rules_summary"] != "0 rules" {
		t.Errorf("rules_summary = %q, want %q (type assertion fails — no GetWebACL)", fu["rules_summary"], "0 rules")
	}
}

// =============================================================================
// EnrichLogsMetricFilters — deep branch tests
// =============================================================================

// cwLogsFullFake implements CWLogsAPI including DescribeLogStreams.
// Allows wave4 tests to exercise the hasStreams=true path and time-bucket logic.
type cwLogsFullFake struct {
	awsclient.CWLogsAPI

	// DescribeMetricFilters
	filtersByGroup   map[string][]cwlogstypes.MetricFilter
	metricFiltersErr error

	// DescribeLogStreams
	streamsByGroup map[string]*cwlogssvc.DescribeLogStreamsOutput
	streamsErr     error
}

func (f *cwLogsFullFake) DescribeMetricFilters(
	_ context.Context,
	in *cwlogssvc.DescribeMetricFiltersInput,
	_ ...func(*cwlogssvc.Options),
) (*cwlogssvc.DescribeMetricFiltersOutput, error) {
	if f.metricFiltersErr != nil {
		return nil, f.metricFiltersErr
	}
	name := ""
	if in != nil && in.LogGroupName != nil {
		name = *in.LogGroupName
	}
	filters := f.filtersByGroup[name]
	return &cwlogssvc.DescribeMetricFiltersOutput{MetricFilters: filters}, nil
}

func (f *cwLogsFullFake) DescribeLogStreams(
	_ context.Context,
	in *cwlogssvc.DescribeLogStreamsInput,
	_ ...func(*cwlogssvc.Options),
) (*cwlogssvc.DescribeLogStreamsOutput, error) {
	if f.streamsErr != nil {
		return nil, f.streamsErr
	}
	name := ""
	if in != nil && in.LogGroupName != nil {
		name = *in.LogGroupName
	}
	if out, ok := f.streamsByGroup[name]; ok {
		return out, nil
	}
	return &cwlogssvc.DescribeLogStreamsOutput{}, nil
}

// Compile-time checks: cwLogsFullFake satisfies both CWLogsAPI interfaces.
var _ awsclient.CWLogsAPI = (*cwLogsFullFake)(nil)
var _ awsclient.CWLogsDescribeLogStreamsAPI = (*cwLogsFullFake)(nil)
var _ awsclient.CWLogsDescribeMetricFiltersAPI = (*cwLogsFullFake)(nil)

// cwLogsNoStreamsFake embeds CWLogsAPI as nil so the DescribeLogStreams
// type assertion in EnrichLogsMetricFilters FAILS (hasStreams=false).
type cwLogsNoStreamsFake struct {
	awsclient.CWLogsAPI // nil embedded — causes panic on any non-overridden call

	filtersByGroup map[string][]cwlogstypes.MetricFilter
}

func (f *cwLogsNoStreamsFake) DescribeMetricFilters(
	_ context.Context,
	in *cwlogssvc.DescribeMetricFiltersInput,
	_ ...func(*cwlogssvc.Options),
) (*cwlogssvc.DescribeMetricFiltersOutput, error) {
	name := ""
	if in != nil && in.LogGroupName != nil {
		name = *in.LogGroupName
	}
	return &cwlogssvc.DescribeMetricFiltersOutput{MetricFilters: f.filtersByGroup[name]}, nil
}

// Compile-time check: cwLogsNoStreamsFake satisfies CWLogsAPI.
var _ awsclient.CWLogsAPI = (*cwLogsNoStreamsFake)(nil)

// TestEnrichLogsMetricFilters_MetricFiltersAPIAssertionFailsReturnsEmpty verifies
// that when clients.CloudWatchLogs does NOT implement CWLogsDescribeMetricFiltersAPI
// (type assertion at enrichment.go:4118 fails), the enricher returns empty results
// without error.
// We simulate this by using a fake that only implements CWLogsDescribeLogStreamsAPI
// (not CWLogsDescribeMetricFiltersAPI), so the enricher cannot cast it.
func TestEnrichLogsMetricFilters_MetricFiltersAPIAssertionFailsReturnsEmpty(t *testing.T) {
	// cwLogsStreamsOnlyFake implements CWLogsAPI via embedding but does NOT
	// override DescribeMetricFilters — so the type assertion
	// clients.CloudWatchLogs.(CWLogsDescribeMetricFiltersAPI) must succeed because
	// the embedded awsclient.CWLogsAPI interface includes it. To get the assertion
	// to FAIL we need a value that is CWLogsAPI-assignable but does NOT implement
	// CWLogsDescribeMetricFiltersAPI.
	//
	// The only way to get hasMetricFilters=false is to have clients.CloudWatchLogs
	// be a type that implements CWLogsAPI (so the nil-check passes) but does NOT
	// implement CWLogsDescribeMetricFiltersAPI. This is impossible in Go when
	// CWLogsAPI embeds CWLogsDescribeMetricFiltersAPI. Therefore we test the
	// nearest achievable guard: the nil-client guard (already covered in the
	// existing test file). This test is a documentation stub confirming the
	// assertion always succeeds for any valid CWLogsAPI implementation.
	//
	// SKIP — assertion cannot fail for any valid CWLogsAPI value because
	// CWLogsAPI embeds CWLogsDescribeMetricFiltersAPI. The nil guard (line 4115)
	// is the only reachable guard; it is already covered by
	// TestEnrichLogsMetricFilters_NilClientReturnsEmptyFindingsNoError.
	t.Skip("CWLogsAPI embeds CWLogsDescribeMetricFiltersAPI — assertion always succeeds for valid clients; nil guard already covered")
}

// TestEnrichLogsMetricFilters_NoStreamsAPISkipsLastEventAt verifies that when
// clients.CloudWatchLogs does NOT implement CWLogsDescribeLogStreamsAPI
// (hasStreams=false at enrichment.go:4125), the last_event_at field is not
// populated in FieldUpdates. Covers enrichment.go:4141 (hasStreams guard).
func TestEnrichLogsMetricFilters_NoStreamsAPISkipsLastEventAt(t *testing.T) {
	// cwLogsNoStreamsFake embeds awsclient.CWLogsAPI as a nil value.
	// The dynamic type is *cwLogsNoStreamsFake which does NOT have a DescribeLogStreams
	// method, so the type assertion .(CWLogsDescribeLogStreamsAPI) fails → hasStreams=false.
	auditGroup := "/aws/cloudtrail/no-streams"
	fake := &cwLogsNoStreamsFake{
		filtersByGroup: map[string][]cwlogstypes.MetricFilter{
			auditGroup: {cwMetricFilter(auditGroup, "SomeFilter")},
		},
	}
	clients := &awsclient.ServiceClients{CloudWatchLogs: fake}
	resources := []resource.Resource{logsGroupResource(auditGroup)}

	result, err := awsclient.EnrichLogsMetricFilters(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No findings (audit group has filters).
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result.Findings))
	}
	// last_event_at must NOT be set when DescribeLogStreams is unavailable.
	if fu := result.FieldUpdates[auditGroup]; fu != nil {
		if _, ok := fu["last_event_at"]; ok {
			t.Errorf("last_event_at must not be set when hasStreams=false, got %q", fu["last_event_at"])
		}
	}
}

// TestEnrichLogsMetricFilters_StreamsErrorNoLastEventAt verifies that when
// DescribeLogStreams returns an error the enricher continues without setting
// last_event_at (no truncation for log-stream errors).
// Covers enrichment.go:4143 (streamsErr != nil path via safeDescribeLogStreams).
func TestEnrichLogsMetricFilters_StreamsErrorNoLastEventAt(t *testing.T) {
	auditGroup := "/aws/cloudtrail/streams-err"
	fake := &cwLogsFullFake{
		filtersByGroup: map[string][]cwlogstypes.MetricFilter{
			auditGroup: {cwMetricFilter(auditGroup, "SomeFilter")},
		},
		streamsErr: errors.New("cloudwatchlogs: DescribeLogStreams throttled"),
	}
	clients := &awsclient.ServiceClients{CloudWatchLogs: fake}
	resources := []resource.Resource{logsGroupResource(auditGroup)}

	result, err := awsclient.EnrichLogsMetricFilters(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(result.Findings))
	}
	// No truncation for log-stream errors — the enricher only truncates on DescribeMetricFilters errors.
	if result.Truncated {
		t.Error("Truncated must be false for DescribeLogStreams errors — only DescribeMetricFilters triggers truncation")
	}
	// last_event_at must NOT be set when DescribeLogStreams errors.
	if fu := result.FieldUpdates[auditGroup]; fu != nil {
		if _, ok := fu["last_event_at"]; ok {
			t.Errorf("last_event_at must not be set when DescribeLogStreams errors, got %q", fu["last_event_at"])
		}
	}
}

// TestEnrichLogsMetricFilters_DescribeMetricFiltersErrorSetsTruncated verifies
// that when DescribeMetricFilters returns an error the enricher sets
// Truncated=true and TruncatedIDs[ID]=true.
// Covers enrichment.go:4175-4178 (DescribeMetricFilters error branch).
func TestEnrichLogsMetricFilters_DescribeMetricFiltersErrorSetsTruncated(t *testing.T) {
	auditGroup := "/aws/cloudtrail/mf-err"
	fake := &cwLogsFullFake{
		metricFiltersErr: errors.New("cloudwatchlogs: DescribeMetricFilters throttled"),
	}
	clients := &awsclient.ServiceClients{CloudWatchLogs: fake}
	resources := []resource.Resource{logsGroupResource(auditGroup)}

	result, err := awsclient.EnrichLogsMetricFilters(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings on DescribeMetricFilters error, got %d", len(result.Findings))
	}
	if !result.Truncated {
		t.Error("Truncated must be true when DescribeMetricFilters fails")
	}
	if !result.TruncatedIDs[auditGroup] {
		t.Errorf("TruncatedIDs[%q] must be true", auditGroup)
	}
}

// TestEnrichLogsMetricFilters_LastEventAt_MinutesAgo verifies that the last_event_at
// FieldUpdate is formatted as "%dm ago" when the most-recent stream event is < 1 hour ago.
// Covers enrichment.go:4150-4151 (dur < time.Hour branch).
func TestEnrichLogsMetricFilters_LastEventAt_MinutesAgo(t *testing.T) {
	logGroup := "/aws/lambda/recent-func"
	tsMillis := time.Now().Add(-25 * time.Minute).UnixMilli()
	fake := &cwLogsFullFake{
		streamsByGroup: map[string]*cwlogssvc.DescribeLogStreamsOutput{
			logGroup: {
				LogStreams: []cwlogstypes.LogStream{
					{LastEventTimestamp: aws.Int64(tsMillis)},
				},
			},
		},
		filtersByGroup: map[string][]cwlogstypes.MetricFilter{},
	}
	clients := &awsclient.ServiceClients{CloudWatchLogs: fake}
	resources := []resource.Resource{logsGroupResource(logGroup)}

	result, err := awsclient.EnrichLogsMetricFilters(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fu := result.FieldUpdates[logGroup]
	if fu == nil {
		t.Fatalf("FieldUpdates[%q] must not be nil", logGroup)
	}
	lastEventAt := fu["last_event_at"]
	if !strings.HasSuffix(lastEventAt, "m ago") {
		t.Errorf("last_event_at = %q, want format \"%%dm ago\" (< 1 hour)", lastEventAt)
	}
}

// TestEnrichLogsMetricFilters_LastEventAt_HoursAgo verifies that the last_event_at
// FieldUpdate is formatted as "%dh ago" when the most-recent stream event is ≥1h but
// < 24h ago. Covers enrichment.go:4152-4153 (dur < 24*time.Hour branch).
func TestEnrichLogsMetricFilters_LastEventAt_HoursAgo(t *testing.T) {
	logGroup := "/aws/lambda/hours-ago-func"
	tsMillis := time.Now().Add(-5 * time.Hour).UnixMilli()
	fake := &cwLogsFullFake{
		streamsByGroup: map[string]*cwlogssvc.DescribeLogStreamsOutput{
			logGroup: {
				LogStreams: []cwlogstypes.LogStream{
					{LastEventTimestamp: aws.Int64(tsMillis)},
				},
			},
		},
		filtersByGroup: map[string][]cwlogstypes.MetricFilter{},
	}
	clients := &awsclient.ServiceClients{CloudWatchLogs: fake}
	resources := []resource.Resource{logsGroupResource(logGroup)}

	result, err := awsclient.EnrichLogsMetricFilters(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fu := result.FieldUpdates[logGroup]
	if fu == nil {
		t.Fatalf("FieldUpdates[%q] must not be nil", logGroup)
	}
	lastEventAt := fu["last_event_at"]
	if !strings.HasSuffix(lastEventAt, "h ago") {
		t.Errorf("last_event_at = %q, want format \"%%dh ago\" (1h–24h)", lastEventAt)
	}
}

// TestEnrichLogsMetricFilters_LastEventAt_DaysAgo verifies that the last_event_at
// FieldUpdate is formatted as "%dd ago" when the most-recent stream event is ≥1d but
// < 7d ago. Covers enrichment.go:4154-4155 (dur < 7*24*time.Hour branch).
func TestEnrichLogsMetricFilters_LastEventAt_DaysAgo(t *testing.T) {
	logGroup := "/aws/lambda/days-ago-func"
	tsMillis := time.Now().Add(-3 * 24 * time.Hour).UnixMilli()
	fake := &cwLogsFullFake{
		streamsByGroup: map[string]*cwlogssvc.DescribeLogStreamsOutput{
			logGroup: {
				LogStreams: []cwlogstypes.LogStream{
					{LastEventTimestamp: aws.Int64(tsMillis)},
				},
			},
		},
		filtersByGroup: map[string][]cwlogstypes.MetricFilter{},
	}
	clients := &awsclient.ServiceClients{CloudWatchLogs: fake}
	resources := []resource.Resource{logsGroupResource(logGroup)}

	result, err := awsclient.EnrichLogsMetricFilters(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fu := result.FieldUpdates[logGroup]
	if fu == nil {
		t.Fatalf("FieldUpdates[%q] must not be nil", logGroup)
	}
	lastEventAt := fu["last_event_at"]
	if !strings.HasSuffix(lastEventAt, "d ago") {
		t.Errorf("last_event_at = %q, want format \"%%dd ago\" (1d–7d)", lastEventAt)
	}
}

// TestEnrichLogsMetricFilters_LastEventAt_DateFormat verifies that the last_event_at
// FieldUpdate is formatted as "YYYY-MM-DD" when the most-recent stream event is ≥7d ago.
// Covers enrichment.go:4156-4157 (default/date-format branch).
func TestEnrichLogsMetricFilters_LastEventAt_DateFormat(t *testing.T) {
	logGroup := "/aws/lambda/old-func"
	tsMillis := time.Now().Add(-10 * 24 * time.Hour).UnixMilli()
	fake := &cwLogsFullFake{
		streamsByGroup: map[string]*cwlogssvc.DescribeLogStreamsOutput{
			logGroup: {
				LogStreams: []cwlogstypes.LogStream{
					{LastEventTimestamp: aws.Int64(tsMillis)},
				},
			},
		},
		filtersByGroup: map[string][]cwlogstypes.MetricFilter{},
	}
	clients := &awsclient.ServiceClients{CloudWatchLogs: fake}
	resources := []resource.Resource{logsGroupResource(logGroup)}

	result, err := awsclient.EnrichLogsMetricFilters(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fu := result.FieldUpdates[logGroup]
	if fu == nil {
		t.Fatalf("FieldUpdates[%q] must not be nil", logGroup)
	}
	lastEventAt := fu["last_event_at"]
	// Must match "YYYY-MM-DD" (10 chars, digits and dashes, no "ago").
	if len(lastEventAt) != 10 || strings.Contains(lastEventAt, "ago") {
		t.Errorf("last_event_at = %q, want \"YYYY-MM-DD\" date format (>=7d ago)", lastEventAt)
	}
	if lastEventAt[4] != '-' || lastEventAt[7] != '-' {
		t.Errorf("last_event_at = %q, expected format 2006-01-02", lastEventAt)
	}
}

// =============================================================================
// EnrichEBSVolumeStatus — nil EC2 client guard
// =============================================================================

// TestEnrichEBSVolumeStatus_NilEC2ClientReturnsEmptyNoError verifies that when
// clients.EC2 is nil the enricher returns a non-nil empty Findings map and no error.
// Covers enrichment.go:409-411 (EC2 nil guard).
func TestEnrichEBSVolumeStatus_NilEC2ClientReturnsEmptyNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{EC2: nil}

	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when EC2 client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
	if result.Truncated {
		t.Error("Truncated must be false when EC2 client is nil")
	}
}

// TestEnrichEBSVolumeStatus_APIErrorReturnsError verifies that when DescribeVolumeStatus
// returns an error the enricher propagates the error (not just truncation).
// Covers enrichment.go:432-434 (API error → return err branch).
func TestEnrichEBSVolumeStatus_APIErrorReturnsError(t *testing.T) {
	apiErr := errors.New("ec2: DescribeVolumeStatus throttled")
	fake := &ebsStatusFake{volumeErr: apiErr}
	clients := &awsclient.ServiceClients{EC2: fake}

	_, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, nil)
	if err == nil {
		t.Fatal("expected error from DescribeVolumeStatus, got nil")
	}
	if !strings.Contains(err.Error(), "throttled") {
		t.Errorf("error = %q, expected it to contain %q", err.Error(), "throttled")
	}
}

// TestEnrichEBSVolumeStatus_WarningStatusProducesFinding verifies that a volume
// with "warning" status produces a "!" finding with summary "volume I/O degraded".
// Covers enrichment.go:450-482 (non-ok, non-impaired status path for "warning").
func TestEnrichEBSVolumeStatus_WarningStatusProducesFinding(t *testing.T) {
	out := &ec2svc.DescribeVolumeStatusOutput{
		VolumeStatuses: []ec2types.VolumeStatusItem{
			{
				VolumeId:     aws.String("vol-warn"),
				VolumeStatus: &ec2types.VolumeStatusInfo{Status: "warning"},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: &ebsStatusFake{volumeOutput: out}}

	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["vol-warn"]
	if !ok {
		t.Fatalf("expected finding for volume with status 'warning'")
	}
	if f.Severity != "!" {
		t.Errorf("severity = %q, want %q", f.Severity, "!")
	}
	if f.Summary != "volume I/O degraded" {
		t.Errorf("summary = %q, want %q", f.Summary, "volume I/O degraded")
	}
	// The I/O State row must reflect the actual status string.
	if len(f.Rows) == 0 {
		t.Fatal("expected at least one finding row")
	}
	if f.Rows[0].Value != "warning" {
		t.Errorf("I/O State row value = %q, want %q", f.Rows[0].Value, "warning")
	}
}

// TestEnrichEBSVolumeStatus_EventAndActionRowsPopulated verifies that when a
// non-ok volume has Events and Actions, the finding rows include "Event" and
// "Action Code" entries.
// Covers enrichment.go:458-476 (event and action row branches).
func TestEnrichEBSVolumeStatus_EventAndActionRowsPopulated(t *testing.T) {
	out := &ec2svc.DescribeVolumeStatusOutput{
		VolumeStatuses: []ec2types.VolumeStatusItem{
			{
				VolumeId:     aws.String("vol-events"),
				VolumeStatus: &ec2types.VolumeStatusInfo{Status: "impaired"},
				Events: []ec2types.VolumeStatusEvent{
					{
						EventType:   aws.String("io-performance:degraded"),
						Description: aws.String("I/O throughput is degraded due to hardware issue"),
					},
				},
				Actions: []ec2types.VolumeStatusAction{
					{Code: aws.String("enable-volume-io")},
				},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: &ebsStatusFake{volumeOutput: out}}

	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings["vol-events"]
	if !ok {
		t.Fatalf("expected finding for impaired volume with events/actions")
	}

	hasEvent := false
	hasAction := false
	for _, row := range f.Rows {
		if row.Label == "Event" {
			hasEvent = true
			if !strings.Contains(row.Value, "degraded") {
				t.Errorf("Event row value = %q, expected to contain %q", row.Value, "degraded")
			}
		}
		if row.Label == "Action Code" {
			hasAction = true
			if row.Value != "enable-volume-io" {
				t.Errorf("Action Code row value = %q, want %q", row.Value, "enable-volume-io")
			}
		}
	}
	if !hasEvent {
		t.Error("finding rows must contain an 'Event' row")
	}
	if !hasAction {
		t.Error("finding rows must contain an 'Action Code' row")
	}
}

// TestEnrichEBSVolumeStatus_KnownIDsFilterExcludesUnmatchedVolumes verifies that
// when a non-empty resource list is provided, volumes NOT in that list are excluded
// from findings even if the API returns them.
// Covers enrichment.go:447-449 (knownIDs filter branch).
func TestEnrichEBSVolumeStatus_KnownIDsFilterExcludesUnmatchedVolumes(t *testing.T) {
	out := &ec2svc.DescribeVolumeStatusOutput{
		VolumeStatuses: []ec2types.VolumeStatusItem{
			// in-scope: vol-known
			{
				VolumeId:     aws.String("vol-known"),
				VolumeStatus: &ec2types.VolumeStatusInfo{Status: "impaired"},
			},
			// out-of-scope: vol-foreign (API returned it but it's not in our resources)
			{
				VolumeId:     aws.String("vol-foreign"),
				VolumeStatus: &ec2types.VolumeStatusInfo{Status: "impaired"},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: &ebsStatusFake{volumeOutput: out}}
	// Only vol-known is in the resource list.
	resources := []resource.Resource{
		{ID: "vol-known", Name: "known-volume"},
	}

	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Findings["vol-known"]; !ok {
		t.Error("vol-known must appear in Findings")
	}
	if _, ok := result.Findings["vol-foreign"]; ok {
		t.Error("vol-foreign must NOT appear in Findings (not in input resource list)")
	}
}

// TestEnrichEBSVolumeStatus_NilVolumeIdSkipped verifies that a VolumeStatusItem
// with a nil VolumeId is silently skipped without a panic or error.
// Covers enrichment.go:442-444 (nil VolumeId guard).
func TestEnrichEBSVolumeStatus_NilVolumeIdSkipped(t *testing.T) {
	out := &ec2svc.DescribeVolumeStatusOutput{
		VolumeStatuses: []ec2types.VolumeStatusItem{
			{
				VolumeId:     nil, // nil — must be skipped
				VolumeStatus: &ec2types.VolumeStatusInfo{Status: "impaired"},
			},
			{
				VolumeId:     aws.String("vol-real"),
				VolumeStatus: &ec2types.VolumeStatusInfo{Status: "impaired"},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: &ebsStatusFake{volumeOutput: out}}

	result, err := awsclient.EnrichEBSVolumeStatus(context.Background(), clients, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only vol-real should appear.
	if _, ok := result.Findings["vol-real"]; !ok {
		t.Error("vol-real must appear in Findings")
	}
	if result.IssueCount != 1 {
		t.Errorf("IssueCount = %d, want 1 (nil VolumeId entry must be skipped)", result.IssueCount)
	}
}
