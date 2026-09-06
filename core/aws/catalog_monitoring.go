// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/semantics/ctevent"
)

func colorAlarm(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	actionsCount, _ := strconv.Atoi(r.Fields["actions_count"])
	return colorFromFindings(alarmStateFindings(r.Fields["state"], actionsCount))
}

func colorLogs(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
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
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return colorFromFindings(trailWave1Wave2Findings(
		r.Fields["is_logging"], r.Fields["latest_delivery_error"],
		r.Fields["latest_delivery_time"], r.Fields["log_file_validation_enabled"]))
}

func colorCTEvents(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	// Every event carries a finding, including the routine tier that colours
	// the row dim — so the fallback never returns healthy here.
	return colorFromFindings(ctEventFindings(
		r.Fields["status"], r.Fields["cause"], r.Fields["error_code"], r.Name))
}

var monitoringTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "CloudWatch Alarms",
		ShortName:     "alarm",
		ListTitle:     "alarms",
		Aliases:       []string{"alarm", "alarms", "cloudwatch", "cw_alarms"},
		Category:      "MONITORING",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "cloudwatch/home?region="+region+"#alarmsV2:alarm/"+url.PathEscape(r.ID))
		},
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
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchCloudWatchAlarmsPage(ctx, c.CloudWatch, continuationToken)
		}),
		FieldKeys: []string{"alarm_name", "state", "metric_name", "namespace", "threshold", "actions_count"},
		Related: []domain.RelatedDef{
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkAlarmSNS, NeedsTargetCache: false},
			{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkAlarmASG, NeedsTargetCache: true, Truncated: true},
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
		Findings: []catalog.FindingDef{
			{Code: CodeAlarmStateAlarm, Phrase: "alarm triggered", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeAlarmStateInsufficient, Phrase: "insufficient data", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeAlarmNoActions, Phrase: "no actions", Severity: domain.SevWarn, Source: "wave1"},
		},
	},
	{
		Name:          "CloudWatch Log Groups",
		ShortName:     "logs",
		Aliases:       []string{"logs", "loggroups", "log-groups", "cwlogs", "log_groups"},
		Category:      "MONITORING",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "cloudwatch/home?region="+region+"#logsV2:log-groups/log-group/"+url.PathEscape(r.ID))
		},
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
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchCloudWatchLogGroupsPage(ctx, c.CloudWatchLogs, continuationToken)
		}),
		Wave2:                  IssueEnricher{Fn: EnrichLogsMetricFilters, Priority: 100},
		FieldKeys:              []string{"log_group_name", "stored_bytes", "retention_days", "creation_time", "kms_key_id"},
		IssueEnricherFieldKeys: []string{"last_event_at"},
		Related: []domain.RelatedDef{
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkLogsLambda, NeedsTargetCache: true, Truncated: true},
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkLogsAlarms, NeedsTargetCache: true, Truncated: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkLogsKMS},
			{TargetType: "apigw", DisplayName: "API Gateway", Checker: checkLogsAPIGW, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ecs-task", DisplayName: "ECS Tasks", Checker: checkLogsECSTask, NeedsTargetCache: true, Truncated: true},
			{TargetType: "kinesis", DisplayName: "Kinesis Streams", Checker: checkLogsKinesis},
			{TargetType: "s3", DisplayName: "S3 (exports)", Checker: checkLogsS3},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("logs")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "KmsKeyId", TargetType: "kms"},
		},
		Findings: []catalog.FindingDef{
			{Code: logsCodeRetentionNeverExpire, Phrase: "retention: never expire", Severity: domain.SevWarn, Source: "wave1", Detail: "No retention policy set — events kept forever, billed indefinitely."},
			{Code: logsCodeStaleEmpty, Phrase: "empty, created over 90 days ago", Severity: domain.SevWarn, Source: "wave1"},
			{Code: logsCodeMissingMetricFilters, Phrase: "audit log group missing metric filters", Severity: domain.SevWarn, Source: "wave2"},
		},
	},
	{
		Name:          "CloudTrail Trails",
		ShortName:     "trail",
		Aliases:       []string{"trail", "cloudtrail", "trails"},
		Category:      "MONITORING",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			arn := r.Fields["trail_arn"]
			if arn == "" {
				return ""
			}
			return consolelink.Regional(region, "cloudtrailv2/home?region="+region+"#/trails/"+arn)
		},
		Columns: []domain.Column{
			{Key: "trail_name", Title: "Trail Name", Width: 28, Sortable: true},
			{Key: "s3_bucket", Title: "S3 Bucket", Width: 28, Sortable: true},
			{Key: "home_region", Title: "Home Region", Width: 16, Sortable: true},
			{Key: "multi_region", Title: "Multi-Region", Width: 14, Sortable: true},
		},
		Color: colorTrail,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			resources, err := FetchCloudTrailTrails(ctx, c.CloudTrail)
			if err != nil {
				return resource.FetchResult{}, err
			}
			return resource.FetchResult{
				Resources:  resources,
				Pagination: &resource.PaginationMeta{IsTruncated: false, TotalHint: len(resources), PageSize: len(resources)},
			}, nil
		}),
		// In-fetcher Wave 2: the trail fetcher already issues GetTrailStatus
		// per-trail and populates is_logging / latest_delivery_error /
		// log_file_validation_enabled at fetch time. The wave2-source
		// FindingDefs below feed the generated table in
		// docs/attention-signals.md, kept in sync by `make check-catalogen` and
		// tests/unit/docs_attention_signals_sync_test.go.
		//
		// EnrichTrailLogBucket carries the log-bucket signals on top of that
		// in-fetcher work. It is cache-only, so Priority 200 lets the s3
		// enricher (100) populate the bucket findings it reads.
		Wave2:     IssueEnricher{Fn: EnrichTrailLogBucket, Priority: 200},
		FieldKeys: []string{"trail_name", "s3_bucket", "home_region", "multi_region", "is_logging", "latest_delivery_error", "log_file_validation_enabled", "trail_arn"},
		Related: []domain.RelatedDef{
			{TargetType: "s3", DisplayName: "S3 Bucket", Checker: checkTrailS3, NeedsTargetCache: true, Truncated: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkTrailLogs, NeedsTargetCache: true, Truncated: true},
			{TargetType: "sns", DisplayName: "SNS Topic", Checker: checkTrailSNS, NeedsTargetCache: true, Truncated: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkTrailKMS, NeedsTargetCache: true, Truncated: true},
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
		Findings: []catalog.FindingDef{
			{Code: CodeTrailLogFileValidationDisabled, Phrase: "log file validation disabled", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeTrailNotLogging, Phrase: "not logging", Severity: domain.SevBroken, Source: "wave2"},
			{Code: CodeTrailDeliveryError, Phrase: "delivery error: <LatestDeliveryError>", Severity: domain.SevBroken, Source: "wave2"},
			{Code: CodeTrailDeliveryStale, Phrase: "delivery stale since <LatestDeliveryTime>", Severity: domain.SevBroken, Source: "wave2"},
		},
	},
	{
		Name:      "CloudTrail Events",
		ShortName: "ct-events",
		Aliases:   []string{"event", "events", "ct-events", "cloudtrail-events"},
		Category:  "MONITORING",
		Columns: []domain.Column{
			{Key: "time", Title: "Time", Width: 22, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
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
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchCloudTrailEventsPage(ctx, c.CloudTrail, continuationToken)
		}),
		FilteredFetcher: filteredFetcherWithClients(func(ctx context.Context, c *ServiceClients, filter map[string]string, continuationToken string) (resource.FetchResult, error) {
			return FetchCloudTrailEventsPageFiltered(ctx, c.CloudTrail, filter, continuationToken)
		}),
		FieldKeys: []string{"event_name", "time", "event_time", "event_time_raw", "user", "source", "resource_type", "resource_name", "read_only", "role_name", "status", "_ct.verb", "_ct.actor", "_ct.origin", "_ct.target", "_ct.target_raw", "_ct.outcome"},
		Related: []domain.RelatedDef{
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkCtEventsRole, NeedsTargetCache: false},
			{TargetType: "iam-user", DisplayName: "IAM Users", Checker: checkCtEventsUser, NeedsTargetCache: false},
			{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkCtEventsEC2, NeedsTargetCache: false},
			{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkCtEventsS3, NeedsTargetCache: false},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkCtEventsLambda, NeedsTargetCache: false},
			{TargetType: "dbi", DisplayName: "RDS Instances", Checker: checkCtEventsRDS, NeedsTargetCache: false},
			{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkCtEventsKMS, NeedsTargetCache: false},
			{TargetType: "secrets", DisplayName: "Secrets", Checker: checkCtEventsSecrets, NeedsTargetCache: false},
			{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkCtEventsVPCE, NeedsTargetCache: false},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkCtEventsSG, NeedsTargetCache: false},
			{TargetType: "ddb", DisplayName: "DynamoDB Tables", Checker: checkCtEventsDDB, NeedsTargetCache: false},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkCtEventsCFN, NeedsTargetCache: false},
			{TargetType: "trail", DisplayName: "CloudTrail Trails", Checker: checkCtEventsTrail, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CT events by AccessKeyId", Checker: checkCtEventsPivotByAccessKeyId, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CT events by Username", Checker: checkCtEventsPivotByUsername, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CT events by EventName", Checker: checkCtEventsPivotByEventName, NeedsTargetCache: false},
			{TargetType: "ct-events", DisplayName: "CT events by SharedEventId", Checker: checkCtEventsPivotBySharedEventId, NeedsTargetCache: false},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "user", TargetType: "iam-user"},
			{FieldPath: "role_name", TargetType: "role"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeCTEventDanger, Phrase: "destructive call", Severity: domain.SevBroken, Source: "wave1", Detail: "CloudTrail recorded a call that either failed or was destructive; the event name and error code in this row say which. Verify it was expected and, if not, find out who made it."},
			{Code: CodeCTEventAttention, Phrase: "root account activity", Severity: domain.SevWarn, Source: "wave1", Detail: "CloudTrail recorded a call worth a look: a modifying call, root-account activity, cross-account access, or a read of sensitive data. The status names which; verify the caller was expected."},
			{Code: CodeCTEventInfo, Phrase: "routine event", Severity: domain.SevDim, Source: "wave1"},
		},
	},
}

var monitoringChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "Log Streams",
		ShortName: "log_streams",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return cloudWatchLogStreamConsoleURL(region, r.Fields["log_group"], r.Fields["stream_name"])
		},
		Columns:   resource.LogStreamColumns(),
		FieldKeys: []string{"stream_name", "last_event", "first_event", "log_group"},
		Children: []domain.ChildViewDef{{
			ChildType:      "log_events",
			Key:            "enter",
			ContextKeys:    map[string]string{"log_group_name": "@parent.log_group_name", "log_stream_name": "Name"},
			DisplayNameKey: "log_stream_name",
		}},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchLogStreams(ctx, c.CloudWatchLogs, parentCtx["log_group_name"], continuationToken)
		}),
	},
	{
		Name:         "Log Events",
		ShortName:    "log_events",
		TitleOmitsID: true,
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return cloudWatchLogStreamConsoleURL(region, r.Fields["log_group"], r.Fields["log_stream"])
		},
		Columns:   resource.LogEventColumns(),
		Color:     colorAnyFindingOrHealthy,
		FieldKeys: []string{"timestamp", "message", "ingestion_time", "event_id", "log_group", "log_stream"},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchLogEvents(ctx, c.CloudWatchLogs, parentCtx["log_group_name"], parentCtx["log_stream_name"], continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeCWLogError, Phrase: "error", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeCWLogWarn, Phrase: "warning", Severity: domain.SevWarn, Source: "wave1"},
		},
	},
	{
		Name:      "Alarm History",
		ShortName: "alarm_history",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			name := r.Fields["alarm_name"]
			if name == "" {
				return ""
			}
			return consolelink.Regional(region, "cloudwatch/home?region="+region+"#alarmsV2:alarm/"+url.PathEscape(name))
		},
		Columns:   resource.AlarmHistoryColumns(),
		Color:     colorAnyFindingOrHealthy,
		FieldKeys: []string{"timestamp", "history_item_type", "history_summary", "alarm_name"},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchAlarmHistory(ctx, c.CloudWatch, parentCtx, continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeAlarmHistoryStateAlarm, Phrase: "alarm", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeAlarmHistoryStateInsufficientData, Phrase: "insufficient data", Severity: domain.SevWarn, Source: "wave1"},
		},
	},
}
