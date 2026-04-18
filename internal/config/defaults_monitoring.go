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
			Detail: []DetailField{
				{Path: "AlarmName"}, {Path: "AlarmArn"}, {Path: "StateValue"}, {Path: "StateReason"},
				{Path: "StateUpdatedTimestamp"}, {Path: "StateTransitionedTimestamp"},
				{Path: "MetricName"}, {Path: "Namespace"}, {Path: "Statistic"}, {Path: "Period"},
				{Path: "EvaluationPeriods"}, {Path: "DatapointsToAlarm"}, {Path: "Threshold"},
				{Path: "ComparisonOperator"}, {Path: "TreatMissingData"}, {Path: "Dimensions"},
				{Path: "AlarmDescription"}, {Path: "AlarmActions"}, {Path: "OKActions"},
				{Path: "InsufficientDataActions"}, {Path: "ActionsEnabled"},
			},
		},
		"logs": {
			List: []ListColumn{
				{Title: "Log Group Name", Path: "LogGroupName", Width: 48},
				{Title: "Size", Key: "stored_bytes", SortPath: "StoredBytes", Width: 14},
				{Title: "Retention", Path: "RetentionInDays", Width: 10},
				{Title: "Metric Filters", Path: "MetricFilterCount", Width: 8},
				{Title: "Created", Path: "", Key: "creation_time", Width: 16},
			},
			Detail: []DetailField{
				{Path: "LogGroupName"}, {Path: "LogGroupArn"}, {Path: "LogGroupClass"},
				{Path: "StoredBytes"}, {Path: "RetentionInDays"}, {Path: "MetricFilterCount"},
				{Path: "DeletionProtectionEnabled"}, {Path: "CreationTime"},
				{Path: "KmsKeyId"}, {Path: "DataProtectionStatus"},
			},
		},
		"trail": {
			List: []ListColumn{
				{Title: "Trail Name", Path: "Name", Width: 28},
				{Title: "Logging", Key: "is_logging", Width: 10},
				{Title: "Last Error", Key: "latest_delivery_error", Width: 32},
				{Title: "S3 Bucket", Path: "S3BucketName", Width: 28},
				{Title: "Home Region", Path: "HomeRegion", Width: 16},
				{Title: "Multi-Region", Path: "IsMultiRegionTrail", Width: 14},
			},
			Detail: []DetailField{
				{Path: "Name"}, {Path: "TrailARN"}, {Path: "S3BucketName"}, {Path: "HomeRegion"},
				{Path: "IsMultiRegionTrail"}, {Path: "IsOrganizationTrail"},
				{Path: "LogFileValidationEnabled"}, {Path: "IncludeGlobalServiceEvents"},
				{Path: "KmsKeyId"}, {Path: "CloudWatchLogsLogGroupArn"},
				{Key: "is_logging", Label: "Logging"},
				{Key: "latest_delivery_error", Label: "Latest Delivery Error"},
			},
		},
		// Child views for monitoring resources
		"alarm_history": {
			List: []ListColumn{
				{Title: "Timestamp", Key: "timestamp", Width: 22},
				{Title: "Type", Key: "history_item_type", Width: 18},
				{Title: "Summary", Key: "history_summary", Width: 60},
			},
			Detail: []DetailField{
				{Path: "Timestamp"}, {Path: "HistoryItemType"}, {Path: "HistorySummary"},
				{Path: "HistoryData"}, {Path: "AlarmName"}, {Path: "AlarmType"},
			},
		},
		"log_streams": {
			List: []ListColumn{
				{Title: "Stream Name", Path: "LogStreamName", Width: 48},
				{Title: "Last Event", Path: "", Key: "last_event", Width: 22},
				{Title: "First Event", Path: "", Key: "first_event", Width: 22},
			},
			Detail: []DetailField{
				{Path: "LogStreamName"}, {Path: "Arn"}, {Path: "CreationTime"},
				{Path: "FirstEventTimestamp"}, {Path: "LastEventTimestamp"},
				{Path: "LastIngestionTime"}, {Path: "UploadSequenceToken"},
			},
		},
		"log_events": {
			List: []ListColumn{
				{Title: "Timestamp", Path: "", Key: "timestamp", Width: 22},
				{Title: "Message", Path: "", Key: "message", Width: 120},
			},
			Detail: []DetailField{
				{Path: "Timestamp"}, {Path: "Message"}, {Path: "IngestionTime"}, {Path: "EventId"},
			},
		},
		"ct-events": {
			List: []ListColumn{
				{Title: "V", Key: "_ct.verb", Width: 1},
				{Title: "TIME", Key: "time", SortKey: "event_time", Width: 15},
				{Title: "ACTOR", Key: "_ct.actor", Width: 36},
				{Title: "ORIGIN", Key: "_ct.origin", Width: 7},
				{Title: "EVENT", Path: "EventName", Width: 34},
				{Title: "TARGET", Key: "_ct.target", Width: 36},
				{Title: "OUTCOME", Key: "_ct.outcome", Width: 14},
			},
			Detail: []DetailField{
				{Path: "EventId"}, {Path: "EventName"}, {Path: "EventTime"}, {Path: "EventSource"},
				{Path: "Username"}, {Path: "ReadOnly"}, {Path: "AccessKeyId"},
				{Path: "Resources"}, {Path: "CloudTrailEvent"},
			},
		},
	}
}
