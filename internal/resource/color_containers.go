package resource

import "strconv"

func init() {
	colorRegistry["eks"] = colorEKSCluster
	colorRegistry["ng"] = colorEKSNodeGroup
}

func colorEKSCluster(r Resource) Color {
	for _, f := range r.Findings {
		if f.Source == "wave1" {
			return ColorFromSeverity(f.Severity)
		}
	}

	if r.Fields["status"] == "FAILED" {
		return ColorBroken
	}
	hasIssues := false
	if n, err := strconv.Atoi(r.Fields["health_issues_count"]); err == nil && n > 0 {
		hasIssues = true
	}
	switch r.Fields["status"] {
	case "ACTIVE":
		if hasIssues {
			return ColorWarning
		}
		return ColorHealthy
	case "CREATING", "UPDATING", "DELETING":
		return ColorWarning
	}
	if hasIssues {
		return ColorWarning
	}
	return ColorHealthy
}

func colorEKSNodeGroup(r Resource) Color {
	for _, f := range r.Findings {
		if f.Source == "wave1" {
			return ColorFromSeverity(f.Severity)
		}
	}
	hasIssues := false
	if n, err := strconv.Atoi(r.Fields["health_issues_count"]); err == nil && n > 0 {
		hasIssues = true
	}
	switch r.Fields["status"] {
	case "ACTIVE":
		if hasIssues {
			return ColorWarning
		}
		return ColorHealthy
	case "CREATING", "UPDATING", "DELETING":
		return ColorWarning
	case "CREATE_FAILED", "DELETE_FAILED", "DEGRADED":
		return ColorBroken
	}
	if hasIssues {
		return ColorWarning
	}
	return ColorHealthy
}
