package config

func containersDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"eks": {
			List: []ListColumn{
				{Title: "Cluster Name", Path: "Name", Width: 28},
				{Title: "Version", Path: "Version", Width: 10},
				{Title: "Status", Path: "Status", Width: 14},
				{Title: "Issues", Key: "health_issues", Width: 28},
				{Title: "Endpoint", Path: "Endpoint", Width: 48},
				{Title: "Platform Version", Path: "PlatformVersion", Width: 18},
			},
			Detail: []DetailField{
				{Path: "Name"}, {Path: "Version"}, {Path: "Status"}, {Path: "Endpoint"},
				{Path: "PlatformVersion"}, {Path: "Arn"}, {Path: "RoleArn"}, {Path: "KubernetesNetworkConfig"},
				{Path: "ResourcesVpcConfig"}, {Path: "Logging"}, {Path: "Identity"}, {Path: "CreatedAt"}, {Path: "Tags"},
				{Key: "health_issues", Label: "Health Issues"},
			},
		},
		"ng": {
			List: []ListColumn{
				{Title: "Node Group", Path: "NodegroupName", Width: 28},
				{Title: "Cluster", Path: "ClusterName", Width: 24},
				{Title: "Status", Path: "Status", Width: 14},
				{Title: "Issues", Key: "health_issues", Width: 28},
				{Title: "Instance Types", Path: "InstanceTypes", Width: 20},
				{Title: "Desired", Path: "ScalingConfig.DesiredSize", Width: 9},
			},
			Detail: []DetailField{
				{Path: "NodegroupName"}, {Path: "ClusterName"}, {Path: "Status"}, {Path: "InstanceTypes"},
				{Path: "AmiType"}, {Path: "CapacityType"}, {Path: "DiskSize"}, {Path: "ScalingConfig"},
				{Path: "NodeRole"}, {Path: "NodegroupArn"}, {Path: "ReleaseVersion"}, {Path: "Version"},
				{Path: "Subnets"}, {Path: "LaunchTemplate"}, {Path: "Labels"}, {Path: "Taints"},
				{Path: "Tags"}, {Path: "Health"}, {Path: "CreatedAt"},
				{Key: "health_issues", Label: "Health Issues"},
			},
		},
	}
}
