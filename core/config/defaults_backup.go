// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

func backupDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"backup": {
			Detail: []DetailField{
				{Path: "BackupPlanName"}, {Path: "BackupPlanId"}, {Path: "BackupPlanArn"},
				{Path: "CreationDate"}, {Path: "LastExecutionDate"},
				{Path: "VersionId"}, {Path: "CreatorRequestId"}, {Path: "AdvancedBackupSettings"},
			},
		},
		"ses": {
			Detail: []DetailField{
				{Path: "IdentityName"}, {Key: "identity_type", Label: "IdentityType"},
				{Path: "SendingEnabled"}, {Path: "VerificationStatus"},
			},
		},
	}
}
