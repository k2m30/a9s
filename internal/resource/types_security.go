package resource

import "strings"

func securityResourceTypes() []ResourceTypeDef {
	return []ResourceTypeDef{
		{
			Name:          "IAM Roles",
			ShortName:     "role",
			Aliases:       []string{"role", "roles", "iam-roles", "iam_roles"},
			Category:      "SECURITY & IAM",
			CloudTrailKey: "Username:Name",
			Columns: []Column{
				{Key: "role_name", Title: "Role Name", Width: 36, Sortable: true},
				{Key: "role_id", Title: "Role ID", Width: 22, Sortable: true},
				{Key: "path", Title: "Path", Width: 20, Sortable: true},
				{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
				{Key: "description", Title: "Description", Width: 30, Sortable: false},
			},
			Color: func(r Resource) Color {
				doc := r.Fields["assume_role_policy_document"]
				if doc != "" &&
					(strings.Contains(doc, `"Principal":"*"`) || strings.Contains(doc, `"Principal": "*"`)) {
					return ColorBroken
				}
				return ColorHealthy
			},
			Children: []ChildViewDef{
				{
					ChildType:      "role_policies",
					Key:            "enter",
					ContextKeys:    map[string]string{"role_name": "ID"},
					DisplayNameKey: "role_name",
				},
			},
		},
		{
			Name:          "IAM Policies",
			ShortName:     "policy",
			Aliases:       []string{"policy", "policies", "iam-policies", "iam_policies"},
			Category:      "SECURITY & IAM",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "policy_name", Title: "Policy Name", Width: 36, Sortable: true},
				{Key: "policy_type", Title: "Type", Width: 10, Sortable: true},
				{Key: "attachment_count", Title: "Attached", Width: 10, Sortable: true},
				{Key: "path", Title: "Path", Width: 20, Sortable: true},
				{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
			},
			Color: func(r Resource) Color {
				if r.Fields["attachment_count"] == "0" && r.Fields["is_attachable"] == "true" {
					return ColorWarning
				}
				return ColorHealthy
			},
		},
		{
			Name:          "IAM Users",
			ShortName:     "iam-user",
			Aliases:       []string{"iam-user", "iam-users", "users", "iam_users"},
			Category:      "SECURITY & IAM",
			CloudTrailKey: "Username:ID",
			Columns: []Column{
				{Key: "user_name", Title: "User Name", Width: 32, Sortable: true},
				{Key: "user_id", Title: "User ID", Width: 22, Sortable: true},
				{Key: "path", Title: "Path", Width: 20, Sortable: true},
				{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
				{Key: "password_last_used", Title: "Password Last Used", Width: 22, Sortable: true},
			},
			// Wave 2 enricher surfaces users with old active keys or no
			// recent sign-in — Wave 1 list is config-only.
			Color: func(_ Resource) Color { return ColorHealthy },
		},
		{
			Name:          "IAM Groups",
			ShortName:     "iam-group",
			Aliases:       []string{"iam-group", "iam-groups", "groups", "iam_groups"},
			Category:      "SECURITY & IAM",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "group_name", Title: "Group Name", Width: 32, Sortable: true},
				{Key: "group_id", Title: "Group ID", Width: 22, Sortable: true},
				{Key: "path", Title: "Path", Width: 20, Sortable: true},
				{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
				{Key: "arn", Title: "ARN", Width: 60, Sortable: true},
			},
			Color: func(_ Resource) Color { return ColorHealthy },
			Children: []ChildViewDef{
				{
					ChildType:      "iam_group_members",
					Key:            "enter",
					ContextKeys:    map[string]string{"group_name": "ID"},
					DisplayNameKey: "group_name",
				},
			},
		},
		{
			Name:          "WAF Web ACLs",
			ShortName:     "waf",
			Aliases:       []string{"waf", "webacl", "web-acl"},
			Category:      "SECURITY & IAM",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 28, Sortable: true},
				{Key: "id", Title: "ID", Width: 38, Sortable: true},
				{Key: "description", Title: "Description", Width: 36, Sortable: false},
			},
			Color: func(_ Resource) Color { return ColorHealthy },
		},
	}
}
