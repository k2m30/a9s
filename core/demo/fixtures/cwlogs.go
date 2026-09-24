// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fixtures

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

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
	// SubscriptionFilters maps log group name to its subscription filters —
	// backs logs:DescribeSubscriptionFilters for the logs:kinesis pivot.
	SubscriptionFilters map[string][]cwlogstypes.SubscriptionFilter
	// ExportTasks are the account's export tasks, served by
	// DescribeExportTasks for the logs:s3 pivot.
	ExportTasks []cwlogstypes.ExportTask
	// MetricFilters maps log group name to its metric filters — backs
	// logs:DescribeMetricFilters for the logs↔alarm bridge, where the alarm
	// watches the metric a filter emits and names no log group at all.
	MetricFilters map[string][]cwlogstypes.MetricFilter
	// EventStreams names the one stream every event of a group was written
	// to, for a group whose events are not spread over its streams in turn:
	// a Lambda execution environment writes each invocation, START to
	// REPORT, to one stream.
	EventStreams map[string]string
}

// OrphanOldLogGroupName is the log group whose metric filter bridges it to an
// alarm: the filter counts ERROR lines into a custom namespace, and the alarm
// is over that metric.
const OrphanOldLogGroupName = "/app/legacy/orphan-old"

// OrphanOldMetricNamespace and OrphanOldMetricName are what its filter emits.
const (
	OrphanOldMetricNamespace = "LogMetrics"
	OrphanOldMetricName      = "OrphanErrorCount"
)

// BuildLogRepeatedLine is written twice at BuildLogRepeatedLineAt in the
// acme-api-build log: two events of one millisecond carrying one line, which
// GetLogEvents hands back with no id of their own.
const (
	BuildLogRepeatedLine   = "[Container] 2026/03/22 03:17:12 Retrying artifact upload (attempt 2 of 3)"
	BuildLogRepeatedLineAt = int64(1774149432000)
)

// LambdaFailedRequestID is the one invocation whose REPORT line says it
// failed, out of memory.
const LambdaFailedRequestID = "ord-896"

// LambdaJSONTimeoutRequestID is the one JSON-format invocation that timed out.
const LambdaJSONTimeoutRequestID = "d7a1c3e5-0b2d-4f6a-8c9e-1a3b5c7d9e02"

// ECSAPIGatewayHistoryLines is how many older access lines the api-gateway
// service's log group holds behind its four newest events.
const ECSAPIGatewayHistoryLines = 236

