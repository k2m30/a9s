package config

func cicdDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"cfn": {
			List: []ListColumn{
				{Title: "Stack Name", Path: "StackName", Width: 36},
				{Title: "Status", Path: "StackStatus", Width: 24},
				{Title: "Drift", Key: "drift_status", Width: 14},
				{Title: "Reason", Path: "StackStatusReason", Width: 32},
				{Title: "Created", Path: "CreationTime", Width: 22},
				{Title: "Updated", Path: "LastUpdatedTime", Width: 22},
				{Title: "Description", Path: "Description", Width: 30},
			},
			Detail: []DetailField{
				{Path: "StackName"}, {Path: "StackId"}, {Path: "StackStatus"}, {Path: "DetailedStatus"},
				{Path: "StackStatusReason"}, {Path: "CreationTime"}, {Path: "LastUpdatedTime"},
				{Path: "DeletionTime"}, {Path: "Description"}, {Path: "RoleARN"}, {Path: "Capabilities"},
				{Path: "EnableTerminationProtection"}, {Path: "DriftInformation"},
				{Path: "Parameters"}, {Path: "Outputs"}, {Path: "Tags"},
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
			Detail: []DetailField{
				{Path: "Name"}, {Path: "PipelineType"}, {Path: "Version"}, {Path: "Created"},
				{Path: "Updated"}, {Path: "ExecutionMode"},
			},
		},
		"cb": {
			List: []ListColumn{
				{Title: "Project Name", Path: "Name", Width: 32},
				{Title: "Source Type", Path: "Source.Type", Width: 14},
				{Title: "Description", Path: "Description", Width: 36},
				{Title: "Last Modified", Path: "LastModified", Width: 22},
			},
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Description"}, {Path: "Arn"}, {Path: "Source"},
				{Path: "Environment"}, {Path: "ServiceRole"}, {Path: "Created"}, {Path: "LastModified"},
				{Path: "Cache"}, {Path: "LogsConfig"}, {Path: "ConcurrentBuildLimit"}, {Path: "Tags"},
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
			Detail: []DetailField{
				{Path: "RepositoryName"}, {Path: "RepositoryUri"}, {Path: "RepositoryArn"},
				{Path: "RegistryId"}, {Path: "ImageTagMutability"}, {Path: "ImageScanningConfiguration"},
				{Path: "EncryptionConfiguration"}, {Path: "CreatedAt"},
			},
		},
		"codeartifact": {
			List: []ListColumn{
				{Title: "Repository", Path: "Name", Width: 28},
				{Title: "Domain", Path: "DomainName", Width: 24},
				{Title: "Description", Path: "Description", Width: 30},
				{Title: "Owner", Path: "DomainOwner", Width: 14},
			},
			Detail: []DetailField{
				{Path: "Name"}, {Path: "DomainName"}, {Path: "DomainOwner"}, {Path: "Arn"},
				{Path: "Description"}, {Path: "AdministratorAccount"}, {Path: "CreatedTime"},
			},
		},
		"cb_builds": {
			List: []ListColumn{
				{Title: "Build #", Key: "build_number", Width: 10},
				{Title: "Status", Key: "build_status", Width: 14},
				{Title: "Start Time", Key: "start_time", Width: 22},
				{Title: "Duration", Key: "duration", Width: 12},
				{Title: "Source Version", Key: "source_version_short", Width: 14},
				{Title: "Initiator", Key: "initiator", Width: 24},
			},
			Detail: []DetailField{
				{Path: "Id"}, {Path: "Arn"}, {Path: "BuildNumber"}, {Path: "BuildStatus"}, {Path: "StartTime"}, {Path: "EndTime"},
				{Path: "CurrentPhase"}, {Path: "SourceVersion"}, {Path: "ResolvedSourceVersion"}, {Path: "Initiator"},
				{Path: "Source"}, {Path: "Environment"}, {Path: "Phases"}, {Path: "Logs"}, {Path: "Cache"}, {Path: "VpcConfig"},
				{Path: "ServiceRole"}, {Path: "TimeoutInMinutes"}, {Path: "QueuedTimeoutInMinutes"}, {Path: "BuildBatchArn"},
			},
		},
		"cb_build_logs": {
			List: []ListColumn{
				{Title: "Timestamp", Key: "timestamp", Width: 22},
				{Title: "Message", Key: "message", Width: 120},
			},
			Detail: []DetailField{
				{Path: "Timestamp"}, {Path: "IngestionTime"}, {Path: "Message"}, {Path: "EventId"},
			},
		},
		"pipeline_stages": {
			List: []ListColumn{
				{Title: "Stage", Key: "stage_name", Width: 20},
				{Title: "Stage Status", Key: "stage_status", Width: 14},
				{Title: "Action", Key: "action_name", Width: 24},
				{Title: "Action Status", Key: "action_status", Width: 14},
				{Title: "Last Changed", Key: "last_change_time", Width: 22},
				{Title: "External URL", Key: "external_url", Width: 40},
			},
			Detail: []DetailField{
				{Path: "StageName"}, {Path: "StageStatus"}, {Path: "ActionName"}, {Path: "ActionStatus"},
				{Path: "LastStatusChange"}, {Path: "ExternalURL"}, {Path: "Token"},
				{Path: "ErrorCode"}, {Path: "ErrorMessage"}, {Path: "RevisionId"}, {Path: "RevisionSummary"},
			},
		},
		"ecr_images": {
			List: []ListColumn{
				{Title: "Tag(s)", Key: "image_tags", Width: 24},
				{Title: "Digest", Key: "digest_short", Width: 16},
				{Title: "Pushed At", Key: "pushed_at", Width: 22},
				{Title: "Size", Key: "image_size", SortPath: "ImageSizeInBytes", Width: 12},
				{Title: "Scan Status", Key: "scan_status", Width: 14},
				{Title: "Findings", Key: "finding_counts", Width: 20},
			},
			Detail: []DetailField{
				{Path: "ImageDigest"}, {Path: "ImageTags"}, {Path: "ImagePushedAt"},
				{Path: "ImageSizeInBytes"}, {Path: "ImageManifestMediaType"},
				{Path: "ArtifactMediaType"}, {Path: "ImageScanStatus"},
				{Path: "ImageScanFindingsSummary"}, {Path: "LastRecordedPullTime"},
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
			Detail: []DetailField{
				{Path: "EventId"}, {Path: "StackId"}, {Path: "StackName"}, {Path: "Timestamp"},
				{Path: "LogicalResourceId"}, {Path: "PhysicalResourceId"},
				{Path: "ResourceType"}, {Path: "ResourceStatus"}, {Path: "ResourceStatusReason"},
				{Path: "ResourceProperties"}, {Path: "ClientRequestToken"},
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
			Detail: []DetailField{
				{Path: "LogicalResourceId"}, {Path: "PhysicalResourceId"},
				{Path: "ResourceType"}, {Path: "ResourceStatus"}, {Path: "ResourceStatusReason"},
				{Path: "LastUpdatedTimestamp"}, {Path: "DriftInformation"}, {Path: "ModuleInfo"},
			},
		},
	}
}
