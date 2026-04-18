package config

func messagingDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"sqs": {
			List: []ListColumn{
				{Title: "Queue Name", Path: "", Key: "queue_name", Width: 36},
				{Title: "Messages", Path: "", Key: "approx_messages", Width: 10},
				{Title: "In Flight", Path: "", Key: "approx_not_visible", Width: 10},
				{Title: "Delay", Path: "", Key: "delay_seconds", Width: 8},
				{Title: "Queue URL", Path: "", Key: "queue_url", Width: 50},
			},
			Detail: []DetailField{
				{Path: "QueueUrl"}, {Path: "Attributes"},
			},
		},
		"sns": {
			List: []ListColumn{
				{Title: "Topic Name", Path: "TopicArn", Width: 40},
				{Title: "Topic ARN", Path: "TopicArn", Width: 60},
			},
			Detail: []DetailField{
				{Path: "TopicArn"},
			},
		},
		"sns-sub": {
			List: []ListColumn{
				{Title: "Topic ARN", Path: "TopicArn", Width: 48},
				{Title: "Protocol", Path: "Protocol", Width: 10},
				{Title: "Endpoint", Path: "Endpoint", Width: 48},
				{Title: "Confirmed", Path: "SubscriptionArn", Width: 22},
				{Title: "Subscription ARN", Path: "SubscriptionArn", Width: 60},
			},
			Detail: []DetailField{
				{Path: "SubscriptionArn"}, {Path: "TopicArn"}, {Path: "Protocol"},
				{Path: "Endpoint"}, {Path: "Owner"},
			},
		},
		"eb-rule": {
			List: []ListColumn{
				{Title: "Rule Name", Path: "Name", Width: 28},
				{Title: "State", Path: "State", Width: 10},
				{Title: "Targets", Key: "target_count", Width: 8},
				{Title: "Event Bus", Path: "EventBusName", Width: 18},
				{Title: "Schedule", Path: "ScheduleExpression", Width: 24},
				{Title: "Description", Path: "Description", Width: 30},
			},
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Arn"}, {Path: "State"}, {Path: "Description"},
				{Path: "EventBusName"}, {Path: "ScheduleExpression"}, {Path: "EventPattern"},
				{Path: "ManagedBy"}, {Path: "RoleArn"},
			},
		},
		"eb_rule_targets": {
			List: []ListColumn{
				{Title: "Target ID", Path: "Id", Width: 20},
				{Title: "Target ARN", Path: "Arn", Width: 48},
				{Title: "Resource", Key: "resource_type_name", Width: 28},
				{Title: "Input", Key: "input_summary", Width: 36},
			},
			Detail: []DetailField{
				{Path: "Id"}, {Path: "Arn"}, {Path: "RoleArn"}, {Path: "Input"}, {Path: "InputPath"}, {Path: "InputTransformer"},
				{Path: "DeadLetterConfig"}, {Path: "RetryPolicy"}, {Path: "SqsParameters"}, {Path: "EcsParameters"},
				{Path: "KinesisParameters"}, {Path: "BatchParameters"}, {Path: "HttpParameters"},
				{Path: "SageMakerPipelineParameters"}, {Path: "RedshiftDataParameters"}, {Path: "AppSyncParameters"},
			},
		},
		"kinesis": {
			List: []ListColumn{
				{Title: "Stream Name", Path: "StreamName", Width: 36},
				{Title: "Status", Path: "StreamStatus", Width: 12},
				{Title: "Mode", Path: "StreamModeDetails.StreamMode", Width: 14},
				{Title: "Created", Path: "StreamCreationTimestamp", Width: 22},
			},
			Detail: []DetailField{
				{Path: "StreamName"}, {Path: "StreamARN"}, {Path: "StreamStatus"},
				{Path: "StreamModeDetails"}, {Path: "StreamCreationTimestamp"},
			},
		},
		"msk": {
			List: []ListColumn{
				{Title: "Cluster Name", Path: "ClusterName", Width: 28},
				{Title: "Type", Path: "ClusterType", Width: 14},
				{Title: "State", Path: "State", Width: 14},
				{Title: "Version", Path: "CurrentVersion", Width: 14},
			},
			Detail: []DetailField{
				{Path: "ClusterName"}, {Path: "ClusterArn"}, {Path: "ClusterType"}, {Path: "State"},
				{Path: "CurrentVersion"}, {Path: "CreationTime"}, {Path: "Provisioned"}, {Path: "Serverless"},
				{Path: "Tags"},
			},
		},
		"sfn": {
			List: []ListColumn{
				{Title: "Name", Path: "Name", Width: 36},
				{Title: "Type", Path: "Type", Width: 10},
				{Title: "ARN", Path: "StateMachineArn", Width: 60},
				{Title: "Created", Path: "CreationDate", Width: 22},
			},
			Detail: []DetailField{
				{Path: "Name"}, {Path: "StateMachineArn"}, {Path: "Type"}, {Path: "CreationDate"},
			},
		},
		"sns_subscriptions": {
			List: []ListColumn{
				{Title: "Protocol", Key: "protocol", Width: 10},
				{Title: "Endpoint", Key: "endpoint", Width: 48},
				{Title: "Status", Key: "confirmation_status", Width: 18},
				{Title: "Owner", Key: "owner", Width: 14},
			},
			Detail: []DetailField{
				{Path: "SubscriptionArn"}, {Path: "TopicArn"}, {Path: "Protocol"},
				{Path: "Endpoint"}, {Path: "Owner"},
			},
		},
		"sfn_execution_history": {
			List: []ListColumn{
				{Title: "Timestamp", Path: "Timestamp", Width: 22},
				{Title: "Event Type", Key: "event_type_short", Width: 24},
				{Title: "State", Key: "state_name", Width: 24},
				{Title: "Detail", Key: "event_detail", Width: 40},
			},
			Detail: []DetailField{
				{Path: "Timestamp"}, {Path: "Type"}, {Path: "Id"}, {Path: "PreviousEventId"},
				{Path: "ActivityFailedEventDetails"}, {Path: "ActivityScheduleFailedEventDetails"},
				{Path: "ActivityScheduledEventDetails"}, {Path: "ActivityStartedEventDetails"},
				{Path: "ActivitySucceededEventDetails"}, {Path: "ActivityTimedOutEventDetails"},
				{Path: "ExecutionAbortedEventDetails"}, {Path: "ExecutionFailedEventDetails"},
				{Path: "ExecutionStartedEventDetails"}, {Path: "ExecutionSucceededEventDetails"},
				{Path: "ExecutionTimedOutEventDetails"},
				{Path: "LambdaFunctionFailedEventDetails"}, {Path: "LambdaFunctionScheduledEventDetails"},
				{Path: "LambdaFunctionStartFailedEventDetails"}, {Path: "LambdaFunctionSucceededEventDetails"},
				{Path: "LambdaFunctionTimedOutEventDetails"},
				{Path: "TaskFailedEventDetails"}, {Path: "TaskScheduledEventDetails"},
				{Path: "TaskStartedEventDetails"}, {Path: "TaskStartFailedEventDetails"},
				{Path: "TaskSubmitFailedEventDetails"}, {Path: "TaskSubmittedEventDetails"},
				{Path: "TaskSucceededEventDetails"}, {Path: "TaskTimedOutEventDetails"},
				{Path: "MapRunFailedEventDetails"}, {Path: "MapRunStartedEventDetails"},
				{Path: "StateEnteredEventDetails"}, {Path: "StateExitedEventDetails"},
			},
		},
		"sfn_executions": {
			List: []ListColumn{
				{Title: "Name", Path: "Name", Width: 36},
				{Title: "Status", Path: "Status", Width: 12},
				{Title: "Start Date", Path: "StartDate", Width: 22},
				{Title: "Stop Date", Path: "StopDate", Width: 22},
				{Title: "Duration", Key: "duration", Width: 12},
			},
			Detail: []DetailField{
				{Path: "ExecutionArn"}, {Path: "Name"}, {Path: "Status"},
				{Path: "StartDate"}, {Path: "StopDate"},
				{Path: "StateMachineArn"}, {Path: "StateMachineAliasArn"}, {Path: "StateMachineVersionArn"},
				{Path: "MapRunArn"}, {Path: "ItemCount"},
				{Path: "RedriveCount"}, {Path: "RedriveDate"},
			},
		},
	}
}
