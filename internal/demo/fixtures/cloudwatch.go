package fixtures

import (
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

// CloudWatchFixtures holds typed fixture data for CloudWatch.
type CloudWatchFixtures struct {
	Alarms []cwtypes.MetricAlarm
	// AlarmHistory maps AlarmName → history items (for DescribeAlarmHistory).
	// A missing entry means no history (child view lands empty) — every
	// graph-root-reachable alarm must have an entry so the alarm→alarm_history
	// drill lands on non-empty content.
	AlarmHistory map[string][]cwtypes.AlarmHistoryItem
}

const relatedAlarmSNSARN = "arn:aws:sns:us-east-1:123456789012:ops-alerts"

// NewCloudWatchFixtures constructs CloudWatchFixtures from the canonical demo data.
var sharedCloudWatchFixtures = sync.OnceValue(func() *CloudWatchFixtures {
	return &CloudWatchFixtures{
		Alarms: []cwtypes.MetricAlarm{
			{
				AlarmName:                  aws.String("api-high-error-rate"),
				AlarmArn:                   aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:api-high-error-rate"),
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
				AlarmActions:               []string{relatedAlarmSNSARN},
				OKActions:                  []string{relatedAlarmSNSARN},
				InsufficientDataActions:    []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("InstanceId"), Value: aws.String("i-0a1b2c3d4e5f60001")},
				},
			},
			{
				AlarmName:             aws.String("rds-cpu-utilization"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:rds-cpu-utilization"),
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
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("DBInstanceIdentifier"), Value: aws.String("prod-api-primary")},
				},
			},
			{
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
				AlarmActions:       []string{"arn:aws:sns:us-east-1:123456789012:ops-critical"},
			},
			{
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
			{
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
			// Issue: INSUFFICIENT_DATA state lasting <1h with very short period → Broken
			// StateUpdatedTimestamp ~1h ago means metric data recently stopped flowing.
			{
				AlarmName:               aws.String("alarm-stale-insufficient"),
				AlarmArn:                aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:alarm-stale-insufficient"),
				AlarmDescription:        aws.String("Alarm with no recent metric data — stream appears stalled"),
				StateValue:              cwtypes.StateValueInsufficientData,
				StateReason:             aws.String("Insufficient Data: 1 datapoint was not found for the metric in the past 60 seconds."),
				StateUpdatedTimestamp:   aws.Time(time.Date(2026, 4, 18, 7, 0, 0, 0, time.UTC)),
				MetricName:              aws.String("RequestCount"),
				Namespace:               aws.String("AWS/ApplicationELB"),
				Threshold:               aws.Float64(100.0),
				ComparisonOperator:      cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:       aws.Int32(1),
				Period:                  aws.Int32(60),
				Statistic:               cwtypes.StatisticSum,
				ActionsEnabled:          aws.Bool(true),
				AlarmActions:            []string{relatedAlarmSNSARN},
				InsufficientDataActions: []string{relatedAlarmSNSARN},
			},
			// prod-dbi-1 alarms — required for dbi→alarm related-panel pivot.
			{
				AlarmName:             aws.String("rds-prod-dbi-1-free-storage"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:rds-prod-dbi-1-free-storage"),
				AlarmDescription:      aws.String("Triggers when prod-dbi-1 free storage drops below 10 GB"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were greater than the threshold (10000000000.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 18, 6, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("FreeStorageSpace"),
				Namespace:             aws.String("AWS/RDS"),
				Threshold:             aws.Float64(10_000_000_000),
				ComparisonOperator:    cwtypes.ComparisonOperatorLessThanThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticAverage,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("DBInstanceIdentifier"), Value: aws.String("prod-dbi-1")},
				},
			},
			{
				AlarmName:             aws.String("rds-prod-dbi-1-cpu"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:rds-prod-dbi-1-cpu"),
				AlarmDescription:      aws.String("Triggers when prod-dbi-1 CPU exceeds 80%"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 5 datapoints were less than the threshold (80.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("CPUUtilization"),
				Namespace:             aws.String("AWS/RDS"),
				Threshold:             aws.Float64(80.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(5),
				Period:                aws.Int32(60),
				Statistic:             cwtypes.StatisticAverage,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("DBInstanceIdentifier"), Value: aws.String("prod-dbi-1")},
				},
			},
			// prod-dbi-aurora-1 alarm — required for the "all pivots
			// non-zero" graph-root assertion on the Aurora dbi fixture.
			{
				AlarmName:             aws.String("rds-prod-dbi-aurora-1-cpu"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:rds-prod-dbi-aurora-1-cpu"),
				AlarmDescription:      aws.String("Triggers when prod-dbi-aurora-1 CPU exceeds 75%"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 5 datapoints were less than the threshold (75.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("CPUUtilization"),
				Namespace:             aws.String("AWS/RDS"),
				Threshold:             aws.Float64(75.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(5),
				Period:                aws.Int32(60),
				Statistic:             cwtypes.StatisticAverage,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("DBInstanceIdentifier"), Value: aws.String("prod-dbi-aurora-1")},
				},
			},
			// acme-docdb-prod alarm — required for dbc→alarm related-panel pivot.
			// Dimension DBClusterIdentifier matches fixtures.ProdDbcID.
			{
				AlarmName:             aws.String("docdb-acme-prod-cpu"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:docdb-acme-prod-cpu"),
				AlarmDescription:      aws.String("Triggers when acme-docdb-prod CPU exceeds 80%"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 5 datapoints were less than the threshold (80.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 18, 6, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("CPUUtilization"),
				Namespace:             aws.String("AWS/DocDB"),
				Threshold:             aws.Float64(80.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(5),
				Period:                aws.Int32(60),
				Statistic:             cwtypes.StatisticAverage,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("DBClusterIdentifier"), Value: aws.String("acme-docdb-prod")},
				},
			},
			// prod-aurora-cluster alarm — required for the dbc→alarm pivot
			// on the Aurora cluster "all pivots non-zero" graph-root.
			{
				AlarmName:             aws.String("aurora-prod-cluster-cpu"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:aurora-prod-cluster-cpu"),
				AlarmDescription:      aws.String("Triggers when prod-aurora-cluster CPU exceeds 80%"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 5 datapoints were less than the threshold (80.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 18, 6, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("CPUUtilization"),
				Namespace:             aws.String("AWS/RDS"),
				Threshold:             aws.Float64(80.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(5),
				Period:                aws.Int32(60),
				Statistic:             cwtypes.StatisticAverage,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("DBClusterIdentifier"), Value: aws.String("prod-aurora-cluster")},
				},
			},
			// Redis prod alarm — required for redis→alarm related-panel pivot.
			// Dimension CacheClusterId matches ProdRedisMemberClusterID so
			// checkRedisAlarms resolves a non-zero count for the demo showroom.
			{
				AlarmName:             aws.String("redis-prod-cache-hits"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:redis-prod-cache-hits"),
				AlarmDescription:      aws.String("Triggers when ElastiCache Redis cache-hit ratio drops below 80%"),
				StateValue:            cwtypes.StateValueAlarm,
				StateReason:           aws.String("Threshold Crossed: 1 datapoint [72.3] was less than or equal to the threshold (80.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("CacheHitRate"),
				Namespace:             aws.String("AWS/ElastiCache"),
				Threshold:             aws.Float64(80.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorLessThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(3),
				DatapointsToAlarm:     aws.Int32(2),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticAverage,
				TreatMissingData:      aws.String("missing"),
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{ProdRedisSNSTopicARN},
				OKActions:             []string{ProdRedisSNSTopicARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("CacheClusterId"), Value: aws.String(ProdRedisMemberClusterID)},
				},
			},
			// orders-prod-throttle — DDB→alarm pivot: Dimensions[TableName=orders-prod].
			{
				AlarmName:             aws.String("orders-prod-throttle"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:orders-prod-throttle"),
				AlarmDescription:      aws.String("Triggers when orders-prod DynamoDB read/write throttle events exceed threshold"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were less than or equal to the threshold (100.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("ThrottledRequests"),
				Namespace:             aws.String("AWS/DynamoDB"),
				Threshold:             aws.Float64(100.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticSum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				OKActions:             []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("TableName"), Value: aws.String(OrdersProdID)},
				},
			},
			// OpenSearch graph-root alarms — required for opensearch→alarm related-panel pivot.
			// Namespace=AWS/ES with Dimensions.DomainName=acme-logs so checkOpenSearchAlarms
			// resolves a count of 2 for the demo showroom.
			{
				AlarmName:             aws.String("acme-logs-cluster-red"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:acme-logs-cluster-red"),
				AlarmDescription:      aws.String("Triggers when acme-logs OpenSearch cluster status is red"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were less than or equal to the threshold (1.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("ClusterStatus.red"),
				Namespace:             aws.String("AWS/ES"),
				Threshold:             aws.Float64(1.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(1),
				Period:                aws.Int32(60),
				Statistic:             cwtypes.StatisticMaximum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				OKActions:             []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("DomainName"), Value: aws.String(GraphRootDomain)},
				},
			},
			{
				AlarmName:             aws.String("acme-logs-freestorage-low"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:acme-logs-freestorage-low"),
				AlarmDescription:      aws.String("Triggers when acme-logs OpenSearch free storage drops below 10 GB"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were greater than the threshold (10000000000.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 14, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("FreeStorageSpace"),
				Namespace:             aws.String("AWS/ES"),
				Threshold:             aws.Float64(10_000_000_000),
				ComparisonOperator:    cwtypes.ComparisonOperatorLessThanThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticAverage,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				OKActions:             []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("DomainName"), Value: aws.String(GraphRootDomain)},
				},
			},
			// acme-warehouse alarms — required for redshift→alarm related-panel pivot.
			// Dimension ClusterIdentifier=acme-warehouse matches checkRedshiftAlarms.
			{
				AlarmName:             aws.String("redshift-acme-warehouse-cpu"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:redshift-acme-warehouse-cpu"),
				AlarmDescription:      aws.String("Triggers when acme-warehouse CPU utilization exceeds 80%"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 5 datapoints were less than the threshold (80.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("CPUUtilization"),
				Namespace:             aws.String("AWS/Redshift"),
				Threshold:             aws.Float64(80.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(5),
				Period:                aws.Int32(60),
				Statistic:             cwtypes.StatisticAverage,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("ClusterIdentifier"), Value: aws.String(AcmeWarehouseID)},
				},
			},
			{
				AlarmName:             aws.String("redshift-acme-warehouse-disk"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:redshift-acme-warehouse-disk"),
				AlarmDescription:      aws.String("Triggers when acme-warehouse disk space utilization exceeds 85%"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were less than the threshold (85.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("PercentageDiskSpaceUsed"),
				Namespace:             aws.String("AWS/Redshift"),
				Threshold:             aws.Float64(85.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticAverage,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("ClusterIdentifier"), Value: aws.String(AcmeWarehouseID)},
				},
			},
			// acme-reporting alarm — required for redshift→alarm related-panel pivot (second graph-root).
			{
				AlarmName:             aws.String("redshift-acme-reporting-cpu"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:redshift-acme-reporting-cpu"),
				AlarmDescription:      aws.String("Triggers when acme-reporting CPU utilization exceeds 75%"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 5 datapoints were less than the threshold (75.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("CPUUtilization"),
				Namespace:             aws.String("AWS/Redshift"),
				Threshold:             aws.Float64(75.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(5),
				Period:                aws.Int32(60),
				Statistic:             cwtypes.StatisticAverage,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("ClusterIdentifier"), Value: aws.String(AcmeReportingID)},
				},
			},
			// acme-services ECS cluster alarm — required for ecs→alarm related-panel pivot.
			// Dimension ClusterName matches the acme-services cluster (ecs.go).
			{
				AlarmName:             aws.String("ecs-acme-services-cpu-reservation"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:ecs-acme-services-cpu-reservation"),
				AlarmDescription:      aws.String("Triggers when acme-services cluster CPU reservation exceeds 90%"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were less than the threshold (90.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 19, 7, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("CPUReservation"),
				Namespace:             aws.String("AWS/ECS"),
				Threshold:             aws.Float64(90.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticAverage,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("ClusterName"), Value: aws.String("acme-services")},
				},
			},
			// api-gateway ECS service alarm — required for ecs-svc→alarm related-panel
			// pivot. Dimensions match ServiceName=api-gateway + ClusterName=acme-services.
			{
				AlarmName:             aws.String("ecs-svc-api-gateway-running-count"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:ecs-svc-api-gateway-running-count"),
				AlarmDescription:      aws.String("Triggers when api-gateway running task count drops below desired"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were greater than or equal to the threshold (4.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 19, 8, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("RunningTaskCount"),
				Namespace:             aws.String("ECS/ContainerInsights"),
				Threshold:             aws.Float64(4.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorLessThanThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(60),
				Statistic:             cwtypes.StatisticMinimum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("ServiceName"), Value: aws.String("api-gateway")},
					{Name: aws.String("ClusterName"), Value: aws.String("acme-services")},
				},
			},
			// api-gateway ECS task alarm — required for ecs-task→alarm related-panel
			// pivot. Dimension TaskId matches the STOPPED api-gateway task UUID.
			{
				AlarmName:             aws.String("ecs-task-api-gateway-memory-util"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:ecs-task-api-gateway-memory-util"),
				AlarmDescription:      aws.String("Triggers when task memory utilization exceeds 90%"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were less than the threshold (90.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 3, 20, 8, 20, 0, 0, time.UTC)),
				MetricName:            aws.String("MemoryUtilized"),
				Namespace:             aws.String("ECS/ContainerInsights"),
				Threshold:             aws.Float64(90.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(60),
				Statistic:             cwtypes.StatisticAverage,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("TaskId"), Value: aws.String("a1b2c3d4e5f6a1b2c3d4e5f6")},
				},
			},
			// acme-prod EKS cluster alarm — required for eks→alarm related-panel pivot.
			// Dimension ClusterName matches the acme-prod EKS cluster (eks.go).
			{
				AlarmName:             aws.String("eks-acme-prod-control-plane-errors"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:eks-acme-prod-control-plane-errors"),
				AlarmDescription:      aws.String("Triggers when EKS control-plane API server error rate exceeds threshold"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were less than the threshold (5.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 18, 9, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("apiserver_request_total"),
				Namespace:             aws.String("AWS/EKS"),
				Threshold:             aws.Float64(5.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticSum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("ClusterName"), Value: aws.String("acme-prod")},
				},
			},
			// api-gateway-authorizer Lambda alarm — required for lambda→alarm
			// related-panel pivot. Dimension FunctionName matches the function
			// name (lambda.go).
			{
				AlarmName:             aws.String("lambda-api-gateway-authorizer-errors"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:lambda-api-gateway-authorizer-errors"),
				AlarmDescription:      aws.String("Triggers when api-gateway-authorizer error count exceeds 5 in 5 minutes"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were less than the threshold (5.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 19, 9, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("Errors"),
				Namespace:             aws.String("AWS/Lambda"),
				Threshold:             aws.Float64(5.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:     aws.Int32(2),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticSum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("FunctionName"), Value: aws.String("api-gateway-authorizer")},
				},
			},
			// web-prod-01-root volume alarm — required for ebs→alarm related-panel pivot.
			// Dimension VolumeId matches vol-0a1b2c3d4e5f60001 (ec2.go buildVolumes).
			{
				AlarmName:             aws.String("ebs-web-prod-root-burst-balance"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:ebs-web-prod-root-burst-balance"),
				AlarmDescription:      aws.String("Triggers when EBS burst balance drops below 20%"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were greater than the threshold (20.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 19, 6, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("BurstBalance"),
				Namespace:             aws.String("AWS/EBS"),
				Threshold:             aws.Float64(20.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorLessThanThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticAverage,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("VolumeId"), Value: aws.String("vol-0a1b2c3d4e5f60001")},
				},
			},
			// Issue: OK state but ActionsEnabled=false → Warning (alarm silenced/muted)
			{
				AlarmName:             aws.String("alarm-muted"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:alarm-muted"),
				AlarmDescription:      aws.String("Alarm with actions disabled — notifications will not fire"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were less than the threshold (50.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("CPUUtilization"),
				Namespace:             aws.String("AWS/EC2"),
				Threshold:             aws.Float64(50.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticAverage,
				ActionsEnabled:        aws.Bool(false),
				AlarmActions:          []string{relatedAlarmSNSARN},
				OKActions:             []string{relatedAlarmSNSARN},
			},
			// EFS prod-app-data alarms — required for efs→alarm related-panel pivot (Count = 2).
			// checkEFSAlarm matches Namespace=AWS/EFS AND Dimensions[Name=FileSystemId, Value=ProdEFSID].
			{
				AlarmName:             aws.String(ProdEFSAlarmAID),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:" + ProdEFSAlarmAID),
				AlarmDescription:      aws.String("Triggers when EFS burst credit balance drops below threshold"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were greater than the threshold (1000000000.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 20, 8, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("BurstCreditBalance"),
				Namespace:             aws.String("AWS/EFS"),
				Threshold:             aws.Float64(1_000_000_000.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorLessThanThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticMinimum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				OKActions:             []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("FileSystemId"), Value: aws.String(ProdEFSID)},
				},
			},
			{
				AlarmName:             aws.String(ProdEFSAlarmBID),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:" + ProdEFSAlarmBID),
				AlarmDescription:      aws.String("Triggers when EFS PercentIOLimit exceeds 90% sustained"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were less than the threshold (90.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 20, 8, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("PercentIOLimit"),
				Namespace:             aws.String("AWS/EFS"),
				Threshold:             aws.Float64(90.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticAverage,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				OKActions:             []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("FileSystemId"), Value: aws.String(ProdEFSID)},
				},
			},
			// acme-public-api alarm — required for apigw:alarm related-panel
			// pivot. Dimension ApiId matches PublicAPIGWID (apigw.go).
			// checkApigwAlarm scans MetricAlarm.Dimensions for Name="ApiId".
			{
				AlarmName:             aws.String("apigw-public-api-5xx"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:apigw-public-api-5xx"),
				AlarmDescription:      aws.String("Triggers when acme-public-api 5XX error count exceeds 10 in 5 minutes"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were less than the threshold (10.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 21, 11, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("5XXError"),
				Namespace:             aws.String("AWS/ApiGateway"),
				Threshold:             aws.Float64(10.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticSum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("ApiId"), Value: aws.String(PublicAPIGWID)},
				},
			},
			// acme-api-build alarm — required for cb:alarm related-panel
			// pivot. checkCbAlarm matches Namespace="AWS/CodeBuild" +
			// dimension ProjectName=acme-api-build.
			{
				AlarmName:             aws.String("cb-acme-api-build-failed-builds"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:cb-acme-api-build-failed-builds"),
				AlarmDescription:      aws.String("Triggers when acme-api-build has 2+ failed builds in 15 minutes"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 1 datapoint was less than the threshold (2.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("FailedBuilds"),
				Namespace:             aws.String("AWS/CodeBuild"),
				Threshold:             aws.Float64(2.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(1),
				Period:                aws.Int32(900),
				Statistic:             cwtypes.StatisticSum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("ProjectName"), Value: aws.String("acme-api-build")},
				},
			},
			// order-fulfillment-workflow alarm — required for sfn:alarm
			// related-panel pivot. checkSFNAlarm matches dimension
			// StateMachineArn to the state machine's full ARN (sfn.go).
			{
				AlarmName:             aws.String("sfn-order-fulfillment-execution-failures"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:sfn-order-fulfillment-execution-failures"),
				AlarmDescription:      aws.String("Triggers when order-fulfillment-workflow has 3+ failed executions in 15 minutes"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 1 datapoint was less than the threshold (3.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 4, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("ExecutionsFailed"),
				Namespace:             aws.String("AWS/States"),
				Threshold:             aws.Float64(3.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(1),
				Period:                aws.Int32(900),
				Statistic:             cwtypes.StatisticSum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("StateMachineArn"), Value: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:order-fulfillment-workflow")},
				},
			},
			// acme-prod-api alarm — required for eb:alarm related-panel
			// pivot. checkEbAlarm matches any dimension Value equal to the
			// environment name (acme-prod-api).
			{
				AlarmName:             aws.String("eb-acme-prod-api-health"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:eb-acme-prod-api-health"),
				AlarmDescription:      aws.String("Triggers when acme-prod-api environment health degrades"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 1 datapoint was less than the threshold (1.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 5, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("EnvironmentHealth"),
				Namespace:             aws.String("AWS/ElasticBeanstalk"),
				Threshold:             aws.Float64(1.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(1),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticMaximum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("EnvironmentName"), Value: aws.String("acme-prod-api")},
				},
			},
			// cf-E1A2B3C4D5E6F7 alarm — required for cf:alarm related-panel
			// pivot. checkCfAlarm matches dimension DistributionId to the
			// distribution's Id (cloudfront.go E1A2B3C4D5E6F7).
			{
				AlarmName:             aws.String("cf-e1a2b3c4d5e6f7-error-rate"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:cf-e1a2b3c4d5e6f7-error-rate"),
				AlarmDescription:      aws.String("Triggers when acme-corp.com CDN 5xx error rate exceeds 5%"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were less than the threshold (5.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 6, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("5xxErrorRate"),
				Namespace:             aws.String("AWS/CloudFront"),
				Threshold:             aws.Float64(5.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticAverage,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("DistributionId"), Value: aws.String("E1A2B3C4D5E6F7")},
				},
			},
			// acme-prod-web alarm — required for elb:alarm related-panel
			// pivot. checkELBAlarms matches dimension LoadBalancer to the ARN
			// suffix after "loadbalancer/" (elb.go fixtProdELBARN).
			{
				AlarmName:             aws.String("elb-acme-prod-web-5xx"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:elb-acme-prod-web-5xx"),
				AlarmDescription:      aws.String("Triggers when acme-prod-web 5XX count exceeds 10 in 5 minutes"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 2 datapoints were less than the threshold (10.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 6, 5, 0, 0, time.UTC)),
				MetricName:            aws.String("HTTPCode_Target_5XX_Count"),
				Namespace:             aws.String("AWS/ApplicationELB"),
				Threshold:             aws.Float64(10.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:     aws.Int32(2),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticSum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("LoadBalancer"), Value: aws.String("app/acme-prod-web/1234567890abcdef")},
				},
			},
			// acme-web-tg alarm — required for tg:alarm related-panel pivot.
			// checkTGAlarm matches dimension TargetGroup containing the TG's
			// ARN suffix (elb.go fixtProdWebTGARN).
			{
				AlarmName:             aws.String("tg-acme-web-tg-unhealthy-hosts"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:tg-acme-web-tg-unhealthy-hosts"),
				AlarmDescription:      aws.String("Triggers when acme-web-tg has 1+ unhealthy hosts"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were less than the threshold (1.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 6, 10, 0, 0, time.UTC)),
				MetricName:            aws.String("UnHealthyHostCount"),
				Namespace:             aws.String("AWS/ApplicationELB"),
				Threshold:             aws.Float64(1.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(60),
				Statistic:             cwtypes.StatisticMaximum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("TargetGroup"), Value: aws.String("targetgroup/acme-web-tg/1234567890abcdef")},
				},
			},
			// nat-0aaa111111111111a alarm — required for nat:alarm related-panel
			// pivot. checkNATAlarm matches dimension NatGatewayId (ec2.go).
			{
				AlarmName:             aws.String("nat-0aaa111111111111a-error-port-alloc"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:nat-0aaa111111111111a-error-port-alloc"),
				AlarmDescription:      aws.String("Triggers when prod-nat-1a has 1+ error port allocation errors"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 5 datapoints were less than the threshold (1.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 6, 15, 0, 0, time.UTC)),
				MetricName:            aws.String("ErrorPortAllocation"),
				Namespace:             aws.String("AWS/NATGateway"),
				Threshold:             aws.Float64(1.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(5),
				Period:                aws.Int32(60),
				Statistic:             cwtypes.StatisticSum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("NatGatewayId"), Value: aws.String("nat-0aaa111111111111a")},
				},
			},
			// vpce-0aaa111111111111a alarm — required for vpce:alarm
			// related-panel pivot. checkVPCEAlarm matches dimension
			// VpcEndpointId (ec2.go).
			{
				AlarmName:             aws.String("vpce-0aaa111111111111a-packet-drop"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:vpce-0aaa111111111111a-packet-drop"),
				AlarmDescription:      aws.String("Triggers when prod-s3-endpoint drops 100+ packets in 5 minutes"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were less than the threshold (100.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 6, 20, 0, 0, time.UTC)),
				MetricName:            aws.String("PacketDropCount"),
				Namespace:             aws.String("AWS/PrivateLinkEndpoints"),
				Threshold:             aws.Float64(100.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticSum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("VpcEndpointId"), Value: aws.String("vpce-0aaa111111111111a")},
				},
			},
			// acme-web-prod-asg alarm — required for alarm:asg related-panel
			// pivot. checkAlarmASG matches dimension AutoScalingGroupName
			// against the acme-web-prod-asg fixture (asg.go).
			{
				AlarmName:             aws.String("asg-acme-web-prod-group-in-service"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:asg-acme-web-prod-group-in-service"),
				AlarmDescription:      aws.String("Triggers when acme-web-prod-asg has fewer than 2 InService instances"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were greater than or equal to the threshold (2.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 6, 25, 0, 0, time.UTC)),
				MetricName:            aws.String("GroupInServiceInstances"),
				Namespace:             aws.String("AWS/AutoScaling"),
				Threshold:             aws.Float64(2.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorLessThanThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(60),
				Statistic:             cwtypes.StatisticMinimum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("AutoScalingGroupName"), Value: aws.String("acme-web-prod-asg")},
				},
			},
			// KMS key-usage alarm — required for alarm:kms related-panel pivot.
			// checkAlarmKMS matches dimension KeyId against the primary
			// production KMS key fixture (kms.go).
			{
				AlarmName:             aws.String("kms-primary-key-decrypt-errors"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:kms-primary-key-decrypt-errors"),
				AlarmDescription:      aws.String("Triggers when primary encryption key sees 5+ decrypt errors in 5 minutes"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 1 datapoint was less than the threshold (5.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 6, 30, 0, 0, time.UTC)),
				MetricName:            aws.String("SecondsUntilKeyMaterialExpiration"),
				Namespace:             aws.String("AWS/KMS"),
				Threshold:             aws.Float64(5.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:     aws.Int32(1),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticSum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("KeyId"), Value: aws.String("a1b2c3d4-5678-90ab-cdef-111111111111")},
				},
			},
			// Log group metric-filter alarm — required for alarm:logs
			// related-panel pivot. checkAlarmLogs matches dimension
			// LogGroupName against the /app/legacy/orphan-old fixture (cwlogs.go).
			{
				AlarmName:             aws.String("logs-orphan-old-error-count"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:logs-orphan-old-error-count"),
				AlarmDescription:      aws.String("Triggers when /app/legacy/orphan-old ERROR count exceeds 20 in 5 minutes"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 1 datapoint was less than the threshold (20.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 6, 35, 0, 0, time.UTC)),
				MetricName:            aws.String("ErrorCount"),
				Namespace:             aws.String("LogMetrics"),
				Threshold:             aws.Float64(20.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:     aws.Int32(1),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticSum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("LogGroupName"), Value: aws.String("/app/legacy/orphan-old")},
				},
			},
			// S3 request-metrics alarm — required for alarm:s3 related-panel
			// pivot. checkAlarmS3 matches dimension BucketName against the
			// graph-root healthy bucket (s3.go HealthyBucketName).
			{
				AlarmName:             aws.String("s3-healthy-bucket-4xx-errors"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:s3-healthy-bucket-4xx-errors"),
				AlarmDescription:      aws.String("Triggers when a9s-demo-healthy 4xx error count exceeds 50 in 5 minutes"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 1 datapoint was less than the threshold (50.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 6, 40, 0, 0, time.UTC)),
				MetricName:            aws.String("4xxErrors"),
				Namespace:             aws.String("AWS/S3"),
				Threshold:             aws.Float64(50.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:     aws.Int32(1),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticSum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("BucketName"), Value: aws.String(HealthyBucketName)},
				},
			},
			// WAF blocked-request alarm — required for alarm:waf related-panel
			// pivot. checkAlarmWAF matches dimension WebACL against the
			// acme-prod-api-waf fixture (waf.go).
			{
				AlarmName:             aws.String("waf-acme-prod-api-blocked-requests"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:waf-acme-prod-api-blocked-requests"),
				AlarmDescription:      aws.String("Triggers when acme-prod-api-waf blocks 100+ requests in 5 minutes"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 1 datapoint was less than the threshold (100.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 6, 45, 0, 0, time.UTC)),
				MetricName:            aws.String("BlockedRequests"),
				Namespace:             aws.String("AWS/WAFV2"),
				Threshold:             aws.Float64(100.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:     aws.Int32(1),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticSum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("WebACL"), Value: aws.String("acme-prod-api-waf")},
				},
			},
			// acme-etl-orders alarm — required for glue:alarm related-panel
			// pivot. checkGlueAlarms matches dimension JobName.
			{
				AlarmName:             aws.String("glue-acme-etl-orders-failed-runs"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:glue-acme-etl-orders-failed-runs"),
				AlarmDescription:      aws.String("Triggers when acme-etl-orders has 1+ failed job runs in 1 hour"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 1 datapoint was less than the threshold (1.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 6, 50, 0, 0, time.UTC)),
				MetricName:            aws.String("glue.driver.aggregate.numFailedTasks"),
				Namespace:             aws.String("Glue"),
				Threshold:             aws.Float64(1.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(1),
				Period:                aws.Int32(3600),
				Statistic:             cwtypes.StatisticSum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("JobName"), Value: aws.String("acme-etl-orders")},
				},
			},
			// clickstream-ingest alarm — required for kinesis:alarm related-panel
			// pivot. checkKinesisAlarms matches dimension StreamName.
			{
				AlarmName:             aws.String("kinesis-clickstream-ingest-iterator-age"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:kinesis-clickstream-ingest-iterator-age"),
				AlarmDescription:      aws.String("Triggers when clickstream-ingest consumer iterator age exceeds 60s"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were less than the threshold (60000.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 6, 55, 0, 0, time.UTC)),
				MetricName:            aws.String("GetRecords.IteratorAgeMilliseconds"),
				Namespace:             aws.String("AWS/Kinesis"),
				Threshold:             aws.Float64(60000.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(60),
				Statistic:             cwtypes.StatisticMaximum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("StreamName"), Value: aws.String("clickstream-ingest")},
				},
			},
			// acme-events-prod alarm — required for msk:alarm related-panel
			// pivot. checkMSKAlarms matches dimension "Cluster Name".
			{
				AlarmName:             aws.String("msk-acme-events-prod-under-replicated"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:msk-acme-events-prod-under-replicated"),
				AlarmDescription:      aws.String("Triggers when acme-events-prod has under-replicated partitions"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were less than the threshold (1.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 7, 0, 0, 0, time.UTC)),
				MetricName:            aws.String("UnderReplicatedPartitions"),
				Namespace:             aws.String("AWS/Kafka"),
				Threshold:             aws.Float64(1.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticMaximum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("Cluster Name"), Value: aws.String("acme-events-prod")},
				},
			},
			// order-processing-queue alarm — required for sqs:alarm related-panel
			// pivot. checkSQSAlarm matches Namespace=AWS/SQS + dimension QueueName.
			{
				AlarmName:             aws.String("sqs-order-processing-queue-oldest-message-age"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:sqs-order-processing-queue-oldest-message-age"),
				AlarmDescription:      aws.String("Triggers when order-processing-queue oldest message age exceeds 300s"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were less than the threshold (300.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 7, 5, 0, 0, time.UTC)),
				MetricName:            aws.String("ApproximateAgeOfOldestMessage"),
				Namespace:             aws.String("AWS/SQS"),
				Threshold:             aws.Float64(300.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(60),
				Statistic:             cwtypes.StatisticMaximum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("QueueName"), Value: aws.String("order-processing-queue")},
				},
			},
			// prod-airflow-etl alarms — required for mwaa→alarm related-panel
			// pivot (Count = 2). checkMWAAAlarm matches Namespace=AWS/MWAA AND
			// Dimensions[Name=EnvironmentName, Value=ProdAirflowEtlID] (no ARN
			// field on Environment — workflow pivot by dimension only).
			{
				AlarmName:             aws.String("mwaa-prod-airflow-etl-scheduler-heartbeat"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:mwaa-prod-airflow-etl-scheduler-heartbeat"),
				AlarmDescription:      aws.String("Triggers when prod-airflow-etl scheduler heartbeat is missed for 5 minutes"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 3 datapoints were greater than the threshold (0.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 7, 10, 0, 0, time.UTC)),
				MetricName:            aws.String("SchedulerHeartbeat"),
				Namespace:             aws.String("AWS/MWAA"),
				Threshold:             aws.Float64(0.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorLessThanOrEqualToThreshold,
				EvaluationPeriods:     aws.Int32(3),
				Period:                aws.Int32(300),
				Statistic:             cwtypes.StatisticSum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("EnvironmentName"), Value: aws.String(ProdAirflowEtlID)},
				},
			},
			{
				AlarmName:             aws.String("mwaa-prod-airflow-etl-task-import-errors"),
				AlarmArn:              aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:mwaa-prod-airflow-etl-task-import-errors"),
				AlarmDescription:      aws.String("Triggers when prod-airflow-etl DAG import errors exceed 3 in 15 minutes"),
				StateValue:            cwtypes.StateValueOk,
				StateReason:           aws.String("Threshold Crossed: 1 datapoint was less than the threshold (3.0)."),
				StateUpdatedTimestamp: aws.Time(time.Date(2026, 4, 22, 7, 15, 0, 0, time.UTC)),
				MetricName:            aws.String("DagProcessing.import_errors"),
				Namespace:             aws.String("AWS/MWAA"),
				Threshold:             aws.Float64(3.0),
				ComparisonOperator:    cwtypes.ComparisonOperatorGreaterThanThreshold,
				EvaluationPeriods:     aws.Int32(1),
				Period:                aws.Int32(900),
				Statistic:             cwtypes.StatisticSum,
				ActionsEnabled:        aws.Bool(true),
				AlarmActions:          []string{relatedAlarmSNSARN},
				Dimensions: []cwtypes.Dimension{
					{Name: aws.String("EnvironmentName"), Value: aws.String(ProdAirflowEtlID)},
				},
			},
		},
		// AlarmHistory — every graph-root-reachable alarm needs at least one
		// entry so alarm→alarm_history drill lands on non-empty content.
		AlarmHistory: map[string][]cwtypes.AlarmHistoryItem{
			"orders-prod-throttle":                      minimalAlarmHistory("orders-prod-throttle"),
			"rds-prod-dbi-aurora-1-cpu":                 minimalAlarmHistory("rds-prod-dbi-aurora-1-cpu"),
			"docdb-acme-prod-cpu":                       minimalAlarmHistory("docdb-acme-prod-cpu"),
			"aurora-prod-cluster-cpu":                   minimalAlarmHistory("aurora-prod-cluster-cpu"),
			"redis-prod-cache-hits":                     minimalAlarmHistory("redis-prod-cache-hits"),
			"redshift-acme-reporting-cpu":               minimalAlarmHistory("redshift-acme-reporting-cpu"),
			"redshift-acme-warehouse-cpu":               minimalAlarmHistory("redshift-acme-warehouse-cpu"),
			"redshift-acme-warehouse-disk":              minimalAlarmHistory("redshift-acme-warehouse-disk"),
			"acme-logs-cluster-red":                     minimalAlarmHistory("acme-logs-cluster-red"),
			"acme-logs-freestorage-low":                 minimalAlarmHistory("acme-logs-freestorage-low"),
			"prod-efs-burst-credit-low":                 minimalAlarmHistory("prod-efs-burst-credit-low"),
			"prod-efs-percent-io-high":                  minimalAlarmHistory("prod-efs-percent-io-high"),
			"cf-e1a2b3c4d5e6f7-error-rate":              minimalAlarmHistory("cf-e1a2b3c4d5e6f7-error-rate"),
			"elb-acme-prod-web-5xx":                     minimalAlarmHistory("elb-acme-prod-web-5xx"),
			"tg-acme-web-tg-unhealthy-hosts":            minimalAlarmHistory("tg-acme-web-tg-unhealthy-hosts"),
			"nat-0aaa111111111111a-error-port-alloc":    minimalAlarmHistory("nat-0aaa111111111111a-error-port-alloc"),
			"vpce-0aaa111111111111a-packet-drop":        minimalAlarmHistory("vpce-0aaa111111111111a-packet-drop"),
			"mwaa-prod-airflow-etl-scheduler-heartbeat": minimalAlarmHistory("mwaa-prod-airflow-etl-scheduler-heartbeat"),
			"mwaa-prod-airflow-etl-task-import-errors":  minimalAlarmHistory("mwaa-prod-airflow-etl-task-import-errors"),
		},
	}
})

func NewCloudWatchFixtures() *CloudWatchFixtures {
	return sharedCloudWatchFixtures()
}

// minimalAlarmHistory returns a canonical 3-item state sequence
// (OK → ALARM → OK) so the alarm_history child view has non-empty content.
// The two StateUpdate items carry HistoryData JSON matching their
// HistorySummary — real DescribeAlarmHistory always populates HistoryData
// with an {"oldState":...,"newState":...} payload for StateUpdate items;
// alarmHistoryFindings (internal/aws/alarm_history.go) parses this field to
// classify the ALARM transition as broken, mirroring colorAlarm's live-alarm
// classification.
func minimalAlarmHistory(alarmName string) []cwtypes.AlarmHistoryItem {
	t0 := time.Date(2026, 4, 20, 8, 0, 0, 0, time.UTC)
	return []cwtypes.AlarmHistoryItem{
		{
			AlarmName:       aws.String(alarmName),
			Timestamp:       aws.Time(t0.Add(2 * time.Hour)),
			HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
			HistorySummary:  aws.String("Alarm updated from ALARM to OK"),
			HistoryData:     aws.String(`{"version":"1.0","oldState":{"stateValue":"ALARM"},"newState":{"stateValue":"OK"}}`),
		},
		{
			AlarmName:       aws.String(alarmName),
			Timestamp:       aws.Time(t0.Add(1 * time.Hour)),
			HistoryItemType: cwtypes.HistoryItemTypeStateUpdate,
			HistorySummary:  aws.String("Alarm updated from OK to ALARM"),
			HistoryData:     aws.String(`{"version":"1.0","oldState":{"stateValue":"OK"},"newState":{"stateValue":"ALARM"}}`),
		},
		{
			AlarmName:       aws.String(alarmName),
			Timestamp:       aws.Time(t0),
			HistoryItemType: cwtypes.HistoryItemTypeConfigurationUpdate,
			HistorySummary:  aws.String("Alarm \"" + alarmName + "\" created"),
		},
	}
}
