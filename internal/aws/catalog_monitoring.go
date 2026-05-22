package aws

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/semantics/ctevent"
)

func colorAlarm(r domain.Resource) domain.Color {
	switch r.Fields["state"] {
	case "ALARM":
		return domain.ColorBroken
	case "INSUFFICIENT_DATA":
		return domain.ColorWarning
	case "OK":
		actionsCount, err := strconv.Atoi(r.Fields["actions_count"])
		if err != nil || actionsCount == 0 {
			return domain.ColorWarning
		}
		return domain.ColorHealthy
	}
	return domain.ColorHealthy
}

func colorLogs(r domain.Resource) domain.Color {
	if r.Fields["retention_days"] == "" {
		return domain.ColorWarning
	}
	if r.Fields["stored_bytes"] == "0 B" {
		ct := r.Fields["creation_time"]
		t, err := time.Parse("2006-01-02 15:04", ct)
		if err == nil && time.Since(t) > 90*24*time.Hour {
			return domain.ColorWarning
		}
	}
	return domain.ColorHealthy
}

func colorTrail(r domain.Resource) domain.Color {
	if r.Fields["is_logging"] == "false" {
		return domain.ColorBroken
	}
	if r.Fields["latest_delivery_error"] != "" && r.Fields["latest_delivery_error"] != "-" {
		return domain.ColorBroken
	}
	switch r.Fields["status"] {
	case "failed", "FAILED", "error", "ERROR":
		return domain.ColorBroken
	}
	if r.Fields["log_file_validation_enabled"] == "false" {
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

func colorCTEvents(r domain.Resource) domain.Color {
	switch r.Fields["status"] {
	case "ct-danger":
		return domain.ColorBroken
	case "ct-attention":
		return domain.ColorWarning
	}
	return domain.ColorDim
}

var monitoringTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "CloudWatch Alarms",
		ShortName:     "alarm",
		ListTitle:     "alarms",
		Aliases:       []string{"alarm", "alarms", "cloudwatch", "cw_alarms"},
		Category:      "MONITORING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "alarm_name", Title: "Alarm Name", Width: 36, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "metric_name", Title: "Metric", Width: 24, Sortable: true},
			{Key: "namespace", Title: "Namespace", Width: 24, Sortable: true},
			{Key: "threshold", Title: "Threshold", Width: 12, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "alarm_history",
			Key:            "enter",
			ContextKeys:    map[string]string{"alarm_name": "alarm_name"},
			DisplayNameKey: "alarm_name",
		}},
		Color: colorAlarm,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchCloudWatchAlarmsPage(ctx, c.CloudWatch, continuationToken)
		},
		FieldKeys: []string{"alarm_name", "state", "metric_name", "namespace", "threshold", "actions_count"},
		Related: []domain.RelatedDef{
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
		},
	},
	{
		Name:          "CloudWatch Log Groups",
		ShortName:     "logs",
		Aliases:       []string{"logs", "loggroups", "log-groups", "cwlogs", "log_groups"},
		Category:      "MONITORING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "log_group_name", Title: "Log Group Name", Width: 48, Sortable: true},
			{Key: "stored_bytes", Title: "Size", Width: 14, Sortable: true},
			{Key: "retention_days", Title: "Retention", Width: 10, Sortable: true},
			{Key: "creation_time", Title: "Created", Width: 16, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "log_streams",
			Key:            "enter",
			ContextKeys:    map[string]string{"log_group_name": "Name"},
			DisplayNameKey: "log_group_name",
		}},
		Color: colorLogs,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchCloudWatchLogGroupsPage(ctx, c.CloudWatchLogs, continuationToken)
		},
		Wave2:                  IssueEnricher{Fn: EnrichLogsMetricFilters, Priority: 100},
		FieldKeys:              []string{"log_group_name", "stored_bytes", "retention_days", "creation_time", "kms_key_id"},
		IssueEnricherFieldKeys: []string{"last_event_at"},
		Related: []domain.RelatedDef{
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkLogsLambda, NeedsTargetCache: true},
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkLogsAlarms, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkLogsKMS},
			{TargetType: "apigw", DisplayName: "API Gateway", Checker: checkLogsAPIGW, NeedsTargetCache: true},
			{TargetType: "ecs-task", DisplayName: "ECS Tasks", Checker: checkLogsECSTask, NeedsTargetCache: true},
			{TargetType: "kinesis", DisplayName: "Kinesis Streams", Checker: checkLogsKinesis},
			{TargetType: "s3", DisplayName: "S3 (exports)", Checker: checkLogsS3},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("logs")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "KmsKeyId", TargetType: "kms"},
		},
	},
	{
		Name:          "CloudTrail Trails",
		ShortName:     "trail",
		Aliases:       []string{"trail", "cloudtrail", "trails"},
		Category:      "MONITORING",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "trail_name", Title: "Trail Name", Width: 28, Sortable: true},
			{Key: "s3_bucket", Title: "S3 Bucket", Width: 28, Sortable: true},
			{Key: "home_region", Title: "Home Region", Width: 16, Sortable: true},
			{Key: "multi_region", Title: "Multi-Region", Width: 14, Sortable: true},
		},
		Color: colorTrail,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			resources, err := FetchCloudTrailTrails(ctx, c.CloudTrail)
			if err != nil {
				return resource.FetchResult{}, err
			}
			return resource.FetchResult{
				Resources:  resources,
				Pagination: &resource.PaginationMeta{IsTruncated: false, TotalHint: len(resources), PageSize: len(resources)},
			}, nil
		},
		// In-fetcher Wave 2: the trail fetcher already issues GetTrailStatus
		// per-trail and populates is_logging / latest_delivery_error /
		// log_file_validation_enabled at fetch time. NoOpIssueEnricher makes
		// the Wave 2 contract explicit for TestAttentionSignalsDoc.
		Wave2:     IssueEnricher{Fn: NoOpIssueEnricher, Priority: 100},
		FieldKeys: []string{"trail_name", "s3_bucket", "home_region", "multi_region", "is_logging", "latest_delivery_error", "log_file_validation_enabled"},
		Related: []domain.RelatedDef{
			{TargetType: "s3", DisplayName: "S3 Bucket", Checker: checkTrailS3, NeedsTargetCache: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkTrailLogs, NeedsTargetCache: true},
			{TargetType: "sns", DisplayName: "SNS Topic", Checker: checkTrailSNS, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkTrailKMS, NeedsTargetCache: true},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkTrailRole},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("trail")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "S3BucketName", TargetType: "s3"},
			{FieldPath: "KmsKeyId", TargetType: "kms"},
			{FieldPath: "SnsTopicARN", TargetType: "sns"},
			{FieldPath: "CloudWatchLogsLogGroupArn", TargetType: "logs"},
			{FieldPath: "CloudWatchLogsRoleArn", TargetType: "role"},
		},
	},
	{
		Name:      "CloudTrail Events",
		ShortName: "ct-events",
		Aliases:   []string{"event", "events", "ct-events", "cloudtrail-events"},
		Category:  "MONITORING",
		Columns: []domain.Column{
			{Key: "time", Title: "Time", Width: 22, Sortable: true},
			{Key: "event_name", Title: "Event Name", Width: 28, Sortable: true},
			{Key: "user", Title: "User", Width: 24, Sortable: true},
			{Key: "source", Title: "Source", Width: 28, Sortable: true},
			{Key: "resource_type", Title: "Resource Type", Width: 20, Sortable: true},
			{Key: "resource_name", Title: "Resource Name", Width: 24, Sortable: true},
			{Key: "read_only", Title: "Read Only", Width: 10, Sortable: true},
		},
		ExcludeFromIssueBadge: true,
		Color:                 colorCTEvents,
		Project:               ctevent.Project,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchCloudTrailEventsPage(ctx, c.CloudTrail, continuationToken)
		},
		FilteredFetcher: func(ctx context.Context, clients any, filter map[string]string, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchCloudTrailEventsPageFiltered(ctx, c.CloudTrail, filter, continuationToken)
		},
		FieldKeys: []string{"event_name", "time", "event_time", "event_time_raw", "user", "source", "resource_type", "resource_name", "read_only", "role_name", "status", "_ct.verb", "_ct.actor", "_ct.origin", "_ct.target", "_ct.target_raw", "_ct.outcome"},
		Related: []domain.RelatedDef{
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkCtEventsRole, NeedsTargetCache: true},
			{TargetType: "iam-user", DisplayName: "IAM Users", Checker: checkCtEventsUser, NeedsTargetCache: true},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkCtEventsEC2, NeedsTargetCache: true},
			{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkCtEventsS3, NeedsTargetCache: true},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkCtEventsLambda, NeedsTargetCache: true},
			{TargetType: "dbi", DisplayName: "RDS Instances", Checker: checkCtEventsRDS, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkCtEventsKMS, NeedsTargetCache: true},
			{TargetType: "secrets", DisplayName: "Secrets", Checker: checkCtEventsSecrets, NeedsTargetCache: true},
			{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkCtEventsVPCE, NeedsTargetCache: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkCtEventsSG, NeedsTargetCache: true},
			{TargetType: "ddb", DisplayName: "DynamoDB Tables", Checker: checkCtEventsDDB, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkCtEventsCFN, NeedsTargetCache: true},
			{TargetType: "trail", DisplayName: "CloudTrail Trails", Checker: checkCtEventsTrail, NeedsTargetCache: true},
			{TargetType: "ct-events", DisplayName: "CT events by AccessKeyId", Checker: checkCtEventsPivotByAccessKeyId, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CT events by Username", Checker: checkCtEventsPivotByUsername, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CT events by EventName", Checker: checkCtEventsPivotByEventName, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CT events by SharedEventId", Checker: checkCtEventsPivotBySharedEventId, NeedsTargetCache: false},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "user", TargetType: "iam-user"},
			{FieldPath: "role_name", TargetType: "role"},
		},
	},
}
