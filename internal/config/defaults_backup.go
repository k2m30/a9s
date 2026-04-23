package config

func backupDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"backup": {
			List: []ListColumn{
				{Title: "Plan Name", Path: "BackupPlanName", Width: 32},
				{Title: "Status", Key: "status", Width: 40},
				{Title: "Plan ID", Path: "BackupPlanId", Width: 38},
				{Title: "Created", Path: "CreationDate", Width: 22},
				{Title: "Last Execution", Path: "LastExecutionDate", Width: 22},
			},
			Detail: []DetailField{
				{Path: "BackupPlanName"}, {Path: "BackupPlanId"}, {Path: "BackupPlanArn"},
				{Path: "CreationDate"}, {Path: "LastExecutionDate"}, {Path: "DeletionDate"},
				{Path: "VersionId"}, {Path: "CreatorRequestId"}, {Path: "AdvancedBackupSettings"},
			},
		},
		"ses": {
			List: []ListColumn{
				{Title: "Identity", Path: "IdentityName", Width: 36},
				{Title: "Type", Path: "IdentityType", Width: 16},
				{Title: "Verification", Path: "VerificationStatus", Width: 16},
				{Title: "Sending", Path: "SendingEnabled", Width: 10},
			},
			Detail: []DetailField{
				{Path: "IdentityName"}, {Path: "IdentityType"},
				{Path: "SendingEnabled"}, {Path: "VerificationStatus"},
			},
		},
	}
}
