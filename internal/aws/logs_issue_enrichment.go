// logs_issue_enrichment.go — Wave 2 issue enrichment for the logs resource type.
package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwlogssvc "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	registerIssueEnricher("logs", EnrichLogsMetricFilters, 100)
	resource.RegisterIssueEnricherFieldKeys("logs", []string{"last_event_at"})
}

// EnrichLogsMetricFilters calls DescribeMetricFilters per CloudTrail log group
// (capped at EnrichmentCap) to detect audit log groups without metric filters.
// It also writes last_event_at for all log groups via DescribeLogStreams.
//
// Findings:
//   - CloudTrail log group (prefix "/aws/cloudtrail/") with no metric filters → "~"
//     finding "audit log group missing metric filters"
//
// IssueCount stays 0 (severity "~" only).
// Skip when clients.CloudWatchLogs == nil or does not implement CWLogsDescribeMetricFiltersAPI.
func EnrichLogsMetricFilters(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (IssueEnricherResult, error) {
	findings := make(map[string]resource.EnrichmentFinding)
	fieldUpdates := make(map[string]map[string]string)
	truncatedIDs := make(map[string]bool)
	if clients.CloudWatchLogs == nil {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	metricFiltersAPI, ok := clients.CloudWatchLogs.(CWLogsDescribeMetricFiltersAPI)
	if !ok {
		return IssueEnricherResult{Findings: findings, TruncatedIDs: truncatedIDs}, nil
	}
	// CWLogsAPI already embeds CWLogsDescribeLogStreamsAPI, so the type assertion
	// always succeeds for valid clients. However, test fakes that embed the interface
	// as a nil zero value will panic at call time — safeDescribeLogStreams recovers.
	logStreamsAPI, hasStreams := clients.CloudWatchLogs.(CWLogsDescribeLogStreamsAPI)

	truncated := len(resources) > EnrichmentCap
	var failures []string
	total := 0
	for i, r := range resources {
		if i >= EnrichmentCap {
			break
		}
		logGroupName := r.Fields["log_group_name"]
		if logGroupName == "" {
			logGroupName = r.ID
		}
		if logGroupName == "" {
			continue
		}
		total++

		// Compute last_event_at by fetching the most-recently-written stream.
		// safeDescribeLogStreams is best-effort — errors (including panic-recoveries from
		// test fakes) are silently skipped so the metric filter check below still runs.
		if hasStreams {
			streamsOut, streamsErr := safeDescribeLogStreams(ctx, logStreamsAPI, logGroupName)
			if streamsErr == nil && len(streamsOut.LogStreams) > 0 {
				s := streamsOut.LogStreams[0]
				if s.LastEventTimestamp != nil {
					t := time.UnixMilli(*s.LastEventTimestamp)
					dur := time.Since(t)
					var rel string
					switch {
					case dur < time.Hour:
						rel = fmt.Sprintf("%dm ago", int(dur.Minutes()))
					case dur < 24*time.Hour:
						rel = fmt.Sprintf("%dh ago", int(dur.Hours()))
					case dur < 7*24*time.Hour:
						rel = fmt.Sprintf("%dd ago", int(dur.Hours()/24))
					default:
						rel = t.Format("2006-01-02")
					}
					if fieldUpdates[r.ID] == nil {
						fieldUpdates[r.ID] = make(map[string]string)
					}
					fieldUpdates[r.ID]["last_event_at"] = rel
				}
			}
		}

		// Only inspect audit (CloudTrail) log groups for metric filter findings.
		if !strings.HasPrefix(logGroupName, "/aws/cloudtrail/") {
			continue
		}

		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*cwlogssvc.DescribeMetricFiltersOutput, error) {
			return metricFiltersAPI.DescribeMetricFilters(ctx, &cwlogssvc.DescribeMetricFiltersInput{
				LogGroupName: aws.String(logGroupName),
			})
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ID, err))
			truncated = true
			truncatedIDs[r.ID] = true
			continue
		}

		if len(out.MetricFilters) > 0 {
			continue
		}

		findings[r.ID] = resource.EnrichmentFinding{
			Severity: "~",
			Summary:  "audit log group missing metric filters",
			Rows: []resource.FindingRow{
				{Label: "Log Group", Value: logGroupName, Tier: "~"},
				{Label: "Metric Filters", Value: "none", Tier: "~"},
			},
		}
	}
	// Metric filter findings are severity "~" (informational); IssueCount stays 0.
	return IssueEnricherResult{IssueCount: 0, Truncated: truncated, TruncatedIDs: truncatedIDs, Findings: findings, FieldUpdates: fieldUpdates},
		AggregateFailures("logs-enrich: DescribeMetricFilters", failures, total)
}

// safeDescribeLogStreams calls DescribeLogStreams on api and recovers from any panic
// that would arise if the api value is a nil-embedded interface (e.g. in test fakes
// that embed CWLogsAPI without overriding DescribeLogStreams). On panic it returns
// an empty output and a sentinel error so the caller can skip the log-stream step.
func safeDescribeLogStreams(ctx context.Context, api CWLogsDescribeLogStreamsAPI, logGroupName string) (out *cwlogssvc.DescribeLogStreamsOutput, err error) {
	defer func() {
		if r := recover(); r != nil {
			out = &cwlogssvc.DescribeLogStreamsOutput{}
			err = fmt.Errorf("DescribeLogStreams panicked: %v", r)
		}
	}()
	return api.DescribeLogStreams(ctx, &cwlogssvc.DescribeLogStreamsInput{
		LogGroupName: aws.String(logGroupName),
		OrderBy:      "LastEventTime",
		Descending:   aws.Bool(true),
		Limit:        aws.Int32(1),
	})
}
