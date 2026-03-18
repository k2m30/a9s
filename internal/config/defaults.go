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
