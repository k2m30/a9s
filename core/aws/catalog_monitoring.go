// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"net/url"
	"strconv"

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

// colorLogs derives the row colour from the log group's findings, and for a row
// that carries none from the same predicate the fetcher used, over the words it
// wrote into Fields for retention and encryption.
func colorLogs(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return colorFromFindings(logsGroupFindings(
		r.Fields["retention"], r.Fields["stored_bytes"],
		r.Fields["creation_time"], r.Fields["encryption"]))
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
		Name:           "CloudWatch Alarms",
		ShortName:      "alarm",
		HumanizeFields: []string{"state", "StateValue", "ComparisonOperator"},
		ListTitle:      "alarms",
		Aliases:        []string{"alarm", "alarms", "cloudwatch", "cw_alarms"},
		Category:       "MONITORING",
		CloudTrailKey:  "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "cloudwatch/home?region="+region+"#alarmsV2:alarm/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "alarm_name", Title: "Alarm Name", Path: "AlarmName", Width: 36},
			{Key: "state", Title: "Status", Path: "StateValue", Width: 12},
			{Title: "Actions On", Path: "ActionsEnabled", Width: 10},
			{Key: "metric_name", Title: "Metric", Path: "MetricName", Width: 24},
			{Key: "namespace", Title: "Namespace", Path: "Namespace", Width: 24},
			{Key: "threshold", Title: "Threshold", Path: "Threshold", Width: 12},
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
			{Code: CodeAlarmStateAlarm, Phrase: "alarm triggered", Severity: domain.SevBroken, Source: "wave1", Detail: "The metric this alarm watches has crossed its threshold, which is the condition somebody set it up to be told about. Open the metric behind it for the last few hours and act on what it measures, rather than on the alarm itself."},
			{Code: CodeAlarmStateInsufficient, Phrase: "insufficient data", Severity: domain.SevWarn, Source: "wave1", Detail: "The alarm has no data to judge, so it is neither confirming health nor able to fire — and a silent alarm looks exactly like a quiet system. Check that the metric is still being published; a deleted resource or a renamed dimension is the usual cause."},
			{Code: CodeAlarmNoActions, Phrase: "no actions", Severity: domain.SevWarn, Source: "wave1", Detail: "This alarm has no action attached, so on its own it changes a colour and tells nobody. An EventBridge rule or a composite alarm may still be watching its state changes; check whether one is before assuming nothing reacts. If nothing does, attach a notification topic or delete the alarm."},
			{Code: CodeAlarmActionsDisabled, Phrase: "actions disabled", Severity: domain.SevWarn, Source: "wave1", Detail: "The alarm still changes state but runs none of its actions, so nobody is notified when it triggers. Switch actions back on for this alarm."},
		},
	},
	{
		Name:           "CloudWatch Log Groups",
		ShortName:      "logs",
		HumanizeFields: []string{"DataProtectionStatus", "LogGroupClass"},
		Aliases:        []string{"logs", "loggroups", "log-groups", "cwlogs", "log_groups"},
		Category:       "MONITORING",
		CloudTrailKey:  "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "cloudwatch/home?region="+region+"#logsV2:log-groups/log-group/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "log_group_name", Title: "Log Group Name", Path: "LogGroupName", Width: 48},
			{Key: "state", Title: "Status", Width: 12},
			{Key: "stored_bytes", Title: "Size", Width: 14, SortKey: "stored_bytes_raw"},
			{Key: "retention", Title: "Retention", Width: 12},
			{Title: "Metric Filters", Path: "MetricFilterCount", Width: 8},
			{Key: "last_event_at", Title: "Last Event", Width: 22},
			{Key: "creation_time", Title: "Created", Width: 16},
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
		FieldKeys:              []string{"log_group_name", "stored_bytes", "stored_bytes_raw", "retention", "creation_time", "kms_key_id"},
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
			{Code: logsCodeRetentionNeverExpire, Phrase: "retention: never expire", Severity: domain.SevWarn, Source: "wave1", Detail: "Events in this group are kept forever and billed forever, and an unbounded audit log is also a growing pile of whatever the application logged. Set a retention period that matches how far back anyone actually looks."},
			{Code: logsCodeStaleEmpty, Phrase: "empty, created over 90 days ago", Severity: domain.SevWarn, Source: "wave1", Detail: "The group has existed for months and holds no events now. That may mean nothing ever wrote to it, or that everything it held has aged out of its retention period. Check the retention setting and the incoming-byte metric, and find the producer that should be writing, before deleting the group."},
			{Code: logsCodeMissingMetricFilters, Phrase: "audit log group missing metric filters", Severity: domain.SevWarn, Source: "wave2", Detail: "This group carries audit or security logs and has no metric filter over it, so nothing here turns an event into an alarm. A subscription filter may be shipping the events somewhere that does, so check for one before concluding nothing watches them. If nothing does, add metric filters and alarms for the events worth waking up for."},
			{Code: CodeLogsNoKMS, Phrase: "not encrypted with KMS", Severity: domain.SevWarn, Source: "wave1", Detail: "Log events are encrypted with the CloudWatch Logs service key, so anyone with read access to the log group can read them and you cannot revoke that access with a key policy. Associate a KMS key with this log group."},
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
			{Key: "trail_name", Title: "Trail Name", Path: "Name", Width: 28},
			{Key: "state", Title: "Status", Width: 12},
			{Key: "s3_bucket", Title: "S3 Bucket", Path: "S3BucketName", Width: 28},
			{Key: "home_region", Title: "Home Region", Path: "HomeRegion", Width: 16},
			{Key: "multi_region", Title: "Multi-Region", Path: "IsMultiRegionTrail", Width: 14},
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
			{Code: CodeTrailLogFileValidationDisabled, Phrase: "log file validation disabled", Severity: domain.SevWarn, Source: "wave1", Detail: "The trail's files carry no integrity digest, so there is no way to prove afterwards that one was not altered or removed. Turn on log file validation — during an incident, unverifiable logs are close to no logs."},
			{Code: CodeTrailNotLogging, Phrase: "not logging", Severity: domain.SevBroken, Source: "wave2", Detail: "This trail is switched off, so it is delivering none of the events it selects. Event history still holds the last ninety days of management events, and another trail or event data store may cover what this one did, so check what else is recording before treating the period as blind. Start it again if this trail's destination is the one you rely on."},
			{Code: CodeTrailDeliveryError, Phrase: "delivery error: <error>", Severity: domain.SevBroken, Source: "wave2", Detail: "CloudTrail cannot write this trail's files to their destination and is retrying, which it will keep doing for up to thirty days before the events are lost. The error names the cause, usually the bucket's policy, a bucket that is gone, or a KMS key CloudTrail may no longer use. Fix it well inside that window."},
			{Code: CodeTrailDeliveryStale, Phrase: "delivery stale since <time>", Severity: domain.SevBroken, Source: "wave2", Detail: "No log file has arrived for far longer than this trail's normal interval, so delivery is behind or stopped. Check the destination bucket's policy and the trail's KMS key. Once delivery resumes, compare the event times either side of the gap before recording it as a hole in the audit trail, because CloudTrail retries for up to thirty days."},
			{Code: CodeTrailNoCloudWatchLogs, Phrase: "not delivering to CloudWatch Logs", Severity: domain.SevWarn, Source: "wave1", Detail: "This trail delivers to S3 but not to CloudWatch Logs, so its events are not where a metric filter and an alarm can see them. EventBridge and whatever reads the bucket may still provide detection; check those first. Attach a log group if you want alerting driven by metric filters over these events."},
			{Code: CodeTrailNoKMS, Phrase: "log files not KMS-encrypted", Severity: domain.SevWarn, Source: "wave1", Detail: "Delivered log files use S3-managed encryption, so anyone who can read the bucket can read the audit trail. Set a KMS key on the trail so log files are encrypted with a key you control."},
			{Code: CodeTrailLogBucketPublic, Phrase: "log bucket is publicly accessible", Severity: domain.SevBroken, Source: "wave2", Detail: "The bucket holding this trail's log files is publicly accessible, so the account's audit history can be read by anyone. Remove the public grant from that bucket's policy and access control list."},
			{Code: CodeTrailLogBucketNoAccessLogging, Phrase: "log bucket has no access logging", Severity: domain.SevWarn, Source: "wave2", Detail: "The bucket holding this trail's log files records no access logging, so reads of the audit history leave no trace. Enable server access logging on that bucket."},
		},
	},
	{
		Name:         "CloudTrail Events",
		ShortName:    "ct-events",
		LifecycleKey: "status",
		Aliases:      []string{"event", "events", "ct-events", "cloudtrail-events"},
		Category:     "MONITORING",
		Columns: []domain.Column{
			{Key: "_ct.verb", Title: "V", Width: 1},
			{Key: "time", Title: "TIME", Width: 15, SortKey: "event_time"},
			{Key: "status", Title: "Status", Width: 12},
			{Key: "_ct.actor", Title: "ACTOR", Width: 36},
			{Key: "_ct.origin", Title: "ORIGIN", Width: 7},
			{Title: "EVENT", Path: "EventName", Width: 34},
			{Key: "_ct.target", Title: "TARGET", Width: 36},
			{Key: "_ct.outcome", Title: "OUTCOME", Width: 14},
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
		FieldKeys: []string{"event_name", "time", "event_time", "user", "source", "resource_type", "resource_name", "read_only", "role_name", "status", "_ct.verb", "_ct.actor", "_ct.origin", "_ct.target", "_ct.target_raw", "_ct.outcome"},
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
			{Code: CodeCTEventDanger, Phrase: "destructive call", Severity: domain.SevBroken, Source: "wave1", Detail: "CloudTrail recorded a call that deletes or tears something down. Verify it was expected and, if not, find out who made it."},
			{Code: CodeCTEventFailedCall, Phrase: "failed: <error>", Severity: domain.SevBroken, Source: "wave1", Detail: "CloudTrail recorded a call that AWS rejected; the error code is in the phrase. A denied call is either a permission gap or someone probing for one."},
			{Code: CodeCTEventAttention, Phrase: "root account activity", Severity: domain.SevWarn, Source: "wave1", Detail: "CloudTrail recorded a call made by the account root user. Root should not be doing day-to-day work; move the task onto a named principal."},
			{Code: CodeCTEventWrite, Phrase: "modifying call", Severity: domain.SevWarn, Source: "wave1", Detail: "CloudTrail recorded a call that changed configuration. Verify the change was expected and that whoever made it meant to."},
			{Code: CodeCTEventCrossAccount, Phrase: "cross-account access", Severity: domain.SevWarn, Source: "wave1", Detail: "The caller belongs to a different account than the one that recorded the event, so this is access across an account boundary. Verify the trust it came through is one you meant to grant."},
			{Code: CodeCTEventSensitiveRead, Phrase: "reads sensitive data (<event>)", Severity: domain.SevWarn, Source: "wave1", Detail: "CloudTrail recorded a read of secret or parameter material. Verify the caller was expected: a read leaves the value in the caller's hands with nothing to revoke afterwards."},
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
			{Code: CodeCWLogError, Phrase: "error", Severity: domain.SevBroken, Source: "wave1", Detail: "This line reports an error from the workload itself, so something it was asked to do did not happen. Read the surrounding lines for the request or job it belongs to: a single error with no repeats usually means a transient dependency, a steady stream means a real fault."},
			{Code: CodeCWLogWarn, Phrase: "warning", Severity: domain.SevWarn, Source: "wave1", Detail: "The workload logged something it could work around but wanted recorded — a retry, a deprecated setting, or a near-limit condition. Worth reading if it repeats; a single one is usually noise."},
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
			{Code: CodeAlarmHistoryStateAlarm, Phrase: "alarm", Severity: domain.SevBroken, Source: "wave1", Detail: "At this point the alarm's metric was over its threshold, so the condition it watches was true then. Line the timestamp up with your deployments and incident timeline to see what it coincided with."},
			{Code: CodeAlarmHistoryStateInsufficientData, Phrase: "insufficient data", Severity: domain.SevWarn, Source: "wave1", Detail: "The alarm had no data to evaluate at this point, so it could not have fired even if the underlying condition was true. A run of these marks a window in which this alarm was protecting nothing."},
		},
	},
}
