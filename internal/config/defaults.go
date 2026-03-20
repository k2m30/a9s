package config

// defaultViews holds the built-in view definitions for all supported resource types.
// Paths use Go field names from AWS SDK v2 structs. ExtractValue matches case-insensitively.
var defaultViews = ViewsConfig{
	Views: map[string]ViewDef{
		"s3_objects": {
			List: []ListColumn{
				{Title: "Key", Path: "Key", Width: 36},
				{Title: "Size", Path: "Size", Width: 12},
				{Title: "Storage Class", Path: "StorageClass", Width: 16},
				{Title: "Last Modified", Path: "LastModified", Width: 22},
			},
			Detail: []string{
				"Key", "Size", "LastModified", "StorageClass", "ETag",
				"ChecksumAlgorithm", "ChecksumType", "Owner", "RestoreStatus",
			},
		},
		"s3": {
			List: []ListColumn{
				{Title: "Bucket Name", Path: "Name", Width: 36},
				{Title: "Region", Path: "BucketRegion", Width: 14},
				{Title: "Creation Date", Path: "CreationDate", Width: 22},
			},
			Detail: []string{"Name", "BucketArn", "BucketRegion", "CreationDate"},
		},
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
		"dbi": {
			List: []ListColumn{
				{Title: "DB Identifier", Path: "DBInstanceIdentifier", Width: 28},
				{Title: "Engine", Path: "Engine", Width: 12},
				{Title: "Version", Path: "EngineVersion", Width: 10},
				{Title: "Status", Path: "DBInstanceStatus", Width: 14},
				{Title: "Class", Path: "DBInstanceClass", Width: 16},
				{Title: "Endpoint", Path: "Endpoint.Address", Width: 40},
				{Title: "Multi-AZ", Path: "MultiAZ", Width: 10},
			},
			Detail: []string{
				"DBInstanceIdentifier", "DBInstanceArn", "Engine", "EngineVersion",
				"DBInstanceStatus", "DBInstanceClass", "Endpoint", "MultiAZ",
				"AllocatedStorage", "StorageType", "Iops", "StorageEncrypted",
				"KmsKeyId", "AvailabilityZone", "PubliclyAccessible",
				"DBSubnetGroup", "VpcSecurityGroups", "BackupRetentionPeriod",
				"PreferredMaintenanceWindow", "PreferredBackupWindow",
				"DeletionProtection", "MasterUsername",
				"PerformanceInsightsEnabled", "Tags",
			},
		},
		"redis": {
			List: []ListColumn{
				{Title: "Cluster ID", Path: "CacheClusterId", Width: 28},
				{Title: "Version", Path: "EngineVersion", Width: 10},
				{Title: "Node Type", Path: "CacheNodeType", Width: 18},
				{Title: "Status", Path: "CacheClusterStatus", Width: 14},
				{Title: "Nodes", Path: "NumCacheNodes", Width: 8},
				{Title: "Endpoint", Path: "ConfigurationEndpoint.Address", Width: 40},
			},
			Detail: []string{
				"CacheClusterId", "ARN", "Engine", "EngineVersion",
				"CacheClusterStatus", "CacheNodeType", "NumCacheNodes",
				"CacheNodes", "ConfigurationEndpoint", "PreferredAvailabilityZone",
				"ReplicationGroupId", "CacheSubnetGroupName", "SecurityGroups",
				"AtRestEncryptionEnabled", "TransitEncryptionEnabled",
				"AuthTokenEnabled", "SnapshotRetentionLimit",
				"PreferredMaintenanceWindow",
			},
		},
		"dbc": {
			List: []ListColumn{
				{Title: "Cluster ID", Path: "DBClusterIdentifier", Width: 28},
				{Title: "Version", Path: "EngineVersion", Width: 10},
				{Title: "Status", Path: "Status", Width: 14},
				{Title: "Instances", Path: "DBClusterMembers", Width: 10},
				{Title: "Endpoint", Path: "Endpoint", Width: 48},
			},
			Detail: []string{
				"DBClusterIdentifier", "DBClusterArn", "Engine", "EngineVersion",
				"Status", "Endpoint", "ReaderEndpoint", "Port", "StorageEncrypted",
				"KmsKeyId", "DeletionProtection", "DBClusterMembers",
				"DBSubnetGroup", "VpcSecurityGroups", "BackupRetentionPeriod",
				"PreferredMaintenanceWindow", "MasterUsername",
			},
		},
		"eks": {
			List: []ListColumn{
				{Title: "Cluster Name", Path: "Name", Width: 28},
				{Title: "Version", Path: "Version", Width: 10},
				{Title: "Status", Path: "Status", Width: 14},
				{Title: "Endpoint", Path: "Endpoint", Width: 48},
				{Title: "Platform Version", Path: "PlatformVersion", Width: 18},
			},
			Detail: []string{
				"Name", "Version", "Status", "Endpoint",
				"PlatformVersion", "Arn", "RoleArn", "KubernetesNetworkConfig",
				"ResourcesVpcConfig", "Logging", "Identity", "CreatedAt", "Tags",
			},
		},
		"secrets": {
			List: []ListColumn{
				{Title: "Secret Name", Path: "Name", Width: 36},
				{Title: "Description", Path: "Description", Width: 30},
				{Title: "Last Accessed", Path: "LastAccessedDate", Width: 18},
				{Title: "Last Changed", Path: "LastChangedDate", Width: 18},
				{Title: "Rotation", Path: "RotationEnabled", Width: 10},
			},
			Detail: []string{
				"Name", "Description", "LastAccessedDate", "LastChangedDate",
				"RotationEnabled", "ARN", "KmsKeyId",
				"CreatedDate", "LastRotatedDate", "RotationLambdaARN",
				"RotationRules", "PrimaryRegion", "Tags",
			},
		},
		"vpc": {
			List: []ListColumn{
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Name", Path: "", Width: 24},
				{Title: "CIDR Block", Path: "CidrBlock", Width: 18},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Default", Path: "IsDefault", Width: 9},
			},
			Detail: []string{
				"VpcId", "CidrBlock", "State", "IsDefault",
				"InstanceTenancy", "DhcpOptionsId", "OwnerId",
				"CidrBlockAssociationSet", "Ipv6CidrBlockAssociationSet", "Tags",
			},
		},
		"sg": {
			List: []ListColumn{
				{Title: "Group ID", Path: "GroupId", Width: 24},
				{Title: "Group Name", Path: "GroupName", Width: 28},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Description", Path: "Description", Width: 36},
			},
			Detail: []string{
				"GroupId", "GroupName", "VpcId", "Description",
				"OwnerId", "SecurityGroupArn", "IpPermissions",
				"IpPermissionsEgress", "Tags",
			},
		},
		"ng": {
			List: []ListColumn{
				{Title: "Node Group", Path: "NodegroupName", Width: 28},
				{Title: "Cluster", Path: "ClusterName", Width: 24},
				{Title: "Status", Path: "Status", Width: 14},
				{Title: "Instance Types", Path: "InstanceTypes", Width: 20},
				{Title: "Desired", Path: "ScalingConfig.DesiredSize", Width: 9},
			},
			Detail: []string{
				"NodegroupName", "ClusterName", "Status", "InstanceTypes",
				"AmiType", "CapacityType", "DiskSize", "ScalingConfig",
				"NodeRole", "NodegroupArn", "ReleaseVersion", "Version",
				"Subnets", "LaunchTemplate", "Labels", "Taints",
				"Tags", "Health", "CreatedAt",
			},
		},
		"subnet": {
			List: []ListColumn{
				{Title: "Subnet ID", Path: "SubnetId", Width: 26},
				{Title: "Name", Path: "", Width: 28},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "CIDR Block", Path: "CidrBlock", Width: 18},
				{Title: "AZ", Path: "AvailabilityZone", Width: 14},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Available IPs", Path: "AvailableIpAddressCount", Width: 14},
			},
			Detail: []string{
				"SubnetId", "VpcId", "CidrBlock", "AvailabilityZone",
				"AvailabilityZoneId", "State", "AvailableIpAddressCount",
				"MapPublicIpOnLaunch", "DefaultForAz", "SubnetArn", "OwnerId", "Tags",
			},
		},
		"rtb": {
			List: []ListColumn{
				{Title: "Route Table ID", Path: "RouteTableId", Width: 26},
				{Title: "Name", Path: "", Width: 28},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Routes", Path: "", Key: "routes_count", Width: 8},
				{Title: "Assoc.", Path: "", Key: "associations_count", Width: 8},
			},
			Detail: []string{
				"RouteTableId", "VpcId", "Routes", "Associations",
				"OwnerId", "Tags",
			},
		},
		"nat": {
			List: []ListColumn{
				{Title: "NAT Gateway ID", Path: "NatGatewayId", Width: 26},
				{Title: "Name", Path: "", Width: 24},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Subnet ID", Path: "SubnetId", Width: 26},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Public IP", Path: "", Key: "public_ip", Width: 16},
			},
			Detail: []string{
				"NatGatewayId", "VpcId", "SubnetId", "State",
				"ConnectivityType", "NatGatewayAddresses", "CreateTime",
				"FailureCode", "FailureMessage", "Tags",
			},
		},
		"igw": {
			List: []ListColumn{
				{Title: "IGW ID", Path: "InternetGatewayId", Width: 26},
				{Title: "Name", Path: "", Width: 28},
				{Title: "VPC ID", Path: "", Key: "vpc_id", Width: 24},
				{Title: "State", Path: "", Key: "state", Width: 12},
			},
			Detail: []string{
				"InternetGatewayId", "Attachments", "OwnerId", "Tags",
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
		"alarm": {
			List: []ListColumn{
				{Title: "Alarm Name", Path: "AlarmName", Width: 36},
				{Title: "State", Path: "StateValue", Width: 12},
				{Title: "Metric", Path: "MetricName", Width: 24},
				{Title: "Namespace", Path: "Namespace", Width: 24},
				{Title: "Threshold", Path: "Threshold", Width: 12},
			},
			Detail: []string{
				"AlarmName", "AlarmArn", "StateValue", "StateReason",
				"StateUpdatedTimestamp", "StateTransitionedTimestamp",
				"MetricName", "Namespace", "Statistic", "Period",
				"EvaluationPeriods", "DatapointsToAlarm", "Threshold",
				"ComparisonOperator", "TreatMissingData", "Dimensions",
				"AlarmDescription", "AlarmActions", "OKActions",
				"InsufficientDataActions", "ActionsEnabled",
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
		"elb": {
			List: []ListColumn{
				{Title: "Name", Path: "LoadBalancerName", Width: 32},
				{Title: "Type", Path: "Type", Width: 12},
				{Title: "Scheme", Path: "Scheme", Width: 14},
				{Title: "State", Path: "State.Code", Width: 12},
				{Title: "DNS Name", Path: "DNSName", Width: 48},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
			},
			Detail: []string{
				"LoadBalancerName", "LoadBalancerArn", "DNSName", "Type",
				"Scheme", "State", "VpcId", "AvailabilityZones",
				"SecurityGroups", "IpAddressType", "CanonicalHostedZoneId",
				"CreatedTime",
			},
		},
		"tg": {
			List: []ListColumn{
				{Title: "Target Group", Path: "TargetGroupName", Width: 32},
				{Title: "Port", Path: "Port", Width: 8},
				{Title: "Protocol", Path: "Protocol", Width: 10},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Target Type", Path: "TargetType", Width: 12},
				{Title: "Health Check", Path: "HealthCheckPath", Width: 24},
			},
			Detail: []string{
				"TargetGroupName", "TargetGroupArn", "Port", "Protocol",
				"ProtocolVersion", "VpcId", "TargetType", "HealthCheckPath",
				"HealthCheckPort", "HealthCheckProtocol", "HealthCheckEnabled",
				"HealthCheckIntervalSeconds", "HealthCheckTimeoutSeconds",
				"HealthyThresholdCount", "UnhealthyThresholdCount",
				"Matcher", "LoadBalancerArns",
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
		"cfn": {
			List: []ListColumn{
				{Title: "Stack Name", Path: "StackName", Width: 36},
				{Title: "Status", Path: "StackStatus", Width: 24},
				{Title: "Created", Path: "CreationTime", Width: 22},
				{Title: "Updated", Path: "LastUpdatedTime", Width: 22},
				{Title: "Description", Path: "Description", Width: 30},
			},
			Detail: []string{
				"StackName", "StackId", "StackStatus", "DetailedStatus",
				"StackStatusReason", "CreationTime", "LastUpdatedTime",
				"DeletionTime", "Description", "RoleARN", "Capabilities",
				"EnableTerminationProtection", "DriftInformation",
				"Parameters", "Outputs", "Tags",
			},
		},
		"role": {
			List: []ListColumn{
				{Title: "Role Name", Path: "RoleName", Width: 36},
				{Title: "Last Used", Path: "RoleLastUsed.LastUsedDate", Width: 22},
				{Title: "Path", Path: "Path", Width: 20},
				{Title: "Created", Path: "CreateDate", Width: 22},
				{Title: "Description", Path: "Description", Width: 30},
			},
			Detail: []string{
				"RoleName", "RoleId", "Arn", "Path",
				"CreateDate", "Description", "MaxSessionDuration",
				"RoleLastUsed", "PermissionsBoundary",
				"AssumeRolePolicyDocument", "Tags",
			},
		},
		"logs": {
			List: []ListColumn{
				{Title: "Log Group Name", Path: "LogGroupName", Width: 48},
				{Title: "Size (bytes)", Path: "StoredBytes", Width: 14},
				{Title: "Retention", Path: "RetentionInDays", Width: 10},
				{Title: "Metric Filters", Path: "MetricFilterCount", Width: 8},
				{Title: "Created", Path: "CreationTime", Width: 16},
			},
			Detail: []string{
				"LogGroupName", "LogGroupArn", "LogGroupClass",
				"StoredBytes", "RetentionInDays", "MetricFilterCount",
				"DeletionProtectionEnabled", "CreationTime",
				"KmsKeyId", "DataProtectionStatus",
			},
		},
		"ssm": {
			List: []ListColumn{
				{Title: "Name", Path: "Name", Width: 40},
				{Title: "Type", Path: "Type", Width: 14},
				{Title: "Version", Path: "Version", Width: 8},
				{Title: "Last Modified", Path: "LastModifiedDate", Width: 22},
				{Title: "Description", Path: "Description", Width: 30},
			},
			Detail: []string{
				"Name", "Type", "Version", "LastModifiedDate",
				"LastModifiedUser", "Description", "KeyId",
				"Tier", "DataType", "AllowedPattern",
			},
		},
		"ddb": {
			List: []ListColumn{
				{Title: "Table Name", Path: "TableName", Width: 36},
				{Title: "Status", Path: "TableStatus", Width: 12},
				{Title: "Items", Path: "ItemCount", Width: 12},
				{Title: "Size (bytes)", Path: "TableSizeBytes", Width: 14},
				{Title: "Billing", Path: "BillingModeSummary.BillingMode", Width: 16},
			},
			Detail: []string{
				"TableName", "TableArn", "TableId", "TableStatus",
				"ItemCount", "TableSizeBytes", "BillingModeSummary",
				"GlobalSecondaryIndexes", "LocalSecondaryIndexes",
				"ProvisionedThroughput", "DeletionProtectionEnabled",
				"StreamSpecification", "SSEDescription",
				"CreationDateTime", "KeySchema", "AttributeDefinitions", "Tags",
			},
		},
		"eip": {
			List: []ListColumn{
				{Title: "Allocation ID", Path: "AllocationId", Width: 26},
				{Title: "Name", Path: "", Width: 24},
				{Title: "Public IP", Path: "PublicIp", Width: 16},
				{Title: "Association", Path: "AssociationId", Width: 26},
				{Title: "Instance", Path: "InstanceId", Width: 20},
				{Title: "Domain", Path: "Domain", Width: 8},
			},
			Detail: []string{
				"AllocationId", "PublicIp", "AssociationId", "InstanceId",
				"Domain", "NetworkBorderGroup", "SubnetId",
				"PrivateIpAddress", "NetworkInterfaceId", "Tags",
			},
		},
		"acm": {
			List: []ListColumn{
				{Title: "Domain Name", Path: "DomainName", Width: 40},
				{Title: "Status", Path: "Status", Width: 14},
				{Title: "Type", Path: "Type", Width: 14},
				{Title: "Expires", Path: "NotAfter", Width: 22},
				{Title: "In Use", Path: "InUse", Width: 8},
			},
			Detail: []string{
				"DomainName", "CertificateArn", "SubjectAlternativeNameSummaries",
				"Status", "Type", "NotBefore", "NotAfter",
				"IssuedAt", "ImportedAt", "InUse", "CreatedAt",
				"RenewalEligibility", "KeyAlgorithm",
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
		"policy": {
			List: []ListColumn{
				{Title: "Policy Name", Path: "PolicyName", Width: 36},
				{Title: "Policy ID", Path: "PolicyId", Width: 22},
				{Title: "Attached", Path: "AttachmentCount", Width: 10},
				{Title: "Path", Path: "Path", Width: 20},
				{Title: "Created", Path: "CreateDate", Width: 22},
			},
			Detail: []string{
				"PolicyName", "PolicyId", "Arn", "Path",
				"AttachmentCount", "PermissionsBoundaryUsageCount",
				"IsAttachable", "DefaultVersionId",
				"CreateDate", "UpdateDate", "Description", "Tags",
			},
		},
		"rds-snap": {
			List: []ListColumn{
				{Title: "Snapshot ID", Path: "DBSnapshotIdentifier", Width: 36},
				{Title: "DB Instance", Path: "DBInstanceIdentifier", Width: 28},
				{Title: "Status", Path: "Status", Width: 12},
				{Title: "Engine", Path: "Engine", Width: 12},
				{Title: "Type", Path: "SnapshotType", Width: 12},
				{Title: "Created", Path: "SnapshotCreateTime", Width: 22},
			},
			Detail: []string{
				"DBSnapshotIdentifier", "DBSnapshotArn", "DBInstanceIdentifier",
				"Status", "Engine", "EngineVersion", "SnapshotType",
				"SnapshotCreateTime", "AllocatedStorage", "StorageType",
				"Encrypted", "KmsKeyId", "AvailabilityZone",
				"MasterUsername", "LicenseModel", "Iops",
				"PercentProgress", "SourceRegion",
			},
		},
		"tgw": {
			List: []ListColumn{
				{Title: "TGW ID", Path: "TransitGatewayId", Width: 26},
				{Title: "Name", Path: "", Width: 28},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Owner", Path: "OwnerId", Width: 14},
				{Title: "Description", Path: "Description", Width: 30},
			},
			Detail: []string{
				"TransitGatewayId", "TransitGatewayArn", "State",
				"OwnerId", "Description", "Options",
				"CreationTime", "Tags",
			},
		},
		"vpce": {
			List: []ListColumn{
				{Title: "Endpoint ID", Path: "VpcEndpointId", Width: 26},
				{Title: "Service Name", Path: "ServiceName", Width: 40},
				{Title: "Type", Path: "VpcEndpointType", Width: 12},
				{Title: "State", Path: "State", Width: 12},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
			},
			Detail: []string{
				"VpcEndpointId", "ServiceName", "VpcEndpointType",
				"State", "VpcId", "SubnetIds", "NetworkInterfaceIds",
				"RouteTableIds", "Groups", "PrivateDnsEnabled",
				"PolicyDocument", "CreationTimestamp",
				"OwnerId", "Tags",
			},
		},
		"eni": {
			List: []ListColumn{
				{Title: "ENI ID", Path: "NetworkInterfaceId", Width: 26},
				{Title: "Name", Path: "", Width: 24},
				{Title: "Status", Path: "Status", Width: 12},
				{Title: "Type", Path: "InterfaceType", Width: 14},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Private IP", Path: "PrivateIpAddress", Width: 16},
			},
			Detail: []string{
				"NetworkInterfaceId", "Status", "InterfaceType",
				"VpcId", "SubnetId", "AvailabilityZone",
				"PrivateIpAddress", "PrivateDnsName",
				"MacAddress", "Description", "OwnerId",
				"RequesterId", "RequesterManaged",
				"SourceDestCheck", "Groups", "Attachment",
				"Association", "TagSet",
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
		"iam-user": {
			List: []ListColumn{
				{Title: "User Name", Path: "UserName", Width: 32},
				{Title: "User ID", Path: "UserId", Width: 22},
				{Title: "Path", Path: "Path", Width: 20},
				{Title: "Created", Path: "CreateDate", Width: 22},
				{Title: "Password Last Used", Path: "PasswordLastUsed", Width: 22},
			},
			Detail: []string{
				"UserName", "UserId", "Arn", "Path",
				"CreateDate", "PasswordLastUsed",
				"PermissionsBoundary", "Tags",
			},
		},
		"iam-group": {
			List: []ListColumn{
				{Title: "Group Name", Path: "GroupName", Width: 32},
				{Title: "Group ID", Path: "GroupId", Width: 22},
				{Title: "Path", Path: "Path", Width: 20},
				{Title: "Created", Path: "CreateDate", Width: 22},
				{Title: "ARN", Path: "Arn", Width: 60},
			},
			Detail: []string{
				"GroupName", "GroupId", "Arn", "Path", "CreateDate",
			},
		},
		"docdb-snap": {
			List: []ListColumn{
				{Title: "Snapshot ID", Path: "DBClusterSnapshotIdentifier", Width: 36},
				{Title: "Cluster ID", Path: "DBClusterIdentifier", Width: 28},
				{Title: "Status", Path: "Status", Width: 12},
				{Title: "Engine", Path: "Engine", Width: 12},
				{Title: "Type", Path: "SnapshotType", Width: 12},
				{Title: "Created", Path: "SnapshotCreateTime", Width: 22},
				{Title: "Storage", Path: "StorageType", Width: 10},
			},
			Detail: []string{
				"DBClusterSnapshotIdentifier", "DBClusterSnapshotArn",
				"DBClusterIdentifier", "Status", "Engine", "EngineVersion",
				"SnapshotType", "SnapshotCreateTime", "ClusterCreateTime",
				"MasterUsername", "Port", "VpcId",
				"StorageEncrypted", "KmsKeyId", "StorageType",
				"PercentProgress", "SourceDBClusterSnapshotArn",
				"AvailabilityZones",
			},
		},
		"cf": {
			List: []ListColumn{
				{Title: "Distribution ID", Path: "Id", Width: 16},
				{Title: "Domain Name", Path: "DomainName", Width: 40},
				{Title: "Status", Path: "Status", Width: 12},
				{Title: "Enabled", Path: "Enabled", Width: 9},
				{Title: "Aliases", Path: "Aliases.Items", Width: 30},
				{Title: "Price Class", Path: "PriceClass", Width: 16},
			},
			Detail: []string{
				"Id", "DomainName", "Status", "Enabled", "Comment",
				"ARN", "Aliases", "Origins", "PriceClass", "HttpVersion",
				"LastModifiedTime", "DefaultCacheBehavior",
			},
		},
		"r53": {
			List: []ListColumn{
				{Title: "Zone ID", Path: "Id", Width: 30},
				{Title: "Name", Path: "Name", Width: 36},
				{Title: "Records", Path: "ResourceRecordSetCount", Width: 9},
				{Title: "Private", Path: "Config.PrivateZone", Width: 9},
				{Title: "Comment", Path: "Config.Comment", Width: 30},
			},
			Detail: []string{
				"Id", "Name", "CallerReference", "ResourceRecordSetCount",
				"Config", "LinkedService",
			},
		},
		"r53_records": {
			List: []ListColumn{
				{Title: "Name", Path: "Name", Width: 40},
				{Title: "Type", Path: "Type", Width: 8},
				{Title: "TTL", Path: "TTL", Width: 8},
				{Title: "Values", Path: "", Key: "values", Width: 50},
			},
			Detail: []string{
				"Name", "Type", "TTL", "ResourceRecords", "AliasTarget",
				"SetIdentifier", "Weight", "Region", "Failover",
				"GeoLocation", "HealthCheckId", "MultiValueAnswer",
			},
		},
		"apigw": {
			List: []ListColumn{
				{Title: "API ID", Path: "ApiId", Width: 14},
				{Title: "Name", Path: "Name", Width: 28},
				{Title: "Protocol", Path: "ProtocolType", Width: 12},
				{Title: "Endpoint", Path: "ApiEndpoint", Width: 50},
				{Title: "Description", Path: "Description", Width: 30},
			},
			Detail: []string{
				"ApiId", "Name", "ProtocolType", "ApiEndpoint",
				"Description", "CreatedDate", "ApiKeySelectionExpression",
				"RouteSelectionExpression", "CorsConfiguration", "Tags",
			},
		},
		"ecr": {
			List: []ListColumn{
				{Title: "Repository", Path: "RepositoryName", Width: 36},
				{Title: "URI", Path: "RepositoryUri", Width: 60},
				{Title: "Tag Mutability", Path: "ImageTagMutability", Width: 16},
				{Title: "Scan", Path: "ImageScanningConfiguration.ScanOnPush", Width: 6},
				{Title: "Created", Path: "CreatedAt", Width: 22},
			},
			Detail: []string{
				"RepositoryName", "RepositoryUri", "RepositoryArn",
				"RegistryId", "ImageTagMutability", "ImageScanningConfiguration",
				"EncryptionConfiguration", "CreatedAt",
			},
		},
		"efs": {
			List: []ListColumn{
				{Title: "File System ID", Path: "FileSystemId", Width: 22},
				{Title: "Name", Path: "Name", Width: 28},
				{Title: "State", Path: "LifeCycleState", Width: 12},
				{Title: "Perf Mode", Path: "PerformanceMode", Width: 16},
				{Title: "Encrypted", Path: "Encrypted", Width: 10},
				{Title: "Mounts", Path: "NumberOfMountTargets", Width: 8},
			},
			Detail: []string{
				"FileSystemId", "Name", "LifeCycleState", "PerformanceMode",
				"ThroughputMode", "Encrypted", "NumberOfMountTargets",
				"FileSystemArn", "OwnerId", "SizeInBytes", "CreationTime", "Tags",
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
		"pipeline": {
			List: []ListColumn{
				{Title: "Pipeline Name", Path: "Name", Width: 30},
				{Title: "Type", Path: "PipelineType", Width: 6},
				{Title: "Version", Path: "Version", Width: 9},
				{Title: "Created", Path: "Created", Width: 22},
				{Title: "Updated", Path: "Updated", Width: 22},
			},
			Detail: []string{
				"Name", "PipelineType", "Version", "Created",
				"Updated", "ExecutionMode",
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
		"waf": {
			List: []ListColumn{
				{Title: "Name", Path: "Name", Width: 28},
				{Title: "ID", Path: "Id", Width: 38},
				{Title: "Description", Path: "Description", Width: 36},
			},
			Detail: []string{
				"Name", "Id", "ARN", "Description", "LockToken",
			},
		},
		"glue": {
			List: []ListColumn{
				{Title: "Job Name", Path: "Name", Width: 32},
				{Title: "Version", Path: "GlueVersion", Width: 10},
				{Title: "Worker Type", Path: "WorkerType", Width: 14},
				{Title: "Workers", Path: "NumberOfWorkers", Width: 9},
				{Title: "Last Modified", Path: "LastModifiedOn", Width: 22},
			},
			Detail: []string{
				"Name", "Role", "GlueVersion", "WorkerType",
				"NumberOfWorkers", "MaxRetries", "Command",
				"CreatedOn", "LastModifiedOn",
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
		"ses": {
			List: []ListColumn{
				{Title: "Identity", Path: "IdentityName", Width: 36},
				{Title: "Type", Path: "IdentityType", Width: 16},
				{Title: "Verification", Path: "VerificationStatus", Width: 16},
				{Title: "Sending", Path: "SendingEnabled", Width: 10},
			},
			Detail: []string{
				"IdentityName", "IdentityType",
				"SendingEnabled", "VerificationStatus",
			},
		},
		"redshift": {
			List: []ListColumn{
				{Title: "Cluster ID", Path: "ClusterIdentifier", Width: 28},
				{Title: "Status", Path: "ClusterStatus", Width: 14},
				{Title: "Node Type", Path: "NodeType", Width: 16},
				{Title: "Nodes", Path: "NumberOfNodes", Width: 7},
				{Title: "Database", Path: "DBName", Width: 16},
				{Title: "Endpoint", Path: "Endpoint.Address", Width: 44},
			},
			Detail: []string{
				"ClusterIdentifier", "ClusterStatus", "NodeType",
				"NumberOfNodes", "DBName", "MasterUsername",
				"Endpoint", "ClusterCreateTime", "ClusterNamespaceArn",
				"AvailabilityZone",
			},
		},
		"trail": {
			List: []ListColumn{
				{Title: "Trail Name", Path: "Name", Width: 28},
				{Title: "S3 Bucket", Path: "S3BucketName", Width: 28},
				{Title: "Home Region", Path: "HomeRegion", Width: 16},
				{Title: "Multi-Region", Path: "IsMultiRegionTrail", Width: 14},
			},
			Detail: []string{
				"Name", "TrailARN", "S3BucketName", "HomeRegion",
				"IsMultiRegionTrail", "IsOrganizationTrail",
				"LogFileValidationEnabled", "IncludeGlobalServiceEvents",
				"KmsKeyId", "CloudWatchLogsLogGroupArn",
			},
		},
		"athena": {
			List: []ListColumn{
				{Title: "Workgroup", Path: "Name", Width: 28},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Description", Path: "Description", Width: 30},
				{Title: "Engine", Path: "EngineVersion.EffectiveEngineVersion", Width: 28},
			},
			Detail: []string{
				"Name", "State", "Description",
				"EngineVersion", "CreationTime",
			},
		},
		"codeartifact": {
			List: []ListColumn{
				{Title: "Repository", Path: "Name", Width: 28},
				{Title: "Domain", Path: "DomainName", Width: 24},
				{Title: "Description", Path: "Description", Width: 30},
				{Title: "Owner", Path: "DomainOwner", Width: 14},
			},
			Detail: []string{
				"Name", "DomainName", "DomainOwner", "Arn",
				"Description", "AdministratorAccount", "CreatedTime",
			},
		},
		"cb": {
			List: []ListColumn{
				{Title: "Project Name", Path: "Name", Width: 32},
				{Title: "Source Type", Path: "Source.Type", Width: 14},
				{Title: "Description", Path: "Description", Width: 36},
				{Title: "Last Modified", Path: "LastModified", Width: 22},
			},
			Detail: []string{
				"Name", "Description", "Arn", "Source",
				"Environment", "ServiceRole", "Created", "LastModified",
				"Cache", "LogsConfig", "ConcurrentBuildLimit", "Tags",
			},
		},
		"opensearch": {
			List: []ListColumn{
				{Title: "Domain Name", Path: "DomainName", Width: 28},
				{Title: "Engine Version", Path: "EngineVersion", Width: 16},
				{Title: "Instance Type", Path: "ClusterConfig.InstanceType", Width: 22},
				{Title: "Instances", Path: "ClusterConfig.InstanceCount", Width: 10},
				{Title: "Endpoint", Path: "Endpoint", Width: 48},
			},
			Detail: []string{
				"DomainName", "DomainId", "ARN", "EngineVersion",
				"ClusterConfig", "EBSOptions", "Endpoint", "Endpoints",
				"EncryptionAtRestOptions", "DomainEndpointOptions",
				"AdvancedSecurityOptions", "Created", "Deleted",
			},
		},
		"kms": {
			List: []ListColumn{
				{Title: "Alias", Path: "AliasName", Width: 32},
				{Title: "Key ID", Path: "KeyId", Width: 38},
				{Title: "Status", Path: "KeyState", Width: 12},
				{Title: "Description", Path: "Description", Width: 36},
			},
			Detail: []string{
				"KeyId", "Arn", "Description", "KeyState",
				"KeyUsage", "KeySpec", "KeyManager", "Enabled",
				"CreationDate", "Origin", "MultiRegion",
				"EncryptionAlgorithms", "SigningAlgorithms",
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
		"backup": {
			List: []ListColumn{
				{Title: "Plan Name", Path: "BackupPlanName", Width: 32},
				{Title: "Plan ID", Path: "BackupPlanId", Width: 38},
				{Title: "Created", Path: "CreationDate", Width: 22},
				{Title: "Last Execution", Path: "LastExecutionDate", Width: 22},
			},
			Detail: []string{
				"BackupPlanName", "BackupPlanId", "BackupPlanArn",
				"CreationDate", "LastExecutionDate", "DeletionDate",
				"VersionId", "CreatorRequestId", "AdvancedBackupSettings",
			},
		},
	},
}

// DefaultConfig returns a copy of the built-in default configuration.
func DefaultConfig() *ViewsConfig {
	cp := ViewsConfig{
		Views: make(map[string]ViewDef, len(defaultViews.Views)),
	}
	for k, v := range defaultViews.Views {
		cols := make([]ListColumn, len(v.List))
		copy(cols, v.List)
		detail := make([]string, len(v.Detail))
		copy(detail, v.Detail)
		cp.Views[k] = ViewDef{List: cols, Detail: detail}
	}
	return &cp
}

// DefaultViewDef returns the built-in default ViewDef for the given resource
// short name. Returns an empty ViewDef if no default exists for the name.
func DefaultViewDef(shortName string) ViewDef {
	v, ok := defaultViews.Views[shortName]
	if !ok {
		return ViewDef{}
	}
	// Return a copy so callers cannot mutate the package-level defaults.
	cols := make([]ListColumn, len(v.List))
	copy(cols, v.List)
	detail := make([]string, len(v.Detail))
	copy(detail, v.Detail)
	return ViewDef{List: cols, Detail: detail}
}
