package resource

func secretsResourceTypes() []ResourceTypeDef {
	return []ResourceTypeDef{
		{
			Name:          "Secrets Manager",
			ShortName:     "secrets",
			Aliases:       []string{"secrets", "secretsmanager", "sm"},
			Category:      "SECRETS & CONFIG",
			CloudTrailKey: "ResourceName:Fields.arn",
			Columns: []Column{
				{Key: "secret_name", Title: "Secret Name", Width: 36, Sortable: true},
				{Key: "description", Title: "Description", Width: 30, Sortable: false},
				{Key: "last_accessed", Title: "Last Accessed", Width: 18, Sortable: true},
				{Key: "last_changed", Title: "Last Changed", Width: 18, Sortable: true},
				{Key: "rotation_enabled", Title: "Rotation", Width: 10, Sortable: true},
			},
			// Wave 2 enricher surfaces secrets whose scheduled rotation has
			// failed — Wave 1 list carries no runtime signal.
			Color: func(_ Resource) Color { return ColorHealthy },
		},
		{
			Name:          "SSM Parameters",
			ShortName:     "ssm",
			Aliases:       []string{"ssm", "parameters", "parameter-store"},
			Category:      "SECRETS & CONFIG",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 40, Sortable: true},
				{Key: "type", Title: "Type", Width: 14, Sortable: true},
				{Key: "version", Title: "Version", Width: 8, Sortable: true},
				{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
				{Key: "description", Title: "Description", Width: 30, Sortable: false},
			},
			Color: func(_ Resource) Color { return ColorHealthy },
		},
		{
			Name:          "KMS Keys",
			ShortName:     "kms",
			Aliases:       []string{"kms", "keys"},
			Category:      "SECRETS & CONFIG",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "alias", Title: "Alias", Width: 32, Sortable: true},
				{Key: "key_id", Title: "Key ID", Width: 38, Sortable: true},
				{Key: "status", Title: "Status", Width: 12, Sortable: true},
				{Key: "description", Title: "Description", Width: 36, Sortable: false},
			},
			Color: func(r Resource) Color {
				switch r.Fields["key_state"] {
				case "Enabled":
					return ColorHealthy
				case "Disabled":
					return ColorDim
				case "PendingDeletion", "PendingImport":
					return ColorWarning
				case "Unavailable":
					return ColorBroken
				}
				return ColorHealthy
			},
		},
	}
}
