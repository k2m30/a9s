// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"

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
	return &cloudwatchlogs.DescribeLogStreamsOutput{LogStreams: f.fix.LogStreams[logGroupName]}, nil
}

func (f *CWLogsFake) GetLogEvents(_ context.Context, input *cloudwatchlogs.GetLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
	var logGroupName string
	if input != nil && input.LogGroupName != nil {
		logGroupName = *input.LogGroupName
	}
	return &cloudwatchlogs.GetLogEventsOutput{Events: f.fix.LogEvents[logGroupName]}, nil
}

func (f *CWLogsFake) FilterLogEvents(_ context.Context, input *cloudwatchlogs.FilterLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	var logGroupName string
	if input != nil && input.LogGroupName != nil {
		logGroupName = *input.LogGroupName
	}
	events := f.fix.LogEvents[logGroupName]
	filtered := make([]cwlogstypes.FilteredLogEvent, 0, len(events))
	for _, e := range events {
		filtered = append(filtered, cwlogstypes.FilteredLogEvent{
			Timestamp:     e.Timestamp,
			Message:       e.Message,
			IngestionTime: e.IngestionTime,
		})
	}
	return &cloudwatchlogs.FilterLogEventsOutput{Events: filtered}, nil
}

// DescribeMetricFilters returns an empty list for all log groups in demo mode.
func (f *CWLogsFake) DescribeMetricFilters(_ context.Context, _ *cloudwatchlogs.DescribeMetricFiltersInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeMetricFiltersOutput, error) {
	return &cloudwatchlogs.DescribeMetricFiltersOutput{MetricFilters: []cwlogstypes.MetricFilter{}}, nil
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
