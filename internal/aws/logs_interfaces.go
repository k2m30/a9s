package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

// CWLogsDescribeLogGroupsAPI defines the interface for the CloudWatchLogs DescribeLogGroups operation.
type CWLogsDescribeLogGroupsAPI interface {
	DescribeLogGroups(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
}

// CWLogsDescribeLogStreamsAPI defines the interface for the CloudWatchLogs DescribeLogStreams operation.
type CWLogsDescribeLogStreamsAPI interface {
	DescribeLogStreams(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
}

// CWLogsDescribeMetricFiltersAPI defines the interface for the CloudWatchLogs DescribeMetricFilters operation.
// Used by Wave 2 EnrichLogsMetricFilters to detect audit log groups missing metric filters.
type CWLogsDescribeMetricFiltersAPI interface {
	DescribeMetricFilters(ctx context.Context, params *cloudwatchlogs.DescribeMetricFiltersInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeMetricFiltersOutput, error)
}

// CWLogsGetLogEventsAPI defines the interface for the CloudWatchLogs GetLogEvents operation.
type CWLogsGetLogEventsAPI interface {
	GetLogEvents(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error)
}

// CWLogsFilterLogEventsAPI defines the interface for the CloudWatchLogs FilterLogEvents operation.
type CWLogsFilterLogEventsAPI interface {
	FilterLogEvents(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error)
}

// CWLogsDescribeSubscriptionFiltersAPI defines the interface for DescribeSubscriptionFilters.
type CWLogsDescribeSubscriptionFiltersAPI interface {
	DescribeSubscriptionFilters(ctx context.Context, params *cloudwatchlogs.DescribeSubscriptionFiltersInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeSubscriptionFiltersOutput, error)
}

// CWLogsAPI is the aggregate interface covering all CloudWatchLogs operations used by a9s fetchers.
// *cloudwatchlogs.Client structurally satisfies this interface.
type CWLogsAPI interface {
	CWLogsDescribeLogGroupsAPI
	CWLogsDescribeLogStreamsAPI
	CWLogsGetLogEventsAPI
	CWLogsFilterLogEventsAPI
	CWLogsDescribeMetricFiltersAPI // Wave 2 enrichment
}
