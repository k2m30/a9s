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
// Includes a mix of write and read-only events across common AWS services.
func cloudTrailEventFixtures() []resource.Resource {
	t1 := time.Date(2026, 3, 28, 14, 30, 15, 0, time.UTC)
	t2 := time.Date(2026, 3, 28, 13, 45, 22, 0, time.UTC)
	t3 := time.Date(2026, 3, 28, 12, 10, 5, 0, time.UTC)
	t4 := time.Date(2026, 3, 28, 11, 55, 48, 0, time.UTC)
	t5 := time.Date(2026, 3, 28, 10, 20, 33, 0, time.UTC)
	t6 := time.Date(2026, 3, 28, 9, 5, 11, 0, time.UTC)

	return []resource.Resource{
		{
			ID:     "evt-0a1b2c3d4e5f60001",
			Name:   "CreateBucket",
			Status: "false",
			Fields: map[string]string{
				"event_name":    "CreateBucket",
				"time":          t1.Format("2006-01-02 15:04:05"),
				"user":          "deploy-bot",
				"source":        "s3.amazonaws.com",
				"resource_type": "AWS::S3::Bucket",
				"resource_name": "webapp-assets-prod",
				"read_only":     "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:         aws.String("evt-0a1b2c3d4e5f60001"),
				EventName:       aws.String("CreateBucket"),
				EventTime:       aws.Time(t1),
				EventSource:     aws.String("s3.amazonaws.com"),
				Username:        aws.String("deploy-bot"),
				ReadOnly:        aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","principalId":"AROAEXAMPLE001","arn":"arn:aws:sts::123456789012:assumed-role/deploy-bot/session","accountId":"123456789012","sessionContext":{"sessionIssuer":{"type":"Role","arn":"arn:aws:iam::123456789012:role/deploy-bot","userName":"deploy-bot"}}},"eventTime":"2026-03-28T14:30:15Z","eventSource":"s3.amazonaws.com","eventName":"CreateBucket","awsRegion":"us-east-1","sourceIPAddress":"198.51.100.10","userAgent":"aws-cli/2.15.0","requestParameters":{"bucketName":"acme-prod-assets-2026","CreateBucketConfiguration":{"LocationConstraint":"us-east-1"}},"responseElements":null,"requestID":"req-s3-create-001","eventID":"evt-0a1b2c3d4e5f60001","readOnly":false,"eventType":"AwsApiCall","managementEvent":true}`),
				Resources: []cloudtrailtypes.Resource{
					{
						ResourceType: aws.String("AWS::S3::Bucket"),
						ResourceName: aws.String("webapp-assets-prod"),
					},
				},
			},
		},
		{
			ID:     "evt-0a1b2c3d4e5f60002",
			Name:   "RunInstances",
			Status: "false",
			Fields: map[string]string{
				"event_name":    "RunInstances",
				"time":          t2.Format("2006-01-02 15:04:05"),
				"user":          "admin",
				"source":        "ec2.amazonaws.com",
				"resource_type": "AWS::EC2::Instance",
				"resource_name": "i-0a1b2c3d4e5f60007",
				"read_only":     "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:         aws.String("evt-0a1b2c3d4e5f60002"),
				EventName:       aws.String("RunInstances"),
				EventTime:       aws.Time(t2),
				EventSource:     aws.String("ec2.amazonaws.com"),
				Username:        aws.String("admin"),
				ReadOnly:        aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"IAMUser","principalId":"AIDAEXAMPLE002","arn":"arn:aws:iam::123456789012:user/admin","accountId":"123456789012"},"eventTime":"2026-03-28T13:45:22Z","eventSource":"ec2.amazonaws.com","eventName":"RunInstances","awsRegion":"us-east-1","sourceIPAddress":"198.51.100.20","userAgent":"console.ec2.amazonaws.com","requestParameters":{"instanceType":"t3.large","instancesSet":{"items":[{"imageId":"ami-0abc123def456789"}]},"subnetId":"subnet-0a1b2c3d4e5f6001"},"responseElements":{"instancesSet":{"items":[{"instanceId":"i-0a1b2c3d4e5f60007"}]}},"requestID":"req-ec2-run-001","eventID":"evt-0a1b2c3d4e5f60002","readOnly":false,"eventType":"AwsApiCall","managementEvent":true}`),
				Resources: []cloudtrailtypes.Resource{
					{
						ResourceType: aws.String("AWS::EC2::Instance"),
						ResourceName: aws.String("i-0a1b2c3d4e5f60007"),
					},
				},
			},
		},
		{
			ID:     "evt-0a1b2c3d4e5f60003",
			Name:   "StopInstances",
			Status: "false",
			Fields: map[string]string{
				"event_name":    "StopInstances",
				"time":          t3.Format("2006-01-02 15:04:05"),
				"user":          "admin",
				"source":        "ec2.amazonaws.com",
				"resource_type": "AWS::EC2::Instance",
				"resource_name": "i-0a1b2c3d4e5f60004",
				"read_only":     "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:         aws.String("evt-0a1b2c3d4e5f60003"),
				EventName:       aws.String("StopInstances"),
				EventTime:       aws.Time(t3),
				EventSource:     aws.String("ec2.amazonaws.com"),
				Username:        aws.String("admin"),
				ReadOnly:        aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"IAMUser","principalId":"AIDAEXAMPLE002","arn":"arn:aws:iam::123456789012:user/admin","accountId":"123456789012"},"eventTime":"2026-03-28T12:10:05Z","eventSource":"ec2.amazonaws.com","eventName":"StopInstances","awsRegion":"us-east-1","sourceIPAddress":"198.51.100.20","userAgent":"console.ec2.amazonaws.com","requestParameters":{"instancesSet":{"items":[{"instanceId":"i-0a1b2c3d4e5f60004"}]},"force":false},"responseElements":{"instancesSet":{"items":[{"instanceId":"i-0a1b2c3d4e5f60004","currentState":{"code":64,"name":"stopping"}}]}},"requestID":"req-ec2-stop-001","eventID":"evt-0a1b2c3d4e5f60003","readOnly":false,"eventType":"AwsApiCall","managementEvent":true}`),
				Resources: []cloudtrailtypes.Resource{
					{
						ResourceType: aws.String("AWS::EC2::Instance"),
						ResourceName: aws.String("i-0a1b2c3d4e5f60004"),
					},
				},
			},
		},
		{
			ID:     "evt-0a1b2c3d4e5f60004",
			Name:   "DeleteTable",
			Status: "false",
			Fields: map[string]string{
				"event_name":    "DeleteTable",
				"time":          t4.Format("2006-01-02 15:04:05"),
				"user":          "ci-runner",
				"source":        "dynamodb.amazonaws.com",
				"resource_type": "AWS::DynamoDB::Table",
				"resource_name": "acme-sessions",
				"read_only":     "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:         aws.String("evt-0a1b2c3d4e5f60004"),
				EventName:       aws.String("DeleteTable"),
				EventTime:       aws.Time(t4),
				EventSource:     aws.String("dynamodb.amazonaws.com"),
				Username:        aws.String("ci-runner"),
				ReadOnly:        aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","principalId":"AROAEXAMPLE003","arn":"arn:aws:sts::123456789012:assumed-role/ci-runner/build-session","accountId":"123456789012","sessionContext":{"sessionIssuer":{"type":"Role","arn":"arn:aws:iam::123456789012:role/ci-runner","userName":"ci-runner"}}},"eventTime":"2026-03-28T11:55:48Z","eventSource":"dynamodb.amazonaws.com","eventName":"DeleteTable","awsRegion":"us-east-1","sourceIPAddress":"198.51.100.30","userAgent":"aws-sdk-go-v2/1.25.0","requestParameters":{"tableName":"acme-sessions-staging"},"responseElements":{"tableDescription":{"tableName":"acme-sessions-staging","tableStatus":"DELETING"}},"requestID":"req-ddb-del-001","eventID":"evt-0a1b2c3d4e5f60004","readOnly":false,"eventType":"AwsApiCall","managementEvent":true}`),
				Resources: []cloudtrailtypes.Resource{
					{
						ResourceType: aws.String("AWS::DynamoDB::Table"),
						ResourceName: aws.String("acme-sessions"),
					},
				},
			},
		},
		{
			ID:     "evt-0a1b2c3d4e5f60005",
			Name:   "AssumeRole",
			Status: "false",
			Fields: map[string]string{
				"event_name":    "AssumeRole",
				"time":          t5.Format("2006-01-02 15:04:05"),
				"user":          "deploy-bot",
				"source":        "sts.amazonaws.com",
				"resource_type": "AWS::IAM::Role",
				"resource_name": "acme-ci-deploy-role",
				"read_only":     "false",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:         aws.String("evt-0a1b2c3d4e5f60005"),
				EventName:       aws.String("AssumeRole"),
				EventTime:       aws.Time(t5),
				EventSource:     aws.String("sts.amazonaws.com"),
				Username:        aws.String("deploy-bot"),
				ReadOnly:        aws.String("false"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","principalId":"AROAEXAMPLE001","arn":"arn:aws:sts::123456789012:assumed-role/deploy-bot/session","accountId":"123456789012","sessionContext":{"sessionIssuer":{"type":"Role","arn":"arn:aws:iam::123456789012:role/deploy-bot","userName":"deploy-bot"}}},"eventTime":"2026-03-28T10:20:33Z","eventSource":"sts.amazonaws.com","eventName":"AssumeRole","awsRegion":"us-east-1","sourceIPAddress":"198.51.100.10","userAgent":"aws-cli/2.15.0","requestParameters":{"roleArn":"arn:aws:iam::123456789012:role/acme-deploy-role","roleSessionName":"deploy-session","durationSeconds":3600},"responseElements":{"credentials":{"accessKeyId":"ASIAEXAMPLE001","expiration":"2026-03-28T11:20:33Z"},"assumedRoleUser":{"arn":"arn:aws:sts::123456789012:assumed-role/acme-deploy-role/deploy-session"}},"requestID":"req-sts-assume-001","eventID":"evt-0a1b2c3d4e5f60005","readOnly":false,"eventType":"AwsApiCall","managementEvent":true}`),
				Resources: []cloudtrailtypes.Resource{
					{
						ResourceType: aws.String("AWS::IAM::Role"),
						ResourceName: aws.String("acme-ci-deploy-role"),
					},
				},
			},
		},
		{
			ID:     "evt-0a1b2c3d4e5f60006",
			Name:   "DescribeInstances",
			Status: "true",
			Fields: map[string]string{
				"event_name":    "DescribeInstances",
				"time":          t6.Format("2006-01-02 15:04:05"),
				"user":          "monitoring-agent",
				"source":        "ec2.amazonaws.com",
				"resource_type": "",
				"resource_name": "",
				"read_only":     "true",
			},
			RawStruct: cloudtrailtypes.Event{
				EventId:         aws.String("evt-0a1b2c3d4e5f60006"),
				EventName:       aws.String("DescribeInstances"),
				EventTime:       aws.Time(t6),
				EventSource:     aws.String("ec2.amazonaws.com"),
				Username:        aws.String("monitoring-agent"),
				ReadOnly:        aws.String("true"),
				CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","principalId":"AROAEXAMPLE004","arn":"arn:aws:sts::123456789012:assumed-role/monitoring-agent/session","accountId":"123456789012","sessionContext":{"sessionIssuer":{"type":"Role","arn":"arn:aws:iam::123456789012:role/monitoring-agent","userName":"monitoring-agent"}}},"eventTime":"2026-03-28T09:05:11Z","eventSource":"ec2.amazonaws.com","eventName":"DescribeInstances","awsRegion":"us-east-1","sourceIPAddress":"198.51.100.40","userAgent":"aws-sdk-python/1.34.0","requestParameters":{"instancesSet":{},"filterSet":{}},"responseElements":null,"requestID":"req-ec2-desc-001","eventID":"evt-0a1b2c3d4e5f60006","readOnly":true,"eventType":"AwsApiCall","managementEvent":true}`),
				Resources:       []cloudtrailtypes.Resource{},
			},
		},
	}
}
