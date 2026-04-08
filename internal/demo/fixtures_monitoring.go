package demo

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	demoData["alarm"] = cloudwatchAlarmFixtures
	demoData["logs"] = cloudwatchLogGroupFixtures
	demoData["ct-events"] = cloudTrailEventFixtures

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
	alarms := []resource.Resource{
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
				AlarmName:                  aws.String(relatedEC2AlarmID1),
				AlarmArn:                   aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:" + relatedEC2AlarmID1),
				AlarmDescription:           aws.String("Triggers when API 5XX error rate exceeds 5%"),
				StateValue:                 cwtypes.StateValueOk,
				StateReason:                aws.String("Threshold Crossed: 3 datapoints were less than or equal to the threshold (5.0)."),
				StateUpdatedTimestamp:      aws.Time(time.Date(2026, 3, 22, 10, 5, 0, 0, time.UTC)),
				StateTransitionedTimestamp: aws.Time(time.Date(2026, 3, 21, 10, 30, 0, 0, time.UTC)),
				MetricName:                 aws.String("5XXError"),
				Namespace:                  aws.String("AWS/ApiGateway"),
				Threshold:                  aws.Float64(5.0),
				ComparisonOperator:         cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:          aws.Int32(3),
				DatapointsToAlarm:          aws.Int32(2),
				Period:                     aws.Int32(300),
				Statistic:                  cwtypes.StatisticAverage,
				TreatMissingData:           aws.String("breaching"),
				ActionsEnabled:             aws.Bool(true),
				AlarmActions:               []string{relatedAlarmSNSID},
				OKActions:                  []string{relatedAlarmSNSID},
				InsufficientDataActions:    []string{relatedAlarmSNSID},
				// Dimensions: EC2 instance — satisfies "CW Alarms → EC2 dimensions" story.
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("InstanceId"), Value: aws.String("i-0a1b2c3d4e5f60001")},
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
				AlarmName:             aws.String(relatedEC2AlarmID2),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:" + relatedEC2AlarmID2),
				AlarmDescription:      aws.String("Triggers when RDS CPU exceeds 80%"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 5 datapoints were less than the threshold (80.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 3, 20, 8, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("CPUUtilization"),
				Namespace:             aws.String("AWS/RDS"),
				Threshold:             aws.Float64(80.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(5),
				Period:                aws.Int32(60),
				Statistic:             cwtypes.StatisticAverage,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSID},
				// Dimensions: RDS instance — satisfies "CW Alarms → RDS dimensions" story.
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
		{
			ID:     "asg-web-scale-out-cpu",
			Name:   "asg-web-scale-out-cpu",
			Status: "OK",
			Fields: map[string]string{
				"alarm_name":  "asg-web-scale-out-cpu",
				"state":       "OK",
				"metric_name": "CPUUtilization",
				"namespace":   "AWS/EC2",
				"threshold":   "70.00",
			},
			RawStruct: cwtypes.MetricAlarm{
				AlarmName:          aws.String("asg-web-scale-out-cpu"),
				AlarmArn:           aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:asg-web-scale-out-cpu"),
				AlarmDescription:   aws.String("Scale out acme-web-prod-asg when CPU exceeds 70%"),
				StateValue:         cwtypes.StateValueOk,
				MetricName:         aws.String("CPUUtilization"),
				Namespace:          aws.String("AWS/EC2"),
				Threshold:          aws.Float64(70.0),
				ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:  aws.Int32(2),
				Period:             aws.Int32(300),
				Statistic:          cwtypes.StatisticAverage,
				ActionsEnabled:     aws.Bool(true),
				AlarmActions:       []string{relatedAlarmSNSID},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("AutoScalingGroupName"), Value: aws.String("acme-web-prod-asg")},
				},
			},
		},
		{
			ID:     "docdb-cpu-utilization",
			Name:   "docdb-cpu-utilization",
			Status: "OK",
			Fields: map[string]string{
				"alarm_name":  "docdb-cpu-utilization",
				"state":       "OK",
				"metric_name": "CPUUtilization",
				"namespace":   "AWS/DocDB",
				"threshold":   "80.00",
			},
			RawStruct: cwtypes.MetricAlarm{
				AlarmName:          aws.String("docdb-cpu-utilization"),
				AlarmArn:           aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:docdb-cpu-utilization"),
				AlarmDescription:   aws.String("Triggers when DocumentDB CPU exceeds 80%"),
				StateValue:         cwtypes.StateValueOk,
				MetricName:         aws.String("CPUUtilization"),
				Namespace:          aws.String("AWS/DocDB"),
				Threshold:          aws.Float64(80.0),
				ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:  aws.Int32(5),
				Period:             aws.Int32(60),
				Statistic:          cwtypes.StatisticAverage,
				ActionsEnabled:     aws.Bool(true),
				AlarmActions:       []string{relatedAlarmSNSID},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("DBClusterIdentifier"), Value: aws.String("acme-docdb-prod")},
				},
			},
		},
		{
			ID:     relatedECSSvcAlarmID,
			Name:   relatedECSSvcAlarmID,
			Status: "ALARM",
			Fields: map[string]string{
				"alarm_name":  relatedECSSvcAlarmID,
				"state":       "ALARM",
				"metric_name": "CPUUtilization",
				"namespace":   "AWS/ECS",
				"threshold":   "80.00",
			},
			RawStruct: cwtypes.MetricAlarm{
				AlarmName:          aws.String(relatedECSSvcAlarmID),
				AlarmArn:           aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:" + relatedECSSvcAlarmID),
				AlarmDescription:   aws.String("Triggers when ECS service CPU exceeds 80%"),
				StateValue:         cwtypes.StateValueAlarm,
				MetricName:         aws.String("CPUUtilization"),
				Namespace:          aws.String("AWS/ECS"),
				Threshold:          aws.Float64(80.0),
				ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:  aws.Int32(2),
				Period:             aws.Int32(300),
				Statistic:          cwtypes.StatisticAverage,
				ActionsEnabled:     aws.Bool(true),
				AlarmActions:       []string{relatedAlarmSNSID},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("ServiceName"), Value: aws.String("api-gateway")},
					{Name: aws.String("ClusterName"), Value: aws.String("acme-services")},
				},
			},
		},
		{
			ID:     "acme-opensearch-cluster-health",
			Name:   "acme-opensearch-cluster-health",
			Status: "OK",
			Fields: map[string]string{
				"alarm_name":  "acme-opensearch-cluster-health",
				"state":       "OK",
				"metric_name": "ClusterStatus.yellow",
				"namespace":   "AWS/ES",
				"threshold":   "1.00",
			},
			RawStruct: cwtypes.MetricAlarm{
				AlarmName:          aws.String("acme-opensearch-cluster-health"),
				AlarmArn:           aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:acme-opensearch-cluster-health"),
				AlarmDescription:   aws.String("Triggers when OpenSearch cluster enters yellow health state"),
				StateValue:         cwtypes.StateValueOk,
				MetricName:         aws.String("ClusterStatus.yellow"),
				Namespace:          aws.String("AWS/ES"),
				Threshold:          aws.Float64(1.0),
				ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:  aws.Int32(1),
				Period:             aws.Int32(300),
				Statistic:          cwtypes.StatisticMaximum,
				ActionsEnabled:     aws.Bool(true),
				AlarmActions:       []string{relatedAlarmSNSID},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("DomainName"), Value: aws.String("acme-logs")},
				},
			},
		},
		{
			ID:     relatedRedisAlarmID,
			Name:   relatedRedisAlarmID,
			Status: "OK",
			Fields: map[string]string{
				"alarm_name":  relatedRedisAlarmID,
				"state":       "OK",
				"metric_name": "CPUUtilization",
				"namespace":   "AWS/ElastiCache",
				"threshold":   "80.00",
			},
			RawStruct: cwtypes.MetricAlarm{
				AlarmName:          aws.String(relatedRedisAlarmID),
				AlarmArn:           aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:" + relatedRedisAlarmID),
				AlarmDescription:   aws.String("Triggers when ElastiCache Redis CPU exceeds 80%"),
				StateValue:         cwtypes.StateValueOk,
				MetricName:         aws.String("CPUUtilization"),
				Namespace:          aws.String("AWS/ElastiCache"),
				Threshold:          aws.Float64(80.0),
				ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:  aws.Int32(5),
				Period:             aws.Int32(60),
				Statistic:          cwtypes.StatisticAverage,
				ActionsEnabled:     aws.Bool(true),
				AlarmActions:       []string{relatedAlarmSNSID},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("CacheClusterId"), Value: aws.String("acme-prod-sessions")},
				},
			},
		},
		{
			ID:     relatedRedshiftAlarmID,
			Name:   relatedRedshiftAlarmID,
			Status: "OK",
			Fields: map[string]string{
				"alarm_name":  relatedRedshiftAlarmID,
				"state":       "OK",
				"metric_name": "CPUUtilization",
				"namespace":   "AWS/Redshift",
				"threshold":   "80.00",
			},
			RawStruct: cwtypes.MetricAlarm{
				AlarmName:          aws.String(relatedRedshiftAlarmID),
				AlarmArn:           aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:" + relatedRedshiftAlarmID),
				AlarmDescription:   aws.String("Triggers when Redshift cluster CPU exceeds 80%"),
				StateValue:         cwtypes.StateValueOk,
				MetricName:         aws.String("CPUUtilization"),
				Namespace:          aws.String("AWS/Redshift"),
				Threshold:          aws.Float64(80.0),
				ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:  aws.Int32(5),
				Period:             aws.Int32(60),
				Statistic:          cwtypes.StatisticAverage,
				ActionsEnabled:     aws.Bool(true),
				AlarmActions:       []string{relatedAlarmSNSID},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("ClusterIdentifier"), Value: aws.String("acme-warehouse")},
				},
			},
		},
		{
			ID:     relatedSFNAlarmID,
			Name:   relatedSFNAlarmID,
			Status: "ALARM",
			Fields: map[string]string{
				"alarm_name":  relatedSFNAlarmID,
				"state":       "ALARM",
				"metric_name": "ExecutionsFailed",
				"namespace":   "AWS/States",
				"threshold":   "1.00",
			},
			RawStruct: cwtypes.MetricAlarm{
				AlarmName:          aws.String(relatedSFNAlarmID),
				AlarmArn:           aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:" + relatedSFNAlarmID),
				AlarmDescription:   aws.String("Triggers when Step Functions executions fail"),
				StateValue:         cwtypes.StateValueAlarm,
				MetricName:         aws.String("ExecutionsFailed"),
				Namespace:          aws.String("AWS/States"),
				Threshold:          aws.Float64(1.0),
				ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:  aws.Int32(1),
				Period:             aws.Int32(60),
				Statistic:          cwtypes.StatisticSum,
				ActionsEnabled:     aws.Bool(true),
				AlarmActions:       []string{relatedAlarmSNSID},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("StateMachineArn"), Value: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow")},
				},
			},
		},
	}

	// Generate 17 more alarms to reach 22 total
	stateMap := map[string]cwtypes.StateValue{
		"OK":                cwtypes.StateValueOk,
		"ALARM":             cwtypes.StateValueAlarm,
		"INSUFFICIENT_DATA": cwtypes.StateValueInsufficientData,
	}
	for i := 0; i < 17; i++ {
		m := alarmMetricPool[i]
		name := alarmNamePool[i]
		alarms = append(alarms, resource.Resource{
			ID:     name,
			Name:   name,
			Status: m.State,
			Fields: map[string]string{
				"alarm_name":  name,
				"state":       m.State,
				"metric_name": m.MetricName,
				"namespace":   m.Namespace,
				"threshold":   fmt.Sprintf("%.2f", m.Threshold),
			},
			RawStruct: cwtypes.MetricAlarm{
				AlarmName:          aws.String(name),
				AlarmArn:           aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:" + name),
				AlarmDescription:   aws.String(fmt.Sprintf("Auto-generated alarm for %s", name)),
				StateValue:         stateMap[m.State],
				MetricName:         aws.String(m.MetricName),
				Namespace:          aws.String(m.Namespace),
				Threshold:          aws.Float64(m.Threshold),
				ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:  aws.Int32(3),
				Period:             aws.Int32(300),
				Statistic:          cwtypes.StatisticAverage,
				ActionsEnabled:     aws.Bool(true),
			},
		})
	}

	return alarms
}

