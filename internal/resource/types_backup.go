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
			Color: func(r Resource) Color {
				switch r.Fields["verification_status"] {
				case "Success":
					return ColorHealthy
				case "Pending":
					return ColorWarning
				case "Failed", "TemporaryFailure":
					return ColorBroken
				}
				if r.Fields["sending_enabled"] == "false" {
					return ColorWarning
				}
				// Fallback on generic status for ad-hoc probes / future fetcher fields.
				switch r.Fields["status"] {
				case "Failed", "failed", "FAILED", "TemporaryFailure":
					return ColorBroken
				case "Pending", "pending", "PENDING":
					return ColorWarning
				}
				return ColorHealthy
			},
		},
	}
}
