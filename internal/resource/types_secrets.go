package resource

import (
	"strings"
	"time"
)

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
			Color: func(r Resource) Color {
				if r.Fields["rotation_enabled"] == "No" {
					return ColorWarning
				}
				if la := r.Fields["last_accessed"]; la != "" {
					if t, err := time.Parse("2006-01-02", la); err == nil {
						if time.Since(t) > 180*24*time.Hour {
							return ColorWarning
						}
					}
				}
				if lc := r.Fields["last_changed"]; lc != "" {
					if t, err := time.Parse("2006-01-02", lc); err == nil {
						if time.Since(t) > 365*24*time.Hour {
							return ColorWarning
						}
					}
				}
				return ColorHealthy
			},
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
			Color: func(r Resource) Color {
				sensitiveSuffixes := []string{
					"_password", "_secret", "_token", "_apikey",
					"_api_key", "_credentials", "_passwd",
				}
				name := strings.ToLower(r.Fields["name"])
				if r.Fields["type"] == "String" {
					for _, suffix := range sensitiveSuffixes {
						if strings.HasSuffix(name, suffix) {
							return ColorBroken
						}
					}
				}
				if lm := r.Fields["last_modified"]; lm != "" {
					if t, err := time.Parse("2006-01-02 15:04", lm); err == nil {
						if time.Since(t) > 365*24*time.Hour {
							return ColorWarning
						}
					}
				}
				return ColorHealthy
			},
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