// NewCWLogsFixtures constructs CWLogsFixtures from the canonical demo data.
var sharedCWLogsFixtures = shared(func() *CWLogsFixtures {
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
			LogGroupName:    aws.String("/aws/lambda/" + LambdaJSONLogFormat),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/" + LambdaJSONLogFormat + ":*"),
			StoredBytes:     aws.Int64(20971520),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1705067200000),
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
			// A nil RetentionInDays means "Never Expire".
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
		// Glue shared job log groups — required for glue:logs related-panel
		// pivot. checkGlueLogs matches the fixed IDs "/aws-glue/jobs/output"
		// and "/aws-glue/jobs/error" (shared across all Glue jobs).
		{
			LogGroupName:    aws.String("/aws-glue/jobs/output"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws-glue/jobs/output:*"),
			StoredBytes:     aws.Int64(41943040),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1715731200000), // 2024-05-15
		},
		{
			LogGroupName:    aws.String("/aws-glue/jobs/error"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws-glue/jobs/error:*"),
			StoredBytes:     aws.Int64(10485760),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1715731200000), // 2024-05-15
		},
		{
			LogGroupName:    aws.String("/aws/docdb/acme-docdb-prod/profiler"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/docdb/acme-docdb-prod/profiler:*"),
			StoredBytes:     aws.Int64(52428800),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1745769600000), // 2025-04-28
		},
		// A log group named after the orders-prod table; DynamoDB writes no
		// log group, so it is not the table's.
		{
			LogGroupName:    aws.String("/aws/dynamodb/tables/" + OrdersProdID + "/insights/default"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/dynamodb/tables/" + OrdersProdID + "/insights/default:*"),
			StoredBytes:     aws.Int64(20971520),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1741996800000), // 2025-03-15
		},
		// OpenSearch graph-root log groups — required for opensearch→logs related-panel pivot.
		// The acme-logs domain's LogPublishingOptions maps each log type to one of these groups.
		{
			LogGroupName:    aws.String(OpenSearchLogGroupSearchSlow),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + OpenSearchLogGroupSearchSlow + ":*"),
			StoredBytes:     aws.Int64(52428800),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1746057600000), // 2025-05-01
		},
		{
			LogGroupName:    aws.String(OpenSearchLogGroupIndexSlow),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + OpenSearchLogGroupIndexSlow + ":*"),
			StoredBytes:     aws.Int64(26214400),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1746057600000), // 2025-05-01
		},
		{
			LogGroupName:    aws.String(OpenSearchLogGroupAudit),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + OpenSearchLogGroupAudit + ":*"),
			StoredBytes:     aws.Int64(10485760),
			RetentionInDays: aws.Int32(90),
			CreationTime:    aws.Int64(1746057600000), // 2025-05-01
		},
		// acme-warehouse Redshift log groups — required for redshift→logs related-panel pivot.
		// checkRedshiftLogs returns /aws/redshift/cluster/acme-warehouse as the prefix ID;
		// the three canonical CloudWatch audit log groups follow this naming convention.
		{
			LogGroupName:    aws.String("/aws/redshift/cluster/" + AcmeWarehouseID + "/connectionlog"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/redshift/cluster/" + AcmeWarehouseID + "/connectionlog:*"),
			StoredBytes:     aws.Int64(52428800),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1741996800000), // 2025-03-15
		},
		{
			LogGroupName:    aws.String("/aws/redshift/cluster/" + AcmeWarehouseID + "/userlog"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/redshift/cluster/" + AcmeWarehouseID + "/userlog:*"),
			StoredBytes:     aws.Int64(20971520),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1741996800000), // 2025-03-15
		},
		{
			LogGroupName:    aws.String("/aws/redshift/cluster/" + AcmeWarehouseID + "/useractivitylog"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/redshift/cluster/" + AcmeWarehouseID + "/useractivitylog:*"),
			StoredBytes:     aws.Int64(10485760),
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
		// bastion-prod (i-0a1b2c3d4e5f60005) CloudWatch-agent log group —
		// required for ec2→logs related-panel pivot. checkEC2Logs matches log
		// groups whose ID contains the instance ID.
		{
			LogGroupName:    aws.String("/aws/ec2/i-0a1b2c3d4e5f60005"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/ec2/i-0a1b2c3d4e5f60005:*"),
			StoredBytes:     aws.Int64(20971520),
			RetentionInDays: aws.Int32(14),
			CreationTime:    aws.Int64(1748736000000), // 2025-06-01
		},
		// The acme-services cluster's ecs exec session transcripts — the one
		// log group a Cluster names, and the witness for the ecs→logs pivot.
		{
			LogGroupName:    aws.String(ECSExecLogGroup),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + ECSExecLogGroup + ":*"),
			StoredBytes:     aws.Int64(10485760),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1750050000000),
		},
		// Where the acme-services cluster's application containers write.
		{
			LogGroupName:    aws.String("/ecs/acme-services/app"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/ecs/acme-services/app:*"),
			StoredBytes:     aws.Int64(157286400),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1750000000000),
		},
		// api-gateway ECS task-definition family log group — required for
		// ecs-svc→logs and ecs-task→logs related-panel pivots. Both read the
		// awslogs-group option of the family's container definitions.
		{
			LogGroupName:    aws.String("/ecs/api-gateway"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/ecs/api-gateway:*"),
			StoredBytes:     aws.Int64(78643200),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1750100000000),
		},
		// The other two families the acme-services cluster runs. One family
		// with a log group would let a reader take the ecs-svc→logs count for
		// a property of the pivot rather than of the service's own definition.
		{
			LogGroupName:    aws.String("/ecs/web-frontend"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/ecs/web-frontend:*"),
			StoredBytes:     aws.Int64(41943040),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1750110000000),
		},
		{
			LogGroupName:    aws.String("/ecs/order-worker"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/ecs/order-worker:*"),
			StoredBytes:     aws.Int64(20971520),
			RetentionInDays: aws.Int32(14),
			CreationTime:    aws.Int64(1750120000000),
		},
		// acme-public-api execution log group — required for apigw:logs
		// related-panel pivot. checkApigwLogs matches log groups whose ID
		// has the "API-Gateway-Execution-Logs_{apiID}/" prefix.
		{
			LogGroupName:    aws.String("API-Gateway-Execution-Logs_" + PublicAPIGWID + "/$default"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:API-Gateway-Execution-Logs_" + PublicAPIGWID + "/$default:*"),
			StoredBytes:     aws.Int64(31457280),
			RetentionInDays: aws.Int32(14),
			CreationTime:    aws.Int64(1750200000000),
		},
		// acme-api-build CodeBuild log group — required for cb:logs
		// related-panel pivot. checkCbLogs matches Project.LogsConfig.
		// CloudWatchLogs.GroupName exactly.
		{
			LogGroupName:    aws.String("/aws/codebuild/acme-api-build"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/codebuild/acme-api-build:*"),
			StoredBytes:     aws.Int64(20971520),
			RetentionInDays: aws.Int32(14),
			CreationTime:    aws.Int64(1750300000000),
		},
		// The groups the demo state machines' LoggingConfiguration names.
		{
			LogGroupName:    aws.String(SFNOrderFulfillmentLogGroup),
			Arn:             aws.String(sfnLogGroupARN(SFNOrderFulfillmentLogGroup)),
			StoredBytes:     aws.Int64(15728640),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1750400000000),
		},
		{
			LogGroupName:    aws.String(SFNWorkflowsLogGroup),
			Arn:             aws.String(sfnLogGroupARN(SFNWorkflowsLogGroup)),
			StoredBytes:     aws.Int64(31457280),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1750400000000),
		},
		// acme-prod-api Elastic Beanstalk log group — required for eb:logs
		// related-panel pivot. checkEbLogs matches log groups whose ID has
		// the "/aws/elasticbeanstalk/{envName}/" prefix.
		{
			LogGroupName:    aws.String("/aws/elasticbeanstalk/acme-prod-api/var/log/eb-engine.log"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/elasticbeanstalk/acme-prod-api/var/log/eb-engine.log:*"),
			StoredBytes:     aws.Int64(10485760),
			RetentionInDays: aws.Int32(14),
			CreationTime:    aws.Int64(1750500000000),
		},
		// CloudTrail delivery log group — required for trail:logs related-panel
		// pivot. checkTrailLogs parses the log group name out of the trail's
		// CloudWatchLogsLogGroupArn and matches it against this cache by ID.
		{
			LogGroupName:    aws.String("/aws/cloudtrail"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/cloudtrail:*"),
			StoredBytes:     aws.Int64(104857600),
			RetentionInDays: aws.Int32(90),
			CreationTime:    aws.Int64(1750600000000),
		},
		// The flow-log destinations EC2Fixtures.FlowLogsByResourceID names.
		{
			LogGroupName:    aws.String("/aws/vpc/flowlogs/vpce-s3-endpoint"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/vpc/flowlogs/vpce-s3-endpoint:*"),
			StoredBytes:     aws.Int64(31457280),
			RetentionInDays: aws.Int32(14),
			CreationTime:    aws.Int64(1750700000000),
		},
		{
			LogGroupName:    aws.String("/aws/vpc/flowlogs/acme-staging"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/vpc/flowlogs/acme-staging:*"),
			StoredBytes:     aws.Int64(41943040),
			RetentionInDays: aws.Int32(14),
			CreationTime:    aws.Int64(1750700000000),
		},
		// security-audit-trail's own per-trail delivery log group — a
		// sub-path under /aws/cloudtrail/ (unlike the shared "/aws/cloudtrail"
		// group above). EnrichLogsMetricFilters only inspects groups matching
		// the "/aws/cloudtrail/" prefix, and no metric filter is registered
		// for this one, so it fires logs.missing-metric-filters.
		{
			LogGroupName:    aws.String("/aws/cloudtrail/security-audit-trail"),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/cloudtrail/security-audit-trail:*"),
			StoredBytes:     aws.Int64(52428800),
			RetentionInDays: aws.Int32(90),
			CreationTime:    aws.Int64(1750800000000),
		},
		// prod-as2-gateway's two structured-log destinations (transfer.go
		// graph root) — required for the transfer→logs related-panel pivot
		// (count ≥2).
		{
			LogGroupName:    aws.String(transferLogGroupPrimaryName),
			Arn:             aws.String(transferLogGroupArn(transferLogGroupPrimaryName)),
			StoredBytes:     aws.Int64(10485760),
			RetentionInDays: aws.Int32(90),
			CreationTime:    aws.Int64(1727740800000),
		},
		{
			LogGroupName:    aws.String(transferLogGroupPartnerAuditName),
			Arn:             aws.String(transferLogGroupArn(transferLogGroupPartnerAuditName)),
			StoredBytes:     aws.Int64(5242880),
			RetentionInDays: aws.Int32(90),
			CreationTime:    aws.Int64(1727740800000),
		},
	}

	// prod-airflow-etl log groups — required for mwaa→logs related-panel
	// pivot (5 per-component groups). Names/ARNs are built from the same
	// mwaaLogGroupName/mwaaLogGroupArn helpers (mwaa.go) that populate the
	// environment's own LoggingConfiguration.*.CloudWatchLogGroupArn, so
	// the two fixture files can never drift apart.
	for _, lg := range []struct {
		component string
		bytes     int64
	}{
		{"DAGProcessing", 15728640},
		{"Scheduler", 20971520},
		{"WebServer", 10485760},
		{"Worker", 31457280},
		{"Task", 52428800},
	} {
		logGroups = append(logGroups, cwlogstypes.LogGroup{
			LogGroupName:    aws.String(mwaaLogGroupName(ProdAirflowEtlID, lg.component)),
			Arn:             aws.String(mwaaLogGroupArn(ProdAirflowEtlID, lg.component)),
			StoredBytes:     aws.Int64(lg.bytes),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1756704000000),
		})
	}

	logGroups = append(logGroups, derivedLogGroups(logGroups)...)

	logGroups = append(logGroups, cwlogstypes.LogGroup{
		LogGroupName:    aws.String(LogGroupNoKMS),
		Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + LogGroupNoKMS + ":*"),
		StoredBytes:     aws.Int64(8388608),
		RetentionInDays: aws.Int32(90),
		CreationTime:    aws.Int64(1735689600000),
	})

	// The second page. Every other group in this file is the target of some
	// pivot, so cutting the list anywhere inside them lowers a pivot count;
	// this one is named to match no pivot's convention and is appended
	// last, so LogGroupsPageSize can sit at len-1 and move only this row.
	logGroups = append(logGroups, cwlogstypes.LogGroup{
		LogGroupName:    aws.String(LogGroupSecondPageOnly),
		Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + LogGroupSecondPageOnly + ":*"),
		StoredBytes:     aws.Int64(4194304),
		RetentionInDays: aws.Int32(30),
		CreationTime:    aws.Int64(1756704000000),
	})

	// LogGroupNoKMS is the only demo group without a customer key. Every
	// other group is encrypted here rather than in forty literals, so a group
	// added later cannot quietly become a second carrier of logs.no-kms.
	for i := range logGroups {
		if logGroups[i].KmsKeyId == nil && aws.ToString(logGroups[i].LogGroupName) != LogGroupNoKMS {
			logGroups[i].KmsKeyId = aws.String(demoLogsKMSKeyARN)
		}
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
		"/aws/lambda/" + LambdaJSONLogFormat: {
			{
				LogStreamName:       aws.String("2026/03/22/[$LATEST]dpt4c1e"),
				CreationTime:        aws.Int64(1774252800000),
				FirstEventTimestamp: aws.Int64(1774252800000),
				LastEventTimestamp:  aws.Int64(1774253316875),
				StoredBytes:         aws.Int64(4096),
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
		// Graph-root log groups that must have streams so logs→log_streams drill lands non-empty.
		"/aws/dynamodb/tables/" + OrdersProdID + "/insights/default": minimalLogStreams("ddb-insights"),
		"/aws/rds/instance/prod-dbi-aurora-1/postgresql":             minimalLogStreams("dbi-aurora-pg"),
		"/aws/rds/cluster/prod-aurora-cluster/postgresql":            minimalLogStreams("dbc-aurora-pg"),
		mwaaLogGroupName(ProdAirflowEtlID, "DAGProcessing"):          minimalLogStreams("mwaa-dag-processing"),
		mwaaLogGroupName(ProdAirflowEtlID, "Scheduler"):              minimalLogStreams("mwaa-scheduler"),
		mwaaLogGroupName(ProdAirflowEtlID, "WebServer"):              minimalLogStreams("mwaa-webserver"),
		mwaaLogGroupName(ProdAirflowEtlID, "Worker"):                 minimalLogStreams("mwaa-worker"),
		mwaaLogGroupName(ProdAirflowEtlID, "Task"):                   minimalLogStreams("mwaa-task"),
		ProdRedisLogGroup:            minimalLogStreams("redis-slow"),
		OpenSearchLogGroupAudit:      minimalLogStreams("os-audit"),
		OpenSearchLogGroupIndexSlow:  minimalLogStreams("os-index-slow"),
		OpenSearchLogGroupSearchSlow: minimalLogStreams("os-search-slow"),
		"/aws/redshift/cluster/" + AcmeWarehouseID + "/connectionlog":   minimalLogStreams("rs-conn"),
		"/aws/redshift/cluster/" + AcmeWarehouseID + "/userlog":         minimalLogStreams("rs-user"),
		"/aws/redshift/cluster/" + AcmeWarehouseID + "/useractivitylog": minimalLogStreams("rs-useract"),
		// Lambda log groups for graph-root-drilled functions — required for
		// lambda→lambda_invocations (parses REPORT lines from FilterLogEvents).
		"/aws/lambda/a9s-demo-s3-notifier": minimalLogStreams("lambda-s3-notifier"),
		"/aws/lambda/acme-inbound-parser":  minimalLogStreams("lambda-inbound-parser"),
		"/aws/lambda/orders-projector":     minimalLogStreams("lambda-orders-projector"),
		// ecs-svc→ecs_svc_logs renders "container/task-id" from the stream
		// name, which only reads on the "<prefix>/<container>/<task>" shape
		// the awslogs driver writes (core/aws/ecs_svc_logs.go). The
		// api-gateway:12 definition's container is api.
		"/ecs/api-gateway": {
			{
				LogStreamName:       aws.String("ecs/api/4f7c1a9e2b6d4f08b1c35a7e9d240c6f"),
				CreationTime:        aws.Int64(1774253700000),
				FirstEventTimestamp: aws.Int64(1774253700000),
				LastEventTimestamp:  aws.Int64(1774253730000),
				StoredBytes:         aws.Int64(8192),
			},
		},
		// The stream prefix is the container's name, not the family's: web
		// and worker are what web-frontend:8 and order-worker:5 declare.
		"/ecs/web-frontend": {
			{
				LogStreamName:       aws.String("ecs/web/8b2e4d6a0c1f47539ae6b2d8c04f1e73"),
				CreationTime:        aws.Int64(1774253600000),
				FirstEventTimestamp: aws.Int64(1774253600000),
				LastEventTimestamp:  aws.Int64(1774253640000),
				StoredBytes:         aws.Int64(6144),
			},
		},
		"/ecs/order-worker": {
			{
				LogStreamName:       aws.String("ecs/worker/1d3f5a7b9c2e46088f4a6c8e0b2d4f69"),
				CreationTime:        aws.Int64(1774253500000),
				FirstEventTimestamp: aws.Int64(1774253500000),
				LastEventTimestamp:  aws.Int64(1774253560000),
				StoredBytes:         aws.Int64(4096),
			},
		},
		"/aws/codebuild/acme-api-build": {
			{
				LogStreamName:       aws.String("build-142/acme-api-build"),
				CreationTime:        aws.Int64(1774149300000),
				FirstEventTimestamp: aws.Int64(1774149300000),
				LastEventTimestamp:  aws.Int64(1774149430000),
				StoredBytes:         aws.Int64(6144),
			},
		},
		transferLogGroupPrimaryName:      minimalLogStreams("transfer-as2-gateway"),
		transferLogGroupPartnerAuditName: minimalLogStreams("transfer-partner-audit"),
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
			{
				Timestamp: aws.Int64(1774253400000),
				// The only demo event in classifyLogEventStatus's warn class
				// (core/aws/log_events.go), so the only carrier of the cwlogs.log-warn
				// finding.
				Message:       aws.String("WARN Token cache miss for issuer https://auth.acme-corp.example; falling back to a full JWKS fetch, added 180 ms to this authorization"),
				IngestionTime: aws.Int64(1774253400100),
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
				Timestamp: aws.Int64(1774253400000),
				// Status: timeout — the only demo TIMEOUT REPORT line, which drives
				// lambda_invocations' Findings-based coloring (lambdaInvocationFindings,
				// core/aws/lambda_invocations.go).
				Message:       aws.String("REPORT RequestId: ord-897 Duration: 67.42 ms Billed Duration: 68 ms Memory Size: 128 MB Max Memory Used: 72 MB Status: timeout"),
				IngestionTime: aws.Int64(1774253400100),
			},
			{
				Timestamp:     aws.Int64(1774253300000),
				Message:       aws.String("REPORT RequestId: " + LambdaFailedRequestID + " Duration: 812.33 ms Billed Duration: 813 ms Memory Size: 128 MB Max Memory Used: 128 MB Status: error Error Type: Runtime.OutOfMemory"),
				IngestionTime: aws.Int64(1774253300100),
			},
		},
	}

	// Lambda REPORT lines for the three graph-root-drilled functions so
	// lambda→lambda_invocations parses at least one invocation each.
	lambdaInvocationReport := func(functionName string) []cwlogstypes.OutputLogEvent {
		base := int64(1774253700000)
		return []cwlogstypes.OutputLogEvent{
			{
				Timestamp:     aws.Int64(base),
				Message:       aws.String("START RequestId: " + functionName + "-req-001 Version: $LATEST"),
				IngestionTime: aws.Int64(base + 100),
			},
			{
				Timestamp:     aws.Int64(base + 1000),
				Message:       aws.String("INFO " + functionName + " processing event"),
				IngestionTime: aws.Int64(base + 1100),
			},
			{
				Timestamp:     aws.Int64(base + 2000),
				Message:       aws.String("REPORT RequestId: " + functionName + "-req-001 Duration: 82.50 ms Billed Duration: 83 ms Memory Size: 256 MB Max Memory Used: 94 MB"),
				IngestionTime: aws.Int64(base + 2100),
			},
			{
				Timestamp:     aws.Int64(base - 60000),
				Message:       aws.String("REPORT RequestId: " + functionName + "-req-000 Duration: 110.20 ms Billed Duration: 111 ms Memory Size: 256 MB Max Memory Used: 98 MB Init Duration: 245.18 ms"),
				IngestionTime: aws.Int64(base - 59900),
			},
		}
	}
	// The build-142 stream of acme-api-build, reached through cb→cb_builds→
	// cb_build_logs. One line of each class classifyBuildLogStatus
	// (core/aws/cb_build_logs.go) recognises, so no class of the build log is
	// demoed only as the unclassified default.
	logEvents["/aws/codebuild/acme-api-build"] = []cwlogstypes.OutputLogEvent{
		{
			Timestamp:     aws.Int64(1774149300000),
			Message:       aws.String("[Container] 2026/03/22 03:15:00 Entering phase INSTALL"),
			IngestionTime: aws.Int64(1774149300100),
		},
		{
			Timestamp:     aws.Int64(1774149310000),
			Message:       aws.String("[Container] 2026/03/22 03:15:10 Running command npm ci --no-audit"),
			IngestionTime: aws.Int64(1774149310100),
		},
		{
			Timestamp:     aws.Int64(1774149365000),
			Message:       aws.String("[Container] 2026/03/22 03:16:05 Phase complete: INSTALL State: SUCCEEDED"),
			IngestionTime: aws.Int64(1774149365100),
		},
		{
			Timestamp:     aws.Int64(1774149420000),
			Message:       aws.String("[Container] 2026/03/22 03:17:00 ERROR: unit suite did not exit successfully: 3 assertions failed in orders/pricing_test.js"),
			IngestionTime: aws.Int64(1774149420100),
		},
		{
			Timestamp:     aws.Int64(1774149430000),
			Message:       aws.String("[Container] 2026/03/22 03:17:10 Uploading artifacts to s3://acme-build-artifacts/acme-api-build/142"),
			IngestionTime: aws.Int64(1774149430100),
		},
		// A retry loop writes the same line twice inside one millisecond.
		// CloudWatch Logs keeps both, and GetLogEvents gives neither an id,
		// so the two are told apart by their order in the response.
		{
			Timestamp:     aws.Int64(BuildLogRepeatedLineAt),
			Message:       aws.String(BuildLogRepeatedLine),
			IngestionTime: aws.Int64(BuildLogRepeatedLineAt + 90),
		},
		{
			Timestamp:     aws.Int64(BuildLogRepeatedLineAt),
			Message:       aws.String(BuildLogRepeatedLine),
			IngestionTime: aws.Int64(BuildLogRepeatedLineAt + 90),
		},
	}

	// Where the api-gateway ECS task definition's containers write
	// (awslogs-group "/ecs/<family>", core/demo/fixtures/ecs.go), reached
	// through ecs-svc→ecs_svc_logs.
	logEvents["/ecs/api-gateway"] = []cwlogstypes.OutputLogEvent{
		{
			Timestamp:     aws.Int64(1774253700000),
			Message:       aws.String("INFO  [http] GET /v1/orders/7842 200 in 31ms"),
			IngestionTime: aws.Int64(1774253700100),
		},
		{
			Timestamp:     aws.Int64(1774253710000),
			Message:       aws.String("WARN  [upstream] pricing-service responded in 1840ms, above the 1500ms budget; served from the stale cache"),
			IngestionTime: aws.Int64(1774253710100),
		},
		{
			Timestamp:     aws.Int64(1774253720000),
			Message:       aws.String("ERROR [http] POST /v1/orders 502 upstream pricing-service refused the connection"),
			IngestionTime: aws.Int64(1774253720100),
		},
		{
			Timestamp:     aws.Int64(1774253730000),
			Message:       aws.String("INFO  [http] GET /v1/health 200 in 2ms"),
			IngestionTime: aws.Int64(1774253730100),
		},
	}

	// Four weeks of access lines before the four above: more than the 200 the
	// service log view shows, so it visibly opens on the newest and Load More
	// reaches back.
	for i := range ECSAPIGatewayHistoryLines {
		at := int64(1774253700000) - int64(ECSAPIGatewayHistoryLines-i)*3*3600*1000
		logEvents["/ecs/api-gateway"] = append(logEvents["/ecs/api-gateway"], cwlogstypes.OutputLogEvent{
			Timestamp:     aws.Int64(at),
			Message:       aws.String(fmt.Sprintf("INFO  [http] GET /v1/orders/%d 200 in %dms", 7000+i, 12+i%40)),
			IngestionTime: aws.Int64(at + 100),
		})
	}

	logEvents["/ecs/web-frontend"] = []cwlogstypes.OutputLogEvent{
		{
			Timestamp:     aws.Int64(1774253600000),
			Message:       aws.String("INFO  [server] listening on 0.0.0.0:3000"),
			IngestionTime: aws.Int64(1774253600100),
		},
		{
			Timestamp:     aws.Int64(1774253620000),
			Message:       aws.String("WARN  [assets] bundle main.js is 2.4MB, above the 1MB budget"),
			IngestionTime: aws.Int64(1774253620100),
		},
		{
			Timestamp:     aws.Int64(1774253640000),
			Message:       aws.String("INFO  [http] GET /checkout 200 in 44ms"),
			IngestionTime: aws.Int64(1774253640100),
		},
	}

	logEvents["/ecs/order-worker"] = []cwlogstypes.OutputLogEvent{
		{
			Timestamp:     aws.Int64(1774253500000),
			Message:       aws.String("INFO  [queue] claimed 12 orders from acme-orders-queue"),
			IngestionTime: aws.Int64(1774253500100),
		},
		{
			Timestamp:     aws.Int64(1774253530000),
			Message:       aws.String("ERROR [queue] order 7842 failed after 3 attempts, sent to acme-orders-dlq"),
			IngestionTime: aws.Int64(1774253530100),
		},
		{
			Timestamp:     aws.Int64(1774253560000),
			Message:       aws.String("INFO  [queue] 11 orders settled in 58s"),
			IngestionTime: aws.Int64(1774253560100),
		},
	}

	logEvents["/aws/lambda/a9s-demo-s3-notifier"] = lambdaInvocationReport("a9s-demo-s3-notifier")
	logEvents["/aws/lambda/acme-inbound-parser"] = lambdaInvocationReport("acme-inbound-parser")
	logEvents["/aws/lambda/orders-projector"] = lambdaInvocationReport("orders-projector")

	// Every text-format invocation carries its START and END lines beside its
	// REPORT, as a runtime writes them; one a fixture leaves out is written a
	// second before the REPORT (START) or a millisecond before it (END).
	for group, events := range logEvents {
		if !strings.HasPrefix(group, "/aws/lambda/") {
			continue
		}
		has := map[string]bool{}
		for _, e := range events {
			has[aws.ToString(e.Message)] = true
		}
		for _, e := range events {
			rid, ok := strings.CutPrefix(aws.ToString(e.Message), "REPORT RequestId: ")
			if !ok {
				continue
			}
			rid, _, _ = strings.Cut(rid, " ")
			at := aws.ToInt64(e.Timestamp)
			if start := "START RequestId: " + rid + " Version: $LATEST"; !has[start] {
				logEvents[group] = append(logEvents[group], cwlogstypes.OutputLogEvent{Timestamp: aws.Int64(at - 1000), Message: aws.String(start), IngestionTime: aws.Int64(at - 900)})
			}
			if end := "END RequestId: " + rid; !has[end] {
				logEvents[group] = append(logEvents[group], cwlogstypes.OutputLogEvent{Timestamp: aws.Int64(at - 1), Message: aws.String(end), IngestionTime: aws.Int64(at + 99)})
			}
		}
	}

	// The invocation list reads the last 24 hours, so every Lambda group's
	// events are moved to end five minutes before the demo starts, keeping
	// their spacing.
	var lambdaNewest int64
	for group, events := range logEvents {
		if strings.HasPrefix(group, "/aws/lambda/") {
			for _, e := range events {
				lambdaNewest = max(lambdaNewest, aws.ToInt64(e.Timestamp))
			}
		}
	}
	shift := time.Now().Add(-5*time.Minute).UnixMilli() - lambdaNewest
	for group, events := range logEvents {
		if strings.HasPrefix(group, "/aws/lambda/") {
			for i := range events {
				events[i].Timestamp = aws.Int64(aws.ToInt64(events[i].Timestamp) + shift)
				events[i].IngestionTime = aws.Int64(aws.ToInt64(events[i].IngestionTime) + shift)
			}
		}
	}

	// LambdaJSONLogFormat writes the JSON log format: each system log event is
	// {"time", "type", "record"} and each application log line
	// {"timestamp", "level", "message", "requestId"}
	// (docs.aws.amazon.com/lambda/latest/dg/monitoring-cloudwatchlogs-logformat.html).
	// LambdaJSONTimeoutRequestID's report carries status timeout, and a
	// traceback its runtime printed without the request ID sits between its
	// platform.start and platform.report. Its events take the same shift as
	// the other Lambda groups', so the times inside the records agree with
	// the events'.
	jsonLine := func(at int64, line string) cwlogstypes.OutputLogEvent {
		return cwlogstypes.OutputLogEvent{Timestamp: aws.Int64(at + shift), Message: aws.String(line), IngestionTime: aws.Int64(at + shift + 100)}
	}
	jsonInvocation := func(rid string, at, span int64, status, report string, extra ...cwlogstypes.OutputLogEvent) []cwlogstypes.OutputLogEvent {
		t := func(ms int64) string { return time.UnixMilli(ms + shift).UTC().Format("2006-01-02T15:04:05.000Z") }
		events := []cwlogstypes.OutputLogEvent{
			jsonLine(at, `{"time":"`+t(at)+`","type":"platform.start","record":{"requestId":"`+rid+`","version":"$LATEST"}}`),
			jsonLine(at+5, `{"timestamp":"`+t(at+5)+`","level":"INFO","message":"transforming batch s3://acme-raw-events/2026/03/22/","requestId":"`+rid+`"}`),
		}
		events = append(events, extra...)
		end := at + span
		return append(events,
			jsonLine(end, `{"time":"`+t(end)+`","type":"platform.runtimeDone","record":{"requestId":"`+rid+`","status":"`+status+`"}}`),
			jsonLine(end+1, `{"time":"`+t(end+1)+`","type":"platform.report","record":{"requestId":"`+rid+`","status":"`+status+`","metrics":{`+report+`}}}`),
		)
	}
	logEvents["/aws/lambda/"+LambdaJSONLogFormat] = slices.Concat(
		jsonInvocation("d7a1c3e5-0b2d-4f6a-8c9e-1a3b5c7d9e01", 1774252800000, 2413, "success",
			`"durationMs":2412.55,"billedDurationMs":2413,"memorySizeMB":512,"maxMemoryUsedMB":211,"initDurationMs":842.17`),
		jsonInvocation(LambdaJSONTimeoutRequestID, 1774252900000, 300000, "timeout",
			`"durationMs":300000.00,"billedDurationMs":300000,"memorySizeMB":512,"maxMemoryUsedMB":498`,
			jsonLine(1774252900900, "Traceback (most recent call last):\n  File \"/var/task/transform.py\", line 88, in lambda_handler\n    rows = fetch_partition(event)\nTimeoutError: read from s3://acme-raw-events timed out"),
		),
		jsonInvocation("d7a1c3e5-0b2d-4f6a-8c9e-1a3b5c7d9e03", 1774253315000, 1874, "success",
			`"durationMs":1873.40,"billedDurationMs":1874,"memorySizeMB":512,"maxMemoryUsedMB":230`),
	)

	// /aws-glue/jobs/output streams to Kinesis for real-time monitoring.
	subscriptionFilters := map[string][]cwlogstypes.SubscriptionFilter{
		"/aws-glue/jobs/output": {
			{
				FilterName:     aws.String("stream-to-audit-log"),
				LogGroupName:   aws.String("/aws-glue/jobs/output"),
				FilterPattern:  aws.String(""),
				DestinationArn: aws.String("arn:aws:kinesis:us-east-1:123456789012:stream/audit-log-stream"),
				Distribution:   cwlogstypes.DistributionByLogStream,
				CreationTime:   aws.Int64(1715731200000),
			},
		},
	}

	// /aws-glue/jobs/error was exported to the logs bucket.
	exportTasks := []cwlogstypes.ExportTask{{
		TaskId:            aws.String("e570a1b2-0000-4000-8000-000000000001"),
		TaskName:          aws.String("glue-errors-2026-03"),
		LogGroupName:      aws.String("/aws-glue/jobs/error"),
		From:              aws.Int64(1772323200000),
		To:                aws.Int64(1774915200000),
		Destination:       aws.String(LogsBucketName),
		DestinationPrefix: aws.String("exports/glue-errors"),
		Status:            &cwlogstypes.ExportTaskStatus{Code: cwlogstypes.ExportTaskStatusCodeCompleted, Message: aws.String("Completed successfully")},
		ExecutionInfo:     &cwlogstypes.ExportTaskExecutionInfo{CreationTime: aws.Int64(1774915260000), CompletionTime: aws.Int64(1774915500000)},
	}}

	metricFilters := map[string][]cwlogstypes.MetricFilter{
		OrphanOldLogGroupName: {
			{
				FilterName:    aws.String("orphan-old-error-count"),
				LogGroupName:  aws.String(OrphanOldLogGroupName),
				FilterPattern: aws.String("ERROR"),
				CreationTime:  aws.Int64(1715731200000),
				MetricTransformations: []cwlogstypes.MetricTransformation{{
					MetricName:      aws.String(OrphanOldMetricName),
					MetricNamespace: aws.String(OrphanOldMetricNamespace),
					MetricValue:     aws.String("1"),
				}},
			},
		},
	}

	return &CWLogsFixtures{
		LogGroups:           logGroups,
		LogStreams:          logStreams,
		LogEvents:           logEvents,
		SubscriptionFilters: subscriptionFilters,
		ExportTasks:         exportTasks,
		MetricFilters:       metricFilters,
		EventStreams: map[string]string{
			"/aws/lambda/api-gateway-authorizer": "2026/03/22/[$LATEST]abc123",
			"/aws/lambda/process-orders":         "2026/03/22/[$LATEST]ord789",
		},
	}
})

// LogGroupSecondPageOnly is the one log group the first page does not carry.
// It is the target of no pivot, so the split costs no count.
const LogGroupSecondPageOnly = "/app/archive/2019-batch-export"

// LogGroupsPageSize splits the log-group fixtures across two pages, making this
// the one paginated demo list: the source of the "+" a pivot renders when it
// matched inside a list that was read only in part. It is one less than the
// number of groups, the largest split that still truncates, so only
// LogGroupSecondPageOnly is off page one and no pivot loses a count.
//
// A group appended after LogGroupSecondPageOnly would land on page two with it
// and take its pivot's count down with it; append before it instead, and raise
// this number in step.
const LogGroupsPageSize = 182

// derivedLogGroups returns a log group for every one the other demo fixtures
// name and have does not hold yet: each MWAA environment's component groups,
// each Redshift cluster's CloudWatch audit exports, the MSK broker log group,
// the Athena Spark workgroup's group, the WAF log group, and the Secrets Manager rotation
// functions' groups. Deriving them from the fixtures that name them keeps
// every related log-group row a row of this list.
func derivedLogGroups(have []cwlogstypes.LogGroup) []cwlogstypes.LogGroup {
	seen := make(map[string]bool, len(have))
	for _, g := range have {
		seen[aws.ToString(g.LogGroupName)] = true
	}
	var names []string
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	envs := NewMWAAFixtures().Environments
	for _, env := range slices.Sorted(maps.Keys(envs)) {
		for _, component := range []string{"DAGProcessing", "Scheduler", "WebServer", "Worker", "Task"} {
			add(mwaaLogGroupName(env, component))
		}
	}
	for _, c := range NewRedshiftFixtures().Clusters {
		id := aws.ToString(c.ClusterIdentifier)
		for _, export := range RedshiftCloudWatchLogExports(id) {
			add("/aws/redshift/cluster/" + id + "/" + export)
		}
	}
	add(MSKBrokerLogGroup)
	add(AthenaSparkLogGroup)
	add(WAFProdAPILogGroup)
	for _, fn := range []string{"rotate-api-key", "rotate-rds-credentials", "rotate-docdb-credentials"} {
		add("/aws/lambda/" + fn)
	}
	groups := make([]cwlogstypes.LogGroup, 0, len(names))
	for _, name := range names {
		groups = append(groups, cwlogstypes.LogGroup{
			LogGroupName:    aws.String(name),
			Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + name + ":*"),
			StoredBytes:     aws.Int64(1048576),
			RetentionInDays: aws.Int32(30),
			CreationTime:    aws.Int64(1756704000000),
		})
	}
	return groups
}

