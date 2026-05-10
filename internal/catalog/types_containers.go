package catalog

import (
	"strconv"

	"github.com/k2m30/a9s/v3/internal/domain"
)

func colorEKSCluster(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	if r.Fields["status"] == "FAILED" {
		return domain.ColorBroken
	}
	hasIssues := false
	if n, err := strconv.Atoi(r.Fields["health_issues_count"]); err == nil && n > 0 {
		hasIssues = true
	}
	switch r.Fields["status"] {
	case "ACTIVE":
		if hasIssues {
			return domain.ColorWarning
		}
		return domain.ColorHealthy
	case "CREATING", "UPDATING", "DELETING":
		return domain.ColorWarning
	}
	if hasIssues {
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

func colorEKSNodeGroup(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	hasIssues := false
	if n, err := strconv.Atoi(r.Fields["health_issues_count"]); err == nil && n > 0 {
		hasIssues = true
	}
	switch r.Fields["status"] {
	case "ACTIVE":
		if hasIssues {
			return domain.ColorWarning
		}
		return domain.ColorHealthy
	case "CREATING", "UPDATING", "DELETING":
		return domain.ColorWarning
	case "CREATE_FAILED", "DELETE_FAILED", "DEGRADED":
		return domain.ColorBroken
	}
	if hasIssues {
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

var containersTypes = []ResourceTypeDef{
	{
		Name:          "EKS Clusters",
		ShortName:     "eks",
		Aliases:       []string{"eks", "kubernetes", "k8s"},
		Category:      "CONTAINERS",
		CloudTrailKey: "ResourceName:Fields.arn",
		Columns: []domain.Column{
			{Key: "cluster_name", Title: "Cluster Name", Width: 28, Sortable: true},
			{Key: "version", Title: "Version", Width: 10, Sortable: true},
			{Key: "status", Title: "Status", Width: 14, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
			{Key: "platform_version", Title: "Platform Version", Width: 18, Sortable: true},
		},
		Color: colorEKSCluster,
	},
	{
		Name:          "EKS Node Groups",
		ShortName:     "ng",
		Aliases:       []string{"ng", "nodegroups", "node-groups"},
		Category:      "CONTAINERS",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "nodegroup_name", Title: "Node Group", Width: 28, Sortable: true},
			{Key: "cluster_name", Title: "Cluster", Width: 24, Sortable: true},
			{Key: "status", Title: "Status", Width: 14, Sortable: true},
			{Key: "instance_types", Title: "Instance Types", Width: 20, Sortable: false},
			{Key: "desired_size", Title: "Desired", Width: 9, Sortable: true},
		},
		Color: colorEKSNodeGroup,
	},
}
