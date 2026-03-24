package demo

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["alarm"] = cloudwatchAlarmFixtures
	demoData["logs"] = cloudwatchLogGroupFixtures

	RegisterChildDemo("log_streams", func(parentCtx map[string]string) []resource.Resource {
		return logStreamFixtures(parentCtx["log_group_name"])
	})
	RegisterChildDemo("log_events", func(parentCtx map[string]string) []resource.Resource {
		return logEventFixtures(parentCtx["log_group_name"], parentCtx["log_stream_name"])
	})
	RegisterChildDemo("alarm_history", func(parentCtx map[string]string) []resource.Resource {
		return alarmHistoryFixtures(parentCtx["alarm_name"])
	})
	RegisterChildDemo("lambda_invocations", func(parentCtx map[string]string) []resource.Resource {
		return lambdaInvocationFixtures(parentCtx["function_name"])
	})
	RegisterChildDemo("lambda_invocation_logs", func(parentCtx map[string]string) []resource.Resource {
		return lambdaInvocationLogFixtures(parentCtx["log_group"], parentCtx["request_id"])
	})
}

// cloudwatchAlarmFixtures returns demo CloudWatch alarm fixtures.
// Field keys: alarm_name, state, metric_name, namespace, threshold
func cloudwatchAlarmFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "api-high-error-rate",
			Name:   "api-high-error-rate",
			Status: "OK",
			Fields: map[string]string{
				"alarm_name":  "api-high-error-rate",
				"state":       "OK",
				"metric_name": "5XXError",
				"namespace":   "AWS/ApiGateway",
				"threshold":   "5.00",
			},
			RawStruct: cwtypes.MetricAlarm{
				AlarmName:          aws.String("api-high-error-rate"),
				AlarmArn:           aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:api-high-error-rate"),
				AlarmDescription:   aws.String("Triggers when API 5XX error rate exceeds 5%"),
				StateValue:         cwtypes.StateValueOk,
				MetricName:         aws.String("5XXError"),
				Namespace:          aws.String("AWS/ApiGateway"),
				Threshold:          aws.Float64(5.0),
				ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:  aws.Int32(3),
				Period:             aws.Int32(300),
				Statistic:          cwtypes.StatisticAverage,
				ActionsEnabled:     aws.Bool(true),
				AlarmActions: []string{
					"arn:aws:sns:us-east-1:123456789012:ops-alerts",
				},
			},
		},
		{
			ID:     "rds-cpu-utilization",
			Name:   "rds-cpu-utilization",
			Status: "OK",
			Fields: map[string]string{
				"alarm_name":  "rds-cpu-utilization",
				"state":       "OK",
				"metric_name": "CPUUtilization",
				"namespace":   "AWS/RDS",
				"threshold":   "80.00",
			},
			RawStruct: cwtypes.MetricAlarm{
				AlarmName:          aws.String("rds-cpu-utilization"),
				AlarmArn:           aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:rds-cpu-utilization"),
				AlarmDescription:   aws.String("Triggers when RDS CPU exceeds 80%"),
				StateValue:         cwtypes.StateValueOk,
				MetricName:         aws.String("CPUUtilization"),
				Namespace:          aws.String("AWS/RDS"),
				Threshold:          aws.Float64(80.0),
				ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:  aws.Int32(5),
				Period:             aws.Int32(60),
				Statistic:          cwtypes.StatisticAverage,
				ActionsEnabled:     aws.Bool(true),
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("DBInstanceIdentifier"), Value: aws.String("prod-api-primary")},
				},
			},
		},
		{
			ID:     "lambda-errors-critical",
			Name:   "lambda-errors-critical",
			Status: "ALARM",
			Fields: map[string]string{
				"alarm_name":  "lambda-errors-critical",
				"state":       "ALARM",
				"metric_name": "Errors",
				"namespace":   "AWS/Lambda",
				"threshold":   "10.00",
			},
			RawStruct: cwtypes.MetricAlarm{
				AlarmName:          aws.String("lambda-errors-critical"),
				AlarmArn:           aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:lambda-errors-critical"),
				AlarmDescription:   aws.String("Critical: Lambda error count exceeds 10"),
				StateValue:         cwtypes.StateValueAlarm,
				MetricName:         aws.String("Errors"),
				Namespace:          aws.String("AWS/Lambda"),
				Threshold:          aws.Float64(10.0),
				ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:  aws.Int32(1),
				Period:             aws.Int32(300),
				Statistic:          cwtypes.StatisticSum,
				ActionsEnabled:     aws.Bool(true),
				AlarmActions: []string{
					"arn:aws:sns:us-east-1:123456789012:ops-critical",
				},
			},
		},
		{
			ID:     "elb-unhealthy-hosts",
			Name:   "elb-unhealthy-hosts",
			Status: "INSUFFICIENT_DATA",
			Fields: map[string]string{
				"alarm_name":  "elb-unhealthy-hosts",
				"state":       "INSUFFICIENT_DATA",
				"metric_name": "UnHealthyHostCount",
				"namespace":   "AWS/ApplicationELB",
				"threshold":   "1.00",
			},
			RawStruct: cwtypes.MetricAlarm{
				AlarmName:          aws.String("elb-unhealthy-hosts"),
				AlarmArn:           aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:elb-unhealthy-hosts"),
				AlarmDescription:   aws.String("Triggers when any target group has unhealthy hosts"),
				StateValue:         cwtypes.StateValueInsufficientData,
				MetricName:         aws.String("UnHealthyHostCount"),
				Namespace:          aws.String("AWS/ApplicationELB"),
				Threshold:          aws.Float64(1.0),
				ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:  aws.Int32(2),
				Period:             aws.Int32(60),
				Statistic:          cwtypes.StatisticMaximum,
				ActionsEnabled:     aws.Bool(true),
			},
		},
		{
			ID:     "disk-space-warning",
			Name:   "disk-space-warning",
			Status: "OK",
			Fields: map[string]string{
				"alarm_name":  "disk-space-warning",
				"state":       "OK",
				"metric_name": "DiskSpaceUtilization",
				"namespace":   "CWAgent",
				"threshold":   "85.00",
			},
			RawStruct: cwtypes.MetricAlarm{
				AlarmName:          aws.String("disk-space-warning"),
				AlarmArn:           aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:disk-space-warning"),
				AlarmDescription:   aws.String("Warning when disk space exceeds 85%"),
				StateValue:         cwtypes.StateValueOk,
				MetricName:         aws.String("DiskSpaceUtilization"),
				Namespace:          aws.String("CWAgent"),
				Threshold:          aws.Float64(85.0),
				ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:  aws.Int32(3),
				Period:             aws.Int32(300),
				Statistic:          cwtypes.StatisticAverage,
				ActionsEnabled:     aws.Bool(true),
			},
		},
	}
}

