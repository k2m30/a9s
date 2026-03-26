package resource

// S3ObjectColumns returns the column definitions used when browsing inside
// an S3 bucket (objects/prefixes), as opposed to the bucket list columns.
func S3ObjectColumns() []Column {
	return []Column{
		{Key: "key", Title: "Key", Width: 50, Sortable: true},
		{Key: "size", Title: "Size", Width: 12, Sortable: true},
		{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
		{Key: "storage_class", Title: "Storage Class", Width: 16, Sortable: true},
	}
}

// R53RecordColumns returns the column definitions used when browsing DNS records
// inside a Route53 hosted zone.
func R53RecordColumns() []Column {
	return []Column{
		{Key: "name", Title: "Name", Width: 40, Sortable: true},
		{Key: "type", Title: "Type", Width: 8, Sortable: true},
		{Key: "ttl", Title: "TTL", Width: 8, Sortable: true},
		{Key: "values", Title: "Values", Width: 50, Sortable: true},
	}
}

// LogStreamColumns returns the column definitions for log streams within a
// CloudWatch Log Group.
func LogStreamColumns() []Column {
	return []Column{
		{Key: "stream_name", Title: "Stream Name", Width: 48, Sortable: true},
		{Key: "last_event", Title: "Last Event", Width: 22, Sortable: true},
		{Key: "first_event", Title: "First Event", Width: 22, Sortable: true},
	}
}

// LogEventColumns returns the column definitions for log events within a
// CloudWatch Log Stream.
func LogEventColumns() []Column {
	return []Column{
		{Key: "timestamp", Title: "Timestamp", Width: 22, Sortable: true},
		{Key: "message", Title: "Message", Width: 120, Sortable: true},
	}
}

// TargetHealthColumns returns the column definitions for target health entries
// within an ELBv2 Target Group.
func TargetHealthColumns() []Column {
	return []Column{
		{Key: "target_id", Title: "Target ID", Width: 24, Sortable: true},
		{Key: "port", Title: "Port", Width: 8, Sortable: true},
		{Key: "az", Title: "AZ", Width: 14, Sortable: true},
		{Key: "health", Title: "Health", Width: 14, Sortable: true},
		{Key: "reason", Title: "Reason", Width: 28, Sortable: true},
		{Key: "description", Title: "Description", Width: 36, Sortable: true},
	}
}

// LambdaInvocationColumns returns the column definitions for Lambda invocations
// within a Lambda function's log group.
func LambdaInvocationColumns() []Column {
	return []Column{
		{Key: "timestamp", Title: "Timestamp", Width: 22, Sortable: true},
		{Key: "request_id", Title: "Request ID", Width: 38, Sortable: true},
		{Key: "status", Title: "Status", Width: 10, Sortable: true},
		{Key: "duration_ms", Title: "Duration", Width: 14, Sortable: true},
		{Key: "memory_used", Title: "Memory", Width: 16, Sortable: true},
		{Key: "cold_start", Title: "Cold Start", Width: 12, Sortable: true},
	}
}

// LambdaInvocationLogColumns returns the column definitions for individual log
// lines within a Lambda invocation.
func LambdaInvocationLogColumns() []Column {
	return []Column{
		{Key: "timestamp", Title: "Timestamp", Width: 22, Sortable: true},
		{Key: "message", Title: "Message", Width: 120, Sortable: true},
	}
}

// EcsSvcEventColumns returns the column definitions for ECS service events.
func EcsSvcEventColumns() []Column {
	return []Column{
		{Key: "timestamp", Title: "Timestamp", Width: 22, Sortable: true},
		{Key: "message", Title: "Message", Width: 120, Sortable: true},
	}
}

// EcsSvcTaskColumns returns the column definitions for ECS service tasks.
func EcsSvcTaskColumns() []Column {
	return []Column{
		{Key: "task_id_short", Title: "Task ID", Width: 14, Sortable: true},
		{Key: "status", Title: "Status", Width: 12, Sortable: true},
		{Key: "health", Title: "Health", Width: 10, Sortable: true},
		{Key: "task_def_short", Title: "Task Definition", Width: 28, Sortable: true},
		{Key: "started_at", Title: "Started At", Width: 22, Sortable: true},
		{Key: "stopped_reason", Title: "Stopped Reason", Width: 40, Sortable: true},
	}
}

// EcsSvcLogColumns returns the column definitions for ECS service container logs.
func EcsSvcLogColumns() []Column {
	return []Column{
		{Key: "timestamp", Title: "Timestamp", Width: 22, Sortable: true},
		{Key: "stream_short", Title: "Stream", Width: 20, Sortable: true},
		{Key: "message", Title: "Message", Width: 120, Sortable: true},
	}
}

// CfnEventColumns returns the column definitions for CloudFormation stack events.
func CfnEventColumns() []Column {
	return []Column{
		{Key: "timestamp", Title: "Timestamp", Width: 22, Sortable: true},
		{Key: "logical_resource_id", Title: "Logical ID", Width: 28, Sortable: true},
		{Key: "resource_type", Title: "Type", Width: 28, Sortable: true},
		{Key: "resource_status", Title: "Status", Width: 24, Sortable: true},
		{Key: "resource_status_reason", Title: "Reason", Width: 40, Sortable: false},
	}
}

// CfnResourceColumns returns the column definitions for CloudFormation stack resources.
func CfnResourceColumns() []Column {
	return []Column{
		{Key: "logical_resource_id", Title: "Logical ID", Width: 28, Sortable: true},
		{Key: "physical_resource_id", Title: "Physical ID", Width: 28, Sortable: true},
		{Key: "resource_type", Title: "Type", Width: 28, Sortable: true},
		{Key: "resource_status", Title: "Status", Width: 24, Sortable: true},
		{Key: "drift_status", Title: "Drift", Width: 12, Sortable: true},
		{Key: "last_updated", Title: "Updated", Width: 22, Sortable: true},
	}
}

// AsgActivityColumns returns the column definitions for Auto Scaling Group
// scaling activities.
func AsgActivityColumns() []Column {
	return []Column{
		{Key: "start_time", Title: "Start Time", Width: 22, Sortable: true},
		{Key: "status_code", Title: "Status", Width: 14, Sortable: true},
		{Key: "description", Title: "Description", Width: 50, Sortable: false},
		{Key: "cause", Title: "Cause", Width: 40, Sortable: false},
	}
}

// AlarmHistoryColumns returns the column definitions for CloudWatch Alarm
// history items.
func AlarmHistoryColumns() []Column {
	return []Column{
		{Key: "timestamp", Title: "Timestamp", Width: 22, Sortable: true},
		{Key: "history_item_type", Title: "Type", Width: 18, Sortable: true},
		{Key: "history_summary", Title: "Summary", Width: 60, Sortable: false},
	}
}

// SFNExecutionColumns returns the column definitions for SFN execution list items.
func SFNExecutionColumns() []Column {
	return []Column{
		{Key: "name", Title: "Name", Width: 36, Sortable: true},
		{Key: "status", Title: "Status", Width: 12, Sortable: true},
		{Key: "start_date", Title: "Start Date", Width: 22, Sortable: true},
		{Key: "stop_date", Title: "Stop Date", Width: 22, Sortable: true},
		{Key: "duration", Title: "Duration", Width: 12, Sortable: true},
	}
}

// SFNExecutionHistoryColumns returns the column definitions for SFN execution
// history events.
func SFNExecutionHistoryColumns() []Column {
	return []Column{
		{Key: "timestamp", Title: "Timestamp", Width: 22, Sortable: true},
		{Key: "event_type_short", Title: "Event Type", Width: 24, Sortable: true},
		{Key: "state_name", Title: "State", Width: 24, Sortable: true},
		{Key: "event_detail", Title: "Detail", Width: 40, Sortable: false},
	}
}

// ELBListenerColumns returns the column definitions for ELB listeners.
func ELBListenerColumns() []Column {
	return []Column{
		{Key: "port", Title: "Port", Width: 8, Sortable: true},
		{Key: "protocol", Title: "Protocol", Width: 10, Sortable: true},
		{Key: "default_action_type", Title: "Action", Width: 16, Sortable: true},
		{Key: "default_action_target", Title: "Target", Width: 32, Sortable: false},
		{Key: "ssl_policy", Title: "SSL Policy", Width: 24, Sortable: false},
		{Key: "certificate_short", Title: "Certificate", Width: 32, Sortable: false},
	}
}

// CBBuildColumns returns the column definitions for CodeBuild builds within
// a CodeBuild project.
func CBBuildColumns() []Column {
	return []Column{
		{Key: "build_number", Title: "Build #", Width: 10, Sortable: true},
		{Key: "build_status", Title: "Status", Width: 14, Sortable: true},
		{Key: "start_time", Title: "Start Time", Width: 22, Sortable: true},
		{Key: "duration", Title: "Duration", Width: 12, Sortable: true},
		{Key: "source_version_short", Title: "Source Version", Width: 14, Sortable: false},
		{Key: "initiator", Title: "Initiator", Width: 24, Sortable: false},
	}
}

// CBBuildLogColumns returns the column definitions for CodeBuild build log
// events within a build.
func CBBuildLogColumns() []Column {
	return []Column{
		{Key: "timestamp", Title: "Timestamp", Width: 22, Sortable: true},
		{Key: "message", Title: "Message", Width: 120, Sortable: false},
	}
}

// PipelineStageColumns returns the column definitions for pipeline stage-action
// pairs within a CodePipeline pipeline's current state.
func PipelineStageColumns() []Column {
	return []Column{
		{Key: "stage_name", Title: "Stage", Width: 20, Sortable: true},
		{Key: "stage_status", Title: "Stage Status", Width: 14, Sortable: true},
		{Key: "action_name", Title: "Action", Width: 24, Sortable: true},
		{Key: "action_status", Title: "Action Status", Width: 14, Sortable: true},
		{Key: "last_change_time", Title: "Last Changed", Width: 22, Sortable: true},
		{Key: "external_url", Title: "External URL", Width: 40, Sortable: false},
	}
}

// ECRImageColumns returns the column definitions for ECR images within
// a repository.
func ECRImageColumns() []Column {
	return []Column{
		{Key: "image_tags", Title: "Tag(s)", Width: 24, Sortable: true},
		{Key: "digest_short", Title: "Digest", Width: 16, Sortable: true},
		{Key: "pushed_at", Title: "Pushed At", Width: 22, Sortable: true},
		{Key: "image_size", Title: "Size", Width: 12, Sortable: true},
		{Key: "scan_status", Title: "Scan Status", Width: 14, Sortable: true},
		{Key: "finding_counts", Title: "Findings", Width: 20, Sortable: false},
	}
}

// RolePolicyColumns returns the column definitions for IAM role policies
// (both managed and inline).
func RolePolicyColumns() []Column {
	return []Column{
		{Key: "policy_name", Title: "Policy Name", Width: 40, Sortable: true},
		{Key: "policy_arn", Title: "Policy ARN", Width: 56, Sortable: true},
		{Key: "policy_type", Title: "Type", Width: 10, Sortable: true},
	}
}

// IAMGroupMemberColumns returns the column definitions for IAM group members.
func IAMGroupMemberColumns() []Column {
	return []Column{
		{Key: "user_name", Title: "User Name", Width: 28, Sortable: true},
		{Key: "user_id", Title: "User ID", Width: 24, Sortable: true},
		{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
		{Key: "password_last_used", Title: "Password Last Used", Width: 22, Sortable: true},
	}
}

// DbiEventColumns returns the column definitions for RDS DB instance events.
func DbiEventColumns() []Column {
	return []Column{
		{Key: "timestamp", Title: "Timestamp", Width: 22, Sortable: true},
		{Key: "event_categories", Title: "Category", Width: 18, Sortable: true},
		{Key: "message", Title: "Message", Width: 60, Sortable: true},
	}
}

// SnsSubscriptionColumns returns the column definitions for SNS topic
// subscriptions (child of SNS Topics).
func SnsSubscriptionColumns() []Column {
	return []Column{
		{Key: "protocol", Title: "Protocol", Width: 10, Sortable: true},
		{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: true},
		{Key: "confirmation_status", Title: "Status", Width: 18, Sortable: true},
		{Key: "owner", Title: "Owner", Width: 14, Sortable: true},
	}
}

// EbRuleTargetColumns returns the column definitions for EventBridge rule targets.
func EbRuleTargetColumns() []Column {
	return []Column{
		{Key: "target_id", Title: "Target ID", Width: 20, Sortable: true},
		{Key: "target_arn", Title: "Target ARN", Width: 48, Sortable: true},
		{Key: "resource_type_name", Title: "Resource", Width: 28, Sortable: true},
		{Key: "input_summary", Title: "Input", Width: 36, Sortable: false},
	}
}

// GlueRunColumns returns the column definitions for Glue Job Runs.
func GlueRunColumns() []Column {
	return []Column{
		{Key: "run_id_short", Title: "Run ID", Width: 12, Sortable: true},
		{Key: "job_run_state", Title: "State", Width: 12, Sortable: true},
		{Key: "started_on", Title: "Started", Width: 22, Sortable: true},
		{Key: "execution_time_human", Title: "Execution Time", Width: 14, Sortable: true},
		{Key: "error_message", Title: "Error Message", Width: 44, Sortable: false},
		{Key: "dpu_hours", Title: "DPU Hours", Width: 10, Sortable: true},
	}
}

// ELBListenerRuleColumns returns the column definitions for ELB listener rules.
func ELBListenerRuleColumns() []Column {
	return []Column{
		{Key: "priority", Title: "Priority", Width: 10, Sortable: true},
		{Key: "conditions_summary", Title: "Conditions", Width: 36, Sortable: true},
		{Key: "action_type", Title: "Action", Width: 16, Sortable: true},
		{Key: "action_target", Title: "Target", Width: 32, Sortable: false},
	}
}
