package config

// defaultViews holds the built-in view definitions for all supported resource types.
// Paths use Go field names from AWS SDK v2 structs. ExtractValue matches case-insensitively.
var defaultViews = ViewsConfig{
	Views: map[string]ViewDef{
		"s3_objects": {
			List: []ListColumn{
				{Title: "Key", Path: "Key", Width: 50},
				{Title: "Size", Path: "Size", Width: 12},
				{Title: "Last Modified", Path: "LastModified", Width: 22},
				{Title: "Storage Class", Path: "StorageClass", Width: 16},
			},
			Detail: []string{
				"Key", "Size", "LastModified", "StorageClass", "ETag",
				"ChecksumAlgorithm", "ChecksumType", "Owner", "RestoreStatus",
			},
		},
		"s3": {
			List: []ListColumn{
				{Title: "Bucket Name", Path: "Name", Width: 40},
				{Title: "Creation Date", Path: "CreationDate", Width: 22},
			},
			Detail: []string{"Name", "CreationDate"},
		},
		"ec2": {
			List: []ListColumn{
				{Title: "Instance ID", Path: "InstanceId", Width: 20},
				{Title: "Name", Path: "Tags", Width: 28},
				{Title: "State", Path: "State.Name", Width: 12},
				{Title: "Type", Path: "InstanceType", Width: 14},
				{Title: "Private IP", Path: "PrivateIpAddress", Width: 16},
				{Title: "Public IP", Path: "PublicIpAddress", Width: 16},
				{Title: "Launch Time", Path: "LaunchTime", Width: 22},
			},
			Detail: []string{
				"InstanceId", "State", "InstanceType", "ImageId",
				"VpcId", "SubnetId", "PrivateIpAddress", "PublicIpAddress",
				"SecurityGroups", "LaunchTime", "Architecture", "Platform", "Tags",
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
				"DBInstanceIdentifier", "Engine", "EngineVersion", "DBInstanceStatus",
				"DBInstanceClass", "Endpoint", "MultiAZ", "AllocatedStorage",
				"StorageType", "AvailabilityZone",
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
				"CacheClusterId", "Engine", "EngineVersion", "CacheClusterStatus",
				"CacheNodeType", "NumCacheNodes", "ConfigurationEndpoint",
				"PreferredAvailabilityZone",
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
				"DBClusterIdentifier", "Engine", "EngineVersion", "Status",
				"Endpoint", "ReaderEndpoint", "Port", "StorageEncrypted",
				"DBClusterMembers",
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
				"RotationEnabled", "ARN", "KmsKeyId", "Tags",
			},
		},
		"vpc": {
			List: []ListColumn{
				{Title: "VPC ID", Path: "VpcId", Width: 24},
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
				{Title: "Name", Path: "Tags", Width: 28},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "CIDR Block", Path: "CidrBlock", Width: 18},
				{Title: "AZ", Path: "AvailabilityZone", Width: 14},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Available IPs", Path: "AvailableIpAddressCount", Width: 14},
			},
			Detail: []string{
				"SubnetId", "VpcId", "CidrBlock", "AvailabilityZone",
				"State", "AvailableIpAddressCount", "MapPublicIpOnLaunch",
				"DefaultForAz", "SubnetArn", "OwnerId", "Tags",
			},
		},
		"rtb": {
			List: []ListColumn{
				{Title: "Route Table ID", Path: "RouteTableId", Width: 26},
				{Title: "Name", Path: "Tags", Width: 28},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Routes", Path: "Routes", Width: 8},
				{Title: "Assoc.", Path: "Associations", Width: 8},
			},
			Detail: []string{
				"RouteTableId", "VpcId", "Routes", "Associations",
				"OwnerId", "Tags",
			},
		},
		"nat": {
			List: []ListColumn{
				{Title: "NAT Gateway ID", Path: "NatGatewayId", Width: 26},
				{Title: "Name", Path: "Tags", Width: 24},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
				{Title: "Subnet ID", Path: "SubnetId", Width: 26},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Public IP", Path: "NatGatewayAddresses", Width: 16},
			},
			Detail: []string{
				"NatGatewayId", "VpcId", "SubnetId", "State",
				"ConnectivityType", "NatGatewayAddresses", "CreateTime", "Tags",
			},
		},
		"igw": {
			List: []ListColumn{
				{Title: "IGW ID", Path: "InternetGatewayId", Width: 26},
				{Title: "Name", Path: "Tags", Width: 28},
				{Title: "VPC ID", Path: "Attachments", Width: 24},
				{Title: "State", Path: "Attachments", Width: 12},
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
				{Title: "Handler", Path: "Handler", Width: 30},
				{Title: "Last Modified", Path: "LastModified", Width: 22},
			},
			Detail: []string{
				"FunctionName", "FunctionArn", "Runtime", "Handler",
				"MemorySize", "Timeout", "CodeSize", "Description",
				"Role", "PackageType", "Architectures", "LastModified",
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
				"MetricName", "Namespace", "Statistic", "Period",
				"EvaluationPeriods", "Threshold", "ComparisonOperator",
				"AlarmDescription", "AlarmActions",
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
				{Title: "Queue Name", Path: "QueueUrl", Width: 36},
				{Title: "Messages", Path: "Attributes", Width: 10},
				{Title: "In Flight", Path: "Attributes", Width: 10},
				{Title: "Delay", Path: "Attributes", Width: 8},
				{Title: "Queue URL", Path: "QueueUrl", Width: 50},
			},
			Detail: []string{
				"QueueUrl", "Attributes",
			},
		},
		"elb": {
			List: []ListColumn{
				{Title: "Name", Path: "LoadBalancerName", Width: 32},
				{Title: "DNS Name", Path: "DNSName", Width: 48},
				{Title: "Type", Path: "Type", Width: 12},
				{Title: "Scheme", Path: "Scheme", Width: 14},
				{Title: "State", Path: "State.Code", Width: 12},
				{Title: "VPC ID", Path: "VpcId", Width: 24},
			},
			Detail: []string{
				"LoadBalancerName", "LoadBalancerArn", "DNSName", "Type",
				"Scheme", "State", "VpcId", "AvailabilityZones",
				"SecurityGroups", "IpAddressType", "CreatedTime",
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
				"VpcId", "TargetType", "HealthCheckPath", "HealthCheckPort",
				"HealthCheckProtocol", "HealthCheckIntervalSeconds",
				"HealthyThresholdCount", "UnhealthyThresholdCount",
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
			},
		},
		"ecs-svc": {
			List: []ListColumn{
				{Title: "Service Name", Path: "ServiceName", Width: 32},
				{Title: "Cluster", Path: "ClusterArn", Width: 24},
				{Title: "Status", Path: "Status", Width: 12},
				{Title: "Desired", Path: "DesiredCount", Width: 9},
				{Title: "Running", Path: "RunningCount", Width: 9},
				{Title: "Launch Type", Path: "LaunchType", Width: 12},
			},
			Detail: []string{
				"ServiceName", "ServiceArn", "ClusterArn", "Status",
				"DesiredCount", "RunningCount", "LaunchType",
				"TaskDefinition", "RoleArn", "CreatedAt",
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
				"StackName", "StackId", "StackStatus", "StackStatusReason",
				"CreationTime", "LastUpdatedTime", "Description",
				"RoleARN", "DriftInformation", "Parameters", "Outputs",
			},
		},
		"role": {
			List: []ListColumn{
				{Title: "Role Name", Path: "RoleName", Width: 36},
				{Title: "Role ID", Path: "RoleId", Width: 22},
				{Title: "Path", Path: "Path", Width: 20},
				{Title: "Created", Path: "CreateDate", Width: 22},
				{Title: "Description", Path: "Description", Width: 30},
			},
			Detail: []string{
				"RoleName", "RoleId", "Arn", "Path",
				"CreateDate", "Description", "MaxSessionDuration",
				"AssumeRolePolicyDocument", "Tags",
			},
		},
		"logs": {
			List: []ListColumn{
				{Title: "Log Group Name", Path: "LogGroupName", Width: 48},
				{Title: "Size (bytes)", Path: "StoredBytes", Width: 14},
				{Title: "Retention", Path: "RetentionInDays", Width: 10},
				{Title: "Created", Path: "CreationTime", Width: 16},
			},
			Detail: []string{
				"LogGroupName", "Arn", "StoredBytes", "RetentionInDays",
				"CreationTime", "KmsKeyId", "DataProtectionStatus",
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
				"Tier", "DataType",
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
				"CreationDateTime", "KeySchema", "AttributeDefinitions",
			},
		},
		"eip": {
			List: []ListColumn{
				{Title: "Allocation ID", Path: "AllocationId", Width: 26},
				{Title: "Public IP", Path: "PublicIp", Width: 16},
				{Title: "Association", Path: "AssociationId", Width: 26},
				{Title: "Instance", Path: "InstanceId", Width: 20},
				{Title: "Domain", Path: "Domain", Width: 8},
			},
			Detail: []string{
				"AllocationId", "PublicIp", "AssociationId", "InstanceId",
				"Domain", "PrivateIpAddress", "NetworkInterfaceId", "Tags",
			},
		},
		"acm": {
			List: []ListColumn{
				{Title: "Domain Name", Path: "DomainName", Width: 40},
				{Title: "Status", Path: "Status", Width: 14},
				{Title: "Type", Path: "Type", Width: 14},
				{Title: "Expires", Path: "NotAfter", Width: 22},
				{Title: "In Use", Path: "InUseBy", Width: 8},
			},
			Detail: []string{
				"DomainName", "CertificateArn", "Status", "Type",
				"NotBefore", "NotAfter", "InUseBy", "CreatedAt",
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
				"HealthCheckType", "CreatedTime", "Tags",
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