// cloudwatchLogGroupFixtures returns demo CloudWatch log group fixtures.
// Field keys: log_group_name, stored_bytes, retention_days, creation_time
func cloudwatchLogGroupFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "/aws/lambda/api-gateway-authorizer",
			Name:   "/aws/lambda/api-gateway-authorizer",
			Status: "",
			Fields: map[string]string{
				"log_group_name": "/aws/lambda/api-gateway-authorizer",
				"stored_bytes":   "52428800",
				"retention_days": "30",
				"creation_time":  "1704067200000",
			},
			RawStruct: cwlogstypes.LogGroup{
				LogGroupName: aws.String("/aws/lambda/api-gateway-authorizer"),
				Arn:          aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/api-gateway-authorizer:*"),
				StoredBytes:  aws.Int64(52428800),
				RetentionInDays: aws.Int32(30),
				CreationTime: aws.Int64(1704067200000),
			},
		},
		{
			ID:     "/aws/eks/acme-prod/cluster",
			Name:   "/aws/eks/acme-prod/cluster",
			Status: "",
			Fields: map[string]string{
				"log_group_name": "/aws/eks/acme-prod/cluster",
				"stored_bytes":   "1073741824",
				"retention_days": "90",
				"creation_time":  "1700000000000",
			},
			RawStruct: cwlogstypes.LogGroup{
				LogGroupName: aws.String("/aws/eks/acme-prod/cluster"),
				Arn:          aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/eks/acme-prod/cluster:*"),
				StoredBytes:  aws.Int64(1073741824),
				RetentionInDays: aws.Int32(90),
				CreationTime: aws.Int64(1700000000000),
			},
		},
		{
			ID:     "/aws/rds/instance/prod-api-primary/postgresql",
			Name:   "/aws/rds/instance/prod-api-primary/postgresql",
			Status: "",
			Fields: map[string]string{
				"log_group_name": "/aws/rds/instance/prod-api-primary/postgresql",
				"stored_bytes":   "536870912",
				"retention_days": "14",
				"creation_time":  "1706745600000",
			},
			RawStruct: cwlogstypes.LogGroup{
				LogGroupName: aws.String("/aws/rds/instance/prod-api-primary/postgresql"),
				Arn:          aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/rds/instance/prod-api-primary/postgresql:*"),
				StoredBytes:  aws.Int64(536870912),
				RetentionInDays: aws.Int32(14),
				CreationTime: aws.Int64(1706745600000),
			},
		},
		{
			ID:     "/acme/application/api",
			Name:   "/acme/application/api",
			Status: "",
			Fields: map[string]string{
				"log_group_name": "/acme/application/api",
				"stored_bytes":   "2147483648",
				"retention_days": "365",
				"creation_time":  "1693526400000",
			},
			RawStruct: cwlogstypes.LogGroup{
				LogGroupName: aws.String("/acme/application/api"),
				Arn:          aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/acme/application/api:*"),
				StoredBytes:  aws.Int64(2147483648),
				RetentionInDays: aws.Int32(365),
				CreationTime: aws.Int64(1693526400000),
			},
		},
		{
			ID:     "/aws/cloudtrail",
			Name:   "/aws/cloudtrail",
			Status: "",
			Fields: map[string]string{
				"log_group_name": "/aws/cloudtrail",
				"stored_bytes":   "10737418240",
				"retention_days": "",
				"creation_time":  "1688169600000",
			},
			RawStruct: cwlogstypes.LogGroup{
				LogGroupName: aws.String("/aws/cloudtrail"),
				Arn:          aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/cloudtrail:*"),
				StoredBytes:  aws.Int64(10737418240),
				CreationTime: aws.Int64(1688169600000),
			},
		},
	}
}

