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
				{Key: "status", Title: "Status", Width: 36, Sortable: true},
			},
			Color: func(r Resource) Color {
				// Color is driven by the top Wave-1 phrase stored in r.Status.
				// Strip the `(+N)` suffix before matching so "verification failed (+1)"
				// maps to Broken just like "verification failed".
				// Wave-2 account-level phrases ("account PROBATION", "account SHUTDOWN",
				// "quota 80%+ used") arrive in r.Status via FieldUpdates;
				// PROBATION/SHUTDOWN map to Broken, quota stays Healthy (~ finding).
				phrase := StripFindingSuffix(r.Status)
				switch phrase {
				case "verification failed", "verify: temp failure", "verification not started",
					"account SHUTDOWN", "account PROBATION":
					return ColorBroken
				case "pending verification", "sending disabled":
					return ColorWarning
				}
				// Fallback: check verification_status field for backward-compatibility
				// with resources that carry the raw AWS enum value in Fields but have
				// not yet had their Status recomputed by the fetcher.
				switch r.Fields["verification_status"] {
				case "FAILED", "Failed", "TemporaryFailure", "TEMPORARY_FAILURE", "NOT_STARTED":
					return ColorBroken
				case "PENDING", "Pending":
					return ColorWarning
				}
				if r.Fields["sending_enabled"] == "false" {
					return ColorWarning
				}
				// Healthy or informational (quota 80%+ used): green row.
				return ColorHealthy
			},
		},
	}
}
