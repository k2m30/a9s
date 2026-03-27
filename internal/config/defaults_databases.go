package config

func databasesDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
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
		"s3": {
			List: []ListColumn{
				{Title: "Bucket Name", Path: "Name", Width: 36},
				{Title: "Region", Path: "BucketRegion", Width: 14},
				{Title: "Creation Date", Path: "CreationDate", Width: 22},
			},
			Detail: []string{"Name", "BucketArn", "BucketRegion", "CreationDate"},
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
		"ddb": {
			List: []ListColumn{
				{Title: "Table Name", Path: "TableName", Width: 36},
				{Title: "Status", Path: "TableStatus", Width: 12},
				{Title: "Items", Path: "ItemCount", Width: 12},
				{Title: "Size", Path: "", Key: "size_bytes", Width: 14},
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
		"efs": {
			List: []ListColumn{
				{Title: "Name", Path: "Name", Width: 28},
				{Title: "File System ID", Path: "FileSystemId", Width: 22},
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
		// Child views for database/storage resources
		"dbi_events": {
			List: []ListColumn{
				{Title: "Timestamp", Path: "Date", Width: 22},
				{Title: "Category", Key: "event_categories", Width: 18},
				{Title: "Message", Path: "Message", Width: 60},
			},
			Detail: []string{
				"Date", "SourceIdentifier", "SourceType",
				"EventCategories", "SourceArn", "Message",
			},
		},
		"s3_objects": {
			List: []ListColumn{
				{Title: "Key", Path: "Key", Width: 36},
				{Title: "Size", Key: "size", Width: 12},
				{Title: "Storage Class", Path: "StorageClass", Width: 16},
				{Title: "Last Modified", Path: "LastModified", Width: 22},
			},
			Detail: []string{
				"Key", "Size", "LastModified", "StorageClass", "ETag",
				"ChecksumAlgorithm", "ChecksumType", "Owner", "RestoreStatus",
			},
		},
	}
}