// alarmHistoryFixtures returns demo alarm history fixtures for any alarm.
func alarmHistoryFixtures(_ string) []resource.Resource {
	ts1 := time.Date(2024, 3, 22, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2024, 3, 22, 10, 5, 0, 0, time.UTC)
	ts3 := time.Date(2024, 3, 22, 10, 5, 1, 0, time.UTC)
	ts4 := time.Date(2024, 3, 21, 14, 30, 0, 0, time.UTC)
	ts5 := time.Date(2024, 3, 20, 8, 0, 0, 0, time.UTC)

	return []resource.Resource{
		{
			ID:     "2024-03-22 10:00:00",
			Name:   "2024-03-22 10:00:00",
			Status: "StateUpdate",
			Fields: map[string]string{
				"timestamp":         "2024-03-22 10:00:00",
				"history_item_type": "StateUpdate",
				"history_summary":   "Alarm updated from OK to ALARM",
			},
			RawStruct: cwtypes.AlarmHistoryItem{
				AlarmName:       aws.String("api-high-error-rate"),
				AlarmType:       cwtypes.AlarmTypeMetricAlarm,
				HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
				HistorySummary:  aws.String("Alarm updated from OK to ALARM"),
				HistoryData:     aws.String(`{"version":"1.0","oldState":{"stateValue":"OK"},"newState":{"stateValue":"ALARM"}}`),
				Timestamp:       &ts1,
			},
		},
		{
			ID:     "2024-03-22 10:05:00",
			Name:   "2024-03-22 10:05:00",
			Status: "StateUpdate",
			Fields: map[string]string{
				"timestamp":         "2024-03-22 10:05:00",
				"history_item_type": "StateUpdate",
				"history_summary":   "Alarm updated from ALARM to OK",
			},
			RawStruct: cwtypes.AlarmHistoryItem{
				AlarmName:       aws.String("api-high-error-rate"),
				AlarmType:       cwtypes.AlarmTypeMetricAlarm,
				HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
				HistorySummary:  aws.String("Alarm updated from ALARM to OK"),
				HistoryData:     aws.String(`{"version":"1.0","oldState":{"stateValue":"ALARM"},"newState":{"stateValue":"OK"}}`),
				Timestamp:       &ts2,
			},
		},
		{
			ID:     "2024-03-22 10:05:01",
			Name:   "2024-03-22 10:05:01",
			Status: "Action",
			Fields: map[string]string{
				"timestamp":         "2024-03-22 10:05:01",
				"history_item_type": "Action",
				"history_summary":   "Published notification to arn:aws:sns:us-east-1:123456789012:ops-alerts",
			},
			RawStruct: cwtypes.AlarmHistoryItem{
				AlarmName:       aws.String("api-high-error-rate"),
				AlarmType:       cwtypes.AlarmTypeMetricAlarm,
				HistoryItemType: cwtypes.HistoryItemTypeAction,
				HistorySummary:  aws.String("Published notification to arn:aws:sns:us-east-1:123456789012:ops-alerts"),
				Timestamp:       &ts3,
			},
		},
		{
			ID:     "2024-03-21 14:30:00",
			Name:   "2024-03-21 14:30:00",
			Status: "ConfigurationUpdate",
			Fields: map[string]string{
				"timestamp":         "2024-03-21 14:30:00",
				"history_item_type": "ConfigurationUpdate",
				"history_summary":   "Alarm threshold changed from 10.0 to 5.0",
			},
			RawStruct: cwtypes.AlarmHistoryItem{
				AlarmName:       aws.String("api-high-error-rate"),
				AlarmType:       cwtypes.AlarmTypeMetricAlarm,
				HistoryItemType: cwtypes.HistoryItemTypeConfigurationUpdate,
				HistorySummary:  aws.String("Alarm threshold changed from 10.0 to 5.0"),
				HistoryData:     aws.String(`{"version":"1.0","type":"Update","updatedAlarm":{"threshold":5.0}}`),
				Timestamp:       &ts4,
			},
		},
		{
			ID:     "2024-03-20 08:00:00",
			Name:   "2024-03-20 08:00:00",
			Status: "StateUpdate",
			Fields: map[string]string{
				"timestamp":         "2024-03-20 08:00:00",
				"history_item_type": "StateUpdate",
				"history_summary":   "Alarm updated from INSUFFICIENT_DATA to OK",
			},
			RawStruct: cwtypes.AlarmHistoryItem{
				AlarmName:       aws.String("api-high-error-rate"),
				AlarmType:       cwtypes.AlarmTypeMetricAlarm,
				HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
				HistorySummary:  aws.String("Alarm updated from INSUFFICIENT_DATA to OK"),
				HistoryData:     aws.String(`{"version":"1.0","oldState":{"stateValue":"INSUFFICIENT_DATA"},"newState":{"stateValue":"OK"}}`),
				Timestamp:       &ts5,
			},
		},
	}
}

