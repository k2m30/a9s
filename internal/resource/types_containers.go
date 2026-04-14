package resource

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
				switch r.Fields["status"] {
				case "ACTIVE":
					return ColorHealthy
				case "CREATING", "UPDATING", "DELETING":
					return ColorWarning
				case "FAILED":
					return ColorBroken
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
			Color: func(_ Resource) Color { return ColorHealthy },
			AlwaysHealthy: true,
		},
	}
}
