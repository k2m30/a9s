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