// logStreamFixtures returns demo log stream fixtures for any log group.
func logStreamFixtures(_ string) []resource.Resource {
	return []resource.Resource{
		{
			ID:     "2024/03/22/[$LATEST]abcdef1234567890",
			Name:   "2024/03/22/[$LATEST]abcdef1234567890",
			Status: "",
			Fields: map[string]string{
				"stream_name":  "2024/03/22/[$LATEST]abcdef1234567890",
				"last_event":   "2024-03-23 00:00",
				"first_event":  "2024-03-22 00:00",
				"stored_bytes": "14 KB",
			},
			RawStruct: cwlogstypes.LogStream{
				LogStreamName:       aws.String("2024/03/22/[$LATEST]abcdef1234567890"),
				Arn:                 aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/api:log-stream:2024/03/22/[$LATEST]abcdef1234567890"),
				FirstEventTimestamp: aws.Int64(1711065600000),
				LastEventTimestamp:  aws.Int64(1711152000000),
				StoredBytes:         aws.Int64(14336),
				CreationTime:        aws.Int64(1711060000000),
			},
		},
		{
			ID:     "2024/03/21/[$LATEST]fedcba0987654321",
			Name:   "2024/03/21/[$LATEST]fedcba0987654321",
			Status: "",
			Fields: map[string]string{
				"stream_name":  "2024/03/21/[$LATEST]fedcba0987654321",
				"last_event":   "2024-03-21 23:59",
				"first_event":  "2024-03-21 00:00",
				"stored_bytes": "2.3 MB",
			},
			RawStruct: cwlogstypes.LogStream{
				LogStreamName:       aws.String("2024/03/21/[$LATEST]fedcba0987654321"),
				Arn:                 aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/api:log-stream:2024/03/21/[$LATEST]fedcba0987654321"),
				FirstEventTimestamp: aws.Int64(1710979200000),
				LastEventTimestamp:  aws.Int64(1711065540000),
				StoredBytes:         aws.Int64(2415919),
				CreationTime:        aws.Int64(1710975000000),
			},
		},
		{
			ID:     "2024/03/20/[$LATEST]1122334455667788",
			Name:   "2024/03/20/[$LATEST]1122334455667788",
			Status: "",
			Fields: map[string]string{
				"stream_name":  "2024/03/20/[$LATEST]1122334455667788",
				"last_event":   "2024-03-20 18:30",
				"first_event":  "2024-03-20 06:00",
				"stored_bytes": "512 KB",
			},
			RawStruct: cwlogstypes.LogStream{
				LogStreamName:       aws.String("2024/03/20/[$LATEST]1122334455667788"),
				Arn:                 aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/api:log-stream:2024/03/20/[$LATEST]1122334455667788"),
				FirstEventTimestamp: aws.Int64(1710914400000),
				LastEventTimestamp:  aws.Int64(1710959400000),
				StoredBytes:         aws.Int64(524288),
				CreationTime:        aws.Int64(1710910000000),
			},
		},
	}
}

