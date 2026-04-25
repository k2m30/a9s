package unit

// aws_logs_enricher_test.go — Behavioral tests for EnrichLogsMetricFilters.
//
// Contract assertions:
//   - Audit log group (name matches /aws/cloudtrail/) + DescribeMetricFilters returns ≥1 filter → 0 findings.
//   - Audit log group + DescribeMetricFilters returns [] → 1 finding sev "~".
//   - Non-audit log group (e.g. /aws/lambda/foo) → 0 findings (skipped).
//   - clients.CloudWatchLogs == nil → 0 findings, no error.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// cwLogsMetricFilterFake implements CWLogsAPI for logs enrichment testing.
// It embeds the interface and overrides only DescribeMetricFilters.
// The results map is keyed by LogGroupName.
type cwLogsMetricFilterFake struct {
	awsclient.CWLogsAPI

	// filtersByGroup maps LogGroupName → slice of MetricFilter.
	filtersByGroup map[string][]cwlogstypes.MetricFilter
}

func (f *cwLogsMetricFilterFake) DescribeMetricFilters(
	_ context.Context,
	in *cloudwatchlogs.DescribeMetricFiltersInput,
	_ ...func(*cloudwatchlogs.Options),
) (*cloudwatchlogs.DescribeMetricFiltersOutput, error) {
	name := ""
	if in != nil && in.LogGroupName != nil {
		name = *in.LogGroupName
	}
	filters := f.filtersByGroup[name]
	return &cloudwatchlogs.DescribeMetricFiltersOutput{MetricFilters: filters}, nil
}

// Compile-time check: cwLogsMetricFilterFake satisfies CWLogsAPI.
var _ awsclient.CWLogsAPI = (*cwLogsMetricFilterFake)(nil)

// logsGroupResource builds a logs Resource stub for the given log group name.
func logsGroupResource(logGroupName string) resource.Resource {
	return resource.Resource{
		ID:   logGroupName,
		Name: logGroupName,
		Fields: map[string]string{
			"log_group_name": logGroupName,
			"stored_bytes":   "4096",
			"retention_days": "30",
			"creation_time":  "2025-01-01 00:00",
		},
	}
}

// cwMetricFilter builds a minimal MetricFilter for the given log group.
func cwMetricFilter(logGroupName, filterName string) cwlogstypes.MetricFilter {
	return cwlogstypes.MetricFilter{
		LogGroupName:  aws.String(logGroupName),
		FilterName:    aws.String(filterName),
		FilterPattern: aws.String("{ $.eventName = \"ConsoleLogin\" }"),
	}
}

// TestEnrichLogsMetricFilters_AuditWithFiltersProducesNoFindings verifies that an
// audit log group with at least one metric filter produces no findings.
func TestEnrichLogsMetricFilters_AuditWithFiltersProducesNoFindings(t *testing.T) {
	auditGroup := "/aws/cloudtrail/audit"
	lambdaGroup := "/aws/lambda/my-function"
	fake := &cwLogsMetricFilterFake{
		filtersByGroup: map[string][]cwlogstypes.MetricFilter{
			auditGroup: {cwMetricFilter(auditGroup, "ConsoleLoginFilter")},
		},
	}
	clients := &awsclient.ServiceClients{CloudWatchLogs: fake}
	resources := []resource.Resource{
		logsGroupResource(auditGroup),
		logsGroupResource(lambdaGroup),
	}

	result, err := awsclient.EnrichLogsMetricFilters(context.Background(), clients, resources, nil)
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

// TestEnrichLogsMetricFilters_AuditNoFiltersProducesFindingSevTilde verifies that
// an audit log group with no metric filters produces a finding with severity "~".
func TestEnrichLogsMetricFilters_AuditNoFiltersProducesFindingSevTilde(t *testing.T) {
	auditGroup := "/aws/cloudtrail/audit"
	lambdaGroup := "/aws/lambda/my-function"
	fake := &cwLogsMetricFilterFake{
		filtersByGroup: map[string][]cwlogstypes.MetricFilter{
			auditGroup: {}, // no filters
		},
	}
	clients := &awsclient.ServiceClients{CloudWatchLogs: fake}
	resources := []resource.Resource{
		logsGroupResource(auditGroup),
		logsGroupResource(lambdaGroup),
	}

	result, err := awsclient.EnrichLogsMetricFilters(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, ok := result.Findings[auditGroup]
	if !ok {
		t.Fatalf("expected finding keyed by %q (no metric filters)", auditGroup)
	}
	if f.Severity != "~" {
		t.Errorf("severity = %q, want %q", f.Severity, "~")
	}
	if _, ok := result.Findings[lambdaGroup]; ok {
		t.Error("lambda group must NOT appear in Findings — non-audit groups are skipped")
	}
	// "~" findings do not contribute to IssueCount.
	if result.IssueCount != 0 {
		t.Errorf("IssueCount = %d, want 0 (sev ~ does not count)", result.IssueCount)
	}
}

// TestEnrichLogsMetricFilters_NonAuditGroupSkipped verifies that a non-audit log
// group (e.g. /aws/lambda/foo) produces no finding regardless of metric filter state.
func TestEnrichLogsMetricFilters_NonAuditGroupSkipped(t *testing.T) {
	lambdaGroup := "/aws/lambda/my-function"
	fake := &cwLogsMetricFilterFake{
		filtersByGroup: map[string][]cwlogstypes.MetricFilter{
			// no filters for lambda group — but it should be skipped anyway
		},
	}
	clients := &awsclient.ServiceClients{CloudWatchLogs: fake}
	resources := []resource.Resource{
		logsGroupResource(lambdaGroup),
		logsGroupResource("/app/service/api"),
		logsGroupResource("/custom/app-logs"),
	}

	result, err := awsclient.EnrichLogsMetricFilters(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Fatal("Findings must not be nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected 0 findings for non-audit groups, got %d: %v", len(result.Findings), result.Findings)
	}
}

// TestEnrichLogsMetricFilters_NilClientReturnsEmptyFindingsNoError verifies that
// when clients.CloudWatchLogs is nil the enricher returns a non-nil empty Findings
// map and no error.
func TestEnrichLogsMetricFilters_NilClientReturnsEmptyFindingsNoError(t *testing.T) {
	clients := &awsclient.ServiceClients{CloudWatchLogs: nil}
	resources := []resource.Resource{
		logsGroupResource("/aws/cloudtrail/audit"),
		logsGroupResource("/aws/lambda/my-function"),
		logsGroupResource("/app/service/api"),
	}

	result, err := awsclient.EnrichLogsMetricFilters(context.Background(), clients, resources, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings == nil {
		t.Error("Findings must not be nil when CloudWatchLogs client is nil")
	}
	if len(result.Findings) != 0 {
		t.Errorf("expected empty Findings, got %d entries", len(result.Findings))
	}
}
