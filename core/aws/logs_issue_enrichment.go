// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// logs_issue_enrichment.go — Wave 2 issue enrichment for the logs resource type.
package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwlogssvc "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// logs canonical FindingCodes.
const (
	logsCodeMissingMetricFilters domain.FindingCode = "logs.missing-metric-filters"
)

// EnrichLogsMetricFilters calls DescribeMetricFilters per CloudTrail log group
// (capped at EnrichmentCap) to detect audit log groups without metric filters.
// It also writes last_event_at for up to EnrichmentCap log groups via DescribeLogStreams.
//
// Findings:
//   - CloudTrail log group (prefix "/aws/cloudtrail/") with no metric filters → "~"
//     finding "audit log group missing metric filters"
func EnrichLogsMetricFilters(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
		FieldUpdates: make(map[string]map[string]string),
	}
	if clients.CloudWatchLogs == nil {
		return result, nil
	}
	metricFiltersAPI, ok := clients.CloudWatchLogs.(CWLogsDescribeMetricFiltersAPI)
	if !ok {
		return result, nil
	}
	logStreamsAPI, hasStreams := clients.CloudWatchLogs.(CWLogsDescribeLogStreamsAPI)

	var failures []Failure
	var mu sync.Mutex

	var streamsErr error
	if hasStreams {
		groups := fieldOnlyCap(resources)
		streamsErr = ForEachParallel(ctx, len(groups), EnrichmentParallelism, func(i int) {
			r := groups[i]
			name := logGroupNameOf(r)
			if name == "" {
				return
			}
			out, err := logStreamsAPI.DescribeLogStreams(ctx, &cwlogssvc.DescribeLogStreamsInput{
				LogGroupName: aws.String(name),
				OrderBy:      "LastEventTime",
				Descending:   aws.Bool(true),
				Limit:        aws.Int32(1),
			})
			// no finding: last_event_at is a column, left empty when the read fails or the group has no stream.
			if err != nil || len(out.LogStreams) == 0 || out.LogStreams[0].LastEventTimestamp == nil {
				return
			}
			t := time.UnixMilli(*out.LogStreams[0].LastEventTimestamp)
			var rel string
			switch dur := time.Since(t); {
			case dur < time.Hour:
				rel = fmt.Sprintf("%dm ago", int(dur.Minutes()))
			case dur < 24*time.Hour:
				rel = fmt.Sprintf("%dh ago", int(dur.Hours()))
			case dur < 7*24*time.Hour:
				rel = fmt.Sprintf("%dd ago", int(dur.Hours()/24))
			default:
				rel = t.Format("2006-01-02")
			}
			mu.Lock()
			if result.FieldUpdates[r.ID] == nil {
				result.FieldUpdates[r.ID] = make(map[string]string)
			}
			result.FieldUpdates[r.ID]["last_event_at"] = rel
			mu.Unlock()
		})
	}

	audit := capAtEnrichmentCap(&result, resources, func(r resource.Resource) bool {
		return strings.HasPrefix(logGroupNameOf(r), "/aws/cloudtrail/")
	}, resourceIDsOf)
	loopErr := ForEachRow(ctx, &result, resourceIDs(audit), EnrichmentParallelism, func(i int) {
		r := audit[i]
		logGroupName := logGroupNameOf(r)
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*cwlogssvc.DescribeMetricFiltersOutput, error) {
			return metricFiltersAPI.DescribeMetricFilters(ctx, &cwlogssvc.DescribeMetricFiltersInput{
				LogGroupName: aws.String(logGroupName),
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			MarkSkipped(&result, r.ID, &failures, err)
			return
		}

		if len(out.MetricFilters) > 0 {
			return
		}

		setWave2Finding(&result, r.ID, logsCodeMissingMetricFilters, []domain.DetailRow{
			{Label: "Log Group", Value: logGroupName, Tier: "~"},
			{Label: "Metric Filters", Value: "none", Tier: "~"},
		})
	})

	MarkInformationalOnly(&result)
	return result, errors.Join(streamsErr, loopErr, AggregateFailures("DescribeMetricFilters", failures, len(audit)))
}

// logGroupNameOf is the name the CloudWatch Logs calls take for a row.
func logGroupNameOf(r resource.Resource) string {
	if name := r.Fields["log_group_name"]; name != "" {
		return name
	}
	return r.ID
}
