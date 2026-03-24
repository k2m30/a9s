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
			Detail: []string{
				"Name", "Role", "GlueVersion", "WorkerType",
				"NumberOfWorkers", "MaxRetries", "Command",
				"CreatedOn", "LastModifiedOn",
			},
		},
		"athena": {
			List: []ListColumn{
				{Title: "Workgroup", Path: "Name", Width: 28},
				{Title: "State", Path: "State", Width: 12},
				{Title: "Description", Path: "Description", Width: 30},
				{Title: "Engine", Path: "EngineVersion.EffectiveEngineVersion", Width: 28},
			},
			Detail: []string{
				"Name", "State", "Description",
				"EngineVersion", "CreationTime",
			},
		},
	}
}
