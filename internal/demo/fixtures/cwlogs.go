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
		// prod-dbi-1 log groups — required for dbi→logs related-panel pivot.
		// Matches EnabledCloudwatchLogsExports = ["postgresql", "upgrade"] on prod-dbi-1.
		{
			LogGroupName:    aws.String("/aws/rds/instance/prod-dbi-1/postgresql"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/rds/instance/prod-dbi-1/postgresql:*"),
			StoredBytes:     aws.Int64(104857600),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1710000000000),
			KmsKeyId:        aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
		},
		{
			LogGroupName:    aws.String("/aws/rds/instance/prod-dbi-1/upgrade"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/rds/instance/prod-dbi-1/upgrade:*"),
			StoredBytes:     aws.Int64(1048576),
			RetentionInDays: aws.Int32(7),
			CreationTime:    aws.Int64(1710000100000),
		},
		// prod-dbi-aurora-1 log group — required for the dbi→logs pivot on
		// the Aurora dbi "all pivots non-zero" graph-root.
		{
			LogGroupName:    aws.String("/aws/rds/instance/prod-dbi-aurora-1/postgresql"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/rds/instance/prod-dbi-aurora-1/postgresql:*"),
			StoredBytes:     aws.Int64(83886080),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1710000200000),
			KmsKeyId:        aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"),
		},
		// prod-aurora-cluster log group — required for the dbc→logs pivot
		// on the Aurora dbc "all pivots non-zero" graph-root.
		{
			LogGroupName:    aws.String("/aws/rds/cluster/prod-aurora-cluster/postgresql"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/rds/cluster/prod-aurora-cluster/postgresql:*"),
			StoredBytes:     aws.Int64(52428800),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1710000300000),
		},
		// Issue: RetentionInDays=nil → Warning (log group never expires, unbounded cost)
		{
			LogGroupName: aws.String("/app/custom/no-retention"),
			Arn:          aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/app/custom/no-retention:*"),
			StoredBytes:  aws.Int64(5368709120),
			// RetentionInDays intentionally omitted (nil) = "Never Expire"
			CreationTime: aws.Int64(1672531200000), // 2023-01-01 — old, growing forever
		},
		// Issue: storedBytes=0 AND creationTime >90d ago → Warning (orphaned / stale log group)
		{
			LogGroupName:    aws.String("/app/legacy/orphan-old"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/app/legacy/orphan-old:*"),
			StoredBytes:     aws.Int64(0),
			RetentionInDays: aws.Int32(7),
			CreationTime:    aws.Int64(1688169600000), // 2023-07-01 — no data written in months
		},
		// acme-docdb-prod log groups — required for dbc→logs related-panel pivot.
		// Naming convention: /aws/docdb/<clusterID>/<logType>.
		{
			LogGroupName:    aws.String("/aws/docdb/acme-docdb-prod/audit"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/docdb/acme-docdb-prod/audit:*"),
			StoredBytes:     aws.Int64(209715200),
			RetentionInDays: aws.Int32(90),
			CreationTime:    aws.Int64(1745769600000), // 2025-04-28
		},
		{
			LogGroupName:    aws.String("/aws/docdb/acme-docdb-prod/profiler"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/docdb/acme-docdb-prod/profiler:*"),
			StoredBytes:     aws.Int64(52428800),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1745769600000), // 2025-04-28
		},
		// orders-prod DynamoDB Contributor Insights log group — DDB→logs pivot.
		// checkDdbLogs matches log groups whose ID contains the table name.
		// Naming convention: /aws/dynamodb/tables/<name>/insights/default.
		{
			LogGroupName:    aws.String("/aws/dynamodb/tables/" + OrdersProdID + "/insights/default"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/dynamodb/tables/" + OrdersProdID + "/insights/default:*"),
			StoredBytes:     aws.Int64(20971520),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1741996800000), // 2025-03-15
		},
		// Redis prod slow-log group — required for redis→logs related-panel pivot.
		// The prod-redis-sessions RG LogDeliveryConfigurations destination points here.
		{
			LogGroupName:    aws.String(ProdRedisLogGroup),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + ProdRedisLogGroup + ":*"),
			StoredBytes:     aws.Int64(10485760),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1741996800000), // 2025-03-15
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
		"/aws/lambda/process-orders": {
			{
				LogStreamName:       aws.String("2026/03/22/[$LATEST]ord789"),
				CreationTime:        aws.Int64(1774253000000),
				FirstEventTimestamp: aws.Int64(1774253000000),
				LastEventTimestamp:  aws.Int64(1774253900000),
				StoredBytes:         aws.Int64(4096),
			},
			{
				LogStreamName:       aws.String("2026/03/21/[$LATEST]ord456"),
				CreationTime:        aws.Int64(1774166000000),
				FirstEventTimestamp: aws.Int64(1774166000000),
				LastEventTimestamp:  aws.Int64(1774167000000),
				StoredBytes:         aws.Int64(3072),
			},
			{
				LogStreamName:       aws.String("2026/03/20/[$LATEST]ord123"),
				CreationTime:        aws.Int64(1774080000000),
				FirstEventTimestamp: aws.Int64(1774080000000),
				LastEventTimestamp:  aws.Int64(1774081000000),
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
				Message:       aws.String("REPORT RequestId: abc-123 Duration: 45.23 ms Billed Duration: 46 ms Memory Size: 256 MB Max Memory Used: 87 MB"),
				IngestionTime: aws.Int64(1774253703100),
			},
			{
				Timestamp:     aws.Int64(1774253600000),
				Message:       aws.String("REPORT RequestId: abc-122 Duration: 312.50 ms Billed Duration: 313 ms Memory Size: 256 MB Max Memory Used: 112 MB Init Duration: 245.18 ms"),
				IngestionTime: aws.Int64(1774253600100),
			},
			{
				Timestamp:     aws.Int64(1774253500000),
				Message:       aws.String("REPORT RequestId: abc-121 Duration: 38.91 ms Billed Duration: 39 ms Memory Size: 256 MB Max Memory Used: 85 MB"),
				IngestionTime: aws.Int64(1774253500100),
			},
		},
		"/aws/lambda/process-orders": {
			{
				Timestamp:     aws.Int64(1774253800000),
				Message:       aws.String("START RequestId: ord-901 Version: $LATEST"),
				IngestionTime: aws.Int64(1774253800100),
			},
			{
				Timestamp:     aws.Int64(1774253801000),
				Message:       aws.String("ERROR Failed to process order ORD-7842: DynamoDB ConditionalCheckFailedException: The conditional request failed. Item {pk: ORDER#7842, sk: STATUS} already exists with status=SHIPPED. Expected status=PENDING for transition to PROCESSING. This usually means a duplicate SQS message was delivered after the order was already fulfilled. Correlation-ID: cx-9f3a-44b1-8e72 Account: 123456789012 Region: us-east-1 Table: acme-orders-prod"),
				IngestionTime: aws.Int64(1774253801100),
			},
			{
				Timestamp:     aws.Int64(1774253802000),
				Message:       aws.String("REPORT RequestId: ord-901 Duration: 1523.47 ms Billed Duration: 1524 ms Memory Size: 128 MB Max Memory Used: 98 MB"),
				IngestionTime: aws.Int64(1774253802100),
			},
			{
				Timestamp:     aws.Int64(1774253700000),
				Message:       aws.String("REPORT RequestId: ord-900 Duration: 82.15 ms Billed Duration: 83 ms Memory Size: 128 MB Max Memory Used: 74 MB"),
				IngestionTime: aws.Int64(1774253700100),
			},
			{
				Timestamp:     aws.Int64(1774253600000),
				Message:       aws.String("REPORT RequestId: ord-899 Duration: 95.33 ms Billed Duration: 96 ms Memory Size: 128 MB Max Memory Used: 76 MB"),
				IngestionTime: aws.Int64(1774253600100),
			},
			{
				Timestamp:     aws.Int64(1774253500000),
				Message:       aws.String("REPORT RequestId: ord-898 Duration: 445.80 ms Billed Duration: 446 ms Memory Size: 128 MB Max Memory Used: 91 MB Init Duration: 387.22 ms"),
				IngestionTime: aws.Int64(1774253500100),
			},
			{
				Timestamp:     aws.Int64(1774253400000),
				Message:       aws.String("REPORT RequestId: ord-897 Duration: 67.42 ms Billed Duration: 68 ms Memory Size: 128 MB Max Memory Used: 72 MB"),
				IngestionTime: aws.Int64(1774253400100),
			},
		},
	}

	return &CWLogsFixtures{
		LogGroups:  logGroups,
		LogStreams: logStreams,
		LogEvents:  logEvents,
	}
}
