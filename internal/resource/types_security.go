package resource

func securityResourceTypes() []ResourceTypeDef {
	return []ResourceTypeDef{
		{
			Name:      "IAM Roles",
			ShortName: "role",
			Aliases:   []string{"role", "roles", "iam-roles"},
			Category:  "SECURITY & IAM",
			Columns: []Column{
				{Key: "role_name", Title: "Role Name", Width: 36, Sortable: true},
				{Key: "role_id", Title: "Role ID", Width: 22, Sortable: true},
				{Key: "path", Title: "Path", Width: 20, Sortable: true},
				{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
				{Key: "description", Title: "Description", Width: 30, Sortable: false},
			},
		},
		{
			Name:      "IAM Policies",
			ShortName: "policy",
			Aliases:   []string{"policy", "policies", "iam-policies"},
			Category:  "SECURITY & IAM",
			Columns: []Column{
				{Key: "policy_name", Title: "Policy Name", Width: 36, Sortable: true},
				{Key: "policy_id", Title: "Policy ID", Width: 22, Sortable: true},
				{Key: "attachment_count", Title: "Attached", Width: 10, Sortable: true},
				{Key: "path", Title: "Path", Width: 20, Sortable: true},
				{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
			},
		},
		{
			Name:      "IAM Users",
			ShortName: "iam-user",
			Aliases:   []string{"iam-user", "iam-users", "users"},
			Category:  "SECURITY & IAM",
			Columns: []Column{
				{Key: "user_name", Title: "User Name", Width: 32, Sortable: true},
				{Key: "user_id", Title: "User ID", Width: 22, Sortable: true},
				{Key: "path", Title: "Path", Width: 20, Sortable: true},
				{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
				{Key: "password_last_used", Title: "Password Last Used", Width: 22, Sortable: true},
			},
		},
		{
			Name:      "IAM Groups",
			ShortName: "iam-group",
			Aliases:   []string{"iam-group", "iam-groups", "groups"},
			Category:  "SECURITY & IAM",
			Columns: []Column{
				{Key: "group_name", Title: "Group Name", Width: 32, Sortable: true},
				{Key: "group_id", Title: "Group ID", Width: 22, Sortable: true},
				{Key: "path", Title: "Path", Width: 20, Sortable: true},
				{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
				{Key: "arn", Title: "ARN", Width: 60, Sortable: true},
			},
		},
		{
			Name:      "WAF Web ACLs",
			ShortName: "waf",
			Aliases:   []string{"waf", "webacl", "web-acl"},
			Category:  "SECURITY & IAM",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 28, Sortable: true},
				{Key: "id", Title: "ID", Width: 38, Sortable: true},
				{Key: "description", Title: "Description", Width: 36, Sortable: false},
			},
		},
	}
}
