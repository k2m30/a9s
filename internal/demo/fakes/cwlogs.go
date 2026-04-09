package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// CWLogsFake implements aws.CWLogsAPI against fixture data loaded at construction time.
type CWLogsFake struct {
	fix *fixtures.CWLogsFixtures
}

// NewCWLogs constructs a CWLogsFake backed by fixture data from the fixtures package.
func NewCWLogs() *CWLogsFake {
	return &CWLogsFake{fix: fixtures.NewCWLogsFixtures()}
}

func (f *CWLogsFake) DescribeLogGroups(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	return &cloudwatchlogs.DescribeLogGroupsOutput{LogGroups: f.fix.LogGroups}, nil
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
