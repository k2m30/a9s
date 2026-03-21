package demo

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	demoData["alarm"] = cloudwatchAlarmFixtures
	demoData["logs"] = cloudwatchLogGroupFixtures
	demoData["trail"] = cloudtrailFixtures
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

// cloudtrailFixtures returns demo CloudTrail trail fixtures.
// Field keys: trail_name, s3_bucket, home_region, multi_region
func cloudtrailFixtures() []resource.Resource {
	return []resource.Resource{
		{
			ID:     "acme-management-trail",
			Name:   "acme-management-trail",
			Status: "",
			Fields: map[string]string{
				"trail_name":     "acme-management-trail",
				"trail_arn":      "arn:aws:cloudtrail:us-east-1:123456789012:trail/acme-management-trail",
				"s3_bucket":      "cloudtrail-audit-logs",
				"home_region":    "us-east-1",
				"multi_region":   "true",
				"org_trail":      "false",
				"log_validation": "true",
			},
			RawStruct: cloudtrailtypes.Trail{
				Name:                       aws.String("acme-management-trail"),
				TrailARN:                   aws.String("arn:aws:cloudtrail:us-east-1:123456789012:trail/acme-management-trail"),
				S3BucketName:               aws.String("cloudtrail-audit-logs"),
				HomeRegion:                 aws.String("us-east-1"),
				IsMultiRegionTrail:         aws.Bool(true),
				IsOrganizationTrail:        aws.Bool(false),
				LogFileValidationEnabled:   aws.Bool(true),
				IncludeGlobalServiceEvents: aws.Bool(true),
				HasCustomEventSelectors:    aws.Bool(true),
				HasInsightSelectors:        aws.Bool(false),
				CloudWatchLogsLogGroupArn:  aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/cloudtrail:*"),
			},
		},
		{
			ID:     "data-events-trail",
			Name:   "data-events-trail",
			Status: "",
			Fields: map[string]string{
				"trail_name":     "data-events-trail",
				"trail_arn":      "arn:aws:cloudtrail:us-east-1:123456789012:trail/data-events-trail",
				"s3_bucket":      "cloudtrail-audit-logs",
				"home_region":    "us-east-1",
				"multi_region":   "false",
				"org_trail":      "false",
				"log_validation": "true",
			},
			RawStruct: cloudtrailtypes.Trail{
				Name:                       aws.String("data-events-trail"),
				TrailARN:                   aws.String("arn:aws:cloudtrail:us-east-1:123456789012:trail/data-events-trail"),
				S3BucketName:               aws.String("cloudtrail-audit-logs"),
				S3KeyPrefix:                aws.String("data-events"),
				HomeRegion:                 aws.String("us-east-1"),
				IsMultiRegionTrail:         aws.Bool(false),
				IsOrganizationTrail:        aws.Bool(false),
				LogFileValidationEnabled:   aws.Bool(true),
				IncludeGlobalServiceEvents: aws.Bool(false),
				HasCustomEventSelectors:    aws.Bool(true),
				HasInsightSelectors:        aws.Bool(false),
			},
		},
		{
			ID:     "security-audit-trail",
			Name:   "security-audit-trail",
			Status: "",
			Fields: map[string]string{
				"trail_name":     "security-audit-trail",
				"trail_arn":      "arn:aws:cloudtrail:us-east-1:123456789012:trail/security-audit-trail",
				"s3_bucket":      "cloudtrail-audit-logs",
				"home_region":    "us-east-1",
				"multi_region":   "true",
				"org_trail":      "true",
				"log_validation": "true",
			},
			RawStruct: cloudtrailtypes.Trail{
				Name:                       aws.String("security-audit-trail"),
				TrailARN:                   aws.String("arn:aws:cloudtrail:us-east-1:123456789012:trail/security-audit-trail"),
				S3BucketName:               aws.String("cloudtrail-audit-logs"),
				S3KeyPrefix:                aws.String("security"),
				HomeRegion:                 aws.String("us-east-1"),
				IsMultiRegionTrail:         aws.Bool(true),
				IsOrganizationTrail:        aws.Bool(true),
				LogFileValidationEnabled:   aws.Bool(true),
				IncludeGlobalServiceEvents: aws.Bool(true),
				HasCustomEventSelectors:    aws.Bool(false),
				HasInsightSelectors:        aws.Bool(true),
				CloudWatchLogsLogGroupArn:  aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/cloudtrail:*"),
			},
		},
	}
}
