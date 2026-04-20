package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

// CloudWatchDescribeAlarmsAPI defines the interface for the CloudWatch DescribeAlarms operation.
type CloudWatchDescribeAlarmsAPI interface {
	DescribeAlarms(ctx context.Context, params *cloudwatch.DescribeAlarmsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmsOutput, error)
}

// CloudWatchDescribeAlarmHistoryAPI defines the interface for the CloudWatch DescribeAlarmHistory operation.
type CloudWatchDescribeAlarmHistoryAPI interface {
	DescribeAlarmHistory(ctx context.Context, params *cloudwatch.DescribeAlarmHistoryInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmHistoryOutput, error)
}

// CloudWatchAPI is the aggregate interface covering all CloudWatch operations used by a9s fetchers.
// *cloudwatch.Client structurally satisfies this interface.
type CloudWatchAPI interface {
	CloudWatchDescribeAlarmsAPI
	CloudWatchDescribeAlarmHistoryAPI
}
