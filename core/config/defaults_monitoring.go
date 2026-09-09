// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

func monitoringDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"alarm": {
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
			Detail: []DetailField{
				{Path: "LogGroupName"}, {Path: "LogGroupArn"}, {Path: "LogGroupClass"},
				{Path: "StoredBytes"}, {Path: "RetentionInDays"}, {Path: "MetricFilterCount"},
				{Path: "DeletionProtectionEnabled"}, {Path: "CreationTime"},
				{Path: "KmsKeyId"}, {Path: "DataProtectionStatus"},
			},
		},
		"trail": {
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
			Detail: []DetailField{
				{Path: "Timestamp"}, {Path: "HistoryItemType"}, {Path: "HistorySummary"},
				{Path: "HistoryData"}, {Path: "AlarmName"}, {Path: "AlarmType"},
			},
		},
		"log_streams": {
			Detail: []DetailField{
				{Path: "LogStreamName"}, {Path: "Arn"}, {Path: "CreationTime"},
				{Path: "FirstEventTimestamp"}, {Path: "LastEventTimestamp"},
				{Path: "LastIngestionTime"}, {Path: "UploadSequenceToken"},
			},
		},
		"log_events": {
			Detail: []DetailField{
				{Path: "Timestamp"}, {Path: "Message"}, {Path: "IngestionTime"}, {Path: "EventId"},
			},
		},
		"ct-events": {
			Detail: []DetailField{
				{Path: "EventId"}, {Path: "EventName"}, {Path: "EventTime"}, {Path: "EventSource"},
				{Path: "Username"}, {Path: "ReadOnly"}, {Path: "AccessKeyId"},
				{Path: "Resources"}, {Path: "CloudTrailEvent"},
			},
		},
	}
}
