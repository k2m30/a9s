// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

func computeDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"ec2": {
			Detail: []DetailField{
				{Path: "InstanceId"}, {Path: "State.Name", Label: "State"}, {Path: "InstanceType"}, {Key: "lifecycle", Label: "InstanceLifecycle"}, {Path: "ImageId"},
				{Path: "KeyName"}, {Path: "Placement"},
				{Path: "VpcId"}, {Path: "SubnetId"}, {Path: "PrivateIpAddress"}, {Path: "PrivateDnsName"},
				{Path: "PublicIpAddress"}, {Path: "IamInstanceProfile"},
				{Path: "SecurityGroups"}, {Path: "BlockDeviceMappings"}, {Path: "EbsOptimized"}, {Path: "MetadataOptions"},
				{Path: "LaunchTime"}, {Path: "Architecture"}, {Path: "Platform"}, {Path: "Tags"},
				{Path: "NetworkInterfaces"}, {Path: "UserData"},
			},
		},
		"ecs": {
			Detail: []DetailField{
				{Path: "ClusterName"}, {Path: "ClusterArn"}, {Path: "Status"},
				{Path: "RunningTasksCount"}, {Path: "PendingTasksCount"},
				{Path: "ActiveServicesCount"}, {Path: "RegisteredContainerInstancesCount"},
				{Path: "CapacityProviders"}, {Path: "DefaultCapacityProviderStrategy"},
				{Path: "Settings"}, {Path: "Configuration"}, {Path: "Tags"},
			},
		},
		"ecs-svc": {
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
			Detail: []DetailField{
				{Path: "TaskArn"}, {Path: "ClusterArn"}, {Path: "LastStatus"}, {Path: "DesiredStatus"},
				{Path: "TaskDefinitionArn"}, {Path: "LaunchType"}, {Path: "Cpu"}, {Path: "Memory"},
				{Path: "Group"}, {Path: "StartedBy"}, {Path: "StartedAt"}, {Path: "StoppedAt"},
				{Path: "StoppedReason"}, {Path: "StopCode"}, {Key: "health_status", Label: "HealthStatus"},
				{Path: "Connectivity"}, {Path: "PlatformVersion"}, {Path: "PlatformFamily"},
				{Path: "AvailabilityZone"}, {Path: "Containers"}, {Path: "Attachments"},
				{Path: "EnableExecuteCommand"}, {Path: "Tags"},
			},
		},
		"lambda": {
			Detail: []DetailField{
				{Path: "FunctionName"}, {Path: "FunctionArn"}, {Path: "Runtime"}, {Path: "Handler"},
				{Path: "MemorySize"}, {Path: "Timeout"}, {Path: "EphemeralStorage"}, {Key: "code_size", Label: "CodeSize"},
				{Path: "Description"}, {Path: "Role"}, {Path: "PackageType"}, {Path: "Architectures"},
				{Path: "State"}, {Path: "LastUpdateStatus"}, {Path: "LastUpdateStatusReason"},
				{Path: "Environment"}, {Path: "VpcConfig"}, {Path: "DeadLetterConfig"},
				{Path: "TracingConfig"}, {Path: "Layers"}, {Path: "LoggingConfig"}, {Path: "KMSKeyArn"},
				{Path: "LastModified"}, {Path: "Concurrency"},
			},
		},
		"asg": {
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
			Detail: []DetailField{
				{Path: "EnvironmentName"}, {Path: "EnvironmentId"}, {Path: "ApplicationName"},
				{Path: "Status"}, {Path: "Health"}, {Path: "HealthStatus"},
				{Path: "VersionLabel"}, {Path: "SolutionStackName"}, {Path: "PlatformArn"},
				{Path: "EndpointURL"}, {Path: "CNAME"}, {Path: "DateCreated"}, {Path: "DateUpdated"},
				{Path: "EnvironmentArn"},
			},
		},
		"asg_activities": {
			Detail: []DetailField{
				{Path: "ActivityId"}, {Path: "StartTime"}, {Path: "EndTime"}, {Path: "StatusCode"}, {Path: "StatusMessage"},
				{Path: "Description"}, {Path: "Cause"}, {Path: "Details"}, {Path: "Progress"},
				{Path: "AutoScalingGroupName"}, {Path: "AutoScalingGroupARN"}, {Path: "AutoScalingGroupState"},
			},
		},
		"ecs_tasks": {
			Detail: []DetailField{
				{Path: "TaskArn"}, {Path: "ClusterArn"}, {Path: "LastStatus"}, {Path: "DesiredStatus"},
				{Key: "health", Label: "HealthStatus"}, {Path: "TaskDefinitionArn"}, {Path: "StartedAt"}, {Path: "StoppedAt"},
				{Path: "StoppedReason"}, {Path: "StopCode"}, {Path: "LaunchType"}, {Path: "PlatformVersion"},
				{Path: "Cpu"}, {Path: "Memory"}, {Path: "Group"}, {Path: "StartedBy"},
				{Path: "Containers"}, {Path: "Attachments"}, {Path: "Tags"},
			},
		},
		"ecs_svc_events": {
			Detail: []DetailField{
				{Path: "Id"}, {Path: "CreatedAt"}, {Path: "Message"},
			},
		},
		"ecs_svc_logs": {
			Detail: []DetailField{
				{Key: "timestamp", Label: "Timestamp"}, {Path: "Message"}, {Key: "ingestion_time", Label: "IngestionTime"}, {Path: "EventId"}, {Path: "LogStreamName"},
			},
		},
		"lambda_invocations": {
			Detail: []DetailField{
				{Key: "request_id"}, {Key: "timestamp"}, {Key: "status"},
				{Key: "duration_ms"}, {Key: "billed_duration_ms"},
				{Key: "memory_size_mb"}, {Key: "memory_used_mb"},
				{Key: "init_duration_ms"}, {Key: "restore_duration_ms"},
				{Key: "error_type"}, {Key: "xray_trace_id"},
			},
		},
		"lambda_invocation_logs": {
			Detail: []DetailField{
				{Key: "timestamp"}, {Path: "message"},
			},
		},
		"ebs": {
			Detail: []DetailField{
				{Path: "VolumeId"}, {Path: "State"}, {Path: "Size"}, {Path: "VolumeType"}, {Path: "Iops"}, {Path: "Throughput"},
				{Path: "Encrypted"}, {Path: "KmsKeyId"}, {Path: "MultiAttachEnabled"},
				{Path: "AvailabilityZone"}, {Path: "CreateTime"},
				{Path: "Attachments"}, {Path: "Tags"},
			},
		},
		"ebs-snap": {
			Detail: []DetailField{
				{Path: "SnapshotId"}, {Path: "State"}, {Path: "VolumeId"}, {Path: "VolumeSize"},
				{Path: "Description"}, {Path: "Encrypted"}, {Path: "KmsKeyId"},
				{Path: "OwnerId"}, {Path: "Progress"}, {Path: "StartTime"}, {Path: "Tags"},
			},
		},
		"ami": {
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
		"lt": {
			Detail: []DetailField{
				{Path: "Template.LaunchTemplateId"}, {Path: "Template.LaunchTemplateName"},
				{Path: "Template.DefaultVersionNumber"}, {Path: "Template.LatestVersionNumber"},
				{Path: "Template.CreatedBy"}, {Path: "Template.CreateTime"}, {Path: "Template.Tags"},
				{Path: "DefaultVersion.LaunchTemplateData.ImageId"},
				{Path: "DefaultVersion.LaunchTemplateData.InstanceType"},
				{Path: "DefaultVersion.LaunchTemplateData.KeyName"},
				{Path: "DefaultVersion.LaunchTemplateData.IamInstanceProfile"},
				{Path: "DefaultVersion.LaunchTemplateData.SecurityGroupIds"},
				{Path: "DefaultVersion.LaunchTemplateData.SecurityGroups"},
				{Path: "DefaultVersion.LaunchTemplateData.NetworkInterfaces"},
				{Path: "DefaultVersion.LaunchTemplateData.BlockDeviceMappings"},
				{Path: "DefaultVersion.LaunchTemplateData.MetadataOptions"},
			},
		},
	}
}
