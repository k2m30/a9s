package resource

func backupResourceTypes() []ResourceTypeDef {
	return []ResourceTypeDef{
		{
			Name:          "Backup Plans",
			ShortName:     "backup",
			Aliases:       []string{"backup", "backup-plans"},
			Category:      "BACKUP",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "plan_name", Title: "Plan Name", Width: 32, Sortable: true},
				{Key: "plan_id", Title: "Plan ID", Width: 38, Sortable: true},
				{Key: "creation_date", Title: "Created", Width: 22, Sortable: true},
				{Key: "last_execution", Title: "Last Execution", Width: 22, Sortable: true},
			},
			// Wave 2 enricher surfaces plans whose recent backup jobs have
			// failed — Wave 1 list is declarative config.
			Color: func(_ Resource) Color { return ColorHealthy },
		},
	}
}
