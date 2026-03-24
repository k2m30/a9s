package config

func computeDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"ec2": {
			List: []ListColumn{
				{Title: "Name", Path: "", Width: 24},
				{Title: "Instance ID", Path: "InstanceId", Width: 20},
				{Title: "State", Path: "State.Name", Width: 12},
				{Title: "Type", Path: "InstanceType", Width: 14},
				{Title: "Private IP", Path: "PrivateIpAddress", Width: 16},
				{Title: "Public IP", Path: "PublicIpAddress", Width: 16},
				{Title: "Launch Time", Path: "LaunchTime", Width: 22},
			},
			Detail: []string{
				"InstanceId", "State", "InstanceType", "ImageId",
				"KeyName", "Placement",
				"VpcId", "SubnetId", "PrivateIpAddress", "PrivateDnsName",
				"PublicIpAddress", "IamInstanceProfile",
				"SecurityGroups", "EbsOptimized", "MetadataOptions",
				"LaunchTime", "Architecture", "Platform", "Tags",
			},
		},
		"ecs": {
			List: []ListColumn{
				{Title: "Cluster Name", Path: "ClusterName", Width: 32},
				{Title: "Status", Path: "Status", Width: 12},
				{Title: "Running", Path: "RunningTasksCount", Width: 9},
				{Title: "Pending", Path: "PendingTasksCount", Width: 9},
				{Title: "Services", Path: "ActiveServicesCount", Width: 10},
			},
			Detail: []string{
				"ClusterName", "ClusterArn", "Status",
				"RunningTasksCount", "PendingTasksCount",
				"ActiveServicesCount", "RegisteredContainerInstancesCount",
				"CapacityProviders", "DefaultCapacityProviderStrategy",
				"Settings", "Tags",
			},
		},
		"ecs-svc": {
			List: []ListColumn{
				{Title: "Service Name", Path: "ServiceName", Width: 32},
				{Title: "Status", Path: "Status", Width: 12},
				{Title: "Desired", Path: "DesiredCount", Width: 9},
				{Title: "Running", Path: "RunningCount", Width: 9},
				{Title: "Launch Type", Path: "LaunchType", Width: 12},
			},
			Detail: []string{
				"ServiceName", "ServiceArn", "ClusterArn", "Status",
				"DesiredCount", "RunningCount", "PendingCount", "LaunchType",
				"TaskDefinition", "DeploymentConfiguration", "Deployments",
				"NetworkConfiguration", "LoadBalancers", "Events",
				"PlatformVersion", "SchedulingStrategy", "EnableExecuteCommand",
				"RoleArn", "CreatedAt", "Tags",
			},
		},
		"ecs-task": {
			List: []ListColumn{
				{Title: "Task ID", Path: "TaskArn", Width: 38},
				{Title: "Cluster", Path: "ClusterArn", Width: 24},
				{Title: "Status", Path: "LastStatus", Width: 12},
				{Title: "Task Definition", Path: "TaskDefinitionArn", Width: 30},
				{Title: "Launch", Path: "LaunchType", Width: 10},
				{Title: "CPU", Path: "Cpu", Width: 6},
				{Title: "Memory", Path: "Memory", Width: 8},
			},
			Detail: []string{
				"TaskArn", "ClusterArn", "LastStatus", "DesiredStatus",
				"TaskDefinitionArn", "LaunchType", "Cpu", "Memory",
				"Group", "StartedBy", "StartedAt", "StoppedAt",
				"StoppedReason", "StopCode", "HealthStatus",
				"Connectivity", "PlatformVersion", "PlatformFamily",
				"AvailabilityZone", "Containers", "Attachments",
				"EnableExecuteCommand", "Tags",
			},
		},
		"lambda": {
			List: []ListColumn{
				{Title: "Function Name", Path: "FunctionName", Width: 36},
				{Title: "Runtime", Path: "Runtime", Width: 16},
				{Title: "Memory", Path: "MemorySize", Width: 8},
				{Title: "Timeout", Path: "Timeout", Width: 8},
				{Title: "State", Path: "State", Width: 10},
				{Title: "Last Modified", Path: "LastModified", Width: 22},
			},
			Detail: []string{
				"FunctionName", "FunctionArn", "Runtime", "Handler",
				"MemorySize", "Timeout", "EphemeralStorage", "CodeSize",
				"Description", "Role", "PackageType", "Architectures",
				"State", "LastUpdateStatus", "LastUpdateStatusReason",
				"Environment", "VpcConfig", "DeadLetterConfig",
				"TracingConfig", "Layers", "LoggingConfig", "LastModified",
			},
		},
		"asg": {
			List: []ListColumn{
				{Title: "ASG Name", Path: "AutoScalingGroupName", Width: 36},
				{Title: "Min", Path: "MinSize", Width: 6},
				{Title: "Max", Path: "MaxSize", Width: 6},
				{Title: "Desired", Path: "DesiredCapacity", Width: 8},
				{Title: "Instances", Path: "Instances", Width: 10},
				{Title: "Status", Path: "Status", Width: 12},
			},
			Detail: []string{
				"AutoScalingGroupName", "AutoScalingGroupARN",
				"MinSize", "MaxSize", "DesiredCapacity",
				"AvailabilityZones", "LaunchConfigurationName",
				"HealthCheckType", "HealthCheckGracePeriod",
				"TargetGroupARNs", "LoadBalancerNames",
				"SuspendedProcesses", "TerminationPolicies",
				"VPCZoneIdentifier", "CreatedTime", "Tags",
			},
		},
		"eb": {
			List: []ListColumn{
				{Title: "Environment", Path: "EnvironmentName", Width: 28},
				{Title: "Application", Path: "ApplicationName", Width: 24},
				{Title: "Status", Path: "Status", Width: 12},
				{Title: "Health", Path: "Health", Width: 10},
				{Title: "Version", Path: "VersionLabel", Width: 16},
			},
			Detail: []string{
				"EnvironmentName", "EnvironmentId", "ApplicationName",
				"Status", "Health", "HealthStatus",
				"VersionLabel", "SolutionStackName", "PlatformArn",
				"EndpointURL", "CNAME", "DateCreated", "DateUpdated",
				"EnvironmentArn",
			},
		},
		"asg_activities": {
			List: []ListColumn{
				{Title: "Start Time", Key: "start_time", Width: 22},
				{Title: "Status", Key: "status_code", Width: 14},
				{Title: "Description", Key: "description", Width: 50},
				{Title: "Cause", Key: "cause", Width: 40},
			},
			Detail: []string{
				"ActivityId", "StartTime", "EndTime", "StatusCode", "StatusMessage",
				"Description", "Cause", "Details", "Progress",
				"AutoScalingGroupName", "AutoScalingGroupARN", "AutoScalingGroupState",
			},
		},
		// Child views for compute resources
		"ecs_tasks": {
			List: []ListColumn{
				{Title: "Task ID", Key: "task_id_short", Width: 14},
				{Title: "Status", Key: "status", Width: 12},
				{Title: "Health", Key: "health", Width: 10},
				{Title: "Task Definition", Key: "task_def_short", Width: 28},
				{Title: "Started At", Key: "started_at", Width: 22},
				{Title: "Stopped Reason", Key: "stopped_reason", Width: 40},
			},
			Detail: []string{
				"TaskArn", "ClusterArn", "LastStatus", "DesiredStatus",
				"HealthStatus", "TaskDefinitionArn", "StartedAt", "StoppedAt",
				"StoppedReason", "StopCode", "LaunchType", "PlatformVersion",
				"Cpu", "Memory", "Group", "StartedBy",
				"Containers", "Attachments", "Tags",
			},
		},
		"ecs_svc_events": {
			List: []ListColumn{
				{Title: "Timestamp", Key: "timestamp", Width: 22},
				{Title: "Message", Key: "message", Width: 120},
			},
			Detail: []string{
				"Id", "CreatedAt", "Message",
			},
		},
		"ecs_svc_logs": {
			List: []ListColumn{
				{Title: "Timestamp", Key: "timestamp", Width: 22},
				{Title: "Stream", Key: "stream_short", Width: 20},
				{Title: "Message", Key: "message", Width: 120},
			},
			Detail: []string{
				"Timestamp", "Message", "IngestionTime", "EventId", "LogStreamName",
			},
		},
		"lambda_invocations": {
			List: []ListColumn{
				{Title: "Timestamp", Key: "timestamp", Width: 22},
				{Title: "Request ID", Key: "request_id", Width: 38},
				{Title: "Status", Key: "status", Width: 10},
				{Title: "Duration", Key: "duration_ms", Width: 14},
				{Title: "Memory", Key: "memory_used", Width: 16},
				{Title: "Cold Start", Key: "cold_start", Width: 12},
			},
			Detail: []string{
				"request_id", "timestamp", "status",
				"duration_ms", "billed_duration_ms",
				"memory_size_mb", "memory_used_mb",
				"init_duration_ms", "xray_trace_id",
			},
		},
		"lambda_invocation_logs": {
			List: []ListColumn{
				{Title: "Timestamp", Key: "timestamp", Width: 22},
				{Title: "Message", Key: "message", Width: 120},
			},
			Detail: []string{
				"timestamp", "message",
			},
		},
	}
}
