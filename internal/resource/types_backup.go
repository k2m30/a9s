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
			Color: func(_ Resource) Color { return ColorHealthy },
			AlwaysHealthy: true,
		},
		{
			Name:          "SES Identities",
			ShortName:     "ses",
			Aliases:       []string{"ses", "email", "ses-identities"},
			Category:      "BACKUP",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "identity_name", Title: "Identity", Width: 36, Sortable: true},
				{Key: "identity_type", Title: "Type", Width: 16, Sortable: true},
				{Key: "verification_status", Title: "Verification", Width: 16, Sortable: true},
				{Key: "sending_enabled", Title: "Sending", Width: 10, Sortable: true},
			},
			Color: func(_ Resource) Color { return ColorHealthy },
			AlwaysHealthy: true,
		},
	}
}
