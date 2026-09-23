// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// CWLogsFake implements aws.CWLogsAPI against fixture data loaded at construction time.
type CWLogsFake struct {
	fix *fixtures.CWLogsFixtures
}

// NewCWLogs constructs a CWLogsFake backed by fixture data from the fixtures package.
func NewCWLogs() *CWLogsFake {
	return &CWLogsFake{fix: fixtures.NewCWLogsFixtures()}
}

func (f *CWLogsFake) DescribeLogGroups(_ context.Context, input *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	groups := f.fix.LogGroups
	if input != nil && input.NextToken != nil && *input.NextToken != "" {
		if len(groups) > fixtures.LogGroupsPageSize {
			return &cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: groups[fixtures.LogGroupsPageSize:]}, nil
		}
		return &cloudwatchlogs.DescribeLogGroupsOutput{}, nil
	}
	if len(groups) > fixtures.LogGroupsPageSize {
		next := "log-groups-page-2"
		return &cloudwatchlogs.DescribeLogGroupsOutput{
			LogGroups: groups[:fixtures.LogGroupsPageSize],
			NextToken: &next,
		}, nil
	}
	return &cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: groups}, nil
}

func (f *CWLogsFake) DescribeLogStreams(_ context.Context, input *cloudwatchlogs.DescribeLogStreamsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	var logGroupName string
	if input != nil && input.LogGroupName != nil {
		logGroupName = *input.LogGroupName
	}
	if !f.hasLogGroup(logGroupName) {
		return nil, &cwlogstypes.ResourceNotFoundException{
			Message: notFoundMessage("The specified log group", logGroupName),
		}
	}
	return &cloudwatchlogs.DescribeLogStreamsOutput{LogStreams: f.fix.LogStreams[logGroupName]}, nil
}

func (f *CWLogsFake) GetLogEvents(_ context.Context, input *cloudwatchlogs.GetLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
	var logGroupName string
	if input != nil && input.LogGroupName != nil {
		logGroupName = *input.LogGroupName
	}
	// GetLogEvents answers in timestamp order; the fixtures list some groups
	// newest first.
	events := slices.Clone(f.fix.LogEvents[logGroupName])
	slices.SortStableFunc(events, func(a, b cwlogstypes.OutputLogEvent) int {
		return cmp.Compare(aws.ToInt64(a.Timestamp), aws.ToInt64(b.Timestamp))
	})
	return &cloudwatchlogs.GetLogEventsOutput{Events: events}, nil
}

func (f *CWLogsFake) FilterLogEvents(_ context.Context, input *cloudwatchlogs.FilterLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	var logGroupName string
	if input != nil && input.LogGroupName != nil {
		logGroupName = *input.LogGroupName
	}
	events := f.fix.LogEvents[logGroupName]
	filtered := make([]cwlogstypes.FilteredLogEvent, 0, len(events))
	for i, e := range events {
		ts := aws.ToInt64(e.Timestamp)
		if input != nil && (input.StartTime != nil && ts < *input.StartTime || input.EndTime != nil && ts > *input.EndTime) {
			continue
		}
		stream := f.filteredStreamName(logGroupName, i)
		if input != nil && !strings.HasPrefix(stream, aws.ToString(input.LogStreamNamePrefix)) {
			continue
		}
		// EventId and LogStreamName are what FilterLogEvents adds over
		// GetLogEvents, and consumers key their rows by them: without an id
		// every event of a group collapses onto one row. The fixture stores
		// events per group, so the stream is the group's own.
		filtered = append(filtered, cwlogstypes.FilteredLogEvent{
			Timestamp:     e.Timestamp,
			Message:       e.Message,
			IngestionTime: e.IngestionTime,
			EventId:       aws.String(fmt.Sprintf("%d%011d", aws.ToInt64(e.Timestamp), i)),
			LogStreamName: aws.String(stream),
		})
	}
	return &cloudwatchlogs.FilterLogEventsOutput{Events: filtered}, nil
}

// DescribeMetricFilters answers either shape of the call AWS accepts: the
// filters of one log group, or the filters that emit one metric.
func (f *CWLogsFake) DescribeMetricFilters(_ context.Context, input *cloudwatchlogs.DescribeMetricFiltersInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeMetricFiltersOutput, error) {
	filters := []cwlogstypes.MetricFilter{}
	if input == nil {
		return &cloudwatchlogs.DescribeMetricFiltersOutput{MetricFilters: filters}, nil
	}
	if input.LogGroupName != nil {
		return &cloudwatchlogs.DescribeMetricFiltersOutput{MetricFilters: f.fix.MetricFilters[*input.LogGroupName]}, nil
	}
	if input.MetricName == nil || input.MetricNamespace == nil {
		return &cloudwatchlogs.DescribeMetricFiltersOutput{MetricFilters: filters}, nil
	}
	for _, group := range slices.Sorted(maps.Keys(f.fix.MetricFilters)) {
		for _, filter := range f.fix.MetricFilters[group] {
			for _, t := range filter.MetricTransformations {
				if aws.ToString(t.MetricName) == *input.MetricName && aws.ToString(t.MetricNamespace) == *input.MetricNamespace {
					filters = append(filters, filter)
				}
			}
		}
	}
	return &cloudwatchlogs.DescribeMetricFiltersOutput{MetricFilters: filters}, nil
}

// DescribeSubscriptionFilters returns subscription filters for the named log
// group from fixture data. Backs the logs:kinesis and logs:s3 related-panel
// pivots (checkLogsKinesis / checkLogsS3).
func (f *CWLogsFake) DescribeSubscriptionFilters(_ context.Context, input *cloudwatchlogs.DescribeSubscriptionFiltersInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeSubscriptionFiltersOutput, error) {
	var logGroupName string
	if input != nil && input.LogGroupName != nil {
		logGroupName = *input.LogGroupName
	}
	return &cloudwatchlogs.DescribeSubscriptionFiltersOutput{SubscriptionFilters: f.fix.SubscriptionFilters[logGroupName]}, nil
}

// filteredStreamName names the stream the i-th event of a group came from,
// cycling the group's fixture streams, and "" for a group the fixtures give
// no streams — the same empty value a real event without one carries.
func (f *CWLogsFake) filteredStreamName(logGroupName string, i int) string {
	streams := f.fix.LogStreams[logGroupName]
	if len(streams) == 0 {
		return ""
	}
	return aws.ToString(streams[i%len(streams)].LogStreamName)
}

// hasLogGroup reports whether the fixtures register this log group. A group
// with no streams still answers an empty stream list.
func (f *CWLogsFake) hasLogGroup(name string) bool {
	return slices.ContainsFunc(f.fix.LogGroups, func(g cwlogstypes.LogGroup) bool {
		return aws.ToString(g.LogGroupName) == name
	})
}
