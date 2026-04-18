package config

func securityDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"role": {
			List: []ListColumn{
				{Title: "Role Name", Path: "RoleName", Width: 36},
				{Title: "Last Used", Path: "RoleLastUsed.LastUsedDate", Width: 22},
				{Title: "Trust", Key: "trust_summary", Width: 12},
				{Title: "Path", Path: "Path", Width: 20},
				{Title: "Created", Path: "CreateDate", Width: 22},
				{Title: "Description", Path: "Description", Width: 30},
			},
			Detail: []DetailField{
				{Path: "RoleName"}, {Path: "RoleId"}, {Path: "Arn"}, {Path: "Path"},
				{Path: "CreateDate"}, {Path: "Description"}, {Path: "MaxSessionDuration"},
				{Path: "RoleLastUsed"}, {Path: "PermissionsBoundary"},
				{Path: "AssumeRolePolicyDocument"}, {Path: "Tags"},
			},
		},
		"policy": {
			List: []ListColumn{
				{Title: "Policy Name", Key: "policy_name", Width: 36},
				{Title: "Type", Key: "policy_type", Width: 10},
				{Title: "Attached", Key: "attachment_count", Width: 10},
				{Title: "Path", Key: "path", Width: 20},
				{Title: "Created", Key: "create_date", Width: 22},
			},
			Detail: []DetailField{
				{Path: "PolicyName"}, {Path: "PolicyId"}, {Path: "Arn"}, {Path: "Path"},
				{Path: "AttachmentCount"}, {Path: "PermissionsBoundaryUsageCount"},
				{Path: "IsAttachable"}, {Path: "DefaultVersionId"},
				{Path: "CreateDate"}, {Path: "UpdateDate"}, {Path: "Description"}, {Path: "Tags"},
				{Path: "Document"},
			},
		},
		"iam-user": {
			List: []ListColumn{
				{Title: "User Name", Path: "UserName", Width: 32},
				{Title: "MFA", Key: "mfa", Width: 5},
				{Title: "Risk", Key: "risk", Width: 14},
				{Title: "User ID", Path: "UserId", Width: 22},
				{Title: "Path", Path: "Path", Width: 20},
				{Title: "Created", Path: "CreateDate", Width: 22},
				{Title: "Password Last Used", Path: "PasswordLastUsed", Width: 22},
			},
			Detail: []DetailField{
				{Path: "UserName"}, {Path: "UserId"}, {Path: "Arn"}, {Path: "Path"},
				{Path: "CreateDate"}, {Path: "PasswordLastUsed"},
				{Path: "PermissionsBoundary"}, {Path: "Tags"},
			},
		},
		"iam-group": {
			List: []ListColumn{
				{Title: "Group Name", Path: "GroupName", Width: 32},
				{Title: "Members", Key: "member_count", Width: 9},
				{Title: "Group ID", Path: "GroupId", Width: 22},
				{Title: "Path", Path: "Path", Width: 20},
				{Title: "Created", Path: "CreateDate", Width: 22},
				{Title: "ARN", Path: "Arn", Width: 60},
			},
			Detail: []DetailField{
				{Path: "GroupName"}, {Path: "GroupId"}, {Path: "Arn"}, {Path: "Path"}, {Path: "CreateDate"},
			},
		},
		"waf": {
			List: []ListColumn{
				{Title: "Name", Path: "Name", Width: 28},
				{Title: "ID", Path: "Id", Width: 38},
				{Title: "Description", Path: "Description", Width: 36},
			},
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Id"}, {Path: "ARN"}, {Path: "Description"}, {Path: "LockToken"},
			},
		},
		// Child views for security resources
		"role_policies": {
			List: []ListColumn{
				{Title: "Policy Name", Key: "policy_name", Width: 40},
				{Title: "Policy ARN", Key: "policy_arn", Width: 56},
				{Title: "Type", Key: "policy_type", Width: 10},
			},
			Detail: []DetailField{
				{Path: "PolicyName"}, {Path: "PolicyArn"}, {Path: "PolicyType"}, {Path: "Document"},
			},
		},
		"iam_group_members": {
			List: []ListColumn{
				{Title: "User Name", Key: "user_name", Width: 28},
				{Title: "User ID", Key: "user_id", Width: 24},
				{Title: "Created", Key: "create_date", Width: 22},
				{Title: "Password Last Used", Key: "password_last_used", Width: 22},
			},
			Detail: []DetailField{
				{Path: "UserName"}, {Path: "UserId"}, {Path: "Arn"}, {Path: "Path"},
				{Path: "CreateDate"}, {Path: "PasswordLastUsed"},
				{Path: "PermissionsBoundary"}, {Path: "Tags"},
			},
		},
	}
}
