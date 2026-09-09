// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

func cicdDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"cfn": {
			Detail: []DetailField{
				{Path: "StackName"}, {Path: "StackId"}, {Path: "StackStatus"}, {Path: "DetailedStatus"},
				{Path: "StackStatusReason"}, {Path: "CreationTime"}, {Path: "LastUpdatedTime"},
				{Path: "DeletionTime"}, {Path: "Description"}, {Path: "RoleARN"}, {Path: "Capabilities"},
				{Path: "EnableTerminationProtection"}, {Path: "DriftInformation"},
				{Path: "Parameters"}, {Path: "Outputs"}, {Path: "Tags"},
			},
		},
		"pipeline": {
			Detail: []DetailField{
				{Path: "Name"}, {Path: "PipelineType"}, {Path: "Version"}, {Path: "Created"},
				{Path: "Updated"}, {Path: "ExecutionMode"},
			},
		},
		"cb": {
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Description"}, {Path: "Arn"}, {Path: "Source"},
				{Path: "Environment"}, {Path: "ServiceRole"}, {Path: "Created"}, {Path: "LastModified"},
				{Path: "Cache"}, {Path: "LogsConfig"}, {Path: "ConcurrentBuildLimit"}, {Path: "Tags"},
			},
		},
		"ecr": {
			Detail: []DetailField{
				{Path: "RepositoryName"}, {Path: "RepositoryUri"}, {Path: "RepositoryArn"},
				{Path: "RegistryId"}, {Path: "ImageTagMutability"}, {Path: "ImageScanningConfiguration"},
				{Path: "EncryptionConfiguration"}, {Path: "CreatedAt"},
			},
		},
		"codeartifact": {
			Detail: []DetailField{
				{Path: "Name"}, {Path: "DomainName"}, {Path: "DomainOwner"}, {Path: "Arn"},
				{Path: "Description"}, {Path: "AdministratorAccount"}, {Path: "CreatedTime"},
			},
		},
		"cb_builds": {
			Detail: []DetailField{
				{Path: "Id"}, {Path: "Arn"}, {Path: "BuildNumber"}, {Path: "BuildStatus"}, {Path: "StartTime"}, {Path: "EndTime"},
				{Path: "CurrentPhase"}, {Path: "SourceVersion"}, {Path: "ResolvedSourceVersion"}, {Path: "Initiator"},
				{Path: "Source"}, {Path: "Environment"}, {Path: "Phases"}, {Path: "Logs"}, {Path: "Cache"}, {Path: "VpcConfig"},
				{Path: "ServiceRole"}, {Path: "TimeoutInMinutes"}, {Path: "QueuedTimeoutInMinutes"}, {Path: "BuildBatchArn"},
			},
		},
		"cb_build_logs": {
			Detail: []DetailField{
				{Path: "Timestamp"}, {Path: "IngestionTime"}, {Path: "Message"}, {Path: "EventId"},
			},
		},
		"pipeline_stages": {
			Detail: []DetailField{
				{Path: "StageName"}, {Path: "StageStatus"}, {Path: "ActionName"}, {Path: "ActionStatus"},
				{Path: "LastStatusChange"}, {Path: "ExternalURL"}, {Path: "Token"},
				{Path: "ErrorCode"}, {Path: "ErrorMessage"}, {Path: "RevisionId"}, {Path: "RevisionSummary"},
			},
		},
		"ecr_images": {
			Detail: []DetailField{
				{Path: "ImageDigest"}, {Path: "ImageTags"}, {Path: "ImagePushedAt"},
				{Path: "ImageSizeInBytes"}, {Path: "ImageManifestMediaType"},
				{Path: "ArtifactMediaType"}, {Path: "ImageScanStatus"},
				{Path: "ImageScanFindingsSummary"}, {Path: "LastRecordedPullTime"},
			},
		},
		// Child views for CI/CD resources
		"cfn_events": {
			Detail: []DetailField{
				{Path: "EventId"}, {Path: "StackId"}, {Path: "StackName"}, {Path: "Timestamp"},
				{Path: "LogicalResourceId"}, {Path: "PhysicalResourceId"},
				{Path: "ResourceType"}, {Path: "ResourceStatus"}, {Path: "ResourceStatusReason"},
				{Path: "ResourceProperties"}, {Path: "ClientRequestToken"},
			},
		},
		"cfn_resources": {
			Detail: []DetailField{
				{Path: "LogicalResourceId"}, {Path: "PhysicalResourceId"},
				{Path: "ResourceType"}, {Path: "ResourceStatus"}, {Path: "ResourceStatusReason"},
				{Path: "LastUpdatedTimestamp"}, {Path: "DriftInformation"}, {Path: "ModuleInfo"},
			},
		},
	}
}
