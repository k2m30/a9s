package config

func containersDefaultViews() map[string]ViewDef {
	return map[string]ViewDef{
		"eks": {
			List: []ListColumn{
				{Title: "Cluster Name", Path: "Name", Width: 28},
				{Title: "Version", Path: "Version", Width: 10},
				{Title: "Status", Path: "Status", Width: 14},
				{Title: "Endpoint", Path: "Endpoint", Width: 48},
				{Title: "Platform Version", Path: "PlatformVersion", Width: 18},
			},
			Detail: []string{
				"Name", "Version", "Status", "Endpoint",
				"PlatformVersion", "Arn", "RoleArn", "KubernetesNetworkConfig",
				"ResourcesVpcConfig", "Logging", "Identity", "CreatedAt", "Tags",
			},
		},
		"ng": {
			List: []ListColumn{
				{Title: "Node Group", Path: "NodegroupName", Width: 28},
				{Title: "Cluster", Path: "ClusterName", Width: 24},
				{Title: "Status", Path: "Status", Width: 14},
				{Title: "Instance Types", Path: "InstanceTypes", Width: 20},
				{Title: "Desired", Path: "ScalingConfig.DesiredSize", Width: 9},
			},
			Detail: []string{
				"NodegroupName", "ClusterName", "Status", "InstanceTypes",
				"AmiType", "CapacityType", "DiskSize", "ScalingConfig",
				"NodeRole", "NodegroupArn", "ReleaseVersion", "Version",
				"Subnets", "LaunchTemplate", "Labels", "Taints",
				"Tags", "Health", "CreatedAt",
			},
		},
	}
}
