package resource

func dataResourceTypes() []ResourceTypeDef {
	return []ResourceTypeDef{
		{
			Name:      "Glue Jobs",
			ShortName: "glue",
			Aliases:   []string{"glue", "glue-jobs"},
			Category:  "DATA & ANALYTICS",
			Columns: []Column{
				{Key: "job_name", Title: "Job Name", Width: 32, Sortable: true},
				{Key: "glue_version", Title: "Version", Width: 10, Sortable: true},
				{Key: "worker_type", Title: "Worker Type", Width: 14, Sortable: true},
				{Key: "num_workers", Title: "Workers", Width: 9, Sortable: true},
				{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
			},
		},
		{
			Name:      "Athena Workgroups",
			ShortName: "athena",
			Aliases:   []string{"athena", "workgroups"},
			Category:  "DATA & ANALYTICS",
			Columns: []Column{
				{Key: "workgroup_name", Title: "Workgroup", Width: 28, Sortable: true},
				{Key: "state", Title: "State", Width: 12, Sortable: true},
				{Key: "description", Title: "Description", Width: 30, Sortable: false},
				{Key: "engine_version", Title: "Engine", Width: 28, Sortable: true},
			},
		},
	}
}
