package config

func databasesDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"dbi": {
			List: []ListColumn{
				{Title: "DB Identifier", Path: "DBInstanceIdentifier", Width: 28},
				{Title: "Engine", Path: "Engine", Width: 12},
				{Title: "Version", Path: "EngineVersion", Width: 10},
				{Title: "Status", Key: "status", SortPath: "DBInstanceStatus", Width: 28},
				{Title: "Class", Path: "DBInstanceClass", Width: 16},
				{Title: "Endpoint", Path: "Endpoint.Address", Width: 40},
				{Title: "Multi-AZ", Path: "MultiAZ", Width: 10},
			},
			Detail: []DetailField{
				{Path: "DBInstanceIdentifier"}, {Path: "DBInstanceArn"}, {Path: "Engine"}, {Path: "EngineVersion"},
				{Path: "DBInstanceStatus"}, {Path: "DBInstanceClass"}, {Path: "Endpoint"}, {Path: "MultiAZ"},
				{Path: "AllocatedStorage"}, {Path: "StorageType"}, {Path: "Iops"}, {Path: "StorageEncrypted"},
				{Path: "KmsKeyId"}, {Path: "AvailabilityZone"}, {Path: "PubliclyAccessible"},
				{Path: "DBSubnetGroup"}, {Path: "VpcSecurityGroups"}, {Path: "BackupRetentionPeriod"},
				{Path: "PreferredMaintenanceWindow"}, {Path: "PreferredBackupWindow"},
				{Path: "DeletionProtection"}, {Path: "MasterUsername"},
				{Path: "PerformanceInsightsEnabled"}, {Path: "TagList"},
			},
		},
		"s3": {
			List: []ListColumn{
				{Title: "Bucket Name", Path: "Name", Width: 36},
				{Title: "Region", Path: "BucketRegion", Width: 14},
				{Title: "Creation Date", Path: "CreationDate", Width: 22},
				// Status column: key-only lookup. NO Path fallback — the extractor
			// would otherwise resolve to RawStruct.Name on every healthy row,
			// rendering the bucket name in the Status cell. Spec §4 S4: healthy
			// rows render blank. Fields["status"] is populated by the Wave-2
			// enricher; absence means blank.
			{Title: "Status", Key: "status", Width: 32},
			},
			Detail: []DetailField{
				{Path: "Name"}, {Path: "BucketArn"}, {Path: "BucketRegion"}, {Path: "CreationDate"},
			},
		},
		"redis": {
			// List API: DescribeReplicationGroups. Each row = one ReplicationGroup.
			// engine_version is not a field on ReplicationGroup (only on CacheCluster);
			// it is omitted to avoid unnecessary DescribeCacheClusters traffic.
			List: []ListColumn{
				// Cluster ID is Path-based (reads ReplicationGroupId from RawStruct)
				// so a regression that populates Fields["cluster_id"] with a stale
				// value cannot surface in the UI — matches the dbc / dbi convention.
				{Title: "Cluster ID", Path: "ReplicationGroupId", Width: 28},
				{Title: "Node Type", Path: "CacheNodeType", Width: 18},
				{Title: "Status", Key: "status", SortPath: "Status", Width: 32},
				{Title: "Nodes", Key: "nodes", Width: 8},
				{Title: "Endpoint", Path: "ConfigurationEndpoint.Address", Width: 40},
			},
			Detail: []DetailField{
				{Path: "ReplicationGroupId"}, {Path: "ARN"}, {Path: "Description"},
				{Path: "Status"}, {Path: "CacheNodeType"}, {Path: "MemberClusters"},
				{Path: "ConfigurationEndpoint"}, {Path: "MultiAZ"}, {Path: "AutomaticFailover"},
				{Path: "KmsKeyId"}, {Path: "AtRestEncryptionEnabled"}, {Path: "TransitEncryptionEnabled"},
				{Path: "AuthTokenEnabled"}, {Path: "SnapshotRetentionLimit"}, {Path: "SnapshotWindow"},
				{Path: "LogDeliveryConfigurations"},
			},
		},
		"dbc": {
			List: []ListColumn{
				{Title: "Cluster ID", Path: "DBClusterIdentifier", Width: 28},
				{Title: "Version", Path: "EngineVersion", Width: 10},
				{Title: "Status", Key: "status", SortPath: "Status", Width: 32},
				{Title: "Instances", Path: "DBClusterMembers", Width: 10},
				{Title: "Endpoint", Path: "Endpoint", Width: 48},
			},
			Detail: []DetailField{
				{Path: "DBClusterIdentifier"}, {Path: "DBClusterArn"}, {Path: "Engine"}, {Path: "EngineVersion"},
				{Path: "Status"}, {Path: "Endpoint"}, {Path: "ReaderEndpoint"}, {Path: "Port"}, {Path: "StorageEncrypted"},
				{Path: "KmsKeyId"}, {Path: "DeletionProtection"}, {Path: "DBClusterMembers"},
				{Path: "DBSubnetGroup"}, {Path: "VpcSecurityGroups"}, {Path: "BackupRetentionPeriod"},
				{Path: "PreferredMaintenanceWindow"}, {Path: "MasterUsername"},
			},
		},
		"ddb": {
			List: []ListColumn{
				{Title: "Table Name", Path: "TableName", Width: 36},
				{Title: "Status", Key: "status", Width: 32},
				{Title: "Items", Path: "ItemCount", Width: 12},
				{Title: "Size", Key: "size_bytes", SortPath: "TableSizeBytes", Width: 14},
				{Title: "Billing", Path: "BillingModeSummary.BillingMode", Width: 16},
			},
			Detail: []DetailField{
				{Path: "TableName"}, {Path: "TableArn"}, {Path: "TableId"}, {Path: "TableStatus"},
				{Path: "ItemCount"}, {Path: "TableSizeBytes"}, {Path: "BillingModeSummary"},
				{Path: "GlobalSecondaryIndexes"}, {Path: "LocalSecondaryIndexes"},
				{Path: "ProvisionedThroughput"}, {Path: "DeletionProtectionEnabled"},
				{Path: "StreamSpecification"}, {Path: "SSEDescription"},
				{Path: "CreationDateTime"}, {Path: "KeySchema"}, {Path: "AttributeDefinitions"},
			},
		},
		"opensearch": {
			List: []ListColumn{
				{Title: "Domain Name", Path: "DomainName", Width: 28},
				{Title: "Status", Key: "status", Width: 12},
				{Title: "Processing", Key: "domain_processing_status", Width: 14},
				{Title: "Engine Version", Path: "EngineVersion", Width: 16},
				{Title: "Instance Type", Path: "ClusterConfig.InstanceType", Width: 22},
				{Title: "Instances", Path: "ClusterConfig.InstanceCount", Width: 10},
				{Title: "Endpoint", Path: "Endpoint", Width: 48},
			},
			Detail: []DetailField{
				{Path: "DomainName"}, {Path: "DomainId"}, {Path: "ARN"}, {Path: "EngineVersion"},
				{Path: "ClusterConfig"}, {Path: "EBSOptions"}, {Path: "Endpoint"}, {Path: "Endpoints"},
				{Path: "EncryptionAtRestOptions"}, {Path: "DomainEndpointOptions"},
				{Path: "AdvancedSecurityOptions"}, {Path: "Created"}, {Path: "Deleted"},
				// {Key: "cluster_health", Label: "Cluster Health"} — re-add when CloudWatch Wave-3 enricher populates this field. Today the fetcher only writes domain_processing_status.
			},
		},
		"redshift": {
			List: []ListColumn{
				{Title: "Cluster ID", Path: "ClusterIdentifier", Width: 28},
				{Title: "Status", Path: "ClusterStatus", Width: 14},
				{Title: "Pending", Path: "PendingModifiedValues.NodeType", Width: 14},
				{Title: "Node Type", Path: "NodeType", Width: 16},
				{Title: "Nodes", Path: "NumberOfNodes", Width: 7},
				{Title: "Database", Path: "DBName", Width: 16},
				{Title: "Endpoint", Path: "Endpoint.Address", Width: 44},
			},
			Detail: []DetailField{
				{Path: "ClusterIdentifier"}, {Path: "ClusterStatus"}, {Path: "NodeType"},
				{Path: "NumberOfNodes"}, {Path: "DBName"}, {Path: "MasterUsername"},
				{Path: "Endpoint"}, {Path: "ClusterCreateTime"}, {Path: "ClusterNamespaceArn"},
				{Path: "AvailabilityZone"},
			},
		},
		"efs": {
			List: []ListColumn{
				{Title: "Name", Path: "Name", Width: 28},
				{Title: "File System ID", Path: "FileSystemId", Width: 22},
				{Title: "Status", Key: "status", Width: 24},
				{Title: "Perf Mode", Path: "PerformanceMode", Width: 16},
				{Title: "Encrypted", Path: "Encrypted", Width: 10},
				{Title: "Mounts", Path: "NumberOfMountTargets", Width: 8},
			},
			Detail: []DetailField{
				{Path: "FileSystemId"}, {Path: "Name"}, {Path: "LifeCycleState"}, {Path: "PerformanceMode"},
				{Path: "ThroughputMode"}, {Path: "Encrypted"}, {Path: "NumberOfMountTargets"},
				{Path: "FileSystemArn"}, {Path: "OwnerId"}, {Path: "SizeInBytes"}, {Path: "CreationTime"}, {Path: "Tags"},
			},
		},
		"rds-snap": {
			List: []ListColumn{
				{Title: "Snapshot ID", Path: "DBSnapshotIdentifier", Width: 36},
				{Title: "DB Instance", Path: "DBInstanceIdentifier", Width: 28},
				{Title: "Status", Path: "Status", Width: 12},
				{Title: "Encrypted", Path: "Encrypted", Width: 10},
				{Title: "Engine", Path: "Engine", Width: 12},
				{Title: "Type", Path: "SnapshotType", Width: 12},
				{Title: "Created", Path: "SnapshotCreateTime", Width: 22},
			},
			Detail: []DetailField{
				{Path: "DBSnapshotIdentifier"}, {Path: "DBSnapshotArn"}, {Path: "DBInstanceIdentifier"},
				{Path: "Status"}, {Path: "Engine"}, {Path: "EngineVersion"}, {Path: "SnapshotType"},
				{Path: "SnapshotCreateTime"}, {Path: "AllocatedStorage"}, {Path: "StorageType"},
				{Path: "Encrypted"}, {Path: "KmsKeyId"}, {Path: "AvailabilityZone"},
				{Path: "MasterUsername"}, {Path: "LicenseModel"}, {Path: "Iops"},
				{Path: "PercentProgress"}, {Path: "SourceRegion"},
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
			Detail: []DetailField{
				{Path: "DBClusterSnapshotIdentifier"}, {Path: "DBClusterSnapshotArn"},
				{Path: "DBClusterIdentifier"}, {Path: "Status"}, {Path: "Engine"}, {Path: "EngineVersion"},
				{Path: "SnapshotType"}, {Path: "SnapshotCreateTime"}, {Path: "ClusterCreateTime"},
				{Path: "MasterUsername"}, {Path: "Port"}, {Path: "VpcId"},
				{Path: "StorageEncrypted"}, {Path: "KmsKeyId"}, {Path: "StorageType"},
				{Path: "PercentProgress"}, {Path: "SourceDBClusterSnapshotArn"},
				{Path: "AvailabilityZones"},
			},
		},
		// Child views for database/storage resources
		"dbi_events": {
			List: []ListColumn{
				{Title: "Timestamp", Path: "Date", Width: 22},
				{Title: "Category", Key: "event_categories", Width: 18},
				{Title: "Message", Path: "Message", Width: 60},
			},
			Detail: []DetailField{
				{Path: "Date"}, {Path: "SourceIdentifier"}, {Path: "SourceType"},
				{Path: "EventCategories"}, {Path: "SourceArn"}, {Path: "Message"},
			},
		},
		"s3_objects": {
			List: []ListColumn{
				{Title: "Key", Path: "Key", Width: 36},
				{Title: "Size", Key: "size", SortPath: "Size", Width: 12},
				{Title: "Storage Class", Path: "StorageClass", Width: 16},
				{Title: "Last Modified", Path: "LastModified", Width: 22},
			},
			Detail: []DetailField{
				{Path: "Key"}, {Path: "Size"}, {Path: "LastModified"}, {Path: "StorageClass"}, {Path: "ETag"},
				{Path: "ChecksumAlgorithm"}, {Path: "ChecksumType"}, {Path: "Owner"}, {Path: "RestoreStatus"},
			},
		},
	}
}
