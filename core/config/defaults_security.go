// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package config

func securityDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"role": {
			Detail: []DetailField{
				{Path: "RoleName"}, {Path: "RoleId"}, {Path: "Arn"}, {Path: "Path"},
				{Path: "CreateDate"}, {Path: "Description"}, {Path: "MaxSessionDuration"},
				{Path: "RoleLastUsed"}, {Path: "PermissionsBoundary"},
				{Path: "AssumeRolePolicyDocument"}, {Path: "Tags"},
			},
		},
		"policy": {
			Detail: []DetailField{
				{Path: "PolicyName"}, {Path: "PolicyId"}, {Path: "Arn"}, {Path: "Path"},
				{Path: "AttachmentCount"}, {Path: "PermissionsBoundaryUsageCount"},
				{Path: "IsAttachable"}, {Path: "DefaultVersionId"},
				{Path: "CreateDate"}, {Path: "UpdateDate"}, {Path: "Description"}, {Path: "Tags"},
				{Path: "Document"},
			},
		},
		"iam-user": {
			Detail: []DetailField{
				{Path: "UserName"}, {Path: "UserId"}, {Path: "Arn"}, {Path: "Path"},
				{Path: "CreateDate"}, {Path: "PasswordLastUsed"},
				{Path: "PermissionsBoundary"}, {Path: "Tags"},
			},
		},
		"iam-group": {
			Detail: []DetailField{
				{Path: "GroupName"}, {Path: "GroupId"}, {Path: "Arn"}, {Path: "Path"}, {Path: "CreateDate"},
			},
		},
		"waf": {
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Id"}, {Path: "ARN"}, {Path: "Description"}, {Path: "LockToken"},
			},
		},
		// Child views for security resources
		"role_policies": {
			Detail: []DetailField{
				{Path: "PolicyName"}, {Path: "PolicyArn"}, {Path: "PolicyType"}, {Path: "Document"},
			},
		},
		"iam_group_members": {
			Detail: []DetailField{
				{Path: "UserName"}, {Path: "UserId"}, {Path: "Arn"}, {Path: "Path"},
				{Path: "CreateDate"}, {Path: "PasswordLastUsed"},
				{Path: "PermissionsBoundary"}, {Path: "Tags"},
			},
		},
	}
}
