package config

func monitoringDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"alarm": {
			List: []ListColumn{
				{Title: "Alarm Name", Path: "AlarmName", Width: 36},
				{Title: "State", Path: "StateValue", Width: 12},
				{Title: "Metric", Path: "MetricName", Width: 24},
				{Title: "Namespace", Path: "Namespace", Width: 24},
				{Title: "Threshold", Path: "Threshold", Width: 12},
			},
			Detail: []string{
				"AlarmName", "AlarmArn", "StateValue", "StateReason",
				"StateUpdatedTimestamp", "StateTransitionedTimestamp",
				"MetricName", "Namespace", "Statistic", "Period",
				"EvaluationPeriods", "DatapointsToAlarm", "Threshold",
				"ComparisonOperator", "TreatMissingData", "Dimensions",
				"AlarmDescription", "AlarmActions", "OKActions",
				"InsufficientDataActions", "ActionsEnabled",
			},
		},
		"logs": {
			List: []ListColumn{
				{Title: "Log Group Name", Path: "LogGroupName", Width: 48},
				{Title: "Size", Path: "", Key: "stored_bytes", Width: 14},
				{Title: "Retention", Path: "RetentionInDays", Width: 10},
				{Title: "Metric Filters", Path: "MetricFilterCount", Width: 8},
				{Title: "Created", Path: "", Key: "creation_time", Width: 16},
			},
			Detail: []string{
				"LogGroupName", "LogGroupArn", "LogGroupClass",
				"StoredBytes", "RetentionInDays", "MetricFilterCount",
				"DeletionProtectionEnabled", "CreationTime",
				"KmsKeyId", "DataProtectionStatus",
			},
		},
		"trail": {
			List: []ListColumn{
				{Title: "Trail Name", Path: "Name", Width: 28},
				{Title: "S3 Bucket", Path: "S3BucketName", Width: 28},
				{Title: "Home Region", Path: "HomeRegion", Width: 16},
				{Title: "Multi-Region", Path: "IsMultiRegionTrail", Width: 14},
			},
			Detail: []string{
				"Name", "TrailARN", "S3BucketName", "HomeRegion",
				"IsMultiRegionTrail", "IsOrganizationTrail",
				"LogFileValidationEnabled", "IncludeGlobalServiceEvents",
				"KmsKeyId", "CloudWatchLogsLogGroupArn",
			},
		},
		// Child views for monitoring resources
		"alarm_history": {
			List: []ListColumn{
				{Title: "Timestamp", Key: "timestamp", Width: 22},
				{Title: "Type", Key: "history_item_type", Width: 18},
				{Title: "Summary", Key: "history_summary", Width: 60},
			},
			Detail: []string{
				"Timestamp", "HistoryItemType", "HistorySummary",
				"HistoryData", "AlarmName", "AlarmType",
			},
		},
		"log_streams": {
			List: []ListColumn{
				{Title: "Stream Name", Path: "LogStreamName", Width: 48},
				{Title: "Last Event", Path: "", Key: "last_event", Width: 22},
				{Title: "First Event", Path: "", Key: "first_event", Width: 22},
			},
			Detail: []string{
				"LogStreamName", "Arn", "CreationTime",
				"FirstEventTimestamp", "LastEventTimestamp",
				"LastIngestionTime", "UploadSequenceToken",
			},
		},
		"log_events": {
			List: []ListColumn{
				{Title: "Timestamp", Path: "", Key: "timestamp", Width: 22},
				{Title: "Message", Path: "", Key: "message", Width: 120},
			},
			Detail: []string{
				"Timestamp", "Message", "IngestionTime", "EventId",
			},
		},
		"ct-events": {
			List: []ListColumn{
				{Title: "V", Key: "_ct.verb", Width: 1},
				{Title: "TIME", Key: "time", Width: 15},
				{Title: "ACTOR", Key: "_ct.actor", Width: 36},
				{Title: "ORIGIN", Key: "_ct.origin", Width: 7},
				{Title: "EVENT", Path: "EventName", Width: 34},
				{Title: "TARGET", Key: "_ct.target", Width: 36},
				{Title: "OUTCOME", Key: "_ct.outcome", Width: 14},
			},
			Detail: []string{
				"EventId", "EventName", "EventTime", "EventSource",
				"Username", "ReadOnly", "AccessKeyId",
				"Resources", "CloudTrailEvent",
			},
		},
	}
}
