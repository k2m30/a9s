package fixtures

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// CWLogsFixtures holds typed fixture data for CloudWatch Logs.
type CWLogsFixtures struct {
	LogGroups []cwlogstypes.LogGroup
	// LogStreams maps log group name to its streams.
	LogStreams map[string][]cwlogstypes.LogStream
	// LogEvents maps log group name to its events (for GetLogEvents / FilterLogEvents).
	LogEvents map[string][]cwlogstypes.OutputLogEvent
}

// NewCWLogsFixtures constructs CWLogsFixtures from the canonical demo data.
func NewCWLogsFixtures() *CWLogsFixtures {
	logGroups := []cwlogstypes.LogGroup{
		{
			LogGroupName:              aws.String("/aws/lambda/api-gateway-authorizer"),
			Arn:                       aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/api-gateway-authorizer:*"),
			LogGroupArn:               aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/api-gateway-authorizer:*"),
			StoredBytes:               aws.Int64(52428800),
			RetentionInDays:           aws.Int32(30),
			CreationTime:              aws.Int64(1704067200000),
			KmsKeyId:                  aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
			LogGroupClass:             cwlogstypes.LogGroupClassStandard,
			MetricFilterCount:         aws.Int32(2),
			DataProtectionStatus:      cwlogstypes.DataProtectionStatusActivated,
			DeletionProtectionEnabled: aws.Bool(false),
		},
		{
			LogGroupName:    aws.String("/aws/lambda/process-orders"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/process-orders:*"),
			StoredBytes:     aws.Int64(73400320),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1705067200000),
		},
		{
			LogGroupName:    aws.String("/aws/eks/acme-prod/cluster"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/eks/acme-prod/cluster:*"),
			StoredBytes:     aws.Int64(1073741824),
			RetentionInDays: aws.Int32(90),
			CreationTime:    aws.Int64(1700000000000),
		},
		{
			LogGroupName:    aws.String("/aws/eks/acme-staging/cluster"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/eks/acme-staging/cluster:*"),
			StoredBytes:     aws.Int64(268435456),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1702000000000),
		},
		{
			LogGroupName:    aws.String("/aws/rds/instance/prod-api-primary/postgresql"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/rds/instance/prod-api-primary/postgresql:*"),
			StoredBytes:     aws.Int64(209715200),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1706000000000),
		},
	}

	logStreams := map[string][]cwlogstypes.LogStream{
		"/aws/lambda/api-gateway-authorizer": {
			{
				LogStreamName:       aws.String("2026/03/22/[$LATEST]abc123"),
				CreationTime:        aws.Int64(1774253700000),
				FirstEventTimestamp: aws.Int64(1774253700000),
				LastEventTimestamp:  aws.Int64(1774253800000),
				StoredBytes:         aws.Int64(1024),
			},
			{
				LogStreamName:       aws.String("2026/03/21/[$LATEST]def456"),
				CreationTime:        aws.Int64(1774167300000),
				FirstEventTimestamp: aws.Int64(1774167300000),
				LastEventTimestamp:  aws.Int64(1774167400000),
				StoredBytes:         aws.Int64(2048),
			},
		},
	}

	logEvents := map[string][]cwlogstypes.OutputLogEvent{
		"/aws/lambda/api-gateway-authorizer": {
			{
				Timestamp:     aws.Int64(1774253700000),
				Message:       aws.String("START RequestId: abc-123 Version: $LATEST"),
				IngestionTime: aws.Int64(1774253700100),
			},
			{
				Timestamp:     aws.Int64(1774253701000),
				Message:       aws.String("INFO Authorizing request for user: alice@acme-corp.com"),
				IngestionTime: aws.Int64(1774253701100),
			},
			{
				Timestamp:     aws.Int64(1774253702000),
				Message:       aws.String("END RequestId: abc-123"),
				IngestionTime: aws.Int64(1774253702100),
			},
			{
				Timestamp:     aws.Int64(1774253703000),
				Message:       aws.String("REPORT RequestId: abc-123 Duration: 45.23 ms Billed Duration: 46 ms"),
				IngestionTime: aws.Int64(1774253703100),
			},
		},
	}

	return &CWLogsFixtures{
		LogGroups: logGroups,
		LogStreams: logStreams,
		LogEvents: logEvents,
	}
}
