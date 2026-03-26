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
			Detail: []string{
				"QueueUrl", "Attributes",
			},
		},
		"sns": {
			List: []ListColumn{
				{Title: "Topic Name", Path: "TopicArn", Width: 40},
				{Title: "Topic ARN", Path: "TopicArn", Width: 60},
			},
			Detail: []string{
				"TopicArn",
			},
		},
		"sns-sub": {
			List: []ListColumn{
				{Title: "Topic ARN", Path: "TopicArn", Width: 48},
				{Title: "Protocol", Path: "Protocol", Width: 10},
				{Title: "Endpoint", Path: "Endpoint", Width: 48},
				{Title: "Subscription ARN", Path: "SubscriptionArn", Width: 60},
			},
			Detail: []string{
				"SubscriptionArn", "TopicArn", "Protocol",
				"Endpoint", "Owner",
			},
		},
		"eb-rule": {
			List: []ListColumn{
				{Title: "Rule Name", Path: "Name", Width: 28},
				{Title: "State", Path: "State", Width: 10},
				{Title: "Event Bus", Path: "EventBusName", Width: 18},
				{Title: "Schedule", Path: "ScheduleExpression", Width: 24},
				{Title: "Description", Path: "Description", Width: 30},
			},
			Detail: []string{
				"Name", "Arn", "State", "Description",
				"EventBusName", "ScheduleExpression", "EventPattern",
				"ManagedBy", "RoleArn",
			},
		},
		"kinesis": {
			List: []ListColumn{
				{Title: "Stream Name", Path: "StreamName", Width: 36},
				{Title: "Status", Path: "StreamStatus", Width: 12},
				{Title: "Mode", Path: "StreamModeDetails.StreamMode", Width: 14},
				{Title: "Created", Path: "StreamCreationTimestamp", Width: 22},
			},
			Detail: []string{
				"StreamName", "StreamARN", "StreamStatus",
				"StreamModeDetails", "StreamCreationTimestamp",
			},
		},
		"msk": {
			List: []ListColumn{
				{Title: "Cluster Name", Path: "ClusterName", Width: 28},
				{Title: "Type", Path: "ClusterType", Width: 14},
				{Title: "State", Path: "State", Width: 14},
				{Title: "Version", Path: "CurrentVersion", Width: 14},
			},
			Detail: []string{
				"ClusterName", "ClusterArn", "ClusterType", "State",
				"CurrentVersion", "CreationTime", "Provisioned", "Serverless",
				"Tags",
			},
		},
		"sfn": {
			List: []ListColumn{
				{Title: "Name", Path: "Name", Width: 36},
				{Title: "Type", Path: "Type", Width: 10},
				{Title: "ARN", Path: "StateMachineArn", Width: 60},
				{Title: "Created", Path: "CreationDate", Width: 22},
			},
			Detail: []string{
				"Name", "StateMachineArn", "Type", "CreationDate",
			},
		},
		"sns_subscriptions": {
			List: []ListColumn{
				{Title: "Protocol", Key: "protocol", Width: 10},
				{Title: "Endpoint", Key: "endpoint", Width: 48},
				{Title: "Status", Key: "confirmation_status", Width: 18},
				{Title: "Owner", Key: "owner", Width: 14},
			},
			Detail: []string{
				"SubscriptionArn", "TopicArn", "Protocol",
				"Endpoint", "Owner",
			},
		},
		"sfn_execution_history": {
			List: []ListColumn{
				{Title: "Timestamp", Path: "Timestamp", Width: 22},
				{Title: "Event Type", Key: "event_type_short", Width: 24},
				{Title: "State", Key: "state_name", Width: 24},
				{Title: "Detail", Key: "event_detail", Width: 40},
			},
			Detail: []string{
				"Timestamp", "Type", "Id", "PreviousEventId",
				"ActivityFailedEventDetails", "ActivityScheduleFailedEventDetails",
				"ActivityScheduledEventDetails", "ActivityStartedEventDetails",
				"ActivitySucceededEventDetails", "ActivityTimedOutEventDetails",
				"ExecutionAbortedEventDetails", "ExecutionFailedEventDetails",
				"ExecutionStartedEventDetails", "ExecutionSucceededEventDetails",
				"ExecutionTimedOutEventDetails",
				"LambdaFunctionFailedEventDetails", "LambdaFunctionScheduledEventDetails",
				"LambdaFunctionStartFailedEventDetails", "LambdaFunctionSucceededEventDetails",
				"LambdaFunctionTimedOutEventDetails",
				"TaskFailedEventDetails", "TaskScheduledEventDetails",
				"TaskStartedEventDetails", "TaskStartFailedEventDetails",
				"TaskSubmitFailedEventDetails", "TaskSubmittedEventDetails",
				"TaskSucceededEventDetails", "TaskTimedOutEventDetails",
				"MapRunFailedEventDetails", "MapRunStartedEventDetails",
				"StateEnteredEventDetails", "StateExitedEventDetails",
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
			Detail: []string{
				"ExecutionArn", "Name", "Status",
				"StartDate", "StopDate",
				"StateMachineArn", "StateMachineAliasArn", "StateMachineVersionArn",
				"MapRunArn", "ItemCount",
				"RedriveCount", "RedriveDate",
			},
		},
	}
}
