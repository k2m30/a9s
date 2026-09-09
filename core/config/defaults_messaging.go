// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

func messagingDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"sqs": {
			Detail: []DetailField{
				{Path: "QueueUrl"}, {Path: "Attributes"},
			},
		},
		"sns": {
			Detail: []DetailField{
				{Path: "TopicArn"}, {Path: "Attributes"},
			},
		},
		"sns-sub": {
			Detail: []DetailField{
				{Path: "SubscriptionArn"}, {Path: "TopicArn"}, {Path: "Protocol"},
				{Path: "Endpoint"}, {Path: "Owner"},
			},
		},
		"eb-rule": {
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Arn"}, {Path: "State"}, {Path: "Description"},
				{Path: "EventBusName"}, {Path: "ScheduleExpression"}, {Path: "EventPattern"},
				{Path: "ManagedBy"}, {Path: "RoleArn"},
			},
		},
		"eb_rule_targets": {
			Detail: []DetailField{
				{Path: "Id"}, {Path: "Arn"}, {Path: "RoleArn"}, {Path: "Input"}, {Path: "InputPath"}, {Path: "InputTransformer"},
				{Path: "DeadLetterConfig"}, {Path: "RetryPolicy"}, {Path: "SqsParameters"}, {Path: "EcsParameters"},
				{Path: "KinesisParameters"}, {Path: "BatchParameters"}, {Path: "HttpParameters"},
				{Path: "SageMakerPipelineParameters"}, {Path: "RedshiftDataParameters"}, {Path: "AppSyncParameters"},
			},
		},
		"kinesis": {
			Detail: []DetailField{
				{Path: "StreamName"}, {Path: "StreamARN"}, {Path: "StreamStatus"},
				{Path: "StreamModeDetails"}, {Path: "StreamCreationTimestamp"},
			},
		},
		"msk": {
			Detail: []DetailField{
				{Path: "ClusterName"}, {Path: "ClusterArn"}, {Path: "ClusterType"}, {Path: "State"},
				{Path: "CurrentVersion"}, {Path: "CreationTime"}, {Path: "Provisioned"}, {Path: "Serverless"},
				{Path: "Tags"},
			},
		},
		"sfn": {
			Detail: []DetailField{
				{Path: "Name"}, {Path: "StateMachineArn"}, {Path: "Type"}, {Path: "CreationDate"},
				{Path: "Status"}, {Path: "RoleArn"},
			},
		},
		"sns_subscriptions": {
			Detail: []DetailField{
				{Path: "SubscriptionArn"}, {Path: "TopicArn"}, {Path: "Protocol"},
				{Path: "Endpoint"}, {Path: "Owner"},
			},
		},
		"sfn_execution_history": {
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
