package config

func securityDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"role": {
			List: []ListColumn{
				{Title: "Role Name", Path: "RoleName", Width: 36},
				{Title: "Last Used", Path: "RoleLastUsed.LastUsedDate", Width: 22},
				{Title: "Path", Path: "Path", Width: 20},
				{Title: "Created", Path: "CreateDate", Width: 22},
				{Title: "Description", Path: "Description", Width: 30},
			},
			Detail: []string{
				"RoleName", "RoleId", "Arn", "Path",
				"CreateDate", "Description", "MaxSessionDuration",
				"RoleLastUsed", "PermissionsBoundary",
				"AssumeRolePolicyDocument", "Tags",
			},
		},
		"policy": {
			List: []ListColumn{
				{Title: "Policy Name", Path: "PolicyName", Width: 36},
				{Title: "Policy ID", Path: "PolicyId", Width: 22},
				{Title: "Attached", Path: "AttachmentCount", Width: 10},
				{Title: "Path", Path: "Path", Width: 20},
				{Title: "Created", Path: "CreateDate", Width: 22},
			},
			Detail: []string{
				"PolicyName", "PolicyId", "Arn", "Path",
				"AttachmentCount", "PermissionsBoundaryUsageCount",
				"IsAttachable", "DefaultVersionId",
				"CreateDate", "UpdateDate", "Description", "Tags",
			},
		},
		"iam-user": {
			List: []ListColumn{
				{Title: "User Name", Path: "UserName", Width: 32},
				{Title: "User ID", Path: "UserId", Width: 22},
				{Title: "Path", Path: "Path", Width: 20},
				{Title: "Created", Path: "CreateDate", Width: 22},
				{Title: "Password Last Used", Path: "PasswordLastUsed", Width: 22},
			},
			Detail: []string{
				"UserName", "UserId", "Arn", "Path",
				"CreateDate", "PasswordLastUsed",
				"PermissionsBoundary", "Tags",
			},
		},
		"iam-group": {
			List: []ListColumn{
				{Title: "Group Name", Path: "GroupName", Width: 32},
				{Title: "Group ID", Path: "GroupId", Width: 22},
				{Title: "Path", Path: "Path", Width: 20},
				{Title: "Created", Path: "CreateDate", Width: 22},
				{Title: "ARN", Path: "Arn", Width: 60},
			},
			Detail: []string{
				"GroupName", "GroupId", "Arn", "Path", "CreateDate",
			},
		},
		"waf": {
			List: []ListColumn{
				{Title: "Name", Path: "Name", Width: 28},
				{Title: "ID", Path: "Id", Width: 38},
				{Title: "Description", Path: "Description", Width: 36},
			},
			Detail: []string{
				"Name", "Id", "ARN", "Description", "LockToken",
			},
		},
	}
}