// logEventFixtures returns demo log event fixtures for any log stream.
func logEventFixtures(_, _ string) []resource.Resource {
	return []resource.Resource{
		{
			ID:     "evt-1711065600000-0",
			Name:   "START RequestId: a1b2c3d4-e5f6-7890-abcd-ef1234567890 Version: $LATEST",
			Status: "META",
			Fields: map[string]string{
				"timestamp":      "2024-03-22 00:00",
				"message":        "START RequestId: a1b2c3d4-e5f6-7890-abcd-ef1234567890 Version: $LATEST",
				"ingestion_time": "2024-03-22 00:00",
			},
			RawStruct: cwlogstypes.OutputLogEvent{
				Timestamp:     aws.Int64(1711065600000),
				Message:       aws.String("START RequestId: a1b2c3d4-e5f6-7890-abcd-ef1234567890 Version: $LATEST"),
				IngestionTime: aws.Int64(1711065601000),
			},
		},
		{
			ID:     "evt-1711065601000-1",
			Name:   "INFO Initializing database connection pool",
			Status: "",
			Fields: map[string]string{
				"timestamp":      "2024-03-22 00:00",
				"message":        "INFO Initializing database connection pool",
				"ingestion_time": "2024-03-22 00:00",
			},
			RawStruct: cwlogstypes.OutputLogEvent{
				Timestamp:     aws.Int64(1711065601000),
				Message:       aws.String("INFO Initializing database connection pool"),
				IngestionTime: aws.Int64(1711065602000),
			},
		},
		{
			ID:     "evt-1711065610000-2",
			Name:   "ERROR Failed to connect to database: connection refused",
			Status: "ERROR",
			Fields: map[string]string{
				"timestamp":      "2024-03-22 00:00",
				"message":        "ERROR Failed to connect to database: connection refused",
				"ingestion_time": "2024-03-22 00:00",
			},
			RawStruct: cwlogstypes.OutputLogEvent{
				Timestamp:     aws.Int64(1711065610000),
				Message:       aws.String("ERROR Failed to connect to database: connection refused"),
				IngestionTime: aws.Int64(1711065611000),
			},
		},
		{
			ID:     "evt-1711065620000-3",
			Name:   "WARN Retrying connection attempt 2/3",
			Status: "WARN",
			Fields: map[string]string{
				"timestamp":      "2024-03-22 00:00",
				"message":        "WARN Retrying connection attempt 2/3",
				"ingestion_time": "2024-03-22 00:00",
			},
			RawStruct: cwlogstypes.OutputLogEvent{
				Timestamp:     aws.Int64(1711065620000),
				Message:       aws.String("WARN Retrying connection attempt 2/3"),
				IngestionTime: aws.Int64(1711065621000),
			},
		},
		{
			ID:     "evt-1711065700000-4",
			Name:   "REPORT RequestId: a1b2c3d4 Duration: 1523.45 ms Billed Duration: 1524 ms Memory",
			Status: "REPORT",
			Fields: map[string]string{
				"timestamp":      "2024-03-22 00:01",
				"message":        "REPORT RequestId: a1b2c3d4 Duration: 1523.45 ms Billed Duration: 1524 ms Memory Size: 256 MB Max Memory Used: 128 MB",
				"ingestion_time": "2024-03-22 00:01",
			},
			RawStruct: cwlogstypes.OutputLogEvent{
				Timestamp:     aws.Int64(1711065700000),
				Message:       aws.String("REPORT RequestId: a1b2c3d4 Duration: 1523.45 ms Billed Duration: 1524 ms Memory Size: 256 MB Max Memory Used: 128 MB"),
				IngestionTime: aws.Int64(1711065701000),
			},
		},
	}
}

