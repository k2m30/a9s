package resource

import (
	"strings"
	"time"
)

func init() {
	colorRegistry["role"] = colorRole
	colorRegistry["policy"] = colorPolicy
	colorRegistry["iam-user"] = colorIAMUser
	colorRegistry["iam-group"] = func(_ Resource) Color { return ColorHealthy }
	colorRegistry["waf"] = func(_ Resource) Color { return ColorHealthy }
}

func colorRole(r Resource) Color {
	doc := r.Fields["assume_role_policy_document"]
	if doc != "" &&
		(strings.Contains(doc, `"Principal":"*"`) || strings.Contains(doc, `"Principal": "*"`)) {
		return ColorBroken
	}
	return ColorHealthy
}

func colorPolicy(r Resource) Color {
	if r.Fields["attachment_count"] == "0" && r.Fields["is_attachable"] == "true" {
		return ColorWarning
	}
	return ColorHealthy
}

func colorIAMUser(r Resource) Color {
	if r.Fields["has_console_password"] != "true" {
		return ColorHealthy
	}
	plu := r.Fields["password_last_used"]
	t, err := time.Parse("2006-01-02 15:04", plu)
	if err != nil {
		return ColorHealthy
	}
	if time.Since(t) > 90*24*time.Hour {
		return ColorWarning
	}
	return ColorHealthy
}
