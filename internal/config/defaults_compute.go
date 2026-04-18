package config

func computeDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"ec2": {
			List: []ListColumn{
				{Title: "Name", Path: "", Width: 24},
				{Title: "State", Path: "State.Name", Width: 12},
				{Title: "Lifecycle", Key: "lifecycle", Width: 12},
				{Title: "Type", Path: "InstanceType", Width: 14},
				{Title: "Private IP", Path: "PrivateIpAddress", Width: 16},
				{Title: "Public IP", Path: "PublicIpAddress", Width: 16},
				{Title: "Instance ID", Path: "InstanceId", Width: 20},
				{Title: "Launch Time", Path: "LaunchTime", Width: 22},
			},
			Detail: []DetailField{
				{Path: "InstanceId"}, {Path: "State"}, {Path: "InstanceType"}, {Path: "InstanceLifecycle"}, {Path: "ImageId"},
				{Path: "KeyName"}, {Path: "Placement"},
				{Path: "VpcId"}, {Path: "SubnetId"}, {Path: "PrivateIpAddress"}, {Path: "PrivateDnsName"},
				{Path: "PublicIpAddress"}, {Path: "IamInstanceProfile"},
				{Path: "SecurityGroups"}, {Path: "BlockDeviceMappings"}, {Path: "EbsOptimized"}, {Path: "MetadataOptions"},
				{Path: "LaunchTime"}, {Path: "Architecture"}, {Path: "Platform"}, {Path: "Tags"},
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
			Detail: []DetailField{
				{Path: "ClusterName"}, {Path: "ClusterArn"}, {Path: "Status"},
				{Path: "RunningTasksCount"}, {Path: "PendingTasksCount"},
				{Path: "ActiveServicesCount"}, {Path: "RegisteredContainerInstancesCount"},
				{Path: "CapacityProviders"}, {Path: "DefaultCapacityProviderStrategy"},
				{Path: "Settings"}, {Path: "Tags"},
			},
		},
		"ecs-svc": {
			List: []ListColumn{
				{Title: "Service Name", Path: "ServiceName", Width: 32},
				{Title: "Cluster", Key: "cluster", Width: 24},
				{Title: "Status", Path: "Status", Width: 12},
				{Title: "Desired", Path: "DesiredCount", Width: 9},
				{Title: "Running", Path: "RunningCount", Width: 9},
				{Title: "Launch Type", Path: "LaunchType", Width: 12},
			},
			Detail: []DetailField{
				{Path: "ServiceName"}, {Path: "ServiceArn"}, {Path: "ClusterArn"}, {Path: "Status"},
				{Path: "DesiredCount"}, {Path: "RunningCount"}, {Path: "PendingCount"}, {Path: "LaunchType"},
				{Path: "TaskDefinition"}, {Path: "DeploymentConfiguration"}, {Path: "Deployments"},
				{Path: "NetworkConfiguration"}, {Path: "LoadBalancers"}, {Path: "Events"},
				{Path: "PlatformVersion"}, {Path: "SchedulingStrategy"}, {Path: "EnableExecuteCommand"},
				{Path: "RoleArn"}, {Path: "CreatedAt"}, {Path: "Tags"},
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
			Detail: []DetailField{
				{Path: "TaskArn"}, {Path: "ClusterArn"}, {Path: "LastStatus"}, {Path: "DesiredStatus"},
				{Path: "TaskDefinitionArn"}, {Path: "LaunchType"}, {Path: "Cpu"}, {Path: "Memory"},
				{Path: "Group"}, {Path: "StartedBy"}, {Path: "StartedAt"}, {Path: "StoppedAt"},
				{Path: "StoppedReason"}, {Path: "StopCode"}, {Path: "HealthStatus"},
				{Path: "Connectivity"}, {Path: "PlatformVersion"}, {Path: "PlatformFamily"},
				{Path: "AvailabilityZone"}, {Path: "Containers"}, {Path: "Attachments"},
				{Path: "EnableExecuteCommand"}, {Path: "Tags"},
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
			Detail: []DetailField{
				{Path: "FunctionName"}, {Path: "FunctionArn"}, {Path: "Runtime"}, {Path: "Handler"},
				{Path: "MemorySize"}, {Path: "Timeout"}, {Path: "EphemeralStorage"}, {Path: "CodeSize"},
				{Path: "Description"}, {Path: "Role"}, {Path: "PackageType"}, {Path: "Architectures"},
				{Path: "State"}, {Path: "LastUpdateStatus"}, {Path: "LastUpdateStatusReason"},
				{Path: "Environment"}, {Path: "VpcConfig"}, {Path: "DeadLetterConfig"},
				{Path: "TracingConfig"}, {Path: "Layers"}, {Path: "LoggingConfig"}, {Path: "LastModified"},
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
			Detail: []DetailField{
				{Path: "AutoScalingGroupName"}, {Path: "AutoScalingGroupARN"},
				{Path: "MinSize"}, {Path: "MaxSize"}, {Path: "DesiredCapacity"},
				{Path: "AvailabilityZones"}, {Path: "LaunchConfigurationName"},
				{Path: "HealthCheckType"}, {Path: "HealthCheckGracePeriod"},
				{Path: "TargetGroupARNs"}, {Path: "LoadBalancerNames"},
				{Path: "SuspendedProcesses"}, {Path: "TerminationPolicies"},
				{Path: "VPCZoneIdentifier"}, {Path: "CreatedTime"}, {Path: "Tags"},
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
			Detail: []DetailField{
				{Path: "EnvironmentName"}, {Path: "EnvironmentId"}, {Path: "ApplicationName"},
				{Path: "Status"}, {Path: "Health"}, {Path: "HealthStatus"},
				{Path: "VersionLabel"}, {Path: "SolutionStackName"}, {Path: "PlatformArn"},
				{Path: "EndpointURL"}, {Path: "CNAME"}, {Path: "DateCreated"}, {Path: "DateUpdated"},
				{Path: "EnvironmentArn"},
			},
		},
		"asg_activities": {
			List: []ListColumn{
				{Title: "Start Time", Key: "start_time", Width: 22},
				{Title: "Status", Key: "status_code", Width: 14},
				{Title: "Description", Key: "description", Width: 50},
				{Title: "Cause", Key: "cause", Width: 40},
			},
			Detail: []DetailField{
				{Path: "ActivityId"}, {Path: "StartTime"}, {Path: "EndTime"}, {Path: "StatusCode"}, {Path: "StatusMessage"},
				{Path: "Description"}, {Path: "Cause"}, {Path: "Details"}, {Path: "Progress"},
				{Path: "AutoScalingGroupName"}, {Path: "AutoScalingGroupARN"}, {Path: "AutoScalingGroupState"},
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
			Detail: []DetailField{
				{Path: "TaskArn"}, {Path: "ClusterArn"}, {Path: "LastStatus"}, {Path: "DesiredStatus"},
				{Path: "HealthStatus"}, {Path: "TaskDefinitionArn"}, {Path: "StartedAt"}, {Path: "StoppedAt"},
				{Path: "StoppedReason"}, {Path: "StopCode"}, {Path: "LaunchType"}, {Path: "PlatformVersion"},
				{Path: "Cpu"}, {Path: "Memory"}, {Path: "Group"}, {Path: "StartedBy"},
				{Path: "Containers"}, {Path: "Attachments"}, {Path: "Tags"},
			},
		},
		"ecs_svc_events": {
			List: []ListColumn{
				{Title: "Timestamp", Key: "timestamp", Width: 22},
				{Title: "Message", Key: "message", Width: 120},
			},
			Detail: []DetailField{
				{Path: "Id"}, {Path: "CreatedAt"}, {Path: "Message"},
			},
		},
		"ecs_svc_logs": {
			List: []ListColumn{
				{Title: "Timestamp", Key: "timestamp", Width: 22},
				{Title: "Stream", Key: "stream_short", Width: 20},
				{Title: "Message", Key: "message", Width: 120},
			},
			Detail: []DetailField{
				{Path: "Timestamp"}, {Path: "Message"}, {Path: "IngestionTime"}, {Path: "EventId"}, {Path: "LogStreamName"},
			},
		},
		"lambda_invocations": {
			List: []ListColumn{
				{Title: "Timestamp", Key: "timestamp", Width: 22},
				{Title: "Request ID", Key: "request_id", Width: 38},
				{Title: "Status", Key: "status", Width: 10},
				{Title: "Duration", Key: "duration_ms", SortKey: "duration_ms_raw", Width: 14},
				{Title: "Memory", Key: "memory_used", SortKey: "memory_used_mb_raw", Width: 16},
				{Title: "Cold Start", Key: "cold_start", Width: 12},
			},
			Detail: []DetailField{
				{Path: "request_id"}, {Path: "timestamp"}, {Path: "status"},
				{Path: "duration_ms"}, {Path: "billed_duration_ms"},
				{Path: "memory_size_mb"}, {Path: "memory_used_mb"},
				{Path: "init_duration_ms"}, {Path: "xray_trace_id"},
			},
		},
		"lambda_invocation_logs": {
			List: []ListColumn{
				{Title: "Timestamp", Key: "timestamp", Width: 22},
				{Title: "Message", Key: "message", Width: 120},
			},
			Detail: []DetailField{
				{Path: "timestamp"}, {Path: "message"},
			},
		},
		"ebs": {
			List: []ListColumn{
				{Title: "Name", Path: "", Width: 24},
				{Title: "Volume ID", Path: "VolumeId", Width: 22},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Size (GiB)", Path: "Size", Width: 10},
				{Title: "Type", Path: "VolumeType", Width: 8},
				{Title: "IOPS", Path: "Iops", Width: 8},
				{Title: "Encrypted", Path: "Encrypted", Width: 10},
				{Title: "Attached To", Path: "Attachments", Width: 20},
				{Title: "AZ", Path: "AvailabilityZone", Width: 16},
				{Title: "Created", Path: "CreateTime", Width: 18},
			},
			Detail: []DetailField{
				{Path: "VolumeId"}, {Path: "State"}, {Path: "Size"}, {Path: "VolumeType"}, {Path: "Iops"}, {Path: "Throughput"},
				{Path: "Encrypted"}, {Path: "KmsKeyId"}, {Path: "MultiAttachEnabled"},
				{Path: "AvailabilityZone"}, {Path: "CreateTime"},
				{Path: "Attachments"}, {Path: "Tags"},
			},
		},
		"ebs-snap": {
			List: []ListColumn{
				{Title: "Name", Path: "", Width: 24},
				{Title: "Snapshot ID", Path: "SnapshotId", Width: 24},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Volume ID", Path: "VolumeId", Width: 22},
				{Title: "Size (GiB)", Path: "VolumeSize", Width: 10},
				{Title: "Encrypted", Path: "Encrypted", Width: 10},
				{Title: "Description", Path: "Description", Width: 30},
				{Title: "Started", Path: "StartTime", Width: 18},
				{Title: "Progress", Path: "Progress", Width: 10},
			},
			Detail: []DetailField{
				{Path: "SnapshotId"}, {Path: "State"}, {Path: "VolumeId"}, {Path: "VolumeSize"},
				{Path: "Description"}, {Path: "Encrypted"}, {Path: "KmsKeyId"},
				{Path: "OwnerId"}, {Path: "Progress"}, {Path: "StartTime"}, {Path: "Tags"},
			},
		},
		"ami": {
			List: []ListColumn{
				{Title: "Name", Path: "Name", Width: 32},
				{Title: "Image ID", Path: "ImageId", Width: 22},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Arch", Path: "Architecture", Width: 10},
				{Title: "Platform", Path: "PlatformDetails", Width: 16},
				{Title: "Root Device", Path: "RootDeviceType", Width: 14},
				{Title: "Created", Path: "CreationDate", Width: 22},
				{Title: "Public", Path: "Public", Width: 8},
			},
			Detail: []DetailField{
				{Path: "ImageId"}, {Path: "Name"}, {Path: "State"}, {Path: "Description"},
				{Path: "Architecture"}, {Path: "PlatformDetails"}, {Path: "UsageOperation"},
				{Path: "Hypervisor"}, {Path: "ImageOwnerAlias"}, {Path: "RootDeviceName"}, {Path: "RootDeviceType"},
				{Path: "SriovNetSupport"}, {Path: "VirtualizationType"}, {Path: "EnaSupport"}, {Path: "BootMode"},
				{Path: "CreationDate"}, {Path: "DeprecationTime"}, {Path: "Public"},
				{Path: "OwnerId"}, {Path: "ImageLocation"},
				{Path: "BlockDeviceMappings"}, {Path: "Tags"},
			},
		},
	}
}
