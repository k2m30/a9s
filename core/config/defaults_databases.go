// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

func databasesDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"dbi": {
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
			Detail: []DetailField{
				{Path: "Name"}, {Path: "BucketArn"}, {Path: "BucketRegion"}, {Path: "CreationDate"},
				{Path: "Policy"},
			},
		},
		"redis": {
			// Each row is one ReplicationGroup from DescribeReplicationGroups;
			// engine_version lives on CacheCluster, not on ReplicationGroup.
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
			Detail: []DetailField{
				{Path: "DBClusterIdentifier"}, {Path: "DBClusterArn"}, {Path: "Engine"}, {Path: "EngineVersion"},
				{Path: "Status"}, {Path: "Endpoint"}, {Path: "ReaderEndpoint"}, {Path: "Port"}, {Path: "StorageEncrypted"},
				{Path: "KmsKeyId"}, {Path: "DeletionProtection"}, {Path: "DBClusterMembers"},
				{Path: "DBSubnetGroup"}, {Path: "VpcSecurityGroups"}, {Path: "BackupRetentionPeriod"},
				{Path: "PreferredMaintenanceWindow"}, {Path: "MasterUsername"},
			},
		},
		"ddb": {
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
			Detail: []DetailField{
				{Path: "DomainName"}, {Path: "DomainId"}, {Path: "ARN"}, {Path: "EngineVersion"},
				{Path: "ClusterConfig"}, {Path: "EBSOptions"}, {Path: "Endpoint"}, {Path: "Endpoints"},
				{Path: "EncryptionAtRestOptions"}, {Path: "DomainEndpointOptions"},
				{Path: "AdvancedSecurityOptions"}, {Path: "VPCOptions"},
				{Path: "Created"}, {Path: "Deleted"},
			},
		},
		"redshift": {
			Detail: []DetailField{
				{Path: "ClusterIdentifier"}, {Path: "ClusterStatus"}, {Path: "NodeType"},
				{Path: "NumberOfNodes"}, {Path: "DBName"}, {Path: "MasterUsername"},
				{Path: "Endpoint"}, {Path: "ClusterCreateTime"}, {Path: "ClusterNamespaceArn"},
				{Path: "VpcId"}, {Path: "AvailabilityZone"},
			},
		},
		"efs": {
			Detail: []DetailField{
				{Path: "FileSystemId"}, {Path: "Name"}, {Path: "LifeCycleState"}, {Path: "PerformanceMode"},
				{Path: "ThroughputMode"}, {Path: "Encrypted"}, {Path: "KmsKeyId"}, {Path: "NumberOfMountTargets"},
				{Path: "FileSystemArn"}, {Path: "OwnerId"}, {Path: "SizeInBytes"}, {Path: "CreationTime"}, {Path: "Tags"},
			},
		},
		"dbi-snap": {
			Detail: []DetailField{
				{Path: "DBSnapshotIdentifier"}, {Path: "DBSnapshotArn"}, {Path: "DBInstanceIdentifier"},
				{Path: "Status"}, {Path: "Engine"}, {Path: "EngineVersion"}, {Path: "SnapshotType"},
				{Path: "SnapshotCreateTime"}, {Path: "AllocatedStorage"}, {Path: "StorageType"},
				{Path: "Encrypted"}, {Path: "KmsKeyId"}, {Path: "AvailabilityZone"},
				{Path: "MasterUsername"}, {Path: "LicenseModel"}, {Path: "Iops"},
				{Path: "PercentProgress"},
				{Path: "SourceDBSnapshotIdentifier"}, {Path: "SourceRegion"},
			},
		},
		"dbc-snap": {
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
		"dbi_events": {
			Detail: []DetailField{
				{Path: "Date"}, {Path: "SourceIdentifier"}, {Path: "SourceType"},
				{Path: "EventCategories"}, {Path: "SourceArn"}, {Path: "Message"},
			},
		},
		"s3_objects": {
			Detail: []DetailField{
				{Path: "Key"}, {Path: "Size"}, {Path: "LastModified"}, {Path: "StorageClass"}, {Path: "ETag"},
				{Path: "ChecksumAlgorithm"}, {Path: "ChecksumType"}, {Path: "Owner"}, {Path: "RestoreStatus"},
			},
		},
	}
}
