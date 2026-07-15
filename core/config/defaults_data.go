// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

func dataDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"glue": {
			List: []ListColumn{
				{Title: "Job Name", Path: "Name", Width: 32},
				{Title: "Status", Width: 12},
				{Title: "Last Run", Key: "last_run", Width: 14},
				{Title: "Version", Path: "GlueVersion", Width: 10},
				{Title: "Worker Type", Path: "WorkerType", Width: 14},
				{Title: "Workers", Path: "NumberOfWorkers", Width: 9},
				{Title: "Last Modified", Path: "LastModifiedOn", Width: 22},
			},
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Role"}, {Path: "GlueVersion"}, {Path: "WorkerType"},
				{Path: "NumberOfWorkers"}, {Path: "MaxRetries"}, {Path: "Command"},
				{Path: "CreatedOn"}, {Path: "LastModifiedOn"},
			},
		},
		"glue_runs": {
			List: []ListColumn{
				{Title: "Run ID", Key: "run_id_short", Width: 12},
				{Title: "State", Path: "JobRunState", Width: 12},
				{Title: "Started", Path: "StartedOn", Width: 22},
				{Title: "Execution Time", Key: "execution_time_human", Width: 14},
				{Title: "Error Message", Path: "ErrorMessage", Width: 44},
				{Title: "DPU Hours", Key: "dpu_hours", Width: 10},
			},
			Detail: []DetailField{
				{Path: "Id"}, {Path: "JobRunState"}, {Path: "StartedOn"}, {Path: "CompletedOn"},
				{Path: "ExecutionTime"}, {Path: "ErrorMessage"}, {Path: "Attempt"}, {Path: "PreviousRunId"},
				{Path: "TriggerName"}, {Path: "JobName"}, {Path: "AllocatedCapacity"}, {Path: "MaxCapacity"},
				{Path: "WorkerType"}, {Path: "NumberOfWorkers"}, {Path: "Timeout"}, {Path: "GlueVersion"},
				{Path: "DPUSeconds"}, {Path: "ExecutionClass"}, {Path: "LogGroupName"},
			},
		},
		"athena": {
			List: []ListColumn{
				{Title: "Workgroup", Path: "Name", Width: 28},
				{Title: "Status", Path: "State", Width: 12},
				{Title: "Cost Cap", Path: "Configuration.BytesScannedCutoffPerQuery", Width: 12},
				{Title: "Description", Path: "Description", Width: 30},
				{Title: "Engine", Path: "EngineVersion.EffectiveEngineVersion", Width: 28},
			},
			Detail: []DetailField{
				{Path: "Name"}, {Path: "State"}, {Path: "Description"},
				{Path: "EngineVersion"}, {Path: "CreationTime"},
			},
		},
		"mwaa": {
			List: []ListColumn{
				{Title: "Name", Path: "Name", Width: 32},
				{Title: "Status", Key: "status", Width: 32},
				{Title: "Airflow", Path: "AirflowVersion", Width: 10},
				{Title: "Class", Path: "EnvironmentClass", Width: 14},
				{Title: "Workers", Path: "MaxWorkers", Width: 9},
				{Title: "Schedulers", Path: "Schedulers", Width: 10},
				{Title: "Access", Path: "WebserverAccessMode", Width: 16, Humanize: true},
				{Title: "Created", Path: "CreatedAt", Width: 22},
			},
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
