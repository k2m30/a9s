package resource

import "strconv"

func containersResourceTypes() []ResourceTypeDef {
	return []ResourceTypeDef{
		{
			Name:          "EKS Clusters",
			ShortName:     "eks",
			Aliases:       []string{"eks", "kubernetes", "k8s"},
			Category:      "CONTAINERS",
			CloudTrailKey: "ResourceName:Fields.arn",
			Columns: []Column{
				{Key: "cluster_name", Title: "Cluster Name", Width: 28, Sortable: true},
				{Key: "version", Title: "Version", Width: 10, Sortable: true},
				{Key: "status", Title: "Status", Width: 14, Sortable: true},
				{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
				{Key: "platform_version", Title: "Platform Version", Width: 18, Sortable: true},
			},
			Color: func(r Resource) Color {
				// Color: wave1 Findings (CREATING/UPDATING/FAILED from the fetcher) drive
				// row color; structural fallback covers ACTIVE / DELETING (no Finding).
				for _, f := range r.Findings {
					if f.Source == "wave1" {
						return ColorFromSeverity(f.Severity)
					}
				}

				// Structural fallback: FAILED without a wave1 Finding.
				if r.Fields["status"] == "FAILED" {
					return ColorBroken
				}
				// Wave 2: health.issues[] non-empty → Warning. Doc treats issues
				// as advisory health signals, not failure (cluster is still
				// running). Status-FAILED above already carries the failure.
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
			},
		},
		{
			Name:          "EKS Node Groups",
			ShortName:     "ng",
			Aliases:       []string{"ng", "nodegroups", "node-groups"},
			Category:      "CONTAINERS",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "nodegroup_name", Title: "Node Group", Width: 28, Sortable: true},
				{Key: "cluster_name", Title: "Cluster", Width: 24, Sortable: true},
				{Key: "status", Title: "Status", Width: 14, Sortable: true},
				{Key: "instance_types", Title: "Instance Types", Width: 20, Sortable: false},
				{Key: "desired_size", Title: "Desired", Width: 9, Sortable: true},
			},
			Color: func(r Resource) Color {
				// PR-03c: wave1 Findings (lifecycle/state) drive color first.
				for _, f := range r.Findings {
					if f.Source == "wave1" {
						return ColorFromSeverity(f.Severity)
					}
				}
				// Structural fallback: covers wave2 signals (health_issues_count)
				// and healthy/terminal lifecycle states when Findings is empty.
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
				// Empty / unknown status: healthy unless health.issues set.
				if hasIssues {
					return ColorWarning
				}
				return ColorHealthy
			},
		},
	}
}
