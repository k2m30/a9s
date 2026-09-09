// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

func dataDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"glue": {
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Role"}, {Path: "GlueVersion"}, {Path: "WorkerType"},
				{Path: "NumberOfWorkers"}, {Path: "MaxRetries"}, {Path: "Command"},
				{Path: "CreatedOn"}, {Path: "LastModifiedOn"},
			},
		},
		"glue_runs": {
			Detail: []DetailField{
				{Path: "Id"}, {Path: "JobRunState"}, {Path: "StartedOn"}, {Path: "CompletedOn"},
				{Path: "ExecutionTime"}, {Path: "ErrorMessage"}, {Path: "Attempt"}, {Path: "PreviousRunId"},
				{Path: "TriggerName"}, {Path: "JobName"}, {Path: "AllocatedCapacity"}, {Path: "MaxCapacity"},
				{Path: "WorkerType"}, {Path: "NumberOfWorkers"}, {Path: "Timeout"}, {Path: "GlueVersion"},
				{Path: "DPUSeconds"}, {Path: "ExecutionClass"}, {Path: "LogGroupName"},
			},
		},
		"athena": {
			Detail: []DetailField{
				{Path: "Name"}, {Path: "State"}, {Path: "Description"},
				{Path: "EngineVersion"}, {Path: "CreationTime"},
			},
		},
		"mwaa": {
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Status"}, {Path: "AirflowVersion"}, {Path: "EnvironmentClass"},
				{Path: "Schedulers"}, {Path: "MinWorkers"}, {Path: "MaxWorkers"},
				{Path: "MinWebservers"}, {Path: "MaxWebservers"}, {Path: "WebserverAccessMode"},
				{Path: "EndpointManagement"}, {Path: "WeeklyMaintenanceWindowStart"}, {Path: "WebserverUrl"},
				{Path: "Arn"}, {Path: "KmsKey"}, {Path: "SourceBucketArn"}, {Path: "DagS3Path"},
				{Path: "ExecutionRoleArn"}, {Path: "ServiceRoleArn"}, {Path: "CeleryExecutorQueue"},
				{Path: "CreatedAt"}, {Path: "LastUpdate"},
			},
		},
	}
}
