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
				{Title: "Event ID", Key: "@id", Width: 22},
				{Title: "Time", Path: "EventTime", Width: 22},
				{Title: "Event Name", Path: "EventName", Width: 28},
				{Title: "User", Path: "Username", Width: 24},
				{Title: "Source", Path: "EventSource", Width: 28},
				{Title: "Read Only", Path: "ReadOnly", Width: 10},
			},
			Detail: []string{
				"EventId", "EventName", "EventTime", "EventSource",
				"Username", "ReadOnly", "AccessKeyId",
				"Resources", "CloudTrailEvent",
			},
		},
	}
}
