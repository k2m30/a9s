package resource

import (
	"strings"
	"time"
)

func init() {
	colorRegistry["secrets"] = colorSecrets
	colorRegistry["ssm"] = colorSSM
	colorRegistry["kms"] = colorKMS
}

func colorSecrets(r Resource) Color {
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
}

func colorSSM(r Resource) Color {
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
}

func colorKMS(r Resource) Color {
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
}
