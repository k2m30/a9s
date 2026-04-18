package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("alarm", []string{"alarm_name", "state", "metric_name", "namespace", "threshold", "actions_count"})

	resource.RegisterPaginated("alarm", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCloudWatchAlarmsPage(ctx, c.CloudWatch, continuationToken)
	})

	resource.RegisterRelated("alarm", []resource.RelatedDef{
		{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkAlarmSNS, NeedsTargetCache: false},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkAlarmASG, NeedsTargetCache: true},
		{TargetType: "apigw", DisplayName: "API Gateways", Checker: checkAlarmAPIGW},
		{TargetType: "cb", DisplayName: "CodeBuild Projects", Checker: checkAlarmCB},
		{TargetType: "dbi", DisplayName: "RDS Instances", Checker: checkAlarmDBI},
		{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkAlarmEC2},
		{TargetType: "ecs", DisplayName: "ECS Clusters", Checker: checkAlarmECS},
		{TargetType: "eks", DisplayName: "EKS Clusters", Checker: checkAlarmEKS},
		{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkAlarmKMS},
		{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkAlarmLambda},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkAlarmLogs},
		{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkAlarmS3},
		{TargetType: "sfn", DisplayName: "Step Functions", Checker: checkAlarmSFN},
		{TargetType: "waf", DisplayName: "WAF Web ACLs", Checker: checkAlarmWAF},
		{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkAlarmCTEvents, NeedsTargetCache: true},
	})

	// cwtypes.MetricAlarm: Dimensions[].Value may reference EC2/RDS/ELB IDs but Dimensions
	// is heterogeneous — a single FieldPath cannot map to one target type. AlarmActions/OKActions
	// contain SNS ARNs, already handled by checkAlarmSNS. No single-type NavigableField applicable.
}

// FetchCloudWatchAlarms calls the CloudWatch DescribeAlarms API and returns all
// pages of alarms. Used by existing tests and the legacy fetcher.
func FetchCloudWatchAlarms(ctx context.Context, api CloudWatchDescribeAlarmsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchCloudWatchAlarmsPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// FetchCloudWatchAlarmsPage calls the CloudWatch DescribeAlarms API and returns
// a single page of alarms. Pass an empty continuationToken for the first page.
func FetchCloudWatchAlarmsPage(ctx context.Context, api CloudWatchDescribeAlarmsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &cloudwatch.DescribeAlarmsInput{
		MaxRecords: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeAlarms(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching CloudWatch alarms: %w", err)
	}

	var resources []resource.Resource
	for _, alarm := range output.MetricAlarms {
		alarmName := ""
		if alarm.AlarmName != nil {
			alarmName = *alarm.AlarmName
		}

		stateValue := string(alarm.StateValue)

		metricName := ""
		if alarm.MetricName != nil {
			metricName = *alarm.MetricName
		}

		namespace := ""
		if alarm.Namespace != nil {
			namespace = *alarm.Namespace
		}

		threshold := ""
		if alarm.Threshold != nil {
			threshold = fmt.Sprintf("%.2f", *alarm.Threshold)
		}

		actionsCount := len(alarm.AlarmActions) + len(alarm.OKActions) + len(alarm.InsufficientDataActions)

		r := resource.Resource{
			ID:     alarmName,
			Name:   alarmName,
			Status: stateValue,
			Fields: map[string]string{
				"alarm_name":    alarmName,
				"state":         stateValue,
				"metric_name":   metricName,
				"namespace":     namespace,
				"threshold":     threshold,
				"actions_count": strconv.Itoa(actionsCount),
			},
			RawStruct: alarm,
		}

		resources = append(resources, r)
	}

	// Build pagination metadata
	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}