func NewCWLogsFixtures() *CWLogsFixtures {
	return sharedCWLogsFixtures()
}

// minimalLogStreams returns one canonical "2026/04/20/[$LATEST]<suffix>"
// log stream so logs→log_streams drill lands on non-empty content for any
// graph-root-reachable log group.
func minimalLogStreams(suffix string) []cwlogstypes.LogStream {
	ts := int64(1774253700000)
	return []cwlogstypes.LogStream{
		{
			LogStreamName:       aws.String("2026/04/20/[$LATEST]" + suffix),
			CreationTime:        aws.Int64(ts),
			FirstEventTimestamp: aws.Int64(ts),
			LastEventTimestamp:  aws.Int64(ts + 60000),
			StoredBytes:         aws.Int64(1024),
		},
	}
}

// demoLogsKMSKeyARN is the synthetic customer key every demo log group but
// LogGroupNoKMS carries.
const demoLogsKMSKeyARN = "arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-5678-90ab-cdef-111111111111"

// ECSExecLogGroup is where the acme-services cluster's ecs exec session
// transcripts are written; the cluster names it in its
// ExecuteCommandConfiguration (ecs.go).
const ECSExecLogGroup = "/ecs/exec/acme-services"

// LogGroupNoKMS is the ONE demo log group with no KMS key.
// Every other log group fixture carries a synthetic KmsKeyId so the demo
// bench shows exactly one row for logs.no-kms.
const LogGroupNoKMS = "/app/acme-unencrypted-audit"

func init() {
	Register(Pin{ShortName: "logs", Rows: 182, Issues: 3, Truncated: true, CoverageGaps: []string{"broken", "dim"}})
}