// lambdaInvocationFixtures returns demo Lambda invocation fixtures.
func lambdaInvocationFixtures(_ string) []resource.Resource {
	return []resource.Resource{
		{
			ID:     "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			Name:   "a1b2c3d4",
			Status: "OK",
			Fields: map[string]string{
				"request_id":       "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
				"timestamp":        "2024-03-22 00:00",
				"status":           "OK",
				"duration_ms":      "523 ms",
				"billed_duration_ms": "600 ms",
				"memory_size_mb":   "256",
				"memory_used_mb":   "128",
				"memory_used":      "128/256 MB",
				"init_duration_ms": "",
				"cold_start":       "no",
				"xray_trace_id":    "",
			},
			RawStruct: cwlogstypes.FilteredLogEvent{
				Timestamp: aws.Int64(1711065600000),
				Message:   aws.String("REPORT RequestId: a1b2c3d4-e5f6-7890-abcd-ef1234567890\tDuration: 523.00 ms\tBilled Duration: 600 ms\tMemory Size: 256 MB\tMax Memory Used: 128 MB\t"),
				EventId:   aws.String("evt-demo-001"),
			},
		},
		{
			ID:     "bbbbbbbb-1111-2222-3333-444444444444",
			Name:   "bbbbbbbb",
			Status: "OK",
			Fields: map[string]string{
				"request_id":       "bbbbbbbb-1111-2222-3333-444444444444",
				"timestamp":        "2024-03-22 00:05",
				"status":           "OK",
				"duration_ms":      "1250 ms",
				"billed_duration_ms": "1300 ms",
				"memory_size_mb":   "256",
				"memory_used_mb":   "200",
				"memory_used":      "200/256 MB",
				"init_duration_ms": "350 ms",
				"cold_start":       "yes",
				"xray_trace_id":    "",
			},
			RawStruct: cwlogstypes.FilteredLogEvent{
				Timestamp: aws.Int64(1711065900000),
				Message:   aws.String("REPORT RequestId: bbbbbbbb-1111-2222-3333-444444444444\tDuration: 1250.00 ms\tBilled Duration: 1300 ms\tMemory Size: 256 MB\tMax Memory Used: 200 MB\tInit Duration: 350.00 ms\t"),
				EventId:   aws.String("evt-demo-002"),
			},
		},
	}
}

// lambdaInvocationLogFixtures returns demo Lambda invocation log fixtures.
func lambdaInvocationLogFixtures(_, _ string) []resource.Resource {
	return []resource.Resource{
		{
			ID:     "log-demo-001",
			Name:   "START RequestId: a1b2c3d4-e5f6-7890-abcd-ef1234567890 Version: $LATEST",
			Status: "META",
			Fields: map[string]string{
				"timestamp": "2024-03-22 00:00",
				"message":   "START RequestId: a1b2c3d4-e5f6-7890-abcd-ef1234567890 Version: $LATEST",
			},
			RawStruct: cwlogstypes.FilteredLogEvent{
				Timestamp: aws.Int64(1711065600000),
				Message:   aws.String("START RequestId: a1b2c3d4-e5f6-7890-abcd-ef1234567890 Version: $LATEST"),
				EventId:   aws.String("log-demo-001"),
			},
		},
		{
			ID:     "log-demo-002",
			Name:   "INFO Processing request for user abc-123",
			Status: "",
			Fields: map[string]string{
				"timestamp": "2024-03-22 00:00",
				"message":   "INFO Processing request for user abc-123",
			},
			RawStruct: cwlogstypes.FilteredLogEvent{
				Timestamp: aws.Int64(1711065601000),
				Message:   aws.String("INFO Processing request for user abc-123"),
				EventId:   aws.String("log-demo-002"),
			},
		},
		{
			ID:     "log-demo-003",
			Name:   "END RequestId: a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			Status: "META",
			Fields: map[string]string{
				"timestamp": "2024-03-22 00:00",
				"message":   "END RequestId: a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			},
			RawStruct: cwlogstypes.FilteredLogEvent{
				Timestamp: aws.Int64(1711065602000),
				Message:   aws.String("END RequestId: a1b2c3d4-e5f6-7890-abcd-ef1234567890"),
				EventId:   aws.String("log-demo-003"),
			},
		},
	}
}
