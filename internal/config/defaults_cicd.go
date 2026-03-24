package config

func cicdDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
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
		// Child views for CI/CD resources
		"cfn_events": {
			List: []ListColumn{
				{Title: "Timestamp", Key: "timestamp", Width: 22},
				{Title: "Logical ID", Path: "LogicalResourceId", Width: 28},
				{Title: "Type", Path: "ResourceType", Width: 28},
				{Title: "Status", Key: "resource_status", Width: 24},
				{Title: "Reason", Key: "resource_status_reason", Width: 40},
			},
			Detail: []string{
				"EventId", "StackId", "StackName", "Timestamp",
				"LogicalResourceId", "PhysicalResourceId",
				"ResourceType", "ResourceStatus", "ResourceStatusReason",
				"ResourceProperties", "ClientRequestToken",
			},
		},
		"cfn_resources": {
			List: []ListColumn{
				{Title: "Logical ID", Path: "LogicalResourceId", Width: 28},
				{Title: "Physical ID", Path: "PhysicalResourceId", Width: 28},
				{Title: "Type", Path: "ResourceType", Width: 28},
				{Title: "Status", Key: "resource_status", Width: 24},
				{Title: "Drift", Key: "drift_status", Width: 12},
				{Title: "Updated", Key: "last_updated", Width: 22},
			},
			Detail: []string{
				"LogicalResourceId", "PhysicalResourceId",
				"ResourceType", "ResourceStatus", "ResourceStatusReason",
				"LastUpdatedTimestamp", "DriftInformation", "ModuleInfo",
			},
		},
	}
}