// cloudwatchLogGroupFixtures returns demo CloudWatch log group fixtures.
// Field keys: log_group_name, stored_bytes, retention_days, creation_time
func cloudwatchLogGroupFixtures() []resource.Resource {
	logGroups := []resource.Resource{
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
		},
		{
			ID:     "/aws/lambda/process-orders",
			Name:   "/aws/lambda/process-orders",
			Status: "",
			Fields: map[string]string{
				"log_group_name": "/aws/lambda/process-orders",
				"stored_bytes":   "73400320",
				"retention_days": "30",
				"creation_time":  "1705067200000",
			},
			RawStruct: cwlogstypes.LogGroup{
				LogGroupName:    aws.String("/aws/lambda/process-orders"),
				Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/process-orders:*"),
				StoredBytes:     aws.Int64(73400320),
				RetentionInDays: aws.Int32(30),
				CreationTime:    aws.Int64(1705067200000),
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
				LogGroupName:    aws.String("/aws/eks/acme-prod/cluster"),
				Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/eks/acme-prod/cluster:*"),
				StoredBytes:     aws.Int64(1073741824),
				RetentionInDays: aws.Int32(90),
				CreationTime:    aws.Int64(1700000000000),
			},
		},
		{
			ID:     "/aws/eks/acme-staging/cluster",
			Name:   "/aws/eks/acme-staging/cluster",
			Status: "",
			Fields: map[string]string{
				"log_group_name": "/aws/eks/acme-staging/cluster",
				"stored_bytes":   "268435456",
				"retention_days": "30",
				"creation_time":  "1702000000000",
			},
			RawStruct: cwlogstypes.LogGroup{
				LogGroupName:    aws.String("/aws/eks/acme-staging/cluster"),
				Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/eks/acme-staging/cluster:*"),
				StoredBytes:     aws.Int64(268435456),
				RetentionInDays: aws.Int32(30),
				CreationTime:    aws.Int64(1702000000000),
			},
		},
		{
			ID:     "/aws/eks/acme-dev/cluster",
			Name:   "/aws/eks/acme-dev/cluster",
			Status: "",
			Fields: map[string]string{
				"log_group_name": "/aws/eks/acme-dev/cluster",
				"stored_bytes":   "134217728",
				"retention_days": "14",
				"creation_time":  "1703000000000",
			},
			RawStruct: cwlogstypes.LogGroup{
				LogGroupName:    aws.String("/aws/eks/acme-dev/cluster"),
				Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/eks/acme-dev/cluster:*"),
				StoredBytes:     aws.Int64(134217728),
				RetentionInDays: aws.Int32(14),
				CreationTime:    aws.Int64(1703000000000),
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
				LogGroupName:    aws.String("/aws/rds/instance/prod-api-primary/postgresql"),
				Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/rds/instance/prod-api-primary/postgresql:*"),
				StoredBytes:     aws.Int64(536870912),
				RetentionInDays: aws.Int32(14),
				CreationTime:    aws.Int64(1706745600000),
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
				LogGroupName:    aws.String("/acme/application/api"),
				Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/acme/application/api:*"),
				StoredBytes:     aws.Int64(2147483648),
				RetentionInDays: aws.Int32(365),
				CreationTime:    aws.Int64(1693526400000),
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
		{
			ID:     "/aws/docdb/acme-docdb-prod/profiler",
			Name:   "/aws/docdb/acme-docdb-prod/profiler",
			Status: "",
			Fields: map[string]string{
				"log_group_name": "/aws/docdb/acme-docdb-prod/profiler",
				"stored_bytes":   "209715200",
				"retention_days": "14",
				"creation_time":  "1706745600000",
			},
			RawStruct: cwlogstypes.LogGroup{
				LogGroupName:    aws.String("/aws/docdb/acme-docdb-prod/profiler"),
				Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/docdb/acme-docdb-prod/profiler:*"),
				StoredBytes:     aws.Int64(209715200),
				RetentionInDays: aws.Int32(14),
				CreationTime:    aws.Int64(1706745600000),
			},
		},
		{
			ID:     "/aws/elasticbeanstalk/acme-prod-api",
			Name:   "/aws/elasticbeanstalk/acme-prod-api",
			Status: "",
			Fields: map[string]string{
				"log_group_name": "/aws/elasticbeanstalk/acme-prod-api",
				"stored_bytes":   "104857600",
				"retention_days": "30",
				"creation_time":  "1707350400000",
			},
			RawStruct: cwlogstypes.LogGroup{
				LogGroupName:    aws.String("/aws/elasticbeanstalk/acme-prod-api"),
				Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/elasticbeanstalk/acme-prod-api:*"),
				StoredBytes:     aws.Int64(104857600),
				RetentionInDays: aws.Int32(30),
				CreationTime:    aws.Int64(1707350400000),
			},
		},
		{
			ID:     relatedSFNLogsID,
			Name:   relatedSFNLogsID,
			Status: "",
			Fields: map[string]string{
				"log_group_name": relatedSFNLogsID,
				"stored_bytes":   "31457280",
				"retention_days": "90",
				"creation_time":  "1708000000000",
			},
			RawStruct: cwlogstypes.LogGroup{
				LogGroupName:    aws.String(relatedSFNLogsID),
				Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + relatedSFNLogsID + ":*"),
				StoredBytes:     aws.Int64(31457280),
				RetentionInDays: aws.Int32(90),
				CreationTime:    aws.Int64(1708000000000),
			},
		},
	}

	// Generate log group fixtures for every Lambda function that sets log_group in its Fields.
	// This satisfies the TestDemoCrossReference/lambda-log-group constraint.
	extraLambdaFns := []string{"image-thumbnail-gen", "cloudwatch-slack-notifier", relatedSecretsLambdaID}
	allLambdaLogGroups := make([]string, len(extraLambdaFns), len(extraLambdaFns)+len(lambdaNamePool))
	copy(allLambdaLogGroups, extraLambdaFns)
	allLambdaLogGroups = append(allLambdaLogGroups, lambdaNamePool...)
	for i, fn := range allLambdaLogGroups {
		name := "/aws/lambda/" + fn
		// Skip if already an explicit fixture above.
		duplicate := false
		for _, existing := range logGroups {
			if existing.ID == name {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		ct := int64(1704067200000 + int64(i)*3600000)
		logGroups = append(logGroups, resource.Resource{
			ID:     name,
			Name:   name,
			Status: "",
			Fields: map[string]string{
				"log_group_name": name,
				"stored_bytes":   "10485760",
				"retention_days": "30",
				"creation_time":  fmt.Sprintf("%d", ct),
			},
			RawStruct: cwlogstypes.LogGroup{
				LogGroupName:    aws.String(name),
				Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + name + ":*"),
				StoredBytes:     aws.Int64(10485760),
				RetentionInDays: aws.Int32(30),
				CreationTime:    aws.Int64(ct),
			},
		})
	}

	// Generate 17 more log groups to reach 22 total
	storedBytesPool := []int64{
		26214400, 104857600, 536870912, 1073741824, 52428800,
		209715200, 2147483648, 10485760, 419430400, 67108864,
		838860800, 26214400, 157286400, 52428800, 1073741824,
		209715200, 67108864,
	}
	retentionPool := []int32{7, 14, 30, 60, 90, 365, 0, 30, 14, 30, 90, 7, 30, 14, 365, 30, 90}
	for i := 0; i < 17; i++ {
		name := logGroupNamePool[i]
		storedBytes := storedBytesPool[i]
		retention := retentionPool[i]
		creationTime := int64(1700000000000 + int64(i)*86400000)
		retentionStr := fmt.Sprintf("%d", retention)
		if retention == 0 {
			retentionStr = ""
		}
		lg := cwlogstypes.LogGroup{
			LogGroupName: aws.String(name),
			Arn:          aws.String(fmt.Sprintf("arn:aws:logs:us-east-1:123456789012:log-group:%s:*", name)),
			StoredBytes:  aws.Int64(storedBytes),
			CreationTime: aws.Int64(creationTime),
		}
		if retention > 0 {
			lg.RetentionInDays = aws.Int32(retention)
		}
		logGroups = append(logGroups, resource.Resource{
			ID:     name,
			Name:   name,
			Status: "",
			Fields: map[string]string{
				"log_group_name": name,
				"stored_bytes":   fmt.Sprintf("%d", storedBytes),
				"retention_days": retentionStr,
				"creation_time":  fmt.Sprintf("%d", creationTime),
			},
			RawStruct: lg,
		})
	}

	return logGroups
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
				"request_id":         "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
				"timestamp":          "2024-03-22 00:00",
				"status":             "OK",
				"duration_ms":        "523 ms",
				"billed_duration_ms": "600 ms",
				"memory_size_mb":     "256",
				"memory_used_mb":     "128",
				"memory_used":        "128/256 MB",
				"init_duration_ms":   "",
				"cold_start":         "no",
				"xray_trace_id":      "",
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
				"request_id":         "bbbbbbbb-1111-2222-3333-444444444444",
				"timestamp":          "2024-03-22 00:05",
				"status":             "OK",
				"duration_ms":        "1250 ms",
				"billed_duration_ms": "1300 ms",
				"memory_size_mb":     "256",
				"memory_used_mb":     "200",
				"memory_used":        "200/256 MB",
				"init_duration_ms":   "350 ms",
				"cold_start":         "yes",
				"xray_trace_id":      "",
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

// ---------------------------------------------------------------------------
// CloudTrail Events
// ---------------------------------------------------------------------------

// cloudTrailEventFixtures returns demo CloudTrail Event fixtures with populated RawStruct.
// Exactly 6 events covering verbs W/D/R/S/I/N per the required matrix:
//   - Row 1: W + Root + resources[] (CreateAccessKey, Console, IAM User resource)
//   - Row 2: D + error + IAMUser + resources[] (DeleteBucket, CLI, AccessDenied, S3 resource)
//   - Row 3: R + Console + Management+no-resources (DescribeInstances, AssumedRole)
//   - Row 4: S + AwsServiceEvent + resources[] (TerminateInstanceInAutoScalingGroup, AWSService)
//   - Row 5: I (ApiCallRateInsight, Insight category)
//   - Row 6: N (VpcEndpointAccess, NetworkActivity category, cross-account)
//
// All ResourceName values and session issuer role names are cross-linked to existing fixture IDs.
func cloudTrailEventFixtures() []resource.Resource {
	t1 := time.Date(2026, 3, 28, 14, 30, 15, 0, time.UTC)
	t2 := time.Date(2026, 3, 28, 13, 45, 22, 0, time.UTC)
	t3 := time.Date(2026, 3, 28, 12, 10, 5, 0, time.UTC)
	t4 := time.Date(2026, 3, 28, 11, 55, 48, 0, time.UTC)
	t5 := time.Date(2026, 3, 28, 10, 20, 33, 0, time.UTC)
	t6 := time.Date(2026, 3, 28, 9, 5, 11, 0, time.UTC)
	// Wireframe cases A–I (ct-event-detail-v2.md §3)
	tA := time.Date(2026, 4, 7, 14, 2, 11, 0, time.UTC)
	tB := time.Date(2026, 4, 7, 14, 7, 42, 0, time.UTC)
	tC := time.Date(2026, 4, 7, 14, 11, 3, 0, time.UTC)
	tD := time.Date(2026, 4, 7, 2, 0, 7, 0, time.UTC)
	tE := time.Date(2026, 4, 7, 3, 42, 18, 0, time.UTC)
	tF := time.Date(2026, 4, 7, 14, 20, 21, 0, time.UTC)
	tG := time.Date(2026, 4, 7, 14, 31, 55, 0, time.UTC)
	tH := time.Date(2026, 4, 7, 9, 14, 0, 0, time.UTC)
	tI := time.Date(2026, 4, 7, 14, 44, 17, 0, time.UTC)
	tJ := time.Date(2026, 4, 7, 15, 10, 5, 0, time.UTC)
	tK := time.Date(2026, 4, 7, 15, 12, 33, 0, time.UTC)
	tL := time.Date(2026, 4, 7, 15, 14, 58, 0, time.UTC)

	return []resource.Resource{
		{
			// Row 1 (W, ct-attention, Console+Root): Root identity creates S3 bucket via Console.
			// userIdentity.type=Root satisfies TestCTEventsFixtureCoverage_AtLeastOneRootEvent.
			// sessionCredentialFromConsole=true → ORIGIN=Console.
			// resources=[S3 Bucket webapp-assets-prod] → TARGET from resources[]; resources[] present.
			// Root identity: Fields.user and Fields.role_name omitted; Username=nil.
			ID:     "evt-0a1b2c3d4e5f60001",
			Name:   "CreateBucket",
			Status: "ct-attention",
			Fields: map[string]string{
				"event_name": "CreateBucket",
				"time":       t1.Format("2006-01-02 15:04:05"),
				"event_time": t1.Format("2006-01-02 15:04:05"),
				"source":     "s3.amazonaws.com",
				"read_only":  "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:         aws.String("evt-0a1b2c3d4e5f60001"),
				EventName:       aws.String("CreateBucket"),
				EventTime:       aws.Time(t1),
				EventSource:     aws.String("s3.amazonaws.com"),
				Username:        nil,
				ReadOnly:        aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"Root","principalId":"123456789012","arn":"arn:aws:iam::123456789012:root","accountId":"123456789012","sessionContext":{"sessionCredentialFromConsole":"true","attributes":{"mfaAuthenticated":"true","creationDate":"2026-03-28T14:20:00Z"}}},"eventTime":"2026-03-28T14:30:15Z","eventSource":"s3.amazonaws.com","eventName":"CreateBucket","awsRegion":"us-east-1","sourceIPAddress":"198.51.100.10","userAgent":"signin.amazonaws.com","requestParameters":{"bucketName":"webapp-assets-prod"},"responseElements":null,"requestID":"req-s3-create-001","eventID":"evt-0a1b2c3d4e5f60001","readOnly":false,"eventType":"AwsApiCall","managementEvent":true,"recipientAccountId":"123456789012","eventCategory":"Management","resources":[{"ARN":"arn:aws:s3:::webapp-assets-prod","accountId":"123456789012","type":"AWS::S3::Bucket"}]}`),
				Resources: []cloudtrailtypes.Resource{
					{ResourceType: aws.String("AWS::S3::Bucket"), ResourceName: aws.String("webapp-assets-prod")},
				},
			},
		},
		{
			// Row 2 (D, ct-danger, CLI+error): IAMUser bob.smith attempts DeleteBucket — AccessDenied.
			// CLI userAgent; direct IAMUser identity; Resources use webapp-assets-prod (real S3 fixture).
			// IAMUser: Fields.role_name omitted; Fields.user=bob.smith retained.
			// errorCode=AccessDenied satisfies TestCTEventsFixtureCoverage_AtLeastOneErrorCodeEvent.
			ID:     "evt-0a1b2c3d4e5f60002",
			Name:   "DeleteBucket",
			Status: "ct-danger",
			Fields: map[string]string{
				"event_name": "DeleteBucket",
				"time":       t2.Format("2006-01-02 15:04:05"),
				"event_time": t2.Format("2006-01-02 15:04:05"),
				"user":       "bob.smith",
				"source":     "s3.amazonaws.com",
				"read_only":  "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:     aws.String("evt-0a1b2c3d4e5f60002"),
				EventName:   aws.String("DeleteBucket"),
				EventTime:   aws.Time(t2),
				EventSource: aws.String("s3.amazonaws.com"),
				Username:    aws.String("bob.smith"),
				ReadOnly:    aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"IAMUser","principalId":"AIDAEXAMPLE222222222","arn":"arn:aws:iam::123456789012:user/bob.smith","accountId":"123456789012","userName":"bob.smith"},"eventTime":"2026-03-28T13:45:22Z","eventSource":"s3.amazonaws.com","eventName":"DeleteBucket","awsRegion":"us-east-1","sourceIPAddress":"198.51.100.20","userAgent":"aws-cli/2.15.0 Python/3.11.0 Darwin/23.0.0 botocore/2.0.0","requestParameters":{"bucketName":"webapp-assets-prod"},"responseElements":null,"errorCode":"AccessDenied","errorMessage":"User: arn:aws:iam::123456789012:user/bob.smith is not authorized to perform: s3:DeleteBucket","requestID":"req-s3-del-001","eventID":"evt-0a1b2c3d4e5f60002","readOnly":false,"eventType":"AwsApiCall","managementEvent":true,"recipientAccountId":"123456789012","eventCategory":"Management","resources":[{"ARN":"arn:aws:s3:::webapp-assets-prod","accountId":"123456789012","type":"AWS::S3::Bucket"}]}`),
				Resources: []cloudtrailtypes.Resource{
					{ResourceType: aws.String("AWS::S3::Bucket"), ResourceName: aws.String("webapp-assets-prod")},
				},
			},
		},
		{
			// Row 3 (R, ct-info, Console+AssumedRole): AssumedRole reads EC2 via Console.
			// sessionCredentialFromConsole=true → ORIGIN=Console; no resources[] → Management+no-resources fallback.
			// AssumedRole sessionIssuer ARN leaf = acme-eks-node-role (real role fixture).
			// AssumedRole: Fields.user omitted; Fields.role_name retained.
			// Satisfies TestCTEventsFixtureCoverage_AllTargetFallbackCategoriesPresent: Management+no-resources.
			ID:     "evt-0a1b2c3d4e5f60003",
			Name:   "DescribeInstances",
			Status: "ct-info",
			Fields: map[string]string{
				"event_name": "DescribeInstances",
				"time":       t3.Format("2006-01-02 15:04:05"),
				"event_time": t3.Format("2006-01-02 15:04:05"),
				"role_name":  "acme-eks-node-role",
				"source":     "ec2.amazonaws.com",
				"read_only":  "true",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:     aws.String("evt-0a1b2c3d4e5f60003"),
				EventName:   aws.String("DescribeInstances"),
				EventTime:   aws.Time(t3),
				EventSource: aws.String("ec2.amazonaws.com"),
				Username:    aws.String("acme-eks-node-role"),
				ReadOnly:    aws.String("true"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","principalId":"AROAEXAMPLE111111111:i-0a1b2c3d4e5f60001","arn":"arn:aws:sts::123456789012:assumed-role/acme-eks-node-role/i-0a1b2c3d4e5f60001","accountId":"123456789012","sessionContext":{"sessionIssuer":{"type":"Role","principalId":"AROAEXAMPLE111111111","arn":"arn:aws:iam::123456789012:role/acme-eks-node-role","accountId":"123456789012","userName":"acme-eks-node-role"},"sessionCredentialFromConsole":"true","attributes":{"mfaAuthenticated":"false","creationDate":"2026-03-28T12:00:00Z"}}},"eventTime":"2026-03-28T12:10:05Z","eventSource":"ec2.amazonaws.com","eventName":"DescribeInstances","awsRegion":"us-east-1","sourceIPAddress":"198.51.100.30","userAgent":"signin.amazonaws.com","requestParameters":{},"responseElements":null,"requestID":"req-ec2-desc-001","eventID":"evt-0a1b2c3d4e5f60003","readOnly":true,"eventType":"AwsApiCall","managementEvent":true,"recipientAccountId":"123456789012","eventCategory":"Management"}`),
				Resources: []cloudtrailtypes.Resource{},
			},
		},
		{
			// Row 4 (S, ct-info, AwsServiceEvent+resources[]): autoscaling terminates EC2 instance.
			// AWSService identity (invokedBy=autoscaling.amazonaws.com) — no sessionIssuer.
			// eventType=AwsServiceEvent satisfies TestCTEventsFixtureCoverage_AllTargetFallbackCategoriesPresent.
			// resources=[EC2 i-0a1b2c3d4e5f60001] → resources[] present (satisfies hasResources).
			// AWSService: Fields.user and Fields.role_name omitted (no userIdentity).
			ID:     "evt-0a1b2c3d4e5f60004",
			Name:   "TerminateInstanceInAutoScalingGroup",
			Status: "ct-info",
			Fields: map[string]string{
				"event_name": "TerminateInstanceInAutoScalingGroup",
				"time":       t4.Format("2006-01-02 15:04:05"),
				"event_time": t4.Format("2006-01-02 15:04:05"),
				"source":     "autoscaling.amazonaws.com",
				"read_only":  "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:     aws.String("evt-0a1b2c3d4e5f60004"),
				EventName:   aws.String("TerminateInstanceInAutoScalingGroup"),
				EventTime:   aws.Time(t4),
				EventSource: aws.String("autoscaling.amazonaws.com"),
				Username:    nil,
				ReadOnly:    aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"AWSService","invokedBy":"autoscaling.amazonaws.com"},"eventTime":"2026-03-28T11:55:48Z","eventSource":"autoscaling.amazonaws.com","eventName":"TerminateInstanceInAutoScalingGroup","awsRegion":"us-east-1","sourceIPAddress":"autoscaling.amazonaws.com","requestParameters":{"instanceId":"i-0a1b2c3d4e5f60001"},"responseElements":{"instance":{"instanceId":"i-0a1b2c3d4e5f60001","currentState":{"name":"shutting-down"}}},"requestID":"req-asg-term-001","eventID":"evt-0a1b2c3d4e5f60004","readOnly":false,"eventType":"AwsServiceEvent","managementEvent":true,"recipientAccountId":"123456789012","eventCategory":"Management","resources":[{"ARN":"arn:aws:ec2:us-east-1:123456789012:instance/i-0a1b2c3d4e5f60001","accountId":"123456789012","type":"AWS::EC2::Instance"}]}`),
				Resources: []cloudtrailtypes.Resource{
					{ResourceType: aws.String("AWS::EC2::Instance"), ResourceName: aws.String("i-0a1b2c3d4e5f60001")},
				},
			},
		},
		{
			// Row 5 (I, ct-info, Insight): ApiCallRateInsight — eventCategory=Insight, I verb.
			// IAMUser identity in JSON; Fields.role_name omitted; Fields.user=bob.smith retained.
			// insightDetails with baseline/insight statistics for detail view.
			ID:     "evt-0a1b2c3d4e5f60005",
			Name:   "ApiCallRateInsight",
			Status: "ct-info",
			Fields: map[string]string{
				"event_name": "ApiCallRateInsight",
				"time":       t5.Format("2006-01-02 15:04:05"),
				"event_time": t5.Format("2006-01-02 15:04:05"),
				"user":       "bob.smith",
				"source":     "cloudtrail.amazonaws.com",
				"read_only":  "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:     aws.String("evt-0a1b2c3d4e5f60005"),
				EventName:   aws.String("ApiCallRateInsight"),
				EventTime:   aws.Time(t5),
				EventSource: aws.String("cloudtrail.amazonaws.com"),
				Username:    aws.String("bob.smith"),
				ReadOnly:    aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.11","userIdentity":{"type":"IAMUser","principalId":"AIDAEXAMPLE222222222","arn":"arn:aws:iam::123456789012:user/bob.smith","accountId":"123456789012","userName":"bob.smith"},"eventTime":"2026-03-28T10:20:33Z","eventSource":"cloudtrail.amazonaws.com","eventName":"ApiCallRateInsight","awsRegion":"us-east-1","sourceIPAddress":"","userAgent":"","requestParameters":null,"responseElements":null,"requestID":"req-insight-001","eventID":"evt-0a1b2c3d4e5f60005","readOnly":false,"eventType":"AwsApiCall","managementEvent":false,"recipientAccountId":"123456789012","eventCategory":"Insight","insightDetails":{"state":"Start","insightType":"ApiCallRateInsight","insightContext":{"statistics":{"baseline":{"average":5.0},"insight":{"average":120.0}}}}}`),
				Resources:   []cloudtrailtypes.Resource{},
			},
		},
		{
			// Row 6 (N, ct-attention, NetworkActivity+cross-account): AssumedRole ci-runner accesses VPC endpoint.
			// eventCategory=NetworkActivity → ClassifyCTVerb returns "N", satisfying AllVerbsPresent.
			// Satisfies AllTargetFallbackCategoriesPresent: NetworkActivity bucket.
			// Cross-account: accountId=999988887777 != recipientAccountId=123456789012.
			// sessionIssuer ARN leaf = ci-runner (real role fixture).
			// AssumedRole: Fields.user omitted; Fields.role_name=ci-runner retained.
			ID:     "evt-0a1b2c3d4e5f60006",
			Name:   "VpcEndpointAccess",
			Status: "ct-attention",
			Fields: map[string]string{
				"event_name": "VpcEndpointAccess",
				"time":       t6.Format("2006-01-02 15:04:05"),
				"event_time": t6.Format("2006-01-02 15:04:05"),
				"role_name":  "ci-runner",
				"source":     "s3.amazonaws.com",
				"read_only":  "true",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:     aws.String("evt-0a1b2c3d4e5f60006"),
				EventName:   aws.String("VpcEndpointAccess"),
				EventTime:   aws.Time(t6),
				EventSource: aws.String("s3.amazonaws.com"),
				Username:    aws.String("ci-runner"),
				ReadOnly:    aws.String("true"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","principalId":"AROAEXAMPLE666666666:ci-session","arn":"arn:aws:sts::999988887777:assumed-role/ci-runner/ci-session","accountId":"999988887777","sessionContext":{"sessionIssuer":{"type":"Role","principalId":"AROAEXAMPLE666666666","arn":"arn:aws:iam::123456789012:role/ci-runner","accountId":"123456789012","userName":"ci-runner"},"attributes":{"mfaAuthenticated":"false","creationDate":"2026-03-28T09:00:00Z"}}},"eventTime":"2026-03-28T09:05:11Z","eventSource":"s3.amazonaws.com","eventName":"VpcEndpointAccess","awsRegion":"us-east-1","sourceIPAddress":"203.0.113.50","userAgent":"aws-sdk-java/2.20 Linux/5.15 Java/17.0","requestParameters":{},"responseElements":null,"requestID":"req-vpc-ep-001","eventID":"evt-0a1b2c3d4e5f60006","readOnly":true,"eventType":"AwsApiCall","managementEvent":false,"recipientAccountId":"123456789012","eventCategory":"NetworkActivity","vpcEndpointId":"vpce-0abc123"}`),
				Resources:   []cloudtrailtypes.Resource{},
			},
		},
		// ---------------------------------------------------------------------------
		// Wireframe cases A–I from ct-event-detail-v2.md §3
		// Fields["user"] and Fields["role_name"] use existing fixture IDs for navigation
		// integrity. The CloudTrailEvent JSON blob carries the accurate wireframe data.
		// Resources use existing fixture IDs; raw event IDs in JSON are for display only.
		// ---------------------------------------------------------------------------
		{
			// Case A — Karpenter ec2:DescribeInstances (R, ct-info)
			// AssumedRole / KarpenterNodeRole, read-only, no error.
			// AssumedRole: Fields.user omitted; Fields.role_name=KarpenterNodeRole retained.
			ID:     "e-a1b2c3d4",
			Name:   "DescribeInstances",
			Status: "ct-info",
			Fields: map[string]string{
				"event_name":     "DescribeInstances",
				"time":           tA.Format("Jan 02 15:04:05"),
				"event_time":     tA.Format(time.RFC3339),
				"event_time_raw": tA.Format(time.RFC3339),
				"role_name":      "KarpenterNodeRole",
				"source":         "ec2.amazonaws.com",
				"read_only":      "true",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:     aws.String("e-a1b2c3d4"),
				EventName:   aws.String("DescribeInstances"),
				EventTime:   aws.Time(tA),
				EventSource: aws.String("ec2.amazonaws.com"),
				Username:    aws.String("KarpenterNodeRole"),
				ReadOnly:    aws.String("true"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T14:02:11Z","eventSource":"ec2.amazonaws.com","eventName":"DescribeInstances","eventCategory":"Management","eventType":"AwsApiCall","awsRegion":"us-east-1","sourceIPAddress":"10.0.14.221","userAgent":"aws-sdk-go-v2/1.30.3","recipientAccountId":"111111111111","eventID":"e-a1b2c3d4","readOnly":true,"userIdentity":{"type":"AssumedRole","arn":"arn:aws:sts::111111111111:assumed-role/KarpenterNodeRole/karpenter-1759","principalId":"AROAEXAMPLE:karpenter-1759","accountId":"111111111111","accessKeyId":"ASIAY44QH8DCKARPEXMP","sessionContext":{"sessionIssuer":{"type":"Role","arn":"arn:aws:iam::111111111111:role/KarpenterNodeRole","principalId":"AROAEXAMPLE","accountId":"111111111111","userName":"KarpenterNodeRole"},"attributes":{"mfaAuthenticated":"false","creationDate":"2026-04-07T13:44:02Z"}}},"requestParameters":{"filterSet":{"items":[{"name":"instance-state-name","valueSet":{"items":[{"value":"running"}]}}]},"maxResults":1000},"responseElements":null}`),
				Resources:   []cloudtrailtypes.Resource{},
			},
		},
		{
			// Case B — SSO Console ec2:TerminateInstances (D verb + MFA, ct-danger)
			// AssumedRole via SSO; two instances terminated; MFA=true.
			// AssumedRole: Fields.user omitted; Fields.role_name=AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d retained.
			// Resources use real EC2 fixture IDs i-0a1b2c3d4e5f60001 and i-0a1b2c3d4e5f60002.
			ID:     "e-b2c3d4e5",
			Name:   "TerminateInstances",
			Status: "ct-danger",
			Fields: map[string]string{
				"event_name":     "TerminateInstances",
				"time":           tB.Format("Jan 02 15:04:05"),
				"event_time":     tB.Format(time.RFC3339),
				"event_time_raw": tB.Format(time.RFC3339),
				"role_name":      "AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d",
				"source":         "ec2.amazonaws.com",
				"read_only":      "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:     aws.String("e-b2c3d4e5"),
				EventName:   aws.String("TerminateInstances"),
				EventTime:   aws.Time(tB),
				EventSource: aws.String("ec2.amazonaws.com"),
				Username:    aws.String("AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d"),
				ReadOnly:    aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T14:07:42Z","eventSource":"ec2.amazonaws.com","eventName":"TerminateInstances","eventCategory":"Management","eventType":"AwsApiCall","awsRegion":"eu-west-1","sourceIPAddress":"AWS Internal","userAgent":"console.amazonaws.com","recipientAccountId":"222222222222","eventID":"e-b2c3d4e5","readOnly":false,"userIdentity":{"type":"AssumedRole","arn":"arn:aws:sts::222222222222:assumed-role/AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d/alice@corp","principalId":"AROAEXAMPLE:alice@corp","accountId":"222222222222","accessKeyId":"ASIAZK7L9PQRSSOXEXMP","sessionContext":{"sessionIssuer":{"type":"Role","arn":"arn:aws:iam::222222222222:role/AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d","principalId":"AROAEXAMPLE","accountId":"222222222222","userName":"AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d"},"attributes":{"mfaAuthenticated":"true","creationDate":"2026-04-07T14:00:00Z"}}},"requestParameters":{"instancesSet":{"items":[{"instanceId":"i-0a1b2c3d4e5f60001"},{"instanceId":"i-0a1b2c3d4e5f60002"}]}},"responseElements":{"instancesSet":{"items":[{"instanceId":"i-0a1b2c3d4e5f60001","currentState":{"code":32,"name":"shutting-down"},"previousState":{"code":16,"name":"running"}},{"instanceId":"i-0a1b2c3d4e5f60002","currentState":{"code":32,"name":"shutting-down"},"previousState":{"code":16,"name":"running"}}]}}}`),
				Resources: []cloudtrailtypes.Resource{
					{ResourceType: aws.String("AWS::EC2::Instance"), ResourceName: aws.String("i-0a1b2c3d4e5f60001")},
					{ResourceType: aws.String("AWS::EC2::Instance"), ResourceName: aws.String("i-0a1b2c3d4e5f60002")},
				},
			},
		},
		{
			// Case C — IAMUser bob s3:PutObject AccessDenied (errorCode → ct-danger, ERROR hoisted)
			// IAMUser: Fields.role_name omitted; Fields.user=bob.smith retained.
			// Resources use webapp-assets-prod (real S3 fixture); requestParameters.bucketName matches.
			ID:     "e-c3d4e5f6",
			Name:   "PutObject",
			Status: "ct-danger",
			Fields: map[string]string{
				"event_name":     "PutObject",
				"time":           tC.Format("Jan 02 15:04:05"),
				"event_time":     tC.Format(time.RFC3339),
				"event_time_raw": tC.Format(time.RFC3339),
				"user":           "bob.smith",
				"source":         "s3.amazonaws.com",
				"read_only":      "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:     aws.String("e-c3d4e5f6"),
				EventName:   aws.String("PutObject"),
				EventTime:   aws.Time(tC),
				EventSource: aws.String("s3.amazonaws.com"),
				Username:    aws.String("bob"),
				ReadOnly:    aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T14:11:03Z","eventSource":"s3.amazonaws.com","eventName":"PutObject","eventCategory":"Management","eventType":"AwsApiCall","awsRegion":"us-east-1","sourceIPAddress":"198.51.100.42","userAgent":"aws-cli/2.17.9 Python/3.12.4 Darwin/24.1.0","recipientAccountId":"333333333333","eventID":"e-c3d4e5f6","readOnly":false,"errorCode":"AccessDenied","errorMessage":"User: arn:aws:iam::333333333333:user/bob is not authorized to perform: s3:PutObject on resource: arn:aws:s3:::webapp-assets-prod/2026/04/07/app.log because no identity-based policy allows the s3:PutObject action","userIdentity":{"type":"IAMUser","principalId":"AIDAIOSFODNN7BOB1XMP","arn":"arn:aws:iam::333333333333:user/bob","accountId":"333333333333","accessKeyId":"AKIAIOSFODNN7BOB1XMP","userName":"bob"},"requestParameters":{"bucketName":"webapp-assets-prod","key":"2026/04/07/app.log"},"responseElements":null}`),
				Resources: []cloudtrailtypes.Resource{
					{ResourceType: aws.String("AWS::S3::Bucket"), ResourceName: aws.String("webapp-assets-prod")},
				},
			},
		},
		{
			// Case D — KMS kms:RotateKey AwsServiceEvent (S verb → ct-attention)
			// No userIdentity ARN — Service: row in ACTOR. Category row appears in ACTION.
			// AWSService: Fields.user and Fields.role_name omitted (no userIdentity).
			// No Resources — KMS key ARN not a resolvable fixture ID.
			ID:     "e-d4e5f6a7",
			Name:   "RotateKey",
			Status: "ct-attention",
			Fields: map[string]string{
				"event_name":     "RotateKey",
				"time":           tD.Format("Jan 02 15:04:05"),
				"event_time":     tD.Format(time.RFC3339),
				"event_time_raw": tD.Format(time.RFC3339),
				"source":         "kms.amazonaws.com",
				"read_only":      "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:     aws.String("e-d4e5f6a7"),
				EventName:   aws.String("RotateKey"),
				EventTime:   aws.Time(tD),
				EventSource: aws.String("kms.amazonaws.com"),
				Username:    nil,
				ReadOnly:    aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T02:00:07Z","eventSource":"kms.amazonaws.com","eventName":"RotateKey","eventCategory":"Management","eventType":"AwsServiceEvent","awsRegion":"us-east-1","sourceIPAddress":"AWS Internal","recipientAccountId":"444444444444","eventID":"e-d4e5f6a7","readOnly":false,"userIdentity":{"type":"AWSService","invokedBy":"kms.amazonaws.com"},"requestParameters":{"keyId":"arn:aws:kms:us-east-1:444444444444:key/2f7e9a5b-8c1d-4e3f-9a0b-1c2d3e4f5a6b","rotationType":"AUTOMATIC","backingKey":true},"responseElements":null}`),
				Resources:   []cloudtrailtypes.Resource{},
			},
		},
		{
			// Case E — Root s3:PutBucketPolicy (Root + W → ct-attention)
			// Root identity: Fields.user and Fields.role_name omitted; Username=nil.
			// Resources updated to prod-artifacts to match requestParameters.bucketName.
			ID:     "e-e5f6a7b8",
			Name:   "PutBucketPolicy",
			Status: "ct-attention",
			Fields: map[string]string{
				"event_name":     "PutBucketPolicy",
				"time":           tE.Format("Jan 02 15:04:05"),
				"event_time":     tE.Format(time.RFC3339),
				"event_time_raw": tE.Format(time.RFC3339),
				"source":         "s3.amazonaws.com",
				"read_only":      "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:     aws.String("e-e5f6a7b8"),
				EventName:   aws.String("PutBucketPolicy"),
				EventTime:   aws.Time(tE),
				EventSource: aws.String("s3.amazonaws.com"),
				Username:    nil,
				ReadOnly:    aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T03:42:18Z","eventSource":"s3.amazonaws.com","eventName":"PutBucketPolicy","eventCategory":"Management","eventType":"AwsApiCall","awsRegion":"us-east-1","sourceIPAddress":"203.0.113.17","userAgent":"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15","recipientAccountId":"555555555555","eventID":"e-e5f6a7b8","readOnly":false,"userIdentity":{"type":"Root","principalId":"555555555555","arn":"arn:aws:iam::555555555555:root","accountId":"555555555555"},"requestParameters":{"bucketName":"prod-artifacts","policy":"{\"Version\":\"2012-10-17\",\"Statement\":[{\"Effect\":\"Allow\",\"Principal\":\"*\",\"Action\":\"s3:GetObject\",\"Resource\":\"arn:aws:s3:::prod-artifacts/*\"}]}"},"responseElements":null,"resources":[{"ARN":"arn:aws:s3:::prod-artifacts","accountId":"555555555555","type":"AWS::S3::Bucket"}]}`),
				Resources: []cloudtrailtypes.Resource{
					{ResourceType: aws.String("AWS::S3::Bucket"), ResourceName: aws.String("prod-artifacts")},
				},
			},
		},
		{
			// Case F — IRSA s3:GetObject via VPC endpoint (WebIdentityUser / IRSA, R → ct-info)
			// Federation row distinguishes IRSA; VPC endpoint in CONTEXT.
			// AssumedRole: Fields.user omitted; Fields.role_name=eks-checkout-svc-sa retained.
			// Resources updated to checkout-config to match requestParameters.bucketName.
			ID:     "e-f6a7b8c9",
			Name:   "GetObject",
			Status: "ct-info",
			Fields: map[string]string{
				"event_name":     "GetObject",
				"time":           tF.Format("Jan 02 15:04:05"),
				"event_time":     tF.Format(time.RFC3339),
				"event_time_raw": tF.Format(time.RFC3339),
				"role_name":      "eks-checkout-svc-sa",
				"source":         "s3.amazonaws.com",
				"read_only":      "true",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:     aws.String("e-f6a7b8c9"),
				EventName:   aws.String("GetObject"),
				EventTime:   aws.Time(tF),
				EventSource: aws.String("s3.amazonaws.com"),
				Username:    aws.String("eks-checkout-svc-sa"),
				ReadOnly:    aws.String("true"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T14:20:21Z","eventSource":"s3.amazonaws.com","eventName":"GetObject","eventCategory":"Management","eventType":"AwsApiCall","awsRegion":"eu-west-1","sourceIPAddress":"10.42.3.18","userAgent":"aws-sdk-go-v2/1.30.3","recipientAccountId":"666666666666","eventID":"e-f6a7b8c9","readOnly":true,"vpcEndpointId":"vpce-0abc123def456","userIdentity":{"type":"AssumedRole","arn":"arn:aws:sts::666666666666:assumed-role/eks-checkout-svc-sa/1717156821993453824","principalId":"AROAEXAMPLE:1717156821993453824","accountId":"666666666666","sessionContext":{"sessionIssuer":{"type":"Role","arn":"arn:aws:iam::666666666666:role/eks-checkout-svc-sa","principalId":"AROAEXAMPLE","accountId":"666666666666","userName":"eks-checkout-svc-sa"},"attributes":{"mfaAuthenticated":"false","creationDate":"2026-04-07T14:15:00Z"}},"webIdFederationData":{"federatedProvider":"oidc.eks.eu-west-1.amazonaws.com/id/EXAMPLE0D8C"}},"requestParameters":{"bucketName":"checkout-config","key":"prod/config.json"},"responseElements":null,"resources":[{"ARN":"arn:aws:s3:::checkout-config","accountId":"666666666666","type":"AWS::S3::Bucket"}]}`),
				Resources: []cloudtrailtypes.Resource{
					{ResourceType: aws.String("AWS::S3::Bucket"), ResourceName: aws.String("checkout-config")},
				},
			},
		},
		{
			// Case G — Cross-account s3:PutObject (caller 888888888888, recipient 777777777777 → ct-attention)
			// AssumedRole: Fields.user omitted; Fields.role_name=CiBuildRole retained.
			// Resources updated to shared-artifacts to match requestParameters.bucketName.
			ID:     "e-a7b8c9d0",
			Name:   "PutObject",
			Status: "ct-attention",
			Fields: map[string]string{
				"event_name":     "PutObject",
				"time":           tG.Format("Jan 02 15:04:05"),
				"event_time":     tG.Format(time.RFC3339),
				"event_time_raw": tG.Format(time.RFC3339),
				"role_name":      "CiBuildRole",
				"source":         "s3.amazonaws.com",
				"read_only":      "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:     aws.String("e-a7b8c9d0"),
				EventName:   aws.String("PutObject"),
				EventTime:   aws.Time(tG),
				EventSource: aws.String("s3.amazonaws.com"),
				Username:    aws.String("CiBuildRole"),
				ReadOnly:    aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T14:31:55Z","eventSource":"s3.amazonaws.com","eventName":"PutObject","eventCategory":"Management","eventType":"AwsApiCall","awsRegion":"us-east-2","sourceIPAddress":"52.14.88.201","userAgent":"aws-cli/2.17.9","recipientAccountId":"777777777777","eventID":"e-a7b8c9d0","readOnly":false,"userIdentity":{"type":"AssumedRole","arn":"arn:aws:sts::888888888888:assumed-role/CiBuildRole/build-4821","principalId":"AROAEXAMPLE:build-4821","accountId":"888888888888","accessKeyId":"ASIAQF3M2N8KCIB1XMPL","sessionContext":{"sessionIssuer":{"type":"Role","arn":"arn:aws:iam::888888888888:role/CiBuildRole","principalId":"AROAEXAMPLE","accountId":"888888888888","userName":"CiBuildRole"},"attributes":{"mfaAuthenticated":"false","creationDate":"2026-04-07T14:25:00Z"}}},"requestParameters":{"bucketName":"shared-artifacts","key":"build-4821.tar.gz"},"responseElements":null,"resources":[{"ARN":"arn:aws:s3:::shared-artifacts","accountId":"888888888888","type":"AWS::S3::Bucket"}]}`),
				Resources: []cloudtrailtypes.Resource{
					{ResourceType: aws.String("AWS::S3::Bucket"), ResourceName: aws.String("shared-artifacts")},
				},
			},
		},
		{
			// Case H — Insight ec2:RunInstances ApiCallRateInsight (no ACTOR, ct-info)
			// eventCategory=Insight; no userIdentity. ACTOR section omitted in detail view.
			// Insight (no userIdentity): Fields.user and Fields.role_name omitted.
			ID:     "e-b8c9d0e1",
			Name:   "RunInstances",
			Status: "ct-info",
			Fields: map[string]string{
				"event_name":     "RunInstances",
				"time":           tH.Format("Jan 02 15:04:05"),
				"event_time":     tH.Format(time.RFC3339),
				"event_time_raw": tH.Format(time.RFC3339),
				"source":         "ec2.amazonaws.com",
				"read_only":      "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:     aws.String("e-b8c9d0e1"),
				EventName:   aws.String("RunInstances"),
				EventTime:   aws.Time(tH),
				EventSource: aws.String("ec2.amazonaws.com"),
				Username:    nil,
				ReadOnly:    aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.11","eventTime":"2026-04-07T09:14:00Z","eventSource":"ec2.amazonaws.com","eventName":"RunInstances","eventCategory":"Insight","eventType":"AwsApiCall","awsRegion":"us-east-1","sourceIPAddress":"","recipientAccountId":"999999999999","eventID":"e-b8c9d0e1","readOnly":false,"requestParameters":null,"responseElements":null,"insightDetails":{"state":"Start","insightType":"ApiCallRateInsight","insightContext":{"statistics":{"baseline":{"average":0.24},"insight":{"average":18.70}},"attributions":[{"attribute":"userIdentityArn","insight":["arn:aws:sts::999999999999:assumed-role/DeployRole/ci-41"],"baseline":["arn:aws:sts::999999999999:assumed-role/DeployRole/ci-*"]}]}}}`),
				Resources:   []cloudtrailtypes.Resource{},
			},
		},
		{
			// Case I — NetworkActivity s3:PutObject VPCE deny (errorCode → ct-danger)
			// eventCategory=NetworkActivity, eventType=AwsVpceEvent, VpceAccessDenied.
			// AssumedRole: Fields.user omitted; Fields.role_name=DataPipelineRole retained.
			// Resources updated to prod-lake to match requestParameters.bucketName.
			ID:     "e-c9d0e1f2",
			Name:   "PutObject",
			Status: "ct-danger",
			Fields: map[string]string{
				"event_name":     "PutObject",
				"time":           tI.Format("Jan 02 15:04:05"),
				"event_time":     tI.Format(time.RFC3339),
				"event_time_raw": tI.Format(time.RFC3339),
				"role_name":      "DataPipelineRole",
				"source":         "s3.amazonaws.com",
				"read_only":      "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:     aws.String("e-c9d0e1f2"),
				EventName:   aws.String("PutObject"),
				EventTime:   aws.Time(tI),
				EventSource: aws.String("s3.amazonaws.com"),
				Username:    aws.String("DataPipelineRole"),
				ReadOnly:    aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T14:44:17Z","eventSource":"s3.amazonaws.com","eventName":"PutObject","eventCategory":"NetworkActivity","eventType":"AwsVpceEvent","awsRegion":"eu-central-1","sourceIPAddress":"10.12.4.77","userAgent":"aws-sdk-java/2.25.11","recipientAccountId":"111111111111","eventID":"e-c9d0e1f2","readOnly":false,"errorCode":"VpceAccessDenied","errorMessage":"The VPC endpoint policy denies the s3:PutObject action on arn:aws:s3:::prod-lake/landing/2026/04/07/batch-0719.parquet","vpcEndpointId":"vpce-0ff11223344556677","userIdentity":{"type":"AssumedRole","arn":"arn:aws:sts::111111111111:assumed-role/DataPipelineRole/dp-0719","principalId":"AROAEXAMPLE:dp-0719","accountId":"111111111111","sessionContext":{"sessionIssuer":{"type":"Role","arn":"arn:aws:iam::111111111111:role/DataPipelineRole","principalId":"AROAEXAMPLE","accountId":"111111111111","userName":"DataPipelineRole"},"attributes":{"mfaAuthenticated":"false","creationDate":"2026-04-07T14:40:00Z"}}},"requestParameters":{"bucketName":"prod-lake","key":"landing/2026/04/07/batch-0719.parquet"},"responseElements":null,"resources":[{"ARN":"arn:aws:s3:::prod-lake","accountId":"111111111111","type":"AWS::S3::Bucket"}]}`),
				Resources: []cloudtrailtypes.Resource{
					{ResourceType: aws.String("AWS::S3::Bucket"), ResourceName: aws.String("prod-lake")},
				},
			},
		},
		{
			// Case J — IAM CreateUser (successful). Actor: alice.johnson (IAM user).
			// Target: new user charlie at path /. Exercises SummarizeIAM requestParameters branch.
			// IAMUser: Fields.role_name omitted; Fields.user=alice.johnson retained.
			ID:     "e-d0e1f2a3",
			Name:   "CreateUser",
			Status: "ct-ok",
			Fields: map[string]string{
				"event_name":     "CreateUser",
				"time":           tJ.Format("Jan 02 15:04:05"),
				"event_time":     tJ.Format(time.RFC3339),
				"event_time_raw": tJ.Format(time.RFC3339),
				"user":           "alice.johnson",
				"source":         "iam.amazonaws.com",
				"read_only":      "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:     aws.String("e-d0e1f2a3"),
				EventName:   aws.String("CreateUser"),
				EventTime:   aws.Time(tJ),
				EventSource: aws.String("iam.amazonaws.com"),
				Username:    aws.String("alice.johnson"),
				ReadOnly:    aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T15:10:05Z","eventSource":"iam.amazonaws.com","eventName":"CreateUser","eventType":"AwsApiCall","awsRegion":"us-east-1","sourceIPAddress":"203.0.113.10","userAgent":"aws-cli/2.15.0 Python/3.11.8 Darwin/24.3.0 botocore/2.15.0","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/alice.johnson","principalId":"AIDAIOSFODNN7EXAMPLE","accountId":"123456789012","userName":"alice.johnson"},"requestParameters":{"userName":"charlie","path":"/"},"responseElements":{"user":{"userId":"AIDAIOSFODNN8EXAMPLE","arn":"arn:aws:iam::123456789012:user/charlie","path":"/","userName":"charlie","createDate":"Apr 7, 2026, 3:10:05 PM"}},"recipientAccountId":"123456789012","eventID":"e-d0e1f2a3"}`),
				Resources: []cloudtrailtypes.Resource{
					{ResourceType: aws.String("AWS::IAM::User"), ResourceName: aws.String("charlie")},
				},
			},
		},
		{
			// Case K — IAM AttachUserPolicy (successful). Actor: alice.johnson (IAM user).
			// Target: user bob, policy AdministratorAccess. Exercises SummarizeIAM policyArn navigable branch.
			// IAMUser: Fields.role_name omitted; Fields.user=alice.johnson retained.
			ID:     "e-e1f2a3b4",
			Name:   "AttachUserPolicy",
			Status: "ct-ok",
			Fields: map[string]string{
				"event_name":     "AttachUserPolicy",
				"time":           tK.Format("Jan 02 15:04:05"),
				"event_time":     tK.Format(time.RFC3339),
				"event_time_raw": tK.Format(time.RFC3339),
				"user":           "alice.johnson",
				"source":         "iam.amazonaws.com",
				"read_only":      "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:     aws.String("e-e1f2a3b4"),
				EventName:   aws.String("AttachUserPolicy"),
				EventTime:   aws.Time(tK),
				EventSource: aws.String("iam.amazonaws.com"),
				Username:    aws.String("alice.johnson"),
				ReadOnly:    aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T15:12:33Z","eventSource":"iam.amazonaws.com","eventName":"AttachUserPolicy","eventType":"AwsApiCall","awsRegion":"us-east-1","sourceIPAddress":"203.0.113.10","userAgent":"aws-cli/2.15.0 Python/3.11.8 Darwin/24.3.0 botocore/2.15.0","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/alice.johnson","principalId":"AIDAIOSFODNN7EXAMPLE","accountId":"123456789012","userName":"alice.johnson"},"requestParameters":{"userName":"bob","policyArn":"arn:aws:iam::aws:policy/AdministratorAccess"},"responseElements":null,"recipientAccountId":"123456789012","eventID":"e-e1f2a3b4"}`),
				Resources: []cloudtrailtypes.Resource{
					{ResourceType: aws.String("AWS::IAM::User"), ResourceName: aws.String("bob")},
					{ResourceType: aws.String("AWS::IAM::ManagedPolicy"), ResourceName: aws.String("arn:aws:iam::aws:policy/AdministratorAccess")},
				},
			},
		},
		{
			// Case L — IAM CreateAccessKey (successful). Actor: alice.johnson (IAM user).
			// Target: user bob. responseElements contain accessKeyId. Exercises SummarizeIAM responseElements branch.
			// IAMUser: Fields.role_name omitted; Fields.user=alice.johnson retained.
			ID:     "e-f2a3b4c5",
			Name:   "CreateAccessKey",
			Status: "ct-ok",
			Fields: map[string]string{
				"event_name":     "CreateAccessKey",
				"time":           tL.Format("Jan 02 15:04:05"),
				"event_time":     tL.Format(time.RFC3339),
				"event_time_raw": tL.Format(time.RFC3339),
				"user":           "alice.johnson",
				"source":         "iam.amazonaws.com",
				"read_only":      "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:     aws.String("e-f2a3b4c5"),
				EventName:   aws.String("CreateAccessKey"),
				EventTime:   aws.Time(tL),
				EventSource: aws.String("iam.amazonaws.com"),
				Username:    aws.String("alice.johnson"),
				ReadOnly:    aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","eventTime":"2026-04-07T15:14:58Z","eventSource":"iam.amazonaws.com","eventName":"CreateAccessKey","eventType":"AwsApiCall","awsRegion":"us-east-1","sourceIPAddress":"203.0.113.10","userAgent":"aws-cli/2.15.0 Python/3.11.8 Darwin/24.3.0 botocore/2.15.0","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/alice.johnson","principalId":"AIDAIOSFODNN7EXAMPLE","accountId":"123456789012","userName":"alice.johnson"},"requestParameters":{"userName":"bob"},"responseElements":{"accessKey":{"accessKeyId":"AKIAIOSFODNN7EXAMPLE","status":"Active","userName":"bob","createDate":"Apr 7, 2026, 3:14:58 PM"}},"recipientAccountId":"123456789012","eventID":"e-f2a3b4c5"}`),
				Resources: []cloudtrailtypes.Resource{
					{ResourceType: aws.String("AWS::IAM::User"), ResourceName: aws.String("bob")},
				},
			},
		},
	}
}
