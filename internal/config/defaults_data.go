package config

func dataDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"glue": {
			List: []ListColumn{
				{Title: "Job Name", Path: "Name", Width: 32},
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
				{Title: "State", Path: "State", Width: 12},
				{Title: "Description", Path: "Description", Width: 30},
				{Title: "Engine", Path: "EngineVersion.EffectiveEngineVersion", Width: 28},
			},
			Detail: []DetailField{
				{Path: "Name"}, {Path: "State"}, {Path: "Description"},
				{Path: "EngineVersion"}, {Path: "CreationTime"},
			},
		},
	}
}
