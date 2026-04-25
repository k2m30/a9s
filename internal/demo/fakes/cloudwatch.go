package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// CloudWatchFake implements aws.CloudWatchAPI against fixture data loaded at construction time.
type CloudWatchFake struct {
	fix *fixtures.CloudWatchFixtures
}

// NewCloudWatch constructs a CloudWatchFake backed by fixture data from the fixtures package.
func NewCloudWatch() *CloudWatchFake {
	return &CloudWatchFake{fix: fixtures.NewCloudWatchFixtures()}
}

func (f *CloudWatchFake) DescribeAlarms(_ context.Context, _ *cloudwatch.DescribeAlarmsInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmsOutput, error) {
	return &cloudwatch.DescribeAlarmsOutput{MetricAlarms: f.fix.Alarms}, nil
}

func (f *CloudWatchFake) DescribeAlarmHistory(_ context.Context, in *cloudwatch.DescribeAlarmHistoryInput, _ ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmHistoryOutput, error) {
	if in == nil || in.AlarmName == nil {
		return &cloudwatch.DescribeAlarmHistoryOutput{AlarmHistoryItems: []cwtypes.AlarmHistoryItem{}}, nil
	}
	items, ok := f.fix.AlarmHistory[*in.AlarmName]
	if !ok {
		return &cloudwatch.DescribeAlarmHistoryOutput{AlarmHistoryItems: []cwtypes.AlarmHistoryItem{}}, nil
	}
	return &cloudwatch.DescribeAlarmHistoryOutput{AlarmHistoryItems: items}, nil
}
