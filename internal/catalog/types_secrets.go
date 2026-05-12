package catalog

import (
	"strings"
	"time"

	"github.com/k2m30/a9s/v3/internal/domain"
)

func colorSecrets(r domain.Resource) domain.Color {
	if r.Fields["rotation_enabled"] == "No" {
		return domain.ColorWarning
	}
	if la := r.Fields["last_accessed"]; la != "" {
		if t, err := time.Parse("2006-01-02", la); err == nil {
			if time.Since(t) > 180*24*time.Hour {
				return domain.ColorWarning
			}
		}
	}
	if lc := r.Fields["last_changed"]; lc != "" {
		if t, err := time.Parse("2006-01-02", lc); err == nil {
			if time.Since(t) > 365*24*time.Hour {
				return domain.ColorWarning
			}
		}
	}
	return domain.ColorHealthy
}

func colorSSM(r domain.Resource) domain.Color {
	sensitiveSuffixes := []string{
		"_password", "_secret", "_token", "_apikey",
		"_api_key", "_credentials", "_passwd",
	}
	name := strings.ToLower(r.Fields["name"])
	if r.Fields["type"] == "String" {
		for _, suffix := range sensitiveSuffixes {
			if strings.HasSuffix(name, suffix) {
				return domain.ColorBroken
			}
		}
	}
	if lm := r.Fields["last_modified"]; lm != "" {
		if t, err := time.Parse("2006-01-02 15:04", lm); err == nil {
			if time.Since(t) > 365*24*time.Hour {
				return domain.ColorWarning
			}
		}
	}
	return domain.ColorHealthy
}

func colorKMS(r domain.Resource) domain.Color {
	switch r.Fields["key_state"] {
	case "Enabled":
		return domain.ColorHealthy
	case "Disabled":
		return domain.ColorDim
	case "PendingDeletion", "PendingImport":
		return domain.ColorWarning
	case "Unavailable":
		return domain.ColorBroken
	}
	return domain.ColorHealthy
}

var secretsTypes = []ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "Secrets Manager",
		ShortName:     "secrets",
		Aliases:       []string{"secrets", "secretsmanager", "sm"},
		Category:      "SECRETS & CONFIG",
		CloudTrailKey: "ResourceName:Fields.arn",
		Columns: []domain.Column{
			{Key: "secret_name", Title: "Secret Name", Width: 36, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
			{Key: "last_accessed", Title: "Last Accessed", Width: 18, Sortable: true},
			{Key: "last_changed", Title: "Last Changed", Width: 18, Sortable: true},
			{Key: "rotation_enabled", Title: "Rotation", Width: 10, Sortable: true},
		},
		Color: colorSecrets,
	},
	{
		Name:          "SSM Parameters",
		ShortName:     "ssm",
		Aliases:       []string{"ssm", "parameters", "parameter-store"},
		Category:      "SECRETS & CONFIG",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 40, Sortable: true},
			{Key: "type", Title: "Type", Width: 14, Sortable: true},
			{Key: "version", Title: "Version", Width: 8, Sortable: true},
			{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
		},
		Color: colorSSM,
	},
	{
		Name:          "KMS Keys",
		ShortName:     "kms",
		Aliases:       []string{"kms", "keys"},
		Category:      "SECRETS & CONFIG",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "alias", Title: "Alias", Width: 32, Sortable: true},
			{Key: "key_id", Title: "Key ID", Width: 38, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "description", Title: "Description", Width: 36, Sortable: false},
		},
		Color: colorKMS,
	},
}
